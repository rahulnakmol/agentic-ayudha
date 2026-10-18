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
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"

	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

// SitePage is one page of the docs site (tell docs --site, WP8.2).
type SitePage struct {
	Out      string // output path in the site, e.g. "index.html" or "skills/grill.html"
	Nav      string // sidebar label
	Group    string // sidebar group, e.g. "Start here", "Your agent", "Clouds", "Project", or "Skills"
	Source   string // repository path the Markdown is read as; relative links resolve from it
	Origin   string // repository path the page is generated from, linked in its footer
	Summary  string // its line in llms.txt; default the first sentence of its first paragraph
	Markdown string
}

// Site is the docs site of Repo at commit Ref (committed on Date).
type Site struct {
	Repo, Ref, Date string
	Proven          map[int]Feature
	Brand           Brand
	UTM             UTM
	Pages           []SitePage
	Media           map[string][]byte // images by repository path (SiteMedia), served from the site
}

// sitePages are the repository documents the site carries, in sidebar order,
// each included when the checkout has it. The guide (docs/guide) comes first:
// its README is the home page, and README.md becomes the reference overview.
// Without a guide, README.md is the home page. The verbs reference and the
// skill pages come from the binary's own registry and embedded skills.
var sitePages = []SitePage{
	{Out: "index.html", Summary: "What aayudha is, the problem it solves, and the real run it was proven on.", Nav: "Welcome", Group: "Start here", Source: "docs/guide/README.md"},
	{Out: "start.html", Summary: "Install aayudha, run the guided tour, file your first story with checkable promises, and ask what to do next.", Nav: "Your first hour", Group: "Start here", Source: "docs/guide/start.md"},
	{Out: "when-to-use-what.html", Summary: "Start from your situation (a vague idea, a bug, a ready pull request, an incident) and find the verb to run.", Nav: "When to use what", Group: "Start here", Source: "docs/guide/when-to-use-what.md"},
	{Out: "adopt.html", Summary: "Three adoption levels, each a complete stopping point, with what each costs and gives.", Nav: "Adopt at your own pace", Group: "Start here", Source: "docs/guide/adopt.md"},
	{Out: "journey.html", Summary: "The real 12 October run, told to you: from a vague goal to a measured release in nine chapters.", Nav: "Prologue: your Monday", Group: "The story", Source: "docs/guide/journey.md"},
	{Out: "journey-1-shape.html", Summary: "Shaping a Goal with a measured baseline and a target, using the grill interview.", Nav: "1. The goal", Group: "The story", Source: "docs/guide/journey-1-shape.md"},
	{Out: "journey-2-break-down.html", Summary: "Breaking the Goal into an Initiative, Features, and small stories with slice and plan.", Nav: "2. The plan", Group: "The story", Source: "docs/guide/journey-2-break-down.md"},
	{Out: "journey-3-ready.html", Summary: "Writing checkable promises for each story and freezing them as Ready.", Nav: "3. The promises", Group: "The story", Source: "docs/guide/journey-3-ready.md"},
	{Out: "journey-4-build.html", Summary: "How the agent builds a Ready story in its own worktree and opens a pull request.", Nav: "4. The build", Group: "The story", Source: "docs/guide/journey-4-build.md"},
	{Out: "journey-5-prove.html", Summary: "How the merge gate, proof packs, and a reviewer of another vendor sent work back until it was right.", Nav: "5. The gate says no", Group: "The story", Source: "docs/guide/journey-5-prove.md"},
	{Out: "journey-6-ship.html", Summary: "How a person's approval and ship merge a proven pull request, with a record of who approved what.", Nav: "6. The yes", Group: "The story", Source: "docs/guide/journey-6-ship.md"},
	{Out: "journey-7-release.html", Summary: "How shipped work moves from dev to a rolling snapshot after a full gate.", Nav: "7. The release", Group: "The story", Source: "docs/guide/journey-7-release.md"},
	{Out: "journey-8-learn.html", Summary: "Measuring the Goal against its baseline, then retro and evolve to learn from the work.", Nav: "8. The number", Group: "The story", Source: "docs/guide/journey-8-learn.md"},
	{Out: "journey-9-explain.html", Summary: "Explaining the work to a boss or auditor with brief and tell, every claim checked.", Nav: "9. The telling", Group: "The story", Source: "docs/guide/journey-9-explain.md"},
	{Out: "built-with-itself.html", Summary: "How aayudha builds and proves itself, and what is still unfinished.", Nav: "Epilogue: built with itself", Group: "The story", Source: "docs/guide/built-with-itself.md"},
	{Out: "unpack.html", Summary: "How a vague request such as \"make it slick\" is unpacked into checkable promises before anything is built.", Nav: "A vague ask", Group: "Guides", Source: "docs/guide/unpack.md"},
	{Out: "guide-review.html", Summary: "What happens when a pull request is ready: the merge gate, the reviewer app, fix rounds, and ship.", Nav: "A pull request is ready", Group: "Guides", Source: "docs/guide/guide-review.md"},
	{Out: "guide-build.html", Summary: "Building a Ready story yourself or handing it to another agent's cloud, and preparing its environment.", Nav: "Hand work to an agent", Group: "Guides", Source: "docs/guide/guide-build.md"},
	{Out: "guide-prove-behaviour.html", Summary: "Proving a behaviour change with a feature map and behaviour checks that drive your product like a person.", Nav: "Prove a behaviour change", Group: "Guides", Source: "docs/guide/guide-prove-behaviour.md"},
	{Out: "guide-loops.html", Summary: "Setting up scheduled and event loops on each agent, and which host can start on which GitHub event.", Nav: "Loops", Group: "Guides", Source: "docs/guide/guide-loops.md"},
	{Out: "guide-solo-founder.html", Summary: "Working alone: what you still do by hand, the solo-mode bypass, the inbox, provisional Decisions, and earned autonomy.", Nav: "Run it alone", Group: "Guides", Source: "docs/guide/guide-solo-founder.md"},
	{Out: "growth.html", Summary: "An illustrative path from a two-founder MVP to a widely used product: team stages, product streams, and what the agent twins carry at each stage.", Nav: "From MVP to scale", Group: "Guides", Source: "docs/guide/growth.md"},
	{Out: "guide-integrations.html", Summary: "When and how to connect PostHog and Sentry read-only: setup, config, key scopes, what each verb reads, and what aayu never does.", Nav: "PostHog and Sentry", Group: "Guides", Source: "docs/guide/guide-integrations.md"},
	{Out: "guide-learn.html", Summary: "Measuring whether a release moved the Goal, running retro, and evolving skills with ratchets.", Nav: "Did it work?", Group: "Guides", Source: "docs/guide/guide-learn.md"},
	{Out: "troubleshooting.html", Summary: "Symptoms, exit codes, and fixes for install, tour, Ready, the merge gate, ship, hand-offs, and loops.", Nav: "Troubleshooting", Group: "Guides", Source: "docs/guide/troubleshooting.md"},
	{Out: "skills/index.html", Summary: "Every verb on its own page: what it is for, the prompt and command, what you get back, its seat and twin, its gates, and what only a person does.", Nav: "Every skill", Group: "Skills", Source: "docs/guide/skills/README.md"},
	{Out: "hosts.html", Nav: "Your agent", Group: "Your agent", Source: "docs/guide/hosts/README.md"},
	{Out: "hosts-claude-code.html", Nav: "Claude Code", Group: "Your agent", Source: "docs/guide/hosts/claude-code.md"},
	{Out: "hosts-codex.html", Nav: "OpenAI Codex", Group: "Your agent", Source: "docs/guide/hosts/codex.md"},
	{Out: "hosts-copilot.html", Nav: "GitHub Copilot", Group: "Your agent", Source: "docs/guide/hosts/copilot.md"},
	{Out: "hosts-amp.html", Nav: "Amp", Group: "Your agent", Source: "docs/guide/hosts/amp.md"},
	{Out: "hosts-cursor.html", Nav: "Cursor", Group: "Your agent", Source: "docs/guide/hosts/cursor.md"},
	{Out: "make-it-yours.html", Summary: "Changing aayu's colours, symbols, and voice in your terminal and docs.", Nav: "Make it yours", Group: "Your agent", Source: "docs/guide/hosts/make-it-yours.md"},
	{Out: "how-it-works.html", Nav: "Overview", Group: "How it works", Source: "docs/guide/how-it-works.md"},
	{Out: "how-architecture.html", Nav: "Architecture", Group: "How it works", Source: "docs/guide/how-architecture.md"},
	{Out: "how-registry.html", Nav: "Verb registry and codegen", Group: "How it works", Source: "docs/guide/how-registry.md"},
	{Out: "how-trust.html", Nav: "Trust and identities", Group: "How it works", Source: "docs/guide/how-trust.md"},
	{Out: "how-ledger.html", Nav: "The ledger branch", Group: "How it works", Source: "docs/guide/how-ledger.md"},
	{Out: "how-proof-packs.html", Nav: "Proof packs", Group: "How it works", Source: "docs/guide/how-proof-packs.md"},
	{Out: "how-merge-gate.html", Nav: "The merge gate", Group: "How it works", Source: "docs/guide/how-merge-gate.md"},
	{Out: "how-floors.html", Nav: "Floors in detail", Group: "How it works", Source: "docs/guide/how-floors.md"},
	{Out: "how-verify-maps.html", Nav: "Feature maps and behaviour checks", Group: "How it works", Source: "docs/guide/how-verify-maps.md"},
	{Out: "how-models.html", Nav: "Model gateway and lanes", Group: "How it works", Source: "docs/guide/how-models.md"},
	{Out: "how-evals.html", Nav: "The eval harness", Group: "How it works", Source: "docs/guide/how-evals.md"},
	{Out: "how-agents.html", Nav: "Hand-offs, environments, loops", Group: "How it works", Source: "docs/guide/how-agents.md"},
	{Out: "how-config-and-audit.html", Nav: "Config, secrets, audit", Group: "How it works", Source: "docs/guide/how-config-and-audit.md"},
	{Out: "how-releases.html", Nav: "Releases and supply chain", Group: "How it works", Source: "docs/guide/how-releases.md"},
	{Out: "how-security.html", Nav: "Security and threat model", Group: "How it works", Source: "docs/guide/how-security.md"},
	{Out: "how-extending.html", Nav: "Extending aayudha", Group: "How it works", Source: "docs/guide/how-extending.md"},
	{Out: "concepts-loop.html", Summary: "The seven steps every change takes, who does what at each, where the gates and floors sit, and how the loops run.", Nav: "The delivery loop", Group: "Concepts", Source: "docs/guide/concepts-loop.md"},
	{Out: "concepts-team.html", Summary: "The fourteen seats and their twins, who answers for what, who owns which layer, which model does the work, and how a seat goes from solo to paired.", Nav: "Your agentic team", Group: "Concepts", Source: "docs/guide/concepts-team.md"},
	{Out: "concepts-seats.html", Nav: "Seats and pairing", Group: "Concepts", Source: "docs/guide/concepts-seats.md"},
	{Out: "concepts-layers.html", Nav: "The nine layers", Group: "Concepts", Source: "docs/guide/concepts-layers.md"},
	{Out: "concepts-proof.html", Nav: "Proof packs", Group: "Concepts", Source: "docs/guide/concepts-proof.md"},
	{Out: "concepts-floors.html", Nav: "Floors", Group: "Concepts", Source: "docs/guide/concepts-floors.md"},
	{Out: "concepts-human-pass.html", Nav: "The human pass and solo mode", Group: "Concepts", Source: "docs/guide/concepts-human-pass.md"},
	{Out: "concepts-ladder.html", Nav: "dev, rolling, main", Group: "Concepts", Source: "docs/guide/concepts-ladder.md"},
	{Out: "overview.html", Nav: "README", Group: "Reference", Source: "README.md"},
	{Out: "reference.html", Nav: "Project reference", Group: "Reference", Source: "docs/README.md"},
	{Out: "getting-started.html", Nav: "Install and connect", Group: "Reference", Source: "docs/getting-started.md"},
	{Out: "verbs.html", Nav: "Verbs", Group: "Reference", Origin: "internal/verbs/registry.go"},
	{Out: "glossary.html", Nav: "Glossary", Group: "Reference", Source: "GLOSSARY.md"},
	{Out: "threat-model.html", Nav: "Threat model", Group: "Reference", Source: "docs/threat-model.md"},
	{Out: "getting-started-azure.html", Nav: "Azure", Group: "Clouds", Source: "docs/getting-started-azure.md"},
	{Out: "getting-started-aws.html", Nav: "AWS", Group: "Clouds", Source: "docs/getting-started-aws.md"},
	{Out: "getting-started-gcp.html", Nav: "Google Cloud", Group: "Clouds", Source: "docs/getting-started-gcp.md"},
	{Out: "contributing.html", Nav: "Contributing", Group: "Project", Source: "CONTRIBUTING.md"},
	{Out: "security.html", Nav: "Security", Group: "Project", Source: "SECURITY.md"},
	{Out: "constraints.html", Nav: "Constraints", Group: "Project", Source: "CONSTRAINTS.md"},
	{Out: "layers.html", Nav: "Layers", Group: "Project", Source: "LAYERS.md"},
	{Out: "raci.html", Nav: "RACI", Group: "Project", Source: "docs/RACI.md"},
}

