package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	aayudha "github.com/rahulnakmol/agentic-ayudha"
	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
	"github.com/rahulnakmol/agentic-ayudha/internal/ready"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// Story #1172: each waiting baseline draft of a Feature is an inbox card,
// approved under the session's one device code.

const baselineDesign = `{"feature": 889, "layers": ["L02"], "story_order": [20, 21, 22]}`

// baselineWorld is Feature #889 (expected paths internal/cli/**, so its
// footprint is L02) with Stories #20-#22 under it, #21 with three criteria,
// and its technical design on the ledger. edit changes the issues first.
// An agent lane then drafts the intent, spec, and tech v1 of #889, as
// `aayu ready 889 --part <part> --request` does.
func baselineWorld(t *testing.T, edit func(g *fake.GitHub)) *missionWorld {
	t.Helper()
	m := newMissionWorld(t)
	m.opts.Assets = aayudha.Assets // the twins' names
	m.issue(889, "Agents pull the next story", "feature", 0, "")
	for n := 20; n <= 22; n++ {
		m.issue(n, fmt.Sprintf("Story %d", n), "story", 889, "")
	}
	m.edit(func() {
		f := m.g.Issues[889]
		f.Body = setSection(f.Body, "Expected paths", "- `internal/cli/**`")
		s := m.g.Issues[21]
		_, fields := workitems.Sections(s.Body)
		s.Body = setSection(s.Body, "Completion Goal", fields["Completion Goal"]+"\n3. [api-trace] state: GET /cart; expected: 200 with no items")
		if edit != nil {
			edit(m.g)
		}
	})
	putLedger(t, m.bare, "specs/889/design.json", baselineDesign)
	for _, part := range []string{"intent", "spec", "tech"} {
		m.draftPart(part, "")
	}
	return m
}

// draftPart is the agent lane's `aayu ready 889 --part part --request`.
func (m *missionWorld) draftPart(part, changes string) {
	m.t.Helper()
	args := []string{"ready", "889", "--repo", "o/r", "--part", part, "--request"}
	if changes != "" {
		args = append(args, "--changes", changes)
	}
	if code, out, errOut := run(m.t, m.lane, m.opts, args...); code != 0 {
		m.t.Fatalf("ready 889 --part %s --request: exit %d: %s%s", part, code, out, errOut)
	}
}

// record is the baseline record at path on the ledger, or false when there is none.
func (m *missionWorld) record(path string) (ready.Baseline, bool) {
	m.t.Helper()
	var b ready.Baseline
	if !slices.Contains(strings.Fields(git(m.t, m.bare, "ls-tree", "-r", "--name-only", "ledger", "--", "specs/889/baselines")), path) {
		return b, false
	}
	if err := json.Unmarshal([]byte(git(m.t, m.bare, "show", "ledger:"+path)), &b); err != nil {
		m.t.Fatalf("ledger %s: %v", path, err)
	}
	return b, true
}

func (m *missionWorld) draftOf(part string, v int) ready.Baseline {
	m.t.Helper()
	b, ok := m.record(ready.BaselinePath(889, part, v, true))
	if !ok {
		m.t.Fatalf("no %s v%d draft of #889", part, v)
	}
	return b
}

