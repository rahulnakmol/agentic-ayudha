package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/identity"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/scope"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

func init() { register("scope", runScope) }

// Markers of the comments scope keeps on a Release issue: the request that
// waits for a person (a drafted baseline, or one change over budget), and the
// plan it hands off.
const (
	ScopeRequestMarker = "<!-- aayudha:scope-request -->"
	ScopePlanMarker    = "<!-- aayudha:scope-plan -->"
)

var scopeRequestRE = regexp.MustCompile(`<!-- scope-request (\{.*\}) -->`)

// scopeRequest is what waits for a person on a Release: accept the draft
// with Hash as the next baseline, or apply one change (add or drop Item).
// Decision is the accepted Decision a change over budget rests on.
type scopeRequest struct {
	Kind     string  `json:"kind"` // baseline, add, or drop
	Item     int     `json:"item,omitempty"`
	Hash     string  `json:"hash,omitempty"`
	Decision int     `json:"decision,omitempty"`
	DriftPct float64 `json:"drift_pct"`
	Over     bool    `json:"over_budget"`
}

// ScopeReport is what scope reports.
type ScopeReport struct {
	Release   int           `json:"release"`
	Version   string        `json:"version"`
	Intent    string        `json:"intent"`
	Milestone int           `json:"milestone,omitempty"`
	Items     []scope.Item  `json:"items"`
	Feedback  []int         `json:"feedback"`
	Points    int           `json:"points"`
	BudgetPct float64       `json:"budget_pct"`
	Baseline  int           `json:"baseline"`
	Drift     *scope.Drift  `json:"drift,omitempty"`
	Problems  []string      `json:"problems"`
	Request   *scopeRequest `json:"request,omitempty"`
	Done      string        `json:"done,omitempty"`
	Next      string        `json:"next,omitempty"`
	DryRun    bool          `json:"dry_run,omitempty"`
}

// scopeRun is one run's state: the Release, its milestone, its plan, and
// what the ledger holds for its version.
type scopeRun struct {
	e        *Env
	c        *github.Client
	w        *ledger.Writer
	cfg      *config.Config
	repo     string
	who      string // the account that writes: the bot, or the person
	n        int
	issue    *gh.Issue
	version  string
	intent   string
	ms       *gh.Milestone
	titles   []string // every milestone's title
	items    []scope.Item
	feedback []int
	base     *scope.Baseline
	draft    *scope.Draft
	drift    *scope.Drift
}

// runScope plans a release (aayu scope). The plan is the GitHub milestone
// named by the Release's Version. An agent adds and drops items, drafts the
// plan, assigns Feedback, and hands a baselined plan off. Accepting a
// baseline, and any change that takes drift over [scope] budget_percent, is a
// person's call: an agent only asks (the request comment), and a person
// accepts it with --accept or in aayu inbox, through GitHub's device flow.
func runScope(ctx context.Context, e *Env, in Values) error {
	acts := 0
	for _, f := range []string{"add", "drop", "assign", "draft", "accept", "handoff"} {
		if in.Has(f) && (in.Bool(f) || in.String(f) != "") {
			acts++
		}
	}
	switch {
	case acts > 1:
		return Errorf(ExitUsage, "scope: use one of --add, --drop, --assign, --draft, --accept, --handoff at a time")
	case in.Has("to") != in.Has("assign"):
		return Errorf(ExitUsage, "scope: --assign and --to go together")
	case in.Has("decision") && !in.Has("add") && !in.Has("drop") && !in.Bool("draft"):
		return Errorf(ExitUsage, "scope: --decision goes with --add, --drop, or --draft")
	}
	repo, err := e.RepoName(ctx)
	if err != nil {
		return err
	}
	l, err := e.Config()
	if err != nil {
		return err
	}
	if len(l.Config.Scope.EffortPoints) != len(workitems.Efforts) {
		return Errorf(ExitUsage, "scope: [scope] effort_points needs %d values, for %s", len(workitems.Efforts), strings.Join(workitems.Efforts, ", "))
	}
	r := &scopeRun{e: e, cfg: &l.Config, repo: repo, n: issueNumber(in.String("release"))}
	var tok identity.Token
	ident, tag := "bot", ""
	if in.Bool("accept") {
		// The person's own client reads; nothing is written before the device flow.
		if r.c, tok, err = e.HumanGitHub(ctx); err != nil {
			return err
		}
		ident = "human"
	} else {
		var bot identity.Bot
		if r.c, bot, err = e.BotGitHub(ctx, ""); err != nil {
			return err
		}
		tok, tag, r.who = bot.Token, bot.Tag, or(bot.Login, bot.Kind)
		if bot.Kind == identity.KindHumanFallback {
			ident = identity.KindHumanFallback
		}
	}
	if r.w, err = e.Ledger(ctx, tok, ident, tag); err != nil {
		return err
	}
	if err := r.load(ctx); err != nil {
		return err
	}
	switch {
	case in.Has("add"):
		return r.change(ctx, "add", issueNumber(in.String("add")), decisionOf(in))
	case in.Has("drop"):
		return r.change(ctx, "drop", issueNumber(in.String("drop")), decisionOf(in))
	case in.Has("assign"):
		return r.assign(ctx, issueNumber(in.String("assign")), in.String("to"))
	case in.Bool("draft"):
		return r.drafted(ctx, decisionOf(in))
	case in.Bool("accept"):
		return r.accept(ctx)
	case in.Bool("handoff"):
		return r.handoff(ctx)
	}
	rep := r.report()
	if rep.Request == nil {
		req, _, _ := r.request(ctx)
		rep.Request = req
	}
	switch {
	case rep.Request != nil:
		rep.Next = fmt.Sprintf("a person accepts the waiting request: aayu scope %d --accept, or aayu inbox", r.n)
	case r.base == nil && len(rep.Problems) == 0:
		rep.Next = fmt.Sprintf("aayu scope %d --draft, then a person accepts it", r.n)
	case r.base == nil:
		rep.Next = "fix the problems, then aayu scope " + strconv.Itoa(r.n) + " --draft"
	}
	return r.emit(rep)
}

