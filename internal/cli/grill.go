package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/design"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations/providers"
	"github.com/rahulnakmol/agentic-ayudha/internal/measure"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

func init() { register("grill", runGrill) }

// GrillResult is what grill reports.
type GrillResult struct {
	Type     string                 `json:"type"`
	Title    string                 `json:"title"`
	Parent   int                    `json:"parent,omitempty"`
	Labels   []string               `json:"labels"`
	Create   []workitems.Problem    `json:"create_problems"`
	Ready    []workitems.Problem    `json:"ready_problems"`
	Pass     bool                   `json:"passes_ready"`
	Baseline *integrations.Baseline `json:"baseline,omitempty"` // fetched for a Goal or Initiative (WP5.2 adapters)
	Issue    int                    `json:"issue,omitempty"`
	URL      string                 `json:"url,omitempty"`
	Identity string                 `json:"identity,omitempty"`
	DryRun   bool                   `json:"dry_run,omitempty"`
	Pairs    []design.PairResult    `json:"pairs,omitempty"` // grill brand: the AA palette check
	Written  string                 `json:"written,omitempty"`
}

// grillDraft is what the interview produced: flags, or the TUI's answers.
type grillDraft struct {
	Title, Body, Labels, Parent string
}

// BaselineDays is the window of the baseline grill fetches for a Goal or
// Initiative: the whole UTC days before today, as measure's default.
const BaselineDays = 28

