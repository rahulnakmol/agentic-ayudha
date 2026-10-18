package cli

import (
	"archive/zip"
	"bytes"
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	if in.Bool("solo-bypass") {
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
	rec := ShipRecord{Repo: repo, Story: story, PR: n, Head: head, ShippedBy: me.Login}

	// 1. The pack the merge gate built for this head, from the artifact of a
	// run of the base branch's workflow (S-4.2b): never a pack on the ledger,
	// a preview, or an artifact another workflow uploaded.
	gp, err := e.gatePack(ctx, c, owner, name, pr, story)
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(gp.tmp) }()
	rec.GateRun, rec.Artifact, rec.PackSHA256 = gp.run.GetID(), gp.artifact, gp.sha256

	// 2. The merge gate's checks are green on the head.
	runs, _, err := c.REST.Checks.ListCheckRunsForRef(ctx, owner, name, head, &gh.ListCheckRunsOptions{ListOptions: gh.ListOptions{PerPage: 100}})
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	for _, want := range gateChecks {
		concl := "missing"
		for _, r := range runs.CheckRuns {
			if r.GetName() == want {
				concl = r.GetStatus()
				if r.GetStatus() == "completed" {
					concl = r.GetConclusion()
				}
				break // the API lists the newest run of a name first
			}
		}
		if concl != "success" {
			return Errorf(ExitBlocked, "ship #%d: %s is %s on %s; the merge gate must be green", n, want, concl, short(head))
		}
	}

	// The pack verifies against the head and the Ready snapshot, and its floors hold.
	s, err := e.proofStore(ctx, c, tok, "human", "")
	if err != nil {
		return err
	}
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
	rep := proof.VerifyLocal(gp.dir, gp.pack, exp)
	if exp.CompletionGoalHash == "" {
		rep.Problems = append(rep.Problems, "no Ready snapshot on the ledger")
	}
	if len(rep.Problems) > 0 {
		return Errorf(ExitBlocked, "ship #%d: the merge gate's pack does not verify: %s", n, strings.Join(rep.Problems, "; "))
	}
	if !gp.pack.FloorsHold() {
		return Errorf(ExitBlocked, "ship #%d: floors do not hold in the merge gate's pack: %s (a floor-waiver Decision under #%d waives one, WP4.9)", n, failing(gp.pack.Checks), story)
	}
	for _, ck := range gp.pack.Checks {
		if ck.Waiver != "" {
			rec.Waivers = append(rec.Waivers, ck.Floor+" by "+ck.Waiver)
		}
	}
	// Every required check of the base branch is green too, whoever passes it.
	required, err := requiredChecks(ctx, c, owner, name, n, pr.GetBase().GetRef(), head, runs.CheckRuns)
	if err != nil {
		return err
	}
	// The review ladder's rung at the head, from the merge gate's own inspect log.
	rec.Review = autonomy.GateRung(ctx, c, owner, name, n, head)
	// Merge on proof (Decision #593): with it, no human pass is needed.
	proofOK, proofWhy, err := e.proofPass(ctx, c, owner, name, pr, rec.Review, &l.Config)
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
	root := config.RepoRoot(e.Dir)
	rec.Footprint = gp.pack.Footprint
	if rec.Footprint == nil {
		rec.Footprint = []string{}
	}
	rec.Pass = passRule(&l.Config)
	rec.Pass.Never = intersect(rec.Footprint, l.Config.Autonomy.Never)
	if len(rec.Pass.Never) > 0 && proofOK == nil {
		if err := rec.Pass.neverOwners(ctx, c, owner, name, pr, root); err != nil {
			return Wrap(ExitExternal, err)
		}
		var plan PlanRecord
		ok, err := readLedgerJSON(ctx, s.Ledger, PlanLedgerPath(story), &plan)
		if err != nil {
			return Wrap(ExitExternal, err)
		}
		if !ok || (plan.Review.ApprovedBy == "" && plan.Review.Bypass == nil) {
			msg := fmt.Sprintf("ship #%d: the footprint touches never layer(s) %s, so the Engineering Manager's plan-time review is required: `aayu plan %d --draft <file> --create`, then a human runs `aayu plan %d --approve` (solo mode: `aayu plan %d --solo-bypass`)", n, strings.Join(rec.Pass.Never, ", "), story, story, story)
			if l.Config.Autonomy.Proof {
				msg += "; no merge on proof: " + proofWhy
			}
			return Errorf(ExitBlocked, "%s", msg)
		}
		rec.PlanReview = &plan.Review
	}
	rec.Streak = e.shipStreak(ctx, c, repo, root, &l.Config, rec.Footprint, rec.Pass.Never)
	reviews, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.PullRequestReview, *gh.Response, error) {
		return c.REST.PullRequests.ListReviews(ctx, owner, name, n, &o)
	})
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	var rs []identity.Review
	ids := map[string]int64{}
	for _, r := range reviews {
		rs = append(rs, identity.Review{User: identity.Account{Login: r.GetUser().GetLogin(), Type: r.GetUser().GetType()}, State: r.GetState(), CommitID: r.GetCommitID()})
		ids[strings.ToLower(r.GetUser().GetLogin())] = r.GetID()
	}
	var why []string
	for _, r := range identity.LatestReviews(rs) {
		if err := identity.HumanPass(r, pr.GetUser().GetLogin(), head, l.Config.Identity.BotLogins); err != nil {
			why = append(why, err.Error())
			continue
		}
		if err := rec.Pass.allows(r.User.Login); err != nil {
			why = append(why, err.Error())
			continue
		}
		rec.HumanPass = HumanPass{Kind: "review", Login: r.User.Login, ReviewID: ids[strings.ToLower(r.User.Login)], Commit: head}
		break
	}
	if rec.HumanPass.Kind == "" && proofOK != nil && !in.Bool("solo-bypass") {
		rec.Proof = proofOK
		note := fmt.Sprintf("merged on proof (Decision #593): required checks %s green, the pack verified with its floors holding, inspect at merge with review rung %s, and nothing in the narrow set",
			strings.Join(or2(required, "(none listed)"), ", "), rec.Review)
		if len(proofOK.Protected) > 0 {
			note += fmt.Sprintf("; CODEOWNERS paths %s merged with no code owner's review", strings.Join(proofOK.Protected, ", "))
		}
		rec.HumanPass = HumanPass{Kind: "proof", Login: me.Login, Commit: head, Note: note}
	} else if rec.HumanPass.Kind == "" && proofWhy != "" && l.Config.Autonomy.Proof {
		why = append(why, "no merge on proof: "+proofWhy)
	}
	// Only a different-family review (A, or B through a paid key) lets an
	// agent merge without a person; a same-family review or none refuses.
	if rec.HumanPass.Kind == "" && rec.Streak.Autonomous && !in.Bool("solo-bypass") && rec.Review != inspect.RungA && rec.Review != inspect.RungB {
		why = append(why, "the merge gate's inspect job at this head did not record a different-family review (a same-family review, or none), which never lets an agent merge without a person")
	} else if rec.HumanPass.Kind == "" && rec.Streak.Autonomous && !in.Bool("solo-bypass") {
		rec.HumanPass = HumanPass{Kind: "autonomous", Login: me.Login, Commit: head,
			Note: fmt.Sprintf("every layer of footprint %s has a streak of at least %d correction-free agent merges and none is a never layer (WP5.7)", strings.Join(rec.Footprint, ", "), rec.Streak.Threshold)}
	}
	if rec.HumanPass.Kind == "" {
		if !in.Bool("solo-bypass") {
			if len(why) == 0 {
				why = append(why, "no approving review")
			}
			refusal := fmt.Sprintf("ship #%d: no human pass: %s (bot reviews and the author's own never count)", n, strings.Join(why, "; "))
			if c.DryRun() {
				return Errorf(ExitBlocked, "%s", refusal)
			}
			// Everything else holds, so the PR waits for a person in the inbox.
			body := fmt.Sprintf("%s head=%s -->\n**Ready to ship.** The merge gate is green and its pack verifies on `%s`; `aayu ship` stopped only for the human pass: %s. "+
				"A person ships it from `aayu inbox`, or with `aayu ship %d` once a person other than the author approves the head.\n", ReadyToShipMarker, head, short(head), strings.Join(why, "; "), n)
			if err := upsertOwnComment(ctx, c, repo, n, me.Login, ReadyToShipMarker, body, actions.Action{Kind: "github.comment.ready-to-ship", Story: fmt.Sprintf("#%d", story)}); err != nil {
				return Errorf(ExitBlocked, "%s; the Ready to ship comment was not posted: %v", refusal, err)
			}
			return WithNext(Errorf(ExitBlocked, "%s; commented Ready to ship, so the PR waits in a person's `aayu inbox`", refusal), "aayu inbox (a person)")
		}
		hp, err := e.soloBypass(ctx, c, owner, name, n, head, me.Login, &l.Config)
		if err != nil {
			return err
		}
		rec.HumanPass = hp
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
	var merged *gh.PullRequestMergeResult
	if _, err := c.Write(ctx, actions.Action{Kind: "github.pull.merge", Target: fmt.Sprintf("%s#%d", repo, n), Story: fmt.Sprintf("#%d", story),
		Request: &actions.Request{Method: http.MethodPut, Path: fmt.Sprintf("/repos/%s/pulls/%d/merge", repo, n)},
		Payload: map[string]any{"sha": head, "method": "squash", "human_pass": rec.HumanPass.Kind}},
		func(ctx context.Context) (any, error) {
			var err error
			merged, _, err = c.REST.PullRequests.Merge(ctx, owner, name, n, "", &gh.PullRequestOptions{CommitTitle: fmt.Sprintf("%s (#%d)", pr.GetTitle(), n), SHA: head, MergeMethod: "squash"})
			return map[string]any{"sha": merged.GetSHA()}, err
		}); err != nil {
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
	if err := e.completeStory(ctx, c, repo, story, rec); err != nil {
		return err
	}
	res := ShipResult{ShipRecord: rec, Record: path.Join(path.Dir(rec.Pack), "ship.json")}
	b, _ := json.MarshalIndent(rec, "", "  ")
	if _, err := s.Ledger.Write(ctx, ledger.Commit{Message: fmt.Sprintf("proof: ship record of #%d (PR #%d)", story, n), Files: map[string][]byte{res.Record: append(b, '\n')}}); err != nil {
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
	if root != "" {
		if wt := BuildWorktree(root, story); dirExists(wt) {
			_, _ = run2(ctx, root, "git", "worktree", "remove", "--force", wt)
			_, _ = run2(ctx, root, "git", "branch", "-D", pr.GetHead().GetRef())
		}
	}
	checkpointed = true
	if err := e.checkpoint(tok); err != nil {
		return Wrap(ExitExternal, fmt.Errorf("merged, but the actions segment did not reach the ledger: %w", err))
	}
	if err := e.Emit(res, func(w io.Writer) error {
		fmt.Fprintf(w, "ship #%d: merged as %s; story #%d is status:merged with its Complete section\n", n, short(rec.MergeCommit), story)
		fmt.Fprintf(w, "  pack %s (sha256 %s): %s\n", rec.Pack, short(rec.PackSHA256), rec.PackLanded)
		fmt.Fprintf(w, "  human pass: %s by %s%s\n", rec.HumanPass.Kind, rec.HumanPass.Login, map[bool]string{true: " — " + rec.HumanPass.Note, false: ""}[rec.HumanPass.Note != ""])
		fmt.Fprintf(w, "  review rung: %s (from the merge gate's inspect log)\n", or(rec.Review, "none recorded"))
		fmt.Fprintf(w, "  ship record %s; actions segment %s\n", res.Record, rec.Actions)
		return nil
	}); err != nil {
		return err
	}
	// Every ship lands an authoritative pack (the merge gate's), so the first
	// one is the first proven ship.
	e.milestone("first-proven-ship", true, "Your first proven ship!", fmt.Sprintf("Story #%d merged with an authoritative proof pack and a recorded human pass.", story))
	return nil
}

func dirExists(p string) bool { st, err := os.Stat(p); return err == nil && st.IsDir() }

// gatePackRun is the merge gate's pack for a PR head, unpacked into dir.
type gatePackRun struct {
	run      *gh.WorkflowRun
	artifact int64
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
func (e *Env) gatePack(ctx context.Context, c *github.Client, owner, name string, pr *gh.PullRequest, story int) (gatePackRun, error) {
	n, head, path := pr.GetNumber(), pr.GetHead().GetSHA(), ".github/workflows/"+MergeGateWorkflow
	runs, _, err := c.REST.Actions.ListWorkflowRunsByFileName(ctx, owner, name, MergeGateWorkflow, &gh.ListWorkflowRunsOptions{Branch: pr.GetHead().GetRef(), ListOptions: gh.ListOptions{PerPage: 50}})
	if err != nil {
		return gatePackRun{}, Wrap(ExitExternal, err)
	}
	slices.SortFunc(runs.WorkflowRuns, func(a, b *gh.WorkflowRun) int { return cmp.Compare(b.GetID(), a.GetID()) })
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
	return gatePackRun{}, Errorf(ExitBlocked, "ship #%d: no merge gate pack for the head %s: %s", n, short(head), strings.Join(why, "; "))
}

// unpackGateRun downloads a merge gate run's aayudha-pack artifact and checks
// the pack in it: for this story, PR, and head, with every artifact's bytes
// matching pack.json (proof.LoadLocal).
func (e *Env) unpackGateRun(ctx context.Context, c *github.Client, owner, name string, run *gh.WorkflowRun, n, story int, head string) (gatePackRun, error) {
	arts, _, err := c.REST.Actions.ListWorkflowRunArtifacts(ctx, owner, name, run.GetID(), nil)
	if err != nil {
		return gatePackRun{}, Wrap(ExitExternal, err)
	}
	var art *gh.Artifact
	for _, a := range arts.Artifacts {
		if a.GetName() == "aayudha-pack" {
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
	g := gatePackRun{run: run, artifact: art.GetID()}
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
	url, err := soloModeComment(ctx, c, owner, name, n, login, cfg, "ship --solo-bypass", "a PR comment")
	if err != nil {
		return HumanPass{}, err
	}
	return HumanPass{Kind: "solo-bypass", Login: login, Comment: url, Commit: head,
		Note: "solo mode: GitHub refuses self-approval and no bot App exists, so no second person could review"}, nil
}

// soloModeComment checks that solo mode holds (no bot identity, no second
// paired human) and finds login's comment on issue or PR n starting
// "Solo-mode bypass:"; it returns the comment's URL. what names the command
// in refusals and where the comment belongs.
func soloModeComment(ctx context.Context, c *github.Client, owner, name string, n int, login string, cfg *config.Config, cmd, where string) (string, error) {
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
		return strings.EqualFold(cm.GetUser().GetLogin(), login) && soloComment.MatchString(cm.GetBody())
	})
	if i < 0 {
		return "", Errorf(ExitBlocked, "%s: post %s starting %q that says why, from your own account, first", cmd, where, SoloBypassPrefix)
	}
	return comments[i].GetHTMLURL(), nil
}

// shipExpectHead pins ship to the head a person chose in the inbox; it is not
// a flag, so only the inbox sets it.
const shipExpectHead = "expect-head"

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
		}); err != nil {
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
