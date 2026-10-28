package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/autonomy"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/identity"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/planning"
	"github.com/rahulnakmol/agentic-ayudha/internal/ready"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
	"github.com/rahulnakmol/agentic-ayudha/internal/verify"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

func init() { register("plan", runPlan) }

// PlanRecord is specs/<story>/plan.json on the ledger: the checked plan, the
// story body it was made for, and the EM's plan-time review.
type PlanRecord struct {
	Story       int                   `json:"story"`
	BodyHash    string                `json:"body_hash"`
	Footprint   []string              `json:"footprint"`
	Checkpoints []planning.Checkpoint `json:"checkpoints"`
	Rollout     *planning.Rollout     `json:"rollout,omitempty"`
	Review      planning.Review       `json:"em_review"`
	PlannedAt   string                `json:"planned_at"`
	PlannedBy   string                `json:"planned_by"`
}

// PlanLedgerPath is where a story's plan lives on the ledger.
func PlanLedgerPath(n int) string { return fmt.Sprintf("specs/%d/plan.json", n) }

// PlanResult is what plan reports.
type PlanResult struct {
	planning.PlanReport
	Criteria []workitems.Criterion `json:"criteria"`
	Never    []string              `json:"never_layers"`
	// Features are the mapped features in the footprint's layers (WP4.5b.5),
	// whose checks a checkpoint names in its checks.
	Features []PlanFeature `json:"features,omitempty"`
	Ledger   string        `json:"ledger,omitempty"`
	DryRun   bool          `json:"dry_run,omitempty"`
	Next     string        `json:"next,omitempty"`
	Warnings []string      `json:"warnings,omitempty"`
}

// PlanFeature is one mapped feature a plan can ground its checkpoints in.
type PlanFeature struct {
	ID     string   `json:"id"`
	Layers []string `json:"layers"`
	File   string   `json:"file"`
	Checks []string `json:"checks"`
}

// planFeatures are the features in the footprint's layers, and every check
// ref of every map.
func planFeatures(ctx context.Context, root string, footprint []string) ([]PlanFeature, map[string]bool) {
	known := map[string]bool{}
	if root == "" {
		return nil, known
	}
	maps, _ := verify.LoadMaps(ctx, root)
	var out []PlanFeature
	for _, m := range maps {
		for _, f := range m.Features {
			pf := PlanFeature{ID: f.ID, Layers: f.Layers, File: f.File}
			for _, c := range f.Checks {
				known[f.Ref(c)] = true
				pf.Checks = append(pf.Checks, f.Ref(c))
			}
			if slices.ContainsFunc(f.Layers, func(l string) bool { return slices.Contains(footprint, l) }) {
				out = append(out, pf)
			}
		}
	}
	return out, known
}

// readPlan reads a story's plan from the ledger.
func readPlan(ctx context.Context, w *ledger.Writer, n int) (*PlanRecord, error) {
	t, err := w.Read(ctx)
	if err != nil {
		return nil, err
	}
	defer t.Close()
	b, err := t.File(ctx, PlanLedgerPath(n))
	if err != nil {
		return nil, err
	}
	var p PlanRecord
	return &p, json.Unmarshal(b, &p)
}

