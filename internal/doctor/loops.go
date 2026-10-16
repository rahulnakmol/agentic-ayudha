package doctor

import (
	"context"
	"fmt"

	"github.com/rahulnakmol/agentic-ayudha/internal/loops"
)

// agentCLIs are the CLIs that show a repository's user has an agent host.
var agentCLIs = map[string][]string{
	loops.HostAmp:    {"amp"},
	loops.HostCursor: {"cursor-agent", "agent"},
	loops.HostClaude: {"claude"},
	loops.HostCodex:  {"codex"},
}

// checkLoops reports this repository's loops, scheduled and event alike, on
// each agent host its user has (the host's CLI is on PATH, or a loop is
// recorded there): active, stale, or missing against loops/active.json
// (Appendix B S-7.1d). A loop whose event a host has no trigger for shows as
// session there: the user's next session picks the event up (D-7.1e).
func checkLoops(_ context.Context, p *Probe) Result {
	if p.Root == "" {
		return Result{Status: Pass, Detail: "no checkout"}
	}
	ls, err := loops.Load(p.Root)
	if err != nil {
		return Result{Status: Fail, Detail: err.Error()}
	}
	if len(ls) == 0 {
		return Result{Status: Pass, Detail: "no loops/*.toml"}
	}
	rec, err := loops.ReadRecord(p.Root)
	if err != nil {
		return Result{Status: Fail, Detail: err.Error()}
	}
	var hosts []string
	for _, h := range loops.AgentHosts {
		found := len(rec[h]) > 0
		for _, bin := range agentCLIs[h] {
			if _, err := p.LookPath(bin); err == nil {
				found = true
			}
		}
		if found {
			hosts = append(hosts, h)
		}
	}
	if len(hosts) == 0 {
		return Result{Status: Warn, Detail: fmt.Sprintf("%d loops, and no agent host here (amp, cursor-agent, claude, codex): only the Actions fallback runs them", len(ls))}
	}
	branch, tz := "dev", "UTC"
	if p.Config != nil {
		if b := p.Config.Config.Branches.Dev; b != "" {
			branch = b
		}
		if z := p.Config.Config.Time.TZ; z != "" {
			tz = z
		}
	}
	count := map[string]int{}
	var lines []string
	for _, s := range loops.States(ls, rec, hosts, p.Repo, branch, tz) {
		count[s.Status]++
		if s.Status == loops.NoHost {
			continue
		}
		line := fmt.Sprintf("%-7s %-18s %s", s.Host, s.Loop, s.Status)
		if s.Detail != "" {
			line += ": " + s.Detail
		}
		lines = append(lines, line)
	}
	detail := fmt.Sprintf("%d active, %d stale, %d missing, %d by the next session on %v", count[loops.Active], count[loops.Stale], count[loops.Missing], count[loops.Session], hosts)
	if count[loops.Stale]+count[loops.Missing] == 0 {
		return Result{Status: Pass, Detail: detail, Lines: lines}
	}
	lines = append(lines, "set them up with `aayu install --loops`, then record each with --record")
	return Result{Status: Warn, Detail: detail, Lines: lines}
}
