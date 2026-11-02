package cli

import (
	"fmt"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
)

// dayIsSet is the last card of a session that started a mission (Story
// #1175).
const dayIsSet = "The day is set"

// daySet is that card's lines: one per Feature whose mission started in this
// session, from its Mission start card, with today's milestone, its stories,
// and the evening review: the next evening window of [time] windows, the
// last window of its day, in [time] tz (Story #1254). With no windows it is
// the end of [time] business_hours. A session that started none has no
// lines: nothing is set until the start.
func (e *Env) daySet(cfg *config.Config, started []InboxItem) []string {
	review := "at the end of [time] business_hours"
	if w, err := cfg.PersonWindows(); err == nil && len(w.Spans) > 0 {
		start, end := eveningWindow(w, e.Now())
		review = fmt.Sprintf("%s-%s %s, the evening window", start.Format("Mon 2 Jan 15:04"), end.Format("15:04"), w.Loc)
	} else if clk, err := cfg.Clock(); err == nil {
		day := clk.Next(e.Now()).In(clk.Loc) // past today's hours, the next business day
		y, m, d := day.Date()
		end := time.Date(y, m, d, clk.End/60, clk.End%60, 0, 0, clk.Loc)
		review = fmt.Sprintf("%s %s, the end of [time] business_hours", end.Format("Mon 2 Jan 15:04"), clk.Loc)
	}
	var out []string
	for _, it := range started {
		var today MissionMilestone
		for _, m := range it.plan.Milestones {
			if m.Gate.State != GatePassed {
				today = m
				break
			}
		}
		out = append(out, fmt.Sprintf("#%d · today %s: %s · evening review %s", it.Number, today.Name, issueRefs(today.Stories), review))
	}
	return out
}

// eveningWindow is the evening window now is in, else the next one: the last
// of the windows on its local day, in their zone. w has a window.
func eveningWindow(w config.Windows, now time.Time) (start, end time.Time) {
	now = now.In(w.Loc)
	for d := 0; ; d++ {
		y, m, day := now.Date()
		last := w.Spans[len(w.Spans)-1]
		start = time.Date(y, m, day+d, last[0]/60, last[0]%60, 0, 0, w.Loc)
		if end = time.Date(y, m, day+d, last[1]/60, last[1]%60, 0, 0, w.Loc); now.Before(end) {
			return start, end
		}
	}
}
