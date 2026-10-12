// Package ledger writes to the orphan `ledger` branch (WP2.4): each write
// builds one commit in a temporary worktree on the current ledger tip and
// pushes it, fetching, re-applying, and retrying with bounded backoff when
// another writer got there first. A per-repository file lock serializes writers
// that share one clone; the fetch-rebase-retry loop serializes writers on
// different machines. WP4.2 adds Git LFS for proof media on private
// repositories (Writer.LFS), deletions, and reads of the ledger tip.
package ledger

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
)

// Errors.
var (
	ErrNoLedger  = errors.New("no ledger branch on the remote; run `aayu init` to create it")
	ErrContended = errors.New("ledger push kept losing the race to other writers")
	ErrLocked    = errors.New("another aayudha process holds the ledger lock")
)

// Commit is one ledger write.
type Commit struct {
	Message string
	Files   map[string][]byte // path -> full content (replaced)
	Append  map[string][]byte // path -> bytes appended to the file at the tip (created when absent)
	Delete  []string          // paths removed at the tip (absent paths are ignored)
}

// Writer writes commits to the ledger branch of the repository cloned at Dir.
type Writer struct {
	Dir      string // a clone or checkout of the repository
	Remote   string // default "origin"
	Branch   string // default "ledger"
	Repo     string // owner/name, the action target
	Token    string // optional: sent as an HTTP header through the environment, never on the command line
	Author   string // "Name <email>" of the commit author; default the clone's git identity, else aayudha
	Log      *actions.Log
	Identity string // stamped on the action: bot, human, or human-fallback
	Tag      string // e.g. identity:human-fallback, recorded as a commit trailer
	// LFS runs the ledger's .gitattributes LFS filters on commit and pushes the
	// LFS objects before the commit (proof media on private repositories, V3).
	LFS bool

	Attempts    int                           // push attempts before ErrContended; default 10
	Backoff     func(n int) time.Duration     // wait before attempt n+1; default 100ms·2^n with jitter, capped at 5s
	LockTimeout time.Duration                 // how long to wait for the clone's lock; default 2 minutes
	Sleep       func(time.Duration)           // default time.Sleep
	OnAttempt   func(attempt int, tip string) // test hook
}

func (w *Writer) defaults() {
	if w.Remote == "" {
		w.Remote = "origin"
	}
	if w.Branch == "" {
		w.Branch = "ledger"
	}
	if w.Attempts == 0 {
		w.Attempts = 10
	}
	if w.Backoff == nil {
		w.Backoff = func(n int) time.Duration {
			d := 100 * time.Millisecond << min(n, 6)
			j, _ := rand.Int(rand.Reader, big.NewInt(int64(d)))
			return min(d/2+time.Duration(j.Int64()), 5*time.Second)
		}
	}
	if w.LockTimeout == 0 {
		w.LockTimeout = 2 * time.Minute
	}
	if w.Sleep == nil {
		w.Sleep = time.Sleep
	}
}

// ContentPath is a content-addressed ledger path: dir/<sha256 of data><ext>.
// Two writers that store the same bytes write the same path, and no write can
// change what a content-addressed path holds.
func ContentPath(dir, ext string, data []byte) string {
	sum := sha256.Sum256(data)
	return path.Join(dir, hex.EncodeToString(sum[:])+ext)
}

// Write lands c on the ledger branch and returns the commit that holds it.
// When c changes nothing at the tip, it returns the tip and pushes nothing.
// With a Log, the push is an action (dry-run plans it and writes nothing).
func (w *Writer) Write(ctx context.Context, c Commit) (string, error) {
	w.defaults()
	paths := slices.Sorted(func(yield func(string) bool) {
		for p := range c.Files {
			if !yield(p) {
				return
			}
		}
		for p := range c.Append {
			if !yield(p) {
				return
			}
		}
		for _, p := range c.Delete {
			if !yield(p) {
				return
			}
		}
	})
	for _, p := range paths {
		if err := checkPath(p); err != nil {
			return "", err
		}
	}
	if w.Log == nil {
		return w.write(ctx, c)
	}
	var sha string
	rec, err := w.Log.Do(ctx, actions.Action{
		Kind: "ledger.push", Target: w.Repo + "@" + w.Branch, Identity: w.Identity,
		Payload: map[string]any{"paths": paths, "message": c.Message, "tag": w.Tag},
	}, func(ctx context.Context) (actions.Outcome, error) {
		var err error
		sha, err = w.write(ctx, c)
		return actions.Outcome{Result: map[string]any{"commit": sha}}, err
	})
	if rec.Status == actions.StatusPlanned {
		return "", err
	}
	return sha, err
}

