package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"maps"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/delegate"
	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
)

// Story #1002: `aayu mission stop` and `aayu mission resume` on the
// missionWorld of Stories #997 and #998, with the receivers' CLIs and
// Cursor's API stubbed, and `aayu ship` on the thin fixture.

// execLog records the follow-ups a stop or resume sends through the
// receivers' CLIs, and answers the Claude Code CLI's follow-up as queued;
// every other call goes to the mission world's stub (Kanha starts).
type execLog struct {
	mu    sync.Mutex
	calls []string
	on    func(name string) // runs while the CLI runs, as a sync that writes meanwhile
	next  func(ctx context.Context, dir string, env []string, stdin, name string, args ...string) (string, error)
}

func (x *execLog) exec(ctx context.Context, dir string, env []string, stdin, name string, args ...string) (string, error) {
	followUp := (name == "amp" && len(args) > 1 && args[0] == "threads" && args[1] == "continue") || (name == "claude" && slices.Contains(args, "-p"))
	if !followUp {
		return x.next(ctx, dir, env, stdin, name, args...)
	}
	if x.on != nil {
		x.on(name)
	}
	x.mu.Lock()
	defer x.mu.Unlock()
	x.calls = append(x.calls, name+" "+strings.Join(args, " "))
	if name == "claude" {
		return `{"ok": true, "session_id": "session_21"}`, nil
	}
	if slices.Contains(args, "T-01a00000-0000-0000-0000-0000000000ff") { // a thread the CLI cannot reach
		return "", fmt.Errorf("thread not found")
	}
	return "", nil
}

func (x *execLog) all() []string { x.mu.Lock(); defer x.mu.Unlock(); return slices.Clone(x.calls) }

// threads is a started mission whose running stories each have a thread:
// the stubbed CLIs and a fake Cursor API reach them.
type threads struct {
	*missionWorld
	x      *execLog
	cursor *fakeCursor
	env    map[string]string // the person's, with the Cursor key
	set    []string
}

func newThreads(t *testing.T) *threads {
	m := newMissionWorld(t)
	w := &threads{missionWorld: m, x: &execLog{next: m.exec}, cursor: &fakeCursor{}}
	srv := httptest.NewServer(w.cursor)
	t.Cleanup(srv.Close)
	m.opts.DelegateExec = w.x.exec
	w.env = maps.Clone(m.person)
	w.env["AAYU_SECRET_CURSOR_API_KEY"] = "cursor-key"
	w.set = []string{"--set", "delegate.cursor_api=" + srv.URL}
	return w
}

// run is story n in progress, built by a thread of receiver (cloud) with task and url.
func (m *missionWorld) run(n int, receiver, task, url string) {
	m.t.Helper()
	m.status(n, "in-progress", "")
	st := delegate.State{Story: n, Receiver: receiver, Mode: "cloud", Task: task, TaskURL: url, Status: dlgWorking, StartedAt: "2026-10-26T09:00:00Z"}
	b, _ := json.Marshal(st)
	putLedger(m.t, m.bare, delegate.LedgerPath(n), string(b))
}

// control runs aayu mission <action> f as the person in w.env, the device code approved by approver.
func (w *threads) control(approver, action string, f int, args ...string) (int, string, string) {
	w.t.Helper()
	return w.mission(w.env, approver, action, f, append(slices.Clone(w.set), args...)...)
}

// repoWrites are the requests that changed the repository on GitHub.
func (m *missionWorld) repoWrites() []string {
	var out []string
	for _, r := range m.g.Writes() {
		if strings.HasPrefix(r.Path, "/repos/") {
			out = append(out, r.Method+" "+r.Path)
		}
	}
	return out
}

const stopMsg = "aayu: @rahulnakmol stopped the mission of #889 (spec drift). Stop your subagents, commit your work at your next checkpoint, push it, and stop. Keep the branch and the PR. Start nothing new until aayu says resume."

// Criterion 1: a stop with three stories running, after the presence check,
// writes the stop where mission next reads it, so next starts nothing; it
// tells the Amp, Claude Code, and Cursor threads to commit at their next
// checkpoint and stop; and it changes no claim, branch, label, or PR.
func TestMissionStopKeepsClaimsAndPRs(t *testing.T) {
	w := newThreads(t)
	w.startedMission(889, []int{20, 21, 22, 23})
	w.run(20, "amp", "T-01a00000-0000-0000-0000-000000000020", "https://ampcode.com/threads/T-01a00000-0000-0000-0000-000000000020")
	w.run(21, "claude", "session_21", "https://claude.ai/code/session_21")
	w.run(22, "cursor", "bc-22/run-1", "https://cursor.com/agents?id=bc-22")
	w.status(23, "ready", "")
	pr := w.g.AddPR("feat: story 20", "Refs #20", "rahulnakmol", "story/20-item", "aaaa", "dev")
	w.edit(func() {
		w.g.Issues[20].Labels = append(w.g.Issues[20].Labels, fake.Label{Name: LeaseLabel})
		w.g.Issues[20].Comments = append(w.g.Issues[20].Comments, &fake.Comment{ID: 8020, Body: leaseBody("rahulnakmol@laptop", time.Now().Add(time.Hour))})
	})
	putLedger(t, w.bare, MissionClaimsPath(889), fmt.Sprintf(`{"story":21,"token":"t21","at":%q}`+"\n", time.Now().UTC().Format(time.RFC3339)))
	claims := w.claims(889)
	labels := map[int][]string{20: w.g.LabelNames(20), 21: w.g.LabelNames(21), 22: w.g.LabelNames(22)}
	// #20's thread opens its PR while the stop reaches it: its sync lands on the ledger first.
	w.x.on = func(name string) {
		if name == "amp" {
			b, _ := json.Marshal(delegate.State{Story: 20, Receiver: "amp", Mode: "cloud", Task: "T-01a00000-0000-0000-0000-000000000020", Status: dlgPROpen, PR: 7020, StartedAt: "2026-10-26T09:00:00Z"})
			putLedger(t, w.bare, delegate.LedgerPath(20), string(b))
		}
	}
	w.g.Reset()

	code, out, errOut := w.control("rahulnakmol", "stop", 889, "--reason", "spec drift")
	if code != 0 || w.codes() != 1 || strings.Index(errOut, "Stops the mission of #889 (spec drift)") > strings.Index(errOut, "/login/device") {
		t.Fatalf("mission stop: exit %d, device codes %d: %s%s", code, w.codes(), out, errOut)
	}
	told := "told to stop its subagents, commit at its next checkpoint, and stop"
	for _, n := range []int{20, 21, 22} {
		if !strings.Contains(out, fmt.Sprintf("#%d %s\n", n, told)) {
			t.Errorf("the stop result does not say #%d was told: %s", n, out)
		}
	}
	if calls := w.x.all(); !slices.Equal(calls, []string{
		"amp threads continue T-01a00000-0000-0000-0000-000000000020 -ox " + stopMsg,
		"claude -p " + stopMsg + " --cloud session_21 --output-format json",
	}) {
		t.Fatalf("CLI calls: %q", calls)
	}
	w.cursor.mu.Lock()
	reqs := slices.Clone(w.cursor.reqs)
	w.cursor.mu.Unlock()
	if len(reqs) != 1 || reqs[0]["path"] != "/v1/agents/bc-22/runs" || reqs[0]["body"].(map[string]any)["prompt"].(map[string]any)["text"] != stopMsg {
		t.Fatalf("Cursor requests: %v", reqs)
	}
	// The stop is recorded on #20's delegation as the sync left it, not on the copy read before.
	var st delegate.State
	_ = json.Unmarshal([]byte(git(t, w.bare, "show", "ledger:"+delegate.LedgerPath(20))), &st)
	if st.Status != dlgPROpen || st.PR != 7020 || len(st.Events) == 0 || st.Events[len(st.Events)-1].Detail != "delegate.mission-stop" {
		t.Fatalf("#20's delegation after the stop: %+v", st)
	}
	p, raw := w.plan(889)
	last := p.Controls[len(p.Controls)-1]
	if p.Pause == nil || p.Pause.Reason != "spec drift" || p.Pause.By != "rahulnakmol" ||
		last != (MissionControl{Action: "stop", By: "rahulnakmol", Tool: "amp", At: last.At, Presence: "device-flow", Reason: "spec drift"}) {
		t.Fatalf("plan.json after the stop:\n%s", raw)
	}
	// Claims, branches, labels, and the PR stay: the stop wrote nothing to the repository.
	if got := w.repoWrites(); len(got) != 0 {
		t.Fatalf("the stop changed the repository: %v", got)
	}
	for n, want := range labels {
		if got := w.g.LabelNames(n); !slices.Equal(got, want) {
			t.Errorf("#%d labels %v, want %v", n, got, want)
		}
	}
	w.g.Mu.Lock()
	open := w.g.Pulls[pr.Number].State == "open"
	w.g.Mu.Unlock()
	if !open {
		t.Fatal("the stop closed #20's PR")
	}
	// mission next starts nothing, and claims nothing.
	if code, out, errOut := w.next(889); code != ExitBlocked || out != "" || !strings.Contains(errOut, "The mission of #889 is paused: spec drift. Nothing starts.") {
		t.Fatalf("mission next after the stop: exit %d: %s%s", code, out, errOut)
	}
	if got := w.claims(889); !maps.Equal(got, claims) || got[21] != 1 {
		t.Fatalf("claims after the stop: %v, before: %v", got, claims)
	}
	// A second stop changes nothing.
	n := w.commits()
	if code, _, errOut := w.control("rahulnakmol", "stop", 889, "--reason", "again"); code != ExitBlocked || w.commits() != n ||
		!strings.Contains(errOut, "The mission of #889 is stopped already (spec drift). Nothing changed.") {
		t.Fatalf("a second stop: exit %d: %s", code, errOut)
	}
}

