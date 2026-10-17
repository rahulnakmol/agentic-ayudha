package delegate

import (
	"regexp"
	"strings"
	"time"
)

// The subscription usage-limit messages the receivers' CLIs print, as they
// print them (founder decision, 2026-10-17: a build pauses until the window
// resets). Only these are recognised; any other failure stays a failure.
//
//   - Codex (codex-rs protocol/src/error.rs, UsageLimitReachedError): `codex
//     exec` writes "ERROR: You’ve hit your usage limit. … try again at
//     <time>." to stderr and exits 1. <time> is the process's local time,
//     "3:04 PM" for today or "Oct 12th, 2026 4:30 PM" for another day, or
//     the message ends "try again later." with no time.
//   - Claude Code: "You've hit your limit · resets 8:10pm
//     (America/Los_Angeles)", and before it "Claude usage limit reached. Your
//     limit will reset at 7am (Asia/Singapore)." (anthropics/claude-code
//     issues 52612, 42003, 55597, 9048). A limit with another reset shape,
//     such as a weekly one with a day, is still a limit, with no reset time.
var (
	codexLimit   = regexp.MustCompile(`You[’']ve hit your usage limit`)
	codexAt      = regexp.MustCompile(`(?i)try again at ((?:[A-Z][a-z]{2} \d{1,2}(?:st|nd|rd|th), \d{4} )?\d{1,2}:\d{2} [AP]M)\.`)
	claudeLimit  = regexp.MustCompile(`You[’']ve hit your limit|Claude usage limit reached`)
	claudeResets = regexp.MustCompile(`(?i)(?:\bresets|will reset at) (\d{1,2})(?::(\d{2}))?\s*([ap]m) \(([A-Za-z]+(?:/[A-Za-z0-9_+-]+)+|UTC)\)`)
	ordinal      = regexp.MustCompile(`(\d)(?:st|nd|rd|th),`)
)

// UsageLimit reports whether a receiver's output says its subscription usage
// limit was reached and, when it says, when the limit resets (zero when it
// does not). now is the current time; loc is the zone Codex prints its local
// times in (the machine's).
func UsageLimit(out string, now time.Time, loc *time.Location) (bool, time.Time) {
	switch {
	case codexLimit.MatchString(out):
		m := codexAt.FindStringSubmatch(out)
		if m == nil {
			return true, time.Time{}
		}
		if t, err := time.ParseInLocation("Jan 2, 2006 3:04 PM", ordinal.ReplaceAllString(m[1], "$1,"), loc); err == nil {
			return true, t
		}
		t, err := time.ParseInLocation("3:04 PM", m[1], loc)
		if err != nil {
			return true, time.Time{}
		}
		n := now.In(loc)
		return true, time.Date(n.Year(), n.Month(), n.Day(), t.Hour(), t.Minute(), 0, 0, loc)
	case claudeLimit.MatchString(out):
		m := claudeResets.FindStringSubmatch(out)
		if m == nil {
			return true, time.Time{}
		}
		zone, err := time.LoadLocation(m[4])
		if err != nil {
			return true, time.Time{}
		}
		clock := m[1] + ":" + m[2] + strings.ToLower(m[3])
		if m[2] == "" {
			clock = m[1] + ":00" + strings.ToLower(m[3])
		}
		t, err := time.Parse("3:04pm", clock)
		if err != nil {
			return true, time.Time{}
		}
		n := now.In(zone)
		at := time.Date(n.Year(), n.Month(), n.Day(), t.Hour(), t.Minute(), 0, 0, zone)
		if at.Before(now) { // the next time the clock shows it
			at = at.AddDate(0, 0, 1)
		}
		return true, at
	}
	return false, time.Time{}
}
