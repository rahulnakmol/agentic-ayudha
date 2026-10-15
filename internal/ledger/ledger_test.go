package ledger

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/github/scratch"
)

func run(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
	return strings.TrimSpace(string(out))
}

// origin makes a bare remote with a dev branch and an orphan ledger branch.
func origin(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	bare := filepath.Join(root, "origin.git")
	run(t, root, "init", "--quiet", "--bare", bare)
	seed := filepath.Join(root, "seed")
	run(t, root, "clone", "--quiet", bare, seed)
	run(t, seed, "checkout", "--quiet", "-b", "dev")
	_ = os.WriteFile(filepath.Join(seed, "README.md"), []byte("product\n"), 0o644)
	run(t, seed, "add", ".")
	run(t, seed, "commit", "--quiet", "-m", "init")
	run(t, seed, "push", "--quiet", "origin", "dev")
	run(t, seed, "checkout", "--quiet", "--orphan", "ledger")
	run(t, seed, "rm", "-rf", "--quiet", ".")
	_ = os.WriteFile(filepath.Join(seed, "README.md"), []byte("ledger\n"), 0o644)
	run(t, seed, "add", ".")
	run(t, seed, "commit", "--quiet", "-m", "ledger: create")
	run(t, seed, "push", "--quiet", "origin", "ledger")
	return bare
}

func clone(t *testing.T, bare string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "clone")
	run(t, filepath.Dir(dir), "clone", "--quiet", "--branch", "dev", bare, dir)
	return dir
}

func noSleep(time.Duration) {}

// WP2.4 accept: 5 concurrent writers all land and no commit is lost. Five run
// from separate clones (other machines: only fetch-rebase-retry orders them),
// five more share one clone (the file lock orders them).
func TestConcurrentWritersAllLand(t *testing.T) {
	bare := origin(t)
	shared := clone(t, bare)
	var wg sync.WaitGroup
	shas := make([]string, 10)
	errs := make([]error, 10)
	retries := make([]int, 10)
	for i := range 10 {
		dir := shared
		if i < 5 {
			dir = clone(t, bare)
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := &Writer{Dir: dir, Repo: "o/r", Sleep: noSleep, Attempts: 30, OnAttempt: func(a int, _ string) { retries[i] = a }}
			shas[i], errs[i] = w.Write(context.Background(), Commit{
				Message: fmt.Sprintf("specs: writer %d", i),
				Files:   map[string][]byte{fmt.Sprintf("specs/%d/ready.md", i): fmt.Appendf(nil, "writer %d\n", i)},
				Append:  map[string][]byte{"actions/2026-10-12/shared.jsonl": fmt.Appendf(nil, "{\"writer\":%d}\n", i)},
			})
		}()
	}
	wg.Wait()
	check := clone(t, bare)
	run(t, check, "fetch", "--quiet", "origin", "ledger")
	tip := run(t, check, "rev-parse", "FETCH_HEAD")
	if n := run(t, check, "rev-list", "--count", tip); n != "11" {
		t.Errorf("ledger has %s commits, want 11 (create + 10 writers)", n)
	}
	raced := 0
	for i := range 10 {
		if errs[i] != nil {
			t.Fatalf("writer %d: %v", i, errs[i])
		}
		if err := exec.Command("git", "-C", check, "merge-base", "--is-ancestor", shas[i], tip).Run(); err != nil {
			t.Errorf("writer %d's commit %s is not on the ledger (lost)", i, shas[i])
		}
		if got := run(t, check, "show", tip+":specs/"+fmt.Sprint(i)+"/ready.md"); got != fmt.Sprintf("writer %d", i) {
			t.Errorf("specs/%d/ready.md = %q", i, got)
		}
		if retries[i] > 0 {
			raced++
		}
	}
	if lines := strings.Count(run(t, check, "show", tip+":actions/2026-10-12/shared.jsonl"), "\n") + 1; lines != 10 {
		t.Errorf("appended segment has %d lines, want 10", lines)
	}
	if raced == 0 {
		t.Log("no writer had to retry this run; the race was not exercised")
	}
	// Temporary worktrees, refs, and the lock are cleaned up.
	if out := run(t, shared, "worktree", "list"); strings.Count(out, "\n") != 0 {
		t.Errorf("leftover worktrees:\n%s", out)
	}
	if out := run(t, shared, "for-each-ref", "refs/aayudha/"); out != "" {
		t.Errorf("leftover refs: %s", out)
	}
}

