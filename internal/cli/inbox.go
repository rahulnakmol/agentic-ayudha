package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/term"
	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/audit"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/decide"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/inspect"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

func init() { register("inbox", runInbox) }

// `aayu inbox` (Story #431): every human-only step waiting
// for a person, each with the twin's recommendation, acted on with one
// keypress. It turns no gate off: each write is the person's, under their own
// token, and each step keeps the rules of the verb that owns it.

// Inbox kinds, in the order the inbox shows them.
const (
	InboxDecision  = "decision"  // an open Decision waiting for acceptance (a provisional one to ratify or revert)
	InboxCapped    = "capped"    // a PR at inspect's cycle cap, waiting for a person
	InboxUnshipped = "unshipped" // a Story whose PR merged into dev without a ship record
	InboxFixed     = "fixed"     // an open Bug whose fix merged into dev
	InboxReplaced  = "replaced"  // an open Story or Bug that another issue replaces
)

var inboxOrder = []string{InboxDecision, InboxCapped, InboxUnshipped, InboxFixed, InboxReplaced}

// InboxMarker marks the comment inbox leaves when it closes an issue.
const InboxMarker = "<!-- aayudha:inbox -->"

// InboxAction is one thing the person can do with an item.
type InboxAction struct {
	Label   string `json:"label"`
	Command string `json:"command"` // what it does, as the verb or inbox step that does it
	Option  string `json:"option,omitempty"`
}

// InboxItem is one human-only step.
type InboxItem struct {
	Kind        string        `json:"kind"`
	Number      int           `json:"number"`
	Title       string        `json:"title"`
	URL         string        `json:"url"`
	Seat        verbs.SeatID  `json:"seat"`
	Why         string        `json:"why"`
	Recommend   *InboxAction  `json:"recommend,omitempty"`
	Others      []InboxAction `json:"others,omitempty"`
	Provisional bool          `json:"provisional,omitempty"`
	PR          int           `json:"pr,omitempty"`          // the merged fix or delivery PR
	ReplacedBy  int           `json:"replaced_by,omitempty"` // the replacing issue
	labels      []string
	mergeSHA    string
}

var (
	replacesRE   = regexp.MustCompile(`(?i)\breplaces #([0-9]+)\b`)
	boldOption   = regexp.MustCompile(`^\s*(?:[-*]|[0-9]+[.)])\s+\*\*(.+?)\*\*`)
	namedOption  = regexp.MustCompile(`(?:^|[.;]\s+)\s*(?:[-*]\s+|[0-9]+[.)]\s+)?\**([A-Za-z][A-Za-z0-9_-]{0,30})\**:\s`)
	optionNumber = regexp.MustCompile(`(?i)\boption\s+([0-9]+)\b`)
	recommended  = regexp.MustCompile(`(?i)\s*\(recommended\)\.?$`)
)

// decisionOptions are the option names of a Decision's Options field, and the
// index of the one its Recommendation names (-1 when none).
func decisionOptions(options, recommendation string) ([]string, int) {
	var names []string
	rec := -1
	for _, line := range strings.Split(options, "\n") {
		if m := boldOption.FindStringSubmatch(line); m != nil {
			name := strings.TrimSuffix(strings.TrimSpace(recommended.ReplaceAllString(m[1], "")), ".")
			if strings.Contains(strings.ToLower(line), "(recommended)") && rec < 0 {
				rec = len(names)
			}
			names = append(names, name)
			continue
		}
		for _, m := range namedOption.FindAllStringSubmatch(line, -1) {
			if !slices.Contains(names, m[1]) {
				names = append(names, m[1])
			}
		}
	}
	if rec < 0 {
		if m := optionNumber.FindStringSubmatch(recommendation); m != nil {
			if i, _ := strconv.Atoi(m[1]); i >= 1 && i <= len(names) {
				rec = i - 1
			}
		}
	}
	for i, n := range names {
		if rec < 0 && decide.MatchOption(recommendation, n) {
			rec = i
		}
	}
	return names, rec
}

