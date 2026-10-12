package cli

import (
	"archive/zip"
	"bytes"
	"context"
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
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/identity"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
	"github.com/rahulnakmol/agentic-ayudha/internal/ready"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

func init() { register("ship", runShip) }

// Gate checks ship requires on the PR head (the merge gate's jobs, WP4.8).
var gateChecks = []string{"aayudha/merge-gate", "aayudha/inspect"}

// SoloBypassPrefix starts the PR comment in which the merging human records a
// solo-mode bypass (Appendix B D2).
const SoloBypassPrefix = "Solo-mode bypass:"

// HumanPass records how the human pass was met.
type HumanPass struct {
	Kind     string `json:"kind"` // review, or solo-bypass
	Login    string `json:"login"`
	ReviewID int64  `json:"review_id,omitempty"`
	Comment  string `json:"comment_url,omitempty"`
	Commit   string `json:"commit"`
	Note     string `json:"note,omitempty"`
}

// ShipRecord is proof/<story>/<head>/authoritative/ship.json on the ledger.
type ShipRecord struct {
	Repo        string    `json:"repo"`
	Story       int       `json:"story"`
	PR          int       `json:"pr"`
	Head        string    `json:"pr_head_sha"`
	MergeCommit string    `json:"merge_commit"`
	Pack        string    `json:"pack"`
	PackLanded  string    `json:"pack_landed,omitempty"` // how the authoritative pack reached the ledger
	HumanPass   HumanPass `json:"human_pass"`
	Waivers     []string  `json:"waivers,omitempty"`
	Actions     string    `json:"actions_segment"`
	ShippedAt   string    `json:"shipped_at"`
	ShippedBy   string    `json:"shipped_by"`
}

// ShipResult is what ship reports.
type ShipResult struct {
	ShipRecord
	Record string `json:"ship_record"`
	Branch string `json:"branch_deleted,omitempty"`
}

// runShip is the thin-slice ship (WP4.10): a green merge gate, an
// authoritative pack that verifies and whose floors hold, one approving
// review from a human who is not the author, then squash-merge, the Complete
// section, status:merged, and cleanup. Autonomy, streaks, and CODEOWNER
// rules for never-layers are WP5.5/WP5.7.
func runShip(ctx context.Context, e *Env, in Values) error {
	if in.Bool("solo-bypass") && e.Agent() {
		return Errorf(ExitBlocked, "ship --solo-bypass is a human's decision; it is refused for agents")
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
	story := StoryOf(pr.GetBody(), "", pr.GetHead().GetRef())
	if story == 0 {
		return Errorf(ExitBlocked, "ship #%d: no story (Refs #n in the PR body); ship proves a story", n)
	}
	rec := ShipRecord{Repo: repo, Story: story, PR: n, Head: head, ShippedBy: me.Login}

	// 1. The merge gate is green on the head.
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

	// 2. An authoritative pack for the head that verifies and whose floors hold.
	s, err := e.proofStore(ctx, c, tok, "human", "")
	if err != nil {
		return err
	}
	t, err := s.Ledger.Read(ctx)
	if err != nil {
		return ledgerErr(err)
	}
	entry, ok, err := authoritative(ctx, s, t, story, head)
	t.Close()
	if err != nil {
		return err
	}
	if !ok {
		if rec.PackLanded, err = e.landGatePack(ctx, c, s, owner, name, story, head, me.Login); err != nil {
			return err
		}
		if t, err = s.Ledger.Read(ctx); err != nil {
			return ledgerErr(err)
		}
		entry, ok, err = authoritative(ctx, s, t, story, head)
		t.Close()
		if err != nil || !ok {
			return Errorf(ExitBlocked, "ship #%d: no authoritative pack for %s after landing the gate's run artifact (%v)", n, short(head), err)
		}
	}
	rec.Pack = entry.Path
	if t, err = s.Ledger.Read(ctx); err != nil {
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
	rep := proof.Verify(entry.Pack, exp, s.Opener(ctx, t, entry))
	t.Close()
	if exp.CompletionGoalHash == "" {
		rep.Problems = append(rep.Problems, "no Ready snapshot on the ledger")
	}
	if len(rep.Problems) > 0 {
		return Errorf(ExitBlocked, "ship #%d: the authoritative pack does not verify: %s", n, strings.Join(rep.Problems, "; "))
	}
	if !entry.Pack.FloorsHold() {
		return Errorf(ExitBlocked, "ship #%d: floors do not hold in the authoritative pack: %s (a floor-waiver Decision under #%d waives one, WP4.9)", n, failing(entry.Pack.Checks), story)
	}
	for _, ck := range entry.Pack.Checks {
		if ck.Waiver != "" {
			rec.Waivers = append(rec.Waivers, ck.Floor+" by "+ck.Waiver)
		}
	}

	// 3. One approving review from a human who is not the author, on the head.
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
		rec.HumanPass = HumanPass{Kind: "review", Login: r.User.Login, ReviewID: ids[strings.ToLower(r.User.Login)], Commit: head}
		break
	}
	if rec.HumanPass.Kind == "" {
		if !in.Bool("solo-bypass") {
			if len(why) == 0 {
				why = append(why, "no approving review")
			}
			return Errorf(ExitBlocked, "ship #%d: no human pass: %s (bot reviews and the author's own never count)", n, strings.Join(why, "; "))
		}
		hp, err := e.soloBypass(ctx, c, owner, name, n, head, me.Login, &l.Config)
		if err != nil {
			return err
		}
		rec.HumanPass = hp
	}

	// 4. Merge, then the Complete section, status:merged, the ship record, and cleanup.
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
	return e.Emit(res, func(w io.Writer) error {
		fmt.Fprintf(w, "ship #%d: merged as %s; story #%d is status:merged with its Complete section\n", n, short(rec.MergeCommit), story)
		fmt.Fprintf(w, "  pack %s%s\n", rec.Pack, map[bool]string{true: " (landed from the gate's run artifact: " + rec.PackLanded + ")", false: ""}[rec.PackLanded != ""])
		fmt.Fprintf(w, "  human pass: %s by %s%s\n", rec.HumanPass.Kind, rec.HumanPass.Login, map[bool]string{true: " — " + rec.HumanPass.Note, false: ""}[rec.HumanPass.Note != ""])
		fmt.Fprintf(w, "  ship record %s; actions segment %s\n", res.Record, rec.Actions)
		return nil
	})
}

func dirExists(p string) bool { st, err := os.Stat(p); return err == nil && st.IsDir() }

// authoritative finds the authoritative pack for a story's head on the ledger.
func authoritative(ctx context.Context, s *proof.Store, t *ledger.Tree, story int, head string) (proof.Entry, bool, error) {
	es, err := s.Packs(ctx, t, story)
	if err != nil {
		return proof.Entry{}, false, err
	}
	for _, x := range es {
		if x.Pack.Authority.Kind == proof.Authoritative && x.Pack.PRHeadSHA == head {
			return x, true, nil
		}
	}
	return proof.Entry{}, false, nil
}

// landGatePack lands the merge gate's pack on the ledger when CI could not
// (no bot App, Appendix B D1): the newest successful run of the base
// branch's aayudha-merge-gate.yml for this head, its aayudha-pack artifact,
// pushed as authoritative under the human's identity with the run recorded.
func (e *Env) landGatePack(ctx context.Context, c *github.Client, s *proof.Store, owner, name string, story int, head, login string) (string, error) {
	runs, _, err := c.REST.Actions.ListWorkflowRunsByFileName(ctx, owner, name, MergeGateWorkflow, &gh.ListWorkflowRunsOptions{HeadSHA: head, Event: "pull_request_target", Status: "success"})
	if err != nil {
		return "", Wrap(ExitExternal, err)
	}
	var run *gh.WorkflowRun
	for _, r := range runs.WorkflowRuns {
		if r.GetHeadSHA() == head && r.GetConclusion() == "success" && r.GetPath() == ".github/workflows/"+MergeGateWorkflow && (run == nil || r.GetID() > run.GetID()) {
			run = r
		}
	}
	if run == nil {
		return "", Errorf(ExitBlocked, "no authoritative pack for %s: no successful %s run for that head", short(head), MergeGateWorkflow)
	}
	arts, _, err := c.REST.Actions.ListWorkflowRunArtifacts(ctx, owner, name, run.GetID(), nil)
	if err != nil {
		return "", Wrap(ExitExternal, err)
	}
	var art *gh.Artifact
	for _, a := range arts.Artifacts {
		if a.GetName() == "aayudha-pack" && !a.GetExpired() {
			art = a
		}
	}
	if art == nil {
		return "", Errorf(ExitBlocked, "merge gate run %d has no aayudha-pack artifact", run.GetID())
	}
	u, _, err := c.REST.Actions.DownloadArtifact(ctx, owner, name, art.GetID(), 3)
	if err != nil {
		return "", Wrap(ExitExternal, err)
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	resp, err := e.HTTP.Do(req)
	if err != nil {
		return "", Wrap(ExitExternal, err)
	}
	defer func() { _ = resp.Body.Close() }()
	zipped, err := io.ReadAll(io.LimitReader(resp.Body, 512<<20))
	if err != nil || resp.StatusCode != http.StatusOK {
		return "", Errorf(ExitExternal, "download artifact %d: HTTP %d %v", art.GetID(), resp.StatusCode, err)
	}
	dir, err := os.MkdirTemp("", "aayudha-gate-pack-")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	if err := unzip(zipped, dir); err != nil {
		return "", Wrap(ExitExternal, err)
	}
	packDir := filepath.Join(dir, fmt.Sprint(story))
	p, err := proof.LoadLocal(packDir)
	if err != nil {
		return "", Errorf(ExitBlocked, "merge gate run %d's pack: %v", run.GetID(), err)
	}
	if p.PRHeadSHA != head || p.Story != story {
		return "", Errorf(ExitBlocked, "merge gate run %d's pack is for #%d at %s, not #%d at %s", run.GetID(), p.Story, short(p.PRHeadSHA), story, short(head))
	}
	note := fmt.Sprintf("run %d of the base branch's %s; landed by %s through aayu ship because CI cannot write the ledger without a bot App (Appendix B D1, D-4.10a)", run.GetID(), MergeGateWorkflow, login)
	if _, err := s.Push(ctx, packDir, false, proof.Authority{Kind: proof.Authoritative, Workflow: ".github/workflows/" + MergeGateWorkflow, RunID: run.GetID(), Writer: login + " (aayu ship)"}); err != nil {
		return "", Wrap(ExitExternal, err)
	}
	return note, nil
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
	if identity.Configured(cfg.Identity) != "" {
		return HumanPass{}, Errorf(ExitBlocked, "ship --solo-bypass: this repository has a bot identity, so the agent can author and a human can approve; get an approving review")
	}
	if len(cfg.Seats) > 0 {
		for _, s := range cfg.Seats {
			if s.Mode == "paired" && s.Human != "" && !strings.EqualFold(s.Human, login) {
				return HumanPass{}, Errorf(ExitBlocked, "ship --solo-bypass: a second human (%s) holds a seat; ask them for an approving review", s.Human)
			}
		}
	}
	comments, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.IssueComment, *gh.Response, error) {
		return c.REST.Issues.ListComments(ctx, owner, name, n, &gh.IssueListCommentsOptions{ListOptions: o})
	})
	if err != nil {
		return HumanPass{}, Wrap(ExitExternal, err)
	}
	i := slices.IndexFunc(comments, func(cm *gh.IssueComment) bool {
		return strings.EqualFold(cm.GetUser().GetLogin(), login) && soloComment.MatchString(cm.GetBody())
	})
	if i < 0 {
		return HumanPass{}, Errorf(ExitBlocked, "ship --solo-bypass: post a PR comment starting %q that says why, from your own account, first", SoloBypassPrefix)
	}
	return HumanPass{Kind: "solo-bypass", Login: login, Comment: comments[i].GetHTMLURL(), Commit: head,
		Note: "solo mode: GitHub refuses self-approval and no bot App exists, so no second person could review (Appendix B D2)"}, nil
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
		fmt.Fprintf(&out, "- Human pass: approving review %d by @%s on `%s`\n", rec.HumanPass.ReviewID, rec.HumanPass.Login, short(rec.HumanPass.Commit))
	default:
		fmt.Fprintf(&out, "- Human pass: none possible; %s merged under a recorded solo-mode bypass (%s): %s\n", rec.HumanPass.Login, rec.HumanPass.Comment, rec.HumanPass.Note)
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
