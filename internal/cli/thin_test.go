package cli

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
	"github.com/rahulnakmol/agentic-ayudha/internal/inspect"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/planning"
	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
	"github.com/rahulnakmol/agentic-ayudha/internal/ready"
)

// thinFixture: story #2 Ready, a checkout whose origin is a local bare
// repository with dev and ledger, and `build 2` run to the PR.
func thinFixture(t *testing.T) (g *fake.GitHub, env map[string]string, opts Options, dir, bare string, prN int, head string) {
	t.Helper()
	g = readyTree(t)
	dir, bare = ledgerCheckout(t)
	git(t, dir, "config", "user.name", "Builder")
	git(t, dir, "config", "user.email", "builder@example.com")
	env = map[string]string{"AAYU_GITHUB_TOKEN": "human-token", "PATH": os.Getenv("PATH"), "AAYU_MODEL": "claude-opus-5-5"}
	opts = Options{IsTerminal: stdinTTY, GitHubAPI: g.URL(), GitHubGap: -1, Dir: dir, CheckpointVerify: holds}
	if code, _, errOut := run(t, env, opts, "ready", "2", "--repo", "o/r"); code != 0 {
		t.Fatalf("ready: %s", errOut)
	}
	code, out, errOut := run(t, env, opts, "build", "2", "--repo", "o/r", "--json")
	var res BuildResult
	_ = json.NewDecoder(strings.NewReader(out)).Decode(&res)
	if code != 0 || !res.Created || res.Branch != "story/2-show-empty-list" || res.Worktree != BuildWorktree(dir, 2) {
		t.Fatalf("build (worktree): %d %+v %s", code, res, errOut)
	}
	if code, _, errOut := run(t, env, opts, "build", "2", "--repo", "o/r"); code != ExitBlocked || !strings.Contains(errOut, "nothing to commit yet") {
		t.Fatalf("build with nothing written: %d %s", code, errOut)
	}
	_ = os.WriteFile(filepath.Join(res.Worktree, "empty.txt"), []byte("Your cart is empty\n"), 0o644)
	code, out, errOut = run(t, env, opts, "build", "2", "--repo", "o/r", "--json")
	res = BuildResult{}
	_ = json.NewDecoder(strings.NewReader(out)).Decode(&res)
	if code != 0 || res.PR == 0 || res.Checkpoint != "1/1" {
		t.Fatalf("build (checkpoint): %d %+v %s", code, res, errOut)
	}
	msg := git(t, res.Worktree, "log", "-1", "--format=%B")
	for _, want := range []string{"Aayudha-Story: #2", "Aayudha-Checkpoint: 1/1", "Aayudha-Model: claude-opus-5-5", "Signed-off-by: Builder <builder@example.com>"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("checkpoint commit lacks %q:\n%s", want, msg)
		}
	}
	head = git(t, res.Worktree, "rev-parse", "HEAD")
	if pushed := git(t, bare, "rev-parse", "refs/heads/"+res.Branch); pushed != head {
		t.Fatalf("branch not pushed: %s vs %s", pushed, head)
	}
	g.Mu.Lock()
	pr := g.Pulls[res.PR]
	pr.Head.SHA = head
	g.Mu.Unlock()
	if !strings.Contains(pr.Body, "Refs #2") || pr.Base.Ref != "dev" {
		t.Fatalf("PR: %+v", pr)
	}
	// The push asked for a review of the head (a human one: no reviewer app is configured).
	g.Mu.Lock()
	asked := slices.ContainsFunc(g.Issues[res.PR].Comments, func(c *fake.Comment) bool { return strings.Contains(c.Body, inspect.RequestMarker) })
	g.Mu.Unlock()
	if !asked || !slices.ContainsFunc(res.Notes, func(n string) bool { return strings.Contains(n, "requested") }) {
		t.Fatalf("build did not ask for a review after the push: notes %v", res.Notes)
	}
	return g, env, opts, dir, bare, res.PR, head
}

// holds stands in for build's verify of a checkpoint: the floors hold.
func holds(context.Context, string) (bool, string, error) { return true, "floors hold", nil }

