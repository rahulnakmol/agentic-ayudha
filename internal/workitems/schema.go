// Package workitems defines the ten work-item types, their Create, Ready, and
// Complete sections, the fields each stage requires, the parents and links each
// type allows, and the label set that holds Delivery Status, Priority, Effort,
// Risk, Health, and Horizon (WP2.1). Issue forms are generated from it (WP2.2,
// `go generate ./...` writes work-items/forms/). The parser that turns an issue
// body into an Item lands in WP2.7.
package workitems

import "slices"

// SchemaVersion is the work-item schema version. Ready snapshots record it, and
// readers keep accepting every version shipped since 0.1 (WP1.9 grandfathering).
const SchemaVersion = 1

// Stage is a point in a work item's life at which fields become required.
type Stage string

// Stages, in order.
const (
	StageCreate   Stage = "create"
	StageReady    Stage = "ready"
	StageComplete Stage = "complete"
)

// Stages lists the stages in order. A field required at a stage is required at
// every later stage too.
var Stages = []Stage{StageCreate, StageReady, StageComplete}

func (s Stage) index() int { return slices.Index(Stages, s) }

// Kind is how a field is entered and rendered in an issue form.
type Kind string

// Field kinds.
const (
	KindText     Kind = "text"     // one line (form input)
	KindMarkdown Kind = "markdown" // free text (form textarea)
	KindEnum     Kind = "enum"     // one of Options (form dropdown)
	KindCriteria Kind = "criteria" // numbered observable criteria (form textarea, see ParseCriteria)
	KindRefs     Kind = "refs"     // issue references, one per line, e.g. "#12" (form textarea)
	KindLinks    Kind = "links"    // typed links, one per line, e.g. "supersedes #12" (form textarea)
)

// Field is one body section of a work item. In an issue body it is the
// "### <Label>" heading GitHub renders for the form field, followed by the value.
type Field struct {
	ID          string
	Label       string
	Kind        Kind
	Options     []string // KindEnum
	RequiredAt  Stage    // empty: never required
	Description string
	Placeholder string
}

// Section groups fields by the stage that fills them.
type Section struct {
	Stage  Stage
	Fields []Field
}

// Type is a work-item type.
type Type struct {
	ID          string // slug, e.g. "story"; the type:<id> label and the form file name
	Name        string // GitHub issue type name, e.g. "Story"
	Description string
	Color       string // GitHub issue type color: gray, blue, green, yellow, orange, red, pink, purple
	LabelColor  string // hex color of the type:<id> label
	Parents     []string
	ParentAt    Stage    // stage at which a parent is required; empty: optional
	Links       []string // link kinds the type may carry (see LinkKinds)
	Effort      bool     // Effort (effort:* label) is required at Ready
	Intake      bool     // filed by outsiders through a form and classified by triage (WP2.9)
	Sections    []Section
}

// Field returns the field with id, if the type has it.
func (t Type) Field(id string) (Field, bool) {
	for _, s := range t.Sections {
		for _, f := range s.Fields {
			if f.ID == id {
				return f, true
			}
		}
	}
	return Field{}, false
}

// Fields returns every field in section order.
func (t Type) Fields() []Field {
	var out []Field
	for _, s := range t.Sections {
		out = append(out, s.Fields...)
	}
	return out
}

// HasCompletionGoal reports whether the type carries a Completion Goal (Story and Bug).
func (t Type) HasCompletionGoal() bool { _, ok := t.Field(FieldCriteria); return ok }

// Field IDs that validation and later verbs refer to by name.
const (
	FieldSummary     = "summary"
	FieldCriteria    = "completion_goal"
	FieldMeasure     = "measure"
	FieldThreshold   = "threshold"
	FieldScenario    = "scenario"
	FieldEnvironment = "environment"
	FieldReversible  = "reversible"
	FieldLinks       = "links"
	FieldBaseline    = "baseline"
	FieldTarget      = "target"
	FieldResult      = "result"
	FieldOwner       = "technical_owner"

	// Grilled spec record (WP5.3b): written by grill, checked by grill and ready.
	FieldAssumptions   = "assumptions"
	FieldOpenQuestions = "open_questions"
	FieldParked        = "parked_ideas"
	FieldProvenance    = "provenance"
)

