package cli

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	"github.com/rahulnakmol/agentic-ayudha/internal/inspect"
	"github.com/rahulnakmol/agentic-ayudha/internal/notify"
	"github.com/rahulnakmol/agentic-ayudha/internal/prbody"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
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
	// record rewrites the state comment even for a cached judgement: prev
	// carries something new the gate must keep (a failed paid call's head).
	record bool
	// paidTried: rung B already reviewed and failed on this head, or cannot
	// step in, so when no installed app can review (inspect.ErrUnavailable)
	// a human does. Without it that error is returned for rung B to step in.
	paidTried bool
	// fallback says why rung B did not review this head, for the comment.
	fallback string
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
	// In the gate under reviewer = "ci" with its model declined on this head
	// (not failed), no one can ask an app that answers only a person's
	// mention: CI holds no person's account, and `aayu inspect` asks an app
	// only after a failed CI review (#691). The gate skips those apps for a
	// review service, which reviews every head unasked, else a person, whose
	// `aayu inspect --approve` counts. Outside CI the order is unchanged.
	var cannotAsk []string
	if e.CI && a.cfg.Inspect.Reviewer == inspect.ReviewerCI && a.prev.PaidFailed != head {
		for _, app := range inspect.Apps {
			if slices.Contains(a.cfg.Inspect.Apps, app.Name) && app.Family != inspect.FamilyService {
				cannotAsk = append(cannotAsk, app.Name)
			}
		}
	}
	choice := inspect.ChooseReviewer(a.cfg.Inspect.Reviewer, a.cfg.Inspect.Apps, a.cfg.Inspect.AppFamilies, a.builder, a.cfg.Autonomy.Approver, cannotAsk...)
	chosen := func(c inspect.Choice) []string {
		ns := append([]string{a.builder.Note(), "reviewer: " + c.Kind() + "; the gate reads its reply and calls no model for it"}, c.Notes...)
		if a.fallback != "" {
			ns = append(ns, a.fallback)
		}
		if a.in.Mode == inspect.ModePeer {
			ns = append(ns, "peer mode needs two model reviewers; a reviewer app reviews alone, high-risk PRs included")
		}
		return ns
	}
	notes := chosen(choice)
	res := inspect.Result{PR: n, Head: head, Findings: []inspect.Finding{}, Criteria: []inspect.Verdict{}, Builder: a.builder}
	if a.in.Draft {
		res.Disposition, res.Summary = inspect.Skipped, "draft pull request: inspect runs when it is ready for review"
		return e.emitInspect(res, notes)
	}
	posts, err := prPosts(ctx, a.c, owner, name, n)
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	replyEvent := e.replyEvent()
	if a.in.Pack == nil && a.in.Story > 0 && e.CI && !replyEvent {
		return Errorf(ExitBlocked, "inspect #%d: %v", n, inspect.ErrNoPack)
	}
	bots := a.cfg.Identity.BotLogins
	human := func(login, typ string) bool { return !identity.IsBot(identity.Account{Login: login, Type: typ}, bots) }
	// The round cap (Decision #544): past MaxRounds rounds on distinct heads
	// inspect requests no review and reads no reply until a person reopens it.
	a.in.Head, a.in.At, a.in.Reopened = head, e.Now(), reopenedAt(posts, mayReopen(ctx, a.c, a.repo, bots), a.prev)
	if a.prev.Capped(head, a.in.Reopened) && !replyEvent {
		res = inspect.StepBackResult(n, head, a.prev)
		res.Builder = a.builder
		if e.CI {
			st := a.prev
			cp := res
			st.Stopped, st.Result, st.Human = head, &cp, "" // a stopped review is the capped card's, not a human wait
			if _, err := a.c.UpsertComment(ctx, a.repo, n, inspect.Marker, inspect.Comment(res, st, a.cfg.Budget.PerPRInspectMax), actions.Action{Kind: "github.comment.inspect",
				Payload: map[string]any{"disposition": res.Disposition, "reviewer": choice.Kind()}}); err != nil {
				return err
			}
		}
		// The same page the review lane's model path sends (Runner.begin): a
		// person decides whether to reopen.
		if pager, err := e.Notifier(ctx); err == nil {
			_, _ = pager.Page(ctx, notify.Page{Key: fmt.Sprintf("inspect-cap:%s#%d", a.repo, n), Subject: fmt.Sprintf("inspect: PR #%d stopped after %d review rounds", n, inspect.MaxRounds),
				Body: fmt.Sprintf("%s#%d ran %d review rounds without converging; inspect asks the builder to step back. Reopen with a `%s` comment.", a.repo, n, inspect.MaxRounds, inspect.ReopenCommand)})
		}
		if err := e.emitInspect(res, append(notes, "no review requested: the review stopped at the round cap")); err != nil {
			return err
		}
		return Errorf(ExitBlocked, "inspect #%d: %s", n, res.Disposition)
	}
	// After a reopen, a head judged before it and not since needs a new
	// request and reply: the earlier ones were the old window's round.
	var floor time.Time
	if !a.in.Reopened.IsZero() && a.prev.Judged(head, time.Time{}) && !a.prev.Judged(head, a.in.Reopened) {
		floor = a.in.Reopened
	}
	// An app someone already asked for this head, in this review window, can
	// answer: it is not skipped (Codex's P2 on #724).
	if len(cannotAsk) > 0 {
		asked := func(app string) bool {
			return slices.ContainsFunc(posts, func(p inspect.Post) bool {
				rs, ok := inspect.ParseRequest(p.Body)
				return ok && p.Kind == "comment" && rs.Head == head && rs.Reviewer == app && p.Updated.After(floor)
			})
		}
		cannotAsk = slices.DeleteFunc(cannotAsk, asked)
		if c := inspect.ChooseReviewer(a.cfg.Inspect.Reviewer, a.cfg.Inspect.Apps, a.cfg.Inspect.AppFamilies, a.builder, a.cfg.Autonomy.Approver, cannotAsk...); c.Kind() != choice.Kind() {
			// The restored app's notes replace the first choice's, such as a
			// fallback to a person (CodeRabbit's minor on #724).
			choice, notes = c, chosen(c)
		}
		if len(cannotAsk) > 0 {
			notes = append(notes, fmt.Sprintf("the gate skips %s: with the CI review declined, nothing asks an app that answers only a person's mention", strings.Join(cannotAsk, ", ")))
		}
	}
	// Each reviewer's wait starts from its own review request for the head. A
	// review service with none of its own starts from the head's first
	// request, since it reviews every head unasked; a mention-only app with
	// none (a fallback the gate cannot ask, since the apps answer only the
	// people who connected them) has no wait to run out until `aayu inspect`
	// outside CI asks it (CodeRabbit's P1 on #641).
	requested := func(c inspect.Choice) (own, first time.Time) {
		for _, p := range posts {
			rs, ok := inspect.ParseRequest(p.Body)
			if !ok || p.Kind != "comment" || rs.Head != head || !p.Updated.After(floor) {
				continue
			}
			if first.IsZero() || p.Updated.Before(first) {
				first = p.Updated
			}
			if rs.Reviewer == c.Kind() && (own.IsZero() || p.Updated.Before(own)) {
				own = p.Updated
			}
		}
		return own, first
	}
	var since time.Time
	// sinceErr is why the gate could not read when a service it turned to
	// began to wait: it then waits no longer, so a person reviews.
	var sinceErr error
	// A review service is the app's own kind, whatever family [inspect]
	// app_families gives it for the rung (CodeRabbit's major on #724).
	service := func(c inspect.Choice) bool {
		app, ok := inspect.AppByName(c.App)
		return ok && app.Family == inspect.FamilyService
	}
	// switchTo makes c the reviewer: outside CI it posts c's own request under
	// the person's account when there is none, and its wait starts now.
	switchTo := func(c inspect.Choice) error {
		choice, sinceErr = c, nil
		own, first := requested(c)
		since = own
		if service(c) {
			since = cmp.Or(own, first)
			// A service the gate turned to with no request on the head at all
			// (the CI review declined) waits from the head's first inspect run,
			// so a service that never reviews it still gives way to a person
			// (Codex's P1 on #724). When the check runs cannot be read (a
			// private repository whose inspect job lacks checks: read), the
			// wait has no start, and a person reviews rather than none.
			if since.IsZero() && e.CI {
				// The head the review stopped at waits from its reopen too: the
				// stopped run asked no one (Codex's P1 on #724).
				runs := floor
				if a.prev.Stopped == head && runs.IsZero() {
					runs = a.in.Reopened
				}
				since, sinceErr = firstInspectRun(ctx, a.c, owner, name, head, runs)
			}
		}
		if !own.IsZero() {
			return nil
		}
		if e.CI {
			if !first.IsZero() && !service(c) {
				notes = append(notes, fmt.Sprintf("%s has no review request of its own for this head, so the gate waits for it; `aayu inspect %d` outside CI asks it", c.Kind(), n))
			}
			return nil
		}
		hc, _, err := e.HumanGitHub(ctx)
		if err != nil {
			return err
		}
		marker := fmt.Sprintf(`%s{"head":%q,"reviewer":%q}`, inspect.RequestMarker, head, c.Kind())
		if _, err := hc.UpsertComment(ctx, a.repo, n, marker, inspect.Request(c, a.in, head, a.prev), actions.Action{Kind: "github.comment.review-request",
			Payload: map[string]any{"head": head, "reviewer": c.Kind()}}); err != nil {
			return Wrap(ExitExternal, err)
		}
		notes = append(notes, "review requested from "+c.Kind()+" under your account")
		since = e.Now()
		return nil
	}
	if err := switchTo(choice); err != nil {
		return err
	}
	if !e.CI {
		if note, err := e.preExistingBug(ctx, a.repo, n, head, a.prev.Result); err != nil {
			notes = append(notes, "pre-existing findings not filed: "+oneLineErr(err))
		} else if note != "" {
			notes = append(notes, note)
		}
	}
	files := inspect.ParseDiff(a.in.Diff)
	var changed []string // every changed path, deletions included: a review of the diff covers them
	for _, f := range files {
		changed = append(changed, f.Path)
	}
	replies := posts
	if !floor.IsZero() {
		replies = slices.DeleteFunc(slices.Clone(posts), func(p inspect.Post) bool { return !p.Updated.After(floor) })
	}
	// A person's review counts only when the [autonomy] approver recorded it
	// for this exact head through `aayu inspect --approve` (presence-verified,
	// on the ledger; Story #639): a comment alone, even from the approver's
	// account, may be an agent holding the person's token.
	approver := strings.TrimPrefix(strings.TrimSpace(a.cfg.Autonomy.Approver), "@")
	var approvals []headApproval
	loaded := false
	backing := func(login string) (headApproval, bool) {
		if !loaded {
			approvals, loaded = e.readApprovals(ctx), true
		}
		i := slices.IndexFunc(approvals, func(ap headApproval) bool {
			return ap.PR == n && ap.Head == head && strings.EqualFold(ap.By, approver)
		})
		if approver == "" || !strings.EqualFold(login, approver) || i < 0 {
			return headApproval{}, false
		}
		return approvals[i], true
	}
	read := func(c inspect.Choice) (inspect.AppReply, error) {
		rep, err := inspect.ReadReply(c, replies, head, since, human, a.in.Criteria, changed)
		if err == nil && c.App == "" {
			if ap, ok := backing(rep.Login); ok {
				rep.Source += fmt.Sprintf("; backed by %s's `aayu inspect --approve` of `%s` at %s, recorded on the ledger (`%s`)", ap.By, short(head), ap.At, ApprovedPath)
			} else {
				err = fmt.Errorf("%w: %s is not backed by the [autonomy] approver's `aayu inspect %d --approve` for `%s` on the ledger", inspect.ErrNoReply, rep.Source, n, short(head))
			}
		}
		if errors.Is(err, inspect.ErrNoReply) && c.App != "" && !since.IsZero() && e.Now().Sub(since) > inspect.ReplyWait {
			err = fmt.Errorf("%w: %s has not replied in the %.0f minutes since the review request", inspect.ErrUnavailable, c.App, inspect.ReplyWait.Minutes())
		}
		if errors.Is(err, inspect.ErrNoReply) && c.App != "" && sinceErr != nil {
			err = fmt.Errorf("%w: %s has not replied, and the gate cannot time its wait: %s", inspect.ErrUnavailable, c.App, oneLineErr(sinceErr))
		}
		return rep, err
	}
	rep, rerr := read(choice)
	// The chosen app cannot review this head: the next installed app in the
	// fixed order (inspect.Apps) reviews in its place, then rung B, then a person.
	for skip := slices.Clone(cannotAsk); errors.Is(rerr, inspect.ErrUnavailable) && choice.App != ""; {
		skip = append(skip, choice.App)
		next := inspect.ChooseReviewer(a.cfg.Inspect.Reviewer, a.cfg.Inspect.Apps, a.cfg.Inspect.AppFamilies, a.builder, a.cfg.Autonomy.Approver, skip...)
		if next.App == "" {
			break
		}
		notes = append(notes, fmt.Sprintf("fallback to %s: %s cannot review this head (%s)", next.App, choice.App, oneLineErr(rerr)))
		if err := switchTo(next); err != nil {
			return err
		}
		rep, rerr = read(choice)
	}
	if replyEvent {
		return e.rerunInspect(ctx, a.c, owner, name, n, head, choice, rerr)
	}
	if errors.Is(rerr, inspect.ErrUnavailable) {
		if !a.paidTried {
			return rerr // rung B steps in (runInspect)
		}
		h := inspect.ChooseReviewer(inspect.ReviewerHuman, nil, nil, a.builder, a.cfg.Autonomy.Approver)
		notes = append(notes, fmt.Sprintf("fallback to %s: %s cannot review this head (%s), and rung B ([inspect] paid_reviewer) cannot review it either", h.Kind(), choice.Kind(), oneLineErr(rerr)))
		if err := switchTo(h); err != nil {
			return err
		}
		rep, rerr = read(choice)
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
		res.Next = "the merge gate's inspect check reads this result"
	case errors.Is(rerr, inspect.ErrNoReply), errors.Is(rerr, inspect.ErrMalformed):
		res.Disposition = NoReply
		if errors.Is(rerr, inspect.ErrMalformed) {
			res.Disposition = Malformed
		}
		res.Summary = fmt.Sprintf("No usable review of `%s` from %s yet, so the gate fails closed: %v.", short(head), choice.Kind(), rerr)
		res.Rung, res.ReviewerFamily = choice.Rung, choice.Family
		res.Review = "requested: " + a.builder.RungLabel(choice.Rung, choice.Kind(), choice.Family)
		res.Next = fmt.Sprintf("the reviewer replies on this PR in the stated format. The reply runs this check again. To request a review, the builder or a person runs `aayu inspect %d` under their own account", n)
		if choice.App == "" {
			who := "the [autonomy] approver"
			if approver != "" {
				who = "@" + approver
			}
			res.Next = fmt.Sprintf("%s reviews the PR, then runs `aayu inspect %d --approve --reason \"<what you checked>\"` in their own terminal, "+
				"or approves it in `aayu inbox` with one device code for several. That person-verified step records the head on the ledger, "+
				"and a comment alone does not count. Its reply runs this check again", who, n)
			if approver == "" {
				res.Next = "set [autonomy] approver in .aayudha/config.toml. Then " + res.Next
			}
		}
		if since.IsZero() {
			notes = append(notes, "no review request for this head on the PR yet")
		}
	case rerr != nil:
		return Wrap(ExitExternal, rerr)
	default:
		var err error
		res, st, err = r.Judge(ctx, a.in, a.prev, rep)
		res.Head, res.Builder = head, a.builder
		res.Rung, res.ReviewerFamily, res.Review = reviewRung(a.builder, choice, rep)
		if err != nil {
			return Wrap(ExitExternal, err)
		}
	}
	res.Notes = append(notes, res.Notes...)
	st.Human = ""
	if rerr != nil && choice.App == "" { // no usable review from a person yet
		st.Human = head
	}
	// A cached judgement keeps its reply, but the rung follows the builder as
	// declared now: a changed label rewrites the record ship reads, as does a
	// change in what the PR description check blocks.
	relabel := e.prBodyFloor(&res, a.pr, a.prev.Result)
	if st.Result != nil && st.Result.DiffHash == res.DiffHash {
		relabel = relabel || st.Result.Review != res.Review
		// The state the comment records is what ship and audit read later.
		st.Result.Notes, st.Result.Head, st.Result.Builder = res.Notes, res.Head, res.Builder
		st.Result.Rung, st.Result.ReviewerFamily, st.Result.Review = res.Rung, res.ReviewerFamily, res.Review
		keepFloor(&st, res)
	}
	if e.CI && res.Disposition != inspect.Skipped && (!res.Cached || relabel || a.record) {
		if _, err := a.c.UpsertComment(ctx, a.repo, n, inspect.Marker, inspect.Comment(res, st, a.cfg.Budget.PerPRInspectMax), actions.Action{Kind: "github.comment.inspect",
			Payload: map[string]any{"disposition": res.Disposition, "diff_hash": res.DiffHash, "reviewer": choice.Kind(), "rung": res.Rung}}); err != nil {
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

// firstInspectRun is when the head's first aayudha/inspect check run at or
// after floor (a reopen) started. An error says why there is no such start:
// the runs cannot be read, or none started in the window (inspect's P1 on
// #724), so the wait cannot be timed and a person reviews.
func firstInspectRun(ctx context.Context, c *github.Client, owner, name, head string, floor time.Time) (time.Time, error) {
	var first time.Time
	// filter=all: by default GitHub lists only the newest run, a rerun's.
	runs, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.CheckRun, *gh.Response, error) {
		r, resp, err := c.REST.Checks.ListCheckRunsForRef(ctx, owner, name, head, &gh.ListCheckRunsOptions{CheckName: gh.Ptr("aayudha/inspect"), Filter: gh.Ptr("all"), ListOptions: o})
		return r.GetCheckRuns(), resp, err
	})
	if err != nil {
		return first, fmt.Errorf("the inspect job needs checks: read (%s)", oneLineErr(err))
	}
	for _, r := range runs {
		if at := r.GetStartedAt().Time; r.GetName() == "aayudha/inspect" && !at.IsZero() && !at.Before(floor) && (first.IsZero() || at.Before(first)) {
			first = at
		}
	}
	if first.IsZero() {
		return first, errors.New("no aayudha/inspect run on this head has started in this review window")
	}
	return first, nil
}

