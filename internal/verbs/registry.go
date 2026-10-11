package verbs

// This file is the single source of truth for verbs, plumbing commands, seats,
// layers, lanes, and hosts (docs/BUILD-PLAN.md ground rule 4). README tables,
// LAYERS.md, docs/RACI.md, SKILL.md frontmatter, and MCP tool schemas are
// generated from it by `go generate ./...` (internal/verbs/gen); CI fails on drift.
//
// Seat modes (solo or paired), model overrides, and MCP write exposure are
// configuration (WP1.2); the data below records only identities and defaults.

// SkillVersion is the version stamped into generated SKILL.md frontmatter.
const SkillVersion = "0.0.0"

// SeatID is the stable slug of a seat.
type SeatID string

// Seat is one of the twelve pod seats. Each seat runs solo (agent only) or
// paired (agent twin plus a human); the mode is configuration, not registry data.
type Seat struct {
	ID   SeatID
	Name string
}

// LayerID identifies an application layer, L01 to L09.
type LayerID string

// Layer is an application layer of this repository with its single owner seat
// and the path globs that place a changed file in it.
type Layer struct {
	ID    LayerID
	Name  string
	Owner SeatID
	Paths []string
}

// LayerScope distinguishes "every layer" from "no layer" from an explicit list.
type LayerScope int

// Layer scopes.
const (
	ScopeNone LayerScope = iota // the command touches no layer
	ScopeSome                   // the command touches the layers listed in IDs
	ScopeAll                    // the command can touch every layer
)

// Layers is a command's layer footprint.
type Layers struct {
	Scope LayerScope
	IDs   []LayerID // set only when Scope == ScopeSome
}

// AllLayers is the footprint of a command that can touch every layer.
func AllLayers() Layers { return Layers{Scope: ScopeAll} }

// NoLayers is the footprint of a command that touches no layer.
func NoLayers() Layers { return Layers{Scope: ScopeNone} }

// OnlyLayers is the footprint of a command that touches exactly the given layers.
func OnlyLayers(ids ...LayerID) Layers { return Layers{Scope: ScopeSome, IDs: ids} }

// HostID identifies a primary host.
type HostID string

// Host is a host that runs aayudha skills.
type Host struct {
	ID   HostID
	Name string
}

// Family is a model vendor family. FamilyHostManaged means the host picks the
// model behind a mode (Amp low/medium/high) and the family is resolved at run time (WP1.6).
type Family string

// Model families.
const (
	FamilyHostManaged Family = "host-managed"
	FamilyAnthropic   Family = "anthropic"
	FamilyOpenAI      Family = "openai"
	FamilyGoogle      Family = "google"
	FamilyXAI         Family = "xai"
)

// ModelDefault is the settled default model for one lane on one host. It is
// documentation data: [models.<lane>.<host>] in config overrides it (WP1.2).
type ModelDefault struct {
	Host          HostID
	Model         string // display name, or the host mode (e.g. "Amp medium")
	Family        Family
	Escalation    string // what to use on escalation or for heavy work; empty if none
	Alternative   string // an equally acceptable default; empty if none
	ContextTokens int    // context window cap; 0 means the model default
}

// LaneID identifies a model lane.
type LaneID string

// Lane is a class of model work with per-host defaults.
type Lane struct {
	ID          LaneID
	Summary     string
	Adversarial bool // reviewer lanes: the model family must differ from the builder's
	Defaults    []ModelDefault
}

// ReviewerFamilyRule is enforced per story by the model gateway (WP1.6).
const ReviewerFamilyRule = "The reviewer model family must differ from the builder model family on every story (reviewer.family != builder.family)."

// InputType is the type of a command input.
type InputType string

// Input types.
const (
	TypeString InputType = "string"
	TypeBool   InputType = "bool"
	TypeInt    InputType = "int"
	TypeIssue  InputType = "issue" // issue number, #number, or issue URL
	TypePR     InputType = "pr"    // pull request number, #number, or PR URL
	TypePath   InputType = "path"
	TypeEnum   InputType = "enum"
)

// Issue and pull request references: 123, #123, or a GitHub URL of the right kind.
// The CLI (WP1.1) and the MCP schemas validate TypeIssue and TypePR inputs against these.
const (
	IssuePattern = `^(#?[1-9][0-9]*|https://github\.com/[^/]+/[^/]+/issues/[1-9][0-9]*)$`
	PRPattern    = `^(#?[1-9][0-9]*|https://github\.com/[^/]+/[^/]+/pull/[1-9][0-9]*)$`
)

// Input is one typed argument or flag of a command.
type Input struct {
	Name        string
	Type        InputType
	Required    bool
	Positional  bool     // positional argument (in declaration order) rather than --flag
	Enum        []string // allowed values when Type == TypeEnum
	Description string
}

// GateID identifies a precondition a command checks before it acts.
type GateID string

// Gate is a named precondition.
type Gate struct {
	ID      GateID
	Summary string
}

// Surface is a place a command is exposed.
type Surface string

// Surfaces.
const (
	SurfaceSkill    Surface = "skill"
	SurfaceCLI      Surface = "cli"
	SurfaceMCP      Surface = "mcp"
	SurfacePlumbing Surface = "plumbing"
)

