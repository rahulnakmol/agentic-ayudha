package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	gh "github.com/google/go-github/v90/github"
	"github.com/shurcooL/githubv4"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/delegate"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/identity"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/ready"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// Kanha's lock (Story #1013, Decision #974). A person who starts a mission
// confirms once that the Feature, its stories, and its open Bugs become
// read-only: their bodies and their effort:, risk:, and priority: labels.
// Comments and status: labels stay open. GitHub has no read-only issue body,
// so aayu refuses its own writes in the transport of its one GitHub client
// (lockGuard), and a mission that finds a locked body changed holds that
// issue and never writes the body back. The lock ends when the last
// milestone of the last mission is done, or at `aayu mission stop --end`.

// Lock states.
const (
	LockActive = "active"
	LockEnded  = "ended"
)

// LockMarker starts the comment a locked issue gets when its body changes
// while the mission holds it (baseline v44, 889-locked).
const LockMarker = "<!-- aayudha:lock -->"

// lockedLabels are the label prefixes a lock holds, by the key plan.json uses.
var lockedLabels = []string{"effort", "risk", "priority"}

// lockLabels are the locked labels among labels: {"priority": "priority:P1"}.
func lockLabels(labels []string) map[string]string {
	out := map[string]string{}
	for _, l := range labels {
		if p, _, ok := strings.Cut(l, ":"); ok && slices.Contains(lockedLabels, p) {
			out[p] = l
		}
	}
	return out
}

// lockNumbers are the issues the start of plan's mission locks: the Feature,
// each story of the plan, and each Bug of it that is open.
func lockNumbers(plan *MissionPlan, open func(n int) bool) []int {
	out := []int{plan.Feature}
	for _, it := range plan.Graph {
		if it.Kind != "bug" || open(it.Number) {
			out = append(out, it.Number)
		}
	}
	return out
}

// startCard is the start card of plan's mission (baseline v44, tui-start):
// the issues its start locks, what the lock holds and leaves open, when it
// ends, the tool, and how many stories run at once (missionAgents).
func startCard(plan *MissionPlan, locked []int, tool string, agents int) []string {
	last := ""
	if len(plan.Milestones) > 0 {
		last = plan.Milestones[len(plan.Milestones)-1].Name
	}
	runs := fmt.Sprintf("It runs in %s, the tool you set for this repository.", orDefault(delegate.Names[tool], tool))
	if tool == "" {
		runs = "It runs in the tool you set for this repository, and none is set there yet: Start refuses until you set one."
	}
	return []string{
		fmt.Sprintf("Starting the mission makes %d %s read-only until its last milestone: %s", len(locked), plural(len(locked), "issue", "issues"), issueRefs(locked)),
		"Locked  each body, and its effort:, risk: and priority: labels",
		"Open    comments and status: labels",
		fmt.Sprintf("Ends    at the last milestone of the last mission (%s)", last),
		runs,
		fmt.Sprintf("Agents  up to %d stories at once", agents),
	}
}

// readLocked reads the issues that the start of plan's mission locks, in the
// lock's order.
func readLocked(ctx context.Context, c *github.Client, repo string, plan *MissionPlan) ([]*gh.Issue, error) {
	owner, name, _ := github.SplitRepo(repo)
	byNumber := map[int]*gh.Issue{}
	for _, n := range lockNumbers(plan, func(int) bool { return true }) {
		is, _, err := c.REST.Issues.Get(ctx, owner, name, n)
		if err != nil {
			return nil, Wrap(ExitExternal, err)
		}
		byNumber[n] = is
	}
	var out []*gh.Issue
	for _, n := range lockNumbers(plan, func(n int) bool { return byNumber[n].GetState() == "open" }) {
		out = append(out, byNumber[n])
	}
	return out, nil
}

