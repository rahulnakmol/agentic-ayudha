// Package ui is how aayu talks to a person on a terminal (WP1.1b): colour,
// themes, symbols, tables, live progress, celebrations, and the seat that
// speaks for a command. It never changes what a machine reads: output for
// --json, MCP, agents (AAYU_AGENT=1), CI, and anything that is not a terminal
// is the Machine level, which adds nothing to a command's stable text.
//
// Meaning never rides on colour or a symbol alone: every status keeps its word
// (PASS, WARN, FAIL), and the Plain level (--plain, [ui] plain = true, or
// TERM=dumb) drops colour, motion, and non-ASCII symbols for screen readers.
package ui

import (
	"fmt"
	"io"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
)

// Level is how much presentation a stream gets.
type Level int

// Levels, from the most conservative.
const (
	Machine Level = iota // not a person on a terminal: the stable text only
	Plain                // a person who wants words only: no colour, motion, or non-ASCII symbols
	Rich                 // a person on a terminal: colour (unless NO_COLOR), Unicode, motion
)

func (l Level) String() string {
	return [...]string{"machine", "plain", "rich"}[l]
}

// Signals are what decide a stream's Level.
type Signals struct {
	TTY     bool   // the stream is a terminal
	JSON    bool   // --json
	Agent   bool   // AAYU_AGENT=1 (agent lanes and the MCP server)
	CI      bool   // --ci, or the CI environment variable
	Plain   bool   // --plain or [ui] plain = true
	NoColor bool   // NO_COLOR is set to a non-empty value (no-color.org)
	Term    string // TERM
	Theme   string // [ui] theme
}

// LevelFor resolves the level of one stream.
func LevelFor(s Signals) Level {
	switch {
	case s.JSON || s.Agent || s.CI || !s.TTY:
		return Machine
	case s.Plain || s.Term == "dumb":
		return Plain
	}
	return Rich
}

// Role is what a piece of text is, which the theme colours.
type Role int

// Roles.
const (
	Title Role = iota
	Accent
	OK
	Warn
	Fail
	Muted
	Command
	Seat
)

// Theme maps roles to styles. Every theme keeps text readable without its
// colours; the colours only repeat what the words say.
type Theme struct {
	Name   string
	Styles map[Role]lipgloss.Style
}

// Themes are the [ui] theme values. "auto" uses the terminal's own 16-colour
// palette, which follows the user's light or dark terminal theme.
var Themes = []string{"auto", "saffron", "contrast", "mono"}

// ThemeNamed returns a theme; an unknown name is "auto".
func ThemeNamed(name string) Theme {
	s := lipgloss.NewStyle
	switch name {
	case "saffron": // warm true-colour palette for dark terminals
		return Theme{"saffron", map[Role]lipgloss.Style{
			Title: s().Bold(true).Foreground(lipgloss.Color("#FF9933")), Accent: s().Foreground(lipgloss.Color("#FFB86B")),
			OK: s().Foreground(lipgloss.Color("#7FD99A")), Warn: s().Foreground(lipgloss.Color("#F5D76E")), Fail: s().Bold(true).Foreground(lipgloss.Color("#FF6B6B")),
			Muted: s().Foreground(lipgloss.Color("#9AA3B2")), Command: s().Foreground(lipgloss.Color("#6FD3E8")), Seat: s().Italic(true).Foreground(lipgloss.Color("#D7A8FF")),
		}}
	case "contrast": // bright basic colours and bold for low vision
		return Theme{"contrast", map[Role]lipgloss.Style{
			Title: s().Bold(true).Underline(true), Accent: s().Bold(true).Foreground(lipgloss.BrightWhite),
			OK: s().Bold(true).Foreground(lipgloss.BrightGreen), Warn: s().Bold(true).Foreground(lipgloss.BrightYellow), Fail: s().Bold(true).Foreground(lipgloss.BrightRed),
			Muted: s(), Command: s().Bold(true).Foreground(lipgloss.BrightCyan), Seat: s().Bold(true).Foreground(lipgloss.BrightMagenta),
		}}
	case "mono": // no hue at all: weight and underline only
		return Theme{"mono", map[Role]lipgloss.Style{
			Title: s().Bold(true), Accent: s().Bold(true), OK: s(), Warn: s().Bold(true), Fail: s().Bold(true).Underline(true),
			Muted: s().Faint(true), Command: s().Underline(true), Seat: s().Italic(true),
		}}
	}
	return Theme{"auto", map[Role]lipgloss.Style{
		Title: s().Bold(true).Foreground(lipgloss.Magenta), Accent: s().Foreground(lipgloss.Magenta),
		OK: s().Foreground(lipgloss.Green), Warn: s().Foreground(lipgloss.Yellow), Fail: s().Bold(true).Foreground(lipgloss.Red),
		Muted: s().Faint(true), Command: s().Foreground(lipgloss.Cyan), Seat: s().Italic(true).Foreground(lipgloss.Magenta),
	}}
}

// Printer writes to one stream at one level.
type Printer struct {
	W     io.Writer
	Level Level
	Color bool // Rich and colour allowed (NO_COLOR unset)
	Theme Theme
	// Static renders motion as its final frame only (tests, recordings that
	// must be byte-stable).
	Static bool
	Now    func() time.Time
	Sleep  func(time.Duration)
	Width  int // terminal columns; 0 when unknown
}