// runGrill (WP5.3): the interview is the skill's (skills/grill and
// references/<type>.md) when a host's model asks the questions, or the
// bubbletea TUI on a human's terminal; either way the binary validates the
// draft at Create and Ready, fetches the baseline of a Goal or Initiative
// through the WP5.2 adapters, and with --create writes the issue through the
// actions log, under the human's identity when a human runs it and the
// bot's otherwise. `grill brand` writes design/brand.yaml after an AA
// palette check.
func runGrill(ctx context.Context, e *Env, in Values) error {
	typ := in.String("type")
	if typ == "brand" {
		return runGrillBrand(e, in)
	}
	t, ok := workitems.FindType(typ)
	if !ok {
		return Errorf(ExitUsage, "grill: unknown type %q", typ)
	}
	repo, err := e.RepoName(ctx)
	if err != nil {
		return err
	}
	c, tag, err := e.grillClient(ctx)
	if err != nil {
		return err
	}
	d := grillDraft{Title: strings.TrimSpace(in.String("title")), Body: in.String("body"), Labels: in.String("labels"), Parent: in.String("parent")}
	create := in.Bool("create")
	if d.Title == "" && strings.TrimSpace(d.Body) == "" && e.interactive() {
		m := &grillModel{name: "grill " + typ, steps: workItemSteps(t, e.Assets)}
		for i := range m.steps {
			m.steps[i].Value = map[string]string{"parent": d.Parent, "labels": d.Labels}[m.steps[i].ID]
		}
		m.check = func(v map[string]string) (string, []string) {
			dd := draftFromAnswers(t, v)
			res, err := e.grillCheck(ctx, c, repo, t, &dd)
			if err != nil {
				return dd.Body, []string{"labels: " + err.Error()}
			}
			var ps []string
			for _, p := range append(res.Create, res.Ready...) {
				ps = append(ps, p.String())
			}
			return "# " + dd.Title + "\n\n" + dd.Body, ps
		}
		vals, ok, err := e.runGrillTUI(m)
		if err != nil {
			return err
		}
		if !ok {
			return Errorf(ExitBlocked, "grill %s: the interview ended without writing anything", typ)
		}
		d, create = draftFromAnswers(t, vals), true
	}
	if d.Title == "" || strings.TrimSpace(d.Body) == "" {
		return Errorf(ExitUsage, "grill %s needs --title and --body (the draft the interview produced, skills/grill/references/%s.md), or a terminal for the interview", typ, typ)
	}
	res, err := e.grillCheck(ctx, c, repo, t, &d)
	if err != nil {
		return Wrap(ExitUsage, err)
	}

	if create && res.Pass {
		if tag != "" {
			res.Labels = append(res.Labels, tag)
		}
		res.Identity, res.DryRun = c.Identity(), c.DryRun()
		owner, name, _ := github.SplitRepo(repo)
		var created *gh.Issue
		_, err := c.Write(ctx, actions.Action{Kind: "github.issue.create", Target: repo, Seat: "product-strategist",
			Request: &actions.Request{Method: http.MethodPost, Path: "/repos/" + repo + "/issues"}, Payload: map[string]any{"title": d.Title, "type": typ, "labels": res.Labels, "tag": tag}},
			func(ctx context.Context) (any, error) {
				var err error
				created, _, err = c.REST.Issues.Create(ctx, owner, name, gh.CreateIssueRequest{Title: d.Title, Body: &d.Body, Labels: res.Labels})
				return map[string]any{"number": created.GetNumber()}, err
			})
		if err != nil {
			return Wrap(ExitExternal, err)
		}
		if created != nil {
			res.Issue, res.URL = created.GetNumber(), created.GetHTMLURL()
			if res.Parent > 0 {
				id := created.GetID()
				_, err := c.Write(ctx, actions.Action{Kind: "github.issue.sub-issue.add", Target: fmt.Sprintf("%s#%d", repo, res.Parent),
					Request: &actions.Request{Method: http.MethodPost, Path: fmt.Sprintf("/repos/%s/issues/%d/sub_issues", repo, res.Parent)}, Payload: map[string]any{"sub_issue": res.Issue}},
					func(ctx context.Context) (any, error) {
						_, _, err := c.REST.SubIssue.Add(ctx, owner, name, int64(res.Parent), gh.SubIssueRequest{SubIssueID: id})
						return nil, err
					})
				if err != nil {
					return Wrap(ExitExternal, err)
				}
			}
		}
	}
	if err := e.Emit(res, func(w io.Writer) error {
		verdict := "passes Create and Ready"
		if !res.Pass {
			verdict = "does not pass yet"
		}
		fmt.Fprintf(w, "%s draft %q: %s\n", typ, d.Title, verdict)
		if res.Baseline != nil {
			fmt.Fprintf(w, "  baseline: %s = %s over %s (%s)\n", res.Baseline.Metric, strconv.FormatFloat(res.Baseline.Value, 'f', -1, 64), res.Baseline.Population, windowText(res.Baseline.Window))
		}
		for _, p := range res.Create {
			fmt.Fprintf(w, "  create: %s\n", p)
		}
		for _, p := range res.Ready {
			fmt.Fprintf(w, "  ready: %s\n", p)
		}
		switch {
		case res.DryRun:
			fmt.Fprintln(w, "  --dry-run: the issue is planned, not written")
		case res.Issue > 0:
			fmt.Fprintf(w, "  created #%d %s (labels %s, as %s)", res.Issue, res.URL, strings.Join(res.Labels, ", "), res.Identity)
			if !t.Intake {
				fmt.Fprintf(w, "; next: `aayu ready %d`", res.Issue)
			}
			fmt.Fprintln(w)
		case res.Pass && !create:
			fmt.Fprintln(w, "  run again with --create to write it as an issue")
		}
		return nil
	}); err != nil {
		return err
	}
	if !res.Pass {
		return Errorf(ExitBlocked, "the %s draft has %d problem(s); keep interviewing", typ, len(res.Create)+len(res.Ready))
	}
	return nil
}

// grillClient is the GitHub client grill writes with: the human's own token
// when a human runs it, the bot (or the tagged human fallback, WP1.4) when an
// agent lane does.
func (e *Env) grillClient(ctx context.Context) (*github.Client, string, error) {
	if e.Agent() {
		c, bot, err := e.BotGitHub(ctx, "")
		return c, bot.Tag, err
	}
	c, _, err := e.HumanGitHub(ctx)
	return c, "", err
}

