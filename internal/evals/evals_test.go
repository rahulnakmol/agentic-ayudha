package evals

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	aayudha "github.com/rahulnakmol/agentic-ayudha"
	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/models"
	"github.com/rahulnakmol/agentic-ayudha/internal/planning"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
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

// WP5.3 accept: one grill case per work-item type (ten), each recording
// `grill <type> … --create` and, except for intake types, which are never
// made Ready (D-5.3a), `ready <issue> --check` on the issue it wrote; tier 1
// passes on all ten.
func TestTier1GrillEveryType(t *testing.T) {
	cases, err := LoadCases(root)
	if err != nil {
		t.Fatal(err)
	}
	covered := map[string]bool{}
	for _, c := range cases {
		if c.Skill != "grill" || len(c.Calls) == 0 || len(c.Calls[0]) < 2 || c.Calls[0][0] != "grill" {
			continue
		}
		typ := c.Calls[0][1]
		ty, ok := workitems.FindType(typ)
		if !ok || !slices.Contains(c.Calls[0], "--create") {
			t.Errorf("%s: first call %v is not a grill --create of a work-item type", c.ID, c.Calls[0])
			continue
		}
		readied := len(c.Calls) > 1 && c.Calls[len(c.Calls)-1][0] == "ready"
		if readied == ty.Intake {
			t.Errorf("%s: a %s case must %s end with ready --check", c.ID, typ, map[bool]string{true: "not", false: ""}[ty.Intake])
		}
		if r := Tier1(context.Background(), root, c, aayudha.Assets); !r.Passed() {
			t.Errorf("tier 1 failed:\n%s", r)
		}
		covered[typ] = true
	}
	for _, id := range workitems.TypeIDs() {
		if !covered[id] {
			t.Errorf("no grill case for type %s", id)
		}
	}
	if len(covered) != 10 {
		t.Errorf("%d types covered; want 10", len(covered))
	}
}

// stubCases loads the stub skill's cases: the tier-2 tests below script
// their fake model replies for exactly those.
func stubCases(root string) ([]Case, error) {
	all, err := LoadCases(root)
	var out []Case
	for _, c := range all {
		if c.Skill == "stub" {
			out = append(out, c)
		}
	}
	return out, err
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
	cfg.MaxUSD = 6 // a group's cap; the fakes charge almost nothing
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
	cases, err := stubCases(root)
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
	if c.Model != "claude-opus-5-5" || c.Lane != "spec" || c.Fallback != "claude-sonnet-5-5" || fake.Requests[1].Model != "gpt-6.1-sol" || c.PassK != 0 || c.Mean != 4.6 || res.CostUSD <= 0 {
		t.Fatalf("results: %+v cost %.4f, judge request model %s", c, res.CostUSD, fake.Requests[1].Model)
	}
	if !strings.Contains(fake.Requests[0].Messages[0].Content, `<untrusted source="eval-input">`) {
		t.Fatal("eval input reached the skill model unwrapped")
	}
}

func TestTier2RefusesSameFamilyJudge(t *testing.T) {
	cases, _ := stubCases(root)
	fake := &models.Fake{}
	if _, err := harness(t, fake, "claude-sonnet-5-5").Tier2(context.Background(), cases, "abc", "dev", time.Now()); err == nil || len(fake.Requests) != 0 {
		t.Fatalf("an anthropic judge for an anthropic skill model: err %v, %d calls", err, len(fake.Requests))
	}
}

