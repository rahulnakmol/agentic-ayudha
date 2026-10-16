package inspect

import (
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"
)

// Risk tiers of the proof comment's risk line.
const (
	RiskHigh   = "high"
	RiskMedium = "medium"
	RiskLow    = "low"
)

// Risk is the proof comment's risk line (P6): a deterministic tier and the
// files a human should read first. It approves nothing and changes no gate;
// it shortens the human pass.
type Risk struct {
	Tier    string   `json:"tier"`
	Reasons []string `json:"reasons"`
	Look    []string `json:"look"`
}

// Thresholds of a medium tier: several layers, or a large diff.
const (
	riskLayers = 3
	riskLines  = 400
	riskFiles  = 20
	riskLook   = 5
)

var (
	manifests = map[string]bool{"go.mod": true, "package.json": true, "requirements.txt": true, "pyproject.toml": true, "Pipfile": true,
		"Cargo.toml": true, "Gemfile": true, "pom.xml": true, "build.gradle": true, "build.gradle.kts": true, "composer.json": true}
	migration = regexp.MustCompile(`(^|/)(migrations?|migrate)/|\.sql$`)
)

// RiskOf computes the tier from the diff: high for a never-layer footprint, a
// protected path, a new dependency, or a migration; medium for a footprint of
// riskLayers layers or more, or a large diff; low otherwise. protected says
// whether a path is protected (owned by a specific CODEOWNERS rule).
func RiskOf(files []DiffFile, footprint Footprint, never []string, protected func(string) bool) Risk {
	r := Risk{Tier: RiskLow, Reasons: []string{}, Look: []string{}}
	look := func(p string) {
		if !slices.Contains(r.Look, p) && len(r.Look) < riskLook {
			r.Look = append(r.Look, p)
		}
	}
	high := map[string][]string{}
	var kinds []string
	flag := func(kind, p string) {
		if high[kind] == nil {
			kinds = append(kinds, kind)
		}
		if !slices.Contains(high[kind], p) {
			high[kind] = append(high[kind], p)
		}
		look(p)
	}
	var paths []string
	lines := 0
	size := map[string]int{}
	for _, f := range files {
		paths = append(paths, f.Path)
		for _, h := range f.Hunks {
			size[f.Path] += len(h.Removed)
			for _, l := range h.Lines {
				if l.Added {
					size[f.Path]++
				}
			}
		}
		lines += size[f.Path]
		if footprint != nil {
			for _, l := range footprint([]string{f.Path}) {
				if slices.Contains(never, l) {
					flag("never-layer "+l, f.Path)
				}
			}
		}
		if protected != nil && protected(f.Path) {
			flag("protected path", f.Path)
		}
		if manifests[path.Base(f.Path)] && !f.Deleted && addsLines(f) {
			flag("dependency manifest changed", f.Path)
		}
		if migration.MatchString(f.Path) && !f.Deleted {
			flag("migration", f.Path)
		}
	}
	for _, k := range kinds {
		r.Reasons = append(r.Reasons, fmt.Sprintf("%s: %s", k, strings.Join(high[k], ", ")))
	}
	if len(kinds) > 0 {
		r.Tier = RiskHigh
		return r
	}
	var layers []string
	if footprint != nil {
		layers = footprint(paths)
	}
	if len(layers) >= riskLayers {
		r.Reasons = append(r.Reasons, fmt.Sprintf("%d layers: %s", len(layers), strings.Join(layers, ", ")))
	}
	if lines > riskLines || len(files) > riskFiles {
		r.Reasons = append(r.Reasons, fmt.Sprintf("large diff: %d files, %d changed lines", len(files), lines))
	}
	if len(r.Reasons) > 0 {
		r.Tier = RiskMedium
	}
	// Look first at the largest changes.
	byFile := slices.Clone(paths)
	slices.SortStableFunc(byFile, func(a, b string) int { return size[b] - size[a] })
	for _, p := range byFile {
		look(p)
		if len(r.Look) >= 3 {
			break
		}
	}
	return r
}

func addsLines(f DiffFile) bool {
	for _, h := range f.Hunks {
		for _, l := range h.Lines {
			if l.Added && strings.TrimSpace(l.Text) != "" {
				return true
			}
		}
	}
	return false
}
