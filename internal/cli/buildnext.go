package cli

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/loops"
	"github.com/rahulnakmol/agentic-ayudha/internal/ready"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// BuildLoop is the loop that builds Ready items on its own
// (loops/build-ready.toml). Its definition in the repository is the person's
// standing ask: `aayu build --next` runs only where it exists.
const BuildLoop = "build-ready"

// BuildPick is what `aayu build --next` decided, with no model: the loop's
// open pull requests to ship, what it has in flight against [build]
// in_flight, the stale claims it blocked, and the queue it picked from.
type BuildPick struct {
	Mode     string          `json:"mode"` // ship or pick
	Limit    int             `json:"in_flight_limit"`
	InFlight []LoopItem      `json:"in_flight"`
	Ship     []LoopItem      `json:"ship"`              // the loop's open PRs, oldest first
	Order    string          `json:"order,omitempty"`   // what ordered the queue
	Release  *PickedRelease  `json:"release,omitempty"` // the current release's baseline, when one is accepted
	Stale    []string        `json:"stale,omitempty"`   // claims blocked because their lease ran out with no PR
	Skipped  []string        `json:"skipped,omitempty"` // queue items passed over, each with why
	Picked   int             `json:"picked,omitempty"`
	Claim    string          `json:"claim,omitempty"` // the pick's line in LoopClaimsPath, kept on the build state
	claimed  map[string]bool // claim tokens already on a loop build state
	Reason   string          `json:"reason"`
}

// LoopItem is a story or bug the loop claimed: its PR, or its live claim.
type LoopItem struct {
	Story int    `json:"story"`
	PR    int    `json:"pr,omitempty"`
	URL   string `json:"url,omitempty"`
	Claim string `json:"claim,omitempty"` // holder until expiry, while there is no PR yet
}

// PickedRelease is the accepted baseline the queue follows.
type PickedRelease struct {
	Version  string `json:"version"`
	Issue    int    `json:"release_issue"`
	Baseline int    `json:"baseline"`
}

// The ledger shapes of `aayu scope` (#676) that the pick reads, by path:
// releases/<vX.Y.Z>/baseline-<n>.json. Like `aayu brain checkpoint`, build
// reads the file contract and does not import the scope package.
const scopeBaselineSchema = "aayudha.scope.baseline/v1"

var baselinePathRE = regexp.MustCompile(`^releases/(v([0-9]+)\.([0-9]+)\.([0-9]+))/baseline-([1-9][0-9]*)\.json$`)

type scopeBaseline struct {
	Schema       string `json:"schema"`
	Version      string `json:"version"`
	ReleaseIssue int    `json:"release_issue"`
	Baseline     int    `json:"baseline"`
	Items        []struct {
		Number int    `json:"number"`
		Type   string `json:"type"`
	} `json:"items"`
}

var (
	refsRE      = regexp.MustCompile(`(?m)\bRefs #([0-9]+)\b`)
	storyHeadRE = regexp.MustCompile(`^story/([0-9]+)-`)
)

// openStoryPRs maps each story to its open pull requests, oldest first: each
// one whose body says Refs #n, or whose head is build's story/<n>- branch.
func openStoryPRs(ctx context.Context, c *github.Client, owner, name string) (map[int][]*gh.PullRequest, error) {
	prs, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.PullRequest, *gh.Response, error) {
		return c.REST.PullRequests.List(ctx, owner, name, &gh.PullRequestListOptions{State: "open", ListOptions: o})
	})
	if err != nil {
		return nil, err
	}
	slices.SortFunc(prs, func(a, b *gh.PullRequest) int { return a.GetNumber() - b.GetNumber() })
	out := map[int][]*gh.PullRequest{}
	for _, pr := range prs {
		var stories []int
		for _, m := range refsRE.FindAllStringSubmatch(pr.GetBody(), -1) {
			n, _ := strconv.Atoi(m[1])
			stories = append(stories, n)
		}
		if m := storyHeadRE.FindStringSubmatch(pr.GetHead().GetRef()); m != nil {
			n, _ := strconv.Atoi(m[1])
			stories = append(stories, n)
		}
		for _, n := range slices.Compact(slices.Sorted(slices.Values(stories))) {
			out[n] = append(out[n], pr)
		}
	}
	return out, nil
}

