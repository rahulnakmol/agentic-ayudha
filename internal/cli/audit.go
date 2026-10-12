package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/audit"
	"github.com/rahulnakmol/agentic-ayudha/internal/autonomy"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/identity"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
)

func init() { register("audit", runAudit) }

// AllowedLicences are the licences compatible with redistributing a
// BSD-3-Clause binary (THIRD_PARTY_NOTICES.md, Policy; the CI security job).
var AllowedLicences = []string{"0BSD", "Apache-2.0", "BSD-2-Clause", "BSD-3-Clause", "ISC", "MIT"}

// PreGateMarker marks the comment `audit --mark-merged` leaves on a story.
const PreGateMarker = "<!-- aayudha:audit-pre-gate -->"

// AuditResult is what audit reports.
type AuditResult struct {
	Report      *audit.Report `json:"report,omitempty"`
	Path        string        `json:"ledger_path,omitempty"`
	Commit      string        `json:"ledger_commit,omitempty"`
	Unchanged   bool          `json:"unchanged,omitempty"`
	MarkedMerge []int         `json:"marked_merged,omitempty"`
	GitHub      github.Stats  `json:"github"`
}

// auditRun is the shared setup of audit and retro: the clients, the ledger
// at the recorded commit, the inputs, and the data Fetch fills.
type auditRun struct {
	repo   string
	c      *github.Client
	bot    identity.Bot
	store  *proof.Store
	tree   *ledger.Tree
	in     audit.Inputs
	data   *audit.Data
	src    audit.Source
	replay bool // --replay: the GitHub part comes from the snapshot
}

func (e *Env) openAudit(ctx context.Context, in Values, verb string, full bool) (*auditRun, error) {
	if config.RepoRoot(e.Dir) == "" {
		return nil, Errorf(ExitUsage, "%s reads the ledger: run it in a checkout of the repository", verb)
	}
	l, err := e.Config()
	if err != nil {
		return nil, err
	}
	cfg := l.Config
	a := &auditRun{}
	if a.repo, err = e.RepoName(ctx); err != nil {
		return nil, err
	}
	if a.c, a.bot, err = e.BotGitHub(ctx, ""); err != nil {
		return nil, err
	}
	if a.store, err = e.proofStore(ctx, a.c, a.bot.Token, botIdent(a.bot), a.bot.Tag); err != nil {
		return nil, err
	}
	b, _ := json.Marshal(cfg)
	sum := sha256.Sum256(b)
	confHash := "sha256:" + hex.EncodeToString(sum[:])
	clock, _ := cfg.Clock()
	a.data = &audit.Data{Dev: e.devBranch(), Ledger: "ledger", GateCheck: "protected-paths", Bots: cfg.Identity.BotLogins,
		SLAHours: cfg.Decisions.SLAHours, Clock: clock, AllowedLicns: AllowedLicences}
	rules := autonomy.Rules{Bots: cfg.Identity.BotLogins, Days: cfg.Autonomy.CorrectionDays, Layers: autonomy.ReadLayers(config.RepoRoot(e.Dir))}
	a.src = audit.Source{GH: a.c, Repo: a.repo, Store: a.store, Now: e.Now(), Rules: rules, Never: cfg.Autonomy.Never,
		Suites: full && verb == "audit", Autonomy: full, Streak: cfg.Autonomy.Streak}
	if rp := in.String("replay"); rp != "" {
		if in.Has("since") || in.Has("until") || in.Has("ledger") {
			return nil, Errorf(ExitUsage, "%s --replay takes its window and ledger commit from the snapshot", verb)
		}
		return a, e.openReplay(ctx, a, verb, rp, confHash)
	}
	until := e.Now().UTC().Truncate(time.Second)
	if s := in.String("until"); s != "" {
		if until, err = parseWhen(s); err != nil {
			return nil, Errorf(ExitUsage, "%s --until: %v", verb, err)
		}
	}
	if a.tree, err = a.store.Ledger.ReadAt(ctx, in.String("ledger")); errors.Is(err, ledger.ErrNoLedger) {
		return nil, Wrap(ExitBlocked, err)
	} else if err != nil {
		return nil, Errorf(ExitUsage, "%s --ledger: %v", verb, err)
	}
	since := until.AddDate(0, 0, -7)
	if s := in.String("since"); s != "" {
		if since, err = parseWhen(s); err != nil {
			return nil, Errorf(ExitUsage, "%s --since: %v", verb, err)
		}
	} else if prev, ok := a.priorCursor(ctx, verb, until); ok {
		since = prev
	}
	if !since.Before(until) {
		return nil, Errorf(ExitUsage, "%s: --since %s is not before --until %s", verb, since.Format(time.RFC3339), until.Format(time.RFC3339))
	}
	a.in = audit.Inputs{Repo: a.repo, Since: since, Until: until, Ledger: a.tree.Tip, ConfigHash: confHash}
	a.src.Tree = a.tree
	return a, nil
}

