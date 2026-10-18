package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/autonomy"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/delegate"
	"github.com/rahulnakmol/agentic-ayudha/internal/doctor"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/models"
	"github.com/rahulnakmol/agentic-ayudha/internal/notify"
	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
	"github.com/rahulnakmol/agentic-ayudha/internal/ready"
	"github.com/rahulnakmol/agentic-ayudha/internal/ui"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

func init() { register("build", runBuild) }

// BuildResult is what build reports.
type BuildResult struct {
	Issue      int      `json:"issue"`
	Worktree   string   `json:"worktree"`
	Branch     string   `json:"branch"`
	Created    bool     `json:"worktree_created,omitempty"`
	Resumed    string   `json:"resumed_from,omitempty"` // the last Aayudha-Checkpoint trailer --resume continued from
	Lease      Lease    `json:"lease"`
	Checkpoint string   `json:"checkpoint,omitempty"` // n/N of the commit this run made
	Attempt    int      `json:"attempt,omitempty"`
	Commit     string   `json:"commit,omitempty"`
	Verify     string   `json:"verify,omitempty"` // hold, miss, or error: what verify said of the checkpoint
	Budget     Usage    `json:"budget"`
	PR         int      `json:"pr,omitempty"`
	PRURL      string   `json:"pr_url,omitempty"`
	Notes      []string `json:"notes,omitempty"`
	Next       string   `json:"next"`
	// Delegation is set when another agent builds the story (--delegate, WP5.5b).
	Delegation *Delegation `json:"delegation,omitempty"`
}

// Usage is a story's build budget use against [autonomy.budget].
type Usage struct {
	WallHours   float64 `json:"wall_hours"`
	MaxWall     float64 `json:"max_wall_hours"`
	CostUSD     float64 `json:"cost_usd"`
	MaxCost     float64 `json:"max_cost_usd"`
	Attempts    int     `json:"failed_attempts"`
	MaxAttempts int     `json:"max_attempts"`
	// MeteredBy names what makes aayu's own model calls pay-as-you-go (the
	// [models.gateway] endpoint or a direct provider's key). The cost limit
	// applies only then; "" means it is off.
	MeteredBy string `json:"metered_by,omitempty"`
}

// Line is the budget as a person reads it. Spend is what aayu meters (its
// gateway calls and AAYU_COST_USD); a subscription host session reports no
// cost, so 0.00 USD is not the real cost of a build done there.
func (u Usage) Line() string {
	spend := fmt.Sprintf("metered spend %.2f USD of %g (subscription use not counted)", u.CostUSD, u.MaxCost)
	switch {
	case u.MeteredBy == "":
		spend = fmt.Sprintf("metered spend %.2f USD, no cost limit: no pay-as-you-go model key or endpoint is configured (subscription use not counted)", u.CostUSD)
	case u.MaxCost <= 0:
		spend = fmt.Sprintf("metered spend %.2f USD, no cost limit: autonomy.budget.cost_usd is 0 (subscription use not counted)", u.CostUSD)
	}
	return fmt.Sprintf("%.1f of %g h, %s, %d of %d failed attempts", u.WallHours, u.MaxWall, spend, u.Attempts, u.MaxAttempts)
}

// BuildState is build/<story>/state.json on the ledger: when the build
// started, its checkpoints with their verify results, and the failures the
// budget counts.
type BuildState struct {
	Story       int                `json:"story"`
	Branch      string             `json:"branch"`
	StartedAt   string             `json:"started_at"`
	Attempts    int                `json:"failed_attempts"`
	Failures    map[string]int     `json:"failures_by_layer,omitempty"` // failed verifies per layer of the checkpoint
	Checkpoints []CheckpointRecord `json:"checkpoints"`
	Blocked     string             `json:"blocked,omitempty"`
	Unblocked   []string           `json:"unblocked,omitempty"` // when a human made the story Ready again after a block
}

// CheckpointRecord is one checkpoint commit and what verify said of it.
type CheckpointRecord struct {
	K       int      `json:"k"`
	N       int      `json:"n"`
	Attempt int      `json:"attempt"`
	Commit  string   `json:"commit"`
	Layers  []string `json:"layers"`
	Verify  string   `json:"verify"` // hold, miss, or error
	Detail  string   `json:"detail,omitempty"`
	At      string   `json:"at"`
	// Model is the builder's model as build resolved it (AAYU_MODEL, else the
	// host's build lane). inspect reads the builder's family from this record,
	// which the PR cannot change, never from the commit trailer alone (Bug #171).
	Model string `json:"model,omitempty"`
}

// BuildLedgerPath is where a story's build state lives on the ledger.
func BuildLedgerPath(n int) string { return fmt.Sprintf("build/%d/state.json", n) }

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