// loopStates are the build states on the ledger that the loop claimed.
func loopStates(ctx context.Context, w *ledger.Writer) ([]BuildState, error) {
	t, err := w.Read(ctx)
	if errors.Is(err, ledger.ErrNoLedger) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer t.Close()
	paths, err := t.List(ctx, "build")
	if err != nil {
		return nil, err
	}
	var out []BuildState
	for _, p := range paths {
		if !strings.HasSuffix(p, "/state.json") {
			continue
		}
		b, err := t.File(ctx, p)
		if err != nil {
			return nil, err
		}
		var st BuildState
		if json.Unmarshal(b, &st) == nil && st.Loop != "" && st.Story > 0 {
			out = append(out, st)
		}
	}
	slices.SortFunc(out, func(a, b BuildState) int { return a.Story - b.Story })
	return out, nil
}

// currentBaseline is the newest accepted baseline of the open Release with
// the lowest version; nil when no open Release has one.
func currentBaseline(ctx context.Context, c *github.Client, w *ledger.Writer, owner, name string) (*scopeBaseline, error) {
	t, err := w.Read(ctx)
	if errors.Is(err, ledger.ErrNoLedger) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer t.Close()
	paths, err := t.List(ctx, "releases")
	if err != nil {
		return nil, err
	}
	type ver struct {
		v    string
		key  []int
		n    int
		path string
	}
	newest := map[string]*ver{}
	for _, p := range paths {
		m := baselinePathRE.FindStringSubmatch(p)
		if m == nil {
			continue
		}
		key := make([]int, 3)
		for i := range key {
			key[i], _ = strconv.Atoi(m[i+2])
		}
		n, _ := strconv.Atoi(m[5])
		if cur := newest[m[1]]; cur == nil || n > cur.n {
			newest[m[1]] = &ver{v: m[1], key: key, n: n, path: p}
		}
	}
	vs := make([]*ver, 0, len(newest))
	for _, v := range newest {
		vs = append(vs, v)
	}
	slices.SortFunc(vs, func(a, b *ver) int { return slices.Compare(a.key, b.key) })
	for _, v := range vs {
		raw, err := t.File(ctx, v.path)
		if err != nil {
			return nil, err
		}
		var b scopeBaseline
		if err := json.Unmarshal(raw, &b); err != nil || b.Schema != scopeBaselineSchema || b.ReleaseIssue == 0 {
			return nil, fmt.Errorf("ledger %s: not an accepted scope baseline (%s)", v.path, scopeBaselineSchema)
		}
		is, _, err := c.REST.Issues.Get(ctx, owner, name, b.ReleaseIssue)
		if err != nil {
			return nil, err
		}
		if is.GetState() == "open" {
			if b.Version == "" {
				b.Version = v.v
			}
			return &b, nil
		}
	}
	return nil, nil
}

// pickLockTTL bounds a pick's local lock: a run that died holding it
// leaves it, and the next run takes it over after this long.
const pickLockTTL = 15 * time.Minute

// lockPick holds this checkout's pick lock from the queue read through the
// claim and the worktree. Two runs on one machine share a lease holder
// (login@host), so the lease alone cannot tell them apart; the second run
// stops here instead of building the same story.
func (e *Env) lockPick() (func(), error) {
	p := filepath.Join(e.stateDir(), "state", "build-next.lock")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return nil, err
	}
	if fi, err := os.Stat(p); err == nil && e.Now().Sub(fi.ModTime()) > pickLockTTL {
		_ = os.Remove(p)
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if errors.Is(err, os.ErrExist) {
		return nil, Errorf(ExitBlocked, "build --next: another build --next run on this machine holds %s; this run claims nothing", p)
	}
	if err != nil {
		return nil, err
	}
	_, _ = fmt.Fprintf(f, "pid %d at %s\n", os.Getpid(), e.Now().UTC().Format(time.RFC3339))
	_ = f.Close()
	return func() { _ = os.Remove(p) }, nil
}

// LoopClaimsPath is the ledger file that orders the loop's claims across
// machines: each pick appends a line before it claims, and the ledger's
// single history decides which of two concurrent picks came first.
const LoopClaimsPath = "build/loop-claims.jsonl"

