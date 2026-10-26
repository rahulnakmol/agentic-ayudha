package cli

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/delegate"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/ready"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// `aayu mission next <f>` (Story #998, Decision #915 M1, F3): aayu, not the
// Kanha, decides which of a mission's stories and Bugs start now, with no
// model call. It reads the Feature's stories as they are now, on every run,
// so a story that joins a Tatkal Feature mid-Release is listed; whether it
// may be Ready is Ready's rule. The order is priority, then slice order (the
// Feature's sub-issue order), then Ready time. Each start is claimed on the
// ledger as admitClaim claims a loop pick, so two runs never start one story.

// WorkSchema is the schema of a work list: what starts now and why the rest
// waits. `aayu loop --due` (#911) returns the same shape with scope repo.
const WorkSchema = "aayudha.work/v1"

// WorkList is the aayudha.work/v1 list.
type WorkList struct {
	Schema string         `json:"schema"`
	Scope  WorkScope      `json:"scope"`
	Items  []WorkListItem `json:"items"`
	note   string         // text only: why nothing is listed
}

// WorkScope is what the list is for: a feature by its number, or the repo.
type WorkScope struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

// WorkListItem is one story or Bug: start it, or why it waits.
type WorkListItem struct {
	Key       string `json:"key"`    // "#20"
	Action    string `json:"action"` // start or waits
	Reason    string `json:"reason,omitempty"`
	Subthread string `json:"subthread,omitempty"` // the thread or task that already builds it
	detail    string // text only: "2 of 2 busy after these starts"
	bug       bool
}

// MissionClaimsPath is the ledger file that orders the starts of Feature f's
// mission across runs, as LoopClaimsPath orders the loop's picks.
func MissionClaimsPath(f int) string { return fmt.Sprintf("missions/%d/claims.jsonl", f) }

// nextEntry is a story or Bug of the list with its sort key.
type nextEntry struct {
	item    WorkListItem
	n       int
	prio    int
	slice   int
	readyAt time.Time
	free    bool // Ready and not running: a start unless the claim, the budget, or the agents hold it
}

// missionNext is `aayu mission next <f>`.
func missionNext(ctx context.Context, e *Env, f int) error {
	// The halt gate ran before the verb (root.go); the breaker stops every start here, a person's run too.
	if _, err := os.Stat(e.PauseFile()); err == nil {
		return Errorf(ExitBudget, "mission next: agent lanes are paused by the [budget] breaker (%s): nothing starts; a human clears it with `aayu resume`", e.PauseFile())
	}
	repo, c, w, err := e.missionLedger(ctx)
	if err != nil {
		return err
	}
	plan, appr, _, err := readMission(ctx, w, f)
	if err != nil {
		return err
	}
	if plan == nil || appr == nil { // Approve to build writes both in one ledger commit
		return WithNext(Errorf(ExitBlocked, "#%d has no mission plan: a person approves it to build first. Nothing starts.", f), fmt.Sprintf("aayu ready %d", f))
	}
	if changed, err := (&approveRun{e: e, c: c, w: w, n: f}).featureChanges(ctx, repo, *appr); err != nil {
		return err
	} else if len(changed) > 0 {
		return WithNext(Errorf(ExitBlocked, "#%d is not approved to build: %s. Nothing starts.", f, strings.Join(changed, "; ")), fmt.Sprintf("aayu ready %d --check", f))
	}
	host, err := e.callerTool(ctx, repo)
	if err != nil {
		return err
	}
	switch {
	case plan.StartedBy == "":
		return WithNext(Errorf(ExitBlocked, "The mission of #%d has not started: a person starts it. Nothing starts.", f), fmt.Sprintf("aayu mission start %d", f))
	case plan.State != "":
		return Errorf(ExitBlocked, "The mission of #%d %s. Nothing starts.", f, plan.State)
	case host != plan.Tool:
		return Errorf(ExitBlocked, "#%d's mission runs on %s, the tool of @%s, who started it: run this there", f, plan.Tool, plan.StartedBy)
	case plan.Pause != nil:
		return Errorf(ExitBlocked, "The mission of #%d is paused: %s. Nothing starts.", f, plan.Pause.Reason)
	}
	// Each mission's clock (Story #1015): ended stories' working totals go into the plan, and the time cards to the inbox.
	l, err := e.Config()
	if err != nil {
		return err
	}
	warn, maxH := missionLimits(&l.Config)
	clocks, err := e.missionClocks(ctx, c, w, repo, f, plan, maxH, l.Config.Identity)
	if err != nil {
		return err
	}
	if err := e.recordWorking(ctx, c, w, f, clocks); err != nil {
		return err
	}
	if err := e.missionCards(ctx, c, repo, f, clocks, warn, maxH, l.Config.Identity); err != nil {
		return err
	}
	list := WorkList{Schema: WorkSchema, Scope: WorkScope{Kind: "feature", ID: strconv.Itoa(f)}, Items: []WorkListItem{}}
	subs, err := featureItems(ctx, c, repo, f, "story", "bug") // in sub-issue order: the slice order
	if err != nil {
		return err
	}
	release, err := e.tatkalRelease(ctx, f)
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	var extra []*gh.Issue
	switch {
	case release == 0 && missionDone(plan, subs):
		list.note = fmt.Sprintf("The mission of #%d is done: nothing starts. A new Bug of #%d starts from its Release's Tatkal mission.", f, f)
		return emitWorkList(e, list)
	case release != 0:
		if extra, err = tatkalBugs(ctx, c, w, repo, f, release); err != nil {
			return err
		}
	}
	return e.listNext(ctx, c, w, repo, f, plan, appr, subs, extra, list, clocks)
}

