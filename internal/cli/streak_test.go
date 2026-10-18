package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/autonomy"
	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
)

// streakRepo serves three merged PRs, newest first: #3 a clean bot PR on
// internal/cli (L02) that the merge gate's log shows reviewed by another
// family (rung A), #2 a bot PR on internal/cli and internal/github (L02,
// L06) with a changes-requested review, and #1 a human's PR on internal/cli.
func streakRepo(t *testing.T) *fake.GitHub {
	t.Helper()
	g := fake.New(t, "o/r", "User")
	bot := map[string]any{"login": "aayudha[bot]", "type": "Bot"}
	pr := func(n int, user map[string]any, merged string) map[string]any {
		return map[string]any{"number": n, "title": "feat: change", "user": user, "merged_at": merged, "merge_commit_sha": strings.Repeat("a", 39) + string(rune('0'+n))}
	}
	withHead := func(p map[string]any, sha string) map[string]any {
		p["head"] = map[string]any{"sha": sha}
		return p
	}
	commit := func(sha string, author map[string]any, msg string) map[string]any {
		return map[string]any{"sha": sha, "author": author, "commit": map[string]any{"message": msg}, "parents": []any{map[string]any{"sha": "p"}}}
	}
	file := func(names ...string) []any {
		var out []any
		for _, n := range names {
			out = append(out, map[string]any{"filename": n})
		}
		return out
	}
	g.Static = map[string]any{
		"/repos/o/r/pulls": []any{
			withHead(pr(3, bot, "2026-10-03T10:00:00Z"), "d3"), pr(2, bot, "2026-10-02T10:00:00Z"),
			pr(1, map[string]any{"login": "founder", "type": "User"}, "2026-10-01T10:00:00Z"),
			map[string]any{"number": 4, "title": "closed unmerged", "user": bot},
		},
		"/repos/o/r/pulls/3/commits":   []any{commit("c3", bot, "feat: x\n\nAayudha-Checkpoint: 1/1")},
		"/repos/o/r/pulls/3/files":     file("internal/cli/a.go"),
		"/repos/o/r/pulls/3/reviews":   []any{},
		"/repos/o/r/pulls/2/commits":   []any{commit("c2", bot, "feat: y\n\nAayudha-Checkpoint: 1/1")},
		"/repos/o/r/pulls/2/files":     file("internal/cli/b.go", "internal/github/c.go"),
		"/repos/o/r/issues/3/comments": []any{},
		"/repos/o/r/issues/2/comments": []any{},
		"/repos/o/r/pulls/2/reviews":   []any{map[string]any{"user": map[string]any{"login": "founder", "type": "User"}, "state": "CHANGES_REQUESTED", "submitted_at": "2026-10-02T09:00:00Z"}},
	}
	g.Runs = []fake.WorkflowRun{{ID: 801, Path: ".github/workflows/" + autonomy.MergeGateWorkflow, HeadSHA: "d3", Event: "pull_request_target", Status: "completed", Conclusion: "success"}}
	g.RunJobs = map[int64][]fake.CheckRun{801: {{ID: 8011, Name: "aayudha/inspect", Status: "completed", Conclusion: "success"}}}
	g.JobLogs = map[int64]string{8011: "2026-10-03T09:00:00.0000000Z inspect #3 at d3: merge (reviewed by claude[bot])\n2026-10-03T09:00:00.0000000Z   review: different-family review (A): claude (anthropic), builder openai\n"}
	return g
}

// status --streak computes per-layer streaks from merged-PR history; with
// --apply it appends only new corrections to the ledger, and only from CI
// with the bot token (D1).
func TestStatusStreak(t *testing.T) {
	g := streakRepo(t)
	dir, bare := ledgerCheckout(t)
	opts := Options{GitHubAPI: g.URL(), GitHubGap: -1, Dir: dir}
	code, out, errOut := run(t, map[string]string{"AAYU_GITHUB_TOKEN": "t"}, opts, "status", "--streak", "--repo", "o/r", "--json")
	if code != 0 {
		t.Fatalf("status --streak: exit %d: %s", code, errOut)
	}
	var rep streakReport
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatal(err)
	}
	got := map[string]autonomy.LayerStreak{}
	for _, s := range rep.Layers {
		got[s.Layer] = s
	}
	if rep.Scanned != 3 || len(rep.Corrections) != 1 || rep.Corrections[0].PR != 2 || rep.Corrections[0].Kind != autonomy.KindChangesRequested ||
		got["L02"].Streak != 1 || got["L02"].Merges != 2 || got["L02"].LastBreak != 2 || got["L06"].Streak != 0 || got["L01"].Merges != 0 ||
		rep.Rungs["A"] != 1 || rep.Rungs[""] != 1 || len(rep.Rungs) != 2 {
		t.Fatalf("report: %+v", rep)
	}

	if code, _, _ := run(t, map[string]string{"AAYU_GITHUB_TOKEN": "t"}, opts, "status", "--streak", "--apply", "--repo", "o/r"); code != ExitUsage {
		t.Errorf("--apply outside CI: exit %d, want %d", code, ExitUsage)
	}
	ci := map[string]string{"GITHUB_TOKEN": "workflow", HaltVariable: ""}
	if code, _, _ := run(t, ci, opts, "status", "--streak", "--apply", "--ci", "--repo", "o/r"); code != ExitBlocked {
		t.Errorf("--apply in CI without the App token: exit %d, want %d", code, ExitBlocked)
	}
	ci["AAYU_BOT_TOKEN"] = "app"
	for i, want := range []int{1, 0} {
		code, out, errOut := run(t, ci, opts, "status", "--streak", "--apply", "--ci", "--repo", "o/r", "--json")
		rep = streakReport{}
		_ = json.Unmarshal([]byte(out), &rep)
		if code != 0 || rep.Recorded != want {
			t.Fatalf("apply run %d: exit %d, recorded %d, want %d: %s", i+1, code, rep.Recorded, want, errOut)
		}
		g.Files = map[string]string{CorrectionsPath: git(t, bare, "show", "ledger:"+CorrectionsPath) + "\n"}
	}
	if lines := strings.Split(g.Files[CorrectionsPath], "\n"); len(lines) != 2 || !strings.Contains(lines[0], `"pr":2,"kind":"changes-requested"`) {
		t.Errorf("ledger %s:\n%s", CorrectionsPath, g.Files[CorrectionsPath])
	}
}
