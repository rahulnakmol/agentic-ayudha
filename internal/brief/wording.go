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

// populationDrift is the population rule (deterministic part): a paragraph
// or list item that cites the Result keeps the Result's own population
// wording. Stating one of the Result's numbers needs "per <population>" in
// the same unit, and "each <population>" or "every <population>" turns a rate
// into a claim about every member, so it is refused.
func populationDrift(d *Draft, g *Grounding) []string {
	pops := populations(g.Result)
	if len(pops) == 0 {
		return nil
	}
	nums := map[string]bool{}
	for _, n := range resultNum.FindAllString(datePart.ReplaceAllString(g.Result.Text, " "), -1) {
		if n != "0" {
			nums[n] = true
		}
	}
	var out []string
	for i, u := range d.Units {
		if !slices.Contains(u.Cites, "R") {
			continue
		}
		text := strings.ToLower(u.Text)
		for _, p := range pops {
			if m := regexp.MustCompile(`\b(each|every) ` + regexp.QuoteMeta(p) + `\b`).FindString(text); m != "" {
				out = append(out, fmt.Sprintf("%s %d says %q, but the Result states a rate per %s, not what each %s does: %q", kindOf(u), i+1, m, p, p, clip(u.Text, 120)))
			}
			stated := ""
			for _, n := range resultNum.FindAllString(datePart.ReplaceAllString(criterionN.ReplaceAllString(u.Text, " "), " "), -1) {
				if nums[n] {
					stated = n
					break
				}
			}
			if stated != "" && !strings.Contains(text, "per "+p) {
				out = append(out, fmt.Sprintf("%s %d states %s from the Result without its population: keep \"per %s\" as the Result words it: %q", kindOf(u), i+1, stated, p, clip(u.Text, 120)))
			}
		}
	}
	return out
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
