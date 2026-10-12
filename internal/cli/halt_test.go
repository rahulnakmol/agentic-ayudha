package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
	"github.com/rahulnakmol/agentic-ayudha/internal/github/scratch"
	"github.com/rahulnakmol/agentic-ayudha/internal/notify"
)

func setVar(g *fake.GitHub, name, value string) {
	g.Mu.Lock()
	defer g.Mu.Unlock()
	if g.Variables == nil {
		g.Variables = map[string]string{}
	}
	g.Variables[name] = value
}

// WP5.7 accept: `halt` stops a running loop within one poll. An agent lane runs
// a verb whose handler loops until its context ends; `aayu halt` sets the
// repository variable mid-run, and the verb exits 4 no later than one poll
// interval after the halt. While halted, verbs refuse before their handler
// runs; `aayu resume` clears the halt (and the [budget] pause) and they run again.
func TestHaltStopsRunningLoop(t *testing.T) {
	g := fake.New(t, "o/r", "User")
	g.Viewer = "founder"
	dir := t.TempDir()
	sink := &notify.Fake{}
	const poll = 200 * time.Millisecond
	opts := Options{GitHubAPI: g.URL(), GitHubGap: -1, HaltPoll: poll, NotifySink: sink, Dir: dir}
	human := map[string]string{"AAYU_GITHUB_TOKEN": "t", "XDG_STATE_HOME": filepath.Join(dir, "state")}
	agent := map[string]string{"AAYU_GITHUB_TOKEN": "t", "AAYU_AGENT": "1", "XDG_STATE_HOME": filepath.Join(dir, "state")}

	started := make(chan struct{}, 1)
	var calls, ticks atomic.Int32
	withHandler(t, "build", func(ctx context.Context, _ *Env, _ Values) error {
		calls.Add(1)
		started <- struct{}{}
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(5 * time.Millisecond):
				ticks.Add(1)
			}
		}
	})

	type result struct {
		code   int
		errOut string
		at     time.Time
	}
	done := make(chan result, 1)
	go func() {
		code, _, errOut := run(t, agent, opts, "build", "7", "--repo", "o/r")
		done <- result{code, errOut, time.Now()}
	}()
	<-started
	time.Sleep(poll + poll/2) // the loop runs across at least one poll that finds no halt

	code, out, errOut := run(t, human, opts, "halt", "--repo", "o/r", "--reason", "runaway spend", "--json")
	haltedAt := time.Now()
	if code != 0 || !strings.Contains(out, `"halted": true`) {
		t.Fatalf("halt: exit %d %s %s", code, out, errOut)
	}
	var r result
	select {
	case r = <-done:
	case <-time.After(5 * poll):
		t.Fatal("the running verb did not stop after the halt")
	}
	if stop := r.at.Sub(haltedAt); r.code != ExitBlocked || stop > poll+150*time.Millisecond || !strings.Contains(r.errOut, "runaway spend") {
		t.Fatalf("running verb: exit %d %v after the halt (poll %v): %s", r.code, stop, poll, r.errOut)
	}
	if n := ticks.Load(); n < 10 {
		t.Errorf("the loop ticked %d times; it was not running when halted", n)
	}
	g.Mu.Lock()
	value := g.Variables[HaltVariable]
	g.Mu.Unlock()
	if !strings.HasPrefix(value, "runaway spend (by @founder at ") || len(sink.Pages) != 1 || !sink.Pages[0].Urgent {
		t.Errorf("variable %q, pages %+v", value, sink.Pages)
	}

	// Halted: a verb refuses before its handler runs; plumbing still works.
	before := calls.Load()
	if code, _, errOut := run(t, agent, opts, "build", "8", "--repo", "o/r"); code != ExitBlocked || calls.Load() != before || !strings.Contains(errOut, "aayu resume") {
		t.Errorf("verb while halted: exit %d, handler calls %d->%d: %s", code, before, calls.Load(), errOut)
	}
	if code, _, _ := run(t, agent, opts, "resume", "--repo", "o/r"); code != ExitBlocked {
		t.Errorf("resume by an agent: exit %d", code)
	}

	// Resume clears the variable and the [budget] breaker's pause file.
	pause := filepath.Join(dir, "state", "aayudha", "state", "paused")
	_ = os.MkdirAll(filepath.Dir(pause), 0o755)
	_ = os.WriteFile(pause, []byte("per_day"), 0o644)
	if code, _, errOut := run(t, human, opts, "resume", "--repo", "o/r"); code != 0 {
		t.Fatalf("resume: exit %d: %s", code, errOut)
	}
	g.Mu.Lock()
	_, still := g.Variables[HaltVariable]
	g.Mu.Unlock()
	if _, err := os.Stat(pause); still || err == nil {
		t.Errorf("after resume: variable present %v, pause file err %v", still, err)
	}
	withHandler(t, "build", func(context.Context, *Env, Values) error { return nil })
	if code, _, errOut := run(t, agent, opts, "build", "9", "--repo", "o/r"); code != 0 {
		t.Errorf("verb after resume: exit %d: %s", code, errOut)
	}
}

