package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
	"github.com/rahulnakmol/agentic-ayudha/internal/notify"
)

// A person's windows (Story #1254): [time] windows are the only times aayu
// asks a person to act. Outside them no work waits for a person: a mission
// past its time limit or its budget still starts stories, and its card waits
// for the next window. A request for action made outside a window (a page, a
// comment approval request, a mission's ask, a Ready to ship comment) is not
// sent: it waits in the ledger's outbox, which every checkout and runner
// reads. In the window, the first `aayu mission tick` or `aayu mission next`
// posts the comments that waited and shows the cards that came due, then
// sends one message that lists every card of the inbox and every page that
// waited. The safety tick runs at half past each hour, and a window starts at
// half past (config.Windows), so the tick opens each window.

// WindowsPath is the ledger file that records each window's one message: a
// run claims the window for windowLease, sends, and records it sent, so each
// window has one message, whichever run sends it, and a run that stops
// half-way leaves the window to the next run.
const WindowsPath = "notify/windows.jsonl"

// OutboxPath is the ledger file of the requests for a person's action that
// wait for the next window, one per key.
const OutboxPath = "notify/outbox.jsonl"

// windowLease is how long a claim on a window's message holds: a run that
// claimed it and stopped before it recorded the message sent gives the window
// to the next run after it.
const windowLease = 10 * time.Minute

// windowSent is one line of WindowsPath.
type windowSent struct {
	Window string `json:"window"`          // the window's start, RFC 3339 UTC
	State  string `json:"state,omitempty"` // "claimed" until Until, then "sent"
	At     string `json:"at"`
	Until  string `json:"until,omitempty"`
	Items  int    `json:"items,omitempty"`
}

// queued is one line of OutboxPath: one request that waits for a window.
type queued struct {
	Key     string         `json:"key"`
	At      string         `json:"at"`
	Page    *notify.Page   `json:"page,omitempty"`    // listed in the window's message
	Ask     *queuedAsk     `json:"ask,omitempty"`     // a mission's ask, posted on its Feature in the window
	Comment *queuedComment `json:"comment,omitempty"` // a Ready to ship comment, posted on its PR in the window
}

type queuedAsk struct {
	Feature int        `json:"feature"`
	Ask     missionAsk `json:"ask"`
}

// queuedComment is a comment for pull request PR at Head: a PR that closed or
// moved on since has nothing to ask, and is dropped.
type queuedComment struct {
	PR     int    `json:"pr"`
	Head   string `json:"head"`
	Story  int    `json:"story,omitempty"`
	Kind   string `json:"kind"`
	Marker string `json:"marker"`
	Body   string `json:"body"`
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

// askLater queues a request for a person's action for the next window's one
// message, and says what became of it. A request that waits only on this
// machine, or not at all, is said on stderr; the command that asks goes on.
func (e *Env) askLater(ctx context.Context, key, subject, body string) string {
	st := e.page(ctx, notify.Page{Key: key, Subject: subject, Body: body})
	if st != notify.Deferred && st != notify.Deduped {
		fmt.Fprintf(e.Err, "aayu: %q is not in the ledger's outbox for the next window (%s); the inbox still holds what it asks for\n", subject, st)
	}
	return st
}

// readNotify reads the window records and the outbox at the ledger's tip.
func readNotify(ctx context.Context, w *ledger.Writer) (ws []windowSent, out []queued, tip string, err error) {
	t, err := w.Read(ctx)
	if err != nil {
		return nil, nil, "", err
	}
	defer t.Close()
	for _, f := range []struct {
		path string
		add  func([]byte) error
	}{
		{WindowsPath, func(b []byte) error { var v windowSent; err := json.Unmarshal(b, &v); ws = append(ws, v); return err }},
		{OutboxPath, func(b []byte) error { var v queued; err := json.Unmarshal(b, &v); out = append(out, v); return err }},
	} {
		raw, err := t.File(ctx, f.path)
		if err = ignoreMissing(err); err != nil {
			return nil, nil, "", err
		}
		for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
			if line == "" {
				continue
			}
			if err := f.add([]byte(line)); err != nil {
				return nil, nil, "", fmt.Errorf("ledger %s: %w", f.path, err)
			}
		}
	}
	return ws, out, t.Tip, nil
}