// grillCheck validates a draft at Create and Ready. For a Goal or Initiative
// it fetches the baseline of the Metric and writes it into the draft's
// Baseline section (d.Body changes). It errs only on an unusable label.
func (e *Env) grillCheck(ctx context.Context, c *github.Client, repo string, t workitems.Type, d *grillDraft) (GrillResult, error) {
	status := "status:planned"
	if t.Intake {
		status = "status:backlog" // intake waits for triage, as the issue forms file it
	}
	labels := []string{"type:" + t.ID, status}
	for _, l := range strings.Split(d.Labels, ",") {
		if l = strings.TrimSpace(l); l == "" {
			continue
		}
		prefix, value, _ := strings.Cut(l, ":")
		known := false
		for _, f := range workitems.LabelFields {
			known = known || (f.Prefix == prefix && slices.Contains(f.Values, value))
		}
		if !known || prefix == "status" {
			return GrillResult{}, fmt.Errorf("grill: label %q is not a label-held field (priority, effort, risk, health, horizon)", l)
		}
		labels = append(labels, l)
	}
	res := GrillResult{Type: t.ID, Title: d.Title, Labels: labels}
	it, ps := workitems.Parse(0, d.Title, d.Body, "", labels)
	res.Create = append(res.Create, ps...)
	res.Create = append(res.Create, workitems.Validate(it, workitems.StageCreate)...)
	if p := strings.TrimSpace(d.Parent); p != "" {
		res.Parent = issueNumber(p)
		owner, name, _ := github.SplitRepo(repo)
		parent, _, err := c.REST.Issues.Get(ctx, owner, name, res.Parent)
		if err != nil {
			res.Ready = append(res.Ready, workitems.Problem{Field: "parent", Message: fmt.Sprintf("#%d: %v", res.Parent, err)})
		} else {
			pt, _ := workitems.TypeOf(parent.GetType().GetName(), github.IssueLabels(parent))
			it.Parent = &workitems.Ref{Number: res.Parent, Type: pt}
		}
	}
	if _, ok := t.Field(workitems.FieldOwner); ok && workitems.Empty(it.Fields[workitems.FieldOwner]) {
		it.Fields[workitems.FieldOwner] = e.technicalOwner()
	}
	if t.ID == "goal" || t.ID == "initiative" {
		var bps []workitems.Problem
		res.Baseline, bps = e.grillBaseline(ctx, repo, &it, d)
		res.Ready = append(res.Ready, bps...)
	}
	res.Ready = append(res.Ready, workitems.Validate(it, workitems.StageReady)...)
	res.Pass = len(res.Create) == 0 && len(res.Ready) == 0
	if res.Create == nil {
		res.Create = []workitems.Problem{}
	}
	if res.Ready == nil {
		res.Ready = []workitems.Problem{}
	}
	return res, nil
}

// grillBaseline is the baseline gate of a Goal or Initiative (WP5.3): a
// Target needs a baseline first, and the baseline must be reproducible. A
// provider metric (`posthog:users event = 'signed_up'`) is fetched through
// its WP5.2 adapter over the BaselineDays before today and written into the
// Baseline section with its population, window, source query, and fetch
// time; when no adapter can answer, the draft is refused Ready naming the
// metric. A snapshot metric (activation, retention, …) comes from `aayu
// measure` exports, so its Baseline must already hold the measured value.
func (e *Env) grillBaseline(ctx context.Context, repo string, it *workitems.Item, d *grillDraft) (*integrations.Baseline, []workitems.Problem) {
	metric := it.Fields["metric"]
	if workitems.Empty(metric) {
		return nil, nil // Validate reports the missing Metric
	}
	name, _, _ := strings.Cut(strings.TrimSpace(metric), "\n")
	name = strings.Trim(strings.TrimSpace(name), "`")
	spec, err := measure.ParseSpec(metric)
	if err != nil {
		return nil, []workitems.Problem{{Field: "metric", Message: err.Error()}}
	}
	refuse := func(why string) []workitems.Problem {
		return []workitems.Problem{{Field: workitems.FieldBaseline, Message: fmt.Sprintf("no reachable baseline for metric %q (a Target needs a reproducible baseline first): %s", name, why)}}
	}
	if spec.Source == "" {
		if _, err := measure.ParseBaseline(it.Fields[workitems.FieldBaseline]); err != nil {
			return nil, refuse(spec.Metric + " is measured from exports by `aayu measure`; put the measured value and its snapshot in Baseline")
		}
		return nil, nil
	}
	to := e.Now().UTC().Truncate(24 * time.Hour)
	b, err := providers.Baseline(ctx, e.integrationDeps(repo), spec.Source, integrations.Query{Metric: spec.Metric, Filter: spec.Filter,
		Window: integrations.Window{From: to.AddDate(0, 0, -BaselineDays), To: to}})
	if err != nil {
		return nil, refuse(err.Error())
	}
	txt := baselineText(b)
	it.Fields[workitems.FieldBaseline] = txt
	d.Body = workitems.SetSection(d.Body, "Baseline", txt)
	return &b, nil
}

