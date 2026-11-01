package cli

import (
	"archive/zip"
	"bytes"
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/autonomy"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/identity"
	"github.com/rahulnakmol/agentic-ayudha/internal/inspect"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/planning"
	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
	"github.com/rahulnakmol/agentic-ayudha/internal/ready"
	"github.com/rahulnakmol/agentic-ayudha/internal/seats"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

func init() { register("ship", runShip) }

// Gate checks ship requires on the PR head (the merge gate's jobs, WP4.8;
// aayudha/pack checks the pack and keeps its comment, S-4.2b).
var gateChecks = []string{"aayudha/merge-gate", "aayudha/inspect", "aayudha/pack"}

// SoloBypassPrefix starts the PR comment in which the merging human records a
// solo-mode bypass (Appendix B D2).
const SoloBypassPrefix = "Solo-mode bypass:"

// ReadyToShipMarker starts the comment ship leaves when everything but the
// human pass holds on a head; `aayu inbox` lists the PR for a person while the
// head is unchanged (readyHead).
const ReadyToShipMarker = "<!-- aayudha:ready-to-ship"

var readyHead = regexp.MustCompile(regexp.QuoteMeta(ReadyToShipMarker) + ` head=([0-9a-f]{40}) -->`)

// HumanPass records how the human pass was met.
type HumanPass struct {
	Kind     string `json:"kind"` // review, autonomous, or solo-bypass
	Login    string `json:"login"`
	ReviewID int64  `json:"review_id,omitempty"`
	Comment  string `json:"comment_url,omitempty"`
	Commit   string `json:"commit"`
	Note     string `json:"note,omitempty"`
}

// ShipRecord is proof/<story>/<head>/authoritative/ship.json on the ledger.
type ShipRecord struct {
	Repo        string `json:"repo"`
	Story       int    `json:"story"`
	PR          int    `json:"pr"`
	Head        string `json:"pr_head_sha"`
	MergeCommit string `json:"merge_commit"`
	Pack        string `json:"pack"`
	PackLanded  string `json:"pack_landed,omitempty"` // how the authoritative pack reached the ledger
	// The merge gate run that built the pack, its aayudha-pack artifact, and
	// the SHA-256 of the artifact's pack.json, as the gate's comment shows it.
	GateRun    int64     `json:"gate_run,omitempty"`
	Artifact   int64     `json:"artifact,omitempty"`
	PackSHA256 string    `json:"pack_sha256,omitempty"`
	HumanPass  HumanPass `json:"human_pass"`
	// GateFix is the gate_fixes entry of a fix PR (Story #1002): the PR refers
	// to its Feature, and the entry, not a Ready snapshot, authorises it.
	GateFix *GateFix `json:"gate_fix,omitempty"`
	// Proof is what a merge on proof rested on (human pass kind proof).
	Proof *ProofPass `json:"proof,omitempty"`
	// The review ladder's rung of the PR's review at the head (Decision
	// #469), from the merge gate's own inspect log (autonomy.GateRung): A,
	// B, C, or human; empty when the log records none.
	Review string `json:"review_rung,omitempty"`
	// Who could give the human pass, the footprint's never layers, the
	// autonomy streak, and the EM plan-time review (WP5.5).
	Footprint  []string         `json:"footprint"`
	Pass       PassRule         `json:"pass_rule"`
	Streak     StreakState      `json:"streak"`
	PlanReview *planning.Review `json:"plan_review,omitempty"`
	Waivers    []string         `json:"waivers,omitempty"`
	Actions    string           `json:"actions_segment"`
	ShippedAt  string           `json:"shipped_at"`
	ShippedBy  string           `json:"shipped_by"`
}

// ShipResult is what ship reports.
type ShipResult struct {
	ShipRecord
	Record string `json:"ship_record"`
	Branch string `json:"branch_deleted,omitempty"`
	// Next says whether the Kanha of the story's Feature was told of the merge (Story #1007).
	Next string `json:"next,omitempty"`
}

// runShip (WP5.5, from the WP4.10 thin slice; S-4.2b): the merge gate's
// pack, taken from the aayudha-pack artifact of the base branch's gate run on
// the head and checked there, a green merge gate, the pack verifying against
// the head and the Ready snapshot with its floors holding, and the human pass:
// an approval of the head by the seat's human (solo: the [autonomy]
// approver), never a bot or the author, plus a CODEOWNER's approval and the
// EM's plan-time review for a never layer, whatever the streak. Then the pack
// (pack.json and data, no media) lands on the ledger under the human's
// identity, the squash-merge, the Complete section, status:merged, the ship
// record (with the run, the artifact, the pack's hash, the approval, the pass
// rule, and the streak state) beside the pack, the lease released, and
// cleanup.
func runShip(ctx context.Context, e *Env, in Values) error {
	// An agent lane or CI can never give the bypass, so it is refused before
	// anything is read; a person's device code is asked for only once every
	// check that needs no person holds, just before the first write (Story #1206).
	if in.Bool("solo-bypass") && (e.Getenv("AAYU_AGENT") == "1" || e.CI) {
		if err := e.requireHuman(ctx, "ship --solo-bypass", AccessAdmin); err != nil {
			return err
		}
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
	c, tok, err := e.HumanGitHub(ctx)
	if err != nil {
		return err
	}
	// Whatever ship did reaches the ledger as a segment, also when it refuses
	// after landing the gate's pack; the merged path checkpoints itself below.
	checkpointed := false
	defer func() {
		if !checkpointed {
			_ = e.checkpoint(tok)
		}
	}()
	me, err := identity.WhoAmI(ctx, e.HTTP, e.githubAPI(), tok)
	if err != nil {
		return Wrap(ExitBlocked, err)
	}
	owner, name, _ := github.SplitRepo(repo)
	pr, _, err := c.REST.PullRequests.Get(ctx, owner, name, n)
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	if pr.GetMerged() || pr.GetState() != "open" {
		return Errorf(ExitUsage, "ship #%d: the PR is %s", n, map[bool]string{true: "already merged", false: "closed"}[pr.GetMerged()])
	}
	head := pr.GetHead().GetSHA()
	if want := in.String(shipExpectHead); want != "" && want != head {
		return headMoved(n, want, head)
	}
	story := StoryOf(pr.GetBody(), "", pr.GetHead().GetRef())
	if story == 0 {
		return Errorf(ExitBlocked, "ship #%d: no story (Refs #n in the PR body); ship proves a story", n)
	}
	// The body can be edited; a build's branch names its story. When they
	// differ, ship would prove, and check for a stop, the wrong story.
	if b := StoryOf("", "", pr.GetHead().GetRef()); b != 0 && b != story {
		return Errorf(ExitBlocked, "ship #%d: the PR body refers to story #%d, but its branch %s is story #%d's; fix the body's Refs line", n, story, pr.GetHead().GetRef(), b)
	}
	// An early build's PR never merges into dev (core design E1), whatever its checks and review say.
	// A story that does not exist has no label; ship refuses it later (no Ready snapshot).
	ref, _, err := c.REST.Issues.Get(ctx, owner, name, story)
	if err != nil && !github.NotFound(err) {
		return Wrap(ExitExternal, err)
	} else if err == nil && contains(github.IssueLabels(ref), ready.EarlyBuildLabel) {
		return Errorf(ExitBlocked, "ship #%d: %s", n, ready.EarlyBuildHold(story))
	}
	// The mission the PR is in holds it before anything lands (Story #1002),
	// and again just before the merge. With no checkout there is no ledger to
	// read, and ship stops before the merge (proofStore).
	var fix *GateFix
	if mw, lerr := e.Ledger(ctx, tok, "human", ""); lerr == nil {
		if fix, err = missionHold(ctx, c, mw, repo, n, story, ref); err != nil {
			return err
		}
	}
	rec := ShipRecord{Repo: repo, Story: story, PR: n, Head: head, ShippedBy: me.Login, GateFix: fix}
	// Who may pass the PR, and how, is the base branch's committed policy at
	// its tip now (#645), never this run's config: the checkout, the user
	// config, AAYU_* variables and --set are the agent's to set. A run whose
	// config would loosen the pass is refused rather than quietly overruled.
	pol, err := basePolicy(ctx, c, owner, name, pr.GetBase().GetRef())
	if err != nil {
		return err
	}
	if loose := loosens(l, pol.Config); len(loose) > 0 {
		return Errorf(ExitBlocked, "ship #%d: the human pass is judged by %s's committed .aayudha/config.toml at %s, and this run's config (the checkout, the user config, AAYU_* variables, or --set) would loosen it: %s; commit a policy change to %s through a reviewed PR instead",
			n, pr.GetBase().GetRef(), short(pol.Tip), strings.Join(loose, "; "), pr.GetBase().GetRef())
	}

	// 1. The pack the merge gate built for this head, from the artifact of a
	// run of the base branch's workflow (S-4.2b): never a pack on the ledger,
	// a preview, or an artifact another workflow uploaded.
	gp, err := e.gatePack(ctx, c, owner, name, "ship", pr, story)
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(gp.tmp) }()
	rec.GateRun, rec.Artifact, rec.PackSHA256 = gp.run.GetID(), gp.artifact, gp.sha256

	// 2. The merge gate's checks are green on the head.
	runs, err := gateGreen(ctx, c, owner, name, "ship", n, head, gp)
	if err != nil {
		return err
	}

	// The pack verifies against the head and the Ready snapshot, and its floors hold.
	s, err := e.proofStore(ctx, c, tok, "human", "")
	if err != nil {
		return err
	}
	if err := verifyGatePack(ctx, s, "ship", n, story, head, gp, fix); err != nil {
		return err
	}
	for _, ck := range gp.pack.Checks {
		if ck.Waiver != "" {
			rec.Waivers = append(rec.Waivers, ck.Floor+" by "+ck.Waiver)
		}
	}
	// Every required check of the base branch is green too, whoever passes it.
	required, gateState, err := requiredChecks(ctx, c, owner, name, "ship", n, pr.GetBase().GetRef(), head, runs)
	if err != nil {
		return err
	}
	// The review ladder's rung at the head, from the merge gate's own inspect log.
	// The rung comes only from a run GitHub lists as for this PR into its
	// base: a helper PR into a branch with an edited gate cannot lend its rung
	// to the record, the streak's pass, or proof. For proof it must also be a
	// fresh judgement, and the pack's run bound the same way.
	var fresh bool
	rec.Review, fresh = autonomy.GateRungFresh(ctx, c, owner, name, n, head, pr.GetBase().GetRef())
	proofRung := rec.Review
	switch {
	case !autonomy.RunFor(gp.run, n, pr.GetBase().GetRef()):
		proofRung = fmt.Sprintf("unproven: the pack's run %d is not listed as for PR #%d into %s", gp.run.GetID(), n, pr.GetBase().GetRef())
	case !fresh && proofRung != "":
		proofRung = "cached " + proofRung
	}
	// Merge on proof (Decision #593): with it, no human pass is needed.
	proofOK, proofWhy, needsPerson, err := e.proofPass(ctx, c, owner, name, pr, pol, proofRung, gateState)
	if err != nil {
		return err
	}
	if proofOK != nil {
		proofOK.Checks = required
	}

	// 3. The human pass (WP5.5): an approval of the head by the seat's human
	// (solo: the [autonomy] approver), never a bot or the author; a never
	// layer also needs a CODEOWNER's approval, whatever the streak, and the
	// EM's plan-time review. A footprint the streak makes autonomous needs no
	// review.
	// The footprint is the gate pack's and the base layers' over the PR's
	// files: the pack's was computed from the PR's own LAYERS.md, which the
	// PR could edit to leave a never layer out.
	files, err := shipFiles(ctx, c, owner, name, pr)
	if err != nil {
		return err
	}
	rec.Footprint = slices.Compact(slices.Sorted(slices.Values(append(slices.Clone(gp.pack.Footprint), pol.Layers.Footprint(files)...))))
	if rec.Footprint == nil {
		rec.Footprint = []string{}
	}
	rec.Pass = passRule(pol.Config)
	rec.Pass.Never = intersect(rec.Footprint, pol.Config.Autonomy.Never)
	if len(rec.Pass.Never) > 0 && proofOK == nil {
		if err := rec.Pass.neverOwners(ctx, c, owner, name, files, pol); err != nil {
			return Wrap(ExitExternal, err)
		}
		var plan PlanRecord
		ok, err := readLedgerJSON(ctx, s.Ledger, PlanLedgerPath(story), &plan)
		if err != nil {
			return Wrap(ExitExternal, err)
		}
		if !ok || (plan.Review.ApprovedBy == "" && plan.Review.Bypass == nil) {
			msg := fmt.Sprintf("ship #%d: the footprint touches never layer(s) %s, so the Engineering Manager's plan-time review is required: `aayu plan %d --draft <file> --create`, then a human runs `aayu plan %d --approve` (solo mode: `aayu plan %d --solo-bypass`)", n, strings.Join(rec.Pass.Never, ", "), story, story, story)
			if proofWhy != "" && pol.Config.Autonomy.Proof {
				msg += "; no merge on proof: " + proofWhy
			} else if proofWhy != "" { // proof off: the narrow set still holds the streak back (#680)
				msg += "; no merge on the streak: " + proofWhy
			}
			return Errorf(ExitBlocked, "%s", msg)
		}
		rec.PlanReview = &plan.Review
	}
	// reviewPass is the first approving review of the head that passes, and
	// why the others do not; ship reads it again after a device code.
	reviewPass := func() (HumanPass, []string, error) {
		reviews, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.PullRequestReview, *gh.Response, error) {
			return c.REST.PullRequests.ListReviews(ctx, owner, name, n, &o)
		})
		if err != nil {
			return HumanPass{}, nil, Wrap(ExitExternal, err)
		}
		var rs []identity.Review
		ids := map[string]int64{}
		for _, r := range reviews {
			rs = append(rs, identity.Review{User: identity.Account{Login: r.GetUser().GetLogin(), Type: r.GetUser().GetType()}, State: r.GetState(), CommitID: r.GetCommitID()})
			ids[strings.ToLower(r.GetUser().GetLogin())] = r.GetID()
		}
		var why []string
		for _, r := range identity.LatestReviews(rs) {
			if err := identity.HumanPass(r, pr.GetUser().GetLogin(), head, pol.Config.Identity.BotLogins); err != nil {
				why = append(why, err.Error())
				continue
			}
			if err := rec.Pass.allows(r.User.Login); err != nil {
				why = append(why, err.Error())
				continue
			}
			return HumanPass{Kind: "review", Login: r.User.Login, ReviewID: ids[strings.ToLower(r.User.Login)], Commit: head}, why, nil
		}
		return HumanPass{}, why, nil
	}
	hp, why, err := reviewPass()
	if err != nil {
		return err
	}
	rec.HumanPass = hp
	if rec.HumanPass.Kind == "" && proofOK != nil && !in.Bool("solo-bypass") {
		rec.Proof = proofOK
		note := fmt.Sprintf("merged on proof (Decision #593): required checks %s green, the pack verified with its floors holding, inspect at merge with review rung %s, and nothing in the narrow set",
			strings.Join(or2(required, "(none listed)"), ", "), rec.Review)
		if len(proofOK.Protected) > 0 {
			note += fmt.Sprintf("; CODEOWNERS paths %s merged with no code owner's review", strings.Join(proofOK.Protected, ", "))
		}
		rec.HumanPass = HumanPass{Kind: "proof", Login: me.Login, Commit: head, Note: note}
	} else if rec.HumanPass.Kind == "" && proofWhy != "" && pol.Config.Autonomy.Proof {
		why = append(why, "no merge on proof: "+proofWhy)
	} else if rec.HumanPass.Kind == "" && proofWhy != "" { // proof off: the narrow set still holds the streak back (#680)
		why = append(why, "no merge on the streak: "+proofWhy)
	}
	// The streak can only give an autonomous pass, so it is read (about two
	// minutes of GitHub history) only when no human pass is settled (#701).
	switch {
	case rec.HumanPass.Kind != "":
		rec.Streak = StreakState{Threshold: pol.Config.Autonomy.Streak, Layers: map[string]int{}, Never: rec.Pass.Never, Skipped: "the human pass is settled: " + rec.HumanPass.Kind}
	case in.Bool("solo-bypass"):
		rec.Streak = StreakState{Threshold: pol.Config.Autonomy.Streak, Layers: map[string]int{}, Never: rec.Pass.Never, Skipped: "the human pass is settled: solo-bypass"}
	default:
		rec.Streak = e.shipStreak(ctx, c, repo, pol, rec.Footprint, rec.Pass.Never)
	}
	// Only a different-family review (A, or B through a paid key) lets an
	// agent merge without a person; a same-family review or none refuses.
	if rec.HumanPass.Kind == "" && rec.Streak.Autonomous && !in.Bool("solo-bypass") && rec.Review != inspect.RungA && rec.Review != inspect.RungB {
		why = append(why, "the merge gate's inspect job at this head did not record a different-family review (a same-family review, or none), which never lets an agent merge without a person")
	} else if rec.HumanPass.Kind == "" && rec.Streak.Autonomous && !in.Bool("solo-bypass") && (gateState != "" && gateState != "success") {
		why = append(why, fmt.Sprintf("required check %s is %s, and only a person's pass merges past it", seats.GateCheck, gateState))
	} else if rec.HumanPass.Kind == "" && rec.Streak.Autonomous && !in.Bool("solo-bypass") && !needsPerson { // the narrow set: a person, whatever the streak
		rec.HumanPass = HumanPass{Kind: "autonomous", Login: me.Login, Commit: head,
			Note: fmt.Sprintf("every layer of footprint %s has a streak of at least %d correction-free agent merges and none is a never layer (WP5.7)", strings.Join(rec.Footprint, ", "), rec.Streak.Threshold)}
	}
	// A build-ready loop PR whose builder stopped it after the loop listed it
	// is not shipped, nor sent to the inbox: the story's status:blocked, read now, beats that list.
	// The hold is the story's label, never the ledger alone: a build branch of
	// the story (story/<n>, bug/<n>) is held whatever its state on the ledger
	// says, or when that state is missing or torn; a loop's recorded branch
	// with another name is held too.
	var st BuildState
	_, rerr := readLedgerJSON(ctx, s.Ledger, BuildLedgerPath(story), &st)
	held := func() error {
		if StoryOf("", "", pr.GetHead().GetRef()) != story && (rerr != nil || st.Loop == "" || st.Branch != pr.GetHead().GetRef()) {
			return nil
		}
		is, _, err := c.REST.Issues.Get(ctx, owner, name, story)
		if err != nil {
			return Wrap(ExitExternal, err)
		}
		switch {
		case !contains(github.IssueLabels(is), "status:blocked"):
			return nil
		case st.Loop != "":
			return Errorf(ExitBlocked, "ship #%d: story #%d is status:blocked, so the %s loop's PR waits for a person to make it Ready again", n, story, st.Loop)
		default:
			return Errorf(ExitBlocked, "ship #%d: story #%d is status:blocked, so its PR waits for a person to make it Ready again", n, story)
		}
	}
	if err := held(); err != nil {
		return err
	}
	if rerr != nil {
		return Wrap(ExitExternal, rerr)
	}

	if rec.HumanPass.Kind == "" {
		if !in.Bool("solo-bypass") {
			if len(why) == 0 {
				why = append(why, "no approving review")
			}
			if len(rec.Pass.NoOwner) > 0 {
				why = append(why, fmt.Sprintf("never-layer path(s) %s have no CODEOWNER, so no review covers them", strings.Join(rec.Pass.NoOwner, ", ")))
			}
			refusal := fmt.Sprintf("ship #%d: no human pass: %s (bot reviews and the author's own never count)", n, strings.Join(why, "; "))
			if c.DryRun() {
				return Errorf(ExitBlocked, "%s", refusal)
			}
			// Everything else holds, so the PR waits for a person in the inbox.
			who := rec.Pass.who()
			// The author's own review never counts, so an author who is the
			// only approver ships it from the inbox alone.
			review := fmt.Sprintf(", or approves the head and runs `aayu ship %d`", n)
			if strings.EqualFold(who, "@"+pr.GetUser().GetLogin()) {
				review = ""
			}
			// Solo mode: the [autonomy] approver ships it with the Solo-mode
			// bypass, CODEOWNER or not (Bug #1031).
			bypass := ""
			if a := pol.Config.Autonomy.Approver; rec.Pass.As == "approver" && soloMode(pol.Config, a, "ship") == nil {
				who = "@" + a
				review = fmt.Sprintf(", or posts a comment starting %q that names the head and runs `aayu ship %d --solo-bypass`", SoloBypassPrefix, n)
				bypass = review
			}
			body := fmt.Sprintf("%s head=%s -->\n**Ready to ship.** The merge gate is green, and its pack verifies on `%s`. `aayu ship` stopped only for the human pass: %s. "+
				"%s ships it from `aayu inbox`%s.\n", ReadyToShipMarker, head, short(head), strings.Join(why, "; "), upperFirst(who), review)
			if err := upsertOwnComment(ctx, c, repo, n, me.Login, ReadyToShipMarker, body, actions.Action{Kind: "github.comment.ready-to-ship", Story: fmt.Sprintf("#%d", story)}); err != nil {
				return Errorf(ExitBlocked, "%s; the Ready to ship comment was not posted: %v", refusal, err)
			}
			return WithNext(Errorf(ExitBlocked, "%s; commented Ready to ship, so the PR waits in the `aayu inbox` of %s%s", refusal, who, bypass), "aayu inbox ("+who+")")
		}
		hp, err := e.soloBypass(ctx, c, owner, name, n, head, me.Login, pol.Config)
		switch {
		case errors.Is(err, errNoSoloBypass) && in.String(shipBypassNote) != "":
			// The inbox's pick is the bypass: its comment is posted once
			// every check below has passed, just before the merge.
			hp = HumanPass{Kind: "solo-bypass", Login: me.Login, Commit: head, Note: soloBypassNote}
		case err != nil:
			return err
		}
		rec.HumanPass = hp
	}
	// Every check that needs no person passed: now the person approves the
	// bypass, before any write. Entering the code takes minutes, so what can
	// change on the same head (the checks, the approving review or the bypass
	// comment) is read again after it; the PR, its base, and the holds are
	// read again below.
	if in.Bool("solo-bypass") {
		if err := e.requireHuman(ctx, "ship --solo-bypass", AccessAdmin); err != nil {
			return err
		}
		now, err := gateGreen(ctx, c, owner, name, "ship", n, head, gp)
		if err != nil {
			return err
		}
		if _, _, err := requiredChecks(ctx, c, owner, name, "ship", n, pr.GetBase().GetRef(), head, now); err != nil {
			return err
		}
		switch {
		case rec.HumanPass.Kind == "review":
			hp, why, err := reviewPass()
			if err != nil {
				return err
			}
			if hp.Kind == "" {
				return Errorf(ExitBlocked, "ship #%d: @%s's approving review no longer passes after the device code: %s; nothing was merged, run ship again", n, rec.HumanPass.Login, strings.Join(or2(why, "it was dismissed"), "; "))
			}
		case rec.HumanPass.Kind == "solo-bypass" && rec.HumanPass.Comment != "":
			if rec.HumanPass, err = e.soloBypass(ctx, c, owner, name, n, head, me.Login, pol.Config); err != nil {
				return err
			}
		}
	}

	// 4. Land the gate's pack (pack.json and data, never media unless the
	// opt-in bucket is set) under the human's identity, merge, then the
	// Complete section, status:merged, the ship record, and cleanup.
	if !c.DryRun() {
		landed, err := s.Push(ctx, gp.dir, s.Bucket != nil, proof.Authority{Kind: proof.Authoritative,
			Workflow: ".github/workflows/" + MergeGateWorkflow, RunID: gp.run.GetID(), Writer: me.Login + " (aayu ship)"})
		if err != nil {
			return Wrap(ExitExternal, err)
		}
		rec.Pack = landed.Path
		rec.PackLanded = fmt.Sprintf("artifact %d of run %d of the base branch's %s (%s), checked and landed by %s through aayu ship", gp.artifact, gp.run.GetID(), MergeGateWorkflow, gp.run.GetHTMLURL(), me.Login)
	}
	// A merge with no person's pass (on proof, or an autonomous footprint)
	// waits for a person's look after merge: every one is audited, so the
	// paths that judge later PRs (internal/models, inspect, verify, proof,
	// evals) never go unseen (founder, 2026-10-18). The inbox finds the look
	// by its label, so the label goes on before the merge, and before
	// ship's last reads of the PR and the base: a merge the inbox would miss
	// never happens.
	// A person's pass merges with no look, so the label an earlier ship
	// added, then did not merge, goes first: left on a reviewed merge, the
	// inbox would offer a correction that reverts it.
	audited := rec.HumanPass.Kind == "proof" || rec.HumanPass.Kind == "autonomous"
	if audited && !c.DryRun() {
		if err := addLabel(ctx, c, repo, n, AuditLabel); err != nil {
			return Errorf(ExitExternal, "ship #%d: not merged: the %s label, by which the inbox finds the look after merge, was not added: %v", n, AuditLabel, err)
		}
	} else if !c.DryRun() && prLabelled(pr, AuditLabel) {
		if err := removeLabel(ctx, c, repo, n, AuditLabel); err != nil {
			return Errorf(ExitExternal, "ship #%d: not merged: a person passed it, but the %s label an earlier ship added was not removed, and the inbox would offer to revert it: %v", n, AuditLabel, err)
		}
	}
	// A merge on proof was judged by the base branch's policy at its tip
	// then; GitHub's merge guards only the head, so a base that moved since
	// (a tightened narrow set, say) stops it.
	// Nor may the PR itself have moved: a base retargeted, a head pushed, or
	// a PR closed since ship read it is not the PR it judged.
	now, err := pullMergeable(ctx, c, owner, name, n)
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	if now.GetState() != "open" || now.GetHead().GetSHA() != head || now.GetBase().GetRef() != pr.GetBase().GetRef() || now.GetBase().GetRepo().GetFullName() != pr.GetBase().GetRepo().GetFullName() {
		return Errorf(ExitBlocked, "ship #%d: the PR changed while ship judged it (now %s, head %s, into %s); run ship again", n, now.GetState(), short(now.GetHead().GetSHA()), now.GetBase().GetRef())
	}
	// GitHub would refuse the merge of a PR that conflicts with its base:
	// refuse it here, before the bypass below is posted.
	if now.Mergeable != nil && !now.GetMergeable() || now.GetMergeableState() == "dirty" {
		base := pr.GetBase().GetRef()
		return WithNext(Errorf(ExitBlocked, "ship #%d: it conflicts with %s, so GitHub cannot merge it; nothing was merged", n, base), fmt.Sprintf("rebase it onto %s, then aayu ship %d", base, n))
	}
	// Nor is a bypass posted while GitHub has not said whether it can merge:
	// a refused merge would leave it on a PR that never merged.
	bypassing := rec.HumanPass.Kind == "solo-bypass" && rec.HumanPass.Comment == "" && !c.DryRun()
	if bypassing && now.Mergeable == nil {
		return WithNext(Errorf(ExitBlocked, "ship #%d: GitHub has not worked out yet whether it can merge; your Solo-mode bypass was not posted, and nothing was merged", n), fmt.Sprintf("aayu ship %d again in a minute", n))
	}
	// Any pass, a person's included, was judged by the policy at that tip.
	tip, err := branchTip(ctx, c, owner, name, pr.GetBase().GetRef())
	if err != nil {
		return err
	}
	if tip != pol.Tip {
		return Errorf(ExitBlocked, "ship #%d: %s moved from %s to %s while ship judged the PR; run ship again so its current policy judges it", n, pr.GetBase().GetRef(), short(pol.Tip), short(tip))
	}
	// A stop given while ship judged the PR still holds it: the label and the mission are read again at the merge.
	if err := held(); err != nil {
		return err
	}
	if _, err := missionHold(ctx, c, s.Ledger, repo, n, story, ref); err != nil {
		return err
	}
	var posted int64 // the bypass comment ship posts below, deleted again if the merge fails
	if bypassing {
		id, url, err := createCommentURL(ctx, c, repo, n, "github.comment.solo-bypass", in.String(shipBypassNote))
		if err != nil {
			return Errorf(ExitExternal, "ship #%d: not merged: your Solo-mode bypass comment was not posted: %v", n, err)
		}
		posted, rec.HumanPass.Comment = id, url
	}
	var merged *gh.PullRequestMergeResult
	if _, err := c.Write(ctx, actions.Action{Kind: "github.pull.merge", Target: fmt.Sprintf("%s#%d", repo, n), Story: fmt.Sprintf("#%d", story),
		Request: &actions.Request{Method: http.MethodPut, Path: fmt.Sprintf("/repos/%s/pulls/%d/merge", repo, n)},
		Payload: map[string]any{"sha": head, "method": "squash", "human_pass": rec.HumanPass.Kind}},
		func(ctx context.Context) (any, error) {
			var err error
			merged, _, err = c.REST.PullRequests.Merge(ctx, owner, name, n, "", &gh.PullRequestOptions{CommitTitle: fmt.Sprintf("%s (#%d)", pr.GetTitle(), n), SHA: head, MergeMethod: "squash"})
			return map[string]any{"sha": merged.GetSHA()}, err
		}); err != nil {
		if posted != 0 {
			if _, derr := c.Write(ctx, actions.Action{Kind: "github.comment.delete", Target: fmt.Sprintf("%s#%d", repo, n),
				Request: &actions.Request{Method: http.MethodDelete, Path: fmt.Sprintf("/repos/%s/issues/comments/%d", repo, posted)}},
				func(ctx context.Context) (any, error) {
					_, err := c.REST.Issues.DeleteComment(ctx, owner, name, posted)
					return nil, err
				}); derr != nil {
				return Wrap(ExitExternal, fmt.Errorf("%w; the Solo-mode bypass comment ship posted for the merge was not deleted: %w", err, derr))
			}
		}
		return Wrap(ExitExternal, err)
	}
	if c.DryRun() {
		return e.Emit(ShipResult{ShipRecord: rec}, func(w io.Writer) error {
			_, err := fmt.Fprintf(w, "ship #%d: --dry-run: merge, Complete section, status:merged, and cleanup planned\n", n)
			return err
		})
	}
	rec.MergeCommit = merged.GetSHA()
	log, err := e.Actions()
	if err != nil {
		return err
	}
	rec.ShippedAt, rec.Actions = e.Now().UTC().Format(time.RFC3339), log.SegmentPath(e.Now())
	if fix == nil { // a gate fix completes no story: its Feature stays as it is
		if err := e.completeStory(ctx, c, repo, story, rec); err != nil {
			return err
		}
	}
	res := ShipResult{ShipRecord: rec, Record: path.Join(path.Dir(rec.Pack), "ship.json")}
	b, _ := json.MarshalIndent(rec, "", "  ")
	written := map[string][]byte{res.Record: append(b, '\n')}
	if audited {
		ab, _ := json.MarshalIndent(AuditEntry{PR: n, Story: story, Head: head, Merge: rec.MergeCommit, Footprint: rec.Footprint, Pass: rec.HumanPass.Kind, MergedAt: rec.ShippedAt}, "", "  ")
		written[AuditLedgerPath(n)] = append(ab, '\n')
	}
	if _, err := s.Ledger.Write(ctx, ledger.Commit{Message: fmt.Sprintf("proof: ship record of #%d (PR #%d)", story, n), Files: written}); err != nil {
		return Wrap(ExitExternal, err)
	}
	if ref := pr.GetHead().GetRef(); pr.GetHead().GetRepo().GetFullName() == repo || pr.GetHead().GetRepo() == nil {
		if _, err := c.Write(ctx, actions.Action{Kind: "github.ref.delete", Target: repo + "@" + ref,
			Request: &actions.Request{Method: http.MethodDelete, Path: fmt.Sprintf("/repos/%s/git/refs/heads/%s", repo, ref)}},
			func(ctx context.Context) (any, error) {
				_, err := c.REST.Git.DeleteRef(ctx, owner, name, "refs/heads/"+ref)
				if github.NotFound(err) || (err != nil && strings.Contains(err.Error(), "Reference does not exist")) {
					err = nil // already gone: the repository deletes head branches on merge
				}
				return nil, err
			}); err == nil {
			res.Branch = ref
		}
	}
	if _, err := e.releaseLease(ctx, c, repo, story, fmt.Sprintf("shipped in #%d", n)); err != nil {
		fmt.Fprintf(e.Err, "aayu: warning: the build lease on #%d was not released: %v\n", story, err)
	}
	if root := config.RepoRoot(e.Dir); root != "" {
		if wt := BuildWorktree(root, story); dirExists(wt) {
			_, _ = run2(ctx, root, "git", "worktree", "remove", "--force", wt)
			_, _ = run2(ctx, root, "git", "branch", "-D", pr.GetHead().GetRef())
		}
	}
	checkpointed = true
	if err := e.checkpoint(tok); err != nil {
		return Wrap(ExitExternal, fmt.Errorf("merged, but the actions segment did not reach the ledger: %w", err))
	}
	// Only now that the merge, the ship record, and the actions segment stand
	// does the Kanha of the story's Feature hear of the merge (Story #1007): a
	// message that fails changes none of them, nor ship's exit code.
	if res.Next = e.tellMerged(ctx, c, s.Ledger, repo, story, n); res.Next != "" {
		if err := e.checkpoint(tok); err != nil { // the message's own record
			fmt.Fprintf(e.Err, "aayu: warning: the record of the message to the Kanha did not reach the ledger; it stays in the local actions log: %v\n", err)
		}
	}
	if err := e.Emit(res, func(w io.Writer) error {
		if fix != nil {
			fmt.Fprintf(w, "ship #%d: merged as %s; the gate fix of %s of #%d for %s\n", n, short(rec.MergeCommit), fix.Milestone, story, fix.Check)
		} else {
			fmt.Fprintf(w, "ship #%d: merged as %s; story #%d is status:merged with its Complete section\n", n, short(rec.MergeCommit), story)
		}
		fmt.Fprintf(w, "  pack %s (sha256 %s): %s\n", rec.Pack, short(rec.PackSHA256), rec.PackLanded)
		fmt.Fprintf(w, "  human pass: %s by %s%s\n", rec.HumanPass.Kind, rec.HumanPass.Login, map[bool]string{true: " — " + rec.HumanPass.Note, false: ""}[rec.HumanPass.Note != ""])
		fmt.Fprintf(w, "  review rung: %s (from the merge gate's inspect log)\n", or(rec.Review, "none recorded"))
		fmt.Fprintf(w, "  ship record %s; actions segment %s\n", res.Record, rec.Actions)
		if res.Next != "" {
			fmt.Fprintf(w, "next: %s\n", res.Next)
		}
		return nil
	}); err != nil {
		return err
	}
	// Every ship lands an authoritative pack (the merge gate's), so the first
	// one is the first proven ship.
	e.milestone("first-proven-ship", true, "Your first proven ship!", fmt.Sprintf("Story #%d merged with an authoritative proof pack and a recorded human pass.", story))
	return nil
}

