package workitems

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// MinOptions is the fewest options a Decision offers at Ready, so the person
// makes a real choice (founder rule, 2026-10-17). Three is the aim.
const MinOptions = 2

// MaxOptions is the most options a Decision offers, so the inbox picks each
// with one key.
const MaxOptions = 9

// Option is one numbered top-level item of a Decision's Options field,
// "N. **Name.** what it does". N is the Decision's own number for it: the
// inbox, the approval batch, and decide all show and accept it by N.
type Option struct {
	N    int    `json:"n"`
	Name string `json:"name"` // the bold title, or the start of the line when it has none
	Text string `json:"text"` // the rest of the line
	// Recommended marks the option the Decision recommends: its line says
	// "(recommended)", or decide.Options read it from the Recommendation.
	Recommended bool `json:"recommended,omitempty"`
}

var (
	optionLine  = regexp.MustCompile(`^([0-9]+)[.)]\s+(.*)$`)
	optionBold  = regexp.MustCompile(`^\*\*(.+?)\*\*[.:,]?(?:\s+(.*))?$`) // a ** inside a code span, as in ops/**, does not end it
	optionPlain = regexp.MustCompile(`^(.+?)(?:: |\. | — | - )(.*)$`)
	// OptionRef finds an option named by its number in a Recommendation:
	// "Option 2", or a leading "2." or "2".
	OptionRef   = regexp.MustCompile(`(?i)^\s*([0-9]+)(?:[.):,]|\s*$)|\boption\s+([0-9]+)\b`)
	fenceRun    = regexp.MustCompile("^(?:`{3,}|~{3,})")
	recommended = regexp.MustCompile(`(?i)\s*\(recommended\b[^)]*\)`) // "(recommended)" or "(recommended, because …)"
)

// ParseOptions reads a Decision's Options field: its numbered top-level items
// (lines starting "N." or "N)" at the margin; indented trade-off lines are
// not options). It returns them and a problem when there are fewer than
// MinOptions or they are numbered out of order.
func ParseOptions(s string) ([]Option, []Problem) {
	var os []Option
	var ps []Problem
	fence := "" // the open fence's delimiter run; only a run of the same character at least as long closes it
	for _, line := range strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n") {
		t := strings.TrimSpace(line)
		if f := fenceRun.FindString(t); f != "" {
			switch {
			case fence == "":
				fence = f
				continue
			case f[0] == fence[0] && len(f) >= len(fence) && strings.Trim(t, string(f[0])) == "": // a closer carries nothing after its run
				fence = ""
				continue
			}
		}
		if fence != "" {
			continue
		}
		for _, line := range inlineOptions(line) {
			m := optionLine.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			n, _ := strconv.Atoi(m[1])
			if want := len(os) + 1; n != want {
				ps = append(ps, Problem{"options", fmt.Sprintf("option %d is numbered out of order; want %d, so each option keeps the number the person picks it by", n, want)})
			}
			os = append(os, optionOf(n, strings.TrimSpace(m[2])))
		}
	}
	marked := 0
	seen := map[string]int{}
	for _, o := range os {
		if o.Recommended {
			marked++
		}
		if strings.TrimSpace(o.Name) == "" {
			ps = append(ps, Problem{"options", fmt.Sprintf("option %d has no name; start it with its name, \"%d. **Name.** what it does\"", o.N, o.N)})
			continue
		}
		if k := strings.ToLower(o.Name); seen[k] != 0 {
			ps = append(ps, Problem{"options", fmt.Sprintf("options %d and %d are both named %q; give each its own name, so accepting it by name is unambiguous", seen[k], o.N, o.Name)})
		} else {
			seen[k] = o.N
		}
	}
	switch {
	case marked > 1:
		ps = append(ps, Problem{"options", fmt.Sprintf("%d options are marked \"(recommended)\"; mark one, the one the Recommendation names", marked)})
	case len(os) > MaxOptions:
		ps = append(ps, Problem{"options", fmt.Sprintf("%d options; offer at most %d, so the person picks each with one key", len(os), MaxOptions)})
	}
	if len(os) < MinOptions {
		ps = append(ps, Problem{"options", fmt.Sprintf("%d numbered option(s); a Decision offers at least %d, and aims for 3, so the person makes a real choice: "+
			"write each as a top-level line \"N. **Name.** what it does\", including the option of not doing it", len(os), MinOptions)})
	}
	return os, ps
}

// inlineOptions splits a line that numbers its options inline, "1. Keep it.
// 2. Drop it.", at each next number in sequence where a sentence ends before
// it ("phase 2. before launch" is prose); any other line is itself.
func inlineOptions(line string) []string {
	if !strings.HasPrefix(line, "1. ") && !strings.HasPrefix(line, "1) ") {
		return []string{line}
	}
	var out []string
	for k := 2; ; k++ {
		i := strings.Index(line, fmt.Sprintf(" %d%c ", k, line[1]))
		if i < 1 || !strings.ContainsRune(".;", rune(line[i-1])) {
			return append(out, line)
		}
		out, line = append(out, line[:i]), line[i+1:]
	}
}

func optionOf(n int, rest string) Option {
	marked := recommended.MatchString(rest)
	rest = strings.TrimSpace(recommended.ReplaceAllString(rest, ""))
	if m := optionBold.FindStringSubmatch(rest); m != nil {
		name := strings.TrimSpace(recommended.ReplaceAllString(m[1], ""))
		return Option{N: n, Name: strings.TrimRight(name, ".:"), Text: strings.TrimSpace(m[2]), Recommended: marked}
	}
	name, text := rest, ""
	if m := optionPlain.FindStringSubmatch(rest); m != nil {
		name, text = m[1], m[2]
	}
	return Option{N: n, Name: strings.TrimRight(name, ".:"), Text: text, Recommended: marked}
}
