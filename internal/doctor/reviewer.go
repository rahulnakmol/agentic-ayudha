package doctor

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/inspect"
)

// checkReviewerApps reports which agent apps can review this repository's
// pull requests in the merge gate (founder decision 2026-10-15: no AI key in
// a product repository's CI): those [inspect] apps names and those it
// detects, each app's family, and the reviewer the gate uses. A detected app
// that config does not name warns, since the gate reads config only.
func checkReviewerApps(ctx context.Context, p *Probe) Result {
	if p.Config == nil {
		return Result{Status: Pass, Detail: "no configuration loaded"}
	}
	cfg := p.Config.Config.Inspect
	if cfg.Reviewer == inspect.ReviewerModel {
		return Result{Status: Warn, Detail: `[inspect] reviewer = "model": the merge gate calls the review lane's model, which needs an AI key in CI; product repositories mention an installed agent app instead (reviewer = "auto")`}
	}
	var found []inspect.Detected
	if p.API != nil && p.Repo != "" {
		found = inspect.DetectApps(ctx, p.API, p.Repo)
	}
	var lines, missing []string
	for _, a := range inspect.Apps {
		fam := string(a.Family)
		if f := cfg.AppFamilies[a.Name]; f != "" {
			fam = f
		}
		state := "not detected"
		if i := slices.IndexFunc(found, func(d inspect.Detected) bool { return d.App == a.Name }); i >= 0 {
			state = "detected: " + found[i].Source
			if !slices.Contains(cfg.Apps, a.Name) && a.Login != "" {
				missing = append(missing, a.Name)
			}
		}
		if slices.Contains(cfg.Apps, a.Name) {
			state = "configured in [inspect] apps; " + state
		}
		lines = append(lines, fmt.Sprintf("%s (%s, family %s): %s", a.Name, a.Mention+orNone(a.Mention), fam, state))
		if a.Login == "" {
			lines = append(lines, "  "+a.Note)
		}
	}
	c := inspect.ChooseReviewer(cfg.Reviewer, cfg.Apps, cfg.AppFamilies, nil, p.Config.Config.Autonomy.Approver)
	lines = append(lines, c.Notes...)
	switch {
	case len(missing) > 0:
		return Result{Status: Warn, Detail: fmt.Sprintf("reviewer apps detected but not configured: %s; add them to [inspect] apps (the gate reads config only)", strings.Join(missing, ", ")), Lines: lines}
	case c.App == "":
		return Result{Status: Warn, Detail: "no reviewer app configured: the merge gate falls back to a human review (install the Claude, Codex, or Cursor app on the org or account and add it to [inspect] apps)", Lines: lines}
	}
	return Result{Status: Pass, Detail: fmt.Sprintf("merge-gate reviewer: %s (%s), unless it is of the builder's family; then the next app, else a human", c.App, c.Family), Lines: lines}
}

func orNone(s string) string {
	if s == "" {
		return "no mention"
	}
	return ""
}
