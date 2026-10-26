package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/agentenv"
	"github.com/rahulnakmol/agentic-ayudha/internal/delegate"
	"github.com/rahulnakmol/agentic-ayudha/internal/ready"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// Story #1004: Start begins the mission's Kanha on the tool the plan records,
// and each story thread the Kanha starts names it. The missionWorld of Story
// #997, with every receiver faked (mission_test.go: exec, the Cursor API, and
// the local Kanha).

// draftedBy records session as the session that drafted Feature f's approved intent.
func (m *missionWorld) draftedBy(f int, session string) {
	m.t.Helper()
	path := ready.BaselinePath(f, "intent", 1, false)
	var b ready.Baseline
	if err := json.Unmarshal([]byte(git(m.t, m.bare, "show", "ledger:"+path)), &b); err != nil {
		m.t.Fatal(err)
	}
	b.SourceSession = session
	data, _ := json.Marshal(b)
	putLedger(m.t, m.bare, path, string(data))
}

// featureWith adds Feature f with stories, and Approve to build of it.
func (m *missionWorld) featureWith(f int, stories ...int) {
	m.t.Helper()
	m.issue(f, fmt.Sprintf("Feature %d", f), "feature", 0, "")
	for _, n := range stories {
		m.issue(n, fmt.Sprintf("Story %d", n), "story", f, "")
	}
	m.feature(f, stories...)
}

func (m *missionWorld) setTool(tool string) { m.mu.Lock(); m.tool = tool; m.mu.Unlock() }

// start approves Feature f and has login start its mission on tool; it returns the output.
func (m *missionWorld) start(f int, login, tool string) string {
	m.t.Helper()
	if code, out, errOut := m.approve(f); code != 0 {
		m.t.Fatalf("approve %d: exit %d: %s%s", f, code, out, errOut)
	}
	m.setTool(tool)
	m.g.Viewer = login
	defer func() { m.g.Viewer = "rahulnakmol" }()
	code, out, errOut := m.mission(m.person, login, "start", f)
	if code != 0 {
		m.t.Fatalf("mission start %d on %s: exit %d: %s%s", f, tool, code, out, errOut)
	}
	return out
}

// parent is the Kanha recorded in specs/<f>/approved.json.
func (m *missionWorld) parent(f int) ready.Parent {
	m.t.Helper()
	var a ready.Approval
	raw := git(m.t, m.bare, "show", "ledger:"+ready.ApprovalPath(f))
	if err := json.Unmarshal([]byte(raw), &a); err != nil || a.Parent == nil {
		m.t.Fatalf("approved.json of #%d has no parent:\n%s", f, raw)
	}
	return *a.Parent
}

// cursorCalls are the requests to Cursor's API: "POST /v1/agents", each with its body.
func (m *missionWorld) cursorCalls() ([]string, []map[string]any) {
	m.cursor.mu.Lock()
	defer m.cursor.mu.Unlock()
	var calls []string
	var bodies []map[string]any
	for _, r := range m.cursor.reqs {
		calls = append(calls, fmt.Sprint(r["method"], " ", r["path"]))
		b, _ := r["body"].(map[string]any)
		bodies = append(bodies, b)
	}
	return calls, bodies
}

// readyStory makes story n Ready, its snapshot on the ledger, as a person's `aayu ready <n>` does.
func (m *missionWorld) readyStory(n int) {
	m.t.Helper()
	if code, out, errOut := run(m.t, m.person, m.opts, "ready", fmt.Sprint(n), "--repo", "o/r"); code != 0 {
		m.t.Fatalf("ready %d: exit %d: %s%s", n, code, out, errOut)
	}
}

// kanhaBuild is the Kanha's `aayu build <n> --delegate <tool>` for a story its list marked start.
func (m *missionWorld) kanhaBuild(n int, tool string) BuildResult {
	m.t.Helper()
	code, out, errOut := run(m.t, m.agent, m.opts, "build", fmt.Sprint(n), "--delegate", tool, "--repo", "o/r", "--json", "--asked", "aayu mission next marked it start")
	var res BuildResult
	if code != 0 || json.Unmarshal([]byte(out), &res) != nil || res.Delegation == nil {
		m.t.Fatalf("build %d --delegate %s: exit %d: %s%s", n, tool, code, out, errOut)
	}
	return res
}

// state is delegate/<n>/state.json.
func (m *missionWorld) state(n int) delegate.State {
	m.t.Helper()
	var st delegate.State
	if err := json.Unmarshal([]byte(git(m.t, m.bare, "show", "ledger:"+delegate.LedgerPath(n))), &st); err != nil {
		m.t.Fatal(err)
	}
	return st
}

// checkKanhaBrief fails unless brief is the Kanha brief of Feature f on tool.
func checkKanhaBrief(t *testing.T, brief string, f int, tool string) {
	t.Helper()
	for _, want := range []string{fmt.Sprintf("# Kanha of Feature #%d", f), fmt.Sprintf("Run `aayu mission next %d`", f),
		"run `aayu build <n> --delegate " + tool + "` once", "Never merge, approve, ship, or skip a gate."} {
		if !strings.Contains(brief, want) {
			t.Fatalf("the Kanha brief does not say %q:\n%s", want, brief)
		}
	}
}

