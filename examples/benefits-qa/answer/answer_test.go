package answer

import (
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/examples/benefits-qa/policy"
	"github.com/rahulnakmol/agentic-ayudha/examples/benefits-qa/store"
)

func index(t *testing.T) *Index {
	t.Helper()
	s, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	return New(s.Passages())
}

// A question the plan documents answer is answered from the right passage,
// quoted in full with its plan and section, and policy grounds it.
func TestAnswerCitesPassage(t *testing.T) {
	ix := index(t)
	for q, want := range map[string]string{
		"What is the dental annual deductible?":            "dental-deductible",
		"How much is the medical deductible for a family?": "medical-deductible",
		"How many weeks of parental leave do I get?":       "parental-leave",
		"Does Example Co match my 401(k) contributions?":   "retirement-match",
	} {
		r := ix.Answer(q)
		if len(r.Citations) != 1 || r.Citations[0].PassageID != want {
			t.Errorf("%q cites %+v, want %s", q, r.Citations, want)
			continue
		}
		if v := policy.Ground(r.Score, len(r.Citations)); v.Outcome != policy.Answer {
			t.Errorf("%q: score %.2f is not grounded: %+v", q, r.Score, v)
		}
	}
	r := ix.Answer("What is the dental annual deductible?")
	if c := r.Citations[0]; c.Plan != "Dental" || c.Section != "Deductible" || !strings.Contains(r.Text, "50 dollars per person") || r.Score != 1 {
		t.Errorf("dental answer: %+v", r)
	}
}

// A question the plan documents do not answer is referred to HR: nothing
// matches, or the best passage holds too little of the question.
func TestGroundRefers(t *testing.T) {
	ix := index(t)
	for _, q := range []string{"Can I bring my dog to work?", "How do I reset my laptop password?", "How much does the company put in my HSA?", "the of and", "How many dollars?"} {
		r := ix.Answer(q)
		v := policy.Ground(r.Score, len(r.Citations))
		if v.Outcome != policy.Refer || v.Reason != policy.ReasonNotInPlans || !strings.Contains(v.Message, "HR specialist") {
			t.Errorf("%q (score %.2f, %d citations): %+v, want refer", q, r.Score, len(r.Citations), v)
		}
	}
	if r := ix.Answer("Can I bring my dog to work?"); r.Score != 0 || len(r.Citations) != 0 || r.Text != "" {
		t.Errorf("no match should be empty: %+v", r)
	}
	if r := New(nil).Answer("dental"); len(r.Citations) != 0 {
		t.Errorf("empty index: %+v", r)
	}
}

func TestTerms(t *testing.T) {
	if got := strings.Join(Terms("What are the Deductibles, and the glasses?"), " "); got != "deductible glass" {
		t.Errorf("terms: %q", got)
	}
}