func decisionOf(in Values) int {
	if !in.Has("decision") {
		return 0
	}
	return issueNumber(in.String("decision"))
}

// load reads the Release, its milestone and plan, and the ledger's records.
func (r *scopeRun) load(ctx context.Context) error {
	owner, name, _ := github.SplitRepo(r.repo)
	is, _, err := r.c.REST.Issues.Get(ctx, owner, name, r.n)
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	it, _ := workitems.Parse(r.n, is.GetTitle(), is.GetBody(), is.GetType().GetName(), github.IssueLabels(is))
	if it.Type != "release" {
		return Errorf(ExitUsage, "scope: #%d is a %q, not a Release", r.n, it.Type)
	}
	if is.GetState() != "open" {
		return Errorf(ExitBlocked, "scope: Release #%d is closed", r.n)
	}
	r.issue, r.version, r.intent = is, strings.TrimSpace(it.Fields["version"]), strings.TrimSpace(it.Fields[workitems.FieldSummary])
	if !scope.VersionRE.MatchString(r.version) {
		return Errorf(ExitBlocked, "scope: Release #%d: Version %q: a planned release needs vX.Y.Z", r.n, r.version)
	}
	mss, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.Milestone, *gh.Response, error) {
		return r.c.REST.Issues.ListMilestones(ctx, owner, name, &gh.MilestoneListOptions{State: "all", ListOptions: o})
	})
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	r.titles = nil
	for _, m := range mss {
		r.titles = append(r.titles, m.GetTitle())
		if m.GetTitle() == r.version {
			r.ms = m
		}
	}
	if err := r.readPlan(ctx); err != nil {
		return err
	}
	return r.readLedger(ctx)
}

// readPlan lists the milestone's issues: the plan's items, with their
// parents, and the Feedback assigned to the release.
func (r *scopeRun) readPlan(ctx context.Context) error {
	r.items, r.feedback = []scope.Item{}, []int{}
	if r.ms == nil {
		return nil
	}
	owner, name, _ := github.SplitRepo(r.repo)
	issues, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.Issue, *gh.Response, error) {
		return r.c.REST.Issues.ListByRepo(ctx, owner, name, &gh.IssueListByRepoOptions{Milestone: strconv.Itoa(r.ms.GetNumber()), State: "all", ListOptions: o})
	})
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	for _, is := range issues {
		if is.IsPullRequest() || is.GetNumber() == r.n {
			continue
		}
		it, err := r.item(ctx, is)
		switch {
		case err != nil:
			return err
		case it.Type == "feedback":
			r.feedback = append(r.feedback, it.Number)
		case slices.Contains(scope.Types, it.Type):
			r.items = append(r.items, it)
		}
	}
	slices.SortFunc(r.items, func(a, b scope.Item) int { return a.Number - b.Number })
	slices.Sort(r.feedback)
	return nil
}

// item is issue is as a plan item: its type, effort, points, and parent.
func (r *scopeRun) item(ctx context.Context, is *gh.Issue) (scope.Item, error) {
	owner, name, _ := github.SplitRepo(r.repo)
	w, _ := workitems.Parse(is.GetNumber(), is.GetTitle(), is.GetBody(), is.GetType().GetName(), github.IssueLabels(is))
	it := scope.Item{Number: is.GetNumber(), Type: w.Type, Title: is.GetTitle(), Effort: w.Label("effort")}
	it.Points = scope.PointsFor(it.Type, it.Effort, r.cfg.Scope.EffortPoints)
	if !slices.Contains(scope.Types, it.Type) {
		return it, nil
	}
	p, _, err := r.c.REST.SubIssue.GetParentIssue(ctx, owner, name, int64(it.Number))
	switch {
	case github.NotFound(err):
	case err != nil:
		return it, Wrap(ExitExternal, err)
	default:
		it.Parent = p.GetNumber()
	}
	return it, nil
}

// readLedger reads the newest baseline, the draft, and the drift record.
func (r *scopeRun) readLedger(ctx context.Context) error {
	t, err := r.w.Read(ctx)
	if errors.Is(err, ledger.ErrNoLedger) {
		return nil
	} else if err != nil {
		return Wrap(ExitExternal, err)
	}
	defer t.Close()
	paths, err := t.List(ctx, "releases/"+r.version)
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	get := func(p string, v any) (bool, error) {
		if !slices.Contains(paths, p) {
			return false, nil
		}
		b, err := t.File(ctx, p)
		if err != nil {
			return false, Wrap(ExitExternal, err)
		}
		if err := json.Unmarshal(b, v); err != nil {
			return false, Errorf(ExitExternal, "ledger %s: %v", p, err)
		}
		return true, nil
	}
	if n := scope.Newest(paths); n > 0 {
		r.base = &scope.Baseline{}
		if _, err := get(scope.BaselinePath(r.version, n), r.base); err != nil {
			return err
		}
	}
	r.draft, r.drift = &scope.Draft{}, &scope.Drift{}
	if ok, err := get(scope.DraftPath(r.version), r.draft); err != nil {
		return err
	} else if !ok {
		r.draft = nil
	}
	if ok, err := get(scope.DriftPath(r.version), r.drift); err != nil {
		return err
	} else if !ok || r.base == nil || r.drift.Baseline != r.base.Baseline {
		r.drift = nil
	}
	return nil
}