type loopClaim struct {
	Story   int    `json:"story"`
	Token   string `json:"token"`
	At      string `json:"at"`
	Release bool   `json:"release,omitempty"`
}

// admitClaim reserves a slot for story n on the ledger, in LoopClaimsPath
// against [build] in_flight; it says why when this pick lost.
func (e *Env) admitClaim(ctx context.Context, w *ledger.Writer, p *BuildPick, n int) (string, error) {
	busy := map[int]bool{}
	for _, it := range p.InFlight {
		busy[it.Story] = true
	}
	tok, lost, err := e.claimSlot(ctx, w, LoopClaimsPath, fmt.Sprintf("build: loop claim of #%d", n), n, busy, p.Limit, p.claimed)
	switch {
	case err != nil:
		return "", err
	case lost == n:
		return fmt.Sprintf("another pick claimed #%d first (ledger %s): nothing claimed", n, LoopClaimsPath), nil
	case lost != 0:
		return fmt.Sprintf("another pick took the last of %d slot(s) first (#%d, ledger %s): nothing claimed", p.Limit, lost, LoopClaimsPath), nil
	}
	p.Claim = tok
	return "", nil
}

// claimSlot appends this run's claim line for story n to the ledger file
// path, then reads the file back: the live lines before it (claimLines) for
// stories not in busy are runs still claiming. When one of them is for n, or
// they and busy fill limit, this run lost: it releases its line and returns
// the story it lost to, n or the one that took the last slot. Otherwise it
// returns its token.
func (e *Env) claimSlot(ctx context.Context, w *ledger.Writer, path, msg string, n int, busy map[int]bool, limit int, carried map[string]bool) (string, int, error) {
	tok := make([]byte, 8)
	_, _ = rand.Read(tok)
	me := loopClaim{Story: n, Token: hex.EncodeToString(tok), At: e.Now().UTC().Format(time.RFC3339)}
	write := func(c loopClaim) error {
		b, _ := json.Marshal(c)
		_, err := w.Write(ctx, ledger.Commit{Message: msg, Append: map[string][]byte{path: append(b, '\n')}})
		return err
	}
	if err := write(me); err != nil {
		return "", 0, Wrap(ExitExternal, err)
	}
	lines, err := e.claimLines(ctx, w, path, carried)
	if err != nil {
		return "", 0, err
	}
	counted := maps.Clone(busy)
	ahead := 0
	for _, c := range lines {
		if c.Token == me.Token {
			break
		}
		lost := 0
		switch {
		case c.Story == n:
			lost = n
		case !counted[c.Story]:
			counted[c.Story] = true
			ahead++
			if len(busy)+ahead >= limit {
				lost = c.Story
			}
		}
		if lost != 0 {
			me.Release = true
			if err := write(me); err != nil {
				return "", 0, Wrap(ExitExternal, err)
			}
			return "", lost, nil
		}
	}
	return me.Token, 0, nil
}

// claimLines are the live lines of the claims file at path, in ledger order:
// not released, not carried by a build state, and younger than pickLockTTL.
func (e *Env) claimLines(ctx context.Context, w *ledger.Writer, path string, carried map[string]bool) ([]loopClaim, error) {
	t, err := w.Read(ctx)
	if err != nil {
		return nil, Wrap(ExitExternal, ignoreMissing(err))
	}
	raw, err := t.File(ctx, path)
	t.Close()
	if err != nil {
		return nil, Wrap(ExitExternal, ignoreMissing(err))
	}
	return e.liveClaims(raw, carried), nil
}

// liveClaims are the live lines of a claims file's bytes, as claimLines says.
func (e *Env) liveClaims(raw []byte, carried map[string]bool) []loopClaim {
	var lines []loopClaim
	released := map[string]bool{}
	sc := bufio.NewScanner(bytes.NewReader(raw))
	for sc.Scan() {
		var c loopClaim
		if json.Unmarshal(sc.Bytes(), &c) != nil {
			continue
		}
		if c.Release {
			released[c.Token] = true
			continue
		}
		lines = append(lines, c)
	}
	return slices.DeleteFunc(lines, func(c loopClaim) bool {
		at, err := time.Parse(time.RFC3339, c.At)
		return err != nil || released[c.Token] || carried[c.Token] || e.Now().Sub(at) > pickLockTTL
	})
}

