package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
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
	// The text report shows the fetched baseline and the next step.
	code, out, errOut := run(t, env, opts, "grill", "goal", "--repo", "o/r", "--title", "First claims without help", "--body", body,
		"--set", "integrations.github.secret=github-read", "--set", "integrations.github.scope=read", "--set", "integrations.github.project=o/r")
	if code != 0 || !strings.Contains(out, `goal draft "First claims without help": passes Create and Ready`) ||
		!strings.Contains(out, "  baseline: prs_merged = 37 over ") || !strings.Contains(out, "2026-09-14T00:00:00Z to 2026-10-12T00:00:00Z (UTC, half-open)") ||
		!strings.Contains(out, "run again with --create") {
		t.Fatalf("text report: exit %d\n%s%s", code, out, errOut)
	}

	// A snapshot metric (activation) comes from `aayu measure` exports: the
	// draft needs the measured value in Baseline. A metric neither snapshot
	// nor <source>:<metric> is refused, and a missing one is a Create problem.
	for _, tc := range []struct {
		metric, baseline string
		code             int
		want             string
	}{
		{"activation", "", ExitBlocked, "baseline: no reachable baseline for metric \"activation\" (a Target needs a reproducible baseline first): activation is measured from exports by `aayu measure`"},
		{"activation", "41% (snapshot 2026-10-01, all claimants)", 0, ""},
		{"median days to first decision", "21 days", ExitBlocked, `metric: unreadable measure spec: metric "median days to first decision": want one of`},
		{"", "", ExitBlocked, "metric:"},
	} {
		f := map[string]string{"summary": "Claimants finish their first claim without help.", "beneficiaries": "claimants", "metric": tc.metric, "baseline": tc.baseline, "target": ">= 45% by 2026-12-31"}
		for k, v := range f {
			if v == "" {
				delete(f, k)
			}
		}
		before := len(g.Issues)
		code, out, errOut := run(t, env, opts, "grill", "goal", "--repo", "o/r", "--json", "--title", "Activation", "--body", workitems.Render(mustType(t, "goal"), f))
		var res GrillResult
		_ = json.NewDecoder(strings.NewReader(out)).Decode(&res)
		if code != tc.code || res.Pass != (tc.code == 0) || !strings.Contains(problemsText(res), tc.want) || res.Baseline != nil || len(g.Issues) != before {
			t.Errorf("metric %q baseline %q: exit %d %+v %s; want %q", tc.metric, tc.baseline, code, res, errOut, tc.want)
		}
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
	if code, out, errOut := run(t, nil, opts, "grill", "brand", "--body", brand("#767676"), "--create", "--dry-run"); code != 0 || !strings.Contains(out, "--dry-run: design/brand.yaml is planned, not written") {
		t.Fatalf("create --dry-run: %d %s %s", code, out, errOut)
	}
	if _, err := os.Stat(filepath.Join(dir, "design", "brand.yaml")); !os.IsNotExist(err) {
		t.Fatal("written with --dry-run")
	}
	// The text report lists every pair and the problems that are not pair results.
	noTone := strings.Replace(brand("#777777"), "voice: {tone: [plain, warm]}\n", "", 1)
	code, out, errOut := run(t, nil, opts, "grill", "brand", "--body", noTone)
	if code != ExitBlocked || !strings.Contains(out, `brand "Benefits QA": does not pass yet`) || !strings.Contains(out, "primary (#777777) on surface (#ffffff) for text: 4.47:1 fails AA") ||
		!strings.Contains(out, "  problem: voice.tone: name at least one tone word") || strings.Contains(out, "problem: AA") {
		t.Fatalf("failing brand, text: %d\n%s%s", code, out, errOut)
	}
	if code, _, errOut := run(t, nil, Options{}, "grill", "brand", "--body", brand("#767676"), "--create"); code != ExitUsage || !strings.Contains(errOut, "runs in a checkout of the repository") {
		t.Fatalf("create outside a checkout: %d %s", code, errOut)
	}
	// Without --body and without a terminal (stdin a pipe or a file, or --json) there is nothing to check.
	if code, _, errOut := run(t, nil, opts, "grill", "brand"); code != ExitUsage || !strings.Contains(errOut, "grill brand needs --body") {
		t.Fatalf("no body, no terminal: %d %s", code, errOut)
	}
	tui := Options{Dir: dir, TUI: func(tea.Model) error { t.Error("--json started the interview"); return nil }}
	if code, _, errOut := run(t, nil, tui, "grill", "brand", "--json"); code != ExitUsage || !strings.Contains(errOut, "grill brand needs --body") {
		t.Fatalf("--json, no body: %d %s", code, errOut)
	}
	f, err := os.CreateTemp(t.TempDir(), "stdin")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	var stdout, stderr bytes.Buffer
	home := t.TempDir()
	if code := Run(t.Context(), []string{"grill", "brand"}, IO{In: f, Out: &stdout, Err: &stderr},
		Options{Dir: dir, Getenv: func(k string) string { return map[string]string{"HOME": home}[k] }}); code != ExitUsage || !strings.Contains(stderr.String(), "grill brand needs --body") {
		t.Fatalf("stdin from a file: %d %s", code, stderr.String())
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
		case "tab":
			msg = tea.KeyPressMsg{Code: tea.KeyTab}
		case "esc":
			msg = tea.KeyPressMsg{Code: tea.KeyEscape}
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
	if v := m.View().Content; !strings.Contains(v, "The draft passes validation.") || !strings.Contains(v, "[c] create") {
		t.Fatalf("passing review:\n%s", v)
	}
	// shift+tab goes back from the review and from a question; pasted text is
	// only ever an answer's text: on the review screen it is not a key.
	keys(m, "shift+tab", "shift+tab")
	if m.steps[m.i].ID != "links" {
		t.Fatalf("shift+tab twice from the review: %s", m.steps[m.i].ID)
	}
	m.Update(tea.PasteMsg{Content: "becomes #12"})
	keys(m, "tab", "tab")
	m.Update(tea.PasteMsg{Content: "c"})
	if got := m.values()["links"]; got != "becomes #12" || m.i != len(m.steps) || m.create {
		t.Fatalf("pasted links %q, step %d, create %t", got, m.i, m.create)
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

// scripted returns a TUI runner that types keys into the model, as a human
// on a terminal would, after bubbletea's Init.
func scripted(ks ...string) func(tea.Model) error {
	return func(tm tea.Model) error {
		if cmd := tm.Init(); cmd != nil {
			return errors.New("grill's model starts with a command")
		}
		keys(tm.(*grillModel), ks...)
		return nil
	}
}

// On a terminal, `grill brand` without --body interviews the human: a pair
// below AA is shown on the review screen and cannot be created; fixing the
// colour lets the human create design/brand.yaml.
func TestGrillBrandTUI(t *testing.T) {
	dir := gitInit(t, "dev", map[string]string{"README.md": "x\n"})
	answers := func(primary string) []string {
		return []string{"Benefits QA", "enter", "plain, warm", "enter", "enter",
			"ink #1a1a1a", "enter", "surface #ffffff", "enter", "primary " + primary, "enter", "enter",
			"ink on surface, text", "enter", "primary on surface, text", "enter", "enter"}
	}
	opts := Options{Dir: dir, TUI: scripted(append(answers("#777777"), "c", "q")...)}
	if code, _, errOut := run(t, nil, opts, "grill", "brand"); code != ExitBlocked || !strings.Contains(errOut, "ended without writing") {
		t.Fatalf("failing pair: %d %s", code, errOut)
	}
	m := &grillModel{name: "grill brand", steps: brandSteps(os.DirFS(filepath.Join("..", ".."))), check: brandCheck}
	keys(m, answers("#777777")...)
	if v := m.View().Content; !strings.Contains(v, "primary (#777777) on surface (#ffffff) for text: 4.47:1 fails AA") {
		t.Fatalf("review:\n%s", v)
	}
	// Pasted palette and pair lists: blank lines are skipped and a line that
	// does not parse is named on the review screen.
	m = &grillModel{name: "grill brand", steps: brandSteps(os.DirFS(filepath.Join("..", ".."))), check: brandCheck}
	keys(m, "Benefits QA", "enter", "plain", "enter", "enter")
	m.Update(tea.PasteMsg{Content: "ink #1a1a1a\n\nsurface: white\npaper #ffffff"})
	keys(m, "ctrl+d")
	m.Update(tea.PasteMsg{Content: "ink on paper, text\n\nink over paper"})
	keys(m, "ctrl+d")
	if v := m.View().Content; !strings.Contains(v, `palette: "surface: white": want `+"`name #rrggbb`") || !strings.Contains(v, `pairs: "ink over paper": want`) ||
		!strings.Contains(v, "2 problem(s)") || !strings.Contains(v, "ink: '#1a1a1a'") {
		t.Fatalf("review of pasted lists:\n%s", v)
	}
	opts.TUI = scripted(append(answers("#767676"), "c")...)
	if code, out, errOut := run(t, nil, opts, "grill", "brand"); code != 0 || !strings.Contains(out, "wrote design/brand.yaml") {
		t.Fatalf("create: %d %s %s", code, out, errOut)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "design", "brand.yaml")); !strings.Contains(string(b), "primary: '#767676'") && !strings.Contains(string(b), `primary: "#767676"`) {
		t.Fatalf("design/brand.yaml:\n%s", b)
	}
}

// On a terminal, `grill <type>` without --title and --body runs the same
// validation as the flag path and writes the issue when the human creates it.
func TestGrillTUICreates(t *testing.T) {
	g := fake.New(t, "o/r", "User")
	env := map[string]string{"AAYU_GITHUB_TOKEN": "human-token"}
	opts := Options{GitHubAPI: g.URL(), GitHubGap: -1,
		TUI: scripted("Export fails", "enter", "Copying claims by hand.", "enter", "enter", "ctrl+d", "ctrl+d", "enter", "c")}
	code, out, errOut := run(t, env, opts, "grill", "feedback", "--repo", "o/r")
	if code != 0 || !strings.Contains(out, "created #1") || !contains(g.LabelNames(1), "status:backlog") {
		t.Fatalf("grill feedback on a terminal: %d %s %s", code, out, errOut)
	}
}

// The text report of grill <type>: the verdict, every Create and Ready
// problem, and the next step: --create, the dry-run plan, or `aayu ready`
// on the created issue. A missing draft without a terminal and an unknown
// type are usage errors.
func TestGrillText(t *testing.T) {
	g := readyTree(t)
	env := map[string]string{"AAYU_GITHUB_TOKEN": "human-token"}
	opts := Options{GitHubAPI: g.URL(), GitHubGap: -1}
	body := fixtureBody(t, "story", nil)
	before := len(g.Issues)
	for _, tc := range []struct {
		args []string
		code int
		want []string
	}{
		{[]string{"--title", "Show empty list", "--body", body, "--parent", "1"}, ExitBlocked, []string{`story draft "Show empty list": does not pass yet`, "  ready: effort"}},
		{[]string{"--title", "Show empty list", "--body", "Just a sentence.", "--labels", "effort:S"}, ExitBlocked, []string{"  create: "}},
		{[]string{"--title", "Show empty list", "--body", body, "--parent", "99", "--labels", "effort:S"}, ExitBlocked, []string{"  ready: parent: #99: "}},
		{[]string{"--title", "Show empty list", "--body", body, "--parent", "1", "--labels", "effort:S"}, 0, []string{"passes Create and Ready", "run again with --create to write it as an issue"}},
		{[]string{"--title", "Show empty list", "--body", body, "--parent", "1", "--labels", "effort:S", "--create", "--dry-run"}, 0, []string{"--dry-run: the issue is planned, not written"}},
		{[]string{"--parent", "1"}, ExitUsage, nil},
	} {
		code, out, errOut := run(t, env, opts, append([]string{"grill", "story", "--repo", "o/r"}, tc.args...)...)
		if code != tc.code {
			t.Errorf("%v: exit %d, want %d\n%s%s", tc.args, code, tc.code, out, errOut)
		}
		for _, w := range tc.want {
			if !strings.Contains(out, w) {
				t.Errorf("%v: output lacks %q:\n%s", tc.args, w, out)
			}
		}
		if tc.code == ExitUsage && !strings.Contains(errOut, "grill story needs --title and --body") {
			t.Errorf("%v: %s", tc.args, errOut)
		}
	}
	if len(g.Issues) != before {
		t.Fatalf("%d issue(s) written without --create or with --dry-run", len(g.Issues)-before)
	}
	code, out, errOut := run(t, env, opts, "grill", "story", "--repo", "o/r", "--title", "Show empty list", "--body", body, "--parent", "1", "--labels", "effort:S", "--create")
	n := before + 1
	if code != 0 || !strings.Contains(out, fmt.Sprintf("created #%d ", n)) || !strings.Contains(out, fmt.Sprintf("; next: `aayu ready %d`", n)) {
		t.Fatalf("create: %d\n%s%s", code, out, errOut)
	}
	if code, _, errOut := run(t, env, opts, "grill", "epic", "--repo", "o/r", "--title", "x", "--body", "y"); code != ExitUsage || !strings.Contains(errOut, "epic") {
		t.Fatalf("unknown type: %d %s", code, errOut)
	}
}

// On a terminal, `grill story --parent 1` asks for the parent (prefilled) and
// the labels after the body: an unusable label and a missing effort are shown
// on the review screen, e jumps to the labels, and once the draft passes the
// story is created under its parent. esc quits without writing anything.
func TestGrillTUIStory(t *testing.T) {
	g := readyTree(t)
	env := map[string]string{"AAYU_GITHUB_TOKEN": "human-token"}
	fields := map[string]string{"title": "Show empty list"}
	b, _ := os.ReadFile(filepath.Join("..", "workitems", "testdata", "items", "story.json"))
	var items map[string]workitems.Item
	if err := json.Unmarshal(b, &items); err != nil {
		t.Fatal(err)
	}
	for k, v := range items["ready"].Fields {
		fields[k] = v
	}
	var parentView string
	var reviews []string
	var afterE []string
	backAt := ""
	answer := func(tm tea.Model) error {
		m := tm.(*grillModel)
		for m.i < len(m.steps) && m.steps[m.i].ID != "labels" {
			if m.steps[m.i].ID == "parent" {
				parentView = m.View().Content
			}
			m.Update(tea.PasteMsg{Content: fields[m.steps[m.i].ID]}) // each answer pasted, as from a ticket
			keys(m, "tab")
		}
		keys(m, "status:done", "tab")
		reviews = append(reviews, m.View().Content)
		keys(m, "e")
		afterE = append(afterE, m.steps[m.i].ID)
		for range len("status:done") {
			keys(m, "backspace")
		}
		keys(m, "priority:P2", "tab")
		reviews = append(reviews, m.View().Content)
		keys(m, "e")
		afterE = append(afterE, m.steps[m.i].ID)
		keys(m, ",effort:S", "tab")
		reviews = append(reviews, m.View().Content)
		keys(m, "shift+tab")
		backAt = m.steps[m.i].ID
		keys(m, "tab", "c")
		return nil
	}
	before := len(g.Issues)
	opts := Options{GitHubAPI: g.URL(), GitHubGap: -1, TUI: scripted("Show empty list", "esc")}
	if code, _, errOut := run(t, env, opts, "grill", "story", "--repo", "o/r", "--parent", "1"); code != ExitBlocked || !strings.Contains(errOut, "grill story: the interview ended without writing anything") || len(g.Issues) != before {
		t.Fatalf("esc: %d %s, %d issue(s) written", code, errOut, len(g.Issues)-before)
	}
	opts.TUI = answer
	code, out, errOut := run(t, env, opts, "grill", "story", "--repo", "o/r", "--parent", "1")
	if code != 0 || !strings.Contains(out, fmt.Sprintf("created #%d", before+1)) {
		t.Fatalf("grill story on a terminal: %d %s %s", code, out, errOut)
	}
	if !strings.Contains(parentView, "Parent (feature, issue number) (optional)") || !strings.Contains(parentView, "> 1") {
		t.Errorf("parent question:\n%s", parentView)
	}
	if len(reviews) != 3 || !strings.Contains(reviews[0], `labels: grill: label "status:done" is not a label-held field`) ||
		!strings.Contains(reviews[1], "effort:") || !strings.Contains(reviews[2], "The draft passes validation.") {
		t.Errorf("reviews:\n%s", strings.Join(reviews, "\n----\n"))
	}
	if strings.Join(afterE, ",") != "labels,labels" || backAt != "labels" {
		t.Errorf("e jumped to %v; shift+tab went back to %s", afterE, backAt)
	}
	g.Mu.Lock()
	created := g.Issues[before+1]
	g.Mu.Unlock()
	if created.Parent != 1 || !contains(g.LabelNames(before+1), "effort:S") || !contains(g.LabelNames(before+1), "priority:P2") {
		t.Fatalf("created story: parent %d labels %v", created.Parent, g.LabelNames(before+1))
	}
}

// editWorld is Story #1179's repository: #1 a Story, #2 an Incident, #3 Bug
// #1169's shape (status:planned, its Links name the Story, so it fails
// Ready), and #4 a Ready Bug. The checkout keeps one actions log.
func editWorld(t *testing.T) (*fake.GitHub, map[string]string, Options) {
	t.Helper()
	g := fake.New(t, "o/r", "User")
	g.AddIssue("Show empty list", fixtureBody(t, "story", nil), "type:story", "status:planned", "effort:S")
	g.AddIssue("Checkout down", fixtureBody(t, "incident", nil), "type:incident")
	g.AddIssue("Questionnaire loses answers on back", bugBody(t, "caused-by #1"), "type:bug", "status:planned", "effort:XS")
	g.AddIssue("Ready bug", bugBody(t, ""), "type:bug", "status:ready", "effort:XS")
	dir, _ := ledgerCheckout(t)
	return g, map[string]string{"AAYU_GITHUB_TOKEN": "human-token"}, Options{GitHubAPI: g.URL(), GitHubGap: -1, Dir: dir}
}

// bugBody is the Ready bug fixture with links as its Links section.
func bugBody(t *testing.T, links string) string {
	return fixtureBody(t, "bug", func(f map[string]string) {
		if links != "" {
			f[workitems.FieldLinks] = links
		}
	})
}

func issueBody(g *fake.GitHub, n int) (string, string) {
	g.Mu.Lock()
	defer g.Mu.Unlock()
	return g.Issues[n].Title, g.Issues[n].Body
}

// Story #1179: grill <type> --edit checks a filed issue's new body as it
// checks a draft and writes it, in one write, only before Ready.
func TestGrillEdit(t *testing.T) {
	// Criterion 1: Bug #3's Links fail Ready; grill checks the new body,
	// writes it through the actions log, and prints the diff.
	t.Run("writes a checked body", func(t *testing.T) {
		g, env, opts := editWorld(t)
		if code, out, _ := run(t, env, opts, "ready", "3", "--repo", "o/r", "--check"); code != ExitBlocked || !strings.Contains(out, "caused-by #1: #1 is a story") {
			t.Fatalf("ready --check before the edit: exit %d\n%s", code, out)
		}
		g.Reset()
		fixed := bugBody(t, "caused-by #2")
		code, out, errOut := run(t, env, opts, "grill", "bug", "--repo", "o/r", "--edit", "3", "--body", fixed)
		if code != 0 || !strings.Contains(out, `#3 "Questionnaire loses answers on back": edited`) ||
			!strings.Contains(out, "  -caused-by #1\n") || !strings.Contains(out, "  +caused-by #2\n") || strings.Contains(out, "ready:") {
			t.Fatalf("grill bug --edit 3: exit %d\n%s%s", code, out, errOut)
		}
		if _, body := issueBody(g, 3); body != fixed {
			t.Fatalf("#3's body:\n%s", body)
		}
		if w := g.Writes(); len(w) != 1 || w[0].Method != http.MethodPatch || w[0].Path != "/repos/o/r/issues/3" {
			t.Fatalf("writes: %+v", w)
		}
		if code, out, _ := run(t, env, opts, "actions", "grep", "github.issue.edit"); code != 0 || !strings.Contains(out, `"target":"o/r#3"`) || !strings.Contains(out, `"status":"done"`) {
			t.Fatalf("actions log:\n%s", out)
		}
		if code, out, _ := run(t, env, opts, "ready", "3", "--repo", "o/r", "--check"); code != 0 {
			t.Fatalf("ready --check after the edit: exit %d\n%s", code, out)
		}
	})

	// Criterion 2: a body that fails Create writes nothing and lists each problem.
	t.Run("fails Create", func(t *testing.T) {
		g, env, opts := editWorld(t)
		_, before := issueBody(g, 3)
		g.Reset()
		bad := fixtureBody(t, "bug", func(f map[string]string) { delete(f, "reproduce"); delete(f, "actual") })
		code, out, errOut := run(t, env, opts, "grill", "bug", "--repo", "o/r", "--edit", "3", "--body", bad)
		if code != ExitBlocked || !strings.Contains(out, "the new body does not pass Create; nothing was written") ||
			!strings.Contains(out, "  create: reproduce: Steps to reproduce is required at Create\n") || !strings.Contains(out, "  create: actual: Actual behaviour is required at Create\n") ||
			!strings.Contains(errOut, "the new body of #3 has 2 Create problem(s); nothing was written") {
			t.Fatalf("failing body: exit %d\n%s%s", code, out, errOut)
		}
		if _, after := issueBody(g, 3); after != before || len(g.Writes()) != 0 {
			t.Fatalf("a failing body was written: %+v", g.Writes())
		}
	})

	// Criterion 3: a Ready issue's body is frozen.
	t.Run("refuses a Ready issue", func(t *testing.T) {
		g, env, opts := editWorld(t)
		g.Reset()
		code, out, errOut := run(t, env, opts, "grill", "bug", "--repo", "o/r", "--edit", "4", "--body", bugBody(t, "caused-by #2"), "--title", "New title")
		if code != ExitBlocked || !strings.Contains(errOut, "#4 is Ready; its body is frozen") || len(g.Writes()) != 0 {
			t.Fatalf("Ready #4: exit %d, writes %+v\n%s%s", code, g.Writes(), out, errOut)
		}
		// A status past Ready is frozen too, and the type must match.
		g.Mu.Lock()
		g.Issues[4].Labels = []fake.Label{{Name: "type:bug"}, {Name: "status:in-progress"}}
		g.Mu.Unlock()
		if code, _, errOut := run(t, env, opts, "grill", "bug", "--repo", "o/r", "--edit", "4", "--body", bugBody(t, "")); code != ExitBlocked || !strings.Contains(errOut, "#4 is status:in-progress, past Ready; its body is frozen") {
			t.Fatalf("in-progress #4: exit %d %s", code, errOut)
		}
		if code, _, errOut := run(t, env, opts, "grill", "story", "--repo", "o/r", "--edit", "3", "--body", bugBody(t, "")); code != ExitUsage || !strings.Contains(errOut, "#3 has type bug, not story") {
			t.Fatalf("wrong type: exit %d %s", code, errOut)
		}
		if len(g.Writes()) != 0 {
			t.Fatalf("writes: %+v", g.Writes())
		}
	})

	// Criterion 5: --title and the body go in one write.
	t.Run("title and body in one write", func(t *testing.T) {
		g, env, opts := editWorld(t)
		g.Reset()
		fixed := bugBody(t, "caused-by #2")
		code, out, errOut := run(t, env, opts, "grill", "bug", "--repo", "o/r", "--json", "--edit", "3", "--body", fixed, "--title", "Back clears the answers")
		var res GrillResult
		if code != 0 || json.Unmarshal([]byte(out), &res) != nil || !res.Edited || res.Issue != 3 || !strings.Contains(res.Diff, "-# Questionnaire loses answers on back\n+# Back clears the answers\n") {
			t.Fatalf("--title with --edit: exit %d\n%s%s", code, out, errOut)
		}
		w := g.Writes()
		if len(w) != 1 || w[0].Path != "/repos/o/r/issues/3" || w[0].Body["title"] != "Back clears the answers" || w[0].Body["body"] != fixed {
			t.Fatalf("writes: %+v", w)
		}
		if title, body := issueBody(g, 3); title != "Back clears the answers" || body != fixed {
			t.Fatalf("#3: %q\n%s", title, body)
		}
	})
}

// grill --edit's other paths: a dry run writes nothing; a body that passes
// Create but not Ready is written and its Ready problems are named; --edit
// does not mix with a new draft's flags or with brand.
func TestGrillEditPlanAndUsage(t *testing.T) {
	g, env, opts := editWorld(t)
	g.Reset()
	code, out, errOut := run(t, env, opts, "grill", "bug", "--repo", "o/r", "--dry-run", "--edit", "3", "--body", bugBody(t, "caused-by #2"))
	if code != 0 || !strings.Contains(out, "--dry-run: the edit is planned, not written") || len(g.Writes()) != 0 {
		t.Fatalf("--dry-run: exit %d, writes %+v\n%s%s", code, g.Writes(), out, errOut)
	}
	still := bugBody(t, "caused-by #1\ndepends-on #2")
	code, out, errOut = run(t, env, opts, "grill", "bug", "--repo", "o/r", "--edit", "3", "--body", still)
	if _, body := issueBody(g, 3); code != 0 || body != still || !strings.Contains(out, "  ready: links: caused-by #1: #1 is a story; caused-by links to an incident\n") ||
		!strings.Contains(out, "next: fix the ready problems, then `aayu ready 3`") {
		t.Fatalf("a body that passes Create only: exit %d\n%s%s", code, out, errOut)
	}
	g.Reset()
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"bug", "--edit", "3", "--body", still, "--create"}, "--create, --assess, --parent, and --labels go with a new draft"},
		{[]string{"bug", "--edit", "3", "--body", still, "--labels", "effort:S"}, "--create, --assess, --parent, and --labels go with a new draft"},
		{[]string{"bug", "--edit", "3"}, "grill bug --edit 3 needs --body: the whole new body"},
		{[]string{"bug", "--edit", "99999999999999999999", "--body", still}, `grill --edit "99999999999999999999": want an issue number`},
		{[]string{"bug", "--edit", "https://github.com/x/y/issues/3", "--body", still}, "the issue is in x/y, and grill edits o/r; nothing was read or written"},
		{[]string{"brand", "--edit", "3", "--body", "version: 1"}, "grill brand has no --edit"},
	} {
		if code, _, errOut := run(t, env, opts, append([]string{"grill", "--repo", "o/r"}, tc.args...)...); code != ExitUsage || !strings.Contains(errOut, tc.want) {
			t.Errorf("%v: exit %d %s", tc.args, code, errOut)
		}
	}
	if len(g.Writes()) != 0 {
		t.Fatalf("writes: %+v", g.Writes())
	}
}

