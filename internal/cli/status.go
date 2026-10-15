package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/notify"
	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
	"github.com/rahulnakmol/agentic-ayudha/internal/ready"
	"github.com/rahulnakmol/agentic-ayudha/internal/status"
	"github.com/rahulnakmol/agentic-ayudha/internal/ui"
)

func init() { register("status", runStatus) }

// Notifier is the [notify] sender (WP1.6) for this run.
func (e *Env) Notifier(ctx context.Context) (*notify.Sender, error) {
	l, err := e.Config()
	if err != nil {
		return nil, err
	}
	clock, err := l.Config.Clock()
	if err != nil {
		return nil, Wrap(ExitUsage, err)
	}
	log, err := e.Actions()
	if err != nil {
		return nil, err
	}
	s := &notify.Sender{Clock: clock, Now: e.Now, State: filepath.Join(e.stateDir(), "state", "notify.json"), Actions: log, Sink: e.NotifySink}
	if s.Sink != nil {
		return s, nil
	}
	n := l.Config.Notify
	secret := func(name string) (string, error) {
		v, _, err := e.SecretStore().Get(name)
		if err != nil {
			return "", Wrap(ExitUsage, fmt.Errorf("[notify]: secret %s: %w", name, err))
		}
		return v, nil
	}
	switch n.Channel {
	case "slack":
		url, err := secret(n.Target)
		if err != nil {
			return nil, err
		}
		s.Sink = notify.Slack{WebhookURL: url, HTTP: e.HTTP}
	case "email":
		pw, err := secret(n.SMTPSecret)
		if err != nil {
			return nil, err
		}
		s.Sink = notify.Email{Addr: n.SMTPAddr, From: n.From, To: n.Target, Password: pw}
	case "file":
		path := n.Target
		switch {
		case path == "":
			path = filepath.Join(e.stateDir(), "state", "notify-outbox.jsonl")
		case !filepath.IsAbs(path):
			base := config.RepoRoot(e.Dir)
			if base == "" {
				base = e.Dir
			}
			path = filepath.Join(base, path)
		}
		s.Sink = notify.File{Path: path}
	case "github":
		tok, err := e.Resolver().Human(ctx)
		if err != nil {
			return nil, Wrap(ExitBlocked, err)
		}
		s.Sink = notify.GitHubMention{API: e.githubAPI(), Issue: n.Issue, Login: n.Target, Token: tok.Value, HTTP: e.HTTP}
	}
	return s, nil
}

// costs sums cost_usd per issue number from the local actions log.
func (e *Env) costs(repo string) map[int]float64 {
	out := map[int]float64{}
	f, err := os.Open(e.ActionsPath())
	if err != nil {
		return out
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 16<<20)
	for sc.Scan() {
		var r actions.Record
		if json.Unmarshal(sc.Bytes(), &r) != nil || r.CostUSD == 0 || r.Story == "" {
			continue
		}
		s := strings.TrimPrefix(strings.TrimPrefix(r.Story, repo), "#")
		if n, err := strconv.Atoi(s); err == nil {
			out[n] += r.CostUSD
		}
	}
	return out
}

type statusReport struct {
	Repo   string           `json:"repo"`
	Roots  []*status.Rollup `json:"roots"`
	Closed []int            `json:"closed,omitempty"`
	Digest string           `json:"digest,omitempty"`
	Sent   string           `json:"digest_status,omitempty"`
	GitHub github.Stats     `json:"github"`
}

type fleetRow struct {
	Repo  string `json:"repo"`
	Goal  int    `json:"goal,omitempty"`
	Title string `json:"title,omitempty"`
	Items int    `json:"items"`
	Done  int    `json:"done"`
	Block int    `json:"blocked"`
	Near  int    `json:"decisions_near_sla"`
	Error string `json:"error,omitempty"`
}

func (e *Env) walker(ctx context.Context, c *github.Client, repo string) (*status.Walker, error) {
	l, err := e.Config()
	if err != nil {
		return nil, err
	}
	clock, err := l.Config.Clock()
	if err != nil {
		return nil, Wrap(ExitUsage, err)
	}
	w := &status.Walker{GH: c, Repo: repo, Clock: clock, SLAHours: l.Config.Decisions.SLAHours, Now: e.Now(),
		Budget: l.Config.Budget.PerStoryUSD, Cost: e.costs(repo)}
	if config.RepoRoot(e.Dir) != "" {
		w.Proof = e.proofCovered(c, repo)
	}
	return w, nil
}

