package cli

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/autonomy"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/identity"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

// StreakState is the autonomy streak of a shipped footprint (WP5.7),
// recorded in the ship record beside the pack.
type StreakState struct {
	Threshold  int            `json:"threshold"`
	Layers     map[string]int `json:"layers"` // streak per footprint layer
	Never      []string       `json:"never_layers,omitempty"`
	Autonomous bool           `json:"autonomous"`
	Error      string         `json:"error,omitempty"`
	Skipped    string         `json:"skipped,omitempty"` // why it was not computed: the human pass was already settled
}

// PassRule is who may give a story's human pass (WP5.5): the Engineering
// Manager seat's paired human, else in solo mode the [autonomy] approver,
// else any human who is not the author; and, when the footprint touches an
// [autonomy] never layer, a CODEOWNER of every changed path in it.
type PassRule struct {
	Approvers []string `json:"approvers,omitempty"` // empty: any human who is not the author
	As        string   `json:"as"`                  // seat, approver, or any-human
	Never     []string `json:"never_layers,omitempty"`
	Owners    []string `json:"codeowners,omitempty"` // logins that own every never-layer path
	NeverPath []string `json:"never_paths,omitempty"`
}

func passRule(cfg *config.Config) PassRule {
	if em := cfg.Seats[string(verbs.EngineeringManager)]; em.Mode == "paired" && em.Human != "" {
		return PassRule{Approvers: []string{em.Human}, As: "seat"}
	}
	if a := cfg.Autonomy.Approver; a != "" {
		return PassRule{Approvers: []string{a}, As: "approver"}
	}
	return PassRule{As: "any-human"}
}

// allows checks a review that is already a human pass (identity.HumanPass)
// against the rule.
func (p PassRule) allows(login string) error {
	if len(p.Approvers) > 0 && !slices.ContainsFunc(p.Approvers, func(a string) bool { return strings.EqualFold(a, login) }) {
		return fmt.Errorf("approval by %s is not the human pass: it must come from %s (%s)", login, strings.Join(p.Approvers, " or "), p.As)
	}
	if len(p.Never) > 0 && !slices.ContainsFunc(p.Owners, func(a string) bool { return strings.EqualFold(a, login) }) {
		return fmt.Errorf("approval by %s does not cover never layer(s) %s: a CODEOWNER of %s must approve (owners: %s), whatever the streak",
			login, strings.Join(p.Never, ", "), strings.Join(p.NeverPath, ", "), listOrNone(p.Owners))
	}
	return nil
}

// neverOwners fills in the never-layer paths of a PR's files and the logins
// that are CODEOWNERS of all of them, by the base branch's layers and
// CODEOWNERS at the policy's tip.
func (p *PassRule) neverOwners(ctx context.Context, c *github.Client, owner, name string, files []string, pol *shipPolicy) error {
	for _, f := range files {
		for _, l := range pol.Layers.Footprint([]string{f}) {
			if slices.Contains(p.Never, l) {
				p.NeverPath = append(p.NeverPath, f)
				break
			}
		}
	}
	text := ""
	if fc, _, _, err := c.REST.Repositories.GetContents(ctx, owner, name, ".github/CODEOWNERS", &gh.RepositoryContentGetOptions{Ref: pol.Tip}); err == nil {
		text, _ = fc.GetContent()
	} else if !github.NotFound(err) {
		return err
	}
	rules := identity.ParseCodeowners(text)
	for i, path := range p.NeverPath {
		owners, _, _ := identity.Owners(rules, path)
		if i == 0 {
			p.Owners = owners
			continue
		}
		p.Owners = slices.DeleteFunc(p.Owners, func(o string) bool { return !slices.Contains(owners, o) })
	}
	return nil
}

// shipStreak computes the footprint's autonomy streak from merged-PR history.
func (e *Env) shipStreak(ctx context.Context, c *github.Client, repo string, pol *shipPolicy, footprint, never []string) StreakState {
	a := pol.Config.Autonomy
	s := StreakState{Threshold: a.Streak, Layers: map[string]int{}, Never: never}
	rules := autonomy.Rules{Bots: pol.Config.Identity.BotLogins, Days: a.CorrectionDays, Layers: pol.Layers}
	prs, issues, err := autonomy.Fetch(ctx, c, repo, rules, autonomy.MaxPRs)
	if err != nil {
		s.Error = err.Error()
		return s
	}
	streaks := rules.Streaks(prs, rules.Detect(prs, issues), a.Streak, a.Never)
	for _, ls := range streaks {
		if slices.Contains(footprint, ls.Layer) {
			s.Layers[ls.Layer] = ls.Streak
		}
	}
	s.Autonomous = len(footprint) > 0 && len(never) == 0 && autonomy.Autonomous(streaks, footprint)
	return s
}

