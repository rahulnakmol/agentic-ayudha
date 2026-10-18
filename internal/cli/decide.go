package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/decide"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/identity"
	"github.com/rahulnakmol/agentic-ayudha/internal/notify"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

func init() { register("decide", runDecide) }

// decisionStep is one Decision's SLA state and what the sweep did about it.
type decisionStep struct {
	decide.Step
	Title       string   `json:"title"`
	Authority   string   `json:"authority"`
	Login       string   `json:"authority_login,omitempty"`
	Reversible  bool     `json:"reversible"`
	Waiver      bool     `json:"waiver,omitempty"`
	Provisional bool     `json:"provisional"`
	RatifyBy    string   `json:"ratify_by,omitempty"`
	Constrains  []int    `json:"constrains,omitempty"` // items shown blocked while it is open
	Did         []string `json:"did"`
}

type decideReport struct {
	Repo      string         `json:"repo"`
	Decisions []decisionStep `json:"decisions,omitempty"`
	Accepted  *acceptReport  `json:"accepted,omitempty"`
	Batch     []acceptReport `json:"batch,omitempty"` // every Decision of a batch (decide a,b --accept x,y), Accepted first
	Waiver    *waiverReport  `json:"waiver,omitempty"`
	DryRun    bool           `json:"dry_run,omitempty"`
}

type acceptReport struct {
	Decision   int    `json:"decision"`
	Option     string `json:"option"`
	By         string `json:"by"`
	Superseded []int  `json:"superseded,omitempty"`
}

type waiverReport struct {
	Decision int    `json:"decision"`
	Floor    string `json:"floor"`
	Item     int    `json:"item"`
	Existing bool   `json:"existing,omitempty"`
}

// runDecide is `aayu decide`: without flags it runs the SLA sweep (WP5.6, V8)
// over one open Decision or all of them; --accept records a human's
// acceptance; --waiver files a floor-waiver Decision on a story or bug;
// --pack (read-only, on demand) grounds a decision pack for a seat twin to
// work with the human (WP5.1b).
func runDecide(ctx context.Context, e *Env, in Values) error {
	repo, err := e.RepoName(ctx)
	if err != nil {
		return err
	}
	l, err := e.Config()
	if err != nil {
		return err
	}
	pol, err := decide.NewPolicy(&l.Config)
	if err != nil {
		return Wrap(ExitUsage, err)
	}
	if in.Bool("pack") {
		if in.Has("accept") || in.Has("waiver") {
			return Errorf(ExitBlocked, "decide --pack is read-only: it does not combine with --accept or --waiver")
		}
		return decidePack(ctx, e, &l.Config, repo, in.String("decision"), in.String("seat"), in.String("draft"))
	}
	if in.Has("seat") || in.Has("draft") {
		return Errorf(ExitUsage, "decide: --seat and --draft need --pack")
	}
	n := 0
	var nums []int // a batch: decide 324,283 --accept 1,2
	if in.Has("decision") {
		for _, part := range strings.Split(in.String("decision"), ",") {
			if !issueArg.MatchString(strings.TrimSpace(part)) {
				return Errorf(ExitUsage, "decide: %q is not an issue; a question needs --pack", in.String("decision"))
			}
			nums = append(nums, issueNumber(strings.TrimSpace(part)))
		}
		n = nums[0]
		if len(nums) > 1 && !in.Has("accept") {
			return Errorf(ExitUsage, "decide: a list of Decisions is a batch for --accept (aayu decide 324,283 --accept 1,2)")
		}
	}
	switch {
	case in.Has("accept") && in.Has("waiver"):
		return Errorf(ExitUsage, "decide: --accept and --waiver do not combine")
	case in.Has("waiver"):
		if n == 0 {
			return Errorf(ExitUsage, "decide --waiver <floor> needs the story or bug the waiver is for")
		}
		return decideWaiver(ctx, e, repo, n, strings.TrimSpace(in.String("waiver")))
	case in.Has("accept"):
		if n == 0 {
			return Errorf(ExitUsage, "decide --accept <option> needs the Decision")
		}
		options := []string{strings.TrimSpace(in.String("accept"))}
		if len(nums) > 1 {
			options = strings.Split(in.String("accept"), ",")
			for i := range options {
				options[i] = strings.TrimSpace(options[i])
			}
		}
		return decideAccept(ctx, e, &l.Config, repo, nums, options)
	}
	return decideSweep(ctx, e, &l.Config, pol, repo, n)
}

