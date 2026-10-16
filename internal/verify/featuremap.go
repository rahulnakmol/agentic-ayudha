package verify

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// A feature map (WP4.5b) is a product-owned project skill
// .agents/skills/verify-<product>/ anywhere in a repository: features/README.md
// holds the map block and the ordered index, and every other features/*.md is
// one capability with its behaviour checks. This file reads maps and checks
// them against the checkout (the drift check); it never runs product code.

// FeatureH2 are the H2s every feature file has, in this order.
var FeatureH2 = []string{"Sub-features", "How to reach it", "Behaviour checks", "Gotchas"}

// CheckKinds are the kinds of behaviour check.
var CheckKinds = []string{"ui", "cli", "api", "lib"}

// HandleKeys are the step keys whose string values are handles: names the
// product's source must contain, so that renaming one is drift.
var HandleKeys = []string{"name", "label", "within", "test"}

// FeatureMap is one product's map.
type FeatureMap struct {
	Dir      string        `json:"dir"` // the skill directory, slash-separated, relative to the root
	Product  string        `json:"product"`
	Control  []string      `json:"control"`  // the control tool's argv, run from the repository root
	Surfaces []SurfaceRule `json:"surfaces"` // how to find the product's surfaces in its source
	Order    []string      `json:"-"`        // feature IDs in the index's order
	Features []Feature     `json:"-"`
	Errors   []string      `json:"-"` // contract errors found while reading
}

// SurfaceRule finds one kind of surface: every match of Pattern (its first
// group, else the whole match) in the files the globs select.
type SurfaceRule struct {
	Kind    string   `json:"kind"`
	Files   []string `json:"files"`
	Exclude []string `json:"exclude,omitempty"`
	Pattern string   `json:"pattern"`
}

// Feature is one features/<id>.md.
type Feature struct {
	File       string              `json:"-"`
	ID         string              `json:"id"`
	Layers     []string            `json:"layers"`
	Sources    []string            `json:"sources"`
	Surfaces   map[string][]string `json:"surfaces,omitempty"`
	VerifiedAt string              `json:"verified_at"`
	Checks     []Check             `json:"-"`
}

// Check is one behaviour check: steps in the control tool's vocabulary.
type Check struct {
	ID       string            `json:"id"`
	Kind     string            `json:"kind"`
	Evidence string            `json:"evidence"`
	Requires string            `json:"requires,omitempty"`
	Steps    []json.RawMessage `json:"steps"`
}

// Ref is a check's map-wide name, <feature>.<check>.
func (f Feature) Ref(c Check) string { return f.ID + "." + c.ID }

var (
	mapReadme = regexp.MustCompile(`(^|/)\.agents/skills/verify-[^/]+/features/README\.md$`)
	fenceRE   = regexp.MustCompile("(?ms)^```(map|feature|check)[ \t]*\n(.*?)\n```[ \t]*$")
	h2RE      = regexp.MustCompile(`(?m)^## (.+?)\s*$`)
	indexRE   = regexp.MustCompile(`\]\(([A-Za-z0-9._-]+)\.md\)`)
	idRE      = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
)

// repoFiles lists the checkout's files: tracked, plus untracked ones git does
// not ignore, so a map being written is read before it is committed.
func repoFiles(ctx context.Context, root string) ([]string, error) {
	cmd := exec.CommandContext(ctx, "git", "ls-files", "--cached", "--others", "--exclude-standard", "-z")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files: %w", err)
	}
	var files []string
	for _, f := range strings.Split(string(out), "\x00") {
		if f != "" && !slices.Contains(files, f) {
			files = append(files, f)
		}
	}
	sort.Strings(files)
	return files, nil
}

// LoadMaps reads every feature map in the checkout. A map that cannot be read
// as a whole is still returned, with its contract errors.
func LoadMaps(ctx context.Context, root string) ([]*FeatureMap, error) {
	files, err := repoFiles(ctx, root)
	if err != nil {
		return nil, err
	}
	return loadMaps(root, files), nil
}