// measure is the plan items measured against the newest baseline, with the
// over-budget Decision the drift record already holds while it still applies.
func (r *scopeRun) measure(items []scope.Item) *scope.Drift {
	if r.base == nil {
		return nil
	}
	d := scope.Measure(*r.base, items, r.cfg.Scope.EffortPoints)
	d.MeasuredAt = r.e.Now().UTC().Format("2006-01-02T15:04:05Z")
	if r.drift != nil && d.Over() {
		d.OverBudgetDecision = r.drift.OverBudgetDecision
	}
	return &d
}

func (r *scopeRun) report() ScopeReport {
	rep := ScopeReport{Release: r.n, Version: r.version, Intent: r.intent, Items: r.items, Feedback: r.feedback, Points: scope.Total(r.items),
		BudgetPct: r.cfg.Scope.BudgetPercent, Problems: scope.Problems(r.intent, r.items), Drift: r.measure(r.items), DryRun: r.c.DryRun()}
	if r.ms != nil {
		rep.Milestone = r.ms.GetNumber()
	}
	if r.base != nil {
		rep.Baseline, rep.BudgetPct = r.base.Baseline, r.base.BudgetPct
	}
	if rep.Problems == nil {
		rep.Problems = []string{}
	}
	return rep
}

func (r *scopeRun) emit(rep ScopeReport) error {
	return r.e.Emit(rep, func(w io.Writer) error {
		fmt.Fprintf(w, "Release #%d %s: %d items, %d points", r.n, r.version, len(rep.Items), rep.Points)
		if rep.Milestone == 0 {
			fmt.Fprint(w, " (no milestone yet)")
		}
		fmt.Fprintln(w)
		if rep.Intent != "" {
			fmt.Fprintf(w, "  intent: %s\n", termText(firstLine(rep.Intent)))
		}
		for _, it := range rep.Items {
			line := fmt.Sprintf("  #%d %s: %s", it.Number, it.Type, termText(it.Title))
			if it.Effort != "" {
				line += fmt.Sprintf(" [%s, %d]", it.Effort, it.Points)
			}
			if it.Parent != 0 {
				line += fmt.Sprintf(" under #%d", it.Parent)
			}
			fmt.Fprintln(w, line)
		}
		if len(rep.Feedback) > 0 {
			fmt.Fprintf(w, "  feedback assigned: %s\n", issueRefs(rep.Feedback))
		}
		switch d := rep.Drift; {
		case d == nil:
			fmt.Fprintln(w, "  no baseline yet: changes are free until a person accepts one")
		default:
			fmt.Fprintf(w, "  baseline %d: drift %.1f%% of %d points, budget %.1f%% (added %d, removed %d, resized %d)\n",
				d.Baseline, d.DriftPct, d.BaselinePoints, d.BudgetPct, len(d.Added), len(d.Removed), len(d.Resized))
			if d.Over() {
				if d.OverBudgetDecision != 0 {
					fmt.Fprintf(w, "  over budget, accepted by a person against Decision #%d\n", d.OverBudgetDecision)
				} else {
					fmt.Fprintln(w, "  over budget with no accepted Decision: changes made outside aayu scope; a person decides")
				}
			}
		}
		for _, p := range rep.Problems {
			fmt.Fprintf(w, "  - %s\n", p)
		}
		if q := rep.Request; q != nil {
			fmt.Fprintf(w, "  waiting for a person: %s\n", q.describe(r.version))
		}
		if rep.Done != "" {
			fmt.Fprintf(w, "  %s\n", rep.Done)
		}
		if rep.Next != "" {
			fmt.Fprintf(w, "  next: %s\n", rep.Next)
		}
		return nil
	})
}

func issueRefs(ns []int) string {
	s := make([]string, len(ns))
	for i, n := range ns {
		s[i] = "#" + strconv.Itoa(n)
	}
	return strings.Join(s, ", ")
}

func (q scopeRequest) describe(version string) string {
	switch q.Kind {
	case "baseline":
		s := fmt.Sprintf("accept the drafted plan as the baseline of %s", version)
		if q.Decision != 0 {
			s += fmt.Sprintf(", %.1f%% from the last baseline, against Decision #%d", q.DriftPct, q.Decision)
		}
		return s
	case "add":
		return fmt.Sprintf("add #%d to %s, taking drift to %.1f%%, against Decision #%d", q.Item, version, q.DriftPct, q.Decision)
	}
	return fmt.Sprintf("drop #%d from %s, taking drift to %.1f%%, against Decision #%d", q.Item, version, q.DriftPct, q.Decision)
}

