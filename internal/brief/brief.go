// Package brief implements `aayu brief` (WP7.3, WP7.4): a host-written
// Markdown draft, grounded in a work item's evidence, is checked
// deterministically (structure, citations, the register's lint, visuals), then
// rendered as one self-contained, accessible HTML page with a grounding
// footer, inline SVG diagrams and charts, labelled illustrations, and
// annotated screens from proof packs.
//
// Register lint: eli5 needs a Flesch-Kincaid grade of at most 6; ste needs
// the STE lint to pass (sentences of at most 20 words, the approved-word list,
// one instruction per sentence); regulatory needs a citation on every
// paragraph and list item. Deterministic checks come first and a model never
// overrides them (ground rule 7).
package brief

import (
	"bytes"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"
)

// Registers and purposes (the registry's enums for `brief`).
var (
	Registers = []string{"eli5", "ste", "engineer", "regulatory"}
	Purposes  = []string{"explain", "ideate", "innovate", "close"}
)

// MaxGrade is the eli5 bar: Flesch-Kincaid grade at most 6.
const MaxGrade = 6.0

// Grounding is everything a brief may say, each source with the ID the
// draft cites: `#n` for issues, `P1`… for proof packs, `R` for the
// subject's Result, `D1`… for measure snapshots. Issue text is untrusted
// input (§0.1).
type Grounding struct {
	Repo      string     `json:"repo"`
	Subject   Issue      `json:"subject"`
	Items     []Issue    `json:"items,omitempty"` // work items under the subject, at any depth
	Packs     []Pack     `json:"packs,omitempty"`
	Result    *Result    `json:"result,omitempty"`
	Snapshots []Snapshot `json:"snapshots,omitempty"`
	Ledger    string     `json:"ledger,omitempty"` // ledger commit the packs were read at
	Notes     []string   `json:"notes,omitempty"`  // what could not be read, and why
}

// Issue is a work item as a source.
type Issue struct {
	Number     int               `json:"number"`
	Type       string            `json:"type"`
	Title      string            `json:"title"`
	State      string            `json:"state"`
	Status     string            `json:"status,omitempty"`
	URL        string            `json:"url,omitempty"`
	UpdatedAt  string            `json:"updated_at,omitempty"`
	BodySHA256 string            `json:"body_sha256,omitempty"`
	Fields     map[string]string `json:"fields,omitempty"` // parsed body sections, untrusted
}

// Pack is a proof pack as a source.
type Pack struct {
	ID         string     `json:"id"`
	Story      int        `json:"story"`
	Authority  string     `json:"authority"` // advisory or authoritative
	RunID      int64      `json:"run_id,omitempty"`
	HeadSHA    string     `json:"pr_head_sha"`
	PR         int        `json:"pr,omitempty"`
	LedgerPath string     `json:"ledger_path"`
	SHA256     string     `json:"sha256"` // of pack.json as stored
	FloorsHold bool       `json:"floors_hold"`
	Checks     []Floor    `json:"checks,omitempty"`
	Artifacts  []Artifact `json:"artifacts,omitempty"`
	CreatedAt  string     `json:"created_at,omitempty"`
	// Criteria is the pack's criterion -> evidence map: every criterion of the
	// story's Ready snapshot, with the artifacts the pack maps to it (none when
	// it is unmapped). A draft cites one entry as [P1.c3].
	Criteria []PackCriterion `json:"criteria,omitempty"`
	Inspect  *Inspection     `json:"inspect,omitempty"` // the inspect result for this pack's head, when one exists
}

// PackCriterion is one entry of a pack's criterion -> evidence map.
type PackCriterion struct {
	ID       int        `json:"id"`
	Kind     string     `json:"evidence_kind,omitempty"` // the evidence kind the criterion asks for
	Text     string     `json:"text,omitempty"`          // "state: …; expected: …" from the Ready snapshot
	Evidence []Artifact `json:"evidence"`                // empty: the pack has no evidence for it
	Verdict  string     `json:"inspect_verdict,omitempty"`
	Judgment string     `json:"inspect_judgement,omitempty"`
}

