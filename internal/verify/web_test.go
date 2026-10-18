package verify

import (
	"context"
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

func envRun(env string, envs map[string]config.Environment) *Run {
	r := &Run{Env: func(k string) string {
		if k == "AAYU_ENV" {
			return env
		}
		return ""
	}}
	r.Verify.Environments = envs
	return r
}

// WP4.5: AAYU_ENV resolving to production, or a target on a production
// host, is refused.
func TestResolveTargetRefusesProduction(t *testing.T) {
	envs := map[string]config.Environment{
		"local":   {URL: "http://localhost:3000/"},
		"staging": {URL: "https://app.example.com"},
		"prod":    {URL: "https://app.example.com/", Production: true},
	}
	if tg, err := ResolveTarget(envRun("", envs)); err != nil || tg.URL != "http://localhost:3000" || tg.Env != "local" {
		t.Fatalf("default env: %+v %v", tg, err)
	}
	for _, env := range []string{"prod", "staging"} { // staging shares the production host
		if _, err := ResolveTarget(envRun(env, envs)); !errors.Is(err, ErrProduction) {
			t.Errorf("AAYU_ENV=%s: %v", env, err)
		}
	}
	if tg, err := ResolveTarget(envRun("ci", envs)); err != nil || tg.URL != "" {
		t.Fatalf("unknown env: %+v %v", tg, err)
	}
}

func TestScanPII(t *testing.T) {
	text := "Seeded: jane@example.com, ops@acme.test, a@b.invalid.\nReal: jane.doe@gmail.com\nCard 4111 1111 1111 1111 and 4111 1111 1111 1112\nSSN 078-05-1120"
	got := ScanPII(text)
	want := []string{"email address ja***om", "payment card number 41***11", "national ID number 07***20"}
	if !slices.Equal(got, want) {
		t.Fatalf("ScanPII: %q, want %q", got, want)
	}
}

func TestStates(t *testing.T) {
	r := &Run{Criteria: []workitems.Criterion{
		{N: 1, Evidence: "screenshot", State: "`/settings` signed in as the seeded admin"},
		{N: 2, Evidence: "api-trace", State: "`/api/x`"},
		{N: 3, Evidence: "recording", State: "the empty list"},
	}}
	got := States(r, Target{URL: "http://localhost:3000"})
	if len(got) != 2 || got[0].URL != "http://localhost:3000/settings" || got[1].URL != "http://localhost:3000/" || !got[1].Video || got[0].Video {
		t.Fatalf("%+v", got)
	}
}

// fixtureApp serves the pages the browser floors are tested against.
func fixtureApp(t *testing.T) *httptest.Server {
	page := func(body string) string {
		return `<!doctype html><html lang="en"><head><meta charset="utf-8"><title>Fixture</title><meta name="viewport" content="width=device-width">
<style>body{font:16px/1.5 sans-serif;color:#111;background:#fff;margin:2rem} button{min-width:48px;min-height:48px;font-size:1rem}</style></head>
<body><main><h1>Orders</h1>` + body + `</main></body></html>`
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/good", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, page(`<p>No orders yet. Seeded owner: jane@example.com</p><button type="button">Create order</button>`))
	})
	mux.HandleFunc("/strict-csp", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'")
		fmt.Fprint(w, page(`<p>No orders yet.</p><button type="button">Create order</button>`))
	})
	mux.HandleFunc("/contrast", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, page(`<p style="color:#bbb">No orders yet.</p><button type="button">Create order</button>`))
	})
	mux.HandleFunc("/pii", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, page(`<p>Owner: jane.doe@gmail.com</p><button type="button">Create order</button>`))
	})
	mux.HandleFunc("/slow", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, page(`<p id="p">Loading</p><script>setTimeout(()=>{const t=Date.now();while(Date.now()-t<2000){}document.getElementById("p").textContent="Done";},50);</script>`))
	})
	mux.HandleFunc("/api/ok", func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, `{"ok":true}`) })
	mux.HandleFunc("/api/broken", func(w http.ResponseWriter, _ *http.Request) { http.Error(w, `{"ok":false}`, 500) })
	s := httptest.NewServer(mux)
	t.Cleanup(s.Close)
	return s
}

