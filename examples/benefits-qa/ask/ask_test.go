package ask

import (
	"errors"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/examples/benefits-qa/contracts"
	"github.com/rahulnakmol/agentic-ayudha/examples/benefits-qa/store"
)

type memory struct {
	events []Event
	err    error
}

func (m *memory) Record(e Event) error { m.events = append(m.events, e); return m.err }

func service(t *testing.T, rec Recorder) *Service {
	t.Helper()
	s, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 9, 14, 9, 0, 0, 0, time.FixedZone("IST", 19800))
	return New(s, rec, func() time.Time { return at })
}

// The workflow records question_asked, then exactly one outcome event, for
// the asker's pseudonymous ID, at the service clock in UTC; every response
// keeps the contract.
func TestServiceRecordsEvents(t *testing.T) {
	m := &memory{}
	svc := service(t, m)
	for q, want := range map[string]struct{ status, event string }{
		"What is the dental annual deductible?": {contracts.StatusAnswered, EventGrounded},
		"Can I bring my dog to work?":           {contracts.StatusReferred, EventReferred},
		"What is my colleague's claim status?":  {contracts.StatusRefused, EventRefused},
	} {
		m.events = nil
		r, err := svc.Ask("emp-0007", q)
		if err != nil || r.Status != want.status || r.Check() != nil {
			t.Errorf("%q: %+v %v (check %v)", q, r, err, r.Check())
		}
		if len(m.events) != 2 || m.events[0].Name != EventAsked || m.events[1].Name != want.event {
			t.Errorf("%q: events %+v", q, m.events)
			continue
		}
		for _, e := range m.events {
			if e.User != "emp-0007" || e.Time != time.Date(2026, 9, 14, 3, 30, 0, 0, time.UTC) || e.Time.Location() != time.UTC {
				t.Errorf("%q: event %+v", q, e)
			}
		}
	}
	m.events = nil
	if err := svc.Visit("emp-0008"); err != nil || len(m.events) != 1 || m.events[0].Name != EventVisit {
		t.Errorf("visit: %v %+v", err, m.events)
	}
}

// A recorder failure is reported, but the answer is still given.
func TestRecorderFailureKeepsAnswer(t *testing.T) {
	boom := errors.New("disk full")
	svc := service(t, &memory{err: boom})
	r, err := svc.Ask("emp-1", "What is the dental annual deductible?")
	if !errors.Is(err, boom) || r.Status != contracts.StatusAnswered {
		t.Errorf("%+v %v", r, err)
	}
	if r, err := New(mustLoad(t), nil, nil).Ask("", "How many sick days do I get?"); err != nil || r.Status != contracts.StatusAnswered {
		t.Errorf("defaults: %+v %v", r, err)
	}
}

func mustLoad(t *testing.T) *store.Store {
	s, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	return s
}
