package cli

import (
	"archive/tar"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	aayudha "github.com/rahulnakmol/agentic-ayudha"
	"github.com/rahulnakmol/agentic-ayudha/internal/agentenv"
	"github.com/rahulnakmol/agentic-ayudha/internal/hosts"
	"github.com/rahulnakmol/agentic-ayudha/internal/skills"
)

// installEnv runs install against the real embedded skills, with a fake binary
// at an absolute path that reports this build's version.
func installEnv(t *testing.T) (repo, home string, opts Options) {
	t.Helper()
	repo = gitInit(t, "dev", map[string]string{"README.md": "x\n"})
	home = t.TempDir()
	bin, _ := filepath.Abs(filepath.FromSlash("/opt/aayudha/bin/aayu")) // binaryPath makes it absolute: D:\opt\… on Windows
	opts = Options{IsTerminal: stdinTTY, Dir: repo, Assets: aayudha.Assets, Build: BuildInfo{Version: "1.0.0"}, LookPath: noTools,
		Executable: func() (string, error) { return bin, nil },
		Exec: func(ctx context.Context, dir, name string, args ...string) (string, error) {
			if name == bin && len(args) == 1 && args[0] == "version" {
				return "aayu 1.0.0\n", nil
			}
			return realGitWithLFS(ctx, dir, name, args...)
		}}
	return repo, home, opts
}

