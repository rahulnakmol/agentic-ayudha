package autonomy

import (
	"cmp"
	"context"
	"io"
	"net/http"
	"regexp"
	"slices"
	"strconv"
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
// the review line are all unknown.
func GateRung(ctx context.Context, c *github.Client, owner, name string, n int, head string) string {
	path := ".github/workflows/" + MergeGateWorkflow
	runs, _, err := c.REST.Actions.ListWorkflowRunsByFileName(ctx, owner, name, MergeGateWorkflow, &gh.ListWorkflowRunsOptions{HeadSHA: head, Event: "pull_request_target", ListOptions: gh.ListOptions{PerPage: 20}})
	if err != nil {
		return ""
	}
	slices.SortFunc(runs.WorkflowRuns, func(a, b *gh.WorkflowRun) int { return cmp.Compare(b.GetID(), a.GetID()) })
	for _, r := range runs.WorkflowRuns {
		if r.GetPath() != path || r.GetEvent() != "pull_request_target" || r.GetHeadSHA() != head || r.GetStatus() != "completed" {
			continue
		}
		jobs, _, err := c.REST.Actions.ListWorkflowJobs(ctx, owner, name, r.GetID(), &gh.ListWorkflowJobsOptions{Filter: "latest"})
		if err != nil {
			return ""
		}
		for _, j := range jobs.Jobs {
			if j.GetName() == "aayudha/inspect" && j.GetConclusion() == "success" {
				// The first review line: aayu prints it before any text a
				// reviewer wrote (findings, notes), so none can stand in for it.
				log := JobLog(ctx, c, owner, name, j.GetID())
				h, m := inspectHeader.FindStringSubmatch(log), reviewLine.FindStringSubmatch(log)
				if h == nil || h[1] != strconv.Itoa(n) || m == nil {
					return ""
				}
				return m[1] + m[2] + m[3]
			}
		}
		return ""
	}
	return ""
}

// Proven reports whether a review at rung counts toward the streak and the
// autonomous pass: a different-family review (A, or B through a paid key) or
// a person's. A same-family review (C) or an unknown rung never does.
func Proven(rung string) bool {
	return rung == inspect.RungA || rung == inspect.RungB || rung == inspect.RungHuman
}

// inspectHeader is the first line aayu inspect prints, naming the PR, and
// reviewLine its review label, in the merge gate's log (an Actions log line
// starts with its timestamp).
var (
	inspectHeader = regexp.MustCompile(`(?m)^(?:\S+Z )?inspect #([0-9]+) at [0-9a-f]+: `)
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