// gateRun records a green merge gate for head, with its pack as the run's
// aayudha-pack artifact (no bot App: CI could not write the ledger).
func gateRun(t *testing.T, g *fake.GitHub, dir string, prN int, head string, checks []proof.Check, footprint ...string) {
	t.Helper()
	b := git(t, dir, "show", "origin/ledger:"+ready.LedgerPath(2))
	snap, _ := ready.ParseSnapshot(b)
	files := map[string][]byte{}
	p := &proof.Pack{Repo: "o/r", Story: 2, PR: prN, PRHeadSHA: head, Seat: "ai-native-engineer", Host: "cli", Identity: "ci",
		CompletionGoalHash: snap.CompletionGoalHash, CreatedAt: "2026-10-12T10:00:00Z", Checks: checks, Footprint: footprint}
	for _, c := range snap.Criteria {
		rel := "evidence/criterion-" + strconv.Itoa(c.N) + ".jsonl"
		files[rel] = []byte(`{"Action":"pass","Test":"TestEmpty"}` + "\n")
		p.Artifacts = append(p.Artifacts, proof.NewArtifact(rel, "api-trace", files[rel]))
		p.Criteria = append(p.Criteria, proof.Criterion{ID: c.N, Evidence: []string{rel}})
	}
	// A capture: in the artifact and hash-checked by ship, never landed on the ledger.
	files["media/criterion-1.png"] = []byte("\x89PNG capture")
	p.Artifacts = append(p.Artifacts, proof.NewArtifact("media/criterion-1.png", "screenshot", files["media/criterion-1.png"]))
	p.Criteria[0].Evidence = append(p.Criteria[0].Evidence, "media/criterion-1.png")
	tmp := t.TempDir()
	if err := proof.WriteLocal(filepath.Join(tmp, "2"), p, files); err != nil {
		t.Fatal(err)
	}
	var zb bytes.Buffer
	zw := zip.NewWriter(&zb)
	_ = filepath.Walk(tmp, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(tmp, path)
		w, _ := zw.Create(filepath.ToSlash(rel))
		b, _ := os.ReadFile(path)
		_, _ = w.Write(b)
		return nil
	})
	_ = zw.Close()
	g.Mu.Lock()
	defer g.Mu.Unlock()
	id := int64(9000 + len(g.Runs))
	g.Runs = append(g.Runs, fake.WorkflowRun{ID: id, Path: ".github/workflows/aayudha-merge-gate.yml", HeadSHA: head, Event: "pull_request_target", Status: "completed", Conclusion: "success"})
	g.Artifacts[id] = []fake.ArtifactZip{{ID: id + 1, Name: "aayudha-pack", Zip: zb.Bytes()}}
	g.CheckRuns[head] = []fake.CheckRun{{Name: "aayudha/merge-gate", Status: "completed", Conclusion: "success"}, {Name: "aayudha/inspect", Status: "completed", Conclusion: "success"}, {Name: "aayudha/pack", Status: "completed", Conclusion: "success"}}
}

func review(login, typ, state, commit string) fake.Review {
	return fake.Review{ID: int64(len(login) + len(state)), User: fake.Account{Login: login, Type: typ}, State: state, CommitID: commit}
}