func loadMaps(root string, files []string) []*FeatureMap {
	var out []*FeatureMap
	for _, f := range files {
		if mapReadme.MatchString(f) {
			out = append(out, loadMap(root, path.Dir(path.Dir(f)), files))
		}
	}
	return out
}

func fences(md, tag string) []string {
	var out []string
	for _, m := range fenceRE.FindAllStringSubmatch(md, -1) {
		if m[1] == tag {
			out = append(out, m[2])
		}
	}
	return out
}

func strictJSON(s string, v any) error {
	dec := json.NewDecoder(strings.NewReader(s))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

func loadMap(root, dir string, files []string) *FeatureMap {
	m := &FeatureMap{Dir: dir}
	bad := func(format string, args ...any) { m.Errors = append(m.Errors, fmt.Sprintf(format, args...)) }
	readme := dir + "/features/README.md"
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(readme)))
	if err != nil {
		bad("%s: %v", readme, err)
		return m
	}
	switch blocks := fences(string(b), "map"); len(blocks) {
	case 1:
		if err := strictJSON(blocks[0], m); err != nil {
			bad("%s: the map block: %v", readme, err)
		}
	default:
		bad("%s: want one map block, found %d", readme, len(blocks))
	}
	m.Dir = dir
	if len(m.Control) == 0 {
		bad("%s: the map block names no control tool (control)", readme)
	}
	for i, r := range m.Surfaces {
		if r.Kind == "" || len(r.Files) == 0 || r.Pattern == "" {
			bad("%s: surface rule %d needs kind, files, and pattern", readme, i+1)
		} else if _, err := regexp.Compile(r.Pattern); err != nil {
			bad("%s: surface rule %s: %v", readme, r.Kind, err)
		}
	}
	for _, x := range indexRE.FindAllStringSubmatch(string(b), -1) {
		if !slices.Contains(m.Order, x[1]) {
			m.Order = append(m.Order, x[1])
		}
	}
	seen := map[string]bool{}
	for _, f := range files {
		if path.Dir(f) != dir+"/features" || !strings.HasSuffix(f, ".md") || path.Base(f) == "README.md" {
			continue
		}
		ft, errs := loadFeature(root, f)
		m.Errors = append(m.Errors, errs...)
		if ft.ID == "" {
			continue
		}
		if seen[ft.ID] {
			bad("%s: feature %q is defined twice", f, ft.ID)
			continue
		}
		seen[ft.ID] = true
		m.Features = append(m.Features, ft)
	}
	sort.SliceStable(m.Features, func(i, j int) bool { return orderOf(m.Order, m.Features[i].ID) < orderOf(m.Order, m.Features[j].ID) })
	return m
}

func orderOf(order []string, id string) int {
	if i := slices.Index(order, id); i >= 0 {
		return i
	}
	return len(order)
}

func loadFeature(root, file string) (Feature, []string) {
	var errs []string
	bad := func(format string, args ...any) { errs = append(errs, file+": "+fmt.Sprintf(format, args...)) }
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
	if err != nil {
		return Feature{}, []string{fmt.Sprintf("%s: %v", file, err)}
	}
	md := string(b)
	var f Feature
	stem := strings.TrimSuffix(path.Base(file), ".md")
	switch blocks := fences(md, "feature"); len(blocks) {
	case 1:
		if err := strictJSON(blocks[0], &f); err != nil {
			bad("the feature block: %v", err)
			return Feature{}, errs
		}
	default:
		bad("want one feature block, found %d", len(blocks))
		return Feature{}, errs
	}
	f.File = file
	if f.ID != stem || !idRE.MatchString(f.ID) {
		bad("feature id %q must be the file name, %q (lowercase letters, digits, and -)", f.ID, stem)
		return Feature{}, errs
	}
	if len(f.Sources) == 0 {
		bad("the feature block lists no sources")
	}
	var h2 []string
	for _, x := range h2RE.FindAllStringSubmatch(md, -1) {
		h2 = append(h2, x[1])
	}
	if !slices.Equal(h2, FeatureH2) {
		bad("the H2s are %q; want %q in this order", h2, FeatureH2)
	}
	for _, raw := range fences(md, "check") {
		var c Check
		if err := strictJSON(raw, &c); err != nil {
			bad("a check block: %v", err)
			continue
		}
		switch {
		case !idRE.MatchString(c.ID):
			bad("check id %q: lowercase letters, digits, and -", c.ID)
		case slices.ContainsFunc(f.Checks, func(x Check) bool { return x.ID == c.ID }):
			bad("check %q is defined twice", c.ID)
		case !slices.Contains(CheckKinds, c.Kind):
			bad("check %q: kind %q, want one of %s", c.ID, c.Kind, strings.Join(CheckKinds, ", "))
		case !slices.Contains(workitems.EvidenceKinds, c.Evidence):
			bad("check %q: evidence %q, want one of %s", c.ID, c.Evidence, strings.Join(workitems.EvidenceKinds, ", "))
		case len(c.Steps) == 0:
			bad("check %q has no steps", c.ID)
		default:
			f.Checks = append(f.Checks, c)
		}
	}
	if len(f.Checks) == 0 {
		bad("feature %q has no behaviour check", f.ID)
	}
	return f, errs
}

