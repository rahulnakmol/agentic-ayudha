package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
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
	opts := Options{IsTerminal: stdinTTY, GitHubAPI: g.URL(), GitHubGap: -1, HaltPoll: poll, NotifySink: sink, Dir: dir}
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

// Bug #1101: a halt releases the mission claim of each story that does not
// run yet, in every mission, so after resume a list claims it again and a
// build with the old claim refuses; a story that runs keeps its claim; and a
// ledger that cannot be written never blocks the halt.
func TestHaltReleasesMissionClaims(t *testing.T) {
	m := newMissionWorld(t)
	now := time.Date(2026, 10, 30, 10, 0, 0, 0, time.Local)
	m.opts.Now = func() time.Time { return now }
	m.startedMission(889, []int{20, 21})
	m.startedMission(920, []int{930})
	for _, n := range []int{20, 21, 930} {
		m.status(n, "ready", "")
	}
	if code, out, errOut := m.next(889); code != 0 || out != "start #20\nstart #21\n" {
		t.Fatalf("mission next 889: exit %d: %q%s", code, out, errOut)
	}
	if code, out, errOut := m.next(920); code != 0 || out != "start #930\n" {
		t.Fatalf("mission next 920: exit %d: %q%s", code, out, errOut)
	}
	m.status(21, "in-progress", "") // #21's thread started before the halt
	// A gate fix's claim (a negative key) is not a story's: the halt leaves it to expire.
	fix := fmt.Sprintf(`{"story":-5,"token":"fix-5","at":%q}`, now.UTC().Format(time.RFC3339))
	putLedger(t, m.bare, MissionClaimsPath(889), git(t, m.bare, "show", "ledger:"+MissionClaimsPath(889))+"\n"+fix+"\n")

	// Criterion 1: the halt at 10:05 releases #20 and #930, each in its own mission; a dry run only says it would.
	now = now.Add(5 * time.Minute)
	// A dry run writes nothing; a story it cannot read (#930, gone here) keeps its claim, and the others are still released.
	n, gone := m.commits(), m.g.Issues[930]
	m.edit(func() { delete(m.g.Issues, 930) })
	if code, out, errOut := run(t, m.person, m.opts, "halt", "--reason", "release cut", "--repo", "o/r", "--dry-run"); code != 0 || m.commits() != n ||
		!strings.Contains(out, "would release 1 mission claim (#20)\nmission claims not released: could not read #930: ") {
		t.Fatalf("halt --dry-run with #930 unreadable: exit %d: %q%s", code, out, errOut)
	}
	m.edit(func() { m.g.Issues[930] = gone })
	code, out, errOut := run(t, m.person, m.opts, "halt", "--reason", "release cut", "--repo", "o/r")
	if code != 0 || !strings.Contains(out, "released 2 mission claims (#20, #930)\n") || strings.Contains(out, "not released") {
		t.Fatalf("halt: exit %d: %q%s", code, out, errOut)
	}
	if got := m.claims(889); len(got) != 2 || got[21] != 1 || got[-5] != 1 {
		t.Fatalf("claims of #889 after the halt: %v (criterion 4: #21 runs and keeps its claim; the gate fix keeps its own)", got)
	}
	if got := m.claims(920); len(got) != 0 {
		t.Fatalf("claims of #920 after the halt: %v", got)
	}

	// Criterion 3: after resume at 10:08, a build with the 10:00 claim refuses and starts nothing.
	now = now.Add(3 * time.Minute)
	if code, _, errOut := run(t, m.person, m.opts, "resume", "--repo", "o/r"); code != 0 {
		t.Fatalf("resume: exit %d: %s", code, errOut)
	}
	m.g.Reset()
	code, _, errOut = run(t, m.agent, m.opts, "build", "20", "--delegate", "amp", "--repo", "o/r")
	if code != ExitBlocked || !strings.Contains(errOut, "the claim of #20 was released by aayu halt at 10:05: run aayu mission next 889") ||
		slices.ContainsFunc(m.g.Writes(), func(r fake.Request) bool { return strings.Contains(r.Path, "/issues/20") }) {
		t.Fatalf("build 20 with the old claim: exit %d: %s", code, errOut)
	}
	// Once the old claim would have expired (10:16), the release no longer refuses: build's own checks apply.
	now = now.Add(8 * time.Minute)
	if _, _, errOut := run(t, m.agent, m.opts, "build", "20", "--delegate", "amp", "--repo", "o/r", "--dry-run"); strings.Contains(errOut, "released by aayu halt") {
		t.Fatalf("build 20 after the old claim expired: %s", errOut)
	}

	// Criteria 2 and 4: the list claims #20 again with a new claim, and never starts #21, which runs.
	if code, out, errOut := m.next(889); code != 0 || out != "start #20\n" {
		t.Fatalf("mission next 889 after resume: exit %d: %q%s", code, out, errOut)
	}
	if got := m.claims(889); len(got) != 3 || got[20] != 1 || got[21] != 1 {
		t.Fatalf("claims of #889 after the new list: %v", got)
	}
	if code, _, errOut := run(t, m.agent, m.opts, "build", "20", "--delegate", "amp", "--repo", "o/r", "--dry-run"); strings.Contains(errOut, "released by aayu halt") {
		t.Fatalf("build 20 with the new claim: exit %d: %s", code, errOut)
	}

	// Criterion 5: with the ledger unwritable, the halt is still set and says the claims were not released.
	hooks := t.TempDir()
	if err := os.WriteFile(filepath.Join(hooks, "pre-receive"), []byte("#!/bin/sh\necho ledger is read-only >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, m.bare, "config", "core.hooksPath", hooks) // the repository's own setting wins over a global hooksPath
	code, out, errOut = run(t, m.person, m.opts, "halt", "--reason", "release cut", "--repo", "o/r")
	m.g.Mu.Lock()
	halted := m.g.Variables[HaltVariable]
	m.g.Mu.Unlock()
	if code != 0 || halted == "" || !strings.Contains(out, "mission claims not released: ") || !strings.HasSuffix(out, "; they expire within 15 minutes\n") {
		t.Fatalf("halt with the ledger unwritable: exit %d, %s=%q: %q%s", code, HaltVariable, halted, out, errOut)
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
	opts := Options{IsTerminal: stdinTTY, HaltPoll: poll, Dir: dir} // resume is a person's act at a terminal
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
