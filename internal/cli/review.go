package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/identity"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/models"
	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
	"github.com/rahulnakmol/agentic-ayudha/internal/ready"
	"github.com/rahulnakmol/agentic-ayudha/internal/uireview"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
	"github.com/rahulnakmol/agentic-ayudha/internal/verify"
)

func init() {
	register("review", runReview)
	register("polish", runPolish)
	register("baseline", runBaseline)
}

// ReviewResult is what review and polish report.
type ReviewResult struct {
	PR       int                  `json:"pr"`
	Critique *uireview.Critique   `json:"critique"`
	Path     string               `json:"critique_path"`
	Polish   *uireview.Comparison `json:"polish,omitempty"`
}

// reviewDir is where review keeps a PR's latest critique and captures:
// .aayudha/review/<pr>/ (git-ignored runtime state).
func reviewDir(root string, pr int) string {
	// Shared by every worktree of the clone, like the actions log, so polish in
	// .aayudha/worktrees/pr-<n> finds the critique review wrote.
	return filepath.Join(mainCheckout(root), ".aayudha", "review", fmt.Sprint(pr))
}

// uiBrowser is the capture and impeccable runner in a throwaway sandbox; the caller must call done.
func (e *Env) uiBrowser(root string, minTarget int) (*uireview.Browser, func(), error) {
	home, err := os.MkdirTemp("", "aayudha-review-home-")
	if err != nil {
		return nil, nil, err
	}
	work, err := os.MkdirTemp("", "aayudha-review-")
	if err != nil {
		_ = os.RemoveAll(home)
		return nil, nil, err
	}
	b := &uireview.Browser{Sandbox: &verify.Sandbox{Getenv: e.Getenv, Home: home}, Root: root, Work: work, MinTargetPx: minTarget, Getenv: e.Getenv}
	return b, func() { _ = os.RemoveAll(home); _ = os.RemoveAll(work) }, nil
}

// reviewSetup resolves what review and polish judge: the PR (the checkout
// must be its head), its story, and the UI states, which are the screenshot
// and recording criteria of the story's Ready snapshot on the
// [verify.environments] target (never production).
func (e *Env) reviewSetup(ctx context.Context, n int) (head string, story int, states []uireview.State, err error) {
	root := config.RepoRoot(e.Dir)
	if root == "" {
		return "", 0, nil, Errorf(ExitUsage, "review runs in a checkout of the change")
	}
	l, err := e.Config()
	if err != nil {
		return "", 0, nil, err
	}
	repo, err := e.RepoName(ctx)
	if err != nil {
		return "", 0, nil, err
	}
	if head, err = gitIn(ctx, root, "rev-parse", "HEAD"); err != nil {
		return "", 0, nil, Wrap(ExitUsage, err)
	}
	c, tok, err := e.readClient(ctx)
	if err != nil {
		return "", 0, nil, err
	}
	owner, name, err := github.SplitRepo(repo)
	if err != nil {
		return "", 0, nil, Wrap(ExitUsage, err)
	}
	pr, _, err := c.REST.PullRequests.Get(ctx, owner, name, n)
	if err != nil {
		return "", 0, nil, Wrap(ExitExternal, err)
	}
	if pr.GetHead().GetSHA() != head {
		return "", 0, nil, Errorf(ExitBlocked, "review %d: this checkout is at %s, but the PR head is %s; check out the PR head first", n, short(head), short(pr.GetHead().GetSHA()))
	}
	msg, _ := gitIn(ctx, root, "log", "-1", "--format=%B", head)
	story = StoryOf(pr.GetBody(), msg, pr.GetHead().GetRef())
	r := &verify.Run{Verify: l.Config.Verify, Env: e.Getenv}
	if story > 0 {
		if w, err := e.Ledger(ctx, tok, "human", ""); err == nil {
			if t, err := w.Read(ctx); err == nil {
				if b, err := t.File(ctx, ready.LedgerPath(story)); err == nil {
					if snap, err := ready.ParseSnapshot(string(b)); err == nil {
						r.Criteria = snap.Criteria
					}
				}
				t.Close()
			}
		}
	}
	t, err := verify.ResolveTarget(r)
	if err != nil {
		return "", 0, nil, Wrap(ExitBlocked, err)
	}
	for _, s := range verify.States(r, t) {
		states = append(states, uireview.State{ID: fmt.Sprint(s.N), URL: s.URL})
	}
	if len(states) == 0 {
		return "", 0, nil, Errorf(ExitBlocked, "review %d: no UI state to review: the story's Ready snapshot has no screenshot or recording criteria, or [verify.environments.<AAYU_ENV>] has no url", n)
	}
	return head, story, states, nil
}

