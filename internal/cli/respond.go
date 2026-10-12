package cli

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/identity"
	"github.com/rahulnakmol/agentic-ayudha/internal/initrepo"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/ops"
	"github.com/rahulnakmol/agentic-ayudha/internal/triage"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

func init() { register("respond", runRespond) }

// TimelineMarker marks an incident's replace-in-place timeline comment.
const TimelineMarker = "<!-- aayudha:timeline -->"

// Variables aayudha-ops.yml passes to `respond --ci` from its inputs.
const (
	opsCatalogSHA  = "AAYUDHA_OPS_CATALOG_SHA"
	opsEnvironment = "AAYUDHA_OPS_ENVIRONMENT"
	opsRequest     = "AAYUDHA_OPS_REQUEST"
)

// opsPassEnv are the only variables an executed catalog action sees: no
// GitHub token, no AAYU_*. The credential comes from the approved
// environment: its secret AAYUDHA_OPS_CREDENTIAL, or an OIDC token the job
// may mint (ACTIONS_ID_TOKEN_REQUEST_*, valid for minutes) to exchange for a
// short-lived cloud credential.
var opsPassEnv = []string{"PATH", "HOME", "LANG", "LC_ALL", "TZ", "TMPDIR", "RUNNER_TEMP", "RUNNER_OS", "RUNNER_ARCH",
	"ACTIONS_ID_TOKEN_REQUEST_URL", "ACTIONS_ID_TOKEN_REQUEST_TOKEN", "AAYUDHA_OPS_CREDENTIAL"}

var runNameRE = regexp.MustCompile(`^aayudha-ops (\S+) for #([0-9]+) \((r-[0-9a-f]+)\)$`)

type respondReport struct {
	Incident    int          `json:"incident"`
	CatalogSHA  string       `json:"catalog_sha,omitempty"`
	Actions     []ops.Action `json:"catalog,omitempty"`
	Proposed    *proposal    `json:"proposed,omitempty"`
	Executed    *execution   `json:"executed,omitempty"`
	Bug         int          `json:"linked_bug,omitempty"`
	Timeline    []string     `json:"timeline"`
	NoCatalog   bool         `json:"no_catalog,omitempty"`
	Untrusted   string       `json:"incident_text,omitempty"`
	DryRun      bool         `json:"dry_run,omitempty"`
	Description string       `json:"-"`
}

type proposal struct {
	Action      string            `json:"action"`
	Params      map[string]string `json:"params"`
	Argv        []string          `json:"argv"`
	Environment string            `json:"environment"`
	Approval    string            `json:"approval"`
	Request     string            `json:"request"`
	RunURL      string            `json:"run_url,omitempty"`
}

type execution struct {
	Action     string   `json:"action"`
	Argv       []string `json:"argv"`
	ApprovedBy string   `json:"approved_by,omitempty"`
	ExitCode   int      `json:"exit_code"`
}

// ghSource reads the catalog through the API (ops.Source).
type ghSource struct {
	c           *github.Client
	owner, name string
	def         string // default branch
}

func (s ghSource) File(ctx context.Context, ref, path string) ([]byte, bool, error) {
	fc, _, _, err := s.c.REST.Repositories.GetContents(ctx, s.owner, s.name, path, &gh.RepositoryContentGetOptions{Ref: ref})
	if github.NotFound(err) || (err == nil && fc == nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	content, err := fc.GetContent()
	return []byte(content), err == nil, err
}

func (s ghSource) OnDefault(ctx context.Context, sha string) (bool, error) {
	if !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(sha) {
		return false, nil
	}
	cmp, _, err := s.c.REST.Repositories.CompareCommits(ctx, s.owner, s.name, sha, s.def, &gh.ListOptions{PerPage: 1})
	if github.NotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	st := cmp.GetStatus()
	return st == "ahead" || st == "identical", nil
}

func (s ghSource) History(ctx context.Context, sha, path string, max int) ([]string, error) {
	cs, _, err := s.c.REST.Repositories.ListCommits(ctx, s.owner, s.name, &gh.CommitsListOptions{SHA: sha, Path: path, ListOptions: gh.ListOptions{PerPage: max}})
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(cs))
	for _, c := range cs {
		out = append(out, c.GetSHA())
	}
	return out, nil
}