// openReplay loads a report's GitHub inputs snapshot from the ledger tip and
// the ledger at the commit it recorded. The configuration must be the one the
// report recorded.
func (e *Env) openReplay(ctx context.Context, a *auditRun, verb, path, confHash string) error {
	tip, err := a.store.Ledger.Read(ctx)
	if err != nil {
		return Wrap(ExitBlocked, err)
	}
	b, err := tip.File(ctx, path)
	tip.Close()
	if err != nil {
		return Errorf(ExitUsage, "%s --replay: %v", verb, err)
	}
	if a.in, err = audit.Thaw(b, path, a.data); err != nil {
		return Errorf(ExitUsage, "%s --replay: %v", verb, err)
	}
	if a.in.ConfigHash != confHash {
		return Errorf(ExitBlocked, "%s --replay: the configuration changed since the report (config hash %s, now %s)", verb, a.in.ConfigHash, confHash)
	}
	if a.tree, err = a.store.Ledger.ReadAt(ctx, a.in.Ledger); err != nil {
		return Errorf(ExitUsage, "%s --replay: %v", verb, err)
	}
	a.src.Tree, a.replay = a.tree, true
	return nil
}

// reportPath is where a verb's report for a cursor goes on the ledger.
func reportPath(verb string, until time.Time) string {
	return fmt.Sprintf("%s/%s/%s-%s.md", verb, until.Format(time.DateOnly), verb, until.Format("20060102T150405Z"))
}

// freeze records the GitHub inputs snapshot beside the report and returns its
// bytes; a replay keeps the snapshot it read.
func (a *auditRun) freeze(report string) []byte {
	if a.replay {
		return nil
	}
	return audit.Freeze(&a.in, a.data, strings.TrimSuffix(report, ".md")+".inputs.json")
}

// emitReplay prints (or writes to out) a report regenerated from its snapshot;
// a replay writes nothing to the ledger.
func (e *Env) emitReplay(v any, md []byte, out string) error {
	if out != "" {
		return os.WriteFile(out, md, 0o644)
	}
	return e.Emit(v, func(w io.Writer) error {
		_, err := w.Write(md)
		return err
	})
}

// unchanged reports whether the ledger tip already holds md at p.
func (a *auditRun) unchanged(ctx context.Context, p string, md []byte) bool {
	tip, err := a.store.Ledger.Read(ctx)
	if err != nil {
		return false
	}
	defer tip.Close()
	old, err := tip.File(ctx, p)
	return err == nil && string(old) == string(md)
}

// priorCursor is the cursor of the newest report of this verb on the ledger
// before until: the default start of the next window.
func (a *auditRun) priorCursor(ctx context.Context, verb string, until time.Time) (time.Time, bool) {
	files, err := a.tree.List(ctx, verb)
	if err != nil {
		return time.Time{}, false
	}
	var best time.Time
	for _, f := range files {
		b, err := a.tree.File(ctx, f)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(b), "\n") {
			rest, ok := strings.CutPrefix(line, audit.HeaderLine)
			if !ok {
				continue
			}
			ts, _, _ := strings.Cut(rest, ";")
			if t, err := time.Parse(time.RFC3339, strings.TrimSpace(strings.TrimSuffix(ts, " |"))); err == nil && t.Before(until) && t.After(best) {
				best = t
			}
		}
	}
	return best, !best.IsZero()
}

// parseWhen reads a date (YYYY-MM-DD, UTC midnight) or an RFC 3339 time.
func parseWhen(s string) (time.Time, error) {
	if t, err := time.Parse(time.DateOnly, s); err == nil {
		return t.UTC(), nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return t, fmt.Errorf("%q: want YYYY-MM-DD or an RFC 3339 time", s)
	}
	return t.UTC().Truncate(time.Second), nil
}