// reviewer builds the two lanes: lane A on the ui-review lane through the
// model gateway, its family checked against the builder's (reviewer ≠ builder).
// builderModel is the head's builder when the caller knows it; else the
// story's local pack names it, if that pack proves the current head.
func (e *Env) reviewer(ctx context.Context, b *uireview.Browser, story int, deep bool, builderModel string) (*uireview.Reviewer, error) {
	l, err := e.Config()
	if err != nil {
		return nil, err
	}
	gw, err := e.Gateway()
	if err != nil {
		return nil, err
	}
	rev, err := gw.Resolver.ResolveDirect(verbs.LaneUIReview, verbs.DirectHost, false)
	if err != nil {
		return nil, Wrap(ExitUsage, err)
	}
	host := verbs.HostID(e.Host)
	if host == "" {
		host = verbs.DirectHost
	}
	builder := verbs.FamilyOf(builderModel)
	if root := config.RepoRoot(e.Dir); builder == "" && root != "" && story > 0 {
		head, _ := gitIn(ctx, root, "rev-parse", "HEAD")
		if p, err := proof.LoadLocal(proof.LocalDir(root, story)); err == nil && p.Model != "" && p.PRHeadSHA == head {
			builder = verbs.FamilyOf(p.Model)
		}
	}
	if builder == "" {
		if bm, err := gw.Resolver.Resolve(verbs.LaneBuild, host); err == nil {
			builder = bm.Family
		}
	}
	key := fmt.Sprintf("#%d", story)
	if err := models.CheckFamilies(key, builder, rev.Family); err != nil {
		return nil, Wrap(ExitBlocked, err)
	}
	// The lane's effort (high for gemini-3.8-flash); --deep raises a lower one.
	effort := rev.Effort
	if effort == "" {
		effort = "medium"
	}
	if deep {
		effort = "high"
	}
	return &uireview.Reviewer{Tools: b, Model: rev.Model, Effort: effort, K: uireview.K, MinTargetPx: l.Config.Verify.Floors.A11y.MinTargetPx, Now: e.Now,
		Complete: func(ctx context.Context, req models.Request) (models.Response, error) {
			resp, _, err := gw.Do(ctx, models.Call{Story: key, Seat: string(verbs.UXResearcher), Lane: verbs.LaneUIReview, Host: verbs.DirectHost, Req: req})
			return resp, err
		}}, nil
}

// review runs both lanes on the PR's states and keeps the critique and the
// screenshots lane A saw in .aayudha/review/<pr>/.
func (e *Env) review(ctx context.Context, n int, deep bool) (*uireview.Critique, string, error) {
	head, story, states, err := e.reviewSetup(ctx, n)
	if err != nil {
		return nil, "", err
	}
	root := config.RepoRoot(e.Dir)
	l, err := e.Config()
	if err != nil {
		return nil, "", err
	}
	b, done, err := e.uiBrowser(root, l.Config.Verify.Floors.A11y.MinTargetPx)
	if err != nil {
		return nil, "", err
	}
	defer done()
	r, err := e.reviewer(ctx, b, story, deep, "")
	if err != nil {
		return nil, "", err
	}
	crit, caps, err := r.Review(ctx, states)
	if err != nil {
		if errors.Is(err, models.ErrBudget) {
			return nil, "", Wrap(ExitBudget, err)
		}
		return nil, "", Wrap(ExitExternal, err)
	}
	crit.Repo, _ = e.RepoName(ctx)
	crit.PR, crit.Head = n, head
	dir := reviewDir(root, n)
	if err := os.MkdirAll(filepath.Join(dir, "captures"), 0o755); err != nil {
		return nil, "", err
	}
	for _, c := range caps {
		for _, p := range []string{c.Rest, c.Focus} {
			if b, err := os.ReadFile(p); err == nil && p != "" {
				_ = os.WriteFile(filepath.Join(dir, "captures", filepath.Base(p)), b, 0o644)
			}
		}
	}
	return crit, dir, nil
}

