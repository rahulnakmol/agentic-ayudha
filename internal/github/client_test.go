package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	gh "github.com/google/go-github/v90/github"
	"github.com/shurcooL/githubv4"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
)

// fakeClock records sleeps instead of sleeping.
type fakeClock struct {
	mu     sync.Mutex
	now    time.Time
	sleeps []time.Duration
}

func (f *fakeClock) Now() time.Time { f.mu.Lock(); defer f.mu.Unlock(); return f.now }
func (f *fakeClock) Sleep(_ context.Context, d time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sleeps = append(f.sleeps, d)
	f.now = f.now.Add(d)
	return nil
}

func newTest(t *testing.T, h http.HandlerFunc, o Options) (*Client, *fakeClock) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	clk := &fakeClock{now: time.Unix(1_800_000_000, 0)}
	o.BaseURL, o.Now, o.Sleep = srv.URL, clk.Now, clk.Sleep
	if o.Log == nil {
		o.Log = newLog(t, false)
	}
	c, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	return c, clk
}

func label(ctx context.Context, c *Client) error {
	_, err := c.Write(ctx, actions.Action{Kind: "github.label.create", Request: &actions.Request{Method: "POST", Path: "/repos/o/r/labels"}},
		func(ctx context.Context) (any, error) {
			_, _, err := c.REST.Issues.CreateLabel(ctx, "o", "r", gh.CreateIssueLabelRequest{Name: "x"})
			return nil, err
		})
	return err
}

func TestRateLimits(t *testing.T) {
	ctx := context.Background()
	reset := time.Unix(1_800_000_000, 0).Add(42 * time.Second)
	tests := []struct {
		name  string
		first func(w http.ResponseWriter)
		wait  time.Duration
	}{
		{"secondary with Retry-After", func(w http.ResponseWriter) {
			w.Header().Set("Retry-After", "7")
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"message":"You have exceeded a secondary rate limit"}`))
		}, 7 * time.Second},
		{"secondary without Retry-After", func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"message":"You have exceeded a secondary rate limit. Please wait a few minutes"}`))
		}, time.Minute},
		{"primary exhausted", func(w http.ResponseWriter) {
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(reset.Unix(), 10))
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"message":"API rate limit exceeded"}`))
		}, 43 * time.Second},
		{"429", func(w http.ResponseWriter) { w.WriteHeader(http.StatusTooManyRequests) }, time.Minute},
	}
	for _, tt := range tests {
		n := 0
		c, clk := newTest(t, func(w http.ResponseWriter, r *http.Request) {
			n++
			if n == 1 {
				tt.first(w)
				return
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"name":"x"}`))
		}, Options{WriteGap: -1})
		if err := label(ctx, c); err != nil {
			t.Errorf("%s: %v", tt.name, err)
			continue
		}
		if n != 2 || len(clk.sleeps) != 1 || !jittered(clk.sleeps[0], tt.wait) || c.Stats().Retries != 1 {
			t.Errorf("%s: %d requests, sleeps %v, want one wait of %v plus up to a quarter", tt.name, n, clk.sleeps, tt.wait)
		}
	}

	// A plain 403 (permissions) is not retried; a limit longer than MaxWait fails at once.
	n := 0
	c, clk := newTest(t, func(w http.ResponseWriter, r *http.Request) {
		n++
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"Resource not accessible by integration"}`))
	}, Options{})
	if err := label(ctx, c); !External(err) || n != 1 || len(clk.sleeps) != 0 {
		t.Errorf("permission 403: err %v, %d requests, sleeps %v", err, n, clk.sleeps)
	}
	n = 0
	c, clk = newTest(t, func(w http.ResponseWriter, r *http.Request) {
		n++
		w.Header().Set("Retry-After", "3600")
		w.WriteHeader(http.StatusForbidden)
	}, Options{})
	if err := label(ctx, c); !External(err) || n != 1 || len(clk.sleeps) != 0 {
		t.Errorf("hour-long limit: err %v, %d requests, sleeps %v", err, n, clk.sleeps)
	}
}

// jittered reports whether got is want plus a jitter of less than a quarter.
func jittered(got, want time.Duration) bool { return got >= want && got < want+want/4 }

// WP5.5: a 5xx is retried with jittered exponential backoff when repeating
// the request cannot write twice (a read, PUT, or DELETE), never for a POST,
// which may have landed.
func TestServerErrorRetries(t *testing.T) {
	ctx := context.Background()
	n := 0
	c, clk := newTest(t, func(w http.ResponseWriter, r *http.Request) {
		n++
		if n <= 2 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(`[]`))
	}, Options{WriteGap: -1})
	if _, _, err := c.REST.Issues.ListLabels(ctx, "o", "r", nil); err != nil || n != 3 || len(clk.sleeps) != 2 ||
		!jittered(clk.sleeps[0], time.Second) || !jittered(clk.sleeps[1], 2*time.Second) {
		t.Fatalf("GET after two 502s: err %v, %d requests, sleeps %v", err, n, clk.sleeps)
	}
	n = 0
	c, clk = newTest(t, func(w http.ResponseWriter, r *http.Request) {
		n++
		w.WriteHeader(http.StatusInternalServerError)
	}, Options{WriteGap: -1})
	if err := label(ctx, c); err == nil || n != 1 || len(clk.sleeps) != 0 {
		t.Fatalf("POST after a 500: err %v, %d requests, sleeps %v", err, n, clk.sleeps)
	}
}

// Bulk writes are serialized with a gap; reads are not delayed.
func TestWritesAreSerialized(t *testing.T) {
	c, clk := newTest(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{}`))
	}, Options{WriteGap: time.Second})
	ctx := context.Background()
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() { defer wg.Done(); _ = label(ctx, c) }()
	}
	wg.Wait()
	_, _, _ = c.REST.Issues.ListLabels(ctx, "o", "r", nil)
	if len(clk.sleeps) != 3 {
		t.Fatalf("4 writes slept %v, want 3 one-second gaps", clk.sleeps)
	}
	for _, s := range clk.sleeps {
		if s != time.Second {
			t.Fatalf("sleeps %v", clk.sleeps)
		}
	}
}

