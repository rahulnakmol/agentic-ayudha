package inspect

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"regexp"
	"slices"
	"strings"
)

// Source reads the PR head without running it: the contents API at the head
// SHA in the merge gate (WP4.8b), a map in tests. Dir lists the files (not
// directories) directly in a directory, as repository paths.
type Source interface {
	File(ctx context.Context, p string) ([]byte, error)
	Dir(ctx context.Context, dir string) ([]string, error)
}

// Context limits: what aayu fetches and sends, so a large PR cannot blow up
// the prompt or the API budget.
const (
	maxContextFiles  = 40     // files fetched per inspection
	maxFileBytes     = 200000 // larger files are not parsed
	maxSymbolSource  = 3000   // bytes of a changed symbol's source
	maxTestSource    = 1500   // bytes of a related test's source
	maxCallersPerSym = 8
	maxContextBytes  = 40000 // the context section of the prompt
)

// Symbol is a declaration the PR changed.
type Symbol struct {
	Name   string `json:"name"` // Name or Recv.Name; for non-Go files the hunk header
	File   string `json:"file"`
	Line   int    `json:"line"`
	Source string `json:"-"`
}

// Caller is a call site of a changed symbol in the same package.
type Caller struct {
	Symbol string `json:"symbol"`
	File   string `json:"file"`
	Line   int    `json:"line"`
	In     string `json:"in"`
}

// Test is a test that exercises a changed symbol (Go), or a test file beside
// the changed files (other languages).
type Test struct {
	Name   string `json:"name"`
	File   string `json:"file"`
	Line   int    `json:"line"`
	Source string `json:"-"`
}

// Area is one changed directory (a Go package) and what aayu retrieved for it:
// the review plan's unit.
type Area struct {
	Dir          string   `json:"dir"`
	Files        []string `json:"files"`
	Layers       []string `json:"layers,omitempty"`
	Symbols      []Symbol `json:"symbols,omitempty"`
	Callers      []Caller `json:"callers,omitempty"`
	Tests        []Test   `json:"tests,omitempty"`
	TestsChanged bool     `json:"tests_changed"`
	Generated    []string `json:"generated,omitempty"` // changed files that say they are generated
	Risks        []string `json:"risks"`
}

// Footprint maps changed paths to layers (verify.LayerGlobs.Footprint).
type Footprint func(paths []string) []string

var generatedHeader = regexp.MustCompile(`(?m)^(//|#|<!--|/\*)?\s*Code generated .* DO NOT EDIT`)

