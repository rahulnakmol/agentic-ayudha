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
	opts := Options{IsTerminal: stdinTTY, GitHubAPI: g.URL(), GitHubGap: -1, Dir: dir}
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

// WP4.5 accept: a capture against a URL whose config marks it production
// exits 4 with no artifacts written (and seeds nothing).
func TestVerifyRefusesProduction(t *testing.T) {
	g := readyTree(t)
	dir, _ := ledgerCheckout(t)
	marker := filepath.Join(t.TempDir(), "seeded")
	_ = os.MkdirAll(filepath.Join(dir, ".aayudha"), 0o755)
	cfg := "version = 1\n[verify]\nseed = [\"touch\", \"" + marker + "\"]\n[verify.environments.prod]\nurl = \"https://app.example.com\"\nproduction = true\n"
	_ = os.WriteFile(filepath.Join(dir, ".aayudha", "config.toml"), []byte(cfg), 0o644)
	git(t, dir, "checkout", "--quiet", "-b", "story/2-x")
	git(t, dir, "commit", "--quiet", "--allow-empty", "-m", "feat: x")
	env := map[string]string{"AAYU_GITHUB_TOKEN": "human-token", "PATH": os.Getenv("PATH"), "AAYU_ENV": "prod"}
	code, out, errOut := run(t, env, Options{IsTerminal: stdinTTY, GitHubAPI: g.URL(), GitHubGap: -1, Dir: dir}, "verify", "--repo", "o/r")
	if code != ExitBlocked || !strings.Contains(errOut, "production environment") {
		t.Fatalf("verify against production: %d %s %s", code, out, errOut)
	}
	if _, err := os.Stat(filepath.Join(dir, ".aayudha", "proof")); !os.IsNotExist(err) {
		t.Fatalf("artifacts written: %v", err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("the seed ran against production")
	}
}

// WP4.9 (0.1 part): a story on a mobile platform gets "skipped: mobile",
// which does not hold (the gate and ship refuse it); a floor-waiver Decision
// under the story waives it and the pack records the Decision.
func TestVerifyMobileSkipAndWaiver(t *testing.T) {
	g := readyTree(t)
	dir, _ := ledgerCheckout(t)
	env := map[string]string{"AAYU_GITHUB_TOKEN": "human-token", "PATH": os.Getenv("PATH")}
	opts := Options{IsTerminal: stdinTTY, GitHubAPI: g.URL(), GitHubGap: -1, Dir: dir}
	if code, _, errOut := run(t, env, opts, "ready", "2", "--repo", "o/r"); code != 0 {
		t.Fatalf("ready: %s", errOut)
	}
	git(t, dir, "checkout", "--quiet", "-b", "story/2-mobile")
	_ = os.WriteFile(filepath.Join(dir, "cov.info"), []byte("SF:x\nend_of_record\n"), 0o644)
	git(t, dir, "add", "cov.info")
	git(t, dir, "commit", "--quiet", "-m", "feat: mobile")
	cfg := "version = 1\n[verify]\nplatforms = [\"web\", \"ios\"]\n[verify.floors.coverage]\ncommand = [\"true\"]\nreport = \"cov.info\"\n"
	_ = os.WriteFile(filepath.Join(dir, ".aayudha", "config.toml"), []byte(cfg), 0o644)
	verifyMobile := func() (int, string) {
		code, out, errOut := run(t, env, opts, "verify", "--floor", "mobile", "--repo", "o/r")
		return code, out + errOut
	}
	code, out := verifyMobile()
	p, err := proof.LoadLocal(proof.LocalDir(dir, 2))
	if code != ExitFloor || err != nil {
		t.Fatalf("mobile story: %d %s %v", code, out, err)
	}
	if c, _ := p.Check("mobile"); c.Result != proof.Skipped || c.Detail != proof.SkippedMobile || p.FloorsHold() {
		t.Fatalf("mobile check: %+v", c)
	}
	// A filed waiver waives nothing until a human accepts it (WP5.6).
	code, out, errOut := run(t, env, opts, "decide", "2", "--waiver", "mobile", "--repo", "o/r", "--json")
	var filed decideReport
	if err := json.Unmarshal([]byte(out), &filed); code != 0 || err != nil {
		t.Fatalf("decide --waiver: %d %s %s", code, out, errOut)
	}
	d := g.Issues[filed.Waiver.Decision]
	if code, out = verifyMobile(); code != ExitFloor {
		t.Fatalf("with an unaccepted waiver: %d %s", code, out)
	}
	if code, out, errOut := run(t, env, opts, "decide", strconv.Itoa(d.Number), "--accept", "waive", "--repo", "o/r"); code != 0 {
		t.Fatalf("accept the waiver: %d %s %s", code, out, errOut)
	}
	if code, out = verifyMobile(); code != 0 || !strings.Contains(out, "waived: mobile by #"+strconv.Itoa(d.Number)) {
		t.Fatalf("with a waiver: %d %s", code, out)
	}
	p, _ = proof.LoadLocal(proof.LocalDir(dir, 2))
	if c, _ := p.Check("mobile"); c.Result != proof.Waived || c.Waiver != "#"+strconv.Itoa(d.Number) || !p.FloorsHold() {
		t.Fatalf("waived check: %+v", c)
	}
}

// WP4.8 merge gate (verify --ci on a PR): criteria without evidence from this
// run fail the gate, and so does a story edited after Ready (Completion Goal
// hash ≠ the Ready snapshot's).
func TestVerifyGateCI(t *testing.T) {
	g := readyTree(t)
	dir, _ := ledgerCheckout(t)
	env := map[string]string{"AAYU_GITHUB_TOKEN": "human-token", "PATH": os.Getenv("PATH")}
	opts := Options{IsTerminal: stdinTTY, GitHubAPI: g.URL(), GitHubGap: -1, Dir: dir}
	if code, _, errOut := run(t, env, opts, "ready", "2", "--repo", "o/r"); code != 0 {
		t.Fatalf("ready: %s", errOut)
	}
	base := git(t, dir, "rev-parse", "HEAD")
	git(t, dir, "checkout", "--quiet", "-b", "story/2-gate")
	_ = os.WriteFile(filepath.Join(dir, "cov.info"), []byte("SF:x\nend_of_record\n"), 0o644)
	git(t, dir, "add", "cov.info")
	git(t, dir, "commit", "--quiet", "-m", "feat: gate")
	pr := g.AddPR("Gate", "Refs #2", "founder", "story/2-gate", git(t, dir, "rev-parse", "HEAD"), "dev")
	g.Mu.Lock()
	pr.Base.SHA = base
	g.Mu.Unlock()
	_ = os.WriteFile(filepath.Join(dir, ".aayudha", "config.toml"), []byte("version = 1\n[verify.floors.coverage]\ncommand = [\"true\"]\nreport = \"cov.info\"\n"), 0o644)
	ci := map[string]string{"GITHUB_TOKEN": "workflow-token", "PATH": os.Getenv("PATH")}
	gate := func() (int, VerifyResult, string) {
		code, out, errOut := run(t, ci, opts, "verify", strconv.Itoa(pr.Number), "--floor", "coverage", "--ci", "--repo", "o/r", "--json")
		var res VerifyResult
		_ = json.NewDecoder(strings.NewReader(out)).Decode(&res)
		return code, res, errOut
	}
	code, res, errOut := gate()
	if code != ExitBlocked || len(res.Gate) != 1 || !strings.Contains(res.Gate[0], "criteria without evidence from this run") {
		t.Fatalf("unmapped criteria: %d %+v %s", code, res.Gate, errOut)
	}
	g.Mu.Lock()
	g.Issues[2].Body = strings.Replace(g.Issues[2].Body, "expected:", "expected: (edited)", 1)
	g.Mu.Unlock()
	if code, res, _ = gate(); code != ExitBlocked || len(res.Gate) != 2 || !strings.Contains(res.Gate[1], "≠ Ready snapshot") {
		t.Fatalf("drifted story: %d %+v", code, res.Gate)
	}
	// A PR that serves no story passes the gate without running a floor.
	g.Mu.Lock()
	pr.Body, pr.Head.Ref = "Tidy the docs.", "docs/tidy"
	g.Mu.Unlock()
	git(t, dir, "commit", "--quiet", "--amend", "-m", "docs: tidy")
	g.Mu.Lock()
	pr.Head.SHA = git(t, dir, "rev-parse", "HEAD")
	g.Mu.Unlock()
	if code, res, errOut := gate(); code != 0 || len(res.Checks) != 0 || res.Story != 0 {
		t.Fatalf("PR without a story: %d %+v %s", code, res, errOut)
	}
}
