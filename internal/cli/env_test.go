package cli

import (
	"bytes"
	"context"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
	"github.com/rahulnakmol/agentic-ayudha/internal/hosts"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

// Zero Options run against the real process: the environment (here the
// config env layer, AAYU_<SECTION>__<KEY>) and the working directory (the
// repository whose config applies). --verbose writes diagnostics, here the
// model substitution, to stderr.
func TestRunUsesProcessEnvironment(t *testing.T) {
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AAYU_AGENT", "")
	t.Setenv("AAYU_MODELS__REVIEW__CURSOR__MODEL", "gpt-6.1-sol")
	t.Chdir(repo)
	var out, errb bytes.Buffer
	code := Run(context.Background(), []string{"models", "show", "--host", "cursor", "--verbose"}, IO{In: strings.NewReader(""), Out: &out, Err: &errb},
		Options{GHToken: func(context.Context) (string, error) { return "", errors.New("gh disabled in tests") }})
	if code != 0 || !regexp.MustCompile(`review\s+cursor\s+grok-4\.7\s+xai\s+fallback`).MatchString(out.String()) {
		t.Fatalf("exit %d\n%s%s", code, out.String(), errb.String())
	}
	if !strings.Contains(errb.String(), "models: review lane on cursor: gpt-6.1-sol (openai) is not available on Cursor") {
		t.Fatalf("--verbose stderr:\n%s", errb.String())
	}
}

// Values reads each input as its declared type, and an unset or mistyped
// input as the zero value.
func TestValues(t *testing.T) {
	v := Values{"days": 14, "title": "x", "create": true, "bad": "14"}
	if v.Int("days") != 14 || v.Int("bad") != 0 || v.Int("unset") != 0 || v.String("title") != "x" || !v.Bool("create") || v.Has("unset") || !v.Has("bad") {
		t.Fatalf("values: %v", v)
	}
}

// Bug #170: human-only verbs run from a skill's shell call in each host, with the founder's token and without
// AAYU_AGENT, are refused as an agent lane before any GitHub call, even when the host gives the command a terminal.
// This holds below the pre-tool hook (#169): the hook blocks a shell command it can read, and the binary still
// refuses one the hook cannot see, such as a script that runs aayu.
func TestHumanOnlyRefusedInHostShells(t *testing.T) {
	g := fake.New(t, "o/r", "User")
	commands := [][]string{
		{"plan", "2", "--solo-bypass"},
		{"decide", "5", "--accept", "B"},
		{"resume"},
	}
	for host, marker := range hostShells {
		env := maps.Clone(marker)
		env["AAYU_GITHUB_TOKEN"] = "founder-token"
		for _, args := range commands {
			code, _, errOut := run(t, env, Options{IsTerminal: stdinTTY, GitHubAPI: g.URL(), GitHubGap: -1}, append(args, "--repo", "o/r")...)
			if code != ExitBlocked || !strings.Contains(errOut, "refused for an agent lane") {
				t.Errorf("%s shell: aayu %s: exit %d: %s", host, strings.Join(args, " "), code, errOut)
			}
		}
		// The pre-tool hook stops the command when the host forwards it, but not one hidden in a script.
		if host != "amp" {
			if r, _ := hosts.RunHook("pre-tool", verbs.HostID(host), "", filepath.Join(t.TempDir(), "paused"), []byte(`{"tool_input":{"command":"aayu ship 4 --solo-bypass"},"command":"aayu ship 4 --solo-bypass"}`)); r.Code != 2 {
				t.Errorf("%s: the pre-tool hook let `aayu ship 4 --solo-bypass` through", host)
			}
		}
		if cmd, _ := hosts.HumanOnlyCommand("sh ./merge.sh"); cmd != "" {
			t.Errorf("the hook was expected not to see inside a script: %q", cmd)
		}
	}
	if w := g.Writes(); len(w) != 0 {
		t.Errorf("refused commands wrote to GitHub: %v", w)
	}
	// No marker and no terminal (a host that sets nothing, a script): human-only actions fail closed.
	for _, args := range commands {
		code, _, errOut := run(t, map[string]string{"AAYU_GITHUB_TOKEN": "founder-token"}, Options{GitHubAPI: g.URL(), GitHubGap: -1}, append(args, "--repo", "o/r")...)
		if code != ExitBlocked || !strings.Contains(errOut, "needs an interactive terminal") {
			t.Errorf("no marker, no terminal: aayu %s: exit %d: %s", strings.Join(args, " "), code, errOut)
		}
	}
}

// Bug #170: the recorded runs in testdata/host-shells (an Amp thread's shell, Claude Code's Bash tool, and a Claude
// Code hook) refused every
// human-only command with exit 4 (their release run predates D-7.8c, which lets an agent promote), naming a marker this binary still recognises; without a marker, a command with no
// terminal was refused and one on a terminal passed the human check.
func TestHostShellRecords(t *testing.T) {
	refusal := regexp.MustCompile(`refused for an agent lane \(([A-Z_]+)=`)
	for file, marker := range map[string]string{"amp-shell.txt": "AMP_THREAD_ID", "claude-code.txt": "CLAUDECODE", "claude-code-bash.txt": "CLAUDECODE"} {
		b, err := os.ReadFile(filepath.Join("testdata", "host-shells", file))
		if err != nil {
			t.Fatal(err)
		}
		runs := strings.Split(string(b), "$ aayu ")[1:]
		if len(runs) != 5 {
			t.Fatalf("%s: %d runs, want ship, plan, decide, release, resume", file, len(runs))
		}
		for _, r := range runs {
			m := refusal.FindStringSubmatch(r)
			if m == nil || m[1] != marker || !strings.Contains(r, "exit=4") || hosts.AgentMarker(func(k string) string { return map[string]string{m[1]: "x"}[k] }) == "" {
				t.Errorf("%s: %q", file, r)
			}
		}
	}
	// Cursor's record is its CLI's source: every place it sets the marker names one this binary recognises.
	src, _ := os.ReadFile(filepath.Join("testdata", "host-shells", "cursor-agent-source.txt"))
	if strings.Count(string(src), `CURSOR_AGENT:"1"`) != 3 || hosts.AgentMarker(func(k string) string { return map[string]string{"CURSOR_AGENT": "1"}[k] }) != "CURSOR_AGENT=1" {
		t.Errorf("cursor-agent-source.txt:\n%s", src)
	}
	b, _ := os.ReadFile(filepath.Join("testdata", "host-shells", "no-marker.txt"))
	if s := string(b); strings.Count(s, "needs an interactive terminal") != 2 || !strings.Contains(s, "the merge gate must be green") || strings.Contains(s, "refused for an agent lane") {
		t.Errorf("no-marker.txt:\n%s", s)
	}
}
