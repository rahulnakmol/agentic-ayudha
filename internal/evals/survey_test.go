package evals

import (
	"context"
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
		// Controls: the recorded replies pass, including a scope line that keeps a quirk out.
		{"survey/tinyjs-rust", "", "", ""},
		{"survey/parsedown-modern-php", "", "", ""},
		{"survey/tinyjs-backlog", "", "", ""},
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
// refused twice scores 1 without the judge.
func TestTier2SurveyRevisesOnceThenJudges(t *testing.T) {
	c, good := surveyCase(t, "survey/tinyjs-rust")
	bad := strings.Replace(good, `"target": "Rust"`, `"target": "Go"`, 1)
	judge := models.Response{Text: `{"score": 4, "reason": "cites the code"}`}
	replies := []models.Response{{Text: bad}, {Text: "```json\n" + good + "```"}, judge}
	for i := 0; i < 3; i++ {
		replies = append(replies, models.Response{Text: good}, judge)
	}
	replies = append(replies, models.Response{Text: bad}, models.Response{Text: bad})
	fake := &models.Fake{Replies: replies}
	h := harness(t, fake, "gpt-6.1-sol")
	res, err := h.Tier2(context.Background(), []Case{c}, "abc", "dev", time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	runs := res.Cases[0].Runs
	if len(fake.Requests) != 11 || !runs[0].Revised || runs[0].Score != 4 || runs[1].Revised || runs[4].Score != 1 || !strings.Contains(runs[4].Floor, `does not name "Go"`) {
		t.Fatalf("%d calls, runs %+v", len(fake.Requests), runs)
	}
	first, revise, judged := fake.Requests[0], fake.Requests[1], fake.Requests[2]
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
	if !strings.Contains(judged.System, "Rubric: survey draft of a legacy C++ interpreter") || !strings.Contains(judged.Messages[0].Content, "74: int main(int argc, char **argv)") {
		t.Errorf("judge request: %.300s", judged.Messages[0].Content)
	}
}
