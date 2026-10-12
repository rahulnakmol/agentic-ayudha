package brief

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations/exports"
	"github.com/rahulnakmol/agentic-ayudha/internal/measure"
	"github.com/rahulnakmol/agentic-ayudha/internal/verify"
)

func loadWords(t *testing.T) *Words {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "skills", "brief", "references", "ste-words.md"))
	if err != nil {
		t.Fatal(err)
	}
	w, err := ParseWords(b)
	if err != nil {
		t.Fatal(err)
	}
	return w
}

func units(t *testing.T, md string) []Unit {
	t.Helper()
	d, err := Parse([]byte(md), "")
	if err != nil {
		t.Fatal(err)
	}
	return d.Units
}

// WP7.3 accept: a planted 30-word sentence fails the STE lint; the other two
// rules (approved words, one instruction per sentence) each catch their
// defect and pass its fix.
func TestSTELint(t *testing.T) {
	w := loadWords(t)
	names := map[string]bool{"csv": true}
	thirty := "The user can open the list and see all the items that the filter shows, and the team can check each item in the same list before the export starts."
	cases := []struct {
		name, text string
		want       []string // substrings, one per expected problem; none means it passes
	}{
		{"planted 30-word sentence", thirty, []string{"sentence 1 has 30 words (at most 20)"}},
		{"20 words pass", "The user can open the list and see all the items that the filter shows in the same list now.", nil},
		{"21 words fail", "The user can open the list and see all the items that the filter shows in the same list again now.", []string{"has 21 words"}},
		{"unapproved word with its replacement", "Utilize the export to make the file.", []string{`"Utilize" is not an approved word (sentence 1); write use`}},
		{"two instructions", "Open the list and select the export.", []string{"sentence 1 gives 2 instructions"}},
		{"one instruction each", "Open the list. Select the export. If the file is empty, open the filter.", nil},
		{"a description is not an instruction", "The gate runs the floors and writes the pack.", nil},
		{"technical names pass", "`aayu verify` runs the floors on the PR, and Playwright checks 12 pages of the CSV.", nil},
	}
	// A metric keeps its own nouns: words of the subject's summary and of the Result are technical names.
	g := &Grounding{Subject: Issue{Title: "Export filtered claims", Fields: map[string]string{"summary": "Reviewers audit offline."}},
		Result: &Result{Text: "Target met: CSV exports per reviewer per week = 4.2"}}
	if got := LintSTE(units(t, "Each reviewer can audit the claims offline [R]."), w, Names(g)); len(got) != 0 {
		t.Errorf("summary and Result words: %q", got)
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := LintSTE(units(t, c.text), w, names)
			if len(got) != len(c.want) {
				t.Fatalf("got %q, want %d problem(s) %q", got, len(c.want), c.want)
			}
			for i, s := range c.want {
				if !strings.Contains(got[i], s) {
					t.Errorf("problem %d = %q, want it to contain %q", i, got[i], s)
				}
			}
		})
	}
}

// The eli5 bar is a Flesch-Kincaid grade of 6. The formula is checked on a
// sentence worked out by hand, and Check fails a hard paragraph and passes
// an easy one.
func TestReadability(t *testing.T) {
	// 6 words, 6 syllables, 1 sentence: 0.39*6 + 11.8*1 - 15.59 = -1.45.
	r := Readability(units(t, "The cat sat on the mat."))
	if r.Words != 6 || r.Syllables != 6 || r.Sentences != 1 || math.Abs(r.Grade-(-1.45)) > 1e-9 {
		t.Fatalf("reading %+v", r)
	}
	for w, n := range map[string]int{"export": 2, "filtered": 2, "claims": 1, "authoritative": 5, "read-only": 3, "12": 1} {
		if got := Syllables(w); got != n {
			t.Errorf("Syllables(%q) = %d, want %d", w, got, n)
		}
	}
	g := &Grounding{Subject: Issue{Number: 7, Title: "Export"}}
	hard := "# Export\n\nThe authoritative proof pack demonstrates comprehensive verification of accessibility requirements across every interactive component [#7].\n"
	easy := "# Export\n\nYou can save the list as a file now [#7]. It has the rows you picked. It has the columns you see.\n"
	for md, fail := range map[string]bool{hard: true, easy: false} {
		d, _ := Parse([]byte(md), "")
		f := Check(d, g, Options{Register: "eli5", Purpose: "explain"})
		if got := len(f) > 0 && f[0].Check == "readability"; got != fail {
			t.Errorf("eli5 check on %q: %v", md, f)
		}
	}
}

