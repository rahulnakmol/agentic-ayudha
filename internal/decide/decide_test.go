package decide

import (
	"strings"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

func policy(t *testing.T, escalate string) Policy {
	t.Helper()
	c := config.Config{Decisions: config.Decisions{SLAHours: 24, ProvisionalAfter: 48, EscalateTo: escalate}, Time: config.Time{TZ: "Europe/London", BusinessHours: "Mon-Fri 09:00-18:00"}}
	p, err := NewPolicy(&c)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func decision(reversible string, labels ...string) Decision {
	fields := map[string]string{workitems.FieldSummary: "Q", "options": "A: one. B: two.", "evaluation": "E", "recommendation": "A", workitems.FieldReversible: reversible}
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
	if !strings.HasSuffix(Accepted("A", "o", at48, false), "on 2026-10-19.") {
		t.Error(Accepted("A", "o", at48, false))
	}
}