// Cite is the citation of a map entry: P1.c3.
func (c PackCriterion) Cite(pack string) string { return fmt.Sprintf("%s.c%d", pack, c.ID) }

// Inspection is the inspect (WP4.8) disposition on a pack's head.
type Inspection struct {
	Disposition string `json:"disposition"`
	Summary     string `json:"summary,omitempty"`
	Model       string `json:"model,omitempty"`
	Head        string `json:"head"`
	By          string `json:"by,omitempty"` // who posted the inspect comment
}

// Floor is one floor result in a pack.
type Floor struct {
	Floor     string  `json:"floor"`
	Result    string  `json:"result"`
	Value     float64 `json:"value"`
	Threshold float64 `json:"threshold"`
	Detail    string  `json:"detail,omitempty"`
}

// Artifact is one evidence file of a pack.
type Artifact struct {
	Path   string `json:"path"`
	Kind   string `json:"kind"`
	SHA256 string `json:"sha256"`
}

// Result is the subject's Result (Goal, Initiative, Feature, Experiment) or
// Outcome (Story, Bug), as written by measure, readout, or ship.
type Result struct {
	Issue  int    `json:"issue"`
	Field  string `json:"field"` // result or outcome
	Text   string `json:"text"`
	SHA256 string `json:"sha256"`
}

// Snapshot is a measure SQLite snapshot (WP7.2) a chart may query.
type Snapshot struct {
	ID     string `json:"id"`
	File   string `json:"file"`
	SHA256 string `json:"sha256"`
	Path   string `json:"-"` // local path; empty when the file is not on this machine
}

// IDs lists every source ID the grounding offers.
func (g *Grounding) IDs() []string {
	ids := []string{fmt.Sprintf("#%d", g.Subject.Number)}
	for _, it := range g.Items {
		ids = append(ids, fmt.Sprintf("#%d", it.Number))
	}
	for _, p := range g.Packs {
		ids = append(ids, p.ID)
		for _, c := range p.Criteria {
			if len(c.Evidence) > 0 { // an unmapped criterion has nothing to cite
				ids = append(ids, c.Cite(p.ID))
			}
		}
	}
	if g.Result != nil {
		ids = append(ids, "R")
	}
	for _, s := range g.Snapshots {
		ids = append(ids, s.ID)
	}
	return ids
}

// AuthoritativePacks returns the packs written by the merge gate.
func (g *Grounding) AuthoritativePacks() []Pack {
	var out []Pack
	for _, p := range g.Packs {
		if p.Authority == "authoritative" {
			out = append(out, p)
		}
	}
	return out
}

func (g *Grounding) pack(id string) (Pack, bool) {
	for _, p := range g.Packs {
		if p.ID == id {
			return p, true
		}
	}
	return Pack{}, false
}

func (g *Grounding) snapshot(ref string) (Snapshot, bool) {
	for _, s := range g.Snapshots {
		if s.ID == ref || s.File == ref {
			return s, true
		}
	}
	return Snapshot{}, false
}

// Draft is a parsed brief draft.
type Draft struct {
	Source   []byte
	Dir      string // directory of the draft file; visual sources are relative to it
	Title    string
	Takeaway string
	Units    []Unit    // prose: paragraphs and list items
	Sections []string  // level-2 headings, in order
	Visuals  []*Visual // fenced visual blocks, in order
	Images   int       // Markdown image embeds (not allowed: visuals are blocks)
	RawHTML  int       // raw HTML in the draft (never rendered)
	doc      ast.Node
}

// Unit is one paragraph or list item of prose.
type Unit struct {
	Text    string   // plain text, citations removed, code spans as one token
	Cites   []string // source IDs cited
	Section string   // the level-2 heading above it
	Item    bool     // a list item
	First   bool     // the first paragraph after the title (the takeaway)
	Code    int      // code spans in it
}

