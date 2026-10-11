package cli

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strings"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/doctor"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/identity"
	"github.com/rahulnakmol/agentic-ayudha/internal/initrepo"
)

func init() {
	register("init", runInit)
	register("uninit", runUninit)
}

// RepoName returns the repository this invocation targets: --repo, else the
// owner/name of the current checkout's origin remote.
func (e *Env) RepoName(ctx context.Context) (string, error) {
	if e.Repo != "" {
		return e.Repo, nil
	}
	if r := e.originRepo(ctx); r != "" {
		return r, nil
	}
	return "", Errorf(ExitUsage, "no repository: pass --repo owner/name or run inside a checkout with a GitHub origin")
}

func (e *Env) originRepo(ctx context.Context) string {
	root := config.RepoRoot(e.Dir)
	if root == "" {
		return ""
	}
	out, err := e.exec()(ctx, root, "git", "remote", "get-url", "origin")
	if err != nil {
		return ""
	}
	if m := originRepo.FindStringSubmatch(strings.TrimSpace(out)); m != nil {
		return m[1]
	}
	return ""
}

// confirmOrBlock asks on a terminal (or takes a human's --yes) and turns a no
// into exit 4.
func (e *Env) confirmOrBlock(verb, question string) error {
	if !e.confirm(question + " [y/N] ") {
		return Errorf(ExitBlocked, "%s not confirmed: it needs a yes on a terminal, or --yes from a human", verb)
	}
	return nil
}

// initSetup gathers what init and uninit need: the checkout, the human's
// client, the repository facts, and the coexistence report.
func (e *Env) initSetup(ctx context.Context) (*initrepo.Init, error) {
	root := config.RepoRoot(e.Dir)
	if root == "" {
		return nil, Errorf(ExitUsage, "init writes the issue forms, .aayudha/, and AGENTS.md into a checkout: run it inside a clone of the repository")
	}
	repo, err := e.RepoName(ctx)
	if err != nil {
		return nil, err
	}
	if o := e.originRepo(ctx); o != "" && !strings.EqualFold(o, repo) {
		return nil, Errorf(ExitUsage, "--repo %s is not this checkout's origin (%s)", repo, o)
	}
	l, err := e.Config()
	if err != nil {
		return nil, err
	}
	c, tok, err := e.HumanGitHub(ctx)
	if err != nil {
		return nil, err
	}
	owner, name, _ := github.SplitRepo(repo)
	info, _, err := c.REST.Repositories.Get(ctx, owner, name)
	if err != nil {
		return nil, Wrap(ExitExternal, fmt.Errorf("repository %s: %w", repo, err))
	}
	u, err := identity.WhoAmI(ctx, e.HTTP, e.githubAPI(), tok)
	if err != nil {
		return nil, Wrap(ExitExternal, err)
	}
	p := e.probe(ctx)
	p.Repo = repo
	co := doctor.Inspect(ctx, p)
	bs, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.Branch, *gh.Response, error) {
		return c.REST.Repositories.ListBranches(ctx, owner, name, &gh.BranchListOptions{ListOptions: o})
	})
	if err != nil {
		return nil, Wrap(ExitExternal, err)
	}
	var branches []string
	for _, b := range bs {
		branches = append(branches, b.GetName())
	}
	co.HasDev = slices.Contains(branches, "dev")
	co.Default = info.GetDefaultBranch()
	st, err := initrepo.LoadState(root)
	if err != nil {
		return nil, Wrap(ExitUsage, err)
	}
	if st == nil {
		st = initrepo.NewState(repo)
	}
	return &initrepo.Init{
		GH: c, Root: root, State: st,
		Repo: initrepo.Repo{
			FullName: repo, Owner: owner, Name: name, Org: info.GetOwner().GetType() == "Organization",
			Private: info.GetPrivate(), Default: info.GetDefaultBranch(), Branches: branches, Coexist: co,
			HumanLogin: u.Login, BotKind: identity.Configured(l.Config.Identity),
		},
	}, nil
}

