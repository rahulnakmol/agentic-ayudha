package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/delegate"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/ready"
)

// The safety tick (Story #1009, core design G18 and F6): events wake a
// mission's Kanha (a merge, a block, a failed start, an answered ask), and
// `aayu mission tick`, the hourly build-ready loop, catches what they missed,
// with no model. It wakes each started mission's Kanha that has a free agent
// and a story that can start and was told nothing in the last tick, at most
// once a tick, and syncs each open cloud pull request of a receiver that does
// not run aayu itself. It never builds, claims, or starts a story: the Kanha
// does, in its own session (Decision #915 M1).

// tickPeriod is how often the tick runs: the hourly cron of loops/build-ready.toml.
const tickPeriod = time.Hour

// keptWakes is how many of its newest wakes a plan keeps: a day of ticks.
// lastTold reads only the newest, so the plan does not grow each hour.
const keptWakes = 24

// kanhaEvent is the status of a delegation event that told the Kanha (#1007,
// #1008): its detail ends "(sent)", "(not sent: …)", or "(recorded; …)".
const kanhaEvent = "kanha"

// Wake statuses in the mission record: a wake is sending until its send
// returns, so a wake left sending may have reached the Kanha.
const (
	wakeSending = "sending"
	wakeSent    = "sent"
	wakeNotSent = "not sent: "
)

// MissionWake is one safety-tick wake of a mission's Kanha, in missions/<f>/plan.json.
type MissionWake struct {
	At     string `json:"at"`
	Event  string `json:"event"`
	Status string `json:"status"` // sending, sent, or not sent: <why>
}

// MissionEvent is one event of a mission's story: its close on GitHub, or a
// line of its delegation's history in delegate/<n>/state.json.
type MissionEvent struct {
	At    string `json:"at"`
	Story int    `json:"story"`
	Event string `json:"event"`
}

// TickReport is what `aayu mission tick` did.
type TickReport struct {
	Repo   string     `json:"repo"`
	Woken  []TickWake `json:"woken"`
	Held   []TickHeld `json:"held"`   // started missions whose Kanha it did not wake, each with why
	Synced []TickSync `json:"synced"` // the cloud delegations it synced
	Ship   []LoopItem `json:"ship"`   // the synced pull requests at merge-ready, for `aayu ship`
	DryRun bool       `json:"dry_run,omitempty"`
}

// TickWake is one Kanha the tick woke.
type TickWake struct {
	Feature int    `json:"feature"`
	Kanha   string `json:"kanha"` // its URL, else its session
	Event   string `json:"event"`
	Status  string `json:"status"`
}

// TickHeld is a started mission whose Kanha the tick did not wake.
type TickHeld struct {
	Feature int    `json:"feature"`
	Why     string `json:"why"`
}

// TickSync is one cloud delegation the tick synced.
type TickSync struct {
	Story    int    `json:"story"`
	PR       int    `json:"pr,omitempty"` // 0 until the receiver's pull request is found
	Receiver string `json:"receiver"`
	Command  string `json:"command"`
	Status   string `json:"status,omitempty"`
	Next     string `json:"next,omitempty"`
	Error    string `json:"error,omitempty"`
}

