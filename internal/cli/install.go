package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/doctor"
	"github.com/rahulnakmol/agentic-ayudha/internal/hosts"
	"github.com/rahulnakmol/agentic-ayudha/internal/inspect"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations/hostcall"
	"github.com/rahulnakmol/agentic-ayudha/internal/journey"
	"github.com/rahulnakmol/agentic-ayudha/internal/upgrade"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

func init() {
	register("install", runInstall)
	register("hook", runHook)
}

// homeDir is the user's home directory (HOME, or USERPROFILE on Windows).
func (e *Env) homeDir() string {
	if h := e.Getenv("HOME"); h != "" {
		return h
	}
	return e.Getenv("USERPROFILE")
}

// installOptions builds the install plan options for this repository.
func (e *Env) installOptions() (hosts.Options, error) {
	root := config.RepoRoot(e.Dir)
	if root == "" {
		return hosts.Options{}, Errorf(ExitUsage, "run inside a git repository")
	}
	if e.Assets == nil {
		return hosts.Options{}, errors.New("this binary has no embedded skills")
	}
	return hosts.Options{Root: root, Home: e.homeDir(), Assets: e.Assets, Version: e.Build.Version,
		SkillsVersion: verbs.SkillVersion, Windows: runtime.GOOS == "windows"}, nil
}

// binaryPath is the absolute, symlink-resolved path of the running binary, after
// checking that running it reports this binary's version (WP3.2: hooks and MCP
// registrations call the binary by absolute path).
func (e *Env) binaryPath(ctx context.Context) (string, error) {
	exe, err := e.Executable()
	if err != nil {
		return "", Wrap(ExitExternal, fmt.Errorf("locate the aayu binary: %w", err))
	}
	if r, err := filepath.EvalSymlinks(exe); err == nil {
		exe = r
	}
	if exe, err = filepath.Abs(exe); err != nil {
		return "", err
	}
	out, err := e.exec()(ctx, "", exe, "version")
	if err != nil || !strings.Contains(out, " "+e.Build.Version) {
		return "", Errorf(ExitExternal, "%s does not report version %s (got %q, %v); install a released aayu and run install from it", exe, e.Build.Version, strings.TrimSpace(out), err)
	}
	return exe, nil
}

type installReport struct {
	Edits   []installEdit `json:"edits"`
	Notes   []string      `json:"notes,omitempty"`
	Applied bool          `json:"applied"`
	Home    string        `json:"home_edits,omitempty"` // applied, skipped, or empty when none
}

type installEdit struct {
	Kind string `json:"kind"`
	Path string `json:"path"`
	Home bool   `json:"home,omitempty"`
}

