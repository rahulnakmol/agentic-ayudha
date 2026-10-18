// Package verify runs the deterministic floors (WP4.3 onward) and records
// their results in a proof pack. A floor is a tool run in a scrubbed child
// process plus a threshold from [verify.floors]; no model is involved, and no
// model disposition can override a floor miss (rule 7).
package verify

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// Run is one verify run over a checkout of a PR head.
type Run struct {
	Root     string // the checkout (PR head)
	Base     string // base commit of the diff
	Head     string // head commit
	Changed  []string
	Lines    map[string][]int // changed (added or modified) line numbers per file, new side
	Floors   config.Floors
	Verify   config.Verify
	Criteria []workitems.Criterion // from the Ready snapshot; nil without a story
	Story    int
	Labels   []string // PR labels (e.g. aayudha:screen-reader)
	Sandbox  *Sandbox
	Work     string // scratch directory for tool outputs
	Now      func() time.Time
	Env      func(string) string // the process environment (AAYU_ENV and the like); never passed to children

	// TestEvents is the `go test -json` stream of the coverage floor's run,
	// which TestEvidence maps to criteria.
	TestEvents []byte

	// Checks selects behaviour checks (--check): <feature>, <feature>.<check>,
	// or "all"; empty runs those the change touches and the criteria map.
	Checks string

	CI      bool // running in CI (the screen-reader floor runs there only on its label or nightly)
	Nightly bool // a scheduled run

	seeded bool // [verify] seed ran in this run

	// Footprint is the set of layers the changed paths fall in.
	Footprint []string
}

// Evidence is one artifact a floor produced, with the criteria it proves.
type Evidence struct {
	Path     string // relative artifact path
	Kind     string
	Data     []byte
	Criteria []int
	// OCRScanned marks a screenshot whose pixels the redaction pass read
	// (tesseract) and found free of personal data; only those leave the run.
	OCRScanned bool
}

// Result is a floor's outcome.
type Result struct {
	Check     proof.Check
	More      []proof.Check // further checks one tool run produces (e.g. the a11y sub-floors)
	Artifacts []Evidence
}

// Floor is one deterministic floor.
type Floor interface {
	Name() string
	// Applies says whether the floor runs on this change; when not, the
	// reason is recorded as a skipped check (e.g. "no-L01").
	Applies(r *Run) (bool, string)
	Run(ctx context.Context, r *Run) (Result, error)
}

// Report is the result of a verify run.
type Report struct {
	Checks    []proof.Check
	Artifacts []Evidence
	Hold      bool // every floor passed, was waived, or was skipped without blocking
}

// Execute runs every floor in order. A floor whose tool is missing or errors
// is "blocked" (never a pass); a floor that does not apply is "skipped".
func Execute(ctx context.Context, r *Run, floors []Floor) Report {
	var rep Report
	for _, f := range floors {
		ok, why := f.Applies(r)
		if !ok {
			rep.Checks = append(rep.Checks, proof.Check{Floor: f.Name(), Result: proof.Skipped, Tool: "aayudha", Version: "-", Detail: "skipped: " + why})
			continue
		}
		res, err := f.Run(ctx, r)
		if err != nil {
			c := res.Check
			c.Floor, c.Result, c.Detail = f.Name(), proof.Blocked, err.Error()
			if c.Tool == "" {
				c.Tool, c.Version = "aayudha", "-"
			}
			rep.Checks = append(rep.Checks, c)
			continue
		}
		res.Check.Floor = f.Name()
		rep.Checks = append(rep.Checks, res.Check)
		rep.Checks = append(rep.Checks, res.More...)
		rep.Artifacts = append(rep.Artifacts, res.Artifacts...)
	}
	p := proof.Pack{Checks: rep.Checks}
	rep.Hold = p.FloorsHold()
	return rep
}

// Pack assembles a proof pack from a report: artifacts, the criteria each
// proves, and every check. Evidence for a criterion comes only from what the
// floors produced in this run.
func Pack(r *Run, rep Report, p proof.Pack) (*proof.Pack, map[string][]byte) {
	files := map[string][]byte{}
	byCriterion := map[int][]string{}
	for _, a := range rep.Artifacts {
		files[a.Path] = a.Data
		p.Artifacts = append(p.Artifacts, proof.NewArtifact(a.Path, a.Kind, a.Data))
		for _, n := range a.Criteria {
			byCriterion[n] = append(byCriterion[n], a.Path)
		}
	}
	for _, c := range r.Criteria {
		if ev := byCriterion[c.N]; len(ev) > 0 {
			p.Criteria = append(p.Criteria, proof.Criterion{ID: c.N, Evidence: ev})
		}
	}
	p.Checks = rep.Checks
	p.Footprint = r.Footprint
	p.PRHeadSHA, p.BaseSHA, p.Story = r.Head, r.Base, r.Story
	return &p, files
}

// Unmapped lists the Ready criteria that this run produced no evidence for.
func Unmapped(r *Run, p *proof.Pack) []int {
	var out []int
	for _, c := range r.Criteria {
		if !slices.ContainsFunc(p.Criteria, func(x proof.Criterion) bool { return x.ID == c.N }) {
			out = append(out, c.N)
		}
	}
	return out
}

// tempFile is a path for a tool's output inside the run's scratch directory.
func (r *Run) tempFile(name string) string {
	return filepath.Join(r.Work, name)
}

// readFile reads a tool's output file; a missing file is empty.
func readFile(p string) []byte {
	b, _ := os.ReadFile(p)
	return b
}

// version runs "<tool> <args>" and returns the first line, for the pack.
func version(ctx context.Context, r *Run, tool string, args ...string) string {
	out, err := r.Sandbox.Run(ctx, Cmd{Dir: r.Root, Name: tool, Args: args})
	if err != nil {
		return "unknown"
	}
	s := strings.TrimSpace(string(out.Stdout) + string(out.Stderr))
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

func plural(n int, s string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, s)
	}
	return fmt.Sprintf("%d %ss", n, s)
}
