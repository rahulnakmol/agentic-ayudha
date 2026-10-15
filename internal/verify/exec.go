package verify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// Network modes a floor's child process ran under (pack check "network").
const (
	NetIsolated     = "isolated"      // own network namespace with only loopback (Linux)
	NetProxyBlocked = "proxy-blocked" // namespaces unavailable: proxies point at a closed port, Go's module proxy is off
	NetAllowed      = "allowed"       // the floor needs the network (rule registries, vulnerability databases, the app under test)
)

// passEnv are the only parent variables a floor's child process sees. Nothing
// that can carry a credential is on the list: no AAYU_*, GITHUB_TOKEN, GH_*,
// cloud keys, or the parent's HOME (where gh and git keep their tokens).
var passEnv = []string{"PATH", "LANG", "LC_ALL", "TZ", "SYSTEMROOT", "COMSPEC", "PATHEXT", "WINDIR", "TMPDIR", "TEMP", "TMP",
	"GOROOT", "GOPATH", "GOMODCACHE", "GOCACHE", "GOFLAGS", "GOTOOLCHAIN", "CGO_ENABLED",
	"NODE_PATH", "PLAYWRIGHT_BROWSERS_PATH", "JAVA_HOME"}

// Sandbox runs a floor's tools: a scrubbed environment with a throwaway HOME,
// and no network unless the floor asks for it.
type Sandbox struct {
	Getenv func(string) string // the parent's environment; nil means os.Getenv
	Home   string              // HOME for children; a fresh directory per run
	// Isolation disables the network namespace (tests of the fallback, or platforms without it).
	NoNamespaces bool
}

// Cmd is one child process.
type Cmd struct {
	Dir     string
	Name    string
	Args    []string
	Network bool              // the floor needs the network
	Env     map[string]string // extra variables for this tool (never credentials)
	Stdin   []byte
	// KeepHome keeps the user's HOME and profile directories (the screen
	// readers read their settings there); credential variables stay scrubbed.
	KeepHome bool
}

// Out is what a child process produced.
type Out struct {
	Stdout, Stderr []byte
	Code           int
	Network        string
}

// Env returns the scrubbed environment for a child.
func (s *Sandbox) Env(c Cmd, network string) []string {
	get := s.Getenv
	if get == nil {
		get = os.Getenv
	}
	env := []string{"HOME=" + s.Home, "USERPROFILE=" + s.Home, "XDG_CONFIG_HOME=" + s.Home + "/.config", "XDG_CACHE_HOME=" + s.Home + "/.cache", "CI=true", "NO_COLOR=1"}
	if c.KeepHome {
		env = []string{"CI=true", "NO_COLOR=1"}
		for _, k := range []string{"HOME", "USERPROFILE", "APPDATA", "LOCALAPPDATA", "PROGRAMDATA", "PROGRAMFILES", "PROGRAMFILES(X86)", "USERNAME", "USER", "LOGNAME", "COMPUTERNAME", "NUMBER_OF_PROCESSORS", "PROCESSOR_ARCHITECTURE", "OS", "DISPLAY", "TMPDIR"} {
			if v := get(k); v != "" {
				env = append(env, k+"="+v)
			}
		}
	}
	for _, k := range passEnv {
		if v := get(k); v != "" {
			env = append(env, k+"="+v)
		} else if v := goEnv()[k]; v != "" {
			env = append(env, k+"="+v) // Go's caches stay where they are when HOME moves
		}
	}
	if network == NetProxyBlocked {
		for _, k := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "http_proxy", "https_proxy", "all_proxy"} {
			env = append(env, k+"=http://127.0.0.1:9")
		}
		env = append(env, "NO_PROXY=", "no_proxy=", "GOPROXY=off", "GOSUMDB=off")
	}
	for k, v := range c.Env {
		if forbidden(k) {
			continue
		}
		env = append(env, k+"="+v)
	}
	return env
}

func forbidden(k string) bool {
	u := strings.ToUpper(k)
	return strings.HasPrefix(u, "AAYU_") || strings.HasPrefix(u, "GH_") || strings.HasPrefix(u, "GITHUB_") || strings.Contains(u, "TOKEN") || strings.Contains(u, "SECRET") || strings.Contains(u, "PASSWORD") || u == "HOME"
}

var (
	goEnvOnce sync.Once
	goEnvVals map[string]string
)

