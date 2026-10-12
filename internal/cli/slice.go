package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/autonomy"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/planning"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

func init() { register("slice", runSlice) }

// SliceResult is what slice reports.
type SliceResult struct {
	planning.SliceReport
	ExpectedPaths []string `json:"expected_paths"`
	Issues        []int    `json:"issues,omitempty"` // the created stories, by draft position
	Closed        bool     `json:"closed_as_split,omitempty"`
	DryRun        bool     `json:"dry_run,omitempty"`
	Next          string   `json:"next,omitempty"`
}

// readDraft decodes a host's JSON draft strictly.
func readDraft(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return Wrap(ExitUsage, err)
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return Errorf(ExitUsage, "%s: %v", path, err)
	}
	return nil
}

// sliceSource reads what is sliced: a Feature, or an XL story under one.
func sliceSource(ctx context.Context, c *github.Client, repo string, n int) (planning.Source, error) {
	owner, name, _ := github.SplitRepo(repo)
	is, _, err := c.REST.Issues.Get(ctx, owner, name, n)
	if err != nil {
		return planning.Source{}, Wrap(ExitExternal, err)
	}
	it, _ := workitems.Parse(n, is.GetTitle(), is.GetBody(), is.GetType().GetName(), github.IssueLabels(is))
	src := planning.Source{Number: n, Type: it.Type, Effort: it.Label("effort"), Feature: n}
	switch {
	case is.GetState() != "open":
		return src, Errorf(ExitBlocked, "slice: #%d is closed", n)
	case it.Type == "feature":
		src.ExpectedPaths = planning.Paths(it.Fields["expected_paths"])
		return src, nil
	case it.Type == "story" && src.XL():
		parent, _, err := c.REST.SubIssue.GetParentIssue(ctx, owner, name, int64(n))
		if err != nil {
			return src, Errorf(ExitBlocked, "slice: XL story #%d has no parent Feature to slice it under: %v", n, err)
		}
		pt, _ := workitems.TypeOf(parent.GetType().GetName(), github.IssueLabels(parent))
		if pt != "feature" {
			return src, Errorf(ExitBlocked, "slice: XL story #%d's parent #%d is a %s, not a Feature", n, parent.GetNumber(), pt)
		}
		pit, _ := workitems.Parse(parent.GetNumber(), parent.GetTitle(), parent.GetBody(), parent.GetType().GetName(), github.IssueLabels(parent))
		src.Feature, src.ExpectedPaths = parent.GetNumber(), planning.Paths(pit.Fields["expected_paths"])
		return src, nil
	}
	return src, Errorf(ExitUsage, "slice: #%d is a %s (effort %q); slice takes a Feature or an XL story", n, it.Type, src.Effort)
}

// runSlice (WP5.4): the host's model slices (skills/slice); the binary
// computes the footprint from the Feature's expected paths through the
// LAYERS.md globs, checks every drafted story (XS-L, passes Ready under the
// Feature, paths within the Feature's, dependencies acyclic) and, with
// --create, writes the stories as sub-issues in dependency order with their
// layer labels and depends-on links, then closes an XL story as split.
func runSlice(ctx context.Context, e *Env, in Values) error {
	repo, err := e.RepoName(ctx)
	if err != nil {
		return err
	}
	n := issueNumber(in.String("feature"))
	c, bot, err := e.BotGitHub(ctx, "")
	if err != nil {
		return err
	}
	src, err := sliceSource(ctx, c, repo, n)
	if err != nil {
		return err
	}
	layers := autonomy.ReadLayers(config.RepoRoot(e.Dir))
	res := SliceResult{ExpectedPaths: src.ExpectedPaths}
	if !in.Has("draft") {
		res.SliceReport = planning.CheckSlice(src, planning.SliceDraft{}, layers)
		res.Pass, res.Stories, res.Problems = false, []planning.StoryReport{}, []string{}
		res.Next = fmt.Sprintf("draft XS-L stories within these paths (skills/slice), then `aayu slice %d --draft <file>`", n)
		return e.Emit(res, func(w io.Writer) error {
			fmt.Fprintf(w, "#%d: Feature #%d, footprint %s from expected paths %s\n  next: %s\n", n, src.Feature, listOrNone(res.Footprint), listOrNone(src.ExpectedPaths), res.Next)
			return nil
		})
	}
	var d planning.SliceDraft
	if err := readDraft(in.String("draft"), &d); err != nil {
		return err
	}
	res.SliceReport = planning.CheckSlice(src, d, layers)
	if res.Pass && in.Bool("create") {
		if err := e.createSlice(ctx, c, bot.Tag, repo, src, d, &res); err != nil {
			return err
		}
	}
	if err := e.Emit(res, func(w io.Writer) error {
		verdict := "passes"
		if !res.Pass {
			verdict = "does not pass yet"
		}
		fmt.Fprintf(w, "slice of #%d (Feature #%d, footprint %s): %d stories, %s\n", n, src.Feature, listOrNone(res.Footprint), len(d.Stories), verdict)
		for i, s := range res.Stories {
			created := ""
			if i < len(res.Issues) && res.Issues[i] > 0 {
				created = fmt.Sprintf(" → #%d", res.Issues[i])
			}
			fmt.Fprintf(w, "  %d. %s (effort %s, layers %s)%s\n", s.N, s.Title, s.Effort, listOrNone(s.Footprint), created)
			for _, p := range s.Problems {
				fmt.Fprintf(w, "     problem: %s\n", p)
			}
		}
		for _, p := range res.Problems {
			fmt.Fprintf(w, "  problem: %s\n", p)
		}
		switch {
		case res.DryRun:
			fmt.Fprintln(w, "  --dry-run: the stories are planned, not written")
		case res.Closed:
			fmt.Fprintf(w, "  closed #%d as split into the stories above\n", n)
		case res.Pass && !in.Bool("create"):
			fmt.Fprintln(w, "  run again with --create to write the stories")
		}
		return nil
	}); err != nil {
		return err
	}
	if !res.Pass {
		return Errorf(ExitBlocked, "the slice of #%d has problems; revise the draft", n)
	}
	return nil
}