func TestInstall(t *testing.T) {
	repo, home, opts := installEnv(t)
	env := map[string]string{"HOME": home}
	before := treeHash(t, repo)

	// --diff is the default: the plan is shown and nothing is written.
	code, out, errs := run(t, env, opts, "install")
	if code != 0 {
		t.Fatalf("install: exit %d\n%s%s", code, out, errs)
	}
	for _, want := range []string{"create .agents/skills/ready/SKILL.md", "create .claude/skills/ready/SKILL.md", "create CLAUDE.md",
		"# Edits in your home directory (1), confirmed separately", "create ~/.config/amp/settings.json", "aayu install --apply writes it"} {
		if !strings.Contains(out, want) {
			t.Errorf("plan lacks %q:\n%s", want, out)
		}
	}
	if treeHash(t, repo) != before {
		t.Fatal("install without --apply wrote to the repository")
	}

	// Agents never apply.
	if code, _, errs := run(t, map[string]string{"HOME": home, "AAYU_AGENT": "1"}, opts, "install", "--apply"); code != ExitBlocked {
		t.Fatalf("agent install --apply: exit %d %s", code, errs)
	}

	// --apply without confirmation writes the repository but not the home directory, and says so.
	code, _, errs = run(t, env, opts, "install", "--apply")
	if code != ExitBlocked || !strings.Contains(errs, "1 home-directory edit(s) not applied") {
		t.Fatalf("install --apply without confirmation: exit %d %s", code, errs)
	}
	if _, err := os.Stat(filepath.Join(home, ".config", "amp", "settings.json")); !os.IsNotExist(err) {
		t.Fatalf("home edit applied without confirmation: %v", err)
	}
	lock, ok, err := hosts.ReadLock(repo)
	if err != nil || !ok || len(lock.Skills) != 35 || strings.Join(lock.Hosts, ",") != "amp,cursor,claude,codex,copilot" {
		t.Fatalf("lock: %v %v %d skills, hosts %v", ok, err, len(lock.Skills), lock.Hosts)
	}
	// The installed copies pass the WP3.1 lint on disk, which includes the
	// no-symlink rule. The plugin's commands that install gives Codex as
	// skills (D-3.2b.k) take the form Codex gives a plugin's commands, not a
	// verb skill's, so the lint leaves them to TestInstallKeepsThePluginsCommands.
	for _, d := range []string{hosts.AgentsSkills, hosts.ClaudeSkills} {
		for _, f := range skills.Lint(os.DirFS(repo), d) {
			if !strings.HasPrefix(f.Path, "aayudha-source-command-") {
				t.Errorf("%s: %s", d, f)
			}
		}
	}

	// --yes confirms for a human; then re-apply changes nothing.
	if code, out, errs := run(t, env, opts, "install", "--apply", "--yes"); code != 0 || !strings.Contains(out, "create ~/.config/amp/settings.json") {
		t.Fatalf("install --apply --yes: exit %d\n%s%s", code, out, errs)
	}
	bin, _ := filepath.Abs(filepath.FromSlash("/opt/aayudha/bin/aayu"))
	cmd, _ := json.Marshal(bin)
	if b, err := os.ReadFile(filepath.Join(home, ".config", "amp", "settings.json")); err != nil || !strings.Contains(string(b), `"command": `+string(cmd)) {
		t.Fatalf("Amp settings: %v %s", err, b)
	}
	applied := treeHash(t, repo)
	if code, out, _ := run(t, env, opts, "install", "--apply", "--yes"); code != 0 || !strings.Contains(out, "install: nothing to change") {
		t.Fatalf("re-apply: exit %d\n%s", code, out)
	}
	if treeHash(t, repo) != applied {
		t.Fatal("re-apply changed the repository")
	}

	// doctor sees a hand-edited copy; --fix restores it; then doctor passes.
	edited := filepath.Join(repo, ".claude", "skills", "ready", "SKILL.md")
	if err := os.WriteFile(edited, []byte("edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, out, _ := run(t, env, opts, "doctor"); !strings.Contains(out, "FAIL  skills-copies") || !strings.Contains(out, ".claude/skills/ready was edited or is stale") {
		t.Fatalf("doctor on an edited copy:\n%s", out)
	}
	// With every host installed, Cursor reads both copies: a warning, not a failure (D-3.2b.g).
	if _, out, _ := run(t, env, opts, "doctor", "--fix"); !strings.Contains(out, "--fix: re-synced 1 file(s)") || !strings.Contains(out, "WARN  skills-copies      Cursor lists each aayudha skill twice") ||
		!strings.Contains(out, "not one installed from a marketplace") { // D-3.2b.i
		t.Fatalf("doctor --fix:\n%s", out)
	}
	if treeHash(t, repo) != applied {
		t.Fatal("doctor --fix did not restore the installed tree")
	}
	// A hook or MCP entry that differs from what install writes fails doctor:
	// a host would run the changed command from the checkout. Reformatting
	// alone is no change.
	if _, out, _ := run(t, env, opts, "doctor"); !strings.Contains(out, "PASS  host-config") {
		t.Fatalf("doctor on the installed tree:\n%s", out)
	}
	settings := filepath.Join(repo, ".claude", "settings.json")
	orig, _ := os.ReadFile(settings)
	var v any
	_ = json.Unmarshal(orig, &v)
	compact, _ := json.Marshal(v)
	_ = os.WriteFile(settings, compact, 0o644)
	if _, out, _ := run(t, env, opts, "doctor"); !strings.Contains(out, "PASS  host-config") {
		t.Fatalf("doctor on a reformatted settings.json:\n%s", out)
	}
	_ = os.WriteFile(settings, orig, 0o644)
	mcpJSON := filepath.Join(repo, ".mcp.json")
	origMCP, _ := os.ReadFile(mcpJSON)
	_ = os.WriteFile(mcpJSON, []byte(strings.Replace(string(origMCP), `"mcp"`, `"status"`, 1)), 0o644)
	if _, out, _ := run(t, env, opts, "doctor"); !strings.Contains(out, "FAIL  host-config") || !strings.Contains(out, ".mcp.json") {
		t.Fatalf("doctor on a changed MCP entry:\n%s", out)
	}
	_ = os.WriteFile(mcpJSON, origMCP, 0o644)
}

func TestHookCommand(t *testing.T) {
	repo, home, opts := installEnv(t)
	env := map[string]string{"HOME": home}
	if code, out, errs := run(t, env, opts, "hook", "pre-tool", "--host", "cursor"); code != 0 || out != "{\"permission\":\"allow\"}\n" {
		t.Fatalf("pre-tool, not paused: exit %d %q %s", code, out, errs)
	}
	pause := filepath.Join(repo, ".aayudha", "state", "paused")
	_ = os.MkdirAll(filepath.Dir(pause), 0o755)
	if err := os.WriteFile(pause, []byte("today's spend reached 100.00 USD\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _, errs := run(t, env, opts, "hook", "pre-tool", "--host", "claude"); code != 2 || !strings.Contains(errs, "today's spend reached 100.00 USD") {
		t.Fatalf("pre-tool, paused: exit %d %s", code, errs)
	}
	if code, _, _ := run(t, env, opts, "hook", "rm-rf", "--host", "claude"); code != ExitUsage {
		t.Fatalf("unknown hook: exit %d", code)
	}
}

// WP4.5b.3 accept: Claude Code reads the product's own verify skill, kept in
// .agents/skills where Amp, Cursor, and Codex read it, through a pointer
// command (D-3.2b.l): one copy for Cursor, which reads .claude/skills too. A
// pointer reads its source, so a changed feature map needs no re-sync; doctor
// sees one behind a changed description and --fix catches it up; a skill the
// product deleted loses its pointer; an earlier install's full copy goes.
func TestInstallMirrorsVerifySkill(t *testing.T) {
	repo, home, opts := installEnv(t)
	env := map[string]string{"HOME": home}
	src := filepath.Join(repo, ".agents", "skills", "verify-shop")
	_ = os.MkdirAll(filepath.Join(src, "features"), 0o755)
	_ = os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("---\nname: verify-shop\ndescription: Check the shop.\n---\n"), 0o644)
	_ = os.WriteFile(filepath.Join(src, "features", "README.md"), []byte("# map\n"), 0o644)
	// What install before D-3.2b.l left: a full copy in .claude/skills.
	old := filepath.Join(repo, ".claude", "skills", "verify-shop")
	_ = os.MkdirAll(old, 0o755)
	_ = os.WriteFile(filepath.Join(old, "SKILL.md"), []byte("---\nname: verify-shop\ndescription: Check the shop.\n---\n"), 0o644)
	_ = os.MkdirAll(filepath.Join(repo, ".aayudha"), 0o755)
	_ = os.WriteFile(filepath.Join(repo, ".aayudha", "lock.toml"), []byte("version = 1\nhosts = [\"claude\"]\n[product_skills]\nverify-shop = { source = \".agents/skills/verify-shop\", sha256 = \"x\" }\n"), 0o644)
	if code, out, errs := run(t, env, opts, "install", "--apply", "--yes"); code != 0 {
		t.Fatalf("install: exit %d\n%s%s", code, out, errs)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatalf("the earlier full copy stayed, so Cursor lists verify-shop twice: %v", err)
	}
	pointer := filepath.Join(repo, ".claude", "commands", "aayudha", "verify-shop.md")
	read := func() string { b, _ := os.ReadFile(pointer); return strings.ReplaceAll(string(b), "\r\n", "\n") }
	if p := read(); !strings.HasPrefix(p, "---\ndescription: Check the shop.\n---\n") || !strings.Contains(p, "Read `.agents/skills/verify-shop/SKILL.md` and follow it") {
		t.Fatalf("pointer: %q", p)
	}
	if b, _ := os.ReadFile(filepath.Join(src, "SKILL.md")); !strings.Contains(string(b), "name: verify-shop") {
		t.Fatalf("install changed the source skill: %q", b)
	}
	if lock, _, _ := hosts.ReadLock(repo); lock.ProductSkills["verify-shop"].Source != ".agents/skills/verify-shop" {
		t.Fatalf("lock product_skills: %v", lock.ProductSkills)
	}

	_ = os.WriteFile(filepath.Join(src, "features", "README.md"), []byte("# map, with a feature\n"), 0o644)
	if _, out, _ := run(t, env, opts, "doctor"); strings.Contains(out, "verify-shop") {
		t.Fatalf("doctor on a changed feature map:\n%s", out)
	}
	_ = os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("---\nname: verify-shop\ndescription: Check the shop and its cart.\n---\n"), 0o644)
	if _, out, _ := run(t, env, opts, "doctor"); !strings.Contains(out, ".claude/commands/aayudha/verify-shop.md is behind its source .agents/skills/verify-shop") {
		t.Fatalf("doctor on a changed description:\n%s", out)
	}
	if _, out, _ := run(t, env, opts, "doctor", "--fix"); strings.Contains(out, "FAIL  skills-copies") {
		t.Fatalf("doctor --fix:\n%s", out)
	}
	if p := read(); !strings.Contains(p, "description: Check the shop and its cart.") {
		t.Fatalf("pointer after --fix: %q", p)
	}

	_ = os.RemoveAll(src)
	if code, out, errs := run(t, env, opts, "install", "--apply", "--yes"); code != 0 {
		t.Fatalf("install after delete: exit %d\n%s%s", code, out, errs)
	}
	if _, err := os.Stat(pointer); !os.IsNotExist(err) {
		t.Errorf("pointer to a deleted skill: %v", err)
	}
	if lock, _, _ := hosts.ReadLock(repo); len(lock.ProductSkills) != 0 {
		t.Errorf("lock after delete: %v", lock.ProductSkills)
	}
}

