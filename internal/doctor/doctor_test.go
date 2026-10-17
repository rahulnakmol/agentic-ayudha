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
	"slices"
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/agentenv"
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

// staleEnv is a repository whose agent environment was generated and
// verified for go 1.25.0, after which go.mod moved to 1.26.0.
func staleEnv(t *testing.T) string {
	root := repoWith(t, map[string]string{"go.mod": "module m\n\ngo 1.25.0\n"})
	d, err := agentenv.Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	s, err := agentenv.Settle(d, map[string]agentenv.Answer{"hosts": {Value: "amp,cursor", Source: "stated"}})
	if err != nil {
		t.Fatal(err)
	}
	s.Tools[0].Version, s.Tools[0].SHA256 = "1.25.8", map[string]string{"amd64": strings.Repeat("a", 64), "arm64": strings.Repeat("b", 64)}
	if err := agentenv.Write(root, s); err != nil {
		t.Fatal(err)
	}
	if err := agentenv.RecordContainer(root, agentenv.Amp, s, agentenv.Result{Image: "debian:12"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module m\n\ngo 1.26.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
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
		Config:  load(t, "identity.app_id=42", "identity.app_slug=aayudha-bot", "integrations.posthog.secret=posthog", "integrations.posthog.scope=read", "integrations.posthog.project=12345", "inspect.apps=codex"),
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
			"/repos/o/r/issues?state=open&labels=aayudha:leased&per_page=100": []any{map[string]any{"number": 9}},
			"/repos/o/r/issues/9/comments?per_page=100":                       []any{map[string]any{"body": "<!-- aayudha:lease holder=bot@ci expires=2099-01-01T00:00:00Z -->"}},
			"/repos/o/r/issues?state=open&labels=type:feedback&per_page=100": []any{map[string]any{"number": 5, "title": "Feedback: a",
				"labels": []any{map[string]any{"name": "type:feedback"}, map[string]any{"name": "fix:check"}}}},
			"/orgs/o/installations?per_page=100": map[string]any{"installations": []any{map[string]any{"app_slug": "chatgpt-codex-connector"}}},
		}),
		Repo: "o/r",
		Combined: fakeExec(map[string]string{
			"/usr/bin/x --version":          "1.0.0\n",
			"/usr/bin/x login status":       "Logged in using ChatGPT\n",
			"/usr/bin/x status":             "Logged in as dev@example.com\n",
			"/usr/bin/x auth status --json": `{"loggedIn": true, "authMethod": "claude.ai"}`,
			"/usr/bin/x account list":       "* dev@example.com\n",
		}),
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
		"leases": func(p *Probe) {
			p.API = fakeAPI(map[string]any{
				"/repos/o/r/issues?state=open&labels=aayudha:leased&per_page=100": []any{map[string]any{"number": 9}, map[string]any{"number": 10}},
				"/repos/o/r/issues/9/comments?per_page=100":                       []any{map[string]any{"body": "<!-- aayudha:lease holder=bot@ci expires=2026-01-01T00:00:00Z -->"}},
				"/repos/o/r/issues/10/comments?per_page=100":                      []any{},
			})
		},
		// An unclassified Feedback issue is listed; a classified one and a pull
		// request carrying the label are not (D-7.7e).
		"feedback-layers": func(p *Probe) {
			p.API = fakeAPI(map[string]any{"/repos/o/r/issues?state=open&labels=type:feedback&per_page=100": []any{
				map[string]any{"number": 5, "title": "Feedback: a", "labels": []any{map[string]any{"name": "type:feedback"}, map[string]any{"name": "fix:check"}}},
				map[string]any{"number": 6, "title": "Feedback: b", "labels": []any{map[string]any{"name": "type:feedback"}, map[string]any{"name": "fix:unknown"}}},
				map[string]any{"number": 7, "title": "A PR", "labels": []any{map[string]any{"name": "type:feedback"}}, "pull_request": map[string]any{}},
				map[string]any{"number": 8, "title": "Feedback: c", "labels": []any{map[string]any{"name": "type:feedback"}, map[string]any{"name": "needs:grill"}}},
			}})
		},
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
		"agent-env": func(p *Probe) {
			p.Root = staleEnv(t)
		},
		// Codex replied on a PR, but config does not name it; Cursor's family is unknown.
		"reviewer-apps": func(p *Probe) {
			p.Config = load(t)
			p.API = fakeAPI(map[string]any{
				"/repos/o/r/pulls?state=all&per_page=10":         []any{map[string]any{"number": 3, "head": map[string]any{"sha": "abc"}}},
				"/repos/o/r/issues/3/comments?per_page=100":      []any{map[string]any{"user": map[string]any{"login": "chatgpt-codex-connector[bot]"}}},
				"/repos/o/r/pulls/3/reviews?per_page=100":        []any{},
				"/repos/o/r/commits/abc/check-runs?per_page=100": map[string]any{"check_runs": []any{map[string]any{"name": "Cursor Bugbot", "app": map[string]any{"slug": "cursor"}}}},
			})
		},
		// A 0.1 triage workflow, still installed; a product's own file of the same name would not count.
		"retired-workflows": func(p *Probe) {
			p.Root = repoWith(t, map[string]string{".github/workflows/aayudha-triage.yml": "name: aayudha-triage\non:\n  issues:\njobs: {}\n"})
		},
		// S-7.1d, S-7.1e, D-7.1f: the digest needs no agent (Actions runs it), no tool runs the review (missing),
		// and triage is recorded in amp and codex (double).
		"loops": func(p *Probe) {
			p.Root = repoWith(t, map[string]string{
				"loops/digest.toml": "description = \"Founder digest.\"\ncommand = [\"status\", \"--digest\"]\ncron = \"5 9 * * 1-5\"\nagent = false\n",
				"loops/review.toml": "description = \"Review.\"\ncommand = [\"seat\", \"list\"]\ncron = \"0 11 1 * *\"\n",
				"loops/triage.toml": "description = \"Triage.\"\ncommand = [\"triage\"]\ncron = \"0 9 * * *\"\nhosts = [\"amp\", \"codex\"]\n",
				"loops/active.json": `{"amp": {"triage": {"digest": "000000000000", "id": "s-1", "at": "2026-10-16"}}, "codex": {"triage": {"digest": "000000000000", "id": "t-1", "at": "2026-10-16"}}}`,
			})
			p.LookPath = func(b string) (string, error) {
				if b == "codex" {
					return "/usr/bin/codex", nil
				}
				return "", os.ErrNotExist
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
		// D-3.2b.a: the enabled Codex plugin's hooks were never trusted in /hooks.
		"codex-hooks": func(p *Probe) {
			home := t.TempDir()
			cache := filepath.Join(home, ".codex", "plugins", "cache", "aayudha", "aayudha", "0.0.0", "hosts", "codex")
			_ = os.MkdirAll(cache, 0o755)
			_ = os.WriteFile(filepath.Join(cache, "hooks.json"), []byte(`{"hooks": {"PreToolUse": [{"matcher": "*", "hooks": [{"type": "command", "command": "aayu hook pre-tool --host codex --skills 0.0.0"}]}]}}`), 0o644)
			_ = os.WriteFile(filepath.Join(home, ".codex", "config.toml"), []byte("[plugins.\"aayudha@aayudha\"]\nenabled = true\n"), 0o644)
			getenv := p.Getenv
			p.Getenv = func(k string) string {
				if k == "HOME" {
					return home
				}
				return getenv(k)
			}
		},
		// No hooks file, and aayu's token is Copilot's own: aayu would write
		// as Copilot's account.
		"copilot": func(p *Probe) {
			p.Root = repoWith(t, map[string]string{".aayudha/lock.toml": "version = 1\nskills_version = \"0.0.0\"\nhosts = [\"copilot\"]\n[skills]\n"})
			env := map[string]string{"COPILOT_GITHUB_TOKEN": "t", "HOME": t.TempDir()}
			p.Getenv = func(k string) string { return env[k] }
			p.LookPath = func(string) (string, error) { return "", errors.New("not found") }
		},
		"mcp": func(p *Probe) {
			overlay(p, map[string]string{"/usr/bin/aayu version": "aayu 1.1.0\n"})
		},
		"proof": func(p *Probe) {
			p.Root = repoWith(t, map[string]string{".aayudha/config.toml": "version = 1\n", "loops/proof-purge.toml": "command = [\"proof\", \"purge\"]\n",
				".github/workflows/aayudha-merge-gate.yml": "jobs:\n  pack:\n    steps:\n      - run: aayudha proof push \"$STORY\" --ci\n"})
		},
		"ledger-rulesets": func(p *Probe) {
			p.Root = repoWith(t, map[string]string{".aayudha/state.json": `{"schema_version": 1, "rulesets_unapplied": "GitHub answered 403: Upgrade to GitHub Pro or make this repository public to enable this feature."}`})
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

// Story #91: the optional-tools check names each missing tool with what it
// serves, and says that a floor without its tool blocks verify; a missing
// tool that no floor needs does not claim to block anything.
func TestOptionalToolsNamesFloors(t *testing.T) {
	probe := func(missing ...string) *Probe {
		p := healthy(t)
		p.LookPath = func(n string) (string, error) {
			if slices.Contains(missing, n) {
				return "", errors.New("not found")
			}
			return "/usr/bin/" + n, nil
		}
		return p
	}
	r := checkOptionalTools(context.Background(), probe("semgrep", "docker"))
	want := "2 optional tool(s) missing: semgrep (sast floor), docker (dast floor: ZAP); a floor whose tool is missing blocks verify, it never passes"
	if r.Status != Warn || r.Detail != want {
		t.Errorf("semgrep and docker missing: %s %q, want warn %q", r.Status, r.Detail, want)
	}
	r = checkOptionalTools(context.Background(), probe("tesseract"))
	if r.Status != Warn || strings.Contains(r.Detail, "blocks verify") || !strings.Contains(r.Detail, "tesseract (OCR in the redaction pass") {
		t.Errorf("only tesseract missing: %s %q", r.Status, r.Detail)
	}
	if r = checkOptionalTools(context.Background(), probe()); r.Status != Pass || r.Detail != "all optional tools found" {
		t.Errorf("every tool found: %s %q", r.Status, r.Detail)
	}
}

// The states a check reports beside the pass and fail of the golden file:
// what doctor tells a user offline, outside a repository, with a machine-user
// bot, a personal repository, an older schema, an unreadable lease, or a
// ledger that keeps proof media in Git LFS.
func TestCheckStates(t *testing.T) {
	lfsLedger := map[string]string{"git ls-tree --name-only origin/ledger": ".gitattributes\n.lfsconfig\n"}
	tests := []struct {
		name   string
		check  func(context.Context, *Probe) Result
		change func(*Probe)
		status Status
		detail string
	}{
		{"offline", checkBinary, func(p *Probe) { p.Latest = nil }, Warn, "could not check for a newer release (offline)"},
		{"no git", checkGit, func(p *Probe) { overlay(p, map[string]string{"git --version": "ERR"}) }, Fail, "git not found; git >= 2.40 is required"},
		{"unreadable git version", checkGit, func(p *Probe) { overlay(p, map[string]string{"git --version": "hub version 2.14\n"}) }, Fail, `cannot read the git version from "hub version 2.14"`},
		{"token refused", checkToken, func(p *Probe) {
			p.WhoAmI = func(context.Context, identity.Token) (identity.User, error) {
				return identity.User{}, errors.New("401 Bad credentials")
			}
		}, Warn, "token from AAYU_GITHUB_TOKEN did not authenticate: 401 Bad credentials"},
		{"machine-user bot", checkBot, func(p *Probe) {
			p.Config = load(t, "identity.machine_user=aayu-machine", "identity.machine_user_secret=machine")
		}, Pass, "bot identity: machine user aayu-machine"},
		{"older schema", checkSchema, func(p *Probe) { p.Root = repoWith(t, map[string]string{".aayudha/config.toml": "version = 0\n"}) }, Warn, "repository files are at an older schema; run aayu upgrade"},
		{"host from --host", checkHost, func(p *Probe) { p.Host = "cursor" }, Pass, "host cursor (from --host)"},
		{"personal repository", checkIssueTypes, func(p *Probe) {
			p.API = fakeAPI(map[string]any{"/repos/o/r": map[string]any{"owner": map[string]any{"type": "User"}}})
		}, Pass, "personal repository: issue types are organization-only"},
		{"worktrees not listed", checkWorktrees, func(p *Probe) { overlay(p, map[string]string{"git worktree list --porcelain": "ERR"}) }, Warn, "could not list worktrees: git: not found"},
		{"lease comment unreadable", checkLeases, func(p *Probe) {
			p.API = fakeAPI(map[string]any{"/repos/o/r/issues?state=open&labels=aayudha:leased&per_page=100": []any{map[string]any{"number": 9}}})
		}, Warn, "could not read #9's lease: 404 Not Found"},
		{"OTLP opted in", checkTelemetry, func(p *Probe) { p.Config = load(t, "telemetry.otlp_endpoint=https://otel.example.com") }, Pass, "and, opted in, OTLP https://otel.example.com"},
		{"ledger without LFS", checkLedgerMedia, func(p *Probe) { overlay(p, map[string]string{"git ls-tree --name-only origin/ledger": "README.md\n"}) }, Pass, "ledger stores no proof media in Git LFS"},
		{"LFS ledger, no token", checkLedgerMedia, func(p *Probe) { overlay(p, lfsLedger); p.API = nil }, Warn, "repository visibility unknown (no GitHub token or repository)"},
		{"LFS ledger, repository unreadable", checkLedgerMedia, func(p *Probe) { overlay(p, lfsLedger); p.API = fakeAPI(nil) }, Warn, "repository visibility unknown: 404 Not Found"},
		{"LFS ledger, private repository", checkLedgerMedia, func(p *Probe) {
			overlay(p, lfsLedger)
			p.API = fakeAPI(map[string]any{"/repos/o/r": map[string]any{"private": true}})
		}, Pass, "private repository: ledger proof media in Git LFS"},
		// D-3.2b.m: Codex reads the project's server only once the person
		// trusts the project; until then the plugin's server is the one it
		// lists. That is the person's setup step, not a doubled server.
		{"untrusted Codex project", checkMCP, func(p *Probe) {
			p.Root = repoWith(t, map[string]string{".aayudha/config.toml": "version = 1\n",
				".aayudha/lock.toml": "version = 1\nskills_version = \"0.0.0\"\nhosts = [\"codex\"]\n[skills]\n",
				".codex/config.toml": "[mcp_servers.aayudha]\ncommand = \"/usr/bin/aayu\"\nargs = [\"mcp\"]\n"})
			home := t.TempDir()
			_ = os.MkdirAll(filepath.Join(home, ".codex"), 0o755)
			_ = os.WriteFile(filepath.Join(home, ".codex", "config.toml"), []byte("[plugins.\"aayudha@aayudha\"]\nenabled = true\n"), 0o644)
			getenv := p.Getenv
			p.Getenv = func(k string) string {
				if k == "HOME" {
					return home
				}
				return getenv(k)
			}
		}, Warn, "trust this project in Codex"},
		// Without aayu install, an enabled aayudha plugin still gives a host
		// the server, and with neither no host has it: doctor said "no MCP
		// registrations" and passed in both cases.
		{"Codex plugin, no install", checkMCP, func(p *Probe) {
			p.Root = repoWith(t, map[string]string{".aayudha/config.toml": "version = 1\n"})
			home := t.TempDir()
			_ = os.MkdirAll(filepath.Join(home, ".codex"), 0o755)
			_ = os.WriteFile(filepath.Join(home, ".codex", "config.toml"), []byte("[plugins.\"aayudha@aayudha\"]\nenabled = true\n"), 0o644)
			getenv := p.Getenv
			p.Getenv = func(k string) string {
				if k == "HOME" {
					return home
				}
				return getenv(k)
			}
		}, Pass, "aayudha MCP server: codex from the aayudha plugin"},
		{"no install, no plugin", checkMCP, func(p *Probe) {
			p.Root = repoWith(t, map[string]string{".aayudha/config.toml": "version = 1\n"})
		}, Warn, "no host has the aayudha MCP server"},
	}
	for _, tt := range tests {
		p := healthy(t)
		tt.change(p)
		if r := tt.check(context.Background(), p); r.Status != tt.status || !strings.Contains(r.Detail, tt.detail) {
			t.Errorf("%s: %s %q, want %s %q", tt.name, r.Status, r.Detail, tt.status, tt.detail)
		}
	}

	// Outside a repository the repository checks have nothing to look at.
	p := healthy(t)
	p.Root = ""
	for _, c := range []func(context.Context, *Probe) Result{checkSchema, checkWorktrees, checkSkillCopies, checkMCP, checkLedgerMedia} {
		if r := c(context.Background(), p); r.Status != Pass || r.Detail != "not in a repository" {
			t.Errorf("outside a repository: %s %q", r.Status, r.Detail)
		}
	}
}
