package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/delegate"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/ready"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// `aayu mission stop <f> [--story n] [--end] --reason` and `aayu mission
// resume <f> [--story n]` (Story #1002; Decision #915's controls, M11, R9 per
// person, Q8 A). A stop is for an extreme case. It writes the stop on
// missions/<f>/plan.json, where `aayu mission next` and `aayu ship` read it,
// then tells each running story thread it can reach to commit at its next
// checkpoint and stop. Claims, branches, and PRs stay. No other mission or
// loop changes, and `aayu halt` stays the emergency stop for a tool incident
// (threat model T18). --story stops one story or Bug; the rest of the mission
// runs on. --end, only by the person who started the mission, ends it: a
// merged story stays merged, an open one loses its claims and is Ready again
// with its PRs closed, and the lock ends. A person stops from any tool;
// resume runs only on the mission's recorded tool. Each control adds an entry
// to the plan's controls, and counts as the person who ran it.

// stopNext is what an agent lane that runs a stop is told to do instead.
const stopNext = "a person runs it in their own terminal, or presses Stop in aayu inbox"

// MissionControlReport is what mission stop and resume report.
type MissionControlReport struct {
	Repo      string          `json:"repo"`
	Feature   int             `json:"feature"`
	Control   string          `json:"control"` // stop, resume, or end
	Story     int             `json:"story,omitempty"`
	Reason    string          `json:"reason,omitempty"`
	By        string          `json:"by"`
	Tool      string          `json:"tool,omitempty"`
	Effect    []string        `json:"effect"` // what the person saw before the device code
	Threads   []MissionThread `json:"threads"`
	Ready     []int           `json:"ready_again,omitempty"` // --end: open stories and Bugs made Ready again
	Closed    []int           `json:"prs_closed,omitempty"`  // --end
	Commented []int           `json:"commented,omitempty"`   // --end: the locked issues
	Work      *WorkList       `json:"work,omitempty"`        // resume: what starts now
	WorkNote  string          `json:"work_note,omitempty"`   // resume: why nothing starts
	Ledger    string          `json:"ledger_path"`
	Commit    string          `json:"ledger_commit,omitempty"`
	DryRun    bool            `json:"dry_run,omitempty"`
}

// MissionThread is a running story's thread, and what the control told it.
type MissionThread struct {
	Story    int    `json:"story"`
	Receiver string `json:"receiver"`
	URL      string `json:"url"`
	Reached  bool   `json:"reached"`
	Line     string `json:"line"`
	Error    string `json:"error,omitempty"`
}