// tellMerged tells the Kanha of story n's Feature that PR pr merged it, so
// the Kanha starts the next story now (Story #1007), and returns ship's
// `next:` line: "" when the story is in no started mission. It never fails
// ship: the safety tick wakes a Kanha the message did not reach.
func (e *Env) tellMerged(ctx context.Context, c *github.Client, w *ledger.Writer, repo string, n, pr int) string {
	var f int // the Feature, once tellKanha has found the Kanha
	note, err := e.tellKanha(ctx, c, w, repo, n, func(ff int) string {
		f = ff
		return fmt.Sprintf("merged #%d (PR #%d): run aayu mission next %d", n, pr, f)
	})
	switch {
	case err != nil && f == 0:
		return fmt.Sprintf("the Kanha of #%d's Feature was not reached: %s; the safety tick wakes it", n, oneLineErr(err))
	case err != nil && note != "": // the send failed, and its event is recorded
		return fmt.Sprintf("the Kanha of #%d was not reached: %s; the safety tick wakes it", f, strings.TrimPrefix(oneLineErr(err), fmt.Sprintf("the Kanha of #%d: ", f)))
	case err != nil: // the event is recorded after the send
		return fmt.Sprintf("the Kanha of #%d may not have been told: its event was not recorded: %s; the safety tick wakes it", f, oneLineErr(err))
	case strings.HasSuffix(note, "(sent)"):
		return fmt.Sprintf("told the Kanha of #%d", f)
	}
	return note // a local Kanha takes it at its next check
}

