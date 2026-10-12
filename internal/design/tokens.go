package design

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Tokens is the part of a design-token source the gate compares against:
// every colour value, and the type scale (font sizes in px, weights, and
// font-family stacks).
type Tokens struct {
	Colors   map[string]string  // #rrggbb or #rrggbbaa → token name
	Sizes    map[float64]string // px → token name
	Weights  map[float64]string
	Families map[string]string // normalised stack → token name
	byName   map[string]string // token name → value as written, for Figma variable drift
}

// ParseTokens reads a W3C design-tokens file (tokens.json): leaves are objects
// with $value and an optional $type. Colours are any $type color; sizes and
// weights are the leaves under a font.size or font.weight group; families are
// leaves under a font.family group (at any depth, so web.font.family counts).
func ParseTokens(b []byte) (*Tokens, error) {
	var root map[string]any
	if err := json.Unmarshal(b, &root); err != nil {
		return nil, fmt.Errorf("tokens: %w", err)
	}
	t := &Tokens{Colors: map[string]string{}, Sizes: map[float64]string{}, Weights: map[float64]string{}, Families: map[string]string{}, byName: map[string]string{}}
	var walk func(path []string, m map[string]any)
	walk = func(path []string, m map[string]any) {
		if v, ok := m["$value"]; ok {
			name := strings.Join(path, ".")
			typ, _ := m["$type"].(string)
			t.add(name, typ, v)
			return
		}
		for k, v := range m {
			if sub, ok := v.(map[string]any); ok && !strings.HasPrefix(k, "$") {
				walk(append(append([]string(nil), path...), k), sub)
			}
		}
	}
	walk(nil, root)
	if len(t.Colors) == 0 {
		return nil, fmt.Errorf("tokens: no colour tokens")
	}
	return t, nil
}

func (t *Tokens) add(name, typ string, v any) {
	switch x := v.(type) {
	case string:
		t.byName[name] = x
		if c, ok := ParseColor(x); ok && (typ == "color" || typ == "") {
			t.Colors[c] = name
		}
		if strings.Contains(name, "font.family") {
			t.Families[normFamily(x)] = name
		}
	case float64:
		t.byName[name] = strconv.FormatFloat(x, 'f', -1, 64)
		switch {
		case strings.Contains(name, "font.size"):
			t.Sizes[x] = name
		case strings.Contains(name, "font.weight"):
			t.Weights[x] = name
		}
	}
}

var (
	hexColor = regexp.MustCompile(`^#([0-9a-fA-F]{3,4}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$`)
	rgbColor = regexp.MustCompile(`^rgba?\(\s*([0-9.]+)[\s,]+([0-9.]+)[\s,]+([0-9.]+)(?:[\s,/]+([0-9.]+%?))?\s*\)$`)
)

// ParseColor normalises a CSS colour (#rgb, #rgba, #rrggbb, #rrggbbaa, rgb(),
// rgba()) to lower-case #rrggbb, or #rrggbbaa below full opacity. Fully
// transparent colours and anything else (keywords, gradients) are not colours
// the gate checks: ok is false.
func ParseColor(s string) (string, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	if m := hexColor.FindStringSubmatch(s); m != nil {
		h := m[1]
		if len(h) <= 4 {
			var b strings.Builder
			for _, c := range h {
				b.WriteRune(c)
				b.WriteRune(c)
			}
			h = b.String()
		}
		if len(h) == 8 {
			if h[6:] == "00" {
				return "", false
			}
			if h[6:] == "ff" {
				h = h[:6]
			}
		}
		return "#" + h, true
	}
	m := rgbColor.FindStringSubmatch(s)
	if m == nil {
		return "", false
	}
	var ch [3]int
	for i := range 3 {
		f, err := strconv.ParseFloat(m[i+1], 64)
		if err != nil || f > 255 {
			return "", false
		}
		ch[i] = int(math.Round(f))
	}
	out := fmt.Sprintf("#%02x%02x%02x", ch[0], ch[1], ch[2])
	if a := m[4]; a != "" {
		f, err := strconv.ParseFloat(strings.TrimSuffix(a, "%"), 64)
		if err != nil {
			return "", false
		}
		if strings.HasSuffix(a, "%") {
			f /= 100
		}
		alpha := int(math.Round(f * 255))
		switch {
		case alpha <= 0:
			return "", false
		case alpha < 255:
			out += fmt.Sprintf("%02x", alpha)
		}
	}
	return out, true
}

func normFamily(s string) string {
	parts := strings.Split(s, ",")
	for i, p := range parts {
		parts[i] = strings.ToLower(strings.Trim(strings.TrimSpace(p), `"'`))
	}
	return strings.Join(parts, ",")
}

// Style is the computed style of one rendered element with its own text or
// box, as the capture script samples it.
type Style struct {
	Selector   string   `json:"selector"`
	Colors     []string `json:"colors"` // text, background, visible border and outline colours, as computed
	FontSize   string   `json:"font_size,omitempty"`
	FontWeight string   `json:"font_weight,omitempty"`
	FontFamily string   `json:"font_family,omitempty"`
}

