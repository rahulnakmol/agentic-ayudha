package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	osuser "os/user"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/autonomy"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/identity"
	"github.com/rahulnakmol/agentic-ayudha/internal/initrepo"
	"github.com/rahulnakmol/agentic-ayudha/internal/inspect"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations/cloud"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
	"github.com/rahulnakmol/agentic-ayudha/internal/ready"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

func init() { register("proof", runProof) }

// ProofMarker marks the merge gate's replace-in-place proof comment on a PR
// (S-4.2b); AdvisoryProofMarker marks an advisory push's comment, so an agent
// preview never replaces the gate's.
const (
	ProofMarker         = "<!-- aayudha:proof -->"
	AdvisoryProofMarker = "<!-- aayudha:proof-advisory -->"
)

// MergeGateWorkflow is the workflow whose CI-written packs are authoritative (WP4.8).
const MergeGateWorkflow = initrepo.MergeGateWorkflow

// DefaultMediaSecret names the secret holding the media store's credential
// (proof.OpenMedia lists what each store takes).
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
// the [proof] media bucket with credentials from the secrets store. The
// bucket, the name of its secret, and the retention its lifecycle rule
// sets come only from trustedProof (#667).
func (e *Env) proofStore(ctx context.Context, c *github.Client, tok identity.Token, ident, tag string) (*proof.Store, error) {
	l, err := e.Config()
	if err != nil {
		return nil, err
	}
	repo, err := e.RepoName(ctx)
	if err != nil {
		return nil, err
	}
	pc, err := e.trustedProof(ctx, c, repo, l)
	if err != nil {
		return nil, err
	}
	w, err := e.Ledger(ctx, tok, ident, tag)
	if err != nil {
		return nil, err
	}
	s := &proof.Store{Repo: repo, Ledger: w, Private: e.private(ctx, c, repo), RetentionDays: pc.RetentionDays}
	if m := pc.Media; m != "" {
		name := pc.MediaSecret
		if name == "" {
			name = DefaultMediaSecret
		}
		s.Bucket, err = proof.OpenMedia(m, proof.MediaOptions{
			Secret:     func() string { v, _, _ := e.SecretStore().Get(name); return v },
			SecretName: name, HTTP: e.HTTP, Now: e.Now, Env: cloud.Env{Getenv: e.Getenv, Now: e.Now},
		})
		if errors.Is(err, proof.ErrMediaSecret) {
			return nil, Wrap(ExitBlocked, err)
		}
		if err != nil {
			return nil, Wrap(ExitUsage, err)
		}
	}
	return s, nil
}

// accountConfigDir is the account's user config directory
// (~/.config/aayudha), found from the system's user database: XDG_CONFIG_HOME
// and HOME are the run's environment, which could name a directory in the
// checkout. It is "" with why when no user config is trusted.
func (e *Env) accountConfigDir() (dir, skipped string) {
	home := e.UserHome
	if home == nil {
		home = accountHome
	}
	if h, err := home(); err != nil {
		skipped = fmt.Sprintf(" (no user config is trusted: %v)", err)
	} else if h != "" {
		dir = filepath.Join(h, ".config", "aayudha")
	}
	return dir, skipped
}

