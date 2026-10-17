package evals

import (
	"context"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/cli"
)

// Evolve PRs (WP7.7) are eval-first: the change adds a case that reproduces an
// accepted Feedback finding on the old skill and passes on the new one.
const (
	EvolveBranchPrefix = cli.EvolveBranchPrefix
	EvolveLabel        = cli.EvolveLabel // applied to evolve PRs for people; IsEvolvePR does not read it
)

// Change is one changed path of a diff: its status (A, M, D, …) and, when
// the diff carried --numstat, the lines it added and deleted.
type Change struct {
	Status         string
	Path           string
	Stat           bool // Added and Deleted are known (a text file in a --numstat diff)
	Added, Deleted int
}

// ParseNameStatus reads `git diff --no-renames` output in --name-status form,
// or in --raw --numstat form (`aayu evolve` and CI), which adds line counts.
func ParseNameStatus(out string) []Change {
	var cs []Change
	at := map[string]int{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		head, p, ok := strings.Cut(line, "\t")
		if !ok || head == "" {
			continue
		}
		if f := strings.Fields(head); strings.HasPrefix(head, ":") && len(f) == 5 {
			head = f[4] // a --raw line: :mode mode sha sha status
		} else if added, err := strconv.Atoi(head); err == nil {
			deleted, path, _ := strings.Cut(p, "\t") // a --numstat line: added deleted path
			if i, ok := at[strings.TrimSpace(path)]; ok {
				if d, err := strconv.Atoi(deleted); err == nil {
					cs[i].Stat, cs[i].Added, cs[i].Deleted = true, added, d
				}
			}
			continue
		} else if head == "-" {
			continue // a binary file in --numstat
		}
		at[strings.TrimSpace(p)] = len(cs)
		cs = append(cs, Change{Status: head[:1], Path: strings.TrimSpace(p)})
	}
	return cs
}

// EvolveTrailer marks a commit `aayu evolve` made.
const EvolveTrailer = "Aayudha-Evolve:"

// IsEvolvePR reports whether a PR is an evolve PR: its head branch starts
// with evolve/ or one of its commits carries the Aayudha-Evolve trailer. Both
// change only with a push, which re-runs CI; a label (aayu:evolve is applied
// for people) can change without one, so it does not decide.
func IsEvolvePR(branch, messages string) bool {
	return strings.HasPrefix(branch, EvolveBranchPrefix) || strings.Contains(messages, "\n"+EvolveTrailer) || strings.HasPrefix(messages, EvolveTrailer)
}

var newCase = regexp.MustCompile(`^evals/cases/([^/]+)/([^/]+)/case\.toml$`)

// Before is the case as recorded on the old skill: every artifact under
// output/ is read from the case's reproduce directory instead.
func (c Case) Before() Case {
	b := c
	move := func(p string) string {
		if rest, ok := strings.CutPrefix(p, "output/"); ok {
			return path.Join(c.Reproduce, rest)
		}
		return p
	}
	b.Artifacts, b.Ready, b.Schemas = nil, nil, nil
	for _, a := range c.Artifacts {
		b.Artifacts = append(b.Artifacts, move(a))
	}
	for _, r := range c.Ready {
		b.Ready = append(b.Ready, move(r))
	}
	for _, s := range c.Schemas {
		b.Schemas = append(b.Schemas, Schema{File: move(s.File), Schema: s.Schema})
	}
	if c.Brief != nil {
		br := *c.Brief
		br.Draft = move(br.Draft)
		b.Brief = &br
	}
	b.ID = c.ID + " (" + c.Reproduce + ")"
	return b
}

// EvolveCase is a new eval case of an evolve change.
type EvolveCase = cli.EvolveCase

// Gate is EvolveGate over `git diff --name-status --no-renames` output, the
// form the binary injects into `aayu evolve` (cli.Options.EvolveGate).
func Gate(ctx context.Context, root, nameStatus string, assets fs.FS) ([]EvolveCase, []string) {
	return EvolveGate(ctx, root, ParseNameStatus(nameStatus), assets)
}

