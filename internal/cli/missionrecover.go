package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/delegate"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/ready"
)

// `aayu mission recover <f>` and `aayu mission retry <f> --story <n>` (Story
// #1003; Decision #915's controls, R9). Recover reads the record (each
// delegate/<n>/state.json and missions/<f>/plan.json) and each host, never
// the Kanha's memory: a story whose thread lives is re-attached to the
// mission's Kanha, a story with a PR keeps it, and a story whose agent is
// gone with no PR is restarted from its branch and brief, under a claim line,
// so no story has two live agents. A host it cannot read marks its stories
// unreadable and keeps that source's cursor; a start that got no answer is
// "maybe started" until the host and GitHub show it did not start. A failed
// step retries after [mission] retry_backoff_minutes, each later retry after
// twice the wait before, up to [mission] retry_max retries; then it waits for
// the person who started the mission, and `aayu mission retry` runs one more
// attempt. A run that changes nothing writes nothing. Both run only on the
// mission's recorded tool; recover also runs on each Kanha wake, and retry is
// a person's decision.

// Story states in the plan.
const (
	stateMaybe      = "maybe started"
	stateUnreadable = "unreadable: "
	stateWaits      = "waits on @"
)

// MissionStoryState is what recover and retry found and did for one story or
// Bug: its state, when its next retry runs, and each attempt of its failed
// step, counted and costed.
type MissionStoryState struct {
	Number    int              `json:"number"`
	State     string           `json:"state,omitempty"` // maybe started, unreadable: <source> (<code>), failed, or waits on @<login>
	RetryAt   string           `json:"retry_at,omitempty"`
	Attempts  []MissionAttempt `json:"attempts,omitempty"`
	CostBasis string           `json:"cost_basis,omitempty"`
}

// MissionAttempt is one attempt of a story's step: the start that failed,
// and each restart and retry after it.
type MissionAttempt struct {
	Kind    string  `json:"kind"` // start, run, restart, or retry
	At      string  `json:"at"`
	By      string  `json:"by,omitempty"`
	Error   string  `json:"error,omitempty"`
	CostUSD float64 `json:"cost_usd"`
}

// MissionSource is a host recover reads: the codex task list, cursor agents,
// amp threads, or claude sessions. Its cursor is the last read that told
// recover something new; a read that fails keeps it.
type MissionSource struct {
	Name       string `json:"name"`
	Cursor     string `json:"cursor,omitempty"`
	Unreadable string `json:"unreadable,omitempty"` // why the last read failed: "401"
	Since      string `json:"since,omitempty"`
}

// sourceNames are the receivers' sources, as recover names them.
var sourceNames = map[string]string{delegate.Codex: "codex task list", delegate.Cursor: "cursor agents", delegate.Amp: "amp threads", delegate.Claude: "claude sessions"}

// stateOf is story n's state in the plan, or nil.
func (p *MissionPlan) stateOf(n int) *MissionStoryState {
	if i := slices.IndexFunc(p.StoryStates, func(s MissionStoryState) bool { return s.Number == n }); i >= 0 {
		return &p.StoryStates[i]
	}
	return nil
}

// storyState is story n's state in the plan, added when it has none.
func (p *MissionPlan) storyState(n int) *MissionStoryState {
	if s := p.stateOf(n); s != nil {
		return s
	}
	p.StoryStates = append(p.StoryStates, MissionStoryState{Number: n})
	return &p.StoryStates[len(p.StoryStates)-1]
}

func (s *MissionStoryState) retries() int {
	return len(slices.DeleteFunc(slices.Clone(s.Attempts), func(a MissionAttempt) bool { return a.Kind != "retry" }))
}

func (s *MissionStoryState) cost() float64 {
	var usd float64
	for _, a := range s.Attempts {
		usd += a.CostUSD
	}
	return usd
}

// summary is the state as mission show prints it.
func (s *MissionStoryState) summary() string {
	if len(s.Attempts) == 0 {
		return s.State
	}
	return fmt.Sprintf("%s (%d attempts, %s)", s.State, len(s.Attempts), ready.USD(s.cost()))
}

// errors lists each failed attempt's error: "1. …; 2. …".
func (s *MissionStoryState) errors() string {
	var out []string
	for _, a := range s.Attempts {
		if a.Error != "" {
			out = append(out, fmt.Sprintf("%d. %s (%s)", len(out)+1, a.Error, a.Kind))
		}
	}
	return strings.Join(out, "; ")
}

func (s MissionSource) summary() string {
	cursor := "no cursor yet"
	if s.Cursor != "" {
		cursor = "cursor " + s.Cursor
	}
	if s.Unreadable != "" {
		return fmt.Sprintf("%s: unreadable (%s) since %s; %s kept", s.Name, s.Unreadable, s.Since, cursor)
	}
	return fmt.Sprintf("%s: quiet since its %s", s.Name, cursor)
}

