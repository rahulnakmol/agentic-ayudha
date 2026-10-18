package workitems

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"

	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
)

func loadFixture(t *testing.T, typ string) map[Stage]Item {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "items", typ+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[Stage]Item
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("%s: %v", typ, err)
	}
	return m
}

func messages(ps []Problem) string {
	var s []string
	for _, p := range ps {
		s = append(s, p.String())
	}
	return strings.Join(s, "\n")
}

// Every type has a fixture per stage that passes that stage. A fixture filled
// only to Create fails Ready whenever the type requires anything at Ready, so
// stage gating is exercised in both directions.
func TestFixturesPerTypeAndStage(t *testing.T) {
	if len(Types) != 10 {
		t.Fatalf("%d types, want ten", len(Types))
	}
	for _, typ := range Types {
		fx := loadFixture(t, typ.ID)
		for _, st := range Stages {
			it, ok := fx[st]
			if !ok {
				t.Fatalf("%s: no %s fixture", typ.ID, st)
			}
			if ps := Validate(it, st); len(ps) > 0 {
				t.Errorf("%s at %s: want pass, got:\n%s", typ.ID, st, messages(ps))
			}
		}
		needsReady := typ.ParentAt == StageReady || typ.Effort
		for _, f := range typ.Fields() {
			needsReady = needsReady || f.RequiredAt == StageReady
		}
		if ps := Validate(fx[StageCreate], StageReady); needsReady && len(ps) == 0 {
			t.Errorf("%s: the Create fixture passes Ready", typ.ID)
		}
		if ps := Validate(fx[StageReady], StageComplete); len(ps) == 0 && typ.ID != "feedback" {
			t.Errorf("%s: the Ready fixture passes Complete", typ.ID)
		}
	}
}

