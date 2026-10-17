package hosts

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

// CodexPluginHooks is the hook file the Codex plugin manifest
// (.codex-plugin/plugin.json) names, relative to the plugin root.
const CodexPluginHooks = "hosts/codex/hooks.json"

// CodexHome is Codex's configuration directory: $CODEX_HOME, else ~/.codex.
func CodexHome(home string, getenv func(string) string) string {
	if d := getenv("CODEX_HOME"); d != "" {
		return d
	}
	return filepath.Join(home, ".codex")
}

// RepoHooks reports whether `aayu install` registered the repository's own
// hooks for host (the commands HookCommand writes, in the host's project hook
// file).
func RepoHooks(root string, host verbs.HostID) bool {
	file := map[verbs.HostID]string{verbs.HostClaude: ClaudeSettings, verbs.HostCodex: CodexHooks, verbs.HostCursor: CursorHooks}[host]
	if file == "" {
		return false
	}
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
	return err == nil && bytes.Contains(b, []byte(" hook pre-tool --host "+string(host)+`"`))
}

// PluginEnabled reports whether the aayudha plugin is enabled for host:
// Claude Code's enabledPlugins in the user's or the project's settings, a
// [plugins."aayudha@…"] table in Codex's config.toml, or the aayudha plugin in
// Cursor's local plugin folder (~/.cursor/plugins/local; Cursor keeps its
// marketplace installs with the account, where aayu cannot read them). Amp has
// no aayudha plugin but the one install writes: false.
func PluginEnabled(host verbs.HostID, root, home, codexHome string) bool {
	ours := func(k string) bool { return strings.HasPrefix(k, ServerName+"@") }
	switch host {
	case verbs.HostClaude:
		files := []string{filepath.Join(root, ".claude", "settings.json"), filepath.Join(root, ".claude", "settings.local.json")}
		if home != "" {
			files = append(files, filepath.Join(home, ".claude", "settings.json"))
		}
		for _, f := range files {
			var s struct {
				Enabled map[string]bool `json:"enabledPlugins"`
			}
			if b, err := os.ReadFile(f); err == nil && json.Unmarshal(b, &s) == nil {
				for k, on := range s.Enabled {
					if on && ours(k) {
						return true
					}
				}
			}
		}
	case verbs.HostCursor:
		if home == "" {
			return false
		}
		manifests, _ := filepath.Glob(filepath.Join(home, ".cursor", "plugins", "local", "*", ".cursor-plugin", "plugin.json"))
		for _, f := range manifests {
			var m struct {
				Name string `json:"name"`
			}
			if b, err := os.ReadFile(f); err == nil && json.Unmarshal(b, &m) == nil && m.Name == ServerName {
				return true
			}
		}
	case verbs.HostCodex:
		cfg, _ := readCodexConfig(codexHome)
		for k, p := range cfg.Plugins {
			if ours(k) && (p.Enabled == nil || *p.Enabled) {
				return true
			}
		}
	}
	return false
}

type codexConfig struct {
	Plugins map[string]struct {
		Enabled *bool `toml:"enabled"`
	} `toml:"plugins"`
	Projects map[string]struct {
		TrustLevel string `toml:"trust_level"`
	} `toml:"projects"`
	Hooks struct {
		State map[string]struct {
			Enabled     *bool  `toml:"enabled"`
			TrustedHash string `toml:"trusted_hash"`
		} `toml:"state"`
	} `toml:"hooks"`
}

