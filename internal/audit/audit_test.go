package audit

import (
	"fmt"

	"strings"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/autonomy"
	"github.com/rahulnakmol/agentic-ayudha/internal/inspect"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
)

var t0 = time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)

func at(h int) time.Time { return t0.Add(time.Duration(h) * time.Hour) }

// Each bypass that the hand-run reconciliations explained in prose has one
// rule here, and each rule has a near miss that must stay unexplained.
func TestClassifyBypasses(t *testing.T) {
	d := &Data{Dev: "dev", Ledger: "ledger", GateCheck: "protected-paths",
		LedgerLog: []ledger.Entry{{SHA: "led1", Subject: "specs: Ready snapshot of #7"}},
		PRs:       []PR{{Number: 5, Base: "dev", MergeSHA: "m5"}},
		Subjects:  map[string]string{"r1": "Rolling snapshot: 1", "r2": "Rolling snapshot: 2", "r3": "Rolling snapshot: 3", "x1": "feat: not a promotion"},
		OnMain:    map[string]bool{"v1": true},
		Runs: []Run{{ID: 1, Workflow: ".github/workflows/ci.yml", Event: "workflow_dispatch", HeadSHA: "r1", Conclusion: "success", CreatedAt: at(1)},
			{ID: 4, Workflow: ".github/workflows/ci.yml", Event: "workflow_dispatch", HeadSHA: "r3", Conclusion: "success", CreatedAt: at(5)},
			{ID: 2, Workflow: ".github/workflows/release.yml", Branch: "v1.0.0", Conclusion: "success"},
			{ID: 3, Workflow: ".github/workflows/release.yml", Branch: "v9.9.9", Conclusion: "success"}},
		Prior:     []PriorReport{{Path: "audit/x.md", Suites: []int64{99}}},
		Incidents: []PriorReport{{Path: "audit/y/incident-z.md", Suites: []int64{77}}}}
	pp := []Rule{{Ruleset: "ppr", Type: "required_status_checks", Details: `Required status check "protected-paths" is failing.`}}
	for _, tc := range []struct {
		s         RuleSuite
		class     string
		accounted bool
		note      string
	}{
		{RuleSuite{Ref: "refs/heads/ledger", Actor: "founder", After: "gone"}, ClassLedger, false, "not in the ledger's history"},
		{RuleSuite{Ref: "refs/heads/rolling", Actor: "founder", After: "x1"}, ClassPromotion, false, "is not a \"Rolling snapshot:\" merge"},
		{RuleSuite{Ref: "refs/heads/rolling", Actor: "founder", After: "r2"}, ClassPromotion, false, "no successful gate run"},
		{RuleSuite{Ref: "refs/heads/main", Actor: "founder", After: "r1"}, ClassPromotion, false, "not a \"Release: v\" merge"},
		{RuleSuite{Ref: "refs/heads/rolling", Actor: "founder", After: "r1", PushedAt: at(2)}, ClassPromotion, true, "after gate run 1"},
		{RuleSuite{Ref: "refs/heads/rolling", Actor: "founder", After: "r3", PushedAt: at(2)}, ClassPromotion, false, "started before the push"},
		{RuleSuite{Ref: "refs/tags/v1.0.0", Actor: "founder", After: "v1"}, ClassTag, true, "release tag v1.0.0 on main"},
		{RuleSuite{Ref: "refs/tags/v9.9.9", Actor: "founder", After: "0000000000000000000000000000000000000000"}, ClassTag, false, "deletion of released tag"},
		{RuleSuite{Ref: "refs/tags/v2.0.0", Actor: "founder", After: "v2"}, ClassTag, false, "no release run refused it"},
		{RuleSuite{Ref: "refs/heads/dev", Actor: "founder", After: "direct"}, ClassOther, false, "a direct push"},
		{RuleSuite{ID: 99, Ref: "refs/heads/dev", Actor: "founder", After: "direct"}, ClassOther, true, "accounted for by audit/x.md"},
		{RuleSuite{Ref: "refs/heads/dev", Actor: "founder", After: "m5", Failed: pp}, ClassSolo, false, "no prior \"Solo-mode bypass:\" comment"},
		{RuleSuite{ID: 78, Ref: "refs/heads/dev", Actor: "founder", After: "m5", Failed: pp}, ClassSolo, false, "no prior \"Solo-mode bypass:\" comment"},
		{RuleSuite{ID: 77, Ref: "refs/heads/dev", Actor: "founder", After: "m5", Failed: pp}, ClassSoloLate, true, "record came after the merge"},
		{RuleSuite{Ref: "refs/heads/dev", Actor: "founder", After: "m5", Failed: append(pp, Rule{Ruleset: "dev", Type: "pull_request"})}, ClassOther, false, "pull_request (dev)"},
	} {
		b := d.classify(tc.s)
		if b.Class != tc.class || b.Accounted != tc.accounted || !strings.Contains(b.Note, tc.note) {
			t.Errorf("%s %s: got %s accounted=%t %q; want %s accounted=%t containing %q", tc.s.Ref, tc.s.After, b.Class, b.Accounted, b.Note, tc.class, tc.accounted, tc.note)
		}
	}
}