// baselineText renders a baseline tuple for the Baseline section: the value
// first (measure reads the first number), then everything needed to re-run it.
func baselineText(b integrations.Baseline) string {
	return fmt.Sprintf("%s\n\n- Metric: %s\n- Population: %s\n- Window: %s\n- Fetched at: %s\n- Source query:\n\n```text\n%s\n```",
		strconv.FormatFloat(b.Value, 'f', -1, 64), b.Metric, b.Population, windowText(b.Window), b.FetchedAt.UTC().Format(time.RFC3339), strings.TrimSpace(b.SourceQuery))
}

func windowText(w integrations.Window) string {
	return fmt.Sprintf("%s to %s (UTC, half-open)", w.From.UTC().Format(time.RFC3339), w.To.UTC().Format(time.RFC3339))
}

// draftFromAnswers builds the issue body from the TUI's answers: each
// answered field as its "### <Label>" section, in schema order.
func draftFromAnswers(t workitems.Type, v map[string]string) grillDraft {
	var b strings.Builder
	for _, f := range t.Fields() {
		if val := strings.TrimSpace(v[f.ID]); val != "" {
			fmt.Fprintf(&b, "### %s\n\n%s\n\n", f.Label, val)
		}
	}
	return grillDraft{Title: v["title"], Body: strings.TrimSpace(b.String()), Labels: v["labels"], Parent: v["parent"]}
}

var (
	paletteLine = regexp.MustCompile(`^\s*[-*]?\s*([A-Za-z0-9_-]+)\s*[:=]?\s*(#[0-9A-Fa-f]{3,6})\s*$`)
	pairLine    = regexp.MustCompile(`^\s*[-*]?\s*([A-Za-z0-9_-]+)\s+on\s+([A-Za-z0-9_-]+)\s*[,:]?\s*(text|large-text|ui)\s*$`)
)

// brandFromAnswers builds a brand from the TUI's answers.
func brandFromAnswers(v map[string]string) (*design.Brand, []string) {
	words := func(s string) []string {
		var out []string
		for _, w := range strings.Split(s, ",") {
			if w = strings.TrimSpace(w); w != "" {
				out = append(out, w)
			}
		}
		return out
	}
	br := &design.Brand{Version: 1, Name: v["name"], Voice: design.Voice{Tone: words(v["tone"]), Avoid: words(v["avoid"])}, Palette: map[string]string{}}
	var ps []string
	for _, l := range strings.Split(v["palette"], "\n") {
		if strings.TrimSpace(l) == "" {
			continue
		}
		m := paletteLine.FindStringSubmatch(l)
		if m == nil {
			ps = append(ps, fmt.Sprintf("palette: %q: want `name #rrggbb`", strings.TrimSpace(l)))
			continue
		}
		br.Palette[m[1]] = strings.ToLower(m[2])
	}
	for _, l := range strings.Split(v["pairs"], "\n") {
		if strings.TrimSpace(l) == "" {
			continue
		}
		m := pairLine.FindStringSubmatch(l)
		if m == nil {
			ps = append(ps, fmt.Sprintf("pairs: %q: want `fg on bg, text|large-text|ui`", strings.TrimSpace(l)))
			continue
		}
		br.Pairs = append(br.Pairs, design.Pair{FG: m[1], BG: m[2], Use: m[3]})
	}
	return br, ps
}