// sampleGrounding is Feature #7 with Story #12, its authoritative pack P1
// (holding screens/1.png), the Feature's Result, and snapshot D1 built from
// the WP7.2 fixture exports.
func sampleGrounding(t *testing.T) *Grounding {
	t.Helper()
	td := filepath.Join("..", "measure", "testdata")
	var files []*exports.File
	for _, k := range []string{"billing", "appinsights", "aws", "gcp"} {
		ext := ".jsonl"
		if k == "billing" {
			ext = ".csv"
		}
		f, err := exports.Read(k, filepath.Join(td, k+ext))
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
	}
	snap := filepath.Join(t.TempDir(), "7-20261012T090000Z.sqlite")
	w := measure.Window{From: time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)}
	st := measure.NewStages(config.Measure{Acquisition: "signed_up", Activation: "created_project", Referral: "invited_user", NorthStar: "created_project"})
	sha, err := measure.Snapshot(context.Background(), snap, w, st, files, time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	shot, err := os.ReadFile(filepath.Join("testdata", "screen.png"))
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(shot)
	return &Grounding{
		Repo:    "o/r",
		Subject: Issue{Number: 7, Type: "feature", Title: "Export filtered claims", State: "open", Status: "released", URL: "https://github.com/o/r/issues/7"},
		Items:   []Issue{{Number: 12, Type: "story", Title: "CSV export of filtered claims", State: "closed", Status: "released"}},
		Packs: []Pack{{ID: "P1", Story: 12, Authority: "authoritative", RunID: 4242, HeadSHA: "0123456789abcdef0123456789abcdef01234567", PR: 30,
			LedgerPath: "proof/12/0123456789abcdef0123456789abcdef01234567/authoritative/pack.json", SHA256: strings.Repeat("ab", 32), FloorsHold: true,
			Checks:    []Floor{{Floor: "coverage", Result: "pass", Value: 94, Threshold: 90}, {Floor: "sast", Result: "pass"}, {Floor: "a11y", Result: "pass"}},
			Artifacts: []Artifact{{Path: "screens/1.png", Kind: "screenshot", SHA256: hex.EncodeToString(h[:])}},
			Criteria: []PackCriterion{
				{ID: 1, Kind: "screenshot", Text: "state: a filtered list; expected: the export follows the filters", Evidence: []Artifact{{Path: "screens/1.png", Kind: "screenshot", SHA256: hex.EncodeToString(h[:])}}, Verdict: "met"},
				{ID: 2, Kind: "api-trace", Text: "state: four of six columns; expected: only the visible columns", Evidence: []Artifact{{Path: "evidence/criterion-2.jsonl", Kind: "api-trace", SHA256: strings.Repeat("ef", 32)}}},
				{ID: 3, Kind: "recording", Text: "state: over 10,000 claims; expected: the list keeps responding", Evidence: []Artifact{}},
			},
			Inspect: &Inspection{Disposition: "merge", Model: "gpt-6.1-sol", Head: "0123456789abcdef0123456789abcdef01234567"}}},
		Result:    &Result{Issue: 7, Field: "result", Text: "Target met: activation = 0.4 against the target >= 0.35 (baseline 0.25, change +0.15).", SHA256: strings.Repeat("cd", 32)},
		Snapshots: []Snapshot{{ID: "D1", File: filepath.Base(snap), SHA256: sha, Path: snap}},
	}
}

func sampleSources() Sources {
	return Sources{Artifact: func(_ context.Context, _ Pack, path string) ([]byte, error) {
		return os.ReadFile(filepath.Join("testdata", filepath.FromSlash(strings.Replace(path, "screens/1.png", "screen.png", 1))))
	}}
}

