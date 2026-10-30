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
	"sync"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/agentenv"
	"github.com/rahulnakmol/agentic-ayudha/internal/delegate"
	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
	"github.com/rahulnakmol/agentic-ayudha/internal/inspect"
	"github.com/rahulnakmol/agentic-ayudha/internal/ready"
)

// Story #1003: `aayu mission recover` and `aayu mission retry` on the
// missionWorld of Stories #997 and #998, with the receivers' CLIs and
// Cursor's API stubbed, some of them failing, and a clock the test moves.

// clock is the time every run of a test reads.
type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time           { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *clock) add(d time.Duration)      { c.mu.Lock(); c.t = c.t.Add(d); c.mu.Unlock() }
func (m *missionWorld) withClock() *clock { c := &clock{t: time.Now()}; m.opts.Now = c.now; return c }

// ampConnected makes Amp's cloud agent connected here: the CLI installed and
// signed in, and the repository's Amp project recorded.
func (m *missionWorld) ampConnected() {
	m.t.Helper()
	look := m.opts.LookPath
	m.opts.LookPath = func(name string) (string, error) {
		if name == "amp" {
			return "/usr/bin/amp", nil
		}
		return look(name)
	}
	for _, env := range []map[string]string{m.person, m.agent} {
		env["AMP_API_KEY"] = "amp-key"
	}
	if err := os.MkdirAll(filepath.Join(m.dir, ".aayudha"), 0o755); err != nil {
		m.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(m.dir, filepath.FromSlash(agentenv.VerifiedPath)), []byte("[cloud.amp]\nmethod = \"cloud\"\nproject = \"o/r\"\nat = \"2026-10-26T00:00:00Z\"\n"), 0o644); err != nil {
		m.t.Fatal(err)
	}
}

// delegated writes delegate/<n>/state.json and marks the story in progress.
func (m *missionWorld) delegated(st delegate.State) {
	m.t.Helper()
	m.status(st.Story, "in-progress", "")
	if st.StartedAt == "" {
		st.StartedAt = "2026-10-26T09:00:00Z"
	}
	b, _ := json.Marshal(st)
	putLedger(m.t, m.bare, delegate.LedgerPath(st.Story), string(b))
}

// recover runs aayu mission recover f as the Kanha in Amp, or on host.
func (m *missionWorld) recover(f int, host string) (int, string, string) {
	m.t.Helper()
	args := []string{"mission", "recover", fmt.Sprint(f), "--repo", "o/r"}
	if host != "" {
		args = append(args, "--host", host)
	}
	return run(m.t, m.agent, m.opts, args...)
}

// handoffs are the receivers' calls that started a story's agent: Amp threads and Codex tasks.
func (m *missionWorld) handoffs(n int) int {
	k := 0
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, c := range m.calls {
		if strings.HasPrefix(c, "amp -ox") || strings.HasPrefix(c, "codex cloud exec") {
			if strings.Contains(m.briefFor(i), fmt.Sprintf("# Delegated story #%d:", n)) {
				k++
			}
		}
	}
	return k
}

// briefFor is the brief of the i-th call (each hand-off sends one).
func (m *missionWorld) briefFor(i int) string {
	k := 0
	for j := 0; j < i; j++ {
		if strings.Contains(m.calls[j], "<brief>") {
			k++
		}
	}
	if k < len(m.briefs) && strings.Contains(m.calls[i], "<brief>") {
		return m.briefs[k]
	}
	return ""
}

// storyLines are recover's lines, one per story.
func storyLines(out string) []string {
	var ls []string
	for _, l := range strings.Split(out, "\n") {
		if strings.HasPrefix(l, "#") {
			ls = append(ls, l)
		}
	}
	return ls
}

const (
	thread60 = "T-01a00000-0000-0000-0000-000000000060"
	thread61 = "T-01a00000-0000-0000-0000-000000000061"
)

// Criteria 1 and 2: the Kanha's orb of #889 was lost with three stories
// running. Recover re-attaches #60, whose Amp thread lives, keeps #61 on
// its PR, and restarts #62, whose Cursor agent is archived with no PR, from
// its branch and brief: one line per story. A second run changes nothing,
// and a story another run holds a claim on is not started a second time.
func TestMissionRecoverReattachesKeepsRestarts(t *testing.T) {
	m := newMissionWorld(t)
	m.ampConnected()
	m.startedMission(889, []int{60, 61, 62})
	for _, n := range []int{60, 61, 62} {
		m.readyStory(n)
	}
	m.delegated(delegate.State{Story: 60, Receiver: "amp", Mode: "cloud", Task: thread60, TaskURL: "https://ampcode.com/threads/" + thread60, Status: dlgWorking})
	m.delegated(delegate.State{Story: 61, Receiver: "amp", Mode: "cloud", Task: thread61, Status: dlgWorking})
	m.delegated(delegate.State{Story: 62, Receiver: "cursor", Mode: "cloud", Task: "bc-62/run-1", TaskURL: "https://cursor.com/agents?id=bc-62", Branch: "story/62-item-62", Status: dlgWorking})
	pr := m.g.AddPR("feat: item 61", "Refs #61", "rahulnakmol", "story/61-item-61", strings.Repeat("a", 40), "dev")
	m.cursor.mu.Lock()
	m.cursor.status = "ARCHIVED"
	m.cursor.mu.Unlock()
	kanha := m.parent(889)
	before := m.handoffs(60) + m.handoffs(61) + m.handoffs(62)

	code, out, errOut := m.recover(889, "")
	if code != 0 {
		t.Fatalf("recover: exit %d: %s%s", code, out, errOut)
	}
	t.Logf("recover:\n%s", out)
	lines := storyLines(out)
	want := []string{
		fmt.Sprintf("#60  re-attached: its Amp thread lives (https://ampcode.com/threads/%s), under the Kanha %s", thread60, kanha.URL),
		fmt.Sprintf("#61  kept on its PR #%d", pr.Number),
		"#62  restarted from story/62-item-62 and its brief: its Cursor task ended (run archived) with no PR",
	}
	if len(lines) != 3 {
		t.Fatalf("recover printed %d story lines, want one per story:\n%s", len(lines), out)
	}
	for i, w := range want {
		if !strings.HasPrefix(lines[i], w) {
			t.Errorf("line %d:\n got %q\nwant %q", i+1, lines[i], w)
		}
	}
	if s := m.state(60); s.Kanha != kanha.Session || s.Task != thread60 {
		t.Errorf("#60 is not re-attached to the Kanha: %+v", s)
	}
	if s := m.state(61); s.PR != pr.Number || s.Status != dlgPROpen {
		t.Errorf("#61 is not kept on its PR: %+v", s)
	}
	s62 := m.state(62)
	if s62.Receiver != "amp" || s62.Branch != "story/62-item-62" || s62.Status != dlgHandedOff {
		t.Errorf("#62 is not restarted on the mission's tool from its branch: %+v", s62)
	}
	if got := m.handoffs(60) + m.handoffs(61) + m.handoffs(62) - before; got != 1 || m.handoffs(62) != 1 {
		t.Fatalf("hand-offs: %d new, #62 %d; want one, for #62: %q", got, m.handoffs(62), m.callLog())
	}
	m.mu.Lock()
	brief := m.briefs[len(m.briefs)-1]
	m.mu.Unlock()
	if !strings.Contains(brief, "# Delegated story #62:") || !strings.Contains(brief, "Continue on its branch `story/62-item-62`") {
		t.Fatalf("#62's brief does not continue its branch:\n%s", brief)
	}
	if n := m.claims(889)[62]; n != 0 {
		t.Errorf("#62's restart claim is still live (%d) after its hand-off is on the delegation", n)
	}

	// Criterion 2: the second run changes nothing, and starts no second agent.
	commits := m.commits()
	code, out, errOut = m.recover(889, "")
	if code != 0 || !strings.Contains(out, "  nothing changed\n") || m.commits() != commits || m.handoffs(62) != 1 ||
		!strings.Contains(out, "#62  re-attached: its Amp thread lives") || !strings.Contains(out, fmt.Sprintf("#61  kept on its PR #%d", pr.Number)) {
		t.Fatalf("second recover: exit %d, commits %s → %s, #62 hand-offs %d: %s%s", code, commits, m.commits(), m.handoffs(62), out, errOut)
	}

	// Another run holds a live claim on #60 when its thread is archived: this run starts nothing for it.
	m.mu.Lock()
	m.archived[thread60] = true
	m.mu.Unlock()
	putLedger(t, m.bare, MissionClaimsPath(889), fmt.Sprintf(`{"story":60,"token":"other","at":%q}`+"\n", time.Now().UTC().Format(time.RFC3339)))
	code, out, errOut = m.recover(889, "")
	if code != 0 || m.handoffs(60) != 0 || !strings.Contains(out, "#60  its Amp thread is archived with no PR: a live claim of another run holds it, so this run starts nothing") {
		t.Fatalf("recover with another run's claim: exit %d, #60 hand-offs %d: %s%s", code, m.handoffs(60), out, errOut)
	}
	// The next wake finds the same claim: the plan records nothing new.
	plan := git(t, m.bare, "show", "ledger:"+MissionPlanPath(889))
	if code, out, errOut := m.recover(889, ""); code != 0 || !strings.Contains(out, "  nothing changed\n") || git(t, m.bare, "show", "ledger:"+MissionPlanPath(889)) != plan {
		t.Fatalf("a second wake with another run's claim: exit %d: %s%s", code, out, errOut)
	}
	// The ledger refuses the restart's claim line: recover says nothing restarted, and starts nothing.
	refuseLedger(t, m.bare, claimsPush)
	code, out, errOut = m.recover(889, "")
	refuseLedger(t, m.bare, "")
	if code != 0 || m.handoffs(60) != 0 || !strings.Contains(out, "#60  its Amp thread is archived with no PR; nothing restarted: ") {
		t.Fatalf("recover when the ledger refuses the claim: exit %d, #60 hand-offs %d: %s%s", code, m.handoffs(60), out, errOut)
	}
}

