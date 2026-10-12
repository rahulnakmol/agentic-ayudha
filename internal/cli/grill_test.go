package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/rahulnakmol/agentic-ayudha/internal/design"
	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
	"github.com/rahulnakmol/agentic-ayudha/internal/secrets"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// githubSearch answers the githubread adapter (api.github.com) and sends the
// fake GitHub's loopback requests to the network.
type githubSearch struct{ searches []string }

func (g *githubSearch) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Hostname() == "127.0.0.1" {
		return http.DefaultTransport.RoundTrip(req)
	}
	body := `{"login":"aayu-reader","type":"User"}`
	h := http.Header{"X-Oauth-Scopes": {"read:org"}}
	if req.URL.Path == "/search/issues" {
		g.searches = append(g.searches, req.URL.Query().Get("q"))
		body, h = `{"total_count":37,"incomplete_results":false,"items":[]}`, http.Header{}
	}
	h.Set("Content-Type", "application/json")
	return &http.Response{StatusCode: 200, Header: h, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
}

// WP5.3 accept: a Goal whose Metric no adapter can reach is refused Ready
// with the message naming the metric, and nothing is written; once the
// integration answers, grill writes the reproducible baseline tuple into the
// Baseline section before the Target and creates the Goal.
func TestGrillGoalBaseline(t *testing.T) {
	g := fake.New(t, "o/r", "User")
	body := workitems.Render(mustType(t, "goal"), map[string]string{
		"summary":       "Claimants finish their first claim without help.",
		"beneficiaries": "claimants",
		"metric":        "github:prs_merged base:dev",
		"target":        ">= 45 by 2026-12-31",
	})
	rt := &githubSearch{}
	env := map[string]string{"AAYU_GITHUB_TOKEN": "human-token", "AAYU_SECRET_GITHUB_READ": "github_pat_read"}
	now := func() time.Time { return time.Date(2026, 10, 12, 9, 30, 0, 0, time.UTC) }
	opts := Options{GitHubAPI: g.URL(), GitHubGap: -1, Now: now, HTTP: &http.Client{Transport: rt},
		Secrets: secrets.New(secrets.Env{Getenv: func(k string) string { return env[k] }})}
	grill := func(sets ...string) (int, GrillResult, string) {
		args := []string{"grill", "goal", "--repo", "o/r", "--json", "--title", "First claims without help", "--body", body, "--create"}
		for _, s := range sets {
			args = append(args, "--set", s)
		}
		code, out, errOut := run(t, env, opts, args...)
		var res GrillResult
		_ = json.NewDecoder(strings.NewReader(out)).Decode(&res)
		return code, res, errOut
	}

	code, res, errOut := grill()
	if code != ExitBlocked || res.Pass || res.Issue != 0 || len(g.Issues) != 0 {
		t.Fatalf("goal without a reachable baseline: exit %d %+v %s", code, res, errOut)
	}
	msg := problemsText(res)
	if !strings.Contains(msg, `baseline: no reachable baseline for metric "github:prs_merged base:dev"`) || !strings.Contains(msg, "[integrations.github]") {
		t.Fatalf("refusal does not name the metric and the integration:\n%s", msg)
	}
	if len(rt.searches) != 0 {
		t.Fatalf("a request was sent without an integration: %v", rt.searches)
	}

	code, res, errOut = grill("integrations.github.secret=github-read", "integrations.github.scope=read", "integrations.github.project=o/r")
	if code != 0 || !res.Pass || res.Issue == 0 || res.Baseline == nil || res.Baseline.Value != 37 {
		t.Fatalf("goal with a reachable baseline: exit %d %+v %s", code, res, errOut)
	}
	// The window is the 28 whole UTC days before today.
	if want := "merged:>=2026-09-14T00:00:00Z merged:<2026-10-12T00:00:00Z"; len(rt.searches) != 1 || !strings.Contains(rt.searches[0], want) {
		t.Fatalf("search %v; want the window %s", rt.searches, want)
	}
	g.Mu.Lock()
	written := g.Issues[res.Issue].Body
	g.Mu.Unlock()
	it, _ := workitems.Parse(res.Issue, "x", written, "", []string{"type:goal"})
	for _, want := range []string{"37\n", "Population:", "Window: 2026-09-14T00:00:00Z to 2026-10-12T00:00:00Z", "GET https://api.github.com/search/issues?", "Fetched at: 2026-10-12T09:30:00Z"} {
		if !strings.Contains(it.Fields["baseline"], want) {
			t.Fatalf("Baseline section lacks %q:\n%s", want, it.Fields["baseline"])
		}
	}
	if it.Fields["target"] != ">= 45 by 2026-12-31" {
		t.Fatalf("target changed: %q", it.Fields["target"])
	}
}

func mustType(t *testing.T, id string) workitems.Type {
	t.Helper()
	ty, ok := workitems.FindType(id)
	if !ok {
		t.Fatal(id)
	}
	return ty
}

