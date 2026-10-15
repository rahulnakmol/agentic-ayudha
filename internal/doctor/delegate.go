package doctor

import (
	"context"
	"fmt"

	"github.com/rahulnakmol/agentic-ayudha/internal/delegate"
)

// checkDelegateEnv compares each delegation receiver's environment with the
// repository's one source, .agents/setup (WP5.5b): drift warns; a receiver
// that is not set up, or whose environment lives in its own UI (Codex), is
// listed but passes.
func checkDelegateEnv(_ context.Context, p *Probe) Result {
	if p.Root == "" {
		return Result{Status: Pass, Detail: "not in a repository"}
	}
	cs := delegate.CheckEnvironments(p.Root)
	if cs[0].Status == "missing" {
		return Result{Status: Pass, Detail: "delegation environments not set up: no " + delegate.SetupPath + " (cloud receivers of `build --delegate` install with it)"}
	}
	var lines []string
	drift := 0
	for _, c := range cs {
		lines = append(lines, fmt.Sprintf("%s: %s: %s", c.Receiver, c.Status, c.Detail))
		for _, l := range c.Lines {
			lines = append(lines, "  "+l)
		}
		if c.Status == "drift" {
			drift++
		}
	}
	if drift > 0 {
		return Result{Status: Warn, Detail: fmt.Sprintf("%d delegation environment(s) drifted from %s; `aayu doctor --fix` re-syncs %s", drift, delegate.SetupPath, delegate.CursorEnvPath), Lines: lines}
	}
	return Result{Status: Pass, Detail: "delegation environments follow " + delegate.SetupPath, Lines: lines}
}
