package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"time"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/chart"
	"github.com/rahulnakmol/agentic-ayudha/internal/design"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/grill"
	"github.com/rahulnakmol/agentic-ayudha/internal/identity"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/planning"
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
	for _, f := range []string{"milestones", "agents"} {
		if in.Has(f) && (in.Has("early-build") || in.Has("part") || in.Bool("request") || in.Bool("check") || in.Bool("guard")) {
			return Errorf(ExitUsage, "ready: --%s goes with Approve to build only, not --early-build, --part, --request, --check, or --guard", f)
		}
	}
	if in.Has("agents") && in.Int("agents") < 1 {
		return Errorf(ExitUsage, "ready --agents %d: want 1 or more stories at once", in.Int("agents"))
	}
	if in.Has("early-build") {
		return runReadyEarly(ctx, e, in, n)
	}
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
		c, tok, err := e.HumanGitHub(ctx)
		if err != nil {
			return err
		}
		r.GH = c
		var is *gh.Issue
		if res, is, err = r.Check(ctx, n); err != nil {
			return err
		}
		if res.Type == "feature" { // the Ready verdict stands; baselines that cannot be read are said, never hidden
			if res.Baselines, err = e.baselineStates(ctx, c, tok, repo, is); err != nil {
				res.Baselines, res.BaselinesNote = nil, err.Error()
			}
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
		// Read the type before the ledger setup, so an agent lane on a Feature
		// always gets the refusal, whatever else it lacks. The bot reads, as
		// it writes a story's Ready; the person's client only when there is no bot.
		c, _, err := e.BotGitHub(ctx, "")
		if err != nil {
			c, _, err = e.HumanGitHub(ctx)
		}
		if err != nil {
			return err
		}
		owner, name, _ := github.SplitRepo(repo)
		is, _, err := c.REST.Issues.Get(ctx, owner, name, n)
		if err != nil {
			return Wrap(ExitExternal, err)
		}
		if typ, _ := workitems.TypeOf(is.GetType().GetName(), github.IssueLabels(is)); typ == "feature" { // Approve to build: a person's decision
			if err := e.approveRefusal(); err != nil {
				return err
			}
			return e.approveToBuild(ctx, r, n, in.String("milestones"), in.Int("agents"))
		}
		if in.Has("milestones") {
			return Errorf(ExitUsage, "ready: --milestones names a Feature's milestones at Approve to build; #%d is not a Feature", n)
		}
		if in.Has("agents") {
			return Errorf(ExitUsage, "ready: --agents sets a Feature's agents at Approve to build; #%d is not a Feature", n)
		}
		w, err := e.botReady(ctx, r)
		if err != nil {
			return err
		}
		// A Story is Ready only while its Feature's approval covers it (Story #996), judged on
		// the item Apply checks, so a type that changes after the read above cannot skip it.
		r.Covered = func(ctx context.Context, it workitems.Item, body string) (string, error) {
			switch it.Type {
			case "feature": // retyped after the read above: its Ready is a person's Approve to build
				return "", Errorf(ExitBlocked, "#%d became a Feature while aayu read it. A Feature's Ready is Approve to build, a person's decision; nothing was written.", n)
			case "story":
			default:
				return "", nil
			}
			return e.storyCovered(ctx, r.GH, w, repo, it, body)
		}
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
		if res.Baselines != nil || res.BaselinesNote != "" {
			fmt.Fprintln(w, "Baselines:")
			for _, b := range res.Baselines {
				fmt.Fprintf(w, "  %s\n", b)
			}
			if res.BaselinesNote != "" {
				fmt.Fprintf(w, "  not read: %s\n", res.BaselinesNote)
			}
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

// botReady makes r write as Arjun: the bot, or the person's token tagged
// identity:human-fallback on the day-one path. It returns Arjun's ledger writer.
func (e *Env) botReady(ctx context.Context, r *ready.Ready) (*ledger.Writer, error) {
	c, bot, err := e.BotGitHub(ctx, "")
	if err != nil {
		return nil, err
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
		return nil, err
	}
	r.Ledger = func(ctx context.Context, c ledger.Commit) (string, error) { return w.Write(ctx, c) }
	return w, nil
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
	Warnings     []string        `json:"warnings,omitempty"`
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
	case !request && (in.Has("changes") || in.Has("fixes")):
		return Errorf(ExitUsage, "ready: --changes and --fixes go with --request")
	case !request && (e.Getenv("AAYU_AGENT") == "1" || e.CI):
		return e.personOnly("ready --part")
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
		return r.request(ctx, who, in.String("changes"), in.String("fixes"))
	}
	return r.approve(ctx)
}

// personOnly is why a person's approval (what: "ready --part") cannot be asked for here.
func (e *Env) personOnly(what string) error {
	if sig := e.agentSignal(); sig != "" {
		return Errorf(ExitBlocked, "%s is a person's decision and is refused for an agent lane (%s)", what, sig)
	}
	return Errorf(ExitBlocked, "%s needs GitHub's device-flow presence check, which is not available here; run it where a person can enter the code", what)
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
	pins, err := r.pins(ctx, it, is.GetBody())
	return it, pins, err
}

// pins is what the part pins on the Feature now (Story #992): only what the
// part owns, so an edit elsewhere is not drift.
func (r *partRun) pins(ctx context.Context, it workitems.Item, body string) ([]ready.Pin, error) {
	if r.part == "intent" {
		return ready.IntentPins(r.n, body), nil
	}
	if r.part == "ux" && !slices.Contains(r.e.footprint(it), string(verbs.L01)) {
		return nil, Errorf(ExitBlocked, "UX design: not needed (no UI layer in the footprint)")
	}
	owner, name, _ := github.SplitRepo(r.repo)
	if r.part == "tech" {
		path := planning.DesignLedgerPath(r.n)
		files, err := r.ledgerFiles(ctx, path)
		if err != nil {
			return nil, err
		}
		if files[path] == nil {
			return nil, Errorf(ExitBlocked, "The technical design of #%d has no design.json yet. Vishwa writes it first: aayu plan %d --draft <file> --create", r.n, r.n)
		}
		comments, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.IssueComment, *gh.Response, error) {
			return r.c.REST.Issues.ListComments(ctx, owner, name, r.n, &gh.IssueListCommentsOptions{ListOptions: o})
		})
		pins := []ready.Pin{ready.FilePin(path, files[path])}
		for _, cm := range comments {
			for _, kind := range chart.Kinds {
				if strings.HasPrefix(cm.GetBody(), chart.Marker(kind)) {
					pins = append(pins, ready.ChartPin(r.n, kind, cm.GetBody()))
				}
			}
		}
		return pins, Wrap(ExitExternal, err)
	}
	subs, err := featureStories(ctx, r.c, r.repo, r.n)
	if err != nil {
		return nil, err
	}
	slices.SortFunc(subs, func(a, b *gh.Issue) int { return a.GetNumber() - b.GetNumber() })
	if r.part == "spec" {
		if len(subs) == 0 {
			return nil, Errorf(ExitBlocked, "#%d has no stories yet. Arjun slices it first: aayu slice %d", r.n, r.n)
		}
		var stories []workitems.Item
		for _, si := range subs {
			s, _ := workitems.Parse(si.GetNumber(), si.GetTitle(), si.GetBody(), si.GetType().GetName(), github.IssueLabels(si))
			stories = append(stories, s)
		}
		return ready.SpecPins(r.n, stories), nil
	}
	var paths []string
	for _, stage := range design.Stages {
		paths = append(paths, design.ManifestPath(r.n, stage))
	}
	files, err := r.ledgerFiles(ctx, paths...)
	if err != nil {
		return nil, err
	}
	var pins []ready.Pin
	for _, p := range paths {
		if files[p] != nil {
			pins = append(pins, ready.FilePin(p, files[p]))
		}
	}
	if len(pins) == 0 {
		return nil, Errorf(ExitBlocked, "The UX design of #%d has no stage manifest on the ledger yet. Hanu writes it first: aayu research %d --from <dir>", r.n, r.n)
	}
	prs, err := openStoryPRs(ctx, r.c, owner, name)
	for _, si := range subs {
		if slices.Contains(github.IssueLabels(si), "early-build") {
			for _, pr := range prs[si.GetNumber()] {
				pins = append(pins, ready.EarlyBuildPin(si.GetNumber(), pr.GetNumber(), pr.GetHead().GetSHA()))
			}
		}
	}
	return pins, Wrap(ExitExternal, err)
}

