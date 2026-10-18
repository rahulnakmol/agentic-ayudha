// Package survey indexes a repository that has code but no product
// documentation, deterministically and with no model (WP5.9): files and
// languages, build systems and their targets, symbols, the include and import
// graph, a name-resolved call graph, entry points, I/O surfaces, tests,
// known-issue markers, documents, and git churn, grouped into modules sized
// for a model to read one at a time. Every fact has a stable ID and a
// path:line, so the analysis the host's model writes from it (the survey
// draft) can be checked claim by claim (Decision #287).
package survey

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// Version is the index schema version.
const Version = 3

// DefaultBudget is a module's default size in tokens (about four bytes each).
const DefaultBudget = 20000

// maxFileBytes is the largest file scanned for symbols; a larger one is listed
// with its size and marked not scanned (generated or data files).
const maxFileBytes = 2 << 20

// File is one file of the checkout.
type File struct {
	Path    string `json:"path"`
	Role    string `json:"role"` // source, test, build, doc, vendored, data
	Lang    string `json:"lang,omitempty"`
	Lines   int    `json:"lines"`
	Bytes   int    `json:"bytes"`
	Scanned bool   `json:"scanned,omitempty"` // symbols, calls, and surfaces were extracted
	Commits int    `json:"commits,omitempty"` // commits touching it (git churn)
}

// Fact is an entry point, an I/O surface, a test, or a document.
type Fact struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	At     string `json:"at"`
	Detail string `json:"detail,omitempty"`
	Count  int    `json:"count,omitempty"` // occurrences in the file, for a surface
}

// Edge is an include, an import, or a call. A call is resolved by name only:
// when several definitions share the name, To lists them all and Ambiguous
// is set; the index never picks one.
type Edge struct {
	Kind      string   `json:"kind"` // include, import, call
	From      string   `json:"from"` // file:<path>, a symbol ID, or a script's or Python main guard's entry ID
	To        []string `json:"to"`
	At        string   `json:"at"`
	Ambiguous bool     `json:"ambiguous,omitempty"`
	Name      string   `json:"name,omitempty"` // the called name, when To lists only the first eight definitions
}

// Module is a group of files read together: a directory, split to the token
// budget along file stems (TinyJS.cpp with TinyJS.h).
type Module struct {
	ID      string   `json:"id"`
	Dir     string   `json:"dir"`
	Files   []string `json:"files"`
	Lines   int      `json:"lines"`
	Tokens  int      `json:"tokens"` // estimate, bytes / 4
	Symbols int      `json:"symbols"`
	FanIn   int      `json:"fan_in"`  // edges from other modules into this one
	FanOut  int      `json:"fan_out"` // edges from this module to others
}

// Count is a number of files and lines for a language or extension.
type Count struct {
	Name  string `json:"name"`
	Files int    `json:"files"`
	Lines int    `json:"lines"`
	// CountedOnly is set for a language the index counts but does not read:
	// no symbols, calls, imports, or traps (lang.go lists which).
	CountedOnly bool `json:"counted_only,omitempty"`
}

// Index is the deterministic survey of one checkout: the same commit gives
// the same bytes (no clock, sorted lists).
type Index struct {
	Version int    `json:"version"`
	Commit  string `json:"commit,omitempty"`
	// Digest is the SHA-256 of every indexed file's path and content, and of
	// the manuals': saved survey sections are stale when it changes, at the
	// same commit too (an uncommitted edit, a checkout with no git).
	Digest     string   `json:"digest,omitempty"`
	History    *History `json:"history,omitempty"`
	Languages  []Count  `json:"languages"`
	NotIndexed []Count  `json:"not_indexed,omitempty"` // file extensions with no language the index knows
	Systems    []string `json:"build_systems"`
	Targets    []Target `json:"targets"`
	Deps       []Dep    `json:"deps"`
	Entries    []Fact   `json:"entries"`
	// Builtins are the names source files register for the programs they
	// embed (an interpreter's natives, a shell's commands), and Uses where an
	// input file calls one: a case set's runner must register what its inputs
	// call.
	Builtins []Fact   `json:"builtins,omitempty"`
	Uses     []Fact   `json:"uses,omitempty"`
	Surfaces []Fact   `json:"surfaces"`
	Traps    []Fact   `json:"traps"` // trap rules that match (traps.go): where to check a semantic the port could get wrong
	Tests    []Fact   `json:"tests"`
	Docs     []Fact   `json:"docs"`
	Markers  []Marker `json:"markers"`
	Modules  []Module `json:"modules"`
	Symbols  []Symbol `json:"symbols"`
	Edges    []Edge   `json:"edges"`
	Files    []File   `json:"files"`

	byPath map[string]int
	ids    map[string]bool
}

// History is what git says about the checkout.
type History struct {
	Commits int    `json:"commits"`
	First   string `json:"first,omitempty"` // date of the oldest commit read
	Last    string `json:"last,omitempty"`
	Authors int    `json:"authors"`
	Shallow bool   `json:"shallow,omitempty"`
}

// Options tune Build.
type Options struct {
	Budget int // module size in tokens; 0 is DefaultBudget
	// Git runs git in the checkout; nil runs the git binary. It returns ""
	// when git fails (not a repository), and the index then walks the tree.
	Git func(ctx context.Context, dir string, args ...string) string
	// Manuals are user manuals and notes from outside the checkout, by name,
	// as text: each is indexed as a document (manual:<name>), cited by line
	// like a file, so a claim a manual states is traced to it.
	Manuals map[string]string
	// Out is the survey's output directory relative to the checkout (slash
	// separated), when it is inside it: its files are never indexed, so a
	// re-index of an unchanged checkout gives the same index and digest.
	Out string
}

var (
	cInclude   = regexp.MustCompile(`(?m)^[ \t]*#[ \t]*include[ \t]*([<"])([^>"]+)[>"]`)
	pyImport   = regexp.MustCompile(`(?m)^[ \t]*(?:from[ \t]+(\.*[\w.]*)[ \t]+import|import[ \t]+([\w.]+))`)
	phpInclude = regexp.MustCompile(`(?m)\b(?:require|include)(?:_once)?[ \t]*\(?[ \t]*(?:__DIR__[ \t]*\.[ \t]*)?['"]([^'"]+)['"]`)
	phpUse     = regexp.MustCompile(`(?m)^[ \t]*use[ \t]+\\?([\w\\]+)`)
	jsImport   = regexp.MustCompile(`(?m)(?:^[ \t]*import\b[^'"\n]*['"]([^'"]+)['"]|\brequire\(\s*['"]([^'"]+)['"]\s*\))`)
	javaImport = regexp.MustCompile(`(?m)^[ \t]*import[ \t]+(?:static[ \t]+)?([\w.]+)`)
	rustMod    = regexp.MustCompile(`(?m)^[ \t]*(?:pub(?:\([^)]*\))?[ \t]+)?mod[ \t]+(\w+)[ \t]*;`)
	rustUse    = regexp.MustCompile(`(?m)^[ \t]*(?:pub[ \t]+)?use[ \t]+(\w+)::`)
	rubyReq    = regexp.MustCompile(`(?m)^[ \t]*require(_relative)?[ \t(]+['"]([^'"]+)['"]`)
	csUsing    = regexp.MustCompile(`(?m)^[ \t]*using[ \t]+([\w.]+)[ \t]*;`)
	pyMain     = regexp.MustCompile(`(?m)^if[ \t]+__name__[ \t]*==[ \t]*['"]__main__['"]`)
	shebang    = regexp.MustCompile(`^#!`)
	goMainPkg  = regexp.MustCompile(`(?m)^package[ \t]+main\b`)
)

