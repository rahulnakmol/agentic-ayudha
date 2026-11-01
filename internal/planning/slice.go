// Package planning checks the drafts of slice and plan (WP5.4). The host's
// model reasons (the skills slice and plan); these checks are the
// deterministic part the binary owns: every story is XS-L and passes Ready
// under its Feature, story footprints come from the Feature's expected paths
// through the LAYERS.md globs, dependencies form no cycle, and a plan has
// exactly one checkpoint per footprint layer with a rollback for operational
// data (L07).
package planning

import (
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/verify"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// LayerLabel prefixes the labels that hold a story's layer footprint
// (layer:L02), written by slice and confirmed by plan.
const LayerLabel = "layer:"

// SliceDraft is the host's draft of a slice (evals/schemas/slice.json).
type SliceDraft struct {
	Feature       int          `json:"feature"`
	Stories       []DraftStory `json:"stories"`
	OpenQuestions []string     `json:"open_questions,omitempty"`
}

// DraftStory is one drafted story: its work-item fields, label-held fields,
// the paths it changes (a subset of the Feature's expected paths), and the
// stories of this draft it depends on, by 1-based position.
type DraftStory struct {
	Title     string            `json:"title"`
	Fields    map[string]string `json:"fields"`
	Labels    []string          `json:"labels"`
	Paths     []string          `json:"paths"`
	DependsOn []int             `json:"depends_on,omitempty"`
}

// Source is what is being sliced: a Feature, or an XL story (closed as split
// into the new stories, which go under its Feature).
type Source struct {
	Number        int
	Type          string // feature or story
	Effort        string
	Feature       int      // the Feature the stories go under
	ExpectedPaths []string // the Feature's Expected paths, one glob per line
}

// XL reports whether the source is an XL item that slice closes as split.
func (s Source) XL() bool { return s.Effort == "XL" }

// StoryReport is one drafted story's check.
type StoryReport struct {
	N         int                 `json:"n"`
	Title     string              `json:"title"`
	Effort    string              `json:"effort"`
	Footprint []string            `json:"footprint"`
	DependsOn []int               `json:"depends_on,omitempty"`
	Problems  []workitems.Problem `json:"problems"`
}

// SliceReport is the check of a whole draft.
type SliceReport struct {
	Source    int           `json:"source"`
	Feature   int           `json:"feature"`
	XL        bool          `json:"xl,omitempty"`
	Footprint []string      `json:"footprint"` // the Feature's, from its expected paths
	Stories   []StoryReport `json:"stories"`
	Problems  []string      `json:"problems"` // problems of the draft as a whole
	Order     []int         `json:"order"`    // positions in dependency order (creation order)
	Pass      bool          `json:"pass"`
}

// Item is a drafted story as the work item slice would create.
func (s DraftStory) Item(feature int) workitems.Item {
	labels := []string{"type:story", "status:planned"}
	for _, l := range s.Labels {
		if !strings.HasPrefix(l, "type:") && !strings.HasPrefix(l, "status:") && !strings.HasPrefix(l, LayerLabel) {
			labels = append(labels, l)
		}
	}
	fields := map[string]string{}
	for k, v := range s.Fields {
		fields[k] = v
	}
	return workitems.Item{Type: "story", Title: s.Title, Fields: fields, Labels: labels, Parent: &workitems.Ref{Number: feature, Type: "feature"}}
}

// GlobFootprint is the layers a set of path globs can touch: a layer counts
// when one of its globs overlaps one of the globs.
func GlobFootprint(l verify.LayerGlobs, globs []string) []string {
	var out []string
	for id, lgs := range l {
		hit := false
		for _, lg := range lgs {
			for _, g := range globs {
				if overlap(lg, g) {
					hit = true
				}
			}
		}
		if hit {
			out = append(out, id)
		}
	}
	slices.Sort(out)
	return out
}

// overlap reports whether two path globs share a path: one matches a sample
// path of the other (in either direction, so a broader glob covers a narrower
// one).
func overlap(a, b string) bool {
	for _, x := range forms(a) {
		for _, y := range forms(b) {
			if verify.Match(x, samples(y)[0]) || verify.Match(y, samples(x)[0]) {
				return true
			}
		}
	}
	return false
}

// within reports whether path glob p lies inside glob g: g matches every
// sample path of p. A parent directory of g (`internal/` for
// `internal/workitems/`) is not within it, nor is a glob that allows names g
// does not (`*_test.go` in `?_test.go`).
func within(p, g string) bool {
	for _, x := range forms(g) {
		for _, y := range forms(p) {
			if !slices.ContainsFunc(samples(y), func(s string) bool { return !verify.Match(x, s) }) {
				return true
			}
		}
	}
	return false
}

// forms is what a path glob stands for, after cleaning (so
// `internal/workitems/../cli/x.go` is `internal/cli/x.go`). A directory
// (`internal/workitems/`) stands for every path under it; a path without a
// wildcard and without the slash (`internal/workitems`) can be a file or a
// directory, so it stands for itself and every path under it. A sibling with a
// longer name (`internal/workitemsx/`) is in neither.
func forms(g string) []string {
	g = strings.TrimPrefix(strings.TrimSpace(strings.Trim(strings.TrimSpace(g), "`")), "/")
	dir := strings.HasSuffix(g, "/")
	g = path.Clean(g)
	if dir {
		return []string{g + "/**"}
	}
	if !strings.ContainsAny(g, "*?") {
		return []string{g, g + "/**"}
	}
	return []string{g}
}

// samples turns a glob into two concrete paths it matches: one with short
// wildcards, one with longer and different ones. A glob that matches both
// allows the other glob's names of either length.
func samples(g string) []string {
	fill := func(deep, star, one string) string {
		return strings.NewReplacer("**", deep, "*", star, "?", one).Replace(g)
	}
	return []string{fill("x/x", "x", "x"), fill("z/z/z", "zz", "z")}
}

// Paths splits an Expected paths field: one glob per line, or per comma
// outside backticks (a backtick-quoted glob keeps its commas), list bullets
// and backticks allowed.
func Paths(field string) []string {
	var out []string
	for _, l := range strings.Split(strings.ReplaceAll(field, "\r\n", "\n"), "\n") {
		l = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(l), "-*"))
		var parts []string
		quoted, from := false, 0
		for i, r := range l {
			switch {
			case r == '`':
				quoted = !quoted
			case r == ',' && !quoted:
				parts, from = append(parts, l[from:i]), i+1
			}
		}
		for _, p := range append(parts, l[from:]) {
			p = strings.Trim(strings.TrimSpace(p), "`")
			if p = strings.TrimSpace(p); p != "" && !workitems.Empty(p) {
				out = append(out, p)
			}
		}
	}
	return out
}

