// Package exports reads the analytics and billing exports `aayu measure`
// snapshots (WP7.2): a billing CSV, Azure Application Insights events, AWS
// CloudWatch RUM events, and Google Analytics 4 (Firebase) BigQuery events,
// each as a file. Exports are untrusted input (§0.1): every row is parsed
// into a fixed shape with bounded, validated fields, a row that does not
// parse is counted and dropped, and nothing in a file is ever interpreted.
package exports

import (
	"bufio"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// ErrExport means a file is unreadable or holds no row of its kind.
var ErrExport = errors.New("unusable export")

// Kinds lists the export kinds, each with the file format it reads.
var Kinds = []struct{ Name, Format string }{
	{"billing", "CSV with a header row: user_id or customer id, created, amount, currency, optional status and amount refunded"},
	{"appinsights", "Application Insights events as JSON lines (Log Analytics export: TimeGenerated, Name, UserId or UserAuthenticatedId; or classic customEvents: timestamp, name, user_Id)"},
	{"aws", "CloudWatch RUM events as JSON lines: event_timestamp (ms), event_type, user_details.userId"},
	{"gcp", "Google Analytics 4 / Firebase BigQuery export as JSON lines: event_timestamp (µs), event_name, user_id or user_pseudo_id"},
}

// Event is one product event.
type Event struct {
	User string
	Name string
	TS   time.Time
}

// Payment is one settled payment, net of refunds, in minor units.
type Payment struct {
	User        string
	TS          time.Time
	AmountMinor int64
	Currency    string
}

// File is one parsed export.
type File struct {
	Kind     string    `json:"kind"`
	Path     string    `json:"path"`
	SHA256   string    `json:"sha256"`
	Rows     int       `json:"rows"`            // rows kept
	Skipped  int       `json:"skipped"`         // rows that did not parse
	Excluded int       `json:"excluded"`        // rows that parsed but do not count (unpaid payments)
	Live     string    `json:"live,omitempty"`  // the cloud integration a live pull came from (WP7.2b)
	Query    string    `json:"query,omitempty"` // the exact request of a live pull
	Events   []Event   `json:"-"`
	Payments []Payment `json:"-"`
}

// MaxBytes bounds an export file.
const MaxBytes = 1 << 30

// Read parses the export at path as kind.
func Read(kind, path string) (*File, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %w", ErrExport, kind, err)
	}
	defer func() { _ = f.Close() }()
	if fi, err := f.Stat(); err != nil || fi.Size() > MaxBytes {
		return nil, fmt.Errorf("%w: %s %s: not a regular file of at most %d bytes", ErrExport, kind, path, int64(MaxBytes))
	}
	h := sha256.New()
	r := io.TeeReader(io.LimitReader(f, MaxBytes), h)
	out := &File{Kind: kind, Path: path}
	switch kind {
	case "billing":
		err = readBilling(r, out)
	case "appinsights", "aws", "gcp":
		err = readLines(r, out, map[string]func(map[string]any) (Event, bool){"appinsights": appInsights, "aws": aws, "gcp": gcp}[kind])
	default:
		return nil, fmt.Errorf("%w: unknown export kind %q", ErrExport, kind)
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %s %s: %w", ErrExport, kind, path, err)
	}
	if _, err := io.Copy(io.Discard, r); err != nil {
		return nil, fmt.Errorf("%w: %s %s: %w", ErrExport, kind, path, err)
	}
	out.SHA256 = hex.EncodeToString(h.Sum(nil))
	if out.Rows == 0 && out.Excluded == 0 {
		return nil, fmt.Errorf("%w: %s %s: no row of this kind (%d unparsable); want %s", ErrExport, kind, path, out.Skipped, format(kind))
	}
	return out, nil
}

func format(kind string) string {
	for _, k := range Kinds {
		if k.Name == kind {
			return k.Format
		}
	}
	return ""
}

// text accepts an identifier or event name: non-empty, at most max bytes,
// no control characters.
func text(s string, max int) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > max || strings.IndexFunc(s, unicode.IsControl) >= 0 {
		return "", false
	}
	return s, true
}

func user(s string) (string, bool)  { return text(s, 256) }
func event(s string) (string, bool) { return text(s, 200) }

var layouts = []string{time.RFC3339Nano, "2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02T15:04:05", "2006-01-02"}

// when parses a timestamp; one without a zone is UTC.
func when(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}

var amountRE = regexp.MustCompile(`^-?[0-9]{1,15}(\.[0-9]{1,2})?$`)

