package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/brain"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/identity"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/seats"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

func init() { register("brain", runBrain) }

func runBrain(ctx context.Context, e *Env, in Values) error {
	ws := in.String("workstream")
	if ws != "" && in.String("action") != "init" {
		return Errorf(ExitUsage, "brain: --workstream goes with init")
	}
	if in.Has("dir") && in.String("action") != "check" {
		return Errorf(ExitUsage, "brain: --dir goes with check")
	}
	switch in.String("action") {
	case "open":
		return e.brainOpen(ctx)
	case "check":
		return e.brainCheck(ctx, in.String("dir"))
	case "init":
		if ws != "" && !brain.WorkstreamName.MatchString(ws) {
			return Errorf(ExitUsage, "brain: workstream %q: use lower-case letters, digits, and hyphens", ws)
		}
		return e.brainInit(ctx, ws)
	default:
		return e.brainCheckpoint(ctx)
	}
}

func (e *Env) brainOpen(ctx context.Context) error {
	repo, err := e.RepoName(ctx)
	if err != nil {
		return err
	}
	name := repo[strings.LastIndex(repo, "/")+1:]
	dir := "../" + name + "-brain"
	steps := []string{
		"git fetch origin " + brain.Branch,
		"git worktree add " + dir + " " + brain.Branch,
		"git -C " + dir + " merge --ff-only origin/" + brain.Branch + " (a local " + brain.Branch + " branch from an earlier worktree catches up with the remote)",
		"Open the folder " + dir + " as a vault in Obsidian (Open folder as vault), then open Home.",
		"Write notes there, run `aayu brain check --dir " + dir + "`, then commit and push to " + brain.Branch + ".",
	}
	return e.Emit(map[string]any{"repo": repo, "branch": brain.Branch, "dir": dir, "steps": steps}, func(w io.Writer) error {
		_, err := fmt.Fprintf(w, "The product brain of %s is the %s branch.\n\n", repo, brain.Branch)
		for i, s := range steps {
			fmt.Fprintf(w, "%d. %s\n", i+1, s)
		}
		return err
	})
}

// brainWriter is the ledger writer pointed at the brain branch.
func (e *Env) brainWriter(ctx context.Context) (*ledger.Writer, *github.Client, error) {
	c, bot, err := e.BotGitHub(ctx, "")
	if err != nil {
		return nil, nil, err
	}
	w, err := e.Ledger(ctx, bot.Token, botIdent(bot), bot.Tag)
	if err != nil {
		return nil, nil, err
	}
	w.Branch = brain.Branch
	return w, c, nil
}

// notVault refuses a brain branch that is not an aayudha vault: a branch of
// that name someone else made, which aayu must not write to.
func notVault(files map[string][]byte) error {
	if brain.IsVault(files) {
		return nil
	}
	return Errorf(ExitBlocked, "the %s branch on the remote is not an aayudha vault (no .aayudha-brain marker, no Home.md of type home): aayu writes nothing to it; rename that branch, then run `aayu brain init`", brain.Branch)
}

// brainFiles reads every file at the tip of the brain branch.
func brainFiles(ctx context.Context, w *ledger.Writer) (map[string][]byte, string, error) {
	t, err := w.Read(ctx)
	if errors.Is(err, ledger.ErrNoLedger) {
		return nil, "", Errorf(ExitBlocked, "no %s branch on the remote: run `aayu brain init`", brain.Branch)
	}
	if err != nil {
		return nil, "", Wrap(ExitExternal, err)
	}
	defer t.Close()
	out, err := w.Output(ctx, w.Dir, "ls-tree", "-r", "-z", "--name-only", t.Tip)
	if err != nil {
		return nil, "", err
	}
	files := map[string][]byte{}
	for p := range strings.SplitSeq(out, "\x00") {
		if p == "" {
			continue
		}
		b, err := t.File(ctx, p)
		if err != nil {
			return nil, "", err
		}
		files[p] = b
	}
	return files, t.Tip, nil
}

