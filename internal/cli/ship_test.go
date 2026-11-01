package cli

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/autonomy"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
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
// arts as its artifacts, a run of pull request pr.
func gateArtifact(g *fake.GitHub, pr int, head string, arts ...fake.ArtifactZip) {
	g.Mu.Lock()
	defer g.Mu.Unlock()
	g.Runs = []fake.WorkflowRun{{ID: 8000, Path: ".github/workflows/aayudha-merge-gate.yml", HeadSHA: head, Event: "pull_request_target", Status: "completed", Conclusion: "success",
		PullRequests: []fake.RunPR{{Number: pr, Base: fake.Branch{Ref: "dev"}}}}}
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
		gateArtifact(g, prN, head, tt.arts...)
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
	// Bug #1031: a check of a gate check's name from outside the gate's own
	// suites stands in for nothing. A newer success of that name, from another
	// workflow or from the PR's own copy of the gate on a review event, does not
	// hide the gate's red run; and with no run of the gate's own the check is missing.
	gateRun(t, g, dir, prN, head, coverageHolds)
	g.Mu.Lock()
	g.Runs = append(g.Runs, fake.WorkflowRun{ID: 9600, Path: ".github/workflows/aayudha-merge-gate.yml", HeadSHA: head, Event: "pull_request_review", Status: "completed", Conclusion: "success", CheckSuiteID: 78})
	gateOwn := g.CheckRuns[head]
	g.CheckRuns[head] = append([]fake.CheckRun{
		{Name: "aayudha/merge-gate", Status: "completed", Conclusion: "success", Suite: &fake.Suite{ID: 77}},
		{Name: "aayudha/merge-gate", Status: "completed", Conclusion: "success", Suite: &fake.Suite{ID: 78}},
		{Name: "aayudha/merge-gate", Status: "completed", Conclusion: "failure"}}, gateOwn...)
	g.Mu.Unlock()
	if code, out := ship(prN); code != ExitBlocked || !strings.Contains(out, "aayudha/merge-gate is failure on "+short(head)) {
		t.Fatalf("ship with a same-name green check from another suite above the gate's red one: %d %s", code, out)
	}
	g.Mu.Lock()
	g.CheckRuns[head] = []fake.CheckRun{
		{Name: "aayudha/merge-gate", Status: "completed", Conclusion: "success", Suite: &fake.Suite{ID: 77}},
		{Name: "aayudha/inspect", Status: "completed", Conclusion: "success", Suite: &fake.Suite{ID: 77}},
		{Name: "aayudha/pack", Status: "completed", Conclusion: "success", Suite: &fake.Suite{ID: 77}}}
	g.Mu.Unlock()
	if code, out := ship(prN); code != ExitBlocked || !strings.Contains(out, "aayudha/merge-gate is missing on "+short(head)) {
		t.Fatalf("ship with gate checks only from another suite: %d %s", code, out)
	}
	g.Mu.Lock()
	g.CheckRuns[head] = gateOwn
	g.Mu.Unlock()
	// Two pull requests can share a head. The gate's run for the other one, or
	// a run GitHub lists for both, is not this PR's gate: its newer green check
	// does not hide this PR's own red one.
	twin := g.AddPR("feat: twin", "Refs #2", "founder", "story/2-twin", head, "dev")
	for _, prs := range [][]fake.RunPR{{{Number: twin.Number}}, {{Number: twin.Number}, {Number: prN}}} {
		gateRun(t, g, dir, prN, head, coverageHolds)
		g.Mu.Lock()
		g.Runs = append(g.Runs, fake.WorkflowRun{ID: 9800, Path: ".github/workflows/aayudha-merge-gate.yml", HeadSHA: head, Event: "pull_request_target", Status: "completed", Conclusion: "success",
			CheckSuiteID: 79, PullRequests: prs})
		g.CheckRuns[head] = append([]fake.CheckRun{{Name: "aayudha/merge-gate", Status: "completed", Conclusion: "success", Suite: &fake.Suite{ID: 79}},
			{Name: "aayudha/merge-gate", Status: "completed", Conclusion: "failure"}}, g.CheckRuns[head][1:]...)
		g.Mu.Unlock()
		if code, out := ship(prN); code != ExitBlocked || !strings.Contains(out, "aayudha/merge-gate is failure on "+short(head)) {
			t.Fatalf("ship with a green gate run for PRs %v on the same head above this PR's red one: %d %s", prs, code, out)
		}
	}
	g.Mu.Lock()
	g.CheckRuns[head], twin.State = gateOwn, "closed"
	g.Runs = slices.DeleteFunc(g.Runs, func(r fake.WorkflowRun) bool { return r.ID == 9800 })
	g.Mu.Unlock()
	// A review on the PR runs the merge gate for its reply job only (#690, #643):
	// that run's skipped gate checks, newer than the gate's own, hide nothing,
	// and ship goes on to the human pass. A newer gate run's red check still refuses.
	gateRun(t, g, dir, prN, head, coverageHolds)
	suite := func(id int64, runs ...fake.CheckRun) []fake.CheckRun {
		for i := range runs {
			runs[i].Status, runs[i].Suite = "completed", &fake.Suite{ID: id}
		}
		return runs
	}
	g.Mu.Lock()
	g.Runs[len(g.Runs)-1].CheckSuiteID = 1 // the gate's run; suite 2 is a review event's reply run
	gate := suite(1, append(g.CheckRuns[head], fake.CheckRun{Name: "aayudha/inspect-reply", Conclusion: "skipped"})...)
	reply := suite(2, fake.CheckRun{Name: "aayudha/inspect-reply", Conclusion: "failure"}, fake.CheckRun{Name: "aayudha/merge-gate", Conclusion: "skipped"},
		fake.CheckRun{Name: "aayudha/inspect", Conclusion: "skipped"}, fake.CheckRun{Name: "aayudha/pack", Conclusion: "skipped"})
	g.CheckRuns[head] = append(slices.Clone(reply), gate...)
	g.Mu.Unlock()
	if code, out := ship(prN); code != ExitBlocked || strings.Contains(out, "the merge gate must be green") || !strings.Contains(out, "no human pass") {
		t.Fatalf("ship with a reply run's skipped checks newer than the gate's green ones: %d %s", code, out)
	}
	g.Mu.Lock()
	g.Runs = append(g.Runs, fake.WorkflowRun{ID: 9700, Path: ".github/workflows/aayudha-merge-gate.yml", HeadSHA: head, Event: "pull_request_target", Status: "completed", Conclusion: "failure", CheckSuiteID: 3,
		PullRequests: []fake.RunPR{{Number: prN, Base: fake.Branch{Ref: "dev"}}}})
	g.CheckRuns[head] = append(suite(3, fake.CheckRun{Name: "aayudha/merge-gate", Conclusion: "failure"}, fake.CheckRun{Name: "aayudha/inspect-reply", Conclusion: "skipped"}), append(reply, gate...)...)
	g.Mu.Unlock()
	if code, out := ship(prN); code != ExitBlocked || !strings.Contains(out, "aayudha/merge-gate is failure on "+short(head)) {
		t.Fatalf("ship with a newer red gate run behind a reply run: %d %s", code, out)
	}
	// Bug #1162: a newer gate run on the head failed in a job outside the gate
	// checks, so the pack is the older run's. The pack lands only with its own
	// run's gate checks: their red check refuses, though the newer run's gate
	// checks are green. With them green, ship goes on to the human pass.
	gateRun(t, g, dir, prN, head, coverageHolds)
	g.Mu.Lock()
	packRun := &g.Runs[len(g.Runs)-1]
	packRun.CheckSuiteID = 5
	packID := packRun.ID
	g.Runs = append(g.Runs, fake.WorkflowRun{ID: 9900, Path: ".github/workflows/aayudha-merge-gate.yml", HeadSHA: head, Event: "pull_request_target", Status: "completed", Conclusion: "failure", CheckSuiteID: 6,
		PullRequests: []fake.RunPR{{Number: prN, Base: fake.Branch{Ref: "dev"}}}})
	newer := suite(6, fake.CheckRun{Name: "aayudha/inspect-reply", Conclusion: "failure"}, fake.CheckRun{Name: "aayudha/merge-gate", Conclusion: "success"},
		fake.CheckRun{Name: "aayudha/inspect", Conclusion: "success"}, fake.CheckRun{Name: "aayudha/pack", Conclusion: "success"})
	g.CheckRuns[head] = append(slices.Clone(newer), suite(5, fake.CheckRun{Name: "aayudha/merge-gate", Conclusion: "success", StartedAt: gateStarted, CompletedAt: gateCompleted},
		fake.CheckRun{Name: "aayudha/inspect", Conclusion: "failure"}, fake.CheckRun{Name: "aayudha/pack", Conclusion: "success"})...)
	g.Mu.Unlock()
	if code, out := ship(prN); code != ExitBlocked || !strings.Contains(out, fmt.Sprintf("aayudha/inspect is failure in run %d, whose pack lands, on %s; the merge gate must be green", packID, short(head))) {
		t.Fatalf("ship with the pack's run red and a newer run's gate checks green: %d %s", code, out)
	}
	// So does a gate check of the pack's run that is not done, as when a
	// re-run of that run starts between the two reads.
	g.Mu.Lock()
	running := suite(5, fake.CheckRun{Name: "aayudha/merge-gate", Conclusion: "success", StartedAt: gateStarted, CompletedAt: gateCompleted}, fake.CheckRun{Name: "aayudha/inspect", Conclusion: "success"},
		fake.CheckRun{Name: "aayudha/pack", Conclusion: "success"})
	running[2].Status, running[2].Conclusion = "in_progress", ""
	g.CheckRuns[head] = append(slices.Clone(newer), running...)
	g.Mu.Unlock()
	if code, out := ship(prN); code != ExitBlocked || !strings.Contains(out, fmt.Sprintf("aayudha/pack is in_progress in run %d, whose pack lands", packID)) {
		t.Fatalf("ship with the pack's run re-running: %d %s", code, out)
	}
	g.Mu.Lock()
	g.CheckRuns[head] = append(slices.Clone(newer), suite(5, fake.CheckRun{Name: "aayudha/merge-gate", Conclusion: "success", StartedAt: gateStarted, CompletedAt: gateCompleted},
		fake.CheckRun{Name: "aayudha/inspect", Conclusion: "success"}, fake.CheckRun{Name: "aayudha/pack", Conclusion: "success"})...)
	g.Mu.Unlock()
	if code, out := ship(prN); code != ExitBlocked || strings.Contains(out, "the merge gate must be green") || !strings.Contains(out, "no human pass") {
		t.Fatalf("ship with the pack's run green and a newer run failed outside the gate checks: %d %s", code, out)
	}
	// One gate run, two attempts: attempt 1 failed its floors, and a re-run of
	// its failed jobs passed. Each attempt's gate job uploaded a pack to the
	// run, and GitHub lists the newest first. The pack that lands is the one
	// made while the aayudha/merge-gate check that ship reads ran: attempt 2's,
	// in any list order. A pack from outside that attempt never lands.
	gateRun(t, g, dir, prN, head, []proof.Check{{Floor: "coverage", Result: proof.Fail, Value: 50, Threshold: 90, Tool: "aayudha", Version: "1"}})
	g.Mu.Lock()
	first := g.Runs[len(g.Runs)-1].ID
	g.Mu.Unlock()
	gateRun(t, g, dir, prN, head, coverageHolds)
	g.Mu.Lock()
	both := g.Runs[len(g.Runs)-1].ID
	g.Runs = slices.DeleteFunc(g.Runs, func(r fake.WorkflowRun) bool { return r.ID == first })
	attempt1, attempt2 := g.Artifacts[first][0], g.Artifacts[both][0]
	attempt1.Created = gateUploaded.Add(-time.Hour) // attempt 1's gate job ran an hour before attempt 2's
	g.Artifacts[both] = []fake.ArtifactZip{attempt2, attempt1}
	g.Mu.Unlock()
	if code, out := ship(prN); code != ExitBlocked || strings.Contains(out, "floors do not hold") || !strings.Contains(out, "no human pass") {
		t.Fatalf("ship with attempt 2's green pack listed before attempt 1's red one: %d %s", code, out)
	}
	g.Mu.Lock()
	g.Artifacts[both] = []fake.ArtifactZip{attempt1, attempt2}
	g.Mu.Unlock()
	if code, out := ship(prN); code != ExitBlocked || strings.Contains(out, "floors do not hold") || !strings.Contains(out, "no human pass") {
		t.Fatalf("ship with attempt 1's red pack listed before attempt 2's green one: %d %s", code, out)
	}
	// The checks ship reads are attempt 1's, but the newest pack is from a
	// later attempt: the two are from two attempts, so ship refuses.
	g.Mu.Lock()
	early := gateUploaded.Add(-2 * time.Hour).Format(time.RFC3339)
	for i, r := range g.CheckRuns[head] {
		if r.Name == "aayudha/merge-gate" {
			g.CheckRuns[head][i].StartedAt, g.CheckRuns[head][i].CompletedAt = early, gateUploaded.Add(-90*time.Minute).Format(time.RFC3339)
		}
	}
	g.Mu.Unlock()
	if code, out := ship(prN); code != ExitBlocked || !strings.Contains(out, fmt.Sprintf("artifact %d of run %d was made at %s, outside the attempt whose aayudha/merge-gate check ran %s", attempt2.ID, both, gateUploaded.Format(time.RFC3339), early)) {
		t.Fatalf("ship with a pack from another attempt than the checks: %d %s", code, out)
	}
	// A re-run whose gate job uploaded no pack leaves only attempt 1's, made
	// before the gate check that ship reads started; a pack with no creation
	// time proves no attempt. Each refuses.
	g.Mu.Lock()
	for i, r := range g.CheckRuns[head] {
		if r.Name == "aayudha/merge-gate" {
			g.CheckRuns[head][i].StartedAt, g.CheckRuns[head][i].CompletedAt = gateStarted, gateCompleted
		}
	}
	g.Artifacts[both] = []fake.ArtifactZip{attempt1}
	g.Mu.Unlock()
	if code, out := ship(prN); code != ExitBlocked || !strings.Contains(out, fmt.Sprintf("artifact %d of run %d was made at %s, outside the attempt whose aayudha/merge-gate check ran %s to %s", attempt1.ID, both, attempt1.Created.Format(time.RFC3339), gateStarted, gateCompleted)) {
		t.Fatalf("ship with only an earlier attempt's pack: %d %s", code, out)
	}
	g.Mu.Lock()
	attempt2.Created = time.Time{}
	g.Artifacts[both] = []fake.ArtifactZip{attempt2}
	for i, r := range g.CheckRuns[head] {
		if r.Name == "aayudha/merge-gate" { // nor does a gate check with no times
			g.CheckRuns[head][i].StartedAt, g.CheckRuns[head][i].CompletedAt = "", ""
		}
	}
	g.Mu.Unlock()
	if code, out := ship(prN); code != ExitBlocked || !strings.Contains(out, fmt.Sprintf("artifact %d of run %d has no creation time, outside the attempt", attempt2.ID, both)) {
		t.Fatalf("ship with a pack that has no creation time: %d %s", code, out)
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
	// The second read of the gate's runs is gateGreen's, for the suites its checks must come from.
	g.Mu.Lock()
	g.Fail, g.FailAfter = "GET /repos/o/r/actions/workflows/aayudha-merge-gate.yml/runs", 1
	g.Mu.Unlock()
	if code, out := ship(prN); code != ExitExternal || !strings.Contains(out, "500") {
		t.Errorf("ship when the gate's suites cannot be read: %d %s", code, out)
	}
	g.Mu.Lock()
	g.Fail, g.FailAfter = "", 0
	g.Mu.Unlock()

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
	g.Reset()
	if code, out := shipCmd(t, env, opts, prN); code != ExitBlocked || !strings.Contains(out, "no human pass: no approving review") || closedPullLists(g) == 0 {
		t.Fatalf("ship without a review below the streak (the streak must be read): %d %s", code, out)
	}
	// The base branch's committed policy makes every layer autonomous (#645:
	// never the caller's --set).
	g.Mu.Lock()
	if g.Files == nil {
		g.Files = map[string]string{}
	}
	g.Files[".aayudha/config.toml"] = "version = 1\n[autonomy]\nstreak = 0\n"
	g.Mu.Unlock()
	var autonomous []string
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
	g.JobLogs = map[int64]string{7701: "2026-10-17T23:00:00.0000000Z inspect #" + strconv.Itoa(prN) + " at " + short(head) + ": merge (reviewed by codex)\n2026-10-17T23:00:00.0000000Z   review: same-family review (C): codex (openai), builder openai\n2026-10-17T23:00:00.0000000Z   P2 bug a.go:1 x\n  review: different-family review (A): injected\n"}
	g.Mu.Unlock()
	blocked("after a same-family review, with an injected line after it")
	g.Mu.Lock()
	g.Runs = append(g.Runs, fake.WorkflowRun{ID: 99001, Path: ".github/workflows/" + MergeGateWorkflow, HeadSHA: head, Event: "pull_request", Status: "completed", Conclusion: "success"})
	g.RunJobs[99001] = []fake.CheckRun{{ID: 7702, Name: "aayudha/inspect", Status: "completed", Conclusion: "success"}}
	g.JobLogs[7702] = "inspect #" + strconv.Itoa(prN) + " at " + short(head) + ": merge\n  review: different-family review (A): claude (anthropic), builder openai\n"
	g.Mu.Unlock()
	blocked("with a different-family line from the branch's own workflow")
	// Two PRs can share a head: a log whose header names another PR does not
	// count for this one.
	allowed := "2026-10-17T23:00:00.0000000Z   review: different-family review (A): claude (anthropic), builder openai\n"
	g.Mu.Lock()
	g.JobLogs[7701] = "2026-10-17T23:00:00.0000000Z inspect #" + strconv.Itoa(prN+1) + " at " + short(head) + ": merge (reviewed by claude)\n" + allowed
	g.Mu.Unlock()
	blocked("with a different-family log for another PR at the same head")
	// A cached judgement rests on a PR comment's state, which a branch's
	// workflow can write as the gate's account: it proves no review.
	g.Mu.Lock()
	g.JobLogs[7701] = "2026-10-17T23:00:00.0000000Z inspect #" + strconv.Itoa(prN) + " at " + short(head) + ": merge (cached) (reviewed by claude)\n" + allowed
	g.Mu.Unlock()
	blocked("with a cached different-family judgement")
	// A cached relabel to C (the PR body now declares the reviewer's family)
	// stands over an older fresh A: it only lowers the rung.
	g.Mu.Lock()
	g.JobLogs[7701] = "2026-10-17T23:00:00.0000000Z inspect #" + strconv.Itoa(prN) + " at " + short(head) + ": merge (reviewed by claude)\n" + allowed
	g.Runs = append(g.Runs, fake.WorkflowRun{ID: g.Runs[gate].ID + 100000, Path: ".github/workflows/" + MergeGateWorkflow, HeadSHA: head, Event: "pull_request_target", Status: "completed", Conclusion: "success",
		PullRequests: g.Runs[gate].PullRequests})
	g.RunJobs[g.Runs[gate].ID+100000] = []fake.CheckRun{{ID: 7703, Name: "aayudha/inspect", Status: "completed", Conclusion: "success"}}
	g.JobLogs[7703] = "2026-10-17T23:00:00.0000000Z inspect #" + strconv.Itoa(prN) + " at " + short(head) + ": merge (cached) (reviewed by claude)\n  review: same-family review (C): claude (anthropic), builder anthropic\n"
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

	// The footprint is also the base layers' over the PR's files: a file in
	// a never layer that the pack's footprint left out still needs a person.
	g.Mu.Lock()
	files := g.Pulls[prN].Files
	g.Pulls[prN].Files = append(slices.Clone(files), "internal/ledger/x.go")
	g.Mu.Unlock()
	if code, out := shipCmd(t, env, opts, prN, autonomous...); code != ExitBlocked || !strings.Contains(out, "the footprint touches never layer(s) L07") {
		t.Fatalf("ship with a never-layer file the pack left out: %d %s", code, out)
	}
	// Bug #680: with [autonomy] proof off, the narrow set still needs a
	// person, the streak whatever: a change to a listed path (criterion 1) or
	// to a guarded table of the committed config (criterion 2) waits for one.
	narrow := func(criterion int, want string) {
		t.Helper()
		code, out := shipCmd(t, env, opts, prN, autonomous...)
		g.Mu.Lock()
		merged := g.Pulls[prN].Merged
		g.Mu.Unlock()
		if code != ExitBlocked || !strings.Contains(out, want) || strings.Contains(out, "human pass: autonomous") || merged ||
			strings.Contains(git(t, bare, "ls-tree", "-r", "--name-only", "ledger"), "proof/2/"+head+"/authoritative/ship.json") {
			t.Fatalf("#680 criterion %d: %d %s", criterion, code, out)
		}
	}
	g.Mu.Lock()
	g.Pulls[prN].Files = append(slices.Clone(files), ".aayudha/config.local.json")
	g.Mu.Unlock()
	narrow(1, "no merge on the streak: it touches .aayudha/config.local.json, in the narrow set that always needs a person ([autonomy] always_person: .aayudha/config.local.json)")
	g.Mu.Lock()
	base := g.Pulls[prN].Base.SHA
	g.RefFiles = map[string]map[string]string{base: maps.Clone(g.Files), head: maps.Clone(g.Files)}
	g.RefFiles[head][".aayudha/config.toml"] = g.Files[".aayudha/config.toml"] + "correction_days = 3\n"
	g.Pulls[prN].Files = append(slices.Clone(files), ".aayudha/config.toml")
	g.Mu.Unlock()
	narrow(2, "no merge on the streak: it changes [autonomy] in .aayudha/config.toml")
	// A file the narrow set must read that cannot be read fails closed.
	g.Mu.Lock()
	g.RefFiles = nil
	g.Pulls[prN].Files = append(slices.Clone(files), ".github/workflows/x.yml")
	g.Fail, g.FailAfter = "GET /repos/o/r/contents/.github/workflows/x.yml", 0
	g.Mu.Unlock()
	if code, out := shipCmd(t, env, opts, prN, autonomous...); code != ExitExternal || strings.Contains(out, "human pass: autonomous") {
		t.Fatalf("ship when a narrow-set file cannot be read: %d %s", code, out)
	}
	// Criterion 4: with proof on, the same listed file gets the reason it got
	// before the fix (dev 70299db); TestShipOnProof's clean PR still merges
	// on proof.
	g.Mu.Lock()
	g.Fail = ""
	conf := g.Files[".aayudha/config.toml"]
	g.Files[".aayudha/config.toml"] = conf + "proof = true\n"
	g.Pulls[prN].Files = append(slices.Clone(files), ".aayudha/config.local.json")
	g.Mu.Unlock()
	narrow(4, "no merge on proof: it touches .aayudha/config.local.json, in the narrow set that always needs a person ([autonomy] always_person: .aayudha/config.local.json)")
	g.Mu.Lock()
	g.Files[".aayudha/config.toml"] = conf
	g.Mu.Unlock()
	// Criterion 3: a PR that touches no narrow-set path still merges on the
	// streak, as before.
	g.Mu.Lock()
	g.Pulls[prN].Files = files
	g.Mu.Unlock()
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
	// No person passed it, so it waits for the audit after merge, as a merge on proof does.
	var entry AuditEntry
	_ = json.Unmarshal([]byte(git(t, bare, "show", "ledger:"+AuditLedgerPath(prN))), &entry)
	if entry.PR != prN || entry.Pass != "autonomous" || !contains(g.LabelNames(prN), AuditLabel) {
		t.Fatalf("an autonomous merge was not sent to the audit: %+v, labels %v", entry, g.LabelNames(prN))
	}
	if c := comment(g, 2, "<!-- aayudha:lease-released"); !strings.Contains(c, "shipped in #"+strconv.Itoa(prN)) {
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
	// The checkout (the PR's own config) turns proof off; the base
	// revision's config is the policy, so a tighter checkout is ignored (and
	// a looser one refused, below).
	_ = os.MkdirAll(filepath.Join(dir, ".aayudha"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, ".aayudha", "config.toml"), []byte("version = 1\n[autonomy]\nproof = false\n"), 0o644)
	// docs was skipped by its path filter, and release-snapshot is neutral:
	// GitHub counts both as passing.
	g.Mu.Lock()
	gate := slices.IndexFunc(g.Runs, func(r fake.WorkflowRun) bool { return r.HeadSHA == head && r.Event == "pull_request_target" })
	g.RunJobs = map[int64][]fake.CheckRun{g.Runs[gate].ID: {{ID: 7701, Name: "aayudha/inspect", Status: "completed", Conclusion: "success"}}}
	g.JobLogs = map[int64]string{7701: "2026-10-17T23:00:00.0000000Z inspect #" + strconv.Itoa(prN) + " at " + short(head) + ": merge (reviewed by codex)\n2026-10-17T23:00:00.0000000Z   review: same-family review (C): codex (openai), builder openai\n"}
	// One rule and two check runs per page: ship reads every page. build must come from GitHub
	// Actions (app 15368): another app's green build does not pass it, and
	// its newest run counts, not a stale failure before a rerun.
	g.RequiredChecks, g.RulesPerPage, g.CheckRunsPerPage = []string{"build@15368", "lint", "docs", "release-snapshot", "security", "protected-paths"}, 1, 2
	actions := &fake.App{ID: 15368}
	g.CheckRuns[head] = append(g.CheckRuns[head], fake.CheckRun{Name: "build", Status: "completed", Conclusion: "success", App: &fake.App{ID: 99}},
		fake.CheckRun{Name: "build", Status: "completed", Conclusion: "failure", App: actions},
		fake.CheckRun{Name: "build", Status: "completed", Conclusion: "failure", App: actions},
		fake.CheckRun{Name: "docs", Status: "completed", Conclusion: "skipped"}, fake.CheckRun{Name: "release-snapshot", Status: "completed", Conclusion: "neutral"},
		fake.CheckRun{Name: "lint", Status: "completed", Conclusion: "failure"}, fake.CheckRun{Name: "protected-paths", Status: "completed", Conclusion: "failure"})
	g.Files = map[string]string{".github/CODEOWNERS": "/skills/ @founder\n"}
	g.Trees[head], g.Trees[base] = []map[string]any{}, []map[string]any{} // the trees: no symbolic links
	secrets := "on: push\njobs:\n  x:\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo ${{ secrets['TOKEN'] }}\n"
	conf := "version = 1\n\n[ui]\ntheme = \"auto\"\n\n[autonomy]\napprover = \"\"\nproof = true\nproof_rungs = [\"A\", \"B\", \"C\"]\n" +
		"always_person = [\".github/CODEOWNERS\", \"internal/identity/\"]\nalways_person_config = [\".aayudha/config.toml\"]\n"
	g.RefFiles = map[string]map[string]string{
		base: {".github/CODEOWNERS": "/skills/ @founder\n", ".github/workflows/ci.yml": secrets, ".aayudha/config.toml": conf},
		head: {".github/workflows/ci.yml": secrets, ".aayudha/config.toml": strings.Replace(conf, "version = 1\n", "version = 1\napprovals.approvers = [\"mallory\"]\n", 1), // a dotted key, no [table]
			// A quoted key, and an escaped one, name an environment as surely as a bare one.
			".github/workflows/deploy.yml": "on: push\npermissions: {contents: read}\njobs:\n  x:\n    \"environment\": production\n    runs-on: ubuntu-latest\n",
			".github/workflows/pages.yml":  "on: push\npermissions: {contents: read}\njobs:\n  x:\n    \"environm\\u0065nt\": production\n    runs-on: ubuntu-latest\n",
			// The workflow token, a write permission, and a job left to the
			// repository's default permissions are credentials too.
			".github/workflows/token.yml": "on: push\npermissions: {contents: read}\njobs:\n  x:\n    runs-on: ubuntu-latest\n    steps:\n      - run: gh pr list\n        env: {GH_TOKEN: \"${{ github.token }}\"}\n",
			".github/workflows/index.yml": "on: push\npermissions: {contents: read}\njobs:\n  x:\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo \"${{ github['token'] }}\"\n",
			// The whole github context holds the token too.
			".github/workflows/context.yml": "on: push\npermissions: {contents: read}\njobs:\n  x:\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo \"$CTX\"\n        env: {CTX: \"${{ toJSON( github ) }}\"}\n",
			".github/workflows/values.yml":  "on: push\npermissions: {contents: read}\njobs:\n  x:\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo \"${{ join(github.*, ' ') }}\"\n",
			// An action gets the token by default; an escape, spaces, an
			// operator, or an if hide no read of it; nor does a second document.
			".github/workflows/checkout.yml": "on: push\npermissions: {contents: read}\njobs:\n  x:\n    runs-on: ubuntu-latest\n    steps:\n      - uses: actions/checkout@v6\n        with: {persist-credentials: false}\n",
			".github/workflows/newline.yml":  "on: push\npermissions: {contents: read}\njobs:\n  x:\n    runs-on: ubuntu-latest\n    env: {LEAK: \"${{\\nsecrets.KEY}}\"}\n    steps:\n      - run: echo hi\n",
			".github/workflows/spaced.yml":   "on: push\npermissions: {contents: read}\njobs:\n  x:\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo \"${{ github . token }}\"\n",
			".github/workflows/or.yml":       "on: push\npermissions: {contents: read}\njobs:\n  x:\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo \"${{ toJSON(github || null) }}\"\n",
			".github/workflows/if.yml":       "on: push\npermissions: {contents: read}\njobs:\n  x:\n    if: github.token != ''\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo hi\n",
			".github/workflows/twodocs.yml":  "on: push\npermissions: {contents: read}\njobs:\n  x:\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo hi\n---\njobs: {}\n",
			".github/workflows/write.yml":    "on: push\njobs:\n  x:\n    permissions: {contents: write}\n    runs-on: ubuntu-latest\n",
			".github/workflows/default.yml":  "on: push\njobs:\n  x:\n    permissions: {contents: read}\n    runs-on: ubuntu-latest\n  y:\n    runs-on: ubuntu-latest\n",
			// A read-only workflow holds none: it merges on proof.
			".github/workflows/lint.yml": "on: pull_request\npermissions: {contents: read}\njobs:\n  lint:\n    if: github.event_name == 'pull_request'\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo \"${{ github.sha }}\"\n"},
	}
	g.Pulls[prN].Files = []string{"empty.txt"}
	g.Mu.Unlock()
	refused := func(want string, args ...string) {
		t.Helper()
		if code, out := shipCmd(t, env, opts, prN, args...); code != ExitBlocked || !strings.Contains(out, want) {
			t.Fatalf("ship, want %q: %d %s", want, code, out)
		}
		g.Mu.Lock()
		defer g.Mu.Unlock()
		if g.Pulls[prN].Merged {
			t.Fatalf("merged although %q", want)
		}
	}
	// streakAll is refused with the base branch's committed policy making
	// every layer autonomous (#645: never the caller's --set), at the base
	// and the head alike, so the PR changes no [autonomy].
	streakAll := func(want string) {
		t.Helper()
		g.Mu.Lock()
		b, h := g.RefFiles[base][".aayudha/config.toml"], g.RefFiles[head][".aayudha/config.toml"]
		for ref, text := range map[string]string{base: b, head: h} {
			g.RefFiles[ref][".aayudha/config.toml"] = strings.Replace(text, "[autonomy]\n", "[autonomy]\nstreak = 0\nnever = []\n", 1)
		}
		g.Mu.Unlock()
		refused(want)
		g.Mu.Lock()
		g.RefFiles[base][".aayudha/config.toml"], g.RefFiles[head][".aayudha/config.toml"] = b, h
		g.Mu.Unlock()
	}
	refused("would loosen it: [autonomy] always_person drops .github/CODEOWNERS, internal/identity/", "--set", "autonomy.always_person=")
	refused("required check build is failure on " + short(head) + "; never merge with a red required check")
	g.Mu.Lock()
	g.CheckRuns[head][slices.IndexFunc(g.CheckRuns[head], func(r fake.CheckRun) bool { return r.App == actions })].Conclusion = "success" // the rerun
	g.Mu.Unlock()
	refused("required check lint is failure on " + short(head) + "; never merge with a red required check")
	g.Mu.Lock()
	g.CheckRuns[head][len(g.CheckRuns[head])-2].Conclusion = "success" // lint
	// A legacy commit status of the same name must pass too; a status alone
	// can satisfy a required check.
	g.Statuses = map[string][]fake.Status{head: {{Context: "lint", State: "failure"}, {Context: "lint", State: "success"}}}
	g.Mu.Unlock()
	refused("required check lint is failure on " + short(head))
	g.Mu.Lock()
	g.Statuses[head] = []fake.Status{{Context: "lint", State: "success"}, {Context: "security", State: "pending"}}
	g.Mu.Unlock()
	refused("required check security is pending on " + short(head))
	g.Mu.Lock()
	g.Statuses[head] = []fake.Status{{Context: "lint", State: "success"}, {Context: "gone", State: "failure"}}
	g.Mu.Unlock()
	refused("required check security is missing on " + short(head))
	g.Mu.Lock()
	g.CheckRuns[head] = append([]fake.CheckRun{{Name: "security", Status: "in_progress"}}, g.CheckRuns[head]...)
	g.Mu.Unlock()
	refused("required check security is in_progress on " + short(head))
	g.Mu.Lock()
	g.CheckRuns[head] = g.CheckRuns[head][1:]
	g.Statuses[head] = []fake.Status{{Context: "lint", State: "success"}, {Context: "security", State: "success"}} // security by a status alone
	g.Pulls[prN].Files = []string{"empty.txt", "internal/identity/review.go"}
	g.Mu.Unlock()
	refused("no merge on proof: it touches internal/identity/review.go, in the narrow set that always needs a person ([autonomy] always_person: internal/identity/)")
	// Nor does the streak pass it, even at rung A with the caller's
	// --set making every layer autonomous.
	g.Mu.Lock()
	logC := g.JobLogs[7701]
	g.JobLogs[7701] = strings.Replace(logC, "same-family review (C): codex (openai), builder openai", "different-family review (A): claude (anthropic), builder openai", 1)
	g.Mu.Unlock()
	streakAll("no merge on proof: it touches internal/identity/review.go")
	g.Mu.Lock()
	g.JobLogs[7701] = logC
	g.RefFiles[base][".aayudha/config.toml"] = strings.Replace(conf, `"internal/identity/"]`, `"internal/identity/", ".github/workflows/protected-paths*.yml"]`, 1)
	g.Pulls[prN].Files = []string{"empty.txt", ".github/workflows/protected-paths-review.yml"}
	g.Mu.Unlock()
	refused("no merge on proof: it touches .github/workflows/protected-paths-review.yml, in the narrow set that always needs a person ([autonomy] always_person: .github/workflows/protected-paths*.yml)")
	// This repository's own policy: a dependency change alone needs a person,
	// since it can change the code of everything in the narrow set.
	ours, err := os.ReadFile(filepath.Join("..", "..", ".aayudha", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"go.sum", "go.mod"} {
		g.Mu.Lock()
		g.RefFiles[base][".aayudha/config.toml"] = string(ours)
		g.Pulls[prN].Files = []string{f}
		g.Mu.Unlock()
		refused("no merge on proof: it touches " + f + ", in the narrow set that always needs a person ([autonomy] always_person: " + f + ")")
	}
	g.Mu.Lock()
	g.RefFiles[base][".aayudha/config.toml"] = conf
	g.Pulls[prN].Files, g.Pulls[prN].Unlisted = []string{"empty.txt"}, 1 // past the listing's 3,000 files
	g.Mu.Unlock()
	refused("GitHub lists 1 of the PR's 2 changed files, so ship cannot judge its footprint")
	g.Mu.Lock()
	g.Pulls[prN].Compared, g.Pulls[prN].Unlisted = []string{}, 0 // past the comparison's 300 files only
	g.Mu.Unlock()
	refused(fmt.Sprintf("no merge on proof: GitHub's comparison of %s...%s lists 0 of the PR's 1 changed files (it lists at most 300), so the narrow set cannot be judged", short(base), short(head)))
	g.Mu.Lock()
	g.Pulls[prN].Compared = nil
	g.Mu.Unlock()
	// The narrow set is judged on the captured base and head, not the live
	// PR's file listing, which a push during ship could swap and restore.
	g.Mu.Lock()
	g.Pulls[prN].Unlisted = 0
	g.Pulls[prN].Compared = []string{"internal/identity/gate.go"}
	g.Mu.Unlock()
	refused("no merge on proof: it touches internal/identity/gate.go, in the narrow set that always needs a person ([autonomy] always_person: internal/identity/)")
	// It fails closed: a base config that does not load, a workflow that does
	// not parse, and a read GitHub refuses each stop it.
	g.Mu.Lock()
	g.Pulls[prN].Compared = nil
	g.RefFiles[base][".aayudha/config.toml"] = conf + "unknown_key = 1\n"
	g.Mu.Unlock()
	refused("dev's config at " + short(base) + " does not load, so ship cannot judge the pass")
	g.Mu.Lock()
	g.RefFiles[base][".aayudha/config.toml"] = conf
	g.RefFiles[head][".github/workflows/broken.yml"] = "on: [push\njobs: {\n"
	g.Mu.Unlock()
	for _, fail := range []string{"GET /repos/o/r/contents/.aayudha/config.toml", "GET /repos/o/r/rules/branches/dev"} {
		g.Mu.Lock()
		g.Fail = fail
		g.Mu.Unlock()
		if code, out := shipCmd(t, env, opts, prN); code != ExitExternal || slices.Contains(g.LabelNames(2), "status:merged") {
			t.Fatalf("ship when %s fails: %d %s", fail, code, out)
		}
	}
	for _, wf := range []string{".github/workflows/deploy.yml", ".github/workflows/pages.yml", ".github/workflows/broken.yml",
		".github/workflows/token.yml", ".github/workflows/index.yml", ".github/workflows/context.yml", ".github/workflows/values.yml",
		".github/workflows/checkout.yml", ".github/workflows/newline.yml", ".github/workflows/spaced.yml", ".github/workflows/or.yml",
		".github/workflows/if.yml", ".github/workflows/twodocs.yml",
		".github/workflows/write.yml", ".github/workflows/default.yml"} {
		g.Mu.Lock()
		g.Pulls[prN].Files, g.Pulls[prN].Unlisted = []string{"empty.txt", wf}, 0
		g.Mu.Unlock()
		refused("no merge on proof: it touches " + wf + ", a workflow that holds a credential (a secret, the workflow token, an action it calls, an environment, or a write permission)")
	}
	g.Mu.Lock()
	g.Pulls[prN].Files = []string{"empty.txt", ".github/workflows/ci.yml"}
	g.Mu.Unlock()
	refused("no merge on proof: it touches .github/workflows/ci.yml, a workflow that holds a credential (a secret, the workflow token, an action it calls, an environment, or a write permission)")
	g.Mu.Lock()
	g.Pulls[prN].Files = []string{"empty.txt", ".aayudha/config.toml"}
	g.Mu.Unlock()
	refused("no merge on proof: it changes [approvals] in .aayudha/config.toml, which decides who may approve or bypass, how a PR is verified, reviewed and proven, or where a credential is sent")
	g.Mu.Lock()
	g.RefFiles[base][".aayudha/config.toml"] = strings.Replace(conf, `"C"]`, `]`, 1)
	g.RefFiles[head][".aayudha/config.toml"] = strings.Replace(g.RefFiles[base][".aayudha/config.toml"], `"auto"`, `"saffron"`, 1) // another table: no person needed
	g.Pulls[prN].Files = []string{"empty.txt", ".aayudha/config.toml", "skills/build/SKILL.md"}
	g.Mu.Unlock()
	refused("no merge on proof: the review rung at the head is C, not one of A, B ([autonomy] proof_rungs)")
	// The integration branch is the base revision's too, not the checkout's.
	g.Mu.Lock()
	g.RefFiles[base][".aayudha/config.toml"] = conf + "\n[branches]\ndev = \"trunk\"\n"
	g.RefFiles[head][".aayudha/config.toml"] = strings.Replace(g.RefFiles[base][".aayudha/config.toml"], `"auto"`, `"saffron"`, 1)
	g.Mu.Unlock()
	refused("no merge on proof: it merges into dev, not the integration branch trunk")
	g.Mu.Lock()
	g.RefFiles[base][".aayudha/config.toml"] = conf
	g.RefFiles[head][".aayudha/config.toml"] = strings.Replace(conf, `"auto"`, `"saffron"`, 1)
	g.Pulls[prN].Files = []string{"empty.txt", ".aayudha/config.toml"}
	g.Mu.Unlock()
	// protected-paths is red only for the CODEOWNERS paths a PR touches.
	refused("no merge on proof: required check protected-paths is failure, and the gate, rerun on the base's CODEOWNERS, finds no CODEOWNERS path without an approval that would explain it")
	// Each rule that requires it counts: a second one, from another app
	// whose run is green, does not hide the first one's failure.
	g.Mu.Lock()
	g.RequiredChecks = append(g.RequiredChecks, "protected-paths@77")
	g.CheckRuns[head] = append(g.CheckRuns[head], fake.CheckRun{Name: "protected-paths", Status: "completed", Conclusion: "success", App: &fake.App{ID: 77}})
	g.Mu.Unlock()
	refused("no merge on proof: required check protected-paths is failure, and the gate, rerun on the base's CODEOWNERS, finds no CODEOWNERS path without an approval that would explain it")
	g.Mu.Lock()
	g.RequiredChecks = g.RequiredChecks[:len(g.RequiredChecks)-1]
	g.CheckRuns[head] = g.CheckRuns[head][:len(g.CheckRuns[head])-1]
	g.Mu.Unlock()
	// Nor does the streak's pass merge past it: only a person's does.
	g.Mu.Lock()
	logC = g.JobLogs[7701]
	g.JobLogs[7701] = strings.Replace(logC, "same-family review (C): codex (openai), builder openai", "different-family review (A): claude (anthropic), builder openai", 1)
	g.Mu.Unlock()
	streakAll("required check protected-paths is failure, and only a person's pass merges past it")
	g.Mu.Lock()
	g.JobLogs[7701] = logC
	g.Mu.Unlock()
	// It is excused only for the gate's own reason, rerun on the base's
	// CODEOWNERS: not when the gate fails closed on a pattern it cannot
	// judge.
	g.Mu.Lock()
	g.Pulls[prN].Files = []string{"empty.txt", ".aayudha/config.toml", "skills/build/SKILL.md"}
	g.RefFiles[base][".github/CODEOWNERS"] = "/skills/ @founder\n*.go @founder\n"
	g.Mu.Unlock()
	refused("no merge on proof: required check protected-paths is failure, and the gate, rerun on the base's CODEOWNERS, finds no CODEOWNERS path without an approval that would explain it")
	g.Mu.Lock()
	g.RefFiles[base][".github/CODEOWNERS"] = "/skills/ @founder\n"
	g.CheckRuns[head] = append([]fake.CheckRun{{Name: "protected-paths", Status: "queued"}}, g.CheckRuns[head]...)
	g.Mu.Unlock()
	// Only its failure is excused: a run not yet done has judged nothing.
	refused("no merge on proof: required check protected-paths is queued; only its failure for the CODEOWNERS paths a PR touches is accepted")
	g.Mu.Lock()
	g.CheckRuns[head] = g.CheckRuns[head][1:]
	// A cached C, which a workflow on a branch could have written as the
	// gate's account, grants no proof.
	g.JobLogs[7701] = strings.Replace(g.JobLogs[7701], ": merge (reviewed by codex)", ": merge (cached) (reviewed by codex)", 1)
	g.Mu.Unlock()
	refused("no merge on proof: the review rung at the head is cached C, not one of A, B, C ([autonomy] proof_rungs)")
	// The gate's runs must be GitHub's for this PR into its base: a helper PR
	// into a branch with an edited gate cannot lend its A, and a pack run
	// listed for no PR proves nothing.
	g.Mu.Lock()
	g.JobLogs[7701] = strings.Replace(g.JobLogs[7701], ": merge (cached) (reviewed by codex)", ": merge (reviewed by codex)", 1)
	g.RefFiles[base][".aayudha/config.toml"] = strings.Replace(conf, `proof_rungs = ["A", "B", "C"]`, `proof_rungs = ["A"]`, 1)
	g.RefFiles[head][".aayudha/config.toml"] = strings.Replace(g.RefFiles[base][".aayudha/config.toml"], `"auto"`, `"saffron"`, 1)
	g.Runs = append(g.Runs, fake.WorkflowRun{ID: 9999, Path: ".github/workflows/" + MergeGateWorkflow, HeadSHA: head, Event: "pull_request_target", Status: "completed", Conclusion: "success",
		PullRequests: []fake.RunPR{{Number: 99, Base: fake.Branch{Ref: "evil"}}}})
	g.RunJobs[9999] = []fake.CheckRun{{ID: 7799, Name: "aayudha/inspect", Status: "completed", Conclusion: "success"}}
	g.JobLogs[7799] = strings.Replace(g.JobLogs[7701], "same-family review (C): codex (openai), builder openai", "different-family review (A): claude (anthropic), builder openai", 1)
	g.Mu.Unlock()
	refused("no merge on proof: the review rung at the head is C, not one of A ([autonomy] proof_rungs)")
	// GitHub lists every open PR on the run's head, so the helper's run can
	// list this PR too: a run listed for several PRs is no one's.
	g.Mu.Lock()
	g.Runs[len(g.Runs)-1].PullRequests = append(g.Runs[len(g.Runs)-1].PullRequests, fake.RunPR{Number: prN, Base: fake.Branch{Ref: "dev"}})
	g.Mu.Unlock()
	refused("no merge on proof: the review rung at the head is C, not one of A ([autonomy] proof_rungs)")
	// Nor does the streak's pass take the helper's A.
	g.Mu.Lock()
	pp := slices.IndexFunc(g.CheckRuns[head], func(r fake.CheckRun) bool { return r.Name == "protected-paths" })
	g.CheckRuns[head][pp].Conclusion = "success"
	g.Mu.Unlock()
	streakAll("did not record a different-family review")
	g.Mu.Lock()
	g.CheckRuns[head][pp].Conclusion = "failure"
	g.Mu.Unlock()
	g.Mu.Lock()
	g.Runs = g.Runs[:len(g.Runs)-1]
	g.RefFiles[base][".aayudha/config.toml"] = conf
	g.RefFiles[head][".aayudha/config.toml"] = strings.Replace(conf, `"auto"`, `"saffron"`, 1)
	prs := g.Runs[gate].PullRequests
	// The PR's own head branch and repository, as a fork's run on a branch of
	// the same name would show them: only a merged PR's run counts without a
	// listed pull request (Bug #1159).
	g.Runs[gate].PullRequests, g.Runs[gate].HeadBranch, g.Runs[gate].HeadRepository = nil, g.Pulls[prN].Head.Ref, &fake.HeadRepo{FullName: "o/r"}
	g.Mu.Unlock()
	// A gate run GitHub lists for no PR is not this open PR's gate: its checks are missing before the rung is read.
	refused("aayudha/merge-gate is missing on " + short(head))
	g.Mu.Lock()
	g.Runs[gate].PullRequests, g.Runs[gate].HeadBranch, g.Runs[gate].HeadRepository = prs, "", nil
	g.Mu.Unlock()
	g.Mu.Lock()
	g.Pulls[prN].Files = append(g.Pulls[prN].Files, ".github/workflows/lint.yml")
	// The policy is the base branch's tip now, not the PR's base revision: a
	// narrow set that tightened on dev since the PR was cut applies.
	tip := strings.Repeat("c", 40)
	g.Refs["heads/dev"] = tip
	g.RefFiles[tip] = map[string]string{".github/CODEOWNERS": "/skills/ @founder\n", ".aayudha/config.toml": strings.Replace(conf, `"internal/identity/"]`, `"internal/identity/", "skills/"]`, 1)}
	g.Mu.Unlock()
	refused("no merge on proof: it touches skills/build/SKILL.md, in the narrow set that always needs a person ([autonomy] always_person: skills/)")
	// And dev moving while ship runs stops the merge: GitHub's merge guards
	// only the head, so ship checks the tip it judged against again.
	g.Mu.Lock()
	g.RefFiles[tip][".aayudha/config.toml"] = conf
	reads := 0
	g.Hook = func(method, p string) {
		if method == "GET" && strings.HasSuffix(p, "/git/ref/heads/dev") {
			if reads++; reads > 1 {
				g.Refs["heads/dev"] = strings.Repeat("d", 40)
			}
		}
	}
	g.Mu.Unlock()
	refused(fmt.Sprintf("dev moved from %s to %s while ship judged the PR; run ship again", short(tip), short(strings.Repeat("d", 40))))
	g.Mu.Lock()
	g.Hook = nil
	delete(g.Refs, "heads/dev")
	// An action's manifest runs with the caller's token.
	files := g.Pulls[prN].Files
	g.Pulls[prN].Files = append(slices.Clone(files), ".github/actions/probe/action.yml")
	g.Mu.Unlock()
	refused("no merge on proof: it touches .github/actions/probe/action.yml, an action's manifest, whose steps run with the token of the workflow that calls it")
	// The loader splits a quoted key on its dots and matches a table whatever
	// its case, and [inspect] decides how a PR is reviewed.
	for _, change := range []string{"\"approvals.approvers\" = [\"mallory\"]\n", "[APPROVALS]\napprovers = [\"mallory\"]\n", "[inspect]\nskip = [\"internal/\"]\n"} {
		g.Mu.Lock()
		g.Pulls[prN].Files = files
		g.RefFiles[head][".aayudha/config.toml"] = strings.Replace(conf, "version = 1\n", "version = 1\n"+change, 1)
		if strings.HasPrefix(change, "[") {
			g.RefFiles[head][".aayudha/config.toml"] = conf + change
		}
		g.Mu.Unlock()
		want := "approvals"
		if strings.Contains(change, "inspect") {
			want = "inspect"
		}
		refused("no merge on proof: it changes [" + want + "] in .aayudha/config.toml")
	}
	// [delegate] names where a credential is sent.
	g.Mu.Lock()
	g.RefFiles[head][".aayudha/config.toml"] = conf + "[delegate]\ncursor_api = \"https://attacker\"\n"
	g.Mu.Unlock()
	refused("no merge on proof: it changes [delegate] in .aayudha/config.toml")
	// A base whose policy is split across config.json is not judged at all.
	g.Mu.Lock()
	g.RefFiles[head][".aayudha/config.toml"] = strings.Replace(conf, `"auto"`, `"saffron"`, 1)
	g.RefFiles[base][".aayudha/config.json"] = `{"autonomy": {"always_person": ["skills/"]}}`
	g.Mu.Unlock()
	refused("dev holds .aayudha/config.json at " + short(base) + ", and ship judges the pass only from .aayudha/config.toml")
	g.Mu.Lock()
	delete(g.RefFiles[base], ".aayudha/config.json")
	g.Mu.Unlock()
	g.Mu.Lock()
	g.RefFiles[head][".aayudha/config.toml"] = strings.Replace(conf, `"auto"`, `"saffron"`, 1)
	// A symbolic link can point a policy file elsewhere.
	g.Trees[head] = []map[string]any{{"path": "skills/build/SKILL.md", "mode": "120000", "type": "blob", "sha": "l1"}}
	g.Mu.Unlock()
	refused("no merge on proof: it changes skills/build/SKILL.md, a symbolic link")
	// Nor removing one, or making it a file: the base's tree counts too.
	g.Mu.Lock()
	g.Trees[base], g.Trees[head] = g.Trees[head], []map[string]any{}
	g.Mu.Unlock()
	refused("no merge on proof: it changes skills/build/SKILL.md, a symbolic link")
	g.Mu.Lock()
	g.Trees[base] = []map[string]any{}
	g.Mu.Unlock()
	// A PR whose base was changed may have been reviewed against another branch.
	g.Mu.Lock()
	g.Trees[head] = []map[string]any{}
	g.Pulls[prN].Timeline = []string{"labeled", "base_ref_changed"}
	g.Mu.Unlock()
	refused("no merge on proof: its base branch was changed after it was opened")
	// Classic branch protection's required checks bind as the rules' do.
	g.Mu.Lock()
	g.Pulls[prN].Timeline = nil
	g.ClassicChecks = []string{"unit"}
	g.Mu.Unlock()
	refused("required check unit is missing on " + short(head))
	g.Mu.Lock()
	g.ClassicChecks = nil
	g.Mu.Unlock()
	// A required check passes only if the newest run in every suite passes:
	// a skipped run from a dispatch cannot stand in for a failing one, and a
	// failing status fails a check pinned to an app too.
	g.Mu.Lock()
	g.Pulls[prN].Timeline = nil
	runsBefore := slices.Clone(g.CheckRuns[head])
	g.CheckRuns[head] = append([]fake.CheckRun{{Name: "build", Status: "completed", Conclusion: "skipped", App: actions, Suite: &fake.Suite{ID: 2}}}, g.CheckRuns[head]...)
	for i := range g.CheckRuns[head] {
		if g.CheckRuns[head][i].Name == "build" && g.CheckRuns[head][i].Suite == nil && g.CheckRuns[head][i].App == actions {
			g.CheckRuns[head][i].Suite = &fake.Suite{ID: 1}
			g.CheckRuns[head][i].Conclusion = "failure"
			break
		}
	}
	g.Mu.Unlock()
	refused("required check build is failure on " + short(head))
	g.Mu.Lock()
	g.CheckRuns[head] = runsBefore
	statusesBefore := g.Statuses[head]
	g.Statuses[head] = append([]fake.Status{{Context: "build", State: "failure"}}, statusesBefore...)
	g.Mu.Unlock()
	refused("required check build is failure on " + short(head))
	g.Mu.Lock()
	g.Statuses[head] = statusesBefore
	// A log naming another head is no review of this one.
	logC = g.JobLogs[7701]
	g.JobLogs[7701] = strings.Replace(logC, " at "+short(head)+":", " at deadbeef:", 1)
	g.Mu.Unlock()
	refused("no merge on proof: the review rung at the head is none recorded")
	// The PR retargeted while ship ran is not the PR it judged.
	g.Mu.Lock()
	g.JobLogs[7701] = logC
	g.Refs["heads/dev"] = base
	g.Hook = func(method, p string) {
		if method == "GET" && strings.HasSuffix(p, "/git/ref/heads/dev") {
			g.Pulls[prN].Base.Ref = "evil"
		}
	}
	g.Mu.Unlock()
	refused("the PR changed while ship judged it (now open, head " + short(head) + ", into evil)")
	g.Mu.Lock()
	g.Hook = nil
	g.Pulls[prN].Base.Ref = "dev"
	delete(g.Refs, "heads/dev")
	// A newer cached run on the unchanged head pushes the fresh judgement to
	// the next page: ship reads every page for it.
	g.RunsPerPage = 1
	g.Runs = append(g.Runs, fake.WorkflowRun{ID: g.Runs[gate].ID + 5000, Path: g.Runs[gate].Path, HeadSHA: head, Event: "pull_request_target", Status: "completed", Conclusion: "success", PullRequests: g.Runs[gate].PullRequests})
	g.Artifacts[g.Runs[gate].ID+5000] = g.Artifacts[g.Runs[gate].ID]
	g.RunJobs[g.Runs[gate].ID+5000] = []fake.CheckRun{{ID: 7788, Name: "aayudha/inspect", Status: "completed", Conclusion: "success"}}
	g.JobLogs[7788] = strings.Replace(strings.Replace(g.JobLogs[7701], ": merge (reviewed by codex)", ": merge (cached) (reviewed by codex)", 1), "same-family review (C): codex (openai)", "different-family review (A): claude (anthropic)", 1)
	g.Mu.Unlock()

	// A build-ready loop PR whose builder stopped it after the loop listed it
	// is not shipped while its story is status:blocked.
	var st BuildState
	_ = json.Unmarshal([]byte(git(t, bare, "show", "ledger:"+BuildLedgerPath(2))), &st)
	st.Loop = BuildLoop
	b, _ := json.Marshal(st)
	putLedger(t, bare, BuildLedgerPath(2), string(b))
	g.Mu.Lock()
	g.Issues[2].Labels = append(g.Issues[2].Labels, fake.Label{Name: "status:blocked"})
	g.Mu.Unlock()
	refused("story #2 is status:blocked, so the build-ready loop's PR waits for a person")
	// An edit of the body to name another story does not step around the
	// stop: the branch still names story #2.
	g.Mu.Lock()
	body := g.Pulls[prN].Body
	g.Pulls[prN].Body = strings.Replace(body, "Refs #2", "Refs #3", 1)
	g.Mu.Unlock()
	refused("the PR body refers to story #3, but its branch story/2-")
	g.Mu.Lock()
	g.Pulls[prN].Body = body
	g.Mu.Unlock()
	// The story cannot be read: ship does not guess that it is not blocked.
	g.Mu.Lock()
	g.Fail, g.FailAfter = "GET /repos/o/r/issues/2", 0
	g.Mu.Unlock()
	if code, out := shipCmd(t, env, opts, prN); code != ExitExternal || !strings.Contains(out, "500") {
		t.Fatalf("ship when the loop's story cannot be read: %d %s", code, out)
	}
	g.Mu.Lock()
	g.Fail = ""
	g.Mu.Unlock()
	// A torn build state does not lift the stop: the story's own branch is
	// still held by its label, and without the label ship still stops.
	putLedger(t, bare, BuildLedgerPath(2), "{")
	refused("story #2 is status:blocked, so its PR waits for a person")
	// Nor does a state that names no loop, or another branch.
	putLedger(t, bare, BuildLedgerPath(2), `{"story":2,"branch":"story/2-elsewhere"}`)
	refused("story #2 is status:blocked, so its PR waits for a person")
	putLedger(t, bare, BuildLedgerPath(2), "{")
	g.Mu.Lock()
	g.Issues[2].Labels = slices.DeleteFunc(g.Issues[2].Labels, func(l fake.Label) bool { return l.Name == "status:blocked" })
	g.Mu.Unlock()
	if code, out := shipCmd(t, env, opts, prN); code != ExitExternal {
		t.Fatalf("ship when the story's build state is unreadable: %d %s", code, out)
	}
	putLedger(t, bare, BuildLedgerPath(2), string(b))
	// A stop given while ship judges the PR, after its first read of the
	// label, still holds it: ship reads the label again at the merge.
	g.Mu.Lock()
	read := false
	g.Hook = func(method, p string) {
		switch {
		case method == "GET" && p == "/repos/o/r/issues/2":
			read = true
		case read && method == "GET" && strings.HasSuffix(p, "/git/ref/heads/dev"):
			g.Issues[2].Labels, read = append(g.Issues[2].Labels, fake.Label{Name: "status:blocked"}), false
		}
	}
	g.Mu.Unlock()
	refused("story #2 is status:blocked, so the build-ready loop's PR waits for a person")
	g.Mu.Lock()
	g.Hook = nil
	g.Issues[2].Labels = slices.DeleteFunc(g.Issues[2].Labels, func(l fake.Label) bool { return l.Name == "status:blocked" })
	// Story #994: everything else would merge it on proof, but #2 is an early build.
	g.Issues[2].Labels = append(g.Issues[2].Labels, fake.Label{Name: "early-build"})
	g.Mu.Unlock()
	refused("ship #" + strconv.Itoa(prN) + ": #2 is an early build: its PR never merges into dev (E1)")
	g.Mu.Lock()
	g.Issues[2].Labels = slices.DeleteFunc(g.Issues[2].Labels, func(l fake.Label) bool { return l.Name == "early-build" })
	g.Mu.Unlock()

	// Bug #1031 criterion 5: a concurrency group cancels one run of a
	// required check and a later run on the same head passes (PR #1085). The
	// latest completed run counts; a cancelled run alone, a later failure, and
	// a later run still in progress decide as before.
	cancelled := fake.CheckRun{ID: 121613879571, Name: "build", Status: "completed", Conclusion: "cancelled", App: actions, Suite: &fake.Suite{ID: 50},
		StartedAt: "2026-10-27T13:55:40Z", CompletedAt: "2026-10-27T13:55:54Z"}
	later := func(status, conclusion string) fake.CheckRun {
		return fake.CheckRun{ID: 121613890736, Name: "build", Status: status, Conclusion: conclusion, App: actions, Suite: &fake.Suite{ID: 51}, StartedAt: "2026-10-27T13:55:59Z"}
	}
	// Suites 49, 50, and 51 are three runs of ci.yml (49 started before the
	// cancelled one); suite 52 is pages.yml, whose job is also called build and
	// replaces nothing of ci's, though its failure still counts.
	earlier := fake.CheckRun{ID: 121613870000, Name: "build", Status: "completed", Conclusion: "success", App: actions, Suite: &fake.Suite{ID: 49}, StartedAt: "2026-10-27T13:50:00Z"}
	pages := func(conclusion string) fake.CheckRun {
		return fake.CheckRun{ID: 121613890999, Name: "build", Status: "completed", Conclusion: conclusion, App: actions, Suite: &fake.Suite{ID: 52}, StartedAt: "2026-10-27T13:56:10Z"}
	}
	g.Mu.Lock()
	runsBefore = slices.Clone(g.CheckRuns[head])
	g.Runs = append(g.Runs,
		fake.WorkflowRun{ID: 7049, Path: ".github/workflows/ci.yml", HeadSHA: head, Event: "pull_request", Status: "completed", Conclusion: "success", CheckSuiteID: 49},
		fake.WorkflowRun{ID: 7050, Path: ".github/workflows/ci.yml", HeadSHA: head, Event: "pull_request", Status: "completed", Conclusion: "cancelled", CheckSuiteID: 50},
		fake.WorkflowRun{ID: 7051, Path: ".github/workflows/ci.yml", HeadSHA: head, Event: "pull_request", Status: "completed", Conclusion: "success", CheckSuiteID: 51},
		fake.WorkflowRun{ID: 7052, Path: ".github/workflows/pages.yml", HeadSHA: head, Event: "pull_request", Status: "completed", Conclusion: "success", CheckSuiteID: 52})
	g.Mu.Unlock()
	for _, c := range []struct {
		runs []fake.CheckRun
		want string
	}{
		{[]fake.CheckRun{cancelled}, "cancelled"},                                                // no replacement
		{[]fake.CheckRun{later("completed", "failure"), cancelled}, "failure"},                   // replaced by a failure
		{[]fake.CheckRun{later("in_progress", ""), cancelled}, "in_progress"},                    // a replacement still running replaces nothing yet
		{[]fake.CheckRun{later("completed", "cancelled"), cancelled}, "cancelled"},               // cancelled again
		{[]fake.CheckRun{cancelled, earlier}, "cancelled"},                                       // a run of the workflow from before the cancel
		{[]fake.CheckRun{pages("success"), cancelled}, "cancelled"},                              // another workflow's job of the same name
		{[]fake.CheckRun{pages("failure"), later("completed", "success"), cancelled}, "failure"}, // replaced, but another suite failed
	} {
		g.Mu.Lock()
		g.CheckRuns[head] = append(slices.Clone(c.runs), runsBefore...)
		g.Mu.Unlock()
		refused("required check build is " + c.want + " on " + short(head))
	}
	// A cancelled check that no rule requires, or a cancelled build from an app
	// the rule does not pin, needs no Actions read (a token may read checks
	// but not Actions): the required check decides.
	g.Mu.Lock()
	g.CheckRuns[head] = append([]fake.CheckRun{later("completed", "failure"),
		{ID: 121613890998, Name: "screen-reader", Status: "completed", Conclusion: "cancelled", App: actions, Suite: &fake.Suite{ID: 53}},
		{ID: 121613890997, Name: "build", Status: "completed", Conclusion: "cancelled", App: &fake.App{ID: 999}, Suite: &fake.Suite{ID: 54}}}, runsBefore...)
	g.Fail, g.FailAfter = "GET /repos/o/r/actions/runs", 0
	g.Mu.Unlock()
	refused("required check build is failure on " + short(head))
	// The replacement started in the same second as the cancelled run (GitHub's
	// times have whole seconds): the higher run ID is the later run.
	same := later("completed", "success")
	same.StartedAt = cancelled.StartedAt
	g.Mu.Lock()
	g.Fail = ""
	g.CheckRuns[head] = append([]fake.CheckRun{same, cancelled}, runsBefore...)
	g.Mu.Unlock()

	// The audit's label goes on before the merge: when it cannot, nothing
	// merges, so no merge on proof escapes the look after merge.
	g.Mu.Lock()
	g.Fail, g.FailAfter = fmt.Sprintf("POST /repos/o/r/issues/%d/labels", prN), 0
	g.Mu.Unlock()
	code, out := shipCmd(t, env, opts, prN)
	g.Mu.Lock()
	early := g.Pulls[prN].Merged
	g.Mu.Unlock()
	if code != ExitExternal || early || !strings.Contains(out, "not merged: the aayu:audit label") {
		t.Fatalf("ship with the audit label failing: %d merged %t %s", code, early, out)
	}
	code, out = shipCmd(t, env, opts, prN)
	if code != 0 || !strings.Contains(out, "human pass: proof by founder") {
		t.Fatalf("ship on proof: %d %s", code, out)
	}
	var sr ShipRecord
	_ = json.Unmarshal([]byte(git(t, bare, "show", "ledger:proof/2/"+head+"/authoritative/ship.json")), &sr)
	if sr.HumanPass.Kind != "proof" || sr.Proof == nil || sr.Proof.Rung != inspect.RungC || !slices.Equal(sr.Proof.Checks, []string{"build", "lint", "docs", "release-snapshot", "security"}) ||
		!slices.Equal(sr.Proof.Protected, []string{"skills/build/SKILL.md"}) || sr.PlanReview != nil || !slices.Equal(sr.Footprint, []string{"L03", "L07", "L09"}) {
		t.Fatalf("ship record: %+v %+v footprint %v", sr.HumanPass, sr.Proof, sr.Footprint)
	}
	g.Mu.Lock()
	body, merged := g.Issues[2].Body, g.Pulls[prN].Merged
	g.Mu.Unlock()
	if !merged || !strings.Contains(body, "- Human pass: not needed, merged on proof (Decision #593): required checks build, lint, docs, release-snapshot, security green") ||
		!strings.Contains(body, "CODEOWNERS paths skills/build/SKILL.md merged with no code owner's review") {
		t.Fatalf("Complete section (merged %t):\n%s", merged, body)
	}

	// Story #618: the merge on proof waits for a person's look after merge,
	// as an audit entry on the ledger and the audit label on the PR.
	var entry AuditEntry
	_ = json.Unmarshal([]byte(git(t, bare, "show", "ledger:"+AuditLedgerPath(prN))), &entry)
	if entry.PR != prN || entry.Story != 2 || entry.Head != head || entry.Merge == "" || entry.Pass != "proof" || !slices.Equal(entry.Footprint, []string{"L03", "L07", "L09"}) {
		t.Fatalf("audit entry: %+v", entry)
	}
	if !slices.Contains(g.LabelNames(prN), AuditLabel) {
		t.Fatalf("PR labels: %v", g.LabelNames(prN))
	}
	samples := func() []Sample {
		t.Helper()
		var out []Sample
		for _, l := range strings.Split(strings.TrimSpace(git(t, bare, "show", "ledger:"+SamplesLedgerPath)), "\n") {
			var s Sample
			if json.Unmarshal([]byte(l), &s) == nil {
				out = append(out, s)
			}
		}
		return out
	}
	// inbox finds the audit card in the read-only list, skips the cards
	// before it, and answers it with keys.
	opts.DevicePoll = time.Millisecond
	opts.IsTerminal = func(any) bool { return true }
	inbox := func(env map[string]string, keys string) string {
		t.Helper()
		var l inboxList
		if code, out, errOut := run(t, env, opts, "inbox", "--list", "--json", "--repo", "o/r"); code != 0 || json.Unmarshal([]byte(out), &l) != nil {
			t.Fatalf("inbox --list: %d %s %s", code, out, errOut)
		}
		at := slices.IndexFunc(l.Items, func(it InboxItem) bool { return it.Kind == InboxAudit && it.Number == prN })
		if at < 0 || at >= l.NeedsYou || l.Items[at].Options[0].Label != "looks right (a clean sample)" || !strings.HasPrefix(l.Items[at].Options[1].Label, "correction: ") {
			t.Fatalf("no audit card for #%d: %+v", prN, l.Items)
		}
		_, out, errOut := runIn(t, env, opts, strings.Repeat("s ", at)+keys+" q", "inbox", "--repo", "o/r")
		if !strings.Contains(out, fmt.Sprintf("Merged unreviewed, to audit #%d", prN)) {
			t.Fatalf("inbox lacks the audit card:\n%s%s", out, errOut)
		}
		return out
	}
	// An agent lane records, labels, and opens nothing.
	agent := maps.Clone(env)
	maps.Copy(agent, hostShells["amp"])
	g.Mu.Lock()
	g.Device = &fake.DeviceFlow{ClientID: DefaultDeviceClientID, Approver: "founder", Outcome: "deny"}
	pulls := len(g.Pulls)
	g.Mu.Unlock()
	if code, out, errOut := runIn(t, agent, opts, "a a a a a a", "inbox", "--repo", "o/r"); code == 0 || strings.Contains(out, "done: sample") {
		t.Fatalf("an agent lane audited: %d %s %s", code, out, errOut)
	}
	g.Mu.Lock()
	n := len(g.Pulls)
	g.Device = &fake.DeviceFlow{ClientID: DefaultDeviceClientID, Approver: "founder", Outcome: "approve"}
	g.Mu.Unlock()
	if git(t, bare, "show", "ledger:"+AuditLedgerPath(prN)) == "" || n != pulls || slices.Contains(g.LabelNames(prN), autonomy.LabelCorrection) {
		t.Fatalf("an agent lane wrote: audit entry gone, %d PRs (was %d), labels %v", n, pulls, g.LabelNames(prN))
	}
	// A labelled merge into another branch still waits for its look.
	g.Mu.Lock()
	g.Pulls[prN].Base.Ref = "rolling"
	g.Mu.Unlock()
	var listed inboxList
	if code, out, errOut := run(t, env, opts, "inbox", "--list", "--json", "--repo", "o/r"); code != 0 || json.Unmarshal([]byte(out), &listed) != nil ||
		!slices.ContainsFunc(listed.Items, func(it InboxItem) bool { return it.Kind == InboxAudit && it.Number == prN }) {
		t.Fatalf("no audit card for a merge into rolling: %d %s %s", code, out, errOut)
	}
	// Nor does removing the label hide it: the ledger entry still waits
	// (#664). The card's title is safe to print.
	g.Mu.Lock()
	g.Pulls[prN].Base.Ref = "dev"
	labels := g.Issues[prN].Labels
	g.Issues[prN].Labels = slices.DeleteFunc(slices.Clone(labels), func(l fake.Label) bool { return l.Name == AuditLabel })
	title := g.Pulls[prN].Title
	g.Pulls[prN].Title = "safe\u202etxt.exe"
	g.Mu.Unlock()
	listed = inboxList{}
	if code, out, errOut := run(t, env, opts, "inbox", "--list", "--json", "--repo", "o/r"); code != 0 || json.Unmarshal([]byte(out), &listed) != nil ||
		!slices.ContainsFunc(listed.Items, func(it InboxItem) bool {
			return it.Kind == InboxAudit && it.Number == prN && !strings.ContainsRune(it.Title, '\u202e')
		}) {
		t.Fatalf("no clean audit card once its label was removed: %d %s %s", code, out, errOut)
	}
	g.Mu.Lock()
	g.Issues[prN].Labels, g.Pulls[prN].Title = labels, title
	g.Mu.Unlock()
	// Looks right: a clean sample, and the entry and the label are gone. A
	// look whose label removal failed is finished by the next look.
	g.Mu.Lock()
	g.Fail, g.FailAfter = fmt.Sprintf("DELETE /repos/o/r/issues/%d/labels/%s", prN, AuditLabel), 0
	g.Mu.Unlock()
	if out := inbox(env, "a"); len(samples()) != 1 || !slices.Contains(g.LabelNames(prN), AuditLabel) {
		t.Fatalf("looks right, label removal failing: %+v, labels %v\n%s", samples(), g.LabelNames(prN), out)
	}
	g.Mu.Lock()
	g.Fail, g.FailAfter = fmt.Sprintf("DELETE /repos/o/r/issues/%d/labels/%s", prN, AuditLabel), 0
	g.Mu.Unlock()
	if out := inbox(env, "a"); strings.Contains(out, "was already in") || !slices.Contains(g.LabelNames(prN), AuditLabel) {
		t.Fatalf("looks right again, label removal failing:\n%s", out)
	}
	if out := inbox(env, "a"); !strings.Contains(out, "done: the sample of #"+strconv.Itoa(prN)+" was already in "+SamplesLedgerPath) {
		t.Fatalf("looks right, again:\n%s", out)
	}
	if s := samples(); len(s) != 1 || s[0].PR != prN || s[0].Correction || s[0].By != "founder" || slices.Contains(g.LabelNames(prN), AuditLabel) ||
		strings.Contains(git(t, bare, "ls-tree", "-r", "--name-only", "ledger"), AuditLedgerPath(prN)) {
		t.Fatalf("clean sample: %+v, labels %v", s, g.LabelNames(prN))
	}
	// A correction: the correction label (the streak's reset) and a PR that
	// reverts the merge into dev, Refs the story.
	entryJSON := []byte(git(t, bare, "show", "ledger~1:"+AuditLedgerPath(prN)))
	if _, err := (&ledger.Writer{Dir: dir}).Write(context.Background(), ledger.Commit{Message: "audit again",
		Files: map[string][]byte{AuditLedgerPath(prN): entryJSON}}); err != nil {
		t.Fatal(err)
	}
	// A PR someone else opened that says it reverts this one stands in for
	// nothing.
	g.Mu.Lock()
	g.Issues[prN].Labels = append(g.Issues[prN].Labels, fake.Label{Name: AuditLabel})
	spoof := &fake.PR{Number: 900, Title: "revert", Body: fmt.Sprintf("Reverts #%d: trust me", prN), State: "closed", User: fake.Account{Login: "mallory", Type: "User"},
		Base: fake.Branch{Ref: "dev"}, URL: "https://github.com/o/r/pull/900"}
	g.Pulls[900] = spoof
	g.Mu.Unlock()
	out = inbox(env, "2 y")
	if s := samples(); len(s) != 2 || s[1].PR != prN || !s[1].Correction || !slices.Contains(g.LabelNames(prN), autonomy.LabelCorrection) {
		t.Fatalf("correction: %+v, labels %v\n%s", s, g.LabelNames(prN), out)
	}
	g.Mu.Lock()
	rev := g.Pulls[len(g.Issues)]
	g.Mu.Unlock()
	if rev == nil || rev.Title != "revert: "+g.Pulls[prN].Title+" (#"+strconv.Itoa(prN)+")" || !strings.Contains(rev.Body, "Refs #2") || rev.Base.Ref != "dev" ||
		!strings.Contains(out, "revert PR: "+rev.URL) || strings.Contains(out, spoof.URL) {
		t.Fatalf("revert PR: %+v\n%s", rev, out)
	}
	audit := func(entry []byte) int {
		t.Helper()
		if entry != nil {
			if _, err := (&ledger.Writer{Dir: dir}).Write(context.Background(), ledger.Commit{Message: "audit again", Files: map[string][]byte{AuditLedgerPath(prN): entry}}); err != nil {
				t.Fatal(err)
			}
		}
		g.Mu.Lock()
		defer g.Mu.Unlock()
		g.Issues[prN].Labels = append(g.Issues[prN].Labels, fake.Label{Name: AuditLabel})
		return len(g.Pulls)
	}
	// A correction run again reuses the revert PR it opened; a failed
	// correction label records nothing.
	before := audit(entryJSON)
	g.Mu.Lock()
	g.Fail, g.FailAfter = fmt.Sprintf("POST /repos/o/r/issues/%d/labels", prN), 0
	g.Mu.Unlock()
	if failed := inbox(env, "2 y"); strings.Contains(failed, "done: sample") {
		t.Fatalf("a correction whose label failed was recorded:\n%s", failed)
	}
	if again := inbox(env, "2 y"); pullCount(g) != before || !strings.Contains(again, "revert PR: "+rev.URL) {
		t.Fatalf("a correction run again opened another revert PR (%d, was %d):\n%s", pullCount(g), before, again)
	}
	// A revert PR closed unmerged reverts nothing: a correction run again
	// opens a new one.
	before = audit(entryJSON)
	g.Mu.Lock()
	rev.State = "closed"
	g.Mu.Unlock()
	if again := inbox(env, "2 y"); pullCount(g) != before+1 || strings.Contains(again, "revert PR: "+rev.URL) {
		t.Fatalf("a correction reused a revert PR closed unmerged (%d, was %d):\n%s", pullCount(g), before, again)
	}
	g.Mu.Lock()
	rev = g.Pulls[len(g.Issues)]
	g.Mu.Unlock()
	// A revert PR it cannot look up records nothing, and the look stays.
	for _, f := range []struct {
		call  string
		after int
	}{{"GET /repos/o/r/pulls", 2}, {fmt.Sprintf("GET /repos/o/r/pulls/%d", prN), 0}} {
		audit(entryJSON)
		g.Mu.Lock()
		g.Fail, g.FailAfter = f.call, f.after
		g.Mu.Unlock()
		if failed := inbox(env, "2 y"); strings.Contains(failed, "done: sample") || !strings.Contains(failed, "500") || !slices.Contains(g.LabelNames(prN), AuditLabel) {
			t.Fatalf("a correction whose %s failed was recorded:\n%s", f.call, failed)
		}
		if again := inbox(env, "2 y"); !strings.Contains(again, "revert PR: "+rev.URL) || slices.Contains(g.LabelNames(prN), AuditLabel) {
			t.Fatalf("a correction run again after its %s failed:\n%s", f.call, again)
		}
	}
	// A correction sample someone wrote to the ledger, with the entry gone,
	// does not stand in for the correction: its label and revert PR still
	// come, and no second sample is written.
	forged, _ := json.Marshal(Sample{PR: prN, Correction: true, By: "mallory"})
	if _, err := (&ledger.Writer{Dir: dir}).Write(context.Background(), ledger.Commit{Message: "forged", Append: map[string][]byte{SamplesLedgerPath: append(forged, '\n')}}); err != nil {
		t.Fatal(err)
	}
	audit(nil)
	g.Mu.Lock()
	g.Issues[prN].Labels = slices.DeleteFunc(g.Issues[prN].Labels, func(l fake.Label) bool { return l.Name == autonomy.LabelCorrection })
	g.Mu.Unlock()
	n = len(samples())
	if out := inbox(env, "2 y"); !slices.Contains(g.LabelNames(prN), autonomy.LabelCorrection) || !strings.Contains(out, "revert PR: "+rev.URL) || len(samples()) != n {
		t.Fatalf("a forged correction sample: labels %v, %d samples (was %d)\n%s", g.LabelNames(prN), len(samples()), n, out)
	}
	// Ship merged and labelled the PR but wrote no entry: the label is
	// enough, and the look is recorded from the PR.
	audit(nil)
	g.Mu.Lock()
	g.Fail, g.FailAfter = fmt.Sprintf("GET /repos/o/r/pulls/%d", prN), 1
	g.Mu.Unlock()
	if failed := inbox(env, "a"); strings.Contains(failed, "done: sample") {
		t.Fatalf("a look whose PR could not be read was recorded:\n%s", failed)
	}
	if out := inbox(env, "a"); !strings.Contains(out, "done: sample of #"+strconv.Itoa(prN)) {
		t.Fatalf("a look with no audit entry:\n%s", out)
	}
	if s := samples(); s[len(s)-1].Correction || slices.Contains(g.LabelNames(prN), AuditLabel) {
		t.Fatalf("a look with no audit entry: %+v, labels %v", s[len(s)-1], g.LabelNames(prN))
	}
}

// Story #1192: the [autonomy] approver approves a PR's head once with `aayu
// inspect --approve` (one device code), the merge gate's inspect records a
// human review of that head, and ship merges on that approval with no second
// code; the ship record names the approval as the human pass. An approval of
// an older head, one by another login, a PR in the narrow set, and a human
// rung with no approval all refuse as before.
func TestShipOnApproval(t *testing.T) {
	g, env, opts, dir, bare, prN, head := thinFixture(t)
	gateRun(t, g, dir, prN, head, coverageHolds)
	base := strings.Repeat("b", 40)
	conf := "version = 1\n\n[autonomy]\napprover = \"lead\"\nproof = true\nproof_rungs = [\"A\", \"B\", \"C\"]\nalways_person = [\"policy/\"]\n"
	_ = os.MkdirAll(filepath.Join(dir, ".aayudha"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, ".aayudha", "config.toml"), []byte(conf), 0o644)
	g.Mu.Lock()
	gate := slices.IndexFunc(g.Runs, func(r fake.WorkflowRun) bool { return r.HeadSHA == head && r.Event == "pull_request_target" })
	g.RunJobs = map[int64][]fake.CheckRun{g.Runs[gate].ID: {{ID: 7701, Name: "aayudha/inspect", Status: "completed", Conclusion: "success"}}}
	g.JobLogs = map[int64]string{7701: "2026-10-31T10:00:00.0000000Z inspect #" + strconv.Itoa(prN) + " at " + short(head) + ": merge (reviewed by lead)\n2026-10-31T10:00:00.0000000Z   review: human review: lead\n"}
	g.Trees[head], g.Trees[base] = []map[string]any{}, []map[string]any{}
	g.RefFiles = map[string]map[string]string{base: {".aayudha/config.toml": conf}}
	g.Pulls[prN].Files = []string{"empty.txt"}
	g.Mu.Unlock()
	approval := func(at, by string) {
		t.Helper()
		b, _ := json.Marshal(headApproval{PR: prN, Head: at, By: by, At: "2026-10-31T09:50:00Z", Reason: "read the diff and the pack"})
		putLedger(t, bare, ApprovedPath, string(b)+"\n")
	}
	codes := func() int {
		g.Mu.Lock()
		defer g.Mu.Unlock()
		if g.Device == nil {
			return 0
		}
		return g.Device.Started
	}
	refused := func(want string) {
		t.Helper()
		code, out := shipCmd(t, env, opts, prN)
		g.Mu.Lock()
		merged := g.Pulls[prN].Merged
		g.Mu.Unlock()
		if code != ExitBlocked || merged || !strings.Contains(out, want) {
			t.Fatalf("ship, want %q: %d merged %t\n%s", want, code, merged, out)
		}
	}
	notOneOf := "no merge on proof: the review rung at the head is human, not one of A, B, C ([autonomy] proof_rungs)"
	// Criterion 5: a human rung with no approval on the ledger waits for a person.
	refused(notOneOf)
	refused("commented Ready to ship, so the PR waits in the `aayu inbox`")
	// Criterion 2: an approval of an older head covers nothing at this one.
	approval(strings.Repeat("a", 40), "lead")
	refused(notOneOf)
	// Criterion 3: an approval by a login that is not the [autonomy] approver,
	// even one recorded for this head, covers nothing.
	approval(head, "mallory")
	refused(notOneOf)

	// The approver approves this head through the device flow: one code.
	person := opts
	person.DevicePoll = time.Millisecond
	g.Mu.Lock()
	g.Viewer = "lead"
	g.Device = &fake.DeviceFlow{ClientID: DefaultDeviceClientID, Approver: "lead", Outcome: "approve"}
	g.Mu.Unlock()
	if code, _, errOut := run(t, env, person, "inspect", strconv.Itoa(prN), "--approve", "--reason", "read the diff and the pack", "--repo", "o/r"); code != 0 || codes() != 1 {
		t.Fatalf("inspect --approve: %d, %d device codes\n%s", code, codes(), errOut)
	}
	g.Mu.Lock()
	g.Viewer = "founder" // the agent ships under the person's GitHub login
	g.Mu.Unlock()
	// Criterion 4: in the narrow set the approval is no pass: the PR still
	// waits for --solo-bypass or the inbox.
	g.Mu.Lock()
	g.Pulls[prN].Files = []string{"empty.txt", "policy/approvers.md"}
	g.Mu.Unlock()
	refused("no merge on proof: it touches policy/approvers.md, in the narrow set that always needs a person ([autonomy] always_person: policy/)")
	refused("commented Ready to ship, so the PR waits in the `aayu inbox`")
	g.Mu.Lock()
	g.Pulls[prN].Files = []string{"empty.txt"}
	g.Mu.Unlock()
	// A paired Engineering Manager seat's human gives the human pass, not the
	// [autonomy] approver: the approver's record covers nothing then.
	paired := conf + "\n[seats.engineering-manager]\nmode = \"paired\"\nhuman = \"em-lead\"\n"
	_ = os.WriteFile(filepath.Join(dir, ".aayudha", "config.toml"), []byte(paired), 0o644)
	g.Mu.Lock()
	g.RefFiles[base][".aayudha/config.toml"] = paired
	g.Mu.Unlock()
	refused(notOneOf)
	// The seat's human may be the approver, written with or without "@".
	conf = strings.Replace(conf, `approver = "lead"`, `approver = "@lead"`, 1) + "\n[seats.engineering-manager]\nmode = \"paired\"\nhuman = \"Lead\"\n"
	_ = os.WriteFile(filepath.Join(dir, ".aayudha", "config.toml"), []byte(conf), 0o644)
	g.Mu.Lock()
	g.RefFiles[base][".aayudha/config.toml"] = conf
	g.Mu.Unlock()

	// Criterion 1: ship merges on the approval, and asks for no second code.
	code, out := shipCmd(t, env, opts, prN)
	if code != 0 || !strings.Contains(out, "human pass: approval by lead") || codes() != 1 {
		t.Fatalf("ship on the approval: %d, %d device codes\n%s", code, codes(), out)
	}
	var sr ShipRecord
	_ = json.Unmarshal([]byte(git(t, bare, "show", "ledger:proof/2/"+head+"/authoritative/ship.json")), &sr)
	if hp := sr.HumanPass; hp.Kind != "approval" || hp.Login != "lead" || hp.Commit != head ||
		!strings.Contains(hp.Note, "@lead approved `"+short(head)+"` with `aayu inspect "+strconv.Itoa(prN)+" --approve`") || !strings.Contains(hp.Note, "(`"+ApprovedPath+"`)") ||
		sr.Proof == nil || sr.Proof.Rung != inspect.RungHuman || sr.Proof.Approval == nil || sr.Proof.Approval.Head != head || sr.Proof.Approval.By != "lead" {
		t.Fatalf("ship record: %+v %+v", sr.HumanPass, sr.Proof)
	}
	g.Mu.Lock()
	body, merged := g.Issues[2].Body, g.Pulls[prN].Merged
	g.Mu.Unlock()
	if !merged || !strings.Contains(body, "- Human pass: @lead approved `"+short(head)+"`") {
		t.Fatalf("Complete section (merged %t):\n%s", merged, body)
	}
	// A person passed it, so it waits for no look after merge.
	if slices.Contains(g.LabelNames(prN), AuditLabel) {
		t.Fatalf("PR labels: %v", g.LabelNames(prN))
	}
}

// pullCount is how many pull requests g holds.
func pullCount(g *fake.GitHub) int {
	g.Mu.Lock()
	defer g.Mu.Unlock()
	return len(g.Pulls)
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
	// The seat is the base branch's committed policy (#645), which also
	// names the agent's second account a machine account.
	policy := "version = 1\n[identity]\nbot_logins = [\"agent-alt\"]\n[seats.engineering-manager]\nmode = \"paired\"\nhuman = \"em-lead\"\n"
	g.Mu.Lock()
	if g.Files == nil {
		g.Files = map[string]string{}
	}
	g.Files[".aayudha/config.toml"] = policy
	g.Mu.Unlock()
	var seat []string
	if code, out := shipCmd(t, env, opts, prN, seat...); code != ExitBlocked || !strings.Contains(out, "approval by reviewer is not the human pass: it must come from em-lead (seat)") {
		t.Fatalf("ship with a non-EM approval: %d %s", code, out)
	}
	// #645: an agent cannot pass its own second account by loosening the
	// policy for one run, through --set or a config in its checkout.
	g.Mu.Lock()
	g.Pulls[prN].Reviews = append(g.Pulls[prN].Reviews, review("agent-alt", "User", "APPROVED", head))
	g.Mu.Unlock()
	for _, c := range []struct {
		why, local string
		args       []string
		want       string
	}{
		{"--set the seat's human", "", []string{"--set", "seats.engineering-manager.human=agent-alt"}, "[seats.engineering-manager] is not paired with em-lead"},
		{"--set no bot logins", "", []string{"--set", "identity.bot_logins=", "--set", "seats.engineering-manager.mode=solo"}, "[identity] bot_logins drops agent-alt; [seats.engineering-manager] is not paired with em-lead"},
		{"a local config", "[seats.engineering-manager]\nmode = \"paired\"\nhuman = \"agent-alt\"\n", nil, "[seats.engineering-manager] is not paired with em-lead"},
	} {
		local := filepath.Join(dir, ".aayudha", "config.local.toml")
		_ = os.MkdirAll(filepath.Dir(local), 0o755)
		if c.local != "" {
			_ = os.WriteFile(local, []byte("version = 1\n"+c.local), 0o644)
		}
		code, out := shipCmd(t, env, opts, prN, c.args...)
		_ = os.Remove(local)
		g.Mu.Lock()
		merged := g.Pulls[prN].Merged
		g.Mu.Unlock()
		if code != ExitBlocked || merged || !strings.Contains(out, "the human pass is judged by dev's committed .aayudha/config.toml") || !strings.Contains(out, "would loosen it: "+c.want) {
			t.Fatalf("ship with %s: %d merged %t %s", c.why, code, merged, out)
		}
	}
	// #667: nor can the checkout name the bucket that ship sends the
	// person's media secret to.
	var code int
	var out string
	local := filepath.Join(dir, ".aayudha", "config.local.toml")
	_ = os.WriteFile(local, []byte("version = 1\n[proof]\nmedia = \"s3://evil/x?endpoint=http://127.0.0.1:9\"\nmedia_secret = \"github-token\"\n"), 0o644)
	code, out = shipCmd(t, env, opts, prN, seat...)
	_ = os.Remove(local)
	if code != ExitBlocked || contains(g.LabelNames(2), "status:merged") || !strings.Contains(out, "the proof media bucket, its secret, and its retention are read only from the user config and dev's committed .aayudha/config.toml") {
		t.Fatalf("ship with the checkout's media bucket: %d %s", code, out)
	}
	g.Mu.Lock()
	g.Pulls[prN].Reviews = g.Pulls[prN].Reviews[:1]
	g.Mu.Unlock()
	if code, out := shipCmd(t, env, opts, prN, append(seat, "--solo-bypass")...); code != ExitBlocked || !strings.Contains(out, "a second human (em-lead) holds a seat") {
		t.Fatalf("solo bypass with a paired seat: %d %s", code, out)
	}
	g.Mu.Lock()
	g.Files[".aayudha/config.toml"] = strings.Replace(policy, "[identity]\n", "[identity]\napp_id = 42\napp_slug = \"aayudha-bot\"\n", 1)
	g.Mu.Unlock()
	if code, out := shipCmd(t, env, opts, prN, append(seat, "--solo-bypass")...); code != ExitBlocked || !strings.Contains(out, "this repository has a bot identity") {
		t.Fatalf("solo bypass with a bot App: %d %s", code, out)
	}
	g.Mu.Lock()
	g.Files[".aayudha/config.toml"] = policy
	g.Mu.Unlock()

	g.Mu.Lock()
	g.Pulls[prN].Reviews = append(g.Pulls[prN].Reviews, review("em-lead", "User", "APPROVED", head))
	g.Mu.Unlock()
	// A base policy that cannot be read stops ship, before and just before
	// the merge alike.
	for _, f := range []struct {
		req   string
		after int
	}{{"GET /repos/o/r/git/ref/heads/dev", 0}, {"GET /repos/o/r/git/ref/heads/dev", 1}, {"GET /repos/o/r/contents/.aayudha/config.json", 0}, {"GET /repos/o/r/contents/LAYERS.md", 0},
		{"GET /repos/o/r/compare/" + g.Pulls[prN].Base.SHA + "..." + head, 0}} {
		g.Mu.Lock()
		g.Fail, g.FailAfter = f.req, f.after
		g.Mu.Unlock()
		if code, out := shipCmd(t, env, opts, prN, seat...); code != ExitExternal || contains(g.LabelNames(2), "status:merged") {
			t.Fatalf("ship when %s (after %d) fails: %d %s", f.req, f.after, code, out)
		}
	}
	g.Mu.Lock()
	g.Fail, g.FailAfter = "PUT /repos/o/r/pulls/"+strconv.Itoa(prN)+"/merge", 0
	g.Mu.Unlock()
	if code, out := shipCmd(t, env, opts, prN, seat...); code != ExitExternal || contains(g.LabelNames(2), "status:merged") {
		t.Fatalf("ship when the merge fails: %d %s %v", code, out, g.LabelNames(2))
	}

	// An earlier ship labelled the PR for the audit and then did not merge
	// it; a person's pass merges it, and the label goes.
	// A comparison cut short (past 300 files) falls back to the PR's own
	// listing, every page; a listing that is still short (past 3,000) stops
	// ship, since a file it leaves out could be in a never layer.
	g.Mu.Lock()
	g.Pulls[prN].Compared, g.Pulls[prN].Unlisted = []string{}, 1
	g.Mu.Unlock()
	if code, out := shipCmd(t, env, opts, prN, seat...); code != ExitBlocked || !strings.Contains(out, "so ship cannot judge its footprint") {
		t.Fatalf("ship with files GitHub does not list: %d %s", code, out)
	}
	// The stale label goes before the merge: one left on a reviewed merge
	// would have the inbox offer to revert it, so a removal that fails stops
	// ship unmerged.
	g.Mu.Lock()
	g.Fail, g.FailAfter = fmt.Sprintf("DELETE /repos/o/r/issues/%d/labels/%s", prN, AuditLabel), 0
	g.Issues[prN].Labels = append(g.Issues[prN].Labels, fake.Label{Name: AuditLabel})
	g.Pulls[prN].Unlisted = 0
	g.Mu.Unlock()
	code, out = shipCmd(t, env, opts, prN, seat...)
	g.Mu.Lock()
	merged := g.Pulls[prN].Merged
	g.Mu.Unlock()
	if code != ExitExternal || !strings.Contains(out, "not merged: a person passed it") || merged {
		t.Fatalf("ship when the stale audit label stays: %d %s", code, out)
	}
	g.Mu.Lock()
	g.Fail, g.FailAfter = "GET /repos/o/r/issues/2/comments", 0
	g.Mu.Unlock()
	code, out = shipCmd(t, env, opts, prN, seat...)
	if code != 0 || !strings.Contains(out, "human pass: review by em-lead") || !strings.Contains(out, "warning: the build lease on #2 was not released") {
		t.Fatalf("ship by the EM's human: %d %s", code, out)
	}
	var sr ShipRecord
	_ = json.Unmarshal([]byte(git(t, bare, "show", "ledger:proof/2/"+head+"/authoritative/ship.json")), &sr)
	if sr.HumanPass.Login != "em-lead" || sr.Pass.As != "seat" || strings.Join(sr.Pass.Approvers, ",") != "em-lead" {
		t.Fatalf("ship record: %+v", sr)
	}
	// A person's review passed it: no audit after merge (Story #618).
	if strings.Contains(git(t, bare, "ls-tree", "-r", "--name-only", "ledger"), "autonomy/audit/") || contains(g.LabelNames(prN), AuditLabel) {
		t.Fatalf("a reviewed merge was sent to the audit: labels %v", g.LabelNames(prN))
	}
	if !contains(g.LabelNames(2), LeaseLabel) {
		t.Fatalf("the unread lease was released: %v", g.LabelNames(2))
	}
}

// #645: a run's own config that loosens the base branch's pass policy is
// named, and only what a layer above the defaults set counts.
func TestShipLoosens(t *testing.T) {
	load := func(t *testing.T, src config.Sources) *config.Loaded {
		t.Helper()
		l, err := config.Load(src)
		if err != nil {
			t.Fatal(err)
		}
		return l
	}
	base := load(t, config.Sources{RepoTOML: []byte("version = 1\n[identity]\nbot_logins = [\"Agent-Alt\"]\napp_id = 42\napp_slug = \"aayudha-bot\"\n" +
		"[autonomy]\napprover = \"Lead\"\nnever = [\"L04\", \"L07\"]\nstreak = 20\ncorrection_days = 14\nproof_rungs = [\"A\", \"B\"]\nalways_person = [\"internal/identity/\"]\n" +
		"[seats.engineering-manager]\nmode = \"paired\"\nhuman = \"em-lead\"\n"), RepoSource: "base"}).Config
	for _, c := range []struct {
		why  string
		sets []string
		want []string
	}{
		{"nothing set: the defaults are looser, but no one chose them", nil, nil},
		{"the same values, in another case", []string{"identity.bot_logins=agent-alt", "autonomy.approver=lead", "seats.engineering-manager.human=EM-Lead"}, nil},
		{"tighter values", []string{"autonomy.streak=40", "autonomy.correction_days=30", "autonomy.never=L04,L07,L09"}, nil},
		{"every loosening", []string{"identity.bot_logins=", "identity.app_id=0", "identity.app_slug=", "autonomy.approver=agent-alt", "autonomy.never=L04",
			"autonomy.streak=0", "autonomy.correction_days=1", "autonomy.proof=true", "autonomy.proof_rungs=A,B,C", "autonomy.always_person=", "seats.engineering-manager.mode=solo"},
			[]string{"[identity] bot_logins drops Agent-Alt", "[identity] has no bot identity", `[autonomy] approver is "agent-alt", not "Lead"`, "[autonomy] never drops L07",
				"[autonomy] streak is 0, below 20", "[autonomy] correction_days is 1, below 14", "[autonomy] proof is on", "[autonomy] proof_rungs adds C",
				"[autonomy] always_person drops internal/identity/", "[seats.engineering-manager] is not paired with em-lead"}},
	} {
		if got := loosens(load(t, config.Sources{Sets: c.sets}), &base); !slices.Equal(got, c.want) {
			t.Errorf("%s: loosens = %q, want %q", c.why, got, c.want)
		}
	}
}

// Story #1007: a merge through ship tells the Kanha of the story's Feature at
// once. kanhaShip is the ship world of a PR for story #2 of Feature #1 with a
// green merge gate and a person's approval, #1's mission started with its
// Kanha k on the ledger, and every receiver CLI stubbed: send answers each
// call, and calls are what ship sent.
type kanhaShip struct {
	g         *fake.GitHub
	env       map[string]string
	opts      Options
	bare      string
	prN       int
	head      string
	mu        sync.Mutex
	calls     []string
	send      func() error
	stall     bool   // each send waits until its context ends, as a stalled CLI does
	shippedAt []bool // at each call: merged, ship record on the ledger, merge in a ledger actions segment
}

const shipKanhaThread = "T-01a17000-0000-7000-8000-000000000001"

func newKanhaShip(t *testing.T, k ready.Parent) *kanhaShip {
	t.Helper()
	g, env, opts, dir, bare, prN, head := thinFixture(t)
	gateRun(t, g, dir, prN, head, coverageHolds)
	g.Mu.Lock()
	g.Pulls[prN].Reviews = []fake.Review{review("reviewer", "User", "APPROVED", head)}
	g.Mu.Unlock()
	var a ready.Approval
	if err := json.Unmarshal([]byte(git(t, bare, "show", "ledger:"+ready.ApprovalPath(1))), &a); err != nil {
		t.Fatal(err)
	}
	a.Parent = &k
	ab, _ := json.Marshal(a)
	putLedger(t, bare, ready.ApprovalPath(1), string(ab))
	pb, _ := json.Marshal(MissionPlan{Schema: MissionPlanSchema, Feature: 1, Approval: a.Version, Tool: k.Host, StartedBy: k.StartedBy})
	putLedger(t, bare, MissionPlanPath(1), string(pb))
	s := &kanhaShip{g: g, env: env, bare: bare, prN: prN, head: head, send: func() error { return nil }}
	opts.DelegateExec = func(ctx context.Context, _ string, _ []string, _, name string, args ...string) (string, error) {
		if s.stall {
			<-ctx.Done() // then killed, as exec.CommandContext reports it
			s.mu.Lock()
			s.calls = append(s.calls, name+" "+strings.Join(args, " "))
			s.mu.Unlock()
			return "", errors.New("amp: signal: killed: ")
		}
		g.Mu.Lock()
		merged := g.Pulls[prN].Merged
		g.Mu.Unlock()
		segs := git(t, bare, "ls-tree", "-r", "--name-only", "ledger", "actions/")
		inSeg := false
		for _, p := range strings.Fields(segs) {
			inSeg = inSeg || strings.Contains(git(t, bare, "show", "ledger:"+p), "github.pull.merge")
		}
		record := strings.Contains(git(t, bare, "ls-tree", "-r", "--name-only", "ledger", "proof/2/"+head+"/authoritative/"), "ship.json")
		s.mu.Lock()
		defer s.mu.Unlock()
		s.calls = append(s.calls, name+" "+strings.Join(args, " "))
		s.shippedAt = append(s.shippedAt, merged && record && inSeg)
		return "", s.send()
	}
	s.opts = opts
	return s
}

func (s *kanhaShip) sent() ([]string, []bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.calls), slices.Clone(s.shippedAt)
}

// Criterion 1: ship merges #2's PR, and #2 belongs to #1: after the merge
// lines it sends the Kanha of #1 one message, and says so; the message goes
// only once the merge, the ship record, and the actions segment stand, and
// its own record reaches the ledger.
func TestShipTellsKanhaAfterMerge(t *testing.T) {
	s := newKanhaShip(t, ready.Parent{Scope: "feature", Key: "1", Host: "amp", Session: shipKanhaThread, URL: "https://ampcode.com/threads/" + shipKanhaThread, Mode: "cloud", StartedBy: "founder"})
	code, out := shipCmd(t, s.env, s.opts, s.prN)
	if code != 0 {
		t.Fatalf("ship: %d %s", code, out)
	}
	msg := fmt.Sprintf("merged #2 (PR #%d): run aayu mission next 1", s.prN)
	calls, shipped := s.sent()
	if want := []string{"amp threads continue " + shipKanhaThread + " -ox " + msg}; !slices.Equal(calls, want) {
		t.Fatalf("ship sent %q, want %q", calls, want)
	}
	if !shipped[0] {
		t.Fatal("ship told the Kanha before the merge, the ship record, and the actions segment stood")
	}
	merged, next := strings.Index(out, fmt.Sprintf("ship #%d: merged as", s.prN)), strings.Index(out, "next: told the Kanha of #1\n")
	if merged < 0 || next < merged {
		t.Fatalf("no next line after the merge lines:\n%s", out)
	}
	var found bool
	for _, p := range strings.Fields(git(t, s.bare, "ls-tree", "-r", "--name-only", "ledger", "actions/")) {
		seg := git(t, s.bare, "show", "ledger:"+p)
		found = found || strings.Contains(seg, `"mission.kanha.tell"`) && strings.Contains(seg, msg)
	}
	if !found {
		t.Fatal("the message's mission.kanha.tell record did not reach the ledger")
	}
}

// Criterion 2: the message to the Kanha fails, waits for a local Kanha's
// next check, has no mission to read, or its records miss the ledger: ship
// still exits 0 with the merge, the Complete section, and status:merged in
// place, and says what happened and why.
func TestShipMergeStandsWhenKanhaUnreached(t *testing.T) {
	amp := ready.Parent{Scope: "feature", Key: "1", Host: "amp", Session: shipKanhaThread, Mode: "cloud", StartedBy: "founder"}
	unreached := func(who, why string) string {
		return "next: the Kanha of " + who + " was not reached: " + why + "; the safety tick wakes it\n"
	}
	for _, c := range []struct {
		name  string
		k     ready.Parent
		setup func(s *kanhaShip)
		calls int
		want  []string
	}{
		{"the amp CLI fails", amp, func(s *kanhaShip) {
			s.send = func() error { return fmt.Errorf("amp: exit status 1: thread %s not found", shipKanhaThread) }
		}, 1, []string{unreached("#1", "amp: amp: exit status 1: thread "+shipKanhaThread+" not found")}},
		{"the amp CLI stalls", amp, func(s *kanhaShip) {
			was := receiverTimeout
			receiverTimeout = 200 * time.Millisecond
			t.Cleanup(func() { receiverTimeout = was })
			s.stall = true
		}, 1, []string{unreached("#1", "amp: context deadline exceeded")}},
		{"a local Kanha takes it at its next check", ready.Parent{Scope: "feature", Key: "1", Host: "codex", Session: "s-123", Mode: "local", Machine: "founder@laptop", StartedBy: "founder"}, nil, 0,
			[]string{"next: the Kanha of #1: merged #2 (PR #7): run aayu mission next 1 (recorded; the local Kanha on founder@laptop takes it at its next check)\n"}},
		{"GitHub fails the read of #2's Feature after the merge", amp, func(s *kanhaShip) {
			s.g.Hook = func(method, path string) { // the fake is locked
				if method == "PUT" && strings.HasSuffix(path, "/merge") {
					s.g.Fail, s.g.FailAfter = "GET /repos/o/r/issues/2/parent", 0
				}
			}
		}, 0, []string{"next: the Kanha of #2's Feature was not reached: ", "; the safety tick wakes it\n"}},
		{"the message's record misses the ledger", amp, func(s *kanhaShip) {
			s.send = func() error { // the ledger refuses every push from now on, whatever the global hooks path
				git(t, s.bare, "config", "core.hooksPath", "hooks")
				return os.WriteFile(filepath.Join(s.bare, "hooks", "pre-receive"), []byte("#!/bin/sh\nexit 1\n"), 0o755)
			}
		}, 1, []string{"next: the Kanha of #1 may not have been told: its event was not recorded: ",
			"aayu: warning: the record of the message to the Kanha did not reach the ledger; it stays in the local actions log: "}},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := newKanhaShip(t, c.k)
			if c.setup != nil {
				c.setup(s)
			}
			code, out := shipCmd(t, s.env, s.opts, s.prN)
			for _, want := range c.want {
				if code != 0 || !strings.Contains(out, want) {
					t.Fatalf("ship: %d, want exit 0 and %q:\n%s", code, want, out)
				}
			}
			if calls, _ := s.sent(); len(calls) != c.calls {
				t.Fatalf("ship sent %q", calls)
			}
			s.g.Mu.Lock()
			merged, body := s.g.Pulls[s.prN].Merged, s.g.Issues[2].Body
			s.g.Mu.Unlock()
			if labels := s.g.LabelNames(2); !merged || !contains(labels, "status:merged") || !strings.Contains(body, "### Outcome") {
				t.Fatalf("after ship: merged %t, labels %v, body:\n%s", merged, labels, body)
			}
		})
	}
}

