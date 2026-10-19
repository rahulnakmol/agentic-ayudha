package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/harmonica"
	"github.com/charmbracelet/x/ansi"
	gh "github.com/google/go-github/v90/github"
	"github.com/sahilm/fuzzy"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/ui"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

// The inbox on a person's terminal: one card at a time with a slim list
// beside it (above it on a phone). The card says what the item is in one
// plain line, gives the seat's counsel, and offers numbered choices with the
// recommendation first and preselected: Enter does it. Every write is one of
// inboxAct's, under the person's own token and the session's one approval
// (humanCaller), and reads its item again first. It opens from the last
// gather and refreshes behind the card; nothing in Update or View reads the
// network or a file, so a frame stays far inside 120 fps. --plain,
// TERM=dumb, and [ui] plain keep the line-by-line inbox (plainInbox).
//
// Motion is short and only visual: the next card slides in, the list's
// highlight glides (a harmonica spring), a done item shows ✓ before it
// leaves the list, and a refused action shakes the card. A key acts on the
// state at once, whatever is still moving. NO_COLOR and [ui] motion = false
// keep it still; colours come from the [ui] theme's roles only.

// inboxRefreshEvery is how often the open inbox asks GitHub what changed.
const inboxRefreshEvery = 45 * time.Second

// Motion timings: one frame at 120 fps, and how long each movement lasts.
const (
	inboxFrame = time.Second / 120
	inboxSlide = 180 * time.Millisecond // the next card
	inboxFlash = 250 * time.Millisecond // ✓ on a done item, before it leaves
	inboxShake = 250 * time.Millisecond // a refused action
)

// inboxTUI reports whether `aayu inbox` runs the full-screen inbox.
func (e *Env) inboxTUI(cfg *config.Config) bool {
	if e.Plain || e.Getenv("AAYU_PLAIN") == "1" || cfg.UI.Plain || e.Getenv("TERM") == "dumb" {
		return false
	}
	if e.TUI != nil {
		return true
	}
	in, ok := e.In.(*os.File)
	if !ok || !e.isTerminal(in) {
		return false
	}
	out, ok := e.Out.(*os.File)
	return ok && e.isTerminal(out)
}

// runInboxTUI runs the full-screen inbox. Each step's output (what decide,
// ship, and the inbox's own steps print) is shown in the status line as it
// comes, and printed in full once the inbox closes.
func (e *Env) runInboxTUI(ctx context.Context, cfg *config.Config, src *inboxSource, login string, work *inboxWork) error {
	out, errOut, repo := e.Out, e.Err, src.repo
	var mu sync.Mutex
	var send func(tea.Msg)
	sink := &lineSink{send: func(s string) {
		mu.Lock()
		f := send
		mu.Unlock()
		if f != nil {
			f(inboxLogMsg(s))
		}
	}}
	m := newInboxModel(repo, login, cfg.UI.Theme, e.Getenv("NO_COLOR") != "")
	m.work = work
	m.now, m.opened = e.Now, e.Now()
	m.recordDone = func(key string, d inboxDone) { e.recordInboxDone(repo, login, key, d) }
	src.progress = func(what string, done, total int) {
		mu.Lock()
		f := send
		mu.Unlock()
		if f != nil {
			f(inboxProgressMsg{what, done, total})
		}
	}
	defer func() { src.progress = nil }()
	if c := e.readInboxCache(repo, login); c != nil {
		for k, d := range c.Done { // written from an earlier session: done until GitHub, read since, still lists it
			m.done[k], m.state[k] = d, "done"
		}
		m.merge(c.Items)
		m.loading, m.fresh = false, c.At
		m.status = "From the last read, " + ago(e.Now(), c.At) + "; reading what changed…"
	}
	m.now = e.Now
	m.gather = func() inboxGatheredMsg {
		at := e.Now()
		before := src.c.Stats()
		changed, err := src.refresh(ctx)
		var items []InboxItem
		if err == nil {
			items, err = src.items(ctx)
		}
		after := src.c.Stats()
		if err == nil {
			e.writeInboxCache(inboxCache{Repo: repo, Login: login, At: e.Now(), Items: items})
		}
		return inboxGatheredMsg{items: items, changed: changed, err: err, calls: after.Calls() - before.Calls(), notModified: after.NotChanged - before.NotChanged, at: at}
	}
	m.open, m.twin = e.openURL, e.twin
	m.act = func(it InboxItem, a InboxAction) (InboxItem, string, error) {
		sink.buf.Reset()
		e.Out, e.Err, e.out, e.talk = sink, sink, nil, nil
		defer func() { e.Out, e.Err, e.out, e.talk = out, errOut, nil, nil }()
		fresh, err := e.inboxApply(ctx, cfg, src, login, it, a)
		sink.flush()
		return fresh, sink.buf.String(), err
	}
	run := e.TUI
	if run == nil {
		m.timer = true
		m.motion = (cfg.UI.Motion == nil || *cfg.UI.Motion) && e.Getenv("NO_COLOR") == ""
		run = func(tm tea.Model) error {
			p := tea.NewProgram(tm, tea.WithInput(e.In), tea.WithOutput(out), tea.WithContext(ctx), tea.WithFPS(120))
			mu.Lock()
			send = p.Send
			mu.Unlock()
			_, err := p.Run()
			return err
		}
	}
	if err := run(m); err != nil {
		return err
	}
	if m.err != nil && len(m.items) == 0 {
		return Wrap(ExitExternal, m.err)
	}
	if len(m.items) == 0 {
		fmt.Fprintf(out, "%s: nothing waits for you.\n", repo)
		return nil
	}
	for _, l := range m.log {
		fmt.Fprintln(out, l)
	}
	done, skipped := 0, 0
	for _, s := range m.state {
		switch s {
		case "done":
			done++
		case "skipped":
			skipped++
		}
	}
	return finishInbox(out, done, skipped, len(m.items)-done-skipped)
}

// ago is how long since t, in round words.
func ago(now, t time.Time) string {
	switch d := now.Sub(t); {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%d min ago", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%d h ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%d days ago", int(d.Hours()/24))
	}
}

// lineSink keeps what a step prints and passes each line on as it comes.
type lineSink struct {
	buf     bytes.Buffer
	partial []byte
	send    func(string)
}

func (s *lineSink) Write(p []byte) (int, error) {
	s.buf.Write(p)
	s.partial = append(s.partial, p...)
	for {
		i := bytes.IndexByte(s.partial, '\n')
		if i < 0 {
			break
		}
		s.send(string(s.partial[:i]))
		s.partial = s.partial[i+1:]
	}
	return len(p), nil
}

func (s *lineSink) flush() {
	if len(s.partial) > 0 {
		s.send(string(s.partial))
		s.partial = nil
	}
}

type (
	inboxGatheredMsg struct {
		items              []InboxItem
		changed            bool
		err                error
		calls, notModified int
		at                 time.Time // when the read began
	}
	inboxDoneMsg struct {
		key   string
		fresh InboxItem
		out   string
		err   error
	}
	inboxLogMsg      string
	inboxWorkTickMsg struct{} // redraws a write's stage once a second, also without motion
	inboxTickMsg     struct{}
	inboxFrameMsg    time.Time
	inboxProgressMsg struct {
		what        string
		done, total int
	}
)

// inboxJob is one write the person asked for.
type inboxJob struct {
	it InboxItem
	a  InboxAction
}

// inboxAsk is a confirmation on screen: what Enter does, in words.
type inboxAsk struct {
	title string
	lines []string
	jobs  []inboxJob
	// prompt, when set, asks for the person's one line (InboxAction.Ask)
	// first: text is what they typed, and Enter with none writes nothing.
	prompt, text string
}

// inboxHit is one line of the finder: an item (or the housekeeping line), or
// a choice on the card being read.
type inboxHit struct {
	row    int // the list row it jumps to, or -1
	item   int // the item it jumps to (housekeeping opens one by one), or -1
	choice int // the card's choice index it selects, or -1
	text   string
	icon   string
	at     []int  // matched byte offsets in text
	plain  string // text cut to the width
	shown  string // the same, its matched letters underlined
}

