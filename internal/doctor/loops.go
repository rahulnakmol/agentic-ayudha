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

// checkLoops reports this repository's loops and the agent tool that runs
// each, against loops/active.json (Appendix B S-7.1d, S-7.1e): active, stale
// (changed since it was set up), missing, or double (recorded in two tools,
// so it runs twice). One tool runs the loops.
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
	found := len(rec) > 0
	for _, h := range loops.AgentHosts {
		for _, bin := range agentCLIs[h] {
			if _, err := p.LookPath(bin); err == nil {
				found = true
			}
		}
	}
	if !found {
		return Result{Status: Warn, Detail: fmt.Sprintf("%d loops, and no agent tool here (amp, cursor-agent, claude, codex): only the Actions fallback runs them", len(ls))}
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
	owners := map[string]bool{}
	var lines []string
	for _, s := range loops.States(ls, rec, p.Repo, branch, tz) {
		count[s.Status]++
		if s.Status == loops.NoHost {
			continue
		}
		owner := s.Owner
		if owner == "" {
			owner = "-"
		} else {
			owners[owner] = true
		}
		line := fmt.Sprintf("%-18s %-12s %s", s.Loop, owner, s.Status)
		if s.Detail != "" {
			line += ": " + s.Detail
		}
		lines = append(lines, line)
	}
	detail := fmt.Sprintf("%d active, %d stale, %d missing, %d double", count[loops.Active], count[loops.Stale], count[loops.Missing], count[loops.Double])
	if len(owners) > 1 {
		lines = append(lines, "the loops are split across tools; one tool should run them all")
	}
	if count[loops.Stale]+count[loops.Missing]+count[loops.Double] == 0 && len(owners) <= 1 {
		return Result{Status: Pass, Detail: detail, Lines: lines}
	}
	lines = append(lines, "set them up in one tool with `aayu install --loops`")
	return Result{Status: Warn, Detail: detail, Lines: lines}
}
