package cli

import (
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	aayudha "github.com/rahulnakmol/agentic-ayudha"
	"github.com/rahulnakmol/agentic-ayudha/internal/decide"
	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
	"github.com/rahulnakmol/agentic-ayudha/internal/notify"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

func decisionBody(reversible, links string) string {
	t, _ := workitems.FindType("decision")
	return workitems.Render(t, map[string]string{
		workitems.FieldSummary:    "Which rules engine?",
		"options":                 "1. **A.** in-house.\n2. **B.** a vendor engine.",
		"evaluation":              "A is cheaper to run; B is faster to build.",
		"recommendation":          "A",
		workitems.FieldReversible: reversible,
		workitems.FieldLinks:      links,
	})
}

// decideTree: Feature #1 holding Decisions #2 (reversible), #3 (irreversible),
// #4 (a floor waiver whose body was edited to say reversible), #5 (supersedes
// #3), and Story #6. Every Decision opened Monday 12 October 10:00 London.
func decideTree(t *testing.T) *fake.GitHub {
	g := fake.New(t, "o/r", "User")
	f := g.AddIssue("Decision engine", "", "type:feature", "status:in-progress")
	add := func(title, body string, labels ...string) *fake.Issue {
		is := g.AddIssue(title, body, labels...)
		f.Subs = append(f.Subs, is.Number)
		is.Parent = f.Number
		return is
	}
	add("Choose the rules engine", decisionBody("true", "constrains #6"), "type:decision", "status:ready")
	add("Choose the data store", decisionBody("false", ""), "type:decision", "status:ready")
	add("Waive the coverage floor on #6", decisionBody("true", "waives #6"), "type:decision", "status:planned", workitems.LabelWaiver)
	add("Choose the data store, again", decisionBody("false", "supersedes #3"), "type:decision", "status:ready")
	add("Rule 1", "", "type:story", "status:ready")
	return g
}

// WP5.6 accept, for a team of two humans (Story #432 shortens the window
// only in solo mode): a reversible Decision fixture at 48 business hours gains
// decision:provisional with the escalation comment mentioning founder B; an
// irreversible one does not. Opened Monday 10:00 London, 24 business hours
// lapse Wednesday 16:00 and 48 lapse the next Monday 13:00 (12:00 UTC).
func TestDecideSLA(t *testing.T) {
	g := decideTree(t)
	sink := &notify.Fake{}
	now := time.Date(2026, 10, 19, 11, 59, 0, 0, time.UTC)
	env := map[string]string{"AAYU_GITHUB_TOKEN": "t", "XDG_STATE_HOME": t.TempDir()}
	opts := Options{IsTerminal: stdinTTY, GitHubAPI: g.URL(), GitHubGap: -1, Now: func() time.Time { return now }, NotifySink: sink}
	sweep := func() []decisionStep {
		t.Helper()
		code, out, errOut := run(t, env, opts, "decide", "--repo", "o/r", "--json", "--set", "decisions.escalate_to=founder-b", "--set", "autonomy.approver=founder-a", "--set", "seats.code-architect.mode=paired", "--set", "seats.code-architect.human=arch")
		if code != 0 {
			t.Fatalf("decide sweep: exit %d: %s", code, errOut)
		}
		var rep decideReport
		if err := json.Unmarshal([]byte(out), &rep); err != nil {
			t.Fatal(err)
		}
		return rep.Decisions
	}
	labels := func(n int) []string { return g.LabelNames(n) }

	// One minute short of 48 business hours: past the SLA, so every Decision
	// pages and escalates to founder B, but nothing applies yet.
	steps := sweep()
	if len(steps) != 4 || !steps[0].Lapsed || steps[0].Overdue || steps[0].SLA.UTC().Format(time.RFC3339) != "2026-10-14T15:00:00Z" {
		t.Fatalf("steps at 47:59: %+v", steps)
	}
	if slices.Contains(labels(2), workitems.LabelProvisional) {
		t.Fatal("#2 provisional before 48 business hours")
	}
	g.Mu.Lock()
	for _, n := range []int{2, 3, 4, 5} {
		if len(g.Issues[n].Comments) != 1 || !strings.Contains(g.Issues[n].Comments[0].Body, "@founder-b") {
			t.Errorf("#%d escalation comments: %+v", n, g.Issues[n].Comments)
		}
	}
	g.Mu.Unlock()
	if len(sink.Pages) != 4 || !strings.HasPrefix(sink.Pages[0].Key, "decision-sla:o/r#2") {
		t.Fatalf("pages at 47:59: %+v", sink.Pages)
	}

	// At 48 business hours.
	now = time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	steps = sweep()
	byN := map[int]decisionStep{}
	for _, s := range steps {
		byN[s.Number] = s
	}
	if s := byN[2]; !s.Apply || s.RatifyBy != "2026-10-22T09:00:00Z" { // Monday 13:00 + 24 business hours = Thursday 10:00 London
		t.Errorf("#2 at 48 h: %+v", s)
	}
	if !slices.Contains(labels(2), workitems.LabelProvisional) || !slices.Contains(labels(2), "status:validation") || slices.Contains(labels(2), "status:ready") {
		t.Errorf("#2 labels: %v", labels(2))
	}
	g.Mu.Lock()
	body2 := g.Issues[2].Body
	g.Mu.Unlock()
	if by, ok := decide.RatifyBy(body2); !ok || by.Format(time.RFC3339) != "2026-10-22T09:00:00Z" || !strings.Contains(body2, "### Decision") {
		t.Errorf("#2 body does not carry the ratify-or-revert deadline:\n%s", body2)
	}
	for _, n := range []int{3, 4, 5} {
		if s := byN[n]; s.Apply || !s.Overdue || slices.Contains(labels(n), workitems.LabelProvisional) {
			t.Errorf("#%d at 48 h: %+v, labels %v", n, s, labels(n))
		}
	}
	if byN[4].Reason != "a waiver is never applied provisionally" {
		t.Errorf("#4 reason: %q", byN[4].Reason)
	}
	// The irreversible Decision re-pages as overdue.
	if !slices.ContainsFunc(sink.Pages, func(p notify.Page) bool { return p.Key == "decision-overdue:o/r#3" }) {
		t.Errorf("no overdue page for #3: %+v", sink.Pages)
	}

	// The text report names each Decision's state and what it blocks.
	if code, out, _ := run(t, env, opts, "decide", "--repo", "o/r", "--set", "decisions.escalate_to=founder-b", "--set", "autonomy.approver=founder-a", "--set", "seats.code-architect.mode=paired", "--set", "seats.code-architect.human=arch"); code != 0 || !strings.Contains(out, `#2 "Choose the rules engine": provisional; ratify or revert by 2026-10-22T09:00:00Z`) ||
		!strings.Contains(out, "blocks #6 while open") || !strings.Contains(out, "overdue (reversible: false; the lapse only re-pages)") {
		t.Errorf("text report: exit %d:\n%s", code, out)
	}
	for _, args := range [][]string{{"decide", "--waiver", "coverage"}, {"decide", "--accept", "A"}, {"decide", "2", "--accept", "A", "--waiver", "coverage"}} {
		if code, _, _ := run(t, env, opts, append(args, "--repo", "o/r")...); code != ExitUsage {
			t.Errorf("%v: exit %d, want %d", args, code, ExitUsage)
		}
	}

	// A rerun the same day writes nothing and pages nobody.
	g.Reset()
	pages := len(sink.Pages)
	sweep()
	if w := g.Writes(); len(w) != 0 || len(sink.Pages) != pages {
		t.Errorf("rerun wrote %v and paged %d more", w, len(sink.Pages)-pages)
	}

	// status shows #6 blocked by the open Decision #2 that constrains it, and #2's hours left.
	code, out, _ := run(t, env, opts, "status", "1", "--repo", "o/r", "--json")
	var st statusReport
	_ = json.Unmarshal([]byte(out), &st)
	if code != 0 || len(st.Roots) != 1 || !slices.Equal(st.Roots[0].Blocked, []int{6}) || st.Roots[0].ProvisionalHours != "#2 24 h" {
		t.Errorf("status: exit %d: %s", code, out)
	}

	// Acceptance: human only, by the authority, an option the Decision lists.
	if code, _, _ := run(t, map[string]string{"AAYU_GITHUB_TOKEN": "t", "AAYU_AGENT": "1", "XDG_STATE_HOME": env["XDG_STATE_HOME"]}, opts, "decide", "2", "--repo", "o/r", "--accept", "A"); code != ExitBlocked {
		t.Errorf("agent accept: exit %d, want %d", code, ExitBlocked)
	}
	if code, _, _ := run(t, env, opts, "decide", "2", "--repo", "o/r", "--accept", "A", "--set", "autonomy.approver=someone-else"); code != ExitBlocked {
		t.Errorf("accept by a human who is not the authority: exit %d, want %d", code, ExitBlocked)
	}
	if code, _, _ := run(t, env, opts, "decide", "2", "--repo", "o/r", "--accept", "C"); code != ExitUsage {
		t.Errorf("accept of an option the Decision does not list: exit %d, want %d", code, ExitUsage)
	}
	if code, out, errOut := run(t, env, opts, "decide", "2", "--repo", "o/r", "--accept", "A"); code != 0 || !strings.Contains(out, "#2 accepted: A, by @o") {
		t.Fatalf("accept: exit %d: %s %s", code, out, errOut)
	}
	g.Mu.Lock()
	if is := g.Issues[2]; is.State != "closed" || !strings.Contains(is.Body, "A, accepted by @o on 2026-10-19. This ratifies or replaces the provisional application.") {
		t.Errorf("#2 after accept: %s\n%s", is.State, is.Body)
	}
	g.Mu.Unlock()
	if l := labels(2); slices.Contains(l, workitems.LabelProvisional) || !slices.Contains(l, "status:done") {
		t.Errorf("#2 labels after ratification: %v", l)
	}
	// Supersede chain: accepting #5 closes #3 as not planned.
	if code, _, errOut := run(t, env, opts, "decide", "5", "--repo", "o/r", "--accept", "B"); code != 0 {
		t.Fatalf("accept #5: exit %d: %s", code, errOut)
	}
	g.Mu.Lock()
	defer g.Mu.Unlock()
	if is := g.Issues[3]; is.State != "closed" || !slices.ContainsFunc(is.Comments, func(c *fake.Comment) bool { return strings.Contains(c.Body, "Superseded by #5") }) {
		t.Errorf("#3 after #5 superseded it: %s %+v", is.State, is.Comments)
	}
}

func TestDecideWaiver(t *testing.T) {
	g := decideTree(t)
	env := map[string]string{"AAYU_GITHUB_TOKEN": "t"}
	opts := Options{IsTerminal: stdinTTY, GitHubAPI: g.URL(), GitHubGap: -1}
	if code, _, _ := run(t, env, opts, "decide", "6", "--repo", "o/r", "--waiver", "redaction"); code != ExitBlocked {
		t.Errorf("redaction waiver: exit %d, want %d", code, ExitBlocked)
	}
	if code, _, _ := run(t, env, opts, "decide", "2", "--repo", "o/r", "--waiver", "coverage"); code != ExitUsage {
		t.Errorf("waiver on a Decision: exit %d, want %d", code, ExitUsage)
	}
	code, out, errOut := run(t, env, opts, "decide", "6", "--repo", "o/r", "--waiver", "coverage", "--json")
	if code != 0 {
		t.Fatalf("waiver: exit %d: %s", code, errOut)
	}
	var rep decideReport
	_ = json.Unmarshal([]byte(out), &rep)
	n := rep.Waiver.Decision
	g.Mu.Lock()
	is := g.Issues[n]
	g.Mu.Unlock()
	it, _ := workitems.Parse(n, is.Title, is.Body, "", g.LabelNames(n))
	if ps := workitems.Validate(it, workitems.StageReady); len(ps) > 0 || it.Fields[workitems.FieldReversible] != "false" || it.Fields[workitems.FieldLinks] != "waives #6" ||
		!slices.Contains(g.LabelNames(n), workitems.LabelWaiver) || is.Parent != 6 {
		t.Errorf("waiver #%d: problems %v, fields %v, labels %v, parent %d", n, ps, it.Fields, g.LabelNames(n), is.Parent)
	}
	// Filing it again finds the open one.
	if code, out, _ := run(t, env, opts, "decide", "6", "--repo", "o/r", "--waiver", "coverage"); code != 0 || !strings.Contains(out, "already open") {
		t.Errorf("second waiver: exit %d: %s", code, out)
	}
}

// A GitHub failure at any step of decide stops it with exit 5 (external).
func TestDecideGitHubFailures(t *testing.T) {
	at48 := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		args []string
		fail []string
	}{
		{[]string{"decide"}, []string{"GET /repos/o/r/issues", "GET /repos/o/r/issues/2/parent", "GET /repos/o/r/issues/2/comments", "POST /repos/o/r/issues/2/comments",
			"DELETE /repos/o/r/issues/2/labels/status:ready", "POST /repos/o/r/issues/2/labels", "PATCH /repos/o/r/issues/2"}},
		{[]string{"decide", "3"}, []string{"GET /repos/o/r/issues/3"}},
		{[]string{"decide", "5", "--accept", "B"}, []string{"GET /user", "GET /repos/o/r/issues/5", "PATCH /repos/o/r/issues/5", "POST /repos/o/r/issues/5/labels",
			"GET /repos/o/r/issues/3", "GET /repos/o/r/issues/3/comments", "POST /repos/o/r/issues/3/comments", "DELETE /repos/o/r/issues/3/labels/status:ready"}},
		{[]string{"decide", "6", "--waiver", "coverage"}, []string{"GET /repos/o/r/issues/6", "GET /repos/o/r/issues/6/sub_issues", "POST /repos/o/r/issues", "POST /repos/o/r/issues/6/sub_issues"}},
	}
	for _, tt := range tests {
		for _, f := range tt.fail {
			g := decideTree(t)
			g.Fail = f
			env := map[string]string{"AAYU_GITHUB_TOKEN": "t", "XDG_STATE_HOME": t.TempDir()}
			opts := Options{IsTerminal: stdinTTY, GitHubAPI: g.URL(), GitHubGap: -1, Now: func() time.Time { return at48 }, NotifySink: &notify.Fake{}}
			if code, _, errOut := run(t, env, opts, append(tt.args, "--repo", "o/r")...); code != ExitExternal {
				t.Errorf("%v with %s failing: exit %d, want %d: %s", tt.args, f, code, ExitExternal, errOut)
			}
		}
	}
}