// In a workflow the job passes vars.AAYUDHA_HALT through the environment: set
// means halted without asking the API, empty means not halted. The host
// pre-tool hook blocks agent tool calls while halted.
func TestHaltFromWorkflowAndHook(t *testing.T) {
	withHandler(t, "status", func(context.Context, *Env, Values) error { return nil })
	if code, _, errOut := run(t, map[string]string{HaltVariable: "incident 12"}, Options{}, "status", "--ci"); code != ExitBlocked || !strings.Contains(errOut, "incident 12") {
		t.Errorf("CI with %s set: exit %d: %s", HaltVariable, code, errOut)
	}
	if code, _, errOut := run(t, nil, Options{Environ: []string{HaltVariable + "="}}, "status", "--ci"); code != 0 {
		t.Errorf("CI with %s empty: exit %d: %s", HaltVariable, code, errOut)
	}

	g := fake.New(t, "o/r", "User")
	setVar(g, HaltVariable, "freeze")
	dir := t.TempDir()
	env := map[string]string{"AAYU_GITHUB_TOKEN": "t", "XDG_STATE_HOME": filepath.Join(dir, "state")}
	opts := Options{GitHubAPI: g.URL(), Dir: dir}
	code, out, errOut := run(t, env, opts, "hook", "pre-tool", "--host", "cursor", "--repo", "o/r")
	if code != 2 || !strings.Contains(out, `"permission":"deny"`) || !strings.Contains(errOut, "freeze") {
		t.Errorf("pre-tool while halted: exit %d %s %s", code, out, errOut)
	}
	// The answer is cached for one poll: a second tool call does not ask GitHub again.
	g.Reset()
	if code, _, _ := run(t, env, opts, "hook", "pre-tool", "--host", "claude", "--repo", "o/r"); code != 2 || len(g.Requests) != 0 {
		t.Errorf("cached pre-tool: exit %d, %d requests", code, len(g.Requests))
	}
}

// TestHaltLive repeats the running-loop check against GitHub: a private
// aayu-scratch-* repository (created with AAYU_LIVE_ADMIN_TOKEN when set,
// deleted afterwards), the real repository variable, and a 3 s poll.
func TestHaltLive(t *testing.T) {
	if os.Getenv("AAYU_LIVE") != "1" {
		t.Skip("live test: set AAYU_LIVE=1")
	}
	tok := scratch.Token(t)
	admin := os.Getenv("AAYU_LIVE_ADMIN_TOKEN")
	if admin == "" {
		admin = tok
	}
	repo := scratch.Repo(t, admin)
	const poll = 3 * time.Second
	dir := t.TempDir()
	opts := Options{HaltPoll: poll, Dir: dir}
	env := map[string]string{"AAYU_GITHUB_TOKEN": tok, "XDG_STATE_HOME": filepath.Join(dir, "state")}
	agent := map[string]string{"AAYU_GITHUB_TOKEN": tok, "AAYU_AGENT": "1", "XDG_STATE_HOME": filepath.Join(dir, "state")}
	started := make(chan struct{}, 1)
	withHandler(t, "build", func(ctx context.Context, _ *Env, _ Values) error {
		started <- struct{}{}
		<-ctx.Done()
		return ctx.Err()
	})
	done := make(chan time.Time, 1)
	var code int
	var errOut string
	go func() {
		code, _, errOut = run(t, agent, opts, "build", "1", "--repo", repo)
		done <- time.Now()
	}()
	<-started
	time.Sleep(poll + time.Second)
	if c, _, e := run(t, env, opts, "halt", "--repo", repo, "--reason", "live test"); c != 0 {
		t.Fatalf("halt: exit %d: %s", c, e)
	}
	haltedAt := time.Now()
	t.Cleanup(func() { run(t, env, opts, "resume", "--repo", repo) })
	select {
	case at := <-done:
		t.Logf("running verb stopped %v after the halt (poll %v): exit %d: %s", at.Sub(haltedAt).Round(time.Millisecond), poll, code, strings.TrimSpace(errOut))
		if code != ExitBlocked || at.Sub(haltedAt) > poll+2*time.Second {
			t.Errorf("running verb: exit %d after %v", code, at.Sub(haltedAt))
		}
	case <-time.After(4 * poll):
		t.Fatal("the running verb did not stop after the halt")
	}
	if c, _, e := run(t, env, opts, "resume", "--repo", repo); c != 0 {
		t.Fatalf("resume: exit %d: %s", c, e)
	}
	if c, _, e := run(t, agent, opts, "status", "--streak", "--repo", repo); c != 0 {
		t.Errorf("a verb after resume: exit %d: %s", c, e)
	}
}
