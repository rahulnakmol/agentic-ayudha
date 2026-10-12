package aayudha

import (
	"io/fs"
	"os"
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

// TestNoMobbinOriginImages runs the Mobbin-origin image rule over the whole repository, not only the
// skills tree (WP6.4: lint blocks Mobbin-origin images anywhere). CI runs it in the `build` job.
func TestNoMobbinOriginImages(t *testing.T) {
	for _, f := range skills.MobbinImages(os.DirFS("."), ".", ".git", "node_modules") {
		t.Error(f)
	}
}
