// Package actions is the single path for external writes (ground rule 6,
// WP1.5). Every write is an Action passed to Log.Do, which applies --dry-run,
// idempotency keys, capability policy, and payload redaction, and appends one
// hash-chained JSON line per action to telemetry/actions.jsonl. The log is not
// the audit source of record; `audit` (WP7.6) reconciles it against GitHub's
// own event history.
package actions

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Request describes the HTTP call a GitHub write will make, so policy can be
// checked before any request is sent.
type Request struct {
	Method string `json:"method"`
	Path   string `json:"path"` // e.g. /repos/o/r/issues/12/labels
}

// Action is one external write.
type Action struct {
	Kind           string         `json:"kind"`             // e.g. github.issue.labels.add, model.call
	Target         string         `json:"target,omitempty"` // e.g. owner/repo#12
	Story          string         `json:"story,omitempty"`  // the story the action serves, for budgets
	Seat           string         `json:"seat,omitempty"`
	Host           string         `json:"host,omitempty"`
	Identity       string         `json:"identity,omitempty"` // human, bot, or human-fallback
	IdempotencyKey string         `json:"idempotency_key,omitempty"`
	Capability     string         `json:"capability,omitempty"` // e.g. labels-only
	Request        *Request       `json:"request,omitempty"`
	Payload        map[string]any `json:"payload,omitempty"`
}

// Outcome is what performing an action reports back.
type Outcome struct {
	Result    any
	TokensIn  int
	TokensOut int
	CostUSD   float64
	Attempts  int
}

// Statuses of a record.
const (
	StatusPlanned = "planned" // --dry-run: logged, not performed
	StatusDone    = "done"
	StatusFailed  = "failed"
	StatusRefused = "refused" // policy refused it before any request
)

// Record is one line of the actions log.
type Record struct {
	Seq  int       `json:"seq"`
	Time time.Time `json:"time"`
	Run  string    `json:"run"`
	Action
	Status     string  `json:"status"`
	Error      string  `json:"error,omitempty"`
	Result     any     `json:"result,omitempty"`
	TokensIn   int     `json:"tokens_in"`
	TokensOut  int     `json:"tokens_out"`
	CostUSD    float64 `json:"cost_usd"`
	WallTimeMS int64   `json:"wall_time_ms"`
	Attempts   int     `json:"attempts"`
	Prev       string  `json:"prev"`
	Hash       string  `json:"hash,omitempty"`
}

// ErrCapability means an action's capability does not allow its write.
var ErrCapability = errors.New("refused by capability")

// Options configure a log.
type Options struct {
	DryRun bool
	Run    string           // run ID, e.g. a timestamp plus random suffix
	Host   string           // default host stamped on actions
	Now    func() time.Time // default time.Now
	Redact []string         // extra payload keys to redact
	Sink   SegmentSink      // where Checkpoint appends segments; nil disables
	OnDone func(rec Record) // observer, e.g. budget accounting
}

// Log is an append-only, hash-chained JSONL actions log.
type Log struct {
	mu        sync.Mutex
	path      string
	opts      Options
	seq       int
	prev      string
	done      map[string]Record // idempotency key -> completed record
	unflushed [][]byte
}

// Open opens (creating if needed) the log at path and loads its chain head and
// completed idempotency keys. It does not verify the chain; VerifyChain does.
func Open(path string, opts Options) (*Log, error) {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Run == "" {
		opts.Run = opts.Now().UTC().Format("20060102T150405Z")
	}
	l := &Log{path: path, opts: opts, done: map[string]Record{}}
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return l, nil
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 16<<20)
	for sc.Scan() {
		var r Record
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			return nil, fmt.Errorf("%s line %d: %w", path, l.seq+1, err)
		}
		l.seq, l.prev = r.Seq, r.Hash
		if r.Status == StatusDone && r.IdempotencyKey != "" {
			l.done[r.IdempotencyKey] = r
		}
	}
	return l, sc.Err()
}

// DryRun reports whether the log only plans actions.
func (l *Log) DryRun() bool { return l.opts.DryRun }