// Criterion 2: a Codex cloud task and a Copilot task cannot hear a stop: the
// result names each with its task URL, and nothing is sent to them. A run on
// its builder's machine, and a thread the follow-up fails to reach, are named
// the same way.
func TestMissionStopNamesUnreachedTasks(t *testing.T) {
	w := newThreads(t)
	w.startedMission(889, []int{20, 21, 22, 23, 24})
	w.run(20, "amp", "T-01a00000-0000-0000-0000-000000000020", "https://ampcode.com/threads/T-01a00000-0000-0000-0000-000000000020")
	w.run(21, "codex", "task_e_21", "https://chatgpt.com/codex/tasks/task_e_21")
	w.run(22, "copilot", "22", "https://github.com/o/r/agents/22")
	w.run(23, "claude", "", "")
	b, _ := json.Marshal(delegate.State{Story: 23, Receiver: "claude", Mode: "local", Status: dlgWorking})
	putLedger(t, w.bare, delegate.LedgerPath(23), string(b))
	w.run(24, "amp", "T-01a00000-0000-0000-0000-0000000000ff", "https://ampcode.com/threads/T-01a00000-0000-0000-0000-0000000000ff")
	code, out, errOut := w.control("rahulnakmol", "stop", 889, "--reason", "spec drift")
	for _, want := range []string{
		"#20 told to stop its subagents, commit at its next checkpoint, and stop\n",
		"#21 not reached: https://chatgpt.com/codex/tasks/task_e_21; it runs to its end, and its PR waits\n",
		"#22 not reached: https://github.com/o/r/agents/22; it runs to its end, and its PR waits\n",
		"#23 not reached: #23's Claude Code run on its builder's machine; it runs to its end, and its PR waits\n",
		"#24 not reached: https://ampcode.com/threads/T-01a00000-0000-0000-0000-0000000000ff; it runs to its end, and its PR waits\n",
	} {
		if code != 0 || !strings.Contains(out, want) {
			t.Fatalf("mission stop, want %q: exit %d: %s%s", want, code, out, errOut)
		}
	}
	if calls := w.x.all(); len(calls) != 2 || !strings.HasPrefix(calls[0], "amp threads continue T-01a00000-0000-0000-0000-000000000020") {
		t.Fatalf("only the Amp threads are told the stop: %q", calls)
	}
	// The JSON result says the same.
	m := w.missionWorld
	m.setPlan(889, func(p *MissionPlan) { p.Pause = nil })
	code, out, _ = w.control("rahulnakmol", "stop", 889, "--reason", "spec drift", "--json")
	var rep MissionControlReport
	if err := json.Unmarshal([]byte(out), &rep); code != 0 || err != nil || len(rep.Threads) != 5 || rep.Threads[1].Reached || rep.Threads[1].URL != "https://chatgpt.com/codex/tasks/task_e_21" ||
		rep.Threads[4].Reached || !strings.Contains(rep.Threads[4].Error, "thread not found") {
		t.Fatalf("stop --json: exit %d %v: %s", code, err, out)
	}
}

// Criterion 4: stopping #889 leaves #874's mission and the loops running: it
// writes only #889's plan, never sets AAYUDHA_HALT, and trips no breaker.
func TestMissionStopLeavesOtherMissionsAndLoops(t *testing.T) {
	w := newThreads(t)
	w.startedMission(874, []int{950, 951})
	w.startedMission(889, []int{20, 21})
	for _, n := range []int{950, 951, 20, 21} {
		w.status(n, "ready", "")
	}
	before := git(t, w.bare, "rev-parse", "ledger")
	w.g.Reset()
	if code, out, errOut := w.control("rahulnakmol", "stop", 889, "--reason", "spec drift"); code != 0 {
		t.Fatalf("stop 889: exit %d: %s%s", code, out, errOut)
	}
	if got := git(t, w.bare, "diff", "--name-only", before, "ledger"); got != MissionPlanPath(889) {
		t.Fatalf("the stop wrote more than #889's plan:\n%s", got)
	}
	if got := w.repoWrites(); len(got) != 0 { // no repository variable (AAYUDHA_HALT), label, or comment
		t.Fatalf("the stop wrote to the repository: %v", got)
	}
	_ = filepath.WalkDir(w.dir+"/state", func(p string, d fs.DirEntry, err error) error {
		if err == nil && d.Name() == "paused" { // the [budget] breaker's file (Env.PauseFile)
			t.Errorf("the stop paused the agent lanes: %s", p)
		}
		return nil
	})
	if code, out, errOut := w.next(874); code != 0 || out != "start #950\nstart #951\n" {
		t.Fatalf("mission next 874 while #889 is stopped: exit %d: %q%s", code, out, errOut)
	}
	if p, _ := w.plan(874); p.Pause != nil {
		t.Fatalf("#874 is paused: %+v", p.Pause)
	}
	if code, _, errOut := w.next(889); code != ExitBlocked || !strings.Contains(errOut, "is paused: spec drift") {
		t.Fatalf("mission next 889: exit %d: %s", code, errOut)
	}
}

