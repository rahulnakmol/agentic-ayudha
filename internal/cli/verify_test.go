package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
	"github.com/rahulnakmol/agentic-ayudha/internal/models"
	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
	"github.com/rahulnakmol/agentic-ayudha/internal/uireview"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
	"github.com/rahulnakmol/agentic-ayudha/internal/verify"
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
	cfg := "version = 1\n[verify]\nseed = [\"touch\", '" + marker + "']\n[verify.environments.prod]\nurl = \"https://app.example.com\"\nproduction = true\n"
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
	// WP4.5b.2: it still runs the feature-map floors, and only them: a
	// behaviour check of a feature it touches fails the gate.
	fdir := filepath.Join(dir, ".agents", "skills", "verify-app", "features")
	_ = os.MkdirAll(fdir, 0o755)
	_ = os.WriteFile(filepath.Join(fdir, "README.md"), []byte("# app\n\n```map\n{\"product\": \"app\", \"control\": [\"sh\", \"-c\", \"echo '{\\\"status\\\": \\\"failed\\\", \\\"reason\\\": \\\"exit 1\\\"}'\"]}\n```\n\n1. [Cov](cov.md)\n"), 0o644)
	_ = os.WriteFile(filepath.Join(fdir, "cov.md"), []byte("# Cov\n\nx\n\n```feature\n{\"id\": \"cov\", \"sources\": [\"cov.info\"], \"verified_at\": \"\"}\n```\n\n## Sub-features\n\n## How to reach it\n\n## Behaviour checks\n\n```check\n{\"id\": \"read\", \"kind\": \"cli\", \"evidence\": \"api-trace\", \"steps\": [{\"run\": [\"cat\"]}]}\n```\n\n## Gotchas\n"), 0o644)
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "--quiet", "--amend", "-m", "docs: tidy")
	g.Mu.Lock()
	pr.Head.SHA = git(t, dir, "rev-parse", "HEAD")
	g.Mu.Unlock()
	code, out, errOut := run(t, ci, opts, "verify", strconv.Itoa(pr.Number), "--ci", "--repo", "o/r", "--json")
	var nostory VerifyResult
	_ = json.NewDecoder(strings.NewReader(out)).Decode(&nostory)
	var floors []string
	for _, c := range nostory.Checks {
		floors = append(floors, c.Floor+" "+c.Result)
	}
	if code != ExitFloor || strings.Join(floors, ", ") != "feature-map pass, behaviour fail" || nostory.Pack != "" {
		t.Fatalf("no-story PR touching a feature: %d %q %s %s", code, floors, out, errOut)
	}
}

// WP4.5b.1: verify --map needs no GitHub and no story; it exits 0 without a
// map or with a map in sync, and 3 when the map drifted from the source.
func TestVerifyMapExitCodes(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "--quiet", "-b", "dev")
	env := map[string]string{"PATH": os.Getenv("PATH")}
	if code, out, errOut := run(t, env, Options{Dir: dir}, "verify", "--map"); code != 0 || !strings.Contains(out, "no feature map here") {
		t.Fatalf("no map: %d %s %s", code, out, errOut)
	}
	fdir := filepath.Join(dir, ".agents", "skills", "verify-app", "features")
	_ = os.MkdirAll(fdir, 0o755)
	_ = os.WriteFile(filepath.Join(dir, "app.sh"), []byte("case $1 in hello) echo hi;; esac # cmd hello\n"), 0o644)
	_ = os.WriteFile(filepath.Join(fdir, "README.md"), []byte("# app\n\n```map\n{\"product\": \"app\", \"control\": [\"sh\", \"control.sh\"], \"surfaces\": [{\"kind\": \"command\", \"files\": [\"app.sh\"], \"pattern\": \"cmd ([a-z]+)\"}]}\n```\n\n1. [Hello](hello.md)\n"), 0o644)
	feature := "# Hello\n\nSays hi.\n\n```feature\n{\"id\": \"hello\", \"sources\": [\"app.sh\"], \"surfaces\": {\"command\": [\"hello\"]}, \"verified_at\": \"\"}\n```\n\n## Sub-features\n\n## How to reach it\n\n## Behaviour checks\n\n```check\n{\"id\": \"hi\", \"kind\": \"cli\", \"evidence\": \"api-trace\", \"steps\": [{\"run\": [\"hello\"], \"stdout\": \"hi\"}]}\n```\n\n## Gotchas\n"
	_ = os.WriteFile(filepath.Join(fdir, "hello.md"), []byte(feature), 0o644)
	if code, out, errOut := run(t, env, Options{Dir: dir}, "verify", "--map", "--json"); code != 0 || !strings.Contains(out, `"verdict": "stale"`) {
		t.Fatalf("in sync (never verified): %d %s %s", code, out, errOut)
	}
	_ = os.WriteFile(filepath.Join(dir, "app.sh"), []byte("case $1 in greet) echo hi;; esac # cmd greet\n"), 0o644)
	code, out, errOut := run(t, env, Options{Dir: dir}, "verify", "--map")
	if code != ExitFloor || !strings.Contains(out, `unmapped command "greet" (app.sh)`) || !strings.Contains(out, `command "hello" of feature hello is no longer in the source`) || !strings.Contains(errOut, "feature map drift") {
		t.Fatalf("drift: %d %s %s", code, out, errOut)
	}
}

