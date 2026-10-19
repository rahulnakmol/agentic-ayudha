package cli

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/agentenv"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/initrepo"
	"github.com/rahulnakmol/agentic-ayudha/internal/loops"
)

// loopsReport is `aayu install --loops`: the one tool that runs this
// repository's loops, what was applied there, the exact steps left, how to
// test one loop now, and each loop's state.
type loopsReport struct {
	Repo     string        `json:"repo"`
	Tool     string        `json:"tool"`
	Detected []string      `json:"detected"`
	Added    []string      `json:"added,omitempty"` // the default loops and their renders, written to a repository that had no loops
	Stale    []string      `json:"stale_files,omitempty"`
	Warnings []string      `json:"warnings,omitempty"`
	Applied  []string      `json:"applied,omitempty"`
	Actions  []string      `json:"actions,omitempty"` // loops with agent = false: the Actions fallback runs them
	Steps    []string      `json:"steps"`
	RunNow   string        `json:"run_now,omitempty"`
	States   []loops.State `json:"states"`
}

// runInstallLoops is `aayu install --loops` (Appendix B S-7.1d, S-7.1e): the
// user picks one tool to run every loop of this repository (offered only
// from the tools agentenv.Detect finds, or named with --host), and it sets
// the loops up there: Amp and Cursor with --apply, Claude Code and Codex
// with a ready prompt per loop. It warns when loops/active.json records loops
// in another tool, since each would run twice. --run-now tests one loop in
// the tool; --record notes loops as set up. It reads only this checkout's
// loops/*.toml; a repository with none first gets the defaults and their
// renders (Appendix B D-7.1g).
func runInstallLoops(ctx context.Context, e *Env, in Values) error {
	root := config.RepoRoot(e.Dir)
	if root == "" {
		return Errorf(ExitUsage, "install --loops: run inside a git repository")
	}
	if h := e.Host; h != "" && h != "all" && !slices.Contains(loops.AgentHosts, h) {
		return Errorf(ExitUsage, "install --loops: %s does not run aayudha's loops; name one that does with --host (%s)", h, strings.Join(loops.AgentHosts, ", "))
	}
	ls, err := loops.Load(root)
	if err != nil {
		return Wrap(ExitUsage, err)
	}
	repo, err := e.RepoName(ctx)
	if err != nil {
		return err
	}
	l, err := e.Config()
	if err != nil {
		return err
	}
	branch, tz := l.Config.Branches.Dev, l.Config.Time.TZ
	if branch == "" {
		branch = "dev"
	}
	var added []string
	if len(ls) == 0 {
		if in.Has("record") {
			return Errorf(ExitUsage, "install --loops --record: no loops/*.toml in this repository; run `aayu install --loops` first, which adds the defaults")
		}
		if ls, added, err = e.addDefaultLoops(root, repo, branch, tz); err != nil {
			return err
		}
	}
	rec, err := loops.ReadRecord(root)
	if err != nil {
		return Wrap(ExitUsage, err)
	}
	if r := in.String("record"); in.Has("record") {
		return e.recordLoops(root, ls, rec, r, repo, branch, tz)
	}
	// --apply and --run-now start runs that act as the person. In a shell with
	// an agent marker (an orb's Terminal tab inherits one) a person may still be
	// at the terminal, so they approve it through the device flow; with no
	// terminal nobody can, so it is refused (Bug #698).
	acts, sig := in.Bool("apply") || in.String("run-now") != "", e.agentSignal()
	refusal := Errorf(ExitBlocked, "install --loops --apply and --run-now start runs that act as you; a human runs them (agent lane: %s)", sig)
	if acts && sig != "" && !e.isTerminal(e.In) {
		return refusal
	}
	rep := loopsReport{Repo: repo, Added: added}
	files, err := loops.Files(ls, repo, branch, tz, initrepo.GuardVersion(e.Build.Version), Implemented)
	if err != nil {
		return Wrap(ExitUsage, err)
	}
	for _, p := range slices.Sorted(maps.Keys(files)) {
		if b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(p))); err != nil || !bytes.Equal(b, files[p]) {
			rep.Stale = append(rep.Stale, p)
		}
	}
	agents := e.agents(ctx, root, repo)
	for _, h := range loops.AgentHosts {
		if agents[h].CLI != "" {
			rep.Detected = append(rep.Detected, h)
		}
	}
	answers := bufio.NewReader(e.In)
	if rep.Tool, err = e.loopTool(answers, rep.Detected); err != nil {
		return err
	}
	tool := rep.Tool
	if !slices.Contains(rep.Detected, tool) {
		rep.Warnings = append(rep.Warnings, fmt.Sprintf("the %s CLI is not detected here; the steps below assume you have %s", tool, tool))
	}
	for _, h := range loops.AgentHosts {
		if h != tool && len(rec[h]) > 0 {
			names := slices.Sorted(maps.Keys(rec[h]))
			rep.Warnings = append(rep.Warnings, fmt.Sprintf("%s records %s as set up in %s. Set up in %s as well, each would run twice: delete them in %s, then forget them with `aayu install --loops --host %s --record %s=` (an empty ID).",
				loops.RecordFile, strings.Join(names, ", "), h, tool, h, h, names[0]))
		}
	}
	mine := toolLoops(ls, tool)
	if p := loopsPending(tool, in.Bool("apply"), in.String("run-now"), mine, repo); sig != "" && len(p.Effect) > 0 {
		if _, err := e.presenceCheck(ctx, p, AccessWrite, refusal); err != nil {
			return err
		}
	}
	for _, l := range ls {
		if !l.NeedsAgent() {
			rep.Actions = append(rep.Actions, l.Name)
		}
	}
	switch tool {
	case loops.HostAmp:
		rep.Applied, rep.Steps = e.ampLoops(ctx, root, agents[tool].CLI, in.Bool("apply"), rec, mine, repo, branch, tz)
	case loops.HostCursor:
		rep.Applied, rep.Steps = e.cursorLoops(ctx, root, in.Bool("apply"), rec, mine, repo, branch, tz)
	case loops.HostClaude:
		rep.Steps = []string{
			fmt.Sprintf("In Claude Code v2.1.225 or later, signed in with a claude.ai subscription (not an API key) and started in this checkout, paste each `/schedule` prompt in %s/claude/routines.md (%d loops). Each saves a routine that runs as you in Anthropic's cloud.", loops.Dir, len(mine)),
			"The routines' cloud environment needs the aayu binary: run `aayu install --env --host claude` and follow it.",
		}
	case loops.HostCodex:
		rep.Steps = []string{fmt.Sprintf("In the ChatGPT desktop app, open a Codex or ChatGPT chat in this repository's local project and paste each prompt in %s/codex/scheduled-tasks.md (%d loops). Each creates a scheduled task that runs as you while the app is open.", loops.Dir, len(mine))}
	}
	for _, l := range ls {
		if l.IsEvent() && !l.Runs(tool) {
			s := fmt.Sprintf("%s starts %s; %s has no %s trigger, so there is nothing to set up for it", l.Name, strings.ToLower(l.Label(tz)[:1])+l.Label(tz)[1:], tool, l.On)
			if c, ok := loops.Covers(ls, l, tool); ok {
				s += ": " + c.Name + " covers it on its schedule"
			}
			rep.Steps = append(rep.Steps, s+".")
		}
	}
	if len(rep.Applied) == 0 && len(mine) > 0 {
		rep.Steps = append(rep.Steps, fmt.Sprintf("Then record them: `aayu install --loops --host %s --record all=<what %s calls them>` (or <loop>=<ID>,…), and commit %s.", tool, tool, loops.RecordFile))
	}
	if len(mine) > 0 {
		test := mine[0]
		if t, ok := loops.Find(mine, "triage-daily"); ok {
			test = t
		}
		rep.RunNow = fmt.Sprintf("aayu install --loops --host %s --run-now %s", tool, test.Name)
		if name := in.String("run-now"); name != "" {
			return e.runLoopNow(ctx, root, agents[tool], tool, mine, name, repo, branch)
		}
		if e.isTerminal(e.In) && !e.Agent() {
			e.printLoops(e.Err, rep)
			fmt.Fprintf(e.Err, "Test the %s loop now in %s (Run now)? [y/N]: ", test.Name, tool)
			if ans, _ := answers.ReadString('\n'); strings.EqualFold(strings.TrimSpace(ans), "y") {
				return e.runLoopNow(ctx, root, agents[tool], tool, mine, test.Name, repo, branch)
			}
			return nil
		}
	} else if in.String("run-now") != "" {
		return Errorf(ExitUsage, "install --loops --run-now: %s runs no loop of this repository", tool)
	}
	if rec, err = loops.ReadRecord(root); err != nil { // --apply may have recorded loops
		return Wrap(ExitUsage, err)
	}
	rep.States = loops.States(ls, rec, repo, branch, tz)
	return e.Emit(rep, func(w io.Writer) error { e.printLoops(w, rep); return nil })
}

