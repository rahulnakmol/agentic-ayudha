// Package production implements the Production Engineer's checks (WP5.10,
// Decision #324): reading a sanitised OpenTelemetry export, bounded queries
// over it, the investigation note, the postmortem, SLOs, and runbooks. It
// never calls a model and never changes anything outside the process: every
// mitigation is a catalog action proposed through `aayu respond`.
package production

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"
)

// Export file names inside an export directory. Traces are OTLP JSON (one
// ExportTraceServiceRequest per line, the OpenTelemetry Collector file
// exporter's format); logs are JSON lines with trace_id and span_id (slog's
// JSON handler, or any logger that follows the OpenTelemetry log correlation
// fields); deploy markers are JSON lines written by the deploy step.
const (
	TracesFile  = "traces.jsonl"
	LogsFile    = "logs.jsonl"
	DeploysFile = "deploys.jsonl"
)

// MaxExportBytes bounds each file read from an export.
const MaxExportBytes = 64 << 20

// Span is one server span with the attributes investigations group by.
type Span struct {
	TraceID     string
	SpanID      string
	Name        string
	Service     string
	Version     string
	Environment string
	Route       string
	Status      int // http.response.status_code; 0 when absent
	Error       bool
	Start, End  time.Time
	Exceptions  []Exception
}

// Duration is the span's duration.
func (s Span) Duration() time.Duration { return s.End.Sub(s.Start) }

// Exception is an OpenTelemetry exception event on a span.
type Exception struct {
	Type, Message, Stack string
}

// Log is one structured log record.
type Log struct {
	Time    time.Time
	Level   string
	Msg     string
	TraceID string
	Service string
	Version string
	Attrs   map[string]string
}

// Deploy is a deploy marker: which revision of which service went live where.
type Deploy struct {
	Time        time.Time `json:"time"`
	Service     string    `json:"service"`
	Environment string    `json:"environment"`
	Revision    string    `json:"revision"`
	Previous    string    `json:"previous,omitempty"`
	Actor       string    `json:"actor,omitempty"`
}

// Export is a loaded telemetry export.
type Export struct {
	Dir     string
	Spans   []Span
	Logs    []Log
	Deploys []Deploy
	Files   []string // the files read, in name order
}

// Span returns the first and last instant any record in the export covers.
func (e *Export) Span() (from, to time.Time) {
	see := func(t time.Time) {
		if t.IsZero() {
			return
		}
		if from.IsZero() || t.Before(from) {
			from = t
		}
		if t.After(to) {
			to = t
		}
	}
	for _, s := range e.Spans {
		see(s.Start)
		see(s.End)
	}
	for _, l := range e.Logs {
		see(l.Time)
	}
	for _, d := range e.Deploys {
		see(d.Time)
	}
	return from, to
}

// ErrNoExport means the directory holds none of the export files.
var ErrNoExport = errors.New("no telemetry export")

// LoadExport reads an export directory. Files it does not know are ignored;
// a malformed line fails the load with its file and line number.
func LoadExport(dir string) (*Export, error) {
	e := &Export{Dir: dir}
	for _, f := range []struct {
		name string
		read func([]byte) error
	}{{TracesFile, e.readTrace}, {LogsFile, e.readLog}, {DeploysFile, e.readDeploy}} {
		fh, err := os.Open(filepath.Join(dir, f.name))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		err = eachLine(io.LimitReader(fh, MaxExportBytes), func(n int, line []byte) error {
			if err := f.read(line); err != nil {
				return fmt.Errorf("%s:%d: %w", f.name, n, err)
			}
			return nil
		})
		_ = fh.Close()
		if err != nil {
			return nil, err
		}
		e.Files = append(e.Files, f.name)
	}
	if len(e.Files) == 0 {
		return nil, fmt.Errorf("%w in %s: want %s, %s, or %s", ErrNoExport, dir, TracesFile, LogsFile, DeploysFile)
	}
	sort.SliceStable(e.Spans, func(i, j int) bool { return e.Spans[i].Start.Before(e.Spans[j].Start) })
	sort.SliceStable(e.Logs, func(i, j int) bool { return e.Logs[i].Time.Before(e.Logs[j].Time) })
	sort.SliceStable(e.Deploys, func(i, j int) bool { return e.Deploys[i].Time.Before(e.Deploys[j].Time) })
	return e, nil
}

func eachLine(r io.Reader, fn func(int, []byte) error) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64<<10), 8<<20)
	n := 0
	for sc.Scan() {
		n++
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		if err := fn(n, line); err != nil {
			return err
		}
	}
	return sc.Err()
}

type otlpKV struct {
	Key   string `json:"key"`
	Value struct {
		String *string          `json:"stringValue"`
		Int    *json.RawMessage `json:"intValue"`
		Double *float64         `json:"doubleValue"`
		Bool   *bool            `json:"boolValue"`
	} `json:"value"`
}

