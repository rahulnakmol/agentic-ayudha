package production

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

// SentryAPI is the read the Sentry source needs: a GET under the
// organization's API, scoped to one project (sentry.Client).
type SentryAPI interface {
	Get(ctx context.Context, path string, v url.Values) (body []byte, request string, err error)
	Project() string
}

// Provenance is where a live query's rows came from: every request it made
// and one SHA-256 over the response bodies, in request order, so a cited
// query stays checkable after the provider's data moves on (Decision #449).
type Provenance struct {
	Source   string   `json:"source"`
	Requests []string `json:"requests"`
	Digest   string   `json:"sha256"`
}

// Sentry answers investigate's bounded queries from one Sentry project, live
// and read-only (Decision #449, phase P1). Each kind returns the same columns
// the export does, so the note, the derived numbers, and TopFrame read a
// Sentry result the way they read an export's. The query's service names the
// project the source is configured for; every request is scoped to that
// project's ID. timeline is not answered: ask requests by version instead.
type Sentry struct{ API SentryAPI }

// sentryPage is the most rows one Sentry events request returns.
const sentryPage = 100

// sentryTopFrames is how many of the most frequent issues get a top frame,
// each read from the issue's latest event: one sample, never a count.
const sentryTopFrames = 3

// sentryFields map the query's fields to Sentry's per dataset; a field
// missing from a dataset cannot be grouped or filtered on there.
var sentryFields = map[string]map[string]string{
	"spans":  {"version": "release", "route": "transaction", "environment": "environment", "status": "span.status_code", "trace": "trace"},
	"errors": {"version": "release", "route": "transaction", "environment": "environment", "exception": "error.type", "trace": "trace"},
	"logs":   {"version": "release", "trace": "trace"},
}

var errSource = errors.New("live source")

// Unanswerable reports whether err is a query a source cannot answer as
// asked, or one that is not bounded: the asker's to fix, not the provider's.
func Unanswerable(err error) bool { return errors.Is(err, errSource) || errors.Is(err, errQuery) }

// Run evaluates the query against the Sentry project.
func (s Sentry) Run(ctx context.Context, q Query, resolve Resolver) (Result, Provenance, error) {
	pv := Provenance{Source: "sentry project " + s.API.Project()}
	if err := q.Check(); err != nil {
		return Result{}, pv, err
	}
	var bodies [][]byte
	get := func(path string, v url.Values) ([]byte, error) {
		b, req, err := s.API.Get(ctx, path, v)
		pv.Requests = append(pv.Requests, req)
		if err != nil {
			// A failed read keeps its place with no answer, so every body
			// stays paired with the request that fetched it.
			bodies = append(bodies, nil)
			return nil, err
		}
		bodies = append(bodies, b)
		return b, nil
	}
	var r Result
	var err error
	switch q.Kind {
	case "requests", "latency", "errors", "exceptions":
		r, err = s.aggregate(q, resolve, get)
	case "logs":
		r, err = s.logs(q, get)
	case "deploys":
		r, err = s.deploys(q, get)
	case "trace":
		r, err = s.trace(q, get)
	default:
		err = fmt.Errorf("%w: %s is not answered from Sentry; ask requests by version (counts per release) instead", errSource, q.Kind)
	}
	h := sha256.New()
	for i, b := range bodies {
		fmt.Fprintf(h, "%s\n", pv.Requests[i])
		h.Write(b)
		h.Write([]byte("\n"))
	}
	pv.Digest = hex.EncodeToString(h.Sum(nil))
	return r, pv, err
}

// events builds an events request for a dataset: the project, the window, the
// fields, the filters, and one more row than the limit, so a cut is visible.
func (s Sentry) events(q Query, dataset string, fields []string, sortBy string, extra ...string) (url.Values, error) {
	v := url.Values{
		"dataset":  {dataset},
		"project":  {s.API.Project()},
		"start":    {q.From.UTC().Format(time.RFC3339)},
		"end":      {q.To.UTC().Format(time.RFC3339)},
		"per_page": {strconv.Itoa(min(q.Limit+1, sentryPage))},
		"field":    fields,
	}
	if sortBy != "" {
		v.Set("sort", sortBy)
	}
	filters := slices.Clone(extra)
	keys := make([]string, 0, len(q.Where))
	for k := range q.Where {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		val := q.Where[k]
		switch k {
		case "service":
			continue // the project
		case "environment":
			v.Set("environment", val)
			continue
		case "level":
			if dataset == "logs" {
				filters = append(filters, "severity:"+quote(strings.ToLower(val)))
			}
			continue
		}
		f, ok := sentryFields[dataset][k]
		if !ok {
			return nil, fmt.Errorf("%w: where %s is not a field of Sentry's %s dataset", errSource, k, dataset)
		}
		if k == "version" {
			val += "*" // a revision may be abbreviated, as in the export
		}
		filters = append(filters, f+":"+quote(val))
	}
	if len(filters) > 0 {
		v.Set("query", strings.Join(filters, " "))
	}
	return v, nil
}

