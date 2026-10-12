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
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/proof/fakes3"
)

const (
	head  = "1111111111111111111111111111111111111111"
	other = "2222222222222222222222222222222222222222"
	cg    = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
)

// sample is a pack with a data artifact for criterion 1 and a screenshot for
// criterion 2, plus the bytes of both.
func sample() (*Pack, map[string][]byte) {
	files := map[string][]byte{
		"coverage/diff.json":   []byte(`{"covered":9,"total":10}`),
		"screens/2-empty.png":  []byte("\x89PNG fake screenshot bytes"),
		"reports/semgrep.json": []byte(`{"results":[]}`),
	}
	p := &Pack{Repo: "o/r", Story: 7, PR: 9, PRHeadSHA: head, Seat: "ai-native-engineer", Host: "claude", Identity: "bot",
		Authority: Authority{Kind: Advisory}, CompletionGoalHash: cg,
		Criteria: []Criterion{{ID: 1, Evidence: []string{"coverage/diff.json"}}, {ID: 2, Evidence: []string{"screens/2-empty.png"}}},
		Artifacts: []Artifact{
			NewArtifact("coverage/diff.json", "metric", files["coverage/diff.json"]),
			NewArtifact("screens/2-empty.png", "screenshot", files["screens/2-empty.png"]),
			NewArtifact("reports/semgrep.json", "report", files["reports/semgrep.json"]),
		},
		Checks:    []Check{{Floor: "coverage", Result: Pass, Value: 90, Threshold: 90, Tool: "aayudha", Version: "dev"}},
		Links:     Links{Issue: "o/r#7", PR: "o/r#9", ReadySnapshot: "specs/7/ready.md"},
		CreatedAt: "2026-10-12T10:00:00Z"}
	for i := range p.Artifacts {
		p.Artifacts[i].Store = StoreGit
	}
	return p, files
}

