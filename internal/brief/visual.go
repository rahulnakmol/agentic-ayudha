package brief

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"html"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Figure is a rendered visual.
type Figure struct {
	N         int        `json:"figure"`
	Kind      string     `json:"kind"`
	Alt       string     `json:"alt"`
	Caption   string     `json:"caption,omitempty"`
	Source    string     `json:"source,omitempty"` // diagram or illustration file, or pack artifact
	SHA256    string     `json:"sha256,omitempty"` // of the image or diagram source
	Generator string     `json:"generator,omitempty"`
	Pack      string     `json:"pack,omitempty"`
	Chart     *ChartData `json:"chart,omitempty"`
	SVG       string     `json:"-"` // static SVG (diagrams and charts), written beside the page
	HTML      string     `json:"-"` // the figure in the page
}

// Sources opens what visuals point at.
type Sources struct {
	// ReadFile reads diagram and illustration files; nil reads from disk.
	ReadFile func(path string) ([]byte, error)
	// Artifact reads a pack's artifact (annotated screens); nil means packs
	// cannot be opened here.
	Artifact func(ctx context.Context, p Pack, path string) ([]byte, error)
}

const maxImageBytes = 5 << 20

var imageTypes = []string{"image/png", "image/jpeg", "image/webp", "image/gif"}

// Annotation is one numbered mark on a screen, at percent coordinates.
type Annotation struct {
	X, Y  float64
	Label string
}

var annotationRE = regexp.MustCompile(`^(?:[-*]\s+)?(\d+(?:\.\d+)?)\s*,\s*(\d+(?:\.\d+)?)\s*:\s*(.+)$`)

// ParseAnnotations reads `x,y: label` lines (percent of width and height).
func ParseAnnotations(body string) ([]Annotation, error) {
	var out []Annotation
	for _, l := range strings.Split(body, "\n") {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		m := annotationRE.FindStringSubmatch(l)
		if m == nil {
			return nil, fmt.Errorf("annotation %q: want `x,y: label` in percent of the screen", l)
		}
		x, _ := strconv.ParseFloat(m[1], 64)
		y, _ := strconv.ParseFloat(m[2], 64)
		if x > 100 || y > 100 {
			return nil, fmt.Errorf("annotation %q: x and y are percentages from 0 to 100", l)
		}
		out = append(out, Annotation{x, y, strings.TrimSpace(m[3])})
	}
	return out, nil
}

// checkVisual is the static check of one visual block.
func checkVisual(v *Visual, d *Draft, g *Grounding) []string {
	var out []string
	if p := CheckAlt(v.Attrs["alt"]); p != "" {
		out = append(out, p)
	}
	switch v.Kind {
	case "diagram":
		src := v.Attrs["src"]
		switch {
		case src != "" && v.Body != "":
			out = append(out, "give the diagram inline or as src, not both")
		case src != "" && !strings.HasSuffix(src, ".diagram"):
			out = append(out, fmt.Sprintf("src %q: a diagram source is a *.diagram file", src))
		case src == "" && v.Body == "":
			out = append(out, "the diagram is empty: write it after the header lines or name a *.diagram file as src")
		case src == "":
			if _, err := ParseDiagram(v.Body); err != nil {
				out = append(out, err.Error())
			}
		}
	case "chart":
		if _, ok := g.snapshot(v.Attrs["snapshot"]); !ok {
			var ids []string
			for _, s := range g.Snapshots {
				ids = append(ids, s.ID+" ("+s.File+")")
			}
			out = append(out, fmt.Sprintf("snapshot %q is not a measure snapshot of this brief's grounding; have %s", v.Attrs["snapshot"], orNone(ids)))
		}
		if k := v.Attrs["kind"]; k != "" && k != "bar" && k != "line" {
			out = append(out, fmt.Sprintf("kind %q: bar or line", k))
		}
		if _, err := CheckQuery(v.Body); err != nil {
			out = append(out, err.Error())
		}
	case "illustration":
		if v.Attrs["src"] == "" {
			out = append(out, "src names the generated image file")
		}
		if v.Attrs["generator"] == "" {
			out = append(out, "generator names the tool that made the image (for example Painter or Gemini); an illustration is always labelled as generated")
		}
	case "screen":
		p, ok := g.pack(v.Attrs["pack"])
		if !ok {
			out = append(out, fmt.Sprintf("pack %q is not a pack of this brief's grounding", v.Attrs["pack"]))
			break
		}
		if !slices.ContainsFunc(p.Artifacts, func(a Artifact) bool { return a.Path == v.Attrs["artifact"] && a.Kind == "screenshot" }) {
			out = append(out, fmt.Sprintf("artifact %q is not a screenshot in pack %s", v.Attrs["artifact"], p.ID))
		}
		if _, err := ParseAnnotations(v.Body); err != nil {
			out = append(out, err.Error())
		}
	}
	return out
}

