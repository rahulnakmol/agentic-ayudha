package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/survey"
)

// DefaultSurveyLimit is the most issues one `survey --create` run files.
const DefaultSurveyLimit = 25

// SurveyBacklogResult is the check of a backlog draft and, with --create,
// what was filed, what was already there, and what waits for the next run.
type SurveyBacklogResult struct {
	survey.BacklogReport
	Review   string         `json:"backlog_md"`
	Filed    map[string]int `json:"filed,omitempty"`    // created by this run, key → issue
	Existing map[string]int `json:"existing,omitempty"` // filed earlier (found by marker), never edited
	Waiting  []string       `json:"waiting,omitempty"`  // past --limit, or under an item not filed yet
	DryRun   bool           `json:"dry_run,omitempty"`
	Next     string         `json:"next"`
}

// surveyFiled is <out>/filed.json: the issue each key was filed as, per
// repository, written after every issue so an interrupted run resumes.
type surveyFiled struct {
	Repo  string         `json:"repo"`
	Items map[string]int `json:"items"`
}

// surveyBacklog checks a backlog draft against the survey draft and the
// index and renders backlog.md for review. With --create and the digest the
// person reviewed, it files the items in order through the actions log:
// at most --limit issues, each with the survey marker and label, skipping
// every key already filed (by marker on GitHub or in filed.json) and never
// editing an existing issue.
func (e *Env) surveyBacklog(ctx context.Context, in Values, indexFile, out string) error {
	if !in.Has("draft") {
		return Errorf(ExitUsage, "survey --backlog needs the survey draft it rests on: --draft <file>")
	}
	ix, err := survey.Load(indexFile)
	if err != nil {
		return Errorf(ExitUsage, "survey --backlog: %v; run `aayu survey` first", err)
	}
	if err := ix.LoadProbes(out); err != nil {
		return Wrap(ExitUsage, err)
	}
	b, err := os.ReadFile(in.String("draft"))
	if err != nil {
		return Wrap(ExitUsage, err)
	}
	d, err := survey.ParseDraft(b)
	if err != nil {
		return Errorf(ExitUsage, "%s: %v", in.String("draft"), err)
	}
	if dr := survey.Check(ix, d); !dr.Pass {
		return Errorf(ExitBlocked, "the survey draft %s does not pass (`aayu survey --draft %s`): %s", in.String("draft"), in.String("draft"), strings.Join(dr.Problems, "; "))
	}
	if b, err = os.ReadFile(in.String("backlog")); err != nil {
		return Wrap(ExitUsage, err)
	}
	bl, err := survey.ParseBacklog(b)
	if err != nil {
		return Errorf(ExitUsage, "%s: %v", in.String("backlog"), err)
	}
	if bl.Product == "" {
		bl.Product = d.Product
	}
	res := SurveyBacklogResult{BacklogReport: survey.CheckBacklog(ix, d, bl), Review: filepath.Join(out, "backlog.md")}
	if err := os.WriteFile(res.Review, []byte(survey.RenderBacklog(bl, res.BacklogReport, in.String("draft"), in.String("backlog"))), 0o644); err != nil {
		return Wrap(ExitInternal, err)
	}
	res.Next = "fix every problem, then run again"
	if res.Pass {
		res.Next = fmt.Sprintf("show the person %s; file it only after they confirm: --create --reviewed %s", res.Review, res.Digest)
	}
	if res.Pass && in.Bool("create") {
		if got := in.String("reviewed"); got == "" || got != res.Digest {
			return Errorf(ExitBlocked, "survey --create files only the backlog the person reviewed: --reviewed %q does not match this draft's digest %s; show them %s again", got, res.Digest, res.Review)
		}
		if err := e.fileBacklog(ctx, in, bl, out, &res); err != nil {
			return err
		}
	}
	if err := e.Emit(res, func(w io.Writer) error { return writeBacklog(w, res) }); err != nil {
		return err
	}
	if !res.Pass {
		return Errorf(ExitBlocked, "the backlog draft has problems; revise it")
	}
	return nil
}

