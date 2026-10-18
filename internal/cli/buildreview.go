package cli

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"

	"github.com/rahulnakmol/agentic-ayudha/internal/autonomy"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/doctor"
	"github.com/rahulnakmol/agentic-ayudha/internal/inspect"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/models"
	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
	"github.com/rahulnakmol/agentic-ayudha/internal/ready"
	"github.com/rahulnakmol/agentic-ayudha/internal/ui"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
	"github.com/rahulnakmol/agentic-ayudha/internal/verify"
)

// The session review (Decision #656, option 2, the session half): before each
// push, aayu build has a reviewer model of a family other than the builder's
// review the branch's diff with inspect's own review (inspect.Runner: the
// same prompt, schema, noise rules, carried findings, and rule 7 as the merge
// gate's model review). On fix the push waits: the builder fixes each finding
// or declines it with a reason (--decline), and build reviews again, up to
// inspect.MaxRounds rounds a push. The PR body carries the rounds as the
// builder's own record. It never counts as a review: in solo mode a review
// written in the build session cannot be told from the builder's word, so the
// merge gate's inspect reviews again and decides.

// sessionReviewFile keeps a story's review loop in its worktree's own git
// directory: never committed, and removed with the worktree.
const sessionReviewFile = "aayudha-session-review.json"

// Markers of the PR body's session review section, replaced in place.
const (
	sessionReviewStart = "<!-- aayudha:session-review -->"
	sessionReviewEnd   = "<!-- /aayudha:session-review -->"
)

// sessionReview is the build session's review loop for one story.
type sessionReview struct {
	State    inspect.State     `json:"state"` // inspect's rounds, carried findings, cache, and spend
	Rounds   []sessionRound    `json:"rounds,omitempty"`
	Declined map[string]string `json:"declined,omitempty"` // item ID -> the builder's reason
	Pushed   int               `json:"pushed"`             // rounds before this index went out with an earlier push
	// PushedAt is when the last push went out: inspect's round cap counts the
	// rounds since, as after a person's reopen, while open P0s and P1s of
	// earlier rounds still carry forward.
	PushedAt time.Time `json:"pushed_at,omitzero"`
	// PushedHead is the head the last push sent: a run on it again has
	// nothing new to review.
	PushedHead string        `json:"pushed_head,omitempty"`
	Pending    string        `json:"pending,omitempty"` // the head whose review holds the push
	Skipped    string        `json:"skipped,omitempty"` // why the last push went out without a review
	Checks     []proof.Check `json:"checks,omitempty"`  // the last checkpoint verify's results, the review's floors
}

// sessionRound is one review of a distinct head.
type sessionRound struct {
	Head        string        `json:"head"`
	Model       string        `json:"model"`
	Family      string        `json:"family"`
	Builder     string        `json:"builder"`
	Disposition string        `json:"disposition"`
	Items       []sessionItem `json:"items,omitempty"` // what the round left to fix: findings P0 to P2, unmet criteria
	CostUSD     float64       `json:"cost_usd"`
}

// sessionItem is one thing a round asks the builder to fix: a finding, or an
// unmet criterion (ID C<n>).
type sessionItem struct {
	ID       string `json:"id"`
	Severity string `json:"severity"`
	File     string `json:"file,omitempty"`
	Line     int    `json:"line,omitempty"`
	Title    string `json:"title"`
	Detail   string `json:"detail,omitempty"`
	Fix      string `json:"fix,omitempty"`
}

// SessionReview is what build reports of the session review.
type SessionReview struct {
	Round       int           `json:"round,omitempty"`
	Model       string        `json:"model,omitempty"`
	Disposition string        `json:"disposition,omitempty"`
	Open        []sessionItem `json:"open,omitempty"`
	Skipped     string        `json:"skipped,omitempty"`
}

func sessionReviewPath(ctx context.Context, wt string) (string, error) {
	d, err := run2(ctx, wt, "git", "rev-parse", "--absolute-git-dir")
	if err != nil {
		return "", err
	}
	return filepath.Join(d, sessionReviewFile), nil
}

