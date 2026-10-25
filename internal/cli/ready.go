package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
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
	n := issueNumber(in.String("issue"))
	if in.Has("part") || in.Bool("request") {
		return runReadyPart(ctx, e, in, n)
	}
	repo, err := e.RepoName(ctx)
	if err != nil {
		return err
	}
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

// ReadyPartReport is what `aayu ready <f> --part <p>` reports.
type ReadyPartReport struct {
	Repo         string          `json:"repo"`
	Feature      int             `json:"feature"`
	Part         string          `json:"part"`
	State        string          `json:"state"` // drafted, unchanged, or baselined
	Baseline     *ready.Baseline `json:"baseline"`
	LedgerPath   string          `json:"ledger_path,omitempty"`
	LedgerCommit string          `json:"ledger_commit,omitempty"`
	DryRun       bool            `json:"dry_run,omitempty"`
}

// partRun is one run of ready --part on a Feature.
type partRun struct {
	e    *Env
	c    *github.Client
	w    *ledger.Writer
	repo string
	n    int
	part string
}

// runReadyPart drafts (--request, any lane) or approves (a person only) a
// baseline of one part of a Feature (Feature #874). An approval is refused
// for an agent lane before anything is read, and otherwise always goes
// through GitHub's device flow, even at a terminal with no agent marker.
func runReadyPart(ctx context.Context, e *Env, in Values, n int) error {
	part, request := in.String("part"), in.Bool("request")
	switch {
	case part == "":
		return Errorf(ExitUsage, "ready: --request goes with --part")
	case in.Bool("check") || in.Bool("guard"):
		return Errorf(ExitUsage, "ready: --part goes with neither --check nor --guard")
	case !request && (e.Getenv("AAYU_AGENT") == "1" || e.CI):
		return e.partRefusal()
	}
	repo, err := e.RepoName(ctx)
	if err != nil {
		return err
	}
	r := &partRun{e: e, repo: repo, n: n, part: part}
	var tok identity.Token
	ident, tag, who := "human", "", ""
	if request {
		var bot identity.Bot
		if r.c, bot, err = e.BotGitHub(ctx, ""); err != nil {
			return err
		}
		tok, tag, who, ident = bot.Token, bot.Tag, or(bot.Login, bot.Kind), "bot"
		if bot.Kind == identity.KindHumanFallback {
			ident = identity.KindHumanFallback
		}
	} else if r.c, tok, err = e.HumanGitHub(ctx); err != nil { // the person's own client; nothing is written before the device flow
		return err
	}
	if r.w, err = e.Ledger(ctx, tok, ident, tag); err != nil {
		return err
	}
	if request {
		return r.request(ctx, who)
	}
	return r.approve(ctx)
}

// partRefusal is why a person's approval cannot be asked for here.
func (e *Env) partRefusal() error {
	if sig := e.agentSignal(); sig != "" {
		return Errorf(ExitBlocked, "ready --part is a person's decision and is refused for an agent lane (%s)", sig)
	}
	return Errorf(ExitBlocked, "ready --part needs GitHub's device-flow presence check, which is not available here; run it where a person can enter the code")
}

// feature reads the Feature and what the part pins in it now.
func (r *partRun) feature(ctx context.Context) (workitems.Item, []ready.Pin, error) {
	owner, name, _ := github.SplitRepo(r.repo)
	is, _, err := r.c.REST.Issues.Get(ctx, owner, name, r.n)
	if err != nil {
		return workitems.Item{}, nil, Wrap(ExitExternal, err)
	}
	it, _ := workitems.Parse(r.n, is.GetTitle(), is.GetBody(), is.GetType().GetName(), github.IssueLabels(is))
	if it.Type != "feature" {
		return it, nil, Errorf(ExitUsage, "ready --part baselines a Feature; #%d is a %s", r.n, or(it.Type, "issue"))
	}
	pins, ok := ready.Pins(r.n, r.part, is.GetBody())
	if !ok {
		return it, nil, Errorf(ExitUsage, "ready --part %s: the %s pins are n/a until #992", r.part, ready.Label(r.part))
	}
	return it, pins, nil
}

