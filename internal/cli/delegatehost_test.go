package cli

import (
	"context"
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/delegate"
	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
	"github.com/rahulnakmol/agentic-ayudha/internal/models"
)

// Bug #1208: an Amp mission starts its stories from its Kanha thread with no
// amp CLI sign-in. Build claims the story and prints a host hand-off; the
// Kanha creates the thread with its own thread tool and records it with
// --thread. The Kanha runs in an Amp thread (m.agent sets AMP_THREAD_ID).

const hostThread = "T-01a1c0de-0000-7000-8000-000000001208"

// hostMission is mission #889 on amp with stories, each Ready, whose Kanha's
// amp CLI is installed and not signed in (no AMP_API_KEY, no saved account).
func (m *missionWorld) hostMission(stories ...int) {
	m.t.Helper()
	m.ampMission(889, stories...)
	delete(m.agent, "AMP_API_KEY")
}

// ampCalls are the amp CLI calls the receivers made.
func (m *missionWorld) ampCalls() []string {
	return slices.DeleteFunc(m.callLog(), func(c string) bool { return !strings.HasPrefix(c, "amp ") })
}

// hostHandedOff lists mission #889 and runs the Kanha's build of #20 with the
// CLI not signed in. It checks build's text: the brief file, the thread
// title, and the command that records the thread. It returns the brief file
// and the amp calls made before the build.
func (m *missionWorld) hostHandedOff() (string, int) {
	m.t.Helper()
	m.hostMission(20, 21)
	if code, out, errOut := m.next(889); code != 0 || !strings.Contains(workLines(out), "start #20\n") {
		m.t.Fatalf("mission next: exit %d: %q%s", code, out, errOut)
	}
	calls := len(m.ampCalls())
	code, out, errOut := run(m.t, m.agent, m.opts, "build", "20", "--delegate", "amp", "--repo", "o/r", "--asked", "aayu mission next marked it start")
	hand := regexp.MustCompile("(?m)^  host hand-off: create one Amp thread titled \"aayu #20: Item 20\" with the text of (\\S+) as its prompt, then run `aayu build 20 --delegate amp --thread <T-id>`$").FindStringSubmatch(out)
	if code != 0 || hand == nil || !strings.HasPrefix(out, "#20 delegated to amp (cloud): host-handoff\n") {
		m.t.Fatalf("the host hand-off of #20: exit %d: %s%s", code, out, errOut)
	}
	return hand[1], calls
}

