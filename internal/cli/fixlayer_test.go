package cli

import (
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// `retro --classify` (S-7.7d) records a Feedback's fix layer as a label and a
// comment with the reason. A check correction files a Story whose Completion
// Goal names the test that catches it; reclassifying it to prose swaps the
// label, keeps one comment, and files no second Story. An incomplete record is
// refused before anything is written.
func TestRetroClassify(t *testing.T) {
	g := fake.New(t, "o/r", "User")
	g.AddIssue("Feedback: retro: skill `build` correction rate 2/2 above 20 %", "x", "type:feedback", "status:backlog", "needs:grill") // #1
	g.AddIssue("A story", "x", "type:story", "status:backlog")                                                                         // #2
	env := map[string]string{"AAYU_GITHUB_TOKEN": "t", "XDG_STATE_HOME": t.TempDir()}
	opts := Options{GitHubAPI: g.URL(), GitHubGap: -1}
	classify := func(args ...string) (int, string, string) {
		return run(t, env, opts, append([]string{"retro", "--repo", "o/r", "--classify", "1"}, args...)...)
	}
	crit := "[metric] state: a build that writes outside its worktree; expected: `TestBuildStaysInWorktree` fails"
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"no reason", []string{"--layer", "check", "--criterion", crit}, "needs --reason"},
		{"prose without why code cannot", []string{"--layer", "prose", "--reason", "judgement"}, "needs --not-catchable"},
		{"check without its proof", []string{"--layer", "check", "--reason", "a test can"}, "needs --criterion"},
		{"check whose proof names nothing", []string{"--layer", "check", "--reason", "a test can", "--criterion", "[metric] state: a build; expected: it fails"}, "in backticks"},
		{"malformed criterion", []string{"--layer", "check", "--reason", "a test can", "--criterion", "`TestX` fails"}, "unmapped"},
	} {
		if code, _, errOut := classify(tc.args...); code != ExitUsage || !strings.Contains(errOut, tc.want) {
			t.Errorf("%s: exit %d, %s; want usage, %q", tc.name, code, errOut, tc.want)
		}
	}
	if len(g.Writes()) != 0 {
		t.Fatalf("refused classifications wrote: %v", g.Writes())
	}
	if code, _, errOut := run(t, env, opts, "retro", "--repo", "o/r", "--classify", "2", "--layer", "prose", "--reason", "r", "--not-catchable", "n"); code != ExitUsage || !strings.Contains(errOut, "not a Feedback issue") {
		t.Errorf("classify a Story: exit %d, %s", code, errOut)
	}
	if code, out, errOut := classify("--layer", "check", "--reason", "a test can catch a write outside the worktree", "--criterion", crit); code != 0 || !strings.Contains(out, "fixed by Story #3") {
		t.Fatalf("classify as check: exit %d: %s %s", code, out, errOut)
	}
	g.Mu.Lock()
	story, fb := g.Issues[3], g.Issues[1]
	g.Mu.Unlock()
	it, _ := workitems.Parse(3, story.Title, story.Body, "", g.LabelNames(3))
	cs, ps := workitems.ParseCriteria(it.Fields[workitems.FieldCriteria])
	if it.Type != "story" || len(ps) > 0 || len(cs) != 1 || !strings.Contains(cs[0].Expected, "`TestBuildStaysInWorktree` fails") || !strings.Contains(story.Title, "Feedback #1") {
		t.Errorf("Story #3: type %q, criteria %v %v, title %q", it.Type, cs, ps, story.Title)
	}
	if labels := strings.Join(g.LabelNames(1), ","); !strings.Contains(labels, "fix:check") || len(fb.Comments) != 1 ||
		!strings.Contains(fb.Comments[0].Body, "### Fix layer: static check") || !strings.Contains(fb.Comments[0].Body, "Story #3") {
		t.Fatalf("Feedback #1 after check: labels %s, comments %d", labels, len(fb.Comments))
	}
	if code, out, errOut := classify("--layer", "prose", "--reason", "the wording is judged per request", "--not-catchable", "no rule sees intent"); code != 0 || !strings.Contains(out, "aayu evolve") {
		t.Fatalf("reclassify as prose: exit %d: %s %s", code, out, errOut)
	}
	labels := strings.Join(g.LabelNames(1), ",")
	g.Mu.Lock()
	defer g.Mu.Unlock()
	if strings.Contains(labels, "fix:check") || !strings.Contains(labels, "fix:prose") || len(g.Issues) != 3 ||
		len(fb.Comments) != 1 || !strings.Contains(fb.Comments[0].Body, "no rule sees intent") {
		t.Errorf("Feedback #1 after prose: labels %s, issues %d, comments %d", labels, len(g.Issues), len(fb.Comments))
	}
}
