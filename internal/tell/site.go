package tell

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"

	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

// SitePage is one page of the docs site (tell docs --site, WP8.2).
type SitePage struct {
	Out      string // output path in the site, e.g. "index.html" or "skills/grill.html"
	Nav      string // sidebar label
	Group    string // sidebar group: "", "Project", or "Skills"
	Source   string // repository path the Markdown is read as; relative links resolve from it
	Origin   string // repository path the page is generated from, linked in its footer
	Markdown string
}

// Site is the docs site of Repo at commit Ref (committed on Date).
type Site struct {
	Repo, Ref, Date string
	Proven          map[int]Feature
	Brand           Brand
	UTM             UTM
	Pages           []SitePage
}

// sitePages are the repository documents the site carries, in sidebar order,
// each included when the checkout has it. The verbs reference and the skill
// pages come from the binary's own registry and embedded skills.
var sitePages = []SitePage{
	{Out: "index.html", Nav: "Overview", Source: "README.md"},
	{Out: "getting-started.html", Nav: "Getting started", Source: "docs/getting-started.md"},
	{Out: "verbs.html", Nav: "Verbs", Origin: "internal/verbs/registry.go"},
	{Out: "contributing.html", Nav: "Contributing", Group: "Project", Source: "CONTRIBUTING.md"},
	{Out: "security.html", Nav: "Security", Group: "Project", Source: "SECURITY.md"},
	{Out: "constraints.html", Nav: "Constraints", Group: "Project", Source: "CONSTRAINTS.md"},
	{Out: "layers.html", Nav: "Layers", Group: "Project", Source: "LAYERS.md"},
	{Out: "raci.html", Nav: "RACI", Group: "Project", Source: "docs/RACI.md"},
}

// SitePages reads the site's pages from the checkout at root and the binary's
// assets (skills/<verb>/SKILL.md). README.md is required: it is the home page,
// followed by the Features the site may claim.
func SitePages(root string, assets fs.FS, proven map[int]Feature) ([]SitePage, error) {
	var pages []SitePage
	for _, p := range sitePages {
		if p.Out == "verbs.html" {
			p.Source, p.Markdown = "verbs.md", verbsMarkdown(assets)
			pages = append(pages, p)
			continue
		}
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(p.Source)))
		if errors.Is(err, fs.ErrNotExist) && p.Out != "index.html" {
			continue
		} else if err != nil {
			return nil, err
		}
		p.Origin, p.Markdown = p.Source, string(b)
		if p.Out == "index.html" {
			p.Markdown = strings.TrimRight(p.Markdown, "\n") + "\n\n" + provenMarkdown(proven)
		}
		pages = append(pages, p)
	}
	for _, c := range verbs.Verbs {
		b, err := fs.ReadFile(assets, "skills/"+c.Name+"/SKILL.md")
		if errors.Is(err, fs.ErrNotExist) {
			continue
		} else if err != nil {
			return nil, err
		}
		pages = append(pages, SitePage{Out: "skills/" + c.Name + ".html", Nav: c.Name, Group: "Skills",
			Source: "skills/" + c.Name + "/SKILL.md", Origin: "skills/" + c.Name + "/SKILL.md", Markdown: stripFrontmatter(string(b))})
	}
	return pages, nil
}

func stripFrontmatter(md string) string {
	if rest, ok := strings.CutPrefix(md, "---\n"); ok {
		if _, body, ok := strings.Cut(rest, "\n---\n"); ok {
			return strings.TrimLeft(body, "\n")
		}
	}
	return md
}

func provenMarkdown(proven map[int]Feature) string {
	var b strings.Builder
	b.WriteString("## Proven Features\n\nA Feature is listed here once every shipped story under it has an authoritative proof pack. ")
	b.WriteString("These are the only work items this site links.\n\n")
	if len(proven) == 0 {
		b.WriteString("None yet.\n")
	}
	ns := make([]int, 0, len(proven))
	for n := range proven {
		ns = append(ns, n)
	}
	sort.Ints(ns)
	for _, n := range ns {
		fmt.Fprintf(&b, "- [%s](%s) (#%d)\n", mdText(proven[n].Title), proven[n].URL, n)
	}
	return b.String()
}

