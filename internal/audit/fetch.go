package audit

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/autonomy"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/inspect"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
	"github.com/rahulnakmol/agentic-ayudha/internal/ready"
)

// ErrNoAdmin means the token cannot read rule suites, which needs repository
// admin; without them the bypasses cannot be reconciled, so audit refuses.
var ErrNoAdmin = errors.New("rule suites need a repository admin token (GET rulesets/rule-suites answered 403)")

// MergeGateWorkflow is the merge gate's workflow file (WP4.8): a story merged
// before GitHub registered it merged without a gate (pre-gate).
const MergeGateWorkflow = "aayudha-merge-gate.yml"

// Source is what Fetch reads from.
type Source struct {
	GH       *github.Client
	Repo     string
	Tree     *ledger.Tree // the ledger at Inputs.Ledger
	Store    *proof.Store
	Now      time.Time // chooses the rule-suite listing period
	Rules    autonomy.Rules
	Never    []string
	Suites   bool // read rule suites (needs admin), workflow runs, refs, and the licence inputs
	Autonomy bool // read merged-PR history for corrections and streaks
	Flow     bool // read the merge gate's runs for retro's flow metrics
	Streak   int  // [autonomy] streak
}

// Fetch reads every source up to in.Until into d and records the segment
// list and code commit in in.
func Fetch(ctx context.Context, s Source, in *Inputs, d *Data) error {
	if err := FetchLedger(ctx, s, in, d); err != nil {
		return err
	}
	owner, name, err := github.SplitRepo(s.Repo)
	if err != nil {
		return err
	}
	if s.Suites {
		if err := d.fetchSuites(ctx, s, in, owner, name); err != nil {
			return err
		}
	}
	if gate, err := GateSince(ctx, s.GH, s.Repo); err != nil {
		return err
	} else if gate.Before(in.Until) {
		d.GateSince = gate
	}
	if err := d.fetchPRs(ctx, s, in, owner, name); err != nil {
		return err
	}
	if err := d.fetchItems(ctx, s, in, owner, name); err != nil {
		return err
	}
	if s.Suites {
		if err := d.fetchRuns(ctx, s, in, owner, name); err != nil {
			return err
		}
		if err := d.fetchRefs(ctx, s, owner, name); err != nil {
			return err
		}
		if err := d.fetchLicence(ctx, s, in, owner, name); err != nil {
			return err
		}
	}
	if s.Flow {
		if err := d.fetchGateRuns(ctx, s, in, owner, name); err != nil {
			return err
		}
	}
	if !s.Autonomy {
		return nil
	}
	return d.fetchAutonomy(ctx, s, in)
}

// FetchLedger reads the ledger part of d at s.Tree (all `--replay` reads) and
// records the segment list in in.
func FetchLedger(ctx context.Context, s Source, in *Inputs, d *Data) error {
	if err := d.fetchLedger(ctx, s, in); err != nil {
		return fmt.Errorf("ledger: %w", err)
	}
	return nil
}