// BuildContext groups the diff's files into areas and retrieves each area's
// changed symbols, same-package callers, and related tests from src. Nothing
// is executed; files are parsed. A file src cannot read is reviewed from the
// diff alone.
func BuildContext(ctx context.Context, files []DiffFile, src Source, footprint Footprint) []Area {
	byDir := map[string]*Area{}
	var dirs []string
	changedTests := map[string]bool{}
	for _, f := range files {
		d := path.Dir(f.Path)
		if byDir[d] == nil {
			byDir[d] = &Area{Dir: d, Risks: []string{}}
			dirs = append(dirs, d)
		}
		byDir[d].Files = append(byDir[d].Files, f.Path)
		if isTestPath(f.Path) || strings.Contains("/"+f.Path, "/testdata/") {
			changedTests[d] = true
		}
	}
	slices.Sort(dirs)
	fetched := 0
	read := func(p string) []byte {
		if src == nil || fetched >= maxContextFiles {
			return nil
		}
		fetched++
		b, err := src.File(ctx, p)
		if err != nil || len(b) > maxFileBytes {
			return nil
		}
		return b
	}
	var out []Area
	for _, d := range dirs {
		a := byDir[d]
		a.TestsChanged = changedTests[d]
		if footprint != nil {
			a.Layers = footprint(a.Files)
		}
		var added []string
		goChanged := map[string][]int{} // changed Go file -> added lines
		isNew := map[string]bool{}      // the diff already shows a new file whole
		for _, f := range files {
			if path.Dir(f.Path) != d || f.Deleted {
				continue
			}
			for _, l := range f.Added() {
				added = append(added, l.Text)
			}
			isNew[f.Path] = f.New
			if strings.HasSuffix(f.Path, ".go") && !strings.HasSuffix(f.Path, "_test.go") {
				for _, l := range f.Added() {
					goChanged[f.Path] = append(goChanged[f.Path], l.N)
				}
				continue
			}
			for _, h := range f.Hunks {
				if h.Context != "" && !slices.ContainsFunc(a.Symbols, func(s Symbol) bool { return s.Name == h.Context && s.File == f.Path }) {
					a.Symbols = append(a.Symbols, Symbol{Name: h.Context, File: f.Path, Line: h.NewStart})
				}
			}
		}
		parsed := map[string]*ast.File{}
		srcs := map[string][]byte{}
		fset := token.NewFileSet()
		parse := func(p string) *ast.File {
			if f, ok := parsed[p]; ok {
				return f
			}
			b := read(p)
			var f *ast.File
			if b != nil {
				if generatedHeader.Match(firstLines(b, 5)) && !slices.Contains(a.Generated, p) && slices.Contains(a.Files, p) {
					a.Generated = append(a.Generated, p)
				}
				f, _ = parser.ParseFile(fset, p, b, parser.SkipObjectResolution)
				srcs[p] = b
			}
			parsed[p] = f
			return f
		}
		for _, p := range sortedFiles(goChanged) {
			f := parse(p)
			if f == nil {
				continue
			}
			for _, decl := range f.Decls {
				start, end := fset.Position(decl.Pos()).Line, fset.Position(decl.End()).Line
				if !slices.ContainsFunc(goChanged[p], func(n int) bool { return n >= start && n <= end }) {
					continue
				}
				src := ""
				if !isNew[p] {
					src = cut(lines(srcs[p], start, end), maxSymbolSource)
				}
				for _, name := range declNames(decl) {
					a.Symbols = append(a.Symbols, Symbol{Name: name, File: p, Line: start, Source: src})
				}
			}
		}
		// Callers and tests in the same package.
		var names []string
		for _, s := range a.Symbols {
			if strings.HasSuffix(s.File, ".go") {
				names = append(names, bareName(s.Name))
			}
		}
		listing, _ := listDir(ctx, src, d)
		if len(names) > 0 {
			for _, p := range listing {
				if !strings.HasSuffix(p, ".go") {
					continue
				}
				f := parse(p)
				if f == nil {
					continue
				}
				if strings.HasSuffix(p, "_test.go") {
					ts := relatedTests(fset, f, srcs[p], p, names)
					if isNew[p] { // the diff shows it whole
						for i := range ts {
							ts[i].Source = ""
						}
					}
					a.Tests = append(a.Tests, ts...)
					continue
				}
				a.Callers = append(a.Callers, callers(fset, f, p, names, a.Callers)...)
			}
		} else {
			for _, p := range listing {
				if isTestPath(p) {
					a.Tests = append(a.Tests, Test{Name: path.Base(p), File: p})
				}
			}
		}
		// Generated files that are not Go: read their header too.
		for _, p := range a.Files {
			if !strings.HasSuffix(p, ".go") && fetched < maxContextFiles {
				if b := read(p); b != nil && generatedHeader.Match(firstLines(b, 5)) {
					a.Generated = append(a.Generated, p)
				}
			}
		}
		a.Risks = risks(a, added)
		out = append(out, *a)
	}
	return out
}

func listDir(ctx context.Context, src Source, d string) ([]string, error) {
	if src == nil {
		return nil, nil
	}
	ps, err := src.Dir(ctx, d)
	slices.Sort(ps)
	return ps, err
}

func sortedFiles(m map[string][]int) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	slices.Sort(ks)
	return ks
}

func declNames(d ast.Decl) []string {
	switch d := d.(type) {
	case *ast.FuncDecl:
		if d.Recv != nil && len(d.Recv.List) > 0 {
			return []string{recvName(d.Recv.List[0].Type) + "." + d.Name.Name}
		}
		return []string{d.Name.Name}
	case *ast.GenDecl:
		var out []string
		for _, s := range d.Specs {
			switch s := s.(type) {
			case *ast.TypeSpec:
				out = append(out, s.Name.Name)
			case *ast.ValueSpec:
				for _, n := range s.Names {
					if n.Name != "_" {
						out = append(out, n.Name)
					}
				}
			}
		}
		return out
	}
	return nil
}

func recvName(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.StarExpr:
		return recvName(t.X)
	case *ast.IndexExpr:
		return recvName(t.X)
	case *ast.IndexListExpr:
		return recvName(t.X)
	case *ast.Ident:
		return t.Name
	}
	return "?"
}

func bareName(n string) string {
	if _, after, ok := strings.Cut(n, "."); ok {
		return after
	}
	return n
}