// MissionRecoverReport is what mission recover and retry report: one line per story.
type MissionRecoverReport struct {
	Repo    string         `json:"repo"`
	Feature int            `json:"feature"`
	Control string         `json:"control"` // recover or retry
	Story   int            `json:"story,omitempty"`
	By      string         `json:"by"`
	Tool    string         `json:"tool"`
	Effect  []string       `json:"effect,omitempty"` // retry: what the person saw before the device code
	Stories []RecoverStory `json:"stories"`
	Changed bool           `json:"changed"`
	Ledger  string         `json:"ledger_path"`
	Commit  string         `json:"ledger_commit,omitempty"`
	DryRun  bool           `json:"dry_run,omitempty"`
}

// RecoverStory is one story's line: what recover found and did.
type RecoverStory struct {
	Story int    `json:"story"`
	State string `json:"state"` // re-attached, kept, restarted, unreadable, maybe started, failed, retried, waits, or left
	Line  string `json:"line"`
	Next  string `json:"next,omitempty"`
}

// recovery is one run of recover or retry over a mission's record.
type recovery struct {
	e        *Env
	c        *github.Client
	w        *ledger.Writer
	repo     string
	f        int
	plan     *MissionPlan // as read
	next     *MissionPlan // with this run's story states and sources
	appr     *ready.Approval
	tool, by string
	verb     string // recover or retry
	now      time.Time
	cfg      config.Delegate
	prs      map[int]int  // story: an open PR that refers to it
	open     map[int]bool // the open PRs
	lists    map[string]listRead
	reads    map[string]sourceRead
	touched  []int
	most     int           // [mission] retry_max
	backoff  time.Duration // [mission] retry_backoff_minutes: the first retry's wait
	changed  bool          // a delegation, a claim, or a hand-off was written
}

type listRead struct {
	tasks []delegate.Task
	err   error
}

type sourceRead struct {
	err  error
	news bool
}

var errClaimed = errors.New("a live claim of another run holds it, so this run starts nothing")

// newRecovery reads what recover and retry act on: the open PRs and the receivers' config.
func (e *Env) newRecovery(ctx context.Context, c *github.Client, w *ledger.Writer, repo string, f int, plan *MissionPlan, appr *ready.Approval, verb, tool, by string) (*recovery, error) {
	l, err := e.Config()
	if err != nil {
		return nil, err
	}
	var next MissionPlan
	_ = json.Unmarshal([]byte(missionJSON(plan)), &next)
	r := &recovery{e: e, c: c, w: w, repo: repo, f: f, plan: plan, next: &next, appr: appr, verb: verb, tool: tool, by: by, now: e.Now(), cfg: l.Config.Delegate,
		prs: map[int]int{}, open: map[int]bool{}, lists: map[string]listRead{}, reads: map[string]sourceRead{}}
	owner, name, _ := github.SplitRepo(repo)
	// The retry limits are the ones committed on the integration branch at its tip, as [build] agents is
	// (missionAgents): a local value may only lower retry_max and lengthen the backoff. A prompt never sets them.
	pol, _, err := e.loopPolicy(ctx, c, owner, name)
	if err != nil {
		return nil, err
	}
	cm, lm := pol.Config.Mission, l.Config.Mission
	r.most, r.backoff = min(lm.RetryMax, cm.RetryMax), time.Duration(max(lm.RetryBackoffMinutes, cm.RetryBackoffMinutes))*time.Minute
	prs, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.PullRequest, *gh.Response, error) {
		return c.REST.PullRequests.List(ctx, owner, name, &gh.PullRequestListOptions{State: "open", ListOptions: o})
	})
	if err != nil {
		return nil, Wrap(ExitExternal, err)
	}
	for _, pr := range prs {
		r.open[pr.GetNumber()] = true
		if n := StoryOf(pr.GetBody(), "", pr.GetHead().GetRef()); n != 0 {
			r.prs[n] = pr.GetNumber()
		}
	}
	return r, nil
}

// openPR is story n's open PR: its delegation's, or one that refers to it.
func (r *recovery) openPR(n int, st delegate.State) int {
	if pr := r.prs[n]; pr != 0 {
		return pr
	}
	if r.open[st.PR] {
		return st.PR
	}
	return 0
}

func (r *recovery) adapter(receiver string) delegate.Adapter {
	return r.e.receiverAdapter(r.cfg, receiver, r.c, r.repo, config.RepoRoot(r.e.Dir))
}

// list reads receiver's task list once per run.
func (r *recovery) list(ctx context.Context, receiver string) ([]delegate.Task, error) {
	if l, ok := r.lists[receiver]; ok {
		return l.tasks, l.err
	}
	tasks, err := r.adapter(receiver).(delegate.Lister).List(ctx)
	r.lists[receiver] = listRead{tasks, err}
	return tasks, err
}

