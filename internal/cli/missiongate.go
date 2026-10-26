package cli

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	gh "github.com/google/go-github/v90/github"
	"github.com/pelletier/go-toml/v2"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/ready"
	"github.com/rahulnakmol/agentic-ayudha/internal/seats"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
	"github.com/rahulnakmol/agentic-ayudha/internal/verify"
)

// The milestone gate (Story #1001, Decisions #915 M2 and #974). When the last
// story or Bug of a milestone has merged, and every earlier milestone passed
// its gate, `aayu mission gate <f>` judges the dev tip: the required checks,
// `aayu verify --check all`, and the evals of the seats the milestone
// touched. Fixed rules decide pass or fail; no model does. A pass marks the
// milestone passed with the dev SHA, so the next milestone's PRs may merge
// (ST16 reads it). Each failed check gets exactly one fix PR, which the
// Kanha starts as a subthread in one agents slot: the PR says `Refs #<f>`,
// no issue is filed, and the plan's gate_fixes records it with the approval
// it rests on. A failed gate holds no start; after [mission] max_no_go failed
// runs the mission stops and asks a person.

// MissionGateSchema is the schema of what `aayu mission gate` reports.
const MissionGateSchema = "aayudha.mission.gate/v1"

// MissionGatesPath is the ledger file with one line per gate run of Feature f.
func MissionGatesPath(f int) string { return fmt.Sprintf("missions/%d/gates.jsonl", f) }

// missionMaxNoGo is [mission] max_no_go, committed on the integration branch
// at its tip; a value the person's own config, the environment, or --set
// gives may only lower it, as buildAgents does. The embedded default never does.
func (e *Env) missionMaxNoGo(ctx context.Context, c *github.Client, repo string) (int, error) {
	owner, name, _ := github.SplitRepo(repo)
	pol, _, err := e.loopPolicy(ctx, c, owner, name)
	if err != nil {
		return 0, err
	}
	l, err := e.Config()
	if err != nil {
		return 0, err
	}
	n := pol.Config.Mission.MaxNoGo
	if layer, _ := l.Origin("mission.max_no_go"); layer != config.LayerDefault {
		n = min(n, l.Config.Mission.MaxNoGo)
	}
	return max(n, 1), nil
}

// evalsResultsMinRuns is how many evals.yml runs of a commit the gate reads,
// as the rolling gate's evals-gate does (ci.yml, `gh run list --limit 20`).
const evalsResultsMinRuns = 20

// GateRun is one gate run of a milestone: the dev SHA and each check's result.
type GateRun struct {
	Milestone string      `json:"milestone"`
	DevSHA    string      `json:"dev_sha"`
	At        string      `json:"at"`
	Result    string      `json:"result"` // passed, failed, or pending (a check reran)
	Checks    []GateCheck `json:"checks"`
	By        string      `json:"by,omitempty"` // the person who ran it once more
}

// GateCheck is one check of a gate run: a required check ("check:build"),
// a verify check ("verify:inbox.ship-once"), or an eval case
// ("eval:mission/stays-in-list").
type GateCheck struct {
	Check  string `json:"check"`
	Result string `json:"result"` // passed, failed, or pending
	Detail string `json:"detail,omitempty"`
	Rerun  string `json:"rerun,omitempty"` // why aayu reran it once
}

// Gate check results.
const (
	gatePass    = "passed"
	gateFail    = "failed"
	gatePending = "pending"
)

// GateReport is what `aayu mission gate <f>` reports.
type GateReport struct {
	Schema       string        `json:"schema"`
	Repo         string        `json:"repo"`
	Feature      int           `json:"feature"`
	DevSHA       string        `json:"dev_sha"`
	Milestones   []GateMSState `json:"milestones"`
	Fixes        []GateFixItem `json:"fixes"`
	Recorded     *GateFix      `json:"recorded,omitempty"` // with --pr
	LedgerCommit string        `json:"ledger_commit,omitempty"`
	DryRun       bool          `json:"dry_run,omitempty"`
}

// GateMSState is a milestone's gate after this run.
type GateMSState struct {
	Name   string      `json:"name"`
	State  string      `json:"state"`
	Detail string      `json:"detail"`
	Checks []GateCheck `json:"checks,omitempty"` // the run this command made
}

// GateFixItem is one failed check and its fix: start (the Kanha starts its
// subthread with Brief), waits, started (a subthread has it), or fixing (its
// PR is recorded).
type GateFixItem struct {
	Milestone string `json:"milestone"`
	Check     string `json:"check"`
	DevSHA    string `json:"dev_sha"`
	Action    string `json:"action"`
	Reason    string `json:"reason,omitempty"`
	PR        int    `json:"pr,omitempty"`
	Brief     string `json:"brief,omitempty"`
}

// gateFixLine is the line a fix PR's body names its check with.
var gateFixLine = regexp.MustCompile(`(?m)^Aayudha-Gate-Fix:[ \t]*([A-Za-z0-9][A-Za-z0-9._-]*)[ \t]+(\S+)[ \t]*$`)

// runnerError is a check that failed for its runner, not its code: GitHub
// cancelled it or could not start it, or its log names a 5xx from GitHub.
var runnerLog = regexp.MustCompile(`(?i)\b(HTTP|status|returned|error)[: ]+5[0-9][0-9]\b|\b5[0-9][0-9] (Internal Server Error|Bad Gateway|Service Unavailable|Gateway Timeout)\b`)

// fixKey is the claims key of a failed check's fix: a negative number, so it
// takes one agents slot in missions/<f>/claims.jsonl beside the stories.
func fixKey(milestone, check, sha string) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(milestone + " " + check + " " + sha))
	return -int(h.Sum32()&0x3fffffff) - 1
}

// noGoTitle is the card and the stop reason after max_no_go failed runs.
func noGoTitle(m string, f, n int) string {
	return fmt.Sprintf("%s of #%d failed its gate %d times: its stories wait", m, f, n)
}

