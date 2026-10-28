package agentenv

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

func repo(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, body := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// A Go module with a pnpm workspace, a Playwright package, CI apt installs,
// and a hand-written setup that pins a newer Go than go.mod's floor: what
// this repository looks like.
var goNode = map[string]string{
	"go.mod":                    "module example.com/m\n\ngo 1.25.0\n\nrequire github.com/jackc/pgx/v5 v5.7.0\n",
	"go.sum":                    "",
	"package.json":              `{"name":"root","private":true,"packageManager":"pnpm@12.0.0+sha512.abc","engines":{"node":"^22.12.0 || ^24.0.0 || >=26.0.0"}}`,
	"pnpm-lock.yaml":            "lockfileVersion: '9.0'\n",
	"pnpm-workspace.yaml":       "packages:\n  - packages/*\n\nallowBuilds:\n  esbuild: false\n",
	"packages/ui/package.json":  `{"name":"@x/ui","scripts":{"test":"vitest run"},"devDependencies":{"playwright":"^1.64.0"}}`,
	"packages/lib/package.json": `{"name":"@x/lib"}`,
	".github/workflows/ci.yml":  "jobs:\n  t:\n    steps:\n      - run: sudo apt-get update -q && sudo apt-get install -y -q git git-lfs ;;\n      - run: sudo apt-get install -y -qq age >/dev/null\n",
	".agents/setup":             "#!/usr/bin/env bash\nGO_VERSION=\"1.27.2\"\n",
	".cursor/environment.json":  `{"name":"old","install":"npm i","terminals":[{"name":"web","command":"pnpm dev"}]}`,
	".claude/settings.json":     `{"model":"opus","hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"./mine.sh"}]}]}}`,
}

// A Python service on uv with Postgres (its URL in .env.example) and Redis
// (from compose), a secret, and Playwright: a different stack.
var pyPG = map[string]string{
	"pyproject.toml":  "[project]\nname = \"svc\"\nrequires-python = \">=3.12\"\ndependencies = [\"fastapi\", \"psycopg[binary]>=3.2\", \"redis>=5\"]\n\n[dependency-groups]\ndev = [\"pytest>=8\", \"playwright>=1.48\"]\n",
	"uv.lock":         "version = 1\n",
	".python-version": "3.12\n",
	".env.example":    "DATABASE_URL=postgres://app:app@localhost:5432/app_test\nSTRIPE_API_KEY=\nLOG_LEVEL=info\n",
	".gitignore":      ".env\n",
	"compose.yaml":    "services:\n  db:\n    image: postgres:16\n  cache:\n    image: redis:7-alpine\n  mail:\n    image: axllent/mailpit\n",
}

func question(d Discovery, id string) *Question {
	for i := range d.Questions {
		if d.Questions[i].ID == id {
			return &d.Questions[i]
		}
	}
	return nil
}

func TestDiscoverGoAndPnpmWorkspace(t *testing.T) {
	d, err := Discover(repo(t, goNode))
	if err != nil {
		t.Fatal(err)
	}
	s := d.Spec
	if g := s.Tool("go"); g == nil || g.Want != ">=1.25.0" || g.Version != "" || !strings.Contains(g.Evidence, "go.mod:3") {
		t.Errorf("go = %+v, want >=1.25.0 from go.mod:3 and no pin before the interview", g)
	}
	if n := s.Tool("node"); n == nil || n.Want != "^22.12.0 || ^24.0.0 || >=26.0.0" {
		t.Errorf("node = %+v", n)
	}
	if p := s.Tool("pnpm"); p == nil || p.Version != "12.0.0" || p.Via != "corepack" {
		t.Errorf("pnpm = %+v, want 12.0.0 through corepack (the +sha512 suffix dropped)", p)
	}
	if strings.Join(s.Packages, " ") != "age git-lfs" {
		t.Errorf("packages = %v, want age and git-lfs from CI (git is baseline, flags and ;; are not packages)", s.Packages)
	}
	var checks []string
	for _, c := range s.Checks {
		checks = append(checks, c.Run)
	}
	if strings.Join(checks, "; ") != "go build ./...; go test ./...; pnpm -r --if-present run test" {
		t.Errorf("checks = %v", checks)
	}
	if len(s.Browsers) != 1 || s.Browsers[0].Run != "pnpm --filter @x/ui exec playwright install --with-deps chromium" {
		t.Errorf("browsers = %+v, want the workspace package that depends on playwright", s.Browsers)
	}
	if len(s.Services) != 1 || s.Services[0].Name != "postgres" {
		t.Errorf("services = %+v, want postgres from go.mod's pgx", s.Services)
	}
	if q := question(d, "go-version"); q == nil || q.Default != "1.27.2" {
		t.Errorf("go-version question = %+v, want the hand-written pin as the default", q)
	}
	if question(d, "existing-setup") == nil || question(d, "browsers") == nil || question(d, "secrets") != nil {
		t.Errorf("questions = %+v: want existing-setup and browsers, and no secrets question without an env file", d.Questions)
	}
}

func TestDiscoverPythonServicesAndSecrets(t *testing.T) {
	d, err := Discover(repo(t, pyPG))
	if err != nil {
		t.Fatal(err)
	}
	s := d.Spec
	if s.Tool("uv") == nil || s.Tool("python").Want != "3.12" {
		t.Errorf("tools = %+v, want uv and python 3.12 from .python-version", s.Tools)
	}
	if len(s.Deps) != 1 || s.Deps[0].Run != "uv sync --frozen" || len(s.Checks) != 1 || s.Checks[0].Run != "uv run pytest" {
		t.Errorf("deps %+v checks %+v", s.Deps, s.Checks)
	}
	names := map[string]Service{}
	for _, sv := range s.Services {
		names[sv.Name] = sv
	}
	pg := names["postgres"]
	if pg.User != "app" || pg.Password != "app" || pg.Database != "app_test" || names["redis"].Name == "" || len(names) != 2 {
		t.Errorf("services = %+v, want postgres app/app_test from DATABASE_URL and redis from compose (mailpit is a note)", s.Services)
	}
	secret := map[string]bool{}
	for _, e := range s.Env {
		secret[e.Name] = e.Secret
	}
	if !secret["STRIPE_API_KEY"] || secret["DATABASE_URL"] || secret["LOG_LEVEL"] {
		t.Errorf("env = %+v, want only STRIPE_API_KEY secret", s.Env)
	}
	if s.EnvFile != ".env.example -> .env" {
		t.Errorf("env file = %q", s.EnvFile)
	}
	if len(s.Browsers) != 1 || s.Browsers[0].Run != "uv run playwright install --with-deps chromium" {
		t.Errorf("browsers = %+v", s.Browsers)
	}
	q := question(d, "secrets")
	if q == nil || !strings.Contains(q.Ask, "STRIPE_API_KEY") || q.Default != "none" {
		t.Errorf("secrets question = %+v", q)
	}
	if _, err := Settle(d, map[string]Answer{"secrets": {Value: "NOT_THERE", Source: "stated"}}); err == nil {
		t.Error("Settle accepted a secret name the env file does not list")
	}
}

func TestRanges(t *testing.T) {
	for _, c := range []struct {
		want string
		in   []string
		out  []string
	}{
		{"^22.12.0 || ^24.0.0 || >=26.0.0", []string{"22.12.0", "22.99.1", "24.0.0", "26.0.0", "30.1.0"}, []string{"22.11.9", "23.0.0", "25.9.9"}},
		{">=18 <21", []string{"18.0.0", "20.19.5"}, []string{"17.9.9", "21.0.0"}},
		{"20", []string{"20.0.0", "20.19.5"}, []string{"21.0.0", "19.9.0"}},
		{"~1.2.3", []string{"1.2.3", "1.2.9"}, []string{"1.3.0", "1.2.2"}},
		{"^0.3.1", []string{"0.3.1", "0.3.9"}, []string{"0.4.0"}},
		{"22.x", []string{"22.0.0", "22.5.1"}, []string{"23.0.0"}},
	} {
		rs, err := Ranges(c.want)
		if err != nil {
			t.Fatalf("%q: %v", c.want, err)
		}
		for _, v := range c.in {
			if pv, _ := parseVersion(v); !inRanges(pv, rs) {
				t.Errorf("%q (%v) should accept %s", c.want, rs, v)
			}
		}
		for _, v := range c.out {
			if pv, _ := parseVersion(v); inRanges(pv, rs) {
				t.Errorf("%q (%v) should refuse %s", c.want, rs, v)
			}
		}
	}
	if _, err := Ranges("latest || banana"); err == nil {
		t.Error("an unreadable range parsed")
	}
}

// The bash in_ranges the scripts use must agree with Go's on the same inputs
// (sort -V and the half-open upper bound are where a script goes wrong).
func TestBashRangesAgree(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("no bash")
	}
	rs, _ := Ranges("^22.12.0 || ^24.0.0 || >=26.0.0")
	for _, v := range []string{"22.11.9", "22.12.0", "22.100.0", "23.0.0", "24.0.0", "25.9.9", "26.0.0", "26.10.0", "9.9.9"} {
		var args []string
		for _, r := range rs {
			args = append(args, fmt.Sprintf("%q", r))
		}
		cmd := exec.Command("bash", "-c", prelude+"in_ranges "+v+" "+strings.Join(args, " "))
		got := cmd.Run() == nil
		pv, _ := parseVersion(v)
		if want := inRanges(pv, rs); got != want {
			t.Errorf("bash in_ranges %s = %v, Go says %v", v, got, want)
		}
	}
}

