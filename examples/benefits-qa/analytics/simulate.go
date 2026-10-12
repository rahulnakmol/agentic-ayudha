package analytics

import (
	"fmt"
	"io"
	// Synthetic, seed-reproducible randomness only: rand.NewPCG(cfg.Seed, …)
	// makes every export reproduce exactly, which is the feature under test.
	// Crypto-grade entropy is not a property this simulator needs, and
	// crypto/rand cannot be seeded, so it cannot provide it.
	"math/rand/v2" // nosemgrep: go.lang.security.audit.crypto.math_random.math-random-used
	"sort"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/examples/benefits-qa/ask"
	"github.com/rahulnakmol/agentic-ayudha/examples/benefits-qa/contracts"
	"github.com/rahulnakmol/agentic-ayudha/examples/benefits-qa/store"
)

// Channels a simulated employee asks through.
const (
	ChannelHRTicket   = "hr-ticket"   // before the product: every question is an HR ticket
	ChannelBenefitsQA = "benefits-qa" // the product's ask workflow; referred questions become HR tickets
)

// HR ticket model (synthetic): a ticket is answered with this probability,
// after a uniform delay of HRMinDays to HRMaxDays.
const (
	HRAnswerRate = 0.6
	HRMinDays    = 1
	HRMaxDays    = 10
	// Horizon is how long after a visit a simulated employee may still ask:
	// visits fall in the first Days-Horizon days, so every cohort can be
	// observed for 14 days inside the window.
	Horizon = 14
)

// Questions are the synthetic questions employees ask: plan questions the
// documents answer, questions they do not, and ones policy refuses.
var Questions = []string{
	"What is the dental annual deductible?",
	"How much is the medical deductible for a family?",
	"How many weeks of parental leave do I get?",
	"Does Example Co match my 401(k) contributions?",
	"When is open enrollment?",
	"What does vision cover for glasses?",
	"How many sick days do I get?",
	"What is the FSA limit?",
	"What is the out-of-pocket maximum?",
	"Is telehealth covered?",
	"Do you cover braces?",
	"Is there a gym reimbursement?",
	"Is preventive care covered?",
	"How much basic life insurance do I get?",
	"Can I bring my dog to work?",
	"How much does the company put in my HSA?",
	"What is the parking policy?",
	"What is my colleague's claim status?",
}

// Config is one simulation run.
type Config struct {
	Channel string
	Seed    uint64
	From    time.Time // window start, UTC midnight
	Days    int       // window length
	Users   int
}

// Stats summarise a run.
type Stats struct {
	Users, Questions, Events int
}

type clock struct{ t time.Time }

// Simulate runs the population through the channel and writes the export,
// sorted by time, then user, then event name, so a seed always gives the
// same bytes.
func Simulate(cfg Config, s *store.Store, w io.Writer) (Stats, error) {
	r, err := simulate(cfg, s)
	if err != nil {
		return r.st, err
	}
	events := append(append([]ask.Event{}, r.workflow...), r.hr...)
	sort.SliceStable(events, func(a, b int) bool {
		x, y := events[a], events[b]
		if !x.Time.Equal(y.Time) {
			return x.Time.Before(y.Time)
		}
		if x.User != y.User {
			return x.User < y.User
		}
		return x.Name < y.Name
	})
	out := NewAppInsights(w)
	for _, e := range events {
		if err := out.Record(e); err != nil {
			return r.st, err
		}
	}
	r.st.Events = len(events)
	return r.st, nil
}

// run is one simulation before it is merged and sorted: the events the ask
// workflow recorded, in the order it recorded them, and the HR specialists'
// answers to tickets.
type run struct {
	workflow, hr []ask.Event
	st           Stats
}

// Validate checks a configuration before anything runs or is written.
func (cfg Config) Validate() error {
	if cfg.Channel != ChannelHRTicket && cfg.Channel != ChannelBenefitsQA {
		return fmt.Errorf("unknown channel %q (want %s or %s)", cfg.Channel, ChannelHRTicket, ChannelBenefitsQA)
	}
	if cfg.Days <= Horizon || cfg.Users <= 0 {
		return fmt.Errorf("need more than %d days and at least one user", Horizon)
	}
	return nil
}

func simulate(cfg Config, s *store.Store) (run, error) {
	if err := cfg.Validate(); err != nil {
		return run{}, err
	}
	rng := rand.New(rand.NewPCG(cfg.Seed, cfg.Seed^0x5eed))
	r := run{st: Stats{Users: cfg.Users}}
	rec := recorderFunc(func(e ask.Event) error { r.workflow = append(r.workflow, e); return nil })
	clk := &clock{}
	svc := ask.New(s, rec, func() time.Time { return clk.t })
	ticket := func(user string, at time.Time) {
		if rng.Float64() < HRAnswerRate {
			d := HRMinDays + rng.IntN(HRMaxDays-HRMinDays+1)
			r.hr = append(r.hr, ask.Event{Name: ask.EventGrounded, User: user, Time: at.Add(time.Duration(d) * 24 * time.Hour)})
		}
	}
	for i := range cfg.Users {
		user := fmt.Sprintf("emp-%04d", i+1)
		visit := cfg.From.AddDate(0, 0, rng.IntN(cfg.Days-Horizon)).Add(time.Duration(8+rng.IntN(9)) * time.Hour)
		clk.t = visit
		_ = svc.Visit(user) // the in-memory recorder never fails
		for range 1 + rng.IntN(3) {
			at := visit.Add(time.Duration(rng.IntN(7*24)) * time.Hour)
			q := Questions[rng.IntN(len(Questions))]
			r.st.Questions++
			if cfg.Channel == ChannelHRTicket {
				ticket(user, at)
				continue
			}
			clk.t = at
			resp, _ := svc.Ask(user, q) // the in-memory recorder never fails
			if resp.Status == contracts.StatusReferred {
				ticket(user, at)
			}
		}
	}
	return r, nil
}

type recorderFunc func(ask.Event) error

func (f recorderFunc) Record(e ask.Event) error { return f(e) }