func listOrNone(xs []string) string {
	if len(xs) == 0 {
		return "(none)"
	}
	return strings.Join(xs, ", ")
}

func (e *Env) createSlice(ctx context.Context, c *github.Client, tag, repo string, src planning.Source, d planning.SliceDraft, res *SliceResult) error {
	owner, name, _ := github.SplitRepo(repo)
	story, _ := workitems.FindType("story")
	res.Issues = make([]int, len(d.Stories))
	res.DryRun = c.DryRun()
	for _, pos := range res.Order {
		s := d.Stories[pos-1]
		it := s.Item(src.Feature)
		var links []string
		for _, dep := range s.DependsOn {
			links = append(links, fmt.Sprintf("depends-on #%d", res.Issues[dep-1]))
		}
		if len(links) > 0 {
			it.Fields[workitems.FieldLinks] = strings.Join(links, "\n")
		}
		body := workitems.Render(story, it.Fields)
		labels := it.Labels
		for _, l := range res.Stories[pos-1].Footprint {
			labels = append(labels, planning.LayerLabel+l)
		}
		if tag != "" {
			labels = append(labels, tag)
		}
		var created *gh.Issue
		if _, err := c.Write(ctx, actions.Action{Kind: "github.issue.create", Target: repo, Seat: "product-strategist",
			Request: &actions.Request{Method: http.MethodPost, Path: "/repos/" + repo + "/issues"}, Payload: map[string]any{"title": s.Title, "type": "story", "labels": labels, "slice_of": src.Number}},
			func(ctx context.Context) (any, error) {
				var err error
				created, _, err = c.REST.Issues.Create(ctx, owner, name, gh.CreateIssueRequest{Title: s.Title, Body: &body, Labels: labels})
				return map[string]any{"number": created.GetNumber()}, err
			}); err != nil {
			return Wrap(ExitExternal, err)
		}
		if created == nil {
			continue // --dry-run
		}
		res.Issues[pos-1] = created.GetNumber()
		id := created.GetID()
		if _, err := c.Write(ctx, actions.Action{Kind: "github.issue.sub-issue.add", Target: fmt.Sprintf("%s#%d", repo, src.Feature),
			Request: &actions.Request{Method: http.MethodPost, Path: fmt.Sprintf("/repos/%s/issues/%d/sub_issues", repo, src.Feature)}, Payload: map[string]any{"sub_issue": created.GetNumber()}},
			func(ctx context.Context) (any, error) {
				_, _, err := c.REST.SubIssue.Add(ctx, owner, name, int64(src.Feature), gh.SubIssueRequest{SubIssueID: id})
				return nil, err
			}); err != nil {
			return Wrap(ExitExternal, err)
		}
	}
	if !src.XL() || res.DryRun {
		return nil
	}
	return e.closeSplit(ctx, c, repo, src.Number, res)
}

// closeSplit closes an XL story as "split into" its new stories: a comment
// naming them, status:cancelled (labels first, so the Ready guard ignores
// it), and the issue closed as not planned.
func (e *Env) closeSplit(ctx context.Context, c *github.Client, repo string, n int, res *SliceResult) error {
	owner, name, _ := github.SplitRepo(repo)
	var refs []string
	for _, x := range res.Issues {
		refs = append(refs, fmt.Sprintf("#%d", x))
	}
	msg := fmt.Sprintf("Split into %s by `aayu slice` (WP5.4): an XL story is never built; its stories are XS-L.", strings.Join(refs, ", "))
	if _, err := c.Write(ctx, actions.Action{Kind: "github.issue.comment", Target: fmt.Sprintf("%s#%d", repo, n),
		Request: &actions.Request{Method: http.MethodPost, Path: fmt.Sprintf("/repos/%s/issues/%d/comments", repo, n)}, Payload: map[string]any{"split_into": res.Issues}},
		func(ctx context.Context) (any, error) {
			_, _, err := c.REST.Issues.CreateComment(ctx, owner, name, n, &gh.IssueComment{Body: &msg})
			return nil, err
		}); err != nil {
		return Wrap(ExitExternal, err)
	}
	is, _, err := c.REST.Issues.Get(ctx, owner, name, n)
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	if err := setStatus(ctx, c, repo, n, github.IssueLabels(is), "cancelled"); err != nil {
		return err
	}
	closed, reason := "closed", "not_planned"
	if _, err := c.Write(ctx, actions.Action{Kind: "github.issue.close", Target: fmt.Sprintf("%s#%d", repo, n),
		Request: &actions.Request{Method: http.MethodPatch, Path: fmt.Sprintf("/repos/%s/issues/%d", repo, n)}, Payload: map[string]any{"state_reason": reason}},
		func(ctx context.Context) (any, error) {
			_, _, err := c.REST.Issues.Update(ctx, owner, name, n, gh.UpdateIssueRequest{State: &closed, StateReason: &reason})
			return nil, err
		}); err != nil {
		return Wrap(ExitExternal, err)
	}
	res.Closed = true
	return nil
}