// StaleGrace is how long after its lease runs out a loop claim with no PR
// waits before the sweep blocks it: one more lease term, so a builder that
// resumes within 4 hours of the expiry (after a usage-limit wait, a paused
// orb, or the next morning's first run) keeps its story. It narrows the race
// between the sweep and a resuming builder; it does not close it.
const StaleGrace = LeaseTTL

// lastActivity is when a build state last moved: its newest checkpoint, else
// its start. A claim with no lock comment left is taken to have expired one
// lease term after it, or at its released lease's expiry when that is later.
func lastActivity(st BuildState) time.Time {
	t, _ := time.Parse(time.RFC3339, st.StartedAt)
	for _, cp := range st.Checkpoints {
		if at, err := time.Parse(time.RFC3339, cp.At); err == nil && at.After(t) {
			t = at
		}
	}
	return t
}

// priorityRank orders P0 first and an item with no priority last.
func priorityRank(labels []string) int {
	for _, l := range labels {
		if p, ok := strings.CutPrefix(l, "priority:"); ok {
			if i := slices.Index(workitems.Priorities, p); i >= 0 {
				return i
			}
		}
	}
	return len(workitems.Priorities)
}

// buildable is the type check of the queue: only Stories and Bugs are built.
func buildable(is *gh.Issue) bool {
	t, _ := workitems.TypeOf(is.GetType().GetName(), github.IssueLabels(is))
	return t == "story" || t == "bug"
}

