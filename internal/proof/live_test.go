package proof

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/github/scratch"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
)

// WP4.2 accept (live): on a private scratch repository the screenshot of a
// pack goes to Git LFS on the ledger, the pack verifies by downloading it
// back from GitHub's LFS store, and purge removes it from the ledger tip with
// a tombstone. A public repository without a bucket refuses media.
func TestLiveLFSScratch(t *testing.T) {
	if os.Getenv("AAYU_LIVE") != "1" {
		t.Skip("set AAYU_LIVE=1 to run against GitHub")
	}
	if _, err := exec.LookPath("git-lfs"); err != nil {
		t.Fatal("git-lfs is required for the live LFS test")
	}
	ctx := context.Background()
	tok := scratch.Token(t)
	admin := os.Getenv("AAYU_LIVE_ADMIN_TOKEN")
	if admin == "" {
		admin = tok
	}
	repo := scratch.Repo(t, admin) // private
	url := "https://github.com/" + repo + ".git"
	w := &ledger.Writer{Dir: filepath.Join(t.TempDir(), "clone"), Repo: repo, Token: tok}
	seed := filepath.Join(t.TempDir(), "seed")
	gitTok(t, tok, filepath.Dir(seed), "clone", "--quiet", url, seed)
	run(t, seed, "checkout", "--quiet", "--orphan", "ledger")
	run(t, seed, "rm", "-rf", "--quiet", ".")
	var ga strings.Builder
	for _, e := range MediaExts {
		ga.WriteString("*." + e + " filter=lfs diff=lfs merge=lfs -text\n")
	}
	_ = os.WriteFile(filepath.Join(seed, ".gitattributes"), []byte(ga.String()), 0o644)
	run(t, seed, "add", ".")
	run(t, seed, "commit", "--quiet", "-m", "ledger: create")
	gitTok(t, tok, seed, "push", "--quiet", "origin", "ledger")
	gitTok(t, tok, filepath.Dir(w.Dir), "clone", "--quiet", url, w.Dir)

	st := &Store{Repo: repo, Ledger: w, Private: true}
	pdir, _ := localPack(t)
	res, err := st.Push(ctx, pdir, true, Authority{})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(res.Stores, "screens/2-empty.png=lfs") {
		t.Fatalf("stores: %v", res.Stores)
	}
	tree, err := w.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	e, err := st.Best(ctx, tree, 7)
	if err != nil {
		t.Fatal(err)
	}
	ptr, _ := tree.File(ctx, "proof/7/"+head+"/advisory/artifacts/screens/2-empty.png")
	if !strings.HasPrefix(string(ptr), "version https://git-lfs.github.com/spec/v1") {
		t.Fatalf("not an LFS pointer on the ledger: %q", ptr)
	}
	r := Verify(e.Pack, Expect{HeadSHA: head, CompletionGoalHash: cg, Criteria: []int{1, 2}}, st.Opener(ctx, tree, e))
	if !r.OK || r.Hashed != 3 {
		t.Fatalf("verify through GitHub LFS: %+v", r)
	}
	t.Logf("pushed %s at %s; verified %d artifacts (screenshot downloaded from GitHub LFS)", res.Path, res.Commit, r.Hashed)
	pr, err := st.Purge(ctx, tree, 7, "live test")
	tree.Close()
	if err != nil || len(pr.Files) != 1 || len(pr.Packs) != 1 {
		t.Fatalf("purge: %+v %v", pr, err)
	}
	tree, _ = w.Read(ctx)
	defer tree.Close()
	if _, err := tree.File(ctx, pr.Files[0]); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("LFS file still on the ledger tip: %v", err)
	}
	e, _ = st.Best(ctx, tree, 7)
	if a, _ := e.Pack.Artifact("screens/2-empty.png"); a.Tombstone == nil {
		t.Fatalf("no tombstone after purge: %+v", a)
	}

	// The same pack on a repository treated as public, with no bucket, is refused.
	pub := &Store{Repo: repo, Ledger: w, Private: false}
	if _, err := pub.Push(ctx, pdir, true, Authority{}); !errors.Is(err, ErrMediaRefused) {
		t.Fatalf("public push --media without a bucket: %v", err)
	}
}

// WP4.2 (live, bucket): push media to a real S3-compatible bucket, set its
// retention rule, verify from it, and purge. Needs AAYU_LIVE_BUCKET (a [proof]
// media value) and AAYU_LIVE_BUCKET_KEYS (ACCESS_KEY_ID:SECRET_ACCESS_KEY);
// the ledger is a local bare repository, since the bucket is what is live.
func TestLiveBucket(t *testing.T) {
	if os.Getenv("AAYU_LIVE") != "1" || os.Getenv("AAYU_LIVE_BUCKET") == "" {
		t.Skip("set AAYU_LIVE=1, AAYU_LIVE_BUCKET, and AAYU_LIVE_BUCKET_KEYS to run against a bucket")
	}
	ctx := context.Background()
	b, err := ParseMedia(os.Getenv("AAYU_LIVE_BUCKET"))
	if err != nil {
		t.Fatal(err)
	}
	var ok bool
	b.AccessKey, b.SecretKey, ok = strings.Cut(os.Getenv("AAYU_LIVE_BUCKET_KEYS"), ":")
	if !ok {
		t.Fatal("AAYU_LIVE_BUCKET_KEYS must be ACCESS_KEY_ID:SECRET_ACCESS_KEY")
	}
	dir, _ := ledgerRemote(t, false)
	repo := "aayu-live/bucket-test"
	st := &Store{Repo: repo, Ledger: &ledger.Writer{Dir: dir, Repo: repo}, Bucket: b, RetentionDays: 1}
	pdir, p := localPack(t)
	if _, err := st.Push(ctx, pdir, true, Authority{}); err != nil {
		t.Fatal(err)
	}
	key := b.Key(repo, 7, p.Artifacts[2].SHA256)
	if got, err := b.Get(ctx, key); err != nil || string(got) != "\x89PNG fake screenshot bytes" {
		t.Fatalf("object %s: %q %v", key, got, err)
	}
	if rule, err := b.Lifecycle(ctx, repo); err != nil || !strings.Contains(normalizeXML(rule), "<Days>1</Days>") {
		t.Fatalf("lifecycle rule: %q %v", rule, err)
	}
	tree, _ := st.Ledger.Read(ctx)
	e, _ := st.Best(ctx, tree, 7)
	if r := Verify(e.Pack, Expect{HeadSHA: head}, st.Opener(ctx, tree, e)); !r.OK || r.Hashed != 3 {
		t.Fatalf("verify from the bucket: %+v", r)
	}
	pr, err := st.Purge(ctx, tree, 7, "live test")
	tree.Close()
	if err != nil || len(pr.Objects) != 1 {
		t.Fatalf("purge: %+v %v", pr, err)
	}
	if _, err := b.Get(ctx, key); !errors.Is(err, ErrNoObject) {
		t.Fatalf("object survived purge: %v", err)
	}
	t.Cleanup(func() { _, _ = b.EnsureLifecycle(context.Background(), repo, 0) })
}

func gitTok(t *testing.T, tok, dir string, args ...string) {
	t.Helper()
	w := &ledger.Writer{Dir: dir, Token: tok}
	if err := w.Exec(context.Background(), dir, args...); err != nil {
		t.Fatal(err)
	}
}
