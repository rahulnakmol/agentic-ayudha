package cli

import (
	"cmp"
	"fmt"
	"regexp"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/brief"
	"github.com/rahulnakmol/agentic-ayudha/internal/prbody"
	"github.com/rahulnakmol/agentic-ayudha/internal/ready"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// prMark, on a line of its own, opens a section of a pull request body that
// aayu writes; the same name after a slash closes it. refreshSections
// rewrites only what is between a pair, so a marker quoted inside a line is
// not one.
var prMark = regexp.MustCompile(`(?m)^<!-- aayudha:pr:([a-z0-9-]+) -->$`)

func prClose(name string) string { return "\n<!-- /aayudha:pr:" + name + " -->" }

// prMarked wraps content in the markers of section name.
func prMarked(name, content string) string {
	return "<!-- aayudha:pr:" + name + " -->\n" + strings.TrimSuffix(content, "\n") + prClose(name) + "\n"
}

// storyBody writes the pull request body of a Story or Bug that aayu build
// opens, in the structure prbody.Check checks. It starts the plain
// sections from the issue's own words (chk, and the Summary of issueBody),
// written once: the agent owns them after that, so a long sentence in the
// Story never blocks a PR the agent cannot edit. What only the agent can say
// (what changed, the risk, who it helps) is prbody.Placeholder. Only the
// facts aayu keeps current (the checks table and the technical detail) sit
// between markers, which a refresh rewrites. risk is the
// issue's risk label value ("" leaves the placeholder); deps are its
// depends-on links, written once as the Dependency rows of a RAID table that
// the agent then owns. builder is the name on the "Built by:" line.
func storyBody(chk ready.Result, issueBody, risk string, deps []prDependency, cps []CheckpointRecord, builder string) string {
	kind, typ := "Story", "feature"
	if chk.Type == "bug" {
		kind, typ = "Bug", "fix"
	}
	_, fields := workitems.Sections(issueBody)
	sentences := storySentences(fields["Summary"])
	summary := strings.Join(sentences, " ")
	title := strings.TrimSuffix(workitems.BareTitle(chk.Title), ".")
	first := ""
	if len(sentences) > 0 {
		first = " " + sentences[0]
	}
	ph := prbody.Placeholder
	var b strings.Builder
	b.WriteString("## What this does\n\n")
	fmt.Fprintf(&b, "**%s.**%s\n", title, first)
	fmt.Fprintf(&b, "\n| At a glance | |\n|---|---|\n| %s | #%d |\n| Type | %s |\n| Who it helps | %s |\n| Risk | %s |\n| Breaking | %s |\n",
		kind, chk.Issue, typ, ph, prCell(cmp.Or(risk, ph)), ph)
	b.WriteString("\n## Why\n\n")
	// Never a line that starts "Story #n": StoryOf reads it as the PR's story.
	why := fmt.Sprintf("This change is for %s #%d.\n", kind, chk.Issue)
	if summary != "" {
		why = summary + "\n\n" + why
	}
	b.WriteString(why)
	b.WriteString("\n## What changed\n\n" + ph + "\n")
	b.WriteString("\n## How we know it works\n\n")
	var rows strings.Builder
	rows.WriteString("| Check or test | What it proves | Result |\n|---|---|---|\n")
	for _, c := range chk.Criteria {
		fmt.Fprintf(&rows, "| Criterion %d (%s) | %s | pending: the merge gate's pack decides |\n", c.N, prCell(c.Evidence), prCell(c.State+"; expected: "+c.Expected))
	}
	for _, cp := range cps {
		fmt.Fprintf(&rows, "| Checkpoint %d/%d verify (%s) | the floors hold after this checkpoint | %s |\n", cp.K, cp.N, prCell(listOrNone(cp.Layers)), prCell(cp.Verify))
	}
	b.WriteString(prMarked("checks", rows.String()))
	b.WriteString("\n## Risk and rollback\n\n" + ph + "\n")
	if len(deps) > 0 {
		b.WriteString("\n| Type | Item | Owner | Status or mitigation |\n|---|---|---|---|\n")
		for _, d := range deps {
			fmt.Fprintf(&b, "| Dependency | #%d must merge first | %s | %s |\n", d.N, prCell(cmp.Or(d.Owner, "unassigned")), prCell(cmp.Or(d.Status, "unknown")))
		}
	}
	b.WriteString("\n## For reviewers\n\n<details><summary>Technical detail</summary>\n\n")
	var detail strings.Builder
	for _, cp := range cps {
		fmt.Fprintf(&detail, "- Checkpoint %d/%d: layers %s; verify %s; commit `%s`", cp.K, cp.N, listOrNone(cp.Layers), cp.Verify, short(cp.Commit))
		if cp.Detail != "" {
			detail.WriteString(". " + oneLine(cp.Detail))
		}
		detail.WriteString("\n")
	}
	if detail.Len() > 0 {
		detail.WriteString("\n")
	}
	fmt.Fprintf(&detail, "Built by: %s\n\nBuilt with `aayu build` from the Ready snapshot of #%d (body hash `%s`).\n", builder, chk.Issue, short(chk.BodyHash))
	b.WriteString(prMarked("detail", detail.String()))
	fmt.Fprintf(&b, "\n</details>\n\nRefs #%d\n", chk.Issue)
	return b.String()
}

// storySentences are the sentences of a Story's Summary that the PR body may
// quote: on one line, so no line of it starts a section of the body, and
// each within brief.MaxSTEWords, so the Story's words never make the body
// fail its own check.
func storySentences(summary string) []string {
	if workitems.Empty(summary) {
		return nil
	}
	var out []string
	for _, s := range brief.Sentences([]brief.Unit{{Text: oneLine(summary)}}) {
		if len(s.Words) <= brief.MaxSTEWords {
			out = append(out, s.Text)
		}
	}
	return out
}

// prDependency is a depends-on link of the Story: the issue it waits for,
// that issue's owner (an @login, or "" when it has none), and its state.
type prDependency struct {
	N             int
	Owner, Status string
}

// prCell is s on one line, safe in a table cell.
func prCell(s string) string { return strings.ReplaceAll(oneLine(s), "|", `\|`) }

// refreshSections returns old with the content of each marked section
// replaced by the same section's content in fresh. Everything else in old is
// kept byte for byte: what a person or the agent wrote, unmarked sections,
// blocks other verbs append (the session review, the declarations), and a
// marker pair someone deleted, which stays deleted.
func refreshSections(old, fresh string) string {
	want := map[string]string{}
	for _, m := range prMark.FindAllStringSubmatchIndex(fresh, -1) {
		name := fresh[m[2]:m[3]]
		if j := strings.Index(fresh[m[1]:], prClose(name)); j >= 0 {
			want[name] = fresh[m[1] : m[1]+j]
		}
	}
	var b strings.Builder
	rest := old
	for {
		m := prMark.FindStringSubmatchIndex(rest)
		if m == nil {
			break
		}
		name, end := rest[m[2]:m[3]], prClose(rest[m[2]:m[3]])
		b.WriteString(rest[:m[1]])
		rest = rest[m[1]:]
		j := strings.Index(rest, end)
		if j < 0 {
			continue // an open marker without its close: not a section
		}
		if c, ok := want[name]; ok {
			b.WriteString(c)
		} else {
			b.WriteString(rest[:j])
		}
		b.WriteString(end)
		rest = rest[j+len(end):]
	}
	b.WriteString(rest)
	return b.String()
}
