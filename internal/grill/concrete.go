package grill

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// Sources of a criterion in the Provenance section: the human stated it,
// confirmed a proposal, or accepted a default offered with its reason.
const (
	SourceStated    = "stated"
	SourceConfirmed = "confirmed"
	SourceDefault   = "default"
)

// Sources lists the sources in the order the skill prefers them.
var Sources = []string{SourceStated, SourceConfirmed, SourceDefault}

// Source is one criterion's line in the Provenance section:
//
//	N. <stated|confirmed|default> (<dimension>[/<topic>]): "<the human's words>"; facts: <id>, <id>
type Source struct {
	N         int    `json:"n"`
	Kind      string `json:"kind"`
	Dimension string `json:"dimension,omitempty"` // catalogue dimension (ux.states), request, or other
	Topic     string `json:"topic,omitempty"`     // the question's topic (ux.steps/screen-count), when the line names one
	Quote     string `json:"quote"`
	// Quotes are the human's words one by one: a criterion that rests on
	// several replies quotes each (`"…" / "…"`), and an elided quote ("… …")
	// is its parts.
	Quotes []string `json:"quotes"`
	Facts  []string `json:"facts,omitempty"` // assessment fact IDs the criterion rests on
}

// Resolution is one "Vague:" line: a qualifier of the request resolved into
// criteria, or deferred to an open question.
type Resolution struct {
	Term     string `json:"term"`
	Criteria []int  `json:"criteria,omitempty"`
	Deferred string `json:"deferred,omitempty"`
}

// Provenance is the parsed Provenance section of a grilled Story or Bug: the
// request in the human's words, how each vague qualifier was resolved, and
// where each criterion came from, so it is auditable which criteria the human
// stated and which they confirmed.
type Provenance struct {
	Request  string       `json:"request"`
	Vague    []Resolution `json:"vague"`
	Terms    []TermDef    `json:"terms,omitempty"`
	Criteria []Source     `json:"criteria"`
}

// TermDef is one "Term:" line (WP5.8): an abbreviation or jargon term of the
// request with the meaning the human confirmed, in their words, or deferred.
//
//	Term: "PDP" → product detail page; "yes, the product page"
type TermDef struct {
	Term     string   `json:"term"`
	Meaning  string   `json:"meaning,omitempty"`
	Quotes   []string `json:"quotes,omitempty"`
	Deferred string   `json:"deferred,omitempty"`
}

var (
	requestLine = regexp.MustCompile(`(?i)^\s*request:\s*(.*)$`)
	vagueLine   = regexp.MustCompile(`(?i)^\s*vague:\s*(.+?)\s*(?:→|->)\s*(.*)$`)
	termLine    = regexp.MustCompile(`(?i)^\s*term:\s*(.+?)\s*(?:→|->)\s*(.*)$`)
	sourceLine  = regexp.MustCompile(`^\s*([0-9]+)[.)]\s+([a-z]+)\s*(?:\(([^)]*)\))?\s*:\s*(.*)$`)
	factsPart   = regexp.MustCompile(`(?i);\s*facts:\s*(.*)$`)
	numberRE    = regexp.MustCompile(`[0-9]`)
	listNumbers = regexp.MustCompile(`[0-9]+`)
	bullet      = regexp.MustCompile(`^\s*(?:[-*]|[0-9]+[.)])\s+`)
)