// Criterion 1: with the intent, spec, and tech v1 drafts of #889 waiting,
// aayu inbox shows three Baseline cards, one per part, each with the part,
// its version, what it pins, and the counsel of the part's owner seat. Opening
// them asks for no code, and a Feature with no L01 has no UX card.
func TestBaselineCards(t *testing.T) {
	m := baselineWorld(t, nil)
	m.device("rahulnakmol")
	code, out, errOut := runIn(t, m.person, m.opts, "sss", "inbox", "--plain", "--repo", "o/r")
	if code != 0 || m.codes() != 0 {
		t.Fatalf("inbox: exit %d, device codes %d:\n%s%s", code, m.codes(), out, errOut)
	}
	m.g.Mu.Lock()
	body := m.g.Issues[889].Body
	m.g.Mu.Unlock()
	design := sha256.Sum256([]byte(baselineDesign))
	intent, spec, tech := m.draftOf("intent", 1), m.draftOf("spec", 1), m.draftOf("tech", 1)
	for _, want := range []string{
		"o/r: 3 need you, @rahulnakmol.\n", // the header counts them
		"[1/3] Baseline #889: Intent v1: Agents pull the next story\n  " + inboxKinds[InboxBaseline].Line + "\n",
		"  Part     intent v1, drafted by ",
		"  Hash     " + intent.Hash[:12] + " is what you approve\n",
		"  Pins\n    #889 body · " + workitems.BodyHash(body)[:12] + "\n",
		"  Arjun (Product Strategist), counsel, not approval: What you agree to: the problem #889 says it solves",
		"  1. Approve intent v1 of #889 at hash " + intent.Hash[:12] + " (recommended)\n  2. Leave it waiting: nothing is frozen\n  3. Open #889 on GitHub\n",
		"[2/3] Baseline #889: Spec v1: Agents pull the next story\n",
		"  Part     spec v1, drafted by ",
		"  Stories  3, each pinned by its title, Completion Goal, effort, layers, and depends-on; and the set of stories\n",
		"  Arjun (Product Strategist), counsel, not approval: What you agree to: 3 stories of #889",
		"  1. Approve spec v1 of #889 at hash " + spec.Hash[:12] + " (recommended)\n",
		"[3/3] Baseline #889: Tech v1: Agents pull the next story\n",
		"  Part     tech v1, drafted by ",
		"  Pins\n    specs/889/design.json · " + hex.EncodeToString(design[:])[:12] + "\n",
		"  Vishwa (Code Architect), counsel, not approval: The contract you freeze: 1 pin of #889, with specs/889/design.json first, at hash " + tech.Hash[:12],
		"  1. Approve tech v1 of #889 at hash " + tech.Hash[:12] + " (recommended)\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("inbox lacks %q", want)
		}
	}
	if strings.Contains(out, "UX v") || strings.Contains(out, "[4/") {
		t.Errorf("a UX card for a Feature with no L01 in its footprint")
	}
	if t.Failed() {
		t.Logf("inbox:\n%s%s", out, errOut)
	}
	// The full-screen inbox lists the same three cards, each with what waits.
	tui := m.opts
	tui.TUI = func(tm tea.Model) error {
		im := tm.(*inboxModel)
		teaDrive(im, im.Init())
		im.Update(tea.WindowSizeMsg{Width: 110, Height: 44})
		view := ansi.Strip(im.View().Content)
		for _, part := range []string{"intent", "spec", "tech"} {
			if !strings.Contains(view, part+" v1 to approve") {
				t.Errorf("the full-screen inbox lacks the %s card:\n%s", part, view)
			}
		}
		return nil
	}
	if code, out, errOut := runIn(t, m.person, tui, "", "inbox", "--repo", "o/r"); code != 0 {
		t.Errorf("full-screen inbox: exit %d: %s%s", code, out, errOut)
	}
	// A repository with no ledger yet has nothing drafted, and the inbox still opens.
	git(t, m.bare, "branch", "-D", "ledger")
	if code, out, errOut := run(t, m.person, m.opts, "inbox", "--list", "--json", "--repo", "o/r"); code != 0 || strings.Contains(out, `"kind": "baseline"`) {
		t.Errorf("inbox with no ledger: exit %d: %s%s", code, out, errOut)
	}
}

