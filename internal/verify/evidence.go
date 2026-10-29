package verify

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// TestEvent is one line of `go test -json`.
type TestEvent struct {
	Time    string  `json:"Time,omitempty"`
	Action  string  `json:"Action"`
	Package string  `json:"Package,omitempty"`
	Test    string  `json:"Test,omitempty"`
	Elapsed float64 `json:"Elapsed,omitempty"`
	Output  string  `json:"Output,omitempty"`
	// ImportPath names a build-output event's build; FailedBuild names it on
	// the fail event of a package whose build failed.
	ImportPath  string `json:"ImportPath,omitempty"`
	FailedBuild string `json:"FailedBuild,omitempty"`
}

func testEvents(b []byte) []TestEvent {
	var out []TestEvent
	sc := bufio.NewScanner(bytes.NewReader(b))
	sc.Buffer(make([]byte, 1<<20), 1<<26)
	for sc.Scan() {
		var ev TestEvent
		if json.Unmarshal(sc.Bytes(), &ev) == nil && ev.Action != "" {
			out = append(out, ev)
		}
	}
	return out
}

// EvidenceMap is .aayudha/evidence/<story>.toml, committed in the PR: which
// tests prove which Completion Goal criterion. The tests run in this verify
// run (the coverage floor's `go test -json`), so the evidence is what CI
// observed, not what the PR claims.
type EvidenceMap struct {
	Criterion []EvidenceCriterion `toml:"criterion"`
}

// EvidenceCriterion is one criterion of an EvidenceMap.
type EvidenceCriterion struct {
	ID    int      `toml:"id"`
	Tests []string `toml:"tests"` // TestName, or package/path:TestName
	// Checks are feature-map behaviour checks, <feature>.<check> (WP4.5b),
	// which the behaviour floor runs in this verify run.
	Checks []string `toml:"checks"`
	// Evals are tier-2 eval cases, <skill>/<name>, whose evals.yml results
	// for the head commit prove the criterion (Bug #1157).
	Evals []string `toml:"evals"`
}

// EvalResults is the part of an evals.yml results file
// (evals/results/<sha>-<group>.json) that proves a criterion.
type EvalResults struct {
	SHA   string     `json:"sha"`
	K     int        `json:"k"`
	Cases []EvalCase `json:"cases"`
	// Run is the evals.yml run that left the results (its URL).
	Run string `json:"-"`
}

// EvalCase is one case's k runs in an EvalResults.
type EvalCase struct {
	Case   string            `json:"case"`
	Runs   []json.RawMessage `json:"runs"`
	Errors int               `json:"errors"`
	PassK  float64           `json:"pass_k"`
	Mean   float64           `json:"mean"`
	NotRun string            `json:"not_run"`
}

// evalHarness reports the first changed path that the tier-2 harness builds
// or reads, or "". An evals.yml run on a PR head uses the PR's own workflow
// and harness, so its results prove nothing once the PR changes them: any Go
// file that is not a test, the module files, evals/, the workflow, or the
// browser floors the ux group installs (packages/ and the pnpm files).
func evalHarness(changed []string) string {
	for _, f := range changed {
		if strings.HasSuffix(f, ".go") && !strings.HasSuffix(f, "_test.go") || f == "go.mod" || f == "go.sum" ||
			strings.HasPrefix(f, "evals/") || f == ".github/workflows/evals.yml" ||
			strings.HasPrefix(f, "packages/") || f == "package.json" || f == "pnpm-lock.yaml" || f == "pnpm-workspace.yaml" {
			return f
		}
	}
	return ""
}

// evalBars is what evals/eval.toml says about a case: its group and the mean bar.
type evalBars struct {
	MinMean float64 `toml:"min_mean"`
	Groups  []struct {
		Name     string   `toml:"name"`
		Prefixes []string `toml:"prefixes"`
	} `toml:"groups"`
}

// groupOf is the group that runs case id, as the harness assigns it: the
// first group with a prefix of id, else the group with no prefixes.
func (b evalBars) groupOf(id string) string {
	rest := ""
	for _, g := range b.Groups {
		if len(g.Prefixes) == 0 {
			rest = g.Name
		}
		for _, p := range g.Prefixes {
			if strings.HasPrefix(id, p) {
				return g.Name
			}
		}
	}
	return rest
}

