package scope

import (
	"slices"
	"strings"
	"testing"
)

var table = []int{1, 2, 3, 5, 8}

func story(n int, effort string, parent int) Item {
	return Item{Number: n, Type: "story", Title: "s", Effort: effort, Points: PointsFor("story", effort, table), Parent: parent}
}

// Drift weighs every change by effort: an added item at its points, a
// removed one at its baseline points, a resize by the change in points only,
// and containers at nothing.
func TestMeasure(t *testing.T) {
	b := Baseline{Baseline: 2, BudgetPct: 20, Items: []Item{
		{Number: 1, Type: "feature"}, story(2, "M", 1), story(3, "S", 1), story(4, "L", 1),
	}} // 3 + 2 + 5 = 10 points
	now := []Item{
		{Number: 1, Type: "feature"},
		story(2, "M", 1),             // unchanged
		story(3, "L", 1),             // resized S to L: +3, not 5
		story(9, "XS", 1),            // added: +1
		{Number: 8, Type: "feature"}, // added container: 0
	} // #4 removed: +5
	d := Measure(b, now, table)
	if d.BaselinePoints != 10 || d.ChangedPoints != 9 || d.DriftPct != 90 || !d.Over() || d.Baseline != 2 {
		t.Fatalf("drift = %+v", d)
	}
	if got := []int{len(d.Added), len(d.Removed), len(d.Resized)}; !slices.Equal(got, []int{2, 1, 1}) ||
		d.Added[0].Number != 8 || d.Removed[0].Number != 4 || d.Resized[0].From != "S" || d.Resized[0].To != "L" {
		t.Fatalf("added %v removed %v resized %v", d.Added, d.Removed, d.Resized)
	}
	// One point of three, and the budget's own edge: 20.0 is within a 20.0 budget.
	if d := Measure(Baseline{BudgetPct: 20, Items: []Item{story(2, "M", 0)}}, []Item{story(2, "M", 0), story(5, "XS", 0)}, table); d.DriftPct != 33.3 {
		t.Fatalf("one of three points = %v", d.DriftPct)
	}
	edge := Measure(Baseline{BudgetPct: 20, Items: []Item{story(2, "L", 0)}}, []Item{story(2, "L", 0), story(5, "XS", 0)}, table)
	if edge.DriftPct != 20 || edge.Over() {
		t.Fatalf("one of five points = %v, over %v", edge.DriftPct, edge.Over())
	}
	// Points come from the table, not from what a record stored.
	stale := Baseline{BudgetPct: 20, Items: []Item{{Number: 2, Type: "story", Effort: "M", Points: 99}}}
	if d := Measure(stale, []Item{story(2, "M", 0)}, table); d.ChangedPoints != 0 || d.BaselinePoints != 3 {
		t.Fatalf("stored points leaked into drift: %+v", d)
	}
}

func TestProblems(t *testing.T) {
	ps := Problems("Ship eligibility", []Item{
		{Number: 1, Type: "feature"}, story(2, "", 1), story(3, "XL", 1),
		{Number: 4, Type: "feature"}, // no Story under it in the plan
		{Number: 5, Type: "bug", Effort: "S", Parent: 4 + 100},
	})
	got := strings.Join(ps, "\n")
	for _, want := range []string{"#2 (story) has no effort", "#3 (story) is XL", "Feature #4 has no Story or Bug"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if len(ps) != 3 || strings.Contains(got, "Feature #1") {
		t.Fatalf("problems:\n%s", got)
	}
	if ps := Problems(" ", nil); len(ps) != 2 {
		t.Fatalf("empty plan: %v", ps)
	}
}

// Versions compare as numbers, and baselines by their number.
func TestNextAndNewest(t *testing.T) {
	if got := Next("v0.3.0", []string{"v0.10.0", "v0.4.0", "v0.3.0", "v0.2.9", "rolling"}); got != "v0.4.0" {
		t.Fatalf("Next = %q", got)
	}
	if got := Next("v0.3.0", []string{"v0.3.0", "v0.2.0"}); got != "" {
		t.Fatalf("Next with none later = %q", got)
	}
	if got := Newest([]string{"releases/v0.3.0/baseline-2.json", "releases/v0.3.0/baseline-10.json", "releases/v0.3.0/draft.json"}); got != 10 {
		t.Fatalf("Newest = %d", got)
	}
	if Hash("a", []Item{story(2, "S", 0), story(1, "S", 0)}) != Hash("a ", []Item{story(1, "S", 0), story(2, "S", 0)}) ||
		Hash("a", []Item{story(1, "S", 0)}) == Hash("a", []Item{story(1, "M", 0)}) {
		t.Fatal("Hash must ignore order and outer space, and see a resize")
	}
}
