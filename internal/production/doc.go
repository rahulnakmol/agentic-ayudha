package production

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/seats"
)

// rule is one thing a part must say, as a pattern and the reason it is asked for.
type rule struct {
	re  *regexp.Regexp
	why string
}

func must(pattern, why string) rule { return rule{regexp.MustCompile(`(?i)` + pattern), why} }

// Section is one required part of a document, written as a "## " heading that
// starts with Title.
type Section struct {
	Title string `json:"title"`
	What  string `json:"what"`
	must  []rule
}

// Kind is a kind of document the Production Engineer writes.
type Kind struct {
	Name     string    // investigation, postmortem, runbook, slo
	Practice string    // the persona practice whose stage artefact titles it
	Sections []Section // in order
	Marker   string    // opens its issue comment, so a rerun replaces it in place
}

var h2RE = regexp.MustCompile(`(?m)^##\s+(.+)$`)

type part struct{ title, body string }

func parts(md string) []part {
	var out []part
	locs := h2RE.FindAllStringSubmatchIndex(md, -1)
	for i, l := range locs {
		end := len(md)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		out = append(out, part{strings.TrimSpace(md[l[2]:l[3]]), strings.TrimSpace(md[l[1]:end])})
	}
	return out
}

// Part returns the body of the first part whose heading starts with title.
func Part(md, title string) (string, bool) {
	ps := parts(md)
	if i := find(ps, title); i >= 0 {
		return ps[i].body, true
	}
	return "", false
}

func find(ps []part, title string) int {
	t := strings.ToLower(title)
	for i, p := range ps {
		h := strings.ToLower(strings.NewReplacer("*", "", "`", "").Replace(p.title))
		if rest, ok := strings.CutPrefix(h, t); ok && (rest == "" || strings.IndexAny(rest[:1], " :(-—,") == 0) {
			return i
		}
	}
	return -1
}

// CheckParts returns what keeps md from having every part of k in order, each
// non-empty and saying what it must.
func (k Kind) CheckParts(md string) []string {
	ps := parts(md)
	var out []string
	last := -1
	for _, s := range k.Sections {
		i := find(ps, s.Title)
		switch {
		case i < 0:
			out = append(out, fmt.Sprintf("missing part %q: %s", "## "+s.Title, s.What))
			continue
		case i < last:
			out = append(out, fmt.Sprintf("part %q is out of order", s.Title))
		case ps[i].body == "":
			out = append(out, fmt.Sprintf("part %q is empty", s.Title))
			continue
		}
		last = i
		for _, r := range s.must {
			if !r.re.MatchString(ps[i].body) {
				out = append(out, fmt.Sprintf("part %q: %s", s.Title, r.why))
			}
		}
	}
	return out
}

// Template is the outline the twin fills: the artefact title, then one line per part.
func (k Kind) Template(artefact, subject string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s: %s\n\n", artefact, subject)
	for _, s := range k.Sections {
		fmt.Fprintf(&b, "## %s\n\n%s\n\n", s.Title, s.What)
	}
	return b.String()
}

// Comment is the issue comment that carries a checked document.
func (k Kind) Comment(md, footer string) string {
	return k.Marker + "\n" + strings.TrimSpace(md) + "\n\n_" + footer + "_\n"
}

// checkCommon is every check that applies to all kinds: the stage's artefact
// as the title (with the one-page bound at a startup) and the evidence rule
// with known resolving each citation.
func checkCommon(md, stage string, pr seats.Practice, known func(string) bool) []string {
	return append(seats.CheckStage(md, pr, stage), seats.CheckGroundingWith(md, known)...)
}

// executed catches a claim that a mitigation, a deploy, or any production
// change has already been made by the writer: the twin only proposes, and a
// person approves a catalog action through the GitHub Environment.
var executed = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(I|we|Atlas)\s+(have\s+|already\s+|just\s+)?(restarted|rolled\s+back|redeployed|deployed|reverted|executed|ran|scaled|flushed|applied|disabled|enabled|killed|purged|failed\s+over|mitigated)\b`),
	regexp.MustCompile(`(?i)\b(the\s+)?(rollback|restart|redeploy|mitigation|catalog\s+action|action)\s+(was|has\s+been|is\s+now)\s+(executed|applied|run|completed|performed|done|deployed)\b`),
}

// shellCommand is a command that would change production outside the catalog,
// including a script run with flags (`deploy.sh --rollback`): the catalog
// action that wraps it is the only way in.
var shellCommand = regexp.MustCompile("(?im)(^|`|\\$\\s*)(kubectl|systemctl|service\\s+\\S+\\s+restart|docker|podman|ssh|sudo|helm|terraform|az\\s|aws\\s|gcloud\\s|git\\s+(revert|push|reset)|rm\\s+-|pkill|kill\\s|curl\\s+-X\\s*(POST|PUT|DELETE|PATCH)|psql|mysql|redis-cli|\\S+\\.sh\\s+-|bash\\s|sh\\s+-c)")

// checkNoExecution returns findings for claimed executions and for production
// commands outside `aayu respond --action`.
func checkNoExecution(md, part string) []string {
	var out []string
	for _, re := range executed {
		if m := re.FindString(md); m != "" {
			out = append(out, fmt.Sprintf("%q claims a production change was made; propose it as `aayu respond <incident> --action <name>` for a person to approve, never as done", m))
		}
	}
	if body, ok := Part(md, part); ok {
		if m := shellCommand.FindString(body); m != "" {
			out = append(out, fmt.Sprintf("part %q runs %q outside the catalog; every mitigation is an `aayu respond --action` proposal", part, strings.Trim(m, "`$ ")))
		}
	}
	return out
}

var templateFence = regexp.MustCompile("(?s)```(kql|sql|logsinsights|cwl|promql|logql|gcl|hogql|sentry)\\s*\\n(.*?)```")

var (
	boundWindow = regexp.MustCompile(`(?i)(ago\(|between|timestamp|timegenerated|time_range|now\(\)|interval|\bstart\b|\bend\b|>=|@timestamp|\[\d+[smhd]\]|statsperiod)`)
	boundLimit  = regexp.MustCompile(`(?i)\b(take|limit|top|head|per_page|max_results)\b`)
)

// checkQueryTemplates returns findings for a live-source query template (a
// fenced kql, sql, logsinsights, cwl, promql, logql, gcl, hogql, or sentry
// block) that does not bound its window and its rows.
func checkQueryTemplates(md string) []string {
	var out []string
	for _, m := range templateFence.FindAllStringSubmatch(md, -1) {
		if !boundWindow.MatchString(m[2]) {
			out = append(out, fmt.Sprintf("the %s query template names no time window; bound every query", m[1]))
		}
		if !boundLimit.MatchString(m[2]) {
			out = append(out, fmt.Sprintf("the %s query template has no row limit (take, limit, top); bound every query", m[1]))
		}
	}
	return out
}
