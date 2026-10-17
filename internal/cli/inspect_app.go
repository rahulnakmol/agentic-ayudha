package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/identity"
	"github.com/rahulnakmol/agentic-ayudha/internal/inspect"
	"github.com/rahulnakmol/agentic-ayudha/internal/notify"
)

// Dispositions of a review that has no usable reply yet: both fail the gate
// closed (exit 4), and neither counts as a fix cycle.
const (
	NoReply   = "no-reply"
	Malformed = "malformed-reply"
)

// appInspect is one inspection through a reviewer app or a human.
type appInspect struct {
	c       *github.Client
	repo    string
	pr      *gh.PullRequest
	in      inspect.Input
	prev    inspect.State
	builder inspect.Builder
	cfg     config.Config
}

// inspectThroughApp is inspect without a model key (founder decision
// 2026-10-15): the reviewer is an agent app installed on the org or account
// (or a human), never the builder's family. Outside CI it posts the review
// request, under the person's own account because the apps answer only the
// people who connected them; in the merge gate it reads the reply, turns it
// into the disposition and verdicts through the same rules as a model's
// reply, and fails closed when the reply is missing or malformed. Run on an
// issue_comment event, the gate re-runs its own failed inspect job once a
// reply for the head exists.
func (e *Env) inspectThroughApp(ctx context.Context, a appInspect) error {
	n, head := a.pr.GetNumber(), a.pr.GetHead().GetSHA()
	owner, name, _ := github.SplitRepo(a.repo)
	choice := inspect.ChooseReviewer(a.cfg.Inspect.Reviewer, a.cfg.Inspect.Apps, a.cfg.Inspect.AppFamilies, a.builder.Candidates, a.cfg.Autonomy.Approver)
	notes := append([]string{a.builder.Note(), "reviewer: " + choice.Kind() + "; the merge gate holds no AI key and reads its reply"}, choice.Notes...)
	if a.in.Mode == inspect.ModePeer {
		notes = append(notes, "peer mode needs two model reviewers; a reviewer app reviews alone, high-risk PRs included")
	}
	res := inspect.Result{PR: n, Head: head, Findings: []inspect.Finding{}, Criteria: []inspect.Verdict{}, Builder: a.builder}
	if a.in.Draft {
		res.Disposition, res.Summary = inspect.Skipped, "draft pull request: inspect runs when it is ready for review"
		return e.emitInspect(res, notes)
	}
	if inspect.DocsOnly(inspect.ParseDiff(a.in.Diff)) {
		res.Disposition, res.Summary = inspect.Skipped, "the PR changes only documentation, which merges without a review"
		return e.emitInspect(res, notes)
	}
	posts, err := prPosts(ctx, a.c, owner, name, n)
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	replyEvent := e.CI && (e.Getenv("GITHUB_EVENT_NAME") == "issue_comment" || e.Getenv("GITHUB_EVENT_NAME") == "pull_request_review")
	if a.in.Pack == nil && a.in.Story > 0 && e.CI && !replyEvent {
		return Errorf(ExitBlocked, "inspect #%d: %v", n, inspect.ErrNoPack)
	}
	var since time.Time
	for _, p := range posts {
		if rs, ok := inspect.ParseRequest(p.Body); ok && p.Kind == "comment" && rs.Head == head && (since.IsZero() || p.Updated.Before(since)) {
			since = p.Updated
		}
	}
	if !e.CI && since.IsZero() {
		hc, _, err := e.HumanGitHub(ctx)
		if err != nil {
			return err
		}
		marker := fmt.Sprintf(`%s{"head":%q`, inspect.RequestMarker, head)
		if _, err := hc.UpsertComment(ctx, a.repo, n, marker, inspect.Request(choice, a.in, head), actions.Action{Kind: "github.comment.review-request",
			Payload: map[string]any{"head": head, "reviewer": choice.Kind()}}); err != nil {
			return Wrap(ExitExternal, err)
		}
		notes = append(notes, "review requested from "+choice.Kind()+" under your account")
		since = e.Now()
	}
	files := inspect.ParseDiff(a.in.Diff)
	var changed []string
	for _, f := range files {
		if !f.Deleted {
			changed = append(changed, f.Path)
		}
	}
	bots := a.cfg.Identity.BotLogins
	human := func(login, typ string) bool { return !identity.IsBot(identity.Account{Login: login, Type: typ}, bots) }
	rep, rerr := inspect.ReadReply(choice, posts, head, since, human, a.in.Criteria, changed)
	if replyEvent {
		return e.rerunInspect(ctx, a.c, owner, name, n, head, choice, rerr)
	}
	pager, err := e.Notifier(ctx)
	if err != nil {
		return err
	}
	r := inspect.Runner{Filter: inspect.Filter{MinConfidence: a.cfg.Inspect.MinConfidence, SuppressAfter: a.cfg.Inspect.SuppressAfter, Generated: a.cfg.Inspect.Generated},
		Page: func(ctx context.Context, subject, body string) error {
			_, err := pager.Page(ctx, notify.Page{Key: fmt.Sprintf("inspect:%s#%d", a.repo, n), Subject: subject, Body: body})
			return err
		}}
	st := a.prev
	switch {
	case rerr == nil && a.in.Pack == nil && a.in.Story > 0:
		res.Disposition, res.Summary = NoReply, fmt.Sprintf("%s replied (%s); the merge gate judges the reply against its pack for `%s`", choice.Kind(), rep.Source, short(head))
		res.Next = "the merge gate's inspect check reads it"
	case errors.Is(rerr, inspect.ErrNoReply), errors.Is(rerr, inspect.ErrMalformed):
		res.Disposition = NoReply
		if errors.Is(rerr, inspect.ErrMalformed) {
			res.Disposition = Malformed
		}
		res.Summary = fmt.Sprintf("No usable review of `%s` from %s yet, so the gate fails closed: %v.", short(head), choice.Kind(), rerr)
		res.Next = fmt.Sprintf("the reviewer replies on this PR in the stated format (a review request: `aayu inspect %d`, run by the builder or a person under their own account); the reply re-runs this check", n)
		if since.IsZero() {
			notes = append(notes, "no review request for this head on the PR yet")
		}
	case rerr != nil:
		return Wrap(ExitExternal, rerr)
	default:
		var err error
		res, st, err = r.Judge(ctx, a.in, a.prev, rep)
		res.Head, res.Builder = head, a.builder
		switch {
		case errors.Is(err, inspect.ErrCycles):
			return Errorf(ExitBlocked, "inspect #%d: %v (paged)", n, err)
		case err != nil:
			return Wrap(ExitExternal, err)
		}
	}
	res.Notes = append(notes, res.Notes...)
	if st.Result != nil && st.Result.DiffHash == res.DiffHash {
		st.Result.Notes = res.Notes
	}
	if e.CI && res.Disposition != inspect.Skipped && !res.Cached {
		if _, err := a.c.UpsertComment(ctx, a.repo, n, inspect.Marker, inspect.Comment(res, st, 0), actions.Action{Kind: "github.comment.inspect",
			Payload: map[string]any{"disposition": res.Disposition, "diff_hash": res.DiffHash, "reviewer": choice.Kind()}}); err != nil {
			return err
		}
	}
	if err := e.emitInspect(res, nil); err != nil {
		return err
	}
	if res.Disposition != inspect.Merge && res.Disposition != inspect.Skipped {
		return Errorf(ExitBlocked, "inspect #%d: %s", n, res.Disposition)
	}
	return nil
}