// handle is a string a check relies on finding in the product's source.
type handle struct{ key, value string }

// handles are the HandleKeys fields of each step's action ({"click": {"name":
// …}}), never values deeper down, which are data (a request body's "name").
func (c Check) handles() []handle {
	var out []handle
	add := func(k string, v any) {
		if s, ok := v.(string); ok && s != "" && slices.Contains(HandleKeys, k) && !slices.Contains(out, handle{k, s}) {
			out = append(out, handle{k, s})
		}
	}
	for _, raw := range c.Steps {
		var step map[string]any
		if json.Unmarshal(raw, &step) != nil {
			continue
		}
		for k, action := range step {
			add(k, action)
			if fields, ok := action.(map[string]any); ok {
				for fk, fv := range fields {
					add(fk, fv)
				}
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].value < out[j].value })
	return out
}

// MapReport is the drift check of one map.
type MapReport struct {
	Dir           string          `json:"dir"`
	Product       string          `json:"product"`
	Contract      []string        `json:"contract,omitempty"`          // the map cannot be read as written
	NotIndexed    []string        `json:"not_indexed,omitempty"`       // feature files the README does not list
	DeadIndex     []string        `json:"dead_index,omitempty"`        // README entries with no feature file
	Unmapped      []Surface       `json:"unmapped_surfaces,omitempty"` // found in the source, listed by no feature
	Gone          []Surface       `json:"gone_surfaces,omitempty"`     // listed by a feature, no longer in the source
	UnknownChecks []string        `json:"unknown_checks,omitempty"`    // evidence maps naming checks the map lacks
	Features      []FeatureReport `json:"features"`
}

// Surface is one route, command, flag, or other surface.
type Surface struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
	Where string `json:"where"` // the source file that has it, or the feature that lists it
}

// FeatureReport is the drift check of one feature.
type FeatureReport struct {
	ID             string   `json:"id"`
	Checks         int      `json:"checks"`
	MissingSources []string `json:"missing_sources,omitempty"`
	MissingHandles []string `json:"missing_handles,omitempty"` // "<check>: <handle>"
	Stale          []string `json:"stale,omitempty"`           // sources changed since verified_at
}

// MapDrift is the drift check of every map in a checkout.
type MapDrift struct {
	Head    string      `json:"head"`
	Maps    []MapReport `json:"maps"`
	Verdict string      `json:"verdict"` // none (no map), clean, stale, or drift
}

// Drift verdicts.
const (
	DriftNone  = "none"
	DriftClean = "clean"
	DriftStale = "stale" // sources changed since verified_at; the checks still hold the map to the source
	DriftFound = "drift"
)