// Identity is the GitHub identity a command writes under (WP1.4).
type Identity string

// Identities.
const (
	IdentityHuman Identity = "human"
	IdentityBot   Identity = "bot"
)

// Mode says who reasons when the command runs (ground rule 5).
type Mode string

// Model-calling modes.
const (
	ModeNone   Mode = "none"   // deterministic, no model
	ModeHost   Mode = "host"   // the host's model reasons inside the skill
	ModeDirect Mode = "direct" // the binary calls a model itself
	ModeHybrid Mode = "hybrid" // the host reasons, plus a direct pass by the binary
)

// Exposure is a command's MCP exposure (WP3.3).
type Exposure string

// MCP exposures.
const (
	ExposeRead  Exposure = "read"  // exposed by default
	ExposeWrite Exposure = "write" // exposed only with [mcp] expose_writes = true
	ExposeNever Exposure = "never" // human-only or admin; never exposed
)

// Command is a verb or a plumbing command.
type Command struct {
	Name     string
	Summary  string // one line, <= 200 chars; the SKILL.md description
	WP       string // the work package that implements the command
	Seat     SeatID // the single Accountable seat
	Owns     SeatID // seat that owns thresholds or policy when it differs from Seat; empty otherwise
	Lane     LaneID // empty for plumbing
	Layers   Layers
	Inputs   []Input
	Outputs  []string
	Gates    []GateID
	Surfaces []Surface
	Identity Identity
	Mode     Mode
	ModeNote string // qualifies Mode, e.g. where the direct pass runs
	Exposure Exposure
}

// Has reports whether c is exposed on surface s.
func (c Command) Has(s Surface) bool {
	for _, x := range c.Surfaces {
		if x == s {
			return true
		}
	}
	return false
}

// Seat IDs.
const (
	ProductStrategist  SeatID = "product-strategist"
	CodeArchitect      SeatID = "code-architect"
	EngineeringManager SeatID = "engineering-manager"
	AINativeEngineer   SeatID = "ai-native-engineer"
	UXResearcher       SeatID = "ux-researcher-evolutionist"
	DevOps             SeatID = "devops-devex-tinkerer"
	QualityArchitect   SeatID = "quality-architect"
	Mentalist          SeatID = "mentalist"
	Auditor            SeatID = "auditor"
	Marketer           SeatID = "marketer-documenter"
	RevenueArchitect   SeatID = "revenue-architect"
	Founder            SeatID = "founder"
)

// Seats lists the twelve seats in charter order.
var Seats = []Seat{
	{ProductStrategist, "Product Strategist"},
	{CodeArchitect, "Code Architect"},
	{EngineeringManager, "Engineering Manager"},
	{AINativeEngineer, "AI Native Engineer (+SRE hat)"},
	{UXResearcher, "UX Researcher & Evolutionist"},
	{DevOps, "DevOps & DevEx Tinkerer"},
	{QualityArchitect, "Quality Architect"},
	{Mentalist, "Mentalist"},
	{Auditor, "Auditor"},
	{Marketer, "Marketer & Documenter"},
	{RevenueArchitect, "Revenue Architect"},
	{Founder, "Founder"},
}

// Layer IDs.
const (
	L01 LayerID = "L01"
	L02 LayerID = "L02"
	L03 LayerID = "L03"
	L04 LayerID = "L04"
	L05 LayerID = "L05"
	L06 LayerID = "L06"
	L07 LayerID = "L07"
	L08 LayerID = "L08"
	L09 LayerID = "L09"
)

// LayerTable maps this repository's paths to layers and owners.
var LayerTable = []Layer{
	{L01, "Experience", UXResearcher, []string{"packages/design-system/**"}},
	{L02, "Interaction / API boundary", CodeArchitect, []string{"cmd/**", "internal/cli/**", "internal/mcp/**"}},
	{L03, "Application / workflow", EngineeringManager, []string{"internal/verbs/**", "internal/workitems/**", "internal/loops/**", "loops/**", "work-items/**", "skills/**", "seats/**"}},
	{L04, "Domain / policy", QualityArchitect, []string{"internal/verify/**", "internal/proof/**", "ops/**", "CONSTRAINTS.md"}},
	{L05, "Intelligence", AINativeEngineer, []string{"internal/models/**", "evals/**"}},
	{L06, "Contracts / integration", CodeArchitect, []string{"internal/github/**", "internal/hosts/**", "internal/notify/**", "hosts/**"}},
	{L07, "Operational data / state", AINativeEngineer, []string{"internal/ledger/**", "internal/config/**", "internal/secrets/**", "internal/identity/**", "internal/actions/**"}},
	{L08, "Analytics / reporting", RevenueArchitect, []string{"internal/telemetry/**"}},
	{L09, "Platform / operations", DevOps, []string{".github/**", ".agents/**", "embed.go", "go.mod", "go.sum"}},
}

// Host IDs.
const (
	HostAmp    HostID = "amp"
	HostCursor HostID = "cursor"
	HostClaude HostID = "claude"
)

