package cli

import (
	"bufio"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"

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
	if err := e.chooseReviewTool(ctx); err != nil {
		return err
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
			if v := e.hostcallDecide(ctx, ev); !v.Allow {
				r = hosts.Deny(verbs.HostID(e.Host), v.Reason)
				r.Next = "read instead, or propose the write with the verb that owns it"
			}
		}
	}
	// Only a call every check let through: a denied build is never credited
	// to the Copilot session that asked for it.
	if name == "pre-tool" && !standAside && r.Code == 0 && e.Host == string(verbs.HostCopilot) {
		hosts.RecordCopilotSession(root, event)
	}
	if name == "post-tool" && !standAside {
		e.logHostCall(ctx, event)
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

// hostcallDecide decides a host's PostHog or Sentry call (Decision #449).
// The hosts' MCP configs name the servers registered under any name; they are
// read only for an MCP call (mcp__<server>__<tool>, or Copilot's
// <server>-<tool>). allow_pii is read only when the call would be
// refused without it.
func (e *Env) hostcallDecide(ctx context.Context, ev hostcall.Event) hostcall.Verdict {
	var o hostcall.Options
	if strings.HasPrefix(ev.Tool, "mcp__") || ev.Dashed {
		o.Servers = map[string]string{}
		root, home := config.RepoRoot(e.Dir), e.homeDir()
		for _, s := range hosts.ProviderServers(root, home, hosts.CodexHome(home, e.Getenv), hosts.CopilotHome(home, e.Getenv)) {
			o.Servers[s.Name] = hostcall.ProviderOf(strings.Join(append([]string{s.Name, s.URL, s.Command}, s.Args...), " "))
		}
	}
	v := hostcall.Decide(ev, o)
	if v.Provider != "" && !v.Allow {
		o.AllowPII = e.committedAllowPII(ctx)
		v = hostcall.Decide(ev, o)
	}
	return v
}

// committedAllowPII reads [integrations.<provider>] allow_pii from
// .aayudha/config.toml as committed where this branch left the dev branch, or
// at HEAD when there is no dev branch to compare with; never from the working
// tree, so an agent that edits the file, or commits it on its own branch,
// cannot opt itself in to personal data. Unreadable, every opt-in stays off.
func (e *Env) committedAllowPII(ctx context.Context) map[string]bool {
	root, git := config.RepoRoot(e.Dir), e.exec()
	ref := "HEAD"
	if mb, err := git(ctx, root, "git", "merge-base", "HEAD", "origin/"+e.devBranch()); err == nil && strings.TrimSpace(mb) != "" {
		ref = strings.TrimSpace(mb)
	}
	out := map[string]bool{}
	var cfg struct {
		Integrations map[string]struct {
			AllowPII bool `toml:"allow_pii"`
		} `toml:"integrations"`
	}
	if b, err := git(ctx, root, "git", "show", ref+":.aayudha/config.toml"); err == nil && toml.Unmarshal([]byte(b), &cfg) == nil {
		for p, i := range cfg.Integrations {
			out[p] = i.AllowPII
		}
	}
	return out
}

// logHostCall appends a PostHog or Sentry call the host ran to the session's
// call log (.aayudha/state/calls). A failure to log is reported on stderr and
// never fails the hook: the call has already run.
func (e *Env) logHostCall(ctx context.Context, event []byte) {
	ev, res, session, ok := hostcall.FromPostHook(event)
	if !ok {
		return
	}
	v := e.hostcallDecide(ctx, ev)
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

// ReviewTools are the values of [inspect] reviewer a person picks from
// (founder, 2026-10-18: "People need to choose one code review tool"; "auto
// cannot be an option").
var ReviewTools = []string{inspect.ReviewerCI, inspect.AppCodex, inspect.AppCodeRabbit, inspect.AppGreptile, inspect.AppClaude, inspect.AppCursor, inspect.ReviewerHuman}

// chooseReviewTool makes install --apply require the merge gate's one review
// tool: when [inspect] reviewer is unset (or a leftover "auto"), it asks on a
// terminal, listing the apps detected on the repository, and writes the
// answer to .aayudha/config.toml; with no terminal or no valid answer, install
// stops and says how to choose.
func (e *Env) chooseReviewTool(ctx context.Context) error {
	root := config.RepoRoot(e.Dir)
	stop := WithNext(Errorf(ExitUsage, `install: choose the merge gate's review tool: set [inspect] reviewer = "<tool>" (%s) in .aayudha/config.toml, or answer on a terminal`, strings.Join(ReviewTools, ", ")), "aayu install --apply")
	if root == "" {
		return stop
	}
	// The repository's own file decides, as the merge gate reads it: a
	// reviewer set only in a personal layer does not count.
	path := filepath.Join(root, ".aayudha", "config.toml")
	b, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	var v struct {
		Inspect struct {
			Reviewer string `toml:"reviewer"`
		} `toml:"inspect"`
	}
	if toml.Unmarshal(b, &v) == nil && v.Inspect.Reviewer != "" && v.Inspect.Reviewer != "auto" {
		return nil
	}
	if !e.isTerminal(e.In) {
		return stop
	}
	var found []string
	if tok, err := e.Resolver().Human(ctx); err == nil {
		if repo, err := e.RepoName(ctx); err == nil {
			for _, d := range inspect.DetectApps(ctx, func(ctx context.Context, path string, out any) error { return e.githubGet(ctx, tok, path, out) }, repo) {
				found = append(found, d.App)
			}
		}
	}
	fmt.Fprintf(e.Err, "Which tool reviews this repository's pull requests? Detected: %s. [%s] (Enter: ci, the model the code-review environment names): ", cmp.Or(strings.Join(found, ", "), "none"), strings.Join(ReviewTools, "/"))
	line, _ := bufio.NewReader(e.In).ReadString('\n')
	ans := cmp.Or(strings.ToLower(strings.TrimSpace(line)), inspect.ReviewerCI)
	if !slices.Contains(ReviewTools, ans) {
		return stop
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	// Write only a file that still parses and names the tool: a dotted
	// inspect.reviewer key or an unusual layout stops install instead.
	out := setInspectReviewer(string(b), ans)
	v.Inspect.Reviewer = ""
	if toml.Unmarshal([]byte(out), &v) != nil || v.Inspect.Reviewer != ans {
		return stop
	}
	return os.WriteFile(path, []byte(out), 0o644)
}

var (
	inspectHeader   = regexp.MustCompile(`(?m)^[ \t]*\[inspect\][ \t]*(#.*)?$`)
	sectionHeader   = regexp.MustCompile(`(?m)^[ \t]*\[`)
	reviewerSetting = regexp.MustCompile(`(?m)^[ \t]*reviewer[ \t]*=.*$`)
)

// setInspectReviewer sets reviewer in the [inspect] section of a config
// file's text, adding the line or the section as needed.
func setInspectReviewer(text, tool string) string {
	line := fmt.Sprintf("reviewer = %q", tool)
	loc := inspectHeader.FindStringIndex(text)
	if loc == nil {
		if text != "" && !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		return text + "\n[inspect]\n" + line + "\n"
	}
	body := text[loc[1]:]
	end := len(body)
	if n := sectionHeader.FindStringIndex(body); n != nil {
		end = n[0]
	}
	if r := reviewerSetting.FindStringIndex(body[:end]); r != nil {
		return text[:loc[1]] + body[:r[0]] + line + body[r[1]:]
	}
	return text[:loc[1]] + "\n" + line + body
}
