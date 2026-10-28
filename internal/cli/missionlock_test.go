package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// Story #1013: Kanha's lock, on the missionWorld of Story #997.

// lockWorld is #889 approved to build with milestones MS1 to MS3: seven Ready
// stories (#20 to #26), an open Bug #27, and a closed Bug #28. #20 has three
// criteria and the labels effort:S, risk:medium, and priority:P1. The person's
// tool is tool.
func lockWorld(t *testing.T, tool string) *missionWorld {
	t.Helper()
	m := newMissionWorld(t)
	m.mu.Lock()
	m.tool = tool
	m.mu.Unlock()
	m.issue(889, "Agents pull the next story", "feature", 0, "")
	for n := 20; n <= 28; n++ {
		typ := "story"
		if n >= 27 {
			typ = "bug"
		}
		m.issue(n, fmt.Sprintf("Story %d", n), typ, 889, "")
	}
	m.edit(func() {
		is := m.g.Issues[20]
		_, fields := workitems.Sections(is.Body)
		is.Body = setSection(is.Body, "Completion Goal", fields["Completion Goal"]+"\n3. [api-trace] state: GET /cart; expected: 200 with no items")
		is.Labels = append(is.Labels, fake.Label{Name: "risk:medium"})
		m.g.Issues[28].State = "closed"
	})
	m.feature(889, 20, 21, 22, 23, 24, 25, 26)
	for n := 20; n <= 27; n++ {
		m.status(n, "ready", "")
	}
	if code, out, errOut := m.approve(889, "--milestones", "MS1=#20,#21,#22 MS2=#23,#24,#25 MS3=#26,#27,#28"); code != 0 {
		t.Fatalf("approve 889: exit %d: %s%s", code, out, errOut)
	}
	return m
}

// started is lockWorld started on amp, where the Kanha's aayu mission next runs.
func startedLock(t *testing.T) *missionWorld {
	t.Helper()
	m := lockWorld(t, "amp")
	if code, out, errOut := m.mission(m.person, "rahulnakmol", "start", 889); code != 0 {
		t.Fatalf("start 889: exit %d: %s%s", code, out, errOut)
	}
	return m
}

// client is the CLI's GitHub client for env, as every aayu verb builds it.
func (m *missionWorld) client(env map[string]string) (*Env, *github.Client) {
	m.t.Helper()
	home := m.t.TempDir()
	e, err := newEnv(IO{In: strings.NewReader(""), Out: io.Discard, Err: io.Discard}, Options{
		Getenv:    func(k string) string { return map[string]string{"HOME": home}[k] + env[k] },
		UserHome:  func() (string, error) { return home, nil },
		GHToken:   func(context.Context) (string, error) { return "", fmt.Errorf("gh disabled in tests") },
		GitHubAPI: m.g.URL(), GitHubGap: -1, Dir: m.dir,
	})
	if err != nil {
		m.t.Fatal(err)
	}
	e.Repo = "o/r"
	c, _, err := e.HumanGitHub(context.Background())
	if err != nil {
		m.t.Fatal(err)
	}
	return e, c
}

// editBody writes issue n's body through c.
func editBody(c *github.Client, n int, body string) error {
	_, err := c.Write(context.Background(), actions.Action{Kind: "github.issue.edit", Target: fmt.Sprintf("o/r#%d", n)}, func(ctx context.Context) (any, error) {
		_, _, err := c.REST.Issues.Update(ctx, "o", "r", n, gh.UpdateIssueRequest{Body: gh.Ptr(body)})
		return nil, err
	})
	return err
}

// labelWrites are the label writes an aayu verb makes on issue n, by what each does.
func labelWrites(c *github.Client, n int) map[string]func() error {
	write := func(f func(ctx context.Context) error) func() error {
		return func() error {
			_, err := c.Write(context.Background(), actions.Action{Kind: "github.issue.labels", Target: fmt.Sprintf("o/r#%d", n)}, func(ctx context.Context) (any, error) { return nil, f(ctx) })
			return err
		}
	}
	return map[string]func() error{
		"add priority:P0": write(func(ctx context.Context) error {
			_, _, err := c.REST.Issues.AddLabelsToIssue(ctx, "o", "r", n, []string{"priority:P0"})
			return err
		}),
		"remove priority:P1": func() error { return removeLabel(context.Background(), c, "o/r", n, "priority:P1") },
		"replace the labels": write(func(ctx context.Context) error {
			_, _, err := c.REST.Issues.ReplaceLabelsForIssue(ctx, "o", "r", n, []string{"type:story", "effort:S", "priority:P1"})
			return err
		}),
		"set the labels in an edit": write(func(ctx context.Context) error {
			_, _, err := c.REST.Issues.Update(ctx, "o", "r", n, gh.UpdateIssueRequest{Labels: []string{"type:story", "effort:L", "risk:medium", "priority:P1"}})
			return err
		}),
		// Two labels of one prefix: the held priority:P1 last does not hide priority:P0.
		"add priority:P0 beside the held priority:P1": write(func(ctx context.Context) error {
			_, _, err := c.REST.Issues.AddLabelsToIssue(ctx, "o", "r", n, []string{"priority:P0", "priority:P1"})
			return err
		}),
		"replace the labels with two priorities": write(func(ctx context.Context) error {
			_, _, err := c.REST.Issues.ReplaceLabelsForIssue(ctx, "o", "r", n, []string{"type:story", "effort:S", "risk:medium", "priority:P0", "priority:P1"})
			return err
		}),
		"remove every label": write(func(ctx context.Context) error {
			_, err := c.REST.Issues.RemoveLabelsForIssue(ctx, "o", "r", n)
			return err
		}),
		// GitHub also takes labels as objects, and a request whose body the client cannot replay.
		"add priority:P0 as a label object": write(func(ctx context.Context) error {
			req, err := c.REST.NewRequest(ctx, http.MethodPost, fmt.Sprintf("repos/o/r/issues/%d/labels", n), map[string]any{"labels": []map[string]string{{"name": "priority:P0"}}})
			if err == nil {
				_, err = c.REST.Do(req, nil)
			}
			return err
		}),
		"edit the body from a stream": write(func(ctx context.Context) error {
			req, err := c.REST.NewRequest(ctx, http.MethodPatch, fmt.Sprintf("repos/o/r/issues/%d", n), nil)
			if err == nil {
				req.Body, req.GetBody = io.NopCloser(strings.NewReader(`{"body":"Another story."}`)), nil
				_, err = c.REST.Do(req, nil)
			}
			return err
		}),
	}
}