// fetch fills the run's data and maps errors to exit codes.
func (a *auditRun) fetch(ctx context.Context) error {
	if a.replay {
		return audit.FetchLedger(ctx, a.src, &a.in, a.data)
	}
	err := audit.Fetch(ctx, a.src, &a.in, a.data)
	switch {
	case errors.Is(err, audit.ErrNoAdmin):
		return Errorf(ExitBlocked, "%v; run it with a repository admin token (AAYU_GITHUB_TOKEN)", err)
	case err != nil && github.External(err):
		return Wrap(ExitExternal, err)
	}
	return err
}

// runAudit is `aayu audit` (WP7.6): it builds the report from GitHub's event
// history up to --until and the ledger at --ledger, and writes it to
// audit/<date>/ on the ledger under the bot identity (the founder's on day one,
// D1). With --mark-merged it instead moves the listed stories that the report
// would call merged without proof (pre-gate) to status:merged.
func runAudit(ctx context.Context, e *Env, in Values) error {
	mark := in.String("mark-merged")
	a, err := e.openAudit(ctx, in, "audit", mark == "")
	if err != nil {
		return err
	}
	defer a.tree.Close()
	if mark != "" {
		return e.markMerged(ctx, a, mark)
	}
	if err := a.fetch(ctx); err != nil {
		return err
	}
	res := AuditResult{Path: reportPath("audit", a.in.Until)}
	snap := a.freeze(res.Path)
	rep := audit.Build(a.in, a.data)
	md := rep.Markdown()
	res.Report = rep
	if a.replay {
		return e.emitReplay(rep, md, in.String("out"))
	}
	if a.unchanged(ctx, res.Path, md) {
		res.Unchanged = true
	} else if res.Commit, err = a.store.Ledger.Write(ctx, ledger.Commit{Message: "audit: report for " + a.in.Until.Format(time.RFC3339),
		Files: map[string][]byte{res.Path: md, a.in.Snapshot: snap}}); err != nil {
		return Wrap(ExitExternal, err)
	}
	if res.Commit != "" && !e.DryRun {
		if err := e.checkpoint(a.bot.Token); err != nil {
			return Wrap(ExitExternal, fmt.Errorf("report written, but the actions segment did not reach the ledger: %w", err))
		}
	}
	if out := in.String("out"); out != "" {
		if err := os.WriteFile(out, md, 0o644); err != nil {
			return err
		}
	}
	res.GitHub = a.c.Stats()
	return e.Emit(res, func(w io.Writer) error {
		c := rep.Counts()
		fmt.Fprintf(w, "audit %s, %s to %s\n", a.repo, a.in.Since.Format(time.RFC3339), a.in.Until.Format(time.RFC3339))
		fmt.Fprintf(w, "  bypasses %d (solo-mode %d, ledger %d, promotions %d, tags %d), unexplained %d; ship --solo-bypass %d\n",
			c.Bypasses, c.Solo, c.LedgerWrites, c.Promotions, c.Tags, c.Unexplained, c.SoloShips)
		fmt.Fprintf(w, "  writes %d: unlogged %d, workflow/App %d; human-fallback writes %d\n", c.Writes, c.Unlogged, c.CI, c.Fallback)
		fmt.Fprintf(w, "  proof coverage %d/%d; merged without proof (pre-gate) %d, after the gate %d; floor misses %d, waivers %d\n",
			c.Covered, c.Delivered, c.PreGate, c.PostGate, c.FloorMisses, c.Waivers)
		fmt.Fprintf(w, "  decisions over SLA %d; corrections %d; never-layer agent merges %d; licence findings %d\n", c.OverSLA, c.Corrections, c.NeverMerges, c.Licence)
		switch {
		case res.Unchanged:
			fmt.Fprintf(w, "report unchanged on the ledger: %s\n", res.Path)
		case res.Commit == "" || e.DryRun:
			fmt.Fprintf(w, "report: %s (--dry-run: not written)\n", res.Path)
		default:
			fmt.Fprintf(w, "report: %s (ledger %s)\n", res.Path, short(res.Commit))
		}
		fmt.Fprintf(w, "regenerate: %s\n  or re-read GitHub: %s\n", a.in.Replay("audit"), a.in.Command("audit"))
		return nil
	})
}

