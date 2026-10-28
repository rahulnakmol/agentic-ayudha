package cli

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/seats"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

// Whether a Ready to ship card can still ship (Story #796). ship left the
// marker when everything but the person's pass held; since then dev may have
// moved, a check may have failed, or new commits may have landed. The inbox
// reads the pull request again on every refresh (GitHub's conditional request
// answers 304 when nothing changed), so a card that GitHub would refuse says
// so before Enter, in plain words, with who acts next, and offers nothing that
// would fail.

// inboxBlock is why a marked PR cannot ship now: the reason in a person's
// words (the sentence after "#n "), a few words for the list, and who acts
// next. The zero value means it can ship.
type inboxBlock struct {
	reason, short, next string
}

// mergeableWait is how long pullMergeable waits before it asks GitHub again
// whether a pull request can merge, while GitHub still computes it.
var mergeableWait = time.Second

// pullMergeable reads pull request n: its head, its base, and whether GitHub
// can merge it (the list endpoint carries no mergeable state). GitHub answers
// null and unknown for a moment after the base moves, while it works that
// out, so pullMergeable asks up to twice more; its Mergeable may still be nil.
func pullMergeable(ctx context.Context, c *github.Client, owner, name string, n int) (*gh.PullRequest, error) {
	for try := 0; ; try++ {
		p, _, err := c.REST.PullRequests.Get(ctx, owner, name, n)
		if err != nil || p.Mergeable != nil || p.GetMergeableState() != "unknown" || try == 2 {
			return p, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(mergeableWait):
		}
	}
}

// pull reads pull request n again (pullMergeable). An unknown answer alone
// never blocks a card (Story #796); ship's own checks still apply.
func (s *inboxSource) pull(ctx context.Context, n int) (*gh.PullRequest, error) {
	p, err := pullMergeable(ctx, s.c, s.owner, s.name, n)
	if err != nil {
		return nil, err
	}
	s.pulls[n] = p
	return p, nil
}

// requiredOf is every required status check of the base branch: its rules
// and its classic protection, read once a refresh, so a rule added while the
// inbox is open counts from the next one. It also notes whether they require
// a PR to be up to date with the base (s.strict). Classic protection is read
// from the branch, as ship reads it, because its own endpoint needs an admin
// token; the branch does not say whether classic protection is strict, so
// only the rules' strict policy counts.
func (s *inboxSource) requiredOf(ctx context.Context, base string) ([]*gh.RuleStatusCheck, error) {
	if s.required != nil {
		return s.required, nil
	}
	checks, strict := []*gh.RuleStatusCheck{}, false
	for rule, err := range s.c.REST.Repositories.ListRulesForBranchIter(ctx, s.owner, s.name, base, &gh.ListOptions{PerPage: 100}) {
		if err != nil {
			return nil, err
		}
		if r, ok := rule.(*gh.RequiredStatusChecksBranchRule); ok {
			checks = append(checks, r.Parameters.RequiredStatusChecks...)
			strict = strict || r.Parameters.StrictRequiredStatusChecksPolicy
		}
	}
	br, _, err := s.c.REST.Repositories.GetBranch(ctx, s.owner, s.name, base, 1)
	switch {
	case github.NotFound(err):
	case err != nil:
		return nil, err
	default:
		for _, rc := range br.GetProtection().GetRequiredStatusChecks().GetChecks() {
			checks = append(checks, &gh.RuleStatusCheck{Context: rc.Context, IntegrationID: rc.AppID})
		}
		strict = strict || br.GetProtection().GetRequiredStatusChecks().Strict
	}
	s.required, s.strict = checks, strict
	return checks, nil
}

// checkName is a check's name as a person reads it: "merge-gate", not
// "aayudha/merge-gate".
func checkName(c string) string { return strings.TrimPrefix(c, "aayudha/") }

// joinWords is "a", "a and b", or "a, b and c".
func joinWords(ws []string) string {
	if len(ws) < 2 {
		return strings.Join(ws, "")
	}
	return strings.Join(ws[:len(ws)-1], ", ") + " and " + ws[len(ws)-1]
}

