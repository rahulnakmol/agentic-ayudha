package autonomy

import (
	"fmt"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/verify"
)

var (
	t0     = time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	layers = verify.LayerGlobs{"L01": {"ui/**"}, "L02": {"api/**", "cmd/main.go"}}
	rules  = Rules{Bots: []string{"aayu-machine"}, Days: 14, Layers: layers}
)

// history returns n correction-free bot PRs on files, each reviewed by a
// different family (rung A), numbered from first and merged a day apart
// starting at start, newest last.
func history(first, n int, start time.Time, files ...string) []PR {
	var out []PR
	for i := 0; i < n; i++ {
		at := start.Add(time.Duration(i) * 24 * time.Hour)
		out = append(out, PR{Number: first + i, Title: fmt.Sprintf("feat: change %d", first+i), Author: "aayudha[bot]", AuthorBot: true,
			MergedAt: at, MergeSHA: fmt.Sprintf("%040d", first+i), Files: files, Rung: "A",
			Reviews: []Review{{Login: "founder", State: "APPROVED", At: at.Add(-time.Hour)}},
			Commits: []Commit{{SHA: fmt.Sprintf("c%039d", first+i), Login: "aayudha[bot]", Bot: true, Message: "feat: x\n\nAayudha-Checkpoint: 1/1", Parents: 1}}})
	}
	return out
}

func streakOf(t *testing.T, prs []PR, issues []Issue, layer string) (LayerStreak, []Correction) {
	t.Helper()
	cs := rules.Detect(prs, issues)
	for _, s := range rules.Streaks(prs, cs, 20, []string{"L02"}) {
		if s.Layer == layer {
			return s, cs
		}
	}
	t.Fatalf("no layer %s", layer)
	return LayerStreak{}, nil
}

// WP5.7 accept: streak math at 19, 20, and 21 against the default threshold of 20.
func TestStreakAtThreshold(t *testing.T) {
	for _, n := range []int{19, 20, 21} {
		// An older corrected PR bounds the streak; a human's PR in between does not count.
		prs := history(1, 1, t0, "ui/a.tsx")
		prs[0].Labels = []string{LabelCorrection}
		prs = append(prs, history(2, n, t0.AddDate(0, 0, 1), "ui/a.tsx")...)
		prs = append(prs, PR{Number: 99, Author: "founder", MergedAt: t0.AddDate(0, 0, 5), Files: []string{"ui/b.tsx"}})
		s, _ := streakOf(t, prs, nil, "L01")
		if s.Streak != n || s.Autonomous != (n >= 20) || s.LastBreak != 1 || s.Merges != n+1 {
			t.Errorf("%d clean merges: %+v", n, s)
		}
		if got := Autonomous(rules.Streaks(prs, rules.Detect(prs, nil), 20, nil), []string{"L01"}); got != (n >= 20) {
			t.Errorf("%d clean merges: footprint [L01] autonomous = %v", n, got)
		}
	}
	// A never-layer stays manual however long its streak.
	s, _ := streakOf(t, history(1, 25, t0, "api/x.go"), nil, "L02")
	if s.Streak != 25 || s.Autonomous {
		t.Errorf("never-layer: %+v", s)
	}
}

// Decision #469, detail 8: only a proven review (different-family, or a
// person's) counts. A same-family (C) or unknown-rung merge neither counts
// nor breaks the streak, but a correction on one still breaks it.
func TestStreakSkipsUnprovenReviews(t *testing.T) {
	prs := history(1, 6, t0, "ui/a.tsx")
	prs[1].Rung, prs[2].Rung, prs[3].Rung, prs[4].Rung = "C", "", "human", "B"
	if s, _ := streakOf(t, prs, nil, "L01"); s.Streak != 4 || s.Skipped != 2 || s.Merges != 6 || s.LastBreak != 0 {
		t.Errorf("A, C, unknown, human, B, A: %+v; want a streak of 4 with 2 skipped", s)
	}
	prs[1].Labels = []string{LabelCorrection}
	if s, _ := streakOf(t, prs, nil, "L01"); s.Streak != 3 || s.Skipped != 1 || s.LastBreak != 2 {
		t.Errorf("a correction on the same-family merge: %+v; want a streak of 3 broken at #2", s)
	}
}

