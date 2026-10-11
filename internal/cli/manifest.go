package cli

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

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
// page posts the manifest to GitHub, the human confirms there, and GitHub
// redirects back to the local server with a one-time code.
func (e *Env) manifestCode(ctx context.Context, postURL string, manifest func(redirect string) string) (string, error) {
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
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	select {
	case c := <-codes:
		return c, nil
	case <-ctx.Done():
		return "", errors.New("timed out waiting for GitHub to redirect back with the App code")
	}
}
