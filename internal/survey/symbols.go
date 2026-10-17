package survey

import (
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"slices"
	"strings"
)

// Symbol is a definition the scanner found: a function, method, or type, with
// the lines it spans.
type Symbol struct {
	ID   string `json:"id"`
	Kind string `json:"kind"` // function, method, class, struct, union, enum, interface, trait, namespace, impl, module, type
	Name string `json:"name"` // qualified where the source qualifies it: CScriptLex::match, Parsedown::text
	At   string `json:"at"`   // path:line of the definition
	End  int    `json:"end"`  // last line of the body
	// Public is set for a type or function a library exposes: a PHP method
	// without private or protected, a class in a header, an exported Go name.
	Public bool `json:"public,omitempty"`
	file   string
	line   int
}

// call is one call site inside a symbol's body, by the called name.
type call struct {
	from int    // index of the calling symbol in its file
	name string // called name, last component
	line int
}

var (
	typeHeader = regexp.MustCompile(`\b(class|struct|union|interface|trait|enum|namespace|impl|object|record)\b\s+(?:class\s+|struct\s+)?([A-Za-z_][\w]*(?:::[A-Za-z_][\w]*)*)`)
	funcName   = regexp.MustCompile(`(operator\s*(?:\(\)|[^\s(\w]+)|~?[A-Za-z_$][\w$]*(?:(?:::|\.)~?[A-Za-z_$][\w$]*)*)\s*(?:<[^(){};]*>)?\s*\(`)
	callName   = regexp.MustCompile(`([A-Za-z_$][\w$]*)\s*\(`)
	cKeywords  = map[string]bool{"if": true, "for": true, "while": true, "switch": true, "catch": true, "return": true, "sizeof": true, "foreach": true,
		"elseif": true, "else": true, "do": true, "try": true, "function": true, "fn": true, "func": true, "fun": true, "new": true, "delete": true, "throw": true,
		"typeof": true, "instanceof": true, "using": true, "lock": true, "synchronized": true, "defined": true, "decltype": true, "alignof": true, "static_assert": true,
		"assert": true, "array": true, "list": true, "isset": true, "unset": true, "empty": true, "echo": true, "print": true, "match": false, "with": true, "when": true,
		"loop": true, "unsafe": true, "async": true, "await": true, "yield": true, "case": true, "default": true, "and": true, "or": true, "not": true, "in": true, "is": true,
		"exit": true, "die": true, "include": true, "require": true, "include_once": true, "require_once": true, "static_cast": true, "dynamic_cast": true,
		"reinterpret_cast": true, "const_cast": true, "noexcept": true, "requires": true, "where": true, "super": true, "this": true, "self": true}
	pyDef     = regexp.MustCompile(`^(\s*)(?:async\s+)?(def|class)\s+([A-Za-z_]\w*)`)
	rubyDef   = regexp.MustCompile(`^(\s*)(def|class|module)\s+(?:self\.)?([A-Za-z_][\w:.?!]*)`)
	shellDef  = regexp.MustCompile(`^\s*(?:function\s+)?([A-Za-z_][\w:-]*)\s*\(\s*\)\s*\{?|^\s*function\s+([A-Za-z_][\w:-]*)\s*\{?`)
	perlDef   = regexp.MustCompile(`^\s*sub\s+([A-Za-z_][\w:]*)`)
	protected = regexp.MustCompile(`\b(private|protected)\b`)
	staticKw  = regexp.MustCompile(`\bstatic\b`)
)

// scope is an open brace while scanning a C-family file.
type scope struct {
	kind string // the symbol kind, "block" for any other brace
	name string
	sym  int // index into the symbols of this file, -1 for a block
}

