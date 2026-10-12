package brief

import (
	"bytes"
	"fmt"
	"html"
	"regexp"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

// Page is what the page says about itself beyond the draft.
type Page struct {
	Register  string
	Purpose   string
	Version   string // aayu version
	Generated time.Time
	Draft     string // SHA-256 of the draft
	Checks    []string
	Review    Review
}

// Review is the adversarial pass (ground rule 5: the binary calls a model
// from another family directly): claims the sources do not support.
type Review struct {
	Model    string          `json:"model,omitempty"`
	Family   string          `json:"family,omitempty"`
	Findings []ReviewFinding `json:"findings"`
	Skipped  string          `json:"skipped,omitempty"` // why it did not run
}

// ReviewFinding is one claim the adversarial pass could not ground.
type ReviewFinding struct {
	Claim   string `json:"claim"`
	Problem string `json:"problem"`
}

// A11yMarker is where the accessibility result goes once the page is checked.
const A11yMarker = `<span data-aayu-a11y>checked after rendering</span>`

// SetA11y writes the accessibility result into a rendered page.
func SetA11y(page []byte, text string) []byte {
	return bytes.Replace(page, []byte(A11yMarker), []byte(`<span data-aayu-a11y>`+html.EscapeString(text)+`</span>`), 1)
}

type figureRenderer struct{ figs map[ast.Node]*Figure }

func (r figureRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindFencedCodeBlock, r.fenced)
}

func (r figureRenderer) fenced(w util.BufWriter, src []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	if f, ok := r.figs[n]; ok {
		_, _ = w.WriteString(f.HTML + "\n")
		return ast.WalkSkipChildren, nil
	}
	fb := n.(*ast.FencedCodeBlock)
	_, _ = w.WriteString("<pre><code>")
	for i := 0; i < fb.Lines().Len(); i++ {
		s := fb.Lines().At(i)
		_, _ = w.WriteString(html.EscapeString(string(s.Value(src))))
	}
	_, _ = w.WriteString("</code></pre>\n")
	return ast.WalkSkipChildren, nil
}

// Labels of registers and purposes on the page.
var (
	RegisterLabel = map[string]string{"eli5": "Plain words (ELI5)", "ste": "Simplified Technical English", "engineer": "Engineer", "regulatory": "Regulatory"}
	PurposeLabel  = map[string]string{"explain": "Explain", "ideate": "Ideate", "innovate": "Innovate", "close": "Close"}
)

