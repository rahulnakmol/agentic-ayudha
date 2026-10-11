package cli

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/telemetry"
)

func gitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestTelemetryOptIn(t *testing.T) {
	var mu sync.Mutex
	var bodies []string
	collector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, r.URL.Path+" "+string(b))
		mu.Unlock()
	}))
	defer collector.Close()

	// Default: a local trace file, and nothing sent anywhere, even with the
	// standard OTEL_* variables set.
	repo := gitRepo(t)
	env := map[string]string{"OTEL_EXPORTER_OTLP_ENDPOINT": collector.URL, "OTEL_TRACES_EXPORTER": "otlp"}
	if code, _, errs := run(t, env, Options{Dir: repo}, "version"); code != 0 {
		t.Fatal(errs)
	}
	names, err := telemetry.ReadSpanNames(filepath.Join(repo, ".aayudha", "telemetry", "traces.jsonl"))
	if err != nil || !slices.Contains(names, "aayu version") {
		t.Fatalf("trace file spans = %v, %v", names, err)
	}
	if len(bodies) != 0 {
		t.Fatalf("telemetry sent without opt-in: %v", bodies)
	}

	// Opt-in: [telemetry] otlp_endpoint sends OTLP JSON to /v1/traces.
	if code, _, errs := run(t, nil, Options{Dir: repo}, "version", "--set", "telemetry.otlp_endpoint="+collector.URL); code != 0 {
		t.Fatal(errs)
	}
	if len(bodies) != 1 || !strings.HasPrefix(bodies[0], "/v1/traces ") || !strings.Contains(bodies[0], `"name":"aayu version"`) {
		t.Fatalf("collector got %v", bodies)
	}

	// Disabled: no file at all.
	off := gitRepo(t)
	run(t, nil, Options{Dir: off}, "version", "--set", "telemetry.disabled=true")
	if _, err := os.Stat(filepath.Join(off, ".aayudha", "telemetry", "traces.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("trace file written while disabled: %v", err)
	}
}
