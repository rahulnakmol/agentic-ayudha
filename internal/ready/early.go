package ready

import "fmt"

// Early builds (Feature #874, core design E1): before a Feature's Approve to
// build, a person may approve one story of it, labelled early-build, so that
// a design can run while nothing else spends build tokens
// (`aayu ready <f> --early-build <story>`). The approval covers that story
// only, at its body hash, within a budget the person saw. Its PR never
// merges into dev.

// EarlyBuildLabel flags a Feature's story as an early build.
const EarlyBuildLabel = "early-build"

// EarlyBuildSchema is the schema of an early-build approval record.
const EarlyBuildSchema = "aayudha.feature.early-build/v1"

// EarlyBuild is a person's approval of one early build of a Feature.
type EarlyBuild struct {
	Schema     string  `json:"schema"`
	Feature    int     `json:"feature"`
	Story      int     `json:"story"`     // the one story the approval covers
	BodyHash   string  `json:"body_hash"` // workitems.BodyHash of the story's body when it was approved
	BudgetUSD  float64 `json:"budget_usd"`
	ApprovedBy string  `json:"approved_by"`
	Presence   string  `json:"presence"` // how the person was proven present: device-flow
	At         string  `json:"at"`
}

// EarlyBuildPath is where the approval of story's early build of Feature f
// lives on the ledger. The path names the story, so a record covers no other.
func EarlyBuildPath(f, story int) string {
	return fmt.Sprintf("specs/%d/approved-early-%d.json", f, story)
}

// EarlyBuildBudget is the budget of one early build: [baselines]
// early_build_budget times [autonomy.budget] cost_usd.
func EarlyBuildBudget(times int, costUSD float64) float64 { return float64(times) * costUSD }

// EarlyBuildEffect is what a person approves, shown before the device code.
func EarlyBuildEffect(f, story, times int, costUSD float64) string {
	return fmt.Sprintf("Approve the early build #%d of #%d. Budget up to $%.2f (%d × [autonomy.budget] cost_usd).",
		story, f, EarlyBuildBudget(times, costUSD), times)
}

// EarlyBuildHold is why inspect holds, and ship refuses, a PR of an early build.
func EarlyBuildHold(story int) string {
	return fmt.Sprintf("#%d is an early build: its PR never merges into dev (E1)", story)
}
