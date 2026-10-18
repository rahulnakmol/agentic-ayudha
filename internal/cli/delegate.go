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
	"github.com/rahulnakmol/agentic-ayudha/internal/agentenv"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/delegate"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/identity"
	"github.com/rahulnakmol/agentic-ayudha/internal/inspect"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/notify"
	"github.com/rahulnakmol/agentic-ayudha/internal/ready"
	"github.com/rahulnakmol/agentic-ayudha/internal/ui"
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
	wallEnd  time.Time // when the build's wall-time budget ends; zero without one
	lease    *Lease    // the story's lease as last renewed during a usage-limit wait
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
	dlgPaused    = "paused" // the local receiver hit its subscription usage limit
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

// agents detects the four receivers here (agentenv.Detect: PATH, version,
// each CLI's local sign-in, aayu's secret store and config, the recorded Amp
// project; no network, model, or task call), by receiver name.
func (e *Env) agents(ctx context.Context, root, repo string) map[string]agentenv.Agent {
	p := agentenv.Probe{Root: root, Repo: repo, LookPath: e.lookPath(), Exec: e.Exec, Getenv: e.Getenv,
		HasSecret: func(name string) bool { v, _, err := e.SecretStore().Get(name); return err == nil && v != "" }}
	if p.Exec == nil {
		p.Exec = agentenv.CombinedExec
	}
	if l, err := e.Config(); err == nil {
		p.CodexEnv, p.CursorSecret = l.Config.Delegate.CodexEnv, l.Config.Delegate.CursorSecret
	}
	out := map[string]agentenv.Agent{}
	for _, a := range agentenv.Detect(ctx, p) {
		out[a.Name] = a
	}
	return out
}

// runsLocally reports whether a's CLI can run here: installed, and signed in
// or holding its own key (delegate.OwnKey), which the scrubbed environment
// passes to it.
func (e *Env) runsLocally(a agentenv.Agent) bool {
	return a.CLI != "" && (a.SignedIn || delegate.OwnKey(a.Name, e.Getenv))
}

// receiverHelp says plainly what is missing for receiver r ("" for auto),
// with the exact install and sign-in commands from agentenv.Detect and what
// to connect for its cloud agent, and offers what is available instead: the
// same receiver in the other mode, and every other receiver available in the
// mode asked for. aayu never installs or signs in for the user.
func (e *Env) receiverHelp(r string, agents map[string]agentenv.Agent, wantLocal bool) string {
	var b strings.Builder
	if a, ok := agents[r]; ok {
		cli := "not installed"
		switch {
		case e.runsLocally(a):
			cli = "installed (" + a.CLI + "), signed in"
		case a.CLI != "":
			cli = "installed (" + a.CLI + "), not signed in"
		}
		fmt.Fprintf(&b, "%s here: CLI %s; cloud: %s.\n", delegate.Names[r], cli, a.CloudDetail)
		if a.CLI == "" {
			fmt.Fprintf(&b, "- Install it: %s\n", strings.Join(a.Install, "; "))
		}
		if !e.runsLocally(a) {
			fmt.Fprintf(&b, "- Sign in: %s\n", strings.Join(a.SignIn, "; "))
		}
		switch {
		case !wantLocal && e.runsLocally(a):
			b.WriteString("- Or add `--local` to run its CLI on this machine in a fresh aayu worktree.\n")
		case wantLocal && a.Cloud:
			b.WriteString("- Or drop `--local`: its cloud agent is connected.\n")
		}
	}
	for _, x := range delegate.Receivers {
		a := agents[x]
		switch {
		case x == r:
		case !wantLocal && a.Cloud:
			fmt.Fprintf(&b, "- %s's cloud agent is connected: `--delegate %s`.\n", delegate.Names[x], x)
		case wantLocal && e.runsLocally(a):
			fmt.Fprintf(&b, "- %s's CLI is ready here: `--delegate %s --local`.\n", delegate.Names[x], x)
		}
	}
	b.WriteString("aayu never installs or signs in for you; `aayu doctor` lists every agent.")
	return b.String()
}

