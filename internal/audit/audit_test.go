package audit

import (
	"strings"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/autonomy"
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
		Prior: []PriorReport{{Path: "audit/x.md", Suites: []int64{99}}}}
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
	if md := string(r.Markdown()); !strings.Contains(md, "| #1 Big | #11 | S | 2 | 3 | 1 | 30.50 | merged without proof |") || !strings.Contains(md, "| `grill` | 2 | 1 |") {
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