func runInstall(ctx context.Context, e *Env, in Values) error {
	if in.Bool("loops") {
		return runInstallLoops(ctx, e, in)
	}
	if in.Bool("env") || in.Bool("verify") || in.Bool("cloud") || in.String("record") != "" {
		return runInstallEnv(ctx, e, in)
	}
	apply := in.Bool("apply")
	if apply {
		if err := e.requireHuman(ctx, "install --apply (it writes the hooks that govern agents)", AccessAdmin); err != nil {
			return err
		}
	}
	o, err := e.installOptions()
	if err != nil {
		return err
	}
	switch e.Host {
	case "":
	case "all":
		o.Hosts = hosts.AllHosts()
	default:
		o.Hosts = []verbs.HostID{verbs.HostID(e.Host)}
	}
	o.Pin = in.String("pin")
	if o.Binary, err = e.binaryPath(ctx); err != nil {
		return err
	}
	res, err := hosts.Plan(o)
	if errors.Is(err, hosts.ErrPinned) {
		return Wrap(ExitBlocked, err)
	}
	if err != nil {
		return Wrap(ExitUsage, fmt.Errorf("install: %w", err))
	}
	res.Notes = append(res.Notes, e.reviewerAppsNote(ctx)...)
	var repo, home []hosts.Edit
	rep := installReport{Notes: res.Notes, Applied: apply}
	for _, ed := range res.Edits {
		rep.Edits = append(rep.Edits, installEdit{ed.Kind, ed.Display(), ed.Home})
		if ed.Home {
			home = append(home, ed)
		} else {
			repo = append(repo, ed)
		}
	}
	text := func(w io.Writer) error {
		if len(res.Edits) == 0 {
			fmt.Fprintln(w, "install: nothing to change")
		}
		if len(repo) > 0 {
			fmt.Fprintf(w, "# Repository edits (%d)\n%s", len(repo), hosts.Render(repo, upgrade.Unified))
		}
		if len(home) > 0 {
			fmt.Fprintf(w, "# Edits in your home directory (%d), confirmed separately\n%s", len(home), hosts.Render(home, upgrade.Unified))
		}
		for _, n := range res.Notes {
			fmt.Fprintln(w, "note:", n)
		}
		if !apply && len(res.Edits) > 0 {
			fmt.Fprintln(w, "install: showing the plan only; aayu install --apply writes it")
		}
		return nil
	}
	if !apply {
		return e.Emit(rep, text)
	}
	if !e.JSON {
		_ = text(e.Out)
	}
	if err := hosts.Apply(o, repo, false); err != nil {
		return err
	}
	if len(home) > 0 {
		if e.confirm(fmt.Sprintf("Apply the %d edit(s) in your home directory listed above? [y/N] ", len(home))) {
			if err := hosts.Apply(o, home, true); err != nil {
				return err
			}
			rep.Home = "applied"
		} else {
			rep.Home = "skipped"
		}
	}
	if e.JSON {
		if err := e.Emit(rep, nil); err != nil {
			return err
		}
	} else {
		fmt.Fprintf(e.Out, "install: applied %d repository edit(s)\n", len(repo))
	}
	if rep.Home == "skipped" {
		next := "aayu install --apply --yes"
		if e.Host != "" {
			next = "aayu install --host " + e.Host + " --apply --yes"
		}
		return WithNext(Errorf(ExitBlocked, "install: %d home-directory edit(s) not applied; confirm on a terminal or pass --yes", len(home)), next)
	}
	return nil
}

// confirm asks a yes/no question. --yes answers for a human (agents cannot pass
// it); otherwise only an interactive terminal can answer.
func (e *Env) confirm(question string) bool {
	if e.Yes {
		return true
	}
	if !e.isTerminal(e.In) {
		// Off a terminal, a person's device-flow approval of this command
		// (presence.go) is its yes: nobody is there to answer the prompt.
		return e.approval != nil
	}
	fmt.Fprint(e.Err, question)
	line, _ := bufio.NewReader(e.In).ReadString('\n')
	ans := strings.ToLower(strings.TrimSpace(line))
	return ans == "y" || ans == "yes"
}