// Visual kinds: fenced blocks whose info string names one.
var VisualKinds = []string{"diagram", "chart", "illustration", "screen"}

// Visual is one fenced visual block: header lines `key: value`, then a body.
type Visual struct {
	N     int // 1-based figure number
	Kind  string
	Attrs map[string]string
	Body  string
	node  *ast.FencedCodeBlock
}

// visualKeys are the header keys each visual kind takes.
var visualKeys = map[string][]string{
	"diagram":      {"alt", "title", "src", "caption"},
	"chart":        {"alt", "title", "snapshot", "x", "y", "kind", "caption"},
	"illustration": {"alt", "src", "generator", "prompt", "caption"},
	"screen":       {"alt", "pack", "artifact", "caption"},
}

// citeRE matches a citation: one or more source IDs in brackets.
var citeRE = regexp.MustCompile(`\[((?:#\d+|P\d+(?:\.c\d+)?|R|D\d+)(?:\s*,\s*(?:#\d+|P\d+(?:\.c\d+)?|R|D\d+))*)\]`)

func newMarkdown() goldmark.Markdown {
	return goldmark.New(goldmark.WithExtensions(extension.Table, extension.Strikethrough))
}

// Parse reads a Markdown draft. dir is where visual `src` paths resolve.
func Parse(src []byte, dir string) (*Draft, error) {
	src = bytes.ReplaceAll(src, []byte("\r\n"), []byte("\n"))
	d := &Draft{Source: src, Dir: dir}
	d.doc = newMarkdown().Parser().Parse(text.NewReader(src))
	section := ""
	afterTitle := false
	err := ast.Walk(d.doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n := n.(type) {
		case *ast.Heading:
			t := inlineText(n, src, nil)
			switch {
			case n.Level == 1 && d.Title == "":
				d.Title, afterTitle = t, true
				return ast.WalkSkipChildren, nil
			case n.Level == 2:
				section = t
				d.Sections = append(d.Sections, t)
			}
			afterTitle = false
			return ast.WalkSkipChildren, nil
		case *ast.Paragraph, *ast.TextBlock:
			u := unit(n, src)
			u.Section = section
			_, u.Item = n.Parent().(*ast.ListItem)
			if afterTitle && !u.Item && d.Takeaway == "" {
				u.First, d.Takeaway = true, u.Text
			}
			afterTitle = false
			d.Units = append(d.Units, u)
			return ast.WalkSkipChildren, nil
		case *ast.FencedCodeBlock:
			afterTitle = false
			lang := string(n.Language(src))
			if !slices.Contains(VisualKinds, lang) {
				return ast.WalkSkipChildren, nil
			}
			v := &Visual{N: len(d.Visuals) + 1, Kind: lang, Attrs: map[string]string{}, node: n}
			var lines []string
			for i := 0; i < n.Lines().Len(); i++ {
				s := n.Lines().At(i)
				lines = append(lines, strings.TrimRight(string(s.Value(src)), "\n"))
			}
			body := 0
			for body < len(lines) {
				k, val, ok := strings.Cut(lines[body], ":")
				k = strings.TrimSpace(k)
				if !ok || !slices.Contains(visualKeys[lang], k) {
					break
				}
				v.Attrs[k] = strings.TrimSpace(val)
				body++
			}
			v.Body = strings.TrimSpace(strings.Join(lines[body:], "\n"))
			d.Visuals = append(d.Visuals, v)
			return ast.WalkSkipChildren, nil
		case *ast.CodeBlock, *ast.HTMLBlock:
			if _, ok := n.(*ast.HTMLBlock); ok {
				d.RawHTML++
			}
			afterTitle = false
			return ast.WalkSkipChildren, nil
		case *ast.ThematicBreak, *ast.Blockquote, *ast.List:
			afterTitle = false // the takeaway is a paragraph right after the title
		}
		return ast.WalkContinue, nil
	})
	if err != nil {
		return nil, err
	}
	// Images and raw inline HTML anywhere in prose.
	_ = ast.Walk(d.doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			switch n.(type) {
			case *ast.Image:
				d.Images++
			case *ast.RawHTML:
				d.RawHTML++
			}
		}
		return ast.WalkContinue, nil
	})
	return d, nil
}

