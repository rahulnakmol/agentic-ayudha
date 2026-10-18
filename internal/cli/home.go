package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/doctor"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/hosts"
	"github.com/rahulnakmol/agentic-ayudha/internal/identity"
	"github.com/rahulnakmol/agentic-ayudha/internal/journey"
	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

func init() { register("next", runNext) }

// Home is what `aayu` with no command prints and `--json` returns (WP1.1b).
type Home struct {
	journey.State
	Next journey.Step `json:"next"`
	At   string       `json:"at"` // when the facts were gathered
}

// homeCacheFile is the last home screen, which host status lines and plugins
// read without asking GitHub (`aayu hook status-line`).
func (e *Env) homeCacheFile() string { return filepath.Join(e.stateDir(), "state", "journey.json") }

// runHome is the root command: the home screen.
func runHome(ctx context.Context, e *Env) error {
	h := e.gatherHome(ctx)
	return e.Emit(h, func(w io.Writer) error {
		journey.Render(e.Look(), h.State, h.Next)
		return nil
	})
}

func runNext(ctx context.Context, e *Env, _ Values) error {
	h := e.gatherHome(ctx)
	return e.Emit(h.Next, func(w io.Writer) error {
		journey.RenderNext(e.Look(), h.Next, e.Agent())
		return nil
	})
}

// gatherHome reads what the checkout and GitHub know. GitHub failures never
// fail the home screen: it says so and shows the local facts.
func (e *Env) gatherHome(ctx context.Context) Home {
	s := journey.State{Ready: []journey.Item{}, InProgress: []journey.Item{}, Blocked: []journey.Item{}, AwaitingHuman: []journey.Item{},
		FailingFloors: []journey.Floor{}, MeasureDue: []journey.Due{}, Hosts: []string{}, Unclassified: []journey.Item{}, Inbox: []journey.Item{}}
	s.Host, _ = doctor.DetectHost(e.Host, e.Getenv)
	root := config.RepoRoot(e.Dir)
	s.InRepo = root != ""
	if s.InRepo {
		_, err := os.Stat(filepath.Join(root, ".aayudha", "config.toml"))
		s.Configured = err == nil
		_, err = os.Stat(filepath.Join(root, ".github", "workflows", "aayudha-merge-gate.yml"))
		s.Gates = err == nil
		if l, ok, _ := hosts.ReadLock(root); ok {
			s.Hosts = append(s.Hosts, l.Hosts...)
		}
		s.OutOfSync = hosts.CheckCopies(root)
		_, err = os.Stat(e.PauseFile())
		s.Paused = err == nil
		s.FailingFloors = localFloorMisses(root)
		s.MeasureDue = e.dueMeasurements(ctx, root)
	}
	s.GitHub = journey.GitHubNoRepo
	if repo, err := e.RepoName(ctx); err == nil {
		s.Repo = repo
		gctx, cancel := context.WithTimeout(ctx, 20*time.Second)
		s.GitHub = e.githubHome(gctx, repo, &s)
		cancel()
	}
	h := Home{State: s, Next: journey.Next(s, e.Agent()), At: e.Now().UTC().Format(time.RFC3339)}
	if s.InRepo {
		if b, err := json.MarshalIndent(h, "", "  "); err == nil {
			_ = os.MkdirAll(filepath.Dir(e.homeCacheFile()), 0o755)
			_ = os.WriteFile(e.homeCacheFile(), append(b, '\n'), 0o644)
		}
	}
	return h
}

