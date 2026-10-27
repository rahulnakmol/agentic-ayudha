package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"time"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/hosts"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/ready"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// ApproveReport is what `aayu ready <feature>` reports: Approve to build.
type ApproveReport struct {
	Repo         string          `json:"repo"`
	Feature      int             `json:"feature"`
	State        string          `json:"state"` // approved, or unchanged when the last approval agrees the Feature as it is
	Approval     *ready.Approval `json:"approval"`
	Plan         *MissionPlan    `json:"plan"` // missions/<f>/plan.json, written with the approval (Story #997)
	LedgerPath   string          `json:"ledger_path"`
	LedgerCommit string          `json:"ledger_commit,omitempty"`
	Ready        *ready.Result   `json:"ready,omitempty"` // the Feature's Ready snapshot, which Arjun posts after the approval
	DryRun       bool            `json:"dry_run,omitempty"`
}

// approveRefusal refuses Approve to build for an agent lane: a host's agent
// marker, aayu's own agent lanes (AAYU_AGENT=1: MCP and the loops), or CI.
// It is checked before anything is written.
func (e *Env) approveRefusal() error {
	return e.laneRefusal("Approve to build (aayu ready on a Feature)")
}

// laneRefusal refuses what, a person's decision, for an agent lane, naming
// the lane's variable but never its value; nil for a person.
func (e *Env) laneRefusal(what string) error {
	lane := ""
	switch m := hosts.AgentMarker(e.Getenv); {
	case m != "":
		lane, _, _ = strings.Cut(m, "=") // the variable, never its value
	case e.Getenv("AAYU_AGENT") == "1":
		lane = "AAYU_AGENT=1"
	case e.CI:
		lane = "--ci"
	default:
		return nil
	}
	return Errorf(ExitBlocked, "%s is a person's decision and is refused for an agent lane (%s)", what, lane)
}

// critique is #876's critique.State of Feature n. Until #876 merges no
// review or finding is recorded, so nothing waits (tests set Options.CritiqueState).
func (e *Env) critique(ctx context.Context, n int) (ready.Critique, error) {
	if e.CritiqueState != nil {
		return e.CritiqueState(ctx, n)
	}
	return ready.Critique{}, nil
}

// approveRun is one Approve to build of a Feature.
type approveRun struct {
	e *Env
	c *github.Client // the person's own client
	w *ledger.Writer // the person's ledger writer
	r *ready.Ready
	n int
	// milestones is --milestones: the mission's milestones the person names.
	milestones string
	// agents is --agents: how many stories the Feature's mission runs at once; 0 keeps the last approval's.
	agents int
}

