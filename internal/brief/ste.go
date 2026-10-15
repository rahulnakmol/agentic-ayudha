package brief

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode"
)

// MaxSTEWords is the STE sentence limit the plan sets (WP7.3).
const MaxSTEWords = 20

// Words is the STE approved-word list (skills/brief/references/ste-words.md):
// every approved word with its part of speech, and unapproved words with the
// approved words to use instead.
type Words struct {
	approved map[string]bool
	verbs    map[string]bool   // approved verbs, base form
	avoid    map[string]string // unapproved word -> what to write instead
}

// Len is the number of approved words.
func (w *Words) Len() int { return len(w.approved) }

// ParseWords reads the word list: after the first `## ` heading, each
// section names a part of speech (Verbs, Nouns, Adjectives, Adverbs, Other)
// and holds comma-separated lowercase words; the section `Avoid` holds
// `word → instead` pairs.
func ParseWords(md []byte) (*Words, error) {
	w := &Words{approved: map[string]bool{}, verbs: map[string]bool{}, avoid: map[string]string{}}
	pos := ""
	sc := bufio.NewScanner(bytes.NewReader(md))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if h, ok := strings.CutPrefix(line, "## "); ok {
			pos = strings.ToLower(strings.TrimSpace(h))
			continue
		}
		if pos == "" || line == "" {
			continue
		}
		for _, part := range strings.Split(line, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if pos == "avoid" {
				bad, good, ok := strings.Cut(part, "→")
				if !ok {
					return nil, fmt.Errorf("ste words: avoid entry %q: want `word → instead`", part)
				}
				w.avoid[strings.TrimSpace(bad)] = strings.TrimSpace(good)
				continue
			}
			if strings.ToLower(part) != part || strings.ContainsFunc(part, unicode.IsSpace) {
				return nil, fmt.Errorf("ste words: %q in %s: want one lowercase word", part, pos)
			}
			w.approved[part] = true
			if pos == "verbs" {
				w.verbs[part] = true
			}
		}
	}
	if len(w.approved) == 0 {
		return nil, errors.New("ste words: no approved words")
	}
	for bad := range w.avoid {
		if w.approved[bad] {
			return nil, fmt.Errorf("ste words: %q is both approved and to avoid", bad)
		}
	}
	return w, nil
}

// Approved reports whether a word (any case) is approved, directly or as a
// regular plural, possessive, or past form of an approved word.
func (w *Words) Approved(word string) bool {
	word = strings.ToLower(strings.ReplaceAll(word, "’", "'"))
	word = strings.TrimSuffix(strings.TrimSuffix(word, "'s"), "'")
	if w.approved[word] {
		return true
	}
	for _, base := range stems(word) {
		if w.approved[base] {
			return true
		}
	}
	return false
}

// isVerb reports whether word is the base form of an approved verb.
func (w *Words) isVerb(word string) bool { return w.verbs[strings.ToLower(word)] }

// stems are the candidate base forms of a regularly inflected word.
func stems(w string) []string {
	var out []string
	switch {
	case strings.HasSuffix(w, "ies") && len(w) > 4:
		out = append(out, w[:len(w)-3]+"y")
	case strings.HasSuffix(w, "es"):
		out = append(out, w[:len(w)-2], w[:len(w)-1])
	case strings.HasSuffix(w, "s") && !strings.HasSuffix(w, "ss"):
		out = append(out, w[:len(w)-1])
	case strings.HasSuffix(w, "ied") && len(w) > 4:
		out = append(out, w[:len(w)-3]+"y")
	case strings.HasSuffix(w, "ed") && len(w) > 4:
		b := w[:len(w)-2]
		out = append(out, b, w[:len(w)-1])
		if n := len(b); n > 2 && b[n-1] == b[n-2] {
			out = append(out, b[:n-1]) // stopped -> stop
		}
	}
	return out
}

