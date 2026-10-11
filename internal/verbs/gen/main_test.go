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