// WP5.3 grill brand: every pair is checked against WCAG 2.2 AA; a failing
// pair is refused and nothing is written; a passing brand is written to
// design/brand.yaml only with --create.
func TestGrillBrand(t *testing.T) {
	dir := gitInit(t, "dev", map[string]string{"README.md": "x\n"})
	brand := func(primary string) string {
		return `version: 1
name: Benefits QA
voice: {tone: [plain, warm]}
palette: {ink: "#1a1a1a", surface: "#ffffff", primary: "` + primary + `"}
pairs:
  - {fg: ink, bg: surface, use: text}
  - {fg: primary, bg: surface, use: text}
`
	}
	opts := Options{Dir: dir}
	// #767676 on white is 4.54:1 (passes text); #777777 is 4.47:1 (fails text, passes ui).
	code, out, _ := run(t, nil, opts, "grill", "brand", "--json", "--body", brand("#777777"), "--create")
	var res GrillResult
	_ = json.NewDecoder(strings.NewReader(out)).Decode(&res)
	if code != ExitBlocked || res.Pass || len(res.Pairs) != 2 || res.Pairs[1].Pass || res.Pairs[1].Ratio != 4.47 {
		t.Fatalf("failing pair: exit %d %s", code, out)
	}
	if _, err := os.Stat(filepath.Join(dir, "design", "brand.yaml")); !os.IsNotExist(err) {
		t.Fatalf("a failing brand was written: %v", err)
	}
	if code, out, errOut := run(t, nil, opts, "grill", "brand", "--body", brand("#767676")); code != 0 || !strings.Contains(out, "4.54:1 passes AA") {
		t.Fatalf("passing brand without --create: %d %s %s", code, out, errOut)
	}
	if _, err := os.Stat(filepath.Join(dir, "design", "brand.yaml")); !os.IsNotExist(err) {
		t.Fatal("written without --create")
	}
	if code, out, errOut := run(t, nil, opts, "grill", "brand", "--body", brand("#767676"), "--create"); code != 0 || !strings.Contains(out, "wrote design/brand.yaml") {
		t.Fatalf("create: %d %s %s", code, out, errOut)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "design", "brand.yaml"))
	br, err := design.ParseBrand(b)
	if err != nil || br.Palette["primary"] != "#767676" || len(br.Pairs) != 2 {
		t.Fatalf("design/brand.yaml: %v\n%s", err, b)
	}
	if code, _, errOut := run(t, nil, opts, "grill", "brand", "--body", "version: 1\nname: x\ncolours: {}\n"); code != ExitUsage || !strings.Contains(errOut, "colours") {
		t.Fatalf("unknown key: %d %s", code, errOut)
	}
}

func keys(m *grillModel, ks ...string) {
	for _, k := range ks {
		var msg tea.KeyPressMsg
		switch k {
		case "enter":
			msg = tea.KeyPressMsg{Code: tea.KeyEnter}
		case "backspace":
			msg = tea.KeyPressMsg{Code: tea.KeyBackspace}
		case "ctrl+d":
			msg = tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl}
		case "shift+tab":
			msg = tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
		default:
			for _, r := range k {
				m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
			}
			continue
		}
		m.Update(msg)
	}
}

// The TUI walks the type's questions with the reference's hints, keeps a
// multi-line answer until an empty line, shows validation problems on the
// review screen and jumps to the first one, and creates only once the draft
// passes.
func TestGrillTUI(t *testing.T) {
	ty := mustType(t, "feedback")
	m := &grillModel{name: "grill feedback", steps: workItemSteps(ty, os.DirFS(filepath.Join("..", "..")))}
	var ids []string
	for _, s := range m.steps {
		ids = append(ids, s.ID)
	}
	if got := strings.Join(ids, ","); got != "title,summary,context,links,labels" {
		t.Fatalf("steps: %s", got)
	}
	if v := m.View().Content; !strings.Contains(v, "Title (required)") || !strings.Contains(v, "question 1 of 5") {
		t.Fatalf("first view:\n%s", v)
	}
	m.check = func(v map[string]string) (string, []string) {
		d := draftFromAnswers(ty, v)
		it, ps := workitems.Parse(0, d.Title, d.Body, "", []string{"type:feedback"})
		ps = append(ps, workitems.Validate(it, workitems.StageCreate)...)
		var out []string
		for _, p := range ps {
			out = append(out, p.String())
		}
		return d.Body, out
	}
	keys(m, "Export fails", "enter") // title
	if hints := m.View().Content; !strings.Contains(hints, "What happened or what you need (required)") || !strings.Contains(hints, "What did the user try to do") {
		t.Fatalf("summary hints from references/feedback.md:\n%s", hints)
	}
	keys(m, "enter", "enter", "ctrl+d", "enter") // empty summary, context, links; labels
	if v := m.View().Content; !strings.Contains(v, "summary: What happened or what you need is required at Create") || !strings.Contains(v, "[e] edit") {
		t.Fatalf("review with a problem:\n%s", v)
	}
	keys(m, "c")
	if m.create {
		t.Fatal("created a draft that does not pass")
	}
	keys(m, "e")
	if m.steps[m.i].ID != "summary" {
		t.Fatalf("e jumped to %s", m.steps[m.i].ID)
	}
	keys(m, "Export of the claims list x", "backspace", "fails.", "enter", "It times out.", "enter", "enter")
	if got := m.steps[1].Value; got != "Export of the claims list fails.\nIt times out." {
		t.Fatalf("multi-line answer %q", got)
	}
	keys(m, "ctrl+d", "ctrl+d", "enter")
	if m.i != len(m.steps) || len(m.problems) != 0 {
		t.Fatalf("review: step %d problems %v", m.i, m.problems)
	}
	keys(m, "c")
	vals := m.values()
	if !m.create || vals["title"] != "Export fails" {
		t.Fatalf("create %t values %v", m.create, vals)
	}
	if d := draftFromAnswers(ty, vals); !bytes.Contains([]byte(d.Body), []byte("### What happened or what you need\n\nExport of the claims list fails.\nIt times out.")) {
		t.Fatalf("body:\n%s", d.Body)
	}
}
