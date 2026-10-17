package production

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/seats"
)

var t0 = time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC)

// stack is a Go stack as debug.Stack prints it from a deferred recover: the
// recovery frames come before panic(), the faulting frame after it.
const stack = `goroutine 7 [running]:
runtime/debug.Stack()
	/usr/local/go/src/runtime/debug/stack.go:26 +0x5e
example.com/app/api.Observe.func1.1()
	/build/src/app/api/observe.go:37 +0x8a
panic({0x8c5a40?, 0xc000020150?})
	/usr/local/go/src/runtime/panic.go:791 +0x132
example.com/app/answer.(*Index).Answer(0xc0000b2000, {0xc00001a0c0, 0x14})
	/build/src/app/answer/answer.go:120 +0x9f5
example.com/app/ask.(*Service).Ask(0xc0000a6040)
	/build/src/app/ask/ask.go:88 +0x1c5`

func exportFixture() *Export {
	e := &Export{Dir: "x", Deploys: []Deploy{
		{Time: t0, Service: "app", Environment: "production", Revision: "aaaaaaa1"},
		{Time: t0.Add(10 * time.Minute), Service: "app", Environment: "production", Revision: "bbbbbbb2", Previous: "aaaaaaa1"},
	}}
	add := func(at time.Duration, version string, status int, ms int, exc bool) {
		s := Span{TraceID: "t", Service: "app", Version: version, Environment: "production", Route: "/api/ask", Status: status,
			Error: status >= 500, Start: t0.Add(at), End: t0.Add(at + time.Duration(ms)*time.Millisecond)}
		if exc {
			s.Exceptions = []Exception{{Type: "runtime.boundsError", Message: "index out of range [1] with length 1; token=ghp_" + strings.Repeat("a", 36), Stack: stack}}
		}
		e.Spans = append(e.Spans, s)
	}
	for i := range 10 {
		add(time.Duration(i)*time.Minute, "aaaaaaa1", 200, 10+i, false) // old revision: clean, 10-19 ms
	}
	for i := range 10 {
		add(10*time.Minute+time.Duration(i)*time.Minute, "bbbbbbb2", map[bool]int{true: 500, false: 200}[i%5 == 0], 30, i%5 == 0)
	}
	// Another service in the same export must stay out of scope.
	e.Spans = append(e.Spans, Span{Service: "other", Version: "bbbbbbb2", Status: 500, Error: true, Start: t0.Add(15 * time.Minute), End: t0.Add(15 * time.Minute)})
	return e
}

func runQuery(t *testing.T, text string, resolve Resolver) Result {
	t.Helper()
	q, err := ParseQuery(text, t0, t0.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	r, err := Run(exportFixture(), q, resolve)
	if err != nil {
		t.Fatalf("%s: %v", text, err)
	}
	return r
}

// Queries are bounded, scoped to one service, and aggregate by revision; an
// exception's top frame is the faulting frame inside the repository, not the
// recovery path, and secrets in messages are redacted.
func TestRun(t *testing.T) {
	r := runQuery(t, "requests by version where service=app", nil)
	got := map[string][]string{}
	for _, row := range r.Rows {
		got[row[0]] = row
	}
	if a, b := got["aaaaaaa1"], got["bbbbbbb2"]; len(r.Rows) != 2 || a[1] != "10" || a[2] != "0" || b[2] != "2" || b[3] != "20.0%" || a[5] != "19.0" {
		t.Errorf("requests by version: %v", r.Rows)
	}
	resolve := func(f string) (string, bool) {
		if p, ok := strings.CutPrefix(f, "/build/src/"); ok {
			return "src/" + p, true
		}
		return "", false
	}
	r = runQuery(t, "exceptions by version where service=app", resolve)
	if len(r.Rows) != 1 || r.Rows[0][3] != "src/app/answer/answer.go:120" || r.Rows[0][4] != "2" || strings.Contains(r.Rows[0][2], "ghp_") {
		t.Errorf("exceptions: %v", r.Rows)
	}
	if r := runQuery(t, "errors where service=app version=bbbbbbb", nil); len(r.Rows) != 1 || r.Rows[0][2] != "2" {
		t.Errorf("errors on an abbreviated revision: %v", r.Rows)
	}
	if r := runQuery(t, "requests by version where service=app limit 1", nil); len(r.Rows) != 1 || r.Matched != 2 || !r.Truncated {
		t.Errorf("limit: %+v", r)
	}
	if r := runQuery(t, "deploys where service=app", nil); len(r.Rows) != 2 || r.Rows[1][4] != "aaaaaaa1" {
		t.Errorf("deploys: %v", r.Rows)
	}
	if d, ok := exportFixture().Deployed("app", "production", t0.Add(5*time.Minute)); !ok || d.Revision != "aaaaaaa1" {
		t.Errorf("deployed at 09:05 = %v", d)
	}
}

func TestQueryBounds(t *testing.T) {
	for text, want := range map[string]string{
		"requests":                             "no scope",
		"requests where service=app limit 501": "limit 501",
		"trace where service=app":              "trace needs",
		"select * from spans":                  "kind",
		"requests where nope=1":                "where",
	} {
		q, err := ParseQuery(text, t0, t0.Add(time.Hour))
		if err == nil {
			err = q.Check()
		}
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: %v, want %q", text, err, want)
		}
	}
	q, _ := ParseQuery("requests where service=app", t0, t0.Add(8*24*time.Hour))
	if err := q.Check(); err == nil || !strings.Contains(err.Error(), "at most") {
		t.Errorf("an 8-day window: %v", err)
	}
	if s := q.String(); !strings.Contains(s, "| window 2026-10-17T09:00:00Z..") || !strings.HasSuffix(s, "| limit 50") {
		t.Errorf("canonical text %q names no window or limit", s)
	}
}

