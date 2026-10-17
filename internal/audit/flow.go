package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/autonomy"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/inspect"
)

// Flow metrics (private research note video-learnings-2026-10.md P1, Appendix B
// S-7.6e): how long each delivered story waited in each stage, how often a
// person had to step in, how much of inspect's advice was acted on, how many
// stories escaped as Bugs, and what kind of change merged. They come from
// data aayu already keeps (timelines, the merge gate's runs, build records
// and dismissals on the ledger, merged PRs), with no model, and are reported,
// never filed as findings.

// Review is what retro reads from a PR's inspect comment.
type Review struct {
	Cycles      int      `json:"cycles"`
	Posted      []string `json:"posted,omitempty"` // finding IDs posted as inline review comments
	Open        []string `json:"open,omitempty"`   // finding IDs in the latest inspection
	Disposition string   `json:"disposition,omitempty"`
}

func reviewOf(s inspect.State) *Review {
	r := &Review{Cycles: s.Cycles, Posted: s.Posted}
	if s.Result != nil {
		r.Disposition = s.Result.Disposition
		for _, f := range s.Result.Findings {
			if f.ID != "" {
				r.Open = append(r.Open, f.ID)
			}
		}
	}
	return r
}

// Bug is a Bug issue and the issue and PR numbers its body links.
type Bug struct {
	Number    int       `json:"number"`
	CreatedAt time.Time `json:"created_at"`
	Links     []int     `json:"links,omitempty"`
}

var linkRE = regexp.MustCompile(`(?:^|[^\w&/])#([0-9]+)\b|/(?:pull|issues)/([0-9]+)\b`)

func bugsOf(issues []autonomy.Issue) []Bug {
	out := []Bug{}
	for _, is := range issues {
		if is.Kind != "bug" {
			continue
		}
		b := Bug{Number: is.Number, CreatedAt: is.CreatedAt}
		for _, m := range linkRE.FindAllStringSubmatch(is.Body, -1) {
			if n, err := strconv.Atoi(m[1] + m[2]); err == nil && !slices.Contains(b.Links, n) {
				b.Links = append(b.Links, n)
			}
		}
		out = append(out, b)
	}
	slices.SortFunc(out, func(a, b Bug) int { return a.Number - b.Number })
	return out
}

// BuildLog is what retro reads from a story's build record on the ledger.
type BuildLog struct {
	First time.Time // the earliest checkpoint
}

// fetchFlowLedger reads build/<n>/state.json and inspect/dismissed.jsonl.
func (d *Data) fetchFlowLedger(ctx context.Context, s Source) error {
	d.Builds = map[int]BuildLog{}
	files, err := s.Tree.List(ctx, "build")
	if err != nil {
		return err
	}
	for _, f := range files {
		if path.Base(f) != "state.json" {
			continue
		}
		b, err := s.Tree.File(ctx, f)
		if err != nil {
			return err
		}
		var st struct {
			Story       int `json:"story"`
			Checkpoints []struct {
				At string `json:"at"`
			} `json:"checkpoints"`
		}
		if json.Unmarshal(b, &st) != nil || st.Story == 0 {
			continue
		}
		var first time.Time
		for _, c := range st.Checkpoints {
			if t, err := parseTime(c.At); err == nil && (first.IsZero() || t.Before(first)) {
				first = t.UTC()
			}
		}
		if !first.IsZero() {
			d.Builds[st.Story] = BuildLog{First: first}
		}
	}
	list, err := s.Tree.List(ctx, "inspect")
	if err != nil || !slices.Contains(list, "inspect/dismissed.jsonl") {
		return err
	}
	b, err := s.Tree.File(ctx, "inspect/dismissed.jsonl")
	if err != nil {
		return err
	}
	for _, line := range strings.Split(string(b), "\n") {
		var x inspect.Dismissal
		if json.Unmarshal([]byte(line), &x) == nil && x.ID != "" {
			d.Dismissed = append(d.Dismissed, x)
		}
	}
	return nil
}

