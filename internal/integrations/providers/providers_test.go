package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations/fixture"
	"github.com/rahulnakmol/agentic-ayudha/internal/secrets"
)

var (
	sept    = integrations.Window{From: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)}
	fetched = time.Date(2026, 10, 12, 9, 30, 0, 0, time.UTC)
)

// deps builds adapter dependencies from --set style config entries and
// AAYU_SECRET_* values, with requests answered by rt.
func deps(t *testing.T, rt http.RoundTripper, env map[string]string, sets ...string) Deps {
	t.Helper()
	l, err := config.Load(config.Sources{Sets: sets})
	if err != nil {
		t.Fatal(err)
	}
	return Deps{
		Config:    &l.Config,
		Secrets:   secrets.New(secrets.Env{Getenv: func(k string) string { return env[k] }}),
		Transport: rt, Repo: "rahulnakmol/agentic-ayudha",
		Now: func() time.Time { return fetched },
	}
}

var configured = []string{
	"integrations.posthog.secret=posthog", "integrations.posthog.scope=read", "integrations.posthog.project=12345",
	"integrations.sentry.secret=sentry", "integrations.sentry.scope=read", "integrations.sentry.org=acme", "integrations.sentry.project=4507",
	"integrations.github.secret=github-read", "integrations.github.scope=read",
	"integrations.growthbook.secret=growthbook", "integrations.growthbook.scope=read",
}

var keys = map[string]string{"AAYU_SECRET_POSTHOG": "phx_test", "AAYU_SECRET_SENTRY": "sntrys_test", "AAYU_SECRET_GITHUB_READ": "ghp_test", "AAYU_SECRET_GROWTHBOOK": "secret_readonly_test"}

// TestBaselinesGolden is the WP5.2 acceptance (GrowthBook added in WP7.2):
// recorded PostHog, Sentry, GitHub, and GrowthBook exchanges produce exactly the baseline tuples in
// testdata/baselines.golden.json, which were written by hand from the
// fixtures, not generated from this code.
func TestBaselinesGolden(t *testing.T) {
	b, err := os.ReadFile("testdata/baselines.golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden map[string]json.RawMessage
	if err := json.Unmarshal(b, &golden); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		source, auth string
		queries      []integrations.Query
		names        []string
	}{
		{"posthog", "Bearer phx_test", []integrations.Query{{Metric: "users", Filter: "event = 'signed_up'", Window: sept}}, []string{"posthog users"}},
		{"sentry", "Bearer sntrys_test", []integrations.Query{
			{Metric: "crash_free_sessions", Filter: "environment:production", Window: sept},
			{Metric: "errors", Window: sept},
		}, []string{"sentry crash_free_sessions", "sentry errors"}},
		{"github", "Bearer ghp_test", []integrations.Query{{Metric: "prs_merged", Filter: "base:dev", Window: sept}}, []string{"github prs_merged"}},
		// The control variation (first in the experiment) is read with the frequentist analysis.
		{"growthbook", "Bearer secret_readonly_test", []integrations.Query{{Metric: "conversion_rate", Filter: "checkout-v2 met_signup", Window: sept}}, []string{"growthbook conversion_rate"}},
		// WP7.2b: the cloud recordings pin each request's own bearer (or SigV4
		// signature); the token exchanges run against fixture.Clouds.
		{"azure", "", []integrations.Query{{Metric: "users", Filter: "Name == 'signed_up'", Window: sept}}, []string{"azure users"}},
		{"aws", "", []integrations.Query{{Metric: "events", Filter: "event_type = 'signed_up'", Window: sept}}, []string{"aws events"}},
		{"gcp", "", []integrations.Query{{Metric: "users", Filter: "event_name = 'sign_up'", Window: sept}}, []string{"gcp users"}},
	}
	seen := 0
	for _, c := range cases {
		t.Run(c.source, func(t *testing.T) {
			r := &fixture.Replayer{T: t, Rec: fixture.Load(t, "testdata/"+c.source+".json"), AuthHeader: "Authorization", AuthValue: c.auth}
			d := deps(t, r, keys, configured...)
			if c.auth == "" {
				r.AuthHeader = ""
				d = cloudDeps(t, r, "key")
			}
			for i, q := range c.queries {
				got, err := Baseline(context.Background(), d, c.source, q)
				if err != nil {
					t.Fatal(err)
				}
				want, ok := golden[c.names[i]]
				if !ok {
					t.Fatalf("no golden tuple %q", c.names[i])
				}
				gotJSON, _ := json.Marshal(got)
				if !sameJSON(t, gotJSON, want) {
					t.Errorf("%s:\ngot  %s\nwant %s", c.names[i], gotJSON, bytes.Join(bytes.Fields(want), []byte(" ")))
				}
				seen++
			}
			r.Done()
		})
	}
	if seen != len(golden) {
		t.Errorf("checked %d of %d golden tuples", seen, len(golden))
	}
}

func sameJSON(t *testing.T, a, b []byte) bool {
	var x, y map[string]any
	if err := json.Unmarshal(a, &x); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &y); err != nil {
		t.Fatal(err)
	}
	xa, _ := json.Marshal(x)
	ya, _ := json.Marshal(y)
	return bytes.Equal(xa, ya)
}

// refuse is a transport that fails the test on any request.
type refuse struct {
	t *testing.T
	n int
}

func (r *refuse) RoundTrip(req *http.Request) (*http.Response, error) {
	r.n++
	r.t.Errorf("request sent: %s %s", req.Method, req.URL)
	return nil, errors.New("no requests expected")
}