// quote quotes a filter value that holds a space or a quote.
func quote(s string) string {
	if strings.ContainsAny(s, " \"") {
		return strconv.Quote(s)
	}
	return s
}

type eventsBody struct {
	Data []map[string]any `json:"data"`
}

// aggregate answers requests, latency, and errors from the spans or errors
// dataset, and exceptions from the errors dataset with each top issue's
// latest event as the sample its top frame comes from.
func (s Sentry) aggregate(q Query, resolve Resolver, get func(string, url.Values) ([]byte, error)) (Result, error) {
	by := q.By
	if len(by) == 0 {
		by = map[string][]string{"errors": {"version", "route"}, "exceptions": {"version"}}[q.Kind]
		if by == nil {
			by = []string{"version"}
		}
	}
	dataset := "spans"
	if q.Kind == "errors" || q.Kind == "exceptions" {
		dataset = "errors"
	}
	var fields []string
	for _, f := range by {
		sf, ok := sentryFields[dataset][f]
		if !ok {
			return Result{}, fmt.Errorf("%w: by %s is not a field of Sentry's %s dataset", errSource, f, dataset)
		}
		fields = append(fields, sf)
	}
	group := len(fields)
	var funcs, extra []string
	switch q.Kind {
	case "requests":
		funcs, extra = []string{"count()", "failure_count()", "p50(span.duration)", "p95(span.duration)"}, []string{"is_transaction:true"}
	case "latency":
		funcs, extra = []string{"count()", "p50(span.duration)", "p95(span.duration)", "p99(span.duration)"}, []string{"is_transaction:true"}
	case "errors":
		funcs = []string{"count()", "min(timestamp)", "max(timestamp)"}
	case "exceptions":
		fields = append(fields, "error.type", "title", "issue.id")
		funcs = []string{"count()", "min(timestamp)", "max(timestamp)"}
	}
	v, err := s.events(q, dataset, append(slices.Clone(fields), funcs...), "-count()", extra...)
	if err != nil {
		return Result{}, err
	}
	b, err := get("/events/", v)
	if err != nil {
		return Result{}, err
	}
	var body eventsBody
	if err := json.Unmarshal(b, &body); err != nil {
		return Result{}, fmt.Errorf("%w: undecodable events answer: %w", errSource, err)
	}
	frames := map[string]string{}
	cols := slices.Clone(by)
	var rows [][]string
	for _, d := range body.Data {
		row := make([]string, 0, len(cols)+6)
		for _, f := range fields[:group] {
			row = append(row, clip(redact(cell(d[f])), 200))
		}
		switch q.Kind {
		case "requests":
			n, errs := int(num(d["count()"])), int(num(d["failure_count()"]))
			row = append(row, strconv.Itoa(n), strconv.Itoa(errs), pct(errs, n), ms(d["p50(span.duration)"]), ms(d["p95(span.duration)"]))
		case "latency":
			row = append(row, cell(d["count()"]), ms(d["p50(span.duration)"]), ms(d["p95(span.duration)"]), ms(d["p99(span.duration)"]))
		case "errors":
			row = append(row, cell(d["count()"]), when(d["min(timestamp)"]), when(d["max(timestamp)"]))
		case "exceptions":
			issue := cell(d["issue.id"])
			if _, seen := frames[issue]; !seen && len(frames) < sentryTopFrames && issue != "" {
				frames[issue] = s.topFrame(issue, resolve, get)
			}
			row = append(row, clip(redact(cell(d["error.type"])), 120), clip(redact(cell(d["title"])), 160), frames[issue],
				cell(d["count()"]), when(d["min(timestamp)"]), when(d["max(timestamp)"]))
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
	}
	return limit(Result{Columns: cols, Rows: rows}, q.Limit), nil
}

// topFrame reads an issue's latest event and returns its innermost frame
// inside the repository (by resolve), else its innermost in-app frame, as
// path:line. A failed read leaves the frame empty: the count still stands.
func (s Sentry) topFrame(issue string, resolve Resolver, get func(string, url.Values) ([]byte, error)) string {
	b, err := get("/issues/"+url.PathEscape(issue)+"/events/latest/", nil)
	if err != nil {
		return ""
	}
	var ev struct {
		Entries []struct {
			Type string `json:"type"`
			Data struct {
				Values []struct {
					Stacktrace *struct {
						Frames []struct {
							Filename string `json:"filename"`
							LineNo   int    `json:"lineNo"`
							InApp    bool   `json:"inApp"`
						} `json:"frames"`
					} `json:"stacktrace"`
				} `json:"values"`
			} `json:"data"`
		} `json:"entries"`
	}
	if json.Unmarshal(b, &ev) != nil {
		return ""
	}
	inApp := ""
	for _, e := range ev.Entries {
		if e.Type != "exception" {
			continue
		}
		for _, x := range e.Data.Values {
			if x.Stacktrace == nil {
				continue
			}
			fs := x.Stacktrace.Frames
			for i := len(fs) - 1; i >= 0; i-- { // Sentry lists the innermost frame last
				f := fs[i]
				if resolve != nil {
					if p, ok := resolve(f.Filename); ok {
						return fmt.Sprintf("%s:%d", p, f.LineNo)
					}
				}
				if inApp == "" && f.InApp && f.Filename != "" {
					inApp = fmt.Sprintf("%s:%d", f.Filename, f.LineNo)
				}
			}
		}
	}
	return inApp
}

func (s Sentry) logs(q Query, get func(string, url.Values) ([]byte, error)) (Result, error) {
	v, err := s.events(q, "logs", []string{"timestamp", "severity", "message", "release", "trace"}, "-timestamp")
	if err != nil {
		return Result{}, err
	}
	b, err := get("/events/", v)
	if err != nil {
		return Result{}, err
	}
	var body eventsBody
	if err := json.Unmarshal(b, &body); err != nil {
		return Result{}, fmt.Errorf("%w: undecodable logs answer: %w", errSource, err)
	}
	r := Result{Columns: []string{"time", "level", "msg", "version", "trace_id", "attributes"}}
	for _, d := range body.Data {
		r.Rows = append(r.Rows, []string{whenNano(d["timestamp"]), cell(d["severity"]), clip(redact(cell(d["message"])), 200), cell(d["release"]), cell(d["trace"]), ""})
	}
	return limit(r, q.Limit), nil
}

// sentryReleases is how many of the project's latest releases deploys reads.
const sentryReleases = 10

// deploys reads the project's latest releases and each one's deploys: a
// release is a build, a deploy says where and when it went live. A release
// with no deploy record yields no row.
func (s Sentry) deploys(q Query, get func(string, url.Values) ([]byte, error)) (Result, error) {
	b, err := get("/releases/", url.Values{"project": {s.API.Project()}, "per_page": {strconv.Itoa(sentryReleases)}})
	if err != nil {
		return Result{}, err
	}
	var releases []struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(b, &releases); err != nil {
		return Result{}, fmt.Errorf("%w: undecodable releases answer: %w", errSource, err)
	}
	var all []Deploy
	for _, rel := range releases {
		b, err := get("/releases/"+url.PathEscape(rel.Version)+"/deploys/", nil)
		if err != nil {
			return Result{}, err
		}
		var ds []struct {
			Environment  string `json:"environment"`
			DateFinished string `json:"dateFinished"`
		}
		if err := json.Unmarshal(b, &ds); err != nil {
			return Result{}, fmt.Errorf("%w: undecodable deploys answer for %s: %w", errSource, rel.Version, err)
		}
		for _, d := range ds {
			t, err := time.Parse(time.RFC3339, d.DateFinished)
			if err != nil {
				continue // a deploy with no finish time did not go live
			}
			all = append(all, Deploy{Time: t.UTC(), Service: q.Where["service"], Environment: d.Environment, Revision: rel.Version})
		}
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].Time.Before(all[j].Time) })
	prev := map[string]string{}
	for i := range all {
		all[i].Previous = prev[all[i].Environment]
		prev[all[i].Environment] = all[i].Revision
	}
	return q.deploys(&Export{Deploys: all}), nil
}

