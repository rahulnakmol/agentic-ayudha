package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/audit"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

func init() { register("evolve", runEvolve) }

// EvolveResult is what evolve reports.
type EvolveResult struct {
	Feedback int          `json:"feedback"`
	Skill    string       `json:"skill"`
	Worktree string       `json:"worktree"`
	Branch   string       `json:"branch"`
	Created  bool         `json:"created,omitempty"`
	Cases    []EvolveCase `json:"cases,omitempty"`
	Commit   string       `json:"commit,omitempty"`
	PR       int          `json:"pr,omitempty"`
	Next     string       `json:"next,omitempty"`
	Evidence []int        `json:"evidence"` // the accepted retro Feedback issues that name the skill
}

// The evolve branch prefix (internal/evals.IsEvolvePR reads it, with the
// Aayudha-Evolve trailer) and the label applied to evolve PRs for people.
const (
	EvolveBranchPrefix = "evolve/"
	EvolveLabel        = "aayu:evolve"
)

var skillName = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// P4 of the private research note video-learnings-2026-10.md (Appendix B S-7.7c): a
// skill changes only on evidence from at least two accepted Feedback issues,
// and at most once a month.
const (
	EvolveMinFeedback  = 2
	EvolveCooldownDays = 30
)

// runEvolve is `aayu evolve` (WP7.7): accepted retro Feedback about a skill
// becomes, first, an eval case that reproduces it on the old skill, then a
// skill diff that passes it, opened as an agent-authored PR that a human
// CODEOWNER reviews; evolve never merges. The first run cuts a worktree from
// origin/dev; the next, after the host wrote the case and the diff there, runs
// the eval-first gate (the same one CI runs), commits, pushes, and opens the PR.
func runEvolve(ctx context.Context, e *Env, in Values) error {
	root := config.RepoRoot(e.Dir)
	if root == "" {
		return Errorf(ExitUsage, "evolve runs in a checkout of the repository")
	}
	if !in.Has("feedback") {
		return listEvolve(ctx, e)
	}
	skill, n := in.String("skill"), issueNumber(in.String("feedback"))
	if !skillName.MatchString(skill) {
		return Errorf(ExitUsage, "evolve --skill %q: want a skill directory name", skill)
	}
	repo, err := e.RepoName(ctx)
	if err != nil {
		return err
	}
	c, bot, err := e.BotGitHub(ctx, "")
	if err != nil {
		return err
	}
	owner, name, _ := github.SplitRepo(repo)
	is, _, err := c.REST.Issues.Get(ctx, owner, name, n)
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	labels := github.IssueLabels(is)
	switch {
	case !slices.Contains(labels, "type:feedback") || !strings.HasPrefix(is.GetTitle(), audit.FeedbackPrefix):
		return Errorf(ExitBlocked, "evolve: #%d is not a Feedback issue filed by `aayu retro` (title %q)", n, is.GetTitle())
	case is.GetState() != "open":
		return Errorf(ExitBlocked, "evolve: Feedback #%d is closed", n)
	case !slices.Contains(labels, workitems.LabelNeedsGrill):
		return Errorf(ExitBlocked, "evolve: Feedback #%d is not accepted (a human accepts it with the %s label)", n, workitems.LabelNeedsGrill)
	}
	all, err := acceptedFeedback(ctx, c, repo)
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	var evidence []int
	for _, x := range all {
		if x.Feedback == n && x.Skill == skill && x.Refused != "" {
			return Errorf(ExitBlocked, "evolve: %s", x.Refused)
		}
		if x.Skill == skill && x.Refused == "" {
			evidence = append(evidence, x.Feedback)
		}
	}
	switch {
	case !slices.Contains(evidence, n):
		return Errorf(ExitBlocked, "evolve: Feedback #%d does not name skill `%s` in its title", n, skill)
	case len(evidence) < EvolveMinFeedback:
		return Errorf(ExitBlocked, "evolve: a skill changes only on at least %d accepted retro Feedback issues that name it and are prose corrections; skill `%s` has #%d only", EvolveMinFeedback, skill, n)
	}
	dev := e.devBranch()
	res := EvolveResult{Evidence: evidence, Feedback: n, Skill: skill, Worktree: filepath.Join(root, ".aayudha", "worktrees", fmt.Sprintf("evolve-%s-%d", skill, n)), Branch: fmt.Sprintf("%s%s-%d", EvolveBranchPrefix, skill, n)}
	if pr, err := recentEvolvePR(ctx, c, repo, skill, res.Branch, e.Now()); err != nil {
		return Wrap(ExitExternal, err)
	} else if pr != nil {
		return Errorf(ExitBlocked, "evolve: PR #%d proposed a change to skill `%s` on %s; a skill gets at most one evolve PR in %d days (P4)",
			pr.GetNumber(), skill, pr.GetCreatedAt().Format(time.DateOnly), EvolveCooldownDays)
	}
	if _, err := os.Stat(res.Worktree); err != nil {
		if _, err := run2(ctx, root, "git", "fetch", "--quiet", "origin", dev); err != nil {
			return Wrap(ExitExternal, err)
		}
		if _, err := run2(ctx, root, "git", "worktree", "add", "--quiet", "-b", res.Branch, res.Worktree, "origin/"+dev); err != nil {
			return Wrap(ExitExternal, err)
		}
		res.Created = true
		res.Next = fmt.Sprintf("in %s, first add evals/cases/%s/<name>/ with feedback = %d and reproduce = \"before\" (before/ holds the current skill's recorded run, "+
			"which must fail tier 1), then change the skill so output/ passes; run `aayu evolve --skill %s --feedback %d` again", res.Worktree, skill, n, skill, n)
		return e.Emit(res, func(w io.Writer) error {
			_, err := fmt.Fprintf(w, "evolve %s for #%d: worktree %s on %s\n  next: %s\n", skill, n, res.Worktree, res.Branch, res.Next)
			return err
		})
	}
	if _, err := run2(ctx, res.Worktree, "git", "add", "--all"); err != nil {
		return Wrap(ExitExternal, err)
	}
	diff, err := run2(ctx, res.Worktree, "git", "diff", "--cached", "--raw", "--numstat", "--no-renames", "origin/"+dev)
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	if e.EvolveGate == nil {
		return Errorf(ExitInternal, "evolve: this binary was built without the eval harness (cli.Options.EvolveGate)")
	}
	cases, problems := e.EvolveGate(ctx, res.Worktree, diff, e.Assets)
	if len(cases) > 0 && !slices.ContainsFunc(cases, func(x EvolveCase) bool { return x.Feedback == n && x.Skill == skill }) {
		problems = append(problems, fmt.Sprintf("no new case is for skill %s and Feedback #%d", skill, n))
	}
	if len(problems) > 0 {
		return Errorf(ExitBlocked, "evolve: the eval-first gate refuses this change; nothing committed:\n  %s", strings.Join(problems, "\n  "))
	}
	res.Cases = cases
	if staged, _ := run2(ctx, res.Worktree, "git", "diff", "--cached", "--name-only"); staged != "" {
		msg := fmt.Sprintf("feat(skills): evolve %s for Feedback #%d (WP7.7)\n\nAn eval case reproduces the finding on the current skill; the skill diff passes it.\n\nAayudha-Evolve: #%d", skill, n, n)
		if _, err := run2(ctx, res.Worktree, "git", "commit", "--quiet", "--signoff", "-m", msg); err != nil {
			return Wrap(ExitExternal, err)
		}
		res.Commit, _ = run2(ctx, res.Worktree, "git", "rev-parse", "HEAD")
	}
	w := &ledger.Writer{Dir: root, Token: bot.Token.Value}
	if !c.DryRun() {
		if _, err := c.Write(ctx, actions.Action{Kind: "git.push", Target: repo + "@" + res.Branch, Payload: map[string]any{"branch": res.Branch, "commit": res.Commit}},
			func(ctx context.Context) (any, error) {
				return nil, w.Exec(ctx, res.Worktree, "push", "--quiet", "--force-with-lease", "origin", "HEAD:refs/heads/"+res.Branch)
			}); err != nil {
			return Wrap(ExitExternal, err)
		}
	}
	if err := openEvolvePR(ctx, c, repo, dev, bot.Tag, &res); err != nil {
		return Wrap(ExitExternal, err)
	}
	if err := e.checkpoint(bot.Token); err != nil {
		fmt.Fprintf(e.Err, "aayu: warning: the actions segment did not reach the ledger: %v\n", err)
	}
	return e.Emit(res, func(w io.Writer) error {
		for _, x := range res.Cases {
			fmt.Fprintf(w, "eval case %s reproduces Feedback #%d on the current %s skill and passes on the new one\n", x.ID, x.Feedback, x.Skill)
		}
		_, err := fmt.Fprintf(w, "evolve %s for #%d: branch %s pushed; PR #%d (agent-authored, %s), which a human CODEOWNER reviews; evolve never merges\n", skill, n, res.Branch, res.PR, EvolveLabel)
		return err
	})
}