// newLock is the lock a person confirms at the start: each issue's body hash
// and locked labels as they are now.
func newLock(by, at string, issues []*gh.Issue) *MissionLock {
	l := &MissionLock{State: LockActive, ConfirmedBy: by, StartedAt: at, Issues: []LockedIssue{}}
	for _, is := range issues {
		l.Issues = append(l.Issues, LockedIssue{Number: is.GetNumber(), BodySHA256: workitems.BodyHash(is.GetBody()), Labels: lockLabels(github.IssueLabels(is))})
	}
	return l
}

func issueNumbers(issues []*gh.Issue) []int {
	out := make([]int, len(issues))
	for i, is := range issues {
		out[i] = is.GetNumber()
	}
	return out
}

// lockedIssue is n's entry in an active lock, or nil.
func (l *MissionLock) lockedIssue(n int) *LockedIssue {
	if i := slices.IndexFunc(l.Issues, func(li LockedIssue) bool { return li.Number == n }); i >= 0 && l.State == LockActive {
		return &l.Issues[i]
	}
	return nil
}

// lockRefusal is the refusal of a write to a locked body or label.
type lockRefusal struct{ issue, feature int }

func (r *lockRefusal) Error() string {
	return fmt.Sprintf("#%d is locked by mission #%d until it ends", r.issue, r.feature)
}

// lockRefused reports whether err is a refusal of the lock.
func lockRefused(err error) bool {
	var r *lockRefusal
	return errors.As(err, &r)
}

// lockUnread is the refusal of a guarded write when the locks cannot be read.
type lockUnread struct{ err error }

func (u *lockUnread) Error() string {
	return fmt.Sprintf("aayu reads the mission locks before it writes an issue body or an effort:, risk:, or priority: label, and could not: %v. Nothing was written.", u.err)
}

// lockHold is what an active lock holds on one issue.
type lockHold struct {
	feature int
	labels  map[string]string
}

// lockGuard is the innermost transport of the CLI's GitHub client
// (Env.newGitHub): it refuses a write that would change a locked body or a
// locked label of the checkout's repository before the request leaves. Every
// aayu write goes through it; reads, comments, and status: labels pass.
type lockGuard struct {
	next http.RoundTripper
	// locks are the checkout's repository and the issues its running missions lock.
	locks func(ctx context.Context) (string, map[int]lockHold, error)
}

// issueWrite is an issue's own endpoint or its labels: owner, name, number, /labels, and a label name.
var issueWrite = regexp.MustCompile(`/repos/([^/]+)/([^/]+)/issues/([0-9]+)(/labels(?:/([^/]+))?)?$`)

func (g *lockGuard) RoundTrip(req *http.Request) (*http.Response, error) {
	next := g.next
	if next == nil {
		next = http.DefaultTransport
	}
	m := issueWrite.FindStringSubmatch(req.URL.EscapedPath())
	if !mutatingRequest(req.Method) || m == nil {
		return next.RoundTrip(req)
	}
	change, err := issueChange(req, m[4] != "", m[5])
	if err != nil {
		return nil, err
	}
	if !change.guarded() {
		return next.RoundTrip(req)
	}
	repo, holds, err := g.locks(req.Context())
	if err != nil {
		return nil, &Error{Code: ExitExternal, Err: &lockUnread{err}}
	}
	n, _ := strconv.Atoi(m[3])
	if h, ok := holds[n]; ok && strings.EqualFold(m[1]+"/"+m[2], repo) && change.changes(h.labels) {
		return nil, &Error{Code: ExitBlocked, Err: &lockRefusal{issue: n, feature: h.feature}}
	}
	return next.RoundTrip(req)
}

func mutatingRequest(m string) bool { return m != http.MethodGet && m != http.MethodHead }

// labelChange is what a write does to an issue's body and labels.
type labelChange struct {
	body   bool
	set    []string // the issue's labels after the write (set is true)
	isSet  bool
	add    []string
	remove string
}

// guarded reports whether the write touches a body or a locked label at all.
func (c labelChange) guarded() bool {
	return c.body || c.isSet || len(lockLabels(c.add)) > 0 || len(lockLabels([]string{c.remove})) > 0
}

