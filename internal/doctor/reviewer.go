package doctor

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
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
		return Result{Status: Warn, Detail: `[inspect] reviewer = "model": the merge gate calls the review lane's model, which needs an AI key in CI; product repositories name an installed agent app instead (reviewer = "codex", ...)`}
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
	case cfg.Reviewer == "" || cfg.Reviewer == "auto":
		return Result{Status: Fail, Detail: noReviewerChosen, Lines: lines}
	case cfg.Reviewer == inspect.ReviewerCI && len(missing) == 0:
		return Result{Status: Pass, Detail: fmt.Sprintf("merge-gate reviewer: ci, the model the %s environment names; the installed apps in [inspect] apps are its fallback", inspect.ReviewEnvironment), Lines: append(lines, reviewEnvSetup)}
	case len(missing) > 0:
		return Result{Status: Warn, Detail: fmt.Sprintf("reviewer apps detected but not configured: %s; add them to [inspect] apps (the gate reads config only)", strings.Join(missing, ", ")), Lines: lines}
	case c.App == "":
		return Result{Status: Warn, Detail: "no reviewer app configured: the merge gate falls back to a human review (install the Claude, Codex, or Cursor app on the org or account and add it to [inspect] apps)", Lines: lines}
	}
	return Result{Status: Pass, Detail: fmt.Sprintf("merge-gate reviewer: %s (%s); its review counts as different-family only for a builder of another family (a review service always does), and is labelled same-family otherwise", c.App, c.Family), Lines: lines}
}

// rungCNote is what a same-family review (rung C) is worth under a's rules:
// never the streak, and merge on proof only where proof is on and
// proof_rungs lists C (@rahulnakmol, 2026-10-18: "#657 a").
func rungCNote(a config.Autonomy) string {
	if a.Proof && slices.Contains(a.ProofRungs, inspect.RungC) {
		return "it never counts toward the streak, but [autonomy] proof is on and proof_rungs lists C, so a PR merges on proof after it (remove C to require A or B)"
	}
	return "it never counts toward the streak, so a person passes every PR"
}

// noReviewerChosen is doctor's failure, and its next step, when [inspect]
// reviewer names no tool (founder, 2026-10-18: "auto cannot be an option").
const noReviewerChosen = `no review tool chosen: the merge gate asks a person to review every PR; set [inspect] reviewer = "<tool>" (ci, the default; codex, coderabbit, greptile, claude, cursor, or human) in .aayudha/config.toml, or run aayu install`

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
		if r, err := (models.Resolver{Models: cfg.Models, Hosts: cfg.Hosts}).Resolve(verbs.LaneBuild, verbs.HostID(host)); err == nil {
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
	case model != "" && len(b.Candidates) == 1:
		builder = fmt.Sprintf("builder %s (%s on %s)", b.Candidates[0], model, host)
	case len(b.Candidates) > 0:
		builder = fmt.Sprintf("builder any of %v (%s picks the model for its modes)", b.Candidates, host)
	case host != "":
		builder = fmt.Sprintf("builder family unknown (%s declares no model aayu can place)", host)
	}
	lines := []string{builder}
	switch cfg.Inspect.Reviewer {
	case "", "auto":
		return Result{Status: Fail, Detail: noReviewerChosen, Lines: lines}
	case inspect.ReviewerHuman:
		return Result{Status: Pass, Detail: `human  a person reviews every PR ([inspect] reviewer = "human"); it counts toward autonomy`, Lines: lines}
	case inspect.ReviewerModel:
		return reviewModelRung(cfg.Models, cfg.Autonomy, b, lines)
	case inspect.ReviewerCI:
		// The model lives in the code-review environment, which doctor cannot
		// read; [inspect] paid_reviewer is the local stand-in.
		lines = append(lines, reviewEnvSetup)
		pr := cfg.Inspect.PaidReviewer
		if pr == "" {
			return Result{Status: Pass, Detail: fmt.Sprintf(`ci  the merge gate calls the model the %s environment names; doctor cannot read it, so the gate checks it on every PR and fails closed, naming what is missing`, inspect.ReviewEnvironment), Lines: lines}
		}
		f := verbs.FamilyOf(pr)
		if b.Rung(f) == inspect.RungA {
			return Result{Status: Pass, Detail: fmt.Sprintf("B  different-family review in CI: %s (%s) unless %s names another, within %.2f USD a PR; it counts toward autonomy", pr, f, inspect.ReviewModelVar, cfg.Budget.PerPRInspectMax), Lines: lines}
		}
		return Result{Status: Warn, Detail: fmt.Sprintf("%s (%s) cannot be shown to differ from the builder, so the gate never uses it: set %s to a model of another family; until then the installed apps, then a person, review", pr, f, inspect.ReviewModelVar), Lines: lines}
	}
	c := inspect.ChooseReviewer(cfg.Inspect.Reviewer, cfg.Inspect.Apps, cfg.Inspect.AppFamilies, b, cfg.Autonomy.Approver)
	// How to reach A: an app of a family the builder cannot be, added to
	// [inspect] apps, and chosen in [inspect] reviewer.
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
		// The chosen reviewer keeps reviewing, so the move names the switch.
		pick := ""
		if cfg.Inspect.Reviewer != a.Name {
			pick = fmt.Sprintf(`, and set [inspect] reviewer = %q`, a.Name)
		}
		switch {
		case a.Login == "" || b.Unknown():
		case f == verbs.FamilyHostManaged:
			up = append(up, fmt.Sprintf("%s: %sset [inspect] app_families.%s to the family it runs, outside the builder's%s", a.Name, add, a.Name, pick))
		case b.Rung(f) != inspect.RungA:
		case !listed:
			up = append(up, fmt.Sprintf("%s (%s): install it and add it to [inspect] apps%s", a.Name, f, pick))
		case pick != "":
			up = append(up, fmt.Sprintf(`%s (%s): already listed; set [inspect] reviewer = %q`, a.Name, f, a.Name))
		}
	}
	switch {
	case c.Rung == inspect.RungA:
	case b.Unknown():
		lines = append(lines, "move up: no reviewer can be shown to differ from an unknown builder; run doctor in the agent host (or pass --host), and declare Aayudha-Host and Aayudha-Model lines on PRs built outside aayu build")
	case len(up) > 0:
		lines = append(lines, "move up (free): "+strings.Join(up, "; "))
	}
	// Rung B (Decision #469; founder, 2026-10-18): opt-in, with a key the
	// repository adds; doctor cannot read repository secrets, so it names one.
	if pr := cfg.Inspect.PaidReviewer; c.Rung != inspect.RungA && !b.Unknown() {
		f := verbs.FamilyOf(pr)
		switch {
		case pr == "":
			lines = append(lines, `move up (paid): set [inspect] reviewer = "ci", the default, and `+reviewEnvSetup)
		case b.Rung(f) == inspect.RungA:
			lines = append(lines, fmt.Sprintf(`move up (paid): set [inspect] reviewer = "ci", the default, to have %s (%s) review every PR; today it reviews only when no installed app can`, pr, f))
		default:
			lines = append(lines, fmt.Sprintf("[inspect] paid_reviewer %s (%s) cannot be shown to differ from the builder, so the gate never uses it; pick a model of another family", pr, f))
		}
	}
	switch c.Rung {
	case inspect.RungA:
		return Result{Status: Pass, Detail: fmt.Sprintf("A  different-family review: %s (%s); it counts toward autonomy", c.App, c.Family), Lines: lines}
	case inspect.RungC:
		return Result{Status: Pass, Detail: fmt.Sprintf("C  same-family review: %s (%s), no other family reachable; %s", c.App, c.Family, rungCNote(cfg.Autonomy)), Lines: lines}
	}
	return Result{Status: Warn, Detail: "no reviewer app can review: a person reviews every PR", Lines: lines}
}

