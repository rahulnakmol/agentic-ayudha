package design

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
)

// CheckFlag checks that an OpenFeature flag definition file (flagd JSON,
// https://flagd.dev/reference/flag-definitions/) declares key as a boolean
// flag that is off by default: the prototype ships dark, and an environment
// turns it on through targeting or an override.
func CheckFlag(defs []byte, key string) error {
	var f struct {
		Flags map[string]struct {
			State          string                     `json:"state"`
			Variants       map[string]json.RawMessage `json:"variants"`
			DefaultVariant string                     `json:"defaultVariant"`
		} `json:"flags"`
	}
	if err := json.Unmarshal(defs, &f); err != nil {
		return fmt.Errorf("flag definitions: %w", err)
	}
	d, ok := f.Flags[key]
	if !ok {
		return fmt.Errorf("flag %q is not defined", key)
	}
	if d.State != "ENABLED" && d.State != "DISABLED" {
		return fmt.Errorf("flag %q: state %q, want ENABLED or DISABLED", key, d.State)
	}
	if len(d.Variants) < 2 {
		return fmt.Errorf("flag %q: want an on and an off variant", key)
	}
	for name, v := range d.Variants {
		var b bool
		if json.Unmarshal(v, &b) != nil {
			return fmt.Errorf("flag %q: variant %q is not a boolean", key, name)
		}
	}
	def, ok := d.Variants[d.DefaultVariant]
	if !ok {
		return fmt.Errorf("flag %q: defaultVariant %q is not a variant", key, d.DefaultVariant)
	}
	if string(def) != "false" {
		return fmt.Errorf("flag %q is on by default (defaultVariant %q); a prototype must be off by default", key, d.DefaultVariant)
	}
	return nil
}

var figmaURL = regexp.MustCompile(`https://(?:www\.)?figma\.com/(?:design|file)/([A-Za-z0-9]+)/[^\s"'` + "`" + `)]*?node-id=([0-9]+)(?:-|:|%3[Aa])([0-9]+)`)

// CodeConnect returns the Figma nodes (file key → node IDs as 1:2) that Code
// Connect files map to code: every figma.com design URL with a node-id in
// figma.connect(...) (React, HTML), @FigmaConnect (Compose), or FigmaConnect
// (SwiftUI) sources.
func CodeConnect(sources map[string][]byte) map[string][]string {
	out := map[string][]string{}
	for _, b := range sources {
		for _, m := range figmaURL.FindAllStringSubmatch(string(b), -1) {
			id := m[2] + ":" + m[3]
			if !slices.Contains(out[m[1]], id) {
				out[m[1]] = append(out[m[1]], id)
			}
		}
	}
	for k := range out {
		sort.Strings(out[k])
	}
	return out
}

// Unmapped lists the nodes of file that no Code Connect source maps.
func Unmapped(mapped map[string][]string, file string, nodes []string) []string {
	var out []string
	for _, n := range nodes {
		if !slices.Contains(mapped[file], n) {
			out = append(out, n)
		}
	}
	return out
}

// GateResult is the figma-diff gate for one prototype state.
type GateResult struct {
	State      string     `json:"state"`
	Node       string     `json:"figma_node"`
	Diff       PixelDiff  `json:"pixel_diff"`
	MaxPercent float64    `json:"max_percent"`
	Colour     []Mismatch `json:"colour_mismatches"`
	Typography []Mismatch `json:"typography_mismatches"`
	Error      string     `json:"error,omitempty"`
	Pass       bool       `json:"pass"`
}

// Judge applies the gate: the pixel difference is at most maxPercent, and
// there are no colour or typography token mismatches whatever the pixels say.
func (g *GateResult) Judge(maxPercent float64) {
	g.MaxPercent = maxPercent
	if g.Colour == nil {
		g.Colour = []Mismatch{}
	}
	if g.Typography == nil {
		g.Typography = []Mismatch{}
	}
	g.Pass = g.Error == "" && g.Diff.Percent <= maxPercent && len(g.Colour) == 0 && len(g.Typography) == 0
}

// Summary is one line per state for the CLI.
func (g GateResult) Summary() string {
	verdict := "pass"
	if !g.Pass {
		verdict = "FAIL"
	}
	if g.Error != "" {
		return fmt.Sprintf("%s (figma %s): %s: %s", g.State, g.Node, verdict, g.Error)
	}
	s := fmt.Sprintf("%s (figma %s): %s: %.2f %% of pixels differ (max %.2f %%, %d anti-aliased masked); %d colour and %d typography token mismatches",
		g.State, g.Node, verdict, g.Diff.Percent, g.MaxPercent, g.Diff.AntiAliased, len(g.Colour), len(g.Typography))
	var ms []string
	for _, m := range append(append([]Mismatch(nil), g.Colour...), g.Typography...) {
		ms = append(ms, m.String())
	}
	if len(ms) > 0 {
		s += ": " + strings.Join(first(ms, 5), "; ")
	}
	return s
}

func first(xs []string, n int) []string {
	if len(xs) > n {
		return append(slices.Clone(xs[:n]), fmt.Sprintf("and %d more", len(xs)-n))
	}
	return xs
}
