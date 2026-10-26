package verbs

import (
	"errors"
	"fmt"
	"slices"
)

// Roles completes a verb's RACI (WP5.1). The Accountable seat is Command.Seat
// and the seat that owns the verb's thresholds or policy is Command.Owns (the
// Accountable seat when empty). The Responsible party is always the Accountable
// seat's agent twin working in the verb's lane, with its paired human when the
// seat is paired: so `inspect` is owned by the Engineering Manager and executed
// by the review lane, and `verify` is executed by the AI Native Engineer against
// thresholds the Quality Architect owns in CONSTRAINTS.md.
type Roles struct {
	Consulted []SeatID // asked before the verb acts
	Informed  []SeatID // told after it acts
}

// RACI holds the Consulted and Informed seats of every verb. A verb with no
// entry fails LintRACI, so each verb's roles are a deliberate choice.
var RACI = map[string]Roles{
	"grill":      {Consulted: []SeatID{CodeArchitect, UXResearcher}, Informed: []SeatID{EngineeringManager}},
	"slice":      {Consulted: []SeatID{CodeArchitect, EngineeringManager}, Informed: []SeatID{AINativeEngineer}},
	"scope":      {Consulted: []SeatID{EngineeringManager, Founder}, Informed: []SeatID{DevOps, Marketer}},
	"plan":       {Consulted: []SeatID{EngineeringManager, QualityArchitect, DataAIArchitect, ProductionEngineer}, Informed: []SeatID{AINativeEngineer}},
	"decide":     {Consulted: []SeatID{ProductStrategist, CodeArchitect}, Informed: []SeatID{EngineeringManager, Auditor}},
	"counsel":    {Consulted: []SeatID{ProductStrategist, CodeArchitect, EngineeringManager}, Informed: []SeatID{Auditor}},
	"triage":     {Informed: []SeatID{EngineeringManager}},
	"brain":      {Consulted: []SeatID{Founder, RevenueArchitect}, Informed: []SeatID{Marketer}},
	"build":      {Consulted: []SeatID{CodeArchitect, ProductionEngineer}, Informed: []SeatID{EngineeringManager}},
	"verify":     {Informed: []SeatID{EngineeringManager}},
	"inspect":    {Consulted: []SeatID{QualityArchitect, CodeArchitect}, Informed: []SeatID{AINativeEngineer}},
	"ship":       {Consulted: []SeatID{QualityArchitect}, Informed: []SeatID{ProductStrategist, Marketer}},
	"mission":    {Consulted: []SeatID{ProductStrategist, AINativeEngineer}, Informed: []SeatID{Founder}},
	"release":    {Consulted: []SeatID{QualityArchitect, EngineeringManager, ProductionEngineer}, Informed: []SeatID{Marketer, Founder}},
	"proof":      {Consulted: []SeatID{AINativeEngineer}, Informed: []SeatID{Auditor}},
	"ready":      {Consulted: []SeatID{CodeArchitect, QualityArchitect}, Informed: []SeatID{EngineeringManager}},
	"research":   {Consulted: []SeatID{ProductStrategist}, Informed: []SeatID{Marketer}},
	"wireframe":  {Consulted: []SeatID{ProductStrategist}, Informed: []SeatID{EngineeringManager}},
	"mockup":     {Consulted: []SeatID{ProductStrategist}, Informed: []SeatID{EngineeringManager}},
	"prototype":  {Consulted: []SeatID{CodeArchitect}, Informed: []SeatID{EngineeringManager}},
	"review":     {Consulted: []SeatID{QualityArchitect}, Informed: []SeatID{EngineeringManager}},
	"polish":     {Informed: []SeatID{EngineeringManager}},
	"baseline":   {Consulted: []SeatID{QualityArchitect}, Informed: []SeatID{EngineeringManager}},
	"experiment": {Consulted: []SeatID{ProductStrategist, UXResearcher, DataAIArchitect}, Informed: []SeatID{Founder}},
	"readout":    {Consulted: []SeatID{ProductStrategist}, Informed: []SeatID{Founder, Marketer}},
	"respond":    {Consulted: []SeatID{AINativeEngineer, DevOps}, Informed: []SeatID{EngineeringManager, Founder}},
	"status":     {Informed: []SeatID{Founder}},
	"audit":      {Consulted: []SeatID{QualityArchitect}, Informed: []SeatID{Founder}},
	"retro":      {Consulted: []SeatID{Auditor, AINativeEngineer}, Informed: []SeatID{Founder}},
	"measure":    {Consulted: []SeatID{ProductStrategist}, Informed: []SeatID{Founder}},
	"evolve":     {Consulted: []SeatID{QualityArchitect, DataAIArchitect}, Informed: []SeatID{EngineeringManager}},
	"brief":      {Consulted: []SeatID{ProductStrategist}, Informed: []SeatID{Marketer}},
	"tell":       {Consulted: []SeatID{Mentalist, ProductStrategist}, Informed: []SeatID{Founder}},
	"init":       {Consulted: []SeatID{DevOps}, Informed: []SeatID{EngineeringManager}},
	"install":    {Informed: []SeatID{EngineeringManager}},
	"doctor":     {Informed: []SeatID{EngineeringManager}},
	"chart":      {Consulted: []SeatID{CodeArchitect, RevenueArchitect}, Informed: []SeatID{AINativeEngineer, QualityArchitect}},
	"survey":     {Consulted: []SeatID{ProductStrategist, QualityArchitect}, Informed: []SeatID{EngineeringManager}},
	// The Production Engineer's verbs (Decision #324): the builder is Consulted where its code is implicated.
	"investigate": {Consulted: []SeatID{AINativeEngineer}, Informed: []SeatID{EngineeringManager}},
	"slo":         {Consulted: []SeatID{ProductStrategist, QualityArchitect}, Informed: []SeatID{EngineeringManager, DevOps}},
	"runbook":     {Consulted: []SeatID{AINativeEngineer, DevOps}, Informed: []SeatID{EngineeringManager}},
}