// braceSymbols finds definitions in a C-family file from its stripped code: at
// each opening brace it reads the text since the previous ; { or } (the
// header) and decides whether the brace opens a type, a function, or a block.
// Calls are the names followed by ( inside each function's body.
func braceSymbols(file, code, lang string) ([]Symbol, []call) {
	var syms []Symbol
	var calls []call
	var stack []scope
	line, hdrStart, hdrLine := 1, 0, 1
	parens := 0
	inFunc := func() int {
		for k := len(stack) - 1; k >= 0; k-- {
			if stack[k].kind == "function" || stack[k].kind == "method" {
				return stack[k].sym
			}
		}
		return -1
	}
	owner := func() string {
		var names []string
		for _, s := range stack {
			if s.sym >= 0 && s.kind != "function" && s.kind != "method" && s.name != "" {
				names = append(names, s.name)
			}
		}
		return strings.Join(names, "::")
	}
	for i := 0; i < len(code); i++ {
		c := code[i]
		switch c {
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
				hdrStart, hdrLine = i+1, line
			}
		case '{':
			header := code[hdrStart:i]
			kind, name, public := classifyHeader(header, lang, parens, inFunc() >= 0)
			startLine := hdrLine + leadingNewlines(header)
			if name != "" {
				// The definition's line is its name's line, not the header's first
				// (a return type or a file header can sit lines above it).
				if loc := regexp.MustCompile(`(^|[^\w$])` + regexp.QuoteMeta(lastName(name)) + `\b`).FindStringIndex(header); loc != nil {
					startLine = hdrLine + strings.Count(header[:loc[0]], "\n")
					if header[loc[0]] == '\n' {
						startLine++
					}
				}
			}
			sc := scope{kind: kind, name: name, sym: -1}
			if kind != "block" {
				q := name
				if o := owner(); o != "" && !strings.Contains(name, "::") && (kind == "function" || kind == "method" || kind != "namespace") {
					q = o + "::" + name
					if kind == "function" {
						kind = "method"
						sc.kind = kind
					}
				}
				if kind == "function" && strings.Contains(name, "::") {
					kind, sc.kind = "method", "method"
				}
				if lang == "php" && kind == "method" {
					public = !protected.MatchString(header)
				}
				syms = append(syms, Symbol{Kind: kind, Name: q, At: file + ":" + itoa(startLine), file: file, line: startLine, Public: public})
				sc.sym = len(syms) - 1
			}
			stack = append(stack, sc)
			hdrStart, hdrLine, parens = i+1, line, 0
		case '}':
			if n := len(stack); n > 0 {
				top := stack[n-1]
				if top.sym >= 0 {
					syms[top.sym].End = line
				}
				stack = stack[:n-1]
			}
			hdrStart, hdrLine, parens = i+1, line, 0
		}
	}
	// Calls: names followed by ( inside each function's body, outside nested
	// definitions' own headers.
	lines := strings.Split(code, "\n")
	for k, s := range syms {
		if s.Kind != "function" && s.Kind != "method" || s.End < s.line {
			continue
		}
		own := lastName(s.Name)
		seen := map[string]bool{}
		for ln := s.line; ln <= s.End && ln <= len(lines); ln++ {
			text := lines[ln-1]
			if ln == s.line {
				// Skip the definition's own name on its header line.
				if j := strings.Index(text, own); j >= 0 {
					text = text[j+len(own):]
				}
			}
			for _, m := range callName.FindAllStringSubmatch(text, -1) {
				n := m[1]
				if cKeywords[n] || seen[n] {
					continue
				}
				seen[n] = true
				calls = append(calls, call{from: k, name: n, line: ln})
			}
		}
	}
	return syms, calls
}

func leadingNewlines(s string) int {
	n := 0
	for _, c := range s {
		switch c {
		case '\n':
			n++
		case ' ', '\t', '\r':
		default:
			return n
		}
	}
	return n
}

