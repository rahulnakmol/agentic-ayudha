package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/delegate"
	"github.com/rahulnakmol/agentic-ayudha/internal/ready"
)

// Story #1012: a local Kanha takes its events at each check on its own
// machine, pauses its Feature while that machine is off, and an event host's
// wake starts the next stories or only records them.

// setMachine records Feature f's Kanha as running on machine (login@machine).
func (m *missionWorld) setMachine(f int, machine string) {
	m.t.Helper()
	var a ready.Approval
	if err := json.Unmarshal([]byte(git(m.t, m.bare, "show", "ledger:"+ready.ApprovalPath(f))), &a); err != nil || a.Parent == nil {
		m.t.Fatalf("approved.json of #%d: %v", f, err)
	}
	a.Parent.Machine = machine
	data, _ := json.MarshalIndent(a, "", "  ")
	putLedger(m.t, m.bare, ready.ApprovalPath(f), string(data))
}

// kanhaEvent records text for Feature f's Kanha on story n at at, as
// tellKanha does for a local Kanha.
func (m *missionWorld) kanhaEvent(f, n int, at time.Time, text string) {
	m.t.Helper()
	st := delegate.State{Story: n, Feature: f}
	if raw, err := exec.Command("git", "-C", m.bare, "show", "ledger:"+delegate.LedgerPath(n)).Output(); err == nil {
		_ = json.Unmarshal(raw, &st)
	}
	st.Events = append(st.Events, delegate.Event{At: at.UTC().Format(time.RFC3339), Status: "kanha", Detail: text + " (recorded)", Text: text})
	data, _ := json.Marshal(st)
	putLedger(m.t, m.bare, delegate.LedgerPath(n), string(data))
}

// laptop is a machine that is never the one the tests run on: the Kanha's machine when it runs elsewhere.
var laptop = func() string {
	if h, _ := os.Hostname(); h == "laptop" {
		return "rahulnakmol@laptop-2"
	}
	return "rahulnakmol@laptop"
}()

func (m *missionWorld) wake(f int) (int, string, string) {
	m.t.Helper()
	m.calls = nil
	return run(m.t, m.person, m.opts, "mission", "wake", fmt.Sprint(f), "--repo", "o/r")
}

// inbox opens `aayu inbox` as the person and quits; it returns what the inbox said before its first step.
func (m *missionWorld) inbox() string {
	m.t.Helper()
	m.calls = nil
	code, out, errOut := runIn(m.t, m.person, m.opts, "q", "inbox", "--repo", "o/r")
	if code != 0 {
		m.t.Fatalf("inbox: exit %d: %s%s", code, out, errOut)
	}
	return errOut
}