// Hosts lists the primary hosts.
var Hosts = []Host{
	{HostAmp, "Amp"},
	{HostCursor, "Cursor"},
	{HostClaude, "Claude Code"},
}

// Lane IDs.
const (
	LaneBuild    LaneID = "build"
	LaneSpec     LaneID = "spec"
	LanePlan     LaneID = "plan"
	LaneReview   LaneID = "review"
	LaneUIReview LaneID = "ui-review"
	LaneChore    LaneID = "chore"
)

func builderDefaults() []ModelDefault {
	return []ModelDefault{
		{Host: HostAmp, Model: "Amp medium", Family: FamilyHostManaged, Escalation: "Amp high"},
		{Host: HostCursor, Model: "Claude Opus", Family: FamilyAnthropic, Alternative: "Claude Sonnet"},
		{Host: HostClaude, Model: "Claude Opus", Family: FamilyAnthropic, Alternative: "Claude Sonnet"},
	}
}

// Lanes lists the model lanes with their settled per-host defaults.
var Lanes = []Lane{
	{ID: LaneBuild, Summary: "Writes code and artifacts.", Defaults: builderDefaults()},
	{ID: LaneSpec, Summary: "Authors specs, decisions, briefs, and copy.", Defaults: builderDefaults()},
	{ID: LanePlan, Summary: "Slices and plans work.", Defaults: builderDefaults()},
	{ID: LaneReview, Summary: "Adversarial code and spec review.", Adversarial: true, Defaults: []ModelDefault{
		{Host: HostAmp, Model: "GPT-6.1 Sol", Family: FamilyOpenAI, Escalation: "GPT-6 Astra (high) for heavy reviews"},
		{Host: HostCursor, Model: "Grok 4.7", Family: FamilyXAI},
		{Host: HostClaude, Model: "GPT-6.1 Sol", Family: FamilyOpenAI, Escalation: "GPT-6 Astra (high) for heavy reviews"},
	}},
	{ID: LaneUIReview, Summary: "Visual and interaction critique of UI.", Adversarial: true, Defaults: []ModelDefault{
		{Host: HostAmp, Model: "Amp high", Family: FamilyHostManaged},
		{Host: HostCursor, Model: "Gemini 3.1 Pro", Family: FamilyGoogle, Alternative: "Fable 5.1"},
		{Host: HostClaude, Model: "Gemini 3.1 Pro", Family: FamilyGoogle, Alternative: "Fable 5.1"},
	}},
	{ID: LaneChore, Summary: "Cheap mechanical work: triage, status, checks.", Defaults: []ModelDefault{
		{Host: HostAmp, Model: "Amp low", Family: FamilyHostManaged},
		{Host: HostCursor, Model: "Haiku 5.5", Family: FamilyAnthropic, ContextTokens: 100000},
		{Host: HostClaude, Model: "Haiku 5.5", Family: FamilyAnthropic, ContextTokens: 100000},
	}},
}

// Gate IDs.
const (
	GateHalt          GateID = "halt"
	GateBudget        GateID = "budget"
	GateReady         GateID = "ready"
	GateValidator     GateID = "validator"
	GateBaseline      GateID = "baseline"
	GateLabelsOnly    GateID = "labels-only"
	GateMergeGate     GateID = "merge-gate"
	GateHumanPass     GateID = "human-pass"
	GateHumanIdentity GateID = "human-identity"
	GatePack          GateID = "authoritative-pack"
	GateResult        GateID = "result"
	GateAuthority     GateID = "decision-authority"
	GateCatalog       GateID = "ops-catalog"
	GateEnvironment   GateID = "environment-approval"
	GateMinSample     GateID = "min-sample"
	GateFigmaDiff     GateID = "figma-diff"
	GateEvalFirst     GateID = "eval-first"
	GateCodeowner     GateID = "codeowner-review"
	GateTTY           GateID = "tty-confirm"
)

// Gates lists every gate a command may declare.
var Gates = []Gate{
	{GateHalt, "The repository is not halted (aayu halt)."},
	{GateBudget, "The [budget] and [autonomy] budget limits are not exceeded."},
	{GateReady, "The issue is Ready and its body hash matches the Ready snapshot."},
	{GateValidator, "The work item passes stage validation for its type."},
	{GateBaseline, "A reproducible baseline exists before a target is set."},
	{GateLabelsOnly, "Writes are limited to labels by the actions capability."},
	{GateMergeGate, "The base-branch merge gate is green with an authoritative pack for the head SHA."},
	{GateHumanPass, "An approving review from a human (type != Bot, not a bot login)."},
	{GateHumanIdentity, "Runs only under a human identity."},
	{GatePack, "An authoritative (CI-written) proof pack exists."},
	{GateResult, "A Result exists for the work item."},
	{GateAuthority, "The actor holds [decisions] authority for the decision type or layer."},
	{GateCatalog, "The action is in the protected ops catalog at a pinned SHA."},
	{GateEnvironment, "Execution passes a GitHub Environment with required human reviewers."},
	{GateMinSample, "The experiment reached its minimum sample size."},
	{GateFigmaDiff, "Screenshot diff against Figma is within tolerance, with zero token mismatches."},
	{GateEvalFirst, "A failing eval case reproducing the finding exists before the skill diff."},
	{GateCodeowner, "A human CODEOWNER review on protected paths."},
	{GateTTY, "Interactive confirmation on a TTY; refused for agents."},
}

