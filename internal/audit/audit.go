// Package audit builds aayudha's audit report (WP7.6): it reconciles GitHub's
// own event history (rule-suite bypasses, issue and PR timelines, workflow
// runs) against the actions segments on the ledger (ground rule 6) and reports
// solo-mode bypasses, founder-identity ledger writes, promotions, tag ruleset
// bypasses, human-fallback writes, unlogged writes, proof coverage, floors,
// waivers, decision SLAs, autonomy exceptions, and the licence scan.
//
// Build is a pure function of Inputs and Data, and Fetch reads every GitHub
// source only up to Inputs.Until (the event cursor) and the ledger only at
// Inputs.Ledger, so a report regenerated with the inputs recorded in its header
// is byte-identical.
package audit

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/autonomy"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
)

// Inputs are what a report is a function of; its header records them.
type Inputs struct {
	Repo       string    `json:"repo"`
	Since      time.Time `json:"since"`
	Until      time.Time `json:"until"` // the GitHub event cursor: nothing at or after it is read
	Ledger     string    `json:"ledger"`
	Code       string    `json:"code"` // the dev commit at Until, whose go.mod and notices the licence scan reads
	ConfigHash string    `json:"config_hash"`
	Segments   []Segment `json:"segments"`
	// Snapshot is the ledger path of the GitHub inputs snapshot written with
	// the report (Freeze), and SnapshotSHA256 its hash.
	Snapshot       string `json:"snapshot,omitempty"`
	SnapshotSHA256 string `json:"snapshot_sha256,omitempty"`
}

// Segment is one actions segment on the ledger.
type Segment struct {
	Path    string `json:"path"`
	SHA256  string `json:"sha256"`
	Records int    `json:"records"`
}

// RuleSuite is one ruleset evaluation GitHub recorded with result "bypass".
type RuleSuite struct {
	ID       int64     `json:"id"`
	Ref      string    `json:"ref"`
	Actor    string    `json:"actor"` // "" when GitHub lists none (a deploy key, D1)
	Before   string    `json:"before_sha"`
	After    string    `json:"after_sha"`
	PushedAt time.Time `json:"pushed_at"`
	Failed   []Rule    `json:"failed_rules"`
}

// Rule is a failed (bypassed) rule of a rule suite.
type Rule struct {
	Ruleset string `json:"ruleset"`
	Type    string `json:"rule_type"`
	Details string `json:"details,omitempty"`
}

// PR is a merged pull request.
type PR struct {
	Number   int       `json:"number"`
	Title    string    `json:"title"`
	Base     string    `json:"base"`
	Head     string    `json:"head"`
	Author   string    `json:"author"`
	MergedAt time.Time `json:"merged_at"`
	MergeSHA string    `json:"merge_sha"`
	Refs     []int     `json:"refs,omitempty"`           // stories it delivers: a story/<n>- branch, Refs/Closes/Fixes/Resolves #n
	Mentions []int     `json:"story_mentions,omitempty"` // "Story #n" in its body
	Body     string    `json:"-"`                        // read by LinkStories when Refs and Mentions are unset
}

// Item is an issue or pull request with its timeline before Until.
type Item struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	PR        bool      `json:"pr"`
	Author    string    `json:"author"`
	CreatedAt time.Time `json:"created_at"`
	Labels    []string  `json:"labels"` // at Until
	Events    []Event   `json:"events"` // nil when the timeline was not read (no change since Since)
	ClosedAt  time.Time `json:"closed_at,omitzero"`
}

// Event is one timeline event.
type Event struct {
	Kind     string    `json:"kind"`
	Actor    string    `json:"actor"`
	At       time.Time `json:"at"`
	Label    string    `json:"label,omitempty"`
	Source   int       `json:"source,omitempty"` // cross-referenced: the referencing issue or PR
	SourcePR bool      `json:"source_pr,omitempty"`
	Body     string    `json:"body,omitempty"` // commented
	ID       int64     `json:"id,omitempty"`
	State    string    `json:"state,omitempty"` // reviewed
}

// Run is a workflow run.
type Run struct {
	ID         int64     `json:"id"`
	Workflow   string    `json:"workflow"` // file path
	Event      string    `json:"event"`
	Actor      string    `json:"actor"`
	Branch     string    `json:"head_branch"`
	HeadSHA    string    `json:"head_sha"`
	Conclusion string    `json:"conclusion"`
	CreatedAt  time.Time `json:"created_at"`
}