// runPlan (WP5.4): the host's model plans (skills/plan); the binary checks
// the plan against the story: it passes Ready, one checkpoint per layer of
// its footprint (the layer labels slice wrote, or the plan's layers, which
// then become the footprint), tests on every checkpoint, a rollback on an L07
// one, every criterion covered. --create writes specs/<story>/plan.json to
// the ledger; a footprint touching an [autonomy] never layer needs the
// Engineering Manager's review, which a human records with --approve.
func runPlan(ctx context.Context, e *Env, in Values) error {
	repo, err := e.RepoName(ctx)
	if err != nil {
		return err
	}
	l, err := e.Config()
	if err != nil {
		return err
	}
	n := issueNumber(in.String("story"))
	if in.Bool("approve") && in.Bool("solo-bypass") {
		return Errorf(ExitUsage, "plan: --approve records a review and --solo-bypass records that there was none; pass one")
	}
	if in.Bool("approve") {
		return e.approvePlan(ctx, repo, n, &l.Config)
	}
	if in.Bool("solo-bypass") {
		return e.bypassPlanReview(ctx, repo, n, &l.Config)
	}
	c, bot, err := e.BotGitHub(ctx, "")
	if err != nil {
		return err
	}
	rd := &ready.Ready{GH: c, Repo: repo, TechnicalOwner: e.technicalOwner()}
	chk, is, err := rd.Check(ctx, n)
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	if chk.Type == "feature" {
		return e.planFeature(ctx, c, bot, repo, is, chk.BodyHash, in)
	}
	if !chk.Pass || (chk.Type != "story" && chk.Type != "bug") {
		return Errorf(ExitBlocked, "plan #%d: a plan is for a story or bug that passes Ready; this %s has %d Ready problem(s) (`aayu ready %d --check`)", n, chk.Type, len(chk.Problems), n)
	}
	labels := github.IssueLabels(is)
	story, _ := workitems.Parse(n, is.GetTitle(), is.GetBody(), is.GetType().GetName(), labels)
	footprint := planning.Footprint(labels)
	never := l.Config.Autonomy.Never
	res := PlanResult{Criteria: chk.Criteria, Never: never}
	var known map[string]bool
	res.Features, known = planFeatures(ctx, config.RepoRoot(e.Dir), footprint)
	if !in.Has("draft") {
		res.PlanReport = planning.PlanReport{Story: n, Footprint: footprint, Problems: []string{}}
		res.Next = fmt.Sprintf("draft one checkpoint per footprint layer (skills/plan), then `aayu plan %d --draft <file>`", n)
		return e.Emit(res, func(w io.Writer) error {
			fmt.Fprintf(w, "#%d %s: footprint %s; %d criteria; never layers %s\n", n, story.Title, listOrNone(footprint), len(chk.Criteria), listOrNone(never))
			for _, f := range res.Features {
				fmt.Fprintf(w, "  feature %s (%s): checks %s\n", f.ID, f.File, strings.Join(f.Checks, ", "))
			}
			fmt.Fprintf(w, "  next: %s\n", res.Next)
			return nil
		})
	}
	var d planning.PlanDraft
	if err := readDraft(in.String("draft"), &d); err != nil {
		return err
	}
	res.PlanReport = planning.CheckPlan(story, footprint, never, d)
	if defs, _, err := readSLOs(config.RepoRoot(e.Dir)); err != nil {
		return Wrap(ExitUsage, err)
	} else if defs != nil {
		var names []string
		for _, o := range defs.SLOs {
			names = append(names, o.Name)
		}
		if ps := planning.CheckRollout(d, names); len(ps) > 0 {
			res.Problems, res.Pass = append(res.Problems, ps...), false
		}
	}
	// A story plan stays inside its Feature's design; a bug fix may need
	// another layer. A plan that is only checked may be checked where the
	// ledger cannot be read; a plan that is written is checked, or refused.
	if chk.Type == "story" {
		design, err := e.featureDesign(ctx, c, bot, repo, n)
		switch {
		case err != nil && in.Bool("create"):
			return Wrap(ExitExternal, err)
		case err != nil:
			res.Warnings = append(res.Warnings, fmt.Sprintf("the technical design of the story's Feature was not read from the ledger (%v); --create reads it and refuses when it cannot", err))
		case design != nil:
			if ps := planning.CheckInDesign(*design, d.Checkpoints); len(ps) > 0 {
				res.Problems, res.Pass = append(res.Problems, ps...), false
			}
		}
	}
	for i, cp := range d.Checkpoints {
		for _, ref := range cp.Checks {
			if !known[ref] {
				res.Problems = append(res.Problems, fmt.Sprintf("checkpoint %d (%s): no check %s in the feature maps (`aayu verify --map --json` lists them)", i+1, cp.Layer, ref))
				res.Pass = false
			}
		}
	}
	if res.Pass && in.Bool("create") {
		res.DryRun = c.DryRun()
		rec := PlanRecord{Story: n, BodyHash: chk.BodyHash, Footprint: res.Footprint, Checkpoints: d.Checkpoints, Rollout: d.Rollout, Review: res.Review,
			PlannedAt: e.Now().UTC().Format(time.RFC3339), PlannedBy: c.Identity()}
		if (res.FromPlan || len(res.Added) > 0) && !res.DryRun {
			layers := res.Added
			if res.FromPlan {
				layers = res.Footprint
			}
			add := make([]string, 0, len(layers))
			for _, x := range layers {
				add = append(add, planning.LayerLabel+x)
			}
			owner, name, _ := github.SplitRepo(repo)
			if _, err := c.Write(ctx, actions.Action{Kind: "github.issue.labels.add", Target: fmt.Sprintf("%s#%d", repo, n),
				Request: &actions.Request{Method: http.MethodPost, Path: fmt.Sprintf("/repos/%s/issues/%d/labels", repo, n)}, Payload: map[string]any{"labels": add}},
				func(ctx context.Context) (any, error) {
					_, _, err := c.REST.Issues.AddLabelsToIssue(ctx, owner, name, n, add)
					return nil, err
				}); err != nil {
				return Wrap(ExitExternal, err)
			}
		}
		if !res.DryRun {
			w, err := e.Ledger(ctx, bot.Token, c.Identity(), bot.Tag)
			if err != nil {
				return err
			}
			b, _ := json.MarshalIndent(rec, "", "  ")
			if _, err := w.Write(ctx, ledger.Commit{Message: fmt.Sprintf("specs: plan of #%d", n), Files: map[string][]byte{PlanLedgerPath(n): append(b, '\n')}}); err != nil {
				return Wrap(ExitExternal, err)
			}
			res.Ledger = PlanLedgerPath(n)
		}
	}
	if err := e.Emit(res, func(w io.Writer) error {
		verdict := "passes"
		if !res.Pass {
			verdict = "does not pass yet"
		}
		fmt.Fprintf(w, "plan of #%d: %d checkpoint(s) over footprint %s, %s\n", n, len(d.Checkpoints), listOrNone(res.Footprint), verdict)
		for i, cp := range d.Checkpoints {
			fmt.Fprintf(w, "  %d/%d %s %s (tests: %s)\n", i+1, len(d.Checkpoints), cp.Layer, cp.Title, strings.Join(cp.Tests, ", "))
		}
		for _, p := range res.Problems {
			fmt.Fprintf(w, "  problem: %s\n", p)
		}
		for _, x := range res.Warnings {
			fmt.Fprintf(w, "  warning: %s\n", x)
		}
		if res.Review.Required {
			fmt.Fprintf(w, "  EM plan-time review required (never layers %s): a human runs `aayu plan %d --approve`\n", strings.Join(res.Review.Layers, ", "), n)
		}
		switch {
		case res.Ledger != "":
			fmt.Fprintf(w, "  wrote %s on the ledger\n", res.Ledger)
		case res.DryRun:
			fmt.Fprintln(w, "  --dry-run: the plan is planned, not written")
		case res.Pass && !in.Bool("create"):
			fmt.Fprintln(w, "  run again with --create to write it to the ledger")
		}
		return nil
	}); err != nil {
		return err
	}
	if !res.Pass {
		return Errorf(ExitBlocked, "the plan of #%d has %d problem(s); revise the draft", n, len(res.Problems))
	}
	return nil
}