// records reads the part's newest approved baseline and the draft that
// waits after it, each nil when there is none, and the ledger tip they are at.
func (r *partRun) records(ctx context.Context) (base, draft *ready.Baseline, tip string, err error) {
	t, err := r.w.Read(ctx)
	if errors.Is(err, ledger.ErrNoLedger) {
		return nil, nil, "", nil
	} else if err != nil {
		return nil, nil, "", Wrap(ExitExternal, err)
	}
	defer t.Close()
	paths, err := t.List(ctx, ready.BaselineDir(r.n))
	if err != nil {
		return nil, nil, "", Wrap(ExitExternal, err)
	}
	get := func(v int, isDraft bool) (*ready.Baseline, error) {
		p := ready.BaselinePath(r.n, r.part, v, isDraft)
		var rec ready.Baseline
		b, err := t.File(ctx, p)
		if err == nil {
			err = json.Unmarshal(b, &rec)
		}
		if err == nil {
			err = rec.Check(r.n, r.part, v) // never approve, or draft after, a record aayu did not write
		}
		if err != nil {
			return nil, Errorf(ExitExternal, "ledger %s: %v; nothing was written", p, err)
		}
		return &rec, nil
	}
	approved, drafted := ready.Versions(paths, r.part)
	if approved > 0 {
		base, err = get(approved, false)
	}
	if err == nil && drafted > approved {
		draft, err = get(drafted, true)
	}
	if err != nil {
		return nil, nil, "", err
	}
	return base, draft, t.Tip, nil
}

// request drafts the part's next version and says on the Feature that it
// waits for a person. It never approves anything.
func (r *partRun) request(ctx context.Context, who string) error {
	it, pins, err := r.feature(ctx)
	if err != nil {
		return err
	}
	base, draft, _, err := r.records(ctx)
	if err != nil {
		return err
	}
	hash := ready.PinsHash(pins)
	rep := ReadyPartReport{Repo: r.repo, Feature: r.n, Part: r.part, DryRun: r.c.DryRun()}
	if base != nil && base.Hash == hash {
		rep.State, rep.Baseline = "unchanged", base
		return r.e.Emit(rep, func(w io.Writer) error {
			_, err := fmt.Fprintf(w, "Nothing changed since %s v%d of #%d. No draft written.\n", ready.Label(r.part), base.Version, r.n)
			return err
		})
	}
	v := 1
	if base != nil {
		v = base.Version + 1
	}
	rep.State, rep.LedgerPath = "drafted", ready.BaselinePath(r.n, r.part, v, true)
	if draft != nil && draft.Hash == hash { // drafted already: the same draft, never a second commit
		rep.Baseline = draft
	} else {
		rep.Baseline = &ready.Baseline{Schema: ready.BaselineSchema, Feature: r.n, Part: r.part, Version: v, Owner: ready.Owner(r.part),
			Pins: pins, Hash: hash, Summary: it.Fields[workitems.FieldSummary], SourceSession: r.e.session(),
			DraftedBy: who, DraftedAt: r.e.Now().UTC().Format(time.RFC3339)}
		rep.LedgerCommit, err = r.write(ctx, fmt.Sprintf("specs: %s v%d draft of #%d", ready.Label(r.part), v, r.n), rep.LedgerPath, "", rep.Baseline)
	}
	if err == nil {
		err = r.comment(ctx, ready.Waits(*rep.Baseline))
	}
	if err != nil {
		return err
	}
	return r.emit(rep, ready.Waits(*rep.Baseline))
}

