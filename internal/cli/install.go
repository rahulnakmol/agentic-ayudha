package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/hosts"
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
	apply := in.Bool("apply")
	if apply && e.Agent() {
		return Errorf(ExitBlocked, "install --apply writes the hooks that govern agents; a human runs it (AAYU_AGENT=1)")
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
		return Errorf(ExitBlocked, "install: %d home-directory edit(s) not applied; confirm on a terminal or pass --yes", len(home))
	}
	return nil
}

// confirm asks a yes/no question. --yes answers for a human (agents cannot pass
// it); otherwise only an interactive terminal can answer.
func (e *Env) confirm(question string) bool {
	if e.Yes {
		return true
	}
	f, ok := e.In.(*os.File)
	if !ok {
		return false
	}
	if fi, err := f.Stat(); err != nil || fi.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	fmt.Fprint(e.Err, question)
	line, _ := bufio.NewReader(f).ReadString('\n')
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
	if f, ok := e.In.(*os.File); ok { // hosts send the event as JSON on stdin; it is not needed
		if fi, err := f.Stat(); err == nil && fi.Mode()&os.ModeCharDevice == 0 {
			_, _ = io.Copy(io.Discard, io.LimitReader(f, 1<<20))
		}
	}
	r, err := hosts.RunHook(name, verbs.HostID(e.Host), config.RepoRoot(e.Dir), e.PauseFile())
	if err != nil {
		return Wrap(ExitUsage, err)
	}
	if name == "pre-tool" {
		if reason := e.haltCached(ctx); reason != "" {
			r = hosts.Deny(verbs.HostID(e.Host), fmt.Sprintf("aayudha: the repository is halted (%s): %s. A human clears it with `aayu resume`.", HaltVariable, reason))
		}
	}
	fmt.Fprint(e.Out, r.Stdout)
	if r.Code != 0 {
		return &Error{Code: r.Code, Err: errors.New(strings.TrimSpace(r.Stderr))}
	}
	return nil
}
