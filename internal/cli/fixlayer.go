package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"slices"
	"strings"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/audit"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// A correction's fix layer (ETHOS 10, Appendix B S-7.7d): the highest layer
// that can fix it for good. The layer is a model judgement with a recorded
// reason; aayu checks only that the record is complete and routes it: a
// codebase, check, or review correction becomes a Story whose Completion Goal
// names its proof, and only a prose correction goes to evolve's skill rewrite.

// FixLayerMarker marks the comment that records a Feedback issue's fix layer.
const FixLayerMarker = "<!-- aayudha:fix-layer -->"

var fixStateRe = regexp.MustCompile(`<!-- aayudha:fix-layer-state (\{.*\}) -->`)

// FixLayer is a Feedback issue's recorded classification.
type FixLayer struct {
	Feedback     int    `json:"feedback"`
	Layer        string `json:"layer"`
	Reason       string `json:"reason"`
	NotCatchable string `json:"not_catchable,omitempty"` // why no type, invariant, lint rule, or test can catch it
	Story        int    `json:"story,omitempty"`         // the Story filed for a codebase, check, or review correction
}

var layerName = map[string]string{workitems.FixCodebase: "codebase", workitems.FixCheck: "static check", workitems.FixProse: "prose", workitems.FixReview: "human review"}

var backtickedName = regexp.MustCompile("`[^`]+`")

// retroClassify is `aayu retro --classify <feedback> --layer … --reason …`.
func retroClassify(ctx context.Context, e *Env, in Values) error {
	n := issueNumber(in.String("classify"))
	fl := FixLayer{Feedback: n, Layer: in.String("layer"), Reason: strings.TrimSpace(in.String("reason")), NotCatchable: strings.TrimSpace(in.String("not-catchable"))}
	criterion := strings.TrimSpace(in.String("criterion"))
	below := fl.Layer == workitems.FixProse || fl.Layer == workitems.FixReview
	switch {
	case n <= 0:
		return Errorf(ExitUsage, "retro --classify %q: want a Feedback issue", in.String("classify"))
	case !slices.Contains(workitems.FixLayers, fl.Layer):
		return Errorf(ExitUsage, "retro --classify needs --layer: %s", strings.Join(workitems.FixLayers, ", "))
	case fl.Reason == "":
		return Errorf(ExitUsage, "retro --classify needs --reason: why %s is the highest layer that can fix the correction for good", fl.Layer)
	case below && fl.NotCatchable == "":
		return Errorf(ExitUsage, "retro --classify --layer %s needs --not-catchable: why no type, invariant, lint rule, or test can catch it", fl.Layer)
	case fl.Layer != workitems.FixProse && criterion == "":
		return Errorf(ExitUsage, "retro --classify --layer %s needs --criterion: the Completion Goal criterion of the Story that fixes it, whose proof is the new %s", fl.Layer, proofOf(fl.Layer))
	case fl.Layer == workitems.FixProse && criterion != "":
		return Errorf(ExitUsage, "retro --classify --layer prose files no Story, so it takes no --criterion; `aayu evolve` rewrites the skill")
	}
	if criterion != "" {
		if !regexp.MustCompile(`^[0-9]+[.)]\s`).MatchString(criterion) {
			criterion = "1. " + criterion
		}
		if _, ps := workitems.ParseCriteria(criterion); len(ps) > 0 {
			return Errorf(ExitUsage, "retro --classify --criterion: %s", ps[0].Message)
		}
		if !below && !backtickedName.MatchString(criterion) {
			return Errorf(ExitUsage, "retro --classify --criterion: name the type, invariant, lint rule, or test that is the proof, in backticks")
		}
	}
	repo, err := e.RepoName(ctx)
	if err != nil {
		return err
	}
	c, bot, err := e.BotGitHub(ctx, "")
	if err != nil {
		return err
	}
	owner, name, _ := github.SplitRepo(repo)
	is, _, err := c.REST.Issues.Get(ctx, owner, name, n)
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	labels := github.IssueLabels(is)
	if t, _ := workitems.TypeOf(is.GetType().GetName(), labels); t != "feedback" || is.IsPullRequest() {
		return Errorf(ExitUsage, "retro --classify: #%d is not a Feedback issue", n)
	}
	if is.GetState() != "open" {
		return Errorf(ExitBlocked, "retro --classify: Feedback #%d is closed", n)
	}
	prev, err := fixLayerState(ctx, c, repo, n)
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	fl.Story = prev.Story
	if fl.Layer != workitems.FixProse && fl.Story == 0 {
		if fl.Story, err = fileFixStory(ctx, c, repo, is, fl, criterion, bot.Tag); err != nil {
			return Wrap(ExitExternal, err)
		}
	}
	want := workitems.LabelFixPrefix + fl.Layer
	if err := setFixLabel(ctx, c, repo, n, labels, want); err != nil {
		return Wrap(ExitExternal, err)
	}
	// The Story carries its Feedback's fix:<layer> label, so fix work is
	// filterable and retro counts fix Stories shipped per layer (D-7.7e). A
	// Story filed earlier follows a reclassification, and loses the label when
	// the correction moves to prose, which no Story fixes.
	if prev.Story != 0 {
		st, _, err := c.REST.Issues.Get(ctx, owner, name, prev.Story)
		if err != nil {
			return Wrap(ExitExternal, err)
		}
		if fl.Layer == workitems.FixProse {
			want = ""
		}
		if err := setFixLabel(ctx, c, repo, prev.Story, github.IssueLabels(st), want); err != nil {
			return Wrap(ExitExternal, err)
		}
	}
	if _, err := c.UpsertComment(ctx, repo, n, FixLayerMarker, fixLayerComment(fl),
		actions.Action{Kind: "github.comment.fix-layer", Seat: "engineering-manager", Payload: map[string]any{"layer": fl.Layer, "story": fl.Story}}); err != nil {
		return Wrap(ExitExternal, err)
	}
	if !e.DryRun {
		if err := e.checkpoint(bot.Token); err != nil {
			fmt.Fprintf(e.Err, "aayu: warning: the actions segment did not reach the ledger: %v\n", err)
		}
	}
	return e.Emit(fl, func(w io.Writer) error {
		fmt.Fprintf(w, "Feedback #%d: fix layer %s (%s)\n", n, fl.Layer, layerName[fl.Layer])
		switch {
		case fl.Layer == workitems.FixProse:
			fmt.Fprintf(w, "  next: once accepted, `aayu evolve` rewrites the skill it names (eval-first)\n")
		case fl.Story != 0:
			fmt.Fprintf(w, "  fixed by Story #%d; next: `aayu grill story` on it, then `aayu ready %d`\n", fl.Story, fl.Story)
		}
		return nil
	})
}

