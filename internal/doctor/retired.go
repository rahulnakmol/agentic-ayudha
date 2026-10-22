package doctor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// RetiredWorkflows are workflows an earlier `aayu init --gates` installed and
// this version does not, with the reason; init offers to remove them and doctor
// warns while one is present.
var RetiredWorkflows = map[string]string{
	"aayudha-triage.yml": "classifies intake issues with a model in CI, and product CI never calls a model; triage runs from your own agent (`aayu triage <issue> --as …`) or an agent app you @-mention",
}

// Retired is a retired workflow present in a checkout.
type Retired struct {
	File, Why string
}

// RetiredAt lists the retired workflows present under root's
// .github/workflows: a file of that name whose workflow name is aayudha's (a
// product's own file of the same name is left alone).
func RetiredAt(root string) []Retired {
	var out []Retired
	for _, f := range slices.Sorted(func(y func(string) bool) {
		for k := range RetiredWorkflows {
			if !y(k) {
				return
			}
		}
	}) {
		b, err := os.ReadFile(filepath.Join(root, ".github", "workflows", f))
		name := regexp.MustCompile(`(?m)^name:\s*["']?` + regexp.QuoteMeta(strings.TrimSuffix(f, ".yml")) + `["']?\s*$`)
		if err == nil && name.Match(b) {
			out = append(out, Retired{File: f, Why: RetiredWorkflows[f]})
		}
	}
	return out
}

func checkRetiredWorkflows(_ context.Context, p *Probe) Result {
	if p.Root == "" {
		return Result{Status: Pass, Detail: "no checkout"}
	}
	rs := RetiredAt(p.Root)
	if len(rs) == 0 {
		return Result{Status: Pass, Detail: "no retired aayudha workflow in .github/workflows"}
	}
	var lines []string
	for _, r := range rs {
		lines = append(lines, fmt.Sprintf(".github/workflows/%s %s", r.File, r.Why))
	}
	return Result{Status: Warn, Detail: fmt.Sprintf("%d retired aayudha workflow(s): run `aayu init` and confirm to remove, or delete by hand", len(rs)), Lines: lines}
}
