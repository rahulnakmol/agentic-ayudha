package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/autonomy"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/delegate"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/inspect"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/models"
	"github.com/rahulnakmol/agentic-ayudha/internal/notify"
	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
	"github.com/rahulnakmol/agentic-ayudha/internal/ready"
	"github.com/rahulnakmol/agentic-ayudha/internal/seats"
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

// headSource reads the PR head through the contents API at its SHA: inspect
// retrieves context without checking out or running the PR's code (WP4.8b).
type headSource struct {
	c                *github.Client
	owner, name, ref string
}

func (h headSource) File(ctx context.Context, p string) ([]byte, error) {
	fc, _, _, err := h.c.REST.Repositories.GetContents(ctx, h.owner, h.name, p, &gh.RepositoryContentGetOptions{Ref: h.ref})
	if err != nil {
		return nil, err
	}
	if fc == nil {
		return nil, fmt.Errorf("%s is not a file", p)
	}
	s, err := fc.GetContent()
	return []byte(s), err
}

func (h headSource) Dir(ctx context.Context, d string) ([]string, error) {
	if d == "." {
		d = ""
	}
	_, dc, _, err := h.c.REST.Repositories.GetContents(ctx, h.owner, h.name, d, &gh.RepositoryContentGetOptions{Ref: h.ref})
	if err != nil {
		return nil, err
	}
	var out []string
	for _, x := range dc {
		if x.GetType() == "file" {
			out = append(out, x.GetPath())
		}
	}
	return out, nil
}

// DismissedPath is where a repository's dismissed inspect findings live on the ledger.
const DismissedPath = "inspect/dismissed.jsonl"

func readDismissed(ctx context.Context, w *ledger.Writer) []inspect.Dismissal {
	t, err := w.Read(ctx)
	if err != nil {
		return nil
	}
	defer t.Close()
	b, err := t.File(ctx, DismissedPath)
	if err != nil {
		return nil
	}
	var out []inspect.Dismissal
	for _, line := range strings.Split(string(b), "\n") {
		var d inspect.Dismissal
		if json.Unmarshal([]byte(line), &d) == nil && d.ID != "" {
			out = append(out, d)
		}
	}
	return out
}

// reviewers picks inspect's reviewers: the review lane's model, then in peer
// mode the configured peer model or the first registry candidate of another
// family. A registry default whose family equals the builder's is replaced
// by the first candidate that differs (a delegated Codex PR, built by OpenAI's
// models, is not reviewed by an OpenAI model); a configured one is refused.
//
// builder lists every family the builder may be (Bug #171: one when it is
// known, all candidates when it is not); each reviewer differs from all.
func reviewerModels(gw *models.Gateway, builder []verbs.Family, peer bool, peerModel, story string) ([]models.Resolved, []string, error) {
	a, err := gw.Resolver.ResolveDirect(verbs.LaneReview, verbs.DirectHost, false)
	if err != nil {
		return nil, nil, Wrap(ExitUsage, err)
	}
	var notes []string
	pick := func(skip ...verbs.Family) (models.Resolved, bool) {
		for _, m := range verbs.PeerReviewCandidates {
			f := verbs.FamilyOf(m)
			if !slices.Contains(skip, f) {
				return models.Resolved{Lane: verbs.LaneReview, Host: verbs.DirectHost, Model: m, Display: m, Family: f, Source: models.SourceDefault}, true
			}
		}
		return models.Resolved{}, false
	}
	differs := func(f verbs.Family) error {
		for _, b := range builder {
			if err := models.CheckFamilies(story, b, f); err != nil {
				return err
			}
		}
		return models.CheckFamilies(story, "", f)
	}
	if err := differs(a.Family); err != nil {
		alt, ok := pick(builder...)
		if a.Source == models.SourceConfig || !ok {
			return nil, nil, Wrap(ExitBlocked, err)
		}
		notes = append(notes, fmt.Sprintf("the review lane's %s is of the builder's family or one it may be (%v); %s reviews instead", a.Model, builder, alt.Model))
		a = alt
	}
	out := []models.Resolved{a}
	if !peer {
		return out, notes, nil
	}
	b, ok := pick(append(slices.Clone(builder), a.Family, verbs.FamilyHostManaged)...)
	if peerModel != "" {
		b, ok = models.Resolved{Lane: verbs.LaneReview, Host: verbs.DirectHost, Model: peerModel, Display: peerModel, Family: verbs.FamilyOf(peerModel), Source: models.SourceConfig}, true
	}
	if !ok || b.Family == a.Family || differs(b.Family) != nil {
		return nil, nil, Errorf(ExitUsage, "inspect: no peer reviewer whose family differs from the builder's (%s) and %s's (%s); set [inspect] peer_model", builder, a.Model, a.Family)
	}
	return append(out, b), notes, nil
}

