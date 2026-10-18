package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	gh "github.com/google/go-github/v90/github"
	"github.com/shurcooL/githubv4"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/autonomy"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// The sampled audit after merge (Decision #593): a PR ship merged with no
// person's pass (on proof, or an autonomous footprint) waits, as an audit
// entry on the ledger and the AuditLabel on its PR, for a
// person to look at in `aayu inbox`. Their answer is a Sample in
// SamplesLedgerPath; a correction also labels it aayu:correction, which resets
// the streak of the layers an agent's PR touched (a person's own PR has no
// streak: autonomy.Rules.Agent), and opens a PR that reverts it. The look never blocks the merge.

// AuditLabel marks a merged PR whose audit entry waits for a person.
const AuditLabel = workitems.LabelAudit

// AuditLedgerPath is where a merged PR's audit entry waits on the ledger.
func AuditLedgerPath(pr int) string { return fmt.Sprintf("autonomy/audit/%d.json", pr) }

// SamplesLedgerPath is the ledger file of people's looks after merge, one
// JSON Sample per line.
const SamplesLedgerPath = "autonomy/samples.jsonl"

// Sample is one person's look at a PR merged with no person's pass.
type Sample struct {
	PR         int    `json:"pr"`
	Correction bool   `json:"correction"`
	By         string `json:"by,omitempty"`
	At         string `json:"at,omitempty"`
}

// AuditEntry is a PR merged with no person's pass, waiting for a person's look.
type AuditEntry struct {
	PR        int      `json:"pr"`
	Story     int      `json:"story"`
	Head      string   `json:"head"`
	Merge     string   `json:"merge_commit"`
	Footprint []string `json:"footprint"`
	Pass      string   `json:"human_pass"` // how it merged: proof or autonomous
	MergedAt  string   `json:"merged_at"`
}

// prLabelled reports whether a PR carries label.
func prLabelled(p *gh.PullRequest, label string) bool {
	return slices.ContainsFunc(p.Labels, func(l *gh.Label) bool { return l.GetName() == label })
}

// pendingAudits lists the PRs whose audit entry waits on the ledger, so a
// look whose label was removed after the merge still reaches a person.
func (e *Env) pendingAudits(ctx context.Context) ([]int, error) {
	if config.RepoRoot(e.Dir) == "" { // no checkout, so no ledger to read: the labels still list them
		return nil, nil
	}
	_, tok, err := e.HumanGitHub(ctx)
	if err != nil {
		return nil, err
	}
	w, err := e.Ledger(ctx, tok, "human", "")
	if err != nil {
		return nil, err
	}
	t, err := w.Read(ctx)
	if errors.Is(err, ledger.ErrNoLedger) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer t.Close()
	paths, err := t.List(ctx, "autonomy/audit/")
	if err != nil {
		return nil, err
	}
	var out []int
	for _, p := range paths {
		if n, err := strconv.Atoi(strings.TrimSuffix(path.Base(p), ".json")); err == nil {
			out = append(out, n)
		}
	}
	return out, nil
}

// auditItem is the inbox item for a merged PR carrying AuditLabel.
func auditItem(p *gh.PullRequest) InboxItem {
	return InboxItem{Kind: InboxAudit, Number: p.GetNumber(), Title: termText(p.GetTitle()), URL: p.GetHTMLURL(), Evidence: p.GetHTMLURL(), Seat: verbs.EngineeringManager, PR: p.GetNumber(), mergeSHA: p.GetMergeCommitSHA(),
		Why: fmt.Sprintf("PR #%d merged on %s with no person's review; a look after merge keeps that honest, and a correction of an agent's PR resets its layers' streak.", p.GetNumber(), short(p.GetMergeCommitSHA())),
		Options: []InboxAction{{N: 1, Recommended: true, Label: "looks right (a clean sample)", Command: "inbox: audit looks right"},
			{N: 2, Label: fmt.Sprintf("correction: label it aayu:correction (an agent's PR loses its layers' streak) and open a PR that reverts #%d", p.GetNumber()), Command: "inbox: audit correction"}}}
}

