package seats

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
)

// The evidence rule (EvidenceRule) as a deterministic check on a twin's
// Markdown output (WP5.1b). A unit is a paragraph, a list item, or a table
// row. A unit that makes a claim about the code, architecture, risk, or
// progress must carry a citation; questions are exempt, and a unit labelled
// "Analogy:" or "Anecdote:" is exempt only when a cited unit stands next to
// it, because an illustration never replaces evidence. With a grounding text,
// every citation must name something the grounding contains, so a citation
// cannot be invented.

var (
	claimCue = regexp.MustCompile(`(?i)\b(code|files?|functions?|methods?|packages?|modules?|layers?|L0[1-9]|architecture|boundar(y|ies)|contracts?|interfaces?|api|schemas?|migrations?|registry|gates?|floors?|thresholds?|coverage|tests?|checkpoints?|packs?|proof|merged?|PRs?|pull requests?|commits?|branch(es)?|builds?|deploy\w*|releases?|incidents?|risks?|regress\w*|bugs?|defects?|estimates?|effort|budgets?|cost|spent|velocity|progress|shipped|blocked|latency|metrics?|baselines?|stor(y|ies)|corrections?|overr?an|flak\w*|struct|fields?|tombstones?|ledger|plan)\b|\d+(\.\d+)?\s?%`)

	citationKinds = []string{"path", "dir", "issue", "adr", "symbol", "commit", "run", "config"}
	citationREs   = map[string]*regexp.Regexp{
		"path":   regexp.MustCompile(`(?:[A-Za-z0-9_.<>-]+/)*[A-Za-z0-9_<>-][A-Za-z0-9_.<>-]*\.(?:go|md|toml|ya?ml|jsonl?|tsx?|m?js|sh|sql|txt|csv|html|css|py)\b(?::\d+(?:-\d+)?)?`),
		"dir":    regexp.MustCompile("`[A-Za-z0-9_.-]+/[A-Za-z0-9_.*<>/-]*`"),
		"issue":  regexp.MustCompile(`(?:^|[^\w&/])#(\d+)\b`),
		"adr":    regexp.MustCompile(`\bADR-\d+\b`),
		"symbol": regexp.MustCompile("`([a-z][A-Za-z0-9_]*\\.[A-Z][A-Za-z0-9_]*(?:\\.[A-Za-z0-9_]+)*)(?:\\(\\))?`"),
		"commit": regexp.MustCompile("`([0-9a-f]{7,40})`"),
		"run":    regexp.MustCompile(`\brun (\d{6,})\b`),
		"config": regexp.MustCompile("`\\[[a-z_.]+\\]`"),
	}

	illustration = regexp.MustCompile(`^(?:[*_]{1,2})?(Analogy|Anecdote)\b`)
	listItem     = regexp.MustCompile(`^\s*(?:[-*+]|\d+[.)])\s+`)
	tableRule    = regexp.MustCompile(`^\|?\s*:?-{3,}`)
	headingLine  = regexp.MustCompile(`^#{1,6}\s+(.*)$`)
)

// unit is one paragraph, list item, or table row of a Markdown document.
type unit struct{ text string }

// parse splits Markdown into units and headings, skipping code fences and
// HTML comments.
func parse(md string) (units []unit, headings []string) {
	var cur []string
	flush := func() {
		if t := strings.TrimSpace(strings.Join(cur, " ")); t != "" {
			units = append(units, unit{t})
		}
		cur = nil
	}
	fence, comment, lastRow := "", false, 0
	for _, line := range strings.Split(strings.ReplaceAll(md, "\r\n", "\n"), "\n") {
		t := strings.TrimSpace(line)
		switch {
		case fence != "":
			if strings.HasPrefix(t, fence) {
				fence = ""
			}
			continue
		case strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~"):
			flush()
			fence = t[:3]
			continue
		case comment:
			if strings.Contains(t, "-->") {
				comment = false
			}
			continue
		case strings.HasPrefix(t, "<!--"):
			flush()
			comment = !strings.Contains(t, "-->")
			continue
		}
		t = strings.TrimSpace(strings.TrimLeft(t, ">")) // blockquotes read as paragraphs
		switch {
		case t == "":
			flush()
		case headingLine.MatchString(t):
			flush()
			headings = append(headings, headingLine.FindStringSubmatch(t)[1])
		case strings.HasPrefix(t, "|"):
			flush()
			switch {
			case !tableRule.MatchString(t):
				units = append(units, unit{t})
				lastRow = len(units)
			case lastRow == len(units) && lastRow > 0:
				units = units[:lastRow-1] // the row above a delimiter row is the header: labels, not claims
			}
		case listItem.MatchString(t):
			flush()
			cur = append(cur, listItem.ReplaceAllString(t, ""))
		default:
			cur = append(cur, t)
		}
	}
	flush()
	return units, headings
}