// inboxZone is a place the mouse can click: a list row or a choice.
type inboxZone struct {
	x0, x1, y0, y1 int
	row, choice    int // -1 when not that kind
	hit            int // a finder line, or -1
}

type inboxModel struct {
	repo, login string
	gather      func() inboxGatheredMsg
	act         func(InboxItem, InboxAction) (InboxItem, string, error)
	open        func(string)
	twin        func(verbs.SeatID) string
	now         func() time.Time

	items      []InboxItem       // needs you first, then housekeeping
	state      map[string]string // by key: "done", "skipped", "not done", "relisted" (done, and GitHub read since still lists it)
	done       map[string]inboxDone
	work       *inboxWork              // the write under way, as its GitHub requests show it
	recordDone func(string, inboxDone) // keeps a done write for later sessions
	rows       []int                   // the list: item indices, then -1 for the housekeeping line
	expanded   bool                    // the housekeeping shown one item per row

	cur, sel  int // the list row, and the choice selected on its card
	armed     bool
	ask       *inboxAsk
	help      bool
	finding   bool
	query     string
	hits      []inboxHit
	hit       int
	findTotal int

	opened              time.Time // when the session began
	loading, refreshing bool
	fresh               time.Time // when the items were last read from GitHub
	every               time.Duration
	timer               bool // refresh every so often (a real program)
	err                 error

	queue      []inboxJob
	total      int
	failed     int
	quitting   bool
	status     string
	log        []string
	refreshLog string // the last refresh, in words: calls and 304s

	w, h    int
	wrapped map[inboxWrapKey][]string
	p       *ui.Printer
	theme   string // [ui] theme, chosen again once the terminal says dark or light
	bgKnown bool   // the terminal answered the background query
	spin    spinner.Model
	zones   []inboxZone
	frame   string
	dirty   bool

	load inboxProgressMsg // how far the first read has got

	// Motion: what moves and since when. Only View reads it; keys act on
	// the state at once.
	motion    bool
	ticking   bool
	cardKey   string
	cardIn    time.Time
	refusal   string    // the status a refused action set, shown in the Fail role while it stands
	refusedAt time.Time // the card shakes for inboxShake after it
	flash     map[string]time.Time
	glide     float64 // the list highlight's row, springing to cur
	glideV    float64
	spring    harmonica.Spring
}

func newInboxModel(repo, login, theme string, noColor bool) *inboxModel {
	p := ui.New(nil, ui.Signals{TTY: true, NoColor: noColor, Theme: theme})
	p.Theme = ui.ThemeFor(theme, true)
	return &inboxModel{repo: repo, login: login, state: map[string]string{}, done: map[string]inboxDone{}, work: &inboxWork{}, recordDone: func(string, inboxDone) {}, loading: true, w: 80, h: 24, every: inboxRefreshEvery,
		spin: spinner.New(spinner.WithSpinner(spinner.Dot)), p: p, theme: theme, dirty: true, now: time.Now,
		flash: map[string]time.Time{}, spring: harmonica.NewSpring(harmonica.FPS(120), 30, 1)}
}

func (m *inboxModel) Init() tea.Cmd {
	m.refreshing = true
	return tea.Batch(m.spinTick(), m.gatherCmd(), tea.RequestBackgroundColor)
}

// spinTick starts the spinner; still, there is none.
func (m *inboxModel) spinTick() tea.Cmd {
	if !m.motion {
		return nil
	}
	return m.spin.Tick
}

// spinner is the spinner's frame and a space, or nothing when still.
func (m *inboxModel) spinner() string {
	if !m.motion {
		return ""
	}
	return m.spin.View() + " "
}

func (m *inboxModel) gatherCmd() tea.Cmd {
	g := m.gather
	return func() tea.Msg { return g() }
}

// tick asks for the next refresh; without a timer (a test drives the
// model and sends inboxTickMsg itself) it is nil.
func (m *inboxModel) tick() tea.Cmd {
	if !m.timer {
		return nil
	}
	return tea.Tick(m.every, func(time.Time) tea.Msg { return inboxTickMsg{} })
}

// merge takes a fresh gather in place: an item keeps its row and its card,
// a done item stays (marked done), one that no longer waits leaves, and a
// new one joins its part of the list. The cursor stays on its item.
func (m *inboxModel) merge(fresh []InboxItem) {
	was := m.cur
	defer func() {
		if m.item() >= 0 || was == m.cur {
			m.glide += float64(m.cur - was)
		}
	}()
	selected := ""
	if i := m.item(); i >= 0 {
		selected = m.items[i].key()
	}
	byKey := map[string]InboxItem{}
	for _, it := range fresh {
		byKey[it.key()] = it
	}
	var out []InboxItem
	seen := map[string]bool{}
	for _, it := range m.items {
		k := it.key()
		if f, ok := byKey[k]; ok {
			out, seen[k] = append(out, f), true
		} else if m.state[k] == "done" && !m.done[k].At.Before(m.opened) { // done this session: it stays, marked
			out, seen[k] = append(out, it), true
		}
	}
	for _, it := range fresh {
		if !seen[it.key()] {
			out = append(out, it)
		}
	}
	needs, house := splitInbox(out)
	m.items = append(needs, house...)
	m.layoutRows()
	m.cur = 0
	for r, i := range m.rows {
		if i >= 0 && m.items[i].key() == selected {
			m.cur = r
		}
	}
	if selected == "" || m.item() < 0 || m.items[m.item()].key() != selected {
		m.cur = min(m.cur, max(len(m.rows)-1, 0))
		m.sel, m.armed = 0, false
	}
}

// relist marks a done item that a read begun after its write still lists:
// it may still need a person, so it is offered again, after a confirmation
// that names the earlier write.
func (m *inboxModel) relist(fresh []InboxItem, at time.Time) {
	changed := false
	for _, it := range fresh {
		k := it.key()
		if d, ok := m.done[k]; ok && m.state[k] == "done" && !at.IsZero() && at.After(d.At) {
			m.state[k], changed = "relisted", true
		}
	}
	if changed {
		m.relayout()
	}
}

// layoutRows lists the needs-you items, the housekeeping one by one when
// expanded, and the housekeeping line.
// A done item leaves the list, after its ✓ when there is motion.
func (m *inboxModel) layoutRows() {
	m.rows = m.rows[:0]
	house := 0
	shown := func(it InboxItem) bool { return m.state[it.key()] != "done" || m.flashing(it.key()) } // done rows come last
	for i, it := range m.items {
		switch {
		case !shown(it):
		case !it.kind().Housekeeping:
			m.rows = append(m.rows, i)
		default:
			house++
		}
	}
	if m.expanded {
		for i, it := range m.items {
			if it.kind().Housekeeping && shown(it) {
				m.rows = append(m.rows, i)
			}
		}
	}
	if house > 0 {
		m.rows = append(m.rows, -1)
	}
	for i, it := range m.items { // done, after the rest: who did it and when, never offered again
		if m.state[it.key()] == "done" && !m.flashing(it.key()) {
			m.rows = append(m.rows, i)
		}
	}
}

// relayout lays the list out again with the cursor on the same item.
func (m *inboxModel) relayout() {
	was, key, onHouse := m.cur, "", m.item() < 0
	if i := m.item(); i >= 0 {
		key = m.items[i].key()
	}
	m.layoutRows()
	for r, i := range m.rows {
		if (i >= 0 && m.items[i].key() == key) || (i < 0 && onHouse) {
			m.cur = r
			m.glide += float64(r - was)
			return
		}
	}
	m.cur = min(m.cur, max(len(m.rows)-1, 0))
	m.sel, m.armed = 0, false
}

func (m *inboxModel) flashing(key string) bool {
	t, ok := m.flash[key]
	return ok && m.now().Sub(t) < inboxFlash
}

