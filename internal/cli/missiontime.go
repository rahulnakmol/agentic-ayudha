package cli

import (
	"bufio"
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/delegate"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

// Mission time (Story #1015, Decision #974, which amends #915). Approve to
// build estimates each mission in working hours along the longest chain of
// its story graph. The clock runs while at least one of the mission's story
// threads works. It pauses while every running story waits on a person, a
// usage limit, or GitHub CI. Past [mission] warn_hours the mission only
// warns. Past [mission] max_hours it starts no new story until a person
// extends it with a reason; running stories finish. Each mission has its
// own estimate, clock, and limits.

// WorkingHours is #886's read of working hours per effort size: learned from
// the history (#886 S8) where a size has it, else #886's working-hour default
// for the effort, under the key #886 names. It is not the elapsed
// [delivery.default_hours].
type WorkingHours struct {
	Learned    map[string]float64 // effort size → learned working hours
	Default    map[string]float64 // effort size → #886's default working hours
	DefaultKey string             // the key #886 names for the defaults
}

// workingHours is #886's read, nil until #886 lands (tests set Options.WorkingHours):
// with none, the plan has no estimate in hours, and nothing warns or refuses at Start.
func (e *Env) workingHours(ctx context.Context) (*WorkingHours, error) {
	if e.WorkingHours != nil {
		return e.WorkingHours(ctx)
	}
	return nil, nil
}

// MissionHours is a mission's estimate in working hours: #886's hours per
// effort size, summed along the longest chain of the mission's story graph,
// because merges in a row set the time.
type MissionHours struct {
	Hours      float64  `json:"hours"`
	Learned    float64  `json:"learned"` // the part from #886's history
	Default    float64  `json:"default"` // the part from #886's defaults: sizes with no history
	Chain      []int    `json:"chain"`
	Basis      string   `json:"basis"`
	DefaultKey string   `json:"default_key,omitempty"`
	NoHistory  []string `json:"no_history,omitempty"` // the sizes on the chain with no learned hours
	// Mark is "over <max_hours> h" when the learned hours alone pass
	// [mission] max_hours (Start needs a person's reason), or "no history"
	// when only default hours take it past.
	Mark string `json:"mark,omitempty"`
}

// over reports whether Start needs a person's reason for this estimate.
func (h *MissionHours) over() bool { return h != nil && strings.HasPrefix(h.Mark, "over ") }

const markNoHistory = "no history"

// effortSizes is the order the estimate names sizes in.
var effortSizes = []string{"XS", "S", "M", "L", "XL"}

// missionLimits are [mission] warn_hours and max_hours.
func missionLimits(c *config.Config) (warn, maxH int) {
	return c.Mission.WarnHours, c.Mission.MaxHours
}

// hrs prints hours with at most one decimal: 19, 1.5.
func hrs(h float64) string { return strconv.FormatFloat(math.Round(h*10)/10, 'f', -1, 64) }

// estimateMission is mission m's estimate, or nil without #886's read. Done
// stories take no more time.
func estimateMission(graph []MissionItem, m Mission, wh *WorkingHours, done func(int) bool, maxH int) *MissionHours {
	if wh == nil {
		return nil
	}
	items := map[int]MissionItem{}
	for _, s := range m.Stories {
		for _, it := range graph {
			if it.Number == s.Number && !done(it.Number) {
				items[it.Number] = it
			}
		}
	}
	hoursOf := func(it MissionItem) (float64, bool) {
		if h, ok := wh.Learned[it.Effort]; ok {
			return h, true
		}
		return wh.Default[it.Effort], false
	}
	// best[n] is the longest chain that ends at n, by hours, then by stories.
	type chain struct {
		hours float64
		path  []int
	}
	best, seen := map[int]chain{}, map[int]bool{}
	var walk func(n int) chain
	walk = func(n int) chain {
		if c, ok := best[n]; ok {
			return c
		}
		if seen[n] { // a depends-on cycle: Ready refuses it; it adds nothing here
			return chain{}
		}
		seen[n] = true
		var longest chain
		for _, d := range items[n].DependsOn {
			if _, in := items[d]; !in {
				continue
			}
			if c := walk(d); c.hours > longest.hours || c.hours == longest.hours && len(c.path) > len(longest.path) {
				longest = c
			}
		}
		h, _ := hoursOf(items[n])
		c := chain{hours: longest.hours + h, path: append(slices.Clone(longest.path), n)}
		best[n] = c
		return c
	}
	var top chain
	for _, n := range slices.Sorted(mapKeys(items)) {
		if c := walk(n); c.hours > top.hours || c.hours == top.hours && len(c.path) > len(top.path) {
			top = c
		}
	}
	est := &MissionHours{Chain: append([]int{}, top.path...), DefaultKey: wh.DefaultKey}
	count, learned := map[string]int{}, map[string]bool{}
	for _, n := range top.path {
		h, ok := hoursOf(items[n])
		est.Hours += h
		if ok {
			est.Learned += h
		} else {
			est.Default += h
		}
		count[items[n].Effort]++
		learned[items[n].Effort] = ok
	}
	sizes := slices.Clone(effortSizes)
	for _, s := range slices.Sorted(mapKeysS(count)) {
		if !slices.Contains(sizes, s) {
			sizes = append(sizes, s)
		}
	}
	var terms, sources, none []string
	for _, s := range sizes {
		if count[s] == 0 {
			continue
		}
		h, _ := hoursOf(MissionItem{Effort: s})
		terms = append(terms, fmt.Sprintf("%d × %s h", count[s], hrs(h)))
		if learned[s] {
			sources = append(sources, s+": #886 history")
		} else {
			sources = append(sources, s+": #886 default")
			none = append(none, s)
		}
	}
	est.NoHistory = none
	switch {
	case len(none) == 0:
		est.Basis = fmt.Sprintf("about %s working hours (%s; source: #886 history)", hrs(est.Hours), strings.Join(terms, " + "))
	case len(none) == len(sources):
		est.Basis = fmt.Sprintf("about %s h, no history (%s; source: #886 default)", hrs(est.Hours), strings.Join(terms, " + "))
	default:
		est.Basis = fmt.Sprintf("about %s h (%s): %s", hrs(est.Hours), strings.Join(sources, "; "), est.split())
	}
	switch {
	case est.Learned > float64(maxH):
		est.Mark = fmt.Sprintf("over %d h", maxH)
	case est.Hours > float64(maxH):
		est.Mark = markNoHistory
	}
	return est
}

// split is "30 h learned + 40 h default (no history for M)".
func (h *MissionHours) split() string {
	return fmt.Sprintf("%s h learned + %s h default (no history for %s)", hrs(h.Learned), hrs(h.Default), andList(h.NoHistory))
}

func mapKeysS[V any](m map[string]V) func(func(string) bool) {
	return func(yield func(string) bool) {
		for k := range m {
			if !yield(k) {
				return
			}
		}
	}
}

// estimateLines are the lines Approve to build shows for each mission's
// estimate: it warns past warn_hours, and past max_hours it says the person's
// override at Start when the learned hours alone pass it. It refuses nothing.
func estimateLines(ms []Mission, warn, maxH int) []string {
	var out []string
	for _, m := range ms {
		h := m.Estimate
		if h == nil {
			continue
		}
		out = append(out, fmt.Sprintf("Mission %d time: %s along #%s", m.Number, h.Basis, joinNums(h.Chain, ", #")))
		switch {
		case h.over():
			out = append(out, fmt.Sprintf("Mission %d: Over %d h: split this Feature, or override with a reason (person only)", m.Number, maxH))
		case h.Hours > float64(maxH) && h.Learned == 0:
			out = append(out, fmt.Sprintf("Warning: mission %d about %s h, no history: over [mission] max_hours (%d); it starts with no reason", m.Number, hrs(h.Hours), maxH))
		case h.Hours > float64(maxH):
			out = append(out, fmt.Sprintf("Warning: mission %d about %s h, %s: over [mission] max_hours (%d); the learned hours alone (%s) are under it, so it starts with no reason",
				m.Number, hrs(h.Hours), h.split(), maxH, hrs(h.Learned)))
		case h.Hours > float64(warn):
			out = append(out, fmt.Sprintf("Warning: mission %d about %s h: over [mission] warn_hours (%d)", m.Number, hrs(h.Hours), warn))
		}
	}
	return out
}

func joinNums(ns []int, sep string) string {
	s := make([]string, len(ns))
	for i, n := range ns {
		s[i] = strconv.Itoa(n)
	}
	return strings.Join(s, sep)
}

// Clock waits: what every running story waited on while the clock paused.
const (
	waitPerson = "person"
	waitUsage  = "usage limit"
	waitCI     = "CI"
	waitIdle   = "no story running"
)

// waitOrder names the not-counted parts in order; a moment when the
// running stories wait on different things counts as the first of them.
var waitOrder = []string{waitPerson, waitUsage, waitCI, waitIdle}

// storyRun is one story's run in a mission: from its start to its end (now
// while it runs), and its waits.
type storyRun struct {
	n          int
	start, end time.Time
	ended      bool
	waits      []clockWait
}

type clockWait struct {
	kind     string
	from, to time.Time
}

// MissionClock is a mission's clock now: its mission time, its limit, the
// time it did not count and why, and each story's working total.
type MissionClock struct {
	Mission    int                `json:"mission"`
	Hours      float64            `json:"hours"`
	Limit      float64            `json:"limit"` // [mission] max_hours, plus each extension
	Extended   int                `json:"extended,omitempty"`
	Wall       float64            `json:"wall"`
	NotCounted map[string]float64 `json:"not_counted"`
	Stories    []StoryClock       `json:"stories"`
}

// StoryClock is a story's working total: the time its own thread worked, less its waits.
type StoryClock struct {
	Number       int     `json:"number"`
	WorkingHours float64 `json:"working_hours"`
	Ended        bool    `json:"ended,omitempty"`
	run          storyRun
}

// StoryRunRecord is an ended story's run in plan.json: its start, its end,
// and its waits, in RFC 3339.
type StoryRunRecord struct {
	Start string    `json:"start"`
	End   string    `json:"end"`
	Waits []RunWait `json:"waits,omitempty"`
}

// RunWait is one wait of a recorded run: on a person, a usage limit, or CI.
type RunWait struct {
	Kind string `json:"kind"`
	From string `json:"from"`
	To   string `json:"to"`
}

func (r storyRun) record() *StoryRunRecord {
	rec := &StoryRunRecord{Start: r.start.UTC().Format(time.RFC3339), End: r.end.UTC().Format(time.RFC3339)}
	for _, w := range r.waits {
		rec.Waits = append(rec.Waits, RunWait{w.kind, w.from.UTC().Format(time.RFC3339), w.to.UTC().Format(time.RFC3339)})
	}
	return rec
}

// recordedRun is story n's ended run from plan.json, when it is whole.
func recordedRun(n int, rec *StoryRunRecord) (storyRun, bool) {
	start, err1 := time.Parse(time.RFC3339, rec.Start)
	end, err2 := time.Parse(time.RFC3339, rec.End)
	if err1 != nil || err2 != nil {
		return storyRun{}, false
	}
	r := storyRun{n: n, start: start, end: end, ended: true}
	for _, w := range rec.Waits {
		from, err1 := time.Parse(time.RFC3339, w.From)
		to, err2 := time.Parse(time.RFC3339, w.To)
		if err1 != nil || err2 != nil {
			return storyRun{}, false
		}
		r.waits = append(r.waits, clockWait{w.Kind, from, to})
	}
	return r, true
}

// reached reports whether the mission used its time: new starts wait.
func (c MissionClock) reached() bool { return c.Limit > 0 && c.Hours >= c.Limit }

// String is "19 h of 48 (11 h not counted: 5 h person, 2 h usage limit, 4 h CI)".
func (c MissionClock) String() string {
	s := fmt.Sprintf("%s h of %s", hrs(c.Hours), hrs(c.Limit))
	var parts []string
	total := 0.0
	for _, k := range waitOrder {
		if v := c.NotCounted[k]; hrs(v) != "0" {
			parts = append(parts, fmt.Sprintf("%s h %s", hrs(v), k))
			total += v
		}
	}
	if len(parts) > 0 {
		s += fmt.Sprintf(" (%s h not counted: %s)", hrs(total), strings.Join(parts, ", "))
	}
	return s
}

// holdReason is why a story of this mission waits once the clock reached its limit.
func (c MissionClock) holdReason(f int) string {
	if c.Extended > 0 {
		return fmt.Sprintf("#%d reached its extended time limit (%s h)", f, hrs(c.Limit))
	}
	return fmt.Sprintf("#%d reached max_hours (%s h)", f, hrs(c.Limit))
}

// clockOf is the clock of one mission's story runs. Between the first start
// and the last end, a moment counts when at least one running story's thread
// works (its local checks too); it does not count when every running story
// waits, or when none runs.
func clockOf(runs []storyRun) (hours, wall float64, not map[string]float64, working map[int]float64) {
	not, working = map[string]float64{}, map[int]float64{}
	var ts []time.Time
	for _, r := range runs {
		ts = append(ts, r.start, r.end)
		for _, w := range r.waits {
			ts = append(ts, w.from, w.to)
		}
		working[r.n] = r.end.Sub(r.start).Hours() - union(r.waits, r.start, r.end)
	}
	if len(ts) == 0 {
		return 0, 0, not, working
	}
	slices.SortFunc(ts, func(a, b time.Time) int { return a.Compare(b) })
	ts = slices.CompactFunc(ts, func(a, b time.Time) bool { return a.Equal(b) })
	for i := 0; i+1 < len(ts); i++ {
		a, d := ts[i], ts[i+1].Sub(ts[i]).Hours()
		wall += d
		kind, running := "", false
		for _, r := range runs {
			if r.start.After(a) || !r.end.After(a) {
				continue
			}
			running = true
			k := ""
			for _, w := range r.waits {
				if !w.from.After(a) && w.to.After(a) && (k == "" || slices.Index(waitOrder, w.kind) < slices.Index(waitOrder, k)) {
					k = w.kind
				}
			}
			if k == "" { // this story's thread works
				kind = ""
				break
			}
			if kind == "" || slices.Index(waitOrder, k) < slices.Index(waitOrder, kind) {
				kind = k
			}
		}
		switch {
		case !running:
			not[waitIdle] += d
		case kind == "":
			hours += d
		default:
			not[kind] += d
		}
	}
	return hours, wall, not, working
}

// union is how many hours of [from, to) the waits cover.
func union(ws []clockWait, from, to time.Time) float64 {
	type iv struct{ a, b time.Time }
	var ivs []iv
	for _, w := range ws {
		a, b := maxTime(w.from, from), minTime(w.to, to)
		if b.After(a) {
			ivs = append(ivs, iv{a, b})
		}
	}
	slices.SortFunc(ivs, func(x, y iv) int { return x.a.Compare(y.a) })
	total, end := 0.0, time.Time{}
	for _, v := range ivs {
		if v.a.Before(end) {
			v.a = end
		}
		if v.b.After(v.a) {
			total += v.b.Sub(v.a).Hours()
			end = v.b
		}
	}
	return total
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

// missionClocks are the clocks of the plan's missions now, from what aayu
// and GitHub recorded of each started story: when its thread started (its
// delegation, else its mission claim) and ended (its issue closed), and its
// waits. On a person: ship's Ready to ship comment on its PR, and a paused
// mission (#1002); an open ask waits on #880. On a usage limit: its
// delegation's pause. On GitHub CI: each check run on its PR's commits (the
// merge gate, evals, end-to-end tests, and the CI review).
func (e *Env) missionClocks(ctx context.Context, c *github.Client, w *ledger.Writer, repo string, f int, plan *MissionPlan, maxH int, id config.Identity) ([]MissionClock, error) {
	if plan.StartedBy == "" {
		return nil, nil
	}
	t, err := w.Read(ctx)
	if err != nil {
		return nil, Wrap(ExitExternal, ignoreMissing(err))
	}
	defer t.Close()
	raw, err := t.File(ctx, MissionClaimsPath(f))
	if err = ignoreMissing(err); err != nil {
		return nil, Wrap(ExitExternal, err)
	}
	claimed := firstClaims(raw)
	now := e.Now()
	owner, name, _ := github.SplitRepo(repo)
	var out []MissionClock
	for _, m := range plan.Missions {
		var runs []storyRun
		for _, s := range m.Stories {
			if s.Run != nil { // an ended story's run does not change: no GitHub read
				if r, ok := recordedRun(s.Number, s.Run); ok {
					runs = append(runs, r)
					continue
				}
			}
			r, err := e.storyRunOf(ctx, c, t, owner, name, id, s.Number, claimed[s.Number], plan.Controls, now)
			if err != nil {
				return nil, err
			}
			if r != nil {
				runs = append(runs, *r)
			}
		}
		h, wall, not, working := clockOf(runs)
		ext, limit := 0, float64(maxH)
		for _, ctl := range plan.Controls {
			if ctl.Action == "extend" && ctl.Mission == m.Number {
				// An extension adds the hours the person approved, not today's max_hours.
				ext, limit = ext+1, limit+cmp.Or(ctl.Hours, float64(maxH))
			}
		}
		mc := MissionClock{Mission: m.Number, Hours: h, Wall: wall, NotCounted: not, Limit: limit, Extended: ext, Stories: []StoryClock{}}
		for _, r := range runs {
			mc.Stories = append(mc.Stories, StoryClock{Number: r.n, WorkingHours: math.Round(working[r.n]*100) / 100, Ended: r.ended, run: r})
		}
		out = append(out, mc)
	}
	return out, nil
}

// stopWaits are story n's waits on a person from the mission's controls
// (Story #1002): from a stop of the mission or of n to its resume, or to
// end while it stays stopped.
func stopWaits(controls []MissionControl, n int, end time.Time) []clockWait {
	var out []clockWait
	from := map[int]time.Time{} // 0: the mission; n: the story
	for _, ctl := range controls {
		at, err := time.Parse(time.RFC3339, ctl.At)
		if err != nil || ctl.Story != 0 && ctl.Story != n {
			continue
		}
		switch ctl.Action {
		case "stop", "end":
			if _, ok := from[ctl.Story]; !ok {
				from[ctl.Story] = at
			}
		case "resume":
			if f, ok := from[ctl.Story]; ok {
				out = append(out, clockWait{waitPerson, f, minTime(at, end)})
				delete(from, ctl.Story)
			}
		}
	}
	for _, f := range from {
		out = append(out, clockWait{waitPerson, f, end})
	}
	return out
}

// firstClaims is when each story was first claimed in a claims file, by a
// claim no later line released: a released claim started nothing.
func firstClaims(raw []byte) map[int]time.Time {
	var claims []loopClaim
	released := map[string]bool{}
	sc := bufio.NewScanner(bytes.NewReader(raw))
	for sc.Scan() {
		var cl loopClaim
		switch {
		case json.Unmarshal(sc.Bytes(), &cl) != nil:
		case cl.Release:
			released[cl.Token] = true
		default:
			claims = append(claims, cl)
		}
	}
	out := map[int]time.Time{}
	for _, cl := range claims {
		if at, err := time.Parse(time.RFC3339, cl.At); err == nil && !released[cl.Token] && (out[cl.Story].IsZero() || at.Before(out[cl.Story])) {
			out[cl.Story] = at
		}
	}
	return out
}

// storyRunOf is story n's run, or nil when it has not started.
func (e *Env) storyRunOf(ctx context.Context, c *github.Client, t *ledger.Tree, owner, name string, id config.Identity, n int, claimedAt time.Time, controls []MissionControl, now time.Time) (*storyRun, error) {
	var st *delegate.State
	if b, err := t.File(ctx, delegate.LedgerPath(n)); ignoreMissing(err) != nil {
		return nil, Wrap(ExitExternal, err)
	} else if b != nil && json.Unmarshal(b, &st) != nil {
		st = nil
	}
	r := &storyRun{n: n, start: claimedAt, end: now}
	if st != nil {
		if at, err := time.Parse(time.RFC3339, st.StartedAt); err == nil {
			r.start = at
		}
	}
	if r.start.IsZero() {
		return nil, nil
	}
	is, _, err := c.REST.Issues.Get(ctx, owner, name, n)
	if err != nil {
		return nil, Wrap(ExitExternal, err)
	}
	if is.GetState() == "closed" {
		r.ended = true
		if at := is.GetClosedAt().Time; !at.IsZero() {
			r.end = at
		}
	}
	r.end = maxTime(r.end, r.start)
	r.waits = append(r.waits, stopWaits(controls, n, r.end)...)
	if st == nil {
		return r, nil
	}
	for _, ev := range st.Events {
		at, err1 := time.Parse(time.RFC3339, ev.At)
		until, err2 := time.Parse(time.RFC3339, ev.Until)
		if ev.Status == dlgPaused && err1 == nil && err2 == nil {
			r.waits = append(r.waits, clockWait{waitUsage, at, minTime(until, r.end)})
		}
	}
	if st.PR == 0 {
		return r, nil
	}
	comments, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.IssueComment, *gh.Response, error) {
		return c.REST.Issues.ListComments(ctx, owner, name, st.PR, &gh.IssueListCommentsOptions{ListOptions: o})
	})
	if err != nil {
		return nil, Wrap(ExitExternal, err)
	}
	for _, cm := range comments {
		// Anyone may comment: only ship's marker, by an account with write access, is a wait on a person.
		if !readyHead.MatchString(cm.GetBody()) {
			continue
		}
		if ok, err := trustedBy(ctx, c, owner, name, id, cm.GetUser()); err != nil {
			return nil, Wrap(ExitExternal, err)
		} else if ok {
			r.waits = append(r.waits, clockWait{waitPerson, cm.GetCreatedAt().Time, r.end})
		}
	}
	commits, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.RepositoryCommit, *gh.Response, error) {
		return c.REST.PullRequests.ListCommits(ctx, owner, name, st.PR, &o)
	})
	if err != nil {
		return nil, Wrap(ExitExternal, err)
	}
	for _, cm := range commits {
		runs, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.CheckRun, *gh.Response, error) {
			res, resp, err := c.REST.Checks.ListCheckRunsForRef(ctx, owner, name, cm.GetSHA(), &gh.ListCheckRunsOptions{Filter: gh.Ptr("all"), ListOptions: o})
			return res.GetCheckRuns(), resp, err
		})
		if err != nil {
			return nil, Wrap(ExitExternal, err)
		}
		for _, cr := range runs {
			from, to := cr.GetStartedAt().Time, cr.GetCompletedAt().Time
			if to.IsZero() {
				to = r.end
			}
			if !from.IsZero() {
				r.waits = append(r.waits, clockWait{waitCI, from, minTime(to, r.end)})
			}
		}
	}
	return r, nil
}