// WP4.5b.4 accept: --maintain posts the maintain brief on one tracking issue
// with [verify] maintainer's mention when the map drifted or a check failed,
// not twice for the same report, and every time with always; nothing drifted
// and nothing failed hands nothing off in drift mode.
func TestVerifyMaintainHandoff(t *testing.T) {
	g := fake.New(t, "o/r", "User")
	dir := checkout(t)
	env := map[string]string{"PATH": os.Getenv("PATH"), "AAYU_GITHUB_TOKEN": "human-token"}
	opts := Options{Dir: dir, GitHubAPI: g.URL(), GitHubGap: -1}
	_ = os.MkdirAll(filepath.Join(dir, ".aayudha"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, ".aayudha", "config.toml"), []byte("version = 1\n\n[verify]\nmaintainer = \"claude\"\n"), 0o644)
	sk := filepath.Join(dir, ".agents", "skills", "verify-app")
	_ = os.MkdirAll(filepath.Join(sk, "features"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "app.sh"), []byte("case $1 in hello) echo hi;; esac # cmd hello\n"), 0o644)
	_ = os.WriteFile(filepath.Join(sk, "control.sh"), []byte("cat >/dev/null\necho '{\"status\": \"failed\", \"reason\": \"printed nothing\", \"artifacts\": []}'\n"), 0o644)
	_ = os.WriteFile(filepath.Join(sk, "features", "README.md"), []byte("# app\n\n```map\n{\"product\": \"app\", \"control\": [\"sh\", \".agents/skills/verify-app/control.sh\"], \"surfaces\": [{\"kind\": \"command\", \"files\": [\"app.sh\"], \"pattern\": \"cmd ([a-z]+)\"}]}\n```\n\n1. [Hello](hello.md)\n"), 0o644)
	_ = os.WriteFile(filepath.Join(sk, "features", "hello.md"), []byte("# Hello\n\nSays hi.\n\n```feature\n{\"id\": \"hello\", \"sources\": [\"app.sh\"], \"surfaces\": {\"command\": [\"hello\"]}, \"verified_at\": \"\"}\n```\n\n## Sub-features\n\n## How to reach it\n\n## Behaviour checks\n\n```check\n{\"id\": \"hi\", \"kind\": \"cli\", \"evidence\": \"api-trace\", \"steps\": [{\"run\": [\"hello\"], \"stdout\": \"hi\"}]}\n```\n\n## Gotchas\n"), 0o644)
	briefs := func() []string {
		g.Mu.Lock()
		defer g.Mu.Unlock()
		var out []string
		for _, is := range g.Issues {
			for _, c := range is.Comments {
				if strings.Contains(c.Body, verify.BriefMarker) {
					out = append(out, c.Body)
				}
			}
		}
		return out
	}

	// In sync, drift mode: no hand-off, no issue.
	if code, _, errOut := run(t, env, opts, "verify", "--map", "--maintain", "drift"); code != 0 || !strings.Contains(errOut, "no hand-off") || len(briefs()) != 0 {
		t.Fatalf("clean map: %d %s %v", code, errOut, briefs())
	}
	// A renamed command: the brief goes to @claude on a new tracking issue; the same drift again posts nothing.
	_ = os.WriteFile(filepath.Join(dir, "app.sh"), []byte("case $1 in greet) echo hi;; esac # cmd greet\n"), 0o644)
	if code, _, errOut := run(t, env, opts, "verify", "--map", "--maintain", "drift"); code != ExitFloor || !strings.Contains(errOut, "for @claude") {
		t.Fatalf("drift: %d %s", code, errOut)
	}
	if b := briefs(); len(b) != 1 || !strings.Contains(b[0], "@claude please run the **maintain** procedure") || !strings.Contains(b[0], `unmapped command "greet" (app.sh)`) || !strings.Contains(b[0], "Edit only `.agents/skills/verify-app/`") {
		t.Fatalf("brief: %v", b)
	}
	if code, _, errOut := run(t, env, opts, "verify", "--map", "--maintain", "drift"); code != ExitFloor || !strings.Contains(errOut, "already") || len(briefs()) != 1 {
		t.Fatalf("same drift again: %d %s %d briefs", code, errOut, len(briefs()))
	}
	// The sweep adds the failing check: a different report, so a second brief on the same issue.
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "--quiet", "-m", "app")
	git(t, dir, "update-ref", "refs/remotes/origin/dev", "HEAD")
	// D-4.5b.7: the Actions sweep only records it, with no mention, once; the record does not stop the person's run.
	ci := map[string]string{"PATH": os.Getenv("PATH"), "GITHUB_TOKEN": "human-token", "AAYU_BOT_TOKEN": "bot-token"}
	for i := 0; i < 2; i++ {
		if code, _, errOut := run(t, ci, opts, "verify", "--check", "all", "--maintain", "drift", "--ci", "--repo", "o/r"); code != ExitFloor || len(briefs()) != 2 {
			t.Fatalf("CI sweep %d: %d %s %d briefs", i, code, errOut, len(briefs()))
		}
	}
	if b := briefs()[1]; !strings.Contains(b, "Recorded by the Actions loop; no agent was woken") || strings.Contains(b, "@claude") {
		t.Fatalf("record: %s", b)
	}
	if code, _, errOut := run(t, env, opts, "verify", "--check", "all", "--maintain", "drift"); code != ExitFloor || len(briefs()) != 3 || !strings.Contains(briefs()[2], "@claude please") || !strings.Contains(briefs()[2], "`hello.hi` failed: printed nothing") {
		t.Fatalf("sweep: %d %s %v", code, errOut, briefs())
	}
	// always posts even an unchanged report.
	if code, _, errOut := run(t, env, opts, "verify", "--map", "--maintain", "always"); code != ExitFloor || len(briefs()) != 4 {
		t.Fatalf("always: %d %s %d briefs", code, errOut, len(briefs()))
	}
	if code, _, _ := run(t, env, opts, "verify", "--maintain", "drift"); code != ExitUsage {
		t.Errorf("--maintain alone: exit %d", code)
	}
	g.Mu.Lock()
	defer g.Mu.Unlock()
	n := 0
	for _, is := range g.Issues {
		if strings.Contains(is.Body, verify.MaintainMarker) {
			n++
		}
	}
	if n != 1 {
		t.Errorf("%d tracking issues, want 1", n)
	}
}