// refuse says why an action did not happen, in the Fail role, and shakes
// the card.
func (m *inboxModel) refuse(s string) {
	m.status, m.refusal, m.refusedAt = s, s, m.now()
}

// item is the item under the cursor, or -1 (the housekeeping line, or none).
func (m *inboxModel) item() int {
	if m.cur < 0 || m.cur >= len(m.rows) {
		return -1
	}
	return m.rows[m.cur]
}

// left counts what still waits: the cards that need a person, and the
// housekeeping. It allocates nothing, for View.
func (m *inboxModel) left() (needs, house int) {
	for i := range m.items {
		it := &m.items[i]
		switch {
		case m.state[it.key()] == "done":
		case inboxKinds[it.Kind].Housekeeping:
			house++
		default:
			needs++
		}
	}
	return needs, house
}

func (m *inboxModel) houseLeft() int { _, n := m.left(); return n }

// house is the housekeeping still to do.
func (m *inboxModel) house() []InboxItem {
	var out []InboxItem
	for _, it := range m.items {
		if it.kind().Housekeeping && m.state[it.key()] != "done" {
			out = append(out, it)
		}
	}
	return out
}

// choices are the card's choices: an item's, or the housekeeping line's.
func (m *inboxModel) choices() []InboxAction {
	if i := m.item(); i >= 0 {
		return m.items[i].choices()
	}
	_, n := m.left()
	if n == 0 {
		return nil
	}
	return []InboxAction{{N: 1, Label: fmt.Sprintf("Tidy all %d", n), Command: "tidy", Recommended: true},
		{N: 2, Label: "Go through them one by one", Command: "expand"}}
}

func (m *inboxModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := m.update(msg)
	return m, tea.Batch(cmd, m.animate())
}

// animate notes a new card and asks for the next frame while anything
// moves; still, it does nothing.
func (m *inboxModel) animate() tea.Cmd {
	if !m.motion {
		m.glide, m.glideV = float64(m.cur), 0
		return nil
	}
	now := m.now()
	key := ""
	switch i := m.item(); {
	case len(m.rows) == 0:
	case i >= 0:
		key = m.items[i].key()
	default:
		key = "housekeeping"
	}
	if key != m.cardKey {
		m.cardKey, m.cardIn = key, now
	}
	if m.ticking || !m.moving(now) {
		return nil
	}
	m.ticking = true
	return tea.Tick(inboxFrame, func(t time.Time) tea.Msg { return inboxFrameMsg(t) })
}

func (m *inboxModel) moving(now time.Time) bool {
	return now.Sub(m.cardIn) < inboxSlide || now.Sub(m.refusedAt) < inboxShake || len(m.flash) > 0 ||
		math.Abs(m.glide-float64(m.cur)) > 0.01 || math.Abs(m.glideV) > 0.01
}

// step advances the motion one frame: the highlight's spring, and the ✓ of
// done items, which leave the list when it lapses.
func (m *inboxModel) step() {
	m.glide, m.glideV = m.spring.Update(m.glide, m.glideV, float64(m.cur))
	if math.Abs(m.glide-float64(m.cur)) < 0.01 && math.Abs(m.glideV) < 0.01 {
		m.glide, m.glideV = float64(m.cur), 0
	}
	gone := false
	for k := range m.flash {
		if !m.flashing(k) {
			delete(m.flash, k)
			gone = true
		}
	}
	if gone {
		m.relayout()
	}
}

func (m *inboxModel) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case inboxFrameMsg:
		m.ticking, m.dirty = false, true
		m.step()
	case tea.BackgroundColorMsg:
		m.bgKnown, m.dirty = true, true
		m.p.Theme = ui.ThemeFor(m.theme, msg.IsDark())
	case inboxProgressMsg:
		if m.loading {
			m.load, m.dirty = msg, true
		}
	case tea.WindowSizeMsg:
		m.w, m.h, m.dirty = msg.Width, msg.Height, true
		if m.finding {
			m.find()
		}
	case spinner.TickMsg:
		if !m.motion || !m.loading && !m.refreshing && len(m.queue) == 0 {
			return nil
		}
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		m.dirty = true
		return cmd
	case inboxGatheredMsg:
		m.dirty = true
		return m.gathered(msg)
	case inboxTickMsg:
		if m.refreshing || len(m.queue) > 0 {
			return m.tick()
		}
		m.refreshing, m.dirty = true, true
		return tea.Batch(m.spinTick(), m.gatherCmd())
	case inboxWorkTickMsg:
		if len(m.queue) > 0 && m.timer {
			m.dirty = true
			return tea.Tick(time.Second, func(time.Time) tea.Msg { return inboxWorkTickMsg{} })
		}
	case inboxLogMsg:
		if s := strings.TrimSpace(string(msg)); s != "" {
			m.status, m.dirty = s, true
		}
	case inboxDoneMsg:
		m.dirty = true
		return m.finished(msg)
	case tea.KeyPressMsg:
		m.dirty = true
		return m.key(msg)
	case tea.MouseClickMsg:
		m.dirty = true
		return m.click(msg.Mouse())
	case tea.MouseWheelMsg:
		m.dirty = true
		switch msg.Mouse().Button {
		case tea.MouseWheelDown:
			m.move(1)
		case tea.MouseWheelUp:
			m.move(-1)
		}
	}
	return nil
}

// gathered takes a read of GitHub; a failed refresh keeps the cards and
// backs off when GitHub limits the rate.
func (m *inboxModel) gathered(msg inboxGatheredMsg) tea.Cmd {
	first := m.loading
	m.loading, m.refreshing = false, false
	if msg.err != nil {
		var rl *gh.RateLimitError
		var ab *gh.AbuseRateLimitError
		switch {
		case first && len(m.items) == 0:
			m.err = msg.err
			return tea.Quit
		case errors.Is(msg.err, github.ErrBudget):
			m.status = "Stopped refreshing: this session used its GitHub call budget. The cards stay; reopen the inbox for fresh ones."
			return nil
		case errors.As(msg.err, &rl) || errors.As(msg.err, &ab) || errors.Is(msg.err, github.ErrRateLimited):
			m.every = min(m.every*2, 10*time.Minute)
			m.status = fmt.Sprintf("GitHub is limiting requests; the next refresh is in %s.", m.every)
		default:
			m.status = "Could not refresh: " + msg.err.Error() + ". R tries again."
		}
		return m.tick()
	}
	m.err = nil
	m.every = inboxRefreshEvery
	m.fresh = m.now()
	m.refreshLog = fmt.Sprintf("%d GitHub %s, %d not modified", msg.calls, plural(msg.calls, "call", "calls"), msg.notModified)
	if msg.changed || first {
		m.merge(msg.items)
	}
	m.relist(msg.items, msg.at)
	if m.quitting {
		return tea.Quit
	}
	switch {
	case first && len(m.items) > 0:
		m.status = "Enter does the preselected choice."
	case strings.HasPrefix(m.status, "From the last read"):
		m.status = "Up to date."
	}
	return m.tick()
}

