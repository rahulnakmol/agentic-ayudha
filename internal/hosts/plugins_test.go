package hosts

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
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
	// The delegate commands' calls pass on every host.
	for _, cmd := range []string{
		`aayu build 4 --delegate cursor --json`, `aayu build 4 --delegate codex --local --json`, `aayu ready 4 --check`,
		`aayu grill story --assess --idea "make the CLI simple" --json`, `aayu status --work`,
	} {
		ev, _ := json.Marshal(map[string]any{"tool_name": "Bash", "tool_input": map[string]string{"command": cmd}, "command": cmd})
		for _, h := range []verbs.HostID{verbs.HostClaude, verbs.HostCodex, verbs.HostCursor, verbs.HostAmp} {
			if r, _ := RunHook("pre-tool", h, "", filepath.Join(t.TempDir(), "none"), ev); r.Code != 0 {
				t.Errorf("%s: %s was blocked: %+v", h, cmd, r)
			}
		}
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

// TestPluginManifests: the Claude Code, Codex, and Cursor plugin manifests
// carry this binary's skills version everywhere they call aayu, their hooks
// call only `aayu hook <name>` for their host, each lists every command source
// in hosts/commands/ except the one that would delegate to the host itself,
// no command grants a human-only verb or uses what Codex refuses in a command
// ($ARGUMENTS, @mentions), and the Amp plugin offers no human-only verb.
func TestPluginManifests(t *testing.T) {
	all, err := SlashCommands(aayudha.Assets, "")
	if err != nil || !slices.ContainsFunc(all, func(c SlashCommand) bool { return c.Name == "next" }) {
		t.Fatalf("commands %v %v", all, err)
	}
	for _, m := range []struct {
		host             verbs.HostID
		manifest, market string
		hooks            string
	}{
		{verbs.HostClaude, ".claude-plugin/plugin.json", ".claude-plugin/marketplace.json", "./hosts/claude/hooks.json"},
		{verbs.HostCodex, ".codex-plugin/plugin.json", ".claude-plugin/marketplace.json", "./hosts/codex/hooks.json"},
		{verbs.HostCursor, ".cursor-plugin/plugin.json", ".cursor-plugin/marketplace.json", "./hosts/cursor/hooks.json"},
	} {
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
		readJSON(t, "../../"+m.manifest, &plugin)
		var market struct {
			Name    string `json:"name"`
			Plugins []struct{ Name, Source string }
		}
		readJSON(t, "../../"+m.market, &market)
		if plugin.Name != "aayudha" || plugin.Version != verbs.SkillVersion || len(market.Plugins) != 1 || market.Plugins[0].Name != plugin.Name || market.Plugins[0].Source != "./" {
			t.Fatalf("%s: plugin %+v, marketplace %+v", m.host, plugin, market)
		}
		srv := plugin.MCPServers[ServerName]
		if srv.Command != "aayu" || !slices.Equal(srv.Args, []string{"mcp", "--skills", verbs.SkillVersion}) {
			t.Errorf("%s: MCP server %+v", m.host, srv)
		}
		want := []string{"./hosts/commands/common/"}
		for _, h := range []verbs.HostID{verbs.HostAmp, verbs.HostClaude, verbs.HostCodex, verbs.HostCursor} {
			if h != m.host {
				want = append(want, "./hosts/commands/"+string(h)+"/")
			}
		}
		if !slices.Equal(plugin.Commands, want) {
			t.Errorf("%s: commands %v, want %v (every source but /%s)", m.host, plugin.Commands, want, m.host)
		}
		if plugin.Hooks != m.hooks {
			t.Fatalf("%s: hooks %s", m.host, plugin.Hooks)
		}
		var hooks struct {
			Hooks map[string]json.RawMessage
		}
		readJSON(t, "../../"+plugin.Hooks, &hooks)
		cmds := regexp.MustCompile(`"command":\s*"([^"]*)"`)
		n := 0
		for ev, raw := range hooks.Hooks {
			for _, c := range cmds.FindAllStringSubmatch(string(raw), -1) {
				n++
				host := m.host != verbs.HostClaude // the Claude hooks name no host: aayu detects it
				if !skills.HookCommand.MatchString(c[1]) || !strings.HasSuffix(c[1], " --skills "+verbs.SkillVersion) || host != strings.Contains(c[1], " --host "+string(m.host)+" ") {
					t.Errorf("%s: %s hook %q", m.host, ev, c[1])
				}
			}
		}
		if n < 2 {
			t.Errorf("%s: %d hooks, want session-start and pre-tool", m.host, n)
		}
	}
	codexRefuses := regexp.MustCompile(`\$ARGUMENTS|\$[0-9]|\{\{|!` + "`" + `|(^|\s)@[A-Za-z]`)
	for _, c := range all {
		if codexRefuses.MatchString(c.Body) {
			t.Errorf("%s: Codex drops a command whose body has $ARGUMENTS, $n, {{…}}, !`…`, or an @mention", c.Name)
		}
	}
	cmds := all
	tools := regexp.MustCompile(`(?m)^allowed-tools:(.*)$`)
	for _, c := range cmds {
		f, _ := filepath.Glob("../../hosts/commands/*/" + c.Name + ".md")
		b, _ := os.ReadFile(f[0])
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
		// Commands appear in the skills inventory: the other agents, never Claude itself.
		inv := regexp.MustCompile(`(?m)^\s*Skills \(\d+\)\s+(.*)$`).FindStringSubmatch(out)
		if inv == nil {
			t.Fatalf("no skills inventory:\n%s", out)
		}
		names := strings.Split(inv[1], ", ")
		for _, w := range []string{"amp", "codex", "cursor"} {
			if !slices.Contains(names, w) {
				t.Errorf("Claude Code lacks /aayudha:%s: %v", w, names)
			}
		}
		if slices.Contains(names, "claude") {
			t.Errorf("Claude Code offers /aayudha:claude, a hand-off to itself")
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
		for _, w := range []string{"aayudha:build", "aayudha:ready", "aayudha:grill", "aayudha:source-command-next",
			"aayudha:source-command-claude", "aayudha:source-command-cursor", "aayudha:source-command-amp"} {
			if !strings.Contains(out, w) {
				t.Errorf("codex prompt input lacks %s", w)
			}
		}
		if strings.Contains(out, "aayudha:source-command-codex") {
			t.Errorf("Codex offers a hand-off to itself")
		}
		// The model is offered the aayudha MCP tools (D-3.2b.c, megaman
		// evidence D-5.5c.12): a fake Responses endpoint records the request
		// Codex sends, with the plugin's server running this binary.
		bin := t.TempDir()
		if out, err := exec.Command("go", "build", "-o", filepath.Join(bin, "aayu"), "../../cmd/aayudha").CombinedOutput(); err != nil {
			t.Fatalf("build aayu: %v\n%s", err, out)
		}
		var body []byte
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if body == nil {
				body, _ = io.ReadAll(r.Body)
			}
			w.Header().Set("Content-Type", "text/event-stream")
			for _, e := range []string{`{"type":"response.created","response":{"id":"r"}}`,
				`{"type":"response.output_item.done","item":{"type":"message","role":"assistant","id":"m","content":[{"type":"output_text","text":"ok"}]}}`,
				`{"type":"response.completed","response":{"id":"r","usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}`} {
				fmt.Fprintf(w, "event: %s\ndata: %s\n\n", regexp.MustCompile(`"type":"([^"]+)"`).FindStringSubmatch(e)[1], e)
			}
		}))
		defer srv.Close()
		cfg := filepath.Join(home, ".codex", "config.toml")
		old, _ := os.ReadFile(cfg)
		_ = os.WriteFile(cfg, []byte("model = \"gpt-5\"\nmodel_provider = \"fake\"\n"+string(old)+
			"\n[model_providers.fake]\nname = \"fake\"\nbase_url = \""+srv.URL+"/v1\"\nwire_api = \"responses\"\nenv_key = \"FAKE_KEY\"\n"), 0o644)
		env = append(env, "FAKE_KEY=x", "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
		run(t, env, t.TempDir(), "codex", "exec", "--skip-git-repo-check", "hi")
		var req struct {
			Tools []struct {
				Type, Name string
				Tools      []struct{ Name string }
			}
		}
		_ = json.Unmarshal(body, &req)
		var offered []string
		for _, tool := range req.Tools {
			if tool.Name == "mcp__"+ServerName {
				for _, f := range tool.Tools {
					offered = append(offered, f.Name)
				}
			}
		}
		for _, w := range []string{"status", "doctor", "grill", "ready"} {
			if !slices.Contains(offered, w) {
				t.Errorf("Codex offered the model aayudha tools %v, without %s", offered, w)
			}
		}
	})
	t.Run("amp", func(t *testing.T) {
		if _, err := exec.LookPath("amp"); err != nil {
			t.Skip("amp is not on PATH")
		}
		dir := t.TempDir()
		tmpl, _ := os.ReadFile(filepath.Join(repo, "hosts", "amp", "aayudha.ts"))
		cmds, _ := SlashCommands(aayudha.Assets, verbs.HostAmp)
		cj, _ := json.Marshal(cmds)
		plugin := strings.NewReplacer("__AAYU__", `"aayu"`, "__VERSION__", `"0.0.0-dev"`, "__COMMANDS__", string(cj)).Replace(string(tmpl))
		_ = os.MkdirAll(filepath.Join(dir, ".amp", "plugins"), 0o755)
		_ = os.WriteFile(filepath.Join(dir, ".amp", "plugins", "aayudha.ts"), []byte(plugin), 0o644)
		out := run(t, nil, dir, "amp", "plugins", "list")
		for _, w := range []string{".amp/plugins/aayudha.ts active", "events: session.start, tool.call", "aayudha: Next step", "aayudha: Build a Ready story",
			"Codex cloud agent", "Cursor's cloud agent", "Claude Code on the web"} {
			if !strings.Contains(out, w) {
				t.Errorf("amp plugins list lacks %q:\n%s", w, out)
			}
		}
		if strings.Contains(out, "an Amp thread in an orb") {
			t.Errorf("Amp offers a hand-off to itself")
		}
	})
}