func TestTier2SpendCap(t *testing.T) {
	cases, _ := stubCases(root)
	item, _ := os.ReadFile(filepath.Join(root, "evals", "cases", "stub", "csv-export", "output", "issue.json"))
	fake := &models.Fake{Replies: []models.Response{{Text: string(item), CostUSD: 0.4}, {Text: `{"score": 5, "reason": "ok"}`, CostUSD: 0.4}}}
	h := harness(t, fake, "gpt-6.1-sol")
	h.Config.MaxUSD = 1.0
	res, err := h.Tier2(context.Background(), cases, "abc", "dev", time.Now())
	if err != nil || len(fake.Requests) != 3 {
		t.Fatalf("spend cap: %v after %d calls, want the cap after 3 (1.2 USD > 1.0)", err, len(fake.Requests))
	}
	runs := res.Cases[0].Runs
	if runs[0].Status != RunPass || runs[1].Status != RunError || !strings.Contains(runs[4].Error, "eval spend cap reached") || res.Cases[0].Errors != 4 {
		t.Fatalf("runs after the cap: %+v", runs)
	}
	if v, _ := Compare(res, nil, h.Config); v != Fail {
		t.Fatal("a capped run passed")
	}
}

// judgeReplies builds fake replies for k runs: a valid skill output, then the
// judge's reply (or replies) for that run.
func judgeReplies(t *testing.T, judge func(run int) []string) []models.Response {
	t.Helper()
	item, err := os.ReadFile(filepath.Join(root, "evals", "cases", "stub", "csv-export", "output", "issue.json"))
	if err != nil {
		t.Fatal(err)
	}
	var out []models.Response
	for i := 0; i < 5; i++ {
		out = append(out, models.Response{Text: string(item)})
		for _, j := range judge(i) {
			out = append(out, models.Response{Text: j})
		}
	}
	return out
}

// A judge reply with prose around a fenced JSON block is scored, without a retry.
func TestJudgeFencedReplyWithProse(t *testing.T) {
	cases, _ := stubCases(root)
	fake := &models.Fake{Replies: judgeReplies(t, func(int) []string {
		return []string{"Here is my assessment.\n\n```json\n{\"score\": 5, \"reason\": \"all three requirements covered\"}\n```\n\nLet me know if you need more."}
	})}
	res, err := harness(t, fake, "gpt-6.1-sol").Tier2(context.Background(), cases, "abc", "dev", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	c := res.Cases[0]
	if len(fake.Requests) != 10 || c.Errors != 0 || c.PassK != 1 || c.Mean != 5 || c.Runs[0].Status != RunPass {
		t.Fatalf("%d calls, case %+v", len(fake.Requests), c)
	}
	if f := fake.Requests[1].Format; f == nil || f.Name != "rubric_score" || !f.Strict {
		t.Fatalf("judge was not asked for structured output: %+v", f)
	}
	if v, _ := Compare(res, nil, harness(t, fake, "gpt-6.1-sol").Config); v != Pass {
		t.Fatalf("verdict %s", v)
	}
}

// A judge reply that cannot be parsed twice makes the run an error, with a
// redacted excerpt; more than one error in k fails the tier outright, while a
// single error leaves pass^k and the mean to the completed runs.
func TestJudgeUnparseableTwice(t *testing.T) {
	cases, _ := stubCases(root)
	const leak = "ghp_abcdefghijklmnopqrstuvwxyz0123456789"
	garbage := "I would rate this highly overall. " + leak
	ok := `{"score": 4, "reason": "fine"}`
	for _, tc := range []struct {
		name      string
		badRuns   int
		verdict   string
		passK     float64
		completed int
	}{
		{"one of five", 1, Pass, 1, 4},
		{"two of five", 2, Fail, 1, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := &models.Fake{Replies: judgeReplies(t, func(run int) []string {
				if run < tc.badRuns {
					return []string{garbage, garbage} // the judgement and its one retry
				}
				return []string{ok}
			})}
			h := harness(t, fake, "gpt-6.1-sol")
			res, err := h.Tier2(context.Background(), cases, "abc", "dev", time.Now())
			if err != nil {
				t.Fatal(err)
			}
			c := res.Cases[0]
			bad := c.Runs[0]
			if bad.Status != RunError || bad.Score != 0 || !strings.Contains(bad.Excerpt, "I would rate this highly") ||
				strings.Contains(bad.Excerpt, leak) || !strings.Contains(bad.Excerpt, "[REDACTED]") {
				t.Fatalf("errored run: %+v", bad)
			}
			if want := 5*2 + tc.badRuns; len(fake.Requests) != want {
				t.Fatalf("%d model calls, want %d (one retry per bad judgement)", len(fake.Requests), want)
			}
			if c.Errors != tc.badRuns || c.PassK != tc.passK || c.Mean != 4 || len(c.Runs)-c.Errors != tc.completed {
				t.Fatalf("case: %+v", c)
			}
			v, reasons := Compare(res, nil, h.Config)
			if v != tc.verdict || (tc.verdict == Fail && !strings.Contains(strings.Join(reasons, "\n"), "2 of 5 runs errored")) {
				t.Fatalf("verdict %s %v", v, reasons)
			}
		})
	}
}

