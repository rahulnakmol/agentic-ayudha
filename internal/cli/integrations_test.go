package cli

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/integrations"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations/fixture"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations/providers"
	"github.com/rahulnakmol/agentic-ayudha/internal/secrets"
)

// counter fails every request and counts them.
type counter struct{ n int }

func (c *counter) RoundTrip(*http.Request) (*http.Response, error) {
	c.n++
	return nil, errors.New("no requests expected")
}

// TestBaselineExitCodes is the WP5.2 acceptance through the CLI: a baseline
// from an adapter that is not configured exits 5 naming the [integrations] key
// to set, and a write-scoped key exits 4 with no request sent. grill (WP5.3)
// and measure (WP7.2) call providers.Baseline; a stand-in measure does here.
func TestBaselineExitCodes(t *testing.T) {
	withHandler(t, "measure", func(ctx context.Context, e *Env, _ Values) error {
		w := integrations.Window{From: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)}
		_, err := providers.Baseline(ctx, e.integrationDeps("o/r"), "posthog", integrations.Query{Metric: "users", Window: w})
		return err
	})
	rt := &counter{}
	env := map[string]string{"AAYU_SECRET_POSTHOG": "phx_test"}
	opts := func() Options {
		return Options{HTTP: &http.Client{Transport: rt}, Secrets: secrets.New(secrets.Env{Getenv: func(k string) string { return env[k] }})}
	}

	code, _, stderr := run(t, env, opts(), "measure")
	if code != ExitExternal || !strings.Contains(stderr, "[integrations.posthog] is not set") || !strings.Contains(stderr, `project = "<the PostHog project ID>"`) {
		t.Fatalf("missing adapter: exit %d, stderr %q; want 5 naming [integrations.posthog]", code, stderr)
	}
	code, _, stderr = run(t, env, opts(), "measure", "--set", "integrations.posthog.secret=posthog",
		"--set", "integrations.posthog.scope=write", "--set", "integrations.posthog.project=12345")
	if code != ExitBlocked || !strings.Contains(stderr, `[integrations.posthog] scope = "write"`) {
		t.Fatalf("write-scoped key: exit %d, stderr %q; want 4", code, stderr)
	}
	if rt.n != 0 {
		t.Fatalf("%d requests sent; want none", rt.n)
	}
}

// TestDoctorIntegrations runs `aayu doctor` against recorded provider answers:
// a verified read-only key, a key the provider reports as write-capable, a
// key whose scope the provider does not report, and an adapter that is not
// configured.
func TestDoctorIntegrations(t *testing.T) {
	ok := func(url, header, body string) fixture.Interaction {
		in := fixture.Interaction{Method: "GET", URL: url, Status: 200, Response: []byte(body)}
		if header != "" {
			in.Header = map[string]string{"X-OAuth-Scopes": header}
		}
		return in
	}
	r := &fixture.Replayer{T: t, Rec: &fixture.Recording{Interactions: []fixture.Interaction{
		ok("https://eu.posthog.com/api/personal_api_keys/@current/", "", `{"label":"aayudha","scopes":["query:read","insight:read"]}`),
		ok("https://sentry.io/api/0/", "", `{"auth":{"scopes":["event:read","project:write"]},"user":{"username":"founder"}}`),
		ok("https://api.github.com/user", "", `{"login":"rahulnakmol","type":"User"}`), // fine-grained: no X-OAuth-Scopes
	}}}
	env := map[string]string{"AAYU_SECRET_POSTHOG": "phx_test", "AAYU_SECRET_SENTRY": "sntrys_test", "AAYU_SECRET_GITHUB_READ": "github_pat_test"}
	repo := gitInit(t, "dev", map[string]string{".aayudha/config.toml": `version = 1
[integrations.posthog]
secret = "posthog"
scope = "read"
url = "https://eu.posthog.com"
project = "12345"
[integrations.sentry]
secret = "sentry"
scope = "read"
org = "acme"
project = "4507"
[integrations.github]
secret = "github-read"
scope = "read"
project = "rahulnakmol/agentic-ayudha"
`})
	_, out, errs := run(t, env, Options{Dir: repo, Exec: realGitWithLFS, LookPath: noTools, HTTP: &http.Client{Transport: r},
		Secrets: secrets.New(secrets.Env{Getenv: func(k string) string { return env[k] }})}, "doctor")
	r.Done()
	if !regexp.MustCompile(`FAIL +integrations +2 of 3 configured integration\(s\) reachable with a read-only key`).MatchString(out) {
		t.Errorf("doctor's integrations row is not the expected FAIL:\n%s", out)
	}
	for _, want := range []string{
		"posthog: reachable as personal API key aayudha; read-only scope verified (query:read, insight:read per GET /api/personal_api_keys/@current/)",
		"sentry: reachable, but the key is write-capable (project:write per GET /api/0/); read verbs refuse it",
		"github: reachable as rahulnakmol; the provider does not report this key's scope, so configured scope read is trusted",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("doctor output lacks %q:\n%s%s", want, out, errs)
		}
	}
}