// A wake before Start starts nothing. A local Kanha whose CLI fails, or names
// no session, is not recorded. A resume that fails leaves its events for the
// next check, and the inbox names the mission it could not check. An event
// host whose tool's cloud agent is not connected only records the wake.
func TestLocalKanhaFailures(t *testing.T) {
	m := newMissionWorld(t)
	now := time.Now()
	m.opts.Now = func() time.Time { return now }
	m.featureWith(889, 20)
	if code, out, errOut := m.wake(889); code != ExitBlocked || !strings.Contains(errOut, "The mission of #889 has not started: a person starts it. Nothing starts.") {
		t.Fatalf("wake before Start: exit %d: %s%s", code, out, errOut)
	}
	if code, out, errOut := m.approve(889); code != 0 {
		t.Fatalf("approve 889: exit %d: %s%s", code, out, errOut)
	}
	m.opts.LocalKanha = nil // Codex's own CLI, through the faked exec
	m.setTool("codex")
	codexOut, codexErr := "", error(nil)
	var during func() // runs while a codex call is in progress
	ex := m.opts.DelegateExec
	m.opts.DelegateExec = func(ctx context.Context, dir string, env []string, stdin, name string, args ...string) (string, error) {
		if name == "/usr/bin/codex" {
			if during != nil {
				during()
			}
			_, _ = ex(ctx, dir, env, stdin, "agent", args...) // records the call; "agent" answers ""
			return codexOut, codexErr
		}
		return ex(ctx, dir, env, stdin, name, args...)
	}
	for _, c := range []struct{ out, err, want string }{
		{"Reading prompt from stdin...\n", "", "Codex: no session ID in the CLI's output: Reading prompt from stdin..."},
		{"", "codex: exit status 1: not signed in", "codex: exit status 1: not signed in"},
	} {
		codexOut, codexErr = c.out, nil
		if c.err != "" {
			codexErr = errors.New(c.err)
		}
		code, out, errOut := m.mission(m.person, "rahulnakmol", "start", 889)
		if code != ExitExternal || !strings.Contains(errOut, "The mission of #889 runs on codex, and no Kanha of it is recorded: "+c.want) {
			t.Fatalf("start with %q: exit %d: %s%s", c.want, code, out, errOut)
		}
	}
	// The mission runs with no Kanha recorded: show names none, and says to start it again.
	if code, out, errOut := run(t, m.person, m.opts, "mission", "show", "889", "--repo", "o/r"); code != 0 || !strings.Contains(out, "running: started by @rahulnakmol") || strings.Contains(out, "Kanha") {
		t.Fatalf("show with no Kanha: exit %d: %s%s", code, out, errOut)
	}
	codexOut, codexErr = `{"type":"thread.started","thread_id":"s-123"}`+"\n", nil
	if code, out, errOut := m.mission(m.person, "rahulnakmol", "start", 889); code != 0 || m.parent(889).Session != "s-123" {
		t.Fatalf("start: exit %d, parent %+v: %s%s", code, m.parent(889), out, errOut)
	}
	// While a wake resumes the Kanha, the inbox opens: it resumes nothing, so the session runs one turn at a time.
	m.kanhaEvent(889, 20, now.Add(-10*time.Minute), "merged #20 (PR #41): run aayu mission next 889")
	said := ""
	during = func() { during = nil; _, _, said = runIn(t, m.person, m.opts, "q", "inbox", "--repo", "o/r") }
	if code, out, errOut := m.wake(889); code != 0 || !strings.Contains(out, "resumed Codex session s-123 with 1 event(s)") || len(m.callLog()) != 1 ||
		!strings.Contains(said, "the Kanha of #889: a check here runs since ") || !strings.Contains(said, "; this one resumes nothing") {
		t.Fatalf("a wake with the inbox opened during its resume: exit %d, calls %q: %q%s\ninbox: %s", code, m.callLog(), out, errOut, said)
	}
	during = nil
	// A lock left by a check that died (older than the local limit) does not hold the next check.
	lock := filepath.Join(m.dir, ".aayudha", "state", "kanha", "889.json.lock")
	if err := os.WriteFile(lock, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(lock, time.Now().Add(-2*time.Hour), time.Now().Add(-2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	// The resume fails: the wake says so, the check records nothing, and the inbox names the mission.
	now = now.Add(5 * time.Minute)
	last := m.parent(889).LastCheck
	m.kanhaEvent(889, 20, now.Add(-time.Minute), "merged #20 (PR #41): run aayu mission next 889")
	codexErr = errors.New("codex: exit status 1: session s-123 not found")
	if code, out, errOut := m.wake(889); code != ExitExternal || !strings.Contains(errOut, "session s-123 not found") || m.parent(889).LastCheck != last {
		t.Fatalf("a failed resume: exit %d, last_check %q: %s%s", code, m.parent(889).LastCheck, out, errOut)
	}
	if said := m.inbox(); !strings.Contains(said, "the Kanha of #889: not checked: ") || !strings.Contains(said, "session s-123 not found") || m.parent(889).LastCheck != last {
		t.Fatalf("inbox open with a failed resume: %s", said)
	}
	look := m.opts.LookPath
	m.opts.LookPath = func(string) (string, error) { return "", os.ErrNotExist } // Codex's CLI was removed
	now = now.Add(5 * time.Minute)
	if code, out, errOut := m.wake(889); code != ExitExternal || !strings.Contains(errOut, "Codex's CLI is not on this machine's PATH") {
		t.Fatalf("a resume with no CLI: exit %d: %s%s", code, out, errOut)
	}
	m.opts.LookPath = look
	// An event host on codex, with Codex cloud connected, while the Kanha's laptop is off and an event waits: the mission is paused, so it records only.
	m.setMachine(889, laptop)
	if code, out, errOut := m.wake(889); code != 0 || !strings.Contains(out, "#889 wake: recorded only: mission next: #889 paused: its Kanha runs on "+laptop+", which is off") || len(m.callLog()) != 0 {
		t.Fatalf("wake with the laptop off: exit %d, calls %q: %q%s", code, m.callLog(), out, errOut)
	}
	// An event host on codex with no Codex cloud environment records the wake only.
	delete(m.person, "AAYU_DELEGATE__CODEX_ENV")
	if code, out, errOut := m.wake(889); code != 0 || !strings.HasPrefix(out, "#889 wake: recorded only: codex's cloud agent is not connected here: ") || len(m.callLog()) != 0 {
		t.Fatalf("wake with no cloud agent: exit %d, calls %q: %q%s", code, m.callLog(), out, errOut)
	}
}

// Criteria 6, 7, and 8. On cursor with no Cursor API key the Kanha is a
// local Cursor chat. With the default check_minutes it takes an event at the
// check 5 minutes after the last one, never before, or at once when the
// inbox opens. While its machine is off, the plan view says #889 is paused,
// and no story of #889 starts.
func TestLocalKanhaPausesWhileMachineOff(t *testing.T) {
	m := newMissionWorld(t)
	m.opts.LocalKanha = nil // the tool's own CLI, through the faked exec
	m.opts.LookPath = func(name string) (string, error) {
		if name == "agent" {
			return "agent", nil
		}
		return "", os.ErrNotExist
	}
	delete(m.person, "AAYU_SECRET_CURSOR_API_KEY")
	ex, limits := m.opts.DelegateExec, []time.Duration{} // each local CLI run's limit: [delegate] local_timeout_minutes, else 60 minutes
	m.opts.DelegateExec = func(ctx context.Context, dir string, env []string, stdin, name string, args ...string) (string, error) {
		dl, ok := ctx.Deadline()
		if !ok {
			dl = time.Now().Add(1000 * time.Hour)
		}
		limits = append(limits, time.Until(dl).Round(time.Minute))
		return ex(ctx, dir, env, stdin, name, args...)
	}
	m.featureWith(889, 20, 21)
	m.calls = nil
	m.start(889, "rahulnakmol", "cursor")
	host, _ := os.Hostname()
	if p := m.parent(889); p.Mode != "local" || p.Session != "c-456" || p.Machine != "rahulnakmol@"+host {
		t.Fatalf("parent %+v: want the local Cursor chat c-456 on rahulnakmol@%s", p, host)
	}
	if calls, _ := m.cursorCalls(); len(calls) != 0 || !slices.Equal(m.callLog(), []string{"agent create-chat", "agent -p --resume=c-456 --force --trust <brief>"}) {
		t.Fatalf("Cursor API calls %q, CLI calls %q: want only a local chat", calls, m.callLog())
	}

	// Criterion 6.
	t0 := time.Date(2026, 10, 27, 14, 0, 0, 0, time.UTC)
	now := t0
	m.opts.Now = func() time.Time { return now }
	if code, out, errOut := m.wake(889); code != 0 || out != "#889 wake: the Kanha of #889: no event since its last check\n" || len(m.callLog()) != 0 {
		t.Fatalf("first check: exit %d, calls %q: %q%s", code, m.callLog(), out, errOut)
	}
	merged := "merged #20 (PR #41): run aayu mission next 889"
	m.kanhaEvent(889, 20, t0.Add(time.Minute), merged)
	now = t0.Add(5*time.Minute - time.Second)
	if code, out, errOut := m.wake(889); code != 0 || !strings.Contains(out, "not due; its next check is at 2026-10-27T14:05:00Z") || len(m.callLog()) != 0 {
		t.Fatalf("a second before the next check: exit %d, calls %q: %q%s", code, m.callLog(), out, errOut)
	}
	now = t0.Add(5 * time.Minute) // the event arrived 4 minutes ago
	if code, out, errOut := m.wake(889); code != 0 || !strings.Contains(out, "resumed Cursor session c-456 with 1 event(s)") ||
		!slices.Equal(m.callLog(), []string{"agent -p --resume=c-456 --force --trust <brief>"}) {
		t.Fatalf("the next check: exit %d, calls %q: %q%s", code, m.callLog(), out, errOut)
	}
	// The events reach the session marked as data: a stop's words or a CLI's error output can be in them.
	if got := m.briefs[len(m.briefs)-1]; !strings.HasPrefix(got, `<untrusted source="aayu: the events of your mission since your last check">`+"\n"+merged+"\n</untrusted>") {
		t.Fatalf("the resume's prompt:\n%s", got)
	}
	if want := []time.Duration{60 * time.Minute, 60 * time.Minute, 60 * time.Minute}; !slices.Equal(limits, want) {
		t.Fatalf("the local CLI runs' limits %v, want %v: create-chat, the brief, and the resume", limits, want)
	}
	if p := m.parent(889); p.LastCheck != "2026-10-27T14:01:00Z" {
		t.Fatalf("last_check %q", p.LastCheck)
	}
	now = t0.Add(10 * time.Minute) // an event is taken once
	if code, out, _ := m.wake(889); code != 0 || !strings.Contains(out, "no event since its last check") || len(m.callLog()) != 0 {
		t.Fatalf("the check after: calls %q: %q", m.callLog(), out)
	}
	// Opening the inbox checks at once, 2 minutes after the last check. It also gives an event stamped in
	// the second of the last one given, whose write landed after that check read the ledger.
	blocked := "blocked #21: spec unclear: run aayu mission next 889"
	late := "answered: #20 resumes"
	m.kanhaEvent(889, 20, t0.Add(time.Minute), late)
	m.kanhaEvent(889, 21, t0.Add(11*time.Minute), blocked)
	now = t0.Add(12 * time.Minute)
	if said := m.inbox(); !strings.HasPrefix(said, "the Kanha of #889: resumed Cursor session c-456 with 2 event(s)\n") ||
		!slices.Equal(m.callLog(), []string{"agent -p --resume=c-456 --force --trust <brief>"}) || !strings.Contains(m.briefs[len(m.briefs)-1], "\n"+late+"\n"+blocked+"\n") ||
		strings.Contains(m.briefs[len(m.briefs)-1], merged) {
		t.Fatalf("inbox open: calls %q: %s\n%s", m.callLog(), said, m.briefs[len(m.briefs)-1])
	}
	show := func() string {
		t.Helper()
		code, out, errOut := run(t, m.person, m.opts, "mission", "show", "889", "--repo", "o/r")
		if code != 0 {
			t.Fatalf("mission show: exit %d: %s%s", code, out, errOut)
		}
		return out
	}
	// On the Kanha's own machine, an event left past one check says how to give it to the Kanha: the machine is on.
	m.kanhaEvent(889, 20, t0.Add(13*time.Minute), merged)
	now = t0.Add(19 * time.Minute)
	if out := show(); !strings.Contains(out, "\n#889 waits: its Kanha runs on this machine, and an event waits since 2026-10-27T14:13:00Z; run aayu mission wake 889, or open aayu inbox\n") ||
		strings.Contains(out, "which is off") {
		t.Fatalf("show on the Kanha's machine:\n%s", out)
	}

	// Criteria 7 and 8: the Kanha runs on his laptop, which is off when #20's PR merges.
	m.setMachine(889, laptop)
	m.status(21, "ready", "")
	now = t0.Add(17 * time.Minute) // within one check: not yet paused
	local := "#889's Kanha is local: it pauses while " + laptop + " is off\n"
	paused := "#889 paused: its Kanha runs on " + laptop + ", which is off; it resumes at the next check"
	if out := show(); !strings.Contains(out, local) || strings.Contains(out, paused) || !strings.Contains(out, "running: started by @rahulnakmol") {
		t.Fatalf("show within one check:\n%s", out)
	}
	now = t0.Add(19 * time.Minute)
	if out := show(); !strings.Contains(out, local) || !strings.Contains(out, "\n"+paused+"\n") {
		t.Fatalf("show with the laptop off:\n%s", out)
	}
	n := m.commits()
	if code, out, errOut := run(t, m.person, m.opts, "mission", "next", "889", "--repo", "o/r"); code != ExitBlocked ||
		!strings.Contains(errOut, paused+". Nothing starts.") || m.commits() != n || len(m.claims(889)) != 0 {
		t.Fatalf("mission next with the laptop off: exit %d, claims %v: %s%s", code, m.claims(889), out, errOut)
	}

	// With CURSOR_API_KEY set and no key in aayu's store, Cursor's cloud agent is connected here: the
	// Kanha is a cloud agent, and its calls carry that key.
	m.featureWith(890, 30)
	m.person["CURSOR_API_KEY"] = "env-key"
	m.calls = nil
	m.start(890, "rahulnakmol", "cursor")
	var auth any
	m.cursor.mu.Lock()
	if n := len(m.cursor.reqs); n > 0 {
		auth = m.cursor.reqs[n-1]["auth"]
	}
	m.cursor.mu.Unlock()
	if p := m.parent(890); p.Mode != "cloud" || p.Session != cursorKanhaAgent || auth != "Bearer env-key" || len(m.callLog()) != 0 {
		t.Fatalf("start with CURSOR_API_KEY: parent %+v, auth %q, CLI calls %q", p, auth, m.callLog())
	}
}

// Criterion 1: `aayu mission wake 889` on an event host. On the mission's
// tool with its cloud agent connected, it starts the next story and records
// the wake. When a start fails, or the session runs on another tool, it only
// records the next stories in the plan, claims none, and the mission's Kanha
// starts them.
func TestMissionWakeStartsOrRecordsOnly(t *testing.T) {
	m := newMissionWorld(t)
	m.featureWith(889, 20, 21)
	t0 := time.Now().Truncate(time.Second)
	m.opts.Now = func() time.Time { return t0 }
	m.start(889, "rahulnakmol", "codex")
	// The wakes come a minute after Start: the Kanha's events are those after its start's second.
	m.opts.Now = func() time.Time { return t0.Add(time.Minute) }
	m.setMachine(889, laptop) // the local Kanha is elsewhere: this is an event host
	m.readyStory(20)
	code, out, errOut := m.wake(889)
	if code != 0 || out != "#889 wake: started #20\n" || !slices.ContainsFunc(m.callLog(), func(c string) bool { return strings.HasPrefix(c, "codex cloud exec --env env-1") }) {
		t.Fatalf("wake on codex: exit %d, calls %q: %q%s", code, m.callLog(), out, errOut)
	}
	if st := m.state(20); st.Receiver != "codex" || st.Mode != "cloud" || st.Feature != 889 {
		t.Fatalf("#20's delegation: %+v", st)
	}
	// (a) The start fails: #21 is Ready again with no claim, and the wake records it.
	m.readyStory(21)
	ex := m.opts.DelegateExec
	m.opts.DelegateExec = func(ctx context.Context, dir string, env []string, stdin, name string, args ...string) (string, error) {
		if name == "codex" && args[0] == "cloud" {
			m.mu.Lock()
			m.calls = append(m.calls, "codex cloud (fails)")
			m.mu.Unlock()
			return "", fmt.Errorf("codex: exit status 1: not signed in")
		}
		return ex(ctx, dir, env, stdin, name, args...)
	}
	code, out, errOut = m.wake(889)
	if code != 0 || !strings.Contains(out, "#889 wake: recorded only: start of #21 failed:") || !strings.Contains(out, "the mission's Kanha starts the next stories") ||
		m.claims(889)[21] != 0 || !slices.Contains(m.labels(21), "status:ready") {
		t.Fatalf("wake with a failed start: exit %d, claims %v: %q%s", code, m.claims(889), out, errOut)
	}
	m.opts.DelegateExec = ex
	// #20's hand-off is an event the Kanha has not read: a wake leaves the plan's events_cursor (Story #1009) for it.
	if plan, _ := m.plan(889); plan.EventsCursor != "" {
		t.Fatalf("a wake moved events_cursor to %s", plan.EventsCursor)
	}
	// (b) A session on another tool (a Claude scheduled task for a Codex mission) starts nothing.
	m.setTool("claude")
	n := m.commits()
	code, out, errOut = m.wake(889)
	if want := "#889 wake: recorded only: this session runs on claude, and the mission runs on codex; the mission's Kanha starts the next stories\n"; code != 0 || out != want ||
		len(m.callLog()) != 0 || m.claims(889)[21] != 0 || m.commits() == n {
		t.Fatalf("wake on claude: exit %d, calls %q: %q%s", code, m.callLog(), out, errOut)
	}
	plan, _ := m.plan(889)
	if len(plan.HostWakes) != 3 || plan.HostWakes[0].Result != "started" || !slices.Equal(plan.HostWakes[0].Started, []int{20}) ||
		plan.HostWakes[1].Result != "record-only" || !slices.Equal(plan.HostWakes[1].Waiting, []int{21}) ||
		plan.HostWakes[2].Result != "record-only" || plan.HostWakes[2].Host != "claude" {
		t.Fatalf("the plan's wakes: %+v", plan.HostWakes)
	}
	m.setTool("codex") // the Kanha's own list still has #20's events
	if code, out, errOut := run(t, m.person, m.opts, "mission", "next", "889", "--repo", "o/r", "--json"); code != 0 || !strings.Contains(out, `"story": 20`) {
		t.Fatalf("the Kanha's mission next after the wakes: exit %d, want #20's events: %s%s", code, out, errOut)
	}
}