func loadSessionReview(ctx context.Context, wt string) (sessionReview, error) {
	var s sessionReview
	p, err := sessionReviewPath(ctx, wt)
	if err != nil {
		return s, err
	}
	b, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	return s, json.Unmarshal(b, &s)
}

func (s *sessionReview) save(ctx context.Context, wt string) error {
	p, err := sessionReviewPath(ctx, wt)
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0o600)
}

// open is what the newest round left to fix that the builder has not declined.
func (s *sessionReview) open() []sessionItem {
	if len(s.Rounds) == 0 {
		return nil
	}
	return slices.DeleteFunc(slices.Clone(s.Rounds[len(s.Rounds)-1].Items), func(it sessionItem) bool { return s.Declined[it.ID] != "" })
}

// decline records the builder's reasons, one "<id>: <why>" a line, for items
// the newest round left open.
func (s *sessionReview) decline(text string) error {
	for line := range strings.SplitSeq(strings.TrimSpace(text), "\n") {
		id, why, ok := strings.Cut(line, ":")
		id, why = strings.TrimSpace(id), clipText(oneLine(strings.TrimSpace(why)), 300)
		if !ok || why == "" {
			return fmt.Errorf("--decline %q: write \"<id>: <why>\", one a line", line)
		}
		if !slices.ContainsFunc(s.open(), func(it sessionItem) bool { return it.ID == id }) {
			return fmt.Errorf("--decline: %s is not an open item of the last session review", id)
		}
		if s.Declined == nil {
			s.Declined = map[string]string{}
		}
		s.Declined[id] = why
	}
	return nil
}

// sessionReviewIn is what one session review reads.
type sessionReviewIn struct {
	cfg     *config.Config
	w       *ledger.Writer
	repo    string
	n       int
	chk     ready.Result
	body    string // the story's body
	wt, dev string
	head    string
	st      BuildState
}