// loadDecision reads a Decision issue with its parent's type.
func loadDecision(ctx context.Context, c *github.Client, repo string, is *gh.Issue) (decide.Decision, error) {
	owner, name, _ := github.SplitRepo(repo)
	labels := github.IssueLabels(is)
	it, _ := workitems.Parse(is.GetNumber(), is.GetTitle(), is.GetBody(), is.GetType().GetName(), labels)
	d := decide.Decision{Number: is.GetNumber(), Title: is.GetTitle(), Body: is.GetBody(), State: is.GetState(), Labels: labels, Created: is.GetCreatedAt().Time, Item: it}
	if it.Type != "decision" {
		return d, Errorf(ExitUsage, "decide: #%d is a %q, not a Decision", d.Number, it.Type)
	}
	parent, _, err := c.REST.SubIssue.GetParentIssue(ctx, owner, name, int64(d.Number))
	switch {
	case err == nil:
		d.ParentType, _ = workitems.TypeOf(parent.GetType().GetName(), github.IssueLabels(parent))
	case !github.NotFound(err):
		return d, err
	}
	return d, nil
}

func constrains(d decide.Decision) []int {
	links, _ := workitems.ParseLinks(d.Item.Fields[workitems.FieldLinks])
	var out []int
	for _, l := range links {
		if l.Kind == "constrains" {
			out = append(out, l.Number)
		}
	}
	return out
}