// classifyHeader decides what a brace opens from the text before it.
func classifyHeader(header, lang string, parens int, insideFunc bool) (kind, name string, public bool) {
	h := strings.Join(strings.Fields(header), " ")
	if h == "" || parens != 0 {
		return "block", "", false
	}
	// A type: class Foo, struct Foo : Base, namespace foo, impl Foo for Bar.
	if m := typeHeader.FindStringSubmatchIndex(h); m != nil && !strings.Contains(h[:m[0]], "(") && !strings.HasSuffix(strings.TrimSpace(h), ")") {
		kw, nm := h[m[2]:m[3]], h[m[4]:m[5]]
		if kw == "impl" {
			if j := strings.Index(h, " for "); j >= 0 {
				nm = strings.Fields(h[j+5:])[0]
			}
			nm = strings.TrimRight(strings.Split(nm, "<")[0], " ")
		}
		if kw == "record" || kw == "object" {
			kw = "class"
		}
		return kw, nm, !insideFunc
	}
	if strings.HasPrefix(h, "=") || strings.HasSuffix(h, "=") || strings.HasSuffix(h, "=>") || strings.HasSuffix(h, ",") || strings.HasSuffix(h, "(") {
		return "block", "", false
	}
	explicit := explicitDef.MatchString(h)
	if insideFunc && !explicit {
		return "block", "", false
	}
	// A function: the first name followed by ( that is not a keyword, with the
	// parameters closed before the brace.
	for _, m := range funcName.FindAllStringSubmatchIndex(h, -1) {
		nm := h[m[2]:m[3]]
		if cKeywords[lastName(nm)] || cKeywords[nm] {
			continue
		}
		before := strings.TrimSpace(h[:m[2]])
		if strings.HasSuffix(before, ".") || strings.HasSuffix(before, "->") || strings.HasSuffix(before, "new") || strings.HasSuffix(before, "return") {
			return "block", "", false // a call or an object literal, not a definition
		}
		if !strings.Contains(h[m[3]:], ")") {
			return "block", "", false
		}
		pub := !insideFunc && !staticKw.MatchString(before) && !protected.MatchString(before)
		if lang == "go" {
			pub = nm != "" && nm[0] >= 'A' && nm[0] <= 'Z'
		}
		return "function", strings.ReplaceAll(nm, ".", "::"), pub
	}
	return "block", "", false
}

func lastName(q string) string {
	if j := strings.LastIndex(q, "::"); j >= 0 {
		return q[j+2:]
	}
	return q
}

// lineSymbols finds definitions in an indentation or keyword language
// (Python, Ruby, shell, Perl): a definition ends where the next one at the
// same or a lower indent starts, or at the end of the file.
func lineSymbols(file, code, lang string) ([]Symbol, []call) {
	lines := strings.Split(code, "\n")
	type open struct {
		indent int
		sym    int
	}
	var syms []Symbol
	var stack []open
	closeTo := func(indent, ln int) {
		for len(stack) > 0 && stack[len(stack)-1].indent >= indent {
			s := &syms[stack[len(stack)-1].sym]
			s.End = lastCode(lines, s.line, ln-1)
			stack = stack[:len(stack)-1]
		}
	}
	for k, text := range lines {
		ln := k + 1
		var kind, name string
		indent := len(text) - len(strings.TrimLeft(text, " \t"))
		switch lang {
		case "python":
			if m := pyDef.FindStringSubmatch(text); m != nil {
				kind, name = map[string]string{"def": "function", "class": "class"}[m[2]], m[3]
			}
		case "ruby":
			if m := rubyDef.FindStringSubmatch(text); m != nil {
				kind, name = map[string]string{"def": "function", "class": "class", "module": "module"}[m[2]], m[3]
			}
		case "perl":
			if m := perlDef.FindStringSubmatch(text); m != nil {
				kind, name, indent = "function", m[1], 0
			}
		case "shell":
			if m := shellDef.FindStringSubmatch(text); m != nil && !cKeywords[m[1]] {
				kind, name, indent = "function", m[1]+m[2], 0
			}
		}
		if strings.TrimSpace(text) != "" && lang == "python" {
			closeTo(indent, ln)
		}
		if kind == "" {
			continue
		}
		if lang != "python" {
			closeTo(indent, ln)
		}
		q := name
		if len(stack) > 0 {
			parent := syms[stack[len(stack)-1].sym]
			q = parent.Name + "::" + name
			if kind == "function" && parent.Kind != "function" {
				kind = "method"
			}
		}
		pub := !strings.HasPrefix(name, "_")
		syms = append(syms, Symbol{Kind: kind, Name: q, At: file + ":" + itoa(ln), file: file, line: ln, Public: pub && len(stack) <= 1})
		stack = append(stack, open{indent, len(syms) - 1})
	}
	closeTo(-1, len(lines)+1)
	var calls []call
	for k, s := range syms {
		if s.Kind != "function" && s.Kind != "method" {
			continue
		}
		seen := map[string]bool{lastName(s.Name): true}
		for ln := s.line + 1; ln <= s.End && ln <= len(lines); ln++ {
			for _, m := range callName.FindAllStringSubmatch(lines[ln-1], -1) {
				if n := m[1]; !cKeywords[n] && !seen[n] {
					seen[n] = true
					calls = append(calls, call{from: k, name: n, line: ln})
				}
			}
		}
	}
	return syms, calls
}

