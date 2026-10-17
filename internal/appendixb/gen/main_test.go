package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/appendixb"
)

// gen runs from the repository root: it checks the rows and keeps the plan's
// pointer, and -print renders the appendix.
func TestRun(t *testing.T) {
	root := t.TempDir()
	write := func(file, data string) {
		p := filepath.Join(root, filepath.FromSlash(file))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(appendixb.Plan, "# Plan\n\n"+appendixb.Pointer+"### Deviations recorded while implementing WP1\n\n| # | A |\n|---|---|\n| D-1a | x |\n")
	t.Chdir(root)
	if err := run(false); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(root, appendixb.Dir, "001-wp1", "001-D-1a.md"))
	if err != nil || string(b) != "# D-1a\n\n## A\n\nx\n" {
		t.Fatalf("row file: %q, %v", b, err)
	}
	if err := run(true); err != nil {
		t.Fatal(err)
	}
	write(appendixb.Dir+"/001-wp1/002-D-1a.md", "# D-1a\n\n## A\n\ny\n")
	for _, p := range []bool{false, true} {
		if err := run(p); err == nil || !strings.Contains(err.Error(), "is used twice") {
			t.Fatalf("run(%v) with a duplicate ID: %v", p, err)
		}
	}
}