func checkPath(p string) error {
	if p == "" || strings.HasPrefix(p, "/") || strings.Contains(p, "\\") || path.Clean(p) != p || strings.HasPrefix(p, "../") || p == ".." {
		return fmt.Errorf("ledger path %q: want a clean relative path with / separators", p)
	}
	return nil
}

func (w *Writer) write(ctx context.Context, c Commit) (string, error) {
	common, err := w.git(ctx, w.Dir, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", err
	}
	unlock, err := lock(filepath.Join(common, "aayudha-ledger.lock"), w.LockTimeout, w.Sleep)
	if err != nil {
		return "", err
	}
	defer unlock()

	id := randID()
	ref := "refs/aayudha/ledger-tip/" + id
	tmp, err := os.MkdirTemp("", "aayudha-ledger-")
	if err != nil {
		return "", err
	}
	wt := filepath.Join(tmp, "wt")
	defer func() {
		_, _ = w.git(context.WithoutCancel(ctx), w.Dir, "worktree", "remove", "--force", wt)
		_, _ = w.git(context.WithoutCancel(ctx), w.Dir, "update-ref", "-d", ref)
		_ = os.RemoveAll(tmp)
		_, _ = w.git(context.WithoutCancel(ctx), w.Dir, "worktree", "prune")
	}()

	for attempt := 0; attempt < w.Attempts; attempt++ {
		if attempt > 0 {
			w.Sleep(w.Backoff(attempt - 1))
		}
		if _, err := w.git(ctx, w.Dir, "fetch", "--no-tags", "--quiet", w.Remote, "+refs/heads/"+w.Branch+":"+ref); err != nil {
			if strings.Contains(err.Error(), "couldn't find remote ref") {
				return "", ErrNoLedger
			}
			return "", err
		}
		tip, err := w.git(ctx, w.Dir, "rev-parse", ref)
		if err != nil {
			return "", err
		}
		if w.OnAttempt != nil {
			w.OnAttempt(attempt, tip)
		}
		if attempt == 0 {
			if _, err := w.git(ctx, w.Dir, "worktree", "add", "--detach", "--quiet", wt, tip); err != nil {
				return "", err
			}
		} else if _, err := w.git(ctx, wt, "reset", "--hard", "--quiet", tip); err != nil {
			return "", err
		}
		if err := apply(wt, c); err != nil {
			return "", err
		}
		if _, err := w.git(ctx, wt, "add", "--all", "--", "."); err != nil {
			return "", err
		}
		if _, err := w.git(ctx, wt, "diff", "--cached", "--quiet"); err == nil {
			return tip, nil // nothing to change
		}
		msg := c.Message
		if w.Tag != "" {
			msg += "\n\nAayudha-Identity: " + w.Tag
		}
		if _, err := w.git(ctx, wt, "commit", "--quiet", "--no-verify", "-m", msg); err != nil {
			return "", err
		}
		sha, err := w.git(ctx, wt, "rev-parse", "HEAD")
		if err != nil {
			return "", err
		}
		if w.LFS {
			if _, err := w.git(ctx, wt, "lfs", "push", w.Remote, sha); err != nil {
				return "", fmt.Errorf("ledger LFS push: %w", err)
			}
		}
		_, err = w.git(ctx, wt, "push", "--quiet", "--porcelain", w.Remote, sha+":refs/heads/"+w.Branch)
		if err == nil {
			return sha, nil
		}
		if !rejected(err) {
			return "", err
		}
	}
	return "", fmt.Errorf("%w after %d attempts", ErrContended, w.Attempts)
}

// rejected reports whether a push failed because the remote moved on. GitHub
// answers a lost concurrent ref update with "[remote rejected] (failed)"; a
// ruleset refusal carries its own reason and is not retried.
func rejected(err error) bool {
	s := err.Error()
	for _, m := range []string{"[rejected]", "non-fast-forward", "fetch first", "stale info", "cannot lock ref", "failed to update ref", "reference already exists", "[remote rejected] (failed)", "incorrect old value"} {
		if strings.Contains(s, m) {
			return true
		}
	}
	return false
}