// missionGate is `aayu mission gate <f>`: it runs the gate of each milestone
// that is due, lists the fix of each failed check, and with --pr records a
// fix PR; --once-more is a person's run of a stopped gate.
func missionGate(ctx context.Context, e *Env, f int, in Values) error {
	if in.Has("pr") {
		return missionGateFix(ctx, e, f, issueNumber(in.String("pr")))
	}
	once := in.Bool("once-more")
	if once {
		if err := e.laneRefusal("mission gate --once-more"); err != nil {
			return err
		}
	}
	repo, c, w, err := e.missionLedger(ctx)
	if err != nil {
		return err
	}
	plan, appr, _, err := readMission(ctx, w, f)
	if err != nil {
		return err
	}
	switch {
	case plan == nil || appr == nil:
		return WithNext(Errorf(ExitBlocked, "#%d has no mission plan: a person approves it to build first. No gate runs.", f), fmt.Sprintf("aayu ready %d", f))
	case plan.StartedBy == "":
		return WithNext(Errorf(ExitBlocked, "The mission of #%d has not started: a person starts it. No gate runs.", f), fmt.Sprintf("aayu mission start %d", f))
	}
	stopped := slices.IndexFunc(plan.Milestones, func(m MissionMilestone) bool { return m.Gate.State == GateStopped })
	switch {
	case once && stopped < 0:
		return Errorf(ExitBlocked, "No gate of #%d is stopped: the gate runs on its own when a milestone's last story merges.", f)
	case !once && stopped >= 0:
		return Errorf(ExitBlocked, "The mission of #%d stopped: %s. A person runs the gate once more, or stops the mission, in aayu inbox.", f, plan.Pause.reason())
	case plan.Pause != nil && stopped < 0:
		return Errorf(ExitBlocked, "The mission of #%d is paused: %s. No gate runs.", f, plan.Pause.Reason)
	}
	by := ""
	var presence *Approval
	if once {
		m := plan.Milestones[stopped]
		if presence, err = e.presenceCheck(ctx, Pending{What: "mission gate --once-more", Effect: []string{
			fmt.Sprintf("Runs the gate of %s of #%d once more on the dev tip: the required checks, aayu verify --check all, and the evals", m.Name, f),
			"A pass resumes the mission; a fail keeps it stopped and asks you again",
		}}, AccessWrite, e.personOnly("mission gate --once-more")); err != nil {
			return err
		}
		by = presence.Login
	}
	maxNoGo, err := e.missionMaxNoGo(ctx, c, repo)
	if err != nil {
		return err
	}
	agents, _, err := e.missionAgents(ctx, c, repo, f, appr.Agents)
	if err != nil {
		return err
	}
	owner, name, _ := github.SplitRepo(repo)
	dev := e.devBranch()
	br, _, err := c.REST.Repositories.GetBranch(ctx, owner, name, dev, 1)
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	sha := br.GetCommit().GetSHA()
	runs, err := gateRuns(ctx, w, f)
	if err != nil {
		return err
	}
	merged, layers := map[int]bool{}, map[int][]string{}
	for _, it := range plan.Graph {
		is, _, err := c.REST.Issues.Get(ctx, owner, name, it.Number)
		if err != nil {
			return Wrap(ExitExternal, err)
		}
		merged[it.Number] = is.GetState() == "closed"
		for _, l := range github.IssueLabels(is) {
			if v, ok := strings.CutPrefix(l, "layer:"); ok {
				layers[it.Number] = append(layers[it.Number], v)
			}
		}
	}
	prs := map[int]*gh.PullRequest{}
	prOf := func(n int) (*gh.PullRequest, error) {
		if prs[n] == nil {
			p, _, err := c.REST.PullRequests.Get(ctx, owner, name, n)
			if err != nil {
				return nil, Wrap(ExitExternal, err)
			}
			prs[n] = p
		}
		return prs[n], nil
	}
	rep := GateReport{Schema: MissionGateSchema, Repo: repo, Feature: f, DevSHA: sha, Milestones: []GateMSState{}, Fixes: []GateFixItem{}, DryRun: c.DryRun()}
	g := &gateTree{root: config.RepoRoot(e.Dir)}
	defer g.close()
	var lines []GateRun
	type outcome struct {
		m    string
		gate MilestoneGate
		stop bool
	}
	var done []outcome
	var asks []missionAsk
	var closes []string
	earlierPassed := true
	for _, m := range plan.Milestones {
		row := GateMSState{Name: m.Name, State: m.Gate.State}
		last := lastGateRun(runs, m.Name, false)
		k := 0
		for _, n := range m.Stories {
			if merged[n] {
				k++
			}
		}
		open, err := openFixes(plan, m.Name, last, prOf)
		if err != nil {
			return err
		}
		switch {
		case m.Gate.State == GatePassed:
			row.Detail = fmt.Sprintf("passed on dev %s at %s", short(m.Gate.DevSHA), m.Gate.At)
		case !earlierPassed:
			row.Detail = "waits for the milestones before it to pass their gates"
		case k < len(m.Stories):
			row.Detail = fmt.Sprintf("building: %d of %d merged; the gate runs when its last story merges", k, len(m.Stories))
		case !once && m.Gate.State == GateFailed && len(open) > 0:
			row.Detail = "failed on dev " + short(m.Gate.DevSHA) + "; the gate runs again when its fixes merge: " + strings.Join(open, ", ")
		case !once && last != nil && last.DevSHA == sha:
			row.Detail = fmt.Sprintf("%s on dev %s, the tip now", last.Result, short(sha))
		default:
			run, err := e.runGate(ctx, c, g, repo, dev, sha, m, layers, runs)
			if err != nil {
				return err
			}
			run.By = by
			row.Checks = run.Checks
			if run.Result == gatePending {
				row.Detail = "pending: " + pendingOf(run.Checks)
				if slices.ContainsFunc(run.Checks, func(ck GateCheck) bool { return ck.Rerun != "" }) {
					lines = append(lines, run)
				}
				break
			}
			lines, runs = append(lines, run), append(runs, run)
			row.State = run.Result
			o := outcome{m: m.Name, gate: MilestoneGate{State: run.Result, DevSHA: sha, At: run.At}}
			if run.Result == gatePass {
				row.Detail = fmt.Sprintf("passed on dev %s: the PRs of the next milestone may merge", short(sha))
				closes = append(closes, noGoAskID(m.Name))
			} else {
				fails := gateFailures(runs, m.Name)
				row.Detail = fmt.Sprintf("failed on dev %s (%d of %d failed runs before the mission stops): %s", short(sha), len(fails), maxNoGo, failedOf(run.Checks))
				if len(fails) >= maxNoGo {
					o.gate.State, o.stop, row.State = GateStopped, true, GateStopped
					row.Detail = noGoTitle(m.Name, f, len(fails))
					asks = append(asks, noGoAsk(f, m.Name, fails))
				}
			}
			done = append(done, o)
		}
		rep.Milestones = append(rep.Milestones, row)
		earlierPassed = earlierPassed && (m.Gate.State == GatePassed || row.State == GatePassed)
	}
	if len(lines) > 0 && !rep.DryRun {
		var buf bytes.Buffer
		for _, l := range lines {
			b, _ := json.Marshal(l)
			buf.Write(append(b, '\n'))
		}
		for _, a := range asks { // before the stop is recorded: a run whose card cannot be posted is not recorded, and runs again
			if err := e.postAsk(ctx, c, repo, f, a); err != nil {
				return err
			}
		}
		msg := fmt.Sprintf("missions: gate of #%d on dev %s", f, short(sha))
		rep.LedgerCommit, err = e.editPlan(ctx, w, f, msg, map[string][]byte{MissionGatesPath(f): buf.Bytes()}, func(p *MissionPlan) error {
			at := e.Now().UTC().Format(time.RFC3339)
			for _, o := range done {
				i := slices.IndexFunc(p.Milestones, func(m MissionMilestone) bool { return m.Name == o.m })
				if i < 0 {
					continue
				}
				p.Milestones[i].Gate = o.gate
				fails := len(gateFailures(runs, o.m))
				switch {
				case o.stop:
					p.Pause = &MissionPause{Reason: noGoTitle(o.m, f, fails), By: "the gate of " + o.m, At: at}
				case o.gate.State == GatePassed && p.Pause != nil && p.Pause.By == "the gate of "+o.m:
					p.Pause = nil // a person's once-more passed: the mission goes on
				}
			}
			if presence != nil {
				p.Controls = append(p.Controls, MissionControl{Action: "gate once more", By: presence.Login, At: at, Presence: presence.Via})
			}
			plan = p
			return nil
		})
		if err != nil {
			return err
		}
		for _, id := range closes {
			if err := e.closeAsk(ctx, c, repo, f, id, "The gate passed, so the mission goes on."); err != nil {
				return err
			}
		}
	}
	if rep.Fixes, err = e.gateFixItems(ctx, c, w, repo, f, plan, runs, agents); err != nil {
		return err
	}
	return e.Emit(rep, func(w io.Writer) error { return renderGate(w, rep) })
}