// listNext classifies the mission's open stories and Bugs, sorts them, and
// claims each start.
func (e *Env) listNext(ctx context.Context, c *github.Client, w *ledger.Writer, repo string, f int, plan *MissionPlan, appr *ready.Approval,
	subs, extra []*gh.Issue, list WorkList, clocks []MissionClock) error {
	now := e.Now()
	owner, name, _ := github.SplitRepo(repo)
	busy := map[int]bool{}
	claims, err := e.claimLines(ctx, w, MissionClaimsPath(f), nil)
	if err != nil {
		return err
	}
	claimed := map[int]bool{}
	for _, cl := range claims {
		claimed[cl.Story], busy[cl.Story] = true, true
	}
	// A gate fix PR takes one agents slot while it is open, and its spend counts against the budget (Story #1001).
	open, err := openFixPRs(ctx, c, owner, name, plan)
	if err != nil {
		return err
	}
	for _, fx := range open {
		busy[fixKey(fx.Milestone, fx.Check, fx.DevSHA)] = true
	}
	t, err := w.Read(ctx)
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	defer t.Close()
	costs, spent := e.costs(repo), 0.0
	openSet := map[int]bool{}
	for _, fx := range open {
		openSet[fx.PR] = true
	}
	fixes, _ := fixSpend(plan, appr.Budget.PerStoryUSD, costs, openSet)
	spent += fixes
	held := map[int]bool{} // open stories that run or hold a claim: each counts at its cap until #872 measures spend per story
	var entries []nextEntry
	states := map[int]string{} // each listed item's state: a depends-on target in the list needs no read
	for _, is := range append(slices.Clone(subs), extra...) {
		states[is.GetNumber()] = is.GetState()
	}
	for i, is := range append(slices.Clone(subs), extra...) {
		n, labels := is.GetNumber(), github.IssueLabels(is)
		spent += costs[n] // the measured spend of every story and Bug, merged ones too
		if is.GetState() != "open" {
			continue
		}
		kind, _ := workitems.TypeOf(is.GetType().GetName(), labels)
		en := nextEntry{n: n, prio: priorityRank(labels), slice: min(i, len(subs)), item: WorkListItem{Key: "#" + strconv.Itoa(n), Action: "waits", bug: kind == "bug"}}
		var st *delegate.State
		if b, err := t.File(ctx, delegate.LedgerPath(n)); ignoreMissing(err) != nil {
			return Wrap(ExitExternal, err)
		} else if b != nil && json.Unmarshal(b, &st) == nil && st != nil {
			en.item.Subthread = st.Task
		}
		running := contains(labels, labelInProgress) || contains(labels, "status:validation")
		if running {
			busy[n] = true
		}
		if running || claimed[n] {
			held[n] = true
		}
		switch {
		case contains(labels, "status:blocked"):
			en.item.Reason = "blocked"
		case usagePaused(st, now) != "":
			en.item.Reason = usagePaused(st, now)
		case plan.stopped(n) != nil: // a person stopped it alone (#1002): it keeps its slot while it runs
			en.item.Reason = fmt.Sprintf("stopped (%s)", plan.stopped(n).Reason)
		case running:
			continue // it runs: the per-mission running view is #895's
		case slices.ContainsFunc(plan.StoryStarts, func(s MissionStoryStart) bool { return s.Number == n }):
			en.item.Reason = startedOn(plan, n) // a Tatkal story the Kanha only orders and tracks
		case !contains(labels, ready.LabelReady):
			en.item.Reason, en.item.detail = "not Ready", storyState(is)
		default:
			open, err := openDeps(ctx, c, owner, name, is.GetBody(), states)
			if err != nil {
				return err
			}
			if len(open) > 0 { // F19: never start before the target merges
				verb := "is"
				if len(open) > 1 {
					verb = "are"
				}
				en.item.Reason = fmt.Sprintf("depends-on %s %s open", strings.Join(open, ", "), verb)
				break
			}
			en.free = true
			if en.readyAt, err = readyAt(ctx, c, owner, name, is); err != nil {
				return Wrap(ExitExternal, err)
			}
			if contains(labels, LeaseLabel) {
				lease, err := readLease(ctx, c, owner, name, n)
				if err != nil {
					return Wrap(ExitExternal, err)
				}
				if lease != nil && lease.Expires.After(now) {
					claimed[n], busy[n], held[n] = true, true, true
				}
			}
		}
		entries = append(entries, en)
	}
	slices.SortStableFunc(entries, func(a, b nextEntry) int {
		return cmp.Or(a.prio-b.prio, a.slice-b.slice, a.readyAt.Compare(b.readyAt), a.n-b.n)
	})
	per := appr.Budget.PerStoryUSD
	agents, _, err := e.missionAgents(ctx, c, repo, f, appr.Agents)
	if err != nil {
		return err
	}
	for n := range held {
		spent += max(per-costs[n], 0)
	}
	for i := range entries {
		en := &entries[i]
		if !en.free {
			continue
		}
		switch {
		case claimed[en.n]:
			en.item.Reason = "claimed by another run"
			continue
		case heldBy(plan, clocks, en.n) != nil: // past its time limit a mission starts nothing new; running stories go on
			en.item.Reason = heldBy(plan, clocks, en.n).holdReason(f)
			continue
		case plan.Budget > 0 && spent+per > plan.Budget: // a story counts at its per-story cap until #872 measures spend per story
			en.item.Reason = fmt.Sprintf("would pass #%d's approved budget", f)
			en.item.detail = fmt.Sprintf("%s + %s > %s USD", usd(spent), usd(per), usd(plan.Budget))
			continue
		case len(busy) >= agents:
			en.item.Reason = "no free agent"
			continue
		}
		// A dry run writes no claim line: it previews the start as admitted.
		lost := 0
		if !c.DryRun() {
			if _, lost, err = e.claimSlot(ctx, w, MissionClaimsPath(f), fmt.Sprintf("missions: claim of #%d in the mission of #%d", en.n, f), en.n, busy, agents, nil); err != nil {
				return err
			}
		}
		// The story this run lost runs too: it holds its cap, as the run that won it counted.
		switch spent += per; lost {
		case 0:
			en.item.Action = "start"
			busy[en.n] = true
		case en.n:
			en.item.Reason, busy[en.n] = "claimed by another run", true
		default: // another run took the last free agent
			en.item.Reason, busy[lost] = "no free agent", true
		}
	}
	for _, en := range entries {
		if en.item.Reason == "no free agent" {
			en.item.detail = fmt.Sprintf("%d of %d busy after these starts", len(busy), agents)
		}
		list.Items = append(list.Items, en.item)
	}
	if len(list.Items) == 0 {
		list.note = fmt.Sprintf("Nothing to list: every open story and Bug of #%d runs, or none is open.", f)
	}
	return emitWorkList(e, list)
}