func (d *Data) fetchLedger(ctx context.Context, s Source, in *Inputs) error {
	var err error
	if d.LedgerLog, err = s.Tree.Log(ctx); err != nil {
		return err
	}
	segs, err := s.Tree.List(ctx, "actions")
	if err != nil {
		return err
	}
	in.Segments = []Segment{}
	for _, p := range segs {
		if !strings.HasSuffix(p, ".jsonl") {
			continue
		}
		b, err := s.Tree.File(ctx, p)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(b)
		seg := Segment{Path: p, SHA256: hex.EncodeToString(sum[:])}
		sc := bufio.NewScanner(bytes.NewReader(b))
		sc.Buffer(make([]byte, 1<<20), 16<<20)
		for sc.Scan() {
			var r actions.Record
			if json.Unmarshal(sc.Bytes(), &r) == nil {
				d.Records = append(d.Records, Record{Record: r, Segment: p})
				seg.Records++
			}
		}
		in.Segments = append(in.Segments, seg)
	}
	entries, err := s.Store.Packs(ctx, s.Tree, 0)
	if err != nil {
		return err
	}
	for _, e := range entries {
		p := Pack{Path: e.Path, Story: e.Pack.Story, Head: e.Pack.PRHeadSHA, Kind: e.Pack.Authority.Kind, CreatedAt: e.Pack.CreatedAt, Checks: e.Pack.Checks,
			Checkpoint: e.Pack.Checkpoint, CostUSD: e.Pack.Cost.USD}
		if p.Kind == proof.Authoritative {
			p.Covers, p.Problem = covers(ctx, s, e)
		}
		d.Packs = append(d.Packs, p)
	}
	slices.SortStableFunc(d.Packs, func(a, b Pack) int { return strings.Compare(a.Path, b.Path) })
	files, err := s.Tree.List(ctx, "proof")
	if err != nil {
		return err
	}
	for _, f := range files {
		if path.Base(f) != "ship.json" {
			continue
		}
		b, err := s.Tree.File(ctx, f)
		if err != nil {
			return err
		}
		var rec struct {
			Story, PR   int
			MergeCommit string `json:"merge_commit"`
			HumanPass   struct {
				Kind, Login string
				CommentURL  string `json:"comment_url"`
			} `json:"human_pass"`
			ShippedAt  time.Time `json:"shipped_at"`
			PlanReview *struct {
				Layers []string
				Bypass *struct {
					RecordedBy string `json:"recorded_by"`
					CommentURL string `json:"comment_url"`
				}
			} `json:"plan_review"`
		}
		if json.Unmarshal(b, &rec) != nil {
			continue
		}
		sh := Ship{Path: f, Story: rec.Story, PR: rec.PR, Merge: rec.MergeCommit, Pass: rec.HumanPass.Kind, Login: rec.HumanPass.Login,
			Comment: rec.HumanPass.CommentURL, ShippedAt: rec.ShippedAt.UTC()}
		if pr := rec.PlanReview; pr != nil && pr.Bypass != nil {
			sh.PlanBypass = &PlanBypass{Layers: pr.Layers, By: pr.Bypass.RecordedBy, Comment: pr.Bypass.CommentURL}
		}
		d.Ships = append(d.Ships, sh)
	}
	if err := d.fetchFlowLedger(ctx, s); err != nil {
		return err
	}
	reports, err := s.Tree.List(ctx, "audit")
	if err != nil {
		return err
	}
	for _, f := range reports {
		if !strings.HasSuffix(f, ".md") {
			continue
		}
		b, err := s.Tree.File(ctx, f)
		if err != nil {
			return err
		}
		if strings.HasPrefix(path.Base(f), "incident-") {
			if ids := SuiteIDs(string(b)); len(ids) > 0 {
				d.Incidents = append(d.Incidents, PriorReport{Path: f, Suites: ids})
			}
			continue
		}
		if ids := PriorSuites(string(b)); len(ids) > 0 {
			d.Prior = append(d.Prior, PriorReport{Path: f, Suites: ids})
		}
	}
	return nil
}

// covers reports whether an authoritative pack verifies against its story's
// Ready snapshot and its floors hold, or why not.
func covers(ctx context.Context, s Source, e proof.Entry) (bool, string) {
	b, err := s.Tree.File(ctx, ready.LedgerPath(e.Pack.Story))
	if err != nil {
		return false, "no Ready snapshot on the ledger"
	}
	snap, err := ready.ParseSnapshot(string(b))
	if err != nil {
		return false, "Ready snapshot unreadable: " + err.Error()
	}
	rep := proof.Verify(e.Pack, proof.Expect{CompletionGoalHash: snap.CompletionGoalHash, Criteria: snap.Numbers()}, s.Store.Opener(ctx, s.Tree, e))
	switch {
	case !rep.OK:
		return false, strings.Join(rep.Problems, "; ")
	case !rep.FloorsHold:
		return false, "floors do not hold"
	}
	return true, ""
}

var suiteID = regexp.MustCompile(`\b[0-9]{7,}\b`)