// verbsMarkdown is the verbs reference, generated from the registry.
func verbsMarkdown(assets fs.FS) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Verbs\n\naayudha has %d verbs and %d plumbing commands. This page is generated from the verb registry "+
		"(`internal/verbs/registry.go`), the same source as the CLI, the MCP schemas, and each skill's frontmatter.\n\n", len(verbs.Verbs), len(verbs.Plumbing))
	b.WriteString("| Verb | Accountable seat | Summary |\n|---|---|---|\n")
	for _, c := range verbs.Verbs {
		fmt.Fprintf(&b, "| [`%s`](#%s) | %s | %s |\n", c.Name, c.Name, cell(verbs.SeatName(c.Seat)), cell(c.Summary))
	}
	for _, c := range verbs.Verbs {
		fmt.Fprintf(&b, "\n## `%s`\n\n%s\n\nUsage: `%s`\n\n| Field | Value |\n|---|---|\n", c.Name, c.Summary, usage(c))
		row := func(k, v string) {
			if v != "" {
				fmt.Fprintf(&b, "| %s | %s |\n", k, cell(v))
			}
		}
		row("Accountable seat", verbs.SeatName(c.Seat))
		if c.Owns != "" {
			row("Policy owner", verbs.SeatName(c.Owns))
		}
		row("Model lane", string(c.Lane))
		row("Layers", c.Layers.String())
		row("Writes as", string(c.Identity))
		mode := string(c.Mode)
		if c.ModeNote != "" {
			mode += " (" + c.ModeNote + ")"
		}
		row("Who reasons", mode)
		row("Surfaces", joinAny(c.Surfaces))
		row("Gates", joinAny(c.Gates))
		row("Outputs", strings.Join(c.Outputs, ", "))
		row("Built in", c.WP)
		if _, err := fs.Stat(assets, "skills/"+c.Name+"/SKILL.md"); err == nil {
			fmt.Fprintf(&b, "| Skill | [%s](skills/%s/SKILL.md) |\n", c.Name, c.Name)
		}
		inputsMarkdown(&b, c.Inputs)
	}
	b.WriteString("\n## Plumbing commands\n\nPlumbing commands are not verbs: they have no skill and no model lane.\n\n| Command | Summary | Seat | Inputs |\n|---|---|---|---|\n")
	for _, c := range verbs.Plumbing {
		var in []string
		for _, i := range c.Inputs {
			in = append(in, "`"+inputName(i)+"`")
		}
		fmt.Fprintf(&b, "| `%s` | %s | %s | %s |\n", c.Name, cell(c.Summary), cell(verbs.SeatName(c.Seat)), strings.Join(in, ", "))
	}
	return b.String()
}

func inputsMarkdown(b *strings.Builder, inputs []verbs.Input) {
	if len(inputs) == 0 {
		return
	}
	b.WriteString("\n| Input | Type | Required | Description |\n|---|---|---|---|\n")
	for _, i := range inputs {
		typ := string(i.Type)
		if len(i.Enum) > 0 {
			typ += ": " + strings.Join(i.Enum, ", ")
		}
		req := "no"
		if i.Required {
			req = "yes"
		}
		desc := i.Description
		if i.HumanOnly {
			desc += " Human identity only; never exposed over MCP."
		}
		fmt.Fprintf(b, "| `%s` | %s | %s | %s |\n", inputName(i), cell(typ), req, cell(desc))
	}
}

func inputName(i verbs.Input) string {
	if i.Positional {
		return "<" + i.Name + ">"
	}
	return "--" + i.Name
}

func usage(c verbs.Command) string {
	parts := []string{"aayu", c.Name}
	flags := false
	for _, i := range c.Inputs {
		switch {
		case i.Positional && i.Required:
			parts = append(parts, "<"+i.Name+">")
		case i.Positional:
			parts = append(parts, "[<"+i.Name+">]")
		default:
			flags = true
		}
	}
	if flags {
		parts = append(parts, "[flags]")
	}
	return strings.Join(parts, " ")
}