func dirExists(p string) bool { st, err := os.Stat(p); return err == nil && st.IsDir() }

// gatePackRun is the merge gate's pack for a PR head, unpacked into dir.
type gatePackRun struct {
	run      *gh.WorkflowRun
	artifact int64
	uploaded time.Time // when the artifact was made
	pack     *proof.Pack
	sha256   string // of the artifact's pack.json
	dir, tmp string
}

// gatePack finds the pack the merge gate built for the PR's head (S-4.2b):
// the aayudha-pack artifact of the newest successful run of the base branch's
// aayudha-merge-gate.yml (event pull_request_target, so the workflow file is
// the base branch's and the pull request cannot have edited it) on exactly
// that head. The pack must be for the story, the PR, and the head, and every
// artifact's bytes must match its hash in pack.json. Nothing on the ledger,
// no preview, and no artifact uploaded by another workflow or by the pull
// request's own copy of the gate counts; a refusal lists every run it passed
// over and why.
func (e *Env) gatePack(ctx context.Context, c *github.Client, owner, name, verb string, pr *gh.PullRequest, story int) (gatePackRun, error) {
	n, head, path := pr.GetNumber(), pr.GetHead().GetSHA(), ".github/workflows/"+MergeGateWorkflow
	runs, _, err := c.REST.Actions.ListWorkflowRunsByFileName(ctx, owner, name, MergeGateWorkflow, &gh.ListWorkflowRunsOptions{Branch: pr.GetHead().GetRef(), ListOptions: gh.ListOptions{PerPage: 50}})
	if err != nil {
		return gatePackRun{}, Wrap(ExitExternal, err)
	}
	slices.SortFunc(runs.WorkflowRuns, func(a, b *gh.WorkflowRun) int { return cmp.Compare(b.GetID(), a.GetID()) })
	// One rule for the checks and the pack: the pack comes only from a run
	// whose suite gateSuites accepts as this pull request's own gate. After a
	// merge, a run started by a later event uses the base branch that now
	// holds the pull request's own code, so its pack is not independent of it
	// (Bug #1159).
	own, err := gateSuites(ctx, c, owner, name, head, n)
	if err != nil {
		return gatePackRun{}, Wrap(ExitExternal, err)
	}
	var why []string
	refuse := func(r *gh.WorkflowRun, format string, args ...any) {
		why = append(why, fmt.Sprintf("run %d: ", r.GetID())+fmt.Sprintf(format, args...))
	}
	for _, r := range runs.WorkflowRuns {
		switch {
		case r.GetPath() != path:
			refuse(r, "a run of %s, not %s", r.GetPath(), path)
		case r.GetEvent() != "pull_request_target":
			refuse(r, "a %s run, so the workflow file came from the pull request, not the base branch (only pull_request_target runs count)", r.GetEvent())
		case r.GetHeadSHA() != head:
			refuse(r, "proves %s, not the head %s", short(r.GetHeadSHA()), short(head))
		case r.GetStatus() != "completed" || r.GetConclusion() != "success":
			refuse(r, "is %s on the head", or(r.GetConclusion(), r.GetStatus()))
		case !own[r.GetCheckSuiteID()]:
			refuse(r, "is not this pull request's own gate run (it lists another pull request, or it started outside the pull request's open window), so aayudha/merge-gate is missing on %s", short(head))
		default:
			g, err := e.unpackGateRun(ctx, c, owner, name, r, n, story, head)
			if err == nil {
				return g, nil
			}
			if CodeOf(err) != ExitBlocked {
				return gatePackRun{}, err
			}
			refuse(r, "%v", err)
		}
	}
	if len(why) == 0 {
		why = append(why, "no run of "+path+" for this pull request's branch")
	}
	if len(why) > 6 {
		why = append(why[:6], fmt.Sprintf("%d older run(s)", len(why)-6))
	}
	return gatePackRun{}, Errorf(ExitBlocked, "%s #%d: no merge gate pack for the head %s: %s", verb, n, short(head), strings.Join(why, "; "))
}

