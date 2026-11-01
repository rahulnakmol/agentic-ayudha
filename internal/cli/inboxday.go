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
// and the evening review at the end of [time] business_hours. A session that
// started none has no lines: nothing is set until the start.
func (e *Env) daySet(cfg *config.Config, started []InboxItem) []string {
	review := "at the end of [time] business_hours"
	if clk, err := cfg.Clock(); err == nil {
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
