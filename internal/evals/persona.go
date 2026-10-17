package evals

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/chart"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/decide"
	"github.com/rahulnakmol/agentic-ayudha/internal/models"
	"github.com/rahulnakmol/agentic-ayudha/internal/seats"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

// PersonaSpec makes a case a persona case (WP5.1b): the skill's Accountable
// seat twin answers Task from the grounding (the case input) at a team stage,
// in its persona's voice. Its Markdown output must pass the evidence rule
// (seats.CheckGrounding: every claim cited, every citation found in the
// grounding) and carry the stage's artefact for Practice (seats.CheckStage)
// before the judge scores clarity, nudging, grounding, practitioner realism,
// and stage fit.
type PersonaSpec struct {
	Seat     string `toml:"seat"`     // the skill's Accountable seat
	Stage    string `toml:"stage"`    // startup, scaleup, or enterprise
	Practice string `toml:"practice"` // the playbook entry whose stage artefact the output carries
	Task     string `toml:"task"`     // what is asked of the twin
	Draft    string `toml:"draft"`    // the recorded output, checked in tier 1
	// Planted is a recorded output with a planted uncited claim; tier 1 fails
	// unless the evidence rule catches it.
	Planted string `toml:"planted"`
	// Kind is the chart note's kind (data or ai) for a chart case: its output
	// must also be a complete chart note of that kind (chart.CheckStructure).
	Kind string `toml:"kind"`
}

// personaOf loads the case's persona and practice from the embedded seats/.
func personaOf(c Case, assets fs.FS) (seats.Persona, seats.Practice, error) {
	ps, err := seats.LoadPersonas(assets)
	if err != nil {
		return seats.Persona{}, seats.Practice{}, err
	}
	p := c.Persona
	cmd, ok := verbs.Find(c.Skill)
	if !ok || string(cmd.Seat) != p.Seat {
		return seats.Persona{}, seats.Practice{}, fmt.Errorf("seat %q is not the Accountable seat of %q", p.Seat, c.Skill)
	}
	if !slices.Contains(config.TeamStages, p.Stage) {
		return seats.Persona{}, seats.Practice{}, fmt.Errorf("stage %q: want one of %s", p.Stage, strings.Join(config.TeamStages, ", "))
	}
	if c.Skill == "chart" && (!slices.Contains(chart.Kinds, p.Kind) || p.Practice != chart.Practice(p.Kind)) {
		return seats.Persona{}, seats.Practice{}, fmt.Errorf("a chart case needs kind data or ai and that kind's practice (%s for %q)", chart.Practice(p.Kind), p.Kind)
	}
	persona := ps[verbs.SeatID(p.Seat)]
	pr, ok := persona.Practice(p.Practice)
	if !ok {
		return seats.Persona{}, seats.Practice{}, fmt.Errorf("practice %q is not in %s", p.Practice, seats.PersonaPath(persona.Seat))
	}
	return persona, pr, nil
}

// checkPersona runs the deterministic checks on one output; a chart case's
// output must also have every part of its kind of note.
func checkPersona(c Case, pr seats.Practice, grounding, out string) []string {
	found := append(seats.CheckGrounding(out, grounding), seats.CheckStage(out, pr, c.Persona.Stage)...)
	if c.Skill == "chart" {
		found = append(found, chart.CheckStructure(out, c.Persona.Kind)...)
	}
	return found
}

// tier1Persona checks the recorded output and that the planted one fails.
func tier1Persona(c Case, assets fs.FS) []string {
	_, pr, err := personaOf(c, assets)
	if err != nil {
		return []string{err.Error()}
	}
	read := func(f string) (string, error) {
		b, err := os.ReadFile(filepath.Join(c.Dir, filepath.FromSlash(f)))
		return string(b), err
	}
	grounding, err := read(c.Input)
	if err != nil {
		return []string{err.Error()}
	}
	draft, err := read(c.Persona.Draft)
	if err != nil {
		return []string{err.Error()}
	}
	out := checkPersona(c, pr, grounding, draft)
	if c.Persona.Planted != "" {
		planted, err := read(c.Persona.Planted)
		if err != nil {
			return append(out, err.Error())
		}
		if len(seats.CheckGrounding(planted, grounding)) == 0 {
			out = append(out, c.Persona.Planted+": the planted uncited claim was not caught by the evidence rule")
		}
	}
	return out
}