// TechDesignResult is what plan reports for a Feature: the check of its
// technical design draft.
type TechDesignResult struct {
	planning.DesignReport
	ExpectedPaths []string `json:"expected_paths"`
	Ledger        string   `json:"ledger,omitempty"`
	DryRun        bool     `json:"dry_run,omitempty"`
	Next          string   `json:"next,omitempty"`
}

// planFeature (Feature #874, S5): the host's model writes the Feature's
// technical design (skills/plan); the binary checks it against the Feature's
// footprint (from its expected paths) and its stories, and --create writes
// specs/<feature>/design.json to the ledger. A Feature never gets a story
// plan record.
func (e *Env) planFeature(ctx context.Context, c *github.Client, bot identity.Bot, repo string, is *gh.Issue, bodyHash string, in Values) error {
	n := is.GetNumber()
	it, _ := workitems.Parse(n, is.GetTitle(), is.GetBody(), is.GetType().GetName(), github.IssueLabels(is))
	paths := planning.Paths(it.Fields["expected_paths"])
	footprint := e.footprint(it)
	subs, err := featureStories(ctx, c, repo, n)
	if err != nil {
		return err
	}
	stories, titles := []int{}, map[int]string{}
	for _, si := range subs {
		stories, titles[si.GetNumber()] = append(stories, si.GetNumber()), si.GetTitle()
	}
	res := TechDesignResult{ExpectedPaths: paths}
	if !in.Has("draft") {
		res.DesignReport = planning.DesignReport{Feature: n, Footprint: footprint, Stories: stories, Problems: []string{}}
		res.Next = fmt.Sprintf("draft the technical design (skills/plan), then aayu plan %d --draft <file>", n)
		return e.Emit(res, func(w io.Writer) error {
			fmt.Fprintf(w, "#%d %s: footprint %s from expected paths %s; %d stories\n", n, is.GetTitle(), listOrNone(footprint), listOrNone(paths), len(stories))
			for _, s := range stories {
				fmt.Fprintf(w, "  story #%d %s\n", s, titles[s])
			}
			fmt.Fprintf(w, "  next: %s\n", res.Next)
			return nil
		})
	}
	var d planning.DesignDraft
	if err := readDraft(in.String("draft"), &d); err != nil {
		return err
	}
	res.DesignReport = planning.CheckDesign(n, footprint, stories, d)
	if res.Pass && in.Bool("create") {
		res.DryRun = c.DryRun()
		if !res.DryRun {
			rec := planning.Design{Feature: n, BodyHash: bodyHash, Footprint: footprint, Contracts: d.Contracts, Layers: d.Layers, StoryOrder: d.StoryOrder, Rollback: d.Rollback,
				PlannedAt: e.Now().UTC().Format(time.RFC3339), PlannedBy: c.Identity()}
			w, err := e.Ledger(ctx, bot.Token, c.Identity(), bot.Tag)
			if err != nil {
				return err
			}
			b, _ := json.MarshalIndent(rec, "", "  ")
			if _, err := w.Write(ctx, ledger.Commit{Message: fmt.Sprintf("specs: technical design of #%d", n), Files: map[string][]byte{planning.DesignLedgerPath(n): append(b, '\n')}}); err != nil {
				return Wrap(ExitExternal, err)
			}
			res.Ledger = planning.DesignLedgerPath(n)
			res.Next = fmt.Sprintf("aayu ready %d --part tech --request", n)
		}
	}
	if err := e.Emit(res, func(w io.Writer) error {
		verdict := "passes"
		if !res.Pass {
			verdict = "does not pass yet"
		}
		fmt.Fprintf(w, "technical design of #%d: layers %s, story order %d of %d stories, %s\n", n, listOrNone(d.Layers), len(d.StoryOrder), len(stories), verdict)
		for _, p := range res.Problems {
			fmt.Fprintf(w, "  problem: %s\n", p)
		}
		switch {
		case res.Ledger != "":
			fmt.Fprintf(w, "  wrote %s on the ledger\n  next: %s\n", res.Ledger, res.Next)
		case res.DryRun:
			fmt.Fprintln(w, "  --dry-run: the design is planned, not written")
		case res.Pass && !in.Bool("create"):
			fmt.Fprintln(w, "  run again with --create to write it to the ledger")
		}
		return nil
	}); err != nil {
		return err
	}
	if !res.Pass {
		return Errorf(ExitBlocked, "the technical design of #%d has %d problem(s); revise the draft", n, len(res.Problems))
	}
	return nil
}

