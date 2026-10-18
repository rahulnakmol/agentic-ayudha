package production

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations/fixture"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations/posthog"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations/sentry"
)

// TestSentrySource (Decision #449, phase P1) replays Sentry answers shaped
// from its OpenAPI spec through the real adapter and checks that each kind
// comes back with the export's columns, redacted, bounded, and with the
// requests it made and a digest over the answers.
func TestSentrySource(t *testing.T) {
	from := time.Date(2026, 10, 17, 8, 0, 0, 0, time.UTC)
	to := from.Add(time.Hour)
	resolve := func(file string) (string, bool) {
		if file == "app/checkout/total.py" {
			return "src/app/checkout/total.py", true
		}
		return "", false
	}
	for _, tt := range []struct {
		file, query string
		cols        string
		rows        []string // each row joined by " | "
		matched     int
		truncated   bool
		requests    int
	}{
		{"requests", "requests where service=web limit 2", "version requests errors error_rate p50_ms p95_ms",
			[]string{"web@2.4.1 | 1840 | 92 | 5.0% | 120.5 | 812.5", "web@2.4.0 | 1215 | 6 | 0.5% | 98.0 | 143.0"}, 3, true, 1},
		{"latency", "latency by version,route where service=web environment=production", "version route requests p50_ms p95_ms p99_ms",
			[]string{"web@2.4.1 | POST /api/checkout/ | 1840 | 120.5 | 812.5 | 1630.0"}, 1, false, 1},
		{"errors", "errors where service=web version=web@2.4", "version route errors first last",
			[]string{"web@2.4.1 | POST /api/checkout/ | 57 | 2026-10-17T08:12:03Z | 2026-10-17T08:58:40Z"}, 1, false, 1},
		{"exceptions", "exceptions where service=web", "version type message top_frame count first last",
			[]string{"web@2.4.1 | ZeroDivisionError | ZeroDivisionError: division by zero, contact [email] | src/app/checkout/total.py:42 | 57 | 2026-10-17T08:12:03Z | 2026-10-17T08:58:40Z",
				"web@2.4.1 | TimeoutError | TimeoutError: payment provider timeout after 5000ms |  | 12 | 2026-10-17T08:20:00Z | 2026-10-17T08:41:12Z"}, 2, false, 3},
		{"logs", "logs where service=web level=ERROR", "time level msg version trace_id attributes",
			[]string{"2026-10-17T08:41:12Z | error | payment provider timeout after 5000ms | web@2.4.1 | da6caabcd90e45fdb81f6655824a5f88 | ",
				"2026-10-17T08:39:57Z | error | division by zero in compute_total | web@2.4.1 | da6caabcd90e45fdb81f6655824a5f88 | "}, 2, false, 1},
		{"deploys", "deploys where service=web environment=production", "time service environment revision previous actor",
			[]string{"2026-10-17T08:05:30Z | web | production | web@2.4.1 | web@2.4.0 | "}, 1, false, 3},
		{"trace", "trace where service=web trace=da6caabcd90e45fdb81f6655824a5f88", "time kind name version status duration_ms detail",
			[]string{"2026-10-17T08:05:00Z | span | http.server POST /api/0/projects/{org}/{proj}/events/{event_id}/attachments/ |  |  | 250.0 | ZeroDivisionError: division by zero",
				"2026-10-17T08:05:00.05Z | span | db SELECT * FROM carts WHERE id = %s |  |  | 180.0 | "}, 2, false, 1},
	} {
		t.Run(tt.file, func(t *testing.T) {
			src, r := sentrySource(t, tt.file)
			q, err := ParseQuery(tt.query, from, to)
			if err != nil {
				t.Fatal(err)
			}
			res, pv, err := src.Run(context.Background(), q, resolve)
			r.Done()
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.Join(res.Columns, " "); got != tt.cols {
				t.Errorf("columns %q; want %q", got, tt.cols)
			}
			var rows []string
			for _, row := range res.Rows {
				rows = append(rows, strings.Join(row, " | "))
			}
			if strings.Join(rows, "\n") != strings.Join(tt.rows, "\n") {
				t.Errorf("rows:\n%s\nwant:\n%s", strings.Join(rows, "\n"), strings.Join(tt.rows, "\n"))
			}
			if res.Matched != tt.matched || res.Truncated != tt.truncated {
				t.Errorf("matched %d truncated %v; want %d %v", res.Matched, res.Truncated, tt.matched, tt.truncated)
			}
			if len(pv.Requests) != tt.requests || len(pv.Digest) != 64 || pv.Source != "sentry project 4507" {
				t.Errorf("provenance %+v; want %d requests, a SHA-256, and the project", pv, tt.requests)
			}
			for _, req := range pv.Requests {
				if !strings.HasPrefix(req, "GET https://sentry.io/api/0/organizations/acme/") || strings.Contains(req, "sntrys_") {
					t.Errorf("request %q is not a GET under the organization, or carries the key", req)
				}
			}
			// The same answers give the same digest; it is what a cited query is checked against.
			src2, r2 := sentrySource(t, tt.file)
			_, pv2, _ := src2.Run(context.Background(), q, resolve)
			r2.Done()
			if pv2.Digest != pv.Digest {
				t.Errorf("digest changed between identical runs: %s, %s", pv.Digest, pv2.Digest)
			}
		})
	}

	t.Run("refused read", func(t *testing.T) {
		src, r := sentrySource(t, "forbidden")
		q, _ := ParseQuery("requests where service=web", from, to)
		_, pv, err := src.Run(context.Background(), q, nil)
		r.Done()
		var he *integrations.HTTPError
		if !errors.As(err, &he) || he.Status != http.StatusForbidden || len(pv.Requests) != 1 {
			t.Fatalf("err %v, provenance %+v; want the 403 and the request it made", err, pv)
		}
	})
	t.Run("kinds and fields Sentry cannot answer", func(t *testing.T) {
		src := Sentry{API: nil}
		for _, text := range []string{"timeline where service=web", "logs where service=web route=/x", "requests by exception where service=web"} {
			q, _ := ParseQuery(text, from, to)
			src.API = noCalls{t}
			if _, _, err := src.Run(context.Background(), q, nil); err == nil || !errors.Is(err, errSource) {
				t.Errorf("%s: err %v; want the Sentry source to refuse it before any request", text, err)
			}
		}
	})
	t.Run("unbounded query", func(t *testing.T) {
		q, _ := ParseQuery("requests where service=web", from, from.Add(8*24*time.Hour))
		if _, _, err := (Sentry{API: noCalls{t}}).Run(context.Background(), q, nil); err == nil {
			t.Error("an 8-day window ran; want the 7-day bound to refuse it")
		}
	})
}

