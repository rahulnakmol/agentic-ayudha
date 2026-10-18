package cli

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
	"github.com/rahulnakmol/agentic-ayudha/internal/inspect"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
	"github.com/rahulnakmol/agentic-ayudha/internal/ready"
)

var coverageHolds = []proof.Check{{Floor: "coverage", Result: proof.Pass, Value: 100, Threshold: 90, Tool: "aayudha", Version: "1"}}

// shipCmd runs ship on PR prN with extra arguments and returns the exit code
// and the combined output.
func shipCmd(t *testing.T, env map[string]string, opts Options, prN int, args ...string) (int, string) {
	t.Helper()
	code, out, errOut := run(t, env, opts, append([]string{"ship", strconv.Itoa(prN), "--repo", "o/r"}, args...)...)
	return code, out + errOut
}

// zipOf zips name -> content; a name ending in "/" is a directory entry.
func zipOf(files map[string]string) []byte {
	var b bytes.Buffer
	zw := zip.NewWriter(&b)
	for name, body := range files {
		w, _ := zw.Create(name)
		if !strings.HasSuffix(name, "/") {
			_, _ = w.Write([]byte(body))
		}
	}
	_ = zw.Close()
	return b.Bytes()
}

// gateArtifact records a green merge gate for head whose only run carries
// arts as its artifacts.
func gateArtifact(g *fake.GitHub, head string, arts ...fake.ArtifactZip) {
	g.Mu.Lock()
	defer g.Mu.Unlock()
	g.Runs = []fake.WorkflowRun{{ID: 8000, Path: ".github/workflows/aayudha-merge-gate.yml", HeadSHA: head, Event: "pull_request_target", Status: "completed", Conclusion: "success"}}
	g.Artifacts = map[int64][]fake.ArtifactZip{8000: arts}
	g.CheckRuns[head] = []fake.CheckRun{{Name: "aayudha/merge-gate", Status: "completed", Conclusion: "success"}, {Name: "aayudha/inspect", Status: "completed", Conclusion: "success"}, {Name: "aayudha/pack", Status: "completed", Conclusion: "success"}}
}

