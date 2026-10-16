package cli

import (
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
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/delegate"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/identity"
	"github.com/rahulnakmol/agentic-ayudha/internal/inspect"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/notify"
	"github.com/rahulnakmol/agentic-ayudha/internal/ready"
)

// Delegation is the part of a build result a delegated story adds.
type Delegation struct {
	Receiver string         `json:"receiver"`
	Mode     string         `json:"mode"` // cloud or local
	Reason   string         `json:"reason,omitempty"`
	Status   string         `json:"status"`
	Task     string         `json:"task,omitempty"`
	TaskURL  string         `json:"task_url,omitempty"`
	PR       int            `json:"pr,omitempty"`
	Proved   string         `json:"proved,omitempty"` // the receiver's own "What I proved" (a preview; the merge gate's proof decides)
	FollowUp bool           `json:"follow_up_sent,omitempty"`
	State    delegate.State `json:"-"`
}

// delegation is a delegated build in progress.
type delegation struct {
	receiver string
	local    bool
	fresh    bool // this run hands off: a new branch and, locally, a new worktree
	handoff  int  // which hand-off of the story this is (from 1)
	reason   string
	adapter  delegate.Adapter
	state    delegate.State
	brief    string
	bin      string // the local CLI
	c        *github.Client
	w        *ledger.Writer
	repo     string
	story    int
	title    string
	branch   string
	dev      string
	root     string
	body     string // the story's body (untrusted; quoted in the brief as data)
	bot      identity.Bot
}

// Delegation statuses (delegate.State.Status).
const (
	dlgHandedOff = "handed-off"
	dlgWorking   = "working"
	dlgPROpen    = "pr-open"
	dlgFix       = "fix-requested"
	dlgReady     = "merge-ready"
	dlgCapped    = "capped"
	dlgFailed    = "failed"
	dlgMerged    = "merged"
)

const delegateMarker = "<!-- aayudha:delegate -->"

func (e *Env) delegateExec() delegate.Exec {
	if e.DelegateExec != nil {
		return e.DelegateExec
	}
	return func(ctx context.Context, dir string, env []string, stdin, name string, args ...string) (string, error) {
		cmd := exec.CommandContext(ctx, name, args...)
		cmd.Dir, cmd.Env = dir, env
		if stdin != "" {
			cmd.Stdin = strings.NewReader(stdin)
		}
		out, err := cmd.CombinedOutput()
		if err != nil {
			return string(out), fmt.Errorf("%s: %w: %s", name, err, clipText(actions.RedactString(string(out)), 400))
		}
		return string(out), nil
	}
}

func clipText(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}

// ghOps gives the Claude adapter the branches on origin.
type ghOps struct {
	e    *Env
	c    *github.Client
	repo string
	root string
}

func (o ghOps) Branches(ctx context.Context, prefix string) ([]string, error) {
	out, err := run2(ctx, o.root, "git", "ls-remote", "--heads", "origin", "refs/heads/"+prefix+"*")
	if err != nil {
		return nil, err
	}
	var bs []string
	for _, l := range strings.Split(out, "\n") {
		if _, ref, ok := strings.Cut(l, "\t"); ok {
			bs = append(bs, strings.TrimPrefix(ref, "refs/heads/"))
		}
	}
	slices.Sort(bs)
	return bs, nil
}

// connected reports which receivers' cloud agents are set up here, without
// any network call: a Cursor API key; Codex's CLI with a cloud environment;
// Claude Code's CLI (its cloud sessions need the user's claude.ai sign-in);
// Amp's CLI with its key.
func (e *Env) connected(root string) map[string]bool {
	l, _ := e.Config()
	out := map[string]bool{}
	if l == nil {
		return out
	}
	cfg := l.Config.Delegate
	if v, _, err := e.SecretStore().Get(orDefault(cfg.CursorSecret, "cursor-api-key")); err == nil && v != "" {
		out[delegate.Cursor] = true
	}
	if _, err := e.lookPath()("codex"); err == nil && cfg.CodexEnv != "" {
		out[delegate.Codex] = true
	}
	if _, err := e.lookPath()("amp"); err == nil && (e.Getenv("AMP_API_KEY") != "") {
		out[delegate.Amp] = true
	}
	if _, err := e.lookPath()("claude"); err == nil {
		out[delegate.Claude] = true
	}
	return out
}