func (e *Env) brainInit(ctx context.Context, ws string) error {
	w, c, err := e.brainWriter(ctx)
	if err != nil {
		return err
	}
	repo := w.Repo
	today := brain.Today(e.Now())
	files := brain.Skeleton(repo, today, !e.private(ctx, c, repo))
	if ws != "" {
		for p, b := range brain.Workstream(repo, ws, today) {
			files[p] = b
		}
		for p, b := range brain.Index(files, today) {
			files[p] = b
		}
	}
	sha, err := w.Create(ctx, ledger.Commit{Message: "brain: create the product brain (aayu brain init)", Files: files})
	outcome := "created"
	if errors.Is(err, ledger.ErrExists) {
		outcome = "exists"
		err = nil
		// The index rewrites whole notes, so a workstream lands only on the tip it
		// read: when a person pushes in between, it reads their change and runs again.
		for attempt := 0; ; attempt++ {
			existing, tip, rerr := brainFiles(ctx, w)
			if rerr != nil {
				return rerr
			}
			if err := notVault(existing); err != nil {
				return err
			}
			if ws == "" {
				break
			}
			add := map[string][]byte{}
			for p, b := range brain.Workstream(repo, ws, today) {
				if _, ok := existing[p]; !ok {
					add[p] = b
				}
			}
			outcome = "workstream exists"
			if len(add) == 0 {
				break
			}
			merged := maps.Clone(existing) // the root Workstreams map and Home list it in the same commit
			maps.Copy(merged, add)
			maps.Copy(add, brain.Index(merged, today))
			outcome = "workstream added"
			sha, err = w.Write(ctx, ledger.Commit{Message: "brain: add the " + ws + " workstream (aayu brain init)", Files: add, Base: tip})
			if errors.Is(err, ledger.ErrMoved) && attempt < 4 {
				continue
			}
			if err != nil {
				return Wrap(ExitExternal, err)
			}
			break
		}
	}
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	return e.Emit(map[string]any{"branch": brain.Branch, "outcome": outcome, "commit": sha, "workstream": ws}, func(out io.Writer) error {
		_, err := fmt.Fprintf(out, "brain: %s (%s %s). Run `aayu brain open` to open it in Obsidian.\n", outcome, brain.Branch, short(sha))
		return err
	})
}

func (e *Env) brainCheck(ctx context.Context, dir string) error {
	var files map[string][]byte
	where := dir
	if dir != "" {
		var err error
		if files, err = readVault(dir); err != nil {
			return Errorf(ExitUsage, "brain check --dir: %v", err)
		}
	} else {
		root := config.RepoRoot(e.Dir)
		if root == "" {
			return Errorf(ExitUsage, "brain check needs a checkout of the repository, or --dir")
		}
		repo, err := e.RepoName(ctx)
		if err != nil {
			return err
		}
		tok := e.Getenv("AAYU_BOT_TOKEN") // a bot-only session reads a private repository's branch with the bot's token
		if _, t, err := e.HumanGitHub(ctx); err == nil {
			tok = t.Value
		}
		var tip string
		if files, tip, err = brainFiles(ctx, &ledger.Writer{Dir: root, Repo: repo, Token: tok, Branch: brain.Branch}); err != nil {
			return err
		}
		where = brain.Branch + "@" + short(tip)
	}
	r := brain.Check(files)
	err := e.Emit(r, func(w io.Writer) error {
		for _, f := range r.Errors {
			fmt.Fprintf(w, "error: %s\n", f)
		}
		for _, f := range r.Warnings {
			fmt.Fprintf(w, "warning: %s\n", f)
		}
		_, err := fmt.Fprintf(w, "brain check %s: %d notes, %d errors, %d warnings\n", where, r.Notes, len(r.Errors), len(r.Warnings))
		return err
	})
	if err == nil && !r.OK() {
		return Errorf(ExitFloor, "brain check: %d errors", len(r.Errors))
	}
	return err
}

