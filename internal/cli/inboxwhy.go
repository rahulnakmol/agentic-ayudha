package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/github"
)

// Every way an inbox write, a read of GitHub, or the session's sign-in can
// fail, in a person's words (Story #797): what happened, whether anything
// changed on GitHub, and who acts next. The words name no URL, HTTP method,
// status code, commit, or Go error; those are the details, one key (d) away
// on the card and in full in the log printed when the inbox closes. Nothing
// here reads GitHub: it explains what GitHub and aayu already said.

// inboxWhy is a failure in a person's words. What starts with the item's
// number; Changed says what GitHub holds now; Next names who acts.
type inboxWhy struct {
	What, Changed, Next string
	Detail              []string
	// Landed reports a merge that GitHub made before a later step failed:
	// the card shows it done, with the step that did not finish.
	Landed bool
	// Retry reports whether pressing Enter again can work (its Next says
	// so); otherwise the card stops offering the action that failed.
	Retry bool
}

// lines are the three lines, in order.
func (w inboxWhy) lines() []string { return []string{w.What, w.Changed, "Next: " + w.Next} }

// inboxCause is what an error says, read once: the kind of failure and
// GitHub's own words for it.
type inboxCause struct {
	status  int    // GitHub's HTTP status, 0 when it gave none
	message string // GitHub's message, or aayu's refusal
	request string // the request, for the details
	rate    bool   // GitHub limits the rate
	reset   time.Time
	budget  bool // the session's call budget is used
	network bool // GitHub was not reached, or did not answer
	refusal string
}

func causeOf(err error) inboxCause {
	var c inboxCause
	c.message = err.Error()
	var rl *gh.RateLimitError
	var ab *gh.AbuseRateLimitError
	var er *gh.ErrorResponse
	var ue *url.Error
	var ne net.Error
	var ae *Error
	switch {
	case errors.As(err, &rl):
		c.rate, c.reset, c.status = true, rl.Rate.Reset.Time, 403
	case errors.As(err, &ab):
		c.rate, c.status = true, 403
		if ab.RetryAfter != nil {
			c.reset = time.Now().Add(*ab.RetryAfter)
		}
	case errors.Is(err, github.ErrRateLimited):
		c.rate = true
	case errors.Is(err, github.ErrBudget):
		c.budget = true
	case errors.As(err, &er):
		c.message = er.Message
		if er.Response != nil {
			c.status = er.Response.StatusCode
			if er.Response.Request != nil {
				c.request = er.Response.Request.Method + " " + er.Response.Request.URL.Path
			}
		}
		for _, e := range er.Errors {
			if e.Message != "" {
				c.message += "; " + e.Message
			} else if e.Code != "" {
				c.message += "; " + e.Field + " " + e.Code
			}
		}
	case errors.As(err, &ue) || errors.As(err, &ne) || errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "connection reset") || strings.Contains(err.Error(), "connection refused"):
		c.network = true
	}
	if errors.As(err, &ae) && c.status == 0 && !c.network && !c.rate && !c.budget {
		c.refusal = err.Error()
	}
	return c
}

// inboxAct is what a write on it does, as a verb for "wasn't …": merged,
// recorded, approved, done.
func inboxVerb(it InboxItem, a InboxAction) (did, not string) {
	switch {
	case it.Kind == InboxReady || it.Kind == InboxCapped && strings.HasPrefix(a.Command, "aayu ship"):
		return "shipped", "wasn't shipped"
	case it.Kind == InboxDecision:
		return "recorded", "wasn't recorded"
	case it.Kind == InboxHumanReview:
		return "approved", "wasn't approved"
	}
	return "done", "wasn't done"
}

