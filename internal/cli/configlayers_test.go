package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/verify"
)

// TestLocalConfigIgnoredInCI (Bug #640, criteria 2 and 3): through the CLI,
// --ci or GITHUB_ACTIONS=true ignores a committed .aayudha/config.local.*
// with a warning naming it, so the environment stays production and verify
// refuses it; without either the local layer applies.
func TestLocalConfigIgnoredInCI(t *testing.T) {
	withHandler(t, "config", func(_ context.Context, e *Env, _ Values) error {
		l, err := e.Config()
		if err != nil {
			return err
		}
		_, err = verify.ResolveTarget(&verify.Run{Verify: l.Config.Verify, Env: e.Getenv})
		fmt.Fprintf(e.Out, "production=%v refused=%v\n", l.Config.Verify.Environments["staging"].Production, errors.Is(err, verify.ErrProduction))
		return nil
	})
	for _, local := range []struct{ name, body string }{
		{"config.local.toml", "version = 1\n[verify.environments.staging]\nproduction = false\n"},
		{"config.local.json", `{"version": 1, "verify": {"environments": {"staging": {"production": false}}}}`},
	} {
		dir := t.TempDir()
		for name, body := range map[string]string{
			".git/HEAD":              "ref: refs/heads/dev\n",
			".aayudha/config.toml":   "version = 1\n[verify.environments.staging]\nurl = \"https://staging.example.com\"\nproduction = true\n",
			".aayudha/" + local.name: local.body,
		} {
			p := filepath.Join(dir, name)
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		warning := "aayu: " + filepath.Join(dir, ".aayudha", local.name) + " ignored: in CI"
		for _, tt := range []struct {
			name string
			env  map[string]string
			args []string
			ci   bool
		}{
			{"--ci", map[string]string{"AAYU_ENV": "staging"}, []string{"config", "show", "--ci"}, true},
			{"GITHUB_ACTIONS=true", map[string]string{"AAYU_ENV": "staging", "GITHUB_ACTIONS": "true"}, []string{"config", "show"}, true},
			{"outside CI", map[string]string{"AAYU_ENV": "staging"}, []string{"config", "show"}, false},
		} {
			code, out, errOut := run(t, tt.env, Options{Dir: dir}, tt.args...)
			if code != ExitOK {
				t.Fatalf("%s, %s: exit %d: %s", local.name, tt.name, code, errOut)
			}
			warned := strings.Contains(errOut, warning)
			if tt.ci && (!warned || !strings.Contains(out, "production=true refused=true")) {
				t.Errorf("%s, %s: stdout %q, stderr %q; want the file ignored with a warning and the target refused", local.name, tt.name, out, errOut)
			}
			if !tt.ci && (warned || !strings.Contains(out, "production=false refused=false")) {
				t.Errorf("%s, %s: stdout %q, stderr %q; want the local layer applied", local.name, tt.name, out, errOut)
			}
		}
	}
}
