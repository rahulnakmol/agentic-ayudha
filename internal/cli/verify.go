package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/identity"
	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
	"github.com/rahulnakmol/agentic-ayudha/internal/ready"
	"github.com/rahulnakmol/agentic-ayudha/internal/ui"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
	"github.com/rahulnakmol/agentic-ayudha/internal/verify"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

func init() { register("verify", runVerify) }

var (
	storyRef     = regexp.MustCompile(`(?im)^\s*(?:refs|closes|fixes|resolves|story:?)\s+#([0-9]+)\b`)
	storyTrailer = regexp.MustCompile(`(?m)^Aayudha-Story:\s*#?([0-9]+)\s*$`)
	checkpoint   = regexp.MustCompile(`(?m)^Aayudha-Checkpoint:\s*([0-9]+/[0-9]+)\s*$`)
	modelTrailer = regexp.MustCompile(`(?m)^Aayudha-Model:\s*(\S+)\s*$`)
	storyBranch  = regexp.MustCompile(`^(?:story|bug)/([0-9]+)(?:-|$)`)
)

// VerifyResult is what verify reports.
type VerifyResult struct {
	Story      int           `json:"story,omitempty"`
	PR         int           `json:"pr,omitempty"`
	Head       string        `json:"head"`
	Base       string        `json:"base"`
	Footprint  []string      `json:"footprint"`
	Checks     []proof.Check `json:"checks"`
	Hold       bool          `json:"floors_hold"`
	Pack       string        `json:"pack,omitempty"` // local pack directory
	Unmapped   []int         `json:"criteria_without_evidence,omitempty"`
	Problems   []string      `json:"problems,omitempty"`
	NoSnapshot bool          `json:"no_ready_snapshot,omitempty"`
	Waivers    []string      `json:"waivers,omitempty"`
	Gate       []string      `json:"gate_problems,omitempty"` // --ci with a PR: why the merge gate fails besides the floors
	// RetentionDays is how long the merge gate keeps the pack as its
	// aayudha-pack run artifact (--ci with a pack; S-4.2b).
	RetentionDays int `json:"retention_days,omitempty"`
	// Artifacts is where a preview without a pack keeps the floors' artifacts.
	Artifacts string `json:"artifacts_dir,omitempty"`
}

