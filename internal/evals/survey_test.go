package evals

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	aayudha "github.com/rahulnakmol/agentic-ayudha"
	"github.com/rahulnakmol/agentic-ayudha/internal/models"
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

// D-5.9l, D-5.9n: a survey reply that fails the floors is revised once; the
// draft that passes is judged for the record, then the skill's self-check
// replies with corrections, which apply to that draft all or nothing; the
// floors run on the result (one revision for a reply that does not apply or
// a result that fails them), and only the corrected draft is scored.
func TestTier2SurveyRevisesOnceThenJudges(t *testing.T) {
	c, good := surveyCase(t, "survey/tinyjs-rust")
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
		{Text: bad}, {Text: "```json\n" + good + "```"}, judge(2), {Text: narrow}, judge(4), // revised, then self-checked
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