// Retro findings: over the checkpoint budget, over the cost budget, merged
// without proof after the gate, and a skill's correction rate; a story within
// budget and pre-gate merges file nothing.
func TestRetroFindings(t *testing.T) {
	in := Inputs{Repo: "o/r", Since: at(0), Until: at(24)}
	d := &Data{Dev: "dev", GateSince: at(5),
		Items: []Item{
			{Number: 1, Title: "Big", Labels: []string{"type:story", "status:merged", "effort:S"}},
			{Number: 2, Title: "Small", Labels: []string{"type:story", "status:merged", "effort:M"}},
			{Number: 3, Title: "Early", Labels: []string{"type:story", "status:ready", "effort:S"}},
			{Number: 4, Title: "Decide", Labels: []string{"type:decision"}, CreatedAt: at(1), ClosedAt: at(3)},
		},
		PRs: []PR{{Number: 11, Base: "dev", Body: "Refs #1", MergedAt: at(10)}, {Number: 12, Base: "dev", Head: "story/2-small", MergedAt: at(11)},
			{Number: 13, Base: "dev", Body: "Closes #3", MergedAt: at(2)}},
		Packs: []Pack{{Path: "p1", Story: 1, Kind: proof.Advisory, Checkpoint: "3/3", CreatedAt: at(9).Format(time.RFC3339)},
			{Path: "p2", Story: 2, Kind: proof.Authoritative, Checkpoint: "2/2", Covers: true, CreatedAt: at(10).Format(time.RFC3339)}},
		Records:     []Record{{Record: actions.Record{Time: at(9), Action: actions.Action{Story: "#1"}, CostUSD: 30.5}}},
		AgentPRs:    []AgentPR{{Number: 11, MergedAt: at(10), Skills: []string{"grill"}}, {Number: 12, MergedAt: at(11), Skills: []string{"grill", "ready"}}},
		Corrections: []autonomy.Correction{{PR: 11, Kind: autonomy.KindHumanCommit, At: at(10)}},
	}
	r := BuildRetro(in, d, 25)
	var titles []string
	for _, f := range r.Findings {
		titles = append(titles, f.Title)
	}
	got := strings.Join(titles, "\n")
	for _, want := range []string{
		"#1 took 3 checkpoints, over the 2 of Effort S",
		"#1 cost 30.50 USD, over the 25.00 USD story budget",
		"#1 merged without proof after the merge gate existed",
		"correction rate 1/2 above 20 %",
		"skill `grill` correction rate 1/2 above 20 %",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("findings lack %q:\n%s", want, got)
		}
	}
	for _, not := range []string{"#2 ", "#3 ", "skill `ready`"} {
		if strings.Contains(got, not) {
			t.Errorf("findings name %q:\n%s", not, got)
		}
	}
	if md := string(r.Markdown()); !strings.Contains(md, "| #1 Big | #11 | S | 2 | 3 | 1 | 30.50 | merged without proof (post-gate) |") || !strings.Contains(md, "| `grill` | 2 | 1 |") {
		t.Errorf("retro report:\n%s", md)
	}
	if body := r.FeedbackBody(r.Findings[0], "retro/x.md"); !strings.Contains(body, "### Context") || !strings.Contains(body, "`retro/x.md`") {
		t.Errorf("feedback body: %s", body)
	}
	a := Build(in, d)
	if len(a.Decisions) != 1 || a.Decisions[0].Open || a.Decisions[0].Hours != 2 {
		t.Errorf("closed Decision: %+v", a.Decisions)
	}
}

