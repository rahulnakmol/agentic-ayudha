package measure

import (
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
)

// FounderMetric is the metric `aayu measure` reads from aayu's own records
// (Story #989, Decision #981): founder minutes per merged PR.
const FounderMetric = "founder_minutes_per_pr"

// Founder measures founder minutes per merged PR in w. Only the records of
// the human identity inside w count: bot and human-fallback records are not
// a person's minutes. Sorted by time, they form sessions: a gap of more than
// o.SessionGapMinutes starts a new one, and each session counts its span
// plus o.ActionMinutes for its last action. The sum is divided by merged,
// the pull requests merged into dev in w, and rounded to a tenth of a minute.
// It is a proxy: time a person spends outside aayu is not recorded.
func Founder(records []actions.Record, merged int, w Window, o config.Outcomes, dev string) (float64, string, error) {
	if merged == 0 {
		return 0, "", fmt.Errorf("%w: no merged pull request in the window; founder minutes per merged PR is undefined", ErrNoData)
	}
	var at []time.Time
	for _, r := range records {
		if r.Identity == "human" && !r.Time.Before(w.From) && r.Time.Before(w.To) {
			at = append(at, r.Time)
		}
	}
	slices.SortFunc(at, time.Time.Compare)
	gap, action := time.Duration(o.SessionGapMinutes)*time.Minute, time.Duration(o.ActionMinutes)*time.Minute
	var total time.Duration
	sessions := 0
	for i, t := range at {
		if i == 0 || t.Sub(at[i-1]) > gap {
			sessions++
			total += action
			continue
		}
		total += t.Sub(at[i-1])
	}
	minutes := total.Minutes()
	return math.Round(minutes/float64(merged)*10) / 10,
		fmt.Sprintf("Founder minutes per merged PR: %s founder minutes in %d sessions of %d human-identity actions in the actions log, over %d pull requests merged into %s "+
			"(a gap of more than [outcomes] session_gap_minutes = %d starts a session; each session adds [outcomes] action_minutes = %d)",
			num(minutes), sessions, len(at), merged, dev, o.SessionGapMinutes, o.ActionMinutes), nil
}