// unit turns a paragraph or text block into prose.
func unit(n ast.Node, src []byte) Unit {
	var u Unit
	t := inlineText(n, src, func(code string) string {
		u.Code++
		return codeToken
	})
	for _, m := range citeRE.FindAllStringSubmatch(t, -1) {
		for _, id := range strings.Split(m[1], ",") {
			u.Cites = append(u.Cites, strings.TrimSpace(id))
		}
	}
	t = citeRE.ReplaceAllString(t, "")
	u.Text = strings.Join(strings.Fields(t), " ")
	u.Text = strings.ReplaceAll(u.Text, " .", ".")
	u.Text = strings.ReplaceAll(u.Text, " ,", ",")
	return u
}

// codeToken stands for a code span in prose: one word, a technical name.
const codeToken = "\uE000code\uE001"

// inlineText concatenates the text under n; code spans go through code
// when it is set, else they keep their text.
func inlineText(n ast.Node, src []byte, code func(string) string) string {
	var b strings.Builder
	_ = ast.Walk(n, func(c ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch c := c.(type) {
		case *ast.CodeSpan:
			var cs strings.Builder
			for x := c.FirstChild(); x != nil; x = x.NextSibling() {
				if t, ok := x.(*ast.Text); ok {
					cs.Write(t.Value(src))
				}
			}
			if code != nil {
				b.WriteString(code(cs.String()))
			} else {
				b.WriteString(cs.String())
			}
			return ast.WalkSkipChildren, nil
		case *ast.Text:
			b.Write(c.Value(src))
			if c.SoftLineBreak() || c.HardLineBreak() {
				b.WriteByte(' ')
			}
		case *ast.String:
			b.Write(c.Value)
		case *ast.AutoLink:
			b.Write(c.URL(src))
			return ast.WalkSkipChildren, nil
		}
		return ast.WalkContinue, nil
	})
	return b.String()
}

// Finding is one failed deterministic check.
type Finding struct {
	Check   string `json:"check"` // structure, citations, readability, ste, citations-regulatory, visual, purpose
	Message string `json:"message"`
}

func (f Finding) String() string { return f.Check + ": " + f.Message }

// Options configure Check.
type Options struct {
	Register string
	Purpose  string
	Words    *Words // the STE approved-word list; required for the ste register
}

