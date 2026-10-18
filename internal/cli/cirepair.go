package cli

import (
	"context"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/delegate"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
)

// tailLines is how much of a failed job's log a CI follow-up quotes.
const tailLines = 60

// failedRequired returns the required checks that failed on head, newest run
// of each, with the tail of its Actions job log: the merge gate's checks and
// [verify] upstream_checks, but never protected-paths (it fails by design) or
// inspect (its findings have their own follow-up).
func failedRequired(ctx context.Context, c *github.Client, owner, name, head string, upstream []string) ([]delegate.FailedCheck, error) {
	required := slices.DeleteFunc(append(slices.Clone(gateChecks), upstream...), func(n string) bool { return n == "aayudha/inspect" })
	rs, _, err := c.REST.Checks.ListCheckRunsForRef(ctx, owner, name, head, &gh.ListCheckRunsOptions{Filter: gh.Ptr("latest"), ListOptions: gh.ListOptions{PerPage: 100}})
	if err != nil {
		return nil, err
	}
	var out []delegate.FailedCheck
	seen := map[string]bool{}
	for _, r := range rs.CheckRuns {
		n := r.GetName()
		if seen[n] || n == "protected-paths" || !slices.Contains(required, n) {
			continue
		}
		seen[n] = true
		if r.GetStatus() == "completed" && (r.GetConclusion() == "failure" || r.GetConclusion() == "timed_out") {
			out = append(out, delegate.FailedCheck{Name: n, URL: r.GetHTMLURL(), Tail: jobTail(ctx, c, owner, name, r.GetID())})
		}
	}
	return out, nil
}

// jobTail is the last tailLines lines of an Actions job's log (a check run's
// ID is its job's), or "" when the log cannot be read.
func jobTail(ctx context.Context, c *github.Client, owner, name string, job int64) string {
	b := jobLog(ctx, c, owner, name, job)
	if b == "" {
		return ""
	}
	lines := strings.Split(strings.TrimRight(b, "\n"), "\n")
	if len(lines) > tailLines {
		lines = lines[len(lines)-tailLines:]
	}
	return strings.Join(lines, "\n")
}

// jobLog is an Actions job's log (up to 8 MiB), or "" when it cannot be read.
func jobLog(ctx context.Context, c *github.Client, owner, name string, job int64) string {
	u, _, err := c.REST.Actions.GetWorkflowJobLogs(ctx, owner, name, job, 2)
	if err != nil {
		return ""
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return ""
	}
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return ""
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	return string(b)
}
