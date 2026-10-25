package measure

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
)

// rec is an actions log record at hh:mm on 2026-10-12 (UTC) by identity.
func rec(hhmm, identity string) actions.Record {
	t, err := time.Parse("2006-01-02 15:04", "2026-10-12 "+hhmm)
	if err != nil {
		panic(err)
	}
	return actions.Record{Time: t, Action: actions.Action{Kind: "github.issue.labels.add", Identity: identity}, Status: actions.StatusDone}
}

var (
	founderWindow   = Window{From: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC)}
	founderDefaults = config.Outcomes{SessionGapMinutes: 10, ActionMinutes: 2}
)

// Story #989, criteria 1 and 4: human actions at 09:00, 09:04, 09:09 and
// 11:30 are two sessions, 09:00-09:09 (9 + 2 minutes) and 11:30 (0 + 2), so
// 13 founder minutes over 12 merged PRs = 1.08, reported as 1.1. Bot and
// human-fallback records, placed where they would bridge the two sessions or
// add a third, change nothing; nor does a human record outside the window.
func TestFounderMinutesPerPR(t *testing.T) {
	human := []actions.Record{rec("09:00", "human"), rec("09:04", "human"), rec("09:09", "human"), rec("11:30", "human")}
	others := []actions.Record{rec("09:15", "bot"), rec("09:25", "human-fallback"), rec("09:35", "bot"), rec("11:20", "human-fallback"),
		rec("14:00", "bot"), rec("15:00", ""), {Time: founderWindow.To, Action: actions.Action{Identity: "human"}}}
	for name, recs := range map[string][]actions.Record{
		"human only":                             human,
		"with bot and human-fallback, unordered": append(append([]actions.Record{others[0], human[3]}, others[1:]...), human[:3]...),
	} {
		v, detail, err := Founder(recs, 12, founderWindow, founderDefaults, "dev")
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if v != 1.1 {
			t.Errorf("%s: value %v, want 1.1 (13 minutes / 12 PRs)", name, v)
		}
		for _, want := range []string{"13 founder minutes", "2 sessions", "4 human-identity actions in the actions log", "12 pull requests merged into dev",
			"[outcomes] session_gap_minutes = 10", "[outcomes] action_minutes = 2"} {
			if !strings.Contains(detail, want) {
				t.Errorf("%s: detail %q lacks %q", name, detail, want)
			}
		}
	}
}

// A gap of exactly session_gap_minutes stays in the session; one minute more
// starts a new one. The keys are read, not assumed: gap 10 vs 11 and
// action_minutes 3.
func TestFounderSessionGapBoundary(t *testing.T) {
	recs := []actions.Record{rec("09:00", "human"), rec("09:10", "human"), rec("09:21", "human")}
	// 09:00-09:10 (10 + 3) and 09:21 (0 + 3) = 16 minutes over 4 PRs = 4.
	if v, _, err := Founder(recs, 4, founderWindow, config.Outcomes{SessionGapMinutes: 10, ActionMinutes: 3}, "dev"); err != nil || v != 4 {
		t.Errorf("gap 10: %v, %v; want 4", v, err)
	}
	// With session_gap_minutes = 11 it is one session: 21 + 3 = 24 over 4 = 6.
	if v, _, err := Founder(recs, 4, founderWindow, config.Outcomes{SessionGapMinutes: 11, ActionMinutes: 3}, "dev"); err != nil || v != 6 {
		t.Errorf("gap 11: %v, %v; want 6", v, err)
	}
}

// Story #989, criterion 3: with no merged pull request the measure is
// undefined, not zero or infinite.
func TestFounderNoMergedPR(t *testing.T) {
	_, _, err := Founder([]actions.Record{rec("09:00", "human")}, 0, founderWindow, founderDefaults, "dev")
	if !errors.Is(err, ErrNoData) || !strings.Contains(err.Error(), "no merged pull request in the window; founder minutes per merged PR is undefined") {
		t.Fatalf("err = %v", err)
	}
}
