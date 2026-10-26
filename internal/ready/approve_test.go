package ready

import "testing"

// The effect a person approves names the stories and the budget exactly:
// runs of numbers as ranges, gaps kept, and cents only when there are cents.
func TestEffectNumbers(t *testing.T) {
	for _, c := range []struct {
		nums []int
		want string
	}{
		{[]int{851, 852, 853, 854, 855, 856}, "#851-#856"},
		{[]int{851, 853, 854, 857}, "#851, #853-#854, #857"},
		{[]int{12}, "#12"},
	} {
		if got := Numbers(c.nums); got != c.want {
			t.Errorf("Numbers(%v) = %q, want %q", c.nums, got, c.want)
		}
	}
	for v, want := range map[float64]string{360: "$360", 62.5: "$62.50", 0.25: "$0.25"} {
		if got := USD(v); got != want {
			t.Errorf("USD(%v) = %q, want %q", v, got, want)
		}
	}
}