// ParseProvenance parses the Provenance section. Lines it does not know are
// problems, so a typo cannot hide a criterion's source.
func ParseProvenance(s string) (Provenance, []workitems.Problem) {
	var p Provenance
	var ps []workitems.Problem
	bad := func(format string, args ...any) {
		ps = append(ps, workitems.Problem{Field: workitems.FieldProvenance, Message: fmt.Sprintf(format, args...)})
	}
	for i, line := range strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if m := requestLine.FindStringSubmatch(line); m != nil {
			p.Request = unquote(m[1])
			continue
		}
		if m := vagueLine.FindStringSubmatch(line); m != nil {
			r := Resolution{Term: unquote(m[1])}
			rest := strings.TrimSpace(m[2])
			if d, ok := cutFold(rest, "deferred"); ok {
				r.Deferred = strings.TrimSpace(strings.TrimLeft(d, ": "))
				if r.Deferred == "" {
					r.Deferred = "deferred"
				}
			} else {
				for _, n := range listNumbers.FindAllString(rest, -1) {
					v, _ := strconv.Atoi(n)
					r.Criteria = append(r.Criteria, v)
				}
				if len(r.Criteria) == 0 {
					bad("line %d: %q resolves to no criteria; write \"→ criteria 1, 2\" or \"→ deferred: <why>\"", i+1, r.Term)
				}
			}
			p.Vague = append(p.Vague, r)
			continue
		}
		if m := termLine.FindStringSubmatch(line); m != nil {
			t := TermDef{Term: unquote(m[1])}
			rest := strings.TrimSpace(m[2])
			if d, ok := cutFold(rest, "deferred"); ok {
				t.Deferred = strings.TrimSpace(strings.TrimLeft(d, ": "))
				if t.Deferred == "" {
					t.Deferred = "deferred"
				}
			} else if mean, said, ok := strings.Cut(rest, ";"); ok && strings.TrimSpace(mean) != "" && strings.TrimSpace(said) != "" {
				t.Meaning, t.Quotes = unquote(mean), quotes(said)
			} else {
				bad("line %d: term %q: write `Term: \"%s\" → <meaning>; \"<the human's words confirming it>\"` or `→ deferred: <why>`", i+1, t.Term, t.Term)
			}
			p.Terms = append(p.Terms, t)
			continue
		}
		if m := sourceLine.FindStringSubmatch(line); m != nil {
			n, _ := strconv.Atoi(m[1])
			src := Source{N: n, Kind: strings.ToLower(m[2]), Dimension: strings.TrimSpace(m[3])}
			if dim, _, ok := strings.Cut(src.Dimension, "/"); ok {
				src.Topic, src.Dimension = src.Dimension, dim
			}
			rest := m[4]
			if f := factsPart.FindStringSubmatchIndex(rest); f != nil {
				for _, id := range strings.Split(rest[f[2]:f[3]], ",") {
					if id = strings.Trim(strings.TrimSpace(id), "`"); id != "" {
						src.Facts = append(src.Facts, id)
					}
				}
				rest = rest[:f[0]]
			}
			src.Quotes = quotes(rest)
			src.Quote = strings.Join(src.Quotes, " / ")
			if !slices.Contains(Sources, src.Kind) {
				bad("criterion %d: source %q: want one of %s", n, src.Kind, strings.Join(Sources, ", "))
			}
			if src.Quote == "" {
				bad("criterion %d: quote the human's words that stated or confirmed it", n)
			}
			p.Criteria = append(p.Criteria, src)
			continue
		}
		bad("line %d is not a Request, Vague, Term, or criterion source line (want `N. stated|confirmed|default (<dimension>): \"<the human's words>\"`)", i+1)
	}
	return p, ps
}

var quotedPart = regexp.MustCompile(`"([^"]*)"|“([^”]*)”`)

// quotes returns the quoted segments of a source line, each split at an
// ellipsis; a line without quotation marks is one quote.
func quotes(rest string) []string {
	var segs []string
	for _, m := range quotedPart.FindAllStringSubmatch(rest, -1) {
		segs = append(segs, m[1]+m[2])
	}
	if len(segs) == 0 {
		segs = []string{unquote(rest)}
	}
	var out []string
	for _, q := range segs {
		for _, part := range strings.FieldsFunc(strings.ReplaceAll(q, "...", "…"), func(r rune) bool { return r == '…' }) {
			if part = strings.TrimSpace(part); part != "" {
				out = append(out, part)
			}
		}
	}
	return out
}

func cutFold(s, prefix string) (string, bool) {
	if len(s) >= len(prefix) && strings.EqualFold(s[:len(prefix)], prefix) {
		return s[len(prefix):], true
	}
	return s, false
}

func unquote(s string) string {
	s = strings.TrimSpace(s)
	for _, q := range [][2]string{{`"`, `"`}, {"“", "”"}, {"`", "`"}} {
		if strings.HasPrefix(s, q[0]) && strings.HasSuffix(s, q[1]) && len(s) >= len(q[0])+len(q[1]) {
			return strings.TrimSpace(s[len(q[0]) : len(s)-len(q[1])])
		}
	}
	return s
}

