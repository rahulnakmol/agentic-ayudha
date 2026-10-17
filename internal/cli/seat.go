package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/identity"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/seats"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

func init() { register("seat", runSeat) }

// paired returns the configured seat → human pairings.
func paired(c config.Config) map[verbs.SeatID]string {
	out := map[verbs.SeatID]string{}
	for id, s := range c.Seats {
		if s.Mode == "paired" && s.Human != "" {
			out[verbs.SeatID(id)] = s.Human
		}
	}
	return out
}

func runSeat(ctx context.Context, e *Env, in Values) error {
	l, err := e.Config()
	if err != nil {
		return err
	}
	action, slug := in.String("action"), in.String("seat")
	root := config.RepoRoot(e.Dir)
	if action == "list" {
		return seatList(e, root, l.Config)
	}
	if err := e.requireHuman("seat " + action); err != nil {
		return err
	}
	if _, ok := verbs.FindSeat(verbs.SeatID(slug)); !ok {
		var ids []string
		for _, s := range verbs.Seats {
			ids = append(ids, string(s.ID))
		}
		return Errorf(ExitUsage, "seat %s: want a seat, one of %s", action, strings.Join(ids, ", "))
	}
	human := strings.TrimPrefix(in.String("human"), "@")
	switch {
	case action == "add" && !seats.ValidLogin(human):
		return Errorf(ExitUsage, "seat add: --human %q: want a GitHub login", in.String("human"))
	case action == "remove" && in.Has("human"):
		return Errorf(ExitUsage, "seat remove: --human is for add")
	case root == "":
		return Errorf(ExitUsage, "seat %s writes .aayudha/config.toml, CODEOWNERS, and AGENTS.md: run it inside a checkout", action)
	}
	if action == "remove" && paired(l.Config)[verbs.SeatID(slug)] == "" {
		return Errorf(ExitUsage, "seat remove: %s is already solo", slug)
	}
	if !e.DryRun {
		q := fmt.Sprintf("Pair the %s seat with @%s (config, CODEOWNERS, an onboarding issue, and the solo-mode ruleset when this makes two humans)?", slug, human)
		if action == "remove" {
			q = fmt.Sprintf("Return the %s seat to solo (config, CODEOWNERS, its human's open stories, and the solo-mode ruleset)?", slug)
		}
		if err := e.confirmOrBlock("seat "+action, q); err != nil {
			return err
		}
	}
	if e.Assets == nil {
		return Errorf(ExitInternal, "seat: this binary has no embedded seat charters")
	}
	c, tok, err := e.HumanGitHub(ctx)
	if err != nil {
		return err
	}
	repo, err := e.RepoName(ctx)
	if err != nil {
		return err
	}
	owner, name, _ := github.SplitRepo(repo)
	info, _, err := c.REST.Repositories.Get(ctx, owner, name)
	if err != nil {
		return Wrap(ExitExternal, fmt.Errorf("repository %s: %w", repo, err))
	}
	actor := ""
	if u, err := identity.WhoAmI(ctx, e.HTTP, e.githubAPI(), tok); err == nil {
		actor = u.Login
	}
	w, err := e.Ledger(ctx, tok, "human", "")
	if err != nil {
		return err
	}
	personas, err := seats.LoadPersonas(e.Assets)
	if err != nil {
		return err
	}
	s := &seats.Seat{
		GH: c, Repo: repo, Root: root, Default: info.GetDefaultBranch(), Paired: paired(l.Config),
		BotLogins: l.Config.Identity.BotLogins, Actor: actor, Now: e.Now,
		Charter:  func(id verbs.SeatID) ([]byte, error) { return fs.ReadFile(e.Assets, "seats/"+string(id)+"/CHARTER.md") },
		Personas: personas,
		Ledger:   func(ctx context.Context, cm ledger.Commit) (string, error) { return w.Write(ctx, cm) },
	}
	var res seats.Result
	if action == "add" {
		res, err = s.Add(ctx, verbs.SeatID(slug), human)
	} else {
		res, err = s.Remove(ctx, verbs.SeatID(slug))
	}
	switch {
	case errors.Is(err, seats.ErrUsage):
		return Wrap(ExitUsage, err)
	case errors.Is(err, seats.ErrAdmin):
		return Wrap(ExitBlocked, err)
	case err != nil:
		return err
	}
	return e.Emit(res, func(w io.Writer) error {
		verb := "paired with @" + res.Human
		if res.Action == "remove" {
			verb = "returned to solo (was @" + res.Human + ")"
		}
		fmt.Fprintf(w, "%s: %s\n", res.Seat, verb)
		for _, f := range res.Files {
			fmt.Fprintf(w, "  %-9s %s\n", f.Status, f.Path)
		}
		if res.Issue != 0 {
			fmt.Fprintf(w, "  onboarding issue #%d\n", res.Issue)
		}
		for _, n := range res.Reassigned {
			fmt.Fprintf(w, "  #%d: Technical Owner handed to %s (re-run `aayu ready %d` if it was Ready)\n", n, seats.TwinOwner(verbs.SeatID(res.Seat)), n)
		}
		fmt.Fprintf(w, "  humans: @%s; policy: %s; ruleset %s: %s\n", strings.Join(res.Humans, ", @"), res.Policy, seats.GateRuleset, res.Ruleset)
		if res.PolicyNote != "" {
			fmt.Fprintf(w, "  %s\n", res.PolicyNote)
		}
		if res.LedgerCommit != "" {
			fmt.Fprintf(w, "  recorded on ledger %s (%s)\n", seats.LedgerLog, short(res.LedgerCommit))
		}
		fmt.Fprintln(w, "  Commit the changed files through a pull request: they are protected paths.")
		return nil
	})
}

func seatList(e *Env, root string, c config.Config) error {
	p := paired(c)
	rows := seats.List(p)
	humans := []string{}
	if root != "" {
		co, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(seats.CodeownersPath(root))))
		humans = seats.Humans(seats.RenderCodeowners(string(co), p, c.Identity.BotLogins), c.Identity.BotLogins)
	}
	policy := seats.PolicySolo
	if len(humans) >= 2 {
		policy = seats.PolicyTwoHuman
	}
	out := map[string]any{"seats": rows, "humans": humans, "policy": policy}
	return e.Emit(out, func(w io.Writer) error {
		for _, r := range rows {
			who := "—"
			if r.Human != "" {
				who = "@" + r.Human
			}
			layers := strings.Join(r.Layers, ",")
			if layers == "" {
				layers = "—"
			}
			fmt.Fprintf(w, "%-28s %-7s %-20s %s\n", r.Seat, r.Mode, who, layers)
		}
		fmt.Fprintf(w, "humans in CODEOWNERS and seats: %d; review policy: %s\n", len(humans), policy)
		return nil
	})
}
