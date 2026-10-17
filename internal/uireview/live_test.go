package uireview

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/models"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

var planted = []string{"focus-visible", "contrast", "target-size"}

// liveModel is a lane's direct model through the AI gateway, as the binary
// resolves it on the direct host.
func liveModel(t *testing.T, lane verbs.LaneID, base, key string) (models.Resolved, func(context.Context, models.Request) (models.Response, error)) {
	t.Helper()
	m, err := models.Resolver{}.ResolveDirect(lane, verbs.DirectHost, false)
	if err != nil {
		t.Fatal(err)
	}
	route, err := models.RouteFor(config.Models{Gateway: config.Gateway{BaseURL: base, Secret: "ai-gateway"}}, m.Model, m.Family)
	if err != nil {
		t.Fatal(err)
	}
	c := models.HTTPClient{BaseURL: route.BaseURL, Style: route.Style, Key: key}
	return m, func(ctx context.Context, r models.Request) (models.Response, error) {
		r.Model = m.Model
		resp, err := c.Complete(ctx, r)
		t.Logf("%s via %s: stop %q, %d in / %d out tokens, cost %v, err %v", m.Model, route.Style, resp.StopReason, resp.TokensIn, resp.TokensOut, resp.CostUSD, err)
		return resp, err
	}
}

// liveResult is what the live acceptance writes for the run's record.
type liveResult struct {
	Model    string      `json:"lane_a_model"`
	Builder  string      `json:"polish_model"`
	Planted  *Critique   `json:"planted"`
	Clean    *Critique   `json:"clean"`
	Polished *Critique   `json:"polished,omitempty"`
	Polish   *Comparison `json:"polish,omitempty"`
	HTML     string      `json:"polished_html,omitempty"`
	Verdicts []string    `json:"verdicts"`
}

