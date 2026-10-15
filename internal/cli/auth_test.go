package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/secrets"
)

var update = flag.Bool("update", false, "rewrite golden files")

// noKeychain is the OS keychain on a headless Linux runner: unavailable.
type noKeychain struct{}

func (noKeychain) Name() string               { return "keychain" }
func (noKeychain) Available() error           { return errors.New("no Secret Service on this machine") }
func (noKeychain) Get(string) (string, error) { panic("unavailable backend used") }
func (noKeychain) Set(string, string) error   { panic("unavailable backend used") }
func (noKeychain) Delete(string) error        { panic("unavailable backend used") }
func (noKeychain) Names() ([]string, error)   { panic("unavailable backend used") }

// headlessStore is the store a headless Linux CI job gets: no keychain, then the
// env and age-file backends under the given home.
func headlessStore(env map[string]string, home string) *secrets.Store {
	getenv := func(k string) string {
		if k == "HOME" {
			return home
		}
		return env[k]
	}
	var environ []string
	for k, v := range env {
		environ = append(environ, k+"="+v)
	}
	cfg, data := secrets.Dirs(getenv)
	return secrets.New(noKeychain{}, secrets.Env{Getenv: getenv, Environ: environ},
		secrets.File{Path: filepath.Join(data, "secrets.age"), IdentityPath: filepath.Join(cfg, "age-identity.txt")})
}

