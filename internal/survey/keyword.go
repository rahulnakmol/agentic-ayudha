package survey

import (
	"path"
	"regexp"
	"strings"
)

// Keyword languages: Fortran, COBOL, Pascal (and Delphi), and Visual Basic
// (VB6, VBA, VBScript, VB.NET). They are case-insensitive, so their symbol
// names are kept in lower case and calls resolve case-insensitively. Each is
// read line by line from code whose comments and string contents are blanked
// (line numbers hold), as the brace and indentation scanners do.

// isKeywordSyntax reports whether a syntax family is read by this file.
func isKeywordSyntax(s string) bool {
	return s == syntaxFortran || s == syntaxCOBOL || s == syntaxPascal || s == syntaxVB
}

// fixedForm reports whether a Fortran file is in fixed (card) form: the
// FORTRAN 77 extensions, unless the file says otherwise.
// mainProgramFile reports whether a Fortran file may hold a main program:
// a source file, not an include file (.inc, .h, .fi, .cmn).
func mainProgramFile(file string) bool {
	switch strings.ToLower(path.Ext(file)) {
	case ".f", ".for", ".ftn", ".f77", ".fpp", ".f90", ".f95", ".f03", ".f08":
		return true
	}
	return false
}

func fixedForm(file string) bool {
	switch strings.ToLower(path.Ext(file)) {
	case ".f", ".for", ".ftn", ".f77", ".fpp":
		return true
	}
	return false
}

// stripKeyword blanks comments and string contents of a keyword-language file
// and returns the comments it removed.
func stripKeyword(file, src, syntax string) (string, []comment) {
	if syntax == syntaxPascal {
		return stripPascal(src)
	}
	lines := strings.Split(src, "\n")
	out := make([]string, len(lines))
	var cs []comment
	fixed := syntax == syntaxFortran && fixedForm(file)
	cobolFixed := syntax == syntaxCOBOL && isCOBOLFixed(src)
	for k, l := range lines {
		ln := k + 1
		b := []byte(strings.TrimRight(l, "\r"))
		lineComment := func(from int) {
			cs = append(cs, comment{line: ln, lines: []string{string(b[from:])}})
			for i := from; i < len(b); i++ {
				b[i] = ' '
			}
		}
		switch {
		case fixed:
			if len(b) > 72 {
				b = b[:72] // columns 73-80 are a card's sequence number
			}
			if len(b) > 0 && strings.ContainsRune("CcDd*!", rune(b[0])) {
				lineComment(0)
			}
		case cobolFixed:
			if len(b) > 72 {
				b = b[:72]
			}
			for i := 0; i < len(b) && i < 6; i++ {
				b[i] = ' '
			}
			if len(b) > 6 && strings.ContainsRune("*/Dd", rune(b[6])) {
				lineComment(6)
			}
		}
		var quote byte
		for i := 0; i < len(b); i++ {
			c := b[i]
			if quote != 0 {
				if c == quote {
					quote = 0
				} else {
					b[i] = ' '
				}
				continue
			}
			switch {
			case c == '\'' && syntax == syntaxVB:
				lineComment(i)
			case c == '"' || c == '\'' && syntax != syntaxVB:
				quote = c
			case c == '!' && syntax == syntaxFortran:
				lineComment(i)
			case c == '*' && i+1 < len(b) && b[i+1] == '>' && syntax == syntaxCOBOL:
				lineComment(i)
			case syntax == syntaxVB && (c == 'R' || c == 'r') && strings.TrimSpace(string(b[:i])) == "" && len(b) >= i+3 &&
				strings.EqualFold(string(b[i:i+3]), "rem") && (len(b) == i+3 || b[i+3] == ' '):
				lineComment(i)
			}
		}
		out[k] = string(b)
	}
	return strings.Join(out, "\n"), cs
}

