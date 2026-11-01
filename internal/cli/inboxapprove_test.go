package cli

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
	"github.com/rahulnakmol/agentic-ayudha/internal/ready"
)

// Story #1173: Approve to build is an inbox card in the session of the
// baselines, so one device code covers the whole Feature.

// approveWorld is baselineWorld's #889 under Initiative #870, with Bug #30
// under it too, [build] agents 4 on dev, [budget] per_story_usd 25, and a
// fixed clock. Its intent, spec, and tech v1 drafts wait for a person.
func approveWorld(t *testing.T) *missionWorld {
	t.Helper()
	m := baselineWorld(t, nil)
	m.issue(30, "Bug 30", "bug", 889, "")
	m.edit(func() {
		m.g.Issues[889].Parent = 870
		m.g.Issues[870].Subs = append(m.g.Issues[870].Subs, 889)
	})
	commitOnDev(m.g, ".aayudha/config.toml", "[build]\nagents = 4\n")
	for _, env := range []map[string]string{m.person, m.agent, m.lane} {
		env["AAYU_BUDGET__PER_STORY_USD"] = "25"
	}
	at := time.Date(2026, 11, 1, 7, 30, 0, 0, time.UTC)
	m.opts.Now = func() time.Time { return at }
	return m
}

// approveDrafts approves the v1 drafts of parts by hand, as missionWorld.feature does.
func (m *missionWorld) approveDrafts(parts ...string) {
	m.t.Helper()
	for _, part := range parts {
		b := m.draftOf(part, 1)
		b.ApprovedBy, b.ApprovedAt, b.Presence = "rahulnakmol", "2026-10-26T00:00:00Z", "device-flow"
		data, _ := json.Marshal(b)
		putLedger(m.t, m.bare, ready.BaselinePath(889, part, 1, false), string(data))
	}
}

// listed is the inbox --list --json of env: each card's key.
func (m *missionWorld) listed(env map[string]string) ([]string, []InboxItem) {
	m.t.Helper()
	code, out, errOut := run(m.t, env, m.opts, "inbox", "--list", "--json", "--repo", "o/r")
	var l inboxList
	if code != 0 || json.Unmarshal([]byte(out), &l) != nil {
		m.t.Fatalf("inbox --list --json: exit %d: %s%s", code, out, errOut)
	}
	var keys []string
	for _, it := range l.Items {
		keys = append(keys, it.key())
	}
	return keys, l.Items
}

// approvalRecords are specs/889/approved.json and missions/889/plan.json,
// or false when no approval is written.
func (m *missionWorld) approvalRecords() (ready.Approval, MissionPlan, bool) {
	m.t.Helper()
	var a ready.Approval
	if !strings.Contains(git(m.t, m.bare, "ls-tree", "-r", "--name-only", "ledger", "--", "specs/889"), ready.ApprovalPath(889)) {
		return a, MissionPlan{}, false
	}
	if err := json.Unmarshal([]byte(git(m.t, m.bare, "show", "ledger:"+ready.ApprovalPath(889))), &a); err != nil {
		m.t.Fatal(err)
	}
	p, _ := m.plan(889)
	return a, p, true
}

// Criterion 1: with the intent, spec, and tech of #889 approved and no
// approval, the inbox has an Approve to build card with its stories and
// Bugs, the budget (3 stories × $25), the agents, the milestones, and its
// high-risk entries by story, read with no device code.
func TestApproveCard(t *testing.T) {
	m := approveWorld(t)
	m.approveDrafts("intent", "spec", "tech")
	m.device("rahulnakmol")
	code, out, errOut := runIn(t, m.person, m.opts, "s", "inbox", "--plain", "--repo", "o/r")
	if code != 0 || m.codes() != 0 {
		t.Fatalf("inbox: exit %d, device codes %d:\n%s%s", code, m.codes(), out, errOut)
	}
	for _, want := range []string{
		"[1/1] Approve to build #889: Agents pull the next story\n  " + inboxKinds[InboxApprove].Line + "\n",
		"  Stories  3 stories and 1 Bug, each as it is now\n" +
			"    #20 Story 20 · story · effort S · MS1\n" +
			"    #21 Story 21 · story · effort S · MS1\n" +
			"    #22 Story 22 · story · effort S · MS1\n" +
			"    #30 Bug 30 · Bug · effort S · MS1\n",
		"  Budget   up to $75 (3 stories × $25, [budget] per_story_usd)\n",
		"  Agents   up to 4 at once ([build] agents on dev)\n",
		"  Milestones MS1 #20, #21, #22, #30; complete when MS1 passes its gate\n",
		"  High-risk entries by story: none shown. They come with #1029, which has not merged, so the card cannot read them yet.\n",
		"  Baselines intent v1, spec v1, technical design v1\n",
		"  Arjun (Product Strategist), counsel, not approval: What you agree to: the 3 stories and 1 Bug of #889 on the card",
		"  1. Approve #889 to build: 3 stories and 1 Bug, budget up to $75, up to 4 at once (recommended)\n" +
			"  2. Approve it with another number of agents\n  3. Leave it: nothing is approved\n  4. Open #889 on GitHub\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("inbox lacks %q", want)
		}
	}
	if t.Failed() {
		t.Logf("inbox:\n%s%s", out, errOut)
	}
	// An agent lane reads the card and has no approve action.
	_, items := m.listed(m.lane)
	i := slices.IndexFunc(items, func(it InboxItem) bool { return it.Kind == InboxApprove })
	if i < 0 || slices.ContainsFunc(items[i].choices(), func(a InboxAction) bool { return strings.HasPrefix(a.Command, "aayu ready") }) {
		t.Errorf("the agent lane's Approve to build card: %+v", items)
	}
}

