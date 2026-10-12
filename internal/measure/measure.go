// Package measure implements the analysis behind `aayu measure` (WP7.2): it
// loads exports into a SQLite snapshot (modernc.org/sqlite, pure Go), runs
// the AARRR cohort and North Star queries over it with the built-in engine
// or, optionally, the duckdb binary, and judges a Goal's or Initiative's
// measured value against its baseline and target. The queries are plain SQL
// that SQLite and DuckDB answer alike, and a Result quotes them, so anyone
// holding the snapshot can re-run them.
package measure

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	_ "modernc.org/sqlite" // registers the "sqlite" driver

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations/exports"
)

// Errors.
var (
	// ErrNoData means the snapshot cannot answer: no export holds the data
	// the metric needs, or no cohort is complete.
	ErrNoData = errors.New("no data to measure")
	// ErrEngine means the query engine failed or is missing.
	ErrEngine = errors.New("query engine")
)

const (
	day  = int64(24 * 60 * 60)
	week = 7 * day
)

// Window is the half-open interval [From, To) a snapshot covers, in UTC.
type Window struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

// Stages is [measure] with defaults applied.
type Stages struct {
	config.Measure
}

// NewStages applies the defaults to [measure].
func NewStages(m config.Measure) Stages {
	if m.ActivationDays <= 0 {
		m.ActivationDays = 7
	}
	return Stages{m}
}

// horizon is how long after acquisition a user must be observable for every
// stage to be decided: the activation window or the retention week (days 7-13).
func (s Stages) horizon() int64 { return max(int64(s.ActivationDays)*day, 2*week) }