// Story #1179 criterion 4: a running mission's lock refuses grill --edit of
// its open Bug #27, even before Ready, naming the mission; nothing reaches GitHub.
func TestGrillEditLocked(t *testing.T) {
	m := startedLock(t)
	m.status(27, "planned", "")
	_, before := issueBody(m.g, 27)
	m.g.Reset()
	body := fixtureBody(t, "bug", func(f map[string]string) { delete(f, workitems.FieldLinks); f["summary"] = "Answers vanish on Back." })
	code, out, errOut := run(t, m.agent, m.opts, "grill", "bug", "--repo", "o/r", "--edit", "27", "--body", body)
	if code != ExitBlocked || !strings.Contains(errOut, "#27 is locked by mission #889 until it ends") {
		t.Fatalf("locked #27: exit %d\n%s%s", code, out, errOut)
	}
	if _, after := issueBody(m.g, 27); after != before || len(m.issueWrites(27)) != 0 {
		t.Fatalf("a locked body was written: %+v", m.issueWrites(27))
	}
}

// Story #1179 criterion 6: grill reads link targets as Ready does, so a Bug
// draft caused-by a Story gets Ready's answer before it is filed.
func TestGrillLinkTargetsAsReady(t *testing.T) {
	g, env, opts := editWorld(t)
	g.Reset()
	const want = "links: caused-by #1: #1 is a story; caused-by links to an incident"
	code, out, errOut := run(t, env, opts, "grill", "bug", "--repo", "o/r", "--dry-run", "--title", "Back clears answers", "--body", bugBody(t, "caused-by #1"), "--labels", "effort:XS")
	if code != ExitBlocked || !strings.Contains(out, "  ready: "+want+"\n") {
		t.Fatalf("grill bug --dry-run: exit %d\n%s%s", code, out, errOut)
	}
	if code, out, _ := run(t, env, opts, "ready", "3", "--repo", "o/r", "--check"); code != ExitBlocked || !strings.Contains(out, "  - "+want+"\n") {
		t.Fatalf("ready 3 --check: exit %d\n%s", code, out)
	}
	if code, out, errOut := run(t, env, opts, "grill", "bug", "--repo", "o/r", "--dry-run", "--title", "Back clears answers", "--body", bugBody(t, "caused-by #2"), "--labels", "effort:XS"); code != 0 {
		t.Fatalf("caused-by the Incident: exit %d\n%s%s", code, out, errOut)
	}
	if code, out, _ := run(t, env, opts, "grill", "bug", "--repo", "o/r", "--title", "Back clears answers", "--body", bugBody(t, "caused-by #99"), "--labels", "effort:XS"); code != ExitBlocked || !strings.Contains(out, "  ready: links: caused-by #99: no such issue\n") {
		t.Fatalf("caused-by no issue: exit %d\n%s", code, out)
	}
	if len(g.Writes()) != 0 {
		t.Fatalf("writes: %+v", g.Writes())
	}
}

