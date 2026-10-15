package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
	"github.com/rahulnakmol/agentic-ayudha/internal/journey"
	"github.com/rahulnakmol/agentic-ayudha/internal/ui"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

// homeRepo is a set-up checkout (Level 1, Claude Code installed) whose GitHub
// has a Ready story, a story in progress, and a story PR without a human pass.
func homeRepo(t *testing.T) (string, *fake.GitHub) {
	t.Helper()
	repo := gitInit(t, "dev", map[string]string{
		".aayudha/config.toml": "version = 1\n",
		".aayudha/lock.toml":   "version = 1\ninstalled_by = \"1.0.0\"\nskills_version = \"0.0.0\"\npin = \"\"\nhosts = [\"claude\"]\n\n[skills]\n",
	})
	g := fake.New(t, "o/r", "User")
	g.AddIssue("Export the report as CSV", "", "type:story", "status:ready")                                            // #1
	g.AddIssue("Sign in with a passkey", "", "type:story", "status:in-progress")                                        // #2
	g.AddIssue("Make Ready stories easy to find", "", "type:feature", "status:ready")                                   // #3: not a story
	g.AddPR("Export the report as CSV", "Refs #1", "aayudha-bot", "story/1-export-csv", strings.Repeat("a", 40), "dev") // #4
	return repo, g
}

// TestHomeModes golden-tests the home screen in every output mode (WP1.1b):
// machines (a pipe, CI, agents) and --json get stable text, a person gets
// Plain or Rich, and only Rich carries colour, symbols, or the seat's voice.
func TestHomeModes(t *testing.T) {
	repo, g := homeRepo(t)
	now := time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC)
	tty := func(any) bool { return true }
	modes := []struct {
		name string
		env  map[string]string
		tty  bool
		args []string
	}{
		{"machine", nil, false, nil},
		{"json", nil, true, []string{"--json"}},
		{"plain", nil, true, []string{"--plain"}},
		{"plain-config", map[string]string{"AAYU_UI__PLAIN": "true"}, true, nil},
		{"term-dumb", map[string]string{"TERM": "dumb"}, true, nil},
		{"no-color", map[string]string{"NO_COLOR": "1", "TERM": "xterm-256color"}, true, nil},
		{"rich", map[string]string{"TERM": "xterm-256color"}, true, nil},
		{"agent", map[string]string{"AAYU_AGENT": "1"}, true, nil},
		{"ci", map[string]string{"CI": "true"}, true, nil},
	}
	for _, m := range modes {
		t.Run(m.name, func(t *testing.T) {
			env := map[string]string{"AAYU_GITHUB_TOKEN": "t"}
			for k, v := range m.env {
				env[k] = v
			}
			opts := Options{Dir: repo, GitHubAPI: g.URL(), GitHubGap: -1, Now: func() time.Time { return now }, UIStatic: true, Assets: testAssets(t)}
			if m.tty {
				opts.IsTerminal = tty
			}
			code, out, errs := run(t, env, opts, append([]string{"--repo", "o/r"}, m.args...)...)
			if code != 0 {
				t.Fatalf("exit %d\n%s%s", code, out, errs)
			}
			got := "-- stdout --\n" + out + "-- stderr --\n" + errs
			golden := filepath.Join("testdata", "home", m.name+".golden")
			if *update {
				_ = os.MkdirAll(filepath.Dir(golden), 0o755)
				_ = os.WriteFile(golden, []byte(got), 0o644)
			}
			want, _ := os.ReadFile(golden)
			if got != string(want) {
				t.Errorf("home (%s) differs from %s (go test ./internal/cli -run TestHomeModes -update):\n%s", m.name, golden, got)
			}
			if m.name != "rich" && m.name != "no-color" && strings.IndexFunc(got, func(r rune) bool { return r > 127 }) >= 0 {
				t.Errorf("%s: non-ASCII symbols outside Rich:\n%s", m.name, got)
			}
			if m.name != "rich" && strings.Contains(got, "\x1b[") {
				t.Errorf("%s: escape sequences outside Rich with colour:\n%s", m.name, got)
			}
		})
	}
	if len(g.Writes()) != 0 {
		t.Errorf("the home screen wrote to GitHub: %v", g.Writes())
	}
}

func TestNextForAgentsAndScripts(t *testing.T) {
	repo, g := homeRepo(t)
	opts := Options{Dir: repo, GitHubAPI: g.URL(), GitHubGap: -1}
	env := map[string]string{"AAYU_GITHUB_TOKEN": "t"}
	code, out, _ := run(t, env, opts, "next", "--repo", "o/r")
	if code != 0 || strings.SplitN(out, "\n", 2)[0] != "aayu ship 4" {
		t.Fatalf("a person's next: exit %d\n%s", code, out)
	}
	// An agent is never offered ship (human only): it continues the build.
	agent := map[string]string{"AAYU_GITHUB_TOKEN": "t", "AAYU_AGENT": "1"}
	code, out, _ = run(t, agent, opts, "next", "--repo", "o/r", "--json")
	var st journey.Step
	if code != 0 || json.Unmarshal([]byte(out), &st) != nil || st.Command != "aayu build 2" || st.HumanOnly {
		t.Fatalf("agent's next: exit %d\n%s", code, out)
	}
	// The status line and plugins read the cached home screen.
	b, err := os.ReadFile(filepath.Join(repo, ".aayudha", "state", "journey.json"))
	if err != nil || !strings.Contains(string(b), `"command": "aayu build 2"`) {
		t.Fatalf("journey cache: %v\n%s", err, b)
	}
}