func (e *Env) fileBacklog(ctx context.Context, in Values, bl survey.Backlog, out string, res *SurveyBacklogResult) error {
	repo, err := e.RepoName(ctx)
	if err != nil {
		return err
	}
	c, tag, err := e.grillClient(ctx)
	if err != nil {
		return err
	}
	owner, name, _ := github.SplitRepo(repo)
	limit := in.Int("limit")
	if limit <= 0 {
		limit = DefaultSurveyLimit
	}
	// What is already filed: the markers on GitHub, then the state file.
	label := survey.Label(bl.Product)
	known := map[string]int{}
	issues, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.Issue, *gh.Response, error) {
		return c.REST.Issues.ListByRepo(ctx, owner, name, &gh.IssueListByRepoOptions{State: "all", Labels: []string{label}, ListOptions: o})
	})
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	for _, is := range issues {
		if k, ok := survey.ParseMarker(bl.Product, is.GetBody()); ok && !is.IsPullRequest() {
			known[k] = is.GetNumber()
		}
	}
	statePath := filepath.Join(out, "filed.json")
	var state surveyFiled
	if b, err := os.ReadFile(statePath); err == nil && json.Unmarshal(b, &state) == nil && state.Repo == repo {
		for k, n := range state.Items {
			if _, ok := known[k]; !ok && n > 0 {
				if is, _, err := c.REST.Issues.Get(ctx, owner, name, n); err == nil {
					if key, ok := survey.ParseMarker(bl.Product, is.GetBody()); ok && key == k {
						known[k] = n
					}
				}
			}
		}
	}
	state = surveyFiled{Repo: repo, Items: map[string]int{}}
	for k, n := range known {
		state.Items[k] = n
	}
	byKey := map[string]survey.Item{}
	for _, it := range bl.Items {
		byKey[it.Key] = it
	}
	res.Filed, res.Existing, res.DryRun = map[string]int{}, map[string]int{}, c.DryRun()
	for _, key := range res.Order {
		it := byKey[key]
		if n, ok := known[key]; ok {
			res.Existing[key] = n
			continue
		}
		var deps []string
		ready := true
		for _, dep := range slices.Concat([]string{it.Parent}, it.DependsOn, it.Constrains) {
			if dep == "" {
				continue
			}
			n, ok := known[dep]
			if !ok && !res.DryRun {
				ready = false
				break
			}
			switch {
			case slices.Contains(it.Constrains, dep):
				deps = append(deps, fmt.Sprintf("constrains #%d", n))
			case dep != it.Parent:
				deps = append(deps, fmt.Sprintf("depends-on #%d", n))
			}
		}
		if !ready || len(res.Filed) >= limit {
			res.Waiting = append(res.Waiting, key)
			continue
		}
		body := survey.Body(bl.Product, it, strings.Join(deps, "\n"))
		labels := slices.Concat([]string{"type:" + it.Type, "status:backlog", label}, it.Labels)
		if tag != "" {
			labels = append(labels, tag)
		}
		var created *gh.Issue
		if _, err := c.Write(ctx, actions.Action{Kind: "github.issue.create", Target: repo, Seat: "code-architect",
			Request: &actions.Request{Method: http.MethodPost, Path: "/repos/" + repo + "/issues"},
			Payload: map[string]any{"title": it.Title, "type": it.Type, "labels": labels, "survey_key": key, "digest": res.Digest}},
			func(ctx context.Context) (any, error) {
				var err error
				created, _, err = c.REST.Issues.Create(ctx, owner, name, gh.CreateIssueRequest{Title: it.Title, Body: &body, Labels: labels})
				return map[string]any{"number": created.GetNumber()}, err
			}); err != nil {
			return Wrap(ExitExternal, err)
		}
		if created == nil { // --dry-run: planned, not written
			res.Filed[key] = 0
			continue
		}
		n := created.GetNumber()
		known[key], res.Filed[key], state.Items[key] = n, n, n
		if b, err := json.MarshalIndent(state, "", "  "); err == nil {
			if err := os.WriteFile(statePath, append(b, '\n'), 0o644); err != nil {
				return Wrap(ExitInternal, err)
			}
		}
		if p, ok := known[it.Parent]; ok && it.Parent != "" {
			id := created.GetID()
			if _, err := c.Write(ctx, actions.Action{Kind: "github.issue.sub-issue.add", Target: fmt.Sprintf("%s#%d", repo, p),
				Request: &actions.Request{Method: http.MethodPost, Path: fmt.Sprintf("/repos/%s/issues/%d/sub_issues", repo, p)}, Payload: map[string]any{"sub_issue": n}},
				func(ctx context.Context) (any, error) {
					_, _, err := c.REST.SubIssue.Add(ctx, owner, name, int64(p), gh.SubIssueRequest{SubIssueID: id})
					return nil, err
				}); err != nil {
				return Wrap(ExitExternal, err)
			}
		}
	}
	switch {
	case len(res.Waiting) > 0:
		res.Next = fmt.Sprintf("%d item(s) wait (--limit %d, or a parent not filed yet): run the same --create again to file the next ones", len(res.Waiting), limit)
	default:
		res.Next = "every item is filed; `aayu ready <story>` makes a Story Ready once its plan exists (`aayu plan <story>`)"
	}
	return nil
}

func writeBacklog(w io.Writer, r SurveyBacklogResult) error {
	verdict := "passes"
	if !r.Pass {
		verdict = "does not pass yet"
	}
	var counts []string
	for _, t := range survey.BacklogTypes {
		if n := r.Counts[t]; n > 0 {
			counts = append(counts, fmt.Sprintf("%d %s", n, t))
		}
	}
	fmt.Fprintf(w, "backlog draft: %s (%s); review digest %s\n", verdict, strings.Join(counts, ", "), r.Digest)
	for _, p := range r.Problems {
		fmt.Fprintf(w, "  problem: %s\n", p)
	}
	for _, ir := range r.Items {
		for _, p := range ir.Problems {
			fmt.Fprintf(w, "  %s (%s): %s\n", ir.Key, ir.Type, p)
		}
	}
	for _, k := range r.Order {
		switch n, filed := r.Filed[k]; {
		case filed && r.DryRun:
			fmt.Fprintf(w, "  %s: planned (--dry-run)\n", k)
		case filed:
			fmt.Fprintf(w, "  %s: filed #%d\n", k, n)
		case r.Existing[k] > 0:
			fmt.Fprintf(w, "  %s: already filed as #%d (not edited)\n", k, r.Existing[k])
		case slices.Contains(r.Waiting, k):
			fmt.Fprintf(w, "  %s: waiting for the next run\n", k)
		}
	}
	fmt.Fprintf(w, "  wrote %s\n  next: %s\n", r.Review, r.Next)
	return nil
}