func joinAny[T ~string](xs []T) string {
	s := make([]string, len(xs))
	for i, x := range xs {
		s[i] = string(x)
	}
	return strings.Join(s, ", ")
}

// cell escapes text for a Markdown table cell.
func cell(s string) string { return strings.ReplaceAll(mdText(s), "|", `\|`) }

// mdText keeps text from opening Markdown links or emphasis.
func mdText(s string) string {
	return strings.NewReplacer(`[`, `\[`, `]`, `\]`, `*`, `\*`, `_`, `\_`, "\n", " ").Replace(s)
}

//go:embed site.css
var siteCSS []byte

var pageTmpl = template.Must(template.New("page").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}} · {{.Name}} docs</title>
<link rel="stylesheet" href="{{.Root}}style.css">
</head>
<body>
<a class="skip" href="#main">Skip to content</a>
<header class="top">
<a class="brand" href="{{.Root}}index.html">{{.Name}} docs</a>
<span class="build">Built from <a href="{{.Commit}}">{{.Short}}</a>{{if .Date}} ({{.Date}}){{end}}</span>
</header>
<div class="layout">
<nav aria-label="Site">
{{range .Groups}}{{if .Name}}<details{{if .Open}} open{{end}}><summary>{{.Name}}</summary>{{end}}<ul>
{{range .Links}}<li><a href="{{.Href}}"{{if .Current}} aria-current="page"{{end}}>{{.Label}}</a></li>
{{end}}</ul>{{if .Name}}</details>{{end}}
{{end}}</nav>
<main id="main">
{{.Body}}
<footer>Generated by <code>aayu tell docs</code> from <a href="{{.Origin}}">{{.OriginPath}}</a>. Links to work items name only Features whose shipped stories all have authoritative proof packs.</footer>
</main>
</div>
</body>
</html>
`))

type navLink struct {
	Href, Label string
	Current     bool
}

type navGroup struct {
	Name  string
	Open  bool
	Links []navLink
}

// Build checks every page with Check and renders the site: one HTML file per
// page and style.css. Any page that claims an unproven Feature or uses an
// avoided word refuses the whole site; the problems name the page.
func (s Site) Build() (map[string][]byte, []int, []Problem, error) {
	var probs []Problem
	cited := map[int]bool{}
	copies := make([]string, len(s.Pages))
	for i, p := range s.Pages {
		d, ps := Check(p.Markdown, s.Repo, s.Proven, s.Brand, s.UTM)
		for _, pr := range ps {
			pr.Page = p.Origin
			probs = append(probs, pr)
		}
		for _, n := range d.Cited {
			cited[n] = true
		}
		copies[i] = d.Copy
	}
	ns := make([]int, 0, len(cited))
	for n := range cited {
		ns = append(ns, n)
	}
	sort.Ints(ns)
	if len(probs) > 0 {
		return nil, ns, probs, nil
	}
	md := goldmark.New(goldmark.WithExtensions(extension.GFM), goldmark.WithParserOptions(parser.WithAutoHeadingID()))
	outOf := map[string]string{}
	for _, p := range s.Pages {
		outOf[p.Source] = p.Out
	}
	blob := "https://github.com/" + s.Repo + "/blob/" + s.Ref + "/"
	_, name, _ := strings.Cut(s.Repo, "/")
	short := s.Ref
	if len(short) > 7 {
		short = short[:7]
	}
	files := map[string][]byte{"style.css": siteCSS}
	for i, p := range s.Pages {
		src := []byte(copies[i])
		doc := md.Parser().Parse(text.NewReader(src), parser.WithContext(parser.NewContext(parser.WithIDs(githubIDs{}))))
		base, err := url.Parse(blob + p.Source)
		if err != nil {
			return nil, ns, nil, err
		}
		title := p.Nav
		err = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
			if !entering {
				return ast.WalkContinue, nil
			}
			switch l := n.(type) {
			case *ast.Heading:
				if l.Level == 1 && title == p.Nav {
					title = strings.Trim(string(l.Lines().Value(src)), " `#")
				}
			case *ast.Link:
				l.Destination = []byte(s.href(p.Out, base, blob, outOf, string(l.Destination), false))
			case *ast.Image:
				l.Destination = []byte(s.href(p.Out, base, blob, outOf, string(l.Destination), true))
			}
			return ast.WalkContinue, nil
		})
		if err != nil {
			return nil, ns, nil, err
		}
		var body bytes.Buffer
		if err := md.Renderer().Render(&body, src, doc); err != nil {
			return nil, ns, nil, err
		}
		root := strings.Repeat("../", strings.Count(p.Out, "/"))
		var groups []navGroup
		for _, q := range s.Pages {
			if len(groups) == 0 || groups[len(groups)-1].Name != q.Group {
				groups = append(groups, navGroup{Name: q.Group})
			}
			g := &groups[len(groups)-1]
			g.Links = append(g.Links, navLink{Href: root + q.Out, Label: q.Nav, Current: q.Out == p.Out})
			g.Open = g.Open || q.Out == p.Out || q.Group == "Project"
		}
		origin, _ := Tag(blob+p.Origin, s.UTM)
		commit, _ := Tag("https://github.com/"+s.Repo+"/commit/"+s.Ref, s.UTM)
		var page bytes.Buffer
		err = pageTmpl.Execute(&page, map[string]any{
			"Title": title, "Name": name, "Root": root, "Commit": commit, "Short": short, "Date": s.Date,
			"Groups": groups, "Body": template.HTML(body.String()), "Origin": origin, "OriginPath": p.Origin, //nolint:gosec // goldmark output with raw HTML omitted
		})
		if err != nil {
			return nil, ns, nil, err
		}
		files[p.Out] = page.Bytes()
	}
	return files, ns, nil, nil
}

