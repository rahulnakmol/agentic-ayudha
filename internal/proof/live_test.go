package proof

import (
	"context"
	"os"
	"regexp"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/integrations/cloud"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
)

// WP4.2 and WP7.2b.2 (live, bucket; opt-in since S-4.2b): push media to a
// real store, set its retention rule, and verify from it. Needs AAYU_LIVE_BUCKET (a [proof]
// media value: s3://, r2://, azblob://, or gs://) and, unless its auth is
// oidc or cli, AAYU_LIVE_BUCKET_KEYS (the [proof] media_secret value:
// ACCESS_KEY_ID:SECRET_ACCESS_KEY, a SAS token, <client-id>:<client-secret>,
// or a service account JSON key); the ledger is a local bare repository,
// since the store is what is live.
func TestLiveBucket(t *testing.T) {
	if os.Getenv("AAYU_LIVE") != "1" || os.Getenv("AAYU_LIVE_BUCKET") == "" {
		t.Skip("set AAYU_LIVE=1, AAYU_LIVE_BUCKET, and AAYU_LIVE_BUCKET_KEYS to run against a bucket")
	}
	ctx := context.Background()
	b, err := OpenMedia(os.Getenv("AAYU_LIVE_BUCKET"), MediaOptions{Secret: func() string { return os.Getenv("AAYU_LIVE_BUCKET_KEYS") },
		SecretName: "AAYU_LIVE_BUCKET_KEYS", Env: cloud.Env{Getenv: os.Getenv}})
	if err != nil {
		t.Fatal(err)
	}
	dir, _ := ledgerRemote(t, false)
	repo := "aayu-live/bucket-test"
	st := &Store{Repo: repo, Ledger: &ledger.Writer{Dir: dir, Repo: repo}, Bucket: b, RetentionDays: 1}
	pdir, p := localPack(t)
	res, err := st.Push(ctx, pdir, true, Authority{})
	if err != nil {
		t.Fatal(err)
	}
	key := b.Key(repo, 7, p.Artifacts[2].SHA256)
	if got, err := b.Get(ctx, key); err != nil || string(got) != "\x89PNG fake screenshot bytes" {
		t.Fatalf("object %s: %q %v", key, got, err)
	}
	if res.LifecycleNote != "" {
		t.Logf("lifecycle: %s", res.LifecycleNote)
	} else if rule, err := b.Lifecycle(ctx, repo); err != nil || !regexp.MustCompile(`<Days>1</Days>|"age":1\b|"daysAfterCreationGreaterThan":1\b`).MatchString(normalizeXML(rule)) {
		t.Fatalf("lifecycle rule: %q %v", rule, err)
	}
	tree, _ := st.Ledger.Read(ctx)
	defer tree.Close()
	e, _ := st.Best(ctx, tree, 7)
	if r := Verify(e.Pack, Expect{HeadSHA: head}, st.Opener(ctx, tree, e)); !r.OK || r.Hashed != 3 {
		t.Fatalf("verify from the bucket: %+v", r)
	}
	t.Cleanup(func() { _, _ = b.EnsureLifecycle(context.Background(), repo, 0) })
}
