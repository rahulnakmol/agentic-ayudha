package doctor

import (
	"context"
	"fmt"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/agentenv"
)

// checkAgentEnv reports each agent host's environment as verified, stale, or
// missing (agentenv.Status): stale or unverified environments warn, since a
// cloud hand-off would start from them.
func checkAgentEnv(_ context.Context, p *Probe) Result {
	if p.Root == "" {
		return Result{Status: Pass, Detail: "not in a repository"}
	}
	sts, err := agentenv.Status(p.Root)
	if err != nil {
		return Result{Status: Warn, Detail: err.Error()}
	}
	if _, ok, _ := agentenv.LoadSpec(p.Root); !ok {
		return Result{Status: Pass, Detail: "agent environments not set up: no " + agentenv.SpecPath + "; `aayu install --env` discovers one from the code"}
	}
	var lines []string
	n := map[string]int{}
	for _, s := range sts {
		n[s.State]++
		live := ""
		if s.Live {
			live = ", live"
		}
		lines = append(lines, fmt.Sprintf("%s: %s%s: %s", s.Host, s.State, live, s.Detail))
		for _, l := range s.Lines {
			lines = append(lines, "  "+l)
		}
	}
	detail := fmt.Sprintf("%d verified, %d stale, %d missing", n[agentenv.Verified], n[agentenv.Stale], n[agentenv.Missing])
	if n[agentenv.Stale]+n[agentenv.Missing] > 0 {
		return Result{Status: Warn, Detail: detail + "; `aayu install --env --apply --verify` refreshes and verifies them", Lines: lines}
	}
	return Result{Status: Pass, Detail: detail, Lines: lines}
}

// checkAgents lists the agent CLIs and cloud connections on this machine
// (agentenv.Detect, offline), with the exact commands for what is missing.
func checkAgents(ctx context.Context, p *Probe) Result {
	if p.Combined == nil || p.LookPath == nil {
		return Result{Status: Pass, Detail: "not probed"}
	}
	pr := agentenv.Probe{Root: p.Root, Repo: p.Repo, LookPath: p.LookPath, Exec: p.Combined, Getenv: p.Getenv, CursorSecret: "cursor-api-key"}
	if p.Config != nil {
		pr.CodexEnv, pr.CursorSecret = p.Config.Config.Delegate.CodexEnv, p.Config.Config.Delegate.CursorSecret
	}
	if p.Secrets != nil {
		pr.HasSecret = func(name string) bool { v, _, err := p.Secrets.Get(name); return err == nil && v != "" }
	}
	var lines, local, cloud []string
	for _, a := range agentenv.Detect(ctx, pr) {
		state := "not installed"
		if a.CLI != "" {
			state = a.CLI + " " + a.Version
			if a.SignedIn && a.Via != "" {
				state += ", runs through " + strings.TrimPrefix(a.Via, "the ")
				local = append(local, a.Name)
			} else if a.SignedIn {
				state += ", signed in"
				local = append(local, a.Name)
			} else {
				state += ", not signed in"
			}
		}
		if a.Cloud {
			cloud = append(cloud, a.Name)
		}
		lines = append(lines, fmt.Sprintf("%s: %s; cloud: %s", a.Name, state, a.CloudDetail))
		switch {
		case a.CLI == "":
			lines = append(lines, "  install: "+strings.Join(a.Install, "; "))
		case !a.SignedIn:
			lines = append(lines, "  sign in: "+strings.Join(a.SignIn, "; "))
		}
	}
	return Result{Status: Pass, Detail: fmt.Sprintf("local CLIs signed in: %s; cloud agents connected: %s", joinOrNone(local), joinOrNone(cloud)), Lines: lines}
}

func joinOrNone(xs []string) string {
	if len(xs) == 0 {
		return "none"
	}
	return strings.Join(xs, ", ")
}
