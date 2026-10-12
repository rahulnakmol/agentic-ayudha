package experiment

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const testName = "two-sided two-proportion z-test"

func num(v float64) string { return strconv.FormatFloat(v, 'g', -1, 64) }

// FormatMinSample is the Experiment's "Minimum sample" field: the per-arm
// size and the power calculation it came from, in a form ParseMinSample reads
// back exactly.
func FormatMinSample(n int, d Design) string {
	return fmt.Sprintf("%d per arm; %s, alpha %s, power %s, baseline rate %s, detectable rate %s",
		n, testName, num(d.Alpha), num(d.Power), num(d.Baseline), num(d.Detectable))
}

var minSampleRE = regexp.MustCompile(`^\s*([0-9]+) per arm; ` + testName + `, alpha ([0-9.eE+-]+), power ([0-9.eE+-]+), baseline rate ([0-9.eE+-]+), detectable rate ([0-9.eE+-]+)\s*$`)

// ParseMinSample reads the "Minimum sample" field and recomputes the size
// from its parameters. The issue body is untrusted (§0.1): a stated size that
// differs from the one its parameters give is refused rather than believed.
func ParseMinSample(s string) (int, Design, error) {
	first, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	m := minSampleRE.FindStringSubmatch(first)
	if m == nil {
		return 0, Design{}, fmt.Errorf("%w: Minimum sample %q: want %q (as `aayu experiment` writes it)", ErrDesign, first,
			FormatMinSample(3841, Design{Baseline: 0.1, Detectable: 0.12, Alpha: 0.05, Power: 0.8}))
	}
	var f [4]float64
	for i := range f {
		v, err := strconv.ParseFloat(m[i+2], 64)
		if err != nil {
			return 0, Design{}, fmt.Errorf("%w: Minimum sample: %q is not a number", ErrDesign, m[i+2])
		}
		f[i] = v
	}
	d := Design{Alpha: f[0], Power: f[1], Baseline: f[2], Detectable: f[3]}
	n, err := d.MinSample()
	if err != nil {
		return 0, d, err
	}
	stated, _ := strconv.Atoi(m[1])
	if stated != n {
		return 0, d, fmt.Errorf("%w: Minimum sample states %d per arm, but its parameters give %d", ErrDesign, stated, n)
	}
	return n, d, nil
}

func pct(v float64) string  { return fmt.Sprintf("%.2f %%", v*100) }
func pts(v float64) string  { return fmt.Sprintf("%+.2f pts", v*100) }
func spct(v float64) string { return fmt.Sprintf("%+.2f %%", v*100) }

// Code renders s as a markdown code span that keeps its backticks (a HogQL
// query quotes identifiers with them), so the source query stays exact.
func Code(s string) string {
	fence := "`"
	for strings.Contains(s, fence) {
		fence += "`"
	}
	return fence + " " + s + " " + fence
}

// Window is the half-open interval a readout covers.
type Window struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

// ResultText is the Result readout writes on the Experiment.
func ResultText(l Lift, minSample int, w Window, source string, fetched time.Time) string {
	verdict := "significant"
	if !l.Significant {
		verdict = "not significant"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Lift of %s over %s: %s relative (%s), %s at alpha %s (z = %.3f, p = %.4g); %s %% confidence interval for the difference [%s, %s].\n\n",
		l.Treatment.Name, l.Control.Name, spct(l.Relative), pts(l.Absolute), verdict, num(l.Alpha), l.Z, l.P,
		num((1-l.Alpha)*100), pts(l.CILow), pts(l.CIHigh))
	b.WriteString("| Arm | Users | Converted | Rate |\n|---|---|---|---|\n")
	for _, a := range []Arm{l.Control, l.Treatment} {
		fmt.Fprintf(&b, "| %s | %d | %d | %s |\n", a.Name, a.Users, a.Converted, pct(a.Rate()))
	}
	fmt.Fprintf(&b, "\nMinimum sample: %d per arm, reached by both arms. Window: %s to %s (UTC, half-open).\n\n", minSample,
		w.From.UTC().Format(time.RFC3339), w.To.UTC().Format(time.RFC3339))
	fmt.Fprintf(&b, "Source: %s\n\nFetched %s by `aayu readout`.\n", Code(source), fetched.UTC().Format(time.RFC3339))
	return b.String()
}
