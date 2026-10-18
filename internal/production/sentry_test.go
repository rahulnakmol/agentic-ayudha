package production

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations/fixture"
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
			if _, _, err := src.Run(context.Background(), q, nil); err == nil || !errors.Is(err, errSentry) {
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
