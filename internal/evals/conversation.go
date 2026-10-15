package evals

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/grill"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// Answer kinds of the scripted human (WP5.3b): confirm states or agrees to
// something specific, default accepts the default the skill offered, park
// and defer keep the topic out of the criteria (a parked idea, an open
// question).
const (
	KindConfirm = "confirm"
	KindDefault = "default"
	KindPark    = "park"
	KindDefer   = "defer"
)

// Answer is one scripted reply.
type Answer struct {
	Kind string `toml:"kind" json:"kind"`
	Text string `toml:"text" json:"text"`
}

// Human is a case's simulated human ([tier2] human): it answers each question
// by the catalogue dimension the skill tagged it with, so a run is
// reproducible whatever the wording of the question.
type Human struct {
	MaxRounds int               `toml:"max_rounds"`
	Fallback  Answer            `toml:"fallback"` // any dimension without a scripted answer
	Readback  string            `toml:"readback"` // the human's reply to the readback
	Answers   map[string]Answer `toml:"answers"`  // by dimension ID (ux.states)
}

// LoadHuman reads a scripted-human file.
func LoadHuman(path string) (Human, error) {
	var h Human
	b, err := os.ReadFile(path)
	if err != nil {
		return h, err
	}
	dec := toml.NewDecoder(strings.NewReader(string(b)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&h); err != nil {
		return h, fmt.Errorf("%s: %w", path, err)
	}
	if h.MaxRounds < 1 || h.Fallback.Text == "" {
		return h, fmt.Errorf("%s: max_rounds >= 1 and a fallback answer are required", path)
	}
	for dim, a := range h.Answers {
		if !slices.Contains([]string{KindConfirm, KindDefault, KindPark, KindDefer}, a.Kind) || a.Text == "" {
			return h, fmt.Errorf("%s: answers.%s: want kind confirm|default|park|defer and a text", path, dim)
		}
	}
	if !slices.Contains([]string{KindPark, KindDefer}, h.Fallback.Kind) {
		return h, fmt.Errorf("%s: the fallback must park or defer: an unscripted topic never becomes a requirement", path)
	}
	return h, nil
}

// Answer returns the scripted reply to a question tagged with dimension.
func (h Human) Answer(dimension string) Answer {
	if a, ok := h.Answers[dimension]; ok {
		return a
	}
	return h.Fallback
}

// Question is one question of a skill's turn.
type Question struct {
	Dimension string   `json:"dimension"`
	Ask       string   `json:"ask"`
	Default   string   `json:"default,omitempty"`
	Facts     []string `json:"facts,omitempty"`
}

// Turn is one reply of the skill in a grill conversation: what it says
// (reflection, proposals, or the final readback), its questions, and, when
// done, the draft.
type Turn struct {
	Say       string     `json:"say"`
	Questions []Question `json:"questions,omitempty"`
	Done      bool       `json:"done"`
	Draft     *draft     `json:"draft,omitempty"`
}

// Exchange is a turn with the human's replies, as a transcript records it.
type Exchange struct {
	Say       string     `json:"say"`
	Questions []Question `json:"questions,omitempty"`
	Answers   []Answer   `json:"answers,omitempty"`
	Done      bool       `json:"done,omitempty"`
}

// Transcript is a grill conversation: the request, the exchanges, and the
// final draft.
type Transcript struct {
	Request   string     `json:"request"`
	Exchanges []Exchange `json:"exchanges"`
}

// conversationProtocol is appended to the skill body in a conversation case:
// how the eval's human is reached. It changes the channel, not the skill.
const conversationProtocol = `## This conversation

You are grilling a human through a channel that carries JSON. Each reply is one turn:
{"say": "...", "questions": [{"dimension": "<catalogue dimension ID, or request, or other>", "ask": "...", "default": "...", "facts": ["<fact ID>"]}], "done": false}
Ask at most five questions per turn, tag each with the catalogue dimension it settles, and cite assessment facts by ID.
The human's answers come back numbered in the order you asked. When the spec is concrete, reply with "done": true,
your plain-language readback in "say" ("Here is what done looks like: ..."), no questions, and "draft": the work item
as JSON {type, title, fields, labels, parent, open_questions}, its fields including assumptions, open_questions,
parked_ideas, and provenance as the skill describes.`

// turnSchema is the JSON Schema of a turn, with the draft schema inside.
func turnSchema(draftSchema []byte) ([]byte, error) {
	var d any
	if err := json.Unmarshal(draftSchema, &d); err != nil {
		return nil, err
	}
	if m, ok := d.(map[string]any); ok {
		delete(m, "$comment")
	}
	s := map[string]any{
		"type": "object", "required": []string{"say", "done"}, "additionalProperties": false,
		"properties": map[string]any{
			"say":  map[string]any{"type": "string", "minLength": 1},
			"done": map[string]any{"type": "boolean"},
			"questions": map[string]any{"type": "array", "maxItems": 5, "items": map[string]any{
				"type": "object", "required": []string{"dimension", "ask"}, "additionalProperties": false,
				"properties": map[string]any{
					"dimension": map[string]any{"type": "string", "minLength": 1},
					"ask":       map[string]any{"type": "string", "minLength": 1},
					"default":   map[string]any{"type": "string"},
					"facts":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				}}},
			"draft": d,
		},
	}
	return json.Marshal(s)
}

// answerText is the human's message for a turn's questions.
func answerText(as []Answer) string {
	var b strings.Builder
	for i, a := range as {
		fmt.Fprintf(&b, "%d. %s\n", i+1, a.Text)
	}
	return strings.TrimSpace(b.String())
}

// CaseRepo returns the absolute path of the case's repository fixture.
func (c Case) CaseRepo() string {
	switch {
	case c.Repo == "":
		return ""
	case filepath.IsAbs(c.Repo):
		return c.Repo
	}
	return filepath.Join(c.Dir, filepath.FromSlash(c.Repo))
}

// Assess runs grill's fact pass on the case's repository fixture for idea,
// with the fixture's own configuration, as `aayu grill <type> --assess`
// would in that checkout (related stories come from the case's GitHub
// fixture issues.json).
func (c Case) Assess(assets fs.FS, typ, idea string) (grill.Assessment, grill.Catalog, error) {
	cat, err := grill.LoadCatalog(assets)
	if err != nil {
		return grill.Assessment{}, cat, err
	}
	root := c.CaseRepo()
	l, err := config.Load(config.Sources{RepoDir: root})
	if err != nil {
		return grill.Assessment{}, cat, err
	}
	in := grill.AssessInput{Root: root, Type: typ, Idea: idea, Catalog: cat, Verify: l.Config.Verify}
	var is []struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		State  string `json:"state"`
		Body   string `json:"body"`
	}
	if err := readJSON(filepath.Join(c.Dir, "github", "repos", "o", "r", "issues.json"), &is); err == nil {
		for _, i := range is {
			in.Stories = append(in.Stories, grill.Issue{Number: i.Number, Title: i.Title, State: i.State, Body: i.Body})
		}
	}
	return grill.Assess(in), cat, nil
}