// Criterion 1: @rahulnakmol starts #889 from the thread that drafted its
// intent, and his tool is amp: that thread gets the Kanha brief, and is the
// parent in specs/889/approved.json; no new thread starts.
func TestMissionStartBriefsDraftingThread(t *testing.T) {
	m := newMissionWorld(t)
	m.scenario889(nil)
	const drafting = "T-01a12c63-e530-769d-9c2b-1cd261372018"
	m.draftedBy(889, drafting)
	out := m.start(889, "rahulnakmol", "amp")
	if want := []string{"amp threads export " + drafting, "amp threads continue " + drafting + " -ox <brief>"}; !slices.Equal(m.callLog(), want) {
		t.Fatalf("receiver calls: %q, want %q", m.callLog(), want)
	}
	checkKanhaBrief(t, m.briefs[0], 889, "amp")
	plan, _ := m.plan(889)
	url := "https://ampcode.com/threads/" + drafting
	want := ready.Parent{Scope: "feature", Key: "889", Host: "amp", Session: drafting, URL: url, Mode: "cloud", StartedBy: "rahulnakmol", StartedAt: plan.StartedAt}
	if got := m.parent(889); got != want || plan.Tool != "amp" {
		t.Fatalf("parent %+v, want %+v; plan tool %q", got, want, plan.Tool)
	}
	if !strings.Contains(out, "the Kanha of #889 is the thread that drafted its intent, "+url+": it has its brief") {
		t.Fatalf("start's report: %s", out)
	}
	// The start is in the actions log, with the session it reached.
	log, _ := os.ReadFile(m.dir + "/.aayudha/telemetry/actions.jsonl")
	if !strings.Contains(string(log), `"kind":"mission.kanha.start"`) || !strings.Contains(string(log), `"thread":"drafting"`) || !strings.Contains(string(log), drafting) {
		t.Fatalf("actions log has no Kanha start:\n%s", log)
	}
	// A new approval of #889, with new milestones, keeps its Kanha: a second Start starts nothing, with no device code.
	if code, out, errOut := m.approve(889, "--milestones", "MS1=#20,#21 MS2=#22,#23"); code != 0 {
		t.Fatalf("approve 889 again: exit %d: %s%s", code, out, errOut)
	}
	if got := m.parent(889); got != want || !strings.Contains(git(t, m.bare, "show", "ledger:"+ready.ApprovalPath(889)), `"version": 2`) {
		t.Fatalf("parent after approval 2: %+v", got)
	}
	n := m.commits()
	if code, _, errOut := m.mission(m.person, "rahulnakmol", "start", 889); code != ExitBlocked || m.codes() != 0 || m.commits() != n ||
		!strings.Contains(errOut, "@rahulnakmol started it on amp. Its tool and budget do not change while it runs.") || len(m.callLog()) != 2 {
		t.Fatalf("a second start: exit %d: %s", code, errOut)
	}
}

// Criterion 2: the drafting thread is not known, or is archived; on claude
// Start begins a new Claude Code session with the Kanha brief and records it
// as the parent. On amp an archived drafting thread gets no brief: a new
// thread does.
func TestMissionStartNewClaudeSession(t *testing.T) {
	m := newMissionWorld(t)
	const archived = "T-01a0aa0e-f4c9-72da-9318-ba0d2ea7144b"
	m.archived[archived] = true
	m.featureWith(889, 20) // the drafting thread is not known
	m.featureWith(874, 950)
	m.draftedBy(874, archived)
	session := ready.Parent{Scope: "feature", Host: "claude", Session: kanhaClaude, URL: "https://claude.ai/code/" + kanhaClaude, Mode: "cloud", StartedBy: "rahulnakmol"}
	for _, f := range []int{889, 874} {
		m.calls = nil
		m.start(f, "rahulnakmol", "claude")
		got, want := m.parent(f), session
		want.Key, want.StartedAt = fmt.Sprint(f), got.StartedAt
		if got != want || !slices.Equal(m.callLog(), []string{"claude --cloud <brief>"}) {
			t.Fatalf("#%d: parent %+v, calls %q", f, got, m.callLog())
		}
		checkKanhaBrief(t, m.briefs[len(m.briefs)-1], f, "claude")
	}
	// On amp, an archived drafting thread, one this account cannot read, and one it reads no export of get no
	// brief: a new thread does. So does a session that is not an Amp thread.
	for f, drafting := range map[int]string{901: archived, 902: "T-01a0aa0e-0000-7000-8000-00000000dead", 903: "T-01a0aa0e-0000-7000-8000-00000000beef", 904: "session_x"} {
		m.featureWith(f, f+100)
		m.draftedBy(f, drafting)
		m.calls = nil
		m.start(f, "rahulnakmol", "amp")
		want := []string{"amp threads export " + drafting, fmt.Sprintf("amp -ox <brief> --project o/r --title aayu #%d: Kanha of #%d --orb-size a1.small", f, f)}
		if f == 904 {
			want = want[1:]
		}
		if got := m.parent(f); got.Session != kanhaAmpThread || got.URL != "https://ampcode.com/threads/"+kanhaAmpThread || !slices.Equal(m.callLog(), want) {
			t.Fatalf("#%d on amp: parent %+v, calls %q", f, got, m.callLog())
		}
	}
}

