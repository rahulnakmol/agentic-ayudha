package evals

import (
	"regexp"
	"strings"
	"testing"
)

// Story #401 (Feedback #329): every deterministic rule in an eval case (a
// case.toml scope rule, a session.toml require, forbid, or say_forbid rule)
// ships with a match, a negated, and a near-miss fixture, and behaves on each
// as its kind says. A rule that misfires on a negation, an ordinary use of a
// word, or a paraphrase fails here, not in a paid evaluation round.
func TestCaseRulesHaveFixtures(t *testing.T) {
	cases, err := LoadCases(root)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, c := range cases {
		rs, err := rules(c)
		if err != nil {
			t.Fatal(err)
		}
		nth := map[string]int{}
		for _, r := range rs {
			n++
			nth[r.kind]++
			for _, p := range r.fixtureProblems() {
				t.Errorf("%s %s rule %d (%s): %s", c.ID, r.kind, nth[r.kind], r.Why, p)
			}
		}
	}
	if n == 0 {
		t.Fatal("no rules found")
	}
	t.Logf("%d rules checked", n)

	// The check names the rule's missing kind and each misfire.
	r := Scope{Pattern: `(?i)\bcsv\b`, Why: "a format", Match: []string{"Export as CSV."}, Negated: []string{"The format is not CSV."}}
	r.re = regexp.MustCompile(r.Pattern)
	got := strings.Join(r.fixtureProblems(), "; ")
	if got != `fires on its negated fixture "The format is not CSV."; no near_miss fixture` {
		t.Errorf("problems: %s", got)
	}
	r.NegatedFires, r.NearMiss, r.Match = true, []string{"csvkit is a tool."}, []string{"Export the list."}
	if got := strings.Join(r.fixtureProblems(), "; "); got != `does not fire on its match fixture "Export the list."` {
		t.Errorf("problems: %s", got)
	}
}