func (e *Env) postReview(ctx context.Context, n int, crit *uireview.Critique, cmp *uireview.Comparison) error {
	repo, err := e.RepoName(ctx)
	if err != nil {
		return err
	}
	c, err := e.inspectClient(ctx)
	if err != nil {
		return err
	}
	_, err = c.UpsertComment(ctx, repo, n, uireview.Marker, uireview.Comment(crit, cmp),
		actions.Action{Kind: "github.comment.review", Payload: map[string]any{"disposition": crit.Disposition, "head": crit.Head}})
	return err
}

func saveCritique(dir string, crit *uireview.Critique) (string, error) {
	b, _ := json.MarshalIndent(crit, "", "  ")
	p := filepath.Join(dir, "critique.json")
	return p, os.WriteFile(p, append(b, '\n'), 0o644)
}

// runReview critiques a PR's UI with both lanes (WP6.6).
func runReview(ctx context.Context, e *Env, in Values) error {
	n := issueNumber(in.String("pr"))
	crit, dir, err := e.review(ctx, n, in.Bool("deep"))
	if err != nil {
		return err
	}
	res := ReviewResult{PR: n, Critique: crit}
	if res.Path, err = saveCritique(dir, crit); err != nil {
		return err
	}
	if err := e.postReview(ctx, n, crit, nil); err != nil {
		return err
	}
	if err := e.Emit(res, func(w io.Writer) error { return renderReview(w, res) }); err != nil {
		return err
	}
	if crit.Disposition != uireview.Ship {
		return Errorf(ExitBlocked, "review #%d: %s", n, crit.Disposition)
	}
	return nil
}

// runPolish re-runs review after the host applied the critique and reports
// which of its findings are gone; any that remain exit 4.
func runPolish(ctx context.Context, e *Env, in Values) error {
	n := issueNumber(in.String("pr"))
	root := config.RepoRoot(e.Dir)
	if root == "" {
		return Errorf(ExitUsage, "polish runs in a checkout of the change")
	}
	raw, err := os.ReadFile(filepath.Join(reviewDir(root, n), "critique.json"))
	if err != nil {
		return Errorf(ExitBlocked, "polish #%d: no critique to apply; run `aayu review %d` first", n, n)
	}
	if wt, err := e.polishWorktree(ctx, root, n); err != nil {
		return err
	} else if wt != "" {
		return Errorf(ExitBlocked, "polish #%d: this checkout is not the PR head, so the critique (%s) is applied in an aayu worktree: %s; apply it there, commit and push, then run `aayu polish %d` in that worktree", n, filepath.Join(reviewDir(root, n), "critique.json"), wt, n)
	}
	var before uireview.Critique
	if err := json.Unmarshal(raw, &before); err != nil {
		return Errorf(ExitUsage, "polish #%d: %v", n, err)
	}
	crit, dir, err := e.review(ctx, n, false)
	if err != nil {
		return err
	}
	cmp := uireview.Compare(&before, crit)
	res := ReviewResult{PR: n, Critique: crit, Polish: &cmp}
	if res.Path, err = saveCritique(dir, crit); err != nil {
		return err
	}
	if err := e.postReview(ctx, n, crit, &cmp); err != nil {
		return err
	}
	if err := e.Emit(res, func(w io.Writer) error { return renderReview(w, res) }); err != nil {
		return err
	}
	if len(cmp.Remaining) > 0 {
		return Errorf(ExitBlocked, "polish #%d: %d finding(s) of the critique remain", n, len(cmp.Remaining))
	}
	return nil
}

