package tell

import (
	"strings"
	"testing"
)

// The site links its own pages relatively (anchors as GitHub writes them),
// serves the images it carries itself, sends every other repository link to
// GitHub at the built commit with UTM tags, styles block quotes that open with
// a bold label as callouts, and refuses the whole site when one page claims an
// unproven Feature.
func TestSiteBuild(t *testing.T) {
	pages := []SitePage{
		{Out: "index.html", Nav: "Overview", Source: "README.md", Origin: "README.md", Markdown: "# Repo\n\n" +
			"Read [the promotion steps](CONTRIBUTING.md#promotion-agent-run-no-approval), [the plan](docs/BUILD-PLAN.md#a), and #7.\n\n![logo](docs/logo.png)\n\n" +
			"![The journey has nine steps](docs/guide/img/journey.svg)\n\n> **You are here:** step 1.\n\n> **Note** without a colon.\n\n> Plain.\n"},
		{Out: "contributing.html", Nav: "Contributing", Group: "Project", Source: "CONTRIBUTING.md", Origin: "CONTRIBUTING.md",
			Markdown: "# Contributing\n\n## Promotion (agent-run, no approval)\n\n## Promotion (agent-run, no approval)\n"},
		{Out: "skills/grill.html", Nav: "grill", Group: "Skills", Source: "skills/grill/SKILL.md", Origin: "skills/grill/SKILL.md",
			Markdown: "# grill\n\nSee [the format](../README.md) and [how to contribute](../../CONTRIBUTING.md).\n\n![Journey](../../docs/guide/img/journey.svg)\n"},
	}
	s := Site{Repo: "o/r", Ref: "abc1234def", Date: "2026-10-12", Brand: Default, UTM: UTM{Source: "r", Medium: "docs", Campaign: "c"},
		Proven: map[int]Feature{7: {Number: 7, Title: "Export", URL: "https://github.com/o/r/issues/7"}}, Pages: pages,
		Media: map[string][]byte{"docs/guide/img/journey.svg": []byte("<svg/>")}}
	files, cited, probs, err := s.Build()
	if err != nil || len(probs) != 0 || len(cited) != 1 || cited[0] != 7 {
		t.Fatalf("build: %v %+v %v", err, probs, cited)
	}
	tags := "?utm_campaign=c&amp;utm_medium=docs&amp;utm_source=r"
	for _, tt := range []struct{ file, want string }{
		{"index.html", `href="contributing.html#promotion-agent-run-no-approval"`},
		{"index.html", `href="https://github.com/o/r/blob/abc1234def/docs/BUILD-PLAN.md` + tags + `#a"`},
		{"index.html", `src="https://raw.githubusercontent.com/o/r/abc1234def/docs/logo.png"`},
		{"index.html", `<title>Repo · r docs</title>`},
		{"index.html", `<img src="docs/guide/img/journey.svg" alt="The journey has nine steps">`},
		{"index.html", `<blockquote class="callout callout-you-are-here">`},
		{"index.html", "<blockquote>\n<p><strong>Note</strong> without a colon.</p>\n</blockquote>\n<blockquote>\n<p>Plain.</p>"},
		{"contributing.html", `id="promotion-agent-run-no-approval"`},
		{"contributing.html", `id="promotion-agent-run-no-approval-1"`},
		{"skills/grill.html", `href="https://github.com/o/r/blob/abc1234def/skills/README.md` + tags + `"`},
		{"skills/grill.html", `href="../contributing.html"`},
		{"skills/grill.html", `href="../style.css"`},
		{"skills/grill.html", `src="../docs/guide/img/journey.svg"`},
		{"skills/grill.html", `<a href="../index.html">Overview</a>`},
	} {
		if !strings.Contains(string(files[tt.file]), tt.want) {
			t.Errorf("%s lacks %s:\n%s", tt.file, tt.want, files[tt.file])
		}
	}
	if len(files["style.css"]) == 0 || string(files["docs/guide/img/journey.svg"]) != "<svg/>" || len(files) != 5 {
		t.Fatalf("files: %v", SiteFiles(files))
	}

	// design/brand.yaml's name titles the site.
	named := s
	named.Brand.Name = "aayudha docs"
	files, _, _, _ = named.Build()
	if b := string(files["index.html"]); !strings.Contains(b, `<title>Repo · aayudha docs</title>`) || !strings.Contains(b, `class="brand" href="index.html">aayudha docs</a>`) {
		t.Errorf("brand name not used as the site name:\n%s", b)
	}

	s.Pages = append(s.Pages, SitePage{Out: "security.html", Nav: "Security", Source: "SECURITY.md", Origin: "SECURITY.md", Markdown: "# Security\n\nFixed in #8.\n"})
	files, _, probs, err = s.Build()
	if err != nil || files != nil || len(probs) != 1 || probs[0].String() != "SECURITY.md line 3: #8 is not a Feature with authoritative packs; copy links only proven Features" {
		t.Fatalf("an unproven claim on one page: %v %v %+v", err, files != nil, probs)
	}
}
