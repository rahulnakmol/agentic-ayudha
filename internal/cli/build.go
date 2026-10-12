package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/ready"
	"github.com/rahulnakmol/agentic-ayudha/internal/verify"
)

func init() { register("build", runBuild) }

// BuildResult is what build reports.
type BuildResult struct {
	Issue      int    `json:"issue"`
	Worktree   string `json:"worktree"`
	Branch     string `json:"branch"`
	Created    bool   `json:"worktree_created,omitempty"`
	Checkpoint string `json:"checkpoint,omitempty"` // n/N of the commit this run made
	Commit     string `json:"commit,omitempty"`
	PR         int    `json:"pr,omitempty"`
	PRURL      string `json:"pr_url,omitempty"`
	Next       string `json:"next"`
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

func slug(s string) string {
	s = strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(s), "-"), "-")
	if len(s) > 40 {
		s = strings.TrimRight(s[:40], "-")
	}
	return s
}

// BuildWorktree is where build keeps a story's worktree: .aayudha/worktrees/<n> (git-ignored).
func BuildWorktree(root string, n int) string {
	return filepath.Join(root, ".aayudha", "worktrees", fmt.Sprint(n))
}

func run2(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s: %w: %s", strings.Join(args[:2], " "), err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

// runBuild is the thin-slice build (WP4.10): it refuses a story that is not
// Ready or whose body drifted from its snapshot; the first run cuts a
// worktree from origin/dev; each later run commits what the agent wrote as a
// checkpoint (Aayudha-Story, Aayudha-Checkpoint, and Aayudha-Model trailers,
// DCO sign-off), pushes the branch, and opens the PR. Leases, the budget
// breaker, per-checkpoint verify, and --resume are WP5.5.
func runBuild(ctx context.Context, e *Env, in Values) error {
	if in.Bool("resume") {
		return Errorf(ExitUsage, "build --resume is n/a until WP5.5")
	}
	root := config.RepoRoot(e.Dir)
	if root == "" {
		return Errorf(ExitUsage, "build runs in a checkout of the repository")
	}
	repo, err := e.RepoName(ctx)
	if err != nil {
		return err
	}
	n := issueNumber(in.String("issue"))
	c, bot, err := e.BotGitHub(ctx, "")
	if err != nil {
		return err
	}
	rd := &ready.Ready{GH: c, Repo: repo, TechnicalOwner: e.technicalOwner()}
	chk, is, err := rd.Check(ctx, n)
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	if why := e.notReady(ctx, c, repo, n, chk, github.IssueLabels(is)); why != "" {
		return Errorf(ExitBlocked, "build #%d: %s", n, why)
	}
	res := BuildResult{Issue: n, Worktree: BuildWorktree(root, n), Branch: fmt.Sprintf("story/%d-%s", n, slug(chk.Title))}
	dev := e.devBranch()
	if _, err := os.Stat(res.Worktree); err != nil {
		if _, err := run2(ctx, root, "git", "fetch", "--quiet", "origin", dev); err != nil {
			return Wrap(ExitExternal, err)
		}
		if _, err := run2(ctx, root, "git", "worktree", "add", "--quiet", "-b", res.Branch, res.Worktree, "origin/"+dev); err != nil {
			return Wrap(ExitExternal, err)
		}
		res.Created = true
		res.Next = fmt.Sprintf("write the change in %s (tests included; map criteria to tests in .aayudha/evidence/%d.toml), then run `aayu build %d` again to commit a checkpoint and open the PR", res.Worktree, n, n)
		return e.Emit(res, func(w io.Writer) error {
			_, err := fmt.Fprintf(w, "#%d %s: worktree %s on %s from origin/%s\n  next: %s\n", n, chk.Title, res.Worktree, res.Branch, dev, res.Next)
			return err
		})
	}
	dirty, err := run2(ctx, res.Worktree, "git", "status", "--porcelain")
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	if dirty != "" {
		prior, _ := run2(ctx, res.Worktree, "git", "log", "--format=%(trailers:key=Aayudha-Checkpoint,valueonly)", "origin/"+dev+"..HEAD")
		k := len(strings.Fields(prior)) + 1
		if _, err := run2(ctx, res.Worktree, "git", "add", "--all"); err != nil {
			return Wrap(ExitExternal, err)
		}
		names, _ := run2(ctx, res.Worktree, "git", "diff", "--cached", "--name-only", "origin/"+dev)
		total := max(k, len(verify.ReadLayers(res.Worktree).Footprint(strings.Fields(names))))
		res.Checkpoint = fmt.Sprintf("%d/%d", k, total)
		msg := fmt.Sprintf("feat: %s (#%d, checkpoint %s)\n\nAayudha-Story: #%d\nAayudha-Checkpoint: %s", strings.ToLower(chk.Title[:1])+chk.Title[1:], n, res.Checkpoint, n, res.Checkpoint)
		if m := e.builderModel(); m != "" {
			msg += "\nAayudha-Model: " + m
		}
		if _, err := run2(ctx, res.Worktree, "git", "commit", "--quiet", "--signoff", "-m", msg); err != nil {
			return Wrap(ExitExternal, err)
		}
		res.Commit, _ = run2(ctx, res.Worktree, "git", "rev-parse", "HEAD")
	}
	ahead, _ := run2(ctx, res.Worktree, "git", "rev-list", "--count", "origin/"+dev+"..HEAD")
	if ahead == "0" {
		return Errorf(ExitBlocked, "build #%d: nothing to commit yet in %s", n, res.Worktree)
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
	owner, name, _ := github.SplitRepo(repo)
	open, _, err := c.REST.PullRequests.List(ctx, owner, name, &gh.PullRequestListOptions{State: "open", Head: owner + ":" + res.Branch})
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	if len(open) > 0 {
		res.PR, res.PRURL = open[0].GetNumber(), open[0].GetHTMLURL()
	} else {
		title := "feat: " + strings.ToLower(chk.Title[:1]) + chk.Title[1:]
		body := fmt.Sprintf("Refs #%d\n\nBuilt with `aayu build` (WP4.10) from the Ready snapshot of #%d (body hash `%s`). The merge gate verifies it on the base branch's definition; `aayu ship` merges it after a human pass.\n", n, n, chk.BodyHash[:12])
		var pr *gh.PullRequest
		if _, err := c.Write(ctx, actions.Action{Kind: "github.pull.create", Target: repo, Story: fmt.Sprintf("#%d", n),
			Request: &actions.Request{Method: http.MethodPost, Path: "/repos/" + repo + "/pulls"}, Payload: map[string]any{"head": res.Branch, "base": dev, "title": title}},
			func(ctx context.Context) (any, error) {
				var err error
				pr, _, err = c.REST.PullRequests.Create(ctx, owner, name, gh.CreatePullRequest{Title: &title, Head: res.Branch, Base: dev, Body: &body})
				return map[string]any{"number": pr.GetNumber()}, err
			}); err != nil {
			return Wrap(ExitExternal, err)
		}
		if pr != nil {
			res.PR, res.PRURL = pr.GetNumber(), pr.GetHTMLURL()
		}
	}
	// The actions segment of this checkpoint goes to the ledger (WP1.5), pushed with the build's own token.
	if err := e.checkpoint(bot.Token); err != nil {
		fmt.Fprintf(e.Err, "aayu: warning: the actions segment did not reach the ledger: %v\n", err)
	}
	res.Next = fmt.Sprintf("`aayu verify` in %s for an advisory pack; the merge gate re-runs it on PR #%d; then `aayu ship %d` after a human pass", res.Worktree, res.PR, res.PR)
	return e.Emit(res, func(w io.Writer) error {
		if res.Commit != "" {
			fmt.Fprintf(w, "#%d: checkpoint %s committed (%s)\n", n, res.Checkpoint, short(res.Commit))
		}
		_, err := fmt.Fprintf(w, "#%d: branch %s pushed; PR #%d %s\n  next: %s\n", n, res.Branch, res.PR, res.PRURL, res.Next)
		return err
	})
}

// notReady says why a story cannot be built: not labelled status:ready, not
// passing Ready, or its body hash differs from the Ready snapshot's.
func (e *Env) notReady(ctx context.Context, c *github.Client, repo string, n int, chk ready.Result, labels []string) string {
	switch {
	case !chk.Pass:
		return fmt.Sprintf("it does not pass Ready (%d problem(s)); run `aayu ready %d --check`", len(chk.Problems), n)
	case !contains(labels, ready.LabelReady):
		return "it is not labelled status:ready; run `aayu ready " + fmt.Sprint(n) + "`"
	}
	snap, err := ready.SnapshotHash(ctx, c, repo, n)
	if err != nil {
		return err.Error()
	}
	if snap != chk.BodyHash {
		return fmt.Sprintf("its body changed after Ready (snapshot %s, now %s); run `aayu ready %d` again", short(snap), short(chk.BodyHash), n)
	}
	return ""
}

func contains(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}

// builderModel is the model the building agent runs on, when known: the
// AAYU_MODEL variable a host sets, else [models.build.<host>].
func (e *Env) builderModel() string {
	if m := e.Getenv("AAYU_MODEL"); m != "" {
		return m
	}
	if l, err := e.Config(); err == nil && e.Host != "" {
		if mc, ok := l.Config.Models.Lanes["build"][e.Host]; ok {
			return mc.Model
		}
	}
	return ""
}
