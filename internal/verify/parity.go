package verify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
	"github.com/rahulnakmol/agentic-ayudha/internal/survey"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// The parity floor (WP5.9, D-5.9m) holds a modernisation Story to its
// behavioural contract in the merge gate: a criterion that runs
// `aayu survey --parity` or `--compat` is evidenced only by this floor
// replaying the committed legacy capture against the PR's new build. Any
// difference from the golden outputs, or any test the legacy build passes and
// the new build fails, fails the floor; a criterion with no run in the
// evidence map blocks it. No model is involved (rule 7), and the floor cannot
// be waived.

// SurveyRun is one parity or compatibility run in a story's evidence map:
//
//	[[criterion]]
//	id = 1
//	[[criterion.parity]]
//	cases = ".aayudha/survey/cases-run_tests.json"  # the committed cases file
//	bin = "target/release/run_tests"                # the new build, in the checkout
//	build = ["cargo", "build", "--release", "--offline"]
//	ids = ["test001", "test002"]                     # optional: the cases this Story ports
//
// The legacy capture is read from .aayudha/survey/golden/<cases file name>,
// where `aayu survey --golden` wrote it from the pinned reference build.
type SurveyRun struct {
	Cases string   `toml:"cases"`
	Bin   string   `toml:"bin"`
	Build []string `toml:"build"` // run in the checkout, without network, before the cases
	IDs   []string `toml:"ids"`   // parity: only these cases
	Areas []string `toml:"areas"` // compat: only these areas of the suite
}

// surveyCriterion matches a criterion that a survey run proves.
var (
	surveyCriterion = regexp.MustCompile(`survey --(parity|compat)\b`)
	surveySet       = regexp.MustCompile(`\b[GS][1-9][0-9]*\b`)
)

// Parity is the parity and compatibility floor.
type Parity struct{}

// Name implements Floor.
func (Parity) Name() string { return "parity" }

// Applies implements Floor.
func (Parity) Applies(r *Run) (bool, string) {
	for _, c := range r.Criteria {
		if surveyCriterion.MatchString(c.State + " " + c.Expected) {
			return true, ""
		}
	}
	return false, "no criterion runs `aayu survey --parity` or `--compat`"
}

// parityOutcome is one criterion's result, the report's unit.
type parityOutcome struct {
	Criterion   int                  `json:"criterion"`
	Mode        string               `json:"mode"` // parity or compat
	Cases       string               `json:"cases"`
	Sets        []string             `json:"sets,omitempty"`
	Ran         int                  `json:"ran"`
	Diffs       []survey.Diff        `json:"diffs,omitempty"`
	Compat      *survey.CompatReport `json:"compat,omitempty"`
	Regressions int                  `json:"regressions,omitempty"`
}