// Criterion 3: ship refuses because inspect says fix (its check is red on
// the head): it sends the Kanha nothing.
func TestShipSendsNothingOnRefusal(t *testing.T) {
	s := newKanhaShip(t, ready.Parent{Scope: "feature", Key: "1", Host: "amp", Session: shipKanhaThread, Mode: "cloud", StartedBy: "founder"})
	s.g.Mu.Lock()
	for i, r := range s.g.CheckRuns[s.head] {
		if r.Name == "aayudha/inspect" || r.Name == "aayudha/merge-gate" {
			s.g.CheckRuns[s.head][i].Conclusion = "failure"
		}
	}
	s.g.Mu.Unlock()
	code, out := shipCmd(t, s.env, s.opts, s.prN)
	if code != ExitBlocked || !strings.Contains(out, "is failure on "+short(s.head)) {
		t.Fatalf("ship with inspect at fix: %d %s", code, out)
	}
	if calls, _ := s.sent(); len(calls) != 0 || strings.Contains(out, "Kanha") {
		t.Fatalf("a refused ship told the Kanha: %q\n%s", calls, out)
	}
	s.g.Mu.Lock()
	defer s.g.Mu.Unlock()
	if s.g.Pulls[s.prN].Merged {
		t.Fatal("merged although inspect says fix")
	}
}