// openGroups are the sidebar groups shown open on every page; the others open
// on their own pages.
var openGroups = map[string]bool{"Start here": true, "Guides": true, "The story": true}

// SitePages reads the site's pages from the checkout at root and the binary's
// assets (skills/<verb>/SKILL.md). README.md is required: it is the overview,
// followed by the Features the site may claim, and the home page when the
// checkout has no guide.
func SitePages(root string, assets fs.FS, proven map[int]Feature) ([]SitePage, error) {
	var pages []SitePage
	home, skills := false, false
	for _, p := range sitePages {
		if p.Out == "skills/index.html" {
			guide, err := guideSkillPages(root)
			if err != nil {
				return nil, err
			}
			if guide != nil {
				pages, skills = append(pages, guide...), true
			}
			continue
		}
		if p.Out == "verbs.html" {
			p.Source, p.Markdown = "verbs.md", verbsMarkdown(assets)
			pages = append(pages, p)
			continue
		}
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(p.Source)))
		if errors.Is(err, fs.ErrNotExist) && p.Source != "README.md" {
			continue
		} else if err != nil {
			return nil, err
		}
		p.Origin, p.Markdown = p.Source, string(b)
		home = home || p.Out == "index.html"
		if p.Source == "README.md" {
			p.Markdown = strings.TrimRight(p.Markdown, "\n") + "\n\n" + provenMarkdown(proven)
			if !home {
				p.Out, p.Nav = "index.html", "Overview"
			}
		}
		pages = append(pages, p)
	}
	if skills {
		return pages, nil
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

// guideSkillPages are the guide's skill pages (docs/guide/skills): its index,
// then one page per verb in journey-stage order, each at skills/<verb>.html.
// It returns nil when the checkout has no index, so the site falls back to the
// embedded skills.
func guideSkillPages(root string) ([]SitePage, error) {
	const dir = "docs/guide/skills/"
	var pages []SitePage
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(dir+"README.md")))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	for _, p := range sitePages {
		if p.Out == "skills/index.html" {
			p.Origin, p.Markdown = p.Source, string(b)
			pages = append(pages, p)
		}
	}
	for _, st := range verbs.Stages {
		for _, c := range verbs.Verbs {
			if c.Stage != st.ID {
				continue
			}
			src := dir + c.Name + ".md"
			b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(src)))
			if errors.Is(err, fs.ErrNotExist) {
				continue
			} else if err != nil {
				return nil, err
			}
			pages = append(pages, SitePage{Out: "skills/" + c.Name + ".html", Nav: c.Name, Group: "Skills", Source: src, Origin: src, Markdown: string(b)})
		}
	}
	return pages, nil
}

