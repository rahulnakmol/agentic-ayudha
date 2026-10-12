// Package status implements `aayu status` (WP2.8): it walks a Goal,
// Initiative, or Feature through its sub-issues and rolls up Delivery Status,
// health, risk, blocked and over-budget items, shipped-but-unmeasured items,
// Decisions near their SLA, and cost, for the table, --json, the Goal comment,
// and the --digest page. Proof coverage (M4), streaks (M5), and provisional
// decision deadlines (WP5.6) are "n/a until" their WPs.
package status

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// Not-yet-available columns (rule 1a).
const (
	ProofNA       = "n/a until M4"
	StreakNA      = "n/a until M5"
	ProvisionalNA = "n/a until WP5.6"
)

// Decision is an open Decision and its SLA clock.
type Decision struct {
	Number   int       `json:"number"`
	Title    string    `json:"title"`
	Deadline time.Time `json:"deadline"`
	Overdue  bool      `json:"overdue"`
}

// Rollup is one Goal, Initiative, or Feature and what lies under it.
type Rollup struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Type   string `json:"type"`
	State  string `json:"state"`
	Status string `json:"status"`
	Health string `json:"health,omitempty"`
	Risk   string `json:"risk,omitempty"`
	Schema int    `json:"schema_version"`
	Items  int    `json:"items"` // work items under it, at any depth

	ByStatus          map[string]int `json:"by_status"`
	ByHealth          map[string]int `json:"by_health"`
	ByRisk            map[string]int `json:"by_risk"`
	Blocked           []int          `json:"blocked"`
	OverBudget        []int          `json:"over_budget"`
	ShippedUnmeasured []int          `json:"shipped_unmeasured"`
	DecisionsNearSLA  []Decision     `json:"decisions_near_sla"`
	Provisional       []int          `json:"provisional_decisions"`
	ProvisionalHours  string         `json:"provisional_hours"`
	ProofCoverage     string         `json:"proof_coverage"`
	Streak            string         `json:"streak"`
	CostUSD           float64        `json:"cost_usd"`
	// Closable: a Feature whose children are all released (or later) and that
	// has a Result, or an Experiment child with a readout Result (the
	// parent-close rule); Initiatives and Goals close on measure (n/a until WP7.2).
	Closable bool      `json:"closable"`
	Children []*Rollup `json:"children,omitempty"` // Initiatives and Features under it

	leaves []node
}

type node struct {
	number  int
	title   string
	typ     string
	status  string
	health  string
	risk    string
	state   string
	labels  []string
	body    string
	created time.Time
}

// Walker fetches and rolls up.
type Walker struct {
	GH       *github.Client
	Repo     string
	Clock    config.Clock
	SLAHours int
	Now      time.Time
	Budget   float64         // [budget] per_story_usd; 0 disables over-budget
	Cost     map[int]float64 // model and API spend per issue number, from the actions log
}

func fromIssue(is *gh.Issue) node {
	labels := github.IssueLabels(is)
	typ, _ := workitems.TypeOf(is.GetType().GetName(), labels)
	n := node{number: is.GetNumber(), title: is.GetTitle(), typ: typ, state: is.GetState(), labels: labels, body: is.GetBody(), created: is.GetCreatedAt().Time}
	it := workitems.Item{Labels: labels}
	n.status, n.health, n.risk = it.Label("status"), it.Label("health"), it.Label("risk")
	if n.status == "" {
		n.status = "none"
	}
	return n
}

// Roll fetches issue n and everything under it.
func (w *Walker) Roll(ctx context.Context, n int) (*Rollup, error) {
	owner, name, err := github.SplitRepo(w.Repo)
	if err != nil {
		return nil, err
	}
	is, _, err := w.GH.REST.Issues.Get(ctx, owner, name, n)
	if err != nil {
		return nil, err
	}
	return w.roll(ctx, fromIssue(is), 0)
}

func container(t string) bool { return t == "goal" || t == "initiative" || t == "feature" }

