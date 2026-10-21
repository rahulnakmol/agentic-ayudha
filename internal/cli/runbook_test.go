package cli

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	aayudha "github.com/rahulnakmol/agentic-ayudha"
	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations/fixture"
	"github.com/rahulnakmol/agentic-ayudha/internal/secrets"
)

// TestRunbookAlertsLive (Decision #449, phase P1): `aayu runbook` checks the
// alerts a runbook's Symptom names against the configured Sentry and PostHog
// projects. A missing alert is a finding; with no provider configured, or a
// provider whose key is refused, the alerts are not checked and never a finding.
func TestRunbookAlertsLive(t *testing.T) {
	dir, _ := ledgerCheckout(t)
	cat, _ := os.ReadFile("testdata/incident/actions.toml")
	rb, _ := os.ReadFile("../production/testdata/runbook.md")
	_ = os.MkdirAll(filepath.Join(dir, "ops", "runbooks"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "ops", "actions.toml"), []byte(strings.ReplaceAll(string(cat), "benefits-qa", "app")), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "ops", "slos.toml"), []byte("version = 1\n[[slo]]\nname = \"ask-availability\"\nservice = \"app\"\n"+
		"journey = \"Ask a benefits question\"\nsli = \"availability\"\nobjective = 95.0\nwindow = \"28d\"\nalert = \"page when the 1h burn rate exceeds 14.4\"\n"+
		"runbook = \"ops/runbooks/ask.md\"\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "ops", "runbooks", "ask.md"), []byte(strings.Replace(string(rb), "exceeds 14.4 (ops/slos.toml).",
		"exceeds 14.4 (ops/slos.toml), with `error-monitor`, `exceptions-spike`, and `disk-full`.", 1)), 0o644)

	sentryScope := func(scopes string) fixture.Interaction {
		return fixture.Interaction{Method: "GET", URL: "https://sentry.io/api/0/", Status: 200, Response: []byte(`{"auth":{"scopes":` + scopes + `}}`)}
	}
	posthogScope := fixture.Interaction{Method: "GET", URL: "https://eu.posthog.com/api/personal_api_keys/@current/", Status: 200,
		Response: []byte(`{"label":"aayudha","scopes":["query:read","alert:read"]}`)}
	g := fake.New(t, "o/r", "User")
	env := map[string]string{"AAYU_GITHUB_TOKEN": "t", "AAYU_SECRET_SENTRY": "sntrys_test", "AAYU_SECRET_POSTHOG": "phx_test"}
	providerFlags := []string{"--set", "integrations.sentry.secret=sentry", "--set", "integrations.sentry.scope=read",
		"--set", "integrations.sentry.org=acme", "--set", "integrations.sentry.project=4507",
		"--set", "integrations.posthog.secret=posthog", "--set", "integrations.posthog.scope=read",
		"--set", "integrations.posthog.project=12345", "--set", "integrations.posthog.url=https://eu.posthog.com"}
	aayu := func(want int, rec []fixture.Interaction, flags ...string) string {
		t.Helper()
		r := &fixture.Replayer{T: t, Rec: &fixture.Recording{Source: "Shaped from Sentry's and PostHog's OpenAPI specs; see internal/production/testdata", Interactions: rec}}
		opts := Options{GitHubAPI: g.URL(), GitHubGap: -1, Assets: aayudha.Assets, Dir: dir, Now: func() time.Time { return time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC) },
			HTTP: &http.Client{Transport: local{r}}, Secrets: secrets.New(secrets.Env{Getenv: func(k string) string { return env[k] }})}
		code, out, errOut := run(t, env, opts, append([]string{"runbook", "--repo", "o/r"}, flags...)...)
		r.Done()
		if code != want {
			t.Fatalf("runbook %v: exit %d, want %d: %s %s", flags, code, want, out, errOut)
		}
		return out
	}

	// Both providers answer: the alert neither has is the one finding.
	rec := append([]fixture.Interaction{sentryScope(`["alerts:read","event:read","org:read","project:read"]`)},
		fixture.Load(t, "../production/testdata/sentry/alerts.json").Interactions...)
	rec = append(rec, posthogScope)
	rec = append(rec, fixture.Load(t, "../production/testdata/posthog/alerts.json").Interactions...)
	out := aayu(ExitBlocked, rec, providerFlags...)
	for _, want := range []string{"the Symptom names alert `disk-full`, which is not an alert or monitor in posthog or sentry",
		"ops/runbooks/ask.md alerts: `error-monitor` found in sentry; `exceptions-spike` found in posthog; `disk-full` missing.",
		"alerts and monitors in sentry project 4507 (GET https://sentry.io/api/0/organizations/acme/detectors/?project=4507",
		"alerts and monitors in posthog project 12345 (GET https://eu.posthog.com/api/projects/12345/alerts/?limit=100; sha256 "} {
		if !strings.Contains(out, want) {
			t.Errorf("both providers: output lacks %q:\n%s", want, out)
		}
	}
	if strings.Count(out, "not an alert or monitor") != 1 {
		t.Errorf("both providers: want exactly one alert finding:\n%s", out)
	}

	// No provider configured: no request, every alert not checked, no finding.
	out = aayu(0, nil)
	if !strings.Contains(out, "`error-monitor` not checked; `exceptions-spike` not checked; `disk-full` not checked.") ||
		!strings.Contains(out, "[integrations.sentry] is not set") || strings.Contains(out, "not an alert or monitor") {
		t.Errorf("no provider:\n%s", out)
	}

	// Sentry answers but PostHog rejects its key: an alert Sentry lacks may be
	// in PostHog, so it is not checked rather than missing.
	rec = append([]fixture.Interaction{sentryScope(`["alerts:read","event:read","org:read","project:read"]`)},
		fixture.Load(t, "../production/testdata/sentry/alerts.json").Interactions...)
	rec = append(rec, fixture.Interaction{Method: "GET", URL: "https://eu.posthog.com/api/personal_api_keys/@current/", Status: 401, Response: []byte(`{"detail":"Invalid token"}`)})
	out = aayu(0, rec, providerFlags...)
	if !strings.Contains(out, "`error-monitor` found in sentry; `exceptions-spike` not checked; `disk-full` not checked.") ||
		!strings.Contains(out, "not checked in posthog:") || strings.Contains(out, "not an alert or monitor") {
		t.Errorf("one provider unread:\n%s", out)
	}

	// A Sentry token that can write and a PostHog key PostHog rejects: no list is read.
	rec = []fixture.Interaction{sentryScope(`["event:admin","event:read","org:read","project:read"]`),
		{Method: "GET", URL: "https://eu.posthog.com/api/personal_api_keys/@current/", Status: 401, Response: []byte(`{"detail":"Invalid token"}`)}}
	out = aayu(0, rec, providerFlags...)
	if !strings.Contains(out, "not checked in sentry: the key can write (event:admin)") || !strings.Contains(out, "not checked in posthog:") ||
		!strings.Contains(out, "`disk-full` not checked.") || strings.Contains(out, "not an alert or monitor") {
		t.Errorf("refused keys:\n%s", out)
	}
}
