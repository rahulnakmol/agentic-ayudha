// Package evals is the skill eval harness (WP3.4). Tier 1 is deterministic and
// runs on every PR through go test: recorded runs of a skill must have their
// artifacts, artifacts must validate against their schemas, produced work items
// must pass Ready validation, every recorded `aayu` call must succeed, and
// injection fixtures must cause no writes. Tier 2 runs the skill on a model k
// times, has a pinned judge from another model family score each run against a
// rubric, and compares the results with the baseline for main's commit.
package evals

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"github.com/rahulnakmol/agentic-ayudha/internal/grill"
	"github.com/rahulnakmol/agentic-ayudha/internal/tell"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// Config is evals/eval.toml.
type Config struct {
	Judge     string  `toml:"judge"`      // pinned judge model ID
	K         int     `toml:"k"`          // runs per tier-2 case
	PassScore int     `toml:"pass_score"` // a run passes at this rubric score or above (1-5)
	MinPassK  float64 `toml:"min_pass_k"` // suite pass^k bar
	MinMean   float64 `toml:"min_mean"`   // suite mean rubric score bar
	MaxDrop   float64 `toml:"max_drop"`   // largest allowed fall of the mean below the baseline
	MaxUSD    float64 `toml:"-"`          // spend cap of the run: the selected group's max_usd (ForGroup)
	Groups    []Group `toml:"groups"`     // tier-2 case groups, each run as its own job with its own spend cap
	MaxTokens struct {
		Skill int `toml:"skill"`
		Judge int `toml:"judge"`
	} `toml:"max_tokens"`
}

// Group is a set of tier-2 cases run together with their own spend cap, so
// one group can never use up another's (D-7.3d). A case belongs to the first
// group with a prefix of its ID, else to the one group without prefixes.
type Group struct {
	Name     string   `toml:"name"`
	Prefixes []string `toml:"prefixes"` // case ID prefixes; none: every case no other group claims
	MaxUSD   float64  `toml:"max_usd"`  // spend cap of one run of this group, on top of [budget]
	// PassK is "required" (the default: the full WP3.4 bar) or "advisory": the
	// group passes on the mean bar and the drop rule, and its pass^k findings
	// are printed but do not fail it (D-7.3g).
	PassK string `toml:"pass_k"`
	// Gate is "enforced" (the default: a failing result fails evals-gate) or
	// "advisory": results are computed and printed but never gate (D-7.3h).
	Gate string `toml:"gate"`
	// K overrides the runs per case for this group; 0 keeps eval.toml's k.
	K int `toml:"k"`
	// Events are the GitHub event names (EVENT in evals.yml) the group runs
	// on; empty runs on every event. A costly group runs on dispatch only.
	Events []string `toml:"events"`
}

// RunsOn reports whether a group runs for the workflow event (empty: any).
func (g Group) RunsOn(event string) bool {
	return len(g.Events) == 0 || event == "" || slices.Contains(g.Events, event)
}

// gate policies of a group.
const (
	GateEnforced = "enforced"
	GateAdvisory = "advisory"
)

// Gates reports whether the group's results gate a promotion. An unknown
// group gates, so a typo cannot open the gate.
func (c Config) Gates(group string) bool {
	g, ok := c.Group(group)
	return !ok || g.Gate != GateAdvisory
}

// pass_k policies of a group.
const (
	PassKRequired = "required"
	PassKAdvisory = "advisory"
)

// Group returns the group named name.
func (c Config) Group(name string) (Group, bool) {
	for _, g := range c.Groups {
		if g.Name == name {
			return g, true
		}
	}
	return Group{}, false
}

// GroupOf returns the name of the group case ID id belongs to.
func (c Config) GroupOf(id string) string {
	rest := ""
	for _, g := range c.Groups {
		if len(g.Prefixes) == 0 {
			rest = g.Name
		}
		for _, p := range g.Prefixes {
			if strings.HasPrefix(id, p) {
				return g.Name
			}
		}
	}
	return rest
}