// SuiteIDs returns every number in text long enough to be a rule-suite ID;
// the audit matches them against the suites it classifies.
func SuiteIDs(text string) []int64 {
	var out []int64
	for _, m := range suiteID.FindAllString(text, -1) {
		if id, err := strconv.ParseInt(m, 10, 64); err == nil && !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out
}

var suiteRow = regexp.MustCompile(`^\|\s*([0-9]{6,})\s*\|`)

// PriorSuites returns the rule suites an earlier report accounts for: every
// suite in a table row of a hand-run reconciliation, and the suites an
// `aayu audit` report marks accounted for.
func PriorSuites(report string) []int64 {
	generated := strings.Contains(report, "Written by `aayu audit`")
	var out []int64
	for _, line := range strings.Split(report, "\n") {
		m := suiteRow.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if generated {
			cells := strings.Split(line, "|")
			if len(cells) < 9 || strings.TrimSpace(cells[7]) != "yes" {
				continue
			}
		}
		if id, err := strconv.ParseInt(m[1], 10, 64); err == nil {
			out = append(out, id)
		}
	}
	return out
}

func (d *Data) fetchSuites(ctx context.Context, s Source, in *Inputs, owner, name string) error {
	period := "month"
	switch age := s.Now.Sub(in.Since); {
	case age < 23*time.Hour:
		period = "day"
	case age < 167*time.Hour:
		period = "week"
	}
	type suite struct {
		ID       int64     `json:"id"`
		Ref      string    `json:"ref"`
		Actor    *string   `json:"actor_name"`
		Before   string    `json:"before_sha"`
		After    string    `json:"after_sha"`
		PushedAt time.Time `json:"pushed_at"`
		Evals    []struct {
			Source struct {
				Name string `json:"name"`
			} `json:"rule_source"`
			Result  string `json:"result"`
			Type    string `json:"rule_type"`
			Details string `json:"details"`
		} `json:"rule_evaluations"`
	}
	get := func(u string, v any) (*gh.Response, error) {
		req, err := s.GH.REST.NewRequest(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		resp, err := s.GH.REST.Do(req, v)
		if resp != nil && resp.StatusCode == http.StatusForbidden {
			return resp, ErrNoAdmin
		}
		return resp, err
	}
	var list []suite
	for page := 1; ; {
		var batch []suite
		resp, err := get(fmt.Sprintf("repos/%s/%s/rulesets/rule-suites?rule_suite_result=bypass&time_period=%s&per_page=100&page=%d", owner, name, period, page), &batch)
		if err != nil {
			return err
		}
		list = append(list, batch...)
		if resp == nil || resp.NextPage == 0 {
			break
		}
		page = resp.NextPage
	}
	for _, x := range list {
		if x.PushedAt.Before(in.Since) || !x.PushedAt.Before(in.Until) {
			continue
		}
		var full suite
		if _, err := get(fmt.Sprintf("repos/%s/%s/rulesets/rule-suites/%d", owner, name, x.ID), &full); err != nil {
			return err
		}
		rs := RuleSuite{ID: x.ID, Ref: x.Ref, Before: x.Before, After: x.After, PushedAt: x.PushedAt.UTC()}
		if full.Actor != nil {
			rs.Actor = *full.Actor
		} else if x.Actor != nil {
			rs.Actor = *x.Actor
		}
		for _, e := range full.Evals {
			if e.Result == "fail" {
				rs.Failed = append(rs.Failed, Rule{Ruleset: e.Source.Name, Type: e.Type, Details: e.Details})
			}
		}
		d.RuleSuites = append(d.RuleSuites, rs)
	}
	return nil
}

func (d *Data) fetchPRs(ctx context.Context, s Source, in *Inputs, owner, name string) error {
	prs, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.PullRequest, *gh.Response, error) {
		return s.GH.REST.PullRequests.List(ctx, owner, name, &gh.PullRequestListOptions{State: "closed", Sort: "created", Direction: "asc", ListOptions: o})
	})
	if err != nil {
		return err
	}
	for _, p := range prs {
		if p.MergedAt == nil || !p.GetMergedAt().Before(in.Until) {
			continue
		}
		pr := PR{Number: p.GetNumber(), Title: p.GetTitle(), Base: p.GetBase().GetRef(), Head: p.GetHead().GetRef(),
			Author: p.GetUser().GetLogin(), MergedAt: p.GetMergedAt().UTC(), MergeSHA: p.GetMergeCommitSHA(), Body: p.GetBody()}
		if s.Flow {
			pr.HeadSHA = p.GetHead().GetSHA()
		}
		pr.LinkStories()
		d.PRs = append(d.PRs, pr)
	}
	slices.SortStableFunc(d.PRs, func(a, b PR) int { return a.Number - b.Number })
	return nil
}