// Run implements Floor.
func (Parity) Run(ctx context.Context, r *Run) (Result, error) {
	// The cases run as `aayu survey --parity` runs them: a scrubbed
	// environment (PATH, a scratch HOME, LC_ALL=C, TZ=UTC), not the network
	// namespace; the build runs in the sandbox.
	res := Result{Check: proof.Check{Tool: "aayu survey --parity/--compat", Version: "1"}}
	runs, err := surveyRuns(r)
	if err != nil {
		return res, err
	}
	var missing []string
	for _, c := range r.Criteria {
		for _, m := range surveyCriterion.FindAllStringSubmatch(c.State+" "+c.Expected, -1) {
			if len(runs[c.N][m[1]]) == 0 {
				missing = append(missing, fmt.Sprintf("criterion %d runs `aayu survey --%s` but %s gives it no [[criterion.%s]] run", c.N, m[1], filepath.ToSlash(rel(r.Root, EvidencePath(r.Root, r.Story))), m[1]))
			}
		}
	}
	if len(missing) > 0 {
		return res, errors.New(strings.Join(missing, "; ") + ": the floor cannot run, so the rewrite cannot merge")
	}
	built := map[string]bool{}
	var outs []parityOutcome
	failures := 0
	for _, c := range r.Criteria {
		for _, mode := range []string{"parity", "compat"} {
			for _, sr := range runs[c.N][mode] {
				o, err := runSurvey(ctx, r, c, mode, sr, built)
				if err != nil {
					return res, fmt.Errorf("criterion %d: %w", c.N, err)
				}
				outs = append(outs, o)
				failures += len(o.Diffs) + o.Regressions
			}
		}
	}
	var parts []string
	evidenced := map[int]bool{}
	for _, o := range outs {
		evidenced[o.Criterion] = true
	}
	for _, o := range outs {
		switch {
		case len(o.Diffs) > 0:
			evidenced[o.Criterion] = false
			var ds []string
			for _, d := range o.Diffs {
				ds = append(ds, d.ID+" "+d.Stream+": "+d.Detail)
			}
			parts = append(parts, fmt.Sprintf("criterion %d: %d of %d cases differ from the legacy capture (%s)", o.Criterion, len(o.Diffs), o.Ran, strings.Join(first(ds, 3), "; ")))
		case o.Regressions > 0:
			evidenced[o.Criterion] = false
			parts = append(parts, fmt.Sprintf("criterion %d: %d test(s) the legacy build passes fail on the new build", o.Criterion, o.Regressions))
		case o.Mode == "compat":
			parts = append(parts, fmt.Sprintf("criterion %d: %d of %d legacy-passing tests pass", o.Criterion, o.Compat.Pass, o.Compat.LegacyPass))
		default:
			parts = append(parts, fmt.Sprintf("criterion %d: %d cases match the legacy capture", o.Criterion, o.Ran))
		}
	}
	res.Check.Value, res.Check.Threshold, res.Check.Detail = float64(failures), 0, strings.Join(parts, "; ")
	res.Check.Result = proof.Pass
	if failures > 0 {
		res.Check.Result = proof.Fail
	}
	kinds := map[int]string{}
	for _, c := range r.Criteria {
		kinds[c.N] = c.Evidence
	}
	for n, ok := range evidenced {
		if !ok {
			continue
		}
		var mine []parityOutcome
		for _, o := range outs {
			if o.Criterion == n {
				mine = append(mine, o)
			}
		}
		b, _ := json.MarshalIndent(mine, "", "  ")
		res.Artifacts = append(res.Artifacts, Evidence{Path: fmt.Sprintf("parity/criterion-%d.json", n), Kind: kinds[n], Data: b, Criteria: []int{n}})
	}
	slices.SortFunc(res.Artifacts, func(a, b Evidence) int { return strings.Compare(a.Path, b.Path) })
	rep, _ := json.MarshalIndent(outs, "", "  ")
	res.Artifacts = append(res.Artifacts, Evidence{Path: "reports/parity.json", Kind: "report", Data: rep})
	return res, nil
}

// surveyRuns reads the evidence map's parity and compat runs: criterion → mode → runs.
func surveyRuns(r *Run) (map[int]map[string][]SurveyRun, error) {
	out := map[int]map[string][]SurveyRun{}
	b, err := os.ReadFile(EvidencePath(r.Root, r.Story))
	if os.IsNotExist(err) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	var m struct {
		Criterion []struct {
			ID     int         `toml:"id"`
			Parity []SurveyRun `toml:"parity"`
			Compat []SurveyRun `toml:"compat"`
		} `toml:"criterion"`
	}
	if err := toml.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("%s: %w", EvidencePath(r.Root, r.Story), err)
	}
	for _, c := range m.Criterion {
		if out[c.ID] == nil {
			out[c.ID] = map[string][]SurveyRun{}
		}
		out[c.ID]["parity"] = append(out[c.ID]["parity"], c.Parity...)
		out[c.ID]["compat"] = append(out[c.ID]["compat"], c.Compat...)
	}
	return out, nil
}