// reviewSession runs one round of the session review on the branch head, and
// reports whether the push waits for the builder. It never fails the build: a
// review that cannot run is a note, and the merge gate still reviews.
func (e *Env) reviewSession(ctx context.Context, s *sessionReview, a sessionReviewIn) (wait bool, rep SessionReview, err error) {
	skip := func(why string) (bool, SessionReview, error) {
		s.Skipped, s.Pending = why, ""
		return false, SessionReview{Skipped: why}, nil
	}
	if a.head == s.PushedHead {
		return false, rep, nil
	}
	cfg := *a.cfg
	if err := e.useReviewModel(&cfg); err != nil {
		return false, rep, Wrap(ExitUsage, err)
	}
	gw, err := e.Gateway()
	if err != nil {
		return false, rep, err
	}
	gw.Models, gw.Resolver.Models = cfg.Models, cfg.Models
	builder := e.sessionBuilder(ctx, gw, a)
	story := fmt.Sprintf("#%d", a.n)
	m := paidReviewer(gw, &cfg, builder, "a build session has no reviewer app", func() string { return "write" }, e.Logf)
	if m == nil {
		if picked, _, err := reviewerModels(gw, builder.Candidates, false, "", story); err == nil && builder.Rung(picked[0].Family) == inspect.RungA {
			if route, err := models.RouteFor(cfg.Models, picked[0].Model, picked[0].Family); err == nil {
				if k, err := gw.Key(route.Secret); err == nil && k != "" {
					m = &picked[0]
				}
			}
		}
	}
	if m == nil {
		return skip(fmt.Sprintf("no reviewer model of a family other than the builder's (%s) can be called here; set %s, %s and %s, or [inspect] paid_reviewer with its key",
			builder.Note(), inspect.ReviewModelVar, inspect.ReviewModelURLVar, inspect.ReviewModelKeyVar))
	}
	diff, err := run2(ctx, a.wt, "git", "diff", "origin/"+a.dev+"...HEAD")
	if err != nil {
		return false, rep, Wrap(ExitExternal, err)
	}
	pack := &proof.Pack{Repo: a.repo, Story: a.n, PRHeadSHA: a.head, Checks: s.Checks, Criteria: []proof.Criterion{}}
	if b, err := os.ReadFile(verify.EvidencePath(a.wt, a.n)); err == nil {
		var em verify.EvidenceMap
		if toml.Unmarshal(b, &em) == nil {
			for _, c := range em.Criterion {
				pack.Criteria = append(pack.Criteria, proof.Criterion{ID: c.ID, Evidence: append(slices.Clone(c.Tests), c.Checks...)})
			}
		}
	}
	in := inspect.Input{Repo: a.repo, Title: "feat: " + a.chk.Title, Diff: diff, Story: a.n, StoryTitle: a.chk.Title, StoryBody: a.body,
		Criteria: a.chk.Criteria, Pack: pack, Head: a.head, At: e.Now(), Reopened: s.PushedAt, Mode: inspect.ModeGeneral, Voice: InspectVoice(e.Assets)}
	in.Files = inspect.ParseDiff(diff)
	in.Areas = inspect.BuildContext(ctx, in.Files, gitSource(a.wt), autonomy.ReadLayers(a.wt).Footprint)
	if a.w != nil {
		in.Dismissed = readDismissed(ctx, a.w)
	}
	g := gw.WithModel(verbs.LaneReview, verbs.DirectHost, m.Model)
	r := inspect.Runner{MaxUSD: cfg.Budget.PerPRInspectMax,
		Filter: inspect.Filter{MinConfidence: cfg.Inspect.MinConfidence, SuppressAfter: cfg.Inspect.SuppressAfter, Generated: cfg.Inspect.Generated},
		Reviewers: []inspect.Reviewer{{Model: m.Model, Complete: func(ctx context.Context, req models.Request) (models.Response, float64, error) {
			req.Effort = cmp.Or(req.Effort, m.Effort)
			if m.Source == models.SourceConfig {
				req.ServiceTier = models.TierPriority // as the merge gate's paid review asks
			}
			resp, rm, err := g.Do(ctx, models.Call{Story: story, Seat: string(verbs.EngineeringManager), Lane: verbs.LaneReview, Host: verbs.DirectHost, Req: req})
			usd, _ := models.Cost(cfg.Models.Prices, rm.Model, models.Billed(req, resp))
			return resp, usd, err
		}}}}
	p := e.progress(fmt.Sprintf("Session review by %s (%s)", m.Model, m.Family))
	res, st, err := r.Run(ctx, in, s.State)
	s.State = st
	if errors.Is(err, models.ErrBudget) {
		// The breaker tripped: agent lanes are paused, the build included.
		p.Done(ui.StatusFail, "budget breaker tripped")
		return false, rep, Wrap(ExitBudget, err)
	}
	if err != nil {
		p.Done(ui.StatusWarn, "could not run")
		return skip("the review by " + m.Model + " could not run: " + oneLineErr(err))
	}
	if !res.Cached {
		rd := sessionRound{Head: a.head, Model: m.Model, Family: string(m.Family), Builder: builder.Note(), Disposition: res.Disposition, CostUSD: res.CostUSD}
		for _, f := range res.Findings {
			if f.Severity != "P3" {
				rd.Items = append(rd.Items, sessionItem{ID: f.ID, Severity: f.Severity, File: f.File, Line: f.Line, Title: clipText(oneLine(f.Title), 200), Detail: clipText(oneLine(f.Detail), 600), Fix: clipText(oneLine(f.Fix), 400)})
			}
		}
		for _, d := range res.Dropped {
			if d.Why == inspect.DropRepeat && d.Severity == "P2" { // still raised; only not posted again
				rd.Items = append(rd.Items, sessionItem{ID: d.ID, Severity: d.Severity, File: d.File, Line: d.Line, Title: clipText(oneLine(d.Title), 200), Detail: clipText(oneLine(d.Detail), 600), Fix: clipText(oneLine(d.Fix), 400)})
			}
		}
		for _, v := range res.Criteria {
			if v.Verdict == "unmet" {
				rd.Items = append(rd.Items, sessionItem{ID: fmt.Sprintf("C%d", v.ID), Severity: "criterion", Title: fmt.Sprintf("criterion %d unmet", v.ID), Detail: clipText(oneLine(v.Evidence), 600)})
			}
		}
		s.Rounds = append(s.Rounds, rd)
	}
	s.Skipped = ""
	last := s.Rounds[len(s.Rounds)-1]
	rep = SessionReview{Round: len(s.Rounds), Model: last.Model, Disposition: last.Disposition, Open: s.open()}
	switch {
	case len(rep.Open) == 0 || last.Disposition == inspect.Merge:
		p.Done(ui.StatusPass, fmt.Sprintf("round %d: %s, %d left open", rep.Round, last.Disposition, len(rep.Open)))
	case len(s.Rounds)-s.Pushed >= inspect.MaxRounds:
		p.Done(ui.StatusWarn, fmt.Sprintf("round %d: %d left open at the cap of %d rounds; the merge gate decides", rep.Round, len(rep.Open), inspect.MaxRounds))
	default:
		p.Done(ui.StatusFail, fmt.Sprintf("round %d: %s, %d to fix or decline", rep.Round, last.Disposition, len(rep.Open)))
		s.Pending = a.head
		return true, rep, nil
	}
	s.Pending = ""
	return false, rep, nil
}