// proofCovered reports whether a done story is covered by proof (WP2.8): an
// authoritative pack on the ledger that maps every criterion of its Ready
// snapshot, whose artifacts match their hashes, and whose floors held (or
// were waived).
func (e *Env) proofCovered(c *github.Client, repo string) func(ctx context.Context, story int) (bool, error) {
	var (
		s *proof.Store
		t *ledger.Tree
	)
	return func(ctx context.Context, story int) (bool, error) {
		if t == nil {
			tok, err := e.Resolver().Human(ctx)
			if err != nil {
				return false, err
			}
			if s, err = e.proofStore(ctx, c, tok, "human", ""); err != nil {
				return false, err
			}
			if t, err = s.Ledger.Read(ctx); errors.Is(err, ledger.ErrNoLedger) {
				return false, nil
			} else if err != nil {
				return false, err
			}
		}
		b, err := t.File(ctx, ready.LedgerPath(story))
		if err != nil {
			return false, nil
		}
		snap, err := ready.ParseSnapshot(string(b))
		if err != nil {
			return false, nil
		}
		es, err := s.Packs(ctx, t, story)
		if err != nil {
			return false, err
		}
		for _, x := range es {
			if x.Pack.Authority.Kind != proof.Authoritative {
				continue
			}
			rep := proof.Verify(x.Pack, proof.Expect{CompletionGoalHash: snap.CompletionGoalHash, Criteria: snap.Numbers()}, s.Opener(ctx, t, x))
			if rep.OK && rep.FloorsHold {
				return true, nil
			}
		}
		return false, nil
	}
}

func runStatus(ctx context.Context, e *Env, in Values) error {
	if in.Bool("work") {
		return statusWork(ctx, e)
	}
	if in.Bool("streak") {
		if in.Bool("all") || in.Bool("digest") || in.Has("issue") {
			return Errorf(ExitUsage, "status --streak takes no issue, --all, or --digest")
		}
		return runStreak(ctx, e, in.Bool("apply"))
	}
	if in.Bool("all") {
		return statusAll(ctx, e)
	}
	repo, err := e.RepoName(ctx)
	if err != nil {
		return err
	}
	c, _, err := e.HumanGitHub(ctx)
	if err != nil {
		return err
	}
	w, err := e.walker(ctx, c, repo)
	if err != nil {
		return err
	}
	var roots []int
	if in.Has("issue") {
		roots = []int{issueNumber(in.String("issue"))}
	} else if roots, err = w.Goals(ctx); err != nil {
		return err
	}
	rep := statusReport{Repo: repo, Roots: []*status.Rollup{}}
	pr := e.progress(fmt.Sprintf("Rolling up %d item(s) from GitHub", len(roots)))
	for _, n := range roots {
		r, err := w.Roll(ctx, n)
		if err != nil {
			pr.Done(ui.StatusFail, err.Error())
			return err
		}
		rep.Roots = append(rep.Roots, r)
	}
	pr.Done(ui.StatusPass, "")
	if in.Bool("apply") {
		if rep.Closed, err = applyStatus(ctx, e, repo, rep.Roots); err != nil {
			return err
		}
	}
	if in.Bool("digest") {
		rep.Digest = status.Digest(repo, rep.Roots, e.Now())
		s, err := e.Notifier(ctx)
		if err != nil {
			return err
		}
		if rep.Sent, err = s.Page(ctx, notify.Page{Key: "status-digest:" + repo, Subject: "aayudha status digest: " + repo, Body: rep.Digest}); err != nil {
			return Wrap(ExitExternal, err)
		}
	}
	rep.GitHub = c.Stats()
	return e.Emit(rep, func(w io.Writer) error {
		if len(rep.Roots) == 0 {
			fmt.Fprintf(w, "%s: no open Goals (label type:goal); pass a Goal, Initiative, or Feature\n", repo)
		} else if look := e.Look(); look.Human() {
			fmt.Fprint(w, look.Table(statusRows(rep.Roots)))
			fmt.Fprintln(w, look.Style(ui.Muted, "Health, risk, decisions, and streak per item: aayu status --json"))
		} else {
			fmt.Fprint(w, status.Table(rep.Roots))
		}
		for _, r := range rep.Roots {
			fmt.Fprintln(w, r.Line())
		}
		for _, n := range rep.Closed {
			fmt.Fprintf(w, "closed #%d (parent-close rule)\n", n)
		}
		if rep.Sent != "" {
			fmt.Fprintf(w, "digest: %s\n", rep.Sent)
		}
		return nil
	})
}