// trustedProof is the [proof] media bucket, the name of its secret, and the
// retention its lifecycle rule sets, from sources the person controls
// (#667): the defaults, the account's user config (~/.config/aayudha, from
// the system's user database), and the integration branch's
// committed .aayudha/config.toml at its tip (only the defaults and the user
// config when that file cannot be found). The integration branch is the
// user config's [branches] dev, else the one the default branch's committed
// config maps (aayu init writes [branches] dev there when a repository has
// no dev), else the default branch.
// The checkout, config.local, AAYU_* variables and --set may belong to an
// agent whose branch the person has checked out, and naming a bucket there
// would send the person's local secret to it, so a run whose values differ
// is refused before any secret is read.
func (e *Env) trustedProof(ctx context.Context, c *github.Client, repo string, run *config.Loaded) (config.Proof, error) {
	userDir, skipped := e.accountConfigDir()
	user, err := config.Load(config.Sources{UserDir: userDir})
	if err != nil {
		return config.Proof{}, Wrap(ExitUsage, fmt.Errorf("config: %w", err))
	}
	owner, name, _ := github.SplitRepo(repo)
	// load is the user config over branch's committed .aayudha/config.toml at
	// its tip; found is false when the branch does not exist. A file there
	// that cannot be read (a directory, or one over the API's 1 MB) or does
	// not load is no config the run may be judged against.
	load := func(branch string) (l *config.Loaded, found bool, err error) {
		ref, _, err := c.REST.Git.GetRef(ctx, owner, name, "heads/"+branch)
		if github.NotFound(err) {
			return &config.Loaded{Config: user.Config}, false, nil
		}
		if err != nil {
			return nil, false, Wrap(ExitExternal, err)
		}
		var text string
		fc, _, _, err := c.REST.Repositories.GetContents(ctx, owner, name, ".aayudha/config.toml", &gh.RepositoryContentGetOptions{Ref: ref.GetObject().GetSHA()})
		switch {
		case github.NotFound(err):
			err = nil
		case err == nil && fc == nil:
			err = errors.New("it is a directory")
		case err == nil:
			text, err = fc.GetContent()
		}
		if err == nil {
			l, err = config.Load(config.Sources{UserDir: userDir, RepoTOML: []byte(text), RepoSource: ".aayudha/config.toml on " + branch})
		}
		if err != nil {
			return nil, true, Errorf(ExitBlocked, "%s's .aayudha/config.toml cannot be read or does not load, so the proof media bucket cannot be trusted: %v", branch, err)
		}
		return l, true, nil
	}
	// A branch is trusted only by name from the user config or the default
	// branch's committed config, never because it is called dev: an agent
	// that can push a branch could create one where none exists.
	branch := user.Config.Branches.Dev
	if branch == "" {
		r, _, err := c.REST.Repositories.Get(ctx, owner, name)
		if err != nil {
			return config.Proof{}, Wrap(ExitExternal, err)
		}
		branch = r.GetDefaultBranch()
	}
	trusted, _, err := load(branch)
	if err == nil && user.Config.Branches.Dev == "" && trusted.Config.Branches.Dev != "" && trusted.Config.Branches.Dev != branch {
		branch = trusted.Config.Branches.Dev
		trusted, _, err = load(branch)
	}
	if err != nil {
		return config.Proof{}, err
	}
	t, r := trusted.Config.Proof, run.Config.Proof
	// Without --repo the repository is the checkout's origin remote, which
	// an agent can point at a repository it controls, with its bucket
	// committed on dev. A bucket is trusted only from a repository the
	// account aayu acts as can push to, as the person's own repositories are.
	if t.Media != "" {
		why, err := cannotPush(ctx, c, owner, name)
		if err != nil {
			return config.Proof{}, err
		}
		if why != "" {
			return config.Proof{}, Errorf(ExitBlocked, "%s, so its committed config cannot name the proof media bucket that receives your secret; "+
				"check the checkout's origin remote, or pass --repo with the repository you push to", why)
		}
	}
	if r.Media != t.Media || t.Media != "" && (r.MediaSecret != t.MediaSecret || r.RetentionDays != t.RetentionDays) {
		return config.Proof{}, Errorf(ExitBlocked, "the proof media bucket, its secret, and its retention are read only from the user config and %s's committed .aayudha/config.toml%s; "+
			"this run's config (the checkout, config.local, AAYU_* variables, or --set) sets [proof] media %q, media_secret %q and retention_days %d, not %q, %q and %d; "+
			"remove those settings, or change %s through a reviewed PR", branch, skipped, r.Media, r.MediaSecret, r.RetentionDays, t.Media, t.MediaSecret, t.RetentionDays, branch)
	}
	return t, nil
}

