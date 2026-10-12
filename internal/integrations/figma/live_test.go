package figma

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestLive reads a real Figma file over REST, behind AAYU_LIVE=1, with a
// personal access token in AAYU_SECRET_FIGMA and a file key and node ID in
// AAYU_LIVE_FIGMA_FILE and AAYU_LIVE_FIGMA_NODE (skipped without them).
func TestLive(t *testing.T) {
	file, node := os.Getenv("AAYU_LIVE_FIGMA_FILE"), os.Getenv("AAYU_LIVE_FIGMA_NODE")
	if os.Getenv("AAYU_LIVE") != "1" || os.Getenv("AAYU_SECRET_FIGMA") == "" || file == "" || node == "" {
		t.Skip("set AAYU_LIVE=1, AAYU_SECRET_FIGMA, AAYU_LIVE_FIGMA_FILE, and AAYU_LIVE_FIGMA_NODE to call Figma")
	}
	c, err := New(conn("", "", os.Getenv("AAYU_SECRET_FIGMA")), nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if _, err := c.Scope(ctx); err != nil {
		t.Fatal(err)
	}
	nodes, err := c.Nodes(ctx, file, node)
	if err != nil || len(nodes) != 1 {
		t.Fatalf("nodes = %+v, %v", nodes, err)
	}
	imgs, err := c.Images(ctx, file, "png", 1, node)
	if err != nil || imgs[node] == "" {
		t.Fatalf("images = %v, %v", imgs, err)
	}
	t.Logf("%s %q renders to %s", node, nodes[0].Name, imgs[node])
}