// change adds item n to the plan or drops it. Before a baseline, or within
// the budget, it applies at once. Over budget, it needs an accepted Decision
// that names the Release, and then only asks a person.
func (r *scopeRun) change(ctx context.Context, op string, n, decision int) error {
	owner, name, _ := github.SplitRepo(r.repo)
	is, _, err := r.c.REST.Issues.Get(ctx, owner, name, n)
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	it, err := r.item(ctx, is)
	if err != nil {
		return err
	}
	in := slices.ContainsFunc(r.items, func(x scope.Item) bool { return x.Number == n })
	rep := r.report()
	switch {
	case !slices.Contains(scope.Types, it.Type):
		return Errorf(ExitUsage, "scope: #%d is a %s; a plan holds Initiatives, Features, Stories, and Bugs (assign Feedback with --assign)", n, or(it.Type, "issue"))
	case op == "add" && in, op == "drop" && !in:
		rep.Done = fmt.Sprintf("#%d is already %s the plan; nothing to do", n, map[string]string{"add": "in", "drop": "not in"}[op])
		return r.emit(rep)
	case op == "add" && is.GetState() != "open":
		return Errorf(ExitBlocked, "scope: #%d is closed", n)
	case op == "add" && is.GetMilestone() != nil:
		return WithNext(Errorf(ExitBlocked, "scope: #%d is planned for %s; drop it there first", n, is.GetMilestone().GetTitle()),
			fmt.Sprintf("aayu scope <the %s Release> --drop %d", is.GetMilestone().GetTitle(), n))
	}
	next := slices.DeleteFunc(slices.Clone(r.items), func(x scope.Item) bool { return x.Number == n })
	if op == "add" {
		next = append(next, it)
	}
	cur, d := r.measure(r.items), r.measure(next)
	if d != nil && d.Over() && d.DriftPct > cur.DriftPct {
		q := scopeRequest{Kind: op, Item: n, DriftPct: d.DriftPct, Over: true, Decision: decision}
		if decision == 0 {
			return WithNext(Errorf(ExitBlocked, "scope: %s #%d takes %s to %.1f%% drift from baseline %d, over the %.1f%% budget; a person decides",
				op, n, r.version, d.DriftPct, d.Baseline, d.BudgetPct),
				fmt.Sprintf("file a Decision that names Release #%d (aayu grill decision --parent %d), a person accepts it (aayu decide <decision> --accept <option>), then aayu scope %d --%s %d --decision <decision>", r.n, r.n, r.n, op, n))
		}
		if err := r.acceptedDecision(ctx, decision); err != nil {
			return err
		}
		if err := r.ask(ctx, q); err != nil {
			return err
		}
		rep.Request = &q
		rep.Done = "asked a person: nothing changes until they accept it"
		rep.Next = fmt.Sprintf("a person runs aayu scope %d --accept, or accepts it in aayu inbox", r.n)
		return r.emit(rep)
	}
	if err := r.apply(ctx, op, it); err != nil {
		return err
	}
	if d != nil {
		d.OverBudgetDecision = 0
		if d.Over() && cur != nil {
			d.OverBudgetDecision = cur.OverBudgetDecision
		}
		if err := r.writeDrift(ctx, *d, fmt.Sprintf("%s #%d", op, n)); err != nil {
			return err
		}
	}
	r.items = next
	rep = r.report()
	rep.Done = fmt.Sprintf("%s #%d (%s)", map[string]string{"add": "added", "drop": "dropped"}[op], n, r.version)
	return r.emit(rep)
}

// apply puts item it in the release's milestone (op add) or takes it out.
func (r *scopeRun) apply(ctx context.Context, op string, it scope.Item) error {
	if op == "add" {
		ms, err := r.milestone(ctx, r.version)
		if err != nil {
			return err
		}
		return setMilestone(ctx, r.c, r.repo, it.Number, ms.GetNumber())
	}
	return setMilestone(ctx, r.c, r.repo, it.Number, 0)
}

// milestone is the milestone titled version, created when missing.
func (r *scopeRun) milestone(ctx context.Context, version string) (*gh.Milestone, error) {
	if r.ms != nil && r.ms.GetTitle() == version {
		return r.ms, nil
	}
	owner, name, _ := github.SplitRepo(r.repo)
	desc := fmt.Sprintf("Release #%d: planned with aayu scope; the plan of record is the newest baseline on the ledger (releases/%s/). A plan, not a promise.", r.n, version)
	var ms *gh.Milestone
	_, err := r.c.Write(ctx, actions.Action{Kind: "github.milestone.create", Target: r.repo,
		Request: &actions.Request{Method: http.MethodPost, Path: fmt.Sprintf("/repos/%s/milestones", r.repo)}, Payload: map[string]any{"title": version}},
		func(ctx context.Context) (any, error) {
			m, _, err := r.c.REST.Issues.CreateMilestone(ctx, owner, name, &gh.Milestone{Title: gh.Ptr(version), Description: gh.Ptr(desc)})
			ms = m
			return m.GetNumber(), err
		})
	if err != nil {
		return nil, Wrap(ExitExternal, err)
	}
	if ms == nil { // --dry-run
		ms = &gh.Milestone{Title: gh.Ptr(version)}
	}
	r.ms = ms
	return ms, nil
}

// setMilestone sets issue n's milestone, or clears it when ms is 0.
func setMilestone(ctx context.Context, c *github.Client, repo string, n, ms int) error {
	owner, name, _ := github.SplitRepo(repo)
	_, err := c.Write(ctx, actions.Action{Kind: "github.issue.milestone", Target: fmt.Sprintf("%s#%d", repo, n),
		Request: &actions.Request{Method: http.MethodPatch, Path: fmt.Sprintf("/repos/%s/issues/%d", repo, n)}, Payload: map[string]any{"milestone": ms}},
		func(ctx context.Context) (any, error) {
			var err error
			if ms == 0 {
				_, _, err = c.REST.Issues.RemoveMilestone(ctx, owner, name, n)
			} else {
				_, _, err = c.REST.Issues.Update(ctx, owner, name, n, gh.UpdateIssueRequest{Milestone: gh.Ptr(ms)})
			}
			return nil, err
		})
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	return nil
}

