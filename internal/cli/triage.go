package cli

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/models"
	"github.com/rahulnakmol/agentic-ayudha/internal/triage"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
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