// Criterion 5: while a baseline of #889 is not approved, the inbox has its
// Baseline cards and no Approve to build card; the card comes once the last
// one is approved.
func TestApproveCardWaitsForBaselines(t *testing.T) {
	m := approveWorld(t)
	for _, c := range []struct {
		approve []string
		want    []string
	}{
		{nil, []string{"baseline#889/intent", "baseline#889/spec", "baseline#889/tech"}},
		{[]string{"intent", "spec"}, []string{"baseline#889/tech"}},
		{[]string{"tech"}, []string{"approve#889"}},
	} {
		m.approveDrafts(c.approve...)
		if keys, _ := m.listed(m.person); !slices.Equal(keys, c.want) {
			t.Fatalf("approved %v: the inbox has %v, want %v", c.approve, keys, c.want)
		}
	}
}

// Criterion 2: approving the three Baseline cards and then the Approve to
// build card in one inbox session asks for one device code, and writes
// approved.json and plan.json as `aayu ready 889` writes them.
func TestApproveCardOneCode(t *testing.T) {
	m := approveWorld(t)
	m.device("rahulnakmol")
	code, out, errOut := runIn(t, m.person, m.opts, "aaaa", "inbox", "--plain", "--repo", "o/r")
	if code != 0 || m.codes() != 1 || strings.Count(errOut, "/login/device") != 1 || !strings.Contains(out, "[4/4] Approve to build #889: ") ||
		!strings.Contains(out, "Approved #889 to build as @rahulnakmol, approval 1.") || !strings.Contains(out, "inbox: 4 done, 0 skipped, 0 left") {
		t.Fatalf("inbox a a a a: exit %d, device codes %d:\n%s%s", code, m.codes(), out, errOut)
	}
	got, gotPlan, ok := m.approvalRecords()
	if !ok || got.By != "rahulnakmol" || got.Agents != 0 || gotPlan.Agents != 4 || len(got.Stories) != 3 || len(gotPlan.Graph) != 4 {
		t.Fatalf("the card's approval: %v %+v %+v", ok, got, gotPlan)
	}

	// The command, in a world where the person approved the baselines in the inbox and left the card.
	cmd := approveWorld(t)
	cmd.device("rahulnakmol")
	if code, out, errOut := runIn(t, cmd.person, cmd.opts, "aaa3", "inbox", "--plain", "--repo", "o/r"); code != 0 {
		t.Fatalf("inbox a a a, leave: exit %d:\n%s%s", code, out, errOut)
	}
	if _, _, ok := cmd.approvalRecords(); ok {
		t.Fatal("leaving the card approved #889")
	}
	if code, out, errOut := cmd.approve(889); code != 0 {
		t.Fatalf("aayu ready 889: exit %d:\n%s%s", code, out, errOut)
	}
	want, wantPlan, _ := cmd.approvalRecords()
	got.Presence.ActionsRecord, want.Presence.ActionsRecord = "", "" // the hash of this machine's own presence record
	if !reflect.DeepEqual(got, want) {
		t.Errorf("approved.json from the card is not the command's:\n%+v\n%+v", got, want)
	}
	if missionJSON(&gotPlan) != missionJSON(&wantPlan) {
		t.Errorf("plan.json from the card is not the command's:\n%s\n%s", missionJSON(&gotPlan), missionJSON(&wantPlan))
	}
}

