package production

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/seats"
)

// Investigation is the investigation note `aayu investigate` checks: from the
// impact to the deployed revision, the evidence of bounded queries, competing
// hypotheses with their counter-evidence, the implicated code, a mitigation
// proposed through the catalog, and the durable fix, each with how to verify
// it and how to roll it back (adapted from the founder's Opsci advisor).
var Investigation = Kind{
	Name: "investigation", Practice: "telemetry-investigation", Marker: "<!-- aayudha:investigation -->",
	Sections: []Section{
		{Title: "Impact and timeline", What: "Who is affected, how badly, since when (UTC), and the events in order, each cited (a query, the timeline, a deploy marker)."},
		{Title: "Deployed revision", What: "The revision serving the window and the one before it, from the deploy marker and `service.version`, as commits; say if it is not the branch head.",
			must: []rule{must("`[0-9a-f]{7,40}`", "name the deployed revision as a commit in backticks")}},
		{Title: "Evidence", What: "What the bounded queries showed, each cited as `query qN` (its window, scope, and limit are in the query log); aggregates before records; observed facts only."},
		{Title: "Hypotheses", What: "At least two competing hypotheses (H1, H2, …), each with its supporting evidence, its counter-evidence, and the observation that would rule it out; then the confidence and why.",
			must: []rule{must(`\bH1\b`, "label the hypotheses H1, H2, …"), must(`\bH2\b`, "keep at least two competing hypotheses (H1 and H2)"), must(`counter-?evidence`, "give each hypothesis its counter-evidence")}},
		{Title: "Implicated code", What: "The code the evidence implicates at the deployed revision, as `path:line`, and the change that introduced it; or `Unverified:` with what would show it.",
			must: []rule{must(`(\.\w+:\d+|unverified)`, "cite the implicated code as path:line, or mark it Unverified: with what is missing")}},
		{Title: "Mitigation", What: "The immediate mitigation as one `aayu respond <incident> --action <name> --params …` proposal from the catalog (or, when none fits, \"no catalog action\" and the action a person should add), with its rationale, risk, the signal that verifies it, and its rollback or stop condition. Proposed, never executed.",
			must: []rule{must(`(aayu respond\s+#?\d+\s+--action\s+\S+|no catalog action)`, "propose the mitigation as `aayu respond <incident> --action <name>` from the catalog, or say no catalog action fits and name the one to add"), must(`\bverif`, "say how the mitigation is verified"), must(`(\brollback\b[^-]|\broll back\b|\bstop (condition|if|when)\b)`, "give the mitigation's rollback or stop condition")}},
		{Title: "Durable fix", What: "The fix that prevents a recurrence, for the follow-up Bug: the owning boundary, the paths, the regression test or scenario, how it is verified, and its rollback.",
			must: []rule{must(`regression`, "name the regression test or scenario"), must(`\bverif`, "say how the fix is verified"), must(`(\brollback\b[^-]|\broll back\b|\brevert)`, "give the fix's rollback")}},
		{Title: "Missing evidence and next queries", What: "What is not known yet, and the next bounded queries (source, window, scope, limit) or sanitised exports that would settle it."},
	},
}

var (
	queryCite  = regexp.MustCompile(`\bquery (q\d+)\b`)
	respondCmd = regexp.MustCompile(`aayu respond\s+#?(\d+)\s+--action\s+([a-z0-9-]+)(?:\s+--params\s+([^\s` + "`" + `]+))?`)
)

// ActionCheck validates a proposed catalog action with its parameters; it
// returns why the catalog would refuse it, or "".
type ActionCheck func(name, params string) string

