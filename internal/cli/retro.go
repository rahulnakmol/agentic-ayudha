package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/audit"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
)

func init() { register("retro", runRetro) }

// RetroResult is what retro reports.
type RetroResult struct {
	Retro     *audit.Retro `json:"retro"`
	Path      string       `json:"ledger_path"`
	Commit    string       `json:"ledger_commit,omitempty"`
	Unchanged bool         `json:"unchanged,omitempty"`
	Filed     []int        `json:"filed,omitempty"`
	GitHub    github.Stats `json:"github"`
}

// runRetro is `aayu retro` (WP7.6): estimate versus observed checkpoints,
// corrections, streaks, floor misses, and cost per story over the window,
// written to retro/<date>/ on the ledger; each finding not filed before
// becomes a Feedback issue on this repository (never a PR).
func runRetro(ctx context.Context, e *Env, in Values) error {
	a, err := e.openAudit(ctx, in, "retro", true)
	if err != nil {
		return err
	}
	defer a.tree.Close()
	if err := a.fetch(ctx); err != nil {
		return err
	}
	l, err := e.Config()
	if err != nil {
		return err
	}
	res := RetroResult{Path: reportPath("retro", a.in.Until)}
	snap := a.freeze(res.Path)
	r := audit.BuildRetro(a.in, a.data, l.Config.Budget.PerStoryUSD)
	md := r.Markdown()
	res.Retro = r
	if a.replay {
		return e.emitReplay(r, md, in.String("out"))
	}
	if a.unchanged(ctx, res.Path, md) {
		res.Unchanged = true
	} else if res.Commit, err = a.store.Ledger.Write(ctx, ledger.Commit{Message: "retro: report for " + a.in.Until.Format(time.RFC3339),
		Files: map[string][]byte{res.Path: md, a.in.Snapshot: snap}}); err != nil {
		return Wrap(ExitExternal, err)
	}
	owner, name, _ := github.SplitRepo(a.repo)
	labels := []string{"type:feedback", "status:backlog", "intake"}
	if a.bot.Tag != "" {
		labels = append(labels, a.bot.Tag)
	}
	for i, f := range r.Findings {
		if f.Issue != 0 {
			continue
		}
		title, body := audit.FeedbackPrefix+f.Title, r.FeedbackBody(f, res.Path)
		rec, err := a.c.Write(ctx, actions.Action{Kind: "github.issue.create", Target: a.repo, Seat: "engineering-manager",
			Request: &actions.Request{Method: http.MethodPost, Path: "/repos/" + a.repo + "/issues"}, Payload: map[string]any{"title": title, "labels": labels, "tag": a.bot.Tag}},
			func(ctx context.Context) (any, error) {
				is, _, err := a.c.REST.Issues.Create(ctx, owner, name, gh.CreateIssueRequest{Title: title, Body: gh.Ptr(body), Labels: labels})
				return map[string]any{"number": is.GetNumber()}, err
			})
		if err != nil {
			return Wrap(ExitExternal, err)
		}
		if m, ok := rec.Result.(map[string]any); ok {
			if n, ok := m["number"].(int); ok {
				r.Findings[i].Issue = n
				res.Filed = append(res.Filed, n)
			}
		}
	}
	if (res.Commit != "" || len(res.Filed) > 0) && !e.DryRun {
		if err := e.checkpoint(a.bot.Token); err != nil {
			return Wrap(ExitExternal, fmt.Errorf("retro written, but the actions segment did not reach the ledger: %w", err))
		}
	}
	if out := in.String("out"); out != "" {
		if err := os.WriteFile(out, md, 0o644); err != nil {
			return err
		}
	}
	res.GitHub = a.c.Stats()
	return e.Emit(res, func(w io.Writer) error {
		fmt.Fprintf(w, "retro %s, %s to %s: %d stories, %d agent merges (%d corrected), %d floor misses, %d findings\n", a.repo,
			a.in.Since.Format(time.RFC3339), a.in.Until.Format(time.RFC3339), len(r.Stories), r.AgentMerges, r.Corrected, len(r.FloorMisses), len(r.Findings))
		for _, f := range r.Findings {
			state := "planned (--dry-run)"
			if f.Issue != 0 {
				state = fmt.Sprintf("#%d", f.Issue)
			}
			fmt.Fprintf(w, "  %s: %s\n", state, f.Title)
		}
		fmt.Fprintf(w, "report: %s\n", res.Path)
		return nil
	})
}