// issueWrites are the recorded writes to issue n itself and its labels.
func (m *missionWorld) issueWrites(n int) []fake.Request {
	var out []fake.Request
	for _, r := range m.g.Writes() {
		if p := fmt.Sprintf("/repos/o/r/issues/%d", n); r.Path == p || strings.HasPrefix(r.Path, p+"/labels") {
			out = append(out, r)
		}
	}
	return out
}

// Criterion 1: the start card in aayu inbox names the 9 issues it locks
// (#889, its 7 stories, and its open Bug, not the closed one), what the lock
// holds and leaves open, when it ends, the tool from his user config, and up
// to 5 stories at once; it offers no tool choice and recommends nothing.
func TestStartCardWords(t *testing.T) {
	m := lockWorld(t, "cursor")
	m.device("rahulnakmol") // no code is asked for: opening the card starts nothing
	code, out, errOut := runIn(t, m.person, m.opts, "q", "inbox", "--repo", "o/r")
	card := "Mission start #889: Start the mission of #889\n" +
		"  " + inboxKinds[InboxStart].Line + "\n" +
		"  Starting the mission makes 9 issues read-only until its last milestone: #889, #20, #21, #22, #23, #24, #25, #26, #27\n" +
		"  Locked  each body, and its effort:, risk: and priority: labels\n" +
		"  Open    comments and status: labels\n" +
		"  Ends    at the last milestone of the last mission (MS3)\n" +
		"  It runs in Cursor, the tool you set for this repository.\n" +
		"  Agents  up to 5 stories at once\n"
	choices := "  1. Start the mission (preselected)\n  2. Not now: nothing is locked\n  3. Read the issues first\n"
	if code != 0 || !strings.Contains(out, card) || !strings.Contains(out, choices) || strings.Contains(out, "recommended") ||
		strings.Contains(out, "4. ") || m.codes() != 0 {
		t.Fatalf("inbox: exit %d, device codes %d:\n%s%s", code, m.codes(), out, errOut)
	}
	// The same words are what Start shows before the device code.
	m.device("nobody") // a refused code: nothing starts
	m.edit(func() { m.g.Device.Outcome = "deny" })
	_, _, errOut = run(t, m.person, m.opts, "mission", "start", "889", "--repo", "o/r")
	for _, l := range strings.Split(strings.TrimSpace(card), "\n")[2:] {
		if !strings.Contains(errOut, "Approving: "+strings.TrimSpace(l)+"\n") {
			t.Errorf("Start's card lacks %q:\n%s", strings.TrimSpace(l), errOut)
		}
	}
}

// Criterion 2: 1 Start the mission runs aayu mission start 889 and, after the
// presence check, plan.json gains the lock: active, confirmed by him, with
// each issue's body hash and its effort:, risk:, and priority: labels.
func TestMissionStartWritesLock(t *testing.T) {
	m := lockWorld(t, "cursor")
	m.device("rahulnakmol")
	code, out, errOut := runIn(t, m.person, m.opts, "1y", "inbox", "--repo", "o/r")
	if code != 0 || m.codes() != 1 || !strings.Contains(out, "@rahulnakmol started the mission of #889 on cursor") {
		t.Fatalf("inbox 1 Start the mission: exit %d, device codes %d:\n%s%s", code, m.codes(), out, errOut)
	}
	p, raw := m.plan(889)
	l := p.Lock
	if l == nil || l.State != LockActive || l.ConfirmedBy != "rahulnakmol" || l.StartedAt != p.StartedAt || l.EndedAt != nil || l.EndReason != nil ||
		!strings.Contains(raw, `"ended_at": null,`) || !strings.Contains(raw, `"end_reason": null,`) {
		t.Fatalf("plan.json lock after Start:\n%s", raw)
	}
	if _, err := time.Parse(time.RFC3339, l.StartedAt); err != nil {
		t.Fatalf("started_at %q: %v", l.StartedAt, err)
	}
	var nums []int
	for _, li := range l.Issues {
		nums = append(nums, li.Number)
	}
	if want := []int{889, 20, 21, 22, 23, 24, 25, 26, 27}; !slices.Equal(nums, want) {
		t.Fatalf("locked issues %v, want %v", nums, want)
	}
	m.g.Mu.Lock()
	body20 := m.g.Issues[20].Body
	m.g.Mu.Unlock()
	if li := l.Issues[1]; li.BodySHA256 != workitems.BodyHash(body20) ||
		!maps.Equal(li.Labels, map[string]string{"effort": "effort:S", "risk": "risk:medium", "priority": "priority:P1"}) {
		t.Fatalf("#20 in the lock: %+v", li)
	}
	if li := l.Issues[0]; len(li.Labels) != 0 || li.BodySHA256 == "" { // the Feature has none of the three labels
		t.Fatalf("#889 in the lock: %+v", li)
	}
}

