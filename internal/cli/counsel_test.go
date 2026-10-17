package cli

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/counsel"
	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
)

// Story #478: `aayu counsel` grounds Atri's counsel on a question, an issue, a path, or a PR without writing;
// --draft checks a counsel (exit 4 on findings); --post puts one labelled counsel comment on the PR, replaced in
// place, and never a review.
func TestCounsel(t *testing.T) {
	g, env, opts, n := inspectFixture(t, "", "Refs #2")
	files := []string{"internal/verify/floors.go", "internal/report/percent.go"}
	for i := range 13 {
		files = append(files, fmt.Sprintf("docs/guide/page%02d.md", i))
	}
	g.Mu.Lock()
	g.Pulls[n].Files = files
	g.Mu.Unlock()
	writeInspectPack(t, opts.Dir, n, strings.Repeat("c", 40), []proof.Check{
		{Floor: "coverage", Result: proof.Pass, Value: 100, Threshold: 90, Tool: "aayudha", Version: "1"},
		{Floor: "sast", Result: proof.Pass, Tool: "semgrep", Version: "1.180.0", Waiver: "#9"},
	})
	layers, err := os.ReadFile(filepath.Join("..", "..", "LAYERS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(opts.Dir, "LAYERS.md"), layers, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(opts.Dir, "internal", "cli"), 0o755); err != nil {
		t.Fatal(err)
	}
	pr := strconv.Itoa(n)
	writes := len(g.Writes())
	counselRun := func(env map[string]string, want int, args ...string) counselReport {
		t.Helper()
		code, out, errOut := run(t, env, opts, append([]string{"counsel", "--repo", "o/r", "--json"}, args...)...)
		if code != want {
			t.Fatalf("counsel %v: exit %d, want %d: %s%s", args, code, want, errOut, clipFindings(out))
		}
		var rep counselReport
		if err := json.NewDecoder(strings.NewReader(out)).Decode(&rep); err != nil {
			t.Fatalf("counsel %v: %v: %s", args, err, out)
		}
		return rep
	}
	factText := func(r counselReport) string {
		var b strings.Builder
		for _, f := range r.Facts {
			fmt.Fprintf(&b, "%s (%s)\n", f.Text, f.Cite)
		}
		return b.String()
	}

	// Grounding: every form reads, none writes.
	onPR := counselRun(env, 0, "--pr", pr)
	facts := factText(onPR)
	for _, want := range []string{
		fmt.Sprintf(`PR #%d "Add Percent for the status table": open`, n),
		"Changed in L04: internal/verify/floors.go", "internal/report/percent.go", "and 2 more", "Story: #2", "Proof pack for #2 at ccccccc", "coverage pass (100, floor 90)",
		"sast pass waived by #9",
		"Builder:", "Open Decision: #5", "CONSTRAINTS.md", "L03 ",
	} {
		if !strings.Contains(facts, want) {
			t.Errorf("--pr facts lack %q:\n%s", want, facts)
		}
	}
	if onPR.Twin == "" || !strings.Contains(onPR.Twin, "Atri") || onPR.Artefact == "" || onPR.Builder == nil ||
		len(onPR.Untrusted) != 1 || onPR.Untrusted[0].Source != "pr-body" {
		t.Errorf("--pr grounding: twin %q, artefact %q, builder %v, untrusted %v", onPR.Twin, onPR.Artefact, onPR.Builder, onPR.Untrusted)
	}
	if q := counselRun(env, 0, "should we split the survey into sections?"); q.Issue != 0 || q.PR != 0 || q.Path != "" ||
		!strings.Contains(factText(q), "Open Decision: #5") || q.Topic != "should we split the survey into sections?" {
		t.Errorf("free question: %+v", q)
	}
	if is := counselRun(env, 0, "#2"); is.Issue != 2 || is.Topic != "Show empty list" || !strings.Contains(factText(is), "#1") {
		t.Errorf("issue topic: issue %d, topic %q, facts %s", is.Issue, is.Topic, factText(is))
	}
	if d := counselRun(env, 0, "--issue", "5"); d.Issue != 5 || d.Topic != "Engine" || strings.Contains(factText(d), "Open Decision: #5") {
		t.Errorf("Decision topic: issue %d, topic %q, facts %s", d.Issue, d.Topic, factText(d))
	}
	if p := counselRun(env, 0, "internal/cli"); p.Path != "internal/cli" ||
		!strings.Contains(factText(p), "Path internal/cli in this checkout: a directory, in L02 Interaction / API boundary (owner Code Architect)") {
		t.Errorf("path topic: %q %s", p.Path, factText(p))
	}
	if code, out, errOut := run(t, env, opts, "counsel", "--repo", "o/r", "--pr", pr); code != 0 ||
		!strings.Contains(out, "# Counsel grounding: Add Percent for the status table") || !strings.Contains(out, "<untrusted source=\"pr-body\">") ||
		!strings.Contains(out, "## The call") || !strings.Contains(out, "Speak and work as Atri") {
		t.Errorf("text grounding: exit %d: %s%s", code, out, errOut)
	}
	if w := len(g.Writes()); w != writes {
		t.Fatalf("counsel grounding wrote %d times", w-writes)
	}

	dir := t.TempDir()
	good := fmt.Sprintf(`# %s: Percent for the status table

## The call

Whether the status table ships with Percent as built (#%d): a two-way door, since it is a code change (#%d).

## Readings

- Operations: the proof pack's coverage floor passes (`+"`ledger/`"+` pack for #2).
- Technology: the change sits in internal/report/percent.go, L03 (LAYERS.md).
- Product: the story asks for the empty list first (#2).

Technology dominates: the risk is in internal/report/percent.go, not the scope (#%d).

## Recommendation

Merge after the normal review, keeping Percent's zero case as #2 states it (#2).

## What not to do

Do not widen the PR to docs/guide/page00.md rewrites (#%d).

## What would change it

Unverified: a reader that depends on NaN would change this.

## Next step

The founder runs `+"`aayu inspect %d`"+` for the review.

## Your question

Is the zero case the behaviour you want?
`, onPR.Artefact, n, n, n, n, n)
	write := func(name, s string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	if r := counselRun(env, 0, "--pr", pr, "--draft", write("good.md", good)); len(r.Findings) != 0 {
		t.Fatalf("good counsel: %v", r.Findings)
	}
	for name, tt := range map[string]struct{ from, to, want string }{
		"missing part":         {"## What not to do", "## Cautions", `missing part "## What not to do"`},
		"no door":              {"a two-way door", "reversible", "one-way or a two-way door"},
		"two recommendations":  {"Merge after the normal review", "- Merge after review (#2).\n- Or split it", "one recommendation"},
		"uncited claim":        {"Product: the story asks for the empty list first (#2).", "Product: the architecture already handles the empty list.", "uncited claim"},
		"no aayu command":      {"`aayu inspect", "`gh pr review", ""},
		"approval claim":       {"Merge after the normal review", "LGTM, I approve this; merge", "claims a review, an approval, or a ruling"},
		"passive review claim": {"Unverified: a reader", "The PR has been reviewed and a reader", "claims a review"},
		"invented citation":    {"(LAYERS.md)", "(#99)", "cites #99"},
	} {
		want := tt.want
		if want == "" {
			want = "`aayu` command"
		}
		r := counselRun(env, 4, "--pr", pr, "--draft", write(strings.ReplaceAll(name, " ", "-")+".md", strings.Replace(good, tt.from, tt.to, 1)), "--post")
		if !strings.Contains(strings.Join(r.Findings, "\n"), want) || r.Posted {
			t.Errorf("%s: findings %v, posted %v", name, r.Findings, r.Posted)
		}
	}
	if code, out, _ := run(t, env, opts, "counsel", "--repo", "o/r", "--pr", pr, "--draft", filepath.Join(dir, "approval-claim.md")); code != 4 ||
		!strings.Contains(out, "not a complete counsel yet:\n- ") {
		t.Errorf("text findings: exit %d: %s", code, out)
	}
	if w := len(g.Writes()); w != writes {
		t.Fatalf("counsel without a passing --post wrote %d times", w-writes)
	}

	// --post: one labelled comment, replaced in place, and no review. The head's Aayudha-Model trailer is only a
	// claim, so the builder's family is unknown and the counsel is labelled same-family (Decision #469).
	gpt := maps.Clone(env)
	gpt["AAYU_MODEL"] = "gpt-6.1-sol"
	p := write("good.md", good)
	if r := counselRun(gpt, 0, "--pr", pr, "--draft", p, "--post"); !r.Posted || r.Label != "same-family counsel: gpt-6.1-sol (openai), builder family unknown" {
		t.Errorf("--post: posted %v, label %q", r.Posted, r.Label)
	}
	if r := counselRun(gpt, 0, "--pr", pr, "--draft", p, "--post"); r.Posted {
		t.Errorf("an unchanged counsel was posted again")
	}
	claude := maps.Clone(env)
	claude["AAYU_MODEL"] = "claude-opus-5-5"
	if r := counselRun(claude, 0, "--pr", pr, "--draft", p, "--post"); !r.Posted || r.Label != "same-family counsel: claude-opus-5-5 (anthropic), builder family unknown" {
		t.Errorf("same-family rerun: posted %v, label %q", r.Posted, r.Label)
	}
	onDecision := write("decision.md", fmt.Sprintf(`# %s: the engine

## The call

Which engine to use is a one-way door once the Decision is accepted (#5).

## Readings

Operations, technology, and product all turn on #5; technology dominates, since L06 always needs a person (CONSTRAINTS.md).

## Recommendation

Keep #5 open until its options are written (#5).

## What not to do

Do not accept #5 without its options (#5).

## What would change it

Unverified: a deadline would.

## Next step

The founder runs `+"`aayu decide 5`"+`.

## Your question

Which options belong on #5?
`, onPR.Artefact))
	if r := counselRun(gpt, 0, "--issue", "5", "--draft", onDecision, "--post"); !r.Posted || r.Label != "written by gpt-6.1-sol (openai)" {
		t.Errorf("--post on an issue: posted %v, label %q", r.Posted, r.Label)
	}
	// Text mode, and the writer named from the spec lane's model when AAYU_MODEL is unset.
	if code, out, errOut := run(t, env, opts, "counsel", "--repo", "o/r", "--pr", pr, "--draft", p, "--post", "--host", "claude",
		"--set", "models.spec.claude.model=claude-fable-5-1"); code != 0 || !strings.Contains(out, "complete: every part is there") ||
		!strings.Contains(out, "Posted as counsel (same-family counsel: claude-fable-5-1 (anthropic), builder family unknown)") {
		t.Errorf("text post: exit %d: %s%s", code, out, errOut)
	}
	if code, out, _ := run(t, env, opts, "counsel", "--repo", "o/r", "--pr", pr, "--draft", p, "--post", "--host", "claude",
		"--set", "models.spec.claude.model=claude-fable-5-1"); code != 0 || !strings.Contains(out, "already carries this counsel") {
		t.Errorf("text rerun: exit %d: %s", code, out)
	}
	g.Mu.Lock()
	var posted []string
	for _, c := range g.Issues[n].Comments {
		if strings.HasPrefix(c.Body, counsel.Marker) {
			posted = append(posted, c.Body)
		}
	}
	reviews := len(g.Pulls[n].Reviews)
	onIssue := commentBodies(g.Issues[5].Comments)
	g.Mu.Unlock()
	if strings.Count(onIssue, counsel.Marker) != 1 {
		t.Errorf("want one counsel comment on Decision #5, got %q", onIssue)
	}
	if len(posted) != 1 || !strings.Contains(posted[0], "claude-fable-5-1 (anthropic)") || !strings.Contains(posted[0], "not a review, an approval, or a Decision") {
		t.Errorf("want one counsel comment carrying the latest label, got %q", posted)
	}
	if reviews != 0 {
		t.Errorf("counsel created %d reviews", reviews)
	}

	for _, args := range [][]string{{"counsel", "--repo", "o/r"}, {"counsel", "--repo", "o/r", "a question?", "--draft", p, "--post"}} {
		if code, _, errOut := run(t, env, opts, args...); code != 2 {
			t.Errorf("%v: exit %d, want 2: %s", args, code, errOut)
		}
	}
}

// clipFindings is a failed run's findings, for the test's message.
func clipFindings(out string) string {
	var rep counselReport
	_ = json.NewDecoder(strings.NewReader(out)).Decode(&rep)
	return strings.Join(rep.Findings, "\n")
}
