package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/grill"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/ready"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// Approve to build cards (Story #1173): a Feature whose baselines are all
// approved and that has no approval yet is a card in aayu inbox, in the same
// session as its Baseline cards, so one device code covers the Feature. The
// card shows what Approve to build approves: the stories and Bugs, the
// budget, the agents, the milestones, and the high-risk entries by story.
// Approving runs `aayu ready <f>` (approveToBuild) pinned to the stories and
// Bugs the card showed.

// approveChanged is what the inbox says when Feature f's stories changed
// after its card was read.
func approveChanged(f int) string {
	return fmt.Sprintf("The stories of #%d changed while you read it.", f)
}

// approveStories is the hash of the stories and Bugs an Approve to build
// card shows: each story as the approval pins it, and each story and Bug of
// the mission plan with its milestone.
func approveStories(rec ready.Approval, p MissionPlan) string {
	type item struct {
		Number                         int
		Kind, Title, Effort, Milestone string
	}
	items := make([]item, len(p.Graph))
	for i, g := range p.Graph {
		items[i] = item{g.Number, g.Kind, g.Title, g.Effort, g.Milestone}
	}
	j, _ := json.Marshal([]any{rec.Stories, items})
	sum := sha256.Sum256(j)
	return hex.EncodeToString(sum[:])
}

// approveCard is Feature f's Approve to build card, read from the ledger tree
// t, or nil when f has an approval already, or a part it needs has no
// approved version (unapproved names the parts with none): its Baseline cards
// come first. A Feature that Approve to build refuses has a card that says
// the command's refusal and offers no approval. It writes nothing.
func (e *Env) approveCard(ctx context.Context, c *github.Client, w *ledger.Writer, t *ledger.Tree, repo string, f int, unapproved []string) (*InboxItem, error) {
	if b, err := t.File(ctx, ready.ApprovalPath(f)); b != nil || ignoreMissing(err) != nil {
		return nil, ignoreMissing(err)
	}
	if slices.ContainsFunc(unapproved, func(p string) bool { return p != "ux" }) {
		return nil, nil
	}
	if len(unapproved) > 0 { // the UX design is needed only when the footprint has L01
		owner, name, _ := github.SplitRepo(repo)
		is, _, err := c.REST.Issues.Get(ctx, owner, name, f)
		if err != nil {
			return nil, Wrap(ExitExternal, err)
		}
		it, _ := workitems.Parse(f, is.GetTitle(), is.GetBody(), is.GetType().GetName(), github.IssueLabels(is))
		if slices.Contains(e.footprint(it), string(verbs.L01)) {
			return nil, nil
		}
	}
	cat, err := e.grillCatalog()
	if err != nil {
		return nil, err
	}
	r := &ready.Ready{Repo: repo, TechnicalOwner: e.technicalOwner(), GH: c, Concrete: func(it workitems.Item) []workitems.Problem { return grill.Check(cat, it) }}
	a := &approveRun{e: e, c: c, w: w, r: r, n: f}
	rec, _, _, _, err := a.prepare(ctx)
	var plan MissionPlan
	if err == nil {
		plan, _, _, err = a.missionPlan(ctx, rec.Agents)
	}
	if err != nil && CodeOf(err) != ExitBlocked {
		return nil, err
	}
	it := approveItem(f, rec, plan, err)
	return &it, nil
}

// approveItem is the card of Feature f's Approve to build: the approval rec
// and the mission plan p it would write, or the command's refusal.
func approveItem(f int, rec ready.Approval, p MissionPlan, refusal error) InboxItem {
	it := InboxItem{Kind: InboxApprove, Number: f, Seat: verbs.ProductStrategist, CounselFrom: "facts"}
	if refusal != nil {
		why := refusal.Error()
		it.Lines, it.Why, it.f.problem = []string{why}, why, "Approve to build is off"
		it.Options = []InboxAction{{N: 1, Label: "leave it: aayu ready refuses it now", Command: inboxLeave}}
		it.Counsel = fmt.Sprintf("Approve to build of #%d is off, and aayu ready %d refuses it too, for the reason on the card. Its next step says "+
			"who acts. The card offers no approval until it holds. You decide; I only advise.", f, f)
		return it
	}
	it.Hash = approveStories(rec, p)
	stories, bugs := 0, 0
	var lines []string
	for _, g := range p.Graph {
		kind := "story"
		if g.Kind == "bug" {
			kind, bugs = "Bug", bugs+1
		} else {
			stories++
		}
		lines = append(lines, fmt.Sprintf("  #%d %s · %s · effort %s · %s", g.Number, termText(g.Title), kind, termText(or(g.Effort, "none")), g.Milestone))
	}
	what := fmt.Sprintf("%d %s", stories, plural(stories, "story", "stories"))
	if bugs > 0 {
		what += fmt.Sprintf(" and %d %s", bugs, plural(bugs, "Bug", "Bugs"))
	}
	ms := make([]string, len(p.Milestones))
	for i, m := range p.Milestones {
		ms[i] = m.Name + " " + issueRefs(m.Stories)
	}
	parts := make([]string, len(rec.Baselines))
	for i, b := range rec.Baselines {
		parts[i] = fmt.Sprintf("%s v%d", ready.Name(b.Part), b.Version)
	}
	it.Lines = append(append([]string{"Stories  " + what + ", each as it is now"}, lines...),
		fmt.Sprintf("Budget   up to %s (%d %s × %s, [budget] per_story_usd)", ready.USD(rec.Budget.TotalUSD), rec.Budget.Stories, plural(rec.Budget.Stories, "story", "stories"), ready.USD(rec.Budget.PerStoryUSD)),
		fmt.Sprintf("Agents   up to %d at once (%s)", p.Agents, p.AgentsSource),
		fmt.Sprintf("Milestones %s; complete when %s", strings.Join(ms, " · "), p.CompleteWhen),
		"High-risk entries by story: none shown. They come with #1029, which has not merged, so the card cannot read them yet.",
		"Baselines "+strings.Join(parts, ", "))
	it.Why = fmt.Sprintf("Every baseline of #%d is approved and it waits for Approve to build; approving reads the stories again after the device code.", f)
	it.f.problem = "to approve to build"
	it.Options = []InboxAction{
		{N: 1, Recommended: true, Label: fmt.Sprintf("approve #%d to build: %s, budget up to %s, up to %d at once", f, what, ready.USD(rec.Budget.TotalUSD), p.Agents),
			Command: fmt.Sprintf("aayu ready %d", f)},
		{N: 2, Label: "approve it with another number of agents", Command: fmt.Sprintf("aayu ready %d --agents", f),
			Ask: "How many stories run at once? A whole number, 1 or more."},
		{N: 3, Label: "leave it: nothing is approved", Command: inboxLeave},
	}
	it.Counsel = fmt.Sprintf("What you agree to: the %s of #%d on the card, a budget up to %s, and up to %d at once. Then I make the stories Ready, "+
		"and a person starts the mission. The card shows no high-risk entries until #1029 merges, so read a story's plan if it worries you. "+
		"Pick 2 to change the agents. You decide; I only advise.", what, f, ready.USD(rec.Budget.TotalUSD), p.Agents)
	return it
}
