package design

import (
	"image"
	"image/color"
	"image/draw"
	"os"
	"strings"
	"testing"
)

// screen draws a synthetic 400×300 UI: a page background, a card, a header
// bar, and a button, all in token colours.
func screen() *image.NRGBA {
	m := image.NewNRGBA(image.Rect(0, 0, 400, 300))
	fill := func(r image.Rectangle, c color.NRGBA) { draw.Draw(m, r, &image.Uniform{c}, image.Point{}, draw.Src) }
	fill(m.Rect, color.NRGBA{0xf5, 0xf6, 0xf8, 0xff})
	fill(image.Rect(20, 20, 380, 280), color.NRGBA{0xff, 0xff, 0xff, 0xff})
	fill(image.Rect(20, 20, 380, 60), color.NRGBA{0x1b, 0x23, 0x30, 0xff})
	fill(image.Rect(40, 220, 160, 264), color.NRGBA{0x1f, 0x5b, 0xd1, 0xff})
	return m
}

// withBlock repaints a w×h block inside the card in a clearly different colour.
func withBlock(w, h int) *image.NRGBA {
	m := screen()
	draw.Draw(m, image.Rect(200, 100, 200+w, 100+h), &image.Uniform{color.NRGBA{0xbf, 0x28, 0x28, 0xff}}, image.Point{}, draw.Src)
	return m
}

// WP6.5 acceptance: a prototype at 1.5 % pixel difference passes the
// default 2 % tolerance and one at 2.5 % fails. 400×300 = 120000 pixels:
// 1800 differ in a 60×30 block (1.5 %), 3000 in a 60×50 block (2.5 %).
func TestFigmaDiffTolerance(t *testing.T) {
	for _, tc := range []struct {
		name    string
		w, h    int
		percent float64
		pass    bool
	}{
		{"1.5 percent", 60, 30, 1.5, true},
		{"2.0 percent (boundary)", 60, 40, 2.0, true},
		{"2.5 percent", 60, 50, 2.5, false},
	} {
		d, err := Diff(screen(), withBlock(tc.w, tc.h), DefaultThreshold)
		if err != nil {
			t.Fatal(err)
		}
		g := GateResult{State: "list", Diff: d}
		g.Judge(2.0)
		if d.Percent != tc.percent || g.Pass != tc.pass {
			t.Errorf("%s: %.4f %% differ (%d px, %d anti-aliased), pass %v; want %.1f %%, pass %v", tc.name, d.Percent, d.Different, d.AntiAliased, g.Pass, tc.percent, tc.pass)
		}
	}
}

// Anti-aliasing is masked: a hard edge rendered with one row of intermediate
// pixels in one image and none in the other differs only on that edge, and
// none of those pixels count.
func TestDiffMasksAntiAliasing(t *testing.T) {
	a := screen()
	b := screen()
	// Soften the button's top edge in b: one row between the white card and the brand blue.
	draw.Draw(b, image.Rect(40, 219, 160, 220), &image.Uniform{color.NRGBA{0x8f, 0xad, 0xe8, 0xff}}, image.Point{}, draw.Src)
	d, err := Diff(a, b, DefaultThreshold)
	if err != nil {
		t.Fatal(err)
	}
	if d.Different != 0 || d.AntiAliased != 120 {
		t.Fatalf("diff %+v; want 0 counted and 120 anti-aliased", d)
	}
	if _, err := Diff(a, image.NewNRGBA(image.Rect(0, 0, 10, 10)), DefaultThreshold); err == nil {
		t.Fatal("different sizes compared without an error")
	}
}