func (s Sentry) trace(q Query, get func(string, url.Values) ([]byte, error)) (Result, error) {
	b, err := get("/trace/"+url.PathEscape(q.Where["trace"])+"/", url.Values{
		"start": {q.From.UTC().Format(time.RFC3339)}, "end": {q.To.UTC().Format(time.RFC3339)}})
	if err != nil {
		return Result{}, err
	}
	type span struct {
		Op          string  `json:"op"`
		Description string  `json:"description"`
		Start       float64 `json:"start_timestamp"`
		Duration    float64 `json:"duration"`
		Errors      []struct {
			Title string `json:"title"`
		} `json:"errors"`
		Children []json.RawMessage `json:"children"`
	}
	var roots []json.RawMessage
	if err := json.Unmarshal(b, &roots); err != nil {
		return Result{}, fmt.Errorf("%w: undecodable trace answer: %w", errSource, err)
	}
	r := Result{Columns: []string{"time", "kind", "name", "version", "status", "duration_ms", "detail"}}
	var walk func([]json.RawMessage, int)
	walk = func(nodes []json.RawMessage, depth int) {
		if depth > 32 {
			return
		}
		for _, raw := range nodes {
			var sp span
			if json.Unmarshal(raw, &sp) != nil {
				continue
			}
			detail := ""
			if len(sp.Errors) > 0 {
				detail = clip(redact(sp.Errors[0].Title), 120)
			}
			start := time.UnixMicro(int64(math.Round(sp.Start * 1e6))).UTC() // Sentry sends seconds as a float
			r.Rows = append(r.Rows, []string{start.Format(time.RFC3339Nano), "span", clip(redact(strings.TrimSpace(sp.Op+" "+sp.Description)), 120), "", "",
				strconv.FormatFloat(sp.Duration, 'f', 1, 64), detail})
			walk(sp.Children, depth+1)
		}
	}
	walk(roots, 0)
	return limit(r, q.Limit), nil
}

