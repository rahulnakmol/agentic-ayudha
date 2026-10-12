package cli

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	aayudha "github.com/rahulnakmol/agentic-ayudha"
	"github.com/rahulnakmol/agentic-ayudha/internal/actions"
)

// serveMCP runs `aayu mcp` over a pair of pipes, exactly as a host runs it over
// stdio, and connects the official SDK client to it.
func serveMCP(t *testing.T, repo string, args ...string) *sdk.ClientSession {
	t.Helper()
	clientR, serverW := io.Pipe()
	serverR, clientW := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	home := t.TempDir()
	go func() {
		done <- Run(ctx, append([]string{"mcp"}, args...), IO{In: serverR, Out: serverW, Err: io.Discard}, Options{
			Dir: repo, Assets: aayudha.Assets, Build: BuildInfo{Version: "1.0.0"},
			Getenv:  func(k string) string { return map[string]string{"HOME": home}[k] },
			GHToken: func(context.Context) (string, error) { return "", errors.New("gh disabled in tests") },
		})
	}()
	client := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "1"}, nil)
	cs, err := client.Connect(ctx, &sdk.IOTransport{Reader: clientR, Writer: clientW}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cs.Close()
		_ = clientW.Close()
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("aayu mcp did not exit")
		}
	})
	return cs
}

func toolNames(t *testing.T, cs *sdk.ClientSession) ([]string, map[string]*sdk.Tool) {
	t.Helper()
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	byName := map[string]*sdk.Tool{}
	for _, tl := range res.Tools {
		names = append(names, tl.Name)
		byName[tl.Name] = tl
	}
	return names, byName
}

func props(t *testing.T, tl *sdk.Tool) map[string]any {
	b, _ := json.Marshal(tl.InputSchema)
	var m struct{ Properties map[string]any }
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m.Properties
}

func call(t *testing.T, cs *sdk.ClientSession, name string, args map[string]any) (string, bool) {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &sdk.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*sdk.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String(), res.IsError
}

// The never-exposed verbs: human-only or admin (registry Exposure never).
var neverExposed = []string{"decide", "ship", "release", "respond", "init", "install"}

func TestMCPDefaultExposure(t *testing.T) {
	cs := serveMCP(t, gitInit(t, "dev", nil))
	names, tools := toolNames(t, cs)
	// Read and authoring verbs only (the SDK lists tools by name).
	want := []string{"brief", "doctor", "grill", "plan", "proof", "ready", "research", "slice", "status"}
	if !slices.Equal(names, want) {
		t.Fatalf("tools = %v, want %v", names, want)
	}
	// ready is exposed as ready --check: the caller cannot choose to write.
	if _, ok := props(t, tools["ready"])["check"]; ok {
		t.Error("ready exposes check; it must be pinned to --check")
	}
	var got Values
	withHandler(t, "ready", func(_ context.Context, _ *Env, in Values) error { got = in; return nil })
	if out, isErr := call(t, cs, "ready", map[string]any{"issue": "#12"}); isErr || !got.Bool("check") || got.String("issue") != "#12" {
		t.Fatalf("ready call: %q error=%v, handler saw %v", out, isErr, got)
	}
	if out, isErr := call(t, cs, "ready", map[string]any{"issue": "12", "check": false}); !isErr || !strings.Contains(out, `unknown argument "check"`) {
		t.Fatalf("ready with check=false: %q", out)
	}
	if out, isErr := call(t, cs, "proof", map[string]any{"action": "purge", "story": "12"}); !isErr || !strings.Contains(out, "writes") {
		t.Fatalf("proof purge without expose_writes: %q", out)
	}
	if _, ok := props(t, tools["status"])["digest"]; ok {
		t.Error("status exposes --digest (a notify write) without expose_writes")
	}
	// A flag-looking positional stays positional.
	got = nil
	withHandler(t, "status", func(_ context.Context, _ *Env, in Values) error { got = in; return nil })
	if out, isErr := call(t, cs, "status", map[string]any{"issue": "--all"}); !isErr || got != nil {
		t.Fatalf("status issue=--all reached the handler: %q %v", out, got)
	}

	// Prompts and resources.
	pr, err := cs.GetPrompt(context.Background(), &sdk.GetPromptParams{Name: "grill", Arguments: map[string]string{"type": "story"}})
	if err != nil || !strings.Contains(pr.Messages[0].Content.(*sdk.TextContent).Text, "`aayu grill story`") {
		t.Fatalf("grill prompt: %v %+v", err, pr)
	}
	pr, err = cs.GetPrompt(context.Background(), &sdk.GetPromptParams{Name: "brief", Arguments: map[string]string{"subject": "7", "register": "eli5"}})
	if err != nil || !strings.Contains(pr.Messages[0].Content.(*sdk.TextContent).Text, "`aayu brief --register=eli5 7`") {
		t.Fatalf("brief prompt: %v %+v", err, pr)
	}
	rr, err := cs.ReadResource(context.Background(), &sdk.ReadResourceParams{URI: "aayudha://config"})
	if err != nil || !strings.Contains(rr.Contents[0].Text, `"key": "autonomy.streak"`) {
		t.Fatalf("config resource: %v %+v", err, rr)
	}
	if _, err := cs.ReadResource(context.Background(), &sdk.ReadResourceParams{URI: "aayudha://status/;rm"}); err == nil {
		t.Fatal("status resource accepted a non-issue id")
	}
}

