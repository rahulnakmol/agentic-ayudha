package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/notify"
)

// A person's windows (Story #1254): [time] windows are the only times aayu
// asks a person to act. Outside them no work waits for a person: a mission
// past its time limit or its budget still starts stories, and its card waits
// for the next window. A request for action made outside a window (a page, a
// comment approval batch, a mission's ask or card, a Ready to ship comment)
// is not sent. In the window, the first `aayu mission tick` or `aayu mission
// next` shows the cards that came due, then sends one message that lists
// every card of the inbox and every page that waited. The inbox is on GitHub
// and the ledger, so the message lists every request, from any checkout.

// WindowsPath is the ledger file that records each window whose one message
// was sent, so each window has one, whichever run sends it.
const WindowsPath = "notify/windows.jsonl"

// windowSent is one line of WindowsPath.
type windowSent struct {
	Window string `json:"window"` // the window's start, RFC 3339 UTC
	At     string `json:"at"`
	Items  int    `json:"items,omitempty"`
}

// outsideWindow is when the next window starts, or the zero time when now is
// inside one (or the config has no windows, or cannot be read).
func (e *Env) outsideWindow() time.Time {
	l, err := e.Config()
	if err != nil {
		return time.Time{}
	}
	w, err := l.Config.PersonWindows()
	if now := e.Now(); err == nil && !w.Within(now) {
		return w.Next(now)
	}
	return time.Time{}
}

// waitsForWindow is the refusal of a request for a person's action outside
// a window: what waits, and when the window opens.
func (e *Env) waitsForWindow(what string, next time.Time) error {
	return WithNext(Errorf(ExitBlocked, "%s asks a person to act, and outside the person's windows ([time] windows) nobody is asked: it waits for the window at %s, and nothing was recorded",
		what, next.Format("Mon 2 Jan 15:04 MST")), "run it again in the window; aayu mission tick does it then")
}

// askLater queues a request for a person's action for the next window's one
// message, and says what became of it. A request that cannot be queued is
// said on stderr; the command that asks goes on.
func (e *Env) askLater(ctx context.Context, key, subject, body string) string {
	st := e.page(ctx, notify.Page{Key: key, Subject: subject, Body: body})
	if strings.HasPrefix(st, "not sent") || strings.HasPrefix(st, "failed") {
		fmt.Fprintf(e.Err, "aayu: %q was not queued for the next window (%s); the inbox still holds what it asks for\n", subject, st)
	}
	return st
}

// windowMessage sends, inside a person's window, the window's one message:
// every card of the inbox, then every page that waited, once per window. A
// repository with no windows flushes waiting pages as before. A failure is
// said on stderr and leaves the window to the next run.
func (e *Env) windowMessage(ctx context.Context, w *ledger.Writer) {
	if err := e.sendWindow(ctx, w); err != nil {
		fmt.Fprintf(e.Err, "aayu: the window's message was not sent (%v); the next run in the window tries again\n", oneLineErr(err))
	}
}

func (e *Env) sendWindow(ctx context.Context, w *ledger.Writer) error {
	l, err := e.Config()
	if err != nil {
		return err
	}
	win, err := l.Config.PersonWindows()
	if err != nil {
		return err
	}
	n, err := e.Notifier(ctx)
	if err != nil {
		return err
	}
	now := e.Now()
	if len(win.Spans) == 0 {
		_, err := n.Flush(ctx)
		return err
	}
	if !win.Within(now) {
		return nil
	}
	key := win.Start(now).UTC().Format(time.RFC3339)
	t, err := w.Read(ctx)
	if err != nil {
		return err
	}
	raw, err := t.File(ctx, WindowsPath)
	tip := t.Tip
	t.Close()
	if err = ignoreMissing(err); err != nil {
		return err
	}
	for _, line := range strings.Split(string(raw), "\n") {
		var ws windowSent
		if json.Unmarshal([]byte(line), &ws) == nil && ws.Window == key {
			return nil // this window's message was sent
		}
	}
	_, items, err := e.inboxItems(ctx)
	if err != nil {
		return err
	}
	var pages []notify.Page
	for _, it := range items {
		if it.Kind == InboxNotReady { // nothing for a person yet
			continue
		}
		pages = append(pages, notify.Page{Key: it.key(), Subject: fmt.Sprintf("%s #%d: %s", it.kind().Word, it.Number, it.Title), Body: it.Why})
	}
	// Claim the window on the ledger first: of two runs, only the one whose line lands sends.
	line, _ := json.Marshal(windowSent{Window: key, At: now.UTC().Format(time.RFC3339), Items: len(pages)})
	if _, err := w.Write(ctx, ledger.Commit{Message: "notify: the message of the window at " + key, Base: tip,
		Files: map[string][]byte{WindowsPath: append(append(raw, line...), '\n')}}); errors.Is(err, ledger.ErrMoved) {
		return nil // another run wrote first; the next run reads its line, or claims the window
	} else if err != nil {
		return err
	}
	if _, err = n.Window(ctx, "window:"+key, pages); err != nil {
		// Not sent: give the window back, so the next run in it sends the message.
		if t, rerr := w.Read(ctx); rerr == nil {
			cur, _ := t.File(ctx, WindowsPath)
			tip := t.Tip
			t.Close()
			kept := strings.Replace(string(cur), string(line)+"\n", "", 1)
			_, _ = w.Write(ctx, ledger.Commit{Message: "notify: the message of the window at " + key + " was not sent", Base: tip, Files: map[string][]byte{WindowsPath: []byte(kept)}})
		}
	}
	return err
}
