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
