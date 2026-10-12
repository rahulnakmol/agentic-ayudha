// Package experiment holds what `aayu experiment` and `aayu readout` (WP6.7)
// compute: the minimum sample size per arm for a conversion experiment, the
// lift between two arms with its two-proportion z-test, and the text both
// verbs write into an Experiment issue. Everything here is deterministic
// (ground rule 7): the numbers a readout writes come from this package, never
// from a provider's own statistics or a model.
package experiment

import (
	"errors"
	"fmt"
	"math"
)

// ErrDesign means the experiment's parameters are unusable (a rate outside
// (0, 1), no detectable change, alpha or power out of range).
var ErrDesign = errors.New("invalid experiment design")

// ErrUnderSample means an arm has not reached the minimum sample size; a
// readout refuses (the min-sample gate).
var ErrUnderSample = errors.New("under minimum sample")

// z is the standard normal quantile function.
func z(p float64) float64 { return math.Sqrt2 * math.Erfinv(2*p-1) }

// Design is a two-arm conversion experiment's power calculation.
type Design struct {
	Baseline   float64 `json:"baseline_rate"`   // control conversion rate p1
	Detectable float64 `json:"detectable_rate"` // treatment rate p2 the test must detect
	Alpha      float64 `json:"alpha"`           // two-sided significance level
	Power      float64 `json:"power"`           // 1 - beta
}

// Check reports whether the design is usable.
func (d Design) Check() error {
	switch {
	case !(d.Baseline > 0 && d.Baseline < 1):
		return fmt.Errorf("%w: baseline rate %v: want a rate strictly between 0 and 1", ErrDesign, d.Baseline)
	case !(d.Detectable > 0 && d.Detectable < 1):
		return fmt.Errorf("%w: detectable rate %v: want a rate strictly between 0 and 1 (baseline %v plus the minimum detectable effect)", ErrDesign, d.Detectable, d.Baseline)
	case d.Detectable == d.Baseline:
		return fmt.Errorf("%w: the minimum detectable effect is zero", ErrDesign)
	case !(d.Alpha > 0 && d.Alpha < 0.5):
		return fmt.Errorf("%w: alpha %v: want a value in (0, 0.5)", ErrDesign, d.Alpha)
	case !(d.Power >= 0.5 && d.Power < 1):
		return fmt.Errorf("%w: power %v: want a value in [0.5, 1)", ErrDesign, d.Power)
	}
	return nil
}

// MinSample is the minimum number of users per arm for a two-sided
// two-proportion z-test to detect a change from Baseline to Detectable with
// the design's alpha and power (equal arms):
//
//	n = (z(1-α/2)·√(2·p̄·(1-p̄)) + z(1-β)·√(p1·(1-p1) + p2·(1-p2)))² / (p2 - p1)²
//
// with p̄ = (p1 + p2) / 2, rounded up.
func (d Design) MinSample() (int, error) {
	if err := d.Check(); err != nil {
		return 0, err
	}
	p1, p2 := d.Baseline, d.Detectable
	pbar := (p1 + p2) / 2
	num := z(1-d.Alpha/2)*math.Sqrt(2*pbar*(1-pbar)) + z(d.Power)*math.Sqrt(p1*(1-p1)+p2*(1-p2))
	n := num * num / ((p2 - p1) * (p2 - p1))
	// Guard against floating-point noise just above an integer.
	return int(math.Ceil(n - 1e-9)), nil
}

// Arm is one variation's users and conversions.
type Arm struct {
	Name      string `json:"name"`
	Users     int64  `json:"users"`
	Converted int64  `json:"converted"`
}

// Rate is the arm's conversion rate.
func (a Arm) Rate() float64 {
	if a.Users == 0 {
		return 0
	}
	return float64(a.Converted) / float64(a.Users)
}

// Lift is a readout of a treatment arm against the control arm.
type Lift struct {
	Control     Arm     `json:"control"`
	Treatment   Arm     `json:"treatment"`
	Absolute    float64 `json:"absolute"` // p2 - p1
	Relative    float64 `json:"relative"` // (p2 - p1) / p1
	Z           float64 `json:"z"`        // pooled two-proportion z statistic
	P           float64 `json:"p_value"`  // two-sided
	CILow       float64 `json:"ci_low"`   // confidence interval for Absolute at 1 - alpha (unpooled)
	CIHigh      float64 `json:"ci_high"`
	Alpha       float64 `json:"alpha"`
	Significant bool    `json:"significant"` // P < Alpha
}

// Readout compares treatment with control after checking that both arms hold
// at least minSample users and are consistent (0 <= converted <= users).
func Readout(control, treatment Arm, minSample int, alpha float64) (Lift, error) {
	for _, a := range []Arm{control, treatment} {
		if a.Users < 0 || a.Converted < 0 || a.Converted > a.Users {
			return Lift{}, fmt.Errorf("%w: arm %s has %d conversions among %d users", ErrDesign, a.Name, a.Converted, a.Users)
		}
	}
	if minSample < 1 {
		return Lift{}, fmt.Errorf("%w: minimum sample %d", ErrDesign, minSample)
	}
	for _, a := range []Arm{control, treatment} {
		if a.Users < int64(minSample) {
			return Lift{}, fmt.Errorf("%w: arm %s has %d of the %d users per arm the design needs (%s %d, %s %d)",
				ErrUnderSample, a.Name, a.Users, minSample, control.Name, control.Users, treatment.Name, treatment.Users)
		}
	}
	p1, p2 := control.Rate(), treatment.Rate()
	n1, n2 := float64(control.Users), float64(treatment.Users)
	l := Lift{Control: control, Treatment: treatment, Absolute: p2 - p1, Alpha: alpha}
	if p1 > 0 {
		l.Relative = (p2 - p1) / p1
	}
	pool := float64(control.Converted+treatment.Converted) / (n1 + n2)
	if se := math.Sqrt(pool * (1 - pool) * (1/n1 + 1/n2)); se > 0 {
		l.Z = (p2 - p1) / se
	}
	l.P = math.Erfc(math.Abs(l.Z) / math.Sqrt2)
	half := z(1-alpha/2) * math.Sqrt(p1*(1-p1)/n1+p2*(1-p2)/n2)
	l.CILow, l.CIHigh = l.Absolute-half, l.Absolute+half
	l.Significant = l.P < alpha
	return l, nil
}
