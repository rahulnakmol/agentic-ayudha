package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/agentenv"
	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
)

// Story #1008: a stop, a block, a failed start, or an answered ask tells the
// Kanha. The missionWorld of Story #997: the mission of #889 runs on amp, its
// Kanha is the Amp thread kanhaAmpThread, and every receiver CLI is a fake.

// ampMission starts the mission of Feature f on amp with Ready stories, with
// Amp's cloud agent connected, and returns the line the Kanha gets for an
// event: its `amp threads continue` call.
func (m *missionWorld) ampMission(f int, stories ...int) func(event string) string {
	m.t.Helper()
	look := m.opts.LookPath
	m.opts.LookPath = func(name string) (string, error) {
		if name == "amp" {
			return "/usr/bin/amp", nil
		}
		return look(name)
	}
	m.agent["AMP_API_KEY"] = "amp-key"
	if err := (agentenv.Records{Cloud: map[string]agentenv.Record{agentenv.Amp: {Method: "cloud", Project: "o/r"}}}).Save(m.dir); err != nil {
		m.t.Fatal(err)
	}
	m.startedMission(f, stories)
	for _, n := range stories {
		m.readyStory(n)
	}
	if p := m.parent(f); p.Session != kanhaAmpThread {
		m.t.Fatalf("the Kanha of #%d: %+v", f, p)
	}
	return func(event string) string { return "amp threads continue " + kanhaAmpThread + " -ox " + event }
}

// labels are issue n's label names.
func (m *missionWorld) labels(n int) []string {
	m.g.Mu.Lock()
	defer m.g.Mu.Unlock()
	var out []string
	for _, l := range m.g.Issues[n].Labels {
		out = append(out, l.Name)
	}
	return out
}

// kanhaEvents are the Kanha events in delegate/<n>/state.json.
func (m *missionWorld) kanhaEvents(n int) []string {
	var out []string
	for _, ev := range m.state(n).Events {
		if ev.Status == "kanha" {
			out = append(out, ev.Detail)
		}
	}
	return out
}

// Criterion 1: story #30's builder stops it because its spec is unclear: #30
// is blocked, and the Kanha of #889 is told to run the list again.
func TestBuildStopTellsKanha(t *testing.T) {
	m := newMissionWorld(t)
	tell := m.ampMission(889, 30)
	m.kanhaBuild(30, "amp")
	calls := len(m.callLog())
	code, out, errOut := run(t, m.agent, m.opts, "build", "30", "--stop", "spec unclear", "--repo", "o/r", "--json")
	if code != ExitBlocked || !slices.Contains(m.labels(30), "status:blocked") {
		t.Fatalf("build 30 --stop: exit %d, labels %v: %s%s", code, m.labels(30), out, errOut)
	}
	want := "blocked #30: spec unclear: run aayu mission next 889"
	if got := m.callLog()[calls:]; !slices.Equal(got, []string{tell(want)}) {
		t.Fatalf("calls after the stop: %q", got)
	}
	if got := m.kanhaEvents(30); !slices.Equal(got, []string{want + " (sent)"}) {
		t.Fatalf("delegate/30/state.json Kanha events: %q", got)
	}
	if !strings.Contains(out, "the Kanha of #889: "+want+" (sent)") {
		t.Fatalf("the stop does not say the Kanha was told: %s", out)
	}
	// A block outside a started mission tells nobody: #950's Feature #874 is approved and not started.
	m.featureWith(874, 950)
	if code, out, errOut := m.approve(874); code != 0 {
		t.Fatalf("approve 874: exit %d: %s%s", code, out, errOut)
	}
	m.readyStory(950)
	m.kanhaBuild(950, "amp")
	calls = len(m.callLog())
	if code, _, errOut := run(t, m.agent, m.opts, "build", "950", "--stop", "spec unclear", "--repo", "o/r"); code != ExitBlocked || len(m.callLog()) != calls || len(m.kanhaEvents(950)) != 0 {
		t.Fatalf("build 950 --stop: exit %d, calls %q: %s", code, m.callLog()[calls:], errOut)
	}
}

// Criterion 2: the subthread of #32 does not start (the amp CLI exits 1): the
// claim line, the lease, and status:in-progress are released, #32 is
// status:ready, the list counts no slot for it, and the Kanha is told why.
func TestFailedStartReleasesClaimAndTells(t *testing.T) {
	m := newMissionWorld(t)
	tell := m.ampMission(889, 32)
	if code, out, errOut := m.next(889); code != 0 || out != "start #32\n" {
		t.Fatalf("mission next: exit %d: %q%s", code, out, errOut)
	}
	exec := m.opts.DelegateExec
	m.opts.DelegateExec = func(ctx context.Context, dir string, env []string, stdin, name string, args ...string) (string, error) {
		out, err := exec(ctx, dir, env, stdin, name, args...)
		if name == "amp" && slices.ContainsFunc(args, func(a string) bool { return strings.HasPrefix(a, "aayu #32:") }) {
			return "", errors.New("amp: exit status 1: Error: no orb capacity")
		}
		return out, err
	}
	if got := m.claims(889); got[32] != 1 {
		t.Fatalf("the list did not claim #32: %v", got)
	}
	m.g.Reset()
	code, out, errOut := run(t, m.agent, m.opts, "build", "32", "--delegate", "amp", "--repo", "o/r", "--asked", "aayu mission next marked it start")
	if code != ExitExternal || !strings.Contains(errOut, "no orb capacity; #32 is status:ready again, and its claim and lease are released") {
		t.Fatalf("build 32: exit %d: %s%s", code, out, errOut)
	}
	if got := m.labels(32); !slices.Contains(got, "status:ready") || slices.Contains(got, "status:in-progress") || slices.Contains(got, LeaseLabel) {
		t.Fatalf("#32's labels after the failed start: %v", got)
	}
	if got := m.claims(889); got[32] != 0 {
		t.Fatalf("the claim of #32 is still live: %v", got)
	}
	if !slices.ContainsFunc(m.g.Writes(), func(r fake.Request) bool {
		b, _ := r.Body["body"].(string)
		return r.Method == "PATCH" && strings.Contains(b, "Lease released") && strings.Contains(b, "the start of its subthread failed")
	}) {
		t.Fatal("the lease of #32 was not released")
	}
	want := "start of #32 failed: build #32 --delegate amp: amp: amp: exit status 1: Error: no orb capacity"
	if got := m.callLog(); got[len(got)-1] != tell(want) {
		t.Fatalf("the last call is %q, want %q", got[len(got)-1], tell(want))
	}
	if got := m.kanhaEvents(32); !slices.Equal(got, []string{want + " (sent)"}) {
		t.Fatalf("delegate/32/state.json Kanha events: %q", got)
	}
	// The slot is free: the list marks #32 start again, and claims one slot for it.
	if code, out, errOut := m.next(889); code != 0 || out != "start #32\n" {
		t.Fatalf("mission next after the failed start: exit %d: %q%s", code, out, errOut)
	}
}