// Term returns the Term line for an abbreviation of the request, matched
// case-insensitively and with or without a plural "s".
func (p Provenance) Term(text string) (TermDef, bool) {
	k := strings.ToLower(text)
	for _, t := range p.Terms {
		if tk := strings.ToLower(t.Term); tk == k || tk+"s" == k || tk == k+"s" {
			return t, true
		}
	}
	return TermDef{}, false
}

// Checkpoint is one line of a grilled spec's Checkpoints: what the human can
// see or try at that step of the build, and the criteria it reaches.
type Checkpoint struct {
	N        int    `json:"n"`
	Text     string `json:"text"`
	Criteria []int  `json:"criteria"`
}

var (
	checkpointLine = regexp.MustCompile(`^\s*([0-9]+)[.)]\s+(.+?)\s*\((?i:criteri(?:on|a))\s+([0-9][0-9 ,and]*)\)\.?\s*$`)
)

// ParseCheckpoints parses the Checkpoints section; a line without its
// criteria is a problem.
func ParseCheckpoints(s string) ([]Checkpoint, []workitems.Problem) {
	var out []Checkpoint
	var ps []workitems.Problem
	for _, l := range strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n") {
		if strings.TrimSpace(l) == "" {
			continue
		}
		m := checkpointLine.FindStringSubmatch(l)
		if m == nil {
			ps = append(ps, workitems.Problem{Field: workitems.FieldCheckpoints, Message: fmt.Sprintf("%q: write `N. <what the human can see or run> (criteria 1, 2)`", clip(strings.TrimSpace(l), 80))})
			continue
		}
		n, _ := strconv.Atoi(m[1])
		c := Checkpoint{N: n, Text: m[2]}
		for _, x := range listNumbers.FindAllString(m[3], -1) {
			v, _ := strconv.Atoi(x)
			c.Criteria = append(c.Criteria, v)
		}
		out = append(out, c)
	}
	return out, ps
}

// checkCheckpoints: a grilled spec says what the human can see at each step
// of the build, every checkpoint reaches criteria that exist, and every
// criterion is reached by one.
func checkCheckpoints(s string, cs []workitems.Criterion) []workitems.Problem {
	if workitems.Empty(s) {
		return []workitems.Problem{{Field: workitems.FieldCheckpoints, Message: "a grilled spec lists its checkpoints: what the human can see or try at each step of the build, in order, each naming the criteria it reaches (`1. <what you can see> (criteria 1, 2)`)"}}
	}
	cps, ps := ParseCheckpoints(s)
	have, reached := map[int]bool{}, map[int]bool{}
	for _, c := range cs {
		have[c.N] = true
	}
	for i, cp := range cps {
		if cp.N != i+1 {
			ps = append(ps, workitems.Problem{Field: workitems.FieldCheckpoints, Message: fmt.Sprintf("checkpoint %d is numbered %d; number them 1, 2, … in order", i+1, cp.N)})
		}
		for _, n := range cp.Criteria {
			if !have[n] {
				ps = append(ps, workitems.Problem{Field: workitems.FieldCheckpoints, Message: fmt.Sprintf("checkpoint %d reaches criterion %d, which does not exist", cp.N, n)})
			}
			reached[n] = true
		}
	}
	for _, c := range cs {
		if !reached[c.N] && len(cps) > 0 {
			ps = append(ps, workitems.Problem{Field: workitems.FieldCheckpoints, Message: fmt.Sprintf("no checkpoint reaches criterion %d; name it in the checkpoint where the human can first see it", c.N)})
		}
	}
	return ps
}

// Source returns the provenance of criterion n.
func (p Provenance) Source(n int) (Source, bool) {
	for _, s := range p.Criteria {
		if s.N == n {
			return s, true
		}
	}
	return Source{}, false
}