// assign gives Feedback n a release: this one, the next release milestone,
// or the backlog, with the matching horizon label. It never edits the body,
// and it is not drift: Feedback becomes planned work only as a Feature.
func (r *scopeRun) assign(ctx context.Context, n int, to string) error {
	owner, name, _ := github.SplitRepo(r.repo)
	is, _, err := r.c.REST.Issues.Get(ctx, owner, name, n)
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	labels := github.IssueLabels(is)
	if typ, _ := workitems.TypeOf(is.GetType().GetName(), labels); typ != "feedback" {
		return Errorf(ExitUsage, "scope: #%d is a %s; --assign takes Feedback (plan work items with --add)", n, or(typ, "issue"))
	}
	horizon, ms := map[string]string{"this": "now", "next": "next", "backlog": "later"}[to], 0
	switch to {
	case "this":
		m, err := r.milestone(ctx, r.version)
		if err != nil {
			return err
		}
		ms = m.GetNumber()
	case "next":
		v := scope.Next(r.version, r.titles)
		if v == "" {
			return WithNext(Errorf(ExitBlocked, "scope: no release milestone after %s; Feedback is not dropped into the backlog by default", r.version),
				fmt.Sprintf("file the next Release (aayu grill release), then aayu scope <it> --assign %d --to this", n))
		}
		if ms, err = r.milestoneNumber(ctx, v); err != nil {
			return err
		}
	}
	if is.GetMilestone().GetNumber() != ms {
		if err := setMilestone(ctx, r.c, r.repo, n, ms); err != nil {
			return err
		}
	}
	want := "horizon:" + horizon
	for _, l := range labels {
		if strings.HasPrefix(l, "horizon:") && l != want {
			if err := removeLabel(ctx, r.c, r.repo, n, l); err != nil {
				return Wrap(ExitExternal, err)
			}
		}
	}
	if !slices.Contains(labels, want) {
		if err := addLabel(ctx, r.c, r.repo, n, want); err != nil {
			return Wrap(ExitExternal, err)
		}
	}
	rep := r.report()
	rep.Done = fmt.Sprintf("Feedback #%d assigned to %s (%s)", n, map[string]string{"this": r.version, "next": scope.Next(r.version, r.titles), "backlog": "the backlog"}[to], want)
	return r.emit(rep)
}

func (r *scopeRun) milestoneNumber(ctx context.Context, title string) (int, error) {
	owner, name, _ := github.SplitRepo(r.repo)
	mss, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.Milestone, *gh.Response, error) {
		return r.c.REST.Issues.ListMilestones(ctx, owner, name, &gh.MilestoneListOptions{State: "all", ListOptions: o})
	})
	if err != nil {
		return 0, Wrap(ExitExternal, err)
	}
	for _, m := range mss {
		if m.GetTitle() == title {
			return m.GetNumber(), nil
		}
	}
	return 0, Errorf(ExitExternal, "scope: milestone %s not found", title)
}

// drafted snapshots the plan as the draft a person accepts. A plan that is
// not broken down or sized is refused; a re-baseline that moves the plan
// over budget from the last baseline needs an accepted Decision.
func (r *scopeRun) drafted(ctx context.Context, decision int) error {
	rep := r.report()
	if len(rep.Problems) > 0 {
		return WithNext(Errorf(ExitBlocked, "scope: %s cannot be drafted (%d problem(s)): %s", r.version, len(rep.Problems), strings.Join(rep.Problems, "; ")),
			fmt.Sprintf("aayu scope %d", r.n))
	}
	q := scopeRequest{Kind: "baseline", Hash: scope.Hash(r.intent, r.items)}
	if d := rep.Drift; d != nil {
		q.DriftPct, q.Over = d.DriftPct, d.Over()
		if q.Over && decision == 0 {
			return WithNext(Errorf(ExitBlocked, "scope: this plan is %.1f%% from baseline %d, over the %.1f%% budget; re-baselining it is a person's decision", d.DriftPct, d.Baseline, d.BudgetPct),
				fmt.Sprintf("file a Decision that names Release #%d (aayu grill decision --parent %d), a person accepts it, then aayu scope %d --draft --decision <decision>", r.n, r.n, r.n))
		}
	}
	if q.Over {
		if err := r.acceptedDecision(ctx, decision); err != nil {
			return err
		}
		q.Decision = decision
	}
	who := r.who
	d := scope.Draft{Schema: scope.DraftSchema, Version: r.version, ReleaseIssue: r.n, DraftedAt: r.e.Now().UTC().Format("2006-01-02T15:04:05Z"), DraftedBy: who,
		Intent: r.intent, BudgetPct: r.cfg.Scope.BudgetPercent, Points: scope.Total(r.items), Hash: q.Hash, Items: r.items}
	if err := r.writeLedger(ctx, fmt.Sprintf("scope: %s draft for Release #%d", r.version, r.n), map[string]any{scope.DraftPath(r.version): d}); err != nil {
		return err
	}
	if err := r.ask(ctx, q); err != nil {
		return err
	}
	rep.Request = &q
	rep.Done = fmt.Sprintf("drafted %d items, %d points, as ledger %s", len(r.items), d.Points, scope.DraftPath(r.version))
	rep.Next = fmt.Sprintf("a person accepts it: aayu scope %d --accept, or aayu inbox", r.n)
	return r.emit(rep)
}

// ask posts the request on the Release, replacing an older one.
func (r *scopeRun) ask(ctx context.Context, q scopeRequest) error {
	b, _ := json.Marshal(q)
	body := fmt.Sprintf("%s\n**Waiting for a person (aayu scope):** %s.\n\nAccept it with `aayu scope %d --accept`, or in `aayu inbox`. "+
		"Accepting is a person's call, through GitHub's device code; an agent only asks.\n<!-- scope-request %s -->\n",
		ScopeRequestMarker, q.describe(r.version), r.n, b)
	if _, err := r.c.UpsertComment(ctx, r.repo, r.n, ScopeRequestMarker, body, actions.Action{Kind: "github.comment.scope-request"}); err != nil {
		return Wrap(ExitExternal, err)
	}
	return nil
}