func runInit(ctx context.Context, e *Env, in Values) error {
	it, err := e.initSetup(ctx)
	if err != nil {
		return err
	}
	resume := in.Bool("resume")
	if !it.State.Complete && len(it.State.Done) > 0 && !resume && !e.DryRun {
		return Wrap(ExitUsage, fmt.Errorf("%w after step %s: run `aayu init --resume`", initrepo.ErrIncomplete, it.State.Done[len(it.State.Done)-1]))
	}
	if !e.DryRun {
		vis := "public"
		if it.Repo.Private {
			vis = "private"
		}
		if err := e.confirmOrBlock("init", fmt.Sprintf("init uses your admin access on %s (%s) to create labels, %s, the ledger branch, and its ruleset, and writes forms, .aayudha/, and AGENTS.md here. Continue?",
			it.Repo.FullName, vis, map[bool]string{true: "issue types", false: "no issue types (personal repository)"}[it.Repo.Org])); err != nil {
			return err
		}
	}
	runErr := it.Run(ctx, resume)
	if runErr == nil {
		x := initrepo.Extras{
			Projects: in.Bool("projects"), Gates: in.Bool("gates"), Identity: in.Bool("identity"), Integrations: in.Bool("integrations"),
			Version: e.Build.Version,
			Confirm: func(summary string) bool {
				fmt.Fprintln(e.Err, summary)
				return e.confirm("Continue? [y/N] ")
			},
			Ask:          e.askLine,
			ManifestCode: e.manifestCode,
		}
		if e.AppManifestCode != nil {
			x.ManifestCode = e.AppManifestCode
		}
		runErr = it.RunExtras(ctx, x)
	}
	if err := e.reportInit("init", it, runErr == nil); err != nil {
		return err
	}
	return runErr
}

func (e *Env) reportInit(verb string, it *initrepo.Init, ok bool) error {
	counts := map[string]int{}
	for _, c := range it.Changes {
		counts[c.Result]++
	}
	rep := map[string]any{
		"repo": it.Repo.FullName, "dry_run": e.DryRun, "complete": ok && it.State.Complete, "changes": it.Changes,
		"counts": counts, "github": it.GH.Stats(), "state": initrepo.StatePath,
	}
	return e.Emit(rep, func(w io.Writer) error {
		head := fmt.Sprintf("%s %s", verb, it.Repo.FullName)
		if e.DryRun {
			head += " (--dry-run: nothing was written)"
		}
		fmt.Fprintln(w, head)
		for _, c := range it.Changes {
			fmt.Fprintf(w, "  %-12s %s\n", c.Step, c)
		}
		s := it.GH.Stats()
		fmt.Fprintf(w, "GitHub calls: %d of %d (%d not modified)\n", s.Calls(), s.Budget, s.NotChanged)
		if verb == "init" && ok && !e.DryRun && counts["written"] > 0 {
			fmt.Fprintln(w, "Commit the written files (.github/ISSUE_TEMPLATE/aayudha-*.yml, .aayudha/, AGENTS.md) through a pull request.")
		}
		return nil
	})
}

func runUninit(ctx context.Context, e *Env, in Values) error {
	it, err := e.initSetup(ctx)
	if err != nil {
		return err
	}
	if len(it.State.Objects) == 0 && len(it.State.Files) == 0 {
		return e.Emit(map[string]any{"repo": it.Repo.FullName, "changes": []any{}}, func(w io.Writer) error {
			_, err := fmt.Fprintf(w, "nothing to remove: no %s\n", initrepo.StatePath)
			return err
		})
	}
	if in.Bool("external") {
		ext := it.State.External()
		return e.Emit(map[string]any{"repo": it.Repo.FullName, "external": ext}, func(w io.Writer) error {
			if len(ext) == 0 {
				_, err := fmt.Fprintln(w, "no external resources recorded (init --projects, --identity, and --integrations create them)")
				return err
			}
			fmt.Fprintln(w, "external resources init created; uninit never deletes them, remove each by hand:")
			for _, o := range ext {
				fmt.Fprintf(w, "  %-12s %s (%s)\n", o.Kind, o.Name, o.ID)
			}
			return nil
		})
	}
	if !e.DryRun {
		if err := e.confirmOrBlock("uninit", "uninit deletes the labels, issue types, rulesets, ledger branch, and files init created on "+it.Repo.FullName+". Continue?"); err != nil {
			return err
		}
	}
	runErr := it.Uninit(ctx)
	if err := e.reportInit("uninit", it, runErr == nil); err != nil {
		return err
	}
	return runErr
}
