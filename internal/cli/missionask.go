package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/ready"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

// A mission's ask (Story #1001): a step only a person takes, which the
// mission posts on its Feature as one comment and `aayu inbox` shows as one
// card. The gate asks after [mission] max_no_go failed runs of a milestone,
// and a fix PR that would pass the Feature's budget asks too. A later
// comment with the closed marker and the ask's ID closes it.

// Markers of a mission ask comment and of the comment that closes it.
const (
	MissionAskMarker   = "<!-- aayudha:mission-ask "
	MissionAskDoneMark = "<!-- aayudha:mission-ask-closed "
)

// missionAsk is one ask: its ID on the Feature, the card's title, why, its
// facts one per line, and the person's options. It gives no recommendation.
type missionAsk struct {
	ID      string        `json:"id"`
	Title   string        `json:"title"`
	Why     string        `json:"why"`
	Lines   []string      `json:"lines"`
	Options []InboxAction `json:"options"`
}

func noGoAskID(m string) string  { return "gate-" + m }
func budgetAskID(pr int) string  { return "budget-" + strconv.Itoa(pr) }
func askClosed(id string) string { return MissionAskDoneMark + id + " -->" }
func missionCmd(f int, s string) string {
	return fmt.Sprintf("aayu mission %s", strings.Replace(s, "<f>", strconv.Itoa(f), 1))
}

// noGoAsk is the ask after milestone m failed its gate len(fails) times: each
// run with its checks and time, and no recommendation.
func noGoAsk(f int, m string, fails []GateRun) missionAsk {
	a := missionAsk{ID: noGoAskID(m), Title: noGoTitle(m, f, len(fails)),
		Why: fmt.Sprintf("The gate of %s failed %d times, so the mission stopped and asks you.", m, len(fails)),
		Options: []InboxAction{
			{N: 1, Label: "Run the gate once more", Command: missionCmd(f, "gate <f> --once-more")},
			{N: 2, Label: "Stop the mission, with a reason", Command: missionCmd(f, "stop <f> --reason"), Ask: "Why stop the mission?"},
		}}
	for i, r := range fails {
		a.Lines = append(a.Lines, fmt.Sprintf("Run %d at %s on dev %s: %s failed", i+1, r.At, short(r.DevSHA), failedOf(r.Checks)))
	}
	return a
}

// budgetAsk is the ask of fix PR n, which would pass the budget: the spend,
// the fix's estimate, and the least raise that lets it go on, rounded up to
// the next 10 USD.
func budgetAsk(f, n int, budget float64, used SourcedNumber, est float64, fixes []GateFix, costs map[int]float64) missionAsk {
	raise := math.Ceil((used.Value+est)/10) * 10
	a := missionAsk{ID: budgetAskID(n), Title: fmt.Sprintf("Fix PR #%d would pass #%d's budget", n, f),
		Why: fmt.Sprintf("#%d used %s of its %s budget, so %s is left, and fix PR #%d is estimated at %s.", f, ready.USD(used.Value), ready.USD(budget),
			ready.USD(budget-used.Value), n, ready.USD(est)),
		Lines: []string{fmt.Sprintf("Used %s: %s", ready.USD(used.Value), used.Source),
			fmt.Sprintf("Estimate of #%d: %s, the per-story cap, until #872 measures spend", n, ready.USD(est))},
		Options: []InboxAction{
			{N: 1, Label: fmt.Sprintf("Raise the budget to %s, with a reason", ready.USD(raise)), Command: missionCmd(f, fmt.Sprintf("budget <f> --to %d --reason", int(raise))),
				Ask: "Why raise the budget?"},
			{N: 2, Label: fmt.Sprintf("Keep #%d waiting", n), Command: missionCmd(f, fmt.Sprintf("budget <f> --keep %d", n))},
		}}
	for _, fx := range gateFixPRs(fixes) {
		if v := costs[fx.PR]; v > 0 {
			a.Lines = append(a.Lines, fmt.Sprintf("Fix PR #%d (%s %s) spent %s", fx.PR, fx.Milestone, fx.Check, ready.USD(v)))
		}
	}
	return a
}

