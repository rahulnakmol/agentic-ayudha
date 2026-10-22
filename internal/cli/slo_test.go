package cli

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	aayudha "github.com/rahulnakmol/agentic-ayudha"
	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations/fixture"
	"github.com/rahulnakmol/agentic-ayudha/internal/production"
	"github.com/rahulnakmol/agentic-ayudha/internal/secrets"
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

// TestSLOCrashFree (Decision #449, phase P1): a crash-free SLO is measured
// from Sentry's release health over the SLO window and recorded beside the
// export's measures with its source and digest; with no Sentry configured it
// is not measured, and no partial status is recorded.
func TestSLOCrashFree(t *testing.T) {
	dir, bare := ledgerCheckout(t)
	t0 := time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC)
	export := filepath.Join(t.TempDir(), "telemetry")
	writeIncidentExport(t, export, "aaaaaaa1", "bbbbbbb2", t0)
	_ = os.MkdirAll(filepath.Join(dir, "ops"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "ops", "slos.toml"), []byte("version = 1\n"+
		"[[slo]]\nname = \"ask-availability\"\nservice = \"app\"\njourney = \"Ask a benefits question\"\nsli = \"availability\"\nobjective = 95.0\nwindow = \"28d\"\n"+
		"[[slo]]\nname = \"app-crash-free\"\nservice = \"app\"\njourney = \"Open the app\"\nsli = \"crash-free\"\nobjective = 99.9\nwindow = \"28d\"\n"), 0o644)
	exp, err := production.LoadExport(export)
	if err != nil {
		t.Fatal(err)
	}
	_, last := exp.Span()
	to := last.Add(time.Second)
	sessions := fixture.Load(t, "../production/testdata/sentry/sessions.json").Interactions[0]
	sessions.URL = "https://sentry.io/api/0/organizations/acme/sessions/?" + url.Values{"field": {"sum(session)"}, "groupBy": {"session.status"}, "project": {"4507"},
		"start": {to.Add(-28 * 24 * time.Hour).UTC().Format(time.RFC3339)}, "end": {to.UTC().Format(time.RFC3339)}, "interval": {"1h"}}.Encode()
	scope := fixture.Interaction{Method: "GET", URL: "https://sentry.io/api/0/", Status: 200,
		Response: []byte(`{"auth":{"scopes":["alerts:read","event:read","org:read","project:read"]}}`)}
	r := &fixture.Replayer{T: t, Rec: &fixture.Recording{Source: "Shaped from Sentry's OpenAPI spec; see internal/production/testdata/sentry/sessions.json", Interactions: []fixture.Interaction{scope, sessions}}}
	g := fake.New(t, "o/r", "User")
	env := map[string]string{"AAYU_GITHUB_TOKEN": "t", "AAYU_SECRET_SENTRY": "sntrys_test"}
	opts := Options{GitHubAPI: g.URL(), GitHubGap: -1, Assets: aayudha.Assets, Dir: dir, Now: func() time.Time { return t0.Add(time.Hour) },
		HTTP: &http.Client{Transport: local{r}}, Secrets: secrets.New(secrets.Env{Getenv: func(k string) string { return env[k] }})}

	// No Sentry configured: the crash-free SLO is not measured, and nothing is recorded.
	code, out, errOut := run(t, env, opts, "slo", "--telemetry", export, "--record", "--repo", "o/r")
	if code != ExitBlocked || !strings.Contains(out+errOut, `not every SLO was measured`) || !strings.Contains(out+errOut, "app-crash-free") {
		t.Fatalf("no Sentry: exit %d %s %s; want the record refused naming the crash-free SLO", code, out, errOut)
	}
	if _, err := gitIn(t.Context(), bare, "show", "ledger:slo/status.json"); err == nil {
		t.Fatal("a status was recorded without the crash-free measure")
	}

	sentryFlags := []string{"--set", "integrations.sentry.secret=sentry", "--set", "integrations.sentry.scope=read",
		"--set", "integrations.sentry.org=acme", "--set", "integrations.sentry.project=4507"}
	code, out, errOut = run(t, env, opts, append([]string{"slo", "--telemetry", export, "--record", "--repo", "o/r"}, sentryFlags...)...)
	if code != 0 {
		t.Fatalf("with Sentry: exit %d %s %s", code, out, errOut)
	}
	r.Done()
	var st production.Status
	if err := json.Unmarshal([]byte(git(t, bare, "show", "ledger:slo/status.json")), &st); err != nil || len(st.SLOs) != 2 {
		t.Fatalf("slo/status.json: %+v %v", st, err)
	}
	cf := st.SLOs[1]
	if cf.Name != "app-crash-free" || cf.Good != 68545 || cf.Total != 68570 || cf.Source != "sentry project 4507 sessions" || len(cf.Digest) != 64 || st.SLOs[0].Source != "" {
		t.Errorf("recorded measures %+v; want the export's availability and Sentry's crash-free with its source and digest", st.SLOs)
	}

	// A token Sentry reports as able to write is refused before the sessions
	// read, and a rejected token leaves the SLO unmeasured; the record is refused.
	for _, c := range []struct {
		name, want string
		scope      fixture.Interaction
	}{
		{"write-capable", "event:admin", fixture.Interaction{Method: "GET", URL: "https://sentry.io/api/0/", Status: 200,
			Response: []byte(`{"auth":{"scopes":["event:admin","event:read","org:read","project:read"]}}`)}},
		{"rejected", "app-crash-free", fixture.Interaction{Method: "GET", URL: "https://sentry.io/api/0/", Status: 401, Response: []byte(`{"detail":"Invalid token"}`)}},
	} {
		r := &fixture.Replayer{T: t, Rec: &fixture.Recording{Source: "Shaped from Sentry's OpenAPI spec", Interactions: []fixture.Interaction{c.scope}}}
		o := opts
		o.HTTP = &http.Client{Transport: local{r}}
		code, out, errOut := run(t, env, o, append([]string{"slo", "--telemetry", export, "--record", "--repo", "o/r"}, sentryFlags...)...)
		r.Done()
		if code != ExitBlocked || !strings.Contains(out+errOut, "not every SLO was measured") || !strings.Contains(out+errOut, c.want) {
			t.Errorf("%s token: exit %d %s %s; want the record refused naming %s", c.name, code, out, errOut, c.want)
		}
	}
}