// gateGreen reads every check run on head and requires the merge gate's
// checks to be green there: the newest run of each in the gate's own suites
// (gateSuites), so an older green run never stands in for a newer red one, nor
// a check of the same name from another workflow or app for the gate's. The
// newest of each in the suite of the run whose pack lands must be green too,
// and the pack must be made while that suite's aayudha/merge-gate check ran:
// that job uploads it, and a run's attempts never overlap. So the pack and the
// checks it lands with come from one attempt of one run (Bug #1162).
func gateGreen(ctx context.Context, c *github.Client, owner, name, verb string, n int, head string, pack gatePackRun) ([]*gh.CheckRun, error) {
	runs, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.CheckRun, *gh.Response, error) { // every page
		r, resp, err := c.REST.Checks.ListCheckRunsForRef(ctx, owner, name, head, &gh.ListCheckRunsOptions{ListOptions: o})
		return r.GetCheckRuns(), resp, err
	})
	if err != nil {
		return nil, Wrap(ExitExternal, err)
	}
	suites, err := gateSuites(ctx, c, owner, name, head, n)
	if err != nil {
		return nil, Wrap(ExitExternal, err)
	}
	newest := func(want string, suites map[int64]bool) (*gh.CheckRun, string) {
		for _, r := range gateCheckRuns(runs, suites) {
			if r.GetName() == want { // the API lists the newest run of a name first
				if r.GetStatus() == "completed" {
					return r, r.GetConclusion()
				}
				return r, r.GetStatus()
			}
		}
		return nil, "missing"
	}
	for _, want := range gateChecks {
		if _, concl := newest(want, suites); concl != "success" {
			return nil, Errorf(ExitBlocked, "%s #%d: %s is %s on %s; the merge gate must be green", verb, n, want, concl, short(head))
		}
	}
	own, id := map[int64]bool{pack.run.GetCheckSuiteID(): true}, pack.run.GetID()
	for _, want := range gateChecks {
		if _, concl := newest(want, own); concl != "success" {
			return nil, Errorf(ExitBlocked, "%s #%d: %s is %s in run %d, whose pack lands, on %s; the merge gate must be green", verb, n, want, concl, id, short(head))
		}
	}
	gate, _ := newest("aayudha/merge-gate", own)
	stamp := func(t time.Time) string { return t.UTC().Format(time.RFC3339) }
	if at, from, to := pack.uploaded, gate.GetStartedAt().Time, gate.GetCompletedAt().Time; at.IsZero() || at.Before(from) || at.After(to) {
		made := "was made at " + stamp(at)
		if at.IsZero() {
			made = "has no creation time"
		}
		return nil, Errorf(ExitBlocked, "%s #%d: artifact %d of run %d %s, outside the attempt whose aayudha/merge-gate check ran %s to %s, so the pack and the checks come from two attempts (push a commit or re-run the gate for a fresh one)",
			verb, n, pack.artifact, id, made, stamp(from), stamp(to))
	}
	return runs, nil
}