// Criterion 3: 2 Not now starts nothing and locks nothing, and the card
// stays in the inbox.
func TestNotNowLocksNothing(t *testing.T) {
	m := lockWorld(t, "cursor")
	n := m.commits()
	m.device("rahulnakmol")
	m.g.Reset()
	code, out, errOut := runIn(t, m.person, m.opts, "2", "inbox", "--repo", "o/r")
	if code != 0 || !strings.Contains(out, "Not now: nothing is locked: nothing was written, and the card stays in the inbox.") || m.codes() != 0 {
		t.Fatalf("inbox 2 Not now: exit %d:\n%s%s", code, out, errOut)
	}
	if p, raw := m.plan(889); m.commits() != n || p.StartedBy != "" || p.Lock != nil || len(m.g.Writes()) != 0 {
		t.Fatalf("Not now wrote: %d commits, writes %v:\n%s", atoiOr(m.commits())-atoiOr(n), m.g.Writes(), raw)
	}
	code, out, errOut = run(t, m.person, m.opts, "inbox", "--list", "--json", "--repo", "o/r")
	var list inboxList
	if code != 0 || json.Unmarshal([]byte(out), &list) != nil ||
		!slices.ContainsFunc(list.Items, func(it InboxItem) bool {
			return it.Kind == InboxStart && it.Number == 889 && it.short() == "a person starts it"
		}) {
		t.Fatalf("inbox --list after Not now: exit %d: %s%s", code, out, errOut)
	}
}

// Criterion 4: while #889 runs, an aayu write that changes #20's body or its
// priority: label is refused before the request leaves the client.
func TestLockedWriteRefusedBeforeRequest(t *testing.T) {
	m := startedLock(t)
	e, c := m.client(m.agent)
	m.g.Reset()
	writes := labelWrites(c, 20)
	writes["edit the body"] = func() error { return editBody(c, 20, "### Summary\n\nAnother story.") }
	for what, write := range writes {
		err := write()
		if err == nil || !strings.Contains(err.Error(), "#20 is locked by mission #889 until it ends") || CodeOf(Wrap(ExitExternal, err)) != ExitBlocked || !lockRefused(err) {
			t.Errorf("%s: %v", what, err)
		}
	}
	if w := m.issueWrites(20); len(w) != 0 {
		t.Fatalf("refused writes reached GitHub: %v", w)
	}
	// ship's completion record of a locked story goes in a comment: its body stays as locked.
	m.g.Mu.Lock()
	before := m.g.Issues[20].Body
	m.g.Mu.Unlock()
	rec := ShipRecord{PR: 99, MergeCommit: strings.Repeat("a", 40), Head: strings.Repeat("b", 40), Pack: "proof/20/pack.json", ShippedAt: "2026-10-26T10:00:00Z",
		HumanPass: HumanPass{Kind: "proof", Note: "on proof"}, Actions: "actions/2026-10-26.jsonl"}
	if err := e.completeStory(context.Background(), c, "o/r", 20, rec); err != nil {
		t.Fatalf("ship's completion of locked #20: %v", err)
	}
	m.g.Mu.Lock()
	after, comments := m.g.Issues[20].Body, m.g.Issues[20].Comments
	m.g.Mu.Unlock()
	if after != before || !strings.HasPrefix(comments[len(comments)-1].Body, "### Outcome\n\nMerged in #99") || !slices.Contains(m.g.LabelNames(20), "status:merged") {
		t.Fatalf("ship's completion of locked #20: body changed %t, last comment %q", after != before, comments[len(comments)-1].Body)
	}
}

// Criterion 4, the other side: a write the lock does not hold goes through:
// another issue's body, #20's labels as the lock holds them, and #20's body
// once the lock ended.
func TestUnlockedWritesUnchanged(t *testing.T) {
	m := startedLock(t)
	_, c := m.client(m.agent)
	m.g.Reset()
	if err := editBody(c, 870, "### Summary\n\nThe Initiative."); err != nil {
		t.Fatalf("an unlocked body: %v", err)
	}
	if _, err := c.Write(context.Background(), actions.Action{Kind: "github.issue.labels.add", Target: "o/r#20"}, func(ctx context.Context) (any, error) {
		_, _, err := c.REST.Issues.AddLabelsToIssue(ctx, "o", "r", 20, []string{"priority:P1", "layer:L03"})
		return nil, err
	}); err != nil {
		t.Fatalf("#20's held priority and another label: %v", err)
	}
	m.setPlan(889, func(p *MissionPlan) { p.Lock.State = LockEnded })
	if err := editBody(c, 20, "### Summary\n\nAfter the mission."); err != nil {
		t.Fatalf("#20's body once the lock ended: %v", err)
	}
	if w := m.issueWrites(20); len(w) != 2 || len(m.issueWrites(870)) != 1 {
		t.Fatalf("writes: %v", m.g.Writes())
	}
	// With no ledger, nothing is locked.
	git(t, m.bare, "branch", "-D", "ledger")
	if err := editBody(c, 870, "### Summary\n\nWith no ledger."); err != nil {
		t.Fatalf("a body with no ledger: %v", err)
	}
}

