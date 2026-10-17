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
// lands (V3), whoever accepts.
var Waivable = []string{"coverage", "sast", "secrets", "deps", "a11y", "keyboard", "reduced-motion", "text-scale", "targets", "perf", "dast", "api", "screen-reader", "mobile", "error-budget"}

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

// Policy is [decisions] with the [time] clock.
type Policy struct {
	Clock       config.Clock
	SLA         time.Duration // business hours before the page and escalation
	Provisional time.Duration // business hours before a reversible recommendation applies
	EscalateTo  string        // login mentioned on escalation (founder B); empty when unset
}

// NewPolicy reads [decisions] and [time].
func NewPolicy(c *config.Config) (Policy, error) {
	clock, err := c.Clock()
	if err != nil {
		return Policy{}, err
	}
	return Policy{Clock: clock, SLA: time.Duration(c.Decisions.SLAHours) * time.Hour,
		Provisional: time.Duration(c.Decisions.ProvisionalAfter) * time.Hour,
		EscalateTo:  strings.TrimPrefix(strings.TrimSpace(c.Decisions.EscalateTo), "@")}, nil
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
	case d.Waiver():
		s.Reason = "a floor waiver is never applied provisionally"
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
	return fmt.Sprintf("Provisional: the recommendation (%s) applies from %s because the Decision was not accepted within %s business hours (V8). "+
		"Ratify or revert by %s (%s) with `aayu decide %d --accept <option>`.",
		oneLine(rec), local(at, p.Clock), hours(p.Provisional), by.UTC().Format(time.RFC3339), local(by, p.Clock), d.Number)
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
	o := strings.TrimSpace(option)
	if o == "" {
		return false
	}
	re, err := regexp.Compile(`(?i)(^|[^\pL\pN])` + regexp.QuoteMeta(o) + `($|[^\pL\pN])`)
	return err == nil && re.MatchString(options)
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
		"options": fmt.Sprintf("waive: #%d ships with the `%s` floor recorded as waived by this Decision.\n"+
			"fix: no waiver; #%d fixes the miss before it ships.", n, floor, n),
		"evaluation":              "A waiver is the only way past a floor miss (rule 7). The floor's report is in the story's proof pack.",
		"recommendation":          "fix",
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