// Criterion 3: on codex, copilot, or opencode Start begins a local Kanha on
// the starter's machine (ST13), records it with mode local and login@machine,
// and says the mission pauses while that machine is off.
func TestMissionStartLocalKanha(t *testing.T) {
	m := newMissionWorld(t)
	host, _ := os.Hostname()
	for i, tool := range []string{"codex", "copilot", "opencode"} {
		f := 889 + i
		m.featureWith(f, 20+i)
		m.calls = nil
		out := m.start(f, "rahulnakmol", tool)
		got := m.parent(f)
		want := ready.Parent{Scope: "feature", Key: fmt.Sprint(f), Host: tool, Session: localSession, Mode: "local", Machine: "rahulnakmol@" + host, StartedBy: "rahulnakmol", StartedAt: got.StartedAt}
		if got != want || !slices.Equal(m.callLog(), []string{"local " + tool + " <brief>"}) {
			t.Fatalf("%s: parent %+v, calls %q", tool, got, m.callLog())
		}
		if line := fmt.Sprintf("the Kanha of #%d runs on this machine; while it is off, #%d pauses at the next merge", f, f); !strings.Contains(out, line) {
			t.Fatalf("%s: start's report: %s", tool, out)
		}
		checkKanhaBrief(t, m.briefs[len(m.briefs)-1], f, tool)
	}
	// Until #1012 builds the local Kanha, Start refuses those tools before the device code, and writes nothing.
	m.featureWith(901, 60)
	if code, out, errOut := m.approve(901); code != 0 {
		t.Fatalf("approve 901: exit %d: %s%s", code, out, errOut)
	}
	m.opts.LocalKanha = nil
	n := m.commits()
	code, _, errOut := m.mission(m.person, "rahulnakmol", "start", 901)
	if code != ExitBlocked || m.codes() != 0 || m.commits() != n ||
		!strings.Contains(errOut, "Start begins the mission's Kanha on opencode, and a local Kanha on opencode is not built yet (#1012). Nothing was started.") {
		t.Fatalf("start with no local Kanha: exit %d, device codes %d: %s", code, m.codes(), errOut)
	}
}

// Criterion 4: on cursor Start creates one Cursor cloud agent with the Kanha
// brief, opens no PR, and records it with mode cloud; each later event is one
// run on that agent (POST /v1/agents/{id}/runs), never a new agent.
func TestMissionStartCursorCloudAgent(t *testing.T) {
	m := newMissionWorld(t)
	m.scenario889(nil)
	m.start(889, "rahulnakmol", "cursor")
	calls, bodies := m.cursorCalls()
	if !slices.Equal(calls, []string{"POST /v1/agents"}) || bodies[0]["autoCreatePR"] != false || len(m.callLog()) != 0 {
		t.Fatalf("Cursor calls %q, first body %v, CLI calls %q", calls, bodies, m.callLog())
	}
	checkKanhaBrief(t, bodies[0]["prompt"].(map[string]any)["text"].(string), 889, "cursor")
	p := m.parent(889)
	want := ready.Parent{Scope: "feature", Key: "889", Host: "cursor", Session: cursorKanhaAgent, URL: "https://cursor.com/agents?id=" + cursorKanhaAgent, Mode: "cloud", StartedBy: "rahulnakmol", StartedAt: p.StartedAt}
	if p != want {
		t.Fatalf("parent %+v, want %+v", p, want)
	}
	// Two later events reach the recorded agent as one run each.
	kanha := delegate.CursorAPI{BaseURL: m.cursorAt, Key: "cursor-key"}
	for _, event := range []string{"merged #20 (PR #41): run aayu mission next 889", "blocked #21: run aayu mission next 889"} {
		if task, err := kanha.FollowUp(context.Background(), delegate.State{Task: p.Session, TaskURL: p.URL}, event); err != nil || task.ID != cursorKanhaAgent+"/run-02" {
			t.Fatalf("event %q: task %+v, %v", event, task, err)
		}
	}
	calls, bodies = m.cursorCalls()
	run := "POST /v1/agents/" + cursorKanhaAgent + "/runs"
	if !slices.Equal(calls, []string{"POST /v1/agents", run, run}) || bodies[2]["prompt"].(map[string]any)["text"] != "blocked #21: run aayu mission next 889" {
		t.Fatalf("Cursor calls after two events: %q", calls)
	}
}

// Criterion 5: the Kanha starts story #60 of #889: delegate/60/state.json has
// feature 889 and the Kanha's session and URL. A story of a Feature whose
// mission has not started names neither.
func TestStoryStateNamesFeatureAndKanha(t *testing.T) {
	m := newMissionWorld(t)
	m.featureWith(889, 60)
	m.featureWith(874, 950)
	m.start(889, "rahulnakmol", "cursor")
	if code, out, errOut := m.approve(874); code != 0 {
		t.Fatalf("approve 874: exit %d: %s%s", code, out, errOut)
	}
	m.readyStory(60)
	m.readyStory(950)
	p := m.parent(889)
	m.kanhaBuild(60, "cursor")
	if st := m.state(60); st.Feature != 889 || st.Kanha != p.Session || st.KanhaURL != p.URL || st.Kanha != cursorKanhaAgent || st.Task != cursorStoryAgent+"/run-01" {
		t.Fatalf("delegate/60/state.json: feature %d, kanha %q (%q), task %q", st.Feature, st.Kanha, st.KanhaURL, st.Task)
	}
	m.kanhaBuild(950, "cursor")
	if st := m.state(950); st.Feature != 0 || st.Kanha != "" || st.KanhaURL != "" {
		t.Fatalf("delegate/950/state.json outside a started mission: %+v", st)
	}
}

