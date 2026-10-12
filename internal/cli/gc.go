package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
)

func init() { register("gc", runGC) }

// GCResult is what gc reports.
type GCResult struct {
	Reclaimed []GCItem `json:"leases_reclaimed"`
	Live      []GCItem `json:"leases_live"`
	Removed   []GCItem `json:"worktrees_removed"`
	Kept      []GCItem `json:"worktrees_kept"`
	DryRun    bool     `json:"dry_run,omitempty"`
}

// GCItem is one lease or worktree gc looked at.
type GCItem struct {
	Story  int    `json:"story"`
	Detail string `json:"detail"`
}

// runGC (WP5.5) prunes what build leaves behind: leases whose lock comment
// expired, or whose story is closed, are reclaimed (comment, label, and
// assignee); worktrees under .aayudha/worktrees/ whose story is closed are
// removed with their local branch when nothing in them is uncommitted or
// unpushed, and kept with the reason otherwise.
func runGC(ctx context.Context, e *Env, _ Values) error {
	repo, err := e.RepoName(ctx)
	if err != nil {
		return err
	}
	c, _, err := e.HumanGitHub(ctx)
	if err != nil {
		return err
	}
	owner, name, _ := github.SplitRepo(repo)
	res := GCResult{Reclaimed: []GCItem{}, Live: []GCItem{}, Removed: []GCItem{}, Kept: []GCItem{}, DryRun: c.DryRun()}
	leased, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.Issue, *gh.Response, error) {
		return c.REST.Issues.ListByRepo(ctx, owner, name, &gh.IssueListByRepoOptions{State: "all", Labels: []string{LeaseLabel}, ListOptions: o})
	})
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	now := e.Now()
	for _, is := range leased {
		n := is.GetNumber()
		l, err := readLease(ctx, c, owner, name, n)
		if err != nil {
			return Wrap(ExitExternal, err)
		}
		why := ""
		switch {
		case l == nil:
			why = "labelled leased without a lock comment"
		case is.GetState() != "open":
			why = fmt.Sprintf("the story is closed (lease of %s)", l.Holder)
		case !l.Expires.After(now):
			why = fmt.Sprintf("the lease of %s expired at %s", l.Holder, l.Expires.UTC().Format(time.RFC3339))
		}
		if why == "" {
			res.Live = append(res.Live, GCItem{n, fmt.Sprintf("%s until %s", l.Holder, l.Expires.UTC().Format(time.RFC3339))})
			continue
		}
		if l == nil {
			if err := removeLeaseLabel(ctx, c, repo, n); err != nil {
				return err
			}
		} else if _, err := e.releaseLease(ctx, c, repo, n, "reclaimed by `aayu gc`: "+why); err != nil {
			return Wrap(ExitExternal, err)
		}
		res.Reclaimed = append(res.Reclaimed, GCItem{n, why})
	}

	if root := config.RepoRoot(e.Dir); root != "" {
		_, _ = run2(ctx, root, "git", "worktree", "prune")
		dirs, _ := os.ReadDir(filepath.Join(root, ".aayudha", "worktrees"))
		for _, d := range dirs {
			n, err := strconv.Atoi(d.Name())
			if !d.IsDir() || err != nil {
				continue
			}
			wt := BuildWorktree(root, n)
			is, _, err := c.REST.Issues.Get(ctx, owner, name, n)
			if err != nil && !github.NotFound(err) {
				return Wrap(ExitExternal, err)
			}
			if err == nil && is.GetState() == "open" {
				res.Kept = append(res.Kept, GCItem{n, "the story is open"})
				continue
			}
			if why := worktreeBusy(ctx, wt); why != "" {
				res.Kept = append(res.Kept, GCItem{n, "the story is closed, but " + why})
				continue
			}
			if !res.DryRun {
				branch, _ := run2(ctx, wt, "git", "rev-parse", "--abbrev-ref", "HEAD")
				if _, err := run2(ctx, root, "git", "worktree", "remove", "--force", wt); err != nil {
					return Wrap(ExitExternal, err)
				}
				if branch != "" && branch != "HEAD" {
					_, _ = run2(ctx, root, "git", "branch", "-D", branch)
				}
			}
			res.Removed = append(res.Removed, GCItem{n, "the story is closed"})
		}
	}
	return e.Emit(res, func(w io.Writer) error {
		for _, x := range res.Reclaimed {
			fmt.Fprintf(w, "lease #%d reclaimed: %s\n", x.Story, x.Detail)
		}
		for _, x := range res.Live {
			fmt.Fprintf(w, "lease #%d live: %s\n", x.Story, x.Detail)
		}
		for _, x := range res.Removed {
			fmt.Fprintf(w, "worktree #%d removed: %s\n", x.Story, x.Detail)
		}
		for _, x := range res.Kept {
			fmt.Fprintf(w, "worktree #%d kept: %s\n", x.Story, x.Detail)
		}
		if len(res.Reclaimed)+len(res.Live)+len(res.Removed)+len(res.Kept) == 0 {
			fmt.Fprintln(w, "gc: no leases or build worktrees")
		}
		if res.DryRun {
			fmt.Fprintln(w, "--dry-run: nothing was changed")
		}
		return nil
	})
}

// worktreeBusy says why a worktree still holds work: uncommitted changes, or
// commits its branch on origin does not have.
func worktreeBusy(ctx context.Context, wt string) string {
	if dirty, err := run2(ctx, wt, "git", "status", "--porcelain"); err != nil || dirty != "" {
		return "it has uncommitted changes"
	}
	branch, _ := run2(ctx, wt, "git", "rev-parse", "--abbrev-ref", "HEAD")
	if n, err := run2(ctx, wt, "git", "rev-list", "--count", "HEAD", "--not", "--remotes=origin"); err == nil && n != "0" {
		return fmt.Sprintf("%s has %s commit(s) not on origin", branch, n)
	}
	return ""
}

// removeLeaseLabel removes a stray lease label.
func removeLeaseLabel(ctx context.Context, c *github.Client, repo string, n int) error {
	owner, name, _ := github.SplitRepo(repo)
	_, err := c.Write(ctx, actions.Action{Kind: "github.issue.labels.remove", Target: fmt.Sprintf("%s#%d", repo, n),
		Request: &actions.Request{Method: http.MethodDelete, Path: fmt.Sprintf("/repos/%s/issues/%d/labels/%s", repo, n, LeaseLabel)}},
		func(ctx context.Context) (any, error) {
			_, err := c.REST.Issues.RemoveLabelForIssue(ctx, owner, name, n, LeaseLabel)
			if github.NotFound(err) {
				err = nil
			}
			return nil, err
		})
	return wrapExternal(err)
}
