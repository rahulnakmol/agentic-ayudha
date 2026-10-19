// Package scope plans what a release contains (the Product Strategist's
// `aayu scope`): the plan is the GitHub milestone named by the Release's
// version, an agent snapshots it as a draft, a person accepts the draft as a
// numbered baseline on the ledger, and every later change is measured against
// that baseline as drift, weighted by effort points, within a budget.
//
// The ledger files are the contract with `aayu brain checkpoint`, which links
// them from the brain's release note and never imports this package:
//
//	releases/<version>/draft.json        the newest agent draft (DraftPath)
//	releases/<version>/baseline-<n>.json each accepted baseline (BaselinePath)
//	releases/<version>/drift.json        the drift against the newest baseline (DriftPath)
//
// <version> keeps its leading "v", so these never mix with the promotion
// records at releases/<issue-number>/<rung>.json.
package scope

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// Schemas of the ledger files.
const (
	DraftSchema    = "aayudha.scope.draft/v1"
	BaselineSchema = "aayudha.scope.baseline/v1"
	DriftSchema    = "aayudha.scope.drift/v1"
)

// Types a plan holds. Feedback is assigned to a release but is not planned
// work until it becomes a Feature (grill), so it is never an Item.
var Types = []string{"initiative", "feature", "story", "bug"}

// Item is one planned work item: a one-line fact with a link, never a copy of
// the issue.
type Item struct {
	Number int    `json:"number"`
	Type   string `json:"type"`
	Title  string `json:"title"`
	Effort string `json:"effort"` // the effort: label value, "" when unsized
	Points int    `json:"points"` // effort points; Stories and Bugs only
	Parent int    `json:"parent"` // parent issue number, 0 when none
}

// Draft is an agent's snapshot of the plan for a person to accept.
type Draft struct {
	Schema       string  `json:"schema"`
	Version      string  `json:"version"`
	ReleaseIssue int     `json:"release_issue"`
	DraftedAt    string  `json:"drafted_at"`
	DraftedBy    string  `json:"drafted_by"`
	Intent       string  `json:"intent"`
	BudgetPct    float64 `json:"budget_pct"`
	Points       int     `json:"points"`
	Hash         string  `json:"hash"` // Hash(intent, items): what the person accepts
	// EffortPoints is the effort table the plan was drafted with: the person
	// accepts it too, even a weight for a size no item carries yet.
	EffortPoints []int  `json:"effort_points"`
	Items        []Item `json:"items"`
}

// Baseline is a plan a person accepted: the release's scope freeze.
type Baseline struct {
	Schema       string  `json:"schema"`
	Version      string  `json:"version"`
	ReleaseIssue int     `json:"release_issue"`
	Baseline     int     `json:"baseline"` // n, from 1
	BaselinedAt  string  `json:"baselined_at"`
	BaselinedBy  string  `json:"baselined_by"`
	Intent       string  `json:"intent"`
	BudgetPct    float64 `json:"budget_pct"`
	Points       int     `json:"points"`
	Hash         string  `json:"hash"`
	Decision     int     `json:"decision"` // the Decision that let an over-budget re-baseline through, else 0
	// EffortPoints is the effort table at acceptance: drift is measured with
	// it, so a later change to [scope] effort_points cannot hide drift.
	EffortPoints []int  `json:"effort_points"`
	Items        []Item `json:"items"`
}

// Resize is a planned item whose effort changed since the baseline.
type Resize struct {
	Number int    `json:"number"`
	Type   string `json:"type"`
	Title  string `json:"title"`
	From   string `json:"from"`
	To     string `json:"to"`
	// FromType is the type at the baseline when the item changed type, so a
	// retyped Story (5 points to an Initiative's 0) is drift, not invisible.
	FromType string `json:"from_type,omitempty"`
}

// Drift is the plan measured against the newest baseline.
type Drift struct {
	Schema             string   `json:"schema"`
	Version            string   `json:"version"`
	ReleaseIssue       int      `json:"release_issue"`
	Baseline           int      `json:"baseline"`
	MeasuredAt         string   `json:"measured_at"`
	BudgetPct          float64  `json:"budget_pct"`
	DriftPct           float64  `json:"drift_pct"`
	BaselinePoints     int      `json:"baseline_points"`
	ChangedPoints      int      `json:"changed_points"`
	OverBudgetDecision int      `json:"over_budget_decision"` // the Decision that let the plan go over budget, else 0
	Added              []Item   `json:"added"`
	Removed            []Item   `json:"removed"`
	Resized            []Resize `json:"resized"`
}

