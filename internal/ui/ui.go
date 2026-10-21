// Package ui is how aayu talks to a person on a terminal (WP1.1b): colour,
// themes, symbols, tables, live progress, celebrations, and the seat that
// speaks for a command. It never changes what a machine reads: output for
// --json, MCP, agents (AAYU_AGENT=1 or a host's agent marker), CI, and anything that is not a terminal
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
	Agent   bool   // an agent lane: AAYU_AGENT=1 or a host's agent marker (hosts.AgentMarkers)
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
	// Bg and Fg are a named palette's background and text colours, which a
	// full-screen view (the inbox) paints so the palette reads as published;
	// empty for the themes that use the terminal's own colours.
	Bg, Fg string
}

// Themes are the [ui] theme values. "auto" uses the terminal's own 16-colour
// palette, which follows the terminal's theme (Nord, Catppuccin, Gruvbox, or
// any other) and its light or dark background; the named palettes are for a
// terminal that is not themed.
var Themes = []string{"auto", "saffron", "contrast", "mono",
	"nord", "catppuccin-mocha", "catppuccin-latte", "gruvbox-dark", "gruvbox-light", "dracula", "tokyonight", "solarized"}

// palette is a named theme from a project's published colours (sources and
// licences in THIRD_PARTY_NOTICES.md, "Colour palettes"). Every role keeps a
// contrast of at least 3:1 against bg, and muted text at least 4:1
// (TestPalettesAreReadable).
type palette struct {
	bg, fg, title, accent, ok, warn, fail, muted, command, seat string
}

var palettes = map[string]palette{
	// https://www.nordtheme.com/docs/colors-and-palettes
	"nord": {bg: "#2E3440", fg: "#D8DEE9", title: "#88C0D0", accent: "#5E81AC", ok: "#A3BE8C", warn: "#EBCB8B", fail: "#BF616A",
		muted: "#81A1C1", command: "#8FBCBB", seat: "#B48EAD"},
	// https://catppuccin.com/palette (Mocha and Latte)
	"catppuccin-mocha": {bg: "#1E1E2E", fg: "#CDD6F4", title: "#CBA6F7", accent: "#B4BEFE", ok: "#A6E3A1", warn: "#F9E2AF", fail: "#F38BA8",
		muted: "#A6ADC8", command: "#89DCEB", seat: "#F5C2E7"},
	"catppuccin-latte": {bg: "#EFF1F5", fg: "#4C4F69", title: "#8839EF", accent: "#1E66F5", ok: "#179299", warn: "#E64553", fail: "#D20F39",
		muted: "#5C5F77", command: "#1E66F5", seat: "#8839EF"},
	// https://github.com/morhetz/gruvbox (bright colours on dark, faded on light)
	"gruvbox-dark": {bg: "#282828", fg: "#EBDBB2", title: "#FE8019", accent: "#83A598", ok: "#B8BB26", warn: "#FABD2F", fail: "#FB4934",
		muted: "#A89984", command: "#8EC07C", seat: "#D3869B"},
	"gruvbox-light": {bg: "#FBF1C7", fg: "#3C3836", title: "#AF3A03", accent: "#076678", ok: "#79740E", warn: "#B57614", fail: "#9D0006",
		muted: "#7C6F64", command: "#427B58", seat: "#8F3F71"},
	// https://draculatheme.com/spec
	"dracula": {bg: "#282A36", fg: "#F8F8F2", title: "#FF79C6", accent: "#FFB86C", ok: "#50FA7B", warn: "#F1FA8C", fail: "#FF5555",
		muted: "#BD93F9", command: "#8BE9FD", seat: "#FF79C6"},
	// https://github.com/enkia/tokyo-night-vscode-theme (Night)
	"tokyonight": {bg: "#1A1B26", fg: "#C0CAF5", title: "#7AA2F7", accent: "#BB9AF7", ok: "#9ECE6A", warn: "#E0AF68", fail: "#F7768E",
		muted: "#737AA2", command: "#7DCFFF", seat: "#BB9AF7"},
	// https://ethanschoonover.com/solarized/ (one palette, dark or light by the
	// terminal's background; on light, its green and yellow fall below 3:1, so
	// success is the emphasised text colour and warnings orange)
	"solarized": {bg: "#002B36", fg: "#93A1A1", title: "#268BD2", accent: "#6C71C4", ok: "#859900", warn: "#B58900", fail: "#DC322F",
		muted: "#839496", command: "#2AA198", seat: "#D33682"},
	"solarized-light": {bg: "#FDF6E3", fg: "#586E75", title: "#268BD2", accent: "#6C71C4", ok: "#586E75", warn: "#CB4B16", fail: "#DC322F",
		muted: "#657B83", command: "#268BD2", seat: "#D33682"},
}