// replyEvent reports whether this is the merge gate's run on a reply (a PR
// comment or review), which only re-runs the head's failed inspect job.
func (e *Env) replyEvent() bool {
	ev := e.Getenv("GITHUB_EVENT_NAME")
	return e.CI && (ev == "issue_comment" || ev == "pull_request_review")
}

// preExistingBug files the pre-existing findings of res, the review of head,
// as one Bug under the person's account (filePreExisting). Every backend
// calls it outside CI: the reviewer apps with the gate's result, the review
// lane's model with its own.
func (e *Env) preExistingBug(ctx context.Context, repo string, n int, head string, res *inspect.Result) (string, error) {
	hc, _, err := e.HumanGitHub(ctx)
	if err != nil {
		return "", err
	}
	return e.filePreExisting(ctx, hc, repo, n, head, res)
}

// reopenedAt is when a person last reopened a stopped review (State.Reopen):
// of the PR comments whose first line is inspect.ReopenCommand, by an
// account may allows, the newest that found the review stopped; zero when
// none did.
func reopenedAt(posts []inspect.Post, may func(login, typ string) bool, st inspect.State) time.Time {
	var cmds []time.Time
	for _, p := range posts {
		first, _, _ := strings.Cut(strings.TrimSpace(p.Body), "\n")
		if p.Kind == "comment" && strings.TrimSpace(first) == inspect.ReopenCommand && may(p.Login, p.Type) {
			cmds = append(cmds, p.Updated)
		}
	}
	return st.Reopen(cmds)
}