// surfaceRules find a file's I/O surfaces in its stripped code (string
// contents blanked, so a word in a message never counts).
var surfaceRules = []struct {
	kind string
	re   *regexp.Regexp
}{
	{"cli-args", regexp.MustCompile(`\b(argv|argc|getopt\w*|sys\.argv|argparse|optparse|click\.command|os\.Args|flag\.(?:String|Bool|Int|Parse)|process\.argv|\$argv|ARGV|yargs|commander|clap::|CommandLine)\b`)},
	{"file", regexp.MustCompile(`\b(fopen|fopen_s|freopen|ifstream|ofstream|fstream|file_get_contents|file_put_contents|fwrite|fread|fgets|readFileSync|writeFileSync|readFile|writeFile|os\.Open|os\.Create|os\.ReadFile|os\.WriteFile|FileInputStream|FileOutputStream|FileReader|FileWriter|File\.open|File\.read|Files\.\w+|std::fs::\w+|File::open|File::create)\s*[(<]|\bopen\s*\(`)},
	{"net", regexp.MustCompile(`\b(socket|connect|bind|listen|accept|getaddrinfo|gethostbyname|curl_\w+|fsockopen|stream_socket_\w+|http\.Get|http\.Post|http\.ListenAndServe|net\.Dial|net\.Listen|urlopen|requests\.\w+|fetch|XMLHttpRequest|HttpClient|HttpURLConnection|TcpStream|TcpListener|reqwest::\w+)\s*\(`)},
	{"env", regexp.MustCompile(`\b(getenv|setenv|putenv|os\.Getenv|os\.environ|process\.env|System\.getenv|std::env::var|env::var|\$_ENV|\$_SERVER|ENV\[)`)},
	{"process", regexp.MustCompile(`\b(system|popen|execl|execlp|execle|execv|execvp|execve|fork|CreateProcess\w*|ShellExecute\w*|subprocess\.\w+|os\.system|shell_exec|proc_open|passthru|exec\.Command|Runtime\.getRuntime|ProcessBuilder|Command::new)\s*\(`)},
	{"stdout", regexp.MustCompile(`\b(printf|puts|std::cout|cout|cerr|std::cerr|println!?|print!?|console\.log|fmt\.Print\w*|System\.out\.\w+|echo|sys\.stdout)\b`)},
	{"stdin", regexp.MustCompile(`\b(scanf|getchar|std::cin|cin|fgets\s*\(\s*\w*,\s*\w+,\s*stdin|sys\.stdin|input\s*\(|readline|STDIN|php://stdin|bufio\.NewReader\(os\.Stdin\))`)},
	{"database", regexp.MustCompile(`\b(sqlite3_\w+|mysql_\w+|mysqli\w*|PDO|pg_\w+|psycopg2|sqlalchemy|sql\.Open|DriverManager|JdbcTemplate|SqlConnection|rusqlite)\b`)},
	{"dynamic-dispatch", regexp.MustCompile(`(\$\w+->\{|->\$\w+\s*\(|\bcall_user_func\w*\s*\(|\bgetattr\s*\(|__getattr__|\bmethod_missing\b|\.send\s*\(|\beval\s*\(|\bReflection\w+|\breflect\.\w+|\bdlsym\s*\(|\bGetProcAddress\s*\(|\bClass\.forName\s*\()`)},
	{"time", regexp.MustCompile(`\b(time\s*\(|clock\s*\(|gettimeofday|localtime|strftime|time\.Now|datetime\.now|Date\.now|new Date\(|System\.currentTimeMillis|microtime|date\s*\()`)},
	{"random", regexp.MustCompile(`\b(rand|srand|random|drand48|mt_rand|Math\.random|rand::\w+|random\.\w+|math/rand)\b\s*[(:]`)},
}

// langSurfaceRules replace surfaceRules for the keyword languages, matched
// against lower-cased code.
var langSurfaceRules = map[string][]struct {
	kind string
	re   *regexp.Regexp
}{
	"fortran": {
		{"cli-args", regexp.MustCompile(`\b(?:get_command_argument|get_command|command_argument_count|getarg|iargc)\b`)},
		{"file", regexp.MustCompile(`\b(?:open|inquire)\s*\(`)},
		{"stdin", regexp.MustCompile(`\bread\s*\(\s*(?:\*|5)\s*[,)]|\bread\s+\*`)},
		{"stdout", regexp.MustCompile(`\bwrite\s*\(\s*(?:\*|6)\s*[,)]|\bprint\s+(?:\*|\d)`)},
		{"env", regexp.MustCompile(`\b(?:get_environment_variable|getenv)\b`)},
		{"process", regexp.MustCompile(`\b(?:execute_command_line|system)\s*\(`)},
		{"time", regexp.MustCompile(`\b(?:cpu_time|date_and_time|system_clock|etime|dtime|secnds)\b`)},
		{"random", regexp.MustCompile(`\b(?:random_number|random_seed|rand|ran|irand)\s*\(`)},
		{"dynamic-dispatch", regexp.MustCompile(`(?m)^\s*external\b|\bprocedure\s*\(\s*\w*\s*\)\s*,\s*pointer`)},
		// A unit number held in a variable (data nread /5/): stdin, stdout, a file, or an internal file, decided at run time.
		{"unit-io", regexp.MustCompile(`\b(?:read|write)\s*\(\s*(?:unit\s*=\s*)?[a-z]\w*\s*[,)]`)},
	},
	"cobol": {
		{"file", regexp.MustCompile(`\bselect\b[^.]*\bassign\b|\b(?:open|read|write|rewrite|delete)\s+(?:input|output|i-o|extend)?\s*[a-z]`)},
		{"stdin", regexp.MustCompile(`\baccept\b`)},
		{"stdout", regexp.MustCompile(`\bdisplay\b`)},
		{"database", regexp.MustCompile(`\bexec\s+sql\b`)},
		{"process", regexp.MustCompile(`\bexec\s+cics\b|\bcall\s+["']system["']`)},
		{"time", regexp.MustCompile(`\bcurrent-date\b|\baccept\s+\w+\s+from\s+(?:date|time|day)\b`)},
		{"dynamic-dispatch", regexp.MustCompile(`\bcall\s+[a-z]`)},
	},
	"pascal": {
		{"cli-args", regexp.MustCompile(`\b(?:paramstr|paramcount)\b`)},
		{"file", regexp.MustCompile(`\b(?:assignfile|assign|reset|rewrite|append|blockread|blockwrite|tfilestream)\b\s*[(.]`)},
		{"stdout", regexp.MustCompile(`\bwriteln?\b`)},
		{"stdin", regexp.MustCompile(`\breadln?\b`)},
		{"env", regexp.MustCompile(`\bgetenvironmentvariable\b|\bgetenv\b`)},
		{"process", regexp.MustCompile(`\b(?:shellexecute\w*|createprocess\w*|winexec|tprocess)\b`)},
		{"time", regexp.MustCompile(`\b(?:now|date|time|gettickcount)\b\s*[;)(]`)},
		{"random", regexp.MustCompile(`\b(?:random|randomize)\b`)},
		{"database", regexp.MustCompile(`\bt(?:ado|ib|sql|fd)\w*(?:query|connection|table)\b|\btdatabase\b`)},
		{"ui", regexp.MustCompile(`\btform\d*\b|\b(?:showmessage|messagedlg)\b`)},
	},
	"vb": {
		{"cli-args", regexp.MustCompile(`\bcommand\$?\s*(?:\(|$)|\bwscript\.arguments\b|\bmy\.application\.commandlineargs\b`)},
		{"file", regexp.MustCompile(`\bopen\b[^\n]*\bfor\s+(?:input|output|append|binary|random)\b|\b(?:filesystemobject|kill|filecopy|mkdir)\b`)},
		{"stdout", regexp.MustCompile(`\b(?:debug\.print|wscript\.echo|console\.write\w*|msgbox)\b`)},
		{"env", regexp.MustCompile(`\benviron\$?\s*\(`)},
		{"process", regexp.MustCompile(`\bshell\s*\(|\bcreateobject\s*\(\s*"`)},
		{"time", regexp.MustCompile(`\b(?:now|date|time|timer)\b`)},
		{"random", regexp.MustCompile(`\b(?:rnd|randomize)\b`)},
		{"database", regexp.MustCompile(`\b(?:opendatabase|adodb|dao\.|openrecordset)\b`)},
		{"ui", regexp.MustCompile(`\bbegin\s+vb\.(?:form|menu|commandbutton|textbox)\b|\bmsgbox\b|\.show\b`)},
		{"dynamic-dispatch", regexp.MustCompile(`\bcallbyname\b|\bcreateobject\b|\bgetobject\b`)},
	},
}