// sentrySource is a Sentry source on the real adapter, answered from a recording.
func sentrySource(t *testing.T, name string) (Sentry, *fixture.Replayer) {
	t.Helper()
	r := &fixture.Replayer{T: t, Rec: fixture.Load(t, filepath.Join("testdata", "sentry", name+".json")), AuthHeader: "Authorization", AuthValue: "Bearer sntrys_test"}
	c, err := sentry.New(integrations.Conn{Name: "sentry", Key: "sntrys_test", Integration: config.Integration{Org: "acme", Project: "4507"}}, r, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	return Sentry{API: c}, r
}

// noCalls fails the test on any request.
type noCalls struct{ t *testing.T }

func (n noCalls) Get(context.Context, string, url.Values) ([]byte, string, error) {
	n.t.Error("a request was made")
	return nil, "", errors.New("no calls")
}
func (noCalls) Project() string { return "4507" }

// answer answers every request with one body: a provider that changed its
// shape, or that sends something else.
type answer struct{ body string }

func (a answer) Get(_ context.Context, path string, _ url.Values) ([]byte, string, error) {
	return []byte(a.body), "GET " + path, nil
}
func (answer) Project() string { return "4507" }

// TestSentrySourceUntrusted: Sentry's answer is data. An answer of the wrong
// shape is an error naming the read, never rows, and a filter value with a
// space is quoted so it cannot add a filter of its own.
func TestSentrySourceUntrusted(t *testing.T) {
	from := time.Date(2026, 10, 17, 8, 0, 0, 0, time.UTC)
	for _, kind := range []string{"requests", "logs", "deploys", "trace where trace=abc"} {
		q, _ := ParseQuery(kind+" where service=web", from, from.Add(time.Hour))
		if _, _, err := (Sentry{API: answer{`"not the shape"`}}).Run(context.Background(), q, nil); err == nil || !strings.Contains(err.Error(), "undecodable") {
			t.Errorf("%s: err %v; want an undecodable answer", kind, err)
		}
	}
	var asked url.Values
	q, _ := ParseQuery(`errors where service=web route=GET_/cart`, from, from.Add(time.Hour))
	q.Where["route"] = `GET /cart" OR release:x`
	_, _, _ = (Sentry{API: recorder{&asked}}).Run(context.Background(), q, nil)
	if got := asked.Get("query"); got != `transaction:"GET /cart\" OR release:x"` {
		t.Errorf("query %q; want the route quoted as one value", got)
	}
}

// recorder keeps the values of the last request and answers with no rows.
type recorder struct{ v *url.Values }

func (r recorder) Get(_ context.Context, _ string, v url.Values) ([]byte, string, error) {
	*r.v = v
	return []byte(`{"data":[]}`), "GET", nil
}
func (recorder) Project() string { return "4507" }

// TestSentryTopFrameFallback: with no frame inside the repository, the top
// frame is the innermost in-app frame of the latest event's exception, never
// a library frame, and an event with no stack trace leaves it empty.
func TestSentryTopFrameFallback(t *testing.T) {
	from := time.Date(2026, 10, 17, 8, 0, 0, 0, time.UTC)
	q, _ := ParseQuery("exceptions where service=web", from, from.Add(time.Hour))
	api := routes{
		"/events/": `{"data":[{"release":"1.2.3","error.type":"TypeError","title":"x is undefined","issue.id":"7","count()":4},` +
			`{"release":"1.2.3","error.type":"Error","title":"boom","issue.id":"8","count()":1}]}`,
		"/issues/7/events/latest/": `{"entries":[{"type":"breadcrumbs","data":{"values":[]}},{"type":"exception","data":{"values":[{"stacktrace":null},` +
			`{"stacktrace":{"frames":[{"filename":"app/cart.js","lineNo":12,"inApp":true},{"filename":"app/total.js","lineNo":40,"inApp":true},{"filename":"node_modules/lib.js","lineNo":3,"inApp":false}]}}]}}]}`,
		"/issues/8/events/latest/": `"not an event"`,
	}
	r, _, err := Sentry{API: api}.Run(context.Background(), q, func(string) (string, bool) { return "", false })
	if err != nil || len(r.Rows) != 2 {
		t.Fatalf("rows %v, err %v; want two rows", r.Rows, err)
	}
	frame := slices.Index(r.Columns, "top_frame")
	if got := r.Rows[0][frame]; got != "app/total.js:40" {
		t.Errorf("top frame %q; want the innermost in-app frame app/total.js:40", got)
	}
	if got := r.Rows[1][frame]; got != "" {
		t.Errorf("top frame %q from an undecodable event; want it empty", got)
	}
}

// routes answers each path with its body.
type routes map[string]string

func (r routes) Get(_ context.Context, path string, _ url.Values) ([]byte, string, error) {
	b, ok := r[path]
	if !ok {
		return nil, "GET " + path, errors.New("no route")
	}
	return []byte(b), "GET " + path, nil
}
func (routes) Project() string { return "4507" }

// TestPostHogSource (Decision #449, phase P1): PostHog answers logs and
// exceptions with the export's columns, through the real adapter, after it
// checks the key reads only.
func TestPostHogSource(t *testing.T) {
	from := time.Date(2026, 10, 17, 8, 0, 0, 0, time.UTC)
	for _, tt := range []struct {
		file, query string
		rows        []string
		truncated   bool
	}{
		{"exceptions", "exceptions where service=web", []string{
			" | TypeError | Cannot read properties of undefined (reading 'id') for [email] |  | 312 | 2026-10-15T09:12:44Z | 2026-10-17T08:41:12Z",
			" | ZeroDivisionError | division by zero |  | 58 | 2026-10-16T21:03:10Z | 2026-10-17T08:39:57Z"}, false},
		{"logs", "logs where service=web level=ERROR limit 1", []string{
			"2026-10-17T08:41:12Z | error | payment provider timeout after 5000ms | web@2.4.1 | da6caabcd90e45fdb81f6655824a5f88 | "}, true},
	} {
		t.Run(tt.file, func(t *testing.T) {
			r := &fixture.Replayer{T: t, Rec: fixture.Load(t, filepath.Join("testdata", "posthog", tt.file+".json")), AuthHeader: "Authorization", AuthValue: "Bearer phx_test"}
			c, err := posthog.New(integrations.Conn{Name: "posthog", Key: "phx_test", Integration: config.Integration{URL: "https://eu.posthog.com", Project: "12345"}}, r, time.Now)
			if err != nil {
				t.Fatal(err)
			}
			q, _ := ParseQuery(tt.query, from, from.Add(time.Hour))
			res, pv, err := PostHog{API: c, Project: "12345"}.Run(context.Background(), q)
			r.Done()
			if err != nil {
				t.Fatal(err)
			}
			var rows []string
			for _, row := range res.Rows {
				rows = append(rows, strings.Join(row, " | "))
			}
			if strings.Join(rows, "\n") != strings.Join(tt.rows, "\n") || res.Truncated != tt.truncated {
				t.Errorf("rows (truncated %v):\n%s\nwant (truncated %v):\n%s", res.Truncated, strings.Join(rows, "\n"), tt.truncated, strings.Join(tt.rows, "\n"))
			}
			if len(pv.Requests) != 1 || !strings.HasPrefix(pv.Requests[0], "POST https://eu.posthog.com/api/projects/12345/query/ ") || len(pv.Digest) != 64 {
				t.Errorf("provenance %+v; want the query request and a SHA-256", pv)
			}
		})
	}
	t.Run("refused before any request", func(t *testing.T) {
		for _, text := range []string{"requests where service=web", "exceptions by route where service=web", "exceptions where service=web version=2.4", "logs where service=web route=/x", "logs where service=web level=loud"} {
			q, _ := ParseQuery(text, from, from.Add(time.Hour))
			if _, _, err := (PostHog{API: noQuery{t}}).Run(context.Background(), q); !errors.Is(err, errSource) {
				t.Errorf("%s: err %v; want PostHog to refuse it", text, err)
			}
		}
	})
}

type noQuery struct{ t *testing.T }

func (n noQuery) Query(context.Context, string, map[string]any) ([]byte, string, error) {
	n.t.Error("a request was made")
	return nil, "", errors.New("no calls")
}

// TestPostHogSourceUntrusted: PostHog's answer is data. An answer of the
// wrong shape is an error naming the read, never rows; a failed read comes
// back as the provider's error; an unbounded query sends nothing.
func TestPostHogSourceUntrusted(t *testing.T) {
	from := time.Date(2026, 10, 17, 8, 0, 0, 0, time.UTC)
	for _, kind := range []string{"logs", "exceptions"} {
		q, _ := ParseQuery(kind+" where service=web", from, from.Add(time.Hour))
		if _, _, err := (PostHog{API: phAnswer{body: `"not the shape"`}, Project: "12345"}).Run(context.Background(), q); err == nil || !strings.Contains(err.Error(), "undecodable") {
			t.Errorf("%s: err %v; want an undecodable answer", kind, err)
		}
		failed := errors.New("posthog: 403 Forbidden")
		if _, pv, err := (PostHog{API: phAnswer{err: failed}, Project: "12345"}).Run(context.Background(), q); !errors.Is(err, failed) || len(pv.Requests) != 1 {
			t.Errorf("%s: err %v, provenance %+v; want the provider's error and the request it made", kind, err, pv)
		}
	}
	q, _ := ParseQuery("logs where service=web", from, from.Add(8*24*time.Hour))
	if _, pv, err := (PostHog{API: phAnswer{err: errors.New("no calls")}, Project: "12345"}).Run(context.Background(), q); err == nil || len(pv.Requests) != 0 {
		t.Errorf("an 8-day window: err %v, requests %v; want it refused before any request", err, pv.Requests)
	}
}

// phAnswer answers every PostHog query with one body or one error.
type phAnswer struct {
	body string
	err  error
}

func (a phAnswer) Query(context.Context, string, map[string]any) ([]byte, string, error) {
	return []byte(a.body), "POST /api/projects/12345/query/", a.err
}