// CheckInvestigation is every deterministic check on an investigation note:
// its parts, the stage's artefact, the evidence rule (known resolves each
// citation, `query qN` included), at least two queries cited, every proposed
// action valid in the catalog for this incident, no claimed execution and no
// production command outside the catalog, and bounded query templates.
func CheckInvestigation(md string, incident int, stage string, pr seats.Practice, known func(string) bool, action ActionCheck) []string {
	out := Investigation.CheckParts(md)
	out = append(out, checkCommon(md, stage, pr, known)...)
	if qs := CitedQueries(md); len(qs) < 2 {
		out = append(out, fmt.Sprintf("the note cites %d bounded queries; cite at least two as `query qN`", len(qs)))
	}
	cmds := respondCmd.FindAllStringSubmatch(md, -1)
	for _, m := range cmds {
		if m[1] != fmt.Sprint(incident) {
			out = append(out, fmt.Sprintf("`aayu respond %s` names another incident than #%d", m[1], incident))
		}
		if why := action(m[2], m[3]); why != "" {
			out = append(out, fmt.Sprintf("proposed action %q: %s", m[2], why))
		}
	}
	out = append(out, checkNoExecution(md, "Mitigation")...)
	return append(out, checkQueryTemplates(md)...)
}

// CitedQueries returns the query IDs a document cites, in order.
func CitedQueries(md string) []string {
	var qs []string
	for _, m := range queryCite.FindAllStringSubmatch(md, -1) {
		if !slices.Contains(qs, m[1]) {
			qs = append(qs, m[1])
		}
	}
	return qs
}

// HandOff is the part of a checked investigation note the follow-up Bug
// carries: the implicated code and the durable fix.
func HandOff(md string) string {
	var b strings.Builder
	for _, t := range []string{"Implicated code", "Durable fix"} {
		if body, ok := Part(md, t); ok {
			fmt.Fprintf(&b, "**%s.** %s\n\n", t, body)
		}
	}
	return strings.TrimSpace(b.String())
}

// Skills are the Production Engineer skills whose documents CheckDoc checks.
var Skills = []string{"investigate", "slo", "runbook", "respond"}

// CheckDoc runs the deterministic floors on a document a Production Engineer
// skill wrote, from the text and its grounding alone (the eval harness's
// tier-1 floors): every part of the skill's kind in order (respond writes the
// postmortem); every proposed catalog action one the grounding lists; no
// claimed execution and no production command outside the catalog; bounded
// queries and query templates; an investigation cites at least two queries
// and a postmortem is blameless. The evidence rule and the stage's artefact
// are checked by the caller.
func CheckDoc(skill, md, grounding string) []string {
	var out []string
	inCatalog := func(name string) {
		if !strings.Contains(grounding, "Catalog action `"+name+"`") {
			out = append(out, fmt.Sprintf("proposed action %q is not in the catalog the grounding lists", name))
		}
	}
	switch skill {
	case "investigate":
		out = Investigation.CheckParts(md)
		if qs := CitedQueries(md); len(qs) < 2 {
			out = append(out, fmt.Sprintf("the note cites %d bounded queries; cite at least two as `query qN`", len(qs)))
		}
		for _, m := range respondCmd.FindAllStringSubmatch(md, -1) {
			inCatalog(m[2])
		}
		out = append(out, checkNoExecution(md, "Mitigation")...)
	case "runbook":
		out = Runbook.CheckParts(md)
		for _, m := range runbookRespond.FindAllStringSubmatch(md, -1) {
			inCatalog(m[1])
		}
		for _, m := range queryArg.FindAllStringSubmatch(md, -1) {
			if err := boundedQuery(m[1]); err != nil {
				out = append(out, fmt.Sprintf("query %q: %v", m[1], err))
			}
		}
		out = append(out, checkNoExecution(md, "Mitigate")...)
	case "slo":
		out = SLODoc.CheckParts(md)
		if _, err := ProposedSLOs(md); err != nil {
			out = append(out, err.Error())
		}
		out = append(out, checkNoExecution(md, "Budget policy")...)
	case "respond":
		out = Postmortem.CheckParts(md)
		for _, re := range blame {
			for _, m := range re.FindAllString(md, -1) {
				out = append(out, fmt.Sprintf("%q blames a person; a blameless postmortem names the system, the conditions, and the decisions", m))
			}
		}
	default:
		return []string{fmt.Sprintf("%q is not a Production Engineer skill (%s)", skill, strings.Join(Skills, ", "))}
	}
	return append(out, checkQueryTemplates(md)...)
}