// The UX part has a card only when the Feature's footprint has L01 (the
// story's notes): with its design stages on the ledger and a UI path, the UX
// draft is a fourth card with Hanu's counsel; once the Feature's paths leave
// L01, it has none.
func TestBaselineUXCard(t *testing.T) {
	paths := func(globs string) func(g *fake.GitHub) {
		return func(g *fake.GitHub) { g.Issues[889].Body = setSection(g.Issues[889].Body, "Expected paths", globs) }
	}
	m := baselineWorld(t, paths("- `internal/cli/**`\n- `packages/design-system/**`"))
	putLedger(t, m.bare, "design/889/research/manifest.json", `{"stage": "research"}`)
	m.draftPart("ux", "")
	ux := m.draftOf("ux", 1)
	code, out, errOut := runIn(t, m.person, m.opts, "ssss", "inbox", "--plain", "--repo", "o/r")
	if code != 0 || !strings.Contains(out, "[4/4] Baseline #889: UX v1: Agents pull the next story\n") ||
		!strings.Contains(out, "  Pins\n    design/889/research/manifest.json · "+ux.Pins[0].Hash[:12]+"\n") ||
		!strings.Contains(out, "  Hanu (UX Researcher & Evolutionist), counsel, not approval: What you freeze: 1 design record of #889") ||
		!strings.Contains(out, "  1. Approve UX v1 of #889 at hash "+ux.Hash[:12]+" (recommended)\n") {
		t.Fatalf("inbox with a UX draft: exit %d:\n%s%s", code, out, errOut)
	}
	m.edit(func() { paths("- `internal/cli/**`")(m.g) })
	code, out, errOut = runIn(t, m.person, m.opts, "sss", "inbox", "--plain", "--repo", "o/r")
	if code != 0 || strings.Contains(out, "UX v1") || !strings.Contains(out, "[3/3] Baseline #889: Tech v1") {
		t.Fatalf("inbox after the paths left L01: exit %d:\n%s%s", code, out, errOut)
	}
}

// Criterion 2: approving the three cards in one inbox session asks GitHub's
// device flow for one code, and writes intent-v1.json, spec-v1.json, and
// tech-v1.json approved by rahulnakmol, as `aayu ready 889 --part <part>`
// writes them: the draft the person saw, with the approval added.
func TestBaselineApproveOneCode(t *testing.T) {
	m := baselineWorld(t, nil)
	drafts := map[string]ready.Baseline{}
	for _, part := range []string{"intent", "spec", "tech"} {
		drafts[part] = m.draftOf(part, 1)
	}
	m.device("rahulnakmol")
	code, out, errOut := runIn(t, m.person, m.opts, "aaa", "inbox", "--plain", "--repo", "o/r")
	if code != 0 || m.codes() != 1 || strings.Count(errOut, "/login/device") != 1 || !strings.Contains(out, "inbox: 3 done, 0 skipped, 0 left") {
		t.Fatalf("inbox a a a: exit %d, device codes %d:\n%s%s", code, m.codes(), out, errOut)
	}
	for _, part := range []string{"intent", "spec", "tech"} {
		b, ok := m.record(fmt.Sprintf("specs/889/baselines/%s-v1.json", part))
		if !ok || b.ApprovedBy != "rahulnakmol" || b.Presence != "device-flow" || b.ApprovedAt == "" {
			t.Errorf("%s-v1.json: written %v: %+v", part, ok, b)
			continue
		}
		was := b
		was.ApprovedBy, was.ApprovedAt, was.Presence = "", "", ""
		if !reflect.DeepEqual(was, drafts[part]) {
			t.Errorf("%s-v1.json is not the draft the card showed:\n%+v\n%+v", part, b, drafts[part])
		}
		if line := fmt.Sprintf("%s v1 of #889 baselined by @rahulnakmol · hash %s", ready.Title(part), b.Hash[:12]); !strings.Contains(out, line) {
			t.Errorf("inbox lacks %q", line)
		}
	}
	// Nothing waits any more.
	if code, out, errOut := run(t, m.person, m.opts, "inbox", "--list", "--json", "--repo", "o/r"); code != 0 || strings.Contains(out, `"kind": "baseline"`) {
		t.Errorf("inbox --list after the approvals: exit %d: %s%s", code, out, errOut)
	}
}

