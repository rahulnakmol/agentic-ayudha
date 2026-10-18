package posthog

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations/fixture"
)

// TestQuery (Decision #449, phase P1): Query checks the key reads only, then
// sends one POST of the query node to the project's query endpoint and
// returns PostHog's answer as sent with the exact request; a key that can
// write is refused before the query.
func TestQuery(t *testing.T) {
	const src = "Shaped from PostHog's OpenAPI spec (OpenAPI 3.1 v1.0.0, read 2026-10-17), operations personal_api_keys @current and query_create. Not live-proven."
	node := map[string]any{"kind": "LogsQuery", "limit": float64(2)}
	body, _ := json.Marshal(map[string]any{"query": node, "name": "aayudha logs"})
	scope := func(scopes string) fixture.Interaction {
		return fixture.Interaction{Method: "GET", URL: "https://us.posthog.com/api/personal_api_keys/@current/", Status: 200,
			Response: []byte(`{"label":"aayudha","scopes":` + scopes + `}`)}
	}
	client := func(rec ...fixture.Interaction) (*Client, *fixture.Replayer) {
		r := &fixture.Replayer{T: t, Rec: &fixture.Recording{Source: src, Interactions: rec}, AuthHeader: "Authorization", AuthValue: "Bearer phx_test"}
		c, err := New(integrations.Conn{Name: "posthog", Key: "phx_test", Integration: config.Integration{Project: "12345"}}, r, time.Now)
		if err != nil {
			t.Fatal(err)
		}
		return c, r
	}

	c, r := client(scope(`["query:read"]`), fixture.Interaction{Method: "POST", URL: "https://us.posthog.com/api/projects/12345/query/", Body: body, Status: 200,
		Response: []byte(`{"results":[{"body":"boom"}],"hasMore":false}`)})
	got, req, err := c.Query(context.Background(), "aayudha logs", node)
	r.Done()
	if err != nil || !strings.Contains(string(got), `"boom"`) || !strings.HasPrefix(req, "POST https://us.posthog.com/api/projects/12345/query/ ") {
		t.Fatalf("Query = %s, %q, %v; want the answer and the POST it sent", got, req, err)
	}

	c, r = client(fixture.Interaction{Method: "GET", URL: "https://us.posthog.com/api/personal_api_keys/@current/", Status: 401, Response: []byte(`{"detail":"Invalid token"}`)})
	if _, _, err = c.Query(context.Background(), "aayudha logs", node); err == nil {
		t.Error("a rejected key: no error; want the scope read's failure")
	}
	r.Done()

	c, r = client(scope(`["query:read"]`), fixture.Interaction{Method: "POST", URL: "https://us.posthog.com/api/projects/12345/query/", Body: body, Status: 500,
		Response: []byte(`{"detail":"server error"}`)})
	if _, req, err := c.Query(context.Background(), "aayudha logs", node); err == nil || !strings.HasPrefix(req, "POST ") {
		t.Errorf("a failed query: %q, %v; want the error with the request it sent", req, err)
	}
	r.Done()

	c, r = client(scope(`["query:read","feature_flag:write"]`))
	_, _, err = c.Query(context.Background(), "aayudha logs", node)
	r.Done()
	if err == nil || !strings.Contains(err.Error(), "feature_flag:write") {
		t.Errorf("a write-capable key: err %v; want it refused before the query, naming the write scope", err)
	}
}

// TestGet (Decision #449, phase P1): Get sends one GET of a list under the
// project and returns PostHog's answer with the exact request; a name that is
// not a list is refused before any request.
func TestGet(t *testing.T) {
	const src = "Shaped from PostHog's OpenAPI spec (OpenAPI 3.1 v1.0.0, read 2026-10-17), operation alerts_list. Not live-proven."
	client := func(rec ...fixture.Interaction) (*Client, *fixture.Replayer) {
		r := &fixture.Replayer{T: t, Rec: &fixture.Recording{Source: src, Interactions: rec}, AuthHeader: "Authorization", AuthValue: "Bearer phx_test"}
		c, err := New(integrations.Conn{Name: "posthog", Key: "phx_test", Integration: config.Integration{Project: "12345"}}, r, time.Now)
		if err != nil {
			t.Fatal(err)
		}
		return c, r
	}

	c, r := client(fixture.Interaction{Method: "GET", URL: "https://us.posthog.com/api/projects/12345/alerts/?limit=100", Status: 200,
		Response: []byte(`{"results":[{"name":"Exceptions spike"}]}`)})
	got, req, err := c.Get(context.Background(), "alerts", url.Values{"limit": {"100"}})
	r.Done()
	if err != nil || !strings.Contains(string(got), "Exceptions spike") || req != "GET https://us.posthog.com/api/projects/12345/alerts/?limit=100" {
		t.Fatalf("Get = %s, %q, %v; want the answer and the GET it sent", got, req, err)
	}

	c, r = client(fixture.Interaction{Method: "GET", URL: "https://us.posthog.com/api/projects/12345/alerts/", Status: 403, Response: []byte(`{"detail":"forbidden"}`)})
	if _, req, err := c.Get(context.Background(), "alerts", nil); err == nil || !strings.HasPrefix(req, "GET ") {
		t.Errorf("a refused read: %q, %v; want the error with the request it sent", req, err)
	}
	r.Done()

	c, r = client()
	if _, req, err := c.Get(context.Background(), "../feature_flags", nil); !errors.Is(err, integrations.ErrQuery) || req != "" {
		t.Errorf("a path for a name: %q, %v; want ErrQuery before any request", req, err)
	}
	r.Done()
}