// Codex runs a plugin's or a project's hooks only after the person trusts
// them (D-3.2b.a, megaman evidence D-5.5c.12): CodexHookTrust reads the trust
// Codex records and says which aayudha hooks it skips. The hashes are the
// ones Codex 0.162.1 reported in its app-server hooks/list for these files.
func TestCodexHookTrust(t *testing.T) {
	root, codexHome := t.TempDir(), t.TempDir()
	repoHooks := filepath.Join(root, ".codex", "hooks.json")
	_ = os.MkdirAll(filepath.Dir(repoHooks), 0o755)
	_ = os.WriteFile(repoHooks, []byte(`{"hooks": {"PreToolUse": [{"hooks": [{"command": "\"/tmp/cxhome/aayu\" hook pre-tool --host codex", "type": "command"}]}],
"SessionStart": [{"hooks": [{"command": "\"/tmp/cxhome/aayu\" hook session-start --host codex", "type": "command"}]}]}}`), 0o644)
	cache := filepath.Join(codexHome, "plugins", "cache", "aayudha", "aayudha", "0.0.0", "hosts", "codex")
	_ = os.MkdirAll(cache, 0o755)
	plugin, _ := os.ReadFile("../../hosts/codex/hooks.json")
	_ = os.WriteFile(filepath.Join(cache, "hooks.json"), plugin, 0o644)
	config := func(s string) { _ = os.WriteFile(filepath.Join(codexHome, "config.toml"), []byte(s), 0o644) }

	config("[plugins.\"aayudha@aayudha\"]\nenabled = true\n")
	found, problems, err := CodexHookTrust(root, codexHome)
	if err != nil || found != 4 || len(problems) != 5 || !strings.Contains(problems[0], "does not trust this project") {
		t.Fatalf("nothing trusted: %d %q %v", found, problems, err)
	}
	// The repository's PreToolUse and the plugin's PreToolUse are trusted; the
	// plugin's SessionStart was trusted at another definition (an update).
	config(fmt.Sprintf(`[plugins."aayudha@aayudha"]
enabled = true
[projects.%q]
trust_level = "trusted"
[hooks.state.%q]
trusted_hash = "sha256:1218764140f472694d705f891e417ee0dd6161635a189e9531bf3bd4dc4ebde8"
[hooks.state."aayudha@aayudha:hosts/codex/hooks.json:pre_tool_use:0:0"]
trusted_hash = "sha256:611739dac789e7fbad81a394975c225003db4aabf33a3b1912c2f52229f89e97"
[hooks.state."aayudha@aayudha:hosts/codex/hooks.json:session_start:0:0"]
trusted_hash = "sha256:0000"
`, root, repoHooks+":pre_tool_use:0:0"))
	_, problems, _ = CodexHookTrust(root, codexHome)
	want := []string{
		".codex/hooks.json: the SessionStart hook is not trusted yet; trust it in Codex's /hooks",
		"plugin aayudha@aayudha: the SessionStart hook changed since it was trusted (an update); trust it again in Codex's /hooks",
	}
	if !slices.Equal(problems, want) {
		t.Errorf("partly trusted:\n got %q\nwant %q", problems, want)
	}
	// No Codex hooks of aayudha's: nothing to check.
	if found, _, _ := CodexHookTrust(t.TempDir(), t.TempDir()); found != 0 {
		t.Errorf("found %d hooks in an empty setup", found)
	}
}

