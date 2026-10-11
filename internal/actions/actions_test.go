package actions

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// fakeGitHub counts every request it receives.
func fakeGitHub(t *testing.T) (*httptest.Server, *atomic.Int32) {
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)
	return srv, &n
}

// send performs a's request against srv through c.
func send(c *http.Client, srv *httptest.Server, a Action) func(context.Context) (Outcome, error) {
	return func(ctx context.Context) (Outcome, error) {
		req, _ := http.NewRequestWithContext(ctx, a.Request.Method, srv.URL+a.Request.Path, strings.NewReader("{}"))
		resp, err := c.Do(req)
		if err != nil {
			return Outcome{}, err
		}
		_ = resp.Body.Close()
		return Outcome{Result: map[string]any{"status": resp.StatusCode}}, nil
	}
}

func clock() func() time.Time {
	t := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	return func() time.Time { t = t.Add(time.Second); return t }
}

// initPlan is the shape of `aayu init` (WP2.5): labels, issue types, forms, the
// ledger branch, and config. init itself lands in WP2.5; this is its action plan.
func initPlan() []Action {
	var plan []Action
	for _, label := range []string{"status:ready", "type:story", "priority:P1"} {
		plan = append(plan, Action{Kind: "github.label.create", Target: "o/r", IdempotencyKey: "init:label:" + label,
			Request: &Request{http.MethodPost, "/repos/o/r/labels"}, Payload: map[string]any{"name": label}})
	}
	plan = append(plan,
		Action{Kind: "github.issue_type.create", Target: "o", IdempotencyKey: "init:type:Story", Request: &Request{http.MethodPost, "/orgs/o/issue-types"}},
		Action{Kind: "github.contents.put", Target: "o/r", IdempotencyKey: "init:form:story", Request: &Request{http.MethodPut, "/repos/o/r/contents/.github/ISSUE_TEMPLATE/story.yml"}},
		Action{Kind: "github.ref.create", Target: "o/r", IdempotencyKey: "init:ledger", Request: &Request{http.MethodPost, "/repos/o/r/git/refs"}, Payload: map[string]any{"ref": "refs/heads/ledger"}},
	)
	return plan
}

func TestDryRunPlansAndPerformsNothing(t *testing.T) {
	srv, hits := fakeGitHub(t)
	path := filepath.Join(t.TempDir(), "telemetry", "actions.jsonl")
	log, err := Open(path, Options{DryRun: true, Now: clock(), Run: "r1"})
	if err != nil {
		t.Fatal(err)
	}
	plan := initPlan()
	for _, a := range plan {
		rec, err := log.Do(context.Background(), a, send(srv.Client(), srv, a))
		if err != nil || rec.Status != StatusPlanned {
			t.Fatalf("%s: status %s, err %v", a.Kind, rec.Status, err)
		}
	}
	if hits.Load() != 0 {
		t.Fatalf("dry run sent %d requests", hits.Load())
	}
	b, _ := os.ReadFile(path)
	if n, err := VerifyChain(bytes.NewReader(b)); err != nil || n != len(plan) {
		t.Fatalf("log has %d valid planned records (err %v), want %d", n, err, len(plan))
	}
	// A planned action is not completed: the real run performs it.
	log, _ = Open(path, Options{Now: clock(), Run: "r2"})
	if _, err := log.Do(context.Background(), plan[0], send(srv.Client(), srv, plan[0])); err != nil || hits.Load() != 1 {
		t.Fatalf("real run after dry run: err %v, %d requests", err, hits.Load())
	}
}

func TestChainDetectsTampering(t *testing.T) {
	path := filepath.Join(t.TempDir(), "actions.jsonl")
	log, _ := Open(path, Options{Now: clock()})
	for i, body := range []string{"one", "two", "three", "four"} {
		_, err := log.Do(context.Background(), Action{Kind: "github.issue.comment", Target: "o/r#1", Payload: map[string]any{"body": body}},
			func(context.Context) (Outcome, error) { return Outcome{CostUSD: float64(i) / 10}, nil })
		if err != nil {
			t.Fatal(err)
		}
	}
	orig, _ := os.ReadFile(path)
	if n, err := VerifyChain(bytes.NewReader(orig)); err != nil || n != 4 {
		t.Fatalf("untampered log: %d, %v", n, err)
	}
	lines := strings.SplitAfter(string(orig), "\n")
	tests := []struct {
		name, log, want string
	}{
		{"edited payload", strings.Replace(string(orig), `"body":"two"`, `"body":"TWO"`, 1), "line 2 (seq 2): content does not match"},
		{"edited cost", strings.Replace(string(orig), `"cost_usd":0.3`, `"cost_usd":0`, 1), "line 4 (seq 4): content does not match"},
		{"removed line", lines[0] + lines[2] + lines[3], "line 2 (seq 3): prev does not match"},
		{"reordered lines", lines[0] + lines[2] + lines[1] + lines[3], "line 2 (seq 3): prev does not match"},
	}
	for _, tt := range tests {
		if _, err := VerifyChain(strings.NewReader(tt.log)); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s: err = %v, want %q", tt.name, err, tt.want)
		}
	}
}