// Criterion 5: @alice edits criterion 3 of #20 at 10:42. aayu mission next
// holds #20 and never writes its body back; #20 gets one lock comment; the
// inbox shows the locked-edit card with three choices and no recommendation.
func TestLockedEditHoldsStoryCommentsAndCard(t *testing.T) {
	m := startedLock(t)
	m.g.Mu.Lock()
	locked := m.g.Issues[20].Body
	m.g.Mu.Unlock()
	putLedger(t, m.bare, "specs/20/ready.md", "# o/r#20: Story 20\n\n### Ready snapshot\n\n## Body at Ready\n\n````markdown\n"+strings.TrimSpace(locked)+"\n````\n")
	edited := strings.Replace(locked, "expected: 200 with no items", "expected: 204 with no body", 1)
	m.edit(func() {
		is := m.g.Issues[20]
		is.Body, is.EditedBy, is.EditedAt = edited, "alice", time.Date(2026, 10, 26, 10, 42, 0, 0, time.Local)
	})
	m.status(20, "planned", "") // the Ready guard strips status:ready from a body that drifted
	m.g.Reset()
	code, out, errOut := m.next(889)
	if code != 0 || !strings.Contains(out, "waits #20: changed while locked\n") || strings.Contains(out, "start #20") {
		t.Fatalf("mission next 889: exit %d: %s%s", code, out, errOut)
	}
	want := LockMarker + "\n" + lockTag(889, workitems.BodyHash(edited)) + "\nThis body changed while the mission of #889 holds it: criterion 3, edited by @alice at 10:42. " +
		"aayu did not change it back. #20 waits until a person keeps the edit in the inbox, or the mission ends.\n"
	lockComments := func() []string {
		m.g.Mu.Lock()
		defer m.g.Mu.Unlock()
		var out []string
		for _, cm := range m.g.Issues[20].Comments {
			if strings.HasPrefix(cm.Body, LockMarker) {
				out = append(out, cm.Body)
			}
		}
		return out
	}
	if cs := lockComments(); len(cs) != 1 || cs[0] != want {
		t.Fatalf("lock comments on #20: %q", cs)
	}
	if w := m.issueWrites(20); len(w) != 0 {
		t.Fatalf("aayu wrote #20 itself: %v", w)
	}
	m.g.Mu.Lock()
	body := m.g.Issues[20].Body
	m.g.Mu.Unlock()
	if body != edited {
		t.Fatal("aayu changed #20's body back")
	}
	// The next wake sees the same body: no second comment.
	if code, out, _ := m.next(889); code != 0 || !strings.Contains(out, "waits #20: changed while locked\n") || len(lockComments()) != 1 {
		t.Fatalf("second mission next: exit %d, %d lock comments: %s", code, len(lockComments()), out)
	}
	code, out, errOut = run(t, m.person, m.opts, "inbox", "--list", "--json", "--repo", "o/r")
	var list inboxList
	if code != 0 || json.Unmarshal([]byte(out), &list) != nil {
		t.Fatalf("inbox --list: exit %d: %s%s", code, out, errOut)
	}
	i := slices.IndexFunc(list.Items, func(it InboxItem) bool { return it.Kind == InboxLocked && it.Number == 20 })
	if i < 0 {
		t.Fatalf("no locked-edit card for #20: %s", out)
	}
	card := list.Items[i]
	var labels []string
	for _, o := range card.Options {
		labels = append(labels, o.Label)
		if o.Recommended {
			t.Errorf("the card recommends %q", o.Label)
		}
	}
	if card.Title != "#20 changed while locked by the mission of #889" || card.short() != "changed while locked" || !slices.Equal(labels, []string{"Keep the edit: #20 is planned again from its new body",
		"Leave #20 waiting", "Open #20 on GitHub to undo the edit yourself"}) || slices.ContainsFunc(list.Items, func(it InboxItem) bool { return it.Kind == InboxLocked && it.Number != 20 }) {
		t.Fatalf("locked-edit card: %+v", card)
	}
	// In the terminal inbox: the card and its three choices, the third opening #20.
	code, out, errOut = runIn(t, m.person, m.opts, "2", "inbox", "--repo", "o/r")
	if code != 0 || !strings.Contains(out, "Changed while locked #20: #20 changed while locked by the mission of #889\n") ||
		!strings.Contains(out, "  1. Keep the edit: #20 is planned again from its new body (preselected)\n  2. Leave #20 waiting\n  3. Open #20 on GitHub to undo the edit yourself\n") ||
		!strings.Contains(out, "Leave #20 waiting: nothing was written, and the card stays in the inbox.") {
		t.Fatalf("inbox: exit %d:\n%s%s", code, out, errOut)
	}
}

// Criterion 6: a comment on #20 and a change of its status: label go
// through while #889 runs.
func TestLockLeavesCommentsAndStatusOpen(t *testing.T) {
	m := startedLock(t)
	_, c := m.client(m.agent)
	m.g.Reset()
	ctx := context.Background()
	if err := postComment(ctx, c, "o/r", 20, "github.comment", "A person's note on #20."); err != nil {
		t.Fatalf("comment on #20: %v", err)
	}
	if err := setStatus(ctx, c, "o/r", 20, m.g.LabelNames(20), "in-progress"); err != nil {
		t.Fatalf("status of #20: %v", err)
	}
	if labels := m.g.LabelNames(20); !slices.Contains(labels, "status:in-progress") || slices.Contains(labels, "status:ready") || !slices.Contains(labels, "priority:P1") {
		t.Fatalf("#20's labels: %v", labels)
	}
	if !slices.ContainsFunc(m.g.Writes(), func(r fake.Request) bool {
		return r.Method == http.MethodPost && r.Path == "/repos/o/r/issues/20/comments"
	}) {
		t.Fatalf("no comment on #20: %v", m.g.Writes())
	}
}