// missionStop is `aayu mission stop <f>`.
func missionStop(ctx context.Context, e *Env, f int, in Values) error {
	what := "mission stop"
	if err := e.laneRefusal(what); err != nil {
		return WithNext(err, stopNext)
	}
	reason, story, end := strings.TrimSpace(in.String("reason")), issueNumber(in.String("story")), in.Bool("end")
	switch {
	case reason == "":
		return Errorf(ExitUsage, "mission stop needs --reason: why you stop it, in one line")
	case end && story != 0:
		return Errorf(ExitUsage, "mission stop --end ends the whole mission: it takes no --story")
	}
	repo, c, w, err := e.missionLedger(ctx)
	if err != nil {
		return err
	}
	plan, _, _, err := readMission(ctx, w, f)
	if err != nil {
		return err
	}
	if err := controllable(plan, f); err != nil {
		return err
	}
	owner, name, _ := github.SplitRepo(repo)
	control := "stop"
	switch {
	case end:
		control = "end"
		me, _, err := c.REST.Users.Get(ctx, "")
		if err != nil {
			return Wrap(ExitExternal, err)
		}
		if !strings.EqualFold(me.GetLogin(), plan.StartedBy) { // Q8 A: a mission you started
			return Errorf(ExitBlocked, "#%d was started by @%s: only @%s may end it", f, plan.StartedBy, plan.StartedBy)
		}
	case story != 0:
		if err := inMission(ctx, c, owner, name, plan, f, story); err != nil {
			return err
		}
		if s := plan.stopped(story); s != nil {
			return WithNext(Errorf(ExitBlocked, "#%d is stopped already (%s). Nothing changed.", story, s.Reason), fmt.Sprintf("aayu mission resume %d --story %d", f, story))
		}
	case plan.Pause != nil:
		return WithNext(Errorf(ExitBlocked, "The mission of #%d is stopped already (%s). Nothing changed.", f, plan.Pause.Reason), fmt.Sprintf("aayu mission resume %d", f))
	}
	tool, err := e.callerTool(ctx, repo) // any tool may stop: the record is what ship and the Kanha read
	if err != nil {
		return err
	}
	var effect []string
	switch control {
	case "end":
		effect = []string{fmt.Sprintf("Ends the mission of #%d (%s): merged stories stay merged; each open story and Bug loses its claims and goes back to status:ready, and its open PRs close", f, reason),
			fmt.Sprintf("Ends the lock of #%d, and posts one comment on each locked issue", f)}
	case "stop":
		if story != 0 {
			effect = []string{fmt.Sprintf("Stops #%d of the mission of #%d (%s): its thread stops its subagents, commits at its next checkpoint, and stops; it keeps its claim, its PR, and its slot", story, f, reason),
				fmt.Sprintf("aayu ship merges no PR of #%d until a person resumes it; the rest of #%d runs on", story, f)}
			break
		}
		effect = []string{fmt.Sprintf("Stops the mission of #%d (%s): aayu mission next starts nothing, and aayu ship merges none of its PRs, until a person resumes it", f, reason),
			"Each running story thread commits at its next checkpoint and stops; claims, branches, and PRs stay",
			"Other missions and loops run on; aayu halt is not set"}
	}
	p, err := e.presenceCheck(ctx, Pending{What: what, Effect: effect}, AccessWrite, e.personOnly(what))
	if err != nil {
		return err
	}
	at := e.Now().UTC().Format(time.RFC3339)
	ctl := MissionControl{Action: control, By: p.Login, Tool: tool, At: at, Presence: p.Via, Story: story, Reason: reason}
	rep := MissionControlReport{Repo: repo, Feature: f, Control: control, Story: story, Reason: reason, By: p.Login, Tool: tool, Effect: effect,
		Threads: []MissionThread{}, Ledger: MissionPlanPath(f), DryRun: c.DryRun()}
	if rep.DryRun {
		return emitControl(e, rep)
	}
	// The issues and threads as they are now, after the device code: a story
	// that started or merged while it waited counts as it is.
	subs, err := controlItems(ctx, c, repo, f, story)
	if err != nil {
		return err
	}
	var msg string
	switch {
	case end:
		// GitHub first, each step safe to run again; the ledger last, so a failed end can run again.
		if err := e.endMission(ctx, c, w, repo, f, plan, subs, p.Login, reason, &rep); err != nil {
			return err
		}
		rep.Commit, err = writePlan(ctx, w, f, fmt.Sprintf("missions: @%s ended the mission of #%d (%s)", p.Login, f, reason), func(pl *MissionPlan) error {
			pl.State = fmt.Sprintf("ended (%s)", reason)
			if pl.Lock != nil {
				pl.Lock.State, pl.Lock.EndedAt, pl.Lock.EndReason = "ended", &at, &reason
			}
			pl.Controls = append(pl.Controls, ctl)
			return nil
		})
		msg = fmt.Sprintf("aayu: @%s ended the mission of #%d (%s). Stop now, and start nothing new: your PR is closed, and the story is Ready again.", p.Login, f, reason)
	case story != 0:
		rep.Commit, err = writePlan(ctx, w, f, fmt.Sprintf("missions: @%s stopped #%d in the mission of #%d (%s)", p.Login, story, f, reason), func(pl *MissionPlan) error {
			if pl.stopped(story) != nil {
				return Errorf(ExitBlocked, "#%d was stopped while the code was waiting. Nothing changed.", story)
			}
			pl.Stopped = append(pl.Stopped, MissionStopped{Number: story, Reason: reason, By: p.Login, At: at})
			pl.Controls = append(pl.Controls, ctl)
			return nil
		})
		msg = fmt.Sprintf("aayu: @%s stopped #%d (%s). Stop your subagents, commit your work at your next checkpoint, push it, and stop. Keep the branch and the PR. Start nothing new until aayu says resume.", p.Login, story, reason)
	default:
		rep.Commit, err = writePlan(ctx, w, f, fmt.Sprintf("missions: @%s stopped the mission of #%d (%s)", p.Login, f, reason), func(pl *MissionPlan) error {
			if pl.Pause != nil {
				return Errorf(ExitBlocked, "The mission of #%d was stopped while the code was waiting (%s). Nothing changed.", f, pl.Pause.Reason)
			}
			pl.Pause = &MissionPause{Reason: reason, By: p.Login, At: at}
			pl.Controls = append(pl.Controls, ctl)
			return nil
		})
		msg = fmt.Sprintf("aayu: @%s stopped the mission of #%d (%s). Stop your subagents, commit your work at your next checkpoint, push it, and stop. Keep the branch and the PR. Start nothing new until aayu says resume.", p.Login, f, reason)
	}
	if err != nil {
		return err
	}
	// Read after the stop is written: from here mission next starts nothing more.
	running, err := runningThreads(ctx, w, subs, func(n int) bool { return story != 0 || plan.stopped(n) == nil })
	if err != nil {
		return err
	}
	told, waits := "told to stop its subagents, commit at its next checkpoint, and stop", "its PR waits"
	if end {
		told, waits = "told to stop: the mission ended", "its PR stays closed"
	}
	if rep.Threads, err = e.tell(ctx, c, w, repo, running, "delegate.mission-"+control, msg, told, waits); err != nil {
		return err
	}
	return emitControl(e, rep)
}