// lastCode is the last non-blank line in [from, to].
func lastCode(lines []string, from, to int) int {
	for ln := min(to, len(lines)); ln > from; ln-- {
		if strings.TrimSpace(lines[ln-1]) != "" {
			return ln
		}
	}
	return from
}

// goSymbols parses a Go file with the standard library parser: functions,
// methods (Receiver::Name), and types, with the calls inside each function.
func goSymbols(file string, src []byte) ([]Symbol, []call, []string) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, src, parser.SkipObjectResolution)
	if err != nil {
		return nil, nil, nil
	}
	var syms []Symbol
	var calls []call
	var imports []string
	for _, im := range f.Imports {
		imports = append(imports, strings.Trim(im.Path.Value, `"`))
	}
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			name, kind := d.Name.Name, "function"
			if d.Recv != nil && len(d.Recv.List) > 0 {
				kind = "method"
				name = recvName(d.Recv.List[0].Type) + "::" + name
			}
			start, end := fset.Position(d.Pos()).Line, fset.Position(d.End()).Line
			syms = append(syms, Symbol{Kind: kind, Name: name, At: file + ":" + itoa(start), End: end, file: file, line: start, Public: ast.IsExported(d.Name.Name)})
			k := len(syms) - 1
			seen := map[string]bool{}
			if d.Body != nil {
				ast.Inspect(d.Body, func(n ast.Node) bool {
					ce, ok := n.(*ast.CallExpr)
					if !ok {
						return true
					}
					var nm string
					switch fn := ce.Fun.(type) {
					case *ast.Ident:
						nm = fn.Name
					case *ast.SelectorExpr:
						nm = fn.Sel.Name
					}
					if nm != "" && !seen[nm] {
						seen[nm] = true
						calls = append(calls, call{from: k, name: nm, line: fset.Position(ce.Pos()).Line})
					}
					return true
				})
			}
		case *ast.GenDecl:
			for _, s := range d.Specs {
				ts, ok := s.(*ast.TypeSpec)
				if !ok {
					continue
				}
				kind := "type"
				switch ts.Type.(type) {
				case *ast.StructType:
					kind = "struct"
				case *ast.InterfaceType:
					kind = "interface"
				}
				start, end := fset.Position(ts.Pos()).Line, fset.Position(ts.End()).Line
				syms = append(syms, Symbol{Kind: kind, Name: ts.Name.Name, At: file + ":" + itoa(start), End: end, file: file, line: start, Public: ast.IsExported(ts.Name.Name)})
			}
		}
	}
	return syms, calls, imports
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

// uniqueIDs gives every symbol of a file a stable ID; an overload or a second
// definition of the same name gets ~2, ~3, … in source order.
func uniqueIDs(syms []Symbol) {
	seen := map[string]int{}
	for k := range syms {
		base := "sym:" + syms[k].file + "#" + syms[k].Name
		seen[base]++
		if n := seen[base]; n > 1 {
			syms[k].ID = base + "~" + itoa(n)
		} else {
			syms[k].ID = base
		}
	}
}

// isDefKind reports whether kind can be called.
func isDefKind(kind string) bool { return slices.Contains([]string{"function", "method"}, kind) }

// explicitDef matches a definition keyword in a header.
var explicitDef = regexp.MustCompile(`\b(function|fn|func|fun|def|sub)\b`)