// Criterion 2, its risk: a card approves only the hash it showed. A draft
// drafted again after the card was read is shown again, not approved; and
// ready --part, pinned to the card's hash, refuses any other.
func TestBaselineApprovesTheHashShown(t *testing.T) {
	m := baselineWorld(t, nil)
	shown := m.draftOf("spec", 1)
	again := shown // the same pins, another draft: it names a fix
	again.Fixes = []string{"spec-v1/quality-architect/1"}
	again.Hash = ready.DraftHash(again.Pins, again.Changes, again.Fixes)
	data, _ := json.Marshal(again)
	lists := 0
	m.edit(func() {
		m.g.Hook = func(method, p string) {
			if method == "GET" && p == "/repos/o/r/issues" {
				if lists++; lists == 2 { // the read again before the write
					putLedger(t, m.bare, ready.BaselinePath(889, "spec", 1, true), string(data))
				}
			}
		}
	})
	m.device("rahulnakmol")
	code, out, errOut := runIn(t, m.person, m.opts, "sa", "inbox", "--plain", "--repo", "o/r")
	if code != 0 || m.codes() != 0 || !strings.Contains(out, "#889 changed on GitHub since you opened it: it was drafted again. Nothing was written; here it is as it is now.") ||
		!strings.Contains(out, "  Hash     "+again.Hash[:12]+" is what you approve\n") {
		t.Fatalf("inbox s a, drafted again: exit %d, device codes %d:\n%s%s", code, m.codes(), out, errOut)
	}
	m.edit(func() { m.g.Hook = nil })
	e, c := m.client(m.person)
	l, err := e.Config()
	if err != nil {
		t.Fatal(err)
	}
	err = e.inboxAct(t.Context(), &l.Config, c, "o/r", "rahulnakmol", InboxItem{Kind: InboxBaseline, Number: 889, Ask: "spec", Hash: shown.Hash},
		InboxAction{N: 1, Command: "aayu ready 889 --part spec"})
	want := fmt.Sprintf("Spec v1 of #889 was drafted again after you read it (hash %s, not %s). Nothing was baselined.", again.Hash[:12], shown.Hash[:12])
	if CodeOf(err) != ExitBlocked || err == nil || !strings.Contains(err.Error(), want) || m.codes() != 0 {
		t.Fatalf("approve the hash shown: %v, device codes %d", err, m.codes())
	}
	if _, ok := m.record("specs/889/baselines/spec-v1.json"); ok {
		t.Fatal("a draft the person did not see was baselined")
	}
}

// A card whose recommendation turned off since it was read (the Feature's
// paths moved, so a story is outside them; the draft hash is the same) is
// shown again, never approved on the stale Enter that skipped the question.
func TestBaselineRecommendationChanged(t *testing.T) {
	m := baselineWorld(t, nil)
	lists := 0
	m.edit(func() {
		m.g.Hook = func(method, p string) {
			if method == "GET" && p == "/repos/o/r/issues" {
				if lists++; lists == 2 { // the read again before the write
					m.g.Issues[889].Body = setSection(m.g.Issues[889].Body, "Expected paths", "- `packages/design-system/**`")
				}
			}
		}
	})
	m.device("rahulnakmol")
	code, out, errOut := runIn(t, m.person, m.opts, "sa", "inbox", "--plain", "--repo", "o/r")
	if code != 0 || m.codes() != 0 || !strings.Contains(out, "#889 changed on GitHub since you opened it: its choices were edited. Nothing was written; here it is as it is now.") {
		t.Fatalf("inbox s a, the recommendation turned off: exit %d, device codes %d:\n%s%s", code, m.codes(), out, errOut)
	}
	if _, ok := m.record("specs/889/baselines/spec-v1.json"); ok {
		t.Fatal("a card whose checks no longer hold was approved on a stale Enter")
	}
}