var notWord = regexp.MustCompile(`[^\p{L}\p{N}]+`)

func norm(s string) string {
	return strings.TrimSpace(notWord.ReplaceAllString(strings.ToLower(s), " "))
}

// Concrete is the deterministic part of judging a grilled spec (WP5.3b),
// shared by tier 1 (the recorded run) and tier 2 (before the judge): the
// spec is concrete (grill.Check), cites only facts the assessment has, and
// at least minCited criteria cite one; and it is faithful to the transcript:
// every criterion's quote is the human's own words (the request or a reply),
// and none comes from a dimension the human parked, deferred, or was never
// asked about, or claims a source the reply does not give.
func Concrete(cat grill.Catalog, a grill.Assessment, it workitems.Item, tr Transcript, minCited int) []string {
	var out []string
	for _, p := range grill.Check(cat, it) {
		out = append(out, p.String())
	}
	prov := it.Fields[workitems.FieldProvenance]
	if workitems.Empty(prov) {
		return append(out, "provenance: a grilled spec records its Provenance")
	}
	p, _ := grill.ParseProvenance(prov)
	for _, pr := range grill.CheckFacts(p, a) {
		out = append(out, pr.String())
	}
	cited := 0
	for _, s := range p.Criteria {
		if len(s.Facts) > 0 {
			cited++
		}
	}
	if cited < minCited {
		out = append(out, fmt.Sprintf("provenance: %d criteria cite an assessment fact; the case wants at least %d", cited, minCited))
	}
	if norm(p.Request) != norm(tr.Request) {
		out = append(out, fmt.Sprintf("provenance: Request %q is not the human's request", clip(p.Request, 80)))
	}
	said := map[string][]Answer{} // dimension → the human's replies
	var all []string
	all = append(all, norm(tr.Request))
	for _, x := range tr.Exchanges {
		for i, q := range x.Questions {
			if i < len(x.Answers) {
				said[q.Dimension] = append(said[q.Dimension], x.Answers[i])
				all = append(all, norm(x.Answers[i].Text))
			}
		}
	}
	for _, s := range p.Criteria {
		var unsaid []string
		inRequest := true
		for _, part := range s.Quotes {
			q := norm(part)
			if q == "" || !slices.ContainsFunc(all, func(t string) bool { return strings.Contains(t, q) }) {
				unsaid = append(unsaid, part)
			}
			inRequest = inRequest && strings.Contains(norm(tr.Request), q)
		}
		if len(s.Quotes) == 0 || len(unsaid) > 0 {
			out = append(out, fmt.Sprintf("fidelity: criterion %d quotes %q, which the human never said", s.N, clip(strings.Join(unsaid, " / "), 80)))
			continue
		}
		if s.Dimension == "" || s.Dimension == "request" {
			if !inRequest && s.Kind == grill.SourceStated {
				out = append(out, fmt.Sprintf("fidelity: criterion %d is stated in the request, but the request does not say %q", s.N, clip(s.Quote, 80)))
			}
			continue
		}
		replies := said[s.Dimension]
		var kinds []string
		for _, r := range replies {
			kinds = append(kinds, r.Kind)
		}
		switch {
		case len(replies) == 0:
			out = append(out, fmt.Sprintf("fidelity: criterion %d comes from %s, which the human was never asked about", s.N, s.Dimension))
		case !slices.Contains(kinds, KindConfirm) && !slices.Contains(kinds, KindDefault):
			out = append(out, fmt.Sprintf("fidelity: criterion %d comes from %s, which the human %s; it belongs in Parked ideas or Open questions", s.N, s.Dimension, strings.Join(kinds, " and ")))
		case s.Kind == grill.SourceDefault && !slices.Contains(kinds, KindDefault):
			out = append(out, fmt.Sprintf("fidelity: criterion %d claims an accepted default on %s, but the human did not accept a default there", s.N, s.Dimension))
		}
	}
	return out
}

// errNotDone is a conversation that never produced its draft.
var errNotDone = errors.New("the skill did not finish the spec")
