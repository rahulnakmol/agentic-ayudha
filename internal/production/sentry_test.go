package production

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
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
	"github.com/rahulnakmol/agentic-ayudha/internal/seats"
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
	t.Run("rollback to an old release", func(t *testing.T) {
		// Twelve releases, newest created first; the oldest was redeployed
		// in the window (a rollback). Its deploy is the newest, the release
		// live when the window opened is the previous, and no other release's
		// deploys are read.
		var list []string
		for i := range 11 {
			list = append(list, fmt.Sprintf(`{"version":"web@3.%d","lastDeploy":{"environment":"production","dateFinished":"2026-10-16T%02d:00:00Z"}}`, 10-i, 20-i))
		}
		list = append(list, `{"version":"web@1.0","lastDeploy":{"environment":"production","dateFinished":"2026-10-17T08:30:00Z"}}`)
		api := scripted{project: "4507", answers: map[string][]byte{
			"/releases/":                  []byte("[" + strings.Join(list, ",") + "]"),
			"/releases/web@1.0/deploys/":  []byte(`[{"environment":"production","dateFinished":"2026-10-17T08:30:00Z"},{"environment":"production","dateFinished":"2026-10-01T09:00:00Z"}]`),
			"/releases/web@3.10/deploys/": []byte(`[{"environment":"production","dateFinished":"2026-10-16T20:00:00Z"}]`),
		}}
		q, _ := ParseQuery("deploys where service=web", from, to)
		r, pv, err := (Sentry{API: &api}).Run(context.Background(), q, nil)
		if err != nil || len(pv.Requests) != 3 || len(r.Rows) != 1 || r.Rows[0][3] != "web@1.0" || r.Rows[0][4] != "web@3.10" {
			t.Errorf("rows %v, requests %v, err %v; want web@1.0 after web@3.10 from three reads", r.Rows, pv.Requests, err)
		}
	})
	t.Run("failed lookup before a good one", func(t *testing.T) {
		// Issue 1's latest event fails and issue 2's answers: the digest
		// pairs each request with its own answer, the failed one with none.
		rec := fixture.Load(t, filepath.Join("testdata", "sentry", "exceptions.json")).Interactions
		events, latest := rec[0].Response, rec[1].Response
		api := scripted{project: "4507", answers: map[string][]byte{"/events/": events, "/issues/2/events/latest/": latest}}
		q, _ := ParseQuery("exceptions where service=web", from, to)
		_, pv, err := (Sentry{API: &api}).Run(context.Background(), q, resolve)
		if err != nil || len(pv.Requests) != 3 {
			t.Fatalf("err %v, requests %v; want three", err, pv.Requests)
		}
		want := sha256.Sum256([]byte(pv.Requests[0] + "\n" + string(events) + "\n" + pv.Requests[1] + "\n\n" + pv.Requests[2] + "\n" + string(latest) + "\n"))
		if !strings.HasSuffix(pv.Requests[1], "/issues/1/events/latest/") || pv.Digest != hex.EncodeToString(want[:]) {
			t.Errorf("requests %v digest %s; want the failed issue 1 read paired with no answer", pv.Requests, pv.Digest)
		}
	})
}

// scripted answers a Sentry read by its path's suffix; any other path fails.
type scripted struct {
	project string
	answers map[string][]byte
}

func (s *scripted) Project() string { return s.project }