// CheckSlice checks a draft against its source and the layer globs.
func CheckSlice(src Source, d SliceDraft, layers verify.LayerGlobs) SliceReport {
	r := SliceReport{Source: src.Number, Feature: src.Feature, XL: src.XL(), Stories: []StoryReport{}, Problems: []string{}}
	// The footprint to cover: a Feature's expected paths; an XL story's own
	// scope is what its stories change (each within the Feature's paths).
	scope := src.ExpectedPaths
	if src.Type == "story" {
		scope = nil
		for _, s := range d.Stories {
			scope = append(scope, s.Paths...)
		}
	}
	if len(src.ExpectedPaths) == 0 {
		r.Problems = append(r.Problems, fmt.Sprintf("Feature #%d has no Expected paths; slice computes the footprint from them (add them to the Feature)", src.Feature))
	}
	r.Footprint = GlobFootprint(layers, scope)
	if r.Footprint == nil {
		r.Footprint = []string{}
	}
	switch {
	case len(d.Stories) == 0:
		r.Problems = append(r.Problems, "the draft has no stories")
	case src.XL() && len(d.Stories) < 2:
		r.Problems = append(r.Problems, fmt.Sprintf("#%d is XL: slice it into at least two XS-L stories", src.Number))
	}
	if d.Feature != 0 && d.Feature != src.Number {
		r.Problems = append(r.Problems, fmt.Sprintf("the draft is for #%d, not #%d", d.Feature, src.Number))
	}
	covered := map[string]bool{}
	for i, s := range d.Stories {
		n := i + 1
		it := s.Item(src.Feature)
		sr := StoryReport{N: n, Title: s.Title, Effort: it.Label("effort"), DependsOn: s.DependsOn}
		sr.Problems = append(sr.Problems, workitems.Validate(it, workitems.StageReady)...)
		if !workitems.Empty(s.Fields[workitems.FieldLinks]) {
			sr.Problems = append(sr.Problems, workitems.Problem{Field: workitems.FieldLinks, Message: "slice writes Links from depends_on; leave the field out"})
		}
		if len(s.Paths) == 0 {
			sr.Problems = append(sr.Problems, workitems.Problem{Field: "paths", Message: "name the paths the story changes (a subset of the Feature's expected paths)"})
		}
		for _, p := range s.Paths {
			if len(src.ExpectedPaths) > 0 && !slices.ContainsFunc(src.ExpectedPaths, func(g string) bool { return within(p, g) }) {
				sr.Problems = append(sr.Problems, workitems.Problem{Field: "paths", Message: fmt.Sprintf("%s is outside the Feature's expected paths", p)})
			}
		}
		sr.Footprint = GlobFootprint(layers, s.Paths)
		if sr.Footprint == nil {
			sr.Footprint = []string{}
		}
		if len(s.Paths) > 0 && len(sr.Footprint) == 0 {
			sr.Problems = append(sr.Problems, workitems.Problem{Field: "paths", Message: "the paths fall in no layer of LAYERS.md"})
		}
		for _, l := range sr.Footprint {
			covered[l] = true
		}
		for _, dep := range s.DependsOn {
			if dep < 1 || dep > len(d.Stories) || dep == n {
				sr.Problems = append(sr.Problems, workitems.Problem{Field: "depends_on", Message: fmt.Sprintf("%d is not another story of this draft (1-%d)", dep, len(d.Stories))})
			}
		}
		if sr.Problems == nil {
			sr.Problems = []workitems.Problem{}
		}
		r.Stories = append(r.Stories, sr)
	}
	for _, l := range r.Footprint {
		if !covered[l] {
			r.Problems = append(r.Problems, fmt.Sprintf("layer %s of the Feature's footprint has no story", l))
		}
	}
	order, cyc := topo(d.Stories)
	if cyc {
		r.Problems = append(r.Problems, "depends_on forms a cycle")
	}
	r.Order = order
	r.Pass = len(r.Problems) == 0
	for _, s := range r.Stories {
		r.Pass = r.Pass && len(s.Problems) == 0
	}
	return r
}

// topo orders the stories so each comes after the stories it depends on
// (stable by position); cyc is true when no such order exists.
func topo(ss []DraftStory) (order []int, cyc bool) {
	done := map[int]bool{}
	for len(order) < len(ss) {
		progress := false
		for i, s := range ss {
			n := i + 1
			if done[n] {
				continue
			}
			ready := true
			for _, d := range s.DependsOn {
				if d >= 1 && d <= len(ss) && d != n && !done[d] {
					ready = false
				}
			}
			if ready {
				done[n], progress = true, true
				order = append(order, n)
			}
		}
		if !progress {
			return order, true
		}
	}
	return order, false
}