// fetchGateRuns reads the merge gate's successful pull_request_target runs
// that completed before Until.
func (d *Data) fetchGateRuns(ctx context.Context, s Source, in *Inputs, owner, name string) error {
	d.GateRuns = []Run{}
	for page := 1; page != 0; {
		res, resp, err := s.GH.REST.Actions.ListWorkflowRunsByFileName(ctx, owner, name, MergeGateWorkflow,
			&gh.ListWorkflowRunsOptions{Created: "<" + stamp(in.Until), ListOptions: gh.ListOptions{PerPage: 100, Page: page}})
		if github.NotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		for _, r := range res.WorkflowRuns {
			if r.GetEvent() != "pull_request_target" || r.GetConclusion() != "success" || !r.GetUpdatedAt().Before(in.Until) {
				continue
			}
			d.GateRuns = append(d.GateRuns, Run{ID: r.GetID(), Workflow: r.GetPath(), Event: r.GetEvent(), Branch: r.GetHeadBranch(), HeadSHA: r.GetHeadSHA(),
				Conclusion: r.GetConclusion(), CreatedAt: r.GetCreatedAt().UTC(), UpdatedAt: r.GetUpdatedAt().UTC()})
		}
		page = resp.NextPage
	}
	slices.SortStableFunc(d.GateRuns, func(a, b Run) int { return int(a.ID - b.ID) })
	return nil
}

// StoryFlow is one delivered story's way from Ready to ship.
type StoryFlow struct {
	Number  int       `json:"number"`
	PR      int       `json:"pr"`
	Ready   time.Time `json:"ready_at,omitzero"`            // the first status:ready label
	Start   time.Time `json:"first_checkpoint_at,omitzero"` // the earliest checkpoint in build/<n>/state.json
	Green   time.Time `json:"gate_green_at,omitzero"`       // the first successful merge gate run on the merged head completed
	Shipped time.Time `json:"shipped_at"`                   // the merge
	Touches int       `json:"touches"`
	Escapes []int     `json:"escapes,omitempty"` // Bugs traced to it
}

// Stage is one stage's hours over the stories that have both of its ends.
type Stage struct {
	Name    string  `json:"name"`
	Stories int     `json:"stories"`
	P50     float64 `json:"p50"`
	P90     float64 `json:"p90"`
}

// InspectFlow is how much of inspect's posted advice was acted on.
type InspectFlow struct {
	PRs       int `json:"prs"`        // PRs merged in the window that inspect reviewed
	WithPosts int `json:"with_posts"` // of them, PRs whose state records posted findings
	Cycles    int `json:"fix_cycles"`
	Posted    int `json:"posted"`
	ActedOn   int `json:"acted_on"` // posted, gone from the latest inspection, and not dismissed
	Dismissed int `json:"dismissed"`
	Open      int `json:"open_at_merge"`
}

// TypeCount is the number of merged PRs of one Conventional Commit type.
type TypeCount struct {
	Type string `json:"type"`
	PRs  int    `json:"prs"`
}

// Flow is retro's flow and review-quality section.
type Flow struct {
	Stories []StoryFlow `json:"stories"`
	Stages  []Stage     `json:"stages"`
	Touches Stage       `json:"touches"`
	Inspect InspectFlow `json:"inspect"`
	Escaped int         `json:"escaped"`
	Mix     []TypeCount `json:"change_mix"`
	Merged  int         `json:"merged_prs"`
	Days    int         `json:"escape_days"` // [autonomy] correction_days
}

var ccType = regexp.MustCompile(`^([a-z]+)(\([^)]*\))?!?: `)

func (d *Data) human(login string) bool {
	return login != "" && !strings.HasSuffix(login, "[bot]") && !slices.ContainsFunc(d.Bots, func(b string) bool { return strings.EqualFold(b, login) })
}

func (d *Data) pr(n int) *PR {
	for i := range d.PRs {
		if d.PRs[i].Number == n {
			return &d.PRs[i]
		}
	}
	return nil
}