// readLedgerJSON reads a JSON file from the ledger; ok is false when the
// ledger or the file does not exist yet.
func readLedgerJSON(ctx context.Context, w *ledger.Writer, p string, v any) (bool, error) {
	t, err := w.Read(ctx)
	if errors.Is(err, ledger.ErrNoLedger) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer t.Close()
	b, err := t.File(ctx, p)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, json.Unmarshal(b, v)
}

// runBuild (WP5.5) builds a Ready story in steps. It refuses a story that
// is not Ready or whose body drifted from its snapshot, and one another
// builder holds a live lease on; it takes or renews the lease, and checks the
// [autonomy.budget] (wall time, spend, failed attempts) before any work. The
// first run cuts .aayudha/worktrees/<n> from origin/dev (or, with --resume,
// from the story branch at its last Aayudha-Checkpoint trailer). Each later
// run commits what the agent wrote as checkpoint k/N (N from the plan when
// one is on the ledger) and runs verify on it: a floor miss is a failed
// attempt, and the attempts budget or a second miss on a never layer moves
// the story to status:blocked and pages. A checkpoint that holds is pushed and
// the PR opened. The progress comment and build/<n>/state.json on the ledger
// record every run.
func runBuild(ctx context.Context, e *Env, in Values) error {
	root := config.RepoRoot(e.Dir)
	if root == "" {
		return Errorf(ExitUsage, "build runs in a checkout of the repository")
	}
	repo, err := e.RepoName(ctx)
	if err != nil {
		return err
	}
	l, err := e.Config()
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
	labels := github.IssueLabels(is)
	why, err := e.notReady(ctx, c, repo, n, chk, labels)
	if err != nil {
		return Wrap(ExitExternal, fmt.Errorf("build #%d: reading the Ready snapshot: %w", n, err))
	}
	if why != "" {
		return Errorf(ExitBlocked, "build #%d: %s", n, why)
	}
	// The branch, commits, and PR title use the title without its issue
	// form's "Story: " prefix (D-5.5c.19).
	chk.Title = workitems.BareTitle(chk.Title)
	if err := e.askedGate(ctx, c, repo, n, chk.Title, labels, in); err != nil {
		return err
	}
	res := BuildResult{Issue: n, Worktree: BuildWorktree(root, n), Branch: fmt.Sprintf("story/%d-%s", n, slug(chk.Title))}
	dev := e.devBranch()
	w, err := e.Ledger(ctx, bot.Token, c.Identity(), bot.Tag)
	if err != nil {
		return err
	}
	var st BuildState
	if _, err := readLedgerJSON(ctx, w, BuildLedgerPath(n), &st); err != nil {
		return Wrap(ExitExternal, err)
	}
	now := e.Now().UTC()
	if st.Blocked != "" {
		// A human made the story Ready again after the block: the budget starts over.
		st = BuildState{Unblocked: append(st.Unblocked, now.Format(time.RFC3339)+" after: "+st.Blocked), Checkpoints: st.Checkpoints}
	}
	if st.StartedAt == "" {
		st.StartedAt = now.Format(time.RFC3339)
	}
	st.Story, st.Branch = n, res.Branch
	if res.Lease, err = e.takeLease(ctx, c, repo, n, leaseHolder(bot.Login), labels); err != nil {
		return err
	}
	// A story being built is in progress (D-5.5c.17): build is the verb that
	// moves it there from status:ready, and later runs accept either label.
	if !contains(labels, labelInProgress) {
		if err := setStatus(ctx, c, repo, n, labels, "in-progress"); err != nil {
			return Wrap(ExitExternal, err)
		}
		labels = append(slices.DeleteFunc(labels, func(l string) bool { return strings.HasPrefix(l, "status:") }), labelInProgress)
	}
	b := l.Config.Autonomy.Budget
	// A story's spend is capped by the lower of the build budget and the
	// breaker's per-story limit, so a config that keeps a lower per_story_usd
	// is reported, and blocks, at the limit that actually stops it. A cap of 0
	// is off on purpose and stays off.
	if p := l.Config.Budget.PerStoryUSD; p > 0 && b.CostUSD > 0 && p < b.CostUSD && e.meteredBy(&l.Config) != "" {
		b.CostUSD = p
	}
	res.Budget = Usage{MaxWall: b.WallHours, MaxCost: b.CostUSD, MaxAttempts: b.Attempts, Attempts: st.Attempts, CostUSD: e.buildCost(repo, n), MeteredBy: e.meteredBy(&l.Config)}
	if started, err := time.Parse(time.RFC3339, st.StartedAt); err == nil {
		res.Budget.WallHours = now.Sub(started).Hours()
	}
	block := func(why string) error { return e.blockBuild(ctx, c, w, repo, n, labels, &st, &res, why) }
	switch {
	case b.WallHours > 0 && res.Budget.WallHours > b.WallHours:
		return block(fmt.Sprintf("wall-time budget breached: %.1f h since the build started, budget %.1f h", res.Budget.WallHours, b.WallHours))
	case b.CostUSD > 0 && res.Budget.MeteredBy != "" && res.Budget.CostUSD > b.CostUSD:
		return block(fmt.Sprintf("cost budget breached: metered spend %.2f USD of %g (subscription use not counted)", res.Budget.CostUSD, b.CostUSD))
	}

	// Delegation (WP5.5b): another agent writes the change. A cloud receiver
	// works on its own branch and this run hands off or syncs; a local one, or a
	// receiver that returns a diff, writes into an aayu worktree, and the
	// checkpoint, verify, push, and PR below follow as for any build.
	var write func(context.Context, string) error
	var dlg *delegation
	if in.String("delegate") != "" {
		if dlg, err = e.newDelegation(ctx, in, c, bot, w, repo, root, n, chk, is.GetBody(), labels); err != nil {
			return err
		}
		var done bool
		write, done, err = e.delegateStep(ctx, dlg, chk, &res)
		if done {
			if eerr := e.emitDelegation(res); eerr != nil && err == nil {
				err = eerr
			}
			return err
		}
		if err != nil {
			return err
		}
		res.Worktree, res.Branch = delegate.Worktree(root, n, dlg.receiver, dlg.handoff), dlg.branch
		st.Branch = res.Branch
		if started, err := time.Parse(time.RFC3339, st.StartedAt); err == nil && b.WallHours > 0 {
			dlg.wallEnd = started.Add(time.Duration(b.WallHours * float64(time.Hour)))
		}
	}

	// The worktree: cut from origin/dev, or resumed from the story branch.
	if _, err := os.Stat(res.Worktree); err != nil {
		if _, err := run2(ctx, root, "git", "fetch", "--quiet", "origin", dev); err != nil {
			return Wrap(ExitExternal, err)
		}
		remote, _ := run2(ctx, root, "git", "ls-remote", "--heads", "origin", res.Branch)
		switch {
		case remote != "" && !in.Bool("resume") && dlg == nil:
			return Errorf(ExitBlocked, "build #%d: %s already exists on origin; `aayu build %d --resume` continues from its last checkpoint", n, res.Branch, n)
		case remote != "":
			if _, err := run2(ctx, root, "git", "fetch", "--quiet", "origin", "+refs/heads/"+res.Branch+":refs/remotes/origin/"+res.Branch); err != nil {
				return Wrap(ExitExternal, err)
			}
			if _, err := run2(ctx, root, "git", "worktree", "add", "--quiet", "-B", res.Branch, res.Worktree, "origin/"+res.Branch); err != nil {
				return Wrap(ExitExternal, err)
			}
			res.Resumed = lastTrailer(ctx, res.Worktree, dev)
		default:
			if _, err := run2(ctx, root, "git", "worktree", "add", "--quiet", "-B", res.Branch, res.Worktree, "origin/"+dev); err != nil {
				return Wrap(ExitExternal, err)
			}
		}
		res.Created = true
		if write == nil { // the host's agent writes the change, then runs build again
			res.Next = fmt.Sprintf("write the change in %s (tests included; map criteria to tests in .aayudha/evidence/%d.toml), then run `aayu build %d` again to commit a checkpoint", res.Worktree, n, n)
			if res.Resumed != "" {
				res.Next = fmt.Sprintf("resumed after checkpoint %s; continue in %s, then run `aayu build %d` again", res.Resumed, res.Worktree, n)
			}
			if err := e.saveBuild(ctx, c, w, repo, n, &st, &res); err != nil {
				return err
			}
			return e.Emit(res, func(w io.Writer) error {
				_, err := fmt.Fprintf(w, "#%d %s: worktree %s on %s (leased by %s until %s)\n  next: %s\n", n, chk.Title, res.Worktree, res.Branch, res.Lease.Holder, res.Lease.Expires.UTC().Format(time.RFC3339), res.Next)
				return err
			})
		}
	} else if in.Bool("resume") {
		res.Resumed = lastTrailer(ctx, res.Worktree, dev)
	}

	if write != nil {
		err := write(ctx, res.Worktree)
		if dlg != nil && dlg.lease != nil { // renewed while the receiver waited out a usage limit
			res.Lease = *dlg.lease
		}
		// The wall clock ran on while the receiver worked, or waited out a usage limit.
		if started, perr := time.Parse(time.RFC3339, st.StartedAt); perr == nil {
			res.Budget.WallHours = e.Now().Sub(started).Hours()
		}
		if err != nil {
			// A usage limit build cannot wait out here is a pause, not a
			// failure: no attempt is counted, and the wall clock runs on.
			var pause *usagePause
			if errors.As(err, &pause) {
				dlg.event(e.Now(), dlgPaused, pause.why)
				dlg.fill(res.Delegation)
				_ = e.saveDelegation(ctx, dlg)
				res.Next = fmt.Sprintf("run `aayu build %d --delegate %s --local` again once the limit resets; a pause is no failed attempt", n, dlg.receiver)
				if pause.afterWall {
					res.Next = fmt.Sprintf("the wall-time budget ends before the limit resets: once it has reset, a person runs `aayu ready %d` for a fresh budget, then `aayu build %d --delegate %s --local`", n, n, dlg.receiver)
				}
				if err := e.saveBuild(ctx, c, w, repo, n, &st, &res); err != nil {
					return err
				}
				_ = e.Emit(res, func(w io.Writer) error {
					_, err := fmt.Fprintf(w, "#%d: paused, not failed: %s\n  budget: %s\n  next: %s\n", n, pause.why, res.Budget.Line(), res.Next)
					return err
				})
				return WithNext(Errorf(ExitBlocked, "build #%d paused: %s", n, pause.why), res.Next)
			}
			dlg.event(e.Now(), dlgFailed, clipText(err.Error(), 200))
			_ = e.saveDelegation(ctx, dlg)
			// A receiver stopped at the wall-time deadline is the budget running
			// out, so the story is blocked and its owner paged, as at a checkpoint.
			if !dlg.wallEnd.IsZero() && !e.Now().Before(dlg.wallEnd) {
				return block(fmt.Sprintf("wall-time budget breached: %s was still running when the %g h budget ended", delegate.Names[dlg.receiver], b.WallHours))
			}
			return Errorf(ExitExternal, "build #%d --delegate %s: %v", n, dlg.receiver, err)
		}
		dlg.fill(res.Delegation) // what the receiver says it proved, for the PR body
	}

	// A checkpoint: commit what the agent wrote, then verify it.
	dirty, err := run2(ctx, res.Worktree, "git", "status", "--porcelain")
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	if dirty != "" {
		if _, err := run2(ctx, res.Worktree, "git", "add", "--all"); err != nil {
			return Wrap(ExitExternal, err)
		}
		names, _ := run2(ctx, res.Worktree, "git", "diff", "--cached", "--name-only", "HEAD")
		lg := autonomy.ReadLayers(res.Worktree)
		layers := lg.Footprint(strings.Fields(names))
		var plan PlanRecord
		hasPlan, _ := readLedgerJSON(ctx, w, PlanLedgerPath(n), &plan)
		k, attempt := nextCheckpoint(ctx, st, res.Worktree, dev)
		total := max(k, len(lg.Footprint(changedSince(ctx, res.Worktree, dev, names))))
		if hasPlan && len(plan.Checkpoints) > 0 {
			total = max(k, len(plan.Checkpoints))
		}
		res.Checkpoint, res.Attempt = fmt.Sprintf("%d/%d", k, total), attempt
		msg := fmt.Sprintf("feat: %s (#%d, checkpoint %s)\n\nAayudha-Story: #%d\nAayudha-Checkpoint: %s", strings.ToLower(chk.Title[:1])+chk.Title[1:], n, res.Checkpoint, n, res.Checkpoint)
		if attempt > 1 {
			msg += fmt.Sprintf("\nAayudha-Attempt: %d", attempt)
		}
		model := e.builderModel()
		switch {
		case dlg != nil:
			msg += "\nAayudha-Delegate: " + dlg.receiver
			if model = dlg.model(); model != "" {
				msg += "\nAayudha-Model: " + model
			}
		case model != "":
			msg += "\nAayudha-Model: " + model
		}
		// The host that built the checkpoint, so inspect can place a builder
		// whose host picks the model (Decision #469); a delegated build names
		// its receiver instead.
		if host, _ := doctor.DetectHost(e.Host, e.Getenv); host != "" && dlg == nil {
			msg += "\nAayudha-Host: " + host
		}
		if _, err := run2(ctx, res.Worktree, "git", "commit", "--quiet", "--signoff", "-m", msg); err != nil {
			return Wrap(ExitExternal, err)
		}
		res.Commit, _ = run2(ctx, res.Worktree, "git", "rev-parse", "HEAD")
		rec := CheckpointRecord{K: k, N: total, Attempt: attempt, Commit: res.Commit, Layers: layers, At: now.Format(time.RFC3339), Model: model}
		if rec.Layers == nil {
			rec.Layers = []string{}
		}
		on := ""
		if len(layers) > 0 {
			on = " on " + strings.Join(layers, ", ")
		}
		vp := e.progress(fmt.Sprintf("Checkpoint %s%s: verifying the floors", res.Checkpoint, on))
		hold, detail, verr := e.checkpointVerify(ctx, res.Worktree)
		switch {
		case verr != nil:
			vp.Done(ui.StatusWarn, "verify could not run")
		case hold:
			vp.Done(ui.StatusPass, "floors hold")
		default:
			vp.Done(ui.StatusFail, "floor miss: "+detail)
		}
		switch {
		case verr != nil:
			rec.Verify, rec.Detail = "error", verr.Error()
			res.Notes = append(res.Notes, "verify could not run on the checkpoint: "+verr.Error())
		case hold:
			rec.Verify, rec.Detail = "hold", detail
		default:
			rec.Verify, rec.Detail = "miss", detail
		}
		res.Verify = rec.Verify
		st.Checkpoints = append(st.Checkpoints, rec)
		if rec.Verify == "miss" {
			st.Attempts++
			res.Budget.Attempts = st.Attempts
			if st.Failures == nil {
				st.Failures = map[string]int{}
			}
			for _, x := range rec.Layers {
				st.Failures[x]++
			}
			for _, x := range rec.Layers {
				if slices.Contains(l.Config.Autonomy.Never, x) && st.Failures[x] >= 2 {
					return block(fmt.Sprintf("second verify failure on never layer %s (checkpoint %s: %s)", x, res.Checkpoint, detail))
				}
			}
			if b.Attempts > 0 && st.Attempts >= b.Attempts {
				return block(fmt.Sprintf("attempts budget breached: %d failed checkpoint attempts, budget %d (last: %s)", st.Attempts, b.Attempts, detail))
			}
			if err := e.saveBuild(ctx, c, w, repo, n, &st, &res); err != nil {
				return err
			}
			res.Next = fmt.Sprintf("fix the floor miss in %s and run `aayu build %d` again (failed attempts %d of %d)", res.Worktree, n, st.Attempts, b.Attempts)
			_ = e.Emit(res, func(w io.Writer) error {
				_, err := fmt.Fprintf(w, "#%d: checkpoint %s (attempt %d) committed (%s); verify: floor miss: %s\n  budget: %s\n  next: %s\n", n, res.Checkpoint, attempt, short(res.Commit), detail, res.Budget.Line(), res.Next)
				return err
			})
			return WithNext(Errorf(ExitFloor, "checkpoint %s: floor miss: %s", res.Checkpoint, detail), fmt.Sprintf("aayu build %d", n))
		}
	}
	ahead, _ := run2(ctx, res.Worktree, "git", "rev-list", "--count", "origin/"+dev+"..HEAD")
	if ahead == "0" {
		if err := e.saveBuild(ctx, c, w, repo, n, &st, &res); err != nil {
			return err
		}
		if dlg != nil {
			// The receiver changed nothing: keep its report and the hand-off's
			// state, so the person sees why (D-5.5c.16), and let the next run
			// hand off afresh.
			dlg.event(e.Now(), dlgFailed, "the local "+dlg.receiver+" run changed nothing")
			dlg.fill(res.Delegation)
			if err := e.saveDelegation(ctx, dlg); err != nil {
				return err
			}
			res.Next = fmt.Sprintf("%s changed nothing in %s; read its report, then `aayu build %d --delegate %s --local` hands off again in a fresh worktree", dlg.receiver, res.Worktree, n, dlg.receiver)
			if err := e.emitDelegation(res); err != nil {
				return err
			}
			return Errorf(ExitBlocked, "build #%d --delegate %s: the receiver changed nothing in %s", n, dlg.receiver, res.Worktree)
		}
		return Errorf(ExitBlocked, "build #%d: nothing to commit yet in %s", n, res.Worktree)
	}
	pp := e.progress(fmt.Sprintf("Pushing %s and opening the pull request", res.Branch))
	perr := e.publishBuild(ctx, c, bot.Token.Value, root, repo, dev, n, chk, &res)
	if perr != nil {
		pp.Done(ui.StatusFail, perr.Error())
	} else {
		pp.Done(ui.StatusPass, fmt.Sprintf("PR #%d", res.PR))
	}
	if err := perr; err != nil {
		// The checkpoint is committed: record it even though the push or the PR
		// failed, so the next run (or --resume) continues after it.
		if serr := e.saveBuild(ctx, c, w, repo, n, &st, &res); serr != nil {
			fmt.Fprintf(e.Err, "aayu: warning: the build state did not reach the ledger: %v\n", serr)
		}
		return err
	}
	if err := e.saveBuild(ctx, c, w, repo, n, &st, &res); err != nil {
		return err
	}
	if dlg != nil {
		dlg.state.PR = res.PR
		dlg.event(e.Now(), dlgPROpen, fmt.Sprintf("checkpoint %s pushed to PR #%d", res.Checkpoint, res.PR))
		dlg.fill(res.Delegation)
		if err := e.saveDelegation(ctx, dlg); err != nil {
			return err
		}
	}
	// The actions segment of this checkpoint goes to the ledger (WP1.5), pushed with the build's own token.
	if err := e.checkpoint(bot.Token); err != nil {
		fmt.Fprintf(e.Err, "aayu: warning: the actions segment did not reach the ledger: %v\n", err)
	}
	// Every push asks the reviewer app for a review of the new head, under the
	// person's account, so the merge gate's inspect has a reply to read even
	// when the app reviews by itself only when a PR opens.
	if note := e.requestReview(ctx, res.PR); note != "" {
		res.Notes = append(res.Notes, note)
	}
	res.Next = fmt.Sprintf("the merge gate re-runs verify and inspect on PR #%d (the reviewer app was asked to review this head); `aayu ship %d` after a human pass", res.PR, res.PR)
	return e.Emit(res, func(w io.Writer) error {
		if res.Commit != "" {
			fmt.Fprintf(w, "#%d: checkpoint %s committed (%s); verify: %s\n  budget: %s\n", n, res.Checkpoint, short(res.Commit), res.Verify, res.Budget.Line())
		}
		for _, x := range res.Notes {
			fmt.Fprintf(w, "  note: %s\n", x)
		}
		_, err := fmt.Fprintf(w, "#%d: branch %s pushed; PR #%d %s\n  next: %s\n", n, res.Branch, res.PR, res.PRURL, res.Next)
		return err
	})
}