// Story #1006: the Kanha of #889 runs on amp, whose orb runs aayu. Each story
// thread it starts owns its PR and ships it on proof (criterion 1). A story
// handed to Codex or Cursor cloud, which cannot run aayu, is told never to
// merge (criterion 2). #52 depends on #40, which merged as PR #41: its brief
// lists PR #41, its ship record, and the asks answered on #40 (criterion 3).
func TestDelegateBriefForMissionStory(t *testing.T) {
	m := newMissionWorld(t)
	// Amp's cloud agent is connected: its CLI is installed and signed in, and its recorded project is this repository.
	look := m.opts.LookPath
	m.opts.LookPath = func(name string) (string, error) {
		if name == "amp" {
			return "/usr/bin/amp", nil
		}
		return look(name)
	}
	m.agent["AMP_API_KEY"] = "amp-key"
	if err := (agentenv.Records{Cloud: map[string]agentenv.Record{agentenv.Amp: {Method: "cloud", Project: "o/r"}}}).Save(m.dir); err != nil {
		t.Fatal(err)
	}
	m.issue(40, "Story 40", "story", 0, "")
	m.closeIssue(40)
	m.issue(889, "Feature 889", "feature", 0, "")
	m.issue(50, "Story 50", "story", 889, "")
	m.issue(52, "Story 52", "story", 889, "depends-on #40")
	m.issue(44, "Story 44", "story", 0, "")
	m.closeIssue(44)
	m.issue(54, "Story 54", "story", 889, "depends-on #44")
	m.feature(889, 50, 52, 54)
	m.featureWith(874, 950, 951)
	// #40 shipped as PR #39, was reverted, and shipped again as PR #41; PR #39's head sorts after PR #41's, so only
	// the ship time puts them in order. #400 is another story, whose record a brief of #52 must not list.
	for _, s := range []struct {
		story, pr int
		head, at  string
	}{{40, 41, fmt.Sprintf("%040d", 41), "2026-10-26T13:41:00Z"}, {40, 39, strings.Repeat("f", 40), "2026-10-25T09:00:00Z"}, {400, 401, fmt.Sprintf("%040d", 401), "2026-10-26T13:41:00Z"}} {
		rec, _ := json.Marshal(ShipRecord{Repo: "o/r", Story: s.story, PR: s.pr, Head: s.head, MergeCommit: fmt.Sprintf("%07d", s.pr*1000), ShippedAt: s.at})
		putLedger(t, m.bare, fmt.Sprintf("proof/%d/%s/authoritative/ship.json", s.story, s.head), string(rec))
		putLedger(t, m.bare, fmt.Sprintf("proof/%d/%s/authoritative/pack.json", s.story, s.head), "{}")
	}
	putLedger(t, m.bare, "proof/44/"+strings.Repeat("4", 40)+"/authoritative/ship.json", "{}")
	putLedger(t, m.bare, "specs/40/answers.md", "Q: Which tool runs the Kanha?\nA: The tool in missions/<f>/plan.json.\n")
	m.start(889, "rahulnakmol", "amp")
	if code, out, errOut := m.approve(874); code != 0 {
		t.Fatalf("approve 874: exit %d: %s%s", code, out, errOut)
	}
	for _, n := range []int{50, 52, 54, 950, 951} {
		m.readyStory(n)
	}
	brief := func(n int, tool string) string {
		if d := m.kanhaBuild(n, tool).Delegation; d.Receiver != tool || d.Mode != "cloud" {
			t.Fatalf("#%d: %+v", n, d)
		}
		if tool == "cursor" {
			_, bodies := m.cursorCalls()
			return bodies[len(bodies)-1]["prompt"].(map[string]any)["text"].(string)
		}
		return m.briefs[len(m.briefs)-1]
	}
	b50 := brief(50, "amp")
	for _, want := range []string{
		"You own story #50 to its merge, in your own worktree on a new branch `" + m.state(50).Branch + "` from `dev`",
		"You may use subagents",
		"After each push, run `aayu inspect <pr>` and act on its findings",
		"Before you ship, rebase the branch on the latest `origin/dev`",
		"run `aayu ship <pr>`: ship decides, and merges on proof",
		"Never merge another way: never `gh pr merge`, never `--admin`",
	} {
		if !strings.Contains(b50, want) {
			t.Errorf("#50's brief on amp lacks %q:\n%s", want, b50)
		}
	}
	if strings.Contains(b50, "## Merged work") {
		t.Errorf("#50 depends on nothing, and its brief lists merged work:\n%s", b50)
	}
	b52 := brief(52, "amp")
	for _, want := range []string{
		"### #40\n\n- Merged as PR #39 (merge commit `0039000`); its ship record is `proof/40/" + strings.Repeat("f", 40) + "/authoritative/ship.json` on the ledger.\n" +
			"- Merged as PR #41 (merge commit `0041000`); its ship record is `proof/40/" + fmt.Sprintf("%040d", 41) + "/authoritative/ship.json` on the ledger.\n",
		"- The asks answered on #40 (`specs/40/answers.md` on the ledger):\n\n```text\nQ: Which tool runs the Kanha?\nA: The tool in missions/<f>/plan.json.\n```\n",
		"run `aayu ship <pr>`",
	} {
		if !strings.Contains(b52, want) {
			t.Errorf("#52's brief lacks %q:\n%s", want, b52)
		}
	}
	if strings.Contains(b52, "PR #401") {
		t.Errorf("#52's brief lists #400's PR:\n%s", b52)
	}
	// A ship record with no PR or merge commit stops the hand-off: no brief leaves without it.
	calls := len(m.callLog())
	if code, out, errOut := run(t, m.agent, m.opts, "build", "54", "--delegate", "amp", "--repo", "o/r", "--asked", "aayu mission next marked it start"); code == 0 ||
		!strings.Contains(errOut, "proof/44/"+strings.Repeat("4", 40)+"/authoritative/ship.json: no PR or merge commit") || len(m.callLog()) != calls {
		t.Fatalf("build 54 with a broken ship record of #44: exit %d: %s%s", code, out, errOut)
	}
	for n, tool := range map[int]string{950: "codex", 951: "cursor"} {
		b := brief(n, tool)
		if !strings.Contains(b, "- Never merge, approve, or close the PR, and never push to `dev`, `rolling`, or `main`. A human merges with `aayu ship` after the merge gate and inspect pass.") ||
			strings.Contains(b, "run `aayu ship <pr>`") || strings.Contains(b, "You own story") {
			t.Errorf("#%d's brief on %s, which cannot run aayu:\n%s", n, tool, b)
		}
	}
}

