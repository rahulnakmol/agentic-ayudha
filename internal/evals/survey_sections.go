package evals

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/models"
	"github.com/rahulnakmol/agentic-ayudha/internal/survey"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

// surveySectionSettings are each section's first output cap and reasoning
// effort (survey-sections-2026-10.md): about five times the largest section
// JSON of the pilots plus a reasoning allowance the effort bounds. In run
// 39101367105 (tiny-js) engineering, modernisation, and questions each stopped
// at that first cap and finished at twice it (engineering used 30648 of
// 32000), so they start there: a cap is not a charge, a draft cut off is.
var surveySectionSettings = map[string]struct {
	cap    int
	effort string
}{
	"engineering":   {32000, "high"},
	"capabilities":  {8000, "medium"},
	"product":       {4000, "medium"},
	"modernisation": {32000, "high"},
	"questions":     {16000, "medium"},
}

// maxSectionCap is the highest cap a truncated section is rerun at.
const maxSectionCap = 64000

// SectionRun is one section of a survey run written in sections: its score,
// what the floors and the self-check did, and what it cost.
type SectionRun struct {
	Name  string `json:"name"`
	Score int    `json:"score"` // the corrected section's; 0 when the floors failed or the run errored first
	// FirstScore is the score of the section that passed the floors before
	// the self-check, recorded only.
	FirstScore  int    `json:"first_score,omitempty"`
	Reason      string `json:"reason,omitempty"`
	FirstReason string `json:"first_reason,omitempty"`
	Floor       string `json:"floor,omitempty"`
	// RevisedFor is what the floors found in the section's first reply.
	RevisedFor string `json:"revised_for,omitempty"`
	// Changed is what the self-check's corrections changed; SelfRevised is
	// set when the corrected section failed the floors once.
	Changed     []string `json:"changed,omitempty"`
	SelfRevised bool     `json:"self_revised,omitempty"`
	MaxTokens   int      `json:"max_tokens"`            // the cap of the attempt that finished
	Truncations int      `json:"truncations,omitempty"` // replies that stopped at their cap, each rerun at twice it
	TokensIn    int      `json:"tokens_in"`
	CachedIn    int      `json:"cached_in,omitempty"`
	CacheWrite  int      `json:"cache_write_in,omitempty"`
	TokensOut   int      `json:"tokens_out"`
	CostUSD     float64  `json:"cost_usd"`
	// Resumed marks a section loaded from an earlier run's saved step: its
	// tokens and cost were charged there.
	Resumed bool `json:"resumed,omitempty"`
}

// sectionOutput is what a section's step saves.
type sectionOutput struct {
	Content     string   `json:"content"` // the scored section, after the self-check
	First       string   `json:"first,omitempty"`
	Corrections string   `json:"corrections,omitempty"`
	FirstScore  int      `json:"first_score,omitempty"`
	FirstReason string   `json:"first_reason,omitempty"`
	Score       int      `json:"score,omitempty"`
	Reason      string   `json:"reason,omitempty"`
	RevisedFor  string   `json:"revised_for,omitempty"`
	Changed     []string `json:"changed,omitempty"`
	SelfRevised bool     `json:"self_revised,omitempty"`
	MaxTokens   int      `json:"max_tokens"`
	Truncations int      `json:"truncations,omitempty"`
}

