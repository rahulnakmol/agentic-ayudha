package github

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
)

// transport is the client's middleware, outermost first: the write gate
// (ground rule 6) and capability guard, serialized writes, the call budget,
// rate-limit retries, the ETag cache, and authentication.
type transport struct {
	c     *Client
	cache *cache
	next  http.RoundTripper

	own WriteGate // the client's own, when Options.Writes shares none
}

// WriteGate serializes writes and keeps GitHub's gap between them. Clients
// that share one (Options.Writes) never overlap their writes, though each
// was built on its own: the inbox's background writes do.
type WriteGate struct {
	mu   sync.Mutex
	last time.Time
}

func mutating(m string) bool { return m != http.MethodGet && m != http.MethodHead }

// retryable reports whether repeating a request after a server error cannot
// write twice: reads, GraphQL queries, and the idempotent PUT and DELETE. A
// POST or PATCH may have landed before the error, so it is never repeated.
func retryable(req *http.Request, graphql bool) bool {
	switch req.Method {
	case http.MethodGet, http.MethodHead, http.MethodPut, http.MethodDelete:
		return true
	}
	return graphql && isQuery(req)
}

// jitter is up to a quarter of d, at random.
func jitter(d time.Duration) time.Duration {
	if d < 4 {
		return 0
	}
	var b [8]byte
	_, _ = rand.Read(b[:])
	return time.Duration(binary.LittleEndian.Uint64(b[:]) % uint64(d/4))
}

func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	o := t.c.opts
	graphql := strings.HasSuffix(req.URL.Path, "/graphql")
	// A GraphQL query is a POST but reads; GraphQL mutations go through Write like any write.
	write := mutating(req.Method) && (!graphql || !isQuery(req))
	if write && req.Context().Value(writeKey{}) == nil {
		return nil, fmt.Errorf("%w: %s %s", ErrUnlogged, req.Method, req.URL.Path)
	}
	if o.Capability != "" && !graphql {
		if err := actions.AllowedRequest(o.Capability, req.Method, req.URL.Path); err != nil {
			return nil, err
		}
	} else if o.Capability != "" && write {
		return nil, fmt.Errorf("%w: GraphQL mutations are outside %s", actions.ErrCapability, o.Capability)
	}
	if write {
		// Serialize writes with a gap between them: GitHub's guidance for avoiding
		// secondary rate limits on bulk writes.
		g := o.Writes
		if g == nil {
			g = &t.own
		}
		g.mu.Lock()
		defer g.mu.Unlock()
		if o.WriteGap > 0 && !g.last.IsZero() {
			if d := g.last.Add(o.WriteGap).Sub(o.Now()); d > 0 {
				if err := o.Sleep(req.Context(), d); err != nil {
					return nil, err
				}
			}
		}
		defer func() { g.last = o.Now() }()
	}

	var cached *entry
	if req.Method == http.MethodGet {
		cached = t.cache.get(req.URL.String())
	}
	for attempt := 0; ; attempt++ {
		if err := t.count(graphql); err != nil {
			return nil, err
		}
		r, err := clone(req)
		if err != nil {
			return nil, err
		}
		if o.Token != "" {
			r.Header.Set("Authorization", "Bearer "+o.Token)
		}
		if cached != nil {
			r.Header.Set("If-None-Match", cached.etag)
		}
		resp, err := t.next.RoundTrip(r)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode == http.StatusNotModified && cached != nil {
			_ = resp.Body.Close()
			t.c.mu.Lock()
			t.c.stats.NotChanged++
			t.c.mu.Unlock()
			return cached.response(req), nil
		}
		wait, limited, err := rateLimitWait(resp, o.Now())
		if err != nil {
			return nil, err
		}
		if !limited && resp.StatusCode >= 500 && attempt < o.ServerRetries && retryable(req, graphql) {
			// A 5xx on a request that is safe to repeat (WP5.5): exponential
			// backoff from one second.
			wait, limited = time.Second<<attempt, true
		}
		if limited {
			wait += jitter(wait) // spread the retries of parallel builders
		}
		if !limited {
			if req.Method == http.MethodGet && resp.StatusCode == http.StatusOK && resp.Header.Get("ETag") != "" {
				if err := t.cache.put(req.URL.String(), resp); err != nil {
					return nil, err
				}
			}
			return resp, nil
		}
		if attempt >= o.Retries || wait > o.MaxWait {
			return resp, nil // go-github turns it into a RateLimitError or AbuseRateLimitError
		}
		_ = resp.Body.Close()
		t.c.mu.Lock()
		t.c.stats.Retries++
		t.c.stats.WaitedMS += wait.Milliseconds()
		t.c.mu.Unlock()
		if err := o.Sleep(req.Context(), wait); err != nil {
			return nil, err
		}
	}
}

