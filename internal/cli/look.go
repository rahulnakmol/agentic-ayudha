package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/x/term"
	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"

	"github.com/rahulnakmol/agentic-ayudha/internal/seats"
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

// voice is the seat's persona for the CLI (WP1.1b), read from its
// persona.toml (WP5.1b, the single source): the twin's name, its purpose up to
// the first colon as the intro, and one of its signature questions, picked by
// the command, as the sign-off. Without a persona, the first clause of the
// charter's purpose speaks and nothing is invented.
func (e *Env) voice(seat verbs.SeatID, command string) ui.Voice {
	v := ui.Voice{Seat: verbs.SeatName(seat)}
	if e.Assets == nil {
		return v
	}
	var p seats.Persona
	if b, err := fs.ReadFile(e.Assets, seats.PersonaPath(seat)); err == nil && toml.Unmarshal(b, &p) == nil && p.Purpose != "" {
		v.Name, v.Intro = p.Name, clause(p.Purpose)
		if len(p.Questions) > 0 {
			h := fnv.New32a()
			_, _ = h.Write([]byte(command))
			v.SignOff = p.Questions[int(h.Sum32()%uint32(len(p.Questions)))]
		}
		return v
	}
	if b, err := fs.ReadFile(e.Assets, path.Join("seats", string(seat), "CHARTER.md")); err == nil {
		v.Intro = clause(firstLine(mdSection(string(b), "purpose")))
	}
	return v
}

// clause is a sentence up to its first colon or full stop, as its own sentence.
func clause(s string) string {
	if i := strings.IndexAny(s, ":."); i > 0 {
		s = s[:i]
	}
	if s == "" {
		return ""
	}
	return s + "."
}

// mdSection returns the body of the first level-2 heading that starts with name.
func mdSection(md, name string) string {
	var b strings.Builder
	in := false
	for _, line := range strings.Split(md, "\n") {
		if strings.HasPrefix(line, "## ") {
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