// MissionTimeMarker starts the comment on the Feature that is a mission's
// time card in the inbox: one per mission, edited in place.
const MissionTimeMarker = "<!-- aayudha:mission-time"

var missionTimeRE = regexp.MustCompile(regexp.QuoteMeta(MissionTimeMarker) + ` mission=([0-9]+) state=([a-z]+) hours=([0-9.]+) add=([0-9.]+) -->`)

func missionTimeMarker(m int) string { return fmt.Sprintf("%s mission=%d ", MissionTimeMarker, m) }

// missionCard is the body of mission m's time card in a state: warn (it ran
// warn_hours), limit (it reached its limit; an extension adds add hours),
// seen (a person saw the warning), or extended (a person extended it).
func missionCard(m int, state string, hours, add float64, line string) string {
	return fmt.Sprintf("%sstate=%s hours=%s add=%s -->\n%s\n", missionTimeMarker(m), state, hrs(hours), hrs(add), line)
}

// upsertCard writes mission m's time card on Feature f as the account c
// writes as, in that account's own comment: a look-alike by anyone else is
// never edited.
func upsertCard(ctx context.Context, c *github.Client, repo string, f, m int, state, body string) error {
	me, _, err := c.REST.Users.Get(ctx, "")
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	if err := upsertOwnComment(ctx, c, repo, f, me.GetLogin(), missionTimeMarker(m), body,
		actions.Action{Kind: "github.comment.mission-time", Payload: map[string]any{"state": state, "mission": m}}); err != nil {
		return Wrap(ExitExternal, err)
	}
	return nil
}