// pickNext is `aayu build --next` (the build-ready loop). Mode ship lists the
// loop's open pull requests and writes nothing. Mode pick first blocks every
// loop claim whose lease ran out with no pull request (it is never retried
// until a person makes it Ready again), then, while the loop's open pull
// requests and live claims are fewer than [build] in_flight, returns the
// first item of the queue that is open, status:ready, passes Ready with an
// undrifted snapshot, and has no open pull request and no live lease. The
// queue is the current release's accepted baseline in its order when there is
// one, and nothing outside it; otherwise every status:ready Story and Bug by
// priority, then age. It returns 0 when it picked nothing; runBuild then
// claims the pick with the lease, as for any build.
func (e *Env) pickNext(ctx context.Context, c *github.Client, w *ledger.Writer, l *config.Loaded, repo, mode string) (*BuildPick, int, error) {
	// The loop's definition and its [build] in_flight are a person's standing
	// ask, so they are read from the integration branch's tip on GitHub, as
	// ship reads its policy: never from the checkout, AAYU_* or --set, which
	// the agent running the loop controls. A local in_flight may only lower it.
	owner, name, _ := github.SplitRepo(repo)
	pol, dev, err := e.loopPolicy(ctx, c, owner, name)
	if err != nil {
		return nil, 0, err
	}
	def, _, _, err := c.REST.Repositories.GetContents(ctx, owner, name, "loops/"+BuildLoop+".toml", &gh.RepositoryContentGetOptions{Ref: pol.Tip})
	if github.NotFound(err) {
		return nil, 0, Errorf(ExitUsage, "build --next runs only for the %s loop: %s has no loops/%s.toml at %s (`aayu install --loops` writes it; a person merges it)", BuildLoop, dev, BuildLoop, short(pol.Tip))
	} else if err != nil {
		return nil, 0, Wrap(ExitExternal, err)
	}
	if text, err := def.GetContent(); err != nil {
		return nil, 0, Wrap(ExitExternal, err)
	} else if _, err := loops.Parse(BuildLoop, []byte(text)); err != nil {
		return nil, 0, Errorf(ExitUsage, "loops/%s.toml on %s at %s: %v", BuildLoop, dev, short(pol.Tip), err)
	}
	p := &BuildPick{Mode: mode, Limit: min(l.Config.Build.InFlight, pol.Config.Build.InFlight), InFlight: []LoopItem{}, Ship: []LoopItem{}}
	prs, err := openStoryPRs(ctx, c, owner, name)
	if err != nil {
		return nil, 0, Wrap(ExitExternal, err)
	}
	states, err := loopStates(ctx, w)
	if err != nil {
		return nil, 0, Wrap(ExitExternal, err)
	}
	p.claimed = map[string]bool{}
	for _, st := range states {
		if st.Claim != "" {
			p.claimed[st.Claim] = true
		}
	}
	now := e.Now()
	for _, st := range states {
		is, _, err := c.REST.Issues.Get(ctx, owner, name, st.Story)
		if err != nil {
			return nil, 0, Wrap(ExitExternal, err)
		}
		// A block counts when the ledger or the story's label records it: the
		// label is set first, so a failed ledger write still leaves it.
		blocked := st.Blocked != "" || contains(github.IssueLabels(is), "status:blocked")
		// The loop's own PR is the one on the branch it built, in this
		// repository; any other PR that names the story is never shipped here.
		// It holds its slot while it is open, even if its story closed or its
		// build was blocked; a blocked build's PR waits for a person, unshipped.
		i := slices.IndexFunc(prs[st.Story], func(pr *gh.PullRequest) bool {
			return pr.GetHead().GetRef() == st.Branch && strings.EqualFold(pr.GetHead().GetRepo().GetFullName(), repo)
		})
		if i >= 0 {
			pr := prs[st.Story][i]
			it := LoopItem{Story: st.Story, PR: pr.GetNumber(), URL: pr.GetHTMLURL()}
			p.InFlight = append(p.InFlight, it)
			if !blocked {
				p.Ship = append(p.Ship, it)
			}
			continue
		}
		if blocked || is.GetState() != "open" {
			continue
		}
		labels := github.IssueLabels(is)
		lease, err := readLease(ctx, c, owner, name, st.Story)
		if err != nil {
			return nil, 0, Wrap(ExitExternal, err)
		}
		if lease != nil && lease.Expires.After(now) {
			p.InFlight = append(p.InFlight, LoopItem{Story: st.Story, Claim: lease.Holder + " until " + lease.Expires.UTC().Format(time.RFC3339)})
			continue
		}
		if !contains(labels, labelInProgress) {
			continue
		}
		// A claim whose lease ran out within StaleGrace may be a builder about
		// to resume: it keeps its slot and is not blocked yet.
		expired := lastActivity(st).Add(LeaseTTL)
		if lease != nil {
			expired = lease.Expires
		} else if released, err := releasedLeaseExpiry(ctx, c, owner, name, st.Story); err != nil {
			return nil, 0, Wrap(ExitExternal, err)
		} else if released.After(expired) {
			// gc reclaimed the lease: a renewal after the last checkpoint moved
			// the expiry later than the state shows.
			expired = released
		}
		if now.Sub(expired) < StaleGrace {
			p.InFlight = append(p.InFlight, LoopItem{Story: st.Story, Claim: fmt.Sprintf("lease ran out %s; blocked as stale after %s", expired.UTC().Format(time.RFC3339), expired.Add(StaleGrace).UTC().Format(time.RFC3339))})
			continue
		}
		if mode != "pick" {
			continue
		}
		// A stale claim: the loop's build went away before a pull request.
		why := fmt.Sprintf("the build-ready loop's claim ran out with no pull request (lease expired %s, more than %s ago)", expired.UTC().Format(time.RFC3339), StaleGrace)
		if lease != nil {
			why = fmt.Sprintf("the build-ready loop's claim ran out with no pull request (lease of %s expired %s, more than %s ago)", lease.Holder, lease.Expires.UTC().Format(time.RFC3339), StaleGrace)
		}
		if c.DryRun() {
			// markBlocked pages, and a page records itself in notify.json.
			p.Stale = append(p.Stale, fmt.Sprintf("#%d: %s; would be blocked, owner paged", st.Story, why))
			continue
		}
		st := st
		res := BuildResult{Issue: st.Story, Branch: st.Branch, Budget: e.buildUsage(l, repo, st, now)}
		if _, err := e.markBlocked(ctx, c, w, repo, st.Story, labels, &st, &res, why); err != nil {
			return nil, 0, err
		}
		p.Stale = append(p.Stale, fmt.Sprintf("#%d: %s; status:blocked, owner paged", st.Story, why))
	}
	slices.SortFunc(p.Ship, func(a, b LoopItem) int { return a.PR - b.PR })
	if mode == "ship" {
		p.Reason = fmt.Sprintf("%d of %d in flight; %d open pull request(s) of the loop to ship", len(p.InFlight), p.Limit, len(p.Ship))
		return p, 0, nil
	}
	if p.Limit <= 0 {
		p.Reason = "[build] in_flight is 0: the loop picks nothing"
		return p, 0, nil
	}
	if len(p.InFlight) >= p.Limit {
		p.Reason = fmt.Sprintf("%d of %d in flight: nothing new until one merges or stops", len(p.InFlight), p.Limit)
		return p, 0, nil
	}

	// The queue.
	base, err := currentBaseline(ctx, c, w, owner, name)
	if err != nil {
		return nil, 0, Wrap(ExitExternal, err)
	}
	var queue []*gh.Issue
	if base != nil {
		p.Release = &PickedRelease{Version: base.Version, Issue: base.ReleaseIssue, Baseline: base.Baseline}
		p.Order = fmt.Sprintf("baseline %d of %s (Release #%d), in its order", base.Baseline, base.Version, base.ReleaseIssue)
		for _, it := range base.Items {
			if it.Type != "story" && it.Type != "bug" {
				continue
			}
			is, _, err := c.REST.Issues.Get(ctx, owner, name, it.Number)
			if err != nil {
				return nil, 0, Wrap(ExitExternal, err)
			}
			queue = append(queue, is)
		}
	} else {
		p.Order = "no accepted baseline for an open Release: priority, then age"
		all, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.Issue, *gh.Response, error) {
			return c.REST.Issues.ListByRepo(ctx, owner, name, &gh.IssueListByRepoOptions{State: "open", Labels: []string{ready.LabelReady}, ListOptions: o})
		})
		if err != nil {
			return nil, 0, Wrap(ExitExternal, err)
		}
		for _, is := range all {
			if !is.IsPullRequest() {
				queue = append(queue, is)
			}
		}
		slices.SortStableFunc(queue, func(a, b *gh.Issue) int {
			if d := priorityRank(github.IssueLabels(a)) - priorityRank(github.IssueLabels(b)); d != 0 {
				return d
			}
			if d := a.GetCreatedAt().Compare(b.GetCreatedAt().Time); d != 0 {
				return d
			}
			return a.GetNumber() - b.GetNumber()
		})
	}
	// A Ready item with a live lease is a claim another pick is still setting
	// up (its state may not be on the ledger yet), or a person's build about
	// to start: it holds a slot, so two picks never fill one.
	for _, is := range queue {
		n, labels := is.GetNumber(), github.IssueLabels(is)
		if is.GetState() != "open" || !contains(labels, ready.LabelReady) || !contains(labels, LeaseLabel) ||
			slices.ContainsFunc(p.InFlight, func(it LoopItem) bool { return it.Story == n }) {
			continue
		}
		lease, err := readLease(ctx, c, owner, name, n)
		if err != nil {
			return nil, 0, Wrap(ExitExternal, err)
		}
		if lease != nil && lease.Expires.After(now) {
			p.InFlight = append(p.InFlight, LoopItem{Story: n, Claim: lease.Holder + " until " + lease.Expires.UTC().Format(time.RFC3339)})
		}
	}
	if len(p.InFlight) >= p.Limit {
		p.Reason = fmt.Sprintf("%d of %d in flight, counting fresh claims on Ready items: nothing new until one merges or stops", len(p.InFlight), p.Limit)
		return p, 0, nil
	}
	rd := &ready.Ready{GH: c, Repo: repo, TechnicalOwner: e.technicalOwner()}
	skip := func(n int, why string) { p.Skipped = append(p.Skipped, fmt.Sprintf("#%d: %s", n, why)) }
	for _, is := range queue {
		n, labels := is.GetNumber(), github.IssueLabels(is)
		switch {
		case is.GetState() != "open":
			continue // done, or dropped: not a skip
		case !buildable(is):
			skip(n, "not a Story or Bug")
			continue
		case !contains(labels, ready.LabelReady):
			if base != nil && !contains(labels, labelInProgress) {
				skip(n, "not labelled status:ready")
			}
			continue
		}
		if open := prs[n]; len(open) > 0 {
			skip(n, fmt.Sprintf("open pull request #%d", open[0].GetNumber()))
			continue
		}
		lease, err := readLease(ctx, c, owner, name, n)
		if err != nil {
			return nil, 0, Wrap(ExitExternal, err)
		}
		if lease != nil && lease.Expires.After(now) {
			skip(n, fmt.Sprintf("claimed by %s until %s", lease.Holder, lease.Expires.UTC().Format(time.RFC3339)))
			continue
		}
		chk, cis, err := rd.Check(ctx, n)
		if err != nil {
			return nil, 0, Wrap(ExitExternal, err)
		}
		why, err := e.notReady(ctx, c, repo, n, chk, github.IssueLabels(cis))
		if err != nil {
			return nil, 0, Wrap(ExitExternal, err)
		}
		if why != "" {
			skip(n, why)
			continue
		}
		// A dry run writes no claim line, so it has none to order: it previews
		// the pick as admitted.
		if c.DryRun() {
		} else if why, err := e.admitClaim(ctx, w, p, n); err != nil {
			return nil, 0, err
		} else if why != "" {
			p.Reason = why
			return p, 0, nil
		}
		p.Picked = n
		p.Reason = fmt.Sprintf("picked #%d: %s", n, p.Order)
		return p, n, nil
	}
	p.Reason = "nothing to build: no item in the queue is Ready and free (" + p.Order + ")"
	return p, 0, nil
}