// stripPascal blanks Pascal's { } and (* *) block comments and // line
// comments, keeping newlines, and the contents of '…' strings. A compiler
// directive ({$R-}) is a comment here; the trap rules read it from the source.
func stripPascal(src string) (string, []comment) {
	b := []byte(src)
	var cs []comment
	line := 1
	blank := func(from, to int) {
		for i := from; i < to && i < len(b); i++ {
			if b[i] == '\n' {
				line++
			} else {
				b[i] = ' '
			}
		}
	}
	for i := 0; i < len(b); i++ {
		c := b[i]
		var end string
		switch {
		case c == '\n':
			line++
			continue
		case c == '\'':
			j := i + 1
			for j < len(b) && b[j] != '\'' && b[j] != '\n' {
				j++
			}
			blank(i+1, j)
			i = j
			if i < len(b) && b[i] == '\n' {
				line++
			}
			continue
		case c == '/' && i+1 < len(b) && b[i+1] == '/':
			end = "\n"
		case c == '{':
			end = "}"
		case c == '(' && i+1 < len(b) && b[i+1] == '*':
			end = "*)"
		default:
			continue
		}
		j := strings.Index(src[i+1:], end)
		stop := len(b)
		if j >= 0 {
			stop = i + 1 + j + len(end)
			if end == "\n" {
				stop--
			}
		}
		cs = append(cs, comment{line: line, lines: strings.Split(src[i:stop], "\n")})
		blank(i, stop)
		i = stop - 1
	}
	return string(b), cs
}

// isCOBOLFixed reports whether a COBOL file uses the fixed reference format:
// not declared free, and some line has a sequence area of digits or spaces
// followed by an indicator column.
func isCOBOLFixed(src string) bool {
	if strings.Contains(strings.ToUpper(src), ">>SOURCE FORMAT FREE") {
		return false
	}
	for _, l := range strings.Split(src, "\n") {
		if len(l) > 7 && (isDigits(l[:6]) || strings.TrimSpace(l[:6]) == "" && strings.ContainsRune("*/- ", rune(l[6]))) && strings.TrimSpace(l) != "" {
			return true
		}
	}
	return false
}

func isDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return s != ""
}

var (
	fUnit = regexp.MustCompile(`^\s*(?:(?:pure|elemental|impure|recursive|non_recursive|module)\s+)*` +
		`(?:(?:integer|real|double\s*precision|double\s*complex|complex|logical|character|type\s*\(\s*\w+\s*\))(?:\s*\*\s*(?:\d+|\(\s*[\w*:]+\s*\))|\s*\([^)]*\))?\s+)?` +
		`(?:(?:pure|elemental|recursive)\s+)*(program|module|submodule\s*\([^)]*\)|subroutine|function)\s+(\w+)`)
	fBlockData = regexp.MustCompile(`^\s*block\s*data\b\s*(\w*)`)
	fEnd       = regexp.MustCompile(`^\s*end\s*(?:(program|module|submodule|subroutine|function|block\s*data)\b(?:\s+\w+)?)?\s*$`)
	fType      = regexp.MustCompile(`^\s*type\s*(?:,[^:]*)?(?:::)?\s*([a-z_]\w*)\s*$`)
	fEndType   = regexp.MustCompile(`^\s*end\s*type\b`)
	fInterface = regexp.MustCompile(`^\s*(?:abstract\s+)?interface\b`)
	fEndIface  = regexp.MustCompile(`^\s*end\s*interface\b`)
	fCall      = regexp.MustCompile(`\bcall\s+(\w+)`)
	fUse       = regexp.MustCompile(`(?m)^\s*use\b(?:\s*,\s*(?:non_)?intrinsic\s*::)?\s*(?:::)?\s*(\w+)`)
	fInclude   = regexp.MustCompile(`(?im)^\s*(?:include|#\s*include)\s*['"<]([^'">]+)['">]`)
	fImplNone  = regexp.MustCompile(`^\s*implicit\s+none\b`)

	cobProgram   = regexp.MustCompile(`^\s*program-id\s*\.?\s*([\w-]+)`)
	cobProcDiv   = regexp.MustCompile(`^\s*procedure\s+division\b`)
	cobSection   = regexp.MustCompile(`^\s{0,4}([\w-]+)\s+section\s*\.\s*$`)
	cobParagraph = regexp.MustCompile(`^\s{0,4}([\w-]+)\s*\.\s*$`)
	cobEndProg   = regexp.MustCompile(`^\s*end\s+program\b`)
	cobPerform   = regexp.MustCompile(`\b(?:perform|go\s+to|thru|through)\s+([a-z0-9][\w-]*)`)
	cobCall      = regexp.MustCompile(`(?i)\bcall\s+['"]([\w-]+)['"]`)
	cobCopy      = regexp.MustCompile(`(?i)\bcopy\s+['"]?([\w-]+)`)
	cobNotTarget = map[string]bool{"until": true, "varying": true, "with": true, "test": true, "forever": true, "times": true}

	pasDef       = regexp.MustCompile(`^\s*(?:class\s+)?(procedure|function|constructor|destructor|operator)\s+([a-z_][\w.]*)`)
	pasHead      = regexp.MustCompile(`^\s*(program|unit|library|package)\s+([a-z_][\w.]*)`)
	pasImpl      = regexp.MustCompile(`^\s*implementation\b`)
	pasForward   = regexp.MustCompile(`\b(forward|external)\s*;`)
	pasUses      = regexp.MustCompile(`(?is)\buses\b([^;]*);`)
	pasInclude   = regexp.MustCompile(`(?i)\{\$(?:i|include)\s+([^}\s]+)\s*\}`)
	vbDef        = regexp.MustCompile(`^\s*(?:(?:public|private|friend|static|shared|overrides|overridable|overloads|mustoverride|notoverridable|protected|async|iterator|default|partial)\s+)*(sub|function|property\s+(?:get|let|set)|property)\s+([a-z_]\w*)`)
	vbEnd        = regexp.MustCompile(`^\s*end\s+(sub|function|property)\b`)
	vbType       = regexp.MustCompile(`^\s*(?:(?:public|private|friend|partial|mustinherit|notinheritable|protected)\s+)*(class|module|structure|interface)\s+([a-z_]\w*)`)
	vbEndType    = regexp.MustCompile(`^\s*end\s+(class|module|structure|interface)\b`)
	vbName       = regexp.MustCompile(`(?im)^\s*attribute\s+vb_name\s*=\s*"([^"]+)"`)
	vbImports    = regexp.MustCompile(`(?im)^\s*imports\s+([\w.]+)`)
	vbDeclare    = regexp.MustCompile(`^\s*(?:(?:public|private)\s+)?declare\s+`)
	identRe      = regexp.MustCompile(`[a-z_][\w]*`)
	keywordWords = map[string]bool{"if": true, "then": true, "else": true, "end": true, "do": true, "while": true, "call": true, "return": true, "and": true, "or": true, "not": true}
)

