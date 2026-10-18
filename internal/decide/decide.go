// Package decide holds the rules of `aayu decide` (WP5.6, V8): who may accept
// a Decision ([decisions] authority), its SLA clock in business hours ([time]),
// what lapses at the SLA (a page to the authority and an escalation comment
// that mentions [decisions] escalate_to) and at provisional_after_hours (a
// reversible Decision's recommendation applies provisionally, labelled
// decision:provisional, with a ratify-or-revert deadline in its body), and
// floor waivers (`decide --waiver`). The CLI does the GitHub reads and writes.
package decide

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// Markers of the comments decide keeps on a Decision, replaced in place.
const (
	EscalationMarker = "<!-- aayudha:decision-escalation -->"
	SupersededMarker = "<!-- aayudha:decision-superseded -->"
)

// LabelWaiver marks a floor-waiver Decision; `audit` counts them (WP7.6).
const LabelWaiver = workitems.LabelWaiver

// FloorsLayer is the layer that owns the floors (CONSTRAINTS.md, L04): the
// authority key a floor waiver falls back to after its parent's type.
const FloorsLayer = "L04"

// Waivable lists the floors a waiver Decision may name (WP4.3–WP4.9). The
// PII redaction floor is not waivable: a capture holding personal data never
// lands (V3), whoever accepts. eval-bar is this repository's skill-eval bars
// (evals.CheckBars, Story #400): a change that lowers one names an accepted
// eval-bar waiver.
var Waivable = []string{"coverage", "sast", "secrets", "deps", "a11y", "keyboard", "reduced-motion", "text-scale", "targets", "perf", "dast", "api", "screen-reader", "mobile", "error-budget", "eval-bar"}

// Decision is an open or closed Decision issue as decide sees it.
type Decision struct {
	Number     int
	Title      string
	Body       string
	State      string
	Labels     []string
	Created    time.Time
	ParentType string // work-item type of the sub-issue parent, "" without one
	Item       workitems.Item
}

// Waiver reports whether d is a floor-waiver Decision.
func (d Decision) Waiver() bool { return slices.Contains(d.Labels, LabelWaiver) }

// Reversible reports whether the Decision says reversible: true. A waiver is
// never reversible, whatever its body says: an SLA lapse must not let a floor
// miss through (rule 7).
func (d Decision) Reversible() bool {
	return !d.Waiver() && strings.EqualFold(strings.TrimSpace(d.Item.Fields[workitems.FieldReversible]), "true")
}

var (
	waiverRE    = regexp.MustCompile(`(?i)\bwaiv`)
	seatRE      = regexp.MustCompile(`(?i)\bseats?\b` + seatNames())
	humanOnlyRE = regexp.MustCompile(`(?i)\bapprov\w*\b.{0,40}\b(pr|prs|pull requests?)\b|\bpublish|\baayu init\b|\bratchet|\bfloors?\b|` +
		`\.github/|\bskills/|\bops/|constraints\.md|codeowners|\.aayudha/config\.toml|\[(autonomy|models|decisions)[].]`)
)

func seatNames() string {
	var b strings.Builder
	for _, s := range verbs.Seats {
		fmt.Fprintf(&b, `|\b%s\b|\b%s\b`, regexp.QuoteMeta(string(s.ID)), regexp.QuoteMeta(s.Name))
	}
	return b.String()
}

// Excluded says why d's recommendation is never applied provisionally,
// whatever its body says and in every mode (V8, Story #432): a waiver of any
// kind, so no agent waits out a floor or ratchet; a seat change; or a step the
// trust model keeps human-only. "" when none applies. Its title, question,
// and links are read, so a waiver without the label still counts.
func (d Decision) Excluded() string {
	text := d.Title + "\n" + d.Item.Fields[workitems.FieldSummary]
	switch {
	case d.Waiver() || waiverRE.MatchString(text+"\n"+d.Item.Fields[workitems.FieldLinks]):
		return "a waiver is never applied provisionally"
	case seatRE.MatchString(text):
		return "a seat change is never applied provisionally"
	case humanOnlyRE.MatchString(text):
		return "a human-only step of the trust model (a PR approval, publishing, init, a protected path, a floor, or a ratchet) is never applied provisionally"
	}
	return ""
}

