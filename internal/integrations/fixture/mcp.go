package fixture

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCPRecording is a remote MCP server's recorded surface: its identity, its
// tool list, and the result each tool returned.
type MCPRecording struct {
	Source  string                     `json:"source"`
	Server  mcp.Implementation         `json:"server"`
	Tools   []*mcp.Tool                `json:"tools"`
	Results map[string]json.RawMessage `json:"results"` // tool name → CallToolResult
}

// Call is one tool call the fixture server received.
type Call struct {
	Tool string
	Args map[string]any
}

// MCPServer serves a recorded MCP surface over streamable HTTP on a local
// test server and returns its URL. Requests without "Authorization: Bearer
// token" get a 401. Calls are recorded in order.
type MCPServer struct {
	URL string

	mu    sync.Mutex
	calls []Call
}

// Calls returns the tool calls received so far.
func (s *MCPServer) Calls() []Call {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Call(nil), s.calls...)
}

// ServeMCP starts a fixture server for the recording at path.
func ServeMCP(t testing.TB, path, token string, extra ...*mcp.Tool) *MCPServer {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var rec MCPRecording
	if err := json.Unmarshal(b, &rec); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	fs := &MCPServer{}
	server := mcp.NewServer(&rec.Server, nil)
	for _, tool := range append(rec.Tools, extra...) {
		raw := rec.Results[tool.Name]
		server.AddTool(tool, func(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			var args map[string]any
			_ = json.Unmarshal(req.Params.Arguments, &args)
			fs.mu.Lock()
			fs.calls = append(fs.calls, Call{Tool: req.Params.Name, Args: args})
			fs.mu.Unlock()
			res := &mcp.CallToolResult{}
			if raw == nil {
				res.IsError = true
				res.Content = []mcp.Content{&mcp.TextContent{Text: "no recorded result for " + req.Params.Name}}
				return res, nil
			}
			if err := json.Unmarshal(raw, res); err != nil {
				t.Errorf("%s: result for %s: %v", path, req.Params.Name, err)
			}
			return res, nil
		})
	}
	h := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+token {
			http.Error(w, `{"error":"invalid_token"}`, http.StatusUnauthorized)
			return
		}
		h.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	fs.URL = srv.URL + "/mcp"
	return fs
}