func TestNoopDryRunAndErrors(t *testing.T) {
	bare := origin(t)
	dir := clone(t, bare)
	ctx := context.Background()
	w := &Writer{Dir: dir, Sleep: noSleep}
	c := Commit{Message: "x", Files: map[string][]byte{"specs/1/ready.md": []byte("a\n")}}
	first, err := w.Write(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	if again, err := w.Write(ctx, c); err != nil || again != first {
		t.Fatalf("rewriting the same content: %s, %v; want the tip %s and no new commit", again, err, first)
	}

	log, _ := actions.Open(filepath.Join(t.TempDir(), "a.jsonl"), actions.Options{DryRun: true})
	dry := &Writer{Dir: dir, Log: log, Repo: "o/r", Identity: "human-fallback", Tag: "identity:human-fallback", Sleep: noSleep}
	if sha, err := dry.Write(ctx, Commit{Message: "y", Files: map[string][]byte{"specs/2/ready.md": []byte("b\n")}}); err != nil || sha != "" {
		t.Fatalf("dry run: %q, %v", sha, err)
	}
	run(t, dir, "fetch", "--quiet", "origin", "ledger")
	if tip := run(t, dir, "rev-parse", "FETCH_HEAD"); tip != first {
		t.Fatalf("dry run moved the ledger to %s", tip)
	}

	// A real write under the human fallback carries the tag as a trailer and is logged.
	log2, _ := actions.Open(filepath.Join(t.TempDir(), "b.jsonl"), actions.Options{})
	tagged := &Writer{Dir: dir, Log: log2, Repo: "o/r", Identity: "human-fallback", Tag: "identity:human-fallback", Sleep: noSleep}
	sha, err := tagged.Write(ctx, Commit{Message: "z", Files: map[string][]byte{"specs/3/ready.md": []byte("c\n")}})
	if err != nil || !strings.Contains(run(t, dir, "log", "-1", "--format=%B", sha), "Aayudha-Identity: identity:human-fallback") {
		t.Fatalf("tagged write: %v", err)
	}

	for _, bad := range []string{"/etc/passwd", "../x", "a/../b", `a\b`} {
		if _, err := w.Write(ctx, Commit{Files: map[string][]byte{bad: nil}}); err == nil {
			t.Errorf("path %q accepted", bad)
		}
	}
	empty := clone(t, bare)
	run(t, empty, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "missing.git"))
	run(t, filepath.Dir(bare), "init", "--quiet", "--bare", filepath.Join(filepath.Dir(bare), "noledger.git"))
	run(t, empty, "remote", "set-url", "origin", filepath.Join(filepath.Dir(bare), "noledger.git"))
	if _, err := (&Writer{Dir: empty, Sleep: noSleep}).Write(ctx, c); !errors.Is(err, ErrNoLedger) {
		t.Errorf("no ledger branch: %v", err)
	}
	if p := ContentPath("blobs", ".png", []byte("x")); p != "blobs/2d711642b726b04401627ca9fbac32f5c8530fb1903cc4db02258717921a4881.png" {
		t.Errorf("ContentPath = %s", p)
	}
}