// gatherInbox lists the human-only steps in repo, read with c. dev is the
// integration branch delivery PRs merge into.
func gatherInbox(ctx context.Context, c *github.Client, repo, dev string) ([]InboxItem, error) {
	owner, name, _ := github.SplitRepo(repo)
	issues, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.Issue, *gh.Response, error) {
		return c.REST.Issues.ListByRepo(ctx, owner, name, &gh.IssueListByRepoOptions{State: "all", ListOptions: o})
	})
	if err != nil {
		return nil, err
	}
	prs, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.PullRequest, *gh.Response, error) {
		return c.REST.PullRequests.List(ctx, owner, name, &gh.PullRequestListOptions{State: "all", ListOptions: o})
	})
	if err != nil {
		return nil, err
	}
	// The first PR merged into dev that delivers each item, by the link
	// convention audit and ship use (a story/<n>- branch, or Refs #n).
	var merged []audit.PR
	var open []*gh.PullRequest
	for _, p := range prs {
		switch {
		case p.GetState() == "open":
			open = append(open, p)
		case p.GetBase().GetRef() == dev && (p.MergedAt != nil || p.GetMerged()):
			ap := audit.PR{Number: p.GetNumber(), Base: dev, Head: p.GetHead().GetRef(), Body: p.GetBody(), MergeSHA: p.GetMergeCommitSHA(), MergedAt: p.GetMergedAt().Time}
			ap.LinkStories()
			merged = append(merged, ap)
		}
	}
	slices.SortFunc(merged, func(a, b audit.PR) int { return a.MergedAt.Compare(b.MergedAt) })
	delivered := func(n int) *audit.PR {
		for i := range merged {
			if slices.Contains(merged[i].Refs, n) {
				return &merged[i]
			}
		}
		return nil
	}
	replacedBy := map[int]int{}
	for _, is := range issues {
		for _, m := range replacesRE.FindAllStringSubmatch(is.GetBody(), -1) {
			if n, _ := strconv.Atoi(m[1]); !is.IsPullRequest() && n != is.GetNumber() && (replacedBy[n] == 0 || is.GetNumber() < replacedBy[n]) {
				replacedBy[n] = is.GetNumber()
			}
		}
	}
	var out []InboxItem
	for _, is := range issues {
		if is.IsPullRequest() || is.GetState() != "open" {
			continue
		}
		labels := github.IssueLabels(is)
		typ, _ := workitems.TypeOf(is.GetType().GetName(), labels)
		it := InboxItem{Number: is.GetNumber(), Title: is.GetTitle(), URL: is.GetHTMLURL(), labels: labels}
		status := ""
		for _, l := range labels {
			if v, ok := strings.CutPrefix(l, "status:"); ok {
				status = v
			}
		}
		switch {
		case typ == "decision":
			out = append(out, decisionItem(is, it, labels))
		case (typ == "story" || typ == "bug") && replacedBy[it.Number] != 0:
			m := replacedBy[it.Number]
			it.Kind, it.Seat, it.ReplacedBy = InboxReplaced, verbs.ProductStrategist, m
			it.Why = fmt.Sprintf("#%d says it replaces #%d, and #%d is still open.", m, it.Number, it.Number)
			it.Recommend = &InboxAction{Label: fmt.Sprintf("close #%d as not planned, replaced by #%d", it.Number, m), Command: "inbox: close as replaced"}
			out = append(out, it)
		case typ == "story" && !slices.Contains([]string{"merged", "released", "measured", "done", "cancelled"}, status):
			pr := delivered(it.Number)
			if pr == nil {
				continue
			}
			it.Kind, it.Seat, it.PR, it.mergeSHA = InboxUnshipped, verbs.Auditor, pr.Number, pr.MergeSHA
			it.Why = fmt.Sprintf("PR #%d merged into %s (%s) without `aayu ship`, so no ship record or human pass exists; the Story is still status:%s.", pr.Number, dev, short(pr.MergeSHA), or(status, "unset"))
			it.Recommend = &InboxAction{Label: fmt.Sprintf("mark #%d merged without proof (it stays uncovered)", it.Number), Command: fmt.Sprintf("aayu audit --mark-merged %d", it.Number)}
			out = append(out, it)
		case typ == "bug":
			pr := delivered(it.Number)
			if pr == nil {
				continue
			}
			it.Kind, it.Seat, it.PR, it.mergeSHA = InboxFixed, verbs.EngineeringManager, pr.Number, pr.MergeSHA
			it.Why = fmt.Sprintf("its fix, PR #%d, merged into %s (%s), and the Bug is still open.", pr.Number, dev, short(pr.MergeSHA))
			it.Recommend = &InboxAction{Label: fmt.Sprintf("close #%d as completed, fixed in #%d", it.Number, pr.Number), Command: "inbox: close as fixed"}
			out = append(out, it)
		}
	}
	for _, p := range open {
		comments, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.IssueComment, *gh.Response, error) {
			return c.REST.Issues.ListComments(ctx, owner, name, p.GetNumber(), &gh.IssueListCommentsOptions{ListOptions: o})
		})
		if err != nil {
			return nil, err
		}
		var st inspect.State
		for _, cm := range comments {
			if strings.Contains(cm.GetBody(), inspect.Marker) {
				st = inspect.ParseState(cm.GetBody())
			}
		}
		if st.Cycles < inspect.MaxCycles {
			continue
		}
		disp := "none recorded"
		if st.Result != nil {
			disp = st.Result.Disposition
		}
		out = append(out, InboxItem{Kind: InboxCapped, Number: p.GetNumber(), Title: p.GetTitle(), URL: p.GetHTMLURL(), Seat: verbs.EngineeringManager,
			Why:       fmt.Sprintf("inspect ran %d of %d fix/rebuild cycles and stopped (last disposition: %s); a person reviews it before anything else runs.", st.Cycles, inspect.MaxCycles, disp),
			Recommend: &InboxAction{Label: "open it and review it", Command: "open"},
			Others:    []InboxAction{{Label: fmt.Sprintf("ship PR #%d now (ship's gates still apply)", p.GetNumber()), Command: fmt.Sprintf("aayu ship %d", p.GetNumber())}}})
	}
	slices.SortStableFunc(out, func(a, b InboxItem) int {
		if d := slices.Index(inboxOrder, a.Kind) - slices.Index(inboxOrder, b.Kind); d != 0 {
			return d
		}
		return a.Number - b.Number
	})
	return out, nil
}

