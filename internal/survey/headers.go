package survey

import (
	"regexp"
	"strings"
)

var (
	accessLabel = regexp.MustCompile(`\b(public|private|protected)\s*:($|[^:])`)
	typeOpen    = regexp.MustCompile(`^(?:template\s*<[^{}]*>\s*)?(class|struct|union)\s+(?:[A-Z_][A-Z0-9_]*\s+)?([A-Za-z_]\w*)\b[^(]*$`)
	scopeOpen   = regexp.MustCompile(`^(?:(?:inline\s+)?namespace\b[\w\s:]*|extern\s*"[^"]*")$`)
	// annotation names the function-like attributes a declaration may start with.
	annotation = map[string]bool{"__declspec": true, "__attribute__": true, "alignas": true, "_Alignas": true, "__pragma": true, "_Pragma": true}
	notDecl    = regexp.MustCompile(`^(?:typedef|using|friend|return|static_assert|enum|template\s*<[^>]*>\s*(?:class|struct))\b`)
)

// blankAnnotations blanks the attributes a declaration may carry, with
// their arguments ([[deprecated("old")]], __attribute__((visibility(…))),
// __declspec(dllexport), alignas(16)), keeping every offset and line break.
func blankAnnotations(h string) string {
	b := []byte(h)
	blank := func(from, to int) {
		for k := from; k < to; k++ {
			if b[k] != '\n' {
				b[k] = ' '
			}
		}
	}
	for i := 0; i < len(b); i++ {
		if b[i] == '[' && i+1 < len(b) && b[i+1] == '[' {
			if j := strings.Index(string(b[i+2:]), "]]"); j >= 0 {
				blank(i, i+2+j+2)
				i += j + 3
			}
			continue
		}
		if (i > 0 && (isWord(b[i-1]))) || !isWord(b[i]) {
			continue
		}
		j := i
		for j < len(b) && isWord(b[j]) {
			j++
		}
		k := j
		for k < len(b) && (b[k] == ' ' || b[k] == '\t' || b[k] == '\n') {
			k++
		}
		if !annotation[string(b[i:j])] || k >= len(b) || b[k] != '(' {
			i = j - 1
			continue
		}
		depth, end := 0, len(b)
		for e := k; e < len(b); e++ {
			if b[e] == '(' {
				depth++
			} else if b[e] == ')' {
				if depth--; depth == 0 {
					end = e + 1
					break
				}
			}
		}
		blank(i, end)
		i = end - 1
	}
	return string(b)
}

func isWord(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

// headerAPI finds the functions a C or C++ header declares without a body
// (in its stripped code: comments, literals, and directives blanked):
// prototypes at file, namespace, or extern "C" scope, and the member
// functions in a class's public section (a struct's members are public until
// a label says otherwise). A library's callers reach these, so each is an
// entry point, even when its body lives in a .cpp file whose definitions the
// index does not mark public. Destructors and operators are left out: a
// caller never asks for them by name.
func headerAPI(code string) []Symbol {
	type open struct {
		class, other bool
		name         string
		public       bool // the class's current section is public, and the class itself is reachable
		reachable    bool
	}
	var stack []open
	var out []Symbol
	reachable := func() bool {
		for _, s := range stack {
			if s.other || s.class && !s.public {
				return false
			}
		}
		return true
	}
	// labels applies the access labels in a chunk to the innermost class and
	// returns the text after the last one, with its offset.
	labels := func(chunk string) (string, int) {
		off := 0
		if n := len(stack); n > 0 && stack[n-1].class {
			for _, m := range accessLabel.FindAllStringSubmatchIndex(chunk, -1) {
				stack[n-1].public = chunk[m[2]:m[3]] == "public" && stack[n-1].reachable
				off = m[3] + 1
			}
		}
		return chunk[off:], off
	}
	decl := func(chunk string, line int) {
		if n := len(stack); n > 0 && stack[n-1].other {
			return
		}
		text, off := labels(chunk)
		if !reachable() {
			return
		}
		h := blankAnnotations(strings.TrimSpace(text)) // __declspec(dllexport) int f(); declares f
		if strings.TrimSpace(h) == "" || notDecl.MatchString(h) {
			return
		}
		for _, m := range funcName.FindAllStringSubmatchIndex(h, -1) {
			nm := h[m[2]:m[3]]
			if cKeywords[nm] {
				continue
			}
			before := strings.TrimSpace(h[:m[2]])
			owner := ""
			if n := len(stack); n > 0 && stack[n-1].class {
				owner = stack[n-1].name
			}
			ctor := owner != "" && nm == owner
			switch {
			case strings.HasPrefix(nm, "~") || strings.HasPrefix(nm, "operator"),
				strings.Contains(before, "=") || strings.HasSuffix(before, ".") || strings.HasSuffix(before, "->"),
				before == "" && !ctor, // a macro call, or a pointer to a function
				!strings.Contains(h[m[3]:], ")"):
				return
			}
			q := nm // qualified by every named scope, as the definition's symbol is: a::Foo::bar
			for k := len(stack) - 1; k >= 0; k-- {
				if stack[k].name != "" {
					q = stack[k].name + "::" + q
				}
			}
			at := off + len(text) - len(strings.TrimLeft(text, " \t\r\n")) + m[2]
			ln := line + strings.Count(chunk[:at], "\n")
			kind := "function"
			if owner != "" {
				kind = "method"
			}
			out = append(out, Symbol{Kind: kind, Name: q, line: ln})
			return
		}
	}
	line, start, startLine, parens := 1, 0, 1, 0
	for i := 0; i < len(code); i++ {
		switch code[i] {
		case '\n':
			line++
		case '(':
			parens++
		case ')':
			if parens > 0 {
				parens--
			}
		case ';':
			if parens == 0 {
				decl(code[start:i], startLine)
				start, startLine = i+1, line
			}
		case '{':
			o := open{other: true}
			if parens == 0 && (len(stack) == 0 || !stack[len(stack)-1].other) {
				text, _ := labels(code[start:i])
				h := strings.Join(strings.Fields(blankAnnotations(text)), " ") // class __declspec(dllexport) Foo
				if m := typeOpen.FindStringSubmatch(h); m != nil {
					r := reachable()
					o = open{class: true, name: m[2], reachable: r, public: r && m[1] != "class"}
				} else if scopeOpen.MatchString(h) {
					o = open{}
					if f := strings.Fields(strings.TrimPrefix(h, "inline ")); len(f) > 1 && f[0] == "namespace" {
						o.name = strings.Join(f[1:], "")
					}
				}
			}
			stack = append(stack, o)
			start, startLine, parens = i+1, line, 0
		case '}':
			if n := len(stack); n > 0 {
				stack = stack[:n-1]
			}
			start, startLine, parens = i+1, line, 0
		}
	}
	return out
}
