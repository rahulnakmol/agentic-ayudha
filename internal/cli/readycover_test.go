package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
	"github.com/rahulnakmol/agentic-ayudha/internal/ready"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// Story #996: Arjun makes a Story Ready only while its Feature's Approve to
// build covers it at its Completion Goal, effort, and title. A Story with no
// parent Feature, under a Feature no person approved, under a Feature that
// changed after it was approved, or that changed or was added after the
// approval, is refused with exit 4 and nothing is written. An early-build
// approval covers its one story. The guard and the build pick never apply the
// rule, so a Story made Ready before it stays Ready.
func TestReadyStoryCoverage(t *testing.T) {
	g := fake.New(t, "o/r", "User")
	g.Viewer = "rahulnakmol"
	numbered := func(is *fake.Issue, n int) *fake.Issue { // the issue numbers of the Story's scenario
		g.Mu.Lock()
		defer g.Mu.Unlock()
		delete(g.Issues, is.Number)
		is.Number, g.Issues[n] = n, is
		return is
	}
	edit := func(f func()) { g.Mu.Lock(); f(); g.Mu.Unlock() }
	storyBody := func(title, assumptions string) string {
		return fixtureBody(t, "story", func(f map[string]string) {
			f[workitems.FieldCriteria] = strings.Replace(f[workitems.FieldCriteria], "cart is empty", title+" is empty", 1)
			f[workitems.FieldAssumptions] = assumptions
		})
	}
	initiative := numbered(g.AddIssue("Human-dependency engine", fixtureBody(t, "initiative", func(f map[string]string) { delete(f, workitems.FieldLinks) }), "type:initiative"), 870)
	feature := numbered(g.AddIssue("Approve to build", fixtureBody(t, "feature", func(f map[string]string) { delete(f, workitems.FieldLinks) }), "type:feature", "status:planned"), 840)
	edit(func() { feature.Parent, initiative.Subs = 870, []int{840} })
	stories := map[int]*fake.Issue{}
	for n := 851; n <= 856; n++ {
		title := fmt.Sprintf("Story %d", n)
		s := numbered(g.AddIssue(title, storyBody(title, "The fake stands in for GitHub."), "type:story", "effort:S", "layer:L02"), n)
		edit(func() { s.Parent, feature.Subs = 840, append(feature.Subs, n) })
		stories[n] = s
	}
	numbered(g.AddIssue("Story 857", storyBody("Story 857", "No Feature yet."), "type:story", "effort:S"), 857)
	old := numbered(g.AddIssue("Story 700", storyBody("Story 700", "Ready before the rule."), "type:story", "effort:S"), 700)

	next, _, opts, dir, bare := buildLoopFixture(t, g)
	opts.IsTerminal, opts.DevicePoll = stdinTTY, 10*time.Millisecond
	state := filepath.Join(dir, "state")
	arjun := map[string]string{"AAYU_GITHUB_TOKEN": "human-token", "AMP_THREAD_ID": "T-arjun", "XDG_STATE_HOME": state, "PATH": os.Getenv("PATH")}
	person := map[string]string{"AAYU_GITHUB_TOKEN": "human-token", "XDG_STATE_HOME": state, "PATH": os.Getenv("PATH")}
	readyRun := func(n int) (int, string) {
		t.Helper()
		code, out, errOut := run(t, arjun, opts, "ready", strconv.Itoa(n), "--repo", "o/r")
		return code, out + errOut
	}
	commits := func() string { return git(t, bare, "rev-list", "--count", "ledger") }
	snapshot := func(n int) string {
		g.Mu.Lock()
		defer g.Mu.Unlock()
		for _, c := range g.Issues[n].Comments {
			if strings.HasPrefix(c.Body, ready.SnapshotMarker) {
				return c.Body
			}
		}
		return ""
	}
	// refused checks that Arjun's ready n exits 4 with want and writes no ledger commit, label, or comment.
	refused := func(n int, want string) {
		t.Helper()
		before, labels, snap := commits(), g.LabelNames(n), snapshot(n)
		g.Reset()
		code, out := readyRun(n)
		if code != ExitBlocked || !strings.Contains(out, want) || len(g.Writes()) != 0 || commits() != before ||
			!slices.Equal(g.LabelNames(n), labels) || snapshot(n) != snap {
			t.Fatalf("ready %d: want exit 4 with %q and nothing written; exit %d, writes %v, ledger commits %s → %s:\n%s", n, want, code, g.Writes(), before, commits(), out)
		}
	}
	passes := func(n int, row string) {
		t.Helper()
		if code, out := readyRun(n); code != 0 || !slices.Contains(g.LabelNames(n), ready.LabelReady) {
			t.Fatalf("ready %d: exit %d, labels %v:\n%s", n, code, g.LabelNames(n), out)
		}
		want := "| Approved to build | " + row + " |"
		if !strings.Contains(snapshot(n), want) || !strings.Contains(git(t, bare, "show", "ledger:"+ready.LedgerPath(n)), want) {
			t.Fatalf("the Ready snapshot of #%d has no row %q:\n%s", n, want, snapshot(n))
		}
	}

	// 7 (setup). #700 was made Ready before the rule: the snapshot and the label the old ready left.
	edit(func() {
		old.Comments = append(old.Comments, &fake.Comment{ID: 7000, Body: ready.Snapshot(ready.Result{Type: "story", BodyHash: workitems.BodyHash(old.Body),
			TechnicalOwner: ready.TwinOwner, Identity: "arjun", LedgerPath: ready.LedgerPath(700)})})
		old.Labels = append(old.Labels, fake.Label{Name: ready.LabelReady})
	})

	// 1. No parent Feature.
	refused(857, "#857 has no parent Feature. A Story is Ready only under a Feature approved to build (Decision #861).")
	// 2. Under a Feature with no approved.json on the ledger.
	notApproved := "Feature #840 is not approved to build. A person runs aayu ready 840."
	refused(853, notApproved)

	// 6. An early-build approval of #852 covers #852 only.
	early := func(body string) string {
		b, _ := json.Marshal(map[string]any{"schema": "aayudha.feature.early-build/v1", "feature": 840, "story": 852, "body_hash": workitems.BodyHash(body),
			"budget_usd": 60, "approved_by": "rahulnakmol", "presence": "device-flow", "at": "2026-10-26T00:00:00Z"})
		return string(b)
	}
	putLedger(t, bare, "specs/840/approved-early-852.json", early(stories[852].Body))
	passes(852, "#840 early build of #852 by @rahulnakmol")
	refused(853, notApproved)
	// The early build covers #852 at the body the person approved.
	body852 := stories[852].Body
	edit(func() { stories[852].Body = storyBody("Story 852", "An edit after the early build.") })
	refused(852, "#852 changed after its early build was approved")
	refused(852, "next: aayu ready 840\n")
	edit(func() { stories[852].Body = body852 })
	// A record that names another story covers neither.
	putLedger(t, bare, "specs/840/approved-early-854.json", early(stories[854].Body))
	if code, out := readyRun(854); code != ExitExternal || !strings.Contains(out, "not an early-build approval of #854 under #840") || slices.Contains(g.LabelNames(854), ready.LabelReady) {
		t.Fatalf("an early-build record of #852 at #854's path: exit %d:\n%s", code, out)
	}

	// A person approves #840 to build: intent v1, spec v1, and technical design v1, then approval 1.
	request := func(part string, args ...string) {
		t.Helper()
		if code, out, errOut := run(t, arjun, opts, append([]string{"ready", "840", "--repo", "o/r", "--part", part, "--request"}, args...)...); code != 0 {
			t.Fatalf("--part %s --request: exit %d: %s%s", part, code, out, errOut)
		}
	}
	approveByHand := func(part string, v int) { // a part's own approval is TestReadyFeatureBaseline's
		t.Helper()
		var b ready.Baseline
		if err := json.Unmarshal([]byte(git(t, bare, "show", "ledger:"+ready.BaselinePath(840, part, v, true))), &b); err != nil {
			t.Fatal(err)
		}
		b.ApprovedBy, b.ApprovedAt, b.Presence = "rahulnakmol", "2026-10-26T00:00:00Z", "device-flow"
		data, _ := json.Marshal(b)
		putLedger(t, bare, ready.BaselinePath(840, part, v, false), string(data))
	}
	request("intent")
	approveByHand("intent", 1)
	request("spec")
	approveByHand("spec", 1)
	design := `{"feature": 840, "layers": ["L02"], "story_order": [851, 852, 853, 854, 855, 856]}`
	putLedger(t, bare, "specs/840/design.json", design)
	request("tech")
	approveByHand("tech", 1)
	edit(func() {
		g.Device = &fake.DeviceFlow{ClientID: DefaultDeviceClientID, Approver: "rahulnakmol", Outcome: "approve"}
	})
	if code, out, errOut := run(t, person, opts, "ready", "840", "--repo", "o/r"); code != 0 || !strings.Contains(out, "Approved #840 to build as @rahulnakmol, approval 1.") {
		t.Fatalf("Approve to build of #840: exit %d: %s%s", code, out, errOut)
	}

	// 5. Covered by approval 1, with only its Assumptions changed: Ready as today, and the snapshot names the approval.
	edit(func() {
		stories[853].Body = storyBody("Story 853", "The fake and a synthetic ledger stand in for GitHub.")
	})
	passes(853, "#840 approval 1 by @rahulnakmol")

	// 4. The effort of #853 changed from S to M, and #858 was added under #840, after approval 1.
	edit(func() {
		stories[853].Labels = slices.DeleteFunc(stories[853].Labels, func(l fake.Label) bool { return l.Name == "effort:S" })
		stories[853].Labels = append(stories[853].Labels, fake.Label{Name: "effort:M"})
	})
	refused(853, "#853 changed after Approve to build (effort S to M). A person acknowledges it: aayu ready 840 --ack 853")
	refused(853, "next: aayu ready 840 --part spec --request") // a command that runs today
	added := numbered(g.AddIssue("Story 858", storyBody("Story 858", "Added later."), "type:story", "effort:S"), 858)
	edit(func() { added.Parent, feature.Subs = 840, append(feature.Subs, 858) })
	refused(858, "#858 was added after Approve to build. A person acknowledges it: aayu ready 840 --ack 858")
	// A changed title or Completion Goal is a change too; the story it names is the only one refused.
	edit(func() { stories[854].Title = "Story 854, renamed" })
	refused(854, "#854 changed after Approve to build (title).")
	edit(func() {
		stories[855].Body = strings.Replace(stories[855].Body, "422 with code cart.empty", "422 with code cart.none", 1)
	})
	refused(855, "#855 changed after Approve to build (Completion Goal).")
	passes(851, "#840 approval 1 by @rahulnakmol")

	// 3. The body of #840 changed after approval 1, then technical design v2 was approved: every story waits.
	fbody := feature.Body
	edit(func() {
		feature.Body = strings.Replace(fbody, "A five-question eligibility check.", "A six-question eligibility check.", 1)
	})
	refused(856, "#840 changed after Approve to build: intent v1 drifted (#840 body changed). Its stories wait until a person approves it again.")
	edit(func() { feature.Body = fbody })
	putLedger(t, bare, "specs/840/design.json", `{"feature": 840, "layers": ["L02", "L03"], "story_order": [851, 852, 853, 854, 855, 856]}`)
	changes := filepath.Join(t.TempDir(), "changes.json")
	data, _ := json.Marshal([]ready.Change{{Ref: "specs/840/design.json", What: "The design adds layer L03 for the merge gate.",
		Why: "The gate reads the approval before a build.", Need: "No story is built before a person agrees."}})
	_ = os.WriteFile(changes, data, 0o644)
	request("tech", "--changes", changes)
	approveByHand("tech", 2)
	refused(856, "#840 changed after Approve to build: technical design v1 to v2. Its stories wait until a person approves it again.")

	// 7. The guard runs on #700's new label and keeps status:ready; build --next still picks #700.
	edit(func() { old.Labels = append(old.Labels, fake.Label{Name: "priority:P0"}) })
	ci := map[string]string{"GITHUB_TOKEN": "workflow-token"}
	if code, out, errOut := run(t, ci, Options{GitHubAPI: g.URL(), GitHubGap: -1}, "ready", "700", "--guard", "--ci", "--repo", "o/r"); code != 0 ||
		!slices.Contains(g.LabelNames(700), ready.LabelReady) || !strings.Contains(out, "#700: Ready holds") {
		t.Fatalf("the guard on #700: exit %d, labels %v: %s%s", code, g.LabelNames(700), out, errOut)
	}
	if code, _, pick, out := next(nil, "--next", "pick", "--dry-run"); code != 0 || pick.Picked != 700 ||
		slices.ContainsFunc(pick.Skipped, func(s string) bool { return strings.HasPrefix(s, "#700:") }) {
		t.Fatalf("build --next with #700 Ready from before the rule: exit %d, %+v: %s", code, pick, out)
	}

	// An approval that agrees technical design v2 covers #856 again; one with no technical design does not.
	var a ready.Approval
	_ = json.Unmarshal([]byte(git(t, bare, "show", "ledger:"+ready.ApprovalPath(840))), &a)
	var tech2 ready.Baseline
	_ = json.Unmarshal([]byte(git(t, bare, "show", "ledger:"+ready.BaselinePath(840, "tech", 2, false))), &tech2)
	approval := func(f func(a *ready.Approval)) {
		b := a
		b.Baselines = slices.Clone(a.Baselines)
		f(&b)
		data, _ := json.Marshal(b)
		putLedger(t, bare, ready.ApprovalPath(840), string(data))
	}
	approval(func(b *ready.Approval) {
		b.Version = 2
		for i := range b.Baselines {
			if b.Baselines[i].Part == "tech" {
				b.Baselines[i] = ready.ApprovedBaseline{Part: "tech", Version: 2, Hash: tech2.Hash}
			}
		}
	})
	passes(856, "#840 approval 2 by @rahulnakmol")
	approval(func(b *ready.Approval) {
		b.Baselines = slices.DeleteFunc(b.Baselines, func(x ready.ApprovedBaseline) bool { return x.Part == "tech" })
	})
	refused(856, "#840 changed after Approve to build: technical design none to v2.")
	// The technical design's design.json is gone from the ledger: it cannot be pinned, so its stories wait.
	approval(func(b *ready.Approval) {
		for i := range b.Baselines {
			if b.Baselines[i].Part == "tech" {
				b.Baselines[i] = ready.ApprovedBaseline{Part: "tech", Version: 2, Hash: tech2.Hash}
			}
		}
	})
	hand := filepath.Join(t.TempDir(), "hand")
	git(t, filepath.Dir(hand), "clone", "--quiet", "--branch", "ledger", bare, hand)
	git(t, hand, "rm", "--quiet", "specs/840/design.json")
	git(t, hand, "commit", "--quiet", "-m", "design.json removed")
	git(t, hand, "push", "--quiet", "origin", "ledger")
	refused(856, "#840 changed after Approve to build: technical design v2 cannot be pinned now.")
	// A record aayu did not write covers nothing.
	putLedger(t, bare, ready.ApprovalPath(840), `{"schema": "aayudha.feature.approved/v1", "feature": 841, "version": 9}`)
	if code, out := readyRun(856); code != ExitExternal || !strings.Contains(out, "not an Approve to build record of #840") {
		t.Fatalf("a forged approval: exit %d:\n%s", code, out)
	}
}