// Policy is [decisions] with the [time] clock.
type Policy struct {
	Clock       config.Clock
	SLA         time.Duration // business hours before the page and escalation
	Provisional time.Duration // business hours before a reversible recommendation applies
	Solo        bool          // the solo window applies: at most one human is configured
	EscalateTo  string        // login mentioned on escalation (founder B); empty when unset
}

// NewPolicy reads [decisions] and [time]. In solo mode the provisional window
// is the shorter of provisional_after_hours and solo_provisional_after_hours,
// never under config.MinSoloProvisionalHours; provisional_after_hours = 0
// still turns provisional application off.
func NewPolicy(c *config.Config) (Policy, error) {
	clock, err := c.Clock()
	if err != nil {
		return Policy{}, err
	}
	p := Policy{Clock: clock, SLA: time.Duration(c.Decisions.SLAHours) * time.Hour,
		Provisional: time.Duration(c.Decisions.ProvisionalAfter) * time.Hour,
		EscalateTo:  strings.TrimPrefix(strings.TrimSpace(c.Decisions.EscalateTo), "@")}
	if solo := time.Duration(max(c.Decisions.SoloProvisional, config.MinSoloProvisionalHours)) * time.Hour; Humans(c) <= 1 && p.Provisional > 0 && solo < p.Provisional {
		p.Provisional, p.Solo = solo, true
	}
	return p, nil
}

// Humans counts the distinct human logins the configuration names: paired
// seats' humans and [autonomy] approver. At most one is solo mode.
func Humans(c *config.Config) int {
	seen := map[string]bool{}
	add := func(login string) {
		if l := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(login), "@")); l != "" {
			seen[l] = true
		}
	}
	for _, s := range c.Seats {
		if s.Mode == "paired" {
			add(s.Human)
		}
	}
	add(c.Autonomy.Approver)
	return len(seen)
}

// Deadlines returns when d's SLA lapses and when its recommendation may apply
// provisionally, counted in business hours from its creation.
func (p Policy) Deadlines(d Decision) (sla, provisional time.Time) {
	return p.Clock.Add(d.Created, p.SLA), p.Clock.Add(d.Created, p.Provisional)
}

// RatifyBy is the ratify-or-revert deadline of a recommendation applied at t:
// one more SLA period of business hours.
func (p Policy) RatifyBy(t time.Time) time.Time { return p.Clock.Add(t, p.SLA) }

// Step is what the SLA sweep owes an open Decision at one instant.
type Step struct {
	Number      int       `json:"number"`
	SLA         time.Time `json:"sla_deadline"`
	ProvisionAt time.Time `json:"provisional_deadline"`
	Lapsed      bool      `json:"sla_lapsed"`         // page the authority and post the escalation comment
	Overdue     bool      `json:"provisional_lapsed"` // past provisional_after_hours
	Apply       bool      `json:"apply_provisionally"`
	Reason      string    `json:"reason,omitempty"` // why a lapsed Decision is not applied
}

// Evaluate decides the step for an open Decision at now. A Decision that is
// already provisional only re-pages; it is never applied twice.
func (p Policy) Evaluate(d Decision, now time.Time) Step {
	sla, prov := p.Deadlines(d)
	s := Step{Number: d.Number, SLA: sla, ProvisionAt: prov}
	if d.State != "open" || p.SLA <= 0 {
		return s
	}
	s.Lapsed = !now.Before(sla)
	if p.Provisional <= 0 || now.Before(prov) {
		return s
	}
	s.Overdue = true
	switch {
	case slices.Contains(d.Labels, workitems.LabelProvisional):
		s.Reason = "already applied provisionally"
	case d.Excluded() != "":
		s.Reason = d.Excluded()
	case !d.Reversible():
		s.Reason = "reversible: false; the lapse only re-pages"
	case len(workitems.Validate(d.Item, workitems.StageReady)) > 0:
		s.Reason = "not Decision-ready, so there is no recommendation to apply"
	default:
		s.Apply = true
	}
	return s
}