// Two clients that share a write gate keep one gap between all their writes;
// two that do not each keep their own.
func TestSharedWriteGate(t *testing.T) {
	h := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{}`))
	}
	ctx := context.Background()
	for _, shared := range []bool{true, false} {
		var gate *WriteGate
		if shared {
			gate = &WriteGate{}
		}
		a, clk := newTest(t, h, Options{WriteGap: time.Second, Writes: gate})
		b, _ := newTest(t, h, Options{WriteGap: time.Second, Writes: gate})
		b.opts.Now, b.opts.Sleep = clk.Now, clk.Sleep // one clock for both
		_ = label(ctx, a)
		_ = label(ctx, b)
		if want := map[bool]int{true: 1, false: 0}[shared]; len(clk.sleeps) != want {
			t.Errorf("shared %v: a write on each client slept %v, want %d gaps", shared, clk.sleeps, want)
		}
	}
}

func TestBudgetDryRunAndCapability(t *testing.T) {
	ctx := context.Background()
	hits := 0
	h := func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Link", fmt.Sprintf(`<%s?page=%d>; rel="next"`, r.URL.Path, hits+1))
		_, _ = w.Write([]byte(`[{"name":"a"}]`))
	}
	// A paginated walk stops at the call budget with ErrBudget.
	c, _ := newTest(t, h, Options{Budget: 3})
	_, err := All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.Label, *gh.Response, error) {
		return c.REST.Issues.ListLabels(ctx, "o", "r", &o)
	})
	if !errors.Is(err, ErrBudget) || hits != 3 || c.Stats().Calls() != 3 {
		t.Fatalf("budget: err %v, %d requests, stats %+v", err, hits, c.Stats())
	}

	// --dry-run: the write is logged as planned and nothing is sent.
	hits = 0
	c, _ = newTest(t, h, Options{Log: newLog(t, true)})
	if err := label(ctx, c); err != nil || hits != 0 {
		t.Fatalf("dry run: err %v, %d requests", err, hits)
	}

	// labels-only: a body edit is refused before any request, even if a perform
	// function lies about what it sends.
	c, _ = newTest(t, h, Options{Capability: actions.CapLabelsOnly})
	_, err = c.Write(ctx, actions.Action{Kind: "github.issue.labels.add", Request: &actions.Request{Method: "POST", Path: "/repos/o/r/issues/1/labels"}},
		func(ctx context.Context) (any, error) {
			_, _, err := c.REST.Issues.Update(ctx, "o", "r", 1, gh.UpdateIssueRequest{Body: gh.Ptr("set priority P0")})
			return nil, err
		})
	if !errors.Is(err, actions.ErrCapability) || hits != 0 {
		t.Fatalf("labels-only: err %v, %d requests", err, hits)
	}
}

// WP5.5: PUT and DELETE are idempotent, so a 5xx on them is retried like a
// read; so is a GraphQL query, which is a POST that cannot write. A GraphQL
// mutation is a POST like any other write and is not repeated.
func TestServerErrorRetriesIdempotentWrites(t *testing.T) {
	ctx := context.Background()
	failFirst := func(n *int, body string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			*n++
			if *n == 1 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			_, _ = w.Write([]byte(body))
		}
	}
	writes := []struct {
		name    string
		method  string
		perform func(ctx context.Context, c *Client) error
	}{
		{"PUT", http.MethodPut, func(ctx context.Context, c *Client) error {
			_, _, err := c.REST.Issues.ReplaceLabelsForIssue(ctx, "o", "r", 1, []string{"x"})
			return err
		}},
		{"DELETE", http.MethodDelete, func(ctx context.Context, c *Client) error {
			_, err := c.REST.Issues.RemoveLabelForIssue(ctx, "o", "r", 1, "x")
			return err
		}},
	}
	for _, tt := range writes {
		n := 0
		c, clk := newTest(t, failFirst(&n, `[]`), Options{WriteGap: -1})
		_, err := c.Write(ctx, actions.Action{Kind: "github.issue.labels", Request: &actions.Request{Method: tt.method, Path: "/repos/o/r/issues/1/labels"}},
			func(ctx context.Context) (any, error) { return nil, tt.perform(ctx, c) })
		if err != nil || n != 2 || len(clk.sleeps) != 1 || !jittered(clk.sleeps[0], time.Second) {
			t.Errorf("%s after a 503: err %v, %d requests, sleeps %v", tt.name, err, n, clk.sleeps)
		}
	}

	n := 0
	c, clk := newTest(t, failFirst(&n, `{"data":{"viewer":{"login":"octocat"}}}`), Options{WriteGap: -1})
	var q struct {
		Viewer struct{ Login string }
	}
	if err := c.GraphQL.Query(ctx, &q, nil); err != nil || q.Viewer.Login != "octocat" || n != 2 || len(clk.sleeps) != 1 || c.Stats().GraphQL != 2 {
		t.Errorf("GraphQL query after a 503: err %v, %+v, %d requests, sleeps %v, stats %+v", err, q, n, clk.sleeps, c.Stats())
	}

	n = 0
	c, clk = newTest(t, failFirst(&n, `{"data":{}}`), Options{WriteGap: -1})
	var m struct {
		AddStar struct{ ClientMutationID string } `graphql:"addStar(input: $input)"`
	}
	_, err := c.Write(ctx, actions.Action{Kind: "github.graphql.mutation", Request: &actions.Request{Method: http.MethodPost, Path: "/graphql"}},
		func(ctx context.Context) (any, error) {
			return nil, c.GraphQL.Mutate(ctx, &m, githubv4.AddStarInput{StarrableID: githubv4.ID("x")}, nil)
		})
	if err == nil || n != 1 || len(clk.sleeps) != 0 {
		t.Errorf("GraphQL mutation after a 503: err %v, %d requests, sleeps %v", err, n, clk.sleeps)
	}

	// A cancelled run stops waiting for the retry and sends nothing more.
	n = 0
	c, _ = newTest(t, failFirst(&n, `[]`), Options{WriteGap: -1})
	c.opts.Sleep = func(context.Context, time.Duration) error { return context.Canceled }
	if _, _, err := c.REST.Issues.ListLabels(ctx, "o", "r", nil); !errors.Is(err, context.Canceled) || n != 1 {
		t.Errorf("GET after a 503 when the wait is cancelled: err %v, %d requests", err, n)
	}
}

// The backoff jitter is less than a quarter of the wait, and none for a wait
// too short to split.
func TestJitterWithinAQuarter(t *testing.T) {
	for _, d := range []time.Duration{0, 3, 4, time.Second, time.Minute} {
		for range 100 {
			if j := jitter(d); j < 0 || (d >= 4 && j >= d/4) || (d < 4 && j != 0) {
				t.Fatalf("jitter(%v) = %v", d, j)
			}
		}
	}
}

// A capability-limited client (labels-only) sends GraphQL queries but
// refuses GraphQL mutations, whose effect it cannot check.
func TestCapabilityRefusesGraphQLMutations(t *testing.T) {
	ctx := context.Background()
	hits := 0
	c, _ := newTest(t, func(w http.ResponseWriter, r *http.Request) {
		hits++
		_, _ = w.Write([]byte(`{"data":{"viewer":{"login":"octocat"}}}`))
	}, Options{Capability: actions.CapLabelsOnly})
	var q struct {
		Viewer struct{ Login string }
	}
	if err := c.GraphQL.Query(ctx, &q, nil); err != nil || hits != 1 {
		t.Fatalf("labels-only GraphQL query: err %v, %d requests", err, hits)
	}
	var m struct {
		AddStar struct{ ClientMutationID string } `graphql:"addStar(input: $input)"`
	}
	_, err := c.Write(ctx, actions.Action{Kind: "github.labels", Request: &actions.Request{Method: http.MethodPost, Path: "/repos/o/r/issues/1/labels"}},
		func(ctx context.Context) (any, error) {
			return nil, c.GraphQL.Mutate(ctx, &m, githubv4.AddStarInput{StarrableID: githubv4.ID("x")}, nil)
		})
	if !errors.Is(err, actions.ErrCapability) || hits != 1 {
		t.Fatalf("labels-only GraphQL mutation: err %v, %d requests", err, hits)
	}
}