// missionTick is `aayu mission tick`.
func missionTick(ctx context.Context, e *Env) error {
	if _, err := os.Stat(e.PauseFile()); err == nil {
		return Errorf(ExitBudget, "mission tick: agent lanes are paused by the [budget] breaker (%s): nothing is woken or synced; a human clears it with `aayu resume`", e.PauseFile())
	}
	repo, c, w, err := e.missionLedger(ctx)
	if err != nil {
		return err
	}
	features, states, err := tickInputs(ctx, w)
	if err != nil {
		return err
	}
	rep := TickReport{Repo: repo, Woken: []TickWake{}, Held: []TickHeld{}, Synced: []TickSync{}, Ship: []LoopItem{}, DryRun: c.DryRun()}
	for _, f := range features {
		if why := e.tickMission(ctx, c, w, repo, f, states, &rep); why != "" {
			rep.Held = append(rep.Held, TickHeld{Feature: f, Why: why})
		}
	}
	e.tickSync(ctx, c, w, repo, states, &rep)
	return e.Emit(rep, func(w io.Writer) error {
		for _, k := range rep.Woken {
			fmt.Fprintf(w, "woke the Kanha of #%d (%s): %s\n", k.Feature, k.Status, k.Event)
		}
		for _, h := range rep.Held {
			fmt.Fprintf(w, "held #%d: %s\n", h.Feature, h.Why)
		}
		for _, s := range rep.Synced {
			fmt.Fprintf(w, "synced #%d (%s): %s: %s\n", s.Story, s.Receiver, s.Command, orDefault(s.Error, s.Status+"; "+s.Next))
		}
		for _, p := range rep.Ship {
			fmt.Fprintf(w, "ship PR #%d (#%d): run aayu ship %d\n", p.PR, p.Story, p.PR)
		}
		if len(rep.Woken)+len(rep.Held)+len(rep.Synced) == 0 {
			fmt.Fprintln(w, "nothing to wake or sync: no mission runs and no cloud pull request is open")
		}
		return nil
	})
}

// tickInputs are the Features with a mission plan on the ledger, and every
// story's delegation state, by story.
func tickInputs(ctx context.Context, w *ledger.Writer) ([]int, map[int]delegate.State, error) {
	t, err := w.Read(ctx)
	if err != nil {
		return nil, nil, Wrap(ExitExternal, ignoreMissing(err))
	}
	defer t.Close()
	var features []int
	states := map[int]delegate.State{}
	for _, dir := range []string{"missions", "delegate"} {
		paths, err := t.List(ctx, dir)
		if err != nil {
			return nil, nil, Wrap(ExitExternal, ignoreMissing(err))
		}
		for _, p := range paths {
			parts := strings.Split(p, "/")
			n, err := strconv.Atoi(parts[min(1, len(parts)-1)])
			switch {
			case len(parts) != 3 || err != nil:
			case dir == "missions" && parts[2] == "plan.json":
				features = append(features, n)
			case dir == "delegate" && parts[2] == "state.json":
				var st delegate.State
				if b, err := t.File(ctx, p); err == nil && json.Unmarshal(b, &st) == nil {
					states[n] = st
				}
			}
		}
	}
	slices.Sort(features)
	return features, states, nil
}