// Render renders a checked draft and its prepared figures as one
// self-contained HTML page: no external requests (its Content Security Policy
// allows inline styles, the diagram script, and data: images only), and the
// grounding in the footer.
func Render(d *Draft, g *Grounding, figs []*Figure, p Page) ([]byte, error) {
	byNode := map[ast.Node]*Figure{}
	for _, f := range figs {
		for _, v := range d.Visuals {
			if v.N == f.N {
				byNode[v.node] = f
			}
		}
	}
	md := goldmark.New(goldmark.WithExtensions(extension.Table, extension.Strikethrough),
		goldmark.WithRendererOptions(renderer.WithNodeRenderers(util.Prioritized(figureRenderer{byNode}, 100))))
	var body bytes.Buffer
	if err := md.Renderer().Render(&body, d.Source, d.doc); err != nil {
		return nil, err
	}
	e := html.EscapeString
	var b strings.Builder
	b.WriteString("<!doctype html>\n<html lang=\"en\">\n<head>\n<meta charset=\"utf-8\">\n")
	b.WriteString(`<meta name="viewport" content="width=device-width, initial-scale=1">` + "\n")
	b.WriteString(`<meta http-equiv="Content-Security-Policy" content="default-src 'none'; img-src data:; style-src 'unsafe-inline'; script-src 'unsafe-inline'">` + "\n")
	fmt.Fprintf(&b, "<meta name=\"generator\" content=\"aayu brief %s\">\n<title>%s · brief on #%d</title>\n<style>%s</style>\n</head>\n<body>\n", e(p.Version), e(d.Title), g.Subject.Number, css)

	subject := fmt.Sprintf("#%d %s", g.Subject.Number, g.Subject.Title)
	if g.Subject.URL != "" {
		subject = fmt.Sprintf(`<a href="%s">%s</a>`, e(g.Subject.URL), e(subject))
	} else {
		subject = e(subject)
	}
	auth := len(g.AuthoritativePacks())
	evidence := fmt.Sprintf("%d proof pack(s), %d authoritative", len(g.Packs), auth)
	if g.Result != nil {
		evidence += " · Result"
	}
	fmt.Fprintf(&b, "<header class=\"masthead\">\n<p class=\"kicker\">Brief on %s %s</p>\n<ul class=\"tags\" aria-label=\"About this brief\"><li>Register: %s</li><li>Purpose: %s</li><li>Grounding: %s</li></ul>\n</header>\n",
		e(titleCase(g.Subject.Type)), subject, e(RegisterLabel[p.Register]), e(PurposeLabel[p.Purpose]), e(evidence))
	fmt.Fprintf(&b, "<main id=\"main\">\n<article>\n%s</article>\n</main>\n", citeLinks(body.String()))

	b.WriteString("<footer class=\"grounding\" aria-labelledby=\"grounding-h\">\n<h2 id=\"grounding-h\">Grounding</h2>\n")
	b.WriteString("<p>This brief makes claims only from the sources below; a bracketed ID in the text links to its source. Work item text is shown as its authors wrote it.</p>\n<ol class=\"sources\">\n")
	for _, s := range sourceLines(g, figs) {
		b.WriteString(s)
	}
	b.WriteString("</ol>\n")
	for _, n := range g.Notes {
		fmt.Fprintf(&b, "<p class=\"note\">%s</p>\n", e(n))
	}
	b.WriteString("<h3>Checks</h3>\n<ul class=\"checks\">\n")
	for _, c := range p.Checks {
		fmt.Fprintf(&b, "<li>%s</li>\n", e(c))
	}
	switch {
	case p.Review.Skipped != "":
		fmt.Fprintf(&b, "<li>Adversarial pass: not run (%s).</li>\n", e(p.Review.Skipped))
	case len(p.Review.Findings) == 0:
		fmt.Fprintf(&b, "<li>Adversarial pass: %s (%s family) found no claim the sources do not support.</li>\n", e(p.Review.Model), e(p.Review.Family))
	default:
		fmt.Fprintf(&b, "<li>Adversarial pass: %s found %d claim(s) the sources do not support.</li>\n", e(p.Review.Model), len(p.Review.Findings))
	}
	fmt.Fprintf(&b, "<li>Accessibility: %s</li>\n</ul>\n", A11yMarker)
	fmt.Fprintf(&b, "<p class=\"generated-by\">Written in the host, checked and rendered by <code>aayu brief</code> %s on %s. Draft SHA-256 <code>%s</code>.</p>\n</footer>\n",
		e(p.Version), p.Generated.UTC().Format("2006-01-02 15:04 MST"), e(short(p.Draft)))
	if len(figs) > 0 {
		fmt.Fprintf(&b, "<script>%s</script>\n", script)
	}
	b.WriteString("</body>\n</html>\n")
	return []byte(b.String()), nil
}

func sourceLines(g *Grounding, figs []*Figure) []string {
	e := html.EscapeString
	var out []string
	issue := func(it Issue, role string) {
		state := it.State
		if it.Status != "" {
			state += ", status:" + it.Status
		}
		title := e(it.Title)
		if it.URL != "" {
			title = fmt.Sprintf(`<a href="%s">%s</a>`, e(it.URL), title)
		}
		extra := ""
		if it.BodySHA256 != "" {
			extra = fmt.Sprintf(" Body SHA-256 <code>%s</code>.", e(short(it.BodySHA256)))
		}
		out = append(out, fmt.Sprintf("<li id=\"src-%d\"><span class=\"id\">#%d</span> %s %s: %s (%s).%s</li>\n", it.Number, it.Number, e(role), e(titleCase(it.Type)), title, e(state), extra))
	}
	issue(g.Subject, "Subject:")
	for _, it := range g.Items {
		issue(it, "Under the subject:")
	}
	for _, pk := range g.Packs {
		var floors []string
		for _, c := range pk.Checks {
			floors = append(floors, fmt.Sprintf("%s %s", c.Floor, c.Result))
		}
		who := "pushed by an agent or a human (advisory)"
		if pk.Authority == "authoritative" {
			who = "written by the merge gate"
			if pk.RunID > 0 {
				who += fmt.Sprintf(" in run %d", pk.RunID)
			}
		}
		hold := "floors hold"
		if !pk.FloorsHold {
			hold = "floors do not hold"
		}
		out = append(out, fmt.Sprintf("<li id=\"src-%s\"><span class=\"id\">%s</span> %s proof pack for #%d at <code>%s</code>, %s; %s: %s. Ledger <code>%s</code>, SHA-256 <code>%s</code>.</li>\n",
			e(pk.ID), e(pk.ID), e(titleCase(pk.Authority)), pk.Story, e(short(pk.HeadSHA)), e(who), hold, e(orNone(floors)), e(pk.LedgerPath), e(short(pk.SHA256))))
	}
	if r := g.Result; r != nil {
		first := strings.TrimSpace(strings.SplitN(strings.TrimSpace(r.Text), "\n", 2)[0])
		out = append(out, fmt.Sprintf("<li id=\"src-R\"><span class=\"id\">R</span> %s on #%d: “%s” SHA-256 <code>%s</code>.</li>\n", e(titleCase(r.Field)), r.Issue, e(clip(first, 240)), e(short(r.SHA256))))
	}
	for _, s := range g.Snapshots {
		var used []string
		for _, f := range figs {
			if f.Chart != nil && f.Chart.Source == s.ID {
				used = append(used, fmt.Sprintf("figure %d", f.N))
			}
		}
		u := ""
		if len(used) > 0 {
			u = " Queried by " + strings.Join(used, ", ") + "."
		}
		out = append(out, fmt.Sprintf("<li id=\"src-%s\"><span class=\"id\">%s</span> Measure snapshot <code>%s</code>, SHA-256 <code>%s</code>.%s</li>\n", e(s.ID), e(s.ID), e(s.File), e(short(s.SHA256)), u))
	}
	return out
}

