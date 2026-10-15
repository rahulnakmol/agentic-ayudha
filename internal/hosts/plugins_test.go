package hosts

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	aayudha "github.com/rahulnakmol/agentic-ayudha"
	"github.com/rahulnakmol/agentic-ayudha/internal/skills"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

func TestHumanOnlyCommand(t *testing.T) {
	tests := []struct {
		shell, cmd string
	}{
		{"aayu ship 4", "aayu ship 4"},
		{"cd repo && /usr/local/bin/aayu release 7 --to main", "aayu release 7 --to main"},
		{`"C:\Program Files\aayudha\aayudha.exe" decide 9 --accept B`, "aayu decide 9 --accept B"},
		{"aayu install --host claude --apply", "aayu install --host claude --apply"},
		{"aayu tell release --publish", "aayu tell release --publish"},
		{"aayu plan 5 --approve", "aayu plan 5 --approve"},
		{"aayu resume", "aayu resume"},
		{"aayu init --gates", "aayu init --gates"},
		// Agents may do these.
		{"aayu install --host claude", ""},
		{"aayu build 12", ""},
		{"aayu tell release --from notes.md", ""},
		{"aayu plan 5 --create", ""},
		{"echo aayushipment ship", ""},
		{"git commit -m 'aayu ship later'", ""},
		{"aayu status; aayu ship 4", "aayu ship 4"},
	}
	for _, tt := range tests {
		got, why := HumanOnlyCommand(tt.shell)
		if got != tt.cmd || (got == "") != (why == "") {
			t.Errorf("%q: got %q (%s), want %q", tt.shell, got, why, tt.cmd)
		}
	}
}

func TestPreToolBlocksHumanOnlyShell(t *testing.T) {
	ship := []byte(`{"tool_name":"Bash","tool_input":{"command":"aayu ship 4"}}`)
	for _, h := range []verbs.HostID{verbs.HostClaude, verbs.HostCodex, verbs.HostAmp} {
		r, _ := RunHook("pre-tool", h, "", filepath.Join(t.TempDir(), "none"), ship)
		if r.Code != 2 || !strings.Contains(r.Stderr, "run `aayu ship 4` in their own terminal") {
			t.Errorf("%s: %+v", h, r)
		}
	}
	r, _ := RunHook("pre-tool", verbs.HostCursor, "", filepath.Join(t.TempDir(), "none"), []byte(`{"command":"aayu release 3 --to rolling"}`))
	if r.Code != 2 || !strings.Contains(r.Stdout, `"permission":"deny"`) {
		t.Errorf("cursor: %+v", r)
	}
	if r, _ := RunHook("pre-tool", verbs.HostClaude, "", filepath.Join(t.TempDir(), "none"), []byte(`{"tool_name":"Bash","tool_input":{"command":"aayu build 4"}}`)); r.Code != 0 {
		t.Errorf("build was blocked: %+v", r)
	}
}

func TestCodexConfigKeepsTheProductsConfig(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".codex", "config.toml")
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	mine := "# my settings\nmodel = \"gpt-6.1-sol\"\n\n[mcp_servers.docs]\nurl = \"https://example.com/mcp\"\n"
	_ = os.WriteFile(path, []byte(mine), 0o644)
	o := Options{Root: root, Home: t.TempDir(), Hosts: []verbs.HostID{verbs.HostCodex}, Assets: fixtureAssets(t), Binary: "/opt/aayu", Version: "1", SkillsVersion: "0.0.0"}
	res, err := Plan(o)
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(o, res.Edits, false); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	if !strings.HasPrefix(string(b), mine+"\n# BEGIN aayudha") || !strings.Contains(string(b), "command = \"/opt/aayu\"") {
		t.Fatalf("config.toml:\n%s", b)
	}
	// A new binary path replaces only the block.
	o.Binary = "/usr/bin/aayu"
	res, _ = Plan(o)
	_ = Apply(o, res.Edits, false)
	b, _ = os.ReadFile(path)
	if strings.Count(string(b), "# BEGIN aayudha") != 1 || !strings.Contains(string(b), "/usr/bin/aayu") || !strings.HasPrefix(string(b), mine) {
		t.Fatalf("re-install:\n%s", b)
	}
	if r := codexRegistration(root); r.Command != "/usr/bin/aayu" || r.Err != nil {
		t.Errorf("registration %+v", r)
	}
	// The product's own aayudha server is left alone, with a note.
	_ = os.WriteFile(path, []byte("[mcp_servers.aayudha]\ncommand = \"aayu\"\n"), 0o644)
	res, _ = Plan(o)
	for _, e := range res.Edits {
		if e.Path == CodexConfig {
			t.Errorf("rewrote a product-owned server entry")
		}
	}
	if len(res.Notes) == 0 || !strings.Contains(res.Notes[0], "outside aayudha's block") {
		t.Errorf("notes %v", res.Notes)
	}
}

