// Package ready implements `aayu ready` (WP2.7): it parses an issue into a
// work item, validates it for Ready, and, when it passes, writes the Ready
// snapshot to the ledger, posts it as a replace-in-place comment, and labels
// the issue status:ready. --check only validates. The Ready guard (run by the
// aayudha-ready-guard workflow) strips status:ready from an issue that no
// longer validates or whose body drifted from its snapshot.
package ready

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strings"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// Markers of the comments ready and the guard keep in place.
const (
	SnapshotMarker = "<!-- aayudha:ready-snapshot -->"
	GuardMarker    = "<!-- aayudha:ready-guard -->"
	LabelReady     = "status:ready"
)

// TwinOwner is the Technical Owner while seats have no humans (WP5.1 n/a).
const TwinOwner = "the Code Architect twin"

// Result is what ready reports.
type Result struct {
	Repo               string                `json:"repo"`
	Issue              int                   `json:"issue"`
	Title              string                `json:"title"`
	Type               string                `json:"type"`
	Pass               bool                  `json:"pass"`
	Problems           []workitems.Problem   `json:"problems"`
	BodyHash           string                `json:"body_hash"`
	CompletionGoalHash string                `json:"completion_goal_hash,omitempty"`
	Criteria           []workitems.Criterion `json:"criteria,omitempty"`
	Measure            string                `json:"measure,omitempty"` // with Threshold, Scenario, Environment: the rest of the Completion Goal, for a readback
	Threshold          string                `json:"threshold,omitempty"`
	Scenario           string                `json:"scenario,omitempty"`
	Environment        string                `json:"environment,omitempty"`
	TechnicalOwner     string                `json:"technical_owner,omitempty"`
	Identity           string                `json:"identity,omitempty"`
	Tag                string                `json:"tag,omitempty"`
	LedgerPath         string                `json:"ledger_path,omitempty"`
	LedgerCommit       string                `json:"ledger_commit,omitempty"`
	Stripped           string                `json:"stripped,omitempty"` // guard: why status:ready was removed
	DryRun             bool                  `json:"dry_run,omitempty"`

	body string
}

// Ready is one run of the verb.
type Ready struct {
	GH             *github.Client
	Repo           string
	TechnicalOwner string // the seat's paired human, else TwinOwner
	Tag            string // identity:human-fallback on the day-one path
	Login          string // who writes: the bot login, or the human's on the fallback path
	Ledger         func(ctx context.Context, c ledger.Commit) (string, error)
	// Concrete, when set, adds grill's concreteness check (WP5.3b): no vague
	// qualifier left in the criteria, and a grilled spec's provenance holds.
	// The guard leaves it unset, so Ready that already holds is never
	// withdrawn by a rule added after it was granted.
	Concrete func(workitems.Item) []workitems.Problem
}