// WP5.7 accept: each correction kind resets the streak of every layer in the
// corrected PR's footprint, and its near miss does not.
func TestCorrectionKinds(t *testing.T) {
	newest := 21
	merged := t0.AddDate(0, 0, newest-1)
	tests := []struct {
		kind    string
		correct func(p *PR, prs *[]PR, issues *[]Issue)
		miss    func(p *PR, prs *[]PR, issues *[]Issue)
	}{
		{KindChangesRequested,
			func(p *PR, _ *[]PR, _ *[]Issue) {
				p.Reviews = append(p.Reviews, Review{Login: "founder", State: "CHANGES_REQUESTED", At: merged.Add(-2 * time.Hour)})
			},
			func(p *PR, _ *[]PR, _ *[]Issue) {
				p.Reviews = append(p.Reviews, Review{Login: "founder", State: "COMMENTED", At: merged.Add(-2 * time.Hour)})
			}},
		{KindHumanCommit,
			func(p *PR, _ *[]PR, _ *[]Issue) {
				p.Commits = append(p.Commits, Commit{SHA: "h1", Login: "founder", Message: "fix: typo", Parents: 1})
			},
			// a merge of the base branch and a commit by a configured machine user are not corrections
			func(p *PR, _ *[]PR, _ *[]Issue) {
				p.Commits = append(p.Commits, Commit{SHA: "m1", Login: "founder", Message: "Merge branch 'dev'", Parents: 2},
					Commit{SHA: "b1", Login: "aayu-machine", Message: "chore: regenerate", Parents: 1})
			}},
		{KindPostPass,
			func(p *PR, _ *[]PR, _ *[]Issue) {
				p.Comments = append(p.Comments, Comment{Body: "<!-- aayudha:inspect -->\nDisposition: **rebuild**", At: merged.Add(-30 * time.Minute)})
			},
			// the same disposition before the human pass is ordinary iteration
			func(p *PR, _ *[]PR, _ *[]Issue) {
				p.Comments = append(p.Comments, Comment{Body: "<!-- aayudha:inspect -->\nDisposition: fix", At: merged.Add(-3 * time.Hour)})
			}},
		{KindRevert,
			func(p *PR, prs *[]PR, _ *[]Issue) {
				*prs = append(*prs, PR{Number: 200, Author: "founder", Title: `Revert "` + p.Title + `"`, Body: "Reverts o/r#21", MergedAt: merged.Add(time.Hour)})
			},
			func(p *PR, prs *[]PR, _ *[]Issue) {
				*prs = append(*prs, PR{Number: 200, Author: "founder", Title: `Revert "feat: something else"`, Body: "Reverts o/r#210", MergedAt: merged.Add(time.Hour)})
			}},
		{KindLinkedIssue,
			func(_ *PR, _ *[]PR, is *[]Issue) {
				*is = append(*is, Issue{Number: 300, Kind: "incident", Body: "Since #21 merged, exports fail.", CreatedAt: merged.AddDate(0, 0, 13)})
			},
			// outside the 14-day window, or naming another number
			func(_ *PR, _ *[]PR, is *[]Issue) {
				*is = append(*is, Issue{Number: 300, Kind: "bug", Body: "Since #21 merged, exports fail.", CreatedAt: merged.AddDate(0, 0, 14)},
					Issue{Number: 301, Kind: "bug", Body: "See #210 and o/r#2100.", CreatedAt: merged.AddDate(0, 0, 1)})
			}},
		{KindManual,
			func(p *PR, _ *[]PR, _ *[]Issue) { p.Labels = append(p.Labels, LabelCorrection) },
			func(p *PR, _ *[]PR, _ *[]Issue) { p.Labels = append(p.Labels, "aayu:corrections-reviewed") }},
	}
	for _, tt := range tests {
		for _, corrected := range []bool{true, false} {
			prs := history(1, newest, t0, "ui/a.tsx", "api/b.go")
			var issues []Issue
			p := &prs[newest-1]
			if corrected {
				tt.correct(p, &prs, &issues)
			} else {
				tt.miss(p, &prs, &issues)
			}
			ui, cs := streakOf(t, prs, issues, "L01")
			api, _ := streakOf(t, prs, issues, "L02")
			want := map[bool]int{true: 0, false: newest}[corrected]
			if ui.Streak != want || api.Streak != want {
				t.Errorf("%s (corrected=%v): streaks L01 %d, L02 %d, want %d; corrections %+v", tt.kind, corrected, ui.Streak, api.Streak, want, cs)
			}
			if corrected && (len(cs) != 1 || cs[0].Kind != tt.kind || cs[0].PR != newest || fmt.Sprint(cs[0].Layers) != "[L01 L02]") {
				t.Errorf("%s: corrections %+v", tt.kind, cs)
			}
		}
	}
}