// approve makes the waiting draft the part's baseline, as the person, after
// GitHub's device flow. The draft and the pins are read again after the code,
// so only the hash the person saw is baselined.
func (r *partRun) approve(ctx context.Context) error {
	_, draft, _, err := r.records(ctx)
	if err != nil {
		return err
	}
	if draft == nil {
		return Errorf(ExitBlocked, "No %s draft of #%d waits for a person. An agent drafts it: aayu ready %d --part %s --request", ready.Label(r.part), r.n, r.n, r.part)
	}
	if err := r.holds(ctx, draft.Hash); err != nil {
		return err
	}
	effect := fmt.Sprintf("Baselines %s v%d of #%d at hash %s, as %s drafted it; a later edit shows as drift.",
		ready.Label(r.part), draft.Version, r.n, draft.Hash[:12], or(draft.DraftedBy, "an agent"))
	a, err := r.e.presenceCheck(ctx, Pending{What: "ready --part", Effect: []string{effect}}, AccessWrite, r.e.partRefusal())
	if err != nil {
		return err
	}
	// The draft or the Feature can change while the code waits: read both again.
	_, now, tip, err := r.records(ctx)
	if err != nil {
		return err
	}
	if now == nil || now.Version != draft.Version || now.Hash != draft.Hash {
		return Errorf(ExitBlocked, "%s v%d of #%d was approved or drafted again while the code waited. Nothing was baselined.", ready.Title(r.part), draft.Version, r.n)
	}
	if err := r.holds(ctx, draft.Hash); err != nil {
		return err
	}
	b := *draft // what the person saw: the hash covers only the pins
	b.ApprovedBy, b.ApprovedAt, b.Presence = a.Login, r.e.Now().UTC().Format(time.RFC3339), a.Via
	rep := ReadyPartReport{Repo: r.repo, Feature: r.n, Part: r.part, State: "baselined", Baseline: &b,
		LedgerPath: ready.BaselinePath(r.n, r.part, b.Version, false), DryRun: r.c.DryRun()}
	rep.LedgerCommit, err = r.write(ctx, fmt.Sprintf("specs: %s v%d baseline of #%d, approved by @%s", ready.Label(r.part), b.Version, r.n, a.Login), rep.LedgerPath, tip, &b)
	if err == nil {
		err = r.comment(ctx, ready.Baselined(b))
	}
	if err != nil {
		return err
	}
	return r.emit(rep, ready.Baselined(b))
}

// holds checks that the part pins hash on the Feature now.
func (r *partRun) holds(ctx context.Context, hash string) error {
	_, pins, err := r.feature(ctx)
	if err != nil {
		return err
	}
	if now := ready.PinsHash(pins); now != hash {
		return r.changed(hash, now)
	}
	return nil
}

func (r *partRun) changed(was, now string) error {
	return WithNext(Errorf(ExitBlocked, "The %s of #%d changed after the draft (hash %s → %s). Nothing was baselined. Draft it again.", ready.Label(r.part), r.n, was[:12], now[:12]),
		fmt.Sprintf("aayu ready %d --part %s --request", r.n, r.part))
}

// write puts one baseline record on the ledger in one commit, on the tip
// base when it is set, so nothing written after that read is hidden.
func (r *partRun) write(ctx context.Context, msg, path, base string, b *ready.Baseline) (string, error) {
	if r.c.DryRun() {
		return "", nil
	}
	data, _ := json.MarshalIndent(b, "", "  ")
	sha, err := r.w.Write(ctx, ledger.Commit{Message: msg, Files: map[string][]byte{path: append(data, '\n')}, Base: base})
	if errors.Is(err, ledger.ErrMoved) {
		return "", Errorf(ExitBlocked, "The ledger changed while aayu wrote %s v%d of #%d. Nothing was baselined. Run it again.", ready.Label(b.Part), b.Version, b.Feature)
	}
	return sha, Wrap(ExitExternal, err)
}

// comment keeps the part's one marker comment on the Feature.
func (r *partRun) comment(ctx context.Context, line string) error {
	marker := ready.BaselineMarker(r.part)
	_, err := r.c.UpsertComment(ctx, r.repo, r.n, marker, marker+"\n"+line+"\n", actions.Action{Kind: "github.comment.baseline",
		Payload: map[string]any{"part": r.part}})
	return Wrap(ExitExternal, err)
}

func (r *partRun) emit(rep ReadyPartReport, line string) error {
	return r.e.Emit(rep, func(w io.Writer) error {
		fmt.Fprintln(w, strings.ReplaceAll(line, "`", ""))
		switch {
		case rep.DryRun:
			_, err := fmt.Fprintf(w, "  --dry-run: ledger %s and the comment planned, not written\n", rep.LedgerPath)
			return err
		case rep.LedgerCommit != "":
			_, err := fmt.Fprintf(w, "  ledger %s (%s)\n", rep.LedgerPath, short(rep.LedgerCommit))
			return err
		}
		return nil
	})
}