// WP4.10 accept: build refuses a story that is not Ready; ship with only a
// bot review (or the author's own) exits 4; one human approval on the head
// ships it: merged, status:merged, Complete section, ship record and actions
// segment on the ledger, branch and worktree cleaned up. The pack comes from
// the merge gate's run artifact (S-4.2b); a refused ship lands nothing.
func TestBuildAndShip(t *testing.T) {
	g, env, opts, dir, bare, prN, head := thinFixture(t)
	if code, _, errOut := run(t, env, opts, "build", "3", "--repo", "o/r"); code != ExitBlocked || !strings.Contains(errOut, "does not pass Ready") {
		t.Fatalf("build on an XL story: %d %s", code, errOut)
	}
	if code, _, errOut := run(t, env, opts, "build", "6", "--repo", "o/r"); code != ExitBlocked {
		t.Fatalf("build on a story without a parent: %d %s", code, errOut)
	}

	ship := func(args ...string) (int, string) {
		code, out, errOut := run(t, env, opts, append([]string{"ship", strconv.Itoa(prN), "--repo", "o/r"}, args...)...)
		return code, out + errOut
	}
	if code, out := ship(); code != ExitBlocked || !strings.Contains(out, "no merge gate pack for the head "+short(head)) {
		t.Fatalf("ship before the gate: %d %s", code, out)
	}
	gateRun(t, g, dir, prN, head, []proof.Check{{Floor: "coverage", Result: proof.Pass, Value: 100, Threshold: 90, Tool: "aayudha", Version: "1"}})
	g.Mu.Lock()
	g.Pulls[prN].Reviews = []fake.Review{review("aayudha-bot[bot]", "Bot", "APPROVED", head)}
	g.Mu.Unlock()
	if code, out := ship(); code != ExitBlocked || !strings.Contains(out, "from a bot identity and is not a human pass") {
		t.Fatalf("ship with only a bot review: %d %s", code, out)
	}
	if got := git(t, bare, "ls-tree", "-r", "--name-only", "ledger", "proof/"); got != "" {
		t.Fatalf("a refused ship landed a pack:\n%s", got)
	}
	// Every worktree of the clone shares one actions log: the build worktree sees the PR it opened.
	wtOpts := opts
	wtOpts.Dir = BuildWorktree(dir, 2)
	if code, out, errOut := run(t, env, wtOpts, "actions", "grep", "github.pull.create"); code != 0 || !strings.Contains(out, "github.pull.create") {
		t.Fatalf("actions log from the build worktree: %d %s %s", code, out, errOut)
	}
	g.Mu.Lock()
	g.Pulls[prN].Reviews = append(g.Pulls[prN].Reviews, review("founder", "User", "APPROVED", head))
	g.Mu.Unlock()
	if code, out := ship(); code != ExitBlocked || !strings.Contains(out, "is the PR author's own") {
		t.Fatalf("ship with the author's own approval: %d %s", code, out)
	}
	g.Mu.Lock()
	g.Pulls[prN].Reviews = append(g.Pulls[prN].Reviews, review("reviewer", "User", "APPROVED", head))
	g.Mu.Unlock()
	code, out := ship()
	if code != 0 || !strings.Contains(out, "human pass: review by reviewer") {
		t.Fatalf("ship: %d %s", code, out)
	}
	g.Mu.Lock()
	pr, story := g.Pulls[prN], g.Issues[2]
	g.Mu.Unlock()
	labels := g.LabelNames(2)
	if !pr.Merged || !contains(labels, "status:merged") || contains(labels, "status:ready") {
		t.Fatalf("after ship: merged %t labels %v", pr.Merged, labels)
	}
	if !strings.Contains(story.Body, "### Outcome") || !strings.Contains(story.Body, "Merged in #"+strconv.Itoa(prN)) || !strings.Contains(story.Body, "approving review") {
		t.Fatalf("Complete section:\n%s", story.Body)
	}
	rec := git(t, bare, "show", "ledger:proof/2/"+head+"/authoritative/ship.json")
	var sr ShipRecord
	_ = json.Unmarshal([]byte(rec), &sr)
	if sr.MergeCommit != pr.MergeCommitSHA || sr.HumanPass.Kind != "review" || sr.HumanPass.Login != "reviewer" || sr.GateRun != 9000 || sr.Artifact != 9001 {
		t.Fatalf("ship record: %s", rec)
	}
	// The landed pack is the artifact's pack.json, with the run that built it, under the human's identity.
	g.Mu.Lock()
	zb := g.Artifacts[9000][0].Zip
	g.Mu.Unlock()
	zr, _ := zip.NewReader(bytes.NewReader(zb), int64(len(zb)))
	f, _ := zr.Open("2/pack.json")
	raw, _ := io.ReadAll(f)
	if sum := sha256.Sum256(raw); sr.PackSHA256 != hex.EncodeToString(sum[:]) {
		t.Fatalf("ship record pack_sha256 %s, artifact's pack.json %x", sr.PackSHA256, sum)
	}
	if got := git(t, bare, "show", "ledger:proof/2/"+head+"/authoritative/pack.json"); !strings.Contains(got, `"run_id": 9000`) || !strings.Contains(got, `"writer": "founder (aayu ship)"`) {
		t.Fatalf("landed pack: %s", got)
	}
	if got := git(t, bare, "ls-tree", "-r", "--name-only", "ledger", "proof/2/"); strings.Contains(got, ".png") || !strings.Contains(got, "evidence/criterion-1.jsonl") {
		t.Fatalf("ledger holds media or lacks data artifacts:\n%s", got)
	}
	if seg := git(t, bare, "show", "ledger:"+sr.Actions); !strings.Contains(seg, "github.pull.merge") {
		t.Fatalf("actions segment %s:\n%s", sr.Actions, seg)
	}
	if _, err := os.Stat(BuildWorktree(dir, 2)); !os.IsNotExist(err) {
		t.Fatalf("worktree left behind: %v", err)
	}
	// status (WP2.8, now with M4): the merged story is covered by its authoritative pack.
	code, out, errOut := run(t, env, opts, "status", "1", "--repo", "o/r", "--json")
	if code != 0 || !strings.Contains(out, `"proof_coverage": "1/1"`) {
		t.Fatalf("status: %d %s %s", code, out, errOut)
	}
}

