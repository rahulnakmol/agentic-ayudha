package survey

import (
	"fmt"
	"strings"
)

// CountedOnly names the languages of the checkout that the index counts but
// does not read, with their size.
func CountedOnly(ix *Index) []string {
	var out []string
	for _, l := range ix.Languages {
		if l.CountedOnly {
			out = append(out, fmt.Sprintf("%s (%d files, %d lines)", l.Name, l.Files, l.Lines))
		}
	}
	return out
}

// Render writes the survey report (SURVEY.md) from a checked draft: every
// claim with its basis and its citations, the capability map, the questions
// for the person, the modernisation path, what the index could not see, and
// how much of the index the draft cites.
func Render(ix *Index, d Draft, r Report) string {
	var b strings.Builder
	p := func(format string, args ...any) { fmt.Fprintf(&b, format, args...) }
	p("# Survey: %s\n\n", oneLine(d.Product))
	p("%s\n\n", oneLine(d.Summary))
	p("Asked for: %q. Indexed at commit `%s`: %d files", oneLine(d.Request), short(ix.Commit), len(ix.Files))
	var ls []string
	for _, l := range ix.Languages {
		ls = append(ls, fmt.Sprintf("%s (%d lines)", l.Name, l.Lines))
	}
	if len(ls) > 0 {
		p(", %s", strings.Join(ls, ", "))
	}
	p(".\n\n")
	p("Each claim says how it is known: **observed** (the code shows it), **stated** (a document or comment says it), " +
		"**inferred** (reasoned from the cited code), **confirmed** (the person said it), or **unknown** (a question asks it). " +
		"Citations are index fact IDs or `path:line` ranges at the commit above.\n\n")

	p("## Limits of this survey\n\n")
	ambiguous, calls := 0, 0
	for _, e := range ix.Edges {
		if e.Kind == "call" {
			calls++
			if e.Ambiguous {
				ambiguous++
			}
		}
	}
	p("- Calls are resolved by name only: %d of %d call edges name more than one definition.\n", ambiguous, calls)
	var dyn []string
	for _, s := range ix.Surfaces {
		if s.Kind == "dynamic-dispatch" {
			dyn = append(dyn, "`"+s.At+"`")
		}
	}
	if len(dyn) > 0 {
		p("- Dynamic dispatch the index cannot follow: %s.\n", strings.Join(dyn, ", "))
	}
	if co := CountedOnly(ix); len(co) > 0 {
		p("- Counted by file and line only, never read (no symbols, calls, imports, or traps, so no claim about them comes from the index): %s.\n", strings.Join(co, ", "))
	}
	if len(ix.NotIndexed) > 0 {
		var ns []string
		for _, c := range ix.NotIndexed {
			ns = append(ns, fmt.Sprintf("%s (%d)", c.Name, c.Files))
		}
		p("- Files with no language the index knows: %s.\n", strings.Join(ns, ", "))
	}
	if h := ix.History; h != nil && h.Shallow {
		p("- The checkout is a shallow clone, so its history is partial.\n")
	}
	p("- Product claims come from documents, inference, or the person; code alone never shows who uses the software or why.\n\n")

	p("## Engineering\n\n")
	for _, topic := range Topics["engineering"] {
		var cs []Claim
		for _, c := range d.Engineering {
			if c.Topic == topic {
				cs = append(cs, c)
			}
		}
		if len(cs) == 0 {
			continue
		}
		p("### %s\n\n", strings.ToUpper(topic[:1])+topic[1:])
		for _, c := range cs {
			p("- %s\n", claimLine(c))
		}
		p("\n")
	}

	p("## Capability map\n\n")
	p("| ID | Capability | What it does | Basis | Evidence |\n|---|---|---|---|---|\n")
	for _, c := range d.Capabilities {
		p("| %s | %s | %s | %s | %s |\n", c.ID, cell(c.Name), cell(c.Text), basisText(c), cell(citeText(c.Cites)))
	}
	p("\n## Product\n\n")
	for _, c := range d.ProductClaims {
		p("- %s\n", claimLine(c))
	}
	p("\n## Questions for the person\n\n")
	if len(d.Questions) == 0 {
		p("None.\n\n")
	} else {
		p("| ID | Question | Why it matters | Default | Status |\n|---|---|---|---|---|\n")
		for _, q := range d.Questions {
			status := q.Status
			switch q.Status {
			case "answered":
				status = fmt.Sprintf("answered: %q", oneLine(q.Answer))
			case "deferred":
				status = "deferred: " + oneLine(q.Reason)
			}
			p("| %s | %s | %s | %s | %s |\n", q.ID, cell(q.Text), cell(q.Why), cell(orDash(q.Default)), cell(status))
		}
		p("\n")
	}
	if m := d.Modernisation; m != nil {
		p("## Modernisation path: %s\n\n", oneLine(m.Target))
		src := "the request"
		if m.TargetSource != "request" {
			src = "the answer to " + m.TargetSource
		}
		p("Target from %s. %s", src, oneLine(m.Strategy))
		if len(m.Cites) > 0 {
			p(" (%s)", citeText(m.Cites))
		}
		p("\n\n### Steps\n\n")
		for k, s := range m.Steps {
			p("%d. **%s** %s.", k+1, s.ID, strings.TrimSuffix(oneLine(s.Title), "."))
			if len(s.Modules) > 0 {
				p(" Replaces %s.", citeText(s.Modules))
			}
			if len(s.DependsOn) > 0 {
				p(" After %s.", strings.Join(s.DependsOn, ", "))
			}
			if len(s.Claims) > 0 {
				p(" Rests on %s.", strings.Join(s.Claims, ", "))
			}
			p(" Parity: %s.\n", strings.Join(s.Parity, ", "))
		}
		p("\n### Parity case sets\n\n")
		for _, g := range m.CaseSets {
			p("- **%s** %s. Inputs from %s; compared %s.", g.ID, strings.TrimSuffix(oneLine(g.Title), "."), citeText(g.From), oneLine(g.Compare))
			if g.Cases != "" {
				p(" Cases: `%s`.", g.Cases)
			}
			if len(g.Exclude) > 0 {
				p(" Not kept: %s.", strings.Join(g.Exclude, "; "))
			}
			p("\n")
		}
		if len(m.Risks) > 0 {
			p("\n### Modernisation risks\n\n")
			for _, c := range m.Risks {
				p("- %s\n", claimLine(c))
			}
		}
		p("\n")
	}
	cov := r.Coverage
	p("## Coverage\n\n")
	p("- Modules cited: %d of %d.", cov.ModulesCited, cov.Modules)
	if len(cov.UncitedModule) > 0 {
		p(" Not cited: %s.", citeText(cov.UncitedModule))
	}
	p("\n- Entry points cited: %d of %d.\n", cov.EntriesCited, cov.Entries)
	p("- Known-issue markers cited: %d of %d.", cov.MarkersCited, cov.Markers)
	if len(cov.UncitedMarker) > 0 {
		p(" Not cited: %s.", citeText(cov.UncitedMarker))
	}
	p("\n- Open questions: %d; unknown claims: %d.\n", cov.OpenQuestions, cov.Unknowns)
	if !r.Pass {
		p("\n## Problems\n\nThis draft does not pass the check yet:\n\n")
		for _, x := range r.Problems {
			p("- %s\n", oneLine(x))
		}
	}
	return b.String()
}