func titleCase(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

var citeHTML = regexp.MustCompile(`\[((?:#\d+|P\d+|R|D\d+)(?:\s*,\s*(?:#\d+|P\d+|R|D\d+))*)\]`)

// citeLinks turns citations in the rendered body into links to the footer.
func citeLinks(s string) string {
	return citeHTML.ReplaceAllStringFunc(s, func(m string) string {
		var links []string
		for _, id := range strings.Split(strings.Trim(m, "[]"), ",") {
			id = strings.TrimSpace(id)
			links = append(links, fmt.Sprintf(`<a class="cite" href="#src-%s" aria-label="source %s">%s</a>`, strings.TrimPrefix(id, "#"), id, id))
		}
		return "[" + strings.Join(links, ", ") + "]"
	})
}

var (
	imgTag   = regexp.MustCompile(`(?is)<img\b[^>]*>`)
	altAttr  = regexp.MustCompile(`(?is)\balt="([^"]*)"`)
	svgImg   = regexp.MustCompile(`(?is)<svg\b[^>]*\brole="img"[^>]*>\s*<title[^>]*>([^<]*)</title>`)
	svgRoles = regexp.MustCompile(`(?is)<svg\b[^>]*\brole="img"`)
)

// AltText checks the rendered page: every <img> has non-empty alt text and
// every SVG image opens with a non-empty <title>. It returns the problems.
func AltText(page []byte) []string {
	var out []string
	for i, tag := range imgTag.FindAll(page, -1) {
		m := altAttr.FindSubmatch(tag)
		if m == nil || strings.TrimSpace(html.UnescapeString(string(m[1]))) == "" {
			out = append(out, fmt.Sprintf("image %d has no alt text", i+1))
		}
	}
	titled := 0
	for _, m := range svgImg.FindAllSubmatch(page, -1) {
		if strings.TrimSpace(string(m[1])) != "" {
			titled++
		}
	}
	if n := len(svgRoles.FindAll(page, -1)); titled < n {
		out = append(out, fmt.Sprintf("%d of %d SVG images have no title", n-titled, n))
	}
	return out
}

const css = `:root{color-scheme:light;--ink:#102a43;--muted:#486581;--line:#d9e2ec;--accent:#0b5fa5;--tint:#f0f4f8;--warn:#8a4b00}
*{box-sizing:border-box}
body{margin:0;font:17px/1.6 system-ui,-apple-system,"Segoe UI",Roboto,sans-serif;color:var(--ink);background:var(--tint)}
.masthead,main,.grounding{max-width:800px;margin:0 auto;padding:0 24px}
.masthead{padding-top:32px}
.kicker{margin:0;color:var(--muted);font-size:.95rem}
.tags{list-style:none;display:flex;flex-wrap:wrap;gap:8px;padding:0;margin:12px 0 0}
.tags li{font-size:.85rem;padding:4px 12px;border-radius:999px;background:#fff;border:1px solid var(--line)}
article{background:#fff;border:1px solid var(--line);border-radius:12px;padding:8px 40px 32px;margin:20px 0;box-shadow:0 1px 2px rgba(16,42,67,.06)}
h1{font-size:2rem;line-height:1.2;margin:28px 0 12px}
h1+p{font-size:1.2rem;line-height:1.5;border-left:4px solid var(--accent);padding:4px 0 4px 16px;margin:0 0 24px}
h2{font-size:1.3rem;margin:32px 0 8px}
a{color:var(--accent)}
a.cite{font-size:.85em;font-weight:600;text-decoration:none;border-bottom:1px solid currentColor}
code{font:.88em ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;background:var(--tint);padding:1px 4px;border-radius:4px}
pre{background:var(--tint);padding:12px;border-radius:8px;white-space:pre-wrap;overflow-wrap:anywhere}
pre code{background:none;padding:0}
figure.visual{margin:28px 0}
.canvas{overflow-x:auto;background:#fff;border:1px solid var(--line);border-radius:8px;padding:12px}
.canvas svg{max-width:100%;height:auto;display:block;margin:0 auto}
figcaption{color:var(--muted);font-size:.95rem;margin-top:8px}
.fig-n{font-weight:600;color:var(--ink)}
.generated{color:var(--warn)}
summary{cursor:pointer;padding:10px 0;color:var(--accent);font-weight:600}
table{border-collapse:collapse;font-size:.9rem;margin:8px 0}
th,td{border:1px solid var(--line);padding:4px 10px;text-align:right}
th{background:var(--tint)}
.shot{position:relative;display:inline-block;max-width:100%}
.shot img{display:block;max-width:100%;border:1px solid var(--line);border-radius:6px}
.mark{position:absolute;transform:translate(-50%,-50%);width:28px;height:28px;border-radius:50%;background:#b84a00;color:#fff;font-weight:700;font-size:14px;display:grid;place-items:center;box-shadow:0 0 0 3px #fff}
.notes{margin:8px 0 0}
.illustration img{display:block;max-width:100%;border-radius:8px;border:1px solid var(--line)}
.diagram-note{font-size:.95rem;background:var(--tint);border-radius:6px;padding:6px 10px;margin:8px 0 0}
.visually-hidden{position:absolute;width:1px;height:1px;overflow:hidden;clip:rect(0 0 0 0);white-space:nowrap}
svg.aayu-diagram .node{cursor:pointer}
svg.aayu-diagram .node:focus{outline:none}
svg.aayu-diagram .node:focus rect,svg.aayu-diagram .node:hover rect{stroke:#b84a00;stroke-width:3}
svg.focused .node:not(.on),svg.focused .edge:not(.on){opacity:.3}
@media (prefers-reduced-motion:no-preference){svg.aayu-diagram .node,svg.aayu-diagram .edge{transition:opacity .15s}}
.grounding{font-size:.92rem;padding-bottom:48px}
.grounding h2{font-size:1.15rem;margin:8px 0}
.grounding h3{font-size:1rem;margin:20px 0 4px}
.sources{padding-left:0;list-style:none}
.sources li{background:#fff;border:1px solid var(--line);border-radius:8px;padding:10px 14px;margin:8px 0}
.sources li:target{outline:3px solid var(--accent)}
.sources .id{font:600 .85rem ui-monospace,Menlo,monospace;background:var(--ink);color:#fff;border-radius:4px;padding:1px 6px;margin-right:6px}
.generated-by,.note{color:var(--muted)}
@media (max-width:640px){article{padding:4px 18px 24px}.masthead,main,.grounding{padding:0 12px}}`

// script makes diagrams interactive: selecting a node (focus, hover, click,
// Enter, or Space) highlights it with its edges and neighbours and shows its
// note; Escape or leaving clears it.
const script = `document.querySelectorAll("svg.aayu-diagram").forEach(function(svg){
var fig=svg.closest("figure"),note=fig.querySelector(".diagram-note"),dflt=note.textContent;
var nodes=svg.querySelectorAll(".node"),edges=svg.querySelectorAll(".edge");
function clear(){svg.classList.remove("focused");nodes.forEach(function(n){n.classList.remove("on")});edges.forEach(function(e){e.classList.remove("on")});note.textContent=dflt}
function show(n){clear();var id=n.getAttribute("data-id");svg.classList.add("focused");n.classList.add("on");
edges.forEach(function(e){var f=e.getAttribute("data-from"),t=e.getAttribute("data-to");if(f!==id&&t!==id)return;e.classList.add("on");
var o=svg.querySelector('.node[data-id="'+CSS.escape(f===id?t:f)+'"]');if(o)o.classList.add("on")});
note.textContent=n.getAttribute("aria-label")}
nodes.forEach(function(n){n.addEventListener("focus",function(){show(n)});n.addEventListener("mouseenter",function(){show(n)});n.addEventListener("click",function(){show(n)});
n.addEventListener("keydown",function(ev){if(ev.key==="Enter"||ev.key===" "){ev.preventDefault();show(n)}if(ev.key==="Escape"){n.blur();clear()}});n.addEventListener("blur",clear)});
svg.addEventListener("mouseleave",function(){if(!svg.contains(document.activeElement))clear()})});`
