package cli

import (
	"context"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/notify"
)

// A person's windows (Story #1254): [time] windows are the only times aayu
// asks a person to act. Outside them no work waits for a person: a mission
// past its time limit or its budget still starts stories, and its card waits
// for the next window. A request for action made outside a window (a page, a
// comment approval batch, a mission card) is queued in the [notify] sender,
// and the next window's first page or flush sends one message that lists
// every queued request.

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

// askLater queues a request for a person's action for the next window's one
// message, and says what became of it. It never fails the command that asks:
// the request itself waits where it is (a card, a Decision) for the window.
func (e *Env) askLater(ctx context.Context, key, subject, body string) string {
	return e.page(ctx, notify.Page{Key: key, Subject: subject, Body: body})
}

// flushRequests sends, inside a window, the one message that lists every
// request queued since the last window. Outside one it sends nothing.
func (e *Env) flushRequests(ctx context.Context) {
	if n, err := e.Notifier(ctx); err == nil {
		_, _ = n.Flush(ctx)
	}
}