// decideSweep applies the SLA rules to Decision n, or to every open Decision when n is 0.
func decideSweep(ctx context.Context, e *Env, cfg *config.Config, pol decide.Policy, repo string, n int) error {
	c, bot, err := e.BotGitHub(ctx, "")
	if err != nil {
		return err
	}
	owner, name, _ := github.SplitRepo(repo)
	var issues []*gh.Issue
	if n != 0 {
		is, _, err := c.REST.Issues.Get(ctx, owner, name, n)
		if err != nil {
			return err
		}
		issues = []*gh.Issue{is}
	} else {
		all, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.Issue, *gh.Response, error) {
			return c.REST.Issues.ListByRepo(ctx, owner, name, &gh.IssueListByRepoOptions{State: "open", Labels: []string{"type:decision"}, ListOptions: o})
		})
		if err != nil {
			return err
		}
		for _, is := range all {
			if !is.IsPullRequest() {
				issues = append(issues, is)
			}
		}
	}
	var sender *notify.Sender
	var sink notify.Sink
	now := e.Now()
	rep := decideReport{Repo: repo, Decisions: []decisionStep{}, DryRun: c.DryRun()}
	for _, is := range issues {
		d, err := loadDecision(ctx, c, repo, is)
		if err != nil {
			return err
		}
		key, value := decide.Authority(cfg, d)
		st := decisionStep{Step: pol.Evaluate(d, now), Title: d.Title, Authority: key + " = " + value, Login: decide.Login(cfg, value),
			Reversible: d.Reversible(), Waiver: d.Waiver(), Provisional: slices.Contains(d.Labels, workitems.LabelProvisional), Constrains: constrains(d), Did: []string{}}
		if t, ok := decide.RatifyBy(d.Body); ok {
			st.RatifyBy = t.UTC().Format(time.RFC3339)
		}
		if st.Lapsed {
			if sender == nil {
				if sender, err = e.Notifier(ctx); err != nil {
					return err
				}
				sink = sender.Sink
			}
			sender.Sink = sink
			if gm, ok := sink.(notify.GitHubMention); ok && st.Login != "" {
				gm.Login = st.Login // page the authority itself on the GitHub channel
				sender.Sink = gm
			}
			page := notify.Page{Key: fmt.Sprintf("decision-sla:%s#%d", repo, d.Number), Subject: fmt.Sprintf("Decision #%d is past its SLA: %s", d.Number, d.Title),
				Body: fmt.Sprintf("%s#%d has not been accepted within %g business hours. Authority: %s. Accept it with `aayu decide %d --accept <option>`.", repo, d.Number, pol.SLA.Hours(), st.Authority, d.Number)}
			if st.Overdue {
				page.Key = fmt.Sprintf("decision-overdue:%s#%d", repo, d.Number)
				page.Subject = fmt.Sprintf("Decision #%d is overdue: %s", d.Number, d.Title)
			}
			sent, err := sender.Page(ctx, page)
			if err != nil {
				return Wrap(ExitExternal, err)
			}
			st.Did = append(st.Did, "page "+page.Key+": "+sent)
			wrote, err := c.UpsertComment(ctx, repo, d.Number, decide.EscalationMarker, decide.EscalationComment(d, pol, st.Authority, st.Login),
				actions.Action{Kind: "github.comment.decision-escalation", Payload: map[string]any{"tag": bot.Tag, "escalate_to": pol.EscalateTo}})
			if err != nil {
				return err
			}
			if wrote {
				st.Did = append(st.Did, "escalation comment")
			}
		}
		if st.Apply {
			if err := applyProvisional(ctx, c, repo, d, pol, st.ProvisionAt); err != nil {
				return err
			}
			st.Provisional = true
			st.RatifyBy = pol.RatifyBy(st.ProvisionAt).UTC().Format(time.RFC3339)
			st.Did = append(st.Did, "applied provisionally ("+workitems.LabelProvisional+")")
		}
		rep.Decisions = append(rep.Decisions, st)
	}
	return e.Emit(rep, func(w io.Writer) error {
		if len(rep.Decisions) == 0 {
			_, err := fmt.Fprintf(w, "%s: no open Decisions\n", repo)
			return err
		}
		for _, s := range rep.Decisions {
			state := fmt.Sprintf("SLA due %s", s.SLA.In(pol.Clock.Loc).Format("2006-01-02 15:04 MST"))
			switch {
			case s.Provisional:
				state = "provisional; ratify or revert by " + s.RatifyBy
			case s.Overdue:
				state = "overdue (" + s.Reason + ")"
			case s.Lapsed:
				state = "past its SLA"
			}
			fmt.Fprintf(w, "#%d %q: %s; authority %s", s.Number, s.Title, state, s.Authority)
			if len(s.Constrains) > 0 {
				fmt.Fprintf(w, "; blocks %s while open", refsOf(s.Constrains))
			}
			if len(s.Did) > 0 {
				fmt.Fprintf(w, "; did: %s", strings.Join(s.Did, ", "))
			}
			fmt.Fprintln(w)
		}
		return nil
	})
}

func refsOf(ns []int) string {
	out := make([]string, len(ns))
	for i, n := range ns {
		out[i] = fmt.Sprintf("#%d", n)
	}
	return strings.Join(out, " ")
}

// applyProvisional moves the Decision off status:ready (so the Ready guard
// does not strip it for the body edit), writes the provisional text and its
// ratify-or-revert deadline into the Decision section, then labels it
// decision:provisional. The label goes last, so an interrupted run is redone,
// and the text depends only on the deadline, so a redo writes the same body.
func applyProvisional(ctx context.Context, c *github.Client, repo string, d decide.Decision, pol decide.Policy, at time.Time) error {
	owner, name, _ := github.SplitRepo(repo)
	if err := setStatus(ctx, c, repo, d.Number, d.Labels, "validation"); err != nil {
		return err
	}
	body := workitems.SetSection(d.Body, "Decision", decide.ProvisionalText(d, pol, at))
	if body != d.Body {
		_, err := c.Write(ctx, actions.Action{Kind: "github.issue.decision-provisional", Target: fmt.Sprintf("%s#%d", repo, d.Number),
			Request: &actions.Request{Method: http.MethodPatch, Path: fmt.Sprintf("/repos/%s/issues/%d", repo, d.Number)}, Payload: map[string]any{"ratify_by": pol.RatifyBy(at).UTC().Format(time.RFC3339)}},
			func(ctx context.Context) (any, error) {
				_, _, err := c.REST.Issues.Update(ctx, owner, name, d.Number, gh.UpdateIssueRequest{Body: gh.Ptr(body)})
				return nil, err
			})
		if err != nil {
			return err
		}
	}
	return addLabel(ctx, c, repo, d.Number, workitems.LabelProvisional)
}

