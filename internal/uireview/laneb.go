package uireview

import (
	"fmt"
	"sort"
	"strings"
)

// axeKinds maps axe rules to finding kinds; other rules are "other".
var axeKinds = map[string]string{
	"color-contrast": "contrast", "color-contrast-enhanced": "contrast", "target-size": "target-size",
	"button-name": "labels", "label": "labels", "link-name": "labels", "select-name": "labels", "image-alt": "labels",
	"aria-input-field-name": "labels", "input-button-name": "labels", "scrollable-region-focusable": "keyboard",
}

var axeSeverity = map[string]int{"critical": 4, "serious": 3, "moderate": 2, "minor": 1}

// impeccableKinds maps impeccable rules to finding kinds; other rules keep
// their place as "other".
var impeccableKinds = map[string]string{
	"low-contrast": "contrast", "gray-on-color": "contrast", "small-touch-target": "target-size", "tiny-text": "hierarchy",
	"flat-type-hierarchy": "hierarchy", "cramped-padding": "layout", "nested-cards": "layout", "everything-centered": "layout",
}

// LaneB turns the deterministic tools' output into lane B's critique. Every
// finding counts (lane B is deterministic: 1 of 1 run). A state whose
// capture or scan failed makes the lane recapture.
func LaneB(caps []Capture, imp map[string][]ImpeccableHit, impErr map[string]error, tools string) Lane {
	l := Lane{Name: "B", Disposition: Ship, Findings: []Finding{}, Tools: tools}
	for _, c := range caps {
		if c.Error != "" {
			l.Errors = append(l.Errors, fmt.Sprintf("%s: capture: %s", c.ID, c.Error))
			continue
		}
		if err := impErr[c.ID]; err != nil {
			l.Errors = append(l.Errors, fmt.Sprintf("%s: impeccable: %v", c.ID, err))
		}
		for _, h := range imp[c.ID] {
			if h.Advisory {
				continue
			}
			kind := impeccableKinds[h.Antipattern]
			if kind == "" {
				kind = "other"
			}
			sev := 2
			if h.Severity == "error" {
				sev = 3
			}
			l.Findings = append(l.Findings, Finding{State: c.ID, Kind: kind, Severity: sev, Element: h.Selector,
				Summary: strings.TrimSpace(h.Name + ": " + h.Snippet), Source: "impeccable:" + h.Antipattern})
		}
		for _, v := range c.Axe {
			kind := axeKinds[v.ID]
			if kind == "" {
				kind = "other"
			}
			l.Findings = append(l.Findings, Finding{State: c.ID, Kind: kind, Severity: max(axeSeverity[v.Impact], 1), Element: v.Target,
				Summary: fmt.Sprintf("%s (%s, %d node(s))", v.Help, v.Impact, v.Nodes), Source: "axe:" + v.ID})
		}
		for _, x := range c.FocusMissing {
			l.Findings = append(l.Findings, Finding{State: c.ID, Kind: "focus-visible", Severity: 3, Element: x,
				Summary: "keyboard focus changes nothing visible on " + x, Fix: "give :focus-visible an outline in the focus token colour", Source: "probe:focus-indicator"})
		}
		for _, x := range c.SmallTargets {
			l.Findings = append(l.Findings, Finding{State: c.ID, Kind: "target-size", Severity: 2, Element: x,
				Summary: "target smaller than the floor: " + x, Fix: "make the control at least the [verify.floors.a11y] min_target_px square", Source: "probe:target-size"})
		}
	}
	sort.SliceStable(l.Findings, func(i, j int) bool { return l.Findings[i].Key() < l.Findings[j].Key() })
	switch {
	case len(l.Errors) > 0:
		l.Disposition = Recapture
	case len(l.Findings) > 0:
		l.Disposition = Fix
	}
	return l
}
