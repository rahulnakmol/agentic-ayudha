package brief

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

var (
	perPhrase  = regexp.MustCompile(`(?i)\bper ([a-z][a-z-]*)`)
	timeUnits  = []string{"second", "minute", "hour", "day", "week", "month", "quarter", "year", "cent", "run", "cohort"}
	resultNum  = regexp.MustCompile(`\d+(?:\.\d+)?`)
	datePart   = regexp.MustCompile(`\d{4}-\d{2}-\d{2}(?:T[0-9:]+Z?)?`)
	criterionN = regexp.MustCompile(`(?i)\bcriteri(?:on|a)\s+[\d ,and]+`)
)

// populations are the "per <noun>" populations the Result states, other
// than time units: "4.2 exports per reviewer per week" gives "reviewer".
func populations(r *Result) []string {
	if r == nil {
		return nil
	}
	var out []string
	for _, m := range perPhrase.FindAllStringSubmatch(r.Text, -1) {
		w := strings.ToLower(m[1])
		if !slices.Contains(timeUnits, strings.TrimSuffix(w, "s")) && !slices.Contains(out, w) {
			out = append(out, w)
		}
	}
	return out
}

// unitNoun is the noun the Result counts: the word before its first
// "per <population>" ("CSV exports per reviewer" gives "exports").
func unitNoun(r *Result, pops []string) string {
	if r == nil || len(pops) == 0 {
		return ""
	}
	m := regexp.MustCompile(`(?i)\b([a-z][a-z_-]*)\s+per\s+` + regexp.QuoteMeta(pops[0]) + `\b`).FindStringSubmatch(r.Text)
	if m == nil {
		return ""
	}
	return strings.ToLower(m[1])
}

func singular(w string) string {
	w = strings.ToLower(w)
	if strings.HasSuffix(w, "s") && !strings.HasSuffix(w, "ss") {
		return w[:len(w)-1]
	}
	return w
}

// substitutes are everyday words a draft puts in place of a count's unit.
var substitutes = regexp.MustCompile(`(?i)\b(files?\s+(?:saved|made|created|downloaded)|saved\s+files?|downloads?)\b`)

var wordTok = regexp.MustCompile(`[\p{L}\p{N}_-]+`)

// rateWording is the population and unit rule (deterministic part), sentence
// by sentence in every paragraph or list item that cites the Result:
//   - population: a sentence that states one of the Result's numbers has the
//     Result's "per <population>" wording (once per sentence is enough), and
//     "each/every <population>" is refused;
//   - unit: each of the Result's numbers is followed, within four words, by
//     the Result's own unit noun ("3 exports", "4.2 CSV exports"), and
//     everyday substitutes for it ("files saved", "downloads") are refused.
//
// So a rate and its target fit in two short sentences ("The rate is 4.2
// exports per reviewer per week. The target is 3 exports per reviewer per
// week."), which keeps ste sentences under the 20-word limit.
func rateWording(d *Draft, g *Grounding) (population, unit []string) {
	pops := populations(g.Result)
	if len(pops) == 0 {
		return nil, nil
	}
	noun := unitNoun(g.Result, pops)
	nums := map[string]bool{}
	for _, n := range resultNum.FindAllString(datePart.ReplaceAllString(g.Result.Text, " "), -1) {
		if n != "0" {
			nums[n] = true
		}
	}
	for i, u := range d.Units {
		if !slices.Contains(u.Cites, "R") {
			continue
		}
		where := fmt.Sprintf("%s %d", kindOf(u), i+1)
		text := strings.ToLower(u.Text)
		for _, p := range pops {
			if m := regexp.MustCompile(`\b(each|every) ` + regexp.QuoteMeta(p) + `\b`).FindString(text); m != "" {
				population = append(population, fmt.Sprintf("%s says %q, but the Result states a rate per %s, not what each %s does: %q", where, m, p, p, clip(u.Text, 120)))
			}
		}
		if noun != "" && !substitutes.MatchString(noun) {
			if m := substitutes.FindString(u.Text); m != "" {
				unit = append(unit, fmt.Sprintf("%s names the count %q; the Result counts %s: say %s: %q", where, m, noun, noun, clip(u.Text, 120)))
			}
		}
		for _, s := range Sentences([]Unit{u}) {
			clean := datePart.ReplaceAllString(criterionN.ReplaceAllString(s.Text, " "), " ")
			lower := strings.ToLower(clean)
			stated := false
			for _, loc := range resultNum.FindAllStringIndex(clean, -1) {
				n := clean[loc[0]:loc[1]]
				if !nums[n] {
					continue
				}
				stated = true
				if noun == "" {
					continue
				}
				next := wordTok.FindAllString(clean[loc[1]:], 4)
				if !slices.ContainsFunc(next, func(w string) bool { return singular(w) == singular(noun) }) {
					unit = append(unit, fmt.Sprintf("%s states %s from the Result without its unit: write \"%s %s\" as the Result counts it: %q", where, n, n, noun, clip(s.Text, 120)))
				}
			}
			if !stated {
				continue
			}
			for _, p := range pops {
				if !strings.Contains(lower, "per "+p) {
					population = append(population, fmt.Sprintf("%s states a number from the Result without its population: keep \"per %s\" in the sentence as the Result words it: %q", where, p, clip(s.Text, 120)))
				}
			}
		}
	}
	return population, unit
}

