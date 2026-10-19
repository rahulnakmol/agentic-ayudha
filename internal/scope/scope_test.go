package scope

import (
	"math"
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
	// A type change is drift: a baselined L Story retyped as an Initiative loses its 5 points.
	if d := Measure(Baseline{BudgetPct: 20, Items: []Item{story(2, "L", 0), story(3, "M", 0)}},
		[]Item{{Number: 2, Type: "initiative", Effort: "L"}, story(3, "M", 0)}, table); d.ChangedPoints != 5 || len(d.Resized) != 1 || d.Resized[0].FromType != "story" || !d.Over() {
		t.Fatalf("a retyped Story: %+v", d)
	}
	// A budget is a number from 0 to 100: NaN would turn every Over off, and 200 would allow any plan.
	for pct, ok := range map[float64]bool{0: true, 20: true, 100: true, -1: false, 100.5: false, 200: false, math.NaN(): false} {
		if BudgetOK(pct) != ok {
			t.Errorf("BudgetOK(%v) = %v", pct, !ok)
		}
	}
	// An effort table has one value of at least 1 per effort: a 0 would hide every change at that size.
	for _, c := range []struct {
		p  []int
		ok bool
	}{{table, true}, {[]int{1, 1, 1, 1, 1}, true}, {[]int{1, 2, 0, 5, 8}, false}, {[]int{1, 2, 3, 5}, false}, {[]int{1, 2, 3, 5, 8, 13}, false}} {
		if EffortPointsOK(c.p) != c.ok {
			t.Errorf("EffortPointsOK(%v) = %v", c.p, !c.ok)
		}
	}
	// The budget compares the exact ratio: 20.01% rounds to 20.0% for people, yet is over a 20.0% budget.
	if d := (Drift{BudgetPct: 20, BaselinePoints: 10000, ChangedPoints: 2001, DriftPct: 20}); !d.Over() {
		t.Fatal("20.01% is within a 20.0% budget")
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
		{Number: 4, Type: "feature"},                     // only a Feature under it: no executable child
		{Number: 6, Type: "feature", Parent: 4},          // and #6 has no Story or Bug either
		{Number: 5, Type: "bug", Effort: "S", Parent: 1}, // the plan's real work sits under #1
	})
	got := strings.Join(ps, "\n")
	for _, want := range []string{"#2 (story) has no effort", "#3 (story) is XL", "Feature #4 has no Story or Bug", "Feature #6 has no Story or Bug"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if len(ps) != 4 || strings.Contains(got, "Feature #1") {
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