func (t *transport) count(graphql bool) error {
	t.c.mu.Lock()
	defer t.c.mu.Unlock()
	if t.c.stats.Calls() >= t.c.opts.Budget {
		return fmt.Errorf("%w (%d calls)", ErrBudget, t.c.opts.Budget)
	}
	if graphql {
		t.c.stats.GraphQL++
	} else {
		t.c.stats.REST++
	}
	return nil
}

// isQuery reports whether a GraphQL POST body is a query rather than a mutation.
func isQuery(req *http.Request) bool {
	if req.GetBody == nil {
		return false
	}
	b, err := req.GetBody()
	if err != nil {
		return false
	}
	defer func() { _ = b.Close() }()
	var body struct {
		Query string `json:"query"`
	}
	if err := json.NewDecoder(b).Decode(&body); err != nil {
		return false
	}
	return !strings.HasPrefix(strings.TrimSpace(body.Query), "mutation")
}

func clone(req *http.Request) (*http.Request, error) {
	r := req.Clone(req.Context())
	if req.Body != nil && req.GetBody != nil {
		b, err := req.GetBody()
		if err != nil {
			return nil, err
		}
		r.Body = b
	}
	return r, nil
}

// rateLimitWait decides whether resp is a rate-limit refusal and how long to
// wait before retrying:
//   - Retry-After on a 403 or 429 (secondary limit): that many seconds;
//   - X-RateLimit-Remaining: 0 on a 403 or 429 (primary limit): until X-RateLimit-Reset;
//   - a 403 or 429 whose message names the secondary rate limit: one minute.
//
// It leaves resp.Body readable.
func rateLimitWait(resp *http.Response, now time.Time) (time.Duration, bool, error) {
	if resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusTooManyRequests {
		return 0, false, nil
	}
	if s := resp.Header.Get("Retry-After"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			return time.Duration(n) * time.Second, true, nil
		}
	}
	if resp.Header.Get("X-RateLimit-Remaining") == "0" {
		if reset, err := strconv.ParseInt(resp.Header.Get("X-RateLimit-Reset"), 10, 64); err == nil {
			return max(time.Unix(reset, 0).Sub(now)+time.Second, time.Second), true, nil
		}
	}
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return 0, false, err
	}
	resp.Body = io.NopCloser(bytes.NewReader(body))
	if bytes.Contains(bytes.ToLower(body), []byte("secondary rate limit")) || resp.StatusCode == http.StatusTooManyRequests {
		return time.Minute, true, nil
	}
	return 0, false, nil
}

// cache is the per-run ETag cache: a repeated GET sends If-None-Match, and a
// 304 (which does not count against the primary rate limit) is answered from
// the cached body.
type cache struct {
	mu sync.Mutex
	m  map[string]*entry
}

type entry struct {
	etag   string
	header http.Header
	body   []byte
}

func newCache() *cache { return &cache{m: map[string]*entry{}} }

func (c *cache) get(url string) *entry {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.m[url]
}

func (c *cache) put(url string, resp *http.Response) error {
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return err
	}
	resp.Body = io.NopCloser(bytes.NewReader(body))
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[url] = &entry{etag: resp.Header.Get("ETag"), header: resp.Header.Clone(), body: body}
	return nil
}

func (e *entry) response(req *http.Request) *http.Response {
	return &http.Response{
		Status: "200 OK", StatusCode: http.StatusOK, Proto: "HTTP/1.1", ProtoMajor: 1, ProtoMinor: 1,
		Header: e.header.Clone(), Body: io.NopCloser(bytes.NewReader(e.body)), ContentLength: int64(len(e.body)), Request: req,
	}
}