// approveToBuild is `aayu ready <feature>` (Feature #877, Decision #903): a
// person approves the Feature's baselines, its stories, and its budget, once,
// through GitHub's device flow every time. Everything is read again after the
// code, and only what the person saw is approved. Then Arjun posts the
// Feature's Ready snapshot.
func (e *Env) approveToBuild(ctx context.Context, r *ready.Ready, n int, milestones string, agents int) error {
	c, tok, err := e.HumanGitHub(ctx)
	if err != nil {
		return err
	}
	w, err := e.Ledger(ctx, tok, "human", "")
	if err != nil {
		return err
	}
	r.GH = c
	a := &approveRun{e: e, c: c, w: w, r: r, n: n, milestones: milestones, agents: agents}
	want, effect, prev, _, err := a.prepare(ctx)
	if err != nil {
		return err
	}
	plan, planEffect, old, err := a.missionPlan(ctx, want.Agents)
	if err != nil {
		return err
	}
	rep := ApproveReport{Repo: r.Repo, Feature: n, LedgerPath: ready.ApprovalPath(n), DryRun: c.DryRun()}
	// Approved already, as it is now, with the same milestones: no new approval, and the snapshot is posted again if it is missing.
	if prev != nil && prev.Agreed(want) && old != nil && old.Approval == prev.Version && missionJSON(&MissionPlan{Milestones: old.Milestones}) == missionJSON(&MissionPlan{Milestones: plan.Milestones}) {
		rep.State, rep.Approval, rep.Plan = "unchanged", prev, old
		return a.ready(ctx, rep)
	}
	effect = append(effect, planEffect...)
	p, err := e.presenceCheck(ctx, Pending{What: "Approve to build", Effect: effect}, AccessWrite,
		Errorf(ExitBlocked, "Approve to build needs GitHub's device-flow presence check, which is not available here; run it where a person can enter the code"))
	if err != nil {
		return err
	}
	// The stories, the baselines, or the reviews can change while the code waits: read it all again.
	got, again, prev2, tip, err := a.prepare(ctx)
	if got.Stories != nil && !slices.Equal(got.Stories, want.Stories) {
		return Errorf(ExitBlocked, "The stories of #%d changed while the code was waiting; nothing was approved.", n)
	}
	if err != nil {
		return err
	}
	// A mission start while the code waits is in the tip the write is based on: the plan is read again, so it is kept.
	plan2, planEffect2, old2, err := a.missionPlan(ctx, got.Agents)
	if err != nil {
		return err
	}
	if !got.Agreed(want) || !slices.Equal(append(again, planEffect2...), effect) || version(prev2) != version(prev) || missionJSON(&plan2) != missionJSON(&plan) ||
		(old == nil) != (old2 == nil) || old != nil && missionJSON(old) != missionJSON(old2) {
		return Errorf(ExitBlocked, "The approval of #%d changed while the code was waiting; nothing was approved.", n)
	}
	got.Version, got.By, got.At = version(prev)+1, p.Login, e.Now().UTC().Format(time.RFC3339)
	got.Presence = ready.Presence{Login: p.Login, Via: p.Via, Permission: p.Permission, ActionsRecord: e.presenceRecord(p.Login)}
	plan.Approval, plan.ApprovedBy, plan.ApprovedAt = got.Version, got.By, got.At
	if old != nil { // a started mission keeps who started it, its tool, its budget, and its stories' starts (Decision #974)
		plan.StartedBy, plan.StartedAt, plan.Tool, plan.ToolSource = old.StartedBy, old.StartedAt, old.Tool, old.ToolSource
		plan.Budget, plan.BudgetBasis, plan.Pause, plan.Controls, plan.StoryStarts = old.Budget, old.BudgetBasis, old.Pause, old.Controls, old.StoryStarts
		plan.GateFixes = old.GateFixes
		for _, om := range old.Missions { // and each ended story's working total (Story #1015)
			for _, os := range om.Stories {
				for i, m := range plan.Missions {
					for j, s := range m.Stories {
						if s.Number == os.Number {
							plan.Missions[i].Stories[j].WorkingHours, plan.Missions[i].Stories[j].Run = os.WorkingHours, os.Run
						}
					}
				}
			}
		}
	}
	if prev2 != nil { // and its Kanha (Story #1004): a new approval starts no second one
		got.Parent = prev2.Parent
	}
	rep.State, rep.Approval, rep.Plan = "approved", &got, &plan
	if !c.DryRun() {
		data, _ := json.MarshalIndent(got, "", "  ")
		pdata, _ := json.MarshalIndent(plan, "", "  ")
		rep.LedgerCommit, err = w.Write(ctx, ledger.Commit{Message: fmt.Sprintf("specs: Approve to build of #%d, approval %d, by @%s, and its mission plan", n, got.Version, p.Login),
			Files: map[string][]byte{rep.LedgerPath: append(data, '\n'), MissionPlanPath(n): append(pdata, '\n')}, Base: tip})
		if errors.Is(err, ledger.ErrMoved) {
			return Errorf(ExitBlocked, "The ledger changed while aayu wrote the approval of #%d. Nothing was approved. Run it again.", n)
		}
		if err != nil {
			return Wrap(ExitExternal, err)
		}
	}
	return a.ready(ctx, rep)
}

func version(a *ready.Approval) int {
	if a == nil {
		return 0
	}
	return a.Version
}

// off is a refusal of Approve to build: the condition, and the next command.
func off(format string, args ...any) error {
	return Errorf(ExitBlocked, "Approve to build is off: "+format, args...)
}