func kvs(in []otlpKV) map[string]string {
	out := map[string]string{}
	for _, a := range in {
		v := a.Value
		switch {
		case v.String != nil:
			out[a.Key] = *v.String
		case v.Int != nil:
			s := string(*v.Int)
			if u, err := strconv.Unquote(s); err == nil {
				s = u
			}
			out[a.Key] = s
		case v.Double != nil:
			out[a.Key] = strconv.FormatFloat(*v.Double, 'f', -1, 64)
		case v.Bool != nil:
			out[a.Key] = strconv.FormatBool(*v.Bool)
		}
	}
	return out
}

func nanos(s string) time.Time {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n == 0 {
		return time.Time{}
	}
	return time.Unix(0, n).UTC()
}

func (e *Export) readTrace(line []byte) error {
	var req struct {
		ResourceSpans []struct {
			Resource struct {
				Attributes []otlpKV `json:"attributes"`
			} `json:"resource"`
			ScopeSpans []struct {
				Spans []struct {
					TraceID    string   `json:"traceId"`
					SpanID     string   `json:"spanId"`
					Parent     string   `json:"parentSpanId"`
					Name       string   `json:"name"`
					Start      string   `json:"startTimeUnixNano"`
					End        string   `json:"endTimeUnixNano"`
					Attributes []otlpKV `json:"attributes"`
					Status     struct {
						Code int `json:"code"`
					} `json:"status"`
					Events []struct {
						Name       string   `json:"name"`
						Attributes []otlpKV `json:"attributes"`
					} `json:"events"`
				} `json:"spans"`
			} `json:"scopeSpans"`
		} `json:"resourceSpans"`
	}
	if err := json.Unmarshal(line, &req); err != nil {
		return err
	}
	for _, rs := range req.ResourceSpans {
		res := kvs(rs.Resource.Attributes)
		env := res["deployment.environment.name"]
		if env == "" {
			env = res["deployment.environment"]
		}
		for _, ss := range rs.ScopeSpans {
			for _, sp := range ss.Spans {
				if sp.Parent != "" {
					continue // investigations read server spans; child spans are reached through their trace
				}
				a := kvs(sp.Attributes)
				status, _ := strconv.Atoi(a["http.response.status_code"])
				s := Span{TraceID: sp.TraceID, SpanID: sp.SpanID, Name: sp.Name, Service: res["service.name"], Version: res["service.version"],
					Environment: env, Route: a["http.route"], Status: status, Error: sp.Status.Code == 2 || status >= 500,
					Start: nanos(sp.Start), End: nanos(sp.End)}
				for _, ev := range sp.Events {
					if ev.Name != "exception" {
						continue
					}
					ea := kvs(ev.Attributes)
					s.Exceptions = append(s.Exceptions, Exception{Type: ea["exception.type"], Message: ea["exception.message"], Stack: ea["exception.stacktrace"]})
				}
				e.Spans = append(e.Spans, s)
			}
		}
	}
	return nil
}

func (e *Export) readLog(line []byte) error {
	var m map[string]any
	if err := json.Unmarshal(line, &m); err != nil {
		return err
	}
	l := Log{Attrs: map[string]string{}}
	for k, v := range m {
		s := fmt.Sprint(v)
		if f, ok := v.(float64); ok {
			s = strconv.FormatFloat(f, 'f', -1, 64)
		}
		switch k {
		case "time", "timestamp":
			t, err := time.Parse(time.RFC3339Nano, s)
			if err != nil {
				return fmt.Errorf("%s: %w", k, err)
			}
			l.Time = t.UTC()
		case "level", "severity":
			l.Level = s
		case "msg", "message", "body":
			l.Msg = s
		case "trace_id":
			l.TraceID = s
		case "service.name":
			l.Service = s
		case "service.version":
			l.Version = s
		default:
			l.Attrs[k] = s
		}
	}
	e.Logs = append(e.Logs, l)
	return nil
}

func (e *Export) readDeploy(line []byte) error {
	var d Deploy
	dec := json.NewDecoder(bytes.NewReader(line))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d); err != nil {
		return err
	}
	if d.Revision == "" || d.Time.IsZero() {
		return errors.New("a deploy marker needs time and revision")
	}
	d.Time = d.Time.UTC()
	e.Deploys = append(e.Deploys, d)
	return nil
}

// Deployed returns the latest deploy marker at or before t for the service
// and environment (either may be empty to match any), and whether one exists.
func (e *Export) Deployed(service, environment string, t time.Time) (Deploy, bool) {
	var out Deploy
	ok := false
	for _, d := range e.Deploys {
		if (service == "" || d.Service == service) && (environment == "" || d.Environment == environment) && !d.Time.After(t) {
			out, ok = d, true
		}
	}
	return out, ok
}