// Pack is a proof pack on the ledger and whether it covers its story.
type Pack struct {
	Path       string        `json:"path"`
	Story      int           `json:"story"`
	Head       string        `json:"head"`
	Kind       string        `json:"kind"`
	CreatedAt  string        `json:"created_at"`
	Checks     []proof.Check `json:"checks"`
	Checkpoint string        `json:"checkpoint,omitempty"` // k/N of the checkpoint the pack proves
	CostUSD    float64       `json:"cost_usd,omitempty"`
	Covers     bool          `json:"covers"`            // authoritative, verifies against the Ready snapshot, floors hold
	Problem    string        `json:"problem,omitempty"` // why an authoritative pack does not cover
}

// Ship is a ship record on the ledger.
type Ship struct {
	Path      string    `json:"path"`
	Story     int       `json:"story"`
	PR        int       `json:"pr"`
	Merge     string    `json:"merge_commit"`
	Pass      string    `json:"human_pass"`
	Login     string    `json:"login"`
	Comment   string    `json:"comment_url,omitempty"`
	ShippedAt time.Time `json:"shipped_at"`
}

// Record is an actions record from a segment.
type Record struct {
	actions.Record
	Segment string `json:"segment"`
}

// PriorReport is an earlier audit report (hand-run or `aayu audit`) on the ledger.
type PriorReport struct {
	Path   string  `json:"path"`
	Suites []int64 `json:"suites"` // rule suites it lists in a table row
}

// NeverMerge is an agent PR merged on a never-layer ([autonomy] never).
type NeverMerge struct {
	PR     int       `json:"pr"`
	At     time.Time `json:"merged_at"`
	Layers []string  `json:"layers"`
}

// AgentPR is an agent PR merged before Until, with its layer footprint and the
// skills it changed.
type AgentPR struct {
	Number   int       `json:"pr"`
	MergedAt time.Time `json:"merged_at"`
	Layers   []string  `json:"layers"`
	Skills   []string  `json:"skills,omitempty"`
}

// Licence is the licence scan input at Inputs.Code.
type Licence struct {
	Modules  []Module `json:"modules"`  // go.mod requirements
	Notices  string   `json:"notices"`  // THIRD_PARTY_NOTICES.md
	Security []string `json:"security"` // conclusions of the CI security job on Code, completed before Until
}

// Module is a go.mod requirement.
type Module struct {
	Path    string `json:"path"`
	Version string `json:"version"`
}

// Data is everything Fetch read.
type Data struct {
	Dev, Ledger  string    // branch names
	GateCheck    string    // the protected-paths check name
	GateSince    time.Time // when the merge gate workflow was registered; zero when it never was
	Bots         []string  // [identity] bot_logins
	RuleSuites   []RuleSuite
	PRs          []PR
	Items        []Item
	Runs         []Run
	LedgerLog    []ledger.Entry
	Packs        []Pack
	Ships        []Ship
	Records      []Record
	Prior        []PriorReport
	Corrections  []autonomy.Correction
	NeverMerges  []NeverMerge
	AgentPRs     []AgentPR
	Streaks      []autonomy.LayerStreak
	Subjects     map[string]string // commit SHA -> subject (promotion commits)
	OnMain       map[string]bool   // tag commit -> reachable from main
	Licence      Licence
	SLAHours     int
	Clock        config.Clock
	AllowedLicns []string
}

// Delivery statuses: a story in one of them is done for proof coverage (WP2.8).
var doneStatus = []string{"merged", "released", "measured", "done"}

// Bypass classes.
const (
	ClassSolo      = "solo-mode bypass"
	ClassLedger    = "ledger write (D1 day-one rule)"
	ClassPromotion = "promotion"
	ClassTag       = "tag ruleset bypass"
	ClassOther     = "unexplained"
)

// Bypass is a classified rule suite.
type Bypass struct {
	RuleSuite
	Class     string `json:"class"`
	Accounted bool   `json:"accounted"`
	PR        int    `json:"pr,omitempty"`
	Comment   string `json:"comment,omitempty"` // the prior "Solo-mode bypass:" comment
	Note      string `json:"note"`
	Logged    bool   `json:"logged,omitempty"` // a ledger write in an actions segment
}