func (e *Env) emitInspect(res inspect.Result, notes []string) error {
	res.Notes = append(notes, res.Notes...)
	return e.Emit(res, func(w io.Writer) error {
		fmt.Fprintf(w, "inspect #%d at %s: %s", res.PR, short(res.Head), res.Disposition)
		if res.Cached {
			fmt.Fprint(w, " (cached)")
		}
		if res.Model != "" {
			fmt.Fprintf(w, " (reviewed by %s)", res.Model)
		}
		fmt.Fprintln(w)
		if res.Overridden != "" {
			fmt.Fprintf(w, "  %s\n", res.Overridden)
		}
		if res.Summary != "" && (res.Disposition == NoReply || res.Disposition == Malformed) {
			fmt.Fprintf(w, "  %s\n", res.Summary)
		}
		for _, f := range res.Findings {
			fmt.Fprintf(w, "  %s %s %s:%d %s [%s]\n", f.Severity, f.Category, f.File, f.Line, f.Title, f.ID)
		}
		for _, v := range res.Criteria {
			fmt.Fprintf(w, "  criterion %d: %s\n", v.ID, v.Verdict)
		}
		if m := res.Coverage; m != nil {
			fmt.Fprintf(w, "  coverage: %s (manifest %s)\n", m.State, short(m.SHA256))
		}
		for _, x := range res.Weakened {
			fmt.Fprintf(w, "  weakened: %s: %s\n", x.File, x.What)
		}
		for _, x := range res.Notes {
			fmt.Fprintf(w, "  note: %s\n", x)
		}
		if res.Next != "" {
			fmt.Fprintf(w, "  next: %s\n", res.Next)
		}
		return nil
	})
}