// receiverHelp says plainly what is missing for receiver r ("" for auto) and
// offers what is available instead: the same receiver with --local when its
// CLI is installed (or in the cloud when the user asked for --local), and
// every other connected receiver. aayu never installs or signs in for the
// user.
func (e *Env) receiverHelp(_ context.Context, _ string, r string, cloud, local map[string]bool, wantLocal bool) string {
	var b strings.Builder
	what := func(x string) string {
		switch {
		case cloud[x] && local[x]:
			return "cloud agent connected, CLI installed"
		case cloud[x]:
			return "cloud agent connected, CLI not installed"
		case local[x]:
			return "cloud agent not connected, CLI installed"
		}
		return "cloud agent not connected, CLI not installed"
	}
	if r != "" {
		fmt.Fprintf(&b, "%s here: %s.\n", delegate.Names[r], what(r))
		switch {
		case !wantLocal && local[r]:
			fmt.Fprintf(&b, "- Its CLI is installed: add `--local` to run it on this machine in a fresh aayu worktree.\n")
		case wantLocal && cloud[r]:
			fmt.Fprintf(&b, "- Its cloud agent is connected: drop `--local` to hand off in the cloud.\n")
		}
	}
	for _, x := range delegate.Receivers {
		if x == r {
			continue
		}
		switch {
		case !wantLocal && cloud[x]:
			fmt.Fprintf(&b, "- %s's cloud agent is connected: `--delegate %s`.\n", delegate.Names[x], x)
		case wantLocal && local[x]:
			fmt.Fprintf(&b, "- %s's CLI is installed: `--delegate %s --local`.\n", delegate.Names[x], x)
		}
	}
	b.WriteString("To connect a receiver, see docs/research/agent-delegation-and-review.md §10; aayu never installs or signs in for you.")
	return b.String()
}

// newDelegation resolves the receiver (auto picks one), the mode (cloud
// unless --local or the cloud agent is not connected), and the adapter, and
// reads the story's delegation state from the ledger.
func (e *Env) newDelegation(ctx context.Context, in Values, c *github.Client, bot identity.Bot, w *ledger.Writer, repo, root string, n int, chk ready.Result, body string, labels []string) (*delegation, error) {
	l, err := e.Config()
	if err != nil {
		return nil, err
	}
	if r := e.Getenv("AAYU_DELEGATED"); r != "" {
		return nil, Errorf(ExitBlocked, "build #%d --delegate: this run is %s working on a delegated story, and a receiver never hands work on to another agent", n, r)
	}
	cfg := l.Config.Delegate
	d := &delegation{receiver: in.String("delegate"), local: in.Bool("local"), c: c, w: w, repo: repo, story: n, title: chk.Title,
		dev: e.devBranch(), root: root, bot: bot, body: body}
	if _, err := readLedgerJSON(ctx, w, delegate.LedgerPath(n), &d.state); err != nil {
		return nil, Wrap(ExitExternal, err)
	}
	ongoing := d.state.Receiver != "" && d.state.Status != dlgFailed && d.state.Status != dlgMerged
	if d.receiver == "auto" && ongoing {
		d.receiver, d.local, d.reason = d.state.Receiver, d.state.Mode == "local", d.state.Reason
	}
	if ongoing && d.receiver == d.state.Receiver && d.state.Mode == "local" {
		d.local = true // a sync keeps its hand-off's mode
	}
	cloud := e.connected(root)
	local := map[string]bool{}
	for _, r := range delegate.Receivers {
		_, local[r] = delegate.LookLocal(r, e.lookPath())
	}
	if d.receiver == "auto" {
		effort := ""
		for _, lb := range labels {
			if v, ok := strings.CutPrefix(lb, "effort:"); ok {
				effort = v
			}
		}
		// Cloud by default: auto picks among cloud agents, and among local CLIs
		// only with --local.
		pickCloud, pickLocal := cloud, map[string]bool{}
		if d.local {
			pickCloud, pickLocal = map[string]bool{}, local
		}
		ch, err := delegate.Choose(effort, pickCloud, pickLocal)
		if err != nil {
			return nil, Errorf(ExitBlocked, "build #%d --delegate auto: %v\n%s", n, err, e.receiverHelp(ctx, root, "", cloud, local, d.local))
		}
		d.receiver, d.local, d.reason = ch.Receiver, ch.Local, ch.Reason
	}
	// Cloud by default; the local CLI only on the user's explicit --local. A
	// receiver that is not available is named plainly, with what to do.
	if !slices.Contains(delegate.Receivers, d.receiver) {
		return nil, Errorf(ExitUsage, "build --delegate %q: want one of %s or auto", d.receiver, strings.Join(delegate.Receivers, ", "))
	}
	if !d.local && !cloud[d.receiver] {
		return nil, Errorf(ExitBlocked, "build #%d --delegate %s: %s's cloud agent is not connected here.\n%s", n, d.receiver, delegate.Names[d.receiver], e.receiverHelp(ctx, root, d.receiver, cloud, local, false))
	}
	if d.local {
		bin, ok := delegate.LookLocal(d.receiver, e.lookPath())
		if !ok {
			return nil, Errorf(ExitBlocked, "build #%d --delegate %s --local: %s's CLI (%s) is not installed here.\n%s", n, d.receiver, delegate.Names[d.receiver], strings.Join(delegate.LocalBins[d.receiver], " or "), e.receiverHelp(ctx, root, d.receiver, cloud, local, true))
		}
		d.bin = bin
	}
	ex := e.delegateExec()
	switch d.receiver {
	case delegate.Cursor:
		key, _, _ := e.SecretStore().Get(orDefault(cfg.CursorSecret, "cursor-api-key"))
		d.adapter = delegate.CursorAPI{BaseURL: orDefault(cfg.CursorAPI, "https://api.cursor.com"), Key: key, Model: cfg.CursorModel, HTTP: e.HTTP}
	case delegate.Amp:
		d.adapter = delegate.AmpCLI{Exec: ex, OrbSize: cfg.AmpOrbSize, Env: delegate.ScrubEnv(e.Environ, delegate.Amp, os.TempDir())}
	case delegate.Claude:
		d.adapter = delegate.ClaudeCloud{Exec: ex, Dir: root, Env: delegate.ScrubEnv(e.Environ, delegate.Claude, os.TempDir()), GitHub: ghOps{e, c, repo, root}}
	case delegate.Codex:
		d.adapter = delegate.CodexCloud{Exec: ex, EnvID: cfg.CodexEnv, Env: delegate.ScrubEnv(e.Environ, delegate.Codex, os.TempDir())}
	default:
		return nil, Errorf(ExitUsage, "build --delegate %q: want one of %s or auto", d.receiver, strings.Join(delegate.Receivers, ", "))
	}
	// Every hand-off is fresh: a new branch and, locally, a new worktree; a
	// sync keeps the hand-off's own.
	d.fresh = d.state.Receiver == "" || d.state.Receiver != d.receiver || d.state.Status == dlgFailed || d.state.Status == dlgMerged
	d.handoff = max(d.state.Handoffs, 1)
	if d.state.Receiver == "" {
		d.handoff = 0
	}
	d.branch = d.state.Branch
	if d.fresh {
		d.handoff++
		for d.local && e.taken(ctx, root, delegate.Worktree(root, n, d.receiver, d.handoff), delegate.Branch(n, slug(chk.Title), d.handoff)) {
			d.handoff++
		}
		d.branch = delegate.Branch(n, slug(chk.Title), d.handoff)
	}
	return d, nil
}