func addLabel(ctx context.Context, c *github.Client, repo string, n int, label string) error {
	owner, name, _ := github.SplitRepo(repo)
	_, err := c.Write(ctx, actions.Action{Kind: "github.issue.labels.add", Target: fmt.Sprintf("%s#%d", repo, n),
		Request: &actions.Request{Method: http.MethodPost, Path: fmt.Sprintf("/repos/%s/issues/%d/labels", repo, n)}, Payload: map[string]any{"labels": []string{label}}},
		func(ctx context.Context) (any, error) {
			_, _, err := c.REST.Issues.AddLabelsToIssue(ctx, owner, name, n, []string{label})
			return nil, err
		})
	return err
}

func removeLabel(ctx context.Context, c *github.Client, repo string, n int, label string) error {
	owner, name, _ := github.SplitRepo(repo)
	_, err := c.Write(ctx, actions.Action{Kind: "github.issue.labels.remove", Target: fmt.Sprintf("%s#%d", repo, n),
		Request: &actions.Request{Method: http.MethodDelete, Path: fmt.Sprintf("/repos/%s/issues/%d/labels/%s", repo, n, label)}, Payload: map[string]any{"label": label}},
		func(ctx context.Context) (any, error) {
			_, err := c.REST.Issues.RemoveLabelForIssue(ctx, owner, name, n, label)
			if github.NotFound(err) {
				err = nil
			}
			return nil, err
		})
	return err
}

func closeIssue(ctx context.Context, c *github.Client, repo string, n int, reason string) error {
	owner, name, _ := github.SplitRepo(repo)
	_, err := c.Write(ctx, actions.Action{Kind: "github.issue.close", Target: fmt.Sprintf("%s#%d", repo, n),
		Request: &actions.Request{Method: http.MethodPatch, Path: fmt.Sprintf("/repos/%s/issues/%d", repo, n)}, Payload: map[string]any{"reason": reason}},
		func(ctx context.Context) (any, error) {
			_, _, err := c.REST.Issues.Update(ctx, owner, name, n, gh.UpdateIssueRequest{State: gh.Ptr("closed"), StateReason: gh.Ptr(reason)})
			return nil, err
		})
	return err
}

// humanCaller resolves the human running a human-only action and refuses
// agents and bot identities (trust model §0.1).
func humanCaller(ctx context.Context, e *Env, cfg *config.Config, what string) (*github.Client, string, error) {
	if err := e.requireHuman(ctx, what, AccessWrite); err != nil {
		return nil, "", err
	}
	c, tok, err := e.HumanGitHub(ctx)
	if err != nil {
		return nil, "", err
	}
	u, err := identity.WhoAmI(ctx, e.HTTP, e.githubAPI(), tok)
	if err != nil {
		return nil, "", Wrap(ExitExternal, err)
	}
	if identity.IsBot(identity.Account{Login: u.Login, Type: u.Type}, cfg.Identity.BotLogins) {
		return nil, "", Errorf(ExitBlocked, "%s: %s is a bot identity; only a human may do this", what, u.Login)
	}
	return c, u.Login, nil
}

