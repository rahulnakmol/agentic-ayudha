package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/identity"
	"github.com/rahulnakmol/agentic-ayudha/internal/initrepo"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
	"github.com/rahulnakmol/agentic-ayudha/internal/ready"
)

func init() { register("proof", runProof) }

// ProofMarker marks the replace-in-place proof comment on a PR and its issue.
const ProofMarker = "<!-- aayudha:proof -->"

// MergeGateWorkflow is the workflow whose CI-written packs are authoritative (WP4.8).
const MergeGateWorkflow = initrepo.MergeGateWorkflow

// DefaultMediaSecret names the secret holding the bucket's "ACCESS_KEY_ID:SECRET_ACCESS_KEY".
const DefaultMediaSecret = "proof-media"

// visibility reports whether the repository is private: from init's state
// when present, else from the API. Unknown counts as public (V3).
func (e *Env) private(ctx context.Context, c *github.Client, repo string) bool {
	if root := config.RepoRoot(e.Dir); root != "" {
		if st, err := initrepo.LoadState(root); err == nil && st != nil && st.Visibility != "" {
			return st.Visibility == "private"
		}
	}
	if c == nil {
		return false
	}
	owner, name, err := github.SplitRepo(repo)
	if err != nil {
		return false
	}
	r, _, err := c.REST.Repositories.Get(ctx, owner, name)
	return err == nil && r.GetPrivate()
}

// proofStore builds the repository's proof store: its ledger under tok, and
// the [proof] media bucket with credentials from the secrets store.
func (e *Env) proofStore(ctx context.Context, c *github.Client, tok identity.Token, ident, tag string) (*proof.Store, error) {
	l, err := e.Config()
	if err != nil {
		return nil, err
	}
	repo, err := e.RepoName(ctx)
	if err != nil {
		return nil, err
	}
	w, err := e.Ledger(ctx, tok, ident, tag)
	if err != nil {
		return nil, err
	}
	s := &proof.Store{Repo: repo, Ledger: w, Private: e.private(ctx, c, repo), RetentionDays: l.Config.Proof.RetentionDays, Now: e.Now}
	if m := l.Config.Proof.Media; m != "" {
		if s.Bucket, err = proof.ParseMedia(m); err != nil {
			return nil, Wrap(ExitUsage, err)
		}
		name := l.Config.Proof.MediaSecret
		if name == "" {
			name = DefaultMediaSecret
		}
		v, _, err := e.SecretStore().Get(name)
		if err != nil || v == "" {
			return nil, Errorf(ExitBlocked, "[proof] media is %s, but secret %q (ACCESS_KEY_ID:SECRET_ACCESS_KEY) is not set: `aayu auth set-secret %s`", m, name, name)
		}
		ak, sk, ok := strings.Cut(strings.TrimSpace(v), ":")
		if !ok {
			return nil, Errorf(ExitUsage, "secret %q must be ACCESS_KEY_ID:SECRET_ACCESS_KEY", name)
		}
		s.Bucket.AccessKey, s.Bucket.SecretKey, s.Bucket.HTTP, s.Bucket.Now = ak, sk, e.HTTP, e.Now
	}
	return s, nil
}

// mediaRefused is the V3 refusal that needs no network: a public repository
// (or one of unknown visibility) without a bucket never pushes media.
func (e *Env) mediaRefused() (bool, error) {
	l, err := e.Config()
	if err != nil {
		return false, err
	}
	if l.Config.Proof.Media != "" {
		return false, nil
	}
	if root := config.RepoRoot(e.Dir); root != "" {
		if st, err := initrepo.LoadState(root); err == nil && st != nil {
			return st.Visibility != "private", nil
		}
	}
	return false, nil // decided against the API in Store.Push
}

// authority is the authority of a pack pushed by this process: authoritative
// only from the merge gate workflow in CI (WP4.8), advisory otherwise.
func (e *Env) authority() proof.Authority {
	ref := e.Getenv("GITHUB_WORKFLOW_REF")
	if e.CI && strings.Contains(ref, "/.github/workflows/"+MergeGateWorkflow+"@") {
		var run int64
		_, _ = fmt.Sscan(e.Getenv("GITHUB_RUN_ID"), &run)
		return proof.Authority{Kind: proof.Authoritative, Workflow: ".github/workflows/" + MergeGateWorkflow, RunID: run}
	}
	return proof.Authority{Kind: proof.Advisory}
}