// A labels record without a payload matches any label; a create result may be
// a bare number.
func TestLoggedMatching(t *testing.T) {
	d := &Data{Records: []Record{
		{Segment: "s", Record: actions.Record{Time: at(1), Status: actions.StatusDone, Action: actions.Action{Request: &actions.Request{Method: "POST", Path: "/repos/o/r/issues/7/labels"}}}},
		{Segment: "s", Record: actions.Record{Time: at(1), Status: actions.StatusDone, Result: float64(8), Action: actions.Action{Request: &actions.Request{Method: "POST", Path: "/repos/o/r/issues"}}}},
		{Segment: "s", Record: actions.Record{Time: at(1), Status: actions.StatusDone, Action: actions.Action{Target: "o/r#9", Request: &actions.Request{Method: "PUT", Path: "/repos/o/r/issues/9/assignees"}}}},
	}}
	in := Inputs{Repo: "o/r"}
	for _, tc := range []struct {
		kind  string
		n     int
		label string
		at    time.Time
		want  string
	}{
		{"labeled", 7, "any", at(1), "s"},
		{"opened", 8, "", at(1), "s"},
		{"assigned", 9, "", at(1), "s"},
		{"labeled", 7, "any", at(2), ""}, // an hour away: outside the tolerance
		{"unlabeled", 7, "any", at(1), ""},
	} {
		if got := d.logged(in, tc.kind, tc.n, tc.label, 0, tc.at, map[string]bool{}); got != tc.want {
			t.Errorf("%s #%d at %s: %q, want %q", tc.kind, tc.n, tc.at, got, tc.want)
		}
	}
	if !payloadHas(map[string]any{"labels": []string{"a"}}, "labels", "a") || payloadHas(map[string]any{"labels": "a"}, "labels", "a") {
		t.Error("payloadHas")
	}
}

// A logged dispatch of one workflow does not account for a direct dispatch of another.
func TestLoggedDispatchNamesItsWorkflow(t *testing.T) {
	in := Inputs{Repo: "o/r", Since: at(0), Until: at(24)}
	d := &Data{Records: []Record{{Segment: "s", Record: actions.Record{Time: at(1), Status: actions.StatusDone,
		Action: actions.Action{Request: &actions.Request{Method: "POST", Path: "/repos/o/r/actions/workflows/aayudha-token.yml/dispatches"}}}}},
		Runs: []Run{{ID: 2, Workflow: ".github/workflows/ci.yml", Event: "workflow_dispatch", Actor: "founder", CreatedAt: at(1)},
			{ID: 3, Workflow: ".github/workflows/aayudha-token.yml", Event: "workflow_dispatch", Actor: "founder", CreatedAt: at(1).Add(time.Minute)}}}
	ws := d.writes(in)
	if len(ws) != 2 || ws[0].Logged != "" || ws[1].Logged != "s" {
		t.Fatalf("dispatches: %+v", ws)
	}
}