// sessionBuilder is the builder's family as inspect would place it from this
// branch: the build record's model per checkpoint, the commits' trailers, and
// the build lane on the host.
func (e *Env) sessionBuilder(ctx context.Context, gw *models.Gateway, a sessionReviewIn) inspect.Builder {
	recs := map[string]string{}
	for _, c := range a.st.Checkpoints {
		recs[c.Commit] = c.Model
	}
	out, _ := run2(ctx, a.wt, "git", "rev-list", "origin/"+a.dev+"..HEAD")
	var commits, claims, hosts []string
	for _, c := range strings.Fields(out) {
		commits = append(commits, c)
		msg, _ := run2(ctx, a.wt, "git", "log", "-1", "--format=%B", c)
		named := false
		for _, m := range modelTrailer.FindAllStringSubmatch(msg, -1) {
			named = named || verbs.Family(m[1]) != verbs.FamilyHostManaged
			if m := strings.ToLower(m[1]); !slices.Contains(claims, m) {
				claims = append(claims, m)
			}
		}
		if h := hostTrailer.FindStringSubmatch(msg); h != nil && !named && !slices.Contains(hosts, strings.ToLower(h[1])) {
			hosts = append(hosts, strings.ToLower(h[1]))
		}
	}
	lane := verbs.Family("")
	if host, _ := doctor.DetectHost(e.Host, e.Getenv); host != "" {
		if b, err := gw.Resolver.Resolve(verbs.LaneBuild, verbs.HostID(host)); err == nil {
			lane = b.Family
		}
	}
	return inspect.BuilderOf(inspect.Records{Checkpoints: recs}, commits, claims, hosts, lane)
}

