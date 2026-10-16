package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/agentenv"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/initrepo"
	"github.com/rahulnakmol/agentic-ayudha/internal/loops"
)

// loopHost is one agent host's part of `aayu install --loops`: whether its
// CLI was detected, what was applied, the exact steps left, and each loop's
// state there.
type loopHost struct {
	Host     string        `json:"host"`
	Detected bool          `json:"detected"`
	Applied  []string      `json:"applied,omitempty"`
	Steps    []string      `json:"steps"`
	States   []loops.State `json:"states"`
}

type loopsReport struct {
	Repo  string     `json:"repo"`
	Stale []string   `json:"stale_files,omitempty"`
	Hosts []loopHost `json:"hosts"`
}

// runInstallLoops is `aayu install --loops` (Appendix B S-7.1d): for each
// agent host this repository's user has (agentenv.Detect, or --host), set up
// the repository's loops in the host's own tooling where it can (with --apply,
// Cursor's Terraform), show the exact steps for the rest, and with --record
// note a loop as set up so doctor can say active, missing, or stale. It is
// per repository: it reads only this checkout's loops/*.toml.
func runInstallLoops(ctx context.Context, e *Env, in Values) error {
	root := config.RepoRoot(e.Dir)
	if root == "" {
		return Errorf(ExitUsage, "install --loops: run inside a git repository")
	}
	ls, err := loops.Load(root)
	if err != nil {
		return Wrap(ExitUsage, err)
	}
	if len(ls) == 0 {
		return Errorf(ExitUsage, "install --loops: no loops/*.toml in this repository; `aayu init --gates` adds the defaults")
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
	rec, err := loops.ReadRecord(root)
	if err != nil {
		return Wrap(ExitUsage, err)
	}
	if r := in.String("record"); r != "" {
		return e.recordLoops(root, ls, rec, r, repo, branch, tz)
	}
	if in.Bool("apply") && e.Agent() {
		return Errorf(ExitBlocked, "install --loops --apply creates automations that run as you; a human runs it (AAYU_AGENT=1)")
	}
	rep := loopsReport{Repo: repo}
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
	hosts := loops.AgentHosts
	if e.Host != "" && e.Host != "all" {
		if !slices.Contains(hosts, e.Host) {
			return Errorf(ExitUsage, "install --loops --host %s: want %s", e.Host, strings.Join(hosts, ", "))
		}
		hosts = []string{e.Host}
	}
	for _, h := range hosts {
		lh := loopHost{Host: h, Detected: agents[h].CLI != ""}
		if e.Host == "" && !lh.Detected {
			continue
		}
		mine := hostLoops(ls, h)
		switch h {
		case loops.HostAmp:
			lh.Steps = e.ampLoopSteps(ctx, root, repo, mine)
		case loops.HostCursor:
			lh.Applied, lh.Steps = e.cursorLoops(ctx, root, in.Bool("apply"), rec, mine, repo, branch, tz)
		case loops.HostClaude:
			lh.Steps = []string{
				"Claude Code has no file or API that creates a routine: create each one in " + loops.Dir + "/claude/routines.md at https://claude.ai/code/routines, or with `/schedule` in Claude Code (signed in with claude.ai, not an API key).",
			}
		case loops.HostCodex:
			lh.Steps = []string{
				"Codex has no file or API that creates a scheduled task: create each one in " + loops.Dir + "/codex/scheduled-tasks.md in the ChatGPT desktop app (Scheduled, New task).",
			}
		}
		if len(mine) > 0 {
			lh.Steps = append(lh.Steps, fmt.Sprintf("Then record each one: `aayu install --loops --host %s --record <loop>=<its %s ID>` (or all=<ID>), and commit %s.", h, h, loops.RecordFile))
		}
		lh.States = loops.States(ls, rec, []string{h}, repo, branch, tz)
		rep.Hosts = append(rep.Hosts, lh)
	}
	return e.Emit(rep, func(w io.Writer) error {
		if len(rep.Stale) > 0 {
			fmt.Fprintf(w, "note: %s differ from loops/*.toml: run `aayu init --gates` (or `go generate ./...` in aayudha) first\n", strings.Join(rep.Stale, ", "))
		}
		if len(rep.Hosts) == 0 {
			fmt.Fprintln(w, "install --loops: no agent CLI detected (amp, cursor-agent, claude, codex); name one with --host")
		}
		for _, lh := range rep.Hosts {
			fmt.Fprintf(w, "\n%s:\n", lh.Host)
			for _, a := range lh.Applied {
				fmt.Fprintln(w, "  applied:", a)
			}
			for i, s := range lh.Steps {
				fmt.Fprintf(w, "  %d. %s\n", i+1, s)
			}
			for _, s := range lh.States {
				fmt.Fprintf(w, "  %-18s %-7s %s\n", s.Loop, s.Status, s.Detail)
			}
		}
		return nil
	})
}

// hostLoops are the loops host starts by itself.
func hostLoops(ls []loops.Loop, host string) []loops.Loop {
	var out []loops.Loop
	for _, l := range ls {
		if ok, _ := l.Native(host); ok && l.Runs(host) && Implemented(l.Verb()) {
			out = append(out, l)
		}
	}
	return out
}

// ampLoopSteps are Amp's steps: Amp has no CLI for schedules or GitHub
// subscriptions, so an Amp thread of the project sets each one with its own
// tools, as the user. One schedule lives on one thread.
func (e *Env) ampLoopSteps(ctx context.Context, root, repo string, ls []loops.Loop) []string {
	project := "<the project's ID: amp projects get " + repo + " --json>"
	if out, err := e.run(ctx, root, "amp", "projects", "get", repo, "--json"); err == nil {
		var p struct {
			ID string `json:"id"`
		}
		if json.Unmarshal([]byte(out), &p) == nil && p.ID != "" {
			project = p.ID
		}
	}
	var steps []string
	for _, l := range ls {
		path := loops.Dir + "/amp/" + l.Name + ".json"
		if _, isEvent := l.Event(); isEvent {
			steps = append(steps, fmt.Sprintf("In any Amp thread, ask: \"Call create_github_subscription with the input in %s, adding project_id %s and agent_mode medium.\"", path, project))
		} else {
			steps = append(steps, fmt.Sprintf("In a new Amp thread of the project (one schedule per thread), ask: \"Call set_schedule with the input in %s, exactly as written.\"", path))
		}
	}
	return steps
}

// cursorLoops applies loops/cursor/aayudha-loops.tf with Terraform when asked
// and possible (terraform on PATH and CURSOR_TOKEN set), recording every
// Cursor loop as set up; otherwise it returns the commands.
func (e *Env) cursorLoops(ctx context.Context, root string, apply bool, rec loops.Record, ls []loops.Loop, repo, branch, tz string) ([]string, []string) {
	dir := loops.Dir + "/cursor"
	steps := []string{fmt.Sprintf("With a Cursor API key in CURSOR_TOKEN: `terraform -chdir=%s init && terraform -chdir=%s apply` (the cursor/cursor provider); set each automation's Run as to Me.", dir, dir)}
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
	return applied, nil
}

// recordLoops is `install --loops --host <h> --record <loop>=<id>[,…]`, or
// all=<id> for every loop the host starts by itself.
func (e *Env) recordLoops(root string, ls []loops.Loop, rec loops.Record, arg, repo, branch, tz string) error {
	if !slices.Contains(loops.AgentHosts, e.Host) {
		return Errorf(ExitUsage, "install --loops --record: name the host with --host (%s)", strings.Join(loops.AgentHosts, ", "))
	}
	mine := hostLoops(ls, e.Host)
	today := e.Now().UTC().Format(time.DateOnly)
	var done []string
	for _, pair := range strings.Split(arg, ",") {
		name, id, ok := strings.Cut(strings.TrimSpace(pair), "=")
		if !ok || id == "" {
			return Errorf(ExitUsage, "install --loops --record %q: want <loop>=<ID>", pair)
		}
		for _, l := range mine {
			if name == "all" || name == l.Name {
				rec.Set(e.Host, l.Name, loops.Entry{Digest: l.Digest(repo, branch, tz), ID: id, At: today})
				done = append(done, l.Name)
			}
		}
		if name != "all" && !slices.Contains(done, name) {
			return Errorf(ExitUsage, "install --loops --record: %s does not start loop %q by itself", e.Host, name)
		}
	}
	if err := rec.Write(root); err != nil {
		return err
	}
	return e.Emit(map[string]any{"host": e.Host, "recorded": done}, func(w io.Writer) error {
		_, err := fmt.Fprintf(w, "install --loops: recorded %s as set up on %s in %s\n", strings.Join(done, ", "), e.Host, loops.RecordFile)
		return err
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