// EvolveCandidate is accepted retro Feedback waiting for evolve.
type EvolveCandidate struct {
	Feedback int    `json:"feedback"`
	Title    string `json:"title"`
	Skill    string `json:"skill,omitempty"` // named in the title as skill `<name>`
	Layer    string `json:"fix_layer,omitempty"`
	Refused  string `json:"refused,omitempty"` // why its fix layer keeps it from a skill rewrite (S-7.7d)
}

var titleSkill = regexp.MustCompile("skill `([a-z][a-z0-9-]*)`")

// acceptedFeedback lists the open retro Feedback a human accepted, with the
// skill each names in its title, its fix layer, and why that layer keeps it
// from a skill rewrite.
func acceptedFeedback(ctx context.Context, c *github.Client, repo string) ([]EvolveCandidate, error) {
	owner, name, _ := github.SplitRepo(repo)
	list, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.Issue, *gh.Response, error) {
		return c.REST.Issues.ListByRepo(ctx, owner, name, &gh.IssueListByRepoOptions{State: "open", Labels: []string{"type:feedback"}, ListOptions: o})
	})
	if err != nil {
		return nil, err
	}
	out := []EvolveCandidate{}
	for _, is := range list {
		if is.IsPullRequest() || !strings.HasPrefix(is.GetTitle(), audit.FeedbackPrefix) || !slices.Contains(github.IssueLabels(is), workitems.LabelNeedsGrill) {
			continue
		}
		cand := EvolveCandidate{Feedback: is.GetNumber(), Title: is.GetTitle()}
		if m := titleSkill.FindStringSubmatch(is.GetTitle()); m != nil {
			cand.Skill = m[1]
		}
		var st FixLayer
		if cand.Layer = workitems.FixLayerOf(github.IssueLabels(is)); cand.Layer == workitems.FixCodebase || cand.Layer == workitems.FixCheck || cand.Layer == workitems.FixReview {
			if st, err = fixLayerState(ctx, c, repo, cand.Feedback); err != nil {
				return nil, err
			}
		}
		cand.Refused = evolveRefusal(cand.Feedback, cand.Layer, st)
		out = append(out, cand)
	}
	slices.SortFunc(out, func(a, b EvolveCandidate) int { return a.Feedback - b.Feedback })
	return out, nil
}

