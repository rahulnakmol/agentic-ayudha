package ratchet

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/evals"
	"github.com/rahulnakmol/agentic-ayudha/internal/gittest"
)

func TestMain(m *testing.M) { gittest.Main(m) }

// The check reads the base's evals/ from git and the change from the working
// tree: a lowered bar committed nowhere is still caught (Story #400).
func TestEvalBars(t *testing.T) {
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
	git := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.com"}, args...)...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	toml := "judge = \"gpt-6.1-sol\"\nk = 5\npass_score = 4\nmin_pass_k = 0.8\nmin_mean = 3.5\nmax_drop = 0.3\n\n[[groups]]\nname = \"skills\"\nmax_usd = 4.0\n"
	write("evals/eval.toml", toml)
	write("evals/cases/stub/one/case.toml", "skill = \"stub\"\ninput = \"input.txt\"\n\n[tier2]\nrubric = \"rubric.md\"\n")
	write("evals/cases/stub/one/input.txt", "x\n")
	write("evals/cases/stub/one/rubric.md", "- 5: Both hold.\n")
	git("init", "-q")
	git("add", ".")
	git("commit", "-q", "-m", "base")

	never := func(int) error { t.Fatal("no waiver to check"); return nil }
	var none strings.Builder
	if problems, err := EvalBarReport(&none, repo, "HEAD", never); err != nil || len(problems) > 0 || none.String() != "eval bars: none lowered\n" {
		t.Fatalf("unchanged: %q %q %v", none.String(), problems, err)
	}
	write("evals/eval.toml", strings.Replace(toml, "min_mean = 3.5", "min_mean = 3.0", 1))
	var log strings.Builder
	problems, err := EvalBarReport(&log, repo, "HEAD", never)
	if err != nil || log.String() != "eval bar lowered: min_mean: 3.5 → 3\n" || len(problems) != 2 ||
		problems[0] != "eval-bar ratchet: min_mean lowers a skill-eval bar without a new waiver: 3.5 → 3" {
		t.Fatalf("lowered min_mean: %q %q %v", log.String(), problems, err)
	}
	if _, _, err := EvalBars(repo, "no-such-commit", never); err == nil || !strings.Contains(err.Error(), "git archive no-such-commit evals") {
		t.Fatalf("a missing base: %v", err)
	}
	write("evals/eval.toml", "judge = \"\"\n")
	if _, err := EvalBarReport(&none, repo, "HEAD", never); err == nil || !strings.HasPrefix(err.Error(), "eval-bar ratchet: ") {
		t.Fatalf("a broken head: %v", err)
	}

	// The waiver is a floor-waiver Decision for eval-bar, closed by the person who accepted it.
	is := &gh.Issue{Number: gh.Ptr(412), Title: gh.Ptr("Waive the " + evals.BarFloor + " floor on #400"), State: gh.Ptr("closed"), StateReason: gh.Ptr("completed"),
		Labels:   []*gh.Label{{Name: "type:decision"}, {Name: "decision:waiver"}},
		Body:     gh.Ptr("### Decision\n\nwaive, accepted by @founder on 2026-10-17.\n"),
		ClosedBy: &gh.User{Login: gh.Ptr("founder"), Type: gh.Ptr("User")}}
	if err := AcceptedWaiver(is, evals.BarFloor, &config.Config{}, "story"); err != nil {
		t.Errorf("an accepted eval-bar waiver was refused: %v", err)
	}
	is.Title = gh.Ptr("Waive the coverage floor on #400")
	if err := AcceptedWaiver(is, evals.BarFloor, &config.Config{}, "story"); err == nil {
		t.Error("a coverage waiver was taken for an eval-bar waiver")
	}
}