// WP4.10/WP5.5 and S-4.2b: ship refuses, before landing or merging anything:
// without the human's token; a PR that does not exist, is already merged or
// closed, or serves no story; no pack from the merge gate for the head: no
// run, a run of the pull request's own copy of the workflow (a pull_request
// run, however green its jobs look), a successful run on another head, a
// failed run, no aayudha-pack artifact, an artifact that is not a zip or has
// an entry outside its directory, no pack for the story, an artifact whose
// bytes do not match pack.json, or a pack for another head; a merge gate check
// that is red or still running on the head; GitHub failing a read on the way
// to the pack (exit 5); a pack with no Ready snapshot on the ledger to verify
// against; and a repository without a ledger.
func TestShipRefusals(t *testing.T) {
	g, env, opts, dir, bare, prN, head := thinFixture(t)
	ship := func(n int, args ...string) (int, string) { return shipCmd(t, env, opts, n, args...) }

	if code, _, errOut := run(t, nil, opts, "ship", strconv.Itoa(prN), "--repo", "o/r"); code != ExitBlocked {
		t.Fatalf("ship without a GitHub token: %d %s", code, errOut)
	}
	if code, out := ship(999); code != ExitExternal || !strings.Contains(out, "404") {
		t.Fatalf("ship of a PR that does not exist: %d %s", code, out)
	}
	merged := g.AddPR("feat: done", "Refs #2", "founder", "story/2-done", head, "dev")
	closed := g.AddPR("feat: dropped", "Refs #2", "founder", "story/2-dropped", head, "dev")
	chore := g.AddPR("chore: tidy", "Tidies the build.", "founder", "chore/tidy", head, "dev")
	g.Mu.Lock()
	merged.Merged, merged.State = true, "closed"
	closed.State = "closed"
	g.Mu.Unlock()
	if code, out := ship(merged.Number); code != ExitUsage || !strings.Contains(out, "the PR is already merged") {
		t.Fatalf("ship of a merged PR: %d %s", code, out)
	}
	if code, out := ship(closed.Number); code != ExitUsage || !strings.Contains(out, "the PR is closed") {
		t.Fatalf("ship of a closed PR: %d %s", code, out)
	}
	if code, out := ship(chore.Number); code != ExitBlocked || !strings.Contains(out, "no story (Refs #n in the PR body)") {
		t.Fatalf("ship of a PR without Refs: %d %s", code, out)
	}
	if code, out := ship(prN); code != ExitBlocked || !strings.Contains(out, "no merge gate pack for the head "+short(head)+": no run of .github/workflows/aayudha-merge-gate.yml") {
		t.Fatalf("ship with no gate run: %d %s", code, out)
	}

	// Runs that are not the base branch's gate on this head, each with a pack that would otherwise ship.
	other := strings.Repeat("f", 40)
	for _, tt := range []struct {
		name string
		edit func(r *fake.WorkflowRun)
		want string
	}{
		{"the PR's own copy of the gate", func(r *fake.WorkflowRun) { r.Event = "pull_request" },
			"a pull_request run, so the workflow file came from the pull request, not the base branch"},
		{"a run on another head", func(r *fake.WorkflowRun) { r.HeadSHA = other }, "proves " + short(other) + ", not the head " + short(head)},
		{"a failed run", func(r *fake.WorkflowRun) { r.Conclusion = "failure" }, "is failure on the head"},
		{"a run in progress", func(r *fake.WorkflowRun) { r.Status, r.Conclusion = "in_progress", "" }, "is in_progress on the head"},
	} {
		gateRun(t, g, dir, prN, head, coverageHolds)
		g.Mu.Lock()
		g.Runs = g.Runs[len(g.Runs)-1:]
		tt.edit(&g.Runs[0])
		id := g.Runs[0].ID
		g.Mu.Unlock()
		if code, out := ship(prN); code != ExitBlocked || !strings.Contains(out, "run "+strconv.FormatInt(id, 10)+": "+tt.want) {
			t.Errorf("ship with %s: %d %s", tt.name, code, out)
		}
	}

	pack := func(id int64, files map[string]string) fake.ArtifactZip {
		return fake.ArtifactZip{ID: id, Name: "aayudha-pack", Zip: zipOf(files)}
	}
	tests := []struct {
		name string
		arts []fake.ArtifactZip
		code int
		want string
	}{
		{"no aayudha-pack artifact", []fake.ArtifactZip{{ID: 8001, Name: "coverage", Zip: zipOf(nil)}}, ExitBlocked, "run 8000: it kept no aayudha-pack artifact"},
		{"an artifact that is not a zip", []fake.ArtifactZip{{ID: 8002, Name: "aayudha-pack", Zip: []byte("not a zip")}}, ExitBlocked, "run 8000: its aayudha-pack artifact 8002: zip"},
		{"an entry outside the artifact", []fake.ArtifactZip{pack(8003, map[string]string{"2/": "", "../escape.json": "{}"})}, ExitBlocked, `artifact entry "../escape.json"`},
		{"no pack for the story", []fake.ArtifactZip{pack(8004, map[string]string{"README.txt": "nothing here"})}, ExitBlocked, "run 8000: its aayudha-pack artifact 8004 holds no pack for #2"},
	}
	for _, tt := range tests {
		gateArtifact(g, head, tt.arts...)
		if code, out := ship(prN); code != tt.code || !strings.Contains(out, tt.want) {
			t.Errorf("ship with %s: %d %s", tt.name, code, out)
		}
	}

	// An artifact whose evidence was changed after the gate hashed it.
	gateRun(t, g, dir, prN, head, coverageHolds)
	g.Mu.Lock()
	run := g.Runs[len(g.Runs)-1]
	zr, _ := zip.NewReader(bytes.NewReader(g.Artifacts[run.ID][0].Zip), int64(len(g.Artifacts[run.ID][0].Zip)))
	files := map[string]string{}
	for _, f := range zr.File {
		rc, _ := f.Open()
		b, _ := io.ReadAll(rc)
		files[f.Name] = string(b)
	}
	files["2/artifacts/evidence/criterion-1.jsonl"] = `{"Action":"pass","Test":"TestSomethingElse"}` + "\n"
	g.Runs, g.Artifacts[run.ID] = []fake.WorkflowRun{run}, []fake.ArtifactZip{pack(run.ID+1, files)}
	g.Mu.Unlock()
	if code, out := ship(prN); code != ExitBlocked || !strings.Contains(out, "artifact evidence/criterion-1.jsonl: hash mismatch") {
		t.Errorf("ship with a tampered artifact: %d %s", code, out)
	}

	// A green pack, but a merge gate check red or still running on the head.
	for _, gate := range []fake.CheckRun{
		{Name: "aayudha/merge-gate", Status: "completed", Conclusion: "failure"},
		{Name: "aayudha/pack", Status: "in_progress"},
	} {
		gateRun(t, g, dir, prN, head, coverageHolds)
		g.Mu.Lock()
		g.CheckRuns[head] = slices.Insert(g.CheckRuns[head], 0, gate)
		g.Mu.Unlock()
		want := gate.Name + " is " + or(gate.Conclusion, gate.Status) + " on "
		if code, out := ship(prN); code != ExitBlocked || !strings.Contains(out, want) || !strings.Contains(out, "the merge gate must be green") {
			t.Fatalf("ship with %s %s/%s: %d %s", gate.Name, gate.Status, gate.Conclusion, code, out)
		}
	}
	if got := git(t, bare, "ls-tree", "-r", "--name-only", "ledger", "proof/"); got != "" {
		t.Fatalf("a refused ship landed a pack:\n%s", got)
	}

	// GitHub failing a read on the way to the gate's pack: exit 5, nothing landed.
	gateRun(t, g, dir, prN, head, coverageHolds)
	g.Mu.Lock()
	runID := g.Runs[len(g.Runs)-1].ID
	g.Mu.Unlock()
	for _, fail := range []string{
		"GET /repos/o/r/commits/" + head + "/check-runs",
		"GET /repos/o/r/actions/workflows/aayudha-merge-gate.yml/runs",
		"GET /repos/o/r/actions/runs/" + strconv.FormatInt(runID, 10) + "/artifacts",
		"GET /repos/o/r/actions/artifacts/" + strconv.FormatInt(runID+1, 10) + "/zip",
	} {
		g.Mu.Lock()
		g.Fail = fail
		g.Mu.Unlock()
		if code, out := ship(prN); code != ExitExternal || !strings.Contains(out, "500") {
			t.Errorf("ship when %s fails: %d %s", fail, code, out)
		}
	}

	// The gate's pack names another head than the run it came from.
	gateRun(t, g, dir, prN, other, coverageHolds)
	g.Mu.Lock()
	g.Runs = g.Runs[len(g.Runs)-1:]
	g.Runs[0].HeadSHA = head
	g.CheckRuns[head] = g.CheckRuns[other]
	g.Mu.Unlock()
	if code, out := ship(prN); code != ExitBlocked || !strings.Contains(out, fmt.Sprintf("its pack is for #2, PR %d, at %s, not #2, PR %d, at %s", prN, short(other), prN, short(head))) {
		t.Fatalf("ship with a pack for another head: %d %s", code, out)
	}

	// The Ready snapshot is gone from the ledger: the landed pack cannot be checked against it.
	gateRun(t, g, dir, prN, head, coverageHolds)
	if _, err := (&ledger.Writer{Dir: dir}).Write(context.Background(), ledger.Commit{Message: "drop the snapshot", Delete: []string{ready.LedgerPath(2)}}); err != nil {
		t.Fatal(err)
	}
	if code, out := ship(prN); code != ExitBlocked || !strings.Contains(out, "the merge gate's pack does not verify: ") || !strings.Contains(out, "no Ready snapshot on the ledger") {
		t.Fatalf("ship without a Ready snapshot: %d %s", code, out)
	}
	git(t, dir, "push", "--quiet", "origin", "--delete", "ledger")
	if code, out := ship(prN); code != ExitBlocked || !strings.Contains(out, "run `aayu init` to create it") {
		t.Fatalf("ship without a ledger: %d %s", code, out)
	}
	g.Mu.Lock()
	defer g.Mu.Unlock()
	if g.Pulls[prN].Merged {
		t.Fatal("a refused ship merged the PR")
	}
}

