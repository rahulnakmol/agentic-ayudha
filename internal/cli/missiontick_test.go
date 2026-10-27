package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/delegate"
	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
	"github.com/rahulnakmol/agentic-ayudha/internal/inspect"
	"github.com/rahulnakmol/agentic-ayudha/internal/models"
	"github.com/rahulnakmol/agentic-ayudha/internal/ready"
)

// Story #1009: the hourly safety tick, on the missionWorld of Story #997. In
// that world #870 is the Initiative, so Feature #874 is the mission whose
// Kanha was told 20 minutes ago.

// tick runs `aayu mission tick` as the build-ready loop's agent, at a time.
func (m *missionWorld) tick(at time.Time) (int, TickReport, string) {
	m.t.Helper()
	opts := m.opts
	opts.Now = func() time.Time { return at }
	env := map[string]string{"AAYU_AGENT": "1"}
	for k, v := range m.agent {
		env[k] = v
	}
	code, out, errOut := run(m.t, env, opts, "mission", "tick", "--repo", "o/r", "--json")
	var r TickReport
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		m.t.Fatalf("mission tick: exit %d: %v: %s%s", code, err, out, errOut)
	}
	return code, r, errOut
}

// workLines is mission next's text output without its event lines (Story #1009): the list alone.
func workLines(out string) string {
	var b strings.Builder
	for _, l := range strings.SplitAfter(out, "\n") {
		if !strings.HasPrefix(l, "event #") {
			b.WriteString(l)
		}
	}
	return b.String()
}

// putState writes delegate/<n>/state.json.
func (m *missionWorld) putState(st delegate.State) {
	m.t.Helper()
	b, _ := json.Marshal(st)
	putLedger(m.t, m.bare, delegate.LedgerPath(st.Story), string(b))
}

// Criterion 2: the Kanha of #889 has a free agent and a runnable story and
// was told nothing since its start two hours ago; the Kanha of #874 has one
// too, and was told of a block 20 minutes ago. The tick tells only #889's,
// once, and starts and claims nothing itself. A tick five minutes later
// tells nobody; an hour on, both Kanhas still have work and are told again.
func TestTickWakesOnlyMissedKanha(t *testing.T) {
	m := newMissionWorld(t)
	m.featureWith(889, 20, 21)
	m.featureWith(874, 950, 951)
	m.start(889, "rahulnakmol", "amp")
	m.start(874, "rahulnakmol", "amp")
	for _, n := range []int{20, 950} {
		m.status(n, "ready", "")
		m.readyAt(n, time.Now().Add(-time.Hour))
	}
	m.status(21, "in-progress", "")
	m.status(951, "blocked", "")
	at := time.Now().Add(2 * time.Hour).UTC().Truncate(time.Second)
	told := at.Add(-20 * time.Minute).Format(time.RFC3339)
	m.putState(delegate.State{Story: 951, Feature: 874, Receiver: "amp", Mode: "cloud", Status: dlgWorking,
		Events: []delegate.Event{{At: told, Status: kanhaEvent, Detail: "blocked #951: spec unclear: run aayu mission next 874 (sent)"}}})
	before := len(m.callLog())

	code, r, errOut := m.tick(at)
	wake := "tick: #20 can start, and no event reached the Kanha since the last tick: run aayu mission next 889"
	calls := m.callLog()[before:]
	if code != 0 || !slices.Equal(calls, []string{"amp threads continue " + kanhaAmpThread + " -ox " + wake}) {
		t.Fatalf("tick: exit %d, receiver calls %q: %s", code, calls, errOut)
	}
	if len(r.Woken) != 1 || r.Woken[0].Feature != 889 || r.Woken[0].Status != wakeSent ||
		!slices.Contains(r.Held, TickHeld{Feature: 874, Why: "its Kanha was told at " + told + ", since the last tick"}) {
		t.Fatalf("woken %+v, held %+v", r.Woken, r.Held)
	}
	p889, _ := m.plan(889)
	if want := []MissionWake{{At: at.Format(time.RFC3339), Event: wake, Status: wakeSent}}; !slices.Equal(p889.Wakes, want) {
		t.Fatalf("plan 889 wakes %+v, want %+v", p889.Wakes, want)
	}
	if p874, _ := m.plan(874); len(p874.Wakes) != 0 {
		t.Fatalf("#874's Kanha was woken: %+v", p874.Wakes)
	}
	// The tick only told the Kanha: #20 is still Ready, unclaimed, and has no thread.
	if labels := m.g.Issues[20].Labels; !slices.ContainsFunc(labels, func(l fake.Label) bool { return l.Name == "status:ready" }) || len(m.claims(889)) != 0 {
		t.Fatalf("the tick started #20: labels %v, claims %v", labels, m.claims(889))
	}
	if exec.Command("git", "-C", m.bare, "show", "ledger:"+delegate.LedgerPath(20)).Run() == nil {
		t.Fatal("the tick handed #20 off")
	}

	// Five minutes on, the wake it recorded counts as a tell: no Kanha is told twice in one tick.
	before = len(m.callLog())
	if code, r, errOut := m.tick(at.Add(5 * time.Minute)); code != 0 || len(r.Woken) != 0 || len(m.callLog()) != before ||
		!slices.Contains(r.Held, TickHeld{Feature: 889, Why: "its Kanha was told at " + at.Format(time.RFC3339) + ", since the last tick"}) {
		t.Fatalf("second tick: exit %d, woken %+v, held %+v: %s", code, r.Woken, r.Held, errOut)
	}
	// An hour on, neither Kanha acted, so both are told: the safety net holds each hour.
	if code, r, errOut := m.tick(at.Add(time.Hour)); code != 0 || len(r.Woken) != 2 || r.Woken[0].Feature != 874 || r.Woken[1].Feature != 889 {
		t.Fatalf("an hour on: exit %d, woken %+v, held %+v: %s", code, r.Woken, r.Held, errOut)
	}
	// The plan keeps only the newest keptWakes wakes, so it does not grow each hour.
	m.setPlan(889, func(p *MissionPlan) {
		for i := range 30 {
			p.Wakes = append(p.Wakes, MissionWake{At: at.Add(time.Duration(i-40) * time.Hour).Format(time.RFC3339), Event: "old", Status: wakeSent})
		}
	})
	if code, r, errOut := m.tick(at.Add(3 * time.Hour)); code != 0 || !slices.ContainsFunc(r.Woken, func(k TickWake) bool { return k.Feature == 889 }) {
		t.Fatalf("a long history: exit %d, woken %+v: %s", code, r.Woken, errOut)
	}
	if p889, _ := m.plan(889); len(p889.Wakes) != keptWakes || p889.Wakes[keptWakes-1].At != at.Add(3*time.Hour).Format(time.RFC3339) {
		t.Fatalf("plan 889 keeps %d wakes, newest %+v", len(p889.Wakes), p889.Wakes[len(p889.Wakes)-1])
	}
}