// InspectVoice is the Engineering Manager twin's voice for inspect's summary
// (inspect is the EM seat's verb, WP5.1b) with the evidence rule: the summary
// speaks as the persona and cites file:line; the findings stay in the reply's
// schema and the floors stay facts. Empty when the personas cannot be read.
func InspectVoice(assets fs.FS) string {
	if assets == nil {
		return ""
	}
	ps, err := seats.LoadPersonas(assets)
	if err != nil {
		return ""
	}
	p, ok := ps[verbs.EngineeringManager]
	if !ok {
		return ""
	}
	return fmt.Sprintf("Write the summary as %s, the %s twin. Voice: %s Pushing back: %s\n\n%s\n\n"+
		"The persona shapes the summary only. Findings stay in the reply's schema, one per problem with its quote, and the deterministic floors stay facts no voice can soften.",
		p.Name, verbs.SeatName(verbs.EngineeringManager), p.Voice, p.PushBack, seats.EvidenceRule)
}

func runInspect(ctx context.Context, e *Env, in Values) error {
	if in.String("dismiss") != "" {
		return e.inspectDismiss(ctx, in)
	}
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
	var deleg delegate.State
	checkpoints := map[string]string{} // checkpoint commit -> model, from build's ledger record (Bug #171)
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
				if b, err := t.File(ctx, delegate.LedgerPath(ins.Story)); err == nil {
					_ = json.Unmarshal(b, &deleg)
				}
				var st BuildState
				if b, err := t.File(ctx, BuildLedgerPath(ins.Story)); err == nil && json.Unmarshal(b, &st) == nil {
					for _, cp := range st.Checkpoints {
						checkpoints[cp.Commit] = cp.Model
					}
				}
				t.Close()
			}
			ins.Dismissed = readDismissed(ctx, w)
		}
		ins.Pack, ins.Evidence, _ = e.packFor(ctx, c, ins.Story, pr.GetHead().GetSHA())
	}
	cfg := l.Config.Inspect
	ins.Voice = InspectVoice(e.Assets)
	ins.Mode = cfg.Mode
	if deleg.Receiver != "" && cfg.DelegatedMode != "" {
		ins.Mode = cfg.DelegatedMode
	}
	if m := in.String("mode"); m != "" {
		ins.Mode = m
	}
	if !ins.Draft && ins.Pack != nil {
		ins.Files = inspect.ParseDiff(diff)
		lg := autonomy.ReadLayers(config.RepoRoot(e.Dir))
		ins.Areas = inspect.BuildContext(ctx, ins.Files, headSource{c, owner, name, pr.GetHead().GetSHA()}, lg.Footprint)
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
	// The builder's family comes from what aayu recorded outside the PR (the
	// delegation record, build's checkpoint models); the pack's model, from the
	// commit trailer, is only a claim. When they are missing or disagree the
	// family is unknown, and every reviewer differs from every candidate
	// (Bug #171).
	var commits []string
	if cs, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.RepositoryCommit, *gh.Response, error) {
		return c.REST.PullRequests.ListCommits(ctx, owner, name, n, &o)
	}); err == nil {
		for _, x := range cs {
			commits = append(commits, x.GetSHA())
		}
	}
	claim, lane := "", verbs.Family("")
	if ins.Pack != nil {
		claim = ins.Pack.Model
		if b, err := gw.Resolver.Resolve(verbs.LaneBuild, verbs.HostID(ins.Pack.Host)); err == nil {
			lane = b.Family
		}
	}
	builder := inspect.BuilderOf(inspect.Records{Receiver: deleg.Receiver, Delegated: deleg.Family, Checkpoints: checkpoints}, commits, claim, lane)
	if cfg.Reviewer != inspect.ReviewerModel {
		return e.inspectThroughApp(ctx, appInspect{c: c, repo: repo, pr: pr, in: ins, prev: prev, builder: builder, cfg: l.Config})
	}
	story := fmt.Sprintf("#%d", ins.Story)
	picked, notes, err := reviewerModels(gw, builder.Candidates, ins.Mode == inspect.ModePeer, cfg.PeerModel, story)
	if err != nil {
		return err
	}
	notes = append([]string{builder.Note()}, notes...)
	pager, err := e.Notifier(ctx)
	if err != nil {
		return err
	}
	r := inspect.Runner{MaxUSD: l.Config.Budget.PerPRInspectMax,
		Filter: inspect.Filter{MinConfidence: cfg.MinConfidence, SuppressAfter: cfg.SuppressAfter, Generated: cfg.Generated},
		Page: func(ctx context.Context, subject, body string) error {
			_, err := pager.Page(ctx, notify.Page{Key: fmt.Sprintf("inspect:%s#%d", repo, n), Subject: subject, Body: body})
			return err
		}}
	for _, m := range picked {
		g := gw.WithModel(verbs.LaneReview, verbs.DirectHost, m.Model)
		r.Reviewers = append(r.Reviewers, inspect.Reviewer{Model: m.Model,
			Complete: func(ctx context.Context, req models.Request) (models.Response, float64, error) {
				resp, rm, err := g.Do(ctx, models.Call{Story: story, Seat: string(verbs.EngineeringManager), Lane: verbs.LaneReview, Host: verbs.DirectHost, Req: req})
				usd, _ := models.Cost(l.Config.Models.Prices, rm.Model, resp)
				return resp, usd, err
			}})
	}
	res, st, err := r.Run(ctx, ins, prev)
	res.Head, res.Builder = pr.GetHead().GetSHA(), builder
	res.Notes = append(notes, res.Notes...)
	if st.Result != nil {
		st.Result.Notes = res.Notes
	}
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
		if lines := inspect.InlineComments(res, st.Posted); len(lines) > 0 {
			if err := e.postInline(ctx, c, repo, n, res, lines); err != nil {
				res.Notes = append(res.Notes, "inline review comments not posted: "+oneLineErr(err))
			} else {
				for _, x := range lines {
					st.Posted = append(st.Posted, x.ID)
				}
			}
		}
		if _, err := c.UpsertComment(ctx, repo, n, inspect.Marker, inspect.Comment(res, st, l.Config.Budget.PerPRInspectMax),
			actions.Action{Kind: "github.comment.inspect", Payload: map[string]any{"disposition": res.Disposition, "diff_hash": res.DiffHash, "mode": res.Mode}}); err != nil {
			return err
		}
	}
	if err := e.Emit(res, func(w io.Writer) error {
		fmt.Fprintf(w, "inspect #%d at %s: %s", n, short(res.Head), res.Disposition)
		if res.Cached {
			fmt.Fprint(w, " (cached)")
		}
		if res.Mode != "" {
			fmt.Fprintf(w, " (%s: %s)", res.Mode, strings.Join(res.Reviewers, " + "))
		}
		fmt.Fprintln(w)
		if res.Overridden != "" {
			fmt.Fprintf(w, "  %s\n", res.Overridden)
		}
		for _, f := range res.Findings {
			fmt.Fprintf(w, "  %s %s %s:%d %s [%s]\n", f.Severity, f.Category, f.File, f.Line, f.Title, f.ID)
		}
		if len(res.Dropped) > 0 {
			fmt.Fprintf(w, "  %d finding(s) not posted (noise rules)\n", len(res.Dropped))
		}
		for _, v := range res.Criteria {
			fmt.Fprintf(w, "  criterion %d: %s\n", v.ID, v.Verdict)
		}
		for _, x := range res.Notes {
			fmt.Fprintf(w, "  note: %s\n", x)
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

func oneLineErr(err error) string {
	s := strings.Join(strings.Fields(err.Error()), " ")
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}

// postInline posts one PR review (event COMMENT; inspect never approves or
// requests changes) holding a comment per new finding on its line.
func (e *Env) postInline(ctx context.Context, c *github.Client, repo string, n int, res inspect.Result, lines []inspect.Inline) error {
	owner, name, _ := github.SplitRepo(repo)
	var cs []*gh.DraftReviewComment
	for _, x := range lines {
		cs = append(cs, &gh.DraftReviewComment{Path: gh.Ptr(x.Path), Line: gh.Ptr(x.Line), Side: gh.Ptr("RIGHT"), Body: gh.Ptr(x.Body)})
	}
	body := fmt.Sprintf("inspect (%s): %s, %d finding(s) on lines of this PR. The summary and state are in inspect's comment.", res.Mode, res.Disposition, len(lines))
	_, err := c.Write(ctx, actions.Action{Kind: "github.pull.review", Target: fmt.Sprintf("%s#%d", repo, n),
		Request: &actions.Request{Method: http.MethodPost, Path: fmt.Sprintf("/repos/%s/pulls/%d/reviews", repo, n)},
		Payload: map[string]any{"comments": len(cs), "head": res.Head}},
		func(ctx context.Context) (any, error) {
			rv, _, err := c.REST.PullRequests.CreateReview(ctx, owner, name, n, &gh.PullRequestReviewRequest{CommitID: gh.Ptr(res.Head), Event: gh.Ptr("COMMENT"), Body: &body, Comments: cs})
			return map[string]any{"id": rv.GetID()}, err
		})
	return err
}

// inspectDismiss records a finding a human dismissed, so later inspections
// stop raising findings like it in this repository (WP4.8b). An agent cannot
// dismiss its reviewer's findings.
func (e *Env) inspectDismiss(ctx context.Context, in Values) error {
	id, reason := in.String("dismiss"), strings.TrimSpace(in.String("reason"))
	if e.Agent() {
		return Errorf(ExitBlocked, "inspect --dismiss is a human's judgement; it is refused for agents")
	}
	if reason == "" {
		return Errorf(ExitUsage, "inspect --dismiss %s: say why with --reason", id)
	}
	repo, err := e.RepoName(ctx)
	if err != nil {
		return err
	}
	n := issueNumber(in.String("pr"))
	c, tok, err := e.HumanGitHub(ctx)
	if err != nil {
		return err
	}
	owner, name, _ := github.SplitRepo(repo)
	comments, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.IssueComment, *gh.Response, error) {
		return c.REST.Issues.ListComments(ctx, owner, name, n, &gh.IssueListCommentsOptions{ListOptions: o})
	})
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	var st inspect.State
	for _, cm := range comments {
		if strings.Contains(cm.GetBody(), inspect.Marker) {
			st = inspect.ParseState(cm.GetBody())
		}
	}
	var f *inspect.Finding
	if st.Result != nil {
		for i := range st.Result.Findings {
			if st.Result.Findings[i].ID == id {
				f = &st.Result.Findings[i]
			}
		}
	}
	if f == nil {
		return Errorf(ExitUsage, "inspect --dismiss: PR #%d's latest inspection has no finding %s", n, id)
	}
	login := ""
	if u, _, err := c.REST.Users.Get(ctx, ""); err == nil {
		login = u.GetLogin()
	}
	d := inspect.Dismissal{ID: f.ID, PR: n, File: f.File, Category: f.Category, Title: f.Title, Reason: reason, By: login, At: e.Now().UTC().Format(time.RFC3339)}
	w, err := e.Ledger(ctx, tok, "human", "")
	if err != nil {
		return err
	}
	b, _ := json.Marshal(d)
	if !c.DryRun() {
		if _, err := w.Write(ctx, ledger.Commit{Message: fmt.Sprintf("inspect: dismiss %s on #%d", id, n), Append: map[string][]byte{DismissedPath: append(b, '\n')}}); err != nil {
			return Wrap(ExitExternal, err)
		}
	}
	return e.Emit(d, func(w io.Writer) error {
		_, err := fmt.Fprintf(w, "dismissed %s (%s %s:%d %s): %s\n", d.ID, f.Severity, d.File, f.Line, d.Title, d.Reason)
		return err
	})
}