// The remaining refusals of decide, each before any write.
func TestDecideRefusals(t *testing.T) {
	for _, tt := range []struct {
		name   string
		want   int
		args   []string
		mutate func(g *fake.GitHub)
	}{
		{"a story is not a Decision", ExitUsage, []string{"decide", "6"}, nil},
		{"accepting a closed Decision", ExitBlocked, []string{"decide", "3", "--accept", "A"}, func(g *fake.GitHub) { g.Issues[3].State = "closed" }},
		{"accepting a Decision that is not Decision-ready", ExitBlocked, []string{"decide", "3", "--accept", "A"}, func(g *fake.GitHub) { g.Issues[3].Body = "### Question\n\nWhich?" }},
		{"a waiver on a closed story", ExitBlocked, []string{"decide", "6", "--waiver", "deps"}, func(g *fake.GitHub) { g.Issues[6].State = "closed" }},
		{"a floor that does not exist", ExitUsage, []string{"decide", "6", "--waiver", "vibes"}, nil},
		{"an invalid time zone", ExitUsage, []string{"decide", "--set", "time.tz=Mars/Olympus"}, nil},
	} {
		g := decideTree(t)
		if tt.mutate != nil {
			tt.mutate(g)
		}
		code, _, errOut := run(t, map[string]string{"AAYU_GITHUB_TOKEN": "t"}, Options{IsTerminal: stdinTTY, GitHubAPI: g.URL(), GitHubGap: -1}, append(tt.args, "--repo", "o/r")...)
		if code != tt.want || len(g.Writes()) != 0 {
			t.Errorf("%s: exit %d (want %d), writes %v: %s", tt.name, code, tt.want, g.Writes(), errOut)
		}
	}
	g := decideTree(t)
	if code, out, _ := run(t, map[string]string{"AAYU_GITHUB_TOKEN": "t"}, Options{IsTerminal: stdinTTY, GitHubAPI: g.URL(), GitHubGap: -1}, "decide", "6", "--waiver", "deps", "--dry-run", "--repo", "o/r"); code != 0 || !strings.Contains(out, "planned (dry run)") || len(g.Writes()) != 0 {
		t.Errorf("dry-run waiver: exit %d: %s", code, out)
	}
	if code, _, _ := run(t, map[string]string{"AAYU_GITHUB_TOKEN": "t"}, Options{IsTerminal: stdinTTY, GitHubAPI: g.URL(), GitHubGap: -1}, "decide"); code != ExitUsage {
		t.Errorf("no --repo outside a checkout: exit %d, want %d", code, ExitUsage)
	}
}

