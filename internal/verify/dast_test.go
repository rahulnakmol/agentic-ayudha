package verify

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
)

const (
	zapClean = `{"@version":"2.17.0","site":[{"alerts":[{"pluginid":"10038","name":"Content Security Policy (CSP) Header Not Set","riskcode":"2","count":"3"}]}]}`
	zapHigh  = `{"@version":"2.17.0","site":[{"alerts":[{"pluginid":"40012","name":"Cross Site Scripting (Reflected)","riskcode":"3","count":"1"}]}]}`
)

// standInRuntime writes a container runtime that records its argv and the
// ZAP plan it was given, and writes report as ZAP's JSON report.
func standInRuntime(t *testing.T, dir, name, report string) {
	t.Helper()
	script := `#!/bin/sh
PATH=/usr/bin:/bin
here=$(dirname "$0")
printf '%s\n' "$@" > "$here/` + name + `.argv"
printf 'HOME=%s XDG_RUNTIME_DIR=%s\n' "$HOME" "$XDG_RUNTIME_DIR" > "$here/` + name + `.env"
for a in "$@"; do case "$a" in *:/zap/wrk:rw) w="${a%:/zap/wrk:rw}";; esac; done
cp "$w/plan.yaml" "$here/` + name + `.plan"
cat > "$w/zap.json" <<'JSON'
` + report + `
JSON
`
	if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

// dastRun is a DAST run on platforms against url, with only bin on PATH,
// on the day after the advisory period.
func dastRun(t *testing.T, bin, url string, platforms ...string) *Run {
	t.Helper()
	r := &Run{Root: t.TempDir(), Work: t.TempDir(), Env: func(string) string { return "" }, Story: 7,
		Now: func() time.Time { return time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC) }}
	r.Verify.DAST, r.Verify.Platforms, r.Verify.NewFloorsBlockingFrom = true, platforms, "2026-11-14"
	r.Floors.DAST.SkippedBlocksShip = true
	if url != "" {
		r.Verify.Environments = map[string]config.Environment{"local": {URL: url}}
	}
	r.Sandbox = &Sandbox{Home: t.TempDir(), Getenv: func(k string) string {
		return map[string]string{"PATH": bin, "HOME": "/home/person", "XDG_RUNTIME_DIR": "/run/user/1000", "GITHUB_TOKEN": "t"}[k]
	}}
	return r
}

// withSeed gives the run a [verify] seed that succeeds: verify starts the app.
func withSeed(t *testing.T, r *Run, bin string) {
	t.Helper()
	p := filepath.Join(bin, "serve")
	if err := os.WriteFile(p, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	r.Verify.Seed = []string{p}
}

// freePort is a loopback port nothing listens on: reserved, then released.
func freePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()
	_, port, _ := net.SplitHostPort(l.Addr().String())
	return port
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(b)
}

func dastCheck(t *testing.T, r *Run) (proof.Check, Report) {
	t.Helper()
	rep := Execute(context.Background(), r, []Floor{DAST{}})
	return checkOf(rep, "dast"), rep
}