// siteImage matches a Markdown image's destination.
var siteImage = regexp.MustCompile(`!\[[^\]]*\]\(<?([^)\s>]+)`)

// SiteMedia reads the images the pages show from the checkout at root, keyed
// by repository path, so the site serves them itself instead of from an
// outside host. Only relative links to files that exist are read.
func SiteMedia(root string, pages []SitePage) (map[string][]byte, error) {
	media := map[string][]byte{}
	for _, p := range pages {
		for _, m := range siteImage.FindAllStringSubmatch(p.Markdown, -1) {
			u, err := url.Parse(m[1])
			if err != nil || u.IsAbs() || u.Path == "" || strings.HasPrefix(u.Path, "/") {
				continue
			}
			rel := path.Join(path.Dir(p.Source), u.Path)
			if strings.HasPrefix(rel, "../") || media[rel] != nil {
				continue
			}
			b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
			if errors.Is(err, fs.ErrNotExist) {
				continue
			} else if err != nil {
				return nil, err
			}
			media[rel] = b
		}
	}
	return media, nil
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
		row("In plain words", c.Reader)
		for _, st := range verbs.Stages {
			if st.ID == c.Stage {
				row("Journey stage", st.Title)
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
<title>{{.Title}} · {{.Name}}</title>
<link rel="stylesheet" href="{{.Root}}style.css">
<link rel="alternate" type="text/markdown" href="{{.Twin}}">
<link rel="alternate" type="text/plain" title="Index of every page, for agents" href="{{.Root}}llms.txt">
</head>
<body>
<a class="skip" href="#main">Skip to content</a>
<header class="top">
<a class="brand" href="{{.Root}}index.html">{{.Name}}</a>
<span class="build">Built from <a href="{{.Commit}}">{{.Short}}</a>{{if .Date}} ({{.Date}}){{end}}</span>
</header>
<details class="menu"><summary>Menu</summary>
<nav aria-label="Site menu">{{template "links" .Groups}}</nav>
</details>
<div class="layout">
<nav class="side" aria-label="Site">{{template "links" .Groups}}</nav>
<main id="main">
{{.Body}}
<footer>Generated by <code>aayu tell docs</code> from <a href="{{.Origin}}">{{.OriginPath}}</a>. Links to work items name only Features whose shipped stories all have authoritative proof packs. Agents: read this page <a href="{{.Twin}}">as Markdown</a>, or start from <a href="{{.Root}}llms.txt">llms.txt</a>.</footer>
</main>
</div>
</body>
</html>
{{define "links"}}
{{range .}}{{if .Name}}<details{{if .Open}} open{{end}}><summary>{{.Name}}</summary>{{end}}<ul>
{{range .Links}}<li><a href="{{.Href}}"{{if .Current}} aria-current="page"{{end}}>{{.Label}}</a></li>
{{end}}</ul>{{if .Name}}</details>{{end}}
{{end}}{{end}}`))

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
// page, its Markdown twin (page.md beside page.html, for agents), llms.txt (an
// index of every page with one line each), style.css, and app-manifest.html
// (the start of init --identity from any device, #578). Any page that claims an unproven Feature or uses an
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
	name += " docs"
	if s.Brand.Name != "" {
		name = s.Brand.Name // design/brand.yaml names the site
	}
	short := s.Ref
	if len(short) > 7 {
		short = short[:7]
	}
	files := map[string][]byte{"style.css": siteCSS, "app-manifest.html": appManifestPage}
	var index []llmsEntry
	var about string
	for p, b := range s.Media {
		files[p] = b
	}
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
			case *extast.Table:
				// A wide table scrolls sideways; keyboard users reach it with Tab (WCAG 2.1.1).
				l.SetAttributeString("tabindex", []byte("0"))
			case *ast.Blockquote:
				if label := calloutLabel(l, src); label != "" {
					l.SetAttributeString("class", []byte("callout callout-"+label))
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
		twin := strings.TrimSuffix(p.Out, ".html") + ".md"
		files[twin] = []byte("> The Markdown version of [" + title + "](" + path.Base(p.Out) + ") on the " + name + " site. " +
			"Every page is listed in [llms.txt](" + root + "llms.txt).\n\n" + s.twinLinks(p.Out, base, blob, outOf, copies[i]))
		index = append(index, llmsEntry{group: p.Group, title: title, href: twin, desc: pageSummary(p, copies[i])})
		if i == 0 {
			about = firstParagraph(copies[i])
		}
		var groups []navGroup
		for _, q := range s.Pages {
			if len(groups) == 0 || groups[len(groups)-1].Name != q.Group {
				groups = append(groups, navGroup{Name: q.Group})
			}
			g := &groups[len(groups)-1]
			g.Links = append(g.Links, navLink{Href: root + q.Out, Label: q.Nav, Current: q.Out == p.Out})
			g.Open = g.Open || q.Out == p.Out || openGroups[q.Group]
		}
		origin, _ := Tag(blob+p.Origin, s.UTM)
		commit, _ := Tag("https://github.com/"+s.Repo+"/commit/"+s.Ref, s.UTM)
		var page bytes.Buffer
		err = pageTmpl.Execute(&page, map[string]any{
			"Title": title, "Name": name, "Root": root, "Twin": path.Base(twin), "Commit": commit, "Short": short, "Date": s.Date,
			"Groups": groups, "Body": template.HTML(body.String()), "Origin": origin, "OriginPath": p.Origin, //nolint:gosec // goldmark output with raw HTML omitted
		})
		if err != nil {
			return nil, ns, nil, err
		}
		files[p.Out] = page.Bytes()
	}
	files["llms.txt"] = llmsTxt(name, about, index)
	return files, ns, nil, nil
}

// llmsEntry is one page's line in llms.txt.
type llmsEntry struct{ group, title, href, desc string }

// llmsTxt is the site's index for agents (the llms.txt convention): the site
// name, what it is (the home page's first paragraph), and one line per page under its sidebar group, linking
// the page's Markdown twin. It is built from the same checked copy as the
// pages, so it cannot drift from them.
func llmsTxt(name, about string, index []llmsEntry) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", name)
	if about != "" {
		fmt.Fprintf(&b, "> %s\n\n", about)
	}
	b.WriteString("Every page below is Markdown; the same path ending in .html is the page for people. " +
		"Start with the first group, then read the How it works pages for the mechanics.\n")
	group := "\x00"
	for _, e := range index {
		if e.group != group {
			group = e.group
			g := group
			if g == "" {
				g = "Start here"
			}
			fmt.Fprintf(&b, "\n## %s\n\n", g)
		}
		fmt.Fprintf(&b, "- [%s](%s)", e.title, e.href)
		if e.desc != "" {
			fmt.Fprintf(&b, ": %s", e.desc)
		}
		b.WriteString("\n")
	}
	return []byte(b.String())
}

// siteLink matches an inline Markdown link or image: its text and destination.
var siteLink = regexp.MustCompile(`(!?\[[^\]\n]*\])\(<?([^)\s>]+)>?((?:\s+"[^"]*")?)\)`)

// twinLinks maps the links of a page's Markdown as the HTML page maps them,
// except that a link to another page goes to that page's Markdown twin.
func (s Site) twinLinks(out string, base *url.URL, blob string, outOf map[string]string, md string) string {
	return siteLink.ReplaceAllStringFunc(md, func(m string) string {
		g := siteLink.FindStringSubmatch(m)
		h := s.href(out, base, blob, outOf, g[2], strings.HasPrefix(g[1], "!"))
		if u, err := url.Parse(h); err == nil && !u.IsAbs() && strings.HasSuffix(u.Path, ".html") {
			u.Path = strings.TrimSuffix(u.Path, ".html") + ".md"
			h = u.String()
		}
		return g[1] + "(" + h + g[3] + ")"
	})
}

// pageSummary is a page's line in llms.txt: a skill page's reader summary
// from the registry, or the first sentence of the page's first paragraph.
func pageSummary(p SitePage, md string) string {
	if p.Summary != "" {
		return p.Summary
	}
	if p.Group == "Skills" {
		for _, c := range verbs.Verbs {
			if c.Name == p.Nav {
				return c.Reader
			}
		}
	}
	return firstSentence(firstParagraph(md))
}

// firstParagraph is the text of a page's first prose paragraph: headings,
// quotes, images, tables, lists, comments, and code are skipped, and links
// keep only their text.
func firstParagraph(md string) string {
	var para []string
	fence := false
	for _, l := range strings.Split(md, "\n") {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "```") {
			fence = !fence
			continue
		}
		if fence {
			continue
		}
		if t == "" {
			if len(para) > 0 {
				break
			}
			continue
		}
		if len(para) == 0 && strings.ContainsAny(t[:1], "#>!|<-*[") {
			continue
		}
		para = append(para, t)
	}
	return siteLink.ReplaceAllStringFunc(strings.Join(para, " "), func(m string) string {
		g := siteLink.FindStringSubmatch(m)
		return strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(g[1], "!"), "["), "]")
	})
}