// addDefaultLoops writes the default loops (loops.Defaults) to a repository
// that has none, with every tool's render of them, as `aayu init --gates`
// renders them, so the steps that follow name files that exist.
func (e *Env) addDefaultLoops(root, repo, branch, tz string) ([]loops.Loop, []string, error) {
	if e.Assets == nil {
		return nil, nil, Errorf(ExitUsage, "install --loops: no loops/*.toml in this repository, and this binary carries no defaults")
	}
	defs, err := loops.Defaults(e.Assets, root)
	if err != nil {
		return nil, nil, Wrap(ExitUsage, err)
	}
	write := func(files map[string][]byte) ([]string, error) {
		var paths []string
		for _, p := range slices.Sorted(maps.Keys(files)) {
			f := filepath.Join(root, filepath.FromSlash(p))
			if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
				return nil, err
			}
			if err := os.WriteFile(f, files[p], 0o644); err != nil {
				return nil, err
			}
			paths = append(paths, p)
		}
		return paths, nil
	}
	added, err := write(defs)
	if err != nil {
		return nil, nil, err
	}
	ls, err := loops.Load(root)
	if err != nil {
		return nil, nil, Wrap(ExitUsage, err)
	}
	renders, err := loops.Files(ls, repo, branch, tz, initrepo.GuardVersion(e.Build.Version), Implemented)
	if err != nil {
		return nil, nil, Wrap(ExitUsage, err)
	}
	more, err := write(renders)
	return ls, append(added, more...), err
}