// callers finds call sites of names in f, at most maxCallersPerSym per symbol
// counting those already found.
func callers(fset *token.FileSet, f *ast.File, p string, names []string, have []Caller) []Caller {
	count := map[string]int{}
	for _, c := range have {
		count[c.Symbol]++
	}
	var out []Caller
	for _, decl := range f.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}
		in := declNames(fd)[0]
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			name := ""
			switch fn := call.Fun.(type) {
			case *ast.Ident:
				name = fn.Name
			case *ast.SelectorExpr:
				name = fn.Sel.Name
			}
			if name == "" || !slices.Contains(names, name) || bareName(in) == name || count[name] >= maxCallersPerSym {
				return true
			}
			count[name]++
			out = append(out, Caller{Symbol: name, File: p, Line: fset.Position(call.Pos()).Line, In: in})
			return true
		})
	}
	return out
}

// relatedTests are the test functions of f that mention one of names.
func relatedTests(fset *token.FileSet, f *ast.File, src []byte, p string, names []string) []Test {
	var out []Test
	for _, decl := range f.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Body == nil || fd.Recv != nil {
			continue
		}
		n := fd.Name.Name
		if !strings.HasPrefix(n, "Test") && !strings.HasPrefix(n, "Fuzz") && !strings.HasPrefix(n, "Benchmark") && !strings.HasPrefix(n, "Example") {
			continue
		}
		hit := false
		ast.Inspect(fd.Body, func(x ast.Node) bool {
			if id, ok := x.(*ast.Ident); ok && slices.Contains(names, id.Name) {
				hit = true
			}
			return !hit
		})
		if hit {
			start, end := fset.Position(fd.Pos()).Line, fset.Position(fd.End()).Line
			out = append(out, Test{Name: n, File: p, Line: start, Source: cut(lines(src, start, end), maxTestSource)})
		}
	}
	return out
}

func lines(b []byte, from, to int) string {
	ls := strings.Split(string(b), "\n")
	if from < 1 || from > len(ls) {
		return ""
	}
	return strings.Join(ls[from-1:min(to, len(ls))], "\n")
}

func firstLines(b []byte, n int) []byte {
	ls := strings.SplitN(string(b), "\n", n+1)
	return []byte(strings.Join(ls[:min(n, len(ls))], "\n"))
}

func cut(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + fmt.Sprintf("\n… (%d more bytes)", len(s)-n)
}

// Risk patterns over the added lines: what a reviewer must check there. They
// direct attention; they are never findings by themselves.
var riskPatterns = []struct {
	re   *regexp.Regexp
	risk string
}{
	{regexp.MustCompile(`\bgo func\b|\bgo [A-Za-z_.]+\(|\bchan\b|sync\.|atomic\.|Mutex`), "concurrency: shared state, locking, goroutine lifetimes, and channel closes"},
	{regexp.MustCompile(`exec\.Command|os/exec|\bsql\.|\.Query(Row|Context)?\(|\.Exec\(|FormValue|URL\.Query|template\.HTML|filepath\.Join\(.*(req|input|name)`), "external input: injection, path traversal, and validation at the boundary"},
	{regexp.MustCompile(`(?i)token|secret|password|api[_-]?key|credential`), "secrets: never logged, printed, committed, or sent to a model"},
	{regexp.MustCompile(`os\.Remove|RemoveAll|Truncate|(?i)\bdelete\b|\bdrop\b`), "destructive operations: what is removed, and whether it can be undone"},
	{regexp.MustCompile(`(^|\s)_\s*(,\s*_\s*)?=\s*[A-Za-z]`), "ignored errors: an error dropped with _ must be harmless"},
	{regexp.MustCompile(`\[\s*[a-zA-Z_]+\s*(\+|-)\s*1\s*\]|<=\s*len\(|len\([^)]*\)\s*-\s*1`), "boundaries: off-by-one at the ends of ranges and slices"},
}

var layerRisks = map[string]string{
	"L02": "API boundary (L02): input validation, error mapping, and exit codes",
	"L04": "policy (L04): a weakened check, floor, or threshold is a P1",
	"L06": "contracts (L06): callers and the wire format stay compatible",
	"L07": "state (L07): data already written stays readable; the change can be rolled back",
	"L09": "platform (L09): pinned versions, permissions, and secrets",
}

func risks(a *Area, added []string) []string {
	out := []string{}
	for _, l := range a.Layers {
		if r, ok := layerRisks[l]; ok {
			out = append(out, r)
		}
	}
	text := strings.Join(added, "\n")
	for _, p := range riskPatterns {
		if p.re.MatchString(text) {
			out = append(out, p.risk)
		}
	}
	code := slices.ContainsFunc(a.Files, func(p string) bool { return !isTestPath(p) && !isDoc(p) })
	if code && !a.TestsChanged {
		out = append(out, "no test changed in this area: check that the evidence covers the change")
	}
	return out
}

func isDoc(p string) bool {
	switch path.Ext(p) {
	case ".md", ".txt", ".rst":
		return true
	}
	return false
}