// checksOf reads the head's check runs and statuses and judges every check
// ship needs: the merge gate's three and every required check of the base
// branch. The protected-paths one may also have failed, which only a person's
// pass merges past (the reason the card waits for a person at all); running
// or missing, it is not ready like any other. It returns the checks by
// state, and for the card's details every check by name and result,
// grouped by result with anything not green first.
func (s *inboxSource) checksOf(ctx context.Context, base, head string) (failed, missing, running, details []string, err error) {
	runs, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.CheckRun, *gh.Response, error) {
		r, resp, err := s.c.REST.Checks.ListCheckRunsForRef(ctx, s.owner, s.name, head, &gh.ListCheckRunsOptions{ListOptions: o})
		return r.GetCheckRuns(), resp, err
	})
	if err != nil {
		return nil, nil, nil, nil, err
	}
	statuses, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.RepoStatus, *gh.Response, error) {
		return s.c.REST.Repositories.ListStatuses(ctx, s.owner, s.name, head, &o)
	})
	if err != nil {
		return nil, nil, nil, nil, err
	}
	required, err := s.requiredOf(ctx, base)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	gate := gateCheckRuns(runs)
	var names []string
	state := map[string]string{}
	judge := func(name, st string) {
		name = termText(name) // a check's name is anyone's text
		if _, seen := state[name]; seen {
			if state[name] == "success" { // the worst state of a name wins
				state[name] = st
			}
			return
		}
		names = append(names, name)
		state[name] = st
	}
	for _, want := range gateChecks {
		st := "missing"
		if i := slices.IndexFunc(gate, func(r *gh.CheckRun) bool { return r.GetName() == want }); i >= 0 { // the newest run of a name first
			st = gate[i].GetStatus()
			if st == "completed" {
				st = gate[i].GetConclusion()
			}
		}
		judge(want, st)
	}
	wf, err := suiteWorkflows(ctx, s.c, s.owner, s.name, head, runs)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	person := false // the protected-paths check failed: the reason the card waits for a person at all
	for _, sc := range required {
		st := checkState(runs, statuses, wf, sc)
		if sc.Context == seats.GateCheck && st == "failure" {
			st, person = "success", true // only its finished failure is accepted, as ship's proofPass does
		}
		judge(sc.Context, st)
	}
	var results []string              // each result once, in the order first seen, green last
	byResult := map[string][]string{} // the checks of each result, named
	for _, name := range names {
		st := state[name]
		res := st
		if person && name == seats.GateCheck {
			res = "failure, which only your pass merges past"
		}
		if byResult[res] == nil && res != "success" {
			results = append(results, res)
		}
		byResult[res] = append(byResult[res], checkName(name))
		switch st {
		case "success":
		case "missing":
			missing = append(missing, checkName(name))
		case "queued", "in_progress", "pending", "waiting", "requested":
			running = append(running, checkName(name))
		default:
			failed = append(failed, checkName(name))
		}
	}
	if byResult["success"] != nil {
		results = append(results, "success")
	}
	for _, res := range results {
		details = append(details, strings.Join(byResult[res], ", ")+": "+res)
	}
	return failed, missing, running, details, nil
}