// taken reports a worktree directory or a branch on origin that an earlier
// hand-off left, so a fresh local hand-off takes the next number.
func (e *Env) taken(ctx context.Context, root, wt, branch string) bool {
	if _, err := os.Stat(wt); err == nil {
		return true
	}
	out, _ := run2(ctx, root, "git", "ls-remote", "--heads", "origin", branch)
	return strings.TrimSpace(out) != ""
}

// writeBrief renders the story's brief from the Ready snapshot on the
// ledger, the plan, AGENTS.md, and CODEOWNERS.
func (e *Env) writeBrief(ctx context.Context, d *delegation, chk ready.Result) error {
	in := delegate.BriefInput{Repo: d.repo, Base: d.dev, Story: d.story, Title: d.title, Branch: d.branch, CycleCap: inspect.MaxCycles, Local: d.local}
	if l, err := e.Config(); err == nil {
		in.Checks = l.Config.Delegate.Checks
	}
	t, err := d.w.Read(ctx)
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	defer t.Close()
	b, err := t.File(ctx, ready.LedgerPath(d.story))
	if err != nil {
		return Errorf(ExitExternal, "build #%d --delegate: no Ready snapshot on the ledger: %v", d.story, err)
	}
	snap, err := ready.ParseSnapshot(string(b))
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	in.Criteria = snap.Criteria
	in.Body = d.body
	var plan PlanRecord
	if ok, _ := readLedgerJSON(ctx, d.w, PlanLedgerPath(d.story), &plan); ok {
		in.Checkpoints = plan.Checkpoints
	}
	if a, err := os.ReadFile(filepath.Join(d.root, "AGENTS.md")); err == nil {
		in.Conventions = string(a)
	}
	if co, err := os.ReadFile(filepath.Join(d.root, ".github", "CODEOWNERS")); err == nil {
		for _, l := range strings.Split(string(co), "\n") {
			if f := strings.Fields(l); len(f) > 1 && !strings.HasPrefix(f[0], "#") {
				in.Protected = append(in.Protected, "`"+strings.TrimPrefix(f[0], "/")+"`")
			}
		}
	}
	if len(in.Checks) == 0 {
		in.Checks = []string{"the checks AGENTS.md lists before every commit"}
	}
	d.brief = delegate.Brief(in)
	return nil
}

