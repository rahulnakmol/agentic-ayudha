package inspect

import (
	"path"
	"regexp"
	"strconv"
	"strings"
)

// DiffFile is one file of a unified diff, with the new side of its hunks.
type DiffFile struct {
	Path    string // the new path; the old path when the file was deleted
	From    string // the old path of a renamed file
	Deleted bool
	New     bool // the PR adds the file
	Hunks   []Hunk
}

// Hunk is one hunk's new side: its header context (the enclosing function
// git names after the second @@) and its added and context lines.
type Hunk struct {
	NewStart int
	Context  string
	Lines    []DiffLine
	Removed  []string // the old side's removed lines, in order (for weakened-check flags)
}

// DiffLine is a line on the new side of a hunk.
type DiffLine struct {
	N     int
	Text  string
	Added bool
}

var hunkHeader = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,\d+)? @@ ?(.*)$`)

// ParseDiff reads a unified diff (git's format, as GitHub serves a PR's diff).
func ParseDiff(diff string) []DiffFile {
	var out []DiffFile
	var f *DiffFile
	var h *Hunk
	n := 0
	oldPath := ""
	for _, line := range strings.Split(strings.ReplaceAll(diff, "\r\n", "\n"), "\n") {
		switch {
		case strings.HasPrefix(line, "diff --git "):
			out = append(out, DiffFile{})
			f, h = &out[len(out)-1], nil
			if _, b, ok := strings.Cut(line, " b/"); ok {
				f.Path = b
			}
		case f == nil:
		case h == nil && strings.HasPrefix(line, "rename from "):
			f.From = gitPath(strings.TrimPrefix(line, "rename from "))
		case h == nil && strings.HasPrefix(line, "rename to "):
			f.Path = gitPath(strings.TrimPrefix(line, "rename to "))
		case h == nil && strings.HasPrefix(line, "--- "):
			oldPath = strings.TrimPrefix(gitPath(strings.TrimPrefix(line, "--- ")), "a/")
			f.New = oldPath == "/dev/null"
			if !f.New && oldPath != f.Path {
				f.From = oldPath
			}
		case h == nil && strings.HasPrefix(line, "+++ "):
			p := gitPath(strings.TrimPrefix(line, "+++ "))
			if p == "/dev/null" {
				f.Deleted, f.Path = true, oldPath
			} else {
				f.Path = strings.TrimPrefix(p, "b/")
			}
		case strings.HasPrefix(line, "@@"):
			m := hunkHeader.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			n, _ = strconv.Atoi(m[1])
			f.Hunks = append(f.Hunks, Hunk{NewStart: n, Context: strings.TrimSpace(m[2])})
			h = &f.Hunks[len(f.Hunks)-1]
		case h == nil:
		case strings.HasPrefix(line, "+"):
			h.Lines = append(h.Lines, DiffLine{N: n, Text: line[1:], Added: true})
			n++
		case strings.HasPrefix(line, " "):
			h.Lines = append(h.Lines, DiffLine{N: n, Text: line[1:]})
			n++
		case strings.HasPrefix(line, "-"):
			h.Removed = append(h.Removed, line[1:])
		}
	}
	return out
}

// Added is the file's added lines on the new side.
func (f DiffFile) Added() []DiffLine {
	var out []DiffLine
	for _, h := range f.Hunks {
		for _, l := range h.Lines {
			if l.Added {
				out = append(out, l)
			}
		}
	}
	return out
}

// Has reports whether line n is on the new side of one of the file's hunks.
func (f DiffFile) Has(n int) bool {
	for _, h := range f.Hunks {
		for _, l := range h.Lines {
			if l.N == n {
				return true
			}
		}
	}
	return false
}

// Find returns the new-side line where quote starts: its first non-empty line
// matched against the hunks' lines, ignoring surrounding space, preferring an
// added line; 0 when the quote is not in the diff.
func (f DiffFile) Find(quote string) int {
	first := ""
	for _, l := range strings.Split(quote, "\n") {
		if l = strings.TrimSpace(strings.TrimPrefix(l, "+")); l != "" {
			first = l
			break
		}
	}
	if first == "" {
		return 0
	}
	found := 0
	for _, h := range f.Hunks {
		for _, l := range h.Lines {
			if strings.TrimSpace(l.Text) != first {
				continue
			}
			if l.Added {
				return l.N
			}
			if found == 0 {
				found = l.N
			}
		}
	}
	return found
}

// gitPath decodes a path git quoted because it holds non-ASCII or control
// characters ("b/docs/\303\251.md").
func gitPath(p string) string {
	if strings.HasPrefix(p, `"`) {
		if u, err := strconv.Unquote(p); err == nil {
			return u
		}
	}
	return p
}

// isTestPath reports whether p is a test file in a common convention.
func isTestPath(p string) bool {
	b := path.Base(p)
	return strings.HasSuffix(b, "_test.go") || strings.Contains(b, ".test.") || strings.Contains(b, ".spec.") ||
		strings.HasPrefix(b, "test_") || strings.HasSuffix(strings.TrimSuffix(b, path.Ext(b)), "_test")
}

// DocsOnly reports whether every file a diff touches is documentation: a
// file under docs/, a Markdown file at the repository root, or a README.md.
// A steering document (an AGENTS.md or CLAUDE.md, and the sources of truth:
// the build plan, the constraints, the contributing guide, the ethos, the
// glossary, the threat model, the security policy) is never documentation,
// nor is anything under a test fixture directory or a directory whose files
// steer the product or its agents (skills, seats, hosts, work items, evals,
// agent and workflow config); other Markdown, such as a fixture a test loads,
// is not either. A renamed file is documentation only when both its old and
// new paths are. Such a PR merges without a review; any other file needs one.
// An empty diff is not docs-only.
func DocsOnly(files []DiffFile) bool {
	for _, f := range files {
		if !docsPath(f.Path) || (f.From != "" && !docsPath(f.From)) {
			return false
		}
	}
	return len(files) > 0
}

func docsPath(p string) bool {
	switch path.Base(p) {
	case "AGENTS.md", "CLAUDE.md", "BUILD-PLAN.md", "CONSTRAINTS.md", "CONTRIBUTING.md", "ETHOS.md", "GLOSSARY.md", "threat-model.md", "SECURITY.md":
		return false
	}
	for _, d := range []string{"testdata/", "fixtures/", "skills/", "seats/", "hosts/", "work-items/", "evals/", ".agents/", ".claude/", ".github/", ".cursor/"} {
		if strings.HasPrefix(p, d) || strings.Contains(p, "/"+d) {
			return false
		}
	}
	md := strings.HasSuffix(strings.ToLower(p), ".md")
	return strings.HasPrefix(p, "docs/") || md && !strings.Contains(p, "/") || path.Base(p) == "README.md"
}
