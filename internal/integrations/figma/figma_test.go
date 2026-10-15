package figma

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations/fixture"
)

func conn(transport, url, key string) integrations.Conn {
	return integrations.Conn{Name: "figma", Integration: config.Integration{Secret: "figma", Scope: "read", Transport: transport, URL: url}, Key: key}
}

// TestREST replays a recorded REST session: who the token belongs to, two
// nodes, a rendered image URL, and local variables in the default mode with
// an alias resolved and a translucent colour.
func TestREST(t *testing.T) {
	r := &fixture.Replayer{T: t, Rec: fixture.Load(t, "testdata/rest.json"), AuthHeader: "X-Figma-Token", AuthValue: "figd_test"}
	c, err := New(conn("", "", "figd_test"), r)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	sc, err := c.Scope(ctx)
	if err != nil || sc.Account != "Rahul" || sc.Verified {
		t.Fatalf("scope = %+v, %v; want account Rahul, unverified (Figma does not report PAT scopes)", sc, err)
	}
	nodes, err := c.Nodes(ctx, "AbCdEf123", "1:2", "4:7")
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 2 || nodes[0].Name != "Button / Primary" || nodes[0].Type != "COMPONENT" || nodes[1].Name != "Login" || !json.Valid(nodes[1].Document) {
		t.Fatalf("nodes = %+v", nodes)
	}
	imgs, err := c.Images(ctx, "AbCdEf123", "png", 2, "1:2")
	if err != nil || imgs["1:2"] != "https://figma-alpha-api.s3.us-west-2.amazonaws.com/images/7f3e2d1c-button-primary" {
		t.Fatalf("images = %v, %v", imgs, err)
	}
	vars, err := c.Variables(ctx, "AbCdEf123")
	if err != nil {
		t.Fatal(err)
	}
	want := []Variable{
		{Name: "color/action/default", Collection: "Tokens", Type: "COLOR", Value: "#1a66cc"},
		{Name: "color/brand/primary", Collection: "Tokens", Type: "COLOR", Value: "#1a66cc"},
		{Name: "color/surface/overlay", Collection: "Tokens", Type: "COLOR", Value: "#00000080"},
		{Name: "font/family/body", Collection: "Tokens", Type: "STRING", Value: "Inter"},
		{Name: "space/4", Collection: "Tokens", Type: "FLOAT", Value: "16"},
	}
	if !reflect.DeepEqual(vars, want) {
		t.Fatalf("variables:\ngot  %+v\nwant %+v", vars, want)
	}
	r.Done()
}

// TestMCPOptIn: with transport = "mcp" the client reads variables and node
// metadata from the remote MCP server with the bearer token, refuses images
// (REST only), and reports the server's tools as the token's scope.
func TestMCPOptIn(t *testing.T) {
	srv := fixture.ServeMCP(t, "testdata/mcp.json", "figma-oauth-test")
	c, err := New(conn("mcp", srv.URL, "figma-oauth-test"), nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	vars, err := c.Variables(ctx, "AbCdEf123", "4:7")
	if err != nil {
		t.Fatal(err)
	}
	if want := []Variable{{Name: "color/brand/primary", Value: "#1A66CC"}, {Name: "space/4", Value: "16"}}; !reflect.DeepEqual(vars, want) {
		t.Fatalf("variables = %+v, want %+v", vars, want)
	}
	nodes, err := c.Nodes(ctx, "AbCdEf123", "4:7")
	if err != nil || len(nodes) != 1 || nodes[0].Metadata != `<frame id="4:7" name="Login" x="0" y="0" width="390" height="844" />` {
		t.Fatalf("nodes = %+v, %v", nodes, err)
	}
	want := []fixture.Call{
		{Tool: "get_variable_defs", Args: map[string]any{"fileKey": "AbCdEf123", "nodeId": "4:7"}},
		{Tool: "get_metadata", Args: map[string]any{"fileKey": "AbCdEf123", "nodeId": "4:7"}},
	}
	if got := srv.Calls(); !reflect.DeepEqual(got, want) {
		t.Fatalf("calls = %+v, want %+v", got, want)
	}
	if _, err := c.Images(ctx, "AbCdEf123", "png", 1, "4:7"); !errors.Is(err, integrations.ErrQuery) {
		t.Fatalf("images over MCP: err = %v, want ErrQuery", err)
	}
	sc, err := c.Scope(ctx)
	if err != nil || len(sc.Write) != 0 || !sc.Verified || sc.Account != "Figma Dev Mode MCP" {
		t.Fatalf("scope = %+v, %v", sc, err)
	}
}