// EvidenceKinds are the kinds of evidence a Completion Goal criterion may name.
var EvidenceKinds = []string{"screenshot", "recording", "sr-transcript", "api-trace", "metric"}

// LinkKind is a typed link between work items, written in the Links section as
// "<kind> #<number>".
type LinkKind struct {
	ID      string
	To      []string // target types
	Summary string
}

// LinkKinds lists every link kind.
var LinkKinds = []LinkKind{
	{"supersedes", []string{"decision"}, "This Decision replaces an earlier one (V8 supersede chain)."},
	{"constrains", []string{"goal", "initiative", "feature", "story", "bug"}, "This Decision constrains the item; it shows blocked while the Decision is open."},
	{"depends-on", []string{"feature", "story", "bug"}, "This item cannot start until the target is merged."},
	{"caused-by", []string{"incident"}, "This Bug was found through the Incident."},
	{"becomes", []string{"feature"}, "This accepted Feedback is delivered by the Feature."},
	{"includes", []string{"feature", "story", "bug"}, "This Release ships the item."},
	{"waives", []string{"story", "bug"}, "This floor-waiver Decision lets one floor miss through on the item (WP5.6)."},
}

// FindLink returns the link kind with id.
func FindLink(id string) (LinkKind, bool) {
	for _, k := range LinkKinds {
		if k.ID == id {
			return k, true
		}
	}
	return LinkKind{}, false
}

// DeliveryStatus lists the Delivery Status values, held in status:<value> labels.
var DeliveryStatus = []string{"backlog", "discovery", "planned", "ready", "in-progress", "blocked", "validation", "merged", "released", "measured", "done", "cancelled"}

// Label-held fields. Their single home is a label on personal repositories
// (WP2.1 "field home decided once"); Projects v2 fields are views only and never
// written as truth.
var (
	Priorities = []string{"P0", "P1", "P2", "P3"}
	Efforts    = []string{"XS", "S", "M", "L", "XL"} // XL is in the enum and blocks Ready
	Risks      = []string{"low", "medium", "high"}
	Healths    = []string{"on-track", "at-risk", "off-track"}
	Horizons   = []string{"now", "next", "later"}
)

// LabelFields maps each label-held field to its label prefix and values.
var LabelFields = []struct {
	Prefix string
	Name   string
	Values []string
}{
	{"status", "Delivery Status", DeliveryStatus},
	{"priority", "Priority", Priorities},
	{"effort", "Effort", Efforts},
	{"risk", "Risk", Risks},
	{"health", "Health", Healths},
	{"horizon", "Horizon", Horizons},
}

func criteriaFields() []Field {
	return []Field{
		{ID: FieldCriteria, Label: "Completion Goal", Kind: KindCriteria, RequiredAt: StageReady,
			Description: "Numbered observable criteria, one per line: `N. [evidence] state: <state>; expected: <what is observed>`. Evidence is one of screenshot, recording, sr-transcript, api-trace, metric.",
			Placeholder: "1. [screenshot] state: cart is empty; expected: the \"Your cart is empty\" message and a Browse button\n2. [api-trace] state: POST /cart with no items; expected: 422 with code cart.empty"},
		{ID: FieldMeasure, Label: "Measure", Kind: KindText, RequiredAt: StageReady, Description: "What is measured to decide the goal is met."},
		{ID: FieldThreshold, Label: "Threshold", Kind: KindText, RequiredAt: StageReady, Description: "The value the measure must reach."},
		{ID: FieldScenario, Label: "Scenario", Kind: KindMarkdown, RequiredAt: StageReady, Description: "The user path or request sequence the criteria are observed in."},
		{ID: FieldEnvironment, Label: "Environment", Kind: KindText, RequiredAt: StageReady, Description: "Where the evidence is captured, with seeded or synthetic data only (V3)."},
	}
}