var (
	cliSkillMCP = []Surface{SurfaceCLI, SurfaceSkill, SurfaceMCP}
	cliSkill    = []Surface{SurfaceCLI, SurfaceSkill}
	cliOnly     = []Surface{SurfaceCLI}
	plumbing    = []Surface{SurfaceCLI, SurfacePlumbing}
)

func pos(name string, t InputType, desc string) Input {
	return Input{Name: name, Type: t, Required: true, Positional: true, Description: desc}
}

func optPos(name string, t InputType, desc string) Input {
	return Input{Name: name, Type: t, Positional: true, Description: desc}
}

func flag(name string, t InputType, desc string) Input {
	return Input{Name: name, Type: t, Description: desc}
}

func enumIn(name string, required, positional bool, desc string, values ...string) Input {
	return Input{Name: name, Type: TypeEnum, Required: required, Positional: positional, Enum: values, Description: desc}
}

// Verbs lists the 32 verbs in plan order (V7, WP0.3).
var Verbs = []Command{
	{
		Name: "grill", WP: "WP5.3",
		Summary: "Interview a human until a work item of the given type is complete and passes validation, then write it as an issue.",
		Seat:    ProductStrategist, Lane: LaneSpec, Layers: AllLayers(),
		Inputs: []Input{
			pos("type", TypeString, "Work-item type, e.g. goal or story (enum lands in WP2.1)."),
			flag("parent", TypeIssue, "Parent issue to link the new work item under."),
		},
		Outputs:  []string{"issue"},
		Gates:    []GateID{GateValidator, GateBaseline, GateHalt},
		Surfaces: cliSkillMCP, Identity: IdentityBot, Mode: ModeHost, Exposure: ExposeRead,
	},
	{
		Name: "slice", WP: "WP5.4",
		Summary: "Split a Feature into XS-L stories with observable criteria, dependencies, and a layer footprint; close XL items as split.",
		Seat:    ProductStrategist, Lane: LanePlan, Layers: AllLayers(),
		Inputs:   []Input{pos("feature", TypeIssue, "Feature (or XL story) to slice.")},
		Outputs:  []string{"stories", "footprint"},
		Gates:    []GateID{GateValidator, GateHalt},
		Surfaces: cliSkillMCP, Identity: IdentityBot, Mode: ModeHost, Exposure: ExposeRead,
	},
	{
		Name: "plan", WP: "WP5.4",
		Summary: "Plan a story: one checkpoint per layer, contracts, tests, instrumentation, and rollback.",
		Seat:    CodeArchitect, Lane: LanePlan, Layers: AllLayers(),
		Inputs:   []Input{pos("story", TypeIssue, "Story or bug to plan.")},
		Outputs:  []string{"plan"},
		Gates:    []GateID{GateValidator, GateHalt},
		Surfaces: cliSkillMCP, Identity: IdentityBot, Mode: ModeHost, Exposure: ExposeRead,
	},
	{
		Name: "decide", WP: "WP5.6",
		Summary: "Record a Decision with options, evaluation, and recommendation; accept it, supersede it, or file a floor waiver.",
		Seat:    Founder, Lane: LaneSpec, Layers: AllLayers(),
		Inputs: []Input{
			pos("decision", TypeIssue, "Decision issue."),
			flag("accept", TypeString, "Option to accept (human identity only)."),
			flag("waiver", TypeString, "Floor to waive; creates a floor-waiver Decision."),
		},
		Outputs:  []string{"decision", "waiver"},
		Gates:    []GateID{GateAuthority, GateHumanIdentity},
		Surfaces: cliSkill, Identity: IdentityHuman, Mode: ModeHost, Exposure: ExposeNever,
	},
	{
		Name: "triage", WP: "WP2.9",
		Summary: "Classify intake issues as Feedback, Bug, or Incident by label only; never edits bodies or touches non-intake issues.",
		Seat:    ProductStrategist, Lane: LaneChore, Layers: NoLayers(),
		Inputs:   []Input{pos("issue", TypeIssue, "Intake issue to classify.")},
		Outputs:  []string{"labels"},
		Gates:    []GateID{GateLabelsOnly, GateHalt},
		Surfaces: cliSkillMCP, Identity: IdentityBot, Mode: ModeHybrid, ModeNote: "direct in Actions", Exposure: ExposeWrite,
	},
	{
		Name: "build", WP: "WP5.5",
		Summary: "Build a Ready story or bug in a worktree with checkpoint commits per layer, verify each checkpoint, and open a PR.",
		Seat:    AINativeEngineer, Lane: LaneBuild, Layers: AllLayers(),
		Inputs: []Input{
			pos("issue", TypeIssue, "Ready story or bug to build."),
			flag("resume", TypeBool, "Resume from the last Aayudha-Checkpoint trailer."),
		},
		Outputs:  []string{"pr", "checkpoint-commits", "progress-comment"},
		Gates:    []GateID{GateReady, GateBudget, GateHalt},
		Surfaces: cliSkillMCP, Identity: IdentityBot, Mode: ModeHost, Exposure: ExposeWrite,
	},
	{
		Name: "verify", WP: "WP4.3",
		Summary: "Run the deterministic floors (coverage, SAST, secrets, deps, a11y, perf) and record results in a proof pack.",
		Seat:    AINativeEngineer, Owns: QualityArchitect, Lane: LaneChore, Layers: AllLayers(),
		Inputs: []Input{
			optPos("pr", TypePR, "Pull request to verify; defaults to the current worktree."),
			flag("floor", TypeString, "Run only this floor."),
		},
		Outputs:  []string{"floor-results", "pack"},
		Gates:    []GateID{GateHalt},
		Surfaces: cliSkillMCP, Identity: IdentityBot, Mode: ModeNone, Exposure: ExposeWrite,
	},
	{
		Name: "inspect", WP: "WP4.8",
		Summary: "Adversarially review a PR against its plan, pack, and criteria; return a structured disposition: merge, fix, or rebuild.",
		Seat:    EngineeringManager, Lane: LaneReview, Layers: AllLayers(),
		Inputs:   []Input{pos("pr", TypePR, "Pull request to inspect.")},
		Outputs:  []string{"disposition (merge|fix|rebuild)", "findings"},
		Gates:    []GateID{GatePack, GateBudget},
		Surfaces: cliSkillMCP, Identity: IdentityBot, Mode: ModeDirect, Exposure: ExposeWrite,
	},
	{
		Name: "ship", WP: "WP5.5",
		Summary: "Squash-merge a PR after a green merge gate and a human pass, write the Complete section, and clean up.",
		Seat:    EngineeringManager, Lane: LaneChore, Layers: AllLayers(),
		Inputs:   []Input{pos("pr", TypePR, "Pull request to ship.")},
		Outputs:  []string{"merge-commit", "complete-section", "status:merged"},
		Gates:    []GateID{GateMergeGate, GateHumanPass},
		Surfaces: cliSkill, Identity: IdentityHuman, Mode: ModeNone, Exposure: ExposeNever,
	},
	{
		Name: "release", WP: "WP7.8",
		Summary: "Promote a Release along dev, rolling, main; stamp included stories released and trigger measurement.",
		Seat:    DevOps, Lane: LaneChore, Layers: OnlyLayers(L09),
		Inputs: []Input{
			pos("release", TypeIssue, "Release issue."),
			enumIn("to", true, false, "Branch to promote to.", "rolling", "main"),
		},
		Outputs:  []string{"tag", "released-stories", "release-notes"},
		Gates:    []GateID{GateHumanIdentity, GateTTY, GateHalt},
		Surfaces: cliSkill, Identity: IdentityHuman, Mode: ModeNone, Exposure: ExposeNever,
	},
	{
		Name: "proof", WP: "WP4.2",
		Summary: "Push, get, verify, list, or purge proof packs on the ledger branch.",
		Seat:    QualityArchitect, Lane: LaneChore, Layers: AllLayers(),
		Inputs: []Input{
			enumIn("action", true, true, "Proof operation.", "push", "get", "verify", "list", "purge"),
			optPos("story", TypeIssue, "Story whose pack to operate on."),
		},
		Outputs:  []string{"pack", "verification-report"},
		Surfaces: cliSkillMCP, Identity: IdentityBot, Mode: ModeNone, Exposure: ExposeRead,
	},
	{
		Name: "ready", WP: "WP2.7",
		Summary: "Validate a work item for Ready, label it status:ready, and post the Ready snapshot; --check is read-only.",
		Seat:    ProductStrategist, Lane: LaneChore, Layers: AllLayers(),
		Inputs: []Input{
			pos("issue", TypeIssue, "Work item to make Ready."),
			flag("check", TypeBool, "Validate only; write nothing."),
		},
		Outputs:  []string{"status:ready", "ready-snapshot"},
		Gates:    []GateID{GateValidator},
		Surfaces: cliSkillMCP, Identity: IdentityBot, Mode: ModeNone, Exposure: ExposeRead,
	},
	{
		Name: "research", WP: "WP6.5",
		Summary: "Gather pinned design references (URLs and IDs only) and user research for a Feature.",
		Seat:    UXResearcher, Lane: LaneSpec, Layers: OnlyLayers(L01),
		Inputs:   []Input{pos("feature", TypeIssue, "Feature to research.")},
		Outputs:  []string{"reference-set"},
		Gates:    []GateID{GateHalt},
		Surfaces: cliSkillMCP, Identity: IdentityBot, Mode: ModeHost, Exposure: ExposeRead,
	},
	{
		Name: "wireframe", WP: "WP6.5",
		Summary: "Produce wireframes for a Feature's states from its research and criteria.",
		Seat:    UXResearcher, Lane: LaneBuild, Layers: OnlyLayers(L01),
		Inputs:   []Input{pos("feature", TypeIssue, "Feature to wireframe.")},
		Outputs:  []string{"wireframes"},
		Gates:    []GateID{GateHalt, GateBudget},
		Surfaces: cliSkillMCP, Identity: IdentityBot, Mode: ModeHost, Exposure: ExposeWrite,
	},
	{
		Name: "mockup", WP: "WP6.5",
		Summary: "Produce design-system mockups for a Feature from its wireframes.",
		Seat:    UXResearcher, Lane: LaneBuild, Layers: OnlyLayers(L01),
		Inputs:   []Input{pos("feature", TypeIssue, "Feature to mock up.")},
		Outputs:  []string{"mockups"},
		Gates:    []GateID{GateHalt, GateBudget},
		Surfaces: cliSkillMCP, Identity: IdentityBot, Mode: ModeHost, Exposure: ExposeWrite,
	},
	{
		Name: "prototype", WP: "WP6.5",
		Summary: "Build a flagged, working prototype of a Feature with Code Connect mapping and a Figma screenshot diff.",
		Seat:    UXResearcher, Lane: LaneBuild, Layers: OnlyLayers(L01, L02),
		Inputs:   []Input{pos("feature", TypeIssue, "Feature to prototype.")},
		Outputs:  []string{"prototype", "feature-flag", "figma-diff"},
		Gates:    []GateID{GateFigmaDiff, GateHalt, GateBudget},
		Surfaces: cliSkillMCP, Identity: IdentityBot, Mode: ModeHost, Exposure: ExposeWrite,
	},
	{
		Name: "review", WP: "WP6.6",
		Summary: "Critique UI with a model lane and a deterministic lane; dispositions recapture, rebuild, fix, or ship.",
		Seat:    UXResearcher, Lane: LaneUIReview, Layers: OnlyLayers(L01),
		Inputs: []Input{
			pos("pr", TypePR, "Pull request whose UI to review."),
			flag("deep", TypeBool, "Run lane A at high effort."),
		},
		Outputs:  []string{"critique", "disposition (recapture|rebuild|fix|ship)"},
		Gates:    []GateID{GateBudget},
		Surfaces: cliSkillMCP, Identity: IdentityBot, Mode: ModeHybrid, ModeNote: "lane A direct", Exposure: ExposeWrite,
	},
	{
		Name: "polish", WP: "WP6.6",
		Summary: "Apply a review critique to a PR's UI and re-run review.",
		Seat:    UXResearcher, Lane: LaneBuild, Layers: OnlyLayers(L01),
		Inputs:   []Input{pos("pr", TypePR, "Pull request to polish.")},
		Outputs:  []string{"commits"},
		Gates:    []GateID{GateHalt, GateBudget},
		Surfaces: cliSkillMCP, Identity: IdentityBot, Mode: ModeHost, Exposure: ExposeWrite,
	},
	{
		Name: "baseline", WP: "WP6.6",
		Summary: "Capture visual baselines for a commit SHA.",
		Seat:    UXResearcher, Lane: LaneChore, Layers: OnlyLayers(L01),
		Inputs:   []Input{flag("sha", TypeString, "Commit to baseline; defaults to HEAD.")},
		Outputs:  []string{"baseline-captures"},
		Gates:    []GateID{GateHalt},
		Surfaces: cliSkillMCP, Identity: IdentityBot, Mode: ModeNone, Exposure: ExposeWrite,
	},
	{
		Name: "experiment", WP: "WP6.7",
		Summary: "Design an experiment for a Feature: hypothesis, metric, flag, and minimum sample size.",
		Seat:    RevenueArchitect, Lane: LanePlan, Layers: OnlyLayers(L03, L08),
		Inputs:   []Input{pos("feature", TypeIssue, "Feature to experiment on.")},
		Outputs:  []string{"experiment", "feature-flag"},
		Gates:    []GateID{GateBaseline, GateHalt},
		Surfaces: cliSkillMCP, Identity: IdentityBot, Mode: ModeHost, Exposure: ExposeWrite,
	},
	{
		Name: "readout", WP: "WP6.7",
		Summary: "Read out an experiment's lift once it reaches minimum sample and write the Result.",
		Seat:    RevenueArchitect, Lane: LaneSpec, Layers: OnlyLayers(L08),
		Inputs:   []Input{pos("experiment", TypeIssue, "Experiment to read out.")},
		Outputs:  []string{"result"},
		Gates:    []GateID{GateMinSample},
		Surfaces: cliSkillMCP, Identity: IdentityBot, Mode: ModeHost, Exposure: ExposeWrite,
	},
	{
		Name: "respond", WP: "WP5.6",
		Summary: "Respond to an incident: propose an ops catalog action, execute it only through human approval, keep the timeline.",
		Seat:    AINativeEngineer, Lane: LaneBuild, Layers: OnlyLayers(L07, L09),
		Inputs: []Input{
			pos("incident", TypeIssue, "Incident issue."),
			flag("action", TypeString, "Catalog action to propose."),
		},
		Outputs:  []string{"timeline", "proposed-action", "linked-bug"},
		Gates:    []GateID{GateCatalog, GateEnvironment, GateHumanIdentity},
		Surfaces: cliSkill, Identity: IdentityHuman, Mode: ModeHost, Exposure: ExposeNever,
	},
	{
		Name: "status", WP: "WP2.8",
		Summary: "Roll up status, health, risk, proof coverage, decisions, and cost per Goal, Initiative, and Feature.",
		Seat:    EngineeringManager, Lane: LaneChore, Layers: AllLayers(),
		Inputs: []Input{
			optPos("issue", TypeIssue, "Goal, Initiative, or Feature to roll up; defaults to all Goals."),
			flag("digest", TypeBool, "Send the digest through [notify]."),
			flag("all", TypeBool, "Iterate [fleet] repos."),
		},
		Outputs:  []string{"rollup", "goal-comment", "digest"},
		Surfaces: cliSkillMCP, Identity: IdentityBot, Mode: ModeNone, Exposure: ExposeRead,
	},
	{
		Name: "audit", WP: "WP7.6",
		Summary: "Audit floors, proof coverage, waivers, decision SLAs, autonomy exceptions, and unlogged writes; write a reproducible report.",
		Seat:    Auditor, Lane: LaneReview, Layers: AllLayers(),
		Inputs:   []Input{flag("since", TypeString, "Start of the audit window (date).")},
		Outputs:  []string{"audit-report"},
		Surfaces: cliSkillMCP, Identity: IdentityBot, Mode: ModeNone, Exposure: ExposeWrite,
	},
	{
		Name: "retro", WP: "WP7.6",
		Summary: "Compare estimates with observed delivery, corrections, floor misses, and cost; file findings as Feedback issues.",
		Seat:    EngineeringManager, Lane: LaneSpec, Layers: AllLayers(),
		Inputs:   []Input{flag("since", TypeString, "Start of the retro window (date).")},
		Outputs:  []string{"retro-report", "feedback-issues"},
		Gates:    []GateID{GateHalt},
		Surfaces: cliSkillMCP, Identity: IdentityBot, Mode: ModeHost, Exposure: ExposeWrite,
	},
	{
		Name: "measure", WP: "WP7.2",
		Summary: "Measure a Goal or Initiative against its baseline from analytics exports and write the Result.",
		Seat:    RevenueArchitect, Lane: LaneChore, Layers: OnlyLayers(L08),
		Inputs:   []Input{optPos("issue", TypeIssue, "Goal or Initiative to measure.")},
		Outputs:  []string{"result", "snapshot"},
		Gates:    []GateID{GateBaseline},
		Surfaces: cliSkillMCP, Identity: IdentityBot, Mode: ModeNone, Exposure: ExposeWrite,
	},
	{
		Name: "evolve", WP: "WP7.7",
		Summary: "Turn accepted retro Feedback into a failing eval case, then a skill diff that passes it, opened as an agent-authored PR.",
		Seat:    AINativeEngineer, Lane: LaneBuild, Layers: OnlyLayers(L05),
		Inputs:   []Input{flag("skill", TypeString, "Limit to one skill.")},
		Outputs:  []string{"eval-case", "skill-pr"},
		Gates:    []GateID{GateEvalFirst, GateCodeowner, GateHalt},
		Surfaces: cliSkillMCP, Identity: IdentityBot, Mode: ModeHost, Exposure: ExposeWrite,
	},
	{
		Name: "brief", WP: "WP7.3",
		Summary: "Explain work to an audience in a chosen register and purpose as a grounded, accessible HTML brief.",
		Seat:    Mentalist, Lane: LaneSpec, Layers: AllLayers(),
		Inputs: []Input{
			pos("subject", TypeIssue, "Work item to brief on."),
			enumIn("register", false, false, "Audience register.", "eli5", "ste", "engineer", "regulatory"),
			enumIn("purpose", false, false, "Purpose of the brief.", "explain", "ideate", "innovate", "close"),
		},
		Outputs:  []string{"html-brief"},
		Gates:    []GateID{GatePack, GateResult},
		Surfaces: cliSkillMCP, Identity: IdentityBot, Mode: ModeHybrid, ModeNote: "adversarial pass direct", Exposure: ExposeRead,
	},
	{
		Name: "tell", WP: "WP7.5",
		Summary: "Write docs, guides, changelogs, release notes, launch copy, or brand copy that cite only proven Features; publish is human-only.",
		Seat:    Marketer, Lane: LaneSpec, Layers: OnlyLayers(L01),
		Inputs: []Input{
			enumIn("kind", true, true, "What to write.", "docs", "guide", "changelog", "release", "launch", "brand"),
			flag("publish", TypeBool, "Publish (human identity only)."),
		},
		Outputs:  []string{"draft", "published-content"},
		Gates:    []GateID{GatePack, GateHumanIdentity},
		Surfaces: cliSkillMCP, Identity: IdentityHuman, Mode: ModeHost, ModeNote: "publish requires the human identity", Exposure: ExposeWrite,
	},
	{
		Name: "init", WP: "WP2.5",
		Summary: "Set up a product repo: labels, issue types, forms, config, ledger branch, and optionally projects, gates, identity, integrations.",
		Seat:    Founder, Lane: LaneChore, Layers: OnlyLayers(L09),
		Inputs: []Input{
			flag("resume", TypeBool, "Resume a partial run from .aayudha/state.json."),
			flag("projects", TypeBool, "Create Projects v2 views."),
			flag("gates", TypeBool, "Install gate workflows."),
			flag("identity", TypeBool, "Create the GitHub App bot identity."),
			flag("integrations", TypeBool, "Configure external integrations."),
		},
		Outputs:  []string{"labels", "issue-types", "forms", "config", "ledger-branch", "state"},
		Gates:    []GateID{GateTTY},
		Surfaces: cliOnly, Identity: IdentityHuman, Mode: ModeNone, Exposure: ExposeNever,
	},
	{
		Name: "install", WP: "WP3.2",
		Summary: "Install skill copies, lock.toml, MCP registration, and hooks for each host; shows a diff unless --apply.",
		Seat:    DevOps, Lane: LaneChore, Layers: OnlyLayers(L09),
		Inputs: []Input{
			enumIn("host", false, false, "Host to install for.", "amp", "cursor", "claude", "all"),
			flag("apply", TypeBool, "Write the changes instead of showing a diff."),
			flag("pin", TypeString, "Skill version to pin."),
		},
		Outputs:  []string{"skill-copies", "lock", "mcp-registration", "hooks"},
		Gates:    []GateID{GateTTY},
		Surfaces: cliOnly, Identity: IdentityHuman, Mode: ModeNone, Exposure: ExposeNever,
	},
	{
		Name: "doctor", WP: "WP1.7",
		Summary: "Check tools, tokens, identity, config, host installs, and existing CI; --fix re-syncs copies after showing a diff.",
		Seat:    DevOps, Lane: LaneChore, Layers: OnlyLayers(L09),
		Inputs: []Input{
			flag("fix", TypeBool, "Re-sync copies and adapters after showing a diff."),
			flag("all", TypeBool, "Iterate [fleet] repos."),
		},
		Outputs:  []string{"report"},
		Surfaces: cliSkillMCP, Identity: IdentityHuman, Mode: ModeNone, Exposure: ExposeRead,
	},
}

