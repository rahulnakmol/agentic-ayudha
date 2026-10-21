package cli

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
)

// A story lease (WP5.5): `build` holds one per story while it works, as the
// story's assignee plus a lock comment that names the holder and an expiry,
// and the label that lets `gc` find leases. Every build run renews it;
// another holder is refused until it expires; `gc` reclaims expired ones;
// `ship` releases it.
const (
	LeaseLabel = "aayudha:leased"
	LeaseTTL   = 4 * time.Hour
)

var (
	leaseTag    = regexp.MustCompile(`<!-- aayudha:lease holder=(\S+) expires=(\S+) -->`)
	releasedTag = regexp.MustCompile(`<!-- aayudha:lease-released expires=(\S+) -->`)
)

// Lease is the lock comment of a story.
type Lease struct {
	Holder    string    `json:"holder"` // <login>@<host>
	Expires   time.Time `json:"expires"`
	CommentID int64     `json:"comment_id"`
	Assignee  string    `json:"assignee,omitempty"`
}

// readLease finds a story's live lock comment; nil when there is none.
func readLease(ctx context.Context, c *github.Client, owner, name string, n int) (*Lease, error) {
	cs, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.IssueComment, *gh.Response, error) {
		return c.REST.Issues.ListComments(ctx, owner, name, n, &gh.IssueListCommentsOptions{ListOptions: o})
	})
	if err != nil {
		return nil, err
	}
	for _, cm := range cs {
		if m := leaseTag.FindStringSubmatch(cm.GetBody()); m != nil {
			exp, err := time.Parse(time.RFC3339, m[2])
			if err != nil {
				continue
			}
			holder := m[1]
			login, _, _ := strings.Cut(holder, "@")
			return &Lease{Holder: holder, Expires: exp, CommentID: cm.GetID(), Assignee: login}, nil
		}
	}
	return nil, nil
}

// releasedLeaseExpiry is when the newest released lease of story n was due
// to expire; zero when no released lock comment records one.
func releasedLeaseExpiry(ctx context.Context, c *github.Client, owner, name string, n int) (time.Time, error) {
	cs, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.IssueComment, *gh.Response, error) {
		return c.REST.Issues.ListComments(ctx, owner, name, n, &gh.IssueListCommentsOptions{ListOptions: o})
	})
	var last time.Time
	for _, cm := range cs {
		if m := releasedTag.FindStringSubmatch(cm.GetBody()); m != nil {
			if exp, err := time.Parse(time.RFC3339, m[1]); err == nil && exp.After(last) {
				last = exp
			}
		}
	}
	return last, err
}

// leaseHolder names this build: the writing login and the machine.
func leaseHolder(login string) string {
	host, _ := os.Hostname()
	if host == "" {
		host = "unknown-host"
	}
	if login == "" {
		login = "unknown"
	}
	return login + "@" + strings.ReplaceAll(host, " ", "-")
}

func leaseBody(holder string, exp time.Time) string {
	return fmt.Sprintf("<!-- aayudha:lease holder=%s expires=%s -->\n**Leased for `aayu build`** by `%s` until %s. Every build run renews it; another builder is refused until it expires, and `aayu gc` reclaims it once it has.",
		holder, exp.UTC().Format(time.RFC3339), holder, exp.UTC().Format(time.RFC3339))
}