// WP4.9 + Appendix B D2: a pack with "skipped: mobile" makes ship exit 4,
// and a waived one ships with the waiver recorded; with no second human,
// --solo-bypass ships only after the merging human's "Solo-mode bypass:"
// comment, and says so in the Complete section.
func TestShipMobileWaiverAndSoloBypass(t *testing.T) {
	g, env, opts, dir, bare, prN, head := thinFixture(t)
	gateRun(t, g, dir, prN, head, []proof.Check{{Floor: "mobile", Result: proof.Skipped, Tool: "aayudha", Version: "0.1", Detail: proof.SkippedMobile}})
	ship := func(args ...string) (int, string) {
		code, out, errOut := run(t, env, opts, append([]string{"ship", strconv.Itoa(prN), "--repo", "o/r"}, args...)...)
		return code, out + errOut
	}
	if code, out := ship("--solo-bypass"); code != ExitBlocked || !strings.Contains(out, "floors do not hold") || !strings.Contains(out, "mobile skipped") {
		t.Fatalf("ship with skipped: mobile: %d %s", code, out)
	}
	// The gate re-ran on a new head after a waiver Decision was filed under the story.
	git(t, BuildWorktree(dir, 2), "commit", "--quiet", "--allow-empty", "-m", "chore: re-run the gate")
	head = git(t, BuildWorktree(dir, 2), "rev-parse", "HEAD")
	g.Mu.Lock()
	g.Pulls[prN].Head.SHA = head
	g.Mu.Unlock()
	gateRun(t, g, dir, prN, head, []proof.Check{{Floor: "mobile", Result: proof.Waived, Tool: "aayudha", Version: "0.1", Detail: proof.SkippedMobile + " (waived by #9)", Waiver: "#9"}}, "L07")
	// L07 is a never layer: the plan shipped with a solo-mode plan-review bypass, not an approval.
	plan := PlanRecord{Story: 2, Footprint: []string{"L07"}, Review: planning.Review{Required: true, Layers: []string{"L07"},
		Bypass: &planning.ReviewBypass{As: "solo-bypass", Reviewer: "none (solo mode)", RecordedBy: "founder", Comment: "https://example.test/o/r/issues/2#issuecomment-81", Note: "solo mode"}}}
	pb, _ := json.Marshal(plan)
	if _, err := (&ledger.Writer{Dir: dir}).Write(context.Background(), ledger.Commit{Message: "plan bypass", Files: map[string][]byte{PlanLedgerPath(2): pb}}); err != nil {
		t.Fatal(err)
	}
	if code, out := ship("--solo-bypass"); code != ExitBlocked || !strings.Contains(out, `post a PR comment starting "Solo-mode bypass:"`) {
		t.Fatalf("solo bypass without the comment: %d %s", code, out)
	}
	if code, _, errOut := run(t, map[string]string{"AAYU_GITHUB_TOKEN": "human-token", "AAYU_AGENT": "1"}, opts, "ship", strconv.Itoa(prN), "--solo-bypass", "--repo", "o/r"); code != ExitBlocked || !strings.Contains(errOut, "refused for an agent lane") {
		t.Fatalf("solo bypass by an agent: %d %s", code, errOut)
	}
	g.Mu.Lock()
	bypass := &fake.Comment{ID: 77, Body: "Solo-mode bypass: one human, no bot App; shipping the thin-slice story."}
	bypass.User.Login, bypass.User.Type = "founder", "User"
	g.Issues[prN].Comments = append(g.Issues[prN].Comments, bypass)
	g.Mu.Unlock()
	// Bug #170: with every solo-mode condition met, an agent running ship --solo-bypass through its host's shell
	// with the founder's token (no AAYU_AGENT) never completes it, even on a terminal: it waits for a person's
	// device code (Story #481), and when none comes nothing is merged; with no terminal and no marker, the same.
	g.Mu.Lock()
	g.Device = &fake.DeviceFlow{ClientID: DefaultDeviceClientID, Approver: "founder", Outcome: "deny"}
	g.Mu.Unlock()
	agentOpts := opts
	agentOpts.DevicePoll = time.Millisecond
	for host, marker := range hostShells {
		agentEnv := maps.Clone(env)
		maps.Copy(agentEnv, marker)
		if code, _, errOut := run(t, agentEnv, agentOpts, "ship", strconv.Itoa(prN), "--solo-bypass", "--repo", "o/r"); code != ExitBlocked || !strings.Contains(errOut, "was not approved: the request was denied") {
			t.Fatalf("solo bypass from a %s shell: %d %s", host, code, errOut)
		}
	}
	noTTY := agentOpts
	noTTY.IsTerminal = nil
	if code, _, errOut := run(t, env, noTTY, "ship", strconv.Itoa(prN), "--solo-bypass", "--repo", "o/r"); code != ExitBlocked || !strings.Contains(errOut, "was not approved") {
		t.Fatalf("solo bypass with no terminal and no marker: %d %s", code, errOut)
	}
	g.Mu.Lock()
	merged := g.Pulls[prN].Merged
	g.Mu.Unlock()
	if merged {
		t.Fatal("a refused solo bypass merged the PR")
	}
	// A person in a terminal outside any host: accepted as before.
	code, out := ship("--solo-bypass")
	if code != 0 || !strings.Contains(out, "human pass: solo-bypass by founder") {
		t.Fatalf("solo bypass: %d %s", code, out)
	}
	g.Mu.Lock()
	body := g.Issues[2].Body
	g.Mu.Unlock()
	if !strings.Contains(body, "merged under a recorded solo-mode bypass") || !strings.Contains(body, "Floor waived: mobile by #9") ||
		!strings.Contains(body, "Plan-time review: none; founder recorded a solo-mode plan-review bypass for layers L07") {
		t.Fatalf("Complete section:\n%s", body)
	}
	rec := git(t, bare, "show", "ledger:proof/2/"+head+"/authoritative/ship.json")
	if !strings.Contains(rec, `"kind": "solo-bypass"`) || !strings.Contains(rec, `"mobile by #9"`) || !strings.Contains(rec, `"reviewer": "none (solo mode)"`) || strings.Contains(rec, `"approved_by"`) {
		t.Fatalf("ship record: %s", rec)
	}
}