// editNotify changes the window records and the outbox at the ledger's tip.
// edit reports whether it changed anything; another write that moved the
// ledger since the read makes it read and edit again.
func editNotify(ctx context.Context, w *ledger.Writer, msg string, edit func(ws *[]windowSent, out *[]queued) bool) error {
	for try := 0; ; try++ {
		ws, out, tip, err := readNotify(ctx, w)
		if err != nil {
			return err
		}
		if !edit(&ws, &out) {
			return nil
		}
		files := map[string][]byte{}
		for path, lines := range map[string]any{WindowsPath: ws, OutboxPath: out} {
			var b strings.Builder
			switch v := lines.(type) {
			case []windowSent:
				for _, l := range v {
					j, _ := json.Marshal(l)
					b.Write(append(j, '\n'))
				}
			case []queued:
				for _, l := range v {
					j, _ := json.Marshal(l)
					b.Write(append(j, '\n'))
				}
			}
			files[path] = []byte(b.String())
		}
		_, err = w.Write(ctx, ledger.Commit{Message: msg, Base: tip, Files: files})
		if !errors.Is(err, ledger.ErrMoved) || try == 4 {
			return err
		}
	}
}

// enqueue puts q in the ledger's outbox for the next window, once per key.
func (e *Env) enqueue(ctx context.Context, w *ledger.Writer, q queued) error {
	q.At = e.Now().UTC().Format(time.RFC3339)
	return editNotify(ctx, w, "notify: "+q.Key+" waits for a person's window", func(_ *[]windowSent, out *[]queued) bool {
		if slices.ContainsFunc(*out, func(o queued) bool { return o.Key == q.Key }) {
			return false
		}
		*out = append(*out, q)
		return true
	})
}

// enqueueHere puts a request in the outbox, with the person's own ledger
// writer when the caller has none.
func (e *Env) enqueueHere(ctx context.Context, q queued) error {
	_, c, w, err := e.missionLedger(ctx)
	if err != nil {
		return err
	}
	if c.DryRun() { // a dry run writes nothing
		return nil
	}
	return e.enqueue(ctx, w, q)
}

// ledgerQueue is the notify.Queue of a repository with person windows: the
// ledger's outbox.
type ledgerQueue struct{ e *Env }

func (q ledgerQueue) Add(ctx context.Context, p notify.Page) error {
	return q.e.enqueueHere(ctx, queued{Key: "page:" + p.Key, Page: &p})
}

// windowMessage sends, inside a person's window, the window's one message:
// it posts the comments that waited, then lists every card of the inbox and
// every page that waited, once per window. A repository with no windows
// flushes waiting pages as before. A failure is said on stderr and leaves
// the window and its requests to the next run.
func (e *Env) windowMessage(ctx context.Context, c *github.Client, w *ledger.Writer, repo string) {
	if err := e.sendWindow(ctx, c, w, repo); err != nil {
		fmt.Fprintf(e.Err, "aayu: the window's message was not sent (%v); the next run in the window tries again, and what waits stays in the ledger's outbox\n", oneLineErr(err))
	}
}

