package evals

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/models"
	"github.com/rahulnakmol/agentic-ayudha/internal/planning"
)

// CheckPlanningDraft on an XL story checks the draft under the story's parent
// Feature (its github/<n>/parent fixture): the recorded two-story split
// passes, a single story is refused as not splitting the XL story, and a
// story outside the Feature's expected paths is refused. Drafts are decoded
// strictly, and a draft kind or fixture the case does not have is an error.
func TestCheckPlanningDraft(t *testing.T) {
	c, err := LoadCase(filepath.Join(root, "evals", "cases", "slice", "xl-story"))
	if err != nil {
		t.Fatal(err)
	}
	c.Tier2 = &Tier2Spec{Draft: "slice", Issue: 50}
	good, err := os.ReadFile(filepath.Join(c.Dir, "output", "slice.json"))
	if err != nil {
		t.Fatal(err)
	}
	if problems, _, err := CheckPlanningDraft(c, good); err != nil || len(problems) != 0 {
		t.Fatalf("recorded XL split: %v %v", err, problems)
	}
	edit := func(f func(*planning.SliceDraft)) []byte {
		var d planning.SliceDraft
		_ = json.Unmarshal(good, &d)
		f(&d)
		b, _ := json.Marshal(d)
		return b
	}
	for name, tc := range map[string]struct {
		raw  []byte
		want string
	}{
		"one story":     {edit(func(d *planning.SliceDraft) { d.Stories = d.Stories[:1]; d.Stories[0].DependsOn = nil }), "#50 is XL: slice it into at least two XS-L stories"},
		"outside paths": {edit(func(d *planning.SliceDraft) { d.Stories[0].Paths = []string{"internal/models/**"} }), "story 1: paths: internal/models/** is outside the Feature's expected paths"},
		"unknown field": {[]byte(`{"feature": 50, "stories": [], "notes": "x"}`), `unknown field "notes"`},
	} {
		problems, _, err := CheckPlanningDraft(c, tc.raw)
		if err != nil || !strings.Contains(strings.Join(problems, "\n"), tc.want) {
			t.Errorf("%s: %v, want %q in %v", name, err, tc.want, problems)
		}
	}

	p, err := LoadCase(filepath.Join(root, "evals", "cases", "plan", "config-migration"))
	if err != nil {
		t.Fatal(err)
	}
	if problems, _, err := CheckPlanningDraft(p, []byte(`{"story": 41, "checkpoints": [], "rollback": "x"}`)); err != nil || !strings.Contains(strings.Join(problems, "\n"), `unknown field "rollback"`) {
		t.Errorf("plan draft with an unknown field: %v %v", err, problems)
	}
	for name, spec := range map[string]Tier2Spec{
		"unknown draft kind": {Draft: "build", Issue: 41},
		"no issue fixture":   {Draft: "plan", Issue: 99},
	} {
		p.Tier2 = &spec
		if _, _, err := CheckPlanningDraft(p, []byte(`{}`)); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

// A slice or plan reply that is JSON of the wrong shape twice is the skill's
// failure (score 1, no judge); one that is not JSON twice is a run error, as
// is a case whose draft kind or fixture is missing.
func TestTier2PlanningReplyShapes(t *testing.T) {
	c, err := LoadCase(filepath.Join(root, "evals", "cases", "slice", "proof-retention-6-layer"))
	if err != nil {
		t.Fatal(err)
	}
	fake := &models.Fake{Replies: []models.Response{
		{Text: `{"feature": 40}`}, {Text: `{"feature": 40}`}, // no stories, twice
		{Text: "I would slice it into five stories."}, {Text: "Five stories."},
	}}
	h := harness(t, fake, "gpt-6.1-sol")
	h.Config.K = 2
	res, err := h.Tier2(context.Background(), []Case{c}, "abc", "dev", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	runs := res.Cases[0].Runs
	if runs[0].Status != RunFail || runs[0].Score != 1 || !strings.Contains(runs[0].Floor, "output is not a valid slice draft") ||
		runs[1].Status != RunError || len(fake.Requests) != 4 {
		t.Fatalf("runs %+v after %d calls", runs, len(fake.Requests))
	}

	for name, spec := range map[string]Tier2Spec{
		"unknown draft kind": {Rubric: c.Tier2.Rubric, Draft: "build", Issue: 40},
		"no issue fixture":   {Rubric: c.Tier2.Rubric, Draft: "slice", Issue: 99},
	} {
		good, _ := os.ReadFile(filepath.Join(c.Dir, "output", "slice.json"))
		broken := c
		broken.Tier2 = &spec
		h := harness(t, &models.Fake{Replies: []models.Response{{Text: string(good)}}}, "gpt-6.1-sol")
		h.Config.K = 1
		res, err := h.Tier2(context.Background(), []Case{broken}, "abc", "dev", time.Now())
		if err != nil || res.Cases[0].Runs[0].Status != RunError {
			t.Errorf("%s: %v %+v", name, err, res)
		}
	}
}

// The plan case's scope rules run on the draft before the judge: the recorded
// plan passes, a plan that adds a downgrade or refuses older configs fails, and
// the same choice raised as an open question passes.
func TestPlanScope(t *testing.T) {
	c, err := LoadCase(filepath.Join(root, "evals", "cases", "plan", "config-migration"))
	if err != nil {
		t.Fatal(err)
	}
	good, err := os.ReadFile(filepath.Join(c.Dir, "output", "plan.json"))
	if err != nil {
		t.Fatal(err)
	}
	edit := func(f func(*planning.PlanDraft)) []byte {
		var d planning.PlanDraft
		if err := json.Unmarshal(good, &d); err != nil {
			t.Fatal(err)
		}
		f(&d)
		b, _ := json.Marshal(d)
		return b
	}
	for _, tc := range []struct {
		name, why string
		raw       []byte
	}{
		{"recorded", "", good},
		{"downgrade", "a downgrade path", edit(func(d *planning.PlanDraft) {
			d.Checkpoints[0].Rollback += "; a downgrade collapses the per-kind values back into one scalar"
		})},
		{"rejects legacy", `refusing older configs, which the story does not ask for ("rejects legacy" in "config load rejects legacy retention_days after the bump")`, edit(func(d *planning.PlanDraft) {
			d.Checkpoints[0].Contracts = append(d.Checkpoints[0].Contracts, "config load rejects legacy retention_days after the bump")
		})},
		// v0.2.0 rolling run 38953604673 floored a plan on "reject legacy": a plan
		// that promises the opposite honours the story (D-5.4d).
		{"promises not to reject legacy", "", edit(func(d *planning.PlanDraft) {
			d.Checkpoints[0].Contracts = append(d.Checkpoints[0].Contracts, "config load does not reject legacy retention_days; it keeps reading them until the migration runs")
			d.Checkpoints[0].Tests = append(d.Checkpoints[0].Tests, "internal/config:loading never refuses old configs")
		})},
		// Push runs 39013697227 and 39014607367 floored plans that rule a
		// downgrade out; the story's rollback is the restore (D-5.4f).
		{"rules out a downgrade", "", edit(func(d *planning.PlanDraft) {
			d.Checkpoints[0].Rollback += "; no downgrade transform or re-serialisation is used, and no downgrade path other than restoring the backup"
		})},
		{"asked instead", "", edit(func(d *planning.PlanDraft) {
			d.OpenQuestions = append(d.OpenQuestions, "Should config load reject legacy retention_days, and is a downgrade needed?")
		})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			problems, _, err := CheckPlanningDraft(c, tc.raw)
			if err != nil {
				t.Fatal(err)
			}
			got := strings.Join(problems, "\n")
			if tc.why == "" {
				if got != "" {
					t.Fatalf("flagged: %s", got)
				}
				return
			}
			if !strings.Contains(got, "out of scope: adds "+tc.why) {
				t.Fatalf("not caught: %q", got)
			}
		})
	}
}