// renderSample checks, prepares, and renders testdata/sample/brief.md.
func renderSample(t *testing.T) ([]byte, []*Figure, *Grounding) {
	t.Helper()
	g := sampleGrounding(t)
	dir := filepath.Join("testdata", "sample")
	src, err := os.ReadFile(filepath.Join(dir, "brief.md"))
	if err != nil {
		t.Fatal(err)
	}
	d, err := Parse(src, dir)
	if err != nil {
		t.Fatal(err)
	}
	if f := Check(d, g, Options{Register: "engineer", Purpose: "explain", Words: loadWords(t)}); len(f) > 0 {
		t.Fatalf("sample draft fails its checks: %v", f)
	}
	figs, f := Prepare(context.Background(), d, g, sampleSources())
	if len(f) > 0 || len(figs) != 4 {
		t.Fatalf("prepare: %d figures, %v", len(figs), f)
	}
	page, err := Render(d, g, figs, Page{Register: "engineer", Purpose: "explain", Version: "test", Generated: time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC),
		Draft: strings.Repeat("0", 64), Checks: []string{"Structure, citations, and visuals: pass."}, Review: Review{Model: "gpt-6.1-sol", Family: "openai", Findings: []ReviewFinding{}}})
	if err != nil {
		t.Fatal(err)
	}
	return page, figs, g
}

// WP7.4 accept: every chart carries its query, and re-running the query on
// the snapshot reproduces the chart's data. The expected rows are the
// acquisition cohorts worked out by hand in internal/cli/measure_test.go
// (u1, u2, u3 in week 0; u4, u5 in week 1; u6 in week 3).
func TestChartGolden(t *testing.T) {
	page, figs, g := renderSample(t)
	var charts []*ChartData
	for _, f := range figs {
		if f.Chart != nil {
			charts = append(charts, f.Chart)
		}
	}
	if len(charts) != 1 {
		t.Fatalf("%d charts", len(charts))
	}
	c := charts[0]
	golden := [][]any{{int64(0), int64(3)}, {int64(1), int64(2)}, {int64(3), int64(1)}}
	if !reflect.DeepEqual(c.Columns, []string{"week", "acquired"}) || !reflect.DeepEqual(c.Rows, golden) {
		t.Fatalf("chart data %v %v, want golden %v", c.Columns, c.Rows, golden)
	}
	if c.Snapshot != g.Snapshots[0].File || c.SHA256 != g.Snapshots[0].SHA256 || c.Query == "" {
		t.Fatalf("chart provenance %+v", c)
	}
	cols, rows, err := Query(context.Background(), g.Snapshots[0].Path, c.Query)
	if err != nil || !reflect.DeepEqual(cols, c.Columns) || !reflect.DeepEqual(rows, c.Rows) {
		t.Fatalf("re-running the query: %v %v %v", cols, rows, err)
	}
	for _, want := range []string{"Data and query", "SELECT week, COUNT(*) AS acquired", c.SHA256, `href="#src-D1"`} {
		if !strings.Contains(string(page), want) {
			t.Errorf("page lacks %q", want)
		}
	}
	// A chart query only reads.
	for _, q := range []string{"DELETE FROM events", "SELECT 1; DROP TABLE events", "ATTACH 'x' AS y", "PRAGMA writable_schema"} {
		if _, _, err := Query(context.Background(), g.Snapshots[0].Path, q); err == nil {
			t.Errorf("query %q was accepted", q)
		}
	}
}

// The rendered page: every image and SVG image has alt text, the
// illustration is labelled as generated, the screen carries its numbered
// notes from the pack, diagrams come as a static SVG image and as focusable
// interactive nodes, and the grounding footer lists every source.
func TestRenderSample(t *testing.T) {
	page, figs, _ := renderSample(t)
	if p := AltText(page); len(p) > 0 {
		t.Fatalf("alt text: %v", p)
	}
	s := string(page)
	for _, want := range []string{
		`<strong class="generated">Generated illustration</strong> made with Painter`,
		`<ol class="notes"><li>The applied filters decide which rows are written.</li>`,
		`<li id="src-7">`, `<li id="src-12">`, `<li id="src-P1">`, `<li id="src-R">`, `<li id="src-D1">`,
		`<li id="src-P1.c1"><span class="id">P1.c1</span> Criterion 1 [screenshot]: state: a filtered list; expected: the export follows the filters. Evidence: screenshot <code>screens/1.png</code>`,
		`<li id="src-P1.c3"><span class="id">P1.c3</span> Criterion 3 [recording]: state: over 10,000 claims; expected: the list keeps responding. <strong>No evidence in the pack</strong>; not proved.</li>`,
		"Inspect: <strong>merge</strong> by gpt-6.1-sol.",
		`[<a class="cite" href="#src-P1" aria-label="source P1">P1</a>]`,
		`tabindex="0" role="button" aria-label="Merge gate: Runs verify again on the head commit in CI."`,
		`default-src 'none'`, A11yMarker,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("page lacks %q", want)
		}
	}
	for _, f := range figs {
		if f.Kind != "diagram" {
			continue
		}
		if !strings.Contains(f.SVG, `role="img" aria-labelledby="fig-2-title fig-2-desc"`) || !strings.Contains(f.SVG, "<title id=\"fig-2-title\">"+f.Alt+"</title>") || strings.Contains(f.SVG, "tabindex") {
			t.Errorf("static diagram SVG: %s", f.SVG[:200])
		}
	}
	if got := SetA11y(page, "axe found 0 issues."); !strings.Contains(string(got), "<span data-aayu-a11y>axe found 0 issues.</span>") {
		t.Error("SetA11y did not fill the marker")
	}
	if p := AltText([]byte(`<img src="data:image/png;base64,AA" alt=""><svg role="img"><title></title></svg>`)); len(p) != 2 {
		t.Errorf("AltText on bad markup: %v", p)
	}
}

