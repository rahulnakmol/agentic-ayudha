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