// Criterion 7: once every story and Bug merged, the last milestone of the
// last mission is done: the lock ends with its time, and #20's body is open.
func TestLockEndsAtLastMilestone(t *testing.T) {
	m := startedLock(t)
	for n := 20; n <= 27; n++ {
		m.closeIssue(n)
	}
	if p, _ := m.plan(889); p.Lock.State != LockActive {
		t.Fatalf("lock before the last milestone: %s", p.Lock.State)
	}
	code, out, errOut := m.next(889)
	p, raw := m.plan(889)
	if code != 0 || !strings.Contains(out, "The mission of #889 is done") || p.Lock.State != LockEnded || p.Lock.EndedAt == nil ||
		p.Lock.EndReason == nil || *p.Lock.EndReason != "MS3, the last milestone of the last mission, is done" {
		t.Fatalf("mission next 889: exit %d: %s%s\n%s", code, out, errOut, raw)
	}
	if _, err := time.Parse(time.RFC3339, *p.Lock.EndedAt); err != nil {
		t.Fatalf("ended_at: %v", err)
	}
	_, c := m.client(m.agent)
	if err := editBody(c, 20, "### Summary\n\nAfter the mission."); err != nil {
		t.Fatalf("#20's body after the lock ended: %v", err)
	}
	// A plan with no milestones ends its lock too, and says why.
	m = startedLock(t)
	m.setPlan(889, func(p *MissionPlan) { p.Milestones = nil })
	for n := 20; n <= 27; n++ {
		m.closeIssue(n)
	}
	code, out, errOut = m.next(889)
	if p, raw := m.plan(889); code != 0 || p.Lock.State != LockEnded || p.Lock.EndReason == nil || *p.Lock.EndReason != "every story and Bug of the mission is merged" {
		t.Fatalf("mission next 889 with no milestones: exit %d: %s%s\n%s", code, out, errOut, raw)
	}
}

// Criterion 8: mission 2 of #889 starts after mission 1 with no new start
// card: the lock he confirmed at mission 1 lists every issue of #889,
// mission 2's too, and stays active.
func TestLockSpansMissions(t *testing.T) {
	m := newMissionWorld(t)
	m.scenario889(map[int]string{22: "depends-on #950"})
	m.mu.Lock()
	m.tool = "amp"
	m.mu.Unlock()
	if code, out, errOut := m.approve(889, "--milestones", "MS1=#20,#21 MS2=#22,#23"); code != 0 {
		t.Fatalf("approve: exit %d: %s%s", code, out, errOut)
	}
	if code, out, errOut := m.mission(m.person, "rahulnakmol", "start", 889); code != 0 {
		t.Fatalf("start: exit %d: %s%s", code, out, errOut)
	}
	p, _ := m.plan(889)
	if len(p.Missions) != 2 || p.Lock == nil {
		t.Fatalf("plan: %+v", p)
	}
	var nums []int
	for _, li := range p.Lock.Issues {
		nums = append(nums, li.Number)
	}
	if !slices.Equal(nums, []int{889, 20, 21, 22, 23}) {
		t.Fatalf("locked issues %v: #22 waits for mission 2 and is locked too", nums)
	}
	for _, n := range []int{20, 21, 23, 950} { // mission 1 ends, and #22's depends-on merges: mission 2 starts
		m.closeIssue(n)
	}
	m.status(22, "ready", "")
	code, out, errOut := m.next(889)
	again, raw := m.plan(889)
	if code != 0 || out != "start #22\n" || again.Lock.State != LockActive || fmt.Sprint(again.Lock.Issues) != fmt.Sprint(p.Lock.Issues) ||
		again.Lock.StartedAt != p.Lock.StartedAt || len(again.Controls) != 1 {
		t.Fatalf("mission next in mission 2: exit %d: %q%s\n%s", code, out, errOut, raw)
	}
	if code, out, _ := run(t, m.person, m.opts, "mission", "show", "889", "--repo", "o/r"); code != 0 || !strings.Contains(out, "mission 2 running: started under approval 1 on amp, with no new Approve to build") {
		t.Fatalf("mission show: %s", out)
	}
	code, out, errOut = run(t, m.person, m.opts, "inbox", "--list", "--json", "--repo", "o/r")
	var list inboxList
	if code != 0 || json.Unmarshal([]byte(out), &list) != nil || slices.ContainsFunc(list.Items, func(it InboxItem) bool { return it.Kind == InboxStart }) {
		t.Fatalf("a start card for mission 2: exit %d: %s%s", code, out, errOut)
	}
}

