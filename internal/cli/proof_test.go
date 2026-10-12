package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
	"github.com/rahulnakmol/agentic-ayudha/internal/proof/fakes3"
	"github.com/rahulnakmol/agentic-ayudha/internal/ready"
)

const headA = "0123456789abcdef0123456789abcdef01234567"

// proofFixture: story #2 made Ready (snapshot on the ledger), PR #8 for it at
// headA, and a local pack mapping every Ready criterion.
func proofFixture(t *testing.T) (gh *fake.GitHub, dir, bare string, env map[string]string, opts Options, prN int) {
	t.Helper()
	gh = readyTree(t)
	dir, bare = ledgerCheckout(t)
	env = map[string]string{"AAYU_GITHUB_TOKEN": "human-token"}
	opts = Options{GitHubAPI: gh.URL(), GitHubGap: -1, Dir: dir}
	code, out, errOut := run(t, env, opts, "ready", "2", "--repo", "o/r", "--json")
	if code != 0 {
		t.Fatalf("ready 2: %d %s", code, errOut)
	}
	var res ready.Result
	_ = json.Unmarshal([]byte(out), &res)
	pr := gh.AddPR("Show empty list", "Refs #2", "founder", "story-2", headA, "dev")
	files := map[string][]byte{}
	p := &proof.Pack{Repo: "o/r", Story: 2, PR: pr.Number, PRHeadSHA: headA, Seat: "ai-native-engineer", Host: "claude", Identity: "human-fallback",
		CompletionGoalHash: res.CompletionGoalHash, CreatedAt: "2026-10-12T10:00:00Z",
		Checks: []proof.Check{{Floor: "coverage", Result: proof.Pass, Value: 95, Threshold: 90, Tool: "aayudha", Version: "dev"}}}
	for _, c := range res.Criteria {
		rel := "evidence/" + c.Evidence + "-" + string(rune('0'+c.N)) + ".json"
		if c.Evidence == "screenshot" {
			rel = "screens/" + string(rune('0'+c.N)) + ".png"
		}
		files[rel] = []byte("evidence for criterion " + c.State)
		p.Artifacts = append(p.Artifacts, proof.NewArtifact(rel, c.Evidence, files[rel]))
		p.Criteria = append(p.Criteria, proof.Criterion{ID: c.N, Evidence: []string{rel}})
	}
	if err := proof.WriteLocal(proof.LocalDir(dir, 2), p, files); err != nil {
		t.Fatal(err)
	}
	return gh, dir, bare, env, opts, pr.Number
}

func commentBodies(cs []*fake.Comment) string {
	var b strings.Builder
	for _, c := range cs {
		b.WriteString(c.Body + "\n")
	}
	return b.String()
}

// WP4.2 through the CLI: a public repository refuses media until [proof]
// media names a bucket; then the pack lands (media in the bucket, the rest on
// the ledger), the proof comment is posted on the issue and the PR, proof
// verify checks it against the Ready snapshot and the PR head, a moved head is
// caught, and purge tombstones the media.
func TestProofPushVerifyPurgeCLI(t *testing.T) {
	fgh, dir, bare, env, opts, prN := proofFixture(t)

	// Public (the API says so), no bucket: refused, nothing written.
	before := git(t, bare, "rev-parse", "ledger")
	if code, _, errOut := run(t, env, opts, "proof", "push", "2", "--media", "--repo", "o/r"); code != ExitBlocked || !strings.Contains(errOut, "V3") {
		t.Fatalf("push --media on a public repo: %d %s", code, errOut)
	}
	if git(t, bare, "rev-parse", "ledger") != before {
		t.Fatal("refused push wrote the ledger")
	}

	s3 := fakes3.New(t, "sk")
	cfg := "version = 1\n[proof]\nmedia = \"s3://proof-bucket/acme?endpoint=" + s3.Server.URL + "\"\nretention_days = 30\n"
	if err := os.WriteFile(filepath.Join(dir, ".aayudha", "config.toml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _, errOut := run(t, env, opts, "proof", "push", "2", "--media", "--repo", "o/r"); code != ExitBlocked || !strings.Contains(errOut, "aayu auth set-secret proof-media") {
		t.Fatalf("push without bucket keys: %d %s", code, errOut)
	}
	env["AAYU_SECRET_PROOF_MEDIA"] = "ak:sk"
	code, out, errOut := run(t, env, opts, "proof", "push", "2", "--media", "--repo", "o/r", "--json")
	if code != 0 {
		t.Fatalf("push: %d %s %s", code, out, errOut)
	}
	var pushed proof.PushResult
	_ = json.Unmarshal([]byte(out), &pushed)
	if pushed.Authority != proof.Advisory || pushed.Path != "proof/2/"+headA+"/advisory/pack.json" || len(s3.Objects) != 1 || s3.Rejected != 0 {
		t.Fatalf("push result %+v, %d objects, %d rejected", pushed, len(s3.Objects), s3.Rejected)
	}
	for _, n := range []int{2, prN} {
		fgh.Mu.Lock()
		cs := fgh.Issues[n].Comments
		fgh.Mu.Unlock()
		if len(cs) == 0 || !strings.Contains(cs[len(cs)-1].Body, ProofMarker) || !strings.Contains(cs[len(cs)-1].Body, "Advisory") {
			t.Fatalf("proof comment on #%d: %+v", n, cs)
		}
	}
	// Pushing again replaces the comment in place.
	if code, _, errOut := run(t, env, opts, "proof", "push", "2", "--media", "--repo", "o/r"); code != 0 {
		t.Fatalf("second push: %s", errOut)
	}
	fgh.Mu.Lock()
	n := strings.Count(commentBodies(fgh.Issues[2].Comments), ProofMarker)
	fgh.Mu.Unlock()
	if n != 1 {
		t.Fatalf("%d proof comments on #2 after two pushes", n)
	}

	if code, out, errOut := run(t, env, opts, "proof", "verify", "2", "--repo", "o/r"); code != 0 || !strings.Contains(out, "floors hold: true") {
		t.Fatalf("verify: %d %s %s", code, out, errOut)
	}
	if code, out, _ := run(t, env, opts, "proof", "list", "--repo", "o/r"); code != 0 || !strings.Contains(out, "#2  0123456") {
		t.Fatalf("list: %d %s", code, out)
	}
	// The PR moves on: the pack no longer proves its head.
	fgh.Mu.Lock()
	fgh.Pulls[prN].Head.SHA = strings.Repeat("9", 40)
	fgh.Mu.Unlock()
	if code, out, _ := run(t, env, opts, "proof", "verify", "2", "--repo", "o/r"); code != ExitBlocked || !strings.Contains(out, "sha ≠ head") {
		t.Fatalf("verify after the head moved: %d %s", code, out)
	}

	code, out, errOut = run(t, env, opts, "proof", "purge", "2", "--repo", "o/r", "--json")
	if code != 0 || len(s3.Objects) != 0 {
		t.Fatalf("purge: %d %s %s, objects %d", code, out, errOut, len(s3.Objects))
	}
	code, out, _ = run(t, env, opts, "proof", "get", "2", "--repo", "o/r", "--json")
	if code != 0 || !strings.Contains(out, `"reason": "purged on request (aayu proof purge #2)"`) {
		t.Fatalf("get after purge: %d %s", code, out)
	}
	if code, _, errOut := run(t, env, opts, "proof", "get", "5", "--repo", "o/r"); code != ExitBlocked || !strings.Contains(errOut, "no proof pack") {
		t.Fatalf("get without a pack: %d %s", code, errOut)
	}
}