// keywordSymbols reads the definitions and call sites of a keyword-language
// file from its stripped code (lower-cased here). raw is the original text, for
// what the stripping blanks (a COBOL CALL's program name, a VB6 VB_Name).
func keywordSymbols(file, code, raw, syntax string) ([]Symbol, []call, []Fact) {
	lc := strings.ToLower(code)
	lines := strings.Split(lc, "\n")
	rawLines := strings.Split(raw, "\n")
	switch syntax {
	case syntaxFortran:
		return fortranSymbols(file, lines)
	case syntaxCOBOL:
		s, c := cobolSymbols(file, lines, rawLines)
		return s, c, nil
	case syntaxPascal:
		s, c := pascalSymbols(file, lines)
		return s, c, nil
	}
	s, c := vbSymbols(file, lines, raw)
	return s, c, nil
}

// fortranSymbols reads program units (program, module, submodule,
// subroutine, function, block data) and derived types, nesting module
// procedures under their module. A unit without IMPLICIT NONE is returned as
// a trap fact: its undeclared names are typed by their first letter.
func fortranSymbols(file string, lines []string) ([]Symbol, []call, []Fact) {
	fixed := fixedForm(file)
	var syms []Symbol
	var traps []Fact
	type open struct {
		sym      int
		implicit bool
	}
	var stack []open
	iface := 0
	body := func(l string) string {
		if fixed {
			if j := strings.IndexByte(l, '\t'); j >= 0 && j < 6 && strings.TrimSpace(l[:j]) == "" || j >= 0 && j < 6 && isDigits(strings.TrimSpace(l[:j])) {
				return l[j+1:] // DEC tab form: a tab ends the label field
			}
		}
		if fixed && len(l) > 6 {
			if l[5] != ' ' && l[5] != '0' {
				return "" // a continuation line holds no statement start
			}
			return l[6:]
		}
		if fixed {
			return ""
		}
		return strings.TrimLeft(l, " \t0123456789") // a free-form statement label
	}
	closeUnit := func(ln int) {
		n := len(stack) - 1
		top := stack[n]
		syms[top.sym].End = ln
		s := syms[top.sym]
		if !top.implicit && s.Kind != "type" && s.Kind != "module" {
			traps = append(traps, Fact{ID: "trap:fortran-implicit-typing:" + s.At, Kind: "fortran-implicit-typing", At: s.At,
				Detail: s.Kind + " " + s.Name + " has no IMPLICIT NONE: undeclared names starting I-N are INTEGER, the rest REAL", Count: 1})
		}
		stack = stack[:n]
	}
	for k, l := range lines {
		ln := k + 1
		st := body(l)
		if strings.TrimSpace(st) == "" {
			continue
		}
		if fInterface.MatchString(st) {
			iface++
			continue
		}
		if fEndIface.MatchString(st) {
			iface = max(0, iface-1)
			continue
		}
		if iface > 0 {
			continue // an interface body declares a procedure defined elsewhere
		}
		if fImplNone.MatchString(st) && len(stack) > 0 {
			stack[len(stack)-1].implicit = true
			continue
		}
		if fEndType.MatchString(st) {
			if n := len(stack); n > 0 && syms[stack[n-1].sym].Kind == "type" {
				syms[stack[n-1].sym].End = ln
				stack = stack[:n-1]
			}
			continue
		}
		if fEnd.MatchString(st) {
			if len(stack) > 0 {
				closeUnit(ln)
			}
			continue
		}
		var kind, name string
		if m := fUnit.FindStringSubmatch(st); m != nil && m[2] != "procedure" {
			kind, name = m[1], m[2]
			if strings.HasPrefix(kind, "submodule") {
				kind = "module"
			}
		} else if m := fBlockData.FindStringSubmatch(st); m != nil {
			kind, name = "block-data", m[1]
			if name == "" {
				name = "_blockdata"
			}
		} else if m := fType.FindStringSubmatch(st); m != nil && !strings.HasPrefix(strings.TrimSpace(st), "type(") && m[1] != "is" {
			kind, name = "type", m[1]
		}
		if kind == "" {
			// A statement outside every unit starts a main program with no
			// PROGRAM statement, as FORTRAN 77 drivers are written; an
			// include file's declarations are not one.
			if len(stack) > 0 || !mainProgramFile(file) || fInclude.MatchString(st) {
				continue
			}
			kind, name = "program", strings.ToLower(strings.TrimSuffix(path.Base(file), path.Ext(file)))
		}
		q := name
		inherited := false
		for j := len(stack) - 1; j >= 0; j-- {
			p := syms[stack[j].sym]
			inherited = inherited || stack[j].implicit
			if p.Kind == "module" || p.Kind == "subroutine" || p.Kind == "function" || p.Kind == "program" {
				q = p.Name + "::" + name
				break
			}
		}
		syms = append(syms, Symbol{Kind: kind, Name: q, At: file + ":" + itoa(ln), file: file, line: ln, Public: len(stack) == 0 || syms[stack[0].sym].Kind == "module"})
		stack = append(stack, open{sym: len(syms) - 1, implicit: inherited})
	}
	for len(stack) > 0 {
		closeUnit(len(lines))
	}
	// Each line's calls belong to the innermost unit that contains it, so a
	// module or a program with CONTAINS does not take its procedures' calls.
	owner := make([]int, len(lines)+1)
	for i := range owner {
		owner[i] = -1
	}
	for k, s := range syms {
		if s.Kind == "type" || s.Kind == "block-data" || s.Kind == "module" {
			continue
		}
		for ln := s.line + 1; ln <= s.End && ln <= len(lines); ln++ {
			owner[ln] = k // later (nested) units overwrite their parent
		}
	}
	var calls []call
	for k, s := range syms {
		seen := map[string]bool{lastName(s.Name): true}
		for ln := s.line + 1; ln <= s.End && ln <= len(lines); ln++ {
			if owner[ln] != k {
				continue
			}
			text := lines[ln-1]
			if fixed && len(text) > 6 {
				text = text[6:]
			}
			add := func(n string) {
				if !seen[n] && !keywordWords[n] {
					seen[n] = true
					calls = append(calls, call{from: k, name: n, line: ln})
				}
			}
			for _, m := range fCall.FindAllStringSubmatch(text, -1) {
				add(m[1])
			}
			for _, m := range callName.FindAllStringSubmatch(text, -1) {
				add(m[1])
			}
		}
	}
	return syms, calls, traps
}

