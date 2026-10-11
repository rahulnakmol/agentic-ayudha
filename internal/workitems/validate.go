package workitems

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Item is a work item as validation sees it: its type, body fields by field ID,
// labels, and parent. The WP2.7 parser builds it from an issue.
type Item struct {
	Type   string            `json:"type"`
	Number int               `json:"number,omitempty"`
	Title  string            `json:"title"`
	Fields map[string]string `json:"fields"`
	Labels []string          `json:"labels,omitempty"`
	Parent *Ref              `json:"parent,omitempty"`
	// LinkTypes holds the type of each linked issue, by number, when the caller
	// has resolved them; link targets missing here are not type-checked.
	LinkTypes map[int]string `json:"link_types,omitempty"`
}

// Ref is a reference to another issue and its work-item type.
type Ref struct {
	Number int    `json:"number"`
	Type   string `json:"type"`
}

// Problem is one validation failure.
type Problem struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (p Problem) String() string { return p.Field + ": " + p.Message }

// Empty reports whether a field value is absent. GitHub renders an optional form
// field left blank as "_No response_".
func Empty(v string) bool {
	v = strings.TrimSpace(v)
	return v == "" || v == "_No response_"
}

// Label returns the value of the label with prefix ("effort" → "M"), or "".
func (it Item) Label(prefix string) string {
	for _, l := range it.Labels {
		if v, ok := strings.CutPrefix(l, prefix+":"); ok {
			return v
		}
	}
	return ""
}

// Validate checks it against its type's requirements at stage. It returns every
// problem found, in a stable order; no problems means the stage passes.
func Validate(it Item, stage Stage) []Problem {
	t, ok := FindType(it.Type)
	if !ok {
		return []Problem{{"type", fmt.Sprintf("unknown work-item type %q; want one of %s", it.Type, strings.Join(TypeIDs(), ", "))}}
	}
	if stage.index() < 0 {
		return []Problem{{"stage", fmt.Sprintf("unknown stage %q", stage)}}
	}
	var ps []Problem
	add := func(field, format string, args ...any) { ps = append(ps, Problem{field, fmt.Sprintf(format, args...)}) }

	if strings.TrimSpace(it.Title) == "" {
		add("title", "required")
	}
	for _, f := range t.Fields() {
		v := it.Fields[f.ID]
		if Empty(v) {
			if f.RequiredAt != "" && f.RequiredAt.index() <= stage.index() {
				add(f.ID, "%s is required at %s", f.Label, stageName(t, f.RequiredAt))
			}
			continue
		}
		switch f.Kind {
		case KindEnum:
			if !slices.Contains(f.Options, strings.TrimSpace(v)) {
				add(f.ID, "%s = %q: want one of %s", f.Label, strings.TrimSpace(v), strings.Join(f.Options, ", "))
			}
		case KindCriteria:
			_, cps := ParseCriteria(v)
			ps = append(ps, cps...)
		case KindLinks:
			ps = append(ps, checkLinks(t, it, v)...)
		}
	}
	for id := range it.Fields {
		if _, ok := t.Field(id); !ok {
			add(id, "not a field of %s", t.Name)
		}
	}
	slices.SortStableFunc(ps, func(a, b Problem) int { return fieldOrder(t, a.Field) - fieldOrder(t, b.Field) })

	// Parent.
	switch {
	case it.Parent == nil:
		if t.ParentAt != "" && t.ParentAt.index() <= stage.index() {
			add("parent", "a %s needs a parent %s at %s", t.Name, strings.Join(t.Parents, " or "), stageName(t, t.ParentAt))
		}
	case len(t.Parents) == 0:
		add("parent", "a %s has no parent; #%d is set as one", t.Name, it.Parent.Number)
	case !slices.Contains(t.Parents, it.Parent.Type):
		add("parent", "#%d is a %s; a %s's parent must be a %s", it.Parent.Number, it.Parent.Type, t.Name, strings.Join(t.Parents, " or "))
	}

	// Labels: one value per label-held field, from its enum.
	for _, lf := range LabelFields {
		var vals []string
		for _, l := range it.Labels {
			if v, ok := strings.CutPrefix(l, lf.Prefix+":"); ok {
				vals = append(vals, v)
			}
		}
		if len(vals) > 1 {
			add(lf.Prefix, "%s has %d labels (%s); keep one", lf.Name, len(vals), strings.Join(vals, ", "))
		}
		for _, v := range vals {
			if !slices.Contains(lf.Values, v) {
				add(lf.Prefix, "%s:%s is not a %s value; want one of %s", lf.Prefix, v, lf.Name, strings.Join(lf.Values, ", "))
			}
		}
	}
	for _, l := range it.Labels {
		if typ, ok := strings.CutPrefix(l, "type:"); ok && typ != t.ID {
			add("type", "label type:%s disagrees with the %s type", typ, t.Name)
		}
	}
	if stage.index() >= StageReady.index() {
		switch effort := it.Label("effort"); {
		case effort == "XL":
			add("effort", "effort:XL blocks Ready; slice it into XS-L stories")
		case effort == "" && t.Effort:
			add("effort", "Effort is required at Ready: add one of the labels effort:XS, effort:S, effort:M, effort:L")
		}
	}
	return ps
}