// runSurveySections is a tier-2 run of a survey draft written in sections, as
// the skill writes it: each section is its own call with its own cap and
// effort, checked against the index and the sections before it, revised once
// on the floors' findings, judged for the record, self-checked as
// corrections, and judged again against its section of the rubric, then
// saved as a step before the next starts. A section that stops at its cap is
// rerun alone at twice it. The run's score is
// max(1, 5 - sum of (5 - section score)): one weak section scores 4, two 3.
func (h *Harness) runSurveySections(ctx context.Context, c Case, lane verbs.LaneID, body, input string) (run Run, err error) {
	w, err := surveySetup(ctx, c, h.Assets)
	if err != nil {
		return Run{}, err
	}
	const task = "Write survey.json for this request one section at a time, as the skill says: each reply is one section. No person is available in this session: leave every question open with your default, and never answer one yourself."
	shared := models.Message{Role: "user", CacheBreak: true, Content: untrusted("aayu survey and aayu survey --module", w.printed) + "\n\n" + w.read +
		"The person asks:\n\n" + untrusted("request", input) + "\n\n" + task}
	system := body + "\n\nYou have indexed the repository and read every module's pack and the files that follow. Reply with the section asked for only: one JSON object, nothing before or after it."
	grounding := "## The person's request\n\n" + input + "\n\n## What the host saw (`aayu survey`, every module pack)\n\n" + w.printed + "\n\n## Files the host read\n\n" + w.read
	parts := map[string]json.RawMessage{}
	var earlier []CallLog // every section's calls so far, resumed ones included
	score := 5
	var reasons []string
	for _, sec := range survey.Sections {
		name := sec.Name
		rubric, err := os.ReadFile(filepath.Join(c.Dir, filepath.FromSlash(c.Survey.Rubrics), name+".md"))
		if err != nil {
			return run, err
		}
		prior := priorSections(parts)
		keys := strings.Join(sec.Keys, ", ")
		ask := prior + "Write the " + name + " section of survey.json now: one JSON object with only its keys (" + keys + "), nothing before or after it."
		if name == "questions" {
			ask += " Write the questions, the summary of the whole survey, and assign, which maps every item waiting with \"?\" to its question."
		} else {
			ask += " An item whose question is not written yet carries \"?\"; the questions section comes last and assigns it."
		}
		set := surveySectionSettings[name]
		req := models.Request{System: system, Messages: []models.Message{shared, {Role: "user", Content: ask}}, MaxTokens: set.cap, Effort: set.effort, Stream: true}
		judgeInput := grounding + priorJudged(parts) + "## Task\n\n" + task + " This is the " + name + " section (" + keys + ").\n\n## Checks\n\n" +
			"Deterministic checks passed (`aayu survey --draft` on the sections so far): every citation resolves to an index fact or a line range inside the file; every claim has a basis its evidence allows; no product claim is observed; every unknown has a question; no question was answered without a person."
		st, err := h.step(ctx, c, "draft/"+name, stepKey(req, string(rubric)), func(prev *Step, calls *[]CallLog) (Step, error) {
			// The section logs after the earlier sections' calls, so their
			// provider retries count toward the sample's ProviderRetries.
			log := slices.Clone(earlier)
			st, err := h.surveySection(ctx, c, w, lane, name, req, prev, parts, judgeInput, string(rubric), &log)
			*calls = append(*calls, log[len(earlier):]...)
			return st, err
		})
		earlier = append(earlier, st.Calls...)
		if !st.Resumed {
			run.Calls = append(run.Calls, st.Calls...)
		}
		var o sectionOutput
		_ = json.Unmarshal([]byte(st.Output), &o)
		sr := SectionRun{Name: name, Score: o.Score, FirstScore: o.FirstScore, Reason: o.Reason, FirstReason: o.FirstReason, RevisedFor: o.RevisedFor,
			Changed: o.Changed, SelfRevised: o.SelfRevised, MaxTokens: o.MaxTokens, Truncations: o.Truncations, CostUSD: st.CostUSD, Resumed: st.Resumed}
		for _, l := range st.Calls {
			sr.TokensIn, sr.CachedIn, sr.CacheWrite, sr.TokensOut = sr.TokensIn+l.TokensIn, sr.CachedIn+l.CachedIn, sr.CacheWrite+l.CacheWriteIn, sr.TokensOut+l.TokensOut
		}
		switch {
		case err != nil:
			run.Sections = append(run.Sections, sr)
			return run, fmt.Errorf("section %s: %w", name, err)
		case st.Status == StepTruncated:
			run.Sections = append(run.Sections, sr)
			return run, &models.ReplyError{Err: fmt.Errorf("section %s stopped at the %d-token cap", name, maxSectionCap), Reply: o.Content, Truncated: true}
		case st.Status == StepFailed:
			sr.Score, sr.Floor = 0, clip(strings.Join(st.Findings, "; "), 600)
			run.Sections = append(run.Sections, sr)
			run.Status, run.Score, run.Floor = RunFail, 1, "survey section "+name+" checks: "+sr.Floor
			run.Answer = answerOf(o.Content)
			return run, nil
		}
		run.Sections = append(run.Sections, sr)
		run.Revised = run.Revised || o.RevisedFor != ""
		parts[name] = json.RawMessage(o.Content)
		score -= 5 - o.Score
		reasons = append(reasons, fmt.Sprintf("%s %d: %s", name, o.Score, o.Reason))
	}
	d, assign, err := survey.MergeSections(parts)
	if err != nil {
		return run, err
	}
	b, err := json.Marshal(d)
	if err != nil {
		return run, err
	}
	run.Answer = answerOf(string(b))
	if problems := append(assign, checkSurvey(c, w, string(b))...); len(problems) > 0 {
		run.Status, run.Score, run.Floor = RunFail, 1, "survey checks on the merged draft: "+clip(strings.Join(problems, "; "), 600)
		return run, nil
	}
	run.Score, run.Reason = max(1, score), clip(strings.Join(reasons, " | "), 1200)
	run.Status, run.Pass = RunFail, run.Score >= h.Config.PassScore
	if run.Pass {
		run.Status = RunPass
	}
	return run, nil
}