// read asks st's receiver whether its task lives: lost says why it is gone,
// "" while it lives; err is a source recover could not read.
func (r *recovery) read(ctx context.Context, st delegate.State) (delegate.Task, string, error) {
	ad := r.adapter(st.Receiver)
	t := delegate.Task{ID: st.Task, URL: st.TaskURL, Status: delegate.TaskWorking}
	var err error
	switch st.Receiver {
	case delegate.Amp:
		archived, aerr := ad.(delegate.AmpCLI).Archived(ctx, st.Task)
		if archived {
			return t, "its Amp thread is archived", nil
		}
		return t, "", aerr
	case delegate.Codex:
		tasks, lerr := r.list(ctx, st.Receiver)
		if lerr != nil {
			return t, "", lerr
		}
		if i := slices.IndexFunc(tasks, func(x delegate.Task) bool { return x.ID == st.Task }); i >= 0 {
			t = tasks[i]
		} else if t, err = ad.Status(ctx, st); err != nil { // older than the list
			return t, "", err
		}
	default:
		if t, err = ad.Status(ctx, st); err != nil {
			return t, "", err
		}
	}
	if t.Status == delegate.TaskFailed {
		return t, fmt.Sprintf("its %s task ended (%s)", delegate.Names[st.Receiver], strings.TrimSpace(t.Detail)), nil
	}
	return t, "", nil
}

// note records a read of receiver's source: its error, or that it told recover something new.
func (r *recovery) note(receiver string, err error, news bool) {
	rd := r.reads[receiver]
	if rd.err == nil {
		rd.err = err
	}
	rd.news = rd.news || news
	r.reads[receiver] = rd
}

var httpCode = regexp.MustCompile(`\b([45]\d\d)\b`)

// unreadable is why a source could not be read: its HTTP status, else the error.
func unreadable(err error) string {
	if m := httpCode.FindStringSubmatch(err.Error()); m != nil {
		return m[1]
	}
	return clipText(oneLineErr(err), 80)
}

// lastFailure is the time and detail of the delegation's last failed event.
func lastFailure(st delegate.State) (string, string) {
	for i := len(st.Events) - 1; i >= 0; i-- {
		if st.Events[i].Status == dlgFailed {
			return st.Events[i].At, st.Events[i].Detail
		}
	}
	return "", ""
}

// per is the cost an attempt counts at, until #872 measures spend per story.
func (r *recovery) per() (float64, string) {
	if r.appr == nil { // Approve to build writes the plan and the approval together
		return 0, ""
	}
	return r.appr.Budget.PerStoryUSD, fmt.Sprintf("each attempt counts at the per-story cap of approval %d (%s) until #872 measures spend per story", r.appr.Version, ready.USD(r.appr.Budget.PerStoryUSD))
}

// touch marks story n's state as one this run writes.
func (r *recovery) touch(n int) *MissionStoryState {
	if !slices.Contains(r.touched, n) {
		r.touched = append(r.touched, n)
	}
	return r.next.storyState(n)
}

// clear drops story n's state when it has no attempts: nothing waits on it.
func (r *recovery) clear(n int) {
	if r.next.stateOf(n) == nil {
		return
	}
	ss := r.touch(n)
	ss.State, ss.RetryAt = "", ""
}

// counted adds the delegation's last failure to story n's attempts once.
func (r *recovery) counted(n int, st delegate.State) *MissionStoryState {
	ss := r.touch(n)
	at, why := lastFailure(st)
	if at != "" && !slices.ContainsFunc(ss.Attempts, func(a MissionAttempt) bool { return a.At == at }) {
		kind := "run"
		if strings.HasPrefix(why, "hand-off failed") {
			kind = "start"
		}
		per, basis := r.per()
		ss.Attempts, ss.CostBasis = append(ss.Attempts, MissionAttempt{Kind: kind, At: at, Error: why, CostUSD: per}), basis
		ss.State = "failed"
	}
	return ss
}

// schedule sets story n's state after a failed attempt: the next retry, or
// once retry_max retries failed, a wait on the person who started the mission.
func (r *recovery) schedule(ss *MissionStoryState) {
	most, backoff := r.most, r.backoff
	k := ss.retries()
	if k >= most {
		ss.State, ss.RetryAt = stateWaits+r.plan.starter(ss.Number), ""
		return
	}
	last := r.now // a failed delegation with no failed event aayu wrote: the backoff runs from now
	if len(ss.Attempts) > 0 {
		last, _ = time.Parse(time.RFC3339, ss.Attempts[len(ss.Attempts)-1].At)
	}
	ss.State, ss.RetryAt = "failed", last.Add(backoff<<k).UTC().Format(time.RFC3339)
}

// starter is who started story n: the mission's starter, or a Tatkal story's.
func (p *MissionPlan) starter(n int) string {
	if i := slices.IndexFunc(p.StoryStarts, func(s MissionStoryStart) bool { return s.Number == n }); i >= 0 {
		return p.StoryStarts[i].StartedBy
	}
	return p.StartedBy
}