// finished records one write and starts the next.
func (m *inboxModel) finished(msg inboxDoneMsg) tea.Cmd {
	for _, l := range strings.Split(strings.TrimRight(msg.out, "\n"), "\n") {
		if strings.TrimSpace(l) != "" {
			m.log = append(m.log, l)
		}
	}
	job := m.queue[0]
	m.queue = m.queue[1:]
	num := job.it.Number
	var ch errInboxChanged
	switch {
	case errors.As(msg.err, &ch):
		m.work.end()
		m.failed++
		if ch.Item.Number == 0 { // it no longer waits
			m.items = slices.DeleteFunc(m.items, func(it InboxItem) bool { return it.key() == msg.key })
			m.layoutRows()
			m.cur = min(m.cur, max(len(m.rows)-1, 0))
		} else {
			for i := range m.items {
				if m.items[i].key() == msg.key {
					m.items[i] = ch.Item
				}
			}
		}
		m.sel, m.armed = 0, false
		m.refuse(fmt.Sprintf("#%d: %v.", num, msg.err))
		m.queue = nil // the rest were asked for against what the person read
	case msg.err != nil:
		_, stage, _, _ := m.work.snapshot()
		wrote := m.work.end()
		m.state[msg.key] = "not done"
		m.failed++
		if len(wrote) > 0 { // part of it landed: say what, so a retry is a choice made knowing it
			msg.err = fmt.Errorf("%w (stopped while %s, after it %s)", msg.err, stage, strings.Join(wrote, ", "))
		}
		m.refuse(fmt.Sprintf("#%d not done: %v", num, msg.err))
		m.log = append(m.log, "  not done: #"+strconv.Itoa(num)+": "+msg.err.Error())
	default:
		m.state[msg.key] = "done"
		d := inboxDone{By: m.login, At: m.now().UTC(), Choice: job.it.button(job.a), Wrote: m.work.end()}
		m.done[msg.key] = d
		m.recordDone(msg.key, d)
		if m.motion {
			m.flash[msg.key] = m.now()
		}
		m.status = fmt.Sprintf("Done: #%d, %s. It is written to GitHub%s.", num, job.a.Label, wroteSuffix(d.Wrote))
		if i := m.item(); i >= 0 && m.items[i].key() == msg.key && len(m.queue) == 0 {
			m.next()
		}
		m.relayout()
	}
	if m.quitting {
		m.queue = nil
		return tea.Quit
	}
	if len(m.queue) > 0 {
		return m.run()
	}
	if m.total > 1 {
		m.status = fmt.Sprintf("Applied %d: %d done, %d not done. Written changes are not undone here.", m.total, m.total-m.failed, m.failed)
		if m.item() < 0 || m.state[m.items[m.item()].key()] != "" {
			m.next()
		}
	}
	m.total, m.failed = 0, 0
	return nil
}

// next moves to the next card still waiting, after the cursor, wrapping.
func (m *inboxModel) next() {
	m.sel, m.armed = 0, false
	for d := 1; d <= len(m.rows); d++ {
		r := (m.cur + d) % len(m.rows)
		i := m.rows[r]
		if (i < 0 && m.houseLeft() > 0) || (i >= 0 && m.state[m.items[i].key()] == "") {
			m.cur = r
			return
		}
	}
}

// run starts the first queued write.
func (m *inboxModel) run() tea.Cmd {
	j, act := m.queue[0], m.act
	m.work.begin(j.it.key(), m.now())
	var redraw tea.Cmd
	if m.timer {
		redraw = tea.Tick(time.Second, func(time.Time) tea.Msg { return inboxWorkTickMsg{} })
	}
	m.status = fmt.Sprintf("Applying #%d…", j.it.Number)
	if m.total > 1 {
		m.status = fmt.Sprintf("Applying %d of %d: #%d…", m.total-len(m.queue)+1, m.total, j.it.Number)
	}
	return tea.Batch(m.spinTick(), redraw, func() tea.Msg {
		fresh, out, err := act(j.it, j.a)
		return inboxDoneMsg{j.it.key(), fresh, out, err}
	})
}

// start queues jobs; with none under way, the first starts now. A job asked
// for while another runs waits its turn: the screen stays live meanwhile.
func (m *inboxModel) start(jobs []inboxJob) tea.Cmd {
	if len(m.queue) > 0 {
		m.queue = append(m.queue, jobs...)
		m.total = max(m.total, 1) + len(jobs)
		m.status = fmt.Sprintf("Queued #%d; it runs after the write under way.", jobs[0].it.Number)
		return nil
	}
	m.queue, m.total, m.failed = jobs, len(jobs), 0
	return m.run()
}

// queued reports whether key is the write under way or waits in the queue.
func (m *inboxModel) queued(key string) (running, waiting bool) {
	for i, j := range m.queue {
		if j.it.key() == key {
			return i == 0, i > 0
		}
	}
	return false, false
}

func wroteSuffix(w []string) string {
	if len(w) == 0 {
		return ""
	}
	return ": " + strings.Join(w, ", ")
}

func (m *inboxModel) key(msg tea.KeyPressMsg) tea.Cmd {
	k := msg.String()
	busy := m.loading && len(m.items) == 0
	writing := len(m.queue) > 0
	switch {
	case (busy || writing) && (k == "ctrl+c" || k == "q") && m.ask == nil && !m.finding && !m.help:
		// Bubble Tea does not wait for a running command when it quits: the
		// write (or read) under way finishes, the rest are not started.
		m.quitting = true
		m.status = "Quitting after the step under way; the rest are not done."
		return nil
	case k == "ctrl+c":
		return tea.Quit
	case busy:
		return nil // nothing to act on before the first read
	case m.ask != nil && m.ask.prompt != "":
		switch k {
		case "enter":
			if strings.TrimSpace(m.ask.text) == "" {
				m.refuse("Say what you checked first; nothing was written.")
				return nil
			}
			jobs := m.ask.jobs
			jobs[0].a.Answer = strings.TrimSpace(m.ask.text)
			m.ask = nil
			return m.start(jobs)
		case "esc":
			m.ask, m.status = nil, "Not applied; nothing was written."
		case "backspace":
			if r := []rune(m.ask.text); len(r) > 0 {
				m.ask.text = string(r[:len(r)-1])
			}
		default:
			m.ask.text += msg.Key().Text
		}
		return nil
	case m.ask != nil:
		switch k {
		case "enter", "y", "Y":
			jobs := m.ask.jobs
			m.ask = nil
			return m.start(jobs)
		case "esc", "n", "N", "q":
			m.ask, m.status = nil, "Not applied; nothing was written."
		}
		return nil
	case m.help:
		if k == "?" || k == "esc" || k == "q" {
			m.help = false
		}
		return nil
	case m.finding:
		return m.findKey(msg)
	}
	if len(m.rows) == 0 {
		if k == "q" || k == "esc" {
			return tea.Quit
		}
		return nil
	}
	switch k {
	case "q":
		return tea.Quit
	case "?":
		m.help = true
	case "j", "down":
		m.move(1)
	case "k", "up":
		m.move(-1)
	case "esc":
		if m.armed {
			m.armed, m.status = false, "Not applied; nothing was written."
		}
	case "enter":
		return m.enter()
	case "s":
		if i := m.item(); i >= 0 && m.state[m.items[i].key()] == "" {
			m.state[m.items[i].key()] = "skipped"
			m.status = fmt.Sprintf("Skipped #%d for now; j and k come back to it.", m.items[i].Number)
		}
		m.next()
	case "A":
		m.askTidy()
	case "/", "ctrl+k":
		m.finding, m.query, m.hit = true, "", 0
		m.find()
	case "o":
		if i := m.item(); i >= 0 {
			m.open(m.items[i].URL)
			m.status = "Opened " + m.items[i].URL
		}
	case "tab":
		m.cycle(1)
	case "shift+tab":
		m.cycle(-1)
	case "R":
		if writing {
			m.status = "A refresh waits for the write under way; it reads GitHub again after it."
		} else if !m.refreshing {
			m.refreshing, m.status = true, "Reading what changed on GitHub…"
			return tea.Batch(m.spinTick(), m.gatherCmd())
		}
	default:
		if len(k) == 1 && k[0] >= '1' && k[0] <= '9' {
			m.pick(int(k[0] - '0'))
		}
	}
	return nil
}

// move moves the cursor by n rows; the new card preselects its first choice.
func (m *inboxModel) move(n int) {
	c := min(max(m.cur+n, 0), max(len(m.rows)-1, 0))
	if c != m.cur {
		m.cur, m.sel, m.armed = c, 0, false
	}
}

