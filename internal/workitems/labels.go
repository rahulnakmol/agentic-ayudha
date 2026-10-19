package workitems

import (
	"slices"
	"strings"
)

// LabelDef is a label init creates (WP2.2).
type LabelDef struct {
	Name        string `json:"name"`
	Color       string `json:"color"` // hex without '#'
	Description string `json:"description"`
}

// Fixed labels outside the label-held fields.
const (
	LabelIntake        = "intake"                  // filed through an intake form; triage reads only these (WP2.9)
	LabelNeedsGrill    = "needs:grill"             // accepted Feedback waiting for grill to create its Feature
	LabelNeedsBug      = "needs:bug"               // an Incident waiting for respond to file its linked Bug (WP5.6)
	LabelHumanFallback = "identity:human-fallback" // written under the human identity because no bot exists (WP1.4)
	LabelProvisional   = "decision:provisional"    // V8: a reversible Decision's recommendation applied after its SLA
	LabelWaiver        = "decision:waiver"         // a floor-waiver Decision (decide --waiver, WP5.6); audit counts them
	LabelCorrection    = "aayu:correction"         // manual correction marker for the streak (WP5.7)
	LabelAudit         = "aayu:audit"              // a PR merged with no person's pass, waiting for a look after merge (#618)
	LabelAgentAuthored = "agent-authored"          // PRs opened by an agent lane (evolve, WP7.7)
	LabelScreenReader  = "aayudha:screen-reader"   // runs the screen-reader floor on a PR (macOS and Windows runners, WP4.7)
	LabelSecurity      = "aayudha:security"        // a Bug aayudha-security.yml filed for a finding on dev, rolling, or main
	LabelFixPrefix     = "fix:"                    // fix:<layer>, the highest layer that can fix a Feedback's correction for good (S-7.7d)
)

// Fix layers of a correction, highest first (ETHOS 10, the inverse pyramid):
// the codebase (a type, invariant, or boundary that makes the mistake
// impossible), a static check (a lint rule, test, or CI check that catches
// it), prose (skill or AGENTS.md guidance, for what needs judgement), and
// human review (what the environment surfaces for a person to look at).
const (
	FixCodebase = "codebase"
	FixCheck    = "check"
	FixProse    = "prose"
	FixReview   = "review"
)

// FixLayers lists the fix layers, highest first.
var FixLayers = []string{FixCodebase, FixCheck, FixProse, FixReview}

// ClassifyCommand records a Feedback issue's fix layer. An open Feedback issue
// without one is listed by `aayu doctor`, the home screen, and the
// session-start hook until it is classified (D-7.7e).
const ClassifyCommand = `aayu retro --classify <issue> --layer codebase|check|prose|review --reason "<why this layer>"`

// FixLayerOf is the fix layer a label set records, "" when none.
func FixLayerOf(labels []string) string {
	for _, l := range labels {
		if v, ok := strings.CutPrefix(l, LabelFixPrefix); ok && slices.Contains(FixLayers, v) {
			return v
		}
	}
	return ""
}

var labelColors = map[string]string{
	"status": "ededed", "priority": "b60205", "effort": "c2e0c6", "risk": "f9d0c4", "health": "0e8a16", "horizon": "d4c5f9",
}

// Labels is the full label set init creates: type:<id> for every type (the
// fallback when the owner has no issue types), the label-held fields, and the
// fixed labels. Names are unique.
func Labels() []LabelDef {
	var out []LabelDef
	for _, t := range Types {
		out = append(out, LabelDef{"type:" + t.ID, t.LabelColor, "Work-item type " + t.Name + " (label fallback for issue types)"})
	}
	for _, lf := range LabelFields {
		for _, v := range lf.Values {
			out = append(out, LabelDef{lf.Prefix + ":" + v, labelColors[lf.Prefix], lf.Name + ": " + v})
		}
	}
	return append(out,
		LabelDef{LabelIntake, "fef2c0", "Filed through an intake form; triage classifies it"},
		LabelDef{LabelNeedsGrill, "fef2c0", "Accepted Feedback waiting for grill to create its Feature"},
		LabelDef{LabelNeedsBug, "fef2c0", "Incident waiting for its linked Bug (respond files it)"},
		LabelDef{LabelHumanFallback, "e4e669", "Written under the human identity: no bot identity configured"},
		LabelDef{LabelProvisional, "d93f0b", "Recommendation applied provisionally after the Decision SLA (V8)"},
		LabelDef{LabelWaiver, "d93f0b", "Floor-waiver Decision (decide --waiver); never applied provisionally"},
		LabelDef{LabelCorrection, "b60205", "A human corrected an agent's merge (resets the streak)"},
		LabelDef{LabelAudit, "fbca04", "Merged with no person's review; waits for a look after merge in aayu inbox"},
		LabelDef{LabelAgentAuthored, "c5def5", "Opened by an agent lane"},
		LabelDef{LabelScreenReader, "5319e7", "Run the VoiceOver and NVDA screen-reader floor on this PR"},
		LabelDef{LabelSecurity, "b60205", "Filed by aayudha-security.yml for a finding on dev, rolling, or main"},
		LabelDef{LabelFixPrefix + FixCodebase, "0e8a16", "Fix layer: a type, invariant, or boundary makes the mistake impossible"},
		LabelDef{LabelFixPrefix + FixCheck, "1d76db", "Fix layer: a lint rule, test, or CI check catches the mistake"},
		LabelDef{LabelFixPrefix + FixProse, "fbca04", "Fix layer: skill or AGENTS.md guidance, for what needs judgement"},
		LabelDef{LabelFixPrefix + FixReview, "d93f0b", "Fix layer: change what the environment surfaces for human review"},
	)
}

// IssueTypeDef is a GitHub issue type init creates on an organization owner.
type IssueTypeDef struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Color       string `json:"color"`
}

// IssueTypes lists the issue types init creates on an organization (R1: personal
// repositories use the type:<id> labels instead, and the parser treats both as one).
func IssueTypes() []IssueTypeDef {
	out := make([]IssueTypeDef, len(Types))
	for i, t := range Types {
		out[i] = IssueTypeDef{t.Name, t.Description, t.Color}
	}
	return out
}