// The refusals ship, decide and inspect --approve make, by their words
// (internal/cli/ship.go, shipproof.go, decide.go, inspect.go). Each is
// matched on the message those files write; an unknown refusal still gets
// plain words and its text behind d.
var (
	reHeadMoved     = regexp.MustCompile(`head moved`)
	reAlreadyMerged = regexp.MustCompile(`the PR is already merged`)
	reClosed        = regexp.MustCompile(`the PR is closed|#[0-9]+ is closed|is closed or a draft`)
	reNoStory       = regexp.MustCompile(`no story \(Refs #n`)
	reCheck         = regexp.MustCompile(`(?:required check )?(\S+) is (\S+) on [0-9a-f]+`)
	reNoPack        = regexp.MustCompile(`no merge gate pack|aayudha-pack artifact|its pack is for`)
	rePack          = regexp.MustCompile(`pack does not verify`)
	reFloors        = regexp.MustCompile(`floors do not hold`)
	rePlan          = regexp.MustCompile(`plan-time review is required`)
	reNoPass        = regexp.MustCompile(`no human pass`)
	reChanged       = regexp.MustCompile(`changed while ship judged it|while ship judged the PR on proof`)
	reSecondHuman   = regexp.MustCompile(`a second human \((\S+)\) holds a seat|has a bot identity`)
	reAuthority     = regexp.MustCompile(`the authority for #[0-9]+ is (\S+)`)
	reNotReady      = regexp.MustCompile(`is not Decision-ready`)
	reApprover      = regexp.MustCompile(`the \[autonomy\] approver is (\S+), not`)
	reNoApprover    = regexp.MustCompile(`no \[autonomy\] approver is set`)
	reBatchOpen     = regexp.MustCompile(`batch \S+ is still open`)
	reLedger        = regexp.MustCompile(`(?i)ledger`)
	reBypass        = regexp.MustCompile(`post .* starting "Solo-mode bypass:"`)
	reConflicts     = regexp.MustCompile(`it conflicts with \S+, so GitHub cannot merge it`)
	reMergeUnknown  = regexp.MustCompile(`GitHub has not worked out yet whether it can merge`)
	reBypassPost    = regexp.MustCompile(`Solo-mode bypass comment was not posted`)
	reHex           = regexp.MustCompile(`\b[0-9a-f]{7,40}\b`)
)

// logDetail is a failed write's quit-log line: the error, then the step,
// GitHub's answer, and the head commit from detail (its Error line repeats
// the error, so it is left out).
func logDetail(err string, detail []string) string {
	out := []string{err}
	for _, d := range detail {
		if !strings.HasPrefix(d, "Error: ") {
			out = append(out, d)
		}
	}
	return strings.Join(out, "; ")
}