// Snapshot builds the SQLite snapshot at path from the exports, keeping only
// rows inside the window, and returns the file's SHA-256. Tables:
//
//	events(source, user_id, name, ts, week)          ts in Unix seconds; week = (ts - from) / 7 days
//	payments(source, user_id, ts, week, amount_minor, currency)
//	sources(kind, path, sha256, rows, skipped, excluded)
//	meta(key, value)                                  window, stages, built_at
func Snapshot(ctx context.Context, path string, w Window, s Stages, files []*exports.File, builtAt time.Time) (string, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	_ = os.Remove(path)
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return "", err
	}
	defer func() { _ = db.Close() }()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback() }()
	for _, q := range []string{
		`CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`,
		`CREATE TABLE sources (kind TEXT NOT NULL, path TEXT NOT NULL, sha256 TEXT NOT NULL, rows INTEGER NOT NULL, skipped INTEGER NOT NULL, excluded INTEGER NOT NULL)`,
		`CREATE TABLE events (source TEXT NOT NULL, user_id TEXT NOT NULL, name TEXT NOT NULL, ts INTEGER NOT NULL, week INTEGER NOT NULL)`,
		`CREATE TABLE payments (source TEXT NOT NULL, user_id TEXT NOT NULL, ts INTEGER NOT NULL, week INTEGER NOT NULL, amount_minor INTEGER NOT NULL, currency TEXT NOT NULL)`,
		`CREATE INDEX events_user ON events (user_id, ts)`,
		`CREATE INDEX payments_user ON payments (user_id, ts)`,
	} {
		if _, err := tx.ExecContext(ctx, q); err != nil {
			return "", err
		}
	}
	from, to := w.From.Unix(), w.To.Unix()
	in := func(t time.Time) bool { return t.Unix() >= from && t.Unix() < to }
	currency := ""
	for _, f := range files {
		if _, err := tx.ExecContext(ctx, `INSERT INTO sources VALUES (?, ?, ?, ?, ?, ?)`, f.Kind, filepath.Base(f.Path), f.SHA256, f.Rows, f.Skipped, f.Excluded); err != nil {
			return "", err
		}
		for _, e := range f.Events {
			if !in(e.TS) {
				continue
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO events VALUES (?, ?, ?, ?, ?)`, f.Kind, e.User, e.Name, e.TS.Unix(), (e.TS.Unix()-from)/week); err != nil {
				return "", err
			}
		}
		for _, p := range f.Payments {
			if !in(p.TS) {
				continue
			}
			if currency != "" && p.Currency != currency {
				return "", fmt.Errorf("%w: payments in %s and %s; measure sums one currency", ErrNoData, currency, p.Currency)
			}
			currency = p.Currency
			if _, err := tx.ExecContext(ctx, `INSERT INTO payments VALUES (?, ?, ?, ?, ?, ?)`, f.Kind, p.User, p.TS.Unix(), (p.TS.Unix()-from)/week, p.AmountMinor, p.Currency); err != nil {
				return "", err
			}
		}
	}
	stages, _ := json.Marshal(s.Measure)
	for k, v := range map[string]string{"from": w.From.UTC().Format(time.RFC3339), "to": w.To.UTC().Format(time.RFC3339),
		"stages": string(stages), "currency": currency, "built_at": builtAt.UTC().Format(time.RFC3339)} {
		if _, err := tx.ExecContext(ctx, `INSERT INTO meta VALUES (?, ?)`, k, v); err != nil {
			return "", err
		}
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	if err := db.Close(); err != nil {
		return "", err
	}
	return fileSHA(path)
}

func fileSHA(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// lit quotes a string as an SQL literal.
func lit(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

// named is the predicate "the event is <name>", or true when name is empty
// and empty means any event, or false when the stage is not configured.
func named(name string, emptyIsAny bool) string {
	switch {
	case name != "":
		return "e.name = " + lit(name)
	case emptyIsAny:
		return "1 = 1"
	}
	return "1 = 0"
}

// CohortSQL is the AARRR query: one row per weekly acquisition cohort.
// Acquired at a user's first acquisition event in the window; activated by
// the activation event within activation_days of it; retained by activity in
// days 7-13 after it; referred by a referral event after it; paying with a
// payment after it (revenue in minor units). Bounds are half-open.
func CohortSQL(s Stages) string {
	acq := strings.Replace(named(s.Acquisition, true), "e.name", "name", 1)
	return fmt.Sprintf(`WITH acq AS (SELECT user_id, MIN(ts) AS ts, MIN(week) AS week FROM events WHERE %s GROUP BY user_id),
act AS (SELECT DISTINCT a.user_id FROM acq a JOIN events e ON e.user_id = a.user_id WHERE %s AND e.ts >= a.ts AND e.ts < a.ts + %d),
ret AS (SELECT DISTINCT a.user_id FROM acq a JOIN events e ON e.user_id = a.user_id WHERE %s AND e.ts >= a.ts + %d AND e.ts < a.ts + %d),
ref AS (SELECT DISTINCT a.user_id FROM acq a JOIN events e ON e.user_id = a.user_id WHERE %s AND e.ts >= a.ts),
rev AS (SELECT a.user_id, SUM(p.amount_minor) AS amount FROM acq a JOIN payments p ON p.user_id = a.user_id WHERE p.ts >= a.ts GROUP BY a.user_id)
SELECT a.week AS week, COUNT(*) AS acquired, COUNT(act.user_id) AS activated, COUNT(ret.user_id) AS retained, COUNT(ref.user_id) AS referred,
COUNT(rev.user_id) AS paying, COALESCE(SUM(rev.amount), 0) AS revenue_minor
FROM acq a LEFT JOIN act ON act.user_id = a.user_id LEFT JOIN ret ON ret.user_id = a.user_id LEFT JOIN ref ON ref.user_id = a.user_id LEFT JOIN rev ON rev.user_id = a.user_id
GROUP BY a.week ORDER BY a.week`,
		acq, named(s.Activation, false), int64(s.ActivationDays)*day, named(s.Retention, true), week, 2*week, named(s.Referral, false))
}

// NorthStarSQL is the North Star query: distinct users who sent the event in
// the window, or the paid total in minor units for "revenue".
func NorthStarSQL(s Stages) (string, error) {
	switch s.NorthStar {
	case "":
		return "", fmt.Errorf("%w: [measure] north_star is not set", ErrNoData)
	case "revenue":
		return `SELECT COALESCE(SUM(amount_minor), 0) AS value FROM payments`, nil
	}
	return "SELECT COUNT(DISTINCT user_id) AS value FROM events WHERE name = " + lit(s.NorthStar), nil
}

// Cohort is one weekly acquisition cohort.
type Cohort struct {
	Week         int64     `json:"week"`  // weeks after the window start
	Start        time.Time `json:"start"` // first day of the cohort week
	Acquired     int64     `json:"acquired"`
	Activated    int64     `json:"activated"`
	Retained     int64     `json:"retained"`
	Referred     int64     `json:"referred"`
	Paying       int64     `json:"paying"`
	RevenueMinor int64     `json:"revenue_minor"`
	// Complete: every user in the cohort was acquired early enough for all
	// stages to be decided inside the window.
	Complete bool `json:"complete"`
}

// Engine runs a query against a snapshot and returns its rows.
type Engine interface {
	Name() string
	Query(ctx context.Context, snapshot, query string) ([]map[string]int64, error)
}

// SQLite is the built-in engine.
type SQLite struct{}

// Name implements Engine.
func (SQLite) Name() string { return "sqlite" }

// Query implements Engine, opening the snapshot read-only.
func (SQLite) Query(ctx context.Context, snapshot, query string) ([]map[string]int64, error) {
	db, err := sql.Open("sqlite", "file:"+snapshot+"?mode=ro")
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrEngine, err)
	}
	defer func() { _ = db.Close() }()
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("%w: sqlite: %w", ErrEngine, err)
	}
	defer func() { _ = rows.Close() }()
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	var out []map[string]int64
	for rows.Next() {
		vals := make([]int64, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, fmt.Errorf("%w: sqlite: %w", ErrEngine, err)
		}
		m := map[string]int64{}
		for i, c := range cols {
			m[c] = vals[i]
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// DuckDB shells out to the optional duckdb binary, which reads the SQLite
// snapshot through its sqlite extension (autoloaded; the first use may
// download it). The child gets only PATH and HOME, never tokens.
type DuckDB struct{ Path string }

// Name implements Engine.
func (DuckDB) Name() string { return "duckdb" }

// Query implements Engine.
func (d DuckDB) Query(ctx context.Context, snapshot, query string) ([]map[string]int64, error) {
	script := "ATTACH " + lit(snapshot) + " AS snap (TYPE sqlite, READ_ONLY); USE snap; " + query + ";"
	cmd := exec.CommandContext(ctx, d.Path, "-json", "-c", script)
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME")}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%w: duckdb: %w: %s", ErrEngine, err, strings.TrimSpace(stderr.String()))
	}
	dec := json.NewDecoder(strings.NewReader(string(out)))
	dec.UseNumber()
	var raw []map[string]json.Number
	if strings.TrimSpace(string(out)) != "" {
		if err := dec.Decode(&raw); err != nil {
			return nil, fmt.Errorf("%w: duckdb output: %w", ErrEngine, err)
		}
	}
	rows := make([]map[string]int64, len(raw))
	for i, r := range raw {
		rows[i] = map[string]int64{}
		for k, v := range r {
			n, err := v.Int64()
			if err != nil {
				return nil, fmt.Errorf("%w: duckdb value %s=%s: %w", ErrEngine, k, v, err)
			}
			rows[i][k] = n
		}
	}
	return rows, nil
}

// FindDuckDB returns the duckdb engine when the binary is on PATH.
func FindDuckDB(lookPath func(string) (string, error)) (DuckDB, error) {
	p, err := lookPath("duckdb")
	if err != nil {
		return DuckDB{}, fmt.Errorf("%w: duckdb is not on PATH; it is optional (omit --engine to use the built-in SQLite)", ErrEngine)
	}
	return DuckDB{Path: p}, nil
}

// Cohorts runs CohortSQL and marks complete cohorts.
func Cohorts(ctx context.Context, eng Engine, snapshot string, w Window, s Stages) ([]Cohort, error) {
	rows, err := eng.Query(ctx, snapshot, CohortSQL(s))
	if err != nil {
		return nil, err
	}
	out := make([]Cohort, 0, len(rows))
	for _, r := range rows {
		start := w.From.Add(time.Duration(r["week"]*week) * time.Second)
		end := start.Add(time.Duration(week) * time.Second)
		out = append(out, Cohort{Week: r["week"], Start: start, Acquired: r["acquired"], Activated: r["activated"], Retained: r["retained"],
			Referred: r["referred"], Paying: r["paying"], RevenueMinor: r["revenue_minor"],
			Complete: !end.Add(time.Duration(s.horizon()) * time.Second).After(w.To)})
	}
	return out, nil
}

// NorthStar runs NorthStarSQL.
func NorthStar(ctx context.Context, eng Engine, snapshot string, s Stages) (int64, string, error) {
	q, err := NorthStarSQL(s)
	if err != nil {
		return 0, "", err
	}
	rows, err := eng.Query(ctx, snapshot, q)
	if err != nil {
		return 0, q, err
	}
	if len(rows) != 1 {
		return 0, q, fmt.Errorf("%w: north star query returned %d rows", ErrEngine, len(rows))
	}
	return rows[0]["value"], q, nil
}

// Metrics a Goal or Initiative can be measured on from the snapshot. Rates
// count complete cohorts only; acquisition counts every cohort.
var Metrics = []string{"acquisition", "activation", "retention", "referral", "revenue", "north_star"}

// Value computes a snapshot metric from the cohorts and North Star.
func Value(metric string, cohorts []Cohort, northStar int64, s Stages) (float64, string, error) {
	if !slices.Contains(Metrics, metric) {
		return 0, "", fmt.Errorf("%w: metric %q", ErrNoData, metric)
	}
	if metric == "north_star" {
		v := float64(northStar)
		if s.NorthStar == "revenue" {
			return v / 100, "North Star: paid revenue in the window", nil
		}
		return v, "North Star: distinct users who sent " + s.NorthStar + " in the window", nil
	}
	var acquired, n, used int64
	for _, c := range cohorts {
		if metric == "acquisition" {
			acquired += c.Acquired
			continue
		}
		if !c.Complete {
			continue
		}
		used++
		acquired += c.Acquired
		n += map[string]int64{"activation": c.Activated, "retention": c.Retained, "referral": c.Referred, "revenue": c.Paying}[metric]
	}
	if metric == "acquisition" {
		return float64(acquired), "users acquired in the window", nil
	}
	if acquired == 0 {
		return 0, "", fmt.Errorf("%w: no complete cohort to compute the %s rate (a cohort is complete when its last user can be observed for %d days inside the window)", ErrNoData, metric, s.horizon()/day)
	}
	return float64(n) / float64(acquired), fmt.Sprintf("%s rate over %d complete cohort(s): %d of %d acquired users", metric, used, n, acquired), nil
}