// TestMCPWritesAndDryRun: with [mcp] expose_writes = true the write verbs appear
// (never the human-only ones), and a dry-run call round-trips through the
// actions log without any external request.
func TestMCPWritesAndDryRun(t *testing.T) {
	repo := gitInit(t, "dev", nil)
	cs := serveMCP(t, repo, "--set", "mcp.expose_writes=true")
	names, tools := toolNames(t, cs)
	for _, w := range []string{"triage", "build", "verify", "inspect", "tell", "evolve"} {
		if !slices.Contains(names, w) {
			t.Errorf("write verb %s missing with expose_writes", w)
		}
	}
	for _, n := range neverExposed {
		if slices.Contains(names, n) {
			t.Errorf("human-only verb %s exposed", n)
		}
	}
	if _, ok := props(t, tools["tell"])["publish"]; ok {
		t.Error("tell exposes the human-only --publish")
	}
	if _, ok := props(t, tools["triage"])["dry_run"]; !ok {
		t.Error("triage has no dry_run argument")
	}

	var requests atomic.Int32
	gh := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer gh.Close()
	var agent string
	// A stand-in triage: one label write through internal/actions, as WP2.9 does.
	withHandler(t, "triage", func(ctx context.Context, e *Env, in Values) error {
		agent = e.Getenv("AAYU_AGENT")
		l, err := e.Actions()
		if err != nil {
			return err
		}
		rec, err := l.Do(ctx, actions.Action{Kind: "github.issue.labels.add", Target: "o/r#" + strings.TrimPrefix(in.String("issue"), "#"),
			Capability: "labels-only", Request: &actions.Request{Method: "POST", Path: "/repos/o/r/issues/12/labels"}},
			func(ctx context.Context) (actions.Outcome, error) {
				req, _ := http.NewRequestWithContext(ctx, http.MethodPost, gh.URL+"/repos/o/r/issues/12/labels", nil)
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					return actions.Outcome{}, err
				}
				_ = resp.Body.Close()
				return actions.Outcome{Attempts: 1}, nil
			})
		if err != nil {
			return err
		}
		return e.Emit(rec, nil)
	})
	out, isErr := call(t, cs, "triage", map[string]any{"issue": "#12", "dry_run": true})
	var rec actions.Record
	if isErr || json.Unmarshal([]byte(out), &rec) != nil || rec.Status != actions.StatusPlanned || rec.Target != "o/r#12" {
		t.Fatalf("dry-run triage: error=%v %q", isErr, out)
	}
	if n := requests.Load(); n != 0 {
		t.Fatalf("dry run sent %d request(s)", n)
	}
	if agent != "1" {
		t.Fatalf("MCP calls must run as an agent lane, AAYU_AGENT = %q", agent)
	}
	log, err := os.ReadFile(filepath.Join(repo, ".aayudha", "telemetry", "actions.jsonl"))
	if err != nil || !strings.Contains(string(log), `"status":"planned"`) {
		t.Fatalf("actions log: %v\n%s", err, log)
	}
	if out, isErr := call(t, cs, "triage", map[string]any{"issue": "#12"}); isErr || !strings.Contains(out, `"status": "done"`) || requests.Load() != 1 {
		t.Fatalf("real triage: error=%v %q, %d request(s)", isErr, out, requests.Load())
	}
}

// TestMCPCancellation: a long verb stops when the client cancels the call.
func TestMCPCancellation(t *testing.T) {
	cs := serveMCP(t, gitInit(t, "dev", nil))
	stopped := make(chan error, 1)
	withHandler(t, "research", func(ctx context.Context, _ *Env, _ Values) error {
		<-ctx.Done()
		stopped <- ctx.Err()
		return ctx.Err()
	})
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if _, err := cs.CallTool(ctx, &sdk.CallToolParams{Name: "research", Arguments: map[string]any{"feature": "3"}}); err == nil {
		t.Fatal("cancelled call returned no error")
	}
	select {
	case err := <-stopped:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("verb stopped with %v, want context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the verb kept running after the client cancelled")
	}
}
