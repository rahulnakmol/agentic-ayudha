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

// WP5.9: a survey run shows the skill what a host sees (the index summary,
// every module pack, and the files read, numbered, as untrusted data), revises
// once on the checks' findings, and is judged only when it passes; a reply
// refused twice scores 1 without the judge. D-5.9l: a draft that passes is
// judged for the record, then self-checked; only the corrected draft is
// scored, and it must pass the floors too (one revision).
func TestTier2SurveyRevisesOnceThenJudges(t *testing.T) {
	c, good := surveyCase(t, "survey/tinyjs-rust")
	bad := strings.Replace(good, `"target": "Rust"`, `"target": "Go"`, 1)
	narrowed := strings.Replace(good, "The interpreter is one class, CTinyJS,", "The interpreter's entry point is the class CTinyJS,", 1)
	if narrowed == good {
		t.Fatal("the recorded draft has no E1 to narrow")
	}
	judge := func(n int) models.Response {
		return models.Response{Text: fmt.Sprintf(`{"score": %d, "reason": "r%d"}`, n, n)}
	}
	replies := []models.Response{
		{Text: bad}, {Text: "```json\n" + good + "```"}, judge(2), {Text: narrowed}, judge(4), // revised, then self-checked
		{Text: good}, judge(3), {Text: bad}, {Text: good}, judge(5), // the corrected draft fails the floors once
		{Text: good}, judge(3), {Text: bad}, {Text: bad}, // and twice: a floor miss, whatever the first draft scored
		{Text: good}, judge(4), {Text: good}, judge(4),
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
	}{{4, 2, "E1 changed", false}, {5, 3, "", true}, {1, 3, "", true}, {4, 4, "", false}} {
		r := runs[i]
		if r.Score != want.score || r.SelfCheck == nil || r.SelfCheck.FirstScore != want.first || r.SelfCheck.Revised != want.revised ||
			strings.Join(r.SelfCheck.Changed, ",") != want.changed {
			t.Errorf("run %d: score %d, self-check %+v; want score %d, first %d, changed %q, revised %v", i+1, r.Score, r.SelfCheck, want.score, want.first, want.changed, want.revised)
		}
	}
	if !runs[0].Revised || runs[1].Revised || !strings.Contains(runs[2].Floor, "after the self-check") || runs[4].Score != 1 || runs[4].SelfCheck != nil || !strings.Contains(runs[4].Floor, `does not name "Go"`) {
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
	// The self-check continues from the draft that passed, not the refused one.
	if n := len(selfCheck.Messages); n != 3 || strings.TrimSpace(selfCheck.Messages[1].Content) != strings.TrimSpace(good) || !strings.Contains(selfCheck.Messages[2].Content, "self-check") {
		t.Errorf("self-check request: %d messages", n)
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