// The live failure on 35aea36: the skill model's reply was not JSON twice. That
// is an error, not a score of 1; JSON of the wrong shape stays a skill failure.
func TestSkillReplyErrors(t *testing.T) {
	cases, _ := stubCases(root)
	fake := &models.Fake{Replies: []models.Response{{Text: ""}, {Text: "```json\n{\"type\": \"story\", \"title\""},
		{Text: `{"type": "story"}`}, {Text: `{"type": "story"}`}}}
	h := harness(t, fake, "gpt-6.1-sol")
	h.Config.K = 2
	res, err := h.Tier2(context.Background(), cases, "abc", "dev", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	runs := res.Cases[0].Runs
	if runs[0].Status != RunError || runs[0].Excerpt == "" || runs[1].Status != RunFail || runs[1].Score != 1 {
		t.Fatalf("runs: %+v", runs)
	}
}

// invented is the csv-export output with the two requirements the 2-scoring
// live run added (evals.yml run 38184730879): a content type and a column order.
func invented(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, "evals", "cases", "stub", "csv-export", "output", "issue.json"))
	if err != nil {
		t.Fatal(err)
	}
	return strings.Replace(string(b), "expected: the CSV header lists exactly the visible columns, without Region",
		"expected: the response has Content-Type text/csv and the CSV header lists exactly the visible columns in the displayed order, without Region", 1)
}

// A recorded output that invents requirements fails tier 1 on the case's scope
// rules, while it still passes the schema and Ready checks.
func TestTier1InventedRequirement(t *testing.T) {
	c := copyCase(t, "stub/csv-export")
	if err := os.WriteFile(filepath.Join(c.Dir, "output", "issue.json"), []byte(invented(t)), 0o644); err != nil {
		t.Fatal(err)
	}
	r := Tier1(context.Background(), root, c, aayudha.Assets)
	out := r.String()
	if status(r, "scope") != Fail || status(r, "ready") != Pass || status(r, "schemas") != Pass ||
		!strings.Contains(out, "adds a content type the request does not state") || !strings.Contains(out, "adds an ordering the request does not state") {
		t.Fatalf("invented requirements not caught:\n%s", out)
	}
}

