package evals

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The skill-eval bars ratchet like the coverage floor (Story #400, Feedback
// #327): a change may raise them, never lower them, unless it names a new
// floor waiver for BarFloor that a human accepted (`aayu decide --waiver
// eval-bar <story>`, then `aayu decide <n> --accept waive`) as `bar_waiver =
// <n>` in evals/eval.toml. A waiver already named on the base does not count
// again. What lowers a bar is a property of the diff, so this needs no model:
//
//   - pass_score, min_pass_k, or min_mean lower, or max_drop higher;
//   - a case whose group gates moving to a group with gate = "advisory", or one
//     held to pass^k moving to pass_k = "advisory";
//   - a case removed;
//   - a deterministic rule of a case (case.toml scope, session.toml require,
//     forbid, say_forbid) removed, or its pattern or allow changed;
//   - a rubric item (a criterion, an anchor, or a paragraph) removed, or
//     reworded so that it loses any of its words. An item that only gains
//     words is kept.
//
// New cases, groups, rules, and rubric items are never lowerings: a new group
// starts advisory.

// BarFloor is the floor a waiver Decision names to lower a skill-eval bar.
const BarFloor = "eval-bar"

// Lowering is one way a change lowers a skill-eval bar.
type Lowering struct {
	Bar  string // what was lowered: an evals/eval.toml key, a case ID, or a rubric path
	What string
}

func (l Lowering) String() string { return l.Bar + ": " + l.What }

// CheckBars compares the evals under the base and head repository roots. It
// returns every lowering and the problems that fail the check: lowerings with
// no new waiver, or a new waiver that accepted (which reads the Decision)
// refuses. No lowering passes whatever the waiver.
func CheckBars(base, head string, accepted func(decision int) error) (lowered []Lowering, problems []string, err error) {
	bc, err := LoadConfig(base)
	if err != nil {
		return nil, nil, fmt.Errorf("base: %w", err)
	}
	hc, err := LoadConfig(head)
	if err != nil {
		return nil, nil, err
	}
	if lowered, err = loweredBars(base, head, bc, hc); err != nil || len(lowered) == 0 {
		return lowered, nil, err
	}
	if hc.BarWaiver == 0 || hc.BarWaiver == bc.BarWaiver {
		for _, l := range lowered {
			problems = append(problems, fmt.Sprintf("%s lowers a skill-eval bar without a new waiver: %s", l.Bar, l.What))
		}
		problems = append(problems, fmt.Sprintf("a skill-eval bar only rises: fix the work instead, or file `aayu decide --waiver %s <story>`, have a human accept it "+
			"(`aayu decide <n> --accept waive`), and name it as `bar_waiver = <n>` in evals/eval.toml", BarFloor))
		return lowered, problems, nil
	}
	if err := accepted(hc.BarWaiver); err != nil {
		problems = append(problems, fmt.Sprintf("bar_waiver = %d: %v", hc.BarWaiver, err))
	}
	return lowered, problems, nil
}

func loweredBars(base, head string, bc, hc Config) ([]Lowering, error) {
	var out []Lowering
	low := func(bar, format string, a ...any) { out = append(out, Lowering{bar, fmt.Sprintf(format, a...)}) }
	if hc.PassScore < bc.PassScore {
		low("pass_score", "%d → %d", bc.PassScore, hc.PassScore)
	}
	if hc.MinPassK < bc.MinPassK {
		low("min_pass_k", "%g → %g", bc.MinPassK, hc.MinPassK)
	}
	if hc.MinMean < bc.MinMean {
		low("min_mean", "%g → %g", bc.MinMean, hc.MinMean)
	}
	if hc.MaxDrop > bc.MaxDrop {
		low("max_drop", "%g → %g (a larger fall below the baseline passes)", bc.MaxDrop, hc.MaxDrop)
	}
	baseCases, err := LoadCases(base)
	if err != nil {
		return nil, fmt.Errorf("base: %w", err)
	}
	headCases, err := LoadCases(head)
	if err != nil {
		return nil, err
	}
	byID := map[string]Case{}
	for _, c := range headCases {
		byID[c.ID] = c
	}
	rubrics := map[string]bool{}
	for _, b := range baseCases {
		if b.Tier2 != nil && b.Tier2.Rubric != "" {
			if rel, err := filepath.Rel(base, filepath.Join(b.Dir, b.Tier2.Rubric)); err == nil {
				rubrics[filepath.ToSlash(rel)] = true
			}
		}
		h, ok := byID[b.ID]
		if !ok {
			low(b.ID, "the case is removed")
			continue
		}
		bg, _ := bc.Group(bc.GroupOf(b.ID))
		hg, _ := hc.Group(hc.GroupOf(b.ID))
		if bg.Gate != GateAdvisory && hg.Gate == GateAdvisory {
			low(b.ID, "moves from group %s, which gates, to %s, which is advisory (gate)", bg.Name, hg.Name)
		}
		if bg.PassK != PassKAdvisory && hg.PassK == PassKAdvisory {
			low(b.ID, "moves from group %s, which holds it to pass^k, to %s, where pass^k is advisory (pass_k)", bg.Name, hg.Name)
		}
		br, err := rules(b)
		if err != nil {
			return nil, fmt.Errorf("base: %w", err)
		}
		hr, err := rules(h)
		if err != nil {
			return nil, err
		}
		kept := map[string]bool{}
		for _, r := range hr {
			kept[r.key] = true
		}
		for _, r := range br {
			if !kept[r.key] {
				low(b.ID, "removes or changes its %s rule %q (%s)", r.kind, r.Pattern, r.Why)
			}
		}
	}
	paths := make([]string, 0, len(rubrics))
	for p := range rubrics {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		bt, _ := readText(filepath.Join(base, p)) // a rubric the base lacks has nothing to lower
		ht, err := readText(filepath.Join(head, p))
		if err != nil {
			low(p, "the rubric is removed")
			continue
		}
		hi := rubricItems(ht)
		for _, item := range rubricItems(bt) {
			if !keptItem(item, hi) {
				low(p, "drops or rewords %q", clipText(item.text, 100))
			}
		}
	}
	return out, nil
}

// ruleOf is a case's deterministic rule with its kind and identity.
type ruleOf struct {
	Scope
	kind, key string
}

// rules lists a case's deterministic rules: its case.toml scope rules and,
// for a session case, its session.toml require, forbid, and say_forbid rules.
func rules(c Case) ([]ruleOf, error) {
	var out []ruleOf
	add := func(kind string, rs []Scope) {
		for _, r := range rs {
			out = append(out, ruleOf{r, kind, kind + "\x00" + r.Pattern + "\x00" + r.Allow})
		}
	}
	add("scope", c.Scope)
	if c.Tier2 != nil && c.Tier2.Session != "" {
		s, err := LoadSession(filepath.Join(c.Dir, c.Tier2.Session))
		if err != nil {
			return nil, err
		}
		add("require", s.Require)
		add("forbid", s.Forbid)
		add("say_forbid", s.SayForbid)
	}
	return out, nil
}

// rubricItem is one item of a rubric: its text, and its lower-case words
// without punctuation.
type rubricItem struct {
	text  string
	words []string
}

// rubricItems splits a rubric into its items: each heading, list item (a
// numbered criterion or a "- 5:" anchor), and paragraph, joining the lines an
// item wraps over.
func rubricItems(text string) []rubricItem {
	var items []rubricItem
	var cur []string
	flush := func() {
		if len(cur) > 0 {
			t := strings.Join(cur, " ")
			items = append(items, rubricItem{t, strings.FieldsFunc(strings.ToLower(t), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsNumber(r) })})
		}
		cur = nil
	}
	for _, line := range strings.Split(text, "\n") {
		t := strings.TrimSpace(line)
		if t == "" {
			flush()
			continue
		}
		if startsItem(t) {
			flush()
		}
		cur = append(cur, t)
	}
	flush()
	return items
}

func startsItem(t string) bool {
	if strings.HasPrefix(t, "#") || strings.HasPrefix(t, "- ") || strings.HasPrefix(t, "* ") {
		return true
	}
	i := 0
	for i < len(t) && t[i] >= '0' && t[i] <= '9' {
		i++
	}
	return i > 0 && i+1 < len(t) && (t[i] == '.' || t[i] == ')') && t[i+1] == ' '
}

// keptItem reports whether some head item holds every word of item, in order.
func keptItem(item rubricItem, head []rubricItem) bool {
	for _, h := range head {
		i := 0
		for _, w := range h.words {
			if i < len(item.words) && w == item.words[i] {
				i++
			}
		}
		if i == len(item.words) {
			return true
		}
	}
	return false
}

func clipText(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n]) + "…"
}

func readText(path string) (string, error) {
	b, err := os.ReadFile(path)
	return string(b), err
}