// Build indexes the checkout at root.
func Build(ctx context.Context, root string, opt Options) (*Index, error) {
	if opt.Budget <= 0 {
		opt.Budget = DefaultBudget
	}
	if opt.Git == nil {
		opt.Git = runGit
	}
	paths, err := listFiles(ctx, root, opt.Git)
	if err != nil {
		return nil, err
	}
	if opt.Out != "" { // the survey's own output, when it sits in the checkout
		paths = slices.DeleteFunc(paths, func(p string) bool { return p == opt.Out || strings.HasPrefix(p, opt.Out+"/") })
	}
	ix := &Index{Version: Version, byPath: map[string]int{}}
	ix.Commit = strings.TrimSpace(opt.Git(ctx, root, "rev-parse", "HEAD"))
	exists := func(p string) bool { _, ok := ix.byPath[p]; return ok }
	files := map[string]*scanned{}
	present := map[string]bool{}
	for _, p := range paths {
		if l, ok := langs[strings.ToLower(path.Ext(p))]; ok && strings.ToLower(path.Ext(p)) != ".inc" {
			present[l.Name] = true
		}
	}
	cpp := present["cpp"]
	// An .inc file is PHP unless the checkout has no PHP and has Fortran or
	// Pascal, whose include files share the extension.
	incLang, incOK := langs[".inc"], true
	switch {
	case present["php"]:
	case present["fortran"]:
		incLang = langs[".f90"]
	case present["pascal"]:
		incLang = langs[".pas"]
	default:
		incOK = false
	}
	fixLang := func(p string, l Lang) Lang {
		if l.Name == "c" && cpp {
			l.Name = "cpp" // a header in a C++ project is C++
		}
		if strings.ToLower(path.Ext(p)) == ".inc" && l.Name != "" {
			if !incOK {
				return Lang{}
			}
			l = incLang
		}
		return l
	}
	var keywordFiles []string
	for _, p := range paths {
		info, err := os.Lstat(filepath.Join(root, filepath.FromSlash(p)))
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		role, lang, _ := classify(p)
		lang = fixLang(p, lang)
		if lang.Name == "" && role == "source" {
			role = "data"
		}
		f := File{Path: p, Role: role, Lang: lang.Name, Bytes: int(info.Size())}
		ix.byPath[p] = len(ix.Files)
		ix.Files = append(ix.Files, f)
	}
	for k := range ix.Files {
		f := &ix.Files[k]
		if f.Bytes > maxFileBytes {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(f.Path)))
		if err != nil || bytes.IndexByte(raw[:min(len(raw), 8000)], 0) >= 0 {
			continue // unreadable or binary
		}
		f.Lines = bytes.Count(raw, []byte("\n"))
		if len(raw) > 0 && raw[len(raw)-1] != '\n' {
			f.Lines++
		}
		_, lang, system := classify(f.Path)
		lang = fixLang(f.Path, lang)
		if f.Role == "data" && path.Ext(f.Path) == "" {
			// A script without an extension (bin/tool) is read by its #! line.
			if l, ok := shebangLang(raw); ok {
				lang, f.Lang, f.Role = l, l.Name, "source"
				if isTestPath(f.Path) {
					f.Role = "test"
				}
			}
		}
		s := &scanned{raw: string(raw), lang: lang.Name}
		files[f.Path] = s
		if f.Role != "build" && lang.Syntax == syntaxBuild {
			ix.buildTraps(f.Path, s.raw) // an included build file: cmake/UserOverride.cmake, rules.mk
		}
		switch {
		case f.Role == "build":
			code, _ := strip(s.raw, syntaxBuild, lang.Name)
			ts, ds := buildFacts(f.Path, system, code, s.raw, exists)
			ix.Targets = append(ix.Targets, ts...)
			ix.Deps = append(ix.Deps, ds...)
			ix.Systems = append(ix.Systems, system)
			ix.buildTraps(f.Path, s.raw)
		case f.Role == "doc":
			ix.Docs = append(ix.Docs, Fact{ID: "doc:" + f.Path, Kind: "doc", At: f.Path + ":1", Detail: firstHeading(s.raw)})
		case lang.Syntax != syntaxNone && (f.Role == "source" || f.Role == "test" || f.Role == "vendored"):
			var code string
			var cs []comment
			if isKeywordSyntax(lang.Syntax) {
				code, cs = stripKeyword(f.Path, s.raw, lang.Syntax)
			} else {
				code, cs = strip(s.raw, lang.Syntax, lang.Name)
			}
			s.code = code
			ix.Markers = append(ix.Markers, markers(f.Path, cs)...)
			if f.Role == "vendored" || !lang.Symbols {
				break
			}
			f.Scanned = true
			switch {
			case lang.Name == "go":
				var imps []string
				s.syms, s.calls, imps = goSymbols(f.Path, raw)
				for _, im := range imps {
					ix.Edges = append(ix.Edges, Edge{Kind: "import", From: "file:" + f.Path, To: []string{"ext:" + im}, At: f.Path + ":1"})
				}
			case isKeywordSyntax(lang.Syntax):
				var traps []Fact
				s.syms, s.calls, traps = keywordSymbols(f.Path, code, s.raw, lang.Syntax)
				ix.Traps = append(ix.Traps, traps...)
			case lang.Syntax == syntaxHash:
				s.syms, s.calls = lineSymbols(f.Path, code, lang.Name)
			default:
				s.syms, s.calls = braceSymbols(f.Path, code, lang.Name)
				if !isHeader(f.Path) && (lang.Name == "c" || lang.Name == "cpp") {
					for k := range s.syms {
						s.syms[k].Public = false // a .cpp file's definitions implement the headers' API
					}
				}
			}
			uniqueIDs(s.syms)
			ix.Symbols = append(ix.Symbols, s.syms...)
			ix.surfaces(f.Path, lang.Name, code)
			ix.traps(f.Path, lang.Name, code, s.raw)
			ix.entries(f.Path, f.Role, lang.Name, s.raw, s.syms)
			if f.Role == "source" && (lang.Name == "c" || lang.Name == "cpp") && isHeader(f.Path) {
				ix.headerEntries(f.Path, s.code)
			}
		}
		if f.Role == "test" {
			ix.Tests = append(ix.Tests, Fact{ID: "test:" + f.Path, Kind: "test", At: f.Path + ":1"})
		}
		if lang.Name != "" && f.Role != "build" {
			if isKeywordSyntax(lang.Syntax) {
				keywordFiles = append(keywordFiles, f.Path) // after every module's symbols are read
			} else {
				ix.imports(f.Path, lang.Name, s.raw, exists)
			}
		}
	}
	for _, p := range keywordFiles {
		ix.keywordImports(p, langs[strings.ToLower(path.Ext(p))].Syntax, files[p].raw, exists)
	}
	ix.opaqueEntries()
	// The call graph, resolved by the called name's last component.
	byName := map[string][]string{}
	for _, s := range ix.Symbols {
		if isDefKind(s.Kind) {
			n := lastName(s.Name)
			byName[n] = append(byName[n], s.ID)
		}
	}
	for _, p := range sortedKeys(files) {
		s := files[p]
		for _, c := range s.calls {
			to := byName[c.name]
			if len(to) == 0 || c.from >= len(s.syms) {
				continue
			}
			from := s.syms[c.from].ID
			to = without(to, from)
			if len(to) == 0 {
				continue
			}
			ix.Edges = append(ix.Edges, callEdge(from, c.name, to, p+":"+itoa(c.line)))
		}
		ix.guardCalls(p, s, byName)
		ix.scriptCalls(p, s, byName)
	}
	ix.builtins(files, byName)
	for _, name := range sortedKeys(opt.Manuals) {
		text := opt.Manuals[name]
		p := "manual:" + name
		if _, ok := ix.byPath[p]; ok { // a checkout file of that name would shadow the manual's evidence and digest
			return nil, fmt.Errorf("survey: manual %s collides with the checkout file %s", name, p)
		}
		lines := strings.Count(text, "\n")
		if text != "" && !strings.HasSuffix(text, "\n") {
			lines++
		}
		ix.byPath[p] = len(ix.Files)
		ix.Files = append(ix.Files, File{Path: p, Role: "doc", Lines: lines, Bytes: len(text)})
		ix.Docs = append(ix.Docs, Fact{ID: "doc:" + p, Kind: "manual", At: p + ":1", Detail: firstHeading(text)})
	}
	ix.history(ctx, root, opt.Git)
	ix.counts()
	ix.modules(opt.Budget)
	if err := ix.digest(root, files, opt.Manuals); err != nil {
		return nil, err
	}
	ix.finish()
	return ix, nil
}