// fakeFetch answers the vendors' release indexes.
func fakeFetch(pages map[string]string) Fetch {
	return func(_ context.Context, u string) ([]byte, error) {
		if b, ok := pages[u]; ok {
			return []byte(b), nil
		}
		return nil, errors.New("404 " + u)
	}
}

var sum = func(c byte) string { return strings.Repeat(string(c), 64) }

var indexes = map[string]string{
	"https://go.dev/dl/?mode=json&include=all": `[
		{"version":"go1.27.2","stable":true,"files":[{"os":"linux","arch":"amd64","kind":"archive","sha256":"` + sum('1') + `"},{"os":"linux","arch":"arm64","kind":"archive","sha256":"` + sum('2') + `"}]},
		{"version":"go1.26.0","stable":true,"files":[{"os":"linux","arch":"amd64","kind":"archive","sha256":"` + sum('9') + `"}]},
		{"version":"go1.25.9rc1","stable":false,"files":[]},
		{"version":"go1.25.8","stable":true,"files":[{"os":"linux","arch":"amd64","kind":"archive","sha256":"` + sum('a') + `"},{"os":"linux","arch":"arm64","kind":"archive","sha256":"` + sum('b') + `"},{"os":"linux","arch":"amd64","kind":"installer","sha256":"` + sum('c') + `"}]},
		{"version":"go1.25.0","stable":true,"files":[]}]`,
	"https://nodejs.org/dist/index.json":              `[{"version":"v26.1.0","lts":"Xenon"},{"version":"v25.0.0","lts":false},{"version":"v24.11.0","lts":"Krypton"},{"version":"v22.23.3","lts":"Jod"},{"version":"v22.11.0","lts":"Jod"}]`,
	"https://nodejs.org/dist/v22.23.3/SHASUMS256.txt": sum('d') + "  node-v22.23.3-linux-x64.tar.xz\n" + sum('e') + "  node-v22.23.3-linux-arm64.tar.xz\n" + sum('f') + "  node-v22.23.3-darwin-x64.tar.gz\n",
	"https://registry.npmjs.org/corepack/latest":      `{"version":"0.36.0"}`,
}