// runPersona is one tier-2 run of a persona case: the skill (its body opens
// with the seat's persona block) answers the task from the grounding at the
// case's team stage. When the deterministic checks fail it gets their
// findings once and revises, as a host would; a reply that still fails scores
// 1 without the judge (rule 7).
func (h *Harness) runPersona(ctx context.Context, c Case, lane verbs.LaneID, body, input, rubric string) (run Run, err error) {
	var calls []CallLog
	defer func() { run.Calls = calls }()
	persona, pr, err := personaOf(c, h.Assets)
	if err != nil {
		return Run{}, err
	}
	stage := fmt.Sprintf("Team stage for this run: %s (`[team] stage = %q`).", c.Persona.Stage, c.Persona.Stage)
	req := models.Request{
		System: body + "\n\n" + stage + " You have the grounding already; it is the only source. " +
			"Reply with your Markdown answer only, nothing before or after it.",
		Messages:  []models.Message{{Role: "user", Content: untrusted("grounding", input) + "\n\n" + c.Persona.Task}},
		MaxTokens: h.Config.MaxTokens.Skill,
	}
	answer, problems, first, err := reviseOnce(ctx, h.complete(lane, "skill", &calls), req, func(a string) []string { return checkPersona(c, pr, input, a) })
	if err != nil {
		return Run{}, err
	}
	run.Revised, run.RevisedFor = len(first) > 0, revisedFor(first)
	if len(problems) > 0 {
		return Run{Status: RunFail, Score: 1, Revised: true, RevisedFor: run.RevisedFor, Floor: "persona checks: " + clip(strings.Join(problems, "; "), 600), Answer: answerOf(answer)}, nil
	}
	d := pr.At(c.Persona.Stage)
	checks := fmt.Sprintf("Deterministic checks passed: every claim about code, architecture, risk, or progress cites a source "+
		"the grounding contains; the output carries the %s artefact %q of practice %q and no other stage's artefact", c.Persona.Stage, d.Artefact, pr.Name)
	if c.Persona.Stage == config.StageStartup {
		checks += fmt.Sprintf("; it is at most %d words", seats.StartupMaxWords)
	}
	if c.Skill == "chart" {
		checks += fmt.Sprintf("; it has every part of a %s chart note in order, each naming what the part must name", c.Persona.Kind)
	}
	judged, err := h.judge(ctx, "## Task\n\n"+c.Persona.Task+"\n\n## Team stage\n\n"+c.Persona.Stage+
		"\n\n## Persona the skill speaks in\n\n"+persona.Prompt(c.Persona.Stage, "set by the case")+
		"\n\n## Grounding (the only source)\n\n"+input+"\n\n## Checks\n\n"+checks+".", rubric, answer, nil, &calls)
	if err != nil {
		return Run{}, err
	}
	judged.Revised, judged.RevisedFor = run.Revised, run.RevisedFor
	judged.Answer = answerOf(answer)
	return judged, nil
}

// reviseOnce asks the skill for its Markdown answer and, when check finds
// problems, gives it the findings once to revise, as a host would. It returns
// the last answer, its remaining problems, and the first draft's findings
// (none when the first draft passed).
func reviseOnce(ctx context.Context, complete func(context.Context, models.Request) (models.Response, error), req models.Request,
	check func(string) []string) (answer string, problems, first []string, err error) {
	for attempt := 0; attempt < 2; attempt++ {
		resp, err := complete(ctx, req)
		if err != nil {
			return "", nil, nil, err
		}
		if resp.Truncated {
			return "", nil, nil, &models.ReplyError{Err: fmt.Errorf("the answer stopped at the %d-token cap", req.MaxTokens), Reply: resp.Text, Truncated: true}
		}
		answer = draftOnly(resp.Text)
		if problems = check(answer); len(problems) == 0 {
			return answer, nil, first, nil
		}
		if attempt == 0 {
			first = problems
		}
		req.Messages = append(req.Messages, models.Message{Role: "assistant", Content: answer},
			models.Message{Role: "user", Content: "The deterministic checks reported:\n- " + strings.Join(problems, "\n- ") +
				"\n\nFix every finding and reply with the whole revised answer only."})
	}
	return answer, problems, first, nil
}

// PackSpec makes a case a decision-pack case (WP5.1b, D-5.6b): the human asks
// Question while stuck; the topic routes it to Seat, whose twin writes the
// pack at Stage from the grounding (the case input: what `aayu decide --pack`
// printed plus what the twin read). The pack must have every part and cite
// every claim (decide.CheckPack) before the judge scores it.
type PackSpec struct {
	Seat     string `toml:"seat"`
	Stage    string `toml:"stage"`
	Question string `toml:"question"`
	Draft    string `toml:"draft"` // the recorded pack, checked in tier 1
	// Incomplete is a recorded pack missing a required part; tier 1 fails
	// unless decide.CheckPack catches it.
	Incomplete string `toml:"incomplete"`
}