// cycle moves to the next (or previous) item that needs you and is not done,
// wrapping: Tab and Shift-Tab.
func (m *inboxModel) cycle(dir int) {
	n := len(m.rows)
	for d := 1; d <= n; d++ {
		r := ((m.cur+dir*d)%n + n) % n
		i := m.rows[r]
		if i >= 0 && !m.items[i].kind().Housekeeping && m.state[m.items[i].key()] != "done" {
			if r != m.cur {
				m.cur, m.sel, m.armed = r, 0, false
			}
			return
		}
	}
	m.status = "Nothing else needs you; the housekeeping and done items stay in the list."
}

// pick selects the choice numbered n; Enter then does it.
func (m *inboxModel) pick(n int) {
	for i, c := range m.choices() {
		if c.N == n {
			m.sel, m.armed = i, false
			m.status = fmt.Sprintf("Choice %d selected; Enter does it.", n)
			return
		}
	}
	m.refuse(fmt.Sprintf("This card has no choice %d.", n))
}

// enter does the selected choice: at once when it is the recommendation of
// a two-way step or opens GitHub, else after a second Enter that reads what
// it does.
func (m *inboxModel) enter() tea.Cmd {
	cs := m.choices()
	if len(cs) == 0 {
		return nil
	}
	c := cs[min(m.sel, len(cs)-1)]
	i := m.item()
	if i < 0 {
		switch c.Command {
		case "tidy":
			m.askTidy()
		case "expand":
			m.expanded = true
			m.layoutRows()
			m.status = "Housekeeping, one item per card. A still tidies all of it."
			m.next()
		}
		return nil
	}
	it := m.items[i]
	running, waiting := m.queued(it.key())
	d, acted := m.done[it.key()]
	switch {
	case running:
		m.refuse(fmt.Sprintf("#%d is being written now; it is not started twice.", it.Number))
	case waiting:
		m.refuse(fmt.Sprintf("#%d is queued already; it runs after the write under way.", it.Number))
	case m.state[it.key()] == "done":
		m.refuse(fmt.Sprintf("#%d is %s; it is written to GitHub and not offered again.", it.Number, d.line(m.now())))
	case acted && c.Command != "open" && !m.armed: // relisted: GitHub still lists it after the write
		m.armed = true
		m.status = fmt.Sprintf("You already did this: %s. GitHub still lists #%d. Enter again does it again.", d.line(m.now()), it.Number)
	case c.Command == "open":
		m.open(it.URL)
		m.status = "Opened " + it.URL
	case c.Ask != "":
		m.ask = &inboxAsk{title: fmt.Sprintf("%s #%d: %s", it.kind().Word, it.Number, it.button(c)), lines: []string{inboxEffect(it, c)}, jobs: []inboxJob{{it, c}}, prompt: c.Ask}
		m.status = "Nothing is written until you type what you checked and press Enter."
	case (!c.Recommended || it.OneWay) && !m.armed:
		m.armed = true
		m.status = "Nothing is written until you press Enter again."
	default:
		m.armed = false
		return m.start([]inboxJob{{it, c}})
	}
	return nil
}

// askTidy asks to do every housekeeping recommendation: one confirmation.
func (m *inboxModel) askTidy() {
	var jobs []inboxJob
	var lines []string
	for _, it := range m.house() {
		if it.tidy() {
			jobs = append(jobs, inboxJob{it, *it.recommended()})
			lines = append(lines, fmt.Sprintf("%s #%d: %s", it.kind().Icon, it.Number, it.recommended().Label))
		}
	}
	if len(jobs) == 0 {
		m.refuse("No housekeeping is left to tidy.")
		return
	}
	m.ask = &inboxAsk{title: fmt.Sprintf("Tidy all %d housekeeping %s?", len(jobs), plural(len(jobs), "item", "items")),
		lines: append(lines, "", fmt.Sprintf("Each is written to GitHub as @%s and is not undone here. Nothing in the code changes.", m.login)), jobs: jobs}
	m.status = "Nothing is written until you press Enter."
}

// find ranks every item, the housekeeping line, and the card's choices
// against the query, fzf-style (sahilm/fuzzy): the best match first and
// selected. It runs on each keystroke, never in View.
func (m *inboxModel) find() {
	var texts []string
	var base []inboxHit
	add := func(h inboxHit) { base, texts = append(base, h), append(texts, h.text) }
	for r, i := range m.rows {
		if i < 0 {
			add(inboxHit{row: r, choice: -1, item: -1, icon: inboxTidy.Icon, text: fmt.Sprintf("%d housekeeping · tidy all", m.houseLeft())})
		} else if !m.items[i].kind().Housekeeping {
			add(m.itemHit(i))
		}
	}
	for i, it := range m.items { // every housekeeping item, folded or not
		if it.kind().Housekeeping {
			add(m.itemHit(i))
		}
	}
	if i := m.item(); i >= 0 || m.houseLeft() > 0 {
		for ci, c := range m.choices() {
			label := c.Label
			if i >= 0 {
				label = m.items[i].button(c)
			}
			add(inboxHit{row: -1, choice: ci, item: -1, icon: "↳", text: fmt.Sprintf("choice %s: %s (this card)", c.ref(), label)})
		}
	}
	m.hits, m.findTotal = m.hits[:0], len(base)
	if m.query == "" {
		m.hits = append(m.hits, base...)
	} else {
		for _, f := range fuzzy.Find(m.query, texts) {
			h := base[f.Index]
			h.at = f.MatchedIndexes
			m.hits = append(m.hits, h)
		}
	}
	for i := range m.hits {
		m.hits[i].plain, m.hits[i].shown = m.highlight(m.hits[i])
	}
	m.hit = min(m.hit, max(len(m.hits)-1, 0))
}

// highlight cuts a finder line to the width and underlines its matched
// letters, once per keystroke or resize, never per frame.
func (m *inboxModel) highlight(h inboxHit) (string, string) {
	text := ansi.Truncate(h.text, max(m.w, 20)-6, "…")
	if len(h.at) == 0 {
		return text, text
	}
	var b strings.Builder
	at := 0
	for bi, r := range text {
		size := len(string(r))
		if at < len(h.at) && h.at[at] == bi {
			b.WriteString(inboxUnder.Render(text[bi : bi+size]))
			at++
		} else {
			b.WriteString(text[bi : bi+size])
		}
	}
	return text, b.String()
}

func (m *inboxModel) itemHit(i int) inboxHit {
	it := m.items[i]
	return inboxHit{row: -1, choice: -1, item: i, icon: it.kind().Icon,
		text: fmt.Sprintf("#%d %s · %s · %s", it.Number, it.Title, it.kind().Word, m.twinName(it.Seat))}
}

func (m *inboxModel) twinName(s verbs.SeatID) string {
	if m.twin == nil {
		return verbs.SeatName(s)
	}
	name, _, _ := strings.Cut(m.twin(s), " (")
	return name
}

func (m *inboxModel) findKey(msg tea.KeyPressMsg) tea.Cmd {
	switch k := msg.String(); k {
	case "esc":
		m.finding = false
	case "enter":
		m.jump(m.hit)
	case "up", "ctrl+p", "ctrl+k":
		m.hit = max(m.hit-1, 0)
	case "down", "ctrl+n", "ctrl+j", "tab":
		m.hit = min(m.hit+1, max(len(m.hits)-1, 0))
	case "backspace":
		if r := []rune(m.query); len(r) > 0 {
			m.query = string(r[:len(r)-1])
			m.hit = 0
			m.find()
		}
	default:
		if t := msg.Key().Text; t != "" {
			m.query += t
			m.hit = 0
			m.find()
		}
	}
	return nil
}

// jump goes to finder line h: its card, or its choice on this card. It never
// runs anything; Enter on the card does.
func (m *inboxModel) jump(h int) {
	m.finding = false
	if h < 0 || h >= len(m.hits) {
		return
	}
	hit := m.hits[h]
	if hit.item >= 0 {
		if !m.expanded && m.items[hit.item].kind().Housekeeping {
			m.expanded = true
			m.layoutRows()
		}
		hit.row = slices.Index(m.rows, hit.item)
	}
	if hit.row >= 0 {
		m.cur, m.sel, m.armed = hit.row, 0, false
		m.status = "Enter does the preselected choice."
		return
	}
	m.sel, m.armed = hit.choice, false
	m.status = fmt.Sprintf("Choice %s selected; Enter does it.", m.choices()[hit.choice].ref())
}

