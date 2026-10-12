package policy

import (
	"strings"
	"testing"
)

// Questions about another person's records and requests for advice are
// refused before any retrieval, with a message that says why; ordinary plan
// questions pass.
func TestScreenRefuses(t *testing.T) {
	for q, want := range map[string]string{
		"What is my colleague's claim status?":          ReasonPersonal,
		"Can you show my manager's salary?":             ReasonPersonal,
		"What did her prescription cost?":               ReasonPersonal,
		"Should I take ibuprofen for my back?":          ReasonAdvice,
		"Which plan is best for my family?":             ReasonAdvice,
		"   ":                                           ReasonEmpty,
		strings.Repeat("deductible ", MaxQuestion/10+1): ReasonTooLong,
	} {
		v := Screen(q)
		if v.Outcome != Refuse || v.Reason != want || v.Message == "" {
			t.Errorf("%.40q: %+v, want refuse %s", q, v, want)
		}
	}
	if v := Screen("What is my colleague's claim status?"); !strings.Contains(v.Message, "another person's records") {
		t.Errorf("personal-data message: %q", v.Message)
	}
	for _, q := range []string{"What is the dental annual deductible?", "Does my spouse count for the family deductible?", "How do I file a claim?",
		"What is my HSA balance?", "Does Example Co's 401(k) match my contributions?", "How many weeks of parental leave do I get?"} {
		if v := Screen(q); v.Outcome != Answer || v.Reason != "" {
			t.Errorf("%q: %+v, want answer", q, v)
		}
	}
}

// Ground: an answer needs a citation and at least MinScore of the question's terms.
func TestGroundThreshold(t *testing.T) {
	for _, tc := range []struct {
		score float64
		cites int
		want  string
	}{{0.5, 1, Answer}, {1, 1, Answer}, {0.49, 1, Refer}, {1, 0, Refer}} {
		if v := Ground(tc.score, tc.cites); v.Outcome != tc.want {
			t.Errorf("Ground(%v, %d) = %+v, want %s", tc.score, tc.cites, v, tc.want)
		}
	}
}