// Names are the technical names a brief may use without approval (STE
// allows technical names): the words of the titles of its subject and work
// items, of the subject's summary, and of the Result (so a metric keeps its
// own nouns, such as "per reviewer"), and the floor names of its packs.
func Names(g *Grounding) map[string]bool {
	out := map[string]bool{}
	add := func(s string) {
		for _, w := range wordRE.FindAllString(s, -1) {
			out[strings.ToLower(w)] = true
		}
	}
	add(g.Subject.Title)
	add(g.Subject.Fields["summary"])
	if g.Result != nil {
		add(g.Result.Text)
	}
	for _, it := range g.Items {
		add(it.Title)
	}
	for _, p := range g.Packs {
		for _, c := range p.Checks {
			add(c.Floor)
		}
	}
	return out
}

var clauseToken = regexp.MustCompile(`[\p{L}\p{N}]+(?:['’\-][\p{L}\p{N}]+)*|[,;]`)

// leadIns may open an instruction before its verb ("If …, open …", "First, open …").
var leadIns = []string{"if", "when", "before", "after", "first", "next", "then", "now", "finally", "to"}

// instructions counts the instructions in a sentence: clauses that start
// with the base form of an approved verb, where clauses are separated by
// commas, semicolons, "and", "then", and "or". A sentence that does not open
// with an instruction (or a lead-in such as "If …,") describes; it counts 0.
func instructions(s string, w *Words) int {
	toks := clauseToken.FindAllString(strings.ReplaceAll(s, codeToken, "code"), -1)
	var starts []string
	at := true
	for _, t := range toks {
		lt := strings.ToLower(t)
		if t == "," || t == ";" || lt == "and" || lt == "then" || lt == "or" {
			at = true
			continue
		}
		if at {
			starts = append(starts, lt)
			at = false
		}
	}
	if len(starts) == 0 {
		return 0
	}
	opens := w.isVerb(starts[0]) || (slices.Contains(leadIns, starts[0]) && len(starts) > 1 && w.isVerb(starts[1]))
	if !opens {
		return 0
	}
	n := 0
	for _, st := range starts {
		if w.isVerb(st) {
			n++
		}
	}
	return n
}

// LintSTE applies the STE rules the plan names to prose: every sentence at
// most 20 words, every word approved (or a technical name: a code span, a
// number, an acronym, a capitalised name inside a sentence, or a word of the
// grounding's titles), and one instruction per sentence.
func LintSTE(units []Unit, w *Words, names map[string]bool) []string {
	var out []string
	unapproved := map[string]bool{}
	for i, s := range Sentences(units) {
		n := i + 1
		if len(s.Words) > MaxSTEWords {
			out = append(out, fmt.Sprintf("sentence %d has %d words (at most %d): %q", n, len(s.Words), MaxSTEWords, clip(s.Text, 160)))
		}
		for _, wd := range s.Words {
			if wd.Code || technicalName(wd) || names[strings.ToLower(wd.Text)] {
				continue
			}
			ok := true
			for _, part := range strings.Split(wd.Text, "-") {
				ok = ok && (w.Approved(part) || names[strings.ToLower(part)])
			}
			if ok {
				continue
			}
			lw := strings.ToLower(wd.Text)
			if unapproved[lw] {
				continue
			}
			unapproved[lw] = true
			msg := fmt.Sprintf("%q is not an approved word (sentence %d)", wd.Text, n)
			if alt, ok := w.avoid[lw]; ok {
				msg += "; write " + alt
			}
			out = append(out, msg)
		}
		if k := instructions(s.Text, w); k > 1 {
			out = append(out, fmt.Sprintf("sentence %d gives %d instructions; write one instruction per sentence: %q", n, k, clip(s.Text, 160)))
		}
	}
	return out
}

// technicalName: a number or anything with a digit, an acronym, or a
// capitalised word that does not open its sentence.
func technicalName(w Word) bool {
	r := []rune(w.Text)
	switch {
	case strings.ContainsFunc(w.Text, unicode.IsDigit):
		return true
	case len(r) >= 2 && strings.ToUpper(w.Text) == w.Text && unicode.IsLetter(r[0]):
		return true
	case !w.First && unicode.IsUpper(r[0]):
		return true
	}
	return false
}