// readVault reads a local vault folder, skipping .git and per-person Obsidian state.
func readVault(dir string) (map[string][]byte, error) {
	files := map[string][]byte{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if rel == ".git" || rel == ".trash" {
				return filepath.SkipDir
			}
			return nil
		}
		if rel == ".git" { // a worktree's .git file
			return nil
		}
		b, err := os.ReadFile(p)
		files[rel] = b
		return err
	})
	return files, err
}

func (e *Env) brainCheckpoint(ctx context.Context) error {
	w, c, err := e.brainWriter(ctx)
	if err != nil {
		return err
	}
	cp, sha, err := e.checkpointBrain(ctx, c, w)
	if err != nil {
		return err
	}
	return e.Emit(map[string]any{"commit": sha, "changed": nonNilStrings(cp.Paths), "needs_review": nonNilStrings(cp.Review)}, func(out io.Writer) error {
		fmt.Fprintf(out, "brain checkpoint: %d notes changed", len(cp.Paths))
		if sha != "" {
			fmt.Fprintf(out, " (%s %s)", brain.Branch, short(sha))
		}
		fmt.Fprintln(out)
		for _, p := range cp.Review {
			fmt.Fprintf(out, "needs review: %s\n", p)
		}
		return nil
	})
}

// checkpointBrain refreshes the brain branch from the repository's records
// and writes one commit when anything changed.
func (e *Env) checkpointBrain(ctx context.Context, c *github.Client, w *ledger.Writer) (brain.Checkpoint, string, error) {
	files, tip, err := brainFiles(ctx, w) // first: without a brain branch there is nothing to read records for
	if err != nil {
		return brain.Checkpoint{}, "", err
	}
	if err := notVault(files); err != nil {
		return brain.Checkpoint{}, "", err
	}
	r, err := e.brainRecords(ctx, c, w)
	if err != nil {
		return brain.Checkpoint{}, "", err
	}
	today := brain.Today(e.Now())
	public := !e.private(ctx, c, w.Repo)
	// A checkpoint rewrites whole notes, so it lands only on the tip it read:
	// when a person pushes in between, it reads their change and runs again.
	for attempt := 0; ; attempt++ {
		if attempt > 0 {
			if files, tip, err = brainFiles(ctx, w); err != nil {
				return brain.Checkpoint{}, "", err
			}
		}
		cp, err := brain.Run(files, r, today, public)
		if err != nil {
			return cp, "", Errorf(ExitBlocked, "brain checkpoint: %v", err)
		}
		if len(cp.Files) == 0 {
			return cp, "", nil
		}
		sha, err := w.Write(ctx, ledger.Commit{Message: "brain: checkpoint " + today + " (aayu brain checkpoint)", Files: cp.Files, Base: tip})
		if errors.Is(err, ledger.ErrMoved) && attempt < 4 {
			continue
		}
		if err != nil {
			return cp, "", Wrap(ExitExternal, err)
		}
		return cp, sha, nil
	}
}

// releaseCheckpoint runs the brain checkpoint after a release lands, when the
// repository has a brain branch. The release has landed, so a failure here is
// reported and does not fail it; `aayu brain checkpoint` runs it again.
func (e *Env) releaseCheckpoint(ctx context.Context, c *github.Client, tok identity.Token) {
	w, err := e.Ledger(ctx, tok, "human", "")
	var cp brain.Checkpoint
	if err == nil {
		w.Branch = brain.Branch
		cp, _, err = e.checkpointBrain(ctx, c, w)
	}
	switch {
	case err == nil:
		e.Logf("brain checkpoint: %d notes changed", len(cp.Paths))
	case strings.Contains(err.Error(), "no "+brain.Branch+" branch"):
	default:
		fmt.Fprintf(e.Err, "warning: the brain checkpoint after the release failed (run `aayu brain checkpoint`): %v\n", err)
	}
}

func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

