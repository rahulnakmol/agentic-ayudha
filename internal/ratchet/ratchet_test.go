package ratchet

import (
	"strings"
	"testing"

	gh "github.com/google/go-github/v90/github"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
)

// The floor only tightens: a drop below the floor less the margin fails, a
// loosening fails without a new waiver (an old one is not reusable), and
// coverage that holds two margins above the floor fails until the floor is
// tightened, which Tighten does without ever lowering it.
func TestCheck(t *testing.T) {
	cur := Ratchet{Floor: 62.0, Margin: 0.5}
	for _, tc := range []struct {
		name     string
		cur      Ratchet
		base     *Ratchet
		measured float64
		want     []string // substrings of the problems, in order
		waiver   int
	}{
		{"within the margin below", cur, &cur, 61.5, nil, 0},
		{"just below the margin", cur, &cur, 61.49, []string{"below its ratchet floor 62.0"}, 0},
		{"within two margins above", cur, &cur, 62.99, nil, 0},
		{"holds two margins above", cur, &cur, 63.0, []string{"holds above its floor 62.0 %: tighten it to 62.5"}, 0},
		{"tightened by the PR", Ratchet{Floor: 62.5, Margin: 0.5}, &cur, 63.0, nil, 0},
		{"a push to dev only guards the floor", cur, nil, 70.0, nil, 0},
		{"a push to dev below the floor", cur, nil, 61.0, []string{"below its ratchet floor"}, 0},
		{"loosened without a waiver", Ratchet{Floor: 60, Margin: 0.5}, &cur, 60, []string{"loosens from 62.0 % to 60.0 % without a new waiver"}, 0},
		{"loosened reusing the base's waiver", Ratchet{Floor: 60, Margin: 0.5, Waiver: 7}, &Ratchet{Floor: 62, Margin: 0.5, Waiver: 7}, 60, []string{"without a new waiver"}, 0},
		{"loosened with a new waiver", Ratchet{Floor: 60, Margin: 0.5, Waiver: 9}, &Ratchet{Floor: 62, Margin: 0.5, Waiver: 7}, 60, nil, 9},
	} {
		got, waiver := Check(tc.cur, tc.base, tc.measured)
		if len(got) != len(tc.want) || waiver != tc.waiver {
			t.Errorf("%s: problems %q, waiver %d; want %q, waiver %d", tc.name, got, waiver, tc.want, tc.waiver)
			continue
		}
		for i, w := range tc.want {
			if !strings.Contains(got[i], w) {
				t.Errorf("%s: %q lacks %q", tc.name, got[i], w)
			}
		}
	}
	for _, tc := range []struct{ measured, want float64 }{{63.79, 63.2}, {62.3, 62.0}, {40, 62.0}} {
		if got := Tighten(cur, tc.measured); got != tc.want {
			t.Errorf("Tighten(%g) = %g, want %g", tc.measured, got, tc.want)
		}
	}
	b := SetFloor([]byte("# keep me\n[coverage]\nfloor = 62.0 # was\nmargin = 0.5\n"), 63.2)
	if r, err := Parse(b); err != nil || r.Coverage.Floor != 63.2 || !strings.Contains(string(b), "# keep me") {
		t.Errorf("SetFloor: %v %+v\n%s", err, r, b)
	}
}

// Total counts each block once, covered when any profile line for it has a
// count (as `go tool cover -func` does when packages share a file).
func TestTotal(t *testing.T) {
	prof := "mode: set\na.go:1.1,2.2 3 1\na.go:3.1,4.2 1 0\nb.go:1.1,2.2 4 0\nb.go:1.1,2.2 4 1\n"
	got, err := Total(strings.NewReader(prof))
	if err != nil || got != 87.5 { // 7 of 8 statements
		t.Fatalf("Total = %g, %v; want 87.5", got, err)
	}
	if _, err := Total(strings.NewReader("mode: set\nbroken\n")); err == nil {
		t.Error("a malformed profile was accepted")
	}
}

// Only a floor-waiver Decision for coverage that a person accepted waives,
// and, once an authority login is configured, only that person (Bug #599).
func TestAcceptedWaiver(t *testing.T) {
	dayOne := &config.Config{}
	authority := &config.Config{Autonomy: config.Autonomy{Approver: "founder"}}
	ok := func() *gh.Issue {
		return &gh.Issue{Number: gh.Ptr(9), Title: gh.Ptr("Waive the coverage floor on #5"), State: gh.Ptr("closed"), StateReason: gh.Ptr("completed"),
			Labels:   []*gh.Label{{Name: "type:decision"}, {Name: "decision:waiver"}},
			Body:     gh.Ptr("### Question\n\nShould #5 ship?\n\n### Decision\n\nwaive, accepted by @founder on 2026-10-16.\n"),
			ClosedBy: &gh.User{Login: gh.Ptr("founder"), Type: gh.Ptr("User")}}
	}
	for name, c := range map[string]*config.Config{"day one": dayOne, "authority": authority} {
		if err := AcceptedWaiver(ok(), "coverage", c, "story"); err != nil {
			t.Fatalf("%s: accepted waiver refused: %v", name, err)
		}
	}
	// Another person with write access closed it, or the founder closed one
	// naming someone else: no waiver once the founder is the authority, as
	// before on the day-one path.
	other := ok()
	other.ClosedBy = &gh.User{Login: gh.Ptr("mallory"), Type: gh.Ptr("User")}
	misnamed := ok()
	misnamed.Body = gh.Ptr("### Decision\n\nwaive, accepted by @mallory on 2026-10-16.\n")
	for _, is := range []*gh.Issue{other, misnamed} {
		if err := AcceptedWaiver(is, "coverage", authority, "story"); err == nil || !strings.Contains(err.Error(), "floor-waiver authority @founder") {
			t.Errorf("%s accepted over the authority: %v", is.GetBody(), err)
		}
		if err := AcceptedWaiver(is, "coverage", dayOne, "story"); err != nil {
			t.Errorf("day one refused %s: %v", is.GetBody(), err)
		}
	}
	for name, edit := range map[string]func(*gh.Issue){
		"not a waiver":    func(is *gh.Issue) { is.Labels = is.Labels[:1] },
		"another floor":   func(is *gh.Issue) { is.Title = gh.Ptr("Waive the perf floor on #5") },
		"still open":      func(is *gh.Issue) { is.State = gh.Ptr("open") },
		"closed unwaived": func(is *gh.Issue) { is.Body = gh.Ptr("### Decision\n\nfix, accepted by @founder.\n") },
		"closed by a bot": func(is *gh.Issue) { is.ClosedBy = &gh.User{Login: gh.Ptr("aayu-bot[bot]"), Type: gh.Ptr("Bot")} },
	} {
		is := ok()
		edit(is)
		if AcceptedWaiver(is, "coverage", dayOne, "story") == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
