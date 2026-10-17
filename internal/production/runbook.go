package production

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/seats"
)

// RunbookDir holds the product's runbooks, one Markdown file per known
// failure (protected with ops/**: a person reviews every change).
const RunbookDir = "ops/runbooks"

// Runbook is one runbook entry `aayu runbook` checks: how to confirm a known
// failure with a bounded query, mitigate it through the catalog, verify the
// mitigation, and roll it back, and when that was last tried.
var Runbook = Kind{
	Name: "runbook", Practice: "runbooks-toil",
	Sections: []Section{
		{Title: "Symptom", What: "The alert or the symptom a person sees, and the SLO in `ops/slos.toml` it protects, by name.",
			must: []rule{must("`[a-z][a-z0-9-]*`", "name the SLO or alert it answers in backticks")}},
		{Title: "Confirm", What: "The bounded query that confirms it: `aayu investigate <incident> --telemetry <dir> --query \"…\"`, or a fenced template for the live system with a window and a row limit.",
			must: []rule{must("(--query\\s+\"|```(kql|sql|cwl|logsinsights|promql|logql|gcl|hogql|sentry))", "give the bounded query that confirms the failure")}},
		{Title: "Mitigate", What: "The catalog action that mitigates it, as `aayu respond <incident> --action <name> --params …`, with its risk; a person approves it through the Environment.",
			must: []rule{must(`aayu respond\s+(<incident>|#?\d+)\s+--action\s+[a-z0-9-]+`, "mitigate only with `aayu respond <incident> --action <name>` from the catalog")}},
		{Title: "Verify", What: "The signal that shows the mitigation worked: the bounded query and the value to expect, and how long to watch.",
			must: []rule{must(`(--query|query|expect)`, "name the query and the value that verifies the mitigation")}},
		{Title: "Rollback", What: "How to undo the mitigation: its catalog rollback action, or `none` and why."},
		{Title: "Tried", What: "When the runbook was last tried end to end (a drill or an incident) and the aayudha-ops run or incident that shows it, or `not yet`."},
	},
}

// SLODoc is the SLO document `aayu slo --draft` checks: the journey, the
// objectives, the error budget, the alerts, and the policy that says what
// happens to releases when the budget is spent.
var SLODoc = Kind{
	Name: "slo", Practice: "slo-error-budgets",
	Sections: []Section{
		{Title: "Journey", What: "Whose journey the SLO protects and what they are trying to get done, from the user's side."},
		{Title: "Objectives", What: "Each SLO by its name in `ops/slos.toml`: the SLI (availability or latency under a threshold), the objective as a percentage, and the window.",
			must: []rule{must(`\d+(\.\d+)?\s?%`, "state each objective as a percentage"), must(`\b\d+\s?(d|days?)\b`, "state each SLO's window in days")}},
		{Title: "Error budget", What: "The budget each objective allows and what is left now, cited to a bounded query or the recorded status.",
			must: []rule{must(`budget`, "state the error budget")}},
		{Title: "Alerts", What: "The burn-rate alert for each SLO (for example a fast 1h and a slow 6h window) and the runbook it opens.",
			must: []rule{must(`burn`, "alert on the budget's burn rate, not on single errors")}},
		{Title: "Budget policy", What: "What happens when the budget is spent: release ships only Bugs until it recovers, unless a person accepts an `error-budget` waiver; who decides; how it escalates.",
			must: []rule{must(`(release|ship)`, "say what happens to releases when the budget is spent")}},
	},
}

var (
	runbookRespond = regexp.MustCompile(`aayu respond\s+(?:<incident>|#?\d+)\s+--action\s+([a-z0-9-]+)(?:\s+--params\s+([^\s` + "`" + `]+))?`)
	queryArg       = regexp.MustCompile(`--query\s+"([^"]+)"`)
	backticked     = regexp.MustCompile("`([a-z][a-z0-9-]*)`")
	tomlFence      = regexp.MustCompile("(?s)```toml\\s*\\n(.*?)```")
)