// guardCalls records the calls a Python program makes at module scope (the
// main guard's block, and the setup before it) as edges from the guard's
// entry: no function holds those calls, so a runner chosen by its guard
// reaches what module scope calls and no other function of its file.
func (ix *Index) guardCalls(p string, s *scanned, byName map[string][]string) {
	k, ok := ix.byPath[p]
	if !ok || ix.Files[k].Lang != "python" {
		return
	}
	m := pyMain.FindStringIndex(s.raw)
	if m == nil {
		return
	}
	guard := strings.Count(s.raw[:m[0]], "\n") + 1
	from := "entry:" + p + ":" + itoa(guard)
	lines := strings.Split(s.code, "\n")
	inDef := func(ln int) bool {
		return slices.ContainsFunc(s.syms, func(d Symbol) bool { return isDefKind(d.Kind) && d.line <= ln && ln <= d.End })
	}
	seen := map[string]bool{}
	other := false // in the guard's else or elif branch, which runs only on import
	for ln := 1; ln <= len(lines); ln++ {
		text := lines[ln-1]
		if ln > guard && text != "" && text[0] != ' ' && text[0] != '\t' {
			t := strings.TrimSpace(text)
			other = strings.HasPrefix(t, "elif ") || strings.HasPrefix(t, "else") && (len(t) == 4 || t[4] == ':' || t[4] == ' ')
		}
		if other || inDef(ln) {
			continue // a function's body runs only when it is called
		}
		if ln == guard {
			text = text[strings.Index(text, ":")+1:]
		}
		for _, c := range callName.FindAllStringSubmatch(text, -1) {
			if to := byName[c[1]]; len(to) > 0 && !cKeywords[c[1]] && !seen[c[1]] {
				seen[c[1]] = true
				ix.Edges = append(ix.Edges, callEdge(from, c[1], to, p+":"+itoa(ln)))
			}
		}
	}
}

// scriptCalls records what a script's file-scope code names as edges from
// its script entry: a script runs its top-level lines, which no function
// holds, so it reaches the functions they name and no others. In shell,
// Perl, and Ruby a name counts with or without parentheses, as they call
// without them; in Python and PHP only a call does (callback = install runs
// nothing).
func (ix *Index) scriptCalls(p string, s *scanned, byName map[string][]string) {
	from := "entry:" + p + ":1"
	if !slices.ContainsFunc(ix.Entries, func(e Fact) bool { return e.ID == from && e.Kind == "script" }) {
		return
	}
	inDef := func(ln int) bool {
		return slices.ContainsFunc(s.syms, func(d Symbol) bool { return isDefKind(d.Kind) && d.line <= ln && ln <= d.End })
	}
	seen := map[string]bool{}
	for k, text := range strings.Split(s.code, "\n") {
		if inDef(k + 1) {
			continue
		}
		for _, loc := range wordRe.FindAllStringIndex(text, -1) {
			w := text[loc[0]:loc[1]]
			if (s.lang == "python" || s.lang == "php") && !strings.HasPrefix(strings.TrimLeft(text[loc[1]:], " \t"), "(") {
				continue
			}
			if to := byName[w]; len(to) > 0 && !cKeywords[w] && !seen[w] {
				seen[w] = true
				ix.Edges = append(ix.Edges, callEdge(from, w, to, p+":"+itoa(k+1)))
			}
		}
	}
}

