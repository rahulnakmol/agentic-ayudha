package doctor

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/inspect"
	"github.com/rahulnakmol/agentic-ayudha/internal/models"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
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
	c := inspect.ChooseReviewer(cfg.Reviewer, cfg.Apps, cfg.AppFamilies, inspect.Builder{}, p.Config.Config.Autonomy.Approver)
	lines = append(lines, c.Notes...)
	switch {
	case len(missing) > 0:
		return Result{Status: Warn, Detail: fmt.Sprintf("reviewer apps detected but not configured: %s; add them to [inspect] apps (the gate reads config only)", strings.Join(missing, ", ")), Lines: lines}
	case c.App == "":
		return Result{Status: Warn, Detail: "no reviewer app configured: the merge gate falls back to a human review (install the Claude, Codex, or Cursor app on the org or account and add it to [inspect] apps)", Lines: lines}
	}
	return Result{Status: Pass, Detail: fmt.Sprintf("merge-gate reviewer: %s (%s); its review counts as different-family only for a builder of another family, and is labelled same-family otherwise", c.App, c.Family), Lines: lines}
}

func orNone(s string) string {
	if s == "" {
		return "no mention"
	}
	return ""
}

// checkReviewRung is the review ladder's rung the PRs built here get
// (Decision #469), and how to move up. The builder is this host's build
// lane: its model's family, or every family a host-managed mode may run.
// Nothing is called (detail 11): an installed app's family is the configured
// one, confirmed only by the model its reply reports.
func checkReviewRung(_ context.Context, p *Probe) Result {
	if p.Config == nil {
		return Result{Status: NA, Detail: "no configuration loaded"}
	}
	cfg := p.Config.Config
	host, _ := DetectHost(p.Host, p.Getenv)
	var claims, hosts []string
	lane, model := verbs.Family(""), ""
	if host != "" {
		if r, err := (models.Resolver{Models: cfg.Models}).Resolve(verbs.LaneBuild, verbs.HostID(host)); err == nil {
			lane, model = r.Family, r.Model
			if model != "" {
				claims = []string{model}
			} else {
				hosts = []string{host}
			}
		}
	}
	b := inspect.BuilderOf(inspect.Records{}, nil, claims, hosts, lane)
	builder := "builder family unknown (no agent host detected; pass --host)"
	switch {
	case b.Family != "" && b.Family != verbs.FamilyHostManaged:
		builder = fmt.Sprintf("builder %s (%s on %s)", b.Family, model, host)
	case len(b.Candidates) > 0:
		builder = fmt.Sprintf("builder any of %v (%s picks the model for its modes)", b.Candidates, host)
	case host != "":
		builder = fmt.Sprintf("builder family unknown (%s declares no model aayu can place)", host)
	}
	c := inspect.ChooseReviewer(cfg.Inspect.Reviewer, cfg.Inspect.Apps, cfg.Inspect.AppFamilies, b, cfg.Autonomy.Approver)
	lines := []string{builder}
	if cfg.Inspect.Reviewer == inspect.ReviewerHuman {
		return Result{Status: Pass, Detail: `human  a person reviews every PR ([inspect] reviewer = "human"); it counts toward autonomy`, Lines: lines}
	}
	// How to reach A: an app of a family the builder cannot be, added to
	// [inspect] apps, and picked (auto picks it; a fixed reviewer does not).
	var up []string
	for _, a := range inspect.Apps {
		f := a.Family
		if v := verbs.Family(cfg.Inspect.AppFamilies[a.Name]); v != "" {
			f = v
		}
		listed := slices.Contains(cfg.Inspect.Apps, a.Name)
		add := ""
		if !listed {
			add = "install it and add it to [inspect] apps, then "
		}
		switch {
		case a.Login == "" || b.Unknown():
		case f == verbs.FamilyHostManaged:
			up = append(up, fmt.Sprintf("%s: %sset [inspect] app_families.%s to the family it runs, outside the builder's", a.Name, add, a.Name))
		case b.Rung(f) != inspect.RungA:
		case !listed:
			up = append(up, fmt.Sprintf("%s (%s): install it and add it to [inspect] apps", a.Name, f))
		case cfg.Inspect.Reviewer != "" && cfg.Inspect.Reviewer != inspect.ReviewerAuto:
			up = append(up, fmt.Sprintf(`%s (%s): already listed; set [inspect] reviewer = "auto" or %q`, a.Name, f, a.Name))
		}
	}
	switch {
	case c.Rung == inspect.RungA:
	case b.Unknown():
		lines = append(lines, "move up: no reviewer can be shown to differ from an unknown builder; run doctor in the agent host (or pass --host), and declare Aayudha-Host and Aayudha-Model lines on PRs built outside aayu build")
	case len(up) > 0:
		lines = append(lines, "move up (free): "+strings.Join(up, "; "))
	}
	switch c.Rung {
	case inspect.RungA:
		return Result{Status: Pass, Detail: fmt.Sprintf("A  different-family review: %s (%s); it counts toward autonomy", c.App, c.Family), Lines: lines}
	case inspect.RungC:
		return Result{Status: Pass, Detail: fmt.Sprintf("C  same-family review: %s (%s), no other family reachable; it never counts toward autonomy, so a person passes every PR", c.App, c.Family), Lines: lines}
	}
	return Result{Status: Warn, Detail: "no reviewer app can review: a person reviews every PR", Lines: lines}
}
