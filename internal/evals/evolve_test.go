package evals

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	aayudha "github.com/rahulnakmol/agentic-ayudha"
	"github.com/rahulnakmol/agentic-ayudha/internal/cli"
	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
)

func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Amp", "GIT_AUTHOR_EMAIL=amp@example.com", "GIT_COMMITTER_NAME=Amp", "GIT_COMMITTER_EMAIL=amp@example.com")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeTree(t *testing.T, root string, files map[string]string) {
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

// `aayu evolve` cuts a worktree, refuses a change without a reproducing eval
// case (nothing committed or opened), and with one opens an agent-authored PR
// labelled aayu:evolve that it never merges. It acts only on accepted
// Feedback filed by retro.
func TestEvolveVerb(t *testing.T) {
	root := filepath.Join("..", "..")
	schema, _ := os.ReadFile(filepath.Join(root, "evals", "schemas", "workitem.json"))
	issue, _ := os.ReadFile(filepath.Join(root, "evals", "cases", "stub", "csv-export", "output", "issue.json"))
	tmp := t.TempDir()
	bare := filepath.Join(tmp, "origin.git")
	gitIn(t, tmp, "init", "--quiet", "--bare", bare)
	seed := filepath.Join(tmp, "seed")
	gitIn(t, tmp, "clone", "--quiet", bare, seed)
	gitIn(t, seed, "checkout", "--quiet", "--orphan", "ledger")
	writeTree(t, seed, map[string]string{"README.md": "ledger\n"})
	gitIn(t, seed, "add", ".")
	gitIn(t, seed, "commit", "--quiet", "-m", "ledger")
	gitIn(t, seed, "push", "--quiet", "origin", "ledger")
	gitIn(t, seed, "checkout", "--quiet", "--orphan", "dev")
	gitIn(t, seed, "rm", "-rf", "--quiet", ".")
	writeTree(t, seed, map[string]string{"evals/schemas/workitem.json": string(schema), "evals/skills/stub/SKILL.md": "# stub\n\nDraft a Story.\n"})
	gitIn(t, seed, "add", ".")
	gitIn(t, seed, "commit", "--quiet", "-m", "dev")
	gitIn(t, seed, "push", "--quiet", "origin", "dev")
	dir := filepath.Join(tmp, "work")
	gitIn(t, tmp, "clone", "--quiet", "--branch", "dev", bare, dir)
	gitIn(t, dir, "config", "user.name", "Evolver") // CI has no git identity; evolve commits as the clone's user
	gitIn(t, dir, "config", "user.email", "evolver@example.com")

	g := fake.New(t, "o/r", "User")
	g.Viewer = "founder"
	for i := 1; i <= 4; i++ {
		g.AddIssue("filler", "")
	}
	g.AddIssue("Feedback: retro: skill `stub` correction rate 2/3 above 20 %", "### What happened or what you need\n\nx\n", "type:feedback", "status:backlog", "intake", "needs:grill")
	g.AddIssue("Feedback: retro: not accepted yet", "x", "type:feedback", "status:backlog", "intake")
	home := t.TempDir()
	env := map[string]string{"AAYU_GITHUB_TOKEN": "human-token", "HOME": home, "PATH": os.Getenv("PATH")}
	run := func(args ...string) (int, string, string) {
		var out, errb bytes.Buffer
		code := cli.Run(context.Background(), append([]string{"evolve", "--repo", "o/r"}, args...), cli.IO{In: strings.NewReader(""), Out: &out, Err: &errb}, cli.Options{
			Dir: dir, GitHubAPI: g.URL(), GitHubGap: -1, Assets: aayudha.Assets, EvolveGate: Gate,
			Getenv: func(k string) string { return env[k] }, Environ: []string{"HOME=" + home},
			GHToken: func(context.Context) (string, error) { return "", errors.New("no gh in tests") },
		})
		return code, out.String(), errb.String()
	}
	if code, out, errOut := run(); code != 0 || !strings.Contains(out, "#5 Feedback: retro: skill `stub`") || !strings.Contains(out, "aayu evolve --skill stub --feedback 5") || strings.Contains(out, "#6") {
		t.Fatalf("listing accepted retro Feedback: %d %s %s", code, out, errOut)
	}
	if code, _, errOut := run("--skill", "Bad", "--feedback", "5"); code != cli.ExitUsage {
		t.Fatalf("bad skill name: %d %s", code, errOut)
	}
	if code, _, errOut := run("--skill", "stub", "--feedback", "6"); code != cli.ExitBlocked || !strings.Contains(errOut, "not accepted") {
		t.Fatalf("not accepted Feedback: %d %s", code, errOut)
	}
	if code, _, errOut := run("--skill", "stub", "--feedback", "1"); code != cli.ExitBlocked || !strings.Contains(errOut, "not a Feedback issue filed by `aayu retro`") {
		t.Fatalf("not retro Feedback: %d %s", code, errOut)
	}
	code, out, errOut := run("--skill", "stub", "--feedback", "5")
	wt := filepath.Join(dir, ".aayudha", "worktrees", "evolve-5")
	if code != 0 || !strings.Contains(out, "evolve/stub-5") {
		t.Fatalf("first run: %d %s %s", code, out, errOut)
	}
	writeTree(t, wt, map[string]string{"evals/skills/stub/SKILL.md": "# stub\n\nDraft a Story. Never add a content type.\n"})
	if code, _, errOut := run("--skill", "stub", "--feedback", "5"); code != cli.ExitBlocked || !strings.Contains(errOut, "adds none") {
		t.Fatalf("skill diff without an eval case: %d %s", code, errOut)
	}
	if ahead := gitIn(t, wt, "rev-list", "--count", "origin/dev..HEAD"); ahead != "0" || len(g.Writes()) != 0 {
		t.Fatalf("a refused change was committed (%s ahead) or written: %v", ahead, g.Writes())
	}
	before := strings.Replace(string(issue), `"title": "`, `"title": "Export as text/csv: `, 1)
	writeTree(t, wt, map[string]string{
		"evals/cases/stub/no-content-type/case.toml": "skill = \"stub\"\ndescription = \"Feedback #5\"\ninput = \"input.txt\"\nartifacts = [\"output/issue.json\"]\n" +
			"schemas = [{ file = \"output/issue.json\", schema = \"evals/schemas/workitem.json\" }]\nready = [\"output/issue.json\"]\n" +
			"scope = [{ pattern = '(?i)\\btext/csv\\b', why = \"a content type the request does not state\" }]\nfeedback = 5\nreproduce = \"before\"\n",
		"evals/cases/stub/no-content-type/input.txt":         "Export the claims list as CSV.\n",
		"evals/cases/stub/no-content-type/output/issue.json": string(issue),
		"evals/cases/stub/no-content-type/before/issue.json": before,
	})
	code, out, errOut = run("--skill", "stub", "--feedback", "5")
	if code != 0 || !strings.Contains(out, "eval case stub/no-content-type reproduces Feedback #5") || !strings.Contains(out, "evolve never merges") {
		t.Fatalf("eval-first change: %d %s %s", code, out, errOut)
	}
	msg := gitIn(t, bare, "log", "-1", "--format=%B", "evolve/stub-5")
	if !strings.Contains(msg, "Aayudha-Evolve: #5") || !strings.Contains(msg, "Signed-off-by: Evolver <evolver@example.com>") {
		t.Fatalf("evolve commit: %s", msg)
	}
	var paths []string
	for _, w := range g.Writes() {
		paths = append(paths, w.Method+" "+w.Path)
		if strings.HasSuffix(w.Path, "/merge") {
			t.Fatalf("evolve merged its PR: %v", w)
		}
	}
	if len(paths) != 2 || paths[0] != "POST /repos/o/r/pulls" || !strings.HasSuffix(paths[1], "/labels") {
		t.Fatalf("writes: %v", paths)
	}
	prN := 0
	for n, pr := range g.Pulls {
		if pr.Head.Ref == "evolve/stub-5" {
			prN = n
		}
	}
	labels := strings.Join(g.LabelNames(prN), ",")
	if !strings.Contains(labels, "agent-authored") || !strings.Contains(labels, "aayu:evolve") {
		t.Fatalf("PR labels: %s", labels)
	}
	if pr := g.Pulls[prN]; pr == nil || !strings.Contains(pr.Body, "Refs #5") || !strings.Contains(pr.Body, "Never auto-merged") || pr.Head.Ref != "evolve/stub-5" {
		t.Fatalf("PR: %+v", pr)
	}
}
