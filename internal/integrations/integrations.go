// Package integrations holds what aayudha's provider adapters share (WP5.2):
// the reproducible baseline tuple, resolution of [integrations.<name>] for a
// read verb (WP1.3 scope refusal before any call), the provider-reported key
// scope that doctor and the adapters verify, and a transport that keeps every
// adapter read-only. The adapters live in the subpackages; providers is the
// catalog verbs and doctor use.
package integrations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/secrets"
)

// Errors. The CLI maps ErrNotConfigured and ErrProvider to exit 5, ErrQuery to
// exit 2, and secrets.ErrWriteScoped (a write-capable key) to exit 4.
var (
	// ErrNotConfigured means [integrations.<name>] is missing or incomplete,
	// or its secret has no value. The message names the key to set.
	ErrNotConfigured = errors.New("not configured")
	// ErrProvider means the provider or the network failed, or answered in a
	// shape the adapter does not understand.
	ErrProvider = errors.New("provider error")
	// ErrQuery means the baseline query itself is invalid (unknown metric,
	// empty window).
	ErrQuery = errors.New("invalid baseline query")
	// ErrNoAdapter means the integration is configured but this binary has no
	// adapter for it.
	ErrNoAdapter = errors.New("no adapter for this integration in this binary")
)

// Window is the half-open interval [From, To) a baseline covers, in UTC.
type Window struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

// Check reports whether the window is usable: both ends set, From before To.
func (w Window) Check() error {
	if w.From.IsZero() || w.To.IsZero() || !w.From.Before(w.To) {
		return fmt.Errorf("%w: window %s/%s: want from < to", ErrQuery, w.From.UTC().Format(time.RFC3339), w.To.UTC().Format(time.RFC3339))
	}
	return nil
}

// Query asks an adapter for one baseline.
type Query struct {
	// Metric is one of the adapter's metric names (providers.Provider.Metrics).
	Metric string
	// Filter narrows the population in the provider's own syntax: a HogQL
	// predicate (PostHog), a Sentry search query, or GitHub search
	// qualifiers. Empty means everything the metric counts.
	Filter string
	Window Window
}

// Baseline is one reproducible baseline (WP5.2): what was measured, over
// whom, when, its value, the exact query that produced it, and when it was
// fetched. Running SourceQuery again over Window reproduces Value.
type Baseline struct {
	Metric      string    `json:"metric"`
	Population  string    `json:"population"`
	Window      Window    `json:"window"`
	Value       float64   `json:"value"`
	SourceQuery string    `json:"source_query"`
	FetchedAt   time.Time `json:"fetched_at"`
}

// Field is a provider-specific key an integration needs besides secret and scope.
type Field struct {
	Key  string // "project", "org"
	Want string // what to put there, for the error message
	Auth string // needed only with this auth ("key", "oidc", "cli"); "" means always
}

// Conn is one configured integration resolved for a read verb.
type Conn struct {
	Name string
	config.Integration
	Key string // the secret's value; never logged or printed
}

// Resolve returns [integrations.<name>] for a read verb. Before any call it
// fails when the section is missing or lacks a needed field (ErrNotConfigured,
// naming the key to set), when its scope is "write" (secrets.ErrWriteScoped,
// WP1.3), or when its secret has no value (ErrNotConfigured).
func Resolve(cfg *config.Config, store *secrets.Store, name string, need ...Field) (Conn, error) {
	var in config.Integration
	ok := false
	if cfg != nil {
		in, ok = cfg.Integrations[name]
	}
	if !ok {
		parts := []string{fmt.Sprintf("secret = %q", name), `scope = "read"`}
		for _, f := range need {
			if f.Auth == "" || f.Auth == "key" {
				parts = append(parts, fmt.Sprintf("%s = %q", f.Key, "<"+f.Want+">"))
			}
		}
		return Conn{}, fmt.Errorf("%w: [integrations.%s] is not set; add it to .aayudha/config.toml with %s, then store a read-only key with `aayu auth set-secret %s`",
			ErrNotConfigured, name, strings.Join(parts, ", "), name)
	}
	if err := secrets.CheckScope(name, in.Scope, false); err != nil {
		return Conn{}, err
	}
	auth := in.Auth
	if auth == "" {
		auth = "key"
	}
	for _, f := range need {
		if f.Auth != "" && f.Auth != auth {
			continue
		}
		if field(in, f.Key) == "" {
			return Conn{}, fmt.Errorf("%w: [integrations.%s] %s is not set; want %s", ErrNotConfigured, name, f.Key, f.Want)
		}
	}
	if !in.StoredKey() {
		// auth = "oidc" or "cli" (cloud integrations): the adapter mints a
		// short-lived token; no secret is stored.
		return Conn{Name: name, Integration: in}, nil
	}
	if store == nil {
		return Conn{}, fmt.Errorf("%w: [integrations.%s] secret %q: no secrets store", ErrNotConfigured, name, in.Secret)
	}
	key, _, err := store.Get(in.Secret)
	if err != nil {
		return Conn{}, fmt.Errorf("%w: [integrations.%s] secret %q has no value: %w", ErrNotConfigured, name, in.Secret, err)
	}
	return Conn{Name: name, Integration: in, Key: key}, nil
}