// cobolSymbols reads PROGRAM-IDs, and the sections and paragraphs of each
// PROCEDURE DIVISION (each ends where the next begins); calls are PERFORM,
// GO TO, and THRU targets, and CALL 'program'.
func cobolSymbols(file string, lines, raw []string) ([]Symbol, []call) {
	var syms []Symbol
	program, proc := -1, false
	lastPara := -1
	fixed := isCOBOLFixed(strings.Join(raw, "\n"))
	closePara := func(ln int) {
		if lastPara >= 0 {
			syms[lastPara].End = lastCode(lines, syms[lastPara].line, ln-1)
			lastPara = -1
		}
	}
	for k, l := range lines {
		ln := k + 1
		if fixed {
			if len(l) <= 7 {
				continue
			}
			l = l[7:] // fixed form: Area A starts at column 8
		}
		switch {
		case cobProgram.MatchString(l):
			closePara(ln)
			if program >= 0 {
				syms[program].End = ln - 1
			}
			m := cobProgram.FindStringSubmatch(l)
			syms = append(syms, Symbol{Kind: "program", Name: m[1], At: file + ":" + itoa(ln), file: file, line: ln, Public: true})
			program, proc = len(syms)-1, false
		case cobProcDiv.MatchString(l):
			proc = true
		case cobEndProg.MatchString(l):
			closePara(ln)
			if program >= 0 {
				syms[program].End = ln
			}
			proc = false
		case proc && cobSection.MatchString(l):
			closePara(ln)
			m := cobSection.FindStringSubmatch(l)
			syms = append(syms, Symbol{Kind: "section", Name: qualify(syms, program, m[1]), At: file + ":" + itoa(ln), file: file, line: ln})
			lastPara = len(syms) - 1
		case proc && cobParagraph.MatchString(l) && !strings.HasPrefix(strings.TrimSpace(l), "exit") && !strings.HasPrefix(strings.TrimSpace(l), "end-") && !strings.HasPrefix(strings.TrimSpace(l), "stop") && !strings.HasPrefix(strings.TrimSpace(l), "goback") && len(l)-len(strings.TrimLeft(l, " ")) <= 4:
			closePara(ln)
			m := cobParagraph.FindStringSubmatch(l)
			syms = append(syms, Symbol{Kind: "paragraph", Name: qualify(syms, program, m[1]), At: file + ":" + itoa(ln), file: file, line: ln})
			lastPara = len(syms) - 1
		}
	}
	closePara(len(lines) + 1)
	if program >= 0 && syms[program].End == 0 {
		syms[program].End = lastCode(lines, syms[program].line, len(lines))
	}
	var calls []call
	for k, s := range syms {
		seen := map[string]bool{}
		end := s.End
		if s.Kind == "program" && k+1 < len(syms) && syms[k+1].Kind != "program" {
			end = syms[k+1].line - 1 // a program's own statements come before its first paragraph
		}
		for ln := s.line + 1; ln <= end && ln <= len(lines); ln++ {
			for _, m := range cobPerform.FindAllStringSubmatch(lines[ln-1], -1) {
				if n := m[1]; !seen[n] && !cobNotTarget[n] && !isDigits(n) {
					seen[n] = true
					calls = append(calls, call{from: k, name: n, line: ln})
				}
			}
			if ln-1 < len(raw) {
				for _, m := range cobCall.FindAllStringSubmatch(raw[ln-1], -1) {
					if n := strings.ToLower(m[1]); !seen[n] {
						seen[n] = true
						calls = append(calls, call{from: k, name: n, line: ln})
					}
				}
			}
		}
	}
	return syms, calls
}