func TestResolvePinsWithChecksums(t *testing.T) {
	s := Spec{Tools: []Tool{{Name: "go", Want: ">=1.25.0"}, {Name: "node", Want: "^22.12.0 || ^24.0.0 || >=26.0.0"}, {Name: "pnpm", Want: "12.0.0", Version: "12.0.0", Via: "corepack"}}}
	if err := Resolve(context.Background(), &s, fakeFetch(indexes)); err != nil {
		t.Fatal(err)
	}
	if g := s.Tool("go"); g.Version != "1.25.8" || g.SHA256["amd64"] != sum('a') || g.SHA256["arm64"] != sum('b') {
		t.Errorf("go = %+v, want the newest stable 1.25 patch (not 1.26, not an rc) with archive checksums", g)
	}
	if n := s.Tool("node"); n.Version != "22.23.3" || n.SHA256["amd64"] != sum('d') || n.SHA256["arm64"] != sum('e') {
		t.Errorf("node = %+v, want the newest LTS of the lowest accepted major (22.11.0 is below the range)", n)
	}
	if c := s.Tool("corepack"); c == nil || c.Version != "0.36.0" {
		t.Errorf("corepack = %+v", c)
	}
	// A pinned spec resolves offline.
	if err := Resolve(context.Background(), &s, fakeFetch(nil)); err != nil {
		t.Errorf("a pinned spec went to the network: %v", err)
	}
}

