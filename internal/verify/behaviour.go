package verify

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"

	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
)

// The behaviour floor (WP4.5b.2) runs a feature map's behaviour checks through
// the map's control tool, a program in the product's own stack (Node and
// Playwright for a web app, a shell or PTY driver for a CLI, plain HTTP for an
// API, the product's test framework for a library). The contract:
//
//	<control...> check     stdin:  {"product", "feature", "check", "out", "base_url", "root"}
//	                        stdout: {"status": "passed"|"failed"|"unreachable", "reason",
//	                                 "artifacts": [{"path", "kind"}]}
//
// Artifacts are written under out; aayu hashes them into the pack. No model
// is involved (rule 7), and any failed check fails the floor.

// Check statuses a control tool reports.
const (
	CheckPassed      = "passed"
	CheckFailed      = "failed"
	CheckUnreachable = "unreachable" // a precondition is missing; it proves nothing and fails nothing
	checkError       = "error"       // the control tool did not answer by the contract
)

// CheckTimeout bounds one behaviour check.
var CheckTimeout = 3 * time.Minute

// SelectAll is the --check value that selects every check (the sweep).
const SelectAll = "all"

// CheckResult is what one check produced.
type CheckResult struct {
	Ref       string          `json:"check"`
	Kind      string          `json:"kind"`
	Status    string          `json:"status"`
	Reason    string          `json:"reason,omitempty"`
	Artifacts []CheckArtifact `json:"artifacts,omitempty"`
	Why       string          `json:"selected_because"`
	Network   string          `json:"network,omitempty"` // how the control tool ran: isolated, proxy-blocked, or allowed (ui and api)
}

// CheckArtifact is one file a check wrote, relative to its output directory.
type CheckArtifact struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
}

type selected struct {
	m   *FeatureMap
	f   Feature
	c   Check
	why string
}

// needsApp says whether a check drives the running app (a network peer).
func needsApp(kind string) bool { return kind == "ui" || kind == "api" }

// Behaviour is the behaviour floor.
type Behaviour struct{}

// Name implements Floor.
func (Behaviour) Name() string { return "behaviour" }

// Applies implements Floor.
func (Behaviour) Applies(r *Run) (bool, string) {
	maps, err := LoadMaps(context.Background(), r.Root)
	if err != nil {
		return true, "" // Run reports it
	}
	if len(maps) == 0 {
		return false, "no feature map (.agents/skills/verify-*/features/README.md)"
	}
	sel, _, err := r.selectChecks(maps)
	if err != nil {
		return true, ""
	}
	if len(sel) == 0 {
		return false, "no behaviour check for this change (it touches no mapped feature, and no criterion maps a check)"
	}
	return true, ""
}

// criterionChecks reads the story's evidence map: criterion → check refs.
func (r *Run) criterionChecks() (map[int][]string, error) {
	out := map[int][]string{}
	if r.Story == 0 {
		return out, nil
	}
	b, err := os.ReadFile(EvidencePath(r.Root, r.Story))
	if os.IsNotExist(err) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	var m EvidenceMap
	if err := toml.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("%s: %w", EvidencePath(r.Root, r.Story), err)
	}
	for _, c := range m.Criterion {
		out[c.ID] = append(out[c.ID], c.Checks...)
	}
	return out, nil
}