// Criterion 1: with AMP_THREAD_ID set and the CLI not signed in, build claims
// #20, writes its brief, prints a host hand-off, and starts no CLI. A second
// build sees the hand-off and starts no second thread.
func TestAmpHostHandoffPrints(t *testing.T) {
	m := newMissionWorld(t)
	path, calls := m.hostHandedOff()
	if got := m.ampCalls()[calls:]; len(got) != 0 {
		t.Fatalf("the host hand-off started the amp CLI: %q", got)
	}
	brief, err := os.ReadFile(path)
	st := m.state(20)
	if err != nil || !strings.Contains(string(brief), "story #20") || delegate.BriefHash(string(brief)) != st.BriefHash ||
		st.Status != dlgHost || st.Receiver != delegate.Amp || st.Mode != "cloud" || st.Feature != 889 || st.Kanha != kanhaAmpThread || st.Task != "" {
		t.Fatalf("the brief file %s (%v) and delegate/20/state.json %+v", path, err, st)
	}
	if got := m.labels(20); !slices.Contains(got, "status:in-progress") || !slices.Contains(got, LeaseLabel) {
		t.Fatalf("#20 is not claimed: %v", got)
	}
	if code, out, errOut := run(t, m.person, m.opts, "mission", "show", "889", "--repo", "o/r"); code != 0 ||
		!strings.Contains(out, "#20 in-progress (its host hand-off of "+st.StartedAt+" waits for its Amp thread)") {
		t.Fatalf("mission show 889: exit %d: %s%s", code, out, errOut)
	}
	code, out, errOut := run(t, m.agent, m.opts, "build", "20", "--delegate", "amp", "--repo", "o/r", "--asked", "aayu mission next marked it start")
	if code != ExitBlocked || !strings.Contains(errOut, "its host hand-off of "+st.StartedAt+" waits for its thread, and build starts no second one") ||
		!strings.Contains(errOut, path) || len(m.ampCalls()) != calls || m.state(20).Handoffs != 1 {
		t.Fatalf("a second build of #20: exit %d: %s%s", code, out, errOut)
	}
	// A brief that cannot be written fails #21's start: its claim, lease, and status are released.
	if err := os.WriteFile(filepath.Join(filepath.Dir(filepath.Dir(path)), "21"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, errOut = run(t, m.agent, m.opts, "build", "21", "--delegate", "amp", "--repo", "o/r", "--asked", "aayu mission next marked it start")
	if code != ExitExternal || !strings.Contains(errOut, "the brief for the host was not written") || !strings.Contains(errOut, "#21 is status:ready again, and its claim and lease are released") ||
		m.claims(889)[21] != 0 || !slices.Contains(m.labels(21), "status:ready") || slices.Contains(m.labels(21), LeaseLabel) {
		t.Fatalf("build 21 with no place for its brief: exit %d, claims %v, labels %v: %s%s", code, m.claims(889), m.labels(21), out, errOut)
	}
}

// Criterion 2: the Kanha created the thread with its thread tool and records
// it: the delegation state records the thread and its URL as the CLI path
// does, and mission show lists #20 running on it. It is not recorded twice.
func TestAmpHostThreadRecorded(t *testing.T) {
	m := newMissionWorld(t)
	m.hostHandedOff()
	st := m.state(20)
	calls := len(m.ampCalls())
	code, out, errOut := run(t, m.agent, m.opts, "build", "20", "--delegate", "amp", "--thread", hostThread, "--repo", "o/r", "--json")
	var rec BuildResult
	if code != 0 || json.Unmarshal([]byte(out), &rec) != nil || rec.Delegation.Task != hostThread || rec.Delegation.Status != dlgHandedOff || rec.Delegation.Host != nil {
		t.Fatalf("build 20 --thread: exit %d: %s%s", code, out, errOut)
	}
	after := m.state(20)
	if after.Task != hostThread || after.TaskURL != "https://ampcode.com/threads/"+hostThread || after.Status != dlgHandedOff ||
		after.BriefHash != st.BriefHash || after.Branch != st.Branch || after.Handoffs != 1 || after.Kanha != kanhaAmpThread {
		t.Fatalf("delegate/20/state.json after --thread: %+v", after)
	}
	if got := m.ampCalls()[calls:]; len(got) != 0 {
		t.Fatalf("recording the thread started the amp CLI: %q", got)
	}
	code, out, errOut = run(t, m.person, m.opts, "mission", "show", "889", "--repo", "o/r")
	if code != 0 || !strings.Contains(out, "#20 in-progress on amp: https://ampcode.com/threads/"+hostThread) {
		t.Fatalf("mission show 889: exit %d: %s%s", code, out, errOut)
	}
	// Once recorded, no thread is recorded again. Another thread is named to stop, and the recorded one is not.
	other := "T-01a1c0de-0000-7000-8000-000000000002"
	if code, _, errOut := run(t, m.agent, m.opts, "build", "20", "--delegate", "amp", "--thread", other, "--repo", "o/r"); code != ExitBlocked ||
		!strings.Contains(errOut, "no host hand-off of #20 waits for its thread") || !strings.Contains(errOut, "Nothing was recorded; stop the thread "+other+".") || m.state(20).Task != hostThread {
		t.Fatalf("a second --thread: exit %d: %s", code, errOut)
	}
	if code, _, errOut := run(t, m.agent, m.opts, "build", "20", "--delegate", "amp", "--thread", hostThread, "--repo", "o/r"); code != ExitBlocked ||
		!strings.Contains(errOut, "Nothing was recorded.") || strings.Contains(errOut, "stop the thread") {
		t.Fatalf("the recorded thread again: exit %d: %s", code, errOut)
	}
}

// Criterion 5: a --thread that is not an Amp thread ID is refused, and
// nothing is recorded: no GitHub write, no ledger commit. An Amp thread ID
// with no hand-off waiting is not recorded either.
func TestAmpHostThreadNotAmpRefused(t *testing.T) {
	m := newMissionWorld(t)
	m.hostHandedOff()
	for _, bad := range []string{"T-1", "task_e_20", "https://ampcode.com/threads/" + hostThread, hostThread + "0"} {
		n := m.commits()
		m.g.Reset()
		code, out, errOut := run(t, m.agent, m.opts, "build", "20", "--delegate", "amp", "--thread", bad, "--repo", "o/r")
		if code != ExitUsage || !strings.Contains(errOut, "not an Amp thread ID") || !strings.Contains(errOut, "Nothing was recorded.") ||
			m.commits() != n || len(m.g.Writes()) != 0 || m.state(20).Task != "" || m.state(20).Status != dlgHost {
			t.Fatalf("--thread %q: exit %d, %d writes: %s%s", bad, code, len(m.g.Writes()), out, errOut)
		}
	}
	for _, args := range [][]string{{"--delegate", "codex"}, {"--delegate", "amp", "--local"}} {
		if code, _, errOut := run(t, m.agent, m.opts, append([]string{"build", "20", "--thread", hostThread, "--repo", "o/r"}, args...)...); code != ExitUsage ||
			!strings.Contains(errOut, "use it with --delegate amp and no --local") {
			t.Fatalf("--thread with %q: exit %d: %s", args, code, errOut)
		}
	}
	// #21 is listed start and has no hand-off: its claim, lease, and status stay as they are.
	n, claims := m.commits(), m.claims(889)
	m.g.Reset()
	code, out, errOut := run(t, m.agent, m.opts, "build", "21", "--delegate", "amp", "--thread", hostThread, "--repo", "o/r", "--asked", "a person asked for #21")
	if code != ExitBlocked || !strings.Contains(errOut, "no host hand-off of #21 waits for its thread") || !strings.Contains(errOut, "Nothing was recorded; stop the thread "+hostThread+".") ||
		m.commits() != n || len(m.g.Writes()) != 0 || !maps.Equal(m.claims(889), claims) || claims[21] != 1 || !slices.Contains(m.labels(21), "status:ready") {
		t.Fatalf("--thread on #21, which has no hand-off: exit %d, %d writes, commits %v→%v, claims %v→%v, labels %v: %s%s", code, len(m.g.Writes()), n, m.commits(), claims, m.claims(889), m.labels(21), out, errOut)
	}
}

// Criterion 3: a host hand-off whose thread is never recorded holds #20 no
// longer than its claim's life: then mission next releases the claim, the
// lease, and status:in-progress, and lists #20 start again. Before that it
// lists no start for #20, and recover starts nothing for it.
func TestAmpHostHandoffLapses(t *testing.T) {
	m := newMissionWorld(t)
	m.hostMission(20)
	clk := m.withClock()
	if code, out, errOut := m.next(889); code != 0 || workLines(out) != "start #20\n" {
		t.Fatalf("mission next: exit %d: %q%s", code, out, errOut)
	}
	if res := m.kanhaBuild(20, "amp"); res.Delegation.Host == nil {
		t.Fatalf("no host hand-off: %+v", res.Delegation)
	}
	clk.add(pickLockTTL - time.Minute)
	calls := len(m.ampCalls())
	if code, out, errOut := m.next(889); code != 0 || strings.Contains(workLines(out), "#20") {
		t.Fatalf("mission next within the claim's life: exit %d: %q%s", code, out, errOut)
	}
	code, out, errOut := run(t, m.agent, m.opts, "mission", "recover", "889", "--repo", "o/r")
	if code != 0 || !strings.Contains(out, "waits for its Amp thread: recover starts nothing for it") || len(m.ampCalls()) != calls {
		t.Fatalf("mission recover within the claim's life: exit %d: %s%s", code, out, errOut)
	}
	clk.add(2 * time.Minute)
	// A preview shows #20 start, and frees nothing.
	n := m.commits()
	m.g.Reset()
	if code, out, errOut := m.next(889, "--dry-run"); code != 0 || workLines(out) != "start #20\n" || m.commits() != n || len(m.g.Writes()) != 0 || m.state(20).Status != dlgHost {
		t.Fatalf("mission next --dry-run after the claim's life: exit %d, %d writes: %q%s", code, len(m.g.Writes()), out, errOut)
	}
	m.g.Reset()
	if code, out, errOut := m.next(889); code != 0 || workLines(out) != "start #20\n" {
		t.Fatalf("mission next after the claim's life: exit %d: %q%s", code, out, errOut)
	}
	if got := m.labels(20); !slices.Contains(got, "status:ready") || slices.Contains(got, "status:in-progress") || slices.Contains(got, LeaseLabel) {
		t.Fatalf("#20's labels after the lapse: %v", got)
	}
	if !slices.ContainsFunc(m.g.Writes(), func(r fake.Request) bool {
		b, _ := r.Body["body"].(string)
		return r.Method == "PATCH" && strings.Contains(b, "Lease released") && strings.Contains(b, "got no thread within 15 minutes")
	}) {
		t.Fatal("the lease of #20 was not released")
	}
	if st := m.state(20); st.Status != dlgLapsed || st.Task != "" {
		t.Fatalf("delegate/20/state.json after the lapse: %+v", st)
	}
	// The thread the Kanha created too late is not recorded.
	n = m.commits()
	if code, _, errOut := run(t, m.agent, m.opts, "build", "20", "--delegate", "amp", "--thread", hostThread, "--repo", "o/r"); code != ExitBlocked ||
		!strings.Contains(errOut, "no host hand-off of #20 waits for its thread (its delegation is lapsed") || m.commits() != n {
		t.Fatalf("--thread after the lapse: exit %d: %s", code, errOut)
	}
	// The next build hands #20 off afresh, on a new branch.
	if res := m.kanhaBuild(20, "amp"); res.Delegation.Host == nil || !strings.HasSuffix(res.Delegation.Host.Brief, "brief-2.md") || m.state(20).Handoffs != 2 {
		t.Fatalf("the build after the lapse: %+v, state %+v", res.Delegation, m.state(20))
	}
}

// Criterion 4: with the amp CLI signed in, build starts #20's thread through
// the CLI, as before, though the Kanha runs in an Amp thread.
func TestAmpSignedInStartsThroughCLI(t *testing.T) {
	m := newMissionWorld(t)
	m.ampMission(889, 20)
	if m.agent["AMP_THREAD_ID"] == "" || m.agent["AMP_API_KEY"] == "" {
		t.Fatalf("the Kanha's environment: %v", m.agent)
	}
	if code, out, errOut := m.next(889); code != 0 || workLines(out) != "start #20\n" {
		t.Fatalf("mission next: exit %d: %q%s", code, out, errOut)
	}
	res := m.kanhaBuild(20, "amp")
	if res.Delegation.Host != nil || res.Delegation.Task != kanhaAmpThread || res.Delegation.Status != dlgHandedOff ||
		!slices.ContainsFunc(m.ampCalls(), func(c string) bool {
			return strings.HasPrefix(c, "amp -ox <brief> --project o/r --title aayu #20: Item 20")
		}) {
		t.Fatalf("build 20 with the CLI signed in: %+v, calls %q", res.Delegation, m.ampCalls())
	}
}

// Criterion 6: inspect asks for fixes on #2's PR and the amp CLI is not
// signed in: the follow-up is printed as a host hand-off to send to #2's
// thread, and is not dropped.
func TestAmpHostFollowUp(t *testing.T) {
	g, env, opts, dir, bare := ampDelegate(t, "")
	exec := opts.DelegateExec
	var calls []string
	opts.DelegateExec = func(ctx context.Context, d string, e []string, stdin, name string, args ...string) (string, error) {
		calls = append(calls, name+" "+strings.Join(args, " "))
		return exec(ctx, d, e, stdin, name, args...)
	}
	if code, res, errOut := delegateRun(t, env, opts, "--delegate", "amp"); code != 0 || res.Delegation.Task == "" {
		t.Fatalf("hand-off: exit %d: %s", code, errOut)
	}
	head := strings.Repeat("c", 40)
	pr := g.AddPR("feat: show empty list", "Refs #2\n\nAmp's change.", "founder", "story/2-show-empty-list", head, "dev")
	g.Mu.Lock()
	pr.Diff = defectDiff
	g.Mu.Unlock()
	writeInspectPack(t, dir, pr.Number, head, nil)
	if code, _, errOut := inspectRun(t, env, opts, &models.Fake{Replies: []models.Response{inspectReply("fix", "P1")}}, pr.Number); code != ExitBlocked {
		t.Fatalf("inspect: exit %d: %s", code, errOut)
	}
	delete(env, "AMP_API_KEY")
	env["AMP_THREAD_ID"] = "T-01a1c0de-0000-7000-8000-00000000aaaa"
	sent := len(calls)
	code, res, errOut := delegateRun(t, env, opts, "--delegate", "amp")
	h := res.Delegation.Host
	if code != 0 || h == nil || h.Thread != res.Delegation.Task || h.Thread == "" || h.Record != "" || res.Delegation.PR != pr.Number {
		t.Fatalf("the follow-up with the CLI not signed in: exit %d %+v: %s", code, res.Delegation, errOut)
	}
	if got := calls[sent:]; len(got) != 0 {
		t.Fatalf("the follow-up started the amp CLI: %q", got)
	}
	msg, err := os.ReadFile(h.Brief)
	if err != nil || !strings.Contains(string(msg), "PR #"+strconv.Itoa(pr.Number)) || !strings.Contains(res.Next, "send the text of "+h.Brief+" to the Amp thread "+h.Thread) {
		t.Fatalf("the follow-up file %s (%v):\n%s\nnext: %s", h.Brief, err, msg, res.Next)
	}
	var st delegate.State
	if err := json.Unmarshal([]byte(git(t, bare, "show", "ledger:"+delegate.LedgerPath(2))), &st); err != nil || len(st.FollowUps) != 1 || st.FollowUps[0].Head != head || st.Status != dlgFix {
		t.Fatalf("delegate/2/state.json: %v %+v", err, st)
	}
}

// Criterion 3, the edges: a thread recorded after the claim's life, before a
// list ran, is kept; a story a person blocked meanwhile stays blocked; and a
// release that stopped half-way runs again on the next list.
func TestAmpHostLapseEdges(t *testing.T) {
	m := newMissionWorld(t)
	m.hostMission(20, 21, 22)
	clk := m.withClock()
	if code, out, errOut := m.next(889); code != 0 || workLines(out) != "start #20\nstart #21\nstart #22\n" {
		t.Fatalf("mission next: exit %d: %q%s", code, out, errOut)
	}
	for _, n := range []int{20, 21, 22} {
		if res := m.kanhaBuild(n, "amp"); res.Delegation.Host == nil {
			t.Fatalf("no host hand-off of #%d: %+v", n, res.Delegation)
		}
	}
	clk.add(pickLockTTL + time.Minute)
	if code, _, errOut := run(t, m.agent, m.opts, "build", "20", "--delegate", "amp", "--thread", hostThread, "--repo", "o/r"); code != 0 {
		t.Fatalf("a late record of #20: exit %d: %s", code, errOut)
	}
	m.status(21, "blocked", "")
	m.edit(func() { m.g.Fail = "DELETE /repos/o/r/issues/22/labels/" + LeaseLabel })
	if code, out, errOut := m.next(889); code != ExitExternal {
		t.Fatalf("mission next with #22's lease stuck: exit %d: %q%s", code, out, errOut)
	}
	m.edit(func() { m.g.Fail = "" })
	if code, out, errOut := m.next(889); code != 0 || workLines(out) != "waits #21: blocked\nstart #22\n" {
		t.Fatalf("mission next after the lapse: exit %d: %q%s", code, out, errOut)
	}
	if st := m.state(20); st.Task != hostThread || st.Status != dlgHandedOff || !slices.Contains(m.labels(20), "status:in-progress") {
		t.Fatalf("#20, recorded late: %+v, labels %v", st, m.labels(20))
	}
	if st := m.state(21); st.Status != dlgLapsed || !slices.Contains(m.labels(21), "status:blocked") || slices.Contains(m.labels(21), "status:ready") {
		t.Fatalf("#21, blocked during the hand-off: %+v, labels %v", st, m.labels(21))
	}
	if st := m.state(22); st.Status != dlgLapsed || !slices.Contains(m.labels(22), "status:ready") || slices.Contains(m.labels(22), "status:in-progress") {
		t.Fatalf("#22, released on the second list: %+v, labels %v", st, m.labels(22))
	}
}

// Criteria 3 and 5, races: the delegation of #20 changes while a --thread
// run renews the lease. When the hand-off lapsed, the record is refused, the
// thread is named to stop, and the lease it renewed is released, so it holds
// #20 no longer. When another run recorded the same thread, it is not named
// to stop, and the lease stays.
func TestAmpHostLateRecordReleasesLease(t *testing.T) {
	for _, c := range []struct {
		status, task, want string
		leased             bool
	}{
		{dlgLapsed, "", "no host hand-off of #20 waits for its thread (now lapsed). Nothing was recorded; stop the thread " + hostThread + ".", false},
		{dlgHandedOff, hostThread, "no host hand-off of #20 waits for its thread (now handed-off). Nothing was recorded.", true},
	} {
		m := newMissionWorld(t)
		m.hostHandedOff()
		now := m.state(20)
		now.Status, now.Task = c.status, c.task
		now.Events = append(now.Events, delegate.Event{At: time.Now().UTC().Format(time.RFC3339), Status: c.status, Detail: "by another run"})
		b, _ := json.Marshal(now)
		once := false
		m.edit(func() {
			m.g.Hook = func(method, path string) {
				if method == "PATCH" && strings.Contains(path, "/issues/comments/") && !once { // the build renews #20's lease
					once = true
					putLedger(t, m.bare, delegate.LedgerPath(20), string(b))
				}
			}
		})
		code, out, errOut := run(t, m.agent, m.opts, "build", "20", "--delegate", "amp", "--thread", hostThread, "--repo", "o/r")
		m.edit(func() { m.g.Hook = nil })
		if !once || code != ExitBlocked || !strings.Contains(errOut, c.want) || strings.Contains(errOut, "stop the thread") == c.leased {
			t.Fatalf("a record racing a %s delegation: once %t, exit %d: %q %q", c.status, once, code, out, errOut)
		}
		if got := m.labels(20); slices.Contains(got, LeaseLabel) != c.leased || m.state(20).Task != c.task {
			t.Fatalf("after the refused record of a %s delegation: labels %v, state %+v", c.status, got, m.state(20))
		}
	}
}