// newDelegation resolves the receiver (auto picks one), the mode (cloud
// unless --local), and the adapter, and reads the story's delegation state
// from the ledger. A receiver not available in that mode is refused with
// receiverHelp.
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
	agents := e.agents(ctx, root, repo)
	cloud, local := map[string]bool{}, map[string]bool{}
	for _, r := range delegate.Receivers {
		cloud[r], local[r] = agents[r].Cloud, e.runsLocally(agents[r])
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
			return nil, Errorf(ExitBlocked, "build #%d --delegate auto: %v\n%s", n, err, e.receiverHelp("", agents, d.local))
		}
		d.receiver, d.local, d.reason = ch.Receiver, ch.Local, ch.Reason
	}
	// Cloud by default; the local CLI only on the user's explicit --local. A
	// receiver that is not available is named plainly, with what to do.
	if !slices.Contains(delegate.Receivers, d.receiver) {
		return nil, Errorf(ExitUsage, "build --delegate %q: want one of %s or auto", d.receiver, strings.Join(delegate.Receivers, ", "))
	}
	if !d.local && !cloud[d.receiver] {
		return nil, Errorf(ExitBlocked, "build #%d --delegate %s: %s's cloud agent is not connected here.\n%s", n, d.receiver, delegate.Names[d.receiver], e.receiverHelp(d.receiver, agents, false))
	}
	if d.local {
		bin, ok := delegate.LookLocal(d.receiver, e.lookPath())
		if !ok || !local[d.receiver] {
			return nil, Errorf(ExitBlocked, "build #%d --delegate %s --local: %s's CLI is not ready here.\n%s", n, d.receiver, delegate.Names[d.receiver], e.receiverHelp(d.receiver, agents, true))
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
		// The receiver's cloud environment is verified first: set up and
		// checked in a clean container when missing or stale (agentenv).
		if err := e.ensureCloudEnv(ctx, d, res); err != nil {
			return nil, true, err
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

// ensureCloudEnv runs agentenv.Ensure for the receiver before a cloud
// hand-off. A verified environment whose files are not on the default branch
// yet, or one that fails verification, stops the hand-off with the exact
// next step; --local stays available.
func (e *Env) ensureCloudEnv(ctx context.Context, d *delegation, res *BuildResult) error {
	ensure := e.EnsureEnv
	if ensure == nil {
		ensure = func(ctx context.Context, root, host string) (agentenv.HostStatus, error) {
			return agentenv.Ensure(ctx, root, host, agentenv.EnsureOptions{Run: e.envRunner(), Fetch: e.envFetch()})
		}
	}
	pp := e.progress(fmt.Sprintf("Checking %s's cloud environment", delegate.Names[d.receiver]))
	st, err := ensure(ctx, d.root, d.receiver)
	switch {
	case errors.Is(err, agentenv.ErrNotLanded):
		pp.Done(ui.StatusFail, "verified, not on the default branch yet")
		return Errorf(ExitBlocked, "build #%d --delegate %s: %s's cloud environment is verified here but its cloud agent reads it from the default branch, which does not have it yet: %v. Then hand off again, or add `--local`", d.story, d.receiver, delegate.Names[d.receiver], err)
	case err != nil:
		pp.Done(ui.StatusFail, err.Error())
		return Errorf(ExitBlocked, "build #%d --delegate %s: %s's cloud environment could not be set up and verified: %v. `aayu install --env --host %s --apply --verify` shows each step; or add `--local`", d.story, d.receiver, delegate.Names[d.receiver], err, d.receiver)
	}
	pp.Done(ui.StatusPass, st.State)
	res.Notes = append(res.Notes, fmt.Sprintf("%s's cloud environment: %s", d.receiver, strings.TrimSpace(st.State+" "+st.Detail)))
	return nil
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
	if p := delegate.Proved(pr.GetBody(), ""); p != "" {
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
	sentFor := slices.ContainsFunc(d.state.FollowUps, func(f delegate.FollowUp) bool { return f.Head == head })
	// P3: a failed required check goes back to the builder, one round per head,
	// within the same cap as inspect's fix cycles. The merge gate still decides.
	var upstream []string
	if l, err := e.Config(); err == nil {
		upstream = l.Config.Verify.UpstreamChecks
	}
	if fails, err := failedRequired(ctx, d.c, owner, name, head, upstream); err != nil {
		res.Notes = append(res.Notes, "required checks not read: "+oneLineErr(err))
	} else if len(fails) > 0 && !sentFor {
		var names []string
		for _, f := range fails {
			names = append(names, f.Name)
		}
		round := len(d.state.FollowUps)
		if round >= inspect.MaxCycles {
			d.event(now, dlgCapped, fmt.Sprintf("%d follow-up rounds; %s failed on %s", round, strings.Join(names, ", "), short(head)))
			page(fmt.Sprintf("%d follow-up rounds reached the cap and %s still failed", round, strings.Join(names, ", ")))
			report()
			_ = e.saveDelegation(ctx, d)
			return nil, true, Errorf(ExitBlocked, "build #%d: PR #%d reached the %d-cycle cap with %s failing; a human takes over (paged)", d.story, d.state.PR, inspect.MaxCycles, strings.Join(names, ", "))
		}
		msg := delegate.CIFollowUpMessage(d.story, d.state.PR, round+1, inspect.MaxCycles, fails)
		d.state.FollowUps = append(d.state.FollowUps, delegate.FollowUp{At: now.UTC().Format(time.RFC3339), Head: head, Cycle: round, Findings: []string{}, Checks: names})
		d.event(now, dlgFix, fmt.Sprintf("round %d: %s failed on %s", round+1, strings.Join(names, ", "), short(head)))
		res.Delegation.FollowUp = true
		report()
		if d.local {
			return e.localWriter(d, msg), false, nil
		}
		if err := e.act(ctx, d, "delegate.follow-up", map[string]any{"pr": d.state.PR, "head": head, "cycle": round, "checks": names}, func(ctx context.Context) error {
			t, err := d.adapter.FollowUp(ctx, d.state, msg)
			if err == nil && t.ID != "" {
				d.state.Task = t.ID
			}
			return err
		}); err != nil {
			return nil, true, Errorf(ExitExternal, "build #%d: follow-up to %s: %v", d.story, d.receiver, err)
		}
		res.Next = fmt.Sprintf("%s failed; sent to %s; sync again after its new commits", strings.Join(names, ", "), d.receiver)
		return nil, true, e.saveDelegation(ctx, d)
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
		if note := e.requestReview(ctx, d.state.PR); note != "" {
			res.Notes = append(res.Notes, note)
		}
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
	case sentFor:
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
		env := delegate.ScrubEnv(e.Environ, d.receiver, gh)
		for {
			var out string
			err := func() error {
				ctx, cancel := context.WithTimeout(ctx, limit)
				defer cancel()
				if !d.wallEnd.IsZero() { // a run never outlasts the build's wall-time budget
					var stop context.CancelFunc
					ctx, stop = context.WithDeadline(ctx, d.wallEnd)
					defer stop()
				}
				return e.act(ctx, d, "delegate.local-run", map[string]any{"bin": filepath.Base(d.bin), "worktree": wt, "prompt_sha256": delegate.BriefHash(prompt)}, func(ctx context.Context) error {
					var err error
					out, err = e.delegateExec()(ctx, wt, env, "", d.bin, delegate.LocalArgs(d.receiver, d.bin, wt, prompt)...)
					if p := delegate.Proved(out, prompt); p != "" {
						d.state.Proved = p
					}
					return err
				})
			}()
			if err == nil {
				return nil
			}
			// A subscription usage limit pauses the build until its window
			// resets, then runs the receiver again (founder decision,
			// 2026-10-17). A pause is no failed attempt; the wall clock runs on.
			limited, resets := delegate.UsageLimit(out, e.Now(), time.Local)
			if !limited {
				return err
			}
			name := delegate.Names[d.receiver]
			switch {
			case resets.IsZero():
				return &usagePause{why: fmt.Sprintf("%s's subscription usage limit was reached and it printed no reset time", name)}
			case !d.wallEnd.IsZero() && !resets.Add(time.Minute).Before(d.wallEnd):
				return &usagePause{afterWall: true, why: fmt.Sprintf("%s's subscription usage limit resets at %s, after the build's wall-time budget ends at %s",
					name, resets.UTC().Format(time.RFC3339), d.wallEnd.UTC().Format(time.RFC3339))}
			}
			d.event(e.Now(), dlgPaused, fmt.Sprintf("%s's subscription usage limit; resumes at %s", name, resets.UTC().Format(time.RFC3339)))
			_ = e.saveDelegation(ctx, d)
			pp := e.progress(fmt.Sprintf("Paused: %s's subscription usage limit; resuming at %s", name, resets.Local().Format("15:04 MST")))
			// The story's lease is renewed before each stretch of the wait, so it
			// never lapses to another builder while this one is paused.
			for wait := resets.Sub(e.Now()) + time.Minute; wait > 0; {
				step := min(wait, LeaseTTL/2)
				l, err := e.takeLease(ctx, d.c, d.repo, d.story, leaseHolder(d.bot.Login), nil)
				if err != nil {
					pp.Done(ui.StatusFail, "could not renew the story's lease while paused")
					return err
				}
				d.lease = &l
				if err := e.pauseFor(ctx, step); err != nil {
					pp.Done(ui.StatusFail, "stopped while paused")
					return err
				}
				wait -= step
			}
			pp.Done(ui.StatusPass, "the window reset")
			d.event(e.Now(), dlgWorking, "resumed after the usage limit reset")
		}
	}
}

// usagePause is a local receiver's subscription usage limit that build cannot
// wait out in this run: no reset time, or one after the wall-time budget.
type usagePause struct {
	why       string
	afterWall bool // the reset comes after the wall-time budget: a rerun then needs a fresh budget
}

func (p *usagePause) Error() string { return p.why }

// pauseFor waits for d or until ctx ends (Options.Sleep in tests).
func (e *Env) pauseFor(ctx context.Context, d time.Duration) error {
	if e.Sleep != nil {
		return e.Sleep(ctx, d)
	}
	t := time.NewTimer(max(d, 0))
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
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

// fixDelegateEnv re-syncs the agent environments' generated files with
// .aayudha/env.toml (doctor --fix), offline: the spec holds every pin, so
// nothing is resolved or verified here (aayu install --env does both).
func (e *Env) fixDelegateEnv() (string, error) {
	root := config.RepoRoot(e.Dir)
	if root == "" {
		return "", nil
	}
	s, ok, err := agentenv.LoadSpec(root)
	if err != nil {
		return "", Wrap(ExitBlocked, fmt.Errorf("doctor --fix: %w", err))
	}
	if !ok {
		return "--fix: no " + agentenv.SpecPath + ", so no agent environment to re-sync; `aayu install --env` sets one up", nil
	}
	files, _, err := agentenv.Generate(root, s)
	if err != nil {
		return "", Wrap(ExitBlocked, fmt.Errorf("doctor --fix: %w", err))
	}
	var changed []string
	for _, f := range files {
		old, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(f.Path)))
		fi, _ := os.Stat(filepath.Join(root, filepath.FromSlash(f.Path)))
		if err == nil && string(old) == string(f.Content) && (!f.Exec || fi.Mode()&0o111 != 0) {
			continue
		}
		changed = append(changed, f.Path)
	}
	if len(changed) == 0 {
		return "--fix: the agent environment files match " + agentenv.SpecPath, nil
	}
	if e.DryRun {
		return "--fix (dry run): would rewrite " + strings.Join(changed, ", ") + " from " + agentenv.SpecPath, nil
	}
	if err := agentenv.Write(root, s); err != nil {
		return "", err
	}
	return "--fix: rewrote " + strings.Join(changed, ", ") + " from " + agentenv.SpecPath + "; commit them to the default branch, then `aayu install --env --verify`", nil
}