// explainFailure says, in a person's words, why a write on it stopped:
// stage is the step it stopped at and wrote what GitHub had already answered
// as written ("may have …" for a write that got no answer).
func explainFailure(it InboxItem, a InboxAction, err error, stage string, wrote []string) inboxWhy {
	n, dev := it.Number, or(it.f.devBr, "dev")
	c := causeOf(err)
	_, not := inboxVerb(it, a)
	var landed, maybe []string
	for _, w := range wrote {
		if m, ok := strings.CutPrefix(w, "may have "); ok {
			maybe = append(maybe, m)
		} else {
			landed = append(landed, w)
		}
	}
	w := inboxWhy{Detail: []string{"Stopped at: " + inboxStepOf(stage).line}}
	if c.request != "" {
		w.Detail = append(w.Detail, fmt.Sprintf("GitHub answered %d to %s", c.status, c.request))
	}
	w.Detail = append(w.Detail, "Error: "+err.Error())
	if it.f.head != "" {
		w.Detail = append(w.Detail, "Head commit: "+short(it.f.head))
	}
	switch {
	case len(landed) == 0 && len(maybe) == 0:
		w.Changed = "Nothing was changed on GitHub."
	case len(maybe) == 0:
		w.Changed = "Already on GitHub: it " + joinWords(landed) + "."
	case len(landed) == 0:
		w.Changed = "GitHub didn't answer, so it may have " + joinWords(maybe) + "; nothing else was changed."
	default:
		w.Changed = "Already on GitHub: it " + joinWords(landed) + "; GitHub didn't answer one more, so it may also have " + joinWords(maybe) + "."
	}
	// A merge that landed: the work is in dev; only a step after it failed.
	if slices.Contains(landed, "merged the pull request") {
		w.Landed = true
		w.What = fmt.Sprintf("#%d is merged into %s, but a step after the merge didn't finish (%s).", n, dev, strings.ToLower(inboxStepOf(stage).line))
		if reLedger.MatchString(c.message) {
			w.What = fmt.Sprintf("#%d is merged into %s, but its ship record didn't reach the ledger.", n, dev)
		}
		story := ""
		if it.Story > 0 {
			story = fmt.Sprintf(" on Story #%d", it.Story)
		}
		w.Next = fmt.Sprintf("nothing to undo; an agent finishes the record%s from the details (d). If it stays open it shows under Housekeeping.", story)
		return w
	}
	retry := "Enter tries again"
	switch m := c.message; {
	case c.budget:
		w.What = fmt.Sprintf("#%d %s: this session used up its GitHub calls.", n, not)
		w.Next = "you quit (q) and open aayu inbox again; it starts a fresh session."
	case c.rate:
		w.What = fmt.Sprintf("#%d %s: GitHub is limiting requests from this account for now.", n, not)
		w.Next = "wait a few minutes, then " + retry + "."
		if !c.reset.IsZero() {
			w.Next = fmt.Sprintf("wait until %s, then %s.", c.reset.Local().Format("15:04"), retry)
		}
	case c.network && len(maybe) > 0:
		w.What = fmt.Sprintf("#%d %s: the connection to GitHub dropped in the middle.", n, not)
		w.Next = "you open it on GitHub (o) to check what landed, then Enter finishes the rest."
	case c.network:
		w.What = fmt.Sprintf("#%d %s: aayu couldn't reach GitHub (the network dropped or timed out).", n, not)
		w.Next = "you check the connection; " + retry + "."
	case c.status == 405 && strings.Contains(strings.ToLower(m), "not mergeable"), c.status == 405 && strings.Contains(strings.ToLower(m), "conflict"):
		w.What = fmt.Sprintf("#%d wasn't merged: it conflicts with %s.", n, dev)
		w.Next = fmt.Sprintf("its builder rebases it onto %s; nothing for you yet.", dev)
	case c.status == 405 && strings.Contains(strings.ToLower(m), "out of date"):
		w.What = fmt.Sprintf("#%d wasn't merged: it is behind %s, and %s only takes pull requests that are up to date.", n, dev, dev)
		w.Next = fmt.Sprintf("its builder updates it from %s; nothing for you yet.", dev)
	case c.status == 405 && strings.Contains(strings.ToLower(m), "review"):
		w.What = fmt.Sprintf("#%d wasn't merged: GitHub wants an approving review first.", n)
		w.Next = "you approve it on GitHub (o opens it), then Enter again."
	case c.status == 405 && strings.Contains(strings.ToLower(m), "status check"):
		w.What = fmt.Sprintf("#%d wasn't merged: GitHub still waits for a required check.", n)
		w.Next = "nothing for you; it comes back as Ready to ship when its checks are green."
	case c.status == 405:
		w.What = fmt.Sprintf("#%d wasn't merged: GitHub refused the merge.", n)
		w.Next = "you open it on GitHub (o) to see why; d shows GitHub's own words."
	case c.status == 409 && strings.Contains(strings.ToLower(m), "head branch was modified"):
		w.What = fmt.Sprintf("#%d wasn't merged: new commits were pushed while it was merging.", n)
		w.Next = "its builder runs aayu ship on the new commits; it comes back here when it is ready."
	case c.status == 409:
		w.What = fmt.Sprintf("#%d %s: it changed on GitHub at the same moment.", n, not)
		w.Next = "R reads it again; then Enter tries once more."
	case c.status == 422:
		w.What = fmt.Sprintf("#%d %s: GitHub turned the change down as not valid.", n, not)
		w.Next = "Enter tries once more; if it fails again, an agent looks at the details (d)."
	case c.status == 401:
		w.What = fmt.Sprintf("#%d %s: GitHub no longer accepts this session's sign-in.", n, not)
		w.Next = "you quit (q) and open aayu inbox again to sign in."
	case c.status == 403:
		w.What = fmt.Sprintf("#%d %s: your GitHub account isn't allowed to do this here.", n, not)
		w.Next = "a repository admin gives your account access, or someone who has it does this."
	case c.status == 404:
		w.What = fmt.Sprintf("#%d %s: GitHub can't find it any more (deleted, moved, or hidden from this account).", n, not)
		w.Next = "R reads the inbox again; nothing for you on this one."
	case c.status >= 500:
		w.What = fmt.Sprintf("#%d %s: GitHub had a problem on its side.", n, not)
		w.Next = "nothing for you to fix; Enter tries again in a minute."
	case c.refusal != "":
		w.What, w.Next = refusalWords(it, a, c.refusal, dev)
	default:
		w.What = fmt.Sprintf("#%d %s: something went wrong inside aayu.", n, not)
		w.Next = "Enter tries once more; if it fails again, an agent looks at the details (d)."
	}
	w.Retry = strings.Contains(w.Next, "Enter")
	return w
}