var wordRe = regexp.MustCompile(`[A-Za-z_][\w]*`)

// callEdge is a call resolved by name. An edge lists at most eight of the
// definitions the name could mean; when it lists fewer than there are, Name
// keeps the called name, so a reader can resolve the rest (runnerReach).
func callEdge(from, name string, to []string, at string) Edge {
	e := Edge{Kind: "call", From: from, To: to, At: at, Ambiguous: len(to) > 1}
	if len(to) > 8 {
		e.To, e.Name = to[:8], name
	}
	return e
}

// digest sets Index.Digest from the files in path order: each path with its
// size and its content, the files the scanner skipped (too large, binary)
// included, then each module's files, so any edit, or a module budget that
// moves a file to another mod: ID, makes the saved survey sections stale.
func (ix *Index) digest(root string, files map[string]*scanned, manuals map[string]string) error {
	h := sha256.New()
	for _, f := range ix.Files {
		fmt.Fprintf(h, "%s\x00%d\x00", f.Path, f.Bytes)
		if s, ok := files[f.Path]; ok {
			h.Write([]byte(s.raw))
		} else if text, ok := manuals[strings.TrimPrefix(f.Path, "manual:")]; ok && strings.HasPrefix(f.Path, "manual:") {
			h.Write([]byte(text))
		} else if err := hashFile(h, filepath.Join(root, filepath.FromSlash(f.Path))); err != nil {
			return err
		}
		h.Write([]byte{0})
	}
	for _, m := range ix.Modules {
		fmt.Fprintf(h, "%s\x00%s\x00", m.ID, strings.Join(m.Files, "\x01"))
	}
	ix.Digest = hex.EncodeToString(h.Sum(nil))
	return nil
}

// hashFile writes a file's bytes to w.
func hashFile(w io.Writer, p string) error {
	fh, err := os.Open(p)
	if err != nil {
		return err
	}
	defer func() { _ = fh.Close() }()
	_, err = io.Copy(w, fh)
	return err
}

// scanned is one file read while building the index.
type scanned struct {
	code  string // comments, literals, and (C family) directives blanked
	raw   string
	lang  string
	calls []call
	syms  []Symbol
}

// listFiles lists the checkout's files: git's tracked files when root is a
// repository, else a walk that skips tool state and build output.
func listFiles(ctx context.Context, root string, git func(context.Context, string, ...string) string) ([]string, error) {
	var out []string
	if ls := git(ctx, root, "ls-files", "-z", "--cached", "--others", "--exclude-standard"); ls != "" {
		for _, p := range strings.Split(ls, "\x00") {
			if p == "" || skipped(p) {
				continue
			}
			out = append(out, p)
		}
		sort.Strings(out)
		return dedupe(out), nil
	}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if rel != "." && skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		out = append(out, rel)
		return nil
	})
	sort.Strings(out)
	return out, err
}

func skipped(p string) bool {
	for _, d := range strings.Split(path.Dir(p), "/") {
		if skipDirs[d] {
			return true
		}
	}
	return false
}

func runGit(ctx context.Context, dir string, args ...string) string {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return string(out)
}

func (ix *Index) surfaces(file, lang, code string) {
	rules := surfaceRules
	if lr, ok := langSurfaceRules[lang]; ok {
		rules, code = lr, strings.ToLower(code) // the keyword languages are case-insensitive
	}
	for _, r := range rules {
		locs := r.re.FindAllStringIndex(code, -1)
		if len(locs) == 0 {
			continue
		}
		line := strings.Count(code[:locs[0][0]], "\n") + 1
		ix.Surfaces = append(ix.Surfaces, Fact{ID: "io:" + r.kind + ":" + file + ":" + itoa(line), Kind: r.kind, At: file + ":" + itoa(line),
			Detail: strings.TrimSpace(strings.TrimRight(code[locs[0][0]:locs[0][1]], "(<:")), Count: len(locs)})
	}
}

func (ix *Index) entries(file, role, lang, raw string, syms []Symbol) {
	have := map[string]bool{}
	add := func(kind string, line int, detail string) {
		at := file + ":" + itoa(line)
		id := "entry:" + at
		if have[id] { // two entries on one line (class A {}; class B {};): the name tells them apart
			name := detail
			if _, n, ok := strings.Cut(detail, " "); ok {
				name = n
			}
			id += "#" + name
			for n := 2; have[id]; n++ {
				id = "entry:" + at + "#" + name + "~" + itoa(n)
			}
		}
		have[id] = true
		ix.Entries = append(ix.Entries, Fact{ID: id, Kind: kind, At: at, Detail: detail})
	}
	for _, s := range syms {
		n := lastName(s.Name)
		switch {
		case s.Kind == "program":
			add("main", s.line, s.Name)
		case isDefKind(s.Kind) && (n == "main" || n == "WinMain" || n == "wmain") && !strings.Contains(s.Name, "::"),
			lang == "java" && s.Kind == "method" && n == "main" && !declares(raw, s.line, "private"), // App::main
			lang == "csharp" && s.Kind == "method" && n == "Main" && declares(raw, s.line, "static"): // Program::Main
			add("main", s.line, s.Name)
		case s.Public && role == "source" && (lang == "php" || lang == "python" || lang == "ruby" || lang == "javascript" || lang == "typescript" || lang == "java" || lang == "csharp" || lang == "go" || lang == "cpp" || lang == "c" || lang == "rust"):
			// A function defined in a C or C++ header (inline int parse(…) {…}) is
			// part of the API every includer calls, as a method is.
			if isDefKind(s.Kind) && (strings.Contains(s.Name, "::") || (lang == "c" || lang == "cpp") && isHeader(file) || exportsFuncs(lang, raw) || exported(lang, raw, s)) || !isDefKind(s.Kind) && (s.Kind == "class" || s.Kind == "struct" || s.Kind == "interface" || s.Kind == "trait" || s.Kind == "type" || s.Kind == "enum" || s.Kind == "union") {
				add("api", s.line, s.Kind+" "+s.Name)
			}
		}
	}
	if lang == "python" {
		if m := pyMain.FindStringIndex(raw); m != nil {
			add("main", strings.Count(raw[:m[0]], "\n")+1, "__main__")
		}
	}
	if shebang.MatchString(raw) && (lang == "shell" || lang == "perl" || lang == "python" || lang == "ruby" || lang == "php") {
		add("script", 1, strings.SplitN(raw, "\n", 2)[0])
	}
}

// declares reports whether line ln of raw has the modifier word.
func declares(raw string, ln int, word string) bool {
	lines := strings.Split(raw, "\n")
	return ln >= 1 && ln <= len(lines) && regexp.MustCompile(`\b`+word+`\b`).MatchString(lines[ln-1])
}