// Write is one GitHub write from the event history and how it reconciles.
type Write struct {
	Target string    `json:"target"` // #N, workflow run, or ledger commit
	Kind   string    `json:"kind"`
	Actor  string    `json:"actor"`
	At     time.Time `json:"at"`
	Logged string    `json:"logged,omitempty"` // the segment holding its record
	CI     bool      `json:"ci,omitempty"`     // a workflow or App write: its run is the record
}

// Story is a delivered story or bug and its proof.
type Story struct {
	Number  int       `json:"number"`
	Title   string    `json:"title"`
	Status  string    `json:"status"`
	PR      int       `json:"pr,omitempty"`
	Merged  time.Time `json:"merged_at,omitzero"`
	Covered bool      `json:"covered"`
	PreGate bool      `json:"pre_gate"`
	Pack    string    `json:"pack,omitempty"`
	Reason  string    `json:"reason,omitempty"`
}

// Decision is a Decision's SLA state.
type Decision struct {
	Number      int     `json:"number"`
	Title       string  `json:"title"`
	Open        bool    `json:"open"`
	Hours       float64 `json:"business_hours"`
	Over        bool    `json:"over_sla"`
	Provisional bool    `json:"provisional"`
}

// Report is a built audit.
type Report struct {
	Inputs      Inputs                `json:"inputs"`
	NewestSuite int64                 `json:"newest_rule_suite,omitempty"`
	GateSince   time.Time             `json:"merge_gate_since,omitzero"`
	Bypasses    []Bypass              `json:"bypasses"`
	SoloShips   []Ship                `json:"solo_ships"`
	Fallback    []Write               `json:"human_fallback_writes"`
	FallbackLdg []ledger.Entry        `json:"human_fallback_ledger_commits"`
	Writes      []Write               `json:"writes"`
	Stories     []Story               `json:"stories"`
	Packs       []Pack                `json:"packs"`
	Decisions   []Decision            `json:"decisions"`
	Corrections []autonomy.Correction `json:"corrections"`
	NeverMerges []NeverMerge          `json:"never_merges"`
	Licence     []string              `json:"licence_findings"`
	Security    []string              `json:"security_job"`
	Modules     int                   `json:"modules_scanned"`
}

// Counts summarise a report.
type Counts struct {
	Bypasses, Unexplained, Solo, SoloShips, LedgerWrites, Promotions, Tags int
	Fallback, Writes, Unlogged, CI                                         int
	Delivered, Covered, PreGate, PostGate                                  int
	FloorMisses, Waivers, OverSLA, Corrections, NeverMerges, Licence       int
}

// Counts computes the summary.
func (r *Report) Counts() Counts {
	var c Counts
	c.Bypasses = len(r.Bypasses)
	for _, b := range r.Bypasses {
		if !b.Accounted {
			c.Unexplained++
		}
		switch b.Class {
		case ClassSolo:
			c.Solo++
		case ClassLedger:
			c.LedgerWrites++
		case ClassPromotion:
			c.Promotions++
		case ClassTag:
			c.Tags++
		}
	}
	c.SoloShips = len(r.SoloShips)
	c.Fallback = len(r.Fallback) + len(r.FallbackLdg)
	c.Writes = len(r.Writes)
	for _, w := range r.Writes {
		switch {
		case w.CI:
			c.CI++
		case w.Logged == "":
			c.Unlogged++
		}
	}
	c.Delivered = len(r.Stories)
	for _, s := range r.Stories {
		switch {
		case s.Covered:
			c.Covered++
		case s.PreGate:
			c.PreGate++
		default:
			c.PostGate++
		}
	}
	for _, p := range r.Packs {
		for _, ch := range p.Checks {
			switch {
			case ch.Result == proof.Waived:
				c.Waivers++
			case ch.Result == proof.Fail || ch.Result == proof.Blocked || proof.BlocksShip(ch):
				if p.Kind == proof.Authoritative {
					c.FloorMisses++
				}
			}
		}
	}
	for _, d := range r.Decisions {
		if d.Over {
			c.OverSLA++
		}
	}
	c.Corrections, c.NeverMerges, c.Licence = len(r.Corrections), len(r.NeverMerges), len(r.Licence)
	return c
}

func (d *Data) in(in Inputs, t time.Time) bool { return !t.Before(in.Since) && t.Before(in.Until) }

