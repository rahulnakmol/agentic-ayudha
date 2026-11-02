package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
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

// Story #1172 and Bug #1253: the waiting baseline drafts of a Feature are
// one inbox card, and each part opens as its own Baseline card, approved
// under the session's one device code.

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

// Bug #1253 criterion 4 (Story #1172's card): with the intent, spec, and
// tech v1 drafts of #889 waiting, the inbox has one Feature card, and opening
// each part shows the Baseline card of today: the part, its version, what it
// pins, and the counsel of the part's owner seat. Opening them asks for no
// code, and a Feature with no L01 has no UX part.
func TestBaselineCards(t *testing.T) {
	m := baselineWorld(t, nil)
	m.device("rahulnakmol")
	// #889 has no Initiative parent here, so its card approves the parts only: 2-4 open them.
	code, out, errOut := runIn(t, m.person, m.opts, "2s3s4ss", "inbox", "--plain", "--repo", "o/r")
	if code != 0 || m.codes() != 0 {
		t.Fatalf("inbox: exit %d, device codes %d:\n%s%s", code, m.codes(), out, errOut)
	}
	m.g.Mu.Lock()
	body := m.g.Issues[889].Body
	m.g.Mu.Unlock()
	design := sha256.Sum256([]byte(baselineDesign))
	intent, spec, tech := m.draftOf("intent", 1), m.draftOf("spec", 1), m.draftOf("tech", 1)
	for _, want := range []string{
		"o/r: 1 needs you, @rahulnakmol.\n", // one card for the Feature
		"[1/1] Feature #889: Agents pull the next story\n",
		"  2. Open the intent v1 on its own\n  3. Open the spec v1 on its own\n  4. Open the technical design v1 on its own\n",
		"Baseline #889: Intent v1: Agents pull the next story\n  " + inboxKinds[InboxBaseline].Line + "\n",
		"  Part     intent v1, drafted by ",
		"  Hash     " + intent.Hash[:12] + " is what you approve\n",
		"  Pins\n    #889 body · " + workitems.BodyHash(body)[:12] + "\n",
		"  Arjun (Product Strategist), counsel, not approval: What you agree to: the problem #889 says it solves",
		"  1. Approve intent v1 of #889 at hash " + intent.Hash[:12] + " (recommended)\n  2. Leave it waiting: nothing is frozen\n  3. Open #889 on GitHub\n",
		"Baseline #889: Spec v1: Agents pull the next story\n",
		"  Part     spec v1, drafted by ",
		"  Stories  3, each pinned by its title, Completion Goal, effort, layers, and depends-on; and the set of stories\n",
		"  Arjun (Product Strategist), counsel, not approval: What you agree to: 3 stories of #889",
		"  1. Approve spec v1 of #889 at hash " + spec.Hash[:12] + " (recommended)\n",
		"Baseline #889: Tech v1: Agents pull the next story\n",
		"  Part     tech v1, drafted by ",
		"  Pins\n    specs/889/design.json · " + hex.EncodeToString(design[:])[:12] + "\n",
		"  Vishwa (Code Architect), counsel, not approval: The contract you freeze: 1 pin of #889, with specs/889/design.json first, at hash " + tech.Hash[:12],
		"  1. Approve tech v1 of #889 at hash " + tech.Hash[:12] + " (recommended)\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("inbox lacks %q", want)
		}
	}
	if strings.Contains(out, "UX v") || strings.Contains(out, "UX design") {
		t.Errorf("a UX part for a Feature with no L01 in its footprint")
	}
	if t.Failed() {
		t.Logf("inbox:\n%s%s", out, errOut)
	}
	// The full-screen inbox lists the one Feature card; opening a part puts its card under it.
	tui := m.opts
	tui.TUI = func(tm tea.Model) error {
		im := tm.(*inboxModel)
		teaDrive(im, im.Init())
		im.Update(tea.WindowSizeMsg{Width: 110, Height: 44})
		if view := ansi.Strip(im.View().Content); !strings.Contains(view, "3 parts to approve") || strings.Contains(view, "v1 to approve") {
			t.Errorf("the full-screen inbox lacks the Feature card:\n%s", view)
		}
		for n, part := range map[string]string{"2": "intent", "3": "spec", "4": "tech"} {
			im.cur = slices.IndexFunc(im.rows, func(i int) bool { return im.items[i].Kind == InboxFeature })
			teaKeys(im, n, "enter")
			if i := im.item(); i < 0 || im.items[i].key() != "baseline#889/"+part {
				t.Errorf("opening %s: the cursor is on %+v", part, im.items[max(i, 0)])
			}
			if view := ansi.Strip(im.View().Content); !strings.Contains(view, part+" v1 to approve") {
				t.Errorf("the full-screen inbox lacks the opened %s card:\n%s", part, view)
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

// The UX part is on the Feature card only when the Feature's footprint has
// L01 (the story's notes): with its design stages on the ledger and a UI
// path, the UX draft is a fourth part that opens with Hanu's counsel; once the
// Feature's paths leave L01, it is not on the card.
func TestBaselineUXCard(t *testing.T) {
	paths := func(globs string) func(g *fake.GitHub) {
		return func(g *fake.GitHub) { g.Issues[889].Body = setSection(g.Issues[889].Body, "Expected paths", globs) }
	}
	m := baselineWorld(t, paths("- `internal/cli/**`\n- `packages/design-system/**`"))
	putLedger(t, m.bare, "design/889/research/manifest.json", `{"stage": "research"}`)
	m.draftPart("ux", "")
	ux := m.draftOf("ux", 1)
	code, out, errOut := runIn(t, m.person, m.opts, "5ss", "inbox", "--plain", "--repo", "o/r")
	if code != 0 || !strings.Contains(out, "  5. Open the UX design v1 on its own\n") || !strings.Contains(out, "] Baseline #889: UX v1: Agents pull the next story\n") ||
		!strings.Contains(out, "  Pins\n    design/889/research/manifest.json · "+ux.Pins[0].Hash[:12]+"\n") ||
		!strings.Contains(out, "  Hanu (UX Researcher & Evolutionist), counsel, not approval: What you freeze: 1 design record of #889") ||
		!strings.Contains(out, "  1. Approve UX v1 of #889 at hash "+ux.Hash[:12]+" (recommended)\n") {
		t.Fatalf("inbox with a UX draft: exit %d:\n%s%s", code, out, errOut)
	}
	m.edit(func() { paths("- `internal/cli/**`")(m.g) })
	code, out, errOut = runIn(t, m.person, m.opts, "s", "inbox", "--plain", "--repo", "o/r")
	if code != 0 || strings.Contains(out, "UX v1") || strings.Contains(out, "UX design") || !strings.Contains(out, "  Tech v1 · hash ") {
		t.Fatalf("inbox after the paths left L01: exit %d:\n%s%s", code, out, errOut)
	}
}

// Bug #1253 criterion 4: opening each part of the Feature card and approving
// it there, in one inbox session, asks GitHub's device flow for one code, and
// writes intent-v1.json, spec-v1.json, and tech-v1.json approved by
// rahulnakmol, as `aayu ready 889 --part <part>` writes them: the draft the
// person saw, with the approval added. Approving one part writes only it.
func TestBaselineApproveOneCode(t *testing.T) {
	m := baselineWorld(t, nil)
	drafts := map[string]ready.Baseline{}
	for _, part := range []string{"intent", "spec", "tech"} {
		drafts[part] = m.draftOf(part, 1)
	}
	m.device("rahulnakmol")
	// Open the spec and leave it (2), which writes nothing; open it again and approve it, then quit: only the spec is written.
	code, first, errOut := runIn(t, m.person, m.opts, "323aq", "inbox", "--plain", "--repo", "o/r")
	if code != 0 || !strings.Contains(first, "\n    Intent v1 · hash ") || strings.Contains(first[strings.LastIndex(first, "Feature #889"):], "Spec v1 · hash") {
		t.Fatalf("inbox 3 a: exit %d:\n%s%s", code, first, errOut)
	}
	for part, want := range map[string]bool{"intent": false, "spec": true, "tech": false} {
		if _, ok := m.record(fmt.Sprintf("specs/889/baselines/%s-v1.json", part)); ok != want {
			t.Fatalf("after approving the spec alone, %s-v1.json written: %v", part, ok)
		}
	}
	// The intent (2) and then the tech (2 again, the only part left); then
	// #889's Approve to build card says the command's refusal (it has no
	// Initiative parent here), and s leaves it (Story #1173).
	m.device("rahulnakmol")
	code, out, errOut := runIn(t, m.person, m.opts, "2a2as", "inbox", "--plain", "--repo", "o/r")
	out = first + out
	if code != 0 || m.codes() != 1 || strings.Count(errOut, "/login/device") != 1 || !strings.Contains(out, "] Approve to build #889: ") {
		t.Fatalf("inbox 2 a 2 a: exit %d, device codes %d:\n%s%s", code, m.codes(), out, errOut)
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
	code, out, errOut := runIn(t, m.person, m.opts, "3a", "inbox", "--plain", "--repo", "o/r")
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
	code, out, errOut := runIn(t, m.person, m.opts, "3a", "inbox", "--plain", "--repo", "o/r")
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
		code, out, errOut := runIn(t, m.person, m.opts, "3a", "inbox", "--plain", "--repo", "o/r")
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
		// The Feature card says it and approves nothing; the spec's own card says it too (2 opens it: no approval comes first).
		code, out, errOut := runIn(t, m.person, m.opts, "2ss", "inbox", "--plain", "--repo", "o/r")
		if code != 0 || !strings.Contains(out, "\n      "+said+"\n") || strings.Contains(out, "Approve intent") ||
			!strings.Contains(out, "  "+inboxKinds[InboxBaseline].Line+"\n  "+said+"\n") || strings.Contains(out, "Approve spec") {
			t.Fatalf("inbox with no stories: exit %d:\n%s%s", code, out, errOut)
		}
	})
	t.Run("while the code waited", func(t *testing.T) {
		m := baselineWorld(t, nil)
		n := m.commits()
		m.edit(func() {
			m.g.Device = &fake.DeviceFlow{ClientID: DefaultDeviceClientID, Approver: "rahulnakmol", Outcome: "approve", OnPoll: func(int) { retitle(m.g) }}
		})
		code, out, errOut := runIn(t, m.person, m.opts, "3a", "inbox", "--plain", "--repo", "o/r")
		if code != 0 || m.codes() != 1 || !strings.Contains(out, "✗ "+said+"\n  Nothing was changed on GitHub.\n") {
			t.Fatalf("inbox s a: exit %d, device codes %d:\n%s%s", code, m.codes(), out, errOut)
		}
		if _, ok := m.record("specs/889/baselines/spec-v1.json"); ok || m.commits() != n {
			t.Fatalf("a spec changed during the code was baselined: %s commits, was %s", m.commits(), n)
		}
		// The next session's card says it, and offers no approval; the spec's own card (2 opens it now) neither.
		code, out, errOut = runIn(t, m.person, m.opts, "2ss", "inbox", "--plain", "--repo", "o/r")
		if code != 0 || !strings.Contains(out, "] Baseline #889: Spec v1: Agents pull the next story\n  "+inboxKinds[InboxBaseline].Line+"\n  "+said+"\n") ||
			!strings.Contains(out, "  1. Leave it for the agent's new draft (preselected)\n  2. Open #889 on GitHub\n") || strings.Contains(out, "Approve spec") {
			t.Fatalf("inbox after the change: exit %d:\n%s%s", code, out, errOut)
		}
	})
}

// approves reports whether card it, or a card inside it, offers an approval.
func approves(it InboxItem) bool {
	cards := append([]InboxItem{it}, it.Parts...)
	if it.Build != nil {
		cards = append(cards, *it.Build)
	}
	return slices.ContainsFunc(cards, func(c InboxItem) bool {
		return slices.ContainsFunc(c.choices(), func(a InboxAction) bool { return a.Command != inboxLeave && a.Command != "open" })
	})
}

// Bug #1253 criterion 5: an agent lane's `aayu inbox --list`, as JSON and
// as Markdown, has the one Feature card of #889 with each part and its
// checklist, and no approve action on it or on any card inside it; a
// person's list has Approve and start. The lane cannot approve another way.
func TestFeatureCardAgentLane(t *testing.T) {
	m := approveWorld(t)
	n := m.commits()
	for name, env := range map[string]map[string]string{"AAYU_AGENT=1": m.lane, "an agent marker": m.agent} {
		keys, items := m.listed(env)
		if !slices.Equal(keys, []string{"feature#889"}) || approves(items[0]) {
			t.Fatalf("%s: cards %v, the Feature card's choices %+v", name, keys, items)
		}
		var parts []string
		for _, p := range items[0].Parts {
			parts = append(parts, p.Ask)
		}
		if !slices.Equal(parts, []string{"intent", "spec", "tech"}) || len(items[0].Parts[1].Checklist) != 4 || items[0].Build == nil {
			t.Errorf("%s: parts %v, spec checklist %v, build %v", name, parts, items[0].Parts[1].Checklist, items[0].Build)
		}
		code, out, errOut := run(t, env, m.opts, "inbox", "--list", "--repo", "o/r")
		if code != 0 || !strings.Contains(out, "**🧱 Feature [#889](") || !strings.Contains(out, "  Spec v1 · hash ") ||
			regexp.MustCompile(`(?m)^- \*\*\d.*(Approve|Open the)`).MatchString(out) || !strings.Contains(out, "- **6** · Not now: nothing is written\n") {
			t.Errorf("%s: inbox --list: exit %d:\n%s%s", name, code, out, errOut)
		}
	}
	_, items := m.listed(m.person)
	if len(items) != 1 || !approves(items[0]) || items[0].choice(1).Command != featureStart {
		t.Errorf("a person's list: %+v", items)
	}
	m.device("rahulnakmol")
	for _, args := range [][]string{{"inbox", "--repo", "o/r"}, {"ready", "889", "--part", "spec", "--repo", "o/r"}, {"ready", "889", "--repo", "o/r"}} {
		if code, out, errOut := runIn(t, m.lane, m.opts, "1y", args...); code != ExitBlocked {
			t.Errorf("the lane's %v: exit %d: %s%s", args, code, out, errOut)
		}
	}
	if m.codes() != 0 || m.commits() != n {
		t.Fatalf("the agent lane: device codes %d, %s commits, was %s", m.codes(), m.commits(), n)
	}
}

// Bug #1253 criterion 1: with the intent, spec, and tech drafts of #889
// waiting and no Approve to build, `aayu inbox --list --json` has one card
// for #889: each part with the version and hash it was drafted at, and the
// Approve to build it leads to: its stories, budget, agents, and baselines.
// That Approve to build is the one the inbox shows once the parts are
// approved. Reading it asks for no device code.
func TestFeatureCard(t *testing.T) {
	m := approveWorld(t)
	m.device("rahulnakmol")
	keys, items := m.listed(m.person)
	if !slices.Equal(keys, []string{"feature#889"}) {
		t.Fatalf("inbox --list: %v, want one card for #889", keys)
	}
	it := items[0]
	for i, part := range []string{"intent", "spec", "tech"} {
		d := m.draftOf(part, 1)
		if p := it.Parts[i]; p.Kind != InboxBaseline || p.Ask != part || p.Version != 1 || p.Hash != d.Hash || len(it.Parts) != 3 {
			t.Errorf("part %d: %s v%d at %s, want %s v1 at %s", i, p.Ask, p.Version, p.Hash, part, d.Hash)
		}
		if line := fmt.Sprintf("  %s v1 · hash %s · ", ready.Title(part), d.Hash[:12]); !slices.ContainsFunc(it.Lines, func(l string) bool { return strings.HasPrefix(l, line) }) {
			t.Errorf("the card lacks %q: %q", line, it.Lines)
		}
	}
	b := it.Build
	if b == nil || b.Kind != InboxApprove || b.Hash == "" {
		t.Fatalf("the card's Approve to build: %+v", b)
	}
	for _, want := range []string{"Stories  3 stories and 1 Bug, each as it is now", "  #21 Story 21 · story · effort S · MS1",
		"Budget   up to $75 (3 stories × $25, [budget] per_story_usd)", "Agents   up to 4 at once ([build] agents on dev)",
		"Baselines intent v1, spec v1, technical design v1"} {
		if !slices.Contains(b.Lines, want) || !slices.Contains(it.Lines, want) {
			t.Errorf("the Approve to build on the card lacks %q: %q", want, b.Lines)
		}
	}
	if a, o := it.choice(1), it.choice(2); a.Command != featureStart || o.Command != featureApprove || it.recommended() != nil {
		t.Errorf("the card's choices: %+v", it.choices())
	}
	m.approveDrafts("intent", "spec", "tech")
	if keys, after := m.listed(m.person); !slices.Equal(keys, []string{"approve#889"}) || after[0].Hash != b.Hash || !slices.Equal(after[0].Lines, b.Lines) {
		t.Errorf("after the parts are approved: %v, the Approve to build card %+v, the Feature card showed %+v", keys, after, b)
	}
	if m.codes() != 0 {
		t.Errorf("reading the card asked for %d device codes", m.codes())
	}
}

// Bug #1253 criterion 2: Approve and start on #889's card, after one device
// code, writes an approval of each part pinned to the hash the card showed
// and #889's Approve to build in one ledger commit, then its mission start;
// the session ends with the day it set. The full-screen inbox does the same,
// and --dry-run plans every step and writes nothing.
func TestFeatureApproveAndStart(t *testing.T) {
	m := approveWorld(t)
	_, items := m.listed(m.person)
	shown := items[0]
	before := strings.Count(git(t, m.bare, "log", "--format=%s", "ledger"), "\n")
	m.device("rahulnakmol")
	code, out, errOut := runIn(t, m.person, m.opts, "1y", "inbox", "--plain", "--repo", "o/r")
	if code != 0 || m.codes() != 1 || strings.Count(errOut, "/login/device") != 1 || !strings.Contains(out, "Approved #889 to build as @rahulnakmol, approval 1.") ||
		!strings.Contains(out, "\n"+dayIsSet+"\n  #889 · today MS1: #20, #21, #22 · evening review ") || !strings.Contains(out, "inbox: 1 done, 0 skipped, 0 left") {
		t.Fatalf("inbox 1 y: exit %d, device codes %d:\n%s%s", code, m.codes(), out, errOut)
	}
	for i, part := range []string{"intent", "spec", "tech"} {
		b, ok := m.record(fmt.Sprintf("specs/889/baselines/%s-v1.json", part))
		was := b
		was.ApprovedBy, was.ApprovedAt, was.Presence = "", "", ""
		if !ok || b.ApprovedBy != "rahulnakmol" || b.Presence != "device-flow" || b.Hash != shown.Parts[i].Hash || !reflect.DeepEqual(was, m.draftOf(part, 1)) {
			t.Errorf("%s-v1.json: written %v: %+v, the card showed %s", part, ok, b, shown.Parts[i].Hash)
		}
	}
	a, p, ok := m.approvalRecords()
	if !ok || a.By != "rahulnakmol" || len(a.Baselines) != 3 || p.StartedBy != "rahulnakmol" || p.Lock == nil {
		t.Fatalf("approval %v %+v, plan started by %q", ok, a, p.StartedBy)
	}
	// The ledger has the parts and Approve to build in one commit, then the start: newest first.
	log := strings.Split(strings.TrimSpace(git(t, m.bare, "log", "--format=%s", "ledger")), "\n")
	log = log[:len(log)-before]
	build := slices.Index(log, "specs: 3 baselines and Approve to build of #889, approval 1, by @rahulnakmol, and its mission plan")
	started := slices.Index(log, "missions: @rahulnakmol started the mission of #889 on cursor")
	if build < 0 || started < 0 || started >= build || slices.ContainsFunc(log, func(s string) bool { return strings.Contains(s, "baselines of #889, approved") }) {
		t.Fatalf("the ledger's new commits, newest first: %q", log)
	}
	if keys, _ := m.listed(m.person); len(keys) != 0 {
		t.Errorf("after Approve and start the inbox has %v", keys)
	}

	t.Run("full screen", func(t *testing.T) {
		m := approveWorld(t)
		opts := m.opts
		opts.TUI = func(tm tea.Model) error {
			im := tm.(*inboxModel)
			teaDrive(im, im.Init())
			teaKeys(im, "1", "enter", "enter", "q") // not a recommendation: the second Enter confirms it
			return nil
		}
		m.device("rahulnakmol")
		code, out, errOut := runIn(t, m.person, opts, "", "inbox", "--repo", "o/r")
		a, p, ok := m.approvalRecords()
		if code != 0 || m.codes() != 1 || !ok || a.By != "rahulnakmol" || p.StartedBy != "rahulnakmol" || !strings.Contains(out, "\n"+dayIsSet+"\n  #889 · today MS1: ") {
			t.Fatalf("full-screen inbox: exit %d, device codes %d, approved %v, started by %q:\n%s%s", code, m.codes(), ok, p.StartedBy, out, errOut)
		}
		for _, part := range []string{"intent", "spec", "tech"} {
			if b, ok := m.record(fmt.Sprintf("specs/889/baselines/%s-v1.json", part)); !ok || b.ApprovedBy != "rahulnakmol" {
				t.Errorf("%s-v1.json: written %v: %+v", part, ok, b)
			}
		}
	})
	t.Run("dry run", func(t *testing.T) {
		m := approveWorld(t)
		n := m.commits()
		m.device("rahulnakmol")
		m.g.Reset()
		code, out, errOut := runIn(t, m.person, m.opts, "1y", "inbox", "--plain", "--dry-run", "--repo", "o/r")
		if code != 0 || !strings.Contains(out, "--dry-run: the mission start of #889 is planned, not run") || strings.Contains(out, "✗") ||
			m.commits() != n || slices.ContainsFunc(m.g.Writes(), func(w fake.Request) bool { return !strings.HasPrefix(w.Path, "/login/") }) {
			t.Fatalf("inbox --dry-run 1 y: exit %d, %s commits (was %s), writes %v:\n%s%s", code, m.commits(), n, m.g.Writes(), out, errOut)
		}
	})
}

// Bug #1253 criterion 4: a read that fails when the Feature card comes back
// after an opened part ends the inbox with the error, never a silent exit.
func TestFeatureOpenPartRefreshFails(t *testing.T) {
	m := baselineWorld(t, nil)
	lists := 0
	m.edit(func() {
		m.g.Hook = func(method, p string) {
			if method == "GET" && p == "/repos/o/r/issues" {
				if lists++; lists >= 2 { // the read again after the part's card
					m.g.Fail, m.g.FailAfter = "GET /repos/o/r/issues", 0
				}
			}
		}
	})
	code, out, errOut := runIn(t, m.person, m.opts, "32s", "inbox", "--plain", "--repo", "o/r")
	if code == 0 || lists < 2 || !strings.Contains(errOut, "injected failure") {
		t.Fatalf("inbox 3 2, the refresh fails: exit %d, %d issue lists:\n%s%s", code, lists, out, errOut)
	}
}

// A step that fails after the parts and Approve to build of #889 are on the
// ledger says they are there, never that nothing changed.
func TestFeatureApproveFailsAfterTheCommit(t *testing.T) {
	m := approveWorld(t)
	m.device("rahulnakmol")
	m.edit(func() { m.g.Fail = "POST /repos/o/r/issues/889/comments" })
	code, out, errOut := runIn(t, m.person, m.opts, "1y", "inbox", "--plain", "--repo", "o/r")
	if _, _, ok := m.approvalRecords(); code != 0 || !ok || !strings.Contains(out, "Already on GitHub: it approved 3 parts and Approve to build of #889 on the ledger") || strings.Contains(out, "Nothing was changed on GitHub") {
		t.Fatalf("inbox 1 y, a comment fails: exit %d, approved %v:\n%s%s", code, ok, out, errOut)
	}
}

// Bug #1253 criterion 3: a part's pins move after the card is shown, and the
// person chooses Approve and start: nothing is written for #889, no part and
// no approval, and the card says which part changed.
func TestFeatureApprovePinMoved(t *testing.T) {
	const said = "The spec of #889 changed after the draft. An agent drafts it again."
	retitle := func(g *fake.GitHub) { g.Issues[21].Title = "Story 21, renamed" } // a pinned field of the spec
	nothing := func(t *testing.T, m *missionWorld, n string) {
		t.Helper()
		for _, part := range []string{"intent", "spec", "tech"} {
			if _, ok := m.record(fmt.Sprintf("specs/889/baselines/%s-v1.json", part)); ok {
				t.Errorf("%s v1 was approved", part)
			}
		}
		if _, _, ok := m.approvalRecords(); ok || m.commits() != n {
			t.Errorf("approved %v; %s ledger commits, was %s", ok, m.commits(), n)
		}
	}
	t.Run("after the card was read", func(t *testing.T) {
		m := approveWorld(t)
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
		code, out, errOut := runIn(t, m.person, m.opts, "1y", "inbox", "--plain", "--repo", "o/r")
		fresh := out[strings.Index(out, said+" Nothing was written; here it is as it is now.")+1:]
		if code != 0 || lists < 2 || m.codes() != 0 || !strings.Contains(out, "\n  "+said+" Nothing was written; here it is as it is now.\n") ||
			!strings.Contains(fresh, "  Spec v1 · hash ") || !strings.Contains(fresh, "\n      "+said+"\n") || strings.Contains(fresh, "Approve and start") {
			t.Fatalf("inbox 1 y: exit %d, %d issue lists, device codes %d:\n%s%s", code, lists, m.codes(), out, errOut)
		}
		nothing(t, m, n)
	})
	t.Run("while the code waited", func(t *testing.T) {
		m := approveWorld(t)
		n := m.commits()
		m.edit(func() {
			m.g.Device = &fake.DeviceFlow{ClientID: DefaultDeviceClientID, Approver: "rahulnakmol", Outcome: "approve", OnPoll: func(int) { retitle(m.g) }}
		})
		code, out, errOut := runIn(t, m.person, m.opts, "1y", "inbox", "--plain", "--repo", "o/r")
		if code != 0 || m.codes() != 1 || !strings.Contains(out, "✗ "+said+"\n  Nothing was changed on GitHub.\n") {
			t.Fatalf("inbox 1 y: exit %d, device codes %d:\n%s%s", code, m.codes(), out, errOut)
		}
		nothing(t, m, n)
	})
	t.Run("in the run that writes the parts", func(t *testing.T) { // the Approve to build run that writes the parts refuses a moved pin itself
		m := approveWorld(t)
		m.edit(func() { retitle(m.g) })
		n := m.commits()
		m.device("rahulnakmol")
		e, c := m.client(m.person)
		_, tok, err := e.HumanGitHub(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		w, err := e.Ledger(t.Context(), tok, "human", "")
		if err != nil {
			t.Fatal(err)
		}
		drafts, parts := map[string]*ready.Baseline{}, map[string][]byte{}
		for _, part := range []string{"intent", "spec", "tech"} {
			d := m.draftOf(part, 1)
			drafts[part], parts[ready.BaselinePath(889, part, 1, false)] = &d, []byte("{}")
		}
		a := &approveRun{e: e, c: c, w: w, r: &ready.Ready{Repo: "o/r", TechnicalOwner: e.technicalOwner(), GH: c}, n: 889, drafts: drafts, parts: parts}
		if err := a.approve(t.Context()); CodeOf(err) != ExitBlocked || fmt.Sprint(err) != said+" Nothing was written for #889." {
			t.Fatalf("approve with the spec moved: %v", err)
		}
		nothing(t, m, n)
	})
	t.Run("its agents moved", func(t *testing.T) { // [build] agents on dev 4 → 20: the Approve to build is not the card's
		m := approveWorld(t)
		_, items := m.listed(m.person)
		commitOnDev(m.g, ".aayudha/config.toml", "[build]\nagents = 20\n")
		n := m.commits()
		m.device("rahulnakmol")
		e, c := m.client(m.person)
		l, err := e.Config()
		if err != nil {
			t.Fatal(err)
		}
		err = e.inboxAct(t.Context(), &l.Config, c, "o/r", "rahulnakmol", items[0], *items[0].choice(1))
		if want := "The Approve to build of #889 changed while you read it: its budget, agents, or milestones. Nothing was written for #889."; CodeOf(err) != ExitBlocked || fmt.Sprint(err) != want || m.codes() != 0 {
			t.Fatalf("approve and start: %v, device codes %d", err, m.codes())
		}
		nothing(t, m, n)
	})
	t.Run("the start cannot run here", func(t *testing.T) { // no tool for the repository: nothing is approved either
		m := approveWorld(t)
		m.opts.RepoTool = func(context.Context, string) (string, error) { return "", nil }
		n := m.commits()
		m.device("rahulnakmol")
		code, out, errOut := runIn(t, m.person, m.opts, "1y", "inbox", "--plain", "--repo", "o/r")
		if code != 0 || m.codes() != 0 || !strings.Contains(out, "The mission of #889 cannot start here: no entry in [hosts] repos names o/r") {
			t.Fatalf("inbox 1 y with no tool: exit %d, device codes %d:\n%s%s", code, m.codes(), out, errOut)
		}
		nothing(t, m, n)
	})
	t.Run("the tech design moved", func(t *testing.T) {
		m := approveWorld(t)
		_, items := m.listed(m.person)
		putLedger(t, m.bare, "specs/889/design.json", `{"feature": 889, "layers": ["L02", "L04"], "story_order": [20, 21, 22]}`)
		n := m.commits()
		m.device("rahulnakmol")
		e, c := m.client(m.person)
		l, err := e.Config()
		if err != nil {
			t.Fatal(err)
		}
		err = e.inboxAct(t.Context(), &l.Config, c, "o/r", "rahulnakmol", items[0], *items[0].choice(1))
		if want := "The tech of #889 changed after the draft. An agent drafts it again. Nothing was written for #889."; CodeOf(err) != ExitBlocked || fmt.Sprint(err) != want || m.codes() != 0 {
			t.Fatalf("approve and start: %v, device codes %d", err, m.codes())
		}
		nothing(t, m, n)
	})
}

// Criterion 5: the spec card in `aayu inbox --plain` has one line per story
// with its title, criteria count, and effort, then the checklist: each story
// has criteria, none is XL, and each is in the Feature's paths. A check that
// fails says which story fails it, and the approval is no longer recommended.
func TestBaselineSpecCard(t *testing.T) {
	spec := func(t *testing.T, edit func(g *fake.GitHub)) string {
		m := baselineWorld(t, edit)
		code, out, errOut := runIn(t, m.person, m.opts, "3s", "inbox", "--plain", "--repo", "o/r")
		i := strings.Index(out, "] Baseline #889: Spec v1")
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
	if code, out, errOut := runIn(t, m.person, m.opts, "4a", "inbox", "--plain", "--repo", "o/r"); code != 0 {
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
	code, out, errOut := runIn(t, m.person, m.opts, "4ss", "inbox", "--plain", "--repo", "o/r")
	// The Feature card says each change too, with its why and need, before any choice (Bug #1253).
	if want := "  Tech v2 · hash " + m.draftOf("tech", 2).Hash[:12] + " · 2 of 2 checks hold\n" +
		"      Changes since v1:\n        specs/889/design.json: The design adds the L04 test layer.\n" +
		"          Why: Each story's checks need a home in the test layer.\n          Need: A person sees each story proven before it merges.\n"; !strings.Contains(out, want) ||
		strings.Index(out, want) > strings.Index(out, "  1. Approve ") {
		t.Errorf("the Feature card lacks %q before its choices:\n%s", want, out)
	}
	i := strings.Index(out, "] Baseline #889: Tech v2: Agents pull the next story\n")
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