// EvolveGate is the eval-first gate (WP7.7) over an evolve change: it must add
// at least one eval case (evals/cases/<skill>/<name>/case.toml) that names the
// Feedback it reproduces and a reproduce directory holding the old skill's
// recorded run, the change must touch that skill (skills/<skill>/ or
// evals/skills/<skill>/), the old run must exist and validate against its
// schemas yet fail another tier-1 check (the case reproduces the finding), and
// the new run must pass tier 1 (the skill diff fixes it). It returns
// the cases and every problem; no problem means the gate passes.
func EvolveGate(ctx context.Context, root string, changes []Change, assets fs.FS) ([]EvolveCase, []string) {
	var cases []EvolveCase
	var problems []string
	for _, ch := range changes {
		m := newCase.FindStringSubmatch(ch.Path)
		if m == nil || ch.Status != "A" {
			continue
		}
		c, err := LoadCase(path.Join(root, path.Dir(ch.Path)))
		if err != nil {
			problems = append(problems, err.Error())
			continue
		}
		cases = append(cases, EvolveCase{ID: c.ID, Skill: c.Skill, Feedback: c.Feedback})
		bad := func(format string, a ...any) { problems = append(problems, c.ID+": "+fmt.Sprintf(format, a...)) }
		if c.Feedback <= 0 {
			bad("case.toml names no feedback = <issue> (the accepted Feedback it reproduces)")
		}
		skill := func(x Change) bool {
			return strings.HasPrefix(x.Path, "skills/"+c.Skill+"/") || strings.HasPrefix(x.Path, "evals/skills/"+c.Skill+"/")
		}
		if !slices.ContainsFunc(changes, skill) {
			bad("the change has no diff to skill %q (skills/%s/ or evals/skills/%s/)", c.Skill, c.Skill, c.Skill)
		} else if added, rewritten, counted := appendOnly(changes, skill); counted && !rewritten {
			bad("the skill diff only appends (%d lines added, none removed or rewritten): rewrite the guidance the Feedback is about "+
				"instead of adding a rule beside it", added)
		}
		if r := Tier1(ctx, root, c, assets); !r.Passed() {
			bad("the new skill's recorded run fails tier 1:\n%s", r)
		}
		if c.Reproduce == "" {
			bad("case.toml names no reproduce = <dir> (the old skill's recorded run, which must fail)")
			continue
		}
		r := Tier1(ctx, root, c.Before(), assets)
		var broken, failed []string
		for _, ch := range r.Checks {
			switch {
			case ch.Status != Fail:
			case ch.Name == "artifacts" || ch.Name == "schemas":
				broken = append(broken, ch.Name+": "+strings.Join(ch.Lines, "; "))
			default:
				failed = append(failed, ch.Name)
			}
		}
		switch {
		case len(broken) > 0:
			bad("the old skill's recorded run in %s/ is missing or malformed (%s); it must be a real run that fails on the finding", c.Reproduce, strings.Join(broken, "; "))
		case len(failed) == 0:
			bad("the old skill's recorded run in %s/ passes tier 1, so the case does not reproduce the finding", c.Reproduce)
		}
	}
	if len(cases) == 0 {
		problems = append([]string{"an evolve PR must add a new eval case (evals/cases/<skill>/<name>/case.toml) before any skill diff (eval-first, WP7.7); this one adds none"}, problems...)
	}
	return cases, problems
}

// appendOnly sums the skill's line counts: whether any change removed or
// rewrote an existing line (a deleted line, or a deleted file), and whether
// the diff carried line counts at all (a --name-status diff does not).
func appendOnly(changes []Change, skill func(Change) bool) (added int, rewritten, counted bool) {
	for _, x := range changes {
		if !skill(x) {
			continue
		}
		switch {
		case x.Status == "D":
			rewritten = true
		case x.Stat:
			counted = true
			added += x.Added
			rewritten = rewritten || x.Deleted > 0
		}
	}
	return added, rewritten, counted
}
