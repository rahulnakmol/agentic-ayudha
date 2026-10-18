package evals

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	aayudha "github.com/rahulnakmol/agentic-ayudha"
	"github.com/rahulnakmol/agentic-ayudha/internal/models"
	"github.com/rahulnakmol/agentic-ayudha/internal/survey"
)

func surveyCase(t *testing.T, id string) (Case, string) {
	t.Helper()
	all, err := LoadCases(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range all {
		if c.ID == id {
			b, err := os.ReadFile(filepath.Join(c.Dir, filepath.FromSlash(c.Survey.Draft)))
			if err != nil {
				t.Fatal(err)
			}
			return c, string(b)
		}
	}
	t.Fatalf("no case %s", id)
	return Case{}, ""
}

// WP5.9: the survey floors fail the replies a model plausibly writes when it
// invents: an answer nobody gave, a product claim read off the code, a line
// the file does not have, a target the person did not name, a number nothing
// measures, and backlog work the person ruled out in the interview.
func TestSurveyFloors(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct{ id, old, new, want string }{
		{"survey/tinyjs-rust", `"status": "open"`, `"status": "answered", "answer": "A C ABI is enough."`, "no person was asked in this session"},
		{"survey/tinyjs-rust", `"claim": "The README addresses developers of applications that need a simple, familiar scripting language with no dependencies beyond the normal C++ libraries.",
      "basis": "stated"`, `"claim": "Game studios embed it.",
      "basis": "observed"`, "a product claim cannot be observed"},
		{"survey/tinyjs-rust", `"Script.cpp:93-95"`, `"Script.cpp:930-950"`, "Script.cpp"},
		{"survey/tinyjs-rust", `"target": "Rust"`, `"target": "Go"`, `the request does not name "Go"`},
		{"survey/tinyjs-rust", `slow loops are accepted.`, `slow loops are accepted, though it runs 10x faster than Duktape.`, "scope: adds a performance figure"},
		// The case-set and capability-map floors on the stage-1 misses: Math cases under the
		// command-line runner, which never registers Math, and host variable access left out of the map.
		{"survey/tinyjs-rust", `"title": "Command-line runner output on each test script followed by print(result)"`, `"title": "Command-line runner output on each test script, and Math.sqrt of a negative number"`, "calls Math.sqrt, which runner entry:Script.cpp:74 does not register"},
		{"survey/tinyjs-rust", `"TinyJS.h:320-328"`, `"TinyJS.h:327-328"`, "TinyJS.h:321 (method CTinyJS::getScriptVariable), 323 (method CTinyJS::getVariable), 325 (method CTinyJS::setVariable)"},
		{"survey/tinyjs-backlog", `"title": "Port the parser and execution to Rust"`, `"title": "Port the parser and fix the prefix increment operator"`, "scope: adds a fix for a quirk the person kept"},
		{"survey/tinyjs-backlog", `"title": "Port the built-in and Math functions to Rust"`, `"title": "Port the built-ins and add a time limit for each script"`, "scope: adds limits for scripts the person said are trusted"},
		{"survey/tinyjs-backlog", `"E14"`, `"Q5"`, "basis Q5 is a question that is not answered yet"},
		// D-5.9i: probe facts, contract levels, and the do-not-carry entries the person confirmed.
		{"survey/maruku-compat", `"probe:check-differs:headers"`, `"probe:check-differs:easy"`, "probe:check-differs:easy: no such probe fact"},
		{"survey/maruku-compat", `"level": "undecided"`, `"level": "exact"`, "K1"},
		{"survey/maruku-backlog", `"title": "Render tables, footnotes, and header attribute lists as Maruku did"`, `"title": "Render tables and footnotes, and support <?maruku ?> instructions"`, "scope: adds a do-not-carry entry the person confirmed"},
		{"survey/maruku-backlog", `"title": "Render tables, footnotes, and header attribute lists as Maruku did"`, `"title": "Render tables, footnotes, and abbreviations as Maruku did"`, "scope: adds syntax the person said no page uses"},
		// Controls: the recorded replies pass, including a scope line that keeps a quirk out
		// and a Decision whose options name the alternative it rejects.
		{"survey/tinyjs-rust", "", "", ""},
		{"survey/parsedown-modern-php", "", "", ""},
		{"survey/tinyjs-backlog", "", "", ""},
		{"survey/minpack-traps", "", "", ""},
		{"survey/maruku-compat", "", "", ""},
		{"survey/maruku-backlog", "", "", ""},
	} {
		c, reply := surveyCase(t, tc.id)
		if tc.old != "" {
			if !strings.Contains(reply, tc.old) {
				t.Fatalf("%s: the recorded reply has no %q", tc.id, tc.old)
			}
			reply = strings.Replace(reply, tc.old, tc.new, 1)
		}
		w, err := surveySetup(ctx, c, aayudha.Assets)
		if err != nil {
			t.Fatal(err)
		}
		got := checkSurvey(c, w, reply)
		if (tc.want == "") != (len(got) == 0) || !strings.Contains(strings.Join(got, "\n"), tc.want) {
			t.Errorf("%s with %q: %q, want %q", tc.id, tc.new, got, tc.want)
		}
	}
}

// selfCheckReply is a corrections reply (D-5.9n) that keeps every item of
// draft except the verdicts given by id.
func selfCheckReply(t *testing.T, draft string, but map[string]string) string {
	t.Helper()
	root, err := decodeOrdered([]byte(draft))
	if err != nil {
		t.Fatal(err)
	}
	items := map[string]slot{}
	var order []string
	index(root.(*object), "", items, map[string]*list{}, map[string]slot{}, &order, func(string, ...any) {})
	var vs []string
	for _, id := range order {
		if v, ok := but[id]; ok {
			vs = append(vs, v)
		} else {
			vs = append(vs, `{"id": "`+id+`", "verdict": "keep"}`)
		}
	}
	return `{"verdicts": [` + strings.Join(vs, ", ") + `]}`
}

// A run whose last judgement errors keeps what it produced: the first draft and
// its score, the self-check, the answer, and each call's cost (survey run
// 39070364531 lost all of it to a gateway 502 at the judge).
func TestTier2SurveyErroredRunKeepsItsWork(t *testing.T) {
	c, good := surveyCase(t, "survey/tinyjs-rust")
	c.Survey.Rubrics = "" // the single-call path, which the other draft cases take
	fake := &models.Fake{Replies: []models.Response{
		{Text: good, CostUSD: 0.25}, {Text: `{"score": 3, "reason": "first"}`}, {Text: selfCheckReply(t, good, nil)},
		{Text: "the judge is not answering in JSON"}, // the final judgement, twice: the run errors
	}}
	h := harness(t, fake, "gpt-6.1-sol")
	h.Config.K = 1
	res, err := h.Tier2(context.Background(), []Case{c}, "abc", "dev", time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	r := res.Cases[0].Runs[0]
	if r.Status != RunError || r.Score != 0 || r.Error == "" {
		t.Fatalf("not an errored run: %+v", r)
	}
	if r.SelfCheck == nil || r.SelfCheck.FirstScore != 3 || r.SelfCheck.FirstDraft != good || r.Answer == "" {
		t.Errorf("the errored run lost its work: self-check %+v, answer %.40q", r.SelfCheck, r.Answer)
	}
	if len(r.Calls) != 5 || r.Calls[0].CostUSD != 0.25 {
		t.Errorf("calls %+v: want all five, the first costing 0.25 USD", r.Calls)
	}
}

// D-5.9l, D-5.9n: a survey reply that fails the floors is revised once; the
// draft that passes is judged for the record, then the skill's self-check
// replies with corrections, which apply to that draft all or nothing; the
// floors run on the result (one revision for a reply that does not apply or
// a result that fails them), and only the corrected draft is scored.
func TestTier2SurveyRevisesOnceThenJudges(t *testing.T) {
	c, good := surveyCase(t, "survey/tinyjs-rust")
	c.Survey.Rubrics = "" // the single-call path, which the other draft cases take
	bad := strings.Replace(good, `"target": "Rust"`, `"target": "Go"`, 1)
	root, err := decodeOrdered([]byte(good))
	if err != nil {
		t.Fatal(err)
	}
	items := map[string]slot{}
	index(root.(*object), "", items, map[string]*list{}, map[string]slot{}, new([]string), func(string, ...any) {})
	e1 := string(encodeOrdered(items["E1"].get()))
	narrowedE1 := strings.Replace(e1, "The interpreter is one class, CTinyJS,", "The interpreter's entry point is the class CTinyJS,", 1)
	if narrowedE1 == e1 {
		t.Fatal("the recorded draft has no E1 to narrow")
	}
	narrow := selfCheckReply(t, good, map[string]string{"E1": `{"id": "E1", "verdict": "replace", "why": "one class is not shown", "item": ` + narrowedE1 + `}`})
	dangling := selfCheckReply(t, good, map[string]string{"E5": `{"id": "E5", "verdict": "remove"}`}) // M1 still rests on E5
	unknown := `{"verdicts": [{"id": "E99", "verdict": "keep"}]}`
	judge := func(n int) models.Response {
		return models.Response{Text: fmt.Sprintf(`{"score": %d, "reason": "r%d"}`, n, n)}
	}
	replies := []models.Response{
		{Text: bad, CachedIn: 900, CacheWriteIn: 40}, {Text: "```json\n" + good + "```"}, judge(2), {Text: narrow}, judge(4), // revised, then self-checked
		{Text: good}, judge(3), {Text: dangling}, {Text: narrow}, judge(5), // the corrected draft fails the floors once
		{Text: good}, judge(3), {Text: unknown}, {Text: good}, // corrections that never apply: a floor miss, whatever the first draft scored
		{Text: good}, judge(4), {Text: selfCheckReply(t, good, nil)}, judge(4),
		{Text: bad}, {Text: bad}, // refused twice before any self-check
	}
	fake := &models.Fake{Replies: replies}
	h := harness(t, fake, "gpt-6.1-sol")
	res, err := h.Tier2(context.Background(), []Case{c}, "abc", "dev", time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	runs := res.Cases[0].Runs
	if len(fake.Requests) != len(replies) {
		t.Fatalf("%d calls, want %d: runs %+v", len(fake.Requests), len(replies), runs)
	}
	for i, want := range []struct {
		score, first int
		changed      string
		revised      bool
	}{{4, 2, "E1 replaced", false}, {5, 3, "E1 replaced", true}, {1, 3, "", true}, {4, 4, "", false}} {
		r := runs[i]
		if r.Score != want.score || r.SelfCheck == nil || r.SelfCheck.FirstScore != want.first || r.SelfCheck.Revised != want.revised ||
			strings.Join(r.SelfCheck.Changed, ",") != want.changed || r.SelfCheck.FirstDraft != good || r.SelfCheck.Corrections == "" {
			t.Errorf("run %d: score %d, self-check %+v; want score %d, first %d, changed %q, revised %v", i+1, r.Score, r.SelfCheck, want.score, want.first, want.changed, want.revised)
		}
	}
	if !runs[0].Revised || runs[1].Revised || !strings.Contains(runs[2].Floor, "after the self-check: corrections not applied") ||
		runs[4].Score != 1 || runs[4].SelfCheck != nil || !strings.Contains(runs[4].Floor, `does not name "Go"`) {
		t.Errorf("runs %+v", runs)
	}
	first, revise, judgedFirst, selfCheck, judged := fake.Requests[0], fake.Requests[1], fake.Requests[2], fake.Requests[3], fake.Requests[4]
	if !first.Stream || !revise.Stream || !selfCheck.Stream {
		t.Error("the survey skill call does not stream")
	}
	for _, want := range []string{`<untrusted source="aayu survey and aayu survey --module">`, "$ aayu survey --module mod:(root)#2", "marker marker:TinyJS.cpp:111",
		`<untrusted source="Script.cpp">`, "74: int main(int argc, char **argv)", "TinyJS.cpp, lines 1-200 of 2200", `<untrusted source="request">`, "leave every question open"} {
		if !strings.Contains(first.Messages[0].Content, want) {
			t.Errorf("the skill prompt lacks %q", want)
		}
	}
	if !strings.Contains(first.System, "## references/formats.md") || !strings.Contains(first.System, "## references/modernisation.md") {
		t.Error("the skill prompt lacks the case's references")
	}
	if len(revise.Messages) != 3 || !strings.Contains(revise.Messages[2].Content, `the request does not name "Go"`) {
		t.Errorf("revision request: %+v", revise.Messages)
	}
	// Prompt caching: the case's shared prefix and the draft end in breakpoints,
	// every call of the case carries its cache key, and the run records the
	// cache reads and writes.
	if !first.Messages[0].CacheBreak || !selfCheck.Messages[0].CacheBreak || !selfCheck.Messages[1].CacheBreak || selfCheck.Messages[2].CacheBreak {
		t.Error("cache breakpoints are not after the shared prefix and the draft")
	}
	for i, r := range fake.Requests {
		if r.CacheKey != "aayu-eval-survey/tinyjs-rust" {
			t.Fatalf("call %d: cache key %q", i, r.CacheKey)
		}
	}
	if c := runs[0].Calls[0]; c.CachedIn != 900 || c.CacheWriteIn != 40 {
		t.Errorf("call log: %+v", c)
	}
	// The self-check continues from the draft that passed, not the refused one, and asks for corrections only.
	if n := len(selfCheck.Messages); n != 3 || strings.TrimSpace(selfCheck.Messages[1].Content) != strings.TrimSpace(good) ||
		!strings.Contains(selfCheck.Messages[2].Content, "Reply with your corrections only") {
		t.Errorf("self-check request: %d messages", n)
	}
	// A result that fails the floors goes back with the findings and asks for the corrections again.
	again := fake.Requests[8].Messages
	if last := again[len(again)-1].Content; !strings.Contains(last, "M1") || !strings.Contains(last, "E5") || !strings.Contains(last, "whole corrections object again") {
		t.Errorf("floor revision of the corrections: %.600s", last)
	}
	for _, j := range []models.Request{judgedFirst, judged} {
		if !strings.Contains(j.System, "Rubric: survey draft of a legacy C++ interpreter") || !strings.Contains(j.Messages[0].Content, "74: int main(int argc, char **argv)") {
			t.Errorf("judge request: %.300s", j.Messages[0].Content)
		}
	}
	if !strings.Contains(judged.Messages[0].Content, "entry point is the class CTinyJS") || strings.Contains(judgedFirst.Messages[0].Content, "entry point is the class CTinyJS") {
		t.Error("the judge did not score the first draft, then the corrected one")
	}
}

// Each criterion of a sectioned case's rubric.md moves whole to a section
// rubric, so splitting the judgement never drops a bar; and every section
// rubric keeps the fabrication rule.
func TestSurveySectionRubricsKeepEveryCriterion(t *testing.T) {
	for _, id := range []string{"survey/tinyjs-rust", "survey/minpack-manual"} {
		t.Run(id, func(t *testing.T) { sectionRubricsKeepEveryCriterion(t, id) })
	}
}

func sectionRubricsKeepEveryCriterion(t *testing.T, id string) {
	c, _ := surveyCase(t, id)
	if c.Survey.Rubrics == "" {
		t.Fatalf("%s is not judged by section", id)
	}
	whole, err := os.ReadFile(filepath.Join(c.Dir, c.Tier2.Rubric))
	if err != nil {
		t.Fatal(err)
	}
	var parts []rubricItem
	for _, name := range survey.SectionNames() {
		b, err := os.ReadFile(filepath.Join(c.Dir, c.Survey.Rubrics, name+".md"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(strings.Join(strings.Fields(string(b)), " "), "1 when it contains a fabrication (a claim the cited code does not support, a user or customer the repository never names, or a number nothing measures)") {
			t.Errorf("%s.md lacks the fabrication rule", name)
		}
		parts = append(parts, rubricItems(string(b))...)
	}
	n := 0
	for _, it := range rubricItems(string(whole)) {
		if len(it.words) == 0 || it.words[0] < "1" || it.words[0] > "9" {
			continue // the heading and the shared paragraphs: each section rubric words its own scale
		}
		n++
		it.words = it.words[1:] // the criterion, not its number
		if !keptItem(it, parts) {
			t.Errorf("criterion not kept by any section rubric: %.80s", it.text)
		}
	}
	if n != 5 {
		t.Errorf("%d criteria in rubric.md, want 5", n)
	}
}

// A sectioned survey run writes each section in its own call, at its own cap
// and effort, judged against its own rubric; a section that stops at its cap
// reruns alone at twice it; the score loses a point per point a section
// loses; and a rerun with the saved steps resumes every finished section
// without a call.
func TestTier2SurveySections(t *testing.T) {
	c, good := surveyCase(t, "survey/tinyjs-rust")
	parts, err := survey.SplitDraft([]byte(good))
	if err != nil {
		t.Fatal(err)
	}
	judge := func(n int) models.Response {
		return models.Response{Text: fmt.Sprintf(`{"score": %d, "reason": "r%d"}`, n, n)}
	}
	final := map[string]int{"engineering": 5, "capabilities": 4, "product": 4, "modernisation": 5, "questions": 5}
	var replies []models.Response
	for _, name := range survey.SectionNames() {
		if name == "engineering" {
			replies = append(replies, models.Response{Text: string(parts[name][:200]), Truncated: true, StopReason: "max_tokens"})
		}
		replies = append(replies, models.Response{Text: string(parts[name])}, judge(3), models.Response{Text: selfCheckReply(t, string(parts[name]), nil)}, judge(final[name]))
	}
	fake := &models.Fake{Replies: replies}
	h := harness(t, fake, "gpt-6.1-sol")
	h.Config.K, h.Steps = 1, t.TempDir()
	res, err := h.Tier2(context.Background(), []Case{c}, "abc", "dev", time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	r := res.Cases[0].Runs[0]
	if len(fake.Requests) != len(replies) {
		t.Fatalf("%d calls, want %d: %+v", len(fake.Requests), len(replies), r)
	}
	if r.Status != RunFail || r.Score != 3 || len(r.Sections) != 5 || r.Answer == "" {
		t.Fatalf("run: status %s, score %d (want 3: two sections at 4), %d sections, floor %q, error %q", r.Status, r.Score, len(r.Sections), r.Floor, r.Error)
	}
	eng := r.Sections[0]
	if eng.Name != "engineering" || eng.Truncations != 1 || eng.MaxTokens != 64000 || eng.FirstScore != 3 || eng.Score != 5 {
		t.Errorf("engineering: %+v; want one truncation, rerun at 64000, first 3, scored 5", eng)
	}
	if got := fake.Requests[0]; got.MaxTokens != 32000 || got.Effort != "high" || !got.Messages[0].CacheBreak || !strings.Contains(got.Messages[1].Content, "Write the engineering section") {
		t.Errorf("first engineering call: cap %d, effort %q", got.MaxTokens, got.Effort)
	}
	if got := fake.Requests[1]; got.MaxTokens != 64000 {
		t.Errorf("the truncated section reran at %d, want 64000", got.MaxTokens)
	}
	capCall, capJudge := fake.Requests[5], fake.Requests[6]
	if capCall.MaxTokens != 8000 || capCall.Effort != "medium" || !strings.Contains(capCall.Messages[1].Content, `"engineering"`) ||
		capCall.Messages[0].Content != fake.Requests[0].Messages[0].Content {
		t.Errorf("capabilities call: cap %d, effort %q; it must share the prefix and see the engineering section", capCall.MaxTokens, capCall.Effort)
	}
	if !strings.Contains(capJudge.System, "the capabilities section") || !strings.Contains(capJudge.Messages[0].Content, "The sections it rests on") {
		t.Errorf("capabilities judge: %.200s", capJudge.System)
	}
	for _, s := range r.Sections {
		if s.TokensOut == 0 && s.CostUSD == 0 && len(r.Calls) == 0 {
			t.Errorf("section %s logged nothing", s.Name)
		}
	}
	// Resume: every section was saved done, so a rerun makes no call.
	again := &models.Fake{}
	h2 := harness(t, again, "gpt-6.1-sol")
	h2.Config.K, h2.Steps = 1, h.Steps
	res, err = h2.Tier2(context.Background(), []Case{c}, "abc", "dev", time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	if r2 := res.Cases[0].Runs[0]; len(again.Requests) != 0 || r2.Score != 3 || !r2.Sections[4].Resumed || len(r2.Calls) != 0 {
		t.Errorf("resume: %d calls, score %d, sections %+v", len(again.Requests), r2.Score, r2.Sections)
	}
	// ProviderRetries is the sample's, not each section's: two 429s spent in
	// engineering leave none for capabilities, whose next fault errors the run.
	noPause(t)
	limited := &models.StatusError{Code: 429, Body: "rate limited"}
	client := &resetting{Fake: &models.Fake{Replies: replies}, fail: map[int]error{1: limited, 2: limited, 8: limited, 9: limited, 10: limited}}
	h3 := harness(t, nil, "gpt-6.1-sol")
	h3.Config.K = 1
	h3.Gateway.Client = func(models.Route, string) models.Client { return client }
	res, err = h3.Tier2(context.Background(), []Case{c}, "abc", "dev", time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	r3 := res.Cases[0].Runs[0]
	if r3.Status != RunError || runRetries(r3.Calls) != ProviderRetries {
		t.Errorf("retries across sections: status %s, %d retries (want %d for the whole sample), error %q", r3.Status, runRetries(r3.Calls), ProviderRetries, r3.Error)
	}
}

// A case whose manual is fetched at run time (survey_docs.go) does not run
// when the fetch fails or the file differs from its pin: no model call, no
// score, no spend, and no failure of the group. Tier 1 never downloads and is
// n/a without the cached file; a file that matches its pin is downloaded once
// and then read from the cache; text that differs from its pin does not run.
func TestSurveyDocsNotRun(t *testing.T) {
	ctx := context.Background()
	c, _ := surveyCase(t, "survey/minpack-manual")
	pdf := []byte("%PDF-1.4 not the guide\n")
	gets := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gets++
		if r.URL.Path == "/missing" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(pdf)
	}))
	defer srv.Close()
	t.Setenv(DocsCacheEnv, t.TempDir())
	if r := Tier1(ctx, root, c, aayudha.Assets); status(r, "survey") != NA || gets != 0 {
		t.Fatalf("tier 1 with an empty cache: survey %s, %d downloads; want n/a and none", status(r, "survey"), gets)
	}
	with := func(url, sum string) Case {
		d := *c.Survey.Docs
		d.URL = url
		if sum != "" {
			d.SHA256 = sum
		}
		s := *c.Survey
		s.Docs = &d
		cc := c
		cc.Survey = &s
		return cc
	}
	for _, tc := range []struct{ path, want string }{{"/missing", "HTTP 404"}, {"/guide.pdf", "the case pins " + c.Survey.Docs.SHA256}} {
		fake := &models.Fake{}
		h := harness(t, fake, "gpt-6.1-sol")
		h.Config.K = 1
		res, err := h.Tier2(ctx, []Case{with(srv.URL+tc.path, "")}, "abc", "dev", time.Unix(0, 0))
		if err != nil {
			t.Fatal(err)
		}
		cr := res.Cases[0]
		if !strings.Contains(cr.NotRun, tc.want) || len(cr.Runs) != 0 || len(fake.Requests) != 0 || res.CostUSD != 0 {
			t.Errorf("%s: not run %q (want %q), %d runs, %d model calls, %.2f USD", tc.path, cr.NotRun, tc.want, len(cr.Runs), len(fake.Requests), res.CostUSD)
		}
		if v, reasons := Compare(res, nil, h.Config); v != Pass || res.Scored() {
			t.Errorf("%s: a group with only a case that did not run: verdict %s %v, scored %v; want pass, unscored (no results file)", tc.path, v, reasons, res.Scored())
		}
		// Beside a case that ran, the one that did not is not scored.
		res.Cases = append(res.Cases, CaseResult{Case: "survey/other", PassK: 1, Mean: 5, Runs: []Run{{Status: RunPass, Score: 5, Pass: true}}})
		res.PassK, res.Mean = 1, 5
		if v, reasons := Compare(res, nil, h.Config); v != Pass || !res.Scored() {
			t.Errorf("%s: a case that did not run counts against the group: %v, scored %v", tc.path, reasons, res.Scored())
		}
		// Against a baseline, the drop is measured over the cases that ran: the
		// other case fell from 5 to 4 while the one that did not run scored 1
		// there, so the whole baseline's mean (3) would hide the drop.
		base := &Results{SHA: "base", Mean: 3, Cases: []CaseResult{
			{Case: "survey/other", PassK: 1, Mean: 5, Runs: []Run{{Status: RunPass, Score: 5, Pass: true}}},
			{Case: c.ID, PassK: 0, Mean: 1, Runs: []Run{{Status: RunFail, Score: 1}}},
		}}
		res.Cases[1].Runs[0].Score, res.Cases[1].Mean, res.Mean = 4, 4, 4
		if v, reasons := Compare(res, base, h.Config); v != Fail || !strings.Contains(strings.Join(reasons, "; "), "below the baseline 5.00") {
			t.Errorf("%s: a drop from 5 to 4 on the cases that ran: %s %v", tc.path, v, reasons)
		}
		// A case new since the baseline is out of the drop too: other fell from
		// 5 to 4.5 while the new case scored 5, so the whole mean (4.75) would
		// hide a drop of 0.5.
		newer := &Results{SHA: "cur", Mean: 4.75, Cases: []CaseResult{
			{Case: "survey/other", PassK: 1, Mean: 4.5, Runs: []Run{{Status: RunPass, Score: 4, Pass: true}, {Status: RunPass, Score: 5, Pass: true}}},
			{Case: "survey/new", PassK: 1, Mean: 5, Runs: []Run{{Status: RunPass, Score: 5, Pass: true}, {Status: RunPass, Score: 5, Pass: true}}},
		}}
		if v, reasons := Compare(newer, &Results{SHA: "base", Mean: 5, Cases: base.Cases[:1]}, h.Config); v != Fail || !strings.Contains(strings.Join(reasons, "; "), "4.50 is 0.50 below the baseline 5.00") {
			t.Errorf("%s: a drop hidden by a new case: %s %v", tc.path, v, reasons)
		}
	}
	matching := with(srv.URL+"/guide.pdf", sumHex(pdf)).Survey.Docs
	gets = 0
	for range 2 {
		if err := matching.fetch(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if gets != 1 {
		t.Errorf("%d downloads of a cached manual, want 1", gets)
	}
	manuals := t.TempDir()
	if err := os.WriteFile(filepath.Join(manuals, matching.Name+".txt"), []byte("other text\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := matching.checkText(manuals); !strings.Contains(fmt.Sprint(err), "not run: pdftotext's text") {
		t.Errorf("text that differs from its pin: %v", err)
	}
}