// Criterion 5: resume, after the presence check, clears the stop, continues
// each stopped story thread, and starts stories up to the agents.
func TestMissionResumeContinues(t *testing.T) {
	w := newThreads(t)
	w.startedMission(889, []int{20, 21, 22, 23})
	w.setPlan(889, func(p *MissionPlan) { p.Agents = 3 })
	w.run(20, "amp", "T-01a00000-0000-0000-0000-000000000020", "https://ampcode.com/threads/T-01a00000-0000-0000-0000-000000000020")
	for _, n := range []int{21, 22, 23} {
		w.status(n, "ready", "")
	}
	if code, out, errOut := w.control("rahulnakmol", "stop", 889, "--reason", "spec drift"); code != 0 {
		t.Fatalf("stop: exit %d: %s%s", code, out, errOut)
	}
	w.edit(func() { w.g.Fail = "GET /repos/o/r/issues/889/sub_issues" })
	if code, out, errOut := w.control("rahulnakmol", "resume", 889); code != ExitExternal || w.codes() != 0 {
		t.Fatalf("resume when GitHub fails: exit %d: %s%s", code, out, errOut)
	} else if p, _ := w.plan(889); p.Pause == nil {
		t.Fatal("a failed resume cleared the stop")
	}
	code, out, errOut := w.control("rahulnakmol", "resume", 889)
	if code != 0 || w.codes() != 1 || !strings.Contains(errOut, "Resumes the mission of #889 on amp: aayu reads missions/889/plan.json and GitHub") {
		t.Fatalf("resume: exit %d, device codes %d: %s%s", code, w.codes(), out, errOut)
	}
	for _, want := range []string{"@rahulnakmol resumed the mission of #889 on amp.\n", "#20 told to continue from its last checkpoint\n",
		"  start #21\n  start #22\n  waits #23: no free agent\n"} {
		if !strings.Contains(out, want) {
			t.Fatalf("resume, want %q:\n%s", want, out)
		}
	}
	if calls := w.x.all(); len(calls) != 2 || calls[1] != "amp threads continue T-01a00000-0000-0000-0000-000000000020 -ox aayu: @rahulnakmol resumed the mission of #889. Continue from your last checkpoint." {
		t.Fatalf("CLI calls: %q", calls)
	}
	p, raw := w.plan(889)
	if p.Pause != nil || p.Controls[len(p.Controls)-1].Action != "resume" || w.claims(889)[21] != 1 || w.claims(889)[22] != 1 {
		t.Fatalf("after resume: claims %v\n%s", w.claims(889), raw)
	}
	if code, _, errOut := w.control("rahulnakmol", "resume", 889); code != ExitBlocked || !strings.Contains(errOut, "The mission of #889 is not stopped. Nothing changed.") {
		t.Fatalf("resume of a running mission: exit %d: %s", code, errOut)
	}
}

// Criterion 6: an agent lane's stop is refused before anything is read, with
// no device code, and the refusal says what a person does instead. Resume is
// refused the same way.
func TestMissionStopRefusedForAgentLane(t *testing.T) {
	m := newMissionWorld(t)
	m.startedMission(889, []int{20})
	n, raw := m.commits(), git(t, m.bare, "show", "ledger:"+MissionPlanPath(889))
	m.g.Reset()
	code, out, errOut := m.mission(m.agent, "rahulnakmol", "stop", 889)
	if code != ExitBlocked || m.codes() != 0 || m.commits() != n || len(m.g.Writes()) != 0 || git(t, m.bare, "show", "ledger:"+MissionPlanPath(889)) != raw ||
		!strings.Contains(errOut, "aayu: mission stop is a person's decision and is refused for an agent lane (AMP_THREAD_ID)\n") ||
		!strings.Contains(errOut, "next: a person runs it in their own terminal, or presses Stop in aayu inbox") {
		t.Fatalf("agent mission stop: exit %d, device codes %d: %s%s", code, m.codes(), out, errOut)
	}
	if code, _, errOut := m.mission(m.agent, "rahulnakmol", "resume", 889); code != ExitBlocked || m.commits() != n ||
		!strings.Contains(errOut, "mission resume is a person's decision and is refused for an agent lane (AMP_THREAD_ID)") {
		t.Fatalf("agent mission resume: exit %d: %s", code, errOut)
	}
}