// surveySection writes, checks, judges, and self-checks one section, rerunning
// it at twice its cap while a reply stops at the cap, up to maxSectionCap.
// When its last saved attempt stopped at a cap, it starts at twice that cap.
func (h *Harness) surveySection(ctx context.Context, c Case, w surveyWorld, lane verbs.LaneID, name string, req models.Request, prev *Step,
	parts map[string]json.RawMessage, judgeInput, rubric string, calls *[]CallLog) (Step, error) {
	if prev != nil && prev.Status == StepTruncated {
		for _, l := range prev.Calls {
			if l.Truncated {
				req.MaxTokens = max(req.MaxTokens, min(2*l.MaxTokens, maxSectionCap))
			}
		}
	}
	complete := h.complete(lane, "skill", calls)
	check := func(text string) []string { return checkSection(c, w, name, parts, text) }
	o := sectionOutput{}
	for {
		o.MaxTokens = req.MaxTokens
		reply, problems, first, err := reviseOnceAsking(ctx, complete, req, check, "Fix every finding and reply with the whole "+name+" section again, with only its keys.")
		var re *models.ReplyError
		if errors.As(err, &re) && re.Truncated {
			o.Truncations++
			if req.MaxTokens >= maxSectionCap {
				o.Content = actions.RedactString(clip(re.Reply, 2000))
				return sectionStep(StepTruncated, o, nil), nil
			}
			req.MaxTokens = min(2*req.MaxTokens, maxSectionCap)
			continue
		}
		o.RevisedFor = revisedFor(first)
		if err != nil {
			return sectionStep("", o, nil), err
		}
		if len(problems) > 0 {
			o.Content = reply
			return sectionStep(StepFailed, o, problems), nil
		}
		o.First = reply
		judged, err := h.judge(ctx, judgeInput, rubric, prettyJSON(reply), nil, calls)
		if err != nil {
			return sectionStep("", o, nil), err
		}
		o.FirstScore, o.FirstReason = judged.Score, judged.Reason
		sreq := req
		sreq.Messages = append(slices.Clone(req.Messages), models.Message{Role: "assistant", Content: reply, CacheBreak: true},
			models.Message{Role: "user", Content: selfCheckPrompt("draft", name+" section")})
		var corrected string
		patch, problems, again, err := reviseOnceAsking(ctx, complete, sreq, func(p string) []string {
			var refused []string
			corrected, o.Changed, refused = applyCorrections(reply, p)
			if len(refused) > 0 {
				for i := range refused {
					refused[i] = "corrections not applied (none is, until all apply): " + refused[i]
				}
				return refused
			}
			return check(corrected)
		}, "Fix every finding and reply with the whole corrections object again, for the "+name+" section you self-checked, never the section itself.")
		if errors.As(err, &re) && re.Truncated {
			o.Truncations++
			if req.MaxTokens >= maxSectionCap {
				return sectionStep(StepTruncated, o, nil), nil
			}
			req.MaxTokens = min(2*req.MaxTokens, maxSectionCap)
			continue
		}
		o.Corrections, o.SelfRevised = actions.RedactString(patch), len(again) > 0
		if err != nil {
			return sectionStep("", o, nil), err
		}
		if len(problems) > 0 {
			o.Content = corrected
			if o.Content == "" {
				o.Content = reply
			}
			return sectionStep(StepFailed, o, append([]string{"after the self-check:"}, problems...)), nil
		}
		o.Content = corrected
		judged, err = h.judge(ctx, judgeInput, rubric, prettyJSON(corrected), nil, calls)
		if err != nil {
			return sectionStep("", o, nil), err
		}
		o.Score, o.Reason = judged.Score, judged.Reason
		return sectionStep("", o, nil), nil
	}
}

func sectionStep(status string, o sectionOutput, findings []string) Step {
	b, _ := json.Marshal(o)
	return Step{Status: status, Output: string(b), Findings: findings}
}

// checkSection runs the floors on one section's reply: it holds that section's
// keys only, and the sections so far, with it, pass `aayu survey --draft`
// (survey.CheckSections), no question is answered or claim confirmed without
// a person, and the case's scope rules hold.
func checkSection(c Case, w surveyWorld, name string, parts map[string]json.RawMessage, reply string) []string {
	got, err := survey.SplitDraft([]byte(reply))
	if err != nil {
		return []string{"the " + name + " section does not parse (unknown keys are refused): " + err.Error()}
	}
	var others []string
	for s := range got {
		if s != name {
			others = append(others, s)
		}
	}
	if len(others) > 0 {
		slices.Sort(others)
		return []string{fmt.Sprintf("the reply holds keys of the %s section(s): reply with the %s section's keys only", strings.Join(others, ", "), name)}
	}
	if _, ok := got[name]; !ok {
		return []string{"the reply holds none of the " + name + " section's keys"}
	}
	all := map[string]json.RawMessage{name: got[name]}
	for k, v := range parts {
		all[k] = v
	}
	d, rep, err := survey.CheckSections(w.ix, all)
	if err != nil {
		return []string{err.Error()}
	}
	return append(append(rep.Problems, noPersonProblems(d)...), scopeProblems(c, reply)...)
}

// priorSections shows the sections written so far, in order, to the skill.
func priorSections(parts map[string]json.RawMessage) string {
	if len(parts) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("The sections written so far, checked and saved:\n\n")
	for _, n := range survey.SectionNames() {
		if p, ok := parts[n]; ok {
			b.WriteString("```json\n" + string(p) + "\n```\n\n")
		}
	}
	return b.String()
}

// priorJudged shows the judge the sections a section rests on.
func priorJudged(parts map[string]json.RawMessage) string {
	if len(parts) == 0 {
		return ""
	}
	return "## The sections it rests on (checked and judged)\n\n" + strings.TrimPrefix(priorSections(parts), "The sections written so far, checked and saved:\n\n")
}