// cannotPush says why the token behind c cannot push to owner/name ("" when
// it can). A person's token needs write or admin access for its account. A
// GitHub App installation token has no account (GET /user refuses it with
// 403), so the repository must be one its installation was granted.
func cannotPush(ctx context.Context, c *github.Client, owner, name string) (string, error) {
	me, resp, err := c.REST.Users.Get(ctx, "")
	if err == nil {
		perm, _, err := c.REST.Repositories.GetPermissionLevel(ctx, owner, name, me.GetLogin())
		if err != nil {
			return "", Wrap(ExitExternal, err)
		}
		if p := permissionOf(perm); p != "admin" && p != "write" {
			return fmt.Sprintf("@%s has %s access to %s/%s", me.GetLogin(), p, owner, name), nil
		}
		return "", nil
	}
	if resp == nil || resp.StatusCode != http.StatusForbidden {
		return "", Wrap(ExitExternal, err)
	}
	repos, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.Repository, *gh.Response, error) {
		l, resp, err := c.REST.Apps.ListRepos(ctx, &o)
		if err != nil {
			return nil, resp, err
		}
		return l.Repositories, resp, nil
	})
	if err != nil {
		return "", Wrap(ExitExternal, err)
	}
	if !slices.ContainsFunc(repos, func(r *gh.Repository) bool { return strings.EqualFold(r.GetFullName(), owner+"/"+name) }) {
		return fmt.Sprintf("the GitHub App installation behind this token was not granted %s/%s", owner, name), nil
	}
	return "", nil
}

// accountHome is the home directory of the account running aayu, from the
// system's user database (os/user), which the run's HOME does not change.
// Outside Windows and macOS a build without cgo (every release binary)
// reads /etc/passwd, and when the UID has no entry there os/user falls back
// to $HOME and $USER, which the run sets. So the home must be the one
// /etc/passwd gives the UID, or no user config is trusted.
func accountHome() (string, error) {
	u, err := osuser.Current()
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return u.HomeDir, nil
	}
	b, err := os.ReadFile(passwdFile)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(b), "\n") {
		if f := strings.Split(line, ":"); len(f) >= 7 && f[2] == u.Uid && f[5] == u.HomeDir {
			return u.HomeDir, nil
		}
	}
	return "", fmt.Errorf("%s gives UID %s no home %q, so it came from the environment", passwdFile, u.Uid, u.HomeDir)
}

// passwdFile is the user database that confirms accountHome (a variable for tests).
var passwdFile = "/etc/passwd"

// inMergeGate reports whether this process is the base-branch merge gate in
// CI (WP4.8): a pull_request_target run (the base branch's copy of the file,
// as ship requires, D-4.2b.a) whose workflow ref names aayudha-merge-gate.yml.
func (e *Env) inMergeGate() bool {
	return e.CI && e.Getenv("GITHUB_EVENT_NAME") == "pull_request_target" &&
		strings.Contains(e.Getenv("GITHUB_WORKFLOW_REF"), "/.github/workflows/"+MergeGateWorkflow+"@")
}