// Check fetches and validates an issue for Ready. It writes nothing.
func (r *Ready) Check(ctx context.Context, n int) (Result, *gh.Issue, error) {
	owner, name, err := github.SplitRepo(r.Repo)
	if err != nil {
		return Result{}, nil, err
	}
	is, _, err := r.GH.REST.Issues.Get(ctx, owner, name, n)
	if err != nil {
		return Result{}, nil, err
	}
	labels := github.IssueLabels(is)
	it, ps := workitems.Parse(n, is.GetTitle(), is.GetBody(), is.GetType().GetName(), labels)
	res := Result{Repo: r.Repo, Issue: n, Title: is.GetTitle(), Type: it.Type, BodyHash: workitems.BodyHash(is.GetBody()), body: is.GetBody()}
	if it.Type != "" {
		parent, _, perr := r.GH.REST.SubIssue.GetParentIssue(ctx, owner, name, int64(n))
		switch {
		case perr == nil:
			pt, _ := workitems.TypeOf(parent.GetType().GetName(), github.IssueLabels(parent))
			it.Parent = &workitems.Ref{Number: parent.GetNumber(), Type: pt}
		case !github.NotFound(perr):
			return res, is, perr
		}
		links, _ := workitems.ParseLinks(it.Fields[workitems.FieldLinks])
		for _, l := range links {
			target, _, lerr := r.GH.REST.Issues.Get(ctx, owner, name, l.Number)
			if github.NotFound(lerr) {
				ps = append(ps, workitems.Problem{Field: workitems.FieldLinks, Message: fmt.Sprintf("%s #%d: no such issue", l.Kind, l.Number)})
				continue
			}
			if lerr != nil {
				return res, is, lerr
			}
			if it.LinkTypes == nil {
				it.LinkTypes = map[int]string{}
			}
			it.LinkTypes[l.Number], _ = workitems.TypeOf(target.GetType().GetName(), github.IssueLabels(target))
		}
		ps = append(ps, workitems.Validate(it, workitems.StageReady)...)
		if r.Concrete != nil {
			ps = append(ps, r.Concrete(it)...)
		}
		res.CompletionGoalHash = workitems.CompletionGoalHash(it)
		res.Criteria, _ = workitems.ParseCriteria(it.Fields[workitems.FieldCriteria])
		res.Measure, res.Threshold = it.Fields[workitems.FieldMeasure], it.Fields[workitems.FieldThreshold]
		res.Scenario, res.Environment = it.Fields[workitems.FieldScenario], it.Fields[workitems.FieldEnvironment]
		res.TechnicalOwner = it.Fields[workitems.FieldOwner]
		if res.TechnicalOwner == "" {
			res.TechnicalOwner = r.TechnicalOwner
		}
		if res.TechnicalOwner == "" {
			ps = append(ps, workitems.Problem{Field: workitems.FieldOwner, Message: "no Technical Owner could be resolved"})
		}
	}
	if t, ok := workitems.FindType(it.Type); ok && t.Intake {
		ps = append(ps, workitems.Problem{Field: "type", Message: "a " + t.Name + " is intake: triage classifies it; it is never made Ready"})
	}
	res.Problems = ps
	res.Pass = len(ps) == 0
	if res.Problems == nil {
		res.Problems = []workitems.Problem{}
	}
	return res, is, nil
}

// LedgerPath is where a work item's Ready snapshot lives on the ledger.
func LedgerPath(n int) string { return fmt.Sprintf("specs/%d/ready.md", n) }

// Snapshot renders the Ready snapshot comment.
func Snapshot(res Result) string {
	var b strings.Builder
	b.WriteString(SnapshotMarker + "\n### Ready snapshot\n\n| | |\n|---|---|\n")
	fmt.Fprintf(&b, "| Schema | work items v%d, %s |\n", workitems.SchemaVersion, res.Type)
	fmt.Fprintf(&b, "| Body hash | `%s` |\n", res.BodyHash)
	cg := "n/a (no Completion Goal on this type)"
	if res.CompletionGoalHash != "" {
		cg = "`" + res.CompletionGoalHash + "`"
	}
	fmt.Fprintf(&b, "| Completion Goal hash | %s |\n", cg)
	fmt.Fprintf(&b, "| Technical Owner | %s |\n", res.TechnicalOwner)
	who := res.Identity
	if res.Tag != "" {
		who += " (`" + res.Tag + "`)"
	}
	fmt.Fprintf(&b, "| Written by | %s |\n", who)
	fmt.Fprintf(&b, "| Ledger | `%s` |\n", res.LedgerPath)
	if len(res.Criteria) > 0 {
		b.WriteString("\nCompletion Goal at Ready:\n\n")
		for _, c := range res.Criteria {
			fmt.Fprintf(&b, "%d. [%s] state: %s; expected: %s\n", c.N, c.Evidence, c.State, c.Expected)
		}
	}
	b.WriteString("\nEditing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.\n")
	return b.String()
}

