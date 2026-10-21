package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/verify"
)

// TestLocalConfigIgnoredInCI (Bug #640, criteria 2 and 3): in CI
// (GITHUB_ACTIONS=true) a committed .aayudha/config.local.* is ignored with a
// notice naming it, so it cannot clear an environment's production flag and
// verify still refuses the target; outside CI the local layer applies.
func TestLocalConfigIgnoredInCI(t *testing.T) {
	for _, local := range []struct{ name, body string }{
		{"config.local.toml", "version = 1\n[verify.environments.staging]\nproduction = false\n"},
		{"config.local.json", `{"version": 1, "verify": {"environments": {"staging": {"production": false}}}}`},
	} {
		root := t.TempDir()
		for name, body := range map[string]string{
			".git/HEAD":              "ref: refs/heads/dev\n",
			".aayudha/config.toml":   "version = 1\n[verify.environments.staging]\nurl = \"https://staging.example.com\"\nproduction = true\n",
			".aayudha/" + local.name: local.body,
		} {
			p := filepath.Join(root, name)
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		file := filepath.Join(root, ".aayudha", local.name)
		for _, ci := range []bool{true, false} {
			env := map[string]string{"AAYU_ENV": "staging"}
			if ci {
				env["GITHUB_ACTIONS"] = "true"
			}
			getenv := func(k string) string { return env[k] }
			l, err := config.Load(config.DefaultSources(getenv, root, nil, nil))
			if err != nil {
				t.Fatalf("%s, CI=%v: %v", local.name, ci, err)
			}
			warned := len(l.Notices) == 1 && strings.Contains(l.Notices[0], file+" ignored")
			_, err = verify.ResolveTarget(&verify.Run{Verify: l.Config.Verify, Env: getenv})
			refused := errors.Is(err, verify.ErrProduction)
			if ci && (!warned || !refused || !l.Config.Verify.Environments["staging"].Production) {
				t.Errorf("%s with GITHUB_ACTIONS=true: notices %q, ResolveTarget %v; want the file ignored with a notice and the target refused", local.name, l.Notices, err)
			}
			if !ci && (warned || refused || l.Config.Verify.Environments["staging"].Production) {
				t.Errorf("%s outside CI: notices %q, ResolveTarget %v; want the local layer applied", local.name, l.Notices, err)
			}
		}
	}
}
