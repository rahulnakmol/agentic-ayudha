package tell

import (
	"strings"
	"testing"
)

// The site links its own pages relatively (anchors as GitHub writes them),
// sends every other repository link to GitHub at the built commit with UTM
// tags, and refuses the whole site when one page claims an unproven Feature.
func TestSiteBuild(t *testing.T) {
	pages := []SitePage{
		{Out: "index.html", Nav: "Overview", Source: "README.md", Origin: "README.md", Markdown: "# Repo\n\n" +
			"Read [the promotion steps](CONTRIBUTING.md#promotion-agent-run-no-approval), [the plan](docs/BUILD-PLAN.md#a), and #7.\n\n![logo](docs/logo.png)\n"},
		{Out: "contributing.html", Nav: "Contributing", Group: "Project", Source: "CONTRIBUTING.md", Origin: "CONTRIBUTING.md",
			Markdown: "# Contributing\n\n## Promotion (agent-run, no approval)\n\n## Promotion (agent-run, no approval)\n"},
		{Out: "skills/grill.html", Nav: "grill", Group: "Skills", Source: "skills/grill/SKILL.md", Origin: "skills/grill/SKILL.md",
			Markdown: "# grill\n\nSee [the format](../README.md) and [how to contribute](../../CONTRIBUTING.md).\n"},
	}
	s := Site{Repo: "o/r", Ref: "abc1234def", Date: "2026-10-12", Brand: Default, UTM: UTM{Source: "r", Medium: "docs", Campaign: "c"},
		Proven: map[int]Feature{7: {Number: 7, Title: "Export", URL: "https://github.com/o/r/issues/7"}}, Pages: pages}
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
		{"contributing.html", `id="promotion-agent-run-no-approval"`},
		{"contributing.html", `id="promotion-agent-run-no-approval-1"`},
		{"skills/grill.html", `href="https://github.com/o/r/blob/abc1234def/skills/README.md` + tags + `"`},
		{"skills/grill.html", `href="../contributing.html"`},
		{"skills/grill.html", `href="../style.css"`},
		{"skills/grill.html", `<a href="../index.html">Overview</a>`},
	} {
		if !strings.Contains(string(files[tt.file]), tt.want) {
			t.Errorf("%s lacks %s:\n%s", tt.file, tt.want, files[tt.file])
		}
	}
	if len(files["style.css"]) == 0 || len(files) != 4 {
		t.Fatalf("files: %v", SiteFiles(files))
	}

	s.Pages = append(s.Pages, SitePage{Out: "security.html", Nav: "Security", Source: "SECURITY.md", Origin: "SECURITY.md", Markdown: "# Security\n\nFixed in #8.\n"})
	files, _, probs, err = s.Build()
	if err != nil || files != nil || len(probs) != 1 || probs[0].String() != "SECURITY.md line 3: #8 is not a Feature with authoritative packs; copy links only proven Features" {
		t.Fatalf("an unproven claim on one page: %v %v %+v", err, files != nil, probs)
	}
}
