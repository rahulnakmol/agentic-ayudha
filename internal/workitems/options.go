package workitems

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
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
	// Letter is the option's letter in a Decision written before options were
	// numbered, "A: in-house." or "waive: …" (A, B, … by position when the
	// line has none). N is still its number, so A is 1 and B is 2.
	Letter string `json:"letter,omitempty"`
}

// Ref is the option's number as every surface shows it: "2", or "2 (B)" for a
// lettered option.
func (o Option) Ref() string {
	if o.Letter == "" {
		return strconv.Itoa(o.N)
	}
	return fmt.Sprintf("%d (%s)", o.N, o.Letter)
}

// Lettered reports whether os were read from lettered or named lines, not
// numbered ones: the Decision should be rewritten as numbered lines.
func Lettered(os []Option) bool { return len(os) > 0 && os[0].Letter != "" }

var (
	optionLine  = regexp.MustCompile(`^([0-9]+)[.)]\s+(.*)$`)
	optionBold  = regexp.MustCompile(`^\*\*(.+?)\*\*[.:,]?(?:\s+(.*))?$`) // a ** inside a code span, as in ops/**, does not end it
	optionPlain = regexp.MustCompile(`^(.+?)(?:: |\. | — | - )(.*)$`)
	openingRef  = regexp.MustCompile(`(?i)^(?:option\s+([0-9]+)\b|([0-9]+)(?:[.):,\s]|$))`)
	fenceRun    = regexp.MustCompile("^(?:`{3,}|~{3,})")
	recommended = regexp.MustCompile(`(?i)\s*\(recommended\b[^)]*\)`) // "(recommended)" or "(recommended, because …)"
	// A Decision written before options were numbered (Lettered): "A: x",
	// "(B) y", "**C.** z", a top-level bullet, or a named line "waive: …".
	letterLine = regexp.MustCompile(`^(?:[-*]\s+)?\**\(?([A-I])[.:)]\)?\**\s+(.*)$`)
	bulletLine = regexp.MustCompile(`^[-*]\s+(.*)$`)
	namedLine  = regexp.MustCompile(`^\**[A-Za-z][A-Za-z0-9_-]{0,30}\**:\s`)
	letterRef  = regexp.MustCompile(`^(?:(?i:option)\s+([A-I])\b|([A-I])(?:[.):,]|$))`)
)

// ParseOptions reads a Decision's Options field: its numbered top-level items
// (lines starting "N." or "N)" at the margin; indented trade-off lines are
// not options). With none numbered, it reads lettered or named top-level
// lines as options 1, 2, … in order (Lettered). It returns them and a problem
// when there are fewer than MinOptions or they are numbered out of order.
func ParseOptions(s string) ([]Option, []Problem) {
	var os []Option
	var ps []Problem
	var margin []string // unnumbered top-level lines, for a lettered Decision
	fence := ""         // the open fence's delimiter run; only a run of the same character at least as long closes it
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
				if line != "" && !unicode.IsSpace(rune(line[0])) {
					margin = append(margin, strings.TrimSpace(line))
				}
				continue
			}
			n, _ := strconv.Atoi(m[1])
			if want := len(os) + 1; n != want {
				ps = append(ps, Problem{"options", fmt.Sprintf("option %d is numbered out of order; want %d, so each option keeps the number the person picks it by", n, want)})
			}
			os = append(os, optionOf(n, strings.TrimSpace(m[2])))
		}
	}
	if len(os) == 0 {
		os, ps = letteredOptions(margin)
	}
	marked := 0
	var named []Option
	for _, o := range os {
		if o.Recommended {
			marked++
		}
		if strings.TrimSpace(o.Name) == "" {
			ps = append(ps, Problem{"options", fmt.Sprintf("option %d has no name; start it with its name, \"%d. **Name.** what it does\"", o.N, o.N)})
			continue
		}
		// The same comparison decide.Pick accepts a typed name with.
		if i := slices.IndexFunc(named, func(p Option) bool { return strings.EqualFold(p.Name, o.Name) }); i >= 0 {
			ps = append(ps, Problem{"options", fmt.Sprintf("options %d and %d are both named %q; give each its own name, so accepting it by name is unambiguous", named[i].N, o.N, o.Name)})
		} else {
			named = append(named, o)
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

// inlineOptions splits a line that numbers or letters its options inline,
// "1. Keep it. 2. Drop it." or "A: in-house. B: vendor.", at each next number
// or letter in sequence where a sentence ends before it ("phase 2. before
// launch" is prose); any other line is itself.
func inlineOptions(line string) []string {
	if len(line) < 3 || line[2] != ' ' {
		return []string{line}
	}
	if numbered, lettered := line[0] == '1' && strings.IndexByte(".)", line[1]) >= 0, line[0] == 'A' && strings.IndexByte(".:)", line[1]) >= 0; !numbered && !lettered {
		return []string{line}
	}
	first, sep := line[0], line[1]
	var out []string
	for k := 1; ; k++ {
		next := fmt.Sprintf(" %d%c ", k+1, sep)
		if first == 'A' {
			next = fmt.Sprintf(" %c%c ", 'A'+k, sep)
		}
		i := strings.Index(line, next)
		if i < 1 || !strings.ContainsRune(".;", rune(line[i-1])) {
			return append(out, line)
		}
		out, line = append(out, line[:i]), line[i+1:]
	}
}

// letteredOptions reads a Decision written before options were numbered: each
// lettered, bulleted, or named top-level line is an option, numbered in order,
// with its own letter or else A, B, … by position.
func letteredOptions(lines []string) ([]Option, []Problem) {
	var os []Option
	var ps []Problem
	for _, l := range lines {
		var o Option
		want := string(rune('A' + len(os)))
		switch m, b := letterLine.FindStringSubmatch(l), bulletLine.FindStringSubmatch(l); {
		case m != nil:
			o = optionOf(len(os)+1, m[2])
			o.Letter = m[1]
			if o.Letter != want {
				ps = append(ps, Problem{"options", fmt.Sprintf("option %s is lettered out of order; want %s, so each option keeps the letter the person knows it by", o.Letter, want)})
			}
		case b != nil:
			o = optionOf(len(os)+1, b[1])
		case namedLine.MatchString(l):
			o = optionOf(len(os)+1, l)
		default:
			continue
		}
		if o.Letter == "" {
			o.Letter = want
		}
		os = append(os, o)
	}
	return os, ps
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

// RecommendedOption is the number of the option a Recommendation opens with,
// "Option 2, because …", "2.", a lettered option's letter ("Option B", "B."),
// or the option's name, or -1 when it opens with none. A number is returned as written, even 0 or one the Decision lacks. Only the opening counts, since a Recommendation often goes on to name
// the options it rejects.
func RecommendedOption(os []Option, rec string) int {
	r := strings.TrimLeft(strings.TrimSpace(rec), "*_ ")
	if m := openingRef.FindStringSubmatch(r); m != nil {
		n, _ := strconv.Atoi(m[1] + m[2])
		return n
	}
	if m := letterRef.FindStringSubmatch(r); m != nil && Lettered(os) {
		return int((m[1] + m[2])[0]-'A') + 1
	}
	best, rr := Option{N: -1}, []rune(r)
	for _, o := range os {
		on := []rune(o.Name)
		if len(on) == 0 || len(on) > len(rr) || len(on) <= len([]rune(best.Name)) || !strings.EqualFold(string(rr[:len(on)]), o.Name) {
			continue
		}
		if rest := rr[len(on):]; len(rest) == 0 || !unicode.IsLetter(rest[0]) && !unicode.IsDigit(rest[0]) {
			best = o
		}
	}
	return best.N
}