func field(in config.Integration, key string) string {
	switch key {
	case "project":
		return in.Project
	case "org":
		return in.Org
	case "url":
		return in.URL
	case "identity":
		return in.Identity
	case "federation":
		return in.Federation
	case "region":
		return in.Region
	}
	return ""
}

// BaseURL is the integration's url, or def when it is not set, without a trailing slash.
func (c Conn) BaseURL(def string) string {
	if c.URL != "" {
		return strings.TrimSuffix(c.URL, "/")
	}
	return def
}

// Scope is what a provider reports about a key (WP1.3: doctor verifies
// read-only scope where the provider API allows).
type Scope struct {
	Verified bool     `json:"verified"`           // the provider API reported the key's scopes
	Scopes   []string `json:"scopes,omitempty"`   // as reported
	Write    []string `json:"write,omitempty"`    // the reported scopes that allow writes
	Account  string   `json:"account,omitempty"`  // who or what the key belongs to, when reported
	Endpoint string   `json:"endpoint,omitempty"` // the request that reported it
}

// Check refuses a key the provider reports as write-capable.
func (s Scope) Check(name string) error {
	if len(s.Write) == 0 {
		return nil
	}
	return fmt.Errorf("%w: the %s key is write-capable (scopes %s, reported by %s); replace it with a read-only key", secrets.ErrWriteScoped, name, strings.Join(s.Write, ", "), s.Endpoint)
}

// Allow is a non-GET request a read-only adapter may send: a query endpoint
// that takes a POST body but changes nothing, or a token exchange.
type Allow struct {
	Method string
	Path   *regexp.Regexp
	// Host, when set, is the only host the request may go to.
	Host string
	// Header and Value, when set, must be on the request: the operation of an
	// AWS JSON protocol call (X-Amz-Target), which always posts to "/".
	Header, Value string
}

// ReadOnly wraps a transport so an adapter can only read: GET and HEAD pass,
// as do the requests in allow; anything else is refused before it leaves the
// process. Adapters never write, so nothing goes through internal/actions
// (ground rule 6); this makes that structural rather than a convention.
func ReadOnly(next http.RoundTripper, allow ...Allow) http.RoundTripper {
	if next == nil {
		next = http.DefaultTransport
	}
	return readOnly{next, allow}
}

type readOnly struct {
	next  http.RoundTripper
	allow []Allow
}

func (r readOnly) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead && !r.allowed(req) {
		return nil, fmt.Errorf("%w: %s %s: integration adapters only read", actions.ErrCapability, req.Method, req.URL.Path)
	}
	return r.next.RoundTrip(req)
}

func (r readOnly) allowed(req *http.Request) bool {
	for _, a := range r.allow {
		if a.Method == req.Method && a.Path.MatchString(req.URL.Path) && (a.Host == "" || a.Host == req.URL.Host) &&
			(a.Header == "" || req.Header.Get(a.Header) == a.Value) {
			return true
		}
	}
	return false
}

// Client returns the HTTP client an adapter uses: next wrapped by ReadOnly,
// with a timeout.
func Client(next http.RoundTripper, allow ...Allow) *http.Client {
	return &http.Client{Transport: ReadOnly(next, allow...), Timeout: time.Minute}
}

// DoJSON sends req and decodes a 2xx JSON response into out. Network
// failures, other statuses, and undecodable bodies are ErrProvider; the
// message carries the status and the start of the body, credential-redacted.
func DoJSON(hc *http.Client, req *http.Request, out any) error {
	resp, err := hc.Do(req)
	if err != nil {
		if errors.Is(err, actions.ErrCapability) {
			return err
		}
		return fmt.Errorf("%w: %s %s: %w", ErrProvider, req.Method, req.URL.Redacted(), err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return fmt.Errorf("%w: %s %s: %w", ErrProvider, req.Method, req.URL.Redacted(), err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("%w: %s %s: %s: %s", ErrProvider, req.Method, req.URL.Redacted(), resp.Status, excerpt(body))
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("%w: %s %s: undecodable response: %w", ErrProvider, req.Method, req.URL.Redacted(), err)
	}
	return nil
}

// NewRequest builds a request with a JSON body (when body is not nil) and the
// headers every adapter sends.
func NewRequest(ctx context.Context, method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrQuery, err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "aayudha")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

func excerpt(b []byte) string {
	s := strings.Join(strings.Fields(string(b)), " ")
	if len(s) > 200 {
		s = s[:200] + "…"
	}
	return actions.RedactString(s)
}
