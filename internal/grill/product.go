package grill

import (
	"encoding/json"
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// ---- product types (WP5.8) ---------------------------------------------------

var (
	webDeps = []string{"next", "react", "react-dom", "vue", "svelte", "@sveltejs/kit", "nuxt", "@angular/core", "astro", "solid-js", "preact"}
	apiDeps = []string{"express", "fastify", "koa", "hono", "@nestjs/core", "@hapi/hapi"}

	goMain     = regexp.MustCompile(`(?m)^package main\b`)
	goCLI      = regexp.MustCompile(`\bflag\.(?:Parse|NewFlagSet|String|Bool|Int)\b|cobra\.Command|urfave/cli|kong\.Parse|\bos\.Args\b`)
	goHTTP     = regexp.MustCompile(`\bhttp\.(?:HandleFunc|Handle|ListenAndServe|NewServeMux)\b|gin\.Default\(|echo\.New\(|chi\.NewRouter\(`)
	pyCLI      = regexp.MustCompile(`argparse\.ArgumentParser|@click\.(?:command|group)|typer\.Typer\(`)
	pyAPI      = regexp.MustCompile(`FastAPI\(|Flask\(__name__\)|@app\.(?:get|post|route)\(`)
	rustCLI    = regexp.MustCompile(`\bclap::|#\[derive\([^)]*Parser`)
	productDoc = map[string]string{
		"web":     "Web app",
		"api":     "HTTP API",
		"cli":     "Command-line tool",
		"library": "Library",
	}
)

// products finds what kind of product the repository is, from conventions
// only: page routes or a web framework (web); API routes, an HTTP server, or
// an API framework (api); a main package that parses arguments, a package
// "bin", or an argument parser (cli); and, when none of those, a Go module, a
// package with "main" or "exports", or Python or Rust sources (library). Each
// type found is a `product:<type>` fact naming its evidence.
func (r *repo) products(pages, apis []route, add func(Fact)) []string {
	ev := map[string][]string{}
	note := func(t, s string) {
		if len(ev[t]) < 3 && !slices.Contains(ev[t], s) {
			ev[t] = append(ev[t], s)
		}
	}
	if len(pages) > 0 {
		note("web", fmt.Sprintf("%d page routes", len(pages)))
	}
	if len(apis) > 0 {
		note("api", fmt.Sprintf("%d API routes", len(apis)))
	}
	lib := false
	for _, p := range r.paths {
		base := path.Base(p)
		switch {
		case base == "package.json":
			var pkg struct {
				Bin     json.RawMessage   `json:"bin"`
				Main    string            `json:"main"`
				Exports json.RawMessage   `json:"exports"`
				Deps    map[string]string `json:"dependencies"`
				Dev     map[string]string `json:"devDependencies"`
			}
			if json.Unmarshal([]byte(r.files[p]), &pkg) != nil {
				continue
			}
			for _, d := range webDeps {
				if _, ok := pkg.Deps[d]; ok {
					note("web", d+" in "+p)
				}
			}
			for _, d := range apiDeps {
				if _, ok := pkg.Deps[d]; ok {
					note("api", d+" in "+p)
				}
			}
			if len(pkg.Bin) > 0 {
				note("cli", `"bin" in `+p)
			}
			lib = lib || pkg.Main != "" || len(pkg.Exports) > 0
		case base == "go.mod" || base == "Cargo.toml" || base == "pyproject.toml" || base == "setup.py":
			lib = true
		case strings.HasSuffix(p, ".go") && !isTest(p):
			if goMain.MatchString(r.files[p]) && goCLI.MatchString(r.files[p]) {
				note("cli", "package main in "+path.Dir(p))
			}
			if goHTTP.MatchString(r.files[p]) {
				note("api", "net/http server in "+p)
			}
		case strings.HasSuffix(p, ".py") && !isTest(p):
			if pyCLI.MatchString(r.files[p]) {
				note("cli", "argument parser in "+p)
			}
			if pyAPI.MatchString(r.files[p]) {
				note("api", "web framework app in "+p)
			}
		case strings.HasSuffix(p, ".rs") && rustCLI.MatchString(r.files[p]):
			note("cli", "clap parser in "+p)
		case base == "openapi.yaml" || base == "openapi.json" || base == "openapi.yml":
			note("api", "OpenAPI document "+p)
		}
	}
	if len(ev) == 0 && lib {
		note("library", "a package or module manifest with no app, server, or command entry point")
	}
	var out []string
	for _, t := range ProductTypes {
		if len(ev[t]) > 0 {
			out = append(out, t)
			add(Fact{ID: "product:" + t, Kind: "product", Text: fmt.Sprintf("%s: %s.", productDoc[t], strings.Join(ev[t], "; "))})
		}
	}
	return out
}

// ---- command-line surface ----------------------------------------------------

var (
	goFlagSet  = regexp.MustCompile(`flag\.NewFlagSet\(\s*"([A-Za-z][\w-]*)"`)
	goFlag     = regexp.MustCompile(`\b\w+\.(?:String|Bool|Int|Int64|Uint|Float64|Duration|Func)(?:Var)?\(\s*(?:&[\w.]+\s*,\s*)?"([A-Za-z][\w-]*)"`)
	cobraUse   = regexp.MustCompile(`Use:\s*"([A-Za-z][\w-]*)`)
	jsCommand  = regexp.MustCompile(`\.command\(\s*['"]([A-Za-z][\w-]*)`)
	jsOption   = regexp.MustCompile(`\.option\(\s*['"](?:-\w,\s*)?--([A-Za-z][\w-]*)`)
	pySub      = regexp.MustCompile(`add_parser\(\s*['"]([A-Za-z][\w-]*)`)
	pyArgument = regexp.MustCompile(`add_argument\(\s*['"](?:-\w['"],\s*['"])?--([A-Za-z][\w-]*)`)
)

// commands lists a command-line tool's commands and flags, read statically
// from the argument parsers it uses (Go flag sets and cobra, commander,
// argparse): a `cli:<name>` fact with the totals and one `command:<name>
// <sub>` fact per command with its flags. A flag defined before any command
// belongs to the tool itself.
func (r *repo) commands(add func(Fact)) {
	type cmd struct {
		name, file string
		flags      []string
	}
	tools := map[string][]*cmd{}
	var toolNames []string
	for _, p := range r.paths {
		src := r.files[p]
		if isTest(p) || src == "" {
			continue
		}
		var subRE, flagRE *regexp.Regexp
		switch {
		case strings.HasSuffix(p, ".go") && goMain.MatchString(src) && goCLI.MatchString(src):
			subRE, flagRE = goFlagSet, goFlag
			if cobraUse.MatchString(src) {
				subRE = cobraUse
			}
		case (strings.HasSuffix(p, ".js") || strings.HasSuffix(p, ".ts") || strings.HasSuffix(p, ".mjs")) && jsCommand.MatchString(src):
			subRE, flagRE = jsCommand, jsOption
		case strings.HasSuffix(p, ".py") && pyCLI.MatchString(src):
			subRE, flagRE = pySub, pyArgument
		default:
			continue
		}
		tool := toolName(r.root, p)
		if _, ok := tools[tool]; !ok {
			toolNames = append(toolNames, tool)
			tools[tool] = []*cmd{{name: "", file: p}}
		}
		type mark struct {
			pos  int
			name string
			sub  bool
		}
		var ms []mark
		for _, m := range subRE.FindAllStringSubmatchIndex(src, -1) {
			ms = append(ms, mark{m[0], src[m[2]:m[3]], true})
		}
		for _, m := range flagRE.FindAllStringSubmatchIndex(src, -1) {
			ms = append(ms, mark{m[0], src[m[2]:m[3]], false})
		}
		sort.Slice(ms, func(i, j int) bool { return ms[i].pos < ms[j].pos })
		cur := tools[tool][0]
		for _, m := range ms {
			if m.sub {
				cur = nil
				for _, c := range tools[tool] {
					if c.name == m.name {
						cur = c
					}
				}
				if cur == nil {
					cur = &cmd{name: clean(m.name, 40), file: p}
					tools[tool] = append(tools[tool], cur)
				}
				continue
			}
			if f := "--" + clean(m.name, 40); !slices.Contains(cur.flags, f) {
				cur.flags = append(cur.flags, f)
			}
		}
	}
	for _, tool := range toolNames {
		cs := tools[tool]
		var subs []string
		total := 0
		for _, c := range cs {
			total += len(c.flags)
			if c.name != "" {
				subs = append(subs, c.name)
			}
		}
		if len(subs) == 0 && total == 0 {
			continue
		}
		text := fmt.Sprintf("The %s command-line tool has %d commands and %d flags in all", tool, len(subs), total)
		if len(subs) > 0 {
			text += " (" + strings.Join(subs, ", ") + ")"
		}
		if g := len(cs[0].flags); g > 0 {
			text += fmt.Sprintf("; %d flags apply to the tool itself (%s)", g, strings.Join(cs[0].flags, ", "))
		}
		add(Fact{ID: "command:" + tool, Kind: "command", Text: text + ".", Source: cs[0].file})
		for _, c := range cs[1:] {
			ft := "no flags"
			if len(c.flags) > 0 {
				ft = strconv.Itoa(len(c.flags)) + " flags: " + strings.Join(c.flags, ", ")
			}
			add(Fact{ID: "command:" + tool + " " + c.name, Kind: "command", Text: fmt.Sprintf("Command `%s %s` takes %s.", tool, c.name, ft), Source: c.file})
		}
	}
}

// toolName is a command's name: the directory under cmd/ that holds it, else
// the directory of its file, else the repository's own name.
func toolName(root, p string) string {
	segs := strings.Split(path.Dir(p), "/")
	if i := lastIndex(segs, "cmd"); i >= 0 && i+1 < len(segs) {
		return clean(segs[i+1], 40)
	}
	if d := path.Base(path.Dir(p)); d != "." && d != "src" && d != "bin" {
		return clean(d, 40)
	}
	return clean(filepath.Base(root), 40)
}

// ---- motion ------------------------------------------------------------------

var (
	motionDecl   = regexp.MustCompile(`(?i)\b(transition|animation)(?:-duration)?\s*:\s*([^\s;}\n"'][^;}\n"']*)`)
	motionInline = regexp.MustCompile(`\b(transition|animation)\s*:\s*["'` + "`" + `]([^"'` + "`" + `]+)`)
	motionTime   = regexp.MustCompile(`\b([0-9]+(?:\.[0-9]+)?)(ms|s)\b`)
	reducedRE    = regexp.MustCompile(`prefers-reduced-motion|useReducedMotion|reducedMotion`)
	motionDeps   = []string{"framer-motion", "motion", "react-spring", "@react-spring/web", "gsap", "lottie-web", "@formkit/auto-animate"}
)

// motion reports the transitions and animations a web product already has:
// a `motion:<file>` fact per file with its durations, a motion library in
// package.json, and the gaps that matter for a motion request: no motion
// at all (no house durations to keep to) or motion without a
// prefers-reduced-motion rule.
func (r *repo) motion(add func(Fact)) {
	found, reduced := false, false
	for _, p := range r.paths {
		src := r.files[p]
		if src == "" || isTest(p) {
			continue
		}
		if reducedRE.MatchString(src) {
			reduced = true
		}
		if path.Base(p) == "package.json" {
			var pkg struct {
				Deps map[string]string `json:"dependencies"`
			}
			if json.Unmarshal([]byte(src), &pkg) == nil {
				for _, d := range motionDeps {
					if v, ok := pkg.Deps[d]; ok {
						found = true
						add(Fact{ID: "motion:" + d, Kind: "motion", Text: fmt.Sprintf("Motion library %s %s is a dependency.", d, clean(v, 30)), Source: p})
					}
				}
			}
			continue
		}
		var n int
		var times []string
		for _, re := range []*regexp.Regexp{motionDecl, motionInline} {
			for _, m := range re.FindAllStringSubmatch(src, -1) {
				if strings.Contains(m[2], "none") && !motionTime.MatchString(m[2]) {
					continue
				}
				n++
				for _, t := range motionTime.FindAllString(m[2], -1) {
					if !slices.Contains(times, t) {
						times = append(times, t)
					}
				}
			}
		}
		if strings.Contains(src, "@keyframes") {
			n++
		}
		if n == 0 {
			continue
		}
		found = true
		text := fmt.Sprintf("%d transition or animation declarations", n)
		if len(times) > 0 {
			text += " (durations " + strings.Join(times, ", ") + ")"
		}
		add(Fact{ID: "motion:" + p, Kind: "motion", Text: text + ".", Source: p})
	}
	switch {
	case !found:
		add(Fact{ID: "gap:motion", Kind: "gap", Text: "No transitions or animations found: there are no house durations or easing to keep to, so every motion number needs the human."})
	case !reduced:
		add(Fact{ID: "gap:reduced-motion", Kind: "gap", Text: "Motion exists but nothing honours prefers-reduced-motion (no media query or hook found)."})
	}
}

// ---- library surface -----------------------------------------------------------

var (
	jsExportObj  = regexp.MustCompile(`module\.exports\s*=\s*\{([^}]*)\}`)
	jsExportProp = regexp.MustCompile(`(?m)^\s*(?:module\.)?exports\.([A-Za-z_$][\w$]*)\s*=`)
	jsExportDecl = regexp.MustCompile(`(?m)^\s*export\s+(?:default\s+)?(?:async\s+)?(?:function\*?|const|let|class)\s+([A-Za-z_$][\w$]*)`)
	jsIdent      = regexp.MustCompile(`^[A-Za-z_$][\w$]*`)
)

// exports reports what a JavaScript library's entry file exports (WP5.8), the
// library's counterpart of a CLI's commands: an `export:<package>` fact with
// each exported name and its parameters, and a `test:<file>` fact for each test
// that calls one of them, so "simple to use" and "fast" questions can quote the
// calls a first-time user makes today.
func (r *repo) exports(add func(Fact)) {
	for _, p := range r.paths {
		if path.Base(p) != "package.json" {
			continue
		}
		var pkg struct {
			Name    string          `json:"name"`
			Main    string          `json:"main"`
			Exports json.RawMessage `json:"exports"`
		}
		if json.Unmarshal([]byte(r.files[p]), &pkg) != nil || pkg.Name == "" {
			continue
		}
		entry := pkg.Main
		var exp any
		if json.Unmarshal(pkg.Exports, &exp) == nil {
			switch e := exp.(type) {
			case string:
				entry = e
			case map[string]any:
				if s, ok := e["."].(string); ok {
					entry = s
				}
			}
		}
		if entry == "" {
			continue
		}
		file := path.Clean(path.Join(path.Dir(p), entry))
		src := r.files[file]
		if src == "" {
			continue
		}
		var names []string
		if m := jsExportObj.FindStringSubmatch(src); m != nil {
			for _, part := range strings.Split(m[1], ",") {
				if n := jsIdent.FindString(strings.TrimSpace(part)); n != "" {
					names = append(names, n)
				}
			}
		}
		for _, re := range []*regexp.Regexp{jsExportProp, jsExportDecl} {
			for _, m := range re.FindAllStringSubmatch(src, -1) {
				names = append(names, m[1])
			}
		}
		slices.Sort(names)
		names = slices.Compact(names)
		if len(names) == 0 {
			continue
		}
		var sigs []string
		for _, n := range names {
			sig := n
			if m := regexp.MustCompile(`(?:function\s+` + regexp.QuoteMeta(n) + `|\b` + regexp.QuoteMeta(n) + `\s*=\s*(?:async\s*)?(?:function\s*)?)\(([^)]*)\)`).FindStringSubmatch(src); m != nil {
				sig = n + "(" + strings.Join(strings.Fields(m[1]), " ") + ")"
			}
			sigs = append(sigs, sig)
		}
		add(Fact{ID: "export:" + pkg.Name, Kind: "export", Source: file,
			Text: fmt.Sprintf("Library %s exports %d name(s) from %s: %s.", pkg.Name, len(sigs), file, strings.Join(sigs, ", "))})
		prefix := ""
		if d := path.Dir(p); d != "." {
			prefix = d + "/"
		}
		for _, t := range r.paths {
			if !isTest(t) || !strings.HasPrefix(t, prefix) {
				continue
			}
			var calls []string
			for _, n := range names {
				if strings.Contains(r.files[t], n+"(") {
					calls = append(calls, n)
				}
			}
			if len(calls) > 0 {
				add(Fact{ID: "test:" + t, Kind: "test", Source: t, Text: fmt.Sprintf("Test %s calls %s.", t, strings.Join(calls, ", "))})
			}
		}
	}
}