// loopTool is the one tool that runs the loops: --host, or the human's pick
// from the detected tools at a terminal, or the only one detected.
func (e *Env) loopTool(answers *bufio.Reader, detected []string) (string, error) {
	switch h := e.Host; {
	case h == "all":
		return "", Errorf(ExitUsage, "install --loops: one tool runs the loops; name it with --host (%s)", strings.Join(loops.AgentHosts, ", "))
	case h != "":
		return h, nil
	case len(detected) == 0:
		return "", Errorf(ExitUsage, "install --loops: no agent CLI detected (amp, cursor-agent, claude, codex); name the tool that runs the loops with --host")
	case e.isTerminal(e.In) && !e.Agent():
		fmt.Fprintf(e.Err, "Which tool should run this repository's loops? Detected: %s. [%s]: ", strings.Join(detected, ", "), strings.Join(detected, "/"))
		line, _ := answers.ReadString('\n')
		if ans := strings.ToLower(strings.TrimSpace(line)); slices.Contains(detected, ans) {
			return ans, nil
		}
		return "", Errorf(ExitUsage, "install --loops: answer one of %s", strings.Join(detected, ", "))
	case len(detected) == 1:
		return detected[0], nil
	}
	return "", Errorf(ExitUsage, "install --loops: one tool runs the loops; name it with --host (detected: %s)", strings.Join(detected, ", "))
}

func (e *Env) printLoops(w io.Writer, rep loopsReport) {
	if len(rep.Added) > 0 {
		fmt.Fprintf(w, "This repository had no loops, so the defaults were added (%d files: loops/*.toml, each tool's render in %s/, and %s); commit them.\n", len(rep.Added), loops.Dir, ".github/workflows/"+loops.Workflow)
	}
	if len(rep.Stale) > 0 {
		fmt.Fprintf(w, "note: %s differ from loops/*.toml: run `aayu init --gates` (or `go generate ./...` in aayudha) first\n", strings.Join(rep.Stale, ", "))
	}
	fmt.Fprintf(w, "The agent loops run in %s.\n", rep.Tool)
	if len(rep.Actions) > 0 {
		fmt.Fprintf(w, "%s need no agent: the GitHub Actions fallback (%s) runs them, with no model.\n", strings.Join(rep.Actions, ", "), loops.Workflow)
	}
	for _, x := range rep.Warnings {
		fmt.Fprintln(w, "warning:", x)
	}
	for _, a := range rep.Applied {
		fmt.Fprintln(w, "  set up:", a)
	}
	for i, s := range rep.Steps {
		fmt.Fprintf(w, "  %d. %s\n", i+1, s)
	}
	if rep.RunNow != "" {
		fmt.Fprintf(w, "  Run now, to test one loop: `%s`\n", rep.RunNow)
	}
	for _, s := range rep.States {
		fmt.Fprintf(w, "  %-18s %-7s %s\n", s.Loop, s.Status, s.Detail)
	}
}

