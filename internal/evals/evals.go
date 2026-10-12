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
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"

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
	MaxUSD    float64 `toml:"max_usd"`    // spend cap for one tier-2 run, on top of [budget]
	MaxTokens struct {
		Skill int `toml:"skill"`
		Judge int `toml:"judge"`
	} `toml:"max_tokens"`
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
	Tier2       *Tier2Spec `toml:"tier2"`
}

// Scope is one known way a skill invents a requirement for this case's input,
// as a pattern over the produced work item's title and fields. It is a per-case
// deterministic guard for a defect the rubric already caught once (as WP7.7
// evolve keeps a failing case); the judge remains the general check.
type Scope struct {
	Pattern string `toml:"pattern"`
	Why     string `toml:"why"`
	re      *regexp.Regexp
}

// OutOfScope returns one line per scope rule the item breaks.
func (c Case) OutOfScope(it workitems.Item) []string {
	keys := make([]string, 0, len(it.Fields))
	for k := range it.Fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	texts := []string{it.Title}
	for _, k := range keys {
		texts = append(texts, it.Fields[k])
	}
	text := strings.Join(texts, "\n")
	var out []string
	for _, r := range c.Scope {
		if m := r.re.FindString(text); m != "" {
			out = append(out, fmt.Sprintf("adds %s (%q)", r.Why, m))
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
}

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
	}
	if c.Skill == "" || c.Input == "" {
		return c, fmt.Errorf("%s: skill and input are required", c.ID)
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