// ForGroup returns the config for one group's run (MaxUSD is the group's cap)
// and the cases that belong to it.
func (c Config) ForGroup(name string, cases []Case) (Config, []Case, error) {
	for _, g := range c.Groups {
		if g.Name != name {
			continue
		}
		c.MaxUSD = g.MaxUSD
		if g.K > 0 {
			c.K = g.K
		}
		var out []Case
		for _, k := range cases {
			if c.GroupOf(k.ID) == name {
				out = append(out, k)
			}
		}
		return c, out, nil
	}
	var names []string
	for _, g := range c.Groups {
		names = append(names, g.Name)
	}
	return c, nil, fmt.Errorf("no tier-2 group %q in evals/eval.toml; have %s", name, strings.Join(names, ", "))
}

// Case is evals/cases/<skill>/<name>/case.toml: a recorded run of a skill plus
// what tier 2 needs to run it again.
type Case struct {
	ID          string     `toml:"-"` // <skill>/<name>
	Dir         string     `toml:"-"`
	Skill       string     `toml:"skill"`
	Description string     `toml:"description"`
	Input       string     `toml:"input"`     // file with the task given to the skill
	Injection   bool       `toml:"injection"` // the input carries a prompt injection; the run must write nothing
	Artifacts   []string   `toml:"artifacts"` // files the recorded run produced
	Schemas     []Schema   `toml:"schemas"`
	Ready       []string   `toml:"ready"` // artifacts holding work items that must pass Ready
	Calls       [][]string `toml:"calls"` // the recorded run's `aayu` calls, without the program name
	Scope       []Scope    `toml:"scope"` // requirements the input does not state; a work item that adds one fails
	Brief       *BriefSpec `toml:"brief"` // a brief case: the input is the grounding, the output a Markdown draft
	Tell        *TellSpec  `toml:"tell"`  // a tell case: the input is the grounding, the output Markdown copy (D-7.5b)
	// Repo is a repository fixture (a directory relative to the case) the
	// recorded calls run in and grill's --assess reads (WP5.3b).
	Repo string `toml:"repo"`
	// Transcript is the recorded grill conversation (WP5.3b); with it, tier 1
	// checks the spec is concrete and faithful to what the human said.
	Transcript string `toml:"transcript"`
	// Cite is how many criteria must cite an assessment fact.
	Cite int `toml:"cite"`
	// Gate is the vagueness gate's expected verdict on the request (WP5.8):
	// "unpack" (the skill must stop and ask) or "pass" (it must ask nothing).
	Gate string `toml:"gate"`
	// Persona makes a persona case (WP5.1b): the input is the grounding, the
	// output the seat twin's Markdown answer at a team stage.
	Persona *PersonaSpec `toml:"persona"`
	// Pack makes a decision-pack case (WP5.1b, D-5.6b).
	Pack *PackSpec `toml:"pack"`
	// Feedback and Reproduce make an evolve case (WP7.7): the accepted Feedback
	// issue it reproduces, and the directory holding the old skill's recorded
	// run (the output/ artifacts under that name), which must fail tier 1.
	Feedback  int        `toml:"feedback"`
	Reproduce string     `toml:"reproduce"`
	Tier2     *Tier2Spec `toml:"tier2"`
	// Review makes a seeded-defect case for inspect (WP4.8b): the input is the
	// PR's diff; tier 2 runs it in every review variant and scores precision
	// and recall against the planted defects, without a judge.
	Review *ReviewSpec `toml:"review"`
}

// Scope is one known way a skill invents a requirement for this case's input,
// as a pattern over the produced work item's title and fields. It is a per-case
// deterministic guard for a defect the rubric already caught once (as WP7.7
// evolve keeps a failing case); the judge remains the general check.
type Scope struct {
	Pattern string `toml:"pattern"`
	Why     string `toml:"why"`
	// Allow, when set, matches phrases that honour the rule rather than break
	// it ("no price" where the human ruled prices out); they are removed before
	// Pattern is matched (WP5.8).
	Allow   string `toml:"allow"`
	re      *regexp.Regexp
	allowRE *regexp.Regexp
}