// ledgerFiles reads paths from the ledger as it is now; a missing ledger or
// file is left out.
func (r *partRun) ledgerFiles(ctx context.Context, paths ...string) (map[string][]byte, error) {
	out := map[string][]byte{}
	t, err := r.w.Read(ctx)
	if err != nil {
		return out, Wrap(ExitExternal, ignoreMissing(err))
	}
	defer t.Close()
	for _, p := range paths {
		b, err := t.File(ctx, p)
		if err = ignoreMissing(err); err != nil {
			return nil, Wrap(ExitExternal, err)
		}
		if b != nil {
			out[p] = b
		}
	}
	return out, nil
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
// waits for a person. It never approves anything. A version after the first
// needs what changed, why it matters, and the need behind it for each pin
// that drifted (Story #993), and names as fixed only findings of its own part.
func (r *partRun) request(ctx context.Context, who, changesFile, fixesList string) error {
	it, pins, err := r.feature(ctx)
	if err != nil {
		return err
	}
	base, draft, _, err := r.records(ctx)
	if err != nil {
		return err
	}
	fixes, err := r.fixes(ctx, fixesList) // checked first, so a bad ID is never ignored
	if err != nil {
		return err
	}
	rep := ReadyPartReport{Repo: r.repo, Feature: r.n, Part: r.part, DryRun: r.c.DryRun()}
	if base != nil && ready.PinsHash(base.Pins) == ready.PinsHash(pins) {
		if len(fixes) > 0 || changesFile != "" { // a fix or a reason needs a change to record it in
			return Errorf(ExitBlocked, "Nothing changed since %s v%d of #%d, so no new version records --changes or --fixes. No draft written.", ready.Label(r.part), base.Version, r.n)
		}
		rep.State, rep.Baseline = "unchanged", base
		return r.e.Emit(rep, func(w io.Writer) error {
			_, err := fmt.Fprintf(w, "Nothing changed since %s v%d of #%d. No draft written.\n", ready.Label(r.part), base.Version, r.n)
			return err
		})
	}
	v, drift := 1, []ready.Drift(nil) // the first version has nothing to change from
	if base != nil {
		v, drift = base.Version+1, ready.Diff(base.Pins, pins)
	}
	l, err := r.e.Config()
	if err != nil {
		return err
	}
	changes, err := r.changes(changesFile, drift, v, l.Config.Baselines.WhyMinWords)
	if err != nil {
		return err
	}
	hash := ready.DraftHash(pins, changes, fixes)
	rep.State, rep.LedgerPath = "drafted", ready.BaselinePath(r.n, r.part, v, true)
	if w := ready.VersionWarning(r.n, r.part, v, l.Config.Baselines.VersionWarn); w != "" {
		rep.Warnings = []string{w}
	}
	if draft != nil && draft.Hash == hash { // drafted already: the same draft, never a second commit
		rep.Baseline = draft
	} else {
		rep.Baseline = &ready.Baseline{Schema: ready.BaselineSchema, Feature: r.n, Part: r.part, Version: v, Owner: ready.Owner(r.part),
			Pins: pins, Changes: changes, Fixes: fixes, Hash: hash, Summary: it.Fields[workitems.FieldSummary], SourceSession: r.e.session(),
			DraftedBy: who, DraftedAt: r.e.Now().UTC().Format(time.RFC3339)}
		rep.LedgerCommit, err = r.write(ctx, fmt.Sprintf("specs: %s v%d draft of #%d", ready.Label(r.part), v, r.n), rep.LedgerPath, "", rep.Baseline)
	}
	if err == nil {
		err = r.comment(ctx, *rep.Baseline, ready.Waits(*rep.Baseline))
	}
	if err != nil {
		return err
	}
	return r.emit(rep, ready.Waits(*rep.Baseline))
}

// changes reads the --changes file and checks it against the drift: what,
// why, and need for each drifted pin, and nothing for a pin that did not.
func (r *partRun) changes(file string, drift []ready.Drift, v, minWords int) ([]ready.Change, error) {
	var given []ready.Change
	if file != "" {
		if err := readDraft(file, &given); err != nil {
			return nil, err
		}
	}
	changes, missing, short, unknown := ready.Reasons(drift, given, minWords)
	switch {
	case len(unknown) > 0 && v == 1:
		return nil, Errorf(ExitUsage, "%s names %s, but %s v1 of #%d is the first version and has no changes.", file, strings.Join(unknown, ", "), ready.Label(r.part), r.n)
	case len(unknown) > 0:
		return nil, Errorf(ExitUsage, "%s names %s: not a change of %s v%d of #%d since v%d. Give one entry per change.", file, strings.Join(unknown, ", "), ready.Label(r.part), v, r.n, v-1)
	}
	if len(missing)+len(short) > 0 {
		msg := fmt.Sprintf("%s v%d of #%d needs what changed, why it matters, and the need behind it for each change.", ready.Label(r.part), v, r.n)
		if len(missing) > 0 {
			msg += " Missing: " + strings.Join(missing, "; ") + "."
		}
		if len(short) > 0 {
			msg += " Too short: " + strings.Join(short, "; ") + "."
		}
		return nil, Errorf(ExitBlocked, "%s No draft written.", msg)
	}
	return changes, nil
}

// fixes checks each ID in the comma-separated list: a finding of this part,
// in a seat's review on the ledger. The IDs are kept in the order given.
func (r *partRun) fixes(ctx context.Context, list string) ([]string, error) {
	var ids []string
	for id := range strings.SplitSeq(list, ",") {
		if id = strings.TrimSpace(id); id != "" && !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	for _, id := range ids {
		part, v, seat, ok := ready.Finding(id)
		switch {
		case !ok:
			return nil, Errorf(ExitUsage, "--fixes %s: not a finding ID (<part>-v<k>/<seat>/<n>)", id)
		case part != r.part:
			return nil, Errorf(ExitUsage, "--fixes %s: a finding of %s, not %s. A version names only findings of its own part as fixed.", id, ready.Label(part), ready.Label(r.part))
		}
		path := ready.ReviewPath(r.n, part, v, seat)
		files, err := r.ledgerFiles(ctx, path)
		if err != nil {
			return nil, err
		}
		var rev ready.Review
		if files[path] == nil || json.Unmarshal(files[path], &rev) != nil || !rev.Has(id) {
			return nil, Errorf(ExitUsage, "--fixes %s: no such finding in %s on the ledger. No draft written.", id, path)
		}
	}
	return ids, nil
}

// baselineStates reads where each part of a Feature stands: its newest
// approved version and its drift from the pins now. It writes nothing.
func (e *Env) baselineStates(ctx context.Context, c *github.Client, tok identity.Token, repo string, is *gh.Issue) ([]ready.PartState, error) {
	w, err := e.Ledger(ctx, tok, "human", "")
	if err != nil {
		return nil, err
	}
	it, _ := workitems.Parse(is.GetNumber(), is.GetTitle(), is.GetBody(), is.GetType().GetName(), github.IssueLabels(is))
	out := []ready.PartState{}
	for _, part := range ready.Parts {
		r := &partRun{e: e, c: c, w: w, repo: repo, n: is.GetNumber(), part: part}
		base, draft, _, err := r.records(ctx)
		if err != nil {
			return nil, err
		}
		s := ready.PartState{Part: part, Drift: []ready.Drift{}}
		if draft != nil {
			s.Draft = draft.Version
		}
		switch {
		case base != nil:
			pins, err := r.pins(ctx, it, is.GetBody())
			if CodeOf(err) == ExitBlocked { // the part has nothing to pin now: say why, and read the other parts
				s.Version, s.Note = base.Version, err.Error()
				break
			}
			if err != nil {
				return nil, err
			}
			s.Version, s.Drift = base.Version, ready.Diff(base.Pins, pins)
		case part == "ux" && !slices.Contains(e.footprint(it), string(verbs.L01)):
			s.Note = "not needed (no UI layer in the footprint)"
		default:
			s.Note = "no baseline yet"
		}
		out = append(out, s)
	}
	return out, nil
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
	if err := r.holds(ctx, ready.PinsHash(draft.Pins)); err != nil {
		return err
	}
	effect := fmt.Sprintf("Baselines %s v%d of #%d at hash %s, as %s drafted it; a later edit shows as drift.",
		ready.Label(r.part), draft.Version, r.n, draft.Hash[:12], or(draft.DraftedBy, "an agent"))
	a, err := r.e.presenceCheck(ctx, Pending{What: "ready --part", Effect: []string{effect}}, AccessWrite, r.e.personOnly("ready --part"))
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
	if err := r.holds(ctx, ready.PinsHash(draft.Pins)); err != nil {
		return err
	}
	b := *draft // what the person saw: the hash covers the pins, changes, and fixes
	b.ApprovedBy, b.ApprovedAt, b.Presence = a.Login, r.e.Now().UTC().Format(time.RFC3339), a.Via
	rep := ReadyPartReport{Repo: r.repo, Feature: r.n, Part: r.part, State: "baselined", Baseline: &b,
		LedgerPath: ready.BaselinePath(r.n, r.part, b.Version, false), DryRun: r.c.DryRun()}
	rep.LedgerCommit, err = r.write(ctx, fmt.Sprintf("specs: %s v%d baseline of #%d, approved by @%s", ready.Label(r.part), b.Version, r.n, a.Login), rep.LedgerPath, tip, &b)
	if err == nil {
		err = r.comment(ctx, b, ready.Baselined(b))
	}
	if err != nil {
		return err
	}
	return r.emit(rep, ready.Baselined(b))
}

// holds checks that the part's pins on the Feature now have the pins hash.
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

// comment keeps the part's one marker comment on the Feature: the line, and
// the version's changes and fixes.
func (r *partRun) comment(ctx context.Context, b ready.Baseline, line string) error {
	marker := ready.BaselineMarker(r.part)
	_, err := r.c.UpsertComment(ctx, r.repo, r.n, marker, ready.Comment(b, line), actions.Action{Kind: "github.comment.baseline",
		Payload: map[string]any{"part": r.part}})
	return Wrap(ExitExternal, err)
}

func (r *partRun) emit(rep ReadyPartReport, line string) error {
	return r.e.Emit(rep, func(w io.Writer) error {
		fmt.Fprintln(w, strings.ReplaceAll(line, "`", ""))
		for _, warn := range rep.Warnings {
			fmt.Fprintf(w, "  warning: %s\n", warn)
		}
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