// cell is a value from a provider's answer as text.
func cell(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	}
	return fmt.Sprint(v)
}

func num(v any) float64 { f, _ := v.(float64); return f }

// ms formats a duration in milliseconds as the export does.
func ms(v any) string {
	f, ok := v.(float64)
	if !ok {
		return ""
	}
	return strconv.FormatFloat(f, 'f', 1, 64)
}

// when formats a provider timestamp as RFC 3339 in UTC, as the export does.
func when(v any) string { return stamp(v, time.RFC3339) }

func whenNano(v any) string { return stamp(v, time.RFC3339Nano) }

func stamp(v any, layout string) string {
	s := cell(v)
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t.UTC().Format(layout)
	}
	return s
}

// MeasureSentry measures a crash-free SLO from the Sentry project's release
// health over the whole SLO window ending at to (Decision #449, phase P1).
// An SLO window is longer than investigate's query bound, so this reads
// aggregates only: one request for the hourly session counts by status, from
// which the window's totals and the last hour's and six hours' burn come.
// A good event is a session that did not crash.
func MeasureSentry(ctx context.Context, api SentryAPI, o SLO, to time.Time) (Measured, Provenance, error) {
	pv := Provenance{Source: "sentry project " + api.Project() + " sessions"}
	if !o.FromSentry() {
		return Measured{}, pv, fmt.Errorf("%w: SLO %q (sli %s) is measured from an export, not from Sentry", errSource, o.Name, o.SLI)
	}
	from := to.Add(-o.window)
	v := url.Values{
		"field":    {"sum(session)"},
		"groupBy":  {"session.status"},
		"project":  {api.Project()},
		"start":    {from.UTC().Format(time.RFC3339)},
		"end":      {to.UTC().Format(time.RFC3339)},
		"interval": {"1h"},
	}
	b, req, err := api.Get(ctx, "/sessions/", v)
	pv.Requests = append(pv.Requests, req)
	if err != nil {
		return Measured{}, pv, err
	}
	sum := sha256.Sum256([]byte(req + "\n" + string(b) + "\n"))
	pv.Digest = hex.EncodeToString(sum[:])
	var body struct {
		Groups []struct {
			By     map[string]any        `json:"by"`
			Totals map[string]*float64   `json:"totals"`
			Series map[string][]*float64 `json:"series"`
		} `json:"groups"`
	}
	if err := json.Unmarshal(b, &body); err != nil {
		return Measured{}, pv, fmt.Errorf("%w: undecodable sessions answer: %w", errSource, err)
	}
	var total, crashed, h1, h1bad, h6, h6bad int
	for _, g := range body.Groups {
		n := g.Totals["sum(session)"]
		if n == nil {
			continue
		}
		bad := g.By["session.status"] == "crashed"
		total += int(*n)
		if bad {
			crashed += int(*n)
		}
		series := g.Series["sum(session)"]
		for i := range series {
			back := len(series) - i // 1 is the last hour
			if series[i] == nil || back > 6 {
				continue
			}
			c := int(*series[i])
			h6 += c
			if bad {
				h6bad += c
			}
			if back == 1 {
				h1 += c
				if bad {
					h1bad += c
				}
			}
		}
	}
	m := measured(o, from, to, total-crashed, total, h1bad, h1, h6bad, h6)
	m.Source, m.Digest = pv.Source, pv.Digest
	return m, pv, nil
}
