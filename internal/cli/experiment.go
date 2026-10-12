package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/experiment"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations/posthog"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations/providers"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

func init() {
	register("experiment", runExperiment)
	register("readout", runReadout)
}

// rate parses a probability-like flag value, or returns def when it is unset.
func rate(name, raw string, def float64) (float64, error) {
	if raw == "" {
		return def, nil
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, Errorf(ExitUsage, "--%s %q: want a number", name, raw)
	}
	return v, nil
}

// detectable turns --mde into the treatment rate: "0.02" adds 0.02 to the
// baseline rate, "10%" multiplies it by 1.10; a leading minus detects a drop.
func detectable(p1 float64, mde string) (float64, error) {
	s := strings.TrimSpace(mde)
	rel := strings.HasSuffix(s, "%")
	v, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(s, "%")), 64)
	if err != nil || v == 0 {
		return 0, Errorf(ExitUsage, "--mde %q: want a non-zero absolute rate change (0.02) or relative change (10%%)", mde)
	}
	if rel {
		return p1 * (1 + v/100), nil
	}
	return p1 + v, nil
}

// issueItem fetches issue n and parses it as a work item.
func issueItem(ctx context.Context, c *github.Client, repo string, n int) (*gh.Issue, workitems.Item, error) {
	owner, name, err := github.SplitRepo(repo)
	if err != nil {
		return nil, workitems.Item{}, Wrap(ExitUsage, err)
	}
	is, _, err := c.REST.Issues.Get(ctx, owner, name, n)
	if err != nil {
		return nil, workitems.Item{}, err
	}
	it, _ := workitems.Parse(n, is.GetTitle(), is.GetBody(), is.GetType().GetName(), github.IssueLabels(is))
	return is, it, nil
}

// setStatus makes status:<to> the issue's only Delivery Status label.
func setStatus(ctx context.Context, c *github.Client, repo string, n int, labels []string, to string) error {
	owner, name, _ := github.SplitRepo(repo)
	target := fmt.Sprintf("%s#%d", repo, n)
	want := "status:" + to
	for _, l := range labels {
		if !strings.HasPrefix(l, "status:") || l == want {
			continue
		}
		_, err := c.Write(ctx, actions.Action{Kind: "github.issue.labels.remove", Target: target,
			Request: &actions.Request{Method: http.MethodDelete, Path: fmt.Sprintf("/repos/%s/issues/%d/labels/%s", repo, n, l)}, Payload: map[string]any{"label": l}},
			func(ctx context.Context) (any, error) {
				_, err := c.REST.Issues.RemoveLabelForIssue(ctx, owner, name, n, l)
				if github.NotFound(err) {
					err = nil
				}
				return nil, err
			})
		if err != nil {
			return err
		}
	}
	if slices.Contains(labels, want) {
		return nil
	}
	_, err := c.Write(ctx, actions.Action{Kind: "github.issue.labels.add", Target: target,
		Request: &actions.Request{Method: http.MethodPost, Path: fmt.Sprintf("/repos/%s/issues/%d/labels", repo, n)}, Payload: map[string]any{"labels": []string{want}}},
		func(ctx context.Context) (any, error) {
			_, _, err := c.REST.Issues.AddLabelsToIssue(ctx, owner, name, n, []string{want})
			return nil, err
		})
	return err
}

// writeResult sets the Result section of issue n's body.
func writeResult(ctx context.Context, c *github.Client, repo string, n int, body, result string) error {
	owner, name, _ := github.SplitRepo(repo)
	nb := workitems.SetSection(body, "Result", result)
	_, err := c.Write(ctx, actions.Action{Kind: "github.issue.result", Target: fmt.Sprintf("%s#%d", repo, n),
		Request: &actions.Request{Method: http.MethodPatch, Path: fmt.Sprintf("/repos/%s/issues/%d", repo, n)}, Payload: map[string]any{"result": result}},
		func(ctx context.Context) (any, error) {
			_, _, err := c.REST.Issues.Update(ctx, owner, name, n, gh.UpdateIssueRequest{Body: gh.Ptr(nb)})
			return nil, err
		})
	return err
}