// CheckRunbook is every deterministic check on a runbook entry: its parts,
// the stage's artefact, the evidence rule, an SLO from slos (when the product
// has any) named in the symptom, every confirm query bounded and valid, every
// proposed action valid in the catalog, the mitigation's catalog rollback
// named, no claimed execution and no command outside the catalog.
func CheckRunbook(md, stage string, pr seats.Practice, known func(string) bool, action ActionCheck, rollback func(name string) string, slos *SLOs) []string {
	out := Runbook.CheckParts(md)
	out = append(out, checkCommon(md, stage, pr, known)...)
	if sym, ok := Part(md, "Symptom"); ok && slos != nil {
		named := false
		for _, m := range backticked.FindAllStringSubmatch(sym, -1) {
			if _, ok := slos.Find(m[1]); ok {
				named = true
			}
		}
		if !named {
			var names []string
			for _, o := range slos.SLOs {
				names = append(names, o.Name)
			}
			out = append(out, fmt.Sprintf("the symptom names no SLO of %s (%s)", SLOPath, strings.Join(names, ", ")))
		}
	}
	now := time.Now().UTC()
	for _, m := range queryArg.FindAllStringSubmatch(md, -1) {
		q, err := ParseQuery(m[1], now.Add(-time.Hour), now)
		if err == nil {
			if q.Where["service"] == "" {
				q.Where["service"] = "<service>" // investigate scopes it to the incident's service
			}
			err = q.Check()
		}
		if err != nil {
			out = append(out, fmt.Sprintf("query %q: %v", m[1], err))
		}
	}
	mit, _ := Part(md, "Mitigate")
	rb, _ := Part(md, "Rollback")
	for _, m := range runbookRespond.FindAllStringSubmatch(md, -1) {
		if why := action(m[1], m[2]); why != "" {
			out = append(out, fmt.Sprintf("proposed action %q: %s", m[1], why))
			continue
		}
		if r := rollback(m[1]); strings.Contains(mit, m[1]) && r != "" && r != "none" && !strings.Contains(rb, r) {
			out = append(out, fmt.Sprintf("part \"Rollback\": the catalog's rollback of %s is %s; name it", m[1], r))
		}
	}
	out = append(out, checkNoExecution(md, "Mitigate")...)
	return append(out, checkQueryTemplates(md)...)
}

// CheckSLODoc is every deterministic check on an SLO document: its parts, the
// stage's artefact, the evidence rule, bounded query templates, every SLO of
// defs named in the objectives, and a fenced toml block (the proposed
// ops/slos.toml) that parses.
func CheckSLODoc(md, stage string, pr seats.Practice, known func(string) bool, defs *SLOs) []string {
	out := SLODoc.CheckParts(md)
	out = append(out, checkCommon(md, stage, pr, known)...)
	if obj, ok := Part(md, "Objectives"); ok {
		if proposed, err := ProposedSLOs(md); err != nil {
			out = append(out, err.Error())
		} else if proposed != nil {
			defs = proposed
		}
		if defs != nil {
			for _, o := range defs.SLOs {
				if !strings.Contains(obj, o.Name) {
					out = append(out, fmt.Sprintf("part \"Objectives\" does not name SLO %q", o.Name))
				}
			}
		}
	}
	out = append(out, checkNoExecution(md, "Budget policy")...)
	return append(out, checkQueryTemplates(md)...)
}

// ProposedSLOs parses the fenced toml block of an SLO document, or returns
// nil when it has none.
func ProposedSLOs(md string) (*SLOs, error) {
	ms := tomlFence.FindAllStringSubmatch(md, -1)
	if len(ms) == 0 {
		return nil, nil
	}
	if len(ms) > 1 {
		return nil, fmt.Errorf("the document has %d toml blocks; keep one, the proposed %s", len(ms), SLOPath)
	}
	s, err := ParseSLOs([]byte(ms[0][1]))
	if err != nil {
		return nil, fmt.Errorf("the proposed %s does not parse: %w", SLOPath, err)
	}
	return s, nil
}

// RunbookName is the file name a runbook is created under: its title's
// subject, lowercased, with runs of other characters as one -.
func RunbookName(md string) string {
	title := ""
	if m := regexp.MustCompile(`(?m)^#\s+(.+)$`).FindStringSubmatch(md); m != nil {
		title = m[1]
		if _, s, ok := strings.Cut(title, ":"); ok {
			title = s
		}
	}
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(strings.TrimSpace(title)) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	name := strings.Trim(b.String(), "-")
	if len(name) > 60 {
		name = strings.Trim(name[:60], "-")
	}
	return name
}

// Uncovered lists the SLOs that name an alert but no runbook, or a runbook
// that is not in have.
func (s *SLOs) Uncovered(have []string) []string {
	var out []string
	for _, o := range s.SLOs {
		if o.Alert != "" && (o.Runbook == "" || !slices.Contains(have, o.Runbook)) {
			out = append(out, o.Name)
		}
	}
	return out
}