// githubHome fills the GitHub facts and returns GitHub's reachability.
func (e *Env) githubHome(ctx context.Context, repo string, s *journey.State) string {
	tok, err := e.Resolver().Human(ctx)
	if err != nil {
		if errors.Is(err, identity.ErrNoToken) {
			return journey.GitHubNoToken
		}
		return err.Error()
	}
	c, err := e.newGitHub(tok, "human", "")
	if err != nil {
		return err.Error()
	}
	owner, name, _ := github.SplitRepo(repo)
	byLabel := func(label string) ([]journey.Item, error) {
		is, _, err := c.REST.Issues.ListByRepo(ctx, owner, name, &gh.IssueListByRepoOptions{State: "open", Labels: []string{label}, ListOptions: gh.ListOptions{PerPage: 100}})
		if err != nil {
			return nil, err
		}
		out := []journey.Item{}
		for _, i := range is {
			if i.IsPullRequest() || (!hasLabel(i, "type:story") && !hasLabel(i, "type:bug")) {
				continue
			}
			out = append(out, journey.Item{Number: i.GetNumber(), Title: i.GetTitle()})
		}
		sort.Slice(out, func(a, b int) bool { return out[a].Number < out[b].Number })
		return out, nil
	}
	for label, dst := range map[string]*[]journey.Item{"status:ready": &s.Ready, "status:in-progress": &s.InProgress, "status:blocked": &s.Blocked} {
		if *dst, err = byLabel(label); err != nil {
			return "unreachable (" + shortErr(err) + ")"
		}
	}
	fbs, _, err := c.REST.Issues.ListByRepo(ctx, owner, name, &gh.IssueListByRepoOptions{State: "open", Labels: []string{"type:feedback"}, ListOptions: gh.ListOptions{PerPage: 100}})
	if err != nil {
		return "unreachable (" + shortErr(err) + ")"
	}
	for _, i := range fbs {
		if !i.IsPullRequest() && workitems.FixLayerOf(github.IssueLabels(i)) == "" {
			s.Unclassified = append(s.Unclassified, journey.Item{Number: i.GetNumber(), Title: i.GetTitle()})
		}
	}
	// The inbox (Story #431) is best effort: the home screen stands without it.
	var id config.Identity
	if l, err := e.Config(); err == nil {
		id = l.Config.Identity
	}
	if items, err := gatherInbox(ctx, c, repo, e.devBranch(), id); err == nil {
		for _, it := range items {
			s.Inbox = append(s.Inbox, journey.Item{Number: it.Number, Title: it.Title, Detail: it.Kind})
		}
	}
	prs, _, err := c.REST.PullRequests.List(ctx, owner, name, &gh.PullRequestListOptions{State: "open", ListOptions: gh.ListOptions{PerPage: 30}})
	if err != nil {
		return "unreachable (" + shortErr(err) + ")"
	}
	sort.Slice(prs, func(a, b int) bool { return prs[a].GetNumber() < prs[b].GetNumber() })
	checked := 0
	for _, pr := range prs {
		story := prStory(pr)
		if pr.GetDraft() || story == 0 || checked == 10 {
			continue
		}
		checked++
		reviews, _, err := c.REST.PullRequests.ListReviews(ctx, owner, name, pr.GetNumber(), &gh.ListOptions{PerPage: 100})
		if err != nil {
			return "unreachable (" + shortErr(err) + ")"
		}
		if humanApproved(reviews) {
			continue
		}
		gate := "none"
		if s.Gates {
			gate = "pending"
			if runs, _, err := c.REST.Checks.ListCheckRunsForRef(ctx, owner, name, pr.GetHead().GetSHA(), &gh.ListCheckRunsOptions{ListOptions: gh.ListOptions{PerPage: 100}}); err == nil {
				gate = gateState(runs.CheckRuns)
			}
		}
		s.AwaitingHuman = append(s.AwaitingHuman, journey.Item{Number: pr.GetNumber(), Title: pr.GetTitle(), Story: story, Gate: gate})
	}
	return journey.GitHubOK
}

// gateCheckRuns drops, from a head's check runs (newest first), those of the
// merge gate's reply runs (#690, #643). A comment or review on the PR runs the
// workflow with only aayudha/inspect-reply, and its skipped merge-gate,
// inspect, and pack checks must not hide the gate's own: a check suite is a
// reply run when its aayudha/inspect-reply check was not skipped, which the
// gate's pull_request_target run always skips.
func gateCheckRuns(runs []*gh.CheckRun) []*gh.CheckRun {
	reply := map[int64]bool{}
	for _, r := range runs {
		if r.GetName() == "aayudha/inspect-reply" && r.GetConclusion() != "skipped" {
			reply[r.GetCheckSuite().GetID()] = true
		}
	}
	var kept []*gh.CheckRun
	for _, r := range runs {
		if !reply[r.GetCheckSuite().GetID()] {
			kept = append(kept, r)
		}
	}
	return kept
}

