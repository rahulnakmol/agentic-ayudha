package counsel

import (
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

// Counsel never claims a review or an approval, but it may say it is not one, and it may report a cited record of
// one (Story #478).
func TestReviewClaims(t *testing.T) {
	for line, claims := range map[string]bool{
		"LGTM.": true,
		"I have reviewed the diff and it is fine.":     true,
		"We approve this change.":                      true,
		"The PR has been reviewed.":                    true,
		"This counsel is not a review or an approval.": false,
		"The PR has been reviewed by @lead (#12).":     false,
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
		"empty part":         {strings.Replace(good, "The facts or answers that would change the recommendation.", "", 1), `part "What would change it" is empty`},
		"question not last":  {good + "## Notes\n\nMore.\n", `must end with "## Your question"`},
		"two-line question":  {strings.Replace(good, "Is this the call?", "Is this the call?\nOr that?", 1), "not one line ending in a question mark"},
		"too long":           {good + strings.Repeat("word ", MaxWords), "words: counsel is read in a few minutes"},
		"two in a paragraph": {strings.Replace(good, "One recommendation and why", "I recommend A. I also recommend B; why", 1), "make one recommendation"},
	} {
		if f := strings.Join(CheckStructure(tt.md), "\n"); !strings.Contains(f, tt.want) {
			t.Errorf("%s: want %q in:\n%s", name, tt.want, f)
		}
	}
}