// New returns a printer for a stream with the given signals.
func New(w io.Writer, s Signals) *Printer {
	l := LevelFor(s)
	return &Printer{W: w, Level: l, Color: l == Rich && !s.NoColor, Theme: ThemeNamed(s.Theme), Now: time.Now, Sleep: time.Sleep}
}

// Human reports whether a person reads this stream (Plain or Rich).
func (p *Printer) Human() bool { return p != nil && p.Level >= Plain }

// Style renders s in a role's style when colour is on.
func (p *Printer) Style(r Role, s string) string {
	if !p.Color {
		return s
	}
	st, ok := p.Theme.Styles[r]
	if !ok {
		return s
	}
	return st.Render(s)
}

// Sym returns the Rich symbol, or the ASCII fallback at other levels.
func (p *Printer) Sym(rich, ascii string) string {
	if p.Level == Rich {
		return rich
	}
	return ascii
}

// Status words aayu prints; each has a symbol and a role.
const (
	StatusPass = "PASS"
	StatusWarn = "WARN"
	StatusFail = "FAIL"
	StatusNA   = "N/A"
	StatusInfo = "INFO"
)

// Mark renders a status word with its symbol (Rich) and colour. The word is
// always there, so the meaning never depends on the colour or the symbol.
func (p *Printer) Mark(status string) string {
	sym, role := "•", Muted
	switch status {
	case StatusPass:
		sym, role = "✓", OK
	case StatusWarn:
		sym, role = "!", Warn
	case StatusFail:
		sym, role = "✗", Fail
	case StatusInfo:
		sym, role = "→", Accent
	}
	if p.Level != Rich {
		return status
	}
	return p.Style(role, sym+" "+status)
}

// Println writes a line.
func (p *Printer) Println(s string) { fmt.Fprintln(p.W, s) }

// Printf writes formatted text.
func (p *Printer) Printf(format string, a ...any) { fmt.Fprintf(p.W, format, a...) }

// Table renders rows under headers: a rounded table at Rich, aligned columns
// without box drawing at Plain (and Machine, for callers that want it).
func (p *Printer) Table(headers []string, rows [][]string) string {
	if p.Level != Rich {
		return alignColumns(headers, rows)
	}
	t := table.New().Border(lipgloss.RoundedBorder()).Headers(headers...).Rows(rows...)
	border := lipgloss.NewStyle()
	if p.Color {
		border = p.Theme.Styles[Muted]
	}
	t = t.BorderStyle(border).StyleFunc(func(row, _ int) lipgloss.Style {
		st := lipgloss.NewStyle().Padding(0, 1)
		if row == table.HeaderRow {
			st = st.Bold(true)
		}
		return st
	})
	if out := t.Render(); p.Width <= 0 || lipgloss.Width(out) <= p.Width {
		return out + "\n"
	}
	return t.Width(p.Width).Render() + "\n"
}

// alignColumns pads each column to its widest cell.
func alignColumns(headers []string, rows [][]string) string {
	all := append([][]string{headers}, rows...)
	widths := make([]int, len(headers))
	for _, r := range all {
		for i, c := range r {
			if i < len(widths) && lipgloss.Width(c) > widths[i] {
				widths[i] = lipgloss.Width(c)
			}
		}
	}
	var b strings.Builder
	for _, r := range all {
		var line strings.Builder
		for i, c := range r {
			if i == len(r)-1 {
				line.WriteString(c)
				break
			}
			line.WriteString(c + strings.Repeat(" ", widths[i]-lipgloss.Width(c)+2))
		}
		b.WriteString(strings.TrimRight(line.String(), " ") + "\n")
	}
	return b.String()
}

// Voice is the seat that speaks for a command: its intro before the command's
// facts and its sign-off after them (persona data from the seat charter).
type Voice struct {
	Seat    string // display name, e.g. "Engineering Manager"
	Name    string // the twin's short name, when the charter gives one
	Intro   string
	SignOff string
}

// Speaker is how the voice is introduced: "Name (Seat)" or the seat alone.
func (v Voice) Speaker() string {
	if v.Name != "" {
		return v.Name + " (" + v.Seat + ")"
	}
	return v.Seat
}

// Intro writes the voice's one-line intro (humans only).
func (p *Printer) Intro(v Voice) {
	if !p.Human() || v.Seat == "" || v.Intro == "" {
		return
	}
	p.Println(p.Style(Seat, v.Speaker()+": "+v.Intro))
}

// SignOff writes the voice's sign-off (humans only).
func (p *Printer) SignOff(v Voice) {
	if !p.Human() || v.Seat == "" || v.SignOff == "" {
		return
	}
	p.Println(p.Style(Seat, p.Sym("— ", "-- ")+v.Speaker()+": "+v.SignOff))
}

// Celebrate marks a milestone: a short sparkle at Rich, one plain line at
// Plain, nothing for machines.
func (p *Printer) Celebrate(title, detail string) {
	if !p.Human() {
		return
	}
	line := title
	if detail != "" {
		line += " " + detail
	}
	if p.Level == Plain {
		p.Println("Milestone: " + line)
		return
	}
	if !p.Static {
		frames := []string{"·", "✦", "✧ ✦", "✦ ✧ ✦", "✧ ✦ ✧ ✦"}
		for _, f := range frames {
			fmt.Fprintf(p.W, "\r\x1b[K%s", p.Style(Accent, f))
			p.Sleep(90 * time.Millisecond)
		}
		fmt.Fprint(p.W, "\r\x1b[K")
	}
	p.Println(p.Style(Title, "★ "+title) + " " + detail)
}