// polishWorktree gives polish a worktree of the PR head when this checkout is
// not at it (WP5.5b: code-changing verbs never write in the main checkout):
// .aayudha/worktrees/pr-<n> on the PR's branch (the critique is shared, reviewDir).
// It returns "" when this checkout is the PR head, or is already an aayu
// worktree (whose head the host must push before polish re-reviews).
func (e *Env) polishWorktree(ctx context.Context, root string, n int) (string, error) {
	main := mainCheckout(root)
	if strings.HasPrefix(root, filepath.Join(main, ".aayudha", "worktrees")+string(filepath.Separator)) {
		return "", nil
	}
	repo, err := e.RepoName(ctx)
	if err != nil {
		return "", err
	}
	c, _, err := e.readClient(ctx)
	if err != nil {
		return "", err
	}
	owner, name, _ := github.SplitRepo(repo)
	pr, _, err := c.REST.PullRequests.Get(ctx, owner, name, n)
	if err != nil {
		return "", Wrap(ExitExternal, err)
	}
	if head, _ := gitIn(ctx, root, "rev-parse", "HEAD"); head == pr.GetHead().GetSHA() {
		return "", nil
	}
	ref := pr.GetHead().GetRef()
	wt := filepath.Join(main, ".aayudha", "worktrees", fmt.Sprintf("pr-%d", n))
	if _, err := run2(ctx, main, "git", "fetch", "--quiet", "origin", "+refs/heads/"+ref+":refs/remotes/origin/"+ref); err != nil {
		return "", Wrap(ExitExternal, err)
	}
	if _, err := os.Stat(wt); err != nil {
		if _, err := run2(ctx, main, "git", "worktree", "add", "--quiet", "-B", ref, wt, "origin/"+ref); err != nil {
			return "", Wrap(ExitExternal, err)
		}
	} else if _, err := run2(ctx, wt, "git", "merge", "--quiet", "--ff-only", "origin/"+ref); err != nil {
		return "", Wrap(ExitBlocked, fmt.Errorf("polish #%d: %s has local work that is not on origin/%s: %w", n, wt, ref, err))
	}
	return wt, nil
}

func renderReview(w io.Writer, r ReviewResult) error {
	c := r.Critique
	fmt.Fprintf(w, "review #%d at %s: %s (lane A %s, lane B %s)\n", r.PR, short(c.Head), c.Disposition, c.LaneA.Disposition, c.LaneB.Disposition)
	for _, l := range []uireview.Lane{c.LaneA, c.LaneB} {
		for _, f := range l.Findings {
			fmt.Fprintf(w, "  lane %s state %s %s (severity %d): %s [%s]\n", l.Name, f.State, f.Kind, f.Severity, f.Summary, f.Source)
		}
		for _, x := range l.Errors {
			fmt.Fprintf(w, "  lane %s error: %s\n", l.Name, x)
		}
	}
	if p := r.Polish; p != nil {
		fmt.Fprintf(w, "polish: %d resolved, %d remaining, %d new\n", len(p.Resolved), len(p.Remaining), len(p.New))
		for _, f := range p.Remaining {
			fmt.Fprintf(w, "  remaining: state %s %s\n", f.State, f.Kind)
		}
	}
	fmt.Fprintf(w, "critique: %s\n", r.Path)
	return nil
}

// BaselineResult is what baseline reports.
type BaselineResult struct {
	SHA    string           `json:"sha"`
	Path   string           `json:"ledger_path"`
	Commit string           `json:"ledger_commit,omitempty"`
	Files  []proof.Artifact `json:"files"`
	DryRun bool             `json:"dry_run,omitempty"`
}