func (p *MissionPause) reason() string {
	if p == nil {
		return "its gate failed"
	}
	return p.Reason
}

// runGate runs the gate of milestone m on the dev tip sha: the required
// checks of dev, then, once none of them is pending, the verify sweep and
// the evals of the seats that own the layers of the milestone's stories.
func (e *Env) runGate(ctx context.Context, c *github.Client, g *gateTree, repo, dev, sha string, m MissionMilestone, layers map[int][]string, runs []GateRun) (GateRun, error) {
	run := GateRun{Milestone: m.Name, DevSHA: sha, At: e.Now().UTC().Format(time.RFC3339)}
	rerun := map[string]bool{}
	for _, r := range runs {
		for _, ck := range r.Checks {
			if r.DevSHA == sha && ck.Rerun != "" {
				rerun[ck.Check] = true
			}
		}
	}
	req, err := e.gateRequired(ctx, c, repo, dev, sha, rerun)
	if err != nil {
		return run, err
	}
	run.Checks = req
	if !slices.ContainsFunc(req, func(ck GateCheck) bool { return ck.Result == gatePending }) {
		v, err := e.gateVerify(ctx, g, sha)
		if err != nil {
			return run, err
		}
		var touched []string
		for _, n := range m.Stories {
			for _, l := range layers[n] {
				if ly, ok := verbs.FindLayer(verbs.LayerID(l)); ok && !slices.Contains(touched, string(ly.Owner)) {
					touched = append(touched, string(ly.Owner))
				}
			}
		}
		slices.Sort(touched)
		ev, err := e.gateEvals(ctx, c, repo, dev, sha, touched)
		if err != nil {
			return run, err
		}
		run.Checks = append(append(run.Checks, v...), ev...)
	}
	run.Result = gatePass
	for _, ck := range run.Checks {
		switch {
		case ck.Result == gatePending:
			run.Result = gatePending
		case ck.Result != gatePass && run.Result == gatePass:
			run.Result = gateFail
		}
	}
	return run, nil
}

// gateRequired judges each required check of dev (its rules and classic
// protection, but not the CODEOWNERS check, which runs only on a PR) on sha
// as ship does (checkState). A check that failed for its runner is rerun
// once, and the reason recorded; it fails the gate when it fails again.
func (e *Env) gateRequired(ctx context.Context, c *github.Client, repo, dev, sha string, rerun map[string]bool) ([]GateCheck, error) {
	owner, name, _ := github.SplitRepo(repo)
	required, err := newInboxSource(c, repo, dev, config.Identity{}).requiredOf(ctx, dev)
	if err != nil {
		return nil, Wrap(ExitExternal, err)
	}
	runs, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.CheckRun, *gh.Response, error) {
		r, resp, err := c.REST.Checks.ListCheckRunsForRef(ctx, owner, name, sha, &gh.ListCheckRunsOptions{ListOptions: o})
		return r.GetCheckRuns(), resp, err
	})
	if err != nil {
		return nil, Wrap(ExitExternal, err)
	}
	statuses, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.RepoStatus, *gh.Response, error) {
		return c.REST.Repositories.ListStatuses(ctx, owner, name, sha, &o)
	})
	if err != nil {
		return nil, Wrap(ExitExternal, err)
	}
	var out []GateCheck
	for _, sc := range required {
		key := "check:" + sc.Context
		if sc.Context == seats.GateCheck || slices.ContainsFunc(out, func(ck GateCheck) bool { return ck.Check == key }) {
			continue
		}
		ck := GateCheck{Check: key, Result: gatePass}
		st := checkState(runs, statuses, sc)
		i := slices.IndexFunc(runs, func(r *gh.CheckRun) bool { return r.GetName() == sc.Context })
		why := ""
		switch {
		case st == "success":
		case st == "missing" || st == "queued" || st == "in_progress" || st == "pending" || st == "waiting" || st == "requested":
			ck.Result, ck.Detail = gatePending, fmt.Sprintf("%s is %s on %s", sc.Context, st, short(sha))
		case i >= 0 && (st == "cancelled" || st == "startup_failure"):
			why = "runner error: " + st
		case i >= 0 && st == "failure" && runnerLog.MatchString(jobTail(ctx, c, owner, name, runs[i].GetID())):
			why = "runner error: its log names a 5xx from GitHub"
		default:
			ck.Result, ck.Detail = gateFail, fmt.Sprintf("%s is %s on %s", sc.Context, st, short(sha))
		}
		switch {
		case why != "" && rerun[key]:
			ck.Result, ck.Detail = gateFail, fmt.Sprintf("%s is %s on %s again after its one rerun (%s)", sc.Context, st, short(sha), why)
		case why != "":
			if !c.DryRun() {
				job := runs[i].GetID()
				if _, err := c.Write(ctx, actions.Action{Kind: "github.actions.rerun", Target: fmt.Sprintf("%s@%s", repo, short(sha)),
					Request: &actions.Request{Method: http.MethodPost, Path: fmt.Sprintf("/repos/%s/actions/jobs/%d/rerun", repo, job)},
					Payload: map[string]any{"job": job, "check": sc.Context, "reason": why}},
					func(ctx context.Context) (any, error) {
						_, err := c.REST.Actions.RerunJobByID(ctx, owner, name, job)
						return job, err
					}); err != nil {
					return nil, Wrap(ExitExternal, err)
				}
			}
			ck.Result, ck.Rerun = gatePending, why
			ck.Detail = fmt.Sprintf("%s is %s on %s; aayu reran it once (%s)", sc.Context, st, short(sha), why)
		}
		out = append(out, ck)
	}
	return out, nil
}

// gateTree is a worktree of the dev tip, made once a gate run when the
// default verify or eval runner needs it.
type gateTree struct {
	root, dir, sha string
}

