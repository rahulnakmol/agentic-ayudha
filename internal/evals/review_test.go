package evals

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	aayudha "github.com/rahulnakmol/agentic-ayudha"
	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/inspect"
	"github.com/rahulnakmol/agentic-ayudha/internal/models"
)

// WP4.8b: the seeded-defect set has defects of several kinds and clean
// controls, belongs to the review group, and every planted defect sits on an
// added line (a moved defect fails tier 1).
func TestReviewCases(t *testing.T) {
	cases, err := LoadCases(root)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	kinds, clean := map[string]bool{}, 0
	for _, c := range cases {
		if c.Review == nil {
			continue
		}
		if g := cfg.GroupOf(c.ID); g != "review" {
			t.Errorf("%s is in group %q, not review", c.ID, g)
		}
		if len(c.Review.Defects) == 0 {
			clean++
		}
		for _, d := range c.Review.Defects {
			kinds[d.Kind] = true
		}
	}
	if clean < 3 || len(kinds) < 5 {
		t.Fatalf("%d clean controls, defect kinds %v", clean, kinds)
	}
	g, _ := cfg.Group("review")
	if g.Gate != GateAdvisory || g.RunsOn("push") || g.RunsOn("schedule") || !g.RunsOn("workflow_dispatch") {
		t.Fatalf("review group: %+v", g)
	}
	c, err := LoadCase(filepath.Join(root, "evals", "cases", "inspect", "race-snapshot"))
	if err != nil {
		t.Fatal(err)
	}
	if bad := Tier1Review(c); len(bad) > 0 {
		t.Fatal(bad)
	}
	c.Review.Defects[0].Lines = []int{2, 2} // `import (` is context, not an added line
	if bad := Tier1Review(c); len(bad) == 0 {
		t.Fatal("a defect on a line the PR does not add passed tier 1")
	}
	in, err := ReviewInput(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	a := in.Areas[0]
	if !slices.ContainsFunc(a.Symbols, func(s inspect.Symbol) bool { return s.Name == "Counter.Snapshot" }) ||
		!slices.ContainsFunc(a.Tests, func(x inspect.Test) bool { return x.Name == "TestSnapshot" }) {
		t.Fatalf("context: %+v", a)
	}
}

// Matching findings to planted defects: within three lines, each defect once;
// a duplicate and a stray finding are false positives; an unplaced finding
// matches only a file's single defect.
func TestScoreFindings(t *testing.T) {
	ds := []Defect{{File: "a.go", Lines: []int{10, 12}}, {File: "a.go", Lines: []int{30, 30}}, {File: "b.go", Lines: []int{5, 5}}}
	fs := []inspect.Finding{
		{File: "a.go", Line: 14}, // defect 1 (12 + 3 slack: 15)
		{File: "a.go", Line: 11}, // a duplicate of defect 1
		{File: "a.go", Line: 0},  // unplaced in a file with two defects: no match
		{File: "b.go", Line: 0},  // unplaced in a file with one defect: defect 3
		{File: "a.go", Line: 26}, // 30 - 3 = 27: outside
		{File: "c.go", Line: 30}, // another file
	}
	if tp, fp, fn := ScoreFindings(fs, ds); tp != 2 || fp != 4 || fn != 1 {
		t.Fatalf("tp %d fp %d fn %d", tp, fp, fn)
	}
	if reviewScore(0, 0, 0, true) != 5 || reviewScore(0, 1, 0, true) != 1 || reviewScore(1, 0, 0, false) != 5 || reviewScore(1, 1, 1, false) != 3 {
		t.Fatal("scores")
	}
}

// reviewFake answers by request: the 0.1 prompt, the v2 prompt (by model),
// and the peer check, so concurrent variants get deterministic replies.
type reviewFake struct {
	mu   sync.Mutex
	reqs []models.Request
}

func (f *reviewFake) Complete(_ context.Context, r models.Request) (models.Response, error) {
	f.mu.Lock()
	f.reqs = append(f.reqs, r)
	f.mu.Unlock()
	loop := `{"severity":"P1","category":"bug","file":"internal/invoice/total.go","line":3,"quote":"for i := 1; i < len(lines); i++ {","title":"Total skips the first invoice line","detail":"the loop starts at 1","fix":"start at 0","confidence":0.9,"criterion":1}`
	qty := `{"severity":"P1","category":"contract","file":"internal/invoice/total.go","line":0,"quote":"n, _ := strconv.Atoi(s)","title":"ParseQty ignores the parse error","detail":"abc and 0 are accepted","fix":"return the error and refuse n < 1","confidence":0.8,"criterion":2}`
	nit := `{"severity":"P2","category":"style","file":"internal/invoice/total.go","line":9,"quote":"// discount (0-100).","title":"comment wording","detail":"d","fix":"f","confidence":0.9,"criterion":0}`
	var text string
	switch {
	case strings.HasPrefix(r.System, inspect.VerifySystem):
		text = `{"verdicts":[]}`
	case strings.Contains(r.System, "quote: copy the exact line"):
		text = `{"disposition":"fix","summary":"s","findings":[` + loop + `,` + nit + `],"criteria":[]}`
		if r.Model == "grok-4.7" {
			text = `{"disposition":"fix","summary":"s","findings":[` + qty + `],"criteria":[]}`
		}
	default: // the 0.1 prompt: line numbers from the model, a nit, no quote
		text = `{"disposition":"fix","summary":"s","findings":[{"severity":"P1","file":"internal/invoice/total.go","line":16,"title":"Total skips line 0","detail":"d","criterion":1},{"severity":"P3","file":"internal/invoice/total.go","line":9,"title":"comment wording","detail":"d","criterion":0}],"criteria":[]}`
	}
	return models.Response{Text: text, Model: r.Model, TokensIn: 1000, TokensOut: 200}, nil
}

// Tier 2 of the review group with a scripted gateway: each variant is run,
// scored against the planted defects, and pooled per variant.
func TestTier2ReviewVariants(t *testing.T) {
	c, err := LoadCase(filepath.Join(root, "evals", "cases", "inspect", "two-defects-invoice"))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	cfg, _, _ = cfg.ForGroup("review", nil)
	cfg.K = 1
	log, err := actions.Open(filepath.Join(t.TempDir(), "actions.jsonl"), actions.Options{})
	if err != nil {
		t.Fatal(err)
	}
	fake := &reviewFake{}
	m := JudgeModels(config.Models{}, cfg.Judge)
	h := &Harness{Root: root, Config: cfg, Assets: aayudha.Assets, Inspect: config.Inspect{MinConfidence: 0.5},
		Gateway: &models.Gateway{Models: m, Resolver: models.Resolver{Models: m}, Actions: log,
			Key:    func(string) (string, error) { return "k", nil },
			Client: func(models.Route, string) models.Client { return fake }}}
	res, err := h.Tier2(context.Background(), []Case{c}, "abc", "dev", time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]ReviewSummary{}
	for _, s := range res.Review {
		got[s.Variant] = s
	}
	if s := got[VariantLegacy]; s.TP != 1 || s.FP != 1 || s.FN != 1 {
		t.Errorf("legacy: %+v", s)
	}
	if s := got[VariantGeneral]; s.TP != 1 || s.FP != 0 || s.FN != 1 || s.Precision != 1 {
		t.Errorf("general: %+v", s)
	}
	if s := got[VariantPeer]; s.TP != 2 || s.FP != 0 || s.FN != 0 || s.Recall != 1 {
		t.Errorf("peer: %+v", s)
	}
	if len(res.Cases) != 3 || res.Cases[2].Case != "inspect/two-defects-invoice@peer" || res.Cases[2].Runs[0].Score != 5 {
		t.Errorf("cases: %+v", res.Cases)
	}
	// The results file says why each run scored as it did: what it posted,
	// marked against the planted defects, then what it dropped and why.
	var verdicts []string
	for _, f := range res.Cases[1].Runs[0].Findings {
		verdicts = append(verdicts, f.Verdict+" "+f.Title)
	}
	if want := []string{"true Total skips the first invoice line", "dropped: linter-owned comment wording"}; !slices.Equal(verdicts, want) {
		t.Errorf("general findings: %q, want %q", verdicts, want)
	}
}
