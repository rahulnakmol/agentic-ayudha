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
	"github.com/rahulnakmol/agentic-ayudha/internal/hosts"
	"github.com/rahulnakmol/agentic-ayudha/internal/identity"
	"github.com/rahulnakmol/agentic-ayudha/internal/inspect"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/models"
	"github.com/rahulnakmol/agentic-ayudha/internal/notify"
	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
	"github.com/rahulnakmol/agentic-ayudha/internal/ready"
	"github.com/rahulnakmol/agentic-ayudha/internal/ui"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
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
	// Review is the session review of the head before its push (Decision #656).
	Review *SessionReview `json:"session_review,omitempty"`
	// Pick is set when the build-ready loop picked the story (--next pick).
	Pick *BuildPick `json:"pick,omitempty"`
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
	// ReviewUSD is what the session review spent on the story's PR: on the
	// ledger, which the PR cannot change, so a resumed build and the merge
	// gate count it against [budget] per_pr_inspect_max.
	ReviewUSD float64 `json:"session_review_usd,omitempty"`
	// ReviewPR is the PR that spend belongs to; 0 before the PR is opened.
	ReviewPR int `json:"session_review_pr,omitempty"`
	// Loop names the loop that claimed the story (BuildLoop, `build --next
	// pick`): its claim and its pull request count toward [build] agents.
	Loop string `json:"loop,omitempty"`
	// Claim is the loop pick's line in LoopClaimsPath: once a state carries
	// it, the line no longer holds a slot of its own.
	Claim string `json:"claim,omitempty"`
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
	mode := in.String("next")
	switch {
	case mode != "" && (n != 0 || in.String("stop") != "" || in.String("delegate") != "" || in.Bool("resume")):
		return Errorf(ExitUsage, "build --next picks the story itself: give no issue, --stop, --delegate, or --resume with it")
	case mode == "" && n == 0:
		return Errorf(ExitUsage, "build: name the Ready story or bug to build, or run `aayu build --next pick` (the build-ready loop)")
	}
	c, bot, err := e.BotGitHub(ctx, "")
	if err != nil {
		return err
	}
	var pick *BuildPick
	if mode == "pick" {
		unlock, err := e.lockPick()
		if err != nil {
			return err
		}
		defer unlock()
	}
	if mode != "" {
		pw, err := e.Ledger(ctx, bot.Token, c.Identity(), bot.Tag)
		if err != nil {
			return err
		}
		if pick, n, err = e.pickNext(ctx, c, pw, l, repo, mode); err != nil {
			return err
		}
		if n == 0 || c.DryRun() {
			return e.emitPick(pick) // a dry run previews the pick and builds nothing
		}
		// The loop a person set up is the ask (D-5.5c.20): its words go to
		// the actions log like a person's.
		in["asked"] = fmt.Sprintf("the %s loop picked #%d (aayu build --next pick, loops/%s.toml): %s", BuildLoop, n, BuildLoop, pick.Order)
	}
	rd := &ready.Ready{GH: c, Repo: repo, TechnicalOwner: e.technicalOwner()}
	chk, is, err := rd.Check(ctx, n)
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	labels := github.IssueLabels(is)
	if why := in.String("stop"); why != "" {
		return e.stopBuild(ctx, c, bot, l, repo, n, labels, why)
	}
	if in.String("delegate") != "" { // a Kanha starts what a mission list marked start: it acts on the live state only
		lw, err := e.Ledger(ctx, bot.Token, c.Identity(), bot.Tag)
		if err != nil {
			return err
		}
		if err := e.changedSinceList(ctx, lw, is, chk.Pass, labels, c.DryRun()); err != nil {
			return err
		}
		if err := tatkalHold(ctx, c, lw, repo, n); err != nil {
			return err
		}
	}
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
	res := BuildResult{Issue: n, Worktree: BuildWorktree(root, n), Branch: fmt.Sprintf("story/%d-%s", n, slug(chk.Title)), Pick: pick}
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
		// A loop's build stays the loop's: its PR still counts toward [build] agents.
		st = BuildState{Unblocked: append(st.Unblocked, now.Format(time.RFC3339)+" after: "+st.Blocked), Checkpoints: st.Checkpoints, ReviewUSD: st.ReviewUSD, ReviewPR: st.ReviewPR, Loop: st.Loop, Claim: st.Claim}
	}
	if st.StartedAt == "" {
		st.StartedAt = now.Format(time.RFC3339)
	}
	st.Story, st.Branch = n, res.Branch
	if pick != nil {
		st.Loop, st.Claim = BuildLoop, pick.Claim
	}
	if st.Loop != "" {
		// No person asked for a loop's build, so its limits are the committed
		// policy of the trusted integration branch, as for [build] agents: AAYU_* or
		// --set may tighten one, never loosen it or turn it off. The run's
		// config is clamped in place, so the session review spends within it too.
		owner, name, _ := github.SplitRepo(repo)
		pol, trusted, err := e.loopPolicy(ctx, c, owner, name)
		if err != nil {
			return err
		}
		// Its worktree is cut from, and its PR opened against, that branch too.
		dev = trusted
		lb, cb := &l.Config.Autonomy.Budget, pol.Config.Autonomy.Budget
		lb.WallHours, lb.CostUSD, lb.Attempts = tighter(lb.WallHours, cb.WallHours), tighter(lb.CostUSD, cb.CostUSD), tighter(lb.Attempts, cb.Attempts)
		lk, ck := &l.Config.Budget, pol.Config.Budget
		lk.PerStoryUSD, lk.PerDayUSD = tighter(lk.PerStoryUSD, ck.PerStoryUSD), tighter(lk.PerDayUSD, ck.PerDayUSD)
		lk.PerPRInspectMax, lk.CIMinutesMonth = tighter(lk.PerPRInspectMax, ck.PerPRInspectMax), tighter(lk.CIMinutesMonth, ck.CIMinutesMonth)
	}
	b, perStory := l.Config.Autonomy.Budget, l.Config.Budget.PerStoryUSD
	// A story's spend is capped by the lower of the build budget and the
	// breaker's per-story limit, so a config that keeps a lower per_story_usd
	// is reported, and blocks, at the limit that actually stops it. A cap of 0
	// is off on purpose and stays off.
	if p := perStory; p > 0 && b.CostUSD > 0 && p < b.CostUSD && e.meteredBy(&l.Config) != "" {
		b.CostUSD = p
	}
	res.Budget = Usage{MaxWall: b.WallHours, MaxCost: b.CostUSD, MaxAttempts: b.Attempts, Attempts: st.Attempts, CostUSD: e.buildCost(repo, n), MeteredBy: e.meteredBy(&l.Config)}
	if started, err := time.Parse(time.RFC3339, st.StartedAt); err == nil {
		res.Budget.WallHours = now.Sub(started).Hours()
	}
	if res.Lease, err = e.takeLease(ctx, c, repo, n, leaseHolder(bot.Login), labels); err != nil {
		return err
	}
	if pick != nil {
		// Two runs that picked the same story both wrote a lock comment; the
		// lease is the first one, so the run that does not hold it stops.
		owner, name, _ := github.SplitRepo(repo)
		if cur, err := readLease(ctx, c, owner, name, n); err != nil {
			return Wrap(ExitExternal, err)
		} else if cur != nil && cur.CommentID != res.Lease.CommentID {
			return Errorf(ExitBlocked, "build --next: another run claimed #%d first (%s)", n, cur.Holder)
		}
		// The loop's claim reaches the ledger before its status changes or its
		// worktree is cut: a later failure leaves a claim [build] agents counts and,
		// once its lease runs out, a Ready item the queue picks again.
		if err := e.saveBuild(ctx, c, w, repo, n, &st, &res); err != nil {
			return err
		}
	}
	// A story being built is in progress (D-5.5c.17): build is the verb that
	// moves it there from status:ready, and later runs accept either label.
	starts := !contains(labels, labelInProgress) // this run starts the story
	if starts {
		if err := setStatus(ctx, c, repo, n, labels, "in-progress"); err != nil {
			return Wrap(ExitExternal, err)
		}
		labels = append(slices.DeleteFunc(labels, func(l string) bool { return strings.HasPrefix(l, "status:") }), labelInProgress)
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
		var done bool
		if dlg, err = e.newDelegation(ctx, in, c, bot, w, repo, root, n, chk, is.GetBody(), labels); err == nil {
			write, done, err = e.delegateStep(ctx, dlg, chk, &res)
		}
		if err != nil && starts && !c.DryRun() {
			err = e.failedStart(ctx, c, w, repo, n, labels, &res, err)
		}
		if dlg == nil {
			return err
		}
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
		// A checkpoint the session review held is on its own ref: it carries
		// on from there when the story branch does not hold it yet.
		held, _ := run2(ctx, root, "git", "ls-remote", "origin", heldRef(res.Branch))
		switch {
		case (remote != "" || held != "") && !in.Bool("resume") && dlg == nil:
			return Errorf(ExitBlocked, "build #%d: %s already exists on origin; `aayu build %d --resume` continues from its last checkpoint", n, res.Branch, n)
		case remote != "" || held != "":
			from := "origin/" + res.Branch
			if remote != "" {
				if _, err := run2(ctx, root, "git", "fetch", "--quiet", "origin", "+refs/heads/"+res.Branch+":refs/remotes/origin/"+res.Branch); err != nil {
					return Wrap(ExitExternal, err)
				}
			} else {
				// The story branch is not on origin: a tracking ref left from an
				// earlier one would make the first push's lease refuse.
				_, _ = run2(ctx, root, "git", "update-ref", "-d", "refs/remotes/origin/"+res.Branch)
			}
			// A checkpoint the session review held is on its own ref: the
			// resume carries on from there when the story branch, if any, does
			// not hold it yet.
			if held != "" {
				if _, err := run2(ctx, root, "git", "fetch", "--quiet", "origin", "+"+heldRef(res.Branch)+":"+heldRef(res.Branch)); err != nil {
					return Wrap(ExitExternal, err)
				}
				if _, err := run2(ctx, root, "git", "merge-base", "--is-ancestor", heldRef(res.Branch), from); remote == "" || err != nil {
					from = heldRef(res.Branch)
				}
			}
			if _, err := run2(ctx, root, "git", "worktree", "add", "--quiet", "-B", res.Branch, res.Worktree, from); err != nil {
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
				if res.Pick != nil {
					writePick(w, res.Pick)
				}
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
				// The wall clock would run out during the wait: that is the budget
				// ending, so the story is blocked (no attempt counted), and
				// `aayu ready` gives the build a fresh budget once the limit resets.
				if pause.afterWall {
					return block("wall-time budget: " + pause.why)
				}
				res.Next = fmt.Sprintf("run `aayu build %d --delegate %s --local` again once the limit resets; a pause is no failed attempt", n, dlg.receiver)
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

	// The session review loop's state (Decision #656): the builder's
	// declines apply to the last round's open items.
	sr, err := loadSessionReview(ctx, res.Worktree)
	if err != nil {
		return Wrap(ExitInternal, fmt.Errorf("build #%d: the session review state: %w", n, err))
	}
	if d := in.String("decline"); d != "" {
		if err := sr.decline(d); err != nil {
			return Errorf(ExitUsage, "build #%d: %v", n, err)
		}
		if err := sr.save(ctx, res.Worktree); err != nil {
			return Wrap(ExitInternal, err)
		}
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
		if last := len(st.Checkpoints) - 1; sr.Pending != "" && last >= 0 && st.Checkpoints[last].Verify == "hold" {
			// A fix for the session review is the same checkpoint again, not
			// a failed attempt: its verify held.
			k, attempt = st.Checkpoints[last].K, st.Checkpoints[last].Attempt+1
		}
		total := max(k, len(lg.Footprint(changedSince(ctx, res.Worktree, dev, names))))
		if hasPlan && len(plan.Checkpoints) > 0 {
			total = max(k, len(plan.Checkpoints))
		}
		res.Checkpoint, res.Attempt = fmt.Sprintf("%d/%d", k, total), attempt
		msg := fmt.Sprintf("feat: %s (#%d, checkpoint %s)\n\nAayudha-Story: #%d\nAayudha-Checkpoint: %s", strings.ToLower(chk.Title[:1])+chk.Title[1:], n, res.Checkpoint, n, res.Checkpoint)
		if attempt > 1 {
			msg += fmt.Sprintf("\nAayudha-Attempt: %d", attempt)
		}
		host, _ := doctor.DetectHost(e.Host, e.Getenv)
		model, effort := e.builderModel()
		var ms []string
		cm, copilot := hosts.CopilotModels{}, false
		if dlg == nil && e.Getenv("AAYU_MODEL") == "" {
			cm, copilot = e.copilotModels(root)
		}
		switch {
		case dlg != nil:
			msg += "\nAayudha-Delegate: " + dlg.receiver
			if model = dlg.model(); model != "" {
				msg += "\nAayudha-Model: " + model
			}
		case copilot:
			// Auto may answer with several models, of several families: the
			// trailers name each one. The build record holds one model, so it
			// names one only when every model Copilot reported is of its family;
			// otherwise it names none, and inspect sees an unknown builder
			// rather than a single family that hides the others.
			msg += "\n" + cm.Trailers()
			model = oneFamily(cm.Models)
		case dlg == nil && host == string(verbs.HostAmp) && e.Getenv("AAYU_MODEL") == "" && e.Getenv("AMP_THREAD_ID") != "":
			// Amp picks the model and says which only through its CLI: one
			// line per Agent model of the thread (else the lane's, as before),
			// and its agent mode as the effort unless AAYU_EFFORT names one.
			// The record holds one model by the Copilot rule.
			var mode string
			ms, mode = e.ampBuilder(ctx, res.Worktree)
			if len(ms) == 0 && model != "" {
				ms = []string{model}
			}
			if mode != "" && e.Getenv("AAYU_EFFORT") == "" {
				effort = mode
			}
			msg += modelTrailers(ms)
			model = oneFamily(ms)
		default:
			if model != "" {
				ms = []string{model}
			}
			msg += modelTrailers(ms)
		}
		// The effort the host runs its model at, one token, once after the last
		// model line: AAYU_EFFORT, else what the host or the lane says (Amp's
		// agent mode, a lane's effort).
		if ef := strings.Fields(effort); dlg == nil && len(ef) == 1 && strings.Contains(msg, "\nAayudha-Model: ") {
			msg += "\nAayudha-Effort: " + ef[0]
		}
		// The host that built the checkpoint, so inspect can place a builder
		// whose host picks the model (Decision #469); a delegated build names
		// its receiver instead, and Copilot's trailers name it already.
		if host != "" && dlg == nil && !copilot {
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
		hold, detail, checks, verr := e.checkpointVerify(ctx, res.Worktree)
		sr.Checks = checks
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
	// The session review (Decision #656): a reviewer of another family reviews
	// the branch before it is pushed; on fix the push waits for the builder. A
	// delegated receiver is reviewed by the merge gate's inspect only.
	if dlg == nil && res.Verify != "miss" {
		// The checkpoint is committed and verified: any failure from here on
		// records it first, so the run after the fix continues after it.
		keep := func(err error) error {
			if serr := e.saveBuild(ctx, c, w, repo, n, &st, &res); serr != nil {
				fmt.Fprintf(e.Err, "aayu: warning: the build state did not reach the ledger: %v\n", serr)
			}
			return err
		}
		head, err := run2(ctx, res.Worktree, "git", "rev-parse", "HEAD")
		if err != nil {
			return keep(Wrap(ExitExternal, err))
		}
		pr, err := storyPR(ctx, c, repo, res.Branch)
		if err != nil {
			return keep(err)
		}
		if st.ReviewPR != 0 && pr.GetNumber() != st.ReviewPR {
			// The PR the spend belongs to is closed: a new PR has its own cap.
			st.ReviewUSD, st.ReviewPR, sr.State.SpentUSD = 0, 0, 0
		}
		var ciSpent float64
		if pr != nil {
			owner, name, _ := github.SplitRepo(repo)
			comments, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.IssueComment, *gh.Response, error) {
				return c.REST.Issues.ListComments(ctx, owner, name, pr.GetNumber(), &gh.IssueListCommentsOptions{ListOptions: o})
			})
			if err != nil {
				return keep(Wrap(ExitExternal, err))
			}
			ciSpent = inspectState(comments, viewer(ctx, c), l.Config.Identity).SpentUSD
		}
		wait, rep, err := e.reviewSession(ctx, &sr, sessionReviewIn{cfg: &l.Config, w: w, repo: repo, n: n, chk: chk, body: is.GetBody(), pr: pr, ciSpent: ciSpent, wt: res.Worktree, dev: dev, head: head, st: st})
		spent := sr.State.SpentUSD > st.ReviewUSD
		st.ReviewUSD = max(st.ReviewUSD, sr.State.SpentUSD)
		if pr != nil {
			st.ReviewPR = pr.GetNumber()
		}
		if serr := sr.save(ctx, res.Worktree); serr != nil && err == nil {
			err = Wrap(ExitInternal, serr)
		}
		if err != nil {
			return keep(err)
		}
		if rep.Round > 0 || rep.Skipped != "" {
			res.Review = &rep
		}
		if wait {
			if err := e.saveBuild(ctx, c, w, repo, n, &st, &res); err != nil {
				return err
			}
			// The held checkpoint goes to a ref of its own, never the PR
			// branch, so --resume elsewhere continues from it.
			if !c.DryRun() {
				gw := &ledger.Writer{Dir: root, Token: bot.Token.Value}
				if _, err := c.Write(ctx, actions.Action{Kind: "git.push", Target: repo + "@" + heldRef(res.Branch), Payload: map[string]any{"ref": heldRef(res.Branch), "commit": head}},
					func(ctx context.Context) (any, error) {
						return nil, gw.Exec(ctx, res.Worktree, "push", "--quiet", "--force", "origin", "HEAD:"+heldRef(res.Branch))
					}); err != nil {
					return Wrap(ExitExternal, err)
				}
			}
			res.Next = fmt.Sprintf("fix each open item in %s and run `aayu build %d` again, or decline one you judge wrong with `aayu build %d --decline \"<id>: <why>\"`; round %d of at most %d before the push", res.Worktree, n, n, rep.Round, inspect.MaxRounds)
			_ = e.Emit(res, func(w io.Writer) error {
				if res.Commit != "" {
					fmt.Fprintf(w, "#%d: checkpoint %s committed (%s); verify: %s\n", n, res.Checkpoint, short(res.Commit), res.Verify)
				}
				fmt.Fprintf(w, "#%d: session review round %d by %s: %s; not pushed yet\n", n, rep.Round, rep.Model, rep.Disposition)
				if rep.Skipped != "" {
					fmt.Fprintf(w, "  this run's review did not happen (%s); round %d's items stay open\n", rep.Skipped, rep.Round)
				}
				for _, it := range rep.Open {
					at := ""
					if it.File != "" {
						at = fmt.Sprintf(" %s:%d", it.File, it.Line)
					}
					fmt.Fprintf(w, "  %s %s%s: %s\n    %s\n", it.ID, it.Severity, at, it.Title, it.Detail)
					if it.Fix != "" {
						fmt.Fprintf(w, "    fix: %s\n", it.Fix)
					}
				}
				_, err := fmt.Fprintf(w, "  next: %s\n", res.Next)
				return err
			})
			return WithNext(Errorf(ExitBlocked, "build #%d: the session review left %d item(s) to fix or decline before the push", n, len(rep.Open)), fmt.Sprintf("aayu build %d", n))
		}
		// The spend reaches the ledger before the push starts the merge gate,
		// which counts it against the same per-PR cap.
		if spent {
			if err := e.saveBuild(ctx, c, w, repo, n, &st, &res); err != nil {
				return err
			}
		}
	}
	pp := e.progress(fmt.Sprintf("Pushing %s and opening the pull request", res.Branch))
	perr := e.publishBuild(ctx, c, bot.Token.Value, root, repo, dev, n, chk, is, st.Checkpoints, &res, sr.summary())
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
	if st.ReviewUSD > 0 {
		st.ReviewPR = res.PR
	}
	// The ledger first: the pushed checkpoint is the record the next run,
	// anywhere, continues from; the worktree's own state comes second.
	if err := e.saveBuild(ctx, c, w, repo, n, &st, &res); err != nil {
		return err
	}
	sr.Pushed, sr.PushedAt, sr.Pending = len(sr.Rounds), e.Now().UTC().Truncate(time.Second), ""
	sr.PushedHead, _ = run2(ctx, res.Worktree, "git", "rev-parse", "HEAD")
	if err := sr.save(ctx, res.Worktree); err != nil {
		return Wrap(ExitInternal, err)
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
	// when the app reviews by itself only when a PR opens. Under the CI review
	// the gate reviews the head itself, so nobody is asked.
	if note := e.requestReview(ctx, res.PR); note != "" {
		res.Notes = append(res.Notes, note)
	}
	res.Next = fmt.Sprintf("the merge gate re-runs verify and inspect on PR #%d; `aayu ship %d` after a human pass", res.PR, res.PR)
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

// heldRef keeps the head a session review held back from the story branch:
// not a branch, so no workflow runs and the PR does not change.
func heldRef(branch string) string { return "refs/aayudha/held/" + branch }

// publishBuild pushes the story branch and finds or opens its PR.
func (e *Env) publishBuild(ctx context.Context, c *github.Client, token, root, repo, dev string, n int, chk ready.Result, is *gh.Issue, cps []CheckpointRecord, res *BuildResult, review string) error {
	gw := &ledger.Writer{Dir: root, Token: token}
	if !c.DryRun() {
		if _, err := c.Write(ctx, actions.Action{Kind: "git.push", Target: repo + "@" + res.Branch, Payload: map[string]any{"branch": res.Branch, "commit": res.Commit}},
			func(ctx context.Context) (any, error) {
				return nil, gw.Exec(ctx, res.Worktree, "push", "--quiet", "--force-with-lease", "origin", "HEAD:refs/heads/"+res.Branch)
			}); err != nil {
			return Wrap(ExitExternal, err)
		}
		// The branch now holds what the session review held: a held ref left
		// behind would outlive a later rewrite of the branch and send a resume
		// back to it. Removing it is best effort; a resume checks it anyway.
		if held, _ := run2(ctx, res.Worktree, "git", "ls-remote", "origin", heldRef(res.Branch)); held != "" {
			_ = gw.Exec(ctx, res.Worktree, "push", "--quiet", "origin", ":"+heldRef(res.Branch))
		}
	}
	owner, name, _ := github.SplitRepo(repo)
	open, err := storyPR(ctx, c, repo, res.Branch)
	if err != nil {
		return err
	}
	// The body declares the builder's host and models (AGENTS.md), as its
	// commits do, and carries the session review's section, replaced in
	// place; the rest of it, a person's edits and declarations included, is
	// kept.
	decl := declarationLines(ctx, res.Worktree, dev)
	// The body aayu writes from the Story and the checkpoints; a refresh
	// rewrites only its marked sections, so the RAID rows (written once, at
	// creation) need no lookup then.
	head, _ := run2(ctx, res.Worktree, "git", "log", "-1", "--format=%B")
	story := func(deps []prDependency) string {
		body := storyBody(chk, is.GetBody(), storyRisk(is), deps, cps, builtBy(head))
		if d := res.Delegation; d != nil {
			// The receiver's report goes in the technical detail, quoted.
			note := fmt.Sprintf("%s (%s, `aayu build --delegate`) built this from the delegation brief. It has no merge rights.\n", d.Receiver, d.Mode)
			if d.Proved != "" {
				note += fmt.Sprintf("\n%s's own check, quoted as it reported it. The merge gate's proof decides.\n\n%s\n", d.Receiver, quoteLines(d.Proved))
			}
			body = strings.Replace(body, "\n</details>", "\n"+note+"\n</details>", 1)
		}
		return body
	}
	update := func(body string) string {
		body = refreshSections(body, story(nil))
		body = withDeclarations(body, decl)
		if review != "" {
			body = withSessionSummary(body, review)
		}
		return body
	}
	labelled := false
	if open != nil {
		res.PR, res.PRURL = open.GetNumber(), open.GetHTMLURL()
		labelled = slices.ContainsFunc(open.Labels, func(l *gh.Label) bool { return l.GetName() == workitems.LabelAgentAuthored })
		if body := open.GetBody(); update(body) != body {
			nb := update(body)
			if _, err := c.Write(ctx, actions.Action{Kind: "github.pull.update", Target: fmt.Sprintf("%s#%d", repo, res.PR), Story: fmt.Sprintf("#%d", n),
				Request: &actions.Request{Method: http.MethodPatch, Path: fmt.Sprintf("/repos/%s/pulls/%d", repo, res.PR)}, Payload: map[string]any{"section": "session-review"}},
				func(ctx context.Context) (any, error) {
					_, _, err := c.REST.PullRequests.Edit(ctx, owner, name, res.PR, &gh.PullRequest{Body: &nb})
					return nil, err
				}); err != nil {
				return Wrap(ExitExternal, err)
			}
		}
	} else {
		title := "feat: " + strings.ToLower(chk.Title[:1]) + chk.Title[1:]
		body := story(e.storyDependencies(ctx, c, owner, name, is.GetBody()))
		body = update(body)
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
	// An agent opened it, so the agent PR rules apply: the streak counts it,
	// and inspect holds its description to the format. An open PR gets the
	// label too, so a run that failed after creating the PR is mended.
	if res.PR != 0 && !labelled {
		labels := []string{workitems.LabelAgentAuthored}
		if _, err := c.Write(ctx, actions.Action{Kind: "github.issue.labels.add", Target: fmt.Sprintf("%s#%d", repo, res.PR),
			Request: &actions.Request{Method: http.MethodPost, Path: fmt.Sprintf("/repos/%s/issues/%d/labels", repo, res.PR)}, Payload: map[string]any{"labels": labels}},
			func(ctx context.Context) (any, error) {
				_, _, err := c.REST.Issues.AddLabelsToIssue(ctx, owner, name, res.PR, labels)
				return nil, err
			}); err != nil {
			return Wrap(ExitExternal, err)
		}
	}
	return nil
}

// storyPR is the story branch's open pull request, or nil.
func storyPR(ctx context.Context, c *github.Client, repo, branch string) (*gh.PullRequest, error) {
	owner, name, _ := github.SplitRepo(repo)
	open, _, err := c.REST.PullRequests.List(ctx, owner, name, &gh.PullRequestListOptions{State: "open", Head: owner + ":" + branch})
	if err != nil {
		return nil, Wrap(ExitExternal, err)
	}
	if len(open) == 0 {
		return nil, nil
	}
	return open[0], nil
}

// builtBy is the "Built by" line of a checkpoint commit's builder trailers:
// the models by name with the effort, and the host or the delegate. What the
// trailers do not say, it does not say.
func builtBy(msg string) string {
	var ms []string
	for _, m := range modelTrailer.FindAllStringSubmatch(msg, -1) {
		ms = append(ms, m[1])
	}
	effort := ""
	if m := effortTrailer.FindStringSubmatch(msg); m != nil {
		effort = m[1]
	}
	s := verbs.BuilderName(ms, effort)
	if m := delegateTrailer.FindStringSubmatch(msg); m != nil {
		return s + ", delegated to " + m[1]
	}
	if m := hostTrailer.FindStringSubmatch(msg); m != nil {
		return s + ", on " + m[1]
	}
	return s
}

// storyRisk is the value of the issue's risk label, or "".
func storyRisk(is *gh.Issue) string {
	for _, l := range github.IssueLabels(is) {
		if r, ok := strings.CutPrefix(l, "risk:"); ok {
			return r
		}
	}
	return ""
}

// storyDependencies are the Story's depends-on links, each with the linked
// issue's first assignee as its owner and its state. A link aayu cannot read
// keeps an unknown owner and state rather than a guess.
func (e *Env) storyDependencies(ctx context.Context, c *github.Client, owner, name, body string) []prDependency {
	_, fields := workitems.Sections(body)
	links, _ := workitems.ParseLinks(fields["Links"])
	var deps []prDependency
	for _, l := range links {
		if l.Kind != "depends-on" {
			continue
		}
		d := prDependency{N: l.Number}
		if dep, _, err := c.REST.Issues.Get(ctx, owner, name, l.Number); err == nil {
			d.Status = dep.GetState()
			if as := dep.Assignees; len(as) > 0 {
				d.Owner = "@" + as[0].GetLogin()
			}
		}
		deps = append(deps, d)
	}
	return deps
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
func (e *Env) checkpointVerify(ctx context.Context, wt string) (hold bool, detail string, checks []proof.Check, err error) {
	if e.CheckpointVerify != nil {
		hold, detail, err = e.CheckpointVerify(ctx, wt)
		return hold, detail, nil, err
	}
	sub := *e
	sub.Dir, sub.config, sub.JSON = wt, nil, true
	var out bytes.Buffer
	sub.Out, sub.Err = &out, io.Discard
	verr := runVerify(ctx, &sub, Values{})
	var res VerifyResult
	if jerr := json.NewDecoder(&out).Decode(&res); jerr != nil {
		if verr != nil {
			return false, "", nil, verr
		}
		return false, "", nil, jerr
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
	return len(miss) == 0, detail, res.Checks, nil
}

// blockBuild moves the story to status:blocked, pages, and records why.
func (e *Env) blockBuild(ctx context.Context, c *github.Client, w *ledger.Writer, repo string, n int, labels []string, st *BuildState, res *BuildResult, why string) error {
	sent, err := e.markBlocked(ctx, c, w, repo, n, labels, st, res, why)
	if err != nil {
		return err
	}
	res.Next = fmt.Sprintf("a human looks at #%d; `aayu ready %d` lets the build continue with a fresh budget", n, n)
	_ = e.Emit(res, func(w io.Writer) error {
		_, err := fmt.Fprintf(w, "#%d: status:blocked: %s (page %s)\n  next: %s\n", n, why, sent, res.Next)
		return err
	})
	return Errorf(ExitBlocked, "build #%d blocked: %s", n, why)
}

// markBlocked moves the story to status:blocked with why, pages its owner,
// and records it on the ledger and in the progress comment.
func (e *Env) markBlocked(ctx context.Context, c *github.Client, w *ledger.Writer, repo string, n int, labels []string, st *BuildState, res *BuildResult, why string) (string, error) {
	st.Blocked = why
	if err := setStatus(ctx, c, repo, n, labels, "blocked"); err != nil {
		return "", Wrap(ExitExternal, err)
	}
	sent := e.page(ctx, notify.Page{Key: fmt.Sprintf("build:%s#%d", repo, n), Subject: fmt.Sprintf("aayu build blocked #%d", n),
		Body: fmt.Sprintf("%s#%d is status:blocked: %s. A human looks at it and runs `aayu ready %d` to let the build continue.", repo, n, why, n)})
	res.Notes = append(res.Notes, "page: "+sent)
	// The block frees the story's slot: its mission's Kanha is told, in the builder's own words for a stop, on
	// one line, clipped.
	reason := clipText(strings.Join(strings.Fields(strings.TrimPrefix(why, stoppedByBuilder)), " "), 300)
	note, err := e.tellKanha(ctx, c, w, repo, n, func(f int) string { return fmt.Sprintf("blocked #%d: %s: run aayu mission next %d", n, reason, f) })
	res.Notes = kanhaNote(res.Notes, note, err)
	return sent, e.saveBuild(ctx, c, w, repo, n, st, res)
}

// failedStart undoes a start whose subthread did not start (Story #1008, R8):
// the mission's claim line, the lease, and status:in-progress go, so story n
// is status:ready again and holds no slot, and its Kanha is told why. It
// returns cause, or what it could not undo with cause.
func (e *Env) failedStart(ctx context.Context, c *github.Client, w *ledger.Writer, repo string, n int, labels []string, res *BuildResult, cause error) error {
	reason := oneLineErr(cause)
	f, _, _, err := storyMission(ctx, c, w, repo, n)
	var claims []loopClaim
	if err == nil && f != 0 {
		claims, err = e.claimLines(ctx, w, MissionClaimsPath(f), nil)
	}
	for _, cl := range claims {
		if cl.Story == n && err == nil {
			cl.Release = true
			b, _ := json.Marshal(cl)
			_, err = w.Write(ctx, ledger.Commit{Message: fmt.Sprintf("missions: the start of #%d failed; its claim is released", n), Append: map[string][]byte{MissionClaimsPath(f): append(b, '\n')}})
		}
	}
	if err == nil {
		_, err = e.releaseLease(ctx, c, repo, n, "the start of its subthread failed")
	}
	if err == nil {
		err = setStatus(ctx, c, repo, n, labels, "ready")
	}
	if err != nil {
		return Wrap(ExitExternal, fmt.Errorf("%s; and its slot was not released: %w", reason, err))
	}
	note, err := e.tellKanha(ctx, c, w, repo, n, func(int) string { return fmt.Sprintf("start of #%d failed: %s", n, reason) })
	res.Notes = kanhaNote(res.Notes, note, err)
	out := &Error{Code: CodeOf(cause), Err: fmt.Errorf("%w; #%d is status:ready again, and its claim and lease are released", cause, n)}
	if ce := (*Error)(nil); errors.As(cause, &ce) {
		out.Next = ce.Next
	}
	return out
}

// kanhaNote adds what tellKanha did to a build's notes: its note, or why it
// told nobody.
func kanhaNote(notes []string, note string, err error) []string {
	if note == "" && err != nil {
		note = "the Kanha was not told: " + oneLineErr(err)
	}
	if note == "" {
		return notes
	}
	return append(notes, note)
}

// stoppedByBuilder starts the block reason of a stop (build --stop).
const stoppedByBuilder = "stopped by its builder: "

// stopBuild is `aayu build <n> --stop <why>`: the builder gives up on a story
// it is building, so it is blocked with that reason and its owner paged, and
// no loop picks it again until a person makes it Ready. The lease stays until
// it expires.
func (e *Env) stopBuild(ctx context.Context, c *github.Client, bot identity.Bot, l *config.Loaded, repo string, n int, labels []string, why string) error {
	if !contains(labels, labelInProgress) {
		return Errorf(ExitBlocked, "build #%d --stop: it is not being built (no status:in-progress)", n)
	}
	// Only the builder stops its own build: another holder's live lease is
	// another build still at work.
	owner, name, _ := github.SplitRepo(repo)
	cur, err := readLease(ctx, c, owner, name, n)
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	if me := leaseHolder(bot.Login); cur != nil && cur.Holder != me && cur.Expires.After(e.Now()) {
		return Errorf(ExitBlocked, "build #%d --stop: %s holds a live lease on it until %s, not %s; only its builder stops it", n, cur.Holder, cur.Expires.UTC().Format(time.RFC3339), me)
	}
	if c.DryRun() {
		// blockBuild pages, and a page records itself in notify.json.
		return Errorf(ExitBlocked, "build #%d --stop --dry-run: would block it (stopped by its builder: %s) and page its owner", n, why)
	}
	w, err := e.Ledger(ctx, bot.Token, c.Identity(), bot.Tag)
	if err != nil {
		return err
	}
	var st BuildState
	if _, err := readLedgerJSON(ctx, w, BuildLedgerPath(n), &st); err != nil {
		return Wrap(ExitExternal, err)
	}
	st.Story = n
	res := BuildResult{Issue: n, Branch: st.Branch, Budget: e.buildUsage(l, repo, st, e.Now())}
	return e.blockBuild(ctx, c, w, repo, n, labels, &st, &res, stoppedByBuilder+why)
}

// tighter is the stricter of a local and a committed limit, where 0 is off:
// a local value may lower a committed limit, never raise it or turn it off.
func tighter[T int | float64](local, committed T) T {
	if committed > 0 && (local <= 0 || local > committed) {
		return committed
	}
	return local
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

// copilotModels reads the models Copilot reported in the Copilot session that
// ran this build in root, which its pre-tool hook recorded within the last
// quarter hour. ok is false when no Copilot session ran it.
func (e *Env) copilotModels(root string) (hosts.CopilotModels, bool) {
	log := hosts.CopilotSessionLog(root, hosts.CopilotHome(e.homeDir(), e.Getenv), 15*time.Minute, e.Now())
	if log == "" {
		return hosts.CopilotModels{}, false
	}
	f, err := os.Open(log)
	if err != nil {
		return hosts.CopilotModels{}, true // Copilot built it; which model is unknown
	}
	defer func() { _ = f.Close() }()
	m, _ := hosts.ReadCopilotSession(f)
	return m, true
}

// builderModel is the model the building agent runs on, when known: the
// AAYU_MODEL variable a host sets, else the host's build lane as resolved now
// ([models.build.<host>], else the default for the host's plan), so a later
// plan change cannot rewrite who built the commits. A host-managed lane names
// no model. The effort is AAYU_EFFORT, else the lane's when the lane named the
// model: only a source that knows it says it.
func (e *Env) builderModel() (model, effort string) {
	effort = e.Getenv("AAYU_EFFORT")
	if m := e.Getenv("AAYU_MODEL"); m != "" {
		return m, effort
	}
	if l, err := e.Config(); err == nil && e.Host != "" {
		r, err := (models.Resolver{Models: l.Config.Models, Hosts: l.Config.Hosts}).Resolve(verbs.LaneBuild, verbs.HostID(e.Host))
		if err == nil && r.Family != verbs.FamilyHostManaged {
			if effort == "" {
				effort = r.Effort
			}
			return r.Model, effort
		}
	}
	return "", effort
}

// ampBuilder is what Amp says of the thread running this build
// (AMP_THREAD_ID): its Agent models and its agent mode, each empty when Amp
// does not say. The amp CLI gets a short timeout, so a slow one never holds a
// checkpoint.
func (e *Env) ampBuilder(ctx context.Context, dir string) (ms []string, mode string) {
	id := e.Getenv("AMP_THREAD_ID")
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	x := e.exec()
	if out, err := x(ctx, dir, "amp", "threads", "usage", id, "--details"); err == nil {
		ms = hosts.AmpAgentModels(out)
	}
	if out, err := x(ctx, dir, "amp", "threads", "markdown", id); err == nil {
		mode = hosts.AmpAgentMode(out)
	}
	return ms, mode
}

// modelTrailers are a checkpoint's model trailers: one Aayudha-Model line per
// model. Every reader takes a trailer's value as one token, so a value that
// is not one is left out.
func modelTrailers(models []string) string {
	var b strings.Builder
	for _, m := range models {
		if len(strings.Fields(m)) == 1 {
			b.WriteString("\nAayudha-Model: " + m)
		}
	}
	return b.String()
}

// oneFamily is the model a build record names for models: the first, when
// every one is of its family; else none, so a single family never hides the
// others.
func oneFamily(models []string) string {
	if len(models) > 0 && !slices.ContainsFunc(models, func(m string) bool { return verbs.FamilyOf(m) != verbs.FamilyOf(models[0]) }) {
		return models[0]
	}
	return ""
}