// missionResume is `aayu mission resume <f>`.
func missionResume(ctx context.Context, e *Env, f int, in Values) error {
	what := "mission resume"
	if err := e.laneRefusal(what); err != nil {
		return WithNext(err, "a person on the mission's tool runs it in their own terminal")
	}
	story := issueNumber(in.String("story"))
	repo, c, w, err := e.missionLedger(ctx)
	if err != nil {
		return err
	}
	plan, _, _, err := readMission(ctx, w, f)
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
	// R9: the tool only, never the person. A Tatkal story a person started on
	// their own tool resumes on that tool (ST9).
	runs, want, starter := fmt.Sprintf("#%d's mission", f), plan.Tool, plan.StartedBy
	if i := slices.IndexFunc(plan.StoryStarts, func(s MissionStoryStart) bool { return s.Number == story }); story != 0 && i >= 0 {
		runs, want, starter = fmt.Sprintf("#%d", story), plan.StoryStarts[i].Tool, plan.StoryStarts[i].StartedBy
	}
	switch {
	case tool != want:
		return Errorf(ExitBlocked, "%s runs on %s, the tool of @%s, who started it: run this there", runs, want, starter)
	case story != 0 && plan.stopped(story) == nil:
		return Errorf(ExitBlocked, "#%d is not stopped in the mission of #%d. Nothing changed.", story, f)
	case story == 0 && plan.Pause == nil:
		return Errorf(ExitBlocked, "The mission of #%d is not stopped. Nothing changed.", f)
	}
	effect := []string{fmt.Sprintf("Resumes the mission of #%d on %s: aayu reads %s and GitHub, continues each stopped story thread, and starts stories up to its agents", f, tool, MissionPlanPath(f))}
	if story != 0 {
		effect = []string{fmt.Sprintf("Resumes #%d of the mission of #%d on %s: only its thread continues", story, f, tool)}
	}
	p, err := e.presenceCheck(ctx, Pending{What: what, Effect: effect}, AccessWrite, e.personOnly(what))
	if err != nil {
		return err
	}
	at := e.Now().UTC().Format(time.RFC3339)
	rep := MissionControlReport{Repo: repo, Feature: f, Control: "resume", Story: story, By: p.Login, Tool: tool, Effect: effect,
		Threads: []MissionThread{}, Ledger: MissionPlanPath(f), DryRun: c.DryRun()}
	if rep.DryRun {
		return emitControl(e, rep)
	}
	subs, err := controlItems(ctx, c, repo, f, story)
	if err != nil {
		return err
	}
	running, err := runningThreads(ctx, w, subs, func(n int) bool { return story != 0 || plan.stopped(n) == nil })
	if err != nil {
		return err
	}
	ctl := MissionControl{Action: "resume", By: p.Login, Tool: tool, At: at, Presence: p.Via, Story: story}
	msg := fmt.Sprintf("aayu: @%s resumed the mission of #%d. Continue from your last checkpoint.", p.Login, f)
	if story != 0 {
		msg = fmt.Sprintf("aayu: @%s resumed #%d. Continue from your last checkpoint.", p.Login, story)
	}
	subject := fmt.Sprintf("the mission of #%d", f)
	if story != 0 {
		subject = fmt.Sprintf("#%d in the mission of #%d", story, f)
	}
	rep.Commit, err = writePlan(ctx, w, f, fmt.Sprintf("missions: @%s resumed %s on %s", p.Login, subject, tool),
		func(pl *MissionPlan) error {
			switch {
			case story != 0 && pl.stopped(story) == nil, story == 0 && pl.Pause == nil:
				return Errorf(ExitBlocked, "It was resumed while the code was waiting. Nothing changed.")
			case story != 0:
				pl.Stopped = slices.DeleteFunc(pl.Stopped, func(s MissionStopped) bool { return s.Number == story })
			default:
				pl.Pause = nil
			}
			pl.Controls = append(pl.Controls, ctl)
			return nil
		})
	if err != nil {
		return err
	}
	if rep.Threads, err = e.tell(ctx, c, w, repo, running, "delegate.mission-resume", msg, "told to continue from its last checkpoint", "it was not told to stop"); err != nil {
		return err
	}
	if story == 0 { // the stories that can start now, up to the agents, as mission next lists them
		var work WorkList
		e.work = &work
		err := missionNext(ctx, e, f)
		e.work = nil
		if err != nil {
			rep.WorkNote = "nothing starts: " + err.Error()
		} else {
			rep.Work = &work
		}
	}
	return emitControl(e, rep)
}