// runRespond is `aayu respond <incident>` (WP5.6). Without --action it shows
// the catalog at the default branch's head and refreshes the timeline with
// the outcome of earlier requests; --action proposes a catalog action and
// dispatches aayudha-ops.yml, where a human approves the environment;
// --close files the linked Bug and the retro entry. Inside aayudha-ops.yml
// (--ci) it re-verifies everything and executes the action.
func runRespond(ctx context.Context, e *Env, in Values) error {
	repo, err := e.RepoName(ctx)
	if err != nil {
		return err
	}
	l, err := e.Config()
	if err != nil {
		return err
	}
	n := issueNumber(in.String("incident"))
	if e.CI {
		return respondExec(ctx, e, &l.Config, repo, n, in)
	}
	if in.Bool("close") && in.Has("action") {
		return Errorf(ExitUsage, "respond: --close and --action do not combine")
	}
	c, tok, err := e.HumanGitHub(ctx)
	if err != nil {
		return err
	}
	owner, name, _ := github.SplitRepo(repo)
	is, it, err := issueItem(ctx, c, repo, n)
	if err != nil {
		return err
	}
	if it.Type != "incident" {
		return Errorf(ExitUsage, "respond: #%d is a %q, not an Incident", n, it.Type)
	}
	rep := respondReport{Incident: n, DryRun: c.DryRun()}
	timeline, err := readTimeline(ctx, c, repo, n)
	if err != nil {
		return err
	}
	if in.Bool("close") {
		return respondClose(ctx, e, c, tok, repo, is, timeline, rep)
	}
	if is.GetState() != "open" {
		return Errorf(ExitBlocked, "respond: incident #%d is closed", n)
	}
	r, _, err := c.REST.Repositories.Get(ctx, owner, name)
	if err != nil {
		return err
	}
	def := r.GetDefaultBranch()
	br, _, err := c.REST.Repositories.GetBranch(ctx, owner, name, def, 0)
	if err != nil {
		return err
	}
	src := ghSource{c: c, owner: owner, name: name, def: def}
	cat, err := ops.Load(ctx, src, br.GetCommit().GetSHA())
	switch {
	case errors.Is(err, ops.ErrNoCatalog) && !in.Has("action"):
		rep.NoCatalog = true
	case errors.Is(err, ops.ErrNoCatalog):
		return Errorf(ExitBlocked, "respond: %v; respond executes catalog actions only", err)
	case err != nil:
		return Wrap(ExitBlocked, err)
	default:
		rep.CatalogSHA, rep.Actions = cat.SHA, cat.Actions
	}
	lines, err := outcomes(ctx, c, &l.Config, repo, n, timeline)
	if err != nil {
		return err
	}
	if in.Has("action") {
		p, line, err := propose(ctx, c, cat, repo, def, n, in, e.Now())
		if err != nil {
			return err
		}
		rep.Proposed = p
		lines = append(lines, line)
	} else {
		rep.Untrusted = triage.Untrusted("issue", is.GetTitle()+"\n\n"+is.GetBody())
	}
	if rep.Timeline, err = writeTimeline(ctx, c, repo, n, timeline, lines, e.Now()); err != nil {
		return err
	}
	return e.Emit(rep, func(w io.Writer) error {
		if p := rep.Proposed; p != nil {
			fmt.Fprintf(w, "proposed %s %s in environment %s (approval %s), request %s\n", p.Action, strings.Join(p.Argv, " "), p.Environment, p.Approval, p.Request)
			if p.Approval == ops.Human {
				fmt.Fprintf(w, "awaiting a human reviewer's approval of environment %s: %s\n", p.Environment, p.RunURL)
			} else if p.RunURL != "" {
				fmt.Fprintf(w, "dispatched: %s\n", p.RunURL)
			}
			return nil
		}
		fmt.Fprintln(w, rep.Untrusted)
		if rep.NoCatalog {
			fmt.Fprintf(w, "no ops catalog (%s) on the default branch: there is nothing respond may run\n", ops.Path)
		} else {
			fmt.Fprintf(w, "ops catalog at %s:\n", short(rep.CatalogSHA))
			for _, a := range rep.Actions {
				fmt.Fprintf(w, "  %s: %s (approval %s, environments %s, max %d/h, rollback %s, blast radius %s)\n", a.Name, a.Summary, a.Effective, strings.Join(a.Environments, ","), a.MaxPerHour, a.Rollback, a.BlastRadius)
			}
		}
		for _, t := range rep.Timeline {
			fmt.Fprintln(w, t)
		}
		return nil
	})
}

