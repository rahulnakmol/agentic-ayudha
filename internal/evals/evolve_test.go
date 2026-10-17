package evals

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	home := t.TempDir()
	env := map[string]string{"AAYU_GITHUB_TOKEN": "human-token", "HOME": home, "PATH": os.Getenv("PATH")}
	gate := Gate
	runEnv := func(env map[string]string, args ...string) (int, string, string) {
		var out, errb bytes.Buffer
		code := cli.Run(context.Background(), append([]string{"evolve", "--repo", "o/r"}, args...), cli.IO{In: strings.NewReader(""), Out: &out, Err: &errb}, cli.Options{
			Dir: dir, GitHubAPI: g.URL(), GitHubGap: -1, Assets: aayudha.Assets, EvolveGate: gate,
			Getenv: func(k string) string { return env[k] }, Environ: []string{"HOME=" + home},
			GHToken: func(context.Context) (string, error) { return "", errors.New("no gh in tests") },
		})
		return code, out.String(), errb.String()
	}
	run := func(args ...string) (int, string, string) { return runEnv(env, args...) }
	noToken := map[string]string{"HOME": home, "PATH": os.Getenv("PATH")}
	if code, _, errOut := runEnv(noToken); code != cli.ExitBlocked {
		t.Fatalf("listing without a token: %d %s", code, errOut)
	}
	if code, _, errOut := runEnv(noToken, "--skill", "stub", "--feedback", "5"); code != cli.ExitBlocked {
		t.Fatalf("evolve without a token: %d %s", code, errOut)
	}
	g.Mu.Lock()
	g.Fail, g.FailAfter = "GET /repos/o/r/issues", 0
	g.Mu.Unlock()
	if code, _, errOut := run(); code != cli.ExitExternal {
		t.Fatalf("listing with GitHub failing: %d %s", code, errOut)
	}
	var ob, eb bytes.Buffer
	if code := cli.Run(context.Background(), []string{"evolve", "--repo", "o/r"}, cli.IO{In: strings.NewReader(""), Out: &ob, Err: &eb},
		cli.Options{Dir: t.TempDir(), Getenv: func(k string) string { return env[k] }}); code != cli.ExitUsage || !strings.Contains(eb.String(), "checkout") {
		t.Fatalf("evolve outside a checkout: %d %s", code, eb.String())
	}
	if code, out, _ := run(); code != 0 || !strings.Contains(out, "no accepted Feedback from retro is waiting") {
		t.Fatalf("listing with nothing accepted: %d %s", code, out)
	}
	g.AddIssue("Feedback: retro: skill `stub` correction rate 2/3 above 20 %", "### What happened or what you need\n\nx\n", "type:feedback", "status:backlog", "intake", "needs:grill", "fix:prose")
	g.AddIssue("Feedback: retro: not accepted yet", "x", "type:feedback", "status:backlog", "intake")
	g.AddIssue("Feedback: retro: correction rate 3/4 above 20 %", "x", "type:feedback", "status:backlog", "intake", "needs:grill")
	if code, out, errOut := run(); code != 0 || !strings.Contains(out, "#7 Feedback: retro: correction rate 3/4 above 20 %\n  names no skill") {
		t.Fatalf("a finding that names no skill: %d %s %s", code, out, errOut)
	}
	// P4 (S-7.7c): a skill changes only on two accepted Feedback issues that name it.
	if code, out, errOut := run(); code != 0 || !strings.Contains(out, "waiting: skill `stub` needs 2 accepted prose Feedback issues, has 1") || strings.Contains(out, "--feedback 5") {
		t.Fatalf("listing one Feedback for a skill: %d %s %s", code, out, errOut)
	}
	if code, _, errOut := run("--skill", "stub", "--feedback", "5"); code != cli.ExitBlocked || !strings.Contains(errOut, "skill `stub` has #5 only") {
		t.Fatalf("evolve on one Feedback: %d %s", code, errOut)
	}
	if code, _, errOut := run("--skill", "stub", "--feedback", "7"); code != cli.ExitBlocked || !strings.Contains(errOut, "#7 does not name skill `stub`") {
		t.Fatalf("evolve on Feedback about no skill: %d %s", code, errOut)
	}
	// S-7.7d: only a prose correction goes to a skill rewrite. An unclassified Feedback, and a codebase one whose
	// classification does not say why code or a check cannot catch it, are refused and are not evidence.
	g.AddIssue("Feedback: retro: skill `stub` correction rate 3/3 above 20 %, a later window", "x", "type:feedback", "status:backlog", "intake", "needs:grill")
	if code, _, errOut := run("--skill", "stub", "--feedback", "8"); code != cli.ExitBlocked || !strings.Contains(errOut, "Feedback #8 has no fix layer") {
		t.Fatalf("evolve on unclassified Feedback: %d %s", code, errOut)
	}
	g.Mu.Lock()
	g.Issues[8].Labels = append(g.Issues[8].Labels, fake.Label{Name: "fix:codebase"})
	g.Mu.Unlock()
	if code, _, errOut := run("--skill", "stub", "--feedback", "8"); code != cli.ExitBlocked || !strings.Contains(errOut, "Feedback #8 is a codebase correction") {
		t.Fatalf("evolve on a codebase correction: %d %s", code, errOut)
	}
	if code, out, errOut := run(); code != 0 || !strings.Contains(out, "not for evolve: Feedback #8 is a codebase correction") || !strings.Contains(out, "has 1") {
		t.Fatalf("listing a codebase correction: %d %s %s", code, out, errOut)
	}
	g.Mu.Lock()
	g.Issues[8].Comments = append(g.Issues[8].Comments, &fake.Comment{ID: 801, Body: cli.FixLayerMarker + "\n<!-- aayudha:fix-layer-state " +
		`{"feedback":8,"layer":"codebase","reason":"a typed field","not_catchable":"the wording is judged per request","story":12}` + " -->"})
	g.Mu.Unlock()
	if code, out, errOut := run(); code != 0 || !strings.Contains(out, "#5 Feedback: retro: skill `stub`") || !strings.Contains(out, "aayu evolve --skill stub --feedback 5") || strings.Contains(out, "#6") {
		t.Fatalf("listing accepted retro Feedback: %d %s %s", code, out, errOut)
	}
	// At most one evolve PR per skill a month: one opened 10 days ago blocks, one 40 days ago does not.
	g.Mu.Lock()
	g.Pulls[100] = &fake.PR{Number: 100, State: "closed", Head: fake.Branch{Ref: "evolve/stub-2"}, CreatedAt: time.Now().AddDate(0, 0, -10)}
	g.Mu.Unlock()
	if code, _, errOut := run("--skill", "stub", "--feedback", "5"); code != cli.ExitBlocked || !strings.Contains(errOut, "PR #100 proposed a change to skill `stub`") {
		t.Fatalf("evolve within a month of the last: %d %s", code, errOut)
	}
	g.Mu.Lock()
	g.Pulls[100].CreatedAt = time.Now().AddDate(0, 0, -40)
	g.Mu.Unlock()
	if code, _, errOut := run("--skill", "Bad", "--feedback", "5"); code != cli.ExitUsage {
		t.Fatalf("bad skill name: %d %s", code, errOut)
	}
	if code, _, errOut := run("--skill", "stub", "--feedback", "99"); code != cli.ExitExternal {
		t.Fatalf("missing issue: %d %s", code, errOut)
	}
	g.Mu.Lock()
	g.Issues[1].Title, g.Issues[1].State = "Feedback: retro: closed one", "closed"
	g.Issues[1].Labels = append(g.Issues[1].Labels, fake.Label{Name: "type:feedback"})
	g.Mu.Unlock()
	if code, _, errOut := run("--skill", "stub", "--feedback", "1"); code != cli.ExitBlocked || !strings.Contains(errOut, "is closed") {
		t.Fatalf("closed Feedback: %d %s", code, errOut)
	}
	g.Mu.Lock()
	g.Issues[1].Title, g.Issues[1].Labels = "filler", nil
	g.Mu.Unlock()
	if code, _, errOut := run("--skill", "stub", "--feedback", "6"); code != cli.ExitBlocked || !strings.Contains(errOut, "not accepted") {
		t.Fatalf("not accepted Feedback: %d %s", code, errOut)
	}
	if code, _, errOut := run("--skill", "stub", "--feedback", "1"); code != cli.ExitBlocked || !strings.Contains(errOut, "not a Feedback issue filed by `aayu retro`") {
		t.Fatalf("not retro Feedback: %d %s", code, errOut)
	}
	code, out, errOut := run("--skill", "stub", "--feedback", "5")
	wt := filepath.Join(dir, ".aayudha", "worktrees", "evolve-stub-5")
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
	gate = nil
	if code, _, errOut := run("--skill", "stub", "--feedback", "5"); code != cli.ExitInternal || !strings.Contains(errOut, "eval harness") {
		t.Fatalf("a binary without the gate: %d %s", code, errOut)
	}
	gate = Gate
	before := strings.Replace(string(issue), `"title": "`, `"title": "Export as text/csv: `, 1)
	writeTree(t, wt, map[string]string{
		"evals/cases/stub/no-content-type/case.toml": "skill = \"stub\"\ndescription = \"Feedback #5\"\ninput = \"input.txt\"\nartifacts = [\"output/issue.json\"]\n" +
			"schemas = [{ file = \"output/issue.json\", schema = \"evals/schemas/workitem.json\" }]\nready = [\"output/issue.json\"]\n" +
			"scope = [{ pattern = '(?i)\\btext/csv\\b', why = \"a content type the request does not state\" }]\nfeedback = 5\nreproduce = \"before\"\n",
		"evals/cases/stub/no-content-type/input.txt":         "Export the claims list as CSV.\n",
		"evals/cases/stub/no-content-type/output/issue.json": string(issue),
		"evals/cases/stub/no-content-type/before/issue.json": before,
	})
	caseFile := filepath.Join(wt, "evals", "cases", "stub", "no-content-type", "case.toml")
	right, _ := os.ReadFile(caseFile)
	_ = os.WriteFile(caseFile, []byte(strings.Replace(string(right), "feedback = 5", "feedback = 4", 1)), 0o644)
	if code, _, errOut := run("--skill", "stub", "--feedback", "5"); code != cli.ExitBlocked || !strings.Contains(errOut, "no new case is for skill stub and Feedback #5") {
		t.Fatalf("a case for another Feedback: %d %s", code, errOut)
	}
	_ = os.WriteFile(caseFile, right, 0o644)
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
	if body := g.Pulls[prN].Body; !strings.Contains(body, "Evidence: #5, #8") {
		t.Fatalf("PR body does not cite both Feedback issues: %s", body)
	}
	g.Mu.Lock()
	g.Fail, g.FailAfter = "GET /repos/o/r/pulls", 0
	g.Mu.Unlock()
	if code, _, errOut := run("--skill", "stub", "--feedback", "5"); code != cli.ExitExternal {
		t.Fatalf("re-run with GitHub failing: %d %s", code, errOut)
	}
	// A Feedback that names another skill does not evolve it, so evolve never cuts a second worktree for it.
	g.Reset()
	if code, _, errOut := run("--skill", "grill", "--feedback", "5"); code != cli.ExitBlocked || !strings.Contains(errOut, "does not name skill `grill`") {
		t.Fatalf("another skill: %d %s", code, errOut)
	}
	// A re-run finds the open PR instead of opening another.
	g.Reset()
	if code, out, errOut := run("--skill", "stub", "--feedback", "5"); code != 0 || !strings.Contains(out, fmt.Sprintf("PR #%d", prN)) {
		t.Fatalf("re-run: %d %s %s", code, out, errOut)
	}
	for _, w := range g.Writes() {
		if w.Path == "/repos/o/r/pulls" {
			t.Fatalf("a re-run opened a second PR")
		}
	}
	if pr := g.Pulls[prN]; pr == nil || !strings.Contains(pr.Body, "Refs #5") || !strings.Contains(pr.Body, "Never auto-merged") || pr.Head.Ref != "evolve/stub-5" {
		t.Fatalf("PR: %+v", pr)
	}
}