// One logged comment accounts for one comment: a direct `gh` comment posted
// minutes after an `aayu` one on the same issue is still unlogged, and a
// comment record that names its comment ID matches only that comment.
func TestLoggedCommentIsNotReused(t *testing.T) {
	post := func(at time.Time, id float64) Record {
		var res any
		if id != 0 {
			res = id
		}
		return Record{Segment: "s", Record: actions.Record{Time: at, Status: actions.StatusDone, Result: res,
			Action: actions.Action{Request: &actions.Request{Method: "POST", Path: "/repos/o/r/issues/7/comments"}}}}
	}
	in := Inputs{Repo: "o/r", Since: at(0), Until: at(24)}
	comment := func(id int64, h time.Duration) Event {
		return Event{Kind: "commented", Actor: "founder", At: at(1).Add(h * time.Minute), ID: id}
	}
	d := &Data{Records: []Record{post(at(1), 0)}, Items: []Item{{Number: 7, Author: "founder", CreatedAt: at(-48),
		Events: []Event{comment(70, 0), comment(71, 3)}}}}
	ws := d.writes(in)
	if len(ws) != 2 || ws[0].Logged != "s" || ws[1].Logged != "" {
		t.Fatalf("an aayu comment then a direct one: %+v", ws)
	}
	d.Records = []Record{post(at(1).Add(3*time.Minute), 71)}
	ws = d.writes(in)
	if ws[0].Logged != "" || ws[1].Logged != "s" {
		t.Fatalf("a record naming comment 71 matched another: %+v", ws)
	}
}

// A ship whose never-layer plan had a solo-mode plan-review bypass is listed
// apart, with its own count; a report without one has no such section, so
// earlier reports replay unchanged.
func TestAuditPlanReviewBypass(t *testing.T) {
	in := Inputs{Repo: "o/r", Since: at(0), Until: at(24)}
	ship := Ship{Path: "proof/5/abc/authoritative/ship.json", Story: 5, PR: 6, Merge: "abc", Pass: "solo-bypass", Login: "founder", ShippedAt: at(3)}
	md := string(Build(in, &Data{Dev: "dev", Ships: []Ship{ship}}).Markdown())
	if strings.Contains(md, "plan-review bypass") {
		t.Errorf("report without a plan-review bypass mentions one:\n%s", md)
	}
	ship.PlanBypass = &PlanBypass{Layers: []string{"L04", "L07"}, By: "founder", Comment: "https://example.test/c/9"}
	r := Build(in, &Data{Dev: "dev", Ships: []Ship{ship}})
	md = string(r.Markdown())
	for _, want := range []string{
		"- Solo-mode plan-review bypasses (`plan --solo-bypass`, no plan-time review): 1",
		"| #5 | #6 | L04, L07 | founder | `proof/5/abc/authoritative/ship.json` |",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("report lacks %q:\n%s", want, md)
		}
	}
	if c := r.Counts(); c.PlanBypasses != 1 || c.SoloShips != 1 {
		t.Errorf("counts: %+v", c)
	}
}

// Decision #469: the report counts the agent merges in the window by their
// review's rung, and leaves out merges outside it.
func TestAuditReviewRungs(t *testing.T) {
	in := Inputs{Repo: "o/r", Since: at(5), Until: at(24)}
	d := &Data{Dev: "dev", AgentPRs: []AgentPR{{Number: 1, MergedAt: at(6), Rung: "A"}, {Number: 2, MergedAt: at(7), Rung: "C"}, {Number: 3, MergedAt: at(8), Rung: "C"},
		{Number: 4, MergedAt: at(9)}, {Number: 5, MergedAt: at(10), Rung: "human"}, {Number: 6, MergedAt: at(2), Rung: "A"}, {Number: 7, MergedAt: at(24), Rung: "B"}}}
	md := string(Build(in, d).Markdown())
	if want := "- Review rungs of agent merges (Decision #469): different-family (A) 1; paid different-family (B) 0; a person's 1; same-family (C) 2; none recorded 1"; !strings.Contains(md, want) {
		t.Errorf("report lacks %q:\n%s", want, md)
	}
	// With no rung on record (a snapshot from before the ladder), the line is
	// left out, so a replayed report stays byte for byte the same.
	for i := range d.AgentPRs {
		d.AgentPRs[i].Rung = ""
	}
	if md = string(Build(in, d).Markdown()); strings.Contains(md, "Review rungs") {
		t.Errorf("a report with no rung on record shows the rung line:\n%s", md)
	}
}

