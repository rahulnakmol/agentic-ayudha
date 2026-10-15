package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	aayudha "github.com/rahulnakmol/agentic-ayudha"
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
	opts = Options{Dir: repo, Assets: aayudha.Assets, Build: BuildInfo{Version: "1.0.0"}, LookPath: noTools,
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
	if err != nil || !ok || len(lock.Skills) != 30 || strings.Join(lock.Hosts, ",") != "amp,cursor,claude,codex" {
		t.Fatalf("lock: %v %v %d skills, hosts %v", ok, err, len(lock.Skills), lock.Hosts)
	}
	// The installed copies pass the WP3.1 lint on disk, which includes the no-symlink rule.
	for _, d := range []string{hosts.AgentsSkills, hosts.ClaudeSkills} {
		for _, f := range skills.Lint(os.DirFS(repo), d) {
			t.Errorf("%s: %s", d, f)
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
	if _, out, _ := run(t, env, opts, "doctor", "--fix"); !strings.Contains(out, "--fix: re-synced 1 file(s)") || !strings.Contains(out, "PASS  skills-copies") {
		t.Fatalf("doctor --fix:\n%s", out)
	}
	if treeHash(t, repo) != applied {
		t.Fatal("doctor --fix did not restore the installed tree")
	}
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