func TestReadyFailures(t *testing.T) {
	story := loadFixture(t, "story")[StageReady]
	decision := loadFixture(t, "decision")[StageReady]
	feature := loadFixture(t, "feature")[StageReady]
	with := func(it Item, f func(*Item)) Item {
		c := it
		c.Fields = map[string]string{}
		for k, v := range it.Fields {
			c.Fields[k] = v
		}
		c.Labels = slices.Clone(it.Labels)
		f(&c)
		return c
	}
	tests := []struct {
		name  string
		item  Item
		stage Stage
		want  []string // every problem, exactly
	}{
		{"XL story", with(story, func(c *Item) {
			c.Labels = append(slices.DeleteFunc(c.Labels, func(l string) bool { return l == "effort:S" }), "effort:XL")
		}), StageReady,
			[]string{"effort: effort:XL blocks Ready; slice it into XS-L stories"}},
		{"XL decision", with(decision, func(c *Item) { c.Labels = append(c.Labels, "effort:XL") }), StageReady,
			[]string{"effort: effort:XL blocks Ready; slice it into XS-L stories"}},
		{"XL is fine at Create", with(story, func(c *Item) {
			c.Labels = append(slices.DeleteFunc(c.Labels, func(l string) bool { return l == "effort:S" }), "effort:XL")
		}), StageCreate, nil},
		{"unmapped criterion", with(story, func(c *Item) {
			c.Fields[FieldCriteria] = "1. [screenshot] state: empty; expected: message shown\n2. state: full; expected: total shown"
		}), StageReady, []string{"completion_goal: criterion 2 is unmapped (no evidence kind); start it with one of [screenshot], [recording], [sr-transcript], [api-trace], [metric]"}},
		{"unknown evidence kind", with(story, func(c *Item) {
			c.Fields[FieldCriteria] = "1. [vibes] state: empty; expected: message shown"
		}), StageReady, []string{`completion_goal: criterion 1 is unmapped (evidence kind "vibes"); start it with one of [screenshot], [recording], [sr-transcript], [api-trace], [metric]`}},
		{"criterion without expected", with(story, func(c *Item) {
			c.Fields[FieldCriteria] = "1. [metric] state: p95 latency under load"
		}), StageReady, []string{`completion_goal: criterion 1 needs "state: <state>; expected: <what is observed>"`}},
		{"no effort on a story", with(story, func(c *Item) { c.Labels = slices.DeleteFunc(c.Labels, func(l string) bool { return l == "effort:S" }) }), StageReady,
			[]string{"effort: Effort is required at Ready: add one of the labels effort:XS, effort:S, effort:M, effort:L"}},
		{"decision without reversible", with(decision, func(c *Item) { delete(c.Fields, FieldReversible) }), StageReady,
			[]string{"reversible: Reversible is required at Decision-ready"}},
		{"decision with a blank reversible", with(decision, func(c *Item) { c.Fields[FieldReversible] = "_No response_" }), StageReady,
			[]string{"reversible: Reversible is required at Decision-ready"}},
		{"decision reversible maybe", with(decision, func(c *Item) { c.Fields[FieldReversible] = "maybe" }), StageReady,
			[]string{`reversible: Reversible = "maybe": want one of true, false`}},
		{"decision with one option", with(decision, func(c *Item) { c.Fields["options"] = "1. **Waive.** Ship with the floor waived." }), StageReady,
			[]string{"options: 1 numbered option(s); a Decision offers at least 2, and aims for 3, so the person makes a real choice: " +
				"write each as a top-level line \"N. **Name.** what it does\", including the option of not doing it"}},
		{"decision with options that are not numbered", with(decision, func(c *Item) { c.Fields["options"] = "A: in-house. B: SurveyJS." }), StageReady,
			[]string{"options: 0 numbered option(s); a Decision offers at least 2, and aims for 3, so the person makes a real choice: " +
				"write each as a top-level line \"N. **Name.** what it does\", including the option of not doing it"}},
		{"decision with options numbered out of order", with(decision, func(c *Item) { c.Fields["options"] = "1. **A.** a\n3. **B.** b" }), StageReady,
			[]string{"options: option 3 is numbered out of order; want 2, so each option keeps the number the person picks it by"}},
		{"decision with two recommendations", with(decision, func(c *Item) { c.Fields["options"] = "1. **A.** a (recommended)\n2. **B.** b (recommended)" }), StageReady,
			[]string{"options: 2 options are marked \"(recommended)\"; mark one, the one the Recommendation names"}},
		{"decision with ten options", with(decision, func(c *Item) {
			c.Fields["options"] = "1. a\n2. b\n3. c\n4. d\n5. e\n6. f\n7. g\n8. h\n9. i\n10. j"
		}), StageReady, []string{"options: 10 options; offer at most 9, so the person picks each with one key"}},
		{"decision with two options of one name", with(decision, func(c *Item) { c.Fields["options"] = "1. **Roll out.** to 5%.\n2. **Roll out.** to 50%." }), StageReady,
			[]string{"options: options 1 and 2 are both named \"Roll out\"; give each its own name, so accepting it by name is unambiguous"}},
		{"decision with a nameless option", with(decision, func(c *Item) { c.Fields["options"] = "1. (recommended)\n2. **Use a vendor.** b" }), StageReady,
			[]string{"options: option 1 has no name; start it with its name, \"1. **Name.** what it does\""}},
		{"decision whose marker and Recommendation disagree", with(decision, func(c *Item) {
			c.Fields["options"] = "1. **A.** a (recommended)\n2. **B.** b"
			c.Fields["recommendation"] = "Option 2, because it is faster."
		}), StageReady, []string{"options: option 1 is marked \"(recommended)\" but the Recommendation names option 2; make them agree"}},
		{"decision recommending an option it lacks", with(decision, func(c *Item) { c.Fields["recommendation"] = "Option 3, the cheapest." }), StageReady,
			[]string{"recommendation: names option 3, but the Decision has options 1-2"}},
		{"decision recommending nothing it offers", with(decision, func(c *Item) { c.Fields["recommendation"] = "Something else entirely." }), StageReady,
			[]string{"recommendation: names none of the options: name one by its number (\"Option 2, because ...\") or its name"}},
		{"decision with one option still passes Create", with(decision, func(c *Item) { c.Fields["options"] = "1. **Waive.** Ship." }), StageCreate, nil},
		{"decision without reversible still passes Create", with(decision, func(c *Item) { delete(c.Fields, FieldReversible) }), StageCreate, nil},
		{"story under an initiative", with(story, func(c *Item) { c.Parent = &Ref{2, "initiative"} }), StageReady,
			[]string{"parent: #2 is a initiative; a Story's parent must be a feature"}},
		// A Story stands on its own: a vague single-feature ask needs no Goal,
		// Initiative, and Feature first (megaman, D-5.5c.12 (8), D-2.1c).
		{"story without parent is Ready", with(story, func(c *Item) { c.Parent = nil }), StageReady, nil},
		{"feature without parent", with(feature, func(c *Item) { c.Parent = nil }), StageReady,
			[]string{"parent: a Feature needs a parent initiative at Ready"}},
		{"link kind not allowed", with(story, func(c *Item) { c.Fields[FieldLinks] = "supersedes #4" }), StageReady,
			[]string{`links: a Story cannot carry a "supersedes" link; allowed: depends-on`}},
		{"supersedes a story", with(decision, func(c *Item) { c.Fields[FieldLinks] = "supersedes #9"; c.LinkTypes = map[int]string{9: "story"} }), StageReady,
			[]string{"links: supersedes #9: #9 is a story; supersedes links to a decision"}},
		{"two priorities", with(story, func(c *Item) { c.Labels = append(c.Labels, "priority:P1", "priority:P2") }), StageReady,
			[]string{"priority: Priority has 2 labels (P1, P2); keep one"}},
		{"type label disagrees", with(story, func(c *Item) { c.Labels = append(c.Labels, "type:bug") }), StageReady,
			[]string{"type: label type:bug disagrees with the Story type"}},
	}
	for _, tt := range tests {
		got := Validate(tt.item, tt.stage)
		var gs []string
		for _, p := range got {
			gs = append(gs, p.String())
		}
		if !slices.Equal(gs, tt.want) {
			t.Errorf("%s:\n got %q\nwant %q", tt.name, gs, tt.want)
		}
	}
}