// request is the request waiting on the Release, read from a comment by an
// account with write access or a bot, and that comment's author.
func (r *scopeRun) request(ctx context.Context) (*scopeRequest, string, error) {
	return scopeRequestOn(ctx, r.c, r.repo, r.n)
}

func scopeRequestOn(ctx context.Context, c *github.Client, repo string, n int) (*scopeRequest, string, error) {
	owner, name, _ := github.SplitRepo(repo)
	comments, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.IssueComment, *gh.Response, error) {
		return c.REST.Issues.ListComments(ctx, owner, name, n, &gh.IssueListCommentsOptions{ListOptions: o})
	})
	if err != nil {
		return nil, "", Wrap(ExitExternal, err)
	}
	for _, cm := range comments {
		m := scopeRequestRE.FindStringSubmatch(cm.GetBody())
		if !strings.Contains(cm.GetBody(), ScopeRequestMarker) || m == nil {
			continue
		}
		var q scopeRequest
		if json.Unmarshal([]byte(m[1]), &q) != nil {
			continue
		}
		return &q, cm.GetUser().GetLogin(), nil
	}
	return nil, "", nil
}

// acceptedDecision checks that Decision d was accepted and names the Release.
func (r *scopeRun) acceptedDecision(ctx context.Context, d int) error {
	owner, name, _ := github.SplitRepo(r.repo)
	is, _, err := r.c.REST.Issues.Get(ctx, owner, name, d)
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	it, _ := workitems.Parse(d, is.GetTitle(), is.GetBody(), is.GetType().GetName(), github.IssueLabels(is))
	accepted := strings.Contains(it.Fields["decision"], "accepted by @")
	switch {
	case it.Type != "decision":
		return Errorf(ExitUsage, "scope: #%d is a %s, not a Decision", d, or(it.Type, "issue"))
	case is.GetState() != "closed" || is.GetStateReason() != "completed" || !accepted:
		return WithNext(Errorf(ExitBlocked, "scope: Decision #%d is not accepted yet; a change over budget rests on a person's accepted Decision", d),
			fmt.Sprintf("a person accepts it: aayu decide %d --accept <option>", d))
	case !regexp.MustCompile(`#` + strconv.Itoa(r.n) + `\b`).MatchString(is.GetBody()):
		return Errorf(ExitBlocked, "scope: Decision #%d does not name Release #%d; it must be about this release's scope", d, r.n)
	}
	return nil
}