// trustedBy applies scopeTrusted in owner/name: a comment counts only from
// aayu's App or an account with write access. Anyone may comment, so any
// other marker is text.
func trustedBy(ctx context.Context, c *github.Client, owner, name string, id config.Identity, u *gh.User) (bool, error) {
	return scopeTrusted(u, id, func(login string) (bool, error) {
		perm, _, err := c.REST.Repositories.GetPermissionLevel(ctx, owner, name, login)
		if err != nil {
			return false, err
		}
		q := permissionOf(perm)
		return q == "write" || q == "admin", nil
	})
}

// missionCards posts each mission's time card once: when its clock reaches
// warn_hours, and when it reaches its limit. A card a person saw, or an
// extension, is not posted again until the next limit.
func (e *Env) missionCards(ctx context.Context, c *github.Client, repo string, f int, clocks []MissionClock, warn, maxH int, id config.Identity) error {
	if c.DryRun() {
		return nil
	}
	owner, name, _ := github.SplitRepo(repo)
	var comments []*gh.IssueComment
	read := false
	for _, mc := range clocks {
		state, line := "", ""
		switch {
		case mc.reached():
			state, line = "limit", fmt.Sprintf("**#%d reached its %s h time limit.** new starts wait: the mission of #%d used its %s working hours. Running stories keep going. "+
				"No recommendation: the mission gives none. A person extends it with `aayu mission extend %d --reason \"…\"`, or stops it.", f, hrs(mc.Limit), f, hrs(mc.Limit), f)
		case warn > 0 && mc.Hours >= float64(warn) && mc.Extended == 0:
			state, line = "warn", fmt.Sprintf("**#%d has run %d h.** Mission %d's clock reached [mission] warn_hours; nothing stops. No recommendation: the mission gives none.", f, warn, mc.Mission)
		default:
			continue
		}
		if !read {
			var err error
			if comments, err = github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.IssueComment, *gh.Response, error) {
				return c.REST.Issues.ListComments(ctx, owner, name, f, &gh.IssueListCommentsOptions{ListOptions: o})
			}); err != nil {
				return Wrap(ExitExternal, err)
			}
			read = true
		}
		hours := float64(warn)
		if state == "limit" {
			hours = mc.Limit
		}
		posted := false
		for _, cm := range comments {
			// A warning only when the mission has no card yet, and a limit card once per limit; only a card by an
			// account with write access counts, as the inbox reads it.
			mm := missionTimeRE.FindStringSubmatch(cm.GetBody())
			if mm == nil || mm[1] != strconv.Itoa(mc.Mission) {
				continue
			}
			if ok, err := trustedBy(ctx, c, owner, name, id, cm.GetUser()); err != nil {
				return Wrap(ExitExternal, err)
			} else if ok {
				posted = state == "warn" || mm[2] == "limit" && mm[3] == hrs(hours)
			}
		}
		if posted {
			continue
		}
		if err := upsertCard(ctx, c, repo, f, mc.Mission, state, missionCard(mc.Mission, state, hours, float64(maxH), line)); err != nil {
			return err
		}
	}
	return nil
}

