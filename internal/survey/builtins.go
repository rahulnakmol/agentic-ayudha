package survey

import (
	"regexp"
	"sort"
	"strings"
)

var (
	// registerCall is a call that registers a builtin: a string literal naming
	// it followed by a reference to the function that implements it
	// (addNative("function Math.abs(a)", scMathAbs, 0), lua_register(L, "f", f),
	// register("f", handlers.f),
	// Tcl_CreateObjCommand(interp, "cmd", Cmd, …), register('name', fn)).
	registerCall = regexp.MustCompile(`\b[A-Za-z_]\w*\s*\(\s*(?:[^()"';{}]*?,\s*)?(?:"([^"\\\n]{1,200})"|'([^'\\\n]{1,200})')\s*,\s*&?\s*([A-Za-z_][\w:.]*)\s*[,)]`)
	builtinName  = regexp.MustCompile(`^[A-Za-z_$][\w$]*(?:\.[A-Za-z_$][\w$]*)*$`)
)

// registeredName is the name a registration string gives its builtin: the
// last word before the parameter list ("function Math.abs(a)" is Math.abs),
// or the whole string when it is a name; "" otherwise.
func registeredName(s string) string {
	if i := strings.Index(s, "("); i >= 0 {
		s = s[:i]
	}
	f := strings.Fields(s)
	if len(f) == 0 {
		return ""
	}
	if n := f[len(f)-1]; builtinName.MatchString(n) {
		return n
	}
	return ""
}

// builtins records the builtins each source file registers (Index.Builtins)
// and where any file calls one (Index.Uses; an input of the same language as
// the host counts too), so a case set can be
// checked against the builtins its runner registers. A registration counts
// only when its function argument is a function of the index and its call is
// code, not a comment.
func (ix *Index) builtins(files map[string]*scanned, defs map[string][]string) {
	names := map[string]bool{}
	for _, p := range sortedKeys(files) {
		s := files[p]
		k, ok := ix.byPath[p]
		if !ok || s.code == "" || ix.Files[k].Role != "source" && ix.Files[k].Role != "test" {
			continue
		}
		for _, m := range registerCall.FindAllStringSubmatchIndex(s.raw, -1) {
			if m[0] >= len(s.code) || s.code[m[0]] != s.raw[m[0]] {
				continue // inside a comment
			}
			lit := m[2:4]
			if lit[0] < 0 {
				lit = m[4:6]
			}
			name := registeredName(s.raw[lit[0]:lit[1]])
			if name == "" || len(defs[lastName(strings.ReplaceAll(s.raw[m[6]:m[7]], ".", "::"))]) == 0 {
				continue
			}
			at := p + ":" + itoa(strings.Count(s.raw[:lit[0]], "\n")+1)
			ix.Builtins = append(ix.Builtins, Fact{ID: "builtin:" + at, Kind: "builtin", At: at, Detail: name})
			names[name] = true
		}
	}
	if len(names) == 0 {
		return
	}
	var pats []string
	for n := range names {
		pats = append(pats, regexp.QuoteMeta(n))
	}
	sort.Strings(pats)
	use := regexp.MustCompile(`(?:^|[^\w$.])(` + strings.Join(pats, "|") + `)\s*\(`)
	for _, p := range sortedKeys(files) {
		s := files[p]
		if _, ok := ix.byPath[p]; !ok {
			continue
		}
		text := s.code // literals and comments blanked: a name in a string or a comment is not a call
		if text == "" {
			text = s.raw
		}
		seen := map[string]bool{}
		for _, m := range use.FindAllStringSubmatchIndex(text, -1) {
			n := text[m[2]:m[3]]
			if seen[n] {
				continue
			}
			seen[n] = true
			at := p + ":" + itoa(strings.Count(text[:m[2]], "\n")+1)
			ix.Uses = append(ix.Uses, Fact{ID: "use:" + n + ":" + at, Kind: "use", At: at, Detail: n})
		}
	}
}
