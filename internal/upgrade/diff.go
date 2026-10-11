package upgrade

import (
	"fmt"
	"strings"
)

// Diff renders the changes as a unified diff (3 lines of context), the form
// `aayu upgrade` shows before --apply.
func Diff(changes []Change) string {
	var b strings.Builder
	for _, c := range changes {
		fmt.Fprintf(&b, "--- a/%s (%s schema v%d)\n+++ b/%s (%s schema v%d)\n", c.Path, c.Kind, c.From, c.Path, c.Kind, c.To)
		b.WriteString(unified(splitLines(string(c.Old)), splitLines(string(c.New)), 3))
	}
	return b.String()
}

// Unified renders a unified diff (3 lines of context) between two texts, without
// file headers. `aayu install` uses it to show its plan.
func Unified(old, new string) string { return unified(splitLines(old), splitLines(new), 3) }

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.SplitAfter(s, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

type edit struct {
	op   byte // ' ', '-', '+'
	line string
}

// edits computes a minimal line edit script by longest common subsequence.
func edits(a, b []string) []edit {
	n, m := len(a), len(b)
	lcs := make([][]int, n+1)
	for i := range lcs {
		lcs[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}
	var out []edit
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case a[i] == b[j]:
			out = append(out, edit{' ', a[i]})
			i++
			j++
		case lcs[i+1][j] >= lcs[i][j+1]:
			out = append(out, edit{'-', a[i]})
			i++
		default:
			out = append(out, edit{'+', b[j]})
			j++
		}
	}
	for ; i < n; i++ {
		out = append(out, edit{'-', a[i]})
	}
	for ; j < m; j++ {
		out = append(out, edit{'+', b[j]})
	}
	return out
}

func unified(a, b []string, ctx int) string {
	es := edits(a, b)
	var sb strings.Builder
	for start := 0; start < len(es); {
		// Find the next change.
		for start < len(es) && es[start].op == ' ' {
			start++
		}
		if start == len(es) {
			break
		}
		lo := max(start-ctx, 0)
		// Extend the hunk while changes are within 2*ctx of each other.
		end, last := start, start
		for end < len(es) {
			if es[end].op != ' ' {
				last = end
			} else if end-last > 2*ctx {
				break
			}
			end++
		}
		hi := min(last+ctx+1, len(es))
		// Line numbers at lo.
		aStart, bStart := 1, 1
		for _, e := range es[:lo] {
			if e.op != '+' {
				aStart++
			}
			if e.op != '-' {
				bStart++
			}
		}
		aLen, bLen := 0, 0
		for _, e := range es[lo:hi] {
			if e.op != '+' {
				aLen++
			}
			if e.op != '-' {
				bLen++
			}
		}
		fmt.Fprintf(&sb, "@@ -%d,%d +%d,%d @@\n", aStart, aLen, bStart, bLen)
		for _, e := range es[lo:hi] {
			line := e.line
			if !strings.HasSuffix(line, "\n") {
				line += "\n\\ No newline at end of file\n"
			}
			sb.WriteString(string(e.op) + line)
		}
		start = hi
	}
	return sb.String()
}