// specRecord is what grill records next to a Completion Goal (WP5.3b): what
// the spec assumes, what is still open, the ideas the human did not confirm,
// and where each criterion came from. All optional at Ready; grill and ready
// check them when Provenance is present (internal/grill).
func specRecord() []Field {
	return []Field{
		{ID: FieldAssumptions, Label: "Assumptions", Kind: KindMarkdown, Description: "What the spec takes as given, one per line; \"none\" when there are none."},
		{ID: FieldOpenQuestions, Label: "Open questions", Kind: KindMarkdown, Description: "One per line, each \"answered: …\" or \"deferred: … (why it can wait)\"."},
		{ID: FieldParked, Label: "Parked ideas", Kind: KindMarkdown, Description: "Ideas raised while grilling that the human did not confirm. Out of scope: never built or checked for this item."},
		{ID: FieldProvenance, Label: "Provenance", Kind: KindMarkdown, Description: "The request in the human's words, how each vague word was resolved, and each criterion's source: stated, confirmed, or default."},
	}
}

func links(desc string) Field {
	return Field{ID: FieldLinks, Label: "Links", Kind: KindLinks, Description: desc, Placeholder: "depends-on #12"}
}

func result() Section {
	return Section{StageComplete, []Field{{ID: FieldResult, Label: "Result", Kind: KindMarkdown, RequiredAt: StageComplete, Description: "Measured outcome against the baseline and target, with its source query."}}}
}

