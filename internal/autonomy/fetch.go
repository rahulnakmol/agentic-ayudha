package autonomy

import (
	"context"
	"slices"
	"strings"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/github"
)

// MaxPRs is how many merged PRs Fetch reads, newest first. Twenty
// correction-free merges per layer is the default threshold, so 100 merges
// cover it for every layer a typical story touches.
const MaxPRs = 100

// Fetch reads the newest merged PRs (up to limit) and, for the agent PRs among
// them, their files, reviews, commits, comments, and review rung (GateRung);
// for later revert PRs their commits; and the Bug and Incident issues opened
// since the oldest of them.
func Fetch(ctx context.Context, c *github.Client, repo string, r Rules, limit int) ([]PR, []Issue, error) {
	owner, name, err := github.SplitRepo(repo)
	if err != nil {
		return nil, nil, err
	}
	var prs []PR
	opts := &gh.PullRequestListOptions{State: "closed", Sort: "updated", Direction: "desc", ListOptions: gh.ListOptions{PerPage: 100}}
	for len(prs) < limit {
		page, resp, err := c.REST.PullRequests.List(ctx, owner, name, opts)
		if err != nil {
			return nil, nil, err
		}
		for _, p := range page {
			if p.MergedAt == nil || len(prs) >= limit {
				continue
			}
			pr := PR{Number: p.GetNumber(), Title: p.GetTitle(), Body: p.GetBody(), Author: p.GetUser().GetLogin(),
				AuthorBot: p.GetUser().GetType() == "Bot", Head: p.GetHead().GetSHA(), MergedAt: p.GetMergedAt().UTC(), MergeSHA: p.GetMergeCommitSHA()}
			for _, l := range p.Labels {
				pr.Labels = append(pr.Labels, l.GetName())
			}
			prs = append(prs, pr)
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	for i := range prs {
		p := &prs[i]
		agent := r.Agent(*p)
		if agent || strings.HasPrefix(strings.ToLower(p.Title), "revert") {
			commits, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.RepositoryCommit, *gh.Response, error) {
				return c.REST.PullRequests.ListCommits(ctx, owner, name, p.Number, &o)
			})
			if err != nil {
				return nil, nil, err
			}
			for _, cm := range commits {
				p.Commits = append(p.Commits, Commit{SHA: cm.GetSHA(), Login: cm.GetAuthor().GetLogin(), Bot: cm.GetAuthor().GetType() == "Bot",
					Message: cm.GetCommit().GetMessage(), Parents: len(cm.Parents)})
			}
		}
		if !agent {
			continue
		}
		files, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.CommitFile, *gh.Response, error) {
			return c.REST.PullRequests.ListFiles(ctx, owner, name, p.Number, &o)
		})
		if err != nil {
			return nil, nil, err
		}
		for _, f := range files {
			p.Files = append(p.Files, f.GetFilename())
			if f.GetPreviousFilename() != "" {
				p.Files = append(p.Files, f.GetPreviousFilename())
			}
		}
		reviews, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.PullRequestReview, *gh.Response, error) {
			return c.REST.PullRequests.ListReviews(ctx, owner, name, p.Number, &o)
		})
		if err != nil {
			return nil, nil, err
		}
		for _, rv := range reviews {
			p.Reviews = append(p.Reviews, Review{Login: rv.GetUser().GetLogin(), Bot: rv.GetUser().GetType() == "Bot", State: rv.GetState(), At: rv.GetSubmittedAt().UTC()})
		}
		comments, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.IssueComment, *gh.Response, error) {
			return c.REST.Issues.ListComments(ctx, owner, name, p.Number, &gh.IssueListCommentsOptions{ListOptions: o})
		})
		if err != nil {
			return nil, nil, err
		}
		for _, cm := range comments {
			p.Comments = append(p.Comments, Comment{Body: cm.GetBody(), At: cm.GetCreatedAt().UTC()})
		}
		p.Rung = GateRung(ctx, c, owner, name, p.Number, p.Head)
	}
	if len(prs) == 0 {
		return prs, nil, nil
	}
	oldest := slices.MinFunc(prs, func(a, b PR) int { return a.MergedAt.Compare(b.MergedAt) }).MergedAt
	var issues []Issue
	for _, kind := range []string{"bug", "incident"} {
		list, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.Issue, *gh.Response, error) {
			return c.REST.Issues.ListByRepo(ctx, owner, name, &gh.IssueListByRepoOptions{State: "all", Labels: []string{"type:" + kind}, Since: oldest, ListOptions: o})
		})
		if err != nil {
			return nil, nil, err
		}
		for _, is := range list {
			if !is.IsPullRequest() {
				issues = append(issues, Issue{Number: is.GetNumber(), Kind: kind, Body: is.GetBody(), CreatedAt: is.GetCreatedAt().UTC()})
			}
		}
	}
	return prs, issues, nil
}
