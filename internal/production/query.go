package production

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
)

// Query bounds (Decision #324): every query names its window, its scope, and
// its row limit, and none of them is unbounded.
const (
	DefaultLimit = 50
	MaxLimit     = 500
	MaxWindow    = 7 * 24 * time.Hour
)

// Kinds are the query kinds, aggregates first: requests, errors, latency,
// exceptions, and timeline aggregate; logs, deploys, and trace return records.
var Kinds = []string{"requests", "errors", "latency", "exceptions", "timeline", "logs", "deploys", "trace"}

// fields a query may group by or filter on.
var fields = []string{"service", "environment", "version", "route", "status", "exception", "level", "trace"}

// Query is one bounded query over an export.
type Query struct {
	Kind   string
	By     []string
	Where  map[string]string
	Limit  int
	From   time.Time
	To     time.Time
	Bucket time.Duration // timeline buckets; default 5 minutes
}

var errQuery = errors.New("query")

// ParseQuery reads `<kind> [by f1,f2] [where k=v ...] [limit N] [bucket 5m]`.
// The window comes from the caller (from, to).
func ParseQuery(text string, from, to time.Time) (Query, error) {
	q := Query{Where: map[string]string{}, From: from.UTC(), To: to.UTC()}
	toks := strings.Fields(text)
	if len(toks) == 0 {
		return q, fmt.Errorf("%w: empty; want one of %s", errQuery, strings.Join(Kinds, ", "))
	}
	q.Kind = strings.ToLower(toks[0])
	if !slices.Contains(Kinds, q.Kind) {
		return q, fmt.Errorf("%w: kind %q: want one of %s", errQuery, toks[0], strings.Join(Kinds, ", "))
	}
	mode := ""
	for _, t := range toks[1:] {
		switch strings.ToLower(t) {
		case "by", "where", "limit", "bucket":
			mode = strings.ToLower(t)
			continue
		}
		switch mode {
		case "by":
			for _, f := range strings.Split(t, ",") {
				if f == "" {
					continue
				}
				if !slices.Contains(fields, f) {
					return q, fmt.Errorf("%w: by %q: want %s", errQuery, f, strings.Join(fields, ", "))
				}
				q.By = append(q.By, f)
			}
		case "where":
			k, v, ok := strings.Cut(t, "=")
			if !ok || !slices.Contains(fields, k) || v == "" {
				return q, fmt.Errorf("%w: where %q: want field=value with a field of %s", errQuery, t, strings.Join(fields, ", "))
			}
			q.Where[k] = v
		case "limit":
			n, err := strconv.Atoi(t)
			if err != nil || n < 1 {
				return q, fmt.Errorf("%w: limit %q: want a positive number", errQuery, t)
			}
			q.Limit = n
		case "bucket":
			d, err := time.ParseDuration(t)
			if err != nil || d < time.Minute {
				return q, fmt.Errorf("%w: bucket %q: want a duration of at least 1m", errQuery, t)
			}
			q.Bucket = d
		default:
			return q, fmt.Errorf("%w: unexpected %q: want by, where, limit, or bucket", errQuery, t)
		}
	}
	if q.Limit == 0 {
		q.Limit = DefaultLimit
	}
	return q, nil
}

// Check reports what makes the query unbounded: a missing or too wide
// window, a limit over MaxLimit, or no scope (a service).
func (q Query) Check() error {
	switch {
	case q.From.IsZero() || q.To.IsZero() || !q.From.Before(q.To):
		return fmt.Errorf("%w: the window needs from < to", errQuery)
	case q.To.Sub(q.From) > MaxWindow:
		return fmt.Errorf("%w: the window is %s; at most %s", errQuery, q.To.Sub(q.From), MaxWindow)
	case q.Limit < 1 || q.Limit > MaxLimit:
		return fmt.Errorf("%w: limit %d: want 1 to %d", errQuery, q.Limit, MaxLimit)
	case q.Where["service"] == "":
		return fmt.Errorf("%w: no scope: name the service (where service=<name>)", errQuery)
	case q.Kind == "trace" && q.Where["trace"] == "":
		return fmt.Errorf("%w: trace needs where trace=<id>", errQuery)
	}
	return nil
}