// Criterion 3: agents set to 10 on the card are the approval's agents, as
// `aayu ready 889 --agents 10` sets them.
func TestApproveCardAgents(t *testing.T) {
	m := approveWorld(t)
	m.approveDrafts("intent", "spec", "tech")
	m.device("rahulnakmol")
	code, out, errOut := runIn(t, m.person, m.opts, "2x\n", "inbox", "--plain", "--repo", "o/r")
	if code != 0 || !strings.Contains(out, "not a number of agents") {
		t.Fatalf("inbox 2 x: exit %d:\n%s%s", code, out, errOut)
	}
	if _, _, ok := m.approvalRecords(); ok {
		t.Fatal("an answer that is not a number approved #889")
	}
	code, out, errOut = runIn(t, m.person, m.opts, "210\n", "inbox", "--plain", "--repo", "o/r")
	a, p, ok := m.approvalRecords()
	if code != 0 || !ok || a.Agents != 10 || p.Agents != 10 || p.AgentsSource != "aayu ready 889 --agents 10 at Approve to build" {
		t.Fatalf("inbox 2 10: exit %d, approved %v: agents %d, plan %d (%s):\n%s%s", code, ok, a.Agents, p.Agents, p.AgentsSource, out, errOut)
	}
}

// Criterion 4: a story of #889 that changed after the card was read is not
// approved, and the inbox says so; the stories are read again after the code.
func TestApproveCardStoriesChanged(t *testing.T) {
	const said = "The stories of #889 changed while you read it."
	t.Run("after the card was read", func(t *testing.T) {
		m := approveWorld(t)
		m.approveDrafts("intent", "spec", "tech")
		n := m.commits()
		lists := 0
		m.edit(func() {
			m.g.Hook = func(method, p string) {
				if method == "GET" && p == "/repos/o/r/issues" {
					if lists++; lists == 2 { // the read again before the write
						m.g.Issues[21].Title = "Story 21, renamed"
					}
				}
			}
		})
		m.device("rahulnakmol")
		code, out, errOut := runIn(t, m.person, m.opts, "a", "inbox", "--plain", "--repo", "o/r")
		if code != 0 || lists < 2 || m.codes() != 0 || !strings.Contains(out, "\n  "+said+" Nothing was written; here it is as it is now.\n") ||
			!strings.Contains(out, "Approve to build is off: the spec changed after v1 was approved") {
			t.Fatalf("inbox a: exit %d, %d issue lists, device codes %d:\n%s%s", code, lists, m.codes(), out, errOut)
		}
		if _, _, ok := m.approvalRecords(); ok || m.commits() != n {
			t.Fatalf("stories the person did not see were approved: %s commits, was %s", m.commits(), n)
		}
	})
	t.Run("while the code waited", func(t *testing.T) {
		m := approveWorld(t)
		m.approveDrafts("intent", "spec", "tech")
		n := m.commits()
		m.edit(func() {
			m.g.Device = &fake.DeviceFlow{ClientID: DefaultDeviceClientID, Approver: "rahulnakmol", Outcome: "approve",
				OnPoll: func(int) { m.g.Issues[21].Title = "Story 21, renamed" }}
		})
		code, out, errOut := runIn(t, m.person, m.opts, "a", "inbox", "--plain", "--repo", "o/r")
		if code != 0 || m.codes() != 1 || !strings.Contains(out, "✗ "+said+"\n  Nothing was changed on GitHub.\n") {
			t.Fatalf("inbox a: exit %d, device codes %d:\n%s%s", code, m.codes(), out, errOut)
		}
		if _, _, ok := m.approvalRecords(); ok || m.commits() != n {
			t.Fatalf("stories changed during the code were approved: %s commits, was %s", m.commits(), n)
		}
	})
	// A Bug added after the card was read changes no baseline, so only the
	// card's pin, checked after the code, holds the approval back.
	t.Run("read again after the code", func(t *testing.T) {
		m := approveWorld(t)
		m.approveDrafts("intent", "spec", "tech")
		n := m.commits()
		_, items := m.listed(m.person)
		card := items[slices.IndexFunc(items, func(it InboxItem) bool { return it.Kind == InboxApprove })]
		m.issue(31, "Bug 31", "bug", 889, "")
		m.device("rahulnakmol")
		e, c := m.client(m.person)
		l, err := e.Config()
		if err != nil {
			t.Fatal(err)
		}
		err = e.inboxAct(t.Context(), &l.Config, c, "o/r", "rahulnakmol", card, *card.recommended())
		if CodeOf(err) != ExitBlocked || !strings.Contains(fmt.Sprint(err), said+" Nothing was approved.") || m.codes() != 1 {
			t.Fatalf("approve the card's stories: %v, device codes %d", err, m.codes())
		}
		if _, _, ok := m.approvalRecords(); ok || m.commits() != n {
			t.Fatalf("a Bug the person did not see was approved: %s commits, was %s", m.commits(), n)
		}
	})
}