// decisionItem is an open Decision as an inbox step: accept its
// recommendation, or another of its options, through `decide --accept`.
func decisionItem(is *gh.Issue, it InboxItem, labels []string) InboxItem {
	w, _ := workitems.Parse(is.GetNumber(), is.GetTitle(), is.GetBody(), is.GetType().GetName(), labels)
	it.Kind, it.Seat = InboxDecision, verbs.Founder
	it.Provisional = slices.Contains(labels, workitems.LabelProvisional)
	if ps := workitems.Validate(w, workitems.StageReady); len(ps) > 0 {
		it.Why = fmt.Sprintf("not Decision-ready (%s): open it and complete it before it can be accepted.", ps[0].String())
		return it
	}
	names, rec := decisionOptions(w.Fields["options"], w.Fields["recommendation"])
	switch {
	case it.Provisional:
		by := "no deadline recorded"
		if t, ok := decide.RatifyBy(is.GetBody()); ok {
			by = t.UTC().Format("2006-01-02 15:04 UTC")
		}
		it.Why = fmt.Sprintf("its recommendation applies provisionally (V8): ratify it, or revert to another option, by %s.", by)
	case slices.Contains(labels, workitems.LabelWaiver):
		it.Why = "a floor waiver: only a person accepts it, and it is never applied provisionally (rule 7)."
	default:
		it.Why = "waiting for its authority to accept an option."
	}
	for i, n := range names {
		a := InboxAction{Label: "accept " + clip(n, 90), Command: fmt.Sprintf("aayu decide %d --accept %q", it.Number, n), Option: n}
		if it.Provisional && i != rec {
			a.Label = "revert: accept " + clip(n, 82)
		}
		if i == rec {
			if it.Provisional {
				a.Label = "ratify: accept " + clip(n, 82)
			}
			it.Recommend = &a
			continue
		}
		it.Others = append(it.Others, a)
	}
	return it
}

