// Package figma is the Figma client (WP6.4, V5): the REST API with a
// personal access token is primary (nodes, rendered image URLs, local
// variables); the remote MCP server is an opt-in ([integrations.figma]
// transport = "mcp") that answers nodes and variables with a token minted
// outside aayudha, since the Go SDK's client-side OAuth is experimental.
package figma

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rahulnakmol/agentic-ayudha/internal/integrations"
)

// Default endpoints.
const (
	DefaultURL    = "https://api.figma.com"
	DefaultMCPURL = "https://mcp.figma.com/mcp"
)

// readTools are the remote MCP tools this client calls or knows to be read-only.
var readTools = []string{"get_metadata", "get_variable_defs", "get_design_context", "get_screenshot", "get_code_connect_map", "whoami"}

// Node is one Figma node.
type Node struct {
	ID       string          `json:"id"`
	Name     string          `json:"name,omitempty"`
	Type     string          `json:"type,omitempty"`
	Document json.RawMessage `json:"document,omitempty"` // REST: the node subtree as Figma returns it
	Metadata string          `json:"metadata,omitempty"` // MCP: get_metadata's answer
}

// Variable is one variable's value in its collection's default mode, with
// aliases resolved.
type Variable struct {
	Name       string `json:"name"`
	Collection string `json:"collection,omitempty"`
	Type       string `json:"type,omitempty"` // COLOR, FLOAT, STRING, BOOLEAN
	Value      string `json:"value"`          // colours as #rrggbb, or #rrggbbaa below full opacity
}

// Client reads Figma files.
type Client struct {
	mcp  bool
	base string
	key  string
	rt   http.RoundTripper
	hc   *http.Client
}

var fileKey = regexp.MustCompile(`^[A-Za-z0-9]+$`)

// New returns a client for a resolved [integrations.figma].
func New(c integrations.Conn, rt http.RoundTripper) (*Client, error) {
	switch c.Transport {
	case "", "rest":
		return &Client{base: c.BaseURL(DefaultURL), key: c.Key, rt: rt, hc: integrations.Client(rt)}, nil
	case "mcp":
		return &Client{mcp: true, base: c.BaseURL(DefaultMCPURL), key: c.Key, rt: rt}, nil
	}
	return nil, fmt.Errorf("%w: [integrations.%s] transport = %q: want rest or mcp", integrations.ErrNotConfigured, c.Name, c.Transport)
}