func (g *gateTree) at(ctx context.Context, sha string) (string, error) {
	if g.dir != "" && g.sha == sha {
		return g.dir, nil
	}
	if g.root == "" {
		return "", Errorf(ExitUsage, "mission gate runs the verify sweep in a checkout of the repository")
	}
	if _, err := gitIn(ctx, g.root, "cat-file", "-e", sha+"^{commit}"); err != nil {
		if _, err := gitIn(ctx, g.root, "fetch", "--quiet", "origin", sha); err != nil {
			return "", Wrap(ExitExternal, fmt.Errorf("fetch the dev tip %s: %w", short(sha), err))
		}
	}
	dir, err := os.MkdirTemp("", "aayudha-gate-")
	if err != nil {
		return "", err
	}
	if _, err := gitIn(ctx, g.root, "worktree", "add", "--quiet", "--detach", dir, sha); err != nil {
		_ = os.RemoveAll(dir)
		return "", Wrap(ExitExternal, err)
	}
	g.dir, g.sha = dir, sha
	return dir, nil
}

func (g *gateTree) close() {
	if g.dir != "" {
		_, _ = gitIn(context.Background(), g.root, "worktree", "remove", "--force", g.dir)
		_ = os.RemoveAll(g.dir)
	}
}

// gateVerify is `aayu verify --check all` on the dev tip: one gate check per
// behaviour check, by its feature-map ref.
func (e *Env) gateVerify(ctx context.Context, g *gateTree, sha string) ([]GateCheck, error) {
	if e.GateVerify != nil {
		return e.GateVerify(ctx, sha)
	}
	wt, err := g.at(ctx, sha)
	if err != nil {
		return nil, err
	}
	sub := *e
	sub.Dir, sub.config, sub.JSON = wt, nil, true
	var out bytes.Buffer
	sub.Out, sub.Err = &out, io.Discard
	verr := runVerify(ctx, &sub, Values{"check": verify.SelectAll})
	var res VerifyResult
	if err := json.NewDecoder(&out).Decode(&res); err != nil {
		if verr != nil {
			return nil, verr
		}
		return nil, err
	}
	var checks []GateCheck
	if b, err := os.ReadFile(filepath.Join(res.Artifacts, "reports", "behaviour.json")); err == nil {
		var rep struct {
			Checks []verify.CheckResult `json:"checks"`
		}
		if json.Unmarshal(b, &rep) == nil {
			for _, cr := range rep.Checks {
				ck := GateCheck{Check: "verify:" + cr.Ref, Result: gatePass, Detail: cr.Status}
				if cr.Status != verify.CheckPassed && cr.Status != verify.CheckUnreachable {
					ck.Result, ck.Detail = gateFail, strings.TrimSpace(cr.Status+" "+cr.Reason)
				}
				checks = append(checks, ck)
			}
		}
	}
	if len(checks) == 0 {
		ck := GateCheck{Check: "verify:" + verify.SelectAll, Result: gatePass, Detail: "no behaviour check failed"}
		if !res.Hold {
			ck.Result, ck.Detail = gateFail, failing(res.Checks)
		}
		checks = append(checks, ck)
	}
	return checks, nil
}

// gateEvals are the tier-2 evals of the skills whose verbs the touched seats
// are Accountable for, from evals.yml's results for the dev tip: one gate
// check per case, failed below the bars of evals/eval.toml (min_mean, and
// min_pass_k where the group's pass_k is required). A gating group with no
// results is dispatched once on dev, and the gate waits for it.
func (e *Env) gateEvals(ctx context.Context, c *github.Client, repo, dev, sha string, touched []string) ([]GateCheck, error) {
	if e.GateEvals != nil {
		return e.GateEvals(ctx, sha, touched)
	}
	var skills []string
	for _, v := range verbs.Verbs {
		if slices.Contains(touched, string(v.Seat)) {
			skills = append(skills, v.Name)
		}
	}
	none := []GateCheck{{Check: "eval:" + strings.Join(touched, ","), Result: gatePass, Detail: "no gating eval group covers the skills of these seats"}}
	if len(skills) == 0 {
		return none, nil
	}
	owner, name, _ := github.SplitRepo(repo)
	var cfg struct {
		MinPassK float64 `toml:"min_pass_k"`
		MinMean  float64 `toml:"min_mean"`
		Groups   []struct {
			Name     string   `toml:"name"`
			Prefixes []string `toml:"prefixes"`
			Gate     string   `toml:"gate"`
			PassK    string   `toml:"pass_k"`
		} `toml:"groups"`
	}
	fc, _, _, err := c.REST.Repositories.GetContents(ctx, owner, name, "evals/eval.toml", &gh.RepositoryContentGetOptions{Ref: sha})
	if github.NotFound(err) {
		return none, nil
	}
	if err != nil {
		return nil, Wrap(ExitExternal, err)
	}
	if b, err := fc.GetContent(); err != nil || toml.Unmarshal([]byte(b), &cfg) != nil {
		return nil, Errorf(ExitBlocked, "mission gate: evals/eval.toml at %s cannot be read", short(sha))
	}
	runs, _, err := c.REST.Actions.ListWorkflowRunsByFileName(ctx, owner, name, evalsWorkflow, &gh.ListWorkflowRunsOptions{HeadSHA: sha, ListOptions: gh.ListOptions{PerPage: evalsResultsMinRuns}})
	if err != nil {
		return nil, Wrap(ExitExternal, err)
	}
	var checks []GateCheck
	pattern := "^(" + strings.Join(skills, "|") + ")/"
	for _, grp := range cfg.Groups {
		covers := len(grp.Prefixes) == 0 || slices.ContainsFunc(grp.Prefixes, func(p string) bool {
			skill, _, _ := strings.Cut(p, "/")
			return slices.Contains(skills, skill)
		})
		if grp.Gate == "advisory" || !covers {
			continue
		}
		res, waiting, dispatched, err := e.evalResults(ctx, c, owner, name, sha, grp.Name, runs.WorkflowRuns)
		if err != nil {
			return nil, err
		}
		switch {
		case res != nil:
			judged := len(checks)
			for _, cs := range res.Cases {
				if !slices.Contains(skills, cs.Skill) || cs.NotRun != "" {
					continue
				}
				ck := GateCheck{Check: "eval:" + cs.Case, Result: gatePass, Detail: fmt.Sprintf("mean %.2f, pass^k %.2f", cs.Mean, cs.PassK)}
				if cs.Mean < cfg.MinMean || grp.PassK != "advisory" && cs.PassK < cfg.MinPassK {
					ck.Result = gateFail
					ck.Detail += fmt.Sprintf(" (bars: mean %.2f, pass^k %.2f)", cfg.MinMean, cfg.MinPassK)
				}
				checks = append(checks, ck)
			}
			if len(checks) == judged { // a gating group whose results judge no case of these skills proves nothing
				checks = append(checks, GateCheck{Check: "eval:" + grp.Name, Result: gateFail, Detail: "its results for " + short(sha) + " judge no case of the touched seats' skills"})
			}
		case waiting != "":
			checks = append(checks, GateCheck{Check: "eval:" + grp.Name, Result: gatePending, Detail: waiting})
		case dispatched:
			checks = append(checks, GateCheck{Check: "eval:" + grp.Name, Result: gateFail, Detail: "its dispatched " + evalsWorkflow + " run left no results for " + short(sha)})
		default:
			if !c.DryRun() {
				inputs := map[string]any{"group": grp.Name, "cases": pattern}
				if _, err := c.Write(ctx, actions.Action{Kind: "github.workflow.dispatch", Target: repo + "@" + evalsWorkflow,
					Request: &actions.Request{Method: http.MethodPost, Path: fmt.Sprintf("/repos/%s/actions/workflows/%s/dispatches", repo, evalsWorkflow)},
					Payload: map[string]any{"ref": dev, "inputs": inputs}},
					func(ctx context.Context) (any, error) {
						_, _, err := c.REST.Actions.CreateWorkflowDispatchEventByFileName(ctx, owner, name, evalsWorkflow, gh.CreateWorkflowDispatchEventRequest{Ref: dev, Inputs: inputs})
						return nil, err
					}); err != nil {
					return nil, Wrap(ExitExternal, err)
				}
			}
			checks = append(checks, GateCheck{Check: "eval:" + grp.Name, Result: gatePending, Detail: fmt.Sprintf("dispatched %s on %s for %s", evalsWorkflow, dev, pattern)})
		}
	}
	if len(checks) == 0 {
		return none, nil
	}
	return checks, nil
}