func (e *Env) sendWindow(ctx context.Context, c *github.Client, w *ledger.Writer, repo string) (err error) {
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
	if !win.Within(now) || c.DryRun() {
		return nil
	}
	key := win.Start(now).UTC().Format(time.RFC3339)
	// Claim the window on the ledger first: of two runs, only the one whose claim lands sends, and a claim
	// whose run stopped lapses.
	var waiting []queued
	claimed := false
	if err := editNotify(ctx, w, "notify: a run claims the message of the window at "+key, func(ws *[]windowSent, out *[]queued) bool {
		claimed, waiting = false, *out
		for _, s := range *ws {
			if s.Window != key {
				continue
			}
			if until, _ := time.Parse(time.RFC3339, s.Until); s.State != "claimed" || now.Before(until) {
				return false // sent, or another run sends it now
			}
		}
		*ws = slices.DeleteFunc(*ws, func(s windowSent) bool { return s.Window == key })
		*ws = append(*ws, windowSent{Window: key, State: "claimed", At: now.UTC().Format(time.RFC3339), Until: now.Add(windowLease).UTC().Format(time.RFC3339)})
		claimed = true
		return true
	}); err != nil || !claimed {
		return err
	}
	done, pages, err := e.postWaiting(ctx, c, repo, waiting)
	sent, items := false, 0
	defer func() { // record what this run did: the requests it handled leave the outbox, and the window is sent or free again
		if ferr := editNotify(ctx, w, "notify: the message of the window at "+key, func(ws *[]windowSent, out *[]queued) bool {
			*out = slices.DeleteFunc(*out, func(q queued) bool { return slices.Contains(done, q.Key) })
			*ws = slices.DeleteFunc(*ws, func(s windowSent) bool { return s.Window == key })
			if sent {
				*ws = append(*ws, windowSent{Window: key, State: "sent", At: e.Now().UTC().Format(time.RFC3339), Items: items})
			}
			return true
		}); ferr != nil && err == nil {
			err = ferr
		}
	}()
	if err != nil {
		return err
	}
	_, inbox, err := e.inboxItems(ctx)
	if err != nil {
		return err
	}
	var list []notify.Page
	for _, it := range inbox {
		if it.Kind == InboxNotReady { // nothing for a person yet
			continue
		}
		list = append(list, notify.Page{Key: it.key(), Subject: fmt.Sprintf("%s #%d: %s", it.kind().Word, it.Number, it.Title), Body: it.Why})
	}
	if !win.Within(e.Now()) { // the window ended while this run read: what waits, waits for the next one
		return nil
	}
	if items, err = n.Window(ctx, "window:"+key, append(list, pages...)); err != nil {
		return err
	}
	sent = items > 0
	if sent {
		for _, q := range waiting {
			if q.Page != nil {
				done = append(done, q.Key)
			}
		}
	}
	return nil
}

// postWaiting posts each comment and ask that waited for the window, and
// returns the keys it handled and the pages for the window's message. It
// stops at the first that cannot be posted, which waits for the next run.
func (e *Env) postWaiting(ctx context.Context, c *github.Client, repo string, waiting []queued) (done []string, pages []notify.Page, err error) {
	owner, name, _ := github.SplitRepo(repo)
	for _, q := range waiting {
		switch {
		case q.Page != nil:
			pages = append(pages, *q.Page)
			continue
		case q.Ask != nil:
			err = e.postAsk(ctx, c, repo, q.Ask.Feature, q.Ask.Ask)
		case q.Comment != nil:
			cm := q.Comment
			pr, _, perr := c.REST.PullRequests.Get(ctx, owner, name, cm.PR)
			if err = perr; err != nil {
				err = Wrap(ExitExternal, err)
				break
			}
			if pr.GetState() != "open" || pr.GetHead().GetSHA() != cm.Head {
				break // closed, or moved on since: it asks for nothing now
			}
			me, _, uerr := c.REST.Users.Get(ctx, "")
			if err = uerr; err != nil {
				err = Wrap(ExitExternal, err)
				break
			}
			err = upsertOwnComment(ctx, c, repo, cm.PR, me.GetLogin(), cm.Marker, cm.Body, actions.Action{Kind: cm.Kind, Story: fmt.Sprintf("#%d", cm.Story)})
		}
		if err != nil {
			return done, pages, err
		}
		done = append(done, q.Key)
	}
	return done, pages, nil
}
