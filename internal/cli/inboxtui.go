package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
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

// inboxReadTimeout bounds one read of GitHub, the first or a refresh, above
// the client's longest rate-limit wait (github.Options.MaxWait, 2 minutes):
// a read GitHub never finishes ends in errInboxSlow, and R tries again
// (Bug #822). A variable so tests can shorten it.
var inboxReadTimeout = 3 * time.Minute

// errInboxSlow is a read of GitHub that ran past inboxReadTimeout.
var errInboxSlow = errors.New("GitHub did not answer the inbox's read in time; run aayu inbox again")

// inboxReading is the status line while R reads GitHub; a finished read replaces it.
const inboxReading = "Reading what changed on GitHub…"

// inboxSlowFirst is the status line when the first read timed out with nothing read.
const inboxSlowFirst = "GitHub did not answer in time, so nothing was read yet. R tries again; q quits."

// Motion timings: one frame at 120 fps, and how long each movement lasts.
const (
	inboxFrame = time.Second / 120
	inboxSlide = 180 * time.Millisecond // the next card
	inboxFlash = 250 * time.Millisecond // ✓ on a done item, before it leaves
	inboxShake = 250 * time.Millisecond // a refused action
)

// Spacing, in terminal cells (display width: an emoji icon is two). Every
// line of the frame is built from these, so each column, the pane divider,
// the group headers, and the key bar start at the same cell on every row.
const (
	inboxPad     = 1   // inner padding: the left and right edge of each pane, and of the head, status, and key lines
	inboxGutter  = 2   // between two columns of the list
	inboxWide    = 100 // at this width and above the list and the card sit side by side; below it they stack
	inboxShare   = 59  // the list pane's share of a wide frame, in per cent
	inboxMarkW   = 5   // the cursor ›, a space, and the mark: [ ] [x] for housekeeping, ✓ ✗ - ! and the write's spinner
	inboxIconW   = 2   // the kind's icon
	inboxWhyW    = 22  // why it waits, in a few words
	inboxAgeW    = 3   // how long since it changed: 12m, 3h, 2d
	inboxWhyMin  = 60  // the list's content width that has room for the why column
	inboxAgeMin  = 48  // and for the age column
	inboxWordCut = 6   // a list cell cut short ends at a word when that drops no more than this many cells

	inboxCursorW = 2             // a card choice's cursor: "› "
	inboxTag     = "recommended" // the tag of the recommended choice, right-aligned at the card's edge
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
// signinFailed ends aayu inbox when its sign-in failed before any card
// showed: the cause in a person's words (signinWords), and GitHub's own on
// the log line above it.
func signinFailed(out io.Writer, err error) error {
	fmt.Fprintln(out, "  sign-in failed: "+err.Error())
	next := "you open aayu inbox again"
	if strings.Contains(err.Error(), "no GitHub token") {
		next = "gh auth login"
	}
	return WithNext(Errorf(CodeOf(err), "%s", signinWords(err)), next)
}

func (e *Env) runInboxTUI(ctx context.Context, cfg *config.Config, repo string, signin func(context.Context) (*inboxSource, string, error)) error {
	out := e.Out
	var mu sync.Mutex
	var send func(tea.Msg)
	post := func(msg tea.Msg) {
		mu.Lock()
		f := send
		mu.Unlock()
		if f != nil {
			f(msg)
		}
	}
	sink := &lineSink{send: func(s string) { post(inboxLogMsg(s)) }}
	// The session is approved inside the inbox, on its first screen: the
	// presence check shows its code there instead of on stderr, and the
	// inbox is read once it is approved.
	var src *inboxSource
	login := ""
	m := newInboxModel(repo, login, cfg.UI.Theme, e.Getenv("NO_COLOR") != "")
	m.now, m.opened, m.dryRun = e.Now, e.Now(), e.DryRun
	sctx, cancel := context.WithCancel(ctx)
	e.showCode = func(_ context.Context, code deviceCodeReply, expires time.Time) error {
		post(inboxCodeMsg{code, expires})
		return nil
	}
	// Bubble Tea does not wait for a sign-in still under way when it quits.
	// A quit stops it, waits for it to end, and only then takes the hook
	// away, so the presence check never falls back to printing the code; a
	// sign-in that starts after the inbox closed does not run.
	var signing sync.WaitGroup
	closed := false
	defer func() {
		cancel()
		mu.Lock()
		closed = true
		mu.Unlock()
		signing.Wait()
		e.showCode = nil
	}()
	m.signin = &inboxSignin{browser: browserHere(e.Getenv), tmuxClip: tmuxClipboard(e.Getenv), run: func() tea.Msg {
		mu.Lock()
		if closed {
			mu.Unlock()
			return inboxSignedMsg{err: errNotApproved}
		}
		signing.Add(1)
		mu.Unlock()
		defer signing.Done()
		s, l, err := signin(sctx)
		return inboxSignedMsg{s, l, err}
	}}
	defer func() {
		if src != nil {
			src.progress = nil
		}
	}()
	setup := func(s *inboxSource, l string) {
		src, login, m.login = s, l, l
		m.recordDone = func(key string, d inboxDone) { e.recordInboxDone(repo, login, key, d) }
		src.progress = func(what string, done, total int) { post(inboxProgressMsg{what, done, total}) }
		if c := e.readInboxCache(repo, login); c != nil {
			for k, d := range c.Done { // written from an earlier session: done until GitHub, read since, still lists it
				m.done[k], m.state[k] = d, "done"
			}
			m.merge(c.Items)
			m.loading, m.fresh = false, c.At
			m.status = "From the last read, " + ago(e.Now(), c.At) + "; reading what changed…"
		}
	}
	m.signedIn = func(s *inboxSource, l string) tea.Cmd {
		setup(s, l)
		m.refreshing = true
		return m.gatherCmd()
	}
	// A terminal with no agent marker needs no code: the session is
	// approved at once and the inbox opens from its last read, as before.
	if e.approval != nil || e.agentSignal() == "" && e.isTerminal(e.In) {
		s, l, err := signin(ctx)
		if err != nil {
			return signinFailed(out, err)
		}
		setup(s, l)
		m.signin = nil
	}
	m.gather = func() inboxGatheredMsg {
		at := e.Now()
		before := src.c.Stats()
		rctx, cancel := context.WithTimeoutCause(ctx, inboxReadTimeout, errInboxSlow)
		defer cancel()
		changed, err := src.refresh(rctx)
		var items []InboxItem
		if err == nil {
			items, err = src.items(rctx)
		}
		if err != nil && errors.Is(context.Cause(rctx), errInboxSlow) {
			err = errInboxSlow
		}
		after := src.c.Stats()
		if err == nil {
			e.writeInboxCache(inboxCache{Repo: repo, Login: login, At: e.Now(), Items: items})
		}
		return inboxGatheredMsg{items: items, changed: changed, err: err, calls: after.Calls() - before.Calls(), notModified: after.NotChanged - before.NotChanged, at: at}
	}
	m.open, m.twin = e.openURL, e.twin
	// Writes run in the background, up to inboxParallel at once: each on
	// its own copy of the Env with its own output, its GitHub requests
	// tagged with its own work (inboxWatch). What they share is safe to
	// share: the client serializes writes, the actions log and the ledger
	// lock, and the approval was taken once, above.
	_, _ = e.Actions()
	m.act = func(it InboxItem, a InboxAction, w *inboxWork) (InboxItem, string, error) {
		ws := &lineSink{send: sink.send}
		we := *e
		we.Out, we.Err, we.out, we.talk = ws, ws, nil, nil
		we.showCode = nil // a write never takes over the screen; the session's approval covers it
		fresh, err := we.inboxApply(context.WithValue(ctx, inboxWorkKey{}, w), cfg, src, login, it, a)
		ws.flush()
		return fresh, ws.buf.String(), err
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
	err := run(m)
	for _, sv := range m.saves { // Bubble Tea does not wait for its commands when it quits
		sv.once.Do(func() { m.recordDone(sv.key, sv.d) })
	}
	if err != nil {
		return err
	}
	// The exits with no card say it in plain words; the log line above them
	// keeps GitHub's own, as a failed card's details do.
	if m.signin != nil && m.signin.err != nil {
		return signinFailed(out, m.signin.err)
	}
	if m.err != nil && len(m.items) == 0 {
		fmt.Fprintln(out, "  read failed: "+m.err.Error())
		return WithNext(Errorf(ExitExternal, "aayu inbox couldn't read GitHub (%s), so nothing was shown or changed", plainCause(m.err)), "you open aayu inbox again")
	}
	if len(m.items) == 0 {
		fmt.Fprintf(out, "%s: nothing waits for you.\n", repo)
		return nil
	}
	for _, l := range m.log {
		fmt.Fprintln(out, l)
	}
	done, skipped, left := 0, 0, 0
	for _, it := range m.items {
		switch k := it.key(); m.state[k] {
		case "done": // a completion from an earlier session is not this session's work
			if !m.done[k].At.Before(m.opened) {
				done++
			}
		case "gone":
		case "skipped":
			skipped++
		default:
			left++
		}
	}
	return finishInbox(out, done, skipped, left)
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
	it    InboxItem
	a     InboxAction
	batch int        // the jobs asked for together, under one confirmation or one Enter
	w     *inboxWork // the write under way; nil while it waits its turn
}

// inboxSave is a done write's record in the cache: written by a command off
// the update loop, and once more on the way out if that command has not run,
// so a quick q never loses it.
type inboxSave struct {
	key  string
	d    inboxDone
	once sync.Once
}

// inboxBatch is how far a bulk write has got, for the "4 of 17 done" line.
type inboxBatch struct{ total, done, failed int }

// inboxParallel is how many writes run at once; the rest wait their turn.
const inboxParallel = 2

// How the inbox quits while writes run: never mid-write.
const (
	quitNo      = iota
	quitCurrent // once the writes running now finish; the queued ones are not started
	quitAll     // once every write, the queued ones too, has finished
)

// inboxAsk is a confirmation on screen: what Enter does, in words.
type inboxAsk struct {
	title string
	lines []string
	jobs  []inboxJob
	// prompt, when set, asks for the person's one line (InboxAction.Ask)
	// first: text is what they typed, and Enter with none writes nothing.
	prompt, text string
	quit         bool // the question q asks while writes run
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
	act         func(InboxItem, InboxAction, *inboxWork) (InboxItem, string, error)
	open        func(string)
	twin        func(verbs.SeatID) string
	now         func() time.Time

	items      []InboxItem       // needs you first, then housekeeping
	state      map[string]string // by key: "done", "skipped", "not done", "relisted" (done, and GitHub read since still lists it)
	done       map[string]inboxDone
	recordDone func(string, inboxDone)            // keeps a done write for later sessions
	rows       []int                              // the list: item indices, group by group (inboxGroups)
	selected   map[string]bool                    // housekeeping picked with x or X for one bulk action; never a needs-you item
	batch      int                                // the last batch of jobs started
	fails      map[string]inboxFail               // the writes that stopped this session, by item
	notes      map[string]string                  // an item that changed on GitHub before its write: how, in words
	batches    map[int]*inboxBatch                // the bulk writes under way
	workTick   bool                               // the once-a-second redraw of the writes runs
	readAfter  bool                               // R was pressed during writes: read GitHub again once they end
	dryRun     bool                               // --dry-run: a write is planned, never done
	saves      []*inboxSave                       // the done writes' records in the cache, written once each before aayu exits
	signin     *inboxSignin                       // the session's approval, while it is asked for (inboxlogin.go); nil once approved
	signedIn   func(*inboxSource, string) tea.Cmd // sets the session up for the approved login and reads GitHub
	live       bool                               // the items come from a read of GitHub this session, so one that leaves it is gone

	cur, sel  int // the list row, and the choice selected on its card
	armed     bool
	ask       *inboxAsk
	help      bool
	details   bool // d: GitHub's own facts under each card's plain lines
	finding   bool
	query     string
	hits      []inboxHit
	hit       int
	findTotal int

	opened              time.Time // when the session began
	loading, refreshing bool
	stalled             bool      // the first read timed out with nothing read: R reads again
	fresh               time.Time // when the items were last read from GitHub
	every               time.Duration
	timer               bool // refresh every so often (a real program)
	err                 error

	queue      []inboxJob
	quitting   int // quitNo, quitCurrent, or quitAll
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
	return &inboxModel{repo: repo, login: login, state: map[string]string{}, done: map[string]inboxDone{}, selected: map[string]bool{}, fails: map[string]inboxFail{}, notes: map[string]string{}, batches: map[int]*inboxBatch{}, recordDone: func(string, inboxDone) {}, loading: true, w: 80, h: 24, every: inboxRefreshEvery,
		spin: spinner.New(spinner.WithSpinner(spinner.Dot)), p: p, theme: theme, dirty: true, now: time.Now,
		flash: map[string]time.Time{}, spring: harmonica.NewSpring(harmonica.FPS(120), 30, 1)}
}

func (m *inboxModel) Init() tea.Cmd {
	if m.signin != nil { // the inbox is read once the session is approved
		return tea.Batch(m.spinTick(), m.signinCmd(), tea.RequestBackgroundColor)
	}
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
		switch f, ok := byKey[k]; {
		case ok:
			out, seen[k] = append(out, f), true
			if m.state[k] == "gone" {
				delete(m.state, k)
			}
		case m.state[k] == "done" && !m.done[k].At.Before(m.opened): // done this session: it stays, marked
			out, seen[k] = append(out, it), true
		case m.live || m.state[k] == "gone": // it left GitHub's list while the person read it: it stays, marked gone
			out, seen[k] = append(out, it), true
			m.state[k] = "gone"
			delete(m.selected, k)
		}
	}
	for _, it := range fresh {
		if !seen[it.key()] {
			out = append(out, it)
		}
	}
	needs, house, waiting := splitInbox(out)
	m.items = append(append(needs, waiting...), house...)
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

// The list's groups, in order: what only a person decides, what agents
// stopped on, the housekeeping (the only part that can be selected), and what
// this session settled.
var inboxGroups = [...]struct{ name, about string }{
	{"Needs you now", "one at a time"},
	{"Not ready yet", "nothing for you; d says why"},
	{"Waiting on the review cap", "agents stopped; you look next"},
	{"Housekeeping", "x select · X all · C clear all"},
	{"Done this session", "written here, or gone from GitHub"},
}

// group is the list group of item i (an index into inboxGroups). A done item
// keeps its place while its ✓ shows, then moves to the last group.
func (m *inboxModel) group(i int) int {
	it := m.items[i]
	switch st := m.state[it.key()]; {
	case st == "gone" || st == "done" && !m.flashing(it.key()):
		return 4
	case it.kind().Housekeeping:
		return 3
	case it.Kind == InboxCapped:
		return 2
	case it.waiting():
		return 1
	}
	return 0
}

// layoutRows lists the items group by group.
func (m *inboxModel) layoutRows() {
	m.rows = m.rows[:0]
	for g := range inboxGroups {
		for i := range m.items {
			if m.group(i) == g {
				m.rows = append(m.rows, i)
			}
		}
	}
}

// relayout lays the list out again with the cursor on the same item.
func (m *inboxModel) relayout() {
	was, key := m.cur, ""
	if i := m.item(); i >= 0 {
		key = m.items[i].key()
	}
	m.layoutRows()
	for r, i := range m.rows {
		if m.items[i].key() == key {
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

// item is the item under the cursor, or -1 when the list is empty.
func (m *inboxModel) item() int {
	if m.cur < 0 || m.cur >= len(m.rows) {
		return -1
	}
	return m.rows[m.cur]
}

// left counts what still waits: the items that need a person, and the
// housekeeping. It allocates nothing, for View.
func (m *inboxModel) left() (needs, house int) {
	for i := range m.items {
		it := &m.items[i]
		switch st := m.state[it.key()]; {
		case st == "done" || st == "gone" || it.Kind == InboxNotReady:
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
		if st := m.state[it.key()]; it.kind().Housekeeping && st != "done" && st != "gone" {
			out = append(out, it)
		}
	}
	return out
}

// choices are the choices of the item under the cursor, without one that
// failed in a way pressing it again cannot fix.
func (m *inboxModel) choices() []InboxAction {
	i := m.item()
	if i < 0 {
		return nil
	}
	it := m.items[i]
	f, failed := m.fails[it.key()]
	if !failed || m.state[it.key()] != "not done" || f.why.Retry {
		return it.choices()
	}
	return slices.DeleteFunc(it.choices(), func(c InboxAction) bool { return c.Command == f.a.Command })
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
	if cmd, ok := m.signinUpdate(msg); ok {
		return cmd
	}
	if s := m.signin; s != nil && (s.running || s.screen()) {
		switch msg := msg.(type) {
		case tea.KeyPressMsg:
			return m.signinKey(msg.String())
		case tea.MouseMsg:
			return nil
		}
	}
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
		if !m.motion || !m.loading && !m.refreshing && m.writing() == 0 {
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
		m.workTick = len(m.queue) > 0 && m.timer
		if m.workTick {
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
	m.loading, m.refreshing, m.stalled = false, false, false
	if msg.err != nil {
		var rl *gh.RateLimitError
		var ab *gh.AbuseRateLimitError
		slow := errors.Is(msg.err, errInboxSlow) || errors.Is(msg.err, github.ErrTimeout)
		switch {
		case slow && first && len(m.items) == 0:
			// Nothing read yet: the loading view stays, saying so, and R
			// reads again instead of the inbox quitting (Bug #822).
			if m.quitting != quitNo { // q during the read: quit, as a finished read would
				return tea.Quit
			}
			m.loading, m.stalled, m.load, m.status = true, true, inboxProgressMsg{}, inboxSlowFirst
			return m.tick()
		case first && len(m.items) == 0:
			m.err = msg.err
			return tea.Quit
		case slow:
			m.status = "GitHub did not answer in time, so nothing was refreshed; the cards are from the last read. R tries again."
		case errors.Is(msg.err, github.ErrBudget):
			m.status = "Stopped refreshing: this session used its GitHub call budget. The cards stay; reopen the inbox for fresh ones."
			return nil
		case errors.As(msg.err, &rl) || errors.As(msg.err, &ab) || errors.Is(msg.err, github.ErrRateLimited):
			m.every = min(m.every*2, 10*time.Minute)
			m.status = fmt.Sprintf("GitHub is limiting requests; the next refresh is in %s.", m.every)
		default:
			m.status = "Not refreshed: " + plainCause(msg.err) + ". R tries again."
			m.log = append(m.log, "  refresh failed: "+msg.err.Error())
		}
		return m.tick()
	}
	m.err = nil
	m.every = inboxRefreshEvery
	m.fresh = m.now()
	m.refreshLog = fmt.Sprintf("%d GitHub %s, %d not modified", msg.calls, plural(msg.calls, "call", "calls"), msg.notModified)
	// Every read is taken in: a card's pull request is read again on each
	// refresh, and a conflict or a finished check changes no updated_at.
	m.merge(msg.items)
	m.live = true
	m.relist(msg.items, msg.at)
	if m.quitting != quitNo && m.writing() == 0 {
		return tea.Quit
	}
	switch {
	case first && len(m.items) > 0:
		m.status = "Enter does the preselected choice."
	case strings.HasPrefix(m.status, "From the last read"), m.status == inboxReading:
		m.status = "Up to date."
	}
	if read := m.deferredRead(); read != nil { // R asked during the writes, and this read began before they ended
		return read
	}
	return m.tick()
}

// finished records one write, then starts what waits for its slot.
func (m *inboxModel) finished(msg inboxDoneMsg) tea.Cmd {
	for _, l := range strings.Split(strings.TrimRight(msg.out, "\n"), "\n") {
		if strings.TrimSpace(l) != "" {
			m.log = append(m.log, l)
		}
	}
	at := slices.IndexFunc(m.queue, func(j inboxJob) bool { return j.w != nil && j.it.key() == msg.key })
	if at < 0 {
		return nil
	}
	job := m.queue[at]
	m.queue = slices.Delete(m.queue, at, at+1)
	var save tea.Cmd // the done write's record in the cache
	b := m.batches[job.batch]
	if b == nil {
		b = &inboxBatch{total: 1}
	}
	num := job.it.Number
	var ch errInboxChanged
	var why inboxWhy // a failure in a person's words
	if msg.err != nil && !errors.As(msg.err, &ch) {
		stage, wrote := job.w.sofar()
		why = explainFailure(job.it, job.a, msg.err, stage, wrote)
	}
	switch {
	case errors.As(msg.err, &ch):
		job.w.end()
		b.failed++
		delete(m.selected, msg.key)
		if ch.Item.Number == 0 { // it no longer waits: it stays, marked gone
			m.state[msg.key] = "gone"
			m.relayout()
		} else {
			for i := range m.items {
				if m.items[i].key() == msg.key {
					m.items[i] = ch.Item
					for _, w := range ch.What { // the exact change goes behind the card (d), its plain words on it
						m.items[i].Details = append(m.items[i].Details, "What changed: "+w)
					}
				}
			}
		}
		if i := m.item(); i >= 0 && m.items[i].key() == msg.key {
			m.sel, m.armed = 0, false
		}
		m.notes[msg.key] = fmt.Sprintf("#%d changed on GitHub since you opened it: %s. Nothing was written; the card shows it as it is now.", num, changedWords(job.it, ch.Item))
		if len(ch.What) > 0 {
			m.log = append(m.log, "  changed since read: #"+strconv.Itoa(num)+": "+strings.Join(ch.What, "; "))
		}
		// The rest of its batch was asked for against what the person read;
		// an item queued on its own still runs.
		n := len(m.queue)
		m.queue = slices.DeleteFunc(m.queue, func(j inboxJob) bool { return j.w == nil && j.batch == job.batch })
		b.failed += n - len(m.queue)
		status := fmt.Sprintf("#%d changed on GitHub; nothing was written. The card says how.", num)
		if dropped := n - len(m.queue); dropped > 0 {
			status = fmt.Sprintf("#%d changed on GitHub; nothing was written, and the %d picked with it did not start.", num, dropped)
		}
		m.refuse(status)
	case why.Landed:
		// The merge landed; a step after it did not: done, with a warning.
		m.state[msg.key] = "done"
		delete(m.selected, msg.key)
		d := inboxDone{By: m.login, At: m.now().UTC(), Choice: job.it.button(job.a), Wrote: job.w.end(), Warning: []string{why.What, "Next: " + why.Next}, Detail: why.Detail}
		m.done[msg.key] = d
		sv := &inboxSave{key: msg.key, d: d}
		m.saves = append(m.saves, sv)
		record := m.recordDone
		save = func() tea.Msg { sv.once.Do(func() { record(sv.key, sv.d) }); return nil }
		b.done++
		m.status = fmt.Sprintf("✓ #%d is merged; one step after it didn't finish. The card says which.", num)
		m.log = append(m.log, "  merged, then stopped: #"+strconv.Itoa(num)+": "+logDetail(msg.err.Error(), why.Detail))
		m.relayout()
	case msg.err != nil:
		_, stage, _, _ := job.w.snapshot()
		wrote := job.w.end()
		m.state[msg.key] = "not done"
		m.fails[msg.key] = inboxFail{stage: stage, err: msg.err.Error(), wrote: wrote, a: job.a, why: why}
		if i := m.item(); i >= 0 && m.items[i].key() == msg.key { // the failed choice may have gone: keep the cursor on one that is left
			m.sel = min(m.sel, max(len(m.choices())-1, 0))
		}
		b.failed++
		if len(wrote) > 0 { // part of it landed: say what, so a retry is a choice made knowing it
			msg.err = fmt.Errorf("%w (stopped while %s, after it %s)", msg.err, stage, strings.Join(wrote, ", "))
		}
		did, _ := inboxVerb(job.it, job.a)
		m.refuse(fmt.Sprintf("✗ #%d not %s; the card says why.", num, did)) // the whole story is on the card, never cut here
		m.log = append(m.log, "  not done: #"+strconv.Itoa(num)+": "+logDetail(msg.err.Error(), why.Detail))
		m.readAfter = true // GitHub's view of it likely moved: the card should show it as it is now
	case m.dryRun: // planned, not written: the item still waits, and no later session sees it done
		job.w.end()
		b.done++
		m.status = fmt.Sprintf("Planned #%d (dry run): nothing was written to GitHub.", num)
	default:
		m.state[msg.key] = "done"
		delete(m.selected, msg.key)
		d := inboxDone{By: m.login, At: m.now().UTC(), Choice: job.it.button(job.a), Wrote: job.w.end()}
		m.done[msg.key] = d
		// The cache is a file: it is written off the update loop, unless
		// the inbox quits now and would not wait for it.
		sv := &inboxSave{key: msg.key, d: d}
		m.saves = append(m.saves, sv)
		record := m.recordDone
		save = func() tea.Msg { sv.once.Do(func() { record(sv.key, sv.d) }); return nil }
		if m.motion {
			m.flash[msg.key] = m.now()
		}
		b.done++
		m.status = "✓ " + inboxDoingFor(job.it, job.a).did + "."
		m.relayout()
	}
	if b.total > 1 && b.done+b.failed >= b.total {
		delete(m.batches, job.batch)
		m.status = fmt.Sprintf("Applied %d: %d done, %d not done. Written changes are not undone here.", b.total, b.done, b.failed)
		if m.dryRun {
			m.status = fmt.Sprintf("Planned %d of %d (dry run), %d not planned: nothing was written to GitHub.", b.done, b.total, b.failed)
		}
		if i := m.item(); i >= 0 && m.state[m.items[i].key()] != "" {
			m.next() // the bulk is over: on to what still waits
		}
	}
	if m.quitting == quitCurrent {
		m.queue = slices.DeleteFunc(m.queue, func(j inboxJob) bool { return j.w == nil })
	}
	if m.quitting != quitNo && m.writing() == 0 && (m.quitting == quitCurrent || len(m.queue) == 0) {
		if save != nil {
			save()
		}
		return tea.Quit
	}
	if read := m.deferredRead(); read != nil {
		return tea.Batch(save, read)
	}
	return tea.Batch(save, m.pump())
}

// deferredRead is the read R asked for during the writes, once they have
// ended and no other read is under way; else nil.
func (m *inboxModel) deferredRead() tea.Cmd {
	if len(m.queue) > 0 || !m.readAfter || m.refreshing || m.gather == nil {
		return nil
	}
	m.readAfter, m.refreshing = false, true
	return tea.Batch(m.spinTick(), m.gatherCmd())
}

// writing counts the writes under way.
func (m *inboxModel) writing() int {
	n := 0
	for _, j := range m.queue {
		if j.w != nil {
			n++
		}
	}
	return n
}

// inboxTouches are the issue and pull request numbers a write on it may
// change: the item, its pull request, and its Story.
func inboxTouches(it InboxItem) []int {
	var out []int
	for _, n := range []int{it.Number, it.PR, it.Story} {
		if n > 0 && !slices.Contains(out, n) {
			out = append(out, n)
		}
	}
	return out
}

// blocker is the write under way that touches what j would, or nil: two
// writes on the same pull request or issue never run at once.
func (m *inboxModel) blocker(j inboxJob) *inboxJob {
	for k, r := range m.queue {
		if r.w != nil && r.it.key() != j.it.key() && slices.ContainsFunc(inboxTouches(r.it), func(n int) bool { return slices.Contains(inboxTouches(j.it), n) }) {
			return &m.queue[k]
		}
	}
	return nil
}

// pump starts the writes that wait, in order, while fewer than
// inboxParallel run; one that touches what a running write touches waits.
func (m *inboxModel) pump() tea.Cmd {
	var cmds []tea.Cmd
	for k := range m.queue {
		// No new write starts while the quit question is open: q promises
		// to leave the queued ones unstarted.
		if m.writing() >= inboxParallel || m.quitting == quitCurrent || m.ask != nil && m.ask.quit {
			break
		}
		j := &m.queue[k]
		if j.w != nil || m.blocker(*j) != nil {
			continue
		}
		j.w = &inboxWork{}
		j.w.begin(j.it.key(), m.now())
		delete(m.fails, j.it.key())
		delete(m.notes, j.it.key())
		job, act := *j, m.act
		cmds = append(cmds, func() tea.Msg {
			fresh, out, err := act(job.it, job.a, job.w)
			return inboxDoneMsg{job.it.key(), fresh, out, err}
		})
	}
	if len(cmds) == 0 {
		return nil
	}
	cmds = append(cmds, m.spinTick())
	if m.timer && !m.workTick {
		m.workTick = true
		cmds = append(cmds, tea.Tick(time.Second, func(time.Time) tea.Msg { return inboxWorkTickMsg{} }))
	}
	return tea.Batch(cmds...)
}

// start queues jobs and starts what can run now: the writes run in the
// background, so the person goes on acting meanwhile. After one Enter the
// cursor moves on to the next item that waits for them.
func (m *inboxModel) start(jobs []inboxJob) tea.Cmd {
	m.batch++
	if len(jobs) > 1 {
		m.batches[m.batch] = &inboxBatch{total: len(jobs)}
	}
	for k := range jobs {
		jobs[k].batch = m.batch
		delete(m.selected, jobs[k].it.key())
	}
	m.queue = append(m.queue, jobs...)
	cmd := m.pump()
	j := jobs[0]
	switch running, _ := m.queued(j.it.key()); {
	case len(jobs) > 1:
		m.status = fmt.Sprintf("Applying %d %s in the background; you can go on meanwhile.", len(jobs), plural(len(jobs), "item", "items"))
	case running:
		m.status = inboxDoingFor(j.it, j.a).doing + "… it runs in the background."
	case m.blocker(j) != nil:
		m.status = fmt.Sprintf("Queued #%d: it waits for the write on #%d, which touches the same pull request or issue.", j.it.Number, m.blocker(j).it.Number)
	default:
		m.status = fmt.Sprintf("Queued #%d: it starts when one of the %d writes under way ends.", j.it.Number, inboxParallel)
	}
	if i := m.item(); len(jobs) == 1 && i >= 0 && m.items[i].key() == j.it.key() {
		m.moveOn(j.it.kind().Housekeeping)
	}
	return cmd
}

// next moves to the next item still to do.
func (m *inboxModel) next() {
	m.sel, m.armed = 0, false
	for d := 1; d <= len(m.rows); d++ {
		r := (m.cur + d) % len(m.rows)
		if it := m.items[m.rows[r]]; m.state[it.key()] == "" && !it.waiting() {
			m.cur = r
			return
		}
	}
}

// moveOn moves the cursor to the next item that waits and has no write on
// it: a needs-you item after a needs-you write, else the next such row.
func (m *inboxModel) moveOn(house bool) {
	for d := 1; d < len(m.rows); d++ {
		r := (m.cur + d) % len(m.rows)
		it := m.items[m.rows[r]]
		running, waiting := m.queued(it.key())
		if m.state[it.key()] == "" && !running && !waiting && !it.waiting() && (house || !it.kind().Housekeeping) {
			m.cur, m.sel, m.armed = r, 0, false
			return
		}
	}
}

// queued reports whether key has a write under way, or one waiting its turn.
func (m *inboxModel) queued(key string) (running, waiting bool) {
	for _, j := range m.queue {
		if j.it.key() == key {
			return j.w != nil, j.w == nil
		}
	}
	return false, false
}

// job is the write on key, running or waiting, or nil.
func (m *inboxModel) job(key string) *inboxJob {
	for k := range m.queue {
		if m.queue[k].it.key() == key {
			return &m.queue[k]
		}
	}
	return nil
}

func wroteSuffix(w []string) string {
	if len(w) == 0 {
		return ""
	}
	return ": " + strings.Join(w, ", ")
}

func (m *inboxModel) key(msg tea.KeyPressMsg) tea.Cmd {
	k := msg.String()
	busy := m.loading && len(m.items) == 0 && (m.refreshing || !m.stalled) // a first read that timed out takes R and q
	writing := len(m.queue) > 0
	switch {
	case m.ask != nil && m.ask.quit:
		switch k {
		case "w", "enter":
			m.ask, m.quitting = nil, quitAll
			if len(m.queue) == 0 { // every write ended while the question was open
				return tea.Quit
			}
			m.status = fmt.Sprintf("Quitting once every write has finished: %d to go.", len(m.queue))
			return m.pump()
		case "q", "ctrl+c":
			m.ask, m.quitting = nil, quitCurrent
			m.queue = slices.DeleteFunc(m.queue, func(j inboxJob) bool { return j.w == nil })
			if m.writing() == 0 {
				return tea.Quit
			}
			m.status = fmt.Sprintf("Quitting once the %d %s under way %s; nothing else is started.", m.writing(), plural(m.writing(), "write", "writes"), plural(m.writing(), "finishes", "finish"))
		case "esc", "n":
			m.ask, m.status = nil, "Staying; the writes go on."
			return m.pump()
		}
		return nil
	case writing && (k == "ctrl+c" || k == "q" && m.ask == nil && !m.finding && !m.help):
		// Bubble Tea does not wait for a running command when it quits, so
		// the inbox never quits mid-write: it asks whether to wait for all.
		n, queued := m.writing(), len(m.queue)-m.writing()
		m.ask = &inboxAsk{quit: true, title: fmt.Sprintf("%d %s under way. Quit?", n, plural(n, "write is", "writes are")), lines: []string{
			fmt.Sprintf("w   wait for every write%s, then quit", map[bool]string{true: fmt.Sprintf(", the %d queued too", queued), false: ""}[queued > 0]),
			fmt.Sprintf("q   quit once the %d under way %s%s", n, plural(n, "finishes", "finish"), map[bool]string{true: fmt.Sprintf("; the %d queued are not started", queued), false: ""}[queued > 0]),
			"Esc stay in the inbox",
			"", "A write is never stopped half done: each one under way finishes and is reported."}}
		return nil
	case busy && (k == "ctrl+c" || k == "q"):
		m.quitting = quitCurrent // the first read finishes, then the inbox quits
		m.status = "Quitting after the read under way."
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
		switch {
		case k == "q" || k == "esc":
			return tea.Quit
		case k == "R" && m.stalled && !m.refreshing: // the first read timed out: read again
			m.refreshing, m.status = true, inboxReading
			return tea.Batch(m.spinTick(), m.gatherCmd())
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
		switch {
		case m.armed:
			m.armed, m.status = false, "Not applied; nothing was written."
		case len(m.selected) > 0:
			clear(m.selected)
			m.status = "Selection cleared; nothing was written."
		}
	case "enter":
		if len(m.selected) > 0 {
			m.askBulk()
			return nil
		}
		return m.enter()
	case "x", "space":
		m.toggle()
	case "X":
		m.selectHouse()
	case "s":
		if i := m.item(); i >= 0 && m.state[m.items[i].key()] == "" {
			m.state[m.items[i].key()] = "skipped"
			m.status = fmt.Sprintf("Skipped #%d for now; j and k come back to it.", m.items[i].Number)
		}
		m.next()
	case "C", "A":
		m.askTidy()
	case "/", "ctrl+k":
		m.finding, m.query, m.hit = true, "", 0
		m.find()
	case "d":
		m.details = !m.details
		m.status = map[bool]string{true: "Details shown: what GitHub said, under each card. d hides them.", false: "Details hidden."}[m.details]
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
			m.readAfter, m.status = true, "A refresh waits for the writes under way; it reads GitHub again after them."
		} else if !m.refreshing {
			m.refreshing, m.status = true, inboxReading
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
		if st := m.state[m.items[i].key()]; !m.items[i].kind().Housekeeping && !m.items[i].waiting() && st != "done" && st != "gone" {
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
	it := m.items[m.item()]
	running, waiting := m.queued(it.key())
	d, acted := m.done[it.key()]
	switch {
	case running:
		m.refuse(fmt.Sprintf("#%d is being written now; it is not started twice.", it.Number))
	case waiting:
		m.refuse(fmt.Sprintf("#%d is queued already; it runs after the write under way.", it.Number))
	case m.state[it.key()] == "gone":
		m.refuse(fmt.Sprintf("#%d is no longer on GitHub's list of what waits; nothing to do.", it.Number))
	case m.state[it.key()] == "done":
		m.refuse(fmt.Sprintf("#%d is %s; it is written to GitHub and not offered again.", it.Number, d.line(m.now())))
	case acted && c.Command != "open" && !m.armed: // relisted: GitHub still lists it after the write
		m.armed = true
		m.status = fmt.Sprintf("You already did this: %s. GitHub still lists #%d. Enter again does it again.", d.line(m.now()), it.Number)
	case c.Command == "open":
		m.open(it.URL)
		m.status = "Opened " + it.URL
	case c.Ask != "":
		m.ask = &inboxAsk{title: fmt.Sprintf("%s #%d: %s", it.kind().Word, it.Number, it.button(c)), lines: []string{inboxEffect(it, c)}, jobs: []inboxJob{{it: it, a: c}}, prompt: c.Ask}
		m.status = "Nothing is written until you type what you checked and press Enter."
	case (!c.Recommended || it.OneWay) && !m.armed:
		m.armed = true
		m.status = "Nothing is written until you press Enter again."
	default:
		m.armed = false
		return m.start([]inboxJob{{it: it, a: c}})
	}
	return nil
}

// askTidy asks to do every housekeeping recommendation: one confirmation.
func (m *inboxModel) askTidy() {
	m.askJobs(m.house(), "Clear all %d housekeeping %s?")
}

// askBulk asks to do the recommendation of every selected housekeeping item:
// one confirmation that lists each.
func (m *inboxModel) askBulk() {
	var picked []InboxItem
	for _, i := range m.rows {
		if it := m.items[i]; m.selected[it.key()] && it.kind().Housekeeping {
			picked = append(picked, it)
		}
	}
	m.askJobs(picked, "Do the recommendation for %d selected housekeeping %s?")
}

func (m *inboxModel) askJobs(items []InboxItem, title string) {
	var jobs []inboxJob
	var lines []string
	for _, it := range items {
		if running, waiting := m.queued(it.key()); it.tidy() && !running && !waiting {
			jobs = append(jobs, inboxJob{it: it, a: *it.recommended()})
			lines = append(lines, fmt.Sprintf("%s #%d: %s", it.kind().Icon, it.Number, it.recommended().Label))
		}
	}
	if len(jobs) == 0 {
		m.refuse("No housekeeping is left to clear.")
		return
	}
	m.ask = &inboxAsk{title: fmt.Sprintf(title, len(jobs), plural(len(jobs), "item", "items")),
		lines: append(lines, "", fmt.Sprintf("Each is written to GitHub as @%s and is not undone here. Nothing in the code changes.", m.login)), jobs: jobs}
	m.status = "Nothing is written until you press Enter."
}

// selectable says why item i cannot be selected for a bulk action, or "".
// Only housekeeping can: a needs-you item is decided on its own.
func (m *inboxModel) selectable(i int) string {
	it := m.items[i]
	running, waiting := m.queued(it.key())
	switch st := m.state[it.key()]; {
	case it.waiting():
		return fmt.Sprintf("#%d is not ready yet; only housekeeping can be selected.", it.Number)
	case !it.kind().Housekeeping:
		return fmt.Sprintf("#%d needs you; only housekeeping can be selected.", it.Number)
	case running || waiting:
		return fmt.Sprintf("#%d is being written already.", it.Number)
	case st == "done" || st == "gone":
		return fmt.Sprintf("#%d is %s; there is nothing to select.", it.Number, st)
	}
	return ""
}

// toggle selects or clears the housekeeping item under the cursor: x.
func (m *inboxModel) toggle() {
	i := m.item()
	if i < 0 {
		return
	}
	if why := m.selectable(i); why != "" {
		m.refuse(why)
		return
	}
	k := m.items[i].key()
	if m.selected[k] {
		delete(m.selected, k)
	} else {
		m.selected[k] = true
	}
	m.sayPicked()
}

// selectHouse selects every housekeeping item that can be, or clears them
// when all are already selected: X. It never selects a needs-you item.
func (m *inboxModel) selectHouse() {
	var can []string
	for _, i := range m.rows {
		if m.selectable(i) == "" {
			can = append(can, m.items[i].key())
		}
	}
	all := len(can) > 0
	for _, k := range can {
		all = all && m.selected[k]
	}
	clear(m.selected)
	if !all {
		for _, k := range can {
			m.selected[k] = true
		}
	}
	m.sayPicked()
}

func (m *inboxModel) sayPicked() {
	if n := len(m.selected); n > 0 {
		m.status = fmt.Sprintf("%d housekeeping %s selected; Enter asks once, then does each.", n, plural(n, "item", "items"))
		return
	}
	m.status = "Nothing selected."
}

// find ranks every item and the card's choices
// against the query, fzf-style (sahilm/fuzzy): the best match first and
// selected. It runs on each keystroke, never in View.
func (m *inboxModel) find() {
	var texts []string
	var base []inboxHit
	add := func(h inboxHit) { base, texts = append(base, h), append(texts, h.text) }
	for _, i := range m.rows {
		add(m.itemHit(i))
	}
	if i := m.item(); i >= 0 {
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

// render draws the frame: the head line, the list and the card (side by side
// on a wide terminal, the list above the card on a narrow one), the status
// line, and the keys. Every line is exactly the terminal's width.
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
	if ansi.StringWidth(head)+3+len(m.repo) <= w-2*inboxPad {
		head += m.style(ui.Muted, " · "+m.repo)
	}
	body := h - 3
	inner := w - 2*inboxPad
	var lines []string
	shift := strings.Repeat(" ", m.cardShift())
	padded := func(ls []string) []string {
		var out []string
		for _, l := range ls {
			out = append(out, cell(l, inner))
		}
		return out
	}
	switch {
	case m.signin.screen():
		lines = padded(m.signinView(inner, body))
	case m.loading && len(m.items) == 0:
		lines = padded(m.skeleton(inner, body))
	case len(m.rows) == 0:
		lines = padded([]string{"", m.style(ui.Title, "Nothing waits for you."), "", "Every human-only step is done. q quits."})
	case w < inboxWide && m.ask != nil:
		lines = padded(m.askView(inner, body))
	case w < inboxWide && m.help:
		lines = padded(m.helpView(inner, body))
	case m.finding:
		lines = padded(m.findView(inner, body))
	case w >= inboxWide:
		lw, rw := m.panes(w)
		list := append([]string{m.columnsHead(lw)}, m.listView(lw, body-1, inboxPad, 2)...)
		var card []string
		switch {
		case m.ask != nil: // the confirmation and the help take the card's place; the list stays
			card = m.askView(rw, body)[1:]
		case m.help:
			card = m.helpView(rw, body+1)[1:]
		default:
			card = m.cardView(rw, body, lw+3*inboxPad+1, 1)
		}
		bar := m.style(ui.Muted, "│")
		for r := range body {
			l, c := "", ""
			if r < len(list) {
				l = list[r]
			}
			if r < len(card) {
				c = shift + card[r]
			}
			lines = append(lines, cell(l, lw)+strings.Repeat(" ", inboxPad)+bar+strings.Repeat(" ", inboxPad)+cell(c, rw))
		}
	default:
		listH := min(len(m.listLines()), max(body/2-1, 4))
		if i := m.item(); i >= 0 && (m.actionView(m.items[i], inner, body) != nil || m.state[m.items[i].key()] == "not done" || m.details) {
			listH = min(listH, max(body/3-1, 4)) // the action's progress needs the room; the list stays in view
		}
		lines = append(lines, m.columnsHead(inner))
		lines = append(lines, m.listView(inner, listH, inboxPad, 2)...)
		lines = append(lines, m.style(ui.Muted, strings.Repeat("─", inner)))
		for _, l := range m.cardView(inner, body-len(lines), inboxPad, len(lines)+1) {
			lines = append(lines, shift+l)
		}
		lines = padded(lines)
	}
	if len(lines) > body {
		lines = lines[:body]
	}
	for len(lines) < body {
		lines = append(lines, strings.Repeat(" ", inner))
	}
	edge := strings.Repeat(" ", inboxPad)
	var b strings.Builder
	b.Grow(w * (h + 1) * 2)
	b.WriteString(edge + cell(head, inner) + edge)
	for _, l := range lines {
		b.WriteString("\n" + edge + cell(l, inner) + edge)
	}
	b.WriteString("\n" + edge + cell(m.statusLine(inner), inner) + edge)
	b.WriteString("\n" + edge + cell(m.keysLine(inner), inner) + edge)
	return b.String()
}

// panes are the content widths of the list and the card on a wide frame:
// with the padding and the divider they fill it exactly.
func (m *inboxModel) panes(w int) (list, card int) {
	inner := w - 2*inboxPad
	list = inner * inboxShare / 100
	return list, inner - list - 2*inboxPad - 1
}

// cell is s cut to width display cells (with …) and padded to exactly width:
// one cell never runs into the next column.
func cell(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if ansi.StringWidth(s) > width {
		s = ansi.Truncate(s, width, "…")
	}
	return s + strings.Repeat(" ", max(width-ansi.StringWidth(s), 0))
}

// wordCell is cell for words: a cut ends at the last word that fits when that
// drops at most inboxWordCut cells, else mid-word as cell does. The width is
// the same either way.
func wordCell(s string, width int) string {
	if width > 1 && ansi.StringWidth(s) > width {
		t := ansi.Truncate(s, width-1, "") // room for the …
		if rest := s[len(t):]; strings.HasPrefix(rest, " ") {
			return cell(strings.TrimRight(t, " ,;:·")+"…", width)
		}
		if i := strings.LastIndexByte(t, ' '); i > 0 && ansi.StringWidth(t[i:]) <= inboxWordCut {
			return cell(strings.TrimRight(t[:i], " ,;:·")+"…", width)
		}
	}
	return cell(s, width)
}

// rcell is cell, aligned right.
func rcell(s string, width int) string {
	if ansi.StringWidth(s) > width {
		return cell(s, width)
	}
	return strings.Repeat(" ", width-ansi.StringWidth(s)) + s
}

// inboxCols are the widths of the list's columns for a content width w; a
// column with no room is 0 and not drawn.
type inboxCols struct{ num, title, why, age int }

func (m *inboxModel) cols(w int) inboxCols {
	c := inboxCols{num: 2}
	for _, it := range m.items {
		c.num = max(c.num, len(strconv.Itoa(it.Number))+1)
	}
	if w >= inboxAgeMin {
		c.age = inboxAgeW
	}
	if w >= inboxWhyMin {
		c.why = inboxWhyW
	}
	c.title = w - inboxMarkW - inboxGutter - inboxIconW - inboxGutter - c.num - inboxGutter
	for _, extra := range []int{c.why, c.age} {
		if extra > 0 {
			c.title -= inboxGutter + extra
		}
	}
	return c
}

// row joins the cells of one list row, each fitted to its column.
func (c inboxCols) row(mark, icon, num, title, why, age string) string {
	gap := strings.Repeat(" ", inboxGutter)
	r := cell(mark, inboxMarkW) + gap + cell(icon, inboxIconW) + gap + rcell(num, c.num) + gap + wordCell(title, max(c.title, 1))
	if c.why > 0 {
		r += gap + wordCell(why, c.why)
	}
	if c.age > 0 {
		r += gap + rcell(age, c.age)
	}
	return r
}

// columnsHead names the list's columns, in the list's own cells, each name
// at the first cell of its column (the number and the age are right-aligned
// in their rows, so their names are padded to the column's width).
func (m *inboxModel) columnsHead(w int) string {
	c := m.cols(w)
	return m.style(ui.Muted, c.row("", "", cell("#", c.num), "TITLE", "WHY IT WAITS", cell("AGE", c.age)))
}

// inboxLine is one line of the list: a group's header, the blank line
// between groups, or row r of m.rows.
type inboxLine struct {
	group, row int // row is -1 for a header or a blank
}

// listLines are the list's lines: each group that has rows, its header first,
// with one blank line between groups.
func (m *inboxModel) listLines() []inboxLine {
	var out []inboxLine
	g := -1
	for r, i := range m.rows {
		if gi := m.group(i); gi != g {
			if g >= 0 {
				out = append(out, inboxLine{gi, -2})
			}
			g = gi
			out = append(out, inboxLine{g, -1})
		}
		out = append(out, inboxLine{g, r})
	}
	return out
}

// skeleton is the first read's screen: the shape of the list in muted blocks,
// and how far the read has got.
func (m *inboxModel) skeleton(w, h int) []string {
	block := func(n int) string { return m.style(ui.Muted, strings.Repeat("░", max(n, 1))) }
	c := m.cols(min(w, max(w*inboxShare/100, 40)))
	if m.stalled && !m.refreshing { // the first read timed out, and no read runs now
		return []string{"", inboxSlowFirst}
	}
	out := []string{"", m.spinner() + "Reading the issues and pull requests of " + m.repo + "…"}
	switch l := m.load; {
	case l.total > 0:
		out = append(out, fmt.Sprintf("%s %d of %d items checked", m.bar(l.done, l.total, 16), l.done, l.total))
	case l.what != "":
		out = append(out, l.what+"…")
	}
	out = append(out, "")
	for _, n := range []int{22, 17, 25, 13} {
		out = append(out, c.row("", block(2), block(3), block(min(n, c.title)), block(min(12, c.why)), block(min(2, c.age))))
	}
	return out
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

// listView is the list: the groups with their counts, and one row per item
// in fixed columns (the mark, the icon, the number, the title, why it waits,
// and its age), the cursor row reversed and marked ›. Each line is exactly w
// cells wide. x and y place it on the screen, for the mouse.
func (m *inboxModel) listView(w, h, x, y int) []string {
	lines := m.listLines()
	c := m.cols(w)
	hi := m.cur // the highlight: on the cursor, or gliding to it
	if m.motion {
		hi = min(max(int(math.Round(m.glide)), 0), max(len(m.rows)-1, 0))
	}
	at := slices.IndexFunc(lines, func(l inboxLine) bool { return l.row == hi })
	off := 0
	if len(lines) > h {
		off = min(max(at-h/2, 0), len(lines)-h)
	}
	counts := map[int]int{}
	for _, i := range m.rows {
		counts[m.group(i)]++
	}
	shown := lines[off:min(off+h, len(lines))]
	for len(shown) > 0 && shown[len(shown)-1].row < 0 {
		shown = shown[:len(shown)-1] // a group's header with none of its rows in view
	}
	var out []string
	for _, l := range shown {
		switch l.row {
		case -2:
			out = append(out, strings.Repeat(" ", w))
			continue
		case -1:
			g := inboxGroups[l.group]
			name := fmt.Sprintf("%s (%d)", g.name, counts[l.group])
			out = append(out, cell(m.style(ui.Accent, name)+"  "+m.style(ui.Muted, g.about), w))
			continue
		}
		it := m.items[m.rows[l.row]]
		mark, why, role := m.rowState(m.rows[l.row])
		cursor := " "
		switch {
		case l.row == hi:
			cursor = "›"
		case role == ui.Accent: // a selected row: only its [x] is coloured, so the list stays calm
			mark, role = m.style(ui.Accent, mark), inboxText
		}
		row := c.row(cursor+" "+mark, it.kind().Icon, "#"+strconv.Itoa(it.Number), it.Title, why, shortAgo(m.now(), it.Updated))
		switch {
		case l.row == hi:
			row = reverse(row)
		case role != inboxText:
			row = m.style(role, row)
		}
		m.zones = append(m.zones, inboxZone{x0: x, x1: x + w, y0: y + len(out), y1: y + len(out), row: l.row, choice: -1, hit: -1})
		out = append(out, row)
	}
	for len(out) < min(h, len(lines)) {
		out = append(out, strings.Repeat(" ", w))
	}
	return out
}

// rowState is item i's mark (three cells), its why-it-waits words, and the
// role its row is drawn in (inboxText for the text colour).
func (m *inboxModel) rowState(i int) (mark, why string, role ui.Role) {
	it := m.items[i]
	k := it.key()
	running, waiting := m.queued(k)
	d := m.done[k]
	switch st := m.state[k]; {
	case running:
		frame := "*"
		if m.motion {
			frame = strings.TrimSpace(ansi.Strip(m.spin.View()))
		}
		why := "writing now"
		if j := m.job(k); j != nil {
			_, stage, _, started := j.w.snapshot()
			why = shortElapsed(m.now().Sub(started)) + " · " + inboxStepOf(stage).line
		}
		return " " + frame + " ", why, ui.Warn
	case waiting:
		return "...", "queued", ui.Muted
	case st == "done":
		who := "you"
		if d.By != m.login {
			who = "@" + d.By
		}
		return " ✓ ", "done by " + who + " " + d.when(m.now()), ui.OK
	case st == "not done" && m.fails[k].why.Retry:
		return " ✗ ", "failed: Enter retries", ui.Fail
	case st == "not done":
		return " ✗ ", "not done: see the card", ui.Fail
	case st == "gone":
		return " - ", "gone from GitHub", ui.Muted
	case st == "relisted":
		return " ! ", "listed again after you", ui.Warn
	case st == "skipped":
		return "   ", "skipped for now", ui.Muted
	case it.waiting():
		return "   ", it.short(), ui.Muted
	case !it.kind().Housekeeping:
		return "   ", it.short(), inboxText
	case m.selected[k]:
		return "[x]", it.short(), ui.Accent
	}
	return "[ ]", it.short(), inboxText
}

// inboxText is no role: the row keeps the text colour.
const inboxText ui.Role = -1

// shortElapsed is a write's running time in a few cells: 6s, 2m05s.
func shortElapsed(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
}

// shortAgo is how long since t in at most three cells: 12m, 3h, 2d.
func shortAgo(now, t time.Time) string {
	switch d := now.Sub(t); {
	case t.IsZero():
		return ""
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", min(int(d.Hours()/24), 99))
	}
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
	}
	if i >= 0 {
		if v := m.actionView(it, w, h); v != nil {
			return v
		}
	}
	wrapTo := m.wrap
	top := []string{m.style(ui.Muted, tag)}
	for _, l := range wrapTo(title, w) {
		top = append(top, inboxBold.Render(l))
	}
	top = append(top, wrapTo(kind.Line, w)...) // what it means, in the text colour: it is read, not skimmed
	var whyLines, writes, details []string     // the list's why-it-waits column, in full; a write that stopped; d's facts
	switch {
	case it.Next != "": // nothing for the person yet: why, and who acts, in the text colour
		whyLines = append([]string{""}, m.wrap(fmt.Sprintf("#%d %s", it.Number, it.Why), w)...)
		whyLines = append(whyLines, m.wrapStyled(ui.Accent, "Next: "+it.Next, w)...)
	case it.Why != "":
		whyLines = m.wrapStyled(ui.Muted, "Why it waits: "+it.Why, w)
	}
	if m.details && i >= 0 {
		details = m.detailLines(it, w)
	}
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
	choiceAt, choiceEnd := map[int]int{}, map[int]int{} // a choice's first and last line
	if i >= 0 {
		writes = m.failLines(it, w)
	}
	choiceLines = append(choiceLines, "")
	// Each choice spans the card's width exactly: the cursor, its number,
	// its label (wrapped under its own text when it is long), and the tag
	// in a fixed column at the right edge, on the first line.
	numW, tagW := 0, 0
	for _, c := range m.choices() {
		numW = max(numW, ansi.StringWidth(c.ref())+inboxGutter)
		if c.Recommended {
			tagW = len(inboxTag)
		}
	}
	room := w - inboxCursorW - numW
	if tagW > 0 {
		room -= inboxGutter + tagW
	}
	for ci, c := range m.choices() {
		label := c.Label
		if i >= 0 {
			label = it.button(c)
		}
		choiceAt[ci] = len(choiceLines)
		for li, l := range wrapTo(label, max(room, 1)) {
			cursor, num, tag := strings.Repeat(" ", inboxCursorW), "", ""
			if li == 0 {
				num = c.ref()
				if c.Recommended {
					tag = inboxTag
				}
				if ci == m.sel {
					cursor = cell("›", inboxCursorW)
				}
			}
			row := cursor + cell(num, numW) + cell(l, max(room, 1))
			if tagW > 0 {
				row += strings.Repeat(" ", inboxGutter) + rcell(tag, tagW)
			}
			if ci == m.sel {
				row = reverse(row)
			}
			choiceLines = append(choiceLines, row)
		}
		choiceEnd[ci] = len(choiceLines) - 1
	}
	if m.armed && i >= 0 {
		c := m.choices()[min(m.sel, len(m.choices())-1)]
		choiceLines = append(choiceLines, "")
		for _, l := range wrapTo(inboxEffect(it, c), w) {
			choiceLines = append(choiceLines, m.style(ui.Warn, l))
		}
		choiceLines = append(choiceLines, inboxBold.Render("Enter again applies it · Esc goes back"))
	}
	// Fit: the counsel gives way, then the full why; the choices and what
	// the selected one does always show.
	extra := func() int {
		return len(top) + len(whyLines) + len(details) + len(writes) + len(counselLines) + len(choiceLines) - h
	}
	if len(details) > 0 && extra() > 0 {
		counselLines = nil // the person asked for the details
	}
	if len(writes) > 0 && extra() > 0 {
		counselLines = nil // a write that stopped matters more than the counsel
	}
	if extra() > 0 && len(counselLines) > 3 {
		keep := max(len(counselLines)-extra()-1, 3)
		if keep < len(counselLines) {
			counselLines = append(counselLines[:keep], m.style(ui.Muted, "▏ … ? shows all of it"))
		}
	}
	if extra() > 0 && len(details) > 3 { // the details give way before the card's own words
		details = append(details[:max(len(details)-extra()-1, 2)], m.style(ui.Muted, cell("… more on a taller terminal", w)))
	}
	if extra() > 0 {
		whyLines = whyLines[:max(len(whyLines)-extra(), 0)]
	}
	if extra() > 0 {
		counselLines = nil
	}
	top = append(append(append(top, whyLines...), details...), writes...)
	out := append(append(top, counselLines...), choiceLines...)
	base := y + len(top) + len(counselLines)
	for ci, at := range choiceAt {
		if at+len(top)+len(counselLines) < h {
			m.zones = append(m.zones, inboxZone{x0: x, x1: x + w, y0: base + at, y1: base + choiceEnd[ci], row: -1, choice: ci, hit: -1})
		}
	}
	return out
}

// actionView is the card's body while a write on its item runs or waits,
// and once it is done: a headline in plain words, what is happening now,
// the steps as a checklist, how long it usually takes, and what the person
// can do meanwhile; then what changed on GitHub and what happens next. It is
// nil for an item with no write this session (and for one that stopped,
// whose card keeps its choices under failLines).
func (m *inboxModel) actionView(it InboxItem, w, h int) []string {
	k := it.key()
	running, waiting := m.queued(k)
	d, done := m.done[k]
	if !running && !waiting && (!done || m.state[k] != "done") {
		return nil
	}
	var a InboxAction
	job := m.job(k)
	if job != nil {
		a = job.a
	}
	if done && a.Label == "" {
		for _, c := range it.choices() {
			if it.button(c) == d.Choice {
				a = c
			}
		}
	}
	do := inboxDoingFor(it, a)
	frame := "*"
	if m.motion {
		frame = strings.TrimSpace(ansi.Strip(m.spin.View()))
	}
	head := func(role ui.Role, s string) []string {
		out := []string{}
		for _, l := range m.wrap(s, w) {
			out = append(out, m.style(role, inboxBold.Render(l)))
		}
		return out
	}
	sub := []string{m.style(ui.Muted, cell(it.kind().Icon+" "+it.Title, w))}
	if job != nil {
		if b := m.batches[job.batch]; b != nil {
			sub = append(sub, m.style(ui.Accent, m.bar(b.done+b.failed, b.total, min(12, max(w/6, 4)))+" ")+
				cell(fmt.Sprintf("%d of %d housekeeping done", b.done+b.failed, b.total), w-min(12, max(w/6, 4))-1))
		}
	}
	switch {
	case waiting:
		out := append(head(ui.Accent, do.doing+": queued"), sub...)
		why := fmt.Sprintf("It starts when one of the %d writes under way ends.", inboxParallel)
		if bl := m.blocker(*job); bl != nil {
			why = fmt.Sprintf("It waits for the write on #%d, which touches the same pull request or issue; two writes on one never run at once.", bl.it.Number)
		}
		out = append(append(out, ""), m.wrap(why, w)...)
		return append(append(out, ""), m.wrapStyled(ui.Muted, "You can act on other items meanwhile.", w)...)
	case running:
		_, stage, calls, started := job.w.snapshot()
		seen, cur := job.w.steps()
		// Every stage seen is done but the current one, also one seen after
		// an earlier sighting of a stage that now repeats: the done steps come
		// first, so the fold below counts them all.
		steps := []string{}
		for j, st := range seen {
			if j != cur {
				steps = append(steps, m.style(ui.OK, "✓ ")+cell(inboxStepOf(st).line, w-2))
			}
		}
		finished := len(steps)
		if cur < len(seen) {
			steps = append(steps, m.style(ui.Warn, cell(frame+" "+inboxStepOf(seen[cur]).line, w)))
		}
		for _, st := range do.plan {
			if !slices.Contains(seen, st) {
				steps = append(steps, m.style(ui.Muted, cell("○ "+inboxStepOf(st).line, w)))
			}
		}
		out := append(append(head(ui.Warn, do.doing), sub...), "")
		out = append(out, m.wrapStyled(ui.Warn, frame+" Now: "+inboxStepOf(stage).now+".", w)...)
		out = append(out, "")
		tail := []string{""}
		tail = append(tail, m.wrapStyled(ui.Muted, fmt.Sprintf("%s; %d GitHub %s so far, %s.", upperFirst(do.usually), calls, plural(calls, "call", "calls"), elapsed(m.now().Sub(started))), w)...)
		tail = append(append(tail, ""), m.wrapStyled(ui.Muted, fmt.Sprintf("It runs in the background: you can act on other items meanwhile (up to %d writes run at once).", inboxParallel), w)...)
		// Fit: the finished steps fold into one line, then the steps still
		// to come; what happens now and the time always show.
		if room := h - len(out) - len(tail); len(steps) > room {
			done := min(finished, len(steps)-room+1)
			if done > 1 {
				steps = append([]string{m.style(ui.OK, "✓ ") + cell(fmt.Sprintf("%d steps done", done), w-2)}, steps[done:]...)
			}
			if len(steps) > room && room > 1 {
				more := len(steps) - room + 1
				steps = append(steps[:room-1], m.style(ui.Muted, cell(fmt.Sprintf("○ and %d more", more), w)))
			}
		}
		return append(append(out, steps...), tail...)
	}
	did := do.did
	if len(d.Warning) > 0 && it.PR > 0 { // merged, but not every step of ship finished
		did = fmt.Sprintf("Merged #%d into %s", it.Number, or(it.f.devBr, "dev"))
	}
	out := append(head(ui.OK, "✓ "+did), sub[0])
	out = append(out, m.style(ui.Muted, cell(fmt.Sprintf("by @%s at %s", d.By, d.when(m.now())), w)))
	for _, l := range d.Warning { // a step after the main change did not finish
		out = append(out, m.wrapStyled(ui.Warn, l, w)...)
	}
	if m.details && len(d.Detail) > 0 {
		out = append(out, m.style(ui.Muted, inboxBold.Render("Details")+" (d hides them)"))
		for _, l := range d.Detail {
			out = append(out, m.wrapStyled(ui.Muted, "· "+l, w)...)
		}
	}
	out = append(out, "", inboxBold.Render("What changed on GitHub"))
	if len(d.Wrote) == 0 {
		out = append(out, cell("  "+d.Choice, w))
	}
	for _, wr := range d.Wrote {
		out = append(out, "  "+m.style(ui.OK, "✓ ")+cell(upperFirst(wr), w-4))
	}
	if it.URL != "" {
		out = append(out, "  "+m.style(ui.Muted, cell(it.URL, w-2)))
	}
	if do.next != "" {
		out = append(append(out, "", inboxBold.Render("What happens next")), m.wrap(do.next, w)...)
	}
	return append(append(out, ""), m.wrapStyled(ui.Muted, "It moves to Done this session; Tab goes to the next item that needs you.", w)...)
}

// detailLines are d's view: GitHub's own facts behind the card, below its
// plain lines, for a person who wants the commit, the check, or the state.
func (m *inboxModel) detailLines(it InboxItem, w int) []string {
	if len(it.Details) == 0 {
		return []string{"", m.style(ui.Muted, cell("No details for this card; d hides this.", w))}
	}
	out := []string{"", m.style(ui.Muted, inboxBold.Render("Details")+" (d hides them)")}
	for _, d := range it.Details {
		out = append(out, m.wrapStyled(ui.Muted, "· "+d, w)...)
	}
	return out
}

// failLines is what the card says above its choices about a write that
// stopped: the step, why, and what GitHub had already answered as written;
// and for an item listed again after a write, that write.
func (m *inboxModel) failLines(it InboxItem, w int) []string {
	k := it.key()
	if f, ok := m.fails[k]; ok && m.state[k] == "not done" {
		out := []string{""}
		for _, l := range m.wrap("✗ "+f.why.What, w) {
			out = append(out, m.style(ui.Fail, inboxBold.Render(l)))
		}
		changed := ui.Muted
		if len(f.wrote) > 0 {
			changed = ui.Warn
		}
		out = append(out, m.wrapStyled(changed, f.why.Changed, w)...)
		out = append(out, m.wrapStyled(ui.Accent, "Next: "+f.why.Next, w)...)
		if m.details {
			out = append(out, m.style(ui.Muted, inboxBold.Render("Details")+" (d hides them)"))
			for _, d := range f.why.Detail {
				out = append(out, m.wrapStyled(ui.Muted, "· "+d, w)...)
			}
		}
		return out
	}
	if note := m.notes[k]; note != "" {
		return append([]string{""}, m.wrapStyled(ui.Warn, note, w)...)
	}
	if d, ok := m.done[k]; ok && m.state[k] == "relisted" {
		return append([]string{""}, m.wrapStyled(ui.Warn, upperFirst(d.line(m.now()))+wroteSuffix(d.Wrote)+". GitHub, read since, still lists it, so it may still need you: Enter asks before doing it again.", w)...)
	}
	return nil
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
	if a.quit {
		return out
	}
	if a.prompt != "" {
		return append(out, "", inboxBold.Render(a.prompt+" ")+a.text+"█", "", inboxBold.Render("Enter applies it · Esc cancels"))
	}
	return append(out, "", inboxBold.Render("Enter applies all · Esc cancels"))
}

// helpView is ? : the plain explainer of what is on the card, its glossary
// terms, and every key, in h lines; the explainer gives way to the keys.
func (m *inboxModel) helpView(w, h int) []string {
	k := inboxTidy
	if i := m.item(); i >= 0 {
		k = m.items[i].kind()
	}
	const keyW = 8 // the key column, and the hanging indent of what it does
	keys := []string{"", inboxBold.Render("Keys")}
	for _, kv := range [][2]string{{"Enter", "do the selected choice; a one-way door asks again"}, {"1-9", "select a choice"}, {"s", "skip for now"},
		{"Tab", "next needs-you item (Shift-Tab back)"}, {"j/k", "move (↓/↑ and the mouse wheel too)"}, {"/", "find any item or choice (Ctrl+K)"},
		{"x", "select housekeeping; never a needs-you item"}, {"X", "select all housekeeping"}, {"C", "clear all housekeeping, after one confirmation"},
		{"Esc", "clear the selection, or go back"}, {"o", "open on GitHub"}, {"d", "show or hide what GitHub said, under each card"}, {"R", "refresh now: read every card's pull request again"}, {"q", "quit"}} {
		for li, l := range m.wrap(kv[1], w-keyW) {
			key := ""
			if li == 0 {
				key = kv[0]
			}
			keys = append(keys, cell(inboxBold.Render(key), keyW)+l)
		}
	}
	if m.refreshLog != "" {
		keys = append(keys, m.style(ui.Muted, cell("Last refresh: "+m.refreshLog+".", w)))
	}
	explain := m.wrap(k.Explain, w)
	if room := h - len(keys) - 3; len(explain) > room {
		explain = append(explain[:max(room-1, 0)], m.style(ui.Muted, "…"))
	}
	out := append([]string{"", inboxBold.Render(strings.TrimSpace(k.Icon + " " + k.Word + ": what this means"))}, explain...)
	out = append(out, m.style(ui.Muted, cell("Glossary: "+strings.Join(k.Terms, ", ")+" in GLOSSARY.md", w)))
	return append(out, keys...)
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
	if n := m.writing(); n > 0 && (m.refusal == "" || m.status != m.refusal) && !strings.HasPrefix(m.status, "Quitting") {
		left = m.spinner() + fmt.Sprintf("%d %s in the background", n, plural(n, "write", "writes"))
		if q := len(m.queue) - n; q > 0 {
			left += fmt.Sprintf(" · %d queued", q)
		}
	}
	right := ""
	switch {
	case len(m.batches) > 0:
		total, done := 0, 0
		for _, b := range m.batches {
			total, done = total+b.total, done+b.done+b.failed
		}
		right = fmt.Sprintf("%s %d/%d", m.bar(done, total, min(12, max(w/8, 4))), done, total)
	case m.refreshing && !m.loading:
		right = m.spinner() + "refreshing"
	case !m.fresh.IsZero():
		right = "read " + ago(m.now(), m.fresh)
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
	case m.signin.screen():
		ks := m.signinKeys()
		for i := 0; i+1 < len(ks); i += 2 {
			keys = append(keys, kb{ks[i], ks[i+1], 0})
		}
	case len(m.rows) == 0:
		keys = []kb{{"q", "quit", 0}}
	case m.ask != nil && m.ask.quit:
		keys = []kb{{"w", "wait for all", 0}, {"q", "quit after the running ones", 0}, {"Esc", "stay", 0}}
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
		keys = []kb{{"Enter", "do it too", 0}, {"Tab", "next", 1}, {"x", "select", 2}, {"/", "find", 2}, {"R", "refresh", 1}, {"?", "help", 1}, {"q", "quit (asks)", 0}}
	case len(m.selected) > 0:
		n := len(m.selected)
		keys = []kb{{strconv.Itoa(n), "selected", 0}, {"Enter", fmt.Sprintf("do all %d", n), 0}, {"x", "toggle", 1}, {"X", "all/none", 2}, {"Esc", "clear", 0}}
	default:
		keys = []kb{{"Enter", "do it", 0}, {"Tab", "next", 1}, {"x", "select", 2}, {"X", "all", 3}, {"C", "clear", 3}, {"/", "find", 2}, {"d", "details", 3}, {"R", "refresh", 1}, {"?", "help", 1}, {"q", "quit", 0}}
		if i := m.item(); i >= 0 && m.items[i].waiting() { // nothing to do on it: Enter only opens it
			keys = []kb{{"Tab", "next", 0}, {"o", "open", 1}, {"x", "select", 2}, {"X", "all", 3}, {"C", "clear", 3}, {"/", "find", 2}, {"d", "details", 1}, {"R", "refresh", 1}, {"?", "help", 1}, {"q", "quit", 0}}
		}
		if m.houseLeft() == 0 {
			keys = slices.DeleteFunc(keys, func(k kb) bool { return k.key == "x" || k.key == "X" || k.key == "C" })
		}
		if m.details {
			for i := range keys {
				if keys[i].key == "d" {
					keys[i] = kb{"d", "hide details", 1}
				}
			}
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