// evalResult is the part of an evals/results file the gate reads.
type evalResult struct {
	Cases []struct {
		Case   string  `json:"case"`
		Skill  string  `json:"skill"`
		PassK  float64 `json:"pass_k"`
		Mean   float64 `json:"mean"`
		NotRun string  `json:"not_run"`
	} `json:"cases"`
}

// evalResults reads group's results for sha from its evals.yml runs: the
// artifact eval-results-<sha>-<group>; else why the gate waits (a run still
// runs), or whether a dispatched run finished with none.
func (e *Env) evalResults(ctx context.Context, c *github.Client, owner, name, sha, group string, runs []*gh.WorkflowRun) (*evalResult, string, bool, error) {
	dispatched := false
	for _, r := range runs {
		if r.GetHeadSHA() != sha {
			continue
		}
		if r.GetStatus() != "completed" {
			return nil, fmt.Sprintf("%s run %s is %s", evalsWorkflow, r.GetHTMLURL(), r.GetStatus()), false, nil
		}
		dispatched = dispatched || r.GetEvent() == "workflow_dispatch"
		arts, _, err := c.REST.Actions.ListWorkflowRunArtifacts(ctx, owner, name, r.GetID(), &gh.ListOptions{PerPage: 100})
		if err != nil {
			return nil, "", false, Wrap(ExitExternal, err)
		}
		for _, a := range arts.Artifacts {
			if a.GetName() != "eval-results-"+sha+"-"+group || a.GetExpired() {
				continue
			}
			u, _, err := c.REST.Actions.DownloadArtifact(ctx, owner, name, a.GetID(), 3)
			if err != nil {
				return nil, "", false, Wrap(ExitExternal, err)
			}
			req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
			resp, err := e.HTTP.Do(req)
			if err != nil {
				return nil, "", false, Wrap(ExitExternal, err)
			}
			zipped, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
			_ = resp.Body.Close()
			if err != nil || resp.StatusCode != http.StatusOK {
				return nil, "", false, Errorf(ExitExternal, "download artifact %d: HTTP %d %v", a.GetID(), resp.StatusCode, err)
			}
			zr, err := zip.NewReader(bytes.NewReader(zipped), int64(len(zipped)))
			if err != nil {
				return nil, "", false, Errorf(ExitExternal, "artifact %d: %v", a.GetID(), err)
			}
			for _, zf := range zr.File {
				if !strings.HasSuffix(zf.Name, ".json") {
					continue
				}
				rc, err := zf.Open()
				if err != nil {
					return nil, "", false, Errorf(ExitExternal, "artifact %d: %v", a.GetID(), err)
				}
				var res evalResult
				err = json.NewDecoder(io.LimitReader(rc, 32<<20)).Decode(&res)
				_ = rc.Close()
				if err != nil {
					return nil, "", false, Errorf(ExitExternal, "artifact %d: %s: %v", a.GetID(), zf.Name, err)
				}
				return &res, "", true, nil
			}
		}
	}
	return nil, "", dispatched, nil
}

// gateRuns reads missions/<f>/gates.jsonl.
func gateRuns(ctx context.Context, w *ledger.Writer, f int) ([]GateRun, error) {
	t, err := w.Read(ctx)
	if err != nil {
		return nil, Wrap(ExitExternal, ignoreMissing(err))
	}
	defer t.Close()
	raw, err := t.File(ctx, MissionGatesPath(f))
	if err := ignoreMissing(err); err != nil {
		return nil, Wrap(ExitExternal, err)
	}
	var out []GateRun
	sc := bufio.NewScanner(bytes.NewReader(raw))
	sc.Buffer(make([]byte, 1<<20), 16<<20)
	for sc.Scan() {
		var r GateRun
		if json.Unmarshal(sc.Bytes(), &r) == nil && r.Milestone != "" {
			out = append(out, r)
		}
	}
	return out, nil
}

// lastGateRun is the newest gate run of milestone m: a passed or failed one,
// or any when pending is set.
func lastGateRun(runs []GateRun, m string, pending bool) *GateRun {
	for i := len(runs) - 1; i >= 0; i-- {
		if runs[i].Milestone == m && (pending || runs[i].Result != gatePending) {
			return &runs[i]
		}
	}
	return nil
}

// gateFailures are the failed gate runs of milestone m, oldest first.
func gateFailures(runs []GateRun, m string) []GateRun {
	var out []GateRun
	for _, r := range runs {
		if r.Milestone == m && r.Result == gateFail {
			out = append(out, r)
		}
	}
	return out
}

func failedOf(checks []GateCheck) string {
	var out []string
	for _, ck := range checks {
		if ck.Result == gateFail {
			out = append(out, ck.Check)
		}
	}
	return strings.Join(out, ", ")
}

func pendingOf(checks []GateCheck) string {
	var out []string
	for _, ck := range checks {
		if ck.Result == gatePending {
			out = append(out, ck.Detail)
		}
	}
	return strings.Join(out, "; ")
}

