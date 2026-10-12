package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/design"
	"github.com/rahulnakmol/agentic-ayudha/internal/github/fake"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations/fixture"
)

// designScreen draws a synthetic 400×300 Figma frame in token colours; seed
// moves the button so the two states differ.
func designScreen(seed int) *image.NRGBA {
	m := image.NewNRGBA(image.Rect(0, 0, 400, 300))
	fill := func(r image.Rectangle, c color.NRGBA) { draw.Draw(m, r, &image.Uniform{c}, image.Point{}, draw.Src) }
	fill(m.Rect, color.NRGBA{0xf5, 0xf6, 0xf8, 0xff})
	fill(image.Rect(20, 20, 380, 280), color.NRGBA{0xff, 0xff, 0xff, 0xff})
	fill(image.Rect(20, 20, 380, 60), color.NRGBA{0x1b, 0x23, 0x30, 0xff})
	fill(image.Rect(40+seed*100, 220, 160+seed*100, 264), color.NRGBA{0x1f, 0x5b, 0xd1, 0xff})
	return m
}

func pngBytes(t *testing.T, m image.Image) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, m); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// figmaRenders serves the generated PNGs at the image URLs the recorded
// Figma session returns, refuses any request to them that carries the Figma
// token, and replays everything else.
type figmaRenders struct {
	t       *testing.T
	replay  http.RoundTripper
	renders map[string][]byte
}

func (f figmaRenders) RoundTrip(req *http.Request) (*http.Response, error) {
	switch {
	case req.URL.Hostname() == "127.0.0.1":
		return http.DefaultTransport.RoundTrip(req)
	case req.URL.Host == "figma-alpha-api.s3.us-west-2.amazonaws.com":
		if req.Header.Get("X-Figma-Token") != "" {
			f.t.Errorf("the Figma token was sent to %s", req.URL.Host)
		}
		b, ok := f.renders[req.URL.Path]
		if !ok {
			return &http.Response{StatusCode: 404, Status: "404 Not Found", Body: io.NopCloser(strings.NewReader("")), Request: req}, nil
		}
		return &http.Response{StatusCode: 200, Status: "200 OK", Header: http.Header{"Content-Type": {"image/png"}}, Body: io.NopCloser(bytes.NewReader(b)), Request: req}, nil
	}
	return f.replay.RoundTrip(req)
}

