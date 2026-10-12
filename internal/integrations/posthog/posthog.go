// Package posthog is the PostHog baseline adapter (WP5.2): it counts events or
// distinct persons over a window with one HogQL query, after checking that
// the personal API key carries only read scopes.
package posthog

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/integrations"
)

// DefaultURL is PostHog US Cloud's private API host; EU Cloud and self-hosted
// instances set [integrations.posthog] url.
const DefaultURL = "https://us.posthog.com"

// Metrics this adapter answers. events counts events matching the filter;
// users counts the distinct persons behind them.
var Metrics = []string{"events", "users"}

// Need lists the fields [integrations.posthog] must set.
var Need = []integrations.Field{{Key: "project", Want: "the PostHog project ID"}}

var projectID = regexp.MustCompile(`^[0-9]+$`)

// Client reads one PostHog project.
type Client struct {
	hc      *http.Client
	base    string
	project string
	key     string
	now     func() time.Time
}

// New returns a client for a resolved [integrations.posthog]. Only the HogQL
// query endpoint may receive a POST.
func New(c integrations.Conn, rt http.RoundTripper, now func() time.Time) (*Client, error) {
	if !projectID.MatchString(c.Project) {
		return nil, fmt.Errorf("%w: [integrations.%s] project = %q: want the numeric PostHog project ID", integrations.ErrNotConfigured, c.Name, c.Project)
	}
	query := regexp.MustCompile(`^/api/projects/` + c.Project + `/query/$`)
	return &Client{
		hc:   integrations.Client(rt, integrations.Allow{Method: http.MethodPost, Path: query}),
		base: c.BaseURL(DefaultURL), project: c.Project, key: c.Key, now: now,
	}, nil
}

func (c *Client) request(ctx context.Context, method, path string, body []byte) (*http.Request, error) {
	var req *http.Request
	var err error
	if body == nil {
		req, err = integrations.NewRequest(ctx, method, c.base+path, nil)
	} else {
		req, err = integrations.NewRequest(ctx, method, c.base+path, bytes.NewReader(body))
	}
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.key)
	return req, nil
}

// Scope reports the personal API key's scopes (GET /api/personal_api_keys/@current/).
// Any scope that is not "<object>:read" (including "*") allows writes.
func (c *Client) Scope(ctx context.Context) (integrations.Scope, error) {
	const path = "/api/personal_api_keys/@current/"
	req, err := c.request(ctx, http.MethodGet, path, nil)
	if err != nil {
		return integrations.Scope{}, err
	}
	var key struct {
		Label  string   `json:"label"`
		Scopes []string `json:"scopes"`
	}
	if err := integrations.DoJSON(c.hc, req, &key); err != nil {
		return integrations.Scope{}, err
	}
	s := integrations.Scope{Verified: true, Scopes: key.Scopes, Account: "personal API key " + key.Label, Endpoint: "GET " + path}
	for _, sc := range key.Scopes {
		if !strings.HasSuffix(sc, ":read") {
			s.Write = append(s.Write, sc)
		}
	}
	return s, nil
}

// HogQL returns the query a baseline runs.
func HogQL(q integrations.Query) (string, error) {
	var sel string
	switch q.Metric {
	case "events":
		sel = "count()"
	case "users":
		sel = "count(DISTINCT person_id)"
	default:
		return "", fmt.Errorf("%w: posthog metric %q: want one of %s", integrations.ErrQuery, q.Metric, strings.Join(Metrics, ", "))
	}
	if err := q.Window.Check(); err != nil {
		return "", err
	}
	const ts = "2006-01-02 15:04:05"
	where := fmt.Sprintf("timestamp >= toDateTime('%s', 'UTC') AND timestamp < toDateTime('%s', 'UTC')",
		q.Window.From.UTC().Format(ts), q.Window.To.UTC().Format(ts))
	if f := strings.TrimSpace(q.Filter); f != "" {
		where += " AND (" + f + ")"
	}
	return "SELECT " + sel + " FROM events WHERE " + where, nil
}

// Baseline checks the key's scope, then runs the metric's HogQL query.
func (c *Client) Baseline(ctx context.Context, q integrations.Query) (integrations.Baseline, error) {
	hogql, err := HogQL(q)
	if err != nil {
		return integrations.Baseline{}, err
	}
	scope, err := c.Scope(ctx)
	if err != nil {
		return integrations.Baseline{}, err
	}
	if err := scope.Check("posthog"); err != nil {
		return integrations.Baseline{}, err
	}
	path := "/api/projects/" + c.project + "/query/"
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false) // keep < and > readable in source_query
	_ = enc.Encode(map[string]any{
		"query": map[string]string{"kind": "HogQLQuery", "query": hogql},
		"name":  "aayudha baseline " + q.Metric,
	})
	body := bytes.TrimSpace(buf.Bytes())
	req, err := c.request(ctx, http.MethodPost, path, body)
	if err != nil {
		return integrations.Baseline{}, err
	}
	var res struct {
		Results [][]any `json:"results"`
	}
	if err := integrations.DoJSON(c.hc, req, &res); err != nil {
		return integrations.Baseline{}, err
	}
	if len(res.Results) != 1 || len(res.Results[0]) != 1 {
		return integrations.Baseline{}, fmt.Errorf("%w: posthog query returned %d rows; want one row with one value", integrations.ErrProvider, len(res.Results))
	}
	v, ok := res.Results[0][0].(float64)
	if !ok {
		return integrations.Baseline{}, fmt.Errorf("%w: posthog query returned %v; want a number", integrations.ErrProvider, res.Results[0][0])
	}
	who := "events"
	if q.Metric == "users" {
		who = "distinct persons with events"
	}
	if f := strings.TrimSpace(q.Filter); f != "" {
		who += " where " + f
	}
	return integrations.Baseline{
		Metric:      q.Metric,
		Population:  fmt.Sprintf("PostHog project %s: %s", c.project, who),
		Window:      integrations.Window{From: q.Window.From.UTC(), To: q.Window.To.UTC()},
		Value:       v,
		SourceQuery: "POST " + c.base + path + " " + string(body),
		FetchedAt:   c.now().UTC(),
	}, nil
}
