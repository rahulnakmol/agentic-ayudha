// Command gen writes the artifacts derived from the work-item schema
// (internal/workitems, WP2.1-WP2.2). It is run from the repository root by
// `go generate ./...` through the directive in embed.go.
//
// Outputs:
//   - work-items/forms/aayudha-<type>.yml: one GitHub issue form per type; stale forms are deleted
//   - work-items/README.md: only the region between the BEGIN/END GENERATED markers
package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

const formsDir = "work-items/forms"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "workitems gen:", err)
		os.Exit(1)
	}
}

func run() error {
	if _, err := os.Stat("go.mod"); err != nil {
		return errors.New("run from the repository root (go generate ./... does this)")
	}
	want := map[string]bool{}
	for _, t := range workitems.Types {
		name := workitems.FormFile(t)
		want[name] = true
		if err := writeFile(filepath.Join(formsDir, name), workitems.Form(t, workitems.FormOptions{})); err != nil {
			return err
		}
	}
	old, _ := filepath.Glob(filepath.Join(formsDir, "*.yml"))
	for _, f := range old {
		if !want[filepath.Base(f)] {
			if err := os.Remove(f); err != nil {
				return err
			}
		}
	}
	return genReadme()
}

func writeFile(path string, data []byte) error {
	if old, err := os.ReadFile(path); err == nil && bytes.Equal(old, data) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

const (
	begin = "<!-- BEGIN GENERATED: work-items (internal/workitems/gen; DO NOT EDIT) -->"
	end   = "<!-- END GENERATED: work-items -->"
)

func genReadme() error {
	path := "work-items/README.md"
	src, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	i, j := bytes.Index(src, []byte(begin)), bytes.Index(src, []byte(end))
	if i < 0 || j < i {
		return fmt.Errorf("%s: missing %q ... %q markers", path, begin, end)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\nSchema version %d.\n\n", begin, workitems.SchemaVersion)
	b.WriteString("| Type | Parent | Links | Completion Goal | Intake | Form |\n|---|---|---|---|---|---|\n")
	for _, t := range workitems.Types {
		parent := "none"
		if len(t.Parents) > 0 {
			parent = strings.Join(t.Parents, ", ")
			if t.ParentAt != "" {
				parent += " (required at " + string(t.ParentAt) + ")"
			}
		}
		links := "none"
		if len(t.Links) > 0 {
			links = strings.Join(t.Links, ", ")
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | [`%s`](forms/%s) |\n", t.Name, parent, links,
			yes(t.HasCompletionGoal()), yes(t.Intake), workitems.FormFile(t), workitems.FormFile(t))
	}
	b.WriteString("\nLabels (one value per prefix; Priority, Effort, Risk, Health, and Horizon live only in labels):\n\n")
	for _, lf := range workitems.LabelFields {
		fmt.Fprintf(&b, "- %s: `%s:` %s\n", lf.Name, lf.Prefix, strings.Join(lf.Values, ", "))
	}
	b.WriteString("- Fixed: ")
	var fixed []string
	for _, l := range workitems.Labels() {
		if !strings.Contains(l.Name, ":") || strings.HasPrefix(l.Name, "needs:") || strings.HasPrefix(l.Name, "identity:") ||
			strings.HasPrefix(l.Name, "decision:") || strings.HasPrefix(l.Name, "aayu:") {
			fixed = append(fixed, "`"+l.Name+"`")
		}
	}
	b.WriteString(strings.Join(fixed, ", ") + "\n\n")
	out := append(append(append([]byte{}, src[:i]...), b.String()...), src[j:]...)
	return writeFile(path, out)
}

func yes(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
