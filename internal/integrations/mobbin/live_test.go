package mobbin

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations"
)

// TestLive searches the real Mobbin MCP server, behind AAYU_LIVE=1, with an
// OAuth access token in AAYU_SECRET_MOBBIN (skipped without one).
func TestLive(t *testing.T) {
	if os.Getenv("AAYU_LIVE") != "1" || os.Getenv("AAYU_SECRET_MOBBIN") == "" {
		t.Skip("set AAYU_LIVE=1 and AAYU_SECRET_MOBBIN to call Mobbin")
	}
	c := New(integrations.Conn{Name: "mobbin", Integration: config.Integration{Secret: "mobbin", Scope: "read"}, Key: os.Getenv("AAYU_SECRET_MOBBIN")}, nil, time.Now)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	sc, err := c.Scope(ctx)
	if err != nil || len(sc.Write) > 0 {
		t.Fatalf("scope = %+v, %v", sc, err)
	}
	set, err := c.Search(ctx, "screen", "login screen with biometric authentication", "ios", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(set.References) == 0 {
		t.Fatal("no references: the result shape may have changed")
	}
	for _, r := range set.References {
		if r.ID == "" || !Page(r.URL) {
			t.Fatalf("reference %+v is not a Mobbin page URL and ID", r)
		}
	}
	t.Logf("%d references, first %s", len(set.References), set.References[0].URL)
}