// Criterion 3: the spec of #889 changes after the card is read, or while the
// device code waits: approving the spec card writes nothing, and the card
// says so in the person's words.
func TestBaselineSpecChanged(t *testing.T) {
	const said = "The spec of #889 changed after the draft. An agent drafts it again."
	retitle := func(g *fake.GitHub) { g.Issues[21].Title = "Story 21, renamed" } // a pinned field of the spec
	t.Run("after the card was read", func(t *testing.T) {
		m := baselineWorld(t, nil)
		n := m.commits()
		m.device("rahulnakmol")
		lists := 0
		m.edit(func() {
			m.g.Hook = func(method, p string) {
				if method == "GET" && p == "/repos/o/r/issues" {
					if lists++; lists == 2 { // the read again before the write
						retitle(m.g)
					}
				}
			}
		})
		code, out, errOut := runIn(t, m.person, m.opts, "sa", "inbox", "--plain", "--repo", "o/r")
		if code != 0 || lists < 2 || m.codes() != 0 ||
			!strings.Contains(out, "#889 changed on GitHub since you opened it: its spec changed after the draft. Nothing was written; here it is as it is now.") ||
			!strings.Contains(out, "\n  "+said+"\n") || !strings.Contains(out, "✗ nothing it pins changed since the draft: an agent drafts it again") ||
			strings.Count(out, "Approve spec v1") != 1 {
			t.Fatalf("inbox s a: exit %d, %d issue lists, device codes %d:\n%s%s", code, lists, m.codes(), out, errOut)
		}
		if _, ok := m.record("specs/889/baselines/spec-v1.json"); ok || m.commits() != n {
			t.Fatalf("a changed spec was baselined: %s commits, was %s", m.commits(), n)
		}
	})
	t.Run("its stories are gone", func(t *testing.T) { // the part has nothing to pin now
		m := baselineWorld(t, nil)
		m.edit(func() { m.g.Issues[889].Subs = nil })
		code, out, errOut := runIn(t, m.person, m.opts, "ss", "inbox", "--plain", "--repo", "o/r")
		if code != 0 || !strings.Contains(out, "  "+inboxKinds[InboxBaseline].Line+"\n  "+said+"\n") || strings.Contains(out, "Approve spec") {
			t.Fatalf("inbox with no stories: exit %d:\n%s%s", code, out, errOut)
		}
	})
	t.Run("while the code waited", func(t *testing.T) {
		m := baselineWorld(t, nil)
		n := m.commits()
		m.edit(func() {
			m.g.Device = &fake.DeviceFlow{ClientID: DefaultDeviceClientID, Approver: "rahulnakmol", Outcome: "approve", OnPoll: func(int) { retitle(m.g) }}
		})
		code, out, errOut := runIn(t, m.person, m.opts, "sa", "inbox", "--plain", "--repo", "o/r")
		if code != 0 || m.codes() != 1 || !strings.Contains(out, "✗ "+said+"\n  Nothing was changed on GitHub.\n") {
			t.Fatalf("inbox s a: exit %d, device codes %d:\n%s%s", code, m.codes(), out, errOut)
		}
		if _, ok := m.record("specs/889/baselines/spec-v1.json"); ok || m.commits() != n {
			t.Fatalf("a spec changed during the code was baselined: %s commits, was %s", m.commits(), n)
		}
		// The next session's card says it, and offers no approval.
		code, out, errOut = runIn(t, m.person, m.opts, "ss", "inbox", "--plain", "--repo", "o/r")
		if code != 0 || !strings.Contains(out, "[2/3] Baseline #889: Spec v1: Agents pull the next story\n  "+inboxKinds[InboxBaseline].Line+"\n  "+said+"\n") ||
			!strings.Contains(out, "  1. Leave it for the agent's new draft (preselected)\n  2. Open #889 on GitHub\n") || strings.Contains(out, "Approve spec") {
			t.Fatalf("inbox after the change: exit %d:\n%s%s", code, out, errOut)
		}
	})
}