// prPosts reads the PR's comments, reviews, and inline review comments.
func prPosts(ctx context.Context, c *github.Client, owner, name string, n int) ([]inspect.Post, error) {
	var out []inspect.Post
	cs, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.IssueComment, *gh.Response, error) {
		return c.REST.Issues.ListComments(ctx, owner, name, n, &gh.IssueListCommentsOptions{ListOptions: o})
	})
	if err != nil {
		return nil, err
	}
	for _, x := range cs {
		out = append(out, inspect.Post{Kind: "comment", ID: x.GetID(), Login: x.GetUser().GetLogin(), Type: x.GetUser().GetType(), Body: x.GetBody(),
			Updated: x.GetUpdatedAt().Time, URL: x.GetHTMLURL()})
	}
	rs, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.PullRequestReview, *gh.Response, error) {
		return c.REST.PullRequests.ListReviews(ctx, owner, name, n, &o)
	})
	if err != nil {
		return nil, err
	}
	for _, x := range rs {
		out = append(out, inspect.Post{Kind: "review", ID: x.GetID(), Login: x.GetUser().GetLogin(), Type: x.GetUser().GetType(), Body: x.GetBody(),
			Commit: x.GetCommitID(), Updated: x.GetSubmittedAt().Time, URL: x.GetHTMLURL(), Approved: x.GetState() == "APPROVED"})
	}
	ls, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.PullRequestComment, *gh.Response, error) {
		return c.REST.PullRequests.ListComments(ctx, owner, name, n, &gh.PullRequestListCommentsOptions{ListOptions: o})
	})
	if err != nil {
		return nil, err
	}
	for _, x := range ls {
		commit := x.GetOriginalCommitID()
		if commit == "" {
			commit = x.GetCommitID()
		}
		line := x.GetLine()
		if line == 0 {
			line = x.GetOriginalLine()
		}
		out = append(out, inspect.Post{Kind: "inline", ID: x.GetID(), Login: x.GetUser().GetLogin(), Type: x.GetUser().GetType(), Body: x.GetBody(),
			Commit: commit, Path: x.GetPath(), Line: line, Updated: x.GetUpdatedAt().Time, URL: x.GetHTMLURL()})
	}
	return out, nil
}

