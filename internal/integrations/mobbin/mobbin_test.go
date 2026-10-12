package mobbin

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations/fixture"
	"github.com/rahulnakmol/agentic-ayudha/internal/secrets"
)

var fetched = time.Date(2026, 10, 12, 9, 30, 0, 0, time.UTC)

func client(t *testing.T, url string) *Client {
	t.Helper()
	conn := integrations.Conn{Name: "mobbin", Integration: config.Integration{Secret: "mobbin", Scope: "read", URL: url}, Key: "mobbin-test-token"}
	return New(conn, nil, func() time.Time { return fetched })
}

// TestResearchReferenceSetURLsOnly is the WP6.4 acceptance: a research search
// against the recorded Mobbin MCP surface returns a reference set of Mobbin
// page URLs and IDs only. The recorded answer carries two preview images,
// CDN image URLs, app names, and a description; none of them survive, and a
// result whose only link is an image is dropped.
func TestResearchReferenceSetURLsOnly(t *testing.T) {
	srv := fixture.ServeMCP(t, "testdata/mcp.json", "mobbin-test-token")
	set, err := client(t, srv.URL).Search(context.Background(), "screen", "login screen with biometric authentication", "ios", 5)
	if err != nil {
		t.Fatal(err)
	}
	want := integrations.ReferenceSet{
		Query: "login screen with biometric authentication",
		References: []integrations.Reference{
			{Source: "mobbin", Kind: "screen", ID: "3f2c1a9e-6b7d-4c1e-9a52-0d4f8e7b6c21", URL: "https://mobbin.com/screens/3f2c1a9e-6b7d-4c1e-9a52-0d4f8e7b6c21"},
			{Source: "mobbin", Kind: "screen", ID: "a81d4e02-93c5-4f7b-b1e6-5c2a7d9f0e34", URL: "https://mobbin.com/screens/a81d4e02-93c5-4f7b-b1e6-5c2a7d9f0e34"},
			{Source: "mobbin", Kind: "flow", ID: "5e6f7a8b-9c0d-4e1f-a2b3-c4d5e6f7a8b9", URL: "https://mobbin.com/flows/5e6f7a8b-9c0d-4e1f-a2b3-c4d5e6f7a8b9"},
		},
		FetchedAt: fetched,
	}
	if !reflect.DeepEqual(set, want) {
		t.Fatalf("reference set:\ngot  %+v\nwant %+v", set, want)
	}
	b, _ := json.Marshal(set)
	for _, banned := range []string{"iVBOR", "bytescale", "image", ".png", ".webp", "Monzo", "Face ID", "data:"} {
		if strings.Contains(string(b), banned) {
			t.Errorf("reference set carries %q: %s", banned, b)
		}
	}
	// Only arguments the tool declares were sent: search_screens has no limit.
	calls := srv.Calls()
	if len(calls) != 1 || !reflect.DeepEqual(calls[0], fixture.Call{Tool: "search_screens", Args: map[string]any{"query": "login screen with biometric authentication", "platform": "ios"}}) {
		t.Fatalf("calls = %+v", calls)
	}
}

// TestWriteToolRefused: a server that offers a tool outside the read-only
// search tools, not annotated read-only, makes the token write-capable; the
// search is refused before any tool call and doctor's scope reports it.
func TestWriteToolRefused(t *testing.T) {
	save := &mcp.Tool{Name: "save_to_collection", InputSchema: map[string]any{"type": "object"}}
	srv := fixture.ServeMCP(t, "testdata/mcp.json", "mobbin-test-token", save)
	c := client(t, srv.URL)
	_, err := c.Search(context.Background(), "flow", "onboarding", "", 0)
	if !errors.Is(err, secrets.ErrWriteScoped) || !strings.Contains(err.Error(), "save_to_collection") {
		t.Fatalf("err = %v, want ErrWriteScoped naming save_to_collection", err)
	}
	if n := len(srv.Calls()); n != 0 {
		t.Fatalf("%d tool calls; want none", n)
	}
	sc, err := c.Scope(context.Background())
	if err != nil || !reflect.DeepEqual(sc.Write, []string{"save_to_collection"}) || sc.Account != "mobbin" {
		t.Fatalf("scope = %+v, %v", sc, err)
	}
}

// TestBadTokenIsProviderError: the server refuses the token; the error is a
// provider error (exit 5) and does not carry the token.
func TestBadTokenIsProviderError(t *testing.T) {
	srv := fixture.ServeMCP(t, "testdata/mcp.json", "another-token")
	_, err := client(t, srv.URL).Scope(context.Background())
	if !errors.Is(err, integrations.ErrProvider) || strings.Contains(err.Error(), "mobbin-test-token") {
		t.Fatalf("err = %v, want ErrProvider without the token", err)
	}
}
