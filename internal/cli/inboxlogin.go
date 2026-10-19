package cli

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/rahulnakmol/agentic-ayudha/internal/ui"
)

// The inbox's first screen, when the session needs a person's approval
// (presence.go): GitHub's device code, large, in the inbox's own frame. The
// approval is presenceCheck's, unchanged; this only shows the code. The code
// lives in memory for the person at this terminal: it is never logged, never
// in the status line, and never in the ledger or the inbox cache.
//
// GitHub's device flow returns no verification_uri_complete (a link with the
// code in it) for aayu's app, so the one-click path is two keys: Enter opens
// https://github.com/login/device in the browser, and c copies the code
// through OSC 52, which reaches the clipboard over SSH and, with
// set-clipboard on, through tmux.

// inboxCode is the pending device code.
type inboxCode struct {
	user, uri string
	expires   time.Time
}

type (
	inboxCodeMsg struct {
		code    deviceCodeReply
		expires time.Time
	}
	inboxSignedMsg struct {
		src   *inboxSource
		login string
		err   error
	}
	inboxSecondMsg   struct{} // the countdown's tick
	inboxApprovedMsg struct{} // "Approved by @login" has been seen; the inbox shows
)

// inboxSignin is the login screen's state: the check running, the code it
// shows, and how it ended.
type inboxSignin struct {
	run      func() tea.Msg // the presence check, again for a new code
	code     *inboxCode
	running  bool
	failed   string // why the code gave no approval, with the next step
	approved string // the approver, shown for a moment before the inbox
	copied   bool
	browser  bool   // a browser opens here (Enter); else the link is for any device
	tmuxClip string // tmux's set-clipboard, inside tmux: OSC 52 passes only when it is "on"
	err      error  // the check's refusal, or the person quit: aayu inbox ends with it
}

// errNotApproved is a quit from the login screen.
var errNotApproved = Errorf(ExitBlocked, "inbox was not approved; nothing was done")

// screen reports whether the login screen shows instead of the inbox.
func (s *inboxSignin) screen() bool {
	return s != nil && (s.code != nil || s.failed != "" || s.approved != "")
}

func (m *inboxModel) signinCmd() tea.Cmd {
	s := m.signin
	s.running, s.failed, s.copied = true, "", false
	return func() tea.Msg { return s.run() }
}

func secondTick() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return inboxSecondMsg{} })
}

// signinUpdate handles the login screen's messages; ok is false for any other.
func (m *inboxModel) signinUpdate(msg tea.Msg) (cmd tea.Cmd, ok bool) {
	s := m.signin
	if s == nil {
		return nil, false
	}
	switch msg := msg.(type) {
	case inboxCodeMsg:
		s.code = &inboxCode{user: msg.code.UserCode, uri: msg.code.VerificationURI, expires: msg.expires}
		m.status, m.dirty = "", true
		if m.timer {
			return tea.Batch(secondTick(), m.spinTick()), true
		}
		return nil, true
	case inboxSecondMsg:
		m.dirty = true
		if s.code != nil && s.running && m.timer {
			return secondTick(), true
		}
		return nil, true
	case inboxApprovedMsg:
		s.approved, m.dirty = "", true
		m.signin = nil
		return nil, true
	case inboxSignedMsg:
		s.running, m.dirty = false, true
		if msg.err != nil {
			if s.code == nil { // refused before any code: aayu inbox ends with the refusal, as before
				s.err = msg.err
				return tea.Quit, true
			}
			s.failed = msg.err.Error()
			var e *Error
			if errors.As(msg.err, &e) && e.Next != "" {
				// Enter asks for a new code here, so the advice to run the
				// command again is left out.
				next, _, _ := strings.Cut(e.Next, "ask for the command again")
				if next = strings.TrimRight(next, "; ,"); next != "" {
					s.failed += "; " + next
				}
			}
			m.status = ""
			return nil, true
		}
		shown := s.code != nil
		s.code = nil // gone from memory once approved
		cmds := []tea.Cmd{m.signedIn(msg.src, msg.login), m.spinTick()}
		if !shown || !m.timer {
			m.signin = nil
			return tea.Batch(cmds...), true
		}
		s.approved = msg.login
		if m.timer {
			cmds = append(cmds, tea.Tick(1200*time.Millisecond, func(time.Time) tea.Msg { return inboxApprovedMsg{} }))
		}
		return tea.Batch(cmds...), true
	}
	return nil, false
}

// signinKey is a key on the login screen, or while the check runs.
func (m *inboxModel) signinKey(k string) tea.Cmd {
	s := m.signin
	m.dirty = true
	switch k {
	case "q", "esc", "ctrl+c":
		s.err = errNotApproved
		return tea.Quit
	}
	switch {
	case s.approved != "":
	case s.failed != "" && k == "enter":
		s.code = nil
		m.status = "Asking GitHub for a new code…"
		return tea.Batch(m.signinCmd(), m.spinTick())
	case s.code == nil:
	case k == "enter" || k == "o":
		if !s.browser {
			m.status = "No browser here: open the link on any device; c copies the code."
			return nil
		}
		m.open(s.code.uri)
		m.status = "Opened " + strings.TrimPrefix(s.code.uri, "https://") + " in your browser; enter the code there."
	case k == "c":
		s.copied = true
		m.status = "" // the body says Copied, next to the code
		return tea.SetClipboard(s.code.user)
	}
	return nil
}

