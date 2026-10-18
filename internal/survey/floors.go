package survey

import (
	"cmp"
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
	// A C or C++ definition's symbol covers the declaration of its name: each
	// cited definition covers one declaration of the name in each header, so
	// every overload a header declares needs its own cite, and a function
	// declared in two headers is covered by citing its one definition. No
	// other entry is covered by a name it shares (parse in lib.js is not
	// parse in lib.rs).
	cited := map[string]map[string]bool{}
	for _, c := range cites {
		if sy, ok := syms[strings.TrimSpace(c)]; ok && isDefKind(sy.Kind) && cFamily(sy.At) {
			if cited[sy.Name] == nil {
				cited[sy.Name] = map[string]bool{}
			}
			cited[sy.Name][sy.ID] = true
		}
	}
	used := map[string]int{} // per header and name: each cited definition covers one declaration
	missing := map[string][]string{}
	for _, e := range ix.Entries {
		direct := slices.ContainsFunc(cites, func(c string) bool { return coversEntry(ix, syms, strings.TrimSpace(c), e) })
		f, _ := splitAt(e.At)
		if n := entryName(ix, e); !direct && n != "" && isHeader(f) && used[f+" "+n] < len(cited[n]) {
			used[f+" "+n]++
			direct = true
		}
		if direct {
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

// cFamily reports whether a symbol's file (path:line) is C or C++.
func cFamily(at string) bool {
	f, _ := splitAt(at)
	switch strings.ToLower(path.Ext(f)) {
	case ".c", ".cc", ".cpp", ".cxx", ".c++", ".h", ".hh", ".hpp", ".hxx", ".inl", ".ipp":
		return true
	}
	return false
}

// citedLines resolves a citation of a checkout file (file:p, p, p:N, or
// p:N-M) to its path and lines (0, 0 for the whole file); "" for any other
// citation.
func citedLines(ix *Index, c string) (file string, first, last int) {
	c = strings.TrimPrefix(c, "file:")
	isFile := func(p string) bool { return slices.ContainsFunc(ix.Files, func(f File) bool { return f.Path == p }) }
	if isFile(c) {
		return c, 0, 0
	}
	j := strings.LastIndex(c, ":")
	if j < 0 || !isFile(c[:j]) {
		return "", 0, 0
	}
	a, b, _ := strings.Cut(c[j+1:], "-")
	first, err1 := strconv.Atoi(a)
	last, err2 := strconv.Atoi(cmp.Or(b, a))
	if err1 != nil || err2 != nil || first < 1 || last < first {
		return "", 0, 0
	}
	return c[:j], first, last
}

// coversEntry reports whether one citation of a capability claim covers an
// entry point.
func coversEntry(ix *Index, syms map[string]Symbol, c string, e Fact) bool {
	if c == e.ID || c == e.At {
		return true
	}
	file, line := splitAt(e.At)
	if s, ok := syms[c]; ok {
		// The symbol of the entry's own definition; one elsewhere covers by name
		// (checkCapabilityMap). Its name, not only its line, tells same-line
		// entries apart.
		name := e.Detail
		if e.Kind == "api" {
			_, name, _ = strings.Cut(e.Detail, " ")
		}
		return s.At == e.At && s.Name == name
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
// comparison) is registered by each of its runners; with one program and no
// runners named, that program is the runner. With no inputs named,
// the checkout's test files the case set draws on (from) are its inputs. A builtin is a name a source file registers for the
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
		if len(programs) == 1 && len(g.Runners) == 0 {
			runners = runnable // the only program runs it
		}
		for _, r := range g.Runners {
			k := slices.IndexFunc(runnable, func(e Fact) bool { return e.ID == r || e.At == r })
			switch t := slices.IndexFunc(ix.Tests, func(e Fact) bool { return e.ID == r }); {
			case k >= 0:
				runners = append(runners, runnable[k])
			case t >= 0: // a test harness that runs the cases through the library
				h := Fact{ID: ix.Tests[t].ID, Kind: "test", At: ix.Tests[t].At}
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
		inputs := g.Inputs
		lines := map[string][][2]int{} // a file the cases are drawn from by line: those lines only
		if len(inputs) == 0 {          // the checkout files the case set draws on are its inputs: test files, and files or lines it cites
			whole := map[string]bool{}
			for _, from := range g.From {
				if slices.ContainsFunc(g.Runners, func(r string) bool { return r == from }) {
					continue
				}
				if slices.ContainsFunc(ix.Tests, func(e Fact) bool { return e.ID == from }) {
					inputs = append(inputs, strings.TrimPrefix(from, "test:"))
					continue
				}
				f, a, b := citedLines(ix, from)
				switch {
				case f == "":
				case a == 0:
					inputs, whole[f] = append(inputs, f), true
				default:
					inputs, lines[f] = append(inputs, f), append(lines[f], [2]int{a, b})
				}
			}
			for f := range whole {
				delete(lines, f)
			}
		}
		for _, in := range inputs {
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
		// An exclusion that names a file's lines (tests/math.js:12-20) leaves
		// out those cases; one that names the file alone leaves out the file.
		skip := map[string][][2]int{}
		for f := range files {
			named := regexp.MustCompile(`(?:^|[^\w./-])` + regexp.QuoteMeta(f) + `(?::(\d+)(?:-(\d+))?)?(?:$|[^\w/-]|\.(?:$|\s))`)
			for _, x := range g.Exclude {
				for _, m := range named.FindAllStringSubmatch(x, -1) { // this path, not one that ends with it
					if m[1] == "" {
						delete(files, f)
						continue
					}
					a, _ := strconv.Atoi(m[1])
					b, _ := strconv.Atoi(cmp.Or(m[2], m[1]))
					skip[f] = append(skip[f], [2]int{a, b})
				}
			}
		}
		in := func(l int, rs [][2]int) bool {
			return slices.ContainsFunc(rs, func(r [2]int) bool { return r[0] <= l && l <= r[1] })
		}
		used := map[string]string{} // builtin → where the case set calls it
		for _, u := range ix.Uses {
			if f, l := splitAt(u.At); files[f] && (lines[f] == nil || in(l, lines[f])) && !in(l, skip[f]) {
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
// resolve), or at file scope in a file the runner loads (its own, one built
// into the same executable, one defining a function it reaches, or one those
// import or include). It also returns whether a runner drives
// the program: it reaches a function defined outside the test files, or
// imports or includes such a file.
func runnerReach(ix *Index) (reach func(Fact, []Fact) bool, drives func(Fact) bool) {
	defs := map[string][]string{}
	for _, s := range ix.Symbols {
		if isDefKind(s.Kind) {
			defs[lastName(s.Name)] = append(defs[lastName(s.Name)], s.ID)
		}
	}
	next := map[string][]string{}
	for _, e := range ix.Edges {
		if e.Kind == "call" {
			next[e.From] = append(next[e.From], e.To...)
			if e.Name != "" { // an edge that lists only some of the name's definitions: all of them
				next[e.From] = append(next[e.From], defs[e.Name]...)
			}
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
			if isDefKind(s.Kind) && (s.At == r.At || r.Kind == "test") { // a test framework runs every test function
				queue = append(queue, s.ID)
			}
		}
		queue = append(queue, next[r.ID]...) // a script's or main guard's calls at file scope
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
	// load is an import or include: the file it loads, and the function that
	// holds it ("" at file scope), which must run for the load to happen.
	type load struct{ file, in string }
	loaded := map[string][]load{} // file → the checkout files it imports or includes
	for _, e := range ix.Edges {
		if e.Kind != "call" && strings.HasPrefix(e.From, "file:") {
			for _, to := range e.To {
				if f, ok := strings.CutPrefix(to, "file:"); ok {
					from := strings.TrimPrefix(e.From, "file:")
					loaded[from] = append(loaded[from], load{f, enclosing(e.At)})
				}
			}
		}
	}
	// loadsMemo holds the files a runner loads: its own, each file defining a
	// function it reaches, and every file those import or include (an import
	// runs a module's file-scope code; an included header's static
	// initialisers run in the program).
	loadsMemo := map[string]map[string]bool{}
	loads := func(r Fact, file string) bool {
		if m, ok := loadsMemo[r.ID]; ok {
			return m[file]
		}
		f, _ := splitAt(r.At)
		m := map[string]bool{f: true}
		queue := []string{f}
		for _, t := range ix.Targets { // the translation units linked with the runner's file
			if (t.Kind == "executable" || t.Kind == "test") && slices.Contains(t.Sources, f) {
				for _, src := range t.Sources {
					if !m[src] {
						m[src] = true
						queue = append(queue, src)
					}
				}
			}
		}
		for g, syms := range byFile {
			for _, s := range syms {
				if !m[g] && reached(r)[s.ID] {
					m[g] = true
					queue = append(queue, g)
				}
			}
		}
		for ; len(queue) > 0; queue = queue[1:] {
			for _, l := range loaded[queue[0]] {
				if !m[l.file] && (l.in == "" || reached(r)[l.in]) { // an import inside a function loads only when it runs
					m[l.file] = true
					queue = append(queue, l.file)
				}
			}
		}
		loadsMemo[r.ID] = m
		return m[file]
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
			for _, l := range loaded[queue[0]] {
				if l.in != "" && !reached(r)[l.in] {
					continue // an import inside a function the harness never runs
				}
				if _, ok := test[l.file]; ok && !test[l.file] {
					return true
				}
				if !seen[l.file] {
					seen[l.file] = true
					queue = append(queue, l.file)
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