func readCodexConfig(codexHome string) (codexConfig, error) {
	var c codexConfig
	if codexHome == "" || codexHome == ".codex" { // no home: nothing to read
		return c, nil
	}
	b, err := os.ReadFile(filepath.Join(codexHome, "config.toml"))
	if errors.Is(err, fs.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	if err := toml.Unmarshal(b, &c); err != nil {
		return c, fmt.Errorf("%s: %w", filepath.Join(codexHome, "config.toml"), err)
	}
	return c, nil
}

// CodexHookTrust checks aayudha's Codex hooks against the trust Codex records
// in config.toml (WP3.2b, D-3.2b.a). Codex runs a non-managed hook only after
// the person trusts its exact definition in /hooks, and it skips the
// repository's .codex/ layer in a project it does not trust, so until then no
// pre-tool hook stops an agent from running a human-only verb. It returns the
// number of aayudha hooks found (the repository's from `aayu install`, and the
// enabled plugin's) and one line per hook Codex will skip.
func CodexHookTrust(root, codexHome string) (found int, problems []string, err error) {
	cfg, err := readCodexConfig(codexHome)
	if err != nil {
		return 0, nil, err
	}
	type source struct{ label, file, key string }
	var sources []source
	if root != "" && RepoHooks(root, verbs.HostCodex) {
		file := filepath.Join(root, filepath.FromSlash(CodexHooks))
		sources = append(sources, source{CodexHooks, file, file})
		trusted := false
		for _, r := range []string{root, resolved(root)} {
			trusted = trusted || cfg.Projects[r].TrustLevel == "trusted"
		}
		if !trusted {
			problems = append(problems, "Codex does not trust this project, so it loads nothing from .codex/ (hooks or MCP): trust it when Codex asks, or set [projects."+fmt.Sprintf("%q", root)+"] trust_level = \"trusted\" in "+filepath.Join(codexHome, "config.toml"))
		}
	}
	ids := make([]string, 0, len(cfg.Plugins))
	for id, p := range cfg.Plugins {
		if strings.HasPrefix(id, ServerName+"@") && (p.Enabled == nil || *p.Enabled) {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	for _, id := range ids {
		mkt := strings.TrimPrefix(id, ServerName+"@")
		files, _ := filepath.Glob(filepath.Join(codexHome, "plugins", "cache", mkt, ServerName, "*", filepath.FromSlash(CodexPluginHooks)))
		if len(files) == 0 {
			continue
		}
		sort.Slice(files, func(i, j int) bool { return modTime(files[i]) > modTime(files[j]) })
		sources = append(sources, source{"plugin " + id, files[0], id + ":" + CodexPluginHooks})
	}
	for _, s := range sources {
		hooks, err := codexHooks(s.file)
		if err != nil {
			return found, problems, err
		}
		for _, h := range hooks {
			found++
			keys := []string{s.key + h.suffix}
			if s.file == s.key { // a project file is keyed by its absolute path
				keys = append(keys, resolved(s.file)+h.suffix)
			}
			state, ok := cfg.Hooks.State[keys[0]]
			if !ok && len(keys) > 1 {
				state = cfg.Hooks.State[keys[1]]
			}
			switch {
			case state.Enabled != nil && !*state.Enabled:
				problems = append(problems, fmt.Sprintf("%s: the %s hook is disabled in /hooks", s.label, h.event))
			case state.TrustedHash == "":
				problems = append(problems, fmt.Sprintf("%s: the %s hook is not trusted yet; trust it in Codex's /hooks", s.label, h.event))
			case state.TrustedHash != h.hash:
				problems = append(problems, fmt.Sprintf("%s: the %s hook changed since it was trusted (an update); trust it again in Codex's /hooks", s.label, h.event))
			}
		}
	}
	return found, problems, nil
}

type codexHook struct{ event, suffix, hash string }

var aayuHookCommand = regexp.MustCompile(`aayu(?:dha)?(?:\.exe)?"? hook [a-z][a-z0-9-]*`)

// codexHooks reads a Codex hooks.json and returns aayudha's command hooks with
// their trust-state key suffix (":<event>:<group>:<handler>") and the hash
// Codex trusts: SHA-256 over the sorted, compact JSON of the event, the
// group's matcher, and the one handler with Codex's defaults filled in
// (codex-rs hooks/src/engine/discovery.rs, hook_hash; checked against the
// hashes Codex 0.162.1 reports in hooks/list).
func codexHooks(file string) ([]codexHook, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var f struct {
		Hooks map[string][]struct {
			Matcher *string          `json:"matcher"`
			Hooks   []map[string]any `json:"hooks"`
		} `json:"hooks"`
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	var out []codexHook
	for _, event := range sortedKeys(f.Hooks) {
		snake := snakeCase(event)
		for g, group := range f.Hooks[event] {
			for i, h := range group.Hooks {
				cmd, _ := h["command"].(string)
				if h["type"] != "command" || !aayuHookCommand.MatchString(cmd) {
					continue
				}
				handler := map[string]any{"type": "command", "command": cmd, "timeout": json.Number("600"), "async": false}
				if t, ok := h["timeout"].(json.Number); ok {
					handler["timeout"] = t
				}
				if a, ok := h["async"].(bool); ok {
					handler["async"] = a
				}
				if m, ok := h["statusMessage"].(string); ok {
					handler["statusMessage"] = m
				}
				identity := map[string]any{"event_name": snake, "hooks": []any{handler}}
				if group.Matcher != nil {
					identity["matcher"] = *group.Matcher
				}
				var buf bytes.Buffer
				enc := json.NewEncoder(&buf)
				enc.SetEscapeHTML(false)
				if err := enc.Encode(identity); err != nil {
					return nil, err
				}
				sum := sha256.Sum256(bytes.TrimSuffix(buf.Bytes(), []byte("\n")))
				out = append(out, codexHook{event: event, suffix: fmt.Sprintf(":%s:%d:%d", snake, g, i), hash: "sha256:" + hex.EncodeToString(sum[:])})
			}
		}
	}
	return out, nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// snakeCase turns a Codex event name (PreToolUse) into its key label (pre_tool_use).
func snakeCase(s string) string {
	var b strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				b.WriteByte('_')
			}
			r += 'a' - 'A'
		}
		b.WriteRune(r)
	}
	return b.String()
}

func resolved(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return p
}

func modTime(p string) int64 {
	if fi, err := os.Stat(p); err == nil {
		return fi.ModTime().UnixNano()
	}
	return 0
}

// pluginReads are the project skill directories in which a host with the
// aayudha plugin enabled would find aayudha's copies beside the plugin's own
// skills: Cursor also reads Claude Code's. Amp keeps the first skill of each
// name (ampcode.com/docs/customize/skills), so it never lists one twice.
var pluginReads = map[verbs.HostID][]string{
	verbs.HostClaude: {ClaudeSkills},
	verbs.HostCodex:  {AgentsSkills},
	verbs.HostCursor: {AgentsSkills, ClaudeSkills},
}

// SkillDoubles reports each host whose aayudha plugin is enabled while a
// project directory it reads still holds aayudha's skill copies, so the host
// lists every skill twice, and each host install left to a plugin that is no
// longer enabled, so it has none (D-3.2b.f). One line per problem, with the fix.
func SkillDoubles(root, home, codexHome string, lock Lock) []string {
	var out []string
	for _, h := range AllHosts() {
		hh, _ := verbs.FindHost(h)
		if !PluginEnabled(h, root, home, codexHome) {
			if slices.Contains(lock.PluginHosts, string(h)) && !slices.Contains(lock.SkillDirs(), SkillDir(h)) {
				out = append(out, fmt.Sprintf("%s: install left the skills to the aayudha plugin, which is no longer enabled, so %s has none; aayu install --apply copies them", hh.Name, hh.Name))
			}
			continue
		}
		for _, d := range pluginReads[h] {
			has := false
			for n := range lock.Skills {
				if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(d), n, "SKILL.md")); err == nil {
					has = true
					break
				}
			}
			if !has {
				continue
			}
			// The installed hosts that read d from install's copies and have no plugin.
			var others []string
			for _, o := range lock.Hosts {
				if SkillDir(verbs.HostID(o)) == d && !PluginEnabled(verbs.HostID(o), root, home, codexHome) {
					oh, _ := verbs.FindHost(verbs.HostID(o))
					others = append(others, oh.Name)
				}
			}
			fix := "aayu install --apply removes the copies"
			if len(others) > 0 {
				fix = "the copies stay for " + strings.Join(others, ", ") + ": add the plugin there too (then aayu install --apply), or disable it for " + hh.Name
			}
			out = append(out, fmt.Sprintf("%s: the aayudha plugin and the copies in %s both give it the aayudha skills, so it lists each twice; %s", hh.Name, d, fix))
		}
	}
	return out
}
