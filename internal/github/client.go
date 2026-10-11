// Package github is aayudha's GitHub API client (WP2.3): REST through go-github
// for issues, sub-issues, issue types, labels, comments, refs, and rulesets, and
// GraphQL through githubv4 only for Projects v2. One transport stack adds an
// ETag cache, primary and secondary rate-limit handling with serialized writes,
// a per-verb call budget, the actions capability guard, and a refusal of any
// mutating request that does not go through internal/actions (ground rule 6).
// GHES is unsupported in 0.1.
package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	gh "github.com/google/go-github/v90/github"
	"github.com/shurcooL/githubv4"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
)

// DefaultBudget is the call budget of one verb run when Options.Budget is 0.
const DefaultBudget = 2000

// Errors.
var (
	// ErrBudget means the run used its call budget; nothing further is sent.
	ErrBudget = errors.New("GitHub API call budget for this run is used up")
	// ErrUnlogged means code tried a GitHub write outside Client.Write, i.e.
	// outside internal/actions (ground rule 6). It is a bug in aayudha.
	ErrUnlogged = errors.New("GitHub write attempted outside internal/actions")
	// ErrRateLimited means GitHub's rate limit would take longer than
	// Options.MaxWait to clear.
	ErrRateLimited = errors.New("GitHub rate limit")
)

// Options configure a client. Zero values are production defaults.
type Options struct {
	Token      string
	BaseURL    string            // REST base, default https://api.github.com/ (tests point it at a fake)
	Transport  http.RoundTripper // innermost transport; default http.DefaultTransport
	Log        *actions.Log      // every write is logged here; required for writes
	Identity   string            // stamped on actions: human, bot, or human-fallback
	Capability string            // e.g. actions.CapLabelsOnly; enforced on every request
	Budget     int               // maximum API calls (including 304s) for this run; 0 means DefaultBudget
	WriteGap   time.Duration     // minimum gap between mutating calls; default 1s (GitHub's guidance), negative disables
	MaxWait    time.Duration     // longest rate-limit wait before failing; default 2 minutes
	Retries    int               // rate-limit retries per request; default 3
	Sleep      func(ctx context.Context, d time.Duration) error
	Now        func() time.Time
}

// Stats counts the calls of one run; verbs report it in --json.
type Stats struct {
	REST       int   `json:"rest"`
	GraphQL    int   `json:"graphql"`
	NotChanged int   `json:"not_modified"` // 304 responses served from the ETag cache
	Retries    int   `json:"retries"`
	WaitedMS   int64 `json:"rate_limit_wait_ms"`
	Budget     int   `json:"budget"`
}

// Calls is the number of requests sent.
func (s Stats) Calls() int { return s.REST + s.GraphQL }

// Client is the GitHub client of one verb run.
type Client struct {
	REST    *gh.Client
	GraphQL *githubv4.Client

	opts  Options
	mu    sync.Mutex
	stats Stats
}

// New builds a client.
func New(opts Options) (*Client, error) {
	if opts.Budget == 0 {
		opts.Budget = DefaultBudget
	}
	if opts.WriteGap == 0 {
		opts.WriteGap = time.Second
	}
	if opts.MaxWait == 0 {
		opts.MaxWait = 2 * time.Minute
	}
	if opts.Retries == 0 {
		opts.Retries = 3
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Sleep == nil {
		opts.Sleep = func(ctx context.Context, d time.Duration) error {
			t := time.NewTimer(d)
			defer t.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-t.C:
				return nil
			}
		}
	}
	if opts.Transport == nil {
		opts.Transport = http.DefaultTransport
	}
	c := &Client{opts: opts}
	c.stats.Budget = opts.Budget
	hc := &http.Client{Transport: &transport{c: c, cache: newCache(), next: opts.Transport}}
	ghOpts := []gh.ClientOptionsFunc{gh.WithHTTPClient(hc), gh.WithDisableRateLimitCheck(), gh.WithUserAgent("aayudha")}
	graphqlURL := "https://api.github.com/graphql"
	if opts.BaseURL != "" {
		base := strings.TrimSuffix(opts.BaseURL, "/") + "/"
		ghOpts = append(ghOpts, gh.WithURLs(&base, &base))
		graphqlURL = base + "graphql"
	}
	var err error
	if c.REST, err = gh.NewClient(ghOpts...); err != nil {
		return nil, err
	}
	c.GraphQL = githubv4.NewEnterpriseClient(graphqlURL, hc)
	return c, nil
}

// Stats returns the call counts so far.
func (c *Client) Stats() Stats {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stats
}

// DryRun reports whether writes are only planned.
func (c *Client) DryRun() bool { return c.opts.Log != nil && c.opts.Log.DryRun() }

// Identity is the identity stamped on this client's writes.
func (c *Client) Identity() string { return c.opts.Identity }

type writeKey struct{}