// verifyGatePack checks the merge gate's pack against the head and the
// story's Ready snapshot on the ledger (a listed gate fix's against its
// Feature's approved record), and that its floors hold.
func verifyGatePack(ctx context.Context, s *proof.Store, verb string, n, story int, head string, gp gatePackRun, fix *GateFix) error {
	t, err := s.Ledger.Read(ctx)
	if err != nil {
		return ledgerErr(err)
	}
	exp := proof.Expect{HeadSHA: head}
	if b, err := t.File(ctx, ready.LedgerPath(story)); err == nil {
		snap, err := ready.ParseSnapshot(string(b))
		if err != nil {
			t.Close()
			return err
		}
		exp.CompletionGoalHash, exp.Criteria = snap.CompletionGoalHash, snap.Numbers()
	}
	t.Close()
	if fix != nil { // a listed gate fix rests on the approved record, not a Ready snapshot (Decision #974)
		exp.CompletionGoalHash, exp.Criteria = fix.Approval.SHA256, nil
	}
	rep := proof.VerifyLocal(gp.dir, gp.pack, exp)
	if exp.CompletionGoalHash == "" {
		rep.Problems = append(rep.Problems, "no Ready snapshot on the ledger")
	}
	if len(rep.Problems) > 0 {
		return Errorf(ExitBlocked, "%s #%d: the merge gate's pack does not verify: %s", verb, n, strings.Join(rep.Problems, "; "))
	}
	if !gp.pack.FloorsHold() {
		return Errorf(ExitBlocked, "%s #%d: floors do not hold in the merge gate's pack: %s (a floor-waiver Decision under #%d waives one, WP4.9)", verb, n, failing(gp.pack.Checks), story)
	}
	return nil
}