// missionHold judges PR n of issue story (ref, nil when it does not exist)
// against the mission it is in (Story #1002; Decisions #915 and #974). A PR
// of a stopped mission, or of a stopped story or Bug, waits until a person
// resumes it. A PR of a milestone waits until every earlier milestone has
// passed its gate (M9 A: build ahead, merge in order); it keeps its claim and
// its slot. A PR that refers to the Feature itself is a gate fix: it merges
// only when the plan's gate_fixes lists it on the approved record as it is
// now, and fix is then that entry. It reads the plan at the ledger tip, so
// ship runs it again just before the merge.
func missionHold(ctx context.Context, c *github.Client, w *ledger.Writer, repo string, n, story int, ref *gh.Issue) (*GateFix, error) {
	if ref == nil {
		return nil, nil
	}
	owner, name, _ := github.SplitRepo(repo)
	f, kind := story, ""
	if kind, _ = workitems.TypeOf(ref.GetType().GetName(), github.IssueLabels(ref)); kind != "feature" {
		parent, _, err := c.REST.SubIssue.GetParentIssue(ctx, owner, name, int64(story))
		if github.NotFound(err) {
			return nil, nil
		} else if err != nil {
			return nil, Wrap(ExitExternal, err)
		}
		f = parent.GetNumber()
	}
	plan, _, _, err := readMission(ctx, w, f)
	if err != nil || plan == nil {
		return nil, err
	}
	var fix *GateFix
	ms, who := "", fmt.Sprintf("#%d", story)
	if kind == "feature" {
		i := slices.IndexFunc(plan.GateFixes, func(g GateFix) bool { return g.PR == n })
		if i < 0 {
			return nil, Errorf(ExitBlocked, "ship #%d: #%d is not a listed gate fix of #%d", n, n, f)
		}
		fix, ms, who = &plan.GateFixes[i], plan.GateFixes[i].Milestone, fmt.Sprintf("#%d", n)
		t, err := w.Read(ctx)
		if err != nil {
			return nil, Wrap(ExitExternal, err)
		}
		record, err := t.File(ctx, ready.ApprovalPath(f))
		t.Close()
		if err := ignoreMissing(err); err != nil {
			return nil, Wrap(ExitExternal, err)
		}
		sum := sha256.Sum256(record)
		switch now := hex.EncodeToString(sum[:]); {
		case record == nil || fix.Approval.Record != ready.ApprovalPath(f) || fix.Approval.SHA256 != now:
			return nil, Errorf(ExitBlocked, "ship #%d: gate fix #%d rests on %s at %s, and #%d's approved record has changed since, so its approval no longer covers the fix", n, n, fix.Approval.Record, short(fix.Approval.SHA256), f)
		case fix.Waits != "":
			return nil, Errorf(ExitBlocked, "ship #%d: gate fix #%d of #%d waits for a person (%s)", n, n, f, fix.Waits)
		}
	} else if i := slices.IndexFunc(plan.Graph, func(it MissionItem) bool { return it.Number == story }); i >= 0 {
		ms = plan.Graph[i].Milestone
	}
	if plan.StartedBy == "" || plan.State != "" { // a mission that has not started, or has ended, holds nothing
		return fix, nil
	}
	if plan.Pause != nil {
		return nil, WithNext(Errorf(ExitBlocked, "ship #%d: #%d's mission is stopped (%s): resume it first", n, f, plan.Pause.Reason), fmt.Sprintf("aayu mission resume %d", f))
	}
	if s := plan.stopped(story); s != nil {
		return nil, WithNext(Errorf(ExitBlocked, "ship #%d: #%d is stopped (%s): resume it first", n, story, s.Reason), fmt.Sprintf("aayu mission resume %d --story %d", f, story))
	}
	for _, m := range plan.Milestones {
		if ms == "" || m.Name == ms {
			break
		}
		if m.Gate.State != GatePassed {
			return nil, Errorf(ExitBlocked, "ship #%d: %s is in %s: it merges after %s's gate passes", n, who, ms, m.Name)
		}
	}
	return fix, nil
}