// accept applies the waiting request as the person, after GitHub's device
// flow, every time: never on a terminal's word alone (31278ca, #647). In
// aayu inbox, the session's one device code covers it. It reads everything
// again and applies only what still holds.
func (r *scopeRun) accept(ctx context.Context) error {
	q, by, err := r.request(ctx)
	if err != nil {
		return err
	}
	if q == nil {
		return Errorf(ExitBlocked, "scope: nothing waits for a person on Release #%d", r.n)
	}
	if ok, err := r.trusted(ctx, by); err != nil {
		return err
	} else if !ok {
		return Errorf(ExitBlocked, "scope: the request on Release #%d is from @%s, who has no write access; it is text, not a request", r.n, by)
	}
	if q.Decision != 0 {
		if err := r.acceptedDecision(ctx, q.Decision); err != nil {
			return err
		}
	}
	effect := "Accepts on Release #" + strconv.Itoa(r.n) + ": " + q.describe(r.version) + "."
	var commit func(login string) (string, error)
	switch q.Kind {
	case "baseline":
		hash := scope.Hash(r.intent, r.items)
		switch {
		case r.draft == nil || r.draft.Hash != q.Hash:
			return WithNext(Errorf(ExitBlocked, "scope: the draft on the ledger is not the one the request names"), fmt.Sprintf("aayu scope %d --draft", r.n))
		case hash != q.Hash:
			return WithNext(Errorf(ExitBlocked, "scope: the plan changed since it was drafted; a person accepts only the plan they see"), fmt.Sprintf("aayu scope %d --draft", r.n))
		}
		if d := r.measure(r.items); d != nil && d.Over() && q.Decision == 0 {
			return WithNext(Errorf(ExitBlocked, "scope: the plan is %.1f%% from baseline %d, over budget, and the request names no Decision", d.DriftPct, d.Baseline),
				fmt.Sprintf("aayu scope %d --draft --decision <accepted decision>", r.n))
		}
		n := 1
		if r.base != nil {
			n = r.base.Baseline + 1
		}
		effect = fmt.Sprintf("Accepts the plan drafted for %s on Release #%d (%d items, %d points) as baseline %d, the scope freeze later changes are measured against.",
			r.version, r.n, len(r.items), scope.Total(r.items), n)
		commit = func(login string) (string, error) {
			now := r.e.Now().UTC().Format("2006-01-02T15:04:05Z")
			b := scope.Baseline{Schema: scope.BaselineSchema, Version: r.version, ReleaseIssue: r.n, Baseline: n, BaselinedAt: now, BaselinedBy: login,
				Intent: r.intent, BudgetPct: r.cfg.Scope.BudgetPercent, Points: scope.Total(r.items), Hash: hash, Decision: q.Decision, Items: r.items}
			d := scope.Measure(b, r.items, r.cfg.Scope.EffortPoints)
			d.MeasuredAt = now
			err := r.writeLedger(ctx, fmt.Sprintf("scope: %s baseline %d for Release #%d, accepted by @%s", r.version, n, r.n, login),
				map[string]any{scope.BaselinePath(r.version, n): b, scope.DriftPath(r.version): d})
			return fmt.Sprintf("baseline %d accepted by @%s: ledger %s", n, login, scope.BaselinePath(r.version, n)), err
		}
	case "add", "drop":
		owner, name, _ := github.SplitRepo(r.repo)
		is, _, err := r.c.REST.Issues.Get(ctx, owner, name, q.Item)
		if err != nil {
			return Wrap(ExitExternal, err)
		}
		it, err := r.item(ctx, is)
		if err != nil {
			return err
		}
		in := slices.ContainsFunc(r.items, func(x scope.Item) bool { return x.Number == q.Item })
		if (q.Kind == "add") == in || r.base == nil {
			return Errorf(ExitBlocked, "scope: the request to %s #%d no longer applies; the plan changed", q.Kind, q.Item)
		}
		next := slices.DeleteFunc(slices.Clone(r.items), func(x scope.Item) bool { return x.Number == q.Item })
		if q.Kind == "add" {
			next = append(next, it)
		}
		d := r.measure(next)
		effect = fmt.Sprintf("Lets %s #%d %s %s, taking drift from baseline %d to %.1f%% against a %.1f%% budget, on Decision #%d.",
			q.Kind, q.Item, map[string]string{"add": "into", "drop": "out of"}[q.Kind], r.version, d.Baseline, d.DriftPct, d.BudgetPct, q.Decision)
		commit = func(login string) (string, error) {
			if err := r.apply(ctx, q.Kind, it); err != nil {
				return "", err
			}
			if d.Over() {
				d.OverBudgetDecision = q.Decision
			}
			r.items = next
			return fmt.Sprintf("%s #%d accepted by @%s on Decision #%d", q.Kind, q.Item, login, q.Decision),
				r.writeDrift(ctx, *d, fmt.Sprintf("%s #%d accepted by @%s on Decision #%d", q.Kind, q.Item, login, q.Decision))
		}
	default:
		return Errorf(ExitBlocked, "scope: unknown request %q on Release #%d", q.Kind, r.n)
	}
	login, err := r.e.scopePerson(ctx, effect)
	if err != nil {
		return err
	}
	done, err := commit(login)
	if err != nil {
		return err
	}
	body := fmt.Sprintf("%s\n**Accepted (aayu scope):** %s, by @%s on %s.\n", ScopeRequestMarker, q.describe(r.version), login, r.e.Now().UTC().Format("2006-01-02"))
	if _, err := r.c.UpsertComment(ctx, r.repo, r.n, ScopeRequestMarker, body, actions.Action{Kind: "github.comment.scope-request", Payload: map[string]any{"by": login}}); err != nil {
		return Wrap(ExitExternal, err)
	}
	if err := r.readLedger(ctx); err != nil {
		return err
	}
	rep := r.report()
	rep.Done = done
	if q.Kind == "baseline" {
		rep.Next = fmt.Sprintf("aayu scope %d --handoff", r.n)
	}
	return r.emit(rep)
}

// trusted reports whether login may leave a request: a bot, or an account
// with write access. Anyone may comment.
func (r *scopeRun) trusted(ctx context.Context, login string) (bool, error) {
	if strings.HasSuffix(login, "[bot]") || slices.Contains(r.cfg.Identity.BotLogins, login) {
		return true, nil
	}
	owner, name, _ := github.SplitRepo(r.repo)
	perm, _, err := r.c.REST.Repositories.GetPermissionLevel(ctx, owner, name, login)
	if err != nil {
		return false, Wrap(ExitExternal, err)
	}
	p := permissionOf(perm)
	return p == "write" || p == "admin", nil
}

// scopePerson proves a person accepts effect through GitHub's device flow,
// every time, as inspect --approve does since #647 (31278ca): a terminal with
// no agent marker is not proof of a person. An approval earlier in the same
// process (aayu inbox's one code per session) covers it. It returns the
// person's login.
func (e *Env) scopePerson(ctx context.Context, effect string) (string, error) {
	refusal := Errorf(ExitBlocked, "scope --accept needs GitHub's device-flow presence check, which is not available here; run it where a person can enter the code")
	if sig := e.agentSignal(); sig != "" {
		refusal = Errorf(ExitBlocked, "scope --accept is a person's decision and is refused for an agent lane (%s); ask a person to run it, or to accept it in aayu inbox", sig)
	}
	a, err := e.presenceCheck(ctx, Pending{What: "scope --accept", Effect: []string{effect}}, AccessWrite, refusal)
	if err != nil {
		return "", err
	}
	return a.Login, nil
}

