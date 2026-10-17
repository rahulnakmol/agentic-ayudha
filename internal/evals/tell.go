package evals

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/design"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/models"
	"github.com/rahulnakmol/agentic-ayudha/internal/tell"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

// TellSpec makes a case a `tell` case (D-7.5b): the input is the grounding
// (the proven and unproven Features as `aayu tell` lists them, and what the
// host read about each: packs, criteria, evidence, Results), the case
// directory's design/brand.yaml is the brand, and the skill writes Markdown
// copy of Kind for the human's Request. The copy must pass `aayu tell --from`'s
// checks (tell.CheckDraft) and the case's scope rules before the judge sees it.
type TellSpec struct {
	Kind string `toml:"kind"`
	// Campaign is the UTM campaign (`--campaign`, or the release tag); default the kind.
	Campaign string `toml:"campaign"`
	Request  string `toml:"request"` // what the human asked for
	Draft    string `toml:"draft"`   // the recorded run's copy, checked in tier 1
}

// tellGrounding is the part of a tell case's input the checks read.
type tellGrounding struct {
	Repo     string         `json:"repo"`
	Proven   []tell.Feature `json:"proven_features"`
	Unproven []tell.Feature `json:"unproven_features"`
}

// tellSetup reads a tell case's grounding and brand, and the UTM tags its
// links get.
func tellSetup(c Case) (tellGrounding, tell.Brand, tell.UTM, error) {
	var g tellGrounding
	if err := readJSON(filepath.Join(c.Dir, filepath.FromSlash(c.Input)), &g); err != nil {
		return g, tell.Brand{}, tell.UTM{}, fmt.Errorf("grounding %s: %w", c.Input, err)
	}
	b, err := tell.LoadBrand(c.Dir)
	if err != nil {
		return g, b, tell.UTM{}, err
	}
	if b.Source != design.BrandPath {
		return g, b, tell.UTM{}, fmt.Errorf("%s: a tell case needs its own %s", c.ID, design.BrandPath)
	}
	_, name, _ := github.SplitRepo(g.Repo)
	campaign := c.Tell.Campaign
	if campaign == "" {
		campaign = c.Tell.Kind
	}
	return g, b, tell.UTM{Source: name, Medium: c.Tell.Kind, Campaign: campaign}, nil
}

// checkTell runs `aayu tell --from`'s checks and the case's scope rules.
func checkTell(c Case, g tellGrounding, b tell.Brand, u tell.UTM, draft string) []string {
	proven := map[int]tell.Feature{}
	for _, f := range g.Proven {
		proven[f.Number] = f
	}
	_, probs := tell.CheckDraft(draft, g.Repo, c.Tell.Kind, proven, g.Unproven, b, u)
	var out []string
	for _, p := range probs {
		out = append(out, p.String())
	}
	for _, r := range c.Scope {
		if m, clause := r.find(draft); m != "" {
			out = append(out, fmt.Sprintf("scope: adds %s (%q in %q)", r.Why, m, clause))
		}
	}
	return out
}

// tier1Tell checks the recorded copy.
func tier1Tell(c Case) []string {
	g, b, u, err := tellSetup(c)
	if err != nil {
		return []string{err.Error()}
	}
	d, err := os.ReadFile(filepath.Join(c.Dir, filepath.FromSlash(c.Tell.Draft)))
	if err != nil {
		return []string{err.Error()}
	}
	return checkTell(c, g, b, u, string(d))
}

