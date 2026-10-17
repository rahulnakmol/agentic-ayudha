package chart

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCheckStructureAI: the recorded AI note is complete; removing what each
// part must name, or moving the open questions, makes it incomplete with a
// message naming the gap.
func TestCheckStructureAI(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "evals", "cases", "chart", "ai-scaleup", "output", "note.md"))
	if err != nil {
		t.Fatal(err)
	}
	good := string(b)
	if got := CheckStructure(good, KindAI); len(got) != 0 {
		t.Fatalf("recorded note: %v", got)
	}
	if got := CheckStructure(good, KindData); !strings.Contains(strings.Join(got, "\n"), `missing part "## Decision"`) {
		t.Errorf("an AI note checked as data: %v", got)
	}
	evaluation := good[strings.Index(good, "## Evaluation"):strings.Index(good, "## Open questions")]
	for name, tt := range map[string]struct{ md, want string }{
		"no simpler options": {strings.Replace(good, "## Simpler options", "## Background", 1), `missing part "## Simpler options"`},
		"no permission":      {strings.Replace(strings.NewReplacer("Permissions", "Access", "permissions", "access", "permission", "access").Replace(good), "## Evidence and access", "## Evidence and permissions", 1), "say how retrieval stays within"},
		"no injection":       {strings.ReplaceAll(strings.ReplaceAll(good, "Prompt injection", "Hostile text"), "injection", "attack"), "say how untrusted sources and prompt injection"},
		"no adversarial":     {strings.NewReplacer("Adversarial", "Hard", "adversarial", "hard").Replace(good), "include adversarial cases"},
		"no threshold":       {strings.Replace(good, evaluation, "## Evaluation\n\nExtend the adversarial set and agree thresholds later.\n\n", 1), "numeric threshold"},
		"questions not last": {good + "\n## Rollout\n\nLater.\n", `must end with "## Open questions"`},
		"out of order":       {strings.Replace(strings.Replace(good, "## Abstention", "## Tmp", 1), "## Question\n", "## Abstention\n\nRefer.\n\n## Question\n", 1), `out of order`},
	} {
		if got := strings.Join(CheckStructure(tt.md, KindAI), "\n"); !strings.Contains(got, tt.want) {
			t.Errorf("%s: got %q, want %q", name, got, tt.want)
		}
	}
}