// Criterion 6: mission #889's tool is cursor, and story #60's brief asks for
// codex: the Kanha starts it on cursor, and says why. A mission never mixes
// agent tools.
func TestMissionNeverMixesTools(t *testing.T) {
	m := newMissionWorld(t)
	m.featureWith(889, 60, 61)
	m.edit(func() {
		is := m.g.Issues[60]
		is.Body = strings.Replace(is.Body, "### Summary\n\n", "### Summary\n\nBuild this one with Codex: `aayu build 60 --delegate codex`. ", 1)
	})
	m.start(889, "rahulnakmol", "cursor")
	m.readyStory(60)
	m.readyStory(61)
	if !strings.Contains(m.g.Issues[60].Body, "--delegate codex") {
		t.Fatal("story #60 does not ask for codex")
	}
	for n, asked := range map[int]string{60: "codex", 61: "auto"} {
		res := m.kanhaBuild(n, asked)
		if d := res.Delegation; d.Receiver != "cursor" || d.Reason != "the mission of #889 runs on cursor (missions/889/plan.json), and a mission never mixes agent tools" || m.state(n).Receiver != "cursor" {
			t.Fatalf("build %d --delegate %s: %+v", n, asked, d)
		}
	}
	calls, bodies := m.cursorCalls()
	if !slices.Equal(calls, []string{"POST /v1/agents", "POST /v1/agents", "POST /v1/agents"}) || bodies[1]["autoCreatePR"] != true || bodies[2]["autoCreatePR"] != true {
		t.Fatalf("Cursor calls: %q", calls)
	}
	if slices.ContainsFunc(m.callLog(), func(c string) bool { return strings.HasPrefix(c, "codex") }) {
		t.Fatalf("codex ran: %q", m.callLog())
	}
}

// Criterion 7: @rahulnakmol (cursor) starts #889, and @bob (codex) starts
// #874 in the same repository: #889's Kanha and story threads run on Cursor,
// #874's on Codex (its Kanha local, ST13), and each plan names its own tool.
func TestTwoMissionsTwoTools(t *testing.T) {
	m := newMissionWorld(t)
	m.featureWith(889, 20)
	m.featureWith(874, 950)
	m.start(889, "rahulnakmol", "cursor")
	m.start(874, "bob", "codex")
	host, _ := os.Hostname()
	for f, want := range map[int][3]string{889: {"cursor", "rahulnakmol", "cloud"}, 874: {"codex", "bob", "local"}} {
		plan, _ := m.plan(f)
		p := m.parent(f)
		if plan.Tool != want[0] || plan.StartedBy != want[1] || p.Host != want[0] || p.StartedBy != want[1] || p.Mode != want[2] {
			t.Fatalf("#%d: plan tool %q by %q; parent %+v", f, plan.Tool, plan.StartedBy, p)
		}
	}
	if p := m.parent(874); p.Machine != "bob@"+host || p.Session != localSession {
		t.Fatalf("#874's Kanha: %+v", p)
	}
	m.readyStory(20)
	m.readyStory(950)
	// Each Kanha asks for auto; each story runs on its own mission's tool.
	if d := m.kanhaBuild(20, "auto").Delegation; d.Receiver != "cursor" {
		t.Fatalf("#20: %+v", d)
	}
	if d := m.kanhaBuild(950, "auto").Delegation; d.Receiver != "codex" || d.Mode != "cloud" || m.state(950).Kanha != localSession {
		t.Fatalf("#950: %+v", d)
	}
	calls, _ := m.cursorCalls()
	codex := slices.DeleteFunc(m.callLog(), func(c string) bool { return !strings.HasPrefix(c, "codex") })
	if !slices.Equal(calls, []string{"POST /v1/agents", "POST /v1/agents"}) || !slices.Equal(codex, []string{"codex cloud exec --env env-1 --branch dev <brief>"}) {
		t.Fatalf("Cursor calls %q; codex calls %q", calls, codex)
	}
}

