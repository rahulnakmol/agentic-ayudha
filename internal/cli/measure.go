package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/identity"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations/exports"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations/providers"
	"github.com/rahulnakmol/agentic-ayudha/internal/measure"
	"github.com/rahulnakmol/agentic-ayudha/internal/status"
)

func init() { register("measure", runMeasure) }

type snapshotInfo struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type northStar struct {
	Event string `json:"event"`
	Value int64  `json:"value"` // distinct users, or minor units for revenue
	Query string `json:"query"`
}

type measureReport struct {
	Window      measure.Window    `json:"window"`
	Engine      string            `json:"engine,omitempty"`
	Sources     []*exports.File   `json:"sources"`
	Snapshot    *snapshotInfo     `json:"snapshot,omitempty"`
	CohortQuery string            `json:"cohort_query,omitempty"`
	Cohorts     []measure.Cohort  `json:"cohorts"`
	NorthStar   *northStar        `json:"north_star,omitempty"`
	Issue       int               `json:"issue,omitempty"`
	Measured    *measure.Measured `json:"measured,omitempty"`
	Result      string            `json:"result,omitempty"`
	Closed      []int             `json:"closed,omitempty"`
	DryRun      bool              `json:"dry_run,omitempty"`
}

func day(name, raw string, def time.Time) (time.Time, error) {
	if raw == "" {
		return def, nil
	}
	t, err := time.Parse(time.DateOnly, raw)
	if err != nil {
		return t, Errorf(ExitUsage, "--%s %q: want YYYY-MM-DD", name, raw)
	}
	return t, nil
}

// runMeasure snapshots the given exports into SQLite, reports the weekly
// AARRR cohorts and the North Star, and, for a Goal or Initiative, measures
// its metric against its baseline and target, writes the Result, and applies
// the parent-close rule to it and the Features under it.
func runMeasure(ctx context.Context, e *Env, in Values) error {
	l, err := e.Config()
	if err != nil {
		return err
	}
	now := e.Now().UTC()
	to, err := day("to", in.String("to"), now.Truncate(24*time.Hour))
	if err != nil {
		return err
	}
	from, err := day("from", in.String("from"), to.AddDate(0, 0, -28))
	if err != nil {
		return err
	}
	if !from.Before(to) {
		return Errorf(ExitUsage, "measure: window %s to %s: want --from before --to", from.Format(time.DateOnly), to.Format(time.DateOnly))
	}
	w := measure.Window{From: from, To: to}
	stages := measure.NewStages(l.Config.Measure)
	rep := measureReport{Window: w, Sources: []*exports.File{}, Cohorts: []measure.Cohort{}}

	for _, k := range exports.Kinds {
		if !in.Has(k.Name) {
			continue
		}
		f, err := exports.Read(k.Name, in.String(k.Name))
		if err != nil {
			return Wrap(ExitUsage, err)
		}
		rep.Sources = append(rep.Sources, f)
	}
	scope := "all"
	if in.Has("issue") {
		rep.Issue = issueNumber(in.String("issue"))
		scope = fmt.Sprint(rep.Issue)
	}
	stamp := now.Format("20060102T150405Z")
	if live := strings.TrimSpace(in.String("live")); live != "" {
		repo, _ := e.RepoName(ctx)
		for _, src := range strings.Split(live, ",") {
			f, err := pullLive(ctx, e, repo, strings.TrimSpace(src), w, filepath.Join(e.stateDir(), "state", "measure", scope+"-"+stamp+"-"+strings.TrimSpace(src)+".jsonl"))
			if err != nil {
				return err
			}
			rep.Sources = append(rep.Sources, f)
		}
	}
	var eng measure.Engine = measure.SQLite{}
	if in.String("engine") == "duckdb" {
		d, err := measure.FindDuckDB(e.lookPath())
		if err != nil {
			return Wrap(ExitExternal, err)
		}
		eng = d
	}
	var nsValue int64
	if len(rep.Sources) > 0 {
		rep.Engine = eng.Name()
		path := filepath.Join(e.stateDir(), "state", "measure", scope+"-"+stamp+".sqlite")
		sha, err := measure.Snapshot(ctx, path, w, stages, rep.Sources, now)
		if err != nil {
			return measureErr(err)
		}
		rep.Snapshot = &snapshotInfo{Path: path, SHA256: sha}
		rep.CohortQuery = measure.CohortSQL(stages)
		if rep.Cohorts, err = measure.Cohorts(ctx, eng, path, w, stages); err != nil {
			return measureErr(err)
		}
		if stages.NorthStar != "" {
			v, q, err := measure.NorthStar(ctx, eng, path, stages)
			if err != nil {
				return measureErr(err)
			}
			nsValue, rep.NorthStar = v, &northStar{Event: stages.NorthStar, Value: v, Query: q}
		}
	}

	if rep.Issue != 0 {
		if err := measureIssue(ctx, e, &rep, stages, nsValue, in.String("metric")); err != nil {
			return err
		}
	} else if len(rep.Sources) == 0 {
		return Errorf(ExitUsage, "measure: nothing to do: pass exports (--billing, --appinsights, --aws, --gcp), --live, or a Goal or Initiative")
	}
	return e.Emit(rep, func(out io.Writer) error {
		for _, f := range rep.Sources {
			fmt.Fprintf(out, "%s %s: %d rows, %d skipped, %d excluded (sha256 %s)\n", f.Kind, f.Path, f.Rows, f.Skipped, f.Excluded, f.SHA256[:12])
		}
		if rep.Snapshot != nil {
			fmt.Fprintf(out, "snapshot %s (sha256 %s, %s)\n\n", rep.Snapshot.Path, rep.Snapshot.SHA256[:12], rep.Engine)
			fmt.Fprintln(out, cohortTable(rep.Cohorts))
		}
		if rep.NorthStar != nil {
			fmt.Fprintf(out, "North Star (%s): %d\n", rep.NorthStar.Event, rep.NorthStar.Value)
		}
		if rep.Result != "" {
			fmt.Fprintf(out, "#%d: %s\n", rep.Issue, strings.SplitN(rep.Result, "\n", 2)[0])
		}
		for _, n := range rep.Closed {
			fmt.Fprintf(out, "closed #%d (parent-close rule)\n", n)
		}
		return nil
	})
}

