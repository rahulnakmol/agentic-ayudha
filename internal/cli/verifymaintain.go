package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/verify"
)

// failedChecks reads the behaviour floor's report among a run's artifacts:
// the checks that did not pass and were not merely unreachable.
func failedChecks(arts []verify.Evidence) (out []verify.CheckResult, ran bool) {
	for _, a := range arts {
		if a.Path != "reports/behaviour.json" {
			continue
		}
		var rep struct {
			Checks []verify.CheckResult `json:"checks"`
		}
		if json.Unmarshal(a.Data, &rep) != nil {
			continue
		}
		ran = true
		for _, c := range rep.Checks {
			if c.Status != verify.CheckPassed && c.Status != verify.CheckUnreachable {
				out = append(out, c)
			}
		}
	}
	return out, ran
}

// maintainHandoff is `verify --maintain` (WP4.5b.4): it posts the maintain
// brief on the tracking issue, @-mentioning [verify] maintainer, when mode is
// always or the map drifted or a check failed, and not twice for the same
// report. Outside CI it posts under the person's own account, because the
// apps answer only the people who connected them. A loop in Actions only
// records the brief, as the bot and with no mention: the user-side schedule
// wakes the agent (D-4.5b.7). It writes a line saying what it did to stderr.
func (e *Env) maintainHandoff(ctx context.Context, mode string, b verify.Brief) error {
	say := func(format string, args ...any) { fmt.Fprintf(e.Err, "maintain: "+format+"\n", args...) }
	switch {
	case b.Drift.Verdict == verify.DriftNone:
		say("no feature map here; nothing to hand off")
		return nil
	case mode == verify.MaintainDrift && !b.Needed():
		say("the maps match the source and no check failed; no hand-off")
		return nil
	}
	l, err := e.Config()
	if err != nil {
		return err
	}
	repo, err := e.RepoName(ctx)
	if err != nil {
		return err
	}
	owner, name, err := github.SplitRepo(repo)
	if err != nil {
		return Wrap(ExitUsage, err)
	}
	var c *github.Client
	if e.CI {
		c, _, err = e.BotGitHub(ctx, "")
	} else {
		c, _, err = e.HumanGitHub(ctx)
	}
	if err != nil {
		return err
	}
	issues, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.Issue, *gh.Response, error) {
		return c.REST.Issues.ListByRepo(ctx, owner, name, &gh.IssueListByRepoOptions{State: "open", Sort: "created", Direction: "asc", ListOptions: o})
	})
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	n := 0
	for _, is := range issues {
		if !is.IsPullRequest() && strings.Contains(is.GetBody(), verify.MaintainMarker) {
			n = is.GetNumber()
			break
		}
	}
	if n == 0 {
		body := verify.MaintainMarker + "\n`aayu verify --maintain` posts the feature maps' maintain brief here (docs/BUILD-PLAN.md WP4.5b.4): " +
			"the weekly pass, and the nightly sweep when a map drifted or a behaviour check failed. The agent app it mentions " +
			"(`[verify] maintainer`) answers with at most one pull request of map corrections, which goes through the normal merge gate. " +
			"No model key is used in this repository's CI.\n"
		rec, err := c.Write(ctx, actions.Action{Kind: "github.issue.create", Target: repo,
			Request: &actions.Request{Method: http.MethodPost, Path: "/repos/" + repo + "/issues"},
			Payload: map[string]any{"title": verify.MaintainTitle}}, func(ctx context.Context) (any, error) {
			is, _, err := c.REST.Issues.Create(ctx, owner, name, gh.CreateIssueRequest{Title: verify.MaintainTitle, Body: &body})
			return is.GetNumber(), err
		})
		if err != nil {
			return Wrap(ExitExternal, err)
		}
		if n, _ = rec.Result.(int); n == 0 {
			say("would open the tracking issue and post the brief (dry run)")
			return nil
		}
	}
	comments, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.IssueComment, *gh.Response, error) {
		return c.REST.Issues.ListComments(ctx, owner, name, n, &gh.IssueListCommentsOptions{ListOptions: o})
	})
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	// A record is compared with records, a hand-off with hand-offs, so the
	// Actions record of a drift never stops the person's run handing it off.
	last := ""
	for _, cm := range comments {
		if d, rec := verify.BriefDigest(cm.GetBody()); d != "" && rec == e.CI {
			last = d
		}
	}
	if mode == verify.MaintainDrift && last == b.Digest() {
		say("the same drift was handed off on #%d already; no new brief", n)
		return nil
	}
	who := l.Config.Verify.Maintainer
	body := b.Comment(who, e.CI)
	// A new comment each time: an edited comment notifies no one.
	if _, err := c.Write(ctx, actions.Action{Kind: "github.comment.verify-maintain", Target: fmt.Sprintf("%s#%d", repo, n),
		Request: &actions.Request{Method: http.MethodPost, Path: fmt.Sprintf("/repos/%s/issues/%d/comments", repo, n)},
		Payload: map[string]any{"maintainer": who, "digest": b.Digest(), "mode": mode}}, func(ctx context.Context) (any, error) {
		cm, _, err := c.REST.Issues.CreateComment(ctx, owner, name, n, &gh.IssueComment{Body: gh.Ptr(body)})
		return cm.GetID(), err
	}); err != nil {
		return Wrap(ExitExternal, err)
	}
	switch m := verify.Maintainers[who]; {
	case e.CI:
		say("brief recorded on #%d; the user-side schedule (amp -x or a Cursor automation) hands it to the maintainer", n)
	case m != "":
		say("brief posted on #%d for %s", n, m)
	case who == "amp":
		say("brief posted on #%d; Amp has no mention, so the verify-maintain loop's Amp thread reads it", n)
	default:
		say("brief posted on #%d with no mention: set [verify] maintainer (claude, codex, cursor, or amp)", n)
	}
	return nil
}