// Types lists the ten work-item types in hierarchy order. The list is
// Appendix B D-2.1a: the plan names the count and some of the types.
var Types = []Type{
	{
		ID: "goal", Name: "Goal", Color: "purple", LabelColor: "5319e7",
		Description: "An outcome for a beneficiary group, measured against a baseline.",
		Sections: []Section{
			{StageCreate, []Field{
				{ID: FieldSummary, Label: "Summary", Kind: KindMarkdown, RequiredAt: StageCreate, Description: "The outcome and who benefits."},
				{ID: "beneficiaries", Label: "Beneficiaries", Kind: KindText, RequiredAt: StageReady, Description: "Beneficiary group slugs from [beneficiaries] in config."},
			}},
			{StageReady, []Field{
				{ID: "metric", Label: "Metric", Kind: KindText, RequiredAt: StageReady, Description: "The north-star metric."},
				{ID: FieldBaseline, Label: "Baseline", Kind: KindMarkdown, RequiredAt: StageReady, Description: "Current value with population, window, source query, and fetch time."},
				{ID: FieldTarget, Label: "Target", Kind: KindText, RequiredAt: StageReady, Description: "Target value and date."},
			}},
			result(),
		},
	},
	{
		ID: "initiative", Name: "Initiative", Color: "blue", LabelColor: "1d76db",
		Description: "A bet that moves a Goal's metric.",
		Parents:     []string{"goal"}, ParentAt: StageReady,
		Sections: []Section{
			{StageCreate, []Field{
				{ID: FieldSummary, Label: "Summary", Kind: KindMarkdown, RequiredAt: StageCreate},
				{ID: "hypothesis", Label: "Hypothesis", Kind: KindMarkdown, RequiredAt: StageReady, Description: "If we do X, metric Y moves by Z because W."},
			}},
			{StageReady, []Field{
				{ID: "metric", Label: "Metric", Kind: KindText, RequiredAt: StageReady},
				{ID: FieldBaseline, Label: "Baseline", Kind: KindMarkdown, RequiredAt: StageReady},
				{ID: FieldTarget, Label: "Target", Kind: KindText, RequiredAt: StageReady},
			}},
			result(),
		},
	},
	{
		ID: "feature", Name: "Feature", Color: "green", LabelColor: "0e8a16",
		Description: "A user-facing capability, sliced into stories.",
		Parents:     []string{"initiative"}, ParentAt: StageReady,
		Links: []string{"depends-on"},
		Sections: []Section{
			{StageCreate, []Field{
				{ID: FieldSummary, Label: "Summary", Kind: KindMarkdown, RequiredAt: StageCreate},
				{ID: "problem", Label: "Problem", Kind: KindMarkdown, RequiredAt: StageReady, Description: "The user problem, with evidence."},
			}},
			{StageReady, []Field{
				{ID: "scope", Label: "Scope", Kind: KindMarkdown, RequiredAt: StageReady, Description: "In scope and out of scope."},
				{ID: "expected_paths", Label: "Expected paths", Kind: KindMarkdown, Description: "Paths the Feature is expected to change; slice computes the layer footprint from them (WP5.4)."},
				links("Typed links, one per line: depends-on #N."),
			}},
			result(),
		},
	},
	{
		ID: "story", Name: "Story", Color: "yellow", LabelColor: "fbca04",
		Description: "An XS-L slice of a Feature with a Completion Goal.",
		Parents:     []string{"feature"}, ParentAt: StageReady,
		Links:  []string{"depends-on"},
		Effort: true,
		Sections: []Section{
			{StageCreate, []Field{
				{ID: FieldSummary, Label: "Summary", Kind: KindMarkdown, RequiredAt: StageCreate, Description: "As a <who>, I want <what>, so that <why>."},
			}},
			{StageReady, append(append(criteriaFields(),
				Field{ID: FieldOwner, Label: "Technical Owner", Kind: KindText, Description: "Resolved by ready: the seat's paired human, else the Code Architect twin."},
				links("Typed links, one per line: depends-on #N."),
			), specRecord()...)},
			{StageComplete, []Field{
				{ID: "outcome", Label: "Outcome", Kind: KindMarkdown, RequiredAt: StageComplete, Description: "Merged PR, authoritative pack, and what was observed."},
			}},
		},
	},
	{
		ID: "bug", Name: "Bug", Color: "red", LabelColor: "d73a4a",
		Description: "A defect, with a Completion Goal that proves the fix.",
		Parents:     []string{"feature", "story"},
		Links:       []string{"caused-by", "depends-on"},
		Effort:      true,
		Sections: []Section{
			{StageCreate, []Field{
				{ID: FieldSummary, Label: "Summary", Kind: KindMarkdown, RequiredAt: StageCreate},
				{ID: "reproduce", Label: "Steps to reproduce", Kind: KindMarkdown, RequiredAt: StageCreate},
				{ID: "expected", Label: "Expected behaviour", Kind: KindMarkdown, RequiredAt: StageCreate},
				{ID: "actual", Label: "Actual behaviour", Kind: KindMarkdown, RequiredAt: StageCreate},
			}},
			{StageReady, append(append(criteriaFields(),
				Field{ID: FieldOwner, Label: "Technical Owner", Kind: KindText, Description: "Resolved by ready."},
				links("Typed links, one per line: caused-by #N, depends-on #N."),
			), specRecord()...)},
			{StageComplete, []Field{
				{ID: "outcome", Label: "Outcome", Kind: KindMarkdown, RequiredAt: StageComplete},
			}},
		},
	},
	{
		ID: "decision", Name: "Decision", Color: "orange", LabelColor: "d93f0b",
		Description: "A choice between options with a recommendation, accepted by its authority.",
		Parents:     []string{"goal", "initiative", "feature", "story", "bug", "incident"},
		Links:       []string{"supersedes", "constrains", "waives"},
		Sections: []Section{
			{StageCreate, []Field{
				{ID: FieldSummary, Label: "Question", Kind: KindMarkdown, RequiredAt: StageCreate, Description: "The decision to make and why now."},
			}},
			{StageReady, []Field{
				{ID: "options", Label: "Options", Kind: KindMarkdown, RequiredAt: StageReady},
				{ID: "evaluation", Label: "Evaluation", Kind: KindMarkdown, RequiredAt: StageReady},
				{ID: "recommendation", Label: "Recommendation", Kind: KindMarkdown, RequiredAt: StageReady},
				{ID: FieldReversible, Label: "Reversible", Kind: KindEnum, Options: []string{"true", "false"}, RequiredAt: StageReady,
					Description: "true lets the recommendation apply provisionally when the SLA lapses (V8); false never does."},
				links("Typed links, one per line: supersedes #N, constrains #N, waives #N."),
			}},
			{StageComplete, []Field{
				{ID: "decision", Label: "Decision", Kind: KindMarkdown, RequiredAt: StageComplete, Description: "The accepted option, who accepted it, and when."},
			}},
		},
	},
	{
		ID: "experiment", Name: "Experiment", Color: "pink", LabelColor: "e99695",
		Description: "A flagged change with a hypothesis, metric, and minimum sample size.",
		Parents:     []string{"feature", "initiative"}, ParentAt: StageReady,
		Sections: []Section{
			{StageCreate, []Field{
				{ID: FieldSummary, Label: "Hypothesis", Kind: KindMarkdown, RequiredAt: StageCreate},
			}},
			{StageReady, []Field{
				{ID: "metric", Label: "Metric", Kind: KindText, RequiredAt: StageReady},
				{ID: FieldBaseline, Label: "Baseline", Kind: KindMarkdown, RequiredAt: StageReady},
				{ID: "flag", Label: "Flag", Kind: KindText, RequiredAt: StageReady, Description: "Feature flag key."},
				{ID: "min_sample", Label: "Minimum sample", Kind: KindText, RequiredAt: StageReady, Description: "Per arm, with the power calculation."},
			}},
			result(),
		},
	},
	{
		ID: "release", Name: "Release", Color: "gray", LabelColor: "bfdadc",
		Description: "A promotion that ships stories to rolling or main.",
		Parents:     []string{"goal"},
		Links:       []string{"includes"},
		Sections: []Section{
			{StageCreate, []Field{
				{ID: FieldSummary, Label: "Summary", Kind: KindMarkdown, RequiredAt: StageCreate},
				{ID: "version", Label: "Version", Kind: KindText, RequiredAt: StageReady, Description: "Snapshot timestamp or vX.Y.Z."},
			}},
			{StageReady, []Field{
				links("Typed links, one per line: includes #N."),
			}},
			{StageComplete, []Field{
				{ID: "evidence", Label: "Shipping evidence", Kind: KindMarkdown, RequiredAt: StageComplete, Description: "Artifact identity, tag, and release run."},
			}},
		},
	},
	{
		ID: "feedback", Name: "Feedback", Color: "blue", LabelColor: "c5def5",
		Description: "Intake: a request or observation from a user, classified by triage.",
		Links:       []string{"becomes"},
		Intake:      true,
		Sections: []Section{
			{StageCreate, []Field{
				{ID: FieldSummary, Label: "What happened or what you need", Kind: KindMarkdown, RequiredAt: StageCreate},
				{ID: "context", Label: "Context", Kind: KindMarkdown, Description: "Where you were and what you were trying to do."},
			}},
			{StageReady, []Field{links("Typed links, one per line: becomes #N.")}},
		},
	},
	{
		ID: "incident", Name: "Incident", Color: "red", LabelColor: "b60205",
		Description: "Intake: production impact, handled by respond.",
		Intake:      true,
		Sections: []Section{
			{StageCreate, []Field{
				{ID: FieldSummary, Label: "Impact", Kind: KindMarkdown, RequiredAt: StageCreate},
				{ID: "severity", Label: "Severity", Kind: KindEnum, Options: []string{"Sev0", "Sev1", "Sev2", "Sev3"}, RequiredAt: StageCreate,
					Description: "Sev0 and Sev1 page outside business hours."},
				{ID: "started", Label: "Started at", Kind: KindText, Description: "When the impact began, with time zone."},
			}},
			{StageComplete, []Field{
				{ID: "timeline", Label: "Timeline", Kind: KindMarkdown, RequiredAt: StageComplete},
			}},
		},
	},
}

// FindType returns the type with slug or GitHub issue type name id.
func FindType(id string) (Type, bool) {
	for _, t := range Types {
		if t.ID == id || t.Name == id {
			return t, true
		}
	}
	return Type{}, false
}

// TypeIDs lists the type slugs in order.
func TypeIDs() []string {
	out := make([]string, len(Types))
	for i, t := range Types {
		out[i] = t.ID
	}
	return out
}