// exported reports whether a top-level function is declared exported where
// the language marks it: Rust pub (not pub(crate)), or a JavaScript or
// TypeScript export. A library's callers reach it.
func exported(lang, raw string, s Symbol) bool {
	var pat string
	switch lang {
	case "rust":
		pat = `\bpub\s+(?:(?:const|async|unsafe|extern\s*(?:"[^"]*")?)\s+)*fn\s+`
	case "javascript", "typescript":
		pat = `\bexport\s+(?:default\s+)?(?:async\s+)?function\s*\*?\s*`
	default:
		return false
	}
	lines := strings.Split(raw, "\n")
	return s.line >= 1 && s.line <= len(lines) && regexp.MustCompile(pat+regexp.QuoteMeta(s.Name)+`\b`).MatchString(lines[s.line-1])
}

// exportsFuncs reports whether a file's public top-level functions are an API
// its importers call: a Go package other than main (exported names), or a
// Python module that is not a script (no shebang, no main guard; names without
// a leading underscore). Elsewhere a top-level function's Public means only
// that it is not static, and a script's functions are its own helpers.
func exportsFuncs(lang, raw string) bool {
	switch lang {
	case "go":
		return !goMainPkg.MatchString(raw)
	case "python":
		return !shebang.MatchString(raw) && !pyMain.MatchString(raw)
	}
	return false
}

// headerEntries adds the functions a header declares without a body
// (headerAPI) as api entry points, once per line.
func (ix *Index) headerEntries(file, code string) {
	have, seen := map[string]bool{}, map[string]bool{}
	for _, e := range ix.Entries {
		have[e.ID], seen[e.At+" "+e.Detail] = true, true
	}
	for _, d := range headerAPI(code) {
		at, detail := file+":"+itoa(d.line), d.Kind+" "+d.Name
		if seen[at+" "+detail] { // the symbol path already listed it
			continue
		}
		id := "entry:" + at
		if have[id] { // another declaration on the same line: its name tells them apart
			id += "#" + d.Name
		}
		for n := 2; have[id]; n++ { // and an ordinal tells same-line overloads apart
			id = "entry:" + at + "#" + d.Name + "~" + itoa(n)
		}
		have[id] = true
		ix.Entries = append(ix.Entries, Fact{ID: id, Kind: "api", At: at, Detail: detail})
	}
}

// opaqueEntries drops a forward-declared type (class CScriptVar;) from the
// entry points when a header defines it: callers reach the definition, which
// is an entry of its own. A type no header defines stays, as the opaque handle
// callers name, even when a .c or .cpp file defines it privately.
func (ix *Index) opaqueEntries() {
	defined := map[string]bool{}
	for _, s := range ix.Symbols {
		switch f, _ := splitAt(s.At); s.Kind {
		case "class", "struct", "union", "enum":
			defined[s.Name] = defined[s.Name] || isHeader(f)
		}
	}
	ix.Entries = slices.DeleteFunc(ix.Entries, func(e Fact) bool {
		name, ok := strings.CutPrefix(e.Detail, "opaque ")
		return ok && defined[name]
	})
}

func isHeader(p string) bool {
	switch strings.ToLower(path.Ext(p)) { // API.HPP is a header too
	case ".h", ".hh", ".hpp", ".hxx", ".inl", ".ipp":
		return true
	}
	return false
}