// goEnv is the parent's resolved Go cache locations and Playwright browsers (`go env`), so a child
// with a throwaway HOME still finds the module cache it may not download into.
func goEnv() map[string]string {
	goEnvOnce.Do(func() {
		goEnvVals = map[string]string{}
		out, err := exec.Command("go", "env", "-json", "GOPATH", "GOMODCACHE", "GOCACHE").Output()
		if err == nil {
			_ = json.Unmarshal(out, &goEnvVals)
		}
		// Playwright's browsers live in the user cache directory, which moves with HOME.
		if d, err := os.UserCacheDir(); err == nil {
			if st, err := os.Stat(filepath.Join(d, "ms-playwright")); err == nil && st.IsDir() {
				goEnvVals["PLAYWRIGHT_BROWSERS_PATH"] = filepath.Join(d, "ms-playwright")
			}
		}
	})
	return goEnvVals
}

// Run runs c and returns its output. A nonzero exit is not an error: floors
// read their tools' exit codes.
func (s *Sandbox) Run(ctx context.Context, c Cmd) (Out, error) {
	mode := NetAllowed
	var cmd *exec.Cmd
	if !c.Network {
		mode = NetProxyBlocked
		if !s.NoNamespaces && namespacesAvailable() {
			mode = NetIsolated
		}
	}
	if mode == NetIsolated {
		self, err := os.Executable()
		if err != nil {
			return Out{}, err
		}
		argv, _ := json.Marshal(append([]string{c.Name}, c.Args...))
		cmd = exec.CommandContext(ctx, self)
		cmd.Env = append(s.Env(c, mode), childMarker+"="+string(argv))
		isolate(cmd)
	} else {
		env := s.Env(c, mode)
		name, err := lookPath(c.Name, env)
		if err != nil {
			return Out{Network: mode}, err
		}
		cmd = exec.CommandContext(ctx, name, c.Args...)
		cmd.Env = env
	}
	cmd.Dir = c.Dir
	if c.Stdin != nil {
		cmd.Stdin = bytes.NewReader(c.Stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	out := Out{Stdout: stdout.Bytes(), Stderr: stderr.Bytes(), Network: mode}
	var ee *exec.ExitError
	switch {
	case errors.As(err, &ee):
		out.Code = ee.ExitCode()
		if mode == NetIsolated && out.Code == childFailed {
			return out, fmt.Errorf("%s: %s", c.Name, strings.TrimSpace(stderr.String()))
		}
		return out, nil
	case err != nil:
		return out, fmt.Errorf("%s: %w", c.Name, err)
	}
	return out, nil
}

// lookPath resolves a tool on the child's PATH, not the parent's. On Windows
// exec.LookPath tries each PATHEXT extension, so npm's .cmd shims resolve too.
func lookPath(name string, env []string) (string, error) {
	if strings.ContainsAny(name, `/\`) {
		return name, nil
	}
	for _, kv := range env {
		if v, ok := strings.CutPrefix(kv, "PATH="); ok {
			for _, dir := range filepath.SplitList(v) {
				if dir == "" {
					continue // a bare name would make exec.LookPath search the parent's PATH
				}
				if p, err := exec.LookPath(filepath.Join(dir, name)); err == nil {
					return p, nil
				}
			}
		}
	}
	return "", fmt.Errorf("%s: not found on the floor's PATH", name)
}

// childMarker carries a child's argv into the re-executed binary, which brings
// up loopback in its new network namespace and then execs the tool.
const childMarker = "AAYU_FLOOR_CHILD_ARGV"

// childFailed is the exit code of a sandbox child that could not start its tool.
const childFailed = 197

func init() {
	if argv := os.Getenv(childMarker); argv != "" {
		runChild(argv)
	}
}

// runChild runs in the re-executed binary inside the namespaces: loopback up,
// marker removed from the environment, then exec of the tool.
func runChild(raw string) {
	var argv []string
	if err := json.Unmarshal([]byte(raw), &argv); err != nil || len(argv) == 0 {
		fmt.Fprintln(os.Stderr, "aayudha sandbox: bad argv")
		os.Exit(childFailed)
	}
	if err := loopbackUp(); err != nil {
		fmt.Fprintln(os.Stderr, "aayudha sandbox: loopback:", err)
		os.Exit(childFailed)
	}
	_ = os.Unsetenv(childMarker)
	path, err := exec.LookPath(argv[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "aayudha sandbox:", err)
		os.Exit(childFailed)
	}
	err = execve(path, argv, os.Environ())
	fmt.Fprintln(os.Stderr, "aayudha sandbox: exec:", err)
	os.Exit(childFailed)
}
