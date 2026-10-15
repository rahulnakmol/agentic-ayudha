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
