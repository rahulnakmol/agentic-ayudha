package models

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

// TestGatewayLive calls the AI gateway once per wire format. It runs only with
// AAYU_LIVE=1 and skips cleanly while the key is absent. CI runs it in
// .github/workflows/models-live.yml with `environment: ai-gateway`, never on PRs.
//
//	AAYU_LIVE=1 AAYU_AI_GATEWAY_URL=https://…/v1 AAYU_SECRET_AI_GATEWAY=… go test -run Live ./internal/models
func TestGatewayLive(t *testing.T) {
	base := os.Getenv("AAYU_MODELS__GATEWAY__BASE_URL")
	if base == "" {
		base = os.Getenv("AAYU_AI_GATEWAY_URL")
	}
	key := os.Getenv("AAYU_SECRET_AI_GATEWAY")
	switch {
	case os.Getenv("AAYU_LIVE") != "1":
		t.Skip("live model test: set AAYU_LIVE=1")
	case base == "" || key == "":
		t.Skip("live model test: AAYU_AI_GATEWAY_URL or AAYU_SECRET_AI_GATEWAY is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// Every registry model ID is served, and its owner is the family the registry gives it.
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSuffix(base, "/")+"/models", nil)
	req.Header.Set("Authorization", "Bearer "+key)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var list struct {
		Data []struct {
			ID      string `json:"id"`
			OwnedBy string `json:"owned_by"`
		} `json:"data"`
	}
	err = json.NewDecoder(resp.Body).Decode(&list)
	_ = resp.Body.Close()
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /models: %s, %v", resp.Status, err)
	}
	owners := map[string]string{}
	for _, m := range list.Data {
		owners[m.ID] = m.OwnedBy
	}
	for _, id := range []string{verbs.ModelClaudeOpus, verbs.ModelClaudeSonnet, verbs.ModelClaudeHaiku, verbs.ModelClaudeFable,
		verbs.ModelGPTSol, verbs.ModelGPTAstra, verbs.ModelGrok, verbs.ModelGemini} {
		owner, ok := owners[id]
		if !ok {
			t.Errorf("gateway does not list %s", id)
			continue
		}
		t.Logf("%s: owned_by %q, registry family %s", id, owner, verbs.FamilyOf(id))
	}

	gw := config.Models{Gateway: config.Gateway{BaseURL: base, Secret: "ai-gateway"}}
	for _, id := range []string{verbs.ModelClaudeHaiku, verbs.ModelGPTSol, verbs.ModelGrok, verbs.ModelGemini} {
		route, err := RouteFor(gw, id, verbs.FamilyOf(id))
		if err != nil {
			t.Fatal(err)
		}
		c := HTTPClient{BaseURL: route.BaseURL, Style: route.Style, Key: key}
		r, err := c.Complete(ctx, Request{Model: id, Messages: []Message{{Role: "user", Content: "Reply with the single word: pong"}}, MaxTokens: 512})
		if err != nil {
			t.Errorf("%s via %s: %v", id, route.Style, err)
			continue
		}
		t.Logf("%s via %s: %q, %d in / %d out tokens, reported cost %v", id, route.Style, strings.TrimSpace(r.Text), r.TokensIn, r.TokensOut, r.CostUSD)
		if r.TokensIn == 0 || !strings.Contains(strings.ToLower(r.Text), "pong") {
			t.Errorf("%s via %s: unexpected reply %+v", id, route.Style, r)
		}
	}
}
