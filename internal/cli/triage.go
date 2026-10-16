package cli

import (
	"context"
	"errors"
	"fmt"
	"io"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/models"
	"github.com/rahulnakmol/agentic-ayudha/internal/triage"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

func init() { register("triage", runTriage) }

// Gateway is the model gateway (WP1.6) for direct model calls of this run.
func (e *Env) Gateway() (*models.Gateway, error) {
	l, err := e.Config()
	if err != nil {
		return nil, err
	}
	log, err := e.Actions()
	if err != nil {
		return nil, err
	}
	clock, err := l.Config.Clock()
	if err != nil {
		return nil, Wrap(ExitUsage, err)
	}
	n, err := e.Notifier(context.Background())
	if err != nil {
		return nil, err
	}
	// The breaker starts from the spend every process logged in this clone's
	// actions log, so the budgets are not per process (Bug #177).
	br := &models.Breaker{Budget: l.Config.Budget, Clock: clock, Now: e.Now, PauseFile: e.PauseFile(), Notify: n}
	if err := br.SeedLog(e.ActionsPath()); err != nil {
		return nil, Wrap(ExitInternal, err)
	}
	m := l.Config.Models
	return &models.Gateway{
		Models: m, Actions: log, HTTP: e.HTTP, Client: e.ModelClient,
		Resolver: models.Resolver{Models: m, Log: func(s string) { e.Logf("%s", s) }},
		Breaker:  br,
		Key: func(name string) (string, error) {
			v, _, err := e.SecretStore().Get(name)
			return v, err
		},
	}, nil
}

// triageClient is the labels-only client triage writes with: in CI the bot
// App token when the workflow minted one (issues: write only), else the
// workflow's own token; on a laptop the bot, or the tagged human on day one.
func (e *Env) triageClient(ctx context.Context) (*github.Client, error) {
	if e.CI && e.Getenv("AAYU_BOT_TOKEN") == "" {
		tok, err := e.Resolver().Human(ctx)
		if err != nil {
			return nil, Wrap(ExitBlocked, err)
		}
		return e.newGitHub(tok, "ci", actions.CapLabelsOnly)
	}
	c, _, err := e.BotGitHub(ctx, actions.CapLabelsOnly)
	return c, err
}

func runTriage(ctx context.Context, e *Env, in Values) error {
	repo, err := e.RepoName(ctx)
	if err != nil {
		return err
	}
	if in.String("issue") == "" {
		if in.String("as") != "" {
			return Errorf(ExitUsage, "triage --as: name the intake issue it classifies")
		}
		return listIntake(ctx, e, repo)
	}
	c, err := e.triageClient(ctx)
	if err != nil {
		return err
	}
	t := &triage.Triage{GH: c, Repo: repo, Classify: func(ctx context.Context, n int, title, body string) (triage.Class, error) {
		gw, err := e.Gateway()
		if err != nil {
			return triage.Class{}, err
		}
		var out triage.Class
		req := models.Request{System: triage.System, MaxTokens: 400,
			Messages: []models.Message{{Role: "user", Content: triage.Untrusted("issue", "Title: "+title+"\n\n"+body)}}}
		err = models.Structured(ctx, func(ctx context.Context, r models.Request) (models.Response, error) {
			resp, _, err := gw.Do(ctx, models.Call{Story: fmt.Sprintf("#%d", n), Seat: string(verbs.ProductStrategist), Lane: verbs.LaneChore, Host: verbs.DirectHost, Req: r})
			return resp, err
		}, req, []byte(triage.Schema), &out)
		return out, err
	}}
	res, err := t.Run(ctx, issueNumber(in.String("issue")), in.String("as"))
	if errors.Is(err, triage.ErrNotIntake) {
		return Wrap(ExitBlocked, err)
	}
	if err != nil {
		return err
	}
	return e.Emit(res, func(w io.Writer) error {
		fmt.Fprintf(w, "#%d: %s", res.Issue, res.Class)
		if res.Class == "feedback" {
			fmt.Fprintf(w, " (accepted: %t)", res.Accepted)
		}
		fmt.Fprintf(w, "; added %v, removed %v\n", res.Added, res.Removed)
		return nil
	})
}

// Intake is an open intake issue that triage would classify.
type Intake struct {
	Issue int    `json:"issue"`
	Title string `json:"title"`
	URL   string `json:"url"`
}

// listIntake is `aayu triage` with no issue: the open intake issues, oldest
// first, for the weekly triage loop's agent to classify one by one with --as.
// It reads only.
func listIntake(ctx context.Context, e *Env, repo string) error {
	c, _, err := e.HumanGitHub(ctx)
	if err != nil {
		return err
	}
	owner, name, _ := github.SplitRepo(repo)
	is, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.Issue, *gh.Response, error) {
		return c.REST.Issues.ListByRepo(ctx, owner, name, &gh.IssueListByRepoOptions{State: "open", Labels: []string{workitems.LabelIntake}, Sort: "created", Direction: "asc", ListOptions: o})
	})
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	out := []Intake{}
	for _, i := range is {
		if !i.IsPullRequest() {
			out = append(out, Intake{i.GetNumber(), i.GetTitle(), i.GetHTMLURL()})
		}
	}
	return e.Emit(out, func(w io.Writer) error {
		if len(out) == 0 {
			_, err := fmt.Fprintln(w, "no open intake issues")
			return err
		}
		for _, i := range out {
			if _, err := fmt.Fprintf(w, "#%d %s\n", i.Issue, i.Title); err != nil {
				return err
			}
		}
		return nil
	})
}