// propose validates a catalog action and dispatches aayudha-ops.yml for it.
func propose(ctx context.Context, c *github.Client, cat *ops.Catalog, repo, def string, n int, in Values, now time.Time) (*proposal, string, error) {
	owner, name, _ := github.SplitRepo(repo)
	a, ok := cat.Find(in.String("action"))
	if !ok {
		return nil, "", Errorf(ExitBlocked, "respond: %q: %v at %s; respond never runs anything else", in.String("action"), ops.ErrUnknown, short(cat.SHA))
	}
	params, err := ops.ParseParams(in.String("params"))
	if err != nil {
		return nil, "", Wrap(ExitUsage, err)
	}
	env, err := a.Environment(params)
	if err != nil {
		return nil, "", Wrap(ExitUsage, err)
	}
	argv, err := a.Render(params)
	if err != nil {
		return nil, "", Wrap(ExitUsage, err)
	}
	if a.Effective == ops.Human {
		if err := requireReviewers(ctx, c, owner, name, env); err != nil {
			return nil, "", err
		}
	}
	if err := rateLimit(ctx, c, owner, name, a, 0, now); err != nil {
		return nil, "", err
	}
	id := make([]byte, 6)
	_, _ = rand.Read(id)
	p := &proposal{Action: a.Name, Params: params, Argv: argv, Environment: env, Approval: a.Effective, Request: "r-" + hex.EncodeToString(id)}
	inputs := map[string]any{"incident": strconv.Itoa(n), "action": a.Name, "params": in.String("params"), "environment": env, "catalog_sha": cat.SHA, "request": p.Request}
	_, err = c.Write(ctx, actions.Action{Kind: "github.workflow.dispatch", Target: fmt.Sprintf("%s#%d", repo, n),
		Request: &actions.Request{Method: http.MethodPost, Path: fmt.Sprintf("/repos/%s/actions/workflows/%s/dispatches", repo, initrepo.OpsWorkflow)}, Payload: inputs},
		func(ctx context.Context) (any, error) {
			d, _, err := c.REST.Actions.CreateWorkflowDispatchEventByFileName(ctx, owner, name, initrepo.OpsWorkflow,
				gh.CreateWorkflowDispatchEventRequest{Ref: def, Inputs: inputs, ReturnRunDetails: gh.Ptr(true)})
			p.RunURL = d.GetHTMLURL()
			return map[string]any{"run_url": p.RunURL}, err
		})
	if err != nil {
		return nil, "", err
	}
	line := fmt.Sprintf("proposed `%s` (%s) in environment `%s` from %s at %s; approval %s; request %s", a.Name, paramText(params), env, ops.Path, short(cat.SHA), a.Effective, p.Request)
	if p.RunURL != "" {
		line += "; run " + p.RunURL
	}
	return p, line, nil
}

