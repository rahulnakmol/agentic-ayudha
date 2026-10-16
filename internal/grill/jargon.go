package grill

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// JargonFile holds the abbreviations and jargon the gate unpacks, as data:
// a Plain list that never needs unpacking and one line per known term with
// its usual meanings.
const JargonFile = "skills/grill/references/jargon.md"

// Jargon is the parsed JargonFile.
type Jargon struct {
	Plain []string            `json:"plain"`
	Terms map[string][]string `json:"terms"` // lower-cased term → its usual meanings
}

// Abbreviation is a term of a request whose meaning the human must confirm.
type Abbreviation struct {
	Text     string   `json:"text"`               // as the request writes it
	Meanings []string `json:"meanings,omitempty"` // the usual meanings; none: unknown, ask
	Pos      int      `json:"-"`
}

var jargonLine = regexp.MustCompile(`^- ([^:]+): (.+)$`)

func parseJargon(s string) (Jargon, error) {
	j := Jargon{Terms: map[string][]string{}}
	inPlain := false
	var plain []string
	for i, line := range strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n") {
		if p, ok := strings.CutPrefix(line, "Plain:"); ok {
			plain, inPlain = append(plain, p), true
			continue
		}
		if inPlain {
			if strings.TrimSpace(line) == "" {
				inPlain = false
			} else {
				plain = append(plain, line)
			}
			continue
		}
		if m := jargonLine.FindStringSubmatch(line); m != nil {
			key := strings.ToLower(strings.TrimSpace(m[1]))
			if _, dup := j.Terms[key]; dup {
				return j, fmt.Errorf("line %d: %q is listed twice", i+1, m[1])
			}
			for _, mean := range strings.Split(m[2], "|") {
				if mean = strings.TrimSpace(mean); mean != "" {
					j.Terms[key] = append(j.Terms[key], mean)
				}
			}
			continue
		}
		if t := strings.TrimSpace(line); t != "" && !strings.HasPrefix(t, "#") && strings.HasPrefix(t, "- ") {
			return j, fmt.Errorf("line %d: want `- <term>: <meaning> | <meaning>`", i+1)
		}
	}
	for _, p := range strings.Split(strings.Join(plain, " "), ",") {
		if p = strings.TrimSpace(p); p != "" {
			j.Plain = append(j.Plain, strings.ToLower(p))
		}
	}
	if len(j.Plain) == 0 || len(j.Terms) == 0 {
		return j, fmt.Errorf("needs a Plain: list and at least one `- <term>: <meaning>` line")
	}
	sort.Strings(j.Plain)
	return j, nil
}

var (
	termToken  = regexp.MustCompile(`[A-Za-z0-9][A-Za-z0-9&+]*`)
	capsShape  = regexp.MustCompile(`^[A-Z0-9&+]*[A-Z][A-Z0-9&+]*[A-Z0-9]s?$`) // SSO, KPIs, B2B, P&L
	numeronym  = regexp.MustCompile(`^[a-z][0-9]{1,2}[a-z]{1,2}$`)             // a11y, i18n, k8s
	identifier = "_/$=.\\-@#"
)

// Abbreviations returns the terms of text outside quotes that are not plain:
// all-capital abbreviations (SSO, KPIs, B2B, P&L), numeronyms (a11y), and
// any term the jargon file lists, each with its usual meanings. A token that
// is part of an identifier, a path, a flag, or an environment variable is
// not a term.
func (c Catalog) Abbreviations(text string) []Abbreviation {
	s := Unquoted(text)
	var out []Abbreviation
	seen := map[string]bool{}
	for _, m := range termToken.FindAllStringIndex(s, -1) {
		tok := s[m[0]:m[1]]
		if m[0] > 0 && strings.ContainsRune(identifier, rune(s[m[0]-1])) {
			continue
		}
		if m[1] < len(s) && (strings.ContainsRune("_/$=(", rune(s[m[1]])) || s[m[1]] == '-' && m[1]+1 < len(s) && s[m[1]+1] != ' ') {
			continue
		}
		key := strings.ToLower(tok)
		meanings, known := c.Jargon.Terms[key]
		if !known && capsShape.MatchString(tok) && strings.HasSuffix(tok, "s") && len(tok) > 2 {
			if mm, ok := c.Jargon.Terms[key[:len(key)-1]]; ok {
				meanings, known, key = mm, true, key[:len(key)-1]
			}
		}
		base := key
		if capsShape.MatchString(tok) && strings.HasSuffix(tok, "s") {
			base = strings.TrimSuffix(key, "s")
		}
		if c.plain(key) || c.plain(base) || seen[base] {
			continue
		}
		if !known && !capsShape.MatchString(tok) && !numeronym.MatchString(tok) {
			continue
		}
		seen[base] = true
		out = append(out, Abbreviation{Text: text[m[0]:m[1]], Meanings: meanings, Pos: m[0]})
	}
	return out
}

func (c Catalog) plain(key string) bool {
	i := sort.SearchStrings(c.Jargon.Plain, key)
	return i < len(c.Jargon.Plain) && c.Jargon.Plain[i] == key
}