// Flow metrics (P1, S-7.6e) on hand-computed, asymmetric data: #1 waits 24 h
// for a person after its gate went green and was touched three times; #2 has
// no build record, ships an hour after green, and escapes as Bug #9. Bug #8
// came before #2 merged, Bug #7 is fixed by #1's own PR, a dismissal of the
// same finding ID on another PR does not count, and a PR into main is not in
// the change mix. The numbers survive a snapshot round trip (--replay).
func TestRetroFlow(t *testing.T) {
	in := Inputs{Repo: "o/r", Since: at(0), Until: at(48)}
	ready := func(h int) Event { return Event{Kind: "labeled", Label: "status:ready", Actor: "founder", At: at(h)} }
	d := &Data{Dev: "dev", Days: 14, Bots: []string{"aayu-bot"},
		Items: []Item{
			{Number: 1, Title: "Waits", Labels: []string{"type:story", "status:merged", "effort:S"}, Events: []Event{ready(0),
				{Kind: "commented", Actor: "founder", At: at(3)}, ready(4),
				{Kind: "commented", Actor: "aayu-bot", At: at(5)}, {Kind: "commented", Actor: "founder", Aayu: true, At: at(5)}}},
			{Number: 2, Title: "Escapes", Labels: []string{"type:story", "status:merged", "effort:XS"}, Events: []Event{ready(1),
				{Kind: "commented", Actor: "founder", At: at(30)}}},
			{Number: 11, PR: true, Events: []Event{{Kind: "reviewed", State: "changes_requested", Actor: "founder", At: at(6)},
				{Kind: "commented", Actor: "github-actions[bot]", At: at(6)}, {Kind: "reviewed", State: "approved", Actor: "founder", At: at(28)}},
				Inspect: &Review{Cycles: 2, Posted: []string{"a", "b", "c"}, Open: []string{"c"}}},
			{Number: 12, PR: true},
		},
		PRs: []PR{{Number: 11, Title: "feat(x): waits", Base: "dev", Body: "Refs #1\nFixes #7", MergedAt: at(29), HeadSHA: "h11"},
			{Number: 12, Title: "fix: escapes", Base: "dev", Head: "story/2-escapes", MergedAt: at(11), HeadSHA: "h12"},
			{Number: 13, Title: "docs: notes", Base: "dev", MergedAt: at(12)}, {Number: 14, Title: "Update things", Base: "dev", MergedAt: at(13)},
			{Number: 15, Title: "feat: promote", Base: "main", MergedAt: at(14)}},
		GateRuns: []Run{{ID: 1, HeadSHA: "h11", UpdatedAt: at(7)}, {ID: 2, HeadSHA: "h11", UpdatedAt: at(5)}, {ID: 3, HeadSHA: "old12", UpdatedAt: at(8)},
			{ID: 4, HeadSHA: "h12", UpdatedAt: at(10)}, {ID: 5, HeadSHA: "h12", UpdatedAt: at(12)}},
		Builds:    map[int]BuildLog{1: {First: at(2)}},
		Dismissed: []inspect.Dismissal{{ID: "b", PR: 11}, {ID: "a", PR: 99}},
		Bugs: []Bug{{Number: 7, CreatedAt: at(30), Links: []int{11}}, {Number: 8, CreatedAt: at(5), Links: []int{2}},
			{Number: 9, CreatedAt: at(20), Links: []int{12}}, {Number: 10, CreatedAt: at(60), Links: []int{1}}},
	}
	check := func(r *Retro) {
		t.Helper()
		f := r.Flow
		want := map[string][3]float64{"Queue: Ready to first checkpoint": {1, 2, 2}, "Build: first checkpoint to gate green": {1, 3, 3},
			"Human wait: gate green to ship": {2, 1, 24}, "Total: Ready to ship": {2, 10, 29}}
		for _, s := range f.Stages {
			if w := want[s.Name]; float64(s.Stories) != w[0] || s.P50 != w[1] || s.P90 != w[2] {
				t.Errorf("%s: %d stories, P50 %g, P90 %g; want %v", s.Name, s.Stories, s.P50, s.P90, w)
			}
		}
		if len(f.Stories) != 2 || f.Stories[0].Touches != 3 || f.Stories[1].Touches != 0 {
			t.Errorf("touches: %+v", f.Stories)
		}
		if f.Escaped != 1 || fmt.Sprint(f.Stories[1].Escapes) != "[9]" || len(f.Stories[0].Escapes) != 0 {
			t.Errorf("escapes: %d %+v", f.Escaped, f.Stories)
		}
		if f.Inspect != (InspectFlow{PRs: 1, WithPosts: 1, Cycles: 2, Posted: 3, ActedOn: 1, Dismissed: 1, Open: 1}) {
			t.Errorf("inspect: %+v", f.Inspect)
		}
		if f.Merged != 4 || fmt.Sprint(f.Mix) != "[{docs 1} {feat 1} {fix 1} {other 1}]" {
			t.Errorf("change mix of %d: %v", f.Merged, f.Mix)
		}
		md := string(r.Markdown())
		for _, w := range []string{"| Human wait: gate green to ship | 2 of 2 | 1.0 h | 24.0 h |", "| #1 | #11 | 2.0 | 3.0 | 24.0 | 29.0 | 3 | — |",
			"| #2 | #12 | n/a | n/a | 1.0 | 10.0 | 0 | Bug #9 |", "**inspect advice acted on:** 33 % (1 of 3 posted findings)", "**Escape rate:** 50 % (1 of 2 stories)"} {
			if !strings.Contains(md, w) {
				t.Errorf("report lacks %q:\n%s", w, md)
			}
		}
	}
	for i := range d.PRs {
		d.PRs[i].LinkStories() // as Fetch does; the snapshot keeps the links, not the body
	}
	r := BuildRetro(in, d, 25)
	check(r)
	snap := Freeze(&in, d, "retro/x.inputs.json")
	thawed := &Data{Dev: "dev", Days: 14, Bots: d.Bots, Builds: d.Builds, Dismissed: d.Dismissed}
	in2, err := Thaw(snap, "retro/x.inputs.json", thawed)
	if err != nil {
		t.Fatal(err)
	}
	r2 := BuildRetro(in2, thawed, 25)
	check(r2)
	if string(r2.Markdown()) != string(BuildRetro(in, d, 25).Markdown()) {
		t.Error("the replayed retro differs")
	}
}

