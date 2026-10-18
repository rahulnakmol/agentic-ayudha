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

// InboxAction is one thing the person can do with an item: one of its
// options, under the number the person picks it by. A Decision's options keep
// the Decision's own numbers (decide.Options), as the approval batch does.
type InboxAction struct {
	N           int    `json:"n"`
	Label       string `json:"label"`   // a Decision option's bold title, or the step's action
	Command     string `json:"command"` // what it does, as the verb or inbox step that does it
	Recommended bool   `json:"recommended,omitempty"`
}

// InboxItem is one human-only step.
type InboxItem struct {
	Kind        string        `json:"kind"`
	Number      int           `json:"number"`
	Title       string        `json:"title"`
	URL         string        `json:"url"`
	Seat        verbs.SeatID  `json:"seat"`
	Why         string        `json:"why"`
	Options     []InboxAction `json:"options,omitempty"`
	OneWay      bool          `json:"one_way,omitempty"` // an irreversible Decision: every pick is confirmed
	Provisional bool          `json:"provisional,omitempty"`
	PR          int           `json:"pr,omitempty"`          // the merged fix or delivery PR
	ReplacedBy  int           `json:"replaced_by,omitempty"` // the replacing issue
	labels      []string
	mergeSHA    string
}

var replacesRE = regexp.MustCompile(`(?i)\breplaces #([0-9]+)\b`)

// recommended is the option the item recommends, or nil.
func (it InboxItem) recommended() *InboxAction {
	for i := range it.Options {
		if it.Options[i].Recommended {
			return &it.Options[i]
		}
	}
	return nil
}

// option is the item's option numbered n, or nil.
func (it InboxItem) option(n int) *InboxAction {
	for i := range it.Options {
		if it.Options[i].N == n {
			return &it.Options[i]
		}
	}
	return nil
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
			it.Options = []InboxAction{{N: 1, Recommended: true, Label: fmt.Sprintf("close #%d as not planned, replaced by #%d", it.Number, m), Command: "inbox: close as replaced"}}
			out = append(out, it)
		case typ == "story" && !slices.Contains([]string{"merged", "released", "measured", "done", "cancelled"}, status):
			pr := delivered(it.Number)
			if pr == nil {
				continue
			}
			it.Kind, it.Seat, it.PR, it.mergeSHA = InboxUnshipped, verbs.Auditor, pr.Number, pr.MergeSHA
			it.Why = fmt.Sprintf("PR #%d merged into %s (%s) without `aayu ship`, so no ship record or human pass exists; the Story is still status:%s.", pr.Number, dev, short(pr.MergeSHA), or(status, "unset"))
			it.Options = []InboxAction{{N: 1, Recommended: true, Label: fmt.Sprintf("mark #%d merged without proof (it stays uncovered)", it.Number), Command: fmt.Sprintf("aayu audit --mark-merged %d", it.Number)}}
			out = append(out, it)
		case typ == "bug":
			pr := delivered(it.Number)
			if pr == nil {
				continue
			}
			it.Kind, it.Seat, it.PR, it.mergeSHA = InboxFixed, verbs.EngineeringManager, pr.Number, pr.MergeSHA
			it.Why = fmt.Sprintf("its fix, PR #%d, merged into %s (%s), and the Bug is still open.", pr.Number, dev, short(pr.MergeSHA))
			it.Options = []InboxAction{{N: 1, Recommended: true, Label: fmt.Sprintf("close #%d as completed, fixed in #%d", it.Number, pr.Number), Command: "inbox: close as fixed"}}
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
			Why: fmt.Sprintf("inspect ran %d of %d fix/rebuild cycles and stopped (last disposition: %s); a person reviews it before anything else runs.", st.Cycles, inspect.MaxCycles, disp),
			Options: []InboxAction{{N: 1, Recommended: true, Label: "open it and review it", Command: "open"},
				{N: 2, Label: fmt.Sprintf("ship PR #%d now (ship's gates still apply)", p.GetNumber()), Command: fmt.Sprintf("aayu ship %d", p.GetNumber())}}})
	}
	slices.SortStableFunc(out, func(a, b InboxItem) int {
		if d := slices.Index(inboxOrder, a.Kind) - slices.Index(inboxOrder, b.Kind); d != 0 {
			return d
		}
		return a.Number - b.Number
	})
	return out, nil
}

// decisionItem is an open Decision as an inbox step: accept one of its
// options, by the Decision's own number, through `decide --accept`.
func decisionItem(is *gh.Issue, it InboxItem, labels []string) InboxItem {
	w, _ := workitems.Parse(is.GetNumber(), is.GetTitle(), is.GetBody(), is.GetType().GetName(), labels)
	it.Kind, it.Seat = InboxDecision, verbs.Founder
	it.Provisional = slices.Contains(labels, workitems.LabelProvisional)
	if ps := workitems.Validate(w, workitems.StageReady); len(ps) > 0 {
		it.Why = fmt.Sprintf("not Decision-ready (%s): open it and complete it before it can be accepted.", ps[0].String())
		return it
	}
	it.OneWay = strings.TrimSpace(w.Fields[workitems.FieldReversible]) != "true" || slices.Contains(labels, workitems.LabelWaiver)
	switch {
	case it.Provisional:
		by := "no deadline recorded"
		if t, ok := decide.RatifyBy(is.GetBody()); ok {
			by = t.UTC().Format("2006-01-02 15:04 UTC")
		}
		it.Why = fmt.Sprintf("its recommendation applies provisionally (V8): ratify it by accepting it, or revert to another option, by %s.", by)
	case slices.Contains(labels, workitems.LabelWaiver):
		it.Why = "a floor waiver: only a person accepts it, and it is never applied provisionally (rule 7)."
	default:
		it.Why = "waiting for its authority to accept an option."
	}
	opts, _ := decide.Options(w)
	for _, o := range opts {
		it.Options = append(it.Options, InboxAction{N: o.N, Label: clip(o.Name, 60), Recommended: o.Recommended,
			Command: fmt.Sprintf("aayu decide %d --accept %d", it.Number, o.N)})
	}
	return it
}

