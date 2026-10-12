package cli

import (
	"context"
	"fmt"
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

// neverOwners fills in the never-layer paths of a PR and the logins that are
// CODEOWNERS of all of them on the base branch.
func (p *PassRule) neverOwners(ctx context.Context, c *github.Client, owner, name string, pr *gh.PullRequest, root string) error {
	files, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.CommitFile, *gh.Response, error) {
		return c.REST.PullRequests.ListFiles(ctx, owner, name, pr.GetNumber(), &o)
	})
	if err != nil {
		return err
	}
	layers := autonomy.ReadLayers(root)
	for _, f := range files {
		for _, l := range layers.Footprint([]string{f.GetFilename()}) {
			if slices.Contains(p.Never, l) {
				p.NeverPath = append(p.NeverPath, f.GetFilename())
				break
			}
		}
	}
	text := ""
	if fc, _, _, err := c.REST.Repositories.GetContents(ctx, owner, name, ".github/CODEOWNERS", &gh.RepositoryContentGetOptions{Ref: pr.GetBase().GetRef()}); err == nil {
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
func (e *Env) shipStreak(ctx context.Context, c *github.Client, repo, root string, cfg *config.Config, footprint, never []string) StreakState {
	a := cfg.Autonomy
	s := StreakState{Threshold: a.Streak, Layers: map[string]int{}, Never: never}
	rules := autonomy.Rules{Bots: cfg.Identity.BotLogins, Days: a.CorrectionDays, Layers: autonomy.ReadLayers(root)}
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

func intersect(a, b []string) []string {
	var out []string
	for _, x := range a {
		if slices.Contains(b, x) {
			out = append(out, x)
		}
	}
	return out
}