func TestCheckInvestigation(t *testing.T) {
	good, err := os.ReadFile("testdata/note.md")
	if err != nil {
		t.Fatal(err)
	}
	pr := seats.Practice{ID: "telemetry-investigation", Startup: seats.Depth{Artefact: "Investigation note", Shape: "x"},
		Scaleup: seats.Depth{Artefact: "Incident assessment", Shape: "x"}, Enterprise: seats.Depth{Artefact: "Investigation record", Shape: "x"}}
	known := func(c string) bool { return c != "query q9" }
	action := func(name, params string) string {
		if name != "rollback-app" {
			return "not in the catalog"
		}
		return ""
	}
	if f := CheckInvestigation(string(good), 1, "startup", pr, known, action); len(f) != 0 {
		t.Fatalf("the recorded note: %v", f)
	}
	for name, tt := range map[string]struct{ old, new, want string }{
		"one hypothesis":        {"| H2 |", "| H1b |", "at least two competing hypotheses"},
		"an invented query":     {"(query q2)", "(query q9)", "cites query q9"},
		"an unknown action":     {"--action rollback-app", "--action restart-api", `"restart-api": not in the catalog`},
		"a claimed rollback":    {"Propose `aayu respond", "We rolled back; propose `aayu respond", "claims a production change"},
		"a shell mitigation":    {"Propose `aayu respond", "Run `kubectl rollout undo deploy/app` or propose `aayu respond", `runs "kubectl"`},
		"a deploy script":       {"Propose `aayu respond", "Run `deploy/deploy.sh --rollback` or propose `aayu respond", `runs "deploy/deploy.sh -"`},
		"another incident":      {"aayu respond 1 ", "aayu respond 2 ", "names another incident"},
		"an unbounded template": {"## Missing evidence and next queries\n", "## Missing evidence and next queries\n\n```kql\nAppExceptions\n| summarize count() by AppVersion\n```\n", "no time window"},
		"no rollback":           {"Rollback: `deploy-app` with `revision=bbbbbbb2` (ops/actions.toml); stop if `aaaaaaa1` shows any 500.", "Then `deploy-app` later (ops/actions.toml).", "rollback or stop condition"},
	} {
		bad := strings.Replace(string(good), tt.old, tt.new, 1)
		if bad == string(good) {
			t.Fatalf("%s: %q is not in the note", name, tt.old)
		}
		if f := strings.Join(CheckInvestigation(bad, 1, "startup", pr, known, action), "\n"); !strings.Contains(f, tt.want) {
			t.Errorf("%s: findings %q, want %q", name, f, tt.want)
		}
	}
}