// writeArtifacts replaces dir with the artifacts, by their pack paths.
func writeArtifacts(dir string, files map[string][]byte) error {
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	for rel, b := range files {
		if err := proof.CleanPath(rel); err != nil {
			return err
		}
		f := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(f, b, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func gitIn(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return "", fmt.Errorf("git %s: %s", args[0], strings.TrimSpace(string(ee.Stderr)))
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// StoryOf finds the story a change serves: "Refs #n" (or Closes/Fixes) in the
// PR body, else an Aayudha-Story trailer on the head commit, else a
// story/<n>-… branch name.
func StoryOf(body, headMessage, branch string) int {
	if m := storyRef.FindStringSubmatch(body); m != nil {
		return issueNumber(m[1])
	}
	if m := storyTrailer.FindStringSubmatch(headMessage); m != nil {
		return issueNumber(m[1])
	}
	if m := storyBranch.FindStringSubmatch(branch); m != nil {
		return issueNumber(m[1])
	}
	return 0
}

func (e *Env) devBranch() string {
	if l, err := e.Config(); err == nil && l.Config.Branches.Dev != "" {
		return l.Config.Branches.Dev
	}
	return "dev"
}

// readClient is a read-only client: the workflow token in CI, else the human's.
func (e *Env) readClient(ctx context.Context) (*github.Client, identity.Token, error) {
	tok, err := e.Resolver().Human(ctx)
	if err != nil {
		return nil, tok, Wrap(ExitBlocked, err)
	}
	ident := "human"
	if e.CI {
		ident = "ci"
	}
	c, err := e.newGitHub(tok, ident, "")
	return c, tok, err
}

func runVerify(ctx context.Context, e *Env, in Values) error {
	root := config.RepoRoot(e.Dir)
	if root == "" {
		return Errorf(ExitUsage, "verify runs in a checkout of the change")
	}
	l, err := e.Config()
	if err != nil {
		return err
	}
	maintain := in.String("maintain")
	if maintain != "" && !in.Bool("map") && !in.Has("check") {
		return Errorf(ExitUsage, "verify --maintain runs with --map or --check")
	}
	if in.Bool("map") {
		return e.verifyMap(ctx, root, maintain)
	}
	if in.Bool("security") {
		return e.verifySecurity(ctx, root, l, in.String("since"))
	}
	if in.Has("since") {
		return Errorf(ExitUsage, "verify --since runs with --security")
	}
	head, err := gitIn(ctx, root, "rev-parse", "HEAD")
	if err != nil {
		return Wrap(ExitUsage, err)
	}
	res := VerifyResult{Head: head}
	var (
		body, branch string
		labels       []string
		pr           *gh.PullRequest
		client       *github.Client
		tok          identity.Token
	)
	repo, _ := e.RepoName(ctx)
	if in.Has("pr") {
		res.PR = issueNumber(in.String("pr"))
		if client, tok, err = e.readClient(ctx); err != nil {
			return err
		}
		owner, name, err := github.SplitRepo(repo)
		if err != nil {
			return Wrap(ExitUsage, err)
		}
		if pr, _, err = client.REST.PullRequests.Get(ctx, owner, name, res.PR); err != nil {
			return Wrap(ExitExternal, err)
		}
		if pr.GetHead().GetSHA() != head {
			return Errorf(ExitBlocked, "verify %d: this checkout is at %s, but the PR head is %s; check out the PR head first", res.PR, short(head), short(pr.GetHead().GetSHA()))
		}
		res.Base, body, branch = pr.GetBase().GetSHA(), pr.GetBody(), pr.GetHead().GetRef()
		for _, lb := range pr.Labels {
			labels = append(labels, lb.GetName())
		}
		if mb, err := gitIn(ctx, root, "merge-base", res.Base, head); err == nil {
			res.Base = mb
		}
	} else {
		branch, _ = gitIn(ctx, root, "rev-parse", "--abbrev-ref", "HEAD")
		base := "origin/" + e.devBranch()
		if res.Base, err = gitIn(ctx, root, "merge-base", base, head); err != nil {
			return Errorf(ExitUsage, "verify: no merge base with %s (fetch it, or pass the PR): %v", base, err)
		}
	}
	msg, _ := gitIn(ctx, root, "log", "-1", "--format=%B", head)
	res.Story = StoryOf(body, msg, branch)
	// The merge gate proves stories; a PR that serves no story has no pack to
	// prove, but its feature-map floors still run: the drift check, and the
	// behaviour checks of what it touches (WP4.5b).
	noStoryGate := e.CI && res.PR > 0 && res.Story == 0

	r := &verify.Run{Root: root, Base: res.Base, Head: head, Floors: l.Config.Verify.Floors, Verify: l.Config.Verify, Story: res.Story, Labels: labels, Now: e.Now, Env: e.Getenv,
		CI: e.CI, Nightly: e.Getenv("GITHUB_EVENT_NAME") == "schedule"}
	if r.Changed, r.Lines, err = verify.Changes(ctx, root, res.Base, head); err != nil {
		return Wrap(ExitUsage, err)
	}
	r.Footprint = verify.ReadLayers(root).Footprint(r.Changed)
	res.Footprint = r.Footprint
	if res.Footprint == nil {
		res.Footprint = []string{}
	}

	var snap ready.LedgerSnapshot
	if res.Story > 0 {
		if client == nil {
			if _, tok, err = e.readClient(ctx); err != nil {
				return err
			}
		}
		w, err := e.Ledger(ctx, tok, "human", "")
		if err != nil {
			return err
		}
		if t, err := w.Read(ctx); err == nil {
			if b, err := t.File(ctx, ready.LedgerPath(res.Story)); err == nil {
				if snap, err = ready.ParseSnapshot(string(b)); err != nil {
					return err
				}
				r.Criteria = snap.Criteria
			}
			t.Close()
		}
		res.NoSnapshot = snap.CompletionGoalHash == ""
	}

	// Captures never run against production (WP4.5, V3): refuse before
	// seeding, capturing, or writing anything.
	if _, err := verify.ResolveTarget(r); err != nil {
		return Wrap(ExitBlocked, err)
	}

	home, err := os.MkdirTemp("", "aayudha-verify-home-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(home) }()
	work, err := os.MkdirTemp("", "aayudha-verify-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(work) }()
	r.Sandbox, r.Work = &verify.Sandbox{Getenv: e.Getenv, Home: home}, work

	floors := verify.Standard()
	switch {
	case in.Has("check"):
		r.Checks, floors = in.String("check"), []verify.Floor{verify.Behaviour{}}
	case noStoryGate:
		floors = verify.MapFloors()
		if f := in.String("floor"); f != "" {
			floors, _ = verify.ByName(floors, f) // another floor does not run without a story
		}
	case in.String("floor") != "":
		if floors, err = verify.ByName(floors, in.String("floor")); err != nil {
			return Wrap(ExitUsage, err)
		}
	}
	// One floor at a time, so a person sees each one run (WP1.1b); the report
	// is the one verify.Execute would build from all of them.
	var rep verify.Report
	for i, f := range floors {
		fp := e.progress(fmt.Sprintf("Floor %d/%d: %s", i+1, len(floors), f.Name()))
		one := verify.Execute(ctx, r, []verify.Floor{f})
		rep.Checks, rep.Artifacts = append(rep.Checks, one.Checks...), append(rep.Artifacts, one.Artifacts...)
		if len(one.Checks) > 0 {
			c := one.Checks[0]
			fp.Done(floorMark(c.Result), c.Result)
		} else {
			fp.Done(ui.StatusNA, "no check")
		}
	}
	// P5 (founder decision 4): a Bug story's proving tests must fail on the base.
	if e.CI && pr != nil && res.Story > 0 && !in.Has("check") && in.String("floor") == "" {
		owner, name, _ := github.SplitRepo(repo)
		if is, _, err := client.REST.Issues.Get(ctx, owner, name, res.Story); err != nil {
			return Wrap(ExitExternal, err)
		} else if slices.ContainsFunc(is.Labels, func(lb *gh.Label) bool { return lb.GetName() == "type:bug" }) {
			c, ok, err := redOnBase(ctx, r.Sandbox, root, res.Base, head, res.Story, e.Now(), l.Config.Verify.RedOnBaseBlockingFrom)
			if err != nil {
				return Wrap(ExitExternal, fmt.Errorf("red-on-base: %w", err))
			}
			if ok {
				rep.Checks = append(rep.Checks, c)
			}
		}
	}
	rep.Hold = (&proof.Pack{Checks: rep.Checks}).FloorsHold()
	if res.Story > 0 && !rep.Hold {
		ws, err := e.waivers(ctx, repo, res.Story)
		if err != nil {
			return Wrap(ExitExternal, err)
		}
		res.Waivers = verify.ApplyWaivers(rep.Checks, res.Story, ws)
		rep.Hold = (&proof.Pack{Checks: rep.Checks}).FloorsHold()
	}
	ev, problems, err := verify.TestEvidence(r)
	if err != nil {
		return Wrap(ExitUsage, err)
	}
	rep.Artifacts = append(rep.Artifacts, ev...)
	res.Checks, res.Hold, res.Problems = rep.Checks, rep.Hold, problems

	if res.Story > 0 && !res.NoSnapshot {
		ident := identity.KindHumanFallback
		if identity.Configured(l.Config.Identity) != "" {
			ident = "bot"
		}
		if e.CI {
			ident = "ci"
		}
		host := e.Host
		if host == "" {
			host = "cli"
		}
		p := proof.Pack{Repo: repo, PR: res.PR, Seat: string(verbs.AINativeEngineer), Host: host, Identity: ident,
			CompletionGoalHash: snap.CompletionGoalHash, CreatedAt: e.Now().UTC().Format("2006-01-02T15:04:05Z"),
			Links: proof.Links{Issue: fmt.Sprintf("%s#%d", repo, res.Story), ReadySnapshot: ready.LedgerPath(res.Story)}}
		if res.PR > 0 {
			p.Links.PR = fmt.Sprintf("%s#%d", repo, res.PR)
		}
		if m := checkpoint.FindStringSubmatch(msg); m != nil {
			p.Checkpoint = m[1]
		}
		if m := modelTrailer.FindStringSubmatch(msg); m != nil {
			p.Model = m[1]
		}
		pack, files := verify.Pack(r, rep, p)
		res.Unmapped = verify.Unmapped(r, pack)
		res.Pack = proof.LocalDir(root, res.Story)
		if err := proof.WriteLocal(res.Pack, pack, files); err != nil {
			return err
		}
		if e.CI {
			res.RetentionDays = proof.ArtifactRetention(l.Config.Proof.RetentionDays, e.Getenv("GITHUB_RETENTION_DAYS"))
		}
	}
	// A preview without a story keeps what the floors captured, so a person or
	// an agent can look at a behaviour check's screenshots and traces (WP4.5b).
	if res.Pack == "" && !e.CI && len(rep.Artifacts) > 0 {
		res.Artifacts = filepath.Join(root, ".aayudha", "state", "verify")
		files := map[string][]byte{}
		for _, a := range rep.Artifacts {
			files[a.Path] = a.Data
		}
		if err := writeArtifacts(res.Artifacts, files); err != nil {
			return err
		}
	}
	// The merge gate (WP4.8): the pack must prove the PR's current head, the
	// story must still match its Ready snapshot, and every criterion must have
	// evidence from this run.
	if e.CI && res.PR > 0 && res.Story > 0 {
		res.Gate = e.gateProblems(ctx, repo, res, snap.CompletionGoalHash)
	}
	if err := e.Emit(res, func(w io.Writer) error { return renderVerify(w, res) }); err != nil {
		return err
	}
	if maintain != "" {
		d, _, err := verify.CheckMaps(ctx, root)
		if err != nil {
			return Wrap(ExitUsage, err)
		}
		b := verify.Brief{Drift: d}
		b.Failed, b.Ran = failedChecks(rep.Artifacts)
		if err := e.maintainHandoff(ctx, maintain, b); err != nil {
			return err
		}
	}
	if !res.Hold {
		return Errorf(ExitFloor, "floor miss: %s", failing(res.Checks))
	}
	if len(res.Gate) > 0 {
		return Errorf(ExitBlocked, "merge gate: %s", strings.Join(res.Gate, "; "))
	}
	return nil
}

// verifyMap is `verify --map`: the feature maps' drift check (WP4.5b.1). It
// reads the maps and the checkout and runs no product code.
func (e *Env) verifyMap(ctx context.Context, root, maintain string) error {
	d, _, err := verify.CheckMaps(ctx, root)
	if err != nil {
		return Wrap(ExitUsage, err)
	}
	if err := e.Emit(d, func(w io.Writer) error {
		switch d.Verdict {
		case verify.DriftNone:
			_, err := fmt.Fprintln(w, "no feature map here (.agents/skills/verify-<product>/features/README.md); `aayu init --verify-skill` scaffolds one")
			return err
		case verify.DriftClean:
			fmt.Fprintf(w, "feature maps in sync with the source at %s:", short(d.Head))
		default:
			fmt.Fprintf(w, "feature maps at %s: %s", short(d.Head), d.Verdict)
		}
		for _, m := range d.Maps {
			n := 0
			for _, f := range m.Features {
				n += f.Checks
			}
			fmt.Fprintf(w, " %s (%d features, %d checks)", m.Dir, len(m.Features), n)
		}
		fmt.Fprintln(w)
		for _, x := range d.Items() {
			fmt.Fprintf(w, "  %s\n", x)
		}
		return nil
	}); err != nil {
		return err
	}
	if maintain != "" {
		if err := e.maintainHandoff(ctx, maintain, verify.Brief{Drift: d}); err != nil {
			return err
		}
	}
	if d.Verdict == verify.DriftFound {
		return Errorf(ExitFloor, "feature map drift: fix the map in its skill directory, or let the maintain pass do it")
	}
	return nil
}

// gateProblems are the merge gate's checks beyond the floors.
func (e *Env) gateProblems(ctx context.Context, repo string, res VerifyResult, snapCG string) []string {
	var out []string
	if res.NoSnapshot {
		return []string{fmt.Sprintf("story #%d has no Ready snapshot on the ledger", res.Story)}
	}
	if len(res.Unmapped) > 0 {
		out = append(out, fmt.Sprintf("criteria without evidence from this run: %v (map them to tests in .aayudha/evidence/%d.toml)", res.Unmapped, res.Story))
	}
	c, _, err := e.readClient(ctx)
	if err != nil {
		return append(out, err.Error())
	}
	owner, name, _ := github.SplitRepo(repo)
	if pr, _, err := c.REST.PullRequests.Get(ctx, owner, name, res.PR); err != nil {
		out = append(out, "PR: "+err.Error())
	} else if pr.GetHead().GetSHA() != res.Head {
		out = append(out, fmt.Sprintf("pack.sha %s ≠ PR head %s (the PR moved; the gate re-runs on the new head)", short(res.Head), short(pr.GetHead().GetSHA())))
	}
	now, _, err := (&ready.Ready{GH: c, Repo: repo, TechnicalOwner: e.technicalOwner()}).Check(ctx, res.Story)
	switch {
	case err != nil:
		out = append(out, "story: "+err.Error())
	case now.CompletionGoalHash != snapCG:
		out = append(out, fmt.Sprintf("story #%d's Completion Goal hash %s ≠ Ready snapshot %s (edited after Ready: run `aayu ready %d` again)", res.Story, short(now.CompletionGoalHash), short(snapCG), res.Story))
	}
	return out
}

// waivers are the accepted floor-waiver Decisions under a story: sub-issues
// of type decision that a human accepted with `decide --accept waive`
// (WP5.6), so they are closed as completed, by someone who is not a bot, with
// "waive, accepted by @<login>" in their Decision section. A filed but
// unaccepted waiver waives nothing (rule 7).
func (e *Env) waivers(ctx context.Context, repo string, story int) ([]verify.Waiver, error) {
	c, _, err := e.readClient(ctx)
	if err != nil {
		return nil, err
	}
	l, err := e.Config()
	if err != nil {
		return nil, err
	}
	owner, name, _ := github.SplitRepo(repo)
	subs, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.SubIssue, *gh.Response, error) {
		return c.REST.SubIssue.ListByIssue(ctx, owner, name, int64(story), &o)
	})
	if err != nil {
		return nil, err
	}
	var out []verify.Waiver
	for _, s := range subs {
		is := (*gh.Issue)(s)
		if t, _ := workitems.TypeOf(is.GetType().GetName(), github.IssueLabels(is)); t != "decision" {
			continue
		}
		_, v := workitems.Sections(is.GetBody())
		if is.GetState() != "closed" || is.GetStateReason() != "completed" || !strings.HasPrefix(strings.ToLower(strings.TrimSpace(v["Decision"])), "waive, accepted by @") {
			continue
		}
		full, _, err := c.REST.Issues.Get(ctx, owner, name, is.GetNumber())
		if err != nil {
			return nil, err
		}
		if by := full.GetClosedBy(); by == nil || identity.IsBot(identity.Account{Login: by.GetLogin(), Type: by.GetType()}, l.Config.Identity.BotLogins) {
			continue
		}
		out = append(out, verify.Waiver{Decision: is.GetNumber(), Title: is.GetTitle(), Body: is.GetBody(), Links: v["Links"]})
	}
	return out, nil
}

func failing(cs []proof.Check) string {
	var out []string
	for _, c := range cs {
		if c.Result == proof.Fail || c.Result == proof.Blocked || proof.BlocksShip(c) {
			out = append(out, c.Floor+" "+c.Result)
		}
	}
	return strings.Join(out, ", ")
}

func renderVerify(w io.Writer, res VerifyResult) error {
	who := "no story"
	if res.Story > 0 {
		who = fmt.Sprintf("story #%d", res.Story)
	}
	fmt.Fprintf(w, "verify %s..%s (%s; footprint %s)\n", short(res.Base), short(res.Head), who, strings.Join(res.Footprint, " "))
	for _, c := range res.Checks {
		fmt.Fprintf(w, "  %-14s %-8s value %-8g threshold %-6g %s %s\n", c.Floor, c.Result, c.Value, c.Threshold, c.Tool, c.Version)
		if c.Detail != "" {
			fmt.Fprintf(w, "  %14s %s\n", "", c.Detail)
		}
	}
	for _, p := range res.Problems {
		fmt.Fprintf(w, "  evidence: %s\n", p)
	}
	for _, x := range res.Waivers {
		fmt.Fprintf(w, "  waived: %s\n", x)
	}
	for _, g := range res.Gate {
		fmt.Fprintf(w, "  merge gate: %s\n", g)
	}
	switch {
	case res.Pack != "":
		fmt.Fprintf(w, "pack written to %s", res.Pack)
		if len(res.Unmapped) > 0 {
			fmt.Fprintf(w, " (criteria without evidence: %v)", res.Unmapped)
		}
		fmt.Fprintln(w, "; `aayu proof push` lands it on the ledger (advisory)")
	case res.Story == 0 && res.PR > 0:
		fmt.Fprintf(w, "merge gate: PR #%d serves no story (no Refs #n, Aayudha-Story trailer, or story/<n>- branch), so there is no pack; only the feature-map floors run\n", res.PR)
	case res.Story > 0 && res.NoSnapshot:
		fmt.Fprintf(w, "no pack: #%d has no Ready snapshot on the ledger (run `aayu ready %d`)\n", res.Story, res.Story)
	default:
		fmt.Fprintln(w, "no pack: no story found (Refs #n in the PR body, an Aayudha-Story trailer, or a story/<n>-… branch)")
	}
	if res.Artifacts != "" {
		fmt.Fprintf(w, "artifacts kept in %s (a preview)\n", res.Artifacts)
	}
	return nil
}

// floorMark is the status word a floor result shows as live progress.
func floorMark(result string) string {
	switch result {
	case proof.Pass, proof.Waived:
		return ui.StatusPass
	case proof.Skipped:
		return ui.StatusNA
	case proof.Fail, proof.Blocked:
		return ui.StatusFail
	}
	return ui.StatusInfo
}
