package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
)

// verify through the CLI: the PR's story comes from "Refs #n", the Ready
// snapshot from the ledger, the floors run on the PR's diff, and the local
// pack is what proof push lands; a floor miss exits 3.
func TestVerifyWritesPackForStory(t *testing.T) {
	g := readyTree(t)
	dir, _ := ledgerCheckout(t)
	env := map[string]string{"AAYU_GITHUB_TOKEN": "human-token", "PATH": os.Getenv("PATH")}
	opts := Options{GitHubAPI: g.URL(), GitHubGap: -1, Dir: dir}
	if code, _, errOut := run(t, env, opts, "ready", "2", "--repo", "o/r"); code != 0 {
		t.Fatalf("ready: %s", errOut)
	}
	base := git(t, dir, "rev-parse", "HEAD")
	git(t, dir, "checkout", "--quiet", "-b", "story/2-empty-list")
	var src, lcov strings.Builder
	lcov.WriteString("SF:src.txt\n")
	for n := 1; n <= 10; n++ {
		src.WriteString("x\n")
		hit := 1
		if n == 10 {
			hit = 0
		}
		lcov.WriteString("DA:" + strconv.Itoa(n) + "," + strconv.Itoa(hit) + "\n")
	}
	lcov.WriteString("end_of_record\n")
	_ = os.WriteFile(filepath.Join(dir, "src.txt"), []byte(src.String()), 0o644)
	_ = os.MkdirAll(filepath.Join(dir, "cov"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "cov", "lcov.info"), []byte(lcov.String()), 0o644)
	git(t, dir, "add", "src.txt", "cov")
	git(t, dir, "commit", "--quiet", "-m", "feat: list")
	head := git(t, dir, "rev-parse", "HEAD")
	pr := g.AddPR("Show empty list", "Implements the empty list.\n\nRefs #2", "founder", "story/2-empty-list", head, "dev")
	g.Mu.Lock()
	pr.Base.SHA = base
	g.Mu.Unlock()
	cfg := "version = 1\n[verify.floors.coverage]\ncommand = [\"true\"]\nreport = \"cov/lcov.info\"\nmin_diff_percent = 90.0\n"
	_ = os.WriteFile(filepath.Join(dir, ".aayudha", "config.toml"), []byte(cfg), 0o644)

	code, out, errOut := run(t, env, opts, "verify", strconv.Itoa(pr.Number), "--floor", "coverage", "--repo", "o/r", "--json")
	if code != 0 {
		t.Fatalf("verify: %d %s %s", code, out, errOut)
	}
	var res VerifyResult
	_ = json.Unmarshal([]byte(out), &res)
	if res.Story != 2 || res.Base != base || res.Head != head || !res.Hold || len(res.Checks) != 1 || res.Checks[0].Value != 90 || len(res.Unmapped) == 0 {
		t.Fatalf("verify result: %+v", res)
	}
	p, err := proof.LoadLocal(proof.LocalDir(dir, 2))
	if err != nil || p.PRHeadSHA != head || p.PR != pr.Number || p.Identity != "human-fallback" || p.CompletionGoalHash == "" {
		t.Fatalf("local pack: %+v %v", p, err)
	}
	// Agent-pushed: advisory. proof verify then names every unmapped criterion.
	if code, _, errOut := run(t, env, opts, "proof", "push", "2", "--repo", "o/r"); code != 0 {
		t.Fatalf("push: %s", errOut)
	}
	if code, out, _ := run(t, env, opts, "proof", "verify", "2", "--repo", "o/r"); code != ExitBlocked || !strings.Contains(out, "missing criterion mapping") {
		t.Fatalf("proof verify of a pack without evidence: %d %s", code, out)
	}

	// Raise the bar: 9 of 10 lines is a floor miss, exit 3.
	_ = os.WriteFile(filepath.Join(dir, ".aayudha", "config.toml"), []byte(strings.Replace(cfg, "90.0", "90.1", 1)), 0o644)
	if code, out, errOut := run(t, env, opts, "verify", strconv.Itoa(pr.Number), "--floor", "coverage", "--repo", "o/r"); code != ExitFloor || !strings.Contains(out, "coverage       fail") {
		t.Fatalf("verify below the floor: %d %s %s", code, out, errOut)
	}
	// A checkout that is not the PR head is refused.
	git(t, dir, "checkout", "--quiet", "dev")
	if code, _, errOut := run(t, env, opts, "verify", strconv.Itoa(pr.Number), "--repo", "o/r"); code != ExitBlocked || !strings.Contains(errOut, "check out the PR head") {
		t.Fatalf("verify on the wrong checkout: %d %s", code, errOut)
	}
}

func TestStoryOf(t *testing.T) {
	for _, c := range []struct {
		body, msg, branch string
		want              int
	}{
		{"Adds X.\n\nRefs #12", "", "", 12},
		{"closes #7 and refs #9", "", "", 7},
		{"no ref", "feat: x\n\nAayudha-Story: #31\nAayudha-Checkpoint: 1/2", "", 31},
		{"", "", "story/44-empty-list", 44},
		{"see #5 for context", "", "feat/x", 0},
	} {
		if got := StoryOf(c.body, c.msg, c.branch); got != c.want {
			t.Errorf("StoryOf(%q, %q, %q) = %d, want %d", c.body, c.msg, c.branch, got, c.want)
		}
	}
}