func plumb(name, wp string, seat SeatID, id Identity, summary string, inputs ...Input) Command {
	return Command{
		Name: name, WP: wp, Summary: summary, Seat: seat, Layers: NoLayers(), Inputs: inputs,
		Surfaces: plumbing, Identity: id, Mode: ModeNone, Exposure: ExposeNever,
	}
}

// Plumbing lists the plumbing commands, which are not verbs (V7).
var Plumbing = []Command{
	plumb("config", "WP1.2", DevOps, IdentityHuman, "Show, validate, or initialise layered configuration.",
		enumIn("action", true, true, "Config operation.", "show", "validate", "init"),
		flag("origin", TypeBool, "Show which layer set each value.")),
	plumb("auth", "WP1.4", DevOps, IdentityHuman, "Show which human token and bot identity aayudha resolves, and their scopes.",
		enumIn("action", false, true, "Auth operation.", "status")),
	plumb("actions", "WP1.5", DevOps, IdentityHuman, "Tail, search, or verify the hash chain of the actions log.",
		enumIn("action", true, true, "Actions-log operation.", "tail", "grep", "verify-chain"),
		optPos("pattern", TypeString, "Pattern for grep.")),
	plumb("models", "WP1.6", DevOps, IdentityHuman, "Show the resolved model for each lane and host.",
		enumIn("action", true, true, "Models operation.", "show")),
	plumb("mcp", "WP3.3", DevOps, IdentityBot, "Serve verbs as MCP tools over stdio."),
	plumb("self-update", "WP1.1", DevOps, IdentityHuman, "Replace the binary with a signed, newer release; refuses downgrades and agents."),
	plumb("upgrade", "WP1.9", DevOps, IdentityHuman, "Migrate repo config, lock, state, and skill copies to this binary's schema versions.",
		flag("apply", TypeBool, "Apply the migration instead of showing a diff.")),
	plumb("version", "WP1.1", DevOps, IdentityHuman, "Print the binary version."),
	plumb("halt", "WP5.7", Founder, IdentityHuman, "Stop every agent lane, loop, and workflow in the repo until resume.",
		flag("reason", TypeString, "Reason recorded with the halt.")),
	plumb("resume", "WP5.7", Founder, IdentityHuman, "Clear a halt."),
	plumb("gc", "WP5.5", DevOps, IdentityHuman, "Prune stale worktrees and leases."),
	plumb("uninit", "WP2.5", DevOps, IdentityHuman, "Remove what init created, by stored IDs.",
		flag("external", TypeBool, "Also list orphaned external resources.")),
	plumb("hook", "WP3.2", DevOps, IdentityBot, "Entry point for host hooks; hooks call only `aayu hook <name>`.",
		pos("name", TypeString, "Hook name.")),
	plumb("seat", "WP5.1", Founder, IdentityHuman, "Add, remove, or list seats and their paired humans.",
		enumIn("action", true, true, "Seat operation.", "add", "remove", "list"),
		optPos("seat", TypeString, "Seat slug."),
		flag("human", TypeString, "GitHub login of the paired human (add).")),
	plumb("lfs-agent", "0.2 (V6)", DevOps, IdentityHuman, "git-lfs custom transfer agent for S3/R2 storage."),
}
