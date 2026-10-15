package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

const tokens = "../../../../packages/design-system/src/styles/tokens.css"

// The command serves the Ask page, the API, and /healthz on one address,
// appends product events to the -events export, and stops cleanly.
func TestRun(t *testing.T) {
	events := filepath.Join(t.TempDir(), "events.jsonl")
	ctx, cancel := context.WithCancel(context.Background())
	ready := make(chan string, 1)
	done := make(chan error, 1)
	var stderr bytes.Buffer
	go func() {
		done <- run(ctx, []string{"-addr", "127.0.0.1:0", "-tokens", tokens, "-events", events}, &stderr, ready)
	}()
	var addr string
	select {
	case addr = <-ready:
	case err := <-done:
		t.Fatalf("run: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("no address")
	}
	base := "http://" + addr
	for path, want := range map[string]string{"/healthz": "ok", "/": "Ask about your benefits"} {
		resp, err := http.Get(base + path)
		if err != nil {
			t.Fatal(err)
		}
		if pid := resp.Header.Get(PIDHeader); (path == "/healthz") != (pid == strconv.Itoa(os.Getpid())) {
			t.Errorf("%s: %s header %q", path, PIDHeader, pid)
		}
		b, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != 200 || !strings.Contains(string(b), want) {
			t.Errorf("%s: %d", path, resp.StatusCode)
		}
	}
	resp, err := http.Post(base+"/api/ask", "application/json", strings.NewReader(`{"question":"How many sick days do I get?","user_id":"emp-0003"}`))
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("/api/ask: %v %v", resp, err)
	}
	_ = resp.Body.Close()
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	b, _ := os.ReadFile(events)
	for _, want := range []string{`"Name":"benefits_visit"`, `"Name":"question_asked","UserAuthenticatedId":"emp-0003"`, `"Name":"answer_grounded","UserAuthenticatedId":"emp-0003"`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("events lack %s:\n%s", want, b)
		}
	}
}

func TestRunRefuses(t *testing.T) {
	for _, args := range [][]string{{"-bogus"}, {"-tokens", "no/such/tokens.css"}, {"-tokens", tokens, "-addr", "256.0.0.1:1"},
		{"-tokens", tokens, "-events", filepath.Join(t.TempDir(), "no", "dir", "e.jsonl")}} {
		if err := run(context.Background(), args, io.Discard, nil); err == nil {
			t.Errorf("%v accepted", args)
		}
	}
}
