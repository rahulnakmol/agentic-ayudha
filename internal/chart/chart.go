// Package chart is the Data & AI Architect's verb (Decision #283; docs/BUILD-PLAN.md WP5.1g): a chart note is the
// data or AI design of an issue, written by the twin after it has interviewed the human and before the issue is
// planned. A data note goes from the decision to its metrics, the design with the trigger that would outgrow it,
// privacy and retention, and the checks that prove it right; an AI note goes from the question to the simpler options,
// the evidence and its permissions, abstention, and an evaluation set. Both end with the assumptions and the open
// questions. The checks here are deterministic (ground rule 7); the host's model writes the note.
package chart

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/seats"
)

// Kinds of chart note.
const (
	KindData = "data"
	KindAI   = "ai"
)

// Kinds lists the chart kinds.
var Kinds = []string{KindData, KindAI}

// rule is one thing a part must say, as a pattern and the reason it is asked for.
type rule struct {
	re  *regexp.Regexp
	why string
}

// Section is one required part of a chart note, written as a "## " heading that starts with Title.
type Section struct {
	Title string
	What  string
	must  []rule
}

func must(pattern, why string) rule { return rule{regexp.MustCompile(`(?i)` + pattern), why} }

// sections are the parts of each kind, in order; the last one ends the note.
var sections = map[string][]Section{
	KindData: {
		{Title: "Decision", What: "The decision the data serves, who makes it, and how often; one level up from the dashboard or table asked for."},
		{Title: "Metrics", What: "Each metric with its definition, grain, window, population, source, and owner (a table reads best).",
			must: []rule{must(`\bgrain\b`, "name each metric's grain"), must(`\bwindows?\b`, "name each metric's window")}},
		{Title: "Design", What: "The smallest model, store, and pipeline that serve today's workload, the option it beat, and the measured trigger that would outgrow it.",
			must: []rule{must(`\btriggers?\b`, "name the measured trigger that would move the design to its next stage")}},
		{Title: "Privacy and retention", What: "Who may read what, any personal data, how long each copy is kept, and how a deletion reaches every copy.",
			must: []rule{must(`\b(retention|retain\w*|kept|keeps?)\b`, "say how long the data is kept"), must(`\bdelet\w*`, "say how a deletion reaches every copy")}},
		{Title: "Checks", What: "How we will know the numbers are right: reconciliations, quality checks with thresholds, and the criteria `plan` turns into checkpoints."},
		{Title: "Open questions", What: "Assumptions you made (each labelled `Assumption:`), and the questions only the human can answer, each with why it matters and a default."},
	},
	KindAI: {
		{Title: "Question", What: "What the feature must answer or do, for whom, and the decision or task it serves."},
		{Title: "Simpler options", What: "The simplest approach that could work (a SQL lookup, a keyword search, a rule, no model) and the evidence that it does or does not suffice."},
		{Title: "Evidence and permissions", What: "The sources, the retrieval approach and why, provenance (version and time), the citation contract, permission-aware retrieval, and prompt-injection and leakage risks.",
			must: []rule{must(`\bpermission`, "say how retrieval stays within what the reader may see"), must(`\binjection\b`, "say how untrusted sources and prompt injection are handled")}},
		{Title: "Abstention", What: "When the system says it does not know or refers the person on, and what it does when sources contradict each other."},
		{Title: "Evaluation", What: "The evaluation set (representative and adversarial cases), its metrics with thresholds, and the latency and cost budgets.",
			must: []rule{must(`\badversarial\b`, "include adversarial cases in the evaluation set"), must(`\d`, "give each evaluation metric a numeric threshold")}},
		{Title: "Open questions", What: "Assumptions you made (each labelled `Assumption:`), and the questions only the human can answer, each with why it matters and a default."},
	},
}

// Sections returns the parts of a kind of note, in order.
func Sections(kind string) []Section { return sections[kind] }

// Practice is the persona practice whose stage artefact titles a note of the kind (seats/data-ai-architect/persona.toml).
func Practice(kind string) string {
	if kind == KindAI {
		return "grounded-ai"
	}
	return "data-design"
}

var h2RE = regexp.MustCompile(`(?m)^##\s+(.+)$`)

// CheckStructure returns what keeps a note from being a complete chart note of its kind: a missing, empty, or
// out-of-order part, a part that leaves out what it must say, or open questions that are not the last part.
func CheckStructure(md, kind string) []string {
	want, ok := sections[kind]
	if !ok {
		return []string{fmt.Sprintf("kind %q: want data or ai", kind)}
	}
	type part struct{ title, body string }
	var parts []part
	locs := h2RE.FindAllStringSubmatchIndex(md, -1)
	for i, l := range locs {
		end := len(md)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		parts = append(parts, part{strings.TrimSpace(md[l[2]:l[3]]), strings.TrimSpace(md[l[1]:end])})
	}
	find := func(title string) (int, string) {
		t := strings.ToLower(title)
		for i, p := range parts {
			h := strings.ToLower(strings.NewReplacer("*", "", "`", "").Replace(p.title))
			if rest, ok := strings.CutPrefix(h, t); ok && (rest == "" || strings.IndexAny(rest[:1], " :(-—,") == 0) {
				return i, p.body
			}
		}
		return -1, ""
	}
	var out []string
	last := -1
	for _, s := range want {
		i, body := find(s.Title)
		switch {
		case i < 0:
			out = append(out, fmt.Sprintf("missing part %q: %s", "## "+s.Title, s.What))
			continue
		case i < last:
			out = append(out, fmt.Sprintf("part %q is out of order", s.Title))
		case body == "":
			out = append(out, fmt.Sprintf("part %q is empty", s.Title))
			continue
		}
		last = i
		for _, r := range s.must {
			if !r.re.MatchString(body) {
				out = append(out, fmt.Sprintf("part %q: %s", s.Title, r.why))
			}
		}
	}
	if i, _ := find("Open questions"); i >= 0 && i != len(parts)-1 {
		out = append(out, `the note must end with "## Open questions"`)
	}
	return out
}

// Check is every deterministic check on a note: its parts (CheckStructure), the stage's artefact of the kind's
// practice as its title with the one-page bound at a startup (seats.CheckStage), and the evidence rule with known
// resolving each citation (seats.CheckGroundingWith).
func Check(md, kind, stage string, pr seats.Practice, known func(cite string) bool) []string {
	out := CheckStructure(md, kind)
	out = append(out, seats.CheckStage(md, pr, stage)...)
	return append(out, seats.CheckGroundingWith(md, known)...)
}

// Template is the outline the twin fills: the artefact title, then one line per part.
func Template(kind, artefact string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s: <the issue in a few words> (#<issue>)\n\n", artefact)
	for _, s := range sections[kind] {
		fmt.Fprintf(&b, "## %s\n\n%s\n\n", s.Title, s.What)
	}
	return b.String()
}

// Marker opens the note's comment on the issue, one per kind, so a rerun replaces it in place.
func Marker(kind string) string { return fmt.Sprintf("<!-- aayudha:chart kind=%s -->", kind) }

// Comment is the issue comment that carries a checked note.
func Comment(kind, md string) string {
	return Marker(kind) + "\n" + strings.TrimSpace(md) + "\n\n_Chart note (" + kind + "), checked by `aayu chart --draft`; planning this issue starts from it._\n"
}
