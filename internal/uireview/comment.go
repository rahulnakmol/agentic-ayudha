package uireview

import (
	"fmt"
	"strings"
)

// Marker marks the replace-in-place review comment on a PR.
const Marker = "<!-- aayudha:review -->"

// Comment renders the PR comment for a critique, with polish's comparison
// when there is one.
func Comment(c *Critique, cmp *Comparison) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n### UI review: %s\n\n", Marker, c.Disposition)
	fmt.Fprintf(&b, "Head `%s`, %d state(s). Lane A: **%s** (%s, %d runs per state, a finding counts at %d of %d runs, severity ≥ %d). Lane B: **%s** (%s).\n\n",
		short(c.Head), len(c.States), c.LaneA.Disposition, c.LaneA.Tools, K, Agree, K, MinSeverity, c.LaneB.Disposition, c.LaneB.Tools)
	if cmp != nil {
		fmt.Fprintf(&b, "polish: %d resolved, %d remaining, %d new.\n\n", len(cmp.Resolved), len(cmp.Remaining), len(cmp.New))
	}
	rows := func(l Lane) {
		if len(l.Findings) == 0 && len(l.Errors) == 0 {
			fmt.Fprintf(&b, "Lane %s: no findings.\n\n", l.Name)
			return
		}
		fmt.Fprintf(&b, "| Lane %s | State | Kind | Severity | Element | Finding | Source |\n|---|---|---|---|---|---|---|\n", l.Name)
		for _, f := range l.Findings {
			src := f.Source
			if f.Runs > 0 {
				src = fmt.Sprintf("%s (%d/%d runs)", f.Source, f.Runs, K)
			}
			fmt.Fprintf(&b, "| | %s | %s | %d | %s | %s | %s |\n", cell(f.State), f.Kind, f.Severity, cell(f.Element), cell(f.Summary), cell(src))
		}
		for _, e := range l.Errors {
			fmt.Fprintf(&b, "| | | error | | | %s | |\n", cell(e))
		}
		b.WriteString("\n")
	}
	rows(c.LaneA)
	rows(c.LaneB)
	return b.String()
}

func cell(s string) string {
	s = strings.NewReplacer("|", "\\|", "\n", " ", "\r", " ").Replace(s)
	if len(s) > 160 {
		s = s[:160] + "…"
	}
	return s
}

func short(s string) string {
	if len(s) > 7 {
		return s[:7]
	}
	return s
}