// recordWorking writes each ended story's working total into the plan, once
// it changed: missions[].stories[].working_hours, with its run and waits, so
// later clocks read it from the plan. Another write first (a moved ledger)
// leaves it for the next run.
func (e *Env) recordWorking(ctx context.Context, c *github.Client, w *ledger.Writer, f int, clocks []MissionClock) error {
	if c.DryRun() || len(clocks) == 0 {
		return nil
	}
	plan, _, tip, err := readMission(ctx, w, f)
	if err != nil || plan == nil {
		return err
	}
	var changed []string
	for _, mc := range clocks {
		for _, sc := range mc.Stories {
			if !sc.Ended {
				continue
			}
			for i, m := range plan.Missions {
				for j, s := range m.Stories {
					if s.Number == sc.Number && m.Number == mc.Mission && (s.WorkingHours != sc.WorkingHours || s.Run == nil) {
						plan.Missions[i].Stories[j].WorkingHours, plan.Missions[i].Stories[j].Run = sc.WorkingHours, sc.run.record()
						changed = append(changed, "#"+strconv.Itoa(sc.Number))
					}
				}
			}
		}
	}
	if len(changed) == 0 {
		return nil
	}
	data, _ := json.MarshalIndent(plan, "", "  ")
	_, err = w.Write(ctx, ledger.Commit{Message: fmt.Sprintf("missions: working hours of %s in the mission of #%d", strings.Join(changed, ", "), f),
		Files: map[string][]byte{MissionPlanPath(f): append(data, '\n')}, Base: tip})
	if err != nil && !errors.Is(err, ledger.ErrMoved) {
		return Wrap(ExitExternal, err)
	}
	return nil
}