// runInbox is `aayu inbox`.
func runInbox(ctx context.Context, e *Env, _ Values) error {
	if e.CI { // the aayudha-approve workflow: an approver's reply to a batch (approve.go)
		return e.approveFromComment(ctx)
	}
	if e.Getenv("AAYU_AGENT") == "1" {
		return Errorf(ExitBlocked, "inbox is human-only: it acts as you on human-only steps, so it is refused when AAYU_AGENT=1")
	}
	// A host's agent marker (an orb's Terminal tab inherits one) does not refuse it
	// here: humanCaller asks the person to approve from any device (presence.go).
	if !e.isTerminal(e.In) {
		return WithNext(Errorf(ExitBlocked, "inbox is human-only and asks for one keypress per step, so it needs a terminal"), "aayu next")
	}
	repo, err := e.RepoName(ctx)
	if err != nil {
		return err
	}
	l, err := e.Config()
	if err != nil {
		return err
	}
	c, login, err := humanCaller(ctx, e, &l.Config, "inbox")
	if err != nil {
		return err
	}
	items, err := gatherInbox(ctx, c, repo, e.devBranch())
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	w := e.Out
	if len(items) == 0 {
		fmt.Fprintf(w, "%s: nothing waits for you.\n", repo)
		return nil
	}
	fmt.Fprintf(w, "%s: %d human-only step(s) wait for you, @%s. Keys: a accept the recommendation, c choose another, s skip, o open, q quit.\n", repo, len(items), login)
	done, skipped := 0, 0
	for i, it := range items {
		fmt.Fprintf(w, "\n[%d/%d] %s #%d: %s\n  %s\n", i+1, len(items), inboxNoun(it), it.Number, it.Title, it.Why)
		switch {
		case it.Recommend != nil && it.Kind == InboxDecision:
			fmt.Fprintf(w, "  the Decision recommends: %s\n", it.Recommend.Label)
		case it.Recommend != nil:
			fmt.Fprintf(w, "  %s recommends: %s\n", e.twin(it.Seat), it.Recommend.Label)
		default:
			fmt.Fprintf(w, "  %s has no recommendation to accept here.\n", e.twin(it.Seat))
		}
		for j, o := range it.Others {
			fmt.Fprintf(w, "  other %d: %s\n", j+1, o.Label)
		}
		for {
			fmt.Fprint(w, "  > ")
			k, err := e.readKey()
			if err != nil {
				fmt.Fprintln(w)
				return finishInbox(w, done, skipped, len(items)-i)
			}
			fmt.Fprintf(w, "%c\n", k)
			var act *InboxAction
			switch k {
			case 'q', 3:
				return finishInbox(w, done, skipped, len(items)-i)
			case 's':
				skipped++
			case 'o':
				fmt.Fprintf(w, "  open: %s\n", it.URL)
				e.openURL(it.URL)
				continue
			case 'a':
				if it.Recommend == nil {
					fmt.Fprintln(w, "  no recommendation: choose another (c), open (o), or skip (s)")
					continue
				}
				act = it.Recommend
			case 'c':
				if len(it.Others) == 0 {
					fmt.Fprintln(w, "  no other option: accept (a), open (o), or skip (s)")
					continue
				}
				act = &it.Others[0]
				if len(it.Others) > 1 {
					fmt.Fprintf(w, "  which other (1-%d)? ", len(it.Others))
					d, err := e.readKey()
					if err != nil {
						return finishInbox(w, done, skipped, len(items)-i)
					}
					fmt.Fprintf(w, "%c\n", d)
					n := int(d - '0')
					if n < 1 || n > len(it.Others) {
						fmt.Fprintln(w, "  not one of the others")
						continue
					}
					act = &it.Others[n-1]
				}
			default:
				fmt.Fprintln(w, "  keys: a, c, s, o, q")
				continue
			}
			if act != nil {
				if act.Command == "open" {
					fmt.Fprintf(w, "  open: %s\n", it.URL)
					e.openURL(it.URL)
				} else if err := e.inboxAct(ctx, &l.Config, c, repo, login, it, *act); err != nil {
					fmt.Fprintf(w, "  not done: %v\n", err)
					var ae *Error
					if errors.As(err, &ae) && ae.Next != "" {
						fmt.Fprintf(w, "  next: %s\n", ae.Next)
					}
				} else {
					done++
				}
			}
			break
		}
	}
	return finishInbox(w, done, skipped, 0)
}

func finishInbox(w io.Writer, done, skipped, left int) error {
	_, err := fmt.Fprintf(w, "\ninbox: %d done, %d skipped, %d left\n", done, skipped, left)
	return err
}

func inboxNoun(it InboxItem) string {
	switch it.Kind {
	case InboxDecision:
		if it.Provisional {
			return "Provisional Decision"
		}
		return "Decision"
	case InboxCapped:
		return "PR at inspect's cycle cap"
	case InboxUnshipped:
		return "Story merged without ship"
	case InboxFixed:
		return "Bug with its fix merged"
	}
	return "Replaced"
}

