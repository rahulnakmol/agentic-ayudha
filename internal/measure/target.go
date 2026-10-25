package measure

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/status"
)

// ErrSpec means a metric, baseline, or target cannot be read.
var ErrSpec = errors.New("unreadable measure spec")

// Spec is what a Goal or Initiative is measured on: a snapshot metric
// (Metrics), aayu:founder_minutes_per_pr from the actions log (Story #989),
// or a provider baseline, "<source>:<metric> [filter]" (posthog, sentry,
// github, growthbook; WP5.2).
type Spec struct {
	Metric string `json:"metric"`
	Source string `json:"source,omitempty"` // empty for snapshot metrics
	Filter string `json:"filter,omitempty"`
}

var specRE = regexp.MustCompile(`^([a-z]+):([a-z_]+)(?:\s+(.+))?$`)

// ParseSpec reads a metric spec from --metric or the first line of the
// issue's Metric field.
func ParseSpec(s string) (Spec, error) {
	s, _, _ = strings.Cut(strings.TrimSpace(s), "\n")
	s = strings.Trim(strings.TrimSpace(s), "`")
	if slices.Contains(Metrics, s) {
		return Spec{Metric: s}, nil
	}
	if m := specRE.FindStringSubmatch(s); m != nil {
		return Spec{Source: m[1], Metric: m[2], Filter: strings.TrimSpace(m[3])}, nil
	}
	return Spec{}, fmt.Errorf("%w: metric %q: want one of %s, or <source>:<metric> [filter] for a provider baseline (posthog:users event = 'signed_up')",
		ErrSpec, s, strings.Join(Metrics, ", "))
}

// String renders the spec as ParseSpec reads it.
func (s Spec) String() string {
	if s.Source == "" {
		return s.Metric
	}
	return strings.TrimSpace(s.Source + ":" + s.Metric + " " + s.Filter)
}

var numberRE = regexp.MustCompile(`(-?[0-9]+(?:\.[0-9]+)?)\s*(%?)`)

func number(s string) (float64, bool) {
	m := numberRE.FindStringSubmatch(s)
	if m == nil {
		return 0, false
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, false
	}
	if m[2] == "%" {
		v /= 100
	}
	return v, true
}

// ParseBaseline reads the baseline value: the first number in the Baseline
// field, in the metric's unit (a trailing % divides by 100).
func ParseBaseline(s string) (float64, error) {
	v, ok := number(s)
	if !ok {
		return 0, fmt.Errorf("%w: Baseline %q holds no number", ErrSpec, strings.TrimSpace(s))
	}
	return v, nil
}

// Target is the value to reach and the direction.
type Target struct {
	Op    string  `json:"op"` // >=, >, <=, <
	Value float64 `json:"value"`
}

var targetRE = regexp.MustCompile(`(>=|≥|<=|≤|>|<)?\s*(-?[0-9]+(?:\.[0-9]+)?)\s*(%?)`)

// ParseTarget reads the Target field: an optional comparator and the first
// number. Without a comparator the target lies in the direction it moves
// from the baseline: at least a target above it, at most one below it.
func ParseTarget(s string, baseline float64) (Target, error) {
	m := targetRE.FindStringSubmatch(s)
	if m == nil {
		return Target{}, fmt.Errorf("%w: Target %q holds no number", ErrSpec, strings.TrimSpace(s))
	}
	v, _ := strconv.ParseFloat(m[2], 64)
	if m[3] == "%" {
		v /= 100
	}
	op := map[string]string{"≥": ">=", "≤": "<="}[m[1]]
	if op == "" {
		op = m[1]
	}
	if op == "" {
		op = ">="
		if v < baseline {
			op = "<="
		}
	}
	return Target{Op: op, Value: v}, nil
}

// Met reports whether value reaches the target.
func (t Target) Met(value float64) bool {
	switch t.Op {
	case ">":
		return value > t.Value
	case "<=":
		return value <= t.Value
	case "<":
		return value < t.Value
	}
	return value >= t.Value
}

// Measured is a Goal's or Initiative's measure against its baseline.
type Measured struct {
	Spec     Spec     `json:"spec"`
	Value    float64  `json:"value"`
	Detail   string   `json:"detail"`
	Baseline float64  `json:"baseline"`
	Target   Target   `json:"target"`
	Met      bool     `json:"met"`
	Window   Window   `json:"window"`
	Query    string   `json:"query"`
	Snapshot string   `json:"snapshot,omitempty"` // file name
	SHA256   string   `json:"snapshot_sha256,omitempty"`
	Engine   string   `json:"engine,omitempty"`
	Sources  []string `json:"sources,omitempty"` // "<kind> <file> sha256 <hash>"
}

func code(s string) string {
	fence := "`"
	for strings.Contains(s, fence) {
		fence += "`"
	}
	return fence + " " + s + " " + fence
}

// ResultText is the Result measure writes on the Goal or Initiative.
func ResultText(m Measured, at time.Time) string {
	head := "Target not met:"
	if m.Met {
		head = status.TargetMet
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s = %s against the target %s %s (baseline %s, change %+.4g).\n\n", head, m.Spec, num(m.Value), m.Target.Op, num(m.Target.Value),
		num(m.Baseline), m.Value-m.Baseline)
	fmt.Fprintf(&b, "%s. Window: %s to %s (UTC, half-open).\n\n", m.Detail, m.Window.From.UTC().Format(time.RFC3339), m.Window.To.UTC().Format(time.RFC3339))
	if m.Snapshot != "" {
		fmt.Fprintf(&b, "Snapshot: %s (SHA-256 %s, engine %s) from %s.\n\n", m.Snapshot, m.SHA256, m.Engine, strings.Join(m.Sources, "; "))
	}
	fmt.Fprintf(&b, "Query: %s\n\nMeasured %s by `aayu measure`.\n", code(strings.Join(strings.Fields(m.Query), " ")), at.UTC().Format(time.RFC3339))
	return b.String()
}

func num(v float64) string { return strconv.FormatFloat(v, 'g', 6, 64) }