// ThemeNamed returns a theme for a dark background; an unknown name is "auto".
func ThemeNamed(name string) Theme { return ThemeFor(name, true) }

// ThemeFor returns a theme for a dark or light terminal background (the
// inbox asks the terminal; elsewhere it is taken as dark). Only "auto" and
// "solarized" change with it: a named palette paints its own background.
func ThemeFor(name string, dark bool) Theme {
	s := lipgloss.NewStyle
	if name == "solarized" && !dark {
		name = "solarized-light"
	}
	if pl, ok := palettes[name]; ok {
		c := func(h string) lipgloss.Style { return s().Foreground(lipgloss.Color(h)) }
		if name == "solarized-light" {
			name = "solarized"
		}
		return Theme{Name: name, Bg: pl.bg, Fg: pl.fg, Styles: map[Role]lipgloss.Style{
			Title: c(pl.title).Bold(true), Accent: c(pl.accent), OK: c(pl.ok), Warn: c(pl.warn), Fail: c(pl.fail).Bold(true),
			Muted: c(pl.muted), Command: c(pl.command), Seat: c(pl.seat).Italic(true),
		}}
	}
	switch name {
	case "saffron": // warm true-colour palette for dark terminals
		return Theme{Name: "saffron", Styles: map[Role]lipgloss.Style{
			Title: s().Bold(true).Foreground(lipgloss.Color("#FF9933")), Accent: s().Foreground(lipgloss.Color("#FFB86B")),
			OK: s().Foreground(lipgloss.Color("#7FD99A")), Warn: s().Foreground(lipgloss.Color("#F5D76E")), Fail: s().Bold(true).Foreground(lipgloss.Color("#FF6B6B")),
			Muted: s().Foreground(lipgloss.Color("#9AA3B2")), Command: s().Foreground(lipgloss.Color("#6FD3E8")), Seat: s().Italic(true).Foreground(lipgloss.Color("#D7A8FF")),
		}}
	case "contrast": // bright basic colours and bold for low vision
		return Theme{Name: "contrast", Styles: map[Role]lipgloss.Style{
			Title: s().Bold(true).Underline(true), Accent: s().Bold(true).Foreground(lipgloss.BrightWhite),
			OK: s().Bold(true).Foreground(lipgloss.BrightGreen), Warn: s().Bold(true).Foreground(lipgloss.BrightYellow), Fail: s().Bold(true).Foreground(lipgloss.BrightRed),
			Muted: s(), Command: s().Bold(true).Foreground(lipgloss.BrightCyan), Seat: s().Bold(true).Foreground(lipgloss.BrightMagenta),
		}}
	case "mono": // no hue at all: weight and underline only
		return Theme{Name: "mono", Styles: map[Role]lipgloss.Style{
			Title: s().Bold(true), Accent: s().Bold(true), OK: s(), Warn: s().Bold(true), Fail: s().Bold(true).Underline(true),
			Muted: s().Faint(true), Command: s().Underline(true), Seat: s().Italic(true),
		}}
	}
	// auto: only the 16 ANSI colours and the default foreground, so the
	// terminal's palette applies. On a light background the ANSI green,
	// yellow, and cyan of common palettes (Gruvbox, Catppuccin Latte,
	// Solarized) and faint text fall near or below 3:1, so success and
	// warnings are bold text, commands blue, and muted text plain.
	ok, warn, command, muted := s().Foreground(lipgloss.Green), s().Foreground(lipgloss.Yellow), s().Foreground(lipgloss.Cyan), s().Faint(true)
	if !dark {
		ok, warn, command, muted = s().Bold(true), s().Bold(true), s().Foreground(lipgloss.Blue), s()
	}
	return Theme{Name: "auto", Styles: map[Role]lipgloss.Style{
		Title: s().Bold(true).Foreground(lipgloss.Magenta), Accent: s().Foreground(lipgloss.Magenta),
		OK: ok, Warn: warn, Fail: s().Bold(true).Foreground(lipgloss.Red),
		Muted: muted, Command: command, Seat: s().Italic(true).Foreground(lipgloss.Magenta),
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