// Criterion 3: the Kanha of #889 wakes 3 hours after a story closed. Its
// `aayu mission next` reads every event since events_cursor in
// missions/889/plan.json, the oldest 3 hours back, misses none, and moves the
// cursor to the newest only once the list and its claims are written: a wake
// that fails before then leaves the cursor, so the next one reads them again.
func TestWakeReadsFromEventsCursor(t *testing.T) {
	m := newMissionWorld(t)
	m.featureWith(889, 20, 21, 22)
	m.start(889, "rahulnakmol", "amp")
	plan, _ := m.plan(889)
	started, _ := time.Parse(time.RFC3339, plan.StartedAt)
	m.status(22, "ready", "")
	closed, failed, toldAt := started.Add(10*time.Minute), started.Add(time.Hour), started.Add(2*time.Hour)
	m.edit(func() { m.g.Issues[20].State, m.g.Issues[20].ClosedAt = "closed", closed.Format(time.RFC3339) })
	m.putState(delegate.State{Story: 21, Feature: 889, Receiver: "amp", Mode: "cloud", Status: dlgFailed, Events: []delegate.Event{
		{At: started.Add(-time.Minute).Format(time.RFC3339), Status: dlgHandedOff, Detail: "brief sent"}, // before the start: not this mission's
		{At: failed.Format(time.RFC3339), Status: dlgFailed, Detail: "hand-off failed: amp: exit status 1"},
		{At: toldAt.Format(time.RFC3339), Status: kanhaEvent, Detail: "start of #21 failed: amp: exit status 1 (sent)"},
	}})
	wake := closed.Add(3 * time.Hour)
	next := func(at time.Time) (int, WorkList, string) {
		t.Helper()
		opts := m.opts
		opts.Now = func() time.Time { return at }
		code, out, errOut := run(t, m.agent, opts, "mission", "next", "889", "--repo", "o/r", "--json")
		var l WorkList
		_ = json.Unmarshal([]byte(out), &l)
		return code, l, errOut
	}
	// The wake fails before it has acted (#22's Ready time cannot be read): no claim, and the cursor stays.
	m.edit(func() { m.g.Fail, m.g.FailAfter = "GET /repos/o/r/issues/22/comments", 0 })
	if code, _, errOut := next(wake); code == 0 || len(m.claims(889)) != 0 {
		t.Fatalf("a failed wake: exit %d, claims %v: %s", code, m.claims(889), errOut)
	} else if p, _ := m.plan(889); p.EventsCursor != "" {
		t.Fatalf("a failed wake moved the cursor to %s", p.EventsCursor)
	}
	code, l, errOut := next(wake)
	want := []MissionEvent{
		{At: closed.UTC().Format(time.RFC3339), Story: 20, Event: "closed"},
		{At: failed.UTC().Format(time.RFC3339), Story: 21, Event: "failed: hand-off failed: amp: exit status 1"},
		{At: toldAt.UTC().Format(time.RFC3339), Story: 21, Event: "kanha: start of #21 failed: amp: exit status 1 (sent)"},
	}
	if code != 0 || !slices.Equal(l.Events, want) || !slices.Contains(l.Items, WorkListItem{Key: "#22", Action: "start"}) {
		t.Fatalf("the late wake: exit %d, events %+v, items %+v: %s", code, l.Events, l.Items, errOut)
	}
	if p, _ := m.plan(889); p.EventsCursor != want[2].At || m.claims(889)[22] != 1 {
		t.Fatalf("after the wake acted: cursor %q, claims %v", p.EventsCursor, m.claims(889))
	}
	// The next wake reads only what came after the cursor.
	if code, l, errOut := next(wake.Add(time.Minute)); code != 0 || len(l.Events) != 0 {
		t.Fatalf("a wake with no new event: exit %d, events %+v: %s", code, l.Events, errOut)
	}
	later := wake.Add(2 * time.Minute).UTC().Format(time.RFC3339)
	st := m.state(21)
	st.Events = append(st.Events, delegate.Event{At: later, Status: kanhaEvent, Detail: "answered: #21 resumes (sent)"})
	m.putState(st)
	if code, l, errOut := next(wake.Add(3 * time.Minute)); code != 0 ||
		!slices.Equal(l.Events, []MissionEvent{{At: later, Story: 21, Event: "kanha: answered: #21 resumes (sent)"}}) {
		t.Fatalf("a wake after one more event: exit %d, events %+v: %s", code, l.Events, errOut)
	}
	if p, _ := m.plan(889); p.EventsCursor != later {
		t.Fatalf("cursor %q, want %s", p.EventsCursor, later)
	}
	// A new Approve to build of the running mission keeps its cursor.
	if code, out, errOut := m.approve(889, "--milestones", "MS1=#20,#21 MS2=#22"); code != 0 {
		t.Fatalf("approve 889 again: exit %d: %s%s", code, out, errOut)
	}
	if p, _ := m.plan(889); p.Approval != 2 || p.EventsCursor != later {
		t.Fatalf("after approval 2: approval %d, cursor %q", p.Approval, p.EventsCursor)
	}
}