// fetchItems reads every issue and PR created before Until; the timeline of
// those changed since Since and of every story, bug, and Decision, from which
// their labels at Until are replayed. An item unchanged since Since has today's
// labels at Until. Without suites or autonomy (--mark-merged, at now) no
// timeline is read: today's labels are the labels at Until.
func (d *Data) fetchItems(ctx context.Context, s Source, in *Inputs, owner, name string) error {
	list, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.Issue, *gh.Response, error) {
		return s.GH.REST.Issues.ListByRepo(ctx, owner, name, &gh.IssueListByRepoOptions{State: "all", Sort: "created", Direction: "asc", ListOptions: o})
	})
	if err != nil {
		return err
	}
	for _, is := range list {
		if !is.GetCreatedAt().Before(in.Until) {
			continue
		}
		it := Item{Number: is.GetNumber(), Title: is.GetTitle(), PR: is.IsPullRequest(), Author: is.GetUser().GetLogin(), CreatedAt: is.GetCreatedAt().UTC(),
			Labels: github.IssueLabels(is)}
		if c := is.GetClosedAt(); !c.IsZero() && c.Before(in.Until) {
			it.ClosedAt = c.UTC()
		}
		typed := slices.ContainsFunc(it.Labels, func(l string) bool { return l == "type:story" || l == "type:bug" || l == "type:decision" })
		if (is.GetUpdatedAt().Before(in.Since) && !typed) || (!s.Suites && !s.Autonomy) {
			d.Items = append(d.Items, it)
			continue
		}
		tl, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.Timeline, *gh.Response, error) {
			return s.GH.REST.Issues.ListIssueTimeline(ctx, owner, name, it.Number, &o)
		})
		if err != nil {
			return err
		}
		it.Events, it.Labels, it.ClosedAt = []Event{}, []string{}, time.Time{}
		for _, t := range tl {
			e := Event{Kind: t.GetEvent(), Actor: t.GetActor().GetLogin(), At: t.GetCreatedAt().UTC(), Label: t.GetLabel().GetName(), Body: t.GetBody(), ID: t.GetID(), State: t.GetState()}
			if e.Kind == "commented" && strings.HasPrefix(strings.TrimSpace(e.Body), "Solo-mode bypass") {
				e.Suites = SuiteIDs(e.Body)
			}
			if e.Kind == "commented" && s.Flow {
				e.Aayu = strings.Contains(e.Body, "<!-- aayudha:")
				if strings.Contains(e.Body, inspect.Marker) && e.At.Before(in.Until) {
					it.Inspect = reviewOf(inspect.ParseState(e.Body))
				}
			}
			if len(e.Body) > MaxBody {
				e.Body = strings.ToValidUTF8(e.Body[:MaxBody], "")
			}
			if e.Kind == "commented" || e.Kind == "reviewed" {
				e.Actor = t.GetUser().GetLogin()
			}
			if e.Kind == "reviewed" {
				e.At = t.GetSubmittedAt().UTC()
			}
			if src := t.GetSource().GetIssue(); src != nil {
				e.Source, e.SourcePR = src.GetNumber(), src.IsPullRequest()
			}
			if t.CreatedAt == nil && t.SubmittedAt == nil || !e.At.Before(in.Until) {
				continue
			}
			switch e.Kind {
			case "labeled":
				if !slices.Contains(it.Labels, e.Label) {
					it.Labels = append(it.Labels, e.Label)
				}
			case "unlabeled":
				it.Labels = slices.DeleteFunc(it.Labels, func(l string) bool { return l == e.Label })
			case "closed":
				it.ClosedAt = e.At
			case "reopened":
				it.ClosedAt = time.Time{}
			}
			it.Events = append(it.Events, e)
		}
		slices.Sort(it.Labels)
		d.Items = append(d.Items, it)
	}
	return nil
}

