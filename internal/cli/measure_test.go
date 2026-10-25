package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations/exports"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations/fixture"
	"github.com/rahulnakmol/agentic-ayudha/internal/measure"
)

// measureFlags points measure at the synthetic exports in
// internal/measure/testdata over 2026-09-07 to 2026-10-12 (five weeks) with
// [measure] acquisition signed_up, activation created_project (7 days),
// retention any event, referral invited_user, North Star created_project.
func measureFlags(extra ...string) []string {
	td := filepath.Join("..", "measure", "testdata")
	return append([]string{"--repo", "o/r", "--json", "--from", "2026-09-07", "--to", "2026-10-12",
		"--billing", filepath.Join(td, "billing.csv"), "--appinsights", filepath.Join(td, "appinsights.jsonl"),
		"--aws", filepath.Join(td, "aws.jsonl"), "--gcp", filepath.Join(td, "gcp.jsonl"),
		"--set", "measure.acquisition=signed_up", "--set", "measure.activation=created_project",
		"--set", "measure.referral=invited_user", "--set", "measure.north_star=created_project"}, extra...)
}

type measureOut struct {
	Sources   []*exports.File   `json:"sources"`
	Cohorts   []measure.Cohort  `json:"cohorts"`
	NorthStar *northStar        `json:"north_star"`
	Measured  *measure.Measured `json:"measured"`
	Closed    []int             `json:"closed"`
	Snapshot  *snapshotInfo     `json:"snapshot"`
}

// wantCohorts is worked out by hand from the fixture (times UTC; a stage's
// window is half-open):
//
//	week 0, from 09-07: u1 signed up 09-07 10:00 (appinsights), u2 09-09 09:00 (gcp), u3 09-12 23:00 (aws)
//	  activated: u1 (created_project 09-08); u2's comes on day 16
//	  retained (any event in days 7-13): u1 (aws 09-15 12:00), u3 (appinsights 09-19 23:00, exactly day 7;
//	    its aws event at 22:59 is day 6); u2 has none in 09-16 09:00..09-23 09:00
//	  referred: u2 (invited 09-10); paying: u1 20.00, u3 9.99 (its 15.50 failed) = 29.99
//	week 1, from 09-14: u4 09-14 00:00 (appinsights, UserAuthenticatedId over UserId), u5 09-16 (gcp pseudo id)
//	  activated: u4 (09-20 23:59, day 6); u5's created_project is exactly day 7, so not activated, but retained
//	  paying: u4 20.00 less 5.00 refunded = 15.00
//	week 2: nobody signs up
//	week 3, from 09-28: u6 10-01 (aws): activated 10-02, referred 10-03; incomplete, since 10-05 + 14 days > 10-12
var wantCohorts = []measure.Cohort{
	{Week: 0, Start: time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC), Acquired: 3, Activated: 1, Retained: 2, Referred: 1, Paying: 2, RevenueMinor: 2999, Complete: true},
	{Week: 1, Start: time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC), Acquired: 2, Activated: 1, Retained: 1, Referred: 0, Paying: 1, RevenueMinor: 1500, Complete: true},
	{Week: 3, Start: time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC), Acquired: 1, Activated: 1, Retained: 0, Referred: 1, Paying: 0, RevenueMinor: 0, Complete: false},
}

func checkCohorts(t *testing.T, out string) measureOut {
	t.Helper()
	var got measureOut
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if !reflect.DeepEqual(got.Cohorts, wantCohorts) {
		t.Errorf("cohorts:\n got %+v\nwant %+v", got.Cohorts, wantCohorts)
	}
	// North Star, distinct users with created_project in the window: u0 (twice), u1 (09-08; its
	// 10-12 00:00 and 09-06 events fall outside), u2, u4, u5, u6, u7 = 7.
	if got.NorthStar == nil || got.NorthStar.Value != 7 {
		t.Errorf("north star: %+v", got.NorthStar)
	}
	return got
}