// TestInstallEnv: an agent may set up and verify agent environments (they land
// through review like any file); a clean-container failure is fixed and retried.
func TestInstallEnv(t *testing.T) {
	repo := gitInit(t, "dev", map[string]string{"go.mod": "module m\n\ngo 1.25.0\n",
		".github/workflows/ci.yml": "on: pull_request\njobs:\n  test: {runs-on: x, steps: [{run: go test ./...}]}\n"})
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "none")) // no machine-wide core.hooksPath
	sum := strings.Repeat("a", 64)
	runs := 0
	opts := Options{Dir: repo, Build: BuildInfo{Version: "1.0.0"},
		EnvFetch: func(_ context.Context, u string) ([]byte, error) {
			return []byte(`[{"version":"go1.25.8","stable":true,"files":[{"os":"linux","arch":"amd64","kind":"archive","sha256":"` + sum + `"},{"os":"linux","arch":"arm64","kind":"archive","sha256":"` + sum + `"}]}]`), nil
		},
		EnvRunner: func(_ context.Context, stdin io.Reader, out io.Writer, name string, args ...string) error {
			if name != "docker" || len(args) == 0 {
				return errors.New("no")
			}
			if args[0] != "run" {
				return nil // info, build
			}
			runs++
			files := map[string]string{}
			tr := tar.NewReader(stdin)
			for h, err := tr.Next(); err == nil; h, err = tr.Next() {
				b, _ := io.ReadAll(tr)
				files[h.Name] = string(b)
			}
			if !strings.Contains(files[".agents/setup"], " unzip\n") {
				fmt.Fprintln(out, ".agents/check: line 20: unzip: command not found")
				return errors.New("exit status 127")
			}
			check := strings.Split(strings.TrimSpace(files[".agents/check"]), "\n")
			fmt.Fprintf(out, "aayu-phase setup-cold 40\naayu-phase setup-warm 2\n%s\naayu-phase check 9\n", strings.Trim(strings.TrimPrefix(check[len(check)-1], "echo "), `"`))
			return nil
		}}
	code, out, errs := run(t, map[string]string{"AAYU_AGENT": "1", "HOME": t.TempDir()}, opts, "install", "--env", "--verify", "--host", "amp")
	if code != 0 {
		t.Fatalf("install --env --verify: exit %d\n%s%s", code, out, errs)
	}
	if runs != 2 || !strings.Contains(out, "fixed: added apt package unzip") || !strings.Contains(out, "PASS amp in debian:12: setup cold 40s, warm 2s, check 9s; 2 attempt(s)") {
		t.Fatalf("want a failing attempt fixed, then a pass; %d runs:\n%s", runs, out)
	}
	sts, err := agentenv.Status(repo)
	if err != nil {
		t.Fatal(err)
	}
	if sts[0].Host != "amp" || sts[0].State != agentenv.Verified || sts[1].State != agentenv.Missing {
		t.Fatalf("status after verifying amp: %+v", sts)
	}
	// The local gate (Story #399): the CI job is listed, it has no local step yet, and the clone's pre-push hook runs it.
	if !strings.Contains(out, "Local gate gaps") || !strings.Contains(out, "test (.github/workflows/ci.yml)") {
		t.Errorf("readback does not name the gate's gap:\n%s", out)
	}
	if hook, err := os.ReadFile(filepath.Join(repo, ".git", "hooks", "pre-push")); err != nil || string(hook) != agentenv.PrePushHook {
		t.Errorf("pre-push hook = %q, %v; want the local gate's hook", hook, err)
	}
	spec, _ := os.ReadFile(filepath.Join(repo, agentenv.SpecPath))
	if !strings.Contains(string(spec), "'unzip'") || !strings.Contains(string(spec), "fix: unzip was missing in a clean container") {
		t.Errorf("the fix is not recorded in the spec:\n%s", spec)
	}
}