func qualify(syms []Symbol, parent int, name string) string {
	if parent >= 0 {
		return syms[parent].Name + "::" + name
	}
	return name
}

// pascalSymbols reads program, unit, procedure, function, constructor, and
// destructor definitions in the implementation part; a definition ends where
// the next top-level one starts (nested routines are not separated).
func pascalSymbols(file string, lines []string) ([]Symbol, []call) {
	var syms []Symbol
	impl := true
	for _, l := range lines {
		if pasImpl.MatchString(l) {
			impl = false // a unit: definitions before "implementation" are declarations
			break
		}
	}
	unit := -1
	for k, l := range lines {
		ln := k + 1
		if pasImpl.MatchString(l) {
			impl = true
			continue
		}
		if m := pasHead.FindStringSubmatch(l); m != nil && unit < 0 {
			kind := "module"
			if m[1] == "program" || m[1] == "library" {
				kind = "program"
			}
			syms = append(syms, Symbol{Kind: kind, Name: m[2], At: file + ":" + itoa(ln), file: file, line: ln, Public: true})
			unit = len(syms) - 1
			continue
		}
		m := pasDef.FindStringSubmatch(l)
		if m == nil || !impl || pasForward.MatchString(l) {
			continue
		}
		if n := len(syms); n > 0 && syms[n-1].Kind != "module" && syms[n-1].Kind != "program" {
			syms[n-1].End = lastCode(lines, syms[n-1].line, ln-1)
		}
		name := strings.ReplaceAll(m[2], ".", "::")
		kind := "function"
		if strings.Contains(name, "::") {
			kind = "method"
		}
		syms = append(syms, Symbol{Kind: kind, Name: name, At: file + ":" + itoa(ln), file: file, line: ln, Public: true})
	}
	for k := range syms {
		if syms[k].End == 0 {
			syms[k].End = lastCode(lines, syms[k].line, len(lines))
		}
	}
	return syms, identCalls(syms, lines, identRe)
}