// uiWords name interface elements and interactions. A brief may name one
// only when its grounding does (a criterion, a field, a title, the Result).
var uiWords = regexp.MustCompile(`(?i)\b(buttons?|menus?|drop-?downs?|checkbox(?:es)?|toggles?|tabs?|dialogs?|modals?|pop-?ups?|icons?|toolbars?|sidebars?|click(?:s|ed|ing)?|tap(?:s|ped|ping)?|press(?:es|ed|ing)?|swip(?:e|es|ed|ing))\b`)

// uiDetails is the interface rule: a UI element or interaction named in the
// prose that no source names is an invented detail.
func uiDetails(d *Draft, g *Grounding) []string {
	b, _ := json.Marshal(g)
	known := map[string]bool{}
	for _, w := range uiWords.FindAllString(string(b), -1) {
		known[stem(w)] = true
	}
	var out []string
	seen := map[string]bool{}
	for i, u := range d.Units {
		for _, w := range uiWords.FindAllString(u.Text, -1) {
			st := stem(w)
			if known[st] || seen[st] {
				continue
			}
			seen[st] = true
			out = append(out, fmt.Sprintf("%s %d names %q, an interface detail no source names; describe what the sources state, not controls or steps: %q", kindOf(u), i+1, w, clip(u.Text, 120)))
		}
	}
	return out
}

// uiRoots are the roots uiWords matches, so forms fold together.
var uiRoots = []string{"button", "menu", "dropdown", "checkbox", "toggle", "tab", "dialog", "modal", "popup", "icon", "toolbar", "sidebar", "click", "tap", "press", "swip"}

// stem folds the forms of a UI word together (buttons, button; clicked, click).
func stem(w string) string {
	w = strings.ReplaceAll(strings.ToLower(w), "-", "")
	for _, r := range uiRoots {
		if strings.HasPrefix(w, r) {
			return r
		}
	}
	return w
}

// absenceRE matches the wordings the skill forbids for missing evidence and
// out-of-scope items, with the word after them.
var absenceRE = regexp.MustCompile(`(?i)\b(untested|not tested|not done|not possible|can't|can’t|cannot)\b(?:\s+([\p{L}\p{N}-]+))?`)

// exclusionRE is "does not include" and its forms; an item the grounding
// marks out of scope is "outside this story's scope" instead.
var exclusionRE = regexp.MustCompile(`(?i)\b(does not include|doesn't include|doesn’t include|do not include|is not included|are not included)\b`)

// outOfScope returns the out-of-scope items the subject's Scope field lists
// ("Out of scope: PDF, scheduled exports, exports of other lists"), each as
// its content words; a sentence names an item when it has all of them.
func outOfScope(g *Grounding) [][]string {
	m := regexp.MustCompile(`(?is)out of scope:?\s*([^.\n]*)`).FindStringSubmatch(g.Subject.Fields["scope"])
	if m == nil {
		return nil
	}
	var out [][]string
	for _, item := range regexp.MustCompile(`(?i),|;|\band\b|\bor\b`).Split(m[1], -1) {
		var words []string
		for _, w := range wordTok.FindAllString(strings.ToLower(item), -1) {
			if len(w) > 2 && !slices.Contains([]string{"the", "other", "for", "with", "any", "all"}, w) {
				words = append(words, singular(w))
			}
		}
		if len(words) > 0 {
			out = append(out, words)
		}
	}
	return out
}

// absenceClaims is the absence rule: missing evidence is written "the
// evidence does not show …" and an excluded item "outside this story's scope",
// never "untested", "not done", "not possible", or "can't". A source's own
// wording may be quoted: the phrase with the word after it ("cannot run")
// passes when the grounding says it.
func absenceClaims(d *Draft, g *Grounding) []string {
	b, _ := json.Marshal(g)
	src := strings.ToLower(strings.ReplaceAll(string(b), "’", "'"))
	var out []string
	for i, u := range d.Units {
		for _, m := range absenceRE.FindAllStringSubmatch(u.Text, -1) {
			quoted := strings.ToLower(strings.ReplaceAll(m[1]+" "+m[2], "’", "'"))
			if m[2] != "" && strings.Contains(src, quoted) {
				continue
			}
			out = append(out, fmt.Sprintf("%s %d says %q: write missing evidence as \"the evidence does not show …\" and an excluded item as \"outside this story's scope\": %q", kindOf(u), i+1, strings.TrimSpace(m[0]), clip(u.Text, 120)))
		}
		if items := outOfScope(g); len(items) > 0 {
			for _, s := range Sentences([]Unit{u}) {
				m := exclusionRE.FindString(s.Text)
				if m == "" {
					continue
				}
				var have []string
				for _, w := range wordTok.FindAllString(s.Text, -1) {
					have = append(have, singular(w))
				}
				if slices.ContainsFunc(items, func(item []string) bool {
					return !slices.ContainsFunc(item, func(w string) bool { return !slices.Contains(have, w) })
				}) {
					out = append(out, fmt.Sprintf("%s %d says %q of items the grounding marks out of scope: write \"outside this story's scope\": %q", kindOf(u), i+1, m, clip(s.Text, 120)))
				}
			}
		}
	}
	return out
}