func pinned(t *testing.T, root string, answers map[string]Answer) Spec {
	t.Helper()
	d, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	s, err := Settle(d, answers)
	if err != nil {
		t.Fatal(err)
	}
	if err := Resolve(context.Background(), &s, fakeFetch(indexes)); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestGenerateKeepsTheOwnersKeys(t *testing.T) {
	root := repo(t, goNode)
	s := pinned(t, root, nil)
	// A human's step with shell syntax: its label must print it, not run it.
	custom := `[ "$(id -u)" = 0 ] || curl -fsSL 'https://tools.example.org/x' > /dev/null`
	s.Steps = append(s.Steps, Step{Run: custom, Evidence: "answer steps (human)"})
	if d := Domains(s); !slices.Contains(d, "tools.example.org") {
		t.Errorf("Domains lacks the step's host: %v", d)
	}
	if err := Write(root, s); err != nil {
		t.Fatal(err)
	}
	cur, _ := os.ReadFile(filepath.Join(root, CursorEnvPath))
	for _, want := range []string{`"install": "bash .agents/setup"`, `"start": "bash .agents/start"`, `"terminals"`, `"pnpm dev"`} {
		if !strings.Contains(string(cur), want) {
			t.Errorf("%s lacks %s:\n%s", CursorEnvPath, want, cur)
		}
	}
	// Writing twice changes nothing and keeps one aayu hook beside the owner's.
	if err := Write(root, s); err != nil {
		t.Fatal(err)
	}
	cl, _ := os.ReadFile(filepath.Join(root, ClaudeSetPath))
	if strings.Count(string(cl), "CLAUDE_CODE_REMOTE") != 1 || !strings.Contains(string(cl), "./mine.sh") || !strings.Contains(string(cl), `"model": "opus"`) {
		t.Errorf("%s:\n%s", ClaudeSetPath, cl)
	}
	for _, f := range []string{SetupPath, CheckPath, StartPath, ResumePath} {
		fi, err := os.Stat(filepath.Join(root, f))
		// Windows has no executable bit; there the scripts run through bash.
		if err != nil || (runtime.GOOS != "windows" && fi.Mode()&0o111 == 0) {
			t.Errorf("%s: %v, mode %v; want an executable", f, err, fi)
		}
		if _, err := exec.LookPath("bash"); err == nil {
			if out, err := exec.Command("bash", "-n", filepath.Join(root, f)).CombinedOutput(); err != nil {
				t.Errorf("bash -n %s: %v\n%s", f, err, out)
			}
		}
	}
	setup, _ := os.ReadFile(filepath.Join(root, SetupPath))
	for _, want := range []string{"GO_VERSION=1.27.2", "go$GO_VERSION.linux-$ARCH.tar.gz", "NODE_VERSION=22.23.3", sum('d'), "corepack@0.36.0", "need_apt ca-certificates curl git xz-utils age git-lfs postgresql",
		"pnpm --filter @x/ui exec playwright install --with-deps chromium", "go mod download", "pnpm install --frozen-lockfile"} {
		if !strings.Contains(string(setup), want) {
			t.Errorf("setup lacks %q", want)
		}
	}
	labelled := false
	for _, line := range strings.Split(string(setup), "\n") {
		label, ok := strings.CutPrefix(line, "step ")
		if !ok || !strings.Contains(line, "tools.example.org") {
			continue
		}
		labelled = true
		label, _, _ = strings.Cut(label, " # answer steps")
		if out, err := exec.Command("bash", "-c", "printf %s "+label).Output(); err != nil || string(out) != "Step: "+custom {
			t.Errorf("the step's label prints %q (%v), want the step verbatim", out, err)
		}
	}
	if !labelled {
		t.Error("setup has no step line for the human's step")
	}
	if strings.Contains(string(setup), "sum=\n") || strings.Contains(string(setup), "sum= ") {
		t.Error("setup has a download without its checksum")
	}
	check, _ := os.ReadFile(filepath.Join(root, CheckPath))
	if !strings.HasSuffix(string(check), "echo \""+Marker+" "+s.Fingerprint()+"\"\n") {
		t.Errorf("check does not end with the marker:\n%s", check)
	}
}

// Setup puts aayu on PATH (Bug #819): aayu's own repository builds it from the
// checkout; a product repository gets the release its committed Amp plugin
// expects, pinned to checksums.txt, also at the path the plugin calls.
func TestSetupInstallsAayu(t *testing.T) {
	own := pinned(t, repo(t, map[string]string{"go.mod": "module " + aayuModule + "\n\ngo 1.25.0\n", "cmd/aayudha/main.go": "package main\n"}), nil)
	if a := own.Tool("aayu"); a == nil || a.Via != "source" {
		t.Fatalf("aayu's own repository: aayu = %+v, want a build from the checkout", a)
	}
	other := pinned(t, repo(t, map[string]string{"go.mod": "module example.com/m\n\ngo 1.25.0\n", "cmd/aayudha/main.go": "package main\n"}), nil)
	if a := other.Tool("aayu"); a != nil {
		t.Errorf("another module with a cmd/aayudha: aayu = %+v, want none", a)
	}
	setup, check := setupScript(own, own.Fingerprint()), checkScript(own, own.Fingerprint())
	build := `CGO_ENABLED=0 go build -trimpath -o "$AAYU_DIR/aayudha" ./cmd/aayudha`
	for _, want := range []string{build, `"vcs.revision=$(git rev-parse HEAD) vcs.modified=false"`, `ln -sf aayudha "$AAYU_DIR/aayu"`, `path_first "$AAYU_DIR"`} {
		if !strings.Contains(setup, want) {
			t.Errorf("setup lacks %q", want)
		}
	}
	if strings.Index(setup, build) < strings.Index(setup, "go mod download") || !strings.Contains(check, "\naayu version\n") {
		t.Error("setup must build aayu after the Go dependencies, and check must run it")
	}

	plugin := "// Written by `aayu install`\nconst AAYU = \"/home/alice/.local/bin/aayudha\"\nconst VERSION = \"0.2.0\"\n"
	checksums := sum('7') + "  aayudha_0.2.0_darwin_arm64.tar.gz\n" + sum('8') + "  aayudha_0.2.0_linux_amd64.tar.gz\n" + sum('9') + "  aayudha_0.2.0_linux_arm64.tar.gz\n"
	d, err := Discover(repo(t, map[string]string{ampPlugin: plugin}))
	if err != nil {
		t.Fatal(err)
	}
	s := d.Spec
	if err := Resolve(context.Background(), &s, fakeFetch(map[string]string{aayuReleases + "0.2.0/checksums.txt": checksums})); err != nil {
		t.Fatal(err)
	}
	a := s.Tool("aayu")
	if a == nil || a.Version != "0.2.0" || a.Path != "/home/alice/.local/bin/aayudha" || a.SHA256["amd64"] != sum('8') || a.SHA256["arm64"] != sum('9') || !strings.Contains(a.Evidence, ampPlugin+":3") {
		t.Fatalf("product repository: aayu = %+v, want 0.2.0 from the plugin at its path, pinned to the linux lines of checksums.txt", a)
	}
	setup, check = setupScript(s, s.Fingerprint()), checkScript(s, s.Fingerprint())
	for _, want := range []string{"AAYU_VERSION=0.2.0", "amd64) sum=" + sum('8'), "arm64) sum=" + sum('9'),
		`fetch "https://github.com/rahulnakmol/agentic-ayudha/releases/download/v$AAYU_VERSION/aayudha_${AAYU_VERSION}_linux_$ARCH.tar.gz" "$sum"`,
		"plugin_aayu='/home/alice/.local/bin/aayudha'", `ln -sf aayudha "$AAYU_DIR/aayu"`, `path_first "$AAYU_DIR"`} {
		if !strings.Contains(setup, want) {
			t.Errorf("setup lacks %q", want)
		}
	}
	if !strings.Contains(check, `expect aayu "$(aayu version)" "aayu 0.2.0"`) {
		t.Errorf("check does not expect aayu 0.2.0:\n%s", check)
	}
	if dev, _ := Discover(repo(t, map[string]string{ampPlugin: strings.Replace(plugin, "0.2.0", "0.0.0-dev", 1)})); dev.Spec.Tool("aayu") != nil {
		t.Error("a plugin installed by a dev build has no release to install, want no aayu step")
	}
}

func TestStatus(t *testing.T) {
	root := repo(t, goNode)
	s := pinned(t, root, map[string]Answer{"hosts": {Value: "amp,cursor,claude", Source: "stated"}})
	if err := Write(root, s); err != nil {
		t.Fatal(err)
	}
	state := func() map[string]HostStatus {
		t.Helper()
		sts, err := Status(root)
		if err != nil {
			t.Fatal(err)
		}
		m := map[string]HostStatus{}
		for _, st := range sts {
			m[st.Host] = st
		}
		return m
	}
	st := state()
	if st[Amp].State != Missing || !strings.Contains(st[Amp].Detail, "never verified") || st[Codex].State != Missing {
		t.Fatalf("before verification: %+v", st)
	}
	if err := RecordContainer(root, Amp, s, Result{Image: "debian:12", Phases: map[string]int{"check": 3}}); err != nil {
		t.Fatal(err)
	}
	if err := RecordCloud(root, Amp, Record{Fingerprint: s.Fingerprint(), Method: "cloud", At: "2026-10-16T00:00:00Z", Evidence: "https://ampcode.com/threads/T-x", Project: "o/r"}); err != nil {
		t.Fatal(err)
	}
	if st = state(); st[Amp].State != Verified || !st[Amp].Live || st[Cursor].State != Missing {
		t.Fatalf("after amp verified: %+v", st)
	}
	// An edited generated file is stale for every host that runs it.
	setup := filepath.Join(root, SetupPath)
	b, _ := os.ReadFile(setup)
	_ = os.WriteFile(setup, append(b, []byte("echo hand edit\n")...), 0o755)
	if st = state(); st[Amp].State != Stale || !strings.Contains(strings.Join(st[Amp].Lines, " "), SetupPath+" differs") {
		t.Fatalf("after a hand edit: %+v", st[Amp])
	}
	_ = os.WriteFile(setup, b, 0o755)
	// The toolchain changes in the code: every host is stale and says what changed.
	_ = os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/m\n\ngo 1.26.0\n"), 0o644)
	st = state()
	if st[Amp].State != Stale || !strings.Contains(strings.Join(st[Amp].Lines, " "), "go: the code now asks for >=1.26.0, the spec has >=1.25.0") {
		t.Fatalf("after go.mod moved: %+v", st[Amp])
	}
}

func TestFix(t *testing.T) {
	s := Spec{Packages: []string{"age"}}
	got, what := Fix(s, "==> step\n.agents/check: line 9: make: command not found\n")
	if what == "" || !contains(got.Packages, "make") || contains(s.Packages, "make") {
		t.Errorf("Fix = %v %q; want make added to a copy", got.Packages, what)
	}
	if _, what := Fix(got, ".agents/check: line 9: make: command not found\n"); what != "" {
		t.Errorf("Fix repeated a fix it already made: %q", what)
	}
	if _, what := Fix(s, "panic: tests failed\n"); what != "" {
		t.Errorf("Fix invented a fix for an ordinary failure: %q", what)
	}
}

const codexWarning = `WARNING: proceeding, even though we could not create PATH aliases: Refusing to create helper binaries under temporary dir "/tmp" (codex_home: AbsolutePathBuf("/tmp/h/codex"))`

func TestDetect(t *testing.T) {
	type ans struct {
		out string
		ok  bool
	}
	table := map[string]ans{
		"/bin/codex --version":           {codexWarning + "\ncodex-cli 0.163.0", true},
		"/bin/codex login status":        {"Logged in using an API key - sk-***", true},
		"/bin/claude --version":          {"2.1.301 (Claude Code)", true},
		"/bin/claude auth status --json": {`{"loggedIn": true, "authMethod": "api_key"}`, true},
		"/bin/agent --version":           {"2026.10.14", true},
		"/bin/agent status":              {"Not logged in", true},
		"/bin/amp --version":             {"0.0.1", true},
		"/bin/amp account list":          {"No saved accounts.", true},
	}
	p := Probe{
		LookPath: func(b string) (string, error) {
			if b == "cursor-agent" {
				return "", errors.New("no")
			}
			return "/bin/" + b, nil
		},
		Exec: func(_ context.Context, _, name string, args ...string) (string, error) {
			a := table[strings.Join(append([]string{name}, args...), " ")]
			if !a.ok {
				return a.out, errors.New("exit 1")
			}
			return a.out, nil
		},
		HasSecret: func(n string) bool { return n == "cursor-api-key" },
		CodexEnv:  "env_123",
	}
	got := map[string]Agent{}
	for _, a := range Detect(context.Background(), p) {
		got[a.Name] = a
	}
	// Codex's stderr warning is not its version (seen on megaman, codex-cli 0.163.0).
	if c := got["codex"]; c.Version != "codex-cli 0.163.0" || !slices.Equal(c.Warnings, []string{codexWarning}) {
		t.Errorf("codex version %q, warnings %q", c.Version, c.Warnings)
	}
	if c := got["codex"]; !c.SignedIn || c.Cloud || !strings.Contains(c.CloudDetail, "API key") {
		t.Errorf("codex with an API key login: %+v; want signed in but not cloud-capable", c)
	}
	if c := got["claude"]; !c.SignedIn || c.Cloud || !strings.Contains(c.CloudDetail, "api_key") {
		t.Errorf("claude with an API key: %+v", c)
	}
	if c := got["cursor"]; c.CLI != "/bin/agent" || c.SignedIn || !c.Cloud {
		t.Errorf("cursor: %+v; want the agent binary, not signed in, cloud through the stored key", c)
	}
	if a := got["amp"]; a.SignedIn || a.Cloud || len(a.Install) != 1 || a.SignIn[0] != "amp login" {
		t.Errorf("amp: %+v", a)
	}
	// D-5.5c.24: a Codex that is not signed in but answers through its own
	// model provider (megaman's gateway wrapper) runs locally; one whose
	// provider key is in an environment variable does not, since the local
	// receiver never gets a model provider's key.
	report := func(env string) string {
		d := `"model provider requires OpenAI auth":"false"`
		if env != "" {
			d += `,"provider auth env var":"` + env + ` (present)"`
		}
		// The shape of codex 0.163.0's report: checks keyed by ID, details
		// holding strings and lists, a warning before it.
		return "WARNING: proceeding (codex_home: AbsolutePathBuf(\"/x\"))\n{\n\"schemaVersion\": 1,\n\"checks\": {" +
			"\"config.load\": {\"id\": \"config.load\", \"status\": \"ok\", \"details\": {\"model provider\": \"private_gateway\", \"PATH entries\": [\"/bin\"]}, \"durationMs\": 4}," +
			"\"auth.credentials\": {\"id\": \"auth.credentials\", \"status\": \"ok\", \"details\": {" + d + "}, \"remediation\": null}}}\n"
	}
	table["/bin/codex login status"] = ans{"Not logged in", false}
	for env, want := range map[string]bool{"": true, "GATEWAY_KEY": false, "CODEX_API_KEY": true} {
		table["/bin/codex doctor --json"] = ans{report(env), true}
		for _, a := range Detect(context.Background(), p) {
			if a.Name == "codex" && (a.SignedIn != want || want && a.Via != "the model provider private_gateway" || a.Cloud) {
				t.Errorf("codex through a model provider (key in %q): %+v; want runs locally %v, not cloud", env, a, want)
			}
		}
	}
	table["/bin/codex login status"] = ans{"Logged in using ChatGPT", true}
	table["/bin/claude auth status --json"] = ans{`{"loggedIn": true, "authMethod": "claude.ai"}`, true}
	table["/bin/amp account list"] = ans{"* me@example.com", true}
	root := repo(t, nil)
	_ = RecordCloud(root, Amp, Record{Fingerprint: "sha256:x", Project: "o/r"})
	p.Root, p.Repo = root, "o/r"
	for _, a := range Detect(context.Background(), p) {
		if !a.Cloud && a.Name != "opencode" { // OpenCode has no cloud agent (Story #1012)
			t.Errorf("%s should be cloud-capable now: %+v", a.Name, a)
		}
	}
	// Nothing installed: every agent says how to install it.
	p.LookPath = func(string) (string, error) { return "", errors.New("no") }
	p.HasSecret = nil
	for _, a := range Detect(context.Background(), p) {
		if a.CLI != "" || a.SignedIn || len(a.Install) == 0 || a.Name != "cursor" && a.Cloud {
			t.Errorf("%s with nothing installed: %+v", a.Name, a)
		}
	}
}

func TestProbeReplies(t *testing.T) {
	want := "sha256:0123456789abcdef"
	stream := `{"type":"system","session_id":"T-01a14222-ad07-73da-8a1e-0886cc74f548"}
{"type":"result","result":"aayu-env ok sha256:0123456789abcdef","is_error":false}
`
	var calls [][]string
	r := ProbeAmp(context.Background(), func(_ context.Context, _, name string, args ...string) (string, error) {
		calls = append(calls, append([]string{name}, args...))
		return stream, nil
	}, "o/r", want)
	if !r.OK || r.URL != "https://ampcode.com/threads/T-01a14222-ad07-73da-8a1e-0886cc74f548" {
		t.Errorf("amp probe: %+v", r)
	}
	// The probe thread archives itself once its reply is read; the URL stays as evidence.
	if len(calls) != 2 || slices.Contains(calls[0], "--no-archive-after-execute") ||
		strings.Join(calls[1], " ") != "amp threads archive T-01a14222-ad07-73da-8a1e-0886cc74f548" {
		t.Errorf("want the probe started without --no-archive-after-execute, then archived; calls %q", calls)
	}
	r = ProbeAmp(context.Background(), func(context.Context, string, string, ...string) (string, error) {
		return strings.Replace(stream, "0123456789abcdef", "ffffffffffffffff", 1), nil
	}, "o/r", want)
	if r.OK || !strings.Contains(r.Detail, "merge the environment") {
		t.Errorf("an older environment passed: %+v", r)
	}
	if _, err := RecordReply(Codex, "aayu-env failed: go: command not found", "", want, timeNow()); err == nil {
		t.Error("a failed reply was recorded")
	}
}

func timeNow() time.Time { return time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC) }

