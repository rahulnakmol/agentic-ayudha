package initrepo

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/prbody"
)

// TestFormsInstallPRTemplate: the forms step writes the pull request template
// as it writes the issue forms, records it as its own, and keeps a copy a
// person has edited.
func TestFormsInstallPRTemplate(t *testing.T) {
	root := t.TempDir()
	in := &Init{GH: &github.Client{}, Root: root, State: NewState("o/r")}
	if err := in.forms(context.Background()); err != nil {
		t.Fatal(err)
	}
	full := filepath.Join(root, filepath.FromSlash(PRTemplatePath))
	b, err := os.ReadFile(full)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != string(prbody.Template()) {
		t.Fatalf("%s is not the embedded template", PRTemplatePath)
	}
	if !slices.ContainsFunc(in.State.Files, func(f File) bool { return f.Path == PRTemplatePath && f.Created }) {
		t.Fatalf("state does not record %s as created by init: %+v", PRTemplatePath, in.State.Files)
	}
	edited := []byte("## Our own template\n")
	if err := os.WriteFile(full, edited, 0o644); err != nil {
		t.Fatal(err)
	}
	in.Changes = nil
	if err := in.forms(context.Background()); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(full); string(b) != string(edited) {
		t.Fatalf("init overwrote a person's template: %q", b)
	}
	if !slices.Contains(in.Changes, Change{"forms", "file", PRTemplatePath, "kept", "exists with other content; not overwritten"}) {
		t.Errorf("changes %+v do not report the template as kept", in.Changes)
	}
}
