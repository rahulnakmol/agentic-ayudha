package decide

import (
	"strings"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// policy is a two-human team's: the general 48-hour window.
func policy(t *testing.T, escalate string) Policy {
	t.Helper()
	c := config.Config{Decisions: config.Decisions{SLAHours: 24, ProvisionalAfter: 48, SoloProvisional: 24, EscalateTo: escalate}, Time: config.Time{TZ: "Europe/London", BusinessHours: "Mon-Fri 09:00-18:00"},
		Seats: map[string]config.Seat{"code-architect": {Mode: "paired", Human: "arch"}}, Autonomy: config.Autonomy{Approver: "founder-a"}}
	p, err := NewPolicy(&c)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func decision(reversible string, labels ...string) Decision {
	fields := map[string]string{workitems.FieldSummary: "Q", "options": "1. **A.** one.\n2. **B.** two.", "evaluation": "E", "recommendation": "A", workitems.FieldReversible: reversible}
	return Decision{Number: 7, State: "open", Labels: append([]string{"type:decision"}, labels...), Created: time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC),
		Item: workitems.Item{Number: 7, Title: "Choose", Type: "decision", Fields: fields, Labels: append([]string{"type:decision"}, labels...)}}
}

func TestAuthorityAndLogin(t *testing.T) {
	c := &config.Config{
		Decisions: config.Decisions{Authority: map[string]string{"story": "code-architect", "L04": "@qa-lead"}},
		Seats:     map[string]config.Seat{"code-architect": {Mode: "paired", Human: "@arch"}},
		Autonomy:  config.Autonomy{Approver: "founder-a"},
	}
	d := decision("true")
	d.ParentType = "story"
	if k, v := Authority(c, d); k != "story" || Login(c, v) != "arch" {
		t.Errorf("story parent: %s = %s (%s)", k, v, Login(c, v))
	}
	w := decision("false", LabelWaiver)
	w.ParentType = "bug"
	if k, v := Authority(c, w); k != "L04" || Login(c, v) != "qa-lead" {
		t.Errorf("waiver on a bug: %s = %s", k, v)
	}
	if k, v := Authority(c, decision("true")); k != "default" || Login(c, v) != "founder-a" {
		t.Errorf("no parent: %s = %s, login %q (want the founder seat, then [autonomy] approver)", k, v, Login(c, v))
	}
	if got := Login(&config.Config{}, "founder"); got != "" {
		t.Errorf("solo founder with no approver: %q, want any human", got)
	}
}

func TestEvaluateAndTexts(t *testing.T) {
	p := policy(t, "")
	at47 := time.Date(2026, 10, 19, 11, 59, 0, 0, time.UTC) // created Mon 10:00 London: 48 business hours lapse Mon 19 Oct 13:00 London
	at48 := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	if s := p.Evaluate(decision("true"), at47); !s.Lapsed || s.Overdue || s.Apply {
		t.Errorf("47:59: %+v", s)
	}
	if s := p.Evaluate(decision("true"), at48); !s.Apply {
		t.Errorf("48:00 reversible: %+v", s)
	}
	notReady := decision("true")
	delete(notReady.Item.Fields, "recommendation")
	for name, d := range map[string]Decision{"irreversible": decision("false"), "waiver": decision("true", LabelWaiver), "not ready": notReady,
		"already provisional": decision("true", workitems.LabelProvisional)} {
		if s := p.Evaluate(d, at48); s.Apply || s.Reason == "" {
			t.Errorf("%s at 48:00: %+v", name, s)
		}
	}
	closed := decision("true")
	closed.State = "closed"
	if s := p.Evaluate(closed, at48); s.Lapsed {
		t.Errorf("closed: %+v", s)
	}

	c := EscalationComment(decision("false"), p, "default = founder", "")
	if !strings.Contains(c, "no `[decisions] escalate_to` login is configured") || !strings.Contains(c, "any human") || !strings.Contains(c, "Not reversible") {
		t.Errorf("escalation without founder B:\n%s", c)
	}
	if c := EscalationComment(decision("true", LabelWaiver), policy(t, "founder-b"), "L04 = qa", "qa"); !strings.Contains(c, "@founder-b") || !strings.Contains(c, "never applied provisionally") || !strings.Contains(c, "@qa") {
		t.Errorf("waiver escalation:\n%s", c)
	}
	d := decision("true")
	d.Item.Fields["recommendation"] = strings.Repeat("word ", 60)
	text := ProvisionalText(d, p, at48)
	if by, ok := RatifyBy(text); !ok || by != p.RatifyBy(at48).UTC() || !strings.Contains(text, "…") {
		t.Errorf("provisional text: %s", text)
	}
	if _, ok := RatifyBy("no deadline here"); ok {
		t.Error("RatifyBy found a deadline in plain text")
	}
	if !MatchOption("A: in-house. B: vendor.", "b") || MatchOption("A: in-house. B: vendor.", "hou") || MatchOption("A", " ") {
		t.Error("MatchOption")
	}
	title, fields := WaiverItem("coverage", 4, "Generated code.")
	if title != "Waive the coverage floor on #4" || !strings.Contains(fields[workitems.FieldSummary], "Generated code.") || fields[workitems.FieldReversible] != "false" {
		t.Errorf("waiver item: %s %v", title, fields)
	}
	// A generated waiver offers both choices, is Decision-ready, and never
	// recommends the waiver by default; accepting "waive" names option 1.
	it := workitems.Item{Type: "decision", Title: title, Fields: fields}
	opts, rec := Options(it)
	if ps := workitems.Validate(it, workitems.StageReady); len(ps) != 0 || len(opts) != 2 ||
		opts[0] != (workitems.Option{N: 1, Name: "Waive", Text: "#4 ships with the `coverage` floor recorded as waived by this Decision."}) ||
		opts[1].N != 2 || opts[1].Name != "Don't waive" || !strings.Contains(opts[1].Text, "#4 stays blocked") || rec != 1 || !opts[1].Recommended {
		t.Errorf("waiver options: %v %+v rec %d", ps, opts, rec)
	}
	for typed, want := range map[string]int{"waive": 1, "1": 1, "Don't waive": 2, "2": 2, "3": 0, "fix": 0} {
		if o, ok := Pick(it, typed); o.N != want || ok != (want != 0) {
			t.Errorf("Pick(%q) = %+v %v, want option %d", typed, o, ok, want)
		}
	}
	if !strings.HasSuffix(Accepted("A", "o", at48, false), "on 2026-10-19.") {
		t.Error(Accepted("A", "o", at48, false))
	}
}

// TestSoloProvisional (Story #432, criterion 1): with at most one human the
// window is the shorter of the two keys, never under 12 business hours; a
// second human restores the general window, and 0 still turns V8 off. Created
// Monday 12 October 10:00 London: 12 business hours end Tuesday 13:00, 20 end
// Wednesday 12:00, 24 end Wednesday 16:00, and 48 end Monday 19 October 13:00.
func TestSoloProvisional(t *testing.T) {
	london := func(day, hour int) time.Time { return time.Date(2026, 10, day, hour-1, 0, 0, 0, time.UTC) } // BST
	tests := []struct {
		name          string
		general, solo int
		seats         map[string]config.Seat
		approver      string
		want          time.Time // zero: never applies
		solomode      bool
	}{
		{"one human, as approver and seat", 48, 24, map[string]config.Seat{"code-architect": {Mode: "paired", Human: "@Founder-A"}}, "founder-a", london(14, 16), true},
		{"no human configured", 48, 24, nil, "", london(14, 16), true},
		{"two humans", 48, 24, map[string]config.Seat{"code-architect": {Mode: "paired", Human: "arch"}}, "founder-a", london(19, 13), false},
		{"a solo seat's login is not a human", 48, 24, map[string]config.Seat{"code-architect": {Mode: "solo", Human: "arch"}}, "founder-a", london(14, 16), true},
		{"the general window is shorter", 20, 24, nil, "", london(14, 12), false},
		{"below the minimum, built without Load", 48, 5, nil, "", london(13, 13), true},
		{"V8 off", 0, 24, nil, "", time.Time{}, false},
	}
	for _, tt := range tests {
		c := config.Config{Decisions: config.Decisions{SLAHours: 24, ProvisionalAfter: tt.general, SoloProvisional: tt.solo}, Seats: tt.seats,
			Autonomy: config.Autonomy{Approver: tt.approver}, Time: config.Time{TZ: "Europe/London", BusinessHours: "Mon-Fri 09:00-18:00"}}
		p, err := NewPolicy(&c)
		if err != nil {
			t.Fatal(err)
		}
		d := decision("true")
		d.Created = london(12, 10)
		if p.Solo != tt.solomode {
			t.Errorf("%s: solo = %v", tt.name, p.Solo)
		}
		if tt.want.IsZero() {
			if s := p.Evaluate(d, london(30, 17)); s.Apply || s.Overdue {
				t.Errorf("%s: %+v", tt.name, s)
			}
			continue
		}
		if s := p.Evaluate(d, tt.want.Add(-time.Minute)); s.Apply {
			t.Errorf("%s: applied a minute early: %+v", tt.name, s)
		}
		if s := p.Evaluate(d, tt.want); !s.Apply {
			t.Errorf("%s: not applied at %s: %+v", tt.name, tt.want, s)
		}
	}
	solo, _ := NewPolicy(&config.Config{Decisions: config.Decisions{SLAHours: 24, ProvisionalAfter: 48, SoloProvisional: 24}, Time: config.Time{TZ: "Europe/London", BusinessHours: "Mon-Fri 09:00-18:00"}})
	if text := ProvisionalText(decision("true"), solo, london(14, 16)); !strings.Contains(text, "within 24 business hours (V8, solo mode)") || !strings.Contains(text, "`aayu inbox`") {
		t.Errorf("solo provisional text: %s", text)
	}
}

// TestProvisionalNever (Story #432, criterion 2): a waiver of any kind (with
// or without its label), a seat change, and a human-only step of the trust
// model are never applied, in solo mode or not, whatever their body says; an
// ordinary two-way door still is.
func TestProvisionalNever(t *testing.T) {
	at := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC) // far past either window
	with := func(title, links string, labels ...string) Decision {
		d := decision("true", labels...)
		d.Title, d.Item.Fields[workitems.FieldLinks] = title, links
		return d
	}
	never := map[string]struct {
		d    Decision
		want string
	}{
		"waiver label":          {with("Ship #6 as it is", "", LabelWaiver), "a waiver"},
		"waives link":           {with("Ship #6 as it is", "waives #6"), "a waiver"},
		"eval-bar waiver title": {with("Waive the eval-bar on #9", ""), "a waiver"},
		"seat change":           {with("Pair a human on the code-architect seat", ""), "a seat change"},
		"seat by name":          {with("Hand triage to the Quality Architect", ""), "a seat change"},
		"autonomy":              {with("Allow L04 in [autonomy] auto", ""), "human-only step"},
		"protected path":        {with("Edit .github/workflows/ci.yml to skip lint", ""), "human-only step"},
		"approve a PR":          {with("Approve the PR for #12 without review", ""), "human-only step"},
		"publish":               {with("Publish v0.3.0 tonight", ""), "human-only step"},
		"ratchet":               {with("Hold the coverage ratchet for a week", ""), "human-only step"},
		"one-way door":          {decision("false"), "reversible: false"},
	}
	for _, p := range []Policy{policy(t, ""), func() Policy { p := policy(t, ""); p.Provisional, p.Solo = 24*time.Hour, true; return p }()} {
		for name, tt := range never {
			if s := p.Evaluate(tt.d, at); s.Apply || !strings.Contains(s.Reason, tt.want) {
				t.Errorf("%s (solo %v): %+v, want refused as %q", name, p.Solo, s, tt.want)
			}
		}
		if s := p.Evaluate(with("Choose the rules engine", "constrains #6"), at); !s.Apply {
			t.Errorf("ordinary two-way door (solo %v): %+v", p.Solo, s)
		}
	}
	if c := EscalationComment(with("Pair a human on the code-architect seat", ""), policy(t, ""), "default = founder", ""); !strings.Contains(c, "Not applied provisionally: a seat change") {
		t.Errorf("seat-change escalation:\n%s", c)
	}
}