// WP5.7 + WP5.5: a footprint whose every layer has a streak at the threshold,
// and no never layer, ships without a review, and the Complete section says
// why; until then ship needs an approving review. A story body that already
// has an Outcome section gets it replaced, the rest kept. --dry-run plans the
// merge and changes nothing. The lease is released even when its label is
// already gone.
func TestShipAutonomousFootprint(t *testing.T) {
	g, env, opts, dir, bare, prN, head := thinFixture(t)
	gateRun(t, g, dir, prN, head, coverageHolds, "L02")
	g.Mu.Lock()
	g.Fail = "GET /repos/o/r/pulls/" + strconv.Itoa(prN) + "/reviews"
	g.Mu.Unlock()
	if code, out := shipCmd(t, env, opts, prN); code != ExitExternal || !strings.Contains(out, "injected failure") {
		t.Fatalf("ship when the reviews cannot be read: %d %s", code, out)
	}
	if code, out := shipCmd(t, env, opts, prN); code != ExitBlocked || !strings.Contains(out, "no human pass: no approving review") {
		t.Fatalf("ship without a review below the streak: %d %s", code, out)
	}
	autonomous := []string{"--set", "autonomy.streak=0"}
	// A same-family inspect review (rung C) never lets an agent merge without
	// a person, whatever the streak. The rung is read only from the log of the
	// inspect job in the base branch's pull_request_target merge gate run at
	// the head: no record, a same-family one, one from a pull_request run (the
	// branch's own workflow), or a later line a reviewer could have injected
	// changes nothing; the gate's own different-family line allows it.
	blocked := func(why string) {
		t.Helper()
		if code, out := shipCmd(t, env, opts, prN, autonomous...); code != ExitBlocked || !strings.Contains(out, "never lets an agent merge without a person") {
			t.Fatalf("ship of an autonomous footprint %s: %d %s", why, code, out)
		}
	}
	blocked("with no inspect job")
	g.Mu.Lock()
	gate := slices.IndexFunc(g.Runs, func(r fake.WorkflowRun) bool { return r.HeadSHA == head && r.Event == "pull_request_target" })
	g.RunJobs = map[int64][]fake.CheckRun{g.Runs[gate].ID: {{ID: 7701, Name: "aayudha/inspect", Status: "completed", Conclusion: "success"}}}
	g.JobLogs = map[int64]string{7701: "2026-10-17T23:00:00.0000000Z inspect #" + strconv.Itoa(prN) + " at abc: merge (reviewed by codex)\n2026-10-17T23:00:00.0000000Z   review: same-family review (C): codex (openai), builder openai\n2026-10-17T23:00:00.0000000Z   P2 bug a.go:1 x\n  review: different-family review (A): injected\n"}
	g.Mu.Unlock()
	blocked("after a same-family review, with an injected line after it")
	g.Mu.Lock()
	g.Runs = append(g.Runs, fake.WorkflowRun{ID: 99001, Path: ".github/workflows/" + MergeGateWorkflow, HeadSHA: head, Event: "pull_request", Status: "completed", Conclusion: "success"})
	g.RunJobs[99001] = []fake.CheckRun{{ID: 7702, Name: "aayudha/inspect", Status: "completed", Conclusion: "success"}}
	g.JobLogs[7702] = "inspect #" + strconv.Itoa(prN) + " at abc: merge\n  review: different-family review (A): claude (anthropic), builder openai\n"
	g.Mu.Unlock()
	blocked("with a different-family line from the branch's own workflow")
	// Two PRs can share a head: a log whose header names another PR does not
	// count for this one.
	allowed := "2026-10-17T23:00:00.0000000Z   review: different-family review (A): claude (anthropic), builder openai\n"
	g.Mu.Lock()
	g.JobLogs[7701] = "2026-10-17T23:00:00.0000000Z inspect #" + strconv.Itoa(prN+1) + " at abc: merge (reviewed by claude)\n" + allowed
	g.Mu.Unlock()
	blocked("with a different-family log for another PR at the same head")
	// A cached judgement rests on a PR comment's state, which a branch's
	// workflow can write as the gate's account: it proves no review.
	g.Mu.Lock()
	g.JobLogs[7701] = "2026-10-17T23:00:00.0000000Z inspect #" + strconv.Itoa(prN) + " at abc: merge (cached) (reviewed by claude)\n" + allowed
	g.Mu.Unlock()
	blocked("with a cached different-family judgement")
	// A cached relabel to C (the PR body now declares the reviewer's family)
	// stands over an older fresh A: it only lowers the rung.
	g.Mu.Lock()
	g.JobLogs[7701] = "2026-10-17T23:00:00.0000000Z inspect #" + strconv.Itoa(prN) + " at abc: merge (reviewed by claude)\n" + allowed
	g.Runs = append(g.Runs, fake.WorkflowRun{ID: g.Runs[gate].ID + 100000, Path: ".github/workflows/" + MergeGateWorkflow, HeadSHA: head, Event: "pull_request_target", Status: "completed", Conclusion: "success"})
	g.RunJobs[g.Runs[gate].ID+100000] = []fake.CheckRun{{ID: 7703, Name: "aayudha/inspect", Status: "completed", Conclusion: "success"}}
	g.JobLogs[7703] = "2026-10-17T23:00:00.0000000Z inspect #" + strconv.Itoa(prN) + " at abc: merge (cached) (reviewed by claude)\n  review: same-family review (C): claude (anthropic), builder anthropic\n"
	g.Mu.Unlock()
	blocked("with a cached relabel to C after a fresh different-family review")
	g.Mu.Lock()
	g.Runs = g.Runs[:len(g.Runs)-1]
	g.Mu.Unlock()
	if code, out := shipCmd(t, env, opts, prN, append(autonomous, "--dry-run")...); code != 0 || !strings.Contains(out, "ship #"+strconv.Itoa(prN)+": --dry-run: merge, Complete section, status:merged, and cleanup planned") {
		t.Fatalf("ship --dry-run: %d %s", code, out)
	}
	g.Mu.Lock()
	dry := g.Pulls[prN].Merged
	g.Issues[2].Body += "\n\n### Outcome\n\nNot shipped yet.\n\n### Notes\n\nKeep this note.\n"
	g.Issues[2].Labels = slices.DeleteFunc(g.Issues[2].Labels, func(l fake.Label) bool { return l.Name == LeaseLabel })
	g.Mu.Unlock()
	if dry || contains(g.LabelNames(2), "status:merged") {
		t.Fatalf("ship --dry-run merged: %t %v", dry, g.LabelNames(2))
	}

	code, out := shipCmd(t, env, opts, prN, autonomous...)
	if code != 0 || !strings.Contains(out, "human pass: autonomous by founder — every layer of footprint L02 has a streak of at least 0 correction-free agent merges and none is a never layer") {
		t.Fatalf("ship of an autonomous footprint: %d %s", code, out)
	}
	g.Mu.Lock()
	body, merged := g.Issues[2].Body, g.Pulls[prN].Merged
	g.Mu.Unlock()
	if !merged || strings.Count(body, "### Outcome") != 1 || strings.Contains(body, "Not shipped yet.") || !strings.Contains(body, "Keep this note.") ||
		!strings.Contains(body, "- Human pass: not needed, autonomous footprint: every layer of footprint L02") {
		t.Fatalf("Complete section (merged %t):\n%s", merged, body)
	}
	var sr ShipRecord
	_ = json.Unmarshal([]byte(git(t, bare, "show", "ledger:proof/2/"+head+"/authoritative/ship.json")), &sr)
	if sr.HumanPass.Kind != "autonomous" || !sr.Streak.Autonomous || sr.Streak.Threshold != 0 || len(sr.Pass.Never) != 0 || sr.Review != inspect.RungA {
		t.Fatalf("ship record: %+v", sr)
	}
	if c := comment(g, 2, "<!-- aayudha:lease-released -->"); !strings.Contains(c, "shipped in #"+strconv.Itoa(prN)) {
		t.Fatalf("lease not released:\n%s", c)
	}
}

