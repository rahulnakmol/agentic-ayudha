package survey

import (
	"encoding/json"
	"path"
	"regexp"
	"sort"
	"strings"
)

// Target is something a build file produces: an executable, a library, a
// test, or a package, with the sources it names.
type Target struct {
	ID      string   `json:"id"`
	System  string   `json:"system"`
	Kind    string   `json:"kind"` // executable, library, test, package, script, rule
	Name    string   `json:"name"`
	At      string   `json:"at"`
	Sources []string `json:"sources,omitempty"`
}

// Dep is an external dependency: declared in a manifest or a build file, or
// included from outside the repository.
type Dep struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Source string `json:"source"` // the manifest, build file, or "include"/"import"
	At     string `json:"at,omitempty"`
	Uses   int    `json:"uses,omitempty"` // include or import sites
}

var (
	cmakeTarget = regexp.MustCompile(`(?is)\b(add_executable|add_library|add_test|cuda_add_executable)\s*\(\s*([^\s)]+)([^)]*)\)`)
	cmakeSet    = regexp.MustCompile(`(?is)\b(?:set|file\s*\(\s*glob(?:_recurse)?)\s*\(?\s*([A-Za-z_]\w*)\s+([^)]*)\)`)
	cmakeVar    = regexp.MustCompile(`\$\{([A-Za-z_]\w*)\}`)
	cmakeDep    = regexp.MustCompile(`(?i)\bfind_package\s*\(\s*([A-Za-z0-9_+-]+)`)
	makeAssign  = regexp.MustCompile(`^([A-Za-z_][\w]*)\s*[:+?]?=\s*(.*)$`)
	makeRule    = regexp.MustCompile(`^([A-Za-z0-9_./${}()-][A-Za-z0-9_./${}() -]*?)\s*:([^=].*|)$`)
	makeVar     = regexp.MustCompile(`\$[({]([A-Za-z_]\w*)(?::\.?(\w+)=\.?(\w+))?[)}]`)
	cargoName   = regexp.MustCompile(`(?m)^\s*name\s*=\s*"([^"]+)"`)
	tomlSection = regexp.MustCompile(`(?m)^\s*\[([^\]]+)\]\s*$`)
	reqLine     = regexp.MustCompile(`^\s*([A-Za-z0-9_.\-\[\]]+)`)
	consoleSc   = regexp.MustCompile(`["']([\w.-]+)\s*=\s*([\w.]+):(\w+)["']`)
	pomArtifact = regexp.MustCompile(`<artifactId>([^<]+)</artifactId>`)
	goRequire   = regexp.MustCompile(`(?m)^\s*(?:require\s+)?([a-z0-9.\-]+\.[a-z]{2,}/[^\s]+)\s+v[0-9]`)
)

