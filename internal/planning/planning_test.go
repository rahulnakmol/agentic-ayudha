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

// pathCheck slices a one-story draft of a Feature with the expected paths and
// returns the story's path problems and its footprint.
func pathCheck(t *testing.T, expected, paths []string) (string, string) {
	t.Helper()
	d := recordedSlice(t)
	d.Stories = d.Stories[:1]
	d.Stories[0].DependsOn, d.Stories[0].Paths = nil, paths
	s := planning.CheckSlice(planning.Source{Number: 1, Type: "feature", Feature: 1, ExpectedPaths: expected}, d, autonomy.ReadLayers(filepath.Join("..", ".."))).Stories[0]
	var got []string
	for _, p := range s.Problems {
		if p.Field == "paths" {
			got = append(got, p.Message)
		}
	}
	return strings.Join(got, "\n"), strings.Join(s.Footprint, ",")
}

// f887 is Feature #887's Expected paths.
var f887 = []string{"internal/workitems/", "internal/delegate/brief.go", "internal/inspect/", "internal/cli/slice.go"}

// A directory in a Feature's Expected paths covers every file under it, with
// or without its slash, and only those: a sibling with a longer name is
// outside (Story #1210, criteria 1-3).
func TestCheckSliceDirectoryPaths(t *testing.T) {
	const outside = " is outside the Feature's expected paths"
	for _, tc := range []struct {
		name      string
		expected  []string
		paths     []string
		problem   string // "" when the paths have no problem
		footprint string
	}{
		{"1_file_under_directory", f887, []string{"internal/workitems/validate.go"}, "", "L03"},
		{"1_nested_file_under_directory", f887, []string{"internal/workitems/gen/main.go"}, "", "L03"},
		{"2_directory_without_slash", []string{"internal/workitems"}, []string{"internal/workitems/validate.go"}, "", "L03"},
		{"2_story_directory_without_slash", f887, []string{"internal/workitems"}, "", "L03"},
		{"3_sibling_with_longer_name", f887, []string{"internal/workitemsx/a.go"}, "internal/workitemsx/a.go" + outside, ""},
		{"3_sibling_without_slash", []string{"internal/workitems"}, []string{"internal/workitemsx/a.go"}, "internal/workitemsx/a.go" + outside, ""},
		{"3_sibling_of_named_file", f887, []string{"internal/cli/slicex.go"}, "internal/cli/slicex.go" + outside, "L02"},
		{"3_parent_directory", f887, []string{"internal/"}, "internal/" + outside, "L02,L03,L04,L05,L06,L07,L08"},
		{"3_parent_glob", f887, []string{"internal/**"}, "internal/**" + outside, "L02,L03,L04,L05,L06,L07,L08"},
		{"3_parent_traversal", f887, []string{"internal/workitems/../verify/footprint.go"}, "internal/workitems/../verify/footprint.go" + outside, "L04"},
		{"3_traversal_back_inside", f887, []string{"internal/verify/../workitems/validate.go"}, "", "L03"},
		{"3_glob_wider_than_expected", []string{"internal/cli/?_test.go"}, []string{"internal/cli/*_test.go"}, "internal/cli/*_test.go" + outside, "L02"},
		{"3_glob_narrower_than_expected", []string{"internal/cli/*_test.go"}, []string{"internal/cli/?_test.go"}, "", "L02"},
		{"1_glob_under_directory", f887, []string{"internal/workitems/*.go"}, "", "L03"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			problems, footprint := pathCheck(t, tc.expected, tc.paths)
			if (tc.problem == "") != (problems == "") || !strings.Contains(problems, tc.problem) || footprint != tc.footprint {
				t.Errorf("problems %q, footprint [%s]; want %q, [%s]", problems, footprint, tc.problem, tc.footprint)
			}
		})
	}
}