func TestSLOs(t *testing.T) {
	for _, tt := range []struct{ toml, want string }{
		{"version = 1\n[[slo]]\nname = \"a\"\nservice = \"app\"\njourney = \"j\"\nsli = \"availability\"\nobjective = 100.0\nwindow = \"28d\"\n", "leaves no error budget"},
		{"version = 1\n[[slo]]\nname = \"a\"\nservice = \"app\"\njourney = \"j\"\nsli = \"latency\"\nobjective = 99.0\nwindow = \"28d\"\n", "needs a threshold"},
		{"version = 1\n[[slo]]\nname = \"a\"\nservice = \"app\"\njourney = \"j\"\nsli = \"availability\"\nobjective = 99.0\nwindow = \"2h\"\n", "between 1d and 90d"},
		{"version = 1\n[[slo]]\nname = \"a\"\nservice = \"app\"\njourney = \"j\"\nsli = \"availability\"\nobjective = 99.0\nwindow = \"28d\"\nowner = \"x\"\n", "unknown keys"},
	} {
		if _, err := ParseSLOs([]byte(tt.toml)); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("ParseSLOs: %v, want %q", err, tt.want)
		}
	}
	defs, err := ParseSLOs([]byte(`version = 1
[[slo]]
name = "ask-availability"
service = "app"
journey = "Ask"
sli = "availability"
objective = 95.0
window = "28d"
[[slo]]
name = "ask-loose"
service = "app"
journey = "Ask"
sli = "availability"
route = "/api/ask"
objective = 80.0
window = "28d"
[[slo]]
name = "ask-latency"
service = "app"
journey = "Ask"
sli = "latency"
threshold = "25ms"
objective = 50.0
window = "7d"
`))
	if err != nil {
		t.Fatal(err)
	}
	// The fixture: 20 asks of app (2 server errors on the new revision, which also
	// takes 30 ms; the old one 10-19 ms) and one error of another service.
	to := t0.Add(20 * time.Minute)
	got := map[string]Measured{}
	for _, o := range defs.SLOs {
		got[o.Name] = Measure(exportFixture(), o, to)
	}
	// 2 bad of 20 against a 5 % budget (1 bad event): twice the budget, so overspent by the whole budget.
	if m := got["ask-availability"]; m.Total != 20 || m.Good != 18 || m.Value != 90 || m.BudgetRemaining != -1 || !m.Spent || m.Burn1h != 2 {
		t.Fatalf("availability: %+v", m)
	}
	// 2 bad of 20 against a 20 % budget (4 bad events): half of it left.
	if m := got["ask-loose"]; m.BudgetRemaining != 0.5 || m.Spent {
		t.Fatalf("loose availability: %+v", m)
	}
	// The ten 30 ms asks are slower than 25 ms: exactly the 50 % budget, so spent.
	if m := got["ask-latency"]; m.Good != 10 || m.BudgetRemaining != 0 || !m.Spent {
		t.Fatalf("latency: %+v", m)
	}
	st := Status{ComputedAt: to, SLOs: []Measured{got["ask-availability"], got["ask-latency"]}}
	if s := st.Stale(to.Add(7*24*time.Hour), defs); !strings.Contains(s, `no measure of SLO "ask-loose"`) {
		t.Fatalf("stale for a missing SLO: %q", s)
	}
	st.SLOs = append(st.SLOs, got["ask-loose"])
	if s := st.Stale(to.Add(StaleAfter), defs); s != "" || st.Stale(to.Add(StaleAfter+time.Minute), defs) == "" {
		t.Fatalf("stale at the boundary: %q", s)
	}
	if spent := st.Spent(); strings.Join(spent, ",") != "ask-availability,ask-latency" {
		t.Fatalf("spent: %v", spent)
	}
}