// Criterion 8: in Tatkal Feature #920, @bob (codex) starts story #930 and
// @rahulnakmol (cursor) starts #931: each runs on its starter's tool, the
// plan records each story's tool and starter, and #920's Kanha lists both as
// running and starts neither. A story of a Feature mission is not a person's to start.
func TestTatkalStoryRunsOnStartersTool(t *testing.T) {
	m := newMissionWorld(t)
	m.issue(906, "Release: v0.4.0", "release", 0, "")
	m.edit(func() {
		m.g.Issues[906].Body = fixtureBody(t, "release", func(f map[string]string) { f["version"], f[workitems.FieldLinks] = "v0.4.0", "" })
	})
	m.tatkal[920] = 906
	m.featureWith(920, 930, 931)
	m.start(920, "rahulnakmol", "amp") // the Tatkal Kanha: a new Amp thread
	// A story that is not Ready does not start, with no device code.
	if code, _, errOut := m.mission(m.person, "bob", "start", 930); code != ExitBlocked || m.codes() != 0 || !strings.Contains(errOut, "#930 is open, not Ready. Nothing was started.") {
		t.Fatalf("start of #930 before Ready: exit %d: %s", code, errOut)
	}
	m.readyStory(930)
	m.readyStory(931)
	// Before a person starts #930, the list marks it start (Story #998), and the Kanha's build of it starts nothing.
	if !strings.Contains(m.briefs[len(m.briefs)-1], "#920 is a Tatkal Feature: a person starts each story on their own tool (`aayu mission start <n>`). Start nothing") {
		t.Fatalf("the Tatkal Kanha brief:\n%s", m.briefs[len(m.briefs)-1])
	}
	if code, out, errOut := m.next(920); code != 0 || !strings.Contains(out, "start #930") {
		t.Fatalf("mission next 920 before any start: exit %d: %q%s", code, out, errOut)
	}
	n, calls := m.commits(), len(m.callLog())
	m.g.Reset()
	code, out, errOut := run(t, m.agent, m.opts, "build", "930", "--delegate", "amp", "--repo", "o/r", "--asked", "aayu mission next marked it start")
	if code != ExitBlocked || m.commits() != n || len(m.callLog()) != calls || len(m.g.Writes()) != 0 ||
		!strings.Contains(errOut, "#930 is a story of Tatkal Feature #920: a person starts it on their own tool, and the Kanha only orders and tracks it. Nothing was started.") {
		t.Fatalf("the Kanha's build of #930 before a person's start: exit %d, %d writes: %s%s", code, len(m.g.Writes()), out, errOut)
	}
	for _, s := range []struct {
		n           int
		login, tool string
	}{{930, "bob", "codex"}, {931, "rahulnakmol", "cursor"}} {
		m.setTool(s.tool)
		m.g.Viewer = s.login
		code, out, errOut := m.mission(m.person, s.login, "start", s.n)
		if code != 0 || !strings.Contains(out, fmt.Sprintf("@%s started #%d of Tatkal Feature #920 on %s; the Kanha of #920 orders and tracks it.", s.login, s.n, s.tool)) ||
			!strings.Contains(out, "handed off to "+s.tool+" (cloud)") {
			t.Fatalf("%s starts #%d: exit %d: %s%s", s.login, s.n, code, out, errOut)
		}
		if st := m.state(s.n); st.Receiver != s.tool || st.Feature != 920 || st.Kanha != kanhaAmpThread {
			t.Fatalf("delegate/%d/state.json: %+v", s.n, st)
		}
	}
	m.g.Viewer = "rahulnakmol"
	plan, raw := m.plan(920)
	if len(plan.StoryStarts) != 2 || plan.StoryStarts[0].Tool != "codex" || plan.StoryStarts[0].StartedBy != "bob" ||
		plan.StoryStarts[1].Tool != "cursor" || plan.StoryStarts[1].StartedBy != "rahulnakmol" || plan.Tool != "amp" {
		t.Fatalf("missions/920/plan.json:\n%s", raw)
	}
	// The Kanha's list starts neither, even when #931's hand-off left it Ready; a sync that asks for another tool
	// keeps #930 on its starter's.
	m.status(931, "ready", "")
	if code, out, errOut := m.next(920); code != 0 || !strings.HasPrefix(out, "waits #931: @rahulnakmol started it on cursor · subthread ") || strings.Count(out, "\n") != 1 {
		t.Fatalf("mission next 920: exit %d: %q%s", code, out, errOut)
	}
	m.g.Viewer = "bob" // the lease is his start's
	code, out, errOut = run(t, m.person, m.opts, "build", "930", "--delegate", "amp", "--repo", "o/r", "--json")
	var res BuildResult
	if code != 0 || json.Unmarshal([]byte(out), &res) != nil || res.Delegation.Receiver != "codex" || slices.ContainsFunc(m.callLog(), func(c string) bool { return strings.HasPrefix(c, "amp -ox") && strings.Contains(c, "#930") }) {
		t.Fatalf("a sync of #930 with --delegate amp: exit %d: %s%s", code, out, errOut)
	}
	m.g.Viewer = "rahulnakmol"
	// A second start of #930, and a person's start of a story of a Feature mission, start nothing.
	n = m.commits()
	if code, _, errOut := m.mission(m.person, "rahulnakmol", "start", 930); code != ExitBlocked || m.codes() != 0 || m.commits() != n ||
		!strings.Contains(errOut, "@bob started #930 at "+plan.StoryStarts[0].StartedAt+" on codex. Its tool does not change.") {
		t.Fatalf("a second start of #930: exit %d: %s", code, errOut)
	}
	m.featureWith(889, 20)
	m.start(889, "rahulnakmol", "cursor")
	if code, _, errOut := m.mission(m.person, "rahulnakmol", "start", 20); code != ExitBlocked || m.codes() != 0 ||
		!strings.Contains(errOut, "#20 is a story of #889, whose Kanha starts it on the tool in missions/889/plan.json. A person starts only a story of a Tatkal Feature.") {
		t.Fatalf("a person's start of #20: exit %d: %s", code, errOut)
	}
}