// TestMissingAdapterNamesTheKey: without [integrations.<name>] (or with a
// required field or the secret missing) nothing is sent, and the error is
// ErrNotConfigured (exit 5) naming the key to set.
func TestMissingAdapterNamesTheKey(t *testing.T) {
	q := integrations.Query{Metric: "errors", Window: sept}
	for _, tt := range []struct {
		name, source string
		sets         []string
		env          map[string]string
		want         []string
	}{
		{"posthog absent", "posthog", nil, keys, []string{`[integrations.posthog] is not set`, `secret = "posthog"`, `scope = "read"`, `project = "<the PostHog project ID>"`, "aayu auth set-secret posthog"}},
		{"sentry absent", "sentry", nil, keys, []string{`[integrations.sentry] is not set`, `org = "<the Sentry organization slug>"`, `project = "<the numeric Sentry project ID>"`}},
		{"github absent", "github", nil, keys, []string{`[integrations.github] is not set`, `secret = "github"`}},
		{"sentry without org", "sentry", []string{"integrations.sentry.secret=sentry", "integrations.sentry.scope=read", "integrations.sentry.project=4507"}, keys, []string{"[integrations.sentry] org is not set"}},
		{"secret unset", "sentry", configured, nil, []string{`[integrations.sentry] secret "sentry" has no value`, "AAYU_SECRET_SENTRY"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rt := &refuse{t: t}
			_, err := Baseline(context.Background(), deps(t, rt, tt.env, tt.sets...), tt.source, q)
			if !errors.Is(err, integrations.ErrNotConfigured) {
				t.Fatalf("err = %v, want ErrNotConfigured", err)
			}
			for _, w := range tt.want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("error %q lacks %q", err, w)
				}
			}
		})
	}
}

// TestSentryOpens (Decision #449, phase P1): Sentry opens the configured
// project's client for the Production Engineer's live reads, sending nothing,
// and an unconfigured Sentry names what to set.
func TestSentryOpens(t *testing.T) {
	rt := &refuse{t: t}
	c, err := Sentry(deps(t, rt, keys, configured...))
	if err != nil || c.Project() != "4507" || rt.n != 0 {
		t.Fatalf("Sentry = %v, %v after %d requests; want the client for project 4507 and no request", c, err, rt.n)
	}
	if _, err := Sentry(deps(t, rt, keys)); !errors.Is(err, integrations.ErrNotConfigured) || !strings.Contains(err.Error(), "[integrations.sentry] is not set") {
		t.Errorf("unconfigured: err %v; want ErrNotConfigured naming [integrations.sentry]", err)
	}
}

// TestWriteScopedKeyRefusedBeforeAnyCall: scope = "write" is refused by the
// WP1.3 check with zero requests, for baselines and for doctor's check alike.
func TestWriteScopedKeyRefusedBeforeAnyCall(t *testing.T) {
	for _, source := range []string{"posthog", "sentry", "github"} {
		t.Run(source, func(t *testing.T) {
			sets := append([]string{}, configured...)
			sets = append(sets, "integrations."+source+".scope=write")
			rt := &refuse{t: t}
			d := deps(t, rt, keys, sets...)
			_, err := Baseline(context.Background(), d, source, integrations.Query{Metric: "users", Window: sept})
			if !errors.Is(err, secrets.ErrWriteScoped) {
				t.Fatalf("Baseline err = %v, want ErrWriteScoped", err)
			}
			if _, err := Check(context.Background(), d, source); !errors.Is(err, secrets.ErrWriteScoped) {
				t.Fatalf("Check err = %v, want ErrWriteScoped", err)
			}
			if rt.n != 0 {
				t.Fatalf("%d requests sent", rt.n)
			}
		})
	}
}

// TestProviderReportedWriteScopeRefused: a key configured as read that the
// provider reports as write-capable is refused after the scope request and
// before the data request.
func TestProviderReportedWriteScopeRefused(t *testing.T) {
	for _, tt := range []struct {
		source, url, header string
		response            string
		write               string
		q                   integrations.Query
	}{
		{"posthog", "https://us.posthog.com/api/personal_api_keys/@current/", "", `{"label":"all access","scopes":["*"]}`, "*", integrations.Query{Metric: "events", Window: sept}},
		{"sentry", "https://sentry.io/api/0/", "", `{"auth":{"scopes":["event:read","project:releases","project:read"]},"user":{"username":"ci"}}`, "project:releases", integrations.Query{Metric: "errors", Window: sept}},
		{"github", "https://api.github.com/user", "repo, read:org", `{"login":"rahulnakmol","type":"User"}`, "repo", integrations.Query{Metric: "issues_opened", Window: sept}},
	} {
		t.Run(tt.source, func(t *testing.T) {
			in := fixture.Interaction{Method: "GET", URL: tt.url, Status: 200, Response: json.RawMessage(tt.response)}
			if tt.header != "" {
				in.Header = map[string]string{"X-OAuth-Scopes": tt.header}
			}
			r := &fixture.Replayer{T: t, Rec: &fixture.Recording{Interactions: []fixture.Interaction{in}}}
			d := deps(t, r, keys, configured...)
			_, err := Baseline(context.Background(), d, tt.source, tt.q)
			if !errors.Is(err, secrets.ErrWriteScoped) || !strings.Contains(err.Error(), tt.write) {
				t.Fatalf("err = %v, want ErrWriteScoped naming %s", err, tt.write)
			}
			r.Done()
			if r.Requests() != 1 {
				t.Fatalf("%d requests; the data request must not be sent", r.Requests())
			}
		})
	}
}
