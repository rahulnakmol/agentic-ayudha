package production

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// SLOPath is the product's SLO definitions; StatusPath is the error-budget
// status `aayu slo --record` writes on the ledger, and release reads.
const (
	SLOPath    = "ops/slos.toml"
	StatusPath = "slo/status.json"
	// StaleAfter is how old a recorded status may be before release treats
	// it as missing: the weekly error-budget review plus a day.
	StaleAfter = 8 * 24 * time.Hour
)

// SLO is one service level objective in ops/slos.toml.
type SLO struct {
	Name      string  `toml:"name" json:"name"`
	Service   string  `toml:"service" json:"service"`
	Journey   string  `toml:"journey" json:"journey"`
	SLI       string  `toml:"sli" json:"sli"`                         // availability | latency (request spans) | crash-free (Sentry sessions)
	Route     string  `toml:"route,omitempty" json:"route,omitempty"` // optional: only spans of this route count
	Objective float64 `toml:"objective" json:"objective"`             // percent of good events, e.g. 99.5
	Window    string  `toml:"window" json:"window"`                   // the budget's rolling window, e.g. 28d
	Threshold string  `toml:"threshold,omitempty" json:"threshold,omitempty"`
	Alert     string  `toml:"alert,omitempty" json:"alert,omitempty"`
	Runbook   string  `toml:"runbook,omitempty" json:"runbook,omitempty"`

	window, threshold time.Duration
}

// SLOs is ops/slos.toml.
type SLOs struct {
	Version int   `toml:"version"`
	SLOs    []SLO `toml:"slo"`
}

var sloNameRE = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)