// Criterion 9: @rahulnakmol picks 1 Keep the edit: after the presence check
// the lock takes #20's new body hash, controls record his keep, and aayu
// mission next lists #20 by the usual rules again.
func TestKeepTheEdit(t *testing.T) {
	m := startedLock(t)
	m.g.Mu.Lock()
	edited := m.g.Issues[20].Body + "\nA person's note in the body."
	m.g.Issues[20].Body, m.g.Issues[20].EditedBy = edited, "alice"
	m.g.Mu.Unlock()
	if code, out, errOut := m.next(889); code != 0 || !strings.Contains(out, "waits #20: changed while locked\n") {
		t.Fatalf("mission next before the keep: exit %d: %s%s", code, out, errOut)
	}
	m.device("rahulnakmol")
	code, out, errOut := runIn(t, m.person, m.opts, "1y", "inbox", "--repo", "o/r")
	if code != 0 || m.codes() != 1 || !strings.Contains(out, "done: kept the edit of #20; the lock of #889 holds its new body") {
		t.Fatalf("inbox 1 Keep the edit: exit %d, device codes %d:\n%s%s", code, m.codes(), out, errOut)
	}
	p, raw := m.plan(889)
	ctl := p.Controls[len(p.Controls)-1]
	if p.Lock.Issues[1].Number != 20 || p.Lock.Issues[1].BodySHA256 != workitems.BodyHash(edited) || p.Lock.State != LockActive ||
		ctl.Action != "keep the edit of #20" || ctl.By != "rahulnakmol" || ctl.Presence != "device-flow" || ctl.At == "" {
		t.Fatalf("plan.json after the keep:\n%s", raw)
	}
	code, out, errOut = m.next(889)
	if code != 0 || strings.Contains(out, "changed while locked") || !strings.Contains(out, "#20") {
		t.Fatalf("mission next after the keep: exit %d: %s%s", code, out, errOut)
	}
	// The card is gone: nothing changed while locked any more.
	code, out, _ = run(t, m.person, m.opts, "inbox", "--list", "--json", "--repo", "o/r")
	var list inboxList
	if code != 0 || json.Unmarshal([]byte(out), &list) != nil || slices.ContainsFunc(list.Items, func(it InboxItem) bool { return it.Kind == InboxLocked }) {
		t.Fatalf("inbox after the keep: %s", out)
	}
}

// Criterion 4, failing closed: when aayu cannot read the locks, it writes no
// body, not even an unlocked one; a comment still goes.
func TestLockGuardFailsClosed(t *testing.T) {
	m := startedLock(t)
	putLedger(t, m.bare, "missions/900/plan.json", "not a plan\n")
	_, c := m.client(m.agent)
	m.g.Reset()
	err := editBody(c, 870, "### Summary\n\nThe Initiative.")
	if err == nil || CodeOf(Wrap(ExitBlocked, err)) != ExitExternal || !strings.Contains(err.Error(), "aayu reads the mission locks before it writes") ||
		!strings.Contains(err.Error(), "missions/900/plan.json: not a mission plan aayu wrote") {
		t.Fatalf("a body write with unreadable locks: %v", err)
	}
	if err := postComment(context.Background(), c, "o/r", 870, "github.comment", "A note."); err != nil {
		t.Fatalf("a comment with unreadable locks: %v", err)
	}
	if w := m.issueWrites(870); len(w) != 0 {
		t.Fatalf("the body write reached GitHub: %v", w)
	}
	// ship completes a story after the merge: its Outcome goes in a comment, so
	// ship still writes its record.
	e, c := m.client(m.agent)
	rec := ShipRecord{PR: 99, MergeCommit: strings.Repeat("a", 40), Head: strings.Repeat("b", 40), Pack: "proof/21/pack.json", ShippedAt: "2026-10-26T10:00:00Z",
		HumanPass: HumanPass{Kind: "proof", Note: "on proof"}, Actions: "actions/2026-10-26.jsonl"}
	if err := e.completeStory(context.Background(), c, "o/r", 21, rec); err != nil {
		t.Fatalf("ship's completion of #21 with unreadable locks: %v", err)
	}
	m.g.Mu.Lock()
	last := m.g.Issues[21].Comments[len(m.g.Issues[21].Comments)-1].Body
	m.g.Mu.Unlock()
	if !strings.HasPrefix(last, "### Outcome\n\nMerged in #99") || slices.ContainsFunc(m.issueWrites(21), func(r fake.Request) bool { return r.Method == http.MethodPatch }) {
		t.Fatalf("ship's completion of #21 with unreadable locks: last comment %q, writes %v", last, m.issueWrites(21))
	}
}

// Criterion 2: Start locks the issues the card named. When one closes while
// he enters the code, nothing starts.
func TestStartRefusesWhenLockedIssuesChange(t *testing.T) {
	m := lockWorld(t, "amp")
	m.device("rahulnakmol")
	m.edit(func() { m.g.Device.OnPoll = func(int) { m.g.Issues[27].State = "closed" } })
	code, out, errOut := run(t, m.person, m.opts, "mission", "start", "889", "--repo", "o/r")
	if code != ExitBlocked || !strings.Contains(errOut, "The issues the start of #889 locks changed while the code was waiting: now #889, #20, #21, #22, #23, #24, #25, #26. Nothing was started.") {
		t.Fatalf("start 889: exit %d: %s%s", code, out, errOut)
	}
	if p, raw := m.plan(889); p.StartedBy != "" || p.Lock != nil {
		t.Fatalf("start wrote:\n%s", raw)
	}
	// When GitHub fails as Start reads them again, nothing starts either.
	m = lockWorld(t, "amp")
	m.device("rahulnakmol")
	m.edit(func() { m.g.Device.OnPoll = func(int) { m.g.Fail, m.g.FailAfter = "GET /repos/o/r/issues/27", 0 } })
	if code, out, errOut := run(t, m.person, m.opts, "mission", "start", "889", "--repo", "o/r"); code != ExitExternal {
		t.Fatalf("start 889 with GitHub failing: exit %d: %s%s", code, out, errOut)
	}
	if p, raw := m.plan(889); p.StartedBy != "" || p.Lock != nil {
		t.Fatalf("start wrote:\n%s", raw)
	}
}