// postAsk posts ask on Feature f, unless an open ask of its ID says the same.
func (e *Env) postAsk(ctx context.Context, c *github.Client, repo string, f int, a missionAsk) error {
	owner, name, _ := github.SplitRepo(repo)
	open, err := openAsks(ctx, c, owner, name, f, e.askTrusted(ctx, c, owner, name))
	if err != nil {
		return err
	}
	data, _ := json.Marshal(a)
	for _, o := range open {
		if b, _ := json.Marshal(o); string(b) == string(data) {
			return nil
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s%s -->\n**%s**\n\n%s\n\n", MissionAskMarker, data, a.Title, a.Why)
	for _, l := range a.Lines {
		fmt.Fprintf(&b, "- %s\n", l)
	}
	b.WriteString("\nIn `aayu inbox`, a person picks one:\n\n")
	for _, o := range a.Options {
		fmt.Fprintf(&b, "%d. %s: `%s`\n", o.N, o.Label, o.Command)
	}
	return wrapExternal(postComment(ctx, c, repo, f, "github.comment.mission-ask", b.String()))
}

// closeAsk closes Feature f's open ask id with a note, when one is open.
func (e *Env) closeAsk(ctx context.Context, c *github.Client, repo string, f int, id, note string) error {
	owner, name, _ := github.SplitRepo(repo)
	open, err := openAsks(ctx, c, owner, name, f, e.askTrusted(ctx, c, owner, name))
	if err != nil {
		return err
	}
	for _, o := range open {
		if o.ID == id {
			return wrapExternal(postComment(ctx, c, repo, f, "github.comment.mission-ask", askClosed(id)+"\n"+note))
		}
	}
	return nil
}

// askTrusted trusts an ask comment as the inbox does (scopeTrusted): from
// aayu's App, or from an account with write access. Anyone may comment, so
// another account's marker neither asks nor closes an ask.
func (e *Env) askTrusted(ctx context.Context, c *github.Client, owner, name string) func(*gh.User) (bool, error) {
	l, err := e.Config()
	id := config.Identity{}
	if err == nil {
		id = l.Config.Identity
	}
	perms := map[string]bool{}
	return func(u *gh.User) (bool, error) {
		return scopeTrusted(u, id, func(login string) (bool, error) {
			if ok, seen := perms[login]; seen {
				return ok, nil
			}
			perm, _, err := c.REST.Repositories.GetPermissionLevel(ctx, owner, name, login)
			if err != nil {
				return false, err
			}
			q := permissionOf(perm)
			perms[login] = q == "admin" || q == "maintain" || q == "write"
			return perms[login], nil
		})
	}
}

// openAsks are Feature f's open asks, the newest of each ID, read from its
// comments; trusted, when set, keeps only the comments of trusted authors.
func openAsks(ctx context.Context, c *github.Client, owner, name string, f int, trusted func(*gh.User) (bool, error)) ([]missionAsk, error) {
	cs, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.IssueComment, *gh.Response, error) {
		return c.REST.Issues.ListComments(ctx, owner, name, f, &gh.IssueListCommentsOptions{ListOptions: o})
	})
	if err != nil {
		return nil, Wrap(ExitExternal, err)
	}
	return asksIn(cs, trusted)
}

func asksIn(cs []*gh.IssueComment, trusted func(*gh.User) (bool, error)) ([]missionAsk, error) {
	var order []string
	byID := map[string]missionAsk{}
	for _, cm := range cs {
		body := cm.GetBody()
		if !strings.Contains(body, MissionAskMarker) && !strings.Contains(body, MissionAskDoneMark) {
			continue
		}
		if trusted != nil {
			if ok, err := trusted(cm.GetUser()); err != nil {
				return nil, err
			} else if !ok {
				continue
			}
		}
		if rest, ok := strings.CutPrefix(body, MissionAskDoneMark); ok {
			id, _, _ := strings.Cut(rest, " -->")
			delete(byID, id)
			continue
		}
		raw, _, _ := strings.Cut(strings.TrimPrefix(body, MissionAskMarker), " -->")
		var a missionAsk
		if json.Unmarshal([]byte(raw), &a) != nil || a.ID == "" {
			continue
		}
		if _, ok := byID[a.ID]; !ok {
			order = append(order, a.ID)
		}
		byID[a.ID] = a
	}
	var out []missionAsk
	for _, id := range order {
		if a, ok := byID[id]; ok {
			out = append(out, a)
		}
	}
	return out, nil
}

// missionItems are Feature it's open mission asks as inbox cards.
func (s *inboxSource) missionItems(ctx context.Context, it InboxItem) ([]InboxItem, error) {
	cs, err := s.commentsOf(ctx, it.Number)
	if err != nil {
		return nil, err
	}
	asks, err := asksIn(cs, func(u *gh.User) (bool, error) {
		return scopeTrusted(u, s.id, func(login string) (bool, error) { return s.canWrite(ctx, login) })
	})
	if err != nil {
		return nil, err
	}
	var out []InboxItem
	for _, a := range asks {
		card := it
		card.Kind, card.Seat, card.Ask, card.Evidence = InboxMission, verbs.EngineeringManager, a.ID, it.URL
		card.Title, card.Why, card.Details = termText(a.Title), termText(a.Why), nil
		for _, l := range a.Lines {
			card.Details = append(card.Details, termText(l))
		}
		for _, o := range a.Options {
			o.Label, o.Command, o.Recommended = termText(o.Label), termText(o.Command), false // a mission ask gives no recommendation
			card.Options = append(card.Options, o)
		}
		card.f.problem = termText(a.Title)
		out = append(out, card)
	}
	return out, nil
}

// missionAct does a mission card's option: the mission verb it names, with
// the person's answer as the reason.
func (e *Env) missionAct(ctx context.Context, it InboxItem, a InboxAction) error {
	fields := strings.Fields(a.Command)
	if len(fields) < 4 || fields[0] != "aayu" || fields[1] != "mission" {
		return Errorf(ExitUsage, "inbox: %q is not a mission step", a.Command)
	}
	in := Values{"action": fields[2], "feature": fields[3]}
	for i := 4; i < len(fields); i++ {
		switch fl := strings.TrimPrefix(fields[i], "--"); fl {
		case "once-more":
			in[fl] = true
		case "reason":
			in[fl] = a.Answer
		case "to":
			if i+1 < len(fields) {
				n, _ := strconv.Atoi(fields[i+1])
				in[fl], i = n, i+1
			}
		default:
			if i+1 < len(fields) {
				in[fl], i = fields[i+1], i+1
			}
		}
	}
	if err := handlers["mission"](ctx, e, in); err != nil || fields[2] != "stop" {
		return err // a gate run or a budget step closes its own card
	}
	repo, c, _, err := e.missionLedger(ctx) // the person stopped the mission: the card has its answer
	if err != nil {
		return err
	}
	return e.closeAsk(ctx, c, repo, it.Number, it.Ask, "A person stopped the mission: "+a.Answer)
}
