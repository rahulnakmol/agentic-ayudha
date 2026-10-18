package delegate

import (
	"testing"
	"time"
)

// The usage-limit messages as the CLIs print them (the sources are in
// usage.go): each is a limit with the right reset, and other failures,
// including a plain API rate limit, are not.
func TestUsageLimit(t *testing.T) {
	la, _ := time.LoadLocation("America/Los_Angeles")
	london, _ := time.LoadLocation("Europe/London")
	now := time.Date(2026, 10, 17, 21, 0, 0, 0, time.UTC) // 14:00 in Los Angeles, 22:00 in London
	tests := []struct {
		name, out string
		limited   bool
		resets    time.Time
	}{
		{"codex Pro, today", "ERROR: You’ve hit your usage limit. Visit https://chatgpt.com/settings/usage to purchase more credits or try again at 11:30 PM.",
			true, time.Date(2026, 10, 17, 23, 30, 0, 0, london)},
		{"codex Plus, another day", "ERROR: You’ve hit your usage limit. Upgrade to Pro (https://chatgpt.com/explore/pro), visit https://chatgpt.com/settings/usage to purchase more credits or try again at Oct 19th, 2026 9:05 AM.",
			true, time.Date(2026, 10, 19, 9, 5, 0, 0, london)},
		{"codex, no reset time", "ERROR: You’ve hit your usage limit. Try again later.", true, time.Time{}},
		{"claude, later today", "You've hit your limit · resets 8:10pm (America/Los_Angeles)", true, time.Date(2026, 10, 17, 20, 10, 0, 0, la)},
		{"claude, the hour has passed today", "You've hit your limit · resets 12pm (America/Los_Angeles)", true, time.Date(2026, 10, 18, 12, 0, 0, 0, la)},
		{"claude, a reset a minute or two past is today's", "You've hit your limit · resets 1:58pm (America/Los_Angeles)", true, time.Date(2026, 10, 17, 13, 58, 0, 0, la)},
		{"claude, the older form", "Claude usage limit reached. Your limit will reset at 7am (Asia/Singapore).", true,
			time.Date(2026, 10, 18, 7, 0, 0, 0, time.FixedZone("SGT", 8*3600))},
		{"an unverified weekly wording is not guessed at", "You've hit your weekly limit", false, time.Time{}},
		{"claude, a limit with an unknown reset shape", "You've hit your limit · resets Oct 20 (America/Los_Angeles)", true, time.Time{}},
		{"an API rate limit is not a subscription window", `Error: 429 {"type":"rate_limit_error","message":"This request would exceed your account's rate limit. Please try again later."}`, false, time.Time{}},
		{"an ordinary failure", "error: go test ./... failed", false, time.Time{}},
	}
	for _, tt := range tests {
		limited, resets := UsageLimit(tt.out, now, london)
		if limited != tt.limited || !resets.Equal(tt.resets) {
			t.Errorf("%s: limited %v, resets %v; want %v, %v", tt.name, limited, resets, tt.limited, tt.resets)
		}
	}
}