// buildFacts reads one build file for targets and dependencies. text is the
// file with comments blanked; raw is the file as written (JSON manifests).
func buildFacts(file, system, text, raw string, exists func(string) bool) ([]Target, []Dep) {
	dir := path.Dir(file)
	rel := func(p string) string {
		p = strings.Trim(p, `"'`)
		for _, pre := range []string{"${CMAKE_CURRENT_LIST_DIR}/", "${CMAKE_CURRENT_SOURCE_DIR}/", "${CMAKE_SOURCE_DIR}/", "${PROJECT_SOURCE_DIR}/", "$(srcdir)/", "./"} {
			p = strings.TrimPrefix(p, pre)
		}
		return path.Clean(path.Join(dir, p))
	}
	var ts []Target
	var ds []Dep
	add := func(kind, name string, line int, srcs []string) {
		sort.Strings(srcs)
		ts = append(ts, Target{ID: "build:" + file + "#" + name, System: system, Kind: kind, Name: name, At: file + ":" + itoa(line), Sources: dedupe(srcs)})
	}
	dep := func(name string, line int) {
		if name == "" {
			return
		}
		at := ""
		if line > 0 {
			at = file + ":" + itoa(line)
		}
		ds = append(ds, Dep{ID: "dep:" + system + ":" + name, Name: name, Source: file, At: at})
	}
	lineOf := func(off int) int { return strings.Count(text[:off], "\n") + 1 }
	isSource := func(p string) bool { _, ok := langs[strings.ToLower(path.Ext(p))]; return ok }
	switch system {
	case "cmake":
		vars := map[string][]string{}
		for _, m := range cmakeSet.FindAllStringSubmatch(text, -1) {
			vars[m[1]] = append(vars[m[1]], strings.Fields(m[2])...)
		}
		var expand func(ws []string, depth int) []string
		expand = func(ws []string, depth int) []string {
			var out []string
			for _, w := range ws {
				if m := cmakeVar.FindStringSubmatch(w); m != nil && w == m[0] && depth < 5 {
					out = append(out, expand(vars[m[1]], depth+1)...)
					continue
				}
				out = append(out, w)
			}
			return out
		}
		for _, m := range cmakeTarget.FindAllStringSubmatchIndex(text, -1) {
			fn, name := strings.ToLower(text[m[2]:m[3]]), text[m[4]:m[5]]
			var srcs []string
			for _, w := range expand(strings.Fields(text[m[6]:m[7]]), 0) {
				if isSource(w) {
					srcs = append(srcs, rel(w))
				}
			}
			kind := map[string]string{"add_executable": "executable", "cuda_add_executable": "executable", "add_library": "library", "add_test": "test"}[fn]
			add(kind, name, lineOf(m[0]), srcs)
		}
		for _, m := range cmakeDep.FindAllStringSubmatchIndex(text, -1) {
			dep(text[m[2]:m[3]], lineOf(m[0]))
		}
	case "make", "autotools":
		lines := joinContinuations(strings.Split(text, "\n"))
		vars := map[string]string{}
		for _, l := range lines {
			if m := makeAssign.FindStringSubmatch(l.text); m != nil {
				vars[m[1]] = strings.TrimSpace(vars[m[1]] + " " + m[2])
			}
		}
		expand := func(s string) []string {
			for depth := 0; depth < 5 && strings.Contains(s, "$"); depth++ {
				s = makeVar.ReplaceAllStringFunc(s, func(v string) string {
					m := makeVar.FindStringSubmatch(v)
					val := vars[m[1]]
					if m[2] != "" {
						val = regexp.MustCompile(`\.`+regexp.QuoteMeta(m[2])+`\b`).ReplaceAllString(val, "."+m[3])
					}
					return val
				})
			}
			return strings.Fields(s)
		}
		for _, l := range lines {
			if strings.HasPrefix(l.text, "\t") || strings.HasPrefix(l.text, ".") || strings.Contains(l.text, "%") {
				continue
			}
			m := makeRule.FindStringSubmatch(l.text)
			if m == nil || makeAssign.MatchString(l.text) {
				continue
			}
			for _, name := range expand(m[1]) {
				var srcs []string
				for _, d := range expand(m[2]) {
					if isSource(d) {
						srcs = append(srcs, rel(d))
						continue
					}
					if strings.HasSuffix(d, ".o") || strings.HasSuffix(d, ".obj") {
						stem := strings.TrimSuffix(strings.TrimSuffix(d, ".o"), ".obj")
						for _, ext := range []string{".cpp", ".cc", ".cxx", ".c", ".m", ".mm"} {
							if exists(rel(stem + ext)) {
								srcs = append(srcs, rel(stem+ext))
								break
							}
						}
					}
				}
				kind := "rule"
				if len(srcs) > 0 {
					kind = "executable"
				}
				if strings.HasSuffix(name, ".a") || strings.HasSuffix(name, ".so") || strings.HasSuffix(name, ".dylib") || strings.HasSuffix(name, ".lib") {
					kind = "library"
				}
				if strings.HasSuffix(name, ".o") {
					continue
				}
				add(kind, name, l.line, srcs)
			}
		}
	case "npm", "composer":
		var m map[string]json.RawMessage
		if json.Unmarshal([]byte(raw), &m) != nil {
			return nil, nil
		}
		var name, typ string
		_ = json.Unmarshal(m["name"], &name)
		_ = json.Unmarshal(m["type"], &typ)
		kind := "package"
		if typ == "library" || typ == "project" {
			kind = typ
		}
		if name != "" {
			add(kind, name, jsonLine(raw, "name"), nil)
		}
		var bin map[string]string
		if json.Unmarshal(m["bin"], &bin) == nil {
			for k, v := range bin {
				add("executable", k, jsonLine(raw, "bin"), []string{rel(v)})
			}
		}
		var scripts map[string]string
		if json.Unmarshal(m["scripts"], &scripts) == nil {
			for k := range scripts {
				add("script", k, jsonLine(raw, k), nil)
			}
		}
		for _, sec := range []string{"dependencies", "require", "peerDependencies"} {
			var deps map[string]string
			if json.Unmarshal(m[sec], &deps) == nil {
				for k := range deps {
					dep(k, jsonLine(raw, k))
				}
			}
		}
	case "cargo", "python":
		sections := tomlSection.FindAllStringSubmatchIndex(raw, -1)
		for k, s := range sections {
			name := raw[s[2]:s[3]]
			end := len(raw)
			if k+1 < len(sections) {
				end = sections[k+1][0]
			}
			body := raw[s[1]:end]
			switch {
			case name == "package" || name == "project" || name == "lib" || name == "[bin]" || strings.HasPrefix(name, "[bin"):
				if n := cargoName.FindStringSubmatch(body); n != nil {
					kind := map[string]string{"package": "package", "project": "package", "lib": "library"}[name]
					if kind == "" {
						kind = "executable"
					}
					add(kind, n[1], strings.Count(raw[:s[0]], "\n")+1, nil)
				}
			case strings.HasSuffix(name, "dependencies") && !strings.Contains(name, "dev"):
				for _, l := range strings.Split(body, "\n") {
					if m := reqLine.FindStringSubmatch(l); m != nil && strings.Contains(l, "=") {
						dep(m[1], 0)
					}
				}
			case name == "project.scripts" || name == "tool.poetry.scripts":
				for _, l := range strings.Split(body, "\n") {
					if kv := strings.SplitN(l, "=", 2); len(kv) == 2 {
						add("executable", strings.TrimSpace(kv[0]), 0, nil)
					}
				}
			}
		}
	case "setuptools":
		for _, m := range consoleSc.FindAllStringSubmatchIndex(raw, -1) {
			add("executable", raw[m[2]:m[3]], lineOf(m[0]), []string{rel(strings.ReplaceAll(raw[m[4]:m[5]], ".", "/") + ".py")})
		}
	case "pip":
		for k, l := range strings.Split(raw, "\n") {
			if t := strings.TrimSpace(l); t != "" && !strings.HasPrefix(t, "#") && !strings.HasPrefix(t, "-") {
				if m := reqLine.FindStringSubmatch(t); m != nil {
					dep(m[1], k+1)
				}
			}
		}
	case "maven":
		if m := pomArtifact.FindStringSubmatchIndex(raw); m != nil {
			add("package", raw[m[2]:m[3]], lineOf(m[0]), nil)
		}
	case "go":
		for _, m := range goRequire.FindAllStringSubmatchIndex(raw, -1) {
			dep(raw[m[2]:m[3]], lineOf(m[0]))
		}
	}
	return ts, ds
}

type srcLine struct {
	line int
	text string
}

// joinContinuations joins backslash-continued lines, keeping each joined
// line's first line number.
func joinContinuations(lines []string) []srcLine {
	var out []srcLine
	for k := 0; k < len(lines); k++ {
		start, text := k+1, lines[k]
		for strings.HasSuffix(strings.TrimRight(text, " \t\r"), `\`) && k+1 < len(lines) {
			k++
			text = strings.TrimSuffix(strings.TrimRight(text, " \t\r"), `\`) + " " + lines[k]
		}
		out = append(out, srcLine{start, strings.TrimRight(text, "\r")})
	}
	return out
}

// jsonLine is the line of the first "key": in a JSON document, or 1.
func jsonLine(raw, key string) int {
	if j := strings.Index(raw, `"`+key+`"`); j >= 0 {
		return strings.Count(raw[:j], "\n") + 1
	}
	return 1
}

func dedupe(xs []string) []string {
	var out []string
	for k, x := range xs {
		if k == 0 || x != xs[k-1] {
			out = append(out, x)
		}
	}
	return out
}
