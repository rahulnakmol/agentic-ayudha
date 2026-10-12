package main

import (
	"context"
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
	branch := func(name string, files ...map[string]string) {
		git(t, root, "checkout", "--quiet", "-B", name, base)
		for _, f := range files {
			write(t, root, f)
		}
		git(t, root, "add", ".")
		git(t, root, "commit", "--quiet", "--allow-empty", "-m", name)
	}
	ctx := context.Background()
	for _, tc := range []struct {
		name, branch, labels string
		files                []map[string]string
		fail                 string
	}{
		{"evolve PR with a skill diff and no eval case", "evolve/stub-5", "", []map[string]string{skillDiff}, "evolve gate"},
		{"labelled evolve PR with no eval case", "feat/stub", "agent-authored,aayu:evolve", []map[string]string{skillDiff}, "evolve gate"},
		{"case whose old run passes does not reproduce", "evolve/stub-5", "", []map[string]string{evolveCase(t, "Export orders "), skillDiff}, "evolve gate"},
		{"case without a skill diff", "evolve/stub-5", "", []map[string]string{evolveCase(t, "Export orders as text/csv ")}, "evolve gate"},
		{"case naming no Feedback", "evolve/stub-5", "", []map[string]string{withCase(evolveCase(t, "Export orders as text/csv "), func(c string) string {
			return strings.Replace(c, "feedback = 5\n", "", 1)
		}), skillDiff}, "evolve gate"},
		{"case without a reproduce run", "evolve/stub-5", "", []map[string]string{withCase(evolveCase(t, "Export orders as text/csv "), func(c string) string {
			return strings.Replace(c, "reproduce = \"before\"\n", "", 1)
		}), skillDiff}, "evolve gate"},
		{"unreadable case", "evolve/stub-5", "", []map[string]string{{"evals/cases/stub/broken/case.toml": "skill = \"stub\"\nunknown = 1\n"}, skillDiff}, "evolve gate"},
		{"new run fails tier 1", "evolve/stub-5", "", []map[string]string{evolveCase(t, "Export orders as text/csv "), skillDiff,
			{"evals/cases/stub/no-content-type/output/issue.json": "{}"}}, "evolve gate"},
		{"eval-first evolve PR", "evolve/stub-5", "agent-authored", []map[string]string{evolveCase(t, "Export orders as text/csv "), skillDiff}, ""},
		{"not an evolve PR", "feat/stub", "", []map[string]string{skillDiff}, ""},
	} {
		branch(tc.branch, tc.files...)
		err := evolveGate(ctx, root, base, tc.branch, strings.Split(tc.labels, ","))
		switch {
		case tc.fail == "" && err != nil:
			t.Errorf("%s: rejected: %v", tc.name, err)
		case tc.fail != "" && (err == nil || !strings.Contains(err.Error(), tc.fail)):
			t.Errorf("%s: not rejected (%v)", tc.name, err)
		}
	}
	if err := evolveGate(ctx, root, "", "evolve/x", nil); err == nil {
		t.Error("an evolve gate without -base passed")
	}
	if err := evolveGate(ctx, root, "no-such-commit", "evolve/x", nil); err == nil || !strings.Contains(err.Error(), "git diff") {
		t.Errorf("an evolve gate against a missing base: %v", err)
	}
	// The command line CI runs.
	branch("evolve/stub-5", skillDiff)
	t.Chdir(root)
	os.Args = []string{"eval", "-evolve-gate", "-base", base, "-branch", "evolve/stub-5", "-labels", ""}
	if err := run(ctx); err == nil || !strings.Contains(err.Error(), "evolve gate") {
		t.Errorf("eval -evolve-gate on an evolve PR without a case: %v", err)
	}
}

func withCase(files map[string]string, edit func(string) string) map[string]string {
	k := "evals/cases/stub/no-content-type/case.toml"
	files[k] = edit(files[k])
	return files
}