// acceptable checks that login may accept option of the open, Decision-ready
// Decision d: the checks decide --accept makes before it writes anything.
func acceptable(cfg *config.Config, d decide.Decision, option, login string) error {
	if d.State != "open" {
		return Errorf(ExitBlocked, "decide: #%d is closed", d.Number)
	}
	if ps := workitems.Validate(d.Item, workitems.StageReady); len(ps) > 0 {
		msgs := make([]string, len(ps))
		for i, p := range ps {
			msgs[i] = p.String()
		}
		return Errorf(ExitBlocked, "decide: #%d is not Decision-ready: %s", d.Number, strings.Join(msgs, "; "))
	}
	key, value := decide.Authority(cfg, d)
	if want := decide.Login(cfg, value); want != "" && !strings.EqualFold(want, login) {
		return Errorf(ExitBlocked, "decide: the authority for #%d is %s (%s = %s); %s cannot accept it", d.Number, want, key, value, login)
	}
	if !decide.MatchOption(d.Item.Fields["options"], option) {
		return Errorf(ExitUsage, "decide: %q is not one of the options of #%d", option, d.Number)
	}
	return nil
}

// decideAccept records a person's acceptance of one option for each of the
// Decisions in nums, as one batch (Story #481): every Decision is checked
// before anything is written. A person at their own terminal accepts by
// running it. Anywhere else the batch goes to the person as one comment on
// the first Decision: approved by an approver's reply when comment approvals
// are on (approve.go; the command then exits 4 and accepts nothing itself),
// else by one device code that covers every item.
func decideAccept(ctx context.Context, e *Env, cfg *config.Config, repo string, nums []int, options []string) error {
	if len(nums) != len(options) {
		return Errorf(ExitUsage, "decide --accept: %d Decision(s) but %d option(s); give one option per Decision, in order (aayu decide 324,283 --accept 1,2)", len(nums), len(options))
	}
	if len(nums) > batchMax {
		return Errorf(ExitUsage, "decide --accept: at most %d Decisions in one batch", batchMax)
	}
	c, tok, err := e.HumanGitHub(ctx)
	if err != nil {
		return err
	}
	u, err := identity.WhoAmI(ctx, e.HTTP, e.githubAPI(), tok)
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	login := u.Login
	if identity.IsBot(identity.Account{Login: u.Login, Type: u.Type}, cfg.Identity.BotLogins) {
		return Errorf(ExitBlocked, "decide --accept: %s is a bot identity; only a human may do this", u.Login)
	}
	owner, name, _ := github.SplitRepo(repo)
	ds := make([]decide.Decision, len(nums))
	b := &batch{Repo: repo, Issue: nums[0], Command: e.commandLine()}
	for i, n := range nums {
		if slices.Contains(nums[:i], n) {
			return Errorf(ExitUsage, "decide --accept: #%d is in the batch twice", n)
		}
		is, _, err := c.REST.Issues.Get(ctx, owner, name, n)
		if err != nil {
			return err
		}
		if ds[i], err = loadDecision(ctx, c, repo, is); err != nil {
			return err
		}
		if err := acceptable(cfg, ds[i], options[i], login); err != nil {
			return err
		}
		b.Items = append(b.Items, batchDecision(ds[i], options[i]))
	}
	via, writer := "", c
	if e.agentSignal() != "" || !e.isTerminal(e.In) {
		if bc, bot, err := e.BotGitHub(ctx, ""); err == nil {
			writer = bc
			if why := e.commentApprovalsOff(ctx, cfg, bot, b.Items); why == "" {
				return e.postCommentBatch(ctx, writer, cfg, b)
			} else {
				b.Why = why
			}
		} else {
			b.Why = fmt.Sprintf("the bot identity is not available (%v).", err)
		}
		if err := e.deviceBatch(ctx, writer, b, login); err != nil {
			return err
		}
		via = "approved through GitHub's device flow"
	}
	rep := decideReport{Repo: repo, DryRun: c.DryRun()}
	for i, d := range ds {
		ar, err := acceptDecision(ctx, e, c, repo, d, options[i], login, via)
		if err != nil {
			return err
		}
		if via != "" {
			line := fmt.Sprintf("#%d accepted: option %s, by @%s through the device code", d.Number, options[i], login)
			if _, err := writer.UpsertComment(ctx, repo, d.Number, fmt.Sprintf("<!-- aayudha:batch-accepted %s -->", b.hash()),
				fmt.Sprintf("<!-- aayudha:batch-accepted %s -->\n%s (batch `%s`).\n", b.hash(), line, b.hash()), actions.Action{Kind: "github.comment.batch-result"}); err != nil {
				return err
			}
		}
		if i == 0 {
			rep.Accepted = &ar
		}
		rep.Batch = append(rep.Batch, ar)
	}
	if len(rep.Batch) == 1 {
		rep.Batch = nil
	}
	return e.Emit(rep, func(w io.Writer) error {
		for _, a := range append([]acceptReport{*rep.Accepted}, rep.Batch[min(1, len(rep.Batch)):]...) {
			fmt.Fprintf(w, "#%d accepted: %s, by @%s", a.Decision, a.Option, a.By)
			if via != "" {
				fmt.Fprint(w, " through the device code")
			}
			if len(a.Superseded) > 0 {
				fmt.Fprintf(w, "; superseded %s", refsOf(a.Superseded))
			}
			fmt.Fprintln(w)
		}
		return nil
	})
}

