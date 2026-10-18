package cli

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/inspect"
	"github.com/rahulnakmol/agentic-ayudha/internal/prbody"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

func hasNote(res inspect.Result, prefix, part string) bool {
	return slices.ContainsFunc(res.Notes, func(s string) bool { return strings.HasPrefix(s, prefix) && strings.Contains(s, part) })
}

// TestPRBodyFloor: an agent's PR whose description lacks a section is held at
// fix whatever the reviewer said, with the section named; once the body is
// fixed the same cached review merges again; a person's PR only gets notes;
// a draft is not checked.
func TestPRBodyFloor(t *testing.T) {
	e := &Env{Options: Options{Assets: os.DirFS(filepath.Join("..", ".."))}}
	chk, issue, cps := storyFixture()
	good := strings.ReplaceAll(storyBody(chk, issue, "", nil, cps, "Nakul"), prbody.Placeholder, "The cart shows the total.")
	bad := strings.Replace(good, "## Why\n", "", 1)
	agent := []*gh.Label{{Name: workitems.LabelAgentAuthored}}

	res := inspect.Result{Disposition: inspect.Merge}
	if !e.prBodyFloor(&res, &gh.PullRequest{Body: &bad, Labels: agent}, nil) {
		t.Error("a new block does not ask for the comment to be written")
	}
	if res.Disposition != inspect.Fix || !strings.Contains(res.Overridden, `"Why"`) || !hasNote(res, prBodyBlocks, `"Why": the section is missing`) {
		t.Fatalf("a missing section on an agent's PR: %s %q %v", res.Disposition, res.Overridden, res.Notes)
	}
	// The stored result is the floored one, with the review's own merge kept
	// in Reviewed; the next run, cached, on the fixed body merges again and
	// drops the last run's notes.
	if res.Reviewed != inspect.Merge {
		t.Fatalf("the review's own disposition is lost: %q", res.Reviewed)
	}
	stored := res
	again := stored
	again.Notes = slices.Clone(stored.Notes)
	if !e.prBodyFloor(&again, &gh.PullRequest{Body: &good, Labels: agent}, &stored) {
		t.Error("a lifted block does not ask for the comment to be written")
	}
	if again.Disposition != inspect.Merge || again.Overridden != "" || hasNote(again, prBodyBlocks, "") {
		t.Fatalf("a fixed body: %s %q %v", again.Disposition, again.Overridden, again.Notes)
	}
	// A floor's fix or a reviewer's rebuild is never softened.
	for _, d := range []string{inspect.Rebuild, NoReply} {
		r := inspect.Result{Disposition: d}
		e.prBodyFloor(&r, &gh.PullRequest{Body: &bad, Labels: agent}, nil)
		if r.Disposition != d || !hasNote(r, prBodyBlocks, "Why") {
			t.Errorf("%s became %s %v", d, r.Disposition, r.Notes)
		}
	}
	// A docs-only skip is held too.
	skip := inspect.Result{Disposition: inspect.Skipped}
	if e.prBodyFloor(&skip, &gh.PullRequest{Body: &bad, Labels: agent}, nil); skip.Disposition != inspect.Fix {
		t.Errorf("a docs-only skip with a bad body: %s", skip.Disposition)
	}

	person := inspect.Result{Disposition: inspect.Merge}
	if e.prBodyFloor(&person, &gh.PullRequest{Body: &bad}, nil); person.Disposition != inspect.Merge || !hasNote(person, prBodyWarns, `"Why"`) {
		t.Fatalf("a person's PR: %s %v", person.Disposition, person.Notes)
	}
	// A bot's PR is an agent's, label or not.
	bot := inspect.Result{Disposition: inspect.Merge}
	if e.prBodyFloor(&bot, &gh.PullRequest{Body: &bad, User: &gh.User{Login: gh.Ptr("aayudha-bot[bot]")}}, nil); bot.Disposition != inspect.Fix {
		t.Errorf("a bot's PR with a bad body: %s", bot.Disposition)
	}
	draft := inspect.Result{Disposition: inspect.Skipped}
	if e.prBodyFloor(&draft, &gh.PullRequest{Body: &bad, Labels: agent, Draft: gh.Ptr(true)}, nil); draft.Disposition != inspect.Skipped || len(draft.Notes) > 0 {
		t.Fatalf("a draft: %s %v", draft.Disposition, draft.Notes)
	}
}

// TestInspectDocsOnlyChecksPRBody: the docs-only short cut still checks the
// description; a person's PR gets the problems as notes and still passes.
func TestInspectDocsOnlyChecksPRBody(t *testing.T) {
	g, env, opts, _ := inspectFixture(t, cleanDiff, "Refs #2")
	docs := g.AddPR("docs: explain Percent", "Docs.", "founder", "docs/percent", strings.Repeat("e", 40), "dev")
	g.Mu.Lock()
	docs.Diff = "diff --git a/docs/guide/percent.md b/docs/guide/percent.md\nnew file mode 100644\n--- /dev/null\n+++ b/docs/guide/percent.md\n@@ -0,0 +1 @@\n+# Percent\n"
	g.Mu.Unlock()
	code, res, errOut := inspectApp(t, env, opts, docs.Number, "--ci")
	if code != 0 || res.Disposition != inspect.Skipped || !hasNote(res, prBodyWarns, `"What this does": the section is missing`) {
		t.Fatalf("docs-only PR by a person: exit %d %s %v %s", code, res.Disposition, res.Notes, errOut)
	}
}