// footprint is the layers a Feature's expected paths fall in.
func (e *Env) footprint(feature workitems.Item) []string {
	return planning.GlobFootprint(autonomy.ReadLayers(config.RepoRoot(e.Dir)), planning.Paths(feature.Fields["expected_paths"]))
}

// featureStories lists Feature n's child stories (SubIssue.ListByIssue),
// leaving out a cancelled one.
func featureStories(ctx context.Context, c *github.Client, repo string, n int) ([]*gh.Issue, error) {
	return featureItems(ctx, c, repo, n, "story")
}

// featureItems are the sub-issues of Feature n of the given types, without
// cancelled ones.
func featureItems(ctx context.Context, c *github.Client, repo string, n int, types ...string) ([]*gh.Issue, error) {
	owner, name, _ := github.SplitRepo(repo)
	subs, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.SubIssue, *gh.Response, error) {
		return c.REST.SubIssue.ListByIssue(ctx, owner, name, int64(n), &o)
	})
	if err != nil {
		return nil, Wrap(ExitExternal, err)
	}
	var out []*gh.Issue
	for _, s := range subs {
		si := (*gh.Issue)(s)
		labels := github.IssueLabels(si)
		if typ, _ := workitems.TypeOf(si.GetType().GetName(), labels); slices.Contains(types, typ) && !slices.Contains(labels, "status:cancelled") {
			out = append(out, si)
		}
	}
	return out, nil
}