// In tier 2 the same guard fails the run before the judge is asked; open
// questions are not requirements and reach the judge as such.
func TestTier2ScopeGuardAndOpenQuestions(t *testing.T) {
	cases, _ := stubCases(root)
	withQuestions := strings.Replace(invented(t), `"parent"`, `"open_questions": ["Should the CSV keep the list's column order?"], "parent"`, 1)
	good, _ := os.ReadFile(filepath.Join(root, "evals", "cases", "stub", "csv-export", "output", "issue.json"))
	goodWithQuestions := strings.Replace(string(good), `"parent"`, `"open_questions": ["Is a UTF-8 byte order mark needed for Excel?"], "parent"`, 1)
	fake := &models.Fake{Replies: []models.Response{{Text: withQuestions}, {Text: goodWithQuestions}, {Text: `{"score": 5, "reason": "ok"}`}}}
	h := harness(t, fake, "gpt-6.1-sol")
	h.Config.K = 2
	res, err := h.Tier2(context.Background(), cases, "abc", "dev", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	runs := res.Cases[0].Runs
	if runs[0].Status != RunFail || runs[0].Score != 1 || !strings.Contains(runs[0].Floor, "out of scope: adds a content type") {
		t.Fatalf("invented requirement: %+v", runs[0])
	}
	if runs[1].Status != RunPass || len(fake.Requests) != 3 {
		t.Fatalf("run 2 %+v after %d calls (the judge must be asked only for run 2)", runs[1], len(fake.Requests))
	}
	if j := fake.Requests[2].Messages[0].Content; !strings.Contains(j, "Open questions the skill left for a human") || !strings.Contains(j, "byte order mark") {
		t.Fatalf("judge prompt lacks the open questions:\n%s", j)
	}
}

// Every call's stop reason and tokens land in the results; a skill reply that
// stopped at the cap is an error marked truncated, without a retry.
func TestTier2RecordsStopsAndTruncation(t *testing.T) {
	cases, _ := stubCases(root)
	good, _ := os.ReadFile(filepath.Join(root, "evals", "cases", "stub", "csv-export", "output", "issue.json"))
	long := `{"type": "story", "title": "Download", "fields": {"summary": "` + strings.Repeat("and the list ", 60)
	fake := &models.Fake{Replies: []models.Response{
		{Text: long, StopReason: "max_tokens", Truncated: true, TokensIn: 1800, TokensOut: 3000},
		{Text: string(good), StopReason: "end_turn", TokensIn: 1800, TokensOut: 610},
		{Text: `{"score": 5, "reason": "ok"}`, StopReason: "completed", TokensIn: 2600, TokensOut: 90},
	}}
	h := harness(t, fake, "gpt-6.1-sol")
	h.Config.K = 2
	res, err := h.Tier2(context.Background(), cases, "abc", "dev", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	r1, r2 := res.Cases[0].Runs[0], res.Cases[0].Runs[1]
	if r1.Status != RunError || !strings.HasPrefix(r1.Error, "truncated: ") || len(r1.Calls) != 1 || !r1.Calls[0].Truncated ||
		r1.Calls[0].StopReason != "max_tokens" || r1.Calls[0].TokensOut != 3000 || r1.Calls[0].MaxTokens != h.Config.MaxTokens.Skill ||
		!strings.Contains(r1.Excerpt, " … ") || !strings.HasSuffix(r1.Excerpt, "and the list") {
		t.Fatalf("truncated run: %+v", r1)
	}
	if r2.Status != RunPass || len(r2.Calls) != 2 || r2.Calls[0].Role != "skill" || r2.Calls[0].TokensOut != 610 ||
		r2.Calls[1].Role != "judge" || r2.Calls[1].StopReason != "completed" || r2.Calls[1].Model != "gpt-6.1-sol" {
		t.Fatalf("completed run calls: %+v", r2.Calls)
	}
}

// The line between conventions of the requested format and invented specifics:
// a CSV header row passes the csv-export scope rules; each invented specific fails.
func TestScopeConventionVersusInvented(t *testing.T) {
	const anchor = "expected: the CSV header lists exactly the visible columns, without Region"
	for _, tc := range []struct {
		name, expected, why string
	}{
		{"header row is a convention", "expected: the CSV starts with a header row naming exactly the visible columns, without Region", ""},
		{"exact column order", "expected: the CSV columns follow the screen order exactly, without Region", "an ordering"},
		{"content type", "expected: the response is served as text/csv with exactly the visible columns", "a content type"},
		{"encoding", "expected: the file is UTF-8 and lists exactly the visible columns", "a file format detail"},
		{"file name", "expected: the file name is claims-export.csv and it lists the visible columns", "a file format detail"},
		{"size limit", "expected: the export holds at most 50,000 rows of the visible columns", "a size limit"},
		{"duration", "expected: the CSV with the visible columns is ready within 5 seconds", "a time limit"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := copyCase(t, "stub/csv-export")
			p := filepath.Join(c.Dir, "output", "issue.json")
			b, _ := os.ReadFile(p)
			if !strings.Contains(string(b), anchor) {
				t.Fatal("fixture anchor moved")
			}
			_ = os.WriteFile(p, []byte(strings.Replace(string(b), anchor, tc.expected, 1)), 0o644)
			r := Tier1(context.Background(), root, c, aayudha.Assets)
			if tc.why == "" {
				if !r.Passed() {
					t.Fatalf("a convention was flagged:\n%s", r)
				}
				return
			}
			if status(r, "scope") != Fail || !strings.Contains(r.String(), "adds "+tc.why) {
				t.Fatalf("invented %s not caught:\n%s", tc.name, r)
			}
		})
	}
}

// flaky fails every call to one model with an endpoint error and answers the rest from a fake.
type flaky struct {
	down string
	fake *models.Fake
}

func (f flaky) Complete(ctx context.Context, r models.Request) (models.Response, error) {
	if r.Model == f.down {
		return models.Response{}, &models.StatusError{Code: 404, Body: `{"error":"model not found"}`}
	}
	return f.fake.Complete(ctx, r)
}

// The skill model comes from the registry lane of the verb the stub stands in
// for (grill: spec); when that model cannot be called, the lane's registry
// alternative takes the call.
func TestTier2SkillModelFromVerbLaneWithFallback(t *testing.T) {
	cases, _ := stubCases(root)
	fake := &models.Fake{Replies: judgeReplies(t, func(int) []string { return []string{`{"score": 5, "reason": "ok"}`} })}
	h := harness(t, fake, "gpt-6.1-sol")
	h.Gateway.Client = func(models.Route, string) models.Client { return flaky{down: "claude-opus-5-5", fake: fake} }
	res, err := h.Tier2(context.Background(), cases, "abc", "dev", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	c := res.Cases[0]
	calls := c.Runs[0].Calls
	if c.Model != "claude-opus-5-5" || c.PassK != 1 || len(calls) != 3 ||
		calls[0].Model != "claude-opus-5-5" || calls[0].Error == "" || calls[1].Model != "claude-sonnet-5-5" || calls[2].Role != "judge" {
		t.Fatalf("case %s pass^k %v, calls %+v", c.Model, c.PassK, calls)
	}
}

func TestTier2NeedsAVerbLane(t *testing.T) {
	c := copyCase(t, "stub/csv-export")
	c.Tier2.Verb = ""
	if _, err := harness(t, &models.Fake{}, "gpt-6.1-sol").Tier2(context.Background(), []Case{c}, "abc", "dev", time.Now()); err == nil || !strings.Contains(err.Error(), "set [tier2] verb") {
		t.Fatalf("a stub without a verb: %v", err)
	}
}

// WP7.3 register evals: a brief case gets its references in the prompt; a
// draft that fails `aayu brief`'s checks (a planted 30-word sentence in the
// ste register) is revised once with the findings, as the skill does, and is
// judged only when it passes; a draft that fails twice scores 1 without the
// judge (rule 7), and the per-case bar then fails the tier.
func TestTier2BriefRevisesOnceThenJudges(t *testing.T) {
	all, err := LoadCases(root)
	if err != nil {
		t.Fatal(err)
	}
	var cases []Case
	for _, c := range all {
		if c.ID == "brief/ste-explain" {
			cases = append(cases, c)
		}
	}
	good, err := os.ReadFile(filepath.Join(root, "evals", "cases", "brief", "ste-explain", "output", "brief.md"))
	if err != nil || len(cases) != 1 {
		t.Fatalf("case: %v", err)
	}
	bad := "# Export of filtered claims\n\nThe user can open the list and see all the items that the filter shows, and the team can check each item in the same list before the export starts [P1].\n"
	judge := models.Response{Text: `{"score": 5, "reason": "grounded"}`}
	replies := []models.Response{{Text: bad}, {Text: "```markdown\n" + string(good) + "```"}, judge}
	for i := 0; i < 3; i++ {
		replies = append(replies, models.Response{Text: string(good)}, judge)
	}
	replies = append(replies, models.Response{Text: bad}, models.Response{Text: bad}, judge)
	fake := &models.Fake{Replies: replies}
	h := harness(t, fake, "gpt-6.1-sol")
	res, err := h.Tier2(context.Background(), cases, "abc", "dev", time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	runs := res.Cases[0].Runs
	if len(fake.Requests) != 11 || !runs[0].Revised || runs[0].Status != RunPass || runs[1].Revised || runs[4].Score != 1 || !strings.Contains(runs[4].Floor, "has 30 words") {
		t.Fatalf("%d calls, runs %+v", len(fake.Requests), runs)
	}
	first, revise, judged := fake.Requests[0], fake.Requests[1], fake.Requests[2]
	if !strings.Contains(first.System, "## references/ste-words.md") || !strings.Contains(first.Messages[0].Content, `<untrusted source="grounding">`) {
		t.Error("the skill prompt lacks the word list or the wrapped grounding")
	}
	if len(revise.Messages) != 3 || !strings.Contains(revise.Messages[2].Content, "sentence 1 has 30 words (at most 20)") {
		t.Errorf("revision request: %+v", revise.Messages)
	}
	if judged.Model != "gpt-6.1-sol" || !strings.Contains(judged.Messages[0].Content, "STE lint") {
		t.Errorf("judge request: %s %.200s", judged.Model, judged.Messages[0].Content)
	}
	if v, reasons := Compare(res, nil, h.Config); v != Fail || !strings.Contains(strings.Join(reasons, "\n"), "brief/ste-explain: pass^5 0.00 is below 0.80") {
		t.Errorf("compare: %s %v", v, reasons)
	}
}

// The grill cases' tier 2 (WP5.3) runs the grill skill with its question set
// appended, on grill's own lane: the recorded Goal scores against the rubric.
func TestTier2GrillReferences(t *testing.T) {
	c, err := LoadCase(filepath.Join(root, "evals", "cases", "grill", "goal-first-claim"))
	if err != nil {
		t.Fatal(err)
	}
	item, _ := os.ReadFile(filepath.Join(c.Dir, "output", "issue.json"))
	fake := &models.Fake{Replies: []models.Response{{Text: string(item)}, {Text: `{"score": 5, "reason": "ok"}`}}}
	h := harness(t, fake, "gpt-6.1-sol")
	h.Config.K = 1
	res, err := h.Tier2(context.Background(), []Case{c}, "abc", "dev", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if res.Cases[0].Lane != "spec" || res.Cases[0].Runs[0].Score != 5 {
		t.Fatalf("results: %+v", res.Cases[0])
	}
	sys := fake.Requests[0].System
	if !strings.Contains(sys, "# grill\n") || !strings.Contains(sys, "## references/goal.md") || !strings.Contains(sys, "### Baseline (`baseline`, required)") {
		t.Fatalf("the skill model did not get the grill skill with references/goal.md:\n%s", sys)
	}
}

// WP5.4 tier 2: slice and plan drafts go through `aayu slice` and `aayu
// plan`'s checks against the case's fixture first; one that fails them scores
// 1 without the judge (rule 7), one that passes is judged.
func TestTier2PlanningDrafts(t *testing.T) {
	for _, tc := range []struct {
		id, floor string
		breakIt   func(raw []byte) []byte
	}{
		{"slice/proof-retention-6-layer", "effort:XL blocks Ready", func(raw []byte) []byte {
			var d planning.SliceDraft
			_ = json.Unmarshal(raw, &d)
			d.Stories[0].Labels = []string{"effort:XL"}
			b, _ := json.Marshal(d)
			return b
		}},
		{"plan/config-migration", "needs a rollback step", func(raw []byte) []byte {
			var d planning.PlanDraft
			_ = json.Unmarshal(raw, &d)
			d.Checkpoints[0].Rollback = ""
			b, _ := json.Marshal(d)
			return b
		}},
	} {
		c, err := LoadCase(filepath.Join(root, "evals", "cases", filepath.FromSlash(tc.id)))
		if err != nil {
			t.Fatal(err)
		}
		good, _ := os.ReadFile(filepath.Join(c.Dir, "output", c.Tier2.Draft+".json"))
		fake := &models.Fake{Replies: []models.Response{{Text: string(good)}, {Text: `{"score": 5, "reason": "ok"}`}, {Text: string(tc.breakIt(good))}}}
		h := harness(t, fake, "gpt-6.1-sol")
		h.Config.K = 2
		res, err := h.Tier2(context.Background(), []Case{c}, "abc", "dev", time.Now())
		if err != nil {
			t.Fatal(err)
		}
		runs := res.Cases[0].Runs
		if runs[0].Score != 5 || runs[1].Score != 1 || !strings.Contains(runs[1].Floor, tc.floor) || len(fake.Requests) != 3 {
			t.Errorf("%s: runs %+v, %d calls", tc.id, runs, len(fake.Requests))
		}
		if !strings.Contains(fake.Requests[0].System, "Reply with the draft you produce as JSON: {"+map[string]string{"slice": "feature", "plan": "story"}[c.Tier2.Draft]) {
			t.Errorf("%s: the skill was not asked for its draft", tc.id)
		}
	}
}

// WP7.3 evidence rule, tier 1: in brief/unmapped-criterion the pack maps no
// evidence to criterion 3. The recorded draft says it is not proved and
// passes; a draft that claims the recording proves it fails, and so does one
// that cites the missing map entry.
func TestTier1UnmappedCriterionClaim(t *testing.T) {
	for draft, want := range map[string]string{
		"# Export\n\nTwo criteria are proved [P1.c1, P1.c2].\n\n- The recording proves criterion 3 [P1].\n": "claims proof of criterion 3, but no pack maps evidence to it",
		"# Export\n\nTwo criteria are proved [P1.c1, P1.c2].\n\n- Criterion 3 holds [P1.c3].\n":             "[P1.c3] is not a source",
	} {
		c := copyCase(t, "brief/unmapped-criterion")
		// The shared grounding sits beside the case directory, as in evals/cases/brief/.
		g, err := os.ReadFile(filepath.Join(root, "evals", "cases", "brief", "grounding-unmapped.json"))
		if err != nil || os.WriteFile(filepath.Join(c.Dir, "..", "grounding-unmapped.json"), g, 0o644) != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(c.Dir, "output", "brief.md"), []byte(draft), 0o644); err != nil {
			t.Fatal(err)
		}
		r := Tier1(context.Background(), root, c, aayudha.Assets)
		if status(r, "brief") != Fail || !strings.Contains(r.String(), want) {
			t.Errorf("planted claim did not fail tier 1 with %q:\n%s", want, r)
		}
	}
	all, _ := LoadCases(root)
	for _, c := range all {
		if c.ID == "brief/unmapped-criterion" {
			if r := Tier1(context.Background(), root, c, aayudha.Assets); !r.Passed() || c.Tier2 != nil {
				t.Fatalf("recorded draft: %s (tier2 %v)", r, c.Tier2)
			}
		}
	}
}

// Tier-2 groups (D-7.3d): the brief cases form their own group with a 12 USD
// cap and every other case falls to the catch-all group with 6, so one
// group cannot use up another's cap; an unknown group is refused.
func TestGroups(t *testing.T) {
	cfg, err := LoadConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	all, err := LoadCases(root)
	if err != nil {
		t.Fatal(err)
	}
	b, brief, err := cfg.ForGroup("brief", all)
	if err != nil || b.MaxUSD != 12 {
		t.Fatalf("brief group: %v, cap %v", err, b.MaxUSD)
	}
	o, other, err := cfg.ForGroup("skills", all)
	if err != nil || o.MaxUSD != 6 || len(brief)+len(other) != len(all) {
		t.Fatalf("skills group: %v, cap %v, %d+%d of %d cases", err, o.MaxUSD, len(brief), len(other), len(all))
	}
	for _, c := range brief {
		if !strings.HasPrefix(c.ID, "brief/") {
			t.Errorf("%s in the brief group", c.ID)
		}
	}
	for _, c := range other {
		if strings.HasPrefix(c.ID, "brief/") {
			t.Errorf("%s outside the brief group", c.ID)
		}
	}
	if _, _, err := cfg.ForGroup("", all); err == nil {
		t.Error("an empty group was accepted")
	}
}

// WP7.3 brief rules, tier 1 (D-7.3d, D-7.3e): planted drafts on the recorded cases
// fail the deterministic checks the skill's lapses map to: a close brief
// without its Limits section, a rate from the Result stated for "each
// reviewer" or without "per reviewer", an interface control no source
// names, absence written as fact, and a count read as a behaviour.
func TestTier1BriefRules(t *testing.T) {
	for _, tc := range []struct{ id, draft, check, want string }{
		{"brief/regulatory-close", "# Closure record\n\nFeature #7 met its target of 3, at 4.2 exports per reviewer per week [R].\n\n## Verification\n\n- The merge gate wrote the authoritative pack [P1].\n", "limits", "`## Limits of this evidence` section"},
		{"brief/eli5-explain", "# Saving claims\n\nEach reviewer saves 4.2 files a week [R].\n", "population", `says "each reviewer"`},
		{"brief/eli5-explain", "# Saving claims\n\nReviewers save files 4.2 times a week [R].\n", "population", `keep "per reviewer"`},
		{"brief/eli5-explain", "# Saving claims\n\nPress one button to save the claims you picked [#12].\n", "ui", `names "Press", an interface detail no source names`},
		// The three lapses of run 38210971285 (D-7.3e), each quoted from a judged draft.
		{"brief/eli5-explain", "# Saving claims\n\nOther lists can't be saved this way yet [#7].\n", "absence", `says "can't be"`},
		{"brief/eli5-explain", "# Saving claims\n\nThis count shows that people use the file [R].\n", "scope", "a behaviour behind the count the Result does not measure"},
		{"brief/engineer-ideate", "# CSV export\n\nIts behaviour at larger sizes is untested [P1.c3].\n", "absence", `says "untested"`},
	} {
		c := copyCase(t, tc.id)
		g, err := os.ReadFile(filepath.Join(root, "evals", "cases", "brief", "grounding.json"))
		if err != nil || os.WriteFile(filepath.Join(c.Dir, "..", "grounding.json"), g, 0o644) != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(c.Dir, "output", "brief.md"), []byte(tc.draft), 0o644); err != nil {
			t.Fatal(err)
		}
		r := Tier1(context.Background(), root, c, aayudha.Assets)
		if status(r, "brief") != Fail || !strings.Contains(r.String(), tc.check+": ") || !strings.Contains(r.String(), tc.want) {
			t.Errorf("%s: planted %s defect did not fail tier 1 with %q:\n%s", tc.id, tc.check, tc.want, r)
		}
	}
}

// An evolve case's old run is its output/ artifacts read from the reproduce
// directory, a brief draft included; other paths stay.
func TestCaseBefore(t *testing.T) {
	c := Case{ID: "brief/x", Reproduce: "before", Input: "in.json", Artifacts: []string{"output/a.json", "fixtures/b.json"}, Ready: []string{"output/a.json"},
		Schemas: []Schema{{File: "output/a.json", Schema: "s.json"}}, Brief: &BriefSpec{Register: "ste", Purpose: "explain", Draft: "output/brief.md"}}
	b := c.Before()
	if strings.Join(b.Artifacts, ",") != "before/a.json,fixtures/b.json" || b.Ready[0] != "before/a.json" || b.Schemas[0].File != "before/a.json" ||
		b.Brief.Draft != "before/brief.md" || c.Brief.Draft != "output/brief.md" || c.Artifacts[0] != "output/a.json" {
		t.Fatalf("before: %+v %+v", b, b.Brief)
	}
}
