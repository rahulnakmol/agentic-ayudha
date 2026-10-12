// Package sentry is the Sentry baseline adapter (WP5.2): crash-free session
// and user rates from the sessions API, and error and affected-user counts
// from the events API, after checking that the auth token carries only read
// scopes.
package sentry

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/integrations"
)

// DefaultURL is Sentry SaaS; the EU region and self-hosted instances set
// [integrations.sentry] url.
const DefaultURL = "https://sentry.io"

// metric is one Sentry metric: the endpoint that answers it and the field it reads.
type metric struct {
	endpoint string // sessions or events
	field    string
	counts   string // what the population counts, for the tuple
}

var metrics = map[string]metric{
	"crash_free_sessions": {"sessions", "crash_free_rate(session)", "sessions"},
	"crash_free_users":    {"sessions", "crash_free_rate(user)", "users with sessions"},
	"errors":              {"events", "count()", "error events"},
	"users_affected":      {"events", "count_unique(user)", "users with error events"},
}

// Metrics this adapter answers. Crash-free rates are fractions in [0, 1].
var Metrics = []string{"crash_free_sessions", "crash_free_users", "errors", "users_affected"}

// Need lists the fields [integrations.sentry] must set.
var Need = []integrations.Field{
	{Key: "org", Want: "the Sentry organization slug"},
	{Key: "project", Want: "the numeric Sentry project ID"},
}

var (
	projectID = regexp.MustCompile(`^[0-9]+$`)
	orgSlug   = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)
)

// Client reads one Sentry project.
type Client struct {
	hc      *http.Client
	base    string
	org     string
	project string
	key     string
	now     func() time.Time
}

// New returns a client for a resolved [integrations.sentry]. It sends GETs only.
func New(c integrations.Conn, rt http.RoundTripper, now func() time.Time) (*Client, error) {
	if !orgSlug.MatchString(c.Org) {
		return nil, fmt.Errorf("%w: [integrations.%s] org = %q: want the Sentry organization slug", integrations.ErrNotConfigured, c.Name, c.Org)
	}
	if !projectID.MatchString(c.Project) {
		return nil, fmt.Errorf("%w: [integrations.%s] project = %q: want the numeric Sentry project ID", integrations.ErrNotConfigured, c.Name, c.Project)
	}
	return &Client{hc: integrations.Client(rt), base: c.BaseURL(DefaultURL), org: c.Org, project: c.Project, key: c.Key, now: now}, nil
}

func (c *Client) get(ctx context.Context, u string, out any) error {
	req, err := integrations.NewRequest(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.key)
	return integrations.DoJSON(c.hc, req, out)
}

// Scope reports the token's scopes (GET /api/0/, the API root, which echoes
// the request's auth). Any scope that is not "<resource>:read" allows writes.
func (c *Client) Scope(ctx context.Context) (integrations.Scope, error) {
	const path = "/api/0/"
	var root struct {
		Auth *struct {
			Scopes []string `json:"scopes"`
		} `json:"auth"`
		User *struct {
			Username string `json:"username"`
		} `json:"user"`
	}
	if err := c.get(ctx, c.base+path, &root); err != nil {
		return integrations.Scope{}, err
	}
	if root.Auth == nil {
		return integrations.Scope{}, fmt.Errorf("%w: GET %s: the token was not accepted (no auth in the response)", integrations.ErrProvider, path)
	}
	s := integrations.Scope{Verified: true, Scopes: root.Auth.Scopes, Account: "organization token", Endpoint: "GET " + path}
	if root.User != nil {
		s.Account = "user " + root.User.Username
	}
	for _, sc := range root.Auth.Scopes {
		if !strings.HasSuffix(sc, ":read") {
			s.Write = append(s.Write, sc)
		}
	}
	return s, nil
}

// URL returns the request a baseline sends.
func (c *Client) URL(q integrations.Query) (string, error) {
	m, ok := metrics[q.Metric]
	if !ok {
		return "", fmt.Errorf("%w: sentry metric %q: want one of %s", integrations.ErrQuery, q.Metric, strings.Join(Metrics, ", "))
	}
	if err := q.Window.Check(); err != nil {
		return "", err
	}
	v := url.Values{
		"field":   {m.field},
		"project": {c.project},
		"start":   {q.Window.From.UTC().Format(time.RFC3339)},
		"end":     {q.Window.To.UTC().Format(time.RFC3339)},
	}
	if f := strings.TrimSpace(q.Filter); f != "" {
		v.Set("query", f)
	}
	switch m.endpoint {
	case "sessions":
		v.Set("interval", "1d")
		v.Set("includeSeries", "0")
	case "events":
		v.Set("dataset", "errors")
	}
	return c.base + "/api/0/organizations/" + c.org + "/" + m.endpoint + "/?" + v.Encode(), nil
}

// Baseline checks the token's scope, then reads the metric over the window.
func (c *Client) Baseline(ctx context.Context, q integrations.Query) (integrations.Baseline, error) {
	u, err := c.URL(q)
	if err != nil {
		return integrations.Baseline{}, err
	}
	scope, err := c.Scope(ctx)
	if err != nil {
		return integrations.Baseline{}, err
	}
	if err := scope.Check("sentry"); err != nil {
		return integrations.Baseline{}, err
	}
	m := metrics[q.Metric]
	var raw any
	switch m.endpoint {
	case "sessions":
		var res struct {
			Groups []struct {
				Totals map[string]any `json:"totals"`
			} `json:"groups"`
		}
		if err := c.get(ctx, u, &res); err != nil {
			return integrations.Baseline{}, err
		}
		if len(res.Groups) != 1 {
			return integrations.Baseline{}, fmt.Errorf("%w: sentry sessions returned %d groups; want one", integrations.ErrProvider, len(res.Groups))
		}
		raw = res.Groups[0].Totals[m.field]
	case "events":
		var res struct {
			Data []map[string]any `json:"data"`
		}
		if err := c.get(ctx, u, &res); err != nil {
			return integrations.Baseline{}, err
		}
		if len(res.Data) != 1 {
			return integrations.Baseline{}, fmt.Errorf("%w: sentry events returned %d rows; want one", integrations.ErrProvider, len(res.Data))
		}
		raw = res.Data[0][m.field]
	}
	// A crash-free rate is null when the window has no sessions: there is no
	// baseline to record, which is not the same as 0.
	v, ok := raw.(float64)
	if !ok {
		return integrations.Baseline{}, fmt.Errorf("%w: sentry %s is %v over the window; want a number (no data?)", integrations.ErrProvider, m.field, raw)
	}
	who := m.counts
	if f := strings.TrimSpace(q.Filter); f != "" {
		who += " where " + f
	}
	return integrations.Baseline{
		Metric:      q.Metric,
		Population:  fmt.Sprintf("Sentry %s project %s: %s", c.org, c.project, who),
		Window:      integrations.Window{From: q.Window.From.UTC(), To: q.Window.To.UTC()},
		Value:       v,
		SourceQuery: "GET " + u,
		FetchedAt:   c.now().UTC(),
	}, nil
}
