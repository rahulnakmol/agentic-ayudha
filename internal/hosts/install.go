package hosts

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

// Repository paths install manages (WP3.2).
const (
	LockPath       = ".aayudha/lock.toml"
	AgentsSkills   = ".agents/skills" // Amp and Cursor
	ClaudeSkills   = ".claude/skills" // Claude Code
	AgentsMD       = "AGENTS.md"
	ClaudeMD       = "CLAUDE.md"
	ClaudeMCP      = ".mcp.json"
	ClaudeSettings = ".claude/settings.json"
	CursorMCP      = ".cursor/mcp.json"
	CursorHooks    = ".cursor/hooks.json"
	CodexConfig    = ".codex/config.toml" // project config: Codex loads it for trusted projects only
	CodexHooks     = ".codex/hooks.json"
	AmpPlugin      = ".amp/plugins/aayudha.ts"
	AmpSettings    = ".config/amp/settings.json" // relative to the user's home
	ServerName     = "aayudha"                   // MCP server name in every host
	LockVersion    = 1
)

// AllHosts returns every host ID in registry order.
func AllHosts() []verbs.HostID {
	out := make([]verbs.HostID, len(verbs.Hosts))
	for i, h := range verbs.Hosts {
		out[i] = h.ID
	}
	return out
}

// SkillDir is the project skill directory a host reads natively. install
// before D-3.2b.g wrote each installed host's; SkillDirs is what it writes now.
func SkillDir(h verbs.HostID) string {
	if h == verbs.HostClaude {
		return ClaudeSkills
	}
	return AgentsSkills
}

// SkillDirs are the directories install keeps aayudha's skill copies in for
// the lock's hosts, chosen so each host reads one copy of each skill
// (D-3.2b.g). Claude Code reads only .claude/skills and Codex only
// .agents/skills. Cursor reads .agents/skills and, with its Third-Party
// Imports setting (on by default), .claude/skills too; Amp reads both and
// keeps the first of each name. So Cursor and Amp share Claude Code's copies
// when it is installed, and .agents/skills holds copies only for Codex or
// when Claude Code is not installed. With Claude Code, Codex, and Cursor all
// installed, Cursor reads both: Doubles reports it.
func (l Lock) SkillDirs() []string {
	has := func(h verbs.HostID) bool { return slices.Contains(l.Hosts, string(h)) }
	var dirs []string
	if has(verbs.HostClaude) {
		dirs = append(dirs, ClaudeSkills)
	}
	if has(verbs.HostCodex) || len(dirs) == 0 && (has(verbs.HostCursor) || has(verbs.HostAmp)) {
		dirs = append(dirs, AgentsSkills)
	}
	sort.Strings(dirs)
	return dirs
}

// Options configure one install plan.
type Options struct {
	Root          string         // repository root
	Home          string         // the user's home directory
	Hosts         []verbs.HostID // empty: the hosts in lock.toml, else every host
	Assets        fs.FS          // embedded tree with skills/ and hosts/
	Binary        string         // absolute path of the aayu binary hooks and MCP registrations run
	Version       string         // the binary's version, recorded as installed_by
	SkillsVersion string         // version of the skills this binary carries
	Pin           string         // --pin: must equal SkillsVersion; "" keeps an existing pin
	Windows       bool           // write text with CRLF line endings
	SkillsOnly    bool           // only skill copies and lock.toml (doctor --fix)
}

// Edit kinds.
const (
	Write  = "write"
	Delete = "delete"
	Unlink = "unlink" // remove a symlink that stands where a real copy belongs (V2)
)

// Edit is one file change. Paths are slash paths relative to the repository
// root, or to the user's home when Home is set.
type Edit struct {
	Kind string
	Path string
	Home bool
	Old  []byte // nil when the file does not exist
	New  []byte // nil unless Kind is Write
}

// Display is the edit's path as shown to a human.
func (e Edit) Display() string {
	if e.Home {
		return "~/" + e.Path
	}
	return e.Path
}

// ErrPinned means the repository pins a skills version this binary does not carry.
var ErrPinned = errors.New("skills are pinned to another version")

// Lock is .aayudha/lock.toml.
type Lock struct {
	Version       int                  `toml:"version"`
	InstalledBy   string               `toml:"installed_by"`
	SkillsVersion string               `toml:"skills_version"`
	Pin           string               `toml:"pin"`
	Hosts         []string             `toml:"hosts"`
	Skills        map[string]LockSkill `toml:"skills"`
	// ProductSkills are the product's own verify skills (WP4.5b.3) that install
	// mirrors from .agents/skills to the hosts that do not read it.
	ProductSkills map[string]LockProductSkill `toml:"product_skills"`
}

// LockProductSkill records one mirrored product skill: its source directory
// and the hash of what was copied.
type LockProductSkill struct {
	Source string `toml:"source"`
	SHA256 string `toml:"sha256"`
}