// rerunInspect runs on a comment or review event in the merge gate's
// workflow: when the PR's head
// has a reply from its reviewer (usable or not), it re-runs the failed jobs
// of the newest pull_request_target gate run on that head, so the inspect
// check on the head itself, not a status anyone could set, carries the
// verdict. A comment with no reply for the head changes nothing.
func (e *Env) rerunInspect(ctx context.Context, c *github.Client, owner, name string, n int, head string, choice inspect.Choice, rerr error) error {
	say := func(format string, args ...any) error {
		msg := fmt.Sprintf(format, args...)
		return e.Emit(map[string]any{"pr": n, "head": head, "reviewer": choice.Kind(), "result": msg}, func(w io.Writer) error {
			_, err := fmt.Fprintf(w, "inspect #%d at %s: %s\n", n, short(head), msg)
			return err
		})
	}
	if errors.Is(rerr, inspect.ErrNoReply) {
		return say("no reply from %s for this head; nothing to re-run", choice.Kind())
	}
	path := ".github/workflows/" + MergeGateWorkflow
	for deadline := e.Now().Add(10 * time.Minute); ; {
		runs, _, err := c.REST.Actions.ListWorkflowRunsByFileName(ctx, owner, name, MergeGateWorkflow, &gh.ListWorkflowRunsOptions{HeadSHA: head, Event: "pull_request_target", ListOptions: gh.ListOptions{PerPage: 20}})
		if err != nil {
			return Wrap(ExitExternal, err)
		}
		var run *gh.WorkflowRun
		for _, r := range runs.WorkflowRuns {
			if r.GetPath() == path && r.GetEvent() == "pull_request_target" && r.GetHeadSHA() == head {
				run = r
				break // newest first
			}
		}
		if run == nil {
			return say("no pull_request_target run of %s on this head to re-run", MergeGateWorkflow)
		}
		if run.GetStatus() != "completed" {
			if e.Now().After(deadline) {
				return Errorf(ExitExternal, "inspect #%d: run %d is still %s after 10 minutes", n, run.GetID(), run.GetStatus())
			}
			if err := e.sleep(ctx); err != nil {
				return err
			}
			continue
		}
		checks, _, err := c.REST.Checks.ListCheckRunsForRef(ctx, owner, name, head, &gh.ListCheckRunsOptions{ListOptions: gh.ListOptions{PerPage: 100}})
		if err != nil {
			return Wrap(ExitExternal, err)
		}
		state := map[string]string{}
		for _, r := range checks.CheckRuns {
			if _, seen := state[r.GetName()]; !seen { // newest first
				state[r.GetName()] = r.GetConclusion()
			}
		}
		if state["aayudha/merge-gate"] != "success" || state["aayudha/inspect"] != "failure" {
			return say("run %d: merge-gate %s, inspect %s; nothing to re-run", run.GetID(), or(state["aayudha/merge-gate"], "missing"), or(state["aayudha/inspect"], "missing"))
		}
		if _, err := c.Write(ctx, actions.Action{Kind: "github.actions.rerun", Target: fmt.Sprintf("%s/%s#%d", owner, name, n),
			Payload: map[string]any{"run": run.GetID(), "head": head, "reason": "reply from " + choice.Kind()}},
			func(ctx context.Context) (any, error) {
				_, err := c.REST.Actions.RerunFailedJobsByID(ctx, owner, name, run.GetID())
				return run.GetID(), err
			}); err != nil {
			return Wrap(ExitExternal, err)
		}
		return say("re-ran the failed jobs of run %d so its inspect check reads the reply from %s", run.GetID(), choice.Kind())
	}
}

// requestReview posts the review request for PR n's head to its reviewer app
// (inspect outside CI, under the person's account), for build and delegate
// syncs, so no one has to remember it. It returns a note; an error never stops
// the caller, since the merge gate fails closed without a reply anyway.
func (e *Env) requestReview(ctx context.Context, n int) string {
	if l, err := e.Config(); err != nil || l.Config.Inspect.Reviewer == inspect.ReviewerModel || e.CI {
		return ""
	}
	sub := *e
	sub.JSON = true
	var out bytes.Buffer
	sub.Out, sub.Err = &out, io.Discard
	err := runInspect(ctx, &sub, Values{"pr": strconv.Itoa(n)})
	var res inspect.Result
	if json.NewDecoder(&out).Decode(&res) != nil {
		if err != nil {
			return "review request not posted: " + oneLineErr(err)
		}
		return ""
	}
	if res.Disposition == inspect.Merge {
		return fmt.Sprintf("PR #%d's reviewer replied merge at %s", n, short(res.Head))
	}
	return fmt.Sprintf("review of PR #%d at %s requested (%s)", n, short(res.Head), res.Disposition)
}