// recentEvolvePR is an evolve PR for skill, other than branch's own, opened
// within EvolveCooldownDays before now, or nil.
func recentEvolvePR(ctx context.Context, c *github.Client, repo, skill, branch string, now time.Time) (*gh.PullRequest, error) {
	owner, name, _ := github.SplitRepo(repo)
	since := now.AddDate(0, 0, -EvolveCooldownDays)
	opts := &gh.PullRequestListOptions{State: "all", Sort: "created", Direction: "desc", ListOptions: gh.ListOptions{PerPage: 100}}
	for {
		prs, resp, err := c.REST.PullRequests.List(ctx, owner, name, opts)
		if err != nil {
			return nil, err
		}
		for _, p := range prs {
			if p.GetCreatedAt().Before(since) {
				return nil, nil
			}
			if h := p.GetHead().GetRef(); h != branch && strings.HasPrefix(h, EvolveBranchPrefix+skill+"-") {
				return p, nil
			}
		}
		if resp.NextPage == 0 {
			return nil, nil
		}
		opts.Page = resp.NextPage
	}
}

// listEvolve is `evolve` without --feedback (the monthly loop): it lists the
// open, accepted Feedback that retro filed, with the skill each names and
// whether that skill has the evidence evolve needs, and writes nothing.
func listEvolve(ctx context.Context, e *Env) error {
	repo, err := e.RepoName(ctx)
	if err != nil {
		return err
	}
	c, _, err := e.HumanGitHub(ctx)
	if err != nil {
		return err
	}
	out, err := acceptedFeedback(ctx, c, repo)
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	per := map[string]int{}
	for _, x := range out {
		if x.Refused == "" {
			per[x.Skill]++
		}
	}
	return e.Emit(out, func(w io.Writer) error {
		if len(out) == 0 {
			_, err := fmt.Fprintln(w, "evolve: no accepted Feedback from retro is waiting (open, type:feedback, needs:grill)")
			return err
		}
		for _, x := range out {
			fmt.Fprintf(w, "#%d %s\n", x.Feedback, x.Title)
			switch {
			case x.Skill == "":
				fmt.Fprintln(w, "  names no skill: evolve does not take it")
			case x.Refused != "":
				fmt.Fprintf(w, "  not for evolve: %s\n", x.Refused)
			case per[x.Skill] < EvolveMinFeedback:
				fmt.Fprintf(w, "  waiting: skill `%s` needs %d accepted prose Feedback issues, has %d\n", x.Skill, EvolveMinFeedback, per[x.Skill])
			default:
				fmt.Fprintf(w, "  aayu evolve --skill %s --feedback %d\n", x.Skill, x.Feedback)
			}
		}
		return nil
	})
}

