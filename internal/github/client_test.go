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
		if n != 2 || len(clk.sleeps) != 1 || clk.sleeps[0] != tt.wait || c.Stats().Retries != 1 {
			t.Errorf("%s: %d requests, sleeps %v, want one wait of %v", tt.name, n, clk.sleeps, tt.wait)
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