// act records one hand-off step in the actions log (a receiver's API or CLI
// is an external write like any other, rule 6).
func (e *Env) act(ctx context.Context, d *delegation, kind string, payload map[string]any, fn func(ctx context.Context) error) error {
	log, err := e.Actions()
	if err != nil {
		return err
	}
	payload["receiver"], payload["mode"] = d.receiver, mode(d.local)
	_, err = log.Do(ctx, actions.Action{Kind: kind, Target: d.repo + "#" + strconv.Itoa(d.story) + "@" + d.receiver, Story: fmt.Sprintf("#%d", d.story),
		Seat: "ai-native-engineer", Host: e.Host, Identity: d.c.Identity(), Payload: payload},
		func(ctx context.Context) (actions.Outcome, error) { return actions.Outcome{Attempts: 1}, fn(ctx) })
	return err
}

func mode(local bool) string {
	if local {
		return "local"
	}
	return "cloud"
}

func (d *delegation) event(now time.Time, status, detail string) {
	if d.state.Status == status && detail == "" {
		return
	}
	d.state.Status = status
	d.state.Events = append(d.state.Events, delegate.Event{At: now.UTC().Format(time.RFC3339), Status: status, Detail: detail})
}

// saveDelegation writes delegate/<n>/state.json and the story's
// replace-in-place delegation comment.
func (e *Env) saveDelegation(ctx context.Context, d *delegation) error {
	if d.c.DryRun() {
		return nil
	}
	b, _ := json.MarshalIndent(d.state, "", "  ")
	if _, err := d.w.Write(ctx, ledger.Commit{Message: fmt.Sprintf("delegate: state of #%d (%s)", d.story, d.state.Status), Files: map[string][]byte{delegate.LedgerPath(d.story): append(b, '\n')}}); err != nil {
		return Wrap(ExitExternal, err)
	}
	var body strings.Builder
	fmt.Fprintf(&body, "%s\n**Delegated to %s** (%s, `aayu build --delegate`): %s", delegateMarker, d.state.Receiver, d.state.Mode, d.state.Status)
	if d.state.TaskURL != "" {
		fmt.Fprintf(&body, " · [task](%s)", d.state.TaskURL)
	}
	if d.state.PR > 0 {
		fmt.Fprintf(&body, " · PR #%d", d.state.PR)
	}
	if d.state.Reason != "" {
		fmt.Fprintf(&body, "\n\nWhy %s: %s.", d.state.Receiver, d.state.Reason)
	}
	body.WriteString("\n\n| When | Status | Detail |\n|---|---|---|\n")
	for _, ev := range d.state.Events {
		fmt.Fprintf(&body, "| %s | %s | %s |\n", ev.At, ev.Status, strings.ReplaceAll(ev.Detail, "|", `\|`))
	}
	fmt.Fprintf(&body, "\nBrief SHA-256 `%s`. The receiver has no merge rights: the merge gate and inspect review its PR, and a human ships it with `aayu ship`.", short(d.state.BriefHash))
	return e.replaceComment(ctx, d.c, d.repo, d.story, delegateMarker, body.String())
}