// Authority resolves who may accept d: the [decisions] authority entry for
// its parent's type, then (for a floor waiver) the floors' layer, then the
// Founder seat, which is decide's Accountable seat. It returns the key that
// matched (or "default") and its value, a seat slug or a login.
func Authority(c *config.Config, d Decision) (key, value string) {
	keys := []string{}
	if d.ParentType != "" {
		keys = append(keys, d.ParentType)
	}
	if d.Waiver() {
		keys = append(keys, FloorsLayer)
	}
	for _, k := range keys {
		if v := strings.TrimSpace(c.Decisions.Authority[k]); v != "" {
			return k, v
		}
	}
	return "default", string(verbs.Founder)
}

// Login resolves an authority value to a GitHub login: a seat's paired human,
// else [autonomy] approver; a value that is not a seat is a login. "" means
// no human is configured (the day-one path): any human may accept.
func Login(c *config.Config, value string) string {
	v := strings.TrimPrefix(strings.TrimSpace(value), "@")
	if _, ok := verbs.FindSeat(verbs.SeatID(v)); !ok {
		return v
	}
	if s, ok := c.Seats[v]; ok && s.Mode == "paired" && s.Human != "" {
		return strings.TrimPrefix(s.Human, "@")
	}
	return strings.TrimPrefix(strings.TrimSpace(c.Autonomy.Approver), "@")
}

// EscalationComment is the comment posted when the SLA lapses. It is stable
// for a given Decision, so a rerun of the sweep changes nothing.
func EscalationComment(d Decision, p Policy, authority, login string) string {
	sla, prov := p.Deadlines(d)
	var b strings.Builder
	b.WriteString(EscalationMarker + "\n### Decision SLA lapsed\n\n")
	who := "Escalation: no `[decisions] escalate_to` login is configured, so nobody is mentioned."
	if p.EscalateTo != "" {
		who = "@" + p.EscalateTo + ": escalating to you as `[decisions] escalate_to`."
	}
	fmt.Fprintf(&b, "%s This Decision was not accepted within %s business hours (due %s).\n\n", who, hours(p.SLA), local(sla, p.Clock))
	acc := "any human (no seat human or `[autonomy] approver` is configured)"
	if login != "" {
		acc = "@" + login
	}
	fmt.Fprintf(&b, "- Authority: `%s` (%s); accept with `aayu decide %d --accept <option>`.\n", authority, acc, d.Number)
	switch {
	case d.Waiver():
		b.WriteString("- This is a floor waiver: it is never applied provisionally (rule 7); the floor miss stays blocking until a human accepts it.\n")
	case d.Excluded() != "":
		fmt.Fprintf(&b, "- Not applied provisionally: %s; it waits for a human to accept it.\n", d.Excluded())
	case d.Reversible():
		fmt.Fprintf(&b, "- Reversible: at %s (%s business hours) the recommendation applies provisionally and must then be ratified or reverted.\n", local(prov, p.Clock), hours(p.Provisional))
	default:
		b.WriteString("- Not reversible: the recommendation is never applied without acceptance; the authority is paged again each day it stays open.\n")
	}
	return b.String()
}

// ProvisionalText is the Decision section written when the recommendation
// applies provisionally. RatifyBy parses the deadline back out of it.
func ProvisionalText(d Decision, p Policy, at time.Time) string {
	by := p.RatifyBy(at)
	rec := strings.TrimSpace(d.Item.Fields["recommendation"])
	rule := "V8"
	if p.Solo {
		rule = "V8, solo mode"
	}
	return fmt.Sprintf("Provisional: the recommendation (%s) applies from %s because the Decision was not accepted within %s business hours (%s). "+
		"Ratify or revert by %s (%s) with `aayu inbox` or `aayu decide %d --accept <option>`.",
		oneLine(rec), local(at, p.Clock), hours(p.Provisional), rule, by.UTC().Format(time.RFC3339), local(by, p.Clock), d.Number)
}