// imports records include and import edges: to a file of the checkout when the
// target resolves to one, else to an external dependency (counted in Deps).
func (ix *Index) imports(file, lang, raw string, exists func(string) bool) {
	dir := path.Dir(file)
	lineOf := func(off int) string { return file + ":" + itoa(strings.Count(raw[:off], "\n")+1) }
	ext := func(kind, name string, at string) {
		ix.Edges = append(ix.Edges, Edge{Kind: kind, From: "file:" + file, To: []string{"ext:" + name}, At: at})
	}
	local := func(kind, target, at string) {
		ix.Edges = append(ix.Edges, Edge{Kind: kind, From: "file:" + file, To: []string{"file:" + target}, At: at})
	}
	resolve := func(cands ...string) string {
		for _, c := range cands {
			if c = path.Clean(c); exists(c) {
				return c
			}
		}
		return ""
	}
	switch lang {
	case "c", "cpp", "objc":
		for _, m := range cInclude.FindAllStringSubmatchIndex(raw, -1) {
			name := raw[m[4]:m[5]]
			if t := resolve(path.Join(dir, name), name); t != "" {
				local("include", t, lineOf(m[0]))
			} else if t := ix.suffix(name); t != "" && raw[m[2]:m[3]] == `"` {
				local("include", t, lineOf(m[0]))
			} else {
				ext("include", name, lineOf(m[0]))
			}
		}
	case "python":
		for _, m := range pyImport.FindAllStringSubmatchIndex(raw, -1) {
			var mod string
			if m[2] >= 0 {
				mod = raw[m[2]:m[3]]
			} else {
				mod = raw[m[4]:m[5]]
			}
			p := strings.ReplaceAll(strings.TrimLeft(mod, "."), ".", "/")
			if t := resolve(path.Join(dir, p+".py"), p+".py", path.Join(p, "__init__.py"), path.Join(dir, p, "__init__.py")); t != "" && p != "" {
				local("import", t, lineOf(m[0]))
			} else if p != "" {
				ext("import", strings.Split(mod, ".")[0], lineOf(m[0]))
			}
		}
	case "php":
		for _, m := range phpInclude.FindAllStringSubmatchIndex(raw, -1) {
			name := raw[m[2]:m[3]]
			if t := resolve(path.Join(dir, name), name); t != "" {
				local("include", t, lineOf(m[0]))
			}
		}
		for _, m := range phpUse.FindAllStringSubmatchIndex(raw, -1) {
			name := raw[m[2]:m[3]]
			if t := ix.suffix(strings.ReplaceAll(name, `\`, "/") + ".php"); t != "" {
				local("import", t, lineOf(m[0]))
			} else {
				ext("import", strings.Split(name, `\`)[0], lineOf(m[0]))
			}
		}
	case "javascript", "typescript":
		for _, m := range jsImport.FindAllStringSubmatchIndex(raw, -1) {
			var name string
			if m[2] >= 0 {
				name = raw[m[2]:m[3]]
			} else {
				name = raw[m[4]:m[5]]
			}
			if strings.HasPrefix(name, ".") {
				p := path.Join(dir, name)
				if t := resolve(p, p+".js", p+".ts", p+".tsx", p+".jsx", p+".mjs", path.Join(p, "index.js"), path.Join(p, "index.ts")); t != "" {
					local("import", t, lineOf(m[0]))
				}
				continue
			}
			parts := strings.Split(name, "/")
			n := parts[0]
			if strings.HasPrefix(n, "@") && len(parts) > 1 {
				n += "/" + parts[1]
			}
			ext("import", n, lineOf(m[0]))
		}
	case "java", "kotlin", "scala", "groovy":
		for _, m := range javaImport.FindAllStringSubmatchIndex(raw, -1) {
			name := raw[m[2]:m[3]]
			if t := ix.suffix(strings.ReplaceAll(name, ".", "/") + path.Ext(file)); t != "" {
				local("import", t, lineOf(m[0]))
			} else {
				ps := strings.Split(name, ".")
				ext("import", strings.Join(ps[:min(2, len(ps))], "."), lineOf(m[0]))
			}
		}
	case "rust":
		for _, m := range rustMod.FindAllStringSubmatchIndex(raw, -1) {
			name := raw[m[2]:m[3]]
			if t := resolve(path.Join(dir, name+".rs"), path.Join(dir, name, "mod.rs")); t != "" {
				local("import", t, lineOf(m[0]))
			}
		}
		for _, m := range rustUse.FindAllStringSubmatchIndex(raw, -1) {
			if n := raw[m[2]:m[3]]; n != "crate" && n != "self" && n != "super" && n != "std" && n != "core" && n != "alloc" {
				ext("import", n, lineOf(m[0]))
			}
		}
	case "ruby":
		for _, m := range rubyReq.FindAllStringSubmatchIndex(raw, -1) {
			name := raw[m[4]:m[5]]
			if t := resolve(path.Join(dir, name+".rb"), path.Join(dir, name), name+".rb", path.Join("lib", name+".rb")); t != "" {
				local("import", t, lineOf(m[0]))
			} else if m[2] < 0 {
				ext("import", strings.Split(name, "/")[0], lineOf(m[0]))
			}
		}
	case "csharp":
		for _, m := range csUsing.FindAllStringSubmatchIndex(raw, -1) {
			ps := strings.Split(raw[m[2]:m[3]], ".")
			ext("import", strings.Join(ps[:min(2, len(ps))], "."), lineOf(m[0]))
		}
	}
}

// suffix finds the one checkout file whose path ends with p, or "".
func (ix *Index) suffix(p string) string {
	var found string
	for _, f := range ix.Files {
		if f.Path == p || strings.HasSuffix(f.Path, "/"+p) {
			if found != "" {
				return "" // ambiguous: do not guess
			}
			found = f.Path
		}
	}
	return found
}

func (ix *Index) history(ctx context.Context, root string, git func(context.Context, string, ...string) string) {
	log := git(ctx, root, "log", "--no-merges", "--format=@%ad %ae", "--date=short", "--name-only", "-n", "5000")
	if log == "" {
		return
	}
	h := &History{}
	authors := map[string]bool{}
	churn := map[string]int{}
	for _, l := range strings.Split(log, "\n") {
		switch {
		case strings.HasPrefix(l, "@"):
			fs := strings.Fields(l[1:])
			h.Commits++
			if len(fs) > 0 {
				if h.Last == "" {
					h.Last = fs[0]
				}
				h.First = fs[0]
			}
			if len(fs) > 1 {
				authors[fs[1]] = true // counted, never written: an address is personal data
			}
		case strings.TrimSpace(l) != "":
			churn[strings.TrimSpace(l)]++
		}
	}
	h.Authors = len(authors)
	h.Shallow = strings.TrimSpace(git(ctx, root, "rev-parse", "--is-shallow-repository")) == "true"
	ix.History = h
	for k := range ix.Files {
		ix.Files[k].Commits = churn[ix.Files[k].Path]
	}
}

func (ix *Index) counts() {
	langs, other := map[string]*Count{}, map[string]*Count{}
	for _, f := range ix.Files {
		switch {
		case f.Lang != "" && f.Role != "build" && f.Role != "doc":
			c := langs[f.Lang]
			if c == nil {
				c = &Count{Name: f.Lang, CountedOnly: !indexedLang(f.Lang)}
				langs[f.Lang] = c
			}
			c.Files++
			c.Lines += f.Lines
		case f.Role == "data":
			ext := strings.ToLower(path.Ext(f.Path))
			if ext == "" {
				ext = "(none)"
			}
			c := other[ext]
			if c == nil {
				c = &Count{Name: ext}
				other[ext] = c
			}
			c.Files++
			c.Lines += f.Lines
		}
	}
	sortCounts := func(m map[string]*Count) []Count {
		var out []Count
		for _, c := range m {
			out = append(out, *c)
		}
		sort.Slice(out, func(i, j int) bool {
			if out[i].Lines != out[j].Lines {
				return out[i].Lines > out[j].Lines
			}
			return out[i].Name < out[j].Name
		})
		return out
	}
	ix.Languages, ix.NotIndexed = sortCounts(langs), sortCounts(other)
	// Dependencies included or imported from outside the checkout.
	uses := map[string]int{}
	first := map[string]string{}
	for _, e := range ix.Edges {
		if e.Kind == "call" || !strings.HasPrefix(e.To[0], "ext:") {
			continue
		}
		n := strings.TrimPrefix(e.To[0], "ext:")
		uses[n]++
		if _, ok := first[n]; !ok {
			first[n] = e.At
		}
	}
	for n, u := range uses {
		ix.Deps = append(ix.Deps, Dep{ID: "dep:include:" + n, Name: n, Source: "include", At: first[n], Uses: u})
	}
}

// modules groups the indexed files (source, test, and vendored code) by
// directory, then splits a directory larger than the budget along file stems.
func (ix *Index) modules(budget int) {
	byDir := map[string][]File{}
	for _, f := range ix.Files {
		if f.Lang == "" || f.Role == "build" || f.Role == "doc" || f.Role == "data" {
			continue
		}
		byDir[path.Dir(f.Path)] = append(byDir[path.Dir(f.Path)], f)
	}
	fileMod := map[string]string{}
	for _, dir := range sortedKeys(byDir) {
		fs := byDir[dir]
		// Stems keep a header with its implementation (TinyJS.h, TinyJS.cpp).
		stems := map[string][]File{}
		for _, f := range fs {
			stem := strings.TrimSuffix(path.Base(f.Path), path.Ext(f.Path))
			stems[stem] = append(stems[stem], f)
		}
		name := dir
		if name == "." {
			name = "(root)"
		}
		var cur *Module
		part := 0
		flush := func() {
			if cur != nil {
				ix.Modules = append(ix.Modules, *cur)
				cur = nil
			}
		}
		for _, stem := range sortedKeys(stems) {
			group := stems[stem]
			tokens := 0
			for _, f := range group {
				tokens += f.Bytes / 4
			}
			if cur != nil && cur.Tokens+tokens > budget {
				flush()
			}
			if cur == nil {
				part++
				id := "mod:" + name
				if part > 1 {
					id += "#" + itoa(part)
				}
				cur = &Module{ID: id, Dir: dir}
			}
			for _, f := range group {
				cur.Files = append(cur.Files, f.Path)
				cur.Lines += f.Lines
				cur.Tokens += f.Bytes / 4
				fileMod[f.Path] = cur.ID
			}
		}
		flush()
	}
	// A directory split in parts: name the first part #1 too.
	parts := map[string]int{}
	for _, m := range ix.Modules {
		parts[strings.SplitN(m.ID, "#", 2)[0]]++
	}
	for k, m := range ix.Modules {
		if parts[m.ID] > 1 && !strings.Contains(m.ID, "#") {
			old := m.ID
			ix.Modules[k].ID = m.ID + "#1"
			for f, id := range fileMod {
				if id == old {
					fileMod[f] = ix.Modules[k].ID
				}
			}
		}
	}
	idx := map[string]int{}
	for k, m := range ix.Modules {
		idx[m.ID] = k
	}
	for _, s := range ix.Symbols {
		if k, ok := idx[fileMod[s.file]]; ok {
			ix.Modules[k].Symbols++
		}
	}
	for _, e := range ix.Edges {
		from := fileMod[fileOf(e.From)]
		for _, t := range e.To {
			to := fileMod[fileOf(t)]
			if from == "" || to == "" || from == to {
				continue
			}
			ix.Modules[idx[from]].FanOut++
			ix.Modules[idx[to]].FanIn++
		}
	}
}

// fileOf is the file of a fact ID: file:<p>, sym:<p>#<name>.
func fileOf(id string) string {
	switch {
	case strings.HasPrefix(id, "file:"):
		return id[5:]
	case strings.HasPrefix(id, "sym:"):
		return strings.SplitN(id[4:], "#", 2)[0]
	case strings.HasPrefix(id, "entry:"): // a script's or main guard's calls
		f, _ := splitAt(id[6:])
		return f
	}
	return ""
}

// finish sorts every list and records the fact IDs for Resolve.
func (ix *Index) finish() {
	sort.Strings(ix.Systems)
	ix.Systems = dedupe(ix.Systems)
	byID := func(n int, id func(int) string) func(i, j int) bool {
		return func(i, j int) bool { return id(i) < id(j) }
	}
	sort.SliceStable(ix.Targets, byID(len(ix.Targets), func(i int) string { return ix.Targets[i].ID }))
	sort.SliceStable(ix.Deps, byID(len(ix.Deps), func(i int) string { return ix.Deps[i].ID }))
	ix.Deps = dedupeDeps(ix.Deps)
	for _, l := range []*[]Fact{&ix.Entries, &ix.Surfaces, &ix.Traps, &ix.Tests, &ix.Docs, &ix.Builtins, &ix.Uses} {
		fs := *l
		sort.SliceStable(fs, func(i, j int) bool { return fs[i].ID < fs[j].ID })
	}
	sort.SliceStable(ix.Markers, func(i, j int) bool { return lessAt(ix.Markers[i].At, ix.Markers[j].At) })
	sort.SliceStable(ix.Symbols, func(i, j int) bool { return lessAt(ix.Symbols[i].At, ix.Symbols[j].At) })
	sort.SliceStable(ix.Edges, func(i, j int) bool {
		if ix.Edges[i].At != ix.Edges[j].At {
			return lessAt(ix.Edges[i].At, ix.Edges[j].At)
		}
		return ix.Edges[i].From+strings.Join(ix.Edges[i].To, ",") < ix.Edges[j].From+strings.Join(ix.Edges[j].To, ",")
	})
	if ix.Traps == nil {
		ix.Traps = []Fact{}
	}
	if ix.Targets == nil {
		ix.Targets = []Target{}
	}
	ix.index()
}

// index builds the lookup tables of a built or loaded index.
func (ix *Index) index() {
	ix.byPath = map[string]int{}
	for k, f := range ix.Files {
		ix.byPath[f.Path] = k
	}
	ix.ids = map[string]bool{}
	for _, f := range ix.Files {
		ix.ids["file:"+f.Path] = true
	}
	for i, s := range ix.Symbols {
		ix.ids[s.ID] = true
		if s.file == "" { // read back from index.json, which keeps only At
			ix.Symbols[i].file, ix.Symbols[i].line = splitAt(s.At)
		}
	}
	for _, t := range ix.Targets {
		ix.ids[t.ID] = true
	}
	for _, d := range ix.Deps {
		ix.ids[d.ID] = true
	}
	for _, l := range [][]Fact{ix.Entries, ix.Surfaces, ix.Traps, ix.Tests, ix.Docs, ix.Builtins} {
		for _, f := range l {
			ix.ids[f.ID] = true
		}
	}
	for _, m := range ix.Markers {
		ix.ids[m.ID] = true
	}
	for _, m := range ix.Modules {
		ix.ids[m.ID] = true
	}
}

func dedupeDeps(ds []Dep) []Dep {
	var out []Dep
	for _, d := range ds {
		if n := len(out); n > 0 && out[n-1].ID == d.ID {
			out[n-1].Uses += d.Uses
			continue
		}
		out = append(out, d)
	}
	return out
}

// lessAt orders path:line locations by path, then line as a number.
func lessAt(a, b string) bool {
	pa, la := splitAt(a)
	pb, lb := splitAt(b)
	if pa != pb {
		return pa < pb
	}
	return la < lb
}

func splitAt(at string) (string, int) {
	j := strings.LastIndex(at, ":")
	if j < 0 {
		return at, 0
	}
	n, _ := strconv.Atoi(at[j+1:])
	return at[:j], n
}

// Resolve checks one citation against the index: a fact ID (file:, sym:,
// build:, dep:, entry:, io:, test:, doc:, marker:, mod:, builtin:), or a location
// path:line or path:first-last whose lines exist in the file.
func (ix *Index) Resolve(cite string) error {
	cite = strings.TrimSpace(cite)
	if ix.ids[cite] {
		return nil
	}
	for _, pre := range []string{"file:", "sym:", "build:", "dep:", "entry:", "io:", "trap:", "test:", "doc:", "marker:", "mod:", "builtin:", "probe:"} {
		if strings.HasPrefix(cite, pre) {
			if pre == "probe:" {
				return fmt.Errorf("%s: no such probe fact (run `aayu survey --golden <cases> --bin <legacy> --run --probe` first)", cite)
			}
			return fmt.Errorf("%s: no such fact in the index", cite)
		}
	}
	j := strings.LastIndex(cite, ":")
	if j < 0 {
		if _, ok := ix.byPath[cite]; ok {
			return nil
		}
		return fmt.Errorf("%s: not a fact ID or path:line", cite)
	}
	p, span := cite[:j], cite[j+1:]
	k, ok := ix.byPath[p]
	if !ok {
		return fmt.Errorf("%s: %s is not a file of the checkout", cite, p)
	}
	first, last := span, span
	if a, b, ok := strings.Cut(span, "-"); ok {
		first, last = a, b
	}
	f, err1 := strconv.Atoi(first)
	l, err2 := strconv.Atoi(last)
	if err1 != nil || err2 != nil || f < 1 || l < f {
		return fmt.Errorf("%s: want path:line or path:first-last", cite)
	}
	if n := ix.Files[k].Lines; l > n {
		return fmt.Errorf("%s: %s has %d lines", cite, p, n)
	}
	return nil
}

// HasFile reports whether p is a file of the checkout.
func (ix *Index) HasFile(p string) bool { _, ok := ix.byPath[p]; return ok }

func firstHeading(s string) string {
	for _, l := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(l), "#=*-")); t != "" {
			return clip(t, 80)
		}
	}
	return ""
}

func itoa(n int) string { return strconv.Itoa(n) }

func sortedKeys[V any](m map[string]V) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

func without(xs []string, x string) []string {
	var out []string
	for _, y := range xs {
		if y != x {
			out = append(out, y)
		}
	}
	return out
}