// controllable says why Feature f's plan takes no stop or resume now, or nil.
func controllable(plan *MissionPlan, f int) error {
	switch {
	case plan == nil:
		return WithNext(Errorf(ExitBlocked, "#%d has no mission plan: a person approves it to build first. Nothing changed.", f), fmt.Sprintf("aayu ready %d", f))
	case plan.StartedBy == "":
		return WithNext(Errorf(ExitBlocked, "The mission of #%d has not started. Nothing changed.", f), fmt.Sprintf("aayu mission start %d", f))
	case plan.State != "":
		return Errorf(ExitBlocked, "The mission of #%d %s. Nothing changed.", f, plan.State)
	}
	return nil
}

// inMission refuses story n unless it is a story or Bug of Feature f: in the
// plan's graph, or a sub-issue of f filed since (a gate's or a Tatkal Bug).
func inMission(ctx context.Context, c *github.Client, owner, name string, plan *MissionPlan, f, n int) error {
	if slices.ContainsFunc(plan.Graph, func(it MissionItem) bool { return it.Number == n }) {
		return nil
	}
	parent, _, err := c.REST.SubIssue.GetParentIssue(ctx, owner, name, int64(n))
	if err != nil && !github.NotFound(err) {
		return Wrap(ExitExternal, err)
	}
	if parent.GetNumber() != f {
		return Errorf(ExitBlocked, "#%d is not a story or Bug of #%d. Nothing changed.", n, f)
	}
	return nil
}

// controlItems are the issues a control acts on: story n, or every story and
// Bug of Feature f as they are now.
func controlItems(ctx context.Context, c *github.Client, repo string, f, n int) ([]*gh.Issue, error) {
	if n == 0 {
		return featureItems(ctx, c, repo, f, "story", "bug")
	}
	owner, name, _ := github.SplitRepo(repo)
	is, _, err := c.REST.Issues.Get(ctx, owner, name, n)
	if err != nil {
		return nil, Wrap(ExitExternal, err)
	}
	return []*gh.Issue{is}, nil
}

// runningThreads are the delegations (delegate/<n>/state.json) of the open
// issues that keep, whose hand-off has not ended: they still run.
func runningThreads(ctx context.Context, w *ledger.Writer, subs []*gh.Issue, keep func(n int) bool) ([]delegate.State, error) {
	t, err := w.Read(ctx)
	if err != nil {
		return nil, Wrap(ExitExternal, ignoreMissing(err))
	}
	defer t.Close()
	var out []delegate.State
	for _, is := range subs {
		n := is.GetNumber()
		if is.GetState() != "open" || !keep(n) {
			continue
		}
		b, err := t.File(ctx, delegate.LedgerPath(n))
		if err := ignoreMissing(err); err != nil {
			return nil, Wrap(ExitExternal, err)
		}
		var st delegate.State
		if b == nil || json.Unmarshal(b, &st) != nil || st.Receiver == "" || slices.Contains([]string{dlgFailed, dlgMerged, dlgCapped}, st.Status) {
			continue
		}
		out = append(out, st)
	}
	return out, nil
}