func (c *Client) get(ctx context.Context, path string, q url.Values, out any) error {
	u := c.base + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := integrations.NewRequest(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Figma-Token", c.key)
	return integrations.DoJSON(c.hc, req, out)
}

// Scope is doctor's reachability check. REST: GET /v1/me (Figma does not
// report a token's scopes, so the configured scope is trusted). MCP: the
// server's tool list, where any tool not known or annotated as read-only
// makes the token write-capable.
func (c *Client) Scope(ctx context.Context) (integrations.Scope, error) {
	if c.mcp {
		m, err := integrations.ConnectMCP(ctx, c.base, c.key, c.rt)
		if err != nil {
			return integrations.Scope{}, err
		}
		defer m.Close()
		return m.Scope(c.base, readTools), nil
	}
	var me struct {
		Handle string `json:"handle"`
	}
	if err := c.get(ctx, "/v1/me", nil, &me); err != nil {
		return integrations.Scope{}, err
	}
	return integrations.Scope{Account: me.Handle, Endpoint: "GET /v1/me"}, nil
}

func checkFile(file string, ids []string) error {
	if !fileKey.MatchString(file) || len(ids) == 0 {
		return fmt.Errorf("%w: figma: want a file key and at least one node ID", integrations.ErrQuery)
	}
	return nil
}

// Nodes returns the named nodes of a file.
func (c *Client) Nodes(ctx context.Context, file string, ids ...string) ([]Node, error) {
	if err := checkFile(file, ids); err != nil {
		return nil, err
	}
	if c.mcp {
		return c.mcpNodes(ctx, file, ids)
	}
	var res struct {
		Nodes map[string]*struct {
			Document json.RawMessage `json:"document"`
		} `json:"nodes"`
	}
	if err := c.get(ctx, "/v1/files/"+file+"/nodes", url.Values{"ids": {strings.Join(ids, ",")}}, &res); err != nil {
		return nil, err
	}
	out := make([]Node, 0, len(ids))
	for _, id := range ids {
		n := res.Nodes[id]
		if n == nil || len(n.Document) == 0 {
			return nil, fmt.Errorf("%w: figma file %s has no node %s", integrations.ErrProvider, file, id)
		}
		var head struct{ Name, Type string }
		_ = json.Unmarshal(n.Document, &head)
		out = append(out, Node{ID: id, Name: head.Name, Type: head.Type, Document: n.Document})
	}
	return out, nil
}

// Images returns, per node ID, the URL Figma renders it to (REST only;
// format png, jpg, svg, or pdf; scale 0.01 to 4). The URLs expire after
// Figma's retention period.
func (c *Client) Images(ctx context.Context, file, format string, scale float64, ids ...string) (map[string]string, error) {
	if err := checkFile(file, ids); err != nil {
		return nil, err
	}
	if c.mcp {
		return nil, fmt.Errorf("%w: figma images need the REST transport ([integrations.figma] transport = \"rest\" with a personal access token)", integrations.ErrQuery)
	}
	q := url.Values{"ids": {strings.Join(ids, ",")}, "format": {format}}
	if scale != 0 {
		q.Set("scale", strconv.FormatFloat(scale, 'f', -1, 64))
	}
	var res struct {
		Err    *string            `json:"err"`
		Images map[string]*string `json:"images"`
	}
	if err := c.get(ctx, "/v1/images/"+file, q, &res); err != nil {
		return nil, err
	}
	if res.Err != nil {
		return nil, fmt.Errorf("%w: figma images: %s", integrations.ErrProvider, *res.Err)
	}
	out := map[string]string{}
	for _, id := range ids {
		u := res.Images[id]
		if u == nil {
			return nil, fmt.Errorf("%w: figma could not render node %s", integrations.ErrProvider, id)
		}
		out[id] = *u
	}
	return out, nil
}

// Variables returns a file's variables sorted by name. REST returns every
// local variable (GET /v1/files/:key/variables/local, Figma Enterprise);
// MCP returns the variables the given nodes use (get_variable_defs), without
// collection or type.
func (c *Client) Variables(ctx context.Context, file string, nodes ...string) ([]Variable, error) {
	if c.mcp {
		if err := checkFile(file, nodes); err != nil {
			return nil, err
		}
		return c.mcpVariables(ctx, file, nodes)
	}
	if !fileKey.MatchString(file) {
		return nil, fmt.Errorf("%w: figma: want a file key", integrations.ErrQuery)
	}
	var res struct {
		Meta struct {
			Variables map[string]struct {
				Name         string                     `json:"name"`
				CollectionID string                     `json:"variableCollectionId"`
				Type         string                     `json:"resolvedType"`
				Values       map[string]json.RawMessage `json:"valuesByMode"`
			} `json:"variables"`
			Collections map[string]struct {
				Name        string `json:"name"`
				DefaultMode string `json:"defaultModeId"`
			} `json:"variableCollections"`
		} `json:"meta"`
	}
	if err := c.get(ctx, "/v1/files/"+file+"/variables/local", nil, &res); err != nil {
		return nil, err
	}
	var resolve func(id string, depth int) (string, error)
	resolve = func(id string, depth int) (string, error) {
		v, ok := res.Meta.Variables[id]
		if !ok || depth > 10 {
			return "", fmt.Errorf("%w: figma variable %s: unresolvable alias", integrations.ErrProvider, id)
		}
		raw := v.Values[res.Meta.Collections[v.CollectionID].DefaultMode]
		var alias struct{ Type, ID string }
		if json.Unmarshal(raw, &alias) == nil && alias.Type == "VARIABLE_ALIAS" {
			return resolve(alias.ID, depth+1)
		}
		return value(v.Type, raw)
	}
	out := make([]Variable, 0, len(res.Meta.Variables))
	for id, v := range res.Meta.Variables {
		val, err := resolve(id, 0)
		if err != nil {
			return nil, err
		}
		out = append(out, Variable{Name: v.Name, Collection: res.Meta.Collections[v.CollectionID].Name, Type: v.Type, Value: val})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func value(typ string, raw json.RawMessage) (string, error) {
	if typ == "COLOR" {
		var c struct{ R, G, B, A float64 }
		if err := json.Unmarshal(raw, &c); err != nil {
			return "", fmt.Errorf("%w: figma colour %s: %w", integrations.ErrProvider, raw, err)
		}
		b := func(x float64) int { return int(math.Round(x * 255)) }
		s := fmt.Sprintf("#%02x%02x%02x", b(c.R), b(c.G), b(c.B))
		if c.A < 1 {
			s += fmt.Sprintf("%02x", b(c.A))
		}
		return s, nil
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return "", fmt.Errorf("%w: figma %s value %s: %w", integrations.ErrProvider, typ, raw, err)
	}
	if f, ok := v.(float64); ok {
		return strconv.FormatFloat(f, 'f', -1, 64), nil
	}
	return fmt.Sprint(v), nil
}

func (c *Client) session(ctx context.Context) (*integrations.MCP, error) {
	m, err := integrations.ConnectMCP(ctx, c.base, c.key, c.rt)
	if err != nil {
		return nil, err
	}
	if err := m.Scope(c.base, readTools).Check("figma"); err != nil {
		m.Close()
		return nil, err
	}
	return m, nil
}

func (c *Client) call(ctx context.Context, m *integrations.MCP, tool, file, node string) (string, error) {
	t, err := m.Tool(tool)
	if err != nil {
		return "", err
	}
	res, err := m.Call(ctx, tool, integrations.Args(t, map[string]any{"fileKey": file, "nodeId": node}))
	if err != nil {
		return "", err
	}
	var text []string
	for _, ct := range res.Content {
		if t, ok := ct.(*mcp.TextContent); ok {
			text = append(text, t.Text)
		}
	}
	return strings.Join(text, "\n"), nil
}

func (c *Client) mcpNodes(ctx context.Context, file string, ids []string) ([]Node, error) {
	m, err := c.session(ctx)
	if err != nil {
		return nil, err
	}
	defer m.Close()
	out := make([]Node, 0, len(ids))
	for _, id := range ids {
		text, err := c.call(ctx, m, "get_metadata", file, id)
		if err != nil {
			return nil, err
		}
		out = append(out, Node{ID: id, Metadata: text})
	}
	return out, nil
}

func (c *Client) mcpVariables(ctx context.Context, file string, nodes []string) ([]Variable, error) {
	m, err := c.session(ctx)
	if err != nil {
		return nil, err
	}
	defer m.Close()
	byName := map[string]string{}
	for _, id := range nodes {
		text, err := c.call(ctx, m, "get_variable_defs", file, id)
		if err != nil {
			return nil, err
		}
		var defs map[string]any
		if err := json.Unmarshal([]byte(text), &defs); err != nil {
			return nil, fmt.Errorf("%w: figma get_variable_defs answered %q; want a JSON object of names to values", integrations.ErrProvider, text)
		}
		for k, v := range defs {
			byName[k] = fmt.Sprint(v)
		}
	}
	out := make([]Variable, 0, len(byName))
	for k, v := range byName {
		out = append(out, Variable{Name: k, Value: v})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}