func or(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// evolveBody is the evolve PR's description in the pull request format
// (internal/prbody), every section written, since no agent edits it later.
func evolveBody(res *EvolveResult) string {
	var ids, ev []string
	for _, x := range res.Cases {
		ids = append(ids, "`evals/cases/"+x.ID+"`")
	}
	for _, x := range res.Evidence {
		ev = append(ev, fmt.Sprintf("#%d", x))
	}
	return fmt.Sprintf(`## What this does

This changes the %[1]s skill so that agents stop making the mistake that Feedback #%[2]d reports.

## Why

People corrected the same prose mistake in the %[1]s skill at least two times. The accepted Feedback is %[3]s.

## What changed

- The %[1]s skill has new or changed instructions.
- New eval cases repeat the mistake: %[4]s.

## How we know it works

- The new eval cases fail on the current %[1]s skill and pass with this change.
- The eval-first check in CI runs them again.
- After the merge, the skills table of `+"`aayu retro`"+` shows the correction rate before and after.

## Risk and rollback

The risk is low. The change is to skill text only. A person who owns `+"`skills/`"+` reviews it, and it ships through rolling. To roll back, run `+"`aayu install --pin <previous version>`"+`.

## For reviewers

<details><summary>Technical detail</summary>

Opened by `+"`aayu evolve`"+` (WP7.7), eval-first: each case's `+"`before/`"+` run fails tier 1 on the current skill. Evidence: %[3]s, each classified as a prose correction. Never auto-merged: `+"`skills/`"+` is a protected path.

</details>

Refs #%[2]d
`, res.Skill, res.Feedback, strings.Join(ev, ", "), strings.Join(ids, ", "))
}

// openEvolvePR opens the evolve PR (or finds the open one) and labels it
// agent-authored and aayu:evolve.
func openEvolvePR(ctx context.Context, c *github.Client, repo, dev, tag string, res *EvolveResult) error {
	owner, name, _ := github.SplitRepo(repo)
	open, _, err := c.REST.PullRequests.List(ctx, owner, name, &gh.PullRequestListOptions{State: "open", Head: owner + ":" + res.Branch})
	if err != nil {
		return err
	}
	if len(open) > 0 {
		res.PR = open[0].GetNumber()
	} else {
		title := fmt.Sprintf("feat(skills): evolve %s for Feedback #%d (WP7.7)", res.Skill, res.Feedback)
		body := evolveBody(res)
		var pr *gh.PullRequest
		if _, err := c.Write(ctx, actions.Action{Kind: "github.pull.create", Target: repo, Story: fmt.Sprintf("#%d", res.Feedback),
			Request: &actions.Request{Method: http.MethodPost, Path: "/repos/" + repo + "/pulls"}, Payload: map[string]any{"head": res.Branch, "base": dev, "title": title}},
			func(ctx context.Context) (any, error) {
				var err error
				pr, _, err = c.REST.PullRequests.Create(ctx, owner, name, gh.CreatePullRequest{Title: &title, Head: res.Branch, Base: dev, Body: &body})
				return map[string]any{"number": pr.GetNumber()}, err
			}); err != nil {
			return err
		}
		res.PR = pr.GetNumber()
	}
	labels := []string{workitems.LabelAgentAuthored, EvolveLabel}
	if tag != "" {
		labels = append(labels, tag)
	}
	_, err = c.Write(ctx, actions.Action{Kind: "github.issue.labels.add", Target: fmt.Sprintf("%s#%d", repo, res.PR),
		Request: &actions.Request{Method: http.MethodPost, Path: fmt.Sprintf("/repos/%s/issues/%d/labels", repo, res.PR)}, Payload: map[string]any{"labels": labels}},
		func(ctx context.Context) (any, error) {
			_, _, err := c.REST.Issues.AddLabelsToIssue(ctx, owner, name, res.PR, labels)
			return nil, err
		})
	return err
}
