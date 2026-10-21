package autonomy

import (
	"cmp"
	"context"
	"io"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/inspect"
)

// MergeGateWorkflow is the merge gate's workflow file, whose aayudha/inspect
// job prints the review's rung.
const MergeGateWorkflow = "aayudha-merge-gate.yml"

// GateRung is the review ladder's rung of PR n's review at head (Decision
// #469): inspect.RungA, RungB, RungC, or RungHuman, or "" when unknown. It is
// read only from the log of the aayudha/inspect job in the newest completed
// pull_request_target run of the base branch's merge gate on that head: the
// workflow file and the aayu that wrote the log are the base branch's, so
// neither the pull request nor a workflow on its branch can write it, as a
// comment could be. No such run or job, a job that did not succeed, a log
// whose header names another PR (two PRs can share a head), or a log without
// the review line are all unknown. A run whose judgement was cached is
// skipped for the newest fresh one.
func GateRung(ctx context.Context, c *github.Client, owner, name string, n int, head string) string {
	rung, _ := GateRungFresh(ctx, c, owner, name, n, head, "")
	return rung
}

// GateRungFresh is GateRung, and whether that rung is a fresh judgement:
// false for a cached C, which stands because it only lowers, but which a
// workflow on a branch could also have written, so it grants nothing. With
// base set, it reads only runs GitHub lists as for PR n into base: a helper
// pull request into a branch with an edited gate cannot stand in.
func GateRungFresh(ctx context.Context, c *github.Client, owner, name string, n int, head, base string) (rung string, fresh bool) {
	path := ".github/workflows/" + MergeGateWorkflow
	// Every page: cached runs on an unchanged head can push the fresh one past the first.
	runs, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.WorkflowRun, *gh.Response, error) {
		rs, resp, err := c.REST.Actions.ListWorkflowRunsByFileName(ctx, owner, name, MergeGateWorkflow, &gh.ListWorkflowRunsOptions{HeadSHA: head, Event: "pull_request_target", ListOptions: o})
		return rs.GetWorkflowRuns(), resp, err
	})
	if err != nil {
		return "", false
	}
	slices.SortFunc(runs, func(a, b *gh.WorkflowRun) int { return cmp.Compare(b.GetID(), a.GetID()) })
	cached := false
	for _, r := range runs {
		if r.GetPath() != path || r.GetEvent() != "pull_request_target" || r.GetHeadSHA() != head || r.GetStatus() != "completed" ||
			base != "" && !RunFor(r, n, base) {
			continue
		}
		jobs, _, err := c.REST.Actions.ListWorkflowJobs(ctx, owner, name, r.GetID(), &gh.ListWorkflowJobsOptions{Filter: "latest"})
		if err != nil {
			return "", false
		}
		for _, j := range jobs.Jobs {
			if j.GetName() == "aayudha/inspect" && j.GetConclusion() == "success" {
				// The first review line: aayu prints it before any text a
				// reviewer wrote (findings, notes), so none can stand in for it.
				log := JobLog(ctx, c, owner, name, j.GetID())
				h, m := inspectHeader.FindStringSubmatch(log), reviewLine.FindStringSubmatch(log)
				if h == nil || h[1] != strconv.Itoa(n) || !strings.HasPrefix(head, h[2]) || m == nil {
					return "", false
				}
				// A cached judgement comes from the state in a PR comment,
				// which a workflow on a branch can also write as the gate's
				// account: a cached A or B proves nothing, so the review is
				// the newest fresh judgement of this head, in an older run.
				// A cached rung that does not count (a relabel to C after the
				// builder's declaration changed) stands: it only lowers.
				rung := m[1] + m[2] + m[3]
				if h[3] != "" && Proven(rung) {
					cached = true
					break
				}
				return rung, h[3] == ""
			}
		}
		if cached {
			cached = false
			continue
		}
		return "", false
	}
	return "", false
}

// RunFor reports whether GitHub lists workflow run r as for PR n into base,
// and for no other PR. GitHub lists every open PR whose head matches the
// run's, so a helper PR's run into a branch with an edited gate can also list
// PR n: a run listed for several is not trusted as n's.
func RunFor(r *gh.WorkflowRun, n int, base string) bool {
	if len(r.PullRequests) != 1 {
		return false
	}
	p := r.PullRequests[0]
	return p.GetNumber() == n && p.GetBase().GetRef() == base
}

// Proven reports whether a review at rung counts toward the streak and the
// streak's autonomous pass: a different-family review (A, or B through a paid
// key) or a person's. A same-family review (C) or an unknown rung never does;
// merge on proof reads [autonomy] proof_rungs instead, which may list C.
func Proven(rung string) bool {
	return rung == inspect.RungA || rung == inspect.RungB || rung == inspect.RungHuman
}

// inspectHeader is the first line aayu inspect prints, naming the PR, and
// reviewLine its review label, in the merge gate's log (an Actions log line
// starts with its timestamp).
var (
	inspectHeader = regexp.MustCompile(`(?m)^(?:\S+Z )?inspect #([0-9]+) at ([0-9a-f]+): \S+( \(cached\))?`)
	reviewLine    = regexp.MustCompile(`(?m)^(?:\S+Z )?\s*review: (?:different-family review \((A|B)[,)]|same-family review \((C)\)|(human) review)`)
)

// JobLog is an Actions job's log (up to 8 MiB), or "" when it cannot be read.
func JobLog(ctx context.Context, c *github.Client, owner, name string, job int64) string {
	u, _, err := c.REST.Actions.GetWorkflowJobLogs(ctx, owner, name, job, 2)
	if err != nil {
		return ""
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return ""
	}
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return ""
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	return string(b)
}
