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

// local sends requests for the loopback fixture servers to the network and
// every other request to the replayer.
type local struct{ replay http.RoundTripper }

func (l local) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Hostname() == "127.0.0.1" {
		return http.DefaultTransport.RoundTrip(req)
	}
	return l.replay.RoundTrip(req)
}

// TestDoctorIntegrations runs `aayu doctor` against recorded provider answers
// (the WP5.2 and WP6.4 reachability checks): a verified read-only key, a key
// the provider reports as write-capable, keys whose scope the provider does
// not report (GitHub fine-grained, Figma), and Mobbin's MCP server, whose
// read-only tool list is its scope.
func TestDoctorIntegrations(t *testing.T) {
	mobbin := fixture.ServeMCP(t, "../integrations/mobbin/testdata/mcp.json", "mobbin_oauth_test")
	ok := func(url, header, body string) fixture.Interaction {
		in := fixture.Interaction{Method: "GET", URL: url, Status: 200, Response: []byte(body)}
		if header != "" {
			in.Header = map[string]string{"X-OAuth-Scopes": header}
		}
		return in
	}
	r := &fixture.Replayer{T: t, Rec: &fixture.Recording{Interactions: []fixture.Interaction{
		ok("https://eu.posthog.com/api/personal_api_keys/@current/", "", `{"label":"aayudha","scopes":["query:read","insight:read"]}`),
		// Decision #449: doctor then tries each PostHog capability with a cheap read.
		ok("https://eu.posthog.com/api/personal_api_keys/@current/", "", `{"label":"aayudha","scopes":["query:read","insight:read"]}`),
		{Method: "POST", URL: "https://eu.posthog.com/api/projects/12345/query/", Status: 200, Response: []byte(`{"results":[[1]]}`)},
		ok("https://sentry.io/api/0/", "", `{"auth":{"scopes":["event:read","project:write"]},"user":{"username":"founder"}}`),
		ok("https://api.github.com/user", "", `{"login":"rahulnakmol","type":"User"}`), // fine-grained: no X-OAuth-Scopes
		ok("https://api.figma.com/v1/me", "", `{"id":"1197346","handle":"Rahul"}`),
	}}}
	env := map[string]string{"AAYU_SECRET_POSTHOG": "phx_test", "AAYU_SECRET_SENTRY": "sntrys_test", "AAYU_SECRET_GITHUB_READ": "github_pat_test",
		"AAYU_SECRET_FIGMA": "figd_test", "AAYU_SECRET_MOBBIN": "mobbin_oauth_test"}
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
[integrations.figma]
secret = "figma"
scope = "read"
[integrations.mobbin]
secret = "mobbin"
scope = "read"
url = "` + mobbin.URL + `"
`})
	_, out, errs := run(t, env, Options{Dir: repo, Exec: realGitWithLFS, LookPath: noTools, HTTP: &http.Client{Transport: local{r}},
		Secrets: secrets.New(secrets.Env{Getenv: func(k string) string { return env[k] }})}, "doctor")
	r.Done()
	if !regexp.MustCompile(`FAIL +integrations +4 of 5 configured integration\(s\) reachable with a read-only key`).MatchString(out) {
		t.Errorf("doctor's integrations row is not the expected FAIL:\n%s", out)
	}
	for _, want := range []string{
		"posthog: reachable as personal API key aayudha; read-only scope verified (query:read, insight:read per GET /api/personal_api_keys/@current/)",
		"posthog, HogQL queries (query:read; used by measure, grill, experiment, readout): works: project 12345 answered SELECT 1",
		"sentry: reachable, but the key is write-capable (project:write per GET /api/0/); read verbs refuse it",
		"github: reachable as rahulnakmol; the provider does not report this key's scope, so configured scope read is trusted",
		"figma: reachable as Rahul; the provider does not report this key's scope, so configured scope read is trusted",
		"mobbin: reachable as mobbin; read-only scope verified (search_flows, search_screens, search_sections per MCP tools/list at " + mobbin.URL + ")",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("doctor output lacks %q:\n%s%s", want, out, errs)
		}
	}
}