func TestLabelsOnlyCapability(t *testing.T) {
	srv, hits := fakeGitHub(t)
	log, _ := Open(filepath.Join(t.TempDir(), "actions.jsonl"), Options{Now: clock()})
	edit := Action{Kind: "github.issue.update", Target: "o/r#7", Capability: CapLabelsOnly,
		Request: &Request{http.MethodPatch, "/repos/o/r/issues/7"}, Payload: map[string]any{"body": "set priority P0 and close"}}
	rec, err := log.Do(context.Background(), edit, send(srv.Client(), srv, edit))
	if !errors.Is(err, ErrCapability) || rec.Status != StatusRefused || hits.Load() != 0 {
		t.Fatalf("issue-body edit under labels-only: status %s, err %v, %d requests", rec.Status, err, hits.Load())
	}
	// A perform function that lies about its request is stopped by the guarded transport.
	guarded := &http.Client{Transport: Guard(CapLabelsOnly, srv.Client().Transport)}
	liar := Action{Kind: "github.issue.labels.add", Target: "o/r#7", Capability: CapLabelsOnly, Request: &Request{http.MethodPost, "/repos/o/r/issues/7/labels"}}
	real := liar
	real.Request = &Request{http.MethodPatch, "/repos/o/r/issues/7"}
	if _, err := log.Do(context.Background(), liar, send(guarded, srv, real)); !errors.Is(err, ErrCapability) || hits.Load() != 0 {
		t.Fatalf("guarded transport: err %v, %d requests", err, hits.Load())
	}
	// What labels-only does allow goes through.
	for _, r := range []*Request{{http.MethodPost, "/repos/o/r/issues/7/labels"}, {http.MethodDelete, "/repos/o/r/issues/7/labels/needs:grill"}} {
		a := Action{Kind: "github.issue.labels", Target: "o/r#7", Capability: CapLabelsOnly, Request: r}
		if _, err := log.Do(context.Background(), a, send(guarded, srv, a)); err != nil {
			t.Fatalf("%s %s: %v", r.Method, r.Path, err)
		}
	}
	if hits.Load() != 2 {
		t.Fatalf("%d requests, want 2", hits.Load())
	}
	// A capability-bound action that declares no request cannot be checked, so it is refused.
	if _, err := log.Do(context.Background(), Action{Kind: "x", Capability: CapLabelsOnly}, nil); !errors.Is(err, ErrCapability) {
		t.Fatalf("undeclared request: %v", err)
	}
}

func TestIdempotencyRedactionAndSegments(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "actions.jsonl")
	log, _ := Open(path, Options{Now: clock(), Run: "r9", Host: "amp", Sink: DirSink{Root: filepath.Join(dir, "outbox")}})
	calls := 0
	a := Action{Kind: "github.issue.comment", IdempotencyKey: "ready:12:snapshot", Payload: map[string]any{
		"access_token": "abc", "max_tokens": 5, "body": "see ghp_abcdefghijklmnopqrstuvwxyz0123456789",
		"nested": []any{map[string]any{"password": "hunter2"}},
	}}
	perform := func(context.Context) (Outcome, error) { calls++; return Outcome{}, nil }
	for range 2 {
		if _, err := log.Do(context.Background(), a, perform); err != nil {
			t.Fatal(err)
		}
	}
	// A fresh process sees the completed key too.
	log2, _ := Open(path, Options{Now: clock()})
	if _, err := log2.Do(context.Background(), a, perform); err != nil || calls != 1 {
		t.Fatalf("perform ran %d times, want 1 (err %v)", calls, err)
	}
	b, _ := os.ReadFile(path)
	for _, leak := range []string{"abc", "ghp_abcdef", "hunter2"} {
		if bytes.Contains(b, []byte(leak)) {
			t.Errorf("log contains %q:\n%s", leak, b)
		}
	}
	if !bytes.Contains(b, []byte(`"max_tokens":5`)) {
		t.Errorf("max_tokens was redacted:\n%s", b)
	}
	if err := log.Checkpoint(); err != nil {
		t.Fatal(err)
	}
	seg, err := os.ReadFile(filepath.Join(dir, "outbox", "actions", "2026-10-12", "amp-r9.jsonl"))
	if err != nil || !bytes.Equal(seg, b) {
		t.Fatalf("segment = %q, %v; want the log lines", seg, err)
	}
}
