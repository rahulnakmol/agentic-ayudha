package verify

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/proof"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

func TestPhrasesAndMissing(t *testing.T) {
	if got := Phrases(`the screen reader says "Create order, button" then “No orders yet”`); !slices.Equal(got, []string{"Create order, button", "No orders yet"}) {
		t.Fatalf("quoted: %q", got)
	}
	if got := Phrases("Orders, heading level 1"); !slices.Equal(got, []string{"Orders, heading level 1"}) {
		t.Fatalf("unquoted: %q", got)
	}
	spoken := []string{"web content", "Orders, heading level 1", "button"}
	if m := Missing([]string{"orders,  heading level 1", "Create order, button"}, spoken); !slices.Equal(m, []string{"Create order, button"}) {
		t.Fatalf("missing: %q", m)
	}
}

// WP4.7 accept: a PR touching only internal/ paths skips the floor with
// "skipped: no-L01" in the pack; other skips say why.
func TestScreenReaderSkips(t *testing.T) {
	layers := ReadLayers(filepath.Join("..", ".."))
	r := &Run{Footprint: layers.Footprint([]string{"internal/verify/sr.go", "internal/cli/verify.go"}), Env: func(string) string { return "" },
		Criteria: []workitems.Criterion{{N: 1, Evidence: "sr-transcript", State: "`/`", Expected: `"Create order, button"`}}}
	r.Verify.Environments = map[string]config.Environment{"local": {URL: "http://localhost:3000"}}
	rep := Execute(context.Background(), r, []Floor{ScreenReader{}})
	p, _ := Pack(r, rep, proof.Pack{})
	c, ok := p.Check("screen-reader")
	if !ok || c.Result != proof.Skipped || c.Detail != "skipped: no-L01" || !rep.Hold {
		t.Fatalf("internal-only PR: %+v (footprint %v)", c, r.Footprint)
	}
	r.Footprint = layers.Footprint([]string{"packages/design-system/src/button.tsx"})
	defer func(os string) { srOS = os }(srOS)
	for _, tc := range []struct{ os, want string }{
		{"linux", "needs a macOS (VoiceOver) or Windows (NVDA) runner"},
		{"darwin", "runs on the aayudha:screen-reader label or nightly"},
	} {
		srOS, r.CI = tc.os, true
		if ok, why := (ScreenReader{}).Applies(r); ok || !strings.Contains(why, tc.want) {
			t.Errorf("%s: applies %t %q", tc.os, ok, why)
		}
	}
	srOS, r.Labels = "darwin", []string{SRLabel}
	if ok, why := (ScreenReader{}).Applies(r); !ok {
		t.Fatalf("labelled L01 PR on macOS: %s", why)
	}
}

// WP4.7 accept with a real screen reader (workflow screen-reader, macOS and
// Windows runners, on label or nightly): an unlabeled button fails against
// the phrase the Ready snapshot expects; the labelled button passes.
func TestScreenReaderFloorReal(t *testing.T) {
	if os.Getenv("AAYU_FLOOR_SR") != "1" {
		t.Skip("set AAYU_FLOOR_SR=1 on macOS or Windows after `npx @guidepup/setup setup --ci` and `install`")
	}
	mux := http.NewServeMux()
	page := func(button string) string {
		return `<!doctype html><html lang="en"><head><title>Orders</title></head><body><main><h1>Orders</h1><p>No orders yet</p>` + button + `</main></body></html>`
	}
	mux.HandleFunc("/labelled", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, page(`<button type="button">Create order</button>`))
	})
	mux.HandleFunc("/unlabeled", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, page(`<button type="button"><svg aria-hidden="true" width="24" height="24"><path d="M12 5v14M5 12h14" stroke="black"/></svg></button>`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	for path, want := range map[string]string{"/unlabeled": proof.Fail, "/labelled": proof.Pass} {
		r := browserRun(t, srv.URL)
		r.Footprint = []string{"L01"}
		r.Criteria = []workitems.Criterion{{N: 1, Evidence: "sr-transcript", State: "`" + path + "`", Expected: `"Orders" then "Create order"`}}
		r.Sandbox.Getenv = func(k string) string {
			if k == "NODE_PATH" {
				p, _ := filepath.Abs(filepath.Join("..", "..", "packages", "floors", "node_modules"))
				return p
			}
			return os.Getenv(k)
		}
		rep := Execute(context.Background(), r, []Floor{ScreenReader{}})
		c := checkOf(rep, "screen-reader")
		t.Logf("%s: %s %s", path, c.Result, c.Detail)
		if c.Result != want {
			t.Errorf("%s: %+v", path, c)
		}
		if want == proof.Pass && !slices.ContainsFunc(rep.Artifacts, func(e Evidence) bool { return e.Kind == "sr-transcript" && slices.Equal(e.Criteria, []int{1}) }) {
			t.Errorf("%s: no transcript artifact", path)
		}
	}
}