// Build computes the report.
func Build(in Inputs, d *Data) *Report {
	r := &Report{Inputs: in, Bypasses: []Bypass{}, SoloShips: []Ship{}, Fallback: []Write{}, FallbackLdg: []ledger.Entry{}, Writes: []Write{},
		Stories: []Story{}, Packs: []Pack{}, Decisions: []Decision{}, Corrections: []autonomy.Correction{}, NeverMerges: []NeverMerge{}, Licence: []string{}, Security: []string{}}
	for _, s := range d.RuleSuites {
		if d.in(in, s.PushedAt) {
			r.Bypasses = append(r.Bypasses, d.classify(s))
			r.NewestSuite = max(r.NewestSuite, s.ID)
		}
	}
	sort.SliceStable(r.Bypasses, func(i, j int) bool { return r.Bypasses[i].ID < r.Bypasses[j].ID })
	for _, s := range d.Ships {
		if s.Pass == "solo-bypass" && d.in(in, s.ShippedAt) {
			r.SoloShips = append(r.SoloShips, s)
		}
	}
	r.Writes = d.writes(in)
	for _, rec := range d.Records {
		if rec.Identity == "human-fallback" && rec.Status == actions.StatusDone && d.in(in, rec.Time) {
			r.Fallback = append(r.Fallback, Write{Target: rec.Target, Kind: rec.Kind, Actor: "human-fallback", At: rec.Time.UTC(), Logged: rec.Segment})
		}
	}
	for _, it := range d.Items {
		for _, e := range it.Events {
			if e.Kind == "labeled" && e.Label == "identity:human-fallback" && d.in(in, e.At) {
				r.Fallback = append(r.Fallback, Write{Target: fmt.Sprintf("#%d", it.Number), Kind: "work item tagged identity:human-fallback", Actor: e.Actor, At: e.At})
			}
		}
	}
	sort.SliceStable(r.Fallback, func(i, j int) bool { return r.Fallback[i].At.Before(r.Fallback[j].At) })
	for _, e := range d.LedgerLog {
		if e.Identity == "identity:human-fallback" && d.in(in, e.At) {
			r.FallbackLdg = append(r.FallbackLdg, e)
		}
	}
	r.Stories, r.GateSince = d.stories(in), d.GateSince
	for _, p := range d.Packs {
		if t, err := parseTime(p.CreatedAt); err == nil && d.in(in, t) {
			r.Packs = append(r.Packs, p)
		}
	}
	r.Decisions = d.decisions(in)
	for _, c := range d.Corrections {
		if d.in(in, c.At) {
			r.Corrections = append(r.Corrections, c)
		}
	}
	for _, m := range d.NeverMerges {
		if d.in(in, m.At) {
			r.NeverMerges = append(r.NeverMerges, m)
		}
	}
	r.Licence, r.Modules = d.licence(), len(d.Licence.Modules)
	r.Security = append(r.Security, d.Licence.Security...)
	return r
}

var quoted = regexp.MustCompile(`"([^"]+)"`)