// minor parses a decimal amount in major units into minor units (cents).
func minor(s string) (int64, bool) {
	s = strings.ReplaceAll(strings.TrimSpace(s), ",", "")
	if s == "" {
		return 0, true
	}
	if !amountRE.MatchString(s) {
		return 0, false
	}
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	whole, frac, _ := strings.Cut(s, ".")
	frac = (frac + "00")[:2]
	w, err1 := strconv.ParseInt(whole, 10, 64)
	f, err2 := strconv.ParseInt(frac, 10, 64)
	if err1 != nil || err2 != nil {
		return 0, false
	}
	v := w*100 + f
	if neg {
		v = -v
	}
	return v, true
}

var currencyRE = regexp.MustCompile(`^[A-Za-z]{3}$`)

func readBilling(r io.Reader, out *File) error {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	cr.ReuseRecord = false
	head, err := cr.Read()
	if err != nil {
		return fmt.Errorf("no header row: %w", err)
	}
	col := map[string]int{}
	for i, h := range head {
		col[strings.ToLower(strings.TrimSpace(strings.TrimPrefix(h, "\ufeff")))] = i
	}
	find := func(names ...string) int {
		for _, n := range names {
			if i, ok := col[n]; ok {
				return i
			}
		}
		return -1
	}
	cUser := find("user_id", "customer_id", "customer id", "customer")
	cTime := find("created", "created (utc)", "created date (utc)", "created_at", "paid_at", "date")
	cAmount := find("amount")
	cCurrency := find("currency")
	cStatus := find("status")
	cRefund := find("amount refunded", "amount_refunded")
	if cUser < 0 || cTime < 0 || cAmount < 0 || cCurrency < 0 {
		return fmt.Errorf("header %q lacks a user (user_id, customer id), time (created), amount, or currency column", strings.Join(head, ","))
	}
	get := func(rec []string, i int) string {
		if i < 0 || i >= len(rec) {
			return ""
		}
		return rec[i]
	}
	for {
		rec, err := cr.Read()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			var pe *csv.ParseError
			if errors.As(err, &pe) {
				out.Skipped++
				continue
			}
			return err
		}
		u, ok1 := user(get(rec, cUser))
		ts, ok2 := when(get(rec, cTime))
		amt, ok3 := minor(get(rec, cAmount))
		ref, ok4 := minor(get(rec, cRefund))
		cur := strings.ToUpper(strings.TrimSpace(get(rec, cCurrency)))
		if !ok1 || !ok2 || !ok3 || !ok4 || !currencyRE.MatchString(cur) || get(rec, cAmount) == "" {
			out.Skipped++
			continue
		}
		if st := strings.ToLower(strings.TrimSpace(get(rec, cStatus))); cStatus >= 0 && st != "paid" && st != "succeeded" {
			out.Excluded++
			continue
		}
		out.Payments = append(out.Payments, Payment{User: u, TS: ts, AmountMinor: amt - ref, Currency: cur})
		out.Rows++
	}
}

func readLines(r io.Reader, out *File, parse func(map[string]any) (Event, bool)) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var m map[string]any
		if json.Unmarshal([]byte(line), &m) != nil {
			out.Skipped++
			continue
		}
		e, ok := parse(m)
		if !ok {
			out.Skipped++
			continue
		}
		out.Events = append(out.Events, e)
		out.Rows++
	}
	return sc.Err()
}

func str(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if s, ok := m[k].(string); ok && strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

// epoch reads an integer timestamp given as a JSON number or a string.
func epoch(v any) (int64, bool) {
	switch x := v.(type) {
	case float64:
		if x != float64(int64(x)) {
			return 0, false
		}
		return int64(x), true
	case string:
		n, err := strconv.ParseInt(x, 10, 64)
		return n, err == nil
	}
	return 0, false
}

func appInsights(m map[string]any) (Event, bool) {
	u, ok1 := user(str(m, "UserAuthenticatedId", "UserId", "user_AuthenticatedId", "user_Id"))
	n, ok2 := event(str(m, "Name", "name"))
	ts, ok3 := when(str(m, "TimeGenerated", "timestamp"))
	return Event{u, n, ts}, ok1 && ok2 && ok3
}

func aws(m map[string]any) (Event, bool) {
	d, _ := m["user_details"].(map[string]any)
	u, ok1 := user(str(d, "userId"))
	n, ok2 := event(str(m, "event_type"))
	ms, ok3 := epoch(m["event_timestamp"])
	return Event{u, n, time.UnixMilli(ms).UTC()}, ok1 && ok2 && ok3 && ms > 0
}

func gcp(m map[string]any) (Event, bool) {
	u, ok1 := user(str(m, "user_id", "user_pseudo_id"))
	n, ok2 := event(str(m, "event_name"))
	us, ok3 := epoch(m["event_timestamp"])
	return Event{u, n, time.UnixMicro(us).UTC()}, ok1 && ok2 && ok3 && us > 0
}