// WP7.2 accept: fixture exports → expected cohorts. Then a Goal measured on
// activation meets its target, gets the Result, and the parent-close rule
// closes it and the closable Feature under it; an Initiative measured on
// revenue misses its target and stays open.
func TestMeasureCohortsAndResults(t *testing.T) {
	g := fake.New(t, "o/r", "User")
	goal := g.AddIssue("Activate more sign-ups", "### Summary\n\nMore new users activate.\n\n### Metric\n\nactivation\n\n### Baseline\n\n0.25 of acquired users activate in 7 days (2026-08).\n\n### Target\n\n≥ 0.35 by 2026-12-31", "type:goal", "status:in-progress")
	ini := g.AddIssue("Guided first project", "", "type:initiative", "status:done")
	feat := g.AddIssue("Project template picker", "### Summary\n\nPick a template.\n\n### Result\n\nShipped; readout #9.", "type:feature", "status:released")
	story := g.AddIssue("Template list", "", "type:story", "status:released")
	link := func(p, c *fake.Issue) { p.Subs = append(p.Subs, c.Number); c.Parent = p.Number }
	link(goal, ini)
	link(ini, feat)
	link(feat, story)
	g.AddIssue("Grow paid revenue", "### Summary\n\nMore revenue.\n\n### Metric\n\nnorth_star\n\n### Baseline\n\n80 USD per five weeks\n\n### Target\n\n120", "type:initiative", "status:in-progress")
	dir := t.TempDir()
	env := map[string]string{"AAYU_GITHUB_TOKEN": "t", "XDG_STATE_HOME": filepath.Join(dir, "state")}
	opts := Options{GitHubAPI: g.URL(), GitHubGap: -1, Now: func() time.Time { return time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC) }}

	// Cohorts only: nothing written to GitHub.
	code, out, errOut := run(t, env, opts, append([]string{"measure"}, measureFlags()...)...)
	if code != 0 {
		t.Fatalf("measure: exit %d: %s", code, errOut)
	}
	got := checkCohorts(t, out)
	rows := map[string][3]int{}
	for _, f := range got.Sources {
		rows[f.Kind] = [3]int{f.Rows, f.Skipped, f.Excluded}
	}
	// billing: 4 paid rows, the 12.345 amount skipped, the failed charge excluded; aws: the
	// non-numeric timestamp and the non-JSON line skipped.
	if want := map[string][3]int{"billing": {4, 1, 1}, "appinsights": {7, 0, 0}, "aws": {7, 2, 0}, "gcp": {8, 0, 0}}; !reflect.DeepEqual(rows, want) {
		t.Errorf("sources rows/skipped/excluded = %v, want %v", rows, want)
	}
	if got.Snapshot == nil || !strings.HasSuffix(got.Snapshot.Path, ".sqlite") || len(g.Writes()) != 0 {
		t.Fatalf("snapshot %+v, GitHub writes %v", got.Snapshot, g.Writes())
	}

	// Goal #1 on activation: complete cohorts (weeks 0 and 1) have 2 of 5 acquired users
	// activated = 0.4, against the target >= 0.35 from the baseline 0.25: met.
	code, out, errOut = run(t, env, opts, append([]string{"measure", "1"}, measureFlags()...)...)
	if code != 0 {
		t.Fatalf("measure 1: exit %d: %s", code, errOut)
	}
	got = checkCohorts(t, out)
	if m := got.Measured; m == nil || m.Value != 0.4 || m.Baseline != 0.25 || m.Target != (measure.Target{Op: ">=", Value: 0.35}) || !m.Met {
		t.Errorf("measured: %+v", got.Measured)
	}
	if !slices.Equal(got.Closed, []int{1, 3}) {
		t.Errorf("closed %v, want the Goal (target met, its Initiative done) and the Feature (released child, Result)", got.Closed)
	}
	g.Mu.Lock()
	body, state := g.Issues[1].Body, g.Issues[1].State
	g.Mu.Unlock()
	if state != "closed" || !strings.Contains(body, "### Result\n\nTarget met: activation = 0.4 against the target >= 0.35 (baseline 0.25, change +0.15).") ||
		!slices.Contains(g.LabelNames(1), "status:done") {
		t.Errorf("Goal after measure: %s, labels %v:\n%s", state, g.LabelNames(1), body)
	}

	// Initiative #5 on the North Star as paid revenue: 20.00 + 9.99 + 15.00 + 50.00 (u0 never
	// signed up but paid) = 94.99, against 120 from the baseline 80: not met, so it stays open.
	code, out, errOut = run(t, env, opts, append([]string{"measure", "5"}, measureFlags("--set", "measure.north_star=revenue")...)...)
	if code != 0 {
		t.Fatalf("measure 5: exit %d: %s", code, errOut)
	}
	got = measureOut{}
	_ = json.Unmarshal([]byte(out), &got)
	if m := got.Measured; m == nil || m.Value != 94.99 || m.Met || len(got.Closed) != 0 {
		t.Errorf("revenue measure: %+v, closed %v", got.Measured, got.Closed)
	}
	g.Mu.Lock()
	body, state = g.Issues[5].Body, g.Issues[5].State
	g.Mu.Unlock()
	if state != "open" || !strings.Contains(body, "Target not met: north_star = 94.99 against the target >= 120 (baseline 80, change +14.99).") ||
		!slices.Contains(g.LabelNames(5), "status:measured") {
		t.Errorf("Initiative after measure: %s, labels %v:\n%s", state, g.LabelNames(5), body)
	}
}