// Check runs the deterministic checks on a draft. A brief with findings is
// not rendered.
func Check(d *Draft, g *Grounding, o Options) []Finding {
	var out []Finding
	add := func(check, format string, args ...any) {
		out = append(out, Finding{check, fmt.Sprintf(format, args...)})
	}
	if d.Title == "" {
		add("structure", "the draft needs a title: one level-1 heading (`# …`) first")
	}
	if d.Takeaway == "" {
		add("structure", "the first block after the title must be a paragraph that states the takeaway")
	}
	if d.Images > 0 {
		add("structure", "%d Markdown image(s): put images in a fenced `illustration` or `screen` block with alt text", d.Images)
	}
	if d.RawHTML > 0 {
		add("structure", "%d raw HTML fragment(s): the draft is Markdown only; raw HTML is never rendered", d.RawHTML)
	}

	ids := g.IDs()
	cited := map[string]bool{}
	for _, u := range d.Units {
		for _, c := range u.Cites {
			cited[c] = true
			if !slices.Contains(ids, c) {
				add("citations", "[%s] is not a source of this brief; cite one of %s", c, strings.Join(ids, ", "))
			}
		}
	}
	if len(cited) == 0 {
		add("citations", "the brief cites no source; cite the evidence for each claim as [P1], [R], [#n], or [D1]")
	}
	for _, p := range proofClaims(d, g) {
		add("evidence", "%s", p)
	}
	if o.Purpose == "close" {
		if g.Result != nil && !cited["R"] {
			add("citations", "a close brief cites the Result [R]")
		}
		citesAuth := false
		for _, p := range g.AuthoritativePacks() {
			citesAuth = citesAuth || cited[p.ID]
		}
		if !citesAuth {
			add("citations", "a close brief cites at least one authoritative pack")
		}
	}

	switch o.Purpose {
	case "ideate":
		if n := itemsIn(d, "Ideas"); n < 2 {
			add("purpose", "an ideate brief has an `## Ideas` section listing at least two ideas (found %d)", n)
		}
	case "innovate":
		if !slices.Contains(d.Sections, "Hypothesis") {
			add("purpose", "an innovate brief has a `## Hypothesis` section: the bet, the metric it moves, and how to measure it")
		}
	}

	if o.Purpose == "close" || o.Register == "regulatory" {
		if n := unitsIn(d, LimitsSection); n == 0 {
			add("limits", "a %s brief has a `## %s` section that states what the evidence and the measures do not show (for example, that usage does not show time saved)", map[bool]string{true: "close", false: "regulatory"}[o.Purpose == "close"], LimitsSection)
		}
	}
	population, unit := rateWording(d, g)
	for _, p := range population {
		add("population", "%s", p)
	}
	for _, p := range unit {
		add("unit", "%s", p)
	}
	for _, p := range uiDetails(d, g) {
		add("ui", "%s", p)
	}
	for _, p := range absenceClaims(d, g) {
		add("absence", "%s", p)
	}

	switch o.Register {
	case "eli5":
		r := Readability(d.Units)
		if r.Grade > MaxGrade {
			add("readability", "Flesch-Kincaid grade %.1f is above %.0f (%d words, %d sentences, %d syllables); use shorter sentences and shorter words%s",
				r.Grade, MaxGrade, r.Words, r.Sentences, r.Syllables, hardest(r))
		}
	case "ste":
		if o.Words == nil {
			add("ste", "no approved-word list loaded")
			break
		}
		for _, p := range LintSTE(d.Units, o.Words, Names(g)) {
			add("ste", "%s", p)
		}
	case "regulatory":
		for i, u := range d.Units {
			if len(u.Cites) == 0 {
				add("citations", "regulatory: %s %d has no citation: %q", kindOf(u), i+1, clip(u.Text, 80))
			}
		}
	}

	for _, v := range d.Visuals {
		for _, p := range checkVisual(v, d, g) {
			add("visual", "figure %d (%s): %s", v.N, v.Kind, p)
		}
	}
	return out
}

func kindOf(u Unit) string {
	if u.Item {
		return "list item"
	}
	return "paragraph"
}

// LimitsSection is the section a close or regulatory brief must have.
const LimitsSection = "Limits of this evidence"

func unitsIn(d *Draft, section string) int {
	n := 0
	for _, u := range d.Units {
		if strings.EqualFold(u.Section, section) && strings.TrimSpace(u.Text) != "" {
			n++
		}
	}
	return n
}

func itemsIn(d *Draft, section string) int {
	n := 0
	for _, u := range d.Units {
		if u.Section == section && u.Item {
			n++
		}
	}
	return n
}

var altPrefix = regexp.MustCompile(`(?i)^(an? |the )?(image|picture|photo|diagram|chart|graph|screenshot|screen|illustration|figure|drawing)\b`)

// CheckAlt checks that alt text states the takeaway: a sentence of at least
// four words ending in a full stop, not a description of the medium.
func CheckAlt(alt string) string {
	alt = strings.TrimSpace(alt)
	switch {
	case alt == "":
		return "alt text is required: one sentence that states the takeaway"
	case len(strings.Fields(alt)) < 4 || !strings.HasSuffix(alt, "."):
		return fmt.Sprintf("alt %q must state the takeaway as a sentence of at least four words ending in a full stop", alt)
	case altPrefix.MatchString(alt):
		return fmt.Sprintf("alt %q describes the medium; state what the reader should learn from it", alt)
	}
	return ""
}

func clip(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}
