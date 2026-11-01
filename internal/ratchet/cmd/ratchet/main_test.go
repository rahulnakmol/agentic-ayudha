package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/agentenv"
	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
	"github.com/rahulnakmol/agentic-ayudha/internal/gittest"
)

func TestMain(m *testing.M) { gittest.Main(m) }

// evalToml is an eval.toml whose bars a change may lower.
const evalToml = "judge = \"gpt-6.1-sol\"\nk = 5\npass_score = 4\nmin_pass_k = 0.8\nmin_mean = 3.5\nmax_drop = 0.3\n\n[[groups]]\nname = \"skills\"\nmax_usd = 4.0\n"

// baseRepo commits a repository the ratchet passes on, plus files, as HEAD,
// changes into it, and returns how to write a file in its working tree.
func baseRepo(t *testing.T, files map[string]string) func(p, body string) {
	t.Helper()
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
	write("ratchets.toml", "[coverage]\nfloor = 50.0\nmargin = 0.5\n")
	write("coverage.out", "mode: set\na.go:1.1,2.2 1 1\na.go:3.1,4.2 1 0\n") // 50 %
	write("evals/eval.toml", evalToml)
	write("evals/cases/stub/one/case.toml", "skill = \"stub\"\ninput = \"input.txt\"\n")
	write("evals/cases/stub/one/input.txt", "x\n")
	for p, body := range files {
		write(p, body)
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "."}, {"commit", "-q", "-m", "base"}} {
		cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.com"}, args...)...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	t.Chdir(repo)
	return write
}

// CI's one ratchet step fails a PR that lowers a skill-eval bar, as it fails
// one that loosens the coverage floor (Story #400).
func TestRunFailsALoweredEvalBar(t *testing.T) {
	write := baseRepo(t, nil)
	if err := run("coverage.out", "HEAD", false); err != nil {
		t.Fatalf("nothing lowered: %v", err)
	}
	write("evals/eval.toml", strings.Replace(evalToml, "pass_score = 4", "pass_score = 3", 1))
	err := run("coverage.out", "HEAD", false)
	if err == nil || !strings.HasPrefix(err.Error(), "eval-bar ratchet: pass_score lowers a skill-eval bar without a new waiver: 4 → 3\n::error::eval-bar ratchet: ") {
		t.Fatalf("pass_score lowered: %v", err)
	}
}

// Bug #1211, criterion 1: a change to the agent environment (its spec or its
// setup) fails CI's ratchet step while a host of the spec's hosts has no
// record of the new environment; it names each such host and the command
// that verifies it. A record left stale by an older change fails nothing.
func TestRunFailsAStaleEnvironmentRecord(t *testing.T) {
	spec := "version = 1\nhosts = ['amp', 'cursor']\n"
	stale := "[container.amp]\nfingerprint = 'sha256:0000000000000000'\n[container.cursor]\nfingerprint = 'sha256:0000000000000000'\n"
	write := baseRepo(t, map[string]string{agentenv.SpecPath: spec, agentenv.SetupPath: "#!/usr/bin/env bash\n", agentenv.VerifiedPath: stale})
	if err := run("coverage.out", "HEAD", false); err != nil {
		t.Fatalf("no environment change, stale records: %v", err)
	}
	named := func(err error) []string {
		var hosts []string
		for _, h := range []string{"amp", "cursor"} {
			if err != nil && strings.Contains(err.Error(), "verified for "+h+": run `aayu install --env --host "+h+" --apply --verify` and commit "+agentenv.VerifiedPath) {
				hosts = append(hosts, h)
			}
		}
		return hosts
	}
	write(agentenv.SpecPath, spec+"packages = ['age']\n")
	err := run("coverage.out", "HEAD", false)
	if got := named(err); !slices.Equal(got, []string{"amp", "cursor"}) || !strings.HasPrefix(err.Error(), "environment record: ") {
		t.Fatalf("spec changed, both records stale: named %v in %v", got, err)
	}
	s, _, _ := agentenv.LoadSpec(".")
	write(agentenv.VerifiedPath, stale+"[cloud.cursor]\nfingerprint = '"+s.Fingerprint()+"'\n")
	if got := named(run("coverage.out", "HEAD", false)); !slices.Equal(got, []string{"amp"}) {
		t.Fatalf("cursor verified live for the new spec: named %v", got)
	}
	write(agentenv.SpecPath, spec)
	write(agentenv.SetupPath, "#!/usr/bin/env bash\necho hand edit\n")
	if got := named(run("coverage.out", "HEAD", false)); !slices.Equal(got, []string{"amp", "cursor"}) {
		t.Fatalf("setup changed: named %v", got)
	}
}

// A coverage-floor waiver another person accepted and closed does not lower
// the floor once the founder is the floor-waiver authority; the founder's
// own does (Bug #599).
func TestCheckWaiverAuthority(t *testing.T) {
	g := fake.New(t, "o/r", "User")
	story := g.AddIssue("A story", "x", "type:story")
	waiver := func(by string) int {
		d := g.AddIssue("Waive the coverage floor on #"+strconv.Itoa(story.Number), "### Decision\n\nwaive, accepted by @"+by+" on 2026-10-18.\n", "type:decision", "decision:waiver")
		g.Mu.Lock()
		defer g.Mu.Unlock()
		d.State, d.StateReason, d.ClosedBy = "closed", "completed", &fake.Account{Login: by, Type: "User"}
		d.Parent, story.Subs = story.Number, append(story.Subs, d.Number)
		return d.Number
	}
	other, own := waiver("mallory"), waiver("founder")
	repo := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", repo).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	t.Chdir(repo)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("GITHUB_REPOSITORY", "o/r")
	t.Setenv("GITHUB_API_URL", g.URL())
	t.Setenv("GH_TOKEN", "t")
	t.Setenv("AAYU_AUTONOMY__APPROVER", "founder")
	if err := checkWaiver(other, "coverage"); err == nil || !strings.Contains(err.Error(), "closed by @mallory, not the floor-waiver authority @founder") {
		t.Errorf("another person's waiver: %v", err)
	}
	if err := checkWaiver(own, "coverage"); err != nil {
		t.Errorf("the founder's waiver: %v", err)
	}
	// A checkout config that does not load cannot name the authority, so no
	// waiver passes, not even the founder's.
	if err := os.MkdirAll(".aayudha", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(".aayudha", "config.toml"), []byte("version = 1\n[autonomy\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := checkWaiver(own, "coverage"); err == nil {
		t.Error("a waiver passed with a config that does not load")
	}
}