// A stop or resume that cannot apply is refused before the device code and
// writes nothing; --dry-run asks for the device code, says what it would do,
// and writes nothing.
func TestMissionControlRefusedOrDryRun(t *testing.T) {
	m := newMissionWorld(t)
	m.startedMission(889, []int{20, 21})
	if code, out, errOut := m.mission(m.person, "rahulnakmol", "stop", 889, "--story", "20", "--reason", "wrong approach"); code != 0 {
		t.Fatalf("stop --story 20: exit %d: %s%s", code, out, errOut)
	}
	n := m.commits()
	for _, tc := range []struct {
		args []string
		code int
		want string
	}{
		{[]string{"stop", "889"}, ExitUsage, "mission stop needs --reason: why you stop it, in one line"},
		{[]string{"stop", "889", "--end", "--story", "20", "--reason", "x"}, ExitUsage, "mission stop --end ends the whole mission: it takes no --story"},
		{[]string{"stop", "777", "--reason", "x"}, ExitBlocked, "#777 has no mission plan: a person approves it to build first. Nothing changed."},
		{[]string{"stop", "889", "--story", "20", "--reason", "again"}, ExitBlocked, "#20 is stopped already (wrong approach). Nothing changed."},
		{[]string{"resume", "889", "--story", "21"}, ExitBlocked, "#21 is not stopped in the mission of #889. Nothing changed."},
	} {
		m.device("rahulnakmol")
		code, out, errOut := run(t, m.person, m.opts, append(append([]string{"mission"}, tc.args...), "--repo", "o/r")...)
		if code != tc.code || m.codes() != 0 || m.commits() != n || !strings.Contains(errOut, tc.want) {
			t.Errorf("mission %v: exit %d, device codes %d, want %d %q: %s%s", tc.args, code, m.codes(), tc.code, tc.want, out, errOut)
		}
	}
	for _, args := range [][]string{{"--reason", "spec drift"}, {"--story", "21", "--reason", "x"}} {
		code, out, errOut := m.mission(m.person, "rahulnakmol", "stop", 889, append(args, "--dry-run")...)
		if code != 0 || m.codes() != 1 || m.commits() != n || !strings.Contains(out, "--dry-run: ledger missions/889/plan.json planned, not written; no thread was told") {
			t.Fatalf("stop %v --dry-run: exit %d: %s%s", args, code, out, errOut)
		}
	}
	if code, out, errOut := m.mission(m.person, "rahulnakmol", "resume", 889, "--story", "20", "--dry-run"); code != 0 || m.commits() != n ||
		!strings.Contains(out, "  Resumes #20 of the mission of #889 on amp: only its thread continues\n") {
		t.Fatalf("resume --dry-run: exit %d: %s%s", code, out, errOut)
	}
	// A person who denies the device code, has no GitHub token, or whose tool aayu cannot read, changes nothing.
	tool := m.opts.RepoTool
	for action, story := range map[string]string{"stop": "21", "resume": "20"} {
		args := []string{"mission", action, "889", "--story", story, "--reason", "x", "--repo", "o/r"}
		m.edit(func() {
			m.g.Device = &fake.DeviceFlow{ClientID: DefaultDeviceClientID, Approver: "rahulnakmol", Outcome: "deny"}
		})
		if code, _, errOut := run(t, m.person, m.opts, args...); code == 0 || m.codes() != 1 || m.commits() != n {
			t.Errorf("%s with the device code denied: exit %d: %s", action, code, errOut)
		}
		if code, _, errOut := run(t, map[string]string{"XDG_STATE_HOME": m.dir + "/state"}, m.opts, args...); code == 0 || m.commits() != n {
			t.Errorf("%s with no GitHub token: exit %d: %s", action, code, errOut)
		}
		m.opts.RepoTool = func(context.Context, string) (string, error) { return "", fmt.Errorf("config unreadable") }
		if code, _, errOut := run(t, m.person, m.opts, args...); code != ExitExternal || m.commits() != n || !strings.Contains(errOut, "config unreadable") {
			t.Errorf("%s when aayu cannot read the tool: exit %d: %s", action, code, errOut)
		}
		m.opts.RepoTool = tool
	}
	// A resume while the [budget] breaker pauses the agent lanes clears the stop, and says why nothing starts.
	if code, out, errOut := m.mission(m.person, "rahulnakmol", "stop", 889, "--reason", "spec drift"); code != 0 {
		t.Fatalf("stop: exit %d: %s%s", code, out, errOut)
	}
	pause := filepath.Join(m.dir, ".aayudha", "state", "paused")
	_ = os.MkdirAll(filepath.Dir(pause), 0o755)
	_ = os.WriteFile(pause, []byte("budget"), 0o644)
	code, out, errOut := m.mission(m.person, "rahulnakmol", "resume", 889)
	_ = os.Remove(pause)
	if p, _ := m.plan(889); code != 0 || p.Pause != nil || !strings.Contains(out, "  nothing starts: mission next: agent lanes are paused by the [budget] breaker") {
		t.Fatalf("resume while the breaker pauses the lanes: exit %d: %s%s", code, out, errOut)
	}
	// Another person's control that lands while the device code waits wins; this one changes nothing.
	for _, tc := range []struct {
		args []string
		edit func(*MissionPlan)
		want string
	}{
		{[]string{"stop", "--reason", "spec drift"}, func(p *MissionPlan) { p.Pause = &MissionPause{Reason: "budget", By: "alice"} },
			"The mission of #889 was stopped while the code was waiting (budget). Nothing changed."},
		{[]string{"stop", "--story", "21", "--reason", "x"}, func(p *MissionPlan) {
			p.Stopped = append(p.Stopped, MissionStopped{Number: 21, Reason: "y", By: "alice"})
		},
			"#21 was stopped while the code was waiting. Nothing changed."},
		{[]string{"resume", "--story", "20"}, func(p *MissionPlan) { p.Stopped = nil }, "It was resumed while the code was waiting. Nothing changed."},
	} {
		m.edit(func() {
			m.g.Hook = func(_, path string) {
				if path == "/login/device/code" {
					m.setPlan(889, tc.edit)
				}
			}
		})
		before, _ := m.plan(889)
		code, _, errOut := m.mission(m.person, "rahulnakmol", tc.args[0], 889, tc.args[1:]...)
		m.edit(func() { m.g.Hook = nil })
		if after, _ := m.plan(889); code != ExitBlocked || len(after.Controls) != len(before.Controls) || !strings.Contains(errOut, tc.want) {
			t.Errorf("mission %v with a control while the code waits: exit %d: %s", tc.args, code, errOut)
		}
		m.setPlan(889, func(p *MissionPlan) {
			p.Pause, p.Stopped = nil, []MissionStopped{{Number: 20, Reason: "wrong approach", By: "rahulnakmol"}}
		})
	}
	m.setPlan(889, func(p *MissionPlan) { p.StartedBy = "" })
	if code, _, errOut := m.mission(m.person, "rahulnakmol", "resume", 889, "--story", "20"); code != ExitBlocked || !strings.Contains(errOut, "The mission of #889 has not started. Nothing changed.") {
		t.Fatalf("resume of a mission not started: exit %d: %s", code, errOut)
	}
	// A plan aayu did not write is refused, never overwritten.
	putLedger(t, m.bare, MissionPlanPath(889), "not json\n")
	for _, action := range []string{"stop", "resume"} {
		if code, _, errOut := m.mission(m.person, "rahulnakmol", action, 889, "--reason", "x"); code != ExitExternal ||
			!strings.Contains(errOut, "ledger missions/889/plan.json: not a record aayu wrote for #889") {
			t.Errorf("%s of a plan aayu did not write: exit %d: %s", action, code, errOut)
		}
	}
}