// stageName names a stage for messages; a Decision's Ready is "Decision-ready".
func stageName(t Type, s Stage) string {
	if s == StageReady && t.ID == "decision" {
		return "Decision-ready"
	}
	return strings.ToUpper(string(s[:1])) + string(s[1:])
}

func fieldOrder(t Type, id string) int {
	for i, f := range t.Fields() {
		if f.ID == id {
			return i
		}
	}
	return len(t.Fields())
}

// Criterion is one numbered observable criterion of a Completion Goal.
type Criterion struct {
	N        int    `json:"n"`
	Evidence string `json:"evidence_kind"`
	State    string `json:"state"`
	Expected string `json:"expected"`
}

var (
	criterionLine = regexp.MustCompile(`^\s*([0-9]+)[.)]\s+(.*)$`)
	evidenceTag   = regexp.MustCompile(`^\[([A-Za-z-]+)\]\s*`)
	stateExpected = regexp.MustCompile(`(?i)^state:\s*(.*?)\s*;\s*expected:\s*(.*?)\s*$`)
)

// ParseCriteria parses Completion Goal criteria, one per line:
//
//	N. [evidence_kind] state: <state>; expected: <expected>
//
// It returns the criteria and a problem for each line that is not a complete,
// mapped criterion. A criterion is unmapped when it names no evidence kind, or
// one outside EvidenceKinds, because no proof could ever be attached to it.
func ParseCriteria(s string) ([]Criterion, []Problem) {
	var cs []Criterion
	var ps []Problem
	bad := func(format string, args ...any) {
		ps = append(ps, Problem{FieldCriteria, fmt.Sprintf(format, args...)})
	}
	for i, line := range strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		m := criterionLine.FindStringSubmatch(line)
		if m == nil {
			bad("line %d is not a numbered criterion (want \"N. [evidence] state: ...; expected: ...\")", i+1)
			continue
		}
		n, _ := strconv.Atoi(m[1])
		c := Criterion{N: n}
		rest := m[2]
		if want := len(cs) + 1; n != want {
			bad("criterion %d is numbered out of order; want %d", n, want)
		}
		if t := evidenceTag.FindStringSubmatch(rest); t != nil {
			c.Evidence = strings.ToLower(t[1])
			rest = rest[len(t[0]):]
		}
		if !slices.Contains(EvidenceKinds, c.Evidence) {
			got := "no evidence kind"
			if c.Evidence != "" {
				got = "evidence kind " + strconv.Quote(c.Evidence)
			}
			bad("criterion %d is unmapped (%s); start it with one of [%s]", n, got, strings.Join(EvidenceKinds, "], ["))
		}
		if se := stateExpected.FindStringSubmatch(rest); se != nil && se[1] != "" && se[2] != "" {
			c.State, c.Expected = se[1], se[2]
		} else {
			bad("criterion %d needs \"state: <state>; expected: <what is observed>\"", n)
		}
		cs = append(cs, c)
	}
	if len(cs) == 0 && len(ps) == 0 {
		bad("no criteria")
	}
	return cs, ps
}

// Link is one typed link from the Links section.
type Link struct {
	Kind   string `json:"kind"`
	Number int    `json:"number"`
}

var linkLine = regexp.MustCompile(`^\s*[-*]?\s*([a-z-]+)\s+#([1-9][0-9]*)\s*$`)

// ParseLinks parses "kind #N" lines (an optional list bullet is allowed).
func ParseLinks(s string) ([]Link, []Problem) {
	var ls []Link
	var ps []Problem
	for i, line := range strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		m := linkLine.FindStringSubmatch(line)
		if m == nil {
			ps = append(ps, Problem{FieldLinks, fmt.Sprintf("line %d is not a link (want \"<kind> #N\")", i+1)})
			continue
		}
		n, _ := strconv.Atoi(m[2])
		ls = append(ls, Link{m[1], n})
	}
	return ls, ps
}

func checkLinks(t Type, it Item, v string) []Problem {
	ls, ps := ParseLinks(v)
	for _, l := range ls {
		k, ok := FindLink(l.Kind)
		switch {
		case !ok || !slices.Contains(t.Links, l.Kind):
			ps = append(ps, Problem{FieldLinks, fmt.Sprintf("a %s cannot carry a %q link; allowed: %s", t.Name, l.Kind, strings.Join(t.Links, ", "))})
		case it.LinkTypes[l.Number] != "" && !slices.Contains(k.To, it.LinkTypes[l.Number]):
			ps = append(ps, Problem{FieldLinks, fmt.Sprintf("%s #%d: #%d is a %s; %s links to a %s", l.Kind, l.Number, l.Number, it.LinkTypes[l.Number], l.Kind, strings.Join(k.To, " or "))})
		}
	}
	return ps
}
