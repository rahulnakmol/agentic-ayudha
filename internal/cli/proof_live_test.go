package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"go.yaml.in/yaml/v4"

	"github.com/rahulnakmol/agentic-ayudha/internal/github/scratch"
	"github.com/rahulnakmol/agentic-ayudha/internal/initrepo"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/ready"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

const liveSum = "package sum\n\n// Sum returns a + b.\nfunc Sum(a, b int) int { return a + b }\n"

const liveSumTest = `package sum

import "testing"

func TestSum(t *testing.T) {
	if Sum(2, 3) != 5 {
		t.Fatal("2 + 3")
	}
}

func TestSumNegative(t *testing.T) {
	if Sum(-2, -3) != -5 {
		t.Fatal("-2 + -3")
	}
}
`

// TestLiveShipFromGateArtifact proves S-4.2b against GitHub (AAYU_LIVE=1,
// AAYU_LIVE_VERSION = a pushed agentic-ayudha commit the scratch gate
// installs; AAYU_LIVE_KEEP=1 keeps the repository as evidence). On a public
// aayu-scratch-* repository with the `init --gates` merge gate (its inspect
// step stubbed: the scratch repository holds no model key) and no bot App,
// bucket, or secret:
//
//  1. the PR's gate run keeps the aayudha-pack artifact and the pack job keeps
//     one PR comment with the verdicts, floors, pack hash, and artifact link;
//  2. after a failing commit, ship refuses the earlier pack ("proves …, not
//     the head") and the failed run;
//  3. a PR that adds a pull_request trigger to the gate file runs its own
//     copy, which uploads a green, forged aayudha-pack for the head; ship
//     refuses it as "a pull_request run";
//  4. on a good head, ship --solo-bypass (the founder's token standing in for
//     the human) lands pack.json and the ship record on the ledger and
//     merges, and nothing landed before.
func TestLiveShipFromGateArtifact(t *testing.T) {
	if os.Getenv("AAYU_LIVE") != "1" {
		t.Skip("live test: set AAYU_LIVE=1")
	}
	version := os.Getenv("AAYU_LIVE_VERSION")
	if version == "" {
		t.Skip("set AAYU_LIVE_VERSION to a pushed agentic-ayudha commit for the scratch gate to install")
	}
	tok := scratch.Token(t)
	admin := os.Getenv("AAYU_LIVE_ADMIN_TOKEN")
	if admin == "" {
		admin = tok
	}
	repo := scratch.Repo(t, admin)
	api := func(method, path string, body any) map[string]any {
		var out map[string]any
		_ = json.Unmarshal(liveAPI(t, tok, method, path, body), &out)
		return out
	}

	dir := filepath.Join(t.TempDir(), "clone")
	g := &ledger.Writer{Dir: dir, Token: admin}
	gitDo := func(args ...string) string {
		t.Helper()
		out, err := g.Output(t.Context(), dir, args...)
		if err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
		return out
	}
	write := func(rel, body string) {
		var y map[string]any
		if err := yaml.Unmarshal([]byte(body), &y); strings.HasSuffix(rel, ".yml") && err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		p := filepath.Join(dir, filepath.FromSlash(rel))
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := g.Exec(t.Context(), filepath.Dir(dir), "clone", "--quiet", "https://github.com/"+repo+".git", dir); err != nil {
		t.Fatal(err)
	}
	gitDo("config", "user.name", "aayudha live test")
	gitDo("config", "user.email", "aayudha-live@users.noreply.github.com")

	// dev: a Go module, the config, and the merge gate exactly as `init --gates` renders it for a release,
	// pointed at the commit under test and fetched straight from GitHub (it is not on the module proxy yet).
	gate := string(initrepo.GateWorkflows("v0.0.1")[initrepo.MergeGateWorkflow])
	gate = strings.ReplaceAll(gate, "v0.0.1", version)
	gate = strings.ReplaceAll(gate, `if [ "$AAYU_VERSION" = dev ]; then`, `if true; then`)
	gate = strings.Replace(gate, `run: aayudha inspect "$PR" --ci --repo "$REPO"`, `run: echo "inspect stubbed by the aayudha live test, since the scratch repository holds no model key"`, 1)
	write(".github/workflows/"+initrepo.MergeGateWorkflow, gate)
	write("go.mod", "module example.com/sum\n\ngo 1.25\n")
	write("README.md", "Scratch product for aayudha's live proof test.\n")
	write(".aayudha/config.toml", "version = 1\n")
	gitDo("add", ".")
	gitDo("commit", "--quiet", "-m", "ci: merge gate from aayu init --gates")
	gitDo("push", "--quiet", "origin", "HEAD:refs/heads/dev")
	gitDo("checkout", "--quiet", "--orphan", "ledger")
	gitDo("rm", "-r", "--quiet", "--cached", ".")
	write("LEDGER.md", "ledger\n")
	gitDo("add", "LEDGER.md")
	gitDo("commit", "--quiet", "-m", "ledger")
	gitDo("push", "--quiet", "origin", "ledger")
	gitDo("checkout", "--quiet", "-f", "-B", "dev", "origin/dev")
	gitDo("clean", "-fdq")
	// Public: Actions minutes are free, and a public repository is the case where artifacts are most exposed.
	liveAPI(t, admin, "PATCH", "/repos/"+repo, map[string]any{"default_branch": "dev", "private": false})

	// A Feature with a Ready Story whose criteria Go tests prove.
	issue := func(title, typ string, labels []string, f func(map[string]string)) (int, int64) {
		is := api("POST", "/repos/"+repo+"/issues", map[string]any{"title": title, "body": fixtureBody(t, typ, f), "labels": append([]string{"type:" + typ}, labels...)})
		return int(is["number"].(float64)), int64(is["id"].(float64))
	}
	feature, _ := issue("Arithmetic", "feature", []string{"status:in-progress"}, nil)
	story, storyID := issue("Add two numbers", "story", []string{"status:planned", "effort:S"}, func(m map[string]string) {
		m[workitems.FieldCriteria] = "1. [api-trace] state: Sum(2, 3); expected: 5\n2. [api-trace] state: Sum(-2, -3); expected: -5"
	})
	api("POST", fmt.Sprintf("/repos/%s/issues/%d/sub_issues", repo, feature), map[string]any{"sub_issue_id": storyID})
	// A person approved the Feature to build (Story #996).
	is := api("GET", fmt.Sprintf("/repos/%s/issues/%d", repo, story), nil)
	it, _ := workitems.Parse(story, is["title"].(string), is["body"].(string), "", []string{"type:story", "status:planned", "effort:S"})
	gitDo("checkout", "--quiet", "-B", "ledger", "origin/ledger")
	write(ready.ApprovalPath(feature), approvalOf(feature, it))
	gitDo("add", "-A")
	gitDo("commit", "--quiet", "-m", "specs: Approve to build")
	gitDo("push", "--quiet", "origin", "ledger")
	gitDo("checkout", "--quiet", "-f", "-B", "dev", "origin/dev")
	env := map[string]string{"AAYU_GITHUB_TOKEN": tok, "PATH": os.Getenv("PATH")}
	opts := Options{Dir: dir, IsTerminal: stdinTTY} // ship --solo-bypass is a person's act at a terminal
	if code, out, errOut := run(t, env, opts, "ready", strconv.Itoa(story), "--repo", repo); code != 0 {
		t.Fatalf("ready: %d %s %s", code, out, errOut)
	}

	// The story's PR.
	branch := fmt.Sprintf("story/%d-add-two-numbers", story)
	gitDo("checkout", "--quiet", "-b", branch)
	write("sum.go", liveSum)
	write("sum_test.go", liveSumTest)
	write(fmt.Sprintf(".aayudha/evidence/%d.toml", story), "[[criterion]]\nid = 1\ntests = [\"TestSum\"]\n\n[[criterion]]\nid = 2\ntests = [\"TestSumNegative\"]\n")
	push := func(msg string) string {
		gitDo("add", "-A")
		gitDo("commit", "--quiet", "-m", msg)
		gitDo("push", "--quiet", "origin", "HEAD")
		return gitDo("rev-parse", "HEAD")
	}
	h1 := push("feat: add two numbers")
	pr := api("POST", "/repos/"+repo+"/pulls", map[string]any{"title": "feat: add two numbers", "head": branch, "base": "dev", "body": fmt.Sprintf("Refs #%d", story)})
	prN := int(pr["number"].(float64))

	// waitRuns waits until the newest gate run of each event on head has completed.
	waitRuns := func(head string, events ...string) map[string]map[string]any {
		t.Helper()
		for deadline := time.Now().Add(30 * time.Minute); ; time.Sleep(15 * time.Second) {
			var list struct {
				Runs []map[string]any `json:"workflow_runs"`
			}
			_ = json.Unmarshal(liveAPI(t, tok, "GET", "/repos/"+repo+"/actions/workflows/"+initrepo.MergeGateWorkflow+"/runs?per_page=50&head_sha="+head, nil), &list)
			newest := map[string]map[string]any{}
			for _, r := range list.Runs { // newest first
				if ev := r["event"].(string); newest[ev] == nil {
					newest[ev] = r
				}
			}
			done := true
			for _, ev := range events {
				done = done && newest[ev] != nil && newest[ev]["status"] == "completed"
			}
			if done {
				for _, ev := range events {
					t.Logf("gate run on %s (%s): %s %s", short(head), ev, newest[ev]["conclusion"], newest[ev]["html_url"])
				}
				return newest
			}
			if time.Now().After(deadline) {
				t.Fatalf("gate runs on %s did not complete: %v", short(head), newest)
			}
		}
	}
	artifactOf := func(run map[string]any) map[string]any {
		var list struct {
			Artifacts []map[string]any `json:"artifacts"`
		}
		_ = json.Unmarshal(liveAPI(t, tok, "GET", fmt.Sprintf("/repos/%s/actions/runs/%.0f/artifacts", repo, run["id"]), nil), &list)
		for _, a := range list.Artifacts {
			if a["name"] == "aayudha-pack" {
				return a
			}
		}
		return nil
	}
	gateComment := func() string {
		var cs []map[string]any
		_ = json.Unmarshal(liveAPI(t, tok, "GET", fmt.Sprintf("/repos/%s/issues/%d/comments?per_page=100", repo, prN), nil), &cs)
		var body string
		for _, c := range cs {
			if b, _ := c["body"].(string); strings.Contains(b, ProofMarker) {
				if body != "" {
					t.Fatalf("more than one gate comment")
				}
				body = b + "\n(" + c["html_url"].(string) + ")"
			}
		}
		return body
	}
	ship := func(args ...string) (int, string) {
		code, out, errOut := run(t, env, opts, append([]string{"ship", strconv.Itoa(prN), "--repo", repo}, args...)...)
		t.Logf("ship %v: exit %d\n%s%s", args, code, out, errOut)
		return code, out + errOut
	}
	ledgerProof := func() string {
		gitDo("fetch", "--quiet", "origin", "ledger")
		out, _ := g.Output(t.Context(), dir, "ls-tree", "-r", "--name-only", "origin/ledger", "proof/")
		return out
	}

	// 1. The gate keeps the pack as a run artifact and comments on the PR.
	r1 := waitRuns(h1, "pull_request_target")["pull_request_target"]
	a1 := artifactOf(r1)
	if r1["conclusion"] != "success" || a1 == nil {
		t.Fatalf("gate on %s: %v, artifact %v", short(h1), r1["conclusion"], a1)
	}
	c1 := gateComment()
	for _, want := range []string{"### Proof pack: proven", "`pack.json` SHA-256: `", fmt.Sprintf("/actions/runs/%.0f/artifacts/%.0f", r1["id"], a1["id"]), "kept until "} {
		if !strings.Contains(c1, want) {
			t.Fatalf("gate comment lacks %q:\n%s", want, c1)
		}
	}
	t.Logf("artifact %.0f (expires %v); gate comment:\n%s", a1["id"], a1["expires_at"], c1)

	// 2. A failing commit: the earlier pack proves another SHA, and the new run failed.
	write("sum.go", strings.Replace(liveSum, "return a + b", "return a - b", 1))
	h2 := push("fix: subtract (wrong)")
	r2 := waitRuns(h2, "pull_request_target")["pull_request_target"]
	if r2["conclusion"] != "failure" {
		t.Fatalf("gate on the failing head: %v", r2["conclusion"])
	}
	t.Logf("gate comment after the failing head:\n%s", gateComment())
	if code, out := ship("--solo-bypass"); code != ExitBlocked || !strings.Contains(out, fmt.Sprintf("proves %s, not the head %s", short(h1), short(h2))) || !strings.Contains(out, "is failure on the head") {
		t.Fatalf("ship on another SHA's pack: %d %s", code, out)
	}

	// 3. The PR edits the gate file to also run on pull_request, and its copy uploads a green pack built from
	// the earlier, passing code: ship refuses it, because only the base branch's copy counts.
	forged := strings.Replace(gate, "on:\n  pull_request_target:", "on:\n  pull_request:\n  pull_request_target:", 1)
	forged = strings.Replace(forged, "group: aayudha-merge-gate-${{", "group: aayudha-merge-gate-${{ github.event_name }}-${{", 1)
	forged = strings.Replace(forged, "          set +e\n", "          set +e\n          git checkout "+h1+" -- sum.go\n", 1)
	forged = strings.Replace(forged, "          exit \"$code\"\n", "          exit 0\n", 1)
	forged = strings.Replace(forged, `run: aayudha proof comment "$STORY" --ci --repo "$REPO"`, `run: echo "the forger skips the comment"`, 1)
	write(".github/workflows/"+initrepo.MergeGateWorkflow, forged)
	h3 := push("ci: run the gate on pull_request too")
	r3 := waitRuns(h3, "pull_request_target", "pull_request")
	if r3["pull_request"]["conclusion"] != "success" || artifactOf(r3["pull_request"]) == nil || r3["pull_request_target"]["conclusion"] != "failure" {
		t.Fatalf("forged run %v (artifact %v), real run %v", r3["pull_request"]["conclusion"], artifactOf(r3["pull_request"]), r3["pull_request_target"]["conclusion"])
	}
	if code, out := ship("--solo-bypass"); code != ExitBlocked || !strings.Contains(out, fmt.Sprintf("run %.0f: a pull_request run", r3["pull_request"]["id"])) {
		t.Fatalf("ship on the forged artifact: %d %s", code, out)
	}
	if got := ledgerProof(); got != "" {
		t.Fatalf("a refused ship landed proof:\n%s", got)
	}

	// 4. A good head with the base branch's gate file: ship lands the pack and merges.
	write("sum.go", liveSum)
	write(".github/workflows/"+initrepo.MergeGateWorkflow, gate)
	h4 := push("fix: add again, and restore the gate")
	r4 := waitRuns(h4, "pull_request_target")["pull_request_target"]
	if r4["conclusion"] != "success" {
		t.Fatalf("gate on the good head: %v", r4["conclusion"])
	}
	c4 := gateComment()
	if !strings.Contains(c4, "### Proof pack: proven") || !strings.Contains(c4, short(h4)) {
		t.Fatalf("gate comment on the good head:\n%s", c4)
	}
	t.Logf("gate comment on the good head:\n%s", c4)
	if code, out := ship(); code != ExitBlocked || !strings.Contains(out, "no human pass") {
		t.Fatalf("ship without a human pass: %d %s", code, out)
	}
	api("POST", fmt.Sprintf("/repos/%s/issues/%d/comments", repo, prN), map[string]any{"body": SoloBypassPrefix + " aayudha live test (S-4.2b); one human holds every seat on this scratch repository."})
	code, out := ship("--solo-bypass")
	if code != 0 || !strings.Contains(out, fmt.Sprintf("artifact %.0f of run %.0f", artifactOf(r4)["id"], r4["id"])) {
		t.Fatalf("ship: %d %s", code, out)
	}
	landed := ledgerProof()
	base := fmt.Sprintf("proof/%d/%s/authoritative/", story, h4)
	if !strings.Contains(landed, base+"pack.json") || !strings.Contains(landed, base+"ship.json") {
		t.Fatalf("ledger after ship:\n%s", landed)
	}
	var rec ShipRecord
	_ = json.Unmarshal([]byte(gitDo("show", "origin/ledger:"+base+"ship.json")), &rec)
	if rec.GateRun != int64(r4["id"].(float64)) || rec.PackSHA256 == "" || !strings.Contains(c4, rec.PackSHA256) {
		t.Fatalf("ship record %+v vs comment:\n%s", rec, c4)
	}
	if p := api("GET", fmt.Sprintf("/repos/%s/pulls/%d", repo, prN), nil); p["merged"] != true {
		t.Fatalf("PR not merged: %v", p["state"])
	}
	t.Logf("shipped: PR https://github.com/%s/pull/%d merged as %s; ledger:\n%s", repo, prN, rec.MergeCommit, landed)
}