// Criteria 9, 10, and 11: a stop of one story tells only its thread to stop
// its subagents and stop; the story keeps its claim, its PR, and its slot,
// and the rest of the mission runs on. A Bug stops the same way. Resume of
// the story continues only its thread, and show no longer lists it stopped.
func TestMissionStopOneStoryOrBug(t *testing.T) {
	w := newThreads(t)
	w.startedMission(889, []int{858, 859, 860, 941}, 941)
	w.setPlan(889, func(p *MissionPlan) { p.Agents, p.Budget = 3, 1000 })
	w.run(858, "amp", "T-01a00000-0000-0000-0000-000000000858", "https://ampcode.com/threads/T-01a00000-0000-0000-0000-000000000858")
	w.run(859, "amp", "T-01a00000-0000-0000-0000-000000000859", "https://ampcode.com/threads/T-01a00000-0000-0000-0000-000000000859")
	w.status(860, "ready", "")
	w.status(941, "ready", "")
	w.edit(func() { w.g.Fail = "GET /repos/o/r/issues/858" })
	if code, out, errOut := w.control("rahulnakmol", "stop", 889, "--story", "858", "--reason", "wrong approach"); code != ExitExternal || w.codes() != 0 {
		t.Fatalf("stop --story when GitHub fails: exit %d: %s%s", code, out, errOut)
	} else if p, _ := w.plan(889); len(p.Stopped) != 0 {
		t.Fatal("a failed stop wrote the plan")
	}
	w.g.Reset()
	code, out, errOut := w.control("rahulnakmol", "stop", 889, "--story", "858", "--reason", "wrong approach")
	if code != 0 || w.codes() != 1 || !strings.Contains(errOut, "Stops #858 of the mission of #889 (wrong approach): its thread stops its subagents, commits at its next checkpoint, and stops; it keeps its claim, its PR, and its slot") ||
		!strings.Contains(out, "#858 told to stop its subagents, commit at its next checkpoint, and stop\n") || strings.Contains(out, "#859") {
		t.Fatalf("stop --story 858: exit %d: %s%s", code, out, errOut)
	}
	if calls := w.x.all(); len(calls) != 1 || calls[0] != "amp threads continue T-01a00000-0000-0000-0000-000000000858 -ox aayu: @rahulnakmol stopped #858 (wrong approach). Stop your subagents, commit your work at your next checkpoint, push it, and stop. Keep the branch and the PR. Start nothing new until aayu says resume." {
		t.Fatalf("only #858's thread is told: %q", calls)
	}
	p, raw := w.plan(889)
	if p.Pause != nil || len(p.Stopped) != 1 || p.Stopped[0].Number != 858 || p.Stopped[0].Reason != "wrong approach" ||
		p.Controls[len(p.Controls)-1] != (MissionControl{Action: "stop", By: "rahulnakmol", Tool: "amp", At: p.Stopped[0].At, Presence: "device-flow", Story: 858, Reason: "wrong approach"}) {
		t.Fatalf("plan.json after stop --story:\n%s", raw)
	}
	if got := w.repoWrites(); len(got) != 0 || !slices.Contains(w.g.LabelNames(858), "status:in-progress") {
		t.Fatalf("stop --story changed the repository: %v %v", got, w.g.LabelNames(858))
	}
	// #858 holds its slot: with 3 agents, #859 runs and one of #860 and #941 starts.
	if code, out, errOut := w.next(889); code != 0 || out != "waits #858: stopped (wrong approach) · subthread T-01a00000-0000-0000-0000-000000000858\nstart #860\nwaits #941 (bug): no free agent (3 of 3 busy after these starts)\n" {
		t.Fatalf("mission next with #858 stopped: exit %d: %q%s", code, out, errOut)
	}
	if code, out, _ := run(t, w.person, w.opts, "mission", "show", "889", "--repo", "o/r"); code != 0 || !strings.Contains(out, "#858 stopped (wrong approach)") {
		t.Fatalf("show with #858 stopped: %s", out)
	}
	// A Bug of the mission stops and resumes as a story does.
	if code, out, errOut := w.control("rahulnakmol", "stop", 889, "--story", "941", "--reason", "repro wrong"); code != 0 {
		t.Fatalf("stop --story 941: exit %d: %s%s", code, out, errOut)
	}
	if p, _ := w.plan(889); p.stopped(941) == nil || p.stopped(941).Reason != "repro wrong" {
		t.Fatalf("Bug #941 not stopped: %+v", p.Stopped)
	}
	if code, out, errOut := w.control("rahulnakmol", "resume", 889, "--story", "941"); code != 0 || w.plan2(889).stopped(941) != nil {
		t.Fatalf("resume --story 941: exit %d: %s%s", code, out, errOut)
	}
	// Resume of #858: only its thread continues, and show no longer lists it stopped.
	calls := len(w.x.all())
	code, out, errOut = w.control("rahulnakmol", "resume", 889, "--story", "858")
	if code != 0 || !strings.Contains(out, "@rahulnakmol resumed #858 in the mission of #889 on amp.\n  #858 told to continue from its last checkpoint\n") || strings.Contains(out, "start ") {
		t.Fatalf("resume --story 858: exit %d: %s%s", code, out, errOut)
	}
	if got := w.x.all()[calls:]; len(got) != 1 || !strings.HasPrefix(got[0], "amp threads continue T-01a00000-0000-0000-0000-000000000858 -ox aayu: @rahulnakmol resumed #858.") {
		t.Fatalf("resume --story 858 told: %q", got)
	}
	if code, out, _ := run(t, w.person, w.opts, "mission", "show", "889", "--repo", "o/r"); code != 0 || strings.Contains(out, "stopped") || w.plan2(889).Stopped != nil {
		t.Fatalf("show after resume --story 858: %s", out)
	}
	// A number that is not the mission's is refused.
	w.issue(990, "Elsewhere", "story", 0, "")
	if code, _, errOut := w.control("rahulnakmol", "stop", 889, "--story", "990", "--reason", "x"); code != ExitBlocked || !strings.Contains(errOut, "#990 is not a story or Bug of #889. Nothing changed.") {
		t.Fatalf("stop --story 990: exit %d: %s", code, errOut)
	}
}

func (m *missionWorld) plan2(f int) *MissionPlan { p, _ := m.plan(f); return &p }

// bobs is mission #874, started by @bob on codex, with #950 merged and #951
// and #952 running as Codex cloud tasks; #951 has a PR.
func bobs(t *testing.T) (*threads, int) {
	w := newThreads(t)
	w.startedMission(874, []int{950, 951, 952})
	w.setPlan(874, func(p *MissionPlan) {
		p.StartedBy, p.Tool = "bob", "codex"
		p.Controls[0].By, p.Controls[0].Tool = "bob", "codex"
	})
	w.closeIssue(950)
	w.status(950, "merged", "")
	w.run(951, "codex", "task_e_951", "https://chatgpt.com/codex/tasks/task_e_951")
	w.run(952, "codex", "task_e_952", "https://chatgpt.com/codex/tasks/task_e_952")
	pr := w.g.AddPR("feat: story 951", "Refs #951", "bob", "story/951-item", "bbbb", "dev")
	return w, pr.Number
}

// as makes login the account the token acts as, with tool for this repository.
func (w *threads) as(login, tool string) {
	w.g.Mu.Lock()
	w.g.Viewer = login
	w.g.Mu.Unlock()
	w.mu.Lock()
	w.tool = tool
	w.mu.Unlock()
}

// Criterion 12: @alice, on claude, stops @bob's mission on codex: the stop
// goes on the plan, ship refuses its PRs (TestShipRefusesStoppedMission),
// each Codex task is named as not reached, and the Kanha's next wake reads
// the stop.
func TestMissionStopFromAnyTool(t *testing.T) {
	w, _ := bobs(t)
	w.as("alice", "claude")
	code, out, errOut := w.control("alice", "stop", 874, "--reason", "spec drift")
	if code != 0 || w.codes() != 1 ||
		!strings.Contains(out, "#951 not reached: https://chatgpt.com/codex/tasks/task_e_951; it runs to its end, and its PR waits\n") ||
		!strings.Contains(out, "#952 not reached: https://chatgpt.com/codex/tasks/task_e_952; it runs to its end, and its PR waits\n") {
		t.Fatalf("alice's stop: exit %d: %s%s", code, out, errOut)
	}
	p, raw := w.plan(874)
	if p.Pause == nil || p.Pause.By != "alice" || p.Controls[len(p.Controls)-1].By != "alice" || p.Controls[len(p.Controls)-1].Tool != "claude" || p.StartedBy != "bob" || len(w.x.all()) != 0 {
		t.Fatalf("plan.json after alice's stop:\n%s", raw)
	}
	// The Kanha, on codex, reads the stop at its next wake.
	if code, _, errOut := run(t, w.agent, w.opts, "mission", "next", "874", "--repo", "o/r", "--host", "codex"); code != ExitBlocked ||
		!strings.Contains(errOut, "The mission of #874 is paused: spec drift. Nothing starts.") {
		t.Fatalf("the Kanha's next wake: exit %d: %s", code, errOut)
	}
}

// Criterion 13: @alice, on claude, cannot resume a mission that runs on
// codex: the refusal names the recorded tool and its starter, before any
// device code, and nothing changes.
func TestMissionResumeRefusedOnOtherTool(t *testing.T) {
	w, _ := bobs(t)
	w.setPlan(874, func(p *MissionPlan) { p.Pause = &MissionPause{Reason: "spec drift", By: "alice"} })
	w.as("alice", "claude")
	n := w.commits()
	w.g.Reset()
	code, out, errOut := w.control("alice", "resume", 874)
	if code != ExitBlocked || w.codes() != 0 || w.commits() != n || len(w.repoWrites()) != 0 ||
		!strings.Contains(errOut, "#874's mission runs on codex, the tool of @bob, who started it: run this there") {
		t.Fatalf("alice's resume: exit %d, device codes %d: %s%s", code, w.codes(), out, errOut)
	}
	// In a Tatkal mission a story that @carol started on her own tool resumes on that tool (ST9), not the mission's.
	w.setPlan(874, func(p *MissionPlan) {
		p.Pause, p.Stopped = nil, []MissionStopped{{Number: 951, Reason: "wrong approach", By: "bob"}}
		p.StoryStarts = []MissionStoryStart{{Number: 951, Tool: "claude", StartedBy: "carol", StartedAt: "2026-10-26T10:00:00Z"}}
	})
	w.as("bob", "codex")
	if code, _, errOut := w.control("bob", "resume", 874, "--story", "951"); code != ExitBlocked || w.codes() != 0 ||
		!strings.Contains(errOut, "#951 runs on claude, the tool of @carol, who started it: run this there") {
		t.Fatalf("bob's resume of carol's story on codex: exit %d: %s", code, errOut)
	}
	w.as("alice", "claude")
	if code, out, errOut := w.control("alice", "resume", 874, "--story", "951"); code != 0 || !strings.Contains(out, "@alice resumed #951 in the mission of #874 on claude.") {
		t.Fatalf("alice's resume of carol's story on claude: exit %d: %s%s", code, out, errOut)
	}
}

