package brief

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

var (
	criterionRef = regexp.MustCompile(`(?i)\bcriteri(?:on|a)\s+(\d+(?:\s*(?:,|and|or|to|through|-|–)\s*\d+)*)`)
	criterionNum = regexp.MustCompile(`\d+|to|through|-|–`)
	proofWord    = regexp.MustCompile(`(?i)\b(prove[sdn]?|proving|show[sn]?|showed|demonstrat\w*|verif\w*|confirm\w*|evidenc(?:e[sd]|ing)|met|meets|satisf\w*|pass(?:es|ed)?)\b`)
	artifactWord = regexp.MustCompile(`(?i)\b(recordings?|(?:api[- ])?traces?|screenshots?|transcripts?|artifacts?)\b`)
	negation     = regexp.MustCompile(`(?i)\b(not|no|none|without|lacks?|missing|unmapped|unproven|never|nothing)\b`)
	mapCite      = regexp.MustCompile(`^(P\d+)\.c(\d+)$`)
)

// criteria expands "1, 2 and 3", "1 to 3", or "1-3" into numbers.
func criteria(list string) []int {
	var out []int
	toks := criterionNum.FindAllString(list, -1)
	for i := 0; i < len(toks); i++ {
		n, err := strconv.Atoi(toks[i])
		if err != nil {
			continue
		}
		if i+2 < len(toks) && !isNum(toks[i+1]) && isNum(toks[i+2]) { // a range
			m, _ := strconv.Atoi(toks[i+2])
			for k := n; k <= m && k-n < 50; k++ {
				out = append(out, k)
			}
			i += 2
			continue
		}
		out = append(out, n)
	}
	return out
}

func isNum(s string) bool { _, err := strconv.Atoi(s); return err == nil }

// proofClaims is the evidence rule: a sentence that says a criterion is
// proved, met, or shown (or that a recording, trace, or screenshot shows
// something) makes a claim about evidence, and evidence is cited only
// through the pack's criterion -> evidence map. Each criterion it names
// needs a citation of its map entry ([P1.c3]) in the same paragraph or list
// item, and an entry exists only when the pack maps evidence to it.
// Negated sentences ("criterion 3 is not proved") claim nothing.
func proofClaims(d *Draft, g *Grounding) []string {
	mapped := map[int][]string{} // criterion -> citable entries
	for _, p := range g.Packs {
		for _, c := range p.Criteria {
			if len(c.Evidence) > 0 {
				mapped[c.ID] = append(mapped[c.ID], c.Cite(p.ID))
			}
		}
	}
	var out []string
	for i, u := range d.Units {
		cited := map[int]bool{}
		anyMap := false
		for _, c := range u.Cites {
			if m := mapCite.FindStringSubmatch(c); m != nil {
				n, _ := strconv.Atoi(m[2])
				cited[n], anyMap = true, true
			}
		}
		for _, s := range Sentences([]Unit{u}) {
			if negation.MatchString(s.Text) || !proofWord.MatchString(s.Text) {
				continue
			}
			var nums []int
			for _, m := range criterionRef.FindAllStringSubmatch(s.Text, -1) {
				nums = append(nums, criteria(m[1])...)
			}
			if len(nums) == 0 {
				if artifactWord.MatchString(s.Text) && !anyMap {
					out = append(out, fmt.Sprintf("%s %d says what an artifact shows without citing the criterion it is mapped to ([P1.c1] and the like): %q", kindOf(u), i+1, clip(s.Text, 120)))
				}
				continue
			}
			for _, n := range nums {
				switch {
				case cited[n]:
				case len(mapped[n]) == 0:
					out = append(out, fmt.Sprintf("%s %d claims proof of criterion %d, but no pack maps evidence to it; say it is not proved: %q", kindOf(u), i+1, n, clip(s.Text, 120)))
				default:
					out = append(out, fmt.Sprintf("%s %d claims proof of criterion %d without citing its evidence map entry (%s): %q", kindOf(u), i+1, n, strings.Join(slices.Compact(mapped[n]), " or "), clip(s.Text, 120)))
				}
			}
		}
	}
	return out
}