// TestVerifyScreenshotReview: verify's screenshot judgement goes through the
// ui-review lane at its high effort, through the same reviewer as `aayu
// review`'s lane A, and is labelled advisory with the model and effort. A
// gateway failure is recorded as skipped rather than stopping verify.
func TestVerifyScreenshotReview(t *testing.T) {
	dir := t.TempDir()
	fakeModel := &models.Fake{Replies: []models.Response{laneAReply("contrast"), laneAReply("contrast"), laneAReply()}}
	newE := func(client models.Client) *Env {
		e, err := newEnv(IO{In: strings.NewReader(""), Out: io.Discard, Err: io.Discard}, Options{Dir: dir,
			Getenv: func(k string) string {
				return map[string]string{"HOME": dir, "XDG_STATE_HOME": dir, "AAYU_SECRET_GOOGLE": "k"}[k]
			},
			GHToken:     func(context.Context) (string, error) { return "", os.ErrNotExist },
			ModelClient: func(models.Route, string) models.Client { return client }})
		if err != nil {
			t.Fatal(err)
		}
		return e
	}
	png := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")
	arts := []verify.Evidence{{Path: "screens/1.png", Kind: "screenshot", Data: png, OCRScanned: true}, {Path: "reports/a11y.json", Kind: "report", Data: []byte("{}")}}
	sr := newE(fakeModel).screenshotReview(context.Background(), arts, 0, "")
	if sr == nil || !sr.Advisory || sr.Skipped != "" || sr.Model != verbs.ModelGemini || sr.Effort != "high" || len(sr.Findings) != 1 || sr.Findings[0].Kind != "contrast" {
		t.Fatalf("screenshot review: %+v", sr)
	}
	if len(fakeModel.Requests) != uireview.K {
		t.Fatalf("%d lane A calls, want %d", len(fakeModel.Requests), uireview.K)
	}
	// One image, and a prompt that says it is the only input (no focused
	// screenshot, markup, or tree to judge focus or labels from).
	if r := fakeModel.Requests[0]; r.Effort != "high" || r.Model != verbs.ModelGemini || len(r.Messages[0].Images) != 1 ||
		!strings.Contains(r.System, "no markup") || strings.Contains(r.Messages[0].Content, "<untrusted") {
		t.Fatalf("lane A request: model %s, effort %q, %d images, system %q", r.Model, r.Effort, len(r.Messages[0].Images), r.System)
	}
	broken := &models.Fake{Err: errors.New("gateway down")}
	if sr := newE(broken).screenshotReview(context.Background(), arts, 0, ""); sr == nil || !sr.Advisory || sr.Skipped == "" {
		t.Fatalf("a failing gateway: %+v", sr)
	}
	if sr := newE(fakeModel).screenshotReview(context.Background(), arts[1:], 0, ""); sr != nil {
		t.Fatalf("no screenshots: %+v", sr)
	}
	// A storyless run is charged to the day, not to a synthetic story #0.
	e := newE(fakeModel)
	log, err := os.ReadFile(e.ActionsPath())
	if err != nil || !strings.Contains(string(log), `"kind":"model.call"`) || strings.Contains(string(log), `"story":`) {
		t.Fatalf("storyless model calls carry a story key (%v):\n%s", err, log)
	}
	// A Google-built head gets no Google review (reviewer ≠ builder), and a
	// screenshot the redaction pass did not read with OCR (a behaviour
	// control's, or any when tesseract is missing) is never sent to the model.
	calls := len(fakeModel.Requests)
	if sr := e.screenshotReview(context.Background(), arts, 0, verbs.ModelGemini); sr == nil || !strings.Contains(sr.Skipped, "google") {
		t.Fatalf("Google builder: %+v", sr)
	}
	unscanned := []verify.Evidence{{Path: "screens/2.png", Kind: "screenshot", Data: png}, {Path: "behaviour/f/c/shot.png", Kind: "screenshot", Data: png}}
	if sr := e.screenshotReview(context.Background(), unscanned, 0, ""); sr == nil || !strings.Contains(sr.Skipped, "OCR") || len(fakeModel.Requests) != calls {
		t.Fatalf("unscanned screenshots: %+v, %d calls", sr, len(fakeModel.Requests)-calls)
	}
	if sr := e.screenshotReview(context.Background(), unscanned[1:], 0, ""); sr != nil {
		t.Fatalf("a behaviour control's screenshot alone: %+v", sr)
	}
	// Findings that quote personal data from the page are not kept: the
	// report goes on the ledger.
	leak := models.Response{Text: `{"findings":[{"kind":"contrast","heuristic":4,"severity":3,"element":"jane.doe@acme.io","summary":"low contrast","fix":"darken"}],"nielsen":[0,0,0,0,0,0,0,0,0,0],"personas":[],"rebuild":false}`}
	leaky := &models.Fake{Replies: []models.Response{leak, leak, leak}}
	if sr := newE(leaky).screenshotReview(context.Background(), arts, 0, ""); sr == nil || len(sr.Findings) != 0 || !strings.Contains(sr.Skipped, "personal data") || strings.Contains(sr.Skipped, "jane.doe@acme.io") {
		t.Fatalf("findings quoting personal data: %+v", sr)
	}
}