// CheckMaps compares every map with the checkout.
func CheckMaps(ctx context.Context, root string) (MapDrift, []*FeatureMap, error) {
	files, err := repoFiles(ctx, root)
	if err != nil {
		return MapDrift{}, nil, err
	}
	head, _ := gitOut(ctx, root, "rev-parse", "HEAD")
	maps := loadMaps(root, files)
	d := MapDrift{Head: strings.TrimSpace(string(head)), Maps: []MapReport{}, Verdict: DriftNone}
	refs := evidenceRefs(root, files)
	for _, m := range maps {
		d.Maps = append(d.Maps, checkMap(ctx, root, files, m, refs))
	}
	if len(maps) > 0 {
		d.Verdict = DriftClean
		for _, r := range d.Maps {
			if r.drifted() {
				d.Verdict = DriftFound
				break
			}
			if slices.ContainsFunc(r.Features, func(f FeatureReport) bool { return len(f.Stale) > 0 }) {
				d.Verdict = DriftStale
			}
		}
	}
	return d, maps, nil
}

func (r MapReport) drifted() bool {
	if len(r.Contract)+len(r.NotIndexed)+len(r.DeadIndex)+len(r.Unmapped)+len(r.Gone)+len(r.UnknownChecks) > 0 {
		return true
	}
	return slices.ContainsFunc(r.Features, func(f FeatureReport) bool { return len(f.MissingSources)+len(f.MissingHandles) > 0 })
}

func matchAny(globs []string, f string) bool {
	return slices.ContainsFunc(globs, func(g string) bool { return Match(g, f) })
}

// sourceFiles are the files a feature's sources select (each a path or glob).
func sourceFiles(files []string, sources []string) []string {
	var out []string
	for _, f := range files {
		if matchAny(sources, f) {
			out = append(out, f)
		}
	}
	return out
}

func checkMap(ctx context.Context, root string, files []string, m *FeatureMap, refs map[string][]string) MapReport {
	r := MapReport{Dir: m.Dir, Product: m.Product, Contract: m.Errors, Features: []FeatureReport{}}
	ids := map[string]bool{}
	for _, f := range m.Features {
		ids[f.ID] = true
	}
	for id := range ids {
		if !slices.Contains(m.Order, id) {
			r.NotIndexed = append(r.NotIndexed, m.Dir+"/features/"+id+".md")
		}
	}
	sort.Strings(r.NotIndexed)
	for _, id := range m.Order {
		if !slices.Contains(files, m.Dir+"/features/"+id+".md") {
			r.DeadIndex = append(r.DeadIndex, id)
		}
	}

	// Surfaces: what the rules find in the source against what the features list.
	found := map[string]map[string]string{} // kind → value → file
	for _, rule := range m.Surfaces {
		re, err := regexp.Compile(rule.Pattern)
		if err != nil {
			continue // a contract error already
		}
		if found[rule.Kind] == nil {
			found[rule.Kind] = map[string]string{}
		}
		for _, f := range files {
			if !matchAny(rule.Files, f) || matchAny(rule.Exclude, f) {
				continue
			}
			b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(f)))
			if err != nil {
				continue
			}
			for _, x := range re.FindAllSubmatch(b, -1) {
				v := string(x[0])
				if len(x) > 1 {
					v = string(x[1])
				}
				if _, ok := found[rule.Kind][v]; !ok {
					found[rule.Kind][v] = f
				}
			}
		}
	}
	listed := map[string]map[string]string{} // kind → value → feature
	for _, f := range m.Features {
		for kind, vals := range f.Surfaces {
			if listed[kind] == nil {
				listed[kind] = map[string]string{}
			}
			for _, v := range vals {
				listed[kind][v] = f.ID
				if found[kind] != nil {
					if _, ok := found[kind][v]; !ok {
						r.Gone = append(r.Gone, Surface{Kind: kind, Value: v, Where: f.ID})
					}
				}
			}
		}
	}
	for kind, vals := range found {
		for v, file := range vals {
			if _, ok := listed[kind][v]; !ok {
				r.Unmapped = append(r.Unmapped, Surface{Kind: kind, Value: v, Where: file})
			}
		}
	}
	sortSurfaces(r.Unmapped)
	sortSurfaces(r.Gone)

	for file, rs := range refs {
		for _, ref := range rs {
			fid, cid, _ := strings.Cut(ref, ".")
			known := false
			for _, f := range m.Features {
				if f.ID == fid && slices.ContainsFunc(f.Checks, func(c Check) bool { return c.ID == cid }) {
					known = true
				}
			}
			if !known && mapOwns(m, fid) {
				r.UnknownChecks = append(r.UnknownChecks, file+": "+ref)
			}
		}
	}
	sort.Strings(r.UnknownChecks)

	for _, f := range m.Features {
		fr := FeatureReport{ID: f.ID, Checks: len(f.Checks)}
		srcs := sourceFiles(files, f.Sources)
		for _, s := range f.Sources {
			if !slices.ContainsFunc(files, func(x string) bool { return Match(s, x) }) {
				fr.MissingSources = append(fr.MissingSources, s)
			}
		}
		var text strings.Builder
		for _, s := range srcs {
			if b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(s))); err == nil {
				text.Write(b)
				text.WriteByte('\n')
			}
		}
		for _, c := range f.Checks {
			for _, h := range c.handles() {
				if !hasHandle(text.String(), h) {
					fr.MissingHandles = append(fr.MissingHandles, c.ID+": "+h.value)
				}
			}
		}
		fr.Stale = stale(ctx, root, f, srcs)
		r.Features = append(r.Features, fr)
	}
	return r
}