// vbSymbols reads Sub, Function, and Property definitions (closed by End Sub,
// End Function, End Property) and VB.NET classes and modules; a VB6 class or
// form file's routines are qualified by its VB_Name.
func vbSymbols(file string, lines []string, raw string) ([]Symbol, []call) {
	var syms []Symbol
	owner := -1
	if loc := vbName.FindStringSubmatchIndex(raw); loc != nil {
		kind := "module"
		switch strings.ToLower(path.Ext(file)) {
		case ".cls", ".frm", ".ctl":
			kind = "class"
		}
		ln := lineOfIndex(raw, loc[0])
		syms = append(syms, Symbol{Kind: kind, Name: strings.ToLower(raw[loc[2]:loc[3]]), At: file + ":" + itoa(ln), file: file, line: ln, End: len(lines), Public: true})
		owner = 0
	}
	var types []int
	open := -1
	for k, l := range lines {
		ln := k + 1
		if vbDeclare.MatchString(l) {
			continue
		}
		if m := vbType.FindStringSubmatch(l); m != nil {
			kind := m[1]
			if kind == "structure" {
				kind = "struct"
			}
			syms = append(syms, Symbol{Kind: kind, Name: m[2], At: file + ":" + itoa(ln), file: file, line: ln, Public: !strings.Contains(l, "private")})
			types = append(types, len(syms)-1)
			continue
		}
		if vbEndType.MatchString(l) && len(types) > 0 {
			syms[types[len(types)-1]].End = ln
			types = types[:len(types)-1]
			continue
		}
		if vbEnd.MatchString(l) && open >= 0 {
			syms[open].End = ln
			open = -1
			continue
		}
		m := vbDef.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		name := m[2]
		kind := "function"
		parent := owner
		if len(types) > 0 {
			parent = types[len(types)-1]
		}
		if parent >= 0 {
			name, kind = syms[parent].Name+"::"+name, "method"
		}
		syms = append(syms, Symbol{Kind: kind, Name: name, At: file + ":" + itoa(ln), file: file, line: ln, Public: !strings.Contains(l, "private")})
		open = len(syms) - 1
	}
	for k := range syms {
		if syms[k].End == 0 {
			syms[k].End = lastCode(lines, syms[k].line, len(lines))
		}
	}
	return syms, identCalls(syms, lines, identRe)
}

// identCalls lists every identifier in each routine's body as a call
// candidate: Pascal and VB call a routine without parentheses, so any name
// may be a call; the call graph keeps only names that resolve to a definition.
func identCalls(syms []Symbol, lines []string, re *regexp.Regexp) []call {
	var calls []call
	for k, s := range syms {
		if !isDefKind(s.Kind) {
			continue
		}
		seen := map[string]bool{lastName(s.Name): true}
		for ln := s.line + 1; ln <= s.End && ln <= len(lines); ln++ {
			for _, n := range re.FindAllString(lines[ln-1], -1) {
				if !seen[n] && !keywordWords[n] {
					seen[n] = true
					calls = append(calls, call{from: k, name: n, line: ln})
				}
			}
		}
	}
	return calls
}