func TestSink(t *testing.T) {
	bare := origin(t)
	dir := clone(t, bare)
	path := filepath.Join(t.TempDir(), "actions.jsonl")
	log, _ := actions.Open(path, actions.Options{Run: "r1", Host: "amp", Sink: Sink{&Writer{Dir: dir, Sleep: noSleep}}})
	for i := range 3 {
		if _, err := log.Do(context.Background(), actions.Action{Kind: "k"}, func(context.Context) (actions.Outcome, error) { return actions.Outcome{}, nil }); err != nil {
			t.Fatal(err)
		}
		if i != 1 {
			if err := log.Checkpoint(); err != nil {
				t.Fatal(err)
			}
		}
	}
	run(t, dir, "fetch", "--quiet", "origin", "ledger")
	date := time.Now().UTC().Format("2006-01-02")
	seg := run(t, dir, "show", "FETCH_HEAD:actions/"+date+"/amp-r1.jsonl")
	local, _ := os.ReadFile(path)
	if seg != strings.TrimSpace(string(local)) {
		t.Fatalf("segment on ledger:\n%s\nlocal log:\n%s", seg, local)
	}
}

// TestLiveConcurrentWriters runs the WP2.4 concurrency check against GitHub
// (AAYU_LIVE=1): a scratch repository with an orphan ledger, five writers in
// separate clones pushing at once with the token as an HTTP header.
func TestLiveConcurrentWriters(t *testing.T) {
	if os.Getenv("AAYU_LIVE") != "1" {
		t.Skip("set AAYU_LIVE=1 to run against GitHub")
	}
	tok := scratch.Token(t)
	admin := os.Getenv("AAYU_LIVE_ADMIN_TOKEN")
	if admin == "" {
		admin = tok
	}
	repo := scratch.Repo(t, admin)
	url := "https://github.com/" + repo + ".git"
	w0 := &Writer{Dir: t.TempDir(), Token: tok}
	seed := filepath.Join(t.TempDir(), "seed")
	if _, err := w0.git(context.Background(), filepath.Dir(seed), "clone", "--quiet", url, seed); err != nil {
		t.Fatal(err)
	}
	run(t, seed, "checkout", "--quiet", "--orphan", "ledger")
	run(t, seed, "rm", "-rf", "--quiet", ".")
	_ = os.WriteFile(filepath.Join(seed, "README.md"), []byte("ledger\n"), 0o644)
	run(t, seed, "add", ".")
	run(t, seed, "commit", "--quiet", "-m", "ledger: create")
	if _, err := w0.git(context.Background(), seed, "push", "--quiet", "origin", "ledger"); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	shas := make([]string, 5)
	errs := make([]error, 5)
	for i := range 5 {
		dir := filepath.Join(t.TempDir(), "clone")
		if _, err := w0.git(context.Background(), filepath.Dir(dir), "clone", "--quiet", url, dir); err != nil {
			t.Fatal(err)
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := &Writer{Dir: dir, Repo: repo, Token: tok, Attempts: 20}
			shas[i], errs[i] = w.Write(context.Background(), Commit{Message: fmt.Sprintf("specs: live writer %d", i),
				Files: map[string][]byte{fmt.Sprintf("specs/%d/ready.md", i): fmt.Appendf(nil, "writer %d\n", i)}})
		}()
	}
	wg.Wait()
	if _, err := w0.git(context.Background(), seed, "fetch", "--quiet", "origin", "ledger"); err != nil {
		t.Fatal(err)
	}
	tip := run(t, seed, "rev-parse", "FETCH_HEAD")
	for i := range 5 {
		if errs[i] != nil {
			t.Fatalf("writer %d: %v", i, errs[i])
		}
		if err := exec.Command("git", "-C", seed, "merge-base", "--is-ancestor", shas[i], tip).Run(); err != nil {
			t.Errorf("writer %d's commit %s is not on the ledger", i, shas[i])
		}
	}
	n := run(t, seed, "rev-list", "--count", tip)
	t.Logf("ledger tip %s: %s commits", tip, n)
	if n != "6" {
		t.Errorf("ledger has %s commits, want 6", n)
	}
}