// The optional DuckDB engine answers the same queries over the same
// snapshot; skipped when duckdb is not installed (it never is in CI).
func TestMeasureDuckDB(t *testing.T) {
	if _, err := exec.LookPath("duckdb"); err != nil {
		t.Skip("duckdb is not on PATH (optional)")
	}
	env := map[string]string{"XDG_STATE_HOME": filepath.Join(t.TempDir(), "state")}
	opts := Options{LookPath: exec.LookPath, Now: func() time.Time { return time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC) }}
	code, out, errOut := run(t, env, opts, append([]string{"measure", "--engine", "duckdb"}, measureFlags()...)...)
	if code != 0 {
		t.Fatalf("measure --engine duckdb: exit %d: %s", code, errOut)
	}
	checkCohorts(t, out)
}

// WP7.2b: --live pulls the same events from Application Insights,
// CloudWatch RUM, and the GA4 BigQuery export that the export files hold
// (the recording carries the files' rows inside the window), so the cohorts
// and North Star are exactly those of the files; each pull keeps its JSON
// lines file, SHA-256, and query.
func TestMeasureLiveMatchesExports(t *testing.T) {
	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC) // RUM serves the last 30 days
	clouds := fixture.NewClouds(t, now)
	r := &fixture.Replayer{T: t, Rec: fixture.Load(t, filepath.Join("testdata", "measure-live.json")), Check: func(req *http.Request, body []byte) error {
		if strings.HasSuffix(req.URL.Host, ".amazonaws.com") {
			return fixture.SigV4()(req, body)
		}
		return nil
	}}
	dir := t.TempDir()
	env := map[string]string{"XDG_STATE_HOME": filepath.Join(dir, "state"),
		"AAYU_SECRET_AZURE": fixture.AzureClient + ":" + fixture.AzureSecret, "AAYU_SECRET_AWS": fixture.AWSKeyID + ":" + fixture.AWSSecret, "AAYU_SECRET_GCP": clouds.GCPKeyJSON()}
	opts := Options{HTTP: &http.Client{Transport: clouds.Route(r)}, Now: func() time.Time { return now }}
	td := filepath.Join("..", "measure", "testdata")
	args := []string{"measure", "--repo", "o/r", "--json", "--from", "2026-09-07", "--to", "2026-10-12", "--billing", filepath.Join(td, "billing.csv"), "--live", "azure,aws,gcp",
		"--set", "measure.acquisition=signed_up", "--set", "measure.activation=created_project", "--set", "measure.referral=invited_user", "--set", "measure.north_star=created_project",
		"--set", "integrations.azure.secret=azure", "--set", "integrations.azure.scope=read", "--set", "integrations.azure.org=" + fixture.AzureTenant,
		"--set", "integrations.azure.project=/subscriptions/0b1c2d3e-0000-4000-8000-000000000001/resourceGroups/rg-benefits/providers/Microsoft.Insights/components/benefits-qa",
		"--set", "integrations.aws.secret=aws", "--set", "integrations.aws.scope=read", "--set", "integrations.aws.region=eu-west-1", "--set", "integrations.aws.project=benefits-qa",
		"--set", "integrations.gcp.secret=gcp", "--set", "integrations.gcp.scope=read", "--set", "integrations.gcp.project=acme-prod.analytics_123456789"}
	code, out, errOut := run(t, env, opts, args...)
	if code != 0 {
		t.Fatalf("measure --live: exit %d: %s", code, errOut)
	}
	r.Done()
	got := checkCohorts(t, out)
	rows := map[string][3]int{}
	for _, f := range got.Sources {
		rows[f.Kind] = [3]int{f.Rows, f.Skipped, f.Excluded}
		if f.Kind != "billing" && (f.Live == "" || f.Query == "" || !strings.HasSuffix(f.Path, "-"+f.Live+".jsonl") || len(f.SHA256) != 64) {
			t.Errorf("live source %+v: want its cloud, query, JSON lines file, and SHA-256", f)
		}
	}
	// Only rows inside the window come back (the classic Application Insights row and the
	// GA4 row at 10-12 00:00 are outside it); RUM's two unusable events are skipped as in the file.
	if want := map[string][3]int{"billing": {4, 1, 1}, "appinsights": {6, 0, 0}, "aws": {7, 2, 0}, "gcp": {7, 0, 0}}; !reflect.DeepEqual(rows, want) {
		t.Errorf("sources rows/skipped/excluded = %v, want %v", rows, want)
	}
	if ex := clouds.Exchanges(); !slices.Equal(ex, []string{"entra client_secret https://management.azure.com/.default", "entra client_secret https://api.loganalytics.io/.default", "google jwt-bearer " + fixture.GCPServiceAccount}) {
		t.Errorf("token exchanges %q", ex)
	}
}