// attempt runs story n's step once more: its sync when it has an open PR
// (which never runs a judged review again: reviewLoop requests a review only
// for a head with none), else a fresh hand-off on its branch with its brief.
// It records the attempt on story n's state and returns the delegation after.
func (r *recovery) attempt(ctx context.Context, n int, st delegate.State, pr int, kind, why string) (delegate.State, error) {
	tool := r.plan.storyTool(n)
	started := r.now.UTC().Format(time.RFC3339)
	if pr != 0 && (st.PR != pr || st.Status == dlgFailed || st.Status == dlgMaybe) { // the sync syncs this PR, and never hands off afresh
		if err := r.setDelegation(ctx, n, func(s *delegate.State) {
			s.PR, s.Status = pr, dlgPROpen
			s.Events = append(s.Events, delegate.Event{At: started, Status: dlgPROpen, Detail: fmt.Sprintf("PR #%d found by aayu mission %s", pr, r.verb)})
		}); err != nil {
			return st, err
		}
	}
	if pr == 0 {
		// The claim line orders the restarts of every run: a live claim of
		// another run for the story (a start or a restart) holds it, and this
		// run starts nothing. The line is released once the hand-off is on
		// the delegation, which guards the story from then on.
		path := MissionClaimsPath(r.f)
		tok, lost, err := r.e.claimSlot(ctx, r.w, path, fmt.Sprintf("missions: %s of #%d in the mission of #%d", kind, n, r.f), n, map[int]bool{}, math.MaxInt, nil)
		if err != nil {
			return st, err
		}
		r.changed = true
		if lost != 0 {
			return st, errClaimed
		}
		defer func() {
			b, _ := json.Marshal(loopClaim{Story: n, Token: tok, At: started, Release: true})
			_, _ = r.w.Write(ctx, ledger.Commit{Message: fmt.Sprintf("missions: the %s of #%d is on its delegation; its claim is released", kind, n), Append: map[string][]byte{path: append(b, '\n')}})
		}()
		if st.Status != dlgFailed { // build hands off afresh only after a failed one
			if err := r.setDelegation(ctx, n, func(s *delegate.State) {
				s.Status = dlgFailed
				s.Events = append(s.Events, delegate.Event{At: started, Status: dlgFailed, Detail: why + " (aayu mission " + r.verb + ")"})
			}); err != nil {
				return st, err
			}
		}
	}
	sub := *r.e
	sub.JSON, sub.restart = true, pr == 0
	var out strings.Builder
	sub.Out, sub.Err = &out, io.Discard
	berr := runBuild(ctx, &sub, Values{"issue": strconv.Itoa(n), "delegate": tool, "asked": fmt.Sprintf("aayu mission %s %d: %s of #%d (%s)", r.verb, r.f, kind, n, why)})
	r.changed = true
	var after delegate.State
	if _, err := readLedgerJSON(ctx, r.w, delegate.LedgerPath(n), &after); err != nil {
		return st, Wrap(ExitExternal, err)
	}
	per, basis := r.per()
	a := MissionAttempt{Kind: kind, At: r.e.Now().UTC().Format(time.RFC3339), By: r.by, CostUSD: per}
	ss := r.touch(n)
	ss.CostBasis = basis
	switch {
	case berr != nil:
		a.Error = clipText(oneLineErr(berr), 200)
		if at, _ := lastFailure(after); at >= started { // the failure build recorded, so recover counts it once
			a.At = at
		}
	case after.Status == dlgMaybe:
		a.Error = "no answer before the timeout"
	}
	ss.Attempts = append(ss.Attempts, a)
	switch {
	case after.Status == dlgMaybe:
		ss.State, ss.RetryAt = stateMaybe, ""
	case berr != nil:
		r.schedule(ss)
	default:
		ss.State, ss.RetryAt = "", ""
	}
	return after, berr
}

// setDelegation edits delegate/<n>/state.json at the ledger tip.
func (r *recovery) setDelegation(ctx context.Context, n int, edit func(*delegate.State)) error {
	for range 3 {
		t, err := r.w.Read(ctx)
		if err != nil {
			return Wrap(ExitExternal, err)
		}
		b, err := t.File(ctx, delegate.LedgerPath(n))
		tip := t.Tip
		t.Close()
		var st delegate.State
		if err != nil || json.Unmarshal(b, &st) != nil {
			return Errorf(ExitExternal, "ledger %s: not a delegation aayu wrote for #%d", delegate.LedgerPath(n), n)
		}
		edit(&st)
		data, _ := json.MarshalIndent(st, "", "  ")
		_, err = r.w.Write(ctx, ledger.Commit{Message: fmt.Sprintf("delegate: mission %s of #%d", r.verb, n), Files: map[string][]byte{delegate.LedgerPath(n): append(data, '\n')}, Base: tip})
		if !errors.Is(err, ledger.ErrMoved) {
			r.changed = r.changed || err == nil
			return Wrap(ExitExternal, err)
		}
	}
	return Errorf(ExitExternal, "the ledger kept moving while aayu wrote %s", delegate.LedgerPath(n))
}

// kanha is the mission's Kanha that Start recorded, or nil.
func (r *recovery) kanha() *ready.Parent {
	if r.appr == nil || r.appr.Parent == nil || r.appr.Parent.Session == "" {
		return nil
	}
	return r.appr.Parent
}