// delegateStep runs one step of a delegated build. A first run hands off (the
// cloud agent gets the brief; a local run returns the writer that runs the
// receiver's CLI in the worktree). A later run syncs: the receiver's status,
// the PR (found by Refs #n, or opened by aayu from the receiver's branch or
// diff), inspect's disposition on its head, and a follow-up on fix within the
// cycle cap. It returns write when the change must be written into an aayu
// worktree now (build's checkpoint, verify, push, and PR follow), and done
// when this run has nothing more for build to do.
func (e *Env) delegateStep(ctx context.Context, d *delegation, chk ready.Result, res *BuildResult) (write func(context.Context, string) error, done bool, err error) {
	now := e.Now()
	res.Delegation = &Delegation{Receiver: d.receiver, Mode: mode(d.local), Reason: d.reason}
	report := func() { d.fill(res.Delegation) }
	if d.fresh {
		if err := e.writeBrief(ctx, d, chk); err != nil {
			return nil, true, err
		}
		d.state = delegate.State{Story: d.story, Receiver: d.receiver, Mode: mode(d.local), Model: d.model(), Family: delegate.Family(d.receiver, d.model()),
			BriefHash: delegate.BriefHash(d.brief), StartedAt: now.UTC().Format(time.RFC3339), Reason: d.reason, Branch: d.branch, Handoffs: d.handoff}
		if d.local {
			d.event(now, dlgWorking, "local "+filepath.Base(d.bin)+" in "+filepath.Base(delegate.Worktree(d.root, d.story, d.receiver, d.handoff)))
			report()
			return e.localWriter(d, d.brief), false, nil
		}
		var task delegate.Task
		err := e.act(ctx, d, "delegate.handoff", map[string]any{"brief_sha256": d.state.BriefHash}, func(ctx context.Context) error {
			var err error
			task, err = d.adapter.Handoff(ctx, delegate.Handoff{Repo: d.repo, Base: d.dev, Story: d.story, Title: d.title, Brief: d.brief, Branch: d.branch})
			return err
		})
		if err != nil {
			d.event(now, dlgFailed, "hand-off failed: "+clipText(err.Error(), 200))
			_ = e.saveDelegation(ctx, d)
			return nil, true, Errorf(ExitExternal, "build #%d --delegate %s: %v", d.story, d.receiver, err)
		}
		d.state.Task, d.state.TaskURL = task.ID, task.URL
		d.event(now, dlgHandedOff, "brief sent")
		report()
		res.Next = fmt.Sprintf("%s works on #%d; `aayu build %d --delegate %s` again to sync: it finds the PR (Refs #%d) and forwards inspect's findings", d.receiver, d.story, d.story, d.receiver, d.story)
		return nil, true, e.saveDelegation(ctx, d)
	}

	// Sync.
	var task delegate.Task
	if !d.local {
		if task, err = d.adapter.Status(ctx, d.state); err != nil {
			return nil, true, Errorf(ExitExternal, "build #%d --delegate %s: status: %v", d.story, d.receiver, err)
		}
		if task.Status == delegate.TaskFailed && d.state.PR == 0 {
			d.event(now, dlgFailed, "the receiver's task failed: "+task.Detail)
			report()
			_ = e.saveDelegation(ctx, d)
			return nil, true, Errorf(ExitBlocked, "build #%d: %s's task failed (%s); `aayu build %d --delegate <receiver>` hands it off again", d.story, d.receiver, task.Detail, d.story)
		}
	}
	if task.Diff != "" && d.state.PR != 0 && d.state.Applied != task.ID {
		// Codex finished a follow-up task: aayu applies its diff on the PR's branch.
		d.state.Applied = task.ID
		d.event(now, dlgWorking, "applying the follow-up task's diff in an aayu worktree")
		report()
		return applyDiff(task.Diff), false, nil
	}
	if d.state.PR == 0 {
		pr, err := e.findDelegatedPR(ctx, d, task)
		if err != nil {
			return nil, true, err
		}
		switch {
		case pr > 0:
			d.state.PR = pr
			d.event(now, dlgPROpen, fmt.Sprintf("PR #%d", pr))
		case task.Diff != "": // Codex: aayu applies the task's diff and opens the PR
			d.state.Applied = task.ID
			d.event(now, dlgWorking, "applying the task's diff in an aayu worktree")
			report()
			return applyDiff(task.Diff), false, nil
		case d.local:
			report()
			res.Next = fmt.Sprintf("the local %s run left no PR; run `aayu build %d --delegate %s --local` again", d.receiver, d.story, d.receiver)
			return e.localWriter(d, d.brief), false, nil
		default:
			d.event(now, dlgWorking, task.Detail)
			report()
			res.Next = fmt.Sprintf("%s is still working on #%d (no PR with Refs #%d yet); sync again later", d.receiver, d.story, d.story)
			return nil, true, e.saveDelegation(ctx, d)
		}
	}
	res.PR = d.state.PR
	return e.reviewLoop(ctx, d, res)
}

func (d *delegation) model() string {
	if d.receiver == delegate.Cursor {
		if a, ok := d.adapter.(delegate.CursorAPI); ok {
			return a.Model
		}
	}
	return ""
}

func (d *delegation) fill(r *Delegation) {
	r.Status, r.Task, r.TaskURL, r.PR, r.Proved, r.State = d.state.Status, d.state.Task, d.state.TaskURL, d.state.PR, d.state.Proved, d.state
}