// runHook is the single entry point for host hooks (`aayu hook <name> --host <host>`).
// Exit code 2 blocks the host's action, per the Claude Code, Cursor, and Amp
// plugin hook protocols.
func runHook(ctx context.Context, e *Env, in Values) error {
	name := in.String("name")
	if loop, ok := isLoopHook(name); ok {
		return runLoopGuard(ctx, e, loop)
	}
	known := false
	for _, h := range hosts.Hooks {
		known = known || h.Name == name
	}
	if !known {
		return Errorf(ExitUsage, "hook %q: unknown hook", name)
	}
	if name == "status-line" {
		_, err := fmt.Fprintln(e.Out, e.statusLine())
		return err
	}
	var event []byte // hosts send the event as JSON on stdin (the Amp plugin as --event); pre-tool reads the tool call in it
	if ev := in.String("event"); ev != "" {
		event = []byte(ev)
	} else if !e.isTerminal(e.In) {
		event, _ = io.ReadAll(io.LimitReader(e.In, 1<<20))
	}
	root := config.RepoRoot(e.Dir)
	var r hosts.HookResult
	// Cursor also runs Claude Code's hooks, the repository's and the plugin's
	// (Third-Party Imports, on by default). Such a hook answers in Cursor's
	// format, and stands aside where Cursor runs another aayudha hook for the
	// event, so each event is checked once (D-3.2b.j).
	standAside := false
	if e.Host != string(verbs.HostCursor) && hosts.CursorEvent(event) {
		standAside = root != "" && hosts.CursorRunsAnother(root, e.homeDir(), in.String("skills") != "")
		e.Host = string(verbs.HostCursor)
	}
	if e.Host == "" { // a marketplace plugin shared by several hosts names none
		e.Host, _ = doctor.DetectHost("", e.Getenv)
	}
	if standAside {
		r = hosts.Deny(verbs.HostCursor, "")
		if name != "pre-tool" {
			r.Stdout = "{}\n"
		}
	} else if in.String("skills") != "" && root != "" && hosts.RepoHooksRun(root, verbs.HostID(e.Host), hosts.CodexHome(e.homeDir(), e.Getenv)) {
		standAside = true
		// A marketplace plugin's hook (it passes --skills) stands aside where
		// the host runs the hooks `aayu install` registered for it, so each
		// runs once (D-3.2b.d). install turns the plugin off in such a
		// repository (D-3.2b.h), so this happens only when something turned it
		// back on.
		switch {
		case name == "pre-tool":
			r = hosts.Deny(verbs.HostID(e.Host), "")
		case name == "post-tool" && e.Host != string(verbs.HostCursor):
			// silent: the repository's own post-tool hook logs the call
		case e.Host == string(verbs.HostCursor):
			r.Stdout = "{}\n"
		default:
			r.Stdout = fmt.Sprintf("aayudha: the aayudha plugin is on here although `aayu install` set this repository up for %s, so the host lists each aayudha skill twice; the repository's hooks run. A person runs `aayu install --host %s --apply`, which turns the plugin off for this repository, or removes the override that turned it back on.\n", e.Host, e.Host)
		}
	} else {
		var err error
		var notes []string
		if name == "session-start" {
			notes = e.sessionNotes()
		}
		if r, err = hosts.RunHook(name, verbs.HostID(e.Host), root, e.PauseFile(), event, notes...); err != nil {
			return Wrap(ExitUsage, err)
		}
	}
	if name == "pre-tool" && !standAside { // the hook that runs refuses a halt, once
		if reason := e.haltCached(ctx); reason != "" {
			r = hosts.Deny(verbs.HostID(e.Host), fmt.Sprintf("aayudha: the repository is halted (%s): %s. A human clears it with `aayu resume`.", HaltVariable, reason))
			r.Next = "a person clears the halt with `aayu resume`"
		}
	}
	// A host's own PostHog or Sentry MCP server or CLI may only read (Decision
	// #449): a write is a catalog action a person approves.
	if name == "pre-tool" && !standAside && r.Code == 0 {
		if ev, ok := hostcall.FromHook(event); ok {
			if v := hostcall.Decide(ev, e.hostcallOptions(ev)); !v.Allow {
				r = hosts.Deny(verbs.HostID(e.Host), v.Reason)
				r.Next = "read instead, or propose the write with the verb that owns it"
			}
		}
	}
	if name == "post-tool" && !standAside {
		e.logHostCall(event)
	}
	// A marketplace plugin calls aayu from PATH and passes the skills version it
	// carries; session-start says so when this binary carries another.
	if v := in.String("skills"); name == "session-start" && v != "" && v != verbs.SkillVersion && e.Host != string(verbs.HostCursor) {
		r.Stdout += fmt.Sprintf("aayudha: the %s plugin carries skills %s, but the aayu on PATH carries %s; update one of them (`aayu self-update`, or update the plugin).\n", e.Host, v, verbs.SkillVersion)
	}
	fmt.Fprint(e.Out, r.Stdout)
	if r.Code != 0 {
		return &Error{Code: r.Code, Err: errors.New(strings.TrimSpace(r.Stderr)), Next: r.Next}
	}
	return nil
}

// hostcallOptions are the repository's choices for host PostHog and Sentry
// calls, and the PostHog and Sentry MCP servers the hosts registered under
// any name. Without a readable config every opt-in stays off.
func (e *Env) hostcallOptions(ev hostcall.Event) hostcall.Options {
	o := hostcall.Options{AllowPII: map[string]bool{}, Servers: map[string]string{}}
	if strings.HasPrefix(ev.Tool, "mcp__") { // the host configs are read only for an MCP call
		root, home := config.RepoRoot(e.Dir), e.homeDir()
		for _, s := range hosts.ProviderServers(root, home, hosts.CodexHome(home, e.Getenv)) {
			o.Servers[s.Name] = hostcall.ProviderOf(strings.Join(append([]string{s.Name, s.URL, s.Command}, s.Args...), " "))
		}
	}
	if l, err := e.Config(); err == nil {
		for _, p := range []string{hostcall.PostHog, hostcall.Sentry} {
			o.AllowPII[p] = l.Config.Integrations[p].AllowPII
		}
	}
	return o
}