// twin names the seat's twin ("Athena (Engineering Manager)"), or the seat.
func (e *Env) twin(seat verbs.SeatID) string {
	v := e.voice(seat, "inbox")
	if v.Name == "" {
		return v.Seat
	}
	return v.Name + " (" + v.Seat + ")"
}

// inboxAct performs one action on an item under the person's client c. A
// Decision goes through `decide --accept`, a capped PR through `ship`; the
// rest are the inbox's own human-only steps.
func (e *Env) inboxAct(ctx context.Context, cfg *config.Config, c *github.Client, repo, login string, it InboxItem, a InboxAction) error {
	switch it.Kind {
	case InboxDecision:
		return decideAccept(ctx, e, cfg, repo, []int{it.Number}, []string{a.Option})
	case InboxCapped:
		return handlers["ship"](ctx, e, Values{"pr": strconv.Itoa(it.Number)})
	}
	var note, status, reason string
	switch it.Kind {
	case InboxReplaced:
		note = fmt.Sprintf("Replaced by #%d. Closed as not planned by @%s through `aayu inbox`.", it.ReplacedBy, login)
		status, reason = "cancelled", "not_planned"
	case InboxFixed:
		note = fmt.Sprintf("Fixed in #%d (`%s`, merged into %s). Closed as completed by @%s through `aayu inbox`.", it.PR, short(it.mergeSHA), e.devBranch(), login)
		status, reason = "done", "completed"
	case InboxUnshipped:
		note = fmt.Sprintf("**Merged without proof (post-gate).** This story's change merged in #%d (`%s`) without `aayu ship`: no ship record or human "+
			"pass was made, and no authoritative pack exists for it on the ledger unless the merge gate wrote one. It is done but unproven, and "+
			"`aayu audit` counts it as uncovered in proof coverage.\n\nMoved to `status:merged` by @%s through `aayu inbox`; no ship record is made after the fact.",
			it.PR, short(it.mergeSHA), login)
		status = "merged"
	}
	marker := InboxMarker
	if it.Kind == InboxUnshipped {
		marker = PostGateMarker
	}
	// In order, stopping at the first failure: status, lease, comment, close.
	err := setStatus(ctx, c, repo, it.Number, it.labels, status)
	if err == nil && slices.Contains(it.labels, LeaseLabel) {
		err = removeLabel(ctx, c, repo, it.Number, LeaseLabel)
	}
	if err == nil {
		_, err = c.UpsertComment(ctx, repo, it.Number, marker, marker+"\n"+note+"\n", actions.Action{Kind: "github.comment.inbox", Payload: map[string]any{"kind": it.Kind, "by": login}})
	}
	if err == nil && reason != "" {
		err = closeIssue(ctx, c, repo, it.Number, reason)
	}
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	fmt.Fprintf(e.Out, "  done: %s\n", a.Label)
	return nil
}

// readKey reads one keypress: raw from a real terminal, else the next
// non-space byte of the input (scripted keypresses in tests).
func (e *Env) readKey() (byte, error) {
	var b [1]byte
	if f, ok := e.In.(*os.File); ok && term.IsTerminal(f.Fd()) {
		if old, err := term.MakeRaw(f.Fd()); err == nil {
			defer func() { _ = term.Restore(f.Fd(), old) }()
		}
		_, err := f.Read(b[:])
		return b[0], err
	}
	for {
		if _, err := io.ReadFull(e.In, b[:]); err != nil {
			return 0, err
		}
		if b[0] != ' ' && b[0] != '\n' && b[0] != '\r' && b[0] != '\t' {
			return b[0], nil
		}
	}
}

// openURL opens url in the browser, best effort, when aayu runs on a real
// terminal; the URL is always printed as well.
func (e *Env) openURL(url string) {
	if f, ok := e.In.(*os.File); ok && term.IsTerminal(f.Fd()) {
		opener := map[string][]string{"darwin": {"open"}, "windows": {"rundll32", "url.dll,FileProtocolHandler"}}[runtime.GOOS]
		if opener == nil {
			opener = []string{"xdg-open"}
		}
		_ = exec.Command(opener[0], append(opener[1:], url)...).Start() // #nosec G204 -- a fixed opener with the item's GitHub URL
	}
}
