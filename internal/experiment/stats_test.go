package experiment

import (
	"errors"
	"testing"
)

// The expected sizes are worked by hand from the formula on MinSample, with
// z(0.975) = 1.959964, z(0.8) = 0.841621, z(0.9) = 1.281552:
//
//	p1 0.10, p2 0.12, alpha 0.05, power 0.8: p̄ 0.11
//	  1.959964·√(2·0.11·0.89) = 1.959964·0.442493 = 0.867271
//	  0.841621·√(0.09 + 0.1056) = 0.841621·0.442267 = 0.372222
//	  (1.239493)² / 0.02² = 1.536343 / 0.0004 = 3840.86 → 3841
//	p1 0.05, p2 0.06, alpha 0.05, power 0.9: p̄ 0.055
//	  1.959964·√(2·0.055·0.945) = 1.959964·0.322413 = 0.631918
//	  1.281552·√(0.0475 + 0.0564) = 1.281552·0.322335 = 0.413090
//	  (1.045008)² / 0.01² = 1.092042 / 0.0001 = 10920.4 → 10921
func TestMinSampleHandComputed(t *testing.T) {
	for _, tt := range []struct {
		d    Design
		want int
	}{
		{Design{Baseline: 0.10, Detectable: 0.12, Alpha: 0.05, Power: 0.8}, 3841},
		{Design{Baseline: 0.05, Detectable: 0.05 + 0.01, Alpha: 0.05, Power: 0.9}, 10921},
		// A drop needs the same size as the symmetric rise from the lower rate.
		{Design{Baseline: 0.12, Detectable: 0.10, Alpha: 0.05, Power: 0.8}, 3841},
	} {
		got, err := tt.d.MinSample()
		if err != nil || got != tt.want {
			t.Errorf("%+v: MinSample = %d, %v; want %d", tt.d, got, err, tt.want)
		}
	}
	for _, bad := range []Design{
		{Baseline: 0.1, Detectable: 0.1, Alpha: 0.05, Power: 0.8},
		{Baseline: 0.95, Detectable: 1.05, Alpha: 0.05, Power: 0.8},
		{Baseline: 0.1, Detectable: 0.12, Alpha: 0.05, Power: 1},
	} {
		if _, err := bad.MinSample(); !errors.Is(err, ErrDesign) {
			t.Errorf("%+v: err %v, want ErrDesign", bad, err)
		}
	}
}

// Both sides of the min-sample gate: an arm with exactly the minimum passes,
// one user fewer is refused.
func TestReadoutMinSampleBoundary(t *testing.T) {
	c := Arm{Name: "control", Users: 3841, Converted: 384}
	tr := Arm{Name: "test", Users: 3841, Converted: 461}
	if _, err := Readout(c, tr, 3841, 0.05); err != nil {
		t.Fatalf("at the minimum: %v", err)
	}
	tr.Users = 3840
	if _, err := Readout(c, tr, 3841, 0.05); !errors.Is(err, ErrUnderSample) {
		t.Fatalf("one under the minimum in the treatment arm: %v, want ErrUnderSample", err)
	}
}

// The Minimum sample field is untrusted: a stated size its parameters do not
// give is refused, and the field round-trips.
func TestParseMinSample(t *testing.T) {
	n, d, err := ParseMinSample("3841 per arm; two-sided two-proportion z-test, alpha 0.05, power 0.8, baseline rate 0.1, detectable rate 0.12")
	if err != nil || n != 3841 || d != (Design{Baseline: 0.1, Detectable: 0.12, Alpha: 0.05, Power: 0.8}) {
		t.Fatalf("parse: %d %+v %v", n, d, err)
	}
	if got := FormatMinSample(n, d); got != "3841 per arm; two-sided two-proportion z-test, alpha 0.05, power 0.8, baseline rate 0.1, detectable rate 0.12" {
		t.Errorf("format: %q", got)
	}
	for _, s := range []string{
		"1000 per arm; two-sided two-proportion z-test, alpha 0.05, power 0.8, baseline rate 0.1, detectable rate 0.12",
		"3841 per arm",
	} {
		if _, _, err := ParseMinSample(s); !errors.Is(err, ErrDesign) {
			t.Errorf("%q: err %v, want ErrDesign", s, err)
		}
	}
}