// TestCopilotUIReviewRoute: a [models.ui-review.copilot] row naming a concrete
// model opts the binary's ui-review calls into the Copilot route, at the
// lane's effort, charged the credits Copilot reports. It fails closed: when
// copilot cannot be run or its reply does not parse, that call and every
// later one go to the normal flash-high route, the label says so, and the
// review still happens.
func TestCopilotUIReviewRoute(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")
	arts := []verify.Evidence{{Path: "screens/1.png", Kind: "screenshot", Data: png, OCRScanned: true}}
	argon := laneAReply("contrast")
	argon.CostUSD = 0.043
	setup := func(copilot models.Client) (*Env, *models.Fake, string) {
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, ".config", "aayudha"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, ".config", "aayudha", "config.toml"), []byte("version = 1\n[models.ui-review.copilot]\nmodel = \"gemini-4-argon\"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		gateway := &models.Fake{Replies: []models.Response{laneAReply("contrast"), laneAReply("contrast"), laneAReply()}}
		var log strings.Builder
		e, err := newEnv(IO{In: strings.NewReader(""), Out: io.Discard, Err: &log}, Options{Dir: dir,
			Getenv: func(k string) string {
				return map[string]string{"HOME": dir, "XDG_STATE_HOME": dir, "AAYU_SECRET_GOOGLE": "k", "COPILOT_GITHUB_TOKEN": "copilot-tok"}[k]
			},
			GHToken: func(context.Context) (string, error) { return "", os.ErrNotExist },
			ModelClient: func(r models.Route, key string) models.Client {
				if r.Via == models.CopilotRoute.Via {
					if key != "copilot-tok" {
						t.Errorf("the Copilot route got key %q, want COPILOT_GITHUB_TOKEN", key)
					}
					return copilot
				}
				return gateway
			}})
		if err != nil {
			t.Fatal(err)
		}
		return e, gateway, e.ActionsPath()
	}

	cp := &models.Fake{Replies: []models.Response{argon}}
	e, gateway, logPath := setup(cp)
	sr := e.screenshotReview(context.Background(), arts, 0, "")
	if sr == nil || sr.Skipped != "" || sr.Model != "gemini-4-argon via Copilot" || len(sr.Findings) != 1 || len(cp.Requests) != uireview.K || len(gateway.Requests) != 0 {
		t.Fatalf("Copilot route: %+v, %d Copilot and %d gateway calls", sr, len(cp.Requests), len(gateway.Requests))
	}
	if r := cp.Requests[0]; r.Model != "gemini-4-argon" || r.Effort != "high" || len(r.Messages[0].Images) != 1 {
		t.Fatalf("Copilot request: model %s, effort %q", r.Model, r.Effort)
	}
	if b, _ := os.ReadFile(logPath); !strings.Contains(string(b), `"via":"copilot"`) || !strings.Contains(string(b), `"cost_source":"reported"`) || !strings.Contains(string(b), `"cost_usd":0.043`) {
		t.Fatalf("actions log does not charge the reported credits:\n%s", b)
	}

	for name, copilot := range map[string]*models.Fake{
		"copilot missing":   {Err: fmt.Errorf("%w: copilot (GitHub Copilot CLI) is not installed", models.ErrCopilot)},
		"reply is not JSON": {Replies: []models.Response{{Text: "Sure! Here is my review."}}},
	} {
		e, gateway, _ := setup(copilot)
		sr := e.screenshotReview(context.Background(), arts, 0, "")
		if sr == nil || sr.Skipped != "" || len(sr.Findings) != 1 || len(copilot.Requests) != 1 || len(gateway.Requests) != uireview.K ||
			!strings.HasPrefix(sr.Model, verbs.ModelGemini+" (the Copilot route failed: ") {
			t.Errorf("%s: %+v, %d Copilot and %d gateway calls", name, sr, len(copilot.Requests), len(gateway.Requests))
		}
		if !strings.Contains(e.IO.Err.(*strings.Builder).String(), "lane A falls back to "+verbs.ModelGemini+" (high)") {
			t.Errorf("%s: the fallback is not reported: %q", name, e.IO.Err.(*strings.Builder).String())
		}
	}
}