// setFixLabel makes want the only fix:<layer> label of issue n, whose labels
// are labels; an empty want removes them all.
func setFixLabel(ctx context.Context, c *github.Client, repo string, n int, labels []string, want string) error {
	for _, l := range labels {
		if strings.HasPrefix(l, workitems.LabelFixPrefix) && l != want {
			if err := removeLabel(ctx, c, repo, n, l); err != nil {
				return err
			}
		}
	}
	if want == "" || slices.Contains(labels, want) {
		return nil
	}
	owner, name, _ := github.SplitRepo(repo)
	_, err := c.Write(ctx, actions.Action{Kind: "github.issue.labels.add", Target: fmt.Sprintf("%s#%d", repo, n), Seat: "engineering-manager",
		Request: &actions.Request{Method: http.MethodPost, Path: fmt.Sprintf("/repos/%s/issues/%d/labels", repo, n)}, Payload: map[string]any{"labels": []string{want}}},
		func(ctx context.Context) (any, error) {
			_, _, err := c.REST.Issues.AddLabelsToIssue(ctx, owner, name, n, []string{want})
			return nil, err
		})
	return err
}

func proofOf(layer string) string {
	switch layer {
	case workitems.FixCodebase:
		return "type, invariant, or boundary"
	case workitems.FixCheck:
		return "lint rule, test, or CI check"
	}
	return "thing the environment surfaces for review"
}