// Mismatch is a value the design tokens do not have.
type Mismatch struct {
	Kind     string `json:"kind"` // colour or typography
	Where    string `json:"where"`
	Value    string `json:"value"`
	Property string `json:"property,omitempty"`
}

func (m Mismatch) String() string {
	if m.Property != "" {
		return fmt.Sprintf("%s %s %s at %s", m.Kind, m.Property, m.Value, m.Where)
	}
	return fmt.Sprintf("%s %s at %s", m.Kind, m.Value, m.Where)
}

// CheckStyles returns every computed colour and type value that is not a
// token value: colour mismatches and typography mismatches, each value
// reported once per selector.
func (t *Tokens) CheckStyles(styles []Style) (colour, typography []Mismatch) {
	seen := map[string]bool{}
	add := func(list *[]Mismatch, m Mismatch) {
		k := m.Kind + m.Property + m.Value + m.Where
		if !seen[k] {
			seen[k] = true
			*list = append(*list, m)
		}
	}
	for _, s := range styles {
		for _, c := range s.Colors {
			hex, ok := ParseColor(c)
			if ok {
				if _, tok := t.Colors[hex]; !tok {
					add(&colour, Mismatch{Kind: "colour", Where: s.Selector, Value: hex})
				}
			}
		}
		if s.FontSize != "" {
			if px, ok := pxValue(s.FontSize); !ok || t.Sizes[px] == "" {
				add(&typography, Mismatch{Kind: "typography", Property: "font-size", Where: s.Selector, Value: s.FontSize})
			}
		}
		if s.FontWeight != "" {
			if w, err := strconv.ParseFloat(s.FontWeight, 64); err != nil || t.Weights[w] == "" {
				add(&typography, Mismatch{Kind: "typography", Property: "font-weight", Where: s.Selector, Value: s.FontWeight})
			}
		}
		if s.FontFamily != "" && len(t.Families) > 0 && t.Families[normFamily(s.FontFamily)] == "" {
			add(&typography, Mismatch{Kind: "typography", Property: "font-family", Where: s.Selector, Value: s.FontFamily})
		}
	}
	return colour, typography
}

func pxValue(s string) (float64, bool) {
	f, err := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(s), "px"), 64)
	return f, err == nil
}

// literalColor finds colour values in markup and CSS: after an attribute's
// quote, a declaration's colon, or an argument separator, so text such as
// "#1042" is not read as a colour.
var literalColor = regexp.MustCompile(`[:=("',]\s*(#[0-9a-fA-F]{3,8}\b|rgba?\([^)]*\))`)

// CheckLiterals returns the colour literals in a text artifact (an SVG or HTML
// mockup) that are not token values. CSS custom properties (var(--…)) are
// token references and pass.
func (t *Tokens) CheckLiterals(where string, text []byte) []Mismatch {
	var out []Mismatch
	seen := map[string]bool{}
	for _, m := range literalColor.FindAllStringSubmatch(string(text), -1) {
		hex, ok := ParseColor(m[1])
		if !ok || seen[hex] {
			continue
		}
		seen[hex] = true
		if _, tok := t.Colors[hex]; !tok {
			out = append(out, Mismatch{Kind: "colour", Where: where, Value: hex})
		}
	}
	return out
}

// Variable is a Figma variable reduced to what the drift check needs.
type Variable struct {
	Name  string // Figma's slash path, e.g. color/text/default
	Type  string // COLOR, FLOAT, STRING
	Value string
}

// CheckVariables compares Figma variables with the tokens of the same name
// (color/text/default ↔ color.text.default): a different value is a token
// mismatch, a colour one for COLOR variables, a typography one for font
// variables. Variables without a token of that name are not compared.
func (t *Tokens) CheckVariables(vars []Variable) (colour, typography []Mismatch) {
	for _, v := range vars {
		name := strings.ReplaceAll(v.Name, "/", ".")
		want, ok := t.byName[name]
		if !ok {
			continue
		}
		m := Mismatch{Where: "figma variable " + v.Name, Value: v.Value + " (token " + want + ")"}
		switch {
		case v.Type == "COLOR":
			a, _ := ParseColor(v.Value)
			b, _ := ParseColor(want)
			if a != b {
				m.Kind = "colour"
				colour = append(colour, m)
			}
		case strings.HasPrefix(name, "font."):
			if !sameNumberOrText(v.Value, want) {
				m.Kind = "typography"
				typography = append(typography, m)
			}
		}
	}
	sort.Slice(colour, func(i, j int) bool { return colour[i].Where < colour[j].Where })
	sort.Slice(typography, func(i, j int) bool { return typography[i].Where < typography[j].Where })
	return colour, typography
}

func sameNumberOrText(a, b string) bool {
	x, e1 := strconv.ParseFloat(a, 64)
	y, e2 := strconv.ParseFloat(b, 64)
	if e1 == nil && e2 == nil {
		return x == y
	}
	return normFamily(a) == normFamily(b)
}