// classify explains one bypass from the event history and the ledger.
func (d *Data) classify(s RuleSuite) Bypass {
	b := Bypass{RuleSuite: s, Class: ClassOther}
	ref := s.Ref
	switch {
	case ref == "refs/heads/"+d.Ledger:
		b.Class = ClassLedger
		e, ok := d.ledgerCommit(s.After)
		switch {
		case !ok:
			b.Note = "ledger commit " + short(s.After) + " is not in the ledger's history at the recorded commit"
		case s.Actor == "":
			b.Accounted, b.Note = true, "deploy-key push (no actor listed, D1), identified by ledger commit "+short(s.After)+": "+e.Subject
		default:
			b.Accounted, b.Note = true, "founder-identity ledger write by "+s.Actor+": "+e.Subject
		}
		b.Logged = d.ledgerLogged(s.After, e.Subject)
	case strings.HasPrefix(ref, "refs/heads/") && (ref == "refs/heads/rolling" || ref == "refs/heads/main"):
		b.Class = ClassPromotion
		branch := strings.TrimPrefix(ref, "refs/heads/")
		subj := d.Subjects[s.After]
		want := map[string]string{"rolling": "Rolling snapshot: ", "main": "Release: v"}[branch]
		run := d.gateRun(s.After, s.PushedAt)
		switch {
		case !strings.HasPrefix(subj, want):
			b.Note = fmt.Sprintf("push to %s whose commit %s is not a %q merge (%q)", branch, short(s.After), strings.TrimSpace(want), subj)
		case run == nil:
			b.Note = fmt.Sprintf("%q with no successful gate run on %s started before the push", subj, short(s.After))
		default:
			b.Accounted, b.Note = true, fmt.Sprintf("agent-run promotion %q after gate run %d (%s) passed", subj, run.ID, run.Branch)
		}
	case strings.HasPrefix(ref, "refs/tags/"):
		b.Class = ClassTag
		tag := strings.TrimPrefix(ref, "refs/tags/")
		del := strings.Trim(s.After, "0") == ""
		rel := d.releaseRun(tag)
		switch {
		case del && (rel == nil || rel.Conclusion != "success"):
			b.Accounted, b.Note = true, "deletion of tag "+tag+", which never released"
		case del:
			b.Note = "deletion of released tag " + tag
		case d.OnMain[s.After]:
			b.Accounted, b.Note = true, "release tag "+tag+" on main"
		case rel != nil && rel.Conclusion != "success":
			b.Accounted, b.Note = true, fmt.Sprintf("tag %s is not on main; release run %d refused it (%s), as rule 8 requires", tag, rel.ID, rel.Conclusion)
		default:
			b.Note = "tag " + tag + " is not on main and no release run refused it"
		}
	case s.Actor == "":
		b.Note = "push to " + ref + " with no actor listed (a deploy key, D1) outside the ledger"
	default:
		pr := d.mergedPR(s.After)
		if pr == nil {
			b.Note = "push to " + ref + " that is no pull request's merge commit (a direct push)"
			break
		}
		b.PR = pr.Number
		other := d.otherFailures(s)
		cm := d.soloComment(pr.Number, s.PushedAt)
		b.Class = ClassSolo
		switch {
		case len(other) > 0:
			b.Class, b.Note = ClassOther, fmt.Sprintf("#%d merged past failing rules other than the %s check: %s", pr.Number, d.GateCheck, strings.Join(other, "; "))
		case cm == "":
			b.Note = fmt.Sprintf("#%d merged into %s past the %s check with no prior \"Solo-mode bypass:\" comment", pr.Number, pr.Base, d.GateCheck)
		default:
			b.Accounted, b.Comment = true, cm
			b.Note = fmt.Sprintf("#%d merged into %s past only the %s check, after a \"Solo-mode bypass:\" comment", pr.Number, pr.Base, d.GateCheck)
		}
	}
	if !b.Accounted {
		for _, p := range d.Prior {
			if slices.Contains(p.Suites, s.ID) {
				b.Accounted, b.Note = true, b.Note+"; accounted for by "+p.Path
				break
			}
		}
	}
	return b
}

// otherFailures lists the failed rules of a merge other than the protected-paths
// status check: anything else means a required check was bypassed.
func (d *Data) otherFailures(s RuleSuite) []string {
	var out []string
	for _, f := range s.Failed {
		if f.Type == "required_status_checks" {
			names := quoted.FindAllStringSubmatch(f.Details, -1)
			ok := len(names) > 0
			for _, n := range names {
				ok = ok && n[1] == d.GateCheck
			}
			if ok {
				continue
			}
		}
		out = append(out, fmt.Sprintf("%s (%s) %s", f.Type, f.Ruleset, f.Details))
	}
	return out
}

func (d *Data) ledgerCommit(sha string) (ledger.Entry, bool) {
	for _, e := range d.LedgerLog {
		if e.SHA == sha {
			return e, true
		}
	}
	return ledger.Entry{}, false
}

// ledgerLogged reports whether a ledger commit is in an actions segment: a
// ledger.push record naming it, or a segment commit, which carries its own log.
func (d *Data) ledgerLogged(sha, subject string) bool {
	if strings.HasPrefix(subject, "actions: segment ") {
		return true
	}
	for _, r := range d.Records {
		if r.Kind == "ledger.push" && resultField(r.Result, "commit") == sha {
			return true
		}
	}
	return false
}

// gateRun is a successful ci.yml gate run on sha that started before the push.
func (d *Data) gateRun(sha string, pushed time.Time) *Run {
	for i := range d.Runs {
		r := &d.Runs[i]
		if r.HeadSHA == sha && r.Event == "workflow_dispatch" && r.Conclusion == "success" && strings.HasSuffix(r.Workflow, "/ci.yml") && r.CreatedAt.Before(pushed) {
			return r
		}
	}
	return nil
}

func (d *Data) releaseRun(tag string) *Run {
	for i := range d.Runs {
		if r := &d.Runs[i]; r.Branch == tag && strings.HasSuffix(r.Workflow, "/release.yml") {
			return r
		}
	}
	return nil
}