// unpackGateRun downloads a merge gate run's aayudha-pack artifact and checks
// the pack in it: for this story, PR, and head, with every artifact's bytes
// matching pack.json (proof.LoadLocal).
func (e *Env) unpackGateRun(ctx context.Context, c *github.Client, owner, name string, run *gh.WorkflowRun, n, story int, head string) (gatePackRun, error) {
	arts, _, err := c.REST.Actions.ListWorkflowRunArtifacts(ctx, owner, name, run.GetID(), nil)
	if err != nil {
		return gatePackRun{}, Wrap(ExitExternal, err)
	}
	// A re-run of the run's failed jobs uploads another pack to the run. The
	// newest (the highest ID) is the latest attempt's, whose checks gateGreen
	// reads; an older attempt's pack never stands in for it.
	var art *gh.Artifact
	for _, a := range arts.Artifacts {
		if a.GetName() == "aayudha-pack" && (art == nil || a.GetID() > art.GetID()) {
			art = a
		}
	}
	switch {
	case art == nil:
		return gatePackRun{}, Errorf(ExitBlocked, "it kept no aayudha-pack artifact")
	case art.GetExpired():
		return gatePackRun{}, Errorf(ExitBlocked, "its aayudha-pack artifact %d expired (push a commit or re-run the gate for a fresh one)", art.GetID())
	}
	u, _, err := c.REST.Actions.DownloadArtifact(ctx, owner, name, art.GetID(), 3)
	if err != nil {
		return gatePackRun{}, Wrap(ExitExternal, err)
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	resp, err := e.HTTP.Do(req)
	if err != nil {
		return gatePackRun{}, Wrap(ExitExternal, err)
	}
	defer func() { _ = resp.Body.Close() }()
	zipped, err := io.ReadAll(io.LimitReader(resp.Body, 512<<20))
	if err != nil || resp.StatusCode != http.StatusOK {
		return gatePackRun{}, Errorf(ExitExternal, "download artifact %d: HTTP %d %v", art.GetID(), resp.StatusCode, err)
	}
	g := gatePackRun{run: run, artifact: art.GetID(), uploaded: art.GetCreatedAt().Time}
	if g.tmp, err = os.MkdirTemp("", "aayudha-gate-pack-"); err != nil {
		return g, err
	}
	fail := func(err error) (gatePackRun, error) { _ = os.RemoveAll(g.tmp); return gatePackRun{}, err }
	if err := unzip(zipped, g.tmp); err != nil {
		return fail(Errorf(ExitBlocked, "its aayudha-pack artifact %d: %v", art.GetID(), err))
	}
	g.dir = filepath.Join(g.tmp, fmt.Sprint(story))
	raw, err := os.ReadFile(filepath.Join(g.dir, proof.FileName))
	if err != nil {
		return fail(Errorf(ExitBlocked, "its aayudha-pack artifact %d holds no pack for #%d", art.GetID(), story))
	}
	if g.pack, err = proof.LoadLocal(g.dir); err != nil {
		return fail(Errorf(ExitBlocked, "its aayudha-pack artifact %d: %v", art.GetID(), err))
	}
	if p := g.pack; p.PRHeadSHA != head || p.Story != story || p.PR != n {
		return fail(Errorf(ExitBlocked, "its pack is for #%d, PR %d, at %s, not #%d, PR %d, at %s", p.Story, p.PR, short(p.PRHeadSHA), story, n, short(head)))
	}
	sum := sha256.Sum256(raw)
	g.sha256 = hex.EncodeToString(sum[:])
	return g, nil
}

func unzip(b []byte, dir string) error {
	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		return err
	}
	for _, f := range zr.File {
		rel := filepath.ToSlash(f.Name)
		if f.FileInfo().IsDir() {
			continue
		}
		if err := proof.CleanPath(rel); err != nil {
			return fmt.Errorf("artifact entry %q: %w", f.Name, err)
		}
		dst := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		data, err := io.ReadAll(io.LimitReader(rc, 256<<20))
		_ = rc.Close()
		if err != nil {
			return err
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

var sectionHead = regexp.MustCompile(`^#{2,3}\s+(.+?)\s*#*\s*$`)

var soloComment = regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(SoloBypassPrefix))

// soloBypass is the closest enforceable human pass while one human runs the
// repository without a bot App (Appendix B D2): GitHub refuses self-approval,
// so the merging human records the bypass in a PR comment starting
// "Solo-mode bypass:" and passes --solo-bypass; ship checks both and records
// the comment in the ship record, the Complete section, and the actions log,
// where `audit` (WP7.6) reconciles it.
func (e *Env) soloBypass(ctx context.Context, c *github.Client, owner, name string, n int, head, login string, cfg *config.Config) (HumanPass, error) {
	url, err := soloModeComment(ctx, c, owner, name, n, login, cfg, "ship --solo-bypass", "a PR comment", head)
	if err != nil {
		return HumanPass{}, err
	}
	return HumanPass{Kind: "solo-bypass", Login: login, Comment: url, Commit: head, Note: soloBypassNote}, nil
}

const soloBypassNote = "solo mode: GitHub refuses self-approval and no bot App exists, so no second person could review"

// createCommentURL posts body on PR n and returns the comment's URL, which
// the ship record cites.
func createCommentURL(ctx context.Context, c *github.Client, repo string, n int, kind, body string) (int64, string, error) {
	owner, name, _ := github.SplitRepo(repo)
	var id int64
	var url string
	_, err := c.Write(ctx, actions.Action{Kind: kind, Target: fmt.Sprintf("%s#%d", repo, n),
		Request: &actions.Request{Method: http.MethodPost, Path: fmt.Sprintf("/repos/%s/issues/%d/comments", repo, n)}},
		func(ctx context.Context) (any, error) {
			cm, _, err := c.REST.Issues.CreateComment(ctx, owner, name, n, &gh.IssueComment{Body: gh.Ptr(body)})
			id, url = cm.GetID(), cm.GetHTMLURL()
			return id, err
		})
	return id, url, err
}

// errNoSoloBypass is a ship refused for want of a bypass that names the head.
var errNoSoloBypass = errors.New("post a Solo-mode bypass")

// namesHead reports whether body names head: a hex word of at least seven
// characters that head starts with.
func namesHead(body, head string) bool {
	for _, w := range hexWord.FindAllString(body, -1) {
		if strings.HasPrefix(head, strings.ToLower(w)) {
			return true
		}
	}
	return false
}

var hexWord = regexp.MustCompile(`\b[0-9a-fA-F]{7,40}\b`)

// soloModeComment checks that solo mode holds (no bot identity, no second
// paired human) and finds login's comment on issue or PR n starting
// "Solo-mode bypass:"; it returns the comment's URL. what names the command
// in refusals and where the comment belongs. With a head, only a comment
// that names that head counts: a bypass is for the commit it was written
// for, never a later push (#799).
func soloModeComment(ctx context.Context, c *github.Client, owner, name string, n int, login string, cfg *config.Config, cmd, where, head string) (string, error) {
	if err := soloMode(cfg, login, cmd); err != nil {
		return "", err
	}
	comments, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.IssueComment, *gh.Response, error) {
		return c.REST.Issues.ListComments(ctx, owner, name, n, &gh.IssueListCommentsOptions{ListOptions: o})
	})
	if err != nil {
		return "", Wrap(ExitExternal, err)
	}
	i := slices.IndexFunc(comments, func(cm *gh.IssueComment) bool {
		return strings.EqualFold(cm.GetUser().GetLogin(), login) && soloComment.MatchString(cm.GetBody()) && (head == "" || namesHead(cm.GetBody(), head))
	})
	if i < 0 && head != "" {
		return "", Errorf(ExitBlocked, "%s: %w for the head %s first: %s starting %q from your own account that names %s and says why", cmd, errNoSoloBypass, short(head), where, SoloBypassPrefix, short(head))
	}
	if i < 0 {
		return "", Errorf(ExitBlocked, "%s: post %s starting %q that says why, from your own account, first", cmd, where, SoloBypassPrefix)
	}
	return comments[i].GetHTMLURL(), nil
}