// selectChecks picks the checks this run executes: with --check, that
// selection; otherwise every check of a feature whose sources or file the
// change touches (of the whole map when its README or control tool changed),
// plus every check an evidence-map criterion names.
func (r *Run) selectChecks(maps []*FeatureMap) ([]selected, map[int][]string, error) {
	crit, err := r.criterionChecks()
	if err != nil {
		return nil, nil, err
	}
	var out []selected
	add := func(m *FeatureMap, f Feature, c Check, why string) {
		if !slices.ContainsFunc(out, func(s selected) bool { return s.m == m && s.f.ID == f.ID && s.c.ID == c.ID }) {
			out = append(out, selected{m, f, c, why})
		}
	}
	if r.Checks != "" {
		for _, m := range maps {
			for _, f := range m.Features {
				for _, c := range f.Checks {
					if r.Checks == SelectAll || r.Checks == f.ID || r.Checks == f.Ref(c) {
						add(m, f, c, "--check "+r.Checks)
					}
				}
			}
		}
		if len(out) == 0 {
			return nil, nil, fmt.Errorf("no behaviour check %q in the feature maps (`aayu verify --map --json` lists them)", r.Checks)
		}
		return out, crit, nil
	}
	for _, m := range maps {
		harness := slices.ContainsFunc(r.Changed, func(p string) bool {
			rest, ok := strings.CutPrefix(p, m.Dir+"/")
			return ok && rest != "SKILL.md" && (rest == "features/README.md" || !strings.HasPrefix(rest, "features/"))
		})
		for _, f := range m.Features {
			why := ""
			switch {
			case harness:
				why = "the map's harness changed"
			case slices.Contains(r.Changed, f.File):
				why = "its feature file changed"
			case slices.ContainsFunc(r.Changed, func(p string) bool { return matchAny(f.Sources, p) }):
				why = "its sources changed"
			}
			if why != "" {
				for _, c := range f.Checks {
					add(m, f, c, why)
				}
			}
		}
	}
	var ns []int
	for n := range crit {
		ns = append(ns, n)
	}
	sort.Ints(ns)
	for _, n := range ns {
		for _, ref := range crit[n] {
			for _, m := range maps {
				for _, f := range m.Features {
					for _, c := range f.Checks {
						if f.Ref(c) == ref {
							add(m, f, c, fmt.Sprintf("criterion %d maps it", n))
						}
					}
				}
			}
		}
	}
	return out, crit, nil
}

// Run implements Floor.
func (Behaviour) Run(ctx context.Context, r *Run) (Result, error) {
	res := Result{Check: proof.Check{Tool: "feature-map control tools", Version: "-", Network: NetIsolated}}
	maps, err := LoadMaps(ctx, r.Root)
	if err != nil {
		return res, err
	}
	sel, crit, err := r.selectChecks(maps)
	if err != nil {
		return res, err
	}
	var target Target
	if slices.ContainsFunc(sel, func(s selected) bool { return needsApp(s.c.Kind) }) {
		// The app is a network peer: refuse production before seeding (V3).
		if target, err = ResolveTarget(r); err != nil {
			return res, err
		}
		if err := seed(ctx, r); err != nil {
			return res, err
		}
		res.Check.Network = NetAllowed
	}
	var results []CheckResult
	var arts []Evidence
	for _, s := range sel {
		cr, ev := runCheck(ctx, r, s, target)
		results = append(results, cr)
		arts = append(arts, ev...)
	}

	// A criterion is evidenced only when every check it names passed and gave
	// an artifact of the criterion's own kind.
	kinds := map[int]string{}
	for _, c := range r.Criteria {
		kinds[c.N] = c.Evidence
	}
	var problems []string
	for n, refs := range crit {
		if len(refs) == 0 {
			continue
		}
		kind, known := kinds[n]
		if !known {
			continue // TestEvidence reports a criterion the Ready snapshot lacks
		}
		ok := true
		for _, ref := range refs {
			i := slices.IndexFunc(results, func(x CheckResult) bool { return x.Ref == ref })
			switch {
			case i < 0:
				ok = false
				problems = append(problems, fmt.Sprintf("criterion %d: no check %s in the feature maps", n, ref))
			case results[i].Status != CheckPassed:
				ok = false
				problems = append(problems, fmt.Sprintf("criterion %d: %s %s", n, ref, results[i].Status))
			case !slices.ContainsFunc(results[i].Artifacts, func(a CheckArtifact) bool { return a.Kind == kind }):
				ok = false
				problems = append(problems, fmt.Sprintf("criterion %d: %s produced no %s", n, ref, kind))
			}
		}
		if !ok {
			continue
		}
		for i := range arts {
			for _, ref := range refs {
				if strings.HasPrefix(arts[i].Path, "behaviour/"+ref+"/") && (arts[i].Kind == kind || arts[i].Kind == "report") {
					arts[i].Criteria = append(arts[i].Criteria, n)
				}
			}
		}
	}
	sort.Strings(problems)

	var passed, failed, unreachable, errs []string
	for _, x := range results {
		line := x.Ref
		if x.Reason != "" {
			line += " (" + x.Reason + ")"
		}
		switch x.Status {
		case CheckPassed:
			passed = append(passed, x.Ref)
		case CheckFailed:
			failed = append(failed, line)
		case CheckUnreachable:
			unreachable = append(unreachable, line)
		default:
			errs = append(errs, line)
		}
	}
	res.Check.Value, res.Check.Threshold = float64(len(failed)), 0
	parts := []string{fmt.Sprintf("%d of %s passed", len(passed), plural(len(results), "check"))}
	if len(failed) > 0 {
		parts = append(parts, "failed: "+strings.Join(first(failed, 5), "; "))
	}
	if len(errs) > 0 {
		parts = append(parts, "control tool error: "+strings.Join(first(errs, 5), "; "))
	}
	if len(unreachable) > 0 {
		parts = append(parts, "unreachable, proving nothing: "+strings.Join(first(unreachable, 5), "; "))
	}
	if len(problems) > 0 {
		parts = append(parts, "criteria without check evidence: "+strings.Join(first(problems, 5), "; "))
	}
	res.Check.Detail = strings.Join(parts, "; ")
	switch {
	case len(failed) > 0:
		res.Check.Result = proof.Fail
	case len(errs) > 0:
		res.Check.Result = proof.Blocked
	default:
		res.Check.Result = proof.Pass
	}
	rep, _ := json.MarshalIndent(map[string]any{"checks": results, "criteria_problems": problems}, "", "  ")
	res.Artifacts = append(arts, Evidence{Path: "reports/behaviour.json", Kind: "report", Data: rep})
	return res, nil
}

