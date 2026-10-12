package uireview

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/verify"
)

// fixtureSite serves testdata/<name>.html at /<name>, plus any extra pages.
func fixtureSite(t testing.TB, extra map[string][]byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := filepath.Base(r.URL.Path)
		b, ok := extra[name]
		if !ok {
			var err error
			if b, err = os.ReadFile(filepath.Join("testdata", name+".html")); err != nil {
				http.NotFound(w, r)
				return
			}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(b)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func testBrowser(t testing.TB) *Browser {
	t.Helper()
	nodePath := os.Getenv("AAYU_FLOOR_NODE_PATH")
	if nodePath == "" {
		nodePath, _ = filepath.Abs(filepath.Join("..", "..", "packages", "floors", "node_modules"))
	}
	get := func(k string) string {
		if k == "NODE_PATH" {
			return nodePath
		}
		return os.Getenv(k)
	}
	return &Browser{Sandbox: &verify.Sandbox{Home: t.TempDir(), Getenv: get}, Root: t.TempDir(), Work: t.TempDir(), MinTargetPx: 44, Getenv: get}
}

func kinds(l Lane) []string {
	var out []string
	for _, f := range l.Findings {
		if !slices.Contains(out, f.Kind) {
			out = append(out, f.Kind)
		}
	}
	slices.Sort(out)
	return out
}

// laneB captures states and runs lane B on them.
func laneB(t *testing.T, b *Browser, states []State) (Lane, []Capture) {
	t.Helper()
	ctx := context.Background()
	caps, err := b.Capture(ctx, states)
	if err != nil {
		t.Fatal(err)
	}
	imp, impErr, version := map[string][]ImpeccableHit{}, map[string]error{}, ""
	for _, c := range caps {
		imp[c.ID], version, impErr[c.ID] = b.Impeccable(ctx, c.URL)
	}
	return LaneB(caps, imp, impErr, "impeccable "+version), caps
}

// WP6.6 lane B with the real tools (CI job floors-web): each of the three
// planted defects (no focus indicator, 3.8:1 body text, a 32 px target) is
// caught in the one deterministic run; the clean page ships.
func TestLaneBWithBrowser(t *testing.T) {
	if os.Getenv("AAYU_FLOOR_WEB") != "1" {
		t.Skip("set AAYU_FLOOR_WEB=1 with node, playwright, axe-core, impeccable, and Chromium (packages/floors)")
	}
	site := fixtureSite(t, nil)
	b := testBrowser(t)
	l, caps := laneB(t, b, []State{{ID: "planted", URL: site.URL + "/planted"}, {ID: "clean", URL: site.URL + "/clean"}})
	for _, f := range l.Findings {
		t.Logf("%s %s sev %d %s: %s", f.State, f.Kind, f.Severity, f.Source, f.Summary)
	}
	planted, clean := Lane{}, Lane{}
	for _, f := range l.Findings {
		switch f.State {
		case "planted":
			planted.Findings = append(planted.Findings, f)
		case "clean":
			clean.Findings = append(clean.Findings, f)
		}
	}
	for _, k := range []string{"focus-visible", "contrast", "target-size"} {
		if !slices.Contains(kinds(planted), k) {
			t.Errorf("lane B missed the planted %s defect: %v", k, kinds(planted))
		}
	}
	if len(clean.Findings) != 0 || len(l.Errors) != 0 {
		t.Errorf("clean page: findings %+v, errors %v", clean.Findings, l.Errors)
	}
	if caps[0].Rest == "" || caps[0].Focus == "" || caps[0].Aria == "" || caps[0].HTML == "" {
		t.Errorf("capture lacks lane A's inputs: %+v", caps[0])
	}
}
