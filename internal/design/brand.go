package design

import (
	"bytes"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v4"
)

// BrandPath is where `grill brand` writes the brand, relative to the repository root (WP5.3); `tell` reads
// its voice (WP7.5).
const BrandPath = "design/brand.yaml"

// Brand is design/brand.yaml: the product's name, voice, palette, and the colour pairs the product puts text
// or controls on, each checked against WCAG 2.2 AA.
type Brand struct {
	Version int               `yaml:"version"`
	Name    string            `yaml:"name"`
	Voice   Voice             `yaml:"voice"`
	Palette map[string]string `yaml:"palette"`
	Pairs   []Pair            `yaml:"pairs"`
}

// Voice is the brand's tone, which `tell` writes in.
type Voice struct {
	Tone  []string `yaml:"tone"`
	Avoid []string `yaml:"avoid,omitempty"`
}

// Pair is a foreground palette colour used on a background palette colour.
type Pair struct {
	FG  string `yaml:"fg" json:"fg"`
	BG  string `yaml:"bg" json:"bg"`
	Use string `yaml:"use" json:"use"` // text, large-text, or ui
}

// AA minimum contrast ratios per use: WCAG 2.2 SC 1.4.3 (text 4.5:1, large text 3:1) and SC 1.4.11 (user
// interface components 3:1).
var AA = map[string]float64{"text": 4.5, "large-text": 3, "ui": 3}

// PairResult is one pair's AA check.
type PairResult struct {
	Pair
	FGHex string  `json:"fg_hex"`
	BGHex string  `json:"bg_hex"`
	Ratio float64 `json:"ratio"`
	Min   float64 `json:"min"`
	Pass  bool    `json:"pass"`
}

func (r PairResult) String() string {
	verdict := "passes"
	if !r.Pass {
		verdict = "fails"
	}
	return fmt.Sprintf("%s (%s) on %s (%s) for %s: %.2f:1 %s AA (%.1f:1)", r.FG, r.FGHex, r.BG, r.BGHex, r.Use, r.Ratio, verdict, r.Min)
}

// ParseBrand reads a brand file strictly: unknown keys are errors.
func ParseBrand(b []byte) (*Brand, error) {
	var br Brand
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(&br); err != nil {
		return nil, fmt.Errorf("brand: %w", err)
	}
	return &br, nil
}

// Marshal renders the brand as YAML.
func (b *Brand) Marshal() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("# Brand written by `aayu grill brand` (WP5.3). Every pair is checked against WCAG 2.2 AA.\n")
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(b); err != nil {
		return nil, err
	}
	return buf.Bytes(), enc.Close()
}

// Check validates the brand and runs the AA palette check. problems lists what is missing or malformed;
// a pair below its AA ratio is a problem too, and every pair's result is returned.
func (b *Brand) Check() (results []PairResult, problems []string) {
	if b.Version != 1 {
		problems = append(problems, "version: want 1")
	}
	if strings.TrimSpace(b.Name) == "" {
		problems = append(problems, "name: required")
	}
	if len(b.Voice.Tone) == 0 {
		problems = append(problems, "voice.tone: name at least one tone word (tell writes in it)")
	}
	if len(b.Palette) == 0 {
		problems = append(problems, "palette: name at least two colours as name: \"#rrggbb\"")
	}
	hexes := map[string]string{}
	names := make([]string, 0, len(b.Palette))
	for n := range b.Palette {
		names = append(names, n)
	}
	slices.Sort(names)
	for _, n := range names {
		h, ok := ParseColor(b.Palette[n])
		if !ok || len(h) != 7 {
			problems = append(problems, fmt.Sprintf("palette.%s: %q is not an opaque hex colour (#rgb or #rrggbb)", n, b.Palette[n]))
			continue
		}
		hexes[n] = h
	}
	text := false
	for i, p := range b.Pairs {
		need, ok := AA[p.Use]
		if !ok {
			problems = append(problems, fmt.Sprintf("pairs[%d].use: %q: want text, large-text, or ui", i, p.Use))
			continue
		}
		fg, okF := hexes[p.FG]
		bg, okB := hexes[p.BG]
		if !okF || !okB {
			problems = append(problems, fmt.Sprintf("pairs[%d]: %s on %s: both must be palette colours", i, p.FG, p.BG))
			continue
		}
		text = text || p.Use == "text"
		r := PairResult{Pair: p, FGHex: fg, BGHex: bg, Ratio: Contrast(fg, bg), Min: need}
		r.Ratio = math.Floor(r.Ratio*100) / 100 // never round a failing ratio up to a pass
		r.Pass = r.Ratio >= need
		if !r.Pass {
			problems = append(problems, "AA: "+r.String())
		}
		results = append(results, r)
	}
	if !text {
		problems = append(problems, "pairs: name at least one text pair (fg on bg with use: text)")
	}
	return results, problems
}

// Contrast is the WCAG contrast ratio of two opaque #rrggbb colours, 1 to 21.
func Contrast(a, b string) float64 {
	la, lb := luminance(a), luminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// luminance is the WCAG relative luminance of #rrggbb.
func luminance(hex string) float64 {
	var l float64
	for i, w := range []float64{0.2126, 0.7152, 0.0722} {
		v, _ := strconv.ParseUint(hex[1+2*i:3+2*i], 16, 8)
		c := float64(v) / 255
		if c <= 0.04045 {
			c /= 12.92
		} else {
			c = math.Pow((c+0.055)/1.055, 2.4)
		}
		l += w * c
	}
	return l
}