// loopsPending is what approving --apply or --run-now starts: the loops,
// in the tool, acting as the person. Claude Code and Codex have no --apply
// (aayu prints their prompts), so --apply alone starts nothing there.
func loopsPending(tool string, apply bool, runNow string, mine []loops.Loop, repo string) Pending {
	names := make([]string, len(mine))
	for i, l := range mine {
		names[i] = l.Name
	}
	p := Pending{What: "install --loops"}
	if apply {
		p.What += " --apply"
		switch tool {
		case loops.HostAmp:
			if len(mine) > 0 { // with no loop to set up, no thread starts: nothing to approve
				p.Effect = append(p.Effect, fmt.Sprintf("Starts one headless Amp thread per loop in orbs of the %s project, to set up %s. They act as you, with your Amp and GitHub accounts.", repo, strings.Join(names, ", ")))
			}
		case loops.HostCursor:
			p.Effect = append(p.Effect, fmt.Sprintf("Runs terraform apply of %s/cursor/aayudha-loops.tf with your CURSOR_TOKEN, which creates a Cursor automation per loop (%s) that runs as you.", loops.Dir, strings.Join(names, ", ")))
		}
	}
	if runNow != "" {
		p.What += " --run-now"
		p.Effect = append(p.Effect, fmt.Sprintf("Runs the loop %s once now with %s's CLI in this checkout. It acts as you.", runNow, tool))
	}
	return p
}

// toolLoops are the loops tool runs.
func toolLoops(ls []loops.Loop, tool string) []loops.Loop {
	var out []loops.Loop
	for _, l := range ls {
		if l.Runs(tool) && Implemented(l.Verb()) {
			out = append(out, l)
		}
	}
	return out
}

// runNowArgs is how each tool's CLI runs one prompt headless, as the user.
var runNowArgs = map[string]struct{ bin, flag string }{
	loops.HostAmp:    {"amp", "-x"},
	loops.HostClaude: {"claude", "-p"},
	loops.HostCodex:  {"codex", "exec"},
	loops.HostCursor: {"cursor-agent", "-p"},
}

// runLoopNow runs one loop's Run now prompt once with the tool's own CLI in
// this checkout, so the user sees it work before it runs on its schedule.
func (e *Env) runLoopNow(ctx context.Context, root string, a agentenv.Agent, tool string, ls []loops.Loop, name, repo, branch string) error {
	l, ok := loops.Find(ls, name)
	if !ok {
		return Errorf(ExitUsage, "install --loops --run-now %s: %s runs no loop by that name", name, tool)
	}
	bin := a.CLI
	if bin == "" {
		bin = runNowArgs[tool].bin
	}
	fmt.Fprintf(e.Err, "install --loops: running loop %s once with %s %s …\n", name, bin, runNowArgs[tool].flag)
	out, err := e.run(ctx, root, bin, runNowArgs[tool].flag, l.RunNowPrompt(repo, branch))
	fmt.Fprintln(e.Out, strings.TrimSpace(out))
	if err != nil {
		return Errorf(ExitExternal, "install --loops --run-now %s: %s: %v", name, bin, err)
	}
	return nil
}

// subscriptionID is the reply of a thread that created an event loop's Amp
// GitHub subscription: `subscription <ID>`.
var subscriptionID = regexp.MustCompile(`^subscription [0-9A-Za-z][0-9A-Za-z-]{7,}$`)

