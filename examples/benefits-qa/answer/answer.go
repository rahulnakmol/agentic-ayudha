// Package answer finds the plan passage that answers a benefits question
// (layer L05, docs/BUILD-PLAN.md WP8.1). It is deterministic retrieval, not a
// language model: passages are ranked with BM25 over their plan, section, and
// text, and the answer is the best passage quoted with its citation. Its score
// is the share of the question's terms the passage holds that are distinctive
// (in at most half of the passages), which policy uses to decide whether the
// answer is grounded.
package answer

import (
	"math"
	"sort"
	"strings"
	"unicode"

	"github.com/rahulnakmol/agentic-ayudha/examples/benefits-qa/policy"
	"github.com/rahulnakmol/agentic-ayudha/examples/benefits-qa/store"
)

// BM25 parameters (the usual defaults).
const (
	k1 = 1.2
	b  = 0.75
)

// Citation names the passage an answer quotes.
type Citation struct {
	PassageID string `json:"passage_id"`
	Plan      string `json:"plan"`
	Section   string `json:"section"`
}

// Result is the best answer to a question.
type Result struct {
	Text      string
	Citations []Citation
	// Score is the share (0 to 1) of the question's distinct terms that the
	// cited passage contains and that are distinctive (in at most half of the
	// passages); 0 when nothing matched.
	Score float64
}

type doc struct {
	p     store.Passage
	terms map[string]int
	n     int
}

// Index ranks passages for questions.
type Index struct {
	docs []doc
	df   map[string]int
	avg  float64

	retrievals int // Answer calls, so tests can prove Respond screens first
}

// New indexes the passages.
func New(passages []store.Passage) *Index {
	ix := &Index{df: map[string]int{}}
	total := 0
	for _, p := range passages {
		toks := Terms(p.Plan + " " + p.Section + " " + p.Text)
		d := doc{p: p, terms: map[string]int{}, n: len(toks)}
		for _, t := range toks {
			if d.terms[t] == 0 {
				ix.df[t]++
			}
			d.terms[t]++
		}
		total += len(toks)
		ix.docs = append(ix.docs, d)
	}
	if len(ix.docs) > 0 {
		ix.avg = float64(total) / float64(len(ix.docs))
	}
	return ix
}

// Answer returns the best passage for the question; a question with no term
// in any passage gets an empty Result.
func (ix *Index) Answer(question string) Result {
	ix.retrievals++
	q := unique(Terms(question))
	if len(q) == 0 {
		return Result{}
	}
	type scored struct {
		i     int
		score float64
	}
	var ranked []scored
	for i, d := range ix.docs {
		s := 0.0
		for _, t := range q {
			f := float64(d.terms[t])
			if f == 0 {
				continue
			}
			n := float64(len(ix.docs))
			idf := math.Log(1 + (n-float64(ix.df[t])+0.5)/(float64(ix.df[t])+0.5))
			s += idf * f * (k1 + 1) / (f + k1*(1-b+b*float64(d.n)/ix.avg))
		}
		if s > 0 {
			ranked = append(ranked, scored{i, s})
		}
	}
	if len(ranked) == 0 {
		return Result{}
	}
	sort.SliceStable(ranked, func(a, c int) bool {
		if ranked[a].score != ranked[c].score {
			return ranked[a].score > ranked[c].score
		}
		return ix.docs[ranked[a].i].p.ID < ix.docs[ranked[c].i].p.ID
	})
	best := ix.docs[ranked[0].i]
	hit := 0
	for _, t := range q {
		// Only a distinctive term counts towards the score: one that is in at
		// most half of the passages. "plan" alone grounds nothing.
		if best.terms[t] > 0 && 2*ix.df[t] <= len(ix.docs) {
			hit++
		}
	}
	return Result{
		Text:      best.p.Text,
		Citations: []Citation{{PassageID: best.p.ID, Plan: best.p.Plan, Section: best.p.Section}},
		Score:     float64(hit) / float64(len(q)),
	}
}

// Respond is the one path from a question to what may be shown: policy
// screens the question before any retrieval (a refused question is never
// looked up), then the best passage is retrieved and policy grounds it. Only
// an answered question returns a passage; a refused or referred one returns
// an empty Result with the verdict that says why.
func (ix *Index) Respond(question string) (Result, policy.Verdict) {
	if v := policy.Screen(question); v.Outcome == policy.Refuse {
		return Result{}, v
	}
	r := ix.Answer(question)
	if v := policy.Ground(r.Score, len(r.Citations)); v.Outcome != policy.Answer {
		return Result{}, v
	}
	return r, policy.Verdict{Outcome: policy.Answer}
}

// stop words carry no meaning for retrieval.
var stop = map[string]bool{}

func init() {
	for _, w := range strings.Fields(`a about an and any are as at be can do does for from get have how i if in is it
		me much my of on or our the there this to what when where which who will with you your many am we`) {
		stop[w] = true
	}
}

// Terms lowercases text, splits it into words, drops stop words, and folds a
// trailing plural "s" or "es" so "deductibles" matches "deductible" and
// "glasses" matches "glass".
func Terms(text string) []string {
	var out []string
	for _, w := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		if stop[w] {
			continue
		}
		switch {
		case strings.HasSuffix(w, "sses"):
			w = strings.TrimSuffix(w, "es")
		case len(w) > 3 && strings.HasSuffix(w, "s") && !strings.HasSuffix(w, "ss"):
			w = strings.TrimSuffix(w, "s")
		}
		out = append(out, w)
	}
	return out
}

func unique(ts []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, t := range ts {
		if !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	return out
}