// tell sends msg to each running thread and says which it reached. An Amp,
// Claude Code, or Cursor cloud thread hears it as a follow-up. A Codex cloud
// task (a follow-up is a new task), a Copilot task, a CLI run on someone's
// machine, and a thread this machine cannot reach are not reached: each runs
// to its end. A reached thread's delegation records the message.
func (e *Env) tell(ctx context.Context, c *github.Client, w *ledger.Writer, repo string, sts []delegate.State, kind, msg, told, waits string) ([]MissionThread, error) {
	out := []MissionThread{}
	if len(sts) == 0 {
		return out, nil
	}
	l, err := e.Config()
	if err != nil {
		return nil, err
	}
	log, err := e.Actions()
	if err != nil {
		return nil, err
	}
	now := e.Now().UTC().Format(time.RFC3339)
	reached := map[int]string{} // story: the task ID its follow-up returned
	for _, st := range sts {
		th := MissionThread{Story: st.Story, Receiver: st.Receiver, URL: or(st.TaskURL, st.Task)}
		if th.URL == "" {
			th.URL = fmt.Sprintf("#%d's %s run on its builder's machine", st.Story, or(delegate.Names[st.Receiver], st.Receiver))
		}
		if st.Mode != "local" && st.Task != "" && slices.Contains([]string{delegate.Amp, delegate.Claude, delegate.Cursor}, st.Receiver) {
			var task delegate.Task
			_, err := log.Do(ctx, actions.Action{Kind: kind, Target: fmt.Sprintf("%s#%d@%s", repo, st.Story, st.Receiver), Story: fmt.Sprintf("#%d", st.Story),
				Seat: "engineering-manager", Host: e.Host, Identity: c.Identity(), Payload: map[string]any{"receiver": st.Receiver, "task": st.Task}},
				func(ctx context.Context) (actions.Outcome, error) {
					var err error
					task, err = e.receiverAdapter(l.Config.Delegate, st.Receiver, c, repo, config.RepoRoot(e.Dir)).FollowUp(ctx, st, msg)
					return actions.Outcome{Attempts: 1}, err
				})
			if err == nil {
				th.Reached = true
				reached[st.Story] = task.ID
			} else {
				th.Error = oneLineErr(err)
			}
		}
		th.Line = told
		if !th.Reached {
			th.Line = fmt.Sprintf("not reached: %s; it runs to its end, and %s", th.URL, waits)
		}
		out = append(out, th)
	}
	if len(reached) == 0 {
		return out, nil
	}
	// Each delegation records the message on the state at the ledger tip, so a
	// sync that wrote meanwhile is kept; a moved tip retries the write, never
	// the follow-ups.
	for range 3 {
		t, err := w.Read(ctx)
		if err != nil {
			return out, Wrap(ExitExternal, err)
		}
		files := map[string][]byte{}
		for n, id := range reached {
			b, err := t.File(ctx, delegate.LedgerPath(n))
			var st delegate.State
			if err != nil || json.Unmarshal(b, &st) != nil {
				t.Close()
				return out, Errorf(ExitExternal, "ledger %s: not a delegation aayu wrote for #%d", delegate.LedgerPath(n), n)
			}
			if id != "" {
				st.Task = id
			}
			st.Events = append(st.Events, delegate.Event{At: now, Status: st.Status, Detail: kind})
			b, _ = json.MarshalIndent(st, "", "  ")
			files[delegate.LedgerPath(n)] = append(b, '\n')
		}
		tip := t.Tip
		t.Close()
		_, err = w.Write(ctx, ledger.Commit{Message: fmt.Sprintf("delegate: %s told %d thread(s)", kind, len(files)), Files: files, Base: tip})
		if !errors.Is(err, ledger.ErrMoved) {
			return out, Wrap(ExitExternal, err)
		}
	}
	return out, Errorf(ExitExternal, "the ledger kept moving while aayu recorded the message to %d thread(s); the threads have it", len(reached))
}

