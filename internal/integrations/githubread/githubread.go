// Package githubread is the GitHub read baseline adapter (WP5.2): issue and
// pull-request counts over a window from the search API, through the WP2.3
// client (rate limits, call budget, and no way to write without an actions
// log), with its own read-only key from [integrations.github].
package githubread

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/identity"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations"
)

// DefaultURL is the GitHub REST API (GHES is unsupported in 0.1).
const DefaultURL = "https://api.github.com"

// metric is a search over a window: the qualifiers that pick the items and
// the date field the window applies to.
type metric struct {
	is, date, counts string
}

var metrics = map[string]metric{
	"issues_opened": {"is:issue", "created", "issues opened"},
	"issues_closed": {"is:issue is:closed", "closed", "issues closed"},
	"prs_opened":    {"is:pr", "created", "pull requests opened"},
	"prs_merged":    {"is:pr is:merged", "merged", "pull requests merged"},
}

// Metrics this adapter answers.
var Metrics = []string{"issues_opened", "issues_closed", "prs_opened", "prs_merged"}

// Client reads one repository.
type Client struct {
	gh   *github.Client
	hc   *http.Client
	base string
	repo string
	key  string
	now  func() time.Time
}

// New returns a client for a resolved [integrations.github]; repo is the
// repository to read (its project, else the current repository).
func New(c integrations.Conn, repo string, rt http.RoundTripper, now func() time.Time) (*Client, error) {
	if _, _, err := github.SplitRepo(repo); err != nil {
		return nil, fmt.Errorf("%w: [integrations.%s] project = %q: want owner/name (or run inside a repository)", integrations.ErrNotConfigured, c.Name, repo)
	}
	base := c.BaseURL(DefaultURL)
	ro := integrations.ReadOnly(rt)
	client, err := github.New(github.Options{Token: c.Key, BaseURL: base, Transport: ro, Budget: 10})
	if err != nil {
		return nil, err
	}
	return &Client{gh: client, hc: &http.Client{Transport: ro, Timeout: time.Minute}, base: base, repo: repo, key: c.Key, now: now}, nil
}

// Scope reports a classic token's scopes from X-OAuth-Scopes on GET /user.
// Only read:* scopes and user:email are read-only. Fine-grained and App
// tokens have no scope header, so their scope cannot be verified here.
func (c *Client) Scope(ctx context.Context) (integrations.Scope, error) {
	u, err := identity.WhoAmI(ctx, c.hc, c.base, identity.Token{Value: c.key, Source: "[integrations.github] secret"})
	if err != nil {
		return integrations.Scope{}, fmt.Errorf("%w: %w", integrations.ErrProvider, err)
	}
	s := integrations.Scope{Verified: u.Listed, Scopes: u.Scopes, Account: u.Login, Endpoint: "GET /user"}
	for _, sc := range u.Scopes {
		if !strings.HasPrefix(sc, "read:") && sc != "user:email" {
			s.Write = append(s.Write, sc)
		}
	}
	return s, nil
}

// Search returns the search query a baseline runs.
func (c *Client) Search(q integrations.Query) (string, error) {
	m, ok := metrics[q.Metric]
	if !ok {
		return "", fmt.Errorf("%w: github metric %q: want one of %s", integrations.ErrQuery, q.Metric, strings.Join(Metrics, ", "))
	}
	if err := q.Window.Check(); err != nil {
		return "", err
	}
	s := fmt.Sprintf("repo:%s %s %s:>=%s %s:<%s", c.repo, m.is,
		m.date, q.Window.From.UTC().Format(time.RFC3339), m.date, q.Window.To.UTC().Format(time.RFC3339))
	if f := strings.TrimSpace(q.Filter); f != "" {
		s += " " + f
	}
	return s, nil
}

// Baseline checks the token's scope, then counts the metric's search results.
func (c *Client) Baseline(ctx context.Context, q integrations.Query) (integrations.Baseline, error) {
	search, err := c.Search(q)
	if err != nil {
		return integrations.Baseline{}, err
	}
	scope, err := c.Scope(ctx)
	if err != nil {
		return integrations.Baseline{}, err
	}
	if err := scope.Check("github"); err != nil {
		return integrations.Baseline{}, err
	}
	res, _, err := c.gh.REST.Search.Issues(ctx, search, &gh.SearchOptions{ListOptions: gh.ListOptions{PerPage: 1}})
	if err != nil {
		return integrations.Baseline{}, fmt.Errorf("%w: GET /search/issues: %w", integrations.ErrProvider, err)
	}
	if res.GetIncompleteResults() {
		return integrations.Baseline{}, fmt.Errorf("%w: GitHub search timed out and reported incomplete results for %q; retry", integrations.ErrProvider, search)
	}
	who := metrics[q.Metric].counts
	if f := strings.TrimSpace(q.Filter); f != "" {
		who += " where " + f
	}
	return integrations.Baseline{
		Metric:      q.Metric,
		Population:  fmt.Sprintf("GitHub %s: %s", c.repo, who),
		Window:      integrations.Window{From: q.Window.From.UTC(), To: q.Window.To.UTC()},
		Value:       float64(res.GetTotal()),
		SourceQuery: "GET " + c.base + "/search/issues?" + url.Values{"q": {search}}.Encode(),
		FetchedAt:   c.now().UTC(),
	}, nil
}
