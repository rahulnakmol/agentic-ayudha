package main

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
	"github.com/rahulnakmol/agentic-ayudha/internal/verify"
)

// Every package under internal/ falls in exactly one layer of the generated
// LAYERS.md, so slice can place a story in it and a PR's footprint names it
// (Story #1210, criterion 5). internal/inspect and internal/delegate are in
// L03, beside internal/loops.
func TestEveryInternalPackageInALayer(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	b, err := os.ReadFile(filepath.Join(root, "LAYERS.md"))
	if err != nil {
		t.Fatal(err)
	}
	layers := verify.ParseLayers(string(b))
	for _, g := range []string{"internal/inspect/**", "internal/delegate/**"} {
		if !slices.Contains(layers["L03"], g) {
			t.Errorf("LAYERS.md L03 lacks %s: %v", g, layers["L03"])
		}
	}
	pkgs := map[string]bool{}
	err = filepath.WalkDir(filepath.Join(root, "internal"), func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == "testdata" || strings.HasPrefix(d.Name(), ".") || strings.HasPrefix(d.Name(), "_")) {
			return filepath.SkipDir
		}
		if !d.IsDir() && strings.HasSuffix(p, ".go") {
			rel, _ := filepath.Rel(root, filepath.Dir(p))
			pkgs[filepath.ToSlash(rel)] = true
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) < 50 {
		t.Fatalf("found %d packages under internal/; the walk is broken", len(pkgs))
	}
	for pkg := range pkgs {
		if got := layers.Footprint([]string{path.Join(pkg, "x.go")}); len(got) != 1 {
			t.Errorf("package %s falls in %d layers of LAYERS.md (%v), want exactly one: add its glob to verbs.LayerTable", pkg, len(got), got)
		}
	}
}

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