// The recurrence check (P4, S-7.7c): an evolve PR's problem came back when a
// retro Feedback issue opened after its merge, or a finding of this window,
// names the same skill. A Feedback from before the merge, a recurrence
// finding itself, and an evolve PR merged more than 30 days before the cursor
// do not count.
func TestRetroRecurrence(t *testing.T) {
	in := Inputs{Repo: "o/r", Since: at(0), Until: at(48)}
	d := &Data{Dev: "dev",
		PRs: []PR{{Number: 20, Base: "dev", Head: "evolve/grill-5", MergedAt: at(10)}, {Number: 21, Base: "dev", Head: "evolve/ready-6", MergedAt: at(12)},
			{Number: 22, Base: "dev", Head: "evolve/plan-7", MergedAt: at(-24 * 35)}, {Number: 23, Base: "dev", Head: "evolve/slice-8", MergedAt: at(5)},
			{Number: 24, Base: "dev", Head: "feat/grill-9", MergedAt: at(6)}},
		Items: []Item{
			{Number: 30, Title: "Feedback: retro: skill `grill` correction rate 2/2 above 20 %, w2", CreatedAt: at(15)},
			{Number: 31, Title: "Feedback: retro: skill `slice` correction rate 2/2 above 20 %, w1", CreatedAt: at(4)},
			{Number: 32, Title: "Feedback: retro: evolve did not hold: skill `slice` (PR #9, Feedback #1) came back", CreatedAt: at(20)},
			{Number: 33, Title: "Feedback: retro: skill `plan` correction rate 1/1 above 20 %, w3", CreatedAt: at(20)},
		},
		AgentPRs:    []AgentPR{{Number: 40, MergedAt: at(30), Skills: []string{"ready"}}},
		Corrections: []autonomy.Correction{{PR: 40, Kind: autonomy.KindRevert, At: at(31)}},
	}
	r := BuildRetro(in, d, 0)
	var back []string
	for _, rc := range r.Recurrence {
		back = append(back, fmt.Sprintf("%s:%d", rc.Skill, len(rc.Back)))
	}
	if strings.Join(back, " ") != "grill:1 ready:1 slice:0" {
		t.Errorf("recurrence: %v", back)
	}
	var titles []string
	for _, f := range r.Findings {
		titles = append(titles, f.Title)
	}
	got := strings.Join(titles, "\n")
	for _, want := range []string{"evolve did not hold: skill `grill` (PR #20, Feedback #5) came back", "evolve did not hold: skill `ready` (PR #21, Feedback #6) came back"} {
		if !strings.Contains(got, want) {
			t.Errorf("findings lack %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "`slice` (PR #23") || strings.Contains(got, "`plan`") {
		t.Errorf("findings:\n%s", got)
	}
	if md := string(r.Markdown()); !strings.Contains(md, "| #23 | `slice` | #8 | 2026-10-12T05:00:00Z | no |") || !strings.Contains(md, "| #20 | `grill` | #5 | 2026-10-12T10:00:00Z | **yes**: Feedback #30 |") {
		t.Errorf("report:\n%s", md)
	}
}

// retro counts every Feedback issue opened before the cursor by its fix layer
// (S-7.7d); a PR, another type, and Feedback opened after the cursor are not
// counted.
func TestRetroFixLayers(t *testing.T) {
	in := Inputs{Repo: "o/r", Since: at(0), Until: at(48)}
	fb := func(n int, created time.Time, labels ...string) Item {
		return Item{Number: n, Title: "Feedback", CreatedAt: created, Labels: append([]string{"type:feedback"}, labels...)}
	}
	d := &Data{Dev: "dev", Items: []Item{fb(1, at(1), "fix:codebase"), fb(2, at(2), "fix:check"), fb(3, at(3), "fix:check", "needs:grill"),
		fb(4, at(4), "fix:prose"), fb(5, at(5)), fb(6, at(50), "fix:review"), {Number: 7, PR: true, Labels: []string{"type:feedback", "fix:prose"}, CreatedAt: at(1)},
		{Number: 8, Labels: []string{"type:story", "fix:check"}, CreatedAt: at(1)},
		// Fix Stories by their fix:<layer> label (D-7.7e): #9 merged by the
		// cursor, #10 done without a PR, #11 merged after the cursor, #12 a
		// Story opened after it, #13 a Story with no fix label.
		{Number: 9, Labels: []string{"type:story", "fix:check"}, CreatedAt: at(2)},
		{Number: 10, Labels: []string{"type:story", "status:done", "fix:codebase"}, CreatedAt: at(2)},
		{Number: 11, Labels: []string{"type:story", "fix:check"}, CreatedAt: at(2)},
		{Number: 12, Labels: []string{"type:story", "fix:review"}, CreatedAt: at(49)},
		{Number: 13, Labels: []string{"type:story", "status:done"}, CreatedAt: at(2)}},
		PRs: []PR{{Number: 20, Base: "dev", MergedAt: at(10), Refs: []int{9}}, {Number: 21, Base: "dev", MergedAt: at(50), Refs: []int{11}}}}
	md := string(BuildRetro(in, d, 0).Markdown())
	for _, want := range []string{"| codebase | 1 | #1 | 1 | #10 |", "| check | 2 | #2, #3 | 3 | #9 |", "| prose | 1 | #4 | 0 | — |", "| review | 0 | — | 0 | — |", "| unclassified | 1 | #5 | 0 | — |"} {
		if !strings.Contains(md, want) {
			t.Errorf("report lacks %q:\n%s", want, md)
		}
	}
}
