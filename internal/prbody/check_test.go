package prbody

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/brief"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

const goodPRBody = `## What this does

The review now stops a pull request with an empty section.

| At a glance | |
|---|---|
| Story or Bug | #12 |
| Type | feature |
| Who it helps | founders who read pull requests |
| Risk | low |
| Breaking | no |

## Why

People could not read the change without the files. Story #12 asks for a clear text.

## What changed

| Before | After |
|---|---|
| Any text passes | An empty section stops an agent pull request |

` + "```mermaid\nflowchart LR\n  A[body] --> B{check}\n```" + `

## How we know it works

| Check or test | What it proves | Result |
|---|---|---|
| TestCheckPRBody | a good text passes | pass |

## Risk and rollback

The risk is low. Remove the check to go back.

> [!WARNING]
> Agents must write every section before the merge.

## For reviewers

<details><summary>Technical detail</summary>

The check reads the level 2 headings.

</details>

Refs #12

Aayudha-Host: amp
`

// thirty is one sentence of 30 words.
const thirty = "This one sentence has many words in it so that it goes on and on past the limit of twenty words that the check sets for plain prose here today."

func steWords(t *testing.T) *brief.Words {
	t.Helper()
	b, err := os.ReadFile("../../skills/brief/references/ste-words.md")
	if err != nil {
		t.Fatal(err)
	}
	w, err := brief.ParseWords(b)
	if err != nil {
		t.Fatal(err)
	}
	return w
}

func fields(ps []workitems.Problem) []string {
	var out []string
	for _, p := range ps {
		out = append(out, p.Field)
	}
	return out
}

func TestCheckPRBodyPassesAGoodBody(t *testing.T) {
	if n := len(brief.Sentences([]brief.Unit{{Text: thirty}})[0].Words); n != 30 {
		t.Fatalf("fixture sentence has %d words, want 30", n)
	}
	block, _ := Check(goodPRBody, steWords(t), true)
	if len(block) > 0 {
		t.Fatalf("a good body blocks: %v", block)
	}
}

func TestCheckPRBodyBlocksAnAgentPR(t *testing.T) {
	for name, tc := range map[string]struct {
		body    string
		section string
	}{
		"missing section": {strings.Replace(goodPRBody, "## Why\n", "", 1), "Why"},
		"empty section": {strings.Replace(goodPRBody, "The risk is low. Remove the check to go back.\n\n> [!WARNING]\n> Agents must write every section before the merge.\n",
			"<!-- What can go wrong. -->\n\n> [!NOTE]\n", 1), "Risk and rollback"},
		"empty details": {strings.Replace(goodPRBody, "The check reads the level 2 headings.\n", "<!-- detail -->\n", 1), "For reviewers"},
		"placeholder":   {strings.Replace(goodPRBody, "| Risk | low |", "| Risk | "+Placeholder+" |", 1), "What this does"},
		"long sentence": {strings.Replace(goodPRBody, "The risk is low.", thirty, 1), "Risk and rollback"},
		"long callout":  {strings.Replace(goodPRBody, "> Agents must write every section before the merge.", "> "+thirty, 1), "Risk and rollback"},
		// A second copy of a section never hides an unfilled one.
		"second copy": {strings.Replace(goodPRBody, "## For reviewers", "## Risk and rollback\n\n"+Placeholder+"\n\n## For reviewers", 1), "Risk and rollback"},
		// An unclosed comment hides the rest of the body on GitHub.
		"unclosed comment":     {strings.Replace(goodPRBody, "## For reviewers", "<!-- note\n\n## For reviewers", 1), "For reviewers"},
		"heading in a comment": {strings.Replace(goodPRBody, "## Why\n", "<!--\n## Why\n-->\n", 1), "Why"},
		"empty deviation": {strings.Replace(goodPRBody, "## For reviewers", "## Deviation from the plan:\n\n<!-- none -->\n\n## For reviewers", 1),
			Deviation},
		"deviation placeholder": {strings.Replace(goodPRBody, "## For reviewers", "## Deviation from the plan:\n\n"+Placeholder+"\n\n## For reviewers", 1),
			Deviation},
	} {
		t.Run(name, func(t *testing.T) {
			block, _ := Check(tc.body, nil, true)
			if !slices.Equal(fields(block), []string{tc.section}) {
				t.Fatalf("blocks %v, want only %q", block, tc.section)
			}
		})
	}
}

