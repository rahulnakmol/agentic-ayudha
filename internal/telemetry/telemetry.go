// Package telemetry records OpenTelemetry traces of verb runs (WP1.8). Traces go
// to a local file by default. Sending them anywhere else is opt-in: only an
// explicit [telemetry] otlp_endpoint enables the OTLP/HTTP exporter; the
// standard OTEL_* environment variables are deliberately ignored so no remote
// telemetry starts by accident.
//
// Both exporters write OTLP JSON (one ExportTraceServiceRequest per line in the
// file), implemented here so the binary does not pull in gRPC and protobuf.
package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// Options configure telemetry for one process.
type Options struct {
	Disabled     bool   // [telemetry] disabled: no traces at all
	Path         string // trace file; "" disables the file exporter
	OTLPEndpoint string // [telemetry] otlp_endpoint, e.g. https://collector.example:4318; "" (default) sends nothing
	HTTP         *http.Client
	Version      string // service.version
}

// Telemetry owns the tracer provider.
type Telemetry struct {
	provider *sdktrace.TracerProvider
	tracer   trace.Tracer
}

// Start builds the provider. With nothing to export it returns a no-op tracer.
func Start(o Options) *Telemetry {
	var exporters []sdktrace.SpanExporter
	if !o.Disabled && o.Path != "" {
		exporters = append(exporters, NewFileExporter(o.Path, aayudhaResource(o.Version)...))
	}
	if !o.Disabled && o.OTLPEndpoint != "" {
		hc := o.HTTP
		if hc == nil {
			hc = &http.Client{Timeout: 10 * time.Second}
		}
		exporters = append(exporters, &otlpExporter{url: strings.TrimSuffix(o.OTLPEndpoint, "/") + "/v1/traces", http: hc, resource: aayudhaResource(o.Version)})
	}
	if len(exporters) == 0 {
		return &Telemetry{tracer: noop.NewTracerProvider().Tracer("aayudha")}
	}
	var opts []sdktrace.TracerProviderOption
	for _, e := range exporters {
		opts = append(opts, sdktrace.WithSyncer(e)) // a CLI run is short; export each span as it ends
	}
	p := sdktrace.NewTracerProvider(opts...)
	return &Telemetry{provider: p, tracer: p.Tracer("aayudha")}
}

// Tracer returns the tracer.
func (t *Telemetry) Tracer() trace.Tracer { return t.tracer }

// Shutdown flushes and stops the exporters.
func (t *Telemetry) Shutdown(ctx context.Context) error {
	if t.provider == nil {
		return nil
	}
	return t.provider.Shutdown(ctx)
}

// End finishes a verb-run span with its exit code.
func End(span trace.Span, exitCode int, err error) {
	span.SetAttributes(attribute.Int("aayu.exit_code", exitCode))
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}
	span.End()
}

func aayudhaResource(version string) []attribute.KeyValue {
	return []attribute.KeyValue{attribute.String("service.name", "aayudha"), attribute.String("service.version", version)}
}

// NewFileExporter writes spans as OTLP JSON, one ExportTraceServiceRequest per
// line, the format of the OpenTelemetry Collector's file exporter, with the
// given resource attributes (service.name, service.version, …). A product can
// use it to keep a sanitised trace export that `aayu investigate` reads.
func NewFileExporter(path string, resource ...attribute.KeyValue) sdktrace.SpanExporter {
	return &fileExporter{path: path, resource: resource}
}

type fileExporter struct {
	mu       sync.Mutex
	path     string
	resource []attribute.KeyValue
}

func (f *fileExporter) ExportSpans(_ context.Context, spans []sdktrace.ReadOnlySpan) error {
	line, err := json.Marshal(request(spans, f.resource))
	if err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(f.path), 0o755); err != nil {
		return err
	}
	fh, err := os.OpenFile(f.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := fh.Write(append(line, '\n')); err != nil {
		_ = fh.Close()
		return err
	}
	return fh.Close()
}

func (*fileExporter) Shutdown(context.Context) error { return nil }