// Criterion 4: story #2's PR (a Codex cloud receiver; the delegate fixture's
// story stands in for #51) gets a new inspect verdict on its head. Each tick
// runs `aayu build 2 --delegate codex` once: it finds the PR, sends the
// findings of a fix verdict once per head, and at merge-ready lists the PR for
// `aayu ship`. The tick itself never hands off, ships, or merges.
func TestTickSyncsCloudPRVerdict(t *testing.T) {
	g, env, opts, dir, bare := delegateFixture(t)
	env["AAYU_DELEGATE__CODEX_ENV"] = "env_123"
	opts.LookPath = func(name string) (string, error) {
		if name == "codex" {
			return "/bin/codex", nil
		}
		return "", os.ErrNotExist
	}
	var submits []string // each `codex cloud exec`: a hand-off or a follow-up, with its branch and prompt
	opts.DelegateExec = func(_ context.Context, _ string, _ []string, _, _ string, args ...string) (string, error) {
		b, _ := os.ReadFile(filepath.Join("..", "delegate", "testdata", "codex", map[string]string{"cloud exec": "exec.txt", "cloud status": "status-running.txt"}[strings.Join(args[:2], " ")]))
		if args[1] == "exec" {
			submits = append(submits, strings.Join(args[5:], " "))
		}
		return string(b), nil
	}
	if code, res, errOut := delegateRun(t, env, opts, "--delegate", "codex"); code != 0 || res.Delegation.Mode != "cloud" || len(submits) != 1 {
		t.Fatalf("hand-off: exit %d %+v: %s", code, res, errOut)
	}
	agent := map[string]string{"AAYU_AGENT": "1"}
	for k, v := range env {
		agent[k] = v
	}
	tick := func() TickReport {
		t.Helper()
		code, out, errOut := run(t, agent, opts, "mission", "tick", "--repo", "o/r", "--json")
		var r TickReport
		if err := json.Unmarshal([]byte(out), &r); code != 0 || err != nil || len(r.Synced) != 1 || r.Synced[0].Command != "aayu build 2 --delegate codex" || r.Synced[0].Error != "" {
			t.Fatalf("tick: exit %d %v %+v: %s%s", code, err, r.Synced, out, errOut)
		}
		return r
	}
	head := strings.Repeat("c", 40)
	pr := g.AddPR("feat: show empty list", "Refs #2\n\nCodex's change.", "founder", "codex/show-empty-list", head, "dev")
	g.Mu.Lock()
	pr.Diff = defectDiff
	g.Mu.Unlock()
	if r := tick(); r.Synced[0].PR != pr.Number || r.Synced[0].Status != dlgPROpen || len(r.Ship) != 0 || len(submits) != 1 {
		t.Fatalf("tick before inspect: %+v, ship %+v, %d submits", r.Synced, r.Ship, len(submits))
	}
	// The merge gate's inspect asks for a fix on the head: the tick sends its findings once, on the PR's branch.
	writeInspectPack(t, dir, pr.Number, head, nil)
	model := &models.Fake{Replies: []models.Response{inspectReply("fix", "P1")}}
	if code, ires, errOut := inspectRun(t, env, opts, model, pr.Number); code != ExitBlocked || len(ires.Findings) != 1 {
		t.Fatalf("inspect: exit %d %+v: %s", code, ires, errOut)
	} else if r := tick(); r.Synced[0].Status != dlgFix || len(submits) != 2 || !strings.HasPrefix(submits[1], "codex/show-empty-list ") ||
		!strings.Contains(submits[1], ires.Findings[0].ID) || len(r.Ship) != 0 {
		t.Fatalf("tick after the fix verdict: %+v, submits %q", r.Synced, submits)
	}
	if r := tick(); !strings.Contains(r.Synced[0].Next, "were sent to codex") || len(submits) != 2 {
		t.Fatalf("a second tick on the same head: %+v, %d submits", r.Synced, len(submits))
	}
	// Codex pushes a fix and inspect says merge: the tick lists the PR for ship and sends nothing.
	head2 := strings.Repeat("d", 40)
	g.Mu.Lock()
	g.Pulls[pr.Number].Head.SHA = head2
	pr.Diff = cleanDiff
	g.Mu.Unlock()
	writeInspectPack(t, dir, pr.Number, head2, nil)
	model.Replies = []models.Response{inspectReply("merge", "")}
	if code, ires, errOut := inspectRun(t, env, opts, model, pr.Number); ires.Disposition != inspect.Merge {
		t.Fatalf("inspect at the fix: exit %d %+v: %s", code, ires, errOut)
	}
	r := tick()
	if r.Synced[0].Status != dlgReady || len(submits) != 2 || len(r.Ship) != 1 || r.Ship[0].PR != pr.Number || r.Ship[0].Story != 2 {
		t.Fatalf("tick at merge-ready: %+v, ship %+v, %d submits", r.Synced, r.Ship, len(submits))
	}
	if code, out, errOut := run(t, agent, opts, "mission", "tick", "--repo", "o/r"); code != 0 ||
		!strings.Contains(out, "synced #2 (codex): aayu build 2 --delegate codex: merge-ready; ") || !strings.Contains(out, fmt.Sprintf("ship PR #%d (#2): run aayu ship %d\n", pr.Number, pr.Number)) {
		t.Fatalf("tick in text: exit %d:\n%s%s", code, out, errOut)
	}
	if g.Pulls[pr.Number].Merged {
		t.Fatal("the tick merged the PR")
	}
	var st delegate.State
	if err := json.Unmarshal([]byte(git(t, bare, "show", "ledger:"+delegate.LedgerPath(2))), &st); err != nil || st.Status != dlgReady || len(st.FollowUps) != 1 || st.Handoffs > 1 {
		t.Fatalf("delegate/2/state.json: %v %+v", err, st)
	}
	// #2 was handed to Codex before its Feature's mission started; the mission then started on Amp. A sync
	// would switch #2 to the mission's tool and hand it off afresh: the tick refuses it and starts nothing.
	feat := g.AddIssue("Feature: lists", "", "type:feature")
	g.Mu.Lock()
	feat.Subs, g.Issues[2].Parent = append(feat.Subs, 2), feat.Number
	g.Mu.Unlock()
	plan, _ := json.Marshal(MissionPlan{Schema: MissionPlanSchema, Feature: feat.Number, StartedBy: "founder", StartedAt: "2026-10-26T00:00:00Z", Tool: "amp"})
	putLedger(t, bare, MissionPlanPath(feat.Number), string(plan))
	var ampCalls int
	agent["AMP_API_KEY"] = "test-key"
	if err := os.WriteFile(filepath.Join(dir, ".aayudha", "env-verified.toml"), []byte("[cloud.amp]\nfingerprint = \"x\"\nmethod = \"cloud\"\nat = \"2026-10-26T00:00:00Z\"\nproject = \"o/r\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	opts.LookPath = func(name string) (string, error) { return "/bin/" + name, nil }
	exec := opts.DelegateExec
	opts.DelegateExec = func(ctx context.Context, dir string, env []string, in, name string, args ...string) (string, error) {
		if name == "amp" {
			ampCalls++
			return "https://ampcode.com/threads/T-01a17000-0000-7000-8000-000000000002\n", nil
		}
		return exec(ctx, dir, env, in, name, args...)
	}
	approveFor(t, g, dir, feat.Number)
	code, out, errOut := run(t, agent, opts, "mission", "tick", "--repo", "o/r", "--json")
	var rs TickReport
	_ = json.Unmarshal([]byte(out), &rs)
	var after delegate.State
	_ = json.Unmarshal([]byte(git(t, bare, "show", "ledger:"+delegate.LedgerPath(2))), &after)
	if code != 0 || len(rs.Synced) != 1 || !strings.Contains(rs.Synced[0].Error, "a sync never hands a story off") || after.Receiver != "codex" || after.Handoffs != st.Handoffs || ampCalls != 0 {
		t.Fatalf("a mission started on another tool: exit %d, synced %+v, #2 now on %s with %d hand-offs, %d amp calls: %s", code, rs.Synced, after.Receiver, after.Handoffs, ampCalls, errOut)
	}
}

// CodeRabbit on #1103: the tick syncs no delegation of a paused or ended
// mission, nor of a story a person stopped alone; the others of a running
// mission are synced.
func TestTickSkipsStoppedMissionDelegations(t *testing.T) {
	m := newMissionWorld(t)
	m.featureWith(889, 20, 21)
	m.start(889, "rahulnakmol", "amp")
	for _, n := range []int{20, 21} {
		m.putState(delegate.State{Story: n, Feature: 889, Receiver: "cursor", Mode: "cloud", Status: dlgPROpen, PR: 500 + n, Task: cursorStoryAgent + "/run-01"})
	}
	synced := func() []int {
		t.Helper()
		_, r, _ := m.tick(time.Now().Add(2 * time.Hour))
		var ns []int
		for _, s := range r.Synced {
			ns = append(ns, s.Story)
		}
		return ns
	}
	if got := synced(); !slices.Equal(got, []int{20, 21}) {
		t.Fatalf("running mission: synced %v, want #20 and #21", got)
	}
	// #21's delegation fails after the tick read it: the sync hands nothing off, so the tick starts no story.
	open21 := delegate.State{Story: 21, Feature: 889, Receiver: "cursor", Mode: "cloud", Status: dlgPROpen, PR: 521, Task: cursorStoryAgent + "/run-01"}
	failed := open21
	failed.Status = dlgFailed
	once := false
	m.edit(func() {
		m.g.Hook = func(method, path string) {
			if !once && path == "/repos/o/r/issues/889" {
				once = true
				m.putState(failed)
			}
		}
	})
	_, r, _ := m.tick(time.Now().Add(2 * time.Hour))
	m.edit(func() { m.g.Hook = nil })
	var st21 delegate.State
	_ = json.Unmarshal([]byte(git(t, m.bare, "show", "ledger:"+delegate.LedgerPath(21))), &st21)
	i := slices.IndexFunc(r.Synced, func(s TickSync) bool { return s.Story == 21 })
	if !once || i < 0 || !strings.Contains(r.Synced[i].Error, "nothing to sync") || !reflect.DeepEqual(st21, failed) {
		t.Fatalf("a delegation failed during the tick: synced %+v, #21 now %+v", r.Synced, st21)
	}
	m.putState(open21)
	m.setPlan(889, func(p *MissionPlan) {
		p.Stopped = []MissionStopped{{Number: 20, Reason: "wrong approach", By: "rahulnakmol"}}
	})
	if got := synced(); !slices.Equal(got, []int{21}) {
		t.Fatalf("#20 stopped: synced %v, want only #21", got)
	}
	// A stop of #21 written while #20 syncs holds the sync of #21 (Bug #1121).
	m.setPlan(889, func(p *MissionPlan) { p.Stopped = nil })
	_, raw := m.plan(889)
	once = false
	m.edit(func() {
		m.g.Hook = func(method, path string) {
			if !once && path == "/repos/o/r/issues/20/parent" {
				once = true
				var p MissionPlan
				_ = json.Unmarshal([]byte(raw), &p)
				p.Stopped = []MissionStopped{{Number: 21, Reason: "wrong approach", By: "rahulnakmol"}}
				b, _ := json.Marshal(p)
				putLedger(t, m.bare, MissionPlanPath(889), string(b))
			}
		}
	})
	got := synced()
	m.edit(func() { m.g.Hook = nil })
	if !once || !slices.Equal(got, []int{20}) {
		t.Fatalf("#21 stopped during the sync of #20: synced %v (hook ran: %v), want only #20", got, once)
	}
	m.setPlan(889, func(p *MissionPlan) { p.Pause = &MissionPause{Reason: "a person stopped it"} })
	// #20 was handed off before its mission started, so its state names no Feature: the pause holds it too.
	m.putState(delegate.State{Story: 20, Receiver: "cursor", Mode: "cloud", Status: dlgPROpen, PR: 520, Task: cursorStoryAgent + "/run-01"})
	if _, r, _ := m.tick(time.Now().Add(2 * time.Hour)); len(r.Synced) != 0 || !slices.Contains(r.Held, TickHeld{Feature: 889, Why: "paused: a person stopped it"}) {
		t.Fatalf("paused mission: synced %+v, held %+v", r.Synced, r.Held)
	}
	m.putState(delegate.State{Story: 20, Feature: 889, Receiver: "cursor", Mode: "cloud", Status: dlgPROpen, PR: 520, Task: cursorStoryAgent + "/run-01"})
	m.setPlan(889, func(p *MissionPlan) { p.Pause, p.State = nil, "ended (done early)" })
	if _, r, _ := m.tick(time.Now().Add(2 * time.Hour)); len(r.Synced) != 0 || !slices.Contains(r.Held, TickHeld{Feature: 889, Why: "ended (done early)"}) {
		t.Fatalf("ended mission: synced %+v, held %+v", r.Synced, r.Held)
	}
	putLedger(t, m.bare, MissionPlanPath(889), "not json") // its pause or end cannot be read
	if _, r, _ := m.tick(time.Now().Add(2 * time.Hour)); len(r.Synced) != 0 || len(r.Held) != 1 || !strings.HasPrefix(r.Held[0].Why, "unreadable: ") {
		t.Fatalf("unreadable plan: synced %+v, held %+v", r.Synced, r.Held)
	}
	// The plan is gone: its pause or end is unknown, so nothing of #889 syncs.
	wt := filepath.Join(t.TempDir(), "ledger")
	git(t, filepath.Dir(wt), "clone", "--quiet", "--branch", "ledger", m.bare, wt)
	git(t, wt, "rm", "--quiet", MissionPlanPath(889))
	git(t, wt, "commit", "--quiet", "-m", "the plan of #889 is removed")
	git(t, wt, "push", "--quiet", "origin", "ledger")
	if _, r, _ := m.tick(time.Now().Add(2 * time.Hour)); len(r.Synced) != 0 {
		t.Fatalf("missing plan: synced %+v", r.Synced)
	}
}

// What the tick tells a person in text, and each mission it holds with why:
// a mission not started (not listed), one with no Kanha recorded, a local
// Kanha, one with nothing to start, and one whose plan is unreadable. A dry
// run sends nothing; a send that fails is recorded as not sent, so the next
// tick tries again. It takes no Feature, and the [budget] breaker stops it.
func TestTickHoldsAndReports(t *testing.T) {
	m := newMissionWorld(t)
	text := func(at time.Time, args ...string) (int, string, string) {
		t.Helper()
		opts := m.opts
		opts.Now = func() time.Time { return at }
		env := map[string]string{"AAYU_AGENT": "1"}
		for k, v := range m.agent {
			env[k] = v
		}
		return run(t, env, opts, append([]string{"mission", "tick", "--repo", "o/r"}, args...)...)
	}
	at := time.Now().Add(2 * time.Hour)
	if code, out, errOut := text(at); code != 0 || out != "nothing to wake or sync: no mission runs and no cloud pull request is open\n" {
		t.Fatalf("no mission: exit %d %q: %s", code, out, errOut)
	}
	if code, _, errOut := run(t, m.agent, m.opts, "mission", "tick", "889", "--repo", "o/r"); code != ExitUsage || !strings.Contains(errOut, "give no Feature") {
		t.Fatalf("tick 889: exit %d: %s", code, errOut)
	}
	if code, _, errOut := run(t, m.agent, m.opts, "mission", "next", "--repo", "o/r"); code != ExitUsage || !strings.Contains(errOut, "mission next: name the Feature") {
		t.Fatalf("next with no Feature: exit %d: %s", code, errOut)
	}

	m.featureWith(889, 20)
	m.start(889, "rahulnakmol", "amp")
	m.status(20, "ready", "")
	m.readyAt(20, time.Now().Add(-time.Hour))
	m.featureWith(874, 950) // started on a local Codex Kanha
	m.start(874, "rahulnakmol", "codex")
	m.featureWith(901, 1001) // started, and its only story is not Ready
	m.start(901, "rahulnakmol", "amp")
	m.featureWith(902, 1002) // approved, not started: not listed
	if code, out, errOut := m.approve(902); code != 0 {
		t.Fatalf("approve 902: exit %d: %s%s", code, out, errOut)
	}
	m.featureWith(903, 1003) // started, and its Kanha's record is lost
	m.start(903, "rahulnakmol", "amp")
	var a ready.Approval
	_ = json.Unmarshal([]byte(git(t, m.bare, "show", "ledger:"+ready.ApprovalPath(903))), &a)
	a.Parent = nil
	b, _ := json.Marshal(a)
	putLedger(t, m.bare, ready.ApprovalPath(903), string(b))
	putLedger(t, m.bare, MissionPlanPath(904), "not json")
	putLedger(t, m.bare, "missions/notes/plan.json", "{}") // not a Feature's: skipped

	// A dry run says what it would send and sends nothing.
	before := len(m.callLog())
	if code, out, errOut := text(at, "--dry-run"); code != 0 || len(m.callLog()) != before ||
		!strings.Contains(out, "woke the Kanha of #889 (dry run: not sent): tick: #20 can start") {
		t.Fatalf("dry run: exit %d: %s%s", code, out, errOut)
	}
	if p, _ := m.plan(889); len(p.Wakes) != 0 {
		t.Fatalf("a dry run recorded a wake: %+v", p.Wakes)
	}
	// The Kanha's thread cannot be reached: the wake is recorded as not sent, and does not count as a tell.
	opts := m.opts
	m.opts.DelegateExec = func(ctx context.Context, dir string, env []string, in, name string, args ...string) (string, error) {
		if name == "amp" && slices.Contains(args, "continue") {
			return "", errors.New("amp: exit status 1: Error: no orb capacity")
		}
		return m.exec(ctx, dir, env, in, name, args...)
	}
	code, out, errOut := text(at)
	m.opts = opts
	for _, want := range []string{
		"woke the Kanha of #889 (not sent: amp: amp: exit status 1: Error: no orb capacity): tick: #20 can start",
		"held #874: its Kanha is local, on rahulnakmol@",
		"held #901: nothing can start: not Ready",
		"held #903: no Kanha is recorded: a person runs aayu mission start 903",
		"held #904: unreadable: ",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("tick output does not say %q:\n%s%s", want, out, errOut)
		}
	}
	if code != 0 || strings.Contains(out, "#902") || strings.Contains(out, "notes") {
		t.Fatalf("tick: exit %d:\n%s%s", code, out, errOut)
	}
	if p, _ := m.plan(889); len(p.Wakes) != 1 || !strings.HasPrefix(p.Wakes[0].Status, wakeNotSent) {
		t.Fatalf("plan 889 wakes: %+v", p.Wakes)
	}
	if code, r, errOut := m.tick(at.Add(time.Minute)); code != 0 || len(r.Woken) != 1 || r.Woken[0].Status != wakeSent {
		t.Fatalf("the next tick after a failed send: exit %d %+v: %s", code, r.Woken, errOut)
	}
	// The [budget] breaker stops the tick before it reads anything.
	_ = os.MkdirAll(filepath.Join(m.dir, ".aayudha", "state"), 0o755)
	_ = os.WriteFile(filepath.Join(m.dir, ".aayudha", "state", "paused"), []byte("per_day_usd"), 0o644)
	if code, _, errOut := text(at); code != ExitBudget {
		t.Fatalf("breaker, the loop's agent: exit %d: %s", code, errOut)
	}
	if code, _, errOut := run(t, m.person, m.opts, "mission", "tick", "--repo", "o/r"); code != ExitBudget || !strings.Contains(errOut, "nothing is woken or synced") {
		t.Fatalf("breaker, a person's run: exit %d: %s", code, errOut)
	}
}

// The tick's failure paths, each made to happen mid-run: another tick wakes
// the Kanha first (it is told at most once), the Feature drifted since
// its approval (held), the plan
// cannot be written after the send (the wake stays "sending", so it counts),
// and mission next cannot move the cursor (it stays, and says so).
func TestTickRacesAndFailures(t *testing.T) {
	m := newMissionWorld(t)
	m.featureWith(889, 20)
	m.start(889, "rahulnakmol", "amp")
	m.status(20, "ready", "")
	m.readyAt(20, time.Now().Add(-time.Hour))
	at := time.Now().Add(2 * time.Hour).UTC().Truncate(time.Second)
	held := func(at time.Time) string {
		t.Helper()
		code, r, errOut := m.tick(at)
		if code != 0 || len(r.Held) != 1 {
			t.Fatalf("tick at %s: exit %d, woken %+v, held %+v: %s", at, code, r.Woken, r.Held, errOut)
		}
		return r.Held[0].Why
	}
	// During this tick's read of GitHub, the plan changes: another tick records
	// its wake, or a person pauses the mission. This tick sends nothing.
	race := func(at time.Time, change func(*MissionPlan)) string {
		t.Helper()
		_, raw := m.plan(889)
		once := false
		m.edit(func() {
			m.g.Hook = func(method, path string) {
				if !once && path == "/repos/o/r/issues/20/comments" {
					once = true
					var p MissionPlan
					_ = json.Unmarshal([]byte(raw), &p)
					change(&p)
					b, _ := json.Marshal(p)
					putLedger(t, m.bare, MissionPlanPath(889), string(b))
				}
			}
		})
		defer m.edit(func() { m.g.Hook = nil })
		before := len(m.callLog())
		why := held(at)
		if n := len(m.callLog()) - before; n != 0 {
			t.Fatalf("a racing change: %q, and %d calls", why, n)
		}
		return why
	}
	if why := race(at, func(p *MissionPlan) {
		p.Wakes = []MissionWake{{At: at.Format(time.RFC3339), Event: "tick: #20 can start", Status: wakeSent}}
	}); why != "another tick woke its Kanha" {
		t.Fatalf("a racing tick: %q", why)
	}
	if why := race(at.Add(2*time.Hour), func(p *MissionPlan) { p.Pause = &MissionPause{Reason: "a person stopped it"} }); why != "paused: a person stopped it" {
		t.Fatalf("a pause during the tick: %q", why)
	}
	m.setPlan(889, func(p *MissionPlan) { p.Pause = nil })
	later := at.Add(2 * time.Hour)

	// The Feature's intent drifted since Approve to build: the Kanha is not woken to start anything.
	body := m.g.Issues[889].Body
	m.edit(func() { m.g.Issues[889].Body += "\n\n### Scope\n\nA new scope nobody approved.\n" })
	if why := held(later); why != "not approved to build: intent v1 drifted (#889 body changed)" {
		t.Fatalf("a drifted Feature: %q", why)
	}
	m.edit(func() { m.g.Issues[889].Body = body })
	// The wake is sent, and the plan cannot be written after it: it stays "sending", which counts as told.
	var raw string
	opts := m.opts
	m.opts.DelegateExec = func(ctx context.Context, dir string, env []string, in, name string, args ...string) (string, error) {
		if name == "amp" && slices.Contains(args, "continue") {
			_, raw = m.plan(889)
			putLedger(t, m.bare, MissionPlanPath(889), "not json")
		}
		return m.exec(ctx, dir, env, in, name, args...)
	}
	code, r, errOut := m.tick(later)
	m.opts = opts
	if code != 0 || len(r.Woken) != 1 || !strings.HasPrefix(r.Woken[0].Status, "sent; left sending in missions/889/plan.json: ") {
		t.Fatalf("the plan is lost after the send: exit %d, woken %+v: %s", code, r.Woken, errOut)
	}
	putLedger(t, m.bare, MissionPlanPath(889), raw)
	if why := held(later.Add(5 * time.Minute)); !strings.HasPrefix(why, "its Kanha was told at "+later.Format(time.RFC3339)) {
		t.Fatalf("after a wake left sending: %q", why)
	}
	// mission next lists an event, and cannot move the cursor: it says so, and the next wake reads it again.
	m.putState(delegate.State{Story: 20, Feature: 889, Receiver: "amp", Mode: "cloud", Status: dlgFailed,
		Events: []delegate.Event{{At: later.Format(time.RFC3339), Status: dlgFailed, Detail: "hand-off failed"}}})
	m.edit(func() {
		m.g.Hook = func(method, path string) {
			if path == "/repos/o/r/issues/20/comments" {
				putLedger(t, m.bare, MissionPlanPath(889), "not json")
			}
		}
	})
	code, out, errOut := m.next(889)
	m.edit(func() { m.g.Hook = nil })
	if code != 0 || !strings.Contains(out, "event #20 at "+later.Format(time.RFC3339)+": failed: hand-off failed") ||
		!strings.Contains(errOut, "the events cursor of #889 stays at") {
		t.Fatalf("a cursor that cannot move: exit %d: %s%s", code, out, errOut)
	}
	// With no GitHub token the tick reads nothing.
	if code, _, errOut := run(t, map[string]string{"XDG_STATE_HOME": m.person["XDG_STATE_HOME"]}, m.opts, "mission", "tick", "--repo", "o/r"); code == 0 {
		t.Fatalf("no token: exit 0: %s", errOut)
	}
}