func paramText(p map[string]string) string {
	if len(p) == 0 {
		return "no parameters"
	}
	keys := make([]string, 0, len(p))
	for k := range p {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for i, k := range keys {
		keys[i] = k + "=" + p[k]
	}
	return strings.Join(keys, ", ")
}

// requireReviewers refuses an environment that a human must approve but that
// has no required reviewers, or that any branch may deploy to.
func requireReviewers(ctx context.Context, c *github.Client, owner, name, env string) error {
	en, _, err := c.REST.Repositories.GetEnvironment(ctx, owner, name, env)
	if github.NotFound(err) {
		return Errorf(ExitBlocked, "respond: GitHub Environment %q does not exist; create it with required reviewers", env)
	}
	if err != nil {
		return err
	}
	reviewers := 0
	for _, r := range en.ProtectionRules {
		if r.GetType() == "required_reviewers" {
			reviewers += len(r.Reviewers)
		}
	}
	if reviewers == 0 {
		return Errorf(ExitBlocked, "respond: environment %q has no required reviewers, so an approval = human action cannot run there", env)
	}
	if en.DeploymentBranchPolicy == nil {
		return Errorf(ExitBlocked, "respond: environment %q lets any branch deploy; restrict it to protected branches", env)
	}
	return nil
}

// rateLimit counts the action's runs of aayudha-ops.yml in the last hour,
// other than self, that were not cancelled or skipped.
func rateLimit(ctx context.Context, c *github.Client, owner, name string, a ops.Action, self int64, now time.Time) error {
	runs, _, err := c.REST.Actions.ListWorkflowRunsByFileName(ctx, owner, name, initrepo.OpsWorkflow,
		&gh.ListWorkflowRunsOptions{Event: "workflow_dispatch", ListOptions: gh.ListOptions{PerPage: 100}})
	if github.NotFound(err) {
		return Errorf(ExitBlocked, "respond: %s is not installed; run `aayu init --gates`", initrepo.OpsWorkflow)
	}
	if err != nil {
		return err
	}
	since := now.Add(-time.Hour)
	count := 0
	for _, r := range runs.WorkflowRuns {
		m := runNameRE.FindStringSubmatch(r.GetName())
		if r.GetID() == self || m == nil || m[1] != a.Name || r.GetCreatedAt().Before(since) {
			continue
		}
		if c := r.GetConclusion(); c == "cancelled" || c == "skipped" {
			continue
		}
		count++
	}
	if count >= a.MaxPerHour {
		return Errorf(ExitBlocked, "respond: %s ran %d time(s) in the last hour; max_per_hour is %d", a.Name, count, a.MaxPerHour)
	}
	return nil
}

type approval struct {
	State        string `json:"state"`
	Environments []struct {
		Name string `json:"name"`
	} `json:"environments"`
	User identity.Account `json:"user"`
}

func runApprovals(ctx context.Context, c *github.Client, owner, name string, id int64) ([]approval, error) {
	req, err := c.REST.NewRequest(ctx, http.MethodGet, fmt.Sprintf("repos/%s/%s/actions/runs/%d/approvals", owner, name, id), nil)
	if err != nil {
		return nil, err
	}
	var as []approval
	_, err = c.REST.Do(req, &as)
	return as, err
}

// humanApproval returns the login of a human who approved env on run id.
func humanApproval(ctx context.Context, c *github.Client, cfg *config.Config, owner, name string, id int64, env string) (string, error) {
	as, err := runApprovals(ctx, c, owner, name, id)
	if err != nil {
		return "", err
	}
	for _, a := range as {
		if a.State != "approved" || identity.IsBot(a.User, cfg.Identity.BotLogins) {
			continue
		}
		for _, en := range a.Environments {
			if en.Name == env {
				return a.User.Login, nil
			}
		}
	}
	return "", nil
}

// respondExec runs inside aayudha-ops.yml after the environment's approval:
// it checks the run, the pinned catalog, the parameters, the human approval,
// and the hourly cap again, then executes the action with only the
// environment's credential in its environment.
func respondExec(ctx context.Context, e *Env, cfg *config.Config, repo string, n int, in Values) error {
	refuse := func(format string, args ...any) error { return Errorf(ExitBlocked, "respond --ci: "+format, args...) }
	if e.Getenv("GITHUB_ACTIONS") != "true" {
		return refuse("runs only inside %s", initrepo.OpsWorkflow)
	}
	c, _, err := e.HumanGitHub(ctx) // the job's workflow token: reads only
	if err != nil {
		return err
	}
	owner, name, _ := github.SplitRepo(repo)
	r, _, err := c.REST.Repositories.Get(ctx, owner, name)
	if err != nil {
		return err
	}
	def := r.GetDefaultBranch()
	wantRef := fmt.Sprintf("%s/.github/workflows/%s@refs/heads/%s", repo, initrepo.OpsWorkflow, def)
	if got := e.Getenv("GITHUB_WORKFLOW_REF"); got != wantRef {
		return refuse("runs only from %s (this is %q)", wantRef, got)
	}
	id, err := strconv.ParseInt(e.Getenv("GITHUB_RUN_ID"), 10, 64)
	if err != nil {
		return refuse("no GITHUB_RUN_ID")
	}
	run, _, err := c.REST.Actions.GetWorkflowRunByID(ctx, owner, name, id)
	if err != nil {
		return err
	}
	if run.GetPath() != ".github/workflows/"+initrepo.OpsWorkflow || run.GetHeadBranch() != def || run.GetEvent() != "workflow_dispatch" {
		return refuse("run %d is not a dispatch of %s on %s", id, initrepo.OpsWorkflow, def)
	}
	m := runNameRE.FindStringSubmatch(run.GetName())
	if m == nil || m[1] != in.String("action") || m[2] != strconv.Itoa(n) || m[3] != e.Getenv(opsRequest) {
		return refuse("run %d (%q) does not match the request", id, run.GetName())
	}
	cat, err := ops.Load(ctx, ghSource{c: c, owner: owner, name: name, def: def}, e.Getenv(opsCatalogSHA))
	if err != nil {
		return Wrap(ExitBlocked, err)
	}
	a, ok := cat.Find(in.String("action"))
	if !ok {
		return refuse("%q: %v at %s", in.String("action"), ops.ErrUnknown, short(cat.SHA))
	}
	params, err := ops.ParseParams(in.String("params"))
	if err != nil {
		return Wrap(ExitUsage, err)
	}
	env, err := a.Environment(params)
	if err != nil {
		return Wrap(ExitUsage, err)
	}
	if env != e.Getenv(opsEnvironment) {
		return refuse("the job runs in environment %q but %s runs in %q", e.Getenv(opsEnvironment), a.Name, env)
	}
	argv, err := a.Render(params)
	if err != nil {
		return Wrap(ExitUsage, err)
	}
	ex := &execution{Action: a.Name, Argv: argv}
	if a.Effective == ops.Human {
		if err := requireReviewers(ctx, c, owner, name, env); err != nil {
			return err
		}
		if ex.ApprovedBy, err = humanApproval(ctx, c, cfg, owner, name, id, env); err != nil {
			return err
		}
		if ex.ApprovedBy == "" {
			return refuse("no human approved environment %s on run %d", env, id)
		}
	}
	if err := rateLimit(ctx, c, owner, name, a, id, e.Now()); err != nil {
		return err
	}
	var childEnv []string
	for _, k := range opsPassEnv {
		if v := e.Getenv(k); v != "" {
			childEnv = append(childEnv, k+"="+v)
		}
	}
	childEnv = append(childEnv, "AAYUDHA_OPS_ENVIRONMENT="+env, "AAYUDHA_OPS_ACTION="+a.Name, fmt.Sprintf("AAYUDHA_OPS_INCIDENT=%d", n))
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir, cmd.Env = e.Dir, childEnv
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err = cmd.Run()
	ex.ExitCode = cmd.ProcessState.ExitCode()
	if err != nil && cmd.ProcessState == nil {
		return Errorf(ExitExternal, "respond: %s: %v", a.Name, err)
	}
	b := out.Bytes()
	if len(b) > 64<<10 {
		b = append(b[:64<<10], "\n… output truncated\n"...)
	}
	if err := e.Emit(map[string]any{"incident": n, "executed": ex, "catalog_sha": cat.SHA, "output": string(b)}, func(w io.Writer) error {
		fmt.Fprintf(w, "executed %s (%s) for incident #%d from %s at %s", a.Name, strings.Join(argv, " "), n, ops.Path, short(cat.SHA))
		if ex.ApprovedBy != "" {
			fmt.Fprintf(w, ", approved by @%s", ex.ApprovedBy)
		}
		fmt.Fprintf(w, "; exit %d\n%s", ex.ExitCode, b)
		return nil
	}); err != nil {
		return err
	}
	if ex.ExitCode != 0 {
		return Errorf(ExitExternal, "respond: %s exited %d", a.Name, ex.ExitCode)
	}
	return nil
}

// outcomes turns finished aayudha-ops.yml runs for incident n into timeline
// lines, once each, with the human who approved them.
func outcomes(ctx context.Context, c *github.Client, cfg *config.Config, repo string, n int, timeline []string) ([]string, error) {
	owner, name, _ := github.SplitRepo(repo)
	runs, _, err := c.REST.Actions.ListWorkflowRunsByFileName(ctx, owner, name, initrepo.OpsWorkflow,
		&gh.ListWorkflowRunsOptions{Event: "workflow_dispatch", ListOptions: gh.ListOptions{PerPage: 100}})
	if github.NotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var lines []string
	for i := len(runs.WorkflowRuns) - 1; i >= 0; i-- {
		r := runs.WorkflowRuns[i]
		m := runNameRE.FindStringSubmatch(r.GetName())
		if m == nil || m[2] != strconv.Itoa(n) || r.GetStatus() != "completed" {
			continue
		}
		done := "request " + m[3] + " finished"
		if slices.ContainsFunc(timeline, func(l string) bool { return strings.Contains(l, done) }) {
			continue
		}
		line := fmt.Sprintf("%s: `%s` %s (run %s)", done, m[1], r.GetConclusion(), r.GetHTMLURL())
		if who, err := approvers(ctx, c, cfg, owner, name, r.GetID()); err == nil && who != "" {
			line += "; environment approved by " + who
		}
		lines = append(lines, line)
	}
	return lines, nil
}

func approvers(ctx context.Context, c *github.Client, cfg *config.Config, owner, name string, id int64) (string, error) {
	as, err := runApprovals(ctx, c, owner, name, id)
	if err != nil {
		return "", err
	}
	var who []string
	for _, a := range as {
		kind := "human"
		if identity.IsBot(a.User, cfg.Identity.BotLogins) {
			kind = "bot, not a human pass"
		}
		who = append(who, fmt.Sprintf("@%s (%s, %s)", a.User.Login, a.State, kind))
	}
	return strings.Join(who, ", "), nil
}

var timelineLine = regexp.MustCompile(`^- `)

func readTimeline(ctx context.Context, c *github.Client, repo string, n int) ([]string, error) {
	owner, name, _ := github.SplitRepo(repo)
	cs, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.IssueComment, *gh.Response, error) {
		return c.REST.Issues.ListComments(ctx, owner, name, n, &gh.IssueListCommentsOptions{ListOptions: o})
	})
	if err != nil {
		return nil, err
	}
	for _, cm := range cs {
		if !strings.Contains(cm.GetBody(), TimelineMarker) {
			continue
		}
		var lines []string
		for _, l := range strings.Split(cm.GetBody(), "\n") {
			if timelineLine.MatchString(l) {
				lines = append(lines, l)
			}
		}
		return lines, nil
	}
	return nil, nil
}