// TestHookProviderCalls (Decision #449): the pre-tool hook lets a host's
// PostHog or Sentry MCP server and CLI read, and refuses a write, in each
// host's own event format.
func TestHookProviderCalls(t *testing.T) {
	repo, home, opts := installEnv(t)
	opts.IsTerminal = nil // the host pipes its event in
	env := map[string]string{"HOME": home}
	for _, tt := range []struct {
		name, stdin string
		args        []string
		code        int
		out, errs   string
	}{
		{"claude posthog write", `{"tool_name":"mcp__posthog__update-feature-flag","tool_input":{"id":3,"active":false}}`,
			[]string{"--host", "claude"}, 2, "", "catalog action a person approves"},
		{"claude posthog read", `{"tool_name":"mcp__posthog__feature-flag-get-all","tool_input":{}}`,
			[]string{"--host", "claude"}, 0, "", ""},
		{"codex sentry-cli release", `{"tool_name":"Bash","tool_input":{"command":"sentry-cli releases new 1.2.3"}}`,
			[]string{"--host", "codex"}, 2, "", "product's CI"},
		{"claude sentry person data", `{"tool_name":"mcp__sentry__get_replay_details","tool_input":{}}`,
			[]string{"--host", "claude"}, 2, "", "allow_pii = true"},
		{"cursor posthog exec write", `{"hook_event_name":"beforeMCPExecution","tool_name":"exec","tool_input":"{\"command\":\"call experiment-launch {\\\"id\\\": 7}\"}","url":"https://mcp.posthog.com/mcp"}`,
			[]string{"--host", "cursor"}, 2, `"permission":"deny"`, "experiment-launch"},
		{"amp event flag", "", []string{"--host", "amp", "--event", `{"tool_name":"mcp__sentry__execute_sentry_tool","tool_input":{"name":"update_issue"}}`}, 2, "", "`update_issue`"},
		{"other MCP server", `{"tool_name":"mcp__github__create_issue","tool_input":{}}`, []string{"--host", "claude"}, 0, "", ""},
		{"copilot sentry read", `{"sessionId":"05b234bf-9c41-41f2-8604-0ff1d8dc3011","cwd":"/r","timestamp":1792286000000,"toolName":"sentry-find_dropped_events","toolArgs":{"statsPeriod":"7d"}}`,
			[]string{"--host", "copilot"}, 0, "", ""},
		{"copilot posthog write", `{"sessionId":"s","toolName":"posthog-update-feature-flag","toolArgs":"{\"id\":3}"}`,
			[]string{"--host", "copilot"}, 2, `"permissionDecision":"deny"`, "`update-feature-flag`"},
		{"copilot bash read", `{"sessionId":"s","toolName":"bash","toolArgs":"{\"command\":\"posthog-cli exp query run \\\"SELECT 1 LIMIT 1\\\"\"}"}`,
			[]string{"--host", "copilot"}, 0, "", ""},
		{"copilot bash release", `{"sessionId":"s","toolName":"bash","toolArgs":{"command":"bash -c 'sentry-cli releases new 1.2.3'"}}`,
			[]string{"--host", "copilot"}, 2, `"permissionDecision":"deny"`, "product's CI"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			code, out, errs := runIn(t, env, opts, tt.stdin, append([]string{"hook", "pre-tool"}, tt.args...)...)
			if code != tt.code || !strings.Contains(out, tt.out) || !strings.Contains(errs, tt.errs) {
				t.Fatalf("exit %d, stdout %q, stderr %q; want exit %d with %q and %q", code, out, errs, tt.code, tt.out, tt.errs)
			}
		})
	}
	// A Sentry MCP server registered under a name that does not say so is
	// known by its package.
	if err := os.WriteFile(filepath.Join(repo, ".mcp.json"), []byte(`{"mcpServers":{"errors":{"command":"npx","args":["-y","@sentry/mcp-server@latest"]}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _, errs := runIn(t, env, opts, `{"tool_name":"mcp__errors__update_issue","tool_input":{}}`, "hook", "pre-tool", "--host", "claude"); code != 2 || !strings.Contains(errs, "`update_issue`") {
		t.Fatalf("a renamed Sentry server's write: exit %d %s; want it refused", code, errs)
	}
	// Under Copilot, a server registered in $COPILOT_HOME/mcp-config.json
	// under another name is known by its package too.
	copilotHome := filepath.Join(t.TempDir(), "copilot")
	_ = os.MkdirAll(copilotHome, 0o755)
	_ = os.WriteFile(filepath.Join(copilotHome, "mcp-config.json"), []byte(`{"mcpServers":{"monitoring":{"type":"local","command":"npx","args":["-y","@sentry/mcp-server"],"tools":["*"]}}}`), 0o644)
	env["COPILOT_HOME"] = copilotHome
	if code, out, errs := runIn(t, env, opts, `{"sessionId":"s","toolName":"monitoring-update_issue","toolArgs":{}}`, "hook", "pre-tool", "--host", "copilot"); code != 2 || !strings.Contains(out, `"permissionDecision":"deny"`) || !strings.Contains(errs, "`update_issue`") {
		t.Fatalf("a Sentry server in COPILOT_HOME under another name: exit %d %s %s; want it refused", code, out, errs)
	}
	delete(env, "COPILOT_HOME")
	// The repository's protected config opts in to personal data, and only
	// as committed on the dev branch counts: an agent cannot opt itself in by
	// editing the file, or by committing it on its own branch.
	replay := func() int {
		code, _, _ := runIn(t, env, opts, `{"tool_name":"mcp__sentry__get_replay_details","tool_input":{}}`, "hook", "pre-tool", "--host", "claude")
		return code
	}
	base := git(t, repo, "rev-parse", "HEAD")
	cfg := filepath.Join(repo, ".aayudha", "config.toml")
	_ = os.MkdirAll(filepath.Dir(cfg), 0o755)
	if err := os.WriteFile(cfg, []byte("version = 1\n[integrations.sentry]\nsecret = \"sentry\"\nscope = \"read\"\norg = \"acme\"\nproject = \"1\"\nallow_pii = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := replay(); code != 2 {
		t.Fatalf("allow_pii = true in the working tree only: exit %d; want the read refused", code)
	}
	git(t, repo, "add", cfg)
	git(t, repo, "commit", "-q", "-m", "allow personal data")
	if code := replay(); code != 0 {
		t.Fatalf("allow_pii = true committed, no dev branch to compare with: exit %d; want the read allowed", code)
	}
	git(t, repo, "update-ref", "refs/remotes/origin/dev", base)
	if code := replay(); code != 2 {
		t.Fatalf("allow_pii = true committed past origin/dev: exit %d; want the read refused", code)
	}
	git(t, repo, "update-ref", "refs/remotes/origin/dev", "HEAD")
	if code := replay(); code != 0 {
		t.Fatalf("allow_pii = true on origin/dev: exit %d; want the read allowed", code)
	}
}

// TestHookLogsProviderCalls (Decision #449): the post-tool hook logs each
// PostHog or Sentry call a host ran, with its window and row limit, under
// .aayudha/state/calls; it never fails the host, and logs nothing else.
func TestHookLogsProviderCalls(t *testing.T) {
	repo, home, opts := installEnv(t)
	opts.IsTerminal = nil
	env := map[string]string{"HOME": home}
	calls := []string{
		`{"session_id":"s-42","tool_name":"mcp__sentry__find_dropped_events","tool_input":{"statsPeriod":"7d"},"tool_response":[{"outcome":"rate_limited"}]}`,
		`{"session_id":"s-42","tool_name":"Bash","tool_input":{"command":"posthog-cli exp query run \"SELECT count() FROM events WHERE timestamp > now() - INTERVAL 1 DAY LIMIT 1\""},"tool_response":{"stdout":"[[42]]"}}`,
		`{"session_id":"s-42","tool_name":"mcp__github__list_issues","tool_input":{},"tool_response":[]}`,
		`{"session_id":"s-42","tool_name":"Bash","tool_input":{"command":"go test ./..."},"tool_response":{"stdout":"ok"}}`,
	}
	for _, c := range calls {
		if code, out, errs := runIn(t, env, opts, c, "hook", "post-tool", "--host", "claude"); code != 0 || out != "" || errs != "" {
			t.Fatalf("post-tool: exit %d %q %q", code, out, errs)
		}
	}
	b, err := os.ReadFile(filepath.Join(repo, ".aayudha", "state", "calls", "s-42.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) != 2 || !strings.Contains(lines[0], `"id":"c1"`) || !strings.Contains(lines[0], `"call":"find_dropped_events"`) ||
		!strings.Contains(lines[0], `"why_not_citable":"it names no row limit"`) ||
		!strings.Contains(lines[1], `"id":"c2"`) || !strings.Contains(lines[1], `"channel":"cli"`) || !strings.Contains(lines[1], `"citable":true`) {
		t.Fatalf("call log:\n%s", b)
	}
	// Copilot's postToolUse and postToolUseFailure (Copilot CLI 1.0.98, live 2026-10-18).
	for _, c := range []string{
		`{"sessionId":"cp-1","timestamp":1792293989252,"cwd":"/r","toolName":"sentry-find_releases","toolArgs":{"statsPeriod":"7d","limit":5},"toolResult":{"resultType":"success","textResultForLlm":"[{\"version\":\"1\"}]"}}`,
		`{"sessionId":"cp-1","timestamp":1792293990785,"cwd":"/r","toolName":"sentry-find_releases","toolArgs":{"statsPeriod":"7d","limit":5},"error":"MCP server 'sentry': exit 2"}`,
	} {
		if code, out, errs := runIn(t, env, opts, c, "hook", "post-tool", "--host", "copilot"); code != 0 || out != "" || errs != "" {
			t.Fatalf("copilot post-tool: exit %d %q %q", code, out, errs)
		}
	}
	b, err = os.ReadFile(filepath.Join(repo, ".aayudha", "state", "calls", "cp-1.jsonl"))
	if lines := strings.Split(strings.TrimSpace(string(b)), "\n"); err != nil || len(lines) != 2 || !strings.Contains(lines[0], `"rows":1`) || !strings.Contains(lines[0], `"citable":true`) ||
		!strings.Contains(lines[1], `"failed":true`) || !strings.Contains(lines[1], `"citable":false`) {
		t.Fatalf("copilot call log: %v\n%s", err, b)
	}
	// A log that cannot be written never fails the host: the call has run.
	broken := filepath.Join(repo, ".aayudha", "state", "calls", "s-43.jsonl")
	if err := os.MkdirAll(broken, 0o755); err != nil {
		t.Fatal(err)
	}
	code, out, errs := runIn(t, env, opts, strings.Replace(calls[0], "s-42", "s-43", 1), "hook", "post-tool", "--host", "claude")
	if code != 0 || out != "" || !strings.Contains(errs, "could not log the sentry call") {
		t.Fatalf("unwritable log: exit %d %q %q; want exit 0 and the failure on stderr", code, out, errs)
	}
}

// Copilot gets the host files but no agent environment or loops yet, so
// install says so instead of succeeding without doing anything for it.
func TestInstallRefusesCopilotEnvAndLoops(t *testing.T) {
	repo, home, opts := installEnv(t)
	env := map[string]string{"HOME": home}
	before := treeHash(t, repo)
	for _, args := range [][]string{{"install", "--env", "--host", "copilot"}, {"install", "--loops", "--host", "copilot", "--repo", "o/r"}} {
		if code, _, errs := run(t, env, opts, args...); code != ExitUsage || !strings.Contains(errs, "copilot") {
			t.Errorf("%v: exit %d %s", args, code, errs)
		}
	}
	if treeHash(t, repo) != before {
		t.Error("a refused install wrote to the repository")
	}
}