// Decision #593: with [autonomy] proof on, a PR into dev merges with no human
// pass once every required check is green, the pack verifies, and inspect is
// at merge with an accepted rung, even on a never layer with no plan review;
// a red required check, a rung outside proof_rungs, or the narrow set (a
// listed path, a workflow that reads a secret, a change to who may approve)
// stops it, and the PR waits for a person.
func TestShipOnProof(t *testing.T) {
	g, env, opts, dir, bare, prN, head := thinFixture(t)
	gateRun(t, g, dir, prN, head, coverageHolds, "L07")
	base := strings.Repeat("b", 40)
	// The checkout (the PR's own config) turns proof off and empties the
	// narrow set; the base revision's config is the policy, so it is ignored.
	_ = os.MkdirAll(filepath.Join(dir, ".aayudha"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, ".aayudha", "config.toml"), []byte("version = 1\n[autonomy]\nproof = false\nalways_person = []\nalways_person_config = []\n"), 0o644)
	g.Mu.Lock()
	gate := slices.IndexFunc(g.Runs, func(r fake.WorkflowRun) bool { return r.HeadSHA == head && r.Event == "pull_request_target" })
	g.RunJobs = map[int64][]fake.CheckRun{g.Runs[gate].ID: {{ID: 7701, Name: "aayudha/inspect", Status: "completed", Conclusion: "success"}}}
	g.JobLogs = map[int64]string{7701: "2026-10-17T23:00:00.0000000Z inspect #" + strconv.Itoa(prN) + " at abc: merge (reviewed by codex)\n2026-10-17T23:00:00.0000000Z   review: same-family review (C): codex (openai), builder openai\n"}
	g.RequiredChecks = []string{"build", "lint", "protected-paths"}
	// build was rerun green: its newest run counts, not the stale failure.
	g.CheckRuns[head] = append(g.CheckRuns[head], fake.CheckRun{Name: "build", Status: "completed", Conclusion: "success"},
		fake.CheckRun{Name: "build", Status: "completed", Conclusion: "failure"},
		fake.CheckRun{Name: "lint", Status: "completed", Conclusion: "failure"}, fake.CheckRun{Name: "protected-paths", Status: "completed", Conclusion: "failure"})
	g.Files = map[string]string{".github/CODEOWNERS": "/skills/ @founder\n"}
	secrets := "on: push\njobs:\n  x:\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo ${{ secrets['TOKEN'] }}\n"
	conf := "version = 1\n\n[notify]\nchannel = \"file\"\n\n[autonomy]\napprover = \"\"\nproof = true\nproof_rungs = [\"A\", \"B\", \"C\"]\n" +
		"always_person = [\".github/CODEOWNERS\", \"internal/identity/\"]\nalways_person_config = [\".aayudha/config.toml\"]\n"
	g.RefFiles = map[string]map[string]string{
		base: {".github/CODEOWNERS": "/skills/ @founder\n", ".github/workflows/ci.yml": secrets, ".aayudha/config.toml": conf},
		head: {".github/workflows/ci.yml": secrets, ".aayudha/config.toml": strings.Replace(conf, `approver = ""`, `approver = "mallory"`, 1)},
	}
	g.Pulls[prN].Files = []string{"empty.txt"}
	g.Mu.Unlock()
	refused := func(want string) {
		t.Helper()
		if code, out := shipCmd(t, env, opts, prN); code != ExitBlocked || !strings.Contains(out, want) {
			t.Fatalf("ship, want %q: %d %s", want, code, out)
		}
		g.Mu.Lock()
		defer g.Mu.Unlock()
		if g.Pulls[prN].Merged {
			t.Fatalf("merged although %q", want)
		}
	}
	refused("required check lint is failure on " + short(head) + "; never merge with a red required check")
	g.Mu.Lock()
	g.CheckRuns[head][len(g.CheckRuns[head])-2].Conclusion = "success" // lint
	g.Pulls[prN].Files = []string{"empty.txt", "internal/identity/review.go"}
	g.Mu.Unlock()
	refused("no merge on proof: it touches internal/identity/review.go, in the narrow set that always needs a person ([autonomy] always_person: internal/identity/)")
	g.Mu.Lock()
	g.Pulls[prN].Files = []string{"empty.txt", ".github/workflows/ci.yml"}
	g.Mu.Unlock()
	refused("no merge on proof: it touches .github/workflows/ci.yml, a workflow that reads a secret or names an environment")
	g.Mu.Lock()
	g.Pulls[prN].Files = []string{"empty.txt", ".aayudha/config.toml"}
	g.Mu.Unlock()
	refused("no merge on proof: it changes [autonomy] in .aayudha/config.toml, which decides who may approve or bypass")
	g.Mu.Lock()
	g.RefFiles[base][".aayudha/config.toml"] = strings.Replace(conf, `"C"]`, `]`, 1)
	g.RefFiles[head][".aayudha/config.toml"] = strings.Replace(g.RefFiles[base][".aayudha/config.toml"], `"file"`, `"slack"`, 1) // another table: no person needed
	g.Pulls[prN].Files = []string{"empty.txt", ".aayudha/config.toml", "skills/build/SKILL.md"}
	g.Mu.Unlock()
	refused("no merge on proof: the review rung at the head is C, not one of A, B ([autonomy] proof_rungs)")
	g.Mu.Lock()
	g.RefFiles[base][".aayudha/config.toml"] = conf
	g.RefFiles[head][".aayudha/config.toml"] = strings.Replace(conf, `"file"`, `"slack"`, 1)
	g.Mu.Unlock()

	code, out := shipCmd(t, env, opts, prN)
	if code != 0 || !strings.Contains(out, "human pass: proof by founder") {
		t.Fatalf("ship on proof: %d %s", code, out)
	}
	var sr ShipRecord
	_ = json.Unmarshal([]byte(git(t, bare, "show", "ledger:proof/2/"+head+"/authoritative/ship.json")), &sr)
	if sr.HumanPass.Kind != "proof" || sr.Proof == nil || sr.Proof.Rung != inspect.RungC || !slices.Equal(sr.Proof.Checks, []string{"build", "lint"}) ||
		!slices.Equal(sr.Proof.Protected, []string{"skills/build/SKILL.md"}) || sr.PlanReview != nil || !slices.Equal(sr.Footprint, []string{"L07"}) {
		t.Fatalf("ship record: %+v %+v", sr.HumanPass, sr.Proof)
	}
	g.Mu.Lock()
	body, merged := g.Issues[2].Body, g.Pulls[prN].Merged
	g.Mu.Unlock()
	if !merged || !strings.Contains(body, "- Human pass: not needed, merged on proof (Decision #593): required checks build, lint green") ||
		!strings.Contains(body, "CODEOWNERS paths skills/build/SKILL.md merged with no code owner's review") {
		t.Fatalf("Complete section (merged %t):\n%s", merged, body)
	}
}

