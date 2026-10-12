package design

import (
	"strings"
	"testing"
)

// WCAG contrast reference values: black on white is 21:1, #767676 on white
// the lightest grey at 4.5:1 (4.54), #777777 just below it (4.47), and the
// ratio is symmetric.
func TestContrast(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want float64
	}{{"#000000", "#ffffff", 21}, {"#767676", "#ffffff", 4.54}, {"#ffffff", "#777777", 4.47}, {"#0b5fff", "#0b5fff", 1}} {
		if got := Contrast(tc.a, tc.b); got < tc.want-0.005 || got > tc.want+0.01 {
			t.Errorf("Contrast(%s, %s) = %.4f, want %.2f", tc.a, tc.b, got, tc.want)
		}
	}
}

// Check refuses a large-text or UI pair below 3:1 but passes a 3.0:1 one,
// names malformed colours, unknown uses, pairs outside the palette, and a
// brand without a text pair, and round-trips through Marshal.
func TestBrandCheck(t *testing.T) {
	b, err := ParseBrand([]byte(`version: 1
name: Acme
voice: {tone: [plain]}
palette: {ink: "#000", paper: "#fff", mid: "#949494", light: "#959595", bad: "blue"}
pairs:
  - {fg: mid, bg: paper, use: ui}
  - {fg: light, bg: paper, use: large-text}
  - {fg: ink, bg: nowhere, use: ui}
  - {fg: ink, bg: paper, use: body}
`))
	if err != nil {
		t.Fatal(err)
	}
	res, problems := b.Check()
	got := strings.Join(problems, "\n")
	for _, want := range []string{`palette.bad: "blue" is not an opaque hex colour`, "AA: light (#959595) on paper (#ffffff) for large-text: 2.99:1 fails AA (3.0:1)",
		"pairs[2]: ink on nowhere: both must be palette colours", `pairs[3].use: "body"`, "pairs: name at least one text pair"} {
		if !strings.Contains(got, want) {
			t.Errorf("problems lack %q:\n%s", want, got)
		}
	}
	if len(res) != 2 || !res[0].Pass || res[0].Ratio != 3.03 || res[1].Pass {
		t.Errorf("results: %+v", res)
	}
	y, err := b.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if b2, err := ParseBrand(y); err != nil || b2.Name != "Acme" || len(b2.Pairs) != 4 {
		t.Fatalf("round trip: %v\n%s", err, y)
	}

	// A brand of a future version with nothing filled in names every missing part.
	empty, err := ParseBrand([]byte("version: 2\n"))
	if err != nil {
		t.Fatal(err)
	}
	res, problems = empty.Check()
	got = strings.Join(problems, "\n")
	for _, want := range []string{"version: want 1", "name: required", "voice.tone: name at least one tone word", "palette: name at least two colours", "pairs: name at least one text pair"} {
		if !strings.Contains(got, want) {
			t.Errorf("empty brand: problems lack %q:\n%s", want, got)
		}
	}
	if len(res) != 0 {
		t.Errorf("empty brand: results %+v", res)
	}
}