func orNone(xs []string) string {
	if len(xs) == 0 {
		return "none"
	}
	return strings.Join(xs, ", ")
}

// Prepare resolves every visual of a checked draft: reads diagram files and
// illustrations, runs chart queries on their snapshots, and opens screens
// from their packs. A visual that cannot be resolved is a finding.
func Prepare(ctx context.Context, d *Draft, g *Grounding, src Sources) ([]*Figure, []Finding) {
	read := src.ReadFile
	if read == nil {
		read = os.ReadFile
	}
	rel := func(p string) string {
		if filepath.IsAbs(p) {
			return p
		}
		return filepath.Join(d.Dir, filepath.FromSlash(p))
	}
	var figs []*Figure
	var out []Finding
	for _, v := range d.Visuals {
		f, err := prepare(ctx, v, g, src, read, rel)
		if err != nil {
			out = append(out, Finding{"visual", fmt.Sprintf("figure %d (%s): %v", v.N, v.Kind, err)})
			continue
		}
		figs = append(figs, f)
	}
	return figs, out
}

func prepare(ctx context.Context, v *Visual, g *Grounding, src Sources, read func(string) ([]byte, error), rel func(string) string) (*Figure, error) {
	f := &Figure{N: v.N, Kind: v.Kind, Alt: strings.TrimSpace(v.Attrs["alt"]), Caption: v.Attrs["caption"]}
	id := fmt.Sprintf("fig-%d", v.N)
	e := html.EscapeString
	switch v.Kind {
	case "diagram":
		body := v.Body
		if s := v.Attrs["src"]; s != "" {
			b, err := read(rel(s))
			if err != nil {
				return nil, err
			}
			body, f.Source = string(b), s
		}
		dg, err := ParseDiagram(body)
		if err != nil {
			return nil, err
		}
		f.SHA256 = sum([]byte(body))
		title := firstOf(v.Attrs["title"], dg.Title, f.Caption)
		f.SVG = xmlHeader + dg.SVG(id, f.Alt, false) + "\n"
		var list strings.Builder
		for _, line := range strings.Split(strings.TrimSuffix(dg.Text(), "."), "; ") {
			fmt.Fprintf(&list, "<li>%s</li>", e(line))
		}
		f.HTML = fmt.Sprintf(`<figure class="visual diagram" id="%s"><div class="canvas">%s</div>`+
			`<p class="diagram-note" aria-live="polite">Select a box to see its note; its connections stay highlighted.</p>`+
			`<figcaption><span class="fig-n">Figure %d.</span> %s</figcaption>`+
			`<details><summary>Diagram as text</summary><ul>%s</ul></details></figure>`,
			id, dg.SVG(id, firstOf(title, f.Alt), true), v.N, e(firstOf(f.Caption, title, f.Alt)), list.String())
		if title != "" && title != f.Alt {
			// The interactive group is named by the title; the takeaway is the alt, said first.
			f.HTML = strings.Replace(f.HTML, `<div class="canvas">`, `<div class="canvas"><p class="visually-hidden">`+e(f.Alt)+`</p>`, 1)
		}
	case "chart":
		s, _ := g.snapshot(v.Attrs["snapshot"])
		if s.Path == "" {
			return nil, fmt.Errorf("snapshot %s (%s) is not on this machine; run `aayu measure` here or fetch it", s.ID, s.File)
		}
		if got, err := fileSum(s.Path); err != nil || got != s.SHA256 {
			return nil, fmt.Errorf("snapshot %s: SHA-256 %s does not match the grounding's %s", s.File, short(got), short(s.SHA256))
		}
		q, _ := CheckQuery(v.Body)
		cols, rows, err := Query(ctx, s.Path, q)
		if err != nil {
			return nil, err
		}
		c := &ChartData{Figure: v.N, Source: s.ID, Snapshot: s.File, SHA256: s.SHA256, Query: q, Columns: cols, Rows: rows, Kind: firstOf(v.Attrs["kind"], "bar")}
		if err := c.axes(v.Attrs["x"], v.Attrs["y"]); err != nil {
			return nil, err
		}
		f.Chart, f.Source, f.SHA256 = c, s.File, s.SHA256
		svg := c.SVG(id, f.Alt)
		f.SVG = xmlHeader + svg + "\n"
		var t strings.Builder
		t.WriteString("<table><thead><tr>")
		for _, col := range cols {
			fmt.Fprintf(&t, `<th scope="col">%s</th>`, e(col))
		}
		t.WriteString("</tr></thead><tbody>")
		for _, r := range rows {
			t.WriteString("<tr>")
			for _, x := range r {
				fmt.Fprintf(&t, "<td>%s</td>", e(cell(x)))
			}
			t.WriteString("</tr>")
		}
		t.WriteString("</tbody></table>")
		f.HTML = fmt.Sprintf(`<figure class="visual chart" id="%s"><div class="canvas">%s</div>`+
			`<figcaption><span class="fig-n">Figure %d.</span> %s From snapshot <a class="cite" href="#src-%s">%s</a>.</figcaption>`+
			`<details class="data"><summary>Data and query</summary>%s<p>Snapshot <code>%s</code>, SHA-256 <code>%s</code>. Re-run the query read-only on that file to get these rows again.</p><pre><code>%s</code></pre></details></figure>`,
			id, svg, v.N, e(firstOf(f.Caption, v.Attrs["title"], f.Alt)), e(s.ID), e(s.ID), t.String(), e(s.File), e(s.SHA256), e(q))
	case "illustration":
		b, err := read(rel(v.Attrs["src"]))
		if err != nil {
			return nil, err
		}
		mime, err := image(b, v.Attrs["src"])
		if err != nil {
			return nil, err
		}
		f.Source, f.SHA256, f.Generator = v.Attrs["src"], sum(b), v.Attrs["generator"]
		f.HTML = fmt.Sprintf(`<figure class="visual illustration" id="%s"><img src="data:%s;base64,%s" alt="%s">`+
			`<figcaption><span class="fig-n">Figure %d.</span> <strong class="generated">Generated illustration</strong> made with %s; it explains, it is not evidence. %s</figcaption></figure>`,
			id, mime, base64.StdEncoding.EncodeToString(b), e(f.Alt), v.N, e(f.Generator), e(f.Caption))
	case "screen":
		p, _ := g.pack(v.Attrs["pack"])
		if src.Artifact == nil {
			return nil, fmt.Errorf("pack %s cannot be opened here", p.ID)
		}
		path := v.Attrs["artifact"]
		b, err := src.Artifact(ctx, p, path)
		if err != nil {
			return nil, fmt.Errorf("pack %s %s: %w", p.ID, path, err)
		}
		for _, a := range p.Artifacts {
			if a.Path == path && a.SHA256 != sum(b) {
				return nil, fmt.Errorf("pack %s %s: SHA-256 %s does not match the pack's %s", p.ID, path, short(sum(b)), short(a.SHA256))
			}
		}
		mime, err := image(b, path)
		if err != nil {
			return nil, err
		}
		notes, _ := ParseAnnotations(v.Body)
		f.Source, f.SHA256, f.Pack = path, sum(b), p.ID
		var marks, list strings.Builder
		for i, a := range notes {
			fmt.Fprintf(&marks, `<span class="mark" style="left:%.2f%%;top:%.2f%%" aria-hidden="true">%d</span>`, a.X, a.Y, i+1)
			fmt.Fprintf(&list, `<li>%s</li>`, e(a.Label))
		}
		ol := ""
		if len(notes) > 0 {
			ol = `<ol class="notes">` + list.String() + `</ol>`
		}
		f.HTML = fmt.Sprintf(`<figure class="visual screen" id="%s"><div class="shot"><img src="data:%s;base64,%s" alt="%s">%s</div>`+
			`<figcaption><span class="fig-n">Figure %d.</span> %s Screenshot <code>%s</code> from <a class="cite" href="#src-%s">%s</a>, the %s pack for #%d at <code>%s</code>.</figcaption>%s</figure>`,
			id, mime, base64.StdEncoding.EncodeToString(b), e(f.Alt), marks.String(), v.N, e(f.Caption), e(path), e(p.ID), e(p.ID), e(p.Authority), p.Story, e(short(p.HeadSHA)), ol)
	}
	return f, nil
}

const xmlHeader = `<?xml version="1.0" encoding="UTF-8"?>` + "\n"

// image accepts PNG, JPEG, WebP, and GIF bytes up to 5 MiB that carry no
// Mobbin origin (WP6.4: Mobbin pixels are never kept).
func image(b []byte, name string) (string, error) {
	if len(b) > maxImageBytes {
		return "", fmt.Errorf("%s is %d bytes; images are at most %d", name, len(b), maxImageBytes)
	}
	mime := http.DetectContentType(b)
	if !slices.Contains(imageTypes, mime) {
		return "", fmt.Errorf("%s is %s; images are PNG, JPEG, WebP, or GIF", name, mime)
	}
	if strings.Contains(strings.ToLower(name), "mobbin") || bytes.Contains(bytes.ToLower(b), []byte("mobbin")) {
		return "", fmt.Errorf("%s is an image of Mobbin origin; reference Mobbin by URL or ID instead (WP6.4)", name)
	}
	return mime, nil
}

func sum(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func fileSum(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return sum(b), nil
}

func short(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}

func firstOf(ss ...string) string {
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}