// Story #1179 criterion 7: a Feature's Expected paths on one line, separated
// by commas, read as one path per comma, the same in grill and slice.
func TestExpectedPathsCommas(t *testing.T) {
	g := fake.New(t, "o/r", "User")
	body := fixtureBody(t, "feature", func(f map[string]string) {
		delete(f, workitems.FieldLinks)
		f["expected_paths"] = "`internal/cli/**`, internal/planning/**,docs/guide/**"
	})
	g.AddIssue("Eligibility questionnaire", body, "type:feature", "status:planned")
	env := map[string]string{"AAYU_GITHUB_TOKEN": "human-token"}
	opts := Options{GitHubAPI: g.URL(), GitHubGap: -1}
	want := []string{"internal/cli/**", "internal/planning/**", "docs/guide/**"}
	code, out, errOut := run(t, env, opts, "grill", "feature", "--repo", "o/r", "--dry-run", "--json", "--title", "Eligibility questionnaire", "--body", body)
	var gr GrillResult
	if json.NewDecoder(strings.NewReader(out)).Decode(&gr) != nil || !slices.Equal(gr.ExpectedPaths, want) {
		t.Fatalf("grill feature --dry-run: exit %d paths %q\n%s%s", code, gr.ExpectedPaths, out, errOut)
	}
	if _, text, _ := run(t, env, opts, "grill", "feature", "--repo", "o/r", "--dry-run", "--title", "Eligibility questionnaire", "--body", body); !strings.Contains(text, "  expected paths: internal/cli/**, internal/planning/**, docs/guide/**\n") {
		t.Fatalf("grill feature text:\n%s", text)
	}
	code, out, errOut = run(t, env, opts, "slice", "1", "--repo", "o/r", "--json")
	var sr SliceResult
	if code != 0 || json.Unmarshal([]byte(out), &sr) != nil || !slices.Equal(sr.ExpectedPaths, want) {
		t.Fatalf("slice 1: exit %d paths %q\n%s%s", code, sr.ExpectedPaths, out, errOut)
	}
}