// threadID finds an Amp thread ID in a CLI's output.
var threadID = regexp.MustCompile(`T-[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)

// ampLoops sets each loop's Amp schedule with --apply: Amp schedules are an
// agent tool (set_schedule) and one thread holds one schedule, so it starts
// one headless Amp thread per loop in an orb of the project (`amp -ox`),
// which calls set_schedule with the rendered input and keeps the schedule
// (for an event loop, create_github_subscription: an orb spawner), and
// records the loop. Without --apply it returns the steps.
func (e *Env) ampLoops(ctx context.Context, root, bin string, apply bool, rec loops.Record, ls []loops.Loop, repo, branch, tz string) ([]string, []string) {
	if !apply {
		return nil, []string{
			fmt.Sprintf("Run `aayu install --loops --host amp --apply`: for each of the %d loops it starts one headless Amp thread in an orb of the project (`amp -ox`), which calls set_schedule with %s/amp/<loop>.json and keeps that schedule (one schedule per thread), or, for an event loop, create_github_subscription with it, and it records each loop.", len(ls), loops.Dir),
			fmt.Sprintf("Or by hand: in a new Amp thread of the project for each loop, ask \"Call set_schedule with the input in %s/amp/<loop>.json, exactly as written.\" For an event loop, ask for create_github_subscription with that input and this project's project_id instead.", loops.Dir),
		}
	}
	if bin == "" {
		bin = "amp"
	}
	var applied, steps, pending []string
	async := false
	today := e.Now().UTC().Format(time.DateOnly)
	for _, l := range ls {
		in, err := os.ReadFile(filepath.Join(root, loops.Dir, "amp", l.Name+".json"))
		if err != nil {
			steps = append(steps, fmt.Sprintf("%s: %v; run `aayu init --gates` first", l.Name, err))
			continue
		}
		prompt := "Call Amp's set_schedule on this thread with exactly this input, and do nothing else. Then reply with one line: `schedule <its ID> on <this thread's ID>`.\n\n" + string(in)
		if l.IsEvent() {
			prompt = "Call Amp's create_github_subscription with exactly this input, adding project_id: the ID of this thread's Amp project, for " + repo + ". Do nothing else. Then reply with one line: `subscription <its ID>`.\n\n" + string(in)
		}
		out, err := e.run(ctx, root, bin, "-ox", "--project", repo, "--no-archive-after-execute", "--title", "aayudha loop: "+l.Name, "-x", prompt)
		if err != nil {
			steps = append(steps, fmt.Sprintf("%s: amp failed: %v: %s", l.Name, err, lastLine(out)))
			continue
		}
		id := lastLine(out)
		if l.IsEvent() && !subscriptionID.MatchString(id) {
			// No subscription ID yet: the spawner is not ready until the thread has set it up, so nothing is recorded.
			pending = append(pending, l.Name)
			applied = append(applied, l.Name+": pending on "+threadID.FindString(out))
			continue
		}
		if t := threadID.FindString(out); t != "" && !strings.Contains(id, t) && !l.IsEvent() {
			id = "schedule on " + t
		}
		async = async || !strings.HasPrefix(id, "schedule ") && !l.IsEvent()
		rec.Set(loops.HostAmp, l.Name, loops.Entry{Digest: l.Digest(repo, branch, tz), ID: id, At: today})
		applied = append(applied, l.Name+": "+id)
	}
	if len(applied) > 0 {
		if err := rec.Write(root); err != nil {
			steps = append(steps, "recording failed: "+err.Error())
		}
		if async {
			// amp -ox returns the thread before its agent runs (Appendix B D-7.1f).
			steps = append(steps, "Each Amp thread sets its schedule a few seconds after it starts: open each one to check it holds the schedule, then record the schedule ID with `aayu install --loops --host amp --record <loop>=schedule <ID> on <thread>`.")
		}
		for _, n := range pending {
			steps = append(steps, fmt.Sprintf("%s is not recorded: its thread had not replied with a subscription ID. Open the thread, check that Amp shows the GitHub subscription, then record it with `aayu install --loops --host amp --record %s=subscription <ID>`.", n, n))
		}
		steps = append(steps, "Commit "+loops.RecordFile+".")
	}
	return applied, steps
}

