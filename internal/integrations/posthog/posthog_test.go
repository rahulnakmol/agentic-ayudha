package posthog

import (
	"context"
	"encoding/json"
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

	c, r = client(scope(`["query:read","feature_flag:write"]`))
	_, _, err = c.Query(context.Background(), "aayudha logs", node)
	r.Done()
	if err == nil || !strings.Contains(err.Error(), "feature_flag:write") {
		t.Errorf("a write-capable key: err %v; want it refused before the query, naming the write scope", err)
	}
}
