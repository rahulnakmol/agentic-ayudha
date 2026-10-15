// Package notify sends pages through the [notify] channel (WP1.6): a Slack
// webhook, email over SMTP, or a GitHub mention comment. Pages are deduplicated
// per key per local day, and non-urgent pages wait for business hours ([time]).
// Later WPs (2.8 digest, 5.5 blocked stories, 5.6 decision SLAs, 5.7 streaks)
// only call Sender.Page.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/smtp"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
	"github.com/rahulnakmol/agentic-ayudha/internal/config"
)

// Page is one notification.
type Page struct {
	Key     string `json:"key"`    // dedupe key, e.g. budget:per_day; one page per key per local day
	Urgent  bool   `json:"urgent"` // Sev0/Sev1: ignores business hours
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

// Sink delivers a page on one channel.
type Sink interface {
	Channel() string
	Deliver(ctx context.Context, p Page) error
}

// Statuses Page returns.
const (
	Sent     = "sent"
	Deduped  = "deduped"  // already paged today under this key
	Deferred = "deferred" // outside business hours; delivered by the next Flush in hours
	NoSink   = "no-channel"
)

// Sender pages through a sink with dedupe and business-hours gating.
type Sender struct {
	Sink    Sink         // nil when [notify] is not configured: pages are logged, not sent
	Clock   config.Clock // business hours and the local day for dedupe
	Now     func() time.Time
	State   string       // JSON file holding sent keys and deferred pages
	Actions *actions.Log // every delivery is an external write (ground rule 6); nil skips logging

	mu sync.Mutex
}

type state struct {
	Sent    map[string]time.Time `json:"sent"` // key@YYYY-MM-DD
	Pending []Page               `json:"pending"`
}

func (s *Sender) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Sender) load() (state, error) {
	st := state{Sent: map[string]time.Time{}}
	b, err := os.ReadFile(s.State)
	if errors.Is(err, os.ErrNotExist) {
		return st, nil
	}
	if err != nil {
		return st, err
	}
	if err := json.Unmarshal(b, &st); err != nil {
		return st, err
	}
	if st.Sent == nil {
		st.Sent = map[string]time.Time{}
	}
	return st, nil
}

func (s *Sender) save(st state) error {
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.State), 0o755); err != nil {
		return err
	}
	return os.WriteFile(s.State, b, 0o644)
}

func (s *Sender) dayKey(key string, t time.Time) string {
	return key + "@" + t.In(s.Clock.Loc).Format("2006-01-02")
}

// Page sends p once per key per local day. Outside business hours a
// non-urgent page is deferred to the next Flush inside business hours.
func (s *Sender) Page(ctx context.Context, p Page) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, err := s.load()
	if err != nil {
		return "", err
	}
	now := s.now()
	dk := s.dayKey(p.Key, now)
	if _, ok := st.Sent[dk]; ok {
		return Deduped, nil
	}
	for _, q := range st.Pending {
		if q.Key == p.Key {
			return Deduped, nil
		}
	}
	if !p.Urgent && !s.Clock.Within(now) {
		st.Pending = append(st.Pending, p)
		return Deferred, s.save(st)
	}
	status, err := s.deliver(ctx, p)
	if err != nil {
		return "", err
	}
	st.Sent[dk] = now
	return status, s.save(st)
}

// Flush delivers deferred pages when inside business hours.
func (s *Sender) Flush(ctx context.Context) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	if !s.Clock.Within(now) {
		return 0, nil
	}
	st, err := s.load()
	if err != nil {
		return 0, err
	}
	n := 0
	var keep []Page
	for _, p := range st.Pending {
		if _, err := s.deliver(ctx, p); err != nil {
			keep = append(keep, p)
			continue
		}
		st.Sent[s.dayKey(p.Key, now)] = now
		n++
	}
	st.Pending = keep
	return n, s.save(st)
}