// Visual and purpose rules that Check enforces before anything renders.
func TestCheckRules(t *testing.T) {
	g := sampleGrounding(t)
	check := func(md, register, purpose string) []Finding {
		d, err := Parse([]byte(md), t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		return Check(d, g, Options{Register: register, Purpose: purpose, Words: loadWords(t)})
	}
	has := func(fs []Finding, sub string) bool {
		for _, f := range fs {
			if strings.Contains(f.String(), sub) {
				return true
			}
		}
		return false
	}
	cases := []struct{ name, md, register, purpose, want string }{
		{"no title", "Text [P1].\n", "engineer", "explain", "needs a title"},
		{"unknown source", "# T\n\nIt shipped [P9].\n", "engineer", "explain", "[P9] is not a source"},
		{"no citation", "# T\n\nIt shipped.\n", "engineer", "explain", "cites no source"},
		{"markdown image", "# T\n\nIt shipped [P1].\n\n![x](https://example.com/x.png)\n", "engineer", "explain", "Markdown image"},
		{"raw html", "# T\n\nIt shipped [P1].\n\n<script>alert(1)</script>\n", "engineer", "explain", "raw HTML"},
		{"close cites R and a pack", "# T\n\nIt shipped [#12].\n", "engineer", "close", "cites the Result [R]"},
		{"ideate lists ideas", "# T\n\nIt shipped [P1].\n\n## Ideas\n\n- One idea.\n", "engineer", "ideate", "at least two ideas (found 1)"},
		{"innovate states a hypothesis", "# T\n\nIt shipped [P1].\n", "engineer", "innovate", "## Hypothesis"},
		{"regulatory cites every item", "# T\n\nIt shipped [P1].\n\n- An uncited item.\n", "regulatory", "explain", "list item 2 has no citation"},
		{"alt names the medium", "# T\n\nIt shipped [P1].\n\n```diagram\nalt: Diagram of the merge flow.\na -> b\n```\n", "engineer", "explain", "describes the medium"},
		{"chart on an unknown snapshot", "# T\n\nIt shipped [P1].\n\n```chart\nalt: Sign-ups fell in week three of the window.\nsnapshot: D9\nSELECT 1\n```\n", "engineer", "explain", `snapshot "D9" is not a measure snapshot`},
		{"chart query writes", "# T\n\nIt shipped [P1].\n\n```chart\nalt: Sign-ups fell in week three of the window.\nsnapshot: D1\nDELETE FROM events\n```\n", "engineer", "explain", "one SELECT"},
		{"illustration without generator", "# T\n\nIt shipped [P1].\n\n```illustration\nalt: Rows go in and a file comes out.\nsrc: x.png\n```\n", "engineer", "explain", "generator names the tool"},
		{"screen not in pack", "# T\n\nIt shipped [P1].\n\n```screen\nalt: The button sits beside the filters.\npack: P1\nartifact: screens/9.png\n```\n", "engineer", "explain", `"screens/9.png" is not a screenshot in pack P1`},
	}
	for _, c := range cases {
		if f := check(c.md, c.register, c.purpose); !has(f, c.want) {
			t.Errorf("%s: findings %v, want one containing %q", c.name, f, c.want)
		}
	}
	// Evidence is cited only through the pack's criterion -> evidence map (P1 maps criteria 1 and 2, not 3).
	for md, want := range map[string]string{
		"# T\n\nIt shipped [P1].\n\nThe recording proves criterion 3 [P1].\n":             "claims proof of criterion 3, but no pack maps evidence to it",
		"# T\n\nIt shipped [P1].\n\nCriterion 1 is met [P1].\n":                           "without citing its evidence map entry (P1.c1)",
		"# T\n\nIt shipped [P1].\n\nCriterion 3 holds [P1.c3].\n":                         "[P1.c3] is not a source",
		"# T\n\nIt shipped [P1].\n\nThe recording shows the list keeps scrolling [P1].\n": "says what an artifact shows without citing the criterion",
		"# T\n\nIt shipped [P1].\n\nCriteria 1 to 3 are met [P1.c1, P1.c2].\n":            "claims proof of criterion 3",
	} {
		if f := check(md, "engineer", "explain"); !has(f, want) {
			t.Errorf("%q: findings %v, want %q", md, f, want)
		}
	}
	for _, md := range []string{
		"# T\n\nIt shipped [P1].\n\nCriteria 1 to 2 are met [P1.c1, P1.c2].\n",
		"# T\n\nIt shipped [P1].\n\nCriterion 3 is not proved: the pack has no evidence for it [P1].\n",
	} {
		if f := check(md, "engineer", "explain"); len(f) > 0 {
			t.Errorf("%q: %v", md, f)
		}
	}
	// Limits: a close or regulatory brief states what its evidence does not show.
	for _, rp := range [][2]string{{"engineer", "close"}, {"regulatory", "explain"}} {
		if f := check("# T\n\nIt shipped [P1, R].\n", rp[0], rp[1]); !has(f, "`## Limits of this evidence` section") {
			t.Errorf("%s/%s without limits: %v", rp[0], rp[1], f)
		}
	}
	if f := check("# T\n\nIt shipped [P1].\n", "engineer", "explain"); has(f, "Limits") {
		t.Errorf("an explain brief needs no limits: %v", f)
	}
	// Population: the Result states a rate per reviewer (rewritten here); a unit citing R keeps it.
	pg := *g
	pg.Result = &Result{Text: "Target met: CSV exports per reviewer per week = 4.2 against the target >= 3 (baseline 0). Window: 2026-09-14 to 2026-10-12."}
	pcheck := func(md string) []Finding {
		d, _ := Parse([]byte(md), "")
		return Check(d, &pg, Options{Register: "engineer", Purpose: "explain"})
	}
	for md, want := range map[string]string{
		"# T\n\nUsage is 4.2 exports a week [R].\n":                              `population: paragraph 1 states a number from the Result without its population: keep "per reviewer"`,
		"# T\n\nEach reviewer exports 4.2 files per reviewer a week [R].\n":      `says "each reviewer"`,
		"# T\n\nUsage beat the target of 3 [R].\n":                               `unit: paragraph 1 states 3 from the Result without its unit: write "3 exports"`,
		"# T\n\nReviewers do it 4.2 times per reviewer per week [R].\n":          `states 4.2 from the Result without its unit`,
		"# T\n\nThe count shows how many files were saved [R].\n":                `names the count "files were saved"`,
		"# T\n\nReviewers made more files this week [R].\n":                      `names the count "made more files"`,
		"# T\n\nReviewers made how many saves per reviewer per week [R].\n":      `names the count "saves"`,
		"# T\n\nThis number counts saves [R].\n":                                 `names the count "saves"`,
		"# T\n\nThis number counts files saved [R].\n":                           `names the count "files saved"; the Result counts exports`,
		"# T\n\nThe count is 4.2 downloads per reviewer per week [R].\n":         `names the count "downloads"`,
		"# T\n\nThe rate is 4.2 exports per reviewer per week. It beat 3 [R].\n": `population: paragraph 1 states a number from the Result without its population`,
	} {
		if f := pcheck(md); !has(f, want) {
			t.Errorf("%q: %v, want %q", md, f, want)
		}
	}
	for _, md := range []string{
		// A rate and its target split into two sentences, each with its unit and population.
		"# T\n\nThe rate is 4.2 exports per reviewer per week [R]. The target is 3 exports per reviewer per week [R].\n",
		// One "per reviewer" covers both rates of one sentence; no second repetition is forced.
		"# T\n\nUsage is 4.2 CSV exports per reviewer per week, against a target of 3 exports [R].\n",
		// "saves" as a verb is not a count (D-7.3h, the false positive in run 38230815198).
		"# T\n\nThe count shows only CSV exports. The evidence does not show that the export saves time [R].\n",
		"# T\n\nThe window ran from 2026-09-14 to 2026-10-12 [R].\n",
		"# T\n\nCriterion 3 is not proved; usage met its target [R].\n",
		"# T\n\nEach reviewer story shipped 4.2 [P1].\n", // does not cite R
	} {
		if f := pcheck(md); has(f, "population") || has(f, "unit") {
			t.Errorf("%q: %v", md, f)
		}
	}
	// Exclusion: an item the Scope field puts out of scope is "outside this story's scope", not "does not include".
	sg := *g
	sg.Subject.Fields = map[string]string{"scope": "In scope: CSV export. Out of scope: PDF, scheduled exports, exports of other lists."}
	for md, bad := range map[string]bool{
		"# T\n\nThis story does not include PDF files, exports at set times, or exports of other lists [#7].\n": true,
		"# T\n\nScheduled exports are not included [#7].\n":                                                     true,
		"# T\n\nPDF and scheduled exports are outside this story's scope [#7].\n":                               false,
		"# T\n\nThe export does not include hidden columns [P1].\n":                                             false, // not an out-of-scope item
	} {
		d, _ := Parse([]byte(md), "")
		if f := Check(d, &sg, Options{Register: "engineer", Purpose: "explain"}); has(f, "of items the grounding marks out of scope") != bad {
			t.Errorf("%q: %v (want an exclusion finding: %v)", md, f, bad)
		}
	}
	// UI details: a control no source names is invented; one the grounding names is fine.
	if f := check("# T\n\nPress one button to export the claims [P1].\n", "engineer", "explain"); !has(f, `names "Press", an interface detail`) || !has(f, `names "button"`) {
		t.Errorf("invented controls: %v", f)
	}
	ug := *g
	ug.Subject.Fields = map[string]string{"summary": "Reviewers click Export CSV to download the filtered claims."}
	d, _ := Parse([]byte("# T\n\nReviewers click Export CSV, and clicking it exports the claims [P1].\n"), "")
	if f := Check(d, &ug, Options{Register: "engineer", Purpose: "explain"}); has(f, "interface detail") {
		t.Errorf("a control the summary names: %v", f)
	}
	// Absence: missing evidence is "the evidence does not show", never "untested" or "can't";
	// a source's own wording ("cannot run", in a floor detail here) may be quoted.
	ag := *g
	ag.Packs = []Pack{g.Packs[0]}
	ag.Packs[0].Checks = append([]Floor{}, g.Packs[0].Checks...)
	ag.Packs[0].Checks = append(ag.Packs[0].Checks, Floor{Floor: "screen-reader", Result: "skipped", Detail: "skipped: the Linux merge gate cannot run a screen reader"})
	for md, bad := range map[string]bool{
		"# T\n\nOther lists can't be saved this way yet [#7].\n":                                    true,
		"# T\n\nOther lists cannot be saved [#7].\n":                                                true,
		"# T\n\nLarger lists are untested [P1].\n":                                                  true,
		"# T\n\nA PDF export is not possible [#7].\n":                                               true,
		"# T\n\nThe merge gate cannot run a screen reader [P1].\n":                                  false,
		"# T\n\nThe evidence does not show larger lists; PDF is outside this story's scope [#7].\n": false,
	} {
		d, _ := Parse([]byte(md), "")
		if f := Check(d, &ag, Options{Register: "engineer", Purpose: "explain"}); has(f, "absence:") != bad {
			t.Errorf("%q: %v (want an absence finding: %v)", md, f, bad)
		}
	}
	if f := check("# T\n\nIt shipped [P1, R].\n\n## Limits of this evidence\n\n- Usage does not show time saved [R].\n", "regulatory", "close"); len(f) > 0 {
		t.Errorf("a cited close brief: %v", f)
	}
	// An image of Mobbin origin is refused when prepared (WP6.4).
	dir := t.TempDir()
	png, _ := os.ReadFile(filepath.Join("testdata", "sample", "illustration.png"))
	_ = os.WriteFile(filepath.Join(dir, "mobbin-ref.png"), png, 0o644)
	d, _ = Parse([]byte("# T\n\nIt shipped [P1].\n\n```illustration\nalt: Rows go in and a file comes out.\nsrc: mobbin-ref.png\ngenerator: Painter\n```\n"), dir)
	if _, f := Prepare(context.Background(), d, g, Sources{}); len(f) != 1 || !strings.Contains(f[0].Message, "Mobbin") {
		t.Errorf("Mobbin image: %v", f)
	}
}

// WP7.3 accept: the rendered page has zero serious or critical axe findings
// (WCAG 2.2 AA tags) in a real browser. Runs in the floors workflow
// (floors-web) with Playwright and axe-core from packages/floors.
func TestBriefPageWithBrowser(t *testing.T) {
	if os.Getenv("AAYU_FLOOR_WEB") != "1" {
		t.Skip("needs Playwright and axe-core: set AAYU_FLOOR_WEB=1 (floors workflow)")
	}
	page, _, _ := renderSample(t)
	// axe skips closed <details>; open them so the data tables, queries, and text alternatives are checked too.
	page = []byte(strings.ReplaceAll(string(page), "<details>", "<details open>"))
	page = []byte(strings.ReplaceAll(string(page), `<details class="data">`, `<details class="data" open>`))
	dir := t.TempDir()
	file := filepath.Join(dir, "brief.html")
	if err := os.WriteFile(file, page, 0o644); err != nil {
		t.Fatal(err)
	}
	nodePath := os.Getenv("AAYU_FLOOR_NODE_PATH")
	if nodePath == "" {
		nodePath, _ = filepath.Abs(filepath.Join("..", "..", "packages", "floors", "node_modules"))
	}
	getenv := func(k string) string {
		if k == "NODE_PATH" {
			return nodePath
		}
		return os.Getenv(k)
	}
	pc, err := verify.CheckPage(context.Background(), &verify.Sandbox{Home: t.TempDir(), Getenv: getenv}, t.TempDir(), t.TempDir(), "file://"+file)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%s: %d critical, %d serious, unreachable %v", pc.Tool, pc.Critical, pc.Serious, pc.Unreachable)
	if pc.Critical+pc.Serious > 0 {
		t.Fatalf("axe: %v", pc.Violations)
	}
	if len(pc.Unreachable) > 0 {
		t.Fatalf("controls Tab does not reach: %v", pc.Unreachable)
	}
}

// Shapes the sample does not reach: a cycle drawn left to right (its back
// edge routed below), an explicit direction kept even when wide, and a line
// chart of one row of zeros, all with finite coordinates.
func TestDiagramAndChartShapes(t *testing.T) {
	d, err := ParseDiagram("direction: right\na: Write the draft\nb: Run the checks\nc: Render the page\na -> b: draft\nb -> c: pass\nb -> a: findings\nc -> d -> e")
	if err != nil {
		t.Fatal(err)
	}
	svg := d.SVG("fig-9", "The draft loops back until the checks pass.", false)
	if strings.Contains(svg, "NaN") || !strings.Contains(svg, `data-from="b" data-to="a"`) || d.Down || len(d.Nodes) != 5 {
		t.Fatalf("cycle diagram (down %v, %d nodes): %s", d.Down, len(d.Nodes), svg)
	}
	for _, bad := range []string{"a -> b -> c: label", "title -> a", "just text", "a:"} {
		if _, err := ParseDiagram(bad); err == nil {
			t.Errorf("ParseDiagram(%q) accepted", bad)
		}
	}
	c := &ChartData{Columns: []string{"day", "n"}, Rows: [][]any{{"mon", int64(0)}}, Kind: "line"}
	if err := c.axes("", ""); err != nil {
		t.Fatal(err)
	}
	svg = c.SVG("fig-8", "Nothing happened on Monday.")
	if strings.Contains(svg, "NaN") || strings.Contains(svg, "Inf") || !strings.Contains(svg, "<polyline") {
		t.Fatalf("one-row line chart: %s", svg)
	}
	if err := (&ChartData{Columns: []string{"day", "who"}, Rows: [][]any{{"mon", "ana"}}}).axes("", ""); err == nil {
		t.Error("a text series was accepted")
	}
}