// markMerged moves stories the audit lists as merged without proof (pre-gate)
// to status:merged, each with a comment naming its merge PR and saying that no
// authoritative pack exists. A story that is not pre-gate, is covered, or has no
// merged PR is refused before anything is written.
func (e *Env) markMerged(ctx context.Context, a *auditRun, spec string) error {
	want, err := parseStories(spec)
	if err != nil {
		return Errorf(ExitUsage, "audit --mark-merged: %v", err)
	}
	if err := a.fetch(ctx); err != nil {
		return err
	}
	rep := audit.Build(a.in, a.data)
	byN := map[int]audit.Story{}
	for _, s := range rep.Stories {
		byN[s.Number] = s
	}
	var refuse []string
	var todo []audit.Story
	for _, n := range want {
		s, ok := byN[n]
		switch {
		case !ok:
			refuse = append(refuse, fmt.Sprintf("#%d: not a delivered story or bug (no merged PR into %s references it)", n, a.data.Dev))
		case s.Covered:
			refuse = append(refuse, fmt.Sprintf("#%d: covered by %s", n, s.Pack))
		case !s.PreGate:
			refuse = append(refuse, fmt.Sprintf("#%d: %s, not pre-gate (merged %s, gate since %s)", n, s.Reason, s.Merged.Format(time.RFC3339), gateText(a.data.GateSince)))
		case s.Status != "merged":
			todo = append(todo, s)
		}
	}
	if len(refuse) > 0 {
		return Errorf(ExitBlocked, "audit --mark-merged refused, nothing written:\n  %s", strings.Join(refuse, "\n  "))
	}
	res := AuditResult{}
	for _, s := range todo {
		pr := a.data.MergePR(s.Number)
		labels := []string{}
		for _, it := range a.data.Items {
			if it.Number == s.Number {
				labels = it.Labels
			}
		}
		if err := setStatus(ctx, a.c, a.repo, s.Number, labels, "merged"); err != nil {
			return Wrap(ExitExternal, err)
		}
		body := fmt.Sprintf("%s\n**Merged without proof (pre-gate).** This story's change merged in #%d (`%s`, %s), before the merge gate "+
			"(`%s`, registered %s) and `ship` existed. No authoritative pack exists for it and it had no human pass: it is done but "+
			"unproven, and `aayu audit` counts it as uncovered in proof coverage.\n\nMoved to `status:merged` by `aayu audit --mark-merged` (WP7.6).\n",
			PreGateMarker, pr.Number, short(pr.MergeSHA), pr.MergedAt.Format(time.RFC3339), audit.MergeGateWorkflow, gateText(a.data.GateSince))
		if _, err := a.c.UpsertComment(ctx, a.repo, s.Number, PreGateMarker, body, actions.Action{Kind: "github.comment.audit", Story: fmt.Sprintf("#%d", s.Number),
			Payload: map[string]any{"pr": pr.Number, "tag": a.bot.Tag}}); err != nil {
			return Wrap(ExitExternal, err)
		}
		res.MarkedMerge = append(res.MarkedMerge, s.Number)
	}
	if len(todo) > 0 && !e.DryRun {
		if err := e.checkpoint(a.bot.Token); err != nil {
			return Wrap(ExitExternal, fmt.Errorf("stories moved, but the actions segment did not reach the ledger: %w", err))
		}
	}
	res.GitHub = a.c.Stats()
	return e.Emit(res, func(w io.Writer) error {
		if len(todo) == 0 {
			_, err := fmt.Fprintln(w, "audit --mark-merged: every listed story is already status:merged")
			return err
		}
		for _, s := range todo {
			fmt.Fprintf(w, "#%d: status:merged (merged without proof, pre-gate, in #%d)%s\n", s.Number, s.PR, map[bool]string{true: " --dry-run: planned", false: ""}[e.DryRun])
		}
		return nil
	})
}

func gateText(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	return t.Format(time.RFC3339)
}

// parseStories reads "38-50,52".
func parseStories(spec string) ([]int, error) {
	var out []int
	for _, part := range strings.Split(spec, ",") {
		lo, hi, isRange := strings.Cut(strings.TrimSpace(strings.ReplaceAll(part, "#", "")), "-")
		a, err1 := strconv.Atoi(lo)
		b := a
		var err2 error
		if isRange {
			b, err2 = strconv.Atoi(hi)
		}
		if err1 != nil || err2 != nil || a < 1 || b < a || b-a > 500 {
			return nil, fmt.Errorf("%q: want issue numbers or ranges, e.g. 38-50,52", part)
		}
		for n := a; n <= b; n++ {
			if !slices.Contains(out, n) {
				out = append(out, n)
			}
		}
	}
	return out, nil
}
