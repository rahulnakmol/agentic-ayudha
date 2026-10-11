package skills

import (
	"bytes"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

// Limits of the skill format (WP3.1).
const (
	MaxDescription = 200 // characters in the frontmatter description
	MaxBodyLines   = 500 // lines of SKILL.md after the frontmatter
)

// Lint rules. Each finding names the rule it breaks.
const (
	RuleFrontmatter = "frontmatter"  // SKILL.md exists and opens with a closed frontmatter block holding the required keys
	RuleName        = "name"         // name is one lowercase word equal to the skill directory
	RuleDescription = "description"  // description is non-empty and at most MaxDescription characters
	RuleBody        = "body-length"  // the body is at most MaxBodyLines lines
	RuleCall        = "aayu-call"    // every `aayu` call names a registry command, a valid first enum value, and known flags
	RuleScript      = "script"       // files under scripts/ only call `aayu <command>`
	RuleHook        = "hook"         // hooks only call `aayu hook <name>`; no inline shell, no prompt hooks
	RuleMobbin      = "mobbin-image" // no Mobbin-origin images, as files or as embeds
	RuleSymlink     = "symlink"      // no symlinks (V2)
)

// requiredKeys are the top-level frontmatter keys the generator writes (internal/verbs/gen).
var requiredKeys = []string{"name", "description", "disable-model-invocation", "user-invocable", "version", "aayu"}

// Finding is one lint failure.
type Finding struct {
	Path string // slash path relative to the linted root
	Rule string
	Msg  string
}

func (f Finding) String() string { return fmt.Sprintf("%s: [%s] %s", f.Path, f.Rule, f.Msg) }

// Lint checks every skill directory directly under root in fsys: root/<skill>/SKILL.md
// plus its scripts/, references/, and any other files. Files directly in root
// (such as README.md) are not skills and are ignored. Findings are sorted by path.
func Lint(fsys fs.FS, root string) []Finding {
	var out []Finding
	add := func(p, rule, format string, args ...any) {
		rel := strings.TrimPrefix(strings.TrimPrefix(p, root), "/")
		out = append(out, Finding{Path: rel, Rule: rule, Msg: fmt.Sprintf(format, args...)})
	}
	entries, err := fs.ReadDir(fsys, root)
	if err != nil {
		add(root, RuleFrontmatter, "cannot read skills root: %v", err)
		return out
	}
	for _, e := range entries {
		p := path.Join(root, e.Name())
		if e.Type()&fs.ModeSymlink != 0 {
			add(p, RuleSymlink, "skills are real copies, never symlinks (V2)")
			continue
		}
		if e.IsDir() {
			lintSkill(fsys, p, add)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

type addFunc func(p, rule, format string, args ...any)

func lintSkill(fsys fs.FS, dir string, add addFunc) {
	skillMD := path.Join(dir, "SKILL.md")
	if b, err := fs.ReadFile(fsys, skillMD); err != nil {
		add(skillMD, RuleFrontmatter, "missing SKILL.md")
	} else {
		lintSkillMD(skillMD, path.Base(dir), string(b), add)
	}
	_ = fs.WalkDir(fsys, dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			add(p, RuleFrontmatter, "cannot read: %v", err)
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			add(p, RuleSymlink, "skills are real copies, never symlinks (V2)")
			return nil
		}
		if d.IsDir() {
			return nil
		}
		b, err := fs.ReadFile(fsys, p)
		if err != nil {
			add(p, RuleFrontmatter, "cannot read: %v", err)
			return nil
		}
		rel := strings.TrimPrefix(p, dir+"/")
		lintMobbin(p, b, add)
		switch {
		case strings.HasPrefix(rel, "scripts/"):
			lintScript(p, string(b), add)
		case p != skillMD && isText(p):
			for _, c := range codeCalls(string(b)) {
				if msg := Resolve(c); msg != "" {
					add(p, RuleCall, "`aayu %s`: %s", strings.Join(c, " "), msg)
				}
			}
		}
		return nil
	})
}

// splitFrontmatter returns the frontmatter lines and the body of a SKILL.md;
// ok is false when the file does not open with a closed --- block.
func splitFrontmatter(s string) (front []string, body string, ok bool) {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	if !strings.HasPrefix(s, "---\n") {
		return nil, s, false
	}
	lines := strings.Split(s[4:], "\n")
	for i, l := range lines {
		if l == "---" {
			return lines[:i], strings.Join(lines[i+1:], "\n"), true
		}
	}
	return nil, s, false
}

var topKey = regexp.MustCompile(`^([a-z][a-z0-9_-]*):[ \t]*(.*)$`)

func lintSkillMD(p, dir, s string, add addFunc) {
	front, body, ok := splitFrontmatter(s)
	if !ok {
		add(p, RuleFrontmatter, "must open with a YAML frontmatter block closed by a --- line")
	} else {
		keys := map[string]string{}
		var hooks []string
		inHooks := false
		for _, l := range front {
			if m := topKey.FindStringSubmatch(l); m != nil {
				keys[m[1]] = m[2]
				inHooks = m[1] == "hooks"
				continue
			}
			if inHooks && (strings.HasPrefix(l, " ") || strings.HasPrefix(l, "\t")) {
				hooks = append(hooks, l)
			}
		}
		for _, k := range requiredKeys {
			if _, ok := keys[k]; !ok {
				add(p, RuleFrontmatter, "frontmatter is missing %q (regenerate with go generate ./...)", k)
			}
		}
		if name, ok := keys["name"]; ok {
			name = scalar(name)
			if !oneWord.MatchString(name) {
				add(p, RuleName, "name %q must be one lowercase word", name)
			} else if name != dir {
				add(p, RuleName, "name %q must equal the skill directory %q", name, dir)
			}
		}
		if d, ok := keys["description"]; ok {
			d = scalar(d)
			switch n := utf8.RuneCountInString(d); {
			case n == 0:
				add(p, RuleDescription, "description is empty")
			case n > MaxDescription:
				add(p, RuleDescription, "description is %d characters, want <= %d", n, MaxDescription)
			}
		}
		if v, ok := keys["hooks"]; ok && strings.TrimSpace(v) != "" && strings.TrimSpace(v) != "{}" && strings.TrimSpace(v) != "[]" {
			add(p, RuleHook, "hooks must be a block of command hooks calling `aayu hook <name>`, not %q", v)
		}
		lintHooks(p, hooks, add)
	}
	if n := lineCount(body); n > MaxBodyLines {
		add(p, RuleBody, "body is %d lines, want <= %d (move detail into references/)", n, MaxBodyLines)
	}
	for _, c := range codeCalls(body) {
		if msg := Resolve(c); msg != "" {
			add(p, RuleCall, "`aayu %s`: %s", strings.Join(c, " "), msg)
		}
	}
}

var oneWord = regexp.MustCompile(`^[a-z]+$`)

// scalar unquotes a one-line YAML scalar (plain, 'single', or "double" quoted).
func scalar(v string) string {
	v = strings.TrimSpace(v)
	if strings.HasPrefix(v, `"`) {
		if s, err := strconv.Unquote(v); err == nil {
			return s
		}
	}
	if len(v) >= 2 && v[0] == '\'' && v[len(v)-1] == '\'' {
		return strings.ReplaceAll(v[1:len(v)-1], "''", "'")
	}
	return v
}

func lineCount(s string) int {
	s = strings.TrimSuffix(s, "\n")
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

// HookCommand is the only command a hook may run: `aayu hook <name>`, optionally
// with the host it runs under.
var HookCommand = regexp.MustCompile(`^aayu hook [a-z][a-z0-9-]*( --host (amp|cursor|claude))?$`)

var hookField = regexp.MustCompile(`^[\s-]*(command|type|prompt|url):[ \t]*(.*)$`)

func lintHooks(p string, lines []string, add addFunc) {
	for _, l := range lines {
		m := hookField.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		v := scalar(m[2])
		switch m[1] {
		case "command":
			if !HookCommand.MatchString(v) {
				add(p, RuleHook, "hook command %q: hooks call only `aayu hook <name>` (no inline shell)", v)
			}
		case "type":
			if v != "command" {
				add(p, RuleHook, "hook type %q: only command hooks calling `aayu hook <name>` are allowed", v)
			}
		default:
			add(p, RuleHook, "hook field %q: only command hooks calling `aayu hook <name>` are allowed", m[1])
		}
	}
}

var (
	scriptSet    = regexp.MustCompile(`^set( -[a-zA-Z]+| -o [a-z]+)*$`)
	scriptCall   = regexp.MustCompile(`^(exec )?aayu( .*)?$`)
	shellSpecial = regexp.MustCompile("[;|&<>`\\\\]|\\$\\(")
)

// lintScript allows only comments (including a shebang), blank lines, `set`
// options, and `aayu <command> …` lines with no shell operators.
func lintScript(p, s string, add addFunc) {
	for i, l := range strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n") {
		l = strings.TrimSpace(l)
		switch {
		case l == "", strings.HasPrefix(l, "#"), scriptSet.MatchString(l):
		case scriptCall.MatchString(l) && !shellSpecial.MatchString(l):
			args := strings.Fields(strings.TrimPrefix(l, "exec "))[1:]
			if msg := Resolve(args); msg != "" {
				add(p, RuleCall, "line %d: `%s`: %s", i+1, l, msg)
			}
		default:
			add(p, RuleScript, "line %d: %q: scripts only call `aayu <command>`", i+1, l)
		}
	}
}

var (
	fence      = regexp.MustCompile("(?s)(?:^|\n)[ \t]*(```|~~~)[^\n]*\n(.*?)\n[ \t]*(?:```|~~~)")
	inlineCode = regexp.MustCompile("`([^`\n]+)`")
	callRE     = regexp.MustCompile(`(?:^|[\s(;&|])aayu((?:[ \t]+[^\s;&|)]+)*)`)
)

// codeCalls returns the argument lists of `aayu` calls inside fenced code blocks
// and inline code spans of markdown text; prose is not parsed.
func codeCalls(md string) [][]string {
	md = strings.ReplaceAll(md, "\r\n", "\n")
	var code []string
	for _, m := range fence.FindAllStringSubmatch(md, -1) {
		code = append(code, m[2])
	}
	rest := fence.ReplaceAllString(md, "\n")
	for _, m := range inlineCode.FindAllStringSubmatch(rest, -1) {
		code = append(code, m[1])
	}
	var out [][]string
	for _, c := range code {
		for _, line := range strings.Split(c, "\n") {
			for _, m := range callRE.FindAllStringSubmatch(line, -1) {
				if args := strings.Fields(m[1]); len(args) > 0 {
					out = append(out, args)
				}
			}
		}
	}
	return out
}

// Resolve checks one `aayu` call's arguments against the registry: the command
// exists, a bare first argument is a valid value when the first positional input
// is an enum, and every --flag is the command's own or global. It returns "" when
// the call resolves.
func Resolve(args []string) string {
	i := 0
	for i < len(args) && strings.HasPrefix(args[i], "-") { // global flags before the command
		name := strings.TrimLeft(strings.SplitN(args[i], "=", 2)[0], "-")
		if !slices.Contains(verbs.GlobalFlags, name) && name != "v" {
			return fmt.Sprintf("unknown global flag %q", args[i])
		}
		if slices.Contains(verbs.GlobalValueFlags, name) && !strings.Contains(args[i], "=") {
			i++
		}
		i++
	}
	if i >= len(args) {
		return "no command"
	}
	c, ok := verbs.Find(args[i])
	if !ok {
		return fmt.Sprintf("%q is not a registered verb or plumbing command", args[i])
	}
	rest := args[i+1:]
	var first *verbs.Input
	for k := range c.Inputs {
		if c.Inputs[k].Positional {
			first = &c.Inputs[k]
			break
		}
	}
	if first != nil && first.Type == verbs.TypeEnum && len(rest) > 0 && bare(rest[0]) && !slices.Contains(first.Enum, rest[0]) {
		return fmt.Sprintf("%s %s: want one of %s", c.Name, rest[0], strings.Join(first.Enum, ", "))
	}
	for _, a := range rest {
		if !strings.HasPrefix(a, "--") || a == "--" {
			continue
		}
		name := strings.SplitN(strings.TrimPrefix(a, "--"), "=", 2)[0]
		if name == "help" || slices.Contains(verbs.GlobalFlags, name) {
			continue
		}
		if !slices.ContainsFunc(c.Inputs, func(in verbs.Input) bool { return !in.Positional && in.Name == name }) {
			return fmt.Sprintf("%s has no flag --%s", c.Name, name)
		}
	}
	return ""
}

// bare reports whether an argument is a literal word rather than a placeholder,
// variable, quoted value, or number.
func bare(s string) bool { return bareWord.MatchString(s) }

var bareWord = regexp.MustCompile(`^[a-z][a-z-]*$`)

var (
	imageExt  = []string{".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".avif", ".bmp", ".tif", ".tiff", ".ico", ".heic"}
	textExt   = []string{".md", ".markdown", ".txt", ".html", ".htm", ".svg", ".json", ".yaml", ".yml", ".toml"}
	imageRefs = regexp.MustCompile(`(?i)!\[[^\]]*\]\(\s*<?([^)\s>]+)|<img\b[^>]*\bsrc\s*=\s*["']?([^"'\s>]+)|url\(\s*["']?([^"')\s]+)`)
)

func isText(p string) bool { return slices.Contains(textExt, strings.ToLower(path.Ext(p))) }

// lintMobbin flags Mobbin-origin images (WP6.4: Mobbin gives URLs and IDs, never
// pixels): an image file whose name or bytes (embedded metadata) mention Mobbin,
// and an image embed in a text file whose URL points at Mobbin. Plain links to
// Mobbin pages are allowed.
func lintMobbin(p string, b []byte, add addFunc) {
	lower := bytes.ToLower(b)
	if slices.Contains(imageExt, strings.ToLower(path.Ext(p))) &&
		(strings.Contains(strings.ToLower(p), "mobbin") || bytes.Contains(lower, []byte("mobbin"))) {
		add(p, RuleMobbin, "image file of Mobbin origin; reference Mobbin by URL or ID instead")
	}
	if !isText(p) {
		return
	}
	for _, m := range imageRefs.FindAllStringSubmatch(string(b), -1) {
		for _, u := range m[1:] {
			if strings.Contains(strings.ToLower(u), "mobbin") {
				add(p, RuleMobbin, "image embed %q of Mobbin origin; link to the Mobbin page instead", u)
			}
		}
	}
}