// WP4.10 grill story: the draft is validated at Create and Ready, and
// written through the actions log only with --create.
func TestGrillStory(t *testing.T) {
	g := readyTree(t)
	env := map[string]string{"AAYU_GITHUB_TOKEN": "human-token"}
	opts := Options{GitHubAPI: g.URL(), GitHubGap: -1}
	body := fixtureBody(t, "story", nil)
	grill := func(args ...string) (int, GrillResult, string) {
		code, out, errOut := run(t, env, opts, append([]string{"grill", "story", "--repo", "o/r", "--json"}, args...)...)
		var res GrillResult
		_ = json.NewDecoder(strings.NewReader(out)).Decode(&res)
		return code, res, errOut
	}
	if code, res, _ := grill("--title", "Show empty list", "--body", body, "--parent", "1"); code != ExitBlocked || res.Pass || !strings.Contains(problemsText(res), "effort") {
		t.Fatalf("draft without effort: %d %+v", code, res)
	}
	before := len(g.Issues)
	code, res, errOut := grill("--title", "Show empty list", "--body", body, "--parent", "1", "--labels", "effort:S,priority:P2")
	if code != 0 || !res.Pass || len(g.Issues) != before {
		t.Fatalf("valid draft without --create: %d %+v %s", code, res, errOut)
	}
	// A human running grill writes under their own identity (WP5.3), untagged.
	code, res, errOut = grill("--title", "Show empty list", "--body", body, "--parent", "1", "--labels", "effort:S,priority:P2", "--create")
	if code != 0 || res.Issue == 0 || res.Identity != "human" || contains(g.LabelNames(res.Issue), "identity:human-fallback") {
		t.Fatalf("create by a human: %d %+v %s", code, res, errOut)
	}
	// An agent lane writes under the bot; with no bot yet, the human fallback is tagged (WP1.4).
	agent := map[string]string{"AAYU_GITHUB_TOKEN": "human-token", "AAYU_AGENT": "1"}
	if code, out, errOut := run(t, agent, opts, "grill", "story", "--repo", "o/r", "--json", "--title", "Show empty list", "--body", body, "--parent", "1",
		"--labels", "effort:S", "--create"); code != 0 || !strings.Contains(out, "identity:human-fallback") {
		t.Fatalf("create by an agent: %d %s %s", code, out, errOut)
	}
	g.Mu.Lock()
	parent := g.Issues[res.Issue].Parent
	g.Mu.Unlock()
	if parent != 1 {
		t.Fatalf("created story's parent: %d", parent)
	}
	if code, out, errOut := run(t, env, opts, "ready", strconv.Itoa(res.Issue), "--check", "--repo", "o/r"); code != 0 {
		t.Fatalf("the grilled story does not pass ready: %s %s", out, errOut)
	}
	if code, _, errOut := grill("--title", "x", "--body", "y", "--labels", "status:done"); code != ExitUsage || !strings.Contains(errOut, "not a label-held field") {
		t.Fatalf("status label: %d %s", code, errOut)
	}
}

func problemsText(r GrillResult) string {
	var b strings.Builder
	for _, p := range append(r.Create, r.Ready...) {
		b.WriteString(p.String() + "\n")
	}
	return b.String()
}