// postCommentBatch posts b for an approver's reply and accepts nothing: the
// aayudha-approve workflow applies the reply (approve.go).
func (e *Env) postCommentBatch(ctx context.Context, c *github.Client, cfg *config.Config, b *batch) error {
	b.Mode, b.Mention = batchByComment, cfg.Approvals.Approvers
	if _, bot, err := e.BotGitHub(ctx, ""); err == nil {
		if open, _, shown, err := openBatch(ctx, c, b.Repo, b.Issue, bot.Login, e.Now()); err != nil {
			return Wrap(ExitExternal, err)
		} else if open != nil {
			return WithNext(Errorf(ExitBlocked, "batch %s is still open on #%d; one batch at a time per issue, so an approver never has to name it", shown, b.Issue),
				fmt.Sprintf("wait for the approver's reply on #%d, or start the batch on another Decision", b.Issue))
		}
	}
	if _, err := c.UpsertComment(ctx, b.Repo, b.Issue, b.marker(), b.render(batchStatus{}), actions.Action{Kind: "github.comment.batch"}); err != nil {
		return Wrap(ExitExternal, err)
	}
	return WithNext(Errorf(ExitBlocked, "posted batch %s on #%d for %s; nothing is accepted until an approver replies `/aayu approve all` (or `approve 1 3`, `skip 2`) there", b.hash(), b.Issue, strings.Join(b.Mention, ", ")),
		fmt.Sprintf("wait for the approver's reply on #%d; the aayudha-approve workflow applies it", b.Issue))
}

// deviceBatch asks for one device code that covers every item of b. The
// batch comment carries the code to the person's phone and, when no
// approval comes, says what happened and the one next step.
func (e *Env) deviceBatch(ctx context.Context, c *github.Client, b *batch, login string) error {
	b.Mode, b.Mention = batchByDevice, []string{login}
	effects := make([]string, len(b.Items))
	for i, it := range b.Items {
		effects[i] = it.Effect + " " + it.Door
	}
	p := Pending{What: "decide --accept", Effect: effects,
		Code: func(ctx context.Context, code deviceCodeReply, expires time.Time) error {
			if _, err := c.UpsertComment(ctx, b.Repo, b.Issue, b.marker(), b.render(batchStatus{Code: &code, Expires: expires}), actions.Action{Kind: "github.comment.batch"}); err != nil {
				return Wrap(ExitExternal, fmt.Errorf("posting the batch on #%d: %w", b.Issue, err))
			}
			fmt.Fprintf(e.Err, "Posted batch %s on #%d with the code.\n", b.hash(), b.Issue)
			return nil
		},
		Failed: func(ctx context.Context, what, next string) {
			_, _ = c.UpsertComment(ctx, b.Repo, b.Issue, b.marker(), b.render(batchStatus{Done: what, Next: next}), actions.Action{Kind: "github.comment.batch"})
		},
	}
	if err := e.requireApproval(ctx, p, AccessWrite); err != nil {
		return err
	}
	if e.approval != nil {
		done := fmt.Sprintf("Approved by @%s through the device code at %s.", e.approval.Login, e.Now().UTC().Format("15:04 UTC"))
		if _, err := c.UpsertComment(ctx, b.Repo, b.Issue, b.marker(), b.render(batchStatus{Done: done}), actions.Action{Kind: "github.comment.batch"}); err != nil {
			return Wrap(ExitExternal, err)
		}
	}
	return nil
}