// takeLease takes or renews the lease on story n for holder; it refuses
// (exit 4) while another holder's lease is live.
func (e *Env) takeLease(ctx context.Context, c *github.Client, repo string, n int, holder string, labels []string) (Lease, error) {
	owner, name, _ := github.SplitRepo(repo)
	cur, err := readLease(ctx, c, owner, name, n)
	if err != nil {
		return Lease{}, Wrap(ExitExternal, err)
	}
	now := e.Now()
	if cur != nil && cur.Holder != holder && cur.Expires.After(now) {
		return *cur, Errorf(ExitBlocked, "build #%d: leased by %s until %s; wait for it, or for `aayu gc` to reclaim it after it expires", n, cur.Holder, cur.Expires.UTC().Format(time.RFC3339))
	}
	l := Lease{Holder: holder, Expires: now.Add(LeaseTTL)}
	body := leaseBody(holder, l.Expires)
	target := fmt.Sprintf("%s#%d", repo, n)
	if cur != nil {
		l.CommentID = cur.CommentID
		if _, err := c.Write(ctx, actions.Action{Kind: "github.issue.comment.edit", Target: target,
			Request: &actions.Request{Method: http.MethodPatch, Path: fmt.Sprintf("/repos/%s/issues/comments/%d", repo, cur.CommentID)}, Payload: map[string]any{"lease": holder, "expires": l.Expires}},
			func(ctx context.Context) (any, error) {
				_, _, err := c.REST.Issues.EditComment(ctx, owner, name, cur.CommentID, &gh.IssueComment{Body: &body})
				return nil, err
			}); err != nil {
			return l, Wrap(ExitExternal, err)
		}
	} else {
		if _, err := c.Write(ctx, actions.Action{Kind: "github.issue.comment", Target: target,
			Request: &actions.Request{Method: http.MethodPost, Path: fmt.Sprintf("/repos/%s/issues/%d/comments", repo, n)}, Payload: map[string]any{"lease": holder, "expires": l.Expires}},
			func(ctx context.Context) (any, error) {
				cm, _, err := c.REST.Issues.CreateComment(ctx, owner, name, n, &gh.IssueComment{Body: &body})
				l.CommentID = cm.GetID()
				return nil, err
			}); err != nil {
			return l, Wrap(ExitExternal, err)
		}
	}
	if !slices.Contains(labels, LeaseLabel) {
		if _, err := c.Write(ctx, actions.Action{Kind: "github.issue.labels.add", Target: target,
			Request: &actions.Request{Method: http.MethodPost, Path: fmt.Sprintf("/repos/%s/issues/%d/labels", repo, n)}, Payload: map[string]any{"labels": []string{LeaseLabel}}},
			func(ctx context.Context) (any, error) {
				_, _, err := c.REST.Issues.AddLabelsToIssue(ctx, owner, name, n, []string{LeaseLabel})
				return nil, err
			}); err != nil {
			return l, Wrap(ExitExternal, err)
		}
	}
	// The holder's login is the assignee; an App's [bot] login cannot be assigned.
	if login, _, _ := strings.Cut(holder, "@"); login != "unknown" && !strings.HasSuffix(login, "[bot]") && (cur == nil || cur.Holder != holder) {
		l.Assignee = login
		if _, err := c.Write(ctx, actions.Action{Kind: "github.issue.assignees.add", Target: target,
			Request: &actions.Request{Method: http.MethodPost, Path: fmt.Sprintf("/repos/%s/issues/%d/assignees", repo, n)}, Payload: map[string]any{"assignees": []string{login}}},
			func(ctx context.Context) (any, error) {
				_, _, err := c.REST.Issues.AddAssignees(ctx, owner, name, n, []string{login})
				return nil, err
			}); err != nil {
			return l, Wrap(ExitExternal, err)
		}
	}
	return l, nil
}

// releaseLease ends a lease: the lock comment says why, the label goes, and
// the holder is unassigned. It does nothing when the story has no lease.
func (e *Env) releaseLease(ctx context.Context, c *github.Client, repo string, n int, why string) (*Lease, error) {
	owner, name, _ := github.SplitRepo(repo)
	cur, err := readLease(ctx, c, owner, name, n)
	if err != nil || cur == nil {
		return nil, err
	}
	target := fmt.Sprintf("%s#%d", repo, n)
	exp := cur.Expires.UTC().Format(time.RFC3339)
	body := fmt.Sprintf("<!-- aayudha:lease-released expires=%s -->\n**Lease released** (held by `%s`, expiring %s): %s.", exp, cur.Holder, exp, why)
	if _, err := c.Write(ctx, actions.Action{Kind: "github.issue.comment.edit", Target: target,
		Request: &actions.Request{Method: http.MethodPatch, Path: fmt.Sprintf("/repos/%s/issues/comments/%d", repo, cur.CommentID)}, Payload: map[string]any{"lease": cur.Holder, "released": why}},
		func(ctx context.Context) (any, error) {
			_, _, err := c.REST.Issues.EditComment(ctx, owner, name, cur.CommentID, &gh.IssueComment{Body: &body})
			return nil, err
		}); err != nil {
		return cur, err
	}
	if _, err := c.Write(ctx, actions.Action{Kind: "github.issue.labels.remove", Target: target,
		Request: &actions.Request{Method: http.MethodDelete, Path: fmt.Sprintf("/repos/%s/issues/%d/labels/%s", repo, n, LeaseLabel)}},
		func(ctx context.Context) (any, error) {
			_, err := c.REST.Issues.RemoveLabelForIssue(ctx, owner, name, n, LeaseLabel)
			if github.NotFound(err) {
				err = nil
			}
			return nil, err
		}); err != nil {
		return cur, err
	}
	if cur.Assignee != "" && !strings.HasSuffix(cur.Assignee, "[bot]") && cur.Assignee != "unknown" {
		if _, err := c.Write(ctx, actions.Action{Kind: "github.issue.assignees.remove", Target: target,
			Request: &actions.Request{Method: http.MethodDelete, Path: fmt.Sprintf("/repos/%s/issues/%d/assignees", repo, n)}, Payload: map[string]any{"assignees": []string{cur.Assignee}}},
			func(ctx context.Context) (any, error) {
				_, _, err := c.REST.Issues.RemoveAssignees(ctx, owner, name, n, []string{cur.Assignee})
				return nil, err
			}); err != nil {
			return cur, err
		}
	}
	return cur, nil
}
