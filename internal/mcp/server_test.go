package mcp

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

// TestLongVerbProgressAndTimeout: a verb that runs past the progress interval
// reports progress to a client that asked for it, and one that outlives the
// timeout is stopped and reported as timed out.
func TestLongVerbProgressAndTimeout(t *testing.T) {
	run := func(ctx context.Context, args []string, stdout, _ io.Writer) int {
		select {
		case <-time.After(120 * time.Millisecond):
			_, _ = io.WriteString(stdout, `{"ok":true}`)
			return 0
		case <-ctx.Done():
			return 1
		}
	}
	srv, err := NewServer(Options{Version: "t", Run: run, Progress: 20 * time.Millisecond, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	var progress atomic.Int32
	client := sdk.NewClient(&sdk.Implementation{Name: "t", Version: "1"}, &sdk.ClientOptions{
		ProgressNotificationHandler: func(context.Context, *sdk.ProgressNotificationClientRequest) { progress.Add(1) },
	})
	ct, st := sdk.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := srv.Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cs.Close() }()

	params := &sdk.CallToolParams{Name: "status", Arguments: map[string]any{}}
	params.SetProgressToken("p1")
	res, err := cs.CallTool(ctx, params)
	if err != nil || res.IsError || res.Content[0].(*sdk.TextContent).Text != `{"ok":true}` {
		t.Fatalf("status: %v %+v", err, res)
	}
	if progress.Load() < 2 {
		t.Fatalf("%d progress notification(s) for a 120 ms run at a 20 ms interval, want >= 2", progress.Load())
	}

	srv2, _ := NewServer(Options{Version: "t", Run: run, Timeout: 30 * time.Millisecond})
	ct2, st2 := sdk.NewInMemoryTransports()
	if _, err := srv2.Connect(ctx, st2, nil); err != nil {
		t.Fatal(err)
	}
	cs2, err := client.Connect(ctx, ct2, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cs2.Close() }()
	params = &sdk.CallToolParams{Name: "status", Arguments: map[string]any{}}
	params.SetProgressToken("p2") // the progress reporter stops with the deadline too
	res, err = cs2.CallTool(ctx, params)
	if err != nil || !res.IsError || !strings.Contains(res.Content[0].(*sdk.TextContent).Text, "timed out after 30ms") {
		t.Fatalf("timeout: %v %+v", err, res)
	}
}

// TestArgs: tool arguments become CLI arguments only when every value has the
// type the registry declares; anything else is refused with the reason, before
// a verb runs.
func TestArgs(t *testing.T) {
	find := func(name string) verbs.Command {
		c, ok := verbs.Find(name)
		if !ok {
			t.Fatal(name)
		}
		return c
	}
	// A registry-shaped command whose optional positional comes first.
	twoPos := verbs.Command{Name: "pair", Inputs: []verbs.Input{
		{Name: "first", Type: verbs.TypeString, Positional: true},
		{Name: "second", Type: verbs.TypeString, Positional: true},
	}}
	for _, tc := range []struct {
		cmd          verbs.Command
		writes       bool
		raw          string
		want, errMsg string
	}{
		{cmd: find("experiment"), writes: true, raw: `{"feature": 3, "hypothesis": "h", "metric": "m", "flag": "f", "mde": "0.02", "days": 14, "dry_run": true}`,
			want: "experiment --json --dry-run --hypothesis=h --metric=m --flag=f --mde=0.02 --days=14 -- 3"},
		{cmd: find("experiment"), writes: true, raw: `{"feature": "3", "hypothesis": "h", "metric": "m", "flag": "f", "mde": "0.02", "days": "14"}`, errMsg: "days: want an integer"},
		{cmd: find("experiment"), writes: true, raw: `{"feature": "3", "hypothesis": "h", "metric": "m", "flag": "f", "mde": "0.02", "days": 1.5}`, errMsg: "days: want an integer"},
		{cmd: find("status"), raw: `[1]`, errMsg: "arguments must be a JSON object"},
		{cmd: find("status"), raw: `{"nope": 1}`, errMsg: `unknown argument "nope"`},
		{cmd: find("status"), raw: `{"all": "yes"}`, errMsg: "all: want a boolean"},
		{cmd: find("verify"), writes: true, raw: `{"dry_run": "yes"}`, errMsg: "dry_run: want a boolean"},
		{cmd: find("verify"), writes: true, raw: `{"pr": "main"}`, errMsg: "pr: want a pull request number"},
		{cmd: find("research"), raw: `{}`, errMsg: "feature is required"},
		{cmd: find("research"), raw: `{"feature": ""}`, errMsg: "feature: want a non-empty string"},
		{cmd: find("research"), raw: `{"feature": "3", "platform": "desktop"}`, errMsg: "platform: want one of ios, android, web"},
		{cmd: twoPos, raw: `{"second": "b"}`, errMsg: "second needs first before it"},
		{cmd: twoPos, raw: `{"first": "a", "second": "b"}`, want: "pair --json -- a b"},
	} {
		got, err := Args(tc.cmd, json.RawMessage(tc.raw), tc.writes)
		switch {
		case tc.errMsg != "" && (err == nil || !strings.Contains(err.Error(), tc.errMsg)):
			t.Errorf("%s %s: err %v, want %q", tc.cmd.Name, tc.raw, err, tc.errMsg)
		case tc.errMsg == "" && (err != nil || strings.Join(got, " ") != tc.want):
			t.Errorf("%s %s: %q %v, want %q", tc.cmd.Name, tc.raw, strings.Join(got, " "), err, tc.want)
		}
	}
	// A verb on the MCP surface marked never-exposed stays hidden even with writes exposed.
	never := verbs.Command{Name: "x", Surfaces: []verbs.Surface{verbs.SurfaceMCP}, Exposure: verbs.ExposeNever}
	if Exposed(never, true) {
		t.Error("a never-exposed verb is exposed")
	}
}

// TestPromptsResourcesAndExitCodes: the grill prompt refuses an unknown type
// and frames the brand as design/brand.yaml; the brief prompt refuses invalid
// arguments; the status and ledger resources run the matching read verb and
// report a failing one; a verb's non-zero exit reaches the client as an error
// carrying the exit code and stderr.
func TestPromptsResourcesAndExitCodes(t *testing.T) {
	var calls [][]string
	run := func(_ context.Context, args []string, stdout, stderr io.Writer) int {
		calls = append(calls, args)
		if slices.Contains(args, "404") || args[0] == "doctor" {
			_, _ = io.WriteString(stderr, "not found")
			return 3
		}
		_, _ = io.WriteString(stdout, `{"ok":true}`)
		return 0
	}
	srv, err := NewServer(Options{Version: "t", Run: run, Assets: os.DirFS(filepath.Join("..", ".."))})
	if err != nil {
		t.Fatal(err)
	}
	ct, st := sdk.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := srv.Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := sdk.NewClient(&sdk.Implementation{Name: "t", Version: "1"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cs.Close() }()

	if _, err := cs.GetPrompt(ctx, &sdk.GetPromptParams{Name: "grill", Arguments: map[string]string{"type": "epic"}}); err == nil || !strings.Contains(err.Error(), `type "epic": want one of`) {
		t.Errorf("grill epic: %v", err)
	}
	pr, err := cs.GetPrompt(ctx, &sdk.GetPromptParams{Name: "grill", Arguments: map[string]string{"type": "brand"}})
	if err != nil || !strings.Contains(pr.Messages[0].Content.(*sdk.TextContent).Text, "Grill the user for the brand (design/brand.yaml)") ||
		!strings.Contains(pr.Messages[0].Content.(*sdk.TextContent).Text, "`aayu grill brand`") {
		t.Errorf("grill brand: %v %+v", err, pr)
	}
	if _, err := cs.GetPrompt(ctx, &sdk.GetPromptParams{Name: "brief", Arguments: map[string]string{"subject": "7", "register": "pirate"}}); err == nil || !strings.Contains(err.Error(), "register: want one of") {
		t.Errorf("brief register=pirate: %v", err)
	}

	for uri, want := range map[string]string{"aayudha://status/12": "status --json -- 12", "aayudha://ledger/12": "proof --json -- get 12"} {
		calls = nil
		rr, err := cs.ReadResource(ctx, &sdk.ReadResourceParams{URI: uri})
		if err != nil || rr.Contents[0].Text != `{"ok":true}` || len(calls) != 1 || strings.Join(calls[0], " ") != want {
			t.Errorf("%s: %v %+v, ran %v", uri, err, rr, calls)
		}
	}
	if _, err := cs.ReadResource(ctx, &sdk.ReadResourceParams{URI: "aayudha://ledger/404"}); err == nil || !strings.Contains(err.Error(), "exited 3: not found") {
		t.Errorf("failing ledger read: %v", err)
	}

	res, err := cs.CallTool(ctx, &sdk.CallToolParams{Name: "doctor", Arguments: map[string]any{}})
	if err != nil || !res.IsError || res.Content[0].(*sdk.TextContent).Text != "exit 3\nnot found" {
		t.Errorf("doctor exiting 3: %v %+v", err, res)
	}
}