// mapOwns says whether an evidence-map reference names this map's feature, or
// no map's feature: an unknown feature ID is reported by the first map.
func mapOwns(m *FeatureMap, fid string) bool {
	return slices.ContainsFunc(m.Order, func(id string) bool { return id == fid }) ||
		slices.ContainsFunc(m.Features, func(f Feature) bool { return f.ID == fid })
}

func sortSurfaces(s []Surface) {
	sort.Slice(s, func(i, j int) bool {
		if s[i].Kind != s[j].Kind {
			return s[i].Kind < s[j].Kind
		}
		return s[i].Value < s[j].Value
	})
}

// hasHandle finds a handle in the source: a test by its name as a word, any
// other handle quoted or as element text: >h<, "h", 'h', or backquoted.
func hasHandle(text string, h handle) bool {
	if h.key == "test" {
		return regexp.MustCompile(`\b` + regexp.QuoteMeta(h.value) + `\b`).MatchString(text)
	}
	for _, q := range []string{">" + h.value + "<", `"` + h.value + `"`, "'" + h.value + "'", "`" + h.value + "`"} {
		if strings.Contains(text, q) {
			return true
		}
	}
	return false
}

// stale lists a feature's sources changed since its verified_at commit, and
// says so when that commit is not in this clone's history.
func stale(ctx context.Context, root string, f Feature, srcs []string) []string {
	if f.VerifiedAt == "" {
		return []string{"never verified (no verified_at)"}
	}
	if len(srcs) == 0 {
		return nil
	}
	if _, err := gitOut(ctx, root, "cat-file", "-e", f.VerifiedAt+"^{commit}"); err != nil {
		return []string{"verified_at " + f.VerifiedAt + " is not in this clone's history"}
	}
	out, err := gitOut(ctx, root, append([]string{"diff", "--name-only", f.VerifiedAt, "--"}, srcs...)...)
	if err != nil {
		return []string{err.Error()}
	}
	var changed []string
	for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if l != "" {
			changed = append(changed, l)
		}
	}
	untracked, _ := gitOut(ctx, root, append([]string{"ls-files", "--others", "--exclude-standard", "--"}, srcs...)...)
	for _, l := range strings.Split(strings.TrimSpace(string(untracked)), "\n") {
		if l != "" && !slices.Contains(changed, l) {
			changed = append(changed, l)
		}
	}
	sort.Strings(changed)
	return changed
}