// click selects the row or choice under the mouse; a click on the selected
// choice does it, as Enter would.
func (m *inboxModel) click(ms tea.Mouse) tea.Cmd {
	if ms.Button != tea.MouseLeft {
		return nil
	}
	for _, z := range m.zones {
		if ms.X < z.x0 || ms.X >= z.x1 || ms.Y < z.y0 || ms.Y > z.y1 {
			continue
		}
		switch {
		case z.hit >= 0:
			m.jump(z.hit)
		case z.row >= 0:
			if z.row != m.cur {
				m.cur, m.sel, m.armed = z.row, 0, false
			}
		case z.choice >= 0 && z.choice == m.sel && m.ask == nil:
			return m.enter()
		case z.choice >= 0:
			m.sel, m.armed = z.choice, false
			m.status = "Selected; click it again, or press Enter, to do it."
		}
		return nil
	}
	return nil
}

func (m *inboxModel) View() tea.View {
	if m.dirty {
		m.frame, m.dirty = m.render(), false
	}
	v := tea.NewView(m.frame)
	// A named palette paints its own background and text colour, so it reads
	// as published on a terminal that is not themed. solarized waits for the
	// terminal's answer, which picks its dark or light half.
	if th := m.p.Theme; th.Bg != "" && m.p.Color && (th.Name != "solarized" || m.bgKnown) {
		v.BackgroundColor, v.ForegroundColor = lipgloss.Color(th.Bg), lipgloss.Color(th.Fg)
	}
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	v.WindowTitle = "aayu inbox: " + m.repo
	return v
}

func (m *inboxModel) style(r ui.Role, s string) string { return m.p.Style(r, s) }

// reverse shows s in reverse video, its trailing spaces too, which reads
// without colour; the row also starts with › for a screen reader.
func reverse(s string) string { return "\x1b[7m" + s + "\x1b[27m" }

// wrap breaks s between words to width columns, once per text and width:
// the text of a card changes when the data or the size does, not per frame.
func (m *inboxModel) wrap(s string, width int) []string {
	width = max(width, 10)
	k := inboxWrapKey{s, width}
	if l, ok := m.wrapped[k]; ok {
		return l
	}
	if m.wrapped == nil || len(m.wrapped) > 4096 {
		m.wrapped = map[inboxWrapKey][]string{}
	}
	l := strings.Split(ansi.Wrap(s, width, " "), "\n")
	m.wrapped[k] = l
	return l
}

type inboxWrapKey struct {
	s string
	w int
}

var (
	inboxBold  = lipgloss.NewStyle().Bold(true)
	inboxFaint = lipgloss.NewStyle().Faint(true)
	inboxUnder = lipgloss.NewStyle().Underline(true)
)

// render draws the frame: the head line, the list and the card side by side
// (stacked on a narrow terminal), the status line, and the keys.
func (m *inboxModel) render() string {
	m.zones = m.zones[:0]
	w, h := max(m.w, 20), max(m.h, 8)
	head := m.style(ui.Title, "aayu inbox")
	if !m.loading && len(m.items) > 0 {
		left, n := m.left()
		head += fmt.Sprintf(" · %d %s you", left, plural(left, "needs", "need"))
		if n > 0 {
			head += fmt.Sprintf(" · %d housekeeping", n)
		}
	}
	if ansi.StringWidth(head)+3+len(m.repo) <= w {
		head += " · " + m.repo
	}
	body := h - 3
	var lines []string
	sh := m.cardShift()
	shift := strings.Repeat(" ", sh)
	switch {
	case m.loading && len(m.items) == 0:
		lines = m.skeleton(w, body)
	case len(m.rows) == 0:
		lines = []string{"", m.style(ui.Title, "Nothing waits for you."), "", "Every human-only step is done. q quits."}
	case m.ask != nil:
		lines = m.askView(w, body)
	case m.help:
		lines = m.helpView(w)
	case m.finding:
		lines = m.findView(w, body)
	case w >= 100:
		lw := min(max(w/3, 30), 44)
		list := m.listView(lw, body, 0, 1)
		card := m.cardView(w-lw-3, body, lw+3, 1)
		for r := range body {
			l, c := "", ""
			if r < len(list) {
				l = list[r]
			}
			if r < len(card) {
				c = shift + ansi.Truncate(card[r], w-lw-3-sh, "")
			}
			lines = append(lines, l+strings.Repeat(" ", max(lw-ansi.StringWidth(l), 0))+" "+m.style(ui.Muted, "│")+" "+c)
		}
	default:
		n := min(len(m.rows), 5, max(body/4, 2))
		lines = m.listView(w, n, 0, 1)
		lines = append(lines, m.style(ui.Muted, strings.Repeat("─", w)))
		for _, l := range m.cardView(w, body-n-1, 0, n+2) {
			lines = append(lines, shift+ansi.Truncate(l, w-sh, ""))
		}
	}
	if len(lines) > body {
		lines = lines[:body]
	}
	for len(lines) < body {
		lines = append(lines, "")
	}
	var b strings.Builder
	b.Grow(w * (h + 1) * 2)
	b.WriteString(ansi.Truncate(head, w, "…"))
	for _, l := range lines {
		b.WriteByte('\n')
		b.WriteString(ansi.Truncate(l, w, "…"))
	}
	b.WriteByte('\n')
	b.WriteString(m.statusLine(w))
	b.WriteByte('\n')
	b.WriteString(m.keysLine(w))
	return b.String()
}

// skeleton is the first read's screen: the shape of the list and the card
// in muted blocks, and how far the read has got.
func (m *inboxModel) skeleton(w, h int) []string {
	block := func(n int) string { return m.style(ui.Muted, strings.Repeat("░", max(n, 1))) }
	say := []string{"", m.spinner() + "Reading the issues and pull requests of " + m.repo + "…"}
	switch l := m.load; {
	case l.total > 0:
		say = append(say, fmt.Sprintf("%s %d of %d items checked", m.bar(l.done, l.total, 16), l.done, l.total))
	case l.what != "":
		say = append(say, l.what+"…")
	}
	list := []string{"  " + block(2) + " " + block(22) + "  " + block(4), "  " + block(2) + " " + block(17), "  " + block(2) + " " + block(25), "  " + block(2) + " " + block(13)}
	card := []string{block(18), block(34), block(44), "", block(30), block(40), block(36), "", "  " + block(26), "  " + block(18)}
	var out []string
	if w >= 100 {
		lw := min(max(w/3, 30), 44)
		right := append(slices.Clone(card), say...)
		for r := range min(len(right), h) {
			l := ""
			if r < len(list) {
				l = list[r]
			}
			out = append(out, l+strings.Repeat(" ", max(lw-ansi.StringWidth(l), 0))+" "+m.style(ui.Muted, "│")+" "+right[r])
		}
		return out
	}
	out = append(append(out, list[:3]...), m.style(ui.Muted, strings.Repeat("─", w)))
	return append(append(out, card...), say...)
}

// cardShift is how far right the card sits this frame: a short slide as a
// new card comes in, a small shake after a refused action, else none.
func (m *inboxModel) cardShift() int {
	if !m.motion {
		return 0
	}
	now := m.now()
	if d := now.Sub(m.refusedAt); d < inboxShake {
		return [...]int{0, 2, 0, 2, 0, 1, 0}[int(d*7/inboxShake)]
	}
	if d := now.Sub(m.cardIn); d < inboxSlide {
		p := 1 - float64(d)/float64(inboxSlide)
		return int(math.Round(4 * p * p))
	}
	return 0
}

// bar is a progress bar of width cells; the counts beside it say the same
// in words.
func (m *inboxModel) bar(done, total, width int) string {
	full := min(done*width/max(total, 1), width)
	return m.style(ui.OK, strings.Repeat("█", full)) + m.style(ui.Muted, strings.Repeat("░", width-full))
}