func TestLabels(t *testing.T) {
	seen := map[string]bool{}
	hex := regexp.MustCompile(`^[0-9a-f]{6}$`)
	for _, l := range Labels() {
		if seen[l.Name] {
			t.Errorf("duplicate label %q", l.Name)
		}
		seen[l.Name] = true
		if len(l.Name) > 50 || len(l.Description) > 100 || !hex.MatchString(l.Color) {
			t.Errorf("label %+v: GitHub allows names <= 50 and descriptions <= 100 characters and a 6-digit hex color", l)
		}
	}
	for _, want := range []string{"type:story", "status:ready", "effort:XL", "identity:human-fallback", "decision:provisional", "needs:grill"} {
		if !seen[want] {
			t.Errorf("label %q missing", want)
		}
	}
	for _, it := range IssueTypes() {
		if !slices.Contains([]string{"gray", "blue", "green", "yellow", "orange", "red", "pink", "purple"}, it.Color) {
			t.Errorf("issue type %s: color %q is not a GitHub issue type color", it.Name, it.Color)
		}
	}
}

// The grill verb's type input is the schema's type list plus brand, the
// brand file (ground rule 4: the registry stays the single source of the CLI
// surface).
func TestGrillTypesMatchSchema(t *testing.T) {
	c, _ := verbs.Find("grill")
	for _, in := range c.Inputs {
		if in.Name == "type" && !slices.Equal(in.Enum, append(TypeIDs(), "brand")) {
			t.Fatalf("grill type enum %v, schema types %v", in.Enum, TypeIDs())
		}
	}
}

// form mirrors the GitHub issue form schema ("Syntax for issue forms" and
// "Syntax for GitHub's form schema"); unknown keys fail decoding.
type form struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Title       string   `yaml:"title"`
	Labels      []string `yaml:"labels"`
	Assignees   []string `yaml:"assignees"`
	Projects    []string `yaml:"projects"`
	Type        string   `yaml:"type"`
	Body        []struct {
		Type       string `yaml:"type"`
		ID         string `yaml:"id"`
		Attributes struct {
			Label       string   `yaml:"label"`
			Description string   `yaml:"description"`
			Placeholder string   `yaml:"placeholder"`
			Value       string   `yaml:"value"`
			Render      string   `yaml:"render"`
			Options     []string `yaml:"options"`
			Multiple    bool     `yaml:"multiple"`
			Default     *int     `yaml:"default"`
		} `yaml:"attributes"`
		Validations *struct {
			Required bool `yaml:"required"`
		} `yaml:"validations"`
	} `yaml:"body"`
}

func checkForm(src []byte) []string {
	var f form
	dec := yaml.NewDecoder(strings.NewReader(string(src)))
	dec.KnownFields(true)
	if err := dec.Decode(&f); err != nil {
		return []string{err.Error()}
	}
	var errs []string
	bad := func(s string) { errs = append(errs, s) }
	if f.Name == "" || f.Description == "" {
		bad("name and description are required")
	}
	idRE := regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	ids, labels, inputs := map[string]bool{}, map[string]bool{}, 0
	for i, el := range f.Body {
		a := el.Attributes
		switch el.Type {
		case "markdown":
			if a.Value == "" || el.ID != "" || el.Validations != nil {
				bad("markdown element needs value and takes no id or validations")
			}
			continue
		case "textarea", "input":
			if len(a.Options) > 0 {
				bad(el.Type + " takes no options")
			}
		case "dropdown":
			if len(a.Options) == 0 || slices.Contains(a.Options, "None") || len(slices.Compact(slices.Sorted(slices.Values(a.Options)))) != len(a.Options) {
				bad("dropdown needs unique options, none of them \"None\"")
			}
		default:
			bad("unknown element type " + el.Type)
		}
		inputs++
		if a.Label == "" || labels[a.Label] {
			bad("element label missing or not unique: " + a.Label)
		}
		labels[a.Label] = true
		if el.ID != "" && (!idRE.MatchString(el.ID) || ids[el.ID]) {
			bad("element id invalid or not unique: " + el.ID)
		}
		ids[el.ID] = true
		_ = i
	}
	if inputs == 0 {
		bad("a form needs at least one non-markdown field")
	}
	return errs
}

