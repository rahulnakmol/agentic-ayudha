package evals

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/brief"
	"github.com/rahulnakmol/agentic-ayudha/internal/models"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

// BriefSpec makes a case a `brief` case (WP7.3): the input is the grounding
// (brief.Grounding as JSON), the skill writes a Markdown draft for the
// register and purpose, and the draft must pass `aayu brief`'s deterministic
// checks (structure, citations, the register's lint, visuals) and render to
// a page whose every image has alt text before the judge sees it.
type BriefSpec struct {
	Register string `toml:"register"`
	Purpose  string `toml:"purpose"`
	Draft    string `toml:"draft"` // the recorded run's draft, checked in tier 1
}

const steWords = "skills/brief/references/ste-words.md"

func loadGrounding(c Case) (*brief.Grounding, error) {
	var g brief.Grounding
	if err := readJSON(filepath.Join(c.Dir, filepath.FromSlash(c.Input)), &g); err != nil {
		return nil, fmt.Errorf("grounding %s: %w", c.Input, err)
	}
	return &g, nil
}

func briefWords(assets fs.FS) (*brief.Words, error) {
	b, err := fs.ReadFile(assets, steWords)
	if err != nil {
		return nil, err
	}
	return brief.ParseWords(b)
}

// checkBrief runs the checks `aayu brief` runs before the adversarial pass,
// renders the page, and checks its alt text. It returns the problems and,
// for eli5, the reading grade.
func checkBrief(ctx context.Context, c Case, g *brief.Grounding, words *brief.Words, draft []byte) ([]string, *brief.Reading) {
	d, err := brief.Parse(draft, c.Dir)
	if err != nil {
		return []string{err.Error()}, nil
	}
	var out []string
	found := brief.Check(d, g, brief.Options{Register: c.Brief.Register, Purpose: c.Brief.Purpose, Words: words})
	figs, more := brief.Prepare(ctx, d, g, brief.Sources{})
	for _, f := range append(found, more...) {
		out = append(out, f.String())
	}
	var r *brief.Reading
	if c.Brief.Register == "eli5" {
		rd := brief.Readability(d.Units)
		r = &rd
	}
	if len(out) > 0 {
		return out, r
	}
	page, err := brief.Render(d, g, figs, brief.Page{Register: c.Brief.Register, Purpose: c.Brief.Purpose, Version: "eval", Generated: time.Unix(0, 0)})
	if err != nil {
		return []string{err.Error()}, r
	}
	for _, p := range brief.AltText(page) {
		out = append(out, "alt text: "+p)
	}
	return out, r
}

// tier1Brief checks the recorded draft.
func tier1Brief(ctx context.Context, c Case, assets fs.FS) []string {
	g, err := loadGrounding(c)
	if err != nil {
		return []string{err.Error()}
	}
	words, err := briefWords(assets)
	if err != nil {
		return []string{err.Error()}
	}
	b, err := os.ReadFile(filepath.Join(c.Dir, filepath.FromSlash(c.Brief.Draft)))
	if err != nil {
		return []string{err.Error()}
	}
	out, _ := checkBrief(ctx, c, g, words, b)
	return out
}

// draftOnly strips a fence a model may wrap the whole draft in.
func draftOnly(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		if i := strings.Index(s, "\n"); i > 0 && strings.HasSuffix(s, "```") {
			s = strings.TrimSpace(s[i+1 : len(s)-3])
		}
	}
	return s + "\n"
}

// skillReferences appends the reference files the case says the host reads.
func (h *Harness) skillReferences(c Case, body string) (string, error) {
	for _, r := range c.Tier2.References {
		b, err := fs.ReadFile(h.Assets, path.Join("skills", c.Skill, "references", r))
		if err != nil {
			return "", fmt.Errorf("%s: reference %s: %w", c.ID, r, err)
		}
		body += "\n\n## references/" + r + "\n\n" + strings.TrimSpace(string(b))
	}
	return body, nil
}