// TestPluginManifests: the marketplace plugin (Claude Code and Codex) carries
// this binary's skills version everywhere it calls aayu, its hooks call only
// `aayu hook <name>`, no command grants a human-only verb, and the Amp
// plugin offers no human-only verb.
func TestPluginManifests(t *testing.T) {
	var plugin struct {
		Name       string   `json:"name"`
		Version    string   `json:"version"`
		Commands   []string `json:"commands"`
		Hooks      string   `json:"hooks"`
		MCPServers map[string]struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"mcpServers"`
	}
	readJSON(t, "../../.claude-plugin/plugin.json", &plugin)
	var market struct {
		Name    string `json:"name"`
		Plugins []struct{ Name, Source string }
	}
	readJSON(t, "../../.claude-plugin/marketplace.json", &market)
	if plugin.Name != "aayudha" || plugin.Version != verbs.SkillVersion || len(market.Plugins) != 1 || market.Plugins[0].Name != plugin.Name || market.Plugins[0].Source != "./" {
		t.Fatalf("plugin %+v, marketplace %+v", plugin, market)
	}
	srv := plugin.MCPServers[ServerName]
	if srv.Command != "aayu" || !slices.Equal(srv.Args, []string{"mcp", "--skills", verbs.SkillVersion}) {
		t.Errorf("MCP server %+v", srv)
	}
	var hooks struct {
		Hooks map[string][]struct {
			Hooks []struct{ Type, Command string }
		}
	}
	readJSON(t, "../../"+plugin.Hooks, &hooks)
	n := 0
	for ev, groups := range hooks.Hooks {
		for _, g := range groups {
			for _, h := range g.Hooks {
				n++
				if h.Type != "command" || !skills.HookCommand.MatchString(h.Command) || !strings.HasSuffix(h.Command, " --skills "+verbs.SkillVersion) {
					t.Errorf("%s hook %q", ev, h.Command)
				}
			}
		}
	}
	if n != 2 {
		t.Errorf("%d hooks, want session-start and pre-tool", n)
	}
	cmds, err := SlashCommands(aayudha.Assets)
	if err != nil || !slices.ContainsFunc(cmds, func(c SlashCommand) bool { return c.Name == "next" }) {
		t.Fatalf("commands %v %v", cmds, err)
	}
	for _, c := range plugin.Commands {
		if c != "./hosts/commands/" {
			t.Errorf("command path %s: the shared sources live in hosts/commands/", c)
		}
	}
	tools := regexp.MustCompile(`(?m)^allowed-tools:(.*)$`)
	for _, c := range cmds {
		b, _ := os.ReadFile("../../hosts/commands/" + c.Name + ".md")
		for _, m := range tools.FindAllStringSubmatch(string(b), -1) {
			for _, call := range regexp.MustCompile(`Bash\(([^)]*)\)`).FindAllStringSubmatch(m[1], -1) {
				if cmd, _ := HumanOnlyCommand(strings.TrimSuffix(call[1], ":*")); cmd != "" {
					t.Errorf("%s grants %s, a human-only command", c.Name, call[1])
				}
			}
		}
	}
	amp, _ := os.ReadFile("../../hosts/amp/aayudha.ts")
	for _, m := range regexp.MustCompile(`verb: '([a-z]+)'`).FindAllStringSubmatch(string(amp), -1) {
		if c, ok := verbs.Find(m[1]); !ok || c.Exposure == verbs.ExposeNever {
			t.Errorf("the Amp plugin offers %s", m[1])
		}
	}
}

func readJSON(t *testing.T, path string, v any) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}

// TestHostPluginsLoad is the per-host smoke test (WP3.2b): each host's own CLI
// loads the plugin and lists its commands, where it has a headless way to.
// It skips a host whose CLI is not on PATH (CI has none); the run that
// produced the Appendix B evidence used Claude Code 2.1.300, Codex 0.162.1,
// and Amp's `amp plugins list`.
func TestHostPluginsLoad(t *testing.T) {
	repo, _ := filepath.Abs("../..")
	run := func(t *testing.T, env []string, dir, name string, args ...string) string {
		t.Helper()
		cmd := exec.Command(name, args...)
		cmd.Dir, cmd.Env = dir, append(os.Environ(), env...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s %v: %v\n%s", name, args, err, out)
		}
		return string(out)
	}
	t.Run("claude", func(t *testing.T) {
		if _, err := exec.LookPath("claude"); err != nil {
			t.Skip("claude is not on PATH")
		}
		env := []string{"HOME=" + t.TempDir()}
		if out := run(t, env, repo, "claude", "plugin", "validate", "."); !strings.Contains(out, "Validation passed") {
			t.Fatal(out)
		}
		run(t, env, repo, "claude", "plugin", "marketplace", "add", "./")
		run(t, env, repo, "claude", "plugin", "install", "aayudha@aayudha")
		out := run(t, env, repo, "claude", "plugin", "details", "aayudha")
		for _, w := range []string{"build", "grill", "ready", "home", "next", "SessionStart, PreToolUse", "aayudha"} {
			if !strings.Contains(out, w) {
				t.Errorf("details lack %q:\n%s", w, out)
			}
		}
	})
	t.Run("codex", func(t *testing.T) {
		if _, err := exec.LookPath("codex"); err != nil {
			t.Skip("codex is not on PATH")
		}
		home := t.TempDir()
		_ = os.MkdirAll(filepath.Join(home, ".codex"), 0o755)
		env := []string{"HOME=" + home, "CODEX_HOME=" + filepath.Join(home, ".codex")}
		run(t, env, repo, "codex", "plugin", "marketplace", "add", "./")
		run(t, env, repo, "codex", "plugin", "add", "aayudha@aayudha")
		if out := run(t, env, repo, "codex", "mcp", "list"); !strings.Contains(out, "mcp --skills "+verbs.SkillVersion) {
			t.Errorf("codex mcp list:\n%s", out)
		}
		out := run(t, env, t.TempDir(), "codex", "debug", "prompt-input", "hi")
		for _, w := range []string{"aayudha:build", "aayudha:ready", "aayudha:grill", "aayudha:source-command-next"} {
			if !strings.Contains(out, w) {
				t.Errorf("codex prompt input lacks %s", w)
			}
		}
	})
	t.Run("amp", func(t *testing.T) {
		if _, err := exec.LookPath("amp"); err != nil {
			t.Skip("amp is not on PATH")
		}
		dir := t.TempDir()
		tmpl, _ := os.ReadFile(filepath.Join(repo, "hosts", "amp", "aayudha.ts"))
		cmds, _ := SlashCommands(aayudha.Assets)
		cj, _ := json.Marshal(cmds)
		plugin := strings.NewReplacer("__AAYU__", `"aayu"`, "__VERSION__", `"0.0.0-dev"`, "__COMMANDS__", string(cj)).Replace(string(tmpl))
		_ = os.MkdirAll(filepath.Join(dir, ".amp", "plugins"), 0o755)
		_ = os.WriteFile(filepath.Join(dir, ".amp", "plugins", "aayudha.ts"), []byte(plugin), 0o644)
		out := run(t, nil, dir, "amp", "plugins", "list")
		for _, w := range []string{".amp/plugins/aayudha.ts active", "events: session.start, tool.call", "aayudha: Next step", "aayudha: Build a Ready story"} {
			if !strings.Contains(out, w) {
				t.Errorf("amp plugins list lacks %q", w)
			}
		}
	})
}