func (s *Sender) deliver(ctx context.Context, p Page) (string, error) {
	if s.Sink == nil {
		return NoSink, nil
	}
	if s.Actions == nil {
		return Sent, s.Sink.Deliver(ctx, p)
	}
	a := actions.Action{Kind: "notify." + s.Sink.Channel(), Target: p.Key, Payload: map[string]any{"subject": p.Subject, "urgent": p.Urgent}}
	_, err := s.Actions.Do(ctx, a, func(ctx context.Context) (actions.Outcome, error) {
		return actions.Outcome{}, s.Sink.Deliver(ctx, p)
	})
	if s.Actions.DryRun() {
		return "planned", err
	}
	return Sent, err
}

// Fake records pages instead of sending them (tests).
type Fake struct {
	mu    sync.Mutex
	Pages []Page
}

// Channel implements Sink.
func (*Fake) Channel() string { return "fake" }

// Deliver implements Sink.
func (f *Fake) Deliver(_ context.Context, p Page) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Pages = append(f.Pages, p)
	return nil
}

// File appends each page as one JSON line to Path: the outbox of a
// repository with no paging channel, kept as an artifact by the Actions loops (WP7.1).
type File struct{ Path string }

// Channel implements Sink.
func (File) Channel() string { return "file" }

// Deliver implements Sink.
func (f File) Deliver(_ context.Context, p Page) error {
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(f.Path), 0o755); err != nil {
		return err
	}
	fh, err := os.OpenFile(f.Path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	_, err = fh.Write(append(b, '\n'))
	if cerr := fh.Close(); err == nil {
		err = cerr
	}
	return err
}

// Slack posts to an incoming-webhook URL.
type Slack struct {
	WebhookURL string
	HTTP       *http.Client
}

// Channel implements Sink.
func (Slack) Channel() string { return "slack" }

// Deliver implements Sink.
func (s Slack) Deliver(ctx context.Context, p Page) error {
	body, _ := json.Marshal(map[string]string{"text": "*" + p.Subject + "*\n" + p.Body})
	return post(ctx, s.HTTP, s.WebhookURL, nil, body)
}

// Email sends through an SMTP server with STARTTLS and PLAIN auth.
type Email struct {
	Addr, From, To, Password string
}

// Channel implements Sink.
func (Email) Channel() string { return "email" }

// Deliver implements Sink.
func (e Email) Deliver(_ context.Context, p Page) error {
	host, _, _ := strings.Cut(e.Addr, ":")
	msg := "From: " + e.From + "\r\nTo: " + e.To + "\r\nSubject: " + strings.ReplaceAll(p.Subject, "\n", " ") +
		"\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n" + p.Body + "\r\n"
	return smtp.SendMail(e.Addr, smtp.PlainAuth("", e.From, e.Password, host), e.From, []string{e.To}, []byte(msg))
}

// GitHubMention comments on an issue, mentioning a login.
type GitHubMention struct {
	API   string // https://api.github.com
	Issue string // owner/name#number
	Login string
	Token string
	HTTP  *http.Client
}

// Channel implements Sink.
func (GitHubMention) Channel() string { return "github" }

// Deliver implements Sink.
func (g GitHubMention) Deliver(ctx context.Context, p Page) error {
	repo, num, ok := strings.Cut(g.Issue, "#")
	if !ok {
		return fmt.Errorf("[notify] issue %q: want owner/name#number", g.Issue)
	}
	body, _ := json.Marshal(map[string]string{"body": "@" + strings.TrimPrefix(g.Login, "@") + " **" + p.Subject + "**\n\n" + p.Body})
	return post(ctx, g.HTTP, strings.TrimSuffix(g.API, "/")+"/repos/"+repo+"/issues/"+num+"/comments",
		map[string]string{"Authorization": "Bearer " + g.Token, "Accept": "application/vnd.github+json"}, body)
}

func post(ctx context.Context, c *http.Client, url string, headers map[string]string, body []byte) error {
	if c == nil {
		c = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("notify: POST returned %s", resp.Status)
	}
	return nil
}