func tokens(t *testing.T) *Tokens {
	t.Helper()
	b, err := os.ReadFile("../../packages/design-system/tokens/tokens.json")
	if err != nil {
		t.Fatal(err)
	}
	tok, err := ParseTokens(b)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

// WP6.5 acceptance: a non-token colour fails the gate even when no pixel
// differs; typography outside the type scale is counted apart.
func TestNonTokenColourFailsRegardlessOfPixels(t *testing.T) {
	tok := tokens(t)
	styles := []Style{
		{Selector: "body", Colors: []string{"rgb(27, 35, 48)", "rgb(245, 246, 248)"}, FontSize: "14px", FontWeight: "400", FontFamily: `ui-sans-serif, system-ui, -apple-system, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif`},
		{Selector: "div.card", Colors: []string{"rgba(0, 0, 0, 0)", "rgb(255, 255, 255)", "rgb(221, 225, 231)"}},
		{Selector: "div.scrim", Colors: []string{"rgba(27, 35, 48, 0.4)"}},
		{Selector: "button.primary", Colors: []string{"rgb(255, 255, 255)", "rgb(31, 91, 209)"}, FontSize: "14px", FontWeight: "600", FontFamily: "ui-sans-serif, system-ui, -apple-system, \"Segoe UI\", Roboto, \"Helvetica Neue\", Arial, sans-serif"},
	}
	colour, typo := tok.CheckStyles(styles)
	if len(colour)+len(typo) != 0 {
		t.Fatalf("token styles flagged: %v %v", colour, typo)
	}
	d, _ := Diff(screen(), screen(), DefaultThreshold)
	off := append(styles, Style{Selector: "span.badge", Colors: []string{"rgb(18, 52, 86)"}, FontSize: "13px", FontWeight: "500", FontFamily: "Inter"})
	g := GateResult{State: "list", Diff: d}
	g.Colour, g.Typography = tok.CheckStyles(off)
	g.Judge(2.0)
	if g.Pass || d.Percent != 0 {
		t.Fatalf("gate passed with a non-token colour at %.2f %% pixel difference: %+v", d.Percent, g)
	}
	if len(g.Colour) != 1 || g.Colour[0].Value != "#123456" || len(g.Typography) != 3 {
		t.Fatalf("mismatches: colour %v typography %v; want #123456 and font-size, font-weight, font-family", g.Colour, g.Typography)
	}
	if !strings.Contains(g.Summary(), "1 colour and 3 typography token mismatches") {
		t.Fatalf("summary: %s", g.Summary())
	}
}

func TestMockupLiteralsAndFigmaVariables(t *testing.T) {
	tok := tokens(t)
	svg := []byte(`<svg><rect fill="#1F5BD1"/><rect fill="var(--ay-color-surface)"/><text fill="#fff">ok</text><rect stroke="#ff00aa"/></svg>`)
	if got := tok.CheckLiterals("list.svg", svg); len(got) != 1 || got[0].Value != "#ff00aa" {
		t.Fatalf("literals: %v", got)
	}
	colour, typo := tok.CheckVariables([]Variable{
		{Name: "color/brand/default", Type: "COLOR", Value: "#1f5bd1"},
		{Name: "color/text/default", Type: "COLOR", Value: "#000000"},
		{Name: "font/size/body", Type: "FLOAT", Value: "15"},
		{Name: "space/4", Type: "FLOAT", Value: "16"},
	})
	if len(colour) != 1 || colour[0].Where != "figma variable color/text/default" || len(typo) != 1 {
		t.Fatalf("variables: %v %v", colour, typo)
	}
}

func TestParseColor(t *testing.T) {
	for in, want := range map[string]string{"#FFF": "#ffffff", "#1b233066": "#1b233066", "#1b2330ff": "#1b2330", "rgb(31, 91, 209)": "#1f5bd1",
		"rgba(27, 35, 48, 0.4)": "#1b233066", "rgb(31 91 209 / 50%)": "#1f5bd180", "rgba(0, 0, 0, 0)": "", "transparent": ""} {
		got, _ := ParseColor(in)
		if got != want {
			t.Errorf("ParseColor(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFlagAndCodeConnect(t *testing.T) {
	defs := []byte(`{"flags": {
		"orders-v2": {"state": "ENABLED", "variants": {"on": true, "off": false}, "defaultVariant": "off"},
		"loud": {"state": "ENABLED", "variants": {"on": true, "off": false}, "defaultVariant": "on"},
		"copy": {"state": "ENABLED", "variants": {"a": "x", "b": "y"}, "defaultVariant": "a"}}}`)
	if err := CheckFlag(defs, "orders-v2"); err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{"loud": "on by default", "copy": "not a boolean", "absent": "not defined"} {
		if err := CheckFlag(defs, key); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v, want %q", key, err, want)
		}
	}
	mapped := CodeConnect(map[string][]byte{
		"src/Button.figma.tsx": []byte(`figma.connect(Button, "https://www.figma.com/design/AbCdEf123/Aayudha-UI?node-id=1-2&m=dev", {})`),
		"Badge.kt":             []byte(`@FigmaConnect(url = "https://figma.com/file/AbCdEf123/x?node-id=9%3A4")`),
	})
	if got := mapped["AbCdEf123"]; len(got) != 2 || got[0] != "1:2" || got[1] != "9:4" {
		t.Fatalf("code connect: %v", mapped)
	}
	if miss := Unmapped(mapped, "AbCdEf123", []string{"1:2", "3:3"}); len(miss) != 1 || miss[0] != "3:3" {
		t.Fatalf("unmapped: %v", miss)
	}
}
