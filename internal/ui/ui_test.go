package ui

import (
	"bytes"
	"math"
	"strconv"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestLevelFor(t *testing.T) {
	tty := Signals{TTY: true}
	tests := []struct {
		name string
		s    Signals
		want Level
	}{
		{"a person on a terminal", tty, Rich},
		{"a pipe", Signals{}, Machine},
		{"--json on a terminal", Signals{TTY: true, JSON: true}, Machine},
		{"agent lane on a terminal", Signals{TTY: true, Agent: true}, Machine},
		{"CI on a terminal", Signals{TTY: true, CI: true}, Machine},
		{"--plain", Signals{TTY: true, Plain: true}, Plain},
		{"TERM=dumb", Signals{TTY: true, Term: "dumb"}, Plain},
		{"--plain does not make a pipe human", Signals{Plain: true}, Machine},
		{"NO_COLOR keeps Rich (symbols and motion) but drops colour", Signals{TTY: true, NoColor: true}, Rich},
	}
	for _, tt := range tests {
		if got := LevelFor(tt.s); got != tt.want {
			t.Errorf("%s: level %s, want %s", tt.name, got, tt.want)
		}
	}
	if p := New(&bytes.Buffer{}, Signals{TTY: true, NoColor: true}); p.Color {
		t.Error("NO_COLOR: colour on")
	}
}

// Meaning never rides on colour or a symbol alone: the status word is there at
// every level, and only Rich adds symbols or escape sequences.
func TestMarkKeepsTheWord(t *testing.T) {
	for _, s := range []Signals{{}, {TTY: true, Plain: true}, {TTY: true, Term: "dumb"}, {TTY: true}, {TTY: true, NoColor: true}} {
		p := New(&bytes.Buffer{}, s)
		for _, w := range []string{StatusPass, StatusWarn, StatusFail, StatusNA} {
			m := p.Mark(w)
			if !strings.Contains(m, w) {
				t.Errorf("%+v: Mark(%s) = %q", s, w, m)
			}
			if p.Level != Rich && m != w {
				t.Errorf("%+v: Mark(%s) = %q, want the bare word below Rich", s, w, m)
			}
			if !p.Color && strings.Contains(m, "\x1b[") {
				t.Errorf("%+v: escape sequence without colour: %q", s, m)
			}
		}
	}
}

func TestProgressAndCelebrationPerLevel(t *testing.T) {
	for _, tt := range []struct {
		s    Signals
		want string
	}{
		{Signals{}, ""}, // machines get nothing on the conversation stream
		{Signals{TTY: true, Plain: true}, "Floor 1/2: coverage ...\nPASS Floor 1/2: coverage: pass (0.0s)\nMilestone: Released! v1.2.0 is on main.\n"},
		{Signals{TTY: true, NoColor: true}, "✓ PASS Floor 1/2: coverage: pass (0.0s)\n★ Released! v1.2.0 is on main.\n"},
	} {
		var b bytes.Buffer
		p := New(&b, tt.s)
		p.Static = true
		p.Start("Floor 1/2: coverage").Done(StatusPass, "pass")
		p.Celebrate("Released!", "v1.2.0 is on main.")
		if b.String() != tt.want {
			t.Errorf("%+v:\n got %q\nwant %q", tt.s, b.String(), tt.want)
		}
	}
}

func TestTablePlainHasNoBoxDrawing(t *testing.T) {
	p := New(&bytes.Buffer{}, Signals{TTY: true, Plain: true})
	got := p.Table([]string{"Status", "Check"}, [][]string{{"PASS", "git"}, {"WARN", "bot-identity"}})
	want := "Status  Check\nPASS    git\nWARN    bot-identity\n"
	if got != want {
		t.Errorf("plain table:\n%q\nwant\n%q", got, want)
	}
	rich := New(&bytes.Buffer{}, Signals{TTY: true, NoColor: true}).Table([]string{"Status", "Check"}, [][]string{{"PASS", "git"}})
	if !strings.Contains(rich, "╭") || !strings.Contains(rich, "PASS") {
		t.Errorf("rich table:\n%s", rich)
	}
}

// Every theme styles every role; "auto" uses only the terminal's 16 ANSI
// colours (so a Nord, Catppuccin, or Gruvbox terminal palette applies) and
// gives no warning or command a colour that light backgrounds wash out.
func TestThemes(t *testing.T) {
	for _, name := range Themes {
		for _, dark := range []bool{true, false} {
			th := ThemeFor(name, dark)
			if th.Name != name {
				t.Errorf("ThemeFor(%q, %v) is %q", name, dark, th.Name)
			}
			for r := Title; r <= Seat; r++ {
				if _, ok := th.Styles[r]; !ok {
					t.Errorf("%s: no style for role %d", name, r)
				}
			}
		}
	}
	for _, dark := range []bool{true, false} {
		th := ThemeFor("auto", dark)
		if th.Bg != "" || th.Fg != "" {
			t.Errorf("auto paints a background: %q %q", th.Bg, th.Fg)
		}
		for r, st := range th.Styles {
			if c := st.GetForeground(); c != nil && c != (lipgloss.NoColor{}) {
				if _, ok := c.(ansi.BasicColor); !ok {
					t.Errorf("auto (dark %v): role %d is %T %v, not one of the 16 ANSI colours", dark, r, c, c)
				}
			}
		}
	}
	if ThemeFor("auto", false).Styles[Warn].GetForeground() != (lipgloss.NoColor{}) {
		t.Error("auto on a light background: warnings in yellow")
	}
	if ThemeFor("solarized", false).Bg != "#FDF6E3" || ThemeFor("solarized", true).Bg != "#002B36" {
		t.Error("solarized does not follow the background")
	}
}

// A named palette is readable as published: each role at least 3:1 against
// its background (WCAG's bar for bold text and interface parts), and muted
// text at least 4:1. The words carry the meaning; the colour repeats it.
func TestPalettesAreReadable(t *testing.T) {
	lum := func(h string) float64 {
		var rgb [3]float64
		for i := range rgb {
			v, err := strconv.ParseUint(h[1+2*i:3+2*i], 16, 8)
			if err != nil {
				t.Fatalf("%q: %v", h, err)
			}
			c := float64(v) / 255
			if c <= 0.03928 {
				rgb[i] = c / 12.92
			} else {
				rgb[i] = math.Pow((c+0.055)/1.055, 2.4)
			}
		}
		return 0.2126*rgb[0] + 0.7152*rgb[1] + 0.0722*rgb[2]
	}
	ratio := func(a, b string) float64 {
		la, lb := lum(a), lum(b)
		return (max(la, lb) + 0.05) / (min(la, lb) + 0.05)
	}
	for name, p := range palettes {
		for role, c := range map[string]string{"fg": p.fg, "title": p.title, "accent": p.accent, "ok": p.ok, "warn": p.warn, "fail": p.fail,
			"muted": p.muted, "command": p.command, "seat": p.seat} {
			bar := 3.0
			if role == "muted" || role == "fg" {
				bar = 4.0
			}
			if r := ratio(c, p.bg); r < bar {
				t.Errorf("%s: %s %s on %s is %.1f:1, below %.1f:1", name, role, c, p.bg, r, bar)
			}
		}
	}
}