var ratifyRE = regexp.MustCompile(`Ratify or revert by ([0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9:]{8}Z)`)

// RatifyBy returns the ratify-or-revert deadline written in a Decision body.
func RatifyBy(body string) (time.Time, bool) {
	m := ratifyRE.FindStringSubmatch(body)
	if m == nil {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, m[1])
	return t, err == nil
}

// Accepted is the Decision section written on acceptance.
func Accepted(option, login string, at time.Time, provisional bool) string {
	s := fmt.Sprintf("%s, accepted by @%s on %s.", option, login, at.UTC().Format(time.DateOnly))
	if provisional {
		s += " This ratifies or replaces the provisional application."
	}
	return s
}

// MatchOption reports whether option names one of the Decision's options: a
// case-insensitive whole-word match in the Options field.
func MatchOption(options, option string) bool {
	return workitems.MentionsOption(options, option)
}

// Options are a Decision's numbered options, each under the Decision's own
// number, and the index of the recommended one (-1 when none is): the option
// marked "(recommended)", else the one its Recommendation names by number
// ("Option 2", "2."), else by name (the longest name it names, so "Don't
// waive" wins over "Waive"). The inbox, the approval batch, and decide's
// output all show options this way, and decide --accept takes the number.
func Options(it workitems.Item) ([]workitems.Option, int) {
	os, _ := workitems.ParseOptions(it.Fields["options"])
	rec := slices.IndexFunc(os, func(o workitems.Option) bool { return o.Recommended })
	r := strings.TrimSpace(it.Fields["recommendation"])
	if m := workitems.OptionRef.FindStringSubmatch(r); rec < 0 && m != nil {
		if i, _ := strconv.Atoi(m[1] + m[2]); i >= 1 && i <= len(os) {
			rec = i - 1
		}
	}
	if rec < 0 {
		rec = workitems.NamedOption(os, r) // -1 when it names none, or more than one
	}
	for i := range os { // one recommendation, even where a draft marks two
		os[i].Recommended = i == rec
	}
	return os, rec
}

// Pick resolves what a person typed to accept, an option's number or its
// name, to that option of it; ok is false when it names none.
func Pick(it workitems.Item, typed string) (workitems.Option, bool) {
	os, _ := Options(it)
	typed = strings.TrimSpace(typed)
	for _, o := range os {
		if typed == strconv.Itoa(o.N) || strings.EqualFold(typed, o.Name) {
			return o, true
		}
	}
	return workitems.Option{}, false
}

// WaiverItem is the body of a floor-waiver Decision for floor on story n.
func WaiverItem(floor string, n int, why string) (title string, fields map[string]string) {
	title = fmt.Sprintf("Waive the %s floor on #%d", floor, n)
	q := fmt.Sprintf("Should #%d ship with its `%s` floor miss waived?", n, floor)
	if w := strings.TrimSpace(why); w != "" {
		q += "\n\n" + w
	}
	return title, map[string]string{
		workitems.FieldSummary: q,
		"options": fmt.Sprintf("1. **Waive.** #%d ships with the `%s` floor recorded as waived by this Decision.\n"+
			"2. **Don't waive.** No waiver: the work is fixed until the floor holds, and #%d stays blocked until then.", n, floor, n),
		"evaluation": "A waiver is the only way past a floor miss (rule 7). The floor's report is in the story's proof pack.",
		// Never the waiver by default: recommending it is the floor's owner seat's call, with evidence.
		"recommendation":          "Option 2, don't waive, until the seat that owns the floor recommends the waiver with evidence.",
		workitems.FieldReversible: "false",
		workitems.FieldLinks:      fmt.Sprintf("waives #%d", n),
	}
}

func hours(d time.Duration) string { return fmt.Sprintf("%g", d.Hours()) }

func local(t time.Time, c config.Clock) string { return t.In(c.Loc).Format("Mon 2 Jan 2006 15:04 MST") }

func oneLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 200 {
		s = s[:200] + "…"
	}
	if s == "" {
		s = "none stated"
	}
	return s
}
