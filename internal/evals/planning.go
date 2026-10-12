package evals

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/autonomy"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/models"
	"github.com/rahulnakmol/agentic-ayudha/internal/planning"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// draftShapes tell the skill the JSON a slice or plan draft takes.
var draftShapes = map[string]string{
	"slice": `{feature, stories: [{title, fields, labels, paths, depends_on}], open_questions}`,
	"plan":  `{story, checkpoints: [{layer, title, contracts, tests, instrumentation, rollback, criteria}], open_questions}`,
}

// runPlanning is one tier-2 run of a slice or plan case (WP5.4): the skill
// drafts, the binary's checks (internal/planning) run first against the
// case's fixture, and only a draft that passes them is judged (rule 7).
func (h *Harness) runPlanning(ctx context.Context, c Case, lane verbs.LaneID, body, input, rubric string) (run Run, err error) {
	var calls []CallLog
	defer func() { run.Calls = calls }()
	shape, ok := draftShapes[c.Tier2.Draft]
	if !ok {
		return Run{}, fmt.Errorf("%s: [tier2] draft %q: want slice or plan", c.ID, c.Tier2.Draft)
	}
	schema, err := os.ReadFile(filepath.Join(h.Root, "evals", "schemas", c.Tier2.Draft+".json"))
	if err != nil {
		return Run{}, err
	}
	req := models.Request{
		System:    body + "\n\nReply with the draft you produce as JSON: " + shape + ".",
		Messages:  []models.Message{{Role: "user", Content: untrusted("eval-input", input)}},
		MaxTokens: h.Config.MaxTokens.Skill,
	}
	var raw json.RawMessage
	if err := models.Structured(ctx, h.complete(lane, "skill", &calls), req, schema, &raw); err != nil {
		var re *models.ReplyError
		if errors.As(err, &re) && !re.Parse && !re.Truncated {
			return Run{Status: RunFail, Score: 1, Floor: "output is not a valid " + c.Tier2.Draft + " draft: " + err.Error()}, nil
		}
		return Run{}, err
	}
	problems, open, err := CheckPlanningDraft(c, raw)
	if err != nil {
		return Run{}, err
	}
	if len(problems) > 0 {
		return Run{Status: RunFail, Score: 1, Floor: "fails `aayu " + c.Tier2.Draft + "`'s checks: " + strings.Join(problems, "; ")}, nil
	}
	var out bytes.Buffer
	_ = json.Indent(&out, raw, "", "  ")
	return h.judge(ctx, input, rubric, out.String(), open, &calls)
}

// CheckPlanningDraft runs `aayu slice` or `aayu plan`'s checks on a draft for
// the case's fixture issue, as the binary would against GitHub.
func CheckPlanningDraft(c Case, raw []byte) (problems, open []string, err error) {
	is, err := fixtureIssue(c, strconv.Itoa(c.Tier2.Issue))
	if err != nil {
		return nil, nil, err
	}
	labels := github.IssueLabels(is)
	it, _ := workitems.Parse(c.Tier2.Issue, is.GetTitle(), is.GetBody(), "", labels)
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	switch c.Tier2.Draft {
	case "slice":
		var d planning.SliceDraft
		if err := dec.Decode(&d); err != nil {
			return []string{err.Error()}, nil, nil
		}
		src := planning.Source{Number: c.Tier2.Issue, Type: it.Type, Effort: it.Label("effort"), Feature: c.Tier2.Issue, ExpectedPaths: planning.Paths(it.Fields["expected_paths"])}
		if it.Type == "story" {
			p, err := fixtureIssue(c, strconv.Itoa(c.Tier2.Issue)+"/parent")
			if err != nil {
				return nil, nil, err
			}
			pit, _ := workitems.Parse(p.GetNumber(), p.GetTitle(), p.GetBody(), "", github.IssueLabels(p))
			src.Feature, src.ExpectedPaths = p.GetNumber(), planning.Paths(pit.Fields["expected_paths"])
		}
		r := planning.CheckSlice(src, d, autonomy.ReadLayers(""))
		problems = append(problems, r.Problems...)
		for _, s := range r.Stories {
			for _, p := range s.Problems {
				problems = append(problems, fmt.Sprintf("story %d: %s", s.N, p))
			}
		}
		return problems, d.OpenQuestions, nil
	case "plan":
		var d planning.PlanDraft
		if err := dec.Decode(&d); err != nil {
			return []string{err.Error()}, nil, nil
		}
		r := planning.CheckPlan(it, planning.Footprint(labels), nil, d)
		return r.Problems, d.OpenQuestions, nil
	}
	return nil, nil, fmt.Errorf("%s: [tier2] draft %q: want slice or plan", c.ID, c.Tier2.Draft)
}

// fixtureIssue reads github/repos/o/r/issues/<rel>.json from the case.
func fixtureIssue(c Case, rel string) (*gh.Issue, error) {
	var is gh.Issue
	if err := readJSON(filepath.Join(c.Dir, "github", "repos", "o", "r", "issues", filepath.FromSlash(rel)+".json"), &is); err != nil {
		return nil, fmt.Errorf("%s: [tier2] issue fixture: %w", c.ID, err)
	}
	return &is, nil
}