// With the aayudha plugin enabled for a host, install registers no hooks or
// MCP server of its own there and removes the ones it wrote before, so the
// host runs each hook and lists the server once (D-3.2b.d, megaman evidence
// D-5.5c.12 defect 7).
func TestInstallLeavesHooksAndMCPToThePlugin(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	mine := `{"hooks": {"PreToolUse": [{"matcher": "Bash", "hooks": [{"type": "command", "command": "./mine.sh"}]}]}}`
	_ = os.MkdirAll(filepath.Join(root, ".claude"), 0o755)
	_ = os.WriteFile(filepath.Join(root, ".claude", "settings.json"), []byte(mine), 0o644)
	o := opts(t, root, home, false, verbs.HostClaude, verbs.HostCodex)
	res, err := Plan(o)
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(o, res.Edits, false); err != nil {
		t.Fatal(err)
	}
	if !RepoHooks(root, verbs.HostClaude) || !RepoHooks(root, verbs.HostCodex) {
		t.Fatal("install without the plugin wrote no repository hooks")
	}
	// The person enables the plugin in both hosts, then installs again.
	_ = os.MkdirAll(filepath.Join(home, ".claude"), 0o755)
	_ = os.WriteFile(filepath.Join(home, ".claude", "settings.json"), []byte(`{"enabledPlugins": {"aayudha@aayudha": true}}`), 0o644)
	_ = os.MkdirAll(filepath.Join(home, ".codex"), 0o755)
	_ = os.WriteFile(filepath.Join(home, ".codex", "config.toml"), []byte("[plugins.\"aayudha@aayudha\"]\nenabled = true\n"), 0o644)
	if res, err = Plan(o); err != nil {
		t.Fatal(err)
	}
	if err := Apply(o, res.Edits, false); err != nil {
		t.Fatal(err)
	}
	if RepoHooks(root, verbs.HostClaude) || RepoHooks(root, verbs.HostCodex) {
		t.Error("repository hooks remain beside the plugin's")
	}
	for _, f := range []string{ClaudeMCP, CodexConfig, CodexHooks} {
		if _, err := os.Stat(filepath.Join(root, f)); err == nil {
			t.Errorf("%s remains beside the plugin", f)
		}
	}
	b, _ := os.ReadFile(filepath.Join(root, ".claude", "settings.json"))
	if !strings.Contains(string(b), "./mine.sh") || strings.Contains(string(b), "enabledMcpjsonServers") || !strings.Contains(string(b), "hook status-line --host claude") {
		t.Errorf("settings.json: the product's hook and the status line stay, aayudha's server approval goes:\n%s", b)
	}
	if n := strings.Count(strings.Join(res.Notes, "\n"), "the aayudha plugin is enabled"); n != 2 {
		t.Errorf("notes %q", res.Notes)
	}
}