func baselineLine(label string, b integrations.Baseline) string {
	return fmt.Sprintf("- %s: %s = %v; population: %s; window %s to %s; source: %s; fetched %s", label, b.Metric, b.Value, b.Population,
		b.Window.From.Format(time.RFC3339), b.Window.To.Format(time.RFC3339), experiment.Code(b.SourceQuery), b.FetchedAt.Format(time.RFC3339))
}

type experimentReport struct {
	Feature    int                     `json:"feature"`
	Experiment int                     `json:"experiment,omitempty"`
	Title      string                  `json:"title"`
	Flag       string                  `json:"flag"`
	Metric     string                  `json:"metric"`
	Baselines  []integrations.Baseline `json:"baselines"`
	Design     experiment.Design       `json:"design"`
	MinSample  int                     `json:"min_sample_per_arm"`
	Body       string                  `json:"body"`
	DryRun     bool                    `json:"dry_run,omitempty"`
}

// runExperiment designs an Experiment under a Feature or Initiative: a
// reproducible PostHog baseline conversion rate (the baseline gate), the
// minimum sample per arm from alpha, power, and the minimum detectable
// effect, and the Experiment issue linked as a sub-issue. The feature flag
// itself is created in PostHog or GrowthBook by a human: integration keys are
// read-only (Appendix B D-6.7a).
func runExperiment(ctx context.Context, e *Env, in Values) error {
	repo, err := e.RepoName(ctx)
	if err != nil {
		return err
	}
	flag, metric, exposure := strings.TrimSpace(in.String("flag")), strings.TrimSpace(in.String("metric")), strings.TrimSpace(in.String("exposure"))
	alpha, err := rate("alpha", in.String("alpha"), 0.05)
	if err != nil {
		return err
	}
	power, err := rate("power", in.String("power"), 0.8)
	if err != nil {
		return err
	}
	days := 28
	if in.Has("days") {
		days = in.Int("days")
	}
	if days < 1 || days > 366 {
		return Errorf(ExitUsage, "--days %d: want 1 to 366", days)
	}
	to := e.Now().UTC().Truncate(24 * time.Hour)
	w := integrations.Window{From: to.AddDate(0, 0, -days), To: to}
	// The flag key and event go into readout's HogQL later; refuse them now.
	if _, err := posthog.ArmsHogQL(flag, metric, w); err != nil {
		return err
	}
	if _, err := detectable(0.5, in.String("mde")); err != nil {
		return err
	}

	c, bot, err := e.BotGitHub(ctx, "")
	if err != nil {
		return err
	}
	parent := issueNumber(in.String("feature"))
	pis, pit, err := issueItem(ctx, c, repo, parent)
	if err != nil {
		return err
	}
	if pit.Type != "feature" && pit.Type != "initiative" {
		return Errorf(ExitUsage, "experiment: #%d is a %q; an Experiment's parent is a Feature or an Initiative", parent, pit.Type)
	}
	if pis.GetState() != "open" {
		return Errorf(ExitBlocked, "experiment: #%d is closed", parent)
	}
	owner, name, _ := github.SplitRepo(repo)
	subs, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.SubIssue, *gh.Response, error) {
		return c.REST.SubIssue.ListByIssue(ctx, owner, name, int64(parent), &o)
	})
	if err != nil {
		return err
	}
	for _, sub := range subs {
		s := (*gh.Issue)(sub)
		it, _ := workitems.Parse(s.GetNumber(), s.GetTitle(), s.GetBody(), "", github.IssueLabels(s))
		if it.Type == "experiment" && s.GetState() == "open" && strings.TrimSpace(it.Fields["flag"]) == flag {
			return Errorf(ExitBlocked, "experiment: #%d already experiments on flag %s under #%d", s.GetNumber(), flag, parent)
		}
	}

	ph, err := providers.PostHog(e.integrationDeps(repo))
	if err != nil {
		return err
	}
	pop, err := ph.Baseline(ctx, integrations.Query{Metric: "users", Filter: exposure, Window: w})
	if err != nil {
		return err
	}
	conv := fmt.Sprintf("event = '%s'", metric)
	if exposure != "" {
		conv = "(" + exposure + ") AND " + conv
	}
	cb, err := ph.Baseline(ctx, integrations.Query{Metric: "users", Filter: conv, Window: w})
	if err != nil {
		return err
	}
	if pop.Value <= 0 {
		return Errorf(ExitBlocked, "experiment: baseline gate: PostHog counts no persons in the population over %s to %s, so there is no baseline rate", w.From.Format(time.DateOnly), w.To.Format(time.DateOnly))
	}
	p1 := cb.Value / pop.Value
	p2, err := detectable(p1, in.String("mde"))
	if err != nil {
		return err
	}
	d := experiment.Design{Baseline: p1, Detectable: p2, Alpha: alpha, Power: power}
	n, err := d.MinSample()
	if err != nil {
		return Wrap(ExitUsage, err)
	}

	rep := experimentReport{Feature: parent, Flag: flag, Metric: metric, Baselines: []integrations.Baseline{pop, cb}, Design: d, MinSample: n, DryRun: c.DryRun()}
	rep.Title = fmt.Sprintf("Experiment: %s on %s", flag, metric)
	t, _ := workitems.FindType("experiment")
	rep.Body = workitems.Render(t, map[string]string{
		workitems.FieldSummary: in.String("hypothesis"),
		"metric":               metric,
		workitems.FieldBaseline: fmt.Sprintf("Conversion rate %v (%v of %v persons converted).\n\n%s\n%s", p1, cb.Value, pop.Value,
			baselineLine("population", pop), baselineLine("converted", cb)),
		"flag":       flag,
		"min_sample": experiment.FormatMinSample(n, d),
	})
	labels := []string{"type:experiment", "status:planned"}
	if bot.Tag != "" {
		labels = append(labels, bot.Tag)
	}
	var created *gh.Issue
	_, err = c.Write(ctx, actions.Action{Kind: "github.issue.create", Target: repo,
		Request: &actions.Request{Method: http.MethodPost, Path: fmt.Sprintf("/repos/%s/issues", repo)}, Payload: map[string]any{"title": rep.Title, "parent": parent, "min_sample": n}},
		func(ctx context.Context) (any, error) {
			is, _, err := c.REST.Issues.Create(ctx, owner, name, gh.CreateIssueRequest{Title: rep.Title, Body: gh.Ptr(rep.Body), Labels: labels})
			created = is
			return map[string]any{"number": is.GetNumber()}, err
		})
	if err != nil {
		return err
	}
	if created != nil {
		rep.Experiment = created.GetNumber()
		_, err = c.Write(ctx, actions.Action{Kind: "github.sub-issue.add", Target: fmt.Sprintf("%s#%d", repo, parent),
			Request: &actions.Request{Method: http.MethodPost, Path: fmt.Sprintf("/repos/%s/issues/%d/sub_issues", repo, parent)}, Payload: map[string]any{"sub_issue": rep.Experiment}},
			func(ctx context.Context) (any, error) {
				_, _, err := c.REST.SubIssue.Add(ctx, owner, name, int64(parent), gh.SubIssueRequest{SubIssueID: created.GetID()})
				return nil, err
			})
		if err != nil {
			return err
		}
	}
	return e.Emit(rep, func(w io.Writer) error {
		who := fmt.Sprintf("#%d", rep.Experiment)
		if rep.DryRun {
			who = "(dry run, not created)"
		}
		fmt.Fprintf(w, "Experiment %s under #%d: baseline rate %.4f, detectable %.4f; minimum sample %d per arm (alpha %v, power %v)\n", who, parent, p1, p2, n, alpha, power)
		return nil
	})
}

