package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/delegate"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
)

// WorkItem is one piece of work in flight: an aayu worktree on this machine,
// or a story delegated to a cloud agent (status --work, WP5.5b).
type WorkItem struct {
	Story    int    `json:"story,omitempty"`
	Agent    string `json:"agent"`              // host (the host's agent builds), a receiver, or evolve
	Where    string `json:"where"`              // the worktree, or "cloud"
	Branch   string `json:"branch,omitempty"`   //
	PR       int    `json:"pr,omitempty"`       //
	Status   string `json:"status,omitempty"`   // a delegation's status
	AgeHours int    `json:"age_hours"`          // since the worktree was cut or the hand-off
	Dirty    bool   `json:"dirty,omitempty"`    // uncommitted changes
	Unpushed int    `json:"unpushed,omitempty"` // commits origin does not have
}

var worktreeName = regexp.MustCompile(`^(\d+)(?:-([a-z]+))?$`)

// statusWork lists the work in flight: every worktree under
// .aayudha/worktrees/ (build, delegate --local, evolve, polish) with its
// agent, story, PR, and age, and every story delegated to a cloud agent.
// `aayu gc` removes the worktrees of closed stories.
func statusWork(ctx context.Context, e *Env) error {
	root := config.RepoRoot(e.Dir)
	if root == "" {
		return Errorf(ExitUsage, "status --work runs in a checkout of the repository")
	}
	main := mainCheckout(root)
	now := e.Now()
	var items []WorkItem
	out, _ := run2(ctx, main, "git", "worktree", "list", "--porcelain")
	base := filepath.Join(main, ".aayudha", "worktrees") + string(filepath.Separator)
	path, branch := "", ""
	flush := func() {
		if !strings.HasPrefix(path, base) {
			return
		}
		name := strings.TrimPrefix(path, base)
		it := WorkItem{Agent: "host", Where: filepath.ToSlash(strings.TrimPrefix(path, main+string(filepath.Separator))), Branch: branch}
		switch m := worktreeName.FindStringSubmatch(name); {
		case m != nil:
			it.Story, _ = strconv.Atoi(m[1])
			if m[2] != "" {
				it.Agent = m[2]
			}
		case strings.HasPrefix(name, "evolve-"):
			it.Agent = "evolve"
		case strings.HasPrefix(name, "pr-"):
			it.Agent = "polish"
		}
		if fi, err := os.Stat(path); err == nil {
			it.AgeHours = int(now.Sub(fi.ModTime()).Hours())
		}
		if d, _ := run2(ctx, path, "git", "status", "--porcelain"); d != "" {
			it.Dirty = true
		}
		if n, err := run2(ctx, path, "git", "rev-list", "--count", "HEAD", "--not", "--remotes=origin"); err == nil {
			it.Unpushed, _ = strconv.Atoi(n)
		}
		items = append(items, it)
	}
	for _, l := range strings.Split(out+"\n", "\n") {
		switch {
		case strings.HasPrefix(l, "worktree "):
			path, branch = strings.TrimPrefix(l, "worktree "), ""
		case strings.HasPrefix(l, "branch "):
			branch = strings.TrimPrefix(strings.TrimPrefix(l, "branch "), "refs/heads/")
		case l == "":
			flush()
			path = ""
		}
	}
	// Cloud delegations from the ledger, and the open PRs of every branch.
	if repo, err := e.RepoName(ctx); err == nil {
		if c, tok, err := e.HumanGitHub(ctx); err == nil {
			if w, err := e.Ledger(ctx, tok, "human", ""); err == nil {
				if t, err := w.Read(ctx); err == nil {
					ps, _ := t.List(ctx, "delegate/")
					for _, p := range ps {
						var st delegate.State
						if b, err := t.File(ctx, p); err == nil && json.Unmarshal(b, &st) == nil && st.Mode == "cloud" && st.Status != dlgMerged && st.Status != dlgFailed {
							it := WorkItem{Story: st.Story, Agent: st.Receiver, Where: "cloud", Branch: st.Branch, PR: st.PR, Status: st.Status}
							if at, err := time.Parse(time.RFC3339, st.StartedAt); err == nil {
								it.AgeHours = int(now.Sub(at).Hours())
							}
							items = append(items, it)
						}
					}
					t.Close()
				}
			}
			owner, name, _ := github.SplitRepo(repo)
			if prs, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.PullRequest, *gh.Response, error) {
				return c.REST.PullRequests.List(ctx, owner, name, &gh.PullRequestListOptions{State: "open", ListOptions: o})
			}); err == nil {
				for i := range items {
					for _, pr := range prs {
						if items[i].PR == 0 && items[i].Branch != "" && pr.GetHead().GetRef() == items[i].Branch {
							items[i].PR = pr.GetNumber()
						}
					}
				}
			}
		}
	}
	slices.SortStableFunc(items, func(a, b WorkItem) int { return a.Story - b.Story })
	if items == nil {
		items = []WorkItem{}
	}
	return e.Emit(items, func(w io.Writer) error {
		if len(items) == 0 {
			_, err := fmt.Fprintln(w, "no work in flight: no aayu worktree and no cloud delegation")
			return err
		}
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "STORY\tAGENT\tWHERE\tBRANCH\tPR\tAGE\tSTATE")
		for _, it := range items {
			state := it.Status
			if it.Dirty {
				state = strings.TrimPrefix(state+", uncommitted changes", ", ")
			}
			if it.Unpushed > 0 {
				state = strings.TrimPrefix(fmt.Sprintf("%s, %d unpushed", state, it.Unpushed), ", ")
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", num(it.Story), it.Agent, it.Where, orDash(it.Branch), num(it.PR), age(it.AgeHours), orDash(state))
		}
		if err := tw.Flush(); err != nil {
			return err
		}
		_, err := fmt.Fprintln(w, "`aayu gc` removes the worktrees of closed stories; `aayu build <n> --delegate <agent>` syncs a delegation.")
		return err
	})
}

func num(n int) string {
	if n == 0 {
		return "-"
	}
	return "#" + strconv.Itoa(n)
}

func age(h int) string {
	if h < 24 {
		return strconv.Itoa(h) + "h"
	}
	return strconv.Itoa(h/24) + "d"
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