// openFixes names the fixes of milestone m's last failed run that have not
// merged or closed: a check with no fix PR yet, or an open fix PR.
func openFixes(plan *MissionPlan, m string, last *GateRun, prOf func(int) (*gh.PullRequest, error)) ([]string, error) {
	if last == nil || last.Result != gateFail {
		return nil, nil
	}
	var out []string
	for _, ck := range last.Checks {
		if ck.Result != gateFail {
			continue
		}
		fx := findFix(plan, m, ck.Check, last.DevSHA)
		if fx == nil {
			out = append(out, ck.Check+" (no PR yet)")
			continue
		}
		p, err := prOf(fx.PR)
		if err != nil {
			return nil, err
		}
		if p.GetState() == "open" {
			out = append(out, fmt.Sprintf("#%d", fx.PR))
		}
	}
	return out, nil
}

func findFix(plan *MissionPlan, m, check, sha string) *GateFix {
	for i, fx := range plan.GateFixes {
		if fx.Milestone == m && fx.Check == check && fx.DevSHA == sha {
			return &plan.GateFixes[i]
		}
	}
	return nil
}

// gateFixItems lists the fix of each failed check of each milestone's last
// failed run that is not passed: its recorded PR, a subthread that has it,
// or a start, claimed in one agents slot as mission next claims a story.
func (e *Env) gateFixItems(ctx context.Context, c *github.Client, w *ledger.Writer, repo string, f int, plan *MissionPlan, runs []GateRun, agents int) ([]GateFixItem, error) {
	out := []GateFixItem{}
	t, err := w.Read(ctx)
	if err != nil {
		return nil, Wrap(ExitExternal, ignoreMissing(err))
	}
	raw, err := t.File(ctx, MissionClaimsPath(f))
	t.Close()
	if err := ignoreMissing(err); err != nil {
		return nil, Wrap(ExitExternal, err)
	}
	started := map[int]bool{} // a fix claimed once stays its subthread's, so one failed check gets one fix PR
	for _, cl := range e.allClaims(raw) {
		started[cl.Story] = true
	}
	var busy map[int]bool
	for _, m := range plan.Milestones {
		last := lastGateRun(runs, m.Name, false)
		if m.Gate.State == GatePassed || last == nil || last.Result != gateFail {
			continue
		}
		for _, ck := range last.Checks {
			if ck.Result != gateFail {
				continue
			}
			it := GateFixItem{Milestone: m.Name, Check: ck.Check, DevSHA: last.DevSHA, Action: "waits"}
			key := fixKey(m.Name, ck.Check, last.DevSHA)
			switch fx := findFix(plan, m.Name, ck.Check, last.DevSHA); {
			case fx != nil:
				it.Action, it.PR = "fixing", fx.PR
				if fx.Waits != "" {
					it.Action, it.Reason = "waits", fmt.Sprintf("fix PR #%d would pass #%d's budget; a person decides in aayu inbox", fx.PR, f)
				}
			case started[key]:
				it.Action, it.Reason = "started", "a subthread has it; it records its PR with aayu mission gate --pr"
			case plan.Pause != nil:
				it.Reason = "the mission is stopped: " + plan.Pause.Reason
			default:
				if busy == nil {
					if busy, err = e.missionBusy(ctx, c, w, repo, f, plan); err != nil {
						return nil, err
					}
				}
				if len(busy) >= agents {
					it.Reason = fmt.Sprintf("no free agent (%d of %d busy)", len(busy), agents)
					break
				}
				lost := 0
				if !c.DryRun() {
					if _, lost, err = e.claimSlot(ctx, w, MissionClaimsPath(f), fmt.Sprintf("missions: claim of the fix of %s %s in the mission of #%d", m.Name, ck.Check, f), key, busy, agents, nil); err != nil {
						return nil, err
					}
				}
				busy[key] = true
				switch lost {
				case 0:
					it.Action, it.Brief = "start", fixBrief(f, m.Name, ck, last.DevSHA)
				case key:
					it.Action, it.Reason = "started", "claimed by another run"
				default:
					it.Reason = fmt.Sprintf("no free agent (%d of %d busy)", len(busy), agents)
				}
			}
			out = append(out, it)
		}
	}
	return out, nil
}

// allClaims are every claim line of a claims file that was not released,
// however old: a fix claimed once is never claimed again.
func (e *Env) allClaims(raw []byte) []loopClaim {
	var lines []loopClaim
	released := map[string]bool{}
	sc := bufio.NewScanner(bytes.NewReader(raw))
	for sc.Scan() {
		var c loopClaim
		if json.Unmarshal(sc.Bytes(), &c) != nil {
			continue
		}
		if c.Release {
			released[c.Token] = true
			continue
		}
		lines = append(lines, c)
	}
	return slices.DeleteFunc(lines, func(c loopClaim) bool { return released[c.Token] })
}

// missionBusy are the mission's busy agents slots: its stories and Bugs in
// progress or validation, its live claims (a fix's among them), and its open
// fix PRs.
func (e *Env) missionBusy(ctx context.Context, c *github.Client, w *ledger.Writer, repo string, f int, plan *MissionPlan) (map[int]bool, error) {
	owner, name, _ := github.SplitRepo(repo)
	busy := map[int]bool{}
	claims, err := e.claimLines(ctx, w, MissionClaimsPath(f), nil)
	if err != nil {
		return nil, err
	}
	for _, cl := range claims {
		busy[cl.Story] = true
	}
	for _, it := range plan.Graph {
		is, _, err := c.REST.Issues.Get(ctx, owner, name, it.Number)
		if err != nil {
			return nil, Wrap(ExitExternal, err)
		}
		if l := github.IssueLabels(is); is.GetState() == "open" && (contains(l, labelInProgress) || contains(l, "status:validation")) {
			busy[it.Number] = true
		}
	}
	open, err := openFixPRs(ctx, c, owner, name, plan)
	for _, fx := range open {
		busy[fixKey(fx.Milestone, fx.Check, fx.DevSHA)] = true
	}
	return busy, err
}

// committed is what a fix PR is checked against the budget with: what the
// mission used (missionUsed), plus each open story or Bug that runs or holds
// a claim, at its measured spend or the per-story cap if more, as mission
// next counts it. usedOf computes it for a plan as it is edited.
func (e *Env) committed(ctx context.Context, c *github.Client, w *ledger.Writer, repo string, f int, plan *MissionPlan, appr *ready.Approval) (usedOf func(*MissionPlan) SourcedNumber, costs map[int]float64, err error) {
	owner, name, _ := github.SplitRepo(repo)
	closed, merged := map[int]bool{}, 0
	for _, it := range plan.Graph {
		is, _, err := c.REST.Issues.Get(ctx, owner, name, it.Number)
		if err != nil {
			return nil, nil, Wrap(ExitExternal, err)
		}
		closed[it.Number] = is.GetState() == "closed"
		if closed[it.Number] {
			merged++
		}
	}
	busy, err := e.missionBusy(ctx, c, w, repo, f, plan)
	if err != nil {
		return nil, nil, err
	}
	open, err := openFixSet(ctx, c, repo, plan)
	if err != nil {
		return nil, nil, err
	}
	costs, per := e.costs(repo), appr.Budget.PerStoryUSD
	var held []int
	for n := range busy {
		if n > 0 && !closed[n] { // a fix's slot is negative, and fixSpend counts it
			held = append(held, n)
		}
	}
	slices.Sort(held)
	heldV, notes := 0.0, []string(nil)
	for _, n := range held {
		v := max(per, costs[n])
		heldV += v
		notes = append(notes, fmt.Sprintf("#%d %s", n, ready.USD(v)))
	}
	return func(p *MissionPlan) SourcedNumber {
		u := missionUsed(p, appr, merged, costs, open)
		if len(notes) > 0 {
			u.Value += heldV
			u.Source += "; running or claimed: " + strings.Join(notes, ", ") + " (each at least the per-story cap)"
		}
		return u
	}, costs, nil
}

