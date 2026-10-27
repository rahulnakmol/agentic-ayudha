package ready

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Approve to build (Feature #877, Decision #903, core design S8): one
// approval per Feature agrees its baselines, its budget, and every story
// listed then. Only a person gives it (`aayu ready <feature>`), through
// GitHub's device flow, and the record lives on the ledger at
// specs/<f>/approved.json. A later approval replaces the file with the next
// version; the ledger's history keeps the earlier ones.

// ApprovalSchema is the schema of an Approve to build record.
const ApprovalSchema = "aayudha.feature.approved/v1"

// ApprovalPath is where a Feature's Approve to build record lives on the ledger.
func ApprovalPath(f int) string { return fmt.Sprintf("specs/%d/approved.json", f) }

// Approval is a person's Approve to build of a Feature.
type Approval struct {
	Schema     string             `json:"schema"`
	Feature    int                `json:"feature"`
	Version    int                `json:"version"` // from 1
	By         string             `json:"by"`
	At         string             `json:"at"`
	Presence   Presence           `json:"presence"`
	Baselines  []ApprovedBaseline `json:"baselines"`
	Stories    []ApprovedStory    `json:"stories"`
	Budget     Budget             `json:"budget"`
	Dismissals []Dismissal        `json:"dismissals"`
	// Agents is how many stories the Feature's mission runs at once (aayu ready <f> --agents N); 0: [build] agents.
	Agents int `json:"agents,omitempty"`
	// Parent is the mission's Kanha, which a person's Start began (Story #1004); nil until then.
	Parent *Parent `json:"parent,omitempty"`
}

// Parent is the session that coordinates approved work: a Feature mission's
// Kanha, started at a person's Start on the tool the plan records. Scope and
// Key name the work (feature and its number; Kala, #911, takes repo). It is
// information, never authority (#871).
type Parent struct {
	Scope     string `json:"scope"`
	Key       string `json:"key"`
	Host      string `json:"host"`              // the tool: amp, claude, cursor, codex, copilot, or opencode
	Session   string `json:"session,omitempty"` // the thread, session, or cloud agent ID
	URL       string `json:"url,omitempty"`
	Mode      string `json:"mode"`              // cloud, or local on the starter's machine
	Machine   string `json:"machine,omitempty"` // login@machine of a local Kanha
	StartedBy string `json:"started_by"`
	StartedAt string `json:"started_at"`
	LastCheck string `json:"last_check,omitempty"` // a local Kanha: when its last check gave it the events before it (Story #1012)
}

// Presence is how the person was proven present: the device-flow approval
// and its presence.approve record in the actions log, which audit matches.
type Presence struct {
	Login         string `json:"login"`
	Via           string `json:"via"`
	Permission    string `json:"permission"`
	ActionsRecord string `json:"actions_record"` // the hash of the presence.approve record
}

// ApprovedBaseline is one part's approved baseline the approval agrees.
type ApprovedBaseline struct {
	Part    string `json:"part"`
	Version int    `json:"version"`
	Hash    string `json:"hash"`
}

// ApprovedStory is one story the approval covers, at its pinned fields.
type ApprovedStory struct {
	Number             int    `json:"number"`
	Title              string `json:"title"`
	Effort             string `json:"effort"`
	CompletionGoalHash string `json:"completion_goal_hash"`
}

// Budget is the Feature's cap: [budget] per_story_usd for each story.
type Budget struct {
	PerStoryUSD float64 `json:"per_story_usd"`
	Stories     int     `json:"stories"`
	TotalUSD    float64 `json:"total_usd"`
}

// Dismissal is a review gap dismissed with a reason, which the approval ratifies.
type Dismissal struct {
	Finding string `json:"finding"` // <part>-v<k>/<seat>/<n>
	Reason  string `json:"reason"`
	Words   string `json:"words"` // the person's own words when they dismissed it
}

// Critique is where the seats' reviews of a Feature's baselines stand, as
// #876's critique.State reports them.
type Critique struct {
	Reviews   int         `json:"reviews"`  // reviews asked for
	In        int         `json:"in"`       // reviews in
	AskedAt   time.Time   `json:"asked_at"` // when they were asked for; the wait runs from it
	Open      []string    `json:"open"`     // open gaps, by finding ID
	Dismissed []Dismissal `json:"dismissed"`
}

// Agreed reports whether a and b agree the same Feature: the same baselines,
// stories, budget, dismissals, and agents, whoever approved them and when.
func (a Approval) Agreed(b Approval) bool {
	agreed := func(x Approval) string {
		j, _ := json.Marshal([]any{x.Baselines, x.Stories, x.Budget, x.Dismissals, x.Agents})
		return string(j)
	}
	return agreed(a) == agreed(b)
}

// Name is the part as Approve to build names it: intent, spec, technical
// design, UX design.
func Name(part string) string {
	switch part {
	case "tech":
		return "technical design"
	case "ux":
		return "UX design"
	}
	return part
}

// Effect is the line a person approves, shown before the device code.
func Effect(a Approval) string {
	nums := make([]int, len(a.Stories))
	for i, s := range a.Stories {
		nums[i] = s.Number
	}
	parts := make([]string, len(a.Baselines))
	for i, b := range a.Baselines {
		parts[i] = fmt.Sprintf("%s v%d", Name(b.Part), b.Version)
	}
	return fmt.Sprintf("Approves #%d to build: %d stories (%s), %s, budget up to %s (%d x %s, [budget] per_story_usd)",
		a.Feature, len(a.Stories), Numbers(nums), strings.Join(parts, ", "), USD(a.Budget.TotalUSD), a.Budget.Stories, USD(a.Budget.PerStoryUSD))
}

// ChangeLine is one change of a baseline version, in a person's effect.
func ChangeLine(part string, v int, c Change) string {
	n := Name(part)
	return fmt.Sprintf("%s v%d changed %s: %s · why it matters: %s · the need behind it: %s", strings.ToUpper(n[:1])+n[1:], v, c.Ref, c.What, c.Why, c.Need)
}

// Ratified is the effect line of the dismissals an approval ratifies.
func Ratified(ds []Dismissal) string {
	items := make([]string, len(ds))
	for i, d := range ds {
		items[i] = fmt.Sprintf("%s (%s)", d.Finding, d.Reason)
	}
	return fmt.Sprintf("%d %s ratified: %s", len(ds), plural(len(ds), "dismissal"), strings.Join(items, ", "))
}

// OpenGaps says the open gaps of the first part version that has any:
// "technical design v1 has 1 open gap (tech-v1/quality-architect/1)".
func OpenGaps(open []string) string {
	first, _, _ := strings.Cut(open[0], "/")
	var ids []string
	for _, id := range open {
		if p, _, _ := strings.Cut(id, "/"); p == first {
			ids = append(ids, id)
		}
	}
	what := first
	if part, v, _, ok := Finding(open[0]); ok {
		what = fmt.Sprintf("%s v%d", Name(part), v)
	}
	return fmt.Sprintf("%s has %d open %s (%s)", what, len(ids), plural(len(ids), "gap"), strings.Join(ids, ", "))
}

func plural(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}

// Numbers lists issue numbers in order, a run of consecutive ones as a
// range: "#851-#856", "#851, #853-#854".
func Numbers(nums []int) string {
	var out []string
	for i := 0; i < len(nums); {
		j := i
		for j+1 < len(nums) && nums[j+1] == nums[j]+1 {
			j++
		}
		if j > i {
			out = append(out, fmt.Sprintf("#%d-#%d", nums[i], nums[j]))
		} else {
			out = append(out, fmt.Sprintf("#%d", nums[i]))
		}
		i = j + 1
	}
	return strings.Join(out, ", ")
}

// USD is an amount of dollars: "$360", or "$62.50" when it has cents.
func USD(v float64) string {
	if v == float64(int64(v)) {
		return fmt.Sprintf("$%d", int64(v))
	}
	return fmt.Sprintf("$%.2f", v)
}