// claimsPush is a pre-receive test true for a push that changes a claims file.
const claimsPush = `git diff --name-only "$old" "$new" | grep -q claims`

// refuseLedger makes the bare ledger remote refuse every push for which the
// shell test is true, or, with "", accept them all again.
func refuseLedger(t *testing.T, bare, test string) {
	t.Helper()
	hook := filepath.Join(bare, "hooks", "pre-receive")
	if test == "" {
		if err := os.Remove(hook); err != nil {
			t.Fatal(err)
		}
		return
	}
	git(t, bare, "config", "core.hooksPath", "hooks") // over any global hooks path
	if err := os.MkdirAll(filepath.Dir(hook), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hook, []byte("#!/bin/sh\nwhile read old new ref; do\n  if "+test+"; then echo 'the ledger refuses this push' >&2; exit 1; fi\ndone\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}

// codexWorld is a mission on codex whose Kanha runs locally: list is what
// `codex cloud list --json` answers, and listErr fails it.
type codexWorld struct {
	*missionWorld
	mu      sync.Mutex
	list    string
	listErr error
	lists   int
	submit  func(brief string) error // fails a `codex cloud exec`
	status  error                    // fails a `codex cloud status`
	done    string                   // task_e_64's diff: its `codex cloud status` says completed, and `codex cloud diff` returns it
	onList  func(k int)              // runs as the k-th `codex cloud list` answers
}

func newCodexWorld(t *testing.T, f int, stories ...int) *codexWorld {
	m := newMissionWorld(t)
	w := &codexWorld{missionWorld: m, list: `{"tasks": [], "cursor": null}`}
	m.opts.DelegateExec = func(ctx context.Context, dir string, env []string, stdin, name string, args ...string) (string, error) {
		if name == "codex" && len(args) > 1 && args[0] == "cloud" && args[1] == "list" {
			w.mu.Lock()
			w.lists++
			k, on := w.lists, w.onList
			w.mu.Unlock()
			if on != nil {
				on(k)
			}
			w.mu.Lock()
			defer w.mu.Unlock()
			return w.list, w.listErr
		}
		if name == "codex" && len(args) > 1 && args[0] == "cloud" && args[1] == "status" {
			w.mu.Lock()
			err := w.status
			w.mu.Unlock()
			if err != nil {
				return "", err
			}
		}
		w.mu.Lock()
		done := w.done
		w.mu.Unlock()
		if name == "codex" && len(args) > 1 && args[0] == "cloud" && (args[1] == "status" || args[1] == "diff") && done != "" && args[2] == "task_e_64" {
			_, _ = m.exec(ctx, dir, env, stdin, name, args...)
			if args[1] == "diff" {
				return done, nil
			}
			return "Status: completed\n", nil
		}
		if name == "codex" && len(args) > 1 && args[0] == "cloud" && args[1] == "exec" {
			w.mu.Lock()
			fail := w.submit
			w.mu.Unlock()
			if fail != nil {
				if err := fail(args[len(args)-1]); err != nil {
					m.mu.Lock()
					m.calls, m.briefs = append(m.calls, "codex cloud exec <brief> (failed)"), append(m.briefs, args[len(args)-1])
					m.mu.Unlock()
					return "", err
				}
			}
		}
		return m.exec(ctx, dir, env, stdin, name, args...)
	}
	m.startedMission(f, stories)
	m.setPlan(f, func(p *MissionPlan) { p.Tool, p.Controls[0].Tool = "codex", "codex" })
	m.setTool("codex")
	for _, n := range stories {
		m.readyStory(n)
	}
	return w
}

// Bug #1148, criterion 1: the start of #64 got no answer, and its Codex task
// is done with no PR. Recover records the task's ID and URL on #64's
// delegation; the next sync opens the PR from the task's diff, and no second
// task starts. A done task of a story handed off before may be that earlier
// hand-off's: recover leaves it to a person.
func TestMissionRecoverReattachesDoneTask(t *testing.T) {
	w := newCodexWorld(t, 889, 64)
	w.delegated(delegate.State{Story: 64, Feature: 889, Receiver: "codex", Mode: "cloud", Branch: "story/64-item-64", Status: dlgMaybe, Handoffs: 2,
		Events: []delegate.Event{{At: time.Now().UTC().Format(time.RFC3339), Status: dlgMaybe, Detail: "no answer before the timeout"}}})
	w.setPlan(889, func(p *MissionPlan) { p.storyState(64).State = stateMaybe })
	w.list = `{"tasks": [{"id": "task_e_64", "url": "https://chatgpt.com/codex/tasks/task_e_64", "title": "# Delegated story #64: Item 64", "status": "completed"}], "cursor": null}`
	code, out, errOut := w.recover(889, "codex")
	if code != 0 || !strings.Contains(out, "#64  maybe started: a codex task task_e_64 exists for #64 and no PR, and #64 was handed off 2 times: it may be an earlier hand-off's") || w.state(64).Task != "" {
		t.Fatalf("recover of a done task after two hand-offs: exit %d: %s%s", code, out, errOut)
	}
	first := w.state(64)
	first.Handoffs = 1
	w.delegated(first)
	// Another run hands #64 off again while recover reads the host: the re-attach writes nothing.
	w.onList = func(k int) {
		if k == 3 { // the re-attach's own read of the list
			again := first
			again.Handoffs, again.StartedAt = 2, "2026-10-26T10:00:00Z"
			w.delegated(again)
		}
	}
	code, out, errOut = w.recover(889, "codex")
	if code != 0 || !strings.Contains(out, "#64  another run changed its delegation while recover read the host: recover leaves it") || w.state(64).Task != "" || w.state(64).Handoffs != 2 {
		t.Fatalf("recover while another run hands #64 off: exit %d, %+v: %s%s", code, w.state(64), out, errOut)
	}
	// Another run re-attaches #64 while this one finds no task and restarts it: the restart writes nothing over it, and starts nothing.
	w.list = `{"tasks": [], "cursor": null}`
	w.onList = func(int) {
		other := first
		other.Task, other.Status = "task_e_64", dlgHandedOff
		w.delegated(other)
	}
	code, out, errOut = w.recover(889, "codex")
	if st := w.state(64); code != 0 || !strings.Contains(out, "#64  no codex task and no PR for #64: another run changed its delegation") ||
		st.Task != "task_e_64" || st.Status != dlgHandedOff || w.handoffs(64) != 0 {
		t.Fatalf("a restart over another run's re-attach: exit %d, %+v, hand-offs %d: %s%s", code, st, w.handoffs(64), out, errOut)
	}
	// The same for a person's retry: it is refused, and nothing changes.
	w.delegated(first)
	if code, _, errOut := w.mission(w.person, "rahulnakmol", "retry", 889, "--story", "64"); code != ExitBlocked ||
		!strings.Contains(errOut, "#64 changed while the retry ran: another run acted on it, so the retry starts nothing.") || w.state(64).Task != "task_e_64" || w.handoffs(64) != 0 {
		t.Fatalf("a retry over another run's re-attach: exit %d, %+v: %s", code, w.state(64), errOut)
	}
	w.onList = nil
	done := `{"tasks": [{"id": "task_e_64", "url": "https://chatgpt.com/codex/tasks/task_e_64", "title": "# Delegated story #64: Item 64", "status": "completed"}], "cursor": null}`
	w.list = done
	w.delegated(first)
	w.setPlan(889, func(p *MissionPlan) { p.storyState(64).State = stateMaybe })
	// --dry-run says what it would re-attach, and writes nothing.
	commits := w.commits()
	code, out, errOut = run(t, w.agent, w.opts, "mission", "recover", "889", "--repo", "o/r", "--host", "codex", "--dry-run")
	if code != 0 || !strings.Contains(out, "#64  re-attached: its Codex task https://chatgpt.com/codex/tasks/task_e_64 (completed) holds it, and it has no PR yet; --dry-run: the delegation is not updated") ||
		w.commits() != commits || w.state(64).Task != "" {
		t.Fatalf("a dry-run re-attach: exit %d, commits %s → %s: %s%s", code, commits, w.commits(), out, errOut)
	}
	// A second task for #64 appears while recover reads the list again: it picks none.
	base := w.lists
	w.onList = func(k int) {
		if k == base+2 {
			w.list = `{"tasks": [{"id": "task_e_64", "title": "# Delegated story #64: Item 64", "status": "completed"}, {"id": "task_e_64y", "title": "# Delegated story #64: Item 64", "status": "running"}], "cursor": null}`
		}
	}
	code, out, errOut = w.recover(889, "codex")
	if code != 0 || !strings.Contains(out, "#64  maybe started: its Codex task https://chatgpt.com/codex/tasks/task_e_64 (completed) held it, and the host's list changed while recover read it") || w.state(64).Task != "" {
		t.Fatalf("a list that changes during the re-attach: exit %d: %s%s", code, out, errOut)
	}
	w.onList, w.list = nil, done
	code, out, errOut = w.recover(889, "codex")
	if code != 0 || !strings.Contains(out, "#64  re-attached: its Codex task https://chatgpt.com/codex/tasks/task_e_64 (completed) holds it, and it has no PR yet; the next sync reads it\n") {
		t.Fatalf("recover of a done task: exit %d: %s%s", code, out, errOut)
	}
	if st := w.state(64); st.Task != "task_e_64" || st.TaskURL != "https://chatgpt.com/codex/tasks/task_e_64" || st.Status != dlgHandedOff {
		t.Fatalf("#64's delegation after the re-attach: %+v", st)
	}
	w.done = "diff --git a/empty.txt b/empty.txt\nnew file mode 100644\n--- /dev/null\n+++ b/empty.txt\n@@ -0,0 +1 @@\n+Your cart is empty\n"
	calls := len(w.callLog())
	w.opts.CheckpointVerify = holds
	git(t, w.dir, "config", "user.name", "Builder") // the sync commits the task's diff
	git(t, w.dir, "config", "user.email", "builder@example.com")
	code, out, errOut = run(t, w.agent, w.opts, "build", "64", "--delegate", "codex", "--repo", "o/r", "--asked", "aayu mission recover re-attached it")
	got := w.state(64)
	if code != 0 || got.PR == 0 || got.Applied != "task_e_64" || w.handoffs(64) != 0 || !slices.Contains(w.callLog()[calls:], "codex cloud diff task_e_64") {
		t.Fatalf("the sync after the re-attach: exit %d, %+v, calls %q: %s%s", code, got, w.callLog()[calls:], out, errOut)
	}
}

// Criterion 3: the Codex task list answers 401. Recover marks those stories
// unreadable, keeps that source's cursor, starts nothing for them, and
// mission show says unreadable, not quiet.
func TestMissionRecoverMarksUnreadable(t *testing.T) {
	w := newCodexWorld(t, 889, 70, 71)
	for _, n := range []int{70, 71} {
		w.delegated(delegate.State{Story: n, Receiver: "codex", Mode: "cloud", Task: fmt.Sprintf("task_e_%d", n), TaskURL: fmt.Sprintf("https://chatgpt.com/codex/tasks/task_e_%d", n), Status: dlgWorking})
	}
	cursor := "2026-10-26T08:00:00Z"
	w.setPlan(889, func(p *MissionPlan) { p.Sources = []MissionSource{{Name: "codex task list", Cursor: cursor}} })
	if code, out, _ := w.mission(w.person, "rahulnakmol", "show", 889); code != 0 || !strings.Contains(out, "source    codex task list: quiet since its cursor "+cursor) {
		t.Fatalf("show before: %s", out)
	}
	w.listErr = errors.New("codex: exit status 1: Error: request failed with status 401 Unauthorized")
	before := w.handoffs(70) + w.handoffs(71)

	code, out, errOut := w.recover(889, "codex")
	t.Logf("recover:\n%s", out)
	for _, n := range []int{70, 71} {
		if !strings.Contains(out, fmt.Sprintf("#%d  unreadable: codex task list (401): aayu starts nothing for it\n", n)) {
			t.Errorf("recover does not mark #%d unreadable: exit %d: %s%s", n, code, out, errOut)
		}
	}
	if w.handoffs(70)+w.handoffs(71) != before || w.lists != 1 {
		t.Fatalf("recover started something, or read the list %d times: %q", w.lists, w.callLog())
	}
	p, raw := w.plan(889)
	if len(p.Sources) != 1 || p.Sources[0].Cursor != cursor || p.Sources[0].Unreadable != "401" ||
		p.stateOf(70) == nil || p.stateOf(70).State != "unreadable: codex task list (401)" {
		t.Fatalf("plan.json after recover:\n%s", raw)
	}
	_, show, _ := w.mission(w.person, "rahulnakmol", "show", 889)
	if !strings.Contains(show, "#70 unreadable: codex task list (401)") || !strings.Contains(show, "codex task list: unreadable (401) since") ||
		!strings.Contains(show, "cursor "+cursor+" kept") || strings.Contains(show, "quiet") {
		t.Fatalf("show after recover:\n%s", show)
	}
	// The list answers again: the stories run, and the source moves past its cursor only when it tells recover something new.
	w.listErr = nil
	w.list = `{"tasks": [{"id": "task_e_70", "url": "https://chatgpt.com/codex/tasks/task_e_70", "title": "# Delegated story #70: Item 70", "status": "running"},
		{"id": "task_e_71", "url": "https://chatgpt.com/codex/tasks/task_e_71", "title": "# Delegated story #71: Item 71", "status": "running"}], "cursor": null}`
	if code, out, errOut := w.recover(889, "codex"); code != 0 || !strings.Contains(out, "#70  re-attached: its Codex thread lives") {
		t.Fatalf("recover after the list answers: exit %d: %s%s", code, out, errOut)
	}
	if p, raw := w.plan(889); p.Sources[0].Unreadable != "" || p.stateOf(70) != nil {
		t.Fatalf("plan.json after the list answers:\n%s", raw)
	}
	// #71 is older than the list, and its own status cannot be read either: it is unreadable, and nothing starts.
	w.list = `{"tasks": [{"id": "task_e_70", "url": "https://chatgpt.com/codex/tasks/task_e_70", "title": "# Delegated story #70: Item 70", "status": "running"}], "cursor": null}`
	w.status = errors.New("exit status 1: Error: request failed with status 503 Service Unavailable")
	if code, out, errOut := w.recover(889, "codex"); code != 0 || !strings.Contains(out, "#71  unreadable: codex task list (503): aayu starts nothing for it\n") || w.handoffs(71) != before {
		t.Fatalf("recover when #71's status fails: exit %d: %s%s", code, out, errOut)
	}
	if p, raw := w.plan(889); p.Sources[0].Unreadable != "503" || p.stateOf(71) == nil {
		t.Fatalf("plan.json when #71's status fails:\n%s", raw)
	}
}

// Criterion 4: the start of #64 sends its request and the host gives no
// answer before the timeout, and a codex task for #64 exists with no PR. The
// record marks #64 maybe started; recover says so and starts nothing; the
// next wake checks the host and GitHub before it starts #64 again.
func TestMissionMaybeStartedCheckedBeforeRestart(t *testing.T) {
	w := newCodexWorld(t, 889, 64)
	w.submit = func(brief string) error {
		if strings.Contains(brief, "# Delegated story #64:") {
			return fmt.Errorf("codex: %w", context.DeadlineExceeded)
		}
		return nil
	}
	if code, out, errOut := run(t, w.agent, w.opts, "mission", "next", "889", "--repo", "o/r", "--host", "codex"); code != 0 || out != "start #64\n" || w.claims(889)[64] != 1 {
		t.Fatalf("mission next: exit %d: %s%s", code, out, errOut)
	}
	code, out, errOut := run(t, w.agent, w.opts, "build", "64", "--delegate", "codex", "--repo", "o/r", "--asked", "aayu mission next marked it start")
	if code == 0 || !strings.Contains(errOut, "Codex gave no answer before the timeout, so #64 may have started there") ||
		!strings.Contains(errOut, "aayu mission recover 889 checks Codex and GitHub for #64 before it starts it again") {
		t.Fatalf("the start of #64: exit %d: %s%s", code, out, errOut)
	}
	if st := w.state(64); st.Status != dlgMaybe || !slices.Contains(w.g.LabelNames(64), "status:in-progress") {
		t.Fatalf("#64 does not keep its slot: %+v, labels %v", st, w.g.LabelNames(64))
	}
	if n := w.claims(889)[64]; n != 1 { // it holds its slot; the hand-off spends the claim for recover, which can restart it at once
		t.Fatalf("the list's claim of #64 is not live (%d)", n)
	}
	if p, raw := w.plan(889); p.stateOf(64) == nil || p.stateOf(64).State != "maybe started" {
		t.Fatalf("plan.json does not mark #64 maybe started:\n%s", raw)
	}
	w.submit = nil
	maybe := w.state(64)
	tries := w.handoffs(64)
	// A sync has no task to read: it writes nothing, so it never overwrites what recover records.
	commits := w.commits()
	if code, _, errOut := run(t, w.agent, w.opts, "build", "64", "--delegate", "codex", "--repo", "o/r", "--asked", "sync"); code != ExitBlocked ||
		!strings.Contains(errOut, "aayu knows no task of it to sync") || !strings.Contains(errOut, "aayu mission recover 889") || w.commits() != commits || w.state(64).Status != dlgMaybe {
		t.Fatalf("a sync of a maybe start: exit %d, commits %s → %s: %s", code, commits, w.commits(), errOut)
	}
	// A running task whose title names no story could be #64's: recover and retry start nothing.
	w.list = `{"tasks": [{"id": "task_e_64", "title": "# Delegated story #64: Item 64", "status": "running"}, {"id": "task_e_x", "title": "Fix the cart total", "status": "running"}], "cursor": null}`
	if code, out, _ := w.recover(889, "codex"); code != 0 || !strings.Contains(out, "#64  unreadable: codex task list (task task_e_x runs, and its title names no story") || w.handoffs(64) != tries {
		t.Fatalf("recover with an unclear task: hand-offs %d: %s", w.handoffs(64), out)
	}
	if code, _, errOut := w.mission(w.person, "rahulnakmol", "retry", 889, "--story", "64"); code != ExitExternal || !strings.Contains(errOut, "its task list cannot be read") || w.handoffs(64) != tries {
		t.Fatalf("retry with an unclear task: exit %d, hand-offs %d: %s", code, w.handoffs(64), errOut)
	}
	// Two tasks for #64: recover picks none, and holds the story.
	w.list = `{"tasks": [{"id": "task_e_64", "title": "# Delegated story #64: Item 64", "status": "running"}, {"id": "task_e_64z", "title": "# Delegated story #64: Item 64", "status": "completed"}], "cursor": null}`
	code, out, errOut = w.recover(889, "codex")
	if code != 0 || !strings.Contains(out, "#64  maybe started: 2 codex tasks exist for #64 (task_e_64, task_e_64z) and no PR: a person picks one, and aayu does not start it again\n") || w.handoffs(64) != tries || w.state(64).Task != "" {
		t.Fatalf("recover with two tasks: exit %d, hand-offs %d → %d: %s%s", code, tries, w.handoffs(64), out, errOut)
	}
	// mission next holds it too: it waits, and holds its slot.
	if code, out, errOut := run(t, w.agent, w.opts, "mission", "next", "889", "--repo", "o/r", "--host", "codex"); code != 0 || !strings.Contains(out, "waits #64: maybe started") {
		t.Fatalf("mission next: exit %d: %s%s", code, out, errOut)
	}
	// A person's retry finds the tasks on the host too: it starts nothing.
	if code, _, errOut := w.mission(w.person, "rahulnakmol", "retry", 889, "--story", "64"); code != ExitBlocked ||
		!strings.Contains(errOut, "a codex task for #64 exists: #64 may run there, so the retry starts nothing.") || w.handoffs(64) != tries {
		t.Fatalf("retry of #64 with its task on the host: exit %d, hand-offs %d → %d: %s", code, tries, w.handoffs(64), errOut)
	}
	// Bug #1148, criterion 2: one running task for #64. Recover records its ID and URL, and starts nothing.
	w.list = `{"tasks": [{"id": "task_e_64", "url": "https://chatgpt.com/codex/tasks/task_e_64", "title": "# Delegated story #64: Item 64", "status": "running"}], "cursor": null}`
	code, out, errOut = w.recover(889, "codex")
	if code != 0 || !strings.Contains(out, "#64  re-attached: its Codex task https://chatgpt.com/codex/tasks/task_e_64 (running) holds it, and it has no PR yet; the next sync reads it\n") ||
		!strings.Contains(out, "next: aayu build 64 --delegate codex") || w.handoffs(64) != tries {
		t.Fatalf("recover with its running task: exit %d, hand-offs %d → %d: %s%s", code, tries, w.handoffs(64), out, errOut)
	}
	if st := w.state(64); st.Task != "task_e_64" || st.TaskURL != "https://chatgpt.com/codex/tasks/task_e_64" || st.Status != dlgHandedOff || st.Branch != maybe.Branch {
		t.Fatalf("#64's delegation after the re-attach: %+v", st)
	}
	if p, raw := w.plan(889); p.stateOf(64) != nil && p.stateOf(64).State != "" {
		t.Fatalf("#64 is still marked after its re-attach:\n%s", raw)
	}
	// The next sync reads that task by its ID, and starts no second one.
	calls := len(w.callLog())
	if code, out, errOut := run(t, w.agent, w.opts, "build", "64", "--delegate", "codex", "--repo", "o/r", "--asked", "sync"); code != 0 || w.handoffs(64) != tries ||
		!slices.Contains(w.callLog()[calls:], "codex cloud status task_e_64") {
		t.Fatalf("the sync after the re-attach: exit %d, hand-offs %d, calls %q: %s%s", code, w.handoffs(64), w.callLog()[calls:], out, errOut)
	}
	// The next wake finds it running: no write, and nothing starts; a retry has no failed step.
	commits = w.commits()
	if code, out, _ := w.recover(889, "codex"); code != 0 || !strings.Contains(out, "#64  re-attached: its Codex thread lives") || w.commits() != commits || w.handoffs(64) != tries {
		t.Fatalf("the wake after the re-attach: commits %s → %s: %s", commits, w.commits(), out)
	}
	if code, _, errOut := w.mission(w.person, "rahulnakmol", "retry", 889, "--story", "64"); code != ExitBlocked || !strings.Contains(errOut, "nothing to retry") || w.handoffs(64) != tries {
		t.Fatalf("retry after the re-attach: exit %d: %s", code, errOut)
	}
	w.delegated(maybe)
	w.setPlan(889, func(p *MissionPlan) { p.storyState(64).State = "maybe started" })
	// The next wake: no task for #64 that can still run on the host, and no PR on GitHub, so it starts again, once.
	w.list = `{"tasks": [{"id": "task_e_old", "title": "# Delegated story #64: Item 64", "status": "error"}], "cursor": null}`
	code, out, errOut = w.recover(889, "codex")
	if code != 0 || !strings.Contains(out, "#64  restarted from story/64-") || !strings.Contains(out, "no codex task and no PR for #64") || w.handoffs(64) != tries+1 {
		t.Fatalf("the next wake: exit %d, hand-offs %d → %d: %s%s", code, tries, w.handoffs(64), out, errOut)
	}
	if p, raw := w.plan(889); p.stateOf(64) != nil && p.stateOf(64).State != "" {
		t.Fatalf("#64 is still marked after its restart:\n%s", raw)
	}
	// A restart that gets no answer either is maybe started again, and the list that cannot be read stops the next check.
	w.delegated(delegate.State{Story: 64, Receiver: "codex", Mode: "cloud", Task: "task_e_64b", Branch: "story/64-item-64", Status: dlgMaybe,
		Events: []delegate.Event{{At: time.Now().UTC().Format(time.RFC3339), Status: dlgMaybe, Detail: "no answer before the timeout"}}})
	w.list = `{"tasks": [], "cursor": null}`
	w.submit = func(string) error { return fmt.Errorf("codex: %w", context.DeadlineExceeded) }
	code, out, errOut = w.recover(889, "codex")
	if code != 0 || !strings.Contains(out, "#64  no codex task and no PR for #64; restarted, and Codex gave no answer before the timeout: maybe started, and aayu does not start it again") {
		t.Fatalf("a restart with no answer: exit %d: %s%s", code, out, errOut)
	}
	if p, raw := w.plan(889); p.stateOf(64) == nil || p.stateOf(64).State != "maybe started" {
		t.Fatalf("plan.json after a restart with no answer:\n%s", raw)
	}
	w.submit, w.listErr = nil, errors.New("exit status 1: network is unreachable")
	if code, out, _ := w.recover(889, "codex"); code != 0 || !strings.Contains(out, "#64  unreadable: codex task list (codex: exit status 1: network is unreachable): aayu starts nothing for it") {
		t.Fatalf("an unreadable list for a maybe start: %s", out)
	}
	if p, raw := w.plan(889); len(p.Sources) != 1 || p.Sources[0].Unreadable == "" || p.Sources[0].Since == "" {
		t.Fatalf("plan.json after an unreadable list:\n%s", raw)
	}
	w.listErr = nil
	// A story outside a started mission has no recover: a start with no answer fails as before, and is Ready again.
	w.featureWith(875, 66)
	if code, out, errOut := w.approve(875); code != 0 {
		t.Fatalf("approve 875: exit %d: %s%s", code, out, errOut)
	}
	w.readyStory(66)
	w.submit = func(string) error { return fmt.Errorf("codex: %w", context.DeadlineExceeded) }
	if code, _, errOut := run(t, w.agent, w.opts, "build", "66", "--delegate", "codex", "--repo", "o/r", "--asked", "a person asked"); code == 0 || strings.Contains(errOut, "maybe") ||
		w.state(66).Status != dlgFailed || !slices.Contains(w.g.LabelNames(66), "status:ready") {
		t.Fatalf("#66 outside a mission: exit %d, %+v, labels %v: %s", code, w.state(66), w.g.LabelNames(66), errOut)
	}
	w.submit = nil
	// A PR opens while a person enters the code of a retry: the retry starts nothing.
	w.delegated(delegate.State{Story: 64, Receiver: "codex", Mode: "cloud", Task: "task_e_64", Status: dlgMaybe})
	w.device("rahulnakmol")
	prc := make(chan *fake.PR, 1)
	w.edit(func() {
		w.g.Device.Pending = 1
		w.g.Device.OnPoll = func(n int) {
			if n == 1 { // the fake is locked here: the PR opens once this poll is answered
				go func() {
					prc <- w.g.AddPR("feat: item 64", "Refs #64", "rahulnakmol", "codex/item-64", strings.Repeat("b", 40), "dev")
				}()
			}
		}
	})
	code, out, errOut = run(t, w.person, w.opts, "mission", "retry", "889", "--story", "64", "--repo", "o/r")
	pr := <-prc
	if code != ExitBlocked || !strings.Contains(errOut, fmt.Sprintf("#64's PR changed while the code waited (now PR #%d): run the retry again.", pr.Number)) || w.handoffs(64) != tries+2 {
		t.Fatalf("retry of #64 whose PR opened meanwhile: exit %d, hand-offs %d: %s%s", code, w.handoffs(64), out, errOut)
	}
	// A PR for the story counts as GitHub's answer: recover keeps it, and starts nothing.
	if code, out, _ := w.recover(889, "codex"); code != 0 || !strings.Contains(out, fmt.Sprintf("#64  kept on its PR #%d", pr.Number)) || w.handoffs(64) != tries+2 {
		t.Fatalf("recover with #64's PR: %s", out)
	}
}

// A start with no answer whose maybe-started record the ledger refuses fails
// as a start: a record only in memory would hold the story with nothing to check it.
func TestMissionMaybeStartNotSavedFailsTheStart(t *testing.T) {
	w := newCodexWorld(t, 889, 67)
	w.submit = func(string) error { return fmt.Errorf("codex: %w", context.DeadlineExceeded) }
	if code, out, errOut := run(t, w.agent, w.opts, "mission", "next", "889", "--repo", "o/r", "--host", "codex"); code != 0 || out != "start #67\n" {
		t.Fatalf("mission next: exit %d: %s%s", code, out, errOut)
	}
	refuseLedger(t, w.bare, `git show "$new:delegate/67/state.json" 2>/dev/null | grep -q '"status": "maybe-started"'`)
	code, out, errOut := run(t, w.agent, w.opts, "build", "67", "--delegate", "codex", "--repo", "o/r", "--asked", "aayu mission next marked it start")
	refuseLedger(t, w.bare, "")
	p, raw := w.plan(889)
	if code == 0 || !strings.Contains(errOut, "the ledger refuses this push") || !slices.Contains(w.g.LabelNames(67), "status:ready") || w.claims(889)[67] != 0 ||
		w.state(67).Status == dlgMaybe || p.stateOf(67) != nil {
		t.Fatalf("a maybe start the ledger refused: exit %d, labels %v, claims %v, plan:\n%s\n%s%s", code, w.g.LabelNames(67), w.claims(889), raw, out, errOut)
	}
}

// A person cancels the start of #68 after Codex may have taken it: the start
// is maybe started, saved on the ledger though the command's context is
// cancelled, so recover checks the host before it starts #68 again.
func TestMissionCancelledStartIsMaybeStarted(t *testing.T) {
	w := newCodexWorld(t, 889, 68)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w.submit = func(string) error { cancel(); return fmt.Errorf("codex: %w", context.Canceled) }
	if code, out, errOut := run(t, w.agent, w.opts, "mission", "next", "889", "--repo", "o/r", "--host", "codex"); code != 0 || out != "start #68\n" {
		t.Fatalf("mission next: exit %d: %s%s", code, out, errOut)
	}
	code, out, errOut := runCtx(t, ctx, w.agent, w.opts, "", "build", "68", "--delegate", "codex", "--repo", "o/r", "--asked", "aayu mission next marked it start")
	raw, gerr := exec.Command("git", "-C", w.bare, "show", "ledger:"+delegate.LedgerPath(68)).Output()
	var st delegate.State
	_ = json.Unmarshal(raw, &st)
	if code == 0 || gerr != nil || st.Status != dlgMaybe || !strings.Contains(errOut, "aayu mission recover 889 checks Codex and GitHub for #68") {
		t.Fatalf("a cancelled start: exit %d, saved %v %+v: %s%s", code, gerr, st, out, errOut)
	}
	if p, raw := w.plan(889); p.stateOf(68) == nil || p.stateOf(68).State != "maybe started" {
		t.Fatalf("plan.json does not mark #68 maybe started:\n%s", raw)
	}
}

// A person cancels the start of #69 just after Codex took it: the hand-off's
// record lands anyway, so the next build syncs #69 and does not hand it off again.
func TestMissionCancelAfterHandoffKeepsRecord(t *testing.T) {
	w := newCodexWorld(t, 889, 69)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w.submit = func(string) error { cancel(); return nil } // Codex answers, then the person's Ctrl-C lands
	if code, out, errOut := run(t, w.agent, w.opts, "mission", "next", "889", "--repo", "o/r", "--host", "codex"); code != 0 || out != "start #69\n" {
		t.Fatalf("mission next: exit %d: %s%s", code, out, errOut)
	}
	code, out, errOut := runCtx(t, ctx, w.agent, w.opts, "", "build", "69", "--delegate", "codex", "--repo", "o/r", "--asked", "aayu mission next marked it start")
	raw, gerr := exec.Command("git", "-C", w.bare, "show", "ledger:"+delegate.LedgerPath(69)).Output()
	var st delegate.State
	_ = json.Unmarshal(raw, &st)
	if gerr != nil || st.Status != dlgHandedOff || st.Task == "" || w.handoffs(69) != 1 {
		t.Fatalf("a hand-off cancelled after Codex took it: exit %d, saved %v %+v, hand-offs %d: %s%s", code, gerr, st, w.handoffs(69), out, errOut)
	}
}

// failingAmp fails each Amp hand-off of story n with a host error while fail is set.
type failingAmp struct {
	mu   sync.Mutex
	fail map[int]bool
}

func (f *failingAmp) set(n int, on bool) { f.mu.Lock(); f.fail[n] = on; f.mu.Unlock() }

func withFailingAmp(m *missionWorld) *failingAmp {
	f := &failingAmp{fail: map[int]bool{}}
	m.opts.DelegateExec = func(ctx context.Context, dir string, env []string, stdin, name string, args ...string) (string, error) {
		if name == "amp" && len(args) > 1 && args[0] == "-ox" {
			f.mu.Lock()
			defer f.mu.Unlock()
			for n, on := range f.fail {
				if on && strings.Contains(args[1], fmt.Sprintf("# Delegated story #%d:", n)) {
					m.mu.Lock()
					m.calls, m.briefs = append(m.calls, "amp -ox <brief> (failed)"), append(m.briefs, args[1])
					m.mu.Unlock()
					return "", errors.New("exit status 1: Error: 503 Service Unavailable")
				}
			}
		}
		return m.exec(ctx, dir, env, stdin, name, args...)
	}
	return f
}

// Criterion 5: the start of #63 fails with a host error. Recover retries it
// after retry_backoff_minutes (10), then after 20; after retry_max (2)
// retries it waits on @rahulnakmol with the errors listed, and starts nothing more.
func TestMissionRetryBackoffAndMax(t *testing.T) {
	m := newMissionWorld(t)
	m.ampConnected()
	clk := m.withClock()
	fail := withFailingAmp(m)
	m.startedMission(889, []int{63})
	m.readyStory(63)
	fail.set(63, true)
	t0 := clk.now()
	if code, _, errOut := run(t, m.agent, m.opts, "build", "63", "--delegate", "amp", "--repo", "o/r", "--asked", "aayu mission next marked it start"); code == 0 || !strings.Contains(errOut, "503") {
		t.Fatalf("the start of #63: exit %d: %s", code, errOut)
	}
	at := func(d time.Duration) string { return t0.Add(d).Truncate(time.Second).Local().Format("15:04") }

	clk.add(5 * time.Minute)
	code, out, errOut := m.recover(889, "")
	if code != 0 || !strings.Contains(out, "#63  failed (hand-off failed: amp: exit status 1: Error: 503 Service Unavailable); retry 1 of 2 at "+at(10*time.Minute)) || m.handoffs(63) != 1 {
		t.Fatalf("recover at +5: exit %d, hand-offs %d: %s%s", code, m.handoffs(63), out, errOut)
	}
	// The story waits for its retry: mission next does not start it before then.
	if code, out, errOut := run(t, m.agent, m.opts, "mission", "next", "889", "--repo", "o/r"); code != 0 || !strings.HasPrefix(out, "waits #63: failed (1 attempts, ") {
		t.Fatalf("mission next before the retry: exit %d: %s%s", code, out, errOut)
	}
	// In a quiet mission no event reaches the Kanha, and only recover runs a retry: the tick wakes it once the retry is due.
	if code, r, errOut := m.tick(clk.now()); code != 0 || len(r.Woken) != 0 {
		t.Fatalf("tick before the retry is due: exit %d, woken %+v: %s", code, r.Woken, errOut)
	}
	clk.add(5 * time.Minute) // +10: the first retry
	wake := "tick: the retry of #63 is due, and no event reached the Kanha since: run aayu mission recover 889, then aayu mission next 889"
	if code, r, errOut := m.tick(clk.now()); code != 0 || len(r.Woken) != 1 || r.Woken[0].Event != wake || r.Woken[0].Status != wakeSent {
		t.Fatalf("tick when the retry is due: exit %d, woken %+v, held %+v: %s", code, r.Woken, r.Held, errOut)
	}
	if code, r, errOut := m.tick(clk.now().Add(time.Minute)); code != 0 || len(r.Woken) != 0 {
		t.Fatalf("a second tick woke the Kanha for the same retry: exit %d, woken %+v: %s", code, r.Woken, errOut)
	}
	code, out, errOut = m.recover(889, "")
	if code != 0 || !strings.Contains(out, "#63  retried (1 of 2): ") || !strings.Contains(out, "retry 2 of 2 at "+at(30*time.Minute)) || m.handoffs(63) != 2 {
		t.Fatalf("recover at +10: exit %d, hand-offs %d: %s%s", code, m.handoffs(63), out, errOut)
	}
	clk.add(19 * time.Minute) // +29: the second retry is not due yet
	if code, out, _ := m.recover(889, ""); code != 0 || !strings.Contains(out, "retry 2 of 2 at "+at(30*time.Minute)) || m.handoffs(63) != 2 {
		t.Fatalf("recover at +29: hand-offs %d: %s", m.handoffs(63), out)
	}
	clk.add(time.Minute) // +30: the second retry, after 20 minutes
	code, out, errOut = m.recover(889, "")
	if code != 0 || !strings.Contains(out, "#63  retried (2 of 2): ") || !strings.Contains(out, "waits on @rahulnakmol: 3 attempts failed: 1. hand-off failed: amp: exit status 1: Error: 503 Service Unavailable (start); 2. ") ||
		!strings.Contains(out, "(retry); 3. ") || !strings.Contains(out, "next: aayu mission retry 889 --story 63") || m.handoffs(63) != 3 {
		t.Fatalf("recover at +30: exit %d, hand-offs %d: %s%s", code, m.handoffs(63), out, errOut)
	}
	p, raw := m.plan(889)
	ss := p.stateOf(63)
	if ss == nil || ss.State != "waits on @rahulnakmol" || ss.RetryAt != "" || len(ss.Attempts) != 3 || ss.retries() != 2 {
		t.Fatalf("plan.json after the last retry:\n%s", raw)
	}
	// It waits: later wakes start nothing more, and write nothing.
	clk.add(2 * time.Hour)
	commits := m.commits()
	if code, out, _ := m.recover(889, ""); code != 0 || !strings.Contains(out, "#63  waits on @rahulnakmol: 3 attempts failed") || m.handoffs(63) != 3 || m.commits() != commits {
		t.Fatalf("recover after the wait: hand-offs %d, commits %s → %s: %s", m.handoffs(63), commits, m.commits(), out)
	}
}

// A failed delegation with no failed event: the first recover sets its retry
// time, a later one keeps it and writes nothing, and the retry runs when due.
func TestMissionRetryWithNoFailedEvent(t *testing.T) {
	m := newMissionWorld(t)
	m.ampConnected()
	clk := m.withClock()
	m.startedMission(889, []int{66})
	m.delegated(delegate.State{Story: 66, Receiver: "amp", Mode: "cloud", Task: "T-01a00000-0000-0000-0000-000000000066", Branch: "story/66-item-66", Status: dlgFailed})
	at := clk.now().Add(10 * time.Minute).Truncate(time.Second).Local().Format("15:04")
	if code, out, errOut := m.recover(889, ""); code != 0 || !strings.Contains(out, "retry 1 of 2 at "+at) {
		t.Fatalf("first recover: exit %d: %s%s", code, out, errOut)
	}
	clk.add(5 * time.Minute)
	commits := m.commits()
	if code, out, _ := m.recover(889, ""); code != 0 || !strings.Contains(out, "retry 1 of 2 at "+at) || m.commits() != commits {
		t.Fatalf("recover at +5 moved the retry or wrote: commits %s → %s: %s", commits, m.commits(), out)
	}
	clk.add(5 * time.Minute)
	if code, out, errOut := m.recover(889, ""); code != 0 || !strings.Contains(out, "#66  retried (1 of 2)") {
		t.Fatalf("recover at +10: exit %d: %s%s", code, out, errOut)
	}
}

// Criterion 6: after that, a person's `aayu mission retry 889 --story 63`,
// and one for a failed Bug #941: one more attempt each, counted and costed
// in the record, and the retry is a control in the plan.
func TestMissionRetryStory(t *testing.T) {
	m := newMissionWorld(t)
	m.ampConnected()
	m.withClock()
	fail := withFailingAmp(m)
	m.startedMission(889, []int{63, 941}, 941)
	m.readyStory(63)
	m.readyStory(941)
	fail.set(63, true)
	if code, _, _ := run(t, m.agent, m.opts, "build", "63", "--delegate", "amp", "--repo", "o/r", "--asked", "aayu mission next marked it start"); code == 0 {
		t.Fatal("the start of #63 did not fail")
	}
	at, _ := lastFailure(m.state(63))     // the last of three failed attempts
	m.setPlan(889, func(p *MissionPlan) { // two retries failed: #63 waits on its starter
		ss := p.storyState(63)
		ss.State = "waits on @rahulnakmol"
		ss.Attempts = []MissionAttempt{{Kind: "start", At: "2026-10-26T09:00:00Z", Error: "e1", CostUSD: 1}, {Kind: "retry", At: "2026-10-26T09:10:00Z", Error: "e2", CostUSD: 1}, {Kind: "retry", At: at, Error: "e3", CostUSD: 1}}
	})
	var appr ready.Approval
	_ = json.Unmarshal([]byte(git(t, m.bare, "show", "ledger:"+ready.ApprovalPath(889))), &appr)
	per := appr.Budget.PerStoryUSD
	m.delegated(delegate.State{Story: 941, Receiver: "amp", Mode: "cloud", Task: "T-01a00000-0000-0000-0000-000000000941", Branch: "story/941-item-941", Status: dlgFailed,
		Events: []delegate.Event{{At: "2026-10-26T09:30:00Z", Status: dlgFailed, Detail: "the receiver's task failed: run error"}}})
	// A retry while the host still fails: one more attempt, counted, and it waits again.
	code, out, errOut := m.mission(m.person, "rahulnakmol", "retry", 889, "--story", "63")
	if code != 0 || !strings.Contains(out, "#63  retried by @rahulnakmol: failed again: ") || !strings.Contains(out, "; waits on @rahulnakmol · 4 attempts, ") || m.handoffs(63) != 2 {
		t.Fatalf("retry #63 while the host fails: exit %d, hand-offs %d: %s%s", code, m.handoffs(63), out, errOut)
	}
	fail.set(63, false)

	code, out, errOut = m.mission(m.person, "rahulnakmol", "retry", 889, "--story", "63")
	if code != 0 || m.codes() != 1 || !strings.Contains(out, "@rahulnakmol retried #63 in the mission of #889 on amp.") ||
		!strings.Contains(out, "#63  retried by @rahulnakmol: restarted from story/63-") || !strings.Contains(out, fmt.Sprintf(" · 5 attempts, %s", ready.USD(3+2*per))) || m.handoffs(63) != 3 {
		t.Fatalf("retry #63: exit %d, hand-offs %d: %s%s", code, m.handoffs(63), out, errOut)
	}
	code, out, errOut = m.mission(m.person, "rahulnakmol", "retry", 889, "--story", "941")
	if code != 0 || !strings.Contains(out, "#941  retried by @rahulnakmol: restarted from story/941-item-941 and its brief") ||
		!strings.Contains(out, fmt.Sprintf(" · 2 attempts, %s", ready.USD(2*per))) || m.handoffs(941) != 1 {
		t.Fatalf("retry Bug #941: exit %d, hand-offs %d: %s%s", code, m.handoffs(941), out, errOut)
	}
	p, raw := m.plan(889)
	s63, s941 := p.stateOf(63), p.stateOf(941)
	if s63 == nil || len(s63.Attempts) != 5 || s63.State != "" || s63.Attempts[4].Kind != "retry" || s63.Attempts[4].By != "rahulnakmol" || s63.Attempts[4].CostUSD != per || s63.Attempts[3].Error == "" ||
		s941 == nil || len(s941.Attempts) != 2 || s941.Attempts[0].Kind != "run" || s941.Attempts[1].By != "rahulnakmol" || s941.cost() != 2*per || s941.CostBasis == "" {
		t.Fatalf("plan.json after the retries:\n%s", raw)
	}
	var retries []MissionControl
	for _, c := range p.Controls {
		if c.Action == "retry" {
			retries = append(retries, c)
		}
	}
	if len(retries) != 3 || retries[0].Story != 63 || retries[2].Story != 941 || retries[2].By != "rahulnakmol" || retries[2].Presence != "device-flow" {
		t.Fatalf("controls: %+v", p.Controls)
	}
	// An agent lane's retry is refused: a retry is a person's decision.
	if code, _, errOut := m.mission(m.agent, "rahulnakmol", "retry", 889, "--story", "63"); code != ExitBlocked || !strings.Contains(errOut, "mission retry is a person's decision") {
		t.Fatalf("agent retry: exit %d: %s", code, errOut)
	}
}

// Criterion 7: a retry of a story whose PR has a judged inspect review on
// its head sends that review's findings and does not run the review again.
func TestMissionRetryDoesNotRerunJudgedReview(t *testing.T) {
	m := newMissionWorld(t)
	m.ampConnected()
	m.startedMission(889, []int{65})
	m.readyStory(65)
	head := strings.Repeat("e", 40)
	pr := m.g.AddPR("feat: item 65", "Refs #65", "rahulnakmol", "story/65-item-65", head, "dev")
	res := inspect.Result{PR: pr.Number, Head: head, Disposition: inspect.Fix, Findings: []inspect.Finding{{ID: "F-65", Severity: "P2", Title: "an empty list divides by zero", Fix: "return 0"}}, Criteria: []inspect.Verdict{}}
	j, _ := json.Marshal(inspect.State{Cycles: 1, Result: &res})
	m.edit(func() {
		c := &fake.Comment{ID: 6500, Body: inspect.Marker + "\n<!-- aayudha:inspect-state " + string(j) + " -->"}
		c.User.Login, c.User.Type = "github-actions[bot]", "Bot"
		m.g.Issues[pr.Number].Comments = append(m.g.Issues[pr.Number].Comments, c)
	})
	m.delegated(delegate.State{Story: 65, Receiver: "amp", Mode: "cloud", Task: "T-01a00000-0000-0000-0000-000000000065", Branch: "story/65-item-65", PR: pr.Number, Status: dlgFailed,
		Events: []delegate.Event{{At: "2026-10-26T09:30:00Z", Status: dlgFailed, Detail: "build #65: follow-up to amp: amp: exit status 1"}}})
	m.g.Reset()

	code, out, errOut := m.mission(m.person, "rahulnakmol", "retry", 889, "--story", "65")
	judged := fmt.Sprintf("PR #%d's review at %s is judged (fix): the retry does not run it again", pr.Number, short(head))
	if code != 0 || !strings.Contains(errOut, judged) || !strings.Contains(out, fmt.Sprintf("#65  retried by @rahulnakmol: the sync of PR #%d ran again (fix-requested); %s", pr.Number, judged)) {
		t.Fatalf("retry #65: exit %d: %s%s", code, out, errOut)
	}
	st := m.state(65)
	if st.Status != dlgFix || len(st.FollowUps) != 1 || st.FollowUps[0].Head != head || !slices.Equal(st.FollowUps[0].Findings, []string{"F-65"}) || m.handoffs(65) != 0 {
		t.Fatalf("#65's delegation after the retry: %+v", st)
	}
	for _, r := range m.g.Writes() {
		if strings.Contains(r.Path, fmt.Sprintf("/issues/%d/comments", pr.Number)) || strings.Contains(r.Path, "/dispatches") || strings.Contains(r.Path, "/pulls/") {
			t.Fatalf("the retry ran a review: %s %s", r.Method, r.Path)
		}
	}
	if !slices.ContainsFunc(m.callLog(), func(c string) bool {
		return strings.HasPrefix(c, "amp threads continue T-01a00000-0000-0000-0000-000000000065")
	}) {
		t.Fatalf("the judged review's findings did not go to #65's thread: %q", m.callLog())
	}
}

// bobsCodex is mission #874 that @bob started on codex, with story #950's start failed.
func bobsCodex(t *testing.T) *missionWorld {
	m := newMissionWorld(t)
	m.startedMission(874, []int{950})
	m.setPlan(874, func(p *MissionPlan) {
		p.StartedBy, p.Tool = "bob", "codex"
		p.Controls[0].By, p.Controls[0].Tool = "bob", "codex"
	})
	m.readyStory(950)
	m.delegated(delegate.State{Story: 950, Receiver: "codex", Mode: "cloud", Branch: "story/950-item-950", Status: dlgFailed,
		Events: []delegate.Event{{At: "2026-10-26T09:30:00Z", Status: dlgFailed, Detail: "hand-off failed: codex: exit status 1: 503"}}})
	return m
}

// as makes login the account the token acts as, with tool for this repository.
func (m *missionWorld) as(login, tool string) {
	m.edit(func() { m.g.Viewer = login })
	m.setTool(tool)
}

// Criterion 8: @carol, on cursor, runs recover or retry on #874, whose
// recorded tool is codex: each is refused with the tool and its starter, and
// the record does not change.
func TestMissionRecoverRefusedOnOtherTool(t *testing.T) {
	m := bobsCodex(t)
	m.as("carol", "cursor")
	commits, raw := m.commits(), git(t, m.bare, "show", "ledger:"+MissionPlanPath(874))
	m.g.Reset()
	want := "#874's mission runs on codex, the tool of @bob, who started it: run this there"
	for _, args := range [][]string{{"recover"}, {"retry", "--story", "950"}} {
		code, out, errOut := m.mission(m.person, "carol", args[0], 874, args[1:]...)
		if code != ExitBlocked || !strings.Contains(errOut, want) || m.codes() != 0 {
			t.Fatalf("carol's %s: exit %d, device codes %d: %s%s", args[0], code, m.codes(), out, errOut)
		}
	}
	if m.commits() != commits || git(t, m.bare, "show", "ledger:"+MissionPlanPath(874)) != raw || len(m.repoWrites()) != 0 || m.handoffs(950) != 0 {
		t.Fatalf("the refusals changed the record: commits %s → %s", commits, m.commits())
	}
	// The Kanha on another tool is refused the same way.
	if code, _, errOut := m.recover(874, "cursor"); code != ExitBlocked || !strings.Contains(errOut, want) {
		t.Fatalf("recover on cursor: exit %d: %s", code, errOut)
	}
}

// Criterion 9: @dana, on codex, retries #950 of @bob's mission #874: the
// retry runs, and controls names retry, @dana, codex, and the time.
func TestMissionRetryByTeammateRecorded(t *testing.T) {
	m := bobsCodex(t)
	m.as("dana", "codex")
	// A recover marks the codex task list unreadable while dana enters her code: the retry keeps that mark.
	m.device("dana")
	m.edit(func() {
		m.g.Device.OnPoll = func(int) {
			m.setPlan(874, func(p *MissionPlan) {
				p.Sources = []MissionSource{{Name: "codex task list", Unreadable: "401", Since: "2026-10-26T09:40:00Z"}}
			})
		}
	})
	code, out, errOut := run(t, m.person, m.opts, "mission", "retry", "874", "--story", "950", "--repo", "o/r")
	if code != 0 || m.codes() != 1 || !strings.Contains(out, "#950  retried by @dana: restarted from story/950-item-950 and its brief") || m.handoffs(950) != 1 {
		t.Fatalf("dana's retry: exit %d, hand-offs %d: %s%s", code, m.handoffs(950), out, errOut)
	}
	p, raw := m.plan(874)
	last := p.Controls[len(p.Controls)-1]
	at, err := time.Parse(time.RFC3339, last.At)
	if p.StartedBy != "bob" || err != nil || time.Since(at) > time.Minute ||
		last != (MissionControl{Action: "retry", By: "dana", Tool: "codex", At: last.At, Presence: "device-flow", Story: 950}) ||
		!strings.Contains(raw, `"control": "retry",`+"\n"+`      "by": "dana",`+"\n"+`      "tool": "codex",`+"\n"+`      "at": "`) {
		t.Fatalf("plan.json after dana's retry:\n%s", raw)
	}
	if ss := p.stateOf(950); ss == nil || len(ss.Attempts) != 2 || ss.Attempts[1].By != "dana" {
		t.Fatalf("#950's attempts:\n%s", raw)
	}
	if len(p.Sources) != 1 || p.Sources[0].Unreadable != "401" {
		t.Fatalf("the retry dropped the source a recover marked meanwhile:\n%s", raw)
	}
}

// Recover twice in a row, with stories in every state, changes nothing the
// second time (criterion 2), and recover is no person's decision: the Kanha runs it.
func TestMissionRecoverIsIdempotent(t *testing.T) {
	w := newCodexWorld(t, 889, 80, 81, 82, 83)
	if code, out, _ := w.recover(889, "codex"); code != 0 || !strings.Contains(out, "  no story runs: mission next starts what can start\n  nothing changed\n") {
		t.Fatalf("recover with no story running: %s", out)
	}
	w.delegated(delegate.State{Story: 83, Receiver: "codex", Mode: "cloud", Task: "task_e_83", TaskURL: "https://chatgpt.com/codex/tasks/task_e_83", Status: dlgWorking}) // older than the list: its status says
	w.delegated(delegate.State{Story: 80, Receiver: "codex", Mode: "cloud", Task: "task_e_80", Status: dlgWorking})
	w.delegated(delegate.State{Story: 81, Receiver: "codex", Mode: "cloud", Task: "task_e_81", Status: dlgMaybe})
	w.delegated(delegate.State{Story: 82, Receiver: "codex", Mode: "cloud", Task: "task_e_82", Branch: "story/82-item-82", Status: dlgWorking})
	w.list = `{"tasks": [{"id": "task_e_80", "title": "# Delegated story #80: Item 80", "status": "running"},
		{"id": "task_e_81", "title": "# Delegated story #81: Item 81", "status": "running"},
		{"id": "task_e_82", "title": "# Delegated story #82: Item 82", "status": "failed"}], "cursor": null}`
	code, out, errOut := w.recover(889, "codex")
	if code != 0 || len(storyLines(out)) != 4 || !strings.Contains(out, "#82  restarted from story/82-item-82") || w.handoffs(82) != 1 ||
		!strings.Contains(out, "#83  re-attached: its Codex thread lives (https://chatgpt.com/codex/tasks/task_e_83)") {
		t.Fatalf("first recover: exit %d: %s%s", code, out, errOut)
	}
	// #82's new task is in the list now; nothing else moved.
	w.list = strings.Replace(w.list, `{"id": "task_e_82", "title": "# Delegated story #82: Item 82", "status": "failed"}`,
		fmt.Sprintf(`{"id": %q, "title": "# Delegated story #82: Item 82", "status": "running"}`, w.state(82).Task), 1)
	commits := w.commits()
	code, out, errOut = w.recover(889, "codex")
	if code != 0 || !strings.Contains(out, "  nothing changed\n") || w.commits() != commits || w.handoffs(80)+w.handoffs(81)+w.handoffs(83) != 0 || w.handoffs(82) != 1 {
		t.Fatalf("second recover: exit %d, commits %s → %s: %s%s", code, commits, w.commits(), out, errOut)
	}
}

// Recover leaves what it must not start: a stopped story, a capped one, one
// whose PR closed, a local run, a Tatkal story on another tool, and a start
// with no answer on a host that lists no tasks. --dry-run writes nothing.
// Retry refuses a run with no story, a story with no failed step, a stopped
// story, another tool's story, and a paused mission.
func TestMissionRecoverLeavesAndRefuses(t *testing.T) {
	m := newMissionWorld(t)
	m.ampConnected()
	m.startedMission(889, []int{87, 88, 90, 91, 92, 93, 94, 95, 96, 97, 98, 99})
	amp := func(n int) delegate.State {
		id := fmt.Sprintf("T-01a00000-0000-0000-0000-0000000000%d", n)
		return delegate.State{Story: n, Receiver: "amp", Mode: "cloud", Task: id, TaskURL: "https://ampcode.com/threads/" + id, Branch: fmt.Sprintf("story/%d-item-%d", n, n), Status: dlgWorking}
	}
	m.readyStory(98)
	for _, n := range []int{88, 90, 98, 99} {
		m.delegated(amp(n))
	}
	capped, closed, local, maybe := amp(91), amp(92), amp(93), amp(95)
	capped.Status, closed.Status, closed.PR, local.Mode, maybe.Status = dlgCapped, dlgFailed, 777, "local", dlgMaybe
	for _, st := range []delegate.State{capped, closed, local, maybe, {Story: 94, Receiver: "claude", Mode: "cloud", Task: "session_94", Status: dlgWorking}, amp(97)} {
		m.delegated(st)
	}
	m.closeIssue(97)
	m.mu.Lock()
	m.archived[amp(98).Task] = true
	m.mu.Unlock()
	pr := m.g.AddPR("feat: item 99", "Refs #99", "rahulnakmol", "story/99-item-99", strings.Repeat("c", 40), "dev")
	// #87's sync failed on an open PR that does not name it: the delegation's PR is the one.
	pr87 := m.g.AddPR("fix: the empty list", "No story named here.", "rahulnakmol", "fix-empty-list", strings.Repeat("d", 40), "dev")
	failed87 := amp(87)
	failed87.PR, failed87.Status = pr87.Number, dlgFailed
	failed87.Events = []delegate.Event{{At: "2026-10-26T09:30:00Z", Status: dlgFailed, Detail: "build #87: follow-up to amp: amp: exit status 1"}}
	m.delegated(failed87)
	m.setPlan(889, func(p *MissionPlan) {
		p.Stopped = []MissionStopped{{Number: 90, Reason: "wrong approach", By: "rahulnakmol"}}
		p.StoryStarts = []MissionStoryStart{{Number: 94, Tool: "claude", StartedBy: "carol", StartedAt: "2026-10-26T10:00:00Z"}}
	})

	commits := m.commits()
	code, out, errOut := run(t, m.agent, m.opts, "mission", "recover", "889", "--repo", "o/r", "--dry-run")
	for _, want := range []string{
		"#88  re-attached: its Amp thread lives (https://ampcode.com/threads/T-01a00000-0000-0000-0000-000000000088); --dry-run: its Kanha is not updated",
		"#98  its Amp thread is archived with no PR: --dry-run: would restart it from story/98-item-98 and its brief",
		fmt.Sprintf("#99  kept on its PR #%d (--dry-run: the delegation is not updated)", pr.Number),
		"  --dry-run: ledger missions/889/plan.json planned, not written; nothing restarted",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("recover --dry-run lacks %q", want)
		}
	}
	if code != 0 || m.commits() != commits || m.handoffs(98) != 0 {
		t.Fatalf("recover --dry-run: exit %d, commits %s → %s: %s%s", code, commits, m.commits(), out, errOut)
	}

	code, out, errOut = m.recover(889, "")
	for _, want := range []string{
		"#90  stopped (wrong approach): recover leaves it until a person resumes it\n",
		"#91  capped: a person takes over, and recover starts nothing for it\n",
		"#92  its PR #777 is not open: recover starts nothing for it\n",
		"#93  runs Amp on its builder's machine: recover reads no host for it\n",
		"#94  runs on claude, the tool of @carol, who started it: recover it there\n",
		"#95  maybe started: Amp lists no tasks, so aayu cannot see whether #95 started there; a person checks Amp, and aayu does not start it again\n",
		"     next: aayu mission retry 889 --story 95\n",
		fmt.Sprintf("#87  kept on its PR #%d\n", pr87.Number),
	} {
		if !strings.Contains(out, want) {
			t.Errorf("recover lacks %q", want)
		}
	}
	if code != 0 || strings.Contains(out, "#96 ") || strings.Contains(out, "#97 ") || m.handoffs(98) != 1 {
		t.Fatalf("recover: exit %d, #98 hand-offs %d: %s%s", code, m.handoffs(98), out, errOut)
	}

	// #88's host could not be read after a retry that succeeded: its agent may run, and its attempts stay.
	m.setPlan(889, func(p *MissionPlan) {
		ss := p.storyState(88)
		ss.State, ss.Attempts = "unreadable: Amp threads (503)", []MissionAttempt{{Kind: "start", At: "2026-10-26T09:00:00Z", Error: "e1", CostUSD: 1}, {Kind: "retry", At: "2026-10-26T09:10:00Z", CostUSD: 1}}
	})
	commits = m.commits()
	for _, c := range []struct {
		args []string
		code int
		want string
	}{
		{nil, ExitUsage, "mission retry needs --story"},
		{[]string{"--story", "96"}, ExitBlocked, "#96 has no failed step in the mission of #889: nothing to retry. Nothing changed."},
		{[]string{"--story", "90"}, ExitBlocked, "#90 is stopped (wrong approach): resume it first. Nothing changed."},
		{[]string{"--story", "94"}, ExitBlocked, "#94 runs on claude, the tool of @carol, who started it: run this there"},
		{[]string{"--story", "200"}, ExitBlocked, "#200 is not a story or Bug of #889. Nothing changed."},
		{[]string{"--story", "88"}, ExitBlocked, "#88 has no failed step in the mission of #889: nothing to retry. Nothing changed."},
	} {
		if code, _, errOut := m.mission(m.person, "rahulnakmol", "retry", 889, c.args...); code != c.code || !strings.Contains(errOut, c.want) {
			t.Errorf("retry %v: exit %d: %s", c.args, code, errOut)
		}
	}
	for _, action := range [][]string{{"recover"}, {"retry", "--story", "87"}} {
		if code, _, errOut := m.mission(m.person, "rahulnakmol", action[0], 5, action[1:]...); code != ExitBlocked || !strings.Contains(errOut, "#5 has no mission plan: a person approves it to build first. Nothing changed.") {
			t.Errorf("%s with no mission plan: exit %d: %s", action[0], code, errOut)
		}
	}
	code, out, errOut = m.mission(m.person, "rahulnakmol", "retry", 889, "--story", "95", "--dry-run")
	if code != 0 || !strings.Contains(out, "  Retries #95 of the mission of #889 once more: it hands #95 off again on amp, from its branch and its brief") || m.commits() != commits {
		t.Fatalf("retry --dry-run: exit %d: %s%s", code, out, errOut)
	}
	code, out, errOut = m.mission(m.person, "rahulnakmol", "retry", 889, "--story", "87", "--dry-run")
	if code != 0 || !strings.Contains(out, fmt.Sprintf("it runs the sync of #87's PR #%d on amp again; the attempt is counted", pr87.Number)) || strings.Contains(out+errOut, "is judged") || m.commits() != commits {
		t.Fatalf("retry #87 --dry-run: exit %d: %s%s", code, out, errOut)
	}
	// The ledger refuses the retry's claim line: the retry fails, starts nothing, and records no attempt.
	refuseLedger(t, m.bare, claimsPush)
	code, out, errOut = m.mission(m.person, "rahulnakmol", "retry", 889, "--story", "95")
	refuseLedger(t, m.bare, "")
	if code != ExitExternal || m.handoffs(95) != 0 || m.commits() != commits {
		t.Fatalf("retry when the ledger refuses the claim: exit %d, hand-offs %d: %s%s", code, m.handoffs(95), out, errOut)
	}
	// A recover restarts #95 while the person enters the device code: the retry starts no second agent.
	m.device("rahulnakmol")
	restarted := amp(95)
	restarted.Task, restarted.Status = "T-01a00000-0000-0000-0000-000000000195", dlgHandedOff
	m.edit(func() {
		m.g.Device.OnPoll = func(int) {
			b, _ := json.Marshal(restarted)
			putLedger(t, m.bare, delegate.LedgerPath(95), string(b))
		}
	})
	code, out, errOut = run(t, m.person, m.opts, "mission", "retry", "889", "--story", "95", "--repo", "o/r")
	if code != ExitBlocked || !strings.Contains(errOut, "#95 changed while the code waited (now handed-off): another run acted on it, so the retry starts nothing.") || m.handoffs(95) != 0 {
		t.Fatalf("retry of #95 that a recover restarted meanwhile: exit %d, hand-offs %d: %s%s", code, m.handoffs(95), out, errOut)
	}
	commits = m.commits()
	// A GitHub read that fails stops the run before it changes anything.
	for _, c := range []struct {
		fail string
		args []string
	}{
		{"GET /user", []string{"recover"}},
		{"GET /repos/o/r/pulls", []string{"recover"}},
		{"GET /repos/o/r/issues/889/sub_issues", []string{"recover"}},
		{"GET /repos/o/r/pulls", []string{"retry", "--story", "87"}},
		{fmt.Sprintf("GET /repos/o/r/pulls/%d", pr87.Number), []string{"retry", "--story", "87"}},
		{fmt.Sprintf("GET /repos/o/r/issues/%d/comments", pr87.Number), []string{"retry", "--story", "87"}},
	} {
		m.edit(func() { m.g.Fail, m.g.FailAfter = c.fail, 0 })
		code, out, errOut := m.mission(m.person, "rahulnakmol", c.args[0], 889, c.args[1:]...)
		m.edit(func() { m.g.Fail = "" })
		if code != ExitExternal || !strings.Contains(errOut, "injected failure") || m.commits() != commits || m.handoffs(87) != 0 {
			t.Errorf("%s with %s failing: exit %d, commits %s → %s: %s%s", c.args[0], c.fail, code, commits, m.commits(), out, errOut)
		}
	}
	// A person stops #87 while the code of its retry waits: the retry runs nothing.
	m.device("rahulnakmol")
	m.edit(func() {
		m.g.Device.OnPoll = func(int) {
			m.setPlan(889, func(p *MissionPlan) {
				p.Stopped = append(p.Stopped, MissionStopped{Number: 87, Reason: "wrong fix", By: "rahulnakmol"})
			})
		}
	})
	code, out, errOut = run(t, m.person, m.opts, "mission", "retry", "889", "--story", "87", "--repo", "o/r")
	if p, _ := m.plan(889); code != ExitBlocked || !strings.Contains(errOut, "#87 was stopped while the code waited (wrong fix): resume it first. Nothing changed.") ||
		slices.ContainsFunc(p.Controls, func(c MissionControl) bool { return c.Action == "retry" && c.Story == 87 }) || p.stateOf(87) != nil {
		t.Fatalf("retry of #87 that a person stopped meanwhile: exit %d: %s%s", code, out, errOut)
	}
	commits = m.commits()
	// The [budget] breaker's pause stops recover before it reads anything.
	pause := (&Env{Options: Options{Dir: m.dir}}).PauseFile()
	if err := os.MkdirAll(filepath.Dir(pause), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pause, []byte("budget"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _, errOut := m.mission(m.person, "rahulnakmol", "recover", 889); code != ExitBudget || !strings.Contains(errOut, "agent lanes are paused by the [budget] breaker") || m.commits() != commits {
		t.Errorf("recover under the breaker: exit %d: %s", code, errOut)
	}
	if err := os.Remove(pause); err != nil {
		t.Fatal(err)
	}
	m.setPlan(889, func(p *MissionPlan) { p.Pause = &MissionPause{Reason: "spec drift", By: "rahulnakmol"} })
	if code, _, errOut := m.recover(889, ""); code != ExitBlocked || !strings.Contains(errOut, "The mission of #889 is paused: spec drift. Nothing starts.") {
		t.Fatalf("recover of a paused mission: exit %d: %s", code, errOut)
	}
	if code, _, errOut := m.mission(m.person, "rahulnakmol", "retry", 889, "--story", "95"); code != ExitBlocked || !strings.Contains(errOut, "The mission of #889 is paused (spec drift): resume it first.") {
		t.Fatalf("retry in a paused mission: exit %d: %s", code, errOut)
	}
}