func runProof(ctx context.Context, e *Env, in Values) error {
	action := in.String("action")
	story := 0
	if in.Has("story") {
		story = issueNumber(in.String("story"))
	}
	if action == "push" && in.Bool("media") {
		if l, err := e.Config(); err != nil {
			return err
		} else if l.Config.Proof.Media == "" {
			return Errorf(ExitBlocked, "proof push --media: %v", proof.ErrMediaRefused)
		}
	}
	if story == 0 && (action == "push" || action == "get" || action == "verify" || action == "comment" || action == "land") {
		return Errorf(ExitUsage, "proof %s needs a story", action)
	}
	repo, err := e.RepoName(ctx)
	if err != nil {
		return err
	}
	switch action {
	case "push":
		return e.proofPush(ctx, repo, story, in.Bool("media"))
	case "comment":
		return e.proofComment(ctx, repo, story)
	case "land":
		return e.proofLand(ctx, repo, story)
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

// proofPush lands the local pack (.aayudha/proof/<story>/) on the ledger as
// an advisory pack and keeps an advisory comment on the PR and the issue. A
// push is never authoritative: only `aayu ship` lands the merge gate's pack,
// after checking the run that built it (S-4.2b).
func (e *Env) proofPush(ctx context.Context, repo string, story int, media bool) error {
	root := config.RepoRoot(e.Dir)
	if root == "" {
		return Errorf(ExitUsage, "proof push needs a checkout holding the pack (.aayudha/proof/%d/)", story)
	}
	dir := proof.LocalDir(root, story)
	if _, err := os.Stat(dir); err != nil {
		return Errorf(ExitBlocked, "no local pack for #%d in %s: run `aayu verify` first", story, dir)
	}
	if e.CI {
		return Errorf(ExitUsage, "proof push --ci: CI writes nothing to the ledger; the merge gate keeps its pack as the aayudha-pack run artifact and `aayu ship` lands it")
	}
	c, bot, err := e.BotGitHub(ctx, "")
	if err != nil {
		return err
	}
	s, err := e.proofStore(ctx, c, bot.Token, botIdent(bot), bot.Tag)
	if err != nil {
		return err
	}
	res, err := s.Push(ctx, dir, media, proof.Authority{Kind: proof.Advisory})
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
		if _, err := c.UpsertComment(ctx, repo, n, AdvisoryProofMarker, body, actions.Action{Kind: "github.comment.proof", Payload: map[string]any{"pack": res.Path, "authority": res.Authority}}); err != nil {
			return err
		}
	}
	return e.Emit(res, func(w io.Writer) error {
		fmt.Fprintf(w, "#%d: %s pack for %s pushed to ledger %s (%s)\n", story, res.Authority, short(res.HeadSHA), res.Path, short(res.Commit))
		for _, st := range res.Stores {
			fmt.Fprintf(w, "  %s\n", st)
		}
		if res.LifecycleNote != "" {
			fmt.Fprintf(w, "  retention: %s\n", res.LifecycleNote)
		}
		return nil
	})
}

// proofLand lands the merge gate's own pack of a PR merged outside ship
// (Bug #1031): the same artifact checks ship makes (gatePack), on the head
// the PR merged at, then the pack verified against the Ready snapshot with its
// floors holding, landed authoritative in one ledger commit. It never reads a
// local pack.
func (e *Env) proofLand(ctx context.Context, repo string, n int) error {
	c, tok, err := e.HumanGitHub(ctx)
	if err != nil {
		return err
	}
	me, err := identity.WhoAmI(ctx, e.HTTP, e.githubAPI(), tok)
	if err != nil {
		return Wrap(ExitBlocked, err)
	}
	owner, name, _ := github.SplitRepo(repo)
	pr, _, err := c.REST.PullRequests.Get(ctx, owner, name, n)
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	if !pr.GetMerged() {
		return Errorf(ExitBlocked, "proof land #%d: the PR is %s, not merged; `aayu ship %d` lands the pack of an open PR", n, pr.GetState(), n)
	}
	story := StoryOf(pr.GetBody(), "", pr.GetHead().GetRef())
	if story == 0 {
		return Errorf(ExitBlocked, "proof land #%d: no story (Refs #n in the PR body)", n)
	}
	if b := StoryOf("", "", pr.GetHead().GetRef()); b != 0 && b != story {
		return Errorf(ExitBlocked, "proof land #%d: the PR body refers to story #%d, but its branch %s is story #%d's", n, story, pr.GetHead().GetRef(), b)
	}
	gp, err := e.gatePack(ctx, c, owner, name, "proof land", pr, story)
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(gp.tmp) }()
	s, err := e.proofStore(ctx, c, tok, "human", "")
	if err != nil {
		return err
	}
	if err := verifyGatePack(ctx, s, "proof land", n, story, pr.GetHead().GetSHA(), gp, nil); err != nil {
		return err
	}
	res, err := s.Push(ctx, gp.dir, s.Bucket != nil, proof.Authority{Kind: proof.Authoritative,
		Workflow: ".github/workflows/" + MergeGateWorkflow, RunID: gp.run.GetID(), Writer: me.Login + " (aayu proof land)"})
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	return e.Emit(res, func(w io.Writer) error {
		_, err := fmt.Fprintf(w, "proof land #%d: the %s pack of #%d at %s, from artifact %d of run %d, landed at ledger %s (%s)\n",
			n, res.Authority, story, short(res.HeadSHA), gp.artifact, gp.run.GetID(), res.Path, short(res.Commit))
		return err
	})
}