// browserRun is a run against the fixture app whose Completion Goal has one
// screenshot criterion per path.
func browserRun(t *testing.T, app string, paths ...string) *Run {
	t.Helper()
	root := t.TempDir()
	nodePath := os.Getenv("AAYU_FLOOR_NODE_PATH")
	if nodePath == "" {
		nodePath, _ = filepath.Abs(filepath.Join("..", "..", "packages", "floors", "node_modules"))
	}
	r := &Run{Root: root, Work: t.TempDir(), Env: func(string) string { return "" }}
	r.Verify.Environments = map[string]config.Environment{"local": {URL: app}}
	r.Floors.A11y.Standard, r.Floors.A11y.KeyboardOnly, r.Floors.A11y.ReducedMotion = "WCAG 2.2 AA", true, true
	r.Floors.A11y.TextScalePercent, r.Floors.A11y.MinTargetPx = 200, 44
	for i, p := range paths {
		r.Criteria = append(r.Criteria, workitems.Criterion{N: i + 1, Evidence: "screenshot", State: "`" + p + "`", Expected: "it renders"})
	}
	r.Sandbox = &Sandbox{Home: t.TempDir(), Getenv: func(k string) string {
		switch k {
		case "PATH":
			return os.Getenv("PATH")
		case "NODE_PATH":
			return nodePath
		}
		return os.Getenv(k)
	}}
	return r
}

func checkOf(rep Report, floor string) proof.Check {
	for _, c := range rep.Checks {
		if c.Floor == floor {
			return c
		}
	}
	return proof.Check{}
}

// WP4.5 accept with a real browser (CI job floors-web): a planted contrast
// defect fails axe; a planted email in a capture blocks the pack and keeps
// no capture; a clean, seeded page passes every web check; Lighthouse budgets
// hold on the clean page and fail on a page that blocks the main thread.
func TestWebFloorsWithBrowser(t *testing.T) {
	if os.Getenv("AAYU_FLOOR_WEB") != "1" {
		t.Skip("set AAYU_FLOOR_WEB=1 with node, playwright, axe-core, lighthouse, and Chromium (packages/floors)")
	}
	app := fixtureApp(t).URL
	ctx := context.Background()

	rep := Execute(ctx, browserRun(t, app, "/good"), []Floor{A11y{}})
	for _, f := range []string{"a11y", "keyboard", "reduced-motion", "text-scale", "targets", "redaction"} {
		if c := checkOf(rep, f); c.Result != proof.Pass {
			t.Errorf("clean page, %s: %+v", f, c)
		}
	}
	if !rep.Hold || !slices.ContainsFunc(rep.Artifacts, func(e Evidence) bool { return e.Path == "screens/1.png" && slices.Equal(e.Criteria, []int{1}) }) {
		t.Fatalf("clean page: hold %t, artifacts %v", rep.Hold, paths(rep.Artifacts))
	}
	// A capture is marked OCR-scanned (and so may leave the run) only when
	// tesseract read it.
	_, lookErr := exec.LookPath("tesseract")
	for _, e := range rep.Artifacts {
		if e.Kind == "screenshot" && e.OCRScanned != (lookErr == nil) {
			t.Errorf("%s: OCRScanned %t with tesseract installed %t", e.Path, e.OCRScanned, lookErr == nil)
		}
	}

	// A page whose own Content Security Policy forbids injected scripts is still checked by axe.
	if c := checkOf(Execute(ctx, browserRun(t, app, "/strict-csp"), []Floor{A11y{}}), "a11y"); c.Result != proof.Pass {
		t.Errorf("strict-CSP page: %+v", c)
	}

	rep = Execute(ctx, browserRun(t, app, "/good", "/contrast"), []Floor{A11y{}})
	if c := checkOf(rep, "a11y"); c.Result != proof.Fail || !strings.Contains(c.Detail, "color-contrast") || rep.Hold {
		t.Fatalf("planted contrast defect: %+v", c)
	}
	t.Logf("contrast: %s", checkOf(rep, "a11y").Detail)

	rep = Execute(ctx, browserRun(t, app, "/good", "/pii"), []Floor{A11y{}})
	if c := checkOf(rep, "redaction"); c.Result != proof.Blocked || !strings.Contains(c.Detail, "state 2: email address ja***om") || rep.Hold || len(rep.Artifacts) != 0 {
		t.Fatalf("planted email: %+v, artifacts %v", c, paths(rep.Artifacts))
	}
	t.Logf("pii: %s", checkOf(rep, "redaction").Detail)

	r := browserRun(t, app, "/good")
	r.Floors.Perf.MinScore, r.Floors.Perf.MaxTBTMs = 0.5, 600
	if c := checkOf(Execute(ctx, r, []Floor{Perf{}}), "perf"); c.Result != proof.Pass {
		t.Fatalf("perf on the clean page: %+v", c)
	}
	r = browserRun(t, app, "/slow")
	r.Floors.Perf.MinScore, r.Floors.Perf.MaxTBTMs = 0.5, 600
	c := checkOf(Execute(ctx, r, []Floor{Perf{}}), "perf")
	if c.Result != proof.Fail || !strings.Contains(c.Detail, "TBT") {
		t.Fatalf("perf on a blocking page: %+v", c)
	}
	t.Logf("perf: %s", c.Detail)
}