func runProof(ctx context.Context, e *Env, in Values) error {
	action := in.String("action")
	story := 0
	if in.Has("story") {
		story = issueNumber(in.String("story"))
	}
	if action == "push" && in.Bool("media") {
		if refused, err := e.mediaRefused(); err != nil {
			return err
		} else if refused {
			return Errorf(ExitBlocked, "proof push --media: %v", proof.ErrMediaRefused)
		}
	}
	if story == 0 && (action == "push" || action == "get" || action == "verify") {
		return Errorf(ExitUsage, "proof %s needs a story", action)
	}
	repo, err := e.RepoName(ctx)
	if err != nil {
		return err
	}
	switch action {
	case "push":
		return e.proofPush(ctx, repo, story, in.Bool("media"))
	case "purge":
		c, bot, err := e.BotGitHub(ctx, "")
		if err != nil {
			return err
		}
		s, err := e.proofStore(ctx, c, bot.Token, botIdent(bot), bot.Tag)
		if err != nil {
			return err
		}
		t, err := s.Ledger.Read(ctx)
		if err != nil {
			return ledgerErr(err)
		}
		defer t.Close()
		reason := fmt.Sprintf("retention: older than %d days", s.RetentionDays)
		if story > 0 {
			reason = "purged on request (aayu proof purge #" + fmt.Sprint(story) + ")"
		}
		res, err := s.Purge(ctx, t, story, reason)
		if err != nil {
			return Wrap(ExitExternal, err)
		}
		return e.Emit(res, func(w io.Writer) error {
			_, err := fmt.Fprintf(w, "purged %d bucket object(s) and %d LFS file(s); tombstoned %d pack(s)\n", len(res.Objects), len(res.Files), len(res.Packs))
			return err
		})
	}
	// get, list, verify read the ledger with the human's token.
	c, tok, err := e.HumanGitHub(ctx)
	if err != nil {
		return err
	}
	s, err := e.proofStore(ctx, c, tok, "human", "")
	if err != nil {
		return err
	}
	t, err := s.Ledger.Read(ctx)
	if err != nil {
		return ledgerErr(err)
	}
	defer t.Close()
	switch action {
	case "list":
		es, err := s.Packs(ctx, t, story)
		if err != nil {
			return err
		}
		if es == nil {
			es = []proof.Entry{}
		}
		return e.Emit(es, func(w io.Writer) error {
			if len(es) == 0 {
				_, err := fmt.Fprintln(w, "no proof packs on the ledger")
				return err
			}
			for _, x := range es {
				fmt.Fprintf(w, "#%d  %s  %-13s  floors hold: %-5t  %s\n", x.Pack.Story, short(x.Pack.PRHeadSHA), x.Pack.Authority.Kind, x.Pack.FloorsHold(), x.Path)
			}
			return nil
		})
	case "get":
		x, err := s.Best(ctx, t, story)
		if errors.Is(err, proof.ErrNoPack) {
			return Wrap(ExitBlocked, err)
		}
		if err != nil {
			return err
		}
		return e.Emit(x, func(w io.Writer) error {
			b, err := proof.Encode(x.Pack)
			if err != nil {
				return err
			}
			fmt.Fprintf(w, "%s\n", x.Path)
			_, err = w.Write(b)
			return err
		})
	case "verify":
		x, err := s.Best(ctx, t, story)
		if errors.Is(err, proof.ErrNoPack) {
			return Wrap(ExitBlocked, err)
		}
		if err != nil {
			return err
		}
		exp := proof.Expect{}
		if b, err := t.File(ctx, ready.LedgerPath(story)); err == nil {
			snap, err := ready.ParseSnapshot(string(b))
			if err != nil {
				return err
			}
			exp.CompletionGoalHash, exp.Criteria = snap.CompletionGoalHash, snap.Numbers()
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if x.Pack.PR > 0 {
			owner, name, _ := github.SplitRepo(repo)
			pr, _, err := c.REST.PullRequests.Get(ctx, owner, name, x.Pack.PR)
			if err != nil {
				return Wrap(ExitExternal, err)
			}
			exp.HeadSHA = pr.GetHead().GetSHA()
		}
		rep := proof.Verify(x.Pack, exp, s.Opener(ctx, t, x))
		if exp.CompletionGoalHash == "" {
			rep.Problems = append(rep.Problems, fmt.Sprintf("no Ready snapshot at ledger %s to check the Completion Goal against", ready.LedgerPath(story)))
			rep.OK = false
		}
		if err := e.Emit(rep, func(w io.Writer) error {
			fmt.Fprintf(w, "#%d %s pack at %s: %d artifact(s) hashed, %d not stored; floors hold: %t\n", story, rep.Authority, short(rep.HeadSHA), rep.Hashed, rep.NotStored, rep.FloorsHold)
			for _, p := range rep.Problems {
				fmt.Fprintf(w, "  - %s\n", p)
			}
			return nil
		}); err != nil {
			return err
		}
		if !rep.OK {
			return Errorf(ExitBlocked, "#%d: proof verify found %d problem(s)", story, len(rep.Problems))
		}
		return nil
	}
	return Errorf(ExitUsage, "proof: unknown action %q", action)
}

func ledgerErr(err error) error {
	if errors.Is(err, ledger.ErrNoLedger) {
		return Wrap(ExitBlocked, err)
	}
	return Wrap(ExitExternal, err)
}

func botIdent(b identity.Bot) string {
	if b.Kind == identity.KindHumanFallback {
		return identity.KindHumanFallback
	}
	return "bot"
}

// proofPush lands the local pack (.aayudha/proof/<story>/) on the ledger and
// posts the replace-in-place proof comment on the PR and the issue.
func (e *Env) proofPush(ctx context.Context, repo string, story int, media bool) error {
	root := config.RepoRoot(e.Dir)
	if root == "" {
		return Errorf(ExitUsage, "proof push needs a checkout holding the pack (.aayudha/proof/%d/)", story)
	}
	dir := proof.LocalDir(root, story)
	if _, err := os.Stat(dir); err != nil {
		return Errorf(ExitBlocked, "no local pack for #%d in %s: run `aayu verify` first", story, dir)
	}
	var (
		c   *github.Client
		bot identity.Bot
		err error
	)
	if e.CI && e.Getenv("AAYU_BOT_TOKEN") == "" {
		return Errorf(ExitBlocked, "proof push --ci needs AAYU_BOT_TOKEN: CI writes the ledger only with the bot App's token, never the workflow token (Appendix B D1)")
	}
	if c, bot, err = e.BotGitHub(ctx, ""); err != nil {
		return err
	}
	s, err := e.proofStore(ctx, c, bot.Token, botIdent(bot), bot.Tag)
	if err != nil {
		return err
	}
	res, err := s.Push(ctx, dir, media, e.authority())
	if errors.Is(err, proof.ErrMediaRefused) {
		return Errorf(ExitBlocked, "proof push --media: %v", err)
	}
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	p, _ := proof.LoadLocal(dir)
	body := ProofComment(repo, res, p)
	targets := []int{story}
	if p != nil && p.PR > 0 {
		targets = append(targets, p.PR)
	}
	for _, n := range targets {
		if _, err := c.UpsertComment(ctx, repo, n, ProofMarker, body, actions.Action{Kind: "github.comment.proof", Payload: map[string]any{"pack": res.Path, "authority": res.Authority}}); err != nil {
			return err
		}
	}
	return e.Emit(res, func(w io.Writer) error {
		fmt.Fprintf(w, "#%d: %s pack for %s pushed to ledger %s (%s)\n", story, res.Authority, short(res.HeadSHA), res.Path, short(res.Commit))
		for _, st := range res.Stores {
			fmt.Fprintf(w, "  %s\n", st)
		}
		return nil
	})
}

// ProofComment renders the replace-in-place proof comment.
func ProofComment(repo string, res proof.PushResult, p *proof.Pack) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n### Proof pack (%s)\n\n", ProofMarker, res.Authority)
	fmt.Fprintf(&b, "Head `%s` · ledger `%s` at `%s`\n\n", short(res.HeadSHA), res.Path, short(res.Commit))
	if res.Authority == proof.Advisory {
		b.WriteString("Advisory: pushed by an agent lane or a human. Only the base-branch merge gate writes authoritative packs.\n\n")
	}
	if p != nil && len(p.Checks) > 0 {
		b.WriteString("| Floor | Result | Value | Threshold | Tool |\n|---|---|---|---|---|\n")
		for _, c := range p.Checks {
			res := c.Result
			if c.Waiver != "" {
				res += " (" + c.Waiver + ")"
			}
			fmt.Fprintf(&b, "| %s | %s | %g | %g | %s %s |\n", c.Floor, res, c.Value, c.Threshold, c.Tool, c.Version)
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "`aayu proof verify %d --repo %s` checks hashes, criterion mapping, and the head commit.\n", res.Story, repo)
	return b.String()
}