// runSurvey builds the new program once per build command, selects the cases
// the criterion names, and replays or scores them.
func runSurvey(ctx context.Context, r *Run, c workitems.Criterion, mode string, sr SurveyRun, built map[string]bool) (parityOutcome, error) {
	o := parityOutcome{Criterion: c.N, Mode: mode, Cases: sr.Cases}
	inside := func(p string) (string, error) {
		if p == "" || filepath.IsAbs(p) || strings.HasPrefix(filepath.Clean(p), "..") {
			return "", fmt.Errorf("%q: give a path inside the checkout", p)
		}
		return filepath.Join(r.Root, p), nil
	}
	cases, err := inside(sr.Cases)
	if err != nil {
		return o, fmt.Errorf("cases %w", err)
	}
	bin, err := inside(sr.Bin)
	if err != nil {
		return o, fmt.Errorf("bin %w", err)
	}
	if key := strings.Join(sr.Build, "\x00"); len(sr.Build) > 0 && !built[key] {
		out, err := r.Sandbox.Run(ctx, Cmd{Dir: r.Root, Name: sr.Build[0], Args: sr.Build[1:]})
		if err != nil {
			return o, err
		}
		if out.Code != 0 {
			return o, fmt.Errorf("build %s exited %d: %s", strings.Join(sr.Build, " "), out.Code, tail(out.Stderr))
		}
		built[key] = true
	}
	if _, err := os.Stat(bin); err != nil {
		return o, fmt.Errorf("no new build at %s: build it before verify, or give the run a build command", sr.Bin)
	}
	b, err := os.ReadFile(cases)
	if err != nil {
		return o, err
	}
	all, probs, err := survey.ParseCases(b)
	if err == nil && len(probs) > 0 {
		err = errors.New(strings.Join(probs, "; "))
	}
	if err != nil {
		return o, fmt.Errorf("%s: %w", sr.Cases, err)
	}
	// The criterion names the case sets or suites it proves; with none named,
	// it proves the whole cases file.
	named := surveySet.FindAllString(c.State+" "+c.Expected, -1)
	var sel survey.Cases
	for _, k := range all.Cases {
		if (len(named) == 0 || slices.Contains(named, k.Set)) && (len(sr.IDs) == 0 || slices.Contains(sr.IDs, k.ID)) && (len(sr.Areas) == 0 || slices.Contains(sr.Areas, k.Area)) {
			sel.Cases = append(sel.Cases, k)
			if !slices.Contains(o.Sets, k.Set) {
				o.Sets = append(o.Sets, k.Set)
			}
		}
	}
	for _, id := range sr.IDs {
		if !slices.ContainsFunc(all.Cases, func(k survey.Case) bool { return k.ID == id }) {
			return o, fmt.Errorf("%s has no case %s", sr.Cases, id)
		}
	}
	if len(sel.Cases) == 0 {
		return o, fmt.Errorf("%s has no case in %s that the criterion names", sr.Cases, strings.Join(named, ", "))
	}
	o.Ran = len(sel.Cases)
	golden := filepath.Join(r.Root, ".aayudha", "survey", "golden", strings.TrimSuffix(filepath.Base(sr.Cases), filepath.Ext(sr.Cases)))
	if mode == "parity" {
		o.Diffs, _, err = survey.Replay(ctx, r.Root, bin, golden, sel)
		return o, err
	}
	// Compat appends its score to the capture's history; give it a copy of
	// the manifest so verify leaves the checkout as it found it.
	m, err := os.ReadFile(filepath.Join(golden, "manifest.json"))
	if err != nil {
		return o, fmt.Errorf("no legacy results in %s: capture the suite with `aayu survey --golden` and commit it", rel(r.Root, golden))
	}
	dir, err := os.MkdirTemp(r.Work, "compat-")
	if err != nil {
		return o, err
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), m, 0o644); err != nil {
		return o, err
	}
	if o.Compat, err = survey.Compat(ctx, r.Root, bin, dir, sel); err != nil {
		return o, err
	}
	o.Regressions = o.Compat.Regressions
	return o, nil
}
