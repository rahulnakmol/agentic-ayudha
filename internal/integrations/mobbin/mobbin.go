// Package mobbin is the Mobbin MCP client (WP6.4). Mobbin's terms allow
// linking to its references, not copying its pixels (J3-18), so a search
// returns Mobbin page URLs and IDs only: preview images, image URLs, app
// names, and descriptions in the server's answer are dropped here and never
// reach a caller.
package mobbin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rahulnakmol/agentic-ayudha/internal/integrations"
)

// DefaultURL is Mobbin's remote MCP endpoint (streamable HTTP).
const DefaultURL = "https://api.mobbin.com/mcp"

// Tools are Mobbin's search tools by reference kind; the published tool set
// is read-only.
var Tools = map[string]string{"screen": "search_screens", "flow": "search_flows", "section": "search_sections"}

var readTools = []string{"search_screens", "search_flows", "search_sections"}

// Client searches Mobbin.
type Client struct {
	endpoint string
	key      string
	rt       http.RoundTripper
	now      func() time.Time
}

// New returns a client for a resolved [integrations.mobbin]; its secret is
// an OAuth access token for the MCP endpoint.
func New(c integrations.Conn, rt http.RoundTripper, now func() time.Time) *Client {
	return &Client{endpoint: c.BaseURL(DefaultURL), key: c.Key, rt: rt, now: now}
}

// Scope connects and lists the server's tools (doctor's reachability check).
func (c *Client) Scope(ctx context.Context) (integrations.Scope, error) {
	m, err := integrations.ConnectMCP(ctx, c.endpoint, c.key, c.rt)
	if err != nil {
		return integrations.Scope{}, err
	}
	defer m.Close()
	return m.Scope(c.endpoint, readTools), nil
}

// Search runs one Mobbin search for kind (screen, flow, or section) and
// returns the references it found as URLs and IDs. platform (ios, web) and
// limit are sent only when the tool declares them; zero values are omitted.
func (c *Client) Search(ctx context.Context, kind, query, platform string, limit int) (integrations.ReferenceSet, error) {
	tool, ok := Tools[kind]
	if !ok || strings.TrimSpace(query) == "" {
		return integrations.ReferenceSet{}, fmt.Errorf("%w: mobbin search: want kind screen, flow, or section and a query", integrations.ErrQuery)
	}
	m, err := integrations.ConnectMCP(ctx, c.endpoint, c.key, c.rt)
	if err != nil {
		return integrations.ReferenceSet{}, err
	}
	defer m.Close()
	if err := m.Scope(c.endpoint, readTools).Check("mobbin"); err != nil {
		return integrations.ReferenceSet{}, err
	}
	t, err := m.Tool(tool)
	if err != nil {
		return integrations.ReferenceSet{}, err
	}
	want := map[string]any{"query": query}
	if platform != "" {
		want["platform"] = platform
	}
	if limit > 0 {
		want["limit"] = limit
	}
	args := integrations.Args(t, want)
	if _, ok := args["query"]; !ok {
		return integrations.ReferenceSet{}, fmt.Errorf("%w: mobbin %s takes no query argument; its schema changed", integrations.ErrProvider, tool)
	}
	res, err := m.Call(ctx, tool, args)
	if err != nil {
		return integrations.ReferenceSet{}, err
	}
	return integrations.ReferenceSet{Query: query, References: References(kind, res), FetchedAt: c.now().UTC()}, nil
}

// References extracts Mobbin references from a tool result: every object
// with an ID and a Mobbin page URL, in the structured content or in JSON
// text, and Mobbin page URLs in plain text. Image, audio, and embedded
// content blocks are skipped, as is every other field.
func References(kind string, res *mcp.CallToolResult) []integrations.Reference {
	var out []integrations.Reference
	seen := map[string]bool{}
	add := func(kind, id, u string) {
		if id == "" || seen[id] {
			return
		}
		seen[id] = true
		out = append(out, integrations.Reference{Source: "mobbin", Kind: kind, ID: id, URL: u})
	}
	inKind := func(id, u string) { add(kind, id, u) }
	walk(res.StructuredContent, inKind)
	for _, c := range res.Content {
		t, ok := c.(*mcp.TextContent)
		if !ok {
			continue // images and other blobs are never read
		}
		var v any
		if json.Unmarshal([]byte(t.Text), &v) == nil {
			walk(v, inKind)
			continue
		}
		for _, m := range pageInText.FindAllStringSubmatch(t.Text, -1) {
			add(strings.TrimSuffix(m[1], "s"), m[2], m[0]) // the link's own kind: screens → screen
		}
	}
	return out
}

var (
	pageInText = regexp.MustCompile(`https://(?:www\.)?mobbin\.com/(screens|flows|sections)/([0-9a-fA-F-]{36})`)
	idKeys     = []string{"id", "screen_id", "flow_id", "section_id"}
	urlKeys    = []string{"mobbin_url", "url", "link", "web_url"}
	imageExt   = []string{".png", ".jpg", ".jpeg", ".gif", ".webp", ".avif", ".svg"}
)

func walk(v any, add func(id, u string)) {
	switch x := v.(type) {
	case []any:
		for _, e := range x {
			walk(e, add)
		}
	case map[string]any:
		id, u := first(x, idKeys), ""
		for _, k := range urlKeys {
			if s, ok := x[k].(string); ok && Page(s) {
				u = s
				break
			}
		}
		if id != "" && u != "" {
			add(id, u)
			return
		}
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys) // deterministic order across nested lists
		for _, k := range keys {
			walk(x[k], add)
		}
	}
}

func first(m map[string]any, keys []string) string {
	for _, k := range keys {
		if s, ok := m[k].(string); ok && s != "" {
			return s
		}
	}
	return ""
}

// Page reports whether u is a Mobbin page (https on mobbin.com, not an image
// file): the only kind of URL a reference may carry.
func Page(u string) bool {
	p, err := url.Parse(u)
	if err != nil || p.Scheme != "https" || (p.Host != "mobbin.com" && p.Host != "www.mobbin.com") {
		return false
	}
	ext := strings.ToLower(path.Ext(p.Path))
	for _, e := range imageExt {
		if ext == e {
			return false
		}
	}
	return true
}
