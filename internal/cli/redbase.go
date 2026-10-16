package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"

	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
	"github.com/rahulnakmol/agentic-ayudha/internal/verify"
)

// RedOnBaseFloor is the check a Bug story's pack carries (P5, founder
// decision 4): each mapped proving test, run on the base commit with the
// head's test files, must fail there, or it does not show the fix.
const RedOnBaseFloor = "red-on-base"

// Per-criterion outcomes of a red-on-base run.
const (
	RedOnBase    = "red on base"   // a mapped test fails on the base: it shows the fix
	GreenOnBase  = "green on base" // every mapped test passes on the base: it does not show the change
	Inconclusive = "inconclusive"  // a mapped test did not run on the base (it does not build there, as with a new API)
)

// redOnBase runs the evidence map's tests for story on base, with the head's
// test files (every *_test.go and testdata file the PR changes) copied over,
// in the sandbox, and returns the check. A green-on-base criterion fails the
// check from blockFrom (a date, UTC) on; before it, or with no date, the check
// is skipped with the outcomes in its detail, so it is advisory. Only Go
// modules are run; ok is false when there is nothing to run.
func redOnBase(ctx context.Context, sb *verify.Sandbox, root, base, head string, story int, now time.Time, blockFrom string) (c proof.Check, ok bool, err error) {
	tests, err := mappedTests(root, story)
	if err != nil || len(tests) == 0 {
		return c, false, err
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		return c, false, nil
	}
	c = proof.Check{Floor: RedOnBaseFloor, Tool: "aayudha red-on-base (go test -json on the base)", Version: "1"}
	wt, err := os.MkdirTemp("", "aayudha-red-on-base-")
	if err != nil {
		return c, false, err
	}
	_ = os.Remove(wt)
	if _, err := gitIn(ctx, root, "worktree", "add", "--quiet", "--detach", wt, base); err != nil {
		return c, false, err
	}
	defer func() { _, _ = gitIn(ctx, root, "worktree", "remove", "--force", wt) }()
	changed, err := gitIn(ctx, root, "diff", "--name-only", "--diff-filter=d", base, head)
	if err != nil {
		return c, false, err
	}
	var copied []string
	for _, p := range strings.Fields(changed) {
		if strings.HasSuffix(p, "_test.go") || strings.Contains("/"+p, "/testdata/") {
			copied = append(copied, p)
		}
	}
	if len(copied) > 0 {
		if _, err := gitIn(ctx, wt, append([]string{"checkout", head, "--"}, copied...)...); err != nil {
			return c, false, err
		}
	}
	names := map[string]bool{}
	for _, ts := range tests {
		for _, t := range ts {
			_, name, scoped := strings.Cut(t, ":")
			if !scoped {
				name = t
			}
			names[regexp.QuoteMeta(name)] = true
		}
	}
	pattern := "^(" + strings.Join(sortedKeys(names), "|") + ")$"
	out, err := sb.Run(ctx, verify.Cmd{Dir: wt, Name: "go", Args: []string{"test", "-json", "-count=1", "-run", pattern, "./..."}})
	if err != nil {
		return c, false, err
	}
	events := goTestEvents(out.Stdout)
	ids := make([]int, 0, len(tests))
	for id := range tests {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	var parts []string
	red, green := 0, 0
	for _, id := range ids {
		outcome := GreenOnBase
		for _, t := range tests[id] {
			switch baseResult(events, t) {
			case "fail":
				outcome = RedOnBase
			case "":
				if outcome != RedOnBase {
					outcome = Inconclusive
				}
			}
		}
		switch outcome {
		case RedOnBase:
			red++
		case GreenOnBase:
			green++
		}
		parts = append(parts, fmt.Sprintf("criterion %d: %s", id, outcome))
	}
	c.Value, c.Threshold = float64(red), float64(len(ids))
	c.Detail = strings.Join(parts, "; ") + fmt.Sprintf(" (base %s)", short(base))
	from, _ := time.Parse(time.DateOnly, blockFrom)
	blocking := !from.IsZero() && !now.UTC().Before(from)
	switch {
	case green == 0:
		c.Result = proof.Pass
	case blocking:
		c.Result = proof.Fail
	default:
		c.Result = proof.Skipped
		when := "with no [verify] red_on_base_blocking_from date"
		if !from.IsZero() {
			when = "until " + blockFrom
		}
		c.Detail = "advisory " + when + ": " + c.Detail
	}
	return c, true, nil
}

// mappedTests reads the story's evidence map: criterion -> tests.
func mappedTests(root string, story int) (map[int][]string, error) {
	b, err := os.ReadFile(verify.EvidencePath(root, story))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var m verify.EvidenceMap
	if err := toml.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	out := map[int][]string{}
	for _, c := range m.Criterion {
		if len(c.Tests) > 0 {
			out[c.ID] = append(out[c.ID], c.Tests...)
		}
	}
	return out, nil
}

func goTestEvents(b []byte) []verify.TestEvent {
	var out []verify.TestEvent
	sc := bufio.NewScanner(bytes.NewReader(b))
	sc.Buffer(make([]byte, 1<<20), 1<<26)
	for sc.Scan() {
		var ev verify.TestEvent
		if json.Unmarshal(sc.Bytes(), &ev) == nil && ev.Action != "" {
			out = append(out, ev)
		}
	}
	return out
}

// baseResult is "pass" or "fail" for a mapped test (TestName or
// pkg/path:TestName), or "" when it did not run.
func baseResult(events []verify.TestEvent, name string) string {
	pkg, test, scoped := strings.Cut(name, ":")
	if !scoped {
		test, pkg = pkg, ""
	}
	res := ""
	for _, ev := range events {
		if ev.Test != test || pkg != "" && ev.Package != pkg && !strings.HasSuffix(ev.Package, "/"+pkg) {
			continue
		}
		switch ev.Action {
		case "fail":
			return "fail"
		case "pass":
			res = "pass"
		}
	}
	return res
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}