// gateState is the merge gate as ship reads it (gateChecks, the newest run of
// each name first, reply runs dropped): passed when every gate check succeeded,
// failed when one finished otherwise, else pending.
func gateState(runs []*gh.CheckRun) string {
	state, runs := "passed", gateCheckRuns(runs)
	for _, want := range gateChecks {
		concl := "pending"
		for _, r := range runs {
			if r.GetName() == want {
				if r.GetStatus() == "completed" {
					concl = r.GetConclusion()
				}
				break
			}
		}
		switch {
		case concl == "pending":
			if state == "passed" {
				state = "pending"
			}
		case concl != "success":
			return "failed"
		}
	}
	return state
}

func hasLabel(i *gh.Issue, name string) bool {
	for _, l := range i.Labels {
		if l.GetName() == name {
			return true
		}
	}
	return false
}

// prStory is the story a pull request serves, by the convention build and the
// merge gate use (StoryOf; the head commit's trailer is not read here).
func prStory(pr *gh.PullRequest) int { return StoryOf(pr.GetBody(), "", pr.GetHead().GetRef()) }

func atoiOr(s string) int {
	n := 0
	_, _ = fmt.Sscan(s, &n)
	return n
}

// humanApproved reports whether the latest decisive review of some person is
// an approval. Bot accounts never count (WP1.4).
func humanApproved(rs []*gh.PullRequestReview) bool {
	last := map[string]string{}
	for _, r := range rs {
		if r.GetUser().GetType() == "Bot" || strings.HasSuffix(r.GetUser().GetLogin(), "[bot]") {
			continue
		}
		if st := r.GetState(); st == "APPROVED" || st == "CHANGES_REQUESTED" || st == "DISMISSED" {
			last[r.GetUser().GetLogin()] = st
		}
	}
	for _, st := range last {
		if st == "APPROVED" {
			return true
		}
	}
	return false
}

func shortErr(err error) string {
	msg := err.Error()
	if i := strings.Index(msg, ": "); i > 0 && len(msg) > 80 {
		msg = msg[:80] + "…"
	}
	return msg
}

// localFloorMisses reads the local proof packs (.aayudha/proof/<story>/) and
// lists every floor that failed or was blocked.
func localFloorMisses(root string) []journey.Floor {
	out := []journey.Floor{}
	files, _ := filepath.Glob(filepath.Join(root, ".aayudha", "proof", "*", proof.FileName))
	sort.Strings(files)
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		p, err := proof.Decode(b)
		if err != nil {
			continue
		}
		story := atoiOr(filepath.Base(filepath.Dir(f)))
		for _, c := range p.Checks {
			if c.Result == proof.Fail || c.Result == proof.Blocked {
				out = append(out, journey.Floor{Story: story, Floor: c.Floor, Result: c.Result, Detail: c.Detail})
			}
		}
	}
	return out
}

// dueMeasurements reads measure/due/ from the ledger branch the checkout has
// (origin/ledger, else ledger) without fetching: release writes one record
// per Goal or Initiative and release (WP7.8).
func (e *Env) dueMeasurements(ctx context.Context, root string) []journey.Due {
	out := []journey.Due{}
	today := e.Now().UTC().Format(time.DateOnly)
	for _, ref := range []string{"refs/remotes/origin/ledger", "refs/heads/ledger"} {
		names, err := e.exec()(ctx, root, "git", "ls-tree", "-r", "--name-only", ref, "--", "measure/due")
		if err != nil {
			continue
		}
		for _, n := range strings.Fields(names) {
			b, err := e.exec()(ctx, root, "git", "show", ref+":"+n)
			if err != nil {
				continue
			}
			var d MeasureDue
			if json.Unmarshal([]byte(b), &d) != nil || d.Due > today {
				continue
			}
			out = append(out, journey.Due{Issue: d.Issue, Type: d.Type, Due: d.Due, Tag: d.Tag})
		}
		break
	}
	return out
}
