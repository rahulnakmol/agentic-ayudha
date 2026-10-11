package aayudha

import (
	"io/fs"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/skills"
)

// TestSkillLint runs the WP3.1 skill lint (internal/skills) over the embedded skills/ tree, which is what
// ships in the binary and what `aayu install` copies. CI runs it in the `build` job. It replaces the WP0.4
// subset (Appendix B D-0.4e).
func TestSkillLint(t *testing.T) {
	if _, err := fs.Stat(Assets, "skills/ready/SKILL.md"); err != nil {
		t.Fatalf("embedded skills tree: %v", err)
	}
	for _, f := range skills.Lint(Assets, "skills") {
		t.Error(f)
	}
}