func (s *scripted) Get(_ context.Context, path string, _ url.Values) ([]byte, string, error) {
	req := "GET " + path
	for suffix, b := range s.answers {
		if strings.HasSuffix(path, suffix) && (suffix != "/events/" || !strings.Contains(path, "/issues/")) {
			return b, req, nil
		}
	}
	return nil, req, errors.New("not found")
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

// listAnswer answers every list read with one body or one error.
type listAnswer struct {
	body string
	err  error
}

func (a listAnswer) Get(context.Context, string, url.Values) ([]byte, string, error) {
	return []byte(a.body), "GET /list/", a.err
}

func (listAnswer) Project() string { return "4507" }

// TestRunbookAlerts (Decision #449, phase P1): the alerts a runbook's Symptom
// names are looked up in Sentry and PostHog; an SLO name is not an alert, and
// with no provider every alert is not checked rather than missing.
func TestRunbookAlerts(t *testing.T) {
	src, r := sentrySource(t, "alerts")
	sentryAlerts, spv, err := src.Alerts(context.Background())
	r.Done()
	if err != nil || len(spv.Requests) != 3 || len(spv.Digest) != 64 {
		t.Fatalf("Sentry alerts: %v, provenance %+v", err, spv)
	}
	pr := &fixture.Replayer{T: t, Rec: fixture.Load(t, filepath.Join("testdata", "posthog", "alerts.json")), AuthHeader: "Authorization", AuthValue: "Bearer phx_test"}
	c, err := posthog.New(integrations.Conn{Name: "posthog", Key: "phx_test", Integration: config.Integration{URL: "https://eu.posthog.com", Project: "12345"}}, pr, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	posthogAlerts, ppv, err := PostHogAlerts(context.Background(), c, "12345")
	pr.Done()
	if err != nil || len(ppv.Requests) != 1 {
		t.Fatalf("PostHog alerts: %v, provenance %+v", err, ppv)
	}
	slos := &SLOs{SLOs: []SLO{{Name: "checkout-availability"}}}
	md := "## Symptom\n\n`error-monitor` fires (`error-monitor` pages), or `nightly-billing` misses, or `exceptions-spike`, or `disk-full`; it protects `checkout-availability`.\n"
	got := RunbookAlerts(md, slos, map[string][]string{"sentry": sentryAlerts, "posthog": posthogAlerts})
	want := []AlertStatus{{"error-monitor", AlertFound, "sentry"}, {"nightly-billing", AlertFound, "sentry"}, {"exceptions-spike", AlertFound, "posthog"}, {"disk-full", AlertMissing, ""}}
	if !slices.Equal(got, want) {
		t.Errorf("RunbookAlerts = %+v; want %+v", got, want)
	}
	for _, a := range RunbookAlerts(md, slos, nil) {
		if a.State != AlertNotChecked {
			t.Errorf("no provider: %+v; want not checked", a)
		}
	}
	for _, c := range []struct {
		name string
		api  listAnswer
	}{
		{"a failed read", listAnswer{err: errors.New("503")}},
		{"a wrong-shape answer", listAnswer{body: `"not a list"`}},
	} {
		if _, pv, err := (Sentry{API: c.api}).Alerts(context.Background()); err == nil || len(pv.Requests) != 1 {
			t.Errorf("Sentry alerts, %s: err %v, requests %v; want an error after the first request", c.name, err, pv.Requests)
		}
		if _, pv, err := PostHogAlerts(context.Background(), c.api, "12345"); err == nil || len(pv.Requests) != 1 {
			t.Errorf("PostHog alerts, %s: err %v, requests %v; want an error", c.name, err, pv.Requests)
		}
	}
	if Slug("[us] High error-rate!") != "us-high-error-rate" {
		t.Errorf("Slug = %q", Slug("[us] High error-rate!"))
	}
}

// TestSeerOpinion: Seer's existing analysis is read with one GET, rendered
// as an untrusted block labelled as Seer's opinion, and carries no citable
// form: a note that cites a path, an issue, or a query only Seer named is
// refused by the evidence floor.
func TestSeerOpinion(t *testing.T) {
	src, r := sentrySource(t, "seer")
	op, pv, err := Seer(context.Background(), src.API, "7")
	r.Done()
	if err != nil || op.Status != "COMPLETED" || len(op.RootCause) != 1 || len(op.Solution) != 1 {
		t.Fatalf("opinion %+v, err %v; want a completed analysis with one cause and one step", op, err)
	}
	if len(pv.Requests) != 1 || !strings.HasPrefix(pv.Requests[0], "GET https://sentry.io/api/0/organizations/acme/issues/7/autofix/") || len(pv.Digest) != 64 {
		t.Errorf("provenance %+v; want one GET of the issue-fix state and a SHA-256", pv)
	}
	md := op.Markdown()
	for _, want := range []string{"Seer's opinion", "never evidence", `<untrusted source="sentry seer">`, "Seer's root cause:", "Seer's proposed step:", "[email]"} {
		if !strings.Contains(md, want) {
			t.Errorf("block lacks %q:\n%s", want, md)
		}
	}
	if strings.Contains(md, "dev@example.com") {
		t.Errorf("block holds an email address:\n%s", md)
	}
	note := "The crash comes from seer/automation/tools.py, as in #4512 and query q2, since `4ebb4f342e30`; `automation.RunRipgrep` is the bug."
	findings := strings.Join(seats.CheckGrounding(note, md), "\n")
	for _, c := range []string{"seer/automation/tools.py", "#4512", "q2", "4ebb4f342e30", "automation.RunRipgrep"} {
		if !strings.Contains(findings, c) {
			t.Errorf("the evidence floor accepted %s from Seer's text; findings:\n%s", c, findings)
		}
	}

	src, r = sentrySource(t, "seer-none")
	op, _, err = Seer(context.Background(), src.API, "8")
	r.Done()
	if err != nil || op.Status != "" || !strings.Contains(op.Markdown(), "No Seer analysis") {
		t.Errorf("never analysed: %+v, %v; want no analysis and no error", op, err)
	}
	if _, _, err := Seer(context.Background(), answer{`["not the shape"]`}, "9"); err == nil || !strings.Contains(err.Error(), "undecodable") {
		t.Errorf("wrong shape: err %v; want an undecodable answer", err)
	}
	if _, pv, err := Seer(context.Background(), routes{}, "10"); err == nil || len(pv.Requests) != 1 || pv.Digest != "" {
		t.Errorf("a failed read: %+v, %v; want the error with its request and no digest", pv, err)
	}
}

// TestMeasureSentry: a crash-free SLO is measured from Sentry's session
// counts over the whole SLO window, as aggregates: every session that did not
// crash is good (errored and abnormal sessions count as good), and the last
// hour's and six hours' crashes give the burn rates.
func TestMeasureSentry(t *testing.T) {
	defs, err := ParseSLOs([]byte("version = 1\n[[slo]]\nname = \"app-crash-free\"\nservice = \"app\"\njourney = \"Open the app\"\nsli = \"crash-free\"\nobjective = 99.9\nwindow = \"28d\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	to := time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC)
	var digest string
	for range 2 {
		src, r := sentrySource(t, "sessions")
		m, pv, err := MeasureSentry(context.Background(), src.API, defs.SLOs[0], to)
		r.Done()
		if err != nil {
			t.Fatal(err)
		}
		// 672 hours of 100 healthy and 2 errored sessions, one abnormal, and 25
		// crashed: 3 in each of hours 6 to 2 before the end and 10 in the last.
		want := Measured{Name: "app-crash-free", SLI: "crash-free", Objective: 99.9, From: to.Add(-28 * 24 * time.Hour), To: to, Covered: "28.0d of 28d",
			Good: 68545, Total: 68570, Value: 99.964, BudgetRemaining: 0.635, Burn1h: 89.286, Burn6h: 39.246, Source: "sentry project 4507 sessions", Digest: pv.Digest}
		if m != want {
			t.Fatalf("measured %+v\nwant     %+v", m, want)
		}
		if len(pv.Requests) != 1 || !strings.Contains(pv.Requests[0], "/organizations/acme/sessions/?") || len(pv.Digest) != 64 {
			t.Errorf("provenance %+v; want one sessions request and a SHA-256", pv)
		}
		if digest != "" && digest != pv.Digest {
			t.Errorf("digest %s, then %s; want the same answer to give the same digest", digest, pv.Digest)
		}
		digest = pv.Digest
	}
	if _, _, err := MeasureSentry(context.Background(), answer{`{"groups":"no"}`}, defs.SLOs[0], to); err == nil || !strings.Contains(err.Error(), "undecodable") {
		t.Errorf("wrong shape: err %v; want an undecodable answer", err)
	}
	if _, pv, err := MeasureSentry(context.Background(), routes{}, defs.SLOs[0], to); err == nil || len(pv.Requests) != 1 || pv.Digest != "" {
		t.Errorf("a failed read: %+v, %v; want the error with its request and no digest", pv, err)
	}
	// A group with no session total (Sentry's null) counts nothing, even crashed.
	m, _, err := MeasureSentry(context.Background(), answer{`{"groups":[{"by":{"session.status":"crashed"},"totals":{"sum(session)":null}},` +
		`{"by":{"session.status":"healthy"},"totals":{"sum(session)":40},"series":{"sum(session)":[null,30,10]}}]}`}, defs.SLOs[0], to)
	if err != nil || m.Good != 40 || m.Total != 40 {
		t.Errorf("a null crashed total: %+v, %v; want 40 good of 40", m, err)
	}
	spans := SLO{Name: "a", SLI: "availability"}
	if _, _, err := MeasureSentry(context.Background(), noCalls{t}, spans, to); err == nil {
		t.Error("an availability SLO was measured from Sentry; want it refused before any request")
	}
}
