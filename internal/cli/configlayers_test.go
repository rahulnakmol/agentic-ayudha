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

// TestNewerLocalConfigIgnoredInCI (Bug #640): CI never reads the local config
// layer, so a committed .aayudha/config.local.toml with a newer schema version
// does not refuse a write verb under --ci or GITHUB_ACTIONS=true; outside CI
// it still does.
func TestNewerLocalConfigIgnoredInCI(t *testing.T) {
	withHandler(t, "verify", func(_ context.Context, e *Env, _ Values) error {
		fmt.Fprintln(e.Out, "ran")
		return nil
	})
	dir := t.TempDir()
	for name, body := range map[string]string{
		".git/HEAD":                  "ref: refs/heads/dev\n",
		".aayudha/config.toml":       "version = 1\n",
		".aayudha/config.local.toml": "version = 99\n",
	} {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, tt := range []struct {
		name string
		env  map[string]string
		args []string
		want int
	}{
		{"--ci", map[string]string{HaltVariable: ""}, []string{"verify", "--ci"}, ExitOK},
		{"GITHUB_ACTIONS=true", map[string]string{HaltVariable: "", "GITHUB_ACTIONS": "true"}, []string{"verify"}, ExitOK},
		{"outside CI", map[string]string{HaltVariable: ""}, []string{"verify"}, ExitUsage},
		// aayu upgrade and doctor plan the schema with the same rule.
		{"upgrade --ci", map[string]string{HaltVariable: ""}, []string{"upgrade", "--ci"}, ExitOK},
		{"upgrade, GITHUB_ACTIONS=true", map[string]string{HaltVariable: "", "GITHUB_ACTIONS": "true"}, []string{"upgrade"}, ExitOK},
		{"upgrade outside CI", map[string]string{HaltVariable: ""}, []string{"upgrade"}, ExitUsage},
	} {
		code, out, errOut := run(t, tt.env, Options{Dir: dir}, tt.args...)
		if code != tt.want {
			t.Errorf("%s: exit %d, stdout %q, stderr %q; want exit %d", tt.name, code, out, errOut, tt.want)
		}
		if tt.want == ExitUsage && !strings.Contains(errOut, "config.local.toml is config schema v99") {
			t.Errorf("%s: stderr %q; want the newer config.local.toml named", tt.name, errOut)
		}
	}
	for args, want := range map[string]string{
		"doctor --ci --json": "repository schema current",
		"doctor --json":      "repository schema is newer than this aayu: .aayudha/config.local.toml is config schema v99",
	} {
		if _, out, errOut := run(t, map[string]string{HaltVariable: ""}, Options{Dir: dir}, strings.Fields(args)...); !strings.Contains(out, want) {
			t.Errorf("%s: stdout %q, stderr %q; want the schema row %q", args, out, errOut, want)
		}
	}
}
