package survey

import (
	"regexp"
	"strings"
)

// comment is one comment's text, line by line, with the line it starts on.
type comment struct {
	line  int // 1-based line of the first text line
	lines []string
}

// strip blanks a source file's comments and the contents of its string
// literals, keeping every newline (so line numbers hold) and each literal's
// quotes, and returns the comments it removed. hashComments adds # comments
// (PHP, and the whole hash family); the hash family has no // or /* */.
func strip(src, syntax, lang string) (code string, comments []comment) {
	b := []byte(src)
	out := make([]byte, len(b))
	copy(out, b)
	line := 1
	blank := func(i int) {
		if out[i] != '\n' {
			out[i] = ' '
		}
	}
	addComment := func(start int, text string) {
		comments = append(comments, comment{line: start, lines: strings.Split(text, "\n")})
	}
	cFamily := syntax == syntaxC
	hash := syntax == syntaxHash || syntax == syntaxBuild || lang == "php"
	sql := syntax == syntaxSQL
	for i := 0; i < len(b); i++ {
		c := b[i]
		switch {
		case c == '\n':
			line++
		case cFamily && c == '/' && i+1 < len(b) && b[i+1] == '/', hash && c == '#' && hashStarts(b, i, lang), sql && c == '-' && i+1 < len(b) && b[i+1] == '-':
			j := i
			for j < len(b) && b[j] != '\n' {
				j++
			}
			addComment(line, string(b[i:j]))
			for k := i; k < j; k++ {
				blank(k)
			}
			i = j - 1
		case cFamily && c == '/' && i+1 < len(b) && b[i+1] == '*':
			j := i + 2
			for j+1 < len(b) && (b[j] != '*' || b[j+1] != '/') {
				j++
			}
			end := min(j+2, len(b))
			addComment(line, string(b[i:end]))
			for k := i; k < end; k++ {
				if b[k] == '\n' {
					line++
				}
				blank(k)
			}
			i = end - 1
		case syntax == syntaxHash && lang == "python" && (c == '"' || c == '\'') && i+2 < len(b) && b[i+1] == c && b[i+2] == c:
			// A triple-quoted string: a docstring is documentation, kept as a comment.
			q := string([]byte{c, c, c})
			j := strings.Index(src[i+3:], q)
			end := len(b)
			if j >= 0 {
				end = i + 3 + j + 3
			}
			addComment(line, src[i:end])
			for k := i + 3; k < end-3 && k < len(b); k++ {
				if b[k] == '\n' {
					line++
				}
				blank(k)
			}
			i = end - 1
		case c == '"' || c == '`' && (lang == "javascript" || lang == "typescript" || lang == "go" || lang == "shell") || c == '\'' && charLiteral(b, i, syntax, lang):
			j := i + 1
			for j < len(b) && b[j] != c {
				if b[j] == '\\' && c != '`' {
					j++
					if j < len(b) && b[j] == '\n' {
						line++
					}
				} else if b[j] == '\n' {
					if c != '`' && c != '\'' && syntax != syntaxHash && lang != "php" {
						break // an unterminated literal ends at the line
					}
					line++
				}
				j++
			}
			for k := i + 1; k < j && k < len(b); k++ {
				blank(k)
			}
			i = min(j, len(b)-1)
			if i < len(b) && b[i] == '\n' {
				line++
			}
		}
	}
	if lang == "c" || lang == "cpp" || lang == "objc" {
		blankDirectives(out)
	}
	return string(out), comments
}

// blankDirectives blanks C preprocessor lines (and their continuations), so a
// macro body with braces is never read as a definition. Includes are read
// from the raw source.
func blankDirectives(out []byte) {
	atStart, inDirective := true, false
	for i := 0; i < len(out); i++ {
		c := out[i]
		switch {
		case c == '\n':
			cont := i > 0 && out[i-1] == '\\'
			if !cont {
				inDirective = false
			}
			atStart = true
			continue
		case atStart && c == '#':
			inDirective = true
		}
		if c != ' ' && c != '\t' {
			atStart = false
		}
		if inDirective && c != '\n' {
			out[i] = ' '
		}
	}
}