func gateJobs(s Spec) []string {
	var out []string
	for _, g := range s.Gate {
		out = append(out, g.Job)
	}
	return out
}

// TestLocalGateMirrorsRequiredJobs: the local gate (.agents/check --full) has
// a step, or a reason it runs only in CI, for every job a pull request must
// pass (Story #399). Adding a CI job without the gate fails here.
func TestLocalGateMirrorsRequiredJobs(t *testing.T) {
	// This repository: the committed gate lists exactly the jobs the workflows
	// have now, with no gap, and covers the required checks (AGENTS.md).
	d, err := Discover("../..")
	if err != nil {
		t.Fatal(err)
	}
	committed, ok, err := LoadSpec("../..")
	if err != nil || !ok {
		t.Fatalf("LoadSpec: %v %v", ok, err)
	}
	if got, want := gateJobs(committed), gateJobs(d.Spec); !slices.Equal(got, want) {
		t.Errorf("%s gate jobs = %v, the workflows have %v: add the job under [[gate]] and run aayu install --env --apply", SpecPath, got, want)
	}
	if gaps := GateGaps(committed); len(gaps) > 0 {
		t.Errorf("CI jobs without a local step or a reason: %v", gaps)
	}
	for _, req := range []string{"build", "generate-drift", "lint", "commits", "security", "docs", "release-snapshot", "protected-paths"} {
		if !slices.Contains(gateJobs(committed), req) {
			t.Errorf("required check %s is not in the local gate", req)
		}
	}

	// Discovery: pull-request jobs only, by check name, in order; a job run
	// only by workflow_dispatch inputs and a push-only workflow are not mirrored.
	root := repo(t, map[string]string{
		".github/workflows/ci.yml": `on:
  pull_request:
  workflow_dispatch:
    inputs: {gate: {type: string}}
jobs:
  vet: {runs-on: x, steps: [{run: "true"}]}
  lint-go: {name: lint, runs-on: x, steps: [{run: "true"}]}
  smoke: {name: "smoke (${{ matrix.os }})", if: "inputs.gate == 'stable'", runs-on: x, steps: [{run: "true"}]}
  integration: {if: "inputs.gate == 'rolling' || inputs.gate == 'stable'", runs-on: x, steps: [{run: "true"}]}
  docs: {if: "github.event_name == 'pull_request' || inputs.gate == 'x'", runs-on: x, steps: [{run: "true"}]}
`,
		".github/workflows/ci.yaml":             "on: pull_request\njobs: [unclosed\n",
		".github/workflows/protected-paths.yml": "on: [push, pull_request_target]\njobs:\n  protected-paths: {runs-on: x, steps: [{run: \"true\"}]}\n",
	})
	d, err = Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := gateJobs(d.Spec); strings.Join(got, " ") != "vet lint docs protected-paths" {
		t.Errorf("gate jobs = %v, want vet lint docs protected-paths", got)
	}
	if !slices.ContainsFunc(d.Spec.Notes, func(n string) bool { return strings.HasPrefix(n, ".github/workflows/ci.yaml: ") }) {
		t.Errorf("notes = %v: an unreadable workflow must be named, not skipped silently", d.Spec.Notes)
	}
	for src, want := range map[string]string{
		"":                                      "",
		"- a list\n":                            "",
		"on: push\njobs: {a: {}}\n":             "",
		"on: pull_request\njobs: {a: {}}\n":     "a",
		"on: {pull_request: {}}\njobs: {a: {}}": "a",
	} {
		if got, err := PullRequestJobs([]byte(src)); err != nil || strings.Join(got, " ") != want {
			t.Errorf("PullRequestJobs(%q) = %v, %v; want %q", src, got, err, want)
		}
	}
	// A spec written before the docs job existed keeps its commands and
	// reasons, and reports the new job as a gap.
	s := d.Spec
	KeepPins(&s, Spec{Gate: []GateJob{{Job: "vet", Run: []string{"go vet ./..."}}, {Job: "lint", Remote: "only in CI"}}})
	if gaps := GateGaps(s); strings.Join(gaps, " ") != "docs (.github/workflows/ci.yml) protected-paths (.github/workflows/protected-paths.yml)" {
		t.Errorf("gaps = %v, want the new docs job", gaps)
	}
	if s.Gate[0].Run[0] != "go vet ./..." || s.Gate[1].Remote != "only in CI" {
		t.Errorf("gate = %+v: the committed commands and reasons were not kept", s.Gate)
	}
}