// prepare reads the Feature as it is now and checks each condition of
// Approve to build: the approval it would write (without who and when), the
// lines a person approves, the last approval, and the ledger tip it read.
// The record's stories are set as soon as they are read, so a caller can
// tell a change in them from another refusal.
func (a *approveRun) prepare(ctx context.Context) (rec ready.Approval, effect []string, prev *ready.Approval, tip string, err error) {
	rec = ready.Approval{Schema: ready.ApprovalSchema, Feature: a.n, Baselines: []ready.ApprovedBaseline{}, Dismissals: []ready.Dismissal{}}
	if prev, tip, err = a.last(ctx); err != nil {
		return rec, nil, nil, "", err
	}
	// With no --agents, the Feature's agents from the last approval hold, as its milestones do.
	if rec.Agents = a.agents; rec.Agents == 0 && prev != nil {
		rec.Agents = prev.Agents
	}
	res, is, err := a.r.Check(ctx, a.n)
	if err != nil {
		return rec, nil, nil, "", Wrap(ExitExternal, err)
	}
	var ps []string
	for _, p := range res.Problems {
		if p.Field == "parent" {
			return rec, nil, nil, "", off("#%d has no Initiative parent (%s). Next: add #%d as a sub-issue of its Initiative, then aayu ready %d", a.n, p.Message, a.n, a.n)
		}
		ps = append(ps, p.String())
	}
	if len(ps) > 0 {
		return rec, nil, nil, "", off("#%d does not pass Ready: %s. Next: aayu ready %d --check", a.n, strings.Join(ps, "; "), a.n)
	}
	it, _ := workitems.Parse(a.n, is.GetTitle(), is.GetBody(), is.GetType().GetName(), github.IssueLabels(is))

	subs, err := featureStories(ctx, a.c, a.r.Repo, a.n)
	if err != nil {
		return rec, nil, nil, "", err
	}
	slices.SortFunc(subs, func(x, y *gh.Issue) int { return x.GetNumber() - y.GetNumber() })
	rec.Stories = []ready.ApprovedStory{}
	for _, si := range subs {
		s, _ := workitems.Parse(si.GetNumber(), si.GetTitle(), si.GetBody(), si.GetType().GetName(), github.IssueLabels(si))
		rec.Stories = append(rec.Stories, ready.StoryOf(s))
	}
	if len(rec.Stories) == 0 {
		return rec, nil, nil, "", off("#%d has no stories. Next: aayu slice %d", a.n, a.n)
	}

	var changes []string
	for _, part := range ready.Parts {
		ux := part == "ux"
		if ux && !slices.Contains(a.e.footprint(it), string(verbs.L01)) {
			continue
		}
		pr := &partRun{e: a.e, c: a.c, w: a.w, repo: a.r.Repo, n: a.n, part: part}
		base, draft, _, err := pr.records(ctx)
		if err != nil {
			return rec, nil, nil, "", err
		}
		if base == nil {
			need, next := "", fmt.Sprintf("aayu ready %d --part %s", a.n, part)
			if ux {
				need = " (L01)"
			}
			if draft == nil {
				next += " --request"
			}
			return rec, nil, nil, "", off("the %s is needed%s and not approved. Next: %s", ready.Name(part), need, next)
		}
		pins, err := pr.pins(ctx, it, is.GetBody())
		if CodeOf(err) == ExitBlocked {
			return rec, nil, nil, "", off("the %s v%d cannot be pinned now: %v", ready.Name(part), base.Version, err)
		} else if err != nil {
			return rec, nil, nil, "", err
		}
		if drift := ready.Diff(base.Pins, pins); len(drift) > 0 {
			ds := make([]string, len(drift))
			for i, d := range drift {
				ds[i] = d.String()
			}
			return rec, nil, nil, "", off("the %s changed after v%d was approved (%s). Next: aayu ready %d --part %s --request",
				ready.Name(part), base.Version, strings.Join(ds, " · "), a.n, part)
		}
		rec.Baselines = append(rec.Baselines, ready.ApprovedBaseline{Part: part, Version: base.Version, Hash: base.Hash})
		if prev == nil {
			continue
		}
		from := 0 // the version the last approval agreed
		for _, b := range prev.Baselines {
			if b.Part == part {
				from = b.Version
			}
		}
		for v := from + 1; v <= base.Version; v++ { // what changed in each version since: the person approves knowing why
			b := base
			if v < base.Version {
				if b, err = pr.version(ctx, v); err != nil {
					return rec, nil, nil, "", err
				}
			}
			for _, c := range b.Changes {
				changes = append(changes, ready.ChangeLine(part, v, c))
			}
		}
	}

	cs, err := a.e.critique(ctx, a.n)
	if err != nil {
		return rec, nil, nil, "", Wrap(ExitExternal, err)
	}
	l, err := a.e.Config()
	if err != nil {
		return rec, nil, nil, "", err
	}
	if len(cs.Open) > 0 {
		return rec, nil, nil, "", off("%s. Next: aayu grill feature %d --findings", ready.OpenGaps(cs.Open), a.n)
	}
	var waited string
	if missing := cs.Reviews - cs.In; missing > 0 {
		if a.e.Now().Sub(cs.AskedAt) < time.Duration(l.Config.Critique.ReviewWaitHours)*time.Hour {
			return rec, nil, nil, "", off("%d of %d reviews are not in; Approve to build opens when they are in or [critique] review_wait_hours have passed", missing, cs.Reviews)
		}
		waited = fmt.Sprintf("%d of %d reviews are not in, and [critique] review_wait_hours (%d) have passed", missing, cs.Reviews, l.Config.Critique.ReviewWaitHours)
	}
	if cs.Dismissed != nil {
		rec.Dismissals = cs.Dismissed
	}
	per := l.Config.Budget.PerStoryUSD
	rec.Budget = ready.Budget{PerStoryUSD: per, Stories: len(rec.Stories), TotalUSD: per * float64(len(rec.Stories))}

	effect = append([]string{ready.Effect(rec)}, changes...)
	if len(rec.Dismissals) > 0 {
		effect = append(effect, ready.Ratified(rec.Dismissals))
	}
	if waited != "" {
		effect = append(effect, waited)
	}
	return rec, effect, prev, tip, nil
}