// changes reports whether the write changes the body or a label the lock holds as held.
func (c labelChange) changes(held map[string]string) bool {
	switch {
	case c.body:
		return true
	case c.isSet && !maps.Equal(lockLabels(c.set), held):
		return true
	case c.remove != "" && len(lockLabels([]string{c.remove})) > 0:
		return true
	}
	// Each locked label in the write, not only the last of its prefix: priority:P0 beside a held priority:P1 changes it.
	for _, l := range slices.Concat(c.set, c.add) {
		if p, _, _ := strings.Cut(l, ":"); len(lockLabels([]string{l})) > 0 && held[p] != l {
			return true
		}
	}
	return false
}

// issueChange reads what a write to an issue (PATCH) or to its labels (POST
// adds, PUT sets, DELETE removes one or all) does to its body and labels.
func issueChange(req *http.Request, labels bool, name string) (labelChange, error) {
	var c labelChange
	if labels && req.Method == http.MethodDelete {
		if name == "" {
			return labelChange{isSet: true, set: []string{}}, nil
		}
		c.remove, _ = url.PathUnescape(name)
		return c, nil
	}
	raw, err := requestBody(req)
	if err != nil || len(bytes.TrimSpace(raw)) == 0 {
		return c, err
	}
	var obj map[string]json.RawMessage
	var list json.RawMessage
	if json.Unmarshal(raw, &obj) == nil {
		if _, ok := obj["body"]; ok && !labels {
			c.body = true
		}
		list = obj["labels"]
	} else {
		list = raw
	}
	if list == nil {
		return c, nil
	}
	names := labelNames(list)
	switch {
	case !labels || req.Method == http.MethodPut: // PATCH labels and PUT replace them all
		c.isSet, c.set = true, names
	default:
		c.add = names
	}
	return c, nil
}

// labelNames reads a labels list: names, or objects with a name.
func labelNames(raw json.RawMessage) []string {
	var names []string
	if json.Unmarshal(raw, &names) == nil {
		return names
	}
	var objs []struct {
		Name string `json:"name"`
	}
	_ = json.Unmarshal(raw, &objs)
	for _, o := range objs {
		names = append(names, o.Name)
	}
	return names
}

func requestBody(req *http.Request) ([]byte, error) {
	if req.GetBody != nil {
		b, err := req.GetBody()
		if err != nil {
			return nil, err
		}
		defer func() { _ = b.Close() }()
		return io.ReadAll(b)
	}
	if req.Body == nil {
		return nil, nil
	}
	raw, err := io.ReadAll(req.Body)
	_ = req.Body.Close()
	req.Body = io.NopCloser(bytes.NewReader(raw))
	return raw, err
}

// missionLocks reads, for each guarded write, the checkout's repository and
// the issues its running missions lock, from the ledger. With no checkout or
// no ledger, nothing is locked.
func (e *Env) missionLocks(tok identity.Token, ident string) func(ctx context.Context) (string, map[int]lockHold, error) {
	return func(ctx context.Context) (string, map[int]lockHold, error) {
		repo, _, plans, err := e.ledgerPlans(ctx, tok, ident)
		if err != nil {
			return "", nil, err
		}
		holds := map[int]lockHold{}
		for _, p := range plans {
			if p.Lock == nil || p.Lock.State != LockActive {
				continue
			}
			for _, li := range p.Lock.Issues {
				holds[li.Number] = lockHold{feature: p.Feature, labels: li.Labels}
			}
		}
		return repo, holds, nil
	}
}

