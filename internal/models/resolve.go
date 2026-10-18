// Package models is the model gateway (WP1.6): lane resolution per host,
// provider clients for three wire formats behind one optional AI gateway,
// reviewer/builder family separation, cost and token accounting into the
// actions log, the [budget] circuit breaker, a fake provider for tests, and a
// structured-output helper with JSON Schema validation.
//
// The binary calls models directly only for host-independent lanes (ground
// rule 5); inside a host the host's model reasons.
package models

import (
	"fmt"
	"slices"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

// Resolution sources.
const (
	SourceDefault  = "default"  // the registry default for the lane and host
	SourceConfig   = "config"   // [models.<lane>.<host>]
	SourceFallback = "fallback" // configured family not available on the host; registry default used
	SourceDirect   = "direct"   // host-managed, or a Copilot catalog model, on this host; the binary's direct call uses DirectHost's row
)

// Resolved is the model a lane uses on a host.
type Resolved struct {
	Lane    verbs.LaneID `json:"lane"`
	Host    verbs.HostID `json:"host"`
	Model   string       `json:"model"`             // model ID; empty for a host-managed mode
	Display string       `json:"display,omitempty"` // registry display name or the host mode
	Family  verbs.Family `json:"family"`
	Effort  string       `json:"effort,omitempty"`
	Source  string       `json:"source"`
	Note    string       `json:"note,omitempty"` // substitution explanation, logged
}

// Resolver resolves lanes from [models] and the registry. Log receives one line
// per substitution.
type Resolver struct {
	Models config.Models
	Hosts  config.Hosts // the host plans that pick a lane's default (Decision #469)
	Log    func(string)
}

// plan is host's plan: [hosts.cursor] plan for Cursor, x when unset.
func (r Resolver) plan(host verbs.HostID) string {
	if host != verbs.HostCursor {
		return ""
	}
	if r.Hosts.Cursor.Plan == verbs.PlanLegacy {
		return verbs.PlanLegacy
	}
	return verbs.PlanX
}

func (r Resolver) logf(format string, args ...any) {
	if r.Log != nil {
		r.Log(fmt.Sprintf(format, args...))
	}
}

func registryDefault(lane verbs.LaneID, host verbs.HostID, plan string) (verbs.ModelDefault, error) {
	l, ok := verbs.FindLane(lane)
	if !ok {
		return verbs.ModelDefault{}, fmt.Errorf("unknown lane %q", lane)
	}
	for _, d := range l.Defaults {
		if d.Host == host && (d.Plan == "" || d.Plan == plan) {
			return d, nil
		}
	}
	return verbs.ModelDefault{}, fmt.Errorf("lane %s has no default for host %q", lane, host)
}

// Resolve returns the model lane uses on host: the [models.<lane>.<host>]
// override when its family is available on the host, else the registry default.
func (r Resolver) Resolve(lane verbs.LaneID, host verbs.HostID) (Resolved, error) {
	h, ok := verbs.FindHost(host)
	if !ok {
		return Resolved{}, fmt.Errorf("unknown host %q", host)
	}
	d, err := registryDefault(lane, host, r.plan(host))
	if err != nil {
		return Resolved{}, err
	}
	def := Resolved{Lane: lane, Host: host, Model: d.ID, Display: d.Model, Family: d.Family, Effort: d.Effort, Source: SourceDefault, Note: d.Note}
	c, ok := r.Models.Lanes[string(lane)][string(host)]
	if !ok {
		return def, nil
	}
	fam := verbs.Family(c.Family)
	if fam == "" {
		fam = verbs.FamilyOf(c.Model)
	}
	if fam == "" && host == verbs.HostCopilot && verbs.CopilotSelector(c.Model) {
		fam = verbs.FamilyHostManaged // Copilot reports the model per response
	}
	if fam == "" {
		return Resolved{}, fmt.Errorf("[models.%s.%s] model %q: unknown family; set family", lane, host, c.Model)
	}
	if slices.Contains(h.Excludes, fam) {
		def.Source = SourceFallback
		def.Note = fmt.Sprintf("%s (%s) is not available on %s; using the lane's %s default %s (%s)", c.Model, fam, h.Name, h.Name, d.ID, d.Family)
		r.logf("models: %s lane on %s: %s", lane, host, def.Note)
		return def, nil
	}
	return Resolved{Lane: lane, Host: host, Model: c.Model, Display: c.Model, Family: fam, Source: SourceConfig}, nil
}

// ResolveDirect resolves a lane for a call the binary makes itself. A
// host-managed mode cannot be called directly, so the DirectHost default is
// used instead; heavy selects the lane's escalation model and effort.
func (r Resolver) ResolveDirect(lane verbs.LaneID, host verbs.HostID, heavy bool) (Resolved, error) {
	res, err := r.Resolve(lane, host)
	if err != nil {
		return res, err
	}
	// Copilot's rows, its defaults and a person's [models.<lane>.copilot],
	// name models in Copilot's own catalog (claude-opus-5.5), which the
	// gateway does not route.
	copilotOnly := host == verbs.HostCopilot
	if res.Family == verbs.FamilyHostManaged || copilotOnly {
		alt, err := r.Resolve(lane, verbs.DirectHost)
		if err != nil {
			return alt, err
		}
		alt.Source = SourceDirect
		alt.Note = fmt.Sprintf("%s is a host-managed mode on %s; the direct call uses the %s default %s", res.Display, host, verbs.DirectHost, alt.Model)
		if copilotOnly && res.Family != verbs.FamilyHostManaged {
			alt.Note = fmt.Sprintf("%s is a model in Copilot's catalog; the direct call uses the %s default %s", res.Display, verbs.DirectHost, alt.Model)
		}
		r.logf("models: %s lane: %s", lane, alt.Note)
		res = alt
	}
	if heavy && res.Source != SourceConfig {
		if d, err := registryDefault(lane, res.Host, r.plan(res.Host)); err == nil && d.EscalationID != "" {
			res.Model, res.Display, res.Effort = d.EscalationID, d.Escalation, d.EscalationEffort
		}
	}
	return res, nil
}

// CheckFamilies enforces reviewer.family != builder.family for one story. A
// host-managed builder's family is unknown to the binary; its reviewer must
// then name a concrete family so the host-side model can differ from it.
func CheckFamilies(story string, builder, reviewer verbs.Family) error {
	switch {
	case reviewer == "" || reviewer == verbs.FamilyHostManaged:
		return fmt.Errorf("story %s: the reviewer needs a concrete model family, not %q", story, reviewer)
	case builder == reviewer:
		return fmt.Errorf("story %s: reviewer family %s equals builder family; %s", story, reviewer, verbs.ReviewerFamilyRule)
	}
	return nil
}