func paths(es []Evidence) []string {
	var out []string
	for _, e := range es {
		out = append(out, e.Path)
	}
	return out
}

// WP4.6 accept with the real tools (CI job floors-dast): a fixture with a
// reflected-XSS route fails the DAST floor and its escaped twin passes; k6
// thresholds hold on a healthy endpoint and fail on a broken one.
func TestDASTAndAPIFloors(t *testing.T) {
	if os.Getenv("AAYU_FLOOR_DAST") != "1" {
		t.Skip("set AAYU_FLOOR_DAST=1 with docker (the pinned ZAP image) and k6")
	}
	srv := fixtureApp(t)
	ctx := context.Background()
	// Two apps whose home page links to a search page; one reflects the query unescaped.
	app := func(escape bool) string {
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
			fmt.Fprint(w, `<!doctype html><html lang="en"><body><a href="/search?q=hello">Search</a></body></html>`)
		})
		mux.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
			q := r.URL.Query().Get("q")
			if escape {
				q = html.EscapeString(q)
			}
			fmt.Fprint(w, `<!doctype html><html lang="en"><body><p>Results for `+q+`</p></body></html>`)
		})
		s := httptest.NewServer(mux)
		t.Cleanup(s.Close)
		return s.URL
	}
	run := func(url string) *Run {
		r := browserRun(t, url)
		// verify "starts" the app (a seed that succeeds) on loopback, so the active rules run (Decision #485, G1).
		r.Verify.DAST, r.Verify.Platforms, r.Verify.Seed = true, []string{"web"}, []string{"true"}
		r.Floors.DAST.SkippedBlocksShip = true
		r.Now = func() time.Time { return time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC) }
		r.Verify.NewFloorsBlockingFrom = "2026-11-14"
		return r
	}
	rep := Execute(ctx, run(app(false)), []Floor{DAST{}})
	if c := checkOf(rep, "dast"); c.Result != proof.Fail || !strings.Contains(c.Detail, "Cross Site Scripting") {
		t.Fatalf("reflected XSS: %+v", c)
	}
	t.Logf("xss: %s", checkOf(rep, "dast").Detail)
	if c := checkOf(Execute(ctx, run(app(true)), []Floor{DAST{}}), "dast"); c.Result != proof.Pass {
		t.Fatalf("escaped route: %+v", c)
	}

	if _, err := exec.LookPath("k6"); err != nil {
		t.Fatal("k6 is not installed")
	}
	for path, want := range map[string]string{"/api/ok": proof.Pass, "/api/broken": proof.Fail} {
		r := browserRun(t, srv.URL)
		script := filepath.Join(r.Root, "contract.js")
		_ = os.WriteFile(script, []byte(`import http from "k6/http";
export const options = { vus: 1, iterations: 5, thresholds: { http_req_failed: ["rate<0.01"], http_req_duration: ["p(95)<1000"] } };
export default function () { http.get(__ENV.BASE_URL + "`+path+`"); }
`), 0o644)
		r.Floors.API.Script = "contract.js"
		c := checkOf(Execute(ctx, r, []Floor{API{}}), "api")
		if c.Result != want {
			t.Fatalf("k6 on %s: %+v", path, c)
		}
		t.Logf("k6 %s: %s %s", path, c.Result, c.Detail)
	}
}
