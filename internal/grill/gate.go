package grill

import (
	"fmt"
	"regexp"
	"strings"
)

// Gate verdicts: a request in words is unpacked (grilled) before any skill
// acts on it, or passes straight through because it is already concrete.
const (
	GateUnpack = "unpack"
	GatePass   = "pass"
)

// Gate reason kinds.
const (
	ReasonVague   = "vague"   // a qualifier with a catalogue ("slick", "fast", "maybe")
	ReasonTerm    = "term"    // an abbreviation or jargon term whose meaning must be confirmed
	ReasonOutcome = "outcome" // nothing observable: no number, exact wording, route, command, or flag
)

// Gate is the vagueness gate (WP5.8): the deterministic verdict on a request
// in words that every entry point taking free text (build, slice, plan, the
// delegate commands, the MCP prompts) reads from `aayu grill <type> --assess
// --idea "<words>" --json` before acting. The host's model then unpacks the
// request with the human, or drafts it as stated; the binary only decides
// which.
type Gate struct {
	Verdict string       `json:"verdict"`
	Reasons []GateReason `json:"reasons"`
	Next    string       `json:"next"`
}

// GateReason is one thing that keeps a request from being concrete.
type GateReason struct {
	Kind       string      `json:"kind"`
	Text       string      `json:"text"` // the request's own words, or what is missing
	Why        string      `json:"why"`
	Meanings   []string    `json:"meanings,omitempty"`   // a term's usual meanings
	Catalogue  string      `json:"catalogue,omitempty"`  // where its dimensions are
	Dimensions []Dimension `json:"dimensions,omitempty"` // an unstated outcome: the rough-idea pass
}

// The gate's next steps, as the skills quote them.
const (
	gateNextUnpack = "Stop and unpack before acting: name these words to the human, ask the riskiest questions first, read back what done looks like, and continue only after the human confirms (skills/grill/SKILL.md, The gate)."
	gateNextPass   = "Concrete: draft the work item from the request's own words without questions, quote the request in Provenance (`stated (request)`), and continue."
)

var (
	issueRef   = regexp.MustCompile(`#[0-9]+\b`)
	anchorDig  = regexp.MustCompile(`[0-9]`)
	anchorLit  = regexp.MustCompile("\"[^\"\n]+\"|“[^”\n]+”|`[^`\n]+`")
	anchorPath = regexp.MustCompile(`(^|[\s(])/[A-Za-z0-9_\[\]:-]+|(^|[\s(])--?[a-z][a-z0-9-]+\b`)
)

// Observable reports whether a request names something to observe: a
// number (an issue reference or a term such as B2B is not one), exact wording or a literal in
// quotes or backticks, a route or path, or a command-line flag. Without one,
// any criterion written from the request alone would be the skill's guess.
func Observable(text string, terms []Abbreviation) bool {
	if anchorLit.MatchString(text) {
		return true
	}
	s := issueRef.ReplaceAllString(text, " ")
	for _, t := range terms { // B2B and a11y are terms, not numbers
		s = strings.ReplaceAll(s, t.Text, " ")
	}
	return anchorDig.MatchString(s) || anchorPath.MatchString(s)
}

// GateFor decides the gate for an assessment's request: unpack when it has a
// vague qualifier, a term whose meaning is not plain, or (with no qualifier
// to decompose) nothing observable; pass otherwise.
func GateFor(cat Catalog, a Assessment) Gate {
	g := Gate{Verdict: GateUnpack, Reasons: []GateReason{}}
	for _, v := range a.Vague {
		g.Reasons = append(g.Reasons, GateReason{Kind: ReasonVague, Text: v.Text, Catalogue: v.Catalogue,
			Why: fmt.Sprintf("%q means nothing to observe until it is broken into the %s dimensions", v.Text, v.Domain)})
	}
	for _, t := range a.Terms {
		why := fmt.Sprintf("%q is an abbreviation or term the human must confirm", t.Text)
		switch {
		case len(t.Meanings) == 0:
			why = fmt.Sprintf("%q is not in %s: ask what it means", t.Text, JargonFile)
		case len(t.Meanings) > 1:
			why = fmt.Sprintf("%q has %d usual meanings: ask which one", t.Text, len(t.Meanings))
		}
		g.Reasons = append(g.Reasons, GateReason{Kind: ReasonTerm, Text: t.Text, Meanings: t.Meanings, Catalogue: "skills/grill/references/jargon.md", Why: why})
	}
	// A vague qualifier's dimensions already lead to something observable;
	// without one, a request that names nothing to observe is a rough idea.
	if len(a.Vague) == 0 && !Observable(a.Idea, a.Terms) {
		r := GateReason{Kind: ReasonOutcome, Text: "no observable result",
			Why: "the request names no number, exact wording, route, command, or flag, so done is not yet defined: who it is for, the outcome, and the smallest provable slice"}
		if d, ok := cat.Domain("idea"); ok {
			r.Catalogue, r.Dimensions = "skills/grill/"+d.File, ForProducts(d.Dimensions, a.Products)
		}
		g.Reasons = append(g.Reasons, r)
	}
	if len(g.Reasons) == 0 {
		g.Verdict = GatePass
		g.Next = gateNextPass
		return g
	}
	g.Next = gateNextUnpack
	return g
}

// Summary is the gate in one line, for text reports and prompts.
func (g Gate) Summary() string {
	if g.Verdict == GatePass {
		return "Gate: pass. " + g.Next
	}
	var ws []string
	for _, r := range g.Reasons {
		switch r.Kind {
		case ReasonOutcome:
			ws = append(ws, "no observable result")
		default:
			ws = append(ws, fmt.Sprintf("%s %q", r.Kind, r.Text))
		}
	}
	return "Gate: unpack (" + strings.Join(ws, ", ") + "). " + g.Next
}
