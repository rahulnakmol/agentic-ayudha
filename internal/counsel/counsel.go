// Package counsel is the Founder twin's on-demand counsel (Story #478): Atri, the Founder's Sage, is consulted on any
// topic, issue, Decision, path, or pull request, the way an oracle is, and answers with the call framed and its door,
// the operations, technology, and product readings with the dominant one, one recommendation, what not to do, what
// would change it, the next aayu command, and the question the person must answer. Counsel is advice: it never
// approves, accepts, merges, or counts as a review. The checks here are deterministic (ground rule 7); the host's
// model writes the counsel.
package counsel

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/seats"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

// Practice is the Founder persona's practice whose stage artefact titles a counsel (seats/founder/persona.toml).
const Practice = "three-perspectives"

// MaxWords bounds a counsel: the person reads it in a few minutes, so detail is cited, not pasted. A startup gets
// the one-page bound of seats.CheckStage as well.
const MaxWords = 500

// rule is one thing a part must say, as a pattern and the reason it is asked for.
type rule struct {
	re  *regexp.Regexp
	why string
}

func must(pattern, why string) rule { return rule{regexp.MustCompile(`(?i)` + pattern), why} }

// Section is one required part of a counsel, written as a "## " heading that starts with Title.
type Section struct {
	Title string
	What  string
	must  []rule
}

// Sections are the parts of a counsel, in order; the last one ends it.
var Sections = []Section{
	{Title: "The call", What: "What this is really about, one level up, and its door: a one-way (irreversible) or a two-way (reversible) door, cited.",
		must: []rule{must(`\b(one|two)-way door\b`, "say whether the call is a one-way or a two-way door")}},
	{Title: "Readings", What: "The operations (cost, risk, people, process), technology (architecture, build or reuse, technical risk), and product (what, for whom, why) readings, each with its cited facts, then which one dominates and why.",
		must: []rule{must(`\boperations\b`, "give the operations reading"), must(`\btechnology\b`, "give the technology reading"),
			must(`\bproduct\b`, "give the product reading"), must(`\bdominat\w*`, "name the reading that dominates and why")}},
	{Title: "Recommendation", What: "One recommendation and why, in a short cited paragraph; a recommendation, never a ruling."},
	{Title: "What not to do", What: "What the counsel advises against, and why, cited."},
	{Title: "What would change it", What: "The facts or answers that would change the recommendation."},
	{Title: "Next step", What: "The concrete next aayu command, in backticks, and who runs it.",
		must: []rule{must("`aayu [a-z]", "give the next step as an `aayu` command in backticks")}},
	{Title: "Your question", What: "The one question the person must answer, in one line."},
}

var (
	h2RE     = regexp.MustCompile(`(?m)^##\s+(.+)$`)
	listItem = regexp.MustCompile(`(?m)^(?:[-*+]|\d+[.)])\s+`)
	// claimed is a first-person claim that a review, an approval, or a ruling happened, or an LGTM given as a
	// statement of its own: counsel never is one and never says it is (Story #478). Negations ("this is not a
	// review") and naming a request ("the body asks for an LGTM") do not match, because each pattern needs the
	// claim's own words next to each other.
	claimed = regexp.MustCompile(`(?i)(?:^|[.!?:;]\s+|[*_]+)LGTM\b|\b(?:I|we)\s+(?:have\s+)?(?:approve[sd]?|sign(?:ed)?\s+off|reviewed)\b|` +
		`\bapproved\s+(?:it|this|the)\b|\bmy\s+(?:approval|ruling)\b`)
	// asserted is the claim without "I": a sentence that opens with "Approved", "Reviewed", or "Signed off", or the
	// twin or this counsel named as approving or reviewing ("Atri approves this PR").
	asserted = regexp.MustCompile(`(?i)(?:^|[.!?:;]\s+|[*_]+)(?:approved|reviewed|signed[ -]off)\b|` +
		`\b(?:Atri|the sage|the founder's sage|the twin|this counsel|counsel)\s+(?:has\s+)?(?:approve[sd]?|review(?:s|ed)|signs?\s+off|signed\s+off)\b`)
	// codeSpan is inline code; command is one that reads as a shell command (a word, a space, more), not a path.
	codeSpan = regexp.MustCompile("`([^`]+)`")
	command  = regexp.MustCompile(`^[a-z][\w.-]*\s+\S`)
	// recommends counts recommendation statements: a counsel makes one.
	recommends = regexp.MustCompile(`(?i)\brecommend(?:s|ed|ing)?\b|\balternatively\b`)
	// passive is the same claim in the passive ("the PR has been reviewed"), cited or not: a citation shows a source
	// exists, not that it records a review, so counsel reports a review as what a record shows ("the record shows
	// @lead's review (#12)").
	passive = regexp.MustCompile(`(?i)\b(?:is|are|has been|have been)\s+(?:approved|reviewed|signed off)\b`)
)