// GetRelease is the Release issue a record belongs to, 0 for none.
func (b *Baseline) GetRelease() int {
	if b == nil {
		return 0
	}
	return b.ReleaseIssue
}

// GetRelease is the Release issue a draft belongs to, 0 for none.
func (d *Draft) GetRelease() int {
	if d == nil {
		return 0
	}
	return d.ReleaseIssue
}

// GetRelease is the Release issue a drift record belongs to, 0 for none.
func (d *Drift) GetRelease() int {
	if d == nil {
		return 0
	}
	return d.ReleaseIssue
}

// Over reports whether the drift is beyond its budget.
func (d Drift) Over() bool {
	if d.BaselinePoints > 0 { // the exact ratio: DriftPct is rounded for people to read
		return float64(d.ChangedPoints)*100/float64(d.BaselinePoints) > d.BudgetPct
	}
	return d.DriftPct > d.BudgetPct
}

// DraftPath, BaselinePath, and DriftPath are the ledger paths for version.
func DraftPath(version string) string { return "releases/" + version + "/draft.json" }

// BaselinePath is the ledger path of baseline n.
func BaselinePath(version string, n int) string {
	return fmt.Sprintf("releases/%s/baseline-%d.json", version, n)
}

// DriftPath is the ledger path of the drift record.
func DriftPath(version string) string { return "releases/" + version + "/drift.json" }

var baselineFile = regexp.MustCompile(`/baseline-([1-9][0-9]*)\.json$`)

// Newest returns the highest baseline number among ledger paths, 0 when none.
func Newest(paths []string) int {
	n := 0
	for _, p := range paths {
		if m := baselineFile.FindStringSubmatch(p); m != nil {
			v, _ := strconv.Atoi(m[1])
			n = max(n, v)
		}
	}
	return n
}

// PointsFor is the effort points of an item of type typ: Stories and Bugs
// carry the plan's weight; the containers above them carry none, so a
// Feature counts through the Stories under it. table holds the points of
// XS, S, M, L, and XL, in that order.
func PointsFor(typ, effort string, table []int) int {
	if typ != "story" && typ != "bug" {
		return 0
	}
	i := slices.Index(workitems.Efforts, effort)
	if i < 0 || i >= len(table) {
		return 0
	}
	return table[i]
}

// Total sums the items' points.
func Total(items []Item) int {
	t := 0
	for _, it := range items {
		t += it.Points
	}
	return t
}

// Problems are what keep a plan from being drafted as a baseline: it is not
// broken down (a Feature with no Story or Bug in the plan), not sized (a
// Story or Bug without one known effort, or an XL that must be sliced first), or it
// has no intent or no items.
func Problems(intent string, items []Item) []string {
	var out []string
	if strings.TrimSpace(intent) == "" {
		out = append(out, "the Release has no Summary: write the release's intent there first")
	}
	if !slices.ContainsFunc(items, func(it Item) bool { return it.Type == "story" || it.Type == "bug" }) {
		out = append(out, "the plan has no Story or Bug: add the work itself with --add, not only its Initiatives and Features")
	}
	children := map[int]bool{}
	for _, it := range items {
		if it.Type == "story" || it.Type == "bug" { // only executable work breaks a Feature down
			children[it.Parent] = true
		}
	}
	for _, it := range items {
		switch {
		case (it.Type == "story" || it.Type == "bug") && it.Effort == "":
			out = append(out, fmt.Sprintf("#%d (%s) has no effort: label: size it XS-L", it.Number, it.Type))
		case (it.Type == "story" || it.Type == "bug") && it.Effort == "XL":
			out = append(out, fmt.Sprintf("#%d (%s) is XL: slice it before it joins a baseline (aayu slice %d)", it.Number, it.Type, it.Number))
		case (it.Type == "story" || it.Type == "bug") && !slices.Contains(workitems.Efforts, it.Effort): // two labels, or one scope cannot weigh
			out = append(out, fmt.Sprintf("#%d (%s) has effort %s: keep one effort: label, XS-L", it.Number, it.Type, it.Effort))
		case it.Type == "feature" && !children[it.Number]:
			out = append(out, fmt.Sprintf("Feature #%d has no Story or Bug in the plan: slice it (aayu slice %d) and add its stories", it.Number, it.Number))
		}
	}
	return out
}