// MissionExtendReport is what `aayu mission extend <f>` reports.
type MissionExtendReport struct {
	Repo         string         `json:"repo"`
	Feature      int            `json:"feature"`
	Mission      int            `json:"mission"`
	Effect       []string       `json:"effect"`
	Control      MissionControl `json:"control"`
	Limit        float64        `json:"limit"` // the mission's time limit now
	LedgerPath   string         `json:"ledger_path"`
	LedgerCommit string         `json:"ledger_commit,omitempty"`
	DryRun       bool           `json:"dry_run,omitempty"`
}

// missionExtend is `aayu mission extend <f> --reason "…"`: a person, after
// the presence check, gives a mission that reached its time limit another
// [mission] max_hours, and the plan records the reason and them. The inbox
// card names its mission; on the command line it is the first one held (0).
// An agent lane is refused before anything is read.
func missionExtend(ctx context.Context, e *Env, f int, reason string, mission int) error {
	if err := e.laneRefusal("mission extend"); err != nil {
		return err
	}
	if reason == "" {
		return Errorf(ExitUsage, "mission extend needs --reason: why the mission of #%d may run another [mission] max_hours", f)
	}
	repo, c, w, err := e.missionLedger(ctx)
	if err != nil {
		return err
	}
	l, err := e.Config()
	if err != nil {
		return err
	}
	_, maxH := missionLimits(&l.Config)
	held := func() (*MissionPlan, MissionClock, string, error) {
		plan, _, tip, err := readMission(ctx, w, f)
		switch {
		case err != nil:
			return nil, MissionClock{}, "", err
		case plan == nil || plan.StartedBy == "":
			return nil, MissionClock{}, "", Errorf(ExitBlocked, "The mission of #%d has not started: nothing to extend.", f)
		}
		clocks, err := e.missionClocks(ctx, c, w, repo, f, plan, maxH, l.Config.Identity)
		if err != nil {
			return nil, MissionClock{}, "", err
		}
		for _, mc := range clocks {
			if mc.reached() && (mission == 0 || mc.Mission == mission) {
				return plan, mc, tip, nil
			}
		}
		var now []string
		for _, mc := range clocks {
			now = append(now, fmt.Sprintf("mission %d %s", mc.Mission, mc))
		}
		return nil, MissionClock{}, "", Errorf(ExitBlocked, "No mission of #%d reached its time limit: nothing to extend (%s).", f, strings.Join(now, "; "))
	}
	_, mc, _, err := held()
	if err != nil {
		return err
	}
	limit := mc.Limit + float64(maxH)
	effect := []string{
		fmt.Sprintf("Extends mission %d of #%d by %d h ([mission] max_hours): it used %s, and new starts resume until %s h", mc.Mission, f, maxH, mc, hrs(limit)),
		fmt.Sprintf("REASON %q", reason),
	}
	p, err := e.presenceCheck(ctx, Pending{What: "mission extend", Effect: effect}, AccessWrite, e.personOnly("mission extend"))
	if err != nil {
		return err
	}
	// Extend only what the person saw: the same mission at the same limit.
	plan, again, tip, err := held()
	if err != nil {
		return err
	}
	if again.Mission != mc.Mission || again.Limit != mc.Limit {
		return Errorf(ExitBlocked, "The time limit of #%d's mission changed while the code was waiting. Nothing was extended.", f)
	}
	ctl := MissionControl{Action: "extend", Mission: mc.Mission, Hours: float64(maxH), Reason: reason, By: p.Login, At: e.Now().UTC().Format(time.RFC3339), Presence: p.Via}
	plan.Controls = append(plan.Controls, ctl)
	rep := MissionExtendReport{Repo: repo, Feature: f, Mission: mc.Mission, Effect: effect, Control: ctl, Limit: limit, LedgerPath: MissionPlanPath(f), DryRun: c.DryRun()}
	if !rep.DryRun {
		data, _ := json.MarshalIndent(plan, "", "  ")
		rep.LedgerCommit, err = w.Write(ctx, ledger.Commit{Message: fmt.Sprintf("missions: @%s extended mission %d of #%d to %s h", p.Login, mc.Mission, f, hrs(limit)),
			Files: map[string][]byte{rep.LedgerPath: append(data, '\n')}, Base: tip})
		if errors.Is(err, ledger.ErrMoved) {
			return Errorf(ExitBlocked, "The ledger changed while aayu wrote the extension of #%d. Nothing was extended. Run it again.", f)
		}
		if err != nil {
			return Wrap(ExitExternal, err)
		}
		// The card's choice is made: it leaves the inbox.
		line := fmt.Sprintf("**Extended.** @%s extended mission %d of #%d to %s h: %s", p.Login, mc.Mission, f, hrs(limit), reason)
		if err := upsertCard(ctx, c, repo, f, mc.Mission, "extended", missionCard(mc.Mission, "extended", limit, 0, line)); err != nil {
			return err
		}
	}
	return e.Emit(rep, func(w io.Writer) error {
		fmt.Fprintf(w, "@%s extended mission %d of #%d to %s h: new starts resume. Reason: %s\n", p.Login, mc.Mission, f, hrs(limit), reason)
		if rep.DryRun {
			_, err := fmt.Fprintf(w, "  --dry-run: ledger %s planned, not written\n", rep.LedgerPath)
			return err
		}
		_, err := fmt.Fprintf(w, "  ledger %s (%s)\n", rep.LedgerPath, short(rep.LedgerCommit))
		return err
	})
}