// dependsOnWord finds the link kind in any case, at offsets into the line itself.
var dependsOnWord = regexp.MustCompile(`(?i)depends-on`)

// openDeps are the depends-on targets in a Ready story's Links that are still
// open (Story #999, F19): "#10", one per target. The body is the one Ready
// froze, as the Ready guard removes status:ready when it drifts. A target in
// states has its state from there; aayu reads the others. A target it cannot
// read counts as open: one in another repository, a link line that ParseLinks
// misses, or an issue that is gone.
func openDeps(ctx context.Context, c *github.Client, owner, name, body string, states map[int]string) ([]string, error) {
	_, fields := workitems.Sections(body)
	var open []string
	for _, line := range strings.Split(fields["Links"], "\n") {
		links, _ := workitems.ParseLinks(line)
		switch {
		case len(links) == 0 && dependsOnWord.MatchString(line): // "Depends-On #10" too
			target := strings.Trim(line[dependsOnWord.FindStringIndex(line)[1]:], ": \t")
			if target == "" {
				target = strings.TrimSpace(line)
			}
			open = append(open, target)
		case len(links) == 0 || links[0].Kind != "depends-on":
		default:
			n := links[0].Number
			st, ok := states[n]
			if !ok {
				is, _, err := c.REST.Issues.Get(ctx, owner, name, n)
				if err != nil && !github.NotFound(err) {
					return nil, Wrap(ExitExternal, err)
				}
				st, states[n] = is.GetState(), is.GetState()
			}
			if key := "#" + strconv.Itoa(n); st != "closed" && !slices.Contains(open, key) {
				open = append(open, key)
			}
		}
	}
	return open, nil
}

