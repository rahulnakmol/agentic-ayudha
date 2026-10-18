package cli

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/hosts"
)

// loopbackWait is how long the local redirect can take.
var loopbackWait = 10 * time.Minute

// askLine reads one line from a human at a terminal; without one it returns "".
func (e *Env) askLine(prompt string) string {
	f, ok := e.In.(*os.File)
	if !ok {
		return ""
	}
	if fi, err := f.Stat(); err != nil || fi.Mode()&os.ModeCharDevice == 0 {
		return ""
	}
	fmt.Fprint(e.Err, prompt)
	line, _ := bufio.NewReader(f).ReadString('\n')
	return strings.TrimSpace(line)
}

var manifestPage = template.Must(template.New("m").Parse(`<!doctype html><html lang="en"><head><meta charset="utf-8"><title>aayudha: create the bot App</title></head>
<body><form id="f" method="post" action="{{.Action}}"><input type="hidden" name="manifest" value="{{.Manifest}}">
<p>Creating the aayudha bot GitHub App. If nothing happens, <button type="submit">continue to GitHub</button>.</p></form>
<script>document.getElementById("f").submit()</script></body></html>`))

// manifestCode runs the GitHub App manifest flow (init --identity): a local
// page posts the manifest to GitHub, the person confirms there, and GitHub
// redirects back to a local server with the one-time code. A browser on
// another device (an orb's Terminal tab, a cloud agent's terminal, SSH, a
// phone) cannot reach that server, so init refuses at once instead of waiting
// (#578). Whoever converts the code first holds the App's key, so the code
// never goes to a page aayu does not serve itself: setup from a phone or a
// cloud agent goes through the owner's token exchange (Decision #576, #594).
func (e *Env) manifestCode(ctx context.Context, postURL string, manifest func(redirect string) string) (string, error) {
	if !localBrowser(e.Getenv, runtime.GOOS) {
		return "", WithNext(Errorf(ExitBlocked, "init --identity needs a browser on this machine: GitHub hands the App's one-time code to a local page, which a browser on another device (a phone, an orb's Terminal tab, a cloud agent, SSH) cannot reach; no App was created"),
			"run `aayu init --identity` in a terminal on the computer whose browser you use; setup from a phone or a cloud agent comes with the owner's token exchange (#594)")
	}
	return e.manifestLoopback(ctx, postURL, manifest)
}

// localBrowser reports whether a browser on this machine can reach a loopback
// callback: not over SSH, not in a cloud or agent session, and a desktop.
// AAYU_REMOTE_BROWSER=1 says the browser is elsewhere.
func localBrowser(getenv func(string) string, goos string) bool {
	for _, v := range []string{"AAYU_REMOTE_BROWSER", "SSH_CONNECTION", "SSH_TTY", "AMP_ORB", "CLAUDE_CODE_REMOTE", "CODESPACES", "GITPOD_WORKSPACE_ID"} {
		if getenv(v) != "" {
			return false
		}
	}
	if hosts.AgentMarker(getenv) != "" {
		return false
	}
	return goos == "darwin" || goos == "windows" || getenv("DISPLAY") != "" || getenv("WAYLAND_DISPLAY") != ""
}

// manifestLoopback is the flow for a browser on this machine.
func (e *Env) manifestLoopback(ctx context.Context, postURL string, manifest func(redirect string) string) (string, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	defer func() { _ = ln.Close() }()
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	state := hex.EncodeToString(b)
	base := "http://" + ln.Addr().String()
	codes := make(chan string, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) {
		_ = manifestPage.Execute(w, map[string]string{"Action": postURL + "?state=" + state, "Manifest": manifest(base + "/callback")})
	})
	mux.HandleFunc("GET /callback", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != state || r.URL.Query().Get("code") == "" {
			http.Error(w, "state mismatch", http.StatusBadRequest)
			return
		}
		fmt.Fprintln(w, "The App is created. You can close this tab and return to the terminal.")
		select {
		case codes <- r.URL.Query().Get("code"):
		default:
		}
	})
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = srv.Serve(ln) }()
	defer func() { _ = srv.Close() }()
	fmt.Fprintf(e.Err, "Open %s/ in your browser to create the App on GitHub (waiting up to 10 minutes).\n", base)
	ctx, cancel := context.WithTimeout(ctx, loopbackWait)
	defer cancel()
	select {
	case c := <-codes:
		return c, nil
	case <-ctx.Done():
		return "", WithNext(Errorf(ExitBlocked, "init --identity: GitHub did not redirect back with the App code within 10 minutes, so no App was created"), "aayu init --identity")
	}
}