// Criterion 14: @dana, on codex, resumes @bob's mission: the plan records
// the resume as hers, on codex, and the starter stays @bob.
func TestMissionControlsRecordWho(t *testing.T) {
	w, _ := bobs(t)
	w.setPlan(874, func(p *MissionPlan) { p.Pause = &MissionPause{Reason: "spec drift", By: "alice"} })
	// The plan as an earlier v1 aayu wrote it: a gate as a string, and a control's "action".
	_, raw := w.plan(874)
	v1 := regexp.MustCompile(`"gate": \{[^}]*\}`).ReplaceAllString(strings.Replace(raw, `"control": "start"`, `"action": "start"`, 1), `"gate": "runs when its last story merges"`)
	if v1 == raw || !strings.Contains(v1, `"action": "start"`) {
		t.Fatalf("no v1 plan:\n%s", v1)
	}
	putLedger(t, w.bare, MissionPlanPath(874), v1)
	w.as("dana", "codex")
	code, out, errOut := w.control("dana", "resume", 874)
	if code != 0 || w.codes() != 1 || !strings.Contains(out, "@dana resumed the mission of #874 on codex.") {
		t.Fatalf("dana's resume: exit %d: %s%s", code, out, errOut)
	}
	p, raw := w.plan(874)
	last := p.Controls[len(p.Controls)-1]
	at, err := time.Parse(time.RFC3339, last.At)
	if p.Pause != nil || p.StartedBy != "bob" || err != nil || time.Since(at) > time.Minute || p.Controls[0].Action != "start" || p.Milestones[0].Gate.State != GateWaits ||
		last != (MissionControl{Action: "resume", By: "dana", Tool: "codex", At: last.At, Presence: "device-flow"}) ||
		!strings.Contains(raw, `"control": "resume",`+"\n"+`      "by": "dana",`+"\n"+`      "tool": "codex",`+"\n"+`      "at": "`) {
		t.Fatalf("plan.json after dana's resume:\n%s", raw)
	}
	if msg := git(t, w.bare, "log", "--format=%s", "ledger", "--", MissionPlanPath(874)); !strings.HasPrefix(msg, "missions: @dana resumed the mission of #874 on codex") {
		t.Fatalf("ledger commit: %s", msg)
	}
}

// Criteria 15 and 16: only @bob, who started #874, ends it. @dana's --end is
// refused before any device code. @bob's, after the presence check, leaves
// #950 merged, releases the claims of #951 and #952, makes them Ready, closes
// #951's PR, ends the mission and its lock, and comments once on each locked
// issue; running it again changes nothing.
func TestMissionEndOnlyByStarter(t *testing.T) {
	w, prN := bobs(t)
	w.setPlan(874, func(p *MissionPlan) {
		p.Lock = &MissionLock{State: "active", ConfirmedBy: "bob", StartedAt: p.StartedAt, Issues: []LockedIssue{{Number: 874}, {Number: 950}, {Number: 951}, {Number: 952}}}
	})
	w.edit(func() {
		w.g.Issues[951].Labels = append(w.g.Issues[951].Labels, fake.Label{Name: LeaseLabel})
		w.g.Issues[951].Comments = append(w.g.Issues[951].Comments, &fake.Comment{ID: 8951, Body: leaseBody("bob@laptop", time.Now().Add(time.Hour))})
	})
	putLedger(t, w.bare, MissionClaimsPath(874), fmt.Sprintf(`{"story":952,"token":"t952","at":%q}`+"\n", time.Now().UTC().Format(time.RFC3339)))

	w.as("dana", "codex")
	n := w.commits()
	w.g.Reset()
	if code, out, errOut := w.control("dana", "stop", 874, "--end", "--reason", "x"); code != ExitBlocked || w.codes() != 0 || w.commits() != n || len(w.repoWrites()) != 0 ||
		!strings.Contains(errOut, "#874 was started by @bob: only @bob may end it") {
		t.Fatalf("dana's --end: exit %d, device codes %d: %s%s", code, w.codes(), out, errOut)
	}

	w.as("bob", "codex")
	// GitHub goes first, and each step is safe to run again: a run that fails
	// at any step leaves the mission running, and the next run goes on from it.
	for _, fail := range []string{"GET /user", "GET /repos/o/r/issues/874/sub_issues", "GET /repos/o/r/pulls", "PATCH /repos/o/r/issues/comments/8951", "POST /repos/o/r/issues/951/labels",
		"GET /repos/o/r/issues/874/comments", "POST /repos/o/r/issues/951/comments", fmt.Sprintf("PATCH /repos/o/r/pulls/%d", prN)} {
		w.edit(func() { w.g.Fail = fail })
		if code, out, errOut := w.control("bob", "stop", 874, "--end", "--reason", "superseded"); code != ExitExternal {
			t.Fatalf("--end failing at %s: exit %d: %s%s", fail, code, out, errOut)
		}
		if p, _ := w.plan(874); p.State != "" || p.Lock.State != "active" {
			t.Fatalf("--end failing at %s ended the mission", fail)
		}
	}
	code, out, errOut := w.control("bob", "stop", 874, "--end", "--reason", "superseded")
	if code != 0 || w.codes() != 1 || !strings.Contains(errOut, "Ends the mission of #874 (superseded)") {
		t.Fatalf("bob's --end: exit %d: %s%s", code, out, errOut)
	}
	w.g.Mu.Lock()
	issue950, prState := w.g.Issues[950].State, w.g.Pulls[prN].State
	w.g.Mu.Unlock()
	if issue950 != "closed" || !slices.Contains(w.g.LabelNames(950), "status:merged") || prState != "closed" {
		t.Fatalf("#950 %s %v, PR #%d %s: %s%s", issue950, w.g.LabelNames(950), prN, prState, out, errOut)
	}
	for _, k := range []int{951, 952} {
		if got := w.g.LabelNames(k); !slices.Contains(got, "status:ready") || slices.Contains(got, "status:in-progress") || slices.Contains(got, LeaseLabel) {
			t.Errorf("#%d labels after --end: %v", k, got)
		}
	}
	if got := w.claims(874); got[952] != 0 {
		t.Fatalf("#952's claim after --end: %v", got)
	}
	p, raw := w.plan(874)
	if p.State != "ended (superseded)" || p.Lock.State != "ended" || p.Lock.EndReason == nil || *p.Lock.EndReason != "superseded" || p.Lock.EndedAt == nil ||
		p.Controls[len(p.Controls)-1].Action != "end" || p.Controls[len(p.Controls)-1].By != "bob" {
		t.Fatalf("plan.json after --end:\n%s", raw)
	}
	w.g.Mu.Lock()
	at := w.g.Issues[951].Comments[len(w.g.Issues[951].Comments)-1].Body
	w.g.Mu.Unlock()
	want := fmt.Sprintf("The mission of #874 ended (superseded) at %s, by @bob: “superseded”. #951 is no longer read-only. Its PR #%d is closed; the story is Ready again.", time.Now().Format("15:04"), prN)
	if !strings.HasSuffix(at, want) {
		t.Fatalf("#951's comment:\n%s\nwant:\n%s", at, want)
	}
	for k, tail := range map[int]string{874: "#874 is no longer read-only.", 950: "#950 is no longer read-only.", 952: "#952 is no longer read-only. The story is Ready again."} {
		w.g.Mu.Lock()
		cs := w.g.Issues[k].Comments
		w.g.Mu.Unlock()
		if got := cs[len(cs)-1].Body; !strings.HasSuffix(got, tail) {
			t.Errorf("#%d's comment: %s", k, got)
		}
		if got := slices.DeleteFunc(slices.Clone(cs), func(c *fake.Comment) bool { return !strings.Contains(c.Body, "aayudha:mission-ended") }); len(got) != 1 {
			t.Errorf("#%d has %d mission-ended comments after the failed runs", k, len(got))
		}
	}
	// #874 is no longer work in flight: show says it ended, and next starts nothing.
	if code, out, _ := run(t, w.person, w.opts, "mission", "show", "874", "--repo", "o/r"); code != 0 || !strings.HasPrefix(strings.SplitN(out, "\n", 2)[1], "ended (superseded)\n") {
		t.Fatalf("show after --end: %s", out)
	}
	if code, _, errOut := run(t, w.agent, w.opts, "mission", "next", "874", "--repo", "o/r", "--host", "codex"); code != ExitBlocked || !strings.Contains(errOut, "The mission of #874 ended (superseded). Nothing starts.") {
		t.Fatalf("next after --end: exit %d: %s", code, errOut)
	}
	n = w.commits()
	if code, _, errOut := w.control("bob", "stop", 874, "--end", "--reason", "superseded"); code != ExitBlocked || w.commits() != n || !strings.Contains(errOut, "The mission of #874 ended (superseded). Nothing changed.") {
		t.Fatalf("a second --end: exit %d: %s", code, errOut)
	}
}

