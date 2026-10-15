package proof

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/integrations/fixture"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/proof/fakecloud"
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

// WP7.2b.2: OpenMedia picks the store by scheme and refuses, before any
// request, a value or credential it cannot use; with oidc or cli no secret
// is needed.
func TestOpenMedia(t *testing.T) {
	const sas = "sv=2023-11-03&sp=rwdl&sig=abc%3D"
	key := fixture.NewClouds(t, time.Now()).GCPKeyJSON()
	for _, tt := range []struct {
		in, secret, want string // want: the store's type, or text of the refusal
	}{
		{"azblob://acmeproof/proof/p?endpoint=http://127.0.0.1:10000/acmeproof", sas, "*proof.AzureBlob"},
		{"azblob://acmeproof/proof?auth=cli", "", "*proof.AzureBlob"},
		{"azblob://acmeproof/proof?auth=oidc&tenant=t1&client=5e6f7a8b-0000-4000-8000-00000000bbbb&subscription=0b1c2d3e-0000-4000-8000-000000000001&group=rg", "", "*proof.AzureBlob"},
		{"gs://acme-proof/p", key, "*proof.GCS"},
		{"gs://acme-proof?auth=oidc&provider=projects/1/locations/global/workloadIdentityPools/gh/providers/aayu&sa=media@acme.iam.gserviceaccount.com", "", "*proof.GCS"},
		{"s3://b", "AK:SK", "*proof.Bucket"},
		{"azblob://acmeproof/proof", "", `secret "proof-media" (a SAS token for the container) is not set`},
		{"azblob://acmeproof/proof", "not-a-sas", "is not a SAS token"},
		{"azblob://acmeproof/Proof_Media", sas, "container"},
		{"azblob://acmeproof/proof?auth=key", "id:secret", "needs tenant="},
		{"azblob://acmeproof/proof?auth=oidc&tenant=t1", "", "needs client="},
		{"azblob://acmeproof/proof?auth=cli&subscription=0b1c2d3e-0000-4000-8000-000000000001", "", "subscription and group go together"},
		{"gs://acme-proof", "", "a service account JSON key"},
		{"gs://acme-proof", `{"type":"authorized_user"}`, "want a service account JSON key"},
		{"gs://acme-proof?auth=oidc&sa=media@acme.iam.gserviceaccount.com", "", "needs provider="},
		{"gcs://acme-proof", "", "not a bucket"},
	} {
		m, err := OpenMedia(tt.in, MediaOptions{Secret: func() string { return tt.secret }, SecretName: "proof-media"})
		got := fmt.Sprintf("%T", m)
		if err != nil {
			got = err.Error()
		}
		if !strings.Contains(got, tt.want) {
			t.Errorf("%s: %s, want %s", tt.in, got, tt.want)
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

// backend is a media store under test with its fake.
type backend struct {
	name     string
	media    Media
	fake     func() (objects map[string][]byte, unlock func())
	rule     func() string // the fake's lifecycle configuration
	object   string        // where the screenshot lands in the fake ("<bucket>/<key>")
	ruleHas  []string      // what the fake's lifecycle must hold after the push
	keep     string        // a rule of the founder's that must survive aayudha's edits
	note     bool          // lifecycle is unmanaged (Azure SAS): the push says so instead
	rejected func() int
}

func backends(t *testing.T, now time.Time) []backend {
	sha := sample2SHA()
	s3 := fakes3.New(t, "sekret")
	b, err := ParseMedia("s3://proof/acme?endpoint=" + s3.Server.URL)
	if err != nil {
		t.Fatal(err)
	}
	b.AccessKey, b.SecretKey = "AKID", "sekret"
	out := []backend{{name: "s3", media: b,
		fake:   func() (map[string][]byte, func()) { s3.Mu.Lock(); return s3.Objects, s3.Mu.Unlock },
		rule:   func() string { s3.Mu.Lock(); defer s3.Mu.Unlock(); return string(s3.Lifecycle["proof"]) },
		object: "proof/acme/proof/o/r/7/" + sha, ruleHas: []string{"<Prefix>acme/proof/o/r/</Prefix>", "<Days>90</Days>"},
		rejected: func() int { return s3.Rejected }}}

	// Azure Blob with an Entra app registration (client secret): blobs with
	// a storage token, the lifecycle rule through Azure Resource Manager.
	clouds := fixture.NewClouds(t, now)
	az := fakecloud.NewAzure(t, "", fixture.EntraToken("https://storage.azure.com/.default"), fixture.EntraToken("https://management.azure.com/.default"))
	az.Lifecycle = []byte(`{"properties":{"policy":{"rules":[{"enabled":true,"name":"keeplogs","type":"Lifecycle","definition":{"actions":{"baseBlob":{"tierToCool":{"daysAfterModificationGreaterThan":30}}},"filters":{"blobTypes":["blockBlob"],"prefixMatch":["logs/"]}}}]}}}`)
	hc := &http.Client{Transport: clouds.Route(fakecloud.Rewrite(http.DefaultTransport, "management.azure.com", az.Server))}
	m, err := OpenMedia("azblob://acmeproof/proof/acme?auth=key&tenant="+fixture.AzureTenant+"&subscription=0b1c2d3e-0000-4000-8000-000000000001&group=rg-proof&endpoint="+az.Server.URL,
		MediaOptions{Secret: func() string { return fixture.AzureClient + ":" + fixture.AzureSecret }, SecretName: "proof-media", HTTP: hc, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	out = append(out, backend{name: "azblob entra", media: m,
		fake:   func() (map[string][]byte, func()) { az.Mu.Lock(); return az.Objects, az.Mu.Unlock },
		rule:   func() string { az.Mu.Lock(); defer az.Mu.Unlock(); return string(az.Lifecycle) },
		object: "proof/acme/proof/o/r/7/" + sha,
		// The founder's own rule is kept beside aayudha's.
		keep: `"name":"keeplogs"`, ruleHas: []string{`"name":"keeplogs"`, `"prefixMatch":["proof/acme/proof/o/r/"]`, `"daysAfterCreationGreaterThan":90`, `"name":"aayudhaproofretention`},
		rejected: func() int { return az.Rejected }})

	// Azure Blob with a SAS token: objects only; the lifecycle is reported as unmanaged.
	sas := "sv=2023-11-03&ss=b&srt=co&sp=rwdl&se=2026-12-31T00%3A00%3A00Z&sig=c2FzLXNpZ25hdHVyZQ%3D%3D"
	azs := fakecloud.NewAzure(t, sas, "", "")
	m, err = OpenMedia("azblob://acmeproof/proof?endpoint="+azs.Server.URL, MediaOptions{Secret: func() string { return "?" + sas }, SecretName: "proof-media", Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	out = append(out, backend{name: "azblob sas", media: m,
		fake:   func() (map[string][]byte, func()) { azs.Mu.Lock(); return azs.Objects, azs.Mu.Unlock },
		rule:   func() string { return "" },
		object: "proof/proof/o/r/7/" + sha, note: true, rejected: func() int { return azs.Rejected }})

	// Cloud Storage with a service account key (signed JWT grant).
	gclouds := fixture.NewClouds(t, now)
	gclouds.GoogleScopes = "https://www.googleapis.com/auth/devstorage.full_control"
	gs := fakecloud.NewGCS(t, fixture.GCPToken)
	gs.Lifecycle = []byte(`{"lifecycle":{"rule":[{"action":{"type":"SetStorageClass","storageClass":"NEARLINE"},"condition":{"age":30}}]}}`)
	m, err = OpenMedia("gs://acme-proof/acme?endpoint="+gs.Server.URL, MediaOptions{Secret: gclouds.GCPKeyJSON, SecretName: "proof-media",
		HTTP: &http.Client{Transport: gclouds.Route(http.DefaultTransport)}, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	out = append(out, backend{name: "gcs key", media: m,
		fake:   func() (map[string][]byte, func()) { gs.Mu.Lock(); return gs.Objects, gs.Mu.Unlock },
		rule:   func() string { gs.Mu.Lock(); defer gs.Mu.Unlock(); return string(gs.Lifecycle) },
		object: "acme-proof/acme/proof/o/r/7/" + sha,
		keep:   `"SetStorageClass"`, ruleHas: []string{`"SetStorageClass"`, `{"action":{"type":"Delete"},"condition":{"age":90,"matchesPrefix":["acme/proof/o/r/"]}}`},
		rejected: func() int { return gs.Rejected }})
	return out
}

func sample2SHA() string {
	p, _ := sample()
	return p.Artifacts[1].SHA256
}

// WP4.2 (S3/R2) and WP7.2b.2 (Azure Blob, Cloud Storage), opt-in since
// S-4.2b: media go to the store with a retention rule merged into its others;
// data artifacts and pack.json go to the ledger; the pack verifies from the
// ledger; a swapped object is a hash mismatch. Every request carries the
// store's credential.
func TestPushVerifyBucket(t *testing.T) {
	now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	for _, bk := range backends(t, now) {
		t.Run(bk.name, func(t *testing.T) {
			ctx := context.Background()
			dir, bare := ledgerRemote(t, false)
			st := &Store{Repo: "o/r", Ledger: &ledger.Writer{Dir: dir, Repo: "o/r"}, Bucket: bk.media, RetentionDays: 90}
			pdir, p := localPack(t)
			count := func() int { o, u := bk.fake(); defer u(); return len(o) }

			// Without --media, only hashes of media are kept.
			res, err := st.Push(ctx, pdir, false, Authority{})
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Contains(res.Stores, "screens/2-empty.png=none") || count() != 0 || res.Authority != Advisory {
				t.Fatalf("push without media: %+v, objects %d", res, count())
			}
			res, err = st.Push(ctx, pdir, true, Authority{})
			if err != nil {
				t.Fatal(err)
			}
			objs, unlock := bk.fake()
			got := objs[bk.object]
			unlock()
			if p.Artifacts[2].Path != "screens/2-empty.png" || got == nil {
				t.Fatalf("store objects: want %s", bk.object)
			}
			if bk.note != (res.LifecycleNote != "") || bk.note && !strings.Contains(res.LifecycleNote, ".aayudha/setup/azure/") {
				t.Fatalf("lifecycle note %q", res.LifecycleNote)
			}
			for _, w := range bk.ruleHas {
				if lc := bk.rule(); !strings.Contains(lc, w) {
					t.Fatalf("lifecycle lacks %s:\n%s", w, lc)
				}
			}
			if r, err := bk.media.Lifecycle(ctx, "o/r"); !bk.note && (err != nil || r == "") {
				t.Fatalf("Lifecycle: %q %v", r, err)
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
				t.Fatalf("verify from ledger and store: %+v", r)
			}
			// A tampered object is a hash mismatch.
			objs, unlock = bk.fake()
			objs[bk.object] = []byte("swapped")
			unlock()
			if r := Verify(e.Pack, exp, st.Opener(ctx, tree, e)); r.OK || !strings.Contains(strings.Join(r.Problems, ";"), "screens/2-empty.png: hash mismatch") {
				t.Fatalf("tampered object: %+v", r)
			}
			tree.Close()

			// retention_days 0 removes aayudha's rule and keeps the others.
			if !bk.note {
				if changed, err := bk.media.EnsureLifecycle(ctx, "o/r", 0); err != nil || !changed {
					t.Fatalf("remove rule: %t %v", changed, err)
				}
				if r, _ := bk.media.Lifecycle(ctx, "o/r"); r != "" || !strings.Contains(bk.rule(), bk.keep) {
					t.Fatalf("after removal: ours %q, store %s", r, bk.rule())
				}
			}
			if n := bk.rejected(); n != 0 {
				t.Fatalf("%d requests failed the credential check", n)
			}
		})
	}
}

// S-4.2b: without a bucket, media never go to the ledger or Git LFS, on a
// public or a private repository, and the refusal happens before anything is
// written.
func TestMediaNeedABucket(t *testing.T) {
	for _, private := range []bool{false, true} {
		dir, bare := ledgerRemote(t, private)
		st := &Store{Repo: "o/r", Ledger: &ledger.Writer{Dir: dir, Repo: "o/r"}, Private: private}
		pdir, _ := localPack(t)
		before := run(t, bare, "rev-parse", "ledger")
		if _, err := st.Push(context.Background(), pdir, true, Authority{}); !errors.Is(err, ErrMediaRefused) {
			t.Fatalf("private %t: push --media without a bucket: %v", private, err)
		}
		if after := run(t, bare, "rev-parse", "ledger"); after != before {
			t.Fatalf("private %t: the refused push wrote the ledger", private)
		}
	}
}

// S-4.2b keeps `proof verify` reading packs pushed before 0.2: a screenshot in
// Git LFS on a private ledger is downloaded and hashed, and a tombstoned
// artifact counts as not stored.
func TestVerifyReadsPacksFromBefore02(t *testing.T) {
	if _, err := exec.LookPath("git-lfs"); err != nil {
		t.Skip("git-lfs is not installed")
	}
	ctx := context.Background()
	dir, bare := ledgerRemote(t, true)
	st := &Store{Repo: "o/r", Ledger: &ledger.Writer{Dir: dir, Repo: "o/r"}}
	p, files := sample()
	p.Artifacts[1].Store = StoreLFS
	base := Dir(7, head, Advisory)
	c := ledger.Commit{Message: "proof: a 0.1 pack with LFS media", Files: map[string][]byte{}}
	for rel, b := range files {
		c.Files[base+"/artifacts/"+rel] = b
	}
	enc, err := Encode(p)
	if err != nil {
		t.Fatal(err)
	}
	c.Files[base+"/"+FileName] = enc
	w := *st.Ledger
	w.LFS = true
	if _, err := w.Write(ctx, c); err != nil {
		t.Fatal(err)
	}
	if ptr := run(t, bare, "show", "ledger:"+base+"/artifacts/screens/2-empty.png"); !strings.HasPrefix(ptr, "version https://git-lfs.github.com/spec/v1") {
		t.Fatalf("screenshot is not an LFS pointer on the ledger:\n%s", ptr)
	}
	tree, _ := st.Ledger.Read(ctx)
	defer tree.Close()
	e, _ := st.Best(ctx, tree, 7)
	exp := Expect{HeadSHA: head, CompletionGoalHash: cg, Criteria: []int{1, 2}}
	if r := Verify(e.Pack, exp, st.Opener(ctx, tree, e)); !r.OK || r.Hashed != 3 {
		t.Fatalf("verify through LFS: %+v", r)
	}
	e.Pack.Artifacts[1].Tombstone = &Tombstone{PurgedAt: "2027-01-10T00:00:00Z", Reason: "retention: older than 90 days"}
	if r := Verify(e.Pack, exp, st.Opener(ctx, tree, e)); !r.OK || r.Hashed != 2 || r.NotStored != 1 {
		t.Fatalf("verify a tombstoned artifact: %+v", r)
	}
}

// S-4.2b: the run artifact keeps [proof] retention_days, never longer than the
// repository allows, and "keep" (0) means as long as it allows.
func TestArtifactRetention(t *testing.T) {
	for _, c := range []struct {
		days  int
		limit string
		want  int
	}{
		{30, "90", 30}, {90, "90", 90}, {91, "90", 90}, {120, "400", 120}, {401, "400", 400},
		{0, "400", 400}, {200, "", 90}, {1, "", 1}, {45, "junk", 45},
	} {
		if got := ArtifactRetention(c.days, c.limit); got != c.want {
			t.Errorf("ArtifactRetention(%d, %q) = %d, want %d", c.days, c.limit, got, c.want)
		}
	}
}