// listView is the list: one row per card, the cursor row reversed (and
// marked › for a screen reader), the housekeeping folded into one row.
// x and y place it on the screen, for the mouse.
func (m *inboxModel) listView(w, h, x, y int) []string {
	off := 0
	if len(m.rows) > h {
		off = min(max(m.cur-h/2, 0), len(m.rows)-h)
	}
	numW := 0
	for _, i := range m.rows {
		if i >= 0 {
			numW = max(numW, len(strconv.Itoa(m.items[i].Number))+1)
		}
	}
	hi := m.cur // the highlight: on the cursor, or gliding to it
	if m.motion {
		hi = min(max(int(math.Round(m.glide)), 0), max(len(m.rows)-1, 0))
	}
	var out []string
	for r := off; r < len(m.rows) && r < off+h; r++ {
		i := m.rows[r]
		var icon, text, num string
		muted, done := false, false
		if i < 0 {
			icon = inboxTidy.Icon
			if n := m.houseLeft(); n > 0 {
				text = fmt.Sprintf("%d housekeeping · A tidies all", n)
			} else {
				text, done = "✓ housekeeping done", true
			}
		} else {
			it := m.items[i]
			icon, text, num = it.kind().Icon, it.Title, "#"+strconv.Itoa(it.Number)
			running, waiting := m.queued(it.key())
			switch st := m.state[it.key()]; {
			case running:
				text = m.spinner() + "writing · " + text
			case waiting:
				text, muted = "queued · "+text, true
			case st == "done":
				text, done = "✓ "+m.done[it.key()].line(m.now())+" · "+text, true
			case st == "relisted":
				text, muted = "still listed after you acted · "+text, true
			case st == "":
			default:
				text, muted = st+" · "+text, true
			}
		}
		room := max(w-2-3-numW-1, 4)
		text = ansi.Truncate(text, room, "…")
		row := icon + " " + text + strings.Repeat(" ", max(room-ansi.StringWidth(text), 0)) + " " + fmt.Sprintf("%*s", numW, num)
		switch {
		case r == hi:
			row = reverse("› " + row)
		case done:
			row = "  " + m.style(ui.OK, row)
		case muted:
			row = "  " + m.style(ui.Muted, row)
		default:
			row = "  " + row
		}
		m.zones = append(m.zones, inboxZone{x0: x, x1: x + w, y0: y + len(out), y1: y + len(out), row: r, choice: -1, hit: -1})
		out = append(out, row)
	}
	return out
}

// cardView is the card for the cursor row: kind and number, the title, its
// one plain line, the twin's counsel with its evidence, and the choices,
// the selected one reversed. The counsel gives way first when it is short of
// room; ? shows all of it.
func (m *inboxModel) cardView(w, h, x, y int) []string {
	i := m.item()
	var kind inboxKind
	var title, tag, counselText, evidence string
	var seat verbs.SeatID
	var it InboxItem
	if i >= 0 {
		it = m.items[i]
		kind, title, seat, counselText, evidence = it.kind(), it.Title, it.Seat, it.Counsel, it.Evidence
		tag = fmt.Sprintf("%s %s · #%d", kind.Icon, kind.Word, it.Number)
		if it.OneWay {
			tag += " · one-way door"
		}
		if it.Provisional {
			tag += " · provisional"
		}
		running, waiting := m.queued(it.key())
		switch st := m.state[it.key()]; {
		case running:
			tag += " · writing now"
		case waiting:
			tag += " · queued"
		case st == "relisted":
			tag += " · still listed after you acted"
		case st != "":
			tag += " · " + st
		}
	} else {
		house := m.house()
		kind, seat = inboxTidy, inboxTidy.Seat
		tag = inboxTidy.Icon + " " + inboxTidy.Word
		title = fmt.Sprintf("%d housekeeping %s", len(house), plural(len(house), "item", "items"))
		if len(house) == 0 {
			title = "Housekeeping is done"
		} else {
			counselText = tidyCounsel(house)
		}
	}
	wrapTo := m.wrap
	top := []string{m.style(ui.Muted, tag)}
	for _, l := range wrapTo(title, w) {
		top = append(top, inboxBold.Render(l))
	}
	top = append(top, wrapTo(kind.Line, w)...) // what it means, in the text colour: it is read, not skimmed
	var counselLines []string
	if counselText != "" {
		who := seatIcons[seat] + " " + m.twinName(seat) + ", counsel, not approval"
		counselLines = append(counselLines, "", m.style(ui.Seat, who))
		for _, l := range wrapTo(counselText, w-2) {
			counselLines = append(counselLines, m.style(ui.Muted, "▏")+" "+l)
		}
		if evidence != "" {
			counselLines = append(counselLines, m.style(ui.Muted, "▏")+" "+m.style(ui.Muted, "Evidence: ")+ansi.Truncate(evidence, w-12, "…"))
		}
	}
	var choiceLines []string
	choiceAt := map[int]int{} // a choice's first line
	if i >= 0 {
		top = append(top, m.writeLines(it, w)...)
	}
	choiceLines = append(choiceLines, "")
	for ci, c := range m.choices() {
		label := c.Label
		if i >= 0 {
			label = it.button(c)
		}
		tail := ""
		if c.Recommended {
			tail = "recommended"
		}
		room := w - 7
		ls := wrapTo(label, room)
		if last := ls[len(ls)-1]; tail != "" && ansi.StringWidth(last)+2+len(tail) > room {
			ls = append(ls, "")
		}
		choiceAt[ci] = len(choiceLines)
		for li, l := range ls {
			num := "   "
			if li == 0 {
				num = fmt.Sprintf("%-3s", c.ref())
			}
			pad := room - ansi.StringWidth(l)
			if li == len(ls)-1 && tail != "" {
				l += strings.Repeat(" ", pad-len(tail)) + tail
				pad = 0
			}
			row := num + l + strings.Repeat(" ", max(pad, 0)) + " "
			switch {
			case ci == m.sel && li == 0:
				row = reverse("› " + row)
			case ci == m.sel:
				row = reverse("  " + row)
			default:
				row = "  " + row
			}
			choiceLines = append(choiceLines, row)
		}
	}
	if m.armed && i >= 0 {
		c := m.choices()[min(m.sel, len(m.choices())-1)]
		choiceLines = append(choiceLines, "")
		for _, l := range wrapTo(inboxEffect(it, c), w) {
			choiceLines = append(choiceLines, m.style(ui.Warn, l))
		}
		choiceLines = append(choiceLines, inboxBold.Render("Enter again applies it · Esc goes back"))
	}
	// Fit: the counsel gives way, then the title.
	if extra := len(top) + len(counselLines) + len(choiceLines) - h; extra > 0 && len(counselLines) > 3 {
		keep := max(len(counselLines)-extra-1, 3)
		if keep < len(counselLines) {
			counselLines = append(counselLines[:keep], m.style(ui.Muted, "▏ … ? shows all of it"))
		}
	}
	out := append(append(top, counselLines...), choiceLines...)
	base := y + len(top) + len(counselLines)
	for ci, at := range choiceAt {
		if at+len(top)+len(counselLines) < h {
			m.zones = append(m.zones, inboxZone{x0: x, x1: x + w, y0: base + at, y1: base + at, row: -1, choice: ci, hit: -1})
		}
	}
	return out
}

// writeLines is what the card says about a write on its item: its stage while
// it runs, or who did it, when, and what it wrote.
func (m *inboxModel) writeLines(it InboxItem, w int) []string {
	k := it.key()
	if running, _ := m.queued(k); running {
		_, stage, calls, started := m.work.snapshot()
		return append([]string{""}, m.wrapStyled(ui.Warn, fmt.Sprintf("%sWriting now: %s · %d GitHub %s · %s. You can read and move meanwhile.",
			m.spinner(), stage, calls, plural(calls, "call", "calls"), elapsed(m.now().Sub(started))), w)...)
	}
	d, ok := m.done[k]
	if !ok {
		return nil
	}
	role, say := ui.OK, "✓ "+upperFirst(d.line(m.now()))+wroteSuffix(d.Wrote)+"."
	if m.state[k] == "relisted" {
		role, say = ui.Warn, upperFirst(d.line(m.now()))+wroteSuffix(d.Wrote)+". GitHub, read since, still lists it, so it may still need you: Enter asks before doing it again."
	}
	return append([]string{""}, m.wrapStyled(role, say, w)...)
}

