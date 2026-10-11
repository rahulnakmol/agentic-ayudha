package evals

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	aayudha "github.com/rahulnakmol/agentic-ayudha"
	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/models"
)

const root = "../.."

// TestTier1 is tier 1 on every PR (CI job build): every case under evals/cases.
func TestTier1(t *testing.T) {
	cases, err := LoadCases(root)
	if err != nil || len(cases) < 3 {
		t.Fatalf("cases: %v (%d)", err, len(cases))
	}
	for _, c := range cases {
		r := Tier1(context.Background(), root, c, aayudha.Assets)
		t.Log("\n" + r.String())
		if !r.Passed() {
			t.Errorf("tier 1 failed:\n%s", r)
		}
	}
}

// copyCase copies a case directory so a test can break it.
func copyCase(t *testing.T, id string) Case {
	t.Helper()
	src := filepath.Join(root, "evals", "cases", filepath.FromSlash(id))
	dst := filepath.Join(t.TempDir(), filepath.FromSlash(id))
	err := filepath.Walk(src, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if fi.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	c, err := LoadCase(dst)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func status(r Tier1Result, check string) string {
	for _, c := range r.Checks {
		if c.Name == check {
			return c.Status
		}
	}
	return "absent"
}

// WP3.4 accept: a fixture with a deleted artifact fails tier 1.
func TestTier1DeletedArtifact(t *testing.T) {
	c := copyCase(t, "stub/csv-export")
	if err := os.Remove(filepath.Join(c.Dir, "output", "issue.json")); err != nil {
		t.Fatal(err)
	}
	r := Tier1(context.Background(), root, c, aayudha.Assets)
	if r.Passed() || status(r, "artifacts") != Fail || !strings.Contains(r.String(), "output/issue.json is missing") {
		t.Fatalf("deleted artifact did not fail tier 1:\n%s", r)
	}
}

func TestTier1CatchesBadOutputs(t *testing.T) {
	c := copyCase(t, "stub/csv-export")
	// An unmapped criterion: valid JSON for the schema, but not Ready.
	b, _ := os.ReadFile(filepath.Join(c.Dir, "output", "issue.json"))
	_ = os.WriteFile(filepath.Join(c.Dir, "output", "issue.json"), []byte(strings.Replace(string(b), "[api-trace]", "[vibes]", 1)), 0o644)
	if r := Tier1(context.Background(), root, c, aayudha.Assets); status(r, "ready") != Fail || status(r, "schemas") != Pass {
		t.Fatalf("unmapped criterion:\n%s", r)
	}
	// A shape the schema rejects.
	_ = os.WriteFile(filepath.Join(c.Dir, "output", "issue.json"), []byte(`{"type": "epic", "title": "", "fields": {}}`), 0o644)
	if r := Tier1(context.Background(), root, c, aayudha.Assets); status(r, "schemas") != Fail {
		t.Fatalf("schema violation:\n%s", r)
	}
	// A recorded call that fails.
	c.Calls = [][]string{{"config", "show", "--origin"}, {"models", "show", "--set", "models.chore.claude.model="}}
	if r := Tier1(context.Background(), root, c, aayudha.Assets); status(r, "calls") != Fail || !strings.Contains(r.String(), "`aayu models show") {
		t.Fatalf("failing call:\n%s", r)
	}
}

// An injection fixture whose recorded run obeyed the injection fails, even
// before the write verb it called exists.
func TestTier1InjectionObeyed(t *testing.T) {
	c := copyCase(t, "stub/injection")
	c.Calls = append(c.Calls, []string{"ship", "12"})
	r := Tier1(context.Background(), root, c, aayudha.Assets)
	if status(r, "no-writes") != Fail || !strings.Contains(r.String(), "`aayu ship 12` is a never verb") {
		t.Fatalf("obeyed injection passed:\n%s", r)
	}
}

func readFixture(t *testing.T, name string) *Results {
	t.Helper()
	var r Results
	if err := readJSON(filepath.Join("testdata", name+".json"), &r); err != nil {
		t.Fatal(err)
	}
	return &r
}

// WP3.4 accept: a results file 0.4 below the baseline is reported as failed.
func TestCompare(t *testing.T) {
	cfg, err := LoadConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	base := readFixture(t, "baseline")
	for _, tc := range []struct {
		name, want, reason string
	}{
		{"drop-0.4", Fail, "is 0.40 below the baseline 4.40"},
		{"drop-0.2", Pass, ""},
		{"passk-fall", Fail, "stub/csv-export: pass^5 0.00 fell below the baseline 1.00"},
	} {
		v, reasons := Compare(readFixture(t, tc.name), base, cfg)
		if v != tc.want || (tc.reason != "" && !strings.Contains(strings.Join(reasons, "\n"), tc.reason)) {
			t.Errorf("%s: %s %v, want %s with %q", tc.name, v, reasons, tc.want, tc.reason)
		}
	}
	// Without a baseline only the absolute bar applies: 4.0 and pass^5 = 1 pass it.
	if v, r := Compare(readFixture(t, "drop-0.4"), nil, cfg); v != Pass {
		t.Errorf("drop-0.4 with no baseline: %s %v", v, r)
	}
}

func harness(t *testing.T, fake *models.Fake, judge string) *Harness {
	t.Helper()
	cfg, err := LoadConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Judge = judge
	log, err := actions.Open(filepath.Join(t.TempDir(), "actions.jsonl"), actions.Options{})
	if err != nil {
		t.Fatal(err)
	}
	m := JudgeModels(config.Models{}, judge)
	return &Harness{Root: root, Config: cfg, Assets: aayudha.Assets, Gateway: &models.Gateway{
		Models: m, Resolver: models.Resolver{Models: m}, Actions: log,
		Key:    func(string) (string, error) { return "k", nil },
		Client: func(models.Route, string) models.Client { return fake },
	}}
}

func TestTier2WithFakeModels(t *testing.T) {
	cases, err := LoadCases(root)
	if err != nil {
		t.Fatal(err)
	}
	item, _ := os.ReadFile(filepath.Join(root, "evals", "cases", "stub", "csv-export", "output", "issue.json"))
	var replies []models.Response
	for i := 0; i < 5; i++ {
		score := `{"score": 5, "reason": "covers the request"}`
		if i == 4 {
			score = `{"score": 3, "reason": "vague"}`
		}
		replies = append(replies, models.Response{Text: string(item), TokensIn: 900, TokensOut: 300}, models.Response{Text: score, TokensIn: 1500, TokensOut: 40})
	}
	fake := &models.Fake{Replies: replies}
	res, err := harness(t, fake, "gpt-6.1-sol").Tier2(context.Background(), cases, "abc", "dev", time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Cases) != 1 || len(res.Cases[0].Runs) != 5 || len(fake.Requests) != 10 {
		t.Fatalf("want one tier-2 case with 5 runs and 10 model calls, got %+v, %d calls", res.Cases, len(fake.Requests))
	}
	c := res.Cases[0]
	if c.Model != "claude-haiku-5-5" || fake.Requests[1].Model != "gpt-6.1-sol" || c.PassK != 0 || c.Mean != 4.6 || res.CostUSD <= 0 {
		t.Fatalf("results: %+v cost %.4f, judge request model %s", c, res.CostUSD, fake.Requests[1].Model)
	}
	if !strings.Contains(fake.Requests[0].Messages[0].Content, `<untrusted source="eval-input">`) {
		t.Fatal("eval input reached the skill model unwrapped")
	}
}

func TestTier2RefusesSameFamilyJudge(t *testing.T) {
	cases, _ := LoadCases(root)
	fake := &models.Fake{}
	if _, err := harness(t, fake, "claude-sonnet-5-5").Tier2(context.Background(), cases, "abc", "dev", time.Now()); err == nil || len(fake.Requests) != 0 {
		t.Fatalf("an anthropic judge for an anthropic skill model: err %v, %d calls", err, len(fake.Requests))
	}
}

func TestTier2SpendCap(t *testing.T) {
	cases, _ := LoadCases(root)
	item, _ := os.ReadFile(filepath.Join(root, "evals", "cases", "stub", "csv-export", "output", "issue.json"))
	fake := &models.Fake{Replies: []models.Response{{Text: string(item), CostUSD: 0.4}, {Text: `{"score": 5, "reason": "ok"}`, CostUSD: 0.4}}}
	h := harness(t, fake, "gpt-6.1-sol")
	h.Config.MaxUSD = 1.0
	if _, err := h.Tier2(context.Background(), cases, "abc", "dev", time.Now()); err == nil || !strings.Contains(err.Error(), "eval spend cap reached") || len(fake.Requests) != 3 {
		t.Fatalf("spend cap: %v after %d calls, want the cap after 3 (1.2 USD > 1.0)", err, len(fake.Requests))
	}
}
