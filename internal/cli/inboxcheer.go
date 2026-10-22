package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/ui"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

// inboxParty is how long the confetti falls; the words stay until a key.
const inboxParty = 1500 * time.Millisecond

// inboxCheer is the short celebration when a person clears a set of
// housekeeping or the inbox (Story #798): a line for the status, one in a
// seat's voice, and, with motion, a burst of confetti over the card pane.
// It never blocks: any key carries on, and it never follows a failure.
type inboxCheer struct {
	at          time.Time
	head, voice string
	seat        verbs.SeatID
}

// cheerFor is the celebration for n housekeeping items tidied, or, with
// zero true, for an inbox with nothing left.
func cheerFor(n int, zero bool) inboxCheer {
	if zero {
		return inboxCheer{head: "Inbox zero. Job well done.", seat: verbs.Founder,
			voice: "Nothing waits on you. Even a king may take a walk when the court is empty; the pod will knock when it needs you."}
	}
	return inboxCheer{head: fmt.Sprintf("Housekeeping cleared: %d tidied. Job well done.", n), seat: verbs.Auditor,
		voice: fmt.Sprintf("%s closed, each with its trail. I went looking for a finding and came back empty-handed.", upperFirst(countWords(n, "loose end", "loose ends")))}
}

// countWords is "one loose end", "three loose ends", or "12 loose ends".
func countWords(n int, one, many string) string {
	words := []string{"no", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine", "ten"}
	s := fmt.Sprint(n)
	if n < len(words) {
		s = words[n]
	}
	return s + " " + plural(n, one, many)
}

// celebrate starts the cheer; its words are also the status.
func (m *inboxModel) celebrate(c inboxCheer) {
	c.at = m.now()
	m.cheer, m.status = &c, c.head
}

// cheerView is the card pane while the cheer shows: its words, then, while
// the confetti falls, the burst in the room below them.
func (m *inboxModel) cheerView(w, h int) []string {
	c := m.cheer
	who := seatIcons[c.seat] + " " + m.twinName(c.seat)
	if m.twin != nil {
		who += ", " + verbs.SeatName(c.seat)
	}
	out := []string{"", m.style(ui.OK, inboxBold.Render("🎉 Job well done")), ""}
	out = append(out, m.wrap(strings.TrimSuffix(c.head, " Job well done."), w)...)
	out = append(out, "", m.style(ui.Seat, who))
	for _, l := range m.wrap(c.voice, w-2) {
		out = append(out, m.style(ui.Muted, "▏")+" "+l)
	}
	out = append(out, "", m.style(ui.Muted, "Any key carries on."))
	if t := m.now().Sub(c.at); m.motion && t < inboxParty {
		out = append(out, confetti(m, w, h-len(out), t)...)
	}
	return out
}

// confetti is h rows of w cells of falling pieces at time t into the burst:
// each piece has its own column, start, speed, glyph and colour, from a
// fixed hash, so a frame is the same on every run.
func confetti(m *inboxModel, w, h int, t time.Duration) []string {
	if w <= 0 || h <= 0 {
		return nil
	}
	glyphs := []string{"*", "+", "•", "·", "✦", "°", "~"}
	roles := []ui.Role{ui.OK, ui.Warn, ui.Accent, ui.Seat, ui.Title}
	grid := make([][]string, h)
	for r := range grid {
		grid[r] = make([]string, w)
	}
	frac := float64(t) / float64(inboxParty)
	for i := range w * h / 10 {
		x := uint64(i+1) * 0x9E3779B97F4A7C15 // splitmix64: neighbouring pieces land apart
		x = (x ^ x>>30) * 0xBF58476D1CE4E5B9
		x = (x ^ x>>27) * 0x94D049BB133111EB
		x ^= x >> 31
		col := int(x % uint64(w))
		start := -int(x >> 16 % uint64(h+1))
		row := start + int(frac*float64(h+2)*(1+float64(x>>32%3)/2))
		if row < 0 || row >= h {
			continue
		}
		grid[row][col] = m.style(roles[int(x>>40%uint64(len(roles)))], glyphs[int(x>>48%uint64(len(glyphs)))])
	}
	out := make([]string, h)
	for r, cells := range grid {
		var b strings.Builder
		for _, c := range cells {
			if c == "" {
				c = " "
			}
			b.WriteString(c)
		}
		out[r] = b.String()
	}
	return out
}
