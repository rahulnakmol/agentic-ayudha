package verbs

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// wantVerbs is the verb list from docs/BUILD-PLAN.md V7 / WP0.3, in plan order,
// then chart, the Data & AI Architect's verb (Decision #283, Appendix B S-5.1g),
// then survey (WP5.9, Decision #287), then the Production Engineer's verbs (WP5.10, Decision #324).
var wantVerbs = strings.Fields(`grill slice plan decide triage build verify inspect ship release proof ready
research wireframe mockup prototype review polish baseline experiment readout respond status audit retro
measure evolve brief tell init install doctor chart survey investigate slo runbook`)

// wantPlumbing is the plumbing list from docs/BUILD-PLAN.md V7, in plan order,
// then the three WP1.1b adds by founder request (Appendix B S-1.1b), with the
// founder's inbox (Story #431) before completion.
var wantPlumbing = strings.Fields(`config auth actions models mcp self-update upgrade version halt resume gc
uninit hook seat lfs-agent next tour inbox completion`)

func names(cs []Command) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.Name
	}
	return out
}

func TestRegistry(t *testing.T) {
	all := append(slices.Clone(Verbs), Plumbing...)

	checks := []struct {
		name  string
		check func() error
	}{
		{"32 verbs in plan order, then chart, survey, and the Production Engineer's", func() error {
			if got := names(Verbs); !slices.Equal(got, wantVerbs) || len(got) != 37 {
				return fmt.Errorf("verbs = %v, want %v", got, wantVerbs)
			}
			return nil
		}},
		{"15 plumbing commands in plan order, then the 3 of WP1.1b and inbox", func() error {
			if got := names(Plumbing); !slices.Equal(got, wantPlumbing) || len(got) != 19 {
				return fmt.Errorf("plumbing = %v, want %v", got, wantPlumbing)
			}
			return nil
		}},
		{"command names unique across verbs and plumbing", func() error {
			return unique(names(all))
		}},
		{"every verb in one journey stage, every stage used, plumbing in none", func() error {
			used := map[StageID]bool{}
			for _, s := range Stages {
				used[s.ID] = false
			}
			for _, c := range Verbs {
				if _, ok := used[c.Stage]; !ok {
					return fmt.Errorf("verb %s: stage %q is not in Stages", c.Name, c.Stage)
				}
				used[c.Stage] = true
			}
			for id, ok := range used {
				if !ok {
					return fmt.Errorf("stage %s has no verb", id)
				}
			}
			for _, c := range Plumbing {
				if c.Stage != "" || c.Reader != "" {
					return fmt.Errorf("plumbing %s has a stage or a reader summary", c.Name)
				}
			}
			return nil
		}},
		{"every verb has a plain reader summary, one sentence, distinct from its agent summary", func() error {
			for _, c := range Verbs {
				r := strings.TrimSpace(c.Reader)
				switch {
				case r == "":
					return fmt.Errorf("verb %s has no reader summary", c.Name)
				case len(r) > 160:
					return fmt.Errorf("verb %s: reader summary is %d chars, want <= 160", c.Name, len(r))
				case !strings.HasSuffix(r, ".") || strings.Count(r, ". ") > 0:
					return fmt.Errorf("verb %s: reader summary is not one sentence ending in a full stop", c.Name)
				case r == c.Summary:
					return fmt.Errorf("verb %s: reader summary repeats the agent summary", c.Name)
				}
			}
			return nil
		}},
		{"14 seats with unique IDs (the thirteenth is Decision #283, the fourteenth Decision #324)", func() error {
			ids := make([]string, len(Seats))
			for i, s := range Seats {
				ids[i] = string(s.ID)
				if s.Name == "" {
					return fmt.Errorf("seat %q has no name", s.ID)
				}
			}
			if len(Seats) != 14 {
				return fmt.Errorf("%d seats, want 14", len(Seats))
			}
			return unique(ids)
		}},
		{"layers L01-L09 in order, each with exactly one known owner", func() error {
			if len(LayerTable) != 9 {
				return fmt.Errorf("%d layers, want 9", len(LayerTable))
			}
			for i, l := range LayerTable {
				if want := LayerID(fmt.Sprintf("L%02d", i+1)); l.ID != want {
					return fmt.Errorf("layer %d is %q, want %q", i, l.ID, want)
				}
				if _, ok := FindSeat(l.Owner); !ok {
					return fmt.Errorf("layer %s owner %q is not a seat", l.ID, l.Owner)
				}
				if l.Name == "" || len(l.Paths) == 0 {
					return fmt.Errorf("layer %s needs a name and at least one path glob", l.ID)
				}
			}
			return nil
		}},
		{"every path glob belongs to exactly one layer", func() error {
			var globs []string
			for _, l := range LayerTable {
				globs = append(globs, l.Paths...)
			}
			return unique(globs)
		}},
		{"every command references known seats, lanes, layers, gates", func() error {
			for _, c := range all {
				if _, ok := FindSeat(c.Seat); !ok {
					return fmt.Errorf("%s: unknown seat %q", c.Name, c.Seat)
				}
				if _, ok := FindSeat(c.Owns); c.Owns != "" && !ok {
					return fmt.Errorf("%s: unknown threshold-owner seat %q", c.Name, c.Owns)
				}
				for _, id := range c.Layers.IDs {
					if _, ok := FindLayer(id); !ok {
						return fmt.Errorf("%s: unknown layer %q", c.Name, id)
					}
				}
				if (c.Layers.Scope == ScopeSome) != (len(c.Layers.IDs) > 0) {
					return fmt.Errorf("%s: layer scope %d inconsistent with IDs %v", c.Name, c.Layers.Scope, c.Layers.IDs)
				}
				for _, g := range c.Gates {
					if _, ok := FindGate(g); !ok {
						return fmt.Errorf("%s: unknown gate %q", c.Name, g)
					}
				}
			}
			for _, c := range Verbs {
				if _, ok := FindLane(c.Lane); !ok {
					return fmt.Errorf("verb %s: unknown lane %q", c.Name, c.Lane)
				}
			}
			for _, c := range Plumbing {
				if c.Lane != "" {
					return fmt.Errorf("plumbing %s has lane %q; plumbing has no lane", c.Name, c.Lane)
				}
			}
			return nil
		}},
		{"summaries non-empty, one line, <= 200 chars", func() error {
			for _, c := range all {
				if c.Summary == "" || len(c.Summary) > 200 || strings.ContainsAny(c.Summary, "\r\n") {
					return fmt.Errorf("%s: summary %q must be one non-empty line of <= 200 chars", c.Name, c.Summary)
				}
			}
			return nil
		}},
		{"enum fields hold known values", func() error {
			for _, c := range all {
				if !slices.Contains([]Identity{IdentityHuman, IdentityBot}, c.Identity) {
					return fmt.Errorf("%s: identity %q", c.Name, c.Identity)
				}
				if !slices.Contains([]Mode{ModeNone, ModeHost, ModeDirect, ModeHybrid}, c.Mode) {
					return fmt.Errorf("%s: mode %q", c.Name, c.Mode)
				}
				if !slices.Contains([]Exposure{ExposeRead, ExposeWrite, ExposeNever}, c.Exposure) {
					return fmt.Errorf("%s: exposure %q", c.Name, c.Exposure)
				}
			}
			return nil
		}},
		{"surfaces follow exposure and kind", func() error {
			for _, c := range Verbs {
				wantSkill := c.Name != "init" && c.Name != "install"
				switch {
				case !c.Has(SurfaceCLI):
					return fmt.Errorf("verb %s lacks the cli surface", c.Name)
				case c.Has(SurfaceSkill) != wantSkill:
					return fmt.Errorf("verb %s skill surface = %v, want %v", c.Name, c.Has(SurfaceSkill), wantSkill)
				case c.Has(SurfaceMCP) != (c.Exposure != ExposeNever):
					return fmt.Errorf("verb %s mcp surface disagrees with exposure %q", c.Name, c.Exposure)
				case c.Has(SurfacePlumbing):
					return fmt.Errorf("verb %s has the plumbing surface", c.Name)
				}
			}
			for _, c := range Plumbing {
				if !slices.Equal(c.Surfaces, []Surface{SurfaceCLI, SurfacePlumbing}) {
					return fmt.Errorf("plumbing %s surfaces = %v, want [cli plumbing]", c.Name, c.Surfaces)
				}
			}
			return nil
		}},
		{"inputs well-formed", func() error {
			for _, c := range all {
				var in []string
				for _, i := range c.Inputs {
					in = append(in, i.Name)
					if i.Description == "" {
						return fmt.Errorf("%s input %s has no description", c.Name, i.Name)
					}
					if (i.Type == TypeEnum) != (len(i.Enum) > 0) {
						return fmt.Errorf("%s input %s: enum values must be set exactly when type is enum", c.Name, i.Name)
					}
					if len(i.ReadOnlyValues) > 0 && c.Exposure != ExposeRead {
						return fmt.Errorf("%s input %s: read-only values only make sense on a read-exposed command", c.Name, i.Name)
					}
					for _, v := range i.ReadOnlyValues {
						if ok := (i.Type == TypeBool && (v == "true" || v == "false")) || (i.Type == TypeEnum && slices.Contains(i.Enum, v)); !ok {
							return fmt.Errorf("%s input %s: read-only value %q is not a value of the input", c.Name, i.Name, v)
						}
					}
					if i.HumanOnly && i.Required {
						return fmt.Errorf("%s input %s: a human-only input cannot be required (MCP never exposes it)", c.Name, i.Name)
					}
				}
				if err := unique(in); err != nil {
					return fmt.Errorf("%s: %w", c.Name, err)
				}
			}
			return nil
		}},
		{"every lane has a default per host and adversarial lanes differ from the builder family", func() error {
			build, _ := FindLane(LaneBuild)
			for _, l := range Lanes {
				for _, h := range Hosts {
					d, ok := defaultFor(l, h.ID)
					if !ok || d.Model == "" || d.Family == "" {
						return fmt.Errorf("lane %s has no default for host %s", l.ID, h.ID)
					}
					if d.Family != FamilyHostManaged && (d.ID == "" || FamilyOf(d.ID) != d.Family) {
						return fmt.Errorf("lane %s on %s: model ID %q does not belong to family %s", l.ID, h.ID, d.ID, d.Family)
					}
					if d.EscalationID != "" && FamilyOf(d.EscalationID) != d.Family {
						return fmt.Errorf("lane %s on %s: escalation %q changes family", l.ID, h.ID, d.EscalationID)
					}
					if slices.Contains(h.Excludes, d.Family) {
						return fmt.Errorf("lane %s on %s: default family %s is excluded on that host", l.ID, h.ID, d.Family)
					}
					b, _ := defaultFor(build, h.ID)
					if l.Adversarial && d.Family != FamilyHostManaged && d.Family == b.Family {
						return fmt.Errorf("lane %s on %s uses builder family %s", l.ID, h.ID, d.Family)
					}
				}
			}
			return nil
		}},
	}
	for _, tc := range checks {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.check(); err != nil {
				t.Error(err)
			}
		})
	}
}