func claimLine(c Claim) string {
	s := fmt.Sprintf("**%s** (%s) %s", c.ID, basisText(c), strings.TrimSpace(oneLine(c.Text)))
	if len(c.Cites) > 0 {
		s += " " + citeText(c.Cites) + "."
	}
	if c.Reasoning != "" {
		s += " Reasoning: " + oneLine(c.Reasoning)
	}
	return s
}

func basisText(c Claim) string {
	switch c.Basis {
	case BasisConfirmed:
		return "confirmed, " + c.Answer
	case BasisUnknown:
		return "unknown, " + c.Question
	}
	return c.Basis
}

func citeText(cs []string) string {
	var out []string
	for _, c := range cs {
		out = append(out, "`"+strings.ReplaceAll(c, "`", "'")+"`")
	}
	return strings.Join(out, ", ")
}

// oneLine puts model-written text on one line and escapes what Markdown would
// otherwise read as HTML or emphasis (`block<Type>Continue`, `xss*`), outside
// code spans, so the report shows the text as written.
func oneLine(s string) string {
	parts := strings.Split(strings.Join(strings.Fields(s), " "), "`")
	for i := 0; i < len(parts); i += 2 {
		if i == len(parts)-1 && len(parts)%2 == 0 {
			break // an unclosed backtick: leave the tail as it is
		}
		parts[i] = mdEscape.Replace(parts[i])
	}
	return strings.Join(parts, "`")
}

var mdEscape = strings.NewReplacer(`\`, `\\`, "*", `\*`, "_", `\_`, "<", `\<`)

func cell(s string) string { return strings.ReplaceAll(oneLine(s), "|", `\|`) }

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "–"
	}
	return s
}