// find returns the rule's first match in text, outside the allowed phrases,
// and the clause around it, so a floor shows what the skill wrote.
func (r Scope) find(text string) (match, clause string) {
	if r.allowRE != nil {
		text = r.allowRE.ReplaceAllStringFunc(text, func(m string) string { return strings.Repeat(" ", len(m)) })
	}
	loc := r.re.FindStringIndex(text)
	if loc == nil {
		return "", ""
	}
	start, end := max(0, loc[0]-80), min(len(text), loc[1]+80)
	if i := strings.LastIndexAny(text[start:loc[0]], ".;\n\""); i >= 0 {
		start += i + 1
	}
	if i := strings.IndexAny(text[loc[1]:end], ".;\n\""); i >= 0 {
		end = loc[1] + i
	}
	return text[loc[0]:loc[1]], strings.Join(strings.Fields(text[start:end]), " ")
}

// OutOfScope returns one line per scope rule the item breaks. A grilled
// spec's record (assumptions, open questions, parked ideas, provenance) is
// where unconfirmed ideas belong, so the rules do not read it (WP5.3b).
func (c Case) OutOfScope(it workitems.Item) []string {
	keys := make([]string, 0, len(it.Fields))
	for k := range it.Fields {
		switch k {
		case workitems.FieldAssumptions, workitems.FieldOpenQuestions, workitems.FieldParked, workitems.FieldProvenance:
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	texts := []string{it.Title}
	for _, k := range keys {
		texts = append(texts, it.Fields[k])
	}
	return c.outOfScopeText(strings.Join(texts, "\n"))
}

// outOfScopeText returns one line per scope rule the text breaks.
func (c Case) outOfScopeText(text string) []string {
	var out []string
	for _, r := range c.Scope {
		if m, clause := r.find(text); m != "" {
			out = append(out, fmt.Sprintf("adds %s (%q in %q)", r.Why, m, clause))
		}
	}
	return out
}

// Schema ties an artifact to the JSON Schema it must validate against.
type Schema struct {
	File   string `toml:"file"`
	Schema string `toml:"schema"` // relative to the repository root
}

// Tier2Spec configures a case's rubric run.
type Tier2Spec struct {
	Rubric string `toml:"rubric"` // file in the case directory
	// Verb is the registry verb the skill stands in for; empty when the skill is
	// itself a verb. The skill model is that verb's registry lane on the direct
	// host (never a model named here), so evals run on the model the verb uses.
	Verb string `toml:"verb"`
	// References are files under the skill's references/ that the host reads
	// for this case (the brief skill's ste-words.md for the ste register); they
	// follow the skill body in the prompt.
	References []string `toml:"references"`
	// Draft is the shape of the skill's reply: empty for a work item, or
	// "slice" or "plan" for those verbs' drafts (WP5.4), checked by
	// internal/planning against Issue, the case's github/ fixture of the
	// Feature or story the draft is for, before the judge is asked.
	Draft string `toml:"draft"`
	Issue int    `toml:"issue"`
	// Human is the scripted human (conversation.go) of a grill conversation
	// case: the skill asks in rounds and the script answers by dimension.
	Human string `toml:"human"`
	// Session is the session.toml (session.go) of a host-skill session case
	// (D-3.4f): the skill works in turns, running commands against the case's
	// world, and the finished session is checked against its floors.
	Session string `toml:"session"`
}

var groupName = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// LoadConfig reads evals/eval.toml under root.
func LoadConfig(root string) (Config, error) {
	var c Config
	b, err := os.ReadFile(filepath.Join(root, "evals", "eval.toml"))
	if err != nil {
		return c, err
	}
	if err := toml.Unmarshal(b, &c); err != nil {
		return c, fmt.Errorf("evals/eval.toml: %w", err)
	}
	if c.Judge == "" || c.K < 1 || c.PassScore < 1 || c.PassScore > 5 {
		return c, errors.New("evals/eval.toml: judge, k >= 1, and pass_score 1-5 are required")
	}
	seen, rest := map[string]bool{}, 0
	for _, g := range c.Groups {
		if !groupName.MatchString(g.Name) || seen[g.Name] || g.MaxUSD <= 0 {
			return c, fmt.Errorf("evals/eval.toml: group %q: want a unique lowercase name and max_usd > 0", g.Name)
		}
		if g.PassK != "" && g.PassK != PassKRequired && g.PassK != PassKAdvisory {
			return c, fmt.Errorf("evals/eval.toml: group %q: pass_k %q: want %q or %q", g.Name, g.PassK, PassKRequired, PassKAdvisory)
		}
		if g.Gate != "" && g.Gate != GateEnforced && g.Gate != GateAdvisory {
			return c, fmt.Errorf("evals/eval.toml: group %q: gate %q: want %q or %q", g.Name, g.Gate, GateEnforced, GateAdvisory)
		}
		seen[g.Name] = true
		if len(g.Prefixes) == 0 {
			rest++
		}
	}
	if rest != 1 {
		return c, errors.New("evals/eval.toml: exactly one [[groups]] entry has no prefixes (it takes every other case)")
	}
	return c, nil
}

// LoadCases reads every evals/cases/<skill>/<name>/case.toml under root, sorted by ID.
func LoadCases(root string) ([]Case, error) {
	files, err := filepath.Glob(filepath.Join(root, "evals", "cases", "*", "*", "case.toml"))
	if err != nil {
		return nil, err
	}
	var out []Case
	for _, f := range files {
		c, err := LoadCase(filepath.Dir(f))
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// LoadCase reads one case directory.
func LoadCase(dir string) (Case, error) {
	var c Case
	b, err := os.ReadFile(filepath.Join(dir, "case.toml"))
	if err != nil {
		return c, err
	}
	dec := toml.NewDecoder(strings.NewReader(string(b)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return c, fmt.Errorf("%s: %w", filepath.Join(dir, "case.toml"), err)
	}
	c.Dir = dir
	c.ID = filepath.Base(filepath.Dir(dir)) + "/" + filepath.Base(dir)
	for i, r := range c.Scope {
		re, err := regexp.Compile(r.Pattern)
		if err != nil {
			return c, fmt.Errorf("%s: scope rule %d: %w", c.ID, i+1, err)
		}
		if r.Why == "" {
			return c, fmt.Errorf("%s: scope rule %d: why is required", c.ID, i+1)
		}
		c.Scope[i].re = re
		if r.Allow != "" {
			if c.Scope[i].allowRE, err = regexp.Compile(r.Allow); err != nil {
				return c, fmt.Errorf("%s: scope rule %d allow: %w", c.ID, i+1, err)
			}
		}
	}
	if c.Skill == "" || c.Input == "" {
		return c, fmt.Errorf("%s: skill and input are required", c.ID)
	}
	if c.Gate != "" && c.Gate != grill.GateUnpack && c.Gate != grill.GatePass {
		return c, fmt.Errorf("%s: gate %q: want %q or %q", c.ID, c.Gate, grill.GateUnpack, grill.GatePass)
	}
	if c.Gate != "" && c.Transcript == "" {
		return c, fmt.Errorf("%s: gate needs a transcript", c.ID)
	}
	if c.Transcript != "" && c.Repo == "" {
		return c, fmt.Errorf("%s: a transcript needs the repo fixture it was grilled in", c.ID)
	}
	if b := c.Brief; b != nil && (b.Register == "" || b.Purpose == "" || b.Draft == "") {
		return c, fmt.Errorf("%s: [brief] needs register, purpose, and draft", c.ID)
	}
	if t := c.Tell; t != nil && (!slices.Contains(tell.Kinds, t.Kind) || t.Request == "" || t.Draft == "") {
		return c, fmt.Errorf("%s: [tell] needs a kind (%s), request, and draft", c.ID, strings.Join(tell.Kinds, ", "))
	}
	if p := c.Persona; p != nil && (p.Seat == "" || p.Stage == "" || p.Practice == "" || p.Task == "" || p.Draft == "") {
		return c, fmt.Errorf("%s: [persona] needs seat, stage, practice, task, and draft", c.ID)
	}
	if p := c.Pack; p != nil && (p.Seat == "" || p.Stage == "" || p.Question == "" || p.Draft == "") {
		return c, fmt.Errorf("%s: [pack] needs seat, stage, question, and draft", c.ID)
	}
	return c, nil
}

// readJSON decodes a JSON file into v.
func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}
