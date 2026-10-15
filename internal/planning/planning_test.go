package planning_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/autonomy"
	"github.com/rahulnakmol/agentic-ayudha/internal/planning"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// recordedSlice is the slice skill's recorded draft of the 6-layer Feature.
func recordedSlice(t *testing.T) planning.SliceDraft {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "evals", "cases", "slice", "proof-retention-6-layer", "output", "slice.json"))
	if err != nil {
		t.Fatal(err)
	}
	var d planning.SliceDraft
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatal(err)
	}
	d.Feature = 1
	return d
}

// problems flattens a slice report's whole-draft and per-story problems.
func problems(r planning.SliceReport) string {
	out := slices.Clone(r.Problems)
	for _, s := range r.Stories {
		for _, p := range s.Problems {
			out = append(out, p.Message)
		}
	}
	return strings.Join(out, "\n")
}

// CheckSlice refuses a draft it cannot place: a Feature without Expected
// paths (no footprint to compute), no stories, a draft for another issue, a
// story that writes Links itself, names no paths, or names paths in no layer.
func TestCheckSliceRefusals(t *testing.T) {
	layers := autonomy.ReadLayers("")
	feature := planning.Source{Number: 1, Type: "feature", Feature: 1,
		ExpectedPaths: []string{"internal/config/**", "internal/proof/**", "internal/cli/proof.go", "skills/proof/**", "internal/github/**", ".github/workflows/**", "assets/raw/**"}}
	if r := planning.CheckSlice(feature, recordedSlice(t), layers); !r.Pass {
		t.Fatalf("the recorded draft does not pass: %s", problems(r))
	}
	for name, tc := range map[string]struct {
		src  func(*planning.Source)
		edit func(*planning.SliceDraft)
		want string
	}{
		"no expected paths": {func(s *planning.Source) { s.ExpectedPaths = nil }, func(*planning.SliceDraft) {}, "Feature #1 has no Expected paths"},
		"no stories":        {nil, func(d *planning.SliceDraft) { d.Stories = nil }, "the draft has no stories"},
		"another feature":   {nil, func(d *planning.SliceDraft) { d.Feature = 7 }, "the draft is for #7, not #1"},
		"links":             {nil, func(d *planning.SliceDraft) { d.Stories[0].Fields[workitems.FieldLinks] = "depends-on #3" }, "slice writes Links from depends_on"},
		"no paths":          {nil, func(d *planning.SliceDraft) { d.Stories[0].Paths = nil }, "name the paths the story changes"},
		"no layer":          {nil, func(d *planning.SliceDraft) { d.Stories[0].Paths = []string{"assets/raw/logo.bin"} }, "the paths fall in no layer of LAYERS.md"},
	} {
		src, d := feature, recordedSlice(t)
		if tc.src != nil {
			tc.src(&src)
		}
		tc.edit(&d)
		r := planning.CheckSlice(src, d, layers)
		if r.Pass || !strings.Contains(problems(r), tc.want) || r.Footprint == nil {
			t.Errorf("%s: pass %v, footprint %v, want %q in:\n%s", name, r.Pass, r.Footprint, tc.want, problems(r))
		}
	}
	// Without expected paths there is no footprint at all.
	if r := planning.CheckSlice(planning.Source{Number: 1, Type: "feature", Feature: 1}, planning.SliceDraft{}, layers); len(r.Footprint) != 0 || r.Footprint == nil {
		t.Errorf("footprint without expected paths: %#v", r.Footprint)
	}
}

// CheckPlan: a story without layer labels takes the plan's layers as its
// footprint (sorted, once each) and the never layers among them need the EM
// review; malformed checkpoints and a plan for another story are refused.
func TestCheckPlan(t *testing.T) {
	story := workitems.Item{Number: 2, Type: "story", Title: "Migrate", Fields: map[string]string{workitems.FieldCriteria: "1. [api-trace] state: a v1 config; expected: 90 days for every kind\n2. [api-trace] state: the rollback; expected: the file is restored"}}
	cp := func(layer string, criteria ...int) planning.Checkpoint {
		return planning.Checkpoint{Layer: layer, Title: "do " + layer, Tests: []string{"TestIt"}, Rollback: "undo", Criteria: criteria}
	}
	r := planning.CheckPlan(story, nil, []string{"L07"}, planning.PlanDraft{Story: 2, Checkpoints: []planning.Checkpoint{cp("L07", 1), cp("L02", 2)}})
	if !r.Pass || !r.FromPlan || strings.Join(r.Footprint, ",") != "L02,L07" || !r.Review.Required || strings.Join(r.Review.Layers, ",") != "L07" {
		t.Fatalf("plan of a story without layer labels: %+v", r)
	}
	if r := planning.CheckPlan(story, []string{"L02"}, []string{"L07"}, planning.PlanDraft{Checkpoints: []planning.Checkpoint{cp("L02", 1, 2)}}); !r.Pass || r.FromPlan || r.Review.Required {
		t.Fatalf("plan within the story's footprint, outside the never layers: %+v", r)
	}

	for name, tc := range map[string]struct {
		d    planning.PlanDraft
		want string
	}{
		"another story":     {planning.PlanDraft{Story: 9, Checkpoints: []planning.Checkpoint{cp("L07", 1, 2)}}, "the plan is for #9, not #2"},
		"bad layer":         {planning.PlanDraft{Checkpoints: []planning.Checkpoint{cp("layer-7", 1, 2)}}, `checkpoint 1: layer "layer-7" is not L01-L09`},
		"no title":          {planning.PlanDraft{Checkpoints: []planning.Checkpoint{{Layer: "L07", Tests: []string{"T"}, Rollback: "undo", Criteria: []int{1, 2}}}}, "checkpoint 1 (L07): title required"},
		"no checkpoints":    {planning.PlanDraft{}, "the plan has no checkpoints"},
		"unknown criterion": {planning.PlanDraft{Checkpoints: []planning.Checkpoint{cp("L07", 1, 2, 9)}}, "checkpoint L07: criterion 9 is not in the Completion Goal"},
	} {
		r := planning.CheckPlan(story, nil, nil, tc.d)
		if r.Pass || !strings.Contains(strings.Join(r.Problems, "\n"), tc.want) || r.Footprint == nil {
			t.Errorf("%s: pass %v, footprint %#v, want %q in %v", name, r.Pass, r.Footprint, tc.want, r.Problems)
		}
	}
}
