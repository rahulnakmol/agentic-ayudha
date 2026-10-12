package config

import (
	"fmt"
	"strings"
	"time"
)

// Clock counts business hours in an IANA time zone ([time] tz business_hours).
// Decision SLAs and paging windows use it; Sev0/Sev1 incidents use wall-clock time.
type Clock struct {
	Loc        *time.Location
	Days       [7]bool // indexed by time.Weekday
	Start, End int     // minutes after local midnight
}

var weekdays = map[string]time.Weekday{
	"sun": time.Sunday, "mon": time.Monday, "tue": time.Tuesday, "wed": time.Wednesday,
	"thu": time.Thursday, "fri": time.Friday, "sat": time.Saturday,
}

// NewClock parses tz (an IANA zone such as "Europe/London") and business hours
// written as "Mon-Fri 09:00-18:00" (a day range or comma-separated days, then a time range).
func NewClock(tz, hours string) (Clock, error) {
	var c Clock
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return c, fmt.Errorf("[time] tz %q: %w", tz, err)
	}
	c.Loc = loc
	days, span, ok := strings.Cut(strings.TrimSpace(hours), " ")
	bad := fmt.Errorf(`[time] business_hours %q: want e.g. "Mon-Fri 09:00-18:00"`, hours)
	if !ok {
		return c, bad
	}
	for _, part := range strings.Split(days, ",") {
		from, to, isRange := strings.Cut(strings.ToLower(strings.TrimSpace(part)), "-")
		a, ok1 := weekdays[from]
		b, ok2 := a, true
		if isRange {
			b, ok2 = weekdays[to]
		}
		if !ok1 || !ok2 {
			return c, bad
		}
		for d := a; ; d = (d + 1) % 7 {
			c.Days[d] = true
			if d == b {
				break
			}
		}
	}
	start, end, ok := strings.Cut(strings.TrimSpace(span), "-")
	if !ok {
		return c, bad
	}
	if c.Start, err = minutes(start); err != nil {
		return c, bad
	}
	if c.End, err = minutes(end); err != nil || c.End <= c.Start {
		return c, bad
	}
	return c, nil
}

func minutes(hhmm string) (int, error) {
	t, err := time.Parse("15:04", hhmm)
	if err != nil {
		return 0, err
	}
	return t.Hour()*60 + t.Minute(), nil
}

// window returns the business window of the local calendar day containing t,
// and whether that day has one. Windows are built from local wall-clock times,
// so they stay at 09:00-18:00 local across DST changes.
func (c Clock) window(t time.Time) (start, end time.Time, ok bool) {
	y, m, d := t.Date()
	if !c.Days[t.Weekday()] {
		return start, end, false
	}
	start = time.Date(y, m, d, c.Start/60, c.Start%60, 0, 0, c.Loc)
	end = time.Date(y, m, d, c.End/60, c.End%60, 0, 0, c.Loc)
	return start, end, true
}

// Add returns the instant at which d of business time has elapsed after t.
func (c Clock) Add(t time.Time, d time.Duration) time.Time {
	t = t.In(c.Loc)
	for {
		if start, end, ok := c.window(t); ok {
			if t.Before(start) {
				t = start
			}
			if t.Before(end) {
				if left := end.Sub(t); d <= left {
					return t.Add(d)
				} else {
					d -= left
				}
			}
		}
		y, m, day := t.Date()
		t = time.Date(y, m, day+1, 0, 0, 0, 0, c.Loc)
	}
}

// Within reports whether t falls inside business hours.
func (c Clock) Within(t time.Time) bool {
	t = t.In(c.Loc)
	start, end, ok := c.window(t)
	return ok && !t.Before(start) && t.Before(end)
}

// Next returns t if it is inside business hours, else the start of the next window.
func (c Clock) Next(t time.Time) time.Time {
	return c.Add(t, 0)
}

// Between returns the business time from a to b; 0 when b is not after a.
func (c Clock) Between(a, b time.Time) time.Duration {
	var d time.Duration
	for t := a.In(c.Loc); t.Before(b); {
		if start, end, ok := c.window(t); ok {
			from, to := t, end
			if from.Before(start) {
				from = start
			}
			if b.Before(to) {
				to = b
			}
			if to.After(from) {
				d += to.Sub(from)
			}
		}
		y, m, day := t.Date()
		t = time.Date(y, m, day+1, 0, 0, 0, 0, c.Loc)
	}
	return d
}