// readyItem is an open Story PR that ship left Ready to ship, as an inbox
// card, or nil when no one who marked it has the write access ship needs:
// anyone may comment, so a marker from any other account is text, not ship's
// word. marks are its markers by head. With a marker on the current head and
// GitHub able to merge it with every check green, it is a Ready card: ship
// it, which keeps every gate of ship. Otherwise it is a not-ready card that
// says why and who acts next, and offers only opening it.
func (s *inboxSource) readyItem(ctx context.Context, p *gh.PullRequest, marks map[string][]string, at map[string]time.Time) (*InboxItem, error) {
	trusted := map[string]bool{} // heads with a marker from an account with write access
	for head, by := range marks {
		for _, login := range by {
			ok, err := s.canWrite(ctx, login)
			if err != nil {
				return nil, err
			}
			if ok {
				trusted[head] = true
				break
			}
		}
	}
	if len(trusted) == 0 {
		return nil, nil
	}
	n := p.GetNumber()
	p, err := s.pull(ctx, n) // its mergeable state and head as they are now
	if err != nil {
		return nil, err
	}
	head := p.GetHead().GetSHA()
	story := StoryOf(p.GetBody(), "", p.GetHead().GetRef())
	it := InboxItem{Kind: InboxReady, Number: n, Title: termText(p.GetTitle()), URL: p.GetHTMLURL(), Seat: verbs.EngineeringManager, PR: n, Updated: p.GetUpdatedAt().Time,
		Evidence: p.GetHTMLURL(), Story: story, Head: head, f: inboxFacts{story: story, head: head}}
	// When ship last said Ready: its marker on this head, else its newest.
	var since time.Time
	var marked string
	for h := range trusted {
		if h == head || marked != head && at[h].After(since) {
			since, marked = at[h], h
		}
	}
	state := p.GetMergeableState()
	mergeable := "unknown"
	if p.Mergeable != nil {
		mergeable = strconv.FormatBool(*p.Mergeable)
	}
	it.Details = []string{
		fmt.Sprintf("GitHub: mergeable %s, mergeable state %s", mergeable, or(state, "unknown")),
		fmt.Sprintf("Head commit %s; ship said Ready to ship on %s at %s", short(head), short(marked), since.UTC().Format("Jan 2 15:04 UTC")),
	}
	var took []string // what dev took since ship said Ready
	for _, m := range slices.Sorted(mapKeys(s.pulls)) {
		q := s.pulls[m]
		if q.GetBase().GetRef() == s.dev && q.MergedAt != nil && q.GetMergedAt().After(since) && m != n {
			took = append(took, "#"+strconv.Itoa(m))
		}
	}
	if len(took) > 0 {
		it.Details = append(it.Details, "Merged into "+s.dev+" since: "+strings.Join(took, ", "))
	}
	var b inboxBlock
	failed, missing, running := []string(nil), []string(nil), []string(nil)
	if marked == head { // read every time: a re-run turns a green check red on the same head
		var details []string
		if failed, missing, running, details, err = s.checksOf(ctx, p.GetBase().GetRef(), head); err != nil {
			return nil, err
		}
		it.Details = append(it.Details, "Checks on "+short(head)+": "+strings.Join(details, " · "))
	}
	dev := p.GetBase().GetRef()
	tookWords := ""
	if len(took) > 0 {
		shown := took
		if len(shown) > 3 {
			shown = append(slices.Clone(took[:2]), fmt.Sprintf("%d more", len(took)-2))
		}
		tookWords = ", which took " + joinWords(shown) + " since ship checked it"
	}
	switch {
	case marked != head:
		b = inboxBlock{"can't ship yet: new commits were pushed since ship checked it, so ship has to check them first.", "new commits since ship",
			"its builder runs aayu ship again once its checks are green; nothing for you yet."}
	case p.Mergeable != nil && !*p.Mergeable || state == "dirty":
		b = inboxBlock{fmt.Sprintf("can't merge: it conflicts with %s%s.", dev, tookWords), "conflicts with " + dev,
			fmt.Sprintf("its builder rebases it onto %s; nothing for you yet.", dev)}
	case state == "behind" && s.strict: // GitHub merges a PR behind its base unless the rules require it up to date
		b = inboxBlock{fmt.Sprintf("can't merge yet: it is behind %s, and %s only takes pull requests that are up to date%s.", dev, dev, strings.Replace(tookWords, ", which took", "; "+dev+" took", 1)),
			"behind " + dev, fmt.Sprintf("its builder updates it from %s; nothing for you yet.", dev)}
	case len(failed) > 0:
		b = inboxBlock{fmt.Sprintf("can't merge: its %s %s failed on its latest commit.", joinWords(failed), plural(len(failed), "check", "checks")),
			checkName(failed[0]) + " check failed", "its builder fixes it; nothing for you yet."}
	case len(missing) > 0:
		b = inboxBlock{fmt.Sprintf("can't merge yet: its %s %s not run on its latest commit.", joinWords(missing), plural(len(missing), "check has", "checks have")),
			checkName(missing[0]) + " not run", "its builder re-runs its checks; nothing for you yet."}
	case len(running) > 0:
		b = inboxBlock{fmt.Sprintf("isn't ready yet: its checks are still running (%s).", joinWords(running)), "checks still running",
			"nothing for you; it comes back as Ready to ship when they are green."}
	}
	if b.reason == "" {
		it.Why = fmt.Sprintf("its merge gate is green on `%s` and `aayu ship` stopped only for a person's pass; Story #%d merges only through ship.", short(head), story)
		it.Options = []InboxAction{{N: 1, Recommended: true, Label: fmt.Sprintf("ship PR #%d (ship's gates still apply: in solo mode under your solo-mode bypass, otherwise once you have approved its head on GitHub)", n), Command: fmt.Sprintf("aayu ship %d", n)},
			{N: 2, Label: "open it and review it", Command: "open"}}
		return &it, nil
	}
	// The card's words name no commit or link; d shows them.
	it.Kind, it.Why, it.Next, it.f.problem, it.Evidence = InboxNotReady, b.reason, b.next, b.short, ""
	it.Details = append(it.Details, "Pull request: "+p.GetHTMLURL())
	return &it, nil
}