// Hash is the identity of a plan: its intent and its items, sorted by number.
// A person accepts exactly the plan an agent drafted when the hashes match.
func Hash(intent string, items []Item) string {
	sorted := slices.Clone(items)
	slices.SortFunc(sorted, func(a, b Item) int { return a.Number - b.Number })
	b, _ := json.Marshal(struct {
		Intent string `json:"intent"`
		Items  []Item `json:"items"`
	}{strings.TrimSpace(intent), sorted})
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// Measure compares the plan's items with the baseline's. Changed points are
// the points of every item added or removed plus the change in points of
// every item resized; drift is changed points as a percentage of the
// baseline's points, rounded to one decimal. Cutting scope is drift too: a
// slip that moves half the plan to the next release needs a person's call.
// The baseline's own effort table weighs both sides, so a later change to
// [scope] effort_points neither shows nor hides drift; table is the fallback
// for a baseline that recorded none.
func Measure(b Baseline, items []Item, table []int) Drift {
	if len(b.EffortPoints) == len(workitems.Efforts) {
		table = b.EffortPoints
	}
	d := Drift{Schema: DriftSchema, Version: b.Version, ReleaseIssue: b.ReleaseIssue, Baseline: b.Baseline, BudgetPct: b.BudgetPct,
		Added: []Item{}, Removed: []Item{}, Resized: []Resize{}}
	was := map[int]Item{}
	for _, it := range b.Items {
		it.Points = PointsFor(it.Type, it.Effort, table)
		was[it.Number] = it
		d.BaselinePoints += it.Points
	}
	now := map[int]bool{}
	for _, it := range items {
		it.Points = PointsFor(it.Type, it.Effort, table)
		now[it.Number] = true
		old, ok := was[it.Number]
		switch {
		case !ok:
			d.Added = append(d.Added, it)
			d.ChangedPoints += it.Points
		case old.Effort != it.Effort || old.Type != it.Type:
			z := Resize{Number: it.Number, Type: it.Type, Title: it.Title, From: old.Effort, To: it.Effort}
			if old.Type != it.Type {
				z.FromType = old.Type
			}
			d.Resized = append(d.Resized, z)
			d.ChangedPoints += abs(it.Points - old.Points)
		}
	}
	for _, it := range b.Items {
		if !now[it.Number] {
			it.Points = was[it.Number].Points
			d.Removed = append(d.Removed, it)
			d.ChangedPoints += it.Points
		}
	}
	slices.SortFunc(d.Added, func(a, b Item) int { return a.Number - b.Number })
	slices.SortFunc(d.Removed, func(a, b Item) int { return a.Number - b.Number })
	slices.SortFunc(d.Resized, func(a, b Resize) int { return a.Number - b.Number })
	switch {
	case d.BaselinePoints > 0:
		d.DriftPct = math.Round(float64(d.ChangedPoints)*1000/float64(d.BaselinePoints)) / 10
	case d.ChangedPoints > 0:
		d.DriftPct = 100 // a baseline with no points: any weighted change is all of it
	}
	return d
}

// BudgetOK reports whether pct is a usable change budget: a number from 0 to
// 100. NaN would make every Over false and 200 would let a plan be replaced.
func BudgetOK(pct float64) bool { return pct >= 0 && pct <= 100 }

// EffortPointsOK reports whether p is a usable effort table: one value of at
// least 1 per effort, in workitems.Efforts order. A 0 would hide every change
// to items of that effort.
func EffortPointsOK(p []int) bool { return len(p) == len(workitems.Efforts) && slices.Min(p) >= 1 }

// VersionRE is the version a planned release carries.
var VersionRE = regexp.MustCompile(`^v([0-9]+)\.([0-9]+)\.([0-9]+)$`)

// Next is the lowest version among titles that is later than this, "" when
// none: the release a deferred item rides.
func Next(this string, titles []string) string {
	key := func(v string) []int {
		m := VersionRE.FindStringSubmatch(v)
		if m == nil {
			return nil
		}
		out := make([]int, 3)
		for i := range out {
			out[i], _ = strconv.Atoi(m[i+1])
		}
		return out
	}
	cur := key(this)
	best, bestKey := "", []int(nil)
	for _, t := range titles {
		k := key(t)
		if k == nil || cur == nil || slices.Compare(k, cur) <= 0 {
			continue
		}
		if bestKey == nil || slices.Compare(k, bestKey) < 0 {
			best, bestKey = t, k
		}
	}
	return best
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
