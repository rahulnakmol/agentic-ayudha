package seats

import (
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/identity"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

const hand = "# protected\n/.github/  @founder\n/skills/  @founder\n"

func TestRenderCodeowners(t *testing.T) {
	tests := []struct {
		name     string
		existing string
		paired   map[verbs.SeatID]string
		want     string
	}{
		{"no paired seat leaves the file alone", hand, nil, hand},
		{"the only human pairs a seat: layer lines, no re-declarations", hand,
			map[verbs.SeatID]string{verbs.DevOps: "founder"},
			hand + "\n" + BlockBegin + "\n/.github/  @founder\n/.agents/  @founder\n/embed.go  @founder\n/go.mod  @founder\n/go.sum  @founder\n/examples/benefits-qa/deploy/  @founder\n" + BlockEnd + "\n"},
		{"a bot login is not a second human", hand,
			map[verbs.SeatID]string{verbs.Mentalist: "founder", verbs.Auditor: "ci-bot"},
			hand},
		// No hand-written owners: the first seat human owns their layer alone;
		// a second human makes every line name both, so neither is a sole owner.
		{"product repo without CODEOWNERS, two humans", "",
			map[verbs.SeatID]string{verbs.UXResearcher: "ann", verbs.RevenueArchitect: "bob"},
			BlockBegin + "\n/packages/design-system/  @ann @bob\n/examples/benefits-qa/web/  @ann @bob\n/internal/telemetry/  @bob @ann\n/examples/benefits-qa/analytics/  @bob @ann\n" + BlockEnd + "\n"},
		{"re-rendering replaces the old block", hand + "\n" + BlockBegin + "\n/stale/  @zed\n" + BlockEnd + "\n",
			nil, hand},
	}
	for _, tt := range tests {
		got := RenderCodeowners(tt.existing, tt.paired, []string{"ci-bot"})
		if got != tt.want {
			t.Errorf("%s:\n%s\nwant:\n%s", tt.name, got, tt.want)
		}
	}
}

// The protected-paths gate fails closed on any pattern it cannot evaluate, so
// every line seat writes for aayudha's own layers must be root-anchored and literal.
func TestBlockStaysReadableByTheGate(t *testing.T) {
	all := map[verbs.SeatID]string{}
	for i, s := range verbs.Seats {
		all[s.ID] = []string{"ann", "bob"}[i%2]
	}
	co := RenderCodeowners(hand, all, nil)
	res := identity.ProtectedPathsGate(identity.GateInput{Codeowners: co, Changed: []string{"internal/proof/pack.go"}, Author: "ann"})
	if res.Pass || !strings.Contains(strings.Join(res.Lines, "\n"), "FAIL internal/proof/pack.go (/internal/proof/)") {
		t.Errorf("gate on the rendered block:\n%s\n%s", strings.Join(res.Lines, "\n"), co)
	}
}

func TestSetSeat(t *testing.T) {
	base := "version = 1\n\n[proof]\nmedia = \"\"\n"
	added := SetSeat(base, verbs.Auditor, "ann")
	if added != base+"\n[seats.auditor]\nmode = \"paired\"\nhuman = \"ann\"\n" {
		t.Errorf("add:\n%s", added)
	}
	if got := SetSeat(added, verbs.Auditor, ""); got != base {
		t.Errorf("remove does not restore the file:\n%s", got)
	}
	// A table with other keys keeps them and goes back to solo; the next table is untouched.
	withPage := "version = 1\n\n[seats.auditor]\npage = \"#audit\" # Slack\nmode = \"paired\"\nhuman = \"ann\"\n\n[time]\ntz = \"UTC\"\n"
	want := "version = 1\n\n[seats.auditor]\nmode = \"solo\"\npage = \"#audit\" # Slack\n\n[time]\ntz = \"UTC\"\n"
	if got := SetSeat(withPage, verbs.Auditor, ""); got != want {
		t.Errorf("remove with page:\n%s\nwant:\n%s", got, want)
	}
}

func TestReplaceOwner(t *testing.T) {
	body := "### Technical Owner\n\n@Ann\n\n### Links\n\ndepends-on #2"
	if got, ok := ReplaceOwner(body, "ann", "the Auditor twin"); !ok || got != "### Technical Owner\n\nthe Auditor twin\n\n### Links\n\ndepends-on #2" {
		t.Errorf("replace: %v\n%s", ok, got)
	}
	for _, b := range []string{"### Technical Owner\n\n@annie\n", "### Summary\n\n@ann wrote this\n"} {
		if _, ok := ReplaceOwner(b, "ann", "x"); ok {
			t.Errorf("replaced in %q", b)
		}
	}
}
