package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/ready"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// ReadyEarlyReport is what `aayu ready <f> --early-build <story>` reports.
type ReadyEarlyReport struct {
	Repo         string            `json:"repo"`
	Feature      int               `json:"feature"`
	Story        int               `json:"story"`
	Effect       string            `json:"effect"` // what the person saw before the device code
	Approval     *ready.EarlyBuild `json:"approval"`
	LedgerPath   string            `json:"ledger_path"`
	LedgerCommit string            `json:"ledger_commit,omitempty"`
	DryRun       bool              `json:"dry_run,omitempty"`
}

// runReadyEarly approves one early build of Feature f (core design E1): a
// child story of f labelled early-build, within the budget the person sees
// before the device code. It is a person's decision: refused for an agent
// lane before anything is read, and every refusal comes before the device
// code and before any write.
func runReadyEarly(ctx context.Context, e *Env, in Values, f int) error {
	story := issueNumber(in.String("early-build"))
	switch {
	case in.Has("part") || in.Bool("request") || in.Bool("check") || in.Bool("guard"):
		return Errorf(ExitUsage, "ready: --early-build goes with none of --part, --request, --check, or --guard")
	case e.Getenv("AAYU_AGENT") == "1" || e.CI:
		return e.personOnly("ready --early-build")
	}
	l, err := e.Config()
	if err != nil {
		return err
	}
	times, cost := l.Config.Baselines.EarlyBuildBudget, l.Config.Autonomy.Budget.CostUSD
	repo, err := e.RepoName(ctx)
	if err != nil {
		return err
	}
	c, tok, err := e.HumanGitHub(ctx) // the person's own client; nothing is written before the device flow
	if err != nil {
		return err
	}
	w, err := e.Ledger(ctx, tok, "human", "")
	if err != nil {
		return err
	}
	hash, err := earlyStory(ctx, c, repo, f, story)
	if err != nil {
		return err
	}
	effect := ready.EarlyBuildEffect(f, story, times, cost)
	a, err := e.presenceCheck(ctx, Pending{What: "ready --early-build", Effect: []string{effect}}, AccessWrite, e.personOnly("ready --early-build"))
	if err != nil {
		return err
	}
	// The story can change while the code waits: the approval covers it only as the person saw it.
	now, err := earlyStory(ctx, c, repo, f, story)
	if err != nil {
		return err
	}
	if now != hash {
		return Errorf(ExitBlocked, "#%d changed while the code waited. Nothing was approved. Run it again.", story)
	}
	rec := ready.EarlyBuild{Schema: ready.EarlyBuildSchema, Feature: f, Story: story, BodyHash: hash,
		BudgetUSD: ready.EarlyBuildBudget(times, cost), ApprovedBy: a.Login, Presence: a.Via, At: e.Now().UTC().Format(time.RFC3339)}
	rep := ReadyEarlyReport{Repo: repo, Feature: f, Story: story, Effect: effect, Approval: &rec,
		LedgerPath: ready.EarlyBuildPath(f, story), DryRun: c.DryRun()}
	if !rep.DryRun {
		data, _ := json.MarshalIndent(rec, "", "  ")
		rep.LedgerCommit, err = w.Write(ctx, ledger.Commit{Message: fmt.Sprintf("specs: early build #%d of #%d, approved by @%s", story, f, a.Login),
			Files: map[string][]byte{rep.LedgerPath: append(data, '\n')}})
		if err != nil {
			return Wrap(ExitExternal, err)
		}
	}
	return e.Emit(rep, func(w io.Writer) error {
		fmt.Fprintf(w, "@%s approved the early build #%d of #%d. Budget up to $%.2f. Its PR never merges into dev.\n", a.Login, story, f, rec.BudgetUSD)
		if rep.DryRun {
			_, err := fmt.Fprintf(w, "  --dry-run: ledger %s planned, not written\n", rep.LedgerPath)
			return err
		}
		_, err := fmt.Fprintf(w, "  ledger %s (%s)\n", rep.LedgerPath, short(rep.LedgerCommit))
		return err
	})
}

// earlyStory checks that story is a child story of Feature f labelled
// early-build, and returns the hash of its body. Each refusal names the
// condition that fails.
func earlyStory(ctx context.Context, c *github.Client, repo string, f, story int) (string, error) {
	owner, name, _ := github.SplitRepo(repo)
	fi, _, err := c.REST.Issues.Get(ctx, owner, name, f)
	if err != nil {
		return "", Wrap(ExitExternal, err)
	}
	if t, _ := workitems.TypeOf(fi.GetType().GetName(), github.IssueLabels(fi)); t != "feature" {
		return "", Errorf(ExitUsage, "ready --early-build approves a story of a Feature; #%d is a %s", f, or(t, "issue"))
	}
	is, _, err := c.REST.Issues.Get(ctx, owner, name, story)
	if err != nil {
		return "", Wrap(ExitExternal, err)
	}
	labels := github.IssueLabels(is)
	t, _ := workitems.TypeOf(is.GetType().GetName(), labels)
	switch {
	case t != "story":
		return "", Errorf(ExitBlocked, "#%d is a %s, not a story, so it is not an early build. Nothing was approved.", story, or(t, "issue"))
	case !contains(labels, ready.EarlyBuildLabel):
		return "", Errorf(ExitBlocked, "#%d has no %s label, so it is not an early build. Nothing was approved.", story, ready.EarlyBuildLabel)
	}
	parent, _, err := c.REST.SubIssue.GetParentIssue(ctx, owner, name, int64(story))
	switch {
	case err != nil && !github.NotFound(err):
		return "", Wrap(ExitExternal, err)
	case parent.GetNumber() == 0:
		return "", Errorf(ExitBlocked, "#%d is not a child of #%d: it has no parent. Nothing was approved.", story, f)
	case parent.GetNumber() != f:
		return "", Errorf(ExitBlocked, "#%d is not a child of #%d: its parent is #%d. Nothing was approved.", story, f, parent.GetNumber())
	}
	return workitems.BodyHash(is.GetBody()), nil
}
