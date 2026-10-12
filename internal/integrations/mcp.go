package integrations

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Remote MCP servers (WP6.4): Mobbin, and Figma when [integrations.figma]
// transport = "mcp". The bearer token is minted outside aayudha (V5: the Go
// SDK's client-side OAuth is experimental) and stored as the integration's
// secret.

// MCP is one remote MCP session.
type MCP struct {
	Session *mcp.ClientSession
	Tools   []*mcp.Tool
}

// ConnectMCP opens a streamable-HTTP session to endpoint with the bearer
// token and lists the server's tools. The transport may only POST to the
// endpoint (JSON-RPC) and DELETE it (session end), besides GETs.
func ConnectMCP(ctx context.Context, endpoint, token string, rt http.RoundTripper) (*MCP, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" {
		return nil, fmt.Errorf("%w: MCP endpoint %q", ErrNotConfigured, endpoint)
	}
	path := regexp.MustCompile("^" + regexp.QuoteMeta(u.Path) + "$")
	hc := &http.Client{
		Transport: ReadOnly(bearer{token, rt}, Allow{Method: http.MethodPost, Path: path}, Allow{Method: http.MethodDelete, Path: path}),
		Timeout:   2 * time.Minute,
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "aayudha", Version: "1"}, nil)
	s, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: endpoint, HTTPClient: hc, DisableStandaloneSSE: true, MaxRetries: -1}, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: MCP %s: %w", ErrProvider, endpoint, err)
	}
	m := &MCP{Session: s}
	for t, err := range s.Tools(ctx, nil) {
		if err != nil {
			_ = s.Close()
			return nil, fmt.Errorf("%w: MCP %s tools/list: %w", ErrProvider, endpoint, err)
		}
		m.Tools = append(m.Tools, t)
	}
	return m, nil
}

// Close ends the session.
func (m *MCP) Close() { _ = m.Session.Close() }

// Scope treats the server's tools as the token's scope: a tool is read-only
// when the server annotates it so or it is one of the read tools the adapter
// knows; any other tool makes the token write-capable.
func (m *MCP) Scope(endpoint string, readTools []string) Scope {
	s := Scope{Verified: true, Endpoint: "MCP tools/list at " + endpoint}
	if r := m.Session.InitializeResult(); r != nil && r.ServerInfo != nil {
		s.Account = r.ServerInfo.Name
	}
	for _, t := range m.Tools {
		s.Scopes = append(s.Scopes, t.Name)
		if !slices.Contains(readTools, t.Name) && (t.Annotations == nil || !t.Annotations.ReadOnlyHint) {
			s.Write = append(s.Write, t.Name)
		}
	}
	return s
}

// Tool returns the named tool, or ErrProvider when the server does not offer it.
func (m *MCP) Tool(name string) (*mcp.Tool, error) {
	for _, t := range m.Tools {
		if t.Name == name {
			return t, nil
		}
	}
	return nil, fmt.Errorf("%w: the MCP server offers no %s tool", ErrProvider, name)
}

// Args keeps the arguments the tool's input schema declares, so a call never
// sends a parameter the server does not know.
func Args(t *mcp.Tool, want map[string]any) map[string]any {
	props := map[string]any{}
	if s, ok := t.InputSchema.(map[string]any); ok {
		props, _ = s["properties"].(map[string]any)
	}
	out := map[string]any{}
	for k, v := range want {
		if _, ok := props[k]; ok {
			out[k] = v
		}
	}
	return out
}

// Call calls a tool; a tool error is ErrProvider.
func (m *MCP) Call(ctx context.Context, name string, args map[string]any) (*mcp.CallToolResult, error) {
	res, err := m.Session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		return nil, fmt.Errorf("%w: MCP %s: %w", ErrProvider, name, err)
	}
	if res.IsError {
		msg := ""
		for _, c := range res.Content {
			if t, ok := c.(*mcp.TextContent); ok {
				msg += t.Text
			}
		}
		return nil, fmt.Errorf("%w: MCP %s failed: %s", ErrProvider, name, excerpt([]byte(msg)))
	}
	return res, nil
}

type bearer struct {
	token string
	next  http.RoundTripper
}

func (b bearer) RoundTrip(req *http.Request) (*http.Response, error) {
	r := req.Clone(req.Context())
	r.Header.Set("Authorization", "Bearer "+b.token)
	next := b.next
	if next == nil {
		next = http.DefaultTransport
	}
	return next.RoundTrip(r)
}

// Reference is one design reference: where it is and its ID at the source,
// never its pixels (WP6.4, J3-18).
type Reference struct {
	Source string `json:"source"` // mobbin, figma
	Kind   string `json:"kind"`   // screen, flow, section, node
	ID     string `json:"id"`
	URL    string `json:"url"`
}

// ReferenceSet is what a research search returns.
type ReferenceSet struct {
	Query      string      `json:"query"`
	References []Reference `json:"references"`
	FetchedAt  time.Time   `json:"fetched_at"`
}
