package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/doctor"
	"github.com/rahulnakmol/agentic-ayudha/internal/hosts"
	"github.com/rahulnakmol/agentic-ayudha/internal/ui"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

func init() { register("tour", runTour) }

// runTour is the guided first run (WP1.1b): check the machine, then reach
// Level 1 (docs/guide/adopt.md) with the real commands, init and install, each
// of which still asks its own confirmations. It writes nothing itself.
func runTour(ctx context.Context, e *Env, _ Values) error {
	if sig := e.agentSignal(); sig != "" {
		return WithNext(Errorf(ExitBlocked, "tour sets a repository up for a person; agents are refused (%s)", sig), "aayu next")
	}
	if !e.isTerminal(e.In) {
		return WithNext(Errorf(ExitBlocked, "tour asks a few questions, so it needs a terminal; without one, run init and install yourself"), "aayu init")
	}
	root := config.RepoRoot(e.Dir)
	if root == "" {
		return WithNext(Errorf(ExitUsage, "tour sets up a git repository, and this directory is not in one"), "git init")
	}
	p := e.Look()
	p.Println(p.Style(ui.Title, "Welcome to aayudha.") + " Three steps take this repository to Level 1: clear work items.")
	p.Println(p.Style(ui.Muted, "Level 1 adds labels, issue forms, a config file, AGENTS.md, and a records branch. No workflows, no secrets, no CI minutes."))
	p.Println("")

	// 1. The machine.
	p.Println(p.Style(ui.Accent, "Step 1 of 3: this machine"))
	pr := e.progress("Checking git, git-lfs, your GitHub sign-in, and the config")
	results := doctor.Run(ctx, e.probe(ctx))
	var problems []string
	for _, r := range results {
		if r.Status == doctor.Fail || (r.Status == doctor.Warn && r.ID == "token") {
			problems = append(problems, fmt.Sprintf("%s %s: %s", p.Mark(strings.ToUpper(string(r.Status))), r.ID, r.Detail))
		}
	}
	if doctor.Blocking(results) {
		pr.Done(ui.StatusFail, "something this repository needs is missing")
		for _, l := range problems {
			p.Println("  " + l)
		}
		return WithNext(Errorf(ExitFloor, "tour: fix the failures above first"), "aayu doctor")
	}
	pr.Done(ui.StatusPass, "ready")
	for _, l := range problems {
		p.Println("  " + l)
	}

	// 2. The host.
	host, _ := doctor.DetectHost(e.Host, e.Getenv)
	var names []string
	for _, h := range verbs.Hosts {
		names = append(names, string(h.ID))
	}
	if host == "" || host == "all" {
		host = "claude"
	}
	ans := strings.ToLower(e.ask(fmt.Sprintf("Which host do you work in? %s [%s]: ", strings.Join(names, ", "), host)))
	if ans != "" {
		if !slices.Contains(names, ans) {
			return WithNext(Errorf(ExitUsage, "tour: unknown host %q", ans), "aayu tour")
		}
		host = ans
	}

	// 3. init, then install, each only when it is still needed.
	p.Println("")
	p.Println(p.Style(ui.Accent, "Step 2 of 3: set up the repository"))
	if _, err := os.Stat(filepath.Join(root, ".aayudha", "config.toml")); err == nil {
		p.Println(p.Mark(ui.StatusPass) + " already set up (.aayudha/config.toml exists); skipping aayu init")
	} else {
		p.Println("Running " + p.Style(ui.Command, "aayu init") + ". It shows exactly what it creates and asks before it uses your admin access.")
		if err := handlers["init"](ctx, e, Values{}); err != nil {
			return WithNext(err, "aayu init --resume")
		}
	}
	p.Println("")
	p.Println(p.Style(ui.Accent, "Step 3 of 3: skills, MCP server, and hooks for "+host))
	installed := false
	if l, ok, _ := hosts.ReadLock(root); ok && slices.Contains(l.Hosts, host) {
		installed = true
		p.Println(p.Mark(ui.StatusPass) + " " + host + " is already installed (.aayudha/lock.toml)")
	}
	if !installed {
		if yes(e.ask(fmt.Sprintf("Install the aayudha skills, MCP server, and hooks for %s? [Y/n]: ", host)), true) {
			e.Host = host
			if err := handlers["install"](ctx, e, Values{"host": host, "apply": true}); err != nil {
				return WithNext(err, "aayu install --host "+host+" --apply")
			}
		} else {
			p.Println("Skipped. Run " + p.Style(ui.Command, "aayu install --host "+host+" --apply") + " when you are ready.")
		}
	}
	p.Println("")
	e.milestone("level-1", true, "Level 1!", "This repository has clear work items. Commit what changed in a pull request.")
	p.Println(p.Style(ui.Title, "Next →") + " " + p.Style(ui.Command, "aayu grill story"))
	p.Println("  Shape your first story with numbered criteria that say what done means, then " + p.Style(ui.Command, "aayu ready <issue>") + ".")
	return nil
}

// ask writes a prompt to the conversation stream and reads one line from the
// input a byte at a time, so the next reader (init, install) loses nothing.
func (e *Env) ask(prompt string) string {
	fmt.Fprint(e.Err, prompt)
	var b []byte
	one := make([]byte, 1)
	for {
		n, err := e.In.Read(one)
		if n == 1 {
			if one[0] == '\n' {
				break
			}
			b = append(b, one[0])
		}
		if err != nil {
			if err != io.EOF {
				return ""
			}
			break
		}
	}
	return strings.TrimSpace(string(b))
}

func yes(ans string, def bool) bool {
	switch strings.ToLower(ans) {
	case "":
		return def
	case "y", "yes":
		return true
	}
	return false
}