// handoff hands the baselined plan to execution: the Release's includes
// links, which aayu release ships, and the plan posted on the Release.
func (r *scopeRun) handoff(ctx context.Context) error {
	if r.base == nil {
		return WithNext(Errorf(ExitBlocked, "scope: %s has no baseline; a person accepts one before hand-off", r.version), fmt.Sprintf("aayu scope %d --draft", r.n))
	}
	d := r.measure(r.items)
	if d.Over() && d.OverBudgetDecision == 0 {
		return WithNext(Errorf(ExitBlocked, "scope: %s drifted %.1f%% from baseline %d, over the %.1f%% budget, through changes outside aayu scope; a person decides before hand-off",
			r.version, d.DriftPct, d.Baseline, d.BudgetPct), fmt.Sprintf("re-baseline: aayu scope %d --draft --decision <accepted decision>", r.n))
	}
	features := map[int]bool{}
	for _, it := range r.items {
		if it.Type == "feature" {
			features[it.Number] = true
		}
	}
	var lines []string
	for _, it := range r.items {
		if it.Type == "feature" || (it.Type == "story" || it.Type == "bug") && !features[it.Parent] {
			lines = append(lines, fmt.Sprintf("includes #%d", it.Number))
		}
	}
	body := workitems.SetSection(r.issue.GetBody(), "Links", strings.Join(lines, "\n"))
	if body != r.issue.GetBody() {
		owner, name, _ := github.SplitRepo(r.repo)
		_, err := r.c.Write(ctx, actions.Action{Kind: "github.issue.scope-handoff", Target: fmt.Sprintf("%s#%d", r.repo, r.n),
			Request: &actions.Request{Method: http.MethodPatch, Path: fmt.Sprintf("/repos/%s/issues/%d", r.repo, r.n)}, Payload: map[string]any{"includes": len(lines), "baseline": r.base.Baseline}},
			func(ctx context.Context) (any, error) {
				_, _, err := r.c.REST.Issues.Update(ctx, owner, name, r.n, gh.UpdateIssueRequest{Body: gh.Ptr(body)})
				return nil, err
			})
		if err != nil {
			return Wrap(ExitExternal, err)
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n## Release plan: %s\n\nBaseline %d, accepted by @%s on %s; ledger `%s`. Drift now: %.1f%% of %d points, budget %.1f%%.\n\n",
		ScopePlanMarker, r.version, r.base.Baseline, r.base.BaselinedBy, r.base.BaselinedAt[:min(10, len(r.base.BaselinedAt))], scope.BaselinePath(r.version, r.base.Baseline), d.DriftPct, d.BaselinePoints, d.BudgetPct)
	b.WriteString("| Item | Type | Effort | Parent |\n|---|---|---|---|\n")
	for _, it := range r.items {
		parent := ""
		if it.Parent != 0 {
			parent = "#" + strconv.Itoa(it.Parent)
		}
		fmt.Fprintf(&b, "| #%d | %s | %s | %s |\n", it.Number, it.Type, it.Effort, parent)
	}
	fmt.Fprintf(&b, "\nThis is the plan of record, not a promise: a slip moves an item to the next release. A change that takes drift past %.1f%% waits for a person's accepted Decision.\n", d.BudgetPct)
	if _, err := r.c.UpsertComment(ctx, r.repo, r.n, ScopePlanMarker, b.String(), actions.Action{Kind: "github.comment.scope-plan"}); err != nil {
		return Wrap(ExitExternal, err)
	}
	if err := r.writeDrift(ctx, *d, "hand-off"); err != nil {
		return err
	}
	rep := r.report()
	rep.Done = fmt.Sprintf("handed off baseline %d: %d includes links on Release #%d, plan posted", r.base.Baseline, len(lines), r.n)
	rep.Next = fmt.Sprintf("aayu next picks the Stories; aayu release %d --to rolling ships them", r.n)
	return r.emit(rep)
}

func (r *scopeRun) writeDrift(ctx context.Context, d scope.Drift, why string) error {
	return r.writeLedger(ctx, fmt.Sprintf("scope: %s drift %.1f%% after %s (Release #%d)", r.version, d.DriftPct, why, r.n), map[string]any{scope.DriftPath(r.version): d})
}

// writeLedger writes files as indented JSON in one ledger commit.
func (r *scopeRun) writeLedger(ctx context.Context, msg string, files map[string]any) error {
	if r.c.DryRun() {
		return nil
	}
	c := ledger.Commit{Message: msg, Files: map[string][]byte{}}
	for p, v := range files {
		b, _ := json.MarshalIndent(v, "", "  ")
		c.Files[p] = append(b, '\n')
	}
	if _, err := r.w.Write(ctx, c); err != nil {
		return Wrap(ExitExternal, err)
	}
	return nil
}

// scopeItem makes an open Release with a waiting scope request an inbox
// card, when a bot or an account with write access asked: anyone may comment.
func (s *inboxSource) scopeItem(ctx context.Context, it *InboxItem) (bool, error) {
	comments, err := s.commentsOf(ctx, it.Number)
	if err != nil {
		return false, err
	}
	for _, cm := range comments {
		m := scopeRequestRE.FindStringSubmatch(cm.GetBody())
		if m == nil || !strings.Contains(cm.GetBody(), ScopeRequestMarker) {
			continue
		}
		var q scopeRequest
		if json.Unmarshal([]byte(m[1]), &q) != nil {
			return false, nil
		}
		if login := cm.GetUser().GetLogin(); cm.GetUser().GetType() != "Bot" && !slices.Contains(s.id.BotLogins, login) {
			if ok, err := s.canWrite(ctx, login); err != nil || !ok {
				return false, err
			}
		}
		is := s.issues[it.Number]
		w, _ := workitems.Parse(it.Number, is.GetTitle(), is.GetBody(), is.GetType().GetName(), it.labels)
		what := q.describe(strings.TrimSpace(w.Fields["version"]))
		it.Kind, it.Seat, it.Evidence, it.f.problem = InboxScope, verbs.ProductStrategist, or(cm.GetHTMLURL(), it.URL), what
		it.Why = what + "; scope reads everything again before it writes."
		it.Options = []InboxAction{{N: 1, Recommended: true, Label: termText(clip(what, 90)), Command: fmt.Sprintf("aayu scope %d --accept", it.Number)},
			{N: 2, Label: "open the Release and read the plan", Command: "open"}}
		return true, nil
	}
	return false, nil
}
