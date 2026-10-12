// Package ask is the benefits-qa example's question workflow (layer L03,
// docs/BUILD-PLAN.md WP8.1): screen the question (policy), retrieve the best
// plan passage (answer), ground it (policy), and shape the contract response,
// recording each step as a product event for analytics.
package ask

import (
	"time"

	"github.com/rahulnakmol/agentic-ayudha/examples/benefits-qa/answer"
	"github.com/rahulnakmol/agentic-ayudha/examples/benefits-qa/contracts"
	"github.com/rahulnakmol/agentic-ayudha/examples/benefits-qa/policy"
	"github.com/rahulnakmol/agentic-ayudha/examples/benefits-qa/store"
)

// Event names the workflow records.
const (
	EventVisit    = "benefits_visit"  // someone opened the benefits page
	EventAsked    = "question_asked"  // a question reached the workflow
	EventGrounded = "answer_grounded" // it was answered from a cited passage
	EventReferred = "answer_referred" // it went to an HR specialist
	EventRefused  = "answer_refused"  // policy refused it
)

// Event is one product event. User is pseudonymous; question text is never
// recorded.
type Event struct {
	Name string
	User string
	Time time.Time
}

// Recorder receives product events.
type Recorder interface {
	Record(Event) error
}

// Discard records nothing.
type Discard struct{}

// Record implements Recorder.
func (Discard) Record(Event) error { return nil }

// Service answers questions.
type Service struct {
	Index    *answer.Index
	Recorder Recorder
	Now      func() time.Time
}

// New builds the service over a store.
func New(s *store.Store, rec Recorder, now func() time.Time) *Service {
	if rec == nil {
		rec = Discard{}
	}
	if now == nil {
		now = time.Now
	}
	return &Service{Index: answer.New(s.Passages()), Recorder: rec, Now: now}
}

// Visit records that a user opened the benefits page.
func (s *Service) Visit(user string) error {
	return s.Recorder.Record(Event{Name: EventVisit, User: user, Time: s.Now().UTC()})
}

// Ask runs the workflow for one question. The response always satisfies
// contracts.AskResponse.Check; an error is only a recorder failure, and the
// response is still valid then.
func (s *Service) Ask(user, question string) (contracts.AskResponse, error) {
	rec := func(name string) error { return s.Recorder.Record(Event{Name: name, User: user, Time: s.Now().UTC()}) }
	err := rec(EventAsked)
	keep := func(e error) {
		if err == nil {
			err = e
		}
	}
	r, v := s.Index.Respond(question) // policy screens before retrieval and grounds after it
	switch v.Outcome {
	case policy.Refuse:
		keep(rec(EventRefused))
		return contracts.AskResponse{Status: contracts.StatusRefused, Citations: []contracts.Citation{}, Reason: v.Reason, Message: v.Message}, err
	case policy.Refer:
		keep(rec(EventReferred))
		return contracts.AskResponse{Status: contracts.StatusReferred, Citations: []contracts.Citation{}, Reason: v.Reason, Message: v.Message}, err
	}
	keep(rec(EventGrounded))
	out := contracts.AskResponse{Status: contracts.StatusAnswered, Answer: r.Text}
	for _, c := range r.Citations {
		out.Citations = append(out.Citations, contracts.Citation{PassageID: c.PassageID, Plan: c.Plan, Section: c.Section})
	}
	return out, err
}
