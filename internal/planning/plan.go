package planning

import (
	"fmt"
	"slices"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// OperationalData is the layer whose checkpoints need a rollback step (L07:
// migrations and state).
const OperationalData = "L07"

// PlanDraft is the host's draft of a plan (evals/schemas/plan.json).
type PlanDraft struct {
	Story         int          `json:"story"`
	Checkpoints   []Checkpoint `json:"checkpoints"`
	OpenQuestions []string     `json:"open_questions,omitempty"`
}

// Checkpoint is one build checkpoint: one layer of the story's footprint.
type Checkpoint struct {
	Layer           string   `json:"layer"`
	Title           string   `json:"title"`
	Contracts       []string `json:"contracts,omitempty"`
	Tests           []string `json:"tests"`
	Instrumentation []string `json:"instrumentation,omitempty"`
	Rollback        string   `json:"rollback,omitempty"`
	Criteria        []int    `json:"criteria,omitempty"` // Completion Goal criteria this checkpoint's tests prove
	// Checks are the feature-map behaviour checks (<feature>.<check>, WP4.5b.5)
	// that prove the same criteria; they go into the evidence map beside tests.
	Checks []string `json:"checks,omitempty"`
}

// Review is the Engineering Manager's plan-time review, required when the
// footprint touches an [autonomy] never layer.
type Review struct {
	Required   bool     `json:"required"`
	Layers     []string `json:"layers,omitempty"`
	ApprovedBy string   `json:"approved_by,omitempty"`
	ApprovedAt string   `json:"approved_at,omitempty"`
	As         string   `json:"as,omitempty"` // seat (the EM's paired human), approver ([autonomy] approver), or solo
	// Bypass records that no one reviewed the plan: solo mode, with the
	// human's "Solo-mode bypass:" comment on the story (plan --solo-bypass).
	// It is never an approval: ApprovedBy stays empty.
	Bypass *ReviewBypass `json:"bypass,omitempty"`
}

// ReviewBypass is a solo-mode plan-review bypass.
type ReviewBypass struct {
	As         string `json:"as"`       // always "solo-bypass"
	Reviewer   string `json:"reviewer"` // always "none (solo mode)"
	RecordedBy string `json:"recorded_by"`
	RecordedAt string `json:"recorded_at"`
	Comment    string `json:"comment_url"`
	Note       string `json:"note"`
}

// PlanReport is the check of a plan draft.
type PlanReport struct {
	Story     int      `json:"story"`
	Footprint []string `json:"footprint"`
	// FromPlan is true when the story carried no layer labels and the plan's
	// layers become its footprint.
	FromPlan bool     `json:"footprint_from_plan,omitempty"`
	Review   Review   `json:"em_review"`
	Problems []string `json:"problems"`
	Pass     bool     `json:"pass"`
}

// Footprint reads a story's layer labels.
func Footprint(labels []string) []string {
	var out []string
	for _, l := range labels {
		if id, ok := strings.CutPrefix(l, LayerLabel); ok {
			out = append(out, id)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// CheckPlan checks a plan for a story: exactly one checkpoint per layer of
// the footprint (the story's layer labels, or the plan's layers when it has
// none), each with tests and an L07 one with a rollback step, and every
// Completion Goal criterion covered by some checkpoint. never lists the
// [autonomy] never layers, which make the EM review required.
func CheckPlan(story workitems.Item, footprint, never []string, d PlanDraft) PlanReport {
	r := PlanReport{Story: story.Number, Problems: []string{}}
	add := func(format string, a ...any) { r.Problems = append(r.Problems, fmt.Sprintf(format, a...)) }
	if d.Story != 0 && d.Story != story.Number {
		add("the plan is for #%d, not #%d", d.Story, story.Number)
	}
	var layers []string
	for i, c := range d.Checkpoints {
		k := i + 1
		switch {
		case !strings.HasPrefix(c.Layer, "L") || len(c.Layer) != 3:
			add("checkpoint %d: layer %q is not L01-L09", k, c.Layer)
		case slices.Contains(layers, c.Layer):
			add("checkpoint %d: a second checkpoint for %s; one checkpoint per layer", k, c.Layer)
		}
		layers = append(layers, c.Layer)
		if strings.TrimSpace(c.Title) == "" {
			add("checkpoint %d (%s): title required", k, c.Layer)
		}
		if len(c.Tests) == 0 {
			add("checkpoint %d (%s): name the tests that prove it", k, c.Layer)
		}
		if c.Layer == OperationalData && strings.TrimSpace(c.Rollback) == "" {
			add("checkpoint %d (%s): an operational-data checkpoint needs a rollback step", k, c.Layer)
		}
	}
	if len(d.Checkpoints) == 0 {
		add("the plan has no checkpoints")
	}
	r.Footprint = footprint
	if len(footprint) == 0 {
		r.FromPlan = true
		r.Footprint = slices.Clone(layers)
		slices.Sort(r.Footprint)
		r.Footprint = slices.Compact(r.Footprint)
	} else {
		for _, l := range footprint {
			if !slices.Contains(layers, l) {
				add("layer %s of the story's footprint has no checkpoint", l)
			}
		}
		for _, l := range layers {
			if !slices.Contains(footprint, l) {
				add("checkpoint for %s is outside the story's footprint (%s); re-slice, or plan within it", l, strings.Join(footprint, ", "))
			}
		}
	}
	if r.Footprint == nil {
		r.Footprint = []string{}
	}
	cs, _ := workitems.ParseCriteria(story.Fields[workitems.FieldCriteria])
	coveredBy := map[int]bool{}
	for _, c := range d.Checkpoints {
		for _, n := range c.Criteria {
			if !slices.ContainsFunc(cs, func(x workitems.Criterion) bool { return x.N == n }) {
				add("checkpoint %s: criterion %d is not in the Completion Goal", c.Layer, n)
			}
			coveredBy[n] = true
		}
	}
	for _, c := range cs {
		if !coveredBy[c.N] {
			add("criterion %d has no checkpoint whose tests prove it", c.N)
		}
	}
	for _, l := range r.Footprint {
		if slices.Contains(never, l) {
			r.Review.Layers = append(r.Review.Layers, l)
		}
	}
	r.Review.Required = len(r.Review.Layers) > 0
	r.Pass = len(r.Problems) == 0
	return r
}