// tickMission wakes Feature f's Kanha when its mission has a free agent and a
// story that can start, and nobody told the Kanha anything in the last tick.
// It says why it did not wake it, or "" (also for a mission not started).
func (e *Env) tickMission(ctx context.Context, c *github.Client, w *ledger.Writer, repo string, f int, states map[int]delegate.State, rep *TickReport) string {
	plan, appr, _, err := readMission(ctx, w, f)
	switch {
	case err != nil: // an unreadable mission keeps its cursor and is shown, never skipped silently (J4 rule 2)
		return "unreadable: " + oneLineErr(err)
	case plan == nil || plan.StartedBy == "":
		return ""
	case appr == nil || appr.Parent == nil:
		return fmt.Sprintf("no Kanha is recorded: a person runs aayu mission start %d", f)
	case plan.State != "": // ended by the person who started it (#1002)
		return plan.State
	case plan.Pause != nil:
		return "paused: " + plan.Pause.Reason
	case appr.Parent.Mode == "local":
		return fmt.Sprintf("its Kanha is local, on %s, and takes events at its own next check (#1012)", appr.Parent.Machine)
	}
	now := e.Now()
	if at := lastTold(f, plan, appr, states); now.Sub(at) < tickPeriod {
		return fmt.Sprintf("its Kanha was told at %s, since the last tick", at.UTC().Format(time.RFC3339))
	}
	changed, err := (&approveRun{e: e, c: c, w: w, n: f}).featureChanges(ctx, repo, *appr)
	if err == nil && len(changed) > 0 {
		return "not approved to build: " + strings.Join(changed, "; ")
	}
	list := WorkList{}
	if err == nil {
		var subs []*gh.Issue
		var clocks []MissionClock // a story past its mission's time limit cannot start, so it wakes nobody (Story #1015)
		var l *config.Loaded
		if l, err = e.Config(); err == nil {
			_, maxH := missionLimits(&l.Config)
			clocks, err = e.missionClocks(ctx, c, w, repo, f, plan, maxH, l.Config.Identity)
		}
		if err == nil {
			if subs, err = featureItems(ctx, c, repo, f, "story", "bug"); err == nil {
				err = e.workList(ctx, c, w, repo, f, plan, appr, subs, clocks, &list, false)
			}
		}
	}
	if err != nil {
		return "unreadable: " + oneLineErr(err)
	}
	var starts, waits []string
	for _, it := range list.Items {
		if it.Action == "start" {
			starts = append(starts, it.Key)
		} else if !slices.Contains(waits, it.Reason) {
			waits = append(waits, it.Reason)
		}
	}
	if len(starts) == 0 {
		return "nothing can start: " + orDefault(strings.Join(waits, "; "), list.note)
	}
	p := appr.Parent
	msg := fmt.Sprintf("tick: %s can start, and no event reached the Kanha since the last tick: run aayu mission next %d", strings.Join(starts, ", "), f)
	wake := TickWake{Feature: f, Kanha: orDefault(p.URL, p.Session), Event: msg, Status: "dry run: not sent"}
	if c.DryRun() {
		rep.Woken = append(rep.Woken, wake)
		return ""
	}
	// The wake is recorded before it is sent, on the plan as read: a second tick
	// that records first wins, so a Kanha is woken at most once a tick, and a
	// pause or end written since the first read holds it.
	at := now.UTC().Format(time.RFC3339)
	held := ""
	ok, err := updatePlan(ctx, w, f, fmt.Sprintf("missions: the tick wakes the Kanha of #%d", f), func(p *MissionPlan) bool {
		switch {
		case p.State != "":
			held = p.State
		case p.Pause != nil:
			held = "paused: " + p.Pause.Reason
		case now.Sub(lastTold(f, p, appr, states)) < tickPeriod:
			held = "another tick woke its Kanha"
		default:
			p.Wakes = append(p.Wakes, MissionWake{At: at, Event: msg, Status: wakeSending})
			p.Wakes = p.Wakes[max(0, len(p.Wakes)-keptWakes):]
			return true
		}
		return false
	})
	switch {
	case err != nil:
		return "unreadable: " + oneLineErr(err)
	case !ok:
		return held
	}
	l, _ := e.Config() // loaded and checked by the GitHub client
	ad := e.receiverAdapter(l.Config.Delegate, p.Host, c, repo, config.RepoRoot(e.Dir))
	log, err := e.Actions()
	if err == nil {
		_, err = log.Do(ctx, actions.Action{Kind: "mission.kanha.wake", Target: fmt.Sprintf("%s#%d@%s", repo, f, p.Host), Story: fmt.Sprintf("#%d", f),
			Seat: "engineering-manager", Host: e.Host, Identity: c.Identity(), Payload: map[string]any{"session": p.Session, "event": msg}},
			func(ctx context.Context) (actions.Outcome, error) {
				_, err := ad.FollowUp(ctx, delegate.State{Task: p.Session, TaskURL: p.URL}, msg)
				return actions.Outcome{Attempts: 1}, err
			})
	}
	wake.Status = wakeSent
	if err != nil {
		wake.Status = wakeNotSent + oneLineErr(err)
	}
	if _, err := updatePlan(ctx, w, f, fmt.Sprintf("missions: the tick's wake of the Kanha of #%d is %s", f, wake.Status), func(p *MissionPlan) bool {
		i := slices.IndexFunc(p.Wakes, func(wk MissionWake) bool { return wk.At == at && wk.Status == wakeSending })
		if i >= 0 {
			p.Wakes[i].Status = wake.Status
		}
		return i >= 0
	}); err != nil {
		wake.Status += "; left sending in " + MissionPlanPath(f) + ": " + oneLineErr(err)
	}
	rep.Woken = append(rep.Woken, wake)
	return ""
}

