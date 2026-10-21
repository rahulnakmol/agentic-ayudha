package main

import (
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

// An empty paths list would stop hosts from loading a skill on their own, so the key is written only when
// the registry sets globs (WP3.1).
func TestFrontmatterPaths(t *testing.T) {
	c, _ := verbs.Find("review")
	if fm := frontmatter(c); strings.Contains(fm, "paths") {
		t.Errorf("no registry paths: frontmatter must omit the key:\n%s", fm)
	}
	c.Paths = []string{"web/**/*.tsx", "packages/ui/**"}
	want := "user-invocable: true\npaths:\n  - \"web/**/*.tsx\"\n  - \"packages/ui/**\"\nversion: "
	if fm := frontmatter(c); !strings.Contains(fm, want) {
		t.Errorf("frontmatter lacks %q:\n%s", want, fm)
	}
}

// A skill page lists every step the registry reserves for a person, and names no build-plan reference
// (docs/guide/AGENTS.md); a verb with none says so instead of an empty list.
func TestSkillHuman(t *testing.T) {
	ship, _ := verbs.Find("ship")
	got := skillHuman(ship)
	for _, want := range []string{"own GitHub identity", "never offered to agents", "solo-mode bypass", "`--solo-bypass`: Solo mode only"} {
		if !strings.Contains(got, want) {
			t.Errorf("ship lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Appendix B") || strings.Contains(skillUsage(ship), "Appendix B") {
		t.Errorf("ship names a plan reference:\n%s", got)
	}
	status, _ := verbs.Find("status")
	if got := skillHuman(status); !strings.Contains(got, "reserves no step") {
		t.Errorf("status: %s", got)
	}
}