// writeTimeline appends entries to the incident's timeline comment.
func writeTimeline(ctx context.Context, c *github.Client, repo string, n int, timeline, add []string, now time.Time) ([]string, error) {
	out := append([]string{}, timeline...)
	for _, a := range add {
		out = append(out, "- "+now.UTC().Format("2006-01-02 15:04 UTC")+": "+a)
	}
	if len(add) == 0 {
		return out, nil
	}
	body := TimelineMarker + "\n### Incident timeline\n\nKept by `aayu respond`; entries are appended, never rewritten.\n\n" + strings.Join(out, "\n") + "\n"
	_, err := c.UpsertComment(ctx, repo, n, TimelineMarker, body, actions.Action{Kind: "github.comment.timeline"})
	return out, err
}

var linkedBug = regexp.MustCompile(`linked Bug #([0-9]+)`)

// respondClose files the incident's linked Bug (caused-by), appends the retro
// entry to the ledger, and closes the incident.
func respondClose(ctx context.Context, e *Env, c *github.Client, tok identity.Token, repo string, is *gh.Issue, timeline []string, rep respondReport) error {
	owner, name, _ := github.SplitRepo(repo)
	n := is.GetNumber()
	for _, l := range timeline {
		if m := linkedBug.FindStringSubmatch(l); m != nil {
			rep.Bug, _ = strconv.Atoi(m[1])
		}
	}
	var add []string
	if rep.Bug == 0 {
		t, _ := workitems.FindType("bug")
		title := fmt.Sprintf("Fix the cause of incident #%d: %s", n, is.GetTitle())
		body := workitems.Render(t, map[string]string{
			workitems.FieldSummary: fmt.Sprintf("Found through incident #%d; its timeline holds what was observed and done.", n),
			"reproduce":            fmt.Sprintf("See the timeline of #%d.", n),
			"expected":             "The service behaves as it did before the incident.",
			"actual":               fmt.Sprintf("Incident #%d: %s", n, is.GetTitle()),
			workitems.FieldLinks:   fmt.Sprintf("caused-by #%d", n),
		})
		_, err := c.Write(ctx, actions.Action{Kind: "github.issue.create", Target: repo,
			Request: &actions.Request{Method: http.MethodPost, Path: fmt.Sprintf("/repos/%s/issues", repo)}, Payload: map[string]any{"title": title, "caused_by": n}},
			func(ctx context.Context) (any, error) {
				b, _, err := c.REST.Issues.Create(ctx, owner, name, gh.CreateIssueRequest{Title: title, Body: gh.Ptr(body), Labels: []string{"type:bug", "status:backlog"}})
				rep.Bug = b.GetNumber()
				return map[string]any{"number": rep.Bug}, err
			})
		if err != nil {
			return err
		}
		add = append(add, fmt.Sprintf("linked Bug #%d filed (caused-by #%d)", rep.Bug, n))
	}
	entry, _ := json.Marshal(map[string]any{"incident": n, "repo": repo, "title": is.GetTitle(), "bug": rep.Bug, "opened": is.GetCreatedAt().UTC(), "closed": e.Now().UTC(), "timeline": timeline})
	if !c.DryRun() {
		w, err := e.Ledger(ctx, tok, "human", "")
		if err != nil {
			return err
		}
		if _, err := w.Write(ctx, ledger.Commit{Message: fmt.Sprintf("retro: incident #%d closed", n), Append: map[string][]byte{"retro/incidents.jsonl": append(entry, '\n')}}); err != nil {
			return err
		}
	}
	add = append(add, "retro entry appended to ledger retro/incidents.jsonl; incident closed")
	var err error
	if rep.Timeline, err = writeTimeline(ctx, c, repo, n, timeline, add, e.Now()); err != nil {
		return err
	}
	if err := removeLabel(ctx, c, repo, n, workitems.LabelNeedsBug); err != nil {
		return err
	}
	if err := setStatus(ctx, c, repo, n, github.IssueLabels(is), "done"); err != nil {
		return err
	}
	if is.GetState() == "open" {
		if err := closeIssue(ctx, c, repo, n, "completed"); err != nil {
			return err
		}
	}
	return e.Emit(rep, func(w io.Writer) error {
		fmt.Fprintf(w, "incident #%d closed; linked Bug #%d; retro entry on ledger retro/incidents.jsonl\n", n, rep.Bug)
		return nil
	})
}
