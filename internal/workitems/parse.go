package workitems

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"strings"
)

var heading = regexp.MustCompile(`^#{2,3}\s+(.+?)\s*#*\s*$`)

// Sections splits an issue body into level-2 or level-3 headed sections, in
// order: heading text to trimmed value. GitHub renders each issue-form field
// as "### <Label>" followed by its value; text before the first heading is
// ignored.
func Sections(body string) (labels []string, values map[string]string) {
	values = map[string]string{}
	cur := ""
	var buf []string
	flush := func() {
		if cur != "" {
			values[cur] = strings.TrimSpace(strings.Join(buf, "\n"))
		}
	}
	inFence := false
	for _, line := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
		}
		if m := heading.FindStringSubmatch(line); m != nil && !inFence {
			flush()
			cur, buf = m[1], nil
			labels = append(labels, cur)
			continue
		}
		buf = append(buf, line)
	}
	flush()
	return labels, values
}

// SetSection returns body with the section headed label (case-insensitive,
// level 2 or 3, outside code fences) holding value: an existing section's
// value is replaced and the rest of the body kept byte for byte; otherwise
// "### <label>" and value are appended. readout and measure write Result with it.
func SetSection(body, label, value string) string {
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	start, end := -1, len(lines)
	inFence := false
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
		}
		m := heading.FindStringSubmatch(line)
		if m == nil || inFence {
			continue
		}
		if start >= 0 {
			end = i
			break
		}
		if strings.EqualFold(m[1], label) {
			start = i
		}
	}
	value = strings.TrimSpace(value)
	if start < 0 {
		return strings.TrimRight(body, "\n") + "\n\n### " + label + "\n\n" + value + "\n"
	}
	out := append([]string{}, lines[:start+1]...)
	out = append(out, "", value)
	if end < len(lines) {
		out = append(out, "")
		out = append(out, lines[end:]...)
		return strings.Join(out, "\n")
	}
	return strings.Join(out, "\n") + "\n"
}

// TypeOf decides an issue's work-item type from its GitHub issue type name
// and its type:<id> labels; both are one notion (R1). It returns "" when the
// issue carries neither, and a problem when they disagree or name no type.
func TypeOf(issueType string, labels []string) (string, []Problem) {
	var found []string
	if issueType != "" {
		if t, ok := FindType(issueType); ok {
			found = append(found, t.ID)
		}
	}
	for _, l := range labels {
		if id, ok := strings.CutPrefix(l, "type:"); ok {
			if _, known := FindType(id); !known {
				return "", []Problem{{"type", "label " + l + " names no work-item type"}}
			}
			found = append(found, id)
		}
	}
	if len(found) == 0 {
		if issueType != "" {
			return "", []Problem{{"type", "issue type " + issueType + " is not a work-item type"}}
		}
		return "", []Problem{{"type", "no work-item type: set the issue type or a type:<type> label"}}
	}
	for _, f := range found[1:] {
		if f != found[0] {
			return "", []Problem{{"type", "the issue type and type:<type> labels disagree: " + strings.Join(found, ", ")}}
		}
	}
	return found[0], nil
}

// Parse turns an issue into an Item: the type from TypeOf, and each body
// section matched to a field of that type by its label (case-insensitive).
// Sections that are not fields of the type are reported as problems.
func Parse(number int, title, body, issueType string, labels []string) (Item, []Problem) {
	typ, ps := TypeOf(issueType, labels)
	it := Item{Type: typ, Number: number, Title: title, Labels: labels, Fields: map[string]string{}}
	if typ == "" {
		return it, ps
	}
	t, _ := FindType(typ)
	heads, values := Sections(body)
	for _, h := range heads {
		f, ok := fieldByLabel(t, h)
		if !ok {
			ps = append(ps, Problem{"body", "section \"" + h + "\" is not a " + t.Name + " field"})
			continue
		}
		if v := values[h]; !Empty(v) {
			it.Fields[f.ID] = v
		}
	}
	return it, ps
}

// fieldByLabel matches a heading to a field by its form label, or by its ID
// for bodies written through the API.
func fieldByLabel(t Type, label string) (Field, bool) {
	for _, f := range t.Fields() {
		if strings.EqualFold(f.Label, label) || f.ID == label {
			return f, true
		}
	}
	return Field{}, false
}

// Render writes item fields back as an issue body in the form's layout, in
// schema order: "### <Label>" and the value, with "_No response_" for blanks
// the way GitHub renders a form. grill and tests use it to build bodies.
func Render(t Type, fields map[string]string) string {
	var b strings.Builder
	for _, f := range t.Fields() {
		v, ok := fields[f.ID]
		if !ok {
			continue
		}
		if Empty(v) {
			v = "_No response_"
		}
		b.WriteString("### " + f.Label + "\n\n" + strings.TrimSpace(v) + "\n\n")
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

// BodyHash is the Ready snapshot's hash of an issue body: SHA-256 of the body
// with CRLF as LF, trailing spaces on each line removed, and the whole trimmed,
// so an edit GitHub's editor makes invisibly does not count as drift.
func BodyHash(body string) string {
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " \t")
	}
	sum := sha256.Sum256([]byte(strings.TrimSpace(strings.Join(lines, "\n"))))
	return hex.EncodeToString(sum[:])
}

// CompletionGoalHash is the Ready snapshot's hash of a Story or Bug's
// Completion Goal: the parsed criteria plus Measure, Threshold, Scenario, and
// Environment, as canonical JSON. It is "" for types without one. The merge
// gate (WP4.8) compares a pack's completion_goal_hash with it.
func CompletionGoalHash(it Item) string {
	t, ok := FindType(it.Type)
	if !ok || !t.HasCompletionGoal() {
		return ""
	}
	cs, _ := ParseCriteria(it.Fields[FieldCriteria])
	b, _ := json.Marshal(struct {
		Criteria    []Criterion `json:"criteria"`
		Measure     string      `json:"measure"`
		Threshold   string      `json:"threshold"`
		Scenario    string      `json:"scenario"`
		Environment string      `json:"environment"`
	}{cs, it.Fields[FieldMeasure], it.Fields[FieldThreshold], it.Fields[FieldScenario], it.Fields[FieldEnvironment]})
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
