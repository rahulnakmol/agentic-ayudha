package evals

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/cli"
	"github.com/rahulnakmol/agentic-ayudha/internal/models"
	"github.com/rahulnakmol/agentic-ayudha/internal/survey"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

// SurveySpec makes a case a survey case (WP5.9). The case's repo fixture is a
// real codebase with no product docs; the input is the person's request.
// Kind "draft": the skill gets what a host has after `aayu survey` and
// `--module` for every module plus the files in Read, and writes survey.json;
// no person is in the session, so every question stays open. Kind "backlog":
// the skill also gets Survey, a checked survey draft with a recorded
// interview, and writes backlog.json. The reply must pass `aayu survey
// --draft` or `--backlog` (survey.Check, survey.CheckBacklog) and the case's
// scope rules before the judge sees it.
type SurveySpec struct {
	Kind   string   `toml:"kind"`   // draft or backlog
	Read   []string `toml:"read"`   // files the host read, as paths under the repo, or path:start-end
	Survey string   `toml:"survey"` // backlog: the checked survey draft the backlog comes from
	Draft  string   `toml:"draft"`  // the recorded run's reply (survey.json or backlog.json), checked in tier 1
	// Probes are probes.json files recorded by `aayu survey --golden … --run
	// --probe` on the fixture's legacy build: the runtime evidence the host
	// saw, printed as the CLI prints it, and citable as probe: facts.
	Probes []string `toml:"probes"`
}

func (s *SurveySpec) check() error {
	switch {
	case s.Kind != "draft" && s.Kind != "backlog":
		return fmt.Errorf("[survey] kind %q: want draft or backlog", s.Kind)
	case s.Draft == "":
		return errors.New("[survey] needs draft, the recorded reply")
	case s.Kind == "backlog" && s.Survey == "":
		return errors.New("[survey] kind backlog needs survey, the checked survey draft")
	case s.Kind == "draft" && len(s.Read) == 0:
		return errors.New("[survey] kind draft needs read, the files the host read")
	}
	return nil
}

// surveyWorld is what a host has in front of it after indexing: the index,
// the text `aayu survey` and `aayu survey --module` print, and the files read.
type surveyWorld struct {
	ix      *survey.Index
	printed string // the index summary and every module pack, as the CLI prints them
	read    string // the files read, with line numbers
	survey  survey.Draft
	surveyJ string
}

