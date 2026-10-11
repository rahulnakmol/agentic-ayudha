package verbs

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// wantVerbs is the verb list from docs/BUILD-PLAN.md V7 / WP0.3, in plan order.
var wantVerbs = strings.Fields(`grill slice plan decide triage build verify inspect ship release proof ready
research wireframe mockup prototype review polish baseline experiment readout respond status audit retro
measure evolve brief tell init install doctor`)

// wantPlumbing is the plumbing list from docs/BUILD-PLAN.md V7, in plan order.
var wantPlumbing = strings.Fields(`config auth actions models mcp self-update upgrade version halt resume gc
uninit hook seat lfs-agent`)

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
		{"32 verbs in plan order", func() error {
			if got := names(Verbs); !slices.Equal(got, wantVerbs) || len(got) != 32 {
				return fmt.Errorf("verbs = %v, want %v", got, wantVerbs)
			}
			return nil
		}},
		{"15 plumbing commands in plan order", func() error {
			if got := names(Plumbing); !slices.Equal(got, wantPlumbing) || len(got) != 15 {
				return fmt.Errorf("plumbing = %v, want %v", got, wantPlumbing)
			}
			return nil
		}},
		{"command names unique across verbs and plumbing", func() error {
			return unique(names(all))
		}},
		{"12 seats with unique IDs", func() error {
			ids := make([]string, len(Seats))
			for i, s := range Seats {
				ids[i] = string(s.ID)
				if s.Name == "" {
					return fmt.Errorf("seat %q has no name", s.ID)
				}
			}
			if len(Seats) != 12 {
				return fmt.Errorf("%d seats, want 12", len(Seats))
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
