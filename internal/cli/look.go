package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"

	"github.com/rahulnakmol/agentic-ayudha/internal/ui"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

// This file is the CLI's side of WP1.1b: which level each stream gets, the seat
// that speaks for a command, milestones, and the next command after an error.

// Look is the printer for the result stream (stdout).
func (e *Env) Look() *ui.Printer {
	if e.out == nil {
		e.out = e.printer(e.Out)
	}
	return e.out
}

// Talk is the printer for the conversation stream (stderr): intros and
// sign-offs, live progress, and celebrations. It never carries a result.
func (e *Env) Talk() *ui.Printer {
	if e.talk == nil {
		e.talk = e.printer(e.Err)
	}
	return e.talk
}

func (e *Env) printer(stream any) *ui.Printer {
	s := ui.Signals{TTY: e.isTerminal(stream), JSON: e.JSON, Agent: e.Agent(), CI: e.CI || e.Getenv("CI") != "",
		Plain: e.Plain || e.Getenv("AAYU_PLAIN") == "1", NoColor: e.Getenv("NO_COLOR") != "", Term: e.Getenv("TERM")}
	if s.TTY && !s.JSON && !s.Agent { // [ui] only matters to a person; a broken config is reported by the command itself
		if l, err := e.Config(); err == nil {
			s.Plain = s.Plain || l.Config.UI.Plain
			s.Theme = l.Config.UI.Theme
		}
	}
	w, _ := stream.(interface{ Write([]byte) (int, error) })
	p := ui.New(w, s)
	p.Static, p.Now = e.UIStatic, e.Now
	if f, ok := stream.(*os.File); ok && p.Level == ui.Rich {
		if w, _, err := term.GetSize(f.Fd()); err == nil {
			p.Width = w
		}
	}
	return p
}

func (e *Env) isTerminal(stream any) bool {
	if e.IsTerminal != nil {
		return e.IsTerminal(stream)
	}
	f, ok := stream.(*os.File)
	return ok && term.IsTerminal(f.Fd())
}

// voice is the seat's persona for the CLI, read from its charter (seats/<seat>/
// CHARTER.md, owned by the persona work): the `## Persona` section's intro and
// sign-off lines (labels Intro or Purpose, Sign-off, and Name or Twin name),
// else the first clause of the charter's `## Purpose`. Nothing is copied here.
func (e *Env) voice(seat verbs.SeatID) ui.Voice {
	v := ui.Voice{Seat: verbs.SeatName(seat)}
	if e.Assets == nil {
		return v
	}
	b, err := fs.ReadFile(e.Assets, path.Join("seats", string(seat), "CHARTER.md"))
	if err != nil {
		return v
	}
	persona := mdSection(string(b), "persona")
	v.Name = field(persona, "twin name", "twin", "name")
	v.Intro = field(persona, "intro", "introduction", "greeting", "purpose")
	v.SignOff = field(persona, "sign-off", "signoff", "sign off", "closing")
	if v.Intro == "" {
		purpose := firstLine(mdSection(string(b), "purpose"))
		if i := strings.IndexAny(purpose, ":."); i > 0 {
			purpose = purpose[:i]
		}
		if purpose != "" {
			v.Intro = strings.ToLower(purpose[:1]) + purpose[1:] + "."
		}
	}
	return v
}

// mdSection returns the body of the first level-2 heading that starts with name.
func mdSection(md, name string) string {
	var b strings.Builder
	in := false
	for _, line := range strings.Split(md, "\n") {
		if strings.HasPrefix(line, "## ") || strings.HasPrefix(line, "<!-- BEGIN GENERATED") {
			if in {
				break
			}
			in = strings.HasPrefix(strings.ToLower(strings.TrimSpace(line[2:])), name)
			continue
		}
		if in {
			b.WriteString(line + "\n")
		}
	}
	return b.String()
}

var labelLine = regexp.MustCompile(`^\s*(?:[-*]\s+)?(?:\*\*([^*]+?)\*\*|([A-Za-z][A-Za-z -]*?)):?\s*[:.—-]?\s*(.*)$`)