// Story #989, criteria 2 and 3, end to end: a Goal measured on
// aayu:founder_minutes_per_pr reads the actions segments on the ledger and
// the pull requests merged into dev. The segments hold human actions at
// 09:00, 09:04, 09:09 and 11:30 on 10-12 (13 founder minutes), bot and
// human-fallback actions between them, and a human action on 09-30, outside
// the window. Twelve PRs merged into dev inside the window; one merged into
// main, one into dev on 10-29 (the window is half-open), and one closed
// unmerged do not count. 13 / 12 = 1.1 against the target <= 1: not met. A
// window with no merged PR writes no Result.
func TestMeasureFounderMinutesPerPR(t *testing.T) {
	root := t.TempDir()
	bare := filepath.Join(root, "origin.git")
	git(t, root, "init", "--quiet", "--bare", bare)
	seed := filepath.Join(root, "seed")
	git(t, root, "clone", "--quiet", bare, seed)
	const date = "2026-10-29T00:00:00Z"
	line := func(seq int, at, identity string) string {
		return fmt.Sprintf(`{"seq":%d,"time":%q,"run":"r","kind":"github.issue.labels.add","target":"o/r#1","identity":%q,"status":"done","tokens_in":0,"tokens_out":0,"cost_usd":0,"wall_time_ms":1,"attempts":1,"prev":""}`, seq, at, identity)
	}
	gitAt(t, seed, date, "checkout", "--quiet", "--orphan", "ledger")
	writeFiles(t, seed, map[string]string{
		"README.md":                      "ledger\n",
		"actions/2026-09-30/cli-a.jsonl": line(1, "2026-09-30T23:59:00Z", "human") + "\n",
		"actions/2026-10-12/cli-b.jsonl": strings.Join([]string{line(1, "2026-10-12T09:00:00Z", "human"), line(2, "2026-10-12T09:04:00Z", "human"),
			line(3, "2026-10-12T09:20:00Z", "bot"), line(4, "2026-10-12T10:00:00Z", "human-fallback")}, "\n") + "\n",
		"actions/2026-10-12/amp-c.jsonl": strings.Join([]string{line(1, "2026-10-12T09:09:00Z", "human"), line(2, "2026-10-12T11:30:00Z", "human"),
			line(3, "2026-10-12T14:00:00Z", "bot")}, "\n") + "\n",
	})
	gitAt(t, seed, date, "add", ".")
	gitAt(t, seed, date, "commit", "--quiet", "-m", "actions: segments")
	gitAt(t, seed, date, "push", "--quiet", "origin", "ledger")
	gitAt(t, seed, date, "checkout", "--quiet", "--orphan", "dev")
	gitAt(t, seed, date, "rm", "-rf", "--quiet", ".")
	gitAt(t, seed, date, "commit", "--quiet", "--allow-empty", "-m", "dev")
	gitAt(t, seed, date, "push", "--quiet", "origin", "dev")
	dir := filepath.Join(root, "work")
	git(t, root, "clone", "--quiet", "--branch", "dev", bare, dir)

	g := fake.New(t, "o/r", "User")
	g.AddIssue("Less founder time per useful change", "### Summary\n\nThe founder spends less time per merged PR.\n\n### Metric\n\naayu:founder_minutes_per_pr\n\n"+
		"### Baseline\n\n1.5 founder minutes per merged PR (2026-09)\n\n### Target\n\n≤ 1.0", "type:goal", "status:in-progress")
	pr := func(n int, base, merged string) string {
		m := "null"
		if merged != "" {
			m = fmt.Sprintf("%q", merged)
		}
		return fmt.Sprintf(`{"number":%d,"state":"closed","base":{"ref":%q},"merged_at":%s}`, n, base, m)
	}
	var prs []string
	for n := 10; n < 22; n++ {
		prs = append(prs, pr(n, "dev", fmt.Sprintf("2026-10-%02dT12:00:00Z", n)))
	}
	prs = append(prs, pr(30, "main", "2026-10-12T12:00:00Z"), pr(31, "dev", "2026-10-29T00:00:00Z"), pr(32, "dev", ""))
	g.Static = map[string]any{"/repos/o/r/pulls": jsonOf(t, "["+strings.Join(prs, ",")+"]")}
	env := map[string]string{"AAYU_GITHUB_TOKEN": "t", "XDG_STATE_HOME": filepath.Join(root, "state"), "PATH": os.Getenv("PATH")}
	opts := Options{GitHubAPI: g.URL(), GitHubGap: -1, Dir: dir, Now: func() time.Time { return time.Date(2026, 10, 29, 9, 0, 0, 0, time.UTC) }}

	code, out, errOut := run(t, env, opts, "measure", "1", "--repo", "o/r", "--json", "--metric", "aayu:founder_minutes_per_pr", "--from", "2026-10-01", "--to", "2026-10-29")
	if code != 0 {
		t.Fatalf("measure: exit %d: %s", code, errOut)
	}
	var got measureOut
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if m := got.Measured; m == nil || m.Value != 1.1 || m.Met || m.Target != (measure.Target{Op: "<=", Value: 1}) {
		t.Fatalf("measured: %+v", got.Measured)
	}
	g.Mu.Lock()
	body := g.Issues[1].Body
	g.Mu.Unlock()
	for _, want := range []string{
		"Target not met: aayu:founder_minutes_per_pr = 1.1 against the target <= 1 (baseline 1.5",
		"13 founder minutes in 2 sessions of 4 human-identity actions in the actions log",
		"over 12 pull requests merged into dev",
		"[outcomes] session_gap_minutes = 10", "[outcomes] action_minutes = 2",
		"Window: 2026-10-01T00:00:00Z to 2026-10-29T00:00:00Z",
		"3 actions segments",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("Result lacks %q:\n%s", want, body)
		}
	}
	if !slices.Contains(g.LabelNames(1), "status:measured") {
		t.Errorf("labels %v, want status:measured", g.LabelNames(1))
	}

	writes := len(g.Writes())
	code, _, errOut = run(t, env, opts, "measure", "1", "--repo", "o/r", "--json", "--from", "2026-11-01", "--to", "2026-11-08")
	if code != ExitBlocked || !strings.Contains(errOut, "no merged pull request in the window; founder minutes per merged PR is undefined") {
		t.Errorf("empty window: exit %d: %s", code, errOut)
	}
	g.Mu.Lock()
	after := g.Issues[1].Body
	g.Mu.Unlock()
	if after != body || len(g.Writes()) != writes {
		t.Errorf("empty window wrote to GitHub: %v", g.Writes()[writes:])
	}

	// A segment line that does not decode would drop a person's minutes from
	// the count, so measure stops and names it instead of measuring less.
	gitAt(t, seed, date, "checkout", "--quiet", "ledger")
	writeFiles(t, seed, map[string]string{"actions/2026-10-13/cli-d.jsonl": line(1, "2026-10-13T09:00:00Z", "human") + "\n{\"seq\":2,\"time\":\n"})
	gitAt(t, seed, date, "add", ".")
	gitAt(t, seed, date, "commit", "--quiet", "-m", "actions: a truncated segment")
	gitAt(t, seed, date, "push", "--quiet", "origin", "ledger")
	code, _, errOut = run(t, env, opts, "measure", "1", "--repo", "o/r", "--json", "--from", "2026-10-01", "--to", "2026-10-29")
	if code == 0 || !strings.Contains(errOut, "actions/2026-10-13/cli-d.jsonl line 2") || len(g.Writes()) != writes {
		t.Errorf("truncated segment: exit %d: %s", code, errOut)
	}
}