// TestDecidePack (WP5.1b, D-5.6b): `decide --pack` is read-only and on demand. It routes an issue or a free question
// to the owning seat's twin, grounds the facts it finds (the issue, its parent and links, open Decisions), prints the
// twin's persona and the pack outline, and --draft checks a pack for every part and cited claims. Nothing is written.
func TestDecidePack(t *testing.T) {
	g := decideTree(t)
	env := map[string]string{"AAYU_GITHUB_TOKEN": "t", "XDG_STATE_HOME": t.TempDir()}
	opts := Options{IsTerminal: stdinTTY, GitHubAPI: g.URL(), GitHubGap: -1, Assets: aayudha.Assets}
	pack := func(want int, args ...string) packReport {
		t.Helper()
		code, out, errOut := run(t, env, opts, append([]string{"decide", "--repo", "o/r", "--json", "--pack"}, args...)...)
		if code != want {
			t.Fatalf("decide --pack %v: exit %d, want %d: %s", args, code, want, errOut)
		}
		var rep packReport
		if err := json.NewDecoder(strings.NewReader(out)).Decode(&rep); err != nil { // the report comes before any error
			t.Fatalf("decide --pack %v: %v: %s", args, err, out)
		}
		return rep
	}
	rep := pack(0, "2")
	facts := ""
	for _, f := range rep.Facts {
		facts += f.Text + " (" + f.Cite + ")\n"
	}
	for _, want := range []string{`#2 "Choose the rules engine": decision, open`, `Parent: #1 "Decision engine": feature`, `Linked (constrains): #6 "Rule 1": story`, `Open Decision: #3 "Choose the data store"`} {
		if !strings.Contains(facts, want) {
			t.Errorf("facts lack %q:\n%s", want, facts)
		}
	}
	if rep.Seat != "engineering-manager" || rep.Twin != "Drona" || rep.Stage != "startup" || rep.Question != "Which rules engine?" ||
		!strings.Contains(rep.Persona, "Speak and work as Drona") || !strings.Contains(rep.Untrusted, "1. **A.** in-house.") || len(rep.Sections) != 8 {
		t.Errorf("pack for #2: %+v", rep)
	}
	q := pack(0, "Should we add a field to the Tombstone schema or a separate index?")
	if q.Seat != "code-architect" || q.Twin != "Vishwa" || q.Issue != 0 || !strings.Contains(q.SeatWhy, `"field"`) {
		t.Errorf("pack for a question: %+v", q)
	}
	if o := pack(0, "2", "--seat", "quality-architect"); o.Twin != "Yama" || o.SeatWhy != "--seat" {
		t.Errorf("--seat: %+v", o)
	}

	dir := t.TempDir()
	good := `# Decision pack

## Question

Which rules engine do we build #6 on?

## Facts

- #2 recommends A, and #6 is blocked while #2 is open.

## Options

1. In-house (#2): cheaper to run, slower to build.
2. Vendor (#2): faster to build, a contract to manage.

## Recommendation

In-house, as #2 recommends; a two-way door until #6 ships.

## Cost of waiting

#6 stays blocked (#6).

## Pre-mortem

Sanjay: in-house failed because the rules outgrew it; early warning: #6 needs a rule type #2 did not list.

## Questions that would change the answer

- Does #6 need rules a vendor already has?

## Your decision

Reply A or B on #2.
`
	write := func(name, s string) string {
		p := dir + "/" + name
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	if r := pack(0, "2", "--draft", write("good.md", good)); len(r.Findings) != 0 {
		t.Errorf("good draft: %v", r.Findings)
	}
	bad := strings.Replace(good, "## Pre-mortem", "## Notes", 1)
	if r := pack(4, "2", "--draft", write("bad.md", bad)); !strings.Contains(strings.Join(r.Findings, "\n"), `missing part "## Pre-mortem"`) {
		t.Errorf("bad draft: %v", r.Findings)
	}
	invented := strings.Replace(good, "#6 stays blocked (#6).", "#6 stays blocked (#99).", 1)
	if r := pack(4, "2", "--draft", write("invented.md", invented)); !strings.Contains(strings.Join(r.Findings, "\n"), "cites #99") {
		t.Errorf("invented citation: %v", r.Findings)
	}

	if code, _, errOut := run(t, env, opts, "decide", "--repo", "o/r", "--pack", "2", "--accept", "A"); code != 4 || !strings.Contains(errOut, "read-only") {
		t.Errorf("--pack --accept: exit %d: %s", code, errOut)
	}
	if code, _, errOut := run(t, env, opts, "decide", "--repo", "o/r", "which engine?"); code != 2 || !strings.Contains(errOut, "a question needs --pack") {
		t.Errorf("question without --pack: exit %d: %s", code, errOut)
	}
	if n := len(g.Writes()); n != 0 {
		t.Errorf("decide --pack wrote %d times", n)
	}
}

// Story #432, criteria 1 and 4, end to end: with no second human configured
// (solo), the reversible Decision #2 applies at 24 business hours (Wednesday
// 16:00 London), a minute before it does not, and the waiver #4 and the
// irreversible #3 and #5 never do; status then lists #2 with its deadline and
// how to reverse it.
func TestSoloProvisional(t *testing.T) {
	g := decideTree(t)
	now := time.Date(2026, 10, 14, 14, 59, 0, 0, time.UTC)
	env := map[string]string{"AAYU_GITHUB_TOKEN": "t", "XDG_STATE_HOME": t.TempDir()}
	opts := Options{IsTerminal: stdinTTY, GitHubAPI: g.URL(), GitHubGap: -1, Now: func() time.Time { return now }, NotifySink: &notify.Fake{}}
	if code, _, errOut := run(t, env, opts, "decide", "--repo", "o/r"); code != 0 || slices.Contains(g.LabelNames(2), workitems.LabelProvisional) {
		t.Fatalf("a minute before 24 business hours: exit %d %s, #2 labels %v", code, errOut, g.LabelNames(2))
	}
	now = now.Add(time.Minute)
	if code, _, errOut := run(t, env, opts, "decide", "--repo", "o/r"); code != 0 {
		t.Fatalf("decide at 24 business hours: exit %d: %s", code, errOut)
	}
	for n, want := range map[int]bool{2: true, 3: false, 4: false, 5: false} {
		if got := slices.Contains(g.LabelNames(n), workitems.LabelProvisional); got != want {
			t.Errorf("#%d provisional = %v at 24 solo business hours", n, got)
		}
	}
	g.Mu.Lock()
	body := g.Issues[2].Body
	g.Mu.Unlock()
	if !strings.Contains(body, "within 24 business hours (V8, solo mode)") {
		t.Errorf("#2 body:\n%s", body)
	}
	code, out, _ := run(t, env, opts, "status", "--repo", "o/r")
	if want := "provisional Decision #2 Choose the rules engine: ratify or revert by 2026-10-19T12:00:00Z with `aayu inbox` or `aayu decide 2 --accept <option>`"; code != 0 || !strings.Contains(out, want) {
		t.Errorf("status: exit %d, want %q:\n%s", code, want, out)
	}
}
