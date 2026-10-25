package ready

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// Each part of a Feature pins what it owns (Story #992, core design S5), and
// nothing else, so a wording edit outside it is not drift:
//
//   - intent: the Feature's body;
//   - spec: each child story's pinned fields (StoryPin), and the set of stories;
//   - tech: specs/<f>/design.json and each chart note on the Feature;
//   - ux: each design stage manifest on the ledger, and the head of each open
//     PR of an early-build story.

// StoryPin is what a spec baseline pins of one story: its title, Completion
// Goal, effort, layers, and dependencies, and not the rest of its body.
type StoryPin struct {
	Number             int      `json:"number"`
	Title              string   `json:"title"`
	CompletionGoalHash string   `json:"completion_goal_hash"` // workitems.CompletionGoalHash
	Effort             string   `json:"effort"`               // the effort:<v> label's value
	Layers             []string `json:"layers"`               // the layer:<id> labels' values, sorted
	DependsOn          []int    `json:"depends_on"`           // the depends-on links, sorted
}

func (s StoryPin) hash() string {
	b, _ := json.Marshal(s)
	return sum(b)
}

func sum(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// IntentPins pins the Feature's body.
func IntentPins(f int, body string) []Pin {
	return []Pin{{Ref: fmt.Sprintf("#%d body", f), Hash: workitems.BodyHash(body)}}
}

// SpecPins pins each story by its pinned fields, in number order, then the
// set of story numbers, so an added or removed story is drift too.
func SpecPins(f int, stories []workitems.Item) []Pin {
	var pins []Pin
	nums := []int{}
	for _, it := range stories {
		s := StoryPin{Number: it.Number, Title: it.Title, CompletionGoalHash: workitems.CompletionGoalHash(it), Layers: []string{}, DependsOn: []int{}}
		for _, l := range it.Labels {
			if v, ok := strings.CutPrefix(l, "effort:"); ok {
				s.Effort = v
			} else if v, ok := strings.CutPrefix(l, "layer:"); ok {
				s.Layers = append(s.Layers, v)
			}
		}
		slices.Sort(s.Layers)
		links, _ := workitems.ParseLinks(it.Fields[workitems.FieldLinks])
		for _, l := range links {
			if l.Kind == "depends-on" {
				s.DependsOn = append(s.DependsOn, l.Number)
			}
		}
		slices.Sort(s.DependsOn)
		s.DependsOn = slices.Compact(s.DependsOn)
		pins = append(pins, Pin{Ref: fmt.Sprintf("#%d", it.Number), Hash: s.hash(), Story: &s})
		nums = append(nums, it.Number)
	}
	slices.SortFunc(pins, func(a, b Pin) int { return a.Story.Number - b.Story.Number })
	slices.Sort(nums)
	set, _ := json.Marshal(nums)
	return append(pins, Pin{Ref: fmt.Sprintf("#%d stories", f), Hash: sum(set)})
}

// FilePin pins a ledger file by the SHA-256 of its bytes.
func FilePin(path string, b []byte) Pin { return Pin{Ref: path, Hash: sum(b)} }

// ChartPin pins a chart note on Feature f by the SHA-256 of its comment's body.
func ChartPin(f int, kind, body string) Pin {
	return Pin{Ref: fmt.Sprintf("#%d chart kind=%s", f, kind), Hash: sum([]byte(body))}
}

// EarlyBuildPin pins an open PR of an early-build story by its head SHA.
func EarlyBuildPin(story, pr int, head string) Pin {
	return Pin{Ref: fmt.Sprintf("#%d PR #%d", story, pr), Hash: head}
}
