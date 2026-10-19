package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	osuser "os/user"
	"path/filepath"
	"runtime"
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

// WP4.2 through the CLI, as S-4.2b leaves it: media are refused until [proof]
// media names a bucket (media never go to the ledger or Git LFS); then the
// advisory pack lands (media in the bucket, the rest on the ledger), its
// advisory comment is kept on the issue and the PR, proof verify checks it
// against the Ready snapshot and the PR head, and a moved head is caught. CI
// never pushes a pack.
func TestProofPushVerifyCLI(t *testing.T) {
	fgh, dir, bare, env, opts, prN := proofFixture(t)

	// No bucket: refused, nothing written.
	before := git(t, bare, "rev-parse", "ledger")
	if code, _, errOut := run(t, env, opts, "proof", "push", "2", "--media", "--repo", "o/r"); code != ExitBlocked || !strings.Contains(errOut, "media never go to the ledger or Git LFS") {
		t.Fatalf("push --media without a bucket: %d %s", code, errOut)
	}
	if code, _, errOut := run(t, env, opts, "proof", "push", "2", "--ci", "--repo", "o/r"); code != ExitUsage || !strings.Contains(errOut, "CI writes nothing to the ledger") {
		t.Fatalf("push --ci: %d %s", code, errOut)
	}
	if git(t, bare, "rev-parse", "ledger") != before {
		t.Fatal("refused push wrote the ledger")
	}

	s3 := fakes3.New(t, "sk")
	cfg := "version = 1\n[proof]\nmedia = \"s3://proof-bucket/acme?endpoint=" + s3.Server.URL + "\"\nretention_days = 30\n"
	if err := os.WriteFile(filepath.Join(dir, ".aayudha", "config.toml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	// #667: the bucket and its secret come only from the user config and
	// dev's committed config. A person who runs proof push in a checkout of
	// an agent's branch sends nothing to the bucket that checkout names.
	evil := fakes3.New(t, "sk")
	env["AAYU_SECRET_OTHER"] = "ak:sk"
	if code, _, errOut := run(t, env, opts, "proof", "push", "2", "--media", "--repo", "o/r"); code != ExitBlocked || !strings.Contains(errOut, "the proof media bucket, its secret, and its retention are read only from") {
		t.Fatalf("push with a bucket only the checkout names: %d %s", code, errOut)
	}
	fgh.Mu.Lock()
	if fgh.Files == nil {
		fgh.Files = map[string]string{}
	}
	fgh.Files[".aayudha/config.toml"] = cfg
	fgh.Mu.Unlock()
	for _, args := range [][]string{{"--set", "proof.media=s3://evil/x?endpoint=" + evil.Server.URL, "--set", "proof.media_secret=other"}, {"--set", "proof.media_secret=other"}, {"--set", "proof.retention_days=1"}} {
		if code, _, errOut := run(t, env, opts, append([]string{"proof", "push", "2", "--media", "--repo", "o/r"}, args...)...); code != ExitBlocked || !strings.Contains(errOut, "the proof media bucket, its secret, and its retention are read only from") {
			t.Fatalf("push with %v: %d %s", args, code, errOut)
		}
	}
	// XDG_CONFIG_HOME is the run's environment: a user config it points at,
	// here in the checkout, is not the account's and names no secret.
	xdg := filepath.Join(dir, ".evil")
	if err := os.MkdirAll(filepath.Join(xdg, "aayudha"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(xdg, "aayudha", "config.toml"), []byte("version = 1\n[proof]\nmedia_secret = \"other\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	env["XDG_CONFIG_HOME"] = xdg
	if code, _, errOut := run(t, env, opts, "proof", "push", "2", "--media", "--repo", "o/r"); code != ExitBlocked || !strings.Contains(errOut, `media_secret "other"`) {
		t.Fatalf("push with a user config XDG_CONFIG_HOME points into the checkout: %d %s", code, errOut)
	}
	delete(env, "XDG_CONFIG_HOME")
	// dev's config that cannot be read, or does not load, trusts nothing.
	for _, f := range []struct{ fail, file, want string }{
		{"GET /repos/o/r/git/ref/heads/dev", cfg, "500"},
		{"GET /repos/o/r/contents/.aayudha/config.toml", cfg, "dev's .aayudha/config.toml cannot be read or does not load"},
		{"", "version = [", "dev's .aayudha/config.toml cannot be read or does not load"},
	} {
		fgh.Mu.Lock()
		fgh.Fail, fgh.FailAfter, fgh.Files[".aayudha/config.toml"] = f.fail, 0, f.file
		fgh.Mu.Unlock()
		if code, _, errOut := run(t, env, opts, "proof", "push", "2", "--media", "--repo", "o/r"); code == 0 || !strings.Contains(errOut, f.want) {
			t.Fatalf("push when %q fails with %q committed: %d %s", f.fail, f.file, code, errOut)
		}
	}
	// A directory where dev's config should be is no config either.
	fgh.Mu.Lock()
	fgh.Fail = ""
	delete(fgh.Files, ".aayudha/config.toml")
	fgh.Files[".aayudha/config.toml/x"] = ""
	fgh.Mu.Unlock()
	if code, _, errOut := run(t, env, opts, "proof", "push", "2", "--media", "--repo", "o/r"); code != ExitBlocked || !strings.Contains(errOut, "it is a directory") {
		t.Fatalf("push when dev's config is a directory: %d %s", code, errOut)
	}
	fgh.Mu.Lock()
	delete(fgh.Files, ".aayudha/config.toml/x")
	fgh.Files[".aayudha/config.toml"] = cfg
	fgh.Mu.Unlock()
	// The account's own user config that does not load trusts nothing.
	badHome := t.TempDir()
	if err := os.MkdirAll(filepath.Join(badHome, ".config", "aayudha"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(badHome, ".config", "aayudha", "config.toml"), []byte("version = ["), 0o644); err != nil {
		t.Fatal(err)
	}
	badOpts := opts
	badOpts.UserHome = func() (string, error) { return badHome, nil }
	if code, _, errOut := run(t, env, badOpts, "proof", "push", "2", "--media", "--repo", "o/r"); code != ExitUsage || !strings.Contains(errOut, "config:") {
		t.Fatalf("push when the account's user config does not load: %d %s", code, errOut)
	}
	if len(evil.Objects) != 0 || len(s3.Objects) != 0 {
		t.Fatalf("a refused push uploaded: %d to the checkout's bucket, %d to dev's", len(evil.Objects), len(s3.Objects))
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
		if len(cs) == 0 || !strings.Contains(cs[len(cs)-1].Body, AdvisoryProofMarker) || !strings.Contains(cs[len(cs)-1].Body, "Advisory") {
			t.Fatalf("proof comment on #%d: %+v", n, cs)
		}
	}
	// A repository with no dev branch: aayu init maps [branches] dev in the
	// default branch's committed config, and the branch it maps is trusted.
	// main's config names no bucket, so the push succeeds only when it
	// follows the mapping to trunk's config.
	mainSHA, trunkSHA := strings.Repeat("a", 40), strings.Repeat("b", 40)
	fgh.Mu.Lock()
	base, def := fgh.Pulls[prN].Base, fgh.Default
	fgh.Pulls[prN].Base.Ref, fgh.Default = "trunk", "main"
	fgh.Refs["heads/main"], fgh.Refs["heads/trunk"] = mainSHA, trunkSHA
	fgh.RefFiles = map[string]map[string]string{
		mainSHA:  {".aayudha/config.toml": "version = 1\n[branches]\ndev = \"trunk\"\n"},
		trunkSHA: {".aayudha/config.toml": cfg + "[branches]\ndev = \"trunk\"\n"},
	}
	fgh.Fail, fgh.FailAfter = "GET /repos/o/r", 0
	fgh.Mu.Unlock()
	if code, _, errOut := run(t, env, opts, "proof", "push", "2", "--media", "--repo", "o/r"); code == 0 || !strings.Contains(errOut, "500") {
		t.Fatalf("push when the repository's default branch cannot be read: %d %s", code, errOut)
	}
	fgh.Mu.Lock()
	fgh.Fail = ""
	fgh.Mu.Unlock()
	if code, _, errOut := run(t, env, opts, "proof", "push", "2", "--media", "--repo", "o/r"); code != 0 {
		t.Fatalf("push in a repository whose default branch maps dev: %d %s", code, errOut)
	}
	fgh.Mu.Lock()
	fgh.Pulls[prN].Base, fgh.Default = base, def
	delete(fgh.Refs, "heads/main")
	delete(fgh.Refs, "heads/trunk")
	fgh.RefFiles = nil
	fgh.Mu.Unlock()
	// Pushing again replaces the comment in place.
	if code, _, errOut := run(t, env, opts, "proof", "push", "2", "--media", "--repo", "o/r"); code != 0 {
		t.Fatalf("second push: %s", errOut)
	}
	fgh.Mu.Lock()
	n := strings.Count(commentBodies(fgh.Issues[2].Comments), AdvisoryProofMarker)
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
	if code, _, errOut := run(t, env, opts, "proof", "get", "5", "--repo", "o/r"); code != ExitBlocked || !strings.Contains(errOut, "no proof pack") {
		t.Fatalf("get without a pack: %d %s", code, errOut)
	}
}

// S-4.2b: the merge gate's aayudha/pack job. Only in the merge gate, with the
// workflow token, proof comment checks the run's pack against the PR head and
// the Ready snapshot and keeps one PR comment: each Ready criterion's verdict,
// the floors, the SHA-256 of pack.json, and the artifact link. It writes
// nothing to the ledger, leaves an advisory comment alone, and fails (exit 4)
// on a tampered artifact or a moved head and (exit 3) on a floor miss.
func TestProofCommentInMergeGate(t *testing.T) {
	fgh, dir, bare, env, opts, prN := proofFixture(t)
	if code, _, errOut := run(t, env, opts, "proof", "comment", "2", "--ci", "--repo", "o/r"); code != ExitUsage || !strings.Contains(errOut, "runs only in the merge gate") {
		t.Fatalf("proof comment outside the merge gate: %d %s", code, errOut)
	}
	if code, _, errOut := run(t, env, opts, "proof", "push", "2", "--repo", "o/r"); code != 0 { // an agent's advisory pack and comment
		t.Fatalf("push: %s", errOut)
	}
	// A pull_request run of the same file runs the PR's own copy of the gate.
	forged := map[string]string{"GITHUB_TOKEN": "workflow-token", "GITHUB_WORKFLOW_REF": "o/r/.github/workflows/aayudha-merge-gate.yml@refs/pull/8/merge", "GITHUB_EVENT_NAME": "pull_request"}
	if code, _, errOut := run(t, forged, opts, "proof", "comment", "2", "--ci", "--repo", "o/r"); code != ExitUsage || !strings.Contains(errOut, "runs only in the merge gate") {
		t.Fatalf("proof comment in a pull_request run: %d %s", code, errOut)
	}
	gate := map[string]string{"GITHUB_TOKEN": "workflow-token", "GITHUB_WORKFLOW_REF": "o/r/.github/workflows/aayudha-merge-gate.yml@refs/pull/8/merge",
		"GITHUB_EVENT_NAME": "pull_request_target", "GITHUB_RUN_ID": "4242", "AAYU_PACK_ARTIFACT_URL": "https://github.com/o/r/actions/runs/4242/artifacts/77", "AAYU_PACK_RETENTION_DAYS": "30"}
	ledgerBefore := git(t, bare, "rev-parse", "ledger")
	comment := func() (int, string, string) {
		t.Helper()
		code, out, errOut := run(t, gate, opts, "proof", "comment", "2", "--ci", "--repo", "o/r")
		fgh.Mu.Lock()
		defer fgh.Mu.Unlock()
		var body string
		for _, c := range fgh.Issues[prN].Comments {
			if strings.Contains(c.Body, ProofMarker) {
				body = c.Body
			}
		}
		return code, out + errOut, body
	}
	// The PR edits its own CI and drops a step: the risk line and the weakened-check flag show it.
	fgh.Mu.Lock()
	fgh.Pulls[prN].Diff = "diff --git a/.github/workflows/ci.yml b/.github/workflows/ci.yml\n--- a/.github/workflows/ci.yml\n+++ b/.github/workflows/ci.yml\n@@ -5,4 +5,3 @@\n       - uses: actions/checkout@v7\n-      - name: Lint\n-        run: golangci-lint run\n       - name: Test\n+        run: go test ./...\n"
	fgh.Mu.Unlock()
	_ = os.MkdirAll(filepath.Join(dir, ".github"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, ".github", "CODEOWNERS"), []byte("* @someone\n/.github/ @founder\n"), 0o644)
	code, out, body := comment()
	if !strings.Contains(body, "**Risk: high** (never-layer L09: .github/workflows/ci.yml; protected path: .github/workflows/ci.yml). Look first at `.github/workflows/ci.yml`.") ||
		!strings.Contains(body, "**Checks this PR weakens**") || !strings.Contains(body, "- `.github/workflows/ci.yml`: ") {
		t.Fatalf("risk line and weakened checks (exit %d, %s):\n%s", code, out, body)
	}
	raw, _ := os.ReadFile(filepath.Join(proof.LocalDir(dir, 2), proof.FileName))
	sum := sha256.Sum256(raw)
	for _, want := range []string{"### Proof pack: proven", "| 1. ", "| 1. cart is empty | evidence from this run | `screens/1.png` |", "| coverage | pass | 95 | 90 |",
		"`pack.json` SHA-256: `" + hex.EncodeToString(sum[:]) + "`", "[aayudha-pack artifact](https://github.com/o/r/actions/runs/4242/artifacts/77) (kept until ",
		"[merge gate run](https://github.com/o/r/actions/runs/4242)"} {
		if code != 0 || !strings.Contains(body, want) {
			t.Fatalf("proof comment (exit %d, %s) lacks %q:\n%s", code, out, want, body)
		}
	}
	fgh.Mu.Lock()
	all := commentBodies(fgh.Issues[prN].Comments)
	fgh.Mu.Unlock()
	if strings.Count(all, ProofMarker) != 1 || strings.Count(all, AdvisoryProofMarker) != 1 {
		t.Fatalf("want one gate comment and the advisory one:\n%s", all)
	}
	if git(t, bare, "rev-parse", "ledger") != ledgerBefore {
		t.Fatal("proof comment wrote the ledger")
	}

	// A floor miss fails the check (exit 3) and says so in the same comment.
	p, _ := proof.LoadLocal(proof.LocalDir(dir, 2))
	p.Checks[0].Result, p.Checks[0].Value = proof.Fail, 80
	_ = os.WriteFile(filepath.Join(proof.LocalDir(dir, 2), proof.FileName), must(proof.Encode(p)), 0o644)
	if code, out, body := comment(); code != ExitFloor || !strings.Contains(body, "### Proof pack: floors do not hold") || !strings.Contains(body, "| coverage | fail | 80 | 90 |") {
		t.Fatalf("floor miss: %d %s\n%s", code, out, body)
	}
	// Evidence changed after the gate hashed it.
	p.Checks[0].Result, p.Checks[0].Value = proof.Pass, 95
	_ = os.WriteFile(filepath.Join(proof.LocalDir(dir, 2), proof.FileName), must(proof.Encode(p)), 0o644)
	_ = os.WriteFile(filepath.Join(proof.LocalDir(dir, 2), "artifacts", filepath.FromSlash(p.Artifacts[0].Path)), []byte("swapped"), 0o644)
	if code, out, body := comment(); code != ExitBlocked || !strings.Contains(body, "### Proof pack: does not verify") || !strings.Contains(body, p.Artifacts[0].Path+": hash mismatch") {
		t.Fatalf("tampered artifact: %d %s\n%s", code, out, body)
	}
	// The PR moved on while the job ran.
	fgh.Mu.Lock()
	fgh.Pulls[prN].Head.SHA = strings.Repeat("9", 40)
	fgh.Mu.Unlock()
	if code, out, body := comment(); code != ExitBlocked || !strings.Contains(body, "sha ≠ head") {
		t.Fatalf("moved head: %d %s\n%s", code, out, body)
	}
}

func must(b []byte, err error) []byte {
	if err != nil {
		panic(err)
	}
	return b
}

// #667: the trusted user config is the account's, so the run's HOME, which an
// agent's environment can set, does not move it.
func TestAccountHomeIgnoresHOME(t *testing.T) {
	t.Setenv("HOME", filepath.Join(t.TempDir(), "agent-home"))
	if h, err := accountHome(); err != nil || h == "" || h == os.Getenv("HOME") {
		t.Fatalf("accountHome() = %q, %v; HOME is %q", h, err, os.Getenv("HOME"))
	}
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return
	}
	// A static build falls back to $HOME for a UID /etc/passwd does not
	// list (an unmapped container UID): that home is never trusted.
	u, err := osuser.Current()
	if err != nil {
		t.Fatal(err)
	}
	old := passwdFile
	t.Cleanup(func() { passwdFile = old })
	passwdFile = filepath.Join(t.TempDir(), "passwd")
	for _, c := range []struct {
		db string
		ok bool
	}{
		{"root:x:0:0:root:/root:/bin/sh\n", u.Uid == "0" && u.HomeDir == "/root"},
		{"agent:x:" + u.Uid + ":0::/tmp/elsewhere:/bin/sh\n", false},
		{"root:x:0:0:root:/root:/bin/sh\nme:x:" + u.Uid + ":0::" + u.HomeDir + ":/bin/sh\n", true},
	} {
		if err := os.WriteFile(passwdFile, []byte(c.db), 0o644); err != nil {
			t.Fatal(err)
		}
		if h, err := accountHome(); (err == nil) != c.ok || c.ok && h != u.HomeDir {
			t.Errorf("accountHome() with %q = %q, %v; want ok %t", c.db, h, err, c.ok)
		}
	}
}