// sessionSummary is the PR body's section on the session review: rounds,
// what was fixed, declined, or left open. Empty before any review or skip.
func (s *sessionReview) summary() string {
	if len(s.Rounds) == 0 && s.Skipped == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString(sessionReviewStart + "\n### Session review (self-attested)\n\n")
	b.WriteString("The build session reviewed its own change before pushing, with inspect's review rubric and a reviewer model of a family other than the builder's. This is the builder's own record, not a review that counts: a review written in the build session cannot be told from the builder's word (Decision #656). The merge gate's inspect reviews this PR and decides.\n\n")
	if len(s.Rounds) > 0 {
		b.WriteString("| Round | Head | Reviewer | Disposition | Left open |\n|---|---|---|---|---|\n")
		for i, r := range s.Rounds {
			fmt.Fprintf(&b, "| %d | `%s` | %s (%s) | %s | %d |\n", i+1, short(r.Head), sessionCell(r.Model), sessionCell(r.Family), r.Disposition, len(r.Items))
		}
		b.WriteString("\n")
		last := s.Rounds[len(s.Rounds)-1]
		seen := map[string]bool{}
		var fixed, declined, open []string
		for _, r := range s.Rounds {
			for _, it := range r.Items {
				if seen[it.ID] {
					continue
				}
				seen[it.ID] = true
				line := fmt.Sprintf("`%s` %s %s", it.ID, it.Severity, sessionCell(it.Title))
				if it.File != "" {
					line += fmt.Sprintf(" (%s:%d)", sessionCell(it.File), it.Line)
				}
				switch {
				case s.Declined[it.ID] != "":
					declined = append(declined, line+": "+sessionCell(s.Declined[it.ID]))
				case slices.ContainsFunc(last.Items, func(x sessionItem) bool { return x.ID == it.ID }):
					open = append(open, line)
				default:
					fixed = append(fixed, line)
				}
			}
		}
		for _, g := range []struct {
			head  string
			items []string
		}{{"Fixed (not raised by a later round)", fixed}, {"Declined by the builder, with its reason", declined}, {"Left open when it pushed (the reviewer said merge, or the round cap was reached)", open}} {
			if len(g.items) > 0 {
				b.WriteString("**" + g.head + ":**\n\n")
				for _, x := range g.items {
					b.WriteString("- " + x + "\n")
				}
				b.WriteString("\n")
			}
		}
	}
	if s.Skipped != "" {
		b.WriteString("The last push went out without a session review: " + sessionCell(s.Skipped) + ".\n\n")
	}
	b.WriteString(sessionReviewEnd)
	return b.String()
}

// withSessionSummary replaces the session review section of a PR body, or
// appends it, keeping everything else.
func withSessionSummary(body, summary string) string {
	if i := strings.Index(body, sessionReviewStart); i >= 0 {
		if j := strings.Index(body[i:], sessionReviewEnd); j >= 0 {
			return body[:i] + summary + body[i+j+len(sessionReviewEnd):]
		}
	}
	return strings.TrimRight(body, "\n") + "\n\n" + summary + "\n"
}

// cell keeps builder- and model-written text on one line, out of the table's
// columns and the body's markers.
func sessionCell(s string) string {
	return strings.NewReplacer("|", `\|`, "<!--", "&lt;!--", "-->", "--&gt;").Replace(clipText(oneLine(s), 300))
}

// gitSource reads the review context from the worktree's committed head, as
// the merge gate reads the PR head: never an uncommitted or ignored file, nor
// a symlink, which could point outside the repository.
type gitSource string

func (d gitSource) File(ctx context.Context, p string) ([]byte, error) {
	if !filepath.IsLocal(p) {
		return nil, fmt.Errorf("%s: outside the repository", p)
	}
	if m, _ := run2(ctx, string(d), "git", "ls-tree", "HEAD", "--", p); !strings.HasPrefix(m, "100") {
		return nil, fmt.Errorf("%s: not a regular file at the head", p)
	}
	var out bytes.Buffer
	cmd := exec.CommandContext(ctx, "git", "show", "HEAD:"+p)
	cmd.Dir, cmd.Stdout = string(d), &out
	err := cmd.Run()
	return out.Bytes(), err
}

func (d gitSource) Dir(ctx context.Context, dir string) ([]string, error) {
	if dir == "." {
		dir = ""
	}
	if dir != "" && !filepath.IsLocal(dir) {
		return nil, fmt.Errorf("%s: outside the repository", dir)
	}
	args := []string{"ls-tree", "HEAD"}
	if dir != "" {
		args = append(args, "--", strings.TrimSuffix(dir, "/")+"/")
	}
	out, err := run2(ctx, string(d), append([]string{"git"}, args...)...)
	var files []string
	for line := range strings.SplitSeq(out, "\n") {
		meta, path, ok := strings.Cut(line, "\t")
		if ok && strings.HasPrefix(meta, "100") {
			files = append(files, path)
		}
	}
	return files, err
}