// findDelegatedPR finds the receiver's PR: the receiver's own PR URL, an open
// PR whose body or branch names the story, or, for a receiver that pushed a
// branch without a PR (claude-code-action), a PR aayu opens from it.
func (e *Env) findDelegatedPR(ctx context.Context, d *delegation, task delegate.Task) (int, error) {
	if m := regexp.MustCompile(`/pull/(\d+)`).FindStringSubmatch(task.PRURL); m != nil {
		return strconv.Atoi(m[1])
	}
	owner, name, _ := github.SplitRepo(d.repo)
	prs, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.PullRequest, *gh.Response, error) {
		return d.c.REST.PullRequests.List(ctx, owner, name, &gh.PullRequestListOptions{State: "open", ListOptions: o})
	})
	if err != nil {
		return 0, Wrap(ExitExternal, err)
	}
	for _, pr := range prs {
		if StoryOf(pr.GetBody(), "", pr.GetHead().GetRef()) == d.story || (task.Branch != "" && pr.GetHead().GetRef() == task.Branch) {
			return pr.GetNumber(), nil
		}
	}
	if task.Branch == "" {
		return 0, nil
	}
	title := "feat: " + strings.ToLower(d.title[:1]) + d.title[1:]
	body := fmt.Sprintf("Refs #%d\n\nBuilt by %s (`aayu build --delegate %s`) from the delegation brief (SHA-256 `%s`); aayu opened this PR from the branch it pushed. The merge gate and inspect review it; a human ships it with `aayu ship`.\n", d.story, d.receiver, d.receiver, short(d.state.BriefHash))
	var pr *gh.PullRequest
	if _, err := d.c.Write(ctx, actions.Action{Kind: "github.pull.create", Target: d.repo, Story: fmt.Sprintf("#%d", d.story),
		Request: &actions.Request{Method: http.MethodPost, Path: "/repos/" + d.repo + "/pulls"}, Payload: map[string]any{"head": task.Branch, "base": d.dev, "title": title, "delegate": d.receiver}},
		func(ctx context.Context) (any, error) {
			var err error
			pr, _, err = d.c.REST.PullRequests.Create(ctx, owner, name, gh.CreatePullRequest{Title: &title, Head: task.Branch, Base: d.dev, Body: &body})
			return map[string]any{"number": pr.GetNumber()}, err
		}); err != nil {
		return 0, Wrap(ExitExternal, err)
	}
	return pr.GetNumber(), nil
}