// ProofComment renders the replace-in-place advisory proof comment.
func ProofComment(repo string, res proof.PushResult, p *proof.Pack) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n### Proof pack (%s)\n\n", AdvisoryProofMarker, res.Authority)
	fmt.Fprintf(&b, "Head `%s` · ledger `%s` at `%s`\n\n", short(res.HeadSHA), res.Path, short(res.Commit))
	b.WriteString("Advisory: pushed by an agent lane or a human. It never satisfies `aayu ship`, which lands only the merge gate's pack.\n\n")
	if p != nil && len(p.Checks) > 0 {
		floorTable(&b, p.Checks)
	}
	fmt.Fprintf(&b, "`aayu proof verify %d --repo %s` checks hashes, criterion mapping, and the head commit.\n", res.Story, repo)
	return b.String()
}

func floorTable(b *strings.Builder, checks []proof.Check) {
	b.WriteString("| Floor | Result | Value | Threshold | Tool |\n|---|---|---|---|---|\n")
	for _, c := range checks {
		res := c.Result
		if c.Waiver != "" {
			res += " (" + c.Waiver + ")"
		}
		fmt.Fprintf(b, "| %s | %s | %g | %g | %s %s |\n", c.Floor, res, c.Value, c.Threshold, c.Tool, c.Version)
	}
	b.WriteString("\n")
}

// GateComment is what the merge gate's aayudha/pack job reports and comments.
type GateComment struct {
	Story      int          `json:"story"`
	PR         int          `json:"pr"`
	HeadSHA    string       `json:"pr_head_sha"`
	PackSHA256 string       `json:"pack_sha256"`
	Artifact   string       `json:"artifact_url,omitempty"`
	Run        string       `json:"run_url,omitempty"`
	Expires    string       `json:"artifact_expires,omitempty"`
	Report     proof.Report `json:"report"`
	Comment    bool         `json:"comment_changed"`
	// Risk and Weakened are read from the PR's diff (P6, founder decision 3):
	// they point the human pass and approve nothing.
	Risk     inspect.Risk        `json:"risk"`
	Weakened []inspect.Weakening `json:"weakened,omitempty"`
}