// ledgerPlans reads the checkout's repository, its ledger, and every mission
// plan on it. With no checkout there is no ledger, and no plan.
func (e *Env) ledgerPlans(ctx context.Context, tok identity.Token, ident string) (repo string, w *ledger.Writer, plans []*MissionPlan, err error) {
	if config.RepoRoot(e.Dir) == "" {
		return "", nil, nil, nil
	}
	if repo, err = e.RepoName(ctx); err != nil {
		return "", nil, nil, err
	}
	if w, err = e.Ledger(ctx, tok, ident, ""); err != nil {
		return "", nil, nil, err
	}
	plans, err = readPlans(ctx, w)
	return repo, w, plans, err
}

// readPlans reads every mission plan on the ledger.
func readPlans(ctx context.Context, w *ledger.Writer) ([]*MissionPlan, error) {
	t, err := w.Read(ctx)
	if errors.Is(err, ledger.ErrNoLedger) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer t.Close()
	paths, err := t.List(ctx, "missions")
	if err != nil {
		return nil, ignoreMissing(err)
	}
	var out []*MissionPlan
	for _, p := range paths {
		if !strings.HasSuffix(p, "/plan.json") {
			continue
		}
		b, err := t.File(ctx, p)
		if err != nil {
			return nil, err
		}
		var plan MissionPlan
		if json.Unmarshal(b, &plan) != nil || plan.Schema != MissionPlanSchema || MissionPlanPath(plan.Feature) != p {
			return nil, fmt.Errorf("ledger %s: not a mission plan aayu wrote", p)
		}
		out = append(out, &plan)
	}
	return out, nil
}

// lockedEdits are the open issues that plan's active lock holds whose body
// is not the one it locked, by number: each waits until a person keeps the
// edit or the mission ends. aayu never writes such a body back; it posts one
// comment on the issue for each new body (baseline v44, 889-locked). subs
// are the Feature's stories and Bugs as read now.
func (e *Env) lockedEdits(ctx context.Context, c *github.Client, w *ledger.Writer, repo string, plan *MissionPlan, subs []*gh.Issue) (map[int]bool, error) {
	changed := map[int]bool{}
	if plan.Lock == nil || plan.Lock.State != LockActive {
		return changed, nil
	}
	owner, name, _ := github.SplitRepo(repo)
	now := map[int]*gh.Issue{}
	for _, is := range subs {
		now[is.GetNumber()] = is
	}
	for _, li := range plan.Lock.Issues {
		is := now[li.Number]
		if is == nil {
			var err error
			if is, _, err = c.REST.Issues.Get(ctx, owner, name, li.Number); err != nil {
				return nil, Wrap(ExitExternal, err)
			}
		}
		hash := workitems.BodyHash(is.GetBody())
		if is.GetState() != "open" || hash == li.BodySHA256 {
			continue
		}
		changed[li.Number] = true
		cs, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.IssueComment, *gh.Response, error) {
			return c.REST.Issues.ListComments(ctx, owner, name, li.Number, &gh.IssueListCommentsOptions{ListOptions: o})
		})
		if err != nil {
			return nil, Wrap(ExitExternal, err)
		}
		tag := lockTag(plan.Feature, hash)
		if slices.ContainsFunc(cs, func(cm *gh.IssueComment) bool {
			return strings.HasPrefix(cm.GetBody(), LockMarker) && strings.Contains(cm.GetBody(), tag)
		}) {
			continue // this body has its comment already
		}
		what, err := lockedChange(ctx, w, li, is.GetBody())
		if err != nil {
			return nil, err
		}
		by, at, err := lastEdit(ctx, c, owner, name, li.Number)
		if err != nil {
			return nil, Wrap(ExitExternal, err)
		}
		if at.IsZero() {
			at = is.GetUpdatedAt().Time
		}
		who := ""
		if by != "" {
			who = " by @" + by
		}
		body := fmt.Sprintf("%s\n%s\nThis body changed while the mission of #%d holds it: %s, edited%s at %s. aayu did not change it back. #%d waits until a person keeps the edit in the inbox, or the mission ends.\n",
			LockMarker, tag, plan.Feature, what, who, at.Local().Format("15:04"), li.Number)
		if err := postComment(ctx, c, repo, li.Number, "github.comment.lock", body); err != nil {
			return nil, Wrap(ExitExternal, err)
		}
	}
	return changed, nil
}

