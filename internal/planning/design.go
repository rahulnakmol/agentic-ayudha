package planning

import (
	"fmt"
	"slices"
	"strings"
)

// DesignDraft is the host's draft of a Feature's technical design (core design
// S5): the contracts it adds or changes, the layers it touches (inside the
// Feature's footprint), the order its stories are built in, and how to roll it
// back. It is decoded strictly; it has no schema file in evals/schemas.
type DesignDraft struct {
	Feature       int      `json:"feature"`
	Contracts     []string `json:"contracts"`
	Layers        []string `json:"layers"`
	StoryOrder    []int    `json:"story_order"`
	Rollback      string   `json:"rollback"`
	OpenQuestions []string `json:"open_questions,omitempty"`
}

// Design is specs/<feature>/design.json on the ledger: the checked design and
// the Feature body it was made for. Every story plan under the Feature stays
// inside its layers.
type Design struct {
	Feature    int      `json:"feature"`
	BodyHash   string   `json:"body_hash"`
	Footprint  []string `json:"footprint"`
	Contracts  []string `json:"contracts"`
	Layers     []string `json:"layers"`
	StoryOrder []int    `json:"story_order"`
	Rollback   string   `json:"rollback,omitempty"`
	PlannedAt  string   `json:"planned_at"`
	PlannedBy  string   `json:"planned_by"`
}

// DesignLedgerPath is where a Feature's technical design lives on the ledger.
func DesignLedgerPath(feature int) string { return fmt.Sprintf("specs/%d/design.json", feature) }

// DesignReport is the check of a design draft.
type DesignReport struct {
	Feature   int      `json:"feature"`
	Footprint []string `json:"footprint"`
	Stories   []int    `json:"stories"`
	Problems  []string `json:"problems"`
	Pass      bool     `json:"pass"`
}

// CheckDesign checks a design draft for a Feature: every layer is in the
// Feature's footprint (from its expected paths), the story order names each
// of the Feature's stories once and nothing else, and a design that touches
// L07 names its rollback.
func CheckDesign(feature int, footprint []string, stories []int, d DesignDraft) DesignReport {
	r := DesignReport{Feature: feature, Footprint: footprint, Stories: stories, Problems: []string{}}
	add := func(format string, a ...any) { r.Problems = append(r.Problems, fmt.Sprintf(format, a...)) }
	if d.Feature != 0 && d.Feature != feature {
		add("the design is for #%d, not #%d", d.Feature, feature)
	}
	if len(d.Layers) == 0 {
		add("the design names no layers")
	}
	for _, l := range d.Layers {
		if !slices.Contains(footprint, l) {
			add("layer %s is not in the footprint of #%d (%s)", l, feature, layerList(footprint))
		}
	}
	for i, s := range d.StoryOrder {
		switch {
		case !slices.Contains(stories, s):
			add("story order: #%d is not a story of #%d", s, feature)
		case slices.Contains(d.StoryOrder[:i], s):
			add("story order: #%d is in the order twice", s)
		}
	}
	for _, s := range stories {
		if !slices.Contains(d.StoryOrder, s) {
			add("story order: #%d is a story of #%d but is not in the order", s, feature)
		}
	}
	if slices.Contains(d.Layers, OperationalData) && strings.TrimSpace(d.Rollback) == "" {
		add("a design that touches %s names its rollback", OperationalData)
	}
	r.Pass = len(r.Problems) == 0
	return r
}

// CheckInDesign returns the checkpoints of a story plan whose layer is outside
// its Feature's technical design.
func CheckInDesign(d Design, cps []Checkpoint) []string {
	var out []string
	for i, cp := range cps {
		if !slices.Contains(d.Layers, cp.Layer) {
			out = append(out, fmt.Sprintf("checkpoint %d (%s) is outside the technical design of #%d (%s)", i+1, cp.Layer, d.Feature, layerList(d.Layers)))
		}
	}
	return out
}

func layerList(ls []string) string {
	if len(ls) == 0 {
		return "none"
	}
	return strings.Join(ls, ", ")
}