// citations returns the citations in s: the cited path, issue, symbol, commit,
// or run, each as the string the grounding must contain.
func citations(s string) []string {
	var out []string
	for _, kind := range citationKinds {
		for _, m := range citationREs[kind].FindAllStringSubmatch(s, -1) {
			switch kind {
			case "path":
				p, _, _ := strings.Cut(m[0], ":")
				out = append(out, p)
			case "dir":
				out = append(out, strings.Trim(m[0], "`"))
			case "issue":
				out = append(out, "#"+m[1])
			case "symbol", "commit", "run":
				out = append(out, m[1])
			default:
				out = append(out, strings.Trim(m[0], "`"))
			}
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

func clip(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}

// CheckGrounding returns the evidence-rule findings for a twin's Markdown
// output. grounding is the text the twin was given; when it is not empty,
// every citation must name something it contains.
func CheckGrounding(md, grounding string) []string {
	if grounding == "" {
		return CheckGroundingWith(md, nil)
	}
	return CheckGroundingWith(md, func(c string) bool { return strings.Contains(grounding, c) })
}

// CheckGroundingWith is CheckGrounding with a resolver: known reports whether
// a citation names a real source (nil accepts every citation).
func CheckGroundingWith(md string, known func(cite string) bool) []string {
	units, _ := parse(md)
	var out []string
	statements := 0
	cited := make([]bool, len(units))
	for i, u := range units {
		cited[i] = len(citations(u.text)) > 0
	}
	for i, u := range units {
		switch {
		case illustration.MatchString(u.text):
			if beside := (i > 0 && cited[i-1]) || (i+1 < len(units) && cited[i+1]); !beside {
				out = append(out, fmt.Sprintf("illustration without the cited point it illustrates: %q", clip(u.text, 90)))
			}
		case strings.HasSuffix(u.text, "?") && !strings.Contains(strings.TrimSuffix(u.text, "?"), ". "):
			// a question to the reader claims nothing
		case claimCue.MatchString(u.text) && !cited[i]:
			statements++
			out = append(out, fmt.Sprintf("uncited claim: %q", clip(u.text, 90)))
		default:
			statements++
		}
		if known == nil {
			continue
		}
		for _, c := range citations(u.text) {
			if !known(c) {
				out = append(out, fmt.Sprintf("cites %s, which the grounding does not contain", c))
			}
		}
	}
	if statements > 0 && !slices.Contains(cited, true) {
		out = append(out, "cites nothing at all")
	}
	return out
}

// normHeading lowercases a heading and strips emphasis, code marks, and a
// trailing colon.
func normHeading(h string) string {
	h = strings.NewReplacer("*", "", "_", "", "`", "").Replace(h)
	return strings.ToLower(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(h), ":")))
}

// Words counts the prose words of a Markdown document (code fences and HTML
// comments excluded).
func Words(md string) int {
	units, headings := parse(md)
	n := 0
	for _, u := range units {
		n += len(strings.Fields(u.text))
	}
	for _, h := range headings {
		n += len(strings.Fields(h))
	}
	return n
}

// CheckStage returns the findings for a twin's output at a team stage: it
// must carry the stage's artefact for the practice as a heading, must not
// carry another stage's artefact, and a startup output stays on one page
// (StartupMaxWords).
func CheckStage(md string, pr Practice, stage string) []string {
	_, headings := parse(md)
	has := func(artefact string) bool {
		a := strings.ToLower(artefact)
		for _, h := range headings {
			if strings.HasPrefix(normHeading(h), a) {
				return true
			}
		}
		return false
	}
	var out []string
	if want := pr.At(stage).Artefact; !has(want) {
		out = append(out, fmt.Sprintf("no heading for the %s artefact %q of practice %s", stage, want, pr.ID))
	}
	for _, st := range config.TeamStages {
		if st != stage && has(pr.At(st).Artefact) {
			out = append(out, fmt.Sprintf("carries the %s artefact %q at the %s stage", st, pr.At(st).Artefact, stage))
		}
	}
	if stage == config.StageStartup {
		if n := Words(md); n > StartupMaxWords {
			out = append(out, fmt.Sprintf("%d words: a startup gets the one-page version (at most %d)", n, StartupMaxWords))
		}
	}
	return out
}
