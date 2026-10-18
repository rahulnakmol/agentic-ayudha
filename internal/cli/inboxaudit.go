package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	gh "github.com/google/go-github/v90/github"
	"github.com/shurcooL/githubv4"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/autonomy"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// The sampled audit after merge (Decision #593): a PR ship merged with no
// person's pass (on proof, or an autonomous footprint) waits, as an audit
// entry on the ledger and the AuditLabel on its PR, for a
// person to look at in `aayu inbox`. Their answer is a Sample in
// SamplesLedgerPath; a correction also resets the streak of the layers the PR
// touched and opens a PR that reverts it. The look never blocks the merge.

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

// auditItem is the inbox item for a merged PR carrying AuditLabel.
func auditItem(p *gh.PullRequest) InboxItem {
	return InboxItem{Kind: InboxAudit, Number: p.GetNumber(), Title: p.GetTitle(), URL: p.GetHTMLURL(), Seat: verbs.EngineeringManager, PR: p.GetNumber(), mergeSHA: p.GetMergeCommitSHA(),
		Why: fmt.Sprintf("PR #%d merged on %s with no person's review; a look after merge keeps that honest, and a correction resets its layers' streak.", p.GetNumber(), short(p.GetMergeCommitSHA())),
		Options: []InboxAction{{N: 1, Recommended: true, Label: "looks right (a clean sample)", Command: "inbox: audit looks right"},
			{N: 2, Label: fmt.Sprintf("correction: reset the streak and open a PR that reverts #%d", p.GetNumber()), Command: "inbox: audit correction"}}}
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
	var entry AuditEntry
	if ok, err := readLedgerJSON(ctx, w, AuditLedgerPath(it.Number), &entry); err != nil {
		return Wrap(ExitExternal, err)
	} else if !ok {
		// An earlier look recorded its sample and removed the entry, then
		// failed to remove the label: finish it.
		if sampled, err := ledgerSampled(ctx, w, it.Number); err != nil {
			return Wrap(ExitExternal, err)
		} else if !sampled {
			return Errorf(ExitBlocked, "inbox: PR #%d has no audit entry at %s; only ship writes one", it.Number, AuditLedgerPath(it.Number))
		}
		if err := removeLabel(ctx, c, repo, it.Number, AuditLabel); err != nil {
			return Wrap(ExitExternal, err)
		}
		fmt.Fprintf(e.Out, "  done: the sample of #%d was already in %s; its %s label is removed\n", it.Number, SamplesLedgerPath, AuditLabel)
		return nil
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
	b, _ := json.Marshal(Sample{PR: it.Number, Correction: correction, By: login, At: e.Now().UTC().Format(time.RFC3339)})
	if _, err := w.Write(ctx, ledger.Commit{Message: fmt.Sprintf("autonomy: audit of PR #%d by %s (correction: %t)", it.Number, login, correction),
		Append: map[string][]byte{SamplesLedgerPath: append(b, '\n')}, Delete: []string{AuditLedgerPath(it.Number)}}); err != nil {
		return Wrap(ExitExternal, err)
	}
	if err := removeLabel(ctx, c, repo, it.Number, AuditLabel); err != nil {
		return Wrap(ExitExternal, err)
	}
	fmt.Fprintf(e.Out, "  done: sample of #%d recorded in %s (correction: %t)\n", it.Number, SamplesLedgerPath, correction)
	return nil
}

// ledgerSampled reports whether SamplesLedgerPath holds a sample of PR n. A
// samples file it cannot read holds none, so the look stays.
func ledgerSampled(ctx context.Context, w *ledger.Writer, n int) (bool, error) {
	t, err := w.Read(ctx)
	if err != nil {
		return false, err
	}
	defer t.Close()
	b, _ := t.File(ctx, SamplesLedgerPath)
	for _, l := range bytes.Split(b, []byte("\n")) {
		var s Sample
		if json.Unmarshal(l, &s) == nil && s.PR == n {
			return true, nil
		}
	}
	return false, nil
}

// revertPR opens a PR that reverts merged PR n (GitHub's revertPullRequest)
// and returns its URL. A revert PR an earlier, interrupted correction opened
// is reused, so a retry opens no second one.
func revertPR(ctx context.Context, c *github.Client, repo string, n, story int) (string, error) {
	owner, name, _ := github.SplitRepo(repo)
	prs, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.PullRequest, *gh.Response, error) {
		return c.REST.PullRequests.List(ctx, owner, name, &gh.PullRequestListOptions{State: "all", ListOptions: o})
	})
	if err != nil {
		return "", err
	}
	for _, p := range prs {
		if strings.HasPrefix(p.GetBody(), revertBody(n)) {
			return p.GetHTMLURL(), nil
		}
	}
	var url string
	_, err = c.Write(ctx, actions.Action{Kind: "github.pull.revert", Target: fmt.Sprintf("%s#%d", repo, n),
		Request: &actions.Request{Method: http.MethodPost, Path: "/graphql"}, Payload: map[string]any{"pr": n}},
		func(ctx context.Context) (any, error) {
			p, _, err := c.REST.PullRequests.Get(ctx, owner, name, n)
			if err != nil {
				return nil, err
			}
			var m struct {
				RevertPullRequest struct {
					RevertPullRequest struct{ URL githubv4.String }
				} `graphql:"revertPullRequest(input: $input)"`
			}
			title := githubv4.String(fmt.Sprintf("revert: %s (#%d)", p.GetTitle(), n))
			body := githubv4.String(fmt.Sprintf("%s a person marked it a correction in its sampled audit after merge (`aayu inbox`).\n\nRefs #%d\n", revertBody(n), story))
			err = c.GraphQL.Mutate(ctx, &m, githubv4.RevertPullRequestInput{PullRequestID: githubv4.ID(p.GetNodeID()), Title: &title, Body: &body}, nil)
			url = string(m.RevertPullRequest.RevertPullRequest.URL)
			return url, err
		})
	return url, err
}

// revertBody starts the body of the PR that reverts PR n.
func revertBody(n int) string { return fmt.Sprintf("Reverts #%d:", n) }