func emitWorkList(e *Env, list WorkList) error {
	if e.work != nil { // mission resume reports the list in its own result
		*e.work = list
		return nil
	}
	return e.Emit(list, func(w io.Writer) error {
		for _, it := range list.Items {
			line := it.Action + " " + it.Key
			if it.bug {
				line += " (bug)"
			}
			if it.Reason != "" {
				line += ": " + it.Reason
			}
			if it.detail != "" {
				line += " (" + it.detail + ")"
			}
			if it.Subthread != "" {
				line += " · subthread " + it.Subthread
			}
			fmt.Fprintln(w, line)
		}
		if list.note != "" {
			fmt.Fprintln(w, list.note)
		}
		return nil
	})
}

func usd(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

// usagePaused says "amp paused until 20:10" while the story's delegation
// waits out its receiver's usage limit, or "".
func usagePaused(st *delegate.State, now time.Time) string {
	if st == nil || len(st.Events) == 0 {
		return ""
	}
	last := st.Events[len(st.Events)-1]
	until, err := time.Parse(time.RFC3339, last.Until)
	if last.Status != dlgPaused || err != nil || !until.After(now) {
		return ""
	}
	return fmt.Sprintf("%s paused until %s", st.Receiver, until.Local().Format("15:04"))
}

// readyAt is when the issue's newest Ready snapshot was posted; its creation
// time when it has none.
func readyAt(ctx context.Context, c *github.Client, owner, name string, is *gh.Issue) (time.Time, error) {
	cs, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.IssueComment, *gh.Response, error) {
		return c.REST.Issues.ListComments(ctx, owner, name, is.GetNumber(), &gh.IssueListCommentsOptions{ListOptions: o})
	})
	at := is.GetCreatedAt().Time
	for _, cm := range cs {
		if strings.Contains(cm.GetBody(), ready.SnapshotMarker) {
			at = cm.GetCreatedAt().Time
		}
	}
	return at, err
}

// missionDone reports whether every story and Bug of the plan's graph has
// merged (closed), so the mission has nothing left to start.
func missionDone(plan *MissionPlan, subs []*gh.Issue) bool {
	return !slices.ContainsFunc(plan.Graph, func(it MissionItem) bool {
		return slices.ContainsFunc(subs, func(is *gh.Issue) bool { return is.GetNumber() == it.Number && is.GetState() == "open" })
	})
}