func lineOfIndex(s string, off int) int { return strings.Count(s[:off], "\n") + 1 }

// keywordImports records Fortran USE and INCLUDE, COBOL COPY, Pascal uses
// and {$I}, and VB.NET Imports.
func (ix *Index) keywordImports(file, syntax, raw string, exists func(string) bool) {
	dir := path.Dir(file)
	lineOf := func(off int) string { return file + ":" + itoa(strings.Count(raw[:off], "\n")+1) }
	edge := func(kind, to, at string) {
		ix.Edges = append(ix.Edges, Edge{Kind: kind, From: "file:" + file, To: []string{to}, At: at})
	}
	include := func(name, at string, exts ...string) {
		for _, e := range append([]string{""}, exts...) {
			for _, c := range []string{path.Join(dir, name+e), name + e} {
				if c = path.Clean(c); exists(c) {
					edge("include", "file:"+c, at)
					return
				}
			}
			if t := ix.suffixFold(name + e); t != "" {
				edge("include", "file:"+t, at)
				return
			}
		}
		edge("include", "ext:"+name, at)
	}
	switch syntax {
	case syntaxFortran:
		code, _ := stripKeyword(file, raw, syntax)
		lc := strings.ToLower(code)
		for _, m := range fUse.FindAllStringSubmatchIndex(lc, -1) {
			name := lc[m[2]:m[3]]
			if t := ix.moduleFile(name); t == file {
				continue // a program using the module defined above it
			} else if t != "" {
				edge("import", "file:"+t, lineOf(m[0]))
			} else if t := ix.suffixFold(name + ".f90"); t != "" {
				edge("import", "file:"+t, lineOf(m[0]))
			} else {
				edge("import", "ext:"+name, lineOf(m[0]))
			}
		}
		for _, m := range fInclude.FindAllStringSubmatchIndex(raw, -1) {
			include(raw[m[2]:m[3]], lineOf(m[0]))
		}
	case syntaxCOBOL:
		code, _ := stripKeyword(file, raw, syntax)
		for _, m := range cobCopy.FindAllStringSubmatchIndex(code, -1) {
			name := code[m[2]:m[3]]
			if strings.EqualFold(name, "replacing") {
				continue
			}
			include(name, lineOf(m[0]), ".cpy", ".cbl", ".cob", ".CPY", ".CBL")
		}
	case syntaxPascal:
		code, _ := stripKeyword(file, raw, syntax)
		for _, m := range pasUses.FindAllStringSubmatchIndex(code, -1) {
			for _, u := range strings.Split(code[m[2]:m[3]], ",") {
				name := strings.TrimSpace(strings.Fields(u + " ")[0])
				if name == "" {
					continue
				}
				if t := ix.suffixFold(name + ".pas"); t != "" {
					edge("import", "file:"+t, lineOf(m[0]))
				} else if t := ix.suffixFold(name + ".pp"); t != "" {
					edge("import", "file:"+t, lineOf(m[0]))
				} else {
					edge("import", "ext:"+strings.ToLower(name), lineOf(m[0]))
				}
			}
		}
		for _, m := range pasInclude.FindAllStringSubmatchIndex(raw, -1) {
			include(strings.Trim(raw[m[2]:m[3]], `'"`), lineOf(m[0]))
		}
	case syntaxVB:
		for _, m := range vbImports.FindAllStringSubmatchIndex(raw, -1) {
			edge("import", "ext:"+raw[m[2]:m[3]], lineOf(m[0]))
		}
	}
}

// suffixFold finds the one file whose path ends with name, ignoring case
// (legacy build systems on case-insensitive file systems mix cases).
func (ix *Index) suffixFold(name string) string {
	ln := strings.ToLower(name)
	found := ""
	for _, f := range ix.Files {
		lp := strings.ToLower(f.Path)
		if lp == ln || strings.HasSuffix(lp, "/"+ln) {
			if found != "" {
				return ""
			}
			found = f.Path
		}
	}
	return found
}

// moduleFile is the file that defines a Fortran module, once symbols are read.
func (ix *Index) moduleFile(name string) string {
	for _, s := range ix.Symbols {
		if s.Kind == "module" && s.Name == name {
			return s.file
		}
	}
	return ""
}