// runBaseline captures [review] baseline paths at a commit and pins the
// screenshots on the ledger under baselines/<sha>/ (WP6.6). The checkout must
// be at that commit.
func runBaseline(ctx context.Context, e *Env, in Values) error {
	root := config.RepoRoot(e.Dir)
	if root == "" {
		return Errorf(ExitUsage, "baseline runs in a checkout")
	}
	l, err := e.Config()
	if err != nil {
		return err
	}
	head, err := gitIn(ctx, root, "rev-parse", "HEAD")
	if err != nil {
		return Wrap(ExitUsage, err)
	}
	sha := head
	if s := in.String("sha"); s != "" {
		if sha, err = gitIn(ctx, root, "rev-parse", "--verify", s+"^{commit}"); err != nil {
			return Errorf(ExitUsage, "baseline: --sha %s: %v", s, err)
		}
		if sha != head {
			return Errorf(ExitBlocked, "baseline: the checkout is at %s, not %s; check it out and start the app first", short(head), short(sha))
		}
	}
	t, err := verify.ResolveTarget(&verify.Run{Verify: l.Config.Verify, Env: e.Getenv})
	if err != nil {
		return Wrap(ExitBlocked, err)
	}
	if t.URL == "" || len(l.Config.Review.Baseline) == 0 {
		return Errorf(ExitBlocked, "baseline: set [verify.environments.%s] url and [review] baseline paths", t.Env)
	}
	var states []uireview.State
	for _, p := range l.Config.Review.Baseline {
		id := slug(p)
		if id == "" {
			id = "root"
		}
		states = append(states, uireview.State{ID: id, URL: t.URL + p})
	}
	b, done, err := e.uiBrowser(root, l.Config.Verify.Floors.A11y.MinTargetPx)
	if err != nil {
		return err
	}
	defer done()
	caps, err := b.Capture(ctx, states)
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	c, bot, err := e.BotGitHub(ctx, "")
	if err != nil {
		return err
	}
	ident := "bot"
	if bot.Kind == identity.KindHumanFallback {
		ident = identity.KindHumanFallback
	}
	store, err := e.proofStore(ctx, c, bot.Token, ident, bot.Tag)
	if err != nil {
		return err
	}
	repo, _ := e.RepoName(ctx)
	dir := path.Join("baselines", sha)
	res := BaselineResult{SHA: sha, Path: path.Join(dir, "manifest.json"), DryRun: c.DryRun()}
	commit := ledger.Commit{Message: "baseline: " + short(sha), Files: map[string][]byte{}}
	lfs := false
	for _, cp := range caps {
		if cp.Error != "" {
			return Errorf(ExitExternal, "baseline: %s (%s): %s", cp.ID, cp.URL, cp.Error)
		}
		data, err := os.ReadFile(cp.Rest)
		if err != nil {
			return err
		}
		a := proof.NewArtifact(cp.ID+".png", "screenshot", data)
		switch {
		case store.Bucket != nil:
			a.Store, a.Object = proof.StoreBucket, store.Bucket.Key(repo, 0, a.SHA256)
			if !c.DryRun() {
				if err := store.Bucket.Put(ctx, a.Object, data); err != nil {
					return Wrap(ExitExternal, err)
				}
			}
		case store.Private:
			a.Store, lfs = proof.StoreLFS, true
			commit.Files[path.Join(dir, a.Path)] = data
		default:
			a.Store = proof.StoreNone
		}
		res.Files = append(res.Files, a)
	}
	m, _ := json.MarshalIndent(map[string]any{"schema_version": 1, "repo": repo, "sha": sha, "created_at": e.Now().UTC().Format("2006-01-02T15:04:05Z"),
		"states": states, "files": res.Files}, "", "  ")
	commit.Files[res.Path] = append(m, '\n')
	w := *store.Ledger
	w.LFS = lfs
	if res.Commit, err = w.Write(ctx, commit); err != nil {
		return err
	}
	return e.Emit(res, func(w io.Writer) error {
		fmt.Fprintf(w, "baseline %s: %d state(s)", short(sha), len(res.Files))
		if res.Commit != "" {
			fmt.Fprintf(w, "; ledger %s (%s)", res.Path, short(res.Commit))
		}
		fmt.Fprintln(w)
		for _, a := range res.Files {
			fmt.Fprintf(w, "  %s %s (%s)\n", a.Path, a.SHA256[:12], a.Store)
		}
		return nil
	})
}