// runCheck hands one check to its map's control tool and collects what it wrote.
func runCheck(ctx context.Context, r *Run, s selected, t Target) (CheckResult, []Evidence) {
	ref := s.f.Ref(s.c)
	cr := CheckResult{Ref: ref, Kind: s.c.Kind, Why: s.why}
	out := filepath.Join(r.Work, "behaviour", ref)
	if err := os.MkdirAll(out, 0o755); err != nil {
		cr.Status, cr.Reason = checkError, err.Error()
		return cr, nil
	}
	in, _ := json.Marshal(map[string]any{"product": s.m.Product, "feature": s.f.ID, "check": s.c, "out": out, "base_url": t.URL, "root": r.Root})
	cctx, cancel := context.WithTimeout(ctx, CheckTimeout)
	defer cancel()
	o, err := r.Sandbox.Run(cctx, Cmd{Dir: r.Root, Name: s.m.Control[0], Args: append(slices.Clone(s.m.Control[1:]), "check"), Stdin: in, Network: needsApp(s.c.Kind)})
	cr.Network = o.Network
	if err != nil {
		cr.Status, cr.Reason = checkError, err.Error()
		return cr, nil
	}
	var got struct {
		Status    string          `json:"status"`
		Reason    string          `json:"reason"`
		Artifacts []CheckArtifact `json:"artifacts"`
	}
	if err := json.Unmarshal(o.Stdout, &got); err != nil || !slices.Contains([]string{CheckPassed, CheckFailed, CheckUnreachable}, got.Status) {
		cr.Status, cr.Reason = checkError, fmt.Sprintf("no result on stdout (exit %d): %s", o.Code, tail(append(o.Stdout, o.Stderr...)))
		return cr, nil
	}
	cr.Status, cr.Reason = got.Status, got.Reason
	var ev []Evidence
	for _, a := range got.Artifacts {
		p := path.Clean(filepath.ToSlash(a.Path))
		if p == "." || strings.HasPrefix(p, "../") || path.IsAbs(p) || !slices.Contains(proof.Kinds, a.Kind) {
			cr.Status, cr.Reason = checkError, fmt.Sprintf("artifact %q (%s) is outside the output directory or of an unknown kind", a.Path, a.Kind)
			return cr, nil
		}
		b, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(p)))
		if err != nil {
			cr.Status, cr.Reason = checkError, fmt.Sprintf("artifact %s: %v", p, err)
			return cr, nil
		}
		cr.Artifacts = append(cr.Artifacts, CheckArtifact{Path: p, Kind: a.Kind})
		ev = append(ev, Evidence{Path: "behaviour/" + ref + "/" + p, Kind: a.Kind, Data: b})
	}
	return cr, ev
}

// MapFloors are the floors a PR that serves no story still runs in the merge
// gate: the drift check and the behaviour checks of what it touches.
func MapFloors() []Floor { return []Floor{MapFloor{}, Behaviour{}} }