func (w *Walker) roll(ctx context.Context, n node, depth int) (*Rollup, error) {
	owner, name, _ := github.SplitRepo(w.Repo)
	r := &Rollup{Number: n.number, Title: n.title, Type: n.typ, State: n.state, Status: n.status, Health: n.health, Risk: n.risk,
		Schema: workitems.SchemaVersion, ByStatus: map[string]int{}, ByHealth: map[string]int{}, ByRisk: map[string]int{},
		Blocked: []int{}, OverBudget: []int{}, ShippedUnmeasured: []int{}, DecisionsNearSLA: []Decision{}, Provisional: []int{},
		ProofCoverage: ProofNA, Streak: StreakNA, ProvisionalHours: ProvisionalNA}
	if depth > 8 {
		return r, nil
	}
	subs, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.SubIssue, *gh.Response, error) {
		return w.GH.REST.SubIssue.ListByIssue(ctx, owner, name, int64(n.number), &o)
	})
	if err != nil {
		return nil, err
	}
	for _, s := range subs {
		c := fromIssue((*gh.Issue)(s))
		r.leaves = append(r.leaves, c)
		if container(c.typ) {
			child, err := w.roll(ctx, c, depth+1)
			if err != nil {
				return nil, err
			}
			r.Children = append(r.Children, child)
			r.leaves = append(r.leaves, child.leaves...)
		}
	}
	w.count(r)
	if r.Type == "feature" {
		r.Closable = closable(n, subs)
	}
	return r, nil
}

var shipped = []string{"released", "measured", "done"}

func closable(n node, subs []*gh.SubIssue) bool {
	if len(subs) == 0 || n.state == "closed" {
		return false
	}
	readout := false
	for _, s := range subs {
		c := fromIssue((*gh.Issue)(s))
		if c.status != "cancelled" && !slices.Contains(shipped, c.status) {
			return false
		}
		if c.typ == "experiment" && hasResult(c) {
			readout = true
		}
	}
	return readout || hasResult(n)
}

// hasResult reports whether a Feature or Experiment body has a Result.
func hasResult(n node) bool {
	it, _ := workitems.Parse(n.number, n.title, n.body, "", []string{"type:" + n.typ})
	return !workitems.Empty(it.Fields[workitems.FieldResult])
}

func (w *Walker) count(r *Rollup) {
	for _, c := range r.leaves {
		r.Items++
		r.ByStatus[c.status]++
		if c.health != "" {
			r.ByHealth[c.health]++
		}
		if c.risk != "" {
			r.ByRisk[c.risk]++
		}
		if c.status == "blocked" {
			r.Blocked = append(r.Blocked, c.number)
		}
		cost := w.Cost[c.number]
		r.CostUSD += cost
		if w.Budget > 0 && cost > w.Budget {
			r.OverBudget = append(r.OverBudget, c.number)
		}
		if c.status == "released" {
			r.ShippedUnmeasured = append(r.ShippedUnmeasured, c.number)
		}
		if c.typ == "decision" && c.state == "open" {
			if slices.Contains(c.labels, workitems.LabelProvisional) {
				r.Provisional = append(r.Provisional, c.number)
			}
			if w.SLAHours > 0 && !c.created.IsZero() {
				deadline := w.Clock.Add(c.created, time.Duration(w.SLAHours)*time.Hour)
				near := w.Clock.Add(c.created, time.Duration(w.SLAHours)*time.Hour*3/4)
				if !w.Now.Before(near) {
					r.DecisionsNearSLA = append(r.DecisionsNearSLA, Decision{c.number, c.title, deadline, !w.Now.Before(deadline)})
				}
			}
		}
	}
	r.CostUSD = float64(int(r.CostUSD*100+0.5)) / 100
	for _, s := range [][]int{r.Blocked, r.OverBudget, r.ShippedUnmeasured, r.Provisional} {
		sort.Ints(s)
	}
	sort.Slice(r.DecisionsNearSLA, func(i, j int) bool { return r.DecisionsNearSLA[i].Number < r.DecisionsNearSLA[j].Number })
}

// Goals lists the open Goals of the repository.
func (w *Walker) Goals(ctx context.Context) ([]int, error) {
	owner, name, err := github.SplitRepo(w.Repo)
	if err != nil {
		return nil, err
	}
	issues, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.Issue, *gh.Response, error) {
		return w.GH.REST.Issues.ListByRepo(ctx, owner, name, &gh.IssueListByRepoOptions{State: "open", Labels: []string{"type:goal"}, ListOptions: o})
	})
	if err != nil {
		return nil, err
	}
	var out []int
	for _, is := range issues {
		if !is.IsPullRequest() {
			out = append(out, is.GetNumber())
		}
	}
	sort.Ints(out)
	return out, nil
}

// Done is the share of items whose status is done, merged, released, or measured.
func (r *Rollup) Done() int {
	return r.ByStatus["merged"] + r.ByStatus["released"] + r.ByStatus["measured"] + r.ByStatus["done"]
}