// tellBrief is what `aayu tell <kind>` prints, which the host reads first.
func tellBrief(c Case, g tellGrounding, b tell.Brand, u tell.UTM) string {
	var s strings.Builder
	fmt.Fprintf(&s, "tell %s brief\n\nVoice (%s): %s\nTone: %s\nNever use: %s\n\n", c.Tell.Kind, b.Source, b.Voice, strings.Join(b.Tone, ", "), strings.Join(b.Avoid, ", "))
	s.WriteString("Features you may claim (every shipped story has an authoritative pack); link them as #N:\n")
	for _, f := range g.Proven {
		fmt.Fprintf(&s, "  #%d %s\n", f.Number, f.Title)
	}
	fmt.Fprintf(&s, "Not yet provable (do not claim or link): %d Feature(s)\n\n", len(g.Unproven))
	fmt.Fprintf(&s, "Links are tagged utm_source=%s&utm_medium=%s&utm_campaign=%s.", u.Source, u.Medium, u.Campaign)
	return s.String()
}

// runTell is one tier-2 run of a tell case. The skill writes the copy; when
// `aayu tell --from`'s checks or the case's scope rules fail, it gets their
// findings once and revises, as it does in a host (the skill's check step).
// Copy that still fails scores 1 without the judge (rule 7); copy that passes
// is judged against the rubric.
func (h *Harness) runTell(ctx context.Context, c Case, lane verbs.LaneID, body, input, rubric string) (run Run, err error) {
	var calls []CallLog
	defer func() { run.Calls = calls }()
	g, b, u, err := tellSetup(c)
	if err != nil {
		return Run{}, err
	}
	brand, err := os.ReadFile(filepath.Join(c.Dir, filepath.FromSlash(design.BrandPath)))
	if err != nil {
		return Run{}, err
	}
	sources := untrusted("aayu tell "+c.Tell.Kind, tellBrief(c, g, b, u)) + "\n\n" + untrusted(design.BrandPath, string(brand)) + "\n\n" + untrusted("grounding", input)
	task := "The human asks: " + c.Tell.Request
	req := models.Request{
		System:    body + "\n\nYou have run `aayu tell " + c.Tell.Kind + "` and read the brand and every Feature's packs already; they follow. Reply with the Markdown draft only, nothing before or after it.",
		Messages:  []models.Message{{Role: "user", Content: sources + "\n\n" + task}},
		MaxTokens: h.Config.MaxTokens.Skill,
	}
	complete := h.complete(lane, "skill", &calls)
	var draft string
	var problems []string
	for attempt := 0; attempt < 2; attempt++ {
		resp, err := complete(ctx, req)
		if err != nil {
			return Run{}, err
		}
		if resp.Truncated {
			return Run{}, &models.ReplyError{Err: fmt.Errorf("the draft stopped at the %d-token cap", req.MaxTokens), Reply: resp.Text, Truncated: true}
		}
		draft = draftOnly(resp.Text)
		if problems = checkTell(c, g, b, u, draft); len(problems) == 0 {
			run.Revised = attempt > 0
			break
		}
		req.Messages = append(req.Messages, models.Message{Role: "assistant", Content: draft},
			models.Message{Role: "user", Content: "`aayu tell " + c.Tell.Kind + " --from` refused the draft:\n- " + strings.Join(problems, "\n- ") + "\n\nFix every finding and reply with the whole revised draft only."})
	}
	if len(problems) > 0 {
		return Run{Status: RunFail, Score: 1, Revised: true, Floor: "tell checks: " + clip(strings.Join(problems, "; "), 600)}, nil
	}
	checks := "Deterministic checks passed (`aayu tell --from`): every link to a work item is a proven Feature; no sentence names an unproven Feature; " +
		"every proven Feature the copy names is linked; no word on the brand's avoid list; no superlative"
	var v struct {
		Score  int    `json:"score"`
		Reason string `json:"reason"`
	}
	jreq := models.Request{
		System: judgeSystem + "\n\n## Rubric\n\n" + strings.TrimSpace(rubric),
		Messages: []models.Message{{Role: "user", Content: "## Task given to the skill\n\nWrite `" + c.Tell.Kind + "` copy. " + task +
			"\n\n## Sources (the only ones)\n\n" + sources + "\n\n## Skill output\n\n" + untrusted("skill-output", draft) + "\n\n## Checks\n\n" + checks + "."}},
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
