package uireview

import (
	"context"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/models"
)

// Tools capture states and run impeccable (Browser in production).
type Tools interface {
	Capture(ctx context.Context, states []State) ([]Capture, error)
	Impeccable(ctx context.Context, url string) ([]ImpeccableHit, string, error)
}

// Reviewer runs both lanes on a set of states.
type Reviewer struct {
	Tools       Tools
	Complete    func(context.Context, models.Request) (models.Response, error) // lane A's model call (the ui-review lane through the gateway)
	Model       string
	Effort      string // medium; high with --deep
	K           int
	MinTargetPx int
	Now         func() time.Time
}

// Review captures the states once and runs the lanes on the same captures,
// each without the other's output.
func (r *Reviewer) Review(ctx context.Context, states []State) (*Critique, []Capture, error) {
	caps, err := r.Tools.Capture(ctx, states)
	if err != nil {
		return nil, nil, err
	}
	imp, impErr, version := map[string][]ImpeccableHit{}, map[string]error{}, ""
	for _, c := range caps {
		if c.Error != "" {
			continue
		}
		hits, v, err := r.Tools.Impeccable(ctx, c.URL)
		imp[c.ID], impErr[c.ID] = hits, err
		if v != "" {
			version = v
		}
	}
	crit := &Critique{SchemaVersion: 1, CreatedAt: r.Now().UTC().Format(time.RFC3339), States: states}
	crit.LaneB = LaneB(caps, imp, impErr, "impeccable "+version+"; axe-core; aayudha focus-indicator and target-size probes")
	k := r.K
	if k == 0 {
		k = K
	}
	if crit.LaneA, err = LaneA(ctx, r.Complete, caps, k, r.MinTargetPx, r.Effort, r.Model); err != nil {
		return crit, caps, err
	}
	crit.Combine()
	return crit, caps, nil
}
