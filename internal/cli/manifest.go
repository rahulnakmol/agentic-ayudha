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
	"net/url"
	"os"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/hosts"
	"github.com/rahulnakmol/agentic-ayudha/internal/initrepo"
)

// AppManifestStart is the page that posts the manifest to GitHub when the
// browser is on another device (#578). It reads everything from the link's
// fragment, which no server receives, and posts only to github.com.
const AppManifestStart = "https://rahulnakmol.github.io/agentic-ayudha/app-manifest.html"

// codeWait is how long a pasted address can take: GitHub's limit for the
// whole manifest flow.
var codeWait = time.Hour

// loopbackWait is how long the local redirect can take.
var loopbackWait = 10 * time.Minute

var manifestCodeRE = regexp.MustCompile(`^[A-Za-z0-9_-]{8,128}$`)

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

// manifestCode runs the GitHub App manifest flow (init --identity). Where the
// browser is on this machine, a local page posts the manifest and GitHub
// redirects back to a local server with the one-time code. Anywhere else (an
// orb's Terminal tab, a cloud agent's terminal, SSH, a phone), the person opens
// an HTTPS link on any device, GitHub redirects to a github.com page, and the
// person pastes that page's address here (#578).
func (e *Env) manifestCode(ctx context.Context, postURL string, manifest func(redirect string) string) (string, error) {
	if localBrowser(e.Getenv, runtime.GOOS) {
		return e.manifestLoopback(ctx, postURL, manifest)
	}
	return e.manifestPaste(ctx, postURL, manifest)
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

// manifestPaste is the flow for a browser on another device. The link's
// fragment carries the manifest; GitHub's redirect lands on github.com
// (initrepo.AppCodePage), so the code reaches no page aayu does not control.
func (e *Env) manifestPaste(ctx context.Context, postURL string, manifest func(redirect string) string) (string, error) {
	if !e.isTerminal(e.In) {
		return "", WithNext(Errorf(ExitBlocked, "init --identity asks you to paste an address from your browser, so it needs a terminal"), "aayu init --identity")
	}
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	state := hex.EncodeToString(b)
	frag := url.Values{"to": {strings.TrimPrefix(postURL, "https://github.com/")}, "state": {state}, "manifest": {manifest(initrepo.AppCodePage)}}.Encode()
	fmt.Fprintf(e.Err, "Open this link on any device, your phone too, to create the App on GitHub:\n\n  %s#%s\n\n"+
		"GitHub then shows an aayudha page. Copy that page's whole address and paste it here (waiting up to an hour).\nAddress: ", AppManifestStart, frag)
	lines := make(chan string)
	go func() {
		defer close(lines)
		r := bufio.NewReader(e.In)
		for {
			l, err := r.ReadString('\n')
			if strings.TrimSpace(l) != "" {
				lines <- l
			}
			if err != nil {
				return
			}
		}
	}()
	ctx, cancel := context.WithTimeout(ctx, codeWait)
	defer cancel()
	for {
		select {
		case l, ok := <-lines:
			if !ok {
				return "", WithNext(Errorf(ExitBlocked, "init --identity: no address was pasted, so no App was created"), "aayu init --identity")
			}
			code, why := pastedCode(l, state)
			if why == "" {
				return code, nil
			}
			fmt.Fprintf(e.Err, "%s\nAddress: ", why)
		case <-ctx.Done():
			return "", WithNext(Errorf(ExitBlocked, "init --identity: no address was pasted within an hour, GitHub's limit for creating the App"), "aayu init --identity")
		}
	}
}

// pastedCode reads GitHub's one-time code from the address the person
// pasted, or says why it cannot be used. Only an address carrying this run's
// state counts: another run's code, or a code from a link someone else sent,
// is never converted.
func pastedCode(line, state string) (code, why string) {
	u, err := url.Parse(strings.TrimSpace(line))
	if err != nil || u.Query().Get("code") == "" {
		return "", "That is not the address of the page GitHub showed: it carries no code."
	}
	q := u.Query()
	if q.Get("state") != state {
		return "", "That address belongs to another run of init --identity (its state is not this run's), so it was not used."
	}
	if !manifestCodeRE.MatchString(q.Get("code")) {
		return "", "That address's code is not one GitHub writes, so it was not used."
	}
	return q.Get("code"), ""
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
		return "", WithNext(Errorf(ExitBlocked, "init --identity: GitHub did not redirect back with the App code within 10 minutes; if your browser is on another device, run it again with AAYU_REMOTE_BROWSER=1 for a link that works on any device"), "aayu init --identity")
	}
}
