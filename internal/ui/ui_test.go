package ui

import (
	"bytes"
	"strings"
	"testing"
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
