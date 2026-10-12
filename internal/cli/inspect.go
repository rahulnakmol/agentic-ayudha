package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/inspect"
	"github.com/rahulnakmol/agentic-ayudha/internal/models"
	"github.com/rahulnakmol/agentic-ayudha/internal/notify"
	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
	"github.com/rahulnakmol/agentic-ayudha/internal/ready"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

func init() { register("inspect", runInspect) }

// inspectClient writes inspect's comment: in CI the workflow token (the merge
// gate grants it pull-requests: write), else the bot or the tagged human.
func (e *Env) inspectClient(ctx context.Context) (*github.Client, error) {
	if e.CI && e.Getenv("AAYU_BOT_TOKEN") == "" {
		tok, err := e.Resolver().Human(ctx)
		if err != nil {
			return nil, Wrap(ExitBlocked, err)
		}
		return e.newGitHub(tok, "ci", "")
	}
	c, _, err := e.BotGitHub(ctx, "")
	return c, err
}

// packFor finds the pack for the PR head: the local one verify left (in CI,
// the merge gate's verify job artifact), else the best one on the ledger.
// Text artifacts become evidence excerpts; media never reach the model.
func (e *Env) packFor(ctx context.Context, c *github.Client, story int, head string) (*proof.Pack, map[string]string, error) {
	ev := map[string]string{}
	if root := config.RepoRoot(e.Dir); root != "" {
		dir := proof.LocalDir(root, story)
		if p, err := proof.LoadLocal(dir); err == nil && p.PRHeadSHA == head {
			for _, a := range p.Artifacts {
				if b, err := os.ReadFile(filepath.Join(dir, "artifacts", filepath.FromSlash(a.Path))); err == nil && !proof.IsMedia(a) {
					ev[a.Path] = excerpt(b)
				}
			}
			return p, ev, nil
		}
	}
	tok, err := e.Resolver().Human(ctx)
	if err != nil {
		return nil, nil, inspect.ErrNoPack
	}
	s, err := e.proofStore(ctx, c, tok, "human", "")
	if err != nil {
		return nil, nil, inspect.ErrNoPack
	}
	t, err := s.Ledger.Read(ctx)
	if err != nil {
		return nil, nil, inspect.ErrNoPack
	}
	defer t.Close()
	es, err := s.Packs(ctx, t, story)
	if err != nil {
		return nil, nil, err
	}
	for _, x := range es {
		if x.Pack.PRHeadSHA != head {
			continue
		}
		open := s.Opener(ctx, t, x)
		for _, a := range x.Pack.Artifacts {
			if a.Store == proof.StoreGit {
				if b, err := open(a); err == nil {
					ev[a.Path] = excerpt(b)
				}
			}
		}
		return x.Pack, ev, nil
	}
	return nil, nil, inspect.ErrNoPack
}

func excerpt(b []byte) string {
	const n = 3000
	if len(b) > n {
		return string(b[:n]) + fmt.Sprintf("\n… (%d more bytes)", len(b)-n)
	}
	return string(b)
}