// inboxChoices is the line under every step that says what each key does.
func inboxChoices(it InboxItem) string {
	verb := "accept"
	if it.Kind != InboxDecision {
		verb = "do"
	}
	var parts []string
	switch r := it.recommended(); {
	case r != nil && len(it.Options) > 1:
		parts = append(parts, fmt.Sprintf("a or Enter = %s the recommendation (option %d)", verb, r.N))
	case r != nil:
		parts = append(parts, "a or Enter = "+r.Label)
	}
	if n := len(it.Options); n > 1 {
		parts = append(parts, fmt.Sprintf("1-%d = %s that option", n, verb))
	}
	open := "o = open on GitHub"
	if it.Kind == InboxDecision && len(it.Options) == 0 {
		open = "o = open on GitHub to complete it"
	}
	return "  " + strings.Join(append(parts, "s = skip for now", open, "q = quit"), " · ")
}

// inboxConfirm is the one line that says exactly what a pick does, asked
// before a pick that is not the recommendation or that is a one-way door.
func inboxConfirm(it InboxItem, a InboxAction) string {
	var s string
	if it.Kind == InboxDecision {
		s = fmt.Sprintf("This accepts option %d (%s) as the answer to Decision #%d and closes it", a.N, a.Label, it.Number)
	} else {
		s = "This will " + a.Label
	}
	switch r := it.recommended(); {
	case a.Recommended:
	case r != nil:
		s += fmt.Sprintf(", instead of the recommendation (option %d, %s)", r.N, r.Label)
	default:
		s += "; it names no recommendation"
	}
	if it.OneWay {
		s += "; it is a one-way door, so it is not undone later"
	}
	return "  " + s + ". Apply it? y = yes · n = no, go back"
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
	fmt.Fprintf(w, "%s: %d human-only step(s) wait for you, @%s. Each step lists its choices.\n", repo, len(items), login)
	done, skipped := 0, 0
	for i, it := range items {
		fmt.Fprintf(w, "\n[%d/%d] %s #%d: %s\n  %s\n", i+1, len(items), inboxNoun(it), it.Number, it.Title, it.Why)
		r := it.recommended()
		switch {
		case len(it.Options) == 0:
			fmt.Fprintf(w, "  %s has no option to accept here.\n", e.twin(it.Seat))
		case it.Kind != InboxDecision && len(it.Options) == 1:
			fmt.Fprintf(w, "  %s recommends: %s\n", e.twin(it.Seat), r.Label)
		default:
			switch {
			case it.Kind != InboxDecision:
				fmt.Fprintf(w, "  %s recommends option %d.\n", e.twin(it.Seat), r.N)
			case r == nil:
				fmt.Fprintln(w, "  the Decision names no recommendation.")
			}
			for _, o := range it.Options {
				mark := ""
				if o.Recommended {
					mark = " (recommended)"
				}
				fmt.Fprintf(w, "  %d. %s%s\n", o.N, o.Label, mark)
			}
		}
		choices := inboxChoices(it)
		fmt.Fprintln(w, choices)
		key := func() (byte, error) {
			fmt.Fprint(w, "  > ")
			k, err := e.readKey()
			switch {
			case err != nil:
			case k == '\r' || k == '\n':
				fmt.Fprintln(w)
			default:
				fmt.Fprintf(w, "%c\n", k)
			}
			return k, err
		}
		for {
			k, err := key()
			if err != nil {
				fmt.Fprintln(w)
				return finishInbox(w, done, skipped, len(items)-i)
			}
			var act *InboxAction
			switch {
			case k == 'q' || k == 3:
				return finishInbox(w, done, skipped, len(items)-i)
			case k == 's':
				skipped++
			case k == 'o':
				fmt.Fprintf(w, "  open: %s\n", it.URL)
				e.openURL(it.URL)
				continue
			case (k == 'a' || k == '\r' || k == '\n') && r != nil:
				act = r
			case k >= '1' && k <= '9' && it.option(int(k-'0')) != nil:
				act = it.option(int(k - '0'))
			default:
				fmt.Fprintln(w, choices)
				continue
			}
			if act != nil && (!act.Recommended || it.OneWay) {
				fmt.Fprintln(w, inboxConfirm(it, *act))
				y, err := key()
				if err != nil {
					fmt.Fprintln(w)
					return finishInbox(w, done, skipped, len(items)-i)
				}
				if y != 'y' && y != 'Y' {
					fmt.Fprintln(w, "  not applied")
					fmt.Fprintln(w, choices)
					continue
				}
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
		return decideAccept(ctx, e, cfg, repo, []int{it.Number}, []string{strconv.Itoa(a.N)})
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