// brandCheck is the TUI's review of a brand: the answers as YAML, and every
// line that did not parse plus the AA check's problems.
func brandCheck(v map[string]string) (string, []string) {
	br, ps := brandFromAnswers(v)
	y, _ := br.Marshal()
	_, probs := br.Check()
	return string(y), append(ps, probs...)
}

// runGrillBrand is `grill brand`: the brand YAML from --body (the host's
// draft) or the TUI, parsed strictly, every colour pair checked against
// WCAG 2.2 AA, and with --create written to design/brand.yaml.
func runGrillBrand(e *Env, in Values) error {
	src, create := in.String("body"), in.Bool("create")
	if strings.TrimSpace(src) == "" && e.interactive() {
		m := &grillModel{name: "grill brand", steps: brandSteps(e.Assets), check: brandCheck}
		vals, ok, err := e.runGrillTUI(m)
		if err != nil {
			return err
		}
		if !ok {
			return Errorf(ExitBlocked, "grill brand: the interview ended without writing anything")
		}
		br, _ := brandFromAnswers(vals)
		y, _ := br.Marshal()
		src, create = string(y), true
	}
	if strings.TrimSpace(src) == "" {
		return Errorf(ExitUsage, "grill brand needs --body with the brand YAML (skills/grill/references/brand.md), or a terminal for the interview")
	}
	br, err := design.ParseBrand([]byte(src))
	if err != nil {
		return Wrap(ExitUsage, err)
	}
	res := GrillResult{Type: "brand", Title: br.Name, Labels: []string{}, Create: []workitems.Problem{}, Ready: []workitems.Problem{}}
	var problems []string
	res.Pairs, problems = br.Check()
	for _, p := range problems {
		field, msg, ok := strings.Cut(p, ": ")
		if !ok {
			field, msg = "brand", p
		}
		res.Ready = append(res.Ready, workitems.Problem{Field: field, Message: msg})
	}
	res.Pass = len(res.Ready) == 0
	if create && res.Pass {
		root := config.RepoRoot(e.Dir)
		if root == "" {
			return Errorf(ExitUsage, "grill brand --create runs in a checkout of the repository")
		}
		res.Written, res.DryRun = filepath.ToSlash(design.BrandPath), e.DryRun
		if !e.DryRun {
			y, err := br.Marshal()
			if err != nil {
				return err
			}
			p := filepath.Join(root, filepath.FromSlash(design.BrandPath))
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(p, y, 0o644); err != nil {
				return err
			}
		}
	}
	if err := e.Emit(res, func(w io.Writer) error {
		verdict := "passes the AA palette check"
		if !res.Pass {
			verdict = "does not pass yet"
		}
		fmt.Fprintf(w, "brand %q: %s\n", br.Name, verdict)
		for _, r := range res.Pairs {
			fmt.Fprintf(w, "  %s\n", r)
		}
		for _, p := range res.Ready {
			if !strings.HasPrefix(p.Field, "AA") {
				fmt.Fprintf(w, "  problem: %s\n", p)
			}
		}
		switch {
		case res.Written != "" && res.DryRun:
			fmt.Fprintf(w, "  --dry-run: %s is planned, not written\n", res.Written)
		case res.Written != "":
			fmt.Fprintf(w, "  wrote %s; commit it in a PR (tell reads its voice)\n", res.Written)
		case res.Pass && !create:
			fmt.Fprintln(w, "  run again with --create to write design/brand.yaml")
		}
		return nil
	}); err != nil {
		return err
	}
	if !res.Pass {
		return Errorf(ExitBlocked, "the brand has %d problem(s); keep interviewing", len(res.Ready))
	}
	return nil
}