// lockTag names the mission and the body a lock comment is about.
func lockTag(f int, hash string) string {
	return fmt.Sprintf("<!-- mission=%d body_sha256=%s -->", f, hash)
}

// lockedChange says what changed in a locked body, against its body at
// Ready when that is the body the lock holds: "criterion 3", or "its Summary
// section"; else "its body".
func lockedChange(ctx context.Context, w *ledger.Writer, li LockedIssue, now string) (string, error) {
	t, err := w.Read(ctx)
	if err != nil {
		return "", Wrap(ExitExternal, ignoreMissing(err))
	}
	defer t.Close()
	file, err := t.File(ctx, ready.LedgerPath(li.Number))
	if err != nil && ignoreMissing(err) != nil {
		return "", Wrap(ExitExternal, err)
	}
	_, was, ok := strings.Cut(string(file), "\n## Body at Ready\n\n````markdown\n")
	was = strings.TrimSuffix(strings.TrimSuffix(was, "\n"), "\n````")
	if !ok || workitems.BodyHash(was) != li.BodySHA256 {
		return "its body", nil
	}
	return bodyChange(was, now), nil
}

// bodyChange names what differs between two bodies: the criteria of the
// Completion Goal by number, and every other section by its label.
func bodyChange(was, now string) string {
	wl, wv := workitems.Sections(was)
	nl, nv := workitems.Sections(now)
	var parts []string
	seen := map[string]bool{}
	for _, l := range append(wl, nl...) {
		if seen[l] || wv[l] == nv[l] {
			continue
		}
		seen[l] = true
		if ns := changedCriteria(wv[l], nv[l]); l == "Completion Goal" && len(ns) > 0 {
			parts = append(parts, plural(len(ns), "criterion ", "criteria ")+andList(ns))
			continue
		}
		parts = append(parts, "its "+l+" section")
	}
	if len(parts) == 0 {
		return "its body"
	}
	return andList(parts)
}

// changedCriteria are the numbers of the criteria that differ, were added, or were removed.
func changedCriteria(was, now string) []string {
	wc, _ := workitems.ParseCriteria(was)
	nc, _ := workitems.ParseCriteria(now)
	by := map[int][2]string{}
	for _, c := range wc {
		v := by[c.N]
		v[0] = fmt.Sprint(c)
		by[c.N] = v
	}
	for _, c := range nc {
		v := by[c.N]
		v[1] = fmt.Sprint(c)
		by[c.N] = v
	}
	var ns []int
	for n, v := range by {
		if v[0] != v[1] {
			ns = append(ns, n)
		}
	}
	slices.Sort(ns)
	out := make([]string, len(ns))
	for i, n := range ns {
		out[i] = strconv.Itoa(n)
	}
	return out
}

// lastEdit is who last edited issue n's body, and when: GitHub keeps both
// only in GraphQL (editor, lastEditedAt).
func lastEdit(ctx context.Context, c *github.Client, owner, name string, n int) (string, time.Time, error) {
	var q struct {
		Repository struct {
			Issue struct {
				LastEditedAt *githubv4.DateTime
				Editor       *struct{ Login string }
			} `graphql:"issue(number: $n)"`
		} `graphql:"repository(owner: $owner, name: $name)"`
	}
	if err := c.GraphQL.Query(ctx, &q, map[string]any{"owner": githubv4.String(owner), "name": githubv4.String(name), "n": githubv4.Int(n)}); err != nil {
		return "", time.Time{}, err
	}
	is := q.Repository.Issue
	var at time.Time
	if is.LastEditedAt != nil {
		at = is.LastEditedAt.Time
	}
	if is.Editor == nil {
		return "", at, nil
	}
	return is.Editor.Login, at, nil
}