// loopPolicy is the policy a loop build is judged by: the committed config
// at the integration branch's tip. As for proof (#681), that branch is
// trusted only by name from the account's user config, else the default
// branch's committed [branches] dev, else the default branch: never from the
// checkout, AAYU_* or --set, which could name a branch the agent can push.
func (e *Env) loopPolicy(ctx context.Context, c *github.Client, owner, name string) (*shipPolicy, string, error) {
	userDir, _ := e.accountConfigDir()
	user, err := config.Load(config.Sources{UserDir: userDir})
	if err != nil {
		return nil, "", Wrap(ExitUsage, fmt.Errorf("config: %w", err))
	}
	branch := user.Config.Branches.Dev
	if branch == "" {
		r, _, err := c.REST.Repositories.Get(ctx, owner, name)
		if err != nil {
			return nil, "", Wrap(ExitExternal, err)
		}
		branch = r.GetDefaultBranch()
		pol, err := basePolicy(ctx, c, owner, name, branch)
		if err != nil || pol.Config.Branches.Dev == "" || pol.Config.Branches.Dev == branch {
			return pol, branch, err
		}
		branch = pol.Config.Branches.Dev
	}
	pol, err := basePolicy(ctx, c, owner, name, branch)
	return pol, branch, err
}

// buildUsage is a story's budget use, for a progress comment written outside
// a build run (a stop, a stale claim).
func (e *Env) buildUsage(l *config.Loaded, repo string, st BuildState, now time.Time) Usage {
	b := l.Config.Autonomy.Budget
	u := Usage{MaxWall: b.WallHours, MaxCost: b.CostUSD, MaxAttempts: b.Attempts, Attempts: st.Attempts, CostUSD: e.buildCost(repo, st.Story), MeteredBy: e.meteredBy(&l.Config)}
	if started, err := time.Parse(time.RFC3339, st.StartedAt); err == nil {
		u.WallHours = now.Sub(started).Hours()
	}
	return u
}

// emitPick reports a --next run that claimed nothing.
func (e *Env) emitPick(p *BuildPick) error {
	return e.Emit(p, func(w io.Writer) error {
		writePick(w, p)
		return nil
	})
}

func writePick(w io.Writer, p *BuildPick) {
	for _, s := range p.Stale {
		fmt.Fprintf(w, "stale claim blocked: %s\n", s)
	}
	if p.Mode == "ship" {
		if len(p.Ship) == 0 {
			fmt.Fprintln(w, "the build-ready loop has no open pull request to ship")
		}
		for _, it := range p.Ship {
			fmt.Fprintf(w, "ship #%d (story #%d): aayu ship %d\n", it.PR, it.Story, it.PR)
		}
	}
	for _, s := range p.Skipped {
		fmt.Fprintf(w, "skipped %s\n", s)
	}
	fmt.Fprintf(w, "build-ready: %s\n", p.Reason)
}