// acceptDecision writes the acceptance of option on Decision d, by login:
// the Decision section (with how it was approved, when via says), status:done,
// decision:provisional cleared (ratified or reverted), the issue closed, and
// every Decision it supersedes closed.
func acceptDecision(ctx context.Context, e *Env, c *github.Client, repo string, d decide.Decision, option, login, via string) (acceptReport, error) {
	owner, name, _ := github.SplitRepo(repo)
	n := d.Number
	provisional := slices.Contains(d.Labels, workitems.LabelProvisional)
	if err := setStatus(ctx, c, repo, n, d.Labels, "done"); err != nil {
		return acceptReport{}, err
	}
	text := decide.Accepted(option, login, e.Now(), provisional)
	if via != "" {
		text += " " + strings.ToUpper(via[:1]) + via[1:] + "."
	}
	body := workitems.SetSection(d.Body, "Decision", text)
	_, err := c.Write(ctx, actions.Action{Kind: "github.issue.decision-accept", Target: fmt.Sprintf("%s#%d", repo, n),
		Request: &actions.Request{Method: http.MethodPatch, Path: fmt.Sprintf("/repos/%s/issues/%d", repo, n)}, Payload: map[string]any{"option": option, "by": login, "via": via}},
		func(ctx context.Context) (any, error) {
			_, _, err := c.REST.Issues.Update(ctx, owner, name, n, gh.UpdateIssueRequest{Body: gh.Ptr(body)})
			return nil, err
		})
	if err != nil {
		return acceptReport{}, err
	}
	if provisional {
		if err := removeLabel(ctx, c, repo, n, workitems.LabelProvisional); err != nil {
			return acceptReport{}, err
		}
	}
	if err := closeIssue(ctx, c, repo, n, "completed"); err != nil {
		return acceptReport{}, err
	}
	ar := acceptReport{Decision: n, Option: option, By: login}
	links, _ := workitems.ParseLinks(d.Item.Fields[workitems.FieldLinks])
	for _, lk := range links {
		if lk.Kind != "supersedes" {
			continue
		}
		old, _, err := c.REST.Issues.Get(ctx, owner, name, lk.Number)
		if err != nil {
			return ar, err
		}
		_, err = c.UpsertComment(ctx, repo, lk.Number, decide.SupersededMarker,
			fmt.Sprintf("%s\nSuperseded by #%d, accepted by @%s (%s).\n", decide.SupersededMarker, n, login, option), actions.Action{Kind: "github.comment.decision-superseded"})
		if err != nil {
			return ar, err
		}
		if old.GetState() == "open" {
			if err := setStatus(ctx, c, repo, lk.Number, github.IssueLabels(old), "cancelled"); err != nil {
				return ar, err
			}
			if err := closeIssue(ctx, c, repo, lk.Number, "not_planned"); err != nil {
				return ar, err
			}
		}
		ar.Superseded = append(ar.Superseded, lk.Number)
	}
	return ar, nil
}