// ParseDuration reads a duration with a d (day) unit as well as Go's units.
func ParseDuration(s string) (time.Duration, error) {
	if d, ok := strings.CutSuffix(s, "d"); ok {
		n, err := strconv.Atoi(d)
		if err != nil || n <= 0 {
			return 0, fmt.Errorf("duration %q: want <days>d, e.g. 28d", s)
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

// ParseSLOs reads and validates ops/slos.toml. Unknown keys are errors.
func ParseSLOs(data []byte) (*SLOs, error) {
	var s SLOs
	dec := toml.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&s); err != nil {
		var sm *toml.StrictMissingError
		if errors.As(err, &sm) {
			return nil, fmt.Errorf("%s: unknown keys:\n%s", SLOPath, sm.String())
		}
		return nil, fmt.Errorf("%s: %w", SLOPath, err)
	}
	var errs []error
	bad := func(i int, format string, a ...any) {
		errs = append(errs, fmt.Errorf("%s: slo %d (%s): %s", SLOPath, i+1, s.SLOs[i].Name, fmt.Sprintf(format, a...)))
	}
	if s.Version != 1 {
		errs = append(errs, fmt.Errorf("%s: version must be 1", SLOPath))
	}
	if len(s.SLOs) == 0 {
		errs = append(errs, fmt.Errorf("%s: no [[slo]]", SLOPath))
	}
	var names []string
	for i := range s.SLOs {
		o := &s.SLOs[i]
		if !sloNameRE.MatchString(o.Name) {
			bad(i, "name must be lowercase letters, digits, and -")
		}
		if slices.Contains(names, o.Name) {
			bad(i, "a second SLO with this name")
		}
		names = append(names, o.Name)
		if o.Service == "" || strings.TrimSpace(o.Journey) == "" {
			bad(i, "service and journey are required: reliability is defined from the user's side")
		}
		if o.Objective <= 0 || o.Objective >= 100 {
			bad(i, "objective %g must be a percentage between 0 and 100, exclusive (100 %% leaves no error budget)", o.Objective)
		}
		var err error
		if o.window, err = ParseDuration(o.Window); err != nil || o.window < 24*time.Hour || o.window > 90*24*time.Hour {
			bad(i, "window %q must be between 1d and 90d", o.Window)
		}
		switch o.SLI {
		case "availability":
			if o.Threshold != "" {
				bad(i, "an availability SLI has no threshold")
			}
		case "latency":
			if o.threshold, err = ParseDuration(o.Threshold); err != nil || o.threshold <= 0 {
				bad(i, "a latency SLI needs a threshold such as 500ms")
			}
		case "crash-free":
			if o.Threshold != "" || o.Route != "" {
				bad(i, "a crash-free SLI counts the app's sessions in Sentry: it has no threshold and no route")
			}
		default:
			bad(i, "sli %q: want availability, latency, or crash-free", o.SLI)
		}
		if o.Runbook != "" && (!strings.HasPrefix(o.Runbook, RunbookDir+"/") || !strings.HasSuffix(o.Runbook, ".md")) {
			bad(i, "runbook %q must be a file in %s/", o.Runbook, RunbookDir)
		}
	}
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	return &s, nil
}

// Find returns the SLO with the name.
func (s *SLOs) Find(name string) (SLO, bool) {
	for _, o := range s.SLOs {
		if o.Name == name {
			return o, true
		}
	}
	return SLO{}, false
}

// good reports whether a span is a good event for the SLI.
func (o SLO) good(s Span) bool {
	if o.SLI == "latency" {
		return !s.Error && s.Duration() <= o.threshold
	}
	return !s.Error
}

// Measured is one SLO measured over an export.
type Measured struct {
	Name      string    `json:"name"`
	SLI       string    `json:"sli"`
	Objective float64   `json:"objective"`
	From      time.Time `json:"from"`
	To        time.Time `json:"to"`
	Covered   string    `json:"covered"` // how much of the SLO window the export covers
	Good      int       `json:"good"`
	Total     int       `json:"total"`
	Value     float64   `json:"value"` // percent of good events
	// BudgetRemaining is the share of the error budget left over the covered
	// window: 1 when nothing failed, 0 when spent, negative when overspent.
	BudgetRemaining float64 `json:"budget_remaining"`
	Burn1h          float64 `json:"burn_rate_1h"` // the bad share in the last hour over the allowed share
	Burn6h          float64 `json:"burn_rate_6h"`
	Spent           bool    `json:"spent"`
	// Source and Digest name where a provider measure came from and a
	// SHA-256 over its answer, so a recorded budget stays checkable; an
	// export's measure leaves them empty (the status names the export).
	Source string `json:"source,omitempty"`
	Digest string `json:"source_sha256,omitempty"`
}

// FromSentry reports whether the SLO is measured from Sentry rather than
// from an export's spans.
func (o SLO) FromSentry() bool { return o.SLI == "crash-free" }

// Measure computes the SLO over the export, from the start of its window (or
// the export's first record, whichever is later) to `to`.
func Measure(e *Export, o SLO, to time.Time) Measured {
	first, _ := e.Span()
	from := to.Add(-o.window)
	if first.After(from) {
		from = first
	}
	var h1, h1bad, h6, h6bad, good, total int
	for _, s := range e.Spans {
		if s.Service != o.Service || (o.Route != "" && s.Route != o.Route) || s.Start.Before(from) || !s.Start.Before(to) {
			continue
		}
		g := o.good(s)
		total++
		if g {
			good++
		}
		if !s.Start.Before(to.Add(-time.Hour)) {
			h1++
			if !g {
				h1bad++
			}
		}
		if !s.Start.Before(to.Add(-6 * time.Hour)) {
			h6++
			if !g {
				h6bad++
			}
		}
	}
	return measured(o, from, to, good, total, h1bad, h1, h6bad, h6)
}

// measured is an SLO's budget from its good and total events over the
// window, and its bad and total events over the last hour and six hours.
func measured(o SLO, from, to time.Time, good, total, h1bad, h1, h6bad, h6 int) Measured {
	m := Measured{Name: o.Name, SLI: o.SLI, Objective: o.Objective, From: from, To: to, Covered: days(to.Sub(from)) + " of " + o.Window,
		Good: good, Total: total}
	allowed := 1 - o.Objective/100
	m.Value, m.BudgetRemaining = 100, 1
	if m.Total > 0 {
		bad := float64(m.Total - m.Good)
		m.Value = round(100 * float64(m.Good) / float64(m.Total))
		m.BudgetRemaining = round(1 - bad/(allowed*float64(m.Total)))
	}
	burn := func(bad, n int) float64 {
		if n == 0 {
			return 0
		}
		return round(float64(bad) / float64(n) / allowed)
	}
	m.Burn1h, m.Burn6h = burn(h1bad, h1), burn(h6bad, h6)
	m.Spent = m.BudgetRemaining <= 0
	return m
}

func round(f float64) float64 { return math.Round(f*1000) / 1000 }

func days(d time.Duration) string {
	if d >= 24*time.Hour {
		return fmt.Sprintf("%.1fd", d.Hours()/24)
	}
	return d.Round(time.Minute).String()
}

// Status is slo/status.json on the ledger: every SLO's budget at one time.
type Status struct {
	Repo       string     `json:"repo"`
	ComputedAt time.Time  `json:"computed_at"`
	Export     string     `json:"export_sha256"`
	Definition string     `json:"definition_sha256"` // of ops/slos.toml
	SLOs       []Measured `json:"slos"`
	RecordedBy string     `json:"recorded_by"`
}

// Spent lists the SLOs whose error budget is spent.
func (s Status) Spent() []string {
	var out []string
	for _, m := range s.SLOs {
		if m.Spent {
			out = append(out, m.Name)
		}
	}
	return out
}

// Stale reports why the status cannot gate a release at now, or "".
func (s Status) Stale(now time.Time, defs *SLOs) string {
	if age := now.Sub(s.ComputedAt); age > StaleAfter {
		return fmt.Sprintf("the error-budget status is %s old (computed %s); it may be at most %s old", days(age), s.ComputedAt.Format(time.RFC3339), days(StaleAfter))
	}
	for _, o := range defs.SLOs {
		if !slices.ContainsFunc(s.SLOs, func(m Measured) bool { return m.Name == o.Name }) {
			return fmt.Sprintf("the error-budget status has no measure of SLO %q", o.Name)
		}
	}
	return ""
}