type otlpExporter struct {
	url      string
	http     *http.Client
	resource []attribute.KeyValue
}

func (o *otlpExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	body, err := json.Marshal(request(spans, o.resource))
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := o.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("OTLP export: %s", resp.Status)
	}
	return nil
}

func (*otlpExporter) Shutdown(context.Context) error { return nil }

// OTLP JSON (opentelemetry-proto, JSON encoding: hex IDs, string nanos).
type kv struct {
	Key   string         `json:"key"`
	Value map[string]any `json:"value"`
}

func attrs(as []attribute.KeyValue) []kv {
	out := make([]kv, 0, len(as))
	for _, a := range as {
		var v map[string]any
		switch a.Value.Type() {
		case attribute.BOOL:
			v = map[string]any{"boolValue": a.Value.AsBool()}
		case attribute.INT64:
			v = map[string]any{"intValue": strconv.FormatInt(a.Value.AsInt64(), 10)}
		case attribute.FLOAT64:
			v = map[string]any{"doubleValue": a.Value.AsFloat64()}
		default:
			v = map[string]any{"stringValue": a.Value.String()}
		}
		out = append(out, kv{string(a.Key), v})
	}
	return out
}

func request(spans []sdktrace.ReadOnlySpan, resource []attribute.KeyValue) map[string]any {
	out := make([]map[string]any, 0, len(spans))
	for _, s := range spans {
		sc := s.SpanContext()
		m := map[string]any{
			"traceId":           sc.TraceID().String(),
			"spanId":            sc.SpanID().String(),
			"name":              s.Name(),
			"kind":              int(s.SpanKind()),
			"startTimeUnixNano": strconv.FormatInt(s.StartTime().UnixNano(), 10),
			"endTimeUnixNano":   strconv.FormatInt(s.EndTime().UnixNano(), 10),
			"attributes":        attrs(s.Attributes()),
			"status":            map[string]any{"code": statusCode(s.Status().Code), "message": s.Status().Description},
		}
		if p := s.Parent(); p.IsValid() {
			m["parentSpanId"] = p.SpanID().String()
		}
		if evs := s.Events(); len(evs) > 0 {
			es := make([]map[string]any, 0, len(evs))
			for _, ev := range evs {
				es = append(es, map[string]any{"name": ev.Name, "timeUnixNano": strconv.FormatInt(ev.Time.UnixNano(), 10), "attributes": attrs(ev.Attributes)})
			}
			m["events"] = es
		}
		out = append(out, m)
	}
	return map[string]any{"resourceSpans": []any{map[string]any{
		"resource":   map[string]any{"attributes": attrs(resource)},
		"scopeSpans": []any{map[string]any{"scope": map[string]any{"name": scopeName(resource)}, "spans": out}},
	}}}
}

// scopeName is the service name, the instrumentation scope of its own spans.
func scopeName(resource []attribute.KeyValue) string {
	for _, a := range resource {
		if a.Key == "service.name" {
			return a.Value.AsString()
		}
	}
	return "aayudha"
}

func statusCode(c codes.Code) int {
	switch c {
	case codes.Ok:
		return 1
	case codes.Error:
		return 2
	}
	return 0
}

// ReadSpanNames returns the span names in a trace file (tests and doctor).
func ReadSpanNames(path string) ([]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, line := range bytes.Split(bytes.TrimSpace(b), []byte("\n")) {
		var req struct {
			ResourceSpans []struct {
				ScopeSpans []struct {
					Spans []struct {
						Name string `json:"name"`
					} `json:"spans"`
				} `json:"scopeSpans"`
			} `json:"resourceSpans"`
		}
		if err := json.Unmarshal(line, &req); err != nil {
			return names, errors.Join(err, fmt.Errorf("%s: malformed line", path))
		}
		for _, rs := range req.ResourceSpans {
			for _, ss := range rs.ScopeSpans {
				for _, s := range ss.Spans {
					names = append(names, s.Name)
				}
			}
		}
	}
	return names, nil
}
