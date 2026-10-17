package decide

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/seats"
	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

// A decision pack (WP5.1b, D-5.6b) is what `aayu decide <issue|"question">
// --pack` helps a stuck human produce on demand: the owning seat's twin, in
// its persona, restates the question, grounds the facts, offers options with
// a recommendation and its door, has the Mentalist pre-mortem it, asks what
// would change the answer, and ends with the one thing to decide. Nothing is
// applied; the human records the outcome with `decide --accept` as before.

// PackSection is one required part of a decision pack, written as a "## "
// heading that starts with Title.
type PackSection struct {
	Title string
	What  string
}

// PackSections are the parts of a decision pack, in order; the last one ends
// the pack.
var PackSections = []PackSection{
	{"Question", "The question in one sentence, and whether it is the right question."},
	{"Facts", "The grounded facts first: code, LAYERS.md, packs, metrics, open issues, each cited."},
	{"Options", "Two or three numbered top-level options, each `N. **Name.** what it does` with its trade-offs: a Decision offers at least two, and the person picks one by its number."},
	{"Recommendation", "The recommended option, why, and its door: one-way (irreversible) or two-way (reversible)."},
	{"Cost of waiting", "What waiting a day or a week costs, cited where it can be."},
	{"Pre-mortem", "The Mentalist twin's pressure test: assume the recommendation failed; the likeliest reasons and the check that warns early."},
	{"Questions that would change the answer", "The few questions whose answers would flip the recommendation."},
	{"Your decision", "The one thing the human decides, answerable in a word or a line."},
}

// route maps topic words to the seat that owns the topic. The first rule with
// a match wins, so the narrower topics come first; delivery and alignment
// (the Engineering Manager) is the default.
var route = []struct {
	seat verbs.SeatID
	re   *regexp.Regexp
}{
	{verbs.Auditor, regexp.MustCompile(`(?i)\b(audit|control|compliance|bypass|evidence trail|unlogged)\b`)},
	// Data and AI design (Decision #283): before the Revenue Architect's metrics and the DevOps "pipeline", so "data
	// pipeline" and "analytics dashboard" reach the data seat while "conversion metric" and "CI pipeline" do not.
	{verbs.DataAIArchitect, regexp.MustCompile(`(?i)\b(data (pipelines?|warehouse|lake|lakehouse|platform|quality|contracts?|products?|lineage|retention)|warehouse|lakehouse|analytics|dashboards?|semantic layer|lineage|etl|elt|cdc|rag|retrieval|embeddings?|vector (index|store|database|db)|knowledge graphs?|graphrag|machine learning|ml|llm|eval sets?|hallucinat\w*|grounded answers?)\b`)},
	{verbs.RevenueArchitect, regexp.MustCompile(`(?i)\b(metrics?|conversion|revenue|pricing|price|experiments?|growth|retention|activation|north star|churn|readout)\b`)},
	{verbs.QualityArchitect, regexp.MustCompile(`(?i)\b(tests?|testing|flak\w*|coverage|floors?|quality|defects?|regression|waive\w*|proof packs?)\b`)},
	// Running production (Decision #324): incidents, SLOs, runbooks, and rollback belong to the Production Engineer.
	{verbs.ProductionEngineer, regexp.MustCompile(`(?i)\b(incidents?|outages?|slos?|error budgets?|latency|on-?call|rollbacks?|production|postmortems?|runbooks?|telemetry|alerts?)\b`)},
	{verbs.DevOps, regexp.MustCompile(`(?i)\b(ci|pipeline|release|install|deploy\w*|workflow|runner|promotion|devex)\b`)},
	{verbs.UXResearcher, regexp.MustCompile(`(?i)\b(ux|ui|screen|wireframe|mockup|prototype|accessibility|a11y|design system|usability)\b`)},
	{verbs.CodeArchitect, regexp.MustCompile(`(?i)\b(design|architecture|contracts?|api|schema|interface|boundar(y|ies)|data model|struct|field|index|migration|layers?|refactor)\b`)},
	{verbs.Marketer, regexp.MustCompile(`(?i)\b(docs?|documentation|launch|positioning|copy|announce\w*|changelog)\b`)},
	{verbs.Mentalist, regexp.MustCompile(`(?i)\b(explain|brief|stakeholders?|narrative)\b`)},
	{verbs.ProductStrategist, regexp.MustCompile(`(?i)\b(scope|customers?|users? need|problem|backlog|priorit\w*|feature request|roadmap)\b`)},
	{verbs.Founder, regexp.MustCompile(`(?i)\b(direction|strategy|vision|hire|hiring|bet|goals?|focus|pivot|budget)\b`)},
	{verbs.EngineeringManager, regexp.MustCompile(`(?i)\b(deliver\w*|estimate|deadline|owner|blocked|alignment|split|merge|review|sprint|plan)\b`)},
}

// typeSeat is the owning seat for an issue type when no topic word matches.
var typeSeat = map[string]verbs.SeatID{
	"goal": verbs.Founder, "initiative": verbs.Founder, "experiment": verbs.RevenueArchitect,
	"incident": verbs.ProductionEngineer, "bug": verbs.AINativeEngineer, "feedback": verbs.ProductStrategist,
	"release": verbs.DevOps,
}

