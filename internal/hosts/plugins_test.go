package hosts

import (
	"bytes"
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
		{"cd repo && /usr/local/bin/aayu ship 7 --solo-bypass", "aayu ship 7 --solo-bypass"},
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
		{"aayu release 7 --to main", ""}, // an agent promotes behind the gate (Story #430)
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
	r, _ := RunHook("pre-tool", verbs.HostCursor, "", filepath.Join(t.TempDir(), "none"), []byte(`{"command":"aayu decide 3 --accept A"}`))
	if r.Code != 2 || !strings.Contains(r.Stdout, `"permission":"deny"`) {
		t.Errorf("cursor: %+v", r)
	}
	if r, _ := RunHook("pre-tool", verbs.HostClaude, "", filepath.Join(t.TempDir(), "none"), []byte(`{"tool_name":"Bash","tool_input":{"command":"aayu build 4"}}`)); r.Code != 0 {
		t.Errorf("build was blocked: %+v", r)
	}
	// An agent never writes the stable-release request that `release --to main`
	// needs (Story #430), with whatever tool posts it; reading it, or the word in a
	// commit, passes.
	for _, tt := range []struct {
		cmd     string
		blocked bool
	}{
		{`gh issue comment 268 --body "Stable release: v0.3.0"`, true},
		{`gh api repos/o/r/issues/268/comments -f body='stable release: v0.3.0'`, true},
		{`curl -X POST -d '{"body":"Stable release: v0.3.0"}' https://api.github.com/repos/o/r/issues/268/comments`, true},
		{`aayu release 268 --to main`, false},
		{`gh issue view 268 --comments`, false},
		{`git commit -m "docs: the Stable release: request form"`, false},
	} {
		ev, _ := json.Marshal(map[string]any{"tool_name": "Bash", "tool_input": map[string]string{"command": tt.cmd}, "command": tt.cmd})
		for _, h := range []verbs.HostID{verbs.HostClaude, verbs.HostCodex, verbs.HostCursor} {
			r, _ := RunHook("pre-tool", h, "", filepath.Join(t.TempDir(), "none"), ev)
			if blocked := r.Code == 2; blocked != tt.blocked || blocked && !strings.Contains(r.Stderr+r.Stdout, "Ask the person to comment `Stable release: <version>`") {
				t.Errorf("%s: %s: %+v", h, tt.cmd, r)
			}
		}
	}
	// The delegate commands' calls, and counsel (Story #478), pass on every host.
	for _, cmd := range []string{
		`aayu build 4 --delegate cursor --json`, `aayu build 4 --delegate codex --local --json`, `aayu ready 4 --check`,
		`aayu grill story --assess --idea "make the CLI simple" --json`, `aayu status --work`,
		`aayu counsel "should we split the survey?"`, `aayu counsel --pr 4 --draft counsel.md --post`,
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
				EnvVars []string `json:"env_vars"`
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
		// Codex gives a plugin's server only the variables it names, so
		// without these the server finds no GitHub token the shell finds.
		if m.host == verbs.HostCodex && !slices.Equal(srv.EnvVars, CodexMCPEnv) {
			t.Errorf("codex: MCP server env_vars %v, want %v", srv.EnvVars, CodexMCPEnv)
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
// and Amp's `amp plugins list`. Cursor is not covered here: its CLI has no
// headless way to list a plugin's skills or commands, so the subtest says so
// and skips (D-3.2b.q).
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
		list := exec.Command("amp", "plugins", "list")
		list.Dir = dir
		b, err := list.CombinedOutput()
		if err != nil && strings.Contains(string(b), "Unable to connect to https://ampcode.com") {
			// amp loads the account's global plugins first; without a sign-in or the network (aayu verify's sandbox
			// has neither) it lists nothing, so this cannot check the plugin here.
			t.Skipf("amp cannot reach ampcode.com to list plugins: %s", strings.TrimSpace(string(b)))
		}
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
	t.Run("cursor", func(t *testing.T) {
		// What the Cursor CLI (2026.10.16-e3cb207) can show about a plugin,
		// recorded on megaman (Appendix B D-5.5c.23): with `--plugin-dir` or a
		// copy in ~/.cursor/plugins/local/aayudha, a signed-in `cursor-agent -p`
		// loads the plugin's MCP server and runs its hook. It does not load a
		// symlinked local plugin. No command lists a plugin's skills or
		// commands (`plugin` has only `marketplace`, `mcp list` omits plugin
		// servers, and the stream-json init event carries no catalog), and a
		// run needs a Cursor sign-in and a model call. So this test proves
		// nothing about Cursor; TestPluginManifests checks its manifest, and
		// the golden trees its install.
		t.Skip("Cursor: no headless way to list a plugin's skills or commands; its MCP server and hook were proven live only (D-3.2b.q)")
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

// install turns the plugin off for Claude Code and Codex, so it gives each of
// them every command its plugin manifest carries, under the plugin's name
// (D-3.2b.k): Claude Code's /aayudha:<name>, and the aayudha:source-command-<name>
// skill Codex makes of a plugin command. Cursor, which reads neither, gets the
// same commands as /aayudha-<name> (D-3.2b.n), and the CLI permissions its
// print mode needs to run aayu and the aayudha MCP tools (D-3.2b.o).
func TestInstallKeepsThePluginsCommands(t *testing.T) {
	for host, manifest := range map[verbs.HostID]string{verbs.HostClaude: ".claude-plugin/plugin.json", verbs.HostCodex: ".codex-plugin/plugin.json", verbs.HostCursor: ".cursor-plugin/plugin.json"} {
		root := t.TempDir()
		o := Options{Root: root, Home: t.TempDir(), Hosts: []verbs.HostID{host}, Assets: os.DirFS("../.."), Binary: "/opt/aayudha/bin/aayu", Version: "1.2.3", SkillsVersion: "0.0.0"}
		res, err := Plan(o)
		if err != nil {
			t.Fatal(err)
		}
		if err := Apply(o, res.Edits, false); err != nil {
			t.Fatal(err)
		}
		var m struct{ Commands []string }
		b, _ := os.ReadFile(filepath.Join("../..", manifest))
		if err := json.Unmarshal(b, &m); err != nil || len(m.Commands) == 0 {
			t.Fatalf("%s: %v", manifest, err)
		}
		for _, dir := range m.Commands {
			files, _ := filepath.Glob(filepath.Join("../..", filepath.FromSlash(dir), "*.md"))
			for _, f := range files {
				name := strings.TrimSuffix(filepath.Base(f), ".md")
				src, _ := os.ReadFile(f)
				switch host {
				case verbs.HostClaude:
					if got, err := os.ReadFile(filepath.Join(root, ".claude/commands/aayudha", name+".md")); err != nil || !bytes.Equal(got, src) {
						t.Errorf("Claude Code: no /aayudha:%s from install (%v)", name, err)
					}
				case verbs.HostCodex:
					got, err := os.ReadFile(filepath.Join(root, ".agents/skills/aayudha-source-command-"+name, "SKILL.md"))
					if err != nil || !strings.HasPrefix(string(got), "---\nname: \"aayudha:source-command-"+name+"\"\n") {
						t.Errorf("Codex: no aayudha:source-command-%s skill from install: %q %v", name, got, err)
					}
				case verbs.HostCursor:
					// Cursor takes a command's first line as its title, so
					// the file starts with the command's own heading.
					got, err := os.ReadFile(filepath.Join(root, ".cursor/commands/aayudha-"+name+".md"))
					if err != nil || !strings.HasPrefix(string(got), "# ") || !strings.Contains(string(src), string(got)) {
						t.Errorf("Cursor: no /aayudha-%s from install: %q %v", name, got, err)
					}
				}
			}
		}
		if host == verbs.HostCursor {
			// Cursor rejects the whole file unless allow and deny are both lists.
			var cli struct {
				Permissions struct{ Allow, Deny []string }
			}
			b, _ := os.ReadFile(filepath.Join(root, ".cursor/cli.json"))
			if json.Unmarshal(b, &cli) != nil || !slices.Contains(cli.Permissions.Allow, "Shell(aayu)") || !slices.Contains(cli.Permissions.Allow, "Mcp(aayudha:*)") || cli.Permissions.Deny == nil {
				t.Errorf("Cursor's print mode cannot run aayu or its MCP tools: .cursor/cli.json %s", b)
			}
			if !slices.ContainsFunc(res.Notes, func(n string) bool { return strings.Contains(n, "cursor-agent mcp enable aayudha") }) {
				t.Errorf("install does not say to approve the MCP server in Cursor's CLI: %q", res.Notes)
			}
		}
	}
}

// The repository install writes is the same whatever plugins the machine
// running it has, so a teammate who clones it without the plugin gets every
// aayudha skill; and in it the plugin steps aside (Claude Code: the shared
// project settings; Codex: the trusted project's config), so a teammate with
// the plugin sees each skill once (D-3.2b.h). Cursor has no such switch, and
// doctor's SkillDoubles says what to do there.
func TestInstallIsTheSameWithOrWithoutThePlugin(t *testing.T) {
	plain, withPlugin := t.TempDir(), t.TempDir()
	write(t, withPlugin, ".claude/settings.json", `{"enabledPlugins": {"aayudha@aayudha": true}}`)
	write(t, withPlugin, ".codex/config.toml", "[plugins.\"aayudha@aayudha\"]\nenabled = true\n")
	write(t, withPlugin, ".cursor/plugins/local/aayudha/.cursor-plugin/plugin.json", `{"name": "aayudha"}`)
	var trees []map[string][]byte
	var root string
	for _, home := range []string{plain, withPlugin} {
		root = t.TempDir()
		o := opts(t, root, home, false, verbs.HostClaude, verbs.HostCodex, verbs.HostCursor)
		res, err := Plan(o)
		if err != nil {
			t.Fatal(err)
		}
		if err := Apply(o, res.Edits, false); err != nil {
			t.Fatal(err)
		}
		trees = append(trees, snapshot(t, root))
	}
	if len(trees[0]) != len(trees[1]) {
		t.Fatalf("%d files without the plugin, %d with it", len(trees[0]), len(trees[1]))
	}
	for f, b := range trees[0] {
		if !bytes.Equal(b, trees[1][f]) {
			t.Errorf("%s differs with the plugin on the installing machine", f)
		}
	}
	for _, f := range []string{".claude/skills/alpha/SKILL.md", ".agents/skills/alpha/SKILL.md", ".claude/settings.json", ".codex/config.toml"} {
		if trees[1][f] == nil {
			t.Fatalf("no %s", f)
		}
	}
	if !strings.Contains(string(trees[1][".claude/settings.json"]), `"aayudha@aayudha": false`) ||
		!strings.Contains(string(trees[1][".codex/config.toml"]), "[plugins.\"aayudha@aayudha\"]\nenabled = false\n# END aayudha") {
		t.Fatal("install does not turn the plugin off for the repository")
	}
	// In that repository the plugins step aside: the project settings beat
	// the user's, and Codex reads the project's once it trusts the project.
	codexHome := filepath.Join(withPlugin, ".codex")
	lock, _, _ := ReadLock(root)
	if PluginEnabled(verbs.HostClaude, root, withPlugin, codexHome) || !PluginEnabled(verbs.HostCodex, root, withPlugin, codexHome) {
		t.Error("Claude Code's plugin should be off here, and Codex's on until it trusts the project")
	}
	has := func(d []string, prefix, says string) bool {
		return slices.ContainsFunc(d, func(l string) bool { return strings.HasPrefix(l, prefix) && strings.Contains(l, says) })
	}
	d := SkillDoubles(root, withPlugin, codexHome, lock)
	if len(d) != 2 || !has(d, "OpenAI Codex:", "trust this project") || !has(d, "Cursor:", "disable the aayudha plugin for this workspace") {
		t.Errorf("doubles before Codex trusts the project: %q", d)
	}
	write(t, withPlugin, ".codex/config.toml", fmt.Sprintf("[plugins.\"aayudha@aayudha\"]\nenabled = true\n[projects.%q]\ntrust_level = \"trusted\"\n", root))
	if d := SkillDoubles(root, withPlugin, codexHome, lock); len(d) != 1 || !strings.HasPrefix(d[0], "Cursor:") {
		t.Errorf("doubles once Codex trusts the project: %q", d)
	}
	// A teammate turning the plugin back on for themselves gets the double named.
	write(t, root, ".claude/settings.local.json", `{"enabledPlugins": {"aayudha@aayudha": true}}`)
	if d := SkillDoubles(root, withPlugin, codexHome, lock); len(d) != 2 || !has(d, "Claude Code:", "settings.local.json") {
		t.Errorf("a local override: %q", d)
	}
}

// Each host reads one copy of each aayudha skill (D-3.2b.g): Cursor also
// reads .claude/skills, so with Claude Code installed it shares Claude Code's
// copies, and .agents/skills holds aayudha's only for Codex (or without
// Claude Code). Only Claude Code, Codex, and Cursor together leave Cursor
// two copies, which install and doctor name.
func TestInstallGivesCursorOneCopy(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	write(t, root, ".agents/skills/verify-web/SKILL.md", "---\nname: verify-web\ndescription: \"The product's own.\"\n---\n")
	exists := func(rel string) bool { _, err := os.Stat(filepath.Join(root, rel)); return err == nil }
	install := func(hosts ...verbs.HostID) (*Result, Lock) {
		t.Helper()
		o := opts(t, root, home, false, hosts...)
		res, err := Plan(o)
		if err != nil {
			t.Fatal(err)
		}
		if err := Apply(o, res.Edits, false); err != nil {
			t.Fatal(err)
		}
		lock, _, _ := ReadLock(root)
		if p := CheckCopies(root); len(p) > 0 {
			t.Errorf("copies after installing %v: %q", lock.Hosts, p)
		}
		return res, lock
	}
	install(verbs.HostCursor)
	if !exists(".agents/skills/alpha/SKILL.md") || exists(".claude/skills/alpha") {
		t.Fatal("Cursor alone reads .agents/skills")
	}
	_, lock := install(verbs.HostClaude)
	if exists(".agents/skills/alpha") || !exists(".claude/skills/alpha/SKILL.md") || CursorReadsBoth(root, lock) {
		t.Error("with Claude Code, Cursor and Claude Code share .claude/skills, and .agents/skills keeps no aayudha copy")
	}
	// The product's verify skill stays in .agents/skills, and Claude Code
	// reaches it through a command that points there, not a second copy in
	// .claude/skills, which Cursor would list too (D-3.2b.l).
	if !exists(".agents/skills/verify-web/SKILL.md") || exists(".claude/skills/verify-web") {
		t.Error("Cursor reads the product's verify skill twice, or not at all")
	}
	if b, err := os.ReadFile(filepath.Join(root, ".claude/commands/aayudha/verify-web.md")); err != nil ||
		!strings.HasPrefix(string(b), "---\ndescription: \"The product's own.\"\n---\n") || !strings.Contains(string(b), "Read `.agents/skills/verify-web/SKILL.md`") {
		t.Errorf("Claude Code's command for the product's verify skill: %q %v", b, err)
	}
	res, lock := install(verbs.HostCodex)
	if !exists(".agents/skills/alpha/SKILL.md") || !CursorReadsBoth(root, lock) || !slices.Contains(res.Notes, cursorBoth) {
		t.Errorf("Codex needs .agents/skills and Claude Code .claude/skills, so Cursor reads both, and install says so: %q", res.Notes)
	}
}