// mayReopen allows a stopped review's reopen only from a person (never a
// bot) with write or admin access to the repository: comments are untrusted,
// and anyone who can comment could otherwise lift the step-back. It reads
// each commenter's permission once, and only for a reopen comment.
func mayReopen(ctx context.Context, c *github.Client, repo string, bots []string) func(login, typ string) bool {
	owner, name, _ := github.SplitRepo(repo)
	seen := map[string]bool{}
	return func(login, typ string) bool {
		if identity.IsBot(identity.Account{Login: login, Type: typ}, bots) {
			return false
		}
		if ok, done := seen[login]; done {
			return ok
		}
		p, _, err := c.REST.Repositories.GetPermissionLevel(ctx, owner, name, login)
		ok := err == nil && (permissionOf(p) == "admin" || permissionOf(p) == "write")
		seen[login] = ok
		return ok
	}
}

// reviewRung settles the review ladder's rung from the reply (Decision
// #469): the family of the model the reviewer reports, else the app's
// configured family. Only an app whose family is host-managed settles it
// from the reported model; a fixed-family app reporting another family's
// model, a reported model aayu cannot place, or one of the builder's
// candidate families, is a same-family review (C) whatever was planned; a
// person's review is human.
func reviewRung(b inspect.Builder, c inspect.Choice, rep inspect.AppReply) (string, verbs.Family, string) {
	if c.App == "" {
		return inspect.RungHuman, "", b.RungLabel(inspect.RungHuman, rep.Login, "")
	}
	f, who := c.Family, c.App
	switch {
	case rep.Model != "" && !modelID.MatchString(rep.Model):
		f, who = "", c.App+" with a model ID aayu does not accept" // reviewer text never reaches the label
	case rep.Model != "" && c.Family == inspect.FamilyService:
		who = c.App + " with " + rep.Model // a service's review is rung A whatever model it names (inspect.FamilyService)
	case rep.Model != "" && c.Family != "" && c.Family != verbs.FamilyHostManaged && verbs.FamilyOf(rep.Model) != c.Family:
		// An app of one family cannot run another's model: the claim is
		// reviewer text (a hallucination or injected PR content), not a fact.
		f, who = "", fmt.Sprintf("%s (%s) reporting %s, not of its family", c.App, c.Family, rep.Model)
	case rep.Model != "":
		f, who = verbs.FamilyOf(rep.Model), c.App+" with "+rep.Model
	}
	rung := b.Rung(f)
	return rung, f, b.RungLabel(rung, who, f)
}

