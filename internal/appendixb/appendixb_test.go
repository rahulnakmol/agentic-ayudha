package appendixb

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/gittest"
)

func TestMain(m *testing.M) { gittest.Main(m) }

const header = "| # | Plan assumption | Finding | Implemented instead |\n|---|---|---|---|\n"

func put(t *testing.T, root, file, data string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(file))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func row(id, a, b, c string) string {
	return "# " + id + "\n\n## Plan assumption\n\n" + a + "\n\n## Finding\n\n" + b + "\n\n## Implemented instead\n\n" + c + "\n"
}

// seed writes a plan whose Appendix B already ends in the pointer, and one
// section with one row.
func seed(t *testing.T, root string) {
	t.Helper()
	put(t, root, Plan, "# Plan\n\n## Appendix B\n\n"+Pointer)
	put(t, root, Dir+"/001-wp1/README.md", "# Deviations recorded while implementing WP1\n\nRows are named by WP.\n\n"+header)
	put(t, root, Dir+"/001-wp1/001-D-1a.md", row("D-1a", "WP1: one table", "Parallel rows collide", "One file per row"))
}

func TestDecisionIDsUnique(t *testing.T) {
	// This repository's own rows load, and no ID is used twice.
	if _, err := Load("../.."); err != nil {
		t.Fatalf("docs/appendix-b does not load:\n%v", err)
	}

	root := t.TempDir()
	seed(t, root)
	put(t, root, Dir+"/002-wp2/README.md", "# Deviations recorded while implementing WP2\n\n"+header)
	put(t, root, Dir+"/002-wp2/001-D-1a.md", row("D-1a", "x", "y", "z"))
	_, err := Load(root)
	want := "row ID D-1a is used twice in Appendix B: docs/appendix-b/001-wp1/001-D-1a.md and docs/appendix-b/002-wp2/001-D-1a.md; give the newer row the next free ID"
	if err == nil || err.Error() != want {
		t.Fatalf("Load with a duplicate ID: got %v, want %q", err, want)
	}
	// go generate refuses it too, so the generate-drift check fails.
	if err := Generate(root, &bytes.Buffer{}, false); err == nil || err.Error() != want {
		t.Fatalf("Generate with a duplicate ID: got %v, want %q", err, want)
	}
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

// Two branches from the same base each add a row to the same section, with the
// same number, and each starts a new section with the same number: both merge
// with no conflict, and the rendered appendix lists everything.
func TestParallelRowsMergeCleanly(t *testing.T) {
	root := t.TempDir()
	seed(t, root)
	git(t, root, "init", "-q", "-b", "main")
	git(t, root, "add", "-A")
	git(t, root, "commit", "-q", "-m", "base")
	for _, b := range []struct{ name, id, sec string }{{"a", "D-1b", "002-wp2"}, {"b", "D-1c", "002-wp3"}} {
		git(t, root, "checkout", "-q", "-b", b.name, "main")
		put(t, root, Dir+"/001-wp1/002-"+b.id+".md", row(b.id, "from "+b.name, "f", "i"))
		put(t, root, Dir+"/"+b.sec+"/README.md", "# Deviations recorded while implementing "+b.sec+"\n\n"+header)
		put(t, root, Dir+"/"+b.sec+"/001-S-"+b.name+".md", row("S-"+b.name, "p", "f", "i"))
		if err := Generate(root, &bytes.Buffer{}, false); err != nil {
			t.Fatal(err)
		}
		git(t, root, "add", "-A")
		git(t, root, "commit", "-q", "-m", "row "+b.id)
	}
	git(t, root, "checkout", "-q", "main")
	git(t, root, "merge", "-q", "--no-edit", "a")
	git(t, root, "merge", "-q", "--no-edit", "b") // fails the test on a conflict
	if err := Generate(root, &bytes.Buffer{}, false); err != nil {
		t.Fatal(err)
	}
	if st := git(t, root, "status", "--porcelain"); st != "" {
		t.Fatalf("go generate changed the merged tree:\n%s", st)
	}
	secs, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	got := Render(secs)
	want := "### Deviations recorded while implementing WP1\n\nRows are named by WP.\n\n" + header +
		"| D-1a | WP1: one table | Parallel rows collide | One file per row |\n" +
		"| D-1b | from a | f | i |\n" +
		"| D-1c | from b | f | i |\n" +
		"\n### Deviations recorded while implementing 002-wp2\n\n" + header + "| S-a | p | f | i |\n" +
		"\n### Deviations recorded while implementing 002-wp3\n\n" + header + "| S-b | p | f | i |\n"
	if got != want {
		t.Fatalf("rendered appendix:\n%s\nwant:\n%s", got, want)
	}
}

// A row written in docs/BUILD-PLAN.md by hand, the old way, moves to its own
// file, and go generate names the new way.
func TestAdoptHandWrittenRows(t *testing.T) {
	root := t.TempDir()
	seed(t, root)
	hand := "\n### Deviations recorded while implementing WP1\n\n" + header +
		"| D-1a | WP1: one table | Parallel rows collide | One file per row |\n" +
		"| D-1b | Links | See [the research](research/x.md) and [a run](https://example.com/r) | #12: `a \\| b` |\n" +
		"\n### Scope addition recorded while adding a thing (WP9)\n\nNew.\n\n" + header +
		"| S-9 | p | f | i |\n"
	put(t, root, Plan, "# Plan\n\n## Appendix B\n\n"+Pointer+hand)

	var out bytes.Buffer
	if err := Generate(root, &out, true); err != nil {
		t.Fatal(err)
	}
	want := "::error file=docs/BUILD-PLAN.md::docs/BUILD-PLAN.md had Appendix B rows written by hand; go generate moved them to their own files. " +
		"Appendix B rows live one file per row in docs/appendix-b/ (AGENTS.md, \"Appendix B rows\"); do not add them to docs/BUILD-PLAN.md. Commit the moved files.\n" +
		"  docs/appendix-b/001-wp1/002-D-1b.md\n" +
		"  docs/appendix-b/002-a-thing-wp9/README.md\n" +
		"  docs/appendix-b/002-a-thing-wp9/001-S-9.md\n"
	if out.String() != want {
		t.Fatalf("output:\n%s\nwant:\n%s", out.String(), want)
	}
	b, _ := os.ReadFile(filepath.Join(root, Plan))
	if string(b) != "# Plan\n\n## Appendix B\n\n"+Pointer {
		t.Fatalf("the plan still holds the hand-written rows:\n%s", b)
	}
	b, _ = os.ReadFile(filepath.Join(root, Dir, "001-wp1", "002-D-1b.md"))
	if want := row("D-1b", "Links", "See [the research](../../research/x.md) and [a run](https://example.com/r)", "\\#12: `a \\| b`"); string(b) != want {
		t.Fatalf("row file:\n%s\nwant:\n%s", b, want)
	}
	secs, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	// The heading was copied without its intro; the section keeps it.
	if got, want := Render(secs), strings.Replace(strings.TrimPrefix(hand, "\n"), "WP1\n\n", "WP1\n\nRows are named by WP.\n\n", 1); got != want {
		t.Fatalf("render does not give back the hand-written rows:\n%s\nwant:\n%s", got, want)
	}

	// A second run has nothing to say.
	out.Reset()
	if err := Generate(root, &out, true); err != nil || out.Len() != 0 {
		t.Fatalf("second run: %v, %q", err, out.String())
	}

	// Editing the generated pointer by hand is undone, and the output says where rows live.
	put(t, root, Plan, "# Plan\n\n## Appendix B\n\n"+strings.Replace(Pointer, "one file per row", "in a table", 1))
	if err := Generate(root, &out, false); err != nil {
		t.Fatal(err)
	}
	if want := "appendixb: docs/BUILD-PLAN.md's Appendix B ends in a generated pointer; go generate restored it. Appendix B rows live one file per row in docs/appendix-b/ (AGENTS.md, \"Appendix B rows\"); do not add them to docs/BUILD-PLAN.md.\n"; out.String() != want {
		t.Fatalf("output:\n%s\nwant:\n%s", out.String(), want)
	}
}

func TestGenerateRefuses(t *testing.T) {
	for _, tc := range []struct{ name, plan, file, data, want string }{
		{"no marker", "# Plan\n", "", "", "docs/BUILD-PLAN.md: missing the"},
		{"hand-written ID used in another section", "", "", "### Deviations recorded while implementing WP2\n\n" + header + "| D-1a | a | b | c |\n",
			"Appendix B row ID D-1a is already used in section \"Deviations recorded while implementing WP1\""},
		{"row without a section", "", "", "| D-2 | a | b | c |\n", "has no \"### \" section"},
		{"short row", "", "", "### Deviations recorded while implementing WP1\n\n" + header + "| D-2 | a |\n", "row \"D-2\" has 2 cells, its table 4"},
		{"bad section name", "", Dir + "/Wp2/README.md", "# x\n\n" + header, "a section directory is named <NNN>-<slug>"},
		{"stray file", "", Dir + "/notes.txt", "x", "only README.md and section directories belong here"},
		{"no title", "", Dir + "/002-wp2/README.md", "x\n\n" + header, "the first line is \"# <section title>\""},
		{"no table", "", Dir + "/002-wp2/README.md", "# x\n\nintro\n", "end with the table's header row"},
		{"no ID column", "", Dir + "/002-wp2/README.md", "# x\n\n| A | B |\n|---|---|\n", "the table's first column is \"#\""},
		{"bad row name", "", Dir + "/001-wp1/D-2.md", row("D-2", "a", "b", "c"), "a row file is named <NNN>-<ID>.md"},
		{"ID differs from name", "", Dir + "/001-wp1/002-D-2.md", row("D-3", "a", "b", "c"), "the first line is \"# D-2\""},
		{"wrong headings", "", Dir + "/001-wp1/002-D-2.md", "# D-2\n\n## Plan assumption\n\na\n", "one \"## <column>\" per column"},
		{"bare pipe", "", Dir + "/001-wp1/002-D-2.md", row("D-2", "a | b", "b", "c"), "the \"Plan assumption\" cell has a \"|\""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			seed(t, root)
			switch {
			case tc.plan != "":
				put(t, root, Plan, tc.plan)
			case tc.file == "" && tc.data != "":
				put(t, root, Plan, "# Plan\n\n"+Pointer+tc.data)
			}
			if tc.file != "" {
				put(t, root, tc.file, tc.data)
			}
			err := Generate(root, &bytes.Buffer{}, false)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, want an error containing %q", err, tc.want)
			}
		})
	}
}