// shipExpectHead pins ship to the head a person chose in the inbox; it is not
// a flag, so only the inbox sets it.
const shipExpectHead = "expect-head"

// shipBypassNote is the Solo-mode bypass comment the inbox asks ship to post
// for the person, once ship's checks pass on the head; it is not a flag.
const shipBypassNote = "bypass-note"

// headMoved refuses a ship whose PR was pushed after the person chose it.
func headMoved(n int, chose, now string) error {
	return WithNext(Errorf(ExitBlocked, "ship #%d: the head moved to %s after you chose to ship %s; nothing was merged", n, short(now), short(chose)), "aayu inbox")
}

// upsertOwnComment replaces login's own comment on issue or PR n that carries
// marker, or posts one: a look-alike someone else wrote is never edited, so
// the comment stays login's word.
func upsertOwnComment(ctx context.Context, c *github.Client, repo string, n int, login, marker, body string, a actions.Action) error {
	owner, name, _ := github.SplitRepo(repo)
	comments, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.IssueComment, *gh.Response, error) {
		return c.REST.Issues.ListComments(ctx, owner, name, n, &gh.IssueListCommentsOptions{ListOptions: o})
	})
	if err != nil {
		return err
	}
	a.Target = fmt.Sprintf("%s#%d", repo, n)
	for _, cm := range comments {
		if !strings.EqualFold(cm.GetUser().GetLogin(), login) || !strings.Contains(cm.GetBody(), marker) {
			continue
		}
		if cm.GetBody() == body {
			return nil
		}
		a.Request = &actions.Request{Method: http.MethodPatch, Path: fmt.Sprintf("/repos/%s/issues/comments/%d", repo, cm.GetID())}
		_, err := c.Write(ctx, a, func(ctx context.Context) (any, error) {
			_, _, err := c.REST.Issues.EditComment(ctx, owner, name, cm.GetID(), &gh.IssueComment{Body: gh.Ptr(body)})
			return cm.GetID(), err
		})
		return err
	}
	a.Request = &actions.Request{Method: http.MethodPost, Path: fmt.Sprintf("/repos/%s/issues/%d/comments", repo, n)}
	_, err = c.Write(ctx, a, func(ctx context.Context) (any, error) {
		cm, _, err := c.REST.Issues.CreateComment(ctx, owner, name, n, &gh.IssueComment{Body: gh.Ptr(body)})
		return cm.GetID(), err
	})
	return err
}

