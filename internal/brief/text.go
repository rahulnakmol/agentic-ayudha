package brief

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// Sentence is one sentence of a prose unit.
type Sentence struct {
	Unit  int    // index into Draft.Units
	Text  string // as written
	Words []Word
}

// Word is one word of a sentence.
type Word struct {
	Text  string
	Code  bool // a code span: a technical name, one word
	First bool // the sentence's first word
}

// abbreviations do not end a sentence.
var abbreviations = regexp.MustCompile(`(?i)(^|\s)(e\.g\.|i\.e\.|etc\.|vs\.|approx\.|fig\.)`)

var sentenceEnd = regexp.MustCompile(`[.!?]+["')\]]*(\s+|$)`)

// Sentences splits prose units into sentences. A list item is at least one
// sentence of its own; a full stop inside a number or after a listed
// abbreviation does not end a sentence, and neither does a colon or a
// semicolon (so joining clauses never shortens a sentence).
func Sentences(units []Unit) []Sentence {
	var out []Sentence
	for i, u := range units {
		t := abbreviations.ReplaceAllStringFunc(u.Text, func(m string) string { return strings.ReplaceAll(m, ".", "\uE002") })
		start := 0
		for _, m := range sentenceEnd.FindAllStringIndex(t, -1) {
			s := strings.TrimSpace(t[start:m[1]])
			start = m[1]
			if s != "" {
				out = append(out, sentence(i, s))
			}
		}
		if s := strings.TrimSpace(t[start:]); s != "" {
			out = append(out, sentence(i, s))
		}
	}
	return out
}

var wordRE = regexp.MustCompile(`\x{E000}code\x{E001}|[\p{L}\p{N}]+(?:['’\-][\p{L}\p{N}]+)*`)

func sentence(unit int, s string) Sentence {
	s = strings.ReplaceAll(s, "\uE002", ".")
	st := Sentence{Unit: unit, Text: strings.ReplaceAll(s, codeToken, "`code`")}
	for i, w := range wordRE.FindAllString(s, -1) {
		st.Words = append(st.Words, Word{Text: w, Code: w == codeToken, First: i == 0})
	}
	return st
}

// Reading is a Flesch-Kincaid reading of prose.
type Reading struct {
	Grade     float64 `json:"grade"`
	Words     int     `json:"words"`
	Sentences int     `json:"sentences"`
	Syllables int     `json:"syllables"`
	Longest   string  `json:"longest,omitempty"` // the sentence that adds most to the grade
}

// Readability computes the Flesch-Kincaid grade level of prose:
// 0.39 × words/sentences + 11.8 × syllables/words − 15.59. Code spans and
// numbers count as one word of one syllable; citations are not words.
func Readability(units []Unit) Reading {
	var r Reading
	worst, worstScore := "", -1.0
	for _, s := range Sentences(units) {
		if len(s.Words) == 0 {
			continue
		}
		r.Sentences++
		syl := 0
		for _, w := range s.Words {
			syl += Syllables(w.Text)
		}
		r.Words += len(s.Words)
		r.Syllables += syl
		if score := 0.39*float64(len(s.Words)) + 11.8*float64(syl)/float64(len(s.Words)); score > worstScore {
			worst, worstScore = s.Text, score
		}
	}
	if r.Words == 0 || r.Sentences == 0 {
		return r
	}
	r.Grade = 0.39*float64(r.Words)/float64(r.Sentences) + 11.8*float64(r.Syllables)/float64(r.Words) - 15.59
	r.Longest = worst
	return r
}

func hardest(r Reading) string {
	if r.Longest == "" {
		return ""
	}
	return fmt.Sprintf("; hardest sentence: %q", clip(r.Longest, 120))
}

var (
	sylTrailing = regexp.MustCompile(`(?:[^laeiouy]es|ed|[^laeiouy]e)$`)
	sylVowels   = regexp.MustCompile(`[aeiouy]{1,2}`)
)

// Syllables estimates the syllables of an English word with the common
// vowel-group heuristic: drop a silent trailing e, es, or ed, then count
// groups of one or two vowels. Hyphenated words add their parts; numbers
// and code spans count one.
func Syllables(w string) int {
	if w == codeToken {
		return 1
	}
	if strings.ContainsAny(w, "-") {
		n := 0
		for _, p := range strings.Split(w, "-") {
			n += Syllables(p)
		}
		return max(n, 1)
	}
	w = strings.ToLower(strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) {
			return r
		}
		return -1
	}, w))
	if len(w) <= 3 {
		return 1
	}
	w = sylTrailing.ReplaceAllString(w, "")
	w = strings.TrimPrefix(w, "y")
	return max(len(sylVowels.FindAllString(w, -1)), 1)
}