// pullLive pulls the window's events from a cloud integration (azure, aws,
// gcp) into a JSON-lines file in that source's export format, then reads it
// back through the export reader, so a live pull and an export file give the
// same rows and cohorts. The file stays beside the snapshot (git-ignored)
// and its SHA-256 and query are recorded, so the Result can be rebuilt.
func pullLive(ctx context.Context, e *Env, repo, src string, w measure.Window, path string) (*exports.File, error) {
	kind, ok := providers.Exporters[src]
	if !ok {
		return nil, Errorf(ExitUsage, "measure --live %q: want a comma-separated list of azure, aws, gcp", src)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	out, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	got, query, err := providers.Export(ctx, e.integrationDeps(repo), src, integrations.Window{From: w.From, To: w.To}, out)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(path)
		return nil, err
	}
	if got != kind {
		return nil, fmt.Errorf("measure --live %s: pulled %s events, want %s", src, got, kind)
	}
	f, err := exports.Read(kind, path)
	if err != nil {
		return nil, Wrap(ExitExternal, fmt.Errorf("measure --live %s: %w", src, err))
	}
	f.Live, f.Query = src, query
	return f, nil
}

func cohortTable(cs []measure.Cohort) string {
	var b strings.Builder
	b.WriteString("| Cohort week | Acquired | Activated | Retained | Referred | Paying | Revenue | Complete |\n|---|---|---|---|---|---|---|---|\n")
	for _, c := range cs {
		fmt.Fprintf(&b, "| %s | %d | %d | %d | %d | %d | %.2f | %t |\n", c.Start.Format(time.DateOnly), c.Acquired, c.Activated, c.Retained, c.Referred, c.Paying, float64(c.RevenueMinor)/100, c.Complete)
	}
	return b.String()
}

func measureErr(err error) error {
	switch {
	case errors.Is(err, measure.ErrNoData):
		return Wrap(ExitBlocked, err)
	case errors.Is(err, measure.ErrEngine):
		return Wrap(ExitExternal, err)
	}
	return err
}

