package inspect

import (
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/brief"
)

// TestCommentSummaryIsSTE: the sentences inspect writes itself, in its
// comment and its Next lines, keep to the Simplified Technical English limit
// (skills/brief/references/writing.md). Tables and the footer's facts are
// structure, not prose.
func TestCommentSummaryIsSTE(t *testing.T) {
	res := StepBackResult(7, "0123456789abcdef", State{})
	res.Cached, res.Overridden = true, "merge downgraded to fix: the PR description is not complete in \"Why\""
	res.Dropped = []Dropped{{Finding: Finding{ID: "f1", Severity: "P2", File: "a.go", Line: 3, Title: "t", Detail: "d"}, Why: DropPreExisting}}
	res.Weakened = []Weakening{{File: "a_test.go", What: "an assertion removed"}}
	text := Comment(res, State{}, 2)
	d, err := brief.Parse([]byte(text), "")
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, s := range brief.Sentences(d.Units) {
		if strings.HasPrefix(s.Text, "Reviewer") {
			continue // the footer: a line of facts
		}
		n++
		if len(s.Words) > brief.MaxSTEWords {
			t.Errorf("a sentence of %d words (at most %d): %q", len(s.Words), brief.MaxSTEWords, s.Text)
		}
	}
	if n < 8 {
		t.Fatalf("only %d sentences checked; the comment changed shape:\n%s", n, text)
	}
}
