package delegate

import (
	"context"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// Live checks of each receiver, behind AAYU_LIVE=1 and the receiver's own
// credentials; each skips cleanly without them (WP5.5b). No receiver was
// connected when they were written: the founder's actions are in
// private research note agent-delegation-and-review.md §10.
//
//	AAYU_LIVE=1 AAYU_LIVE_CURSOR_KEY=… [AAYU_LIVE_DELEGATE_REPO=owner/scratch] go test ./internal/delegate -run Live
//	AAYU_LIVE=1 AAYU_LIVE_CODEX_ENV=env_… (codex logged in)                  go test ./internal/delegate -run Live
//	AAYU_LIVE=1 AMP_API_KEY=sgamp_… [AAYU_LIVE_DELEGATE_REPO=…]             go test ./internal/delegate -run Live
func live(t *testing.T, vars ...string) {
	t.Helper()
	if os.Getenv("AAYU_LIVE") != "1" {
		t.Skip("set AAYU_LIVE=1 and the receiver's credentials")
	}
	for _, v := range vars {
		if os.Getenv(v) == "" {
			t.Skipf("set %s", v)
		}
	}
}

func run(ctx context.Context, dir string, env []string, stdin, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir, cmd.Env = dir, env
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// Cursor: the key reaches the API; with a scratch repository, a hand-off
// starts an agent whose run reports a status.
func TestLiveCursor(t *testing.T) {
	live(t, "AAYU_LIVE_CURSOR_KEY")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	c := CursorAPI{BaseURL: "https://api.cursor.com", Key: os.Getenv("AAYU_LIVE_CURSOR_KEY"), HTTP: &http.Client{Timeout: time.Minute}}
	if err := c.do(ctx, http.MethodGet, "/v1/me", nil, nil); err != nil {
		t.Fatalf("GET /v1/me: %v", err)
	}
	repo := os.Getenv("AAYU_LIVE_DELEGATE_REPO")
	if repo == "" {
		t.Skip("set AAYU_LIVE_DELEGATE_REPO to a scratch repository for a hand-off")
	}
	task, err := c.Handoff(ctx, Handoff{Repo: repo, Base: "main", Story: 1, Title: "live smoke",
		Brief: "Add a file LIVE_SMOKE.md containing the word ok, and open a pull request whose body starts with `Refs #1`."})
	if err != nil {
		t.Fatal(err)
	}
	st, err := c.Status(ctx, State{Story: 1, Task: task.ID})
	if err != nil || st.Status == "" {
		t.Fatalf("status: %v %+v", err, st)
	}
	t.Logf("cursor agent %s: %s", task.URL, st.Status)
}

// Codex: the CLI is logged in and the cloud environment accepts a task.
func TestLiveCodex(t *testing.T) {
	live(t, "AAYU_LIVE_CODEX_ENV")
	if _, err := exec.LookPath("codex"); err != nil {
		t.Skip("codex is not on PATH")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	c := CodexCloud{Exec: run, EnvID: os.Getenv("AAYU_LIVE_CODEX_ENV"), Env: os.Environ()}
	task, err := c.Handoff(ctx, Handoff{Base: "main", Brief: "Reply with a one-line summary of the repository; change no file."})
	if err != nil {
		t.Fatal(err)
	}
	st, err := c.Status(ctx, State{Task: task.ID})
	t.Logf("codex task %s: %+v %v", task.URL, st.Status, err)
}

// Amp: an orb thread starts for a scratch repository.
func TestLiveAmp(t *testing.T) {
	live(t, "AMP_API_KEY", "AAYU_LIVE_DELEGATE_REPO")
	if _, err := exec.LookPath("amp"); err != nil {
		t.Skip("amp is not on PATH")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	a := AmpCLI{Exec: run, OrbSize: "a1.tiny", Env: os.Environ()}
	task, err := a.Handoff(ctx, Handoff{Repo: os.Getenv("AAYU_LIVE_DELEGATE_REPO"), Story: 1, Title: "live smoke", Brief: "Reply ok; change no file."})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("amp thread %s", task.URL)
}

// Local headless mode with only the receiver's CLI installed and nothing set
// up in the cloud (WP5.5c): each CLI on PATH runs the documented headless
// command (LocalArgs) in a scratch git repository with the scrubbed
// environment, writes one file, and reports what it proved. A CLI that is not
// signed in skips, naming the receiver.
//
//	AAYU_LIVE=1 go test ./internal/delegate -run LiveLocal -v
func TestLiveLocalReceivers(t *testing.T) {
	live(t)
	for _, r := range Receivers {
		t.Run(r, func(t *testing.T) {
			bin, ok := LookLocal(r, exec.LookPath)
			if !ok {
				t.Skipf("%s: no %s on PATH", r, strings.Join(LocalBins[r], " or "))
			}
			wt := t.TempDir()
			if out, err := run(context.Background(), wt, os.Environ(), "", "git", "init", "--quiet"); err != nil {
				t.Fatalf("git init: %v %s", err, out)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()
			gh := t.TempDir()
			prompt := "Create a file named LIVE_SMOKE.md in the current directory containing exactly the word ok. Then end your final message with this section:\n\n" +
				ProvedHeading + "\n1. proved: <how you checked that LIVE_SMOKE.md contains ok>\n"
			out, err := run(ctx, wt, ScrubEnv(os.Environ(), r, gh), "", bin, LocalArgs(r, bin, wt, prompt)...)
			if err != nil && regexp.MustCompile(`(?i)authenticat|log ?in|sign ?in|api key|unauthori`).MatchString(out) {
				t.Skipf("%s is not signed in: %s", r, clip(out, 200))
			}
			if err != nil {
				t.Fatalf("%s %v: %v\n%s", bin, LocalArgs(r, bin, wt, "…"), err, clip(out, 2000))
			}
			b, _ := os.ReadFile(filepath.Join(wt, "LIVE_SMOKE.md"))
			if strings.TrimSpace(string(b)) != "ok" || Proved(out, prompt) == "" {
				t.Fatalf("%s: LIVE_SMOKE.md %q, proved %q\n%s", r, b, Proved(out, prompt), clip(out, 2000))
			}
			t.Logf("%s (%s) proved: %s", r, bin, Proved(out, prompt))
		})
	}
}