// modelID is what a reported model may look like to be named in a label.
var modelID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/-]{0,63}$`)

// Notes the PR description check adds start with these.
const (
	prBodyBlocks = "PR description blocks: "
	prBodyWarns  = "PR description warns: "
)

// prBodyFloor applies the PR description check (prbody.Check) to
// res on every path that emits a result. A blocking problem on an agent PR
// (labelled agent-authored or human-fallback, or declaring Aayudha-Host)
// turns merge, or a docs-only skip, into fix, as a floor does (rule 7); a
// warning is a note. The check is deterministic and runs on the body as it is
// now: a cached result first gets back the review's own disposition
// (Reviewed). It reports whether the check's notes differ from the ones prev
// recorded, so the comment is written again. A draft is not checked.
func (e *Env) prBodyFloor(res *inspect.Result, pr *gh.PullRequest, prev *inspect.Result) bool {
	ours := func(s string) bool { return strings.HasPrefix(s, prBodyBlocks) || strings.HasPrefix(s, prBodyWarns) }
	res.Notes = slices.DeleteFunc(res.Notes, ours) // a cached result carries the last run's
	if res.Reviewed != "" {
		res.Disposition, res.Reviewed, res.Overridden = res.Reviewed, "", ""
	}
	if !pr.GetDraft() {
		// An agent's PR declares itself: by label, or by the Aayudha-Host
		// line build writes when it opens the PR (so a failed label call
		// still blocks). A bot author alone (Dependabot, a cloud agent's own
		// PR) only warns: it cannot rewrite its body.
		agent := hostTrailer.MatchString(pr.GetBody()) || slices.ContainsFunc(pr.Labels, func(l *gh.Label) bool {
			return l.GetName() == workitems.LabelAgentAuthored || l.GetName() == workitems.LabelHumanFallback
		})
		words, _ := e.steWords() // without the list, sentence length is still checked
		block, warn := prbody.Check(pr.GetBody(), words, agent)
		var why []string
		for _, p := range block {
			res.Notes = append(res.Notes, prBodyBlocks+"\""+p.Field+"\": "+p.Message)
			why = append(why, "\""+p.Field+"\"")
		}
		for _, p := range warn {
			res.Notes = append(res.Notes, prBodyWarns+"\""+p.Field+"\": "+p.Message)
		}
		if len(block) > 0 && (res.Disposition == inspect.Merge || res.Disposition == inspect.Skipped) {
			res.Overridden = fmt.Sprintf("%s downgraded to fix: the PR description is not complete in %s (the notes say why)", res.Disposition, strings.Join(slices.Compact(why), ", "))
			res.Reviewed, res.Disposition = res.Disposition, inspect.Fix
		}
	}
	var was []string
	if prev != nil {
		was = slices.DeleteFunc(slices.Clone(prev.Notes), func(s string) bool { return !ours(s) })
	}
	return !slices.Equal(was, slices.DeleteFunc(slices.Clone(res.Notes), func(s string) bool { return !ours(s) }))
}

// keepFloor records in the stored state what the PR description check
// decided, so everything that reads the state (ship, the inbox, a
// delegation) sees the same disposition as the gate.
func keepFloor(st *inspect.State, res inspect.Result) {
	if st.Result != nil {
		st.Result.Disposition, st.Result.Overridden, st.Result.Reviewed = res.Disposition, res.Overridden, res.Reviewed
	}
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
		if res.Review != "" {
			fmt.Fprintf(w, "  review: %s\n", res.Review)
		}
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
		for _, r := range gateCheckRuns(checks.CheckRuns) {
			if _, seen := state[r.GetName()]; !seen { // newest first, reply runs dropped
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
	l, err := e.Config()
	if err != nil || l.Config.Inspect.Reviewer == inspect.ReviewerModel || e.CI {
		return ""
	}
	if l.Config.Inspect.Reviewer == inspect.ReviewerCI {
		return fmt.Sprintf("the merge gate's CI review reviews PR #%d's head; nobody is asked", n)
	}
	sub := *e
	sub.JSON = true
	var out bytes.Buffer
	sub.Out, sub.Err = &out, io.Discard
	err = runInspect(ctx, &sub, Values{"pr": strconv.Itoa(n)})
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