// logHostCall appends a PostHog or Sentry call the host ran to the session's
// call log (.aayudha/state/calls). A failure to log is reported on stderr and
// never fails the hook: the call has already run.
func (e *Env) logHostCall(event []byte) {
	ev, res, session, ok := hostcall.FromPostHook(event)
	if !ok {
		return
	}
	v := hostcall.Decide(ev, e.hostcallOptions(ev))
	if v.Provider == "" || !v.Allow {
		return
	}
	en := hostcall.NewEntry(ev, v, res, e.Host, session, e.Now())
	if _, err := hostcall.Append(filepath.Join(e.stateDir(), "state", "calls"), en); err != nil {
		fmt.Fprintf(e.Err, "aayudha: could not log the %s call: %v\n", v.Provider, err)
	}
}

// statusLine is one line of journey state from the last home screen, for a
// host's status line. It never touches the network: hosts refresh it often.
func (e *Env) statusLine() string {
	b, err := os.ReadFile(e.homeCacheFile())
	var h Home
	if err != nil || json.Unmarshal(b, &h) != nil {
		return "aayudha | run aayu for the home screen"
	}
	parts := []string{"aayudha"}
	for _, c := range []struct {
		label string
		n     int
	}{{"Ready", len(h.Ready)}, {"building", len(h.InProgress)}, {"waiting for you", len(h.AwaitingHuman)}, {"blocked", len(h.Blocked)}, {"floor misses", len(h.FailingFloors)},
		{"Feedback to classify", len(h.Unclassified)}, {"inbox", len(h.Inbox)}} {
		if c.n > 0 {
			parts = append(parts, fmt.Sprintf("%s %d", c.label, c.n))
		}
	}
	parts = append(parts, "next: "+h.Next.Command)
	if at, err := time.Parse(time.RFC3339, h.At); err == nil {
		if age := e.Now().Sub(at); age > 10*time.Minute {
			parts = append(parts, fmt.Sprintf("as of %s ago", age.Truncate(time.Minute)))
		}
	}
	return strings.Join(parts, " | ")
}

// sessionNotes is what a session starts with from the last home screen,
// without touching the network: the open Feedback that has no fix layer
// (D-7.7e), with the home screen's age when it is older than ten minutes.
func (e *Env) sessionNotes() []string {
	b, err := os.ReadFile(e.homeCacheFile())
	var h Home
	if err != nil || json.Unmarshal(b, &h) != nil {
		return nil
	}
	note := journey.UnclassifiedNote(h.Unclassified)
	if note == "" {
		return nil
	}
	if at, err := time.Parse(time.RFC3339, h.At); err == nil {
		if age := e.Now().Sub(at); age > 10*time.Minute {
			note += fmt.Sprintf(" (as of %s ago; `aayu` refreshes it)", age.Truncate(time.Minute))
		}
	}
	return []string{"aayudha: " + note}
}

// reviewerAppsNote reports, best effort, the agent apps that can review this
// repository's PRs in the merge gate (founder decision 2026-10-15), so the
// person installing knows what to put in [inspect] apps. Nothing without a
// token or a repository.
func (e *Env) reviewerAppsNote(ctx context.Context) []string {
	tok, err := e.Resolver().Human(ctx)
	if err != nil {
		return nil
	}
	repo, err := e.RepoName(ctx)
	if err != nil {
		return nil
	}
	found := inspect.DetectApps(ctx, func(ctx context.Context, path string, out any) error { return e.githubGet(ctx, tok, path, out) }, repo)
	var names []string
	for _, d := range found {
		names = append(names, d.App+" ("+d.Source+")")
	}
	if len(names) == 0 {
		return []string{"reviewer apps: none detected on " + repo + "; the merge gate needs one installed on the org or account (Claude, Codex, or Cursor) named in [inspect] apps, else it asks a human"}
	}
	return []string{"reviewer apps detected on " + repo + ": " + strings.Join(names, ", ") + "; name them in [inspect] apps for the merge gate (`aayu doctor` shows the choice)"}
}