// WP5.5: when the Engineering Manager seat is paired with a human, that
// human's approval is the only human pass, and --solo-bypass is refused (a
// second human could review), as it is when a bot identity exists. A merge
// GitHub refuses leaves the story as it was; a lease ship cannot read is
// reported, and the merge stands.
func TestShipEMSeatApprover(t *testing.T) {
	g, env, opts, dir, bare, prN, head := thinFixture(t)
	gateRun(t, g, dir, prN, head, coverageHolds)
	g.Mu.Lock()
	g.Pulls[prN].Reviews = []fake.Review{review("reviewer", "User", "APPROVED", head)}
	g.Mu.Unlock()
	seat := []string{"--set", "seats.engineering-manager.mode=paired", "--set", "seats.engineering-manager.human=em-lead"}
	if code, out := shipCmd(t, env, opts, prN, seat...); code != ExitBlocked || !strings.Contains(out, "approval by reviewer is not the human pass: it must come from em-lead (seat)") {
		t.Fatalf("ship with a non-EM approval: %d %s", code, out)
	}
	if code, out := shipCmd(t, env, opts, prN, append(seat, "--solo-bypass")...); code != ExitBlocked || !strings.Contains(out, "a second human (em-lead) holds a seat") {
		t.Fatalf("solo bypass with a paired seat: %d %s", code, out)
	}
	if code, out := shipCmd(t, env, opts, prN, append(seat, "--solo-bypass", "--set", "identity.app_id=42", "--set", "identity.app_slug=aayudha-bot")...); code != ExitBlocked || !strings.Contains(out, "this repository has a bot identity") {
		t.Fatalf("solo bypass with a bot App: %d %s", code, out)
	}

	g.Mu.Lock()
	g.Pulls[prN].Reviews = append(g.Pulls[prN].Reviews, review("em-lead", "User", "APPROVED", head))
	g.Fail = "PUT /repos/o/r/pulls/" + strconv.Itoa(prN) + "/merge"
	g.Mu.Unlock()
	if code, out := shipCmd(t, env, opts, prN, seat...); code != ExitExternal || contains(g.LabelNames(2), "status:merged") {
		t.Fatalf("ship when the merge fails: %d %s %v", code, out, g.LabelNames(2))
	}

	g.Mu.Lock()
	g.Fail, g.FailAfter = "GET /repos/o/r/issues/2/comments", 0
	g.Mu.Unlock()
	code, out := shipCmd(t, env, opts, prN, seat...)
	if code != 0 || !strings.Contains(out, "human pass: review by em-lead") || !strings.Contains(out, "warning: the build lease on #2 was not released") {
		t.Fatalf("ship by the EM's human: %d %s", code, out)
	}
	var sr ShipRecord
	_ = json.Unmarshal([]byte(git(t, bare, "show", "ledger:proof/2/"+head+"/authoritative/ship.json")), &sr)
	if sr.HumanPass.Login != "em-lead" || sr.Pass.As != "seat" || strings.Join(sr.Pass.Approvers, ",") != "em-lead" {
		t.Fatalf("ship record: %+v", sr)
	}
	if !contains(g.LabelNames(2), LeaseLabel) {
		t.Fatalf("the unread lease was released: %v", g.LabelNames(2))
	}
}