// WP4.1 accept: the schema validates samples (and rejects broken ones).
func TestSchemaValidatesSamples(t *testing.T) {
	p, _ := sample()
	b, err := Encode(p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decode(b); err != nil {
		t.Fatalf("sample pack: %v", err)
	}
	if errs := Validate(schemaV1, mustRead(t, "testdata/pack-v1.json")); len(errs) > 0 {
		t.Fatalf("testdata/pack-v1.json: %v", errs)
	}
	for name, mutate := range map[string]func(string) string{
		"short head sha": func(s string) string { return strings.Replace(s, head, "abc123", 1) },
		"unknown kind":   func(s string) string { return strings.Replace(s, `"kind": "metric"`, `"kind": "video"`, 1) },
		"absolute path": func(s string) string {
			return strings.Replace(s, `"path": "coverage/diff.json"`, `"path": "/coverage/diff.json"`, 1)
		},
		"unknown result":    func(s string) string { return strings.Replace(s, `"result": "pass"`, `"result": "ok"`, 1) },
		"extra field":       func(s string) string { return strings.Replace(s, `"story": 7,`, `"story": 7, "verdict": "merge",`, 1) },
		"missing checks":    func(s string) string { return strings.Replace(s, `"checks"`, `"checkz"`, 1) },
		"newer schema":      func(s string) string { return strings.Replace(s, `"schema_version": 1`, `"schema_version": 2`, 1) },
		"store not allowed": func(s string) string { return strings.Replace(s, `"store": "git"`, `"store": "s3"`, 1) },
	} {
		bad := mutate(string(b))
		if bad == string(b) {
			t.Fatalf("%s: mutation did not apply", name)
		}
		if _, err := Decode([]byte(bad)); err == nil {
			t.Errorf("%s: decoded without error", name)
		}
	}
}

func mustRead(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// WP4.1 accept: hash mismatch, missing criterion mapping, and sha ≠ head are
// each detected by proof verify, and a sound pack verifies.
func TestVerifyDetects(t *testing.T) {
	open := func(files map[string][]byte) Opener {
		return func(a Artifact) ([]byte, error) {
			b, ok := files[a.Path]
			if !ok {
				return nil, os.ErrNotExist
			}
			return b, nil
		}
	}
	exp := Expect{HeadSHA: head, CompletionGoalHash: cg, Criteria: []int{1, 2}}
	p, files := sample()
	if r := Verify(p, exp, open(files)); !r.OK || r.Hashed != 3 || !r.FloorsHold {
		t.Fatalf("sound pack: %+v", r)
	}
	tests := []struct {
		name string
		edit func(p *Pack, files map[string][]byte, exp *Expect)
		want string
	}{
		{"hash mismatch", func(_ *Pack, f map[string][]byte, _ *Expect) {
			f["screens/2-empty.png"] = []byte("\x89PNG other bytes!!!!!!!!!!")
		}, "screens/2-empty.png: hash mismatch"},
		{"missing criterion mapping", func(p *Pack, _ map[string][]byte, _ *Expect) { p.Criteria = p.Criteria[:1] }, "criterion 2 has no evidence in the pack (missing criterion mapping)"},
		{"criterion added after Ready", func(_ *Pack, _ map[string][]byte, e *Expect) { e.Criteria = []int{1, 2, 3} }, "criterion 3 has no evidence"},
		{"sha ≠ head", func(_ *Pack, _ map[string][]byte, e *Expect) { e.HeadSHA = other }, "(sha ≠ head)"},
		{"Completion Goal drift", func(_ *Pack, _ map[string][]byte, e *Expect) { e.CompletionGoalHash = strings.Repeat("b", 64) }, "but the Ready snapshot has bbbbbbbbbbbb"},
		{"dangling evidence", func(p *Pack, _ map[string][]byte, _ *Expect) { p.Criteria[0].Evidence = []string{"gone.json"} }, "criterion 1: evidence gone.json is not an artifact"},
	}
	for _, tt := range tests {
		p, files := sample()
		e := exp
		tt.edit(p, files, &e)
		r := Verify(p, e, open(files))
		if r.OK || !slices.ContainsFunc(r.Problems, func(s string) bool { return strings.Contains(s, tt.want) }) {
			t.Errorf("%s: problems %q, want one containing %q", tt.name, r.Problems, tt.want)
		}
	}
	// A failed floor is reported but is not an integrity problem; rule 7 keeps it from holding.
	p, files = sample()
	p.Checks = append(p.Checks, Check{Floor: "sast", Result: Fail, Value: 1, Tool: "semgrep"})
	if r := Verify(p, exp, open(files)); !r.OK || r.FloorsHold {
		t.Fatalf("failed floor: %+v", r)
	}
}

func TestParseMedia(t *testing.T) {
	for in, want := range map[string]string{
		"s3://proof-media/acme?region=eu-west-2":                      "https://s3.eu-west-2.amazonaws.com proof-media acme eu-west-2",
		"s3://b?endpoint=http://127.0.0.1:9000":                       "http://127.0.0.1:9000 b  us-east-1",
		"r2://0123abcd/aayu-proof/repo-a":                             "https://0123abcd.r2.cloudflarestorage.com aayu-proof repo-a auto",
		"s3://b/p/q?region=auto&endpoint=https://minio.example:9000/": "https://minio.example:9000 b p/q auto",
	} {
		b, err := ParseMedia(in)
		if err != nil {
			t.Fatalf("%s: %v", in, err)
		}
		if got := strings.Join([]string{b.Endpoint, b.Name, b.Prefix, b.Region}, " "); got != want {
			t.Errorf("%s: %q, want %q", in, got, want)
		}
	}
	for _, in := range []string{"", "gs://b", "s3:///nobucket", "r2://acct", "s3://b?endpoint=ftp://x"} {
		if _, err := ParseMedia(in); err == nil {
			t.Errorf("%q parsed", in)
		}
	}
}

func run(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// ledgerRemote is a clone whose origin is a local bare repository with a
// ledger branch; lfs adds the media .gitattributes init writes on a private repository.
func ledgerRemote(t *testing.T, lfs bool) (dir, bare string) {
	t.Helper()
	root := t.TempDir()
	bare = filepath.Join(root, "origin.git")
	run(t, root, "init", "--quiet", "--bare", bare)
	seed := filepath.Join(root, "seed")
	run(t, root, "clone", "--quiet", bare, seed)
	run(t, seed, "checkout", "--quiet", "--orphan", "ledger")
	_ = os.WriteFile(filepath.Join(seed, "README.md"), []byte("ledger\n"), 0o644)
	if lfs {
		var ga strings.Builder
		for _, e := range MediaExts {
			ga.WriteString("*." + e + " filter=lfs diff=lfs merge=lfs -text\n")
		}
		_ = os.WriteFile(filepath.Join(seed, ".gitattributes"), []byte(ga.String()), 0o644)
	}
	run(t, seed, "add", ".")
	run(t, seed, "commit", "--quiet", "-m", "ledger")
	run(t, seed, "push", "--quiet", "origin", "ledger")
	dir = filepath.Join(root, "work")
	run(t, root, "clone", "--quiet", bare, dir)
	return dir, bare
}

func localPack(t *testing.T) (string, *Pack) {
	t.Helper()
	p, files := sample()
	dir := filepath.Join(t.TempDir(), "pack")
	if err := WriteLocal(dir, p, files); err != nil {
		t.Fatal(err)
	}
	return dir, p
}

// WP4.2: media go to the private bucket with a retention rule; data
// artifacts and pack.json go to the ledger; the pack verifies from the
// ledger; purge deletes the bucket objects and tombstones pack.json.
func TestPushVerifyPurgeBucket(t *testing.T) {
	ctx := context.Background()
	s3 := fakes3.New(t, "sekret")
	dir, bare := ledgerRemote(t, false)
	b, err := ParseMedia("s3://proof/acme?endpoint=" + s3.Server.URL)
	if err != nil {
		t.Fatal(err)
	}
	b.AccessKey, b.SecretKey = "AKID", "sekret"
	now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	st := &Store{Repo: "o/r", Ledger: &ledger.Writer{Dir: dir, Repo: "o/r"}, Bucket: b, RetentionDays: 90, Now: func() time.Time { return now }}
	pdir, p := localPack(t)

	// Without --media, only hashes of media are kept.
	res, err := st.Push(ctx, pdir, false, Authority{})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(res.Stores, "screens/2-empty.png=none") || len(s3.Objects) != 0 || res.Authority != Advisory {
		t.Fatalf("push without media: %+v, objects %d", res, len(s3.Objects))
	}
	res, err = st.Push(ctx, pdir, true, Authority{})
	if err != nil {
		t.Fatal(err)
	}
	key := "proof/acme/proof/o/r/7/" + p.Artifacts[2].SHA256
	if p.Artifacts[2].Path != "screens/2-empty.png" || s3.Objects[key] == nil {
		t.Fatalf("bucket objects: %v (want %s)", keys(s3.Objects), key)
	}
	if lc := string(s3.Lifecycle["proof"]); !strings.Contains(lc, "<Prefix>acme/proof/o/r/</Prefix>") || !strings.Contains(lc, "<Days>90</Days>") {
		t.Fatalf("lifecycle: %s", lc)
	}
	if got := run(t, bare, "show", "ledger:proof/7/"+head+"/advisory/artifacts/coverage/diff.json"); got != `{"covered":9,"total":10}` {
		t.Fatalf("data artifact on ledger: %s", got)
	}
	if out := run(t, bare, "ls-tree", "-r", "--name-only", "ledger"); strings.Contains(out, ".png") {
		t.Fatalf("media committed to the ledger:\n%s", out)
	}
	// Re-push is idempotent: no lifecycle change, no new commit.
	again, err := st.Push(ctx, pdir, true, Authority{})
	if err != nil || again.Commit != res.Commit || again.Lifecycle {
		t.Fatalf("re-push: %+v %v (first %s)", again, err, res.Commit)
	}

	tree, err := st.Ledger.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	e, err := st.Best(ctx, tree, 7)
	if err != nil {
		t.Fatal(err)
	}
	exp := Expect{HeadSHA: head, CompletionGoalHash: cg, Criteria: []int{1, 2}}
	if r := Verify(e.Pack, exp, st.Opener(ctx, tree, e)); !r.OK || r.Hashed != 3 {
		t.Fatalf("verify from ledger and bucket: %+v", r)
	}
	// A tampered bucket object is a hash mismatch.
	s3.Mu.Lock()
	s3.Objects[key] = []byte("swapped")
	s3.Mu.Unlock()
	if r := Verify(e.Pack, exp, st.Opener(ctx, tree, e)); r.OK || !strings.Contains(strings.Join(r.Problems, ";"), "screens/2-empty.png: hash mismatch") {
		t.Fatalf("tampered object: %+v", r)
	}
	tree.Close()

	// Retention purge leaves a fresh pack alone; a story purge removes its media.
	tree, _ = st.Ledger.Read(ctx)
	if pr, err := st.Purge(ctx, tree, 0, "retention"); err != nil || len(pr.Packs) != 0 {
		t.Fatalf("retention purge of a fresh pack: %+v %v", pr, err)
	}
	pr, err := st.Purge(ctx, tree, 7, "on request")
	tree.Close()
	if err != nil || len(pr.Objects) != 1 || len(s3.Objects) != 0 {
		t.Fatalf("purge: %+v %v, objects left %v", pr, err, keys(s3.Objects))
	}
	tree, _ = st.Ledger.Read(ctx)
	defer tree.Close()
	e, _ = st.Best(ctx, tree, 7)
	a, _ := e.Pack.Artifact("screens/2-empty.png")
	if a.Tombstone == nil || a.Tombstone.Reason != "on request" || a.SHA256 != p.Artifacts[2].SHA256 {
		t.Fatalf("tombstone: %+v", a)
	}
	if r := Verify(e.Pack, exp, st.Opener(ctx, tree, e)); !r.OK || r.NotStored != 1 {
		t.Fatalf("verify after purge: %+v", r)
	}
	if s3.Rejected != 0 {
		t.Fatalf("%d requests failed signature checks", s3.Rejected)
	}
}

// V3: a public repository without a bucket never pushes media, and the
// refusal happens before anything is written.
func TestPublicRepoRefusesMedia(t *testing.T) {
	dir, bare := ledgerRemote(t, false)
	st := &Store{Repo: "o/r", Ledger: &ledger.Writer{Dir: dir, Repo: "o/r"}}
	pdir, _ := localPack(t)
	before := run(t, bare, "rev-parse", "ledger")
	if _, err := st.Push(context.Background(), pdir, true, Authority{}); !errors.Is(err, ErrMediaRefused) {
		t.Fatalf("public push --media: %v", err)
	}
	if after := run(t, bare, "rev-parse", "ledger"); after != before {
		t.Fatal("the refused push wrote the ledger")
	}
}

// WP4.2: on a private repository without a bucket, media go to Git LFS on the
// ledger; purge removes them from the tip and tombstones the pack.
func TestPrivateRepoMediaInLFS(t *testing.T) {
	if _, err := exec.LookPath("git-lfs"); err != nil {
		t.Skip("git-lfs is not installed")
	}
	ctx := context.Background()
	dir, bare := ledgerRemote(t, true)
	st := &Store{Repo: "o/r", Ledger: &ledger.Writer{Dir: dir, Repo: "o/r"}, Private: true}
	pdir, _ := localPack(t)
	res, err := st.Push(ctx, pdir, true, Authority{})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(res.Stores, "screens/2-empty.png=lfs") {
		t.Fatalf("stores: %v", res.Stores)
	}
	ptr := run(t, bare, "show", "ledger:proof/7/"+head+"/advisory/artifacts/screens/2-empty.png")
	if !strings.HasPrefix(ptr, "version https://git-lfs.github.com/spec/v1") {
		t.Fatalf("screenshot is not an LFS pointer on the ledger:\n%s", ptr)
	}
	tree, _ := st.Ledger.Read(ctx)
	e, _ := st.Best(ctx, tree, 7)
	if r := Verify(e.Pack, Expect{HeadSHA: head, CompletionGoalHash: cg, Criteria: []int{1, 2}}, st.Opener(ctx, tree, e)); !r.OK || r.Hashed != 3 {
		t.Fatalf("verify through LFS: %+v", r)
	}
	pr, err := st.Purge(ctx, tree, 7, "on request")
	tree.Close()
	if err != nil || len(pr.Files) != 1 {
		t.Fatalf("purge: %+v %v", pr, err)
	}
	if out := run(t, bare, "ls-tree", "-r", "--name-only", "ledger"); strings.Contains(out, ".png") {
		t.Fatalf("LFS file still on the ledger tip:\n%s", out)
	}
}

func keys(m map[string][]byte) []string {
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}
