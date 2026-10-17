package survey

import (
	"path"
	"strings"
)

// Comment syntax families: how the scanner strips comments and strings.
const (
	syntaxC     = "c"     // // and /* */ comments, "…" and '…' literals (C, C++, Java, JS, Go, Rust, PHP, …)
	syntaxHash  = "hash"  // # comments (Python, Ruby, shell, Perl, …)
	syntaxNone  = ""      // not scanned for symbols (data, markup)
	syntaxSQL   = "sql"   // -- comments
	syntaxBuild = "build" // build files: # comments, parsed for targets only
)

// Lang is one language the index recognises: its name, how its comments are
// written, and whether the scanner extracts symbols from it.
type Lang struct {
	Name    string
	Syntax  string
	Symbols bool // the scanner extracts definitions and calls
}

// langs maps a file extension (lower case, with the dot) to its language. A
// language missing here is counted under "not indexed" by extension, never guessed.
var langs = map[string]Lang{
	".c": {"c", syntaxC, true}, ".h": {"c", syntaxC, true},
	".cc": {"cpp", syntaxC, true}, ".cpp": {"cpp", syntaxC, true}, ".cxx": {"cpp", syntaxC, true}, ".c++": {"cpp", syntaxC, true},
	".hh": {"cpp", syntaxC, true}, ".hpp": {"cpp", syntaxC, true}, ".hxx": {"cpp", syntaxC, true}, ".inl": {"cpp", syntaxC, true}, ".ipp": {"cpp", syntaxC, true},
	".m": {"objc", syntaxC, true}, ".mm": {"objc", syntaxC, true},
	".go":   {"go", syntaxC, true},
	".rs":   {"rust", syntaxC, true},
	".java": {"java", syntaxC, true}, ".kt": {"kotlin", syntaxC, true}, ".kts": {"kotlin", syntaxC, true}, ".scala": {"scala", syntaxC, true}, ".groovy": {"groovy", syntaxC, true},
	".cs": {"csharp", syntaxC, true}, ".swift": {"swift", syntaxC, true},
	".js": {"javascript", syntaxC, true}, ".mjs": {"javascript", syntaxC, true}, ".cjs": {"javascript", syntaxC, true}, ".jsx": {"javascript", syntaxC, true},
	".ts": {"typescript", syntaxC, true}, ".tsx": {"typescript", syntaxC, true},
	".php": {"php", syntaxC, true}, ".inc": {"php", syntaxC, true},
	".py": {"python", syntaxHash, true}, ".rb": {"ruby", syntaxHash, true}, ".pl": {"perl", syntaxHash, true}, ".pm": {"perl", syntaxHash, true},
	".sh": {"shell", syntaxHash, true}, ".bash": {"shell", syntaxHash, true}, ".zsh": {"shell", syntaxHash, true},
	".lua": {"lua", syntaxNone, false}, ".r": {"r", syntaxHash, false}, ".tcl": {"tcl", syntaxHash, false},
	".f": {"fortran", syntaxNone, false}, ".f90": {"fortran", syntaxNone, false}, ".cob": {"cobol", syntaxNone, false}, ".cbl": {"cobol", syntaxNone, false},
	".pas": {"pascal", syntaxNone, false}, ".vb": {"vb", syntaxNone, false}, ".bas": {"basic", syntaxNone, false},
	".sql":   {"sql", syntaxSQL, false},
	".cmake": {"cmake", syntaxBuild, false}, ".mk": {"make", syntaxBuild, false}, ".gradle": {"gradle", syntaxC, false},
}

// docExt are the extensions of human-written documents.
var docExt = map[string]bool{".md": true, ".markdown": true, ".rst": true, ".txt": true, ".adoc": true, ".1": true, ".3": true, ".man": true, ".pod": true, ".org": true}