// proofComment is the merge gate's aayudha/pack job (S-4.2b). It checks the
// pack this run's gate job built, downloaded from the aayudha-pack run
// artifact into .aayudha/proof/<story>/, against the PR's current head and the
// story's Ready snapshot, and keeps one comment on the PR with each
// criterion's verdict, the floors, the pack's SHA-256, and a link to the
// artifact. It uses only the workflow token and writes nothing to the ledger.
// It exits 4 when the pack does not verify and 3 when a floor does not hold,
// so the check fails with it.
func (e *Env) proofComment(ctx context.Context, repo string, story int) error {
	if !e.inMergeGate() {
		return Errorf(ExitUsage, "proof comment runs only in the merge gate (a pull_request_target run of %s, --ci); an agent previews its work with `aayu verify`, which stays advisory", MergeGateWorkflow)
	}
	root := config.RepoRoot(e.Dir)
	if root == "" {
		return Errorf(ExitUsage, "proof comment needs a checkout holding the pack (.aayudha/proof/%d/)", story)
	}
	dir := proof.LocalDir(root, story)
	raw, err := os.ReadFile(filepath.Join(dir, proof.FileName))
	if err != nil {
		return Errorf(ExitBlocked, "no pack for #%d in %s: the gate job kept none in its aayudha-pack artifact", story, dir)
	}
	p, err := proof.Decode(raw)
	if err != nil {
		return Errorf(ExitBlocked, "#%d: the gate's pack.json: %v", story, err)
	}
	if p.Story != story || p.PR == 0 {
		return Errorf(ExitBlocked, "#%d: the gate's pack is for #%d, PR %d", story, p.Story, p.PR)
	}
	c, tok, err := e.readClient(ctx)
	if err != nil {
		return err
	}
	owner, name, _ := github.SplitRepo(repo)
	pr, _, err := c.REST.PullRequests.Get(ctx, owner, name, p.PR)
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	exp := proof.Expect{HeadSHA: pr.GetHead().GetSHA()}
	var crit []workitems.Criterion
	w, err := e.Ledger(ctx, tok, "ci", "")
	if err != nil {
		return err
	}
	t, err := w.Read(ctx)
	if err != nil {
		return ledgerErr(err)
	}
	b, err := t.File(ctx, ready.LedgerPath(story))
	// A gate fix PR of a Feature rests on the approved record its entry names (Bug #1100).
	fix, fixCurrent := listedGateFix(ctx, t, story, p.PR)
	t.Close()
	if err == nil {
		snap, err := ready.ParseSnapshot(string(b))
		if err != nil {
			return err
		}
		exp.CompletionGoalHash, exp.Criteria, crit = snap.CompletionGoalHash, snap.Numbers(), snap.Criteria
	}
	stale := "" // the Feature's approved record has changed since the entry was made
	if fix != nil && exp.CompletionGoalHash == "" {
		if exp.CompletionGoalHash, exp.Criteria, crit = fix.Approval.SHA256, nil, nil; !fixCurrent {
			stale = staleGateFix(fix, story)
		}
	}
	sum := sha256.Sum256(raw)
	g := GateComment{Story: story, PR: p.PR, HeadSHA: p.PRHeadSHA, PackSHA256: hex.EncodeToString(sum[:]),
		Artifact: e.Getenv("AAYU_PACK_ARTIFACT_URL")}
	if id := e.Getenv("GITHUB_RUN_ID"); id != "" {
		g.Run = fmt.Sprintf("%s/%s/actions/runs/%s", strings.TrimRight(or(e.Getenv("GITHUB_SERVER_URL"), "https://github.com"), "/"), repo, id)
	}
	if d, err := strconv.Atoi(e.Getenv("AAYU_PACK_RETENTION_DAYS")); err == nil && d > 0 {
		g.Expires = e.Now().UTC().AddDate(0, 0, d).Format(time.DateOnly)
	}
	if g.Risk, g.Weakened, err = e.prRisk(ctx, c, owner, name, p.PR, root); err != nil {
		return Wrap(ExitExternal, err)
	}
	g.Report = proof.VerifyLocal(dir, p, exp)
	if stale != "" {
		g.Report.Problems = append(g.Report.Problems, stale)
		g.Report.OK = false
	}
	if exp.CompletionGoalHash == "" {
		g.Report.Problems = append(g.Report.Problems, fmt.Sprintf("no Ready snapshot at ledger %s", ready.LedgerPath(story)))
		g.Report.OK = false
	}
	body := gateCommentBody(g, p, crit)
	if g.Comment, err = c.UpsertComment(ctx, repo, p.PR, ProofMarker, body, actions.Action{Kind: "github.comment.proof",
		Payload: map[string]any{"story": story, "pack_sha256": g.PackSHA256, "ok": g.Report.OK, "floors_hold": g.Report.FloorsHold}}); err != nil {
		return Wrap(ExitExternal, err)
	}
	if err := e.Emit(g, func(w io.Writer) error {
		fmt.Fprintf(w, "#%d pack at %s (sha256 %s): %d artifact(s) hashed; floors hold: %t\n", story, short(p.PRHeadSHA), short(g.PackSHA256), g.Report.Hashed, g.Report.FloorsHold)
		for _, pr := range g.Report.Problems {
			fmt.Fprintf(w, "  - %s\n", pr)
		}
		return nil
	}); err != nil {
		return err
	}
	if !g.Report.OK {
		return Errorf(ExitBlocked, "#%d: the gate's pack does not verify: %s", story, strings.Join(g.Report.Problems, "; "))
	}
	if !g.Report.FloorsHold {
		return Errorf(ExitFloor, "floor miss: %s", failing(p.Checks))
	}
	return nil
}

// prRisk reads PR n's diff and computes its risk line and weakened checks,
// with the layers, never-layers, and CODEOWNERS of the base checkout at root.
func (e *Env) prRisk(ctx context.Context, c *github.Client, owner, name string, n int, root string) (inspect.Risk, []inspect.Weakening, error) {
	diff, _, err := c.REST.PullRequests.GetRaw(ctx, owner, name, n, gh.RawOptions{Type: gh.Diff})
	if err != nil {
		return inspect.Risk{}, nil, err
	}
	var never []string
	if l, err := e.Config(); err == nil {
		never = l.Config.Autonomy.Never
	}
	var rules []identity.CodeownersRule
	for _, f := range []string{".github/CODEOWNERS", "CODEOWNERS", "docs/CODEOWNERS"} {
		if b, err := os.ReadFile(filepath.Join(root, f)); err == nil {
			rules = identity.ParseCodeowners(string(b))
			break
		}
	}
	protected := func(p string) bool {
		_, pattern, ok := identity.Owners(rules, p)
		return ok && strings.Trim(pattern, "/*") != "" // a catch-all owner protects nothing in particular
	}
	files := inspect.ParseDiff(diff)
	return inspect.RiskOf(files, autonomy.ReadLayers(root).Footprint, never, protected), inspect.Weakened(files), nil
}

