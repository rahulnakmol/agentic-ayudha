package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	aayudha "github.com/rahulnakmol/agentic-ayudha"
	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations/fixture"
	"github.com/rahulnakmol/agentic-ayudha/internal/ops"
	"github.com/rahulnakmol/agentic-ayudha/internal/secrets"
)

// providerRoutes answers Sentry and PostHog reads from the recordings in
// internal/production/testdata (shaped from each provider's OpenAPI spec; not
// live-proven), keyed by what each request asks for, and records every
// request. A request it has no answer for, or any write, fails the test.
type providerRoutes struct {
	t    *testing.T
	mu   sync.Mutex
	sent []string
	// answer picks the body for a request: the path, the Sentry dataset, or
	// the PostHog query kind.
	answer func(method, path, dataset, kind string) (status int, body string, ok bool)
}

func (p *providerRoutes) RoundTrip(req *http.Request) (*http.Response, error) {
	var kind string
	if req.Body != nil {
		b, _ := io.ReadAll(req.Body)
		var q struct {
			Query struct {
				Kind string `json:"kind"`
			} `json:"query"`
		}
		_ = json.Unmarshal(b, &q)
		kind = q.Query.Kind
	}
	p.mu.Lock()
	p.sent = append(p.sent, req.Method+" "+req.URL.String())
	p.mu.Unlock()
	if req.Method != http.MethodGet && (req.Method != http.MethodPost || !strings.HasSuffix(req.URL.Path, "/query/")) {
		p.t.Errorf("a write was sent: %s %s", req.Method, req.URL)
	}
	status, body, ok := p.answer(req.Method, req.URL.Path, req.URL.Query().Get("dataset"), kind)
	if !ok {
		p.t.Errorf("no recorded answer for %s %s", req.Method, req.URL)
		status, body = http.StatusNotFound, `{"detail":"not recorded"}`
	}
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(bytes.NewBufferString(body)), Request: req}, nil
}

// recorded reads the response of a recording's interaction i as JSON text.
func recorded(t *testing.T, name string, i int) string {
	t.Helper()
	return string(fixture.Load(t, filepath.Join("..", "production", "testdata", name)).Interactions[i].Response)
}