// LockSkill records one installed skill.
type LockSkill struct {
	Version string `toml:"version"`
	SHA256  string `toml:"sha256"`
}

// ReadLock reads the lock file; ok is false when there is none.
func ReadLock(root string) (l Lock, ok bool, err error) {
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(LockPath)))
	if errors.Is(err, fs.ErrNotExist) {
		return Lock{}, false, nil
	}
	if err != nil {
		return Lock{}, false, err
	}
	if err := toml.Unmarshal(b, &l); err != nil {
		return Lock{}, false, fmt.Errorf("%s: %w", LockPath, err)
	}
	return l, true, nil
}

func renderLock(l Lock) []byte {
	var b strings.Builder
	b.WriteString("# Written by `aayu install` (WP3.2); `aayu install --apply` and `aayu doctor --fix` rewrite it. Do not edit.\n")
	b.WriteString("# Skill hashes cover each skill directory with LF line endings, so the lock is the same on every OS.\n")
	fmt.Fprintf(&b, "version = %d\ninstalled_by = %q\nskills_version = %q\npin = %q\n", l.Version, l.InstalledBy, l.SkillsVersion, l.Pin)
	q := make([]string, len(l.Hosts))
	for i, h := range l.Hosts {
		q[i] = fmt.Sprintf("%q", h)
	}
	fmt.Fprintf(&b, "hosts = [%s]\n", strings.Join(q, ", "))
	b.WriteString("\n[skills]\n")
	names := make([]string, 0, len(l.Skills))
	for n := range l.Skills {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		fmt.Fprintf(&b, "%s = { version = %q, sha256 = %q }\n", n, l.Skills[n].Version, l.Skills[n].SHA256)
	}
	if len(l.ProductSkills) > 0 {
		b.WriteString("\n[product_skills]\n")
		for _, n := range slices.Sorted(maps.Keys(l.ProductSkills)) {
			fmt.Fprintf(&b, "%s = { source = %q, sha256 = %q }\n", n, l.ProductSkills[n].Source, l.ProductSkills[n].SHA256)
		}
	}
	return []byte(b.String())
}

// skillFile is one file of a skill, relative to the skill directory.
type skillFile struct {
	rel  string
	data []byte // LF line endings
}

// sourceSkills reads every skills/<name>/ directory that holds a SKILL.md.
func sourceSkills(assets fs.FS) (map[string][]skillFile, error) {
	out := map[string][]skillFile{}
	entries, err := fs.ReadDir(assets, "skills")
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := path.Join("skills", e.Name())
		if _, err := fs.Stat(assets, path.Join(dir, "SKILL.md")); err != nil {
			continue
		}
		var files []skillFile
		err := fs.WalkDir(assets, dir, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			b, err := fs.ReadFile(assets, p)
			if err != nil {
				return err
			}
			files = append(files, skillFile{rel: strings.TrimPrefix(p, dir+"/"), data: lf(b)})
			return nil
		})
		if err != nil {
			return nil, err
		}
		out[e.Name()] = files
	}
	return out, nil
}

// productSkills reads the product's own verify skills, .agents/skills/verify-*/
// with a SKILL.md (WP4.5b.3), that are not aayudha skills of the same name.
func (p *planner) productSkills(ours map[string][]skillFile) (map[string][]skillFile, error) {
	out := map[string][]skillFile{}
	if p.underSymlink(AgentsSkills) {
		return out, nil
	}
	entries, err := os.ReadDir(p.abs(AgentsSkills, false))
	if errors.Is(err, fs.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		dir := AgentsSkills + "/" + e.Name()
		if _, aayudha := ours[e.Name()]; aayudha || !e.IsDir() || !strings.HasPrefix(e.Name(), "verify-") {
			continue
		}
		files, err := readCopy(p.abs(dir, false))
		if err != nil || files["SKILL.md"] == nil {
			continue // a symlink or no skill: nothing to mirror
		}
		for _, rel := range slices.Sorted(maps.Keys(files)) {
			out[e.Name()] = append(out[e.Name()], skillFile{rel: rel, data: lf(files[rel])})
		}
	}
	return out, nil
}

