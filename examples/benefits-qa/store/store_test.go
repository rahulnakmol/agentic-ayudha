package store

import (
	"errors"
	"strings"
	"testing"
)

// The embedded seed loads: synthetic plan text for Example Co, plan year
// 2026, every passage with a unique ID, a plan, a section, and text.
func TestLoadSeed(t *testing.T) {
	s, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if s.Employer() != "Example Co" || s.PlanYear() != 2026 {
		t.Errorf("seed describes %q %d", s.Employer(), s.PlanYear())
	}
	ps := s.Passages()
	if len(ps) != 16 {
		t.Errorf("%d passages, want 16", len(ps))
	}
	seen := map[string]bool{}
	for _, p := range ps {
		if seen[p.ID] || p.Plan == "" || p.Section == "" || p.Text == "" {
			t.Errorf("passage %+v", p)
		}
		seen[p.ID] = true
	}
	p, ok := s.Get("dental-deductible")
	if !ok || p.Plan != "Dental" || !strings.Contains(p.Text, "50 dollars per person") {
		t.Errorf("dental-deductible: %v %+v", ok, p)
	}
	if _, ok := s.Get("no-such-passage"); ok {
		t.Error("an unknown ID was found")
	}
	ps[0].Text = "changed"
	if q, _ := s.Get(ps[0].ID); q.Text == "changed" {
		t.Error("Passages exposes the store's own slice")
	}
}

// A seed that is not synthetic, has a duplicate or malformed ID, an empty
// field, or a personal email address is refused, naming the problem.
func TestParseRefuses(t *testing.T) {
	ok := `{"id":"a-b","plan":"P","section":"S","text":"Ask hr@example.com."}`
	for name, tc := range map[string]struct{ seed, want string }{
		"not json":      {`{`, "unexpected end"},
		"version":       {`{"version":2,"synthetic":true,"passages":[` + ok + `]}`, "version 2, want 1"},
		"not synthetic": {`{"version":1,"passages":[` + ok + `]}`, "not marked synthetic"},
		"no passages":   {`{"version":1,"synthetic":true}`, "no passages"},
		"bad id":        {`{"version":1,"synthetic":true,"passages":[{"id":"A B","plan":"P","section":"S","text":"t"}]}`, `id "A B" is not a lowercase slug`},
		"empty text":    {`{"version":1,"synthetic":true,"passages":[{"id":"x","plan":"P","section":"S","text":" "}]}`, "passage x: plan, section, and text are required"},
		"duplicate":     {`{"version":1,"synthetic":true,"passages":[` + ok + `,` + ok + `]}`, "passage a-b: duplicate id"},
		"personal email": {`{"version":1,"synthetic":true,"passages":[{"id":"x","plan":"P","section":"S","text":"Write to asha.rao@gmail.com"}]}`,
			"passage x: an email address outside example.com is personal data"},
		"email in a section": {`{"version":1,"synthetic":true,"passages":[{"id":"y","plan":"P","section":"Ask asha.rao@gmail.com","text":"t"}]}`,
			"passage y: an email address outside example.com is personal data"},
		"email in a plan": {`{"version":1,"synthetic":true,"passages":[{"id":"z","plan":"asha.rao@gmail.com","section":"S","text":"t"}]}`,
			"passage z: an email address outside example.com is personal data"},
	} {
		_, err := Parse([]byte(tc.seed))
		if !errors.Is(err, ErrSeed) || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: %v, want %q", name, err, tc.want)
		}
	}
	if _, err := Parse([]byte(`{"version":1,"synthetic":true,"passages":[` + ok + `]}`)); err != nil {
		t.Errorf("an example.com address is reserved, not personal: %v", err)
	}
}