func apply(wt string, c Commit) error {
	for _, p := range c.Delete {
		if err := os.Remove(filepath.Join(wt, filepath.FromSlash(p))); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	for p, b := range c.Files {
		f := filepath.Join(wt, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(f, b, 0o644); err != nil {
			return err
		}
	}
	for p, b := range c.Append {
		f := filepath.Join(wt, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
			return err
		}
		h, err := os.OpenFile(f, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		if _, err := h.Write(b); err != nil {
			_ = h.Close()
			return err
		}
		if err := h.Close(); err != nil {
			return err
		}
	}
	return nil
}

// git runs git with a scrubbed, deterministic environment. The token, if any,
// travels as an http.extraHeader in GIT_CONFIG_* variables, so it is never on
// a command line.
func (w *Writer) git(ctx context.Context, dir string, args ...string) (string, error) {
	out, err := w.gitIn(ctx, dir, nil, args...)
	return strings.TrimSpace(string(out)), err
}

// Exec runs git in dir with the writer's scrubbed environment and token
// (clones and pushes of scratch repositories in live tests).
func (w *Writer) Exec(ctx context.Context, dir string, args ...string) error {
	_, err := w.git(ctx, dir, args...)
	return err
}

// Output runs git in dir like Exec and returns its trimmed standard output
// (release's promotion steps).
func (w *Writer) Output(ctx context.Context, dir string, args ...string) (string, error) {
	return w.git(ctx, dir, args...)
}

func (w *Writer) gitIn(ctx context.Context, dir string, stdin []byte, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	env := []string{"GIT_TERMINAL_PROMPT=0", "LC_ALL=C"}
	if len(args) < 2 || args[0] != "lfs" || args[1] != "smudge" {
		env = append(env, "GIT_LFS_SKIP_SMUDGE=1") // checkouts never download media; Tree.Content does on request
	}
	for _, k := range []string{"PATH", "HOME", "SYSTEMROOT", "TMPDIR", "TEMP", "TMP", "XDG_CONFIG_HOME", "SSH_AUTH_SOCK", "GIT_SSH_COMMAND"} {
		if v, ok := os.LookupEnv(k); ok {
			env = append(env, k+"="+v)
		}
	}
	n := 0
	cfg := func(k, v string) {
		env = append(env, fmt.Sprintf("GIT_CONFIG_KEY_%d=%s", n, k), fmt.Sprintf("GIT_CONFIG_VALUE_%d=%s", n, v))
		n++
	}
	if w.Token != "" {
		basic := base64.StdEncoding.EncodeToString([]byte("x-access-token:" + w.Token))
		cfg("http.https://github.com/.extraheader", "AUTHORIZATION: basic "+basic)
	}
	if name, email, ok := parseAuthor(w.Author); ok {
		cfg("user.name", name)
		cfg("user.email", email)
	} else if out, err := exec.CommandContext(ctx, "git", "-C", dir, "config", "user.email").Output(); err != nil || len(bytes.TrimSpace(out)) == 0 {
		cfg("user.name", "aayudha")
		cfg("user.email", "aayudha@users.noreply.github.com")
	}
	cfg("commit.gpgsign", "false")
	if w.LFS {
		cfg("filter.lfs.clean", "git-lfs clean -- %f")
		cfg("filter.lfs.smudge", "git-lfs smudge --skip -- %f")
		cfg("filter.lfs.process", "git-lfs filter-process --skip")
		cfg("filter.lfs.required", "true")
	}
	env = append(env, "GIT_CONFIG_COUNT="+strconv.Itoa(n))
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git %s: %w: %s%s", args[0], err, strings.TrimSpace(stderr.String()), strings.TrimSpace(stdout.String()))
	}
	return stdout.Bytes(), nil
}

func parseAuthor(a string) (name, email string, ok bool) {
	i, j := strings.LastIndex(a, "<"), strings.LastIndex(a, ">")
	if i <= 0 || j < i {
		return "", "", false
	}
	return strings.TrimSpace(a[:i]), a[i+1 : j], true
}

func randID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// lock takes an exclusive lock file. A lock older than staleAfter is taken over
// (its holder died).
func lock(path string, timeout time.Duration, sleep func(time.Duration)) (func(), error) {
	const staleAfter = 10 * time.Minute
	deadline := time.Now().Add(timeout)
	for {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			_, _ = fmt.Fprintf(f, "%d %s\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339))
			_ = f.Close()
			return func() { _ = os.Remove(path) }, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, err
		}
		if st, serr := os.Stat(path); serr == nil && time.Since(st.ModTime()) > staleAfter {
			_ = os.Remove(path)
			continue
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("%w (%s)", ErrLocked, path)
		}
		sleep(50 * time.Millisecond)
	}
}

// Tree is a read-only view of the ledger tip, fetched once.
type Tree struct {
	w   *Writer
	ref string
	Tip string
}

// Read fetches the ledger tip. Close releases the private ref.
func (w *Writer) Read(ctx context.Context) (*Tree, error) {
	w.defaults()
	ref := "refs/aayudha/ledger-read/" + randID()
	if _, err := w.git(ctx, w.Dir, "fetch", "--no-tags", "--quiet", w.Remote, "+refs/heads/"+w.Branch+":"+ref); err != nil {
		if strings.Contains(err.Error(), "couldn't find remote ref") {
			return nil, ErrNoLedger
		}
		return nil, err
	}
	tip, err := w.git(ctx, w.Dir, "rev-parse", ref)
	if err != nil {
		return nil, err
	}
	return &Tree{w: w, ref: ref, Tip: tip}, nil
}

// ReadAt fetches the ledger and returns a view of commit sha, which must be the
// tip or one of its ancestors ("" means the tip). `audit` reads a recorded
// ledger commit so a report can be regenerated byte for byte (WP7.6).
func (w *Writer) ReadAt(ctx context.Context, sha string) (*Tree, error) {
	t, err := w.Read(ctx)
	if err != nil || sha == "" {
		return t, err
	}
	full, err := w.git(ctx, w.Dir, "rev-parse", "--verify", "--quiet", sha+"^{commit}")
	if err == nil {
		_, err = w.git(ctx, w.Dir, "merge-base", "--is-ancestor", full, t.Tip)
	}
	if err != nil {
		t.Close()
		return nil, fmt.Errorf("ledger commit %s is not on the %s branch", sha, w.Branch)
	}
	t.Tip = full
	return t, nil
}

// Entry is one ledger commit.
type Entry struct {
	SHA      string    `json:"sha"`
	Subject  string    `json:"subject"`
	Author   string    `json:"author"`
	At       time.Time `json:"at"`
	Identity string    `json:"identity,omitempty"` // the Aayudha-Identity trailer (Writer.Tag)
}

// Log returns the commits reachable from the tree's commit, oldest first.
func (t *Tree) Log(ctx context.Context) ([]Entry, error) {
	out, err := t.w.git(ctx, t.w.Dir, "log", "--reverse", "--format=%H%x1f%s%x1f%an%x1f%cI%x1f%(trailers:key=Aayudha-Identity,valueonly,separator=%x2c)%x1e", t.Tip)
	if err != nil {
		return nil, err
	}
	var es []Entry
	for _, rec := range strings.Split(out, "\x1e") {
		f := strings.Split(strings.TrimSpace(rec), "\x1f")
		if len(f) != 5 {
			continue
		}
		at, _ := time.Parse(time.RFC3339, f[3])
		es = append(es, Entry{SHA: f[0], Subject: f[1], Author: f[2], At: at.UTC(), Identity: strings.TrimSpace(f[4])})
	}
	return es, nil
}

// Close deletes the tree's private ref.
func (t *Tree) Close() { _, _ = t.w.git(context.Background(), t.w.Dir, "update-ref", "-d", t.ref) }

// File returns the bytes of p at the tip as stored in git (an LFS pointer for LFS files).
func (t *Tree) File(ctx context.Context, p string) ([]byte, error) {
	if err := checkPath(p); err != nil {
		return nil, err
	}
	b, err := t.w.gitIn(ctx, t.w.Dir, nil, "cat-file", "blob", t.Tip+":"+p)
	if err != nil && (strings.Contains(err.Error(), "does not exist") || strings.Contains(err.Error(), "Not a valid object name") || strings.Contains(err.Error(), "exists on disk, but not in")) {
		return nil, fmt.Errorf("%s: %w", p, os.ErrNotExist)
	}
	return b, err
}

// Content returns the content of p, downloading it from LFS when the blob is an LFS pointer.
func (t *Tree) Content(ctx context.Context, p string) ([]byte, error) {
	b, err := t.File(ctx, p)
	if err != nil || !bytes.HasPrefix(b, []byte("version https://git-lfs.github.com/spec/v1")) {
		return b, err
	}
	w := *t.w
	w.LFS = false
	return w.gitIn(ctx, t.w.Dir, b, "lfs", "smudge", "--", p)
}

// List returns the paths of files under prefix at the tip, sorted.
func (t *Tree) List(ctx context.Context, prefix string) ([]string, error) {
	out, err := t.w.git(ctx, t.w.Dir, "ls-tree", "-r", "--name-only", t.Tip, "--", prefix)
	if err != nil || out == "" {
		return nil, err
	}
	ps := strings.Split(out, "\n")
	slices.Sort(ps)
	return ps, nil
}

// Sink is the ledger segment sink for the actions log (WP1.5): each checkpoint
// appends its lines to actions/<date>/<host>-<run>.jsonl on the ledger. A
// segment push carries the log itself, so it is not logged as an action.
type Sink struct{ W *Writer }

// Append implements actions.SegmentSink.
func (s Sink) Append(date, host, run string, lines [][]byte) error {
	if host == "" {
		host = "cli"
	}
	var b bytes.Buffer
	for _, l := range lines {
		b.Write(l)
		b.WriteByte('\n')
	}
	w := *s.W
	w.Log = nil
	p := path.Join("actions", date, host+"-"+run+".jsonl")
	_, err := w.Write(context.Background(), Commit{Message: "actions: segment " + host + "-" + run, Append: map[string][]byte{p: b.Bytes()}})
	return err
}