// writeCaptures writes a --captures directory: each state's screenshot and
// the computed styles the capture script would sample.
func writeCaptures(t *testing.T, shots map[string]image.Image, styles []design.Style) string {
	t.Helper()
	dir := t.TempDir()
	var caps []map[string]any
	for _, id := range []string{"list", "empty"} {
		if err := os.WriteFile(filepath.Join(dir, id+".png"), pngBytes(t, shots[id]), 0o644); err != nil {
			t.Fatal(err)
		}
		caps = append(caps, map[string]any{"id": id, "png": id + ".png", "styles": styles})
	}
	b, _ := json.Marshal(caps)
	if err := os.WriteFile(filepath.Join(dir, "captures.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

var tokenStyles = []design.Style{
	{Selector: "body", Colors: []string{"rgb(27, 35, 48)", "rgb(245, 246, 248)"}, FontSize: "14px", FontWeight: "400"},
	{Selector: "header.bar", Colors: []string{"rgb(255, 255, 255)", "rgb(27, 35, 48)"}, FontSize: "20px", FontWeight: "600"},
	{Selector: "button.primary", Colors: []string{"rgb(255, 255, 255)", "rgb(31, 91, 209)"}, FontSize: "14px", FontWeight: "600"},
}

// TestDesignChain is the WP6.5 eval chain on a fixture Feature: the host's
// recorded outputs for research, wireframe, mockup, and prototype
// (testdata/design/orders) go through the four verbs against a fake GitHub, a
// local ledger remote, the recorded Mobbin MCP surface, and a recorded Figma
// REST session with generated frame renders. All four stages land under
// design/1/ on the ledger, each pinning the manifest before it by SHA-256,
// with references as URLs and IDs only. A prototype whose capture uses a
// non-token colour misses the gate (exit 3) and writes nothing.
func TestDesignChain(t *testing.T) {
	g := fake.New(t, "o/r", "User")
	g.AddIssue("Orders list redesign", "### Summary\n\nRedesign the orders list.", "type:feature", "status:in-progress")
	g.AddIssue("A story", "", "type:story")
	dir, bare := ledgerCheckout(t)
	tokens, err := os.ReadFile("../../packages/design-system/tokens/tokens.json")
	if err != nil {
		t.Fatal(err)
	}
	for p, b := range map[string]string{
		"packages/design-system/tokens/tokens.json": string(tokens),
		"flags.flagd.json":                          `{"flags": {"orders-v2": {"state": "ENABLED", "variants": {"on": true, "off": false}, "defaultVariant": "off"}}}`,
		"src/orders/OrderList.figma.tsx": `import figma from "@figma/code-connect"
import { Button } from "@aayudha/design-system"
figma.connect(Button, "https://www.figma.com/design/AbCdEf123/Aayudha-UI?node-id=1-2", { example: () => <Button>New order</Button> })
`,
	} {
		_ = os.MkdirAll(filepath.Join(dir, filepath.Dir(p)), 0o755)
		if err := os.WriteFile(filepath.Join(dir, p), []byte(b), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mob := fixture.ServeMCP(t, "../integrations/mobbin/testdata/mcp.json", "mobbin_oauth_test")
	figmaOpts := func() (Options, *fixture.Replayer) {
		r := &fixture.Replayer{T: t, Rec: fixture.Load(t, "testdata/design/figma.json"), AuthHeader: "X-Figma-Token", AuthValue: "figd_test"}
		tr := figmaRenders{t: t, replay: r, renders: map[string][]byte{
			"/images/orders-list": pngBytes(t, designScreen(0)), "/images/orders-empty": pngBytes(t, designScreen(1))}}
		return Options{GitHubAPI: g.URL(), GitHubGap: -1, Dir: dir, HTTP: &http.Client{Transport: tr}}, r
	}
	env := map[string]string{"AAYU_GITHUB_TOKEN": "human-token", "AAYU_SECRET_FIGMA": "figd_test", "AAYU_SECRET_MOBBIN": "mobbin_oauth_test"}
	common := []string{"--repo", "o/r", "--set", "integrations.figma.secret=figma", "--set", "integrations.figma.scope=read",
		"--set", "integrations.mobbin.secret=mobbin", "--set", "integrations.mobbin.scope=read", "--set", "integrations.mobbin.url=" + mob.URL}
	fx, _ := filepath.Abs("testdata/design/orders")
	plain := Options{GitHubAPI: g.URL(), GitHubGap: -1, Dir: dir}

	if code, out, errOut := run(t, env, plain, append([]string{"wireframe", "1", "--from", fx + "/wireframe"}, common...)...); code != ExitBlocked || !strings.Contains(errOut, "needs the research stage") {
		t.Fatalf("wireframe before research: exit %d: %s%s", code, out, errOut)
	}
	if code, _, errOut := run(t, env, plain, append([]string{"research", "2", "--from", fx + "/research"}, common...)...); code != ExitUsage || !strings.Contains(errOut, "belong to a Feature") {
		t.Fatalf("research on a story: exit %d: %s", code, errOut)
	}

	steps := [][]string{
		{"research", "1", "--from", fx + "/research", "--query", "orders list", "--platform", "ios"},
		{"wireframe", "1", "--from", fx + "/wireframe"},
		{"mockup", "1", "--from", fx + "/mockup"},
	}
	for _, s := range steps {
		if code, out, errOut := run(t, env, plain, append(s, common...)...); code != 0 {
			t.Fatalf("%s: exit %d: %s\n%s", s[0], code, errOut, out)
		}
	}

	// 0.5 % of the list state differs (a 40×15 block): within the 2 % default.
	list := designScreen(0)
	draw.Draw(list, image.Rect(200, 100, 240, 115), &image.Uniform{color.NRGBA{0x4f, 0x5a, 0x6b, 0xff}}, image.Point{}, draw.Src)
	shots := map[string]image.Image{"list": list, "empty": designScreen(1)}

	// A non-token colour fails the gate although the pixels are within tolerance.
	off := append(append([]design.Style(nil), tokenStyles...), design.Style{Selector: "span.badge", Colors: []string{"rgb(18, 52, 86)"}})
	opts, r := figmaOpts()
	code, out, errOut := run(t, env, opts, append([]string{"prototype", "1", "--from", fx + "/prototype", "--captures", writeCaptures(t, shots, off)}, common...)...)
	if code != ExitFloor || !strings.Contains(errOut, "1 colour and 0 typography token mismatches") || !strings.Contains(out, "nothing written") {
		t.Fatalf("prototype with a non-token colour: exit %d: %s\n%s", code, errOut, out)
	}
	r.Done()
	if got := git(t, bare, "ls-tree", "-r", "--name-only", "ledger", "design/1/prototype"); got != "" {
		t.Fatalf("a failed gate wrote the ledger: %s", got)
	}

	opts, r = figmaOpts()
	if code, out, errOut := run(t, env, opts, append([]string{"prototype", "1", "--from", fx + "/prototype", "--captures", writeCaptures(t, shots, tokenStyles)}, common...)...); code != 0 {
		t.Fatalf("prototype: exit %d: %s\n%s", code, errOut, out)
	}
	r.Done()

	manifest := func(stage string) (design.Manifest, string) {
		raw := git(t, bare, "show", "ledger:"+design.ManifestPath(1, stage))
		var m design.Manifest
		if err := json.Unmarshal([]byte(raw), &m); err != nil {
			t.Fatalf("%s manifest: %v", stage, err)
		}
		sum := sha256.Sum256([]byte(raw + "\n")) // git show trims the trailing newline Encode writes
		return m, hex.EncodeToString(sum[:])
	}
	prevSum := ""
	for i, stage := range design.Stages {
		m, sum := manifest(stage)
		if m.Feature != 1 || m.Stage != stage || len(m.References) < 3 {
			t.Fatalf("%s manifest: %+v", stage, m)
		}
		if i == 0 {
			if len(m.Inputs) != 0 || m.Query != "orders list" {
				t.Fatalf("research inputs/query: %+v", m)
			}
		} else if len(m.Inputs) != 1 || m.Inputs[0].Stage != design.Stages[i-1] || m.Inputs[0].SHA256 != prevSum {
			t.Fatalf("%s does not pin %s %s: %+v", stage, design.Stages[i-1], prevSum, m.Inputs)
		}
		for _, ref := range m.References {
			if err := design.CheckReference(ref); err != nil {
				t.Errorf("%s: %v", stage, err)
			}
		}
		prevSum = sum
	}
	proto, _ := manifest(design.Prototype)
	if proto.Flag != "orders-v2" || proto.Captures != "supplied" || len(proto.Figma) != 2 || proto.Figma[0].Name != "Orders / List" ||
		proto.Figma[0].SHA256 != hexSum(pngBytes(t, designScreen(0))) || strings.Join(proto.CodeConnect["AbCdEf123"], ",") != "1:2" {
		t.Fatalf("prototype manifest: %+v", proto)
	}
	if len(proto.Gate) != 3 || !proto.Gate[0].Pass || proto.Gate[0].Diff.Percent != 0.5 || proto.Gate[2].State != "figma-variables" || !proto.Gate[2].Pass {
		t.Fatalf("figma-diff: %+v", proto.Gate)
	}
	// This fixture repository is public: media are pinned by hash only (V3); text lands on the ledger.
	files := git(t, bare, "ls-tree", "-r", "--name-only", "ledger", "design/1")
	for _, want := range []string{"design/1/research/references.json", "design/1/research/files/notes.md", "design/1/wireframe/files/list.svg",
		"design/1/mockup/files/empty.html", "design/1/prototype/figma-diff.json", "design/1/prototype/code-connect/src/orders/OrderList.figma.tsx"} {
		if !strings.Contains(files, want) {
			t.Errorf("ledger lacks %s:\n%s", want, files)
		}
	}
	if strings.Contains(files, ".png") {
		t.Errorf("media on the ledger of a public repository:\n%s", files)
	}
	if msg := git(t, bare, "log", "-1", "--format=%B", "ledger"); !strings.Contains(msg, "design: prototype for #1") || !strings.Contains(msg, "identity:human-fallback") {
		t.Fatalf("ledger commit: %s", msg)
	}
}

func hexSum(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}