var snapshotHash = regexp.MustCompile("\\| Body hash \\| `([0-9a-f]{64})` \\|")

// Apply validates and, when the item passes, writes the snapshot to the
// ledger, posts it, and labels the issue status:ready. A failing item is
// returned with its problems and nothing is written.
func (r *Ready) Apply(ctx context.Context, n int) (Result, error) {
	res, is, err := r.Check(ctx, n)
	if err != nil || !res.Pass {
		return res, err
	}
	res.Identity, res.Tag, res.LedgerPath, res.DryRun = r.Login, r.Tag, LedgerPath(n), r.GH.DryRun()
	snap := Snapshot(res)
	if r.Ledger != nil {
		file := fmt.Sprintf("# %s#%d: %s\n\n%s\n## Body at Ready\n\n````markdown\n%s\n````\n", r.Repo, n, res.Title, strings.Replace(snap, SnapshotMarker+"\n", "", 1), strings.TrimSpace(res.body))
		sha, err := r.Ledger(ctx, ledger.Commit{Message: fmt.Sprintf("specs: Ready snapshot of #%d", n), Files: map[string][]byte{res.LedgerPath: []byte(file)}})
		if err != nil {
			return res, fmt.Errorf("ledger: %w", err)
		}
		res.LedgerCommit = sha
	}
	if _, err := r.GH.UpsertComment(ctx, r.Repo, n, SnapshotMarker, snap, actions.Action{Kind: "github.comment.ready-snapshot",
		Payload: map[string]any{"body_hash": res.BodyHash, "completion_goal_hash": res.CompletionGoalHash, "tag": r.Tag}}); err != nil {
		return res, err
	}
	labels := github.IssueLabels(is)
	for _, l := range labels {
		if strings.HasPrefix(l, "status:") && l != LabelReady {
			if err := r.removeLabel(ctx, n, l); err != nil {
				return res, err
			}
		}
	}
	add := []string{}
	if !slices.Contains(labels, LabelReady) {
		add = append(add, LabelReady)
	}
	if r.Tag != "" && !slices.Contains(labels, r.Tag) {
		add = append(add, r.Tag)
	}
	if len(add) > 0 {
		owner, name, _ := github.SplitRepo(r.Repo)
		_, err := r.GH.Write(ctx, actions.Action{Kind: "github.issue.labels.add", Target: fmt.Sprintf("%s#%d", r.Repo, n),
			Request: &actions.Request{Method: http.MethodPost, Path: fmt.Sprintf("/repos/%s/issues/%d/labels", r.Repo, n)}, Payload: map[string]any{"labels": add}},
			func(ctx context.Context) (any, error) {
				_, _, err := r.GH.REST.Issues.AddLabelsToIssue(ctx, owner, name, n, add)
				return nil, err
			})
		if err != nil {
			return res, err
		}
	}
	return res, nil
}

func (r *Ready) removeLabel(ctx context.Context, n int, label string) error {
	owner, name, _ := github.SplitRepo(r.Repo)
	_, err := r.GH.Write(ctx, actions.Action{Kind: "github.issue.labels.remove", Target: fmt.Sprintf("%s#%d", r.Repo, n),
		Request: &actions.Request{Method: http.MethodDelete, Path: fmt.Sprintf("/repos/%s/issues/%d/labels/%s", r.Repo, n, label)}, Payload: map[string]any{"label": label}},
		func(ctx context.Context) (any, error) {
			_, err := r.GH.REST.Issues.RemoveLabelForIssue(ctx, owner, name, n, label)
			if github.NotFound(err) {
				err = nil
			}
			return nil, err
		})
	return err
}