func runInspect(ctx context.Context, e *Env, in Values) error {
	repo, err := e.RepoName(ctx)
	if err != nil {
		return err
	}
	l, err := e.Config()
	if err != nil {
		return err
	}
	n := issueNumber(in.String("pr"))
	c, err := e.inspectClient(ctx)
	if err != nil {
		return err
	}
	owner, name, err := github.SplitRepo(repo)
	if err != nil {
		return Wrap(ExitUsage, err)
	}
	pr, _, err := c.REST.PullRequests.Get(ctx, owner, name, n)
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	diff, _, err := c.REST.PullRequests.GetRaw(ctx, owner, name, n, gh.RawOptions{Type: gh.Diff})
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	ins := inspect.Input{Repo: repo, PR: n, Title: pr.GetTitle(), Body: pr.GetBody(), Draft: pr.GetDraft(), Diff: diff}
	ins.Story = StoryOf(pr.GetBody(), "", pr.GetHead().GetRef())
	res := inspect.Result{PR: n, Head: pr.GetHead().GetSHA()}
	if ins.Story == 0 && !ins.Draft {
		return Errorf(ExitBlocked, "inspect #%d: no story (Refs #n in the PR body or a story/<n>- branch); inspect judges a PR against its story", n)
	}
	if ins.Story > 0 {
		if is, _, err := c.REST.Issues.Get(ctx, owner, name, ins.Story); err == nil {
			ins.StoryTitle, ins.StoryBody = is.GetTitle(), is.GetBody()
		}
		tok, err := e.Resolver().Human(ctx)
		if err != nil {
			return Wrap(ExitBlocked, err)
		}
		if w, err := e.Ledger(ctx, tok, "human", ""); err == nil {
			if t, err := w.Read(ctx); err == nil {
				if b, err := t.File(ctx, ready.LedgerPath(ins.Story)); err == nil {
					if snap, err := ready.ParseSnapshot(string(b)); err == nil {
						ins.Criteria = snap.Criteria
					}
				}
				t.Close()
			}
		}
		ins.Pack, ins.Evidence, _ = e.packFor(ctx, c, ins.Story, pr.GetHead().GetSHA())
	}

	comments, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.IssueComment, *gh.Response, error) {
		return c.REST.Issues.ListComments(ctx, owner, name, n, &gh.IssueListCommentsOptions{ListOptions: o})
	})
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	var prev inspect.State
	for _, cm := range comments {
		if strings.Contains(cm.GetBody(), inspect.Marker) {
			prev = inspect.ParseState(cm.GetBody())
		}
	}

	gw, err := e.Gateway()
	if err != nil {
		return err
	}
	reviewer, err := gw.Resolver.ResolveDirect(verbs.LaneReview, verbs.DirectHost, false)
	if err != nil {
		return Wrap(ExitUsage, err)
	}
	builder := verbs.Family("")
	if ins.Pack != nil && ins.Pack.Model != "" {
		builder = verbs.FamilyOf(ins.Pack.Model)
	} else if ins.Pack != nil {
		if b, err := gw.Resolver.Resolve(verbs.LaneBuild, verbs.HostID(ins.Pack.Host)); err == nil {
			builder = b.Family
		}
	}
	story := fmt.Sprintf("#%d", ins.Story)
	if err := models.CheckFamilies(story, builder, reviewer.Family); err != nil {
		return Wrap(ExitBlocked, err)
	}
	pager, err := e.Notifier(ctx)
	if err != nil {
		return err
	}
	r := inspect.Runner{Model: reviewer.Model, MaxUSD: l.Config.Budget.PerPRInspectMax,
		Complete: func(ctx context.Context, req models.Request) (models.Response, float64, error) {
			resp, m, err := gw.Do(ctx, models.Call{Story: story, Seat: string(verbs.EngineeringManager), Lane: verbs.LaneReview, Host: verbs.DirectHost, Req: req})
			usd, _ := models.Cost(l.Config.Models.Prices, m.Model, resp)
			return resp, usd, err
		},
		Page: func(ctx context.Context, subject, body string) error {
			_, err := pager.Page(ctx, notify.Page{Key: fmt.Sprintf("inspect:%s#%d", repo, n), Subject: subject, Body: body})
			return err
		}}
	res, st, err := r.Run(ctx, ins, prev)
	res.Head = pr.GetHead().GetSHA()
	switch {
	case errors.Is(err, inspect.ErrNoPack):
		return Errorf(ExitBlocked, "inspect #%d: %v", n, err)
	case errors.Is(err, inspect.ErrCycles):
		return Errorf(ExitBlocked, "inspect #%d: %v (paged)", n, err)
	case errors.Is(err, inspect.ErrBudget), errors.Is(err, models.ErrBudget):
		return Errorf(ExitBudget, "inspect #%d: %v", n, err)
	case err != nil:
		return Wrap(ExitExternal, err)
	}
	if res.Disposition != inspect.Skipped && !res.Cached {
		if _, err := c.UpsertComment(ctx, repo, n, inspect.Marker, inspect.Comment(res, st, l.Config.Budget.PerPRInspectMax),
			actions.Action{Kind: "github.comment.inspect", Payload: map[string]any{"disposition": res.Disposition, "diff_hash": res.DiffHash}}); err != nil {
			return err
		}
	}
	if err := e.Emit(res, func(w io.Writer) error {
		fmt.Fprintf(w, "inspect #%d at %s: %s", n, short(res.Head), res.Disposition)
		if res.Cached {
			fmt.Fprint(w, " (cached)")
		}
		fmt.Fprintln(w)
		if res.Overridden != "" {
			fmt.Fprintf(w, "  %s\n", res.Overridden)
		}
		for _, f := range res.Findings {
			fmt.Fprintf(w, "  %s %s:%d %s\n", f.Severity, f.File, f.Line, f.Title)
		}
		for _, v := range res.Criteria {
			fmt.Fprintf(w, "  criterion %d: %s\n", v.ID, v.Verdict)
		}
		if res.Next != "" {
			fmt.Fprintf(w, "  next: %s\n", res.Next)
		}
		return nil
	}); err != nil {
		return err
	}
	if res.Disposition == inspect.Fix || res.Disposition == inspect.Rebuild {
		return Errorf(ExitBlocked, "inspect #%d: %s", n, res.Disposition)
	}
	return nil
}