// endMission is --end's work on GitHub (Q8 A): each open story and Bug of
// the mission loses its claims (its lease and its claim lines) and goes back
// to status:ready, its open PRs close, and each locked issue gets one
// comment. A merged story stays merged. Each step is safe to run again.
func (e *Env) endMission(ctx context.Context, c *github.Client, w *ledger.Writer, repo string, f int, plan *MissionPlan, subs []*gh.Issue, by, reason string, rep *MissionControlReport) error {
	owner, name, _ := github.SplitRepo(repo)
	prs, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.PullRequest, *gh.Response, error) {
		return c.REST.PullRequests.List(ctx, owner, name, &gh.PullRequestListOptions{State: "open", ListOptions: o})
	})
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	claims, err := e.claimLines(ctx, w, MissionClaimsPath(f), nil)
	if err != nil {
		return err
	}
	closed, kinds := map[int][]int{}, map[int]string{}
	var release []byte
	for _, is := range subs {
		n := is.GetNumber()
		kinds[n], _ = workitems.TypeOf(is.GetType().GetName(), github.IssueLabels(is))
		if is.GetState() != "open" {
			continue
		}
		if _, err := e.releaseLease(ctx, c, repo, n, fmt.Sprintf("the mission of #%d ended (%s)", f, reason)); err != nil {
			return Wrap(ExitExternal, err)
		}
		for _, cl := range claims {
			if cl.Story == n {
				cl.Release = true
				b, _ := json.Marshal(cl)
				release = append(release, append(b, '\n')...)
			}
		}
		if err := setStatus(ctx, c, repo, n, github.IssueLabels(is), "ready"); err != nil {
			return Wrap(ExitExternal, err)
		}
		rep.Ready = append(rep.Ready, n)
		for _, pr := range prs {
			if StoryOf(pr.GetBody(), "", pr.GetHead().GetRef()) == n {
				closed[n] = append(closed[n], pr.GetNumber())
			}
		}
	}
	if len(release) > 0 {
		if _, err := w.Write(ctx, ledger.Commit{Message: fmt.Sprintf("missions: the mission of #%d ended; its claims are released", f), Append: map[string][]byte{MissionClaimsPath(f): release}}); err != nil {
			return Wrap(ExitExternal, err)
		}
	}
	// The locked issues: the lock's (Story #1013), else the Feature and every issue of the plan.
	locked := []int{f}
	for _, it := range plan.Graph {
		locked = append(locked, it.Number)
	}
	if plan.Lock != nil {
		locked = locked[:0]
		for _, li := range plan.Lock.Issues {
			locked = append(locked, li.Number)
		}
	}
	marker := fmt.Sprintf("<!-- aayudha:mission-ended feature=%d -->", f)
	at := e.Now().Local().Format("15:04")
	for _, n := range locked {
		cs, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.IssueComment, *gh.Response, error) {
			return c.REST.Issues.ListComments(ctx, owner, name, n, &gh.IssueListCommentsOptions{ListOptions: o})
		})
		if err != nil {
			return Wrap(ExitExternal, err)
		}
		if slices.ContainsFunc(cs, func(cm *gh.IssueComment) bool { return strings.Contains(cm.GetBody(), marker) }) {
			continue // an earlier --end that failed later posted it
		}
		body := fmt.Sprintf("%s\nThe mission of #%d ended (%s) at %s, by @%s: “%s”.", marker, f, reason, at, by, reason)
		if plan.Lock != nil {
			body += fmt.Sprintf(" #%d is no longer read-only.", n)
		}
		what := "the " + or(kinds[n], "story")
		switch prs := closed[n]; {
		case len(prs) == 1:
			body += fmt.Sprintf(" Its PR #%d is closed; %s is Ready again.", prs[0], what)
		case len(prs) > 1:
			body += fmt.Sprintf(" Its PRs %s are closed; %s is Ready again.", andList(strings.Split(issueRefs(prs), ", ")), what)
		case slices.Contains(rep.Ready, n):
			body += fmt.Sprintf(" %s is Ready again.", upperFirst(what))
		}
		if err := postComment(ctx, c, repo, n, "github.comment.mission-ended", body); err != nil {
			return Wrap(ExitExternal, err)
		}
		rep.Commented = append(rep.Commented, n)
	}
	// The PRs close last: a run that fails before this closes them again, and
	// the comments already name them.
	for _, is := range subs {
		n := is.GetNumber()
		for _, k := range closed[n] {
			if _, err := c.Write(ctx, actions.Action{Kind: "github.pull.close", Target: fmt.Sprintf("%s#%d", repo, k), Story: fmt.Sprintf("#%d", n),
				Request: &actions.Request{Method: http.MethodPatch, Path: fmt.Sprintf("/repos/%s/pulls/%d", repo, k)}, Payload: map[string]any{"state": "closed", "mission": f}},
				func(ctx context.Context) (any, error) {
					_, _, err := c.REST.PullRequests.Edit(ctx, owner, name, k, &gh.PullRequest{State: gh.Ptr("closed")})
					return nil, err
				}); err != nil {
				return Wrap(ExitExternal, err)
			}
			rep.Closed = append(rep.Closed, k)
		}
	}
	return nil
}

