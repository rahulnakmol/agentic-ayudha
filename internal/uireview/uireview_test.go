package uireview

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/models"
)

type finding struct {
	Kind     string `json:"kind"`
	Severity int    `json:"severity"`
}

// reply renders a lane A answer with the given findings.
func reply(rebuild bool, fs ...finding) models.Response {
	out := []RawFinding{}
	for _, f := range fs {
		out = append(out, RawFinding{Kind: f.Kind, Heuristic: 4, Severity: f.Severity, Element: "button", Summary: f.Kind + " problem", Fix: "fix it"})
	}
	b, _ := json.Marshal(map[string]any{"findings": out, "nielsen": make([]int, 10),
		"personas": []map[string]any{{"persona": "keyboard-only", "blocked": false, "note": "ok"}}, "rebuild": rebuild})
	return models.Response{Text: string(b)}
}

func capture(t *testing.T, id string) Capture {
	dir := t.TempDir()
	c := Capture{ID: id, URL: "http://127.0.0.1/" + id, HTML: "<main>page</main>", Aria: "- main", Rest: filepath.Join(dir, "rest.png"), Focus: filepath.Join(dir, "focus.png")}
	_ = os.WriteFile(c.Rest, []byte("rest-png"), 0o644)
	_ = os.WriteFile(c.Focus, []byte("focus-png"), 0o644)
	return c
}

// WP6.6: a lane A finding counts only when at least 2 of 3 runs report its
// kind at severity 2 or more. focus-visible (2 runs), contrast (3), and
// target-size (runs 1 and 3; run 2 rates it 1) count; hierarchy (once at 2,
// once at 1) and copy (one run) do not. Rebuild needs 2 runs as well. Every
// request carries the rest and focus screenshots.
func TestLaneAConsensus(t *testing.T) {
	fake := &models.Fake{Replies: []models.Response{
		reply(true, finding{"focus-visible", 3}, finding{"contrast", 3}, finding{"target-size", 2}, finding{"hierarchy", 2}),
		reply(false, finding{"focus-visible", 4}, finding{"contrast", 2}, finding{"target-size", 1}, finding{"hierarchy", 1}),
		reply(false, finding{"contrast", 3}, finding{"target-size", 3}, finding{"copy", 2}),
		reply(false), reply(false, finding{"layout", 2}), reply(false),
	}}
	l, err := LaneA(context.Background(), fake.Complete, []Capture{capture(t, "planted"), capture(t, "clean")}, K, 44, "medium", "gemini-3.1-pro-low")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]Finding{}
	for _, f := range l.Findings {
		got[f.Key()] = f
	}
	if len(got) != 3 || got["planted/focus-visible"].Runs != 2 || got["planted/contrast"].Runs != 3 || got["planted/target-size"].Runs != 2 {
		t.Fatalf("consensus findings: %+v", l.Findings)
	}
	if got["planted/focus-visible"].Severity != 3 || got["planted/contrast"].Severity != 3 || got["planted/target-size"].Severity != 2 {
		t.Fatalf("severities (lower median of the agreeing runs): %+v", l.Findings)
	}
	if l.Disposition != Fix || len(l.Runs) != 6 {
		t.Fatalf("disposition %s with 1 of 3 runs asking to rebuild; runs %d", l.Disposition, len(l.Runs))
	}
	if len(fake.Requests) != 6 || len(fake.Requests[0].Messages[0].Images) != 2 || fake.Requests[0].Effort != "medium" ||
		!strings.Contains(fake.Requests[0].Messages[0].Content, `<untrusted source="markup">`) {
		t.Fatalf("lane A request: %+v", fake.Requests[0])
	}

	fake = &models.Fake{Replies: []models.Response{reply(true), reply(true), reply(false)}}
	l, err = LaneA(context.Background(), fake.Complete, []Capture{capture(t, "clean")}, K, 44, "high", "m")
	if err != nil || l.Disposition != Rebuild || len(l.Findings) != 0 {
		t.Fatalf("2 of 3 asking to rebuild: %s %+v %v", l.Disposition, l.Findings, err)
	}
	fake = &models.Fake{Replies: []models.Response{reply(false)}}
	if l, _ = LaneA(context.Background(), fake.Complete, []Capture{capture(t, "clean")}, K, 44, "medium", "m"); l.Disposition != Ship {
		t.Fatalf("clean: %s", l.Disposition)
	}
	fake = &models.Fake{Replies: []models.Response{{Text: "not json"}}}
	if _, err = LaneA(context.Background(), fake.Complete, []Capture{capture(t, "clean")}, K, 44, "medium", "m"); err == nil {
		t.Fatal("lane A with no usable run returned no error")
	}
}

func TestLaneBAndDispositions(t *testing.T) {
	caps := []Capture{{ID: "planted", URL: "u", Axe: []AxeHit{{ID: "color-contrast", Impact: "serious", Help: "contrast", Nodes: 1}},
		FocusMissing: []string{"Save"}, SmallTargets: []string{"Help (32x32)"}}, {ID: "clean", URL: "v"}}
	imp := map[string][]ImpeccableHit{"planted": {{Antipattern: "low-contrast", Name: "Low contrast text", Severity: "warning", Snippet: "3.8:1"},
		{Antipattern: "em-dash", Advisory: true}}}
	l := LaneB(caps, imp, nil, "t")
	if l.Disposition != Fix || !slices.Equal(kinds(l), []string{"contrast", "focus-visible", "target-size"}) || len(l.Findings) != 4 {
		t.Fatalf("lane B: %s %+v", l.Disposition, l.Findings)
	}
	if l := LaneB([]Capture{{ID: "clean"}}, nil, nil, "t"); l.Disposition != Ship {
		t.Fatalf("clean lane B: %s", l.Disposition)
	}
	if l := LaneB([]Capture{{ID: "x", Error: "HTTP 500"}}, nil, nil, "t"); l.Disposition != Recapture {
		t.Fatalf("broken capture: %s", l.Disposition)
	}
	// Lane A cannot override lane B (rule 7).
	c := Critique{LaneA: Lane{Disposition: Ship}, LaneB: Lane{Disposition: Fix}}
	c.Combine()
	if c.Disposition != Fix || Worst(Rebuild, Recapture) != Recapture || Worst(Fix, Ship) != Fix {
		t.Fatalf("combined: %s", c.Disposition)
	}
}

// polish compares the re-run with the critique it applied, by lane, state, and kind.
func TestCompare(t *testing.T) {
	before := &Critique{LaneA: Lane{Findings: []Finding{{State: "s", Kind: "contrast"}, {State: "s", Kind: "focus-visible"}}},
		LaneB: Lane{Findings: []Finding{{State: "s", Kind: "target-size"}}}}
	after := &Critique{LaneA: Lane{Findings: []Finding{{State: "s", Kind: "focus-visible"}, {State: "s", Kind: "copy"}}}}
	c := Compare(before, after)
	if len(c.Resolved) != 2 || len(c.Remaining) != 1 || c.Remaining[0].Kind != "focus-visible" || len(c.New) != 1 || c.New[0].Kind != "copy" {
		t.Fatalf("compare: %+v", c)
	}
}