// A Kanha that does not start leaves the mission started with no parent, and
// says so; a second Start starts only the Kanha, on the plan's tool, with no
// change to the starter, the tool, or the budget. A tool aayu has no Kanha
// for is refused before the device code.
func TestMissionStartRetriesOnlyTheKanha(t *testing.T) {
	m := newMissionWorld(t)
	m.featureWith(889, 20)
	if code, out, errOut := m.approve(889); code != 0 {
		t.Fatalf("approve: exit %d: %s%s", code, out, errOut)
	}
	m.setTool("gemini")
	n := m.commits()
	if code, _, errOut := m.mission(m.person, "rahulnakmol", "start", 889); code != ExitBlocked || m.codes() != 0 || m.commits() != n ||
		!strings.Contains(errOut, "Start begins the mission's Kanha on gemini, and aayu starts no Kanha on gemini. Nothing was started.") {
		t.Fatalf("start on gemini: exit %d: %s", code, errOut)
	}
	m.setTool("cursor")
	noKey := map[string]string{}
	for k, v := range m.person {
		noKey[k] = v
	}
	delete(noKey, "AAYU_SECRET_CURSOR_API_KEY")
	code, _, errOut := m.mission(noKey, "rahulnakmol", "start", 889)
	if code != ExitExternal || !strings.Contains(errOut, "The mission of #889 runs on cursor, and no Kanha of it is recorded: cursor: not connected: no API key") ||
		!strings.Contains(errOut, "next: aayu mission start 889") {
		t.Fatalf("start with no Cursor key: exit %d: %s", code, errOut)
	}
	started, raw := m.plan(889)
	if started.StartedBy != "rahulnakmol" || strings.Contains(git(t, m.bare, "show", "ledger:"+ready.ApprovalPath(889)), `"parent"`) {
		t.Fatalf("after the failed Kanha: plan\n%s", raw)
	}
	m.setTool("claude") // his tool changed since: the Kanha still starts on the plan's
	code, out, errOut := m.mission(m.person, "rahulnakmol", "start", 889)
	if code != 0 || m.codes() != 1 || !strings.Contains(errOut, "Approving: Starts the Kanha of #889's mission: @rahulnakmol started the mission at "+started.StartedAt+" on cursor, and it has no Kanha") ||
		strings.Contains(out, "started the mission of #889") || !strings.Contains(out, "the Kanha of #889 runs on cursor: https://cursor.com/agents?id="+cursorKanhaAgent) {
		t.Fatalf("second start: exit %d: %s%s", code, out, errOut)
	}
	if again, raw2 := m.plan(889); raw2 != raw || m.parent(889).Host != "cursor" || again.Tool != "cursor" {
		t.Fatalf("the second start changed the plan:\n%s", raw2)
	}
}