// shipMission puts Feature #1's mission plan on the thin fixture's ledger:
// started on amp, with stories #2 and #3 in MS1, and edit applied.
func shipMission(t *testing.T, bare string, edit func(*MissionPlan)) {
	t.Helper()
	p := MissionPlan{Schema: MissionPlanSchema, Feature: 1, Approval: 1, StartedBy: "founder", StartedAt: "2026-10-26T09:00:00Z", Tool: "amp",
		Graph:      []MissionItem{{Number: 2, Kind: "story", Milestone: "MS1"}, {Number: 3, Kind: "story", Milestone: "MS1"}},
		Milestones: []MissionMilestone{{Name: "MS1", Stories: []int{2, 3}, Gate: MilestoneGate{State: GateWaits}}}, Controls: []MissionControl{}}
	edit(&p)
	b, _ := json.MarshalIndent(p, "", "  ")
	putLedger(t, bare, MissionPlanPath(1), string(b))
}

// shipRefused runs ship on PR n and wants a refusal that says want, with
// nothing merged and no pack landed.
func shipRefused(t *testing.T, g *fake.GitHub, env map[string]string, opts Options, bare string, n int, want string) {
	t.Helper()
	code, out := shipCmd(t, env, opts, n)
	g.Mu.Lock()
	merged := g.Pulls[n].Merged
	g.Mu.Unlock()
	if code != ExitBlocked || !strings.Contains(out, want) || merged || git(t, bare, "ls-tree", "-r", "--name-only", "ledger", "proof/") != "" {
		t.Fatalf("ship #%d, want %q: exit %d, merged %t: %s", n, want, code, merged, out)
	}
}

// Criteria 3, 9, and 12: ship refuses a PR of a stopped mission, and of a
// stopped story, before anything lands, and again when the stop comes while
// ship judges the PR; a stop of another story holds this one nothing.
func TestShipRefusesStoppedMission(t *testing.T) {
	g, env, opts, dir, bare, prN, head := thinFixture(t)
	gateRun(t, g, dir, prN, head, coverageHolds)
	g.Mu.Lock()
	g.Pulls[prN].Reviews = []fake.Review{review("reviewer", "User", "APPROVED", head)}
	g.Mu.Unlock()
	shipMission(t, bare, func(p *MissionPlan) { p.Pause = &MissionPause{Reason: "spec drift", By: "rahulnakmol"} })
	shipRefused(t, g, env, opts, bare, prN, fmt.Sprintf("ship #%d: #1's mission is stopped (spec drift): resume it first\n  next: aayu mission resume 1", prN))
	shipMission(t, bare, func(p *MissionPlan) {
		p.Stopped = []MissionStopped{{Number: 2, Reason: "wrong approach", By: "rahulnakmol"}}
	})
	shipRefused(t, g, env, opts, bare, prN, fmt.Sprintf("ship #%d: #2 is stopped (wrong approach): resume it first\n  next: aayu mission resume 1 --story 2", prN))

	// #3 stopped holds #2 nothing; a stop of the mission that comes while ship
	// judges the PR, after its first read, still holds it at the merge.
	shipMission(t, bare, func(p *MissionPlan) {
		p.Stopped = []MissionStopped{{Number: 3, Reason: "wrong approach", By: "rahulnakmol"}}
	})
	g.Mu.Lock()
	parents := 0
	g.Hook = func(method, p string) {
		if method == "GET" && p == "/repos/o/r/issues/2/parent" {
			if parents++; parents == 2 {
				shipMission(t, bare, func(p *MissionPlan) { p.Pause = &MissionPause{Reason: "spec drift", By: "rahulnakmol"} })
			}
		}
	}
	g.Mu.Unlock()
	code, out := shipCmd(t, env, opts, prN)
	g.Mu.Lock()
	merged := g.Pulls[prN].Merged
	g.Hook = nil
	g.Mu.Unlock()
	if code != ExitBlocked || merged || parents != 2 || !strings.Contains(out, "#1's mission is stopped (spec drift): resume it first") ||
		!strings.Contains(git(t, bare, "ls-tree", "-r", "--name-only", "ledger", "proof/"), "proof/2/"+head+"/authoritative/pack.json") {
		t.Fatalf("a stop while ship judged: exit %d, merged %t, parent reads %d: %s", code, merged, parents, out)
	}
	shipMission(t, bare, func(p *MissionPlan) {
		p.Stopped = []MissionStopped{{Number: 3, Reason: "wrong approach", By: "rahulnakmol"}}
	})
	// A mission ship cannot read holds the PR too.
	g.Mu.Lock()
	g.Fail = "GET /repos/o/r/issues/2/parent"
	g.Mu.Unlock()
	code, out = shipCmd(t, env, opts, prN)
	g.Mu.Lock()
	merged = g.Pulls[prN].Merged
	g.Mu.Unlock()
	if code != ExitExternal || merged {
		t.Fatalf("ship when the mission cannot be read: exit %d: %s", code, out)
	}
	if code, out := shipCmd(t, env, opts, prN); code != 0 {
		t.Fatalf("ship with only #3 stopped: exit %d: %s", code, out)
	}
}

