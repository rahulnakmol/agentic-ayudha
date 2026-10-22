package cli

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/grill"
	"github.com/rahulnakmol/agentic-ayudha/internal/identity"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/ready"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

func init() { register("ready", runReady) }

// issueNumber reads a TypeIssue input: 12, #12, or an issue URL.
func issueNumber(raw string) int {
	n, _ := strconv.Atoi(raw[strings.LastIndexAny(raw, "#/")+1:])
	return n
}

// technicalOwner resolves a Story's Technical Owner: the Code Architect seat's
// paired human, else its agent twin (seats are n/a until WP5.1).
func (e *Env) technicalOwner() string {
	if l, err := e.Config(); err == nil {
		if s, ok := l.Config.Seats[string(verbs.CodeArchitect)]; ok && s.Mode == "paired" && s.Human != "" {
			return "@" + s.Human
		}
	}
	return ready.TwinOwner
}

func runReady(ctx context.Context, e *Env, in Values) error {
	repo, err := e.RepoName(ctx)
	if err != nil {
		return err
	}
	n := issueNumber(in.String("issue"))
	r := &ready.Ready{Repo: repo, TechnicalOwner: e.technicalOwner()}
	if !in.Bool("guard") {
		cat, err := e.grillCatalog()
		if err != nil {
			return err
		}
		r.Concrete = func(it workitems.Item) []workitems.Problem { return grill.Check(cat, it) }
	}
	var res ready.Result
	switch {
	case in.Bool("check") && in.Bool("guard"):
		return Errorf(ExitUsage, "ready: --check and --guard are exclusive")
	case in.Bool("check"):
		c, _, err := e.HumanGitHub(ctx)
		if err != nil {
			return err
		}
		r.GH = c
		res, _, err = r.Check(ctx, n)
		if err != nil {
			return err
		}
	case in.Bool("guard"):
		// The guard runs in the aayudha-ready-guard workflow with that
		// workflow's issues-only token; it writes labels and a comment, never the ledger.
		tok, err := e.Resolver().Human(ctx)
		if err != nil {
			return Wrap(ExitBlocked, err)
		}
		if r.GH, err = e.newGitHub(tok, "ci", ""); err != nil {
			return err
		}
		if res, err = r.Guard(ctx, n); err != nil {
			return err
		}
		return e.Emit(res, func(w io.Writer) error {
			if res.Stripped != "" {
				_, err := fmt.Fprintf(w, "#%d: removed status:ready: %s\n", n, res.Stripped)
				return err
			}
			_, err := fmt.Fprintf(w, "#%d: Ready holds (or the issue is not labelled status:ready)\n", n)
			return err
		})
	default:
		c, bot, err := e.BotGitHub(ctx, "")
		if err != nil {
			return err
		}
		r.GH, r.Tag, r.Login = c, bot.Tag, bot.Login
		if r.Login == "" {
			r.Login = bot.Kind
		}
		ident := "bot"
		if bot.Kind == identity.KindHumanFallback {
			ident = identity.KindHumanFallback
		}
		w, err := e.Ledger(ctx, bot.Token, ident, bot.Tag)
		if err != nil {
			return err
		}
		r.Ledger = func(ctx context.Context, c ledger.Commit) (string, error) { return w.Write(ctx, c) }
		if res, err = r.Apply(ctx, n); err != nil {
			return err
		}
	}
	if err := e.Emit(res, func(w io.Writer) error {
		verdict := "passes Ready"
		if !res.Pass {
			verdict = "does not pass Ready"
		}
		fmt.Fprintf(w, "#%d %s (%s): %s\n", n, res.Title, res.Type, verdict)
		for _, p := range res.Problems {
			fmt.Fprintf(w, "  - %s\n", p)
		}
		for _, p := range res.Warnings {
			fmt.Fprintf(w, "  - warning: %s\n", p)
		}
		if res.Pass && !in.Bool("check") {
			if res.DryRun {
				fmt.Fprintln(w, "  --dry-run: snapshot, ledger write, and status:ready planned, not written")
			} else {
				fmt.Fprintf(w, "  labelled status:ready; snapshot posted and written to ledger %s (%s)\n", res.LedgerPath, short(res.LedgerCommit))
			}
			if res.Tag != "" {
				fmt.Fprintf(w, "  written as %s, tagged %s\n", res.Identity, res.Tag)
			}
		}
		return nil
	}); err != nil {
		return err
	}
	if !res.Pass {
		return WithNext(Errorf(ExitBlocked, "#%d does not pass Ready (%d problem(s)): fix them in the issue body", n, len(res.Problems)), fmt.Sprintf("aayu ready %d --check", n))
	}
	if !in.Bool("check") && !in.Bool("guard") && !res.DryRun {
		e.milestone("first-ready", true, "Your first Ready story!", fmt.Sprintf("#%d is Ready; the build lane can pick it up with aayu build %d.", n, n))
	}
	return nil
}

func short(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}