// openFixPRs are the plan's fix PRs that are still open.
func openFixPRs(ctx context.Context, c *github.Client, owner, name string, plan *MissionPlan) ([]GateFix, error) {
	var out []GateFix
	for _, fx := range plan.GateFixes {
		p, _, err := c.REST.PullRequests.Get(ctx, owner, name, fx.PR)
		if err != nil {
			return nil, Wrap(ExitExternal, err)
		}
		if p.GetState() == "open" {
			out = append(out, fx)
		}
	}
	return out, nil
}

// fixBrief is what the Kanha gives a fix subthread.
func fixBrief(f int, m string, ck GateCheck, sha string) string {
	return fmt.Sprintf("Fix the gate check %s of %s of #%d. It failed on dev %s: %s. "+
		"Branch from origin/dev and open one draft PR to dev. Its body says `Refs #%d` and, on a line of its own, `Aayudha-Gate-Fix: %s %s`. "+
		"File no issue. Then run `aayu mission gate %d --pr <the PR's number>`, and build the fix only when it records the PR.",
		ck.Check, m, f, short(sha), ck.Detail, f, m, ck.Check, f)
}

// missionGateFix is `aayu mission gate <f> --pr <n>`: it records fix PR n in
// the plan's gate_fixes, with the failed check its body names, the dev SHA
// the gate failed on, and the approved record it rests on, by its hash. It
// files no issue and writes no triage record. A fix that would pass the
// Feature's budget waits, and one inbox card asks a person.
func missionGateFix(ctx context.Context, e *Env, f, n int) error {
	repo, c, w, err := e.missionLedger(ctx)
	if err != nil {
		return err
	}
	plan, appr, _, err := readMission(ctx, w, f)
	if err != nil {
		return err
	}
	if plan == nil || appr == nil || plan.StartedBy == "" {
		return Errorf(ExitBlocked, "#%d has no started mission, so it has no gate fix", f)
	}
	owner, name, _ := github.SplitRepo(repo)
	pr, _, err := c.REST.PullRequests.Get(ctx, owner, name, n)
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	mm := gateFixLine.FindStringSubmatch(pr.GetBody())
	switch {
	case pr.GetState() != "open" || pr.GetBase().GetRef() != e.devBranch():
		return Errorf(ExitBlocked, "#%d is not an open PR to %s: a gate fix is one", n, e.devBranch())
	case !regexp.MustCompile(`(?i)\bRefs #` + strconv.Itoa(f) + `\b`).MatchString(pr.GetBody()):
		return Errorf(ExitBlocked, "#%d does not say `Refs #%d`: a gate fix PR of #%d says it", n, f, f)
	case mm == nil:
		return Errorf(ExitBlocked, "#%d names no failed check: its body needs the line `Aayudha-Gate-Fix: <milestone> <check>`", n)
	}
	runs, err := gateRuns(ctx, w, f)
	if err != nil {
		return err
	}
	ms, check := mm[1], mm[2]
	last := lastGateRun(runs, ms, false)
	if last == nil || last.Result != gateFail || !slices.ContainsFunc(last.Checks, func(ck GateCheck) bool { return ck.Check == check && ck.Result == gateFail }) {
		return Errorf(ExitBlocked, "%s did not fail in the last gate of %s of #%d: a fix PR comes only from a failed gate check (Decision #974)", check, ms, f)
	}
	if fx := findFix(plan, ms, check, last.DevSHA); fx != nil && fx.PR != n {
		return Errorf(ExitBlocked, "%s of %s has its fix PR already, #%d: one failed check gets one fix PR", check, ms, fx.PR)
	}
	t, err := w.Read(ctx)
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	record, err := t.File(ctx, ready.ApprovalPath(f))
	t.Close()
	if err != nil || record == nil {
		return Errorf(ExitBlocked, "ledger %s cannot be read: %v", ready.ApprovalPath(f), err)
	}
	sum := sha256.Sum256(record)
	fix := GateFix{PR: n, Milestone: ms, Check: check, DevSHA: last.DevSHA, Approval: GateApproval{Record: ready.ApprovalPath(f), SHA256: hex.EncodeToString(sum[:])},
		At: e.Now().UTC().Format(time.RFC3339)}
	// The budget: what the mission used and holds, with this fix at the per-story cap, until #872 measures spend.
	usedOf, costs, err := e.committed(ctx, c, w, repo, f, plan, appr)
	if err != nil {
		return err
	}
	used, est := usedOf(plan), appr.Budget.PerStoryUSD
	if fx := findFix(plan, ms, check, last.DevSHA); fx != nil {
		fix = *fx
	} else if plan.Budget > 0 && used.Value+est > plan.Budget {
		fix.Waits = "budget"
	}
	rep := GateReport{Schema: MissionGateSchema, Repo: repo, Feature: f, DevSHA: last.DevSHA, Milestones: []GateMSState{}, Fixes: []GateFixItem{}, Recorded: &fix, DryRun: c.DryRun()}
	if !rep.DryRun && findFix(plan, ms, check, last.DevSHA) == nil {
		if fix.Waits != "" { // before the fix is recorded: a fix whose card cannot be posted is not recorded, and is asked again
			if err := e.postAsk(ctx, c, repo, f, budgetAsk(f, n, plan.Budget, used, est, plan.GateFixes, costs)); err != nil {
				return err
			}
		}
		rep.LedgerCommit, err = e.editPlan(ctx, w, f, fmt.Sprintf("missions: fix PR #%d of %s %s in the mission of #%d", n, ms, check, f), nil, func(p *MissionPlan) error {
			if fx := findFix(p, ms, check, last.DevSHA); fx != nil && fx.PR != n {
				return Errorf(ExitBlocked, "%s of %s has its fix PR already, #%d: one failed check gets one fix PR", check, ms, fx.PR)
			}
			if findFix(p, ms, check, last.DevSHA) == nil {
				p.GateFixes = append(p.GateFixes, fix)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	if err := e.Emit(rep, func(w io.Writer) error {
		if fix.Waits != "" {
			_, err := fmt.Fprintf(w, "Fix PR #%d of %s %s waits: it would pass #%d's budget (%s used + %s, the fix's estimate, > %s). A person decides in aayu inbox.\n",
				n, ms, check, f, ready.USD(used.Value), ready.USD(est), ready.USD(plan.Budget))
			return err
		}
		_, err := fmt.Fprintf(w, "Fix PR #%d is the fix of %s %s (dev %s) in the mission of #%d: build it.\n", n, ms, check, short(last.DevSHA), f)
		return err
	}); err != nil {
		return err
	}
	if fix.Waits != "" {
		return Errorf(ExitBudget, "fix PR #%d waits for a person: it would pass #%d's budget", n, f)
	}
	return nil
}

// editPlan writes Feature f's mission plan as edit leaves it, read again at
// the ledger tip, with appends, in one commit; a write that loses a race
// reads and edits again.
func (e *Env) editPlan(ctx context.Context, w *ledger.Writer, f int, msg string, appends map[string][]byte, edit func(*MissionPlan) error) (string, error) {
	for attempt := 0; ; attempt++ {
		plan, _, tip, err := readMission(ctx, w, f)
		if err != nil {
			return "", err
		}
		if plan == nil {
			return "", Errorf(ExitBlocked, "#%d has no mission plan", f)
		}
		if err := edit(plan); err != nil {
			return "", err
		}
		data, _ := json.MarshalIndent(plan, "", "  ")
		commit, err := w.Write(ctx, ledger.Commit{Message: msg, Files: map[string][]byte{MissionPlanPath(f): append(data, '\n')}, Append: appends, Base: tip})
		if errors.Is(err, ledger.ErrMoved) && attempt < 3 {
			continue
		}
		if err != nil {
			return "", Wrap(ExitExternal, err)
		}
		return commit, nil
	}
}

func renderGate(w io.Writer, rep GateReport) error {
	fmt.Fprintf(w, "Gate of #%d on dev %s\n", rep.Feature, short(rep.DevSHA))
	for _, m := range rep.Milestones {
		fmt.Fprintf(w, "%-9s %s\n", m.Name, m.Detail)
		for _, ck := range m.Checks {
			line := fmt.Sprintf("          %s %s", ck.Result, ck.Check)
			if ck.Result != gatePass && ck.Detail != "" {
				line += ": " + ck.Detail
			}
			fmt.Fprintln(w, line)
		}
	}
	for _, it := range rep.Fixes {
		line := fmt.Sprintf("%s fix %s %s", it.Action, it.Milestone, it.Check)
		if it.PR != 0 {
			line += fmt.Sprintf(" · PR #%d", it.PR)
		}
		if it.Reason != "" {
			line += ": " + it.Reason
		}
		fmt.Fprintln(w, line)
	}
	if rep.DryRun {
		fmt.Fprintln(w, "  --dry-run: nothing written, no check rerun, and no fix claimed")
	}
	return nil
}

// missionBudget is `aayu mission budget <f>`, a person's answer to a fix PR
// that would pass the budget: --to raises the budget, with a reason, after
// the presence check, and lets each waiting fix that now fits go on; --keep
// keeps a fix PR waiting and closes its card.
func missionBudget(ctx context.Context, e *Env, f int, in Values) error {
	if err := e.laneRefusal("mission budget"); err != nil {
		return err
	}
	repo, c, w, err := e.missionLedger(ctx)
	if err != nil {
		return err
	}
	plan, appr, _, err := readMission(ctx, w, f)
	if err != nil {
		return err
	}
	if plan == nil || appr == nil || plan.StartedBy == "" {
		return Errorf(ExitBlocked, "#%d has no started mission, so it has no budget to change", f)
	}
	if in.Has("keep") {
		n := issueNumber(in.String("keep"))
		if !slices.ContainsFunc(plan.GateFixes, func(fx GateFix) bool { return fx.PR == n && fx.Waits != "" }) {
			return Errorf(ExitBlocked, "#%d is no gate fix of #%d that waits for the budget", n, f)
		}
		if err := e.closeAsk(ctx, c, repo, f, budgetAskID(n), fmt.Sprintf("#%d keeps waiting: the budget stays %s.", n, ready.USD(plan.Budget))); err != nil {
			return err
		}
		return e.Emit(map[string]any{"feature": f, "kept": n}, func(w io.Writer) error {
			_, err := fmt.Fprintf(w, "Fix PR #%d of #%d keeps waiting; the budget stays %s.\n", n, f, ready.USD(plan.Budget))
			return err
		})
	}
	to, reason := float64(in.Int("to")), strings.TrimSpace(in.String("reason"))
	switch {
	case to <= plan.Budget:
		return Errorf(ExitUsage, "mission budget --to: give a budget above %s, the budget now", ready.USD(plan.Budget))
	case reason == "":
		return Errorf(ExitUsage, "mission budget --to needs --reason: a raise is a person's decision with a reason")
	}
	p, err := e.presenceCheck(ctx, Pending{What: "mission budget", Effect: []string{
		fmt.Sprintf("Raises the budget of #%d from %s to %s: %s", f, ready.USD(plan.Budget), ready.USD(to), reason),
		"Each gate fix PR that waits for the budget and now fits goes on",
	}}, AccessWrite, e.personOnly("mission budget"))
	if err != nil {
		return err
	}
	usedOf, costs, err := e.committed(ctx, c, w, repo, f, plan, appr)
	if err != nil {
		return err
	}
	var freed []int
	commit, err := e.editPlan(ctx, w, f, fmt.Sprintf("missions: @%s raised the budget of #%d to %s", p.Login, f, ready.USD(to)), nil, func(pl *MissionPlan) error {
		at := e.Now().UTC().Format(time.RFC3339)
		pl.BudgetBasis = fmt.Sprintf("raised from %s to %s by @%s at %s: %s; before: %s", ready.USD(pl.Budget), ready.USD(to), p.Login, at, reason, pl.BudgetBasis)
		pl.Budget = to
		pl.Controls = append(pl.Controls, MissionControl{Action: "budget", By: p.Login, At: at, Presence: p.Via, Reason: reason})
		used := usedOf(pl).Value
		freed = nil
		for i := range pl.GateFixes { // a freed fix counts as the others do: an open one at least at the cap
			fx := &pl.GateFixes[i]
			if v := max(costs[fx.PR], appr.Budget.PerStoryUSD); fx.Waits == "budget" && used+v <= to {
				fx.Waits, used = "", used+v
				freed = append(freed, fx.PR)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	for _, n := range freed {
		if err := e.closeAsk(ctx, c, repo, f, budgetAskID(n), fmt.Sprintf("@%s raised the budget to %s: %s. #%d goes on.", p.Login, ready.USD(to), reason, n)); err != nil {
			return err
		}
	}
	return e.Emit(map[string]any{"feature": f, "budget": to, "freed": freed, "ledger_commit": commit}, func(w io.Writer) error {
		_, err := fmt.Fprintf(w, "@%s raised the budget of #%d to %s; fix PRs that go on: %s\n", p.Login, f, ready.USD(to), issueRefs(freed))
		return err
	})
}