// Bug #1031 criterion 4: `aayu proof land <pr>` lands the merge gate's own
// pack of a PR merged outside ship, after the checks ship makes on the
// merged head, as one authoritative ledger commit naming the gate's run and
// the person; then the audit counts the story as covered. An open PR, a gate
// run on another head, and an expired artifact each exit 4 with the reason.
func TestProofLand(t *testing.T) {
	g, env, opts, dir, bare, prN, head := thinFixture(t)
	gateRun(t, g, dir, prN, head, coverageHolds)
	land := func() (int, string) {
		code, out, errOut := run(t, env, opts, "proof", "land", strconv.Itoa(prN), "--repo", "o/r")
		return code, out + errOut
	}
	if code, out := land(); code != ExitBlocked || !strings.Contains(out, "the PR is open, not merged") {
		t.Fatalf("proof land of an open PR: %d %s", code, out)
	}
	if code, out, errOut := run(t, env, opts, "proof", "land", "999", "--repo", "o/r"); code != ExitExternal || !strings.Contains(out+errOut, "404") {
		t.Fatalf("proof land of a PR that does not exist: %d %s %s", code, out, errOut)
	}
	g.Mu.Lock()
	g.Pulls[prN].Merged, g.Pulls[prN].State = true, "closed"
	good := g.Runs[len(g.Runs)-1]
	arts := g.Artifacts[good.ID]
	g.Artifacts[good.ID] = []fake.ArtifactZip{{ID: arts[0].ID, Name: arts[0].Name, Zip: arts[0].Zip, Expired: true}}
	// The body's story must be the branch's, as ship requires.
	g.Pulls[prN].Body = "Refs #3"
	g.Mu.Unlock()
	if code, out := land(); code != ExitBlocked || !strings.Contains(out, "the PR body refers to story #3, but its branch story/2-show-empty-list is story #2's") {
		t.Fatalf("proof land with a body story that is not the branch's: %d %s", code, out)
	}
	// A body that names two stories is refused, as the gate refuses it.
	g.Mu.Lock()
	g.Pulls[prN].Body = "Refs #2\nFixes: #3"
	g.Mu.Unlock()
	if code, out := land(); code != ExitBlocked || !strings.Contains(out, "the PR body refers to stories #2 and #3: keep one") {
		t.Fatalf("proof land with two stories in the body: %d %s", code, out)
	}
	g.Mu.Lock()
	g.Pulls[prN].Body = "Refs #2"
	g.Mu.Unlock()
	if code, out := land(); code != ExitBlocked || !strings.Contains(out, fmt.Sprintf("proof land #%d: no merge gate pack for the head %s: run %d: its aayudha-pack artifact %d expired", prN, short(head), good.ID, arts[0].ID)) {
		t.Fatalf("proof land with an expired artifact: %d %s", code, out)
	}
	other := strings.Repeat("e", 40)
	g.Mu.Lock()
	g.Artifacts[good.ID] = arts
	g.Runs = []fake.WorkflowRun{good}
	g.Runs[0].HeadSHA = other
	g.Mu.Unlock()
	if code, out := land(); code != ExitBlocked || !strings.Contains(out, "proves "+short(other)+", not the head "+short(head)) {
		t.Fatalf("proof land with a gate run on another head: %d %s", code, out)
	}
	if got := git(t, bare, "ls-tree", "-r", "--name-only", "ledger", "proof/"); got != "" {
		t.Fatalf("a refused proof land landed a pack:\n%s", got)
	}
	// A gate pack whose floors do not hold is refused, as ship refuses it.
	g.Mu.Lock()
	g.Runs[0].HeadSHA = head
	g.Mu.Unlock()
	gateRun(t, g, dir, prN, head, []proof.Check{{Floor: "coverage", Result: proof.Fail, Value: 50, Threshold: 90, Tool: "aayudha", Version: "1"}})
	if code, out := land(); code != ExitBlocked || !strings.Contains(out, fmt.Sprintf("proof land #%d: floors do not hold in the merge gate's pack: coverage", prN)) {
		t.Fatalf("proof land of a pack whose floors fail: %d %s", code, out)
	}
	g.Mu.Lock()
	g.Runs = []fake.WorkflowRun{good}
	gateRuns := g.CheckRuns[head]
	// The head's checks now, as ship reads them: a newer red gate run, and a
	// red required check, refuse; a red protected-paths, which a solo-mode
	// bypass merges past, does not.
	g.CheckRuns[head] = append([]fake.CheckRun{{ID: 99, Name: "aayudha/pack", Status: "completed", Conclusion: "failure"}}, gateRuns...)
	g.Mu.Unlock()
	if code, out := land(); code != ExitBlocked || !strings.Contains(out, "proof land #"+strconv.Itoa(prN)+": aayudha/pack is failure on "+short(head)) {
		t.Fatalf("proof land with a newer red gate run: %d %s", code, out)
	}
	// Bug #1162: a newer gate run failed outside the gate checks, so the pack
	// is the older run's, and it lands only with that run's gate checks green.
	g.Mu.Lock()
	g.Runs = append(g.Runs, fake.WorkflowRun{ID: 9900, Path: ".github/workflows/aayudha-merge-gate.yml", HeadSHA: head, Event: "pull_request_target", Status: "completed", Conclusion: "failure", CheckSuiteID: 6,
		PullRequests: []fake.RunPR{{Number: prN, Base: fake.Branch{Ref: "dev"}}}})
	g.CheckRuns[head] = []fake.CheckRun{{Name: "aayudha/merge-gate", Status: "completed", Conclusion: "success", Suite: &fake.Suite{ID: 6}},
		{Name: "aayudha/inspect", Status: "completed", Conclusion: "success", Suite: &fake.Suite{ID: 6}}, {Name: "aayudha/pack", Status: "completed", Conclusion: "success", Suite: &fake.Suite{ID: 6}},
		{Name: "aayudha/merge-gate", Status: "completed", Conclusion: "success"}, {Name: "aayudha/inspect", Status: "completed", Conclusion: "failure"}, {Name: "aayudha/pack", Status: "completed", Conclusion: "success"}}
	g.Mu.Unlock()
	if code, out := land(); code != ExitBlocked || !strings.Contains(out, fmt.Sprintf("proof land #%d: aayudha/inspect is failure in run %d, whose pack lands, on %s", prN, good.ID, short(head))) {
		t.Fatalf("proof land with the pack's run red and a newer run's gate checks green: %d %s", code, out)
	}
	g.Mu.Lock()
	g.Runs = []fake.WorkflowRun{good}
	g.RequiredChecks = []string{"build", "protected-paths"}
	g.CheckRuns[head] = append(slices.Clone(gateRuns), fake.CheckRun{Name: "build", Status: "completed", Conclusion: "failure"},
		fake.CheckRun{Name: "protected-paths", Status: "completed", Conclusion: "failure"})
	g.Mu.Unlock()
	if code, out := land(); code != ExitBlocked || !strings.Contains(out, "proof land #"+strconv.Itoa(prN)+": required check build is failure on "+short(head)) {
		t.Fatalf("proof land with a red required check: %d %s", code, out)
	}
	g.Mu.Lock()
	g.CheckRuns[head] = append(slices.Clone(gateRuns), fake.CheckRun{Name: "build", Status: "completed", Conclusion: "success"},
		fake.CheckRun{Name: "protected-paths", Status: "completed", Conclusion: "failure"})
	g.Mu.Unlock()
	// The audit's inputs: the merged PR delivered #2, and it is uncovered.
	g.Mu.Lock()
	g.Static = map[string]any{"/repos/o/r/rulesets/rule-suites": jsonOf(t, "[]"), "/repos/o/r/commits": jsonOf(t, "[]"), "/repos/o/r/actions/runs": jsonOf(t, `{"total_count":0,"workflow_runs":[]}`),
		"/repos/o/r/actions/workflows/release.yml/runs":       jsonOf(t, `{"total_count":0,"workflow_runs":[]}`),
		"/repos/o/r/actions/workflows/aayudha-merge-gate.yml": jsonOf(t, `{"id":1,"path":".github/workflows/aayudha-merge-gate.yml","created_at":"2026-10-01T00:00:00Z"}`),
		"/repos/o/r/issues/2/timeline":                        jsonOf(t, `[{"event":"labeled","actor":{"login":"founder"},"created_at":"2026-10-02T00:00:00Z","label":{"name":"type:story"}}]`)}
	g.Pulls[prN].MergedAt = time.Now().UTC().Add(-time.Hour)
	g.Mu.Unlock()
	if code, out, errOut := run(t, env, opts, "audit", "--repo", "o/r", "--dry-run"); code != 0 || !strings.Contains(out, "proof coverage 0/1; merged without proof: pre-gate 0, post-gate 1") {
		t.Fatalf("audit before proof land: %d %s %s", code, out, errOut)
	}
	// Bug #1159: once the PR merges, GitHub lists no pull request for its gate
	// runs. A run with none counts only on the PR's head branch, in this
	// repository, started while the PR was open, when no other pull request
	// from that branch has the head; a run that lists another pull request
	// never counts.
	opened, ran := time.Now().UTC().Add(-3*time.Hour), time.Now().UTC().Add(-2*time.Hour)
	setRun := func(prs []fake.RunPR, branch, repo string, at time.Time) {
		g.Mu.Lock()
		g.Pulls[prN].CreatedAt = opened
		g.Runs[0].PullRequests, g.Runs[0].HeadBranch, g.Runs[0].HeadRepository, g.Runs[0].CreatedAt = prs, branch, &fake.HeadRepo{FullName: repo}, at
		g.Mu.Unlock()
	}
	missing := func(what string) {
		t.Helper()
		if code, out := land(); code != ExitBlocked || !strings.Contains(out, "aayudha/merge-gate is missing on "+short(head)) {
			t.Fatalf("proof land with %s: %d %s", what, code, out)
		}
	}
	branch := g.Pulls[prN].Head.Ref
	for _, c := range []struct {
		what         string
		prs          []fake.RunPR
		branch, repo string
		at           time.Time
	}{
		{"a run that lists another PR", []fake.RunPR{{Number: prN + 1}}, branch, "o/r", ran},
		{"another branch", nil, "story/9-other", "o/r", ran},
		{"a fork's run", nil, branch, "fork/r", ran},
		{"a run after the merge", nil, branch, "o/r", time.Now().UTC()},
		{"a run before the PR opened", nil, branch, "o/r", opened.Add(-time.Minute)},
	} {
		setRun(c.prs, c.branch, c.repo, c.at)
		missing(c.what)
	}
	// A red gate run re-run green after the merge: the re-run keeps its
	// created_at, but its latest attempt started after the merge.
	setRun(nil, branch, "o/r", ran)
	g.Mu.Lock()
	g.Runs[0].RunStartedAt, g.Runs[0].Attempt = time.Now().UTC(), 2
	g.Mu.Unlock()
	missing("a re-run started after the merge")
	g.Mu.Lock()
	g.Runs[0].RunStartedAt, g.Runs[0].Attempt = time.Time{}, 0
	g.Mu.Unlock()
	// A second pull request from the same branch at the same head: the run
	// could be either one's, so it is neither's.
	setRun(nil, branch, "O/R", ran)
	twin := g.AddPR("Show empty list again", "Refs #2", "founder", branch, head, "rolling")
	missing("a second PR from the branch at the head")
	g.Mu.Lock()
	twin.Head.SHA = strings.Repeat("f", 40)
	g.Mu.Unlock()
	// A newer green gate run with its own pack, started by an event after the
	// merge (an edit of the body): it ran the base branch that now holds the
	// PR's code, so neither its checks nor its pack count. proof land takes
	// the pack of the PR's own run from before the merge.
	g.Mu.Lock()
	checks := slices.Clone(g.CheckRuns[head])
	g.Mu.Unlock()
	gateRun(t, g, dir, prN, head, coverageHolds)
	g.Mu.Lock()
	late := &g.Runs[len(g.Runs)-1]
	late.PullRequests, late.HeadBranch, late.HeadRepository, late.CheckSuiteID = nil, branch, &fake.HeadRepo{FullName: "o/r"}, 77
	late.CreatedAt, late.RunStartedAt = time.Now().UTC(), time.Now().UTC()
	g.CheckRuns[head] = checks
	g.Mu.Unlock()
	// The repository's name compares without case, as GitHub's does.
	before := git(t, bare, "rev-list", "--count", "ledger")
	if code, out := land(); code != 0 || !strings.Contains(out, fmt.Sprintf("from artifact %d of run %d", arts[0].ID, good.ID)) {
		t.Fatalf("proof land: %d %s", code, out)
	}
	if n, _ := strconv.Atoi(before); git(t, bare, "rev-list", "--count", "ledger") != strconv.Itoa(n+1) {
		t.Fatalf("proof land did not make exactly one ledger commit after %s", before)
	}
	var p proof.Pack
	if err := json.Unmarshal([]byte(git(t, bare, "show", "ledger:proof/2/"+head+"/authoritative/pack.json")), &p); err != nil ||
		p.Authority != (proof.Authority{Kind: proof.Authoritative, Workflow: ".github/workflows/aayudha-merge-gate.yml", RunID: good.ID, Writer: "founder (aayu proof land)"}) {
		t.Fatalf("landed pack: %v %+v", err, p.Authority)
	}
	code, out, errOut := run(t, env, opts, "audit", "--repo", "o/r", "--dry-run")
	if code != 0 || !strings.Contains(out, "proof coverage 1/1; merged without proof: pre-gate 0, post-gate 0") {
		t.Fatalf("audit after proof land: %d %s %s", code, out, errOut)
	}
	// A failed read of the PR, or of the PRs from its branch, is an error, never a gate.
	c, err := github.New(github.Options{Token: "t", BaseURL: g.URL(), WriteGap: -1, ServerRetries: -1})
	if err != nil {
		t.Fatal(err)
	}
	for _, fail := range []string{fmt.Sprintf("GET /repos/o/r/pulls/%d", prN), "GET /repos/o/r/pulls"} {
		g.Mu.Lock()
		g.Fail, g.FailAfter = fail, 0
		g.Mu.Unlock()
		if suites, err := gateSuites(context.Background(), c, "o", "r", head, prN); err == nil {
			t.Errorf("gateSuites with %s failing: %v", fail, suites)
		}
	}
}