// soloMode checks that solo mode holds for login: no bot identity and no
// second paired human, so no second person could review.
func soloMode(cfg *config.Config, login, cmd string) error {
	if identity.Configured(cfg.Identity) != "" {
		return Errorf(ExitBlocked, "%s: this repository has a bot identity, so the agent can author and a human can approve; get an approving review", cmd)
	}
	for _, s := range cfg.Seats {
		if s.Mode == "paired" && s.Human != "" && !strings.EqualFold(s.Human, login) {
			return Errorf(ExitBlocked, "%s: a second human (%s) holds a seat; ask them for an approving review", cmd, s.Human)
		}
	}
	return nil
}

// completeStory writes the story's Complete section (Outcome) and moves it to
// status:merged. The label moves first, so the Ready guard (which acts only on
// status:ready) leaves the edit alone.
func (e *Env) completeStory(ctx context.Context, c *github.Client, repo string, story int, rec ShipRecord) error {
	owner, name, _ := github.SplitRepo(repo)
	is, _, err := c.REST.Issues.Get(ctx, owner, name, story)
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	target := fmt.Sprintf("%s#%d", repo, story)
	for _, lb := range github.IssueLabels(is) {
		if strings.HasPrefix(lb, "status:") {
			if _, err := c.Write(ctx, actions.Action{Kind: "github.issue.labels.remove", Target: target,
				Request: &actions.Request{Method: http.MethodDelete, Path: fmt.Sprintf("/repos/%s/issues/%d/labels/%s", repo, story, lb)}},
				func(ctx context.Context) (any, error) {
					_, err := c.REST.Issues.RemoveLabelForIssue(ctx, owner, name, story, lb)
					if github.NotFound(err) {
						err = nil
					}
					return nil, err
				}); err != nil {
				return Wrap(ExitExternal, err)
			}
		}
	}
	if _, err := c.Write(ctx, actions.Action{Kind: "github.issue.labels.add", Target: target,
		Request: &actions.Request{Method: http.MethodPost, Path: fmt.Sprintf("/repos/%s/issues/%d/labels", repo, story)}, Payload: map[string]any{"labels": []string{"status:merged"}}},
		func(ctx context.Context) (any, error) {
			_, _, err := c.REST.Issues.AddLabelsToIssue(ctx, owner, name, story, []string{"status:merged"})
			return nil, err
		}); err != nil {
		return Wrap(ExitExternal, err)
	}
	var out strings.Builder
	fmt.Fprintf(&out, "Merged in #%d as %s on %s.\n\n", rec.PR, rec.MergeCommit, rec.ShippedAt[:10])
	fmt.Fprintf(&out, "- Authoritative pack: `%s` on `ledger` (head `%s`)", rec.Pack, short(rec.Head))
	if rec.PackLanded != "" {
		fmt.Fprintf(&out, ", from %s", rec.PackLanded)
	}
	out.WriteString("\n")
	switch rec.HumanPass.Kind {
	case "review":
		fmt.Fprintf(&out, "- Human pass: approving review %d by @%s on `%s` (%s)\n", rec.HumanPass.ReviewID, rec.HumanPass.Login, short(rec.HumanPass.Commit), rec.Pass.As)
	case "autonomous":
		fmt.Fprintf(&out, "- Human pass: not needed, autonomous footprint: %s\n", rec.HumanPass.Note)
	case "proof":
		fmt.Fprintf(&out, "- Human pass: not needed, %s\n", rec.HumanPass.Note)
	default:
		fmt.Fprintf(&out, "- Human pass: none possible; %s merged under a recorded solo-mode bypass (%s): %s\n", rec.HumanPass.Login, rec.HumanPass.Comment, rec.HumanPass.Note)
	}
	if r := rec.PlanReview; r != nil && r.Bypass != nil {
		fmt.Fprintf(&out, "- Plan-time review: none; %s recorded a solo-mode plan-review bypass for layers %s (%s): %s\n", r.Bypass.RecordedBy, strings.Join(r.Layers, ", "), r.Bypass.Comment, r.Bypass.Note)
	} else if r != nil && r.ApprovedBy != "" {
		fmt.Fprintf(&out, "- Plan-time review: approved by @%s (%s) for layers %s\n", r.ApprovedBy, r.As, strings.Join(r.Layers, ", "))
	}
	for _, w := range rec.Waivers {
		fmt.Fprintf(&out, "- Floor waived: %s\n", w)
	}
	fmt.Fprintf(&out, "- Ship record: `%s`; actions segment: `%s`\n", path.Join(path.Dir(rec.Pack), "ship.json"), rec.Actions)
	body := setSection(is.GetBody(), "Outcome", strings.TrimSpace(out.String()))
	if _, err := c.Write(ctx, actions.Action{Kind: "github.issue.complete", Target: target,
		Request: &actions.Request{Method: http.MethodPatch, Path: fmt.Sprintf("/repos/%s/issues/%d", repo, story)}, Payload: map[string]any{"section": "Outcome", "pr": rec.PR}},
		func(ctx context.Context) (any, error) {
			_, _, err := c.REST.Issues.Update(ctx, owner, name, story, gh.UpdateIssueRequest{Body: &body})
			return nil, err
		}); lockRefused(err) || errors.As(err, new(*lockUnread)) {
		// A running mission locks the story's body (Story #1013): the Outcome goes in a comment, and the body stays as locked.
		// When the locks cannot be read, the merge already happened: the comment keeps the ship record writable.
		return wrapExternal(postComment(ctx, c, repo, story, "github.comment.outcome", "### Outcome\n\n"+strings.TrimSpace(out.String())+"\n"))
	} else if err != nil {
		return Wrap(ExitExternal, err)
	}
	return nil
}

// setSection replaces a ### section of an issue body, or appends it.
func setSection(body, label, value string) string {
	labels, _ := workitems.Sections(body)
	if !slices.Contains(labels, label) {
		return strings.TrimRight(body, "\n") + "\n\n### " + label + "\n\n" + value + "\n"
	}
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	var out []string
	skip := false
	for _, line := range lines {
		if m := sectionHead.FindStringSubmatch(line); m != nil {
			if skip {
				skip = false
			}
			if m[1] == label {
				out = append(out, line, "", value, "")
				skip = true
				continue
			}
		}
		if !skip {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

// or2 is xs, or a one-element list holding def when xs is empty.
func or2(xs []string, def string) []string {
	if len(xs) == 0 {
		return []string{def}
	}
	return xs
}