// publishBuild pushes the story branch and finds or opens its PR.
func (e *Env) publishBuild(ctx context.Context, c *github.Client, token, root, repo, dev string, n int, chk ready.Result, res *BuildResult) error {
	gw := &ledger.Writer{Dir: root, Token: token}
	if !c.DryRun() {
		if _, err := c.Write(ctx, actions.Action{Kind: "git.push", Target: repo + "@" + res.Branch, Payload: map[string]any{"branch": res.Branch, "commit": res.Commit}},
			func(ctx context.Context) (any, error) {
				return nil, gw.Exec(ctx, res.Worktree, "push", "--quiet", "--force-with-lease", "origin", "HEAD:refs/heads/"+res.Branch)
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
		body := fmt.Sprintf("Refs #%d\n\nBuilt with `aayu build` (WP5.5) from the Ready snapshot of #%d (body hash `%s`), one verified checkpoint commit per layer. The merge gate verifies it on the base branch's definition; `aayu ship` merges it after a human pass.\n", n, n, chk.BodyHash[:12])
		if d := res.Delegation; d != nil {
			body = fmt.Sprintf("Refs #%d\n\nBuilt by %s (%s, `aayu build --delegate`, WP5.5b) from the delegation brief of #%d's Ready snapshot (body hash `%s`). aayu committed, verified, and pushed the change; the receiver has no merge rights. The merge gate and inspect review it; `aayu ship` merges it after a human pass.\n", n, d.Receiver, d.Mode, n, chk.BodyHash[:12])
			if d.Proved != "" {
				body += fmt.Sprintf("\n%s\n\n%s's own check, quoted as it reported it (a preview; the merge gate's proof decides):\n\n%s\n", delegate.ProvedHeading, d.Receiver, quoteLines(d.Proved))
			}
		}
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
	return nil
}

// changedSince lists the paths the branch changes against origin/dev plus the
// staged ones.
func changedSince(ctx context.Context, wt, dev, staged string) []string {
	names, _ := run2(ctx, wt, "git", "diff", "--name-only", "origin/"+dev+"...HEAD")
	return append(strings.Fields(names), strings.Fields(staged)...)
}

// lastTrailer is the newest Aayudha-Checkpoint trailer on the branch.
func lastTrailer(ctx context.Context, wt, dev string) string {
	out, _ := run2(ctx, wt, "git", "log", "-1", "--format=%(trailers:key=Aayudha-Checkpoint,valueonly)", "origin/"+dev+"..HEAD")
	return strings.TrimSpace(out)
}

// nextCheckpoint numbers the commit this run makes: the next checkpoint
// after the last one that held, or another attempt at one that missed. The
// ledger state says which; without it the branch's trailers do (--resume on
// another machine).
func nextCheckpoint(ctx context.Context, st BuildState, wt, dev string) (k, attempt int) {
	if len(st.Checkpoints) > 0 && st.Checkpoints[len(st.Checkpoints)-1].Commit != "" {
		last := st.Checkpoints[len(st.Checkpoints)-1]
		if last.Verify == "miss" {
			return last.K, last.Attempt + 1
		}
		return last.K + 1, 1
	}
	if t := lastTrailer(ctx, wt, dev); t != "" {
		k, _ := strconv.Atoi(strings.Split(t, "/")[0])
		return k + 1, 1
	}
	return 1, 1
}

// meteredBy names what lets aayu's own model calls incur pay-as-you-go spend:
// the [models.gateway] endpoint, or a direct provider's key in the secret
// store. With neither, the cost limit is off: work in a subscription host
// session reports no cost, so a limit on metered spend would mean nothing.
func (e *Env) meteredBy(c *config.Config) string {
	if c.Models.Gateway.BaseURL != "" {
		return "[models.gateway] base_url"
	}
	for _, name := range models.DirectSecrets() {
		if v, _, err := e.SecretStore().Get(name); err == nil && v != "" {
			return "the " + name + " key"
		}
	}
	// A host or wrapper that reports its pay-as-you-go spend is metered too.
	if v, err := strconv.ParseFloat(e.Getenv("AAYU_COST_USD"), 64); err == nil && v > 0 {
		return "AAYU_COST_USD"
	}
	return ""
}

// buildCost is the story's model spend: the gateway's calls in the actions
// log plus what the host reports in AAYU_COST_USD.
func (e *Env) buildCost(repo string, n int) float64 {
	cost := e.costs(repo)[n]
	if v, err := strconv.ParseFloat(e.Getenv("AAYU_COST_USD"), 64); err == nil && v > 0 {
		cost += v
	}
	return cost
}

// checkpointVerify runs verify on a checkpoint in its worktree. A floor that
// fails only because its tool is not installed here does not count as a miss
// (the merge gate runs it with the pinned tools); it is named in the detail.
func (e *Env) checkpointVerify(ctx context.Context, wt string) (hold bool, detail string, err error) {
	if e.CheckpointVerify != nil {
		return e.CheckpointVerify(ctx, wt)
	}
	sub := *e
	sub.Dir, sub.config, sub.JSON = wt, nil, true
	var out bytes.Buffer
	sub.Out, sub.Err = &out, io.Discard
	verr := runVerify(ctx, &sub, Values{})
	var res VerifyResult
	if jerr := json.NewDecoder(&out).Decode(&res); jerr != nil {
		if verr != nil {
			return false, "", verr
		}
		return false, "", jerr
	}
	var miss, absent []string
	for _, ck := range res.Checks {
		if (&proof.Pack{Checks: []proof.Check{ck}}).FloorsHold() {
			continue
		}
		if strings.Contains(ck.Detail, "is not installed") {
			absent = append(absent, ck.Floor)
			continue
		}
		miss = append(miss, ck.Floor+" "+ck.Result)
	}
	switch {
	case len(miss) > 0:
		detail = strings.Join(miss, ", ")
	case len(absent) > 0:
		detail = "floors hold; not run here (tool not installed; the merge gate runs them): " + strings.Join(absent, ", ")
	default:
		detail = "floors hold"
	}
	return len(miss) == 0, detail, nil
}

// blockBuild moves the story to status:blocked, pages, and records why.
func (e *Env) blockBuild(ctx context.Context, c *github.Client, w *ledger.Writer, repo string, n int, labels []string, st *BuildState, res *BuildResult, why string) error {
	st.Blocked = why
	if err := setStatus(ctx, c, repo, n, labels, "blocked"); err != nil {
		return Wrap(ExitExternal, err)
	}
	sent := e.page(ctx, notify.Page{Key: fmt.Sprintf("build:%s#%d", repo, n), Subject: fmt.Sprintf("aayu build blocked #%d", n),
		Body: fmt.Sprintf("%s#%d is status:blocked: %s. A human looks at it and runs `aayu ready %d` to let the build continue.", repo, n, why, n)})
	res.Notes = append(res.Notes, "page: "+sent)
	if err := e.saveBuild(ctx, c, w, repo, n, st, res); err != nil {
		return err
	}
	res.Next = fmt.Sprintf("a human looks at #%d; `aayu ready %d` lets the build continue with a fresh budget", n, n)
	_ = e.Emit(res, func(w io.Writer) error {
		_, err := fmt.Fprintf(w, "#%d: status:blocked: %s (page %s)\n  next: %s\n", n, why, sent, res.Next)
		return err
	})
	return Errorf(ExitBlocked, "build #%d blocked: %s", n, why)
}

const progressMarker = "<!-- aayudha:progress -->"

// saveBuild writes the build state to the ledger and the replace-in-place
// progress comment on the story.
func (e *Env) saveBuild(ctx context.Context, c *github.Client, w *ledger.Writer, repo string, n int, st *BuildState, res *BuildResult) error {
	if c.DryRun() {
		return nil
	}
	b, _ := json.MarshalIndent(st, "", "  ")
	if _, err := w.Write(ctx, ledger.Commit{Message: fmt.Sprintf("build: state of #%d", n), Files: map[string][]byte{BuildLedgerPath(n): append(b, '\n')}}); err != nil {
		return Wrap(ExitExternal, err)
	}
	var body strings.Builder
	fmt.Fprintf(&body, "%s\n**Build progress** (`aayu build`, branch `%s`", progressMarker, st.Branch)
	if res.PR > 0 {
		fmt.Fprintf(&body, ", PR #%d", res.PR)
	}
	body.WriteString(")\n\n| Checkpoint | Attempt | Layers | Commit | Verify |\n|---|---|---|---|---|\n")
	for _, cp := range st.Checkpoints {
		fmt.Fprintf(&body, "| %d/%d | %d | %s | `%s` | %s: %s |\n", cp.K, cp.N, cp.Attempt, listOrNone(cp.Layers), short(cp.Commit), cp.Verify, cp.Detail)
	}
	u := res.Budget
	fmt.Fprintf(&body, "\nBudget: %s.", u.Line())
	if st.Blocked != "" {
		fmt.Fprintf(&body, "\n\n**Blocked:** %s", st.Blocked)
	}
	return e.replaceComment(ctx, c, repo, n, progressMarker, body.String())
}

// replaceComment edits the story's comment that starts with marker, or posts it.
func (e *Env) replaceComment(ctx context.Context, c *github.Client, repo string, n int, marker, body string) error {
	owner, name, _ := github.SplitRepo(repo)
	cs, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.IssueComment, *gh.Response, error) {
		return c.REST.Issues.ListComments(ctx, owner, name, n, &gh.IssueListCommentsOptions{ListOptions: o})
	})
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	target := fmt.Sprintf("%s#%d", repo, n)
	for _, cm := range cs {
		if strings.HasPrefix(cm.GetBody(), marker) {
			id := cm.GetID()
			_, err := c.Write(ctx, actions.Action{Kind: "github.issue.comment.edit", Target: target,
				Request: &actions.Request{Method: http.MethodPatch, Path: fmt.Sprintf("/repos/%s/issues/comments/%d", repo, id)}, Payload: map[string]any{"marker": marker}},
				func(ctx context.Context) (any, error) {
					_, _, err := c.REST.Issues.EditComment(ctx, owner, name, id, &gh.IssueComment{Body: &body})
					return nil, err
				})
			return wrapExternal(err)
		}
	}
	_, err = c.Write(ctx, actions.Action{Kind: "github.issue.comment", Target: target,
		Request: &actions.Request{Method: http.MethodPost, Path: fmt.Sprintf("/repos/%s/issues/%d/comments", repo, n)}, Payload: map[string]any{"marker": marker}},
		func(ctx context.Context) (any, error) {
			_, _, err := c.REST.Issues.CreateComment(ctx, owner, name, n, &gh.IssueComment{Body: &body})
			return nil, err
		})
	return wrapExternal(err)
}

func wrapExternal(err error) error {
	if err == nil {
		return nil
	}
	return Wrap(ExitExternal, err)
}

// labelInProgress is the Delivery Status build gives a story it builds.
const labelInProgress = "status:in-progress"

// notReady says why a story cannot be built: not labelled status:ready (or
// status:in-progress, which an earlier build run set), not passing Ready, or
// its body hash differs from the Ready snapshot's. The Ready guard acts only
// on status:ready, so the hash check here is what catches a body edited
// while the story is in progress.
// A failure to read the snapshot is an error, not a reason: the story may
// well be Ready.
func (e *Env) notReady(ctx context.Context, c *github.Client, repo string, n int, chk ready.Result, labels []string) (string, error) {
	switch {
	case !chk.Pass:
		return fmt.Sprintf("it does not pass Ready (%d problem(s)); run `aayu ready %d --check`", len(chk.Problems), n), nil
	case !contains(labels, ready.LabelReady) && !contains(labels, labelInProgress):
		return "it is not labelled status:ready; run `aayu ready " + fmt.Sprint(n) + "`", nil
	}
	snap, err := ready.SnapshotHash(ctx, c, repo, n)
	if err != nil {
		return "", err
	}
	if snap != chk.BodyHash {
		return fmt.Sprintf("its body changed after Ready (snapshot %s, now %s); run `aayu ready %d` again", short(snap), short(chk.BodyHash), n), nil
	}
	return "", nil
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