// Criterion 3: an ask from #33's subthread is answered: #33 keeps its slot
// (status:in-progress, its lease, and the list's count), and the Kanha is
// told that #33 resumes. #880's answer path is the caller; this drives it as
// that path will.
func TestAnsweredAskKeepsSlotAndTells(t *testing.T) {
	m := newMissionWorld(t)
	tell := m.ampMission(889, 33, 34)
	m.setPlan(889, func(p *MissionPlan) { p.Agents = 1 })
	wait34 := "waits #34: no free agent (1 of 1 busy after these starts)\n"
	if code, out, errOut := m.next(889); code != 0 || out != "start #33\n"+wait34 {
		t.Fatalf("mission next: exit %d: %q%s", code, out, errOut)
	}
	m.kanhaBuild(33, "amp")
	before := m.labels(33)
	calls := len(m.callLog())
	withHandler(t, "status", func(ctx context.Context, e *Env, _ Values) error { return e.answeredAsk(ctx, 33) })
	if code, out, errOut := run(t, m.agent, m.opts, "status", "--repo", "o/r"); code != 0 {
		t.Fatalf("the answer event: exit %d: %s%s", code, out, errOut)
	}
	want := "answered: #33 resumes"
	if got := m.callLog()[calls:]; !slices.Equal(got, []string{tell(want)}) {
		t.Fatalf("calls after the answer: %q", got)
	}
	if got := m.kanhaEvents(33); !slices.Equal(got, []string{want + " (sent)"}) {
		t.Fatalf("delegate/33/state.json Kanha events: %q", got)
	}
	if got := m.labels(33); !slices.Equal(got, before) || !slices.Contains(got, "status:in-progress") || !slices.Contains(got, LeaseLabel) {
		t.Fatalf("#33's labels changed: %v, were %v", got, before)
	}
	if got := m.claims(889); got[33] != 1 {
		t.Fatalf("the claim of #33 is not live: %v", got)
	}
	// #33 still holds the mission's one agent: #34 waits.
	if code, out, errOut := m.next(889); code != 0 || out != wait34 {
		t.Fatalf("mission next after the answer: exit %d: %q%s", code, out, errOut)
	}
}

// A Kanha whose send fails still gets the event in delegate/<n>/state.json,
// where its next wake reads it; a local Kanha gets it there only, at its
// next check (#1012).
func TestKanhaEventRecordedWhenNotSent(t *testing.T) {
	m := newMissionWorld(t)
	m.ampMission(889, 35)
	m.kanhaBuild(35, "amp")
	exec := m.opts.DelegateExec
	m.opts.DelegateExec = func(ctx context.Context, dir string, env []string, stdin, name string, args ...string) (string, error) {
		if name == "amp" && len(args) > 1 && args[1] == "continue" {
			return "", errors.New("amp: exit status 1: Error: thread not found")
		}
		return exec(ctx, dir, env, stdin, name, args...)
	}
	if code, _, errOut := run(t, m.agent, m.opts, "build", "35", "--stop", "spec unclear", "--repo", "o/r"); code != ExitBlocked || !slices.Contains(m.labels(35), "status:blocked") {
		t.Fatalf("build 35 --stop: exit %d: %s", code, errOut)
	}
	if got := m.kanhaEvents(35); len(got) != 1 || !strings.HasPrefix(got[0], "blocked #35: spec unclear: run aayu mission next 889 (not sent: ") {
		t.Fatalf("delegate/35/state.json Kanha events: %q", got)
	}
	// The local Codex Kanha of #890 takes the event at its next check (#1012): it is recorded, and nothing runs.
	m.featureWith(890, 36)
	m.start(890, "rahulnakmol", "codex")
	m.readyStory(36)
	m.kanhaBuild(36, "codex")
	calls := len(m.callLog())
	if code, _, errOut := run(t, m.agent, m.opts, "build", "36", "--stop", "spec unclear", "--repo", "o/r"); code != ExitBlocked || len(m.callLog()) != calls {
		t.Fatalf("build 36 --stop: exit %d, calls %q: %s", code, m.callLog()[calls:], errOut)
	}
	host, _ := os.Hostname()
	want := fmt.Sprintf("blocked #36: spec unclear: run aayu mission next 890 (recorded; the local Kanha on rahulnakmol@%s takes it at its next check)", host)
	if got := m.kanhaEvents(36); !slices.Equal(got, []string{want}) {
		t.Fatalf("delegate/36/state.json Kanha events: %q", got)
	}
}