// decideWaiver files a floor-waiver Decision on story or bug n: never
// reversible, labelled decision:waiver, a sub-issue of n. A human accepts it
// with --accept waive; nothing waives a floor before that.
func decideWaiver(ctx context.Context, e *Env, repo string, n int, floor string) error {
	if floor == "redaction" {
		return Errorf(ExitBlocked, "decide --waiver: the redaction floor is never waivable: a capture holding personal data does not land (V3)")
	}
	if !slices.Contains(decide.Waivable, floor) {
		return Errorf(ExitUsage, "decide --waiver %q: want one of %s", floor, strings.Join(decide.Waivable, ", "))
	}
	c, _, err := e.HumanGitHub(ctx)
	if err != nil {
		return err
	}
	owner, name, _ := github.SplitRepo(repo)
	is, it, err := issueItem(ctx, c, repo, n)
	if err != nil {
		return err
	}
	if it.Type != "story" && it.Type != "bug" {
		return Errorf(ExitUsage, "decide --waiver: #%d is a %q; a floor waiver is for a story or bug", n, it.Type)
	}
	if is.GetState() != "open" {
		return Errorf(ExitBlocked, "decide --waiver: #%d is closed", n)
	}
	title, fields := decide.WaiverItem(floor, n, "")
	subs, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.SubIssue, *gh.Response, error) {
		return c.REST.SubIssue.ListByIssue(ctx, owner, name, int64(n), &o)
	})
	if err != nil {
		return err
	}
	rep := decideReport{Repo: repo, Waiver: &waiverReport{Floor: floor, Item: n}, DryRun: c.DryRun()}
	for _, s := range subs {
		si := (*gh.Issue)(s)
		if si.GetTitle() == title && slices.Contains(github.IssueLabels(si), workitems.LabelWaiver) && si.GetState() == "open" {
			rep.Waiver.Decision, rep.Waiver.Existing = si.GetNumber(), true
			return emitWaiver(e, rep)
		}
	}
	t, _ := workitems.FindType("decision")
	body := workitems.Render(t, fields)
	labels := []string{"type:decision", "status:planned", workitems.LabelWaiver}
	var created *gh.Issue
	_, err = c.Write(ctx, actions.Action{Kind: "github.issue.create", Target: repo,
		Request: &actions.Request{Method: http.MethodPost, Path: fmt.Sprintf("/repos/%s/issues", repo)}, Payload: map[string]any{"title": title, "waiver": floor, "item": n}},
		func(ctx context.Context) (any, error) {
			is, _, err := c.REST.Issues.Create(ctx, owner, name, gh.CreateIssueRequest{Title: title, Body: gh.Ptr(body), Labels: labels})
			created = is
			return map[string]any{"number": is.GetNumber()}, err
		})
	if err != nil {
		return err
	}
	if created != nil {
		rep.Waiver.Decision = created.GetNumber()
		_, err = c.Write(ctx, actions.Action{Kind: "github.sub-issue.add", Target: fmt.Sprintf("%s#%d", repo, n),
			Request: &actions.Request{Method: http.MethodPost, Path: fmt.Sprintf("/repos/%s/issues/%d/sub_issues", repo, n)}, Payload: map[string]any{"sub_issue": rep.Waiver.Decision}},
			func(ctx context.Context) (any, error) {
				_, _, err := c.REST.SubIssue.Add(ctx, owner, name, int64(n), gh.SubIssueRequest{SubIssueID: created.GetID()})
				return nil, err
			})
		if err != nil {
			return err
		}
	}
	return emitWaiver(e, rep)
}

func emitWaiver(e *Env, rep decideReport) error {
	return e.Emit(rep, func(w io.Writer) error {
		switch {
		case rep.DryRun:
			fmt.Fprintf(w, "floor waiver for %s on #%d planned (dry run)\n", rep.Waiver.Floor, rep.Waiver.Item)
		case rep.Waiver.Existing:
			fmt.Fprintf(w, "floor waiver #%d for %s on #%d is already open\n", rep.Waiver.Decision, rep.Waiver.Floor, rep.Waiver.Item)
		default:
			fmt.Fprintf(w, "floor waiver #%d for %s on #%d filed; a human accepts it with `aayu decide %d --accept waive`\n", rep.Waiver.Decision, rep.Waiver.Floor, rep.Waiver.Item, rep.Waiver.Decision)
		}
		return nil
	})
}