// Criterion 5: a wake that cannot check a changed body starts nothing, so no
// story starts from a spec that moved. Once it can, the comment names no
// editor GitHub did not give.
func TestLockedEditCheckFailsClosed(t *testing.T) {
	m := startedLock(t)
	m.edit(func() { m.g.Issues[20].Body += "\nA person's note." })
	// Each read and write of the check fails once, right after the request before it.
	for _, c := range []struct{ after, fail string }{
		{"GET /repos/o/r/issues/889/sub_issues", "GET /repos/o/r/issues/889"},
		{"", "GET /repos/o/r/issues/20/comments"},
		{"GET /repos/o/r/issues/20/comments", "POST /graphql"},
		{"", "POST /repos/o/r/issues/20/comments"},
	} {
		m.edit(func() {
			m.g.Fail, m.g.FailAfter = "", 0
			if c.after == "" {
				m.g.Fail = c.fail
			}
			m.g.Hook = func(method, path string) {
				if method+" "+path == c.after {
					m.g.Fail, m.g.Hook = c.fail, nil
				}
			}
		})
		if code, out, errOut := m.next(889); code != ExitExternal || strings.Contains(out, "start #") || len(m.claims(889)) != 0 {
			t.Errorf("mission next with %s failing: exit %d, claims %v: %s%s", c.fail, code, m.claims(889), out, errOut)
		}
		m.edit(func() { m.g.Fail, m.g.Hook = "", nil })
	}
	if code, out, errOut := m.next(889); code != 0 || !strings.Contains(out, "waits #20: changed while locked\n") {
		t.Fatalf("mission next: exit %d: %s%s", code, out, errOut)
	}
	m.g.Mu.Lock()
	last := m.g.Issues[20].Comments[len(m.g.Issues[20].Comments)-1].Body
	m.g.Mu.Unlock()
	if !strings.Contains(last, "holds it: its body, edited at ") || strings.Contains(last, " by @") {
		t.Fatalf("lock comment: %q", last)
	}
}

// Criterion 9: Keep the edit keeps only the body the card showed, and only
// while the lock holds it.
func TestKeepTheEditRefusals(t *testing.T) {
	m := startedLock(t)
	m.edit(func() { m.g.Issues[20].Body += "\nA person's note." })
	e, c := m.client(m.person)
	codes := m.codes()
	err := e.keepEdit(context.Background(), c, "o/r", InboxItem{Feature: 889, Number: 20, BodyHash: workitems.BodyHash("an earlier edit")})
	if err == nil || CodeOf(err) != ExitBlocked || err.Error() != "#20's body changed again since the card was read. Nothing was kept." || m.codes() != codes {
		t.Fatalf("keep a stale card: %v, device codes %d", err, m.codes()-codes)
	}
	// The body changes again while he enters the code: nothing is kept.
	m.device("rahulnakmol")
	m.edit(func() {
		m.g.Device.OnPoll = func(int) { m.g.Issues[20].Body = strings.TrimSuffix(m.g.Issues[20].Body, "\n") + "\nA second note." }
	})
	code, out, errOut := runIn(t, m.person, m.opts, "1y", "inbox", "--repo", "o/r")
	if p, raw := m.plan(889); !strings.Contains(out+errOut, "#20's body changed again since the card was read. Nothing was kept.") ||
		slices.ContainsFunc(p.Controls, func(c MissionControl) bool { return strings.HasPrefix(c.Action, "keep the edit") }) {
		t.Fatalf("keep while the body moved: exit %d:\n%s%s\n%s", code, out, errOut, raw)
	}
	// A dry run says what the keep would do, and keeps nothing.
	m.device("rahulnakmol")
	code, out, errOut = runIn(t, m.person, m.opts, "1y", "inbox", "--dry-run", "--repo", "o/r")
	if p, raw := m.plan(889); !strings.Contains(out, "--dry-run: the lock of #889 would take #20's new body") || p.Controls[len(p.Controls)-1].Action != "start" {
		t.Fatalf("keep in a dry run: exit %d:\n%s%s\n%s", code, out, errOut, raw)
	}
	// The mission ends while he enters the code: nothing is kept.
	m.device("rahulnakmol")
	m.edit(func() {
		m.g.Device.OnPoll = func(int) {
			if p, _ := m.plan(889); p.Lock.State == LockActive {
				m.setPlan(889, func(p *MissionPlan) { p.Lock.State = LockEnded })
			}
		}
	})
	code, out, errOut = runIn(t, m.person, m.opts, "1y", "inbox", "--repo", "o/r")
	if !strings.Contains(out+errOut, "#20 is not locked by the mission of #889 any more. Nothing was kept.") {
		t.Fatalf("keep after the lock ended: exit %d:\n%s%s", code, out, errOut)
	}
	if p, raw := m.plan(889); slices.ContainsFunc(p.Controls, func(c MissionControl) bool { return strings.HasPrefix(c.Action, "keep the edit") }) {
		t.Fatalf("a keep was recorded:\n%s", raw)
	}
}

// Criterion 5: the lock comment names what changed: criteria by number,
// other sections by label, and else the body.
func TestBodyChangeNamesWhatChanged(t *testing.T) {
	was := "### Summary\n\nPull the next story.\n\n### Completion Goal\n\n" +
		"1. [api-trace] state: GET /a; expected: 200\n2. [api-trace] state: GET /b; expected: 200\n3. [api-trace] state: GET /c; expected: 200\n"
	for _, c := range []struct{ now, want string }{
		{strings.Replace(was, "/c;", "/d;", 1), "criterion 3"},
		{strings.Replace(strings.Replace(was, "/a;", "/x;", 1), "/c;", "/d;", 1), "criteria 1 and 3"},
		{strings.Replace(was, "Pull the next", "Push the next", 1), "its Summary section"},
		{strings.Replace(strings.Replace(was, "Pull the next", "Push the next", 1), "/b;", "/x;", 1), "its Summary section and criterion 2"},
		{"A note above the sections.\n\n" + was, "its body"},
	} {
		if got := bodyChange(was, c.now); got != c.want {
			t.Errorf("bodyChange: %q, want %q, for:\n%s", got, c.want, c.now)
		}
	}
}