// lastTold is when the Kanha of Feature f was last told anything: its brief
// at Start, a tick's wake that may have reached it, or an event sent to it
// for one of its stories (delegate/<n>/state.json, #1007 and #1008).
func lastTold(f int, plan *MissionPlan, appr *ready.Approval, states map[int]delegate.State) time.Time {
	at, _ := time.Parse(time.RFC3339, appr.Parent.StartedAt)
	later := func(s string) {
		if t, err := time.Parse(time.RFC3339, s); err == nil && t.After(at) {
			at = t
		}
	}
	for _, wk := range plan.Wakes {
		if !strings.HasPrefix(wk.Status, wakeNotSent) {
			later(wk.At)
		}
	}
	for n, st := range states {
		if st.Feature != f && !slices.ContainsFunc(plan.Graph, func(it MissionItem) bool { return it.Number == n }) {
			continue
		}
		for _, ev := range st.Events {
			if ev.Status == kanhaEvent && !strings.Contains(ev.Detail, "("+wakeNotSent) {
				later(ev.At)
			}
		}
	}
	return at
}

// tickSync syncs each open cloud delegation of a receiver that does not run
// aayu (Codex, Cursor, Claude Code; an Amp thread ships its own, Story #1006)
// once, with `aayu build <n> --delegate <receiver>`: it finds the receiver's
// pull request, sends inspect's findings on a new head once, or reports
// merge-ready, which goes to the ship list. A sync never hands a story off:
// a delegation that failed, merged, or reached its cap is not synced, and
// only those are handed off afresh. A story of a paused or ended mission, or
// one a person stopped alone (#1002), is not synced either: a sync would send
// its receiver findings, or take a story an end sent back to Ready. Nor is a
// story whose mission plan cannot be read, as its pause or end is unknown.
// The plan is read again for each story, so a stop written during the tick
// holds every sync that has not started.
func (e *Env) tickSync(ctx context.Context, c *github.Client, w *ledger.Writer, repo string, states map[int]delegate.State, rep *TickReport) {
	for _, n := range slices.Sorted(maps.Keys(states)) {
		st := states[n]
		if st.Mode != "cloud" || st.Receiver == "" || st.Receiver == delegate.Amp || slices.Contains([]string{dlgMerged, dlgFailed, dlgCapped}, st.Status) {
			continue
		}
		// The plan is read just before this sync, so a stop written during the tick holds it. A delegation handed
		// off before its mission started records no Feature: its mission is found from the story's parent.
		if f := st.Feature; f != 0 {
			p, _, _, err := readMission(ctx, w, f)
			if err != nil || p == nil || p.State != "" || p.Pause != nil || p.stopped(n) != nil {
				continue // an unreadable or missing plan holds too: its pause or end is unknown
			}
		} else if _, p, _, err := storyMission(ctx, c, w, repo, n); err != nil ||
			p != nil && p.StartedBy != "" && (p.State != "" || p.Pause != nil || p.stopped(n) != nil) {
			continue
		}
		s := TickSync{Story: n, PR: st.PR, Receiver: st.Receiver, Command: fmt.Sprintf("aayu build %d --delegate %s", n, st.Receiver)}
		sub := *e
		sub.JSON = true
		var out bytes.Buffer
		sub.Out, sub.Err = &out, io.Discard
		err := runBuild(ctx, &sub, Values{"issue": strconv.Itoa(n), "delegate": st.Receiver, "sync-only": true,
			"asked": fmt.Sprintf("the %s loop's safety tick syncs the delegation of #%d to %s (aayu mission tick, loops/%s.toml)", BuildLoop, n, st.Receiver, BuildLoop)})
		var res BuildResult
		if json.NewDecoder(&out).Decode(&res) == nil && res.Delegation != nil {
			s.Status, s.Next, s.PR = res.Delegation.Status, res.Next, max(s.PR, res.Delegation.PR)
		}
		if err != nil {
			s.Error = oneLineErr(err)
		}
		rep.Synced = append(rep.Synced, s)
		if err == nil && s.Status == dlgReady {
			rep.Ship = append(rep.Ship, LoopItem{Story: n, PR: s.PR, URL: res.PRURL})
		}
	}
}

