package cli

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

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
	"github.com/rahulnakmol/agentic-ayudha/internal/secrets"
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

// ApprovedPath is where the [autonomy] approver's reviews of PR heads, made
// through `aayu inspect --approve`, live on the ledger (Story #639).
const ApprovedPath = "inspect/approved.jsonl"

// headApproval is one approver's review of one PR head (inspectApprove).
type headApproval struct {
	PR     int    `json:"pr"`
	Head   string `json:"head"`
	By     string `json:"by"`
	At     string `json:"at"`
	Reason string `json:"reason"`
}

// readApprovals reads the recorded head approvals; none when the ledger
// cannot be read, so the human fallback fails closed.
func (e *Env) readApprovals(ctx context.Context) []headApproval {
	tok, err := e.Resolver().Human(ctx)
	if err != nil {
		return nil
	}
	w, err := e.Ledger(ctx, tok, "human", "")
	if err != nil {
		return nil
	}
	t, err := w.Read(ctx)
	if err != nil {
		return nil
	}
	defer t.Close()
	b, err := t.File(ctx, ApprovedPath)
	if err != nil {
		return nil
	}
	var out []headApproval
	for _, line := range strings.Split(string(b), "\n") {
		var a headApproval
		if json.Unmarshal([]byte(line), &a) == nil && a.Head != "" {
			out = append(out, a)
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
	return fmt.Sprintf("Write the summary as %s. Voice: %s Pushing back: %s\n\n%s\n\n"+
		"The persona shapes the summary only. Findings stay in the reply's schema, one per problem with its quote, and the deterministic floors stay facts no voice can soften.",
		p.Heading(), p.Voice, p.PushBack, seats.EvidenceRule)
}

func runInspect(ctx context.Context, e *Env, in Values) error {
	if in.String("dismiss") != "" {
		return e.inspectDismiss(ctx, in)
	}
	if in.Bool("approve") {
		return e.inspectApprove(ctx, in)
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
	// The story as verify finds it: the body's reference, then the head
	// commit's Aayudha-Story trailer, then the branch. A failed read fails
	// closed rather than judging a story PR as storyless.
	hc, hr, err := c.REST.Git.GetCommit(ctx, owner, name, pr.GetHead().GetSHA())
	if err != nil && (hr == nil || hr.StatusCode != http.StatusNotFound) {
		return Wrap(ExitExternal, err)
	}
	ins.Story = StoryOf(pr.GetBody(), hc.GetMessage(), pr.GetHead().GetRef())
	res := inspect.Result{PR: n, Head: pr.GetHead().GetSHA()}
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
	if !ins.Draft && (ins.Pack != nil || ins.Story == 0) {
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
	// The state (and the cached result it carries) is read only from a comment
	// the merge gate or this run's own account wrote, in the gate and outside it.
	prev := inspectState(comments, viewer(ctx, c), l.Config.Identity)
	var posts []inspect.Post
	for _, cm := range comments {
		posts = append(posts, inspect.Post{Kind: "comment", ID: cm.GetID(), Login: cm.GetUser().GetLogin(), Type: cm.GetUser().GetType(), Body: cm.GetBody(), Updated: cm.GetUpdatedAt().Time})
	}
	bots := l.Config.Identity.BotLogins
	ins.Head, ins.At = pr.GetHead().GetSHA(), e.Now()
	ins.Reopened = reopenedAt(posts, mayReopen(ctx, c, repo, bots), prev)

	// A bad review setting is reported after the docs-only exit, which needs no review.
	reviewErr := e.useReviewModel(&l.Config)
	gw, err := e.Gateway()
	if err != nil {
		return err
	}
	// The builder's family comes from what aayu recorded outside the PR (the
	// delegation record, build's checkpoint models); the pack's model, from the
	// commit trailer, is only a claim. When they are missing or disagree the
	// family is unknown, and every reviewer differs from every candidate
	// (Bug #171).
	//
	// Every model the PR declares is a claim (Decision #469): the
	// Aayudha-Model trailers of all its commits (a session that changed model
	// writes one per model), the body's Aayudha-Model lines for a PR built
	// outside aayu build, and the pack's model. A declaration (a commit, the
	// body, the pack) that names a host but no model, or only host-managed,
	// adds its host: that host picked the model, so every family its modes
	// may use is a candidate, and a host aayu cannot place makes the builder
	// unknown.
	var commits, claims, hosts []string
	lane := verbs.Family("")
	declare := func(models []string, host string, named bool) {
		concrete := false
		for _, m := range models {
			if m = strings.ToLower(m); !slices.Contains(claims, m) {
				claims = append(claims, m)
			}
			concrete = concrete || verbs.Family(m) != verbs.FamilyHostManaged
		}
		if host = strings.ToLower(host); !concrete && (named || len(models) > 0) && !slices.Contains(hosts, host) {
			hosts = append(hosts, host)
		}
	}
	trailers := func(text string) {
		var models []string
		for _, m := range modelTrailer.FindAllStringSubmatch(text, -1) {
			models = append(models, m[1])
		}
		h := hostTrailer.FindStringSubmatch(text)
		if h == nil {
			declare(models, "", false)
		} else {
			declare(models, h[1], true)
		}
	}
	trailers(pr.GetBody())
	// Without the commits' declarations a family could be missing, so a
	// failed listing fails the run rather than reviewing on the rest.
	cs, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.RepositoryCommit, *gh.Response, error) {
		return c.REST.PullRequests.ListCommits(ctx, owner, name, n, &o)
	})
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	for _, x := range cs {
		commits = append(commits, x.GetSHA())
		trailers(x.GetCommit().GetMessage())
	}
	if ins.Pack != nil {
		_, known := verbs.FindHost(verbs.HostID(ins.Pack.Host))
		var models []string
		if ins.Pack.Model != "" {
			models = []string{ins.Pack.Model}
		}
		if known {
			if b, err := gw.Resolver.Resolve(verbs.LaneBuild, verbs.HostID(ins.Pack.Host)); err == nil {
				lane = b.Family
			}
		}
		// A build lane that names a family is the model the pack's host ran.
		declare(models, ins.Pack.Host, known && (lane == "" || lane == verbs.FamilyHostManaged))
	}
	builder := inspect.BuilderOf(inspect.Records{Receiver: deleg.Receiver, Delegated: deleg.Family, Checkpoints: checkpoints}, commits, claims, hosts, lane)
	// A docs-only PR merges without a review, whichever backend reviews.
	if !ins.Draft && inspect.DocsOnly(inspect.ParseDiff(ins.Diff)) {
		return e.emitInspect(inspect.Result{PR: n, Head: pr.GetHead().GetSHA(), Findings: []inspect.Finding{}, Criteria: []inspect.Verdict{}, Builder: builder,
			Disposition: inspect.Skipped, Summary: "the PR changes only documentation, which merges without a review"}, []string{builder.Note()})
	}
	if reviewErr != nil {
		return Wrap(ExitUsage, reviewErr)
	}
	perm := func() string {
		p, _, err := c.REST.Repositories.GetPermissionLevel(ctx, owner, name, pr.GetUser().GetLogin())
		if err != nil {
			return ""
		}
		return permissionOf(p)
	}
	// The CI review fails closed, naming the fix, when the code-review
	// environment lacks what it calls (a reply event's job holds none).
	if e.CI && !e.replyEvent() && cfg.Reviewer == inspect.ReviewerCI {
		if miss := missingReviewModel(gw, &l.Config); len(miss) > 0 {
			return Errorf(ExitBlocked, "inspect #%d: [inspect] reviewer = \"ci\" calls the model the %s environment names, and %s is not set: add it in Settings → Environments → %s (docs/guide/how-review.md)",
				n, inspect.ReviewEnvironment, strings.Join(miss, " and "), inspect.ReviewEnvironment)
		}
	}
	paid := paidReviewer(gw, &l.Config, builder, "", perm, e.Logf)
	// A head whose rung B failed says why on every later run, whichever
	// reviewer comes first.
	fallback := ""
	if prev.PaidFailed != "" && prev.PaidFailed == pr.GetHead().GetSHA() {
		fallback = fmt.Sprintf("fallback: rung B ([inspect] paid_reviewer %s) failed on %s (%s); the reviewer app reviews, and the next push tries it again", l.Config.Inspect.PaidReviewer, short(prev.PaidFailed), cmp.Or(prev.PaidFailedWhy, "reason not recorded"))
	}
	switch {
	case paid == nil:
	case e.replyEvent():
		// A reply only re-runs the head's failed inspect job, which makes
		// the paid call itself if one is due: one here would be a second run.
		e.Logf("inspect: [inspect] paid_reviewer %s is not called on a reply event; the head's inspect job reviews", paid.Model)
		paid = nil
	case prev.PaidFailed != "" && prev.PaidFailed == pr.GetHead().GetSHA():
		e.Logf("inspect: %s", fallback)
		paid = nil
	}
	// The chosen tool is the only reviewer while it answers. When an app
	// cannot review this head (a usage-limit or quota reply, or no reply
	// within inspect.ReplyWait), the next installed app does, then rung B,
	// then a human; when the CI review (rung B) cannot, the apps do, then a human.
	unavailable := ""
	if cfg.Reviewer != inspect.ReviewerModel && paid == nil {
		a := appInspect{c: c, repo: repo, pr: pr, in: ins, prev: prev, builder: builder, cfg: l.Config,
			paidTried: e.replyEvent() || prev.PaidFailed == pr.GetHead().GetSHA() || cfg.Reviewer == inspect.ReviewerCI, fallback: fallback}
		err := e.inspectThroughApp(ctx, a)
		if a.paidTried || !errors.Is(err, inspect.ErrUnavailable) {
			return err
		}
		unavailable = oneLineErr(err)
		if paid = paidReviewer(gw, &l.Config, builder, unavailable, perm, e.Logf); paid == nil {
			a.paidTried = true
			return e.inspectThroughApp(ctx, a)
		}
	}
	// The review lane's model judges a PR against its story's pack; a PR
	// without a story goes to a reviewer app (the merge gate's default), or
	// to the paid reviewer, which judges its diff and description as an app does.
	if ins.Story == 0 && !ins.Draft && paid == nil {
		return Errorf(ExitBlocked, "inspect #%d: no story (Refs #n in the PR body or a story/<n>- branch); a model reviewer judges a PR against its story, so a PR without one needs a reviewer app ([inspect] reviewer)", n)
	}
	story := fmt.Sprintf("#%d", ins.Story)
	picked, notes, err := reviewerModels(gw, builder.Candidates, ins.Mode == inspect.ModePeer, cfg.PeerModel, story)
	if paid != nil {
		picked, err = []models.Resolved{*paid}, nil
		why := `[inspect] reviewer = "ci"`
		if unavailable != "" {
			why = "fallback: no installed reviewer app can review this head (" + unavailable + ")"
		}
		notes = []string{fmt.Sprintf("paid reviewer (rung B): %s, so [inspect] paid_reviewer %s (%s) reviews with its endpoint's key, within %.2f USD for this PR", why, paid.Model, paid.Family, l.Config.Budget.PerPRInspectMax)}
	}
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
	var served []string // the service tier of each reply ("" when it states none)
	for _, m := range picked {
		r.Reviewers = append(r.Reviewers, inspect.Reviewer{Model: m.Model, Complete: e.reviewCall(gw, &l.Config, m.Model, story, &served)})
	}
	res, st, err := r.Run(ctx, ins, prev)
	res.Head, res.Builder = pr.GetHead().GetSHA(), builder
	if len(served) > 0 {
		notes = append(notes, tierNote(served))
	}
	if res.Model != "" {
		res.ReviewerFamily = verbs.FamilyOf(res.Model)
		res.Rung = builder.Rung(res.ReviewerFamily)
		// A cached result comes from a PR comment anyone can write, so its
		// model counts only when it is one the review lane picks now.
		if !slices.ContainsFunc(picked, func(m models.Resolved) bool { return m.Model == res.Model }) {
			res.Rung = inspect.RungC
		}
		if paid != nil && res.Rung == inspect.RungA {
			res.Rung = inspect.RungB
		}
		res.Review = builder.RungLabel(res.Rung, res.Model, res.ReviewerFamily)
	}
	res.Notes = append(notes, res.Notes...)
	if !e.CI && err == nil {
		if note, err := e.preExistingBug(ctx, repo, n, res.Head, &res); err != nil {
			res.Notes = append(res.Notes, "pre-existing findings not filed: "+oneLineErr(err))
		} else if note != "" {
			res.Notes = append(res.Notes, note)
		}
	}
	// As in the app path, a cached review's rung follows the builder as
	// declared now, and a changed label rewrites the record ship reads.
	relabel := false
	if st.Result != nil {
		relabel = st.Result.Review != res.Review
		st.Result.Notes, st.Result.Head, st.Result.Builder = res.Notes, res.Head, res.Builder
		st.Result.Rung, st.Result.ReviewerFamily, st.Result.Review = res.Rung, res.ReviewerFamily, res.Review
	}
	switch {
	case errors.Is(err, inspect.ErrNoPack):
		return Errorf(ExitBlocked, "inspect #%d: %v", n, err)
	case err != nil && paid != nil && cfg.Reviewer != inspect.ReviewerModel:
		// A paid review that fails (the gateway timed out on a large diff, the
		// provider is down) or whose PR spent its [budget] per_pr_inspect_max
		// leaves the PR to the reviewer app, as an unset key does: the gate
		// still needs that app's reply (a human's when the app cannot review),
		// so it fails closed without one. What the failed call cost stays on
		// the PR's record, so retries cannot spend past the cap.
		// The comment says why, so a failure shows without --verbose; the
		// endpoint's URL is never shown (useReviewModel).
		why := err.Error()
		if base := strings.TrimSuffix(l.Config.Models.Gateway.BaseURL, "/"); base != "" {
			why = strings.ReplaceAll(why, base, "<endpoint>")
			// A dial, TLS, or proxy error names the host again, outside the URL.
			if u, err := url.Parse(base); err == nil && u.Hostname() != "" {
				why = strings.ReplaceAll(why, u.Hostname(), "<endpoint host>")
			}
		}
		why = oneLineErr(errors.New(why))
		fallback := fmt.Sprintf("fallback: rung B ([inspect] paid_reviewer %s) failed on this head (%s); the reviewer app reviews, and the next push tries it again", paid.Model, why)
		e.Logf("inspect: %s", fallback)
		fb := prev
		fb.SpentUSD, fb.PaidFailed, fb.PaidFailedWhy = max(prev.SpentUSD, st.SpentUSD), pr.GetHead().GetSHA(), why
		return e.inspectThroughApp(ctx, appInspect{c: c, repo: repo, pr: pr, in: ins, prev: fb, builder: builder, cfg: l.Config, record: fb.PaidFailed != prev.PaidFailed, paidTried: true, fallback: fallback})
	case errors.Is(err, inspect.ErrBudget), errors.Is(err, models.ErrBudget):
		return Errorf(ExitBudget, "inspect #%d: %v", n, err)
	case err != nil:
		return Wrap(ExitExternal, err)
	}
	// The review lane's model reviewed this head, so no person's review is
	// awaited: a human wait an earlier run recorded is cleared, even on a
	// cached judgement, or the inbox would still offer it (Codex's P2 on #678).
	waited := st.Human != ""
	st.Human = ""
	if res.Disposition != inspect.Skipped && (!res.Cached || relabel || waited) {
		if lines := inspect.InlineComments(res, st.Posted); len(lines) > 0 && !res.Cached {
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
		if res.Review != "" {
			fmt.Fprintf(w, "  review: %s\n", res.Review)
		}
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
	if res.Disposition == inspect.Fix || res.Disposition == inspect.Rebuild || res.Disposition == inspect.StepBack {
		return Errorf(ExitBlocked, "inspect #%d: %s", n, res.Disposition)
	}
	return nil
}

func oneLineErr(err error) string {
	s := strings.ToValidUTF8(strings.Join(strings.Fields(err.Error()), " "), "\uFFFD")
	if len(s) > 200 {
		s = s[:200]
		for !utf8.ValidString(s) { // a cut inside a multibyte rune
			s = s[:len(s)-1]
		}
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
	if err := e.requireHuman(ctx, "inspect --dismiss", AccessWrite); err != nil {
		return err
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
	var ident config.Identity
	if l, err := e.Config(); err == nil {
		ident = l.Config.Identity
	}
	login := viewer(ctx, c)
	st := inspectState(comments, login, ident)
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

// inspectApprove records the [autonomy] approver's review of the PR's head on
// the ledger and posts it as the human review the merge gate reads (Story
// #639; founder, 2026-10-18: "agree"). It is human-only (presence) and runs
// only as the approver. The gate's human fallback counts a reply only when
// such a record names its exact head, because a comment alone may be an agent
// holding the person's token (Codex's P2 on #634).
func (e *Env) inspectApprove(ctx context.Context, in Values) error {
	repo, err := e.RepoName(ctx)
	if err != nil {
		return err
	}
	l, err := e.Config()
	if err != nil {
		return err
	}
	a, err := e.approveHead(ctx, &l.Config, repo, issueNumber(in.String("pr")), "", in.String("reason"))
	if err != nil {
		return err
	}
	return e.Emit(a, func(w io.Writer) error {
		_, err := fmt.Fprintf(w, "approved #%d at %s as %s, recorded on the ledger; the merge gate's inspect check reads it\n", a.PR, short(a.Head), a.By)
		return err
	})
}

// approveHead is the record step of `aayu inspect --approve`, and of the
// inbox's human-review card (Story #675): as the [autonomy] approver, after
// the presence check, it records the approval of PR n's head on the ledger and
// posts the human-review reply the merge gate reads. want, when set, is the
// head the person was shown: a PR now at another head records nothing. A
// session's earlier approval covers the presence check (presence.go), so one
// inbox session approves several PRs with one device code.
func (e *Env) approveHead(ctx context.Context, cfg *config.Config, repo string, n int, want, reason string) (headApproval, error) {
	if reason = strings.TrimSpace(reason); reason == "" {
		return headApproval{}, Errorf(ExitUsage, "inspect --approve: say what you reviewed with --reason; nothing was recorded")
	}
	approver := strings.TrimPrefix(strings.TrimSpace(cfg.Autonomy.Approver), "@")
	if approver == "" {
		return headApproval{}, Errorf(ExitBlocked, "inspect --approve: no [autonomy] approver is set; name the person who approves in .aayudha/config.toml")
	}
	c, tok, err := e.HumanGitHub(ctx)
	if err != nil {
		return headApproval{}, err
	}
	if login := viewer(ctx, c); !strings.EqualFold(login, approver) {
		return headApproval{}, Errorf(ExitBlocked, "inspect --approve: the [autonomy] approver is %s, not %s", approver, cmp.Or(login, "an unknown account"))
	}
	owner, name, _ := github.SplitRepo(repo)
	// headOf is the PR's head while it is open and ready for review, else
	// none: a closed or draft PR gets no approval (CodeRabbit on #678).
	headOf := func() (string, error) {
		pr, _, err := c.REST.PullRequests.Get(ctx, owner, name, n)
		if err == nil && (pr.GetState() != "open" || pr.GetDraft()) {
			return "", Errorf(ExitBlocked, "inspect --approve: PR #%d is closed or a draft, so nothing was recorded", n)
		}
		return pr.GetHead().GetSHA(), err
	}
	head, err := headOf()
	var ae *Error
	if errors.As(err, &ae) {
		return headApproval{}, err
	}
	if err != nil {
		return headApproval{}, Wrap(ExitExternal, err)
	}
	if want != "" && head != want {
		return headApproval{}, Errorf(ExitBlocked, "inspect --approve: PR #%d's head moved from %s to %s since you read it, so nothing was recorded; review the new head", n, short(want), short(head))
	}
	// The person approves this head by name through the device flow, always:
	// a terminal without an agent marker is not proof of a person here, since
	// this record is what the gate trusts over a comment written with the
	// person's token (CodeRabbit's Major on #647). A host's agent marker (an
	// orb's Terminal tab) is no refusal: the device code proves the person.
	// aayu's own lanes (AAYU_AGENT=1) and CI are refused. A push after it
	// voids it.
	refusal := Errorf(ExitBlocked, "inspect --approve needs GitHub's device-flow presence check, which is not available here; run it where a person can enter the code")
	lane := ""
	switch {
	case e.Getenv("AAYU_AGENT") == "1":
		lane = "AAYU_AGENT=1"
	case e.CI:
		lane = "--ci"
	}
	if lane != "" {
		refusal = Errorf(ExitBlocked, "inspect --approve is a person's decision and is refused for an agent lane (%s); ask a person to run it in their own terminal", lane)
	}
	if _, err := e.presenceCheck(ctx, Pending{What: "inspect --approve", Effect: []string{fmt.Sprintf("Records your review of PR #%d at commit %s as the human review the merge gate reads.", n, short(head))}}, AccessWrite, refusal); err != nil {
		return headApproval{}, err
	}
	if now, err := headOf(); err != nil || now != head {
		return headApproval{}, Errorf(ExitBlocked, "inspect --approve: PR #%d's head moved from %s while you approved; review the new head and run it again", n, short(head))
	}
	diff, _, err := c.REST.PullRequests.GetRaw(ctx, owner, name, n, gh.RawOptions{Type: gh.Diff})
	if err != nil {
		return headApproval{}, Wrap(ExitExternal, err)
	}
	reviewed := []string{} // every changed path, as the gate reads the diff
	for _, f := range inspect.ParseDiff(diff) {
		reviewed = append(reviewed, f.Path)
	}
	a := headApproval{PR: n, Head: head, By: approver, At: e.Now().UTC().Format(time.RFC3339), Reason: reason}
	reply, _ := json.Marshal(map[string]any{"head": head, "disposition": inspect.Merge, "summary": reason, "reviewed": reviewed, "findings": []any{}, "criteria": []any{}})
	marker := fmt.Sprintf("<!-- aayudha:inspect-approval %s -->", head)
	body := fmt.Sprintf("%s\nHuman review of `%s` by @%s through `aayu inspect %d --approve`, recorded on the ledger (`%s`).\n\n```%s\n%s\n```\n",
		marker, short(head), approver, n, ApprovedPath, inspect.ReplyFence, reply)
	if !c.DryRun() {
		w, err := e.Ledger(ctx, tok, "human", "")
		if err != nil {
			return headApproval{}, err
		}
		b, _ := json.Marshal(a)
		if _, err := w.Write(ctx, ledger.Commit{Message: fmt.Sprintf("inspect: approve #%d at %s", n, short(head)), Append: map[string][]byte{ApprovedPath: append(b, '\n')}}); err != nil {
			return headApproval{}, Wrap(ExitExternal, err)
		}
	}
	if _, err := c.UpsertComment(ctx, repo, n, marker, body, actions.Action{Kind: "github.comment.inspect-approval", Payload: map[string]any{"head": head}}); err != nil {
		return headApproval{}, Wrap(ExitExternal, err)
	}
	return a, nil
}

// paidReviewer is rung B of the review ladder (Decision #469; founder,
// 2026-10-18): [inspect] paid_reviewer, called with its endpoint's key (in
// the merge gate, the model and key the code-review environment names; see
// useReviewModel), only when [inspect] reviewer is ci or, as the fallback,
// when no installed app can review the head (unavailable says why: a usage-limit reply, or none in time); the
// model's family is one the builder cannot be, the key is present, and
// [budget] per_pr_inspect_max caps the spend. Only a PR whose author has write
// access to the repository (perm, the author's effective permission, is admin
// or write; "" when it cannot be read) gets a paid review, so a person who can
// only read cannot open PRs to spend the key. An author association of MEMBER
// or COLLABORATOR is no proof: an organization member may have read access
// only. perm is called only when everything else allows a paid review. nil
// leaves the review to the reviewer apps, or a human.
func paidReviewer(gw *models.Gateway, cfg *config.Config, b inspect.Builder, unavailable string, perm func() string, logf func(string, ...any)) *models.Resolved {
	ic := cfg.Inspect
	m := ic.PaidReviewer
	if m == "" || ic.Reviewer == inspect.ReviewerHuman || ic.Reviewer == inspect.ReviewerModel {
		return nil
	}
	if unavailable == "" && ic.Reviewer != inspect.ReviewerCI {
		return nil // the chosen app reviews while it answers
	}
	f := verbs.FamilyOf(m)
	why := ""
	route, err := models.RouteFor(cfg.Models, m, f)
	switch {
	case b.Rung(f) != inspect.RungA:
		why = fmt.Sprintf("its family (%s) cannot be shown to differ from the builder's", f)
	case cfg.Budget.PerPRInspectMax <= 0:
		why = "[budget] per_pr_inspect_max is 0, so nothing caps its spend"
	case err != nil:
		why = err.Error()
	default:
		if k, err := gw.Key(route.Secret); err != nil || k == "" {
			why = fmt.Sprintf("the merge gate holds no %s key (the repository secret %s)", route.Secret, secrets.EnvVar(route.Secret))
		} else if p := perm(); p != "admin" && p != "write" {
			why = fmt.Sprintf("the PR's author has %s access to the repository, not write, so no paid review is spent on it", cmp.Or(p, "unknown"))
		}
	}
	if why != "" {
		logf("inspect: [inspect] paid_reviewer %s is not used: %s; a reviewer app or a human reviews", m, why)
		return nil
	}
	return &models.Resolved{Lane: verbs.LaneReview, Host: verbs.DirectHost, Model: m, Display: m, Family: f, Effort: reviewEffort(m, ic), Source: models.SourceConfig}
}

// reviewEffort is the reasoning effort a review model is called at
// (founder, 2026-10-18): high, and medium for Claude Opus, Claude Fable, and
// GPT-6 Astra. [inspect] paid_effort, when set, overrides it.
func reviewEffort(model string, ic config.Inspect) string {
	if ic.PaidEffort != "" {
		return ic.PaidEffort
	}
	m := strings.ToLower(model)
	for _, heavy := range []string{"opus", "fable", "astra"} {
		if strings.Contains(m, heavy) {
			return "medium"
		}
	}
	return "high"
}

// reviewCall is the one way a review model is called: the CI review, its
// fallback for an app that cannot review, the review lane's model, and
// build's session review. It asks for fast (priority) serving at
// reviewEffort; an endpoint without tiers ignores the ask. The call is
// charged at the priority rate whatever tier the reply states, so
// per_pr_inspect_max holds. Each reply's tier is appended to *served, which
// the caller turns into a note with tierNote.
func (e *Env) reviewCall(gw *models.Gateway, cfg *config.Config, model, story string, served *[]string) func(context.Context, models.Request) (models.Response, float64, error) {
	g := gw.WithModel(verbs.LaneReview, verbs.DirectHost, model)
	return func(ctx context.Context, req models.Request) (models.Response, float64, error) {
		req.Effort = cmp.Or(req.Effort, reviewEffort(model, cfg.Inspect))
		req.ServiceTier = models.TierPriority
		resp, rm, err := g.Do(ctx, models.Call{Story: story, Seat: string(verbs.EngineeringManager), Lane: verbs.LaneReview, Host: verbs.DirectHost, Req: req})
		usd, _ := models.Cost(cfg.Models.Prices, rm.Model, models.Billed(req, resp))
		if err == nil {
			*served = append(*served, resp.ServiceTier)
			// The run's log shows what the review was served at and used; a
			// subscription credential makes the USD an estimate.
			fmt.Fprintf(e.Err, "inspect: %s replied (service tier %s): %d tokens in, %d out, %.4f USD at list price\n",
				cmp.Or(resp.Model, rm.Model), cmp.Or(resp.ServiceTier, "not stated"), resp.TokensIn, resp.TokensOut, usd)
		}
		return resp, usd, err
	}
}

// useReviewModel points the CI review at the model the code-review
// environment names (inspect.ReviewModelURLVar, inspect.ReviewModelVar):
// the gateway route, with inspect.ReviewModelSecret as its key, and
// [inspect] paid_reviewer. A URL with credentials or a query is refused, and
// never echoed, so no log or comment shows a secret; only its host is named.
// inspect.ReviewModelStyleVar sets the endpoint's wire format. A bad setting
// fails inspect only under reviewer = "ci"; when an app reviews, it leaves
// the CI fallback unavailable, so the app's review is still read.
func (e *Env) useReviewModel(c *config.Config) error {
	if r := c.Inspect.Reviewer; r == inspect.ReviewerModel || r == inspect.ReviewerHuman {
		return nil // the review lane's configured gateway, or no model at all
	}
	if raw := e.Getenv(inspect.ReviewModelURLVar); raw != "" {
		style := e.Getenv(inspect.ReviewModelStyleVar)
		var bad error
		if u, err := url.Parse(raw); err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || operationPath(u.Path) {
			bad = fmt.Errorf("%s must be the model endpoint's https base URL (such as %s), ending before /chat/completions, /responses, or /messages, with no credentials or query in it; the key goes in %s",
				inspect.ReviewModelURLVar, inspect.RecommendedReviewURL, inspect.ReviewModelKeyVar)
		} else if !slices.Contains([]string{"", models.StyleMessages, models.StyleResponses, models.StyleChat}, style) {
			bad = fmt.Errorf("%s = %q: want messages, responses, or chat", inspect.ReviewModelStyleVar, style)
		}
		if bad != nil {
			if c.Inspect.Reviewer != inspect.ReviewerCI {
				e.Logf("inspect: the CI review fallback is unavailable: %v", bad)
				return nil
			}
			return bad
		}
		// A different endpoint: no wire-format override of the old gateway carries over; the same one keeps them.
		g := config.Gateway{BaseURL: raw, Secret: inspect.ReviewModelSecret}
		if raw == c.Models.Gateway.BaseURL {
			g.APIStyle, g.Styles = c.Models.Gateway.APIStyle, c.Models.Gateway.Styles
		}
		if style != "" {
			g.APIStyle, g.Styles = style, nil
		}
		c.Models.Gateway = g
	}
	if m := e.Getenv(inspect.ReviewModelVar); m != "" {
		c.Inspect.PaidReviewer = m
	}
	return nil
}

// operationPath reports whether p ends in a model API operation, which the
// client appends itself, so a URL ending in one would be called twice over.
func operationPath(p string) bool {
	p = strings.TrimSuffix(p, "/")
	for _, op := range []string{"/chat/completions", "/responses", "/messages"} {
		if strings.HasSuffix(p, op) {
			return true
		}
	}
	return false
}

// missingReviewModel names what the CI review lacks: the endpoint, the
// model, or the key.
func missingReviewModel(gw *models.Gateway, c *config.Config) []string {
	var miss []string
	if c.Models.Gateway.Secret != inspect.ReviewModelSecret {
		miss = append(miss, "the variable "+inspect.ReviewModelURLVar)
	}
	if c.Inspect.PaidReviewer == "" {
		miss = append(miss, "the variable "+inspect.ReviewModelVar)
	}
	if k, err := gw.Key(inspect.ReviewModelSecret); err != nil || k == "" {
		miss = append(miss, "the secret "+inspect.ReviewModelKeyVar)
	}
	return miss
}

// tierNote records the service tier the paid review was served at, and flags
// one that is not fast: the review asked for priority (fast) serving.
func tierNote(served []string) string {
	var slow []string
	for _, t := range served {
		if t != models.TierPriority && t != "fast" {
			slow = append(slow, cmp.Or(t, "none stated"))
		}
	}
	if len(slow) > 0 {
		return fmt.Sprintf("**not fast:** the review asked for service tier %s (fast) but was served at %s; it is charged at the %s rate", models.TierPriority, strings.Join(slow, ", "), models.TierPriority)
	}
	return fmt.Sprintf("served at service tier %s (fast), as asked", served[len(served)-1])
}

// viewer is the account c writes as, or "" when the token cannot say (a
// workflow's GITHUB_TOKEN; gateWriter names that one).
func viewer(ctx context.Context, c *github.Client) string {
	if u, _, err := c.REST.Users.Get(ctx, ""); err == nil {
		return u.GetLogin()
	}
	return ""
}

// inspectWriter reports whether a comment by login can carry inspect's state:
// the merge gate's identity (gateWriter) or self, the account this run writes
// as. Anyone can post the marker, so a forged state would otherwise lift the
// cycle cap, pass a cached merge as this head's review, or feed findings to
// --dismiss, the inbox, a delegated agent's follow-up, or a brief (Bug #601).
func inspectWriter(login, self string, id config.Identity) bool {
	return (self != "" && strings.EqualFold(login, self)) || gateWriter(login, id)
}

// inspectState is the state in the last inspect comment a trusted writer
// posted on a PR (inspectWriter); a zero state when there is none.
func inspectState(comments []*gh.IssueComment, self string, id config.Identity) inspect.State {
	var st inspect.State
	for _, cm := range comments {
		if strings.Contains(cm.GetBody(), inspect.Marker) && inspectWriter(cm.GetUser().GetLogin(), self, id) {
			st = inspect.ParseState(cm.GetBody())
		}
	}
	return st
}

// gateWriter reports whether login is an account the merge gate writes its
// inspect comment as: the workflow's token, the bot App, or the machine user.
func gateWriter(login string, id config.Identity) bool {
	return strings.EqualFold(login, "github-actions[bot]") ||
		(id.AppSlug != "" && strings.EqualFold(login, id.AppSlug+"[bot]")) ||
		(id.MachineUser != "" && strings.EqualFold(login, id.MachineUser))
}
