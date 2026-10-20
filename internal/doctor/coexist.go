package doctor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
)

// Workflows and branch names init uses (WP2.5, WP2.6). init never edits an
// existing workflow, prefixes every check it registers with "aayudha/", adds
// its rulesets alongside existing ones, and maps branch names in config
// instead of renaming anything.
var (
	initWorkflows = []string{"aayudha-token.yml", "aayudha-ready-guard.yml"}
	initBranches  = []string{"dev", "rolling", "main", "ledger", "brain"}
)

// Workflow is an existing workflow file and the check names its jobs report.
type Workflow struct {
	File   string   `json:"file"`
	Checks []string `json:"checks"`
}

var (
	jobLine  = regexp.MustCompile(`^  ([A-Za-z0-9_-]+):\s*(#.*)?$`)
	nameLine = regexp.MustCompile(`^    name:\s*["']?([^"'#]+?)["']?\s*(#.*)?$`)
)

// ParseWorkflow returns the check names of a workflow's jobs: each job's
// name, or its ID when unnamed. It reads the conventional two-space layout.
func ParseWorkflow(src string) []string {
	var checks []string
	inJobs := false
	cur := -1
	for _, l := range strings.Split(src, "\n") {
		switch {
		case strings.HasPrefix(l, "jobs:"):
			inJobs = true
			continue
		case l != "" && !strings.HasPrefix(l, " ") && !strings.HasPrefix(l, "#"):
			inJobs = false
		}
		if !inJobs {
			continue
		}
		if m := jobLine.FindStringSubmatch(l); m != nil {
			checks = append(checks, m[1])
			cur = len(checks) - 1
		} else if m := nameLine.FindStringSubmatch(l); m != nil && cur >= 0 {
			checks[cur] = strings.TrimSpace(m[1])
		}
	}
	return checks
}

// Coexistence is the report init reads to add alongside a product's own CI.
type Coexistence struct {
	Workflows     []Workflow `json:"workflows"`
	OtherBranches []string   `json:"other_branches"`
	HasDev        bool       `json:"has_dev"`
	Default       string     `json:"default_branch,omitempty"`
	Rulesets      []string   `json:"rulesets,omitempty"`
	RulesetsKnown bool       `json:"rulesets_known"`
	Collisions    []string   `json:"collisions,omitempty"`
}

// Inspect builds the coexistence report for the repository at p.Root.
func Inspect(ctx context.Context, p *Probe) Coexistence {
	var c Coexistence
	files, _ := filepath.Glob(filepath.Join(p.Root, ".github", "workflows", "*.y*ml"))
	sort.Strings(files)
	for _, f := range files {
		base := filepath.Base(f)
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		if slices.Contains(initWorkflows, base) {
			c.Collisions = append(c.Collisions, fmt.Sprintf("workflow %s already exists; init installs a file with that name", base))
		}
		w := Workflow{File: base, Checks: ParseWorkflow(string(b))}
		for _, ch := range w.Checks {
			if strings.HasPrefix(ch, "aayudha/") {
				c.Collisions = append(c.Collisions, fmt.Sprintf("check %q in %s uses the aayudha/ prefix that init registers", ch, base))
			}
		}
		c.Workflows = append(c.Workflows, w)
	}
	out, _ := p.Exec(ctx, p.Root, "git", "for-each-ref", "--format=%(refname:short)", "refs/heads", "refs/remotes/origin")
	seen := map[string]bool{}
	for _, ref := range strings.Fields(out) {
		name := strings.TrimPrefix(ref, "origin/")
		if name == "origin" || name == "HEAD" || seen[name] {
			continue
		}
		seen[name] = true
		if name == "dev" {
			c.HasDev = true
		}
		if !slices.Contains(initBranches, name) {
			c.OtherBranches = append(c.OtherBranches, name)
		}
	}
	sort.Strings(c.OtherBranches)
	if head, err := p.Exec(ctx, p.Root, "git", "symbolic-ref", "--short", "refs/remotes/origin/HEAD"); err == nil {
		c.Default = strings.TrimPrefix(strings.TrimSpace(head), "origin/")
	} else if cur, err := p.Exec(ctx, p.Root, "git", "symbolic-ref", "--short", "HEAD"); err == nil {
		c.Default = strings.TrimSpace(cur)
	}
	if p.API != nil && p.Repo != "" {
		var rs []struct {
			Name string `json:"name"`
		}
		if err := p.API(ctx, "/repos/"+p.Repo+"/rulesets", &rs); err == nil {
			c.RulesetsKnown = true
			for _, r := range rs {
				c.Rulesets = append(c.Rulesets, r.Name)
			}
		}
	}
	return c
}

func checkCoexistence(ctx context.Context, p *Probe) Result {
	if p.Root == "" {
		return Result{Status: Pass, Detail: "not in a repository"}
	}
	c := Inspect(ctx, p)
	var ls []string
	for _, w := range c.Workflows {
		ls = append(ls, fmt.Sprintf("existing workflow %s (checks: %s): init leaves it byte-identical and adds aayudha/* checks alongside", w.File, strings.Join(w.Checks, ", ")))
	}
	if len(c.OtherBranches) > 0 {
		ls = append(ls, "existing branches outside dev/rolling/main/ledger: "+strings.Join(c.OtherBranches, ", "))
	}
	if !c.HasDev && c.Default != "" {
		ls = append(ls, fmt.Sprintf("no dev branch: init writes [branches] dev = %q to config instead of renaming anything", c.Default))
	}
	switch {
	case !c.RulesetsKnown:
		ls = append(ls, "existing rulesets: unknown (no GitHub token or repository)")
	case len(c.Rulesets) > 0:
		ls = append(ls, "existing rulesets: "+strings.Join(c.Rulesets, ", ")+"; init adds its own alongside")
	}
	ls = append(ls, c.Collisions...)
	r := Result{Status: Pass, Detail: "coexistence report: nothing to coexist with", Lines: ls}
	if len(c.Workflows) > 0 || len(c.OtherBranches) > 0 || len(c.Rulesets) > 0 || !c.HasDev {
		r.Detail = "coexistence report: init adds alongside the existing setup"
	}
	if len(c.Collisions) > 0 {
		r.Status = Warn
		r.Detail = fmt.Sprintf("coexistence report: %d name collision(s) init must resolve", len(c.Collisions))
	}
	return r
}
