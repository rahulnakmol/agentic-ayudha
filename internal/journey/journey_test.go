package journey

import (
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

func TestNext(t *testing.T) {
	ready := []Item{{Number: 42, Title: "Export CSV"}, {Number: 43, Title: "Search"}}
	base := State{InRepo: true, Configured: true, GitHub: GitHubOK, Hosts: []string{"claude"}, Ready: ready}
	with := func(f func(*State)) State { s := base; f(&s); return s }
	tests := []struct {
		name  string
		s     State
		agent bool
		want  string
		human bool
	}{
		{"outside a repository", State{}, false, "aayu tour", true},
		{"not set up", State{InRepo: true, GitHub: GitHubOK}, false, "aayu tour", true},
		{"no GitHub sign-in", with(func(s *State) { s.GitHub = GitHubNoToken }), false, "gh auth login", true},
		{"no host installed: the detected one", with(func(s *State) { s.Hosts, s.Host = nil, "codex" }), false, "aayu install --host codex --apply", true},
		{"copies out of sync", with(func(s *State) { s.OutOfSync = []string{".claude/skills/build was edited"} }), false, "aayu doctor --fix", true},
		{"a PR whose gate passed beats Ready work", with(func(s *State) {
			s.AwaitingHuman = []Item{{Number: 50, Story: 41, Gate: "pending"}, {Number: 51, Story: 40, Gate: "passed"}}
		}), false, "aayu ship 51", true},
		{"an agent never gets ship: it builds the Ready story", with(func(s *State) {
			s.AwaitingHuman = []Item{{Number: 51, Story: 40, Gate: "passed"}}
		}), true, "aayu build 42", false},
		{"a pending gate is not a person's turn yet", with(func(s *State) { s.AwaitingHuman = []Item{{Number: 50, Story: 41, Gate: "pending"}} }), false, "aayu build 42", false},
		{"a floor miss beats a new Ready story", with(func(s *State) { s.FailingFloors = []Floor{{Story: 39, Floor: "coverage", Result: "fail"}} }), false, "aayu build 39", false},
		{"in progress beats Ready", with(func(s *State) { s.InProgress = []Item{{Number: 38, Title: "Login"}} }), false, "aayu build 38", false},
		{"Ready: the lowest number", base, false, "aayu build 42", false},
		{"a due measurement when nothing is Ready", with(func(s *State) {
			s.Ready = nil
			s.MeasureDue = []Due{{Issue: 7, Type: "goal", Due: "2026-10-01", Tag: "v1.0.0"}}
		}), false, "aayu measure 7", false},
		{"nothing waiting: shape the next story", with(func(s *State) { s.Ready = nil }), false, "aayu grill story", false},
		{"paused: a person resumes; an agent has nothing it may do but shape", with(func(s *State) { s.Paused, s.Ready = true, nil }), false, "aayu resume", true},
		{"a paused agent lane is told a person must resume", with(func(s *State) { s.Paused = true }), true, "aayu resume", true},
		{"only human steps left: the agent is told who must act", State{InRepo: true, GitHub: GitHubOK}, true, "aayu tour", true},
	}
	for _, tt := range tests {
		got := Next(tt.s, tt.agent)
		if got.Command != tt.want || got.HumanOnly != tt.human {
			t.Errorf("%s: next %q (human only %v), want %q (%v); why: %s", tt.name, got.Command, got.HumanOnly, tt.want, tt.human, got.Why)
		}
	}
}

func TestLevelAndWhere(t *testing.T) {
	if l := (State{InRepo: true}).Level(); l != 0 {
		t.Errorf("unconfigured: level %d", l)
	}
	if l := (State{InRepo: true, Configured: true}).Level(); l != 1 {
		t.Errorf("configured: level %d", l)
	}
	if l := (State{InRepo: true, Configured: true, Gates: true}).Level(); l != 2 {
		t.Errorf("gates: level %d", l)
	}
	if w := Where(Step{Stage: verbs.StageExplain}); w != verbs.StageShape {
		t.Errorf("a stage off the drawn path falls back to Shape, got %s", w)
	}
}