// measureIssue measures a Goal or Initiative and writes its Result.
func measureIssue(ctx context.Context, e *Env, rep *measureReport, stages measure.Stages, nsValue int64, metric string) error {
	repo, err := e.RepoName(ctx)
	if err != nil {
		return err
	}
	c, bot, err := e.BotGitHub(ctx, "")
	if err != nil {
		return err
	}
	n := rep.Issue
	is, it, err := issueItem(ctx, c, repo, n)
	if err != nil {
		return err
	}
	if it.Type != "goal" && it.Type != "initiative" {
		return Errorf(ExitUsage, "measure: #%d is a %q; measure writes Results on Goals and Initiatives", n, it.Type)
	}
	if is.GetState() != "open" {
		return Errorf(ExitBlocked, "measure: #%d is closed", n)
	}
	if metric == "" {
		metric = it.Fields["metric"]
	}
	spec, err := measure.ParseSpec(metric)
	if err != nil {
		return Wrap(ExitUsage, err)
	}
	baseline, err := measure.ParseBaseline(it.Fields["baseline"])
	if err != nil {
		return Wrap(ExitBlocked, fmt.Errorf("measure: baseline gate: #%d: %w", n, err))
	}
	target, err := measure.ParseTarget(it.Fields["target"], baseline)
	if err != nil {
		return Wrap(ExitBlocked, fmt.Errorf("measure: #%d: %w", n, err))
	}
	m := measure.Measured{Spec: spec, Baseline: baseline, Target: target, Window: rep.Window}
	switch spec.Source {
	case "":
		if rep.Snapshot == nil {
			return Errorf(ExitUsage, "measure: %s is read from exports; pass --billing, --appinsights, --aws, --gcp, or --live", spec.Metric)
		}
		if m.Value, m.Detail, err = measure.Value(spec.Metric, rep.Cohorts, nsValue, stages); err != nil {
			return measureErr(err)
		}
		m.Query = rep.CohortQuery
		if spec.Metric == "north_star" {
			m.Query = rep.NorthStar.Query
		}
		m.Snapshot, m.SHA256, m.Engine = filepath.Base(rep.Snapshot.Path), rep.Snapshot.SHA256, rep.Engine
		for _, f := range rep.Sources {
			src := fmt.Sprintf("%s %s sha256 %s", f.Kind, filepath.Base(f.Path), f.SHA256)
			if f.Live != "" {
				src += fmt.Sprintf(" (pulled live from %s: %s)", f.Live, strings.Join(strings.Fields(f.Query), " "))
			}
			m.Sources = append(m.Sources, src)
		}
	case "aayu":
		if err := founderMinutes(ctx, e, c, bot, repo, &m); err != nil {
			return err
		}
	default:
		b, err := providers.Baseline(ctx, e.integrationDeps(repo), spec.Source, integrations.Query{Metric: spec.Metric, Filter: spec.Filter,
			Window: integrations.Window{From: rep.Window.From, To: rep.Window.To}})
		if err != nil {
			return err
		}
		m.Value, m.Detail, m.Query = b.Value, b.Population, b.SourceQuery
		m.Window = measure.Window{From: b.Window.From, To: b.Window.To}
	}
	m.Met = target.Met(m.Value)
	rep.Measured = &m
	rep.Result = measure.ResultText(m, e.Now())
	rep.DryRun = c.DryRun()
	to := "measured"
	if m.Met {
		to = "done"
	}
	if err := setStatus(ctx, c, repo, n, github.IssueLabels(is), to); err != nil {
		return err
	}
	if err := writeResult(ctx, c, repo, n, is.GetBody(), rep.Result); err != nil {
		return err
	}
	if rep.DryRun {
		return nil
	}
	// The parent-close rule, on the measured item and everything under it.
	wk, err := e.walker(ctx, c, repo)
	if err != nil {
		return err
	}
	r, err := wk.Roll(ctx, n)
	if err != nil {
		return err
	}
	rep.Closed, err = parentClose(ctx, c, repo, []*status.Rollup{r}, nil)
	return err
}

// founderMinutes measures aayu:founder_minutes_per_pr (Story #989) from the
// actions segments on the ledger and the pull requests merged into dev in
// the window.
func founderMinutes(ctx context.Context, e *Env, c *github.Client, bot identity.Bot, repo string, m *measure.Measured) error {
	if m.Spec.Metric != measure.FounderMetric || m.Spec.Filter != "" {
		return Errorf(ExitUsage, "measure: metric %q: aayu measures only aayu:%s", m.Spec, measure.FounderMetric)
	}
	l, err := e.Config()
	if err != nil {
		return err
	}
	w, err := e.Ledger(ctx, bot.Token, botIdent(bot), bot.Tag)
	if err != nil {
		return err
	}
	tree, err := w.Read(ctx)
	if err != nil {
		return err
	}
	defer tree.Close()
	paths, err := tree.List(ctx, "actions")
	if err != nil {
		return err
	}
	var records []actions.Record
	segments := 0
	for _, p := range paths {
		if !strings.HasSuffix(p, ".jsonl") {
			continue
		}
		b, err := tree.File(ctx, p)
		if err != nil {
			return err
		}
		segments++
		for i, line := range strings.Split(string(b), "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			var r actions.Record
			if err := json.Unmarshal([]byte(line), &r); err != nil {
				// A dropped record would hide a person's minutes and understate the measure.
				return Wrap(ExitBlocked, fmt.Errorf("measure: ledger %s: %s line %d: %w", tree.Tip, p, i+1, err))
			}
			records = append(records, r)
		}
	}
	owner, name, err := github.SplitRepo(repo)
	if err != nil {
		return err
	}
	dev := e.devBranch()
	prs, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.PullRequest, *gh.Response, error) {
		return c.REST.PullRequests.List(ctx, owner, name, &gh.PullRequestListOptions{State: "closed", Base: dev, ListOptions: o})
	})
	if err != nil {
		return err
	}
	merged := 0
	for _, p := range prs {
		if at := p.GetMergedAt().Time; p.MergedAt != nil && p.GetBase().GetRef() == dev && !at.Before(m.Window.From) && at.Before(m.Window.To) {
			merged++
		}
	}
	if m.Value, m.Detail, err = measure.Founder(records, merged, m.Window, l.Config.Outcomes, dev); err != nil {
		return measureErr(err)
	}
	m.Query = fmt.Sprintf("ledger %s: %d actions segments under actions/, records with identity human; GET /repos/%s/pulls?state=closed&base=%s, merged_at in the window",
		tree.Tip, segments, repo, dev)
	return nil
}