// Criteria 7 and 8: a PR of MS2 waits while MS1's gate has not passed, and
// keeps its story's labels and claim; once the gate passes on dev, it merges.
func TestShipMergesMilestonesInOrder(t *testing.T) {
	g, env, opts, dir, bare, prN, head := thinFixture(t)
	gateRun(t, g, dir, prN, head, coverageHolds)
	g.Mu.Lock()
	g.Pulls[prN].Reviews = []fake.Review{review("reviewer", "User", "APPROVED", head)}
	g.Mu.Unlock()
	order := func(ms1, ms2 string) func(*MissionPlan) {
		return func(p *MissionPlan) {
			p.Graph = []MissionItem{{Number: 2, Kind: "story", Milestone: "MS3"}, {Number: 3, Kind: "story", Milestone: "MS1"}, {Number: 4, Kind: "story", Milestone: "MS2"}}
			p.Milestones = []MissionMilestone{{Name: "MS1", Stories: []int{3}, Gate: MilestoneGate{State: ms1}}, {Name: "MS2", Stories: []int{4}, Gate: MilestoneGate{State: ms2}},
				{Name: "MS3", Stories: []int{2}, Gate: MilestoneGate{State: GateWaits}}}
		}
	}
	labels := g.LabelNames(2)
	shipMission(t, bare, order(GateFailed, GateWaits))
	shipRefused(t, g, env, opts, bare, prN, fmt.Sprintf("ship #%d: #2 is in MS3: it merges after MS1's gate passes", prN))
	shipMission(t, bare, order(GatePassed, GateWaits))
	shipRefused(t, g, env, opts, bare, prN, fmt.Sprintf("ship #%d: #2 is in MS3: it merges after MS2's gate passes", prN))
	if got := g.LabelNames(2); !slices.Equal(got, labels) || !slices.Contains(got, LeaseLabel) {
		t.Fatalf("#2 lost its slot: labels %v, before %v", got, labels)
	}
	shipMission(t, bare, func(p *MissionPlan) {
		order(GatePassed, GatePassed)(p)
		p.Milestones[1].Gate = MilestoneGate{State: GatePassed, DevSHA: strings.Repeat("d", 40), At: "2026-10-26T14:10:00Z"}
	})
	if code, out := shipCmd(t, env, opts, prN); code != 0 || !strings.Contains(out, "merged as") {
		t.Fatalf("ship once MS1 and MS2 passed: exit %d: %s", code, out)
	}
}

// Criteria 17 and 18: a PR that refers to the Feature merges on proof only
// as a gate fix that gate_fixes lists on the approved record as it is now;
// it completes no story. Any other such PR is refused before anything lands.
func TestShipMergesOnlyListedFixPR(t *testing.T) {
	g, env, opts, dir, bare, _, head := thinFixture(t)
	record := `{"feature": 1, "version": 1}` + "\n"
	putLedger(t, bare, "specs/1/approved.json", record)
	git(t, dir, "fetch", "--quiet", "origin")
	sum := sha256.Sum256([]byte(record))
	fix := g.AddPR("fix(mission): the MS1 gate of #1", "Refs #1\n\nAayudha-Gate-Fix: MS1 verify:inbox.ship-once\n", "founder", "fix/1-ms1-gate", head, "dev")
	other := g.AddPR("fix: a gate of #1", "Refs #1\n\nFixes the MS1 gate.\n", "founder", "fix/1-other", head, "dev")
	entry := GateFix{PR: fix.Number, Milestone: "MS1", Check: "verify:inbox.ship-once", DevSHA: strings.Repeat("d", 40),
		Approval: GateApproval{Record: "specs/1/approved.json", SHA256: hex.EncodeToString(sum[:])}, At: "2026-10-26T15:00:00Z"}
	shipMission(t, bare, func(p *MissionPlan) { p.GateFixes = []GateFix{entry} })
	shipRefused(t, g, env, opts, bare, other.Number, fmt.Sprintf("ship #%d: #%d is not a listed gate fix of #1", other.Number, other.Number))
	stale := entry
	stale.Approval.SHA256 = strings.Repeat("0", 64)
	shipMission(t, bare, func(p *MissionPlan) { p.GateFixes = []GateFix{stale} })
	shipRefused(t, g, env, opts, bare, fix.Number, fmt.Sprintf("ship #%d: gate fix #%d rests on specs/1/approved.json at 0000000, and #1's approved record has changed since, so its approval no longer covers the fix", fix.Number, fix.Number))
	waits := entry
	waits.Waits = "the fix changes the approved scope"
	shipMission(t, bare, func(p *MissionPlan) { p.GateFixes = []GateFix{waits} })
	shipRefused(t, g, env, opts, bare, fix.Number, fmt.Sprintf("ship #%d: gate fix #%d of #1 waits for a person (the fix changes the approved scope)", fix.Number, fix.Number))
	shipMission(t, bare, func(p *MissionPlan) { p.GateFixes = []GateFix{entry} })

	// Merge on proof (Decision #593), as for a story PR: the base's policy turns proof on, inspect is at merge
	// with rung C, and nothing is in the narrow set.
	gateRunFor(t, g, dir, 1, fix.Number, head, coverageHolds)
	base := strings.Repeat("b", 40)
	conf := "version = 1\n\n[autonomy]\napprover = \"\"\nproof = true\nproof_rungs = [\"A\", \"B\", \"C\"]\n"
	g.Mu.Lock()
	gate := g.Runs[len(g.Runs)-1].ID
	g.RunJobs = map[int64][]fake.CheckRun{gate: {{ID: 7701, Name: "aayudha/inspect", Status: "completed", Conclusion: "success"}}}
	g.JobLogs = map[int64]string{7701: fmt.Sprintf("2026-10-26T15:00:00.0000000Z inspect #%d at %s: merge (reviewed by codex)\n2026-10-26T15:00:00.0000000Z   review: same-family review (C): codex (openai), builder openai\n", fix.Number, short(head))}
	g.Trees[head], g.Trees[base] = []map[string]any{}, []map[string]any{}
	g.RefFiles = map[string]map[string]string{base: {".aayudha/config.toml": conf}, head: {".aayudha/config.toml": conf}}
	for _, pr := range g.Pulls { // they share the head, so GitHub's comparison may answer for any of them
		pr.Files = []string{"empty.txt"}
	}
	g.Mu.Unlock()
	labels := g.LabelNames(1)
	code, out := shipCmd(t, env, opts, fix.Number)
	g.Mu.Lock()
	merged, feature := g.Pulls[fix.Number].Merged, g.Issues[1].State
	g.Mu.Unlock()
	if code != 0 || !merged || !strings.Contains(out, "the gate fix of MS1 of #1 for verify:inbox.ship-once") || !strings.Contains(out, "human pass: proof by founder") ||
		feature != "open" || !slices.Equal(g.LabelNames(1), labels) {
		t.Fatalf("ship of the listed fix: exit %d, merged %t, #1 %s %v: %s", code, merged, feature, g.LabelNames(1), out)
	}
	var sr ShipRecord
	_ = json.Unmarshal([]byte(git(t, bare, "show", "ledger:proof/1/"+head+"/authoritative/ship.json")), &sr)
	if sr.GateFix == nil || *sr.GateFix != entry || sr.HumanPass.Kind != "proof" {
		t.Fatalf("ship record: %+v", sr)
	}
}