// Criterion 1 in each inbox: the full-screen inbox keeps the spacing of the
// start card's lines that fit and wraps the rest, and Not now writes
// nothing; a narrow terminal and --list show the same lines; with no tool
// set, the card says Start refuses.
func TestStartCardInEveryInbox(t *testing.T) {
	m := lockWorld(t, "cursor")
	var wide, narrow, status string
	opts := m.opts
	opts.TUI = func(tm tea.Model) error {
		im := tm.(*inboxModel)
		teaDrive(im, im.Init())
		im.cur = slices.IndexFunc(im.rows, func(i int) bool { return im.items[i].Kind == InboxStart })
		im.Update(tea.WindowSizeMsg{Width: 176, Height: 40})
		wide = ansi.Strip(im.View().Content)
		im.Update(tea.WindowSizeMsg{Width: 70, Height: 40})
		narrow = ansi.Strip(im.View().Content)
		teaKeys(im, "2", "enter")
		status = im.status
		teaKeys(im, "q")
		return nil
	}
	m.device("rahulnakmol")
	n := m.commits()
	m.g.Reset()
	if code, out, errOut := runIn(t, m.person, opts, "", "inbox", "--repo", "o/r"); code != 0 {
		t.Fatalf("full-screen inbox: exit %d: %s%s", code, out, errOut)
	}
	// The spacing of a line that fits stays, so Locked, Open, and Ends line up; a long line wraps.
	held := []string{"│ Locked  each body, and its effort:, risk: and priority: labels", "│ Open    comments and status: labels",
		"│ Ends    at the last milestone of the last mission (MS3)", "│ Starting the mission makes 9 issues read-only until its last "}
	for _, l := range held {
		if !strings.Contains(wide, l) || !strings.Contains(narrow, strings.TrimPrefix(l, "│ ")) {
			t.Errorf("the full-screen card lacks %q:\n%s\n%s", l, wide, narrow)
		}
	}
	if status != "Not now: nothing is locked: nothing was written, and the card stays in the inbox." || m.commits() != n || len(m.g.Writes()) != 0 {
		t.Fatalf("Not now: %q, writes %v", status, m.g.Writes())
	}
	m.person["COLUMNS"] = "70"
	code, out, errOut := runIn(t, m.person, m.opts, "q", "inbox", "--repo", "o/r")
	if code != 0 || !strings.Contains(out, "\n  Starting the mission makes 9 issues read-only until its last\n  milestone: #889,") ||
		!strings.Contains(out, "\n  Locked  each body, and its effort:, risk: and priority: labels\n  Open    comments and status: labels\n") {
		t.Fatalf("inbox in 70 columns: exit %d:\n%s%s", code, out, errOut)
	}
	code, out, errOut = run(t, m.agent, m.opts, "inbox", "--list", "--repo", "o/r")
	if code != 0 || !strings.Contains(out, "_A Feature is approved to build; starting its mission locks its issues until the last milestone._  \n"+
		"Starting the mission makes 9 issues read-only until its last milestone: \\#889, \\#20, \\#21, \\#22, \\#23, \\#24, \\#25, \\#26, \\#27  \nLocked  each body") {
		t.Fatalf("inbox --list: exit %d:\n%s%s", code, out, errOut)
	}
	m.setTool("")
	code, out, errOut = run(t, m.person, m.opts, "inbox", "--list", "--json", "--repo", "o/r")
	var list inboxList
	if code != 0 || json.Unmarshal([]byte(out), &list) != nil || len(list.Items) != 1 ||
		!slices.Contains(list.Items[0].Lines, "It runs in the tool you set for this repository, and none is set there yet: Start refuses until you set one.") {
		t.Fatalf("inbox --list with no tool: exit %d: %s%s", code, out, errOut)
	}
	// A closed Feature has no card.
	m.closeIssue(889)
	if code, out, errOut = run(t, m.person, m.opts, "inbox", "--list", "--json", "--repo", "o/r"); code != 0 || json.Unmarshal([]byte(out), &list) != nil || len(list.Items) != 0 {
		t.Fatalf("inbox --list with #889 closed: exit %d: %s%s", code, out, errOut)
	}
}

// A mission started before its plan held a lock has none: mission next lists
// its stories as before, and posts no lock comment.
func TestMissionWithNoLock(t *testing.T) {
	m := startedLock(t)
	m.setPlan(889, func(p *MissionPlan) { p.Lock = nil })
	m.edit(func() { m.g.Issues[20].Body += "\nA person's note." })
	code, out, errOut := m.next(889)
	if code != 0 || strings.Contains(out, "changed while locked") || !strings.Contains(out, "#20") {
		t.Fatalf("mission next: exit %d: %s%s", code, out, errOut)
	}
	m.g.Mu.Lock()
	defer m.g.Mu.Unlock()
	if slices.ContainsFunc(m.g.Issues[20].Comments, func(c *fake.Comment) bool { return strings.HasPrefix(c.Body, LockMarker) }) {
		t.Fatal("a lock comment on #20 with no lock")
	}
}
