package prbody

import (
	_ "embed"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/brief"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// template is the pull request template `aayu init` installs and this
// repository keeps as .github/pull_request_template.md.
//
//go:embed pull_request_template.md
var template []byte

// Template is the pull request template: the sections Check reads,
// each with short guidance.
func Template() []byte { return slices.Clone(template) }

// Sections are the level-2 sections every pull request body carries, in
// order. Reviewers is the last; its technical detail is not plain prose.
var Sections = []string{"What this does", "Why", "What changed", "How we know it works", "Risk and rollback", Reviewers}

// Reviewers is the section for technical detail, inside <details>.
const Reviewers = "For reviewers"

// Deviation is the optional section AGENTS.md asks for when the work
// differs from the plan, with that exact wording.
const Deviation = "Deviation from the plan:"

// Placeholder marks what the author still has to write: in the template's
// At a glance table, and where `aayu build` cannot know the answer. A
// section that still holds it is unfilled.
const Placeholder = "_(to write)_"

var (
	prHeading    = regexp.MustCompile(`^##\s+(.+?)\s*#*\s*$`)
	htmlComment  = regexp.MustCompile(`(?s)<!--.*?-->`)
	htmlSummary  = regexp.MustCompile(`(?is)<summary>.*?</summary>`)
	htmlTag      = regexp.MustCompile(`</?[A-Za-z][^>\n]*>`)
	tableRule    = regexp.MustCompile(`(?m)^\s*\|?[\s:|-]*-[\s:|-]*\|?\s*$`)
	tableHead    = regexp.MustCompile(`(?m)^\s*\|.*\n\s*\|?[\s:|-]*-[\s:|-]*\|?\s*$`)
	fenceLine    = regexp.MustCompile("^(`{3,}|~{3,})")
	calloutMark  = regexp.MustCompile(`\[!(?:NOTE|TIP|IMPORTANT|WARNING|CAUTION)\]`)
	trailerLine  = regexp.MustCompile(`(?im)^\s*(?:(?:refs|fixes|closes)\s+#\d+|aayudha-[a-z]+:.*)\s*$`)
	sentenceSize = regexp.MustCompile(`^sentence \d+ has \d+ words`)
	// unapprovedWord reads brief.LintSTE's word message, with its advice.
	unapprovedWord = regexp.MustCompile(`^("[^"]*") is not an approved word \(sentence \d+\)(?:; write (.*))?$`)
	inlineCode     = regexp.MustCompile("`[^`\n]*`")
)

// prSection is one level-2 section of a body: its heading and raw text.
type prSection struct{ name, text string }

// prSplit splits body at its level-2 headings. A heading inside a code fence,
// an HTML comment, or a <details> block does not start a section: the For
// reviewers detail may have headings of its own.
func prSplit(body string) []prSection {
	var out []prSection
	body = uncomment(body)
	fence, details := "", 0
	for _, line := range strings.Split(body, "\n") {
		t := strings.TrimSpace(line)
		switch m := fenceLine.FindString(t); {
		case fence == "" && m != "":
			fence = m
		case fence != "" && strings.HasPrefix(t, fence) && strings.Trim(t, fence[:1]) == "":
			fence = ""
		}
		if fence == "" {
			tags := strings.ToLower(inlineCode.ReplaceAllString(line, "")) // a tag in a code span is text
			details += strings.Count(tags, "<details") - strings.Count(tags, "</details>")
			details = max(details, 0)
		}
		if m := prHeading.FindStringSubmatch(line); m != nil && fence == "" && details == 0 {
			out = append(out, prSection{name: m[1]})
			continue
		}
		if len(out) > 0 {
			out[len(out)-1].text += line + "\n"
		}
	}
	return out
}

// uncomment drops the text of body's HTML comments and keeps their lines. An
// unclosed comment hides the rest of the body, as it does on GitHub. An
// opener in a code span or a fence is text, not a comment.
func uncomment(body string) string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	masked := codeMask(body)
	var b strings.Builder
	last := 0
	for _, m := range htmlComment.FindAllStringIndex(masked, -1) {
		b.WriteString(body[last:m[0]])
		b.WriteString(strings.Repeat("\n", strings.Count(body[m[0]:m[1]], "\n")))
		last = m[1]
	}
	if i := strings.Index(masked[last:], "<!--"); i >= 0 {
		return b.String() + body[last:last+i]
	}
	return b.String() + body[last:]
}

