package evals

import (
	"os"
	"path/filepath"
	"testing"

	aayudha "github.com/rahulnakmol/agentic-ayudha"
)

// TestAssessIgnoresLocalConfigInCI (Bug #640): a case's grill assessment
// loads its fixture's config as aayu does, so in CI a committed
// .aayudha/config.local.toml is not read; outside CI it still is.
func TestAssessIgnoresLocalConfigInCI(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		"repo/.aayudha/config.toml":       "version = 1\n",
		"repo/.aayudha/config.local.toml": "not toml = [\n",
	} {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	c := Case{Dir: dir, Repo: "repo"}
	t.Setenv("GITHUB_ACTIONS", "true")
	if _, _, err := c.Assess(aayudha.Assets, "story", "add a login page"); err != nil {
		t.Fatalf("in CI: %v; want the local file skipped", err)
	}
	t.Setenv("GITHUB_ACTIONS", "")
	if _, _, err := c.Assess(aayudha.Assets, "story", "add a login page"); err == nil {
		t.Fatal("outside CI: no error; want the malformed local file read and refused")
	}
}