func fakeGitHubUser(t *testing.T, login, typ, scopes string, listed bool) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user" || !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			http.Error(w, "unexpected", 400)
			return
		}
		if listed {
			w.Header().Set("X-OAuth-Scopes", scopes)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"login": login, "type": typ})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestAuthStatusGolden(t *testing.T) {
	noGH := func(context.Context) (string, error) { return "", errors.New("gh not installed") }
	cases := []struct {
		name   string
		scopes string
		listed bool
		env    map[string]string
		args   []string
	}{
		{"over-scoped classic token, no bot identity (day one)", "repo, workflow, admin:org, delete_repo", true,
			map[string]string{"AAYU_GITHUB_TOKEN": "ghp_test"}, nil},
		{"fine-grained token, machine-user bot", "", false,
			map[string]string{"AAYU_GITHUB_TOKEN": "github_pat_test"},
			[]string{"--set", "identity.machine_user=aayu-machine", "--set", "identity.machine_user_secret=machine-pat"}},
		{"right-sized classic token in CI without the App token", "repo, read:org", true,
			map[string]string{"GITHUB_TOKEN": "ghs_actions"},
			[]string{"--ci", "--set", "identity.app_id=42", "--set", "identity.app_slug=aayudha-bot"}},
	}
	var got bytes.Buffer
	for _, c := range cases {
		srv := fakeGitHubUser(t, "rahulnakmol", "User", c.scopes, c.listed)
		code, out, errs := run(t, c.env, Options{GitHubAPI: srv.URL, GHToken: noGH, Secrets: headlessStore(c.env, t.TempDir())},
			append([]string{"auth", "status"}, c.args...)...)
		if code != 0 {
			t.Fatalf("%s: exit %d: %s", c.name, code, errs)
		}
		got.WriteString("## " + c.name + "\n" + out + "\n")
	}
	golden := "testdata/auth-status.golden"
	if *update {
		_ = os.MkdirAll("testdata", 0o755)
		_ = os.WriteFile(golden, got.Bytes(), 0o644)
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != string(want) {
		t.Fatalf("auth status output differs from %s (run go test -update):\n%s", golden, got.String())
	}
}

// TestSecretCanary runs every secret-handling command with a canary value and
// then scans home, the repo, the temp directory, and the command output for it.
// The keychain is unavailable, as on a headless Linux runner, so the env and
// age-file backends do the work.
func TestSecretCanary(t *testing.T) {
	const canary = "CANARY-7d1e0b5c-never-on-disk"
	home, repo, tmp := t.TempDir(), t.TempDir(), t.TempDir()
	t.Setenv("TMPDIR", tmp)
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	srv := fakeGitHubUser(t, "rahulnakmol", "User", "repo", true)
	noGH := func(context.Context) (string, error) { return "", errors.New("gh not installed") }

	envCanary := map[string]string{"HOME": home, "AAYU_SECRET_POSTHOG": canary + "-env", "AAYU_GITHUB_TOKEN": "ghp_x"}
	steps := []struct {
		env   map[string]string
		stdin string
		args  []string
		want  int
	}{
		{map[string]string{"HOME": home}, canary + "\n", []string{"auth", "set-secret", "ai-gateway"}, 0},
		{map[string]string{"HOME": home}, canary + "-second\n", []string{"auth", "set-secret", "github-token"}, 0},
		{map[string]string{"HOME": home}, "", []string{"auth", "list-secrets", "--set", "integrations.posthog.secret=posthog", "--set", "integrations.posthog.scope=read"}, 0},
		{envCanary, "", []string{"auth", "list-secrets", "--json"}, 0},
		{map[string]string{"HOME": home}, "", []string{"auth", "status", "--verbose"}, 0}, // human token from the age file
		{envCanary, "", []string{"auth", "status", "--json"}, 0},
		{map[string]string{"HOME": home}, "", []string{"auth", "delete-secret", "ai-gateway"}, 0},
		{map[string]string{"HOME": home, "AAYU_AGENT": "1"}, canary + "\n", []string{"auth", "set-secret", "x"}, ExitBlocked},
	}
	for _, s := range steps {
		var out, errb bytes.Buffer
		getenv := func(k string) string { return s.env[k] }
		var environ []string
		for k, v := range s.env {
			environ = append(environ, k+"="+v)
		}
		code := Run(context.Background(), s.args, IO{In: strings.NewReader(s.stdin), Out: &out, Err: &errb},
			Options{Getenv: getenv, Environ: environ, Dir: repo, GitHubAPI: srv.URL, GHToken: noGH, Secrets: headlessStore(s.env, home)})
		if code != s.want {
			t.Fatalf("aayu %s: exit %d, want %d: %s", strings.Join(s.args, " "), code, s.want, errb.String())
		}
		if strings.Contains(out.String()+errb.String(), "CANARY") {
			t.Fatalf("aayu %s printed a secret:\n%s%s", strings.Join(s.args, " "), out.String(), errb.String())
		}
		for _, dir := range []string{home, repo, tmp} {
			_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return err
				}
				b, _ := os.ReadFile(p)
				if bytes.Contains(b, []byte("CANARY")) {
					t.Fatalf("after aayu %s: canary in %s", strings.Join(s.args, " "), p)
				}
				return nil
			})
		}
	}
	// The values did round-trip through the encrypted file, with private permissions.
	st := headlessStore(map[string]string{}, home)
	if v, backend, err := st.Get("github-token"); err != nil || v != canary+"-second" || backend != "file" {
		t.Fatalf("github-token = %q from %s, %v", v, backend, err)
	}
	if _, _, err := st.Get("ai-gateway"); !errors.Is(err, secrets.ErrNotFound) {
		t.Fatalf("deleted secret still resolves: %v", err)
	}
	cfg, data := secrets.Dirs(func(k string) string { return map[string]string{"HOME": home}[k] })
	for _, p := range []string{filepath.Join(data, "secrets.age"), filepath.Join(cfg, "age-identity.txt")} {
		// Windows has no permission bits (Go reports 0666 for any writable file): the files take their
		// directory's ACL, under the user profile the user's, SYSTEM's, and Administrators' (Appendix B, D-0.4f).
		if fi, err := os.Stat(p); err != nil || runtime.GOOS != "windows" && fi.Mode().Perm() != 0o600 {
			t.Fatalf("%s: %v, mode %v; want 0600", p, err, fi.Mode().Perm())
		}
	}
}

func TestIntegrationScope(t *testing.T) {
	for _, tt := range []struct {
		scope     string
		needWrite bool
		refused   bool
	}{{"read", false, false}, {"write", false, true}, {"read", true, true}, {"write", true, false}} {
		err := secrets.CheckScope("posthog", tt.scope, tt.needWrite)
		if (err != nil) != tt.refused {
			t.Errorf("scope %s, write %v: %v", tt.scope, tt.needWrite, err)
		}
		if tt.scope == "write" && !tt.needWrite && !errors.Is(err, secrets.ErrWriteScoped) {
			t.Errorf("write-scoped key for a read verb: %v", err)
		}
	}
}