// codeMask is body with its code spans and fenced lines blanked to spaces,
// at the same offsets.
func codeMask(body string) string {
	blank := func(s string) string { return strings.Repeat(" ", len(s)) }
	var b strings.Builder
	fence := ""
	for _, line := range strings.SplitAfter(body, "\n") {
		text, nl := strings.CutSuffix(line, "\n")
		t := strings.TrimSpace(text)
		m := fenceLine.FindString(t)
		switch {
		case fence == "" && m != "":
			fence, text = m, blank(text)
		case fence != "":
			if strings.HasPrefix(t, fence) && strings.Trim(t, fence[:1]) == "" {
				fence = ""
			}
			text = blank(text)
		default:
			text = inlineCode.ReplaceAllStringFunc(text, blank)
		}
		b.WriteString(text)
		if nl {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// Visible is body as a reader sees it outside code: no HTML comments and no
// fenced blocks, so an example line there declares nothing.
func Visible(body string) string {
	var b strings.Builder
	fence := ""
	for _, line := range strings.Split(uncomment(body), "\n") {
		t := strings.TrimSpace(line)
		switch m := fenceLine.FindString(t); {
		case fence == "" && m != "":
			fence = m
		case fence != "" && strings.HasPrefix(t, fence) && strings.Trim(t, fence[:1]) == "":
			fence = ""
		case fence == "":
			b.WriteString(line + "\n")
		}
	}
	return b.String()
}

// prName folds a heading for comparison: case and a trailing colon do not count.
func prName(s string) string { return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(s), ":")) }

// prFilled reports whether a section has content a reader sees: prose, a
// list, a table row, a callout, or code. HTML comments, tags, a <details>
// summary, table headings and rules, callout markers, and whitespace do not
// count. For reviewers counts only what is inside its <details>, else what is
// not the Refs line and declarations that follow it.
func prFilled(name, text string) bool {
	text = htmlComment.ReplaceAllString(text, "")
	if prName(name) == prName(Reviewers) {
		if i := strings.Index(strings.ToLower(text), "<details"); i >= 0 {
			text = text[i:]
			if j := strings.Index(strings.ToLower(text), "</details>"); j >= 0 {
				text = text[:j]
			}
		} else {
			text = trailerLine.ReplaceAllString(text, "")
		}
	}
	text = htmlSummary.ReplaceAllString(text, "")
	text = htmlTag.ReplaceAllString(text, "")
	text = tableHead.ReplaceAllString(text, "")
	text = tableRule.ReplaceAllString(text, "")
	text = calloutMark.ReplaceAllString(text, "")
	return strings.Trim(text, " \t\n>|") != ""
}

// Check checks a pull request body against the agreed structure
// (Sections, and Deviation when present) and its plain sections' prose
// against ASD-STE100: sentences of at most brief.MaxSTEWords words, and
// approved words when words is set. Tables, code, mermaid diagrams, HTML,
// headings, callout markers, and the For reviewers section are not prose.
//
// For an agent-authored PR (agent), a missing, empty, or unfilled section and
// a sentence that is too long block; unapproved words only warn. For any
// other PR everything warns. An agent can always fix its own prose, so the
// floor holds it to the structure; a person is never blocked on wording.
func Check(body string, words *brief.Words, agent bool) (block, warn []workitems.Problem) {
	hard := func(field, msg string) {
		p := workitems.Problem{Field: field, Message: msg}
		if agent {
			block = append(block, p)
		} else {
			warn = append(warn, p)
		}
	}
	plain := map[string]bool{prName(Deviation): true}
	for _, s := range Sections[:len(Sections)-1] {
		plain[prName(s)] = true
	}
	secs := prSplit(body)
	// Every copy of a section counts: a second "## What changed" (pasted from
	// a Story, say) never hides an unfilled one.
	for _, want := range Sections {
		found := false
		for _, sec := range secs {
			if prName(sec.name) != prName(want) {
				continue
			}
			found = true
			switch {
			case strings.Contains(sec.text, Placeholder):
				hard(want, "the section still holds a placeholder: write it")
			case !prFilled(want, sec.text):
				hard(want, "the section is empty: comments and table headings alone do not count")
			}
		}
		if !found {
			hard(want, "the section is missing: add a \"## "+want+"\" heading and write it")
		}
	}
	for _, s := range secs {
		switch {
		case prName(s.name) != prName(Deviation):
		case strings.Contains(s.text, Placeholder):
			hard(Deviation, "the section still holds a placeholder: write it")
		case !prFilled(s.name, s.text):
			hard(Deviation, "the section is empty: say what the plan assumed, what you found, and what you built instead")
		}
	}
	d, err := brief.Parse([]byte(body), "")
	if err != nil {
		return block, warn
	}
	units := map[string][]brief.Unit{}
	var order []string
	for _, u := range d.Units {
		if !plain[prName(u.Section)] {
			continue
		}
		u.Text = strings.TrimSpace(calloutMark.ReplaceAllString(strings.ReplaceAll(u.Text, Placeholder, ""), ""))
		if u.Text == "" {
			continue
		}
		if _, ok := units[u.Section]; !ok {
			order = append(order, u.Section)
		}
		units[u.Section] = append(units[u.Section], u)
	}
	for _, sec := range order {
		for _, s := range brief.Sentences(units[sec]) {
			if len(s.Words) > brief.MaxSTEWords {
				hard(sec, fmt.Sprintf("a sentence has %d words (at most %d); split it: %q", len(s.Words), brief.MaxSTEWords, clipText(s.Text, 120)))
			}
		}
		if words == nil {
			continue
		}
		// One warning per section names the words outside the list, so a
		// long body does not flood the inspect comment.
		var outside []string
		for _, m := range brief.LintSTE(units[sec], words, nil) {
			switch w := unapprovedWord.FindStringSubmatch(m); {
			case sentenceSize.MatchString(m):
			case w != nil && w[2] != "":
				outside = append(outside, w[1]+" (write "+w[2]+")")
			case w != nil:
				outside = append(outside, w[1])
			default:
				warn = append(warn, workitems.Problem{Field: sec, Message: m})
			}
		}
		if len(outside) > 0 {
			more := ""
			if len(outside) > maxListed {
				outside, more = outside[:maxListed], fmt.Sprintf(", and %d more", len(outside)-maxListed)
			}
			warn = append(warn, workitems.Problem{Field: sec, Message: "words not in the STE list: " + strings.Join(outside, ", ") + more})
		}
	}
	return block, warn
}

// maxListed is how many words outside the STE list one warning names.
const maxListed = 8

func clipText(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}