func TestFormsPassGitHubFormSchema(t *testing.T) {
	names := map[string]bool{}
	for _, typ := range Types {
		for _, opts := range []FormOptions{{}, {IssueTypes: true}} {
			src := Form(typ, opts)
			if errs := checkForm(src); len(errs) > 0 {
				t.Errorf("%s %+v:\n%s", typ.ID, opts, strings.Join(errs, "\n"))
			}
			if strings.Contains(string(src), "\ntype: ") != opts.IssueTypes {
				t.Errorf("%s: type key present = %t, want %t", typ.ID, !opts.IssueTypes, opts.IssueTypes)
			}
		}
		var f form
		_ = yaml.Unmarshal(Form(typ, FormOptions{}), &f)
		if names[f.Name] {
			t.Errorf("form name %q is not unique", f.Name)
		}
		names[f.Name] = true
		// The form requires exactly the Create-stage fields, and never offers a label-held field.
		for _, el := range f.Body[1:] {
			fld, ok := typ.Field(el.ID)
			if !ok {
				t.Errorf("%s: form element %q is not a schema field", typ.ID, el.ID)
			}
			if el.Validations.Required != (fld.RequiredAt == StageCreate) {
				t.Errorf("%s.%s: required = %t, want %t", typ.ID, el.ID, el.Validations.Required, fld.RequiredAt == StageCreate)
			}
		}
		if !slices.Contains(f.Labels, "type:"+typ.ID) || slices.Contains(f.Labels, LabelIntake) != typ.Intake {
			t.Errorf("%s: labels %v", typ.ID, f.Labels)
		}
	}
	// The checker itself rejects what GitHub rejects.
	for _, broken := range []string{
		"name: x\ndescription: y\nbody:\n  - type: markdown\n    attributes:\n      value: hi\n",
		"name: x\ndescription: y\nbody:\n  - type: dropdown\n    id: a\n    attributes:\n      label: A\n      options: [None, b]\n",
		"name: x\ndescription: y\nbody:\n  - type: input\n    id: a b\n    attributes:\n      label: A\n",
		"name: x\ndescription: y\nbody:\n  - type: input\n    attributes:\n      label: A\n  - type: textarea\n    attributes:\n      label: A\n",
		"name: x\nbody:\n  - type: input\n    attributes:\n      label: A\n",
		"name: x\ndescription: y\nbogus: 1\nbody:\n  - type: input\n    attributes:\n      label: A\n",
	} {
		if len(checkForm([]byte(broken))) == 0 {
			t.Errorf("checker accepted a broken form:\n%s", broken)
		}
	}
}

// An option's label is its bold title, without a "(recommended …)" marker,
// and a ** inside a code span does not end it; indented trade-offs and
// fenced lines are not options.
func TestParseOptions(t *testing.T) {
	os, ps := ParseOptions("1. **Seat `p` owning L07 (with `ops/**` moved) (recommended, see the Evaluation).** It owns L07.\n" +
		"   - Trade-off: gains a; costs b.\n" +
		"   1. not an option\n" +
		"```\n2. not an option either\n```\n" +
		"~~~\n2. nor this\n```\n2. nor this, inside the tilde fence\n~~~not-a-close\n2. nor this, still fenced\n~~~\n" +
		"2. **Keep the hat** (L07 stays put).\n" +
		"3. Do nothing: nothing changes.")
	want := []Option{
		{N: 1, Name: "Seat `p` owning L07 (with `ops/**` moved)", Text: "It owns L07.", Recommended: true},
		{N: 2, Name: "Keep the hat", Text: "(L07 stays put)."},
		{N: 3, Name: "Do nothing", Text: "nothing changes."},
	}
	if len(ps) != 0 || !slices.Equal(os, want) {
		t.Errorf("ParseOptions = %+v %v\nwant %+v", os, ps, want)
	}
	// A plain option's name is kept whole, however long.
	long := "Keep the current engine and revisit it next quarter once the volumes are known"
	if os, _ := ParseOptions("1. " + long + "\n2. Swap it"); len(os) != 2 || os[0].Name != long {
		t.Errorf("long plain option = %+v", os)
	}
	// Prose that names a number is not a second option.
	if os, ps := ParseOptions("1. **Keep.** Complete phase 2. before launch."); len(os) != 1 || len(ps) != 1 {
		t.Errorf("prose with a number = %+v %v", os, ps)
	}
	// Options numbered inline on one line count, in sequence only.
	os, ps = ParseOptions("1. Keep CALC_DEBUG. 2. Replace it with a --debug flag, as step 4. says.")
	if len(ps) != 0 || len(os) != 2 || os[0].Name != "Keep CALC_DEBUG" || os[1].N != 2 || os[1].Name != "Replace it with a --debug flag, as step 4" {
		t.Errorf("inline options = %+v %v", os, ps)
	}
}