// auditAct records the person's look at a merged PR: a sample in the ledger
// (removing the audit entry), and for a correction the correction label (the
// streak's manual correction) and a revert PR.
func (e *Env) auditAct(ctx context.Context, c *github.Client, repo, login string, it InboxItem, correction bool) error {
	_, tok, err := e.HumanGitHub(ctx)
	if err != nil {
		return err
	}
	w, err := e.Ledger(ctx, tok, "human", "")
	if err != nil {
		return err
	}
	owner, name, _ := github.SplitRepo(repo)
	var entry AuditEntry
	sampled := false
	if ok, err := readLedgerJSON(ctx, w, AuditLedgerPath(it.Number), &entry); err != nil {
		return Wrap(ExitExternal, err)
	} else if !ok {
		// No entry: the look already recorded its sample and removed it, or
		// ship merged and labelled the PR but did not write it. The label
		// is enough: the entry is rebuilt from the PR. A clean sample on
		// the ledger only clears the label; a correction's sample does not
		// stand in for the correction, which anyone who can write the
		// ledger could have forged, so its steps run again.
		prior, err := ledgerSample(ctx, w, it.Number)
		if err != nil {
			return Wrap(ExitExternal, err)
		}
		if prior != nil && !prior.Correction && !correction {
			if err := removeLabel(ctx, c, repo, it.Number, AuditLabel); err != nil {
				return Wrap(ExitExternal, err)
			}
			fmt.Fprintf(e.Out, "  done: the sample of #%d was already in %s; its %s label is removed\n", it.Number, SamplesLedgerPath, AuditLabel)
			return nil
		}
		sampled = prior != nil && prior.Correction && correction
		p, _, err := c.REST.PullRequests.Get(ctx, owner, name, it.Number)
		if err != nil {
			return Wrap(ExitExternal, err)
		}
		entry = AuditEntry{PR: it.Number, Story: StoryOf(p.GetBody(), "", p.GetHead().GetRef()), Merge: p.GetMergeCommitSHA()}
	}
	if correction {
		if err := addLabel(ctx, c, repo, it.Number, autonomy.LabelCorrection); err != nil {
			return Wrap(ExitExternal, err)
		}
		url, err := revertPR(ctx, c, repo, it.Number, entry.Story)
		if err != nil {
			return Wrap(ExitExternal, err)
		}
		fmt.Fprintf(e.Out, "  revert PR: %s\n", url)
	}
	commit := ledger.Commit{Message: fmt.Sprintf("autonomy: audit of PR #%d by %s (correction: %t)", it.Number, login, correction), Delete: []string{AuditLedgerPath(it.Number)}}
	if !sampled {
		b, _ := json.Marshal(Sample{PR: it.Number, Correction: correction, By: login, At: e.Now().UTC().Format(time.RFC3339)})
		commit.Append = map[string][]byte{SamplesLedgerPath: append(b, '\n')}
	}
	if _, err := w.Write(ctx, commit); err != nil {
		return Wrap(ExitExternal, err)
	}
	if err := removeLabel(ctx, c, repo, it.Number, AuditLabel); err != nil {
		return Wrap(ExitExternal, err)
	}
	fmt.Fprintf(e.Out, "  done: sample of #%d recorded in %s (correction: %t)\n", it.Number, SamplesLedgerPath, correction)
	return nil
}

// ledgerSample is the newest sample of PR n in SamplesLedgerPath, or nil. A
// samples file it cannot read holds none, so the look stays.
func ledgerSample(ctx context.Context, w *ledger.Writer, n int) (*Sample, error) {
	t, err := w.Read(ctx)
	if err != nil {
		return nil, err
	}
	defer t.Close()
	b, _ := t.File(ctx, SamplesLedgerPath)
	var out *Sample
	for _, l := range bytes.Split(b, []byte("\n")) {
		var s Sample
		if json.Unmarshal(l, &s) == nil && s.PR == n {
			out = &s
		}
	}
	return out, nil
}

// revertPR opens a PR that reverts merged PR n (GitHub's revertPullRequest)
// and returns its URL. A revert PR of n that this account already opened
// into the same base, still open or merged, is reused, so a look run again
// opens no second one; one closed unmerged reverts nothing, so a new one
// opens; a PR anyone else opened, whatever its text says, never stands in.
func revertPR(ctx context.Context, c *github.Client, repo string, n, story int) (string, error) {
	owner, name, _ := github.SplitRepo(repo)
	me, _, err := c.REST.Users.Get(ctx, "")
	if err != nil {
		return "", err
	}
	orig, _, err := c.REST.PullRequests.Get(ctx, owner, name, n)
	if err != nil {
		return "", err
	}
	prs, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.PullRequest, *gh.Response, error) {
		return c.REST.PullRequests.List(ctx, owner, name, &gh.PullRequestListOptions{State: "all", Base: orig.GetBase().GetRef(), ListOptions: o})
	})
	if err != nil {
		return "", err
	}
	for _, p := range prs {
		if (p.GetState() == "open" || p.MergedAt != nil) && strings.EqualFold(p.GetUser().GetLogin(), me.GetLogin()) && p.GetBase().GetRef() == orig.GetBase().GetRef() && strings.HasPrefix(p.GetBody(), revertBody(n)) {
			return p.GetHTMLURL(), nil
		}
	}
	var url string
	_, err = c.Write(ctx, actions.Action{Kind: "github.pull.revert", Target: fmt.Sprintf("%s#%d", repo, n),
		Request: &actions.Request{Method: http.MethodPost, Path: "/graphql"}, Payload: map[string]any{"pr": n}},
		func(ctx context.Context) (any, error) {
			var m struct {
				RevertPullRequest struct {
					RevertPullRequest struct{ URL githubv4.String }
				} `graphql:"revertPullRequest(input: $input)"`
			}
			title := githubv4.String(fmt.Sprintf("revert: %s (#%d)", orig.GetTitle(), n))
			body := githubv4.String(fmt.Sprintf("%s a person marked it a correction in its sampled audit after merge (`aayu inbox`).\n\nRefs #%d\n", revertBody(n), story))
			err := c.GraphQL.Mutate(ctx, &m, githubv4.RevertPullRequestInput{PullRequestID: githubv4.ID(orig.GetNodeID()), Title: &title, Body: &body}, nil)
			url = string(m.RevertPullRequest.RevertPullRequest.URL)
			return url, err
		})
	return url, err
}

// revertBody starts the body of the PR that reverts PR n.
func revertBody(n int) string { return fmt.Sprintf("Reverts #%d:", n) }
