package inspect

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/models"
	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

const pkgDiff = `diff --git a/internal/report/percent.go b/internal/report/percent.go
--- a/internal/report/percent.go
+++ b/internal/report/percent.go
@@ -1,6 +1,9 @@ package report
 package report
 
 // Percent is covered as a percentage of total.
 func Percent(covered, total int) float64 {
+	if total < 0 {
+		return 0
+	}
 	return float64(covered) / float64(total) * 100
 }
diff --git a/LAYERS.md b/LAYERS.md
--- a/LAYERS.md
+++ b/LAYERS.md
@@ -1,1 +1,1 @@
-old
+new
`

// scripted is a reviewer whose replies are fixed, in order.
func scripted(model string, replies ...any) (Reviewer, *int) {
	calls := 0
	return Reviewer{Model: model, Complete: func(_ context.Context, req models.Request) (models.Response, float64, error) {
		if calls >= len(replies) {
			return models.Response{}, 0, errors.New("unexpected call")
		}
		b, _ := json.Marshal(replies[calls])
		calls++
		return models.Response{Text: string(b), Model: model}, 0.01, nil
	}}, &calls
}

func finding(sev, cat, file, quote, title string, conf float64) map[string]any {
	return map[string]any{"severity": sev, "category": cat, "file": file, "line": 1, "quote": quote, "title": title,
		"detail": title + " in detail", "fix": "fix it", "confidence": conf, "criterion": 0}
}

func reply(disp string, fs ...map[string]any) map[string]any {
	if fs == nil {
		fs = []map[string]any{}
	}
	return map[string]any{"disposition": disp, "summary": "s", "findings": fs, "criteria": []map[string]any{{"id": 1, "verdict": "met", "evidence": "e"}}}
}

func verdict(id, v string) map[string]any {
	return map[string]any{"verdicts": []map[string]any{{"id": id, "verdict": v, "reason": "line 7 shows otherwise"}}}
}

func testInput(mode string) Input {
	return Input{Repo: "o/r", PR: 9, Story: 2, Diff: pkgDiff, Mode: mode,
		Pack: &proof.Pack{PRHeadSHA: "abc", Checks: []proof.Check{{Floor: "coverage", Result: proof.Pass}}}}
}

func whys(ds []Dropped) []string {
	var out []string
	for _, d := range ds {
		out = append(out, d.Why)
	}
	slices.Sort(out)
	return out
}

