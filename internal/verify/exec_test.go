package verify

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// WP4.3 accept: a floor's child process cannot read AAYU_* or GITHUB_TOKEN
// (or any other credential-shaped variable, or the parent's HOME), whether
// or not network namespaces are available.
func TestChildEnvironmentIsScrubbed(t *testing.T) {
	parent := map[string]string{
		"PATH": os.Getenv("PATH"), "HOME": "/home/parent",
		"AAYU_GITHUB_TOKEN": "canary-aayu", "AAYU_SECRET_AI_GATEWAY": "canary-gw", "GITHUB_TOKEN": "canary-gh",
		"GH_TOKEN": "canary-ghcli", "AAYU_BOT_TOKEN": "canary-bot", "AWS_SECRET_ACCESS_KEY": "canary-aws",
	}
	for _, noNS := range []bool{false, true} {
		s := &Sandbox{Getenv: func(k string) string { return parent[k] }, Home: t.TempDir(), NoNamespaces: noNS}
		out, err := s.Run(context.Background(), Cmd{Dir: t.TempDir(), Name: "env", Env: map[string]string{"GITHUB_TOKEN": "canary-extra", "SEMGREP_X": "ok"}})
		if err != nil || out.Code != 0 {
			t.Fatalf("env (namespaces off: %t): %v %d %s", noNS, err, out.Code, out.Stderr)
		}
		env := string(out.Stdout)
		if strings.Contains(env, "canary") || strings.Contains(env, "/home/parent") || strings.Contains(env, childMarker) {
			t.Fatalf("child saw a credential or the parent's HOME (namespaces off: %t):\n%s", noNS, env)
		}
		if !strings.Contains(env, "SEMGREP_X=ok") || !strings.Contains(env, "HOME="+s.Home) {
			t.Fatalf("child env lacks its own variables:\n%s", env)
		}
		if noNS && (!strings.Contains(env, "HTTPS_PROXY=http://127.0.0.1:9") || out.Network != NetProxyBlocked) {
			t.Fatalf("fallback without namespaces: network %s\n%s", out.Network, env)
		}
	}
}

// No-network default: with namespaces, a child cannot reach a listener in the
// parent's network namespace, but its own loopback works.
func TestChildHasNoNetwork(t *testing.T) {
	if !namespacesAvailable() {
		t.Skip("user and network namespaces are not available here; the proxy-blocked fallback is covered above")
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			_ = c.Close()
		}
	}()
	s := &Sandbox{Getenv: func(k string) string { return map[string]string{"PATH": os.Getenv("PATH")}[k] }, Home: t.TempDir()}
	port := ln.Addr().(*net.TCPAddr).Port
	script := "exec 3<>/dev/tcp/127.0.0.1/" + strconv.Itoa(port) + " && echo reached || echo blocked"
	out, err := s.Run(context.Background(), Cmd{Dir: t.TempDir(), Name: "bash", Args: []string{"-c", script}})
	if err != nil || out.Network != NetIsolated || !strings.Contains(string(out.Stdout), "blocked") {
		t.Fatalf("isolated child: %v %s %q %q", err, out.Network, out.Stdout, out.Stderr)
	}
	out, _ = s.Run(context.Background(), Cmd{Dir: t.TempDir(), Name: "bash", Args: []string{"-c", "cat /sys/class/net/lo/flags"}})
	if f, err := strconv.ParseInt(strings.TrimSpace(string(out.Stdout)), 0, 64); err != nil || f&1 == 0 { // IFF_UP
		t.Fatalf("loopback flags in the namespace: %q %q", out.Stdout, out.Stderr)
	}
	out, _ = s.Run(context.Background(), Cmd{Dir: t.TempDir(), Name: "bash", Args: []string{"-c", "grep CapAmb /proc/self/status"}})
	if !strings.Contains(string(out.Stdout), "0000000000000000") {
		t.Fatalf("the tool kept ambient capabilities: %s", out.Stdout)
	}
}

// An isolated child's background processes die with it (PID namespace), so
// code under test cannot outlive its floor.
func TestChildLeavesNoProcesses(t *testing.T) {
	if !namespacesAvailable() {
		t.Skip("namespaces are not available here")
	}
	mark := filepath.Join(t.TempDir(), "survived")
	s := &Sandbox{Getenv: func(k string) string { return map[string]string{"PATH": os.Getenv("PATH")}[k] }, Home: t.TempDir()}
	out, err := s.Run(context.Background(), Cmd{Dir: t.TempDir(), Name: "sh", Args: []string{"-c", "(sleep 1; touch " + mark + ") & echo started"}})
	if err != nil || !strings.Contains(string(out.Stdout), "started") {
		t.Fatalf("%v %s %s", err, out.Stdout, out.Stderr)
	}
	time.Sleep(2 * time.Second)
	if _, err := os.Stat(mark); err == nil {
		t.Fatal("a background process outlived the sandboxed tool")
	}
}