func (d *Data) fetchRuns(ctx context.Context, s Source, in *Inputs, owner, name string) error {
	add := func(rs []*gh.WorkflowRun) {
		for _, r := range rs {
			if !r.GetCreatedAt().Before(in.Until) || slices.ContainsFunc(d.Runs, func(x Run) bool { return x.ID == r.GetID() }) {
				continue
			}
			d.Runs = append(d.Runs, Run{ID: r.GetID(), Workflow: r.GetPath(), Event: r.GetEvent(), Actor: r.GetActor().GetLogin(), Branch: r.GetHeadBranch(),
				HeadSHA: r.GetHeadSHA(), Conclusion: r.GetConclusion(), CreatedAt: r.GetCreatedAt().UTC()})
		}
	}
	window := stamp(in.Since) + ".." + stamp(in.Until)
	for page := 1; page != 0; {
		res, resp, err := s.GH.REST.Actions.ListRepositoryWorkflowRuns(ctx, owner, name, &gh.ListWorkflowRunsOptions{Event: "workflow_dispatch", Created: window, ListOptions: gh.ListOptions{PerPage: 100, Page: page}})
		if err != nil {
			return err
		}
		add(res.WorkflowRuns)
		page = resp.NextPage
	}
	for page := 1; page != 0; {
		res, resp, err := s.GH.REST.Actions.ListWorkflowRunsByFileName(ctx, owner, name, "release.yml", &gh.ListWorkflowRunsOptions{Created: "<" + stamp(in.Until), ListOptions: gh.ListOptions{PerPage: 100, Page: page}})
		if github.NotFound(err) {
			break
		}
		if err != nil {
			return err
		}
		add(res.WorkflowRuns)
		page = resp.NextPage
	}
	slices.SortStableFunc(d.Runs, func(a, b Run) int { return int(a.ID - b.ID) })
	return nil
}

// GateSince reads when the merge gate workflow was registered (zero if never).
func GateSince(ctx context.Context, c *github.Client, repo string) (time.Time, error) {
	owner, name, err := github.SplitRepo(repo)
	if err != nil {
		return time.Time{}, err
	}
	wf, _, err := c.REST.Actions.GetWorkflowByFileName(ctx, owner, name, MergeGateWorkflow)
	if github.NotFound(err) {
		return time.Time{}, nil
	}
	return wf.GetCreatedAt().UTC(), err
}

// fetchRefs reads the subjects of promotion commits and whether tag commits are on main.
func (d *Data) fetchRefs(ctx context.Context, s Source, owner, name string) error {
	d.Subjects, d.OnMain = map[string]string{}, map[string]bool{}
	for _, r := range d.RuleSuites {
		switch {
		case r.Ref == "refs/heads/rolling" || r.Ref == "refs/heads/main":
			c, _, err := s.GH.REST.Repositories.GetCommit(ctx, owner, name, r.After, nil)
			if err != nil && !github.NotFound(err) {
				return err
			}
			subj, _, _ := strings.Cut(c.GetCommit().GetMessage(), "\n")
			d.Subjects[r.After] = subj
		case strings.HasPrefix(r.Ref, "refs/tags/") && strings.Trim(r.After, "0") != "":
			cmp, _, err := s.GH.REST.Repositories.CompareCommits(ctx, owner, name, "main", r.After, nil)
			if err != nil && !github.NotFound(err) {
				return err
			}
			d.OnMain[r.After] = cmp.GetStatus() == "identical" || cmp.GetStatus() == "behind"
		}
	}
	return nil
}

var requireLine = regexp.MustCompile(`^\s*(?:require\s+)?([a-zA-Z0-9][^\s]*\.[^\s]+)\s+(v[^\s]+)`)