func TestHomeOutsideARepoAndWithoutGitHub(t *testing.T) {
	code, out, _ := run(t, nil, Options{}, "--plain")
	if code != 0 || !strings.Contains(out, "You are not in a git repository.") || !strings.Contains(out, "Next: aayu tour") {
		t.Fatalf("outside a repository: exit %d\n%s", code, out)
	}
	repo, _ := homeRepo(t)
	code, out, _ = run(t, nil, Options{Dir: repo}, "--repo", "o/r")
	if code != 0 || !strings.Contains(out, "GitHub: not signed in") || !strings.Contains(out, "Next: gh auth login") {
		t.Fatalf("no token: exit %d\n%s", code, out)
	}
}

// Every error ends with the exact command to run next, in text and JSON, and
// the exit codes do not change.
func TestErrorsEndWithNext(t *testing.T) {
	tests := []struct {
		args []string
		code int
		next string
	}{
		{[]string{"ready"}, ExitUsage, "aayu ready --help"},
		{[]string{"redy"}, ExitUsage, "aayu ready --help"},
		{[]string{"tour"}, ExitBlocked, "aayu init"},
		{[]string{"config", "validate", "--set", "budget.per_week_usd=1"}, ExitUsage, "aayu config --help"},
	}
	for _, tt := range tests {
		code, _, errs := run(t, nil, Options{}, tt.args...)
		if code != tt.code || !strings.HasSuffix(errs, "\n  next: "+tt.next+"\n") {
			t.Errorf("aayu %s: exit %d, stderr %q; want exit %d ending with next: %s", strings.Join(tt.args, " "), code, errs, tt.code, tt.next)
		}
		code, out, _ := run(t, nil, Options{}, append(tt.args, "--json")...)
		var e struct {
			Error struct {
				Code int
				Next string
			}
		}
		if code != tt.code || json.Unmarshal([]byte(out), &e) != nil || e.Error.Next != tt.next || e.Error.Code != tt.code {
			t.Errorf("aayu %s --json: exit %d, stdout %q", strings.Join(tt.args, " "), code, out)
		}
	}
}

func TestVoiceReadsThePersona(t *testing.T) {
	persona := "name = \"Kiran\"\npurpose = \"Keeps work flowing to a proven merge: elevates the concept.\"\nquestions = [\"Who owns the next step, and by when?\", \"What does the pack say?\"]\n"
	v := voiceOf(t, map[string]string{"seats/engineering-manager/persona.toml": persona}, "status")
	if v.Name != "Kiran" || v.Intro != "Keeps work flowing to a proven merge." || v.Speaker() != "Kiran (Engineering Manager)" {
		t.Errorf("persona: %+v", v)
	}
	if v.SignOff != "Who owns the next step, and by when?" && v.SignOff != "What does the pack say?" {
		t.Errorf("sign-off is not a signature question: %q", v.SignOff)
	}
	if again := voiceOf(t, map[string]string{"seats/engineering-manager/persona.toml": persona}, "status"); again.SignOff != v.SignOff {
		t.Errorf("the sign-off of one command changed between runs")
	}
	// No persona: the charter's purpose speaks; nothing is invented.
	if v := voiceOf(t, map[string]string{"seats/engineering-manager/CHARTER.md": "## Purpose\n\nOwns flow and the merge decision: work moves.\n"}, "status"); v.Intro != "Owns flow and the merge decision." || v.SignOff != "" || v.Name != "" {
		t.Errorf("charter fallback: %+v", v)
	}
}

// testAssets is a fixed charter with a persona section, so the goldens show a
// seat speaking without churning when the real charters change.
func testAssets(t *testing.T) fstest.MapFS {
	t.Helper()
	return fstest.MapFS{"seats/engineering-manager/persona.toml": {Data: []byte("name = \"Mira\"\npurpose = \"Here is where the work stands: and the rest.\"\nquestions = [\"Small steps, real proof?\"]\n")}}
}

func voiceOf(t *testing.T, files map[string]string, command string) ui.Voice {
	t.Helper()
	fsys := fstest.MapFS{}
	for k, v := range files {
		fsys[k] = &fstest.MapFile{Data: []byte(v)}
	}
	e, _ := newEnv(IO{}, Options{Assets: fsys, Getenv: func(string) string { return "" }, Dir: t.TempDir()})
	return e.voice(verbs.EngineeringManager, command)
}