// refusalWords are the words for a refusal by ship, decide, or inspect
// --approve, which check before they write.
func refusalWords(it InboxItem, a InboxAction, m, dev string) (what, next string) {
	n := it.Number
	_, not := inboxVerb(it, a)
	switch {
	case reHeadMoved.MatchString(m):
		return fmt.Sprintf("#%d %s: new commits were pushed after you opened it.", n, not),
			"its builder runs aayu ship on the new commits; it comes back here when it is ready."
	case reAlreadyMerged.MatchString(m):
		return fmt.Sprintf("#%d is already merged; there was nothing left to ship.", n), "nothing; R reads the inbox again."
	case reClosed.MatchString(m):
		return fmt.Sprintf("#%d is closed, so there was nothing to do.", n), "nothing; R reads the inbox again."
	case reNoStory.MatchString(m):
		return fmt.Sprintf("#%d %s: it names no Story, and ship only merges a Story's pull request.", n, not),
			"its builder adds the Story it delivers (Refs #<story>) to the pull request."
	case reNoPack.MatchString(m):
		return fmt.Sprintf("#%d %s: the merge gate's proof for its latest commit is missing or expired.", n, not),
			"its builder re-runs the merge gate (a push or a re-run); nothing for you yet."
	case rePack.MatchString(m):
		return fmt.Sprintf("#%d %s: its proof doesn't match what the Story asked for.", n, not), "its builder fixes the proof; nothing for you yet."
	case reFloors.MatchString(m):
		return fmt.Sprintf("#%d %s: a quality floor fails in its proof, such as test coverage.", n, not),
			"its builder fixes it, or you accept a floor waiver Decision."
	case rePlan.MatchString(m):
		return fmt.Sprintf("#%d %s: it changes a part of the code that needs an approved plan first.", n, not),
			"its builder drafts the plan; then you approve it (aayu plan <story> --approve)."
	case reConflicts.MatchString(m):
		return fmt.Sprintf("#%d %s: it conflicts with %s.", n, not, dev), fmt.Sprintf("its builder rebases it onto %s; nothing for you yet.", dev)
	case reMergeUnknown.MatchString(m):
		return fmt.Sprintf("#%d %s: GitHub is still working out whether it can merge, so nothing was merged.", n, not), "Enter tries again in a minute."
	case reBypassPost.MatchString(m):
		return fmt.Sprintf("#%d %s: aayu couldn't post your Solo-mode bypass note, so nothing was merged.", n, not), "Enter tries again; if it fails again, an agent looks at the details (d)."
	case reBypass.MatchString(m):
		return fmt.Sprintf("#%d %s: it needs your Solo-mode bypass note on the pull request first.", n, not), "Enter posts it and ships again."
	case reNoPass.MatchString(m):
		return fmt.Sprintf("#%d %s: it needs an approving review on GitHub first.", n, not), "you approve its latest commit on GitHub (o opens it), then Enter again."
	case reChanged.MatchString(m):
		return fmt.Sprintf("#%d %s: it or %s changed while ship was checking it, so nothing was merged.", n, not, dev), "R reads it again; then Enter tries once more."
	case reSecondHuman.MatchString(m):
		who := "another person"
		if s := reSecondHuman.FindStringSubmatch(m); s[1] != "" {
			who = "@" + s[1]
		}
		return fmt.Sprintf("#%d %s: solo mode doesn't apply here, so your own pass can't merge it.", n, not), "you ask " + who + " for an approving review on GitHub."
	case reAuthority.MatchString(m):
		return fmt.Sprintf("#%d %s: this Decision is for %s to accept.", n, not, reAuthority.FindStringSubmatch(m)[1]), "you ask them, or change who the authority is."
	case reNotReady.MatchString(m):
		return fmt.Sprintf("#%d %s: the Decision isn't complete enough to accept.", n, not), "you open it on GitHub (o) and complete it."
	case reApprover.MatchString(m):
		return fmt.Sprintf("#%d %s: only %s's review counts here.", n, not, reApprover.FindStringSubmatch(m)[1]), "you ask them to approve it."
	case reNoApprover.MatchString(m):
		return fmt.Sprintf("#%d %s: no approver is named for this repository.", n, not), "a person names one under [autonomy] approver in .aayudha/config.toml."
	case reBatchOpen.MatchString(m):
		return fmt.Sprintf("#%d %s: another approval request is still open on it.", n, not), "you answer that one on GitHub first (o opens it)."
	case reCheck.MatchString(m):
		s := reCheck.FindStringSubmatch(m)
		name, state := checkName(s[1]), s[2]
		if slices.Contains([]string{"queued", "in_progress", "pending", "waiting", "requested"}, state) {
			return fmt.Sprintf("#%d %s: its %s check is still running.", n, not, name), "nothing for you; it comes back as Ready to ship when it is green."
		}
		if state == "missing" {
			return fmt.Sprintf("#%d %s: its %s check hasn't run on its latest commit.", n, not, name), "its builder re-runs its checks; nothing for you yet."
		}
		return fmt.Sprintf("#%d %s: its %s check failed on its latest commit.", n, not, name), "its builder fixes it; nothing for you yet."
	case reLedger.MatchString(m):
		return fmt.Sprintf("#%d %s: aayu couldn't read or write its records (the ledger branch).", n, not), "Enter tries once more; if it fails again, an agent runs aayu doctor."
	}
	return fmt.Sprintf("#%d %s: aayu stopped before writing anything it wasn't sure of.", n, not), "you read the details (d); an agent can act on them."
}

