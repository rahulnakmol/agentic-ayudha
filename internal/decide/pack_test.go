package decide

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

// TestOwningSeat: the topic picks the twin; a narrower topic wins over the
// delivery default; the issue type decides when no topic word matches.
func TestOwningSeat(t *testing.T) {
	tests := []struct {
		question, typ string
		want          verbs.SeatID
	}{
		{"Should we split #212 or hold PR #219?", "", verbs.EngineeringManager},
		{"Do we add a field to the Tombstone struct or a separate index?", "", verbs.CodeArchitect},
		{"Is the flaky purge test worth quarantining?", "", verbs.QualityArchitect},
		{"Which pricing page should we test for conversion?", "", verbs.RevenueArchitect},
		{"Should we focus the quarter on one bet?", "", verbs.Founder},
		{"Who should do the thing?", "", verbs.EngineeringManager},
		{"What now?", "experiment", verbs.RevenueArchitect},
		{"What now?", "goal", verbs.Founder},
	}
	for _, tt := range tests {
		if got, why := OwningSeat(tt.question, tt.typ); got != tt.want {
			t.Errorf("%q (%s): %s (%s), want %s", tt.question, tt.typ, got, why, tt.want)
		}
	}
}

// TestCheckPack: the recorded EM pack is decidable; dropping any one part, or
// breaking one rule, makes it not decidable with a message naming the gap.
func TestCheckPack(t *testing.T) {
	root := filepath.Join("..", "..", "evals", "cases", "decide")
	g, err := os.ReadFile(filepath.Join(root, "grounding.md"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(root, "em-split-or-replan", "output", "pack.md"))
	if err != nil {
		t.Fatal(err)
	}
	good := string(b)
	if f := CheckPack(good, string(g)); len(f) > 0 {
		t.Fatalf("recorded pack: %v", f)
	}
	for _, s := range PackSections {
		re := regexp.MustCompile(`(?s)## ` + regexp.QuoteMeta(s.Title) + `\n.*?(\n## |\z)`)
		without := re.ReplaceAllString(good, "$1")
		if got := strings.Join(CheckPack(without, string(g)), "\n"); !strings.Contains(got, `missing part "## `+s.Title+`"`) {
			t.Errorf("without %q: %q", s.Title, got)
		}
	}
	tests := []struct{ name, from, to, want string }{
		{"two-sentence question", "or re-planned as one M story while\nPR #219 waits?", "or re-planned as one M story. PR #219 waits?", "the question is 2 sentences"},
		{"one option", "2. **Re-plan as M.**", "   **Re-plan as M.**", "1 options"},
		{"no door", "This is a two-way door", "This is reversible", "one-way or a two-way door"},
		{"decision not last", "## Your decision", "## Your decision\n\nReply re-plan (#212).\n\n## Notes", "must end with"},
		{"uncited claim", "- #231 is blocked by #212.", "- #231 is blocked by #212.\n- The audit reader already accepts unknown fields, so the risk is low.", "uncited claim"},
	}
	for _, tt := range tests {
		if !strings.Contains(good, tt.from) {
			t.Fatalf("%s: fixture lacks %q", tt.name, tt.from)
		}
		got := strings.Join(CheckPack(strings.Replace(good, tt.from, tt.to, 1), string(g)), "\n")
		if !strings.Contains(got, tt.want) {
			t.Errorf("%s: got %q, want %q", tt.name, got, tt.want)
		}
	}
}
