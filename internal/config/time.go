package config

import (
	"fmt"
	"slices"
	"strings"
	"time"
	_ "time/tzdata" // Windows and minimal Linux images have no zoneinfo; [time] tz must load everywhere (WP8.2)
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

// Windows are a person's windows ([time] windows, Story #1254): the only
// times of the day aayu asks a person to act, every day, in [time] tz.
// Outside them no work waits for a person, and every request for action
// waits for the next one. No windows means a person may be asked at any time.
type Windows struct {
	Loc   *time.Location
	Spans [][2]int // [start, end) in minutes after local midnight, in order
}

// NewWindows parses tz and windows written as "10:30-11:30", one per entry.
// Windows may not overlap or cross midnight, and each starts at half past an
// hour: the hourly safety tick runs then, so it opens every window.
func NewWindows(tz string, windows []string) (Windows, error) {
	var w Windows
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return w, fmt.Errorf("[time] tz %q: %w", tz, err)
	}
	w.Loc = loc
	for _, s := range windows {
		bad := fmt.Errorf(`[time] windows: %q: want e.g. "10:30-11:30", a start before its end on one day`, s)
		start, end, ok := strings.Cut(strings.TrimSpace(s), "-")
		if !ok {
			return w, bad
		}
		a, err1 := minutes(start)
		b, err2 := minutes(end)
		if err1 != nil || err2 != nil || b <= a {
			return w, bad
		}
		if a%60 != 30 { // the safety tick (loops/build-ready.toml) runs at half past each hour, and sends the window's message
			return w, fmt.Errorf("[time] windows: %q starts at :%02d: a window starts at half past an hour, when the safety tick runs and sends its one message", s, a%60)
		}
		w.Spans = append(w.Spans, [2]int{a, b})
	}
	slices.SortFunc(w.Spans, func(x, y [2]int) int { return x[0] - y[0] })
	for i := 1; i < len(w.Spans); i++ {
		if w.Spans[i][0] < w.Spans[i-1][1] {
			return w, fmt.Errorf("[time] windows: %s overlaps %s", span(w.Spans[i-1]), span(w.Spans[i]))
		}
	}
	return w, nil
}

func span(s [2]int) string {
	return fmt.Sprintf("%02d:%02d-%02d:%02d", s[0]/60, s[0]%60, s[1]/60, s[1]%60)
}

// String names the windows in their zone: "10:30-11:30 and 18:30-19:30 Asia/Kolkata".
func (w Windows) String() string {
	if len(w.Spans) == 0 {
		return "any time (no [time] windows)"
	}
	var s []string
	for _, sp := range w.Spans {
		s = append(s, span(sp))
	}
	return strings.Join(s, " and ") + " " + w.Loc.String()
}

// at is the window of the local day d (any time on it) as instants.
func (w Windows) at(d time.Time, sp [2]int) (start, end time.Time) {
	y, m, day := d.Date()
	return time.Date(y, m, day, sp[0]/60, sp[0]%60, 0, 0, w.Loc), time.Date(y, m, day, sp[1]/60, sp[1]%60, 0, 0, w.Loc)
}

// window is the window t is in, else the next one: its start and end.
func (w Windows) window(t time.Time) (start, end time.Time) {
	t = t.In(w.Loc)
	for d := 0; ; d++ {
		y, m, day := t.Date()
		dt := time.Date(y, m, day+d, 12, 0, 0, 0, w.Loc)
		for _, sp := range w.Spans {
			if start, end = w.at(dt, sp); t.Before(end) {
				return start, end
			}
		}
	}
}

// Within reports whether t is inside a window; always true with no windows.
func (w Windows) Within(t time.Time) bool {
	if len(w.Spans) == 0 {
		return true
	}
	start, _ := w.window(t)
	return !t.Before(start)
}

// Next is t when it is inside a window, else the start of the next window.
func (w Windows) Next(t time.Time) time.Time {
	if len(w.Spans) == 0 {
		return t
	}
	start, _ := w.window(t)
	if t.Before(start) {
		return start
	}
	return t
}

// End is the end of the window t is in, else the end of the next window: the
// moment a request shown at t has gone unanswered through a whole window's
// end. With no windows it is t.
func (w Windows) End(t time.Time) time.Time {
	if len(w.Spans) == 0 {
		return t
	}
	_, end := w.window(t)
	return end
}

// Start is the start of the window t is in, else of the next window. With no
// windows it is t.
func (w Windows) Start(t time.Time) time.Time {
	if len(w.Spans) == 0 {
		return t
	}
	start, _ := w.window(t)
	return start
}