// story is what recover finds and does for story n, whose delegation is st.
func (r *recovery) story(ctx context.Context, n int, st delegate.State) RecoverStory {
	out := RecoverStory{Story: n}
	say := func(state, line string) RecoverStory { out.State, out.Line = state, line; return out }
	name, src := delegate.Names[st.Receiver], sourceNames[st.Receiver]
	pr := r.openPR(n, st)
	retryNext := fmt.Sprintf("aayu mission retry %d --story %d", r.f, n)
	switch {
	case r.plan.stopped(n) != nil:
		return say("left", fmt.Sprintf("stopped (%s): recover leaves it until a person resumes it", r.plan.stopped(n).Reason))
	case r.plan.storyTool(n) != r.tool:
		return say("left", fmt.Sprintf("runs on %s, the tool of @%s, who started it: recover it there", r.plan.storyTool(n), r.plan.starter(n)))
	case st.Status == dlgCapped:
		return say("left", "capped: a person takes over, and recover starts nothing for it")
	case pr != 0:
		if st.PR != pr || st.Status == dlgMaybe {
			if r.c.DryRun() {
				return say("kept", fmt.Sprintf("kept on its PR #%d (--dry-run: the delegation is not updated)", pr))
			}
			if err := r.setDelegation(ctx, n, func(s *delegate.State) {
				s.PR, s.Status = pr, dlgPROpen
				s.Events = append(s.Events, delegate.Event{At: r.now.UTC().Format(time.RFC3339), Status: dlgPROpen, Detail: fmt.Sprintf("PR #%d found by aayu mission recover", pr)})
			}); err != nil {
				return say("failed", "its PR could not be recorded: "+oneLineErr(err))
			}
		}
		r.clear(n)
		return say("kept", fmt.Sprintf("kept on its PR #%d", pr))
	case st.PR != 0:
		return say("left", fmt.Sprintf("its PR #%d is not open: recover starts nothing for it", st.PR))
	case st.Mode == "local":
		return say("left", fmt.Sprintf("runs %s on its builder's machine: recover reads no host for it", name))
	case st.Status == dlgMaybe:
		if _, ok := r.adapter(st.Receiver).(delegate.Lister); !ok {
			r.touch(n).State = stateMaybe
			out.Next = retryNext
			return say("maybe started", fmt.Sprintf("maybe started: %s lists no tasks, so aayu cannot see whether #%d started there; a person checks %s, and aayu does not start it again", name, n, name))
		}
		tasks, err := r.list(ctx, st.Receiver)
		r.note(st.Receiver, err, false)
		if err != nil {
			r.touch(n).State = fmt.Sprintf("%s%s (%s)", stateUnreadable, src, unreadable(err))
			return say("unreadable", r.next.stateOf(n).State+": aayu starts nothing for it")
		}
		if slices.ContainsFunc(tasks, func(t delegate.Task) bool { return delegate.ForStory(t.Title, n) }) {
			r.touch(n).State = stateMaybe
			return say("maybe started", fmt.Sprintf("maybe started: a %s task exists for #%d but no PR yet; aayu does not start it again", st.Receiver, n))
		}
		r.note(st.Receiver, nil, true)
		return r.restart(ctx, n, st, fmt.Sprintf("no %s task and no PR for #%d", st.Receiver, n), "restart")
	case st.Status == dlgFailed:
		_, why := lastFailure(st)
		ss := r.counted(n, st)
		r.schedule(ss) // from the attempts alone, so a second run finds the same
		most := r.most
		if strings.HasPrefix(ss.State, stateWaits) {
			out.Next = retryNext
			return say("waits", fmt.Sprintf("%s: %d attempts failed: %s", ss.State, len(ss.Attempts), ss.errors()))
		}
		at, _ := time.Parse(time.RFC3339, ss.RetryAt)
		if r.now.Before(at) {
			return say("failed", fmt.Sprintf("failed (%s); retry %d of %d at %s", why, ss.retries()+1, most, at.Local().Format("15:04")))
		}
		k := ss.retries() + 1
		got := r.restart(ctx, n, st, why, "retry")
		got.Line = fmt.Sprintf("retried (%d of %d): %s", k, most, got.Line)
		return got
	}
	// A running story: its host says whether its agent lives.
	t, lost, err := r.read(ctx, st)
	r.note(st.Receiver, err, lost != "")
	switch {
	case err != nil:
		r.touch(n).State = fmt.Sprintf("%s%s (%s)", stateUnreadable, src, unreadable(err))
		return say("unreadable", r.next.stateOf(n).State+": aayu starts nothing for it")
	case lost != "":
		return r.restart(ctx, n, st, lost+" with no PR", "restart")
	}
	r.clear(n)
	where := or(t.URL, or(st.TaskURL, st.Task))
	if p := r.kanha(); p != nil && st.Kanha != p.Session {
		if r.c.DryRun() {
			return say("re-attached", fmt.Sprintf("re-attached: its %s thread lives (%s); --dry-run: its Kanha is not updated", name, where))
		}
		if err := r.setDelegation(ctx, n, func(s *delegate.State) {
			s.Kanha, s.KanhaURL = p.Session, p.URL
			s.Events = append(s.Events, delegate.Event{At: r.now.UTC().Format(time.RFC3339), Status: s.Status, Detail: "re-attached to the Kanha " + or(p.URL, p.Session) + " by aayu mission recover"})
		}); err != nil {
			return say("failed", "it lives, and its re-attach was not recorded: "+oneLineErr(err))
		}
		return say("re-attached", fmt.Sprintf("re-attached: its %s thread lives (%s), under the Kanha %s", name, where, or(p.URL, p.Session)))
	}
	return say("re-attached", fmt.Sprintf("re-attached: its %s thread lives (%s)", name, where))
}