// surveySetup indexes the case's repo through the CLI, as a host would, and
// reads the files the case says the host read.
func surveySetup(ctx context.Context, c Case, assets fs.FS) (surveyWorld, error) {
	var w surveyWorld
	if c.Repo == "" {
		return w, errors.New("a survey case needs repo, the codebase fixture")
	}
	repo, err := filepath.Abs(filepath.Join(c.Dir, filepath.FromSlash(c.Repo)))
	if err != nil {
		return w, err
	}
	out, err := os.MkdirTemp("", "aayu-survey-eval-")
	if err != nil {
		return w, err
	}
	defer func() { _ = os.RemoveAll(out) }()
	home, err := os.MkdirTemp("", "aayu-survey-home-")
	if err != nil {
		return w, err
	}
	defer func() { _ = os.RemoveAll(home) }()
	aayu := func(args ...string) (string, error) {
		var o, e bytes.Buffer
		code := cli.Run(ctx, append(args, "--out", out, "--plain"), cli.IO{In: strings.NewReader(""), Out: &o, Err: &e}, cli.Options{
			Dir: repo, Assets: assets, Build: cli.BuildInfo{Version: "0.0.0-eval"}, GitHubGap: -1,
			Getenv:  func(k string) string { return map[string]string{"HOME": home}[k] },
			GHToken: func(context.Context) (string, error) { return "", errors.New("gh is not used in evals") },
		})
		if code != 0 {
			return "", fmt.Errorf("`aayu %s` exited %d: %s", strings.Join(args, " "), code, strings.TrimSpace(e.String()))
		}
		return o.String(), nil
	}
	var p strings.Builder
	sum, err := aayu("survey", repo)
	if err != nil {
		return w, err
	}
	// The temporary paths mean nothing to the model; name them as a host would see them.
	sum = strings.ReplaceAll(strings.ReplaceAll(sum, filepath.Join(out, "index.json"), ".aayudha/survey/index.json"), repo, ".")
	p.WriteString("$ aayu survey\n" + sum)
	if w.ix, err = survey.Load(filepath.Join(out, "index.json")); err != nil {
		return w, err
	}
	for _, m := range w.ix.Modules {
		pack, err := aayu("survey", "--module", m.ID)
		if err != nil {
			return w, err
		}
		p.WriteString("\n$ aayu survey --module " + m.ID + "\n" + pack)
	}
	for i, f := range c.Survey.Probes {
		b, err := os.ReadFile(filepath.Join(c.Dir, filepath.FromSlash(f)))
		if err != nil {
			return w, err
		}
		var pr survey.Probes
		if err := json.Unmarshal(b, &pr); err != nil {
			return w, fmt.Errorf("%s: %w", f, err)
		}
		dir := filepath.Join(out, "golden", strconv.Itoa(i))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return w, err
		}
		if err := os.WriteFile(filepath.Join(dir, "probes.json"), b, 0o644); err != nil {
			return w, err
		}
		fmt.Fprintf(&p, "\n$ aayu survey --golden %s --bin %s --run --probe\n", pr.Cases, filepath.Base(pr.Bin))
		for _, r := range pr.Ran {
			p.WriteString("  probe ran: " + r + "\n")
		}
		for _, r := range pr.Skipped {
			p.WriteString("  probe skipped: " + r + "\n")
		}
		fmt.Fprintf(&p, "  %d probe fact(s):\n", len(pr.Facts))
		for _, ft := range pr.Facts {
			p.WriteString("  " + ft.ID + ": " + ft.Detail + "\n")
		}
	}
	if err := w.ix.LoadProbes(out); err != nil {
		return w, err
	}
	w.printed = p.String()
	var r strings.Builder
	for _, f := range c.Survey.Read {
		text, err := readNumbered(repo, f)
		if err != nil {
			return w, fmt.Errorf("read %s: %w", f, err)
		}
		r.WriteString(untrusted(f, text) + "\n\n")
	}
	w.read = r.String()
	if c.Survey.Kind == "backlog" {
		b, err := os.ReadFile(filepath.Join(c.Dir, filepath.FromSlash(c.Survey.Survey)))
		if err != nil {
			return w, err
		}
		if w.survey, err = survey.ParseDraft(b); err != nil {
			return w, fmt.Errorf("%s: %w", c.Survey.Survey, err)
		}
		if rep := survey.Check(w.ix, w.survey); !rep.Pass {
			return w, fmt.Errorf("%s does not pass `aayu survey --draft`: %s", c.Survey.Survey, strings.Join(rep.Problems, "; "))
		}
		w.surveyJ = string(b)
	}
	return w, nil
}

// readNumbered reads a file under repo, or a path:start-end range of it, with
// each line numbered as an editor shows it, so the model can cite path:line.
func readNumbered(repo, spec string) (string, error) {
	path, start, end := spec, 1, 0
	if i := strings.LastIndex(spec, ":"); i > 0 {
		a, b, ok := strings.Cut(spec[i+1:], "-")
		s, err1 := strconv.Atoi(a)
		e, err2 := strconv.Atoi(b)
		if ok && err1 == nil && err2 == nil && s >= 1 && e >= s {
			path, start, end = spec[:i], s, e
		}
	}
	b, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(path)))
	if err != nil {
		return "", err
	}
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	if end == 0 || end > len(lines) {
		end = len(lines)
	}
	var s strings.Builder
	fmt.Fprintf(&s, "%s, lines %d-%d of %d\n", path, start, end, len(lines))
	for n := start; n <= end; n++ {
		fmt.Fprintf(&s, "%d: %s\n", n, lines[n-1])
	}
	return s.String(), nil
}