// endLock ends Feature f's active lock: the last milestone of its last
// mission is done. A dry run writes nothing.
func endLock(ctx context.Context, c *github.Client, w *ledger.Writer, f int, plan *MissionPlan, at string) error {
	if plan.Lock == nil || plan.Lock.State != LockActive || c.DryRun() {
		return nil
	}
	reason := "every story and Bug of the mission is merged"
	if ms := plan.Milestones; len(ms) > 0 {
		reason = fmt.Sprintf("%s, the last milestone of the last mission, is done", ms[len(ms)-1].Name)
	}
	_, err := writePlan(ctx, w, f, fmt.Sprintf("missions: the lock of #%d ended: %s", f, reason), func(p *MissionPlan) error {
		if p.Lock != nil && p.Lock.State == LockActive {
			p.Lock.State, p.Lock.EndedAt, p.Lock.EndReason = LockEnded, &at, &reason
		}
		return nil
	})
	return err
}

// keepEdit is a person's Keep the edit on a locked-edit card, through
// GitHub's device flow: the lock takes the issue's body as it is now, which
// must be the body the card showed, and controls record the keep. aayu
// mission next then lists the issue by the usual rules.
func (e *Env) keepEdit(ctx context.Context, c *github.Client, repo string, it InboxItem) error {
	f, n := it.Feature, it.Number
	owner, name, _ := github.SplitRepo(repo)
	same := func() error { // the body is still the one the card showed
		is, _, err := c.REST.Issues.Get(ctx, owner, name, n)
		if err != nil {
			return Wrap(ExitExternal, err)
		}
		if workitems.BodyHash(is.GetBody()) != it.BodyHash {
			return Errorf(ExitBlocked, "#%d's body changed again since the card was read. Nothing was kept.", n)
		}
		return nil
	}
	if err := same(); err != nil {
		return err
	}
	p, err := e.presenceCheck(ctx, Pending{What: "keep the edit", Effect: []string{
		fmt.Sprintf("Keeps the edit of #%d: the lock of #%d takes its body as it is now, and #%d is planned again from it", n, f, n)}},
		AccessWrite, e.personOnly("keep the edit"))
	if err != nil {
		return err
	}
	if err := same(); err != nil { // it can change while the person enters the code
		return err
	}
	_, tok, err := e.HumanGitHub(ctx)
	if err != nil {
		return err
	}
	w, err := e.Ledger(ctx, tok, "human", "")
	if err != nil {
		return err
	}
	if c.DryRun() {
		fmt.Fprintf(e.Out, "  --dry-run: the lock of #%d would take #%d's new body\n", f, n)
		return nil
	}
	at := e.Now().UTC().Format(time.RFC3339)
	if _, err := writePlan(ctx, w, f, fmt.Sprintf("missions: @%s kept the edit of #%d in the lock of #%d", p.Login, n, f), func(pl *MissionPlan) error {
		li := pl.Lock.lockedIssue(n)
		if li == nil {
			return Errorf(ExitBlocked, "#%d is not locked by the mission of #%d any more. Nothing was kept.", n, f)
		}
		li.BodySHA256 = it.BodyHash
		pl.Controls = append(pl.Controls, MissionControl{Action: fmt.Sprintf("keep the edit of #%d", n), By: p.Login, At: at, Presence: p.Via})
		return nil
	}); err != nil {
		return err
	}
	fmt.Fprintf(e.Out, "  done: kept the edit of #%d; the lock of #%d holds its new body\n", n, f)
	return nil
}

// missionFacts are a mission plan as the inbox reads it: whether a person
// may start it now, and the tool it would run on.
type missionFacts struct {
	plan      *MissionPlan
	startable bool
	tool      string
	agents    int // how many stories it would run at once
}