// restart hands story n off again and says how it went.
func (r *recovery) restart(ctx context.Context, n int, st delegate.State, why, kind string) RecoverStory {
	out := RecoverStory{Story: n, State: "restarted"}
	if kind == "retry" {
		out.State = "retried"
	}
	branch := or(st.Branch, "a new branch")
	if r.c.DryRun() {
		out.Line = fmt.Sprintf("%s: --dry-run: would restart it from %s and its brief", why, branch)
		return out
	}
	after, err := r.attempt(ctx, n, st, 0, kind, why)
	ss := r.next.stateOf(n)
	switch {
	case errors.Is(err, errClaimed):
		out.State, out.Line = "left", why+": "+err.Error()
	case after.Status == dlgMaybe:
		out.State, out.Line = "maybe started", fmt.Sprintf("%s; restarted, and %s gave no answer before the timeout: maybe started, and aayu does not start it again", why, delegate.Names[st.Receiver])
	case err != nil && strings.HasPrefix(ss.State, stateWaits):
		out.State, out.Next = "waits", fmt.Sprintf("aayu mission retry %d --story %d", r.f, n)
		out.Line = fmt.Sprintf("%s; the restart failed: %s; %s: %d attempts failed: %s", why, oneLineErr(err), ss.State, len(ss.Attempts), ss.errors())
	case err != nil:
		at, _ := time.Parse(time.RFC3339, ss.RetryAt)
		most := r.most
		out.State = "failed"
		out.Line = fmt.Sprintf("%s; the restart failed: %s; retry %d of %d at %s", why, oneLineErr(err), ss.retries()+1, most, at.Local().Format("15:04"))
	default:
		out.Line = fmt.Sprintf("restarted from %s and its brief: %s (%s)", after.Branch, why, or(after.TaskURL, after.Task))
	}
	return out
}

// write records this run's story states and sources on the plan, with the
// control, when the run changed anything; a run that changed nothing writes nothing.
func (r *recovery) write(ctx context.Context, ctl MissionControl, msg string) (string, bool, error) {
	at := r.now.UTC().Format(time.RFC3339)
	for receiver, rd := range r.reads {
		name := sourceNames[receiver]
		i := slices.IndexFunc(r.next.Sources, func(s MissionSource) bool { return s.Name == name })
		switch {
		case rd.err != nil && i < 0:
			r.next.Sources = append(r.next.Sources, MissionSource{Name: name, Unreadable: unreadable(rd.err), Since: at})
		case rd.err != nil:
			s := &r.next.Sources[i]
			if s.Unreadable == "" {
				s.Since = at
			}
			s.Unreadable = unreadable(rd.err) // the cursor stays where the last good read left it
		case i >= 0 && (rd.news || r.next.Sources[i].Unreadable != ""):
			r.next.Sources[i] = MissionSource{Name: name, Cursor: at}
		case i < 0 && rd.news:
			r.next.Sources = append(r.next.Sources, MissionSource{Name: name, Cursor: at})
		}
	}
	slices.SortFunc(r.next.Sources, func(a, b MissionSource) int { return strings.Compare(a.Name, b.Name) })
	r.next.StoryStates = slices.DeleteFunc(r.next.StoryStates, func(s MissionStoryState) bool { return s.State == "" && len(s.Attempts) == 0 })
	was, now := missionJSON(r.plan), missionJSON(r.next)
	if was == now && !r.changed {
		return "", false, nil
	}
	if r.c.DryRun() {
		return "", true, nil
	}
	touched := map[int]*MissionStoryState{}
	for _, n := range r.touched {
		touched[n] = r.next.stateOf(n)
	}
	commit, err := writePlan(ctx, r.w, r.f, msg, func(pl *MissionPlan) error {
		for n, ss := range touched {
			pl.StoryStates = slices.DeleteFunc(pl.StoryStates, func(s MissionStoryState) bool { return s.Number == n })
			if ss != nil {
				pl.StoryStates = append(pl.StoryStates, *ss)
			}
		}
		slices.SortFunc(pl.StoryStates, func(a, b MissionStoryState) int { return a.Number - b.Number })
		pl.Sources = r.next.Sources
		pl.Controls = append(pl.Controls, ctl)
		return nil
	})
	return commit, true, err
}