// Do runs perform for a, unless --dry-run is set, the idempotency key already
// completed, or policy refuses it. It always logs the outcome, except for an
// idempotent repeat, which returns the earlier record.
func (l *Log) Do(ctx context.Context, a Action, perform func(context.Context) (Outcome, error)) (Record, error) {
	if a.Host == "" {
		a.Host = l.opts.Host
	}
	l.mu.Lock()
	prior, repeat := l.done[a.IdempotencyKey]
	l.mu.Unlock()
	if a.IdempotencyKey != "" && repeat {
		return prior, nil
	}

	rec := Record{Action: a}
	rec.Payload = Redact(a.Payload, l.opts.Redact)
	start := l.opts.Now()
	var err error
	switch {
	case Allowed(a) != nil:
		err = Allowed(a)
		rec.Status = StatusRefused
	case l.opts.DryRun:
		rec.Status = StatusPlanned
	default:
		var out Outcome
		out, err = perform(ctx)
		rec.Status = StatusDone
		if err != nil {
			rec.Status = StatusFailed
		}
		rec.Result = redactAny(out.Result, l.opts.Redact)
		rec.TokensIn, rec.TokensOut, rec.CostUSD, rec.Attempts = out.TokensIn, out.TokensOut, out.CostUSD, max(out.Attempts, 1)
	}
	if err != nil {
		rec.Error = redactString(err.Error())
	}
	rec.WallTimeMS = l.opts.Now().Sub(start).Milliseconds()
	rec, werr := l.append(rec)
	if werr != nil {
		return rec, errors.Join(err, fmt.Errorf("actions log: %w", werr))
	}
	if l.opts.OnDone != nil {
		l.opts.OnDone(rec)
	}
	return rec, err
}

func (l *Log) append(rec Record) (Record, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	rec.Seq = l.seq + 1
	rec.Time = l.opts.Now().UTC()
	rec.Run = l.opts.Run
	rec.Prev = l.prev
	h, err := hashRecord(rec)
	if err != nil {
		return rec, err
	}
	rec.Hash = h
	line, err := json.Marshal(rec)
	if err != nil {
		return rec, err
	}
	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return rec, err
	}
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return rec, err
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		_ = f.Close()
		return rec, err
	}
	if err := f.Close(); err != nil {
		return rec, err
	}
	l.seq, l.prev = rec.Seq, rec.Hash
	if rec.Status == StatusDone && rec.IdempotencyKey != "" {
		l.done[rec.IdempotencyKey] = rec
	}
	l.unflushed = append(l.unflushed, line)
	return rec, nil
}

// hashRecord is sha256 over the record's JSON with Hash empty; Prev links it to
// the previous line, so editing, removing, or reordering any line breaks the chain.
func hashRecord(r Record) (string, error) {
	r.Hash = ""
	b, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

// VerifyChain checks every line of a log: each hash matches its content, each
// prev matches the line before, and sequence numbers have no gaps. It returns
// the number of valid lines and an error naming the first broken line.
func VerifyChain(r io.Reader) (int, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), 16<<20)
	prev, n := "", 0
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var rec Record
		if err := json.Unmarshal(line, &rec); err != nil {
			return n, fmt.Errorf("line %d: not a record: %w", n+1, err)
		}
		want, err := hashRecord(rec)
		if err != nil {
			return n, err
		}
		switch {
		case rec.Hash != want:
			return n, fmt.Errorf("line %d (seq %d): content does not match its hash; the line was edited", n+1, rec.Seq)
		case rec.Prev != prev:
			return n, fmt.Errorf("line %d (seq %d): prev does not match the line before; a line was removed, inserted, or reordered", n+1, rec.Seq)
		case rec.Seq != n+1:
			return n, fmt.Errorf("line %d: seq %d, want %d", n+1, rec.Seq, n+1)
		}
		prev = rec.Hash
		n++
	}
	return n, sc.Err()
}

// SegmentSink receives the lines logged since the last checkpoint. The ledger
// writer (WP2.4) appends them to ledger/actions/<date>/<host>-<run>.jsonl;
// until then DirSink writes the same layout to a local directory.
type SegmentSink interface {
	Append(date, host, run string, lines [][]byte) error
}

// DirSink writes segments under Root as actions/<date>/<host>-<run>.jsonl.
type DirSink struct{ Root string }

// Append implements SegmentSink.
func (d DirSink) Append(date, host, run string, lines [][]byte) error {
	if host == "" {
		host = "cli"
	}
	path := filepath.Join(d.Root, "actions", date, host+"-"+run+".jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	for _, l := range lines {
		if _, err := f.Write(append(l, '\n')); err != nil {
			_ = f.Close()
			return err
		}
	}
	return f.Close()
}

// Checkpoint appends the lines logged since the last checkpoint to the
// segment sink (called at each build checkpoint and on ship).
func (l *Log) Checkpoint() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.opts.Sink == nil || len(l.unflushed) == 0 {
		return nil
	}
	date := l.opts.Now().UTC().Format("2006-01-02")
	if err := l.opts.Sink.Append(date, l.opts.Host, l.opts.Run, l.unflushed); err != nil {
		return err
	}
	l.unflushed = nil
	return nil
}
