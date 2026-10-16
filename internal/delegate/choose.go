package delegate

import (
	"fmt"
	"strings"
)

// Choice is the receiver `--delegate auto` picked, and why.
type Choice struct {
	Receiver string `json:"receiver"`
	Local    bool   `json:"local"`
	Reason   string `json:"reason"`
}

// Cost notes per receiver (research §5, 2026-10-15): what one more task costs
// the founder, which decides the cheapest adequate receiver.
var costNote = map[string]string{
	Codex:  "Codex cloud runs on the ChatGPT plan's allowance with no VM charge",
	Claude: "Claude Code on the web runs on the claude.ai plan's usage",
	Amp:    "Amp charges API-price credits plus orb minutes (a1.small 0.17 USD/h)",
	Cursor: "Cursor charges the model's API price under a spend limit",
}

// Choose picks the cheapest adequate receiver for a story's Effort among those
// connected. Small stories (XS, S) go to a receiver whose marginal cost is a
// plan allowance (Codex, then Claude Code); larger ones (M, L) to a receiver
// with native follow-ups on review comments (Cursor's runs, then Amp's
// threads), which a multi-checkpoint story's fix cycles need. The caller
// passes cloud agents by default and local CLIs only for --local, so a local
// CLI is picked only when cloud is empty.
func Choose(effort string, cloud, local map[string]bool) (Choice, error) {
	order := []string{Codex, Claude, Amp, Cursor}
	size := "small"
	if e := strings.ToUpper(effort); e == "M" || e == "L" {
		order, size = []string{Cursor, Amp, Claude, Codex}, "larger"
	}
	for i, r := range order {
		if cloud[r] {
			why := fmt.Sprintf("effort %s (%s story): %s", orDash(effort), size, costNote[r])
			if i > 0 {
				why += fmt.Sprintf("; %s is not connected", strings.Join(order[:i], ", "))
			}
			return Choice{Receiver: r, Reason: why}, nil
		}
	}
	for _, r := range order {
		if local[r] {
			return Choice{Receiver: r, Local: true, Reason: fmt.Sprintf("effort %s: no cloud receiver is connected; %s's CLI is on this machine and runs in an aayu worktree", orDash(effort), r)}, nil
		}
	}
	return Choice{}, fmt.Errorf("no receiver is available here in the mode asked for: %w", ErrNotConnected)
}

func orDash(s string) string {
	if s == "" {
		return "unset"
	}
	return s
}