// changedWords says how an item changed on GitHub since the person read it.
func changedWords(was, now InboxItem) string {
	switch {
	case now.Number == 0:
		return "it no longer waits for you: someone or something already handled it"
	case was.Head != "" && now.Head != "" && was.Head != now.Head, was.f.head != "" && now.f.head != "" && was.f.head != now.f.head:
		return "new commits were pushed since you opened it"
	case now.waiting() && !was.waiting():
		return "it " + strings.TrimSuffix(now.Why, ".")
	case was.Kind != now.Kind:
		return "it is now " + strings.ToLower(now.kind().Word)
	case !slices.EqualFunc(was.Options, now.Options, func(a, b InboxAction) bool { return a.Label == b.Label && a.Recommended == b.Recommended }):
		return "its choices were edited"
	case was.Title != now.Title:
		return "its title was edited"
	case was.OneWay != now.OneWay:
		return nowOr(now.OneWay, "it is %s a one-way door")
	case was.BodyHash != now.BodyHash:
		return "its text was edited"
	}
	return "its details changed"
}

// plainCause is why a read of GitHub failed, in a few plain words that fit
// a status line: no commit, link, or HTTP words.
func plainCause(err error) string {
	c := causeOf(err)
	switch {
	case c.budget:
		return "this session used its GitHub calls"
	case c.rate:
		return "GitHub is limiting requests"
	case c.network:
		return "GitHub couldn't be reached"
	case c.status == 401:
		return "GitHub no longer accepts this sign-in"
	case c.status == 403:
		return "this account may not read it"
	case c.status >= 500:
		return "GitHub had a problem"
	case c.status != 0:
		return "GitHub turned the read down"
	}
	return "aayu hit an error"
}

// signinWords is a failed sign-in in a person's words (presence.go's
// refusals): what happened and that nothing was done.
func signinWords(err error) string {
	m := err.Error()
	switch {
	case strings.Contains(m, "the code expired"):
		return "The code expired before it was approved. Nothing was done."
	case strings.Contains(m, "the request was denied"):
		return "The request was cancelled on GitHub. Nothing was done."
	case strings.Contains(m, "a bot identity"):
		return "A bot account approved it, and only a person can. Nothing was done; approve it signed in to GitHub as yourself."
	case strings.Contains(m, "but aayu acts as"):
		s := regexp.MustCompile(`approved by @(\S+), but aayu acts as @(\S+) here`).FindStringSubmatch(m)
		if s != nil {
			return fmt.Sprintf("@%s approved it, but this terminal acts as @%s. Nothing was done; approve it signed in to GitHub as @%s.", s[1], strings.TrimSuffix(s[2], ";"), strings.TrimSuffix(s[2], ";"))
		}
		return "Another GitHub account approved it than the one this terminal acts as. Nothing was done."
	case strings.Contains(m, " access; @"):
		s := regexp.MustCompile(`needs (\S+) access; @(\S+) has (\S+)`).FindStringSubmatch(m)
		if s != nil {
			return fmt.Sprintf("@%s has %s access here, and the inbox needs %s. Nothing was done; a repository admin can give @%s %s access.", s[2], s[3], s[1], s[2], s[1])
		}
	case strings.Contains(m, "no GitHub token"):
		return "aayu has no GitHub sign-in on this machine yet. Nothing was done."
	case strings.Contains(m, "was not approved; nothing was done"):
		return "You left the sign-in before it was approved. Nothing was done."
	case strings.Contains(m, "recording the approval"):
		return "aayu couldn't record the approval on this machine (its actions log). Nothing was done."
	}
	return "GitHub gave no approval: " + plainCause(err) + ". Nothing was done."
}
