package main

import (
	"context"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func write(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for p, c := range files {
		full := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// evolveCase is a case for the stub skill that reproduces Feedback #5: its
// before/ run (the current skill) adds a content type the request does not
// state, which the case's scope rule catches; its output/ run (the new skill)
// does not.
func evolveCase(t *testing.T, beforeTitle string) map[string]string {
	t.Helper()
	repo := filepath.Join("..", "..", "..", "..")
	issue, err := os.ReadFile(filepath.Join(repo, "evals", "cases", "stub", "csv-export", "output", "issue.json"))
	if err != nil {
		t.Fatal(err)
	}
	before := strings.Replace(string(issue), `"title": "`, `"title": "`+beforeTitle, 1)
	return map[string]string{
		"evals/cases/stub/no-content-type/case.toml": `skill = "stub"
description = "Feedback #5: the stub skill invents a content type."
input = "input.txt"
artifacts = ["output/issue.json"]
schemas = [{ file = "output/issue.json", schema = "evals/schemas/workitem.json" }]
ready = ["output/issue.json"]
scope = [{ pattern = '(?i)\btext/csv\b', why = "a content type the request does not state" }]
feedback = 5
reproduce = "before"
`,
		"evals/cases/stub/no-content-type/input.txt":         "Let finance export the orders list as CSV.\n",
		"evals/cases/stub/no-content-type/output/issue.json": string(issue),
		"evals/cases/stub/no-content-type/before/issue.json": before,
	}
}

// WP7.7 accept: an evolve PR without a new eval case is rejected by CI (the
// build job runs this gate on every PR); one with a case that reproduces its
// Feedback on the old skill and passes on the new one is not; other PRs are
// not evolve PRs.
func TestEvolveGateCI(t *testing.T) {
	repo := filepath.Join("..", "..", "..", "..")
	schema, err := os.ReadFile(filepath.Join(repo, "evals", "schemas", "workitem.json"))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	git(t, root, "init", "--quiet", "-b", "dev")
	write(t, root, map[string]string{"evals/schemas/workitem.json": string(schema), "evals/skills/stub/SKILL.md": "# stub\n\nDraft a Story.\n"})
	git(t, root, "add", ".")
	git(t, root, "commit", "--quiet", "-m", "base")
	base := git(t, root, "rev-parse", "HEAD")
	skillDiff := map[string]string{"evals/skills/stub/SKILL.md": "# stub\n\nDraft a Story. Never add a content type the request does not state.\n"}
	branch := func(name, msg string, files ...map[string]string) {
		git(t, root, "checkout", "--quiet", "-B", name, base)
		for _, f := range files {
			write(t, root, f)
		}
		git(t, root, "add", ".")
		git(t, root, "commit", "--quiet", "--allow-empty", "-m", msg)
	}
	ctx := context.Background()
	for _, tc := range []struct {
		name, branch, msg string
		files             []map[string]string
		fail              string
	}{
		{"evolve PR with a skill diff and no eval case", "evolve/stub-5", "x", []map[string]string{skillDiff}, "evolve gate"},
		{"evolve commit on another branch with no eval case", "feat/stub", "feat: x\n\nAayudha-Evolve: #5", []map[string]string{skillDiff}, "evolve gate"},
		{"case whose old run is missing", "evolve/stub-5", "x", []map[string]string{withoutBefore(evolveCase(t, "Export orders as text/csv ")), skillDiff}, "evolve gate"},
		{"case whose old run passes does not reproduce", "evolve/stub-5", "x", []map[string]string{evolveCase(t, "Export orders "), skillDiff}, "evolve gate"},
		{"case without a skill diff", "evolve/stub-5", "x", []map[string]string{evolveCase(t, "Export orders as text/csv ")}, "evolve gate"},
		{"case naming no Feedback", "evolve/stub-5", "x", []map[string]string{withCase(evolveCase(t, "Export orders as text/csv "), func(c string) string {
			return strings.Replace(c, "feedback = 5\n", "", 1)
		}), skillDiff}, "evolve gate"},
		{"case without a reproduce run", "evolve/stub-5", "x", []map[string]string{withCase(evolveCase(t, "Export orders as text/csv "), func(c string) string {
			return strings.Replace(c, "reproduce = \"before\"\n", "", 1)
		}), skillDiff}, "evolve gate"},
		{"unreadable case", "evolve/stub-5", "x", []map[string]string{{"evals/cases/stub/broken/case.toml": "skill = \"stub\"\nunknown = 1\n"}, skillDiff}, "evolve gate"},
		{"new run fails tier 1", "evolve/stub-5", "x", []map[string]string{evolveCase(t, "Export orders as text/csv "), skillDiff,
			{"evals/cases/stub/no-content-type/output/issue.json": "{}"}}, "evolve gate"},
		{"eval-first evolve PR", "evolve/stub-5", "feat(skills): evolve stub\n\nAayudha-Evolve: #5", []map[string]string{evolveCase(t, "Export orders as text/csv "), skillDiff}, ""},
		{"not an evolve PR, whatever its labels", "feat/stub", "feat: x", []map[string]string{skillDiff}, ""},
	} {
		branch(tc.branch, tc.msg, tc.files...)
		err := evolveGate(ctx, root, base, "HEAD", tc.branch)
		switch {
		case tc.fail == "" && err != nil:
			t.Errorf("%s: rejected: %v", tc.name, err)
		case tc.fail != "" && (err == nil || !strings.Contains(err.Error(), tc.fail)):
			t.Errorf("%s: not rejected (%v)", tc.name, err)
		}
	}
	if err := evolveGate(ctx, root, "", "HEAD", "evolve/x"); err == nil {
		t.Error("an evolve gate without -base passed")
	}
	if err := evolveGate(ctx, root, "no-such-commit", "HEAD", "evolve/x"); err == nil || !strings.Contains(err.Error(), "git fetch") {
		t.Errorf("an evolve gate against a missing base: %v", err)
	}
	// The command line CI runs.
	branch("evolve/stub-5", "x", skillDiff)
	t.Chdir(root)
	flag.CommandLine = flag.NewFlagSet("eval", flag.ExitOnError)
	os.Args = []string{"eval", "-evolve-gate", "-base", base, "-branch", "evolve/stub-5"}
	if err := run(ctx); err == nil || !strings.Contains(err.Error(), "evolve gate") {
		t.Errorf("eval -evolve-gate on an evolve PR without a case: %v", err)
	}
}

// CI's checkout is the PR's test-merge commit, one commit deep. When GitHub
// recomputes that merge (the base moved) while the run is going, the checked-out
// merge is no longer on any ref, so deepening it cannot fetch HEAD^1 ("git log
// HEAD^1..HEAD: exit status 128", seen on #138). The gate works from the PR's
// base and head SHAs instead and fetches them by SHA, so it does not depend on
// the merge commit. This replays that sequence against a local origin.
func TestEvolveGateAfterTestMergeRecomputed(t *testing.T) {
	repo := filepath.Join("..", "..", "..", "..")
	schema, err := os.ReadFile(filepath.Join(repo, "evals", "schemas", "workitem.json"))
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	bare := filepath.Join(tmp, "origin.git")
	git(t, tmp, "init", "--quiet", "--bare", "-b", "dev", bare)
	git(t, bare, "config", "uploadpack.allowReachableSHA1InWant", "true") // as GitHub allows
	work := filepath.Join(tmp, "work")
	git(t, tmp, "clone", "--quiet", bare, work)
	git(t, work, "checkout", "--quiet", "-b", "dev")
	write(t, work, map[string]string{"evals/schemas/workitem.json": string(schema), "evals/skills/stub/SKILL.md": "# stub\n"})
	git(t, work, "add", ".")
	git(t, work, "commit", "--quiet", "-m", "base")
	base := git(t, work, "rev-parse", "HEAD")
	git(t, work, "checkout", "--quiet", "-b", "evolve/stub-5")
	write(t, work, evolveCase(t, "Export orders as text/csv "))
	write(t, work, map[string]string{"evals/skills/stub/SKILL.md": "# stub\n\nNever add a content type.\n"})
	git(t, work, "add", ".")
	git(t, work, "commit", "--quiet", "-m", "feat(skills): evolve stub\n\nAayudha-Evolve: #5")
	head := git(t, work, "rev-parse", "HEAD")
	merge := func(onto string) string {
		git(t, work, "checkout", "--quiet", "--detach", onto)
		git(t, work, "merge", "--quiet", "--no-ff", "-m", "Merge "+head+" into "+onto, head)
		return git(t, work, "rev-parse", "HEAD")
	}
	git(t, work, "push", "--quiet", "origin", "dev", "evolve/stub-5")
	m1 := merge(base)
	git(t, work, "push", "--quiet", "origin", m1+":refs/pull/1/merge")

	// The run checks out the test merge, one commit deep, as actions/checkout does.
	ci := filepath.Join(tmp, "ci")
	git(t, tmp, "init", "--quiet", ci)
	git(t, ci, "remote", "add", "origin", "file://"+bare)
	git(t, ci, "fetch", "--quiet", "--no-tags", "--depth=1", "origin", "+refs/pull/1/merge:refs/remotes/pull/1/merge")
	git(t, ci, "checkout", "--quiet", "refs/remotes/pull/1/merge")

	// Meanwhile dev moves and GitHub recomputes the merge; the old one becomes unreachable and is pruned.
	git(t, work, "checkout", "--quiet", "dev")
	write(t, work, map[string]string{"README.md": "moved on\n"})
	git(t, work, "add", ".")
	git(t, work, "commit", "--quiet", "-m", "dev moves")
	git(t, work, "push", "--quiet", "origin", "dev")
	git(t, work, "push", "--quiet", "--force", "origin", merge("dev")+":refs/pull/1/merge")
	git(t, bare, "reflog", "expire", "--expire=now", "--all")
	git(t, bare, "gc", "--quiet", "--prune=now")

	// The old step: deepen the merge commit, then read HEAD^1. It cannot.
	old := exec.Command("sh", "-c", `git fetch --no-tags --quiet --deepen=2 origin; git log --format=%B HEAD^1..HEAD`)
	old.Dir = ci
	if out, err := old.CombinedOutput(); err == nil || !strings.Contains(string(out), "HEAD^1") {
		t.Fatalf("the old step did not reproduce the failure: %v\n%s", err, out)
	} else {
		t.Logf("old step, as on #138: %v: %s", err, strings.TrimSpace(string(out)))
	}
	// The new step: base and head SHAs from the event, fetched by SHA.
	t.Chdir(ci)
	flag.CommandLine = flag.NewFlagSet("eval", flag.ExitOnError)
	os.Args = []string{"eval", "-evolve-gate", "-base", base, "-head", head, "-branch", "evolve/stub-5"}
	if err := run(context.Background()); err != nil {
		t.Fatalf("evolve gate on base and head SHAs: %v", err)
	}
	// An evolve PR without a case is still refused this way (by its trailer, on a non-evolve branch).
	git(t, work, "checkout", "--quiet", "-b", "feat/x", base)
	write(t, work, map[string]string{"evals/skills/stub/SKILL.md": "# stub\n\nchanged\n"})
	git(t, work, "commit", "--quiet", "-am", "feat: x\n\nAayudha-Evolve: #5")
	git(t, work, "push", "--quiet", "origin", "feat/x")
	if err := evolveGate(context.Background(), ci, base, git(t, work, "rev-parse", "HEAD"), "feat/x"); err == nil || !strings.Contains(err.Error(), "evolve gate") {
		t.Fatalf("an evolve commit without a case: %v", err)
	}
}

func withoutBefore(files map[string]string) map[string]string {
	delete(files, "evals/cases/stub/no-content-type/before/issue.json")
	return files
}

func withCase(files map[string]string, edit func(string) string) map[string]string {
	k := "evals/cases/stub/no-content-type/case.toml"
	files[k] = edit(files[k])
	return files
}