// flow computes the flow section for the stories retro lists.
func (d *Data) flow(in Inputs, stories []RetroStory) Flow {
	f := Flow{Stories: []StoryFlow{}, Mix: []TypeCount{}, Days: d.Days}
	var queue, build, wait, total, touches []float64
	hours := func(from, to time.Time) (float64, bool) {
		if from.IsZero() || to.IsZero() || to.Before(from) {
			return 0, false
		}
		return to.Sub(from).Hours(), true
	}
	for _, s := range stories {
		pr, it := d.pr(s.PR), d.item(s.Number)
		if pr == nil || it == nil {
			continue
		}
		sf := StoryFlow{Number: s.Number, PR: s.PR, Shipped: pr.MergedAt, Start: d.Builds[s.Number].First}
		readies := 0
		for _, e := range it.Events {
			if e.Kind == "labeled" && e.Label == "status:ready" && !e.At.After(pr.MergedAt) {
				if sf.Ready.IsZero() || e.At.Before(sf.Ready) {
					sf.Ready = e.At
				}
				readies++
			}
		}
		if readies > 1 {
			sf.Touches += readies - 1 // made Ready again after a block or a change
		}
		for _, r := range d.GateRuns {
			if pr.HeadSHA != "" && r.HeadSHA == pr.HeadSHA && !r.UpdatedAt.After(pr.MergedAt) && (sf.Green.IsZero() || r.UpdatedAt.Before(sf.Green)) {
				sf.Green = r.UpdatedAt
			}
		}
		for _, x := range []*Item{it, d.item(s.PR)} {
			if x == nil {
				continue
			}
			for _, e := range x.Events {
				if e.At.Before(sf.Ready) || e.At.After(pr.MergedAt) || !d.human(e.Actor) {
					continue
				}
				if (e.Kind == "commented" && !e.Aayu) || (e.Kind == "reviewed" && e.State != "approved") {
					sf.Touches++
				}
			}
		}
		for _, c := range d.Corrections {
			if c.PR == s.PR && c.Kind != autonomy.KindChangesRequested {
				sf.Touches++
			}
		}
		for _, b := range d.Bugs {
			if b.Number != s.Number && b.CreatedAt.After(pr.MergedAt) && b.CreatedAt.Before(in.Until) &&
				(d.Days <= 0 || b.CreatedAt.Before(pr.MergedAt.AddDate(0, 0, d.Days))) &&
				(slices.Contains(b.Links, s.Number) || slices.Contains(b.Links, s.PR)) && !serves(*pr, b.Number) {
				sf.Escapes = append(sf.Escapes, b.Number)
			}
		}
		if len(sf.Escapes) > 0 {
			f.Escaped++
		}
		for _, x := range []struct {
			to       *[]float64
			from, at time.Time
		}{{&queue, sf.Ready, sf.Start}, {&build, sf.Start, sf.Green}, {&wait, sf.Green, sf.Shipped}, {&total, sf.Ready, sf.Shipped}} {
			if h, ok := hours(x.from, x.at); ok {
				*x.to = append(*x.to, h)
			}
		}
		touches = append(touches, float64(sf.Touches))
		f.Stories = append(f.Stories, sf)
	}
	f.Stages = []Stage{stage("Queue: Ready to first checkpoint", queue), stage("Build: first checkpoint to gate green", build),
		stage("Human wait: gate green to ship", wait), stage("Total: Ready to ship", total)}
	f.Touches = stage("Touches per story", touches)
	mix := map[string]int{}
	for _, p := range d.PRs {
		if p.Base != d.Dev || !d.in(in, p.MergedAt) {
			continue
		}
		f.Merged++
		t := "other"
		if m := ccType.FindStringSubmatch(p.Title); m != nil {
			t = m[1]
		}
		mix[t]++
		it := d.item(p.Number)
		if it == nil || it.Inspect == nil {
			continue
		}
		rv := it.Inspect
		f.Inspect.PRs++
		f.Inspect.Cycles += rv.Cycles
		if len(rv.Posted) > 0 {
			f.Inspect.WithPosts++
		}
		for _, id := range rv.Posted {
			f.Inspect.Posted++
			switch {
			case slices.ContainsFunc(d.Dismissed, func(x inspect.Dismissal) bool { return x.ID == id && x.PR == p.Number }):
				f.Inspect.Dismissed++
			case slices.Contains(rv.Open, id):
				f.Inspect.Open++
			default:
				f.Inspect.ActedOn++
			}
		}
	}
	for t, n := range mix {
		f.Mix = append(f.Mix, TypeCount{Type: t, PRs: n})
	}
	slices.SortFunc(f.Mix, func(a, b TypeCount) int {
		if a.PRs != b.PRs {
			return b.PRs - a.PRs
		}
		return strings.Compare(a.Type, b.Type)
	})
	return f
}