// missionRecover is `aayu mission recover <f>`.
func missionRecover(ctx context.Context, e *Env, f int) error {
	if _, err := os.Stat(e.PauseFile()); err == nil {
		return Errorf(ExitBudget, "mission recover: agent lanes are paused by the [budget] breaker (%s): nothing starts; a human clears it with `aayu resume`", e.PauseFile())
	}
	repo, c, w, err := e.missionLedger(ctx)
	if err != nil {
		return err
	}
	plan, appr, _, err := readMission(ctx, w, f)
	if err != nil {
		return err
	}
	if err := controllable(plan, f); err != nil {
		return err
	}
	tool, err := e.callerTool(ctx, repo)
	if err != nil {
		return err
	}
	switch {
	case tool != plan.Tool:
		return Errorf(ExitBlocked, "#%d's mission runs on %s, the tool of @%s, who started it: run this there", f, plan.Tool, plan.StartedBy)
	case plan.Pause != nil:
		return WithNext(Errorf(ExitBlocked, "The mission of #%d is paused: %s. Nothing starts.", f, plan.Pause.Reason), fmt.Sprintf("aayu mission resume %d", f))
	}
	me, _, err := c.REST.Users.Get(ctx, "")
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	r, err := e.newRecovery(ctx, c, w, repo, f, plan, appr, "recover", tool, me.GetLogin())
	if err != nil {
		return err
	}
	subs, err := controlItems(ctx, c, repo, f, 0)
	if err != nil {
		return err
	}
	slices.SortFunc(subs, func(a, b *gh.Issue) int { return a.GetNumber() - b.GetNumber() })
	rep := MissionRecoverReport{Repo: repo, Feature: f, Control: "recover", By: r.by, Tool: tool, Stories: []RecoverStory{}, Ledger: MissionPlanPath(f), DryRun: c.DryRun()}
	for _, is := range subs {
		n := is.GetNumber()
		var st delegate.State
		if is.GetState() != "open" {
			continue
		}
		if _, err := readLedgerJSON(ctx, w, delegate.LedgerPath(n), &st); err != nil {
			return Wrap(ExitExternal, err)
		}
		if st.Receiver == "" || st.Status == dlgMerged { // not started: mission next starts it
			continue
		}
		rep.Stories = append(rep.Stories, r.story(ctx, n, st))
	}
	ctl := MissionControl{Action: "recover", By: r.by, Tool: tool, At: r.now.UTC().Format(time.RFC3339), Presence: "none"}
	if rep.Commit, rep.Changed, err = r.write(ctx, ctl, fmt.Sprintf("missions: @%s recovered the mission of #%d on %s", r.by, f, tool)); err != nil {
		return err
	}
	return emitRecover(e, rep)
}

// missionRetry is `aayu mission retry <f> --story <n>`.
func missionRetry(ctx context.Context, e *Env, f int, in Values) error {
	what := "mission retry"
	if err := e.laneRefusal(what); err != nil {
		return WithNext(err, "a person on the mission's tool runs it in their own terminal")
	}
	story := issueNumber(in.String("story"))
	if story == 0 {
		return Errorf(ExitUsage, "mission retry needs --story: the story or Bug whose failed step runs once more")
	}
	repo, c, w, err := e.missionLedger(ctx)
	if err != nil {
		return err
	}
	plan, appr, _, err := readMission(ctx, w, f)
	if err != nil {
		return err
	}
	if err := controllable(plan, f); err != nil {
		return err
	}
	tool, err := e.callerTool(ctx, repo)
	if err != nil {
		return err
	}
	runs := fmt.Sprintf("#%d's mission", f)
	if slices.ContainsFunc(plan.StoryStarts, func(s MissionStoryStart) bool { return s.Number == story }) {
		runs = fmt.Sprintf("#%d", story)
	}
	switch want := plan.storyTool(story); {
	case tool != want:
		return Errorf(ExitBlocked, "%s runs on %s, the tool of @%s, who started it: run this there", runs, want, plan.starter(story))
	case plan.Pause != nil:
		return WithNext(Errorf(ExitBlocked, "The mission of #%d is paused (%s): resume it first. Nothing changed.", f, plan.Pause.Reason), fmt.Sprintf("aayu mission resume %d", f))
	case plan.stopped(story) != nil:
		return WithNext(Errorf(ExitBlocked, "#%d is stopped (%s): resume it first. Nothing changed.", story, plan.stopped(story).Reason), fmt.Sprintf("aayu mission resume %d --story %d", f, story))
	}
	owner, name, _ := github.SplitRepo(repo)
	if err := inMission(ctx, c, owner, name, plan, f, story); err != nil {
		return err
	}
	var st delegate.State
	if _, err := readLedgerJSON(ctx, w, delegate.LedgerPath(story), &st); err != nil {
		return Wrap(ExitExternal, err)
	}
	r, err := e.newRecovery(ctx, c, w, repo, f, plan, appr, "retry", tool, "")
	if err != nil {
		return err
	}
	pr := r.openPR(story, st)
	ss := plan.stateOf(story)
	if st.Status != dlgFailed && st.Status != dlgMaybe && (ss == nil || ss.State == "" || len(ss.Attempts) == 0) {
		return Errorf(ExitBlocked, "#%d has no failed step in the mission of #%d: nothing to retry. Nothing changed.", story, f)
	}
	step := fmt.Sprintf("hands #%d off again on %s, from its branch and its brief", story, tool)
	var judged string
	if pr != 0 {
		step = fmt.Sprintf("runs the sync of #%d's PR #%d on %s again", story, pr, tool)
		if judged, err = judgedHead(ctx, e, c, owner, name, pr); err != nil {
			return err
		}
	}
	effect := []string{fmt.Sprintf("Retries #%d of the mission of #%d once more: it %s; the attempt is counted and costed in %s", story, f, step, MissionPlanPath(f))}
	if judged != "" {
		effect = append(effect, judged)
	}
	p, err := e.presenceCheck(ctx, Pending{What: what, Effect: effect}, AccessWrite, e.personOnly(what))
	if err != nil {
		return err
	}
	r.by, r.now = p.Login, e.Now()
	rep := MissionRecoverReport{Repo: repo, Feature: f, Control: "retry", Story: story, By: p.Login, Tool: tool, Effect: effect, Stories: []RecoverStory{}, Ledger: MissionPlanPath(f), DryRun: c.DryRun()}
	if rep.DryRun {
		return emitRecover(e, rep)
	}
	// The issue and its delegation as they are now, after the device code.
	if _, err := readLedgerJSON(ctx, w, delegate.LedgerPath(story), &st); err != nil {
		return Wrap(ExitExternal, err)
	}
	if st.Status == dlgFailed {
		r.counted(story, st)
	}
	line := RecoverStory{Story: story, State: "retried"}
	after, err := r.attempt(ctx, story, st, pr, "retry", "a person's retry")
	got := r.next.stateOf(story)
	switch {
	case errors.Is(err, errClaimed):
		line.State, line.Line = "left", err.Error()
	case err != nil:
		line.State, line.Line = "failed", fmt.Sprintf("retried by @%s: failed again: %s; %s", p.Login, oneLineErr(err), or(got.State, "failed"))
	case pr != 0:
		line.Line = fmt.Sprintf("retried by @%s: the sync of PR #%d ran again (%s)", p.Login, pr, after.Status)
		if judged != "" {
			line.Line += "; " + judged
		}
	default:
		line.Line = fmt.Sprintf("retried by @%s: restarted from %s and its brief (%s)", p.Login, after.Branch, or(after.TaskURL, after.Task))
	}
	if got != nil {
		line.Line += fmt.Sprintf(" · %d attempts, %s", len(got.Attempts), ready.USD(got.cost()))
	}
	rep.Stories = append(rep.Stories, line)
	ctl := MissionControl{Action: "retry", By: p.Login, Tool: tool, At: r.now.UTC().Format(time.RFC3339), Presence: p.Via, Story: story}
	r.changed = true // a retry is always recorded
	if rep.Commit, rep.Changed, err = r.write(ctx, ctl, fmt.Sprintf("missions: @%s retried #%d in the mission of #%d on %s", p.Login, story, f, tool)); err != nil {
		return err
	}
	return emitRecover(e, rep)
}