// Check reports what keeps a Story's or Bug's spec from being concrete
// (WP5.3b). Always: a criterion or the Threshold that uses a vague qualifier
// with nothing to observe (no number outside quotes). When the item was
// grilled (it has a Provenance section): every criterion has exactly one
// source, every qualifier of the request is resolved into criteria or
// deferred to an open question, assumptions are listed, every open question
// is answered or deferred, and no parked idea appears in a criterion.
func Check(cat Catalog, it workitems.Item) []workitems.Problem {
	t, ok := workitems.FindType(it.Type)
	if !ok || !t.HasCompletionGoal() {
		return nil
	}
	var ps []workitems.Problem
	add := func(field, format string, args ...any) {
		ps = append(ps, workitems.Problem{Field: field, Message: fmt.Sprintf(format, args...)})
	}
	cs, _ := workitems.ParseCriteria(it.Fields[workitems.FieldCriteria])
	for _, c := range cs {
		text := c.State + "; " + c.Expected
		if hs := cat.Find(text); len(hs) > 0 && !numberRE.MatchString(Unquoted(text)) {
			add(workitems.FieldCriteria, "criterion %d says %s with nothing to observe: resolve it into a number, a count, a named state, or exact quoted wording (%s)", c.N, quoteHits(hs), cat.files(hs))
		}
	}
	if th := it.Fields[workitems.FieldThreshold]; !workitems.Empty(th) {
		if hs := cat.Find(th); len(hs) > 0 && !numberRE.MatchString(Unquoted(th)) {
			add(workitems.FieldThreshold, "Threshold says %s with no number; name the value the measure must reach", quoteHits(hs))
		}
	}

	prov := it.Fields[workitems.FieldProvenance]
	if workitems.Empty(prov) {
		return ps
	}
	p, pps := ParseProvenance(prov)
	ps = append(ps, pps...)
	if p.Request == "" {
		add(workitems.FieldProvenance, "no `Request: \"<the human's words>\"` line")
	}
	have := map[int]bool{}
	for _, c := range cs {
		have[c.N] = true
	}
	count := map[int]int{}
	for _, s := range p.Criteria {
		count[s.N]++
		if !have[s.N] {
			add(workitems.FieldProvenance, "criterion %d has a source but no such criterion exists", s.N)
		}
	}
	for _, c := range cs {
		switch count[c.N] {
		case 0:
			add(workitems.FieldProvenance, "criterion %d has no source: record whether the human stated it, confirmed it, or accepted it as a default, in their words", c.N)
		case 1:
		default:
			add(workitems.FieldProvenance, "criterion %d has %d sources; keep one", c.N, count[c.N])
		}
	}
	deferredQs := false
	for _, l := range lines(it.Fields[workitems.FieldOpenQuestions]) {
		switch {
		case hasFold(l, "deferred:"):
			deferredQs = true
		case hasFold(l, "answered:"):
		default:
			add(workitems.FieldOpenQuestions, "%q is neither answered nor deferred; ask it, or write \"deferred: <question> (<why it can wait>)\"", clip(l, 80))
		}
	}
	seen := map[string]bool{}
	for _, h := range cat.Find(p.Request) {
		if seen[h.Qualifier] {
			continue
		}
		seen[h.Qualifier] = true
		var res *Resolution
		for i, r := range p.Vague {
			if qualifierRE(h.Qualifier).MatchString(r.Term) {
				res = &p.Vague[i]
				break
			}
		}
		switch {
		case res == nil:
			add(workitems.FieldProvenance, "the request says %q but no `Vague:` line resolves it into criteria or defers it (%s)", h.Text, cat.files([]Hit{h}))
		case res.Deferred != "" && !deferredQs:
			add(workitems.FieldProvenance, "%q is deferred, but Open questions has no \"deferred:\" entry for it", res.Term)
		default:
			for _, n := range res.Criteria {
				if !have[n] {
					add(workitems.FieldProvenance, "%q resolves to criterion %d, which does not exist", res.Term, n)
				}
			}
		}
	}
	for _, ab := range cat.Abbreviations(p.Request) {
		t, ok := p.Term(ab.Text)
		switch {
		case !ok:
			add(workitems.FieldProvenance, "the request says %q but no `Term:` line records the meaning the human confirmed (%s)", ab.Text, JargonFile)
		case t.Deferred != "" && !deferredQs:
			add(workitems.FieldProvenance, "term %q is deferred, but Open questions has no \"deferred:\" entry for it", t.Term)
		}
	}
	if workitems.Empty(it.Fields[workitems.FieldAssumptions]) {
		add(workitems.FieldAssumptions, "a grilled spec lists the assumptions it rests on (write \"none\" when there are none)")
	}
	ps = append(ps, checkCheckpoints(it.Fields[workitems.FieldCheckpoints], cs)...)
	for _, c := range cs {
		src, ok := p.Source(c.N)
		if !ok {
			continue
		}
		said := normWords(strings.Join(append(append([]string{}, src.Quotes...), p.Request), " "))
		for _, pl := range Placements(c.Expected) {
			if !strings.Contains(said, normWords(pl.Key)) {
				add(workitems.FieldCriteria, "criterion %d decides where something goes (%q), but the human's words in its Provenance never say so; ask the human, or defer it to Open questions", c.N, pl.Text)
			}
		}
	}
	crit := strings.ToLower(it.Fields[workitems.FieldCriteria])
	for _, l := range lines(it.Fields[workitems.FieldParked]) {
		if label := ideaLabel(l); len(label) >= 3 && strings.Contains(crit, label) {
			add(workitems.FieldParked, "parked idea %q appears in the Completion Goal; a parked idea enters the criteria only when the human confirms it (then remove it from Parked ideas)", label)
		}
	}
	return ps
}

