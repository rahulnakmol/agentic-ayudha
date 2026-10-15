//go:build unix

// serve.sh is a POSIX script that starts the app detached and tracks it by its
// Unix pid (kill, nohup, $!); it does not run on Windows (docs/BUILD-PLAN.md
// Appendix B, D-0.4g), so these tests are Unix only.

package deploy

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

// serve.sh builds the example, starts it detached on the given port, and
// returns once /healthz answers; the app then serves the Ask page with the
// design-system tokens, and a second run replaces the first instance.
func TestServeScript(t *testing.T) {
	if _, err := exec.LookPath("curl"); err != nil {
		t.Skip("curl is not installed")
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
	_ = ln.Close()
	tmp := t.TempDir()
	serve := func() string {
		cmd := exec.Command("./serve.sh")
		cmd.Env = append(os.Environ(), "BQA_PORT="+port, "TMPDIR="+tmp)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("serve.sh: %v\n%s", err, out)
		}
		return string(out)
	}
	pidOf := func() int {
		b, err := os.ReadFile(filepath.Join(tmp, "benefits-qa-"+port, "pid"))
		if err != nil {
			t.Fatal(err)
		}
		n, _ := strconv.Atoi(strings.TrimSpace(string(b)))
		return n
	}
	out := serve()
	first := pidOf()
	t.Cleanup(func() { _ = syscall.Kill(pidOf(), syscall.SIGTERM) })
	if !strings.Contains(out, "benefits-qa: http://127.0.0.1:"+port) {
		t.Errorf("output: %s", out)
	}
	get := func(path string) (int, string) {
		resp, err := http.Get("http://127.0.0.1:" + port + path)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = resp.Body.Close() }()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(b)
	}
	if code, body := get("/healthz"); code != 200 || body != "ok\n" {
		t.Errorf("/healthz: %d %q", code, body)
	}
	if code, body := get("/"); code != 200 || !strings.Contains(body, "Ask about your benefits") {
		t.Errorf("/: %d", code)
	}
	if code, body := get("/tokens.css"); code != 200 || !strings.Contains(body, "--ay-color-brand-default") {
		t.Errorf("/tokens.css: %d", code)
	}
	// The second run succeeds only if the first instance released the port: its new process binds it and answers.
	serve()
	if second := pidOf(); second == first {
		t.Errorf("the second run did not replace pid %d", first)
	}
	if code, _ := get("/healthz"); code != 200 {
		t.Errorf("/healthz after the second run: %d", code)
	}
}

// Another server already answering /healthz on the port is not taken for the
// new instance: serve.sh fails, says why, and leaves that server running.
func TestServeScriptRefusesForeignServer(t *testing.T) {
	if _, err := exec.LookPath("curl"); err != nil {
		t.Skip("curl is not installed")
	}
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "ok\n") }))
	defer foreign.Close()
	port := strconv.Itoa(foreign.Listener.Addr().(*net.TCPAddr).Port)
	cmd := exec.Command("./serve.sh")
	cmd.Env = append(os.Environ(), "BQA_PORT="+port, "TMPDIR="+t.TempDir())
	out, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "did not answer /healthz on port "+port) {
		t.Fatalf("serve.sh on a taken port: %v\n%s", err, out)
	}
	resp, err := http.Get(foreign.URL + "/healthz")
	if err != nil {
		t.Fatalf("the foreign server was stopped: %v", err)
	}
	_ = resp.Body.Close()
}