func (m *inboxModel) wrapStyled(r ui.Role, s string, w int) []string {
	var out []string
	for _, l := range m.wrap(s, w) {
		out = append(out, m.style(r, l))
	}
	return out
}

// elapsed is a short duration in words: 8 s, 1 min 24 s.
func elapsed(d time.Duration) string {
	d = d.Round(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("%d s", int(d.Seconds()))
	}
	return fmt.Sprintf("%d min %d s", int(d.Minutes()), int(d.Seconds())%60)
}

func (m *inboxModel) askView(w, h int) []string {
	a := m.ask
	out := []string{"", inboxBold.Render(a.title), ""}
	lines := a.lines
	if room := h - 6; len(lines) > room && room > 2 {
		lines = append(slices.Clone(lines[:room-1]), fmt.Sprintf("… and %d more", len(lines)-room+1))
	}
	for _, l := range lines {
		out = append(out, m.wrap(l, w)...)
	}
	if a.prompt != "" {
		return append(out, "", inboxBold.Render(a.prompt+" ")+a.text+"█", "", inboxBold.Render("Enter applies it · Esc cancels"))
	}
	return append(out, "", inboxBold.Render("Enter applies all · Esc cancels"))
}

// helpView is ? : the plain explainer of what is on the card, its glossary
// terms, and every key.
func (m *inboxModel) helpView(w int) []string {
	k := inboxTidy
	if i := m.item(); i >= 0 {
		k = m.items[i].kind()
	}
	wrapTo := func(s string) []string { return m.wrap(s, w-2) }
	out := []string{"", inboxBold.Render(strings.TrimSpace(k.Icon + " " + k.Word + ": what this means"))}
	out = append(out, wrapTo(k.Explain)...)
	out = append(out, m.style(ui.Muted, "Glossary: "+strings.Join(k.Terms, ", ")+" in GLOSSARY.md"), "", inboxBold.Render("Keys"))
	for _, l := range []string{"Enter   do the selected choice (another choice or a", "        one-way door asks for Enter again)", "1-9     select a choice", "s       skip to the next card",
		"Tab     next item that needs you (Shift-Tab: previous)", "j/k     move (↓/↑ and the mouse wheel too)", "/       find any item or choice (Ctrl+K too)", "A       tidy all housekeeping, after one confirmation",
		"o       open on GitHub     R  refresh now     q  quit"} {
		out = append(out, ansi.Truncate(l, w, "…"))
	}
	if m.refreshLog != "" {
		out = append(out, m.style(ui.Muted, "Last refresh: "+m.refreshLog+"."))
	}
	return out
}

// findView is the finder: the query, how many match, and the ranked lines
// with their matched letters underlined.
func (m *inboxModel) findView(w, h int) []string {
	out := []string{"", inboxBold.Render("Find: ") + m.query + "█" + m.style(ui.Muted, fmt.Sprintf("   %d of %d", len(m.hits), m.findTotal)), ""}
	room := h - 4
	off := 0
	if m.hit >= room {
		off = m.hit - room + 1
	}
	for hi := off; hi < len(m.hits) && hi < off+room; hi++ {
		hit := m.hits[hi]
		text := hit.shown
		if hi == m.hit {
			text = hit.plain
		}
		row := "  " + hit.icon + " " + text
		if hi == m.hit {
			row = reverse("› " + hit.icon + " " + text + strings.Repeat(" ", max(w-5-ansi.StringWidth(text), 0)))
		}
		m.zones = append(m.zones, inboxZone{x0: 0, x1: w, y0: 1 + len(out), y1: 1 + len(out), row: -1, choice: -1, hit: hi})
		out = append(out, row)
	}
	if len(m.hits) == 0 {
		out = append(out, m.style(ui.Muted, "Nothing matches. Backspace edits; Esc goes back."))
	}
	return out
}

// statusLine is the last result on the left, and how fresh the cards are on
// the right when it fits.
func (m *inboxModel) statusLine(w int) string {
	left := m.status
	if m.refusal != "" && left == m.refusal {
		left = m.style(ui.Fail, left)
	}
	if len(m.queue) > 0 || m.loading && len(m.items) > 0 {
		left = m.spinner() + left
	}
	if len(m.queue) > 0 && m.refusal != left {
		_, stage, calls, started := m.work.snapshot()
		left = fmt.Sprintf("Writing #%d: %s · %d %s · %s", m.queue[0].it.Number, stage, calls, plural(calls, "call", "calls"), elapsed(m.now().Sub(started)))
		left = m.spinner() + left
	}
	right := ""
	switch {
	case m.total > 1 && len(m.queue) > 0:
		done := m.total - len(m.queue)
		right = fmt.Sprintf("%s %d/%d", m.bar(done, m.total, min(12, max(w/8, 4))), done, m.total)
	case m.refreshing && !m.loading:
		right = m.spinner() + "refreshing"
	case !m.fresh.IsZero():
		right = "updated " + ago(m.now(), m.fresh)
	}
	if lw, rw := ansi.StringWidth(left), ansi.StringWidth(right); right != "" && lw+rw+3 <= w {
		return left + strings.Repeat(" ", w-lw-rw) + m.style(ui.Muted, right)
	}
	return ansi.Truncate(left, w, "…")
}

// keysLine is the few keys that work now, in words; at a narrow width the
// least needed go first, never cut in half.
func (m *inboxModel) keysLine(w int) string {
	type kb struct {
		key, what string
		prio      int
	}
	var keys []kb
	switch {
	case len(m.rows) == 0:
		keys = []kb{{"q", "quit", 0}}
	case m.ask != nil && m.ask.prompt != "":
		keys = []kb{{"type", "what you checked", 1}, {"Enter", "apply it", 0}, {"Esc", "cancel", 0}}
	case m.ask != nil:
		keys = []kb{{"Enter", "apply all", 0}, {"Esc", "cancel", 0}}
	case m.help:
		keys = []kb{{"?/Esc", "close", 0}}
	case m.finding:
		keys = []kb{{"type", "to find", 1}, {"↑↓", "move", 2}, {"Enter", "jump", 0}, {"Esc", "back", 0}}
	case m.armed:
		keys = []kb{{"Enter", "apply it", 0}, {"Esc", "back", 0}, {"1-9", "pick another", 1}}
	case len(m.queue) > 0:
		keys = []kb{{"Tab", "next item", 1}, {"j/k", "read others", 0}, {"Enter", "queue it", 1}, {"?", "help", 2}, {"q", "quit after this write", 0}}
	default:
		keys = []kb{{"Enter", "do it", 0}, {"Tab", "next", 1}, {"1-9", "pick", 2}, {"s", "skip", 2}, {"j/k", "move", 3}, {"/", "find", 2}, {"A", "tidy", 4}, {"?", "help", 1}, {"q", "quit", 0}}
		if m.houseLeft() == 0 {
			keys = slices.DeleteFunc(keys, func(k kb) bool { return k.key == "A" })
		}
	}
	for p := 5; p >= 0; p-- {
		var parts []string
		width := 0
		for _, k := range keys {
			if k.prio <= p {
				parts = append(parts, inboxBold.Render(k.key)+" "+k.what)
				width += len([]rune(k.key)) + 1 + len(k.what) + 3
			}
		}
		if width-3 <= w || p == 0 {
			return ansi.Truncate(strings.Join(parts, inboxFaint.Render(" · ")), w, "")
		}
	}
	return ""
}
