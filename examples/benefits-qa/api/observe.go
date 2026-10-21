package api

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Observe wraps the routes of mux so every request is an OpenTelemetry server
// span and one JSON log line that carries the span's trace_id and span_id, so
// a trace, its log, and its exception can be joined (Decision #324: what
// `aayu investigate` reads). A handler that panics is recovered into a 500,
// and the panic is recorded on the span as an exception event with its stack
// trace, as the OpenTelemetry semantic conventions for exceptions describe.
// /healthz is not traced: deploy and verify poll it, and it carries no user
// traffic.
func Observe(mux *http.ServeMux, next http.Handler, tr trace.Tracer, log *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, route := mux.Handler(r)
		if route == "GET /healthz" {
			next.ServeHTTP(w, r)
			return
		}
		ctx, span := tr.Start(r.Context(), r.Method+" "+route, trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(attribute.String("http.request.method", r.Method), attribute.String("http.route", route), attribute.String("url.path", r.URL.Path)))
		start := time.Now()
		rec := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		defer func() {
			if p := recover(); p != nil {
				span.AddEvent("exception", trace.WithAttributes(
					attribute.String("exception.type", fmt.Sprintf("%T", p)),
					attribute.String("exception.message", fmt.Sprint(p)),
					attribute.String("exception.stacktrace", string(debug.Stack())),
				))
				if !rec.wrote {
					fail(rec, http.StatusInternalServerError, "internal error")
				}
				rec.status = http.StatusInternalServerError
			}
			span.SetAttributes(attribute.Int("http.response.status_code", rec.status))
			level := slog.LevelInfo
			if rec.status >= 500 {
				span.SetStatus(codes.Error, http.StatusText(rec.status))
				level = slog.LevelError
			}
			sc := span.SpanContext()
			log.LogAttrs(ctx, level, "request", slog.String("trace_id", sc.TraceID().String()), slog.String("span_id", sc.SpanID().String()),
				slog.String("http.route", route), slog.Int("http.response.status_code", rec.status),
				slog.Float64("duration_ms", float64(time.Since(start).Microseconds())/1000))
			span.End()
		}()
		next.ServeHTTP(rec, r.WithContext(ctx))
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (s *statusWriter) WriteHeader(code int) {
	s.status, s.wrote = code, true
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusWriter) Write(b []byte) (int, error) {
	s.wrote = true
	return s.ResponseWriter.Write(b)
}
