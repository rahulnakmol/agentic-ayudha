package survey

import (
	"fmt"
	"path"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// MaxOpenQuestions is the most open questions a draft may hold. The skill's
// interview asks three to five questions a round, riskiest first, and adds
// the gaps an answer opens as new questions in the next round; so the open
// questions of a draft are the round the person is asked next, and a round is
// at most five. A further gap is merged into a question one answer settles,
// or is a risk that carries its default.
const MaxOpenQuestions = 5

// checkCapabilityMap is the capability map's floor: every entry point of the
// index (a program's main, a script, an exported function or type) is cited
// by a capability claim, by its entry ID or location, a line range that holds
// it, or the symbol that defines it. A file cited whole covers nothing: the
// map says which capability each entry point serves.
func checkCapabilityMap(ix *Index, d Draft, cov *Coverage, add func(string, ...any)) {
	syms := map[string]Symbol{}
	for _, s := range ix.Symbols {
		syms[s.ID] = s
	}
	var cites []string
	for _, c := range d.Capabilities {
		cites = append(cites, c.Cites...)
	}
	missing := map[string][]string{}
	for _, e := range ix.Entries {
		if slices.ContainsFunc(cites, func(c string) bool { return coversEntry(ix, syms, strings.TrimSpace(c), e) }) {
			cov.EntriesCovered++
			continue
		}
		cov.UncoveredEntry = append(cov.UncoveredEntry, e.ID)
		f, n := splitAt(e.At)
		missing[f] = append(missing[f], itoa(n)+" ("+e.Detail+")")
	}
	for _, f := range sortedKeys(missing) {
		add("capabilities: no capability covers the entry point(s) %s:%s; cite each from the capability it serves (its entry, its line or a range that holds it, or its symbol), or add the capability", f, strings.Join(missing[f], ", "))
	}
}

// coversEntry reports whether one citation of a capability claim covers an
// entry point.
func coversEntry(ix *Index, syms map[string]Symbol, c string, e Fact) bool {
	if c == e.ID || c == e.At {
		return true
	}
	file, line := splitAt(e.At)
	if s, ok := syms[c]; ok {
		return s.At == e.At || isDefKind(s.Kind) && s.Name == entryName(ix, e)
	}
	j := strings.LastIndex(c, ":")
	if j < 0 || c[:j] != file {
		return false
	}
	first, last, _ := strings.Cut(c[j+1:], "-")
	if last == "" {
		last = first
	}
	a, err1 := strconv.Atoi(first)
	b, err2 := strconv.Atoi(last)
	return err1 == nil && err2 == nil && a <= line && line <= b
}

// entryName is the qualified name of the function an entry point declares
// ("CTinyJS::getVariable"), so the symbol of its definition in another file
// covers it; "" for a type or a script.
func entryName(ix *Index, e Fact) string {
	if kind, name, ok := strings.Cut(e.Detail, " "); ok && (kind == "method" || kind == "function") {
		return name
	}
	if e.Kind == "main" {
		for _, s := range ix.Symbols {
			if s.At == e.At && isDefKind(s.Kind) {
				return s.Name
			}
		}
	}
	return ""
}

// checkRunners is the case sets' floor: when the checkout has more than one
// program (main or script entries), each case set names the runners that run
// it (a program, or a test harness that drives the library); every input it names is a file of the checkout; and every builtin its
// cases call (in an input file, or named in the case set's title or
// comparison) is registered by each of its runners. A builtin is a name a source file registers for the
// program it embeds (Index.Builtins); a runner registers it when the
// registration is reachable from the runner's main through the call graph.
func checkRunners(ix *Index, m *Modernisation, add func(string, ...any)) {
	var runnable []Fact
	programs := map[string]bool{} // a file with a shebang and a main guard is one program
	for _, e := range ix.Entries {
		if e.Kind == "main" || e.Kind == "script" {
			runnable = append(runnable, e)
			f, _ := splitAt(e.At)
			programs[f] = true
		}
	}
	registered := map[string][]Fact{}
	for _, b := range ix.Builtins {
		registered[b.Detail] = append(registered[b.Detail], b)
	}
	reach, drives := runnerReach(ix)
	for _, g := range m.CaseSets {
		if len(programs) > 1 && len(g.Runners) == 0 {
			add("case set %s: name its runners (the programs that run it, of %s), and its inputs when they are files of the checkout: this checkout has %d programs", g.ID, listAt(runnable, 4), len(programs))
		}
		var runners []Fact
		for _, r := range g.Runners {
			k := slices.IndexFunc(runnable, func(e Fact) bool { return e.ID == r || e.At == r })
			switch t := slices.IndexFunc(ix.Tests, func(e Fact) bool { return e.ID == r }); {
			case k >= 0:
				runners = append(runners, runnable[k])
			case t >= 0: // a test harness that runs the cases through the library
				h := Fact{ID: ix.Tests[t].ID, Kind: "script", At: ix.Tests[t].At}
				if !drives(h) {
					add("case set %s: runner %s is a test file that calls no function and loads no file outside the tests, so it cannot run the cases through the program: name a program or a harness that drives it", g.ID, r)
					continue
				}
				runners = append(runners, h)
			default:
				add("case set %s: runner %q is not a main or script entry of the index (%s), nor a test file", g.ID, r, listAt(runnable, 4))
			}
		}
		files := map[string]bool{}
		for _, in := range g.Inputs {
			n := 0
			for _, f := range ix.Files {
				if ok, _ := path.Match(in, f.Path); ok || in == f.Path {
					files[f.Path] = true
					n++
				}
			}
			if n == 0 {
				add("case set %s: input %q matches no file of the checkout", g.ID, in)
			}
		}
		for f := range files {
			for _, x := range g.Exclude {
				if strings.Contains(x, f) {
					delete(files, f)
				}
			}
		}
		used := map[string]string{} // builtin → where the case set calls it
		for _, u := range ix.Uses {
			if f, _ := splitAt(u.At); files[f] {
				if _, ok := used[u.Detail]; !ok || lessAt(u.At, used[u.Detail]) {
					used[u.Detail] = u.At
				}
			}
		}
		text := g.Title + "\n" + g.Compare
		for name := range registered {
			if _, ok := used[name]; !ok && mentions(text, name) {
				used[name] = "its title or comparison"
			}
		}
		for _, r := range runners {
			for _, name := range sortedKeys(used) {
				if !reach(r, registered[name]) {
					add("case set %s: %s calls %s, which runner %s does not register (registered at %s): put those cases in a case set whose runner registers it, or exclude them naming the claim or question that decided it", g.ID, used[name], name, r.ID, listAt(registered[name], 3))
				}
			}
		}
	}
}

// runnerReach returns whether any of a builtin's registrations is reachable
// from a runner: in a function the runner's main reaches through call edges
// (ambiguous edges count, so the floor never fails on a call it cannot
// resolve), or at file scope in a file the runner loads (its own, or one
// defining a function it reaches). It also returns whether a runner drives
// the program: it reaches a function defined outside the test files, or
// imports or includes such a file.
func runnerReach(ix *Index) (reach func(Fact, []Fact) bool, drives func(Fact) bool) {
	next := map[string][]string{}
	for _, e := range ix.Edges {
		if e.Kind == "call" {
			next[e.From] = append(next[e.From], e.To...)
		}
	}
	byFile := map[string][]Symbol{}
	for _, s := range ix.Symbols {
		f, _ := splitAt(s.At)
		byFile[f] = append(byFile[f], s)
	}
	enclosing := func(at string) string {
		f, line := splitAt(at)
		best, bestLine := "", 0
		for _, s := range byFile[f] {
			_, l := splitAt(s.At)
			if isDefKind(s.Kind) && l <= line && line <= s.End && l >= bestLine {
				best, bestLine = s.ID, l
			}
		}
		return best
	}
	memo := map[string]map[string]bool{}
	reached := func(r Fact) map[string]bool {
		if m, ok := memo[r.ID]; ok {
			return m
		}
		seen := map[string]bool{}
		var queue []string
		f, _ := splitAt(r.At)
		for _, s := range byFile[f] {
			if isDefKind(s.Kind) && (s.At == r.At || r.Kind == "script") {
				queue = append(queue, s.ID)
			}
		}
		for len(queue) > 0 {
			id := queue[0]
			queue = queue[1:]
			if seen[id] {
				continue
			}
			seen[id] = true
			queue = append(queue, next[id]...)
		}
		memo[r.ID] = seen
		return seen
	}
	// loads reports whether a runner loads a file: the runner's own file, or a
	// file defining a function the runner reaches.
	loads := func(r Fact, file string) bool {
		if f, _ := splitAt(r.At); f == file {
			return true
		}
		for _, s := range byFile[file] {
			if reached(r)[s.ID] {
				return true
			}
		}
		return false
	}
	reach = func(r Fact, regs []Fact) bool {
		for _, b := range regs {
			enc := enclosing(b.At)
			if f, _ := splitAt(b.At); enc == "" && loads(r, f) || enc != "" && reached(r)[enc] {
				return true
			}
		}
		return false
	}
	test := map[string]bool{}
	for _, f := range ix.Files {
		test[f.Path] = f.Role == "test"
	}
	loaded := map[string][]string{} // file → the checkout files it imports or includes
	for _, e := range ix.Edges {
		if e.Kind != "call" && strings.HasPrefix(e.From, "file:") {
			for _, to := range e.To {
				if f, ok := strings.CutPrefix(to, "file:"); ok {
					loaded[strings.TrimPrefix(e.From, "file:")] = append(loaded[strings.TrimPrefix(e.From, "file:")], f)
				}
			}
		}
	}
	drives = func(r Fact) bool {
		for f, syms := range byFile {
			for _, s := range syms {
				if !test[f] && reached(r)[s.ID] {
					return true
				}
			}
		}
		// A spec calls the program at file scope, where the index records no
		// calls: it drives the program when it loads a file outside the tests,
		// itself or through a test helper.
		f, _ := splitAt(r.At)
		seen := map[string]bool{f: true}
		for queue := []string{f}; len(queue) > 0; queue = queue[1:] {
			for _, to := range loaded[queue[0]] {
				if _, ok := test[to]; ok && !test[to] {
					return true
				}
				if !seen[to] {
					seen[to] = true
					queue = append(queue, to)
				}
			}
		}
		return false
	}
	return reach, drives
}

// mentions reports whether text names a builtin: a dotted name anywhere
// (Math.sqrt), a plain name only as a call (print(…)).
func mentions(text, name string) bool {
	pat := `(?:^|[^\w$.])` + regexp.QuoteMeta(name)
	if strings.Contains(name, ".") {
		pat += `(?:$|[^\w$])`
	} else {
		pat += `\s*\(`
	}
	return regexp.MustCompile(pat).MatchString(text)
}

func listAt(fs []Fact, n int) string {
	var at []string
	for _, f := range fs {
		at = append(at, f.ID)
	}
	sort.Strings(at)
	if len(at) > n {
		return strings.Join(at[:n], ", ") + fmt.Sprintf(", and %d more", len(at)-n)
	}
	return strings.Join(at, ", ")
}