func TestCheckRunbook(t *testing.T) {
	good, err := os.ReadFile("testdata/runbook.md")
	if err != nil {
		t.Fatal(err)
	}
	pr := seats.Practice{ID: "runbooks-toil", Startup: seats.Depth{Artefact: "Runbook entry", Shape: "x"},
		Scaleup: seats.Depth{Artefact: "Runbook set", Shape: "x"}, Enterprise: seats.Depth{Artefact: "Operational readiness review", Shape: "x"}}
	defs, _ := ParseSLOs([]byte("version = 1\n[[slo]]\nname = \"ask-availability\"\nservice = \"app\"\njourney = \"Ask\"\nsli = \"availability\"\nobjective = 99.5\nwindow = \"28d\"\n"))
	action := func(name, params string) string {
		if name != "rollback-app" && name != "deploy-app" {
			return "not in the catalog"
		}
		return ""
	}
	rollback := func(name string) string {
		return map[string]string{"rollback-app": "deploy-app", "deploy-app": "rollback-app"}[name]
	}
	check := func(md string) string {
		return strings.Join(CheckRunbook(md, "startup", pr, func(string) bool { return true }, action, rollback, defs), "\n")
	}
	if f := check(string(good)); f != "" {
		t.Fatalf("the recorded runbook: %s", f)
	}
	for name, tt := range map[string]struct{ old, new, want string }{
		"no SLO":              {"The `ask-availability` alert", "The `ask-errors` alert", "names no SLO"},
		"an unbounded query":  {`"errors by version,route limit 20"`, `"errors by version,route limit 900"`, "limit 900"},
		"an unknown action":   {"--action rollback-app", "--action restart-api", `"restart-api": not in the catalog`},
		"no catalog rollback": {"`deploy-app` with the reverted revision", "Redeploy by hand", "the catalog's rollback of rollback-app is deploy-app"},
		"a claimed run":       {"Propose `aayu respond", "We rolled back; propose `aayu respond", "claims a production change"},
		"a shell mitigation":  {"Propose `aayu respond", "Run `kubectl rollout undo` or propose `aayu respond", `runs "kubectl"`},
		"a missing part":      {"## Tried\n\nNot yet.\n", "", `missing part "## Tried"`},
	} {
		bad := strings.Replace(string(good), tt.old, tt.new, 1)
		if bad == string(good) {
			t.Fatalf("%s: %q is not in the runbook", name, tt.old)
		}
		if f := check(bad); !strings.Contains(f, tt.want) {
			t.Errorf("%s: findings %q, want %q", name, f, tt.want)
		}
	}
}

func TestCheckPostmortem(t *testing.T) {
	good, err := os.ReadFile("testdata/postmortem.md")
	if err != nil {
		t.Fatal(err)
	}
	pr := seats.Practice{ID: "incident-command", Startup: seats.Depth{Artefact: "Incident log", Shape: "x"},
		Scaleup: seats.Depth{Artefact: "Postmortem", Shape: "x"}, Enterprise: seats.Depth{Artefact: "Incident review", Shape: "x"}}
	check := func(md, stage string) string {
		return strings.Join(CheckPostmortem(md, 1, stage, pr, func(c string) bool { return c != "query q9" }), "\n")
	}
	if f := check(string(good), "startup"); f != "" {
		t.Fatalf("the recorded postmortem: %s", f)
	}
	for name, tt := range map[string]struct{ old, new, want string }{
		"a person blamed":    {"- Contributing: the tests", "- Contributing: @dev forgot to test it, and the tests", `"@dev forgot" blames a person`},
		"human error":        {"Root cause: the tie-break", "Root cause: human error; the tie-break", `"human error" blames a person`},
		"no contributing":    {"- Contributing:", "- Also:", "name the contributing conditions"},
		"no owners":          {"| Action | Owner | Tracking |", "| Action | Who | Tracking |", "give each action an owner"},
		"an invented query":  {"(query q5).\n- The catalog", "(query q9).\n- The catalog", "cites query q9"},
		"the wrong artefact": {"# Incident log:", "# Postmortem:", "Incident log"},
	} {
		bad := strings.Replace(string(good), tt.old, tt.new, 1)
		if bad == string(good) {
			t.Fatalf("%s: %q is not in the postmortem", name, tt.old)
		}
		if f := check(bad, "startup"); !strings.Contains(f, tt.want) {
			t.Errorf("%s: findings %q, want %q", name, f, tt.want)
		}
	}
	noIncident := strings.ReplaceAll(strings.ReplaceAll(string(good), " | #1 |", " | query q1 |"), "#1, query q5", "query q5")
	if f := check(noIncident, "startup"); !strings.Contains(f, `cite the incident's timeline (#1)`) {
		t.Errorf("a timeline that never cites the incident: %q", f)
	}
}