func defaultFor(l Lane, h HostID) (ModelDefault, bool) {
	for _, d := range l.Defaults {
		if d.Host == h {
			return d, true
		}
	}
	return ModelDefault{}, false
}

func unique(xs []string) error {
	seen := map[string]bool{}
	for _, x := range xs {
		if seen[x] {
			return fmt.Errorf("duplicate %q", x)
		}
		seen[x] = true
	}
	return nil
}

// WP5.1 accept: the registry lint holds every verb to one Accountable seat
// with distinct Consulted and Informed seats, and every layer to one owner.
func TestLintRACI(t *testing.T) {
	if err := LintRACI(); err != nil {
		t.Fatalf("registry lint: %v", err)
	}
	breaks := []struct {
		name    string
		breakIt func() func()
		want    string
	}{
		{"verb without a RACI entry", func() func() {
			r := RACI["ship"]
			delete(RACI, "ship")
			return func() { RACI["ship"] = r }
		}, "ship: no RACI entry"},
		{"Accountable seat also Consulted", func() func() {
			r := RACI["plan"]
			RACI["plan"] = Roles{Consulted: []SeatID{CodeArchitect}}
			return func() { RACI["plan"] = r }
		}, "plan: code-architect is both Accountable and Consulted"},
		{"threshold owner also Consulted", func() func() {
			r := RACI["verify"]
			RACI["verify"] = Roles{Consulted: []SeatID{QualityArchitect}}
			return func() { RACI["verify"] = r }
		}, "verify: quality-architect is both the threshold owner and Consulted"},
		{"verb without an Accountable seat", func() func() {
			s := Verbs[0].Seat
			Verbs[0].Seat = ""
			return func() { Verbs[0].Seat = s }
		}, "grill: Accountable seat"},
		{"layer without an owner", func() func() {
			o := LayerTable[3].Owner
			LayerTable[3].Owner = ""
			return func() { LayerTable[3].Owner = o }
		}, "layer L04: owner"},
	}
	for _, b := range breaks {
		restore := b.breakIt()
		err := LintRACI()
		restore()
		if err == nil || !strings.Contains(err.Error(), b.want) {
			t.Errorf("%s: lint = %v, want an error containing %q", b.name, err, b.want)
		}
	}
}