// Write performs one GitHub write as an action: a is logged through
// actions.Do (which applies --dry-run, the idempotency key, the capability,
// and redaction), and perform runs with a context that lets its mutating
// requests through the transport. In a dry run perform never runs.
func (c *Client) Write(ctx context.Context, a actions.Action, perform func(ctx context.Context) (any, error)) (actions.Record, error) {
	if c.opts.Log == nil {
		return actions.Record{}, fmt.Errorf("%w: client has no actions log", ErrUnlogged)
	}
	if a.Identity == "" {
		a.Identity = c.opts.Identity
	}
	if a.Capability == "" {
		a.Capability = c.opts.Capability
	}
	return c.opts.Log.Do(ctx, a, func(ctx context.Context) (actions.Outcome, error) {
		res, err := perform(context.WithValue(ctx, writeKey{}, true))
		return actions.Outcome{Result: res}, err
	})
}

// SplitRepo splits owner/name.
func SplitRepo(repo string) (owner, name string, err error) {
	owner, name, ok := strings.Cut(repo, "/")
	if !ok || owner == "" || name == "" || strings.Contains(name, "/") {
		return "", "", fmt.Errorf("repository %q: want owner/name", repo)
	}
	return owner, name, nil
}

// All collects every page of a REST list call, 100 items per page.
func All[T any](ctx context.Context, list func(ctx context.Context, opts gh.ListOptions) ([]T, *gh.Response, error)) ([]T, error) {
	return AllPer(ctx, 100, list)
}

// AllPer is All with an explicit page size.
func AllPer[T any](ctx context.Context, perPage int, list func(ctx context.Context, opts gh.ListOptions) ([]T, *gh.Response, error)) ([]T, error) {
	var out []T
	opts := gh.ListOptions{PerPage: perPage}
	for {
		items, resp, err := list(ctx, opts)
		if err != nil {
			return out, err
		}
		out = append(out, items...)
		if resp == nil || resp.NextPage == 0 {
			return out, nil
		}
		opts.Page = resp.NextPage
	}
}

// NotFound reports whether err is a GitHub 404.
func NotFound(err error) bool {
	var er *gh.ErrorResponse
	return errors.As(err, &er) && er.Response != nil && er.Response.StatusCode == http.StatusNotFound
}

// External reports whether err came from GitHub or the network rather than
// from aayudha: callers map it to exit code 5.
func External(err error) bool {
	var er *gh.ErrorResponse
	var rl *gh.RateLimitError
	var ab *gh.AbuseRateLimitError
	return errors.As(err, &er) || errors.As(err, &rl) || errors.As(err, &ab) ||
		errors.Is(err, ErrRateLimited) || errors.Is(err, ErrBudget)
}

// UpsertComment keeps one comment per marker on an issue, replaced in place:
// it edits the comment whose body contains marker, or creates one, and does
// nothing when the body is already current. It reports whether it wrote.
func (c *Client) UpsertComment(ctx context.Context, repo string, number int, marker, body string, a actions.Action) (bool, error) {
	owner, name, err := SplitRepo(repo)
	if err != nil {
		return false, err
	}
	comments, err := All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.IssueComment, *gh.Response, error) {
		return c.REST.Issues.ListComments(ctx, owner, name, number, &gh.IssueListCommentsOptions{ListOptions: o})
	})
	if err != nil {
		return false, err
	}
	a.Target = fmt.Sprintf("%s#%d", repo, number)
	if a.Payload == nil {
		a.Payload = map[string]any{}
	}
	a.Payload["marker"] = marker
	for _, cm := range comments {
		if !strings.Contains(cm.GetBody(), marker) {
			continue
		}
		if cm.GetBody() == body {
			return false, nil
		}
		a.Request = &actions.Request{Method: http.MethodPatch, Path: fmt.Sprintf("/repos/%s/issues/comments/%d", repo, cm.GetID())}
		_, err := c.Write(ctx, a, func(ctx context.Context) (any, error) {
			_, _, err := c.REST.Issues.EditComment(ctx, owner, name, cm.GetID(), &gh.IssueComment{Body: gh.Ptr(body)})
			return cm.GetID(), err
		})
		return true, err
	}
	a.Request = &actions.Request{Method: http.MethodPost, Path: fmt.Sprintf("/repos/%s/issues/%d/comments", repo, number)}
	_, err = c.Write(ctx, a, func(ctx context.Context) (any, error) {
		cm, _, err := c.REST.Issues.CreateComment(ctx, owner, name, number, &gh.IssueComment{Body: gh.Ptr(body)})
		return cm.GetID(), err
	})
	return true, err
}

// IssueLabels returns an issue's label names.
func IssueLabels(is *gh.Issue) []string {
	out := make([]string, 0, len(is.Labels))
	for _, l := range is.Labels {
		out = append(out, l.GetName())
	}
	return out
}