// loosens lists how this run's pass policy is looser than base's (#645),
// counting only what a layer above the defaults set (the checkout, the user
// config, AAYU_* variables, --set): a bot login, never layer or paired seat it
// drops, another approver, a cleared bot identity, a shorter streak or
// correction window, or proof turned on, widened, or with a smaller narrow set. A tighter run is no loosening; ship still judges by base.
func loosens(run *config.Loaded, base *config.Config) []string {
	set := func(keys ...string) bool {
		for _, k := range keys {
			if name, _ := run.Origin(k); name != "" && name != config.LayerDefault {
				return true
			}
		}
		return false
	}
	missing := func(have, want []string) []string {
		var out []string
		for _, w := range want {
			if !slices.ContainsFunc(have, func(h string) bool { return strings.EqualFold(h, w) }) {
				out = append(out, w)
			}
		}
		return out
	}
	r := &run.Config
	var out []string
	if m := missing(r.Identity.BotLogins, base.Identity.BotLogins); len(m) > 0 && set("identity.bot_logins") {
		out = append(out, "[identity] bot_logins drops "+strings.Join(m, ", "))
	}
	if identity.Configured(base.Identity) != "" && identity.Configured(r.Identity) == "" && set("identity.app_id", "identity.machine_user") {
		out = append(out, "[identity] has no bot identity")
	}
	if a := base.Autonomy.Approver; a != "" && !strings.EqualFold(r.Autonomy.Approver, a) && set("autonomy.approver") {
		out = append(out, fmt.Sprintf("[autonomy] approver is %q, not %q", r.Autonomy.Approver, a))
	}
	if m := missing(r.Autonomy.Never, base.Autonomy.Never); len(m) > 0 && set("autonomy.never") {
		out = append(out, "[autonomy] never drops "+strings.Join(m, ", "))
	}
	if x, b := r.Autonomy.Streak, base.Autonomy.Streak; x < b && set("autonomy.streak") {
		out = append(out, fmt.Sprintf("[autonomy] streak is %d, below %d", x, b))
	}
	if x, b := r.Autonomy.CorrectionDays, base.Autonomy.CorrectionDays; x < b && set("autonomy.correction_days") {
		out = append(out, fmt.Sprintf("[autonomy] correction_days is %d, below %d", x, b))
	}
	if r.Autonomy.Proof && !base.Autonomy.Proof && set("autonomy.proof") {
		out = append(out, "[autonomy] proof is on")
	}
	if m := missing(base.Autonomy.ProofRungs, r.Autonomy.ProofRungs); len(m) > 0 && set("autonomy.proof_rungs") {
		out = append(out, "[autonomy] proof_rungs adds "+strings.Join(m, ", "))
	}
	for _, k := range []struct {
		key        string
		run, based []string
	}{{"always_person", r.Autonomy.AlwaysPerson, base.Autonomy.AlwaysPerson}, {"always_person_config", r.Autonomy.AlwaysPersonConfig, base.Autonomy.AlwaysPersonConfig},
		{"always_person_sections", r.Autonomy.AlwaysPersonSections, base.Autonomy.AlwaysPersonSections}} {
		if m := missing(k.run, k.based); len(m) > 0 && set("autonomy."+k.key) {
			out = append(out, fmt.Sprintf("[autonomy] %s drops %s", k.key, strings.Join(m, ", ")))
		}
	}
	for _, slug := range slices.Sorted(maps.Keys(base.Seats)) {
		if b, x := base.Seats[slug], r.Seats[slug]; b.Mode == "paired" && b.Human != "" &&
			(x.Mode != "paired" && set("seats."+slug+".mode") || !strings.EqualFold(x.Human, b.Human) && set("seats."+slug+".human")) {
			out = append(out, fmt.Sprintf("[seats.%s] is not paired with %s", slug, b.Human))
		}
	}
	return out
}

func intersect(a, b []string) []string {
	var out []string
	for _, x := range a {
		if slices.Contains(b, x) {
			out = append(out, x)
		}
	}
	return out
}
