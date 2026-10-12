package doctor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/identity"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations/providers"
	"github.com/rahulnakmol/agentic-ayudha/internal/secrets"
)

var update = flag.Bool("update", false, "rewrite golden files")

type down struct{}

func (down) Name() string               { return "keychain" }
func (down) Available() error           { return errors.New("no Secret Service") }
func (down) Get(string) (string, error) { return "", secrets.ErrNotFound }
func (down) Set(string, string) error   { return nil }
func (down) Delete(string) error        { return nil }
func (down) Names() ([]string, error)   { return nil, nil }

func load(t *testing.T, sets ...string) *config.Loaded {
	t.Helper()
	l, err := config.Load(config.Sources{Sets: sets})
	if err != nil {
		t.Fatal(err)
	}
	return l
}

// repoWith creates a repository root holding the given files.
func repoWith(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, body := range files {
		p := filepath.Join(root, rel)
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// overlay answers from table first and falls back to the probe's Exec; the
// value "ERR" makes a command fail.
func overlay(p *Probe, table map[string]string) {
	base := p.Exec
	p.Exec = func(ctx context.Context, dir, name string, args ...string) (string, error) {
		if out, ok := table[strings.Join(append([]string{name}, args...), " ")]; ok {
			if out == "ERR" {
				return "", fmt.Errorf("%s: not found", name)
			}
			return out, nil
		}
		return base(ctx, dir, name, args...)
	}
}

// fakeExec answers commands from a table keyed by "name args...".
func fakeExec(table map[string]string) func(context.Context, string, string, ...string) (string, error) {
	return func(_ context.Context, _ string, name string, args ...string) (string, error) {
		out, ok := table[strings.Join(append([]string{name}, args...), " ")]
		if !ok {
			return "", fmt.Errorf("%s: not found", name)
		}
		return out, nil
	}
}

func fakeAPI(responses map[string]any) func(context.Context, string, any) error {
	return func(_ context.Context, path string, out any) error {
		v, ok := responses[path]
		if !ok {
			return errors.New("404 Not Found")
		}
		b, _ := json.Marshal(v)
		return json.Unmarshal(b, out)
	}
}

// healthy is a probe on which every check passes.
func healthy(t *testing.T) *Probe {
	env := map[string]string{"AMP_THREAD_ID": "T-1", "AAYU_SECRET_POSTHOG": "phc"}
	getenv := func(k string) string { return env[k] }
	p := &Probe{
		Root: repoWith(t, map[string]string{".aayudha/config.toml": "version = 1\n",
			".aayudha/lock.toml": "version = 1\nskills_version = \"0.0.0\"\nhosts = [\"claude\"]\n[skills]\n",
			".mcp.json":          `{"mcpServers": {"aayudha": {"command": "/usr/bin/aayu", "args": ["mcp"]}}}`}),
		Getenv:   getenv,
		LookPath: func(string) (string, error) { return "/usr/bin/x", nil },
		Exec: fakeExec(map[string]string{
			"git --version":                 "git version 2.45.1\n",
			"/usr/bin/aayu version":         "aayu 1.2.0\n",
			"git lfs version":               "git-lfs/3.5.1 (GitHub; linux amd64; go 1.22)\n",
			"git worktree list --porcelain": "worktree /repo\nHEAD abc\nbranch refs/heads/dev\n",
			"git for-each-ref --format=%(refname:short) refs/heads refs/remotes/origin": "dev\nmain\norigin/HEAD\norigin/dev\n",
			"git symbolic-ref --short refs/remotes/origin/HEAD":                         "origin/dev\n",
		}),
		Config:  load(t, "identity.app_id=42", "identity.app_slug=aayudha-bot", "integrations.posthog.secret=posthog", "integrations.posthog.scope=read", "integrations.posthog.project=12345"),
		Secrets: secrets.New(down{}, secrets.Env{Getenv: getenv}, secrets.File{Path: "/tmp/s.age", IdentityPath: "/tmp/id"}),
		Human: func(context.Context) (identity.Token, error) {
			return identity.Token{Value: "t", Source: "AAYU_GITHUB_TOKEN"}, nil
		},
		WhoAmI: func(context.Context, identity.Token) (identity.User, error) {
			return identity.User{Login: "rahulnakmol", Type: "User", Scopes: []string{"read:org", "repo"}, Listed: true}, nil
		},
		API: fakeAPI(map[string]any{
			"/repos/o/r":          map[string]any{"owner": map[string]any{"type": "Organization"}},
			"/repos/o/r/rulesets": []any{},
		}),
		Repo:      "o/r",
		Version:   "1.2.0",
		Latest:    func(context.Context) (string, error) { return "v1.2.0", nil },
		TracePath: ".aayudha/telemetry/traces.jsonl",
	}
	p.Integration = realIntegrations(p, providerAPI{
		"https://us.posthog.com/api/personal_api_keys/@current/": `{"label":"aayudha","scopes":["query:read"]}`,
	})
	return p
}

// providerAPI answers integration providers' GETs by URL; a missing URL is a
// 401, as a provider answers an unknown key.
type providerAPI map[string]string

func (a providerAPI) RoundTrip(req *http.Request) (*http.Response, error) {
	body, ok := a[req.URL.String()]
	status := http.StatusOK
	if !ok {
		status, body = http.StatusUnauthorized, `{"detail":"Invalid token"}`
	}
	return &http.Response{StatusCode: status, Status: fmt.Sprint(status), Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
}

// realIntegrations checks integrations with the real adapters against api.
func realIntegrations(p *Probe, api providerAPI) func(context.Context, string) (integrations.Scope, error) {
	return func(ctx context.Context, name string) (integrations.Scope, error) {
		d := providers.Deps{Secrets: p.Secrets, Transport: api, Repo: p.Repo}
		if p.Config != nil {
			d.Config = &p.Config.Config
		}
		return providers.Check(ctx, d, name)
	}
}

// failing returns, per check, a change to a healthy probe that makes it fail or warn.
func failing(t *testing.T) map[string]func(*Probe) {
	return map[string]func(*Probe){
		"binary":  func(p *Probe) { p.Version = "1.1.0" },
		"git":     func(p *Probe) { overlay(p, map[string]string{"git --version": "git version 2.39.5\n"}) },
		"git-lfs": func(p *Probe) { overlay(p, map[string]string{"git lfs version": "ERR"}) },
		"gh": func(p *Probe) {
			p.LookPath = func(n string) (string, error) {
				if n == "gh" {
					return "", errors.New("not found")
				}
				return "/usr/bin/" + n, nil
			}
		},
		"token": func(p *Probe) {
			p.WhoAmI = func(context.Context, identity.Token) (identity.User, error) {
				return identity.User{Login: "rahulnakmol", Scopes: []string{"admin:org", "delete_repo", "repo", "workflow"}, Listed: true}, nil
			}
		},
		"bot-identity": func(p *Probe) { p.Config = load(t) },
		"config": func(p *Probe) {
			p.Config, p.ConfigErr = nil, errors.New("config: .aayudha/config.toml: '' has invalid keys: colour")
		},
		"schema": func(p *Probe) { p.Root = repoWith(t, map[string]string{".aayudha/config.toml": "version = 2\n"}) },
		"host":   func(p *Probe) { p.Getenv = func(string) string { return "" } },
		"secrets": func(p *Probe) {
			p.Secrets = secrets.New(down{}, secrets.Env{Getenv: func(string) string { return "" }})
		},
		"issue-types": func(p *Probe) { p.API = nil },
		// Each state a configured integration can be in: a write-scoped key
		// (refused, not sent), an unreachable provider, an unset secret, and an
		// integration this binary has no adapter for.
		"integrations": func(p *Probe) {
			p.Config = load(t, "integrations.posthog.secret=posthog", "integrations.posthog.scope=write", "integrations.posthog.project=12345",
				"integrations.sentry.secret=posthog", "integrations.sentry.scope=read", "integrations.sentry.org=acme", "integrations.sentry.project=4507",
				"integrations.github.secret=github-read", "integrations.github.scope=read",
				"integrations.amplitude.secret=amplitude", "integrations.amplitude.scope=read")
			p.Integration = realIntegrations(p, providerAPI{})
		},
		"optional-tools": func(p *Probe) {
			p.LookPath = func(n string) (string, error) {
				if n == "node" || n == "semgrep" {
					return "/usr/bin/" + n, nil
				}
				return "", errors.New("not found")
			}
		},
		"worktrees": func(p *Probe) {
			overlay(p, map[string]string{"git worktree list --porcelain": "worktree /repo\nHEAD a\n\nworktree /tmp/wt-x\nHEAD b\nprunable gitdir file points to non-existent location\n"})
		},
		"coexistence": func(p *Probe) {
			p.Root = repoWith(t, map[string]string{
				".github/workflows/ci.yml":            "name: ci\non: push\njobs:\n  build:\n    runs-on: ubuntu-latest\n  test:\n    name: aayudha/test\n    runs-on: ubuntu-latest\n",
				".github/workflows/aayudha-token.yml": "name: x\njobs:\n  mint:\n    runs-on: ubuntu-latest\n",
			})
			overlay(p, map[string]string{
				"git for-each-ref --format=%(refname:short) refs/heads refs/remotes/origin": "master\norigin/master\n",
				"git symbolic-ref --short refs/remotes/origin/HEAD":                         "origin/master\n",
			})
			p.API = fakeAPI(map[string]any{"/repos/o/r/rulesets": []any{map[string]any{"name": "protect-master"}}})
		},
		"telemetry": func(p *Probe) { p.Config = load(t, "telemetry.disabled=true") },
		"mcp": func(p *Probe) {
			overlay(p, map[string]string{"/usr/bin/aayu version": "aayu 1.1.0\n"})
		},
		"ledger-media": func(p *Probe) {
			overlay(p, map[string]string{"git ls-tree --name-only origin/ledger": ".gitattributes\n.lfsconfig\nREADME.md\n"})
		},
		"skills-copies": func(p *Probe) {
			p.Root = repoWith(t, map[string]string{".aayudha/config.toml": "version = 1\n",
				".aayudha/lock.toml":             "version = 1\nhosts = [\"claude\"]\n[skills]\nready = { version = \"0.0.0\", sha256 = \"x\" }\nstatus = { version = \"0.0.0\", sha256 = \"x\" }\n",
				".claude/skills/status/SKILL.md": "edited\n"})
		},
	}
}

// TestChecksGolden renders every check on a healthy probe and on its failing
// fixture and compares the output with testdata/checks.golden.
func TestChecksGolden(t *testing.T) {
	ctx := context.Background()
	fails := failing(t)
	var got bytes.Buffer
	for _, c := range Checks {
		for _, mode := range []string{"pass", "fail"} {
			p := healthy(t)
			if mode == "fail" {
				f, ok := fails[c.ID]
				if !ok {
					continue // not-yet-available checks have one state
				}
				f(p)
			}
			r := c.Run(ctx, p)
			r.ID = c.ID
			if mode == "pass" && r.Status != Pass && r.Status != NA {
				t.Errorf("%s on a healthy probe: %s %s", c.ID, r.Status, r.Detail)
			}
			if mode == "fail" && c.ID != "telemetry" && r.Status == Pass {
				t.Errorf("%s on its failing fixture passed: %s", c.ID, r.Detail)
			}
			fmt.Fprintf(&got, "## %s %s\n", c.ID, mode)
			// Machine-specific paths are replaced so the golden file is portable.
			var buf bytes.Buffer
			_ = Render(&buf, []Result{r})
			got.WriteString(strings.ReplaceAll(buf.String(), p.Root, "<root>"))
		}
	}
	golden := filepath.Join("testdata", "checks.golden")
	if *update {
		_ = os.MkdirAll("testdata", 0o755)
		_ = os.WriteFile(golden, got.Bytes(), 0o644)
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != string(want) {
		t.Fatalf("doctor checks differ from %s (go test ./internal/doctor -update):\n%s", golden, got.String())
	}
}

func TestBlockingOnlyOnFloorFailures(t *testing.T) {
	ctx := context.Background()
	p := healthy(t)
	if rs := Run(ctx, p); Blocking(rs) {
		t.Fatalf("healthy probe blocks: %+v", rs)
	}
	for id, f := range failing(t) {
		p := healthy(t)
		f(p)
		want := id == "git" || id == "git-lfs" || id == "config" || id == "schema" || id == "ledger-media"
		if got := Blocking(Run(ctx, p)); got != want {
			t.Errorf("%s failing: blocking = %v, want %v", id, got, want)
		}
	}
}

func TestFleet(t *testing.T) {
	api := fakeAPI(map[string]any{
		"/repos/o/a":                            map[string]any{"visibility": "private", "default_branch": "master", "owner": map[string]any{"type": "User"}},
		"/repos/o/a/contents/.github/workflows": []any{map[string]any{"name": "ci.yml"}},
		"/repos/o/a/branches?per_page=100":      []any{map[string]any{"name": "master"}},
		"/repos/o/a/rulesets":                   []any{},
	})
	rows := Fleet(context.Background(), []string{"o/a", "o/gone"}, api)
	if len(rows) != 2 || rows[0].Status != Pass || !strings.Contains(rows[0].Detail, "1 workflow(s); default master, no dev") || rows[1].Status != Fail {
		t.Fatalf("%+v", rows)
	}
}