// gateCommentBody renders the merge gate's proof comment: each Ready
// criterion's verdict, the floors, the pack's hash, and where the pack is.
func gateCommentBody(g GateComment, p *proof.Pack, crit []workitems.Criterion) string {
	var b strings.Builder
	verdict := "proven"
	switch {
	case !g.Report.OK:
		verdict = "does not verify"
	case !g.Report.FloorsHold:
		verdict = "floors do not hold"
	}
	fmt.Fprintf(&b, "%s\n### Proof pack: %s\n\n", ProofMarker, verdict)
	fmt.Fprintf(&b, "Story #%d at head `%s`, built by this pull request's merge gate run from what it ran.\n\n", g.Story, short(g.HeadSHA))
	fmt.Fprintf(&b, "**Risk: %s**", g.Risk.Tier)
	if len(g.Risk.Reasons) > 0 {
		fmt.Fprintf(&b, " (%s)", strings.Join(g.Risk.Reasons, "; "))
	}
	if len(g.Risk.Look) > 0 {
		fmt.Fprintf(&b, ". Look first at `%s`", strings.Join(g.Risk.Look, "`, `"))
	}
	b.WriteString(". The tier is computed from the diff; it approves nothing.\n\n")
	if len(g.Weakened) > 0 {
		b.WriteString("**Checks this PR weakens** (a PR may edit its own checks; a human decides whether this is right):\n\n")
		for _, w := range g.Weakened {
			fmt.Fprintf(&b, "- `%s`: %s\n", w.File, w.What)
		}
		b.WriteString("\n")
	}
	evidence := map[int][]string{}
	for _, c := range p.Criteria {
		evidence[c.ID] = c.Evidence
	}
	stored := map[string]bool{}
	for _, a := range p.Artifacts {
		stored[a.Path] = true
	}
	ids := []int{}
	text := map[int]string{}
	for _, c := range crit {
		ids, text[c.N] = append(ids, c.N), c.State
	}
	if len(ids) == 0 {
		for _, c := range p.Criteria {
			ids = append(ids, c.ID)
		}
	}
	b.WriteString("| Criterion | Verdict | Evidence |\n|---|---|---|\n")
	for _, n := range ids {
		v, refs := "no evidence", []string{}
		for _, ref := range evidence[n] {
			refs = append(refs, "`"+ref+"`")
		}
		if len(refs) > 0 {
			v = "evidence from this run"
			for _, ref := range evidence[n] {
				if !stored[ref] {
					v = "evidence missing from the pack"
				}
			}
		}
		label := fmt.Sprint(n)
		if t := strings.TrimSpace(text[n]); t != "" {
			label += ". " + strings.ReplaceAll(t, "|", "\\|")
		}
		fmt.Fprintf(&b, "| %s | %s | %s |\n", label, v, or(strings.Join(refs, ", "), "—"))
	}
	b.WriteString("\n")
	if len(p.Checks) > 0 {
		floorTable(&b, p.Checks)
	}
	if len(g.Report.Problems) > 0 {
		b.WriteString("Problems:\n\n")
		for _, pr := range g.Report.Problems {
			fmt.Fprintf(&b, "- %s\n", pr)
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "`pack.json` SHA-256: `%s`", g.PackSHA256)
	if g.Artifact != "" {
		fmt.Fprintf(&b, " · [aayudha-pack artifact](%s)", g.Artifact)
	}
	if g.Expires != "" {
		fmt.Fprintf(&b, " (kept until %s)", g.Expires)
	}
	if g.Run != "" {
		fmt.Fprintf(&b, " · [merge gate run](%s)", g.Run)
	}
	b.WriteString("\n\n")
	b.WriteString("`aayu ship` lands this pack on the ledger (pack.json and data, no media) after checking that it came from this workflow's run on this head and that every hash matches. An agent's own preview never counts.\n")
	return b.String()
}
