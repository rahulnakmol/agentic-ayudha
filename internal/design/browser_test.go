package design

import (
	"bytes"
	"context"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/verify"
)

// prototypeServer serves the fixture prototype (testdata/prototype) at
// /orders with the design-system tokens at /tokens.css, and the OpenFeature
// flag definitions with orders-v2 on when flagOn (a preview environment's
// override) or as committed (off).
func prototypeServer(t testing.TB, flagOn bool) *httptest.Server {
	t.Helper()
	page, err := os.ReadFile("testdata/prototype/index.html")
	if err != nil {
		t.Fatal(err)
	}
	css, err := os.ReadFile("../../packages/design-system/src/styles/tokens.css")
	if err != nil {
		t.Fatal(err)
	}
	flags, err := os.ReadFile("testdata/prototype/flags.flagd.json")
	if err != nil {
		t.Fatal(err)
	}
	if flagOn {
		flags = bytes.Replace(flags, []byte(`"defaultVariant": "off"`), []byte(`"defaultVariant": "on"`), 1)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/orders":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(page)
		case "/tokens.css":
			w.Header().Set("Content-Type", "text/css")
			_, _ = w.Write(css)
		case "/flags.flagd.json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(flags)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func browserCapture(t *testing.T) Capture {
	t.Helper()
	nodePath := os.Getenv("AAYU_FLOOR_NODE_PATH")
	if nodePath == "" {
		nodePath, _ = filepath.Abs(filepath.Join("..", "..", "packages", "floors", "node_modules"))
	}
	sb := &verify.Sandbox{Home: t.TempDir(), Getenv: func(k string) string {
		if k == "NODE_PATH" {
			return nodePath
		}
		return os.Getenv(k)
	}}
	return BrowserCapture(sb, t.TempDir(), t.TempDir())
}

// WP6.5 with a real browser (CI job floors-web): the fixture prototype
// renders only with its OpenFeature flag on; captured at the frame size, its
// computed colours and type are all tokens; against the clean capture as the
// frame, a 1.5 % block passes the 2 % gate, a 2.5 % block fails it, and a
// non-token colour fails it with no pixel tolerance spent.
func TestPrototypeCaptureWithBrowser(t *testing.T) {
	if os.Getenv("AAYU_FLOOR_WEB") != "1" {
		t.Skip("set AAYU_FLOOR_WEB=1 with node, playwright, and Chromium (packages/floors)")
	}
	ctx := context.Background()
	capture := browserCapture(t)

	off := prototypeServer(t, false)
	caps, err := capture(ctx, []CaptureState{{ID: "off", URL: off.URL + "/orders", Width: 640, Height: 400}})
	if err != nil || caps[0].Error != "" {
		t.Fatalf("flag off: %v %+v", err, caps)
	}
	if !slices.ContainsFunc(caps[0].Styles, func(s Style) bool { return s.Selector == "p.legacy" }) ||
		slices.ContainsFunc(caps[0].Styles, func(s Style) bool { return strings.HasPrefix(s.Selector, "section.card") }) {
		t.Fatalf("with orders-v2 off the prototype rendered: %+v", caps[0].Styles)
	}

	on := prototypeServer(t, true)
	var states []CaptureState
	for _, v := range []string{"", "block15", "block25", "offtoken"} {
		states = append(states, CaptureState{ID: "v-" + v, URL: on.URL + "/orders?variant=" + v, Width: 640, Height: 400})
	}
	caps, err = capture(ctx, states)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile("../../packages/design-system/tokens/tokens.json")
	tok, err := ParseTokens(b)
	if err != nil {
		t.Fatal(err)
	}
	frame, err := png.Decode(bytes.NewReader(caps[0].data))
	if err != nil || frame.Bounds().Dx() != 640 || frame.Bounds().Dy() != 400 {
		t.Fatalf("clean capture: %v %v", frame.Bounds(), err)
	}
	if !slices.ContainsFunc(caps[0].Styles, func(s Style) bool { return strings.HasPrefix(s.Selector, "section.card") }) {
		t.Fatalf("with orders-v2 on the prototype did not render: %+v", caps[0].Styles)
	}
	want := map[string]struct {
		percent float64
		colour  int
		pass    bool
	}{"v-": {0, 0, true}, "v-block15": {1.5, 0, true}, "v-block25": {2.5, 0, false}, "v-offtoken": {-1, 1, false}}
	for _, c := range caps {
		if c.Error != "" {
			t.Fatalf("%s: %s", c.ID, c.Error)
		}
		shot, _ := png.Decode(bytes.NewReader(c.data))
		g := GateResult{State: c.ID}
		if g.Diff, err = Diff(frame, shot, DefaultThreshold); err != nil {
			t.Fatal(err)
		}
		g.Colour, g.Typography = tok.CheckStyles(c.Styles)
		g.Judge(2.0)
		w := want[c.ID]
		if (w.percent >= 0 && g.Diff.Percent != w.percent) || len(g.Colour) != w.colour || len(g.Typography) != 0 || g.Pass != w.pass {
			t.Errorf("%s: %s", c.ID, g.Summary())
		}
		t.Log(g.Summary())
	}
}
