package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
	"github.com/rahulnakmol/agentic-ayudha/internal/ready"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// Story #995: a person approves a Feature to build, once, through GitHub's
// device flow, and specs/<f>/approved.json on the ledger keeps what was
// agreed: the baselines, the stories, the budget, and the dismissals. An
// agent lane and MCP are refused, and so is a Feature that a condition keeps
// off.
func TestReadyApproveToBuild(t *testing.T) {
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
	initiative := numbered(g.AddIssue("Human-dependency engine", fixtureBody(t, "initiative", func(f map[string]string) { delete(f, workitems.FieldLinks) }), "type:initiative"), 870)
	feature := numbered(g.AddIssue("Approve to build", fixtureBody(t, "feature", func(f map[string]string) { delete(f, workitems.FieldLinks) }), "type:feature", "status:planned"), 840)
	edit(func() { feature.Parent, initiative.Subs = 870, []int{840} })
	stories := map[int]*fake.Issue{}
	for n := 851; n <= 856; n++ {
		title := fmt.Sprintf("Story %d", n)
		s := numbered(g.AddIssue(title, fixtureBody(t, "story", func(f map[string]string) {
			f[workitems.FieldCriteria] = strings.Replace(f[workitems.FieldCriteria], "cart is empty", title+" is empty", 1)
		}), "type:story", "effort:S", "layer:L02"), n)
		edit(func() { s.Parent, feature.Subs = 840, append(feature.Subs, n) })
		stories[n] = s
	}
	dir, bare := ledgerCheckout(t)
	state := filepath.Join(dir, "state")
	agent := map[string]string{"AAYU_GITHUB_TOKEN": "human-token", "AMP_THREAD_ID": "T-agent-thread", "XDG_STATE_HOME": state}
	person := map[string]string{"AAYU_GITHUB_TOKEN": "human-token", "XDG_STATE_HOME": state} // a terminal with no agent marker
	var crit ready.Critique                                                                  // #876's critique.State, faked until #876 merges
	opts := Options{IsTerminal: stdinTTY, GitHubAPI: g.URL(), GitHubGap: -1, Dir: dir, DevicePoll: 10 * time.Millisecond,
		CritiqueState: func(_ context.Context, f int) (ready.Critique, error) {
			if f != 840 {
				t.Errorf("critique.State of #%d", f)
			}
			return crit, nil
		}}
	readyRun := func(env map[string]string, args ...string) (int, string, string) {
		t.Helper()
		return run(t, env, opts, append([]string{"ready", "840", "--repo", "o/r"}, args...)...)
	}
	commits := func() string { return git(t, bare, "rev-list", "--count", "ledger") }
	approved := func() (ready.Approval, string) {
		t.Helper()
		raw := git(t, bare, "show", "ledger:"+ready.ApprovalPath(840))
		var a ready.Approval
		if err := json.Unmarshal([]byte(raw), &a); err != nil {
			t.Fatalf("approved.json: %v\n%s", err, raw)
		}
		return a, raw
	}
	device := func(outcome string, onPoll func(int)) {
		edit(func() {
			g.Device = &fake.DeviceFlow{ClientID: DefaultDeviceClientID, Approver: "rahulnakmol", Outcome: outcome, OnPoll: onPoll}
		})
	}
	started := func() int {
		g.Mu.Lock()
		defer g.Mu.Unlock()
		if g.Device == nil {
			return 0
		}
		return g.Device.Started
	}
	comments := func() int { g.Mu.Lock(); defer g.Mu.Unlock(); return len(feature.Comments) }
	// nothing checks that a refused run wrote no ledger commit, label, or comment.
	nothing := func(what string, before string, f func()) {
		t.Helper()
		g.Reset()
		c, labels := comments(), g.LabelNames(840)
		f()
		if w := g.Writes(); len(w) != 0 || commits() != before || comments() != c || !slices.Equal(g.LabelNames(840), labels) {
			t.Fatalf("%s wrote: %v, ledger commits %s → %s, comments %d → %d", what, w, before, commits(), c, comments())
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
	request := func(part string, args ...string) {
		t.Helper()
		if code, out, errOut := readyRun(map[string]string{"AAYU_GITHUB_TOKEN": "human-token", "AAYU_AGENT": "1", "XDG_STATE_HOME": state},
			append([]string{"--part", part, "--request"}, args...)...); code != 0 {
			t.Fatalf("--part %s --request: exit %d: %s%s", part, code, out, errOut)
		}
	}
	changesFile := func(c ready.Change) string {
		p := filepath.Join(t.TempDir(), "changes.json")
		data, _ := json.Marshal([]ready.Change{c})
		_ = os.WriteFile(p, data, 0o644)
		return p
	}

	// Intent v1, spec v2 (#856 renamed after v1), and technical design v1, each approved and not drifted.
	request("intent")
	approveByHand("intent", 1)
	request("spec")
	approveByHand("spec", 1)
	edit(func() { stories[856].Title = "Story 856, renamed" })
	request("spec", "--changes", changesFile(ready.Change{Ref: "#856", What: "The title names what the story shows.",
		Why: "A person reads titles before each build.", Need: "The founder approves knowing each story."}))
	approveByHand("spec", 2)
	design := `{"feature": 840, "layers": ["L02"], "story_order": [851, 852, 853, 854, 855, 856]}`
	putLedger(t, bare, "specs/840/design.json", design)
	request("tech")
	approveByHand("tech", 1)

	// 1. An agent lane (AMP_THREAD_ID) and an MCP client are refused with exit 4, and nothing is written.
	refused := "Approve to build (aayu ready on a Feature) is a person's decision and is refused for an agent lane (AMP_THREAD_ID)"
	n := commits()
	device("approve", nil)
	nothing("the agent lane", n, func() {
		if code, out, errOut := readyRun(agent); code != ExitBlocked || !strings.Contains(errOut, refused) || started() != 0 {
			t.Fatalf("agent ready 840: exit %d, device codes %d: %s%s", code, started(), out, errOut)
		}
	})
	cs := serveMCPWith(t, Options{Dir: dir, GitHubAPI: g.URL(), GitHubGap: -1}, agent, "--repo", "o/r", "--set", "mcp.expose_writes=true")
	nothing("the MCP call", n, func() {
		out, isErr := call(t, cs, "ready", map[string]any{"issue": "840"})
		if !isErr || !strings.Contains(out, refused) || !regexp.MustCompile(`"code":\s*4\b`).MatchString(out) || started() != 0 {
			t.Fatalf("MCP ready 840: error %v, device codes %d: %s", isErr, started(), out)
		}
	})
	// A story is still Ready from an agent lane: only a Feature's Ready is a person's.
	if code, out, errOut := run(t, agent, opts, "ready", "851", "--repo", "o/r"); code != 0 || !slices.Contains(g.LabelNames(851), "status:ready") {
		t.Fatalf("agent ready 851: exit %d: %s%s", code, out, errOut)
	}
	n = commits()

	// 4. One condition fails at a time: each is refused with exit 4, one line with the next command, and nothing is written.
	off := func(want string) {
		t.Helper()
		nothing(want, n, func() {
			code, out, errOut := readyRun(person)
			if code != ExitBlocked || !strings.Contains(errOut, want) || started() != 0 {
				t.Fatalf("want %q: exit %d, device codes %d: %s%s", want, code, started(), out, errOut)
			}
		})
	}
	putLedger(t, bare, "specs/840/design.json", design+"\n")
	n = commits()
	off("Approve to build is off: the technical design changed after v1 was approved (specs/840/design.json changed). Next: aayu ready 840 --part tech --request")
	putLedger(t, bare, "specs/840/design.json", design)
	n = commits()
	edit(func() { feature.Parent = 0 })
	off("Approve to build is off: #840 has no Initiative parent")
	off("Next: add #840 as a sub-issue of its Initiative, then aayu ready 840")
	edit(func() { feature.Parent = 870 })

	// 2. A person in a terminal: the effect before the device code, then one presence check and the record.
	code, out, errOut := readyRun(person)
	effect := "Approving: Approves #840 to build: 6 stories (#851-#856), intent v1, spec v2, technical design v1, budget up to $360 (6 x $60, [budget] per_story_usd)\n"
	at, enter := strings.Index(errOut, effect), strings.Index(errOut, "/login/device")
	if code != 0 || at < 0 || enter < at || started() != 1 {
		t.Fatalf("person ready 840: exit %d, device codes %d: %s%s", code, started(), out, errOut)
	}
	a, raw := approved()
	logged, _ := os.ReadFile(filepath.Join(dir, ".aayudha", "telemetry", "actions.jsonl"))
	var presence struct{ Kind, Hash string }
	for _, line := range strings.Split(strings.TrimSpace(string(logged)), "\n") {
		_ = json.Unmarshal([]byte(line), &presence)
		if presence.Kind == "presence.approve" {
			break
		}
	}
	base := func(part string, v int) ready.ApprovedBaseline {
		var b ready.Baseline
		_ = json.Unmarshal([]byte(git(t, bare, "show", "ledger:"+ready.BaselinePath(840, part, v, false))), &b)
		return ready.ApprovedBaseline{Part: part, Version: v, Hash: b.Hash}
	}
	var want []ready.ApprovedStory
	for n := 851; n <= 856; n++ {
		it, _ := workitems.Parse(n, stories[n].Title, stories[n].Body, "", []string{"type:story"})
		want = append(want, ready.ApprovedStory{Number: n, Title: stories[n].Title, Effort: "S", CompletionGoalHash: workitems.CompletionGoalHash(it)})
	}
	if a.Schema != ready.ApprovalSchema || a.Feature != 840 || a.Version != 1 || a.By != "rahulnakmol" || a.At == "" ||
		a.Presence != (ready.Presence{Login: "rahulnakmol", Via: "device-flow", Permission: "admin", ActionsRecord: presence.Hash}) || presence.Hash == "" ||
		!slices.Equal(a.Baselines, []ready.ApprovedBaseline{base("intent", 1), base("spec", 2), base("tech", 1)}) ||
		!slices.Equal(a.Stories, want) || a.Budget != (ready.Budget{PerStoryUSD: 60, Stories: 6, TotalUSD: 360}) ||
		!strings.Contains(raw, `"dismissals": []`) {
		t.Fatalf("approved.json:\n%s\npresence record %q", raw, presence.Hash)
	}

	// 3. Then Arjun posts the Feature's Ready snapshot and labels it status:ready.
	g.Mu.Lock()
	var snap string
	for _, c := range feature.Comments {
		if strings.HasPrefix(c.Body, ready.SnapshotMarker) {
			snap = c.Body
		}
	}
	g.Mu.Unlock()
	if !strings.Contains(out, "Approved #840 to build as @rahulnakmol, approval 1. Next: Arjun makes its 6 stories Ready.") ||
		!slices.Contains(g.LabelNames(840), "status:ready") || slices.Contains(g.LabelNames(840), "status:planned") || snap == "" ||
		!strings.Contains(git(t, bare, "show", "ledger:specs/840/ready.md"), "## Body at Ready") {
		t.Fatalf("after the approval: labels %v, snapshot %q: %s", g.LabelNames(840), snap, out)
	}
	// Run again with nothing changed: no new approval and no device code.
	n = commits()
	device("approve", nil)
	if code, out, errOut := readyRun(person); code != 0 || started() != 0 || commits() != n ||
		!strings.Contains(out, "#840 is approved to build already: approval 1 by @rahulnakmol agrees it as it is now.") {
		t.Fatalf("again with nothing changed: exit %d, device codes %d, commits %s → %s: %s%s", code, started(), n, commits(), out, errOut)
	}

	// 6. Technical design v2, with its drift section, is approved; the next Approve to build shows that section and writes version 2.
	design2 := `{"feature": 840, "layers": ["L02", "L03"], "story_order": [851, 852, 853, 854, 855, 856]}`
	putLedger(t, bare, "specs/840/design.json", design2)
	why := ready.Change{Ref: "specs/840/design.json", What: "The design adds layer L03 for the merge gate.",
		Why: "The gate reads the approval before a build.", Need: "No story is built before a person agrees."}
	request("tech", "--changes", changesFile(why))
	approveByHand("tech", 2)

	// 5. The Completion Goal of #853 changes while the code waits: nothing is approved.
	n = commits()
	goal := stories[853].Body
	device("approve", func(int) {
		stories[853].Body = strings.Replace(goal, "422 with code cart.empty", "422 with code cart.none", 1)
	})
	if code, out, errOut := readyRun(person); code != ExitBlocked || started() != 1 || commits() != n ||
		!strings.Contains(errOut, "The stories of #840 changed while the code was waiting; nothing was approved.") {
		t.Fatalf("#853 changed during the code: exit %d, commits %s → %s: %s%s", code, n, commits(), out, errOut)
	}
	edit(func() { stories[853].Body = goal })

	device("approve", nil)
	code, out, errOut = readyRun(person)
	drift := "Approving: Technical design v2 changed specs/840/design.json: The design adds layer L03 for the merge gate. · why it matters: The gate reads the approval before a build. · the need behind it: No story is built before a person agrees.\n"
	if at := strings.Index(errOut, drift); code != 0 || at < 0 || at > strings.Index(errOut, "/login/device") || !strings.Contains(errOut, "technical design v2, budget") {
		t.Fatalf("approval after technical design v2: exit %d: %s%s", code, out, errOut)
	}
	if a, _ := approved(); a.Version != 2 || !slices.Contains(a.Baselines, base("tech", 2)) {
		t.Fatalf("approval 2: %+v", a)
	}
	history := strings.Fields(git(t, bare, "log", "--format=%H", "ledger", "--", ready.ApprovalPath(840)))
	var first ready.Approval
	if len(history) != 2 || json.Unmarshal([]byte(git(t, bare, "show", history[1]+":"+ready.ApprovalPath(840))), &first) != nil || first.Version != 1 {
		t.Fatalf("the ledger log of approved.json: %v, first %+v", history, first)
	}

	// 7. An open gap, then reviews not in, keep it off; dismissed and every review in, the dismissal is ratified.
	n = commits()
	device("approve", nil)
	crit = ready.Critique{Reviews: 5, In: 2, AskedAt: time.Now().Add(-time.Hour), Open: []string{"tech-v1/quality-architect/1"}}
	off("Approve to build is off: technical design v1 has 1 open gap (tech-v1/quality-architect/1). Next: aayu grill feature 840 --findings")
	dismissal := ready.Dismissal{Finding: "tech-v1/quality-architect/1", Reason: "out of scope for v0.4.0", Words: "Not this release; v0.5.0 takes it."}
	crit.Open, crit.Dismissed = nil, []ready.Dismissal{dismissal}
	off("3 of 5 reviews are not in; Approve to build opens when they are in or [critique] review_wait_hours have passed")
	// Past [critique] review_wait_hours the reviews no longer hold it, and the person sees how many are missing.
	device("deny", nil)
	if code, out, errOut := readyRun(person, "--set", "critique.review_wait_hours=1"); code != ExitBlocked || started() != 1 || commits() != n ||
		!strings.Contains(errOut, "Approving: 3 of 5 reviews are not in, and [critique] review_wait_hours (1) have passed\n") {
		t.Fatalf("past the review wait: exit %d: %s%s", code, out, errOut)
	}
	crit.In = 5
	device("approve", nil)
	code, out, errOut = readyRun(person)
	ratified := "Approving: 1 dismissal ratified: tech-v1/quality-architect/1 (out of scope for v0.4.0)\n"
	if at := strings.Index(errOut, ratified); code != 0 || at < 0 || at > strings.Index(errOut, "/login/device") || strings.Contains(errOut, "reviews are not in") {
		t.Fatalf("approval with the gap dismissed: exit %d: %s%s", code, out, errOut)
	}
	if a, _ := approved(); a.Version != 3 || !slices.Equal(a.Dismissals, []ready.Dismissal{dismissal}) {
		t.Fatalf("approval 3: %+v", a)
	}

	// 4. The UX part is needed (L01 in the footprint) and not approved: everything else holds.
	body := fixtureBody(t, "feature", func(f map[string]string) {
		delete(f, workitems.FieldLinks)
		f["expected_paths"] = "- `packages/design-system/**`\n- `internal/cli/ready.go`"
	})
	edit(func() { feature.Body = body })
	intent := ready.Baseline{Schema: ready.BaselineSchema, Feature: 840, Part: "intent", Version: 2, Owner: "product-strategist",
		Pins: ready.IntentPins(840, body), DraftedBy: "arjun", ApprovedBy: "rahulnakmol"}
	intent.Hash = ready.PinsHash(intent.Pins)
	data, _ := json.Marshal(intent)
	putLedger(t, bare, ready.BaselinePath(840, "intent", 2, false), string(data))
	n = commits()
	device("approve", nil)
	off("Approve to build is off: the UX design is needed (L01) and not approved. Next: aayu ready 840 --part ux --request")
}