// firstSentence cuts text after its first sentence, at most 240 characters.
func firstSentence(text string) string {
	if i := strings.Index(text, ". "); i > 0 {
		text = text[:i+1]
	}
	if r := []rune(text); len(r) > 240 {
		text = string(r[:239]) + "…"
	}
	return text
}

// href maps a link of the page at out: a link to another page's source
// becomes a relative link to that page, and an image in Media a relative link
// to the site's copy; any other relative link resolves as GitHub resolves it
// from the source file and gets the UTM tags (an image, the raw file).
// Absolute links were tagged by Check.
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
		if image && s.Media[rest] != nil {
			return strings.Repeat("../", strings.Count(out, "/")) + rest
		}
		if image {
			return "https://raw.githubusercontent.com/" + s.Repo + "/" + s.Ref + "/" + rest
		}
	}
	t, _ := Tag(r.String(), s.UTM)
	return t
}

// calloutLabel names a callout: a block quote that opens with a bold label
// ending in a colon ("> **Tip:** …", "> **You are here:** …") is styled by
// that label, here "tip" or "you-are-here". The label stays in the text, so
// the callout reads the same on GitHub and to a screen reader.
func calloutLabel(q *ast.Blockquote, src []byte) string {
	para, ok := q.FirstChild().(*ast.Paragraph)
	if !ok {
		return ""
	}
	em, ok := para.FirstChild().(*ast.Emphasis)
	if !ok || em.Level != 2 {
		return ""
	}
	var label strings.Builder
	for c := em.FirstChild(); c != nil; c = c.NextSibling() {
		t, ok := c.(*ast.Text)
		if !ok {
			return ""
		}
		label.Write(t.Segment.Value(src))
	}
	name, ok := strings.CutSuffix(strings.TrimSpace(label.String()), ":")
	if !ok || name == "" {
		return ""
	}
	return strings.ToLower(strings.Join(strings.FieldsFunc(name, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsNumber(r) }), "-"))
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