// featureDesign reads, from the ledger, the technical design of the Feature
// that story n is under: nil when it has no parent, or the parent has no
// design (only a Feature has one), or there is no ledger.
func (e *Env) featureDesign(ctx context.Context, c *github.Client, bot identity.Bot, repo string, n int) (*planning.Design, error) {
	owner, name, _ := github.SplitRepo(repo)
	parent, _, err := c.REST.SubIssue.GetParentIssue(ctx, owner, name, int64(n))
	if err != nil {
		return nil, ignoreMissing(err)
	}
	w, err := e.Ledger(ctx, bot.Token, c.Identity(), bot.Tag)
	if err != nil {
		return nil, err
	}
	t, err := w.Read(ctx)
	if err != nil {
		return nil, ignoreMissing(err)
	}
	defer t.Close()
	path := planning.DesignLedgerPath(parent.GetNumber())
	b, err := t.File(ctx, path)
	if err != nil {
		return nil, ignoreMissing(err)
	}
	var d planning.Design
	if err := json.Unmarshal(b, &d); err != nil {
		return nil, fmt.Errorf("%s on the ledger: %w", path, err)
	}
	return &d, nil
}

// ignoreMissing is nil for a missing parent issue, ledger, or ledger file,
// else err.
func ignoreMissing(err error) error {
	if github.NotFound(err) || errors.Is(err, ledger.ErrNoLedger) || errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// approvePlan records the Engineering Manager's plan-time review: a human
// (never an agent) who is the EM seat's paired human, else the [autonomy]
// approver, else, in solo mode, the one human, recorded as such.
func (e *Env) approvePlan(ctx context.Context, repo string, n int, cfg *config.Config) error {
	if err := e.requireHuman(ctx, "plan --approve (the Engineering Manager's review)", AccessWrite); err != nil {
		return err
	}
	_, tok, err := e.HumanGitHub(ctx)
	if err != nil {
		return err
	}
	me, err := identity.WhoAmI(ctx, e.HTTP, e.githubAPI(), tok)
	if err != nil {
		return Wrap(ExitBlocked, err)
	}
	w, err := e.Ledger(ctx, tok, "human", "")
	if err != nil {
		return err
	}
	p, err := readPlan(ctx, w, n)
	if errors.Is(err, ledger.ErrNoLedger) || errors.Is(err, os.ErrNotExist) {
		return Errorf(ExitBlocked, "plan --approve: #%d has no plan on the ledger; `aayu plan %d --draft <file> --create` first", n, n)
	}
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	if !p.Review.Required {
		return Errorf(ExitUsage, "plan --approve: #%d's plan touches no [autonomy] never layer; no review is needed", n)
	}
	as := "solo"
	switch em := cfg.Seats[string(verbs.EngineeringManager)]; {
	case em.Mode == "paired" && em.Human != "":
		if !strings.EqualFold(em.Human, me.Login) {
			return Errorf(ExitBlocked, "plan --approve: the Engineering Manager seat's human is %s, not %s", em.Human, me.Login)
		}
		as = "seat"
	case cfg.Autonomy.Approver != "":
		if !strings.EqualFold(cfg.Autonomy.Approver, me.Login) {
			return Errorf(ExitBlocked, "plan --approve: the [autonomy] approver is %s, not %s", cfg.Autonomy.Approver, me.Login)
		}
		as = "approver"
	}
	p.Review.ApprovedBy, p.Review.ApprovedAt, p.Review.As = me.Login, e.Now().UTC().Format(time.RFC3339), as
	b, _ := json.MarshalIndent(p, "", "  ")
	if _, err := w.Write(ctx, ledger.Commit{Message: fmt.Sprintf("specs: EM plan-time review of #%d by %s", n, me.Login), Files: map[string][]byte{PlanLedgerPath(n): append(b, '\n')}}); err != nil {
		return Wrap(ExitExternal, err)
	}
	return e.Emit(p, func(w io.Writer) error {
		_, err := fmt.Fprintf(w, "#%d: EM plan-time review of layers %s recorded by %s (as %s) in %s\n", n, strings.Join(p.Review.Layers, ", "), me.Login, as, PlanLedgerPath(n))
		return err
	})
}

// bypassPlanReview records, in solo mode, that a never-layer plan proceeds
// with no plan-time review at all: no second person exists to review it, and
// the one human did not review it as the Engineering Manager. It needs the
// human's comment on the story starting "Solo-mode bypass:" and is written as
// a bypass (em_review.bypass), never as an approval, so `ship` accepts it,
// and the ship record and `audit` list it as a plan-review bypass.
func (e *Env) bypassPlanReview(ctx context.Context, repo string, n int, cfg *config.Config) error {
	if err := e.requireHuman(ctx, "plan --solo-bypass", AccessAdmin); err != nil {
		return err
	}
	c, tok, err := e.HumanGitHub(ctx)
	if err != nil {
		return err
	}
	me, err := identity.WhoAmI(ctx, e.HTTP, e.githubAPI(), tok)
	if err != nil {
		return Wrap(ExitBlocked, err)
	}
	w, err := e.Ledger(ctx, tok, "human", "")
	if err != nil {
		return err
	}
	p, err := readPlan(ctx, w, n)
	if errors.Is(err, ledger.ErrNoLedger) || errors.Is(err, os.ErrNotExist) {
		return Errorf(ExitBlocked, "plan --solo-bypass: #%d has no plan on the ledger; `aayu plan %d --draft <file> --create` first", n, n)
	}
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	if !p.Review.Required {
		return Errorf(ExitUsage, "plan --solo-bypass: #%d's plan touches no [autonomy] never layer; no review is needed", n)
	}
	if p.Review.ApprovedBy != "" {
		return Errorf(ExitUsage, "plan --solo-bypass: #%d's plan was already reviewed by %s; no bypass is needed", n, p.Review.ApprovedBy)
	}
	owner, name, _ := github.SplitRepo(repo)
	url, err := soloModeComment(ctx, c, owner, name, n, me.Login, cfg, "plan --solo-bypass", "a comment on the story", "")
	if err != nil {
		return err
	}
	p.Review.Bypass = &planning.ReviewBypass{As: "solo-bypass", Reviewer: "none (solo mode)", RecordedBy: me.Login, RecordedAt: e.Now().UTC().Format(time.RFC3339), Comment: url,
		Note: "solo mode: no second person exists to review the plan, and no one reviewed it as the Engineering Manager"}
	b, _ := json.MarshalIndent(p, "", "  ")
	if _, err := w.Write(ctx, ledger.Commit{Message: fmt.Sprintf("specs: solo-mode plan-review bypass of #%d recorded by %s (no review)", n, me.Login), Files: map[string][]byte{PlanLedgerPath(n): append(b, '\n')}}); err != nil {
		return Wrap(ExitExternal, err)
	}
	return e.Emit(p, func(w io.Writer) error {
		_, err := fmt.Fprintf(w, "#%d: no plan-time review of layers %s; solo-mode bypass recorded by %s (%s) in %s\n", n, strings.Join(p.Review.Layers, ", "), me.Login, url, PlanLedgerPath(n))
		return err
	})
}