// missionItem makes Feature it's mission time card from aayu's newest time
// comment on it, by an account with write access: a limit card, else a
// warning nobody marked seen. A Feature with neither has no card.
func (s *inboxSource) missionItem(ctx context.Context, it *InboxItem) (bool, error) {
	comments, err := s.commentsOf(ctx, it.Number)
	if err != nil {
		return false, err
	}
	// A mission's state is its newest trusted card, whoever wrote it: a person who extends, stops, or marks it
	// seen may write as another account than the one that posted the card.
	type card struct {
		mm  []string
		url string
		at  time.Time
	}
	newest := map[int]card{}
	for _, cm := range comments {
		mm := missionTimeRE.FindStringSubmatch(cm.GetBody())
		if mm == nil {
			continue
		}
		if ok, err := scopeTrusted(cm.GetUser(), s.id, func(login string) (bool, error) { return s.canWrite(ctx, login) }); err != nil {
			return false, err
		} else if !ok {
			continue
		}
		at := cmp.Or(cm.GetUpdatedAt().Time, cm.GetCreatedAt().Time)
		if m, _ := strconv.Atoi(mm[1]); !at.Before(newest[m].at) {
			newest[m] = card{mm, cm.GetHTMLURL(), at}
		}
	}
	var limit, warn []string
	url := ""
	for _, m := range slices.Sorted(maps.Keys(newest)) {
		switch cd := newest[m]; {
		case cd.mm[2] == "limit":
			limit, url = cd.mm, cd.url
		case cd.mm[2] == "warn" && limit == nil:
			warn, url = cd.mm, cd.url
		}
	}
	f := it.Number
	it.Kind, it.Seat, it.Evidence, it.CounselFrom = InboxMissionTime, verbs.EngineeringManager, or(url, it.URL), "facts"
	switch {
	case limit != nil:
		m, _ := strconv.Atoi(limit[1])
		add := limit[4]
		it.Title, it.f.problem, it.f.mission = fmt.Sprintf("#%d reached its %s h time limit", f, limit[3]), "time limit reached", m
		it.Why = fmt.Sprintf("new starts wait: the mission of #%d used its %s working hours. Running stories keep going.", f, limit[3])
		it.Counsel = it.Why + " No recommendation: the mission gives none."
		it.Options = []InboxAction{
			{N: 1, Label: fmt.Sprintf("Extend for %s h, with a reason", add), Command: fmt.Sprintf("aayu mission extend %d --reason", f),
				Ask: fmt.Sprintf("Why may the mission of #%d run another %s h? It asks for GitHub's device code next.", f, add)},
			{N: 2, Label: "Stop the mission", Command: fmt.Sprintf("aayu mission stop %d --reason", f),
				Ask: fmt.Sprintf("Why do you stop the mission of #%d? It asks for GitHub's device code next.", f)},
		}
	case warn != nil:
		m, _ := strconv.Atoi(warn[1])
		it.Title, it.f.problem, it.f.mission = fmt.Sprintf("#%d has run %s h", f, warn[3]), "ran "+warn[3]+" h", m
		it.Why = fmt.Sprintf("the clock of mission %d of #%d reached [mission] warn_hours (%s h). Nothing stops.", m, f, warn[3])
		it.Counsel = it.Why + " No recommendation: the mission gives none."
		it.Options = []InboxAction{{N: 1, Label: "mark it seen", Command: "inbox: mark the mission card seen"}}
	default:
		return false, nil
	}
	return true, nil
}