// fileFixStory files the Story that fixes a codebase, check, or review
// correction where it belongs.
func fileFixStory(ctx context.Context, c *github.Client, repo string, fb *gh.Issue, fl FixLayer, criterion, tag string) (int, error) {
	owner, name, _ := github.SplitRepo(repo)
	what := strings.TrimPrefix(strings.TrimPrefix(fb.GetTitle(), audit.FeedbackPrefix), "Feedback: ")
	title := fmt.Sprintf("Fix at the %s layer (Feedback #%d): %s", layerName[fl.Layer], fl.Feedback, what)
	summary := fmt.Sprintf("Feedback #%d is a correction classified at the %s layer: %s\n\nThis Story fixes it there, so the mistake becomes "+
		"impossible or caught rather than warned against in a skill. Its proof is the new %s that the Completion Goal names.", fl.Feedback, layerName[fl.Layer], fl.Reason, proofOf(fl.Layer))
	if fl.Layer == workitems.FixReview {
		summary = fmt.Sprintf("Feedback #%d is a correction classified at the human review layer: %s\n\nThis Story changes what the environment "+
			"surfaces for review (the proof comment, inspect's review plan, the risk line) so a person sees it at the right moment. Why no "+
			"type, invariant, lint rule, or test can catch it: %s", fl.Feedback, fl.Reason, fl.NotCatchable)
	}
	t, _ := workitems.FindType("story")
	body := workitems.Render(t, map[string]string{workitems.FieldSummary: summary, workitems.FieldCriteria: criterion})
	labels := []string{"type:story", "status:backlog", workitems.LabelFixPrefix + fl.Layer}
	if tag != "" {
		labels = append(labels, tag)
	}
	var num int
	_, err := c.Write(ctx, actions.Action{Kind: "github.issue.create", Target: repo, Seat: "engineering-manager",
		Request: &actions.Request{Method: http.MethodPost, Path: "/repos/" + repo + "/issues"}, Payload: map[string]any{"title": title, "feedback": fl.Feedback, "layer": fl.Layer}},
		func(ctx context.Context) (any, error) {
			is, _, err := c.REST.Issues.Create(ctx, owner, name, gh.CreateIssueRequest{Title: title, Body: gh.Ptr(body), Labels: labels})
			num = is.GetNumber()
			return map[string]any{"number": num}, err
		})
	return num, err
}

func fixLayerComment(fl FixLayer) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n### Fix layer: %s\n\n", FixLayerMarker, layerName[fl.Layer])
	b.WriteString("The highest layer that can fix this correction for good (codebase, static check, prose, human review; ETHOS 10). " +
		"Recorded by `aayu retro --classify`; the layer is a judgement, so its reason is kept here.\n\n")
	fmt.Fprintf(&b, "- **Reason:** %s\n", oneLine(fl.Reason))
	if fl.NotCatchable != "" {
		fmt.Fprintf(&b, "- **Why no type, invariant, lint rule, or test can catch it:** %s\n", oneLine(fl.NotCatchable))
	}
	switch {
	case fl.Layer == workitems.FixProse:
		b.WriteString("- **Fixed by:** a skill rewrite through `aayu evolve`, eval-first, once a human accepts this Feedback.\n")
	case fl.Story != 0:
		fmt.Fprintf(&b, "- **Fixed by:** Story #%d, whose Completion Goal names the new %s.\n", fl.Story, proofOf(fl.Layer))
	}
	st, _ := json.Marshal(fl)
	fmt.Fprintf(&b, "\n<!-- aayudha:fix-layer-state %s -->\n", st)
	return b.String()
}

func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

// fixLayerState reads the classification recorded on Feedback n (a zero
// value when none).
func fixLayerState(ctx context.Context, c *github.Client, repo string, n int) (FixLayer, error) {
	owner, name, _ := github.SplitRepo(repo)
	comments, err := github.All(ctx, func(ctx context.Context, o gh.ListOptions) ([]*gh.IssueComment, *gh.Response, error) {
		return c.REST.Issues.ListComments(ctx, owner, name, n, &gh.IssueListCommentsOptions{ListOptions: o})
	})
	var fl FixLayer
	for _, cm := range comments {
		if strings.Contains(cm.GetBody(), FixLayerMarker) {
			if m := fixStateRe.FindStringSubmatch(cm.GetBody()); m != nil {
				_ = json.Unmarshal([]byte(m[1]), &fl)
			}
		}
	}
	return fl, err
}

// evolveRefusal says why evolve may not rewrite a skill for a Feedback with
// this fix layer ("" when it may): only a prose correction, or a codebase or
// check correction whose classification says why code or a check cannot
// catch it.
func evolveRefusal(n int, layer string, st FixLayer) string {
	switch {
	case layer == "":
		return fmt.Sprintf("Feedback #%d has no fix layer: classify it first with `aayu retro --classify %d --layer … --reason …`", n, n)
	case layer == workitems.FixProse:
		return ""
	case layer == workitems.FixReview:
		return fmt.Sprintf("Feedback #%d is a human review correction: it changes what the environment surfaces for review (%s), not a skill", n, fixStoryRef(st))
	case st.Layer == layer && st.NotCatchable != "":
		return ""
	}
	return fmt.Sprintf("Feedback #%d is a %s correction, fixed by %s; a skill rewrite needs its classification to say why no type, invariant, lint rule, "+
		"or test can catch it (`aayu retro --classify %d --layer %s --not-catchable …`), or a reclassification to prose", n, layerName[layer], fixStoryRef(st), n, layer)
}

func fixStoryRef(st FixLayer) string {
	if st.Story == 0 {
		return "its Story"
	}
	return fmt.Sprintf("Story #%d", st.Story)
}