// Guard re-checks an issue labelled status:ready. When it no longer validates,
// has no snapshot, or its body hash differs from the snapshot's, the guard
// removes status:ready and explains why in a replace-in-place comment.
func (r *Ready) Guard(ctx context.Context, n int) (Result, error) {
	res, is, err := r.Check(ctx, n)
	if err != nil {
		return res, err
	}
	if !slices.Contains(github.IssueLabels(is), LabelReady) {
		return res, nil
	}
	snapHash, err := SnapshotHash(ctx, r.GH, r.Repo, n)
	if err != nil {
		return res, err
	}
	switch {
	case !res.Pass:
		var ps []string
		for _, p := range res.Problems {
			ps = append(ps, "- "+p.String())
		}
		res.Stripped = "it no longer passes Ready:\n\n" + strings.Join(ps, "\n")
	case snapHash == "":
		res.Stripped = "it has no Ready snapshot: `status:ready` is set only by `aayu ready`."
	case snapHash != res.BodyHash:
		res.Stripped = fmt.Sprintf("its body changed after Ready (snapshot `%s`, now `%s`).", snapHash[:12], res.BodyHash[:12])
	default:
		return res, nil
	}
	if err := r.removeLabel(ctx, n, LabelReady); err != nil {
		return res, err
	}
	msg := GuardMarker + "\nRemoved `status:ready`: " + res.Stripped + "\n\nFix the issue, then run `aayu ready " + fmt.Sprint(n) + "` to snapshot it again.\n"
	_, err = r.GH.UpsertComment(ctx, r.Repo, n, GuardMarker, msg, actions.Action{Kind: "github.comment.ready-guard"})
	return res, err
}

// SnapshotHash returns the body hash in an issue's Ready snapshot comment, or
// "" when it has none.
func SnapshotHash(ctx context.Context, c *github.Client, repo string, n int) (string, error) {
	owner, name, err := github.SplitRepo(repo)
	if err != nil {
		return "", err
	}
	comments, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.IssueComment, *gh.Response, error) {
		return c.REST.Issues.ListComments(ctx, owner, name, n, &gh.IssueListCommentsOptions{ListOptions: o})
	})
	if err != nil {
		return "", err
	}
	hash := ""
	for _, cm := range comments {
		if strings.Contains(cm.GetBody(), SnapshotMarker) {
			if m := snapshotHash.FindStringSubmatch(cm.GetBody()); m != nil {
				hash = m[1]
			}
		}
	}
	return hash, nil
}

// LedgerSnapshot is a Ready snapshot as written to the ledger (specs/<n>/ready.md):
// what `proof verify`, the merge gate, and the screen-reader floor compare
// against, rather than anything an agent says at run time.
type LedgerSnapshot struct {
	BodyHash           string                `json:"body_hash"`
	CompletionGoalHash string                `json:"completion_goal_hash"`
	Criteria           []workitems.Criterion `json:"criteria"`
}

var (
	snapshotCGHash = regexp.MustCompile("\\| Completion Goal hash \\| `([0-9a-f]{64})` \\|")
	snapshotBody   = regexp.MustCompile("(?s)\n## Body at Ready\n\n````markdown\n(.*)\n````\n?$")
)

// ParseSnapshot reads a ledger Ready snapshot. The criteria come from the
// Completion Goal of the body at Ready.
func ParseSnapshot(file string) (LedgerSnapshot, error) {
	var s LedgerSnapshot
	m := snapshotHash.FindStringSubmatch(file)
	if m == nil {
		return s, fmt.Errorf("not a Ready snapshot: no body hash")
	}
	s.BodyHash = m[1]
	if m := snapshotCGHash.FindStringSubmatch(file); m != nil {
		s.CompletionGoalHash = m[1]
	}
	if m := snapshotBody.FindStringSubmatch(file); m != nil {
		_, values := workitems.Sections(m[1])
		s.Criteria, _ = workitems.ParseCriteria(values["Completion Goal"])
	}
	return s, nil
}

// Numbers returns the snapshot's criterion numbers.
func (s LedgerSnapshot) Numbers() []int {
	ns := []int{}
	for _, c := range s.Criteria {
		ns = append(ns, c.N)
	}
	return ns
}