// evidenceRefs reads the checks every .aayudha/evidence/*.toml names, by file.
func evidenceRefs(root string, files []string) map[string][]string {
	out := map[string][]string{}
	for _, f := range files {
		if path.Dir(f) != ".aayudha/evidence" || !strings.HasSuffix(f, ".toml") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(f)))
		if err != nil {
			continue
		}
		var m EvidenceMap
		if toml.Unmarshal(b, &m) != nil {
			continue // verify reports a malformed evidence map
		}
		for _, c := range m.Criterion {
			out[f] = append(out[f], c.Checks...)
		}
	}
	return out
}

// staleMark starts the line of a stale feature in Items.
const staleMark = "stale, to re-check:"

// Items are the drift and staleness findings as one line each.
func (d MapDrift) Items() []string {
	var out []string
	for _, r := range d.Maps {
		p := r.Dir + ": "
		for _, x := range r.Contract {
			out = append(out, p+"contract: "+x)
		}
		for _, x := range r.NotIndexed {
			out = append(out, p+"not in the index: "+x)
		}
		for _, x := range r.DeadIndex {
			out = append(out, p+"index entry without a feature file: "+x)
		}
		for _, s := range r.Unmapped {
			out = append(out, fmt.Sprintf("%sunmapped %s %q (%s)", p, s.Kind, s.Value, s.Where))
		}
		for _, s := range r.Gone {
			out = append(out, fmt.Sprintf("%s%s %q of feature %s is no longer in the source", p, s.Kind, s.Value, s.Where))
		}
		for _, x := range r.UnknownChecks {
			out = append(out, p+"evidence map names an unknown check: "+x)
		}
		for _, f := range r.Features {
			for _, x := range f.MissingSources {
				out = append(out, fmt.Sprintf("%sfeature %s: source %s matches no file", p, f.ID, x))
			}
			for _, x := range f.MissingHandles {
				out = append(out, fmt.Sprintf("%sfeature %s: handle not in its sources: %s", p, f.ID, x))
			}
			if len(f.Stale) > 0 {
				out = append(out, fmt.Sprintf("%sfeature %s: %s %s", p, f.ID, staleMark, strings.Join(f.Stale, ", ")))
			}
		}
	}
	return out
}

// MapFloor is the feature-map floor (WP4.5b.1): the drift check, in every
// verify run. Drift is reported, never blocking: a renamed control a check
// uses fails that check in the behaviour floor, and the maintain pass corrects
// the map in its own PR (Appendix B S-4.5b).
type MapFloor struct{}

// Name implements Floor.
func (MapFloor) Name() string { return "feature-map" }

// Applies implements Floor.
func (MapFloor) Applies(r *Run) (bool, string) {
	files, err := repoFiles(context.Background(), r.Root)
	if err != nil {
		return true, "" // Run reports it
	}
	if !slices.ContainsFunc(files, mapReadme.MatchString) {
		return false, "no feature map (.agents/skills/verify-*/features/README.md)"
	}
	return true, ""
}

// Run implements Floor.
func (MapFloor) Run(ctx context.Context, r *Run) (Result, error) {
	res := Result{Check: proof.Check{Tool: "aayudha feature map", Version: "1"}}
	d, _, err := CheckMaps(ctx, r.Root)
	if err != nil {
		return res, err
	}
	var drift, stale []string
	for _, x := range d.Items() {
		if strings.Contains(x, staleMark) {
			stale = append(stale, x)
		} else {
			drift = append(drift, x)
		}
	}
	res.Check.Result, res.Check.Value = proof.Pass, float64(len(drift))
	switch {
	case len(drift) > 0:
		res.Check.Detail = fmt.Sprintf("advisory: %s, for the maintain pass: %s", plural(len(drift), "drift item"), strings.Join(first(drift, 5), "; "))
	case len(stale) > 0:
		res.Check.Detail = fmt.Sprintf("in sync; %s changed since verified_at", plural(len(stale), "feature"))
	default:
		res.Check.Detail = "in sync with the source"
	}
	rep, _ := json.MarshalIndent(d, "", "  ")
	res.Artifacts = append(res.Artifacts, Evidence{Path: "reports/feature-map.json", Kind: "report", Data: rep})
	return res, nil
}
