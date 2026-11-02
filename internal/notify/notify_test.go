package notify

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
)

// Story #1254, criterion 5: outside a person's window a page is sent to nobody;
// at the next window's start one message lists every page that waited, in
// order, and each is sent once. An urgent page (a halt) never waits.
func TestPagesWaitForAWindow(t *testing.T) {
	w, err := config.NewWindows("Asia/Kolkata", []string{"10:30-11:30", "18:30-19:30"})
	if err != nil {
		t.Fatal(err)
	}
	clock, err := config.NewClock("Asia/Kolkata", "Mon-Fri 09:00-18:00")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 2, 14, 0, 0, 0, w.Loc) // Monday afternoon: in business hours, between the windows
	sink := &Fake{}
	s := &Sender{Sink: sink, Clock: clock, Windows: &w, Now: func() time.Time { return now }, State: filepath.Join(t.TempDir(), "notify.json")}
	ctx := context.Background()
	for _, p := range []Page{
		{Key: "decision-sla:o/r#5", Subject: "Decision #5 is past its SLA", Body: "Accept it with `aayu decide 5 --accept <option>`."},
		{Key: "inspect:o/r#7", Subject: "inspect: PR #7 needs you"},
		{Key: "decision-sla:o/r#5", Subject: "Decision #5 is past its SLA"}, // the same request again: queued once
	} {
		if st, err := s.Page(ctx, p); err != nil || st != Deferred && st != Deduped {
			t.Fatalf("%s at 14:00: %q, %v", p.Key, st, err)
		}
	}
	if n, err := s.Flush(ctx); err != nil || n != 0 || len(sink.Pages) != 0 {
		t.Fatalf("flush at 14:00: %d, %v, sent %+v", n, err, sink.Pages)
	}
	if st, err := s.Page(ctx, Page{Key: "halt:o/r", Urgent: true, Subject: "aayudha halted o/r"}); err != nil || st != Sent || len(sink.Pages) != 1 {
		t.Fatalf("an urgent page at 14:00: %q, %v, sent %+v", st, err, sink.Pages)
	}
	now = time.Date(2026, 11, 2, 18, 29, 0, 0, w.Loc)
	if n, _ := s.Flush(ctx); n != 0 || len(sink.Pages) != 1 {
		t.Fatalf("flush at 18:29: %d, sent %+v", n, sink.Pages)
	}
	now = time.Date(2026, 11, 2, 18, 30, 0, 0, w.Loc)
	if n, err := s.Flush(ctx); err != nil || n != 2 || len(sink.Pages) != 2 {
		t.Fatalf("flush at 18:30: %d, %v, sent %+v", n, err, sink.Pages)
	}
	msg := sink.Pages[1]
	if msg.Subject != "aayudha: 2 requests for your window" || !strings.Contains(msg.Body, "1. Decision #5 is past its SLA\n   Accept it with `aayu decide 5 --accept <option>`.\n") ||
		!strings.Contains(msg.Body, "2. inspect: PR #7 needs you\n") {
		t.Fatalf("the window's one message: %+v", msg)
	}
	// Each request went out once: asked again in the window, it is not sent again today.
	if st, _ := s.Page(ctx, Page{Key: "inspect:o/r#7", Subject: "inspect: PR #7 needs you"}); st != Deduped || len(sink.Pages) != 2 {
		t.Fatalf("the same request after the window's message: %q, sent %d", st, len(sink.Pages))
	}
	// In a window, a new request with others waiting goes out with them, as one message.
	now = time.Date(2026, 11, 2, 20, 0, 0, 0, w.Loc)
	_, _ = s.Page(ctx, Page{Key: "a", Subject: "first"})
	now = time.Date(2026, 11, 3, 10, 45, 0, 0, w.Loc)
	if st, err := s.Page(ctx, Page{Key: "b", Subject: "second"}); err != nil || st != Sent || len(sink.Pages) != 3 ||
		!strings.Contains(sink.Pages[2].Body, "1. first\n") || !strings.Contains(sink.Pages[2].Body, "2. second\n") {
		t.Fatalf("a request in a window with one waiting: %q, %v, sent %+v", st, err, sink.Pages)
	}
	// The same request asked again in the window sends what waited, itself once among them.
	now = time.Date(2026, 11, 3, 12, 0, 0, 0, w.Loc)
	_, _ = s.Page(ctx, Page{Key: "c", Subject: "third"})
	_, _ = s.Page(ctx, Page{Key: "d", Subject: "fourth"})
	now = time.Date(2026, 11, 3, 18, 31, 0, 0, w.Loc)
	if st, err := s.Page(ctx, Page{Key: "c", Subject: "third"}); err != nil || st != Sent || len(sink.Pages) != 4 ||
		!strings.Contains(sink.Pages[3].Body, "1. third\n") || !strings.Contains(sink.Pages[3].Body, "2. fourth\n") || strings.Count(sink.Pages[3].Body, "third") != 1 {
		t.Fatalf("a waiting request asked again in a window: %q, %v, sent %+v", st, err, sink.Pages)
	}
}

// queueFunc is a Queue for tests.
type queueFunc func(Page) error

func (f queueFunc) Add(_ context.Context, p Page) error { return f(p) }

// Goldfish review, round 2, findings 3 and 11: with a Queue (the ledger's
// outbox), a page outside a window waits there, not in this machine's state.
// A queue that cannot take it keeps it here and says so; the next window's
// message sends it.
func TestPagesWaitInTheQueue(t *testing.T) {
	w, err := config.NewWindows("Asia/Kolkata", []string{"10:30-11:30"})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 2, 14, 0, 0, 0, w.Loc)
	var queued []Page
	var qerr error
	sink := &Fake{}
	s := &Sender{Sink: sink, Clock: config.Clock{Loc: w.Loc}, Windows: &w, Now: func() time.Time { return now }, State: filepath.Join(t.TempDir(), "notify.json"),
		Queue: queueFunc(func(p Page) error { queued = append(queued, p); return qerr })}
	ctx := context.Background()
	if st, err := s.Page(ctx, Page{Key: "a", Subject: "first"}); err != nil || st != Deferred || len(queued) != 1 {
		t.Fatalf("a page at 14:00: %q, %v, queued %+v", st, err, queued)
	}
	qerr = errors.New("the ledger is not reachable")
	if st, err := s.Page(ctx, Page{Key: "b", Subject: "second"}); st != Deferred || err == nil || !strings.Contains(err.Error(), "on this machine only") {
		t.Fatalf("a page the queue refused: %q, %v", st, err)
	}
	now = time.Date(2026, 11, 3, 10, 30, 0, 0, w.Loc)
	if n, err := s.Window(ctx, "window:x", nil); err != nil || n != 1 || len(sink.Pages) != 1 || !strings.Contains(sink.Pages[0].Body, "1. second\n") || strings.Contains(sink.Pages[0].Body, "first") {
		t.Fatalf("the window's message: %d, %v, %+v", n, err, sink.Pages)
	}
}