// HashSkill hashes a skill's files: relative path and LF content, in path order.
func HashSkill(files map[string][]byte) string {
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	h := sha256.New()
	for _, n := range names {
		h.Write([]byte(n))
		h.Write([]byte{0})
		h.Write(lf(files[n]))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func lf(b []byte) []byte { return bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n")) }

func (o Options) text(b []byte) []byte {
	b = lf(b)
	if o.Windows {
		return bytes.ReplaceAll(b, []byte("\n"), []byte("\r\n"))
	}
	return b
}

// planner accumulates edits, and notes for edits it could not plan.
type planner struct {
	o     Options
	edits []Edit
	notes []string
}

func (p *planner) abs(rel string, home bool) string {
	base := p.o.Root
	if home {
		base = p.o.Home
	}
	return filepath.Join(base, filepath.FromSlash(rel))
}

// read returns a file's content, or nil when it is absent. A path whose
// directory chain inside the repository passes through a symlink reads as
// absent: the symlink is replaced, never written through.
func (p *planner) read(rel string, home bool) ([]byte, error) {
	if !home && p.underSymlink(rel) {
		return nil, nil
	}
	b, err := os.ReadFile(p.abs(rel, home))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return b, err
}

func (p *planner) underSymlink(rel string) bool {
	parts := strings.Split(rel, "/")
	for i := 1; i <= len(parts); i++ {
		fi, err := os.Lstat(p.abs(strings.Join(parts[:i], "/"), false))
		if err != nil {
			return false
		}
		if fi.Mode()&fs.ModeSymlink != 0 {
			return true
		}
	}
	return false
}

// write plans rel to hold content (already in final line endings); an
// existing file that differs only in line endings is left alone.
func (p *planner) write(rel string, home bool, content []byte) error {
	old, err := p.read(rel, home)
	if err != nil {
		return err
	}
	if old != nil && bytes.Equal(lf(old), lf(content)) {
		return nil
	}
	p.edits = append(p.edits, Edit{Kind: Write, Path: rel, Home: home, Old: old, New: content})
	return nil
}

// Result is an install plan.
type Result struct {
	Edits []Edit
	Lock  Lock
	Notes []string // edits install could not plan and a human must make
}

// Plan computes every edit install would make. It reads the repository and the
// home directory and writes nothing.
func Plan(o Options) (*Result, error) {
	p := &planner{o: o}
	old, _, err := ReadLock(o.Root)
	if err != nil {
		return nil, err
	}
	pin := old.Pin
	switch {
	case o.Pin != "" && o.Pin != o.SkillsVersion:
		return nil, fmt.Errorf("%w: --pin %s, but this aayu carries skills %s; install aayu %s to pin it", ErrPinned, o.Pin, o.SkillsVersion, o.Pin)
	case o.Pin != "":
		pin = o.Pin
	case pin != "" && pin != o.SkillsVersion:
		return nil, fmt.Errorf("%w: %s pins skills %s and this aayu carries %s; run install with --pin %s to move the pin, or use aayu %s", ErrPinned, LockPath, pin, o.SkillsVersion, o.SkillsVersion, pin)
	}

	hosts := map[verbs.HostID]bool{}
	for _, h := range old.Hosts {
		hosts[verbs.HostID(h)] = true
	}
	for _, h := range o.Hosts {
		hosts[h] = true
	}
	if len(hosts) == 0 {
		for _, h := range AllHosts() {
			hosts[h] = true
		}
	}
	var hostList []string
	for _, h := range AllHosts() {
		if hosts[h] {
			hostList = append(hostList, string(h))
		}
	}

	src, err := sourceSkills(o.Assets)
	if err != nil {
		return nil, err
	}
	lock := Lock{Version: LockVersion, InstalledBy: o.Version, SkillsVersion: o.SkillsVersion, Pin: pin, Hosts: hostList, Skills: map[string]LockSkill{}}
	if o.SkillsOnly && old.InstalledBy != "" {
		lock.InstalledBy = old.InstalledBy // a re-sync restores copies; it does not re-install
	}
	for name, files := range src {
		m := map[string][]byte{}
		for _, f := range files {
			m[f.rel] = f.data
		}
		lock.Skills[name] = LockSkill{Version: o.SkillsVersion, SHA256: HashSkill(m)}
	}

	dirs := lock.SkillDirs()
	for _, d := range dirs {
		if err := p.skillCopies(d, src, old.Skills); err != nil {
			return nil, err
		}
	}
	// A directory an earlier install filled that no host needs now loses
	// aayudha's copies, so no host lists a skill twice; the product's own
	// skills there stay.
	before := old.SkillDirs()
	for _, h := range old.Hosts {
		if d := SkillDir(verbs.HostID(h)); !slices.Contains(before, d) {
			before = append(before, d)
		}
	}
	for _, d := range before {
		if slices.Contains(dirs, d) || p.underSymlink(d) {
			continue
		}
		if fi, err := os.Lstat(p.abs(d, false)); err != nil || !fi.IsDir() {
			continue
		}
		previous := maps.Clone(old.Skills)
		if previous == nil {
			previous = map[string]LockSkill{}
		}
		for n := range src {
			previous[n] = LockSkill{}
		}
		if err := p.skillCopies(d, nil, previous); err != nil {
			return nil, err
		}
	}
	if len(dirs) == 2 && slices.Contains(hostList, string(verbs.HostCursor)) {
		p.notes = append(p.notes, cursorBoth)
	}
	if slices.Contains(hostList, string(verbs.HostClaude)) {
		// The product's verify skills live where Amp, Cursor, and Codex read
		// them; Claude Code gets a copy, recorded so a later install removes
		// the copy of a skill the product deleted.
		prod, err := p.productSkills(src)
		if err != nil {
			return nil, err
		}
		previous := map[string]LockSkill{}
		for n, ps := range old.ProductSkills {
			previous[n] = LockSkill{SHA256: ps.SHA256}
		}
		for n, files := range prod {
			m := map[string][]byte{}
			for _, f := range files {
				m[f.rel] = f.data
			}
			if lock.ProductSkills == nil {
				lock.ProductSkills = map[string]LockProductSkill{}
			}
			lock.ProductSkills[n] = LockProductSkill{Source: AgentsSkills + "/" + n, SHA256: HashSkill(m)}
		}
		if err := p.skillCopies(ClaudeSkills, prod, previous); err != nil {
			return nil, err
		}
	}
	if err := p.write(LockPath, false, o.text(renderLock(lock))); err != nil {
		return nil, err
	}
	if !o.SkillsOnly {
		if err := p.hostFiles(hosts, lock); err != nil {
			return nil, err
		}
	}
	sort.SliceStable(p.edits, func(i, j int) bool {
		a, b := p.edits[i], p.edits[j]
		if a.Home != b.Home {
			return !a.Home
		}
		if (a.Kind == Unlink) != (b.Kind == Unlink) {
			return a.Kind == Unlink
		}
		return a.Path < b.Path
	})
	return &Result{Edits: p.edits, Lock: lock, Notes: p.notes}, nil
}

// skillCopies plans real copies of every source skill under dir, removes files
// that are not part of a managed skill, and removes skills aayudha installed
// before that this binary no longer carries. Skills aayudha never installed
// (the product's own) are left alone.
func (p *planner) skillCopies(dir string, src map[string][]skillFile, previous map[string]LockSkill) error {
	if fi, err := os.Lstat(p.abs(dir, false)); err == nil && fi.Mode()&fs.ModeSymlink != 0 {
		p.edits = append(p.edits, Edit{Kind: Unlink, Path: dir})
	}
	names := make([]string, 0, len(src))
	for n := range src {
		names = append(names, n)
	}
	for n := range previous {
		if _, ok := src[n]; !ok {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		sdir := dir + "/" + name
		if fi, err := os.Lstat(p.abs(sdir, false)); err == nil && fi.Mode()&fs.ModeSymlink != 0 && !p.underSymlink(dir) {
			p.edits = append(p.edits, Edit{Kind: Unlink, Path: sdir})
		}
		want := map[string]bool{}
		for _, f := range src[name] {
			want[f.rel] = true
			if err := p.write(sdir+"/"+f.rel, false, p.o.text(f.data)); err != nil {
				return err
			}
		}
		if p.underSymlink(sdir) {
			continue
		}
		err := filepath.WalkDir(p.abs(sdir, false), func(fp string, d fs.DirEntry, err error) error {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			if err != nil || d.IsDir() {
				return err
			}
			rel, err := filepath.Rel(p.abs(sdir, false), fp)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			if !want[rel] {
				old, err := os.ReadFile(fp)
				if err != nil && d.Type()&fs.ModeSymlink == 0 {
					return err
				}
				kind := Delete
				if d.Type()&fs.ModeSymlink != 0 {
					kind = Unlink
				}
				p.edits = append(p.edits, Edit{Kind: kind, Path: sdir + "/" + rel, Old: old})
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// HookCommand is the exact command a host runs for an aayudha hook. The binary
// path is always double-quoted, so a path with spaces (C:\Program Files\…) runs.
func HookCommand(binary, name string, h verbs.HostID) string {
	return fmt.Sprintf(`"%s" hook %s --host %s`, binary, name, h)
}

var importsAgents = regexp.MustCompile(`(?m)^@AGENTS\.md\s*$`)

// ourHook matches the commands HookCommand writes, for any binary path.
var ourHook = regexp.MustCompile(`^"[^"]+" hook [a-z][a-z0-9-]* --host (amp|cursor|claude|codex)$`)

func (p *planner) hostFiles(hosts map[verbs.HostID]bool, lock Lock) error {
	existing, err := p.read(AgentsMD, false)
	if err != nil {
		return err
	}
	if err := p.write(AgentsMD, false, p.o.text(RenderAgentsMD(lf(existing), p.agentsSections(lock)...))); err != nil {
		return err
	}
	server := map[string]any{"command": p.o.Binary, "args": []any{"mcp"}}
	if hosts[verbs.HostClaude] {
		cm, err := p.read(ClaudeMD, false)
		if err != nil {
			return err
		}
		switch {
		case cm == nil:
			cm = []byte("# CLAUDE.md\n\nClaude Code reads the shared agent guidance from AGENTS.md through this import (written by `aayu install`).\n\n@AGENTS.md\n")
		case !importsAgents.Match(cm):
			cm = append(bytes.TrimRight(lf(cm), "\n"), []byte("\n\n@AGENTS.md\n")...)
		}
		if err := p.write(ClaudeMD, false, p.o.text(cm)); err != nil {
			return err
		}
		if err := p.mergeJSON(ClaudeMCP, false, func(m map[string]any) {
			obj(m, "mcpServers")[ServerName] = server
		}); err != nil {
			return err
		}
		if err := p.mergeJSON(ClaudeSettings, false, func(m map[string]any) {
			m["enabledMcpjsonServers"] = addString(m["enabledMcpjsonServers"], ServerName)
			// The repository's copies, hooks, and server serve every teammate,
			// so the aayudha plugin steps aside here for whoever has it: the
			// shared project setting overrides the user's (D-3.2b.h).
			obj(m, "enabledPlugins")[PluginID] = false
			// The status line shows journey state (WP3.2b); one the product set itself stays.
			if sl, ok := m["statusLine"].(map[string]any); !ok || ourHook.MatchString(fmt.Sprint(sl["command"])) {
				m["statusLine"] = map[string]any{"type": "command", "command": HookCommand(p.o.Binary, "status-line", verbs.HostClaude)}
			} else {
				p.notes = append(p.notes, ClaudeSettings+" already has a statusLine, so install leaves it: add `"+HookCommand(p.o.Binary, "status-line", verbs.HostClaude)+"` to it for the aayudha journey state")
			}
			setHooks(m, map[string]any{
				"SessionStart": map[string]any{"hooks": []any{map[string]any{"type": "command", "command": HookCommand(p.o.Binary, "session-start", verbs.HostClaude)}}},
				"PreToolUse": map[string]any{"matcher": "*",
					"hooks": []any{map[string]any{"type": "command", "command": HookCommand(p.o.Binary, "pre-tool", verbs.HostClaude)}}},
			})
		}); err != nil {
			return err
		}
	}
	if hosts[verbs.HostCursor] {
		if err := p.mergeJSON(CursorMCP, false, func(m map[string]any) {
			obj(m, "mcpServers")[ServerName] = server
		}); err != nil {
			return err
		}
		if err := p.mergeJSON(CursorHooks, false, func(m map[string]any) {
			m["version"] = json.Number("1")
			pre := map[string]any{"command": HookCommand(p.o.Binary, "pre-tool", verbs.HostCursor)}
			setHooks(m, map[string]any{
				"sessionStart":         map[string]any{"command": HookCommand(p.o.Binary, "session-start", verbs.HostCursor)},
				"beforeShellExecution": pre, "beforeMCPExecution": pre,
			})
		}); err != nil {
			return err
		}
		p.notes = append(p.notes, cursorPlugin)
	}
	if hosts[verbs.HostCodex] {
		if err := p.codexConfig(); err != nil {
			return err
		}
		if err := p.mergeJSON(CodexHooks, false, func(m map[string]any) {
			setHooks(m, map[string]any{
				"SessionStart": map[string]any{"hooks": []any{map[string]any{"type": "command", "command": HookCommand(p.o.Binary, "session-start", verbs.HostCodex)}}},
				"PreToolUse":   map[string]any{"hooks": []any{map[string]any{"type": "command", "command": HookCommand(p.o.Binary, "pre-tool", verbs.HostCodex)}}},
			})
		}); err != nil {
			return err
		}
		p.notes = append(p.notes, "Codex reads .codex/ only in a project you trust, and runs these hooks only after you trust them in /hooks; until then the aayudha plugin, if you have it, is not turned off here either (`aayu doctor` reports both)")
	}
	if hosts[verbs.HostAmp] {
		tmpl, err := fs.ReadFile(p.o.Assets, "hosts/amp/aayudha.ts")
		if err != nil {
			return err
		}
		bin, _ := json.Marshal(p.o.Binary)
		ver, _ := json.Marshal(p.o.Version)
		cmds, err := SlashCommands(p.o.Assets, verbs.HostAmp)
		if err != nil {
			return err
		}
		cj, _ := json.Marshal(cmds)
		plugin := bytes.ReplaceAll(bytes.ReplaceAll(bytes.ReplaceAll(lf(tmpl), []byte("__AAYU__"), bin), []byte("__VERSION__"), ver), []byte("__COMMANDS__"), cj)
		if err := p.write(AmpPlugin, false, p.o.text(plugin)); err != nil {
			return err
		}
		if _, err := os.Stat(p.abs(AmpSettings+"c", true)); err == nil {
			p.notes = append(p.notes, fmt.Sprintf("~/%sc exists, so install leaves Amp's user settings alone: add \"amp.mcpServers\": {%q: {\"command\": %q, \"args\": [\"mcp\"]}} to it by hand", AmpSettings, ServerName, p.o.Binary))
		} else if err := p.mergeJSON(AmpSettings, true, func(m map[string]any) {
			obj(m, "amp.mcpServers")[ServerName] = server
		}); err != nil {
			return err
		}
	}
	return nil
}

// Codex's MCP registration is a marked block in the project's
// .codex/config.toml: text outside the markers is never changed, and an
// aayudha server the product wrote itself is left alone with a note.
const (
	codexBegin = "# BEGIN aayudha (written by `aayu install`; do not edit inside this block)"
	codexEnd   = "# END aayudha"
)

var codexBlock = regexp.MustCompile(`(?s)` + regexp.QuoteMeta(codexBegin) + `.*?` + regexp.QuoteMeta(codexEnd) + `\n?`)

func (p *planner) codexConfig() error {
	old, err := p.read(CodexConfig, false)
	if err != nil {
		return err
	}
	cmd, _ := json.Marshal(p.o.Binary) // a JSON string is a TOML basic string
	text := string(lf(old))
	// The aayudha plugin steps aside in this trusted project: the project
	// layer overrides the user's [plugins] entry (D-3.2b.h). A table the
	// product wrote itself stays, since TOML allows it once.
	plugin := "\n[plugins.\"" + PluginID + "\"]\nenabled = false\n"
	if strings.Contains(codexBlock.ReplaceAllLiteralString(text, ""), "[plugins.\""+PluginID+"\"]") {
		plugin = ""
		p.notes = append(p.notes, CodexConfig+" already has a [plugins.\""+PluginID+"\"] table outside aayudha's block, so install leaves it: set enabled = false there, or Codex lists each aayudha skill twice for anyone with the plugin")
	}
	block := codexBegin + "\n[mcp_servers." + ServerName + "]\ncommand = " + string(cmd) + "\nargs = [\"mcp\"]\n" + plugin + codexEnd + "\n"
	switch {
	case codexBlock.MatchString(text):
		text = codexBlock.ReplaceAllLiteralString(text, block)
	case strings.Contains(text, "[mcp_servers."+ServerName+"]"):
		p.notes = append(p.notes, CodexConfig+" already defines [mcp_servers."+ServerName+"] outside aayudha's block, so install leaves it: point its command at "+string(cmd)+" with args [\"mcp\"]")
		return nil
	case text == "":
		text = block
	default:
		text = strings.TrimRight(text, "\n") + "\n\n" + block
	}
	return p.write(CodexConfig, false, p.o.text([]byte(text)))
}

func (p *planner) agentsSections(lock Lock) []AgentsSection {
	var dirs []string
	for _, d := range lock.SkillDirs() {
		dirs = append(dirs, "`"+d+"/`")
	}
	where := "The skill copies in " + strings.Join(dirs, " and ")
	var names []string
	for _, h := range lock.Hosts {
		if hh, ok := verbs.FindHost(verbs.HostID(h)); ok {
			names = append(names, hh.Name)
		}
	}
	skills := fmt.Sprintf(`## Aayudha skills

This repository runs the aayudha verbs as host skills, installed by `+"`aayu install`"+` (skills version %s).

- %s are managed: do not edit them. `+"`aayu install --apply`"+` or `+"`aayu doctor --fix`"+`
  rewrites them from the binary, and `+"`.aayudha/lock.toml`"+` records their hashes.
- Inside a skill, the host's model reasons; every external write goes through an `+"`aayu`"+` command, which logs it.
- `+"`aayu doctor`"+` checks the tools, tokens, configuration, and these copies.
- Seat charters and the RACI arrive with WP5.1.`, lock.SkillsVersion, where)
	hostsSec := fmt.Sprintf(`## Aayudha hosts

Installed for: %s. The %s MCP server serves the verbs as tools (write verbs only with `+"`[mcp] expose_writes = true`"+`).
Host hooks run `+"`aayu hook <name>`"+` only: `+"`session-start`"+` reports skill copies out of sync with the lock,
`+"`pre-tool`"+` blocks agent tool calls while the `+"`[budget]`"+` breaker has paused agent lanes and blocks shell commands that
run a human-only verb, and `+"`status-line`"+` shows where the work stands.

%s`, strings.Join(names, ", "), "`"+ServerName+"`", wrapText("B"+strings.TrimPrefix(Routing, "aayudha: b"), 118))
	return []AgentsSection{{ID: "skills", Body: skills}, {ID: "hosts", Body: hostsSec}}
}

// mergeJSON reads a JSON object file (or starts an empty one), applies set, and
// plans the result with sorted keys and two-space indentation. A file that is
// not plain JSON (comments, trailing commas) is refused rather than rewritten.
func (p *planner) mergeJSON(rel string, home bool, set func(map[string]any)) error {
	old, err := p.read(rel, home)
	if err != nil {
		return err
	}
	m := map[string]any{}
	if len(bytes.TrimSpace(old)) > 0 {
		dec := json.NewDecoder(bytes.NewReader(old))
		dec.UseNumber()
		if err := dec.Decode(&m); err != nil {
			msg := fmt.Sprintf("%s: cannot merge into it (%v); fix the file or add the aayudha entries by hand", (Edit{Path: rel, Home: home}).Display(), err)
			if home {
				p.notes = append(p.notes, msg)
				return nil
			}
			return errors.New(msg)
		}
	}
	set(m)
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(m); err != nil {
		return err
	}
	return p.write(rel, home, p.o.text(buf.Bytes()))
}

// obj returns m[key] as an object, creating it when absent or of another type.
func obj(m map[string]any, key string) map[string]any {
	if v, ok := m[key].(map[string]any); ok {
		return v
	}
	v := map[string]any{}
	m[key] = v
	return v
}

func addString(v any, s string) []any {
	list, _ := v.([]any)
	for _, x := range list {
		if x == s {
			return list
		}
	}
	return append(list, s)
}

// PluginID is the aayudha plugin's ID in Claude Code and Codex: the plugin
// from this repository's marketplace, which is also named aayudha.
const PluginID = ServerName + "@" + ServerName

// The install notes for Cursor, whose plugin aayu cannot turn off per
// repository and which also reads .claude/skills (D-3.2b.g, D-3.2b.h).
const (
	cursorPlugin = "Cursor has no setting a repository can commit to turn a plugin off, so if you also use the aayudha Cursor plugin here, Cursor lists each aayudha skill and the MCP server twice: disable the plugin for this workspace in Cursor. aayu doctor sees a plugin only in ~/.cursor/plugins/local, not one installed from a marketplace"
	cursorBoth   = "Claude Code needs the copies in .claude/skills and Codex those in .agents/skills, and Cursor reads both (with Third-Party Imports, on by default), so Cursor lists each aayudha skill twice; turn off Third-Party Imports in Cursor's settings, or install for Claude Code or Codex, not both"
)

// setHooks puts aayudha's entry for each event into m's hooks, replacing the
// ones it wrote before.
func setHooks(m map[string]any, entries map[string]any) {
	hk := obj(m, "hooks")
	for ev, entry := range entries {
		hk[ev] = replaceHooks(hk[ev], entry.(map[string]any))
	}
}

// replaceHooks drops the entries aayudha wrote before (recognised by their
// commands) from a host's hook list and appends entry; the product's own hooks
// stay, in order.
func replaceHooks(v any, entry map[string]any) []any {
	list, _ := v.([]any)
	var out []any
	for _, e := range list {
		if !isOurs(e) {
			out = append(out, e)
		}
	}
	return append(out, entry)
}

func isOurs(e any) bool {
	m, ok := e.(map[string]any)
	if !ok {
		return false
	}
	if c, ok := m["command"].(string); ok {
		return ourHook.MatchString(c)
	}
	inner, ok := m["hooks"].([]any)
	if !ok || len(inner) == 0 {
		return false
	}
	for _, h := range inner {
		hm, _ := h.(map[string]any)
		c, _ := hm["command"].(string)
		if !ourHook.MatchString(c) {
			return false
		}
	}
	return true
}

// Apply performs edits. Home edits are applied only when home is true.
func Apply(o Options, edits []Edit, home bool) error {
	p := &planner{o: o}
	for _, e := range edits {
		if e.Home && !home {
			continue
		}
		fp := p.abs(e.Path, e.Home)
		switch e.Kind {
		case Unlink, Delete:
			if err := os.Remove(fp); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return err
			}
			removeEmptyParents(filepath.Dir(fp), o.Root)
		case Write:
			if err := os.MkdirAll(filepath.Dir(fp), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(fp, e.New, 0o644); err != nil {
				return err
			}
		}
	}
	return nil
}

// removeEmptyParents removes empty directories from dir up to, not including, root.
func removeEmptyParents(dir, root string) {
	for dir != root && strings.HasPrefix(dir, root) {
		if os.Remove(dir) != nil {
			return
		}
		dir = filepath.Dir(dir)
	}
}

// Render writes the plan as a reviewable diff: unified diffs for changed files,
// one line for new, deleted, and unlinked files.
func Render(edits []Edit, unified func(old, new string) string) string {
	var b strings.Builder
	for _, e := range edits {
		switch {
		case e.Kind == Unlink:
			fmt.Fprintf(&b, "unlink %s (a symlink; replaced by real copies, V2)\n", e.Display())
		case e.Kind == Delete:
			fmt.Fprintf(&b, "delete %s\n", e.Display())
		case e.Old == nil:
			fmt.Fprintf(&b, "create %s (%d lines)\n", e.Display(), bytes.Count(e.New, []byte("\n")))
		default:
			fmt.Fprintf(&b, "--- a/%s\n+++ b/%s\n%s", e.Display(), e.Display(), unified(string(lf(e.Old)), string(lf(e.New))))
		}
	}
	return b.String()
}

// Registration is the aayudha MCP server entry install wrote for one host.
type Registration struct {
	Host    verbs.HostID
	File    string // as shown to a human: repository-relative, or ~/ for home files
	Command string // "" when the entry is missing
	Err     error  // the file could not be read or parsed
}

// Registrations reads the aayudha MCP server entry of each host in the lock.
func Registrations(root, home string, lockHosts []string) []Registration {
	var out []Registration
	for _, h := range lockHosts {
		var rel, key string
		isHome := false
		switch verbs.HostID(h) {
		case verbs.HostClaude:
			rel, key = ClaudeMCP, "mcpServers"
		case verbs.HostCursor:
			rel, key = CursorMCP, "mcpServers"
		case verbs.HostAmp:
			rel, key, isHome = AmpSettings, "amp.mcpServers", true
		case verbs.HostCodex:
			out = append(out, codexRegistration(root))
			continue
		default:
			continue
		}
		r := Registration{Host: verbs.HostID(h), File: (Edit{Path: rel, Home: isHome}).Display()}
		base := root
		if isHome {
			base = home
		}
		b, err := os.ReadFile(filepath.Join(base, filepath.FromSlash(rel)))
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			r.Err = err
		} else if err == nil {
			var raw map[string]json.RawMessage
			var servers map[string]struct {
				Command string `json:"command"`
			}
			if err := json.Unmarshal(b, &raw); err != nil {
				r.Err = err
			} else if v := raw[key]; len(v) > 0 {
				if err := json.Unmarshal(v, &servers); err != nil {
					r.Err = err
				}
				r.Command = servers[ServerName].Command
			}
		}
		out = append(out, r)
	}
	return out
}

// codexRegistration reads aayudha's server from the project's .codex/config.toml.
func codexRegistration(root string) Registration {
	r := Registration{Host: verbs.HostCodex, File: CodexConfig}
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(CodexConfig)))
	if errors.Is(err, fs.ErrNotExist) {
		return r
	}
	if err != nil {
		r.Err = err
		return r
	}
	var cfg struct {
		Servers map[string]struct {
			Command string `toml:"command"`
		} `toml:"mcp_servers"`
	}
	if err := toml.Unmarshal(b, &cfg); err != nil {
		r.Err = err
		return r
	}
	r.Command = cfg.Servers[ServerName].Command
	return r
}

// SlashCommand is one host-neutral command source in hosts/commands/<dir>/<name>.md
// (Claude Code command format: description and argument-hint frontmatter; the
// host appends the user's arguments, so no body uses $ARGUMENTS, which Codex
// refuses). common/ holds home and next; <agent>/ holds the hand-off to that
// agent. The Claude Code, Codex, and Cursor plugin manifests list common/ and
// every agent's directory but their own; the Amp plugin gets the same commands
// from install.
type SlashCommand struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Hint        string `json:"hint,omitempty"`
	Body        string `json:"body"`
}

// SlashCommands reads hosts/commands/*/*.md from the embedded assets, by name,
// for host: a delegate command named after the host itself (/amp in Amp) is
// left out, since a host never delegates to itself. An empty host gets all.
func SlashCommands(assets fs.FS, host verbs.HostID) ([]SlashCommand, error) {
	files, err := fs.Glob(assets, "hosts/commands/*/*.md")
	if err != nil {
		return nil, err
	}
	out := []SlashCommand{}
	for _, f := range files {
		b, err := fs.ReadFile(assets, f)
		if err != nil {
			return nil, err
		}
		c := SlashCommand{Name: strings.TrimSuffix(path.Base(f), ".md")}
		if host != "" && path.Base(path.Dir(f)) == string(host) {
			continue
		}
		text := string(lf(b))
		if rest, ok := strings.CutPrefix(text, "---\n"); ok {
			if i := strings.Index(rest, "\n---\n"); i >= 0 {
				for _, l := range strings.Split(rest[:i], "\n") {
					k, v, _ := strings.Cut(l, ":")
					v = strings.Trim(strings.TrimSpace(v), `"'`)
					switch strings.TrimSpace(k) {
					case "description":
						c.Description = v
					case "argument-hint":
						c.Hint = v
					}
				}
				text = rest[i+5:]
			}
		}
		c.Body = strings.TrimSpace(text)
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}