// signinView is the login screen's body, w cells wide and h lines tall.
func (m *inboxModel) signinView(w, h int) []string {
	s := m.signin
	tw := min(w, 76)
	var out []string
	add := func(r ui.Role, text string) {
		for _, l := range m.wrap(text, tw) {
			out = append(out, m.style(r, l))
		}
	}
	plain := func(text string) {
		out = append(out, m.wrap(text, tw)...)
	}
	out = append(out, "")
	if s.approved != "" {
		out = append(out, m.style(ui.OK, "✓ Approved by @"+s.approved), "")
		plain("Loading your inbox…")
		return out
	}
	add(ui.Title, "Approve this session on GitHub")
	plain("aayu inbox acts as you on human-only steps, so GitHub asks you, once per session, to prove it is you at this terminal.")
	out = append(out, "")
	c := s.code
	link := ansi.SetHyperlink(c.uri) + c.uri + ansi.ResetHyperlink()
	open := "Enter opens it in your browser"
	if !s.browser {
		open = "open it on any device"
	}
	out = append(out, inboxBold.Render("1")+"  Go to "+link)
	out = append(out, "   "+m.style(ui.Muted, open))
	out = append(out, "", inboxBold.Render("2")+"  Enter this code")
	spaced := strings.Join(strings.Split(c.user, ""), " ")
	role := ui.Accent
	if s.failed != "" {
		role = ui.Muted
	}
	bw := ansi.StringWidth(spaced) + 6
	out = append(out,
		"   "+m.style(role, "┌"+strings.Repeat("─", bw)+"┐"),
		"   "+m.style(role, "│   ")+m.style(role, inboxBold.Render(spaced))+m.style(role, "   │"),
		"   "+m.style(role, "└"+strings.Repeat("─", bw)+"┘"))
	switch {
	case s.failed != "":
	case s.copied:
		out = append(out, "   "+m.style(ui.OK, "✓ Copied"))
		for _, l := range m.wrap("If it is not in your clipboard, select the code with the mouse.", tw-3) {
			out = append(out, "   "+m.style(ui.Muted, l))
		}
	default:
		out = append(out, "   "+m.style(ui.Muted, "c copies it"))
	}
	out = append(out, "")
	if s.failed != "" {
		add(ui.Fail, "✗ "+upperFirst(s.failed))
		out = append(out, "")
		plain("Enter asks GitHub for a new code; q quits.")
		return out
	}
	left := c.expires.Sub(m.now()).Round(time.Second)
	sp := m.spinner() // the wrapped line hangs under its text, not under the spinner
	for i, l := range m.wrap("Waiting for GitHub… this screen moves on by itself once you approve.", tw-ansi.StringWidth(sp)) {
		if i > 0 {
			sp = strings.Repeat(" ", ansi.StringWidth(sp))
		}
		out = append(out, sp+l)
	}
	out = append(out, m.style(ui.Muted, fmt.Sprintf("Expires in %d:%02d (at %s).", int(max(left, 0).Minutes()), int(max(left, 0).Seconds())%60, c.expires.UTC().Format("15:04 UTC"))))
	out = append(out, "")
	add(ui.Muted, "Approve only if you asked for this command: aayu inbox. The code is shown only here: aayu never logs it.")
	if s.tmuxClip != "" && s.tmuxClip != "on" {
		out = append(out, "")
		add(ui.Warn, fmt.Sprintf("Inside tmux the copy reaches your clipboard only with set-clipboard on (it is %s): run tmux set -s set-clipboard on, or select the code with the mouse.", s.tmuxClip))
	}
	if len(out) > h {
		out = out[:h]
	}
	return out
}

// signinKeys is the login screen's key bar.
func (m *inboxModel) signinKeys() []string {
	s := m.signin
	switch {
	case s.approved != "":
		return nil
	case s.failed != "":
		return []string{"Enter", "new code", "q", "quit"}
	case s.code == nil:
		return []string{"q", "quit"}
	case s.browser:
		return []string{"Enter", "open GitHub", "c", "copy the code", "q", "quit"}
	}
	return []string{"c", "copy the code", "q", "quit"}
}

// browserHere reports whether openURL can open a browser on this machine:
// macOS and Windows have one; elsewhere it takes a display and xdg-open, the
// opener openURL runs ($BROWSER alone is not enough: openURL does not use it).
func browserHere(getenv func(string) string) bool {
	switch {
	case runtime.GOOS == "darwin" || runtime.GOOS == "windows":
		return true
	case getenv("DISPLAY") == "" && getenv("WAYLAND_DISPLAY") == "":
		return false
	}
	_, err := exec.LookPath("xdg-open")
	return err == nil
}

// tmuxClipboard is tmux's set-clipboard when aayu runs inside tmux, else "".
func tmuxClipboard(getenv func(string) string) string {
	if getenv("TMUX") == "" {
		return ""
	}
	out, err := exec.Command("tmux", "show", "-sv", "set-clipboard").Output() // #nosec G204 -- a fixed read of tmux's own option
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
