package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/gittest"
)

func TestMain(m *testing.M) { gittest.Main(m) }

// CI's one ratchet step fails a PR that lowers a skill-eval bar, as it fails
// one that loosens the coverage floor (Story #400).
func TestRunFailsALoweredEvalBar(t *testing.T) {
	repo := t.TempDir()
	write := func(p, body string) {
		f := filepath.Join(repo, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	toml := "judge = \"gpt-6.1-sol\"\nk = 5\npass_score = 4\nmin_pass_k = 0.8\nmin_mean = 3.5\nmax_drop = 0.3\n\n[[groups]]\nname = \"skills\"\nmax_usd = 4.0\n"
	write("ratchets.toml", "[coverage]\nfloor = 50.0\nmargin = 0.5\n")
	write("coverage.out", "mode: set\na.go:1.1,2.2 1 1\na.go:3.1,4.2 1 0\n") // 50 %
	write("evals/eval.toml", toml)
	write("evals/cases/stub/one/case.toml", "skill = \"stub\"\ninput = \"input.txt\"\n")
	write("evals/cases/stub/one/input.txt", "x\n")
	for _, args := range [][]string{{"init", "-q"}, {"add", "."}, {"commit", "-q", "-m", "base"}} {
		cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.com"}, args...)...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	t.Chdir(repo)
	if err := run("coverage.out", "HEAD", false); err != nil {
		t.Fatalf("nothing lowered: %v", err)
	}
	write("evals/eval.toml", strings.Replace(toml, "pass_score = 4", "pass_score = 3", 1))
	err := run("coverage.out", "HEAD", false)
	if err == nil || !strings.HasPrefix(err.Error(), "eval-bar ratchet: pass_score lowers a skill-eval bar without a new waiver: 4 → 3\n::error::eval-bar ratchet: ") {
		t.Fatalf("pass_score lowered: %v", err)
	}
}