// Criterion 4: an agent lane's `aayu inbox --list --json` has the three
// Baseline cards with their checklists, and no approve action; a person's
// list has it. The lane cannot approve another way either.
func TestBaselineAgentLane(t *testing.T) {
	m := baselineWorld(t, nil)
	n := m.commits()
	list := func(env map[string]string) []InboxItem {
		t.Helper()
		code, out, errOut := run(t, env, m.opts, "inbox", "--list", "--json", "--repo", "o/r")
		var l inboxList
		if code != 0 || json.Unmarshal([]byte(out), &l) != nil {
			t.Fatalf("inbox --list --json: exit %d: %s%s", code, out, errOut)
		}
		return slices.DeleteFunc(l.Items, func(it InboxItem) bool { return it.Kind != InboxBaseline })
	}
	approves := func(it InboxItem) bool {
		return slices.ContainsFunc(it.choices(), func(a InboxAction) bool { return strings.HasPrefix(a.Command, "aayu ready") })
	}
	for name, env := range map[string]map[string]string{"AAYU_AGENT=1": m.lane, "an agent marker": m.agent} {
		cards := list(env)
		var parts []string
		for _, it := range cards {
			parts = append(parts, it.Ask)
			if len(it.Checklist) == 0 || approves(it) {
				t.Errorf("%s: the %s card: checklist %v, choices %v", name, it.Ask, it.Checklist, it.choices())
			}
		}
		if !slices.Equal(parts, []string{"intent", "spec", "tech"}) || len(cards[1].Checklist) != 4 {
			t.Errorf("%s: cards %v, spec checklist %v", name, parts, cards[1].Checklist)
		}
	}
	for _, it := range list(m.person) {
		if !approves(it) {
			t.Errorf("a person's list: the %s card has no approve action: %v", it.Ask, it.choices())
		}
	}
	m.device("rahulnakmol")
	for _, args := range [][]string{{"inbox", "--repo", "o/r"}, {"ready", "889", "--part", "spec", "--repo", "o/r"}} {
		if code, out, errOut := runIn(t, m.lane, m.opts, "a", args...); code != ExitBlocked {
			t.Errorf("the lane's %v: exit %d: %s%s", args, code, out, errOut)
		}
	}
	if m.codes() != 0 || m.commits() != n {
		t.Fatalf("the agent lane: device codes %d, %s commits, was %s", m.codes(), m.commits(), n)
	}
}