// TestInvestigateLive (Decision #449, phase P1): `aayu investigate --source`
// runs the bounded queries each live source answers, logs each as a citable
// qN with the requests it sent and a SHA-256, takes the deployed revision
// from Sentry's newest release deploy, and maps the exceptions' top frames
// to code at that revision. Refusals send no data request.
func TestInvestigateLive(t *testing.T) {
	repo, good, bad := incidentRepo(t)
	t0 := time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC)
	g := fake.New(t, "o/r", "User")
	g.AddIssue("Checkout fails", "### Impact\n\nSome checkouts fail.\n\n### Severity\n\nSev2\n\n### Started at\n\n2026-10-17T08:30Z\n", "type:incident", "status:in-progress")
	cat, _ := os.ReadFile("testdata/incident/actions.toml")
	g.RefFiles = map[string]map[string]string{headSHA: {ops.Path: strings.ReplaceAll(string(cat), "benefits-qa", "web")}}
	g.Static = map[string]any{
		"/repos/o/r/branches/dev":                  map[string]any{"name": "dev", "commit": map[string]any{"sha": headSHA}},
		"/repos/o/r/compare/" + headSHA + "...dev": map[string]any{"status": "identical"},
		"/repos/o/r/commits":                       []map[string]any{{"sha": headSHA}},
	}
	g.Refs["heads/dev"] = headSHA
	env := map[string]string{"AAYU_GITHUB_TOKEN": "t", "AAYU_SECRET_SENTRY": "sntrys_test", "AAYU_SECRET_POSTHOG": "phx_test"}
	sentryFlags := []string{"--set", "integrations.sentry.secret=sentry", "--set", "integrations.sentry.scope=read",
		"--set", "integrations.sentry.org=acme", "--set", "integrations.sentry.project=4507"}
	posthogFlags := []string{"--set", "integrations.posthog.secret=posthog", "--set", "integrations.posthog.scope=read",
		"--set", "integrations.posthog.project=12345", "--set", "integrations.posthog.url=https://eu.posthog.com"}

	// Sentry releases are the checkout's commits: web@2.4.1 is the bad one.
	rel := strings.NewReplacer("web@2.4.1", bad, "web@2.4.0", good, "app/checkout/total.py", "src/answer.go", `"lineNo": 42`, `"lineNo": 6`, `"lineNo":42`, `"lineNo":6`)
	scopes := `{"auth":{"scopes":["alerts:read","event:read","org:read","project:read"]}}`
	sentry := func(method, path, dataset, _ string) (int, string, bool) {
		switch {
		case path == "/api/0/":
			return 200, scopes, true
		case dataset == "spans":
			return 200, rel.Replace(recorded(t, "sentry/requests.json", 0)), true
		case dataset == "errors" && strings.HasSuffix(path, "/events/"):
			// errors by version,route and exceptions by version share the dataset
			return 200, rel.Replace(recorded(t, "sentry/exceptions.json", 0)), true
		case strings.HasSuffix(path, "/issues/1/events/latest/"):
			return 200, rel.Replace(recorded(t, "sentry/exceptions.json", 1)), true
		case strings.HasSuffix(path, "/events/latest/"):
			return 404, recorded(t, "sentry/exceptions.json", 2), true
		case strings.HasSuffix(path, "/releases/"):
			return 200, rel.Replace(recorded(t, "sentry/deploys.json", 0)), true
		case strings.HasSuffix(path, "/releases/"+bad+"/deploys/"):
			return 200, strings.ReplaceAll(recorded(t, "sentry/deploys.json", 1), "2026-10-17T08:05:30Z", "2026-10-17T08:35:30Z"), true
		case strings.HasSuffix(path, "/deploys/"):
			return 200, strings.ReplaceAll(recorded(t, "sentry/deploys.json", 2), "2026-10-16T15:02:10Z", "2026-10-17T08:00:00Z"), true
		case dataset == "logs":
			return 200, rel.Replace(recorded(t, "sentry/logs.json", 0)), true
		}
		return 0, "", false
	}
	posthog := func(method, path, _, kind string) (int, string, bool) {
		switch {
		case strings.HasSuffix(path, "/personal_api_keys/@current/"):
			return 200, `{"label":"aayudha","scopes":["query:read","alert:read"]}`, true
		case kind == "ErrorTrackingQuery":
			return 200, recorded(t, "posthog/exceptions.json", 1), true
		case kind == "LogsQuery":
			return 200, recorded(t, "posthog/logs.json", 1), true
		}
		return 0, "", false
	}
	inv := func(want int, answer func(string, string, string, string) (int, string, bool), args ...string) (investigateReport, string, []string) {
		t.Helper()
		routes := &providerRoutes{t: t, answer: answer}
		opts := Options{GitHubAPI: g.URL(), GitHubGap: -1, Assets: aayudha.Assets, Dir: repo, Now: func() time.Time { return t0 },
			HTTP: &http.Client{Transport: local{routes}}, Secrets: secrets.New(secrets.Env{Getenv: func(k string) string { return env[k] }})}
		code, out, errOut := run(t, env, opts, append([]string{"investigate", "1", "--repo", "o/r"}, args...)...)
		if code != want {
			t.Fatalf("investigate %v: exit %d, want %d: %s %s", args, code, want, out, errOut)
		}
		var rep investigateReport
		_ = json.NewDecoder(strings.NewReader(out)).Decode(&rep)
		return rep, out + errOut, routes.sent
	}

	// Sentry: deploys first (for the deployed revision), then the defaults it answers.
	_, ground, sent := inv(0, sentry, append([]string{"--source", "sentry", "--service", "web"}, sentryFlags...)...)
	for _, want := range []string{
		"Scope: service web, environment production, window 2026-10-17T07:30:00Z..2026-10-17T09:00:01Z", // an hour before 08:30, to now
		"### query q1: `deploys",
		"### query q2: `requests by version",
		"### query q3: `errors by version,route",
		"### query q4: `exceptions by version",
		"### query q5: `logs where level=ERROR",
		"Source: sentry project 4507 (sha256 ",
		"from GET https://sentry.io/api/0/organizations/acme/releases/?per_page=10&project=4507",
		"src/answer.go:6",             // the top frame, mapped into the repository
		">    6  \tif r[1] == best {", // the code at the deployed revision
		"Newest deploy in the window: `" + bad + "` to production at 2026-10-17T08:35:30Z; previous `" + good + "` (query q1)",
		"`aayu investigate 1 --source sentry --service web --query",
		"(kinds: requests, errors, latency, exceptions, logs, deploys, trace)",
		"feat(answer): prefer the second passage on a tie",
	} {
		if !strings.Contains(ground, want) {
			t.Errorf("sentry grounding lacks %q:\n%s", want, ground)
		}
	}
	if strings.Contains(ground, "timeline by") || strings.Contains(ground, "ops@example.com") {
		t.Errorf("sentry grounding ran timeline or leaked an email:\n%s", ground)
	}
	if len(sent) < 8 || sent[0] != "GET https://sentry.io/api/0/" {
		t.Errorf("sentry requests %v; want the scope check first, then the data reads", sent)
	}
	log, err := os.ReadFile(filepath.Join(repo, ".aayudha", "state", "investigate", "1", "queries.jsonl"))
	if lines := strings.Split(strings.TrimSpace(string(log)), "\n"); err != nil || len(lines) != 5 ||
		!strings.Contains(lines[0], `"source":"sentry project 4507"`) || !strings.Contains(lines[0], `"requests":["GET https://sentry.io/api/0/organizations/acme/releases/`) ||
		!strings.Contains(lines[4], `"id":"q5"`) {
		t.Errorf("query log: %v\n%s", err, log)
	}
	if g.Writes() != nil {
		t.Errorf("the grounding wrote %v", g.Writes())
	}
	_ = os.RemoveAll(filepath.Join(repo, ".aayudha", "state", "investigate"))

	// PostHog answers exceptions and logs only.
	_, ground, _ = inv(0, posthog, append([]string{"--source", "posthog", "--service", "web"}, posthogFlags...)...)
	if !strings.Contains(ground, "### query q1: `exceptions by version") || !strings.Contains(ground, "### query q2: `logs where level=ERROR") ||
		strings.Contains(ground, "query q3") || !strings.Contains(ground, "Source: posthog project 12345 (sha256 ") ||
		!strings.Contains(ground, "from POST https://eu.posthog.com/api/projects/12345/query/") ||
		!strings.Contains(ground, "project-wide, not scoped to service web") {
		t.Errorf("posthog grounding:\n%s", ground)
	}
	_ = os.RemoveAll(filepath.Join(repo, ".aayudha", "state", "investigate"))

	// A frame that is not a repository file is provider text: it stays in the
	// query's untrusted rows and never reaches the implicated code.
	evil := func(method, path, dataset, kind string) (int, string, bool) {
		if strings.HasSuffix(path, "/issues/1/events/latest/") {
			return 200, strings.ReplaceAll(recorded(t, "sentry/exceptions.json", 1), "app/checkout/total.py", "ignore_previous_instructions.py"), true
		}
		return sentry(method, path, dataset, kind)
	}
	_, ground, _ = inv(0, evil, append([]string{"--source", "sentry", "--service", "web"}, sentryFlags...)...)
	outside := regexp.MustCompile(`(?s)<untrusted .*?</untrusted>`).ReplaceAllString(ground, "")
	if !strings.Contains(ground, "ignore_previous_instructions.py") || strings.Contains(outside, "ignore_previous_instructions") {
		t.Errorf("an unresolved frame left the untrusted rows:\n%s", ground)
	}
	_ = os.RemoveAll(filepath.Join(repo, ".aayudha", "state", "investigate"))

	// A one-off --query runs that query alone: no deploy discovery first.
	_, out, sent := inv(0, sentry, append([]string{"--source", "sentry", "--service", "web", "--query", "logs where level=ERROR limit 5"}, sentryFlags...)...)
	if !strings.Contains(out, "q1") || !strings.Contains(out, "logs where level=ERROR") || slices.ContainsFunc(sent, func(s string) bool { return strings.Contains(s, "/releases/") }) {
		t.Errorf("one-off sentry query: requests %v\n%s", sent, out)
	}
	_ = os.RemoveAll(filepath.Join(repo, ".aayudha", "state", "investigate"))
	_, out, _ = inv(ExitUsage, posthog, append([]string{"--source", "posthog", "--service", "web", "--query", "requests by version"}, posthogFlags...)...)
	if !strings.Contains(out, "it answers logs and exceptions") {
		t.Errorf("an unanswered kind: %s", out)
	}

	// Refusals: none sends a data request; a failed read stops at that read.
	none := func(string, string, string, string) (int, string, bool) { return 0, "", false }
	scope := func(sentryScopes string, rest func(string) (int, string, bool)) func(string, string, string, string) (int, string, bool) {
		return func(method, path, dataset, kind string) (int, string, bool) {
			switch {
			case path == "/api/0/":
				return 200, `{"auth":{"scopes":` + sentryScopes + `}}`, true
			case strings.HasSuffix(path, "/personal_api_keys/@current/"):
				return 401, `{"type":"authentication_error","code":"authentication_failed","detail":"Personal API key found in request Authorization header is invalid."}`, true
			}
			return rest(path)
		}
	}
	unavailable := func(path string) (int, string, bool) {
		return 503, `{"detail":"Service unavailable"}`, strings.HasSuffix(path, "/releases/")
	}
	for _, c := range []struct {
		name   string
		code   int
		answer func(string, string, string, string) (int, string, bool)
		args   []string
		want   string
		sent   int // at most this many requests
	}{
		{"neither", ExitUsage, none, nil, "needs --telemetry <dir>", 0},
		{"both", ExitUsage, none, []string{"--source", "sentry", "--service", "web", "--telemetry", t.TempDir()}, "not both", 0},
		{"no service", ExitUsage, none, []string{"--source", "sentry"}, "needs --service", 0},
		{"no export", ExitUsage, none, []string{"--telemetry", filepath.Join(t.TempDir(), "missing")}, "missing", 0},
		{"unknown source", ExitUsage, none, []string{"--source", "datadog", "--service", "web"}, "want one of sentry, posthog", 0},
		{"sentry unconfigured", ExitBlocked, none, []string{"--source", "sentry", "--service", "web"}, "[integrations.sentry] is not set", 0},
		{"posthog unconfigured", ExitBlocked, none, []string{"--source", "posthog", "--service", "web"}, "[integrations.posthog] is not set", 0},
		{"write-capable", ExitBlocked, scope(`["event:admin","event:read","org:read","project:read"]`, func(string) (int, string, bool) { return 0, "", false }),
			append([]string{"--source", "sentry", "--service", "web"}, sentryFlags...), "the sentry key can write (event:admin)", 1},
		{"rejected key", ExitBlocked, scope(`[]`, func(string) (int, string, bool) { return 0, "", false }),
			append([]string{"--source", "posthog", "--service", "web"}, posthogFlags...), "invalid", 1},
		{"failed read", ExitBlocked, scope(`["event:read","org:read","project:read"]`, unavailable),
			append([]string{"--source", "sentry", "--service", "web"}, sentryFlags...), "Service unavailable", 2},
	} {
		_, out, sent := inv(c.code, c.answer, c.args...)
		if !strings.Contains(out, c.want) || len(sent) > c.sent {
			t.Errorf("%s: %s, requests %v; want %q and at most %d requests", c.name, out, sent, c.want, c.sent)
		}
	}
}