// checkSurvey runs `aayu survey --draft` or `--backlog`'s checks on a reply,
// the case's scope rules, and, for a draft written with no person in the
// session, the rule that no question is answered and no claim confirmed.
func checkSurvey(c Case, w surveyWorld, reply string) []string {
	var out []string
	switch c.Survey.Kind {
	case "draft":
		d, err := survey.ParseDraft([]byte(reply))
		if err != nil {
			return []string{"survey.json does not parse (unknown keys are refused): " + err.Error()}
		}
		out = append(out, survey.Check(w.ix, d).Problems...)
		for _, q := range d.Questions {
			if q.Status == "answered" {
				out = append(out, fmt.Sprintf("question %s is answered, but no person was asked in this session: leave it open", q.ID))
			}
		}
		for id, cl := range d.Claims() {
			if cl.Basis == survey.BasisConfirmed {
				out = append(out, fmt.Sprintf("claim %s is confirmed, but no person was asked in this session: mark it unknown with a question", id))
			}
		}
	case "backlog":
		bl, err := survey.ParseBacklog([]byte(reply))
		if err != nil {
			return []string{"backlog.json does not parse (unknown keys are refused): " + err.Error()}
		}
		r := survey.CheckBacklog(w.ix, w.survey, bl)
		out = append(out, r.Problems...)
		for _, it := range r.Items {
			for _, p := range it.Problems {
				out = append(out, "item "+it.Key+": "+p)
			}
		}
	}
	text := reply
	if c.Survey.Kind == "backlog" {
		text = withoutDecisionOptions(reply)
	}
	for _, r := range c.Scope {
		if m, clause := r.find(text); m != "" {
			out = append(out, fmt.Sprintf("scope: adds %s (%q in %q)", r.Why, m, clause))
		}
	}
	return out
}

// withoutDecisionOptions blanks each Decision item's options: a Decision
// names the alternatives it rejects ("1. Keep the patch"), which is not work
// the backlog adds. Its recommendation and every other item stay checked.
func withoutDecisionOptions(reply string) string {
	var v map[string]any
	if json.Unmarshal([]byte(reply), &v) != nil {
		return reply
	}
	items, _ := v["items"].([]any)
	for _, x := range items {
		it, _ := x.(map[string]any)
		if f, ok := it["fields"].(map[string]any); ok && it["type"] == "decision" {
			delete(f, "options")
		}
	}
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false) // keep "<?maruku" as written, or a pattern for it never matches
	if enc.Encode(v) != nil {
		return reply
	}
	return b.String()
}

// tier1Survey checks the recorded reply.
func tier1Survey(ctx context.Context, c Case, assets fs.FS) []string {
	w, err := surveySetup(ctx, c, assets)
	if err != nil {
		return []string{err.Error()}
	}
	b, err := os.ReadFile(filepath.Join(c.Dir, filepath.FromSlash(c.Survey.Draft)))
	if err != nil {
		return []string{err.Error()}
	}
	return checkSurvey(c, w, string(b))
}