// Decision #485 G1, G2, G3, G5: active rules only against an app verify
// started on a loopback host (or an opted-in URL), n/a off web and api, a
// blocking skip with no target, and high alerts advisory until the date.
func TestDASTGuardrails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the stand-in runtime is a sh script")
	}
	setGOOS(t, "linux")
	active := func(plan string) bool {
		if !strings.Contains(plan, "type: activeScan") {
			return false
		}
		for _, id := range DASTRules {
			if !strings.Contains(plan, "{id: "+id+",") {
				return false
			}
		}
		return true
	}
	t.Run("active or passive by target", func(t *testing.T) {
		for _, tc := range []struct {
			name       string
			url        string
			seed, opt  bool
			wantActive bool
		}{
			{"seeded loopback", "http://127.0.0.1:FREE", true, false, true},
			{"seeded localhost", "http://localhost:FREE", true, false, true},
			// Codex review of #509: a seed that only loads fixtures into an app
			// the person already runs did not start it.
			{"seeded, but the app was already up", "UP", true, false, false},
			{"loopback the person started", "http://127.0.0.1:8080", false, false, false},
			{"seeded, but a shared URL", "https://staging.example.test", true, false, false},
			{"a shared URL", "https://staging.example.test", false, false, false},
			{"a shared URL the repo config opts in", "https://staging.example.test", false, true, true},
		} {
			t.Run(tc.name, func(t *testing.T) {
				switch {
				case tc.url == "UP":
					up := httptest.NewServer(http.NotFoundHandler())
					defer up.Close()
					tc.url = up.URL
				case strings.HasSuffix(tc.url, ":FREE"):
					tc.url = strings.TrimSuffix(tc.url, "FREE") + freePort(t)
				}
				bin := t.TempDir()
				standInRuntime(t, bin, "docker", zapClean)
				r := dastRun(t, bin, tc.url, "web")
				if tc.seed {
					withSeed(t, r, bin)
				}
				if tc.opt {
					r.Verify.Environments["local"] = config.Environment{URL: tc.url, ActiveScan: true}
				}
				c, _ := dastCheck(t, r)
				plan := read(t, filepath.Join(bin, "docker.plan"))
				if c.Result != proof.Pass || active(plan) != tc.wantActive || !strings.Contains(plan, `urls: ["`+tc.url+`/"]`) {
					t.Fatalf("active = %v, want %v: %+v\n%s", active(plan), tc.wantActive, c, plan)
				}
				if tc.wantActive != strings.HasPrefix(c.Detail, "active scan") || !tc.wantActive && !strings.Contains(c.Detail, "cannot see a reflected XSS") {
					t.Fatalf("the detail does not name the mode: %q", c.Detail)
				}
			})
		}
	})

	t.Run("n/a off web and api", func(t *testing.T) {
		for _, platforms := range [][]string{{"cli"}, nil, {"ios", "library"}} {
			bin := t.TempDir()
			standInRuntime(t, bin, "docker", zapHigh)
			r := dastRun(t, bin, "http://127.0.0.1:8080", platforms...)
			withSeed(t, r, bin)
			c, rep := dastCheck(t, r)
			if c.Result != proof.Skipped || c.Detail != "n/a: dast (no web or api in [verify] platforms)" || !rep.Hold || proof.BlocksShip(c) || read(t, filepath.Join(bin, "docker.argv")) != "" {
				t.Fatalf("%v: %+v hold=%v", platforms, c, rep.Hold)
			}
		}
	})

	t.Run("no runnable target blocks until waived", func(t *testing.T) {
		bin := t.TempDir()
		standInRuntime(t, bin, "docker", zapClean)
		for _, r := range []*Run{dastRun(t, bin, "", "api"), func() *Run { r := dastRun(t, bin, "http://127.0.0.1:8080", "API"); r.Verify.DAST = false; return r }()} {
			c, rep := dastCheck(t, r)
			for _, want := range []string{"skipped: dast: no runnable target", "[verify] seed", "[verify.environments.local] url", "aayu decide <story> --waiver dast"} {
				if !strings.Contains(c.Detail, want) {
					t.Fatalf("detail lacks %q: %q", want, c.Detail)
				}
			}
			if c.Result != proof.Skipped || rep.Hold || !proof.BlocksShip(c) {
				t.Fatalf("a web or api story with nothing to scan must block: %+v hold=%v", c, rep.Hold)
			}
			if got := ApplyWaivers(rep.Checks, 7, []Waiver{{Decision: 9, Title: "Waive dast on #7", Links: "waives #7"}}); len(got) != 1 || !(&proof.Pack{Checks: rep.Checks}).FloorsHold() {
				t.Fatalf("the waiver does not release it: %v %+v", got, rep.Checks)
			}
		}
		// Before the blocking date the same skip names the fix but does not block.
		r := dastRun(t, bin, "", "web")
		r.Now = func() time.Time { return time.Date(2026, 11, 13, 0, 0, 0, 0, time.UTC) }
		if c, _ := dastCheck(t, r); c.Result != proof.Skipped || proof.BlocksShip(c) || !strings.Contains(c.Detail, "advisory until 2026-11-14") || !strings.Contains(c.Detail, "[verify] seed") {
			t.Fatalf("no target before the date: %+v", c)
		}
		r = dastRun(t, bin, "", "web")
		r.Floors.DAST.SkippedBlocksShip = false
		if c, rep := dastCheck(t, r); c.Result != proof.Skipped || !rep.Hold {
			t.Fatalf("skipped_blocks_ship = false: %+v", c)
		}
		// Codex review of #509: a run with no story, such as the security
		// workflow's, has nothing a waiver could release, so it records the
		// skip and its fix without blocking.
		r = dastRun(t, bin, "", "web")
		r.Story = 0
		if c, rep := dastCheck(t, r); c.Result != proof.Skipped || !rep.Hold || proof.BlocksShip(c) || !strings.Contains(c.Detail, "no story or Bug to waive it on") || !strings.Contains(c.Detail, "[verify] seed") {
			t.Fatalf("no story: %+v hold=%v", c, rep.Hold)
		}
	})

	// Codex security review of #509: verify --security holds the Bug-writing
	// token, so it never runs the seed, the repository's own code. A loopback
	// target, which only the seed could start, is a non-blocking skip; a
	// deployed URL still gets its passive scan.
	t.Run("no seed beside the security token", func(t *testing.T) {
		for _, tc := range []struct{ url, want string }{
			{"http://127.0.0.1:" + freePort(t), "never runs [verify] seed"},
			{"https://staging.example.test", "passive scan only"},
		} {
			bin := t.TempDir()
			standInRuntime(t, bin, "docker", zapClean)
			r := dastRun(t, bin, tc.url, "web")
			ran := filepath.Join(bin, "seed-ran")
			withSeed(t, r, bin)
			if err := os.WriteFile(r.Verify.Seed[0], []byte("#!/bin/sh\n: > '"+ran+"'\n"), 0o755); err != nil {
				t.Fatal(err)
			}
			r.Story, r.NoSeed = 0, true
			c, rep := dastCheck(t, r)
			if _, err := os.Stat(ran); err == nil {
				t.Fatalf("%s: the seed ran", tc.url)
			}
			if !strings.Contains(c.Detail, tc.want) || !rep.Hold || proof.BlocksShip(c) {
				t.Fatalf("%s: %+v hold=%v", tc.url, c, rep.Hold)
			}
		}
	})

	t.Run("a target that is not an absolute URL is blocked", func(t *testing.T) {
		// Codex review of #509: a scheme-relative or non-HTTP url is not a target
		// either, and with no container runtime the url still blocks, never skips.
		for _, target := range []string{"127.0.0.1:8080", "//127.0.0.1:8080", "ftp://127.0.0.1:8080"} {
			for _, runtime := range []bool{true, false} {
				bin := t.TempDir()
				if runtime {
					standInRuntime(t, bin, "docker", zapClean)
				}
				r := dastRun(t, bin, target, "web")
				if c, rep := dastCheck(t, r); c.Result != proof.Blocked || rep.Hold || !strings.Contains(c.Detail, "not an absolute http or https URL") || read(t, filepath.Join(bin, "docker.argv")) != "" {
					t.Fatalf("%s (runtime %v): %+v", target, runtime, c)
				}
			}
		}
	})

	t.Run("with no blocking date a high alert stays advisory", func(t *testing.T) {
		bin := t.TempDir()
		standInRuntime(t, bin, "docker", zapHigh)
		r := dastRun(t, bin, "http://127.0.0.1:8080", "web")
		r.Verify.NewFloorsBlockingFrom = ""
		if c, _ := dastCheck(t, r); c.Result != proof.Pass || !strings.HasPrefix(c.Detail, "advisory (no [verify] new_floors_blocking_from date)") {
			t.Fatalf("%+v", c)
		}
	})

	t.Run("high alerts are advisory until the date", func(t *testing.T) {
		for _, tc := range []struct {
			now  time.Time
			want string
		}{
			{time.Date(2026, 11, 13, 23, 59, 0, 0, time.UTC), proof.Pass},
			{time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC), proof.Fail},
		} {
			bin := t.TempDir()
			standInRuntime(t, bin, "docker", zapHigh)
			r := dastRun(t, bin, "http://127.0.0.1:8080", "web")
			withSeed(t, r, bin)
			r.Now = func() time.Time { return tc.now }
			c, rep := dastCheck(t, r)
			if c.Result != tc.want || !strings.Contains(c.Detail, "Cross Site Scripting (Reflected)") {
				t.Fatalf("%s: %+v", tc.now, c)
			}
			if advisory := strings.Contains(c.Detail, "advisory until 2026-11-14"); advisory != (tc.want == proof.Pass) {
				t.Fatalf("%s: advisory detail: %q", tc.now, c.Detail)
			}
			if tc.want == proof.Fail && (len(rep.Findings) != 1 || rep.Findings[0].Floor != "dast" || rep.Findings[0].ID != "40012") {
				t.Fatalf("the failing alert is not a finding a security Bug can name: %+v", rep.Findings)
			}
		}
	})
}

