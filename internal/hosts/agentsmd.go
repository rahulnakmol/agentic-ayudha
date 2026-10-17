package hosts

import (
	"strings"
)

// AgentsSection is one aayudha-managed section of a repository's AGENTS.md. ID is
// a stable slug; Body is markdown that starts with its own "## " heading.
//
// Section owners: "work-items" (init, WP2.5), "skills" and "hosts" (install,
// WP3.2), "raci" (WP5.1).
type AgentsSection struct {
	ID   string
	Body string
}

func beginMarker(id string) string { return "<!-- BEGIN aayudha:" + id + " -->" }
func endMarker(id string) string   { return "<!-- END aayudha:" + id + " -->" }

// RenderAgentsMD returns existing with each section placed between its
// "<!-- BEGIN aayudha:<id> -->" and "<!-- END aayudha:<id> -->" lines. A section
// already present is replaced in place; a new one is appended at the end, in
// argument order, after one blank line. Content outside the markers is never
// changed, so a product's own AGENTS.md survives. Empty input starts with
// "# AGENTS.md". The output ends with exactly one newline, and rendering the same
// sections again is byte-identical. Input and output use LF line endings.
func RenderAgentsMD(existing []byte, sections ...AgentsSection) []byte {
	s := string(existing)
	if strings.TrimSpace(s) == "" {
		s = "# AGENTS.md\n"
	}
	for _, sec := range sections {
		block := beginMarker(sec.ID) + "\n" + strings.Trim(sec.Body, "\n") + "\n" + endMarker(sec.ID)
		if i, j, ok := findSection(s, sec.ID); ok {
			s = s[:i] + block + s[j:]
			continue
		}
		s = strings.TrimRight(s, "\n") + "\n\n" + block + "\n"
	}
	return []byte(strings.TrimRight(s, "\n") + "\n")
}

// findSection locates a section's begin marker line start and the end of its
// end marker; both markers must sit on their own lines.
func findSection(s, id string) (start, end int, ok bool) {
	b, e := beginMarker(id), endMarker(id)
	i := lineIndex(s, b, 0)
	if i < 0 {
		return 0, 0, false
	}
	j := lineIndex(s, e, i+len(b))
	if j < 0 {
		return 0, 0, false
	}
	return i, j + len(e), true
}

// lineIndex returns the index of the first occurrence of line at or after from
// that occupies a whole line, or -1.
func lineIndex(s, line string, from int) int {
	for from <= len(s) {
		k := strings.Index(s[from:], line)
		if k < 0 {
			return -1
		}
		k += from
		after := k + len(line)
		if (k == 0 || s[k-1] == '\n') && (after == len(s) || s[after] == '\n') {
			return k
		}
		from = after
	}
	return -1
}

// RemoveAgentsSection returns existing without section id (both markers and
// everything between them) and the blank lines before it; content outside the
// markers is unchanged. uninit (WP2.5) uses it to take back init's section.
func RemoveAgentsSection(existing []byte, id string) []byte {
	s := string(existing)
	i, j, ok := findSection(s, id)
	if !ok {
		return existing
	}
	head := strings.TrimRight(s[:i], "\n")
	tail := strings.TrimLeft(s[j:], "\n")
	if tail != "" {
		head += "\n\n"
	}
	return []byte(strings.TrimRight(head+tail, "\n") + "\n")
}

// wrapText breaks s into lines of at most width bytes at spaces.
func wrapText(s string, width int) string {
	var b strings.Builder
	n := 0
	for i, w := range strings.Fields(s) {
		switch {
		case i == 0:
		case n+1+len(w) > width:
			b.WriteByte('\n')
			n = 0
		default:
			b.WriteByte(' ')
			n++
		}
		b.WriteString(w)
		n += len(w)
	}
	return b.String()
}
