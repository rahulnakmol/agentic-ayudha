package cli

import (
	"context"
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
	"strconv"
	"strings"
	"time"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/identity"
	"github.com/rahulnakmol/agentic-ayudha/internal/initrepo"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/ops"
	"github.com/rahulnakmol/agentic-ayudha/internal/production"
	"github.com/rahulnakmol/agentic-ayudha/internal/ready"
	"github.com/rahulnakmol/agentic-ayudha/internal/tell"
	"github.com/rahulnakmol/agentic-ayudha/internal/ui"
	"github.com/rahulnakmol/agentic-ayudha/internal/verify"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

func init() { register("release", runRelease) }

// DefaultRunPoll is how often release polls a workflow run.
const DefaultRunPoll = 15 * time.Second

// The workflows the promotion procedure drives (CONTRIBUTING.md, Promotion).
const (
	gateWorkflow  = "ci.yml"      // dispatched on the candidate with gate=rolling|stable; required
	evalsWorkflow = "evals.yml"   // tier-2 evals of the source tip before a rolling gate; optional
	tagWorkflow   = "release.yml" // runs on the v* tag push; optional
)

// MeasureWindowDays is how long after a release its Goals and Initiatives
// become due for `measure`: measure's default window.
const MeasureWindowDays = 28

// RunRef is a workflow run the promotion waited for.
type RunRef struct {
	Workflow   string `json:"workflow"`
	ID         int64  `json:"id"`
	URL        string `json:"url"`
	Conclusion string `json:"conclusion"`
	Attempts   int    `json:"attempts"`
}

// Artifact is a built artifact's identity: a gate run artifact (rolling) or a
// GitHub Release asset (main).
type Artifact struct {
	Name   string `json:"name"`
	Digest string `json:"digest,omitempty"`
	Size   int64  `json:"size"`
}

// MeasureDue is measure/due/<issue>/<tag>.json on the ledger: a Goal or
// Initiative that shipped work in the release tagged Tag and is due for
// `measure` after Due. One file per release, so a later release never erases
// or postpones an earlier obligation.
type MeasureDue struct {
	Issue   int    `json:"issue"`
	Type    string `json:"type"`
	Due     string `json:"due"`
	Release int    `json:"release"`
	Tag     string `json:"tag"`
	Stories []int  `json:"stories"`
}

// Promotion is releases/<issue>/<target>.json on the ledger: one rung of a
// Release's ladder, with the artifact's identity and the shipping evidence.
type Promotion struct {
	Repo            string         `json:"repo"`
	Release         int            `json:"release"`
	Version         string         `json:"version"`
	Source          string         `json:"source"`
	Target          string         `json:"target"`
	SourceSHA       string         `json:"source_sha"`
	Commit          string         `json:"commit"`
	Tree            string         `json:"tree,omitempty"`
	Message         string         `json:"message"`
	Tag             string         `json:"tag"`
	Candidate       string         `json:"candidate"`
	Evals           *RunRef        `json:"evals,omitempty"`
	Gate            *RunRef        `json:"gate,omitempty"`
	ReleaseRun      *RunRef        `json:"release_run,omitempty"`
	Artifacts       []Artifact     `json:"artifacts"`
	Stories         []int          `json:"stories_released"`
	AlreadyReleased []int          `json:"already_released"`
	NotListed       []int          `json:"shipped_not_listed"`    // shipped in this range but not linked by the Release
	NotMerged       []int          `json:"referenced_not_merged"` // referenced by a PR in this range but not status:merged; not stamped
	Notes           string         `json:"release_notes,omitempty"`
	MeasureDue      []MeasureDue   `json:"measure_due"`
	PromotedBy      string         `json:"promoted_by"`
	PromotedAt      string         `json:"promoted_at"`
	Agent           string         `json:"agent,omitempty"`          // the agent marker when an agent ran it (Story #430); empty for a person
	Request         *StableRequest `json:"stable_request,omitempty"` // main: who asked for the stable release
	DryRun          bool           `json:"dry_run,omitempty"`
	Notices         []string       `json:"notices,omitempty"`
}

var versionRE = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`)

// StableRequest is the explicit stable-release request that promoting to main needs
// (AGENTS.md, Promotion and release): a person's comment on the Release issue,
// or a person's yes at a terminal.
type StableRequest struct {
	Via    string `json:"via"` // comment, terminal, or device-flow
	Author string `json:"author"`
	URL    string `json:"url,omitempty"`
	At     string `json:"at,omitempty"`
}

// StableRequestPrefix starts the first line of a stable-release request
// comment: "Stable release: v1.2.0". The pre-tool hook refuses an agent's shell
// command that would write one (hosts.StableRequestCommand).
const StableRequestPrefix = "Stable release:"

// shippedStatus is where a story already is when an earlier promotion released it.
var shippedStatus = []string{"released", "measured", "done", "cancelled"}

// runRelease drives the promotion procedure of CONTRIBUTING.md for one rung
// of a Release issue's ladder: a promote/… candidate holding the exact merge
// commit, the evals and the ci.yml gate on it, a fast-forward push of that
// commit, the tag, and (main) release.yml. Then it stamps every story the
// promoted range shipped `released`, publishes the release notes (tell),
// records the artifact identity and shipping evidence, and makes the Goals and
// Initiatives above the stories due for measure.
func runRelease(ctx context.Context, e *Env, in Values) (err error) {
	// An agent may promote (founder policy, Story #430): the gate below decides, and
	// main needs a person's stable-release request. CI may not: its token is a
	// workflow or bot token.
	if e.CI {
		return Errorf(ExitBlocked, "release runs under a person's own account; it is refused in CI, where the token is a workflow or bot token")
	}
	agent := e.agentSignal()
	person := agent == "" && e.isTerminal(e.In) // a person at a terminal confirms below; nobody else is asked
	n := issueNumber(in.String("release"))
	to := in.String("to")
	repo, err := e.RepoName(ctx)
	if err != nil {
		return err
	}
	root := config.RepoRoot(e.Dir)
	if root == "" {
		return Errorf(ExitUsage, "release needs a checkout of the repository: it builds the candidate merge commit there")
	}
	l, err := e.Config()
	if err != nil {
		return err
	}
	src, dst := pick(l.Config.Branches.Dev, "dev"), pick(l.Config.Branches.Rolling, "rolling")
	if to == "main" {
		src, dst = dst, pick(l.Config.Branches.Main, "main")
	}
	c, tok, err := e.HumanGitHub(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := e.checkpoint(tok); cerr != nil && err == nil && !c.DryRun() {
			e.Logf("release: actions segment not checkpointed: %v", cerr)
		}
	}()
	me, err := e.humanAccount(ctx, tok, "release")
	if err != nil {
		return err
	}
	owner, name, _ := github.SplitRepo(repo)

	// The Release issue passes Ready.
	rd := ready.Ready{GH: c, Repo: repo, TechnicalOwner: e.technicalOwner()}
	res, is, err := rd.Check(ctx, n)
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	if res.Type != "release" {
		return Errorf(ExitUsage, "release #%d is a %q, not a Release", n, res.Type)
	}
	if is.GetState() != "open" {
		return Errorf(ExitBlocked, "release #%d is closed", n)
	}
	if !res.Pass {
		var ps []string
		for _, p := range res.Problems {
			ps = append(ps, p.String())
		}
		return Errorf(ExitBlocked, "release #%d does not pass Ready: %s", n, strings.Join(ps, "; "))
	}
	it, _ := workitems.Parse(n, is.GetTitle(), is.GetBody(), is.GetType().GetName(), github.IssueLabels(is))
	version := strings.TrimSpace(it.Fields["version"])
	if to == "main" && !versionRE.MatchString(version) {
		return Errorf(ExitBlocked, "release #%d: Version %q: promoting to %s needs vX.Y.Z", n, version, dst)
	}

	g := &ledger.Writer{Dir: root, Repo: repo, Token: tok.Value}
	gitOut := func(dir string, args ...string) (string, error) { return g.Output(ctx, dir, args...) }
	if _, err := gitOut(root, "fetch", "--quiet", "--prune", "origin"); err != nil {
		return Wrap(ExitExternal, err)
	}
	srcSHA, err := gitOut(root, "rev-parse", "origin/"+src, "origin/"+dst)
	if err != nil {
		return Errorf(ExitBlocked, "release: origin needs both %s and %s: %v", src, dst, err)
	}
	srcSHA, _, _ = strings.Cut(srcSHA, "\n")
	revs, _ := gitOut(root, "rev-list", "--no-merges", "origin/"+dst+"..origin/"+src) // both refs exist; an empty range refuses below
	now := e.Now().UTC()
	ts := now.Format("20060102T1504Z")
	p := Promotion{Repo: repo, Release: n, Version: version, Source: src, Target: dst, SourceSHA: srcSHA, PromotedBy: me.Login,
		PromotedAt: now.Format(time.RFC3339), Agent: agent, Artifacts: []Artifact{}, Stories: []int{}, AlreadyReleased: []int{}, NotListed: []int{}, NotMerged: []int{}, MeasureDue: []MeasureDue{}}
	if to == "main" {
		p.Candidate, p.Message, p.Tag = "promote/main-"+version, "Release: "+version, version
		if out, _ := gitOut(root, "ls-remote", "--tags", "origin", "refs/tags/"+version); out != "" {
			return Errorf(ExitBlocked, "release: tag %s already exists; a Release's version ships once", version)
		}
		// The ladder: this Release reached rolling first, and that commit is still on rolling.
		prior, err := e.priorRung(ctx, tok, n)
		if err != nil {
			return err
		}
		if prior == nil {
			return Errorf(ExitBlocked, "release #%d has not been promoted to %s yet; run `aayu release %d --to rolling` first", n, src, n)
		}
		if _, err := gitOut(root, "merge-base", "--is-ancestor", prior.Commit, "origin/"+src); err != nil {
			return Errorf(ExitBlocked, "release #%d: its %s snapshot %s is no longer on %s", n, src, short(prior.Commit), src)
		}
		if person {
			p.Request = &StableRequest{Via: "terminal", Author: me.Login, At: now.Format(time.RFC3339)}
		} else if p.Request, err = e.stableRequest(ctx, c, repo, n, version, prior.PromotedAt, l.Config.Identity.BotLogins); err != nil {
			return err
		} else if p.Request == nil {
			why := fmt.Sprintf("release #%d --to %s needs the explicit stable-release request: a person with write access comments %q on Release #%d, as the first line, after its %s snapshot (%s). An agent cannot write it; ask the person",
				n, dst, StableRequestPrefix+" "+version, n, src, prior.PromotedAt)
			if c.DryRun() {
				p.Notices = append(p.Notices, why)
			} else {
				// Or the person approves it from any device (presence.go): their yes,
				// as at a terminal.
				refusal := WithNext(Errorf(ExitBlocked, "%s", why), fmt.Sprintf("a person comments `%s %s` on #%d", StableRequestPrefix, version, n))
				a, err := e.presenceCheck(ctx, fmt.Sprintf("release #%d --to main (%s)", n, version), AccessWrite, refusal)
				if err != nil {
					return err
				}
				p.Request = &StableRequest{Via: a.Via, Author: a.Login, At: e.Now().UTC().Format(time.RFC3339)}
			}
		}
	} else {
		p.Candidate, p.Message = "promote/rolling-"+ts, "Rolling snapshot: "+ts
		if p.Tag, err = rollingTag(gitOut, root, now); err != nil {
			return Wrap(ExitExternal, err)
		}
	}

	if strings.TrimSpace(revs) == "" {
		return Errorf(ExitBlocked, "release: nothing to promote: %s has no commits that %s lacks", src, dst)
	}

	// What the range ships, against what the Release lists.
	shipped, err := rangeStories(ctx, c, repo, strings.Fields(revs), pick(l.Config.Branches.Dev, "dev"))
	if err != nil {
		return err
	}
	links, _ := workitems.ParseLinks(it.Fields[workitems.FieldLinks])
	listed, err := includedStories(ctx, c, repo, links)
	if err != nil {
		return err
	}
	status, kind := map[int]string{}, map[int]string{}
	for _, s := range append(slices.Clone(listed), shipped...) {
		if _, ok := status[s]; !ok {
			x, _, err := c.REST.Issues.Get(ctx, owner, name, s)
			if err != nil {
				return err
			}
			status[s] = workitems.Item{Labels: github.IssueLabels(x)}.Label("status")
			kind[s], _ = workitems.TypeOf(x.GetType().GetName(), github.IssueLabels(x))
		}
	}
	var problems []string
	for _, s := range listed {
		switch {
		case slices.Contains(shipped, s):
		case slices.Contains(shippedStatus, status[s]):
			p.AlreadyReleased = append(p.AlreadyReleased, s)
		default:
			problems = append(problems, fmt.Sprintf("#%d (status %s) is not in %s..%s", s, pick(status[s], "none"), dst, src))
		}
	}
	if len(problems) > 0 {
		return Errorf(ExitBlocked, "release #%d includes work this promotion does not ship: %s", n, strings.Join(problems, "; "))
	}
	for _, s := range shipped {
		switch {
		case slices.Contains(shippedStatus, status[s]):
		case status[s] != "merged" && !slices.Contains(listed, s):
			// A PR in the range refs it (an umbrella or partly delivered story), but the story
			// itself has not merged: stamping it released would claim work that is not done.
			p.NotMerged = append(p.NotMerged, s)
			continue
		default:
			p.Stories = append(p.Stories, s)
		}
		if !slices.Contains(listed, s) {
			p.NotListed = append(p.NotListed, s)
		}
	}
	if err := e.productionGate(ctx, c, tok, gitOut, root, repo, src, p.Stories, kind); err != nil {
		return err
	}
	// Decision #480: an open security Bug seen on rolling or main holds the promotion, and main takes only a
	// rolling tip the security workflow scanned green.
	guarded := []string{pick(l.Config.Branches.Rolling, "rolling"), pick(l.Config.Branches.Main, "main")}
	if err := e.securityGate(ctx, c, repo, guarded, src, srcSHA, to == "main"); err != nil {
		return err
	}
	notes, utm, err := e.releaseNotes(ctx, c, repo, &p, shipped)
	if err != nil {
		return err
	}
	if c.DryRun() {
		p.DryRun = true
		return e.Emit(p, func(out io.Writer) error { return promotionText(out, p) })
	}
	wfs, _, err := c.REST.Actions.ListWorkflows(ctx, owner, name, &gh.ListOptions{PerPage: 100})
	if err != nil {
		return err
	}
	has := map[string]bool{}
	for _, w := range wfs.Workflows {
		has[path.Base(w.GetPath())] = true
	}
	if !has[gateWorkflow] {
		return Errorf(ExitBlocked, "release: no %s workflow to gate the candidate (dispatch input gate=%s)", gateWorkflow, gateFor(to))
	}
	// The TTY gate is gone for release (Story #430): a person at a terminal is still
	// asked (or says --yes), and an agent, or a run without a terminal, goes on
	// with no prompt: the gate decides, and main already holds a person's request.
	if person && !e.confirm(fmt.Sprintf("Promote %s (%s) to %s as %q for Release #%d? [y/N] ", src, short(srcSHA), dst, p.Message, n)) {
		return Errorf(ExitBlocked, "release not confirmed: nothing was promoted")
	}

	// 1. The candidate: the exact merge commit that will land.
	wt := filepath.Join(e.stateDir(), "state", filepath.FromSlash(p.Candidate)) // git-ignored runtime state
	defer func() {
		_, _ = gitOut(root, "worktree", "remove", "--force", wt)
		_ = os.RemoveAll(wt)
		_, _ = gitOut(root, "branch", "-D", p.Candidate)
	}()
	if _, err := gitOut(root, "worktree", "add", "--quiet", "-b", p.Candidate, wt, "origin/"+dst); err != nil {
		return Wrap(ExitExternal, err)
	}
	if _, err := gitOut(wt, "merge", "--quiet", "--no-ff", "--signoff", "-m", p.Message, "origin/"+src); err != nil {
		return Errorf(ExitBlocked, "release: %s does not merge cleanly into %s: %v", src, dst, err)
	}
	if p.Commit, err = gitOut(wt, "rev-parse", "HEAD"); err != nil {
		return err
	}
	p.Tree, _ = gitOut(wt, "rev-parse", "HEAD^{tree}")
	if err := e.gitPush(ctx, g, wt, repo, p.Candidate, "HEAD:refs/heads/"+p.Candidate); err != nil {
		return err
	}
	defer func() {
		_ = e.gitPush(ctx, g, root, repo, p.Candidate, ":refs/heads/"+p.Candidate)
	}()

	// 2. The gate: evals of the source tip (rolling), then ci.yml on the candidate.
	if to == "rolling" {
		if has[evalsWorkflow] {
			if p.Evals, err = e.dispatchAndWait(ctx, c, repo, evalsWorkflow, src, srcSHA, nil, false); err != nil {
				return err
			}
			if p.Evals.Conclusion != "success" {
				return Errorf(ExitBlocked, "release: %s ended %s on %s (%s); nothing was promoted", evalsWorkflow, p.Evals.Conclusion, short(srcSHA), p.Evals.URL)
			}
		} else {
			p.Notices = append(p.Notices, "no "+evalsWorkflow+": tier-2 evals not run")
		}
	}
	if p.Gate, err = e.dispatchAndWait(ctx, c, repo, gateWorkflow, p.Candidate, p.Commit, map[string]any{"gate": gateFor(to)}, true); err != nil {
		return err
	}
	if p.Gate.Conclusion != "success" {
		return Errorf(ExitBlocked, "release: the %s gate failed on %s (%s); nothing was promoted", gateFor(to), short(p.Commit), p.Gate.URL)
	}
	arts, _, err := c.REST.Actions.ListWorkflowRunArtifacts(ctx, owner, name, p.Gate.ID, &gh.ListOptions{PerPage: 100})
	if err != nil {
		return err
	}
	for _, a := range arts.Artifacts {
		p.Artifacts = append(p.Artifacts, Artifact{Name: a.GetName(), Digest: a.GetDigest(), Size: a.GetSizeInBytes()})
	}

	// 3. Land the gated commit (a fast-forward), then tag it and record it.
	if err := e.gitPush(ctx, g, wt, repo, dst, p.Commit+":refs/heads/"+dst); err != nil {
		return err
	}
	releaseErr, err := e.afterLanding(ctx, c, tok, g, wt, repo, is, &p, has[tagWorkflow], notes, utm, now)
	if err != nil {
		// The target already holds the commit: say what shipped and what is left.
		return Errorf(ExitExternal, "release: %s is on %s as %q, but finishing the promotion failed: %v; finish it by hand: tag %s, then the stamps, notes, and Shipping evidence",
			short(p.Commit), dst, p.Message, err, p.Tag)
	}
	if err := e.Emit(p, func(out io.Writer) error { return promotionText(out, p) }); err != nil {
		return err
	}
	if releaseErr == nil {
		e.milestone("release", false, "Released!", fmt.Sprintf("%s is on %s as %s; %d stor(ies) stamped released.", short(p.Commit), dst, p.Tag, len(p.Stories)))
	}
	return releaseErr
}

// afterLanding runs once the gated commit is on the target: the tag, the
// tag's release.yml (main), the artifact identity, and the bookkeeping. A
// release.yml that stays red is reported, not returned as a failure of these
// steps, since everything else still has to happen.
func (e *Env) afterLanding(ctx context.Context, c *github.Client, tok identity.Token, g *ledger.Writer, wt, repo string, is *gh.Issue, p *Promotion,
	hasTagRun bool, notes tell.Draft, utm tell.UTM, now time.Time) (releaseErr, err error) {
	owner, name, _ := github.SplitRepo(repo)
	main := strings.HasPrefix(p.Message, "Release: ")
	args := []string{"tag", p.Tag, p.Commit}
	if main {
		args = []string{"tag", "-a", p.Tag, "-m", fmt.Sprintf("Release %s: %s", p.Tag, is.GetTitle()), p.Commit}
	}
	if _, err := g.Output(ctx, wt, args...); err != nil {
		return nil, err
	}
	if err := e.gitPush(ctx, g, wt, repo, p.Tag, "refs/tags/"+p.Tag+":refs/tags/"+p.Tag); err != nil {
		return nil, err
	}
	if main && hasTagRun {
		if p.ReleaseRun, err = e.waitPushRun(ctx, c, repo, tagWorkflow, p.Commit); err != nil {
			return nil, err
		}
		if p.ReleaseRun.Conclusion != "success" {
			releaseErr = Errorf(ExitExternal, "release: %s is on %s and tagged %s, but %s ended %s (%s); resume it with `gh run rerun %d --failed`",
				short(p.Commit), p.Target, p.Tag, tagWorkflow, p.ReleaseRun.Conclusion, p.ReleaseRun.URL, p.ReleaseRun.ID)
		}
	} else if main {
		p.Notices = append(p.Notices, "no "+tagWorkflow+": nothing builds the tag")
	}
	if main {
		rel, _, err := c.REST.Repositories.GetReleaseByTag(ctx, owner, name, p.Tag)
		switch {
		case github.NotFound(err):
			p.Notices = append(p.Notices, "no GitHub Release for "+p.Tag+": no release assets to identify")
		case err != nil:
			return releaseErr, err
		}
		for _, a := range rel.GetAssets() {
			p.Artifacts = append(p.Artifacts, Artifact{Name: a.GetName(), Digest: a.GetDigest(), Size: int64(a.GetSize())})
		}
	}
	return releaseErr, e.bookkeep(ctx, c, tok, repo, is, p, notes, utm, now, main)
}

// bookkeep runs after the gated commit landed: it stamps the shipped stories
// released, publishes the release notes, records the rung and the measure
// due dates on the ledger, and writes the Release's Shipping evidence.
func (e *Env) bookkeep(ctx context.Context, c *github.Client, tok identity.Token, repo string, is *gh.Issue, p *Promotion, notes tell.Draft, utm tell.UTM, now time.Time, final bool) error {
	owner, name, _ := github.SplitRepo(repo)
	for _, s := range p.Stories {
		x, _, err := c.REST.Issues.Get(ctx, owner, name, s)
		if err != nil {
			return err
		}
		if err := setStatus(ctx, c, repo, s, github.IssueLabels(x), "released"); err != nil {
			return err
		}
	}
	pub, err := e.publishTell(ctx, c, tok, p.PromotedBy, repo, "release", p.Tag, p.Tag, notes, utm)
	if err != nil {
		return err
	}
	p.Notes = pub.Copy
	due, err := e.measureDue(ctx, c, repo, p, now)
	if err != nil {
		return err
	}
	rec, _ := json.MarshalIndent(p, "", "  ")
	files := map[string][]byte{rungPath(p.Release, p.Target): append(rec, '\n')}
	for _, d := range due {
		b, _ := json.MarshalIndent(d, "", "  ")
		files[fmt.Sprintf("measure/due/%d/%s.json", d.Issue, d.Tag)] = append(b, '\n')
	}
	lw, err := e.Ledger(ctx, tok, "human", "")
	if err != nil {
		return err
	}
	if _, err := lw.Write(ctx, ledger.Commit{Message: fmt.Sprintf("release: #%d %s %s (%s)", p.Release, p.Target, p.Tag, p.PromotedBy), Files: files}); err != nil {
		return err
	}
	return e.shippingEvidence(ctx, c, repo, is, *p, final)
}

// productionGate is the Production Engineer's release gate (Decision #324),
// read from the source branch being promoted. When it defines SLOs, the error
// budget recorded on the ledger must be fresh (production.StaleAfter), and
// while any budget is spent only Bugs ship: a Story ships only with an
// accepted error-budget waiver. When its ops catalog names rollback actions,
// each must have run successfully through aayudha-ops at least once, so no
// release depends on a rollback nobody has tried.
func (e *Env) productionGate(ctx context.Context, c *github.Client, tok identity.Token, gitOut func(string, ...string) (string, error), root, repo, src string, shipping []int, kind map[int]string) error {
	if b, err := gitOut(root, "show", "origin/"+src+":"+production.SLOPath); err == nil && strings.TrimSpace(b) != "" {
		defs, err := production.ParseSLOs([]byte(b))
		if err != nil {
			return Errorf(ExitBlocked, "release: %v", err)
		}
		st, err := e.sloStatus(ctx, tok.Value)
		switch {
		case err != nil:
			return Wrap(ExitExternal, err)
		case st == nil:
			return Errorf(ExitBlocked, "release: %s defines SLOs but no error budget is recorded; run `aayu slo --telemetry <export> --record` first", production.SLOPath)
		}
		if why := st.Stale(e.Now().UTC(), defs); why != "" {
			return Errorf(ExitBlocked, "release: %s; run `aayu slo --telemetry <export> --record` first", why)
		}
		if spent := st.Spent(); len(spent) > 0 {
			var held []string
			for _, s := range shipping {
				if kind[s] == "bug" {
					continue
				}
				ws, err := e.waivers(ctx, repo, s)
				if err != nil {
					return err
				}
				if !slices.ContainsFunc(ws, func(w verify.Waiver) bool { return strings.HasPrefix(w.Title, "Waive the error-budget floor on #") }) {
					held = append(held, fmt.Sprintf("#%d (%s)", s, pick(kind[s], "unknown type")))
				}
			}
			if len(held) > 0 {
				return Errorf(ExitBlocked, "release: the error budget of %s is spent (%s), so only Bugs ship until it recovers; %s would ship. Ship the fixes first, or a person accepts an error-budget waiver on each (`aayu decide --waiver error-budget <n>`)",
					strings.Join(spent, ", "), production.StatusPath, strings.Join(held, ", "))
			}
		}
	}
	b, err := gitOut(root, "show", "origin/"+src+":"+ops.Path)
	if err != nil || strings.TrimSpace(b) == "" {
		return nil
	}
	cat, err := ops.Parse([]byte(b))
	if err != nil {
		return Errorf(ExitBlocked, "release: %v", err)
	}
	var rollbacks []string
	for _, a := range cat.Actions {
		if a.Rollback != "" && a.Rollback != "none" && !slices.Contains(rollbacks, a.Rollback) {
			rollbacks = append(rollbacks, a.Rollback)
		}
	}
	if len(rollbacks) == 0 {
		return nil
	}
	owner, name, _ := github.SplitRepo(repo)
	tried := map[string]bool{}
	opts := &gh.ListWorkflowRunsOptions{Status: "success", ListOptions: gh.ListOptions{PerPage: 100}}
	for page := 0; page < 10; page++ {
		runs, resp, err := c.REST.Actions.ListWorkflowRunsByFileName(ctx, owner, name, initrepo.OpsWorkflow, opts)
		if github.NotFound(err) {
			break
		}
		if err != nil {
			return err
		}
		for _, r := range runs.WorkflowRuns {
			if m := runNameRE.FindStringSubmatch(r.GetName()); m != nil && r.GetConclusion() == "success" {
				tried[m[1]] = true
			}
		}
		if resp == nil || resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	var untried []string
	for _, r := range rollbacks {
		if !tried[r] {
			untried = append(untried, r)
		}
	}
	if len(untried) > 0 {
		return Errorf(ExitBlocked, "release: rollback action(s) %s in %s have never run successfully through %s; try each once (`aayu respond <incident> --action <name>`, approved by a person) before a release depends on it",
			strings.Join(untried, ", "), ops.Path, initrepo.OpsWorkflow)
	}
	return nil
}

func pick(v, def string) string {
	if v != "" {
		return v
	}
	return def
}

func gateFor(to string) string {
	if to == "main" {
		return "stable"
	}
	return "rolling"
}

func rungPath(n int, target string) string { return fmt.Sprintf("releases/%d/%s.json", n, target) }

// rollingTag is rolling-<date>, or rolling-<date>.<n> for the n-th snapshot
// of the day (n ≥ 2).
func rollingTag(gitOut func(string, ...string) (string, error), root string, now time.Time) (string, error) {
	base := "rolling-" + now.Format(time.DateOnly)
	out, err := gitOut(root, "ls-remote", "--tags", "origin", "refs/tags/"+base+"*")
	if err != nil {
		return "", err
	}
	taken := map[string]bool{}
	for _, line := range strings.Split(out, "\n") {
		if f := strings.Fields(line); len(f) == 2 {
			taken[strings.TrimSuffix(strings.TrimPrefix(f[1], "refs/tags/"), "^{}")] = true
		}
	}
	if !taken[base] {
		return base, nil
	}
	for i := 2; ; i++ {
		if t := base + "." + strconv.Itoa(i); !taken[t] {
			return t, nil
		}
	}
}

// priorRung reads this Release's rolling rung from the ledger.
func (e *Env) priorRung(ctx context.Context, tok identity.Token, n int) (*Promotion, error) {
	w, err := e.Ledger(ctx, tok, "human", "")
	if err != nil {
		return nil, err
	}
	t, err := w.Read(ctx)
	if errors.Is(err, ledger.ErrNoLedger) {
		return nil, nil
	} else if err != nil {
		return nil, Wrap(ExitExternal, err)
	}
	defer t.Close()
	l, _ := e.Config()
	b, err := t.File(ctx, rungPath(n, pick(l.Config.Branches.Rolling, "rolling")))
	if err != nil {
		return nil, nil
	}
	var p Promotion
	if err := json.Unmarshal(b, &p); err != nil {
		return nil, fmt.Errorf("ledger %s: %w", rungPath(n, "rolling"), err)
	}
	return &p, nil
}

// rangeStories maps the commits of a promoted range to the stories their
// merged pull requests into dev serve (Refs #n, trailer, or story/<n>- branch).
func rangeStories(ctx context.Context, c *github.Client, repo string, shas []string, dev string) ([]int, error) {
	owner, name, _ := github.SplitRepo(repo)
	seenPR := map[int]bool{}
	var out []int
	for _, sha := range shas {
		prs, _, err := c.REST.PullRequests.ListPullRequestsWithCommit(ctx, owner, name, sha, &gh.ListOptions{PerPage: 100})
		if err != nil {
			return nil, err
		}
		for _, pr := range prs {
			if pr.MergedAt == nil || pr.GetBase().GetRef() != dev || seenPR[pr.GetNumber()] {
				continue
			}
			seenPR[pr.GetNumber()] = true
			for _, s := range storiesOf(pr.GetBody(), pr.GetHead().GetRef()) {
				if !slices.Contains(out, s) {
					out = append(out, s)
				}
			}
		}
	}
	slices.Sort(out)
	return out, nil
}

// storiesOf is every story a merged pull request serves: one PR may deliver several
// (one Refs line each), and a release must stamp all of them, not only the first.
func storiesOf(body, branch string) []int {
	var out []int
	for _, m := range storyRef.FindAllStringSubmatch(body, -1) {
		if n := issueNumber(m[1]); n != 0 && !slices.Contains(out, n) {
			out = append(out, n)
		}
	}
	if len(out) == 0 {
		if s := StoryOf(body, "", branch); s != 0 {
			out = append(out, s)
		}
	}
	return out
}

// includedStories expands a Release's `includes` links: a story or bug is
// itself; a Feature is the stories and bugs under it.
func includedStories(ctx context.Context, c *github.Client, repo string, links []workitems.Link) ([]int, error) {
	owner, name, _ := github.SplitRepo(repo)
	var out []int
	add := func(n int) {
		if !slices.Contains(out, n) {
			out = append(out, n)
		}
	}
	for _, l := range links {
		if l.Kind != "includes" {
			continue
		}
		x, _, err := c.REST.Issues.Get(ctx, owner, name, l.Number)
		if err != nil {
			return nil, err
		}
		if typ, _ := workitems.TypeOf(x.GetType().GetName(), github.IssueLabels(x)); typ != "feature" {
			add(l.Number)
			continue
		}
		subs, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.SubIssue, *gh.Response, error) {
			return c.REST.SubIssue.ListByIssue(ctx, owner, name, int64(l.Number), &o)
		})
		if err != nil {
			return nil, err
		}
		for _, s := range subs {
			x := (*gh.Issue)(s)
			if typ, _ := workitems.TypeOf(x.GetType().GetName(), github.IssueLabels(x)); typ == "story" || typ == "bug" {
				add(x.GetNumber())
			}
		}
	}
	slices.Sort(out)
	return out, nil
}

// pushRetries is how often gitPush retries a ref update the remote could not
// apply: GitHub answers a ref it could not lock, as when the push just before
// is still being processed, with a bare "[remote rejected] … (failed)", and
// git with "cannot lock ref". Every release push is idempotent.
const pushRetries = 3

// gitPush pushes refspec as one git.push action.
func (e *Env) gitPush(ctx context.Context, g *ledger.Writer, dir, repo, ref, refspec string) error {
	log, err := e.Actions()
	if err != nil {
		return err
	}
	_, err = log.Do(ctx, actions.Action{Kind: "git.push", Target: repo + "@" + ref, Identity: "human", Payload: map[string]any{"refspec": refspec}},
		func(ctx context.Context) (actions.Outcome, error) {
			for attempt := 0; ; attempt++ {
				_, err := g.Output(ctx, dir, "push", "--quiet", "origin", refspec)
				if err == nil || attempt == pushRetries || !strings.Contains(err.Error(), "(failed)") && !strings.Contains(err.Error(), "cannot lock ref") {
					return actions.Outcome{}, err
				}
				select {
				case <-ctx.Done():
					return actions.Outcome{}, err
				case <-time.After(e.runPoll()):
				}
			}
		})
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	return nil
}

func (e *Env) runPoll() time.Duration {
	if e.RunPoll > 0 {
		return e.RunPoll
	}
	return DefaultRunPoll
}

// dispatchAndWait dispatches workflow on ref (asking GitHub for the run's ID)
// and waits for the run, which must be on sha. With
// retry, a failed run has its failed jobs re-run once (a transient failure,
// such as a signing timeout, should not stop a promotion).
func (e *Env) dispatchAndWait(ctx context.Context, c *github.Client, repo, workflow, ref, sha string, inputs map[string]any, retry bool) (*RunRef, error) {
	owner, name, _ := github.SplitRepo(repo)
	var id int64
	_, err := c.Write(ctx, actions.Action{Kind: "github.workflow.dispatch", Target: repo + "@" + workflow,
		Request: &actions.Request{Method: http.MethodPost, Path: fmt.Sprintf("/repos/%s/actions/workflows/%s/dispatches", repo, workflow)},
		Payload: map[string]any{"ref": ref, "inputs": inputs}},
		func(ctx context.Context) (any, error) {
			d, _, err := c.REST.Actions.CreateWorkflowDispatchEventByFileName(ctx, owner, name, workflow,
				gh.CreateWorkflowDispatchEventRequest{Ref: ref, Inputs: inputs, ReturnRunDetails: gh.Ptr(true)})
			if d != nil {
				id = d.GetWorkflowRunID()
			}
			return map[string]any{"run_id": id}, err
		})
	if err != nil {
		return nil, Wrap(ExitExternal, err)
	}
	if id == 0 {
		return nil, Errorf(ExitExternal, "release: dispatching %s returned no run ID", workflow)
	}
	return e.waitRun(ctx, c, repo, workflow, id, sha, retry)
}

// waitPushRun waits for the run of workflow that a push of sha (a tag) started.
func (e *Env) waitPushRun(ctx context.Context, c *github.Client, repo, workflow, sha string) (*RunRef, error) {
	owner, name, _ := github.SplitRepo(repo)
	deadline := e.Now().Add(10 * time.Minute)
	for {
		runs, _, err := c.REST.Actions.ListWorkflowRunsByFileName(ctx, owner, name, workflow,
			&gh.ListWorkflowRunsOptions{HeadSHA: sha, Event: "push", ListOptions: gh.ListOptions{PerPage: 20}})
		if err != nil {
			return nil, Wrap(ExitExternal, err)
		}
		if len(runs.WorkflowRuns) > 0 {
			return e.waitRun(ctx, c, repo, workflow, runs.WorkflowRuns[0].GetID(), sha, true)
		}
		if e.Now().After(deadline) {
			return nil, Errorf(ExitExternal, "release: no %s run started for %s within 10 minutes", workflow, short(sha))
		}
		if err := e.sleep(ctx); err != nil {
			return nil, err
		}
	}
}

func (e *Env) sleep(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(e.runPoll()):
		return nil
	}
}

func (e *Env) waitRun(ctx context.Context, c *github.Client, repo, workflow string, id int64, sha string, retry bool) (*RunRef, error) {
	owner, name, _ := github.SplitRepo(repo)
	ref := &RunRef{Workflow: workflow, ID: id}
	want := 1 // the attempt whose result counts; after a re-run, a later one
	pr := e.progress(fmt.Sprintf("Waiting for %s run %d", workflow, id))
	for {
		r, _, err := c.REST.Actions.GetWorkflowRunByID(ctx, owner, name, id)
		if err != nil {
			pr.Done(ui.StatusFail, err.Error())
			return nil, Wrap(ExitExternal, err)
		}
		pr.Update(fmt.Sprintf("Waiting for %s run %d (%s, attempt %d)", workflow, id, pick(r.GetStatus(), "queued"), max(1, r.GetRunAttempt())))
		if r.GetHeadSHA() != "" && r.GetHeadSHA() != sha {
			return nil, Errorf(ExitExternal, "release: %s run %d is on %s, not %s", workflow, id, short(r.GetHeadSHA()), short(sha))
		}
		ref.URL, ref.Attempts = r.GetHTMLURL(), max(1, r.GetRunAttempt())
		if r.GetStatus() == "completed" && ref.Attempts >= want {
			ref.Conclusion = r.GetConclusion()
			if ref.Conclusion == "success" || !retry || want > 1 {
				pr.Done(map[bool]string{true: ui.StatusPass, false: ui.StatusFail}[ref.Conclusion == "success"], ref.Conclusion)
				return ref, nil
			}
			e.Logf("release: %s run %d ended %s; re-running its failed jobs once", workflow, id, ref.Conclusion)
			if _, err := c.Write(ctx, actions.Action{Kind: "github.workflow.rerun", Target: fmt.Sprintf("%s@%s#%d", repo, workflow, id),
				Request: &actions.Request{Method: http.MethodPost, Path: fmt.Sprintf("/repos/%s/actions/runs/%d/rerun-failed-jobs", repo, id)}},
				func(ctx context.Context) (any, error) {
					_, err := c.REST.Actions.RerunFailedJobsByID(ctx, owner, name, id)
					return nil, err
				}); err != nil {
				return nil, Wrap(ExitExternal, err)
			}
			want = ref.Attempts + 1
		}
		if err := e.sleep(ctx); err != nil {
			pr.Done(ui.StatusFail, err.Error())
			return nil, err
		}
	}
}

// parents walks up from n through the sub-issue parents (at most 8 levels).
func parents(ctx context.Context, c *github.Client, repo string, n int) ([]*gh.Issue, error) {
	owner, name, _ := github.SplitRepo(repo)
	var out []*gh.Issue
	for i := 0; i < 8; i++ {
		p, _, err := c.REST.SubIssue.GetParentIssue(ctx, owner, name, int64(n))
		if github.NotFound(err) {
			break
		} else if err != nil {
			return nil, err
		}
		out = append(out, p)
		n = p.GetNumber()
	}
	return out, nil
}

// releaseNotes is `tell release`: notes for the shipped stories that cite
// only proven Features, checked by tell before anything lands (a story title
// with an avoided word refuses the promotion, not its bookkeeping).
func (e *Env) releaseNotes(ctx context.Context, c *github.Client, repo string, p *Promotion, shipped []int) (tell.Draft, tell.UTM, error) {
	owner, name, _ := github.SplitRepo(repo)
	utm := tell.UTM{Source: name, Medium: "release", Campaign: p.Tag}
	brand, err := tell.LoadBrand(config.RepoRoot(e.Dir))
	if err != nil {
		return tell.Draft{}, utm, Wrap(ExitUsage, err)
	}
	proven, _, _, err := e.provenFeatures(ctx, c, repo)
	if err != nil {
		return tell.Draft{}, utm, err
	}
	var stories []tell.Shipped
	for _, s := range shipped {
		x, _, err := c.REST.Issues.Get(ctx, owner, name, s)
		if err != nil {
			return tell.Draft{}, utm, err
		}
		ps, err := parents(ctx, c, repo, s)
		if err != nil {
			return tell.Draft{}, utm, err
		}
		st := tell.Shipped{Number: s, Title: x.GetTitle()}
		for _, q := range ps {
			if typ, _ := workitems.TypeOf(q.GetType().GetName(), github.IssueLabels(q)); typ == "feature" {
				st.Feature = q.GetNumber()
				break
			}
		}
		stories = append(stories, st)
	}
	title := p.Version
	if ts, ok := strings.CutPrefix(p.Message, "Rolling snapshot: "); ok {
		title = "Rolling snapshot " + ts + " (" + p.Tag + ")"
	}
	d, probs := tell.Check(tell.ReleaseNotes(title, proven, stories), repo, proven, brand, utm)
	if len(probs) > 0 {
		return d, utm, Errorf(ExitBlocked, "release: the release notes fail tell's check (%s); fix the story title or the brand, then promote", probs[0])
	}
	return d, utm, nil
}

// measureDue makes every Goal and Initiative above a released story due for
// measure one measure window after the release.
func (e *Env) measureDue(ctx context.Context, c *github.Client, repo string, p *Promotion, now time.Time) ([]MeasureDue, error) {
	by := map[int]*MeasureDue{}
	for _, s := range p.Stories {
		ps, err := parents(ctx, c, repo, s)
		if err != nil {
			return nil, Wrap(ExitExternal, err)
		}
		for _, q := range ps {
			typ, _ := workitems.TypeOf(q.GetType().GetName(), github.IssueLabels(q))
			if typ != "goal" && typ != "initiative" {
				continue
			}
			d := by[q.GetNumber()]
			if d == nil {
				d = &MeasureDue{Issue: q.GetNumber(), Type: typ, Due: now.AddDate(0, 0, MeasureWindowDays).Format(time.DateOnly), Release: p.Release, Tag: p.Tag}
				by[q.GetNumber()] = d
			}
			d.Stories = append(d.Stories, s)
		}
	}
	var out []MeasureDue
	for _, d := range by {
		out = append(out, *d)
	}
	slices.SortFunc(out, func(a, b MeasureDue) int { return a.Issue - b.Issue })
	p.MeasureDue = append(p.MeasureDue, out...)
	return out, nil
}

// shippingEvidence appends this rung to the Release's Shipping evidence and
// labels it status:released (labels first, so the Ready guard ignores the body
// edit); the main rung closes it.
func (e *Env) shippingEvidence(ctx context.Context, c *github.Client, repo string, is *gh.Issue, p Promotion, final bool) error {
	owner, name, _ := github.SplitRepo(repo)
	n := is.GetNumber()
	if err := setStatus(ctx, c, repo, n, github.IssueLabels(is), "released"); err != nil {
		return Wrap(ExitExternal, err)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "**%s → %s** on %s by @%s: `%s` as `%s`, tag `%s`.\n\n", p.Source, p.Target, p.PromotedAt, p.PromotedBy, short(p.Commit), p.Message, p.Tag)
	if p.Agent != "" {
		fmt.Fprintf(&b, "- Run by an agent (`%s`) under @%s's account\n", p.Agent, p.PromotedBy)
	}
	if r := p.Request; r != nil {
		fmt.Fprintf(&b, "- Stable release requested by @%s (%s) %s\n", r.Author, r.Via, r.URL)
	}
	fmt.Fprintf(&b, "- Artifact identity: commit `%s`, tree `%s`, merging %s `%s`\n", p.Commit, p.Tree, p.Source, p.SourceSHA)
	for _, r := range []*RunRef{p.Evals, p.Gate, p.ReleaseRun} {
		if r != nil {
			fmt.Fprintf(&b, "- %s run %d: %s (attempts %d) %s\n", r.Workflow, r.ID, r.Conclusion, r.Attempts, r.URL)
		}
	}
	for _, a := range p.Artifacts {
		fmt.Fprintf(&b, "- Artifact `%s` %s (%d bytes)\n", a.Name, pick(a.Digest, "no digest"), a.Size)
	}
	fmt.Fprintf(&b, "- Stories released: %s", refsList(p.Stories))
	if len(p.AlreadyReleased) > 0 {
		fmt.Fprintf(&b, "; already released: %s", refsList(p.AlreadyReleased))
	}
	if len(p.NotListed) > 0 {
		fmt.Fprintf(&b, "; shipped but not listed here: %s", refsList(p.NotListed))
	}
	if len(p.NotMerged) > 0 {
		fmt.Fprintf(&b, "; referenced but not merged, not stamped: %s", refsList(p.NotMerged))
	}
	b.WriteString("\n")
	fmt.Fprintf(&b, "- Release notes: `%s` on `ledger`; record `%s`\n", p.Notes, rungPath(p.Release, p.Target))
	for _, d := range p.MeasureDue {
		fmt.Fprintf(&b, "- Measure due: #%d (%s) after %s\n", d.Issue, d.Type, d.Due)
	}
	for _, s := range p.Notices {
		fmt.Fprintf(&b, "- Note: %s\n", s)
	}
	it, _ := workitems.Parse(n, is.GetTitle(), is.GetBody(), is.GetType().GetName(), github.IssueLabels(is))
	evidence := strings.TrimSpace(it.Fields["evidence"])
	if workitems.Empty(evidence) {
		evidence = ""
	}
	if evidence != "" {
		evidence += "\n\n"
	}
	body := setSection(is.GetBody(), "Shipping evidence", evidence+strings.TrimSpace(b.String()))
	req := gh.UpdateIssueRequest{Body: &body}
	if final {
		req.State, req.StateReason = gh.Ptr("closed"), gh.Ptr("completed")
	}
	_, err := c.Write(ctx, actions.Action{Kind: "github.issue.complete", Target: fmt.Sprintf("%s#%d", repo, n),
		Request: &actions.Request{Method: http.MethodPatch, Path: fmt.Sprintf("/repos/%s/issues/%d", repo, n)}, Payload: map[string]any{"section": "Shipping evidence", "tag": p.Tag}},
		func(ctx context.Context) (any, error) {
			_, _, err := c.REST.Issues.Update(ctx, owner, name, n, req)
			return nil, err
		})
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	return nil
}

func promotionText(out io.Writer, p Promotion) error {
	verb := "promoted"
	if p.DryRun {
		verb = "would promote"
	}
	fmt.Fprintf(out, "release #%d: %s %s (%s) to %s as %q", p.Release, verb, p.Source, short(p.SourceSHA), p.Target, p.Message)
	if p.Commit != "" {
		fmt.Fprintf(out, " = %s", short(p.Commit))
	}
	fmt.Fprintf(out, ", tag %s (candidate %s)\n", p.Tag, p.Candidate)
	if p.Agent != "" {
		fmt.Fprintf(out, "  run by an agent (%s) under @%s\n", p.Agent, p.PromotedBy)
	}
	if r := p.Request; r != nil {
		fmt.Fprintf(out, "  stable release requested by @%s (%s) %s\n", r.Author, r.Via, r.URL)
	}
	for _, r := range []*RunRef{p.Evals, p.Gate, p.ReleaseRun} {
		if r != nil {
			fmt.Fprintf(out, "  %s: %s %s\n", r.Workflow, r.Conclusion, r.URL)
		}
	}
	fmt.Fprintf(out, "  stories released: %s\n", refsList(p.Stories))
	if len(p.AlreadyReleased) > 0 {
		fmt.Fprintf(out, "  already released: %s\n", refsList(p.AlreadyReleased))
	}
	if len(p.NotListed) > 0 {
		fmt.Fprintf(out, "  shipped but not listed by the Release: %s\n", refsList(p.NotListed))
	}
	if len(p.NotMerged) > 0 {
		fmt.Fprintf(out, "  referenced but not merged (not stamped): %s\n", refsList(p.NotMerged))
	}
	if p.Notes != "" {
		fmt.Fprintf(out, "  release notes: %s\n", p.Notes)
	}
	for _, d := range p.MeasureDue {
		fmt.Fprintf(out, "  measure due: #%d (%s) after %s\n", d.Issue, d.Type, d.Due)
	}
	for _, s := range p.Notices {
		fmt.Fprintf(out, "  note: %s\n", s)
	}
	return nil
}

// stableRequest finds the stable-release request for version on Release n: a
// comment whose first non-blank line is "Stable release: <version>", written
// after the rolling snapshot (rollingAt) by a non-bot account with write,
// maintain, or admin access, and carrying no aayudha marker (aayu never writes
// one). It returns nil when there is none. In solo mode an agent holds the
// person's token, so GitHub cannot tell who typed the comment: aayu writes no
// such comment, and the pre-tool hook refuses an agent's command that would.
func (e *Env) stableRequest(ctx context.Context, c *github.Client, repo string, n int, version, rollingAt string, botLogins []string) (*StableRequest, error) {
	owner, name, _ := github.SplitRepo(repo)
	cs, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.IssueComment, *gh.Response, error) {
		return c.REST.Issues.ListComments(ctx, owner, name, n, &gh.IssueListCommentsOptions{ListOptions: o})
	})
	if err != nil {
		return nil, Wrap(ExitExternal, err)
	}
	access := map[string]bool{}
	for _, cm := range cs {
		first, _, _ := strings.Cut(strings.TrimSpace(cm.GetBody()), "\n")
		v, ok := strings.CutPrefix(strings.TrimSpace(first), StableRequestPrefix)
		u := cm.GetUser()
		if !ok || strings.TrimSpace(v) != version || strings.Contains(cm.GetBody(), "<!-- aayudha:") || cm.GetCreatedAt().UTC().Format(time.RFC3339) <= rollingAt || // both UTC RFC 3339, so they order as text
			identity.IsBot(identity.Account{Login: u.GetLogin(), Type: u.GetType()}, botLogins) {
			continue
		}
		ok, seen := access[u.GetLogin()]
		if !seen {
			lvl, _, err := c.REST.Repositories.GetPermissionLevel(ctx, owner, name, u.GetLogin())
			if err != nil {
				return nil, Wrap(ExitExternal, err)
			}
			ok = lvl.GetPermission() == "admin" || lvl.GetPermission() == "write" || lvl.GetRoleName() == "maintain"
			access[u.GetLogin()] = ok
		}
		if ok {
			return &StableRequest{Via: "comment", Author: u.GetLogin(), URL: cm.GetHTMLURL(), At: cm.GetCreatedAt().UTC().Format(time.RFC3339)}, nil
		}
	}
	return nil, nil
}