// tatkalBugs are the open Bugs, under the Features of Release r other than
// its Tatkal Feature f, whose missions are done: they start from the Tatkal
// mission, and their parent does not change (Decision #915). A Feature is in
// the Release when its milestone is the Release's version.
func tatkalBugs(ctx context.Context, c *github.Client, w *ledger.Writer, repo string, f, r int) ([]*gh.Issue, error) {
	owner, name, _ := github.SplitRepo(repo)
	rel, _, err := c.REST.Issues.Get(ctx, owner, name, r)
	if err != nil {
		return nil, Wrap(ExitExternal, err)
	}
	it, _ := workitems.Parse(r, rel.GetTitle(), rel.GetBody(), rel.GetType().GetName(), github.IssueLabels(rel))
	version := strings.TrimSpace(it.Fields["version"])
	mss, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.Milestone, *gh.Response, error) {
		return c.REST.Issues.ListMilestones(ctx, owner, name, &gh.MilestoneListOptions{State: "all", ListOptions: o})
	})
	if err != nil {
		return nil, Wrap(ExitExternal, err)
	}
	i := slices.IndexFunc(mss, func(m *gh.Milestone) bool { return version != "" && m.GetTitle() == version })
	if i < 0 {
		return nil, nil
	}
	issues, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.Issue, *gh.Response, error) {
		return c.REST.Issues.ListByRepo(ctx, owner, name, &gh.IssueListByRepoOptions{Milestone: strconv.Itoa(mss[i].GetNumber()), State: "all", ListOptions: o})
	})
	if err != nil {
		return nil, Wrap(ExitExternal, err)
	}
	var out []*gh.Issue
	for _, is := range issues {
		if t, _ := workitems.TypeOf(is.GetType().GetName(), github.IssueLabels(is)); t != "feature" || is.GetNumber() == f || is.IsPullRequest() {
			continue
		}
		plan, _, _, err := readMission(ctx, w, is.GetNumber())
		if err != nil {
			return nil, err
		}
		if plan == nil {
			continue
		}
		subs, err := featureItems(ctx, c, repo, is.GetNumber(), "bug", "story")
		if err != nil {
			return nil, err
		}
		if !missionDone(plan, subs) {
			continue
		}
		for _, s := range subs {
			if t, _ := workitems.TypeOf(s.GetType().GetName(), github.IssueLabels(s)); t == "bug" && s.GetState() == "open" {
				out = append(out, s)
			}
		}
	}
	return out, nil
}

// changedSinceList refuses to start a story a mission list marked start
// when its live state changed since (Story #998, J4 rule 3): build reads the
// issue first, and releases the claim so its agent is free again. A story a
// build already moved to in progress is not a change. claimed says a list
// marked n start and its claim is live.
func (e *Env) changedSinceList(ctx context.Context, w *ledger.Writer, is *gh.Issue, pass bool, labels []string, dryRun bool) (claimed bool, err error) {
	n := is.GetNumber()
	t, err := w.Read(ctx)
	if err != nil {
		return false, Wrap(ExitExternal, ignoreMissing(err))
	}
	defer t.Close()
	paths, err := t.List(ctx, "missions")
	if err != nil {
		return false, Wrap(ExitExternal, ignoreMissing(err))
	}
	var path string
	var claim loopClaim
	for _, p := range paths {
		if !strings.HasSuffix(p, "/claims.jsonl") {
			continue
		}
		raw, err := t.File(ctx, p)
		if err != nil {
			return false, Wrap(ExitExternal, err)
		}
		lines := e.liveClaims(raw, nil)
		if i := slices.IndexFunc(lines, func(c loopClaim) bool { return c.Story == n }); i >= 0 {
			path, claim = p, lines[i]
		}
	}
	now := ""
	switch {
	case path == "":
		return false, nil // no list marked it start: build's own Ready checks apply
	case is.GetState() != "open":
		now = "closed"
	case contains(labels, "status:blocked"):
		now = "blocked"
	case !contains(labels, ready.LabelReady) && !contains(labels, labelInProgress):
		now = storyState(is)
	case !pass:
		now = "not Ready"
	default:
		return true, nil
	}
	if !dryRun {
		claim.Release = true
		b, _ := json.Marshal(claim)
		if _, err := w.Write(ctx, ledger.Commit{Message: fmt.Sprintf("missions: #%d changed since the list (%s); its claim is released", n, now), Append: map[string][]byte{path: append(b, '\n')}}); err != nil {
			return false, Wrap(ExitExternal, err)
		}
	}
	return false, Errorf(ExitBlocked, "#%d changed since the list: now %s; not started", n, now)
}

// heldBy is the clock of story n's mission when it reached its time limit, else nil.
func heldBy(plan *MissionPlan, clocks []MissionClock, n int) *MissionClock {
	for _, m := range plan.Missions {
		if !slices.ContainsFunc(m.Stories, func(s MissionStory) bool { return s.Number == n }) {
			continue
		}
		for i := range clocks {
			if clocks[i].Mission == m.Number && clocks[i].reached() {
				return &clocks[i]
			}
		}
	}
	return nil
}