// packPersona checks the routing and stage and returns the owning twin.
func packPersona(c Case, assets fs.FS) (seats.Persona, error) {
	p := c.Pack
	if got, why := decide.OwningSeat(p.Question, ""); string(got) != p.Seat {
		return seats.Persona{}, fmt.Errorf("the question routes to %s (%s), not %s", got, why, p.Seat)
	}
	if !slices.Contains(config.TeamStages, p.Stage) {
		return seats.Persona{}, fmt.Errorf("stage %q: want one of %s", p.Stage, strings.Join(config.TeamStages, ", "))
	}
	ps, err := seats.LoadPersonas(assets)
	if err != nil {
		return seats.Persona{}, err
	}
	return ps[verbs.SeatID(p.Seat)], nil
}

// tier1Pack checks the routing, the recorded pack, and that the incomplete one fails.
func tier1Pack(c Case, assets fs.FS) []string {
	if _, err := packPersona(c, assets); err != nil {
		return []string{err.Error()}
	}
	read := func(f string) (string, error) {
		b, err := os.ReadFile(filepath.Join(c.Dir, filepath.FromSlash(f)))
		return string(b), err
	}
	grounding, err := read(c.Input)
	if err != nil {
		return []string{err.Error()}
	}
	draft, err := read(c.Pack.Draft)
	if err != nil {
		return []string{err.Error()}
	}
	out := decide.CheckPack(draft, grounding)
	if c.Pack.Incomplete != "" {
		bad, err := read(c.Pack.Incomplete)
		if err != nil {
			return append(out, err.Error())
		}
		if len(decide.CheckPack(bad, grounding)) == 0 {
			out = append(out, c.Pack.Incomplete+": the incomplete pack was not caught")
		}
	}
	return out
}

// runPack is one tier-2 run of a decision-pack case: the decide skill, in the
// owning twin's persona at the case's stage, writes the pack from the
// grounding; it revises once on CheckPack's findings, else scores 1 without
// the judge (rule 7).
func (h *Harness) runPack(ctx context.Context, c Case, lane verbs.LaneID, body, input, rubric string) (run Run, err error) {
	var calls []CallLog
	defer func() { run.Calls = calls }()
	persona, err := packPersona(c, h.Assets)
	if err != nil {
		return Run{}, err
	}
	req := models.Request{
		System: body + "\n\n## Persona of the owning twin (from `aayu decide --pack`)\n\n" + persona.Prompt(c.Pack.Stage, "set by the case") +
			"\nYou have already run `aayu decide --pack`; the grounding is its output plus what you read, and it is the only source. " +
			fmt.Sprintf("Reply with the decision pack in Markdown only, at most %d words, nothing before or after it.", decide.PackMaxWords),
		Messages:  []models.Message{{Role: "user", Content: untrusted("grounding", input) + "\n\nThe human is stuck and asks: " + c.Pack.Question}},
		MaxTokens: h.Config.MaxTokens.Skill,
	}
	pack, problems, first, err := reviseOnce(ctx, h.complete(lane, "skill", &calls), req, func(a string) []string { return decide.CheckPack(a, input) })
	if err != nil {
		return Run{}, err
	}
	if len(problems) > 0 {
		return Run{Status: RunFail, Score: 1, Revised: true, RevisedFor: revisedFor(first), Floor: "pack checks: " + clip(strings.Join(problems, "; "), 600), Answer: answerOf(pack)}, nil
	}
	checks := "Deterministic checks passed: every part is present in order, the question is one sentence, there are two or three " +
		"options, the recommendation names its door, the pack ends with the one decision, and every claim cites a source the grounding contains."
	judged, err := h.judge(ctx, "## The human's question\n\n"+c.Pack.Question+"\n\n## Owning twin and team stage\n\n"+persona.Name+", the "+
		verbs.SeatName(persona.Seat)+" twin, at the "+c.Pack.Stage+" stage\n\n## Grounding (the only source)\n\n"+input+"\n\n## Checks\n\n"+checks,
		rubric, pack, nil, &calls)
	if err != nil {
		return Run{}, err
	}
	judged.Revised, judged.RevisedFor = len(first) > 0, revisedFor(first)
	judged.Answer = answerOf(pack)
	return judged, nil
}

// answerOf is the answer as a run records it: redacted and clipped.
func answerOf(s string) string { return clip(actions.RedactString(s), 8000) }

// revisedFor is the first draft's findings as a run records them.
func revisedFor(first []string) string {
	return clip(actions.RedactString(strings.Join(first, "; ")), 600)
}