// Line is the one-line summary of a rollup.
func (r *Rollup) Line() string {
	var parts []string
	for _, s := range workitems.DeliveryStatus {
		if n := r.ByStatus[s]; n > 0 {
			parts = append(parts, fmt.Sprintf("%s %d", s, n))
		}
	}
	if n := r.ByStatus["none"]; n > 0 {
		parts = append(parts, fmt.Sprintf("no status %d", n))
	}
	s := fmt.Sprintf("#%d %s %q: %d items (%s)", r.Number, r.Type, r.Title, r.Items, strings.Join(parts, ", "))
	if len(r.Blocked) > 0 {
		s += fmt.Sprintf("; blocked %s", refs(r.Blocked))
	}
	if r.Closable {
		s += "; closable (all children released, Result present)"
	}
	return s
}

func refs(ns []int) string {
	out := make([]string, len(ns))
	for i, n := range ns {
		out[i] = fmt.Sprintf("#%d", n)
	}
	return strings.Join(out, " ")
}

// Table renders a rollup tree as markdown, one row per Goal, Initiative, and Feature.
func Table(roots []*Rollup) string {
	var b strings.Builder
	b.WriteString("| Item | Items | Done | Ready | In progress | Blocked | Health (on/at/off) | Risk (l/m/h) | Decisions near SLA | Shipped, unmeasured | Proof | Streak | Cost |\n")
	b.WriteString("|---|---|---|---|---|---|---|---|---|---|---|---|---|\n")
	var row func(r *Rollup, depth int)
	row = func(r *Rollup, depth int) {
		fmt.Fprintf(&b, "| %s#%d %s %s | %d | %d | %d | %d | %d | %d/%d/%d | %d/%d/%d | %d | %d | %s | %s | $%.2f |\n",
			strings.Repeat("· ", depth), r.Number, r.Type, strings.ReplaceAll(r.Title, "|", "\\|"), r.Items, r.Done(), r.ByStatus["ready"], r.ByStatus["in-progress"], len(r.Blocked),
			r.ByHealth["on-track"], r.ByHealth["at-risk"], r.ByHealth["off-track"], r.ByRisk["low"], r.ByRisk["medium"], r.ByRisk["high"],
			len(r.DecisionsNearSLA), len(r.ShippedUnmeasured), r.ProofCoverage, r.Streak, r.CostUSD)
		for _, c := range r.Children {
			row(c, depth+1)
		}
	}
	for _, r := range roots {
		row(r, 0)
	}
	return b.String()
}

// CommentMarker marks the replace-in-place status comment on a Goal.
const CommentMarker = "<!-- aayudha:status -->"

// Comment is the Goal comment.
func Comment(r *Rollup, at time.Time) string {
	return CommentMarker + "\n### Status\n\n" + Table([]*Rollup{r}) + "\nUpdated " + at.UTC().Format("2006-01-02 15:04 UTC") + " by `aayu status --apply`.\n"
}

// Digest is the founder digest (status --digest): one section per Goal.
func Digest(repo string, roots []*Rollup, at time.Time) string {
	var b strings.Builder
	fmt.Fprintf(&b, "aayudha status digest for %s, %s\n", repo, at.UTC().Format("2006-01-02"))
	for _, r := range roots {
		fmt.Fprintf(&b, "\n%s\n", r.Line())
		fmt.Fprintf(&b, "  done %d of %d; ready %d; in progress %d; blocked %d\n", r.Done(), r.Items, r.ByStatus["ready"], r.ByStatus["in-progress"], len(r.Blocked))
		if len(r.DecisionsNearSLA) > 0 {
			for _, d := range r.DecisionsNearSLA {
				state := "due"
				if d.Overdue {
					state = "overdue since"
				}
				fmt.Fprintf(&b, "  decision #%d %q %s %s\n", d.Number, d.Title, state, d.Deadline.Format("2006-01-02 15:04 MST"))
			}
		}
		if len(r.ShippedUnmeasured) > 0 {
			fmt.Fprintf(&b, "  shipped but not measured: %s\n", refs(r.ShippedUnmeasured))
		}
		if len(r.OverBudget) > 0 {
			fmt.Fprintf(&b, "  over budget: %s\n", refs(r.OverBudget))
		}
		fmt.Fprintf(&b, "  provisional decisions: %d (deadlines %s); proof coverage %s; streak %s; cost $%.2f\n", len(r.Provisional), r.ProvisionalHours, r.ProofCoverage, r.Streak, r.CostUSD)
	}
	return b.String()
}
