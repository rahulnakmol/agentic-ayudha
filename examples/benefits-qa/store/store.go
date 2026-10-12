// Package store holds the benefit plan passages the benefits-qa example
// answers from (layer L07, docs/BUILD-PLAN.md WP8.1). The data is synthetic
// plan text for the fictional employer Example Co (V3), embedded in the
// binary from seed/plans.json and checked when it loads.
package store

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// SchemaVersion is the seed format this package reads.
const SchemaVersion = 1

//go:embed seed/plans.json
var seedJSON []byte

// Passage is one section of a benefit plan document.
type Passage struct {
	ID      string `json:"id"`
	Plan    string `json:"plan"`
	Section string `json:"section"`
	Text    string `json:"text"`
}

// Seed is the seed file.
type Seed struct {
	Version   int       `json:"version"`
	Synthetic bool      `json:"synthetic"`
	Employer  string    `json:"employer"`
	PlanYear  int       `json:"plan_year"`
	Note      string    `json:"note"`
	Passages  []Passage `json:"passages"`
}

// Store is a loaded, checked seed. It is read-only.
type Store struct {
	seed Seed
	byID map[string]int
}

var (
	idRE = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	// emailRE finds anything shaped like an email address: a local part, "@",
	// and a domain name or an address literal ([192.0.2.1], [IPv6:…]).
	emailRE = regexp.MustCompile(`[^\s@<>()"',;]+@(\[[^\]\s]*\]|[A-Za-z0-9-]+(?:\.[A-Za-z0-9-]+)*)`)
	// reservedRE is example.com and its subdomains (RFC 2606), the only domain synthetic text may use.
	reservedRE = regexp.MustCompile(`(?i)^([a-z0-9-]+\.)*example\.com$`)
)

// ErrSeed is wrapped by every seed problem.
var ErrSeed = errors.New("benefits-qa seed")

// Load reads the embedded seed.
func Load() (*Store, error) { return Parse(seedJSON) }

// Parse reads and checks a seed: the version, the synthetic flag, every
// passage (a unique slug ID, a plan, a section, and text), and that no text
// anywhere in it, metadata included, holds an email address outside the
// reserved example.com domain.
func Parse(b []byte) (*Store, error) {
	var s Seed
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSeed, err)
	}
	if s.Version != SchemaVersion {
		return nil, fmt.Errorf("%w: version %d, want %d", ErrSeed, s.Version, SchemaVersion)
	}
	if !s.Synthetic {
		return nil, fmt.Errorf("%w: not marked synthetic; proof captures use synthetic data only (V3)", ErrSeed)
	}
	if len(s.Passages) == 0 {
		return nil, fmt.Errorf("%w: no passages", ErrSeed)
	}
	if personal(s.Employer, s.Note) {
		return nil, fmt.Errorf("%w: seed metadata: an email address outside example.com is personal data", ErrSeed)
	}
	st := &Store{seed: s, byID: map[string]int{}}
	for i, p := range s.Passages {
		switch {
		case !idRE.MatchString(p.ID):
			return nil, fmt.Errorf("%w: passage %d: id %q is not a lowercase slug", ErrSeed, i+1, p.ID)
		case strings.TrimSpace(p.Plan) == "" || strings.TrimSpace(p.Section) == "" || strings.TrimSpace(p.Text) == "":
			return nil, fmt.Errorf("%w: passage %s: plan, section, and text are required", ErrSeed, p.ID)
		}
		if _, dup := st.byID[p.ID]; dup {
			return nil, fmt.Errorf("%w: passage %s: duplicate id", ErrSeed, p.ID)
		}
		if personal(p.Plan, p.Section, p.Text) {
			return nil, fmt.Errorf("%w: passage %s: an email address outside example.com is personal data", ErrSeed, p.ID)
		}
		st.byID[p.ID] = i
	}
	return st, nil
}

// personal reports whether any of the texts holds an email address whose
// domain is not example.com or one of its subdomains; an address literal is
// never reserved.
func personal(texts ...string) bool {
	for _, t := range texts {
		for _, m := range emailRE.FindAllStringSubmatch(t, -1) {
			if !reservedRE.MatchString(strings.TrimSuffix(m[1], ".")) {
				return true
			}
		}
	}
	return false
}

// Passages returns every passage in seed order.
func (s *Store) Passages() []Passage { return append([]Passage(nil), s.seed.Passages...) }

// Get returns the passage with the ID.
func (s *Store) Get(id string) (Passage, bool) {
	i, ok := s.byID[id]
	if !ok {
		return Passage{}, false
	}
	return s.seed.Passages[i], true
}

// Employer and PlanYear describe the seed.
func (s *Store) Employer() string { return s.seed.Employer }

// PlanYear is the plan year the passages describe.
func (s *Store) PlanYear() int { return s.seed.PlanYear }
