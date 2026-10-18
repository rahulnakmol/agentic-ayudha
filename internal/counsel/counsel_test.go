package counsel

import (
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

// Counsel never claims a review or an approval, but it may say it is not one, and it may report what a cited record
// shows (Story #478).
func TestReviewClaims(t *testing.T) {
	for line, claims := range map[string]bool{
		"I\napprove this PR (#2).":                     true,
		"- One point (#2).\n- Approved. Ship it.":      true,
		"The PR was reviewed.":                         true,
		"The change was approved yesterday.":           true,
		"Approval was granted (#2).":                   true,
		"Review complete; sign-off given (#2).":        true,
		"It is decided: ship (#2).":                    true,
		"Ask for approval with `aayu inspect 2`.":      false,
		"Has the review been done?":                    false,
		"LGTM.":                                        true,
		"I have reviewed the diff and it is fine.":     true,
		"We approve this change.":                      true,
		"The PR has been reviewed.":                    true,
		"This counsel is not a review or an approval.": false,
		"The PR has been reviewed and may merge (#2).": true,
		"The record shows @lead's review (#12).":       false,
		"Ask for a review with `aayu inspect 12`.":     false,
		"Who approves the waiver?":                     false,
	} {
		got := false
		for _, f := range CheckStructure(line) {
			got = got || strings.HasPrefix(f, "claims a review")
		}
		if got != claims {
			t.Errorf("%q: claims a review = %v, want %v", line, got, claims)
		}
	}
}

// The label says different-family only when the builder's family is known and the writer's differs from it and
// from every candidate (Decision #469); a self-reported builder or an unreported writer is same-family.
func TestLabel(t *testing.T) {
	for _, tt := range []struct {
		model   string
		onPR    bool
		builder verbs.Family
		cands   []verbs.Family
		want    string
	}{
		{"gpt-6.1-sol", true, verbs.FamilyAnthropic, nil, "different-family counsel: gpt-6.1-sol (openai) vs builder anthropic"},
		{"claude-opus-5-5", true, verbs.FamilyAnthropic, nil, "same-family counsel: claude-opus-5-5 (anthropic), builder anthropic"},
		{"gpt-6.1-sol", true, "", []verbs.Family{verbs.FamilyAnthropic}, "same-family counsel: gpt-6.1-sol (openai), builder family unknown"},
		{"", true, verbs.FamilyAnthropic, nil, "same-family counsel: model not reported, builder anthropic"},
		{"gpt-6.1-sol", false, "", nil, "written by gpt-6.1-sol (openai)"},
	} {
		if got := Label(tt.model, tt.onPR, tt.builder, tt.cands); got != tt.want {
			t.Errorf("Label(%q, %v, %q, %v) = %q, want %q", tt.model, tt.onPR, tt.builder, tt.cands, got, tt.want)
		}
	}
}

// A counsel's parts come in order, none empty, ending with the one question, within MaxWords.
func TestCheckStructure(t *testing.T) {
	good := Template("Three-hat line")
	good = strings.Replace(good, "What this is really about", "A two-way door: what this is really about", 1)
	good = strings.Replace(good, "The concrete next aayu command", "Run `aayu decide 5`: the concrete next command", 1)
	good = strings.Replace(good, "The one question the person must answer, in one line.", "Is this the call?", 1)
	if f := CheckStructure(good); len(f) != 0 {
		t.Fatalf("the filled template: %v", f)
	}
	for name, tt := range map[string]struct{ md, want string }{
		"out of order": {strings.NewReplacer("## Recommendation", "## What not to do", "## What not to do", "## Recommendation").Replace(good),
			`part "What not to do" is out of order`},
		"empty part":               {strings.Replace(good, "The facts or answers that would change the recommendation.", "", 1), `part "What would change it" is empty`},
		"question not last":        {good + "## Notes\n\nMore.\n", `must end with "## Your question"`},
		"two-line question":        {strings.Replace(good, "Is this the call?", "Is this the call?\nOr that?", 1), "not one question on one line"},
		"too long":                 {good + strings.Repeat("word ", MaxWords), "words: counsel is read in a few minutes"},
		"two recommendation parts": {strings.Replace(good, "## What not to do", "## Recommendation\n\nAlso B.\n\n## What not to do", 1), `part "Recommendation" appears 2 times`},
		"extra part":               {strings.Replace(good, "## What not to do", "## Alternative\n\nChoose B (#2).\n\n## What not to do", 1), `part "Alternative" is not a part of a counsel`},
		"two questions":            {strings.Replace(good, "Is this the call?", "Ship now? Or wait?", 1), "not one question on one line"},
		"another command":          {strings.Replace(good, "Run `aayu decide 5`:", "Run `aayu decide 5`, then `gh pr merge 5`:", 1), "`gh pr merge 5` is not an aayu command"},
		"fenced parts":             {strings.Replace(good, "## The call", "```\n## The call", 1) + "```\n", "has a code block"},
		"two commands":             {strings.Replace(good, "Run `aayu decide 5`:", "Run `aayu inspect 2`, then `aayu ship 2`:", 1), "gives 2 commands"},
		"also choose":              {strings.Replace(good, "One recommendation and why", "Choose A (#2). Also choose B (#2); why", 1), "make one recommendation"},
		"two in a paragraph":       {strings.Replace(good, "One recommendation and why", "I recommend A. I also recommend B; why", 1), "make one recommendation"},
	} {
		if f := strings.Join(CheckStructure(tt.md), "\n"); !strings.Contains(f, tt.want) {
			t.Errorf("%s: want %q in:\n%s", name, tt.want, f)
		}
	}
}