// applyStatus posts the Goal comments and closes closable Features, as the bot
// (or the tagged human on the day-one path).
func applyStatus(ctx context.Context, e *Env, repo string, roots []*status.Rollup) ([]int, error) {
	c, bot, err := e.BotGitHub(ctx, "")
	if err != nil {
		return nil, err
	}
	return parentClose(ctx, c, repo, roots, func(r *status.Rollup) error {
		if r.Type != "goal" {
			return nil
		}
		_, err := c.UpsertComment(ctx, repo, r.Number, status.CommentMarker, status.Comment(r, e.Now()), actions.Action{Kind: "github.comment.status", Payload: map[string]any{"tag": bot.Tag}})
		return err
	})
}

// parentClose walks rollup trees top-down, calls visit on each node, and
// closes every node the parent-close rule allows (status.Rollup.Closable).
func parentClose(ctx context.Context, c *github.Client, repo string, roots []*status.Rollup, visit func(r *status.Rollup) error) ([]int, error) {
	owner, name, _ := github.SplitRepo(repo)
	var closed []int
	var walk func(r *status.Rollup) error
	walk = func(r *status.Rollup) error {
		if visit != nil {
			if err := visit(r); err != nil {
				return err
			}
		}
		if r.Closable {
			_, err := c.Write(ctx, actions.Action{Kind: "github.issue.close", Target: fmt.Sprintf("%s#%d", repo, r.Number),
				Request: &actions.Request{Method: http.MethodPatch, Path: fmt.Sprintf("/repos/%s/issues/%d", repo, r.Number)}, Payload: map[string]any{"rule": "parent-close"}},
				func(ctx context.Context) (any, error) {
					_, _, err := c.REST.Issues.Update(ctx, owner, name, r.Number, gh.UpdateIssueRequest{State: gh.Ptr("closed"), StateReason: gh.Ptr("completed")})
					return nil, err
				})
			if err != nil {
				return err
			}
			closed = append(closed, r.Number)
		}
		for _, ch := range r.Children {
			if err := walk(ch); err != nil {
				return err
			}
		}
		return nil
	}
	for _, r := range roots {
		if err := walk(r); err != nil {
			return closed, err
		}
	}
	return closed, nil
}

// statusAll prints one row per [fleet] repo with its top Goal's rollup.
func statusAll(ctx context.Context, e *Env) error {
	l, err := e.Config()
	if err != nil {
		return err
	}
	if len(l.Config.Fleet.Repos) == 0 {
		return Errorf(ExitUsage, "status --all: no [fleet] repos in the user config (~/.config/aayudha/config.toml)")
	}
	c, _, err := e.HumanGitHub(ctx)
	if err != nil {
		return err
	}
	var rows []fleetRow
	for _, repo := range l.Config.Fleet.Repos {
		row := fleetRow{Repo: repo}
		w, err := e.walker(ctx, c, repo)
		if err != nil {
			return err
		}
		goals, err := w.Goals(ctx)
		switch {
		case err != nil:
			row.Error = err.Error()
		case len(goals) == 0:
			row.Error = "no open Goal"
		default:
			r, err := w.Roll(ctx, goals[0])
			if err != nil {
				row.Error = err.Error()
				break
			}
			row.Goal, row.Title, row.Items, row.Done, row.Block, row.Near = r.Number, r.Title, r.Items, r.Done(), len(r.Blocked), len(r.DecisionsNearSLA)
		}
		rows = append(rows, row)
	}
	return e.Emit(rows, func(w io.Writer) error {
		for _, r := range rows {
			if r.Error != "" {
				fmt.Fprintf(w, "%-32s %s\n", r.Repo, r.Error)
				continue
			}
			fmt.Fprintf(w, "%-32s #%d %s: %d/%d done, %d blocked, %d decisions near SLA\n", r.Repo, r.Goal, r.Title, r.Done, r.Items, r.Block, r.Near)
		}
		return nil
	})
}

// statusRows is the rollup for a person: the columns that fit a terminal.
func statusRows(roots []*status.Rollup) ([]string, [][]string) {
	var rows [][]string
	var row func(r *status.Rollup, depth int)
	row = func(r *status.Rollup, depth int) {
		rows = append(rows, []string{fmt.Sprintf("%s#%d %s %s", strings.Repeat("  ", depth), r.Number, r.Type, r.Title),
			fmt.Sprintf("%d/%d", r.Done(), r.Items), fmt.Sprint(r.ByStatus["ready"]), fmt.Sprint(r.ByStatus["in-progress"]),
			fmt.Sprint(len(r.Blocked)), r.ProofCoverage, fmt.Sprintf("$%.2f", r.CostUSD)})
		for _, c := range r.Children {
			row(c, depth+1)
		}
	}
	for _, r := range roots {
		row(r, 0)
	}
	return []string{"Item", "Done", "Ready", "In progress", "Blocked", "Proof", "Cost"}, rows
}