func (d *Data) mergedPR(sha string) *PR {
	for i := range d.PRs {
		if d.PRs[i].MergeSHA == sha {
			return &d.PRs[i]
		}
	}
	return nil
}

func (d *Data) item(n int) *Item {
	for i := range d.Items {
		if d.Items[i].Number == n {
			return &d.Items[i]
		}
	}
	return nil
}

// soloComment is the URL of the first comment on PR n starting
// "Solo-mode bypass:" posted at or before the merge.
func (d *Data) soloComment(n int, merged time.Time) string {
	it := d.item(n)
	if it == nil {
		return ""
	}
	for _, e := range it.Events {
		if e.Kind == "commented" && !e.At.After(merged) && strings.HasPrefix(strings.TrimSpace(e.Body), "Solo-mode bypass:") {
			return fmt.Sprintf("#%d (comment %d, %s)", n, e.ID, stamp(e.At))
		}
	}
	return ""
}

// stories lists the delivered stories and bugs: a done status at Until, or a
// merged PR that serves them; each is covered by an authoritative
// pack or not.
func (d *Data) stories(in Inputs) []Story {
	out := []Story{}
	for _, it := range d.Items {
		if it.PR || (!slices.Contains(it.Labels, "type:story") && !slices.Contains(it.Labels, "type:bug")) {
			continue
		}
		s := Story{Number: it.Number, Title: it.Title, Status: statusOf(it.Labels)}
		if pr := d.mergePR(it); pr != nil {
			s.PR, s.Merged = pr.Number, pr.MergedAt
		}
		if s.PR == 0 && !slices.Contains(doneStatus, s.Status) {
			continue
		}
		for _, p := range d.Packs {
			if p.Story != s.Number || p.Kind != proof.Authoritative {
				continue
			}
			if p.Covers {
				s.Covered, s.Pack, s.Reason = true, p.Path, ""
				break
			}
			s.Pack, s.Reason = p.Path, "authoritative pack does not cover: "+p.Problem
		}
		if !s.Covered {
			s.PreGate = s.PR != 0 && (d.GateSince.IsZero() || s.Merged.Before(d.GateSince))
			switch {
			case s.Reason != "":
			case s.PreGate:
				s.Reason = "merged without proof (pre-gate)"
			case s.PR == 0:
				s.Reason = "done without a merged PR or an authoritative pack"
			default:
				s.Reason = "merged without proof"
			}
		}
		out = append(out, s)
	}
	return out
}

var (
	serveRef   = regexp.MustCompile(`(?i)\b(?:refs|closes|closed|close|fixes|fixed|fix|resolves|resolved|resolve)\s+#([0-9]+)\b`)
	storyRef   = regexp.MustCompile(`(?i)\bstory:?\s+#([0-9]+)\b`)
	storyBrnch = regexp.MustCompile(`^story/([0-9]+)-`)
)

// LinkStories fills a PR's Refs and Mentions from its body and head branch.
func (p *PR) LinkStories() {
	add := func(dst *[]int, re *regexp.Regexp, text string) {
		for _, m := range re.FindAllStringSubmatch(text, -1) {
			var n int
			if _, err := fmt.Sscan(m[1], &n); err == nil && !slices.Contains(*dst, n) {
				*dst = append(*dst, n)
			}
		}
	}
	p.Refs, p.Mentions = []int{}, []int{}
	add(&p.Refs, storyBrnch, p.Head)
	add(&p.Refs, serveRef, p.Body)
	add(&p.Mentions, storyRef, p.Body)
}

// serves reports whether a PR delivers story n: a story/<n>- branch or "Refs #n"
// (Closes, Fixes, Resolves) in its body, or, in a PR that refs no story that
// way (written before the convention), "Story #n" or "Story: #n".
func serves(p PR, n int) bool {
	if p.Refs == nil && p.Mentions == nil {
		p.LinkStories()
	}
	return slices.Contains(p.Refs, n) || (len(p.Refs) == 0 && slices.Contains(p.Mentions, n))
}

// mergePR is the first PR merged into dev that delivers the item.
func (d *Data) mergePR(it Item) *PR {
	var best *PR
	for i := range d.PRs {
		p := &d.PRs[i]
		if p.Base == d.Dev && serves(*p, it.Number) && (best == nil || p.MergedAt.Before(best.MergedAt)) {
			best = p
		}
	}
	return best
}