// runSurvey is one tier-2 run of a survey case. The skill writes the JSON
// reply; when the checks fail it gets their findings once and revises, as a
// host does after `aayu survey --draft` or `--backlog`. A reply that still
// fails scores 1 without the judge (rule 7); one that passes is judged.
func (h *Harness) runSurvey(ctx context.Context, c Case, lane verbs.LaneID, body, input, rubric string) (run Run, err error) {
	var calls []CallLog
	defer func() { run.Calls = calls }()
	w, err := surveySetup(ctx, c, h.Assets)
	if err != nil {
		return Run{}, err
	}
	file, task := "survey.json", "Write survey.json for this request. No person is available in this session: leave every question open with your default, and never answer one yourself."
	ctxText := untrusted("aayu survey and aayu survey --module", w.printed) + "\n\n" + w.read
	if c.Survey.Kind == "backlog" {
		file, task = "backlog.json", "The survey below has passed `aayu survey --draft`, and its answered questions record the interview with the person. Write backlog.json from it."
		ctxText += untrusted("survey.json", w.surveyJ) + "\n\n"
	}
	req := models.Request{
		System: body + "\n\nYou have indexed the repository and read every module's pack and the files that follow. Reply with " + file + " only: one JSON object, nothing before or after it.",
		// The case's shared prefix (the skill, the packs and files, the request)
		// is cached once for every run and revision.
		Messages:  []models.Message{{Role: "user", Content: ctxText + "The person asks:\n\n" + untrusted("request", input) + "\n\n" + task, CacheBreak: true}},
		MaxTokens: h.Config.MaxTokens.Skill,
		// A survey draft runs for minutes; streaming keeps the gateway from
		// closing the idle connection (HTTP 524 in run 39014616715).
		Stream: true,
	}
	reply, problems, first, err := reviseOnce(ctx, h.complete(lane, "skill", &calls), req, func(d string) []string { return checkSurvey(c, w, d) })
	if err != nil {
		return Run{Revised: len(first) > 0, RevisedFor: revisedFor(first), Answer: answerOf(reply)}, err
	}
	run.Revised, run.RevisedFor = len(first) > 0, revisedFor(first)
	if len(problems) > 0 {
		return Run{Status: RunFail, Score: 1, Revised: true, RevisedFor: run.RevisedFor, Floor: "survey checks: " + clip(strings.Join(problems, "; "), 600), Answer: answerOf(reply)}, nil
	}
	checks := "Deterministic checks passed (`aayu survey --draft`): every citation resolves to an index fact or a line range inside the file; every claim has a basis its evidence allows; no product claim is observed; every unknown has a question; the modernisation target comes from the request; no question was answered without a person"
	if c.Survey.Kind == "backlog" {
		checks = "Deterministic checks passed (`aayu survey --backlog`): every item passes the work-item schema at its stage; every item rests on survey claims, answered questions, or index facts, never on an unknown; the Goal rests on a stated or confirmed product claim; every modernisation Story names the case sets its Completion Goal proves"
	}
	grounding := "## The person's request\n\n" + input + "\n\n## What the host saw (`aayu survey`, every module pack)\n\n" + w.printed + "\n\n## Files the host read\n\n" + w.read
	if c.Survey.Kind == "backlog" {
		grounding += "## survey.json (checked, with the recorded interview)\n\n" + w.surveyJ + "\n\n"
	}
	judgeInput := "## Task\n\n" + task + "\n\n" + grounding + "## Checks\n\n" + checks + "."
	// The self-check (D-5.9l): the draft that passed the floors is judged for
	// the record only, so its effect stays visible; the skill then verifies
	// every item against its evidence and only the corrected draft is scored.
	firstJudged, err := h.judge(ctx, judgeInput, rubric, prettyJSON(reply), nil, &calls)
	if err != nil {
		return Run{Revised: run.Revised, RevisedFor: run.RevisedFor, Answer: answerOf(reply)}, err
	}
	sc := &SelfCheck{FirstScore: firstJudged.Score, FirstReason: firstJudged.Reason, FirstDraft: actions.RedactString(reply)}
	// The self-check replies with corrections only (D-5.9n), applied to the
	// draft all or nothing; the floors then run on the result. A reply that
	// does not apply, or a result that fails the floors, gets one revision.
	// The draft is the second cached prefix: the self-check and its revision read it.
	req.Messages = append(req.Messages, models.Message{Role: "assistant", Content: reply, CacheBreak: true}, models.Message{Role: "user", Content: selfCheckPrompt(c.Survey.Kind, file)})
	var corrected string
	patch, problems, again, err := reviseOnceAsking(ctx, h.complete(lane, "skill", &calls), req, func(p string) []string {
		var refused []string
		corrected, sc.Changed, refused = applyCorrections(reply, p)
		if len(refused) > 0 {
			for i := range refused {
				refused[i] = "corrections not applied (none is, until all apply): " + refused[i]
			}
			return refused
		}
		return checkSurvey(c, w, corrected)
	}, "Fix every finding and reply with the whole corrections object again, for the "+file+" you self-checked, never the draft itself.")
	if err != nil {
		return Run{Revised: run.Revised, RevisedFor: run.RevisedFor, SelfCheck: sc, Answer: answerOf(reply)}, err
	}
	sc.Corrections, sc.Revised = actions.RedactString(patch), len(again) > 0
	if len(problems) > 0 {
		return Run{Status: RunFail, Score: 1, Revised: run.Revised, RevisedFor: run.RevisedFor, SelfCheck: sc,
			Floor: "survey checks after the self-check: " + clip(strings.Join(problems, "; "), 600), Answer: answerOf(cmp.Or(corrected, patch))}, nil
	}
	judged, err := h.judge(ctx, judgeInput, rubric, prettyJSON(corrected), nil, &calls)
	if err != nil {
		return Run{Revised: run.Revised, RevisedFor: run.RevisedFor, SelfCheck: sc, Answer: answerOf(corrected)}, err
	}
	judged.Revised, judged.RevisedFor, judged.SelfCheck = run.Revised, run.RevisedFor, sc
	judged.Answer = answerOf(corrected)
	return judged, nil
}

