package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/autonomy"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/notify"
)

// CorrectionsPath is the ledger file the streak workflow appends to (WP5.7).
const CorrectionsPath = "autonomy/corrections.jsonl"

type streakReport struct {
	Repo         string                 `json:"repo"`
	Threshold    int                    `json:"threshold"`
	Never        []string               `json:"never"`
	Scanned      int                    `json:"merged_prs_scanned"`
	Layers       []autonomy.LayerStreak `json:"layers"`
	Corrections  []autonomy.Correction  `json:"corrections"`
	Recorded     int                    `json:"recorded,omitempty"` // corrections new to the ledger file
	LedgerCommit string                 `json:"ledger_commit,omitempty"`
	GitHub       github.Stats           `json:"github"`
}

// runStreak is `status --streak` (WP5.7, V4): it computes every layer's
// streak from merged-PR history. With --apply, which only the protected
// aayudha-streak workflow runs (--ci, bot token), it appends the corrections
// the ledger does not hold yet to autonomy/corrections.jsonl and pages them.
func runStreak(ctx context.Context, e *Env, apply bool) error {
	if apply && !e.CI {
		return Errorf(ExitUsage, "status --streak --apply: corrections reach the ledger only from the protected aayudha-streak workflow (--ci)")
	}
	l, err := e.Config()
	if err != nil {
		return err
	}
	repo, err := e.RepoName(ctx)
	if err != nil {
		return err
	}
	c, _, err := e.HumanGitHub(ctx)
	if err != nil {
		return err
	}
	a := l.Config.Autonomy
	rules := autonomy.Rules{Bots: l.Config.Identity.BotLogins, Days: a.CorrectionDays, Layers: autonomy.ReadLayers(config.RepoRoot(e.Dir))}
	prs, issues, err := autonomy.Fetch(ctx, c, repo, rules, autonomy.MaxPRs)
	if err != nil {
		return err
	}
	rep := streakReport{Repo: repo, Threshold: a.Streak, Never: a.Never, Scanned: len(prs), Corrections: rules.Detect(prs, issues)}
	if rep.Never == nil {
		rep.Never = []string{}
	}
	if rep.Corrections == nil {
		rep.Corrections = []autonomy.Correction{}
	}
	rep.Layers = rules.Streaks(prs, rep.Corrections, a.Streak, a.Never)
	if apply {
		if err := recordCorrections(ctx, e, c, repo, &rep); err != nil {
			return err
		}
	}
	rep.GitHub = c.Stats()
	return e.Emit(rep, func(w io.Writer) error {
		fmt.Fprintf(w, "Autonomy streaks for %s (threshold %d; %d merged PRs scanned)\n\n", repo, rep.Threshold, rep.Scanned)
		fmt.Fprintln(w, "| Layer | Streak | Agent merges | Not counted (review not different-family) | Never-layer | Autonomous | Last correction |")
		fmt.Fprintln(w, "|---|---|---|---|---|---|---|")
		for _, s := range rep.Layers {
			last := "—"
			if s.LastBreak != 0 {
				last = fmt.Sprintf("#%d", s.LastBreak)
			}
			fmt.Fprintf(w, "| %s | %d | %d | %d | %t | %t | %s |\n", s.Layer, s.Streak, s.Merges, s.Skipped, s.Never, s.Autonomous, last)
		}
		fmt.Fprintf(w, "\n%d correction(s) in the scanned history", len(rep.Corrections))
		if apply {
			fmt.Fprintf(w, "; %d new on ledger %s", rep.Recorded, CorrectionsPath)
		}
		fmt.Fprintln(w)
		for _, c := range rep.Corrections {
			fmt.Fprintf(w, "- #%d %s: %s (%s)\n", c.PR, c.Kind, c.Ref, strings.Join(c.Layers, ", "))
		}
		return nil
	})
}

// recordCorrections appends the corrections the ledger file lacks, under the
// bot identity (in CI the App token; refused without it, D1).
func recordCorrections(ctx context.Context, e *Env, c *github.Client, repo string, rep *streakReport) error {
	owner, name, _ := github.SplitRepo(repo)
	have := map[string]bool{}
	f, _, _, err := c.REST.Repositories.GetContents(ctx, owner, name, CorrectionsPath, &gh.RepositoryContentGetOptions{Ref: "ledger"})
	switch {
	case github.NotFound(err):
	case err != nil:
		return err
	default:
		text, err := f.GetContent()
		if err != nil {
			return err
		}
		for _, line := range strings.Split(text, "\n") {
			var old autonomy.Correction
			if json.Unmarshal([]byte(line), &old) == nil && old.PR != 0 {
				have[old.Key()] = true
			}
		}
	}
	var lines []byte
	var fresh []string
	for _, cr := range rep.Corrections {
		if have[cr.Key()] {
			continue
		}
		b, err := json.Marshal(cr)
		if err != nil {
			return err
		}
		lines = append(append(lines, b...), '\n')
		fresh = append(fresh, fmt.Sprintf("#%d %s: %s", cr.PR, cr.Kind, cr.Ref))
	}
	rep.Recorded = len(fresh)
	if len(fresh) == 0 {
		return nil
	}
	_, bot, err := e.BotGitHub(ctx, "")
	if err != nil {
		return err
	}
	w, err := e.Ledger(ctx, bot.Token, "bot", bot.Tag)
	if err != nil {
		return err
	}
	if rep.LedgerCommit, err = w.Write(ctx, ledger.Commit{
		Message: fmt.Sprintf("autonomy: %d correction(s)", len(fresh)),
		Append:  map[string][]byte{CorrectionsPath: lines},
	}); err != nil {
		return err
	}
	if n, err := e.Notifier(ctx); err == nil {
		_, _ = n.Page(ctx, notify.Page{Key: "autonomy-corrections:" + repo, Subject: "aayudha: autonomy streak corrections in " + repo,
			Body: "New corrections reset the streak of the layers they touched:\n- " + strings.Join(fresh, "\n- ")})
	}
	return nil
}