// PolicyOwner is the seat that owns a command's thresholds or policy.
func PolicyOwner(c Command) SeatID {
	if c.Owns != "" {
		return c.Owns
	}
	return c.Seat
}

// LintRACI is the registry lint of WP5.1: every command has exactly one known
// Accountable seat, every verb has a RACI entry whose Consulted and Informed
// seats are known, unique, and distinct from each other and from the
// Accountable seat, and every layer has exactly one known owner seat. The
// generator runs it, so a registry that breaks it fails `go generate` and CI.
func LintRACI() error {
	var errs []error
	bad := func(format string, args ...any) { errs = append(errs, fmt.Errorf(format, args...)) }
	known := func(id SeatID) bool { _, ok := FindSeat(id); return ok }
	for _, c := range append(slices.Clone(Verbs), Plumbing...) {
		if !known(c.Seat) {
			bad("%s: Accountable seat %q is not one of the fourteen seats", c.Name, c.Seat)
		}
		if c.Owns != "" && !known(c.Owns) {
			bad("%s: threshold owner %q is not a seat", c.Name, c.Owns)
		}
	}
	for _, c := range Verbs {
		r, ok := RACI[c.Name]
		if !ok {
			bad("%s: no RACI entry (add its Consulted and Informed seats to verbs.RACI)", c.Name)
			continue
		}
		seen := map[SeatID]string{c.Seat: "Accountable"}
		if c.Owns != "" {
			seen[c.Owns] = "the threshold owner"
		}
		for _, role := range []struct {
			name  string
			seats []SeatID
		}{{"Consulted", r.Consulted}, {"Informed", r.Informed}} {
			for _, s := range role.seats {
				if !known(s) {
					bad("%s: %s seat %q is not a seat", c.Name, role.name, s)
				}
				if prev, dup := seen[s]; dup {
					bad("%s: %s is both %s and %s", c.Name, s, prev, role.name)
				}
				seen[s] = role.name
			}
		}
	}
	for name := range RACI {
		if c, ok := Find(name); !ok || c.Has(SurfacePlumbing) {
			bad("RACI entry %q is not a verb", name)
		}
	}
	for _, l := range LayerTable {
		if !known(l.Owner) {
			bad("layer %s: owner %q is not a seat (every layer has exactly one owner seat)", l.ID, l.Owner)
		}
	}
	return errors.Join(errs...)
}

// LayersOwned lists the layers a seat owns, in table order.
func LayersOwned(id SeatID) []Layer {
	var out []Layer
	for _, l := range LayerTable {
		if l.Owner == id {
			out = append(out, l)
		}
	}
	return out
}