// String is the canonical text of the query with its window, scope, and limit.
func (q Query) String() string {
	var b strings.Builder
	b.WriteString(q.Kind)
	if len(q.By) > 0 {
		b.WriteString(" by " + strings.Join(q.By, ","))
	}
	if len(q.Where) > 0 {
		keys := make([]string, 0, len(q.Where))
		for k := range q.Where {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		b.WriteString(" where")
		for _, k := range keys {
			b.WriteString(" " + k + "=" + q.Where[k])
		}
	}
	if q.Kind == "timeline" {
		b.WriteString(" bucket " + q.bucket().String())
	}
	fmt.Fprintf(&b, " | window %s..%s | limit %d", q.From.Format(time.RFC3339), q.To.Format(time.RFC3339), q.Limit)
	return b.String()
}

func (q Query) bucket() time.Duration {
	if q.Bucket == 0 {
		return 5 * time.Minute
	}
	return q.Bucket
}

// Result is a query's table. Matched counts the rows before the limit.
type Result struct {
	Columns   []string
	Rows      [][]string
	Matched   int
	Truncated bool
}

// Resolver maps a stack frame's file to a repository path at the deployed
// revision; ok is false for files outside the repository.
type Resolver func(file string) (path string, ok bool)

// Run evaluates the query over the export. resolve may be nil.
func Run(e *Export, q Query, resolve Resolver) (Result, error) {
	if err := q.Check(); err != nil {
		return Result{}, err
	}
	switch q.Kind {
	case "logs":
		return q.logs(e), nil
	case "deploys":
		return q.deploys(e), nil
	case "trace":
		return q.trace(e), nil
	}
	spans := q.spans(e)
	by := q.By
	if len(by) == 0 {
		by = map[string][]string{"errors": {"version", "route"}, "exceptions": {"version"}}[q.Kind]
		if by == nil {
			by = []string{"version"}
		}
	}
	type group struct {
		key   []string
		spans []Span
	}
	var groups []*group
	index := map[string]*group{}
	add := func(key []string, s Span) {
		k := strings.Join(key, "\x00")
		g := index[k]
		if g == nil {
			g = &group{key: key}
			index[k] = g
			groups = append(groups, g)
		}
		g.spans = append(g.spans, s)
	}
	for _, s := range spans {
		if q.Kind == "errors" && !s.Error {
			continue
		}
		key := make([]string, 0, len(by)+1)
		if q.Kind == "timeline" {
			key = append(key, s.Start.Truncate(q.bucket()).Format(time.RFC3339))
		}
		for _, f := range by {
			key = append(key, spanField(s, f))
		}
		if q.Kind == "exceptions" {
			for _, x := range s.Exceptions {
				frame := TopFrame(x.Stack, resolve)
				add(append(slices.Clone(key), x.Type, clip(redact(x.Message), 160), frame), s)
			}
			continue
		}
		add(key, s)
	}
	var cols []string
	if q.Kind == "timeline" {
		cols = append(cols, "bucket")
	}
	cols = append(cols, by...)
	var rows [][]string
	for _, g := range groups {
		row := slices.Clone(g.key)
		n, errs := len(g.spans), 0
		for _, s := range g.spans {
			if s.Error {
				errs++
			}
		}
		first, last := g.spans[0].Start, g.spans[len(g.spans)-1].Start
		switch q.Kind {
		case "requests":
			p50, p95, _ := percentiles(g.spans)
			row = append(row, strconv.Itoa(n), strconv.Itoa(errs), pct(errs, n), p50, p95)
		case "errors":
			row = append(row, strconv.Itoa(n), first.Format(time.RFC3339), last.Format(time.RFC3339))
		case "latency":
			p50, p95, p99 := percentiles(g.spans)
			row = append(row, strconv.Itoa(n), p50, p95, p99)
		case "exceptions":
			row = append(row, strconv.Itoa(n), first.Format(time.RFC3339), last.Format(time.RFC3339))
		case "timeline":
			row = append(row, strconv.Itoa(n), strconv.Itoa(errs))
		}
		rows = append(rows, row)
	}
	switch q.Kind {
	case "requests":
		cols = append(cols, "requests", "errors", "error_rate", "p50_ms", "p95_ms")
	case "errors":
		cols = append(cols, "errors", "first", "last")
	case "latency":
		cols = append(cols, "requests", "p50_ms", "p95_ms", "p99_ms")
	case "exceptions":
		cols = append(cols, "type", "message", "top_frame", "count", "first", "last")
	case "timeline":
		cols = append(cols, "requests", "errors")
	}
	if q.Kind == "exceptions" || q.Kind == "errors" {
		countCol := slices.Index(cols, map[string]string{"exceptions": "count", "errors": "errors"}[q.Kind])
		sort.SliceStable(rows, func(i, j int) bool { return atoi(rows[i][countCol]) > atoi(rows[j][countCol]) })
	}
	return limit(Result{Columns: cols, Rows: rows}, q.Limit), nil
}

func atoi(s string) int { n, _ := strconv.Atoi(s); return n }

func limit(r Result, n int) Result {
	r.Matched = len(r.Rows)
	if len(r.Rows) > n {
		r.Rows, r.Truncated = r.Rows[:n], true
	}
	return r
}

func (q Query) inWindow(t time.Time) bool { return !t.Before(q.From) && t.Before(q.To) }

func (q Query) spans(e *Export) []Span {
	var out []Span
	for _, s := range e.Spans {
		if !q.inWindow(s.Start) || !q.match(func(f string) string { return spanField(s, f) }) {
			continue
		}
		out = append(out, s)
	}
	return out
}

func (q Query) match(get func(string) string) bool {
	for k, v := range q.Where {
		if k == "level" {
			continue // log records only (logs)
		}
		got := get(k)
		if k == "version" && strings.HasPrefix(got, v) {
			continue // a revision may be abbreviated
		}
		if got != v {
			return false
		}
	}
	return true
}

func spanField(s Span, f string) string {
	switch f {
	case "service":
		return s.Service
	case "environment":
		return s.Environment
	case "version":
		return s.Version
	case "route":
		return s.Route
	case "status":
		return strconv.Itoa(s.Status)
	case "exception":
		if len(s.Exceptions) > 0 {
			return s.Exceptions[0].Type
		}
		return ""
	case "trace":
		return s.TraceID
	}
	return ""
}

func percentiles(spans []Span) (p50, p95, p99 string) {
	ds := make([]float64, 0, len(spans))
	for _, s := range spans {
		ds = append(ds, float64(s.Duration().Microseconds())/1000)
	}
	sort.Float64s(ds)
	at := func(p float64) string {
		if len(ds) == 0 {
			return ""
		}
		i := int(math.Ceil(p*float64(len(ds)))) - 1
		return strconv.FormatFloat(ds[max(i, 0)], 'f', 1, 64)
	}
	return at(0.50), at(0.95), at(0.99)
}

func pct(a, b int) string {
	if b == 0 {
		return ""
	}
	return strconv.FormatFloat(100*float64(a)/float64(b), 'f', 1, 64) + "%"
}

func (q Query) logs(e *Export) Result {
	r := Result{Columns: []string{"time", "level", "msg", "version", "trace_id", "attributes"}}
	for _, l := range e.Logs {
		if !q.inWindow(l.Time) {
			continue
		}
		ok := q.match(func(f string) string {
			switch f {
			case "service":
				return l.Service
			case "version":
				return l.Version
			case "trace":
				return l.TraceID
			case "route":
				return l.Attrs["http.route"]
			case "status":
				return l.Attrs["http.response.status_code"]
			}
			return ""
		})
		if lv := q.Where["level"]; lv != "" && !strings.EqualFold(lv, l.Level) {
			ok = false
		}
		if !ok {
			continue
		}
		keys := make([]string, 0, len(l.Attrs))
		for k := range l.Attrs {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		var as []string
		for _, k := range keys {
			as = append(as, k+"="+clip(redact(l.Attrs[k]), 80))
		}
		r.Rows = append(r.Rows, []string{l.Time.Format(time.RFC3339Nano), l.Level, clip(redact(l.Msg), 200), l.Version, l.TraceID, strings.Join(as, " ")})
	}
	return limit(r, q.Limit)
}

func (q Query) deploys(e *Export) Result {
	r := Result{Columns: []string{"time", "service", "environment", "revision", "previous", "actor"}}
	var ds []Deploy
	for _, d := range e.Deploys {
		if q.inWindow(d.Time) && q.Where["service"] == d.Service && (q.Where["environment"] == "" || q.Where["environment"] == d.Environment) {
			ds = append(ds, d)
		}
	}
	// Oldest first, and a cut keeps the newest: the last row is the deploy
	// serving the end of the window, which investigate reads as deployed.
	sort.SliceStable(ds, func(i, j int) bool { return ds[i].Time.Before(ds[j].Time) })
	r.Matched = len(ds)
	if len(ds) > q.Limit {
		ds, r.Truncated = ds[len(ds)-q.Limit:], true
	}
	for _, d := range ds {
		r.Rows = append(r.Rows, []string{d.Time.Format(time.RFC3339), d.Service, d.Environment, d.Revision, d.Previous, d.Actor})
	}
	return r
}

func (q Query) trace(e *Export) Result {
	r := Result{Columns: []string{"time", "kind", "name", "version", "status", "duration_ms", "detail"}}
	id := q.Where["trace"]
	for _, s := range e.Spans {
		if s.TraceID != id || s.Service != q.Where["service"] || !q.inWindow(s.Start) {
			continue
		}
		detail := ""
		if len(s.Exceptions) > 0 {
			detail = s.Exceptions[0].Type + ": " + clip(redact(s.Exceptions[0].Message), 120)
		}
		r.Rows = append(r.Rows, []string{s.Start.Format(time.RFC3339Nano), "span", s.Name, s.Version, strconv.Itoa(s.Status),
			strconv.FormatFloat(float64(s.Duration().Microseconds())/1000, 'f', 1, 64), detail})
	}
	for _, l := range e.Logs {
		if l.TraceID == id && q.inWindow(l.Time) {
			r.Rows = append(r.Rows, []string{l.Time.Format(time.RFC3339Nano), "log", clip(redact(l.Msg), 120), l.Version, l.Attrs["http.response.status_code"], l.Attrs["duration_ms"], l.Level})
		}
	}
	return limit(r, q.Limit)
}

var email = regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`)

// redact removes secret-shaped values and email addresses from telemetry
// text before anyone, model or person, reads it.
func redact(s string) string { return email.ReplaceAllString(actions.RedactString(s), "[email]") }

func clip(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len([]rune(s)) <= n {
		return s
	}
	return string([]rune(s)[:n-1]) + "…"
}

var frameLine = regexp.MustCompile(`^\s*(\S+\.go):(\d+)`)

// Frame is one stack frame's file and line.
type Frame struct {
	File string
	Line int
}

// Frames lists the file:line frames of a Go stack trace, outermost last.
func Frames(stack string) []Frame {
	var out []Frame
	for _, l := range strings.Split(stack, "\n") {
		if m := frameLine.FindStringSubmatch(l); m != nil {
			n, _ := strconv.Atoi(m[2])
			out = append(out, Frame{File: m[1], Line: n})
		}
	}
	return out
}

// TopFrame is the innermost frame inside the repository (by resolve), as
// path:line; without a resolver, the innermost frame outside the Go runtime
// and standard library; "" when there is none.
func TopFrame(stack string, resolve Resolver) string {
	frames := Frames(stack)
	panicked := strings.Index(stack, "panic(")
	if panicked >= 0 {
		// Frames above the panic call are the recovery path (debug.Stack, the deferred recover).
		rest := stack[panicked:]
		frames = Frames(rest)
	}
	for _, f := range frames {
		if resolve != nil {
			if p, ok := resolve(f.File); ok {
				return fmt.Sprintf("%s:%d", p, f.Line)
			}
			continue
		}
		if !strings.Contains(f.File, "/runtime/") && !strings.Contains(f.File, "/src/net/") {
			return fmt.Sprintf("%s:%d", f.File, f.Line)
		}
	}
	return ""
}
