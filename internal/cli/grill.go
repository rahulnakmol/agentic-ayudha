package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

func init() { register("grill", runGrill) }

// GrillResult is what grill reports.
type GrillResult struct {
	Type     string              `json:"type"`
	Title    string              `json:"title"`
	Parent   int                 `json:"parent,omitempty"`
	Labels   []string            `json:"labels"`
	Create   []workitems.Problem `json:"create_problems"`
	Ready    []workitems.Problem `json:"ready_problems"`
	Pass     bool                `json:"passes_ready"`
	Issue    int                 `json:"issue,omitempty"`
	URL      string              `json:"url,omitempty"`
	Identity string              `json:"identity,omitempty"`
	DryRun   bool                `json:"dry_run,omitempty"`
}

// runGrill is the thin-slice `grill story` (WP4.10): the interview is the
// skill's (skills/grill, references/story.md) and the host's model asks the
// questions; the binary validates the draft at Create and Ready and, with
// --create, writes it as an issue through the actions log. The TUI, the other
// types, and baselines are WP5.3.
func runGrill(ctx context.Context, e *Env, in Values) error {
	typ := in.String("type")
	if typ != "story" {
		return Errorf(ExitUsage, "grill %s: only `grill story` exists in this binary (WP4.10); the other types are n/a until WP5.3", typ)
	}
	repo, err := e.RepoName(ctx)
	if err != nil {
		return err
	}
	title, body := strings.TrimSpace(in.String("title")), in.String("body")
	if title == "" || strings.TrimSpace(body) == "" {
		return Errorf(ExitUsage, "grill story needs --title and --body: the draft the interview produced (skills/grill/references/story.md)")
	}
	labels := []string{"type:story", "status:planned"}
	for _, l := range strings.Split(in.String("labels"), ",") {
		if l = strings.TrimSpace(l); l == "" {
			continue
		}
		prefix, value, _ := strings.Cut(l, ":")
		known := false
		for _, f := range workitems.LabelFields {
			known = known || (f.Prefix == prefix && slices.Contains(f.Values, value))
		}
		if !known || prefix == "status" {
			return Errorf(ExitUsage, "grill: label %q is not a label-held field (priority, effort, risk, health, horizon)", l)
		}
		labels = append(labels, l)
	}
	res := GrillResult{Type: typ, Title: title, Labels: labels}
	it, ps := workitems.Parse(0, title, body, "", labels)
	res.Create = append(res.Create, ps...)
	res.Create = append(res.Create, workitems.Validate(it, workitems.StageCreate)...)

	c, bot, err := e.BotGitHub(ctx, "")
	if err != nil {
		return err
	}
	owner, name, err := github.SplitRepo(repo)
	if err != nil {
		return Wrap(ExitUsage, err)
	}
	var parent *gh.Issue
	if in.Has("parent") {
		res.Parent = issueNumber(in.String("parent"))
		if parent, _, err = c.REST.Issues.Get(ctx, owner, name, res.Parent); err != nil {
			return Wrap(ExitExternal, err)
		}
		pt, _ := workitems.TypeOf(parent.GetType().GetName(), github.IssueLabels(parent))
		it.Parent = &workitems.Ref{Number: res.Parent, Type: pt}
	}
	if it.Fields[workitems.FieldOwner] == "" {
		it.Fields[workitems.FieldOwner] = e.technicalOwner()
	}
	res.Ready = workitems.Validate(it, workitems.StageReady)
	res.Pass = len(res.Create) == 0 && len(res.Ready) == 0
	if res.Create == nil {
		res.Create = []workitems.Problem{}
	}
	if res.Ready == nil {
		res.Ready = []workitems.Problem{}
	}

	if in.Bool("create") && res.Pass {
		if bot.Tag != "" {
			res.Labels = append(res.Labels, bot.Tag)
		}
		res.Identity, res.DryRun = c.Identity(), c.DryRun()
		var created *gh.Issue
		_, err := c.Write(ctx, actions.Action{Kind: "github.issue.create", Target: repo, Seat: "product-strategist",
			Request: &actions.Request{Method: http.MethodPost, Path: "/repos/" + repo + "/issues"}, Payload: map[string]any{"title": title, "labels": res.Labels, "tag": bot.Tag}},
			func(ctx context.Context) (any, error) {
				var err error
				created, _, err = c.REST.Issues.Create(ctx, owner, name, gh.CreateIssueRequest{Title: title, Body: &body, Labels: res.Labels})
				return map[string]any{"number": created.GetNumber()}, err
			})
		if err != nil {
			return Wrap(ExitExternal, err)
		}
		if created != nil {
			res.Issue, res.URL = created.GetNumber(), created.GetHTMLURL()
			if parent != nil {
				id := created.GetID()
				_, err := c.Write(ctx, actions.Action{Kind: "github.issue.sub-issue.add", Target: fmt.Sprintf("%s#%d", repo, res.Parent),
					Request: &actions.Request{Method: http.MethodPost, Path: fmt.Sprintf("/repos/%s/issues/%d/sub_issues", repo, res.Parent)}, Payload: map[string]any{"sub_issue": res.Issue}},
					func(ctx context.Context) (any, error) {
						_, _, err := c.REST.SubIssue.Add(ctx, owner, name, int64(res.Parent), gh.SubIssueRequest{SubIssueID: id})
						return nil, err
					})
				if err != nil {
					return Wrap(ExitExternal, err)
				}
			}
		}
	}
	if err := e.Emit(res, func(w io.Writer) error {
		verdict := "passes Create and Ready"
		if !res.Pass {
			verdict = "does not pass yet"
		}
		fmt.Fprintf(w, "story draft %q: %s\n", title, verdict)
		for _, p := range res.Create {
			fmt.Fprintf(w, "  create: %s\n", p)
		}
		for _, p := range res.Ready {
			fmt.Fprintf(w, "  ready: %s\n", p)
		}
		switch {
		case res.DryRun:
			fmt.Fprintln(w, "  --dry-run: the issue is planned, not written")
		case res.Issue > 0:
			fmt.Fprintf(w, "  created #%d %s (labels %s); next: `aayu ready %d`\n", res.Issue, res.URL, strings.Join(res.Labels, ", "), res.Issue)
		case res.Pass && !in.Bool("create"):
			fmt.Fprintln(w, "  run again with --create to write it as an issue")
		}
		return nil
	}); err != nil {
		return err
	}
	if !res.Pass {
		return Errorf(ExitBlocked, "the story draft has %d problem(s); keep interviewing", len(res.Create)+len(res.Ready))
	}
	return nil
}