// reviewModelRung is the rung of [inspect] reviewer = "model": the review
// lane's direct model, chosen as inspect chooses it (internal/cli
// reviewerModels): a default of a family the builder may be gives way to the
// first peer candidate outside them; a configured one blocks every PR.
func reviewModelRung(cfg config.Models, a config.Autonomy, b inspect.Builder, lines []string) Result {
	m, err := (models.Resolver{Models: cfg}).ResolveDirect(verbs.LaneReview, verbs.DirectHost, false)
	if err != nil {
		return Result{Status: Fail, Detail: "model  the review lane resolves no model: " + err.Error(), Lines: lines}
	}
	if m.Family == "" || m.Family == verbs.FamilyHostManaged || slices.Contains(b.Candidates, m.Family) {
		i := slices.IndexFunc(verbs.PeerReviewCandidates, func(id string) bool {
			f := verbs.FamilyOf(id)
			return f != "" && !slices.Contains(b.Candidates, f)
		})
		if m.Source == models.SourceConfig || i < 0 {
			return Result{Status: Fail, Detail: fmt.Sprintf("model  the review lane's %s is of a family the builder may be (%v); inspect blocks every PR until [models] names a review model of another family", m.Model, b.Candidates), Lines: lines}
		}
		lines = append(lines, fmt.Sprintf("the review lane's %s is of a family the builder may be; inspect calls %s instead", m.Model, verbs.PeerReviewCandidates[i]))
		m.Model, m.Family = verbs.PeerReviewCandidates[i], verbs.FamilyOf(verbs.PeerReviewCandidates[i])
	}
	if b.Rung(m.Family) == inspect.RungA {
		return Result{Status: Pass, Detail: fmt.Sprintf("A  different-family review: the review lane's %s (%s), called through the AI gateway; it counts toward autonomy", m.Model, m.Family), Lines: lines}
	}
	return Result{Status: Pass, Detail: fmt.Sprintf("C  same-family review: the review lane's %s (%s) cannot be shown to differ from the builder; %s", m.Model, m.Family, rungCNote(a)), Lines: lines}
}

// reviewEnvSetup says where the CI review's model is set.
var reviewEnvSetup = fmt.Sprintf("set up the %s environment (Settings → Environments → %s, deployment branches: the base branches only): variables %s (the model endpoint's base URL, such as %s) and %s (the model ID, such as %s), secret %s (its key); `aayu init --gates` creates it",
	inspect.ReviewEnvironment, inspect.ReviewEnvironment, inspect.ReviewModelURLVar, inspect.RecommendedReviewURL, inspect.ReviewModelVar, inspect.RecommendedReviewModel, inspect.ReviewModelKeyVar)
