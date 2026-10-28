package cli

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
)

// MissionHostWake is one `aayu mission wake` on an event host, in the plan
// (Story #1012, J3 and K2): it started the next stories, or it only recorded
// them, and why. The mission's Kanha starts what a wake only recorded.
type MissionHostWake struct {
	At      string `json:"at"`
	Host    string `json:"host"`
	Result  string `json:"result"` // started or record-only
	Reason  string `json:"reason,omitempty"`
	Started []int  `json:"started,omitempty"`
	Waiting []int  `json:"waiting,omitempty"` // the next stories it did not start
}

// Host wake results.
const (
	hostWakeStarted    = "started"
	hostWakeRecordOnly = "record-only"
	hostWakeChecked    = "checked" // the check of a local Kanha on its own machine
)

// MissionWakeReport is what `aayu mission wake <f>` reports.
type MissionWakeReport struct {
	Repo    string `json:"repo"`
	Feature int    `json:"feature"`
	Result  string `json:"result"`
	Note    string `json:"note"`
	Started []int  `json:"started,omitempty"`
	Waiting []int  `json:"waiting,omitempty"`
	Ledger  string `json:"ledger"`
}

// missionWake is `aayu mission wake <f>`, stateless: everything it needs is
// on the ledger and GitHub. On the machine of f's local Kanha it is that
// Kanha's check, every [mission] check_minutes. Elsewhere it is an event
// host's wake (a Claude scheduled task or a Cursor Automation on a merged
// PR): on the mission's tool, with that tool's cloud agent connected, it
// starts the next stories as the Kanha would, with the session's own GitHub
// sign-in; else, or when a start fails, it only records the next stories in
// the plan, and the mission's Kanha starts them.
func missionWake(ctx context.Context, e *Env, f int) error {
	repo, c, w, err := e.missionLedger(ctx)
	if err != nil {
		return err
	}
	plan, appr, _, err := readMission(ctx, w, f)
	if err != nil {
		return err
	}
	if plan == nil || plan.StartedBy == "" || appr == nil {
		return WithNext(Errorf(ExitBlocked, "The mission of #%d has not started: a person starts it. Nothing starts.", f), fmt.Sprintf("aayu mission start %d", f))
	}
	rep := MissionWakeReport{Repo: repo, Feature: f, Ledger: MissionPlanPath(f)}
	if p := appr.Parent; p != nil && localHere(p) {
		rep.Result = hostWakeChecked
		if rep.Note, err = e.checkLocalKanha(ctx, c, w, repo, f, false); err != nil {
			return err
		}
		return emitWake(e, rep)
	}
	tool, err := e.callerTool(ctx, repo)
	if err != nil {
		return err
	}
	why := ""
	if tool != plan.Tool {
		why = fmt.Sprintf("this session runs on %s, and the mission runs on %s", orDefault(tool, "no known tool"), plan.Tool)
	} else if a := e.agents(ctx, config.RepoRoot(e.Dir), repo)[tool]; !a.Cloud {
		why = fmt.Sprintf("%s's cloud agent is not connected here: %s", tool, a.CloudDetail)
	}
	if why == "" {
		var work WorkList
		e.work, e.keepEvents = &work, true // the Kanha has not read these events: its cursor stays
		err := missionNext(ctx, e, f)
		e.work, e.keepEvents = nil, false
		if err != nil {
			why = "mission next: " + oneLineErr(err)
		}
		out := e.Out
		e.Out = io.Discard // each build's own report; the wake reports what started
		for _, it := range work.Items {
			if it.Action != "start" {
				continue
			}
			n := issueNumber(it.Key)
			if err := handlers["build"](ctx, e, Values{"issue": strconv.Itoa(n), "delegate": tool}); err != nil {
				rep.Waiting = append(rep.Waiting, n)
				why = strings.TrimPrefix(why+"; ", "; ") + fmt.Sprintf("start of #%d failed: %s", n, oneLineErr(err))
				continue
			}
			rep.Started = append(rep.Started, n)
		}
		e.Out = out
	}
	rep.Result, rep.Note = hostWakeStarted, fmt.Sprintf("started %s", orDefault(issueRefs(rep.Started), "nothing: no story can start now"))
	if why != "" {
		rep.Result, rep.Note = hostWakeRecordOnly, "recorded only: "+why+"; the mission's Kanha starts the next stories"
	}
	wk := MissionHostWake{At: e.Now().UTC().Format(time.RFC3339), Host: tool, Result: rep.Result, Reason: why, Started: rep.Started, Waiting: rep.Waiting}
	if _, err := writePlan(ctx, w, f, fmt.Sprintf("missions: a wake on %s for #%d: %s", orDefault(tool, "an unknown tool"), f, rep.Result), func(pl *MissionPlan) error {
		pl.HostWakes = append(pl.HostWakes, wk)
		pl.HostWakes = pl.HostWakes[max(0, len(pl.HostWakes)-keptWakes):]
		return nil
	}); err != nil {
		return err
	}
	return emitWake(e, rep)
}

func emitWake(e *Env, rep MissionWakeReport) error {
	return e.Emit(rep, func(w io.Writer) error {
		_, err := fmt.Fprintf(w, "#%d wake: %s\n", rep.Feature, rep.Note)
		return err
	})
}