// A story in internal/inspect or internal/delegate falls in the layer
// LAYERS.md gives it, L03, with no path problem (Story #1210, criterion 4).
func TestCheckSliceInspectAndDelegateLayer(t *testing.T) {
	for _, tc := range []struct {
		name      string
		paths     []string
		footprint string
	}{
		{"inspect_file", []string{"internal/inspect/filter.go"}, "L03"},
		{"inspect_directory", []string{"internal/inspect/"}, "L03"},
		{"delegate_file", []string{"internal/delegate/brief.go"}, "L03"},
		{"delegate_and_cli", []string{"internal/delegate/brief.go", "internal/cli/slice.go"}, "L02,L03"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if problems, footprint := pathCheck(t, f887, tc.paths); problems != "" || footprint != tc.footprint {
				t.Errorf("problems %q, footprint [%s]; want none, [%s]", problems, footprint, tc.footprint)
			}
		})
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

// A Feature's technical design (Feature #874, story #991): its layers stay in
// the Feature's footprint, its story order names each story once and nothing
// else, and touching L07 needs a rollback; a story plan's checkpoints stay in
// the design's layers.
func TestCheckDesign(t *testing.T) {
	footprint, stories := []string{"L02", "L03", "L07"}, []int{12, 13, 14}
	good := func() planning.DesignDraft {
		return planning.DesignDraft{Feature: 10, Layers: []string{"L02", "L07"}, StoryOrder: []int{14, 12, 13}, Rollback: "revert"}
	}
	if r := planning.CheckDesign(10, footprint, stories, good()); !r.Pass || len(r.Problems) != 0 {
		t.Fatalf("a good design: %+v", r)
	}
	for name, tc := range map[string]struct {
		edit func(*planning.DesignDraft)
		want string // "" when the design passes
	}{
		"other feature":   {func(d *planning.DesignDraft) { d.Feature = 11 }, "the design is for #11, not #10"},
		"no layers":       {func(d *planning.DesignDraft) { d.Layers = nil }, "the design names no layers"},
		"layer outside":   {func(d *planning.DesignDraft) { d.Layers = append(d.Layers, "L05") }, "layer L05 is not in the footprint of #10 (L02, L03, L07)"},
		"story missing":   {func(d *planning.DesignDraft) { d.StoryOrder = []int{14, 12} }, "story order: #13 is a story of #10 but is not in the order"},
		"not a story":     {func(d *planning.DesignDraft) { d.StoryOrder = append(d.StoryOrder, 99) }, "story order: #99 is not a story of #10"},
		"twice":           {func(d *planning.DesignDraft) { d.StoryOrder = append(d.StoryOrder, 12) }, "story order: #12 is in the order twice"},
		"L07 no rollback": {func(d *planning.DesignDraft) { d.Rollback = " " }, "a design that touches L07 names its rollback"},
		"no L07":          {func(d *planning.DesignDraft) { d.Layers, d.Rollback = []string{"L03"}, "" }, ""},
		"unnumbered":      {func(d *planning.DesignDraft) { d.Feature = 0 }, ""},
	} {
		d := good()
		tc.edit(&d)
		r := planning.CheckDesign(10, footprint, stories, d)
		if got := strings.Join(r.Problems, "\n"); r.Pass != (tc.want == "") || got != tc.want {
			t.Errorf("%s: pass %v, problems %q, want %q", name, r.Pass, got, tc.want)
		}
	}
	if r := planning.CheckDesign(10, nil, nil, planning.DesignDraft{Layers: []string{"L02"}}); strings.Join(r.Problems, "\n") != "layer L02 is not in the footprint of #10 (none)" {
		t.Errorf("a Feature with no footprint: %q", r.Problems)
	}

	design := planning.Design{Feature: 10, Layers: []string{"L02", "L07"}}
	cps := []planning.Checkpoint{{Layer: "L02"}, {Layer: "L06"}, {Layer: "L07"}}
	if got := planning.CheckInDesign(design, cps); strings.Join(got, "\n") != "checkpoint 2 (L06) is outside the technical design of #10 (L02, L07)" {
		t.Errorf("a story plan outside the design: %q", got)
	}
	if got := planning.CheckInDesign(design, cps[:1]); len(got) != 0 {
		t.Errorf("a story plan inside the design: %q", got)
	}
	if planning.DesignLedgerPath(874) != "specs/874/design.json" {
		t.Errorf("design path %s", planning.DesignLedgerPath(874))
	}
}