// TestReviewLive is the WP6.6 acceptance against the real ui-review lane
// (gemini-3.8-flash through the AI gateway) and the real lane B tools:
// each planted defect is caught by lane A in at least 2 of 3 runs and by lane
// B; the clean page ships from both lanes; polish (the build lane's model
// applying the critique, standing in for the host) removes all three on the
// re-run. Runs only with AAYU_LIVE=1, the gateway URL and key, and the floor
// tools (ux-review-live.yml, environment ai-gateway, dispatched on dev).
func TestReviewLive(t *testing.T) {
	base, key := os.Getenv("AAYU_AI_GATEWAY_URL"), os.Getenv("AAYU_SECRET_AI_GATEWAY")
	switch {
	case os.Getenv("AAYU_LIVE") != "1":
		t.Skip("live review: set AAYU_LIVE=1")
	case base == "" || key == "":
		t.Skip("live review: AAYU_AI_GATEWAY_URL or AAYU_SECRET_AI_GATEWAY is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
	defer cancel()
	out := os.Getenv("AAYU_REVIEW_OUT")
	if out == "" {
		out = t.TempDir()
	}
	rev, laneA := liveModel(t, verbs.LaneUIReview, base, key)
	build, builder := liveModel(t, verbs.LaneBuild, base, key)
	if err := models.CheckFamilies("live", build.Family, rev.Family); err != nil {
		t.Fatal(err)
	}
	res := liveResult{Model: rev.Model, Builder: build.Model}
	defer func() {
		b, _ := json.MarshalIndent(res, "", "  ")
		_ = os.WriteFile(filepath.Join(out, "review-live.json"), b, 0o644)
		if s := os.Getenv("GITHUB_STEP_SUMMARY"); s != "" {
			f, err := os.OpenFile(s, os.O_APPEND|os.O_WRONLY, 0o644)
			if err == nil {
				fmt.Fprintf(f, "### UI review live acceptance (WP6.6)\n\nLane A: `%s`; polish: `%s`.\n\n", rev.Model, build.Model)
				for _, v := range res.Verdicts {
					fmt.Fprintf(f, "- %s\n", v)
				}
				_ = f.Close()
			}
		}
	}()
	verdict := func(ok bool, format string, args ...any) {
		v := fmt.Sprintf(format, args...)
		if ok {
			v = "PASS " + v
		} else {
			v = "FAIL " + v
			t.Error(v)
		}
		res.Verdicts = append(res.Verdicts, v)
		t.Log(v)
	}

	polished := map[string][]byte{}
	site := fixtureSite(t, polished)
	b := testBrowser(t)
	b.Work = out // keep the screenshots for the run's artifact
	reviewer := &Reviewer{Tools: b, Complete: laneA, Model: rev.Model, Effort: "medium", K: K, MinTargetPx: 44, Now: time.Now}
	run := func(id, path string) *Critique {
		c, _, err := reviewer.Review(ctx, []State{{ID: id, URL: site.URL + path}})
		if err != nil {
			t.Fatalf("review %s: %v", path, err)
		}
		return c
	}
	runsReporting := func(c *Critique, kind string) int {
		n := 0
		for _, r := range c.LaneA.Runs {
			if r.Reply != nil && slices.ContainsFunc(r.Reply.Findings, func(f RawFinding) bool { return f.Kind == kind && f.Severity >= MinSeverity }) {
				n++
			}
		}
		return n
	}

	res.Planted = run("planted", "/planted")
	for _, k := range planted {
		n := runsReporting(res.Planted, k)
		verdict(n >= Agree, "planted %s: lane A caught it in %d of %d runs", k, n, K)
		verdict(slices.Contains(kinds(res.Planted.LaneB), k), "planted %s: lane B caught it in 1 of 1 run", k)
	}
	res.Clean = run("clean", "/clean")
	verdict(res.Clean.LaneA.Disposition == Ship, "clean: lane A disposition %s %v", res.Clean.LaneA.Disposition, kinds(res.Clean.LaneA))
	verdict(res.Clean.LaneB.Disposition == Ship, "clean: lane B disposition %s %v", res.Clean.LaneB.Disposition, kinds(res.Clean.LaneB))

	// polish: the build lane's model applies the critique to the page's source, as the host would.
	src, err := os.ReadFile("testdata/planted.html")
	if err != nil {
		t.Fatal(err)
	}
	var findings strings.Builder
	for _, l := range []Lane{res.Planted.LaneA, res.Planted.LaneB} {
		for _, f := range l.Findings {
			fmt.Fprintf(&findings, "- lane %s, %s (severity %d) on %s: %s. Fix: %s\n", l.Name, f.Kind, f.Severity, f.Element, f.Summary, f.Fix)
		}
	}
	skill, _ := os.ReadFile("../../skills/polish/SKILL.md")
	body := string(skill)
	if i := strings.Index(body[3:], "\n---"); i >= 0 {
		body = body[i+7:]
	}
	resp, err := builder(ctx, models.Request{MaxTokens: 8000,
		System: "You are the host's model running the aayudha polish skill below on a pull request.\n\n" + body,
		Messages: []models.Message{{Role: "user", Content: "Apply this UI review critique to the page. Change only what the findings need.\n\nCritique:\n" + findings.String() +
			"\nThe page, planted.html:\n```html\n" + string(src) + "```\n\nReply with the complete corrected file in one ```html fenced block and nothing else."}}})
	if err != nil {
		t.Fatalf("polish model: %v", err)
	}
	m := regexp.MustCompile("(?s)```html\\s*\\n(.*?)```").FindStringSubmatch(resp.Text)
	if m == nil {
		t.Fatalf("polish model reply has no html block: %.300s", resp.Text)
	}
	res.HTML = m[1]
	polished["polished"] = []byte(m[1])
	res.Polished = run("polished", "/polished")
	before := *res.Planted // the same state, renamed so Compare matches it with the polished capture
	before.LaneA.Findings, before.LaneB.Findings = renamed(before.LaneA.Findings, "polished"), renamed(before.LaneB.Findings, "polished")
	cmp := Compare(&before, res.Polished)
	res.Polish = &cmp
	for _, k := range planted {
		gone := !slices.ContainsFunc(cmp.Remaining, func(f Finding) bool { return f.Kind == k }) &&
			!slices.Contains(kinds(res.Polished.LaneA), k) && !slices.Contains(kinds(res.Polished.LaneB), k)
		verdict(gone, "polish removed %s (lane A runs reporting it after polish: %d of %d)", k, runsReporting(res.Polished, k), K)
	}
}

func renamed(fs []Finding, state string) []Finding {
	out := slices.Clone(fs)
	for i := range out {
		out[i].State = state
	}
	return out
}