// startAndLockCards reads the mission plans on the ledger for the inbox: the
// ones a person may start, and the ones that hold a lock.
func (e *Env) startAndLockCards(ctx context.Context) ([]missionFacts, error) {
	c, tok, err := e.HumanGitHub(ctx)
	if err != nil {
		return nil, err
	}
	repo, w, plans, err := e.ledgerPlans(ctx, tok, "human")
	if err != nil {
		return nil, err
	}
	var out []missionFacts
	tool, read := "", false
	for _, p := range plans {
		mf := missionFacts{plan: p}
		if p.StartedBy == "" {
			_, appr, _, err := readMission(ctx, w, p.Feature)
			if err != nil {
				return nil, err
			}
			mf.startable = appr != nil && appr.Version == p.Approval
			if mf.startable {
				if mf.agents, _, err = e.missionAgents(ctx, c, repo, p.Feature, appr.Agents); err != nil {
					return nil, err
				}
			}
		}
		if mf.startable && !read {
			if tool, err = e.repoTool(ctx, repo); err != nil {
				return nil, err
			}
			read = true
		}
		mf.tool = tool
		if mf.startable || p.Lock != nil && p.Lock.State == LockActive {
			out = append(out, mf)
		}
	}
	return out, nil
}

// lockItems are the inbox's mission cards: Start the mission of each plan a
// person may start (baseline v44, j-inbox-card-889), and one card for each
// open issue that a running mission locks and whose body a person changed.
// Neither card recommends a choice: both calls are a person's.
func (s *inboxSource) lockItems(ms []missionFacts) []InboxItem {
	var out []InboxItem
	for _, m := range ms {
		p, f := m.plan, m.plan.Feature
		feature := s.issues[f]
		if feature == nil || feature.GetState() != "open" {
			continue
		}
		if m.startable {
			locked := lockNumbers(p, func(n int) bool { return s.issues[n] == nil || s.issues[n].GetState() == "open" })
			it := InboxItem{Kind: InboxStart, Number: f, Title: fmt.Sprintf("Start the mission of #%d", f), URL: feature.GetHTMLURL(),
				Seat: verbs.EngineeringManager, Evidence: feature.GetHTMLURL(), Updated: feature.GetUpdatedAt().Time, Lines: startCard(p, locked, m.tool, m.agents),
				Why: fmt.Sprintf("#%d is approved to build (approval %d), and its mission has not started.", f, p.Approval),
				Options: []InboxAction{
					{N: 1, Label: "Start the mission", Command: fmt.Sprintf("aayu mission start %d", f)},
					{N: 2, Label: "Not now: nothing is locked", Command: inboxLeave},
					{N: 3, Label: "Read the issues first", Command: "open"},
				}}
			it.f.problem = fmt.Sprintf("%d %s", len(locked), plural(len(locked), "issue", "issues"))
			out = append(out, it)
		}
		if p.Lock == nil || p.Lock.State != LockActive {
			continue
		}
		for _, li := range p.Lock.Issues {
			is := s.issues[li.Number]
			if is == nil || is.GetState() != "open" || workitems.BodyHash(is.GetBody()) == li.BodySHA256 {
				continue
			}
			n := li.Number
			out = append(out, InboxItem{Kind: InboxLocked, Number: n, Title: fmt.Sprintf("#%d changed while locked by the mission of #%d", n, f), URL: is.GetHTMLURL(),
				Seat: verbs.EngineeringManager, Evidence: is.GetHTMLURL(), Updated: is.GetUpdatedAt().Time, Feature: f, BodyHash: workitems.BodyHash(is.GetBody()),
				Why: fmt.Sprintf("its body is not the one the lock of #%d holds; aayu did not change it back, and #%d waits.", f, n),
				Options: []InboxAction{
					{N: 1, Label: fmt.Sprintf("Keep the edit: #%d is planned again from its new body", n), Command: "inbox: keep the edit"},
					{N: 2, Label: fmt.Sprintf("Leave #%d waiting", n), Command: inboxLeave},
					{N: 3, Label: fmt.Sprintf("Open #%d on GitHub to undo the edit yourself", n), Command: "open"},
				}})
		}
	}
	return out
}
