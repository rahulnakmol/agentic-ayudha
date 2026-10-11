package cli

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/telemetry"
)

// realGitWithLFS runs real commands but reports git-lfs as installed, so the
// tests do not depend on git-lfs being present on the machine.
func realGitWithLFS(ctx context.Context, dir, name string, args ...string) (string, error) {
	if name == "git" && len(args) > 0 && args[0] == "lfs" {
		return "git-lfs/3.5.1 (test)\n", nil
	}
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	return string(out), err
}

func gitInit(t *testing.T, branch string, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, body := range files {
		p := filepath.Join(dir, rel)
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		_ = os.WriteFile(p, []byte(body), 0o644)
	}
	for _, args := range [][]string{
		{"init", "-q", "-b", branch}, {"add", "-A"},
		{"-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "init"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	return dir
}

func noTools(string) (string, error) { return "", fmt.Errorf("not found") }

func TestDoctorCoexistenceReport(t *testing.T) {
	ci := "name: ci\non: [push]\njobs:\n  build:\n    runs-on: ubuntu-latest\n    steps: [{run: make}]\n  test:\n    name: unit tests\n    runs-on: ubuntu-latest\n    steps: [{run: make test}]\n"
	repo := gitInit(t, "master", map[string]string{".github/workflows/ci.yml": ci})
	before, _ := os.ReadFile(filepath.Join(repo, ".github/workflows/ci.yml"))
	code, out, errs := run(t, nil, Options{Dir: repo, Exec: realGitWithLFS, LookPath: noTools}, "doctor")
	if code != 0 {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	for _, want := range []string{
		"coexistence report: init adds alongside the existing setup",
		"existing workflow ci.yml (checks: build, unit tests): init leaves it byte-identical",
		"existing branches outside dev/rolling/main/ledger: master",
		`no dev branch: init writes [branches] dev = "master" to config instead of renaming anything`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("doctor output lacks %q:\n%s", want, out)
		}
	}
	if after, _ := os.ReadFile(filepath.Join(repo, ".github/workflows/ci.yml")); string(after) != string(before) {
		t.Fatal("doctor changed ci.yml")
	}
	// The run left a trace (WP1.8 acceptance).
	names, err := telemetry.ReadSpanNames(filepath.Join(repo, ".aayudha", "telemetry", "traces.jsonl"))
	if err != nil || !slices.Contains(names, "aayu doctor") {
		t.Fatalf("trace spans = %v, %v", names, err)
	}
	// --json carries the same report.
	_, out, _ = run(t, nil, Options{Dir: repo, Exec: realGitWithLFS, LookPath: noTools}, "doctor", "--json")
	var rep struct {
		Results  []struct{ ID, Status string }
		Blocking bool
	}
	if err := json.Unmarshal([]byte(out), &rep); err != nil || rep.Blocking || len(rep.Results) == 0 {
		t.Fatalf("--json: %v %s", err, out)
	}
}

func treeHash(t *testing.T, root string) string {
	h := sha256.New()
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		if d.IsDir() && (rel == ".git" || rel == filepath.Join(".aayudha", "telemetry")) {
			return filepath.SkipDir // git internals and doctor's own trace
		}
		if !d.IsDir() {
			b, _ := os.ReadFile(p)
			fmt.Fprintf(h, "%s %x\n", rel, sha256.Sum256(b))
		}
		return nil
	})
	return fmt.Sprintf("%x", h.Sum(nil))
}

func TestDoctorFixIsNoOpOnHealthyRepo(t *testing.T) {
	repo := gitInit(t, "dev", map[string]string{".aayudha/config.toml": "version = 1\n", "README.md": "x\n"})
	before := treeHash(t, repo)
	code, out, errs := run(t, nil, Options{Dir: repo, Exec: realGitWithLFS, LookPath: noTools}, "doctor", "--fix")
	if code != 0 || !strings.Contains(out, "--fix: nothing to re-sync") {
		t.Fatalf("exit %d\n%s%s", code, out, errs)
	}
	if after := treeHash(t, repo); after != before {
		t.Fatal("doctor --fix changed the repository")
	}
	status, _ := exec.Command("git", "-C", repo, "status", "--porcelain", "--untracked-files=all").Output()
	if s := strings.TrimSpace(string(status)); s != "" && !strings.Contains(s, ".aayudha/telemetry") {
		t.Fatalf("git status after --fix:\n%s", s)
	}
}

func TestDoctorFailsOnMissingGitLFS(t *testing.T) {
	repo := gitInit(t, "dev", nil)
	exe := func(ctx context.Context, dir, name string, args ...string) (string, error) {
		if name == "git" && len(args) > 0 && args[0] == "lfs" {
			return "", fmt.Errorf("git: 'lfs' is not a git command")
		}
		return realGitWithLFS(ctx, dir, name, args...)
	}
	if code, out, _ := run(t, nil, Options{Dir: repo, Exec: exe, LookPath: noTools}, "doctor"); code != ExitFloor || !strings.Contains(out, "git-lfs not found") {
		t.Fatalf("exit %d\n%s", code, out)
	}
}