// SelfCheck is what a survey run's self-verification turn did (D-5.9l).
type SelfCheck struct {
	// FirstScore is the judge's score of the draft that passed the floors
	// before the self-check, recorded to show the turn's effect and never
	// counted; FirstReason is why.
	FirstScore  int    `json:"first_score"`
	FirstReason string `json:"first_reason,omitempty"`
	// FirstDraft is that draft in full, and Corrections the self-check's
	// reply that was applied to it (D-5.9n): together they reproduce the
	// scored draft exactly.
	FirstDraft  string `json:"first_draft,omitempty"`
	Corrections string `json:"corrections,omitempty"`
	// Changed lists what the applied corrections changed, as "E3 replaced",
	// "E5 removed", "R9 added", or "summary set".
	Changed []string `json:"changed"`
	// Revised is set when the corrected draft failed the floors once.
	Revised bool `json:"revised,omitempty"`
}

// selfCheckPrompt asks for the skill's self-check of a draft that passed,
// as corrections only (D-5.9n).
func selfCheckPrompt(kind, file string) string {
	check := "Now self-check it, as the skill's **Self-check** says: for every claim, trap finding, do-not-carry entry, interface, question and its default, risk, modernisation step, and case set, re-read the lines or probe facts it cites in what you were shown above, not your draft, and ask whether they say exactly this, for this scope, with these numbers, and for this build. Correct or narrow what they do not support, remove what nothing supports, or turn it into a risk or a question."
	id, lists, fields := "id", `"risks", "questions", or "modernisation.steps"`, `"summary" or "modernisation.strategy"`
	if kind == "backlog" {
		check = "Now self-check it, as the skill's section 4 says: for every item, re-read the claims and answers it rests on, in the survey and the files above, and ask whether the person asked for this work at this scope and whether each Completion Goal can be checked by the command it names. Correct, narrow, or remove what fails."
		id, lists, fields = "key", `"items"`, `"open_questions"`
	}
	return "Your " + file + " passes the checks. " + check + "\n\n" +
		"Do not rewrite the draft. Reply with your corrections only, as one JSON object:\n\n" +
		`{"verdicts": [{"id": "<` + id + `>", "verdict": "keep"}, {"id": "<` + id + `>", "verdict": "replace", "why": "…", "item": {the whole corrected item, with the same ` + id + `}}, {"id": "<` + id + `>", "verdict": "remove", "why": "…"}], "add": [{"list": "<list key path>", "why": "…", "item": {a new item with an unused ` + id + `}}], "fields": {"<key path>": <new value>}}` + "\n\n" +
		"- Give exactly one verdict for every item that has an " + id + ", in every list: keep it when its evidence supports it as written; replace it with the whole corrected item when it must change; remove it when nothing supports it.\n" +
		"- add appends a new item to a list by its key path, such as " + lists + ".\n" +
		"- fields sets text that has no " + id + ", by its key path, such as " + fields + "; never request or product.\n" +
		"- When you remove an item, correct every item that refers to it.\n\n" +
		"The corrections are applied to your draft exactly as written, all or nothing, and the checks run again on the result."
}

// prettyJSON indents a JSON reply for the judge, or returns it as it is.
func prettyJSON(s string) string {
	var b bytes.Buffer
	if json.Indent(&b, []byte(s), "", "  ") != nil {
		return s
	}
	return b.String()
}