// buildNames are the file names of build systems the index reads for targets.
var buildNames = map[string]string{
	"makefile": "make", "gnumakefile": "make", "cmakelists.txt": "cmake", "configure.ac": "autotools", "configure.in": "autotools", "makefile.am": "autotools",
	"meson.build": "meson", "build": "bazel", "build.bazel": "bazel", "workspace": "bazel", "sconstruct": "scons", "premake5.lua": "premake",
	"go.mod": "go", "cargo.toml": "cargo", "package.json": "npm", "composer.json": "composer", "pom.xml": "maven", "build.gradle": "gradle",
	"build.gradle.kts": "gradle", "settings.gradle": "gradle", "setup.py": "setuptools", "pyproject.toml": "python", "setup.cfg": "setuptools",
	"requirements.txt": "pip", "gemfile": "bundler", "build.xml": "ant", "build.sbt": "sbt", "package.swift": "swiftpm", "rakefile": "rake",
}

// buildExt are build files recognised by extension.
var buildExt = map[string]string{".vcxproj": "msbuild", ".vcproj": "msbuild", ".sln": "msbuild", ".csproj": "msbuild", ".pro": "qmake", ".gemspec": "rubygems"}

// vendorDirs are directory names whose contents are third-party code: indexed
// as vendored, so their symbols never count as the product's.
var vendorDirs = map[string]bool{"vendor": true, "third_party": true, "thirdparty": true, "3rdparty": true, "external": true, "extern": true, "deps": true, "node_modules": true, "bower_components": true}

// skipDirs are never indexed: build output and tool state.
var skipDirs = map[string]bool{".git": true, ".hg": true, ".svn": true, ".aayudha": true, ".idea": true, ".vscode": true, "__pycache__": true, ".tox": true, ".venv": true, "venv": true, "target": true, "dist": true, ".gradle": true, ".next": true}

// classify returns a file's role and language. Roles: source, test, build, doc,
// vendored, data.
func classify(p string) (role string, lang Lang, system string) {
	base := strings.ToLower(path.Base(p))
	ext := strings.ToLower(path.Ext(p))
	if s, ok := buildNames[base]; ok {
		return "build", Lang{Name: s, Syntax: syntaxBuild}, s
	}
	if s, ok := buildExt[ext]; ok {
		return "build", Lang{Name: s}, s
	}
	for _, d := range strings.Split(path.Dir(p), "/") {
		if vendorDirs[strings.ToLower(d)] {
			l := langs[ext]
			return "vendored", l, ""
		}
	}
	if docExt[ext] || strings.HasPrefix(base, "readme") || base == "changelog" || base == "news" || base == "authors" || base == "copying" || base == "license" || base == "install" || base == "todo" {
		return "doc", Lang{}, ""
	}
	l, ok := langs[ext]
	if !ok {
		return "data", Lang{}, ""
	}
	if isTestPath(p) {
		return "test", l, ""
	}
	return "source", l, ""
}

// isTestPath reports whether a path is a test by the usual conventions of
// most stacks: a tests directory, or a test prefix or suffix in the name.
func isTestPath(p string) bool {
	lp := strings.ToLower(p)
	for _, d := range strings.Split(path.Dir(lp), "/") {
		switch d {
		case "test", "tests", "testing", "spec", "specs", "__tests__", "testdata", "t":
			return true
		}
	}
	orig := strings.TrimSuffix(path.Base(p), path.Ext(p))
	stem := strings.ToLower(orig)
	digitAfter := len(stem) > 4 && stem[4] >= '0' && stem[4] <= '9'
	return strings.HasPrefix(stem, "test_") || strings.HasPrefix(stem, "test") && digitAfter || strings.HasPrefix(stem, "run_tests") ||
		strings.HasSuffix(stem, "_test") || strings.HasSuffix(stem, "_tests") || strings.HasSuffix(stem, "_spec") ||
		strings.HasSuffix(stem, ".test") || strings.HasSuffix(stem, ".spec") ||
		strings.HasSuffix(orig, "Test") || strings.HasSuffix(orig, "Tests") || strings.HasPrefix(orig, "Test") && len(orig) > 4 && orig[4] >= 'A' && orig[4] <= 'Z'
}
