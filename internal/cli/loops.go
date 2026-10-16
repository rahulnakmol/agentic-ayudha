package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"time"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/initrepo"
	"github.com/rahulnakmol/agentic-ayudha/internal/loops"
)

// LoopHookPrefix names the loop guard hook: `aayu hook loop:<name>`.
const LoopHookPrefix = "loop:"

type loopGuard struct {
	Loop      string `json:"loop"`
	Due       bool   `json:"due"`
	Reason    string `json:"reason"`
	CIMinutes int    `json:"ci_minutes_this_month,omitempty"`
}

// runLoopGuard is `aayu hook loop:<name>` (WP7.1), the first step of every
// scheduled run on every host: it exits 4 while the repository is halted and 6
// while the [budget] breaker has paused agent lanes or, in Actions, the loops'
// runs this month have used [budget] ci_minutes_month; otherwise it says
// whether this run is the scheduled one (in Actions, written to GITHUB_OUTPUT
// as due=true|false); an event loop is due whenever its host's trigger
// started it. It writes nothing anywhere else.
func runLoopGuard(ctx context.Context, e *Env, name string) error {
	root := config.RepoRoot(e.Dir)
	if root == "" {
		return Errorf(ExitUsage, "hook %s%s: run it in a checkout (loops/*.toml)", LoopHookPrefix, name)
	}
	ls, err := loops.Load(root)
	if err != nil {
		return Wrap(ExitUsage, err)
	}
	l, ok := loops.Find(ls, name)
	if !ok {
		return Errorf(ExitUsage, "hook %s%s: no loops/%s.toml", LoopHookPrefix, name, name)
	}
	if err := e.checkHalt(ctx); err != nil {
		return err
	}
	if _, err := os.Stat(e.PauseFile()); err == nil {
		return Errorf(ExitBudget, "loop %s: agent lanes are paused by the [budget] breaker (%s); a human clears it with `aayu resume`", name, e.PauseFile())
	}
	cfg, err := e.Config()
	if err != nil {
		return err
	}
	clock, err := cfg.Config.Clock()
	if err != nil {
		return Wrap(ExitUsage, err)
	}
	g := loopGuard{Loop: name}
	if e.CI && cfg.Config.Budget.CIMinutesMonth > 0 {
		if g.CIMinutes, err = loopMinutes(ctx, e); err != nil {
			return err
		}
		if g.CIMinutes >= cfg.Config.Budget.CIMinutesMonth {
			return Errorf(ExitBudget, "loop %s: %s used %d Actions minutes this month; [budget] ci_minutes_month is %d", name, loops.Workflow, g.CIMinutes, cfg.Config.Budget.CIMinutesMonth)
		}
	}
	event, fired := e.Getenv("GITHUB_EVENT_NAME"), ""
	if event == "schedule" {
		var ev struct {
			Schedule string `json:"schedule"`
		}
		if b, err := os.ReadFile(e.Getenv("GITHUB_EVENT_PATH")); err == nil {
			_ = json.Unmarshal(b, &ev)
		}
		if fired = ev.Schedule; fired == "" {
			return Errorf(ExitUsage, "loop %s: a schedule event without github.event.schedule", name)
		}
	}
	owners := []string(nil)
	if e.CI {
		rec, err := loops.ReadRecord(root)
		if err != nil {
			return Wrap(ExitUsage, err)
		}
		owners = rec.Owners(name)
	}
	switch {
	case len(owners) > 0:
		g.Reason = fmt.Sprintf("not due here: %s runs it (%s), so the Actions fallback leaves it there", strings.Join(owners, ", "), loops.RecordFile)
	case event == "workflow_dispatch":
		g.Due, g.Reason = true, "dispatched by hand"
	case l.Due(clock.Loc, e.Now(), fired):
		g.Due, g.Reason = true, "scheduled: "+l.Label(clock.Loc.String())
	case fired != "":
		g.Reason = fmt.Sprintf("not due: cron %q is for the other UTC offset of %s today", fired, clock.Loc)
	default:
		g.Reason = "not due: " + l.Label(clock.Loc.String())
	}
	if out := e.Getenv("GITHUB_OUTPUT"); out != "" {
		f, err := os.OpenFile(out, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(f, "due=%t\n", g.Due)
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			return err
		}
	}
	return e.Emit(g, func(w io.Writer) error {
		_, err := fmt.Fprintf(w, "loop %s: %s\n", name, g.Reason)
		return err
	})
}

// loopMinutes sums the runner minutes of this month's finished runs of the
// Actions fallback, each rounded up to a whole minute as GitHub bills them.
func loopMinutes(ctx context.Context, e *Env) (int, error) {
	repo, err := e.RepoName(ctx)
	if err != nil {
		return 0, err
	}
	c, _, err := e.HumanGitHub(ctx)
	if err != nil {
		return 0, err
	}
	owner, name, _ := github.SplitRepo(repo)
	now := e.Now().UTC()
	month := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	total := 0
	opts := &gh.ListWorkflowRunsOptions{Created: ">=" + month.Format(time.DateOnly), ListOptions: gh.ListOptions{PerPage: 100}}
	for {
		runs, resp, err := c.REST.Actions.ListWorkflowRunsByFileName(ctx, owner, name, loops.Workflow, opts)
		if github.NotFound(err) {
			return 0, nil
		}
		if err != nil {
			return 0, err
		}
		for _, r := range runs.WorkflowRuns {
			if r.GetStatus() != "completed" || r.RunStartedAt == nil {
				continue
			}
			if d := r.GetUpdatedAt().Sub(r.GetRunStartedAt().Time); d > 0 {
				total += int(math.Ceil(d.Minutes()))
			}
		}
		if resp == nil || resp.NextPage == 0 {
			return total, nil
		}
		opts.Page = resp.NextPage
	}
}

// loopFiles renders a product's loops/*.toml for init --gates (WP7.1); a
// repository without loops gets none.
func (e *Env) loopFiles(it *initrepo.Init) (map[string][]byte, error) {
	ls, err := loops.Load(it.Root)
	if err != nil {
		return nil, Wrap(ExitUsage, err)
	}
	if len(ls) == 0 {
		return nil, nil
	}
	l, err := e.Config()
	if err != nil {
		return nil, err
	}
	branch := l.Config.Branches.Dev
	if branch == "" {
		branch = "dev"
	}
	files, err := loops.Files(ls, it.Repo.FullName, branch, l.Config.Time.TZ, initrepo.GuardVersion(e.Build.Version), Implemented)
	return files, Wrap(ExitUsage, err)
}

func isLoopHook(name string) (string, bool) { return strings.CutPrefix(name, LoopHookPrefix) }