// reviewLoop reads inspect's verdict on the PR's head and acts on it: merge
// is ready for a human's ship; fix within the cap sends the findings once per
// head (local: the CLI addresses them in the worktree); the cap or a rebuild
// pages a human.
func (e *Env) reviewLoop(ctx context.Context, d *delegation, res *BuildResult) (func(context.Context, string) error, bool, error) {
	now := e.Now()
	report := func() { d.fill(res.Delegation) }
	owner, name, _ := github.SplitRepo(d.repo)
	pr, _, err := d.c.REST.PullRequests.Get(ctx, owner, name, d.state.PR)
	if err != nil {
		return nil, true, Wrap(ExitExternal, err)
	}
	res.PRURL = pr.GetHTMLURL()
	if p := delegate.Proved(pr.GetBody()); p != "" {
		d.state.Proved = p
	}
	head := pr.GetHead().GetSHA()
	if d.state.Branch == "" || d.state.Branch != pr.GetHead().GetRef() {
		d.state.Branch = pr.GetHead().GetRef()
	}
	switch {
	case pr.GetMerged():
		d.event(now, dlgMerged, "merged")
		report()
		res.Next = "merged"
		return nil, true, e.saveDelegation(ctx, d)
	case pr.GetState() == "closed":
		d.event(now, dlgFailed, fmt.Sprintf("PR #%d was closed unmerged", d.state.PR))
		report()
		_ = e.saveDelegation(ctx, d)
		return nil, true, Errorf(ExitBlocked, "build #%d: PR #%d was closed unmerged", d.story, d.state.PR)
	}
	comments, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.IssueComment, *gh.Response, error) {
		return d.c.REST.Issues.ListComments(ctx, owner, name, d.state.PR, &gh.IssueListCommentsOptions{ListOptions: o})
	})
	if err != nil {
		return nil, true, Wrap(ExitExternal, err)
	}
	var st inspect.State
	for _, cm := range comments {
		if strings.Contains(cm.GetBody(), inspect.Marker) {
			st = inspect.ParseState(cm.GetBody())
		}
	}
	r := st.Result
	page := func(why string) {
		sent := e.page(ctx, notify.Page{Key: fmt.Sprintf("delegate:%s#%d", d.repo, d.story), Subject: fmt.Sprintf("aayu: delegated #%d needs a human", d.story),
			Body: fmt.Sprintf("%s#%d (delegated to %s, PR #%d): %s.", d.repo, d.story, d.receiver, d.state.PR, why)})
		res.Notes = append(res.Notes, "page: "+sent)
	}
	switch {
	case (r == nil || r.Head != head) && st.Cycles >= inspect.MaxCycles:
		// inspect stops at its cap and will not review this head: a human takes over.
		d.event(now, dlgCapped, fmt.Sprintf("%d fix cycles; inspect will not review %s", st.Cycles, short(head)))
		page(fmt.Sprintf("%d fix cycles reached the cap", st.Cycles))
		report()
		_ = e.saveDelegation(ctx, d)
		return nil, true, Errorf(ExitBlocked, "build #%d: PR #%d reached the %d-cycle cap; a human takes over (paged)", d.story, d.state.PR, inspect.MaxCycles)
	case r == nil || r.Head != head:
		d.event(now, dlgPROpen, "")
		report()
		res.Next = fmt.Sprintf("waiting for the merge gate's inspect on PR #%d at %s", d.state.PR, short(head))
		return nil, true, e.saveDelegation(ctx, d)
	case r.Disposition == inspect.Merge:
		d.event(now, dlgReady, fmt.Sprintf("inspect merge on %s", short(head)))
		report()
		res.Next = fmt.Sprintf("PR #%d is ready for a human pass: `aayu ship %d`", d.state.PR, d.state.PR)
		return nil, true, e.saveDelegation(ctx, d)
	case r.Disposition == inspect.Rebuild:
		d.event(now, dlgCapped, "inspect asked for a rebuild: the story or plan needs rework")
		page("inspect asked for a rebuild")
		report()
		_ = e.saveDelegation(ctx, d)
		return nil, true, Errorf(ExitBlocked, "build #%d: inspect asked for a rebuild of PR #%d; a human reviews the story and plan", d.story, d.state.PR)
	case slices.ContainsFunc(d.state.FollowUps, func(f delegate.FollowUp) bool { return f.Head == head }):
		report()
		res.Next = fmt.Sprintf("the findings on %s were sent to %s; waiting for its new commits on PR #%d", short(head), d.receiver, d.state.PR)
		return nil, true, e.saveDelegation(ctx, d)
	}
	var fs []delegate.Finding
	var ids []string
	for _, f := range r.Findings {
		fs = append(fs, delegate.Finding{ID: f.ID, Severity: f.Severity, Category: f.Category, File: f.File, Line: f.Line, Title: f.Title, Detail: f.Detail, Fix: f.Fix})
		ids = append(ids, f.ID)
	}
	msg := delegate.FollowUpMessage(d.story, d.state.PR, st.Cycles, inspect.MaxCycles, r.Disposition, fs)
	fu := delegate.FollowUp{At: now.UTC().Format(time.RFC3339), Head: head, Cycle: st.Cycles, Findings: ids}
	if ids == nil {
		fu.Findings = []string{}
	}
	d.state.FollowUps = append(d.state.FollowUps, fu)
	d.event(now, dlgFix, fmt.Sprintf("cycle %d: %d finding(s) on %s", st.Cycles, len(ids), short(head)))
	res.Delegation.FollowUp = true
	report()
	if d.local {
		return e.localWriter(d, msg), false, nil
	}
	err = e.act(ctx, d, "delegate.follow-up", map[string]any{"pr": d.state.PR, "head": head, "cycle": st.Cycles, "findings": ids}, func(ctx context.Context) error {
		t, err := d.adapter.FollowUp(ctx, d.state, msg)
		if err == nil && t.ID != "" {
			d.state.Task = t.ID
		}
		return err
	})
	if err != nil {
		return nil, true, Errorf(ExitExternal, "build #%d: follow-up to %s: %v", d.story, d.receiver, err)
	}
	res.Next = fmt.Sprintf("%d finding(s) sent to %s; sync again after its new commits", len(ids), d.receiver)
	return nil, true, e.saveDelegation(ctx, d)
}