// evalLine is case id's evidence line from the head's results, or why there is none:
// the case must have run all k runs, with no error, every run passing, and a
// mean at the bar.
func (r *Run) evalLine(id string, bars evalBars, results map[string]*EvalResults) ([]byte, string) {
	if f := evalHarness(r.Changed); f != "" {
		return nil, fmt.Sprintf("this change edits %s, which the eval harness builds or reads, so evals.yml results do not count", f)
	}
	if r.EvalResults == nil {
		return nil, "evals.yml results are not read in this run (no GitHub client)"
	}
	group := bars.groupOf(id)
	res, seen := results[group]
	if !seen {
		var err error
		if res, err = r.EvalResults(group); err != nil {
			return nil, err.Error()
		}
		results[group] = res
	}
	switch {
	case res == nil:
		return nil, fmt.Sprintf("no evals.yml run of group %s left results for %s", group, short(r.Head))
	case res.SHA != r.Head:
		return nil, fmt.Sprintf("the group %s results are for %s, not the head %s", group, short(res.SHA), short(r.Head))
	}
	i := slices.IndexFunc(res.Cases, func(c EvalCase) bool { return c.Case == id })
	if i < 0 {
		return nil, fmt.Sprintf("the group %s results for %s have no case %s", group, short(r.Head), id)
	}
	c := res.Cases[i]
	if c.NotRun != "" {
		return nil, fmt.Sprintf("%s did not run in %s: %s", id, res.Run, c.NotRun)
	}
	if c.Errors > 0 || len(c.Runs) < res.K || c.PassK < 1 || c.Mean < bars.MinMean {
		return nil, fmt.Sprintf("%s in %s: %d of %d runs, %d with an error, pass^k %.2f, mean %.2f against the bar %.2f",
			id, res.Run, len(c.Runs), res.K, c.Errors, c.PassK, c.Mean, bars.MinMean)
	}
	line, _ := json.Marshal(map[string]any{"eval": id, "sha": res.SHA, "k": res.K, "runs": len(c.Runs), "errors": c.Errors,
		"pass_k": c.PassK, "mean": c.Mean, "min_mean": bars.MinMean, "run": res.Run})
	return append(line, '\n'), ""
}

// EvidencePath is the evidence map of a story in a checkout.
func EvidencePath(root string, story int) string {
	return filepath.Join(root, ".aayudha", "evidence", fmt.Sprintf("%d.toml", story))
}

// TestEvidence turns the evidence map and the run's test events into one
// artifact per criterion: the events of the named tests. A criterion whose
// tests did not all run and pass gets no artifact, and is reported.
func TestEvidence(r *Run) ([]Evidence, []string, error) {
	b, err := os.ReadFile(EvidencePath(r.Root, r.Story))
	if os.IsNotExist(err) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	var m EvidenceMap
	if err := toml.Unmarshal(b, &m); err != nil {
		return nil, nil, fmt.Errorf("%s: %w", EvidencePath(r.Root, r.Story), err)
	}
	events := testEvents(r.TestEvents)
	var bars evalBars
	if slices.ContainsFunc(m.Criterion, func(c EvidenceCriterion) bool { return len(c.Evals) > 0 }) {
		b, err := os.ReadFile(filepath.Join(r.Root, "evals", "eval.toml"))
		if err != nil {
			return nil, nil, fmt.Errorf("the evidence map names eval cases: %w", err)
		}
		if err := toml.Unmarshal(b, &bars); err != nil {
			return nil, nil, fmt.Errorf("evals/eval.toml: %w", err)
		}
	}
	results := map[string]*EvalResults{} // per group, read once
	var out []Evidence
	var problems []string
	for _, c := range m.Criterion {
		kind := "api-trace"
		known := false
		for _, rc := range r.Criteria {
			if rc.N == c.ID {
				known = true
				if rc.Evidence == "metric" || rc.Evidence == "api-trace" {
					kind = rc.Evidence
				}
			}
		}
		if !known {
			problems = append(problems, fmt.Sprintf("criterion %d in the evidence map is not in the Ready snapshot", c.ID))
			continue
		}
		var sel []TestEvent
		ok := len(c.Tests)+len(c.Evals) > 0
		for _, name := range c.Tests {
			pkg, test, scoped := strings.Cut(name, ":")
			if !scoped {
				test, pkg = pkg, ""
			}
			// A package is an import path or its directory (`./mathx`, `.` for
			// the module root); a test is a top-level test or a subtest
			// (`TestSum/positive`), which covers its own subtests.
			pkg = strings.TrimSuffix(strings.TrimPrefix(pkg, "./"), "/")
			if pkg == "." {
				pkg = ""
			}
			match := func(ev TestEvent) bool {
				return (ev.Test == test || strings.HasPrefix(ev.Test, test+"/")) && (pkg == "" || ev.Package == pkg || strings.HasSuffix(ev.Package, "/"+pkg))
			}
			passed := slices.ContainsFunc(events, func(ev TestEvent) bool { return match(ev) && ev.Test == test && ev.Action == "pass" })
			failed := slices.ContainsFunc(events, func(ev TestEvent) bool { return match(ev) && ev.Action == "fail" })
			if !passed || failed {
				ok = false
				problems = append(problems, fmt.Sprintf("criterion %d: test %s did not pass in this run", c.ID, name))
			}
			for _, ev := range events {
				if match(ev) {
					sel = append(sel, ev)
				}
			}
		}
		var lines [][]byte
		for _, id := range c.Evals {
			line, why := r.evalLine(id, bars, results)
			if why != "" {
				ok = false
				problems = append(problems, fmt.Sprintf("criterion %d: eval %s: %s", c.ID, id, why))
			}
			lines = append(lines, line)
		}
		if !ok {
			continue
		}
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		for _, ev := range sel {
			ev.Time = "" // keep the artifact byte-stable across identical runs
			ev.Elapsed = 0
			_ = enc.Encode(ev)
		}
		for _, line := range lines {
			buf.Write(line)
		}
		out = append(out, Evidence{Path: fmt.Sprintf("evidence/criterion-%d.jsonl", c.ID), Kind: kind, Data: buf.Bytes(), Criteria: []int{c.ID}})
	}
	return out, problems, nil
}