// The deterministic noise rules on one reviewer's reply (WP4.8b): the quote
// places the finding, the duplicate merges keeping the higher severity, and
// style, generated files, files outside the diff, low-confidence P2s, and P3
// nits are not posted.
func TestGeneralModeFilters(t *testing.T) {
	zero := "return float64(covered) / float64(total) * 100"
	a, calls := scripted("gpt-6.1-sol", reply("fix",
		finding("P1", "bug", "internal/report/percent.go", zero, "Percent divides by zero when total is zero", 0.9),
		finding("P2", "bug", "internal/report/percent.go", zero, "division by zero when total is zero", 0.8),
		finding("P2", "style", "internal/report/percent.go", "if total < 0 {", "prefer early return style", 0.9),
		finding("P2", "maintainability", "internal/report/percent.go", "if total < 0 {", "Typo in comment", 0.9),
		finding("P1", "bug", "internal/other/x.go", "x", "bug in a file the PR does not change", 0.9),
		finding("P2", "bug", "internal/report/percent.go", "return 0", "negative total may hide a caller bug", 0.2),
		finding("P3", "maintainability", "internal/report/percent.go", "return 0", "name the constant", 0.9),
		finding("P2", "docs", "LAYERS.md", "new", "table out of date", 0.9),
	))
	r := Runner{Reviewers: []Reviewer{a}, Filter: Filter{MinConfidence: 0.5, Generated: []string{"LAYERS.md"}}}
	res, st, err := r.Run(context.Background(), testInput(""), State{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Findings) != 1 || res.Findings[0].Severity != "P1" || res.Findings[0].Line != 8 || res.Findings[0].ID == "" {
		t.Fatalf("findings: %+v", res.Findings)
	}
	want := []string{DropDuplicate, DropGenerated, DropLinterOwned, DropLinterOwned, DropLowConfidence, DropNit, DropOutsideDiff}
	if got := whys(res.Dropped); !slices.Equal(got, want) {
		t.Fatalf("dropped %v, want %v", got, want)
	}
	if res.Disposition != Fix || *calls != 1 || res.Mode != ModeGeneral || st.Cycles != 1 {
		t.Fatalf("%s, %d calls, mode %s, cycles %d", res.Disposition, *calls, res.Mode, st.Cycles)
	}
	in := InlineComments(res, nil)
	if len(in) != 1 || in[0].Line != 8 || !strings.Contains(in[0].Body, FindingMarker+res.Findings[0].ID) {
		t.Fatalf("inline: %+v", in)
	}
	if len(InlineComments(res, []string{res.Findings[0].ID})) != 0 {
		t.Fatal("a finding already posted is posted again")
	}
}

// Peer mode: two reviewers of different families. A finding both raise is
// confirmed (A+B, highest severity); one only A raises and B refutes is
// dropped; a P0 security finding only B raises and A refutes is kept as
// disputed (protected category).
func TestPeerModeCrossCheck(t *testing.T) {
	zero := "return float64(covered) / float64(total) * 100"
	real := finding("P1", "bug", "internal/report/percent.go", zero, "Percent divides by zero when total is zero", 0.9)
	bogus := finding("P1", "bug", "internal/report/percent.go", "return 0", "negative total returns 0 instead of an error", 0.7)
	z := finding("P0", "security", "internal/report/percent.go", "if total < 0 {", "attacker controls total", 0.6)
	bogusID := FindingID("internal/report/percent.go", "bug", "negative total returns 0 instead of an error")
	zID := FindingID("internal/report/percent.go", "security", "attacker controls total")
	a, ca := scripted("gpt-6.1-sol", reply("fix", real, bogus), verdict(zID, "refute"))
	realB := finding("P2", "bug", "internal/report/percent.go", zero, "division by zero when total is 0", 0.8)
	b, cb := scripted("grok-4.7", reply("fix", realB, z), verdict(bogusID, "refute"))
	r := Runner{Reviewers: []Reviewer{a, b}, Filter: Filter{MinConfidence: 0.5}}
	res, _, err := r.Run(context.Background(), testInput(ModePeer), State{})
	if err != nil {
		t.Fatal(err)
	}
	if *ca != 2 || *cb != 2 || res.Mode != ModePeer || len(res.Reviewers) != 2 {
		t.Fatalf("calls A %d B %d, mode %s %v", *ca, *cb, res.Mode, res.Reviewers)
	}
	var gotReal, gotZ bool
	for _, f := range res.Findings {
		switch {
		case strings.Contains(f.Title, "divides by zero"):
			gotReal = f.Severity == "P1" && slices.Equal(f.Sources, []string{"A", "B"})
		case f.ID == zID:
			gotZ = f.Disputed != ""
		case f.ID == bogusID:
			t.Fatalf("the refuted finding was posted: %+v", f)
		}
	}
	if !gotReal || !gotZ || len(res.Findings) != 2 || !slices.Contains(whys(res.Dropped), DropRefuted) || res.Disposition != Fix {
		t.Fatalf("findings %+v dropped %v disposition %s", res.Findings, whys(res.Dropped), res.Disposition)
	}
}

// A fix that rests only on a finding the peer refuted is lifted to merge; the
// floors still decide afterwards.
func TestPeerLiftsRefutedFix(t *testing.T) {
	bogus := finding("P1", "bug", "internal/report/percent.go", "return 0", "negative total returns 0 instead of an error", 0.7)
	id := FindingID("internal/report/percent.go", "bug", "negative total returns 0 instead of an error")
	a, _ := scripted("gpt-6.1-sol", reply("fix", bogus))
	b, cb := scripted("grok-4.7", reply("merge"), verdict(id, "refute"))
	r := Runner{Reviewers: []Reviewer{a, b}}
	res, _, err := r.Run(context.Background(), testInput(ModePeer), State{})
	if err != nil || res.Disposition != Merge || len(res.Findings) != 0 || *cb != 2 {
		t.Fatalf("%v %s %+v, %d B calls", err, res.Disposition, res.Findings, *cb)
	}
	in := testInput(ModePeer)
	in.Pack.Checks = []proof.Check{{Floor: "coverage", Result: proof.Fail}}
	a, _ = scripted("gpt-6.1-sol", reply("fix", bogus))
	b, _ = scripted("grok-4.7", reply("merge"), verdict(id, "refute"))
	res, _, _ = Runner{Reviewers: []Reviewer{a, b}}.Run(context.Background(), in, State{})
	if res.Disposition != Fix || !strings.Contains(res.Overridden, "floor coverage fail") {
		t.Fatalf("rule 7: %s %q", res.Disposition, res.Overridden)
	}
}

// Learned suppression: two distinct dismissals of similar findings in the
// same directory suppress a new one; a single unrelated dismissal does not;
// a P0 in a protected category is never suppressed.
func TestSuppressDismissed(t *testing.T) {
	fl := Filter{SuppressAfter: 2}
	f := Finding{ID: "new", Severity: "P2", Category: "maintainability", File: "internal/report/percent.go", Title: "Percent lacks a doc example"}
	ds := []Dismissal{
		{ID: "d1", PR: 3, File: "internal/report/sum.go", Category: "maintainability", Title: "Sum lacks a doc example"},
		{ID: "d2", PR: 4, File: "internal/report/percent.go", Category: "maintainability", Title: "Percent lacks doc example"},
	}
	if keep, drop := fl.Suppress([]Finding{f}, ds); len(keep) != 0 || len(drop) != 1 || drop[0].Why != DropDismissed {
		t.Fatalf("two similar dismissals: keep %+v", keep)
	}
	if keep, _ := fl.Suppress([]Finding{f}, ds[:1]); len(keep) != 1 {
		t.Fatal("one dismissal suppressed a finding")
	}
	p0 := Finding{ID: "d1", Severity: "P0", Category: "security", File: "internal/report/sum.go", Title: "Sum lacks a doc example"}
	if keep, _ := fl.Suppress([]Finding{p0}, ds); len(keep) != 1 {
		t.Fatal("a protected P0 was suppressed")
	}
}

type mapSource map[string]string

func (m mapSource) File(_ context.Context, p string) ([]byte, error) {
	if s, ok := m[p]; ok {
		return []byte(s), nil
	}
	return nil, errors.New("not found")
}

func (m mapSource) Dir(_ context.Context, d string) ([]string, error) {
	var out []string
	for p := range m {
		if strings.HasPrefix(p, d+"/") && !strings.Contains(strings.TrimPrefix(p, d+"/"), "/") {
			out = append(out, p)
		}
	}
	return out, nil
}

// Context retrieval: the changed function, its caller in another file of the
// package, the test that exercises it, the layer, and the risks; a generated
// file is named and not reviewed.
func TestBuildContext(t *testing.T) {
	src := mapSource{
		"internal/report/percent.go":      "package report\n\n// Percent is covered as a percentage of total.\nfunc Percent(covered, total int) float64 {\n\tif total < 0 {\n\t\treturn 0\n\t}\n\treturn float64(covered) / float64(total) * 100\n}\n",
		"internal/report/table.go":        "package report\n\nfunc Row(c, t int) string {\n\treturn fmt.Sprint(Percent(c, t))\n}\n",
		"internal/report/percent_test.go": "package report\n\nfunc TestPercent(t *testing.T) {\n\tif Percent(1, 2) != 50 {\n\t\tt.Fatal()\n\t}\n}\n\nfunc TestOther(t *testing.T) {}\n",
		"LAYERS.md":                       "<!-- Code generated by internal/verbs/gen. DO NOT EDIT. -->\nnew\n",
	}
	foot := func(ps []string) []string {
		if slices.ContainsFunc(ps, func(p string) bool { return strings.HasPrefix(p, "internal/") }) {
			return []string{"L04"}
		}
		return nil
	}
	areas := BuildContext(context.Background(), ParseDiff(pkgDiff), src, foot)
	if len(areas) != 2 || areas[0].Dir != "." || areas[1].Dir != "internal/report" {
		t.Fatalf("areas: %+v", areas)
	}
	if !slices.Equal(areas[0].Generated, []string{"LAYERS.md"}) {
		t.Fatalf("generated: %+v", areas[0].Generated)
	}
	a := areas[1]
	if len(a.Symbols) != 1 || a.Symbols[0].Name != "Percent" || !strings.Contains(a.Symbols[0].Source, "total < 0") {
		t.Fatalf("symbols: %+v", a.Symbols)
	}
	if len(a.Callers) != 1 || a.Callers[0].In != "Row" || a.Callers[0].Line != 4 {
		t.Fatalf("callers: %+v", a.Callers)
	}
	if len(a.Tests) != 1 || a.Tests[0].Name != "TestPercent" || a.TestsChanged {
		t.Fatalf("tests: %+v changed %t", a.Tests, a.TestsChanged)
	}
	plan := PlanText(areas)
	for _, want := range []string{"policy (L04)", "no test changed in this area", "Percent ← Row", "generated (not reviewed): LAYERS.md"} {
		if !strings.Contains(plan, want) {
			t.Errorf("plan lacks %q:\n%s", want, plan)
		}
	}
}

// Bug #171: the builder's family is known only from a record that covers the PR (a delegation, or a checkpoint
// naming a model family for every commit) with one family the trailer, if any, does not contradict; otherwise every
// family any source names is a candidate.
func TestBuilderOf(t *testing.T) {
	cp := func(m map[string]string) Records { return Records{Checkpoints: m} }
	for _, tt := range []struct {
		name    string
		rec     Records
		commits []string
		claim   string
		lane    verbs.Family
		family  verbs.Family
		cands   []verbs.Family
		source  string
	}{
		{"record covers every commit", cp(map[string]string{"a": "claude-opus-5-5", "b": "claude-sonnet-5-5"}), []string{"a", "b"}, "", verbs.FamilyOpenAI,
			verbs.FamilyAnthropic, []verbs.Family{verbs.FamilyAnthropic}, "attributed, not verified"},
		{"a hand-made commit the record lacks", cp(map[string]string{"a": "claude-opus-5-5"}), []string{"a", "b"}, "", verbs.FamilyOpenAI,
			"", []verbs.Family{verbs.FamilyAnthropic}, "names a model family for 1 of 2 commits"},
		{"a checkpoint without a model does not cover its commit", cp(map[string]string{"a": "claude-opus-5-5", "b": ""}), []string{"a", "b"}, "", "",
			"", []verbs.Family{verbs.FamilyAnthropic}, "for 1 of 2 commits"},
		{"two families in the record", cp(map[string]string{"a": "claude-opus-5-5", "b": "gpt-6.1-sol"}), []string{"a", "b"}, "", "",
			"", []verbs.Family{verbs.FamilyAnthropic, verbs.FamilyOpenAI}, "name 2 families"},
		{"a host-managed build records no model", cp(map[string]string{"a": ""}), []string{"a"}, "", verbs.FamilyHostManaged,
			"", nil, "no build or delegation record"},
		{"no record: the claim and the lane are candidates", Records{}, []string{"a"}, "grok-4.7", verbs.FamilyAnthropic,
			"", []verbs.Family{verbs.FamilyXAI, verbs.FamilyAnthropic}, "trailer claims grok-4.7 (self-reported, unverified)"},
		{"a delegation covers the PR", Records{Receiver: "codex", Delegated: verbs.FamilyOpenAI}, []string{"a", "b"}, "", verbs.FamilyAnthropic,
			verbs.FamilyOpenAI, []verbs.Family{verbs.FamilyOpenAI}, "handed the story to codex"},
		{"a trailer contradicting the delegation", Records{Receiver: "codex", Delegated: verbs.FamilyOpenAI}, []string{"a"}, "claude-opus-5-5", "",
			"", []verbs.Family{verbs.FamilyOpenAI, verbs.FamilyAnthropic}, "the records say openai"},
	} {
		got := BuilderOf(tt.rec, tt.commits, tt.claim, tt.lane)
		if got.Family != tt.family || !slices.Equal(got.Candidates, tt.cands) || !strings.Contains(got.Note(), tt.source) {
			t.Errorf("%s: %+v; want family %q, candidates %v, a note with %q", tt.name, got, tt.family, tt.cands, tt.source)
		}
	}
}

const testFileDiff = `diff --git a/internal/report/percent_test.go b/internal/report/percent_test.go
--- a/internal/report/percent_test.go
+++ b/internal/report/percent_test.go
@@ -1,3 +1,6 @@ package report
 package report
 
+func TestPercentNegative(t *testing.T) {
+	if Percent(1, -1) != 0 {
+		t.Fatal("negative total")
+	}
+}
`

// Test gaps on a test file become criterion verdicts, never line findings
// (research P1: all 8 of inspect's seeded-set false positives were these):
// a P2 gap makes its met criterion unclear and the fix it alone caused is
// lifted to merge; a P1 gap makes its criterion unmet, which still blocks; a
// gap naming no criterion is a note; a test that restates the code is
// counted as test-value and never posted.
func TestTestGapsBecomeVerdicts(t *testing.T) {
	in := testInput("")
	in.Diff = pkgDiff + testFileDiff
	gap := finding("P2", "test", "internal/report/percent_test.go", "func TestPercentNegative(t *testing.T) {", "No test for a zero total", 0.85)
	gap["criterion"] = 1
	restates := finding("P3", "test-value", "internal/report/percent_test.go", `t.Fatal("negative total")`, "asserts the implementation's branch", 0.9)
	bugInTest := finding("P2", "bug", "internal/report/percent_test.go", "if Percent(1, -1) != 0 {", "compares with the wrong operand", 0.9)
	a, _ := scripted("gpt-6.1-sol", reply("fix", gap, restates))
	res, _, err := Runner{Reviewers: []Reviewer{a}}.Run(context.Background(), in, State{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Findings) != 0 || res.Criteria[0].Verdict != "unclear" || !strings.Contains(res.Criteria[0].Evidence, "No test for a zero total") || res.Disposition != Merge {
		t.Fatalf("P2 gap: findings %+v criteria %+v disposition %s", res.Findings, res.Criteria, res.Disposition)
	}
	if got := whys(res.Dropped); !slices.Equal(got, []string{DropTestGap, DropTestValue}) {
		t.Fatalf("dropped %v", got)
	}
	if c := Comment(res, State{}, 1); !strings.Contains(c, "1 test-gap, 1 test-value") {
		t.Fatalf("comment does not count them:\n%s", c)
	}

	gap["severity"] = "P1"
	noCrit := finding("P2", "test", "internal/report/percent_test.go", "", "missing table test", 0.9)
	a, _ = scripted("gpt-6.1-sol", reply("merge", gap, noCrit, bugInTest))
	res, _, _ = Runner{Reviewers: []Reviewer{a}}.Run(context.Background(), in, State{})
	if res.Criteria[0].Verdict != "unmet" || res.Disposition != Fix || !strings.Contains(res.Overridden, "criterion 1 unmet") {
		t.Fatalf("P1 gap: %+v %s %q", res.Criteria, res.Disposition, res.Overridden)
	}
	if len(res.Findings) != 1 || res.Findings[0].Title != "compares with the wrong operand" {
		t.Fatalf("a real defect in a test file is still a finding: %+v", res.Findings)
	}
	if !slices.ContainsFunc(res.Notes, func(n string) bool { return strings.Contains(n, "missing table test") }) {
		t.Fatalf("a gap without a criterion is a note: %v", res.Notes)
	}
}

// The sealed coverage manifest: every changed file is reviewed, skipped with
// a reason, or unreviewed; a file past the prompt's diff cap is unreviewed,
// so the run is partial and merge becomes fix. The seal changes with the list.
func TestCoverageManifest(t *testing.T) {
	in := testInput("")
	in.Diff = pkgDiff + testFileDiff + "diff --git a/old.go b/old.go\n--- a/old.go\n+++ /dev/null\n@@ -1,1 +0,0 @@\n-package old\n"
	a, _ := scripted("gpt-6.1-sol", reply("merge"))
	res, _, _ := Runner{Reviewers: []Reviewer{a}, Filter: Filter{Generated: []string{"LAYERS.md"}}}.Run(context.Background(), in, State{})
	m := res.Coverage
	want := []Coverage{
		{Path: "internal/report/percent.go", Status: Reviewed},
		{Path: "LAYERS.md", Status: SkippedCov, Why: "generated: the drift check owns it"},
		{Path: "internal/report/percent_test.go", Status: Reviewed},
		{Path: "old.go", Status: SkippedCov, Why: "deleted"},
	}
	if m == nil || !slices.Equal(m.Files, want) || m.State != "complete" || res.Disposition != Merge {
		t.Fatalf("manifest %+v, disposition %s", m, res.Disposition)
	}
	big := "diff --git a/internal/big/big.go b/internal/big/big.go\n--- /dev/null\n+++ b/internal/big/big.go\n@@ -0,0 +1,1 @@\n+// " + strings.Repeat("x", maxDiffBytes) + "\n"
	in.Diff = pkgDiff + big
	a, _ = scripted("gpt-6.1-sol", reply("merge"))
	res2, _, _ := Runner{Reviewers: []Reviewer{a}}.Run(context.Background(), in, State{})
	if res2.Coverage.State != "partial" || !slices.Equal(res2.Coverage.Unreviewed(), []string{"internal/big/big.go"}) ||
		res2.Disposition != Fix || !strings.Contains(res2.Overridden, "left unreviewed: internal/big/big.go") || res2.Coverage.SHA256 == m.SHA256 {
		t.Fatalf("over the cap: %+v %s %q", res2.Coverage, res2.Disposition, res2.Overridden)
	}
}

// A PR may edit its own checks; each weakening is flagged, and tightening or
// adding checks is not.
func TestWeakenedChecks(t *testing.T) {
	diff := `diff --git a/internal/report/percent_test.go b/internal/report/percent_test.go
--- a/internal/report/percent_test.go
+++ b/internal/report/percent_test.go
@@ -1,8 +1,6 @@
 func TestPercent(t *testing.T) {
-	if Percent(1, 2) != 50 {
-		t.Fatal("half")
-	}
-	if Percent(0, 2) != 0 { t.Errorf("zero") }
+	t.Skip("flaky")
+	if Percent(1, 2) != 50 { t.Fatal("half") }
 }
diff --git a/.github/workflows/ci.yml b/.github/workflows/ci.yml
--- a/.github/workflows/ci.yml
+++ b/.github/workflows/ci.yml
@@ -5,6 +5,5 @@
       - uses: actions/checkout@v7
-      - name: Lint
-        run: golangci-lint run
       - name: Test
+        continue-on-error: true
         run: go test ./...
diff --git a/.aayudha/config.toml b/.aayudha/config.toml
--- a/.aayudha/config.toml
+++ b/.aayudha/config.toml
@@ -1,4 +1,4 @@
 [verify.floors.coverage]
-threshold = 90
+threshold = 80
-per_pr_inspect_max = 2.0
+per_pr_inspect_max = 5.0
-min_score = 3
+min_score = 4
`
	got := Weakened(ParseDiff(diff))
	want := []Weakening{
		{"internal/report/percent_test.go", "fewer assertions (2 removed, 1 added)"},
		{"internal/report/percent_test.go", "a test is skipped or narrowed (skip or only added)"},
		{".github/workflows/ci.yml", "fewer steps (1 removed, 0 added)"},
		{".github/workflows/ci.yml", "continue-on-error added: a failing step no longer fails the job"},
		{".aayudha/config.toml", "threshold `threshold` loosened from 90 to 80"},
		{".aayudha/config.toml", "threshold `per_pr_inspect_max` loosened from 2 to 5"},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("weakened:\n%+v\nwant\n%+v", got, want)
	}
	if w := Weakened(ParseDiff(pkgDiff + testFileDiff)); len(w) != 0 {
		t.Fatalf("adding a test is not a weakening: %+v", w)
	}
	// D-4.5b.9: a feature map's behaviour checks. Renaming a handle is not a
	// weakening; dropping an assertion or a check is, and so is deleting the file.
	fm := "examples/app/.agents/skills/verify-app/features/"
	mapDiff := "diff --git a/" + fm + "ask.md b/" + fm + "ask.md\n--- a/" + fm + "ask.md\n+++ b/" + fm + "ask.md\n@@ -1,9 +1,4 @@\n" +
		"-  {\"click\": {\"role\": \"button\", \"name\": \"Ask\"}},\n+  {\"click\": {\"role\": \"button\", \"name\": \"Send\"}},\n" +
		"-  {\"expectText\": \"50 dollars\"},\n" +
		" ]}\n" +
		"-```check\n-{\"id\": \"empty\", \"kind\": \"ui\", \"steps\": [{\"goto\": \"/\"}, {\"expect\": {\"role\": \"heading\"}}]}\n-```\n" +
		"diff --git a/" + fm + "refer.md b/" + fm + "refer.md\ndeleted file mode 100644\n--- a/" + fm + "refer.md\n+++ /dev/null\n@@ -1 +0,0 @@\n-# Refer\n"
	want = []Weakening{
		{fm + "ask.md", "fewer behaviour checks (1 removed, 0 added)"},
		{fm + "ask.md", "fewer behaviour assertions (2 removed, 0 added)"},
		{fm + "refer.md", "deleted"},
	}
	if got := Weakened(ParseDiff(mapDiff)); !slices.Equal(got, want) {
		t.Fatalf("feature map weakened:\n%+v\nwant\n%+v", got, want)
	}
	rename := "diff --git a/" + fm + "ask.md b/" + fm + "ask.md\n--- a/" + fm + "ask.md\n+++ b/" + fm + "ask.md\n@@ -1 +1 @@\n" +
		"-  {\"click\": {\"name\": \"Ask\"}}, {\"expectText\": \"x\"},\n+  {\"click\": {\"name\": \"Send\"}}, {\"expectText\": \"x\"},\n"
	if w := Weakened(ParseDiff(rename)); len(w) != 0 {
		t.Fatalf("a renamed handle is not a weakening: %+v", w)
	}
}

func TestRiskOf(t *testing.T) {
	file := func(p string, added, removed int) DiffFile {
		h := Hunk{NewStart: 1}
		for i := range added {
			h.Lines = append(h.Lines, DiffLine{N: i + 1, Text: "x", Added: true})
		}
		for range removed {
			h.Removed = append(h.Removed, "y")
		}
		return DiffFile{Path: p, Hunks: []Hunk{h}}
	}
	layer := func(paths []string) []string {
		var out []string
		for _, p := range paths {
			l := "L" + strings.SplitN(p, "/", 2)[0]
			if !slices.Contains(out, l) {
				out = append(out, l)
			}
		}
		slices.Sort(out)
		return out
	}
	protected := func(p string) bool { return strings.HasPrefix(p, ".github/") }
	tests := []struct {
		name    string
		files   []DiffFile
		tier    string
		reasons []string
		look    []string
	}{
		{"small change in two layers", []DiffFile{file("01/a.go", 10, 2), file("02/b.go", 30, 0)}, RiskLow, nil, []string{"02/b.go", "01/a.go"}},
		{"three layers", []DiffFile{file("01/a.go", 1, 0), file("02/b.go", 1, 0), file("03/c.go", 1, 0)}, RiskMedium, []string{"3 layers: L01, L02, L03"}, nil},
		{"401 changed lines", []DiffFile{file("01/a.go", 300, 101)}, RiskMedium, []string{"large diff: 1 files, 401 changed lines"}, []string{"01/a.go"}},
		{"400 changed lines", []DiffFile{file("01/a.go", 300, 100)}, RiskLow, nil, nil},
		{"never-layer", []DiffFile{file("01/a.go", 500, 0), file("09/deploy.go", 1, 0)}, RiskHigh, []string{"never-layer L09: 09/deploy.go"}, []string{"09/deploy.go"}},
		{"protected path and a migration", []DiffFile{file(".github/workflows/ci.yml", 1, 0), file("01/db/migrations/0002_add.sql", 3, 0)}, RiskHigh,
			[]string{"protected path: .github/workflows/ci.yml", "migration: 01/db/migrations/0002_add.sql"}, []string{".github/workflows/ci.yml", "01/db/migrations/0002_add.sql"}},
		{"dependency added", []DiffFile{file("go.mod", 1, 0)}, RiskHigh, []string{"dependency manifest changed: go.mod"}, nil},
		{"dependency removed only", []DiffFile{file("go.mod", 0, 1)}, RiskLow, nil, nil},
	}
	for _, tt := range tests {
		got := RiskOf(tt.files, layer, []string{"L09"}, protected)
		if got.Tier != tt.tier || (tt.reasons != nil || got.Tier == RiskLow) && !slices.Equal(got.Reasons, append([]string{}, tt.reasons...)) || tt.look != nil && !slices.Equal(got.Look, tt.look) {
			t.Errorf("%s: %+v", tt.name, got)
		}
	}
}

// Peer mode pairs each of B's findings with at most one of A's (D-4.8d.8),
// from findings the review eval recorded: one leak anchored on its cause and
// its symptom four lines apart, and one parse defect on the same line under
// two categories, merge; distinct problems on the same or nearby lines stay
// apart, and a finding pairs with its own problem, not with the first one
// nearby.
func TestPeerPairsOneDefectOnce(t *testing.T) {
	f := func(src string, line int, cat, title, detail string) Finding {
		return Finding{Severity: "P1", Category: cat, File: "x.go", Line: line, Title: title, Detail: detail, Sources: []string{src},
			ID: FindingID("x.go", cat, title)}
	}
	cases := []struct {
		name  string
		a, b  []Finding
		posts []string // titles posted, each with its sources
	}{
		{"leak, cause and symptom",
			[]Finding{f("A", 23, "bug", "Response body is never closed", "Any Fetch that gets a response, including a 404, returns without closing resp.Body. net/http then keeps the connection out of the idle pool, and repeated conversions leak sockets.")},
			[]Finding{f("B", 27, "bug", "Close the response body on every successful HTTP request", "After c.HTTP.Do returns a response without an error, Fetch reaches the status check without arranging to close resp.Body; repeated calls leave response resources unreleased.")},
			[]string{"Response body is never closed A+B"}},
		{"parse defect, two categories",
			[]Finding{f("A", 15, "bug", "Include the first invoice line in the total", "The loop starts at index 1, so the subtotal skips the first line and returns 45 rather than 225."),
				f("A", 23, "contract", "Reject invalid and nonpositive form quantities", "ParseQty(\"abc\") discards the conversion error and returns (0, nil); ParseQty(\"0\") also returns (0, nil). Both violate criterion 2.")},
			[]Finding{f("B", 23, "bug", "ParseQty never returns an error", "ParseQty(\"abc\") makes strconv.Atoi fail, but the error is discarded and n is 0, so it returns (0, nil). ParseQty(\"0\") also returns (0, nil). Criterion 2 requires an error for both.")},
			[]string{"Include the first invoice line in the total A", "Reject invalid and nonpositive form quantities A+B"}},
		{"distinct problems on one line and nearby",
			[]Finding{f("A", 13, "security", "Customer search input is executable SQL", "SearchHandler forwards q unchanged; interpolating it into a quoted SQL expression lets ' OR 1=1 -- replace the filter.")},
			[]Finding{f("B", 13, "bug", "Name match is case-sensitive, so q=ann misses Ann Lee", "Search uses LIKE with the raw text and no LOWER, so under a case-sensitive collation '%ann%' does not match Ann Lee."),
				f("B", 19, "contract", "No-match search encodes as JSON null", "out stays nil when rows.Next never runs, and encoding/json encodes a nil slice as null.")},
			[]string{"Customer search input is executable SQL A", "Name match is case-sensitive, so q=ann misses Ann Lee B", "No-match search encodes as JSON null B"}},
		{"each pairs with its own problem",
			[]Finding{f("A", 20, "bug", "Cap the initial wait as well as subsequent waits", "Retry(ctx, 3, time.Minute, f) waits a full minute after the first failure."),
				f("A", 22, "bug", "Check cancellation before invoking each attempt", "Retry invokes f without checking ctx.Err().")},
			[]Finding{f("B", 22, "bug", "Check cancellation before each attempt", "An already-cancelled request still calls f."),
				f("B", 21, "bug", "Cap the first wait too", "A one-minute base waits a minute, against the 30 s cap.")},
			[]string{"Cap the initial wait as well as subsequent waits A+B", "Check cancellation before invoking each attempt A+B"}},
	}
	for _, c := range cases {
		pair := pairPeer(c.a, c.b)
		keep, _ := mergePeer(c.a, c.b, pair, nil, nil)
		var got []string
		for _, k := range keep {
			got = append(got, k.Title+" "+strings.Join(k.Sources, "+"))
		}
		if !slices.Equal(got, c.posts) {
			t.Errorf("%s: posted %q, want %q", c.name, got, c.posts)
		}
	}
}

// A PR that touches only documentation merges without a review; any file
// that steers the product or its agents, even Markdown, needs one.
func TestDocsOnly(t *testing.T) {
	files := func(ps ...string) []DiffFile {
		var out []DiffFile
		for _, p := range ps {
			out = append(out, DiffFile{Path: p})
		}
		return out
	}
	for _, tc := range []struct {
		paths []string
		want  bool
	}{
		{[]string{"docs/guide/how-trust.md", "README.md", "examples/benefits-qa/README.md"}, true},
		{[]string{"docs/img/flow.png"}, true},
		{nil, false},
		{[]string{"docs/guide/how-trust.md", "internal/inspect/diff.go"}, false},
		{[]string{"skills/build/SKILL.md"}, false},
		{[]string{"AGENTS.md"}, false},
		{[]string{"internal/cli/AGENTS.md"}, false},
		{[]string{"docs/guide/AGENTS.md"}, false},
		{[]string{"docs/BUILD-PLAN.md"}, false},
		{[]string{"GLOSSARY.md"}, false},
		{[]string{"docs/threat-model.md"}, false},
		{[]string{"evals/cases/grill/bakery-ux/rubric.md"}, false},
		{[]string{".github/PULL_REQUEST_TEMPLATE.md"}, false},
		{[]string{"internal/hosts/testdata/skills/x.md"}, false},
		{[]string{"CHANGELOG.txt"}, false},
	} {
		if got := DocsOnly(files(tc.paths...)); got != tc.want {
			t.Errorf("DocsOnly(%v) = %v, want %v", tc.paths, got, tc.want)
		}
	}
	// A rename is judged by both paths: code or a steering document moved
	// into docs/ needs a review, with or without edits.
	for _, tc := range []struct {
		diff string
		want bool
	}{
		{"diff --git a/internal/auth.go b/docs/auth.go\nsimilarity index 100%\nrename from internal/auth.go\nrename to docs/auth.go\n", false},
		{"diff --git a/docs/BUILD-PLAN.md b/docs/archive/plan.md\nsimilarity index 90%\nrename from docs/BUILD-PLAN.md\nrename to docs/archive/plan.md\n--- a/docs/BUILD-PLAN.md\n+++ b/docs/archive/plan.md\n@@ -1 +1 @@\n-# Plan\n+# Old plan\n", false},
		{"diff --git a/docs/a.md b/docs/guide/a.md\nsimilarity index 100%\nrename from docs/a.md\nrename to docs/guide/a.md\n", true},
		{"diff --git \"a/docs/\\303\\251.md\" \"b/docs/\\303\\251.md\"\n--- \"a/docs/\\303\\251.md\"\n+++ \"b/docs/\\303\\251.md\"\n@@ -1 +1 @@\n-a\n+b\n", true},
	} {
		if got := DocsOnly(ParseDiff(tc.diff)); got != tc.want {
			t.Errorf("DocsOnly(%q) = %v, want %v", tc.diff, got, tc.want)
		}
	}
}