// stage is the nearest-rank P50 and P90 of vs.
func stage(name string, vs []float64) Stage {
	s := Stage{Name: name, Stories: len(vs)}
	if len(vs) == 0 {
		return s
	}
	slices.Sort(vs)
	rank := func(p float64) float64 { return vs[int(math.Ceil(p*float64(len(vs))))-1] }
	s.P50, s.P90 = rank(0.5), rank(0.9)
	return s
}

func pct(n, of int) string {
	if of == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%.0f %%", 100*float64(n)/float64(of))
}

func hoursCell(from, to time.Time) string {
	if from.IsZero() || to.IsZero() || to.Before(from) {
		return "n/a"
	}
	return fmt.Sprintf("%.1f", to.Sub(from).Hours())
}

// markdown renders the flow section.
func (f Flow) markdown(p func(string, ...any), stories int) {
	p("## Flow (reported, not gated)\n\n")
	p("No finding comes from this section (P1 of the private research note video-learnings-2026-10.md). Ready is the story's first `status:ready` label; " +
		"the first checkpoint is the earliest in `build/<n>/state.json` on the ledger; gate green is when the first successful `aayudha-merge-gate.yml` run " +
		"on the merged head completed; ship is the merge. A stage counts only the stories that have both of its ends.\n\n")
	if len(f.Stories) == 0 {
		p("No stories delivered in the window.\n\n")
	} else {
		p("| Measure | Stories | P50 | P90 |\n|---|---|---|---|\n")
		for _, s := range append(slices.Clone(f.Stages), f.Touches) {
			if s.Stories == 0 {
				p("| %s | 0 of %d | n/a | n/a |\n", s.Name, stories)
				continue
			}
			unit := " h"
			if s.Name == f.Touches.Name {
				unit = ""
			}
			p("| %s | %d of %d | %.1f%s | %.1f%s |\n", s.Name, s.Stories, stories, s.P50, unit, s.P90, unit)
		}
		p("\n| Story | PR | Queue (h) | Build (h) | Human wait (h) | Total (h) | Touches | Escaped as |\n|---|---|---|---|---|---|---|---|\n")
		for _, s := range f.Stories {
			esc := "—"
			if len(s.Escapes) > 0 {
				var b []string
				for _, n := range s.Escapes {
					b = append(b, fmt.Sprintf("Bug #%d", n))
				}
				esc = strings.Join(b, ", ")
			}
			p("| #%d | #%d | %s | %s | %s | %s | %d | %s |\n", s.Number, s.PR, hoursCell(s.Ready, s.Start), hoursCell(s.Start, s.Green),
				hoursCell(s.Green, s.Shipped), hoursCell(s.Ready, s.Shipped), s.Touches, esc)
		}
		p("\nTouches: comments and reviews by a person on the story or its merge PR between Ready and ship (not a bot, not a comment carrying an " +
			"aayudha marker, and not the approving review, which is ship's human pass), each later `status:ready`, and the PR's WP5.7 corrections " +
			"other than a changes-requested review. In solo mode an agent writing as the founder without a marker counts as a person.\n\n")
	}
	in := f.Inspect
	p("**inspect advice acted on:** %s (%d of %d posted findings). inspect reviewed %d of the %d PRs merged in the window, with %d fix cycles; "+
		"%d recorded posted findings: %d acted on (gone from the latest inspection), %d dismissed by a person, %d open at merge.\n\n",
		pct(in.ActedOn, in.Posted), in.ActedOn, in.Posted, in.PRs, f.Merged, in.Cycles, in.WithPosts, in.ActedOn, in.Dismissed, in.Open)
	p("**Escape rate:** %s (%d of %d stories): a Bug opened after the story merged and within `[autonomy] correction_days` (%d), whose body links "+
		"the story or its merge PR, and which that PR does not itself fix.\n\n", pct(f.Escaped, len(f.Stories)), f.Escaped, len(f.Stories), f.Days)
	p("**Change mix** (Conventional Commit type of the %d PRs merged into dev in the window):", f.Merged)
	if len(f.Mix) == 0 {
		p(" none.\n\n")
		return
	}
	var parts []string
	for _, m := range f.Mix {
		parts = append(parts, fmt.Sprintf("%s %d (%s)", m.Type, m.PRs, pct(m.PRs, f.Merged)))
	}
	p(" %s.\n\n", strings.Join(parts, ", "))
}
