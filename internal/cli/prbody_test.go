package cli

import (
	"slices"
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/prbody"
	"github.com/rahulnakmol/agentic-ayudha/internal/ready"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

func storyFixture() (ready.Result, string, []CheckpointRecord) {
	chk := ready.Result{Issue: 42, Title: "Story: Show the cart total", Type: "story",
		Criteria: []workitems.Criterion{{N: 1, Evidence: "screenshot", State: "two items in the cart", Expected: "the total | 30 USD"}}}
	issue := "### Summary\n\nPeople do not see what they pay. The cart must show the total.\n\n### Completion Goal\n\n1. [screenshot] state: two items in the cart; expected: the total\n"
	cps := []CheckpointRecord{{K: 1, N: 2, Layers: []string{"L02"}, Verify: "hold", Commit: "abcdef1234567"}}
	return chk, issue, cps
}

// TestStoryBodyStructure: the body aayu build writes has every section the
// check requires, its own prose passes, and only the placeholders it leaves
// for the agent block, as unfilled.
func TestStoryBodyStructure(t *testing.T) {
	chk, issue, cps := storyFixture()
	body := storyBody(chk, issue, "", nil, cps, "Nakul (claude-opus)")
	block, _ := prbody.Check(body, nil, true)
	var got []string
	for _, p := range block {
		got = append(got, p.Field)
		if !strings.Contains(p.Message, "placeholder") {
			t.Errorf("%s blocks for another reason than its placeholder: %s", p.Field, p.Message)
		}
	}
	if want := []string{"What this does", "What changed", "Risk and rollback"}; !slices.Equal(got, want) {
		t.Fatalf("blocks %v, want the placeholder sections %v:\n%s", got, want, body)
	}
	for _, want := range []string{"**Show the cart total.** People do not see what they pay.", "| Story | #42 |", "| Type | feature |",
		"| Criterion 1 (screenshot) | two items in the cart; expected: the total \\| 30 USD |", "| Checkpoint 1/2 verify (L02) |",
		"Built by: Nakul (claude-opus)", "commit `abcdef1`", "\nRefs #42\n"} {
		if !strings.Contains(body, want) {
			t.Errorf("body lacks %q:\n%s", want, body)
		}
	}
	// Only the Refs line names the story to StoryOf, so an edit of it is seen.
	if n := StoryOf(strings.Replace(body, "Refs #42", "Refs #43", 1), "", ""); n != 43 {
		t.Errorf("StoryOf reads story #%d from a body whose Refs line says #43:\n%s", n, body)
	}
	// Filled in by the agent, it passes.
	filled := strings.ReplaceAll(body, prbody.Placeholder, "The cart shows the total.")
	if block, _ := prbody.Check(filled, nil, true); len(block) > 0 {
		t.Fatalf("a filled body blocks: %v", block)
	}
	// The Story's risk label fills the Risk row; its depends-on links become
	// Dependency rows of a RAID table, owner and state included.
	b := storyBody(chk, issue, "medium", []prDependency{{N: 12, Owner: "@rahulnakmol", Status: "open"}, {N: 13}}, cps, "x")
	for _, want := range []string{"| Risk | medium |", "| Type | Item | Owner | Status or mitigation |",
		"| Dependency | #12 must merge first | @rahulnakmol | open |", "| Dependency | #13 must merge first | unassigned | unknown |"} {
		if !strings.Contains(b, want) {
			t.Errorf("body lacks %q:\n%s", want, b)
		}
	}
	if strings.Contains(body, "| Type | Item |") {
		t.Errorf("a Story without dependencies got an empty RAID table:\n%s", body)
	}
	// A Story's Summary never adds a section or a long sentence to the body,
	// and a label's pipe never breaks the table.
	tricky := "### Summary\n\nThe cart is\n#### What changed\nshort.\n\n" + strings.Repeat("word ", 25) + "end.\n"
	b = storyBody(chk, tricky, "high|now", nil, cps, "x")
	if strings.Count(b, "\n## What changed") != 1 || strings.Contains(b, "\n#### ") {
		t.Errorf("the Summary added a section:\n%s", b)
	}
	if strings.Contains(b, "word word") || !strings.Contains(b, "The cart is #### What changed short.") || !strings.Contains(b, `| Risk | high\|now |`) {
		t.Errorf("Summary or risk cell:\n%s", b)
	}
	filledTricky := strings.ReplaceAll(b, prbody.Placeholder, "The cart shows the total.")
	if block, _ := prbody.Check(filledTricky, nil, true); len(block) > 0 {
		t.Errorf("a filled body from a tricky Summary blocks: %v", block)
	}
	chk.Type = "bug"
	if b := storyBody(chk, issue, "", nil, cps, "x"); !strings.Contains(b, "| Bug | #42 |") || !strings.Contains(b, "| Type | fix |") {
		t.Errorf("a Bug's body:\n%s", b)
	}
}

// TestRefreshSections: a refresh rewrites only aayu's marked sections; the
// agent's text, a person's edit, the session-review block, the declarations,
// and a deleted marker pair stay as they are.
func TestRefreshSections(t *testing.T) {
	chk, issue, cps := storyFixture()
	old := storyBody(chk, issue, "", nil, cps, "Nakul")
	old = strings.Replace(old, "## What changed\n\n"+prbody.Placeholder, "## What changed\n\n- The cart shows the total.", 1)
	old = strings.Replace(old, "| Risk | "+prbody.Placeholder+" |", "| Risk | low |", 1)
	// The agent rewrites Why, which aayu wrote once and never refreshes, and a
	// person removes the checks markers and writes the table themselves.
	old = strings.Replace(old, "This change is for Story #42.", "A person's own reason.", 1)
	i, j := strings.Index(old, "<!-- aayudha:pr:checks -->"), strings.Index(old, "<!-- /aayudha:pr:checks -->")
	old = old[:i] + "| Check or test | What it proves | Result |\n|---|---|---|\n| A person's own check | it works | pass |\n" + old[j+len("<!-- /aayudha:pr:checks -->\n"):]
	// A marker quoted inside a line is not a section.
	old = strings.Replace(old, "## What changed\n\n", "## What changed\n\n- aayu writes `<!-- aayudha:pr:detail -->` around its detail.\n", 1)
	tail := "\n<!-- aayudha:session-review -->\nSession review: 2 findings, both fixed.\n<!-- /aayudha:session-review -->\n\nAayudha-Host: amp\nAayudha-Model: claude-opus\n"
	old += tail

	cps = append(cps, CheckpointRecord{K: 2, N: 2, Layers: []string{"L04"}, Verify: "hold", Commit: "1234567890"})
	got := refreshSections(old, storyBody(chk, issue, "", nil, cps, "Nakul"))
	for _, want := range []string{"- The cart shows the total.", "| Risk | low |", "A person's own reason.", "| A person's own check | it works | pass |",
		"- aayu writes `<!-- aayudha:pr:detail -->` around its detail.", "- Checkpoint 2/2: layers L04", tail} {
		if !strings.Contains(got, want) {
			t.Errorf("refreshed body lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "aayudha:pr:checks") || strings.Contains(got, "| Checkpoint 2/2 verify") || strings.Contains(got, "This change is for") {
		t.Errorf("refresh re-added a deleted section or rewrote the agent's Why:\n%s", got)
	}
	if !strings.HasSuffix(got, tail) {
		t.Errorf("the appended blocks moved:\n%s", got)
	}
	// An open marker whose close was deleted is text: kept, and the refresh ends.
	open := got + "\n<!-- aayudha:pr:detail -->\nA person's note.\n"
	if r := refreshSections(open, storyBody(chk, issue, "", nil, cps, "Nakul")); !strings.HasSuffix(r, "<!-- aayudha:pr:detail -->\nA person's note.\n") {
		t.Errorf("an unclosed marker:\n%s", r)
	}
	if again := refreshSections(got, storyBody(chk, issue, "", nil, cps, "Nakul")); again != got {
		t.Errorf("a second refresh with the same inputs changed the body:\n%s", again)
	}
	// Without the new checkpoint, the marked content goes back byte for byte.
	if back := refreshSections(got, storyBody(chk, issue, "", nil, cps[:1], "Nakul")); back != old {
		t.Errorf("a refresh did not restore the marked sections exactly:\n%s\nwant:\n%s", back, old)
	}
}

// TestBuiltBy: the PR body's "Built by" line reads the checkpoint's own
// trailers: a known model by its name with the effort, an unknown ID as it
// is, and nothing a trailer does not say.
func TestBuiltBy(t *testing.T) {
	for msg, want := range map[string]string{
		"feat: x\n\nAayudha-Model: claude-opus-5-5\nAayudha-Effort: medium\nAayudha-Host: amp":                                 "Claude Opus 5.5, medium effort, on amp",
		"feat: x\n\nAayudha-Model: claude-amp-medium\nAayudha-Host: amp":                                                       "claude-amp-medium, on amp",
		"feat: x\n\nAayudha-Delegate: codex":                                                                                   "model not reported, delegated to codex",
		"feat: x\n\nAayudha-Model: gpt-6.1-sol\nAayudha-Model: claude-sonnet-5.5\nAayudha-Effort: high\nAayudha-Host: copilot": "GPT-6.1 Sol and Claude Sonnet 5.5, high effort, on copilot",
	} {
		if got := builtBy(msg); got != want {
			t.Errorf("builtBy(%q) = %q, want %q", msg, got, want)
		}
	}
}

// TestPassRuleWho: the people ship's Ready to ship text names are the ones
// whose pass allows accepts: on a never layer, only an approver who is also a
// CODEOWNER, and nobody when none is.
func TestPassRuleWho(t *testing.T) {
	never := PassRule{Never: []string{"L03"}, NeverPath: []string{"internal/autonomy/"}, Owners: []string{"Alice", "bob"}}
	withApprover := never
	withApprover.Approvers = []string{"alice"}
	outsider := never
	outsider.Approvers = []string{"carol"}
	at := never // config may write the approver with an "@"
	at.Approvers = []string{"@bob"}
	for _, tc := range []struct {
		rule PassRule
		want string
	}{
		{PassRule{}, "a person other than the author"},
		{PassRule{Approvers: []string{"rahulnakmol"}}, "@rahulnakmol"},
		{never, "@Alice or @bob"},
		{withApprover, "@Alice"},
		{at, "@bob"},
		{outsider, "nobody configured (no approver is a CODEOWNER of internal/autonomy/)"},
	} {
		if got := tc.rule.who(); got != tc.want {
			t.Errorf("%+v: who() = %q, want %q", tc.rule, got, tc.want)
		}
		// Every person named passes allows.
		for _, f := range strings.Fields(strings.ReplaceAll(tc.want, " or ", " ")) {
			if login, ok := strings.CutPrefix(f, "@"); ok {
				if err := tc.rule.allows(login); err != nil {
					t.Errorf("%+v names @%s, whom allows refuses: %v", tc.rule, login, err)
				}
			}
		}
	}
}
