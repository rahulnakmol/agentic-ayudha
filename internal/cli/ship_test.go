package cli

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
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
	g.CheckRuns[head] = []fake.CheckRun{{Name: "aayudha/merge-gate", Status: "completed", Conclusion: "success"}, {Name: "aayudha/inspect", Status: "completed", Conclusion: "success"}}
}

// WP4.10/WP5.5 ship refuses, before merging anything: without the human's
// token; a PR that does not exist, is already merged or closed, or serves no
// story; a merge gate that is red or still running on the head; a gate whose
// pack cannot be trusted: no aayudha-pack artifact, an artifact that is not a
// zip or has an entry outside its directory, no pack for the story, no
// successful run, or a pack for another head; GitHub failing a read on the
// way to the pack (exit 5); a pack with no Ready snapshot on the ledger to
// verify against; and a repository without a ledger.
func TestShipRefusals(t *testing.T) {
	g, env, opts, dir, _, prN, head := thinFixture(t)
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

	for _, gate := range []fake.CheckRun{
		{Name: "aayudha/merge-gate", Status: "completed", Conclusion: "failure"},
		{Name: "aayudha/merge-gate", Status: "in_progress"},
	} {
		g.Mu.Lock()
		g.CheckRuns[head] = []fake.CheckRun{gate, {Name: "aayudha/inspect", Status: "completed", Conclusion: "success"}}
		g.Mu.Unlock()
		want := "aayudha/merge-gate is " + gate.Conclusion + " on "
		if gate.Conclusion == "" {
			want = "aayudha/merge-gate is in_progress on "
		}
		if code, out := ship(prN); code != ExitBlocked || !strings.Contains(out, want) || !strings.Contains(out, "the merge gate must be green") {
			t.Fatalf("ship with the gate %s/%s: %d %s", gate.Status, gate.Conclusion, code, out)
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
		{"no aayudha-pack artifact", []fake.ArtifactZip{{ID: 8001, Name: "coverage", Zip: zipOf(nil)}}, ExitBlocked, "merge gate run 8000 has no aayudha-pack artifact"},
		{"an artifact that is not a zip", []fake.ArtifactZip{{ID: 8002, Name: "aayudha-pack", Zip: []byte("not a zip")}}, ExitExternal, "zip"},
		{"an entry outside the artifact", []fake.ArtifactZip{pack(8003, map[string]string{"2/": "", "../escape.json": "{}"})}, ExitExternal, `artifact entry "../escape.json"`},
		{"no pack for the story", []fake.ArtifactZip{pack(8004, map[string]string{"README.txt": "nothing here"})}, ExitBlocked, "merge gate run 8000's pack: "},
	}
	for _, tt := range tests {
		gateArtifact(g, head, tt.arts...)
		if code, out := ship(prN); code != tt.code || !strings.Contains(out, tt.want) {
			t.Errorf("ship with %s: %d %s", tt.name, code, out)
		}
	}
	gateArtifact(g, head)
	g.Mu.Lock()
	g.Runs = nil
	g.Mu.Unlock()
	if code, out := ship(prN); code != ExitBlocked || !strings.Contains(out, "no successful aayudha-merge-gate.yml run for that head") {
		t.Fatalf("ship with a green check but no gate run: %d %s", code, out)
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
	other := strings.Repeat("f", 40)
	gateRun(t, g, dir, prN, other, coverageHolds)
	g.Mu.Lock()
	g.Runs[len(g.Runs)-1].HeadSHA = head
	g.CheckRuns[head] = g.CheckRuns[other]
	g.Mu.Unlock()
	if code, out := ship(prN); code != ExitBlocked || !strings.Contains(out, "pack is for #2 at "+short(other)+", not #2 at "+short(head)) {
		t.Fatalf("ship with a pack for another head: %d %s", code, out)
	}

	// The Ready snapshot is gone from the ledger: the landed pack cannot be checked against it.
	gateRun(t, g, dir, prN, head, coverageHolds)
	if _, err := (&ledger.Writer{Dir: dir}).Write(context.Background(), ledger.Commit{Message: "drop the snapshot", Delete: []string{ready.LedgerPath(2)}}); err != nil {
		t.Fatal(err)
	}
	if code, out := ship(prN); code != ExitBlocked || !strings.Contains(out, "the authoritative pack does not verify: ") || !strings.Contains(out, "no Ready snapshot on the ledger") {
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
	if sr.HumanPass.Kind != "autonomous" || !sr.Streak.Autonomous || sr.Streak.Threshold != 0 || len(sr.Pass.Never) != 0 {
		t.Fatalf("ship record: %+v", sr)
	}
	if c := comment(g, 2, "<!-- aayudha:lease-released -->"); !strings.Contains(c, "shipped in #"+strconv.Itoa(prN)) {
		t.Fatalf("lease not released:\n%s", c)
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
