package cli

import (
	"bufio"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/ready"
	"github.com/rahulnakmol/agentic-ayudha/internal/scope"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// The overnight card (Story #1178, Decision #1168). Once the evening review
// is done, `aayu inbox` proposes what runs tonight: the authoring of the next
// Feature in the release order to the approval line, the verify sweep, and
// the evals. Each run has the bounds of every build, [autonomy.budget]
// wall_hours and cost_usd, read from config and never from a prompt, and ends
// at the next start of [time] business_hours. A person approves it under the
// session's device code, which writes missions/overnight/<date>.json; nothing
// is set by default, so a card left writes nothing and nothing of it runs.
// The hourly tick (`aayu mission tick`) starts each run of the plan, stops one
// that reaches a bound, and the morning inbox names it. Missions start no
// story outside the day (missionnext.go).

// OvernightSchema is the schema of an overnight plan.
const OvernightSchema = "aayudha.overnight/v1"

// OvernightPath is where the overnight plan set on the evening of date (in
// [time] tz, 2006-01-02) lives on the ledger.
func OvernightPath(date string) string { return "missions/overnight/" + date + ".json" }

// The runs an overnight card proposes.
const (
	RunAuthor = "author"
	RunSweep  = "verify-sweep"
	RunEvals  = "evals"
)

// How an overnight run stopped: at its hour bound, at its cost bound, or at
// the next start of business hours.
const (
	BoundHours = "hours"
	BoundCost  = "cost"
	BoundDay   = "day"
)

// The overnight card's own step, and its two cards: tonight's plan, and the
// morning's report of a run that stopped at a bound.
const (
	overnightApprove = "inbox: approve tonight's runs"
	overnightTonight = "tonight"
	overnightStopped = "stopped"
)

// OvernightPlan is what a person set to run overnight on the evening of Date.
type OvernightPlan struct {
	Schema   string         `json:"schema"`
	Date     string         `json:"date"`
	By       string         `json:"by"`
	At       string         `json:"at"`
	Presence ready.Presence `json:"presence"` // the session's device-flow approval, with its presence.approve record
	Runs     []OvernightRun `json:"runs"`
}

// OvernightRun is one run of an overnight plan, with its bounds.
type OvernightRun struct {
	Run     string         `json:"run"`
	Feature int            `json:"feature,omitempty"` // the Feature an author run drafts
	Does    string         `json:"does"`
	Hours   float64        `json:"hours"`    // [autonomy.budget] wall_hours; 0: only its end bounds it
	CostUSD float64        `json:"cost_usd"` // [autonomy.budget] cost_usd; 0: no cost bound
	Ends    string         `json:"ends_at"`  // the next start of [time] business_hours: no run goes past it
	Started string         `json:"started_at,omitempty"`
	Stopped *OvernightStop `json:"stopped,omitempty"`
}

// OvernightStop is when and why a run stopped.
type OvernightStop struct {
	At    string `json:"at"`
	Bound string `json:"bound"` // hours, cost, or day
	Why   string `json:"why"`
}

// name is the run as a person reads it: "author #982", "verify-sweep".
func (r OvernightRun) name() string {
	if r.Feature != 0 {
		return fmt.Sprintf("%s #%d", r.Run, r.Feature)
	}
	return r.Run
}

// bounds are the run's bounds in words: "8 h, 60 USD, and 09:00 Tue 3 Nov Europe/London".
func (r OvernightRun) bounds(clock config.Clock) string {
	var b []string
	if r.Hours > 0 {
		b = append(b, hrs(r.Hours)+" h")
	}
	if r.CostUSD > 0 {
		b = append(b, usd(r.CostUSD)+" USD")
	}
	ends, _ := time.Parse(time.RFC3339, r.Ends)
	return andList(append(b, "the start of the day, "+dayTime(clock, ends)))
}

// localDate is t's date in [time] tz.
func localDate(clock config.Clock, t time.Time) string { return t.In(clock.Loc).Format(time.DateOnly) }

// overnightRuns are the runs a card proposes tonight at now: the authoring of
// Feature f, the verify sweep, and the evals, each with the bounds of every
// build, from config, and the next start of the day.
func overnightRuns(cfg *config.Config, clock config.Clock, now time.Time, f int) []OvernightRun {
	runs := []OvernightRun{
		{Run: RunAuthor, Feature: f, Does: fmt.Sprintf("author #%d to the approval line: an agent drafts each part of its baseline (`aayu ready %d --part <part> --request`); "+
			"drafts only, every approval stays in the morning", f, f)},
		{Run: RunSweep, Does: "the verify sweep: every feature-map behaviour check on the dev tip (`aayu verify --check all`)"},
		{Run: RunEvals, Does: "the evals: the tier-2 evals of the dev tip (evals.yml)"},
	}
	b := cfg.Autonomy.Budget
	for i := range runs {
		runs[i].Hours, runs[i].CostUSD, runs[i].Ends = b.WallHours, b.CostUSD, clock.Next(now).UTC().Format(time.RFC3339)
	}
	return runs
}

// releaseOrder are the open Features planned in a release, in the release
// order: by version, then by number, as a release's scope plan lists them.
func (s *inboxSource) releaseOrder() []int {
	type planned struct {
		n int
		v []int
	}
	var fs []planned
	for n, is := range s.issues {
		m := scope.VersionRE.FindStringSubmatch(is.GetMilestone().GetTitle())
		if is.IsPullRequest() || is.GetState() != "open" || m == nil {
			continue
		}
		if typ, _ := workitems.TypeOf(is.GetType().GetName(), github.IssueLabels(is)); typ != "feature" {
			continue
		}
		p := planned{n: n}
		for _, x := range m[1:] {
			v, _ := strconv.Atoi(x)
			p.v = append(p.v, v)
		}
		fs = append(fs, p)
	}
	slices.SortFunc(fs, func(a, b planned) int { return cmp.Or(slices.Compare(a.v, b.v), a.n-b.n) })
	out := make([]int, len(fs))
	for i, p := range fs {
		out[i] = p.n
	}
	return out
}

// overnightItems are the overnight cards, each titled with its Feature.
// reviewDone says no evening card waits.
func (s *inboxSource) overnightItems(ctx context.Context, reviewDone bool) ([]InboxItem, error) {
	cards, err := s.overnight(ctx, s.releaseOrder(), reviewDone)
	if err != nil {
		return nil, err
	}
	var out []InboxItem
	for _, it := range cards {
		if is := s.issues[it.Number]; is != nil {
			it.Title = termText(it.Title + ": " + is.GetTitle())
			it.URL, it.Evidence, it.Updated, it.labels = is.GetHTMLURL(), is.GetHTMLURL(), is.GetUpdatedAt().Time, github.IssueLabels(is)
		}
		out = append(out, it)
	}
	return out, nil
}

// overnightCards reads the overnight plans on the ledger: a card for this
// morning when a run of last night stopped at its bound, and, once today's
// evening review is done and nothing is set tonight, a card that proposes
// tonight's runs for the first of features (the release order) not yet
// approved to build. Before the end of business hours it proposes nothing.
func (e *Env) overnightCards(ctx context.Context, features []int, reviewDone bool) ([]InboxItem, error) {
	l, err := e.Config()
	if err != nil || config.RepoRoot(e.Dir) == "" { // `aayu inbox` itself refuses such a config; no checkout: no ledger
		return nil, nil
	}
	clock, _ := l.Config.Clock() // a config that loaded has a clock (config.Validate)
	_, t, err := e.overnightLedger(ctx)
	if err != nil || t == nil {
		return nil, err
	}
	defer t.Close()
	plans, err := readOvernight(ctx, t)
	if err != nil {
		return nil, err
	}
	now := e.Now()
	today := localDate(clock, now)
	var out []InboxItem
	_, _, evening := eveningDay(clock, now)
	for _, p := range plans { // the morning, until the day ends: last night's runs that stopped at a bound
		if it, ok := morningCard(clock, p, today); ok && !evening {
			out = append(out, it)
		}
	}
	if !evening || !reviewDone || slices.ContainsFunc(plans, func(p OvernightPlan) bool { return p.Date == today }) {
		return out, nil
	}
	for _, f := range features {
		if b, err := t.File(ctx, ready.ApprovalPath(f)); ignoreMissing(err) != nil {
			return nil, err
		} else if b != nil { // approved to build: authored already
			continue
		}
		runs := overnightRuns(&l.Config, clock, now, f)
		it := InboxItem{Kind: InboxOvernight, Number: f, Ask: overnightTonight, Title: "Tonight", Seat: verbs.EngineeringManager, CounselFrom: "facts", Runs: runs}
		it.f.problem = "set tonight's runs"
		it.Lines = append(it.Lines, fmt.Sprintf("Tonight, until %s, the next start of [time] business_hours. Nothing else starts but the loops scheduled before.", dayStart(clock, now)))
		for i, r := range runs {
			it.Lines = append(it.Lines, fmt.Sprintf("%d. %s", i+1, r.Does), "   Bound: "+r.bounds(clock)+".")
		}
		it.Lines = append(it.Lines, "The bounds are [autonomy.budget] wall_hours and cost_usd; a run stops at the first bound it reaches.")
		it.Why = fmt.Sprintf("the evening review is done, #%d is next in the release order, and nothing is set to run tonight.", f)
		it.Counsel = fmt.Sprintf("What this is about: what runs while you are away. #%d is the next Feature in the release order and is not approved to build, so an agent "+
			"can draft its baseline tonight for your morning; the verify sweep and the evals check the dev tip. Each run stops at its bounds. "+
			"Nothing runs that you do not approve here; if you leave it, only the loops scheduled before run.", f)
		it.Options = []InboxAction{
			{N: 1, Label: fmt.Sprintf("approve tonight's %d runs, each within its bounds", len(runs)), Command: overnightApprove},
			{N: 2, Label: "not now: nothing runs tonight", Command: inboxLeave},
		}
		out = append(out, it)
		break
	}
	return out, nil
}

// morningCard is the card that names the runs of plan p that stopped at a
// bound, on the day (today) its night ended.
func morningCard(clock config.Clock, p OvernightPlan, today string) (InboxItem, bool) {
	var stopped []OvernightRun
	f := 0
	for _, r := range p.Runs {
		ends, err := time.Parse(time.RFC3339, r.Ends)
		if err != nil || localDate(clock, ends) != today || p.Date == today {
			return InboxItem{}, false
		}
		if r.Feature != 0 {
			f = r.Feature
		}
		if r.Stopped != nil && r.Stopped.Bound != BoundDay {
			stopped = append(stopped, r)
		}
	}
	if len(stopped) == 0 || f == 0 {
		return InboxItem{}, false
	}
	it := InboxItem{Kind: InboxOvernight, Number: f, Ask: overnightStopped, Title: "Last night", Seat: verbs.EngineeringManager, CounselFrom: "facts"}
	it.f.problem = "a run stopped at its bound"
	it.Lines = append(it.Lines, fmt.Sprintf("Set by @%s on the evening of %s, in %s.", termText(p.By), p.Date, OvernightPath(p.Date)))
	var names []string
	for _, r := range stopped {
		at, _ := time.Parse(time.RFC3339, r.Stopped.At)
		it.Lines = append(it.Lines, fmt.Sprintf("%s stopped at %s: %s.", r.name(), dayTime(clock, at), termText(r.Stopped.Why)))
		names = append(names, r.name())
	}
	it.Why = fmt.Sprintf("%s stopped at %s last night.", andList(names), plural(len(stopped), "its bound", "their bounds"))
	it.Counsel = fmt.Sprintf("What this is about: %s did not finish last night, because %s reached %s. Nothing of it runs again by itself. "+
		"If it is still worth doing, set it again in this evening's overnight card.", andList(names), plural(len(stopped), "it", "they"), plural(len(stopped), "a bound", "their bounds"))
	it.Options = []InboxAction{{N: 1, Label: "not now: nothing is written", Command: inboxLeave}}
	return it, true
}

// overnightLedger is the ledger the person writes, read at its tip; t is nil
// when there is no ledger yet.
func (e *Env) overnightLedger(ctx context.Context) (w *ledger.Writer, t *ledger.Tree, err error) {
	_, tok, err := e.HumanGitHub(ctx)
	if err == nil {
		w, err = e.Ledger(ctx, tok, "human", "")
	}
	if err == nil {
		t, err = w.Read(ctx)
	}
	if errors.Is(err, ledger.ErrNoLedger) {
		return w, nil, nil
	}
	return w, t, err
}

// readOvernight reads every overnight plan on the ledger, oldest first.
func readOvernight(ctx context.Context, t *ledger.Tree) ([]OvernightPlan, error) {
	paths, err := t.List(ctx, "missions/overnight")
	if err != nil {
		return nil, ignoreMissing(err)
	}
	var out []OvernightPlan
	for _, path := range paths {
		b, err := t.File(ctx, path)
		if err != nil {
			return nil, err
		}
		var p OvernightPlan
		if json.Unmarshal(b, &p) != nil || p.Schema != OvernightSchema || OvernightPath(p.Date) != path {
			return nil, fmt.Errorf("ledger %s: not an overnight plan aayu wrote", path)
		}
		out = append(out, p)
	}
	slices.SortFunc(out, func(a, b OvernightPlan) int { return strings.Compare(a.Date, b.Date) })
	return out, nil
}

// overnightAct is a person's approval of tonight's runs, as the card shows
// them, under the session's device code: it writes them to
// missions/overnight/<date>.json once.
func (e *Env) overnightAct(ctx context.Context, it InboxItem) error {
	l, _ := e.Config()           // the inbox loaded it
	clock, _ := l.Config.Clock() // a config that loaded has a clock (config.Validate)
	var effect []string
	for _, r := range it.Runs {
		effect = append(effect, fmt.Sprintf("Runs %s tonight, bounded by %s", r.name(), r.bounds(clock)))
	}
	p, err := e.presenceCheck(ctx, Pending{What: "overnight plan", Effect: effect}, AccessWrite, e.personOnly("the overnight plan"))
	if err != nil {
		return err
	}
	w, t, err := e.overnightLedger(ctx)
	if err != nil || t == nil { // the card came from the ledger
		return Wrap(ExitExternal, cmp.Or(err, ledger.ErrNoLedger))
	}
	now := e.Now()
	plan := OvernightPlan{Schema: OvernightSchema, Date: localDate(clock, now), By: p.Login, At: now.UTC().Format(time.RFC3339),
		Presence: ready.Presence{Login: p.Login, Via: p.Via, Permission: p.Permission, ActionsRecord: e.presenceRecord(p.Login)}, Runs: it.Runs}
	was, err := t.File(ctx, OvernightPath(plan.Date))
	tip := t.Tip
	t.Close()
	if err := ignoreMissing(err); err != nil {
		return Wrap(ExitExternal, err)
	}
	if was != nil { // an evening has one plan: a second session's approval changes nothing
		return Errorf(ExitBlocked, "tonight's runs were set already, in %s; nothing was written", OvernightPath(plan.Date))
	}
	data, _ := json.MarshalIndent(plan, "", "  ")
	if _, err := w.Write(ctx, ledger.Commit{Message: fmt.Sprintf("missions: @%s set tonight's runs (%s)", p.Login, plan.Date),
		Files: map[string][]byte{OvernightPath(plan.Date): append(data, '\n')}, Base: tip}); err != nil {
		return Wrap(ExitExternal, err)
	}
	var names []string
	for _, r := range plan.Runs {
		names = append(names, r.name())
	}
	fmt.Fprintf(e.Out, "  done: @%s set tonight's runs (%s), recorded in %s\n", p.Login, strings.Join(names, ", "), OvernightPath(plan.Date))
	return nil
}

// TickOvernight is what the tick did with one run of an overnight plan.
type TickOvernight struct {
	Date   string `json:"date"`
	Run    string `json:"run"`
	Action string `json:"action"` // start, runs, or stop
	Does   string `json:"does,omitempty"`
	Detail string `json:"detail"`
}

// tickOvernight starts each run of an overnight plan whose night has not
// ended, and stops each started one that reached its hour or cost bound, or
// the start of the day, recording both in its plan. Only a plan a person set
// runs: with none, nothing starts but the loops scheduled before.
func (e *Env) tickOvernight(ctx context.Context, w *ledger.Writer, dryRun bool, rep *TickReport) error {
	l, _ := e.Config()           // loaded and checked by the GitHub client
	clock, _ := l.Config.Clock() // a config that loaded has a clock (config.Validate)
	for try := 1; ; try++ {
		t, err := w.Read(ctx)
		if errors.Is(err, ledger.ErrNoLedger) {
			return nil
		}
		if err != nil {
			return Wrap(ExitExternal, err)
		}
		plans, err := readOvernight(ctx, t)
		tip := t.Tip
		t.Close()
		if err != nil {
			return Wrap(ExitExternal, err)
		}
		var lines []TickOvernight
		files := map[string][]byte{}
		for _, p := range plans {
			changed := false
			for i := range p.Runs {
				line, ch := e.overnightStep(clock, &p.Runs[i])
				if line.Action != "" {
					line.Date = p.Date
					lines = append(lines, line)
				}
				changed = changed || ch
			}
			if changed {
				data, _ := json.MarshalIndent(p, "", "  ")
				files[OvernightPath(p.Date)] = append(data, '\n')
			}
		}
		if len(files) > 0 && !dryRun {
			_, err = w.Write(ctx, ledger.Commit{Message: "missions: the tick starts and stops overnight runs", Files: files, Base: tip})
			if errors.Is(err, ledger.ErrMoved) && try < 3 {
				continue
			}
			if err != nil {
				return Wrap(ExitExternal, err)
			}
		}
		rep.Overnight = lines
		return nil
	}
}

// overnightStep is what the tick does with run r at now, and whether r changed.
func (e *Env) overnightStep(clock config.Clock, r *OvernightRun) (TickOvernight, bool) {
	now := e.Now()
	ends, err := time.Parse(time.RFC3339, r.Ends)
	if r.Stopped != nil || err != nil {
		return TickOvernight{}, false
	}
	at := now.UTC().Format(time.RFC3339)
	if r.Started == "" {
		if !now.Before(ends) { // its night ended before any tick started it
			return TickOvernight{}, false
		}
		r.Started = at
		until := ends
		if bound := now.Add(time.Duration(r.Hours * float64(time.Hour))); r.Hours > 0 && bound.Before(until) {
			until = bound
		}
		detail := "until " + dayTime(clock, until)
		if r.CostUSD > 0 {
			detail += ", or " + usd(r.CostUSD) + " USD spent"
		}
		return TickOvernight{Run: r.name(), Action: "start", Does: r.Does, Detail: detail}, true
	}
	started, _ := time.Parse(time.RFC3339, r.Started)
	ran, spent := now.Sub(started), e.overnightSpend(started)
	switch {
	case r.Hours > 0 && ran >= time.Duration(r.Hours*float64(time.Hour)):
		r.Stopped = &OvernightStop{At: at, Bound: BoundHours, Why: fmt.Sprintf("its %s h bound", hrs(r.Hours))}
	case r.CostUSD > 0 && spent >= r.CostUSD:
		r.Stopped = &OvernightStop{At: at, Bound: BoundCost, Why: fmt.Sprintf("its %s USD bound (%s USD spent)", usd(r.CostUSD), usd(math.Round(spent*100)/100))}
	case !now.Before(ends):
		r.Stopped = &OvernightStop{At: at, Bound: BoundDay, Why: "the start of the day, " + dayTime(clock, ends)}
	default:
		detail := fmt.Sprintf("%s h", hrs(ran.Hours()))
		if r.Hours > 0 {
			detail += " of " + hrs(r.Hours)
		}
		if r.CostUSD > 0 {
			detail += fmt.Sprintf(", %s of %s USD", usd(math.Round(spent*100)/100), usd(r.CostUSD))
		}
		return TickOvernight{Run: r.name(), Action: "runs", Detail: detail}, false
	}
	return TickOvernight{Run: r.name(), Action: "stop", Detail: r.Stopped.Why}, true
}

// overnightSpend is the metered spend in the actions log since t: every
// model call's cost, whichever run made it, so no run passes its cost bound
// unseen. Subscription use reports no cost, as for every build.
func (e *Env) overnightSpend(t time.Time) float64 {
	f, err := os.Open(e.ActionsPath())
	if err != nil {
		return 0
	}
	defer func() { _ = f.Close() }()
	sum := 0.0
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 16<<20)
	for sc.Scan() {
		var r actions.Record
		if json.Unmarshal(sc.Bytes(), &r) == nil && r.CostUSD > 0 && !r.Time.Before(t) {
			sum += r.CostUSD
		}
	}
	return sum
}
