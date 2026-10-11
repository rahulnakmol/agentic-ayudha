package aayudha

import (
	"io/fs"
	"path"
	"regexp"
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

// TestSkillLint is the WP0.4 skill lint over the embedded skills/ tree: frontmatter present, name matches the
// directory, at most 500 lines, and every `aayu <command>` resolves in the registry. WP3.1 replaces it with
// the full lint (hooks, scripts, Mobbin-origin images).
func TestSkillLint(t *testing.T) {
	call := regexp.MustCompile(`\baayu ([a-z][a-z-]*)`)
	n := 0
	err := fs.WalkDir(Assets, "skills", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() != "SKILL.md" {
			return err
		}
		n++
		b, err := fs.ReadFile(Assets, p)
		if err != nil {
			return err
		}
		s := string(b)
		dir := path.Base(path.Dir(p))
		if !strings.HasPrefix(s, "---\n") {
			t.Errorf("%s: must start with YAML frontmatter", p)
		}
		if !strings.Contains(s, "\nname: "+dir+"\n") {
			t.Errorf("%s: frontmatter name must be %q", p, dir)
		}
		if lines := strings.Count(s, "\n"); lines > 500 {
			t.Errorf("%s: %d lines, want <= 500", p, lines)
		}
		for _, m := range call.FindAllStringSubmatch(s, -1) {
			if _, ok := verbs.Find(m[1]); !ok {
				t.Errorf("%s: `aayu %s` is not a registered verb or plumbing command", p, m[1])
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Fatal("no skills/*/SKILL.md found in the embedded tree")
	}
}