// A Tatkal story starts only once its Tatkal mission has started; --dry-run
// records and hands off nothing; a failed hand-off says how to finish it.
func TestTatkalStoryStartRefusals(t *testing.T) {
	m := newMissionWorld(t)
	m.tatkal[920] = 906
	m.issue(906, "Release: v0.4.0", "release", 0, "")
	m.edit(func() {
		m.g.Issues[906].Body = fixtureBody(t, "release", func(f map[string]string) { f["version"], f[workitems.FieldLinks] = "v0.4.0", "" })
	})
	m.featureWith(920, 930)
	if code, out, errOut := m.approve(920); code != 0 {
		t.Fatalf("approve: exit %d: %s%s", code, out, errOut)
	}
	m.readyStory(930)
	if code, _, errOut := m.mission(m.person, "rahulnakmol", "start", 930); code != ExitBlocked || m.codes() != 0 ||
		!strings.Contains(errOut, "The Tatkal mission of #920 has not started, so #930 does not start. Nothing was started.") {
		t.Fatalf("start of #930 before #920's: exit %d: %s", code, errOut)
	}
	m.setTool("amp")
	if code, out, errOut := m.mission(m.person, "rahulnakmol", "start", 920); code != 0 {
		t.Fatalf("start 920: exit %d: %s%s", code, out, errOut)
	}
	m.setTool("cursor")
	n := m.commits()
	if code, out, errOut := m.mission(m.person, "rahulnakmol", "start", 930, "--dry-run"); code != 0 || m.commits() != n ||
		!strings.Contains(out, "--dry-run: ledger missions/920/plan.json planned, not written; nothing handed off") {
		t.Fatalf("start 930 --dry-run: exit %d: %s%s", code, out, errOut)
	}
	m.opts.EnsureEnv = func(context.Context, string, string) (agentenv.HostStatus, error) {
		return agentenv.HostStatus{}, errors.New("no Docker daemon answers")
	}
	code, out, errOut := m.mission(m.person, "rahulnakmol", "start", 930)
	if code != ExitExternal || !strings.Contains(errOut, "#930 is started on cursor, and its hand-off did not finish") || !strings.Contains(errOut, "next: aayu build 930 --delegate cursor") {
		t.Fatalf("start 930 with no cloud environment: exit %d: %s%s", code, out, errOut)
	}
	if p, _ := m.plan(920); len(p.StoryStarts) != 1 || p.StoryStarts[0].Tool != "cursor" {
		t.Fatalf("story starts: %+v", p.StoryStarts)
	}
}

// Start fails closed: an unreadable tool entry, cost estimate, or story
// parent starts nothing. Another ledger write while the Kanha is recorded is
// read again, and the parent lands on the same approval.
func TestMissionStartFailsClosed(t *testing.T) {
	m := newMissionWorld(t)
	m.featureWith(889, 20)
	if code, out, errOut := m.approve(889); code != 0 {
		t.Fatalf("approve: exit %d: %s%s", code, out, errOut)
	}
	n := m.commits()
	tool, est := m.opts.RepoTool, m.opts.CostEstimate
	m.opts.RepoTool = func(context.Context, string) (string, error) { return "", errors.New("the user config is unreadable") }
	if code, _, errOut := m.mission(m.person, "rahulnakmol", "start", 889); code != ExitExternal || !strings.Contains(errOut, "the user config is unreadable") {
		t.Fatalf("tool entry failing: exit %d: %s", code, errOut)
	}
	m.opts.RepoTool = tool
	m.opts.CostEstimate = func(context.Context, int) (*CostEstimate, error) { return nil, errors.New("no estimate store") }
	if code, _, errOut := m.mission(m.person, "rahulnakmol", "start", 889); code != ExitExternal || !strings.Contains(errOut, "no estimate store") {
		t.Fatalf("estimate failing: exit %d: %s", code, errOut)
	}
	m.opts.CostEstimate = est
	m.edit(func() { m.g.Fail = "GET /repos/o/r/issues/20/parent" })
	if code, _, errOut := m.mission(m.person, "rahulnakmol", "start", 20); code != ExitExternal || m.codes() != 0 {
		t.Fatalf("parent read failing: exit %d: %s", code, errOut)
	}
	m.edit(func() { m.g.Fail = "" })
	if m.commits() != n {
		t.Fatal("a failed start wrote the ledger")
	}
	// A Kanha that starts while the ledger keeps moving is not recorded: Start names it and says to stop it
	// first, never to start another.
	m.featureWith(874, 950)
	if code, out, errOut := m.approve(874); code != 0 {
		t.Fatalf("approve 874: exit %d: %s%s", code, out, errOut)
	}
	busy := m.opts
	busy.LedgerAttempt = func(int, string) {
		if strings.Contains(git(t, m.bare, "show", "ledger:"+MissionPlanPath(874)), `"started_by"`) {
			putLedger(t, m.bare, fmt.Sprintf("specs/874/other-%d.json", time.Now().UnixNano()), "{}")
		}
	}
	m.device("rahulnakmol")
	url := "https://cursor.com/agents?id=" + cursorKanhaAgent
	code, out, errOut := run(t, m.person, busy, "mission", "start", "874", "--repo", "o/r")
	if code != ExitExternal || !strings.Contains(errOut, "The Kanha of #874 started on cursor ("+url+"), and aayu did not record it: ") ||
		!strings.Contains(errOut, "Do not start another while it runs.") || !strings.Contains(errOut, "next: stop the Kanha at "+url+", then run aayu mission start 874") {
		t.Fatalf("a Kanha the ledger did not take: exit %d: %s%s", code, out, errOut)
	}
	race, pushed := m.opts, false
	race.LedgerAttempt = func(int, string) {
		if !pushed && strings.Contains(git(t, m.bare, "show", "ledger:"+MissionPlanPath(889)), `"started_by"`) {
			pushed = true
			putLedger(t, m.bare, "specs/889/other.json", "{}")
		}
	}
	m.device("rahulnakmol")
	if code, out, errOut := run(t, m.person, race, "mission", "start", "889", "--repo", "o/r"); code != 0 || !pushed || m.parent(889).Host != "cursor" {
		t.Fatalf("a ledger write while the Kanha is recorded: exit %d, pushed %t: %s%s", code, pushed, out, errOut)
	}
}