// judgedHead says that PR n's head has a judged inspect review, which a
// retry does not run again, or "".
func judgedHead(ctx context.Context, e *Env, c *github.Client, owner, name string, n int) (string, error) {
	pr, _, err := c.REST.PullRequests.Get(ctx, owner, name, n)
	if err != nil {
		return "", Wrap(ExitExternal, err)
	}
	comments, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.IssueComment, *gh.Response, error) {
		return c.REST.Issues.ListComments(ctx, owner, name, n, &gh.IssueListCommentsOptions{ListOptions: o})
	})
	if err != nil {
		return "", Wrap(ExitExternal, err)
	}
	var id config.Identity
	if l, err := e.Config(); err == nil {
		id = l.Config.Identity
	}
	head := pr.GetHead().GetSHA()
	if res := inspectState(comments, viewer(ctx, c), id).Result; res != nil && res.Head == head {
		return fmt.Sprintf("PR #%d's review at %s is judged (%s): the retry does not run it again", n, short(head), res.Disposition), nil
	}
	return "", nil
}

func emitRecover(e *Env, rep MissionRecoverReport) error {
	return e.Emit(rep, func(w io.Writer) error {
		subject := fmt.Sprintf("the mission of #%d", rep.Feature)
		if rep.Control == "retry" {
			fmt.Fprintf(w, "@%s retried #%d in %s on %s.\n", rep.By, rep.Story, subject, rep.Tool)
		} else {
			fmt.Fprintf(w, "@%s recovered %s on %s: aayu read the record and each host.\n", rep.By, subject, rep.Tool)
		}
		if rep.DryRun {
			for _, l := range rep.Effect {
				fmt.Fprintf(w, "  %s\n", l)
			}
		}
		for _, s := range rep.Stories {
			fmt.Fprintf(w, "#%d  %s\n", s.Story, s.Line)
			if s.Next != "" {
				fmt.Fprintf(w, "     next: %s\n", s.Next)
			}
		}
		if len(rep.Stories) == 0 && rep.Control == "recover" {
			fmt.Fprintln(w, "  no story runs: mission next starts what can start")
		}
		switch {
		case rep.DryRun:
			_, err := fmt.Fprintf(w, "  --dry-run: ledger %s planned, not written; nothing restarted\n", rep.Ledger)
			return err
		case !rep.Changed:
			_, err := fmt.Fprintln(w, "  nothing changed")
			return err
		}
		_, err := fmt.Fprintf(w, "  ledger %s (%s)\n", rep.Ledger, short(rep.Commit))
		return err
	})
}