// missionTimeAct does a person's choice on a mission time card.
func (e *Env) missionTimeAct(ctx context.Context, c *github.Client, repo string, it InboxItem, a InboxAction) error {
	switch {
	case strings.HasPrefix(a.Command, "aayu mission extend"):
		if err := e.laneRefusal("mission extend"); err != nil {
			return err
		}
		return missionExtend(ctx, e, it.Number, strings.TrimSpace(a.Answer), it.f.mission) // the card's mission, not the first one held
	}
	m, state, line := it.f.mission, "seen", fmt.Sprintf("Seen: the warning of mission %d of #%d.", it.f.mission, it.Number)
	if strings.HasPrefix(a.Command, "aayu mission stop") { // #1002's stop, with the person's reason; then the card's choice is made
		if err := missionStop(ctx, e, it.Number, Values{"reason": a.Answer}); err != nil {
			return err
		}
		state, line = "stopped", fmt.Sprintf("**Stopped.** The mission of #%d was stopped at its time limit: %s", it.Number, strings.TrimSpace(a.Answer))
	}
	if err := upsertCard(ctx, c, repo, it.Number, m, state, missionCard(m, state, 0, 0, line)); err != nil {
		return err
	}
	if state == "seen" {
		fmt.Fprintf(e.Out, "  done: %s\n", a.Label)
	}
	return nil
}