// MergePR is the PR that delivered story n, if Fetch saw one.
func (d *Data) MergePR(n int) *PR {
	if it := d.item(n); it != nil {
		return d.mergePR(*it)
	}
	return nil
}

func statusOf(labels []string) string {
	for _, l := range labels {
		if s, ok := strings.CutPrefix(l, "status:"); ok {
			return s
		}
	}
	return ""
}

func (d *Data) decisions(in Inputs) []Decision {
	out := []Decision{}
	for _, it := range d.Items {
		if it.PR || !slices.Contains(it.Labels, "type:decision") || !it.CreatedAt.Before(in.Until) {
			continue
		}
		end, open := in.Until, it.ClosedAt.IsZero() || !it.ClosedAt.Before(in.Until)
		if !open {
			if it.ClosedAt.Before(in.Since) {
				continue
			}
			end = it.ClosedAt
		}
		h := d.businessHours(it.CreatedAt, end)
		out = append(out, Decision{Number: it.Number, Title: it.Title, Open: open, Hours: h, Over: d.SLAHours > 0 && h > float64(d.SLAHours),
			Provisional: slices.Contains(it.Labels, "decision:provisional")})
	}
	return out
}

// businessHours counts the business hours of [from, to) by the [time] clock.
func (d *Data) businessHours(from, to time.Time) float64 {
	c := d.Clock
	sum := to.Sub(from)
	if c.Loc != nil {
		sum = 0
		t := from.In(c.Loc)
		for day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, c.Loc); day.Before(to); day = day.AddDate(0, 0, 1) {
			if !c.Days[day.Weekday()] {
				continue
			}
			s := time.Date(day.Year(), day.Month(), day.Day(), c.Start/60, c.Start%60, 0, 0, c.Loc)
			e := time.Date(day.Year(), day.Month(), day.Day(), c.End/60, c.End%60, 0, 0, c.Loc)
			if s.Before(from) {
				s = from
			}
			if e.After(to) {
				e = to
			}
			if e.After(s) {
				sum += e.Sub(s)
			}
		}
	}
	return float64(int(sum.Hours()*10+0.5)) / 10
}

// licence checks every go.mod requirement against THIRD_PARTY_NOTICES.md: a
// row with the module and version whose licence is on the allow list.
func (d *Data) licence() []string {
	out := []string{}
	rows := map[string][2]string{}
	for _, line := range strings.Split(d.Licence.Notices, "\n") {
		f := strings.Split(line, "|")
		if len(f) < 5 {
			continue
		}
		mod, ver := strings.Trim(strings.TrimSpace(f[1]), "`"), strings.TrimSpace(f[2])
		if _, seen := rows[mod]; !seen && strings.HasPrefix(ver, "v") {
			rows[mod] = [2]string{ver, strings.TrimSpace(f[3])}
		}
	}
	deny := regexp.MustCompile(`(?i)\b(A?GPL|LGPL|MPL|SSPL|BUSL|EUPL|CC-BY-NC|proprietary)`)
	for _, m := range d.Licence.Modules {
		row, ok := rows[m.Path]
		switch {
		case !ok:
			out = append(out, fmt.Sprintf("`%s` %s is not in THIRD_PARTY_NOTICES.md", m.Path, m.Version))
		case row[0] != m.Version:
			out = append(out, fmt.Sprintf("`%s` is %s in go.mod but %s in THIRD_PARTY_NOTICES.md", m.Path, m.Version, row[0]))
		case deny.MatchString(row[1]) || !slices.ContainsFunc(d.AllowedLicns, func(a string) bool { return strings.Contains(row[1], a) }):
			out = append(out, fmt.Sprintf("`%s` %s: licence %q is not on the allow list (%s)", m.Path, m.Version, row[1], strings.Join(d.AllowedLicns, ", ")))
		}
	}
	return out
}

func resultField(v any, k string) string {
	if m, ok := v.(map[string]any); ok {
		if s, ok := m[k].(string); ok {
			return s
		}
		if f, ok := m[k].(float64); ok {
			return fmt.Sprint(int64(f))
		}
	}
	if f, ok := v.(float64); ok && (k == "number" || k == "id") {
		return fmt.Sprint(int64(f))
	}
	return ""
}

func short(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

func stamp(t time.Time) string { return t.UTC().Format(time.RFC3339) }

func parseTime(s string) (time.Time, error) { return time.Parse(time.RFC3339, s) }