// href maps a link of the page at out: a link to another page's source
// becomes a relative link to that page; any other relative link resolves as
// GitHub resolves it from the source file and gets the UTM tags (an image, the
// raw file). Absolute links were tagged by Check.
func (s Site) href(out string, base *url.URL, blob string, outOf map[string]string, dest string, image bool) string {
	u, err := url.Parse(dest)
	if dest == "" || strings.HasPrefix(dest, "#") || err != nil || u.IsAbs() {
		return dest
	}
	r := base.ResolveReference(u)
	if rest, ok := strings.CutPrefix(r.Scheme+"://"+r.Host+r.Path, blob); ok {
		if to, ok := outOf[rest]; ok {
			h := strings.Repeat("../", strings.Count(out, "/")) + to
			if r.Fragment != "" {
				h += "#" + r.Fragment
			}
			return h
		}
		if image {
			return "https://raw.githubusercontent.com/" + s.Repo + "/" + s.Ref + "/" + rest
		}
	}
	t, _ := Tag(r.String(), s.UTM)
	return t
}

// githubIDs gives headings the anchors GitHub gives them, so links written
// for GitHub (CONTRIBUTING.md#promotion-agent-run-no-human-approval) work on
// the site: lower case, letters, digits, '-' and '_' kept, spaces to '-',
// everything else dropped, and -1, -2, ... on repeats.
type githubIDs map[string]int

func (g githubIDs) Generate(value []byte, _ ast.NodeKind) []byte {
	var b strings.Builder
	for _, r := range strings.ToLower(string(value)) {
		switch {
		case unicode.IsLetter(r) || unicode.IsNumber(r) || r == '-' || r == '_':
			b.WriteRune(r)
		case r == ' ':
			b.WriteByte('-')
		}
	}
	id := b.String()
	if id == "" {
		id = "section"
	}
	n := g[id]
	g[id] = n + 1
	if n > 0 {
		id = fmt.Sprintf("%s-%d", id, n)
	}
	return []byte(id)
}

func (g githubIDs) Put(value []byte) { g[string(value)]++ }

// SiteFiles lists a built site's files in a stable order.
func SiteFiles(files map[string][]byte) []string {
	out := make([]string, 0, len(files))
	for f := range files {
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}