// hashStarts reports whether a # at i starts a comment: in shell only at a
// word start (not ${#x}); in PHP not as an attribute (#[…]).
func hashStarts(b []byte, i int, lang string) bool {
	if lang == "php" && i+1 < len(b) && b[i+1] == '[' {
		return false
	}
	if lang == "shell" || lang == "perl" {
		return i == 0 || b[i-1] == ' ' || b[i-1] == '\t' || b[i-1] == '\n' || b[i-1] == ';'
	}
	return true
}

// charLiteral reports whether a ' at i opens a literal. In C-family languages
// it is a character literal only when it closes within a few bytes ('a',
// '\n', '\x41'), so a Rust lifetime ('a) or a C++14 digit separator is not;
// in the hash family and PHP it is a string.
func charLiteral(b []byte, i int, syntax, lang string) bool {
	if syntax == syntaxHash || lang == "php" || lang == "javascript" || lang == "typescript" || lang == "python" {
		return true
	}
	if syntax != syntaxC {
		return false
	}
	for j := i + 1; j < len(b) && j <= i+6; j++ {
		switch b[j] {
		case '\\':
			j++
		case '\'':
			return j > i+1
		case '\n':
			return false
		}
	}
	return false
}

// Markers: inline TODO/FIXME/BUG/HACK/XXX notes, and list blocks under a
// heading such as "NOTE:", "TODO:", "Known issues:", "Bugs:", or "Limitations:".
var (
	inlineMarker  = regexp.MustCompile(`\b(TODO|FIXME|BUG|HACK|XXX)\b[:\s(]+(.*)`)
	markerHeading = regexp.MustCompile(`(?i)^\s*(?:[*#/-]+\s*)?(note|notes|todo|fixme|bugs?|known (?:issues?|bugs|problems)|limitations?|caveats?|restrictions?)\s*:\s*$`)
	commentLead   = regexp.MustCompile(`^\s*(?:/\*+|\*+/?|//+|#+|--|""")?\s?`)
)

// Marker is a note in a comment that names a known issue or unfinished work.
type Marker struct {
	ID   string `json:"id"`
	Kind string `json:"kind"` // todo, fixme, bug, hack, xxx, note, known-issue, limitation
	At   string `json:"at"`
	Text string `json:"text"`
}

// markers finds the markers in one file's comments.
func markers(file string, cs []comment) []Marker {
	var ms []Marker
	add := func(kind string, line int, text string) {
		text = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(text), "*/"))
		if text == "" {
			return
		}
		ms = append(ms, Marker{ID: "marker:" + file + ":" + itoa(line), Kind: kind, At: file + ":" + itoa(line), Text: clip(text, 200)})
	}
	for _, c := range cs {
		heading := ""
		for k, raw := range c.lines {
			ln := c.line + k
			text := strings.TrimRight(commentLead.ReplaceAllString(raw, ""), " \t*/")
			if m := markerHeading.FindStringSubmatch(raw); m != nil {
				heading = blockKind(m[1])
				continue
			}
			if heading != "" {
				if strings.TrimSpace(text) == "" {
					heading = ""
					continue
				}
				add(heading, ln, strings.TrimLeft(text, "-*• \t"))
				continue
			}
			if m := inlineMarker.FindStringSubmatch(raw); m != nil {
				add(strings.ToLower(m[1]), ln, m[2])
			}
		}
	}
	return ms
}

func blockKind(h string) string {
	h = strings.ToLower(h)
	switch {
	case strings.HasPrefix(h, "known") || strings.HasPrefix(h, "bug"):
		return "known-issue"
	case strings.HasPrefix(h, "limit") || strings.HasPrefix(h, "caveat") || strings.HasPrefix(h, "restrict"):
		return "limitation"
	case strings.HasPrefix(h, "todo"):
		return "todo"
	case strings.HasPrefix(h, "fixme"):
		return "fixme"
	}
	return "note"
}

func clip(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > n {
		return string(r[:n-1]) + "…"
	}
	return s
}
