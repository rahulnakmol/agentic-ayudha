package ready

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Drift after a baseline (Story #993): what changed is computed from the pins
// when it is read, and nothing writes a drift record. A new version says, for
// each change, what changed, why it matters, and the need behind it.

// Drift is one pin that differs from a baseline's: added, removed, or changed.
type Drift struct {
	Ref    string   `json:"ref"`
	Kind   string   `json:"kind"`             // added, removed, or changed
	Fields []string `json:"fields,omitempty"` // of a changed story pin: which of its pinned fields changed
}

// What says what happened to the pin: "added", or "Completion Goal changed".
func (d Drift) What() string {
	if len(d.Fields) > 0 {
		return strings.Join(d.Fields, ", ") + " changed"
	}
	return d.Kind
}

func (d Drift) String() string { return d.Ref + " " + d.What() }

// Diff is how the pins now differ from a baseline's pins, one entry per pin:
// the changed and added pins in the order of now, then the removed ones. The
// spec's pin of the set of stories is left out when a story was added or
// removed, because that story's own entry says it.
func Diff(was, now []Pin) []Drift {
	old, cur := map[string]Pin{}, map[string]bool{}
	for _, p := range was {
		old[p.Ref] = p
	}
	var out []Drift
	set, moved := -1, false
	for _, p := range now {
		cur[p.Ref] = true
		o, ok := old[p.Ref]
		switch {
		case !ok:
			out, moved = append(out, Drift{Ref: p.Ref, Kind: "added"}), moved || p.Story != nil
		case o.Hash != p.Hash:
			if p.Story == nil && strings.HasSuffix(p.Ref, " stories") {
				set = len(out)
			}
			out = append(out, Drift{Ref: p.Ref, Kind: "changed", Fields: storyFields(o.Story, p.Story)})
		}
	}
	for _, p := range was {
		if !cur[p.Ref] {
			out, moved = append(out, Drift{Ref: p.Ref, Kind: "removed"}), moved || p.Story != nil
		}
	}
	if moved && set >= 0 {
		out = slices.Delete(out, set, set+1)
	}
	return out
}

// storyFields names the pinned fields of a story that differ; none when a
// pin carries no story fields.
func storyFields(a, b *StoryPin) []string {
	if a == nil || b == nil {
		return nil
	}
	var out []string
	for _, f := range []struct {
		name string
		same bool
	}{
		{"title", a.Title == b.Title},
		{"Completion Goal", a.CompletionGoalHash == b.CompletionGoalHash},
		{"effort", a.Effort == b.Effort},
		{"layers", slices.Equal(a.Layers, b.Layers)},
		{"depends-on", slices.Equal(a.DependsOn, b.DependsOn)},
	} {
		if !f.same {
			out = append(out, f.name)
		}
	}
	return out
}

// Change is why one drift is in a new version, as the drafting agent says it.
// The text is untrusted (threat T6): the marker comment escapes it.
type Change struct {
	Ref  string `json:"ref"`
	What string `json:"what"`
	Why  string `json:"why"`
	Need string `json:"need"`
}

// Reasons checks that changes give what, why, and need, each of at least
// minWords words, for each drift, and returns them in the order of drift.
// missing names each drift with an empty field, short each field with too
// few words, and unknown each entry that is not a drift or names one twice.
func Reasons(drift []Drift, changes []Change, minWords int) (out []Change, missing, short, unknown []string) {
	given := map[string]Change{}
	for _, c := range changes {
		_, twice := given[c.Ref]
		switch {
		case twice:
			unknown = append(unknown, c.Ref+" twice")
		case !slices.ContainsFunc(drift, func(d Drift) bool { return d.Ref == c.Ref }):
			unknown = append(unknown, c.Ref)
		}
		given[c.Ref] = c
	}
	for _, d := range drift {
		c := given[d.Ref]
		c = Change{Ref: d.Ref, What: strings.TrimSpace(c.What), Why: strings.TrimSpace(c.Why), Need: strings.TrimSpace(c.Need)}
		var empty []string
		for _, f := range []struct{ name, text string }{{"what", c.What}, {"why", c.Why}, {"need", c.Need}} {
			switch n := len(strings.Fields(f.text)); {
			case n == 0:
				empty = append(empty, f.name)
			case n < minWords:
				short = append(short, fmt.Sprintf("%s %s: %d words, at least %d ([baselines] why_min_words)", d.Ref, f.name, n, minWords))
			}
		}
		if len(empty) > 0 {
			missing = append(missing, fmt.Sprintf("%s (%s): %s", d.Ref, d.What(), strings.Join(empty, ", ")))
		}
		out = append(out, c)
	}
	return out, missing, short, unknown
}

// DraftHash is what a person approves: one "<ref> <hash>" line per pin, then
// one line per change and per fixed finding, so a reason edited after the
// person saw it is another draft. With neither, it is PinsHash.
func DraftHash(pins []Pin, changes []Change, fixes []string) string {
	var b strings.Builder
	for _, p := range pins {
		fmt.Fprintf(&b, "%s %s\n", p.Ref, p.Hash)
	}
	for _, c := range changes {
		j, _ := json.Marshal(c) // JSON keeps a newline in the text from making a line of its own
		fmt.Fprintf(&b, "change %s\n", j)
	}
	for _, f := range fixes {
		fmt.Fprintf(&b, "fixes %s\n", f)
	}
	return sum([]byte(b.String()))
}

var findingID = regexp.MustCompile(`^(intent|spec|tech|ux)-v([1-9][0-9]*)/([a-z][a-z-]*)/([1-9][0-9]*)$`)

// Finding reads a review finding ID, <part>-v<k>/<seat>/<n> (Story #920).
func Finding(id string) (part string, version int, seat string, ok bool) {
	m := findingID.FindStringSubmatch(id)
	if m == nil {
		return "", 0, "", false
	}
	v, _ := strconv.Atoi(m[2])
	return m[1], v, m[3], true
}

// ReviewPath is where one seat's review of version v of a part lives on the ledger.
func ReviewPath(f int, part string, v int, seat string) string {
	return fmt.Sprintf("specs/%d/reviews/%s-v%d/%s.json", f, part, v, seat)
}

// Review is the part of a seat's review record that a new version reads:
// the IDs of its findings.
type Review struct {
	Findings []ReviewFinding `json:"findings"`
}

// ReviewFinding is one finding of a review, by its ID.
type ReviewFinding struct {
	ID string `json:"id"`
}

// Has reports whether the review holds the finding id.
func (r Review) Has(id string) bool {
	return slices.ContainsFunc(r.Findings, func(f ReviewFinding) bool { return f.ID == id })
}

// VersionWarning is what a draft of version v says when v reaches warn
// ([baselines] version_warn); "" before it.
func VersionWarning(f int, part string, v, warn int) string {
	if warn <= 0 || v < warn {
		return ""
	}
	return fmt.Sprintf("The %s of #%d reaches v%d ([baselines] version_warn). Check why each review finds new gaps.", Label(part), f, v)
}

// PartState is where one part of a Feature stands, from the pins at read time.
type PartState struct {
	Part    string  `json:"part"`
	Version int     `json:"version,omitempty"` // the newest version a person approved; 0 when none
	Drift   []Drift `json:"drift"`
	Draft   int     `json:"draft,omitempty"` // a newer draft that waits for a person
	Note    string  `json:"note,omitempty"`  // why there is no version, or why its pins could not be read now
}

// String is the part's line in the Baselines block.
func (s PartState) String() string {
	var line string
	switch {
	case s.Version == 0:
		line = Label(s.Part) + ": " + s.Note
	case s.Note != "":
		line = fmt.Sprintf("%s v%d · not read: %s", Label(s.Part), s.Version, s.Note)
	case len(s.Drift) == 0:
		line = fmt.Sprintf("%s v%d · no drift", Label(s.Part), s.Version)
	default:
		ds := make([]string, len(s.Drift))
		for i, d := range s.Drift {
			ds[i] = d.String()
		}
		line = fmt.Sprintf("%s v%d · drift: %s", Label(s.Part), s.Version, strings.Join(ds, " · "))
	}
	if s.Draft > 0 {
		line += fmt.Sprintf(" · v%d draft waits for a person", s.Draft)
	}
	return line
}

// Comment is the body of a part's marker comment: the line, then, when the
// version has changes, a table of them and the findings it fixes. Agent text
// is escaped, so it never ends the table, opens markup, or mentions anyone.
func Comment(b Baseline, line string) string {
	var s strings.Builder
	s.WriteString(BaselineMarker(b.Part) + "\n" + line + "\n")
	if len(b.Changes) > 0 {
		fmt.Fprintf(&s, "\n#### What changed since v%d\n\n| Change | Why it matters | Need |\n|---|---|---|\n", b.Version-1)
		for _, c := range b.Changes {
			fmt.Fprintf(&s, "| %s: %s | %s | %s |\n", cell(c.Ref), cell(c.What), cell(c.Why), cell(c.Need))
		}
	}
	if len(b.Fixes) > 0 {
		s.WriteString("\nFixes: `" + strings.Join(b.Fixes, "`, `") + "`\n")
	}
	return s.String()
}

// cellText leaves agent text no active markup in a table cell: no cell end,
// HTML, mention, link, image, code span, or autolink (a zero-width space
// breaks "://" and "www.").
var cellText = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "|", `\|`, "@", "&#64;", "\r", " ", "\n", " ",
	"[", `\[`, "]", `\]`, "`", "\\`", "://", ":&#8203;//", "www.", "www&#8203;.")

func cell(s string) string { return cellText.Replace(s) }