// Decision #485 G4: docker first, then podman, by the fully qualified digest
// with the host network on Linux; the runtime's host name on macOS and
// Windows; no runtime skips locally and blocks in CI.
func TestContainerRuntime(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the stand-in runtime is a sh script")
	}
	run := func(t *testing.T, ci bool, runtimes ...string) (proof.Check, Report, string) {
		bin := t.TempDir()
		for _, rt := range runtimes {
			standInRuntime(t, bin, rt, zapClean)
		}
		r := dastRun(t, bin, "http://127.0.0.1:8080", "web")
		withSeed(t, r, bin)
		r.CI = ci
		c, rep := dastCheck(t, r)
		return c, rep, bin
	}
	t.Run("linux", func(t *testing.T) {
		setGOOS(t, "linux")
		for _, tc := range []struct {
			present []string
			want    string
		}{{[]string{"docker", "podman"}, "docker"}, {[]string{"podman"}, "podman"}, {[]string{"docker"}, "docker"}} {
			c, _, bin := run(t, false, tc.present...)
			argv := strings.Split(strings.TrimSpace(read(t, filepath.Join(bin, tc.want+".argv"))), "\n")
			if c.Result != proof.Pass || !strings.Contains(c.Detail, "under "+tc.want) {
				t.Fatalf("%v: %+v", tc.present, c)
			}
			// The person's image store, not a fresh one under the sandbox's HOME, so the image is not pulled again.
			if env := read(t, filepath.Join(bin, tc.want+".env")); env != "HOME=/home/person XDG_RUNTIME_DIR=/run/user/1000\n" {
				t.Fatalf("%s ran with %q", tc.want, env)
			}
			for _, other := range ContainerRuntimes {
				if other != tc.want && read(t, filepath.Join(bin, other+".argv")) != "" {
					t.Fatalf("%v: ran %s as well as %s", tc.present, other, tc.want)
				}
			}
			i := slices.Index(argv, "--network")
			if i < 0 || i+1 >= len(argv) || argv[i+1] != "host" || !slices.Contains(argv, ZAPImage) || ZAPImage != "docker.io/zaproxy/zap-stable@sha256:7aaa659b0d43078febd82e29bad112285c370727e86ab8340444220e17d9f0d2" ||
				!slices.ContainsFunc(argv, func(a string) bool { return strings.HasSuffix(a, string(filepath.Separator)+"zap:/zap/wrk:rw") }) {
				t.Fatalf("%s argv: %q", tc.want, argv)
			}
		}
	})
	t.Run("macOS and Windows reach the host by name", func(t *testing.T) {
		for _, os := range []string{"darwin", "windows"} {
			setGOOS(t, os)
			for rt, host := range map[string]string{"docker": "host.docker.internal", "podman": "host.containers.internal"} {
				_, _, bin := run(t, false, rt)
				plan, argv := read(t, filepath.Join(bin, rt+".plan")), read(t, filepath.Join(bin, rt+".argv"))
				if !strings.Contains(plan, `urls: ["http://`+host+`:8080/"]`) || strings.Contains(argv, "--network") {
					t.Fatalf("%s %s: plan %s argv %q", os, rt, plan, argv)
				}
				// A target with no port keeps none.
				bin = t.TempDir()
				standInRuntime(t, bin, rt, zapClean)
				r := dastRun(t, bin, "http://localhost", "web")
				withSeed(t, r, bin)
				dastCheck(t, r)
				if plan := read(t, filepath.Join(bin, rt+".plan")); !strings.Contains(plan, `urls: ["http://`+host+`/"]`) {
					t.Fatalf("%s %s portless: %s", os, rt, plan)
				}
			}
		}
	})
	t.Run("no runtime", func(t *testing.T) {
		setGOOS(t, "linux")
		c, rep, _ := run(t, false)
		if c.Result != proof.Skipped || !rep.Hold || proof.BlocksShip(c) || !strings.Contains(c.Detail, "install Docker or Podman") || !strings.Contains(c.Detail, "aayudha-security.yml") {
			t.Fatalf("locally: %+v hold=%v", c, rep.Hold)
		}
		if c, rep, _ := run(t, true); c.Result != proof.Blocked || rep.Hold {
			t.Fatalf("in CI: %+v hold=%v", c, rep.Hold)
		}
	})
}

func setGOOS(t *testing.T, os string) {
	t.Helper()
	old := goos
	goos = os
	t.Cleanup(func() { goos = old })
}