// Placement is a detail of where something appears, named in a
// criterion's expected result: a route, a named screen or page, or a
// position relative to another element. Key is the word the human must
// have said for the skill to write it (WP5.3b).
type Placement struct {
	Text string `json:"text"`
	Key  string `json:"key"`
}

var (
	placeRoute    = regexp.MustCompile("`(/[^`\\s]*)`")
	placeScreen   = regexp.MustCompile(`(?i)\b(?:on|in|onto|into)\s+(?:the\s+|a\s+|its\s+own\s+)?([a-z][a-z-]*)\s+(?:screen|page|step|dialog|modal|panel|tab)\b`)
	placeRelative = regexp.MustCompile(`(?i)\b(above|below|beside|beneath|underneath|next to|to the left of|to the right of|at the top of|at the bottom of)\b`)
)

// Placements returns the placement details of an expected result.
func Placements(expected string) []Placement {
	var out []Placement
	for _, m := range placeRoute.FindAllStringSubmatch(expected, -1) {
		segs := strings.FieldsFunc(m[1], func(r rune) bool { return r == '/' })
		key := ""
		for i := len(segs) - 1; i >= 0 && key == ""; i-- {
			if !strings.HasPrefix(segs[i], "[") && !strings.HasPrefix(segs[i], ":") {
				key = segs[i]
			}
		}
		if key != "" {
			out = append(out, Placement{Text: m[0], Key: key})
		}
	}
	for _, m := range placeScreen.FindAllStringSubmatch(expected, -1) {
		// "on every step" states a scope, not where something goes.
		if !slices.Contains([]string{"every", "each", "any", "all"}, strings.ToLower(m[1])) {
			out = append(out, Placement{Text: m[0], Key: m[1]})
		}
	}
	for _, m := range placeRelative.FindAllStringSubmatch(expected, -1) {
		out = append(out, Placement{Text: m[0], Key: m[1]})
	}
	return out
}

var nonWord = regexp.MustCompile(`[^\p{L}\p{N}]+`)

// normWords lowercases s, keeps only words, and pads it with spaces so a
// key matches whole words.
func normWords(s string) string {
	return " " + strings.TrimSpace(nonWord.ReplaceAllString(strings.ToLower(s), " ")) + " "
}

// ideaLabel is a parked idea's name: the text before its first colon or dash.
func ideaLabel(l string) string {
	for _, sep := range []string{":", " — ", " – ", " - ", " ("} {
		if i := strings.Index(l, sep); i > 0 {
			l = l[:i]
		}
	}
	return strings.ToLower(strings.Trim(strings.TrimSpace(l), "*_\"`"))
}

func lines(s string) []string {
	if workitems.Empty(s) {
		return nil
	}
	var out []string
	for _, l := range strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n") {
		if l = strings.TrimSpace(bullet.ReplaceAllString(l, "")); l != "" {
			out = append(out, l)
		}
	}
	return out
}

func hasFold(s, prefix string) bool { _, ok := cutFold(s, prefix); return ok }

