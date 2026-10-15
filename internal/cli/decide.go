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
	if in.Has("decision") {
		if !issueArg.MatchString(in.String("decision")) {
			return Errorf(ExitUsage, "decide: %q is not an issue; a question needs --pack", in.String("decision"))
		}
		n = issueNumber(in.String("decision"))
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
		return decideAccept(ctx, e, &l.Config, repo, n, strings.TrimSpace(in.String("accept")))
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
	if err := e.requireHuman(what); err != nil {
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

// decideAccept records a human's acceptance of an option: the Decision
// section, status:done, decision:provisional cleared (ratified or reverted),
// the issue closed, and every Decision it supersedes closed.
func decideAccept(ctx context.Context, e *Env, cfg *config.Config, repo string, n int, option string) error {
	c, login, err := humanCaller(ctx, e, cfg, "decide --accept")
	if err != nil {
		return err
	}
	owner, name, _ := github.SplitRepo(repo)
	is, _, err := c.REST.Issues.Get(ctx, owner, name, n)
	if err != nil {
		return err
	}
	d, err := loadDecision(ctx, c, repo, is)
	if err != nil {
		return err
	}
	if d.State != "open" {
		return Errorf(ExitBlocked, "decide: #%d is closed", n)
	}
	if ps := workitems.Validate(d.Item, workitems.StageReady); len(ps) > 0 {
		msgs := make([]string, len(ps))
		for i, p := range ps {
			msgs[i] = p.String()
		}
		return Errorf(ExitBlocked, "decide: #%d is not Decision-ready: %s", n, strings.Join(msgs, "; "))
	}
	key, value := decide.Authority(cfg, d)
	if want := decide.Login(cfg, value); want != "" && !strings.EqualFold(want, login) {
		return Errorf(ExitBlocked, "decide: the authority for #%d is %s (%s = %s); %s cannot accept it", n, want, key, value, login)
	}
	if !decide.MatchOption(d.Item.Fields["options"], option) {
		return Errorf(ExitUsage, "decide: %q is not one of the options of #%d", option, n)
	}
	provisional := slices.Contains(d.Labels, workitems.LabelProvisional)
	if err := setStatus(ctx, c, repo, n, d.Labels, "done"); err != nil {
		return err
	}
	body := workitems.SetSection(d.Body, "Decision", decide.Accepted(option, login, e.Now(), provisional))
	_, err = c.Write(ctx, actions.Action{Kind: "github.issue.decision-accept", Target: fmt.Sprintf("%s#%d", repo, n),
		Request: &actions.Request{Method: http.MethodPatch, Path: fmt.Sprintf("/repos/%s/issues/%d", repo, n)}, Payload: map[string]any{"option": option, "by": login}},
		func(ctx context.Context) (any, error) {
			_, _, err := c.REST.Issues.Update(ctx, owner, name, n, gh.UpdateIssueRequest{Body: gh.Ptr(body)})
			return nil, err
		})
	if err != nil {
		return err
	}
	if provisional {
		if err := removeLabel(ctx, c, repo, n, workitems.LabelProvisional); err != nil {
			return err
		}
	}
	if err := closeIssue(ctx, c, repo, n, "completed"); err != nil {
		return err
	}
	rep := decideReport{Repo: repo, Accepted: &acceptReport{Decision: n, Option: option, By: login}, DryRun: c.DryRun()}
	links, _ := workitems.ParseLinks(d.Item.Fields[workitems.FieldLinks])
	for _, lk := range links {
		if lk.Kind != "supersedes" {
			continue
		}
		old, _, err := c.REST.Issues.Get(ctx, owner, name, lk.Number)
		if err != nil {
			return err
		}
		_, err = c.UpsertComment(ctx, repo, lk.Number, decide.SupersededMarker,
			fmt.Sprintf("%s\nSuperseded by #%d, accepted by @%s (%s).\n", decide.SupersededMarker, n, login, option), actions.Action{Kind: "github.comment.decision-superseded"})
		if err != nil {
			return err
		}
		if old.GetState() == "open" {
			if err := setStatus(ctx, c, repo, lk.Number, github.IssueLabels(old), "cancelled"); err != nil {
				return err
			}
			if err := closeIssue(ctx, c, repo, lk.Number, "not_planned"); err != nil {
				return err
			}
		}
		rep.Accepted.Superseded = append(rep.Accepted.Superseded, lk.Number)
	}
	return e.Emit(rep, func(w io.Writer) error {
		fmt.Fprintf(w, "#%d accepted: %s, by @%s", n, option, login)
		if len(rep.Accepted.Superseded) > 0 {
			fmt.Fprintf(w, "; superseded %s", refsOf(rep.Accepted.Superseded))
		}
		fmt.Fprintln(w)
		return nil
	})
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