func (d *Data) fetchLicence(ctx context.Context, s Source, in *Inputs, owner, name string) error {
	cs, _, err := s.GH.REST.Repositories.ListCommits(ctx, owner, name, &gh.CommitsListOptions{SHA: d.Dev, Until: in.Until.Add(-time.Second), ListOptions: gh.ListOptions{PerPage: 1}})
	if err != nil || len(cs) == 0 {
		return err
	}
	in.Code = cs[0].GetSHA()
	file := func(p string) (string, error) {
		fc, _, _, err := s.GH.REST.Repositories.GetContents(ctx, owner, name, p, &gh.RepositoryContentGetOptions{Ref: in.Code})
		if github.NotFound(err) {
			return "", nil
		}
		if err != nil {
			return "", err
		}
		return fc.GetContent()
	}
	mod, err := file("go.mod")
	if err != nil {
		return err
	}
	inBlock := false
	for _, line := range strings.Split(mod, "\n") {
		t := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(t, "require ("):
			inBlock = true
			continue
		case t == ")":
			inBlock = false
			continue
		}
		if !inBlock && !strings.HasPrefix(t, "require ") {
			continue
		}
		if m := requireLine.FindStringSubmatch(t); m != nil {
			d.Licence.Modules = append(d.Licence.Modules, Module{Path: m[1], Version: m[2]})
		}
	}
	if d.Licence.Notices, err = file("THIRD_PARTY_NOTICES.md"); err != nil {
		return err
	}
	res, _, err := s.GH.REST.Checks.ListCheckRunsForRef(ctx, owner, name, in.Code, &gh.ListCheckRunsOptions{CheckName: gh.Ptr("security"), Filter: gh.Ptr("all")})
	if err != nil {
		return err
	}
	for _, r := range res.CheckRuns {
		if r.GetStatus() == "completed" && (r.CompletedAt == nil || r.GetCompletedAt().Before(in.Until)) {
			d.Licence.Security = append(d.Licence.Security, r.GetConclusion())
		}
	}
	slices.Sort(d.Licence.Security)
	return nil
}

func (d *Data) fetchAutonomy(ctx context.Context, s Source, in *Inputs) error {
	prs, issues, err := autonomy.Fetch(ctx, s.GH, s.Repo, s.Rules, autonomy.MaxPRs)
	if err != nil {
		return err
	}
	prs = slices.DeleteFunc(prs, func(p autonomy.PR) bool { return !p.MergedAt.Before(in.Until) })
	issues = slices.DeleteFunc(issues, func(i autonomy.Issue) bool { return !i.CreatedAt.Before(in.Until) })
	for _, c := range s.Rules.Detect(prs, issues) {
		if c.At.Before(in.Until) {
			d.Corrections = append(d.Corrections, c)
		}
	}
	slices.SortStableFunc(d.Corrections, func(a, b autonomy.Correction) int { return strings.Compare(a.Key(), b.Key()) })
	if s.Flow {
		d.Bugs = bugsOf(issues)
	}
	for _, p := range prs {
		if !s.Rules.Agent(p) {
			continue
		}
		a := AgentPR{Number: p.Number, MergedAt: p.MergedAt, Layers: s.Rules.Layers.Footprint(p.Files)}
		for _, f := range p.Files {
			if rest, ok := strings.CutPrefix(f, "skills/"); ok {
				if sk, _, ok := strings.Cut(rest, "/"); ok && !slices.Contains(a.Skills, sk) {
					a.Skills = append(a.Skills, sk)
				}
			}
		}
		var hit []string
		for _, l := range a.Layers {
			if slices.Contains(s.Never, l) {
				hit = append(hit, l)
			}
		}
		if len(hit) > 0 {
			d.NeverMerges = append(d.NeverMerges, NeverMerge{PR: p.Number, At: p.MergedAt, Layers: hit})
		}
		d.AgentPRs = append(d.AgentPRs, a)
	}
	slices.SortStableFunc(d.NeverMerges, func(a, b NeverMerge) int { return a.PR - b.PR })
	slices.SortStableFunc(d.AgentPRs, func(a, b AgentPR) int { return a.Number - b.Number })
	d.Streaks = s.Rules.Streaks(prs, d.Corrections, s.Streak, s.Never)
	return nil
}