var armName = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,64}$`)

type readoutReport struct {
	Experiment int               `json:"experiment"`
	Source     string            `json:"source"`
	Window     experiment.Window `json:"window"`
	MinSample  int               `json:"min_sample_per_arm"`
	Lift       experiment.Lift   `json:"lift"`
	Result     string            `json:"result"`
	DryRun     bool              `json:"dry_run,omitempty"`
}

// runReadout reads per-arm counts for an Experiment from PostHog or
// GrowthBook, refuses while either arm is under the minimum sample (the
// min-sample gate, exit 4, nothing written), and otherwise writes the lift as
// the Experiment's Result and marks it measured.
func runReadout(ctx context.Context, e *Env, in Values) error {
	repo, err := e.RepoName(ctx)
	if err != nil {
		return err
	}
	c, _, err := e.BotGitHub(ctx, "")
	if err != nil {
		return err
	}
	n := issueNumber(in.String("experiment"))
	is, it, err := issueItem(ctx, c, repo, n)
	if err != nil {
		return err
	}
	if it.Type != "experiment" {
		return Errorf(ExitUsage, "readout: #%d is a %q, not an Experiment", n, it.Type)
	}
	if is.GetState() != "open" {
		return Errorf(ExitBlocked, "readout: Experiment #%d is closed", n)
	}
	minSample, d, err := experiment.ParseMinSample(it.Fields["min_sample"])
	if err != nil {
		return Wrap(ExitBlocked, fmt.Errorf("readout: min-sample gate: #%d: %w", n, err))
	}
	flag, metric := strings.TrimSpace(it.Fields["flag"]), strings.TrimSpace(it.Fields["metric"])
	source := in.String("source")
	if source == "" {
		source = "posthog"
	}
	deps := e.integrationDeps(repo)
	var arms []experiment.Arm
	var w experiment.Window
	var query string
	switch source {
	case "posthog":
		from := is.GetCreatedAt().UTC()
		if s := in.String("from"); s != "" {
			if from, err = time.Parse(time.DateOnly, s); err != nil {
				return Errorf(ExitUsage, "--from %q: want YYYY-MM-DD", s)
			}
		}
		w = experiment.Window{From: from, To: e.Now().UTC().Truncate(time.Second)}
		ph, err := providers.PostHog(deps)
		if err != nil {
			return err
		}
		got, q, err := ph.Arms(ctx, flag, metric, integrations.Window{From: w.From, To: w.To})
		if err != nil {
			return err
		}
		query = q
		// PostHog names the control variant "control"; it goes first.
		i := slices.IndexFunc(got, func(a posthog.Arm) bool { return a.Variant == "control" })
		if i < 0 {
			return Wrap(ExitExternal, fmt.Errorf("%w: PostHog has no %q variant for flag %s in the window", integrations.ErrProvider, "control", flag))
		}
		arms = append(arms, experiment.Arm{Name: got[i].Variant, Users: got[i].Users, Converted: got[i].Converted})
		for j, a := range got {
			if j != i {
				arms = append(arms, experiment.Arm{Name: a.Variant, Users: a.Users, Converted: a.Converted})
			}
		}
	case "growthbook":
		if m := in.String("metric"); m != "" {
			metric = m
		}
		gb, err := providers.GrowthBook(deps)
		if err != nil {
			return err
		}
		got, gw, q, err := gb.Results(ctx, flag, metric)
		if err != nil {
			return err
		}
		query, w = q, experiment.Window{From: gw.From, To: gw.To}
		for _, v := range got {
			arms = append(arms, experiment.Arm{Name: v.Key, Users: v.Users, Converted: v.Converted})
		}
	}
	if len(arms) != 2 {
		return Wrap(ExitExternal, fmt.Errorf("%w: %s returned %d arms for flag %s; readout compares exactly two (control and one treatment)", integrations.ErrProvider, source, len(arms), flag))
	}
	for _, a := range arms {
		// Arm names come from the provider and land in the issue body.
		if !armName.MatchString(a.Name) {
			return Wrap(ExitExternal, fmt.Errorf("%w: %s arm name %q: want letters, digits, ., _ and -", integrations.ErrProvider, source, a.Name))
		}
	}
	lift, err := experiment.Readout(arms[0], arms[1], minSample, d.Alpha)
	if err != nil {
		if errors.Is(err, experiment.ErrUnderSample) {
			return Wrap(ExitBlocked, fmt.Errorf("readout: min-sample gate: %w; nothing written", err))
		}
		return Wrap(ExitExternal, err)
	}
	rep := readoutReport{Experiment: n, Source: source, Window: w, MinSample: minSample, Lift: lift, DryRun: c.DryRun()}
	rep.Result = experiment.ResultText(lift, minSample, w, query, e.Now())
	// Status first: the Ready guard strips status:ready on a body edit.
	if err := setStatus(ctx, c, repo, n, github.IssueLabels(is), "measured"); err != nil {
		return err
	}
	if err := writeResult(ctx, c, repo, n, is.GetBody(), rep.Result); err != nil {
		return err
	}
	return e.Emit(rep, func(w io.Writer) error {
		fmt.Fprintf(w, "Experiment #%d: %s\n", n, strings.SplitN(rep.Result, "\n", 2)[0])
		return nil
	})
}