// localWriter runs the receiver's CLI on prompt in the worktree, with a
// scrubbed environment and no GitHub credential, under a time limit.
func (e *Env) localWriter(d *delegation, prompt string) func(context.Context, string) error {
	return func(ctx context.Context, wt string) error {
		gh, err := delegate.EmptyDir()
		if err != nil {
			return err
		}
		defer func() { _ = os.RemoveAll(gh) }()
		limit := 60 * time.Minute
		if l, err := e.Config(); err == nil && l.Config.Delegate.LocalTimeoutMinutes > 0 {
			limit = time.Duration(l.Config.Delegate.LocalTimeoutMinutes) * time.Minute
		}
		ctx, cancel := context.WithTimeout(ctx, limit)
		defer cancel()
		env := delegate.ScrubEnv(e.Environ, d.receiver, gh)
		return e.act(ctx, d, "delegate.local-run", map[string]any{"bin": filepath.Base(d.bin), "worktree": wt, "prompt_sha256": delegate.BriefHash(prompt)}, func(ctx context.Context) error {
			out, err := e.delegateExec()(ctx, wt, env, "", d.bin, delegate.LocalArgs(d.receiver, d.bin, wt, prompt)...)
			if p := delegate.Proved(out); p != "" {
				d.state.Proved = p
			}
			return err
		})
	}
}

// applyDiff applies a receiver's returned diff in the worktree.
func applyDiff(diff string) func(context.Context, string) error {
	return func(ctx context.Context, wt string) error {
		f, err := os.CreateTemp("", "aayu-delegate-*.diff")
		if err != nil {
			return err
		}
		defer func() { _ = os.Remove(f.Name()) }()
		if _, err := f.WriteString(diff); err != nil {
			return err
		}
		_ = f.Close()
		if _, err := run2(ctx, wt, "git", "apply", "--whitespace=nowarn", f.Name()); err != nil {
			return errors.New("the receiver's diff does not apply: " + err.Error())
		}
		return nil
	}
}

// quoteLines quotes text as a Markdown block quote, so a receiver's report
// cannot add headings or other structure to a PR body.
func quoteLines(text string) string {
	return "> " + strings.ReplaceAll(strings.TrimSpace(text), "\n", "\n> ")
}

// emitDelegation reports a delegation step.
func (e *Env) emitDelegation(res BuildResult) error {
	return e.Emit(res, func(w io.Writer) error {
		d := res.Delegation
		if d == nil {
			return nil
		}
		fmt.Fprintf(w, "#%d delegated to %s (%s): %s", res.Issue, d.Receiver, d.Mode, d.Status)
		if d.TaskURL != "" {
			fmt.Fprintf(w, " · %s", d.TaskURL)
		}
		if d.PR > 0 {
			fmt.Fprintf(w, " · PR #%d", d.PR)
		}
		fmt.Fprintln(w)
		if d.Reason != "" {
			fmt.Fprintf(w, "  why: %s\n", d.Reason)
		}
		if d.Proved != "" {
			fmt.Fprintf(w, "  %s says it proved (a preview; the merge gate's proof decides):\n", d.Receiver)
			for _, l := range strings.Split(d.Proved, "\n") {
				fmt.Fprintf(w, "    %s\n", l)
			}
		}
		if d.FollowUp {
			fmt.Fprintln(w, "  follow-up sent with inspect's findings")
		}
		for _, x := range res.Notes {
			fmt.Fprintf(w, "  note: %s\n", x)
		}
		if res.Next != "" {
			fmt.Fprintf(w, "  next: %s\n", res.Next)
		}
		return nil
	})
}

// fixDelegateEnv writes .cursor/environment.json from the environment source
// (doctor --fix, WP5.5b), keeping every key but name and install, and shows
// what changed. Codex's environment lives in its UI; Amp reads .agents/setup.
func (e *Env) fixDelegateEnv() (string, error) {
	root := config.RepoRoot(e.Dir)
	if root == "" {
		return "", nil
	}
	if _, err := os.Stat(filepath.Join(root, delegate.SetupPath)); err != nil {
		return "--fix: no " + delegate.SetupPath + ", so no delegation environment to generate", nil
	}
	p := filepath.Join(root, filepath.FromSlash(delegate.CursorEnvPath))
	old, _ := os.ReadFile(p)
	b, err := delegate.CursorEnvironment(old)
	if err != nil {
		return "", Wrap(ExitBlocked, fmt.Errorf("doctor --fix: %w", err))
	}
	if string(old) == string(b) {
		return "--fix: " + delegate.CursorEnvPath + " already installs with " + delegate.SetupCommand, nil
	}
	if e.DryRun {
		return "--fix (dry run): would write " + delegate.CursorEnvPath + ":\n" + string(b), nil
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(p, b, 0o644); err != nil {
		return "", err
	}
	return "--fix: wrote " + delegate.CursorEnvPath + " (install: " + delegate.SetupCommand + "); commit it to the default branch for Cursor's cloud agents:\n" + string(b), nil
}
