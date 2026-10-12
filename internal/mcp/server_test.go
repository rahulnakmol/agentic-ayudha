package mcp

import (
	"context"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
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
	res, err = cs2.CallTool(ctx, &sdk.CallToolParams{Name: "status", Arguments: map[string]any{}})
	if err != nil || !res.IsError || !strings.Contains(res.Content[0].(*sdk.TextContent).Text, "timed out after 30ms") {
		t.Fatalf("timeout: %v %+v", err, res)
	}
}