var (
	rungRE     = regexp.MustCompile(`^releases/(\d+)/(rolling|main)\.json$`)
	baselineRE = regexp.MustCompile(`^releases/(v\d+\.\d+\.\d+)/baseline-(\d+)\.json$`)
	driftRE    = regexp.MustCompile(`^releases/(v\d+\.\d+\.\d+)/drift\.json$`)
	issueURLRE = regexp.MustCompile(`/issues/(\d+)$`)
)

// planItem is a planned issue in the release-planning verb's ledger records.
type planItem struct {
	Number int    `json:"number"`
	Type   string `json:"type"`
	Title  string `json:"title"`
	Effort string `json:"effort"`
	Parent int    `json:"parent"`
}

func (p planItem) item() brain.PlanItem {
	return brain.PlanItem{Number: p.Number, Type: p.Type, Title: p.Title, Effort: p.Effort, Parent: p.Parent}
}

func items(ps []planItem) []brain.PlanItem {
	out := make([]brain.PlanItem, len(ps))
	for i, p := range ps {
		out[i] = p.item()
	}
	return out
}

// brainRecords reads the repository's records for a checkpoint: work items
// (type, parent, title, closed date), release tags, the ledger's promotion,
// baseline, and drift records, the seats, and the approver.
func (e *Env) brainRecords(ctx context.Context, c *github.Client, w *ledger.Writer) (brain.Records, error) {
	owner, name, err := github.SplitRepo(w.Repo)
	if err != nil {
		return brain.Records{}, err
	}
	r := brain.Records{Repo: w.Repo, Issues: map[int]brain.Issue{}}
	if rp, _, err := c.REST.Repositories.Get(ctx, owner, name); err == nil {
		r.Inception = brain.Today(rp.GetCreatedAt().Time)
	}
	issues, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.Issue, *gh.Response, error) {
		return c.REST.Issues.ListByRepo(ctx, owner, name, &gh.IssueListByRepoOptions{State: "all", ListOptions: o})
	})
	if err != nil {
		return r, Wrap(ExitExternal, err)
	}
	for _, is := range issues {
		if is.IsPullRequest() {
			continue
		}
		typ, _ := workitems.TypeOf(is.GetType().GetName(), github.IssueLabels(is))
		if typ == "" {
			continue
		}
		it := brain.Issue{Number: is.GetNumber(), Type: typ, Title: is.GetTitle()}
		if m := issueURLRE.FindStringSubmatch(is.GetParentIssueURL()); m != nil {
			it.Parent, _ = strconv.Atoi(m[1])
		}
		if is.ClosedAt != nil {
			it.Closed = brain.Today(is.GetClosedAt().Time)
		}
		r.Issues[it.Number] = it
	}
	rels, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.RepositoryRelease, *gh.Response, error) {
		return c.REST.Repositories.ListReleases(ctx, owner, name, &o)
	})
	if err != nil {
		return r, Wrap(ExitExternal, err)
	}
	for _, rl := range rels {
		if rl.GetDraft() || rl.PublishedAt == nil {
			continue
		}
		r.Tags = append(r.Tags, brain.Tag{Name: rl.GetTagName(), Date: brain.Today(rl.GetPublishedAt().Time), URL: rl.GetHTMLURL()})
	}
	if err := ledgerReleases(ctx, w, &r); err != nil {
		return r, err
	}
	l, err := e.Config()
	if err != nil {
		return r, err
	}
	r.Approver = l.Config.Autonomy.Approver
	personas := map[verbs.SeatID]seats.Persona{}
	if e.Assets != nil {
		if personas, err = seats.LoadPersonas(e.Assets); err != nil {
			return r, err
		}
	}
	for _, s := range verbs.Seats {
		p := personas[s.ID]
		st := brain.Seat{ID: string(s.ID), Name: s.Name, Twin: p.Name, Purpose: p.Purpose}
		if cs, ok := l.Config.Seats[string(s.ID)]; ok && cs.Mode == "paired" {
			st.Human = cs.Human
		}
		for _, c := range append(slices.Clone(verbs.Verbs), verbs.Plumbing...) {
			if c.Seat == s.ID {
				st.Verbs = append(st.Verbs, c.Name)
			}
		}
		r.Seats = append(r.Seats, st)
	}
	return r, nil
}

