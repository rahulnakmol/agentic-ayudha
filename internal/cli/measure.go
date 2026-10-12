package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/github"
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
	var eng measure.Engine = measure.SQLite{}
	if in.String("engine") == "duckdb" {
		d, err := measure.FindDuckDB(e.lookPath())
		if err != nil {
			return Wrap(ExitExternal, err)
		}
		eng = d
	}
	scope := "all"
	if in.Has("issue") {
		rep.Issue = issueNumber(in.String("issue"))
		scope = fmt.Sprint(rep.Issue)
	}
	var nsValue int64
	if len(rep.Sources) > 0 {
		rep.Engine = eng.Name()
		path := filepath.Join(e.stateDir(), "state", "measure", fmt.Sprintf("%s-%s.sqlite", scope, now.Format("20060102T150405Z")))
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
		return Errorf(ExitUsage, "measure: nothing to do: pass exports (--billing, --appinsights, --aws, --gcp) or a Goal or Initiative")
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
	c, _, err := e.BotGitHub(ctx, "")
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
	if spec.Source == "" {
		if rep.Snapshot == nil {
			return Errorf(ExitUsage, "measure: %s is read from exports; pass --billing, --appinsights, --aws, or --gcp", spec.Metric)
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
			m.Sources = append(m.Sources, fmt.Sprintf("%s %s sha256 %s", f.Kind, filepath.Base(f.Path), f.SHA256))
		}
	} else {
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
