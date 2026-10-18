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

// Human is a case's simulated human ([tier2] human). The skill tags each
// question with a topic, <dimension>/<key> (ux.steps/confirmation-placement,
// request/dependency), the keys coming from the catalogue's Topics; the
// human answers by that topic, else by the topic's dimension, else with the
// fallback. A run is reproducible whatever the wording of the questions, and
// a follow-up on another topic of the same dimension gets its own answer.
type Human struct {
	MaxRounds  int               `toml:"max_rounds"`
	Fallback   Answer            `toml:"fallback"`   // any topic and dimension without a scripted answer
	Readback   string            `toml:"readback"`   // the human's reply to the readback
	Topics     map[string]Answer `toml:"topics"`     // by topic (ux.steps/screen-count)
	Dimensions map[string]Answer `toml:"dimensions"` // by dimension (ux.steps), for topics without their own answer
}

var topicRE = regexp.MustCompile(`^[a-z][a-z0-9-]*(\.[a-z][a-z0-9-]*)?/[a-z0-9][a-z0-9-]*$`)

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
	for _, m := range []map[string]Answer{h.Topics, h.Dimensions} {
		for key, a := range m {
			if !slices.Contains([]string{KindConfirm, KindDefault, KindPark, KindDefer}, a.Kind) || a.Text == "" {
				return h, fmt.Errorf("%s: %s: want kind confirm|default|park|defer and a text", path, key)
			}
		}
	}
	for t := range h.Topics {
		if !topicRE.MatchString(t) {
			return h, fmt.Errorf("%s: topics.%s: want <dimension>/<key>", path, t)
		}
	}
	if !slices.Contains([]string{KindPark, KindDefer}, h.Fallback.Kind) {
		return h, fmt.Errorf("%s: the fallback must park or defer: an unscripted topic never becomes a requirement", path)
	}
	return h, nil
}

// Answer returns the scripted reply to a question tagged with topic.
func (h Human) Answer(topic string) Answer {
	if a, ok := h.Topics[topic]; ok {
		return a
	}
	if a, ok := h.Dimensions[dimensionOf(topic)]; ok {
		return a
	}
	return h.Fallback
}

func dimensionOf(topic string) string { d, _, _ := strings.Cut(topic, "/"); return d }

// Question is one question of a skill's turn.
type Question struct {
	Topic   string   `json:"topic"` // <dimension>/<key>
	Ask     string   `json:"ask"`
	Default string   `json:"default,omitempty"`
	Facts   []string `json:"facts,omitempty"`
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
{"say": "...", "questions": [{"topic": "<dimension>/<topic>", "ask": "...", "default": "...", "facts": ["<fact ID>"]}], "done": false}
Ask at most five questions per turn. Tag each with the topic it settles: a catalogue dimension and one of its Topics
(ux.steps/confirmation-placement), a request topic (request/dependency, request/size, request/existing-behaviour,
request/parent, request/scope), term/<term> for the meaning of an abbreviation (term/pdp), or other/<key> for
anything else. Cite assessment facts by ID. Read the assessment's gate first: when it is "pass", ask nothing and
reply "done" with the draft in your first turn.
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
				"type": "object", "required": []string{"topic", "ask"}, "additionalProperties": false,
				"properties": map[string]any{
					"topic":   map[string]any{"type": "string", "pattern": topicRE.String()},
					"ask":     map[string]any{"type": "string", "minLength": 1},
					"default": map[string]any{"type": "string"},
					"facts":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
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
	// In CI the local layer is never read (Bug #640), as for every command.
	l, err := config.Load(config.Sources{RepoDir: root, CI: config.InCI(os.Getenv)})
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
// every criterion's and term's quote is the human's own words (the request or a reply),
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
	said, saidDim := map[string][]Answer{}, map[string][]Answer{} // topic, dimension → the human's replies
	var all []string
	all = append(all, norm(tr.Request))
	for _, x := range tr.Exchanges {
		for i, q := range x.Questions {
			if i < len(x.Answers) {
				said[q.Topic] = append(said[q.Topic], x.Answers[i])
				saidDim[dimensionOf(q.Topic)] = append(saidDim[dimensionOf(q.Topic)], x.Answers[i])
				all = append(all, norm(x.Answers[i].Text))
			}
		}
	}
	for _, t := range p.Terms {
		for _, part := range t.Quotes {
			if q := norm(part); q == "" || !slices.ContainsFunc(all, func(x string) bool { return strings.Contains(x, q) }) {
				out = append(out, fmt.Sprintf("fidelity: term %q quotes %q, which the human never said", t.Term, clip(part, 80)))
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
		if s.Topic == "" && (s.Dimension == "" || s.Dimension == "request") {
			if !inRequest && s.Kind == grill.SourceStated {
				out = append(out, fmt.Sprintf("fidelity: criterion %d is stated in the request, but the request does not say %q", s.N, clip(s.Quote, 80)))
			}
			continue
		}
		replies, asked := saidDim[s.Dimension], s.Dimension
		if s.Topic != "" {
			replies, asked = said[s.Topic], s.Topic
		}
		var kinds []string
		for _, r := range replies {
			kinds = append(kinds, r.Kind)
		}
		switch {
		case len(replies) == 0:
			out = append(out, fmt.Sprintf("fidelity: criterion %d comes from %s, which the human was never asked about", s.N, asked))
		case !slices.Contains(kinds, KindConfirm) && !slices.Contains(kinds, KindDefault):
			out = append(out, fmt.Sprintf("fidelity: criterion %d comes from %s, which the human %s; it belongs in Parked ideas or Open questions", s.N, asked, strings.Join(kinds, " and ")))
		case s.Kind == grill.SourceDefault && !slices.Contains(kinds, KindDefault):
			out = append(out, fmt.Sprintf("fidelity: criterion %d claims an accepted default on %s, but the human did not accept a default there", s.N, asked))
		}
	}
	return out
}

// GateFloor is the vagueness gate's deterministic check of a conversation
// (WP5.8), shared by tier 1 and tier 2: on "pass" the skill asks nothing (a
// concrete ask gets no friction), and on "unpack" its first turn asks
// before it writes anything.
func GateFloor(verdict string, first Turn) string {
	switch {
	case verdict == grill.GatePass && len(first.Questions) > 0:
		return fmt.Sprintf("the gate passed a concrete request, but the skill asked %d question(s)", len(first.Questions))
	case verdict == grill.GateUnpack && (first.Done || len(first.Questions) == 0):
		return "the gate said unpack, but the skill's first turn asked nothing before writing the spec"
	}
	return ""
}

// errNotDone is a conversation that never produced its draft.
var errNotDone = errors.New("the skill did not finish the spec")