// readEvents adds to list the mission's events since its plan's
// events_cursor, oldest first, and then moves the cursor to the newest one.
// It runs after the list and its claims are written, so a run that fails
// before that leaves the cursor, and the next wake reads the same events. A
// dry run reads them and moves nothing. A cursor it cannot move is reported,
// and the events are read again at the next wake.
func (e *Env) readEvents(ctx context.Context, w *ledger.Writer, f int, plan *MissionPlan, subs []*gh.Issue, list *WorkList, dryRun bool) {
	since, _ := time.Parse(time.RFC3339, orDefault(plan.EventsCursor, plan.StartedAt))
	evs, err := missionEvents(ctx, w, subs, since)
	if err != nil {
		fmt.Fprintf(e.Err, "aayu: mission next: the events of #%d since %s are not read, and its cursor stays: %s\n", f, since.UTC().Format(time.RFC3339), oneLineErr(err))
		return
	}
	list.Events = evs
	if len(evs) == 0 || dryRun {
		return
	}
	to := evs[len(evs)-1].At
	if _, err := updatePlan(ctx, w, f, fmt.Sprintf("missions: the Kanha of #%d read its events to %s", f, to), func(p *MissionPlan) bool {
		cur, _ := time.Parse(time.RFC3339, orDefault(p.EventsCursor, p.StartedAt))
		next, _ := time.Parse(time.RFC3339, to)
		p.EventsCursor = to
		return next.After(cur)
	}); err != nil {
		fmt.Fprintf(e.Err, "aayu: mission next: the events cursor of #%d stays at %s, and the next wake reads these events again: %s\n", f, since.UTC().Format(time.RFC3339), oneLineErr(err))
	}
}

// missionEvents are the events of the mission's stories and Bugs (subs)
// after since, oldest first: each close on GitHub, and each line of each
// one's delegation history.
func missionEvents(ctx context.Context, w *ledger.Writer, subs []*gh.Issue, since time.Time) ([]MissionEvent, error) {
	t, err := w.Read(ctx)
	if err != nil {
		return nil, ignoreMissing(err)
	}
	defer t.Close()
	var evs []MissionEvent
	add := func(at time.Time, n int, what string) {
		if at.After(since) {
			evs = append(evs, MissionEvent{At: at.UTC().Format(time.RFC3339), Story: n, Event: what})
		}
	}
	for _, is := range subs {
		n := is.GetNumber()
		if is.GetState() == "closed" && is.ClosedAt != nil {
			add(is.GetClosedAt().Time, n, "closed")
		}
		b, err := t.File(ctx, delegate.LedgerPath(n))
		if err = ignoreMissing(err); err != nil {
			return nil, err
		}
		var st delegate.State
		if b == nil || json.Unmarshal(b, &st) != nil {
			continue
		}
		for _, ev := range st.Events {
			at, err := time.Parse(time.RFC3339, ev.At)
			if err == nil {
				add(at, n, strings.TrimSuffix(ev.Status+": "+ev.Detail, ": "))
			}
		}
	}
	slices.SortStableFunc(evs, func(a, b MissionEvent) int { return strings.Compare(a.At, b.At) })
	return evs, nil
}

// updatePlan changes Feature f's mission plan on the ledger, on the plan as
// it is now, and writes it when change says so; a ledger that moved between
// the read and the write is read again. It reports whether it wrote.
// A plan that is gone changes nothing: aayu never deletes one.
func updatePlan(ctx context.Context, w *ledger.Writer, f int, msg string, change func(*MissionPlan) bool) (bool, error) {
	for try := 1; ; try++ {
		plan, _, tip, err := readMission(ctx, w, f)
		if err != nil || plan == nil || !change(plan) {
			return false, err
		}
		data, _ := json.MarshalIndent(plan, "", "  ")
		_, err = w.Write(ctx, ledger.Commit{Message: msg, Files: map[string][]byte{MissionPlanPath(f): append(data, '\n')}, Base: tip})
		if !errors.Is(err, ledger.ErrMoved) || try == 3 { // the third move in a row is an error
			return err == nil, Wrap(ExitExternal, err)
		}
	}
}