// CheckStructure returns what keeps a counsel from being complete: a missing, empty, or out-of-order part, a part
// that leaves out what it must say, more than one recommendation, a question that is not the last part or not one
// line, a claim of a review, an approval, or a ruling, and more than MaxWords words.
func CheckStructure(md string) []string {
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
		for i, p := range parts {
			if matches(p.title, title) {
				return i, p.body
			}
		}
		return -1, ""
	}
	var out []string
	for _, p := range parts {
		if !slices.ContainsFunc(Sections, func(s Section) bool { return matches(p.title, s.Title) }) {
			out = append(out, fmt.Sprintf("part %q is not a part of a counsel; the parts are only the seven, so advice cannot hide in another", p.title))
		}
	}
	for _, s := range Sections {
		n := 0
		for _, p := range parts {
			if matches(p.title, s.Title) {
				n++
			}
		}
		if n > 1 {
			out = append(out, fmt.Sprintf("part %q appears %d times; each part appears once", s.Title, n))
		}
	}
	last := -1
	for _, s := range Sections {
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
	if _, r := find("Recommendation"); r != "" && (listItem.MatchString(r) || strings.Contains(r, "\n\n") || len(recommends.FindAllString(r, -1)) > 1) {
		out = append(out, "the recommendation is more than one paragraph or a list; make one recommendation")
	}
	if _, next := find("Next step"); next != "" {
		for _, m := range codeSpan.FindAllStringSubmatch(next, -1) {
			if command.MatchString(m[1]) && !strings.HasPrefix(m[1], "aayu ") {
				out = append(out, fmt.Sprintf("part %q: `%s` is not an aayu command; the next step goes through aayu", "Next step", m[1]))
			}
		}
	}
	if i, q := find("Your question"); i >= 0 {
		if i != len(parts)-1 {
			out = append(out, `the counsel must end with "## Your question"`)
		}
		if q != "" && (strings.Contains(q, "\n") || !strings.HasSuffix(q, "?") || strings.Count(q, "?") > 1) {
			out = append(out, "the question to answer is not one question on one line ending in a question mark")
		}
	}
	for _, line := range strings.Split(md, "\n") {
		ms := append(claimed.FindAllString(line, -1), asserted.FindAllString(line, -1)...)
		ms = append(ms, passive.FindAllString(line, -1)...)
		for _, m := range ms {
			out = append(out, fmt.Sprintf("claims a review, an approval, or a ruling (%q): counsel is advice and never one", m))
		}
	}
	if n := seats.Words(md); n > MaxWords {
		out = append(out, fmt.Sprintf("%d words: counsel is read in a few minutes, so at most %d; cite detail instead of pasting it", n, MaxWords))
	}
	return out
}

// matches reports whether a "## " heading names the part title: the title, then nothing or a separator.
func matches(heading, title string) bool {
	h := strings.ToLower(strings.NewReplacer("*", "", "`", "").Replace(heading))
	rest, ok := strings.CutPrefix(h, strings.ToLower(title))
	return ok && (rest == "" || strings.IndexAny(rest[:1], " :(-—,") == 0)
}

// Check is every deterministic check on a counsel: its parts (CheckStructure), the stage's artefact of the
// three-perspectives practice as its title with the one-page bound at a startup (seats.CheckStage), and the evidence
// rule with known resolving each citation (seats.CheckGroundingWith).
func Check(md, stage string, pr seats.Practice, known func(cite string) bool) []string {
	out := CheckStructure(md)
	out = append(out, seats.CheckStage(md, pr, stage)...)
	return append(out, seats.CheckGroundingWith(md, known)...)
}

// Template is the outline the twin fills: the artefact title, then one line per part.
func Template(artefact string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s: <the topic in a few words>\n\n", artefact)
	for _, s := range Sections {
		fmt.Fprintf(&b, "## %s\n\n%s\n\n", s.Title, s.What)
	}
	return b.String()
}

// Label says which model wrote the counsel and, on a pull request, how its family compares with the builder's
// (the review ladder's rule, Decision #469): different-family only when the builder's family is known and the
// writer's family is known and differs; otherwise same-family. Without a pull request it names the writer only.
func Label(model string, onPR bool, builder verbs.Family, candidates []verbs.Family) string {
	writer := "model not reported"
	fam := verbs.FamilyOf(model)
	if model != "" {
		writer = model
		if fam != "" {
			writer += " (" + string(fam) + ")"
		}
	}
	if !onPR {
		return "written by " + writer
	}
	b := "builder family unknown"
	if builder != "" {
		b = "builder " + string(builder)
	}
	if builder != "" && fam != "" && fam != builder && !slices.Contains(candidates, fam) {
		return fmt.Sprintf("different-family counsel: %s vs %s", writer, b)
	}
	return fmt.Sprintf("same-family counsel: %s, %s", writer, b)
}

// Marker opens the counsel comment on an issue or pull request, so a rerun replaces it in place.
const Marker = "<!-- aayudha:counsel -->"

// Comment is the comment that carries a checked counsel: headed as counsel from the twin, advice only and never a
// review, an approval, or a Decision, with the writer's label.
func Comment(heading, label, md string) string {
	return Marker + "\n**Counsel from " + heading + ".** Advice only: not a review, an approval, or a Decision; the person decides. _" +
		label + "._\n\n" + strings.TrimSpace(md) + "\n\n_Checked by `aayu counsel --draft`: every part is there and every claim is cited or marked unverified._\n"
}