// cursorLoops applies loops/cursor/aayudha-loops.tf with Terraform when asked
// and possible (terraform on PATH and CURSOR_TOKEN set), recording every
// Cursor loop as set up; otherwise it returns the commands, and the
// /automate prompts for a user without Terraform or a key.
func (e *Env) cursorLoops(ctx context.Context, root string, apply bool, rec loops.Record, ls []loops.Loop, repo, branch, tz string) ([]string, []string) {
	dir := loops.Dir + "/cursor"
	steps := []string{
		fmt.Sprintf("With Terraform and a Cursor API key in CURSOR_TOKEN: `aayu install --loops --host cursor --apply` runs `terraform -chdir=%s init && terraform -chdir=%s apply` (the cursor/cursor provider) and records the loops.", dir, dir),
		fmt.Sprintf("Without them: paste each prompt in %s/automate.md into a local Cursor agent session; the /automate skill creates the automation.", dir),
	}
	_, tfErr := e.lookPath()("terraform")
	switch {
	case !apply:
		return nil, steps
	case tfErr != nil:
		return nil, append(steps, "terraform is not on PATH: install it, then run aayu install --loops --host cursor --apply again.")
	case e.Getenv("CURSOR_TOKEN") == "":
		return nil, append(steps, "CURSOR_TOKEN is not set: create a key at https://cursor.com/dashboard (Integrations), export it, and run again.")
	}
	for _, args := range [][]string{{"-chdir=" + dir, "init", "-input=false"}, {"-chdir=" + dir, "apply", "-auto-approve", "-input=false"}} {
		if out, err := e.run(ctx, root, "terraform", args...); err != nil {
			return nil, append(steps, fmt.Sprintf("terraform %s failed: %v: %s", args[1], err, lastLine(out)))
		}
	}
	var applied []string
	for _, l := range ls {
		rec.Set(loops.HostCursor, l.Name, loops.Entry{Digest: l.Digest(repo, branch, tz), ID: "terraform:" + dir, At: e.Now().UTC().Format(time.DateOnly)})
		applied = append(applied, l.Name)
	}
	if err := rec.Write(root); err != nil {
		return nil, append(steps, "recording failed: "+err.Error())
	}
	return applied, []string{"Commit " + loops.RecordFile + "."}
}

// recordLoops is `install --loops --host <h> --record <loop>=<id>[,…]`, or
// all=<id> for every loop the tool runs; an empty ID forgets the loop there.
// Recording a loop that another tool holds warns that it runs twice.
func (e *Env) recordLoops(root string, ls []loops.Loop, rec loops.Record, arg, repo, branch, tz string) error {
	if !slices.Contains(loops.AgentHosts, e.Host) {
		return Errorf(ExitUsage, "install --loops --record: name the tool with --host (%s)", strings.Join(loops.AgentHosts, ", "))
	}
	mine := toolLoops(ls, e.Host)
	today := e.Now().UTC().Format(time.DateOnly)
	var done, forgot, warn []string
	for _, pair := range strings.Split(arg, ",") {
		name, id, ok := strings.Cut(strings.TrimSpace(pair), "=")
		if !ok {
			return Errorf(ExitUsage, "install --loops --record %q: want <loop>=<ID>", pair)
		}
		matched := false
		for _, l := range mine {
			if name != "all" && name != l.Name {
				continue
			}
			matched = true
			if id == "" {
				rec.Delete(e.Host, l.Name)
				forgot = append(forgot, l.Name)
				continue
			}
			rec.Set(e.Host, l.Name, loops.Entry{Digest: l.Digest(repo, branch, tz), ID: id, At: today})
			done = append(done, l.Name)
			if o := rec.Owners(l.Name); len(o) > 1 {
				warn = append(warn, fmt.Sprintf("%s is also recorded in %s: it runs twice until you delete it from all but one", l.Name, strings.Join(slices.DeleteFunc(o, func(h string) bool { return h == e.Host }), ", ")))
			}
		}
		if _, held := rec[e.Host][name]; !matched && held && id == "" {
			rec.Delete(e.Host, name) // a loop that no longer runs in the tool, such as one now agent = false
			forgot = append(forgot, name)
			continue
		}
		if !matched && name != "all" {
			return Errorf(ExitUsage, "install --loops --record: %s runs no loop %q", e.Host, name)
		}
	}
	if err := rec.Write(root); err != nil {
		return err
	}
	return e.Emit(map[string]any{"host": e.Host, "recorded": done, "forgotten": forgot, "warnings": warn}, func(w io.Writer) error {
		if len(done) > 0 {
			fmt.Fprintf(w, "install --loops: recorded %s as set up in %s in %s\n", strings.Join(done, ", "), e.Host, loops.RecordFile)
		}
		if len(forgot) > 0 {
			fmt.Fprintf(w, "install --loops: forgot %s in %s\n", strings.Join(forgot, ", "), e.Host)
		}
		for _, x := range warn {
			fmt.Fprintln(w, "warning:", x)
		}
		return nil
	})
}

func (e *Env) run(ctx context.Context, dir, name string, args ...string) (string, error) {
	x := e.Exec
	if x == nil {
		x = agentenv.CombinedExec
	}
	return x(ctx, dir, name, args...)
}

func lastLine(s string) string {
	s = strings.TrimSpace(s)
	return s[strings.LastIndex(s, "\n")+1:]
}
