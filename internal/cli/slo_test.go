package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	aayudha "github.com/rahulnakmol/agentic-ayudha"
	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
	"github.com/rahulnakmol/agentic-ayudha/internal/production"
)

const sloDoc = "# SLO line: asking a benefits question\n\n" +
	"## Journey\n\nA member asks a benefits question and gets a cited answer (query q1).\n\n" +
	"## Objectives\n\n`ask-availability`: 95 % of asks on `/api/ask` without a server error over 28 days (query q1).\n\n" +
	"## Error budget\n\nThe budget is 5 % of asks; 2 of 20 asks failed in the export, so it is spent (query q3).\n\n" +
	"## Alerts\n\nPage when the 1h burn rate exceeds 14.4, with the runbook named in ops/slos.toml.\n\n" +
	"## Budget policy\n\nWhile the budget is spent, release ships only Bugs for the SLOs in ops/slos.toml.\n\n" +
	"```toml\nversion = 1\n\n[[slo]]\nname = \"ask-availability\"\nservice = \"app\"\njourney = \"Ask a benefits question\"\nsli = \"availability\"\n" +
	"route = \"/api/ask\"\nobjective = 95.0\nwindow = \"28d\"\nalert = \"page when the 1h burn rate exceeds 14.4\"\nrunbook = \"ops/runbooks/ask-returns-500-after-a-deploy.md\"\n```\n"

// Decision #324: `aayu slo` checks an SLO document and writes its proposed
// ops/slos.toml, measures the budget from an export and records it on the
// ledger (where release reads it); `aayu runbook` names an alerting SLO with
// no runbook, and writes a checked one.
func TestSLOAndRunbook(t *testing.T) {
	dir, bare := ledgerCheckout(t)
	t0 := time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC)
	export := filepath.Join(t.TempDir(), "telemetry")
	writeIncidentExport(t, export, "aaaaaaa1", "bbbbbbb2", t0)
	cat, _ := os.ReadFile("testdata/incident/actions.toml")
	_ = os.MkdirAll(filepath.Join(dir, "ops"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "ops", "actions.toml"), []byte(strings.ReplaceAll(string(cat), "benefits-qa", "app")), 0o644)
	g := fake.New(t, "o/r", "User")
	env := map[string]string{"AAYU_GITHUB_TOKEN": "t"}
	opts := Options{GitHubAPI: g.URL(), GitHubGap: -1, Assets: aayudha.Assets, Dir: dir, Now: func() time.Time { return t0.Add(time.Hour) }}
	aayu := func(want int, args ...string) string {
		t.Helper()
		code, out, errOut := run(t, env, opts, append(args, "--repo", "o/r")...)
		if code != want {
			t.Fatalf("%v: exit %d, want %d: %s %s", args, code, want, out, errOut)
		}
		return out
	}

	// No SLOs yet: the grounding asks for the first one, and runbook has nothing to cover.
	if out := aayu(0, "slo"); !strings.Contains(out, "No SLOs are defined yet") {
		t.Fatalf("slo with no SLOs:\n%s", out)
	}
	doc := filepath.Join(t.TempDir(), "slo.md")
	_ = os.WriteFile(doc, []byte(strings.Replace(sloDoc, "over 28 days", "over the month", 1)), 0o644)
	if out := aayu(ExitBlocked, "slo", "--telemetry", export, "--draft", doc, "--create"); !strings.Contains(out, "state each SLO's window in days") {
		t.Fatalf("a document with no window:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(dir, "ops", "slos.toml")); err == nil {
		t.Fatal("a document with findings wrote ops/slos.toml")
	}
	_ = os.WriteFile(doc, []byte(sloDoc), 0o644)
	aayu(0, "slo", "--telemetry", export, "--draft", doc, "--create")
	defs, _, err := readSLOs(dir)
	if err != nil || defs == nil || defs.SLOs[0].Objective != 95 {
		t.Fatalf("ops/slos.toml: %+v %v", defs, err)
	}

	// Measured and recorded: 2 of 20 asks failed against a 5 % budget, so it is spent.
	var rep sloReport
	_ = json.Unmarshal([]byte(aayu(0, "slo", "--json")), &rep)
	if !strings.Contains(rep.Stale, "no error-budget status is recorded") {
		t.Fatalf("before a record: %+v", rep.Stale)
	}
	aayu(0, "slo", "--telemetry", export, "--record")
	var st production.Status
	if err := json.Unmarshal([]byte(git(t, bare, "show", "ledger:slo/status.json")), &st); err != nil || len(st.SLOs) != 1 ||
		st.SLOs[0].Total != 20 || st.SLOs[0].Good != 18 || !st.SLOs[0].Spent || !st.ComputedAt.Equal(t0.Add(time.Hour)) {
		t.Fatalf("slo/status.json: %+v %v", st, err)
	}
	rep = sloReport{}
	_ = json.Unmarshal([]byte(aayu(0, "slo", "--json")), &rep)
	if rep.Stale != "" || strings.Join(rep.Spent, ",") != "ask-availability" {
		t.Fatalf("after the record: stale %q, spent %v", rep.Stale, rep.Spent)
	}

	// The alerting SLO has no runbook until a checked one is written.
	if out := aayu(ExitBlocked, "runbook"); !strings.Contains(out, `SLO "ask-availability" alerts but has no runbook`) {
		t.Fatalf("runbook before an entry:\n%s", out)
	}
	rb, _ := os.ReadFile("../production/testdata/runbook.md")
	draft := filepath.Join(t.TempDir(), "rb.md")
	_ = os.WriteFile(draft, []byte(strings.Replace(string(rb), "--action rollback-app", "--action restart-app", 1)), 0o644)
	if out := aayu(ExitBlocked, "runbook", "--draft", draft, "--create"); !strings.Contains(out, `"restart-app": not in ops/actions.toml`) {
		t.Fatalf("a runbook with an unknown action:\n%s", out)
	}
	_ = os.WriteFile(draft, rb, 0o644)
	aayu(0, "runbook", "--draft", draft, "--create")
	if out := aayu(0, "runbook"); !strings.Contains(out, "1 runbook(s) checked; every alerting SLO has one") {
		t.Fatalf("runbook after the entry:\n%s", out)
	}
}