func quoteHits(hs []Hit) string {
	var qs []string
	for _, h := range hs {
		if q := strconv.Quote(h.Text); !slices.Contains(qs, q) {
			qs = append(qs, q)
		}
	}
	return strings.Join(qs, ", ")
}

// files names the catalogues that decompose the hits.
func (c Catalog) files(hs []Hit) string {
	var fs []string
	for _, h := range hs {
		if d, ok := c.Domain(h.Domain); ok && !slices.Contains(fs, "skills/grill/"+d.File) {
			fs = append(fs, "skills/grill/"+d.File)
		}
	}
	return strings.Join(fs, ", ")
}

func clip(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}

// CheckFacts reports every fact a criterion's source cites that the
// assessment of the repository does not have: a criterion grounded in a
// fact that is not there is grounded in nothing.
func CheckFacts(p Provenance, a Assessment) []workitems.Problem {
	ids := map[string]bool{}
	for _, f := range a.Facts {
		ids[f.ID] = true
	}
	var ps []workitems.Problem
	for _, s := range p.Criteria {
		for _, f := range s.Facts {
			if !ids[f] {
				ps = append(ps, workitems.Problem{Field: workitems.FieldProvenance, Message: fmt.Sprintf("criterion %d cites %q, which `aayu grill %s --assess` does not find in this repository", s.N, f, a.Type)})
			}
		}
	}
	return ps
}

// Readback is the plain-language summary of a Story's or Bug's spec that the
// human confirms or edits before it is written: the goal and how it is
// measured, what done looks like (one sentence per criterion), the
// checkpoints the human will see as it is built, the meaning of each term,
// then what it assumes, leaves open, and parks.
func Readback(it workitems.Item) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Here is what done looks like for %q:\n", it.Title)
	if g := strings.TrimSpace(it.Fields[workitems.FieldSummary]); !workitems.Empty(g) {
		fmt.Fprintf(&b, "Goal: %s\n", strings.Join(strings.Fields(g), " "))
	}
	if m, t := strings.TrimSpace(it.Fields[workitems.FieldMeasure]), strings.TrimSpace(it.Fields[workitems.FieldThreshold]); !workitems.Empty(m) && !workitems.Empty(t) {
		fmt.Fprintf(&b, "Measured by %s; met at %s.\n", strings.TrimSuffix(m, "."), strings.TrimSuffix(t, "."))
	}
	cs, _ := workitems.ParseCriteria(it.Fields[workitems.FieldCriteria])
	if len(cs) > 0 {
		b.WriteString("Done when:\n")
	}
	for _, c := range cs {
		fmt.Fprintf(&b, "%d. %s: %s (shown by a %s).\n", c.N, c.State, strings.TrimSuffix(c.Expected, "."), c.Evidence)
	}
	if cps, _ := ParseCheckpoints(it.Fields[workitems.FieldCheckpoints]); len(cps) > 0 {
		b.WriteString("Checkpoints, what you will see as it is built:\n")
		for _, cp := range cps {
			fmt.Fprintf(&b, "%d. %s.\n", cp.N, strings.TrimSuffix(cp.Text, "."))
		}
	}
	if p, _ := ParseProvenance(it.Fields[workitems.FieldProvenance]); len(p.Terms) > 0 {
		var ts []string
		for _, t := range p.Terms {
			if t.Deferred != "" {
				ts = append(ts, fmt.Sprintf("%s is still to be confirmed", t.Term))
			} else {
				ts = append(ts, fmt.Sprintf("%s means %s", t.Term, t.Meaning))
			}
		}
		fmt.Fprintf(&b, "Terms: %s.\n", strings.Join(ts, "; "))
	}
	for _, s := range []struct{ id, label string }{
		{workitems.FieldAssumptions, "It assumes"},
		{workitems.FieldOpenQuestions, "Still open"},
		{workitems.FieldParked, "Parked, not in this item"},
	} {
		if ls := lines(it.Fields[s.id]); len(ls) > 0 {
			for i, l := range ls {
				ls[i] = strings.TrimSuffix(l, ".")
			}
			text := strings.Join(ls, "; ")
			if !strings.ContainsAny(text[len(text)-1:], ".?!") {
				text += "."
			}
			fmt.Fprintf(&b, "%s: %s\n", s.label, text)
		}
	}
	return strings.TrimSpace(b.String())
}