// field finds a labelled one-line value in a persona section: "**Label:** text",
// "- **Label**: text", "Label: text", or a "### Label" heading followed by text.
func field(sec string, labels ...string) string {
	lines := strings.Split(sec, "\n")
	match := func(l string) bool {
		l = strings.ToLower(strings.Trim(strings.TrimSpace(l), ":. "))
		for _, want := range labels {
			if l == want {
				return true
			}
		}
		return false
	}
	for i, line := range lines {
		if h, ok := strings.CutPrefix(strings.TrimSpace(line), "### "); ok && match(h) {
			return firstLine(strings.Join(lines[i+1:], "\n"))
		}
		m := labelLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		label := m[1]
		if label == "" {
			label = m[2]
			if !strings.Contains(line, ":") {
				continue
			}
		}
		if match(label) && strings.TrimSpace(m[3]) != "" {
			return strings.Trim(strings.TrimSpace(m[3]), `"“”`)
		}
	}
	return ""
}

func firstLine(s string) string {
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "#") {
			return l
		}
	}
	return ""
}

// milestones is <state>/state/milestones.json: the first time each milestone
// happened in this checkout, so a celebration plays once.
func (e *Env) milestonesPath() string {
	return filepath.Join(e.stateDir(), "state", "milestones.json")
}

// milestone celebrates a first-time milestone (once per checkout) on a
// person's terminal. once false celebrates every time (a release).
func (e *Env) milestone(key string, once bool, title, detail string) {
	t := e.Talk()
	if !t.Human() || e.DryRun {
		return
	}
	if once {
		seen := map[string]string{}
		b, err := os.ReadFile(e.milestonesPath())
		if err == nil {
			_ = json.Unmarshal(b, &seen)
		}
		if seen[key] != "" {
			return
		}
		seen[key] = e.Now().UTC().Format(time.RFC3339)
		if b, err := json.MarshalIndent(seen, "", "  "); err == nil {
			_ = os.MkdirAll(filepath.Dir(e.milestonesPath()), 0o755)
			_ = os.WriteFile(e.milestonesPath(), append(b, '\n'), 0o644)
		}
	}
	t.Celebrate(title, detail)
}

// progress starts a live step on the conversation stream.
func (e *Env) progress(label string) *ui.Progress { return e.Talk().Start(label) }

var backticked = regexp.MustCompile("`((?:aayu|gh|git) [^`]+)`")

// nextFor is the exact command to run after a failed command: one the error
// already names, else one for its exit code. Every error ends with it.
func nextFor(cmd *cobra.Command, args []string, err error, code int) string {
	var ce *Error
	if errors.As(err, &ce) && ce.Next != "" {
		return ce.Next
	}
	msg := err.Error()
	if m := backticked.FindAllStringSubmatch(msg, -1); m != nil {
		return m[0][1]
	}
	name := "aayu"
	if cmd != nil && cmd.Name() != "aayu" {
		name = "aayu " + cmd.Name()
	}
	switch {
	case strings.Contains(msg, "unknown command"):
		if cmd != nil {
			if s := cmd.Root().SuggestionsFor(firstArg(args)); len(s) > 0 {
				return "aayu " + s[0] + " --help"
			}
		}
		return "aayu --help"
	case strings.Contains(msg, "no GitHub token") || strings.Contains(msg, "gh auth"):
		return "gh auth login"
	case strings.Contains(msg, "halted"):
		return "aayu resume"
	}
	switch code {
	case ExitUsage:
		return name + " --help"
	case ExitFloor:
		return "aayu verify"
	case ExitBlocked:
		return "aayu next"
	case ExitExternal:
		return "aayu doctor"
	case ExitBudget:
		return "aayu status"
	case ExitInternal:
		return "gh issue create --repo rahulnakmol/agentic-ayudha --title " + fmt.Sprintf("%q", name+": internal error")
	}
	return "aayu next"
}

func firstArg(args []string) string {
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			return a
		}
	}
	return ""
}