// The generated gate runs every job, stops a job at its first failing
// command, names each failed CI job, says what runs only in CI, fails on a job
// with no local step, and runs only with --full.
func TestLocalGateScript(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("no bash")
	}
	dir := t.TempDir()
	s := Spec{Gate: []GateJob{
		// A pre-push hook's GIT_DIR must not reach the gate's commands, or tests that run git in a temporary
		// directory change the pushing clone.
		{Job: "ok", Workflow: "ci.yml", Run: []string{`[ -z "${GIT_DIR:-}${GIT_INDEX_FILE:-}${GIT_WORK_TREE:-}" ]`, "echo ok-ran > ok"}},
		{Job: "bad", Workflow: "ci.yml", Run: []string{"true", "false | true", "touch after-fail"}},
		{Job: "later", Workflow: "ci.yml", Run: []string{"echo later-ran > later"}, Remote: "the other platforms"},
		{Job: "remote", Workflow: "pp.yml", Remote: "it reads reviews"},
		{Job: "gap", Workflow: "ci.yml"},
	}}
	script := filepath.Join(dir, ".agents", "check") // it runs from the directory above .agents
	if err := os.MkdirAll(filepath.Dir(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(script, []byte(checkScript(s, "fp")), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", script, "--full")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_DIR="+filepath.Join(dir, "elsewhere.git"), "GIT_INDEX_FILE=index", "GIT_WORK_TREE="+dir)
	out, err := cmd.CombinedOutput()
	var ee *exec.ExitError
	if !errors.As(err, &ee) || ee.ExitCode() != 1 {
		t.Fatalf("gate exit = %v, want 1:\n%s", err, out)
	}
	for _, want := range []string{
		`aayu-gate: FAIL: CI job "bad" fails here: false | true`, // pipefail
		`aayu-gate: NOT RUN LOCALLY: part of CI job "later": the other platforms`,
		`aayu-gate: NOT RUN LOCALLY: CI job "remote" runs only in CI: it reads reviews`,
		`aayu-gate: FAIL: CI job "gap" (ci.yml) has no local step`,
		"aayu-gate: failed CI jobs: bad gap",
	} {
		if !strings.Contains(string(out), want) {
			t.Errorf("gate output lacks %q:\n%s", want, out)
		}
	}
	for f, want := range map[string]bool{"ok": true, "later": true, "after-fail": false} {
		if _, err := os.Stat(filepath.Join(dir, f)); (err == nil) != want {
			t.Errorf("%s written = %v, want %v:\n%s", f, err == nil, want, out)
		}
	}
	if strings.Contains(string(out), Marker) {
		t.Errorf("the gate went on to the environment checks:\n%s", out)
	}
	plain := exec.Command("bash", script)
	if out, err := plain.CombinedOutput(); err != nil || strings.Contains(string(out), "aayu-gate") {
		t.Errorf("without --full: %v, want the environment check only:\n%s", err, out)
	}
}

// The pre-push hook replaces git-lfs's stock hook, whose work it does, but
// never a hand-written one.
func TestInstallPrePush(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "pre-push")
	stock := "#!/bin/sh\ncommand -v git-lfs >/dev/null 2>&1 || { echo >&2 \"\\nThis repository is configured for Git LFS but 'git-lfs' was not found on your path.\\n\"; exit 2; }\ngit lfs pre-push \"$@\"\n"
	for _, c := range []struct {
		old     string
		replace bool
	}{{"", true}, {stock, true}, {PrePushHook, true}, {stock + "./scripts/my-check\n", false}} {
		if c.old != "" {
			if err := os.WriteFile(p, []byte(c.old), 0o755); err != nil {
				t.Fatal(err)
			}
		} else {
			_ = os.Remove(p)
		}
		_, err := InstallPrePush(dir)
		got, _ := os.ReadFile(p)
		if replaced := err == nil && string(got) == PrePushHook; replaced != c.replace {
			t.Errorf("old hook %q: replaced = %v (err %v), want %v", c.old, replaced, err, c.replace)
		}
	}
	// .agents/setup does the same in a cloud host's clone.
	if _, err := exec.LookPath("bash"); err == nil {
		for _, c := range []struct {
			old     string
			replace bool
		}{{"", true}, {stock, true}, {stock + "./scripts/my-check\n", false}} {
			clone := t.TempDir()
			if out, err := exec.Command("git", "init", "-q", clone).CombinedOutput(); err != nil {
				t.Fatalf("git init: %v %s", err, out)
			}
			hook := filepath.Join(clone, ".git", "hooks", "pre-push")
			if c.old != "" {
				if err := os.WriteFile(hook, []byte(c.old), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			cmd := exec.Command("bash", "-ec", "step() { :; }\n"+setupHook(Spec{Gate: []GateJob{{Job: "build"}}}))
			cmd.Dir = clone
			cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+filepath.Join(clone, "none")) // no machine-wide core.hooksPath
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("setup hook step: %v %s", err, out)
			}
			got, _ := os.ReadFile(hook)
			if (string(got) == PrePushHook) != c.replace {
				t.Errorf("setup with old hook %q: hook = %q, want replaced %v", c.old, got, c.replace)
			}
		}
	}
	if !strings.Contains(PrePushHook, `exec git lfs pre-push "$@"`) || !strings.Contains(PrePushHook, "--full </dev/null") {
		t.Errorf("hook does not chain git-lfs or lets the gate read the refs on stdin:\n%s", PrePushHook)
	}
}
