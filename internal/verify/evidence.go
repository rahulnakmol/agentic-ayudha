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
	Criterion []struct {
		ID    int      `toml:"id"`
		Tests []string `toml:"tests"` // TestName, or package/path:TestName
		// Checks are feature-map behaviour checks, <feature>.<check> (WP4.5b),
		// which the behaviour floor runs in this verify run.
		Checks []string `toml:"checks"`
	} `toml:"criterion"`
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
		ok := len(c.Tests) > 0
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
		out = append(out, Evidence{Path: fmt.Sprintf("evidence/criterion-%d.jsonl", c.ID), Kind: kind, Data: buf.Bytes(), Criteria: []int{c.ID}})
	}
	return out, problems, nil
}