// OwningSeat picks the seat whose twin works the decision: the first topic
// rule that matches the question, else the issue type's seat, else the
// Engineering Manager (delivery and alignment). why says which.
func OwningSeat(question, issueType string) (seat verbs.SeatID, why string) {
	for _, r := range route {
		if m := r.re.FindString(question); m != "" {
			return r.seat, fmt.Sprintf("the question mentions %q", strings.ToLower(m))
		}
	}
	if s, ok := typeSeat[issueType]; ok {
		return s, "the issue is a " + issueType
	}
	return verbs.EngineeringManager, "delivery and alignment is the default"
}

var (
	doorRE    = regexp.MustCompile(`(?i)\b(one|two)-way door\b`)
	optionRE  = regexp.MustCompile(`(?m)^(?:(?:[-*+]|\d+[.)])\s+|###\s+)`) // top-level items or ### headings; nested bullets are trade-offs
	h2RE      = regexp.MustCompile(`(?m)^##\s+(.+)$`)
	sentences = regexp.MustCompile(`[.?!](\s|$)`)
)

// CheckPack returns what keeps a decision pack from being decidable in one
// sitting: a missing or out-of-order part, a question that is not one
// sentence, other than two or three options, no door on the recommendation,
// a decision that is not the last part or is more than one line, more than
// PackMaxWords words, and every
// finding of the evidence rule (seats.CheckGrounding) against the grounding.
func CheckPack(md, grounding string) []string {
	return CheckPackWith(md, func(c string) bool { return strings.Contains(grounding, c) })
}

// CheckPackWith is CheckPack with a citation resolver (seats.CheckGroundingWith).
func CheckPackWith(md string, known func(cite string) bool) []string {
	var out []string
	locs := h2RE.FindAllStringSubmatchIndex(md, -1)
	type part struct {
		title, body string
	}
	var parts []part
	for i, l := range locs {
		end := len(md)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		parts = append(parts, part{strings.TrimSpace(md[l[2]:l[3]]), strings.TrimSpace(md[l[1]:end])})
	}
	find := func(title string) (int, string) {
		for i, p := range parts {
			h, t := strings.ToLower(p.title), strings.ToLower(title)
			if rest, ok := strings.CutPrefix(h, t); ok && (rest == "" || strings.IndexAny(rest[:1], " :(-—") == 0) {
				return i, p.body
			}
		}
		return -1, ""
	}
	last := -1
	for _, s := range PackSections {
		i, body := find(s.Title)
		switch {
		case i < 0:
			out = append(out, fmt.Sprintf("missing part %q: %s", "## "+s.Title, s.What))
			continue
		case i < last:
			out = append(out, fmt.Sprintf("part %q is out of order", s.Title))
		case body == "":
			out = append(out, fmt.Sprintf("part %q is empty", s.Title))
		}
		last = i
	}
	if _, q := find("Question"); q != "" {
		first := strings.SplitN(q, "\n\n", 2)[0]
		if n := len(sentences.FindAllString(first, -1)); n != 1 {
			out = append(out, fmt.Sprintf("the question is %d sentences; restate it as one", n))
		}
	}
	if i, o := find("Options"); i >= 0 {
		if n := len(optionRE.FindAllString(o, -1)); n < 2 || n > 3 {
			out = append(out, fmt.Sprintf("%d options; offer two or three, each with its trade-offs", n))
		}
	}
	if i, r := find("Recommendation"); i >= 0 && !doorRE.MatchString(r) {
		out = append(out, "the recommendation does not say whether it is a one-way or a two-way door")
	}
	if i, d := find("Your decision"); i >= 0 {
		if i != len(parts)-1 {
			out = append(out, "the pack must end with \"## Your decision\"")
		}
		if strings.Count(strings.TrimSpace(d), "\n\n") > 0 {
			out = append(out, "the decision is more than one paragraph; ask for the one thing to decide")
		}
	}
	if n := seats.Words(md); n > PackMaxWords {
		// The target sits a fifth under the limit: revisions in run 39027321306 cut about 50 words less than they were
		// told, so a target a tenth under left them just over it.
		aim := PackMaxWords * 4 / 5
		out = append(out, fmt.Sprintf("%d words: a pack a busy human decides from in five minutes is at most %d; cut at least %d words, to about %d, "+
			"by dropping the weakest fact, trade-off, or question and citing detail instead of pasting it", n, PackMaxWords, n-aim, aim))
	}
	return append(out, seats.CheckGroundingWith(md, known)...)
}

// PackMaxWords bounds a pack: a busy human decides from it in under five
// minutes, so detail is linked, not pasted.
const PackMaxWords = 500

// PackTemplate is the outline the twin fills, one line per part.
func PackTemplate() string {
	var b strings.Builder
	for _, s := range PackSections {
		fmt.Fprintf(&b, "## %s\n\n%s\n\n", s.Title, s.What)
	}
	return b.String()
}