func TestCheckPRBodyIgnoresLongSentencesOutsideProse(t *testing.T) {
	for name, body := range map[string]string{
		"table cell":   strings.Replace(goodPRBody, "| a good text passes |", "| "+thirty+" |", 1),
		"code block":   strings.Replace(goodPRBody, "A[body] --> B{check}", thirty, 1),
		"reviewers":    strings.Replace(goodPRBody, "The check reads the level 2 headings.", thirty, 1),
		"html":         strings.Replace(goodPRBody, "## Why\n", "## Why\n\n<!-- "+thirty+" -->\n", 1),
		"deviation ok": strings.Replace(goodPRBody, "## For reviewers", "## Deviation from the plan:\n\nThe plan assumed one check. We built two.\n\n## For reviewers", 1),
		// A comment opener in code is text: it hides nothing after it.
		"opener in a code span": strings.Replace(goodPRBody, "## Why\n", "Write `<!--` to start a note.\n\n## Why\n", 1),
		"opener in a fence":     strings.Replace(goodPRBody, "## Why\n", "```html\n<!--\n```\n\n## Why\n", 1),
		// The technical detail may have its own headings and fences.
		"details tag in code": strings.Replace(goodPRBody, "| a good text passes |", "| a `<details>` block passes |", 1),
		"heading in details":  strings.Replace(goodPRBody, "The check reads the level 2 headings.", "## Debug\n\n"+thirty+"\n\n~~~\n```\n## Not a heading\n~~~", 1),
	} {
		t.Run(name, func(t *testing.T) {
			if block, _ := Check(body, nil, true); len(block) > 0 {
				t.Fatalf("blocks: %v", block)
			}
		})
	}
}

func TestCheckPRBodyOnlyWarnsAPerson(t *testing.T) {
	body := strings.Replace(strings.Replace(goodPRBody, "## Why\n", "", 1), "The risk is low.", thirty, 1)
	block, warn := Check(body, steWords(t), false)
	if len(block) > 0 {
		t.Fatalf("a person's PR blocks: %v", block)
	}
	if !slices.Contains(fields(warn), "Why") || !slices.Contains(fields(warn), "Risk and rollback") {
		t.Fatalf("warnings %v miss the missing section or the long sentence", warn)
	}
	// Unapproved words warn an agent too, and never block.
	block, warn = Check(strings.Replace(goodPRBody, "The risk is low.", "We utilize it.", 1), steWords(t), true)
	if len(block) > 0 || !strings.Contains(messages(warn), "utilize") {
		t.Fatalf("block %v, warn %v: want a warning for utilize only", block, warn)
	}
	// Words outside the list are one warning per section, not one per word.
	_, warn = Check(strings.Replace(goodPRBody, "The risk is low.", "We utilize frobnicate quuxes.", 1), steWords(t), true)
	if n := strings.Count(messages(warn), "Risk and rollback"); n != 1 {
		t.Fatalf("%d warnings for one section, want 1: %v", n, warn)
	}
}

// TestPRTemplate: the template shows exactly the sections the check requires,
// and this repository's own copy is the one aayu init installs.
func TestPRTemplate(t *testing.T) {
	var got []string
	for _, s := range prSplit(string(Template())) {
		got = append(got, s.name)
	}
	if !slices.Equal(got, Sections) {
		t.Fatalf("template sections %q, want %q", got, Sections)
	}
	if !strings.Contains(string(Template()), Deviation) {
		t.Errorf("the template does not name the %q section", Deviation)
	}
	repo, err := os.ReadFile("../../.github/pull_request_template.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(repo) != string(Template()) {
		t.Error(".github/pull_request_template.md differs from internal/workitems/pull_request_template.md; copy it")
	}
	// Unfilled, the template is empty where the author writes, and its At a
	// glance table, whose rows look filled, holds placeholders.
	block, _ := Check(string(Template()), nil, true)
	if want := []string{"What this does", "Why", "What changed", "How we know it works", "Risk and rollback", "For reviewers"}; !slices.Equal(fields(block), want) {
		t.Errorf("the empty template blocks %v, want %q", fields(block), want)
	}
}

func messages(ps []workitems.Problem) string {
	var s []string
	for _, p := range ps {
		s = append(s, p.String())
	}
	return strings.Join(s, "\n")
}