// writePlan applies edit to Feature f's plan at the ledger tip and writes it
// on that tip, again when another write moved the ledger meanwhile (the
// mission's claims move it often). edit refuses a plan that no longer takes
// the control.
func writePlan(ctx context.Context, w *ledger.Writer, f int, msg string, edit func(*MissionPlan) error) (string, error) {
	for range 3 {
		plan, _, tip, err := readMission(ctx, w, f)
		if err != nil {
			return "", err
		}
		if err := controllable(plan, f); err != nil {
			return "", err
		}
		if err := edit(plan); err != nil {
			return "", err
		}
		data, _ := json.MarshalIndent(plan, "", "  ")
		commit, err := w.Write(ctx, ledger.Commit{Message: msg, Files: map[string][]byte{MissionPlanPath(f): append(data, '\n')}, Base: tip})
		if !errors.Is(err, ledger.ErrMoved) {
			return commit, Wrap(ExitExternal, err)
		}
	}
	return "", Errorf(ExitExternal, "the ledger kept moving while aayu wrote %s. Nothing changed. Run it again.", MissionPlanPath(f))
}

func emitControl(e *Env, rep MissionControlReport) error {
	return e.Emit(rep, func(w io.Writer) error {
		subject := fmt.Sprintf("the mission of #%d", rep.Feature)
		if rep.Story != 0 {
			subject = fmt.Sprintf("#%d in the mission of #%d", rep.Story, rep.Feature)
		}
		switch rep.Control {
		case "resume":
			fmt.Fprintf(w, "@%s resumed %s on %s.\n", rep.By, subject, rep.Tool)
		case "end":
			fmt.Fprintf(w, "@%s ended %s: “%s”.\n", rep.By, subject, rep.Reason)
		default:
			fmt.Fprintf(w, "@%s stopped %s: “%s”.\n", rep.By, subject, rep.Reason)
		}
		if rep.DryRun {
			for _, l := range rep.Effect {
				fmt.Fprintf(w, "  %s\n", l)
			}
			_, err := fmt.Fprintf(w, "  --dry-run: ledger %s planned, not written; no thread was told\n", rep.Ledger)
			return err
		}
		for _, th := range rep.Threads {
			fmt.Fprintf(w, "  #%d %s\n", th.Story, th.Line)
		}
		if len(rep.Ready) > 0 {
			fmt.Fprintf(w, "  Ready again: %s; PRs closed: %s\n", issueRefs(rep.Ready), or(issueRefs(rep.Closed), "none"))
		}
		if len(rep.Commented) > 0 {
			fmt.Fprintf(w, "  commented on %s\n", issueRefs(rep.Commented))
		}
		if rep.Control == "stop" {
			fmt.Fprintln(w, "  claims, branches, and PRs stay; aayu ship merges none of its PRs until a person resumes it")
		}
		if rep.Work != nil {
			for _, it := range rep.Work.Items {
				line := "  " + it.Action + " " + it.Key
				if it.Reason != "" {
					line += ": " + it.Reason
				}
				fmt.Fprintln(w, line)
			}
		}
		if rep.Work != nil && rep.Work.note != "" {
			fmt.Fprintf(w, "  %s\n", rep.Work.note)
		}
		if rep.WorkNote != "" {
			fmt.Fprintf(w, "  %s\n", rep.WorkNote)
		}
		_, err := fmt.Fprintf(w, "  ledger %s (%s)\n", rep.Ledger, short(rep.Commit))
		return err
	})
}