// Criterion 5: the spec card in `aayu inbox --plain` has one line per story
// with its title, criteria count, and effort, then the checklist: each story
// has criteria, none is XL, and each is in the Feature's paths. A check that
// fails says which story fails it, and the approval is no longer recommended.
func TestBaselineSpecCard(t *testing.T) {
	spec := func(t *testing.T, edit func(g *fake.GitHub)) string {
		m := baselineWorld(t, edit)
		code, out, errOut := runIn(t, m.person, m.opts, "s", "inbox", "--plain", "--repo", "o/r")
		i := strings.Index(out, "[2/3] Baseline #889: Spec v1")
		if code != 0 || i < 0 {
			t.Fatalf("inbox: exit %d:\n%s%s", code, out, errOut)
		}
		return out[i:]
	}
	good := spec(t, nil)
	if want := "  Stories  3, each pinned by its title, Completion Goal, effort, layers, and depends-on; and the set of stories\n" +
		"    #20 Story 20 · 2 criteria · effort S\n" +
		"    #21 Story 21 · 3 criteria · effort S\n" +
		"    #22 Story 22 · 2 criteria · effort S\n" +
		"  Checklist\n" +
		"    ✓ nothing it pins changed since the draft\n" +
		"    ✓ each story has criteria\n" +
		"    ✓ no story is XL\n" +
		"    ✓ each story is in the Feature's paths\n"; !strings.Contains(good, want) || !strings.Contains(good, " (recommended)") {
		t.Errorf("spec card:\n%s\nwant:\n%s", good, want)
	}
	bad := spec(t, func(g *fake.GitHub) {
		g.Issues[20].Body = setSection(g.Issues[20].Body, "Completion Goal", "_No response_")
		for i, l := range g.Issues[20].Labels { // label text is anyone's: it reaches the terminal with no control characters
			if l.Name == "effort:S" {
				g.Issues[20].Labels[i].Name = "effort:S\x1b[2J"
			}
		}
		g.Issues[20].Labels = slices.DeleteFunc(g.Issues[20].Labels, func(l fake.Label) bool { return l.Name == "layer:L02" })
		for i, l := range g.Issues[22].Labels {
			if l.Name == "effort:S" {
				g.Issues[22].Labels[i].Name = "effort:XL"
			}
		}
		for i, l := range g.Issues[21].Labels {
			if l.Name == "layer:L02" {
				g.Issues[21].Labels[i].Name = "layer:L07\a"
			}
		}
	})
	if want := "    #20 Story 20 · 0 criteria · effort S\uFFFD[2J\n" +
		"    #21 Story 21 · 3 criteria · effort S\n" +
		"    #22 Story 22 · 2 criteria · effort XL\n" +
		"  Checklist\n" +
		"    ✓ nothing it pins changed since the draft\n" +
		"    ✗ each story has criteria: #20: no criteria\n" +
		"    ✗ no story is XL: #22: XL\n" +
		"    ✗ each story is in the Feature's paths: #20 names no layer; #21 is in L07\uFFFD, outside L02\n"; !strings.Contains(bad, want) || strings.ContainsAny(bad, "\x1b\a") ||
		!strings.Contains(bad, "  1. Approve spec v1 of #889 at hash ") || strings.Contains(bad, "(recommended)") {
		t.Errorf("spec card with failing checks:\n%s\nwant:\n%s", bad, want)
	}
}

// Criterion 6: a tech v2 draft that waits after an approved v1 shows each
// change since v1 with what changed, why, and the need behind it.
func TestBaselineChangesSinceV1(t *testing.T) {
	m := baselineWorld(t, nil)
	m.device("rahulnakmol")
	if code, out, errOut := runIn(t, m.person, m.opts, "ssa", "inbox", "--plain", "--repo", "o/r"); code != 0 {
		t.Fatalf("approve tech v1: exit %d:\n%s%s", code, out, errOut)
	}
	if _, ok := m.record("specs/889/baselines/tech-v1.json"); !ok {
		t.Fatal("tech v1 was not approved")
	}
	putLedger(t, m.bare, "specs/889/design.json", `{"feature": 889, "layers": ["L02", "L04"], "story_order": [20, 21, 22]}`)
	changes := filepath.Join(t.TempDir(), "changes.json")
	if err := os.WriteFile(changes, []byte(`[{"ref": "specs/889/design.json", "what": "The design adds the L04 test layer.",
		"why": "Each story's checks need a home in the test layer.", "need": "A person sees each story proven before it merges."}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	m.draftPart("tech", changes)
	code, out, errOut := runIn(t, m.person, m.opts, "sss", "inbox", "--plain", "--repo", "o/r")
	i := strings.Index(out, "[3/3] Baseline #889: Tech v2: Agents pull the next story\n")
	want := "  Changes since v1:\n" +
		"    specs/889/design.json: The design adds the L04 test layer.\n" +
		"      Why: Each story's checks need a home in the test layer.\n" +
		"      Need: A person sees each story proven before it merges.\n" +
		"  Checklist\n" +
		"    ✓ nothing it pins changed since the draft\n" +
		"    ✓ each change since v1 says what changed, why, and the need behind it\n"
	if code != 0 || i < 0 || !strings.Contains(out[i:], want) || !strings.Contains(out[i:], "  Part     tech v2, drafted by ") {
		t.Fatalf("tech v2 card: exit %d:\n%s%s\nwant:\n%s", code, out, errOut, want)
	}
}