// runBrief is one tier-2 run of a brief case. The skill writes a draft; when
// `aayu brief`'s checks fail, it gets their findings once and revises, as it
// does in a host (the skill's step 3). A draft that still fails scores 1
// without the judge (rule 7); one that passes is judged against the rubric.
func (h *Harness) runBrief(ctx context.Context, c Case, lane verbs.LaneID, body, input, rubric string) (run Run, err error) {
	var calls []CallLog
	defer func() { run.Calls = calls }()
	g, err := loadGrounding(c)
	if err != nil {
		return Run{}, err
	}
	words, err := briefWords(h.Assets)
	if err != nil {
		return Run{}, err
	}
	task := fmt.Sprintf("Write a `%s` brief with purpose `%s` on #%d.", c.Brief.Register, c.Brief.Purpose, g.Subject.Number)
	req := models.Request{
		System:    body + "\n\nYou have the grounding already. Reply with the Markdown draft only, nothing before or after it.",
		Messages:  []models.Message{{Role: "user", Content: untrusted("grounding", input) + "\n\n" + task}},
		MaxTokens: h.Config.MaxTokens.Skill,
	}
	complete := h.complete(lane, "skill", &calls)
	var draft string
	var problems []string
	var reading *brief.Reading
	for attempt := 0; attempt < 2; attempt++ {
		resp, err := complete(ctx, req)
		if err != nil {
			return Run{}, err
		}
		if resp.Truncated {
			return Run{}, &models.ReplyError{Err: fmt.Errorf("the draft stopped at the %d-token cap", req.MaxTokens), Reply: resp.Text, Truncated: true}
		}
		draft = draftOnly(resp.Text)
		if problems, reading = checkBrief(ctx, c, g, words, []byte(draft)); len(problems) == 0 {
			run.Revised = attempt > 0
			break
		}
		req.Messages = append(req.Messages, models.Message{Role: "assistant", Content: draft},
			models.Message{Role: "user", Content: "`aayu brief` reported:\n- " + strings.Join(problems, "\n- ") + "\n\nFix every finding and reply with the whole revised draft only."})
	}
	if len(problems) > 0 {
		return Run{Status: RunFail, Score: 1, Revised: true, Floor: "brief checks: " + clip(strings.Join(problems, "; "), 600)}, nil
	}
	checks := "Deterministic checks passed: structure, citations, visuals"
	switch c.Brief.Register {
	case "eli5":
		checks += fmt.Sprintf(", Flesch-Kincaid grade %.1f (at most %.0f)", reading.Grade, brief.MaxGrade)
	case "ste":
		checks += ", STE lint (sentences of at most 20 words, approved words, one instruction per sentence)"
	case "regulatory":
		checks += ", a citation on every paragraph and list item"
	}
	gj, _ := json.MarshalIndent(g, "", "  ")
	var v struct {
		Score  int    `json:"score"`
		Reason string `json:"reason"`
	}
	jreq := models.Request{
		System: judgeSystem + "\n\n## Rubric\n\n" + strings.TrimSpace(rubric),
		Messages: []models.Message{{Role: "user", Content: "## Task given to the skill\n\n" + task + "\n\n## Grounding (the only sources)\n\n" + untrusted("grounding", string(gj)) +
			"\n\n## Skill output\n\n" + untrusted("skill-output", draft) + "\n\n## Checks\n\n" + checks + "."}},
		MaxTokens: h.Config.MaxTokens.Judge,
		Format:    &models.Format{Name: "rubric_score", Schema: judgeSchema, Strict: true},
	}
	if err := models.Structured(ctx, h.complete(judgeLane, "judge", &calls), jreq, judgeSchema, &v); err != nil {
		return Run{}, err
	}
	revised := run.Revised
	run = Run{Status: RunFail, Score: v.Score, Pass: v.Score >= h.Config.PassScore, Reason: clip(v.Reason, 600), Revised: revised}
	if run.Pass {
		run.Status = RunPass
	}
	return run, nil
}
