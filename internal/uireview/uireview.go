// Package uireview is UI review (WP6.6): two isolated lanes critique the
// captured states of a change. Lane A is a model from the ui-review lane,
// run k = 3 times per state; a finding counts only when at least 2 of the 3
// runs report its kind at Nielsen severity 2 or more. Lane B is
// deterministic: impeccable's detector, axe, and aayudha's focus-indicator
// and target-size probes. Each lane returns a disposition (recapture,
// rebuild, fix, ship); a lane B finding can never be overridden by lane A
// (ground rule 7). polish compares a re-run against the critique it applied,
// and baseline captures a commit's states.
package uireview

import (
	"slices"
)

// Plan constants (WP6.6).
const (
	K           = 3 // lane A runs per state
	Agree       = 2 // runs that must report a kind for it to count
	MinSeverity = 2 // Nielsen severity a run's finding needs to count
)

// Dispositions, worst first.
const (
	Recapture = "recapture" // the captures are missing or broken; capture again before judging
	Rebuild   = "rebuild"   // the design is wrong at its root; back to mockup or prototype
	Fix       = "fix"       // findings to fix in this change
	Ship      = "ship"      // nothing to fix
)

var order = []string{Ship, Fix, Rebuild, Recapture}

// Worst returns the worse of two dispositions.
func Worst(a, b string) string {
	if slices.Index(order, b) > slices.Index(order, a) {
		return b
	}
	return a
}

// Kinds are the finding classes both lanes use; lane A's consensus is per
// state and kind.
var Kinds = []string{"focus-visible", "contrast", "target-size", "keyboard", "labels", "hierarchy", "layout", "copy",
	"feedback", "consistency", "error-prevention", "other"}

// State is one UI state to review.
type State struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

// Finding is a counted finding of one lane.
type Finding struct {
	State     string `json:"state"`
	Kind      string `json:"kind"`
	Severity  int    `json:"severity"`            // Nielsen 0-4
	Heuristic int    `json:"heuristic,omitempty"` // Nielsen heuristic 1-10 (lane A)
	Element   string `json:"element,omitempty"`
	Summary   string `json:"summary"`
	Fix       string `json:"fix,omitempty"`
	Source    string `json:"source"`         // lane B: impeccable:<rule>, axe:<rule>, or probe:<name>; lane A: the model
	Runs      int    `json:"runs,omitempty"` // lane A: runs that reported the kind at severity >= 2
}

// Key identifies a finding across runs of review: state and kind.
func (f Finding) Key() string { return f.State + "/" + f.Kind }

// Lane is one lane's critique.
type Lane struct {
	Name        string    `json:"name"`
	Disposition string    `json:"disposition"`
	Findings    []Finding `json:"findings"`
	Tools       string    `json:"tools,omitempty"` // lane B: tool versions; lane A: the model
	Runs        []Run     `json:"runs,omitempty"`  // lane A: every run, for the record
	Errors      []string  `json:"errors,omitempty"`
}

// Critique is what review writes: both lanes and the combined disposition.
type Critique struct {
	SchemaVersion int     `json:"schema_version"`
	Repo          string  `json:"repo,omitempty"`
	PR            int     `json:"pr,omitempty"`
	Head          string  `json:"head,omitempty"`
	CreatedAt     string  `json:"created_at"`
	States        []State `json:"states"`
	LaneA         Lane    `json:"lane_a"`
	LaneB         Lane    `json:"lane_b"`
	Disposition   string  `json:"disposition"`
}

// Combine sets the critique's disposition: the worse of the two lanes.
func (c *Critique) Combine() { c.Disposition = Worst(c.LaneA.Disposition, c.LaneB.Disposition) }

// Comparison is polish's verdict: the findings of the critique it applied
// that are gone, those that remain, and new ones.
type Comparison struct {
	Resolved  []Finding `json:"resolved"`
	Remaining []Finding `json:"remaining"`
	New       []Finding `json:"new"`
}

// Compare matches findings by lane, state, and kind.
func Compare(before, after *Critique) Comparison {
	var c Comparison
	has := func(fs []Finding, f Finding) bool {
		return slices.ContainsFunc(fs, func(g Finding) bool { return g.Key() == f.Key() })
	}
	for _, p := range [][2][]Finding{{before.LaneA.Findings, after.LaneA.Findings}, {before.LaneB.Findings, after.LaneB.Findings}} {
		for _, f := range p[0] {
			if has(p[1], f) {
				c.Remaining = append(c.Remaining, f)
			} else {
				c.Resolved = append(c.Resolved, f)
			}
		}
		for _, f := range p[1] {
			if !has(p[0], f) {
				c.New = append(c.New, f)
			}
		}
	}
	return c
}