// ledgerReleases reads the ledger's promotion records (releases/<issue>/<rung>.json),
// and the release-planning verb's newest baseline and drift per version.
func ledgerReleases(ctx context.Context, w *ledger.Writer, r *brain.Records) error {
	lw := *w
	lw.Branch = "ledger"
	t, err := lw.Read(ctx)
	if errors.Is(err, ledger.ErrNoLedger) {
		return nil
	}
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	defer t.Close()
	paths, err := t.List(ctx, "releases")
	if err != nil {
		return err
	}
	newest := map[string]int{}
	for _, p := range paths {
		read := func(v any) error {
			b, err := t.File(ctx, p)
			if err != nil {
				return err
			}
			if err := json.Unmarshal(b, v); err != nil {
				return fmt.Errorf("ledger %s: %w", p, err)
			}
			return nil
		}
		switch {
		case rungRE.MatchString(p):
			var g struct {
				Release int    `json:"release"`
				Version string `json:"version"`
				Target  string `json:"target"`
				Tag     string `json:"tag"`
				At      string `json:"promoted_at"`
				Stories []int  `json:"stories_released"`
				Already []int  `json:"already_released"`
			}
			if err := read(&g); err != nil {
				return err
			}
			if len(g.At) > 10 {
				g.At = g.At[:10]
			}
			r.Rungs = append(r.Rungs, brain.Rung{Path: p, Release: g.Release, Version: g.Version, Target: g.Target, Tag: g.Tag, Date: g.At, Stories: append(g.Stories, g.Already...)})
		case baselineRE.MatchString(p):
			m := baselineRE.FindStringSubmatch(p)
			n, _ := strconv.Atoi(m[2])
			if prev, ok := newest[m[1]]; ok && n <= prev { // baseline-0 is a baseline too
				continue
			}
			var b struct {
				Version      string     `json:"version"`
				ReleaseIssue int        `json:"release_issue"`
				BaselinedAt  string     `json:"baselined_at"`
				BaselinedBy  string     `json:"baselined_by"`
				BudgetPct    float64    `json:"budget_pct"`
				Items        []planItem `json:"items"`
			}
			if err := read(&b); err != nil {
				return err
			}
			newest[m[1]] = n
			r.Baseline = slices.DeleteFunc(r.Baseline, func(x brain.Baseline) bool { return x.Version == m[1] })
			at := b.BaselinedAt
			if len(at) > 10 {
				at = at[:10]
			}
			r.Baseline = append(r.Baseline, brain.Baseline{Path: p, N: n, Version: m[1], ReleaseIssue: b.ReleaseIssue, BaselinedAt: at,
				BaselinedBy: b.BaselinedBy, BudgetPct: b.BudgetPct, Items: items(b.Items)})
		case driftRE.MatchString(p):
			var d struct {
				Baseline  int        `json:"baseline"`
				BudgetPct float64    `json:"budget_pct"`
				DriftPct  float64    `json:"drift_pct"`
				Decision  int        `json:"over_budget_decision"`
				Added     []planItem `json:"added"`
				Removed   []planItem `json:"removed"`
				Resized   []struct {
					planItem
					From     string `json:"from"`
					To       string `json:"to"`
					FromType string `json:"from_type"` // only when the type changed since the baseline
				} `json:"resized"`
			}
			if err := read(&d); err != nil {
				return err
			}
			dr := brain.Drift{Path: p, Version: driftRE.FindStringSubmatch(p)[1], Baseline: d.Baseline, BudgetPct: d.BudgetPct, DriftPct: d.DriftPct,
				Decision: d.Decision, Added: items(d.Added), Removed: items(d.Removed)}
			for _, rz := range d.Resized {
				dr.Resized = append(dr.Resized, brain.Resize{PlanItem: rz.item(), From: rz.From, To: rz.To, FromType: rz.FromType})
			}
			r.Drift = append(r.Drift, dr)
		}
	}
	return nil
}