// last reads the Feature's last approval, nil when it has none, and the
// ledger tip it read.
func (a *approveRun) last(ctx context.Context) (*ready.Approval, string, error) {
	t, err := a.w.Read(ctx)
	if err != nil { // no ledger yet: no approval either
		return nil, "", Wrap(ExitExternal, ignoreMissing(err))
	}
	defer t.Close()
	b, err := t.File(ctx, ready.ApprovalPath(a.n))
	if b == nil || err != nil {
		return nil, t.Tip, Wrap(ExitExternal, ignoreMissing(err))
	}
	var prev ready.Approval
	if err := json.Unmarshal(b, &prev); err != nil || prev.Schema != ready.ApprovalSchema || prev.Feature != a.n || prev.Version < 1 {
		return nil, "", Errorf(ExitExternal, "ledger %s: not an Approve to build record of #%d; nothing was approved", ready.ApprovalPath(a.n), a.n)
	}
	return &prev, t.Tip, nil
}

// version reads approved version v of the part.
func (r *partRun) version(ctx context.Context, v int) (*ready.Baseline, error) {
	p := ready.BaselinePath(r.n, r.part, v, false)
	files, err := r.ledgerFiles(ctx, p)
	if err != nil {
		return nil, err
	}
	var b ready.Baseline
	if err := json.Unmarshal(files[p], &b); err != nil || b.Check(r.n, r.part, v) != nil {
		return nil, Errorf(ExitExternal, "ledger %s: not %s v%d of #%d; nothing was approved", p, ready.Label(r.part), v, r.n)
	}
	return &b, nil
}

// presenceRecord is the hash of the newest presence.approve record of login
// in this machine's actions log: the record audit matches to the approval.
func (e *Env) presenceRecord(login string) string {
	b, _ := os.ReadFile(e.ActionsPath())
	hash := ""
	for line := range bytes.SplitSeq(bytes.TrimSpace(b), []byte("\n")) {
		var r actions.Record
		if json.Unmarshal(line, &r) == nil && r.Kind == "presence.approve" && r.Payload["approver"] == login {
			hash = r.Hash
		}
	}
	return hash
}

// ready has Arjun post the Feature's Ready snapshot and label it
// status:ready, as the bot, after the approval.
func (a *approveRun) ready(ctx context.Context, rep ApproveReport) error {
	if _, err := a.e.botReady(ctx, a.r); err != nil {
		return err
	}
	res, err := a.r.Apply(ctx, a.n)
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	rep.Ready = &res
	if !res.Pass {
		return Errorf(ExitBlocked, "#%d was approved to build, but no longer passes Ready; fix it, then run aayu ready %d again", a.n, a.n)
	}
	return a.e.Emit(rep, func(w io.Writer) error {
		next := fmt.Sprintf("Next: Arjun makes its %d stories Ready.", len(rep.Approval.Stories))
		if rep.State == "unchanged" {
			fmt.Fprintf(w, "#%d is approved to build already: approval %d by @%s agrees it as it is now. %s\n", a.n, rep.Approval.Version, rep.Approval.By, next)
		} else {
			fmt.Fprintf(w, "Approved #%d to build as @%s, approval %d. %s\n", a.n, rep.Approval.By, rep.Approval.Version, next)
		}
		switch {
		case rep.DryRun:
			_, err := fmt.Fprintf(w, "  --dry-run: ledger %s and %s, the Ready snapshot, and status:ready planned, not written\n", rep.LedgerPath, MissionPlanPath(a.n))
			return err
		case rep.LedgerCommit != "":
			fmt.Fprintf(w, "  ledger %s and %s (%s)\n", rep.LedgerPath, MissionPlanPath(a.n), short(rep.LedgerCommit))
		}
		_, err := fmt.Fprintf(w, "  Ready snapshot posted and labelled status:ready; ledger %s\n", res.LedgerPath)
		return err
	})
}
