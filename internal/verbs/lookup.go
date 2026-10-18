package verbs

import "strings"

// FindSeat returns the seat with the given ID.
func FindSeat(id SeatID) (Seat, bool) {
	for _, s := range Seats {
		if s.ID == id {
			return s, true
		}
	}
	return Seat{}, false
}

// FindLayer returns the layer with the given ID.
func FindLayer(id LayerID) (Layer, bool) {
	for _, l := range LayerTable {
		if l.ID == id {
			return l, true
		}
	}
	return Layer{}, false
}

// FindLane returns the lane with the given ID.
func FindLane(id LaneID) (Lane, bool) {
	for _, l := range Lanes {
		if l.ID == id {
			return l, true
		}
	}
	return Lane{}, false
}

// FindHost returns the host with the given ID.
func FindHost(id HostID) (Host, bool) {
	for _, h := range Hosts {
		if h.ID == id {
			return h, true
		}
	}
	return Host{}, false
}

// FindGate returns the gate with the given ID.
func FindGate(id GateID) (Gate, bool) {
	for _, g := range Gates {
		if g.ID == id {
			return g, true
		}
	}
	return Gate{}, false
}

// Find returns the verb or plumbing command with the given name.
func Find(name string) (Command, bool) {
	for _, c := range Verbs {
		if c.Name == name {
			return c, true
		}
	}
	for _, c := range Plumbing {
		if c.Name == name {
			return c, true
		}
	}
	return Command{}, false
}

// SeatName returns the display name of a seat, or the raw ID if it is unknown.
func SeatName(id SeatID) string {
	if s, ok := FindSeat(id); ok {
		return s.Name
	}
	return string(id)
}

// String renders a footprint as "all", "none", or a comma-separated list of layer IDs.
func (l Layers) String() string {
	switch l.Scope {
	case ScopeAll:
		return "all"
	case ScopeSome:
		ids := make([]string, len(l.IDs))
		for i, id := range l.IDs {
			ids[i] = string(id)
		}
		return strings.Join(ids, ", ")
	default:
		return "none"
	}
}

// FamilyOf infers a model's vendor family from its ID, the way the gateway's
// owned_by reports it. The family is a property of the model, never of the
// endpoint that serves it. It returns "" for an unrecognised ID.
func FamilyOf(modelID string) Family {
	id := strings.ToLower(modelID)
	// A gateway-qualified ID (OpenRouter's anthropic/claude-…) names the model after its last slash.
	if i := strings.LastIndex(id, "/"); i >= 0 {
		id = id[i+1:]
	}
	switch {
	case strings.HasPrefix(id, "claude-"):
		return FamilyAnthropic
	case strings.HasPrefix(id, "gpt-"), strings.HasPrefix(id, "o1"), strings.HasPrefix(id, "o3"), strings.HasPrefix(id, "o4"):
		return FamilyOpenAI
	case strings.HasPrefix(id, "gemini-"):
		return FamilyGoogle
	case strings.HasPrefix(id, "grok-"):
		return FamilyXAI
	}
	return ""
}

// CopilotSelector reports whether id is a Copilot selector (Auto, an Auto tier,
// or HydraFusion) rather than a model: the family is whichever model Copilot
// reports for the response.
func CopilotSelector(id string) bool {
	switch strings.ToLower(id) {
	case "auto", CopilotAutoEfficiency, CopilotAutoBalance, CopilotAutoIntelligence, CopilotHydraFusion:
		return true
	}
	return false
}
