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

// Seat is one of the fourteen pod seats. Each seat runs solo (agent only) or
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
	// Excludes lists model families the host cannot run. A lane configured with
	// an excluded family on this host falls back to the lane's registry default
	// for the host, and the model gateway logs the substitution (WP1.6).
	Excludes []Family
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
	Host             HostID
	Model            string // display name, or the host mode (e.g. "Amp medium")
	ID               string // model ID at the AI gateway or provider; empty for host-managed modes
	Family           Family
	Effort           string // reasoning effort to request; empty means the provider default
	Escalation       string // what to use on escalation or for heavy work; empty if none
	EscalationID     string // model ID of the escalation, when it is a model rather than a host mode
	EscalationEffort string // reasoning effort for the escalation model
	Alternative      string // an equally acceptable default; empty if none
	AlternativeID    string // model ID of the alternative, used as a fallback when the default cannot be called
	ContextTokens    int    // context window cap; 0 means the model default
	Note             string // e.g. a substitution the gateway forces
	// Plan is the host plan the default applies to ([hosts.cursor] plan,
	// Decision #469); empty for every plan.
	Plan string
}

// Cursor's plans (Decision #469): since 12 October 2026 a new plan ("x")
// includes only Cursor's own models (Grok, Composer), and Claude, GPT, and
// Gemini are list-price pay-as-you-go; a legacy Pro, Pro+, or Ultra plan
// keeps them included. aayu cannot tell the plans apart, so "x", which never
// spends list-price money by surprise, is the default.
const (
	PlanX      = "x"
	PlanLegacy = "legacy"
)

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
	// HumanOnly inputs act under the human identity (tell --publish, decide
	// --accept); the MCP server never exposes them (WP3.3).
	HumanOnly bool
	// ReadOnlyValues pins the input of an ExposeRead command to the values that
	// keep it read-only while [mcp] expose_writes is false (WP3.3), e.g.
	// ready --check = "true". Bool inputs use "true" or "false".
	ReadOnlyValues []string
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

// GlobalFlags are the names of the CLI's global flags (WP1.1); GlobalValueFlags
// are those that take a value. A registry input with a global flag's name reads
// the global flag instead of defining its own.
var (
	GlobalFlags      = []string{"dry-run", "json", "repo", "host", "yes", "verbose", "set", "ci", "plain"}
	GlobalValueFlags = []string{"repo", "host", "set"}
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

// StageID names a stage of the journey from an idea to a measured release.
type StageID string

// Journey stages, in reading order.
const (
	StageSetup     StageID = "setup"
	StageShape     StageID = "shape"
	StageBreakDown StageID = "break-down"
	StageReady     StageID = "ready"
	StageDesign    StageID = "design"
	StageBuild     StageID = "build"
	StageProve     StageID = "prove"
	StageShip      StageID = "ship"
	StageRelease   StageID = "release"
	StageLearn     StageID = "learn"
	StageExplain   StageID = "explain"
	StageOperate   StageID = "operate"
)

// Stage is a journey stage with the title readers see. The docs site's verb
// map groups the verbs by stage (WP8.2).
type Stage struct {
	ID    StageID
	Title string
}

// Stages lists the journey stages in reading order. Every verb has exactly one.
var Stages = []Stage{
	{StageSetup, "Set up"},
	{StageShape, "Shape the goal"},
	{StageBreakDown, "Break it down"},
	{StageReady, "Get a story Ready"},
	{StageDesign, "Design the screens"},
	{StageBuild, "Build it"},
	{StageProve, "Prove it"},
	{StageShip, "Ship it"},
	{StageRelease, "Release it"},
	{StageLearn, "Measure and learn"},
	{StageExplain, "Explain it"},
	{StageOperate, "Keep it running"},
}

// Command is a verb or a plumbing command.
type Command struct {
	Name     string
	Summary  string  // one line, <= 200 chars; the SKILL.md description
	Reader   string  // one plain sentence for people new to aayudha: the docs site's verb map and journey pages; verbs only
	WP       string  // the work package that implements the command
	Stage    StageID // the journey stage (verbs only; empty for plumbing)
	Seat     SeatID  // the single Accountable seat
	Owns     SeatID  // seat that owns thresholds or policy when it differs from Seat; empty otherwise
	Lane     LaneID  // empty for plumbing
	Layers   Layers
	Inputs   []Input
	Outputs  []string
	Gates    []GateID
	Surfaces []Surface
	Identity Identity
	Mode     Mode
	ModeNote string // qualifies Mode, e.g. where the direct pass runs
	Exposure Exposure
	ReadOnly bool // never writes; still runs on a repo whose schema is newer than the binary (WP1.9)
	// Paths are glob patterns that limit when a host loads the skill on its own
	// (SKILL.md `paths`, WP3.1). Empty means no restriction and the key is omitted.
	Paths []string
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
	DataAIArchitect    SeatID = "data-ai-architect" // the Code Architect's counterpart for data and AI (Decision #283)
	EngineeringManager SeatID = "engineering-manager"
	AINativeEngineer   SeatID = "ai-native-engineer"
	ProductionEngineer SeatID = "production-engineer" // runs the software the AI Native Engineer builds (Decision #324)
	UXResearcher       SeatID = "ux-researcher-evolutionist"
	DevOps             SeatID = "devops-devex-tinkerer"
	QualityArchitect   SeatID = "quality-architect"
	Mentalist          SeatID = "mentalist"
	Auditor            SeatID = "auditor"
	Marketer           SeatID = "marketer-documenter"
	RevenueArchitect   SeatID = "revenue-architect"
	Founder            SeatID = "founder"
)

// Seats lists the fourteen seats in charter order.
var Seats = []Seat{
	{ProductStrategist, "Product Strategist"},
	{CodeArchitect, "Code Architect"},
	{DataAIArchitect, "Data & AI Architect"},
	{EngineeringManager, "Engineering Manager"},
	{AINativeEngineer, "AI Native Engineer"},
	{ProductionEngineer, "Production Engineer"},
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

// LayerTable maps this repository's paths to layers and owners, including
// the nine-layer example product examples/benefits-qa (WP8.1).
var LayerTable = []Layer{
	{L01, "Experience", UXResearcher, []string{"packages/design-system/**", "examples/benefits-qa/web/**"}},
	{L02, "Interaction / API boundary", CodeArchitect, []string{"cmd/**", "internal/cli/**", "internal/mcp/**", "examples/benefits-qa/api/**", "examples/benefits-qa/cmd/**"}},
	// L03 also holds every verb's own package under internal/ that no other
	// layer names (Story #1210): L03 is the default, and moving one to another
	// layer is the Code Architect's call.
	{L03, "Application / workflow", EngineeringManager, []string{"internal/verbs/**", "internal/workitems/**", "internal/loops/**",
		"internal/agentenv/**", "internal/audit/**", "internal/autonomy/**", "internal/brain/**", "internal/brief/**", "internal/chart/**",
		"internal/counsel/**", "internal/decide/**", "internal/delegate/**", "internal/design/**", "internal/doctor/**", "internal/experiment/**",
		"internal/gittest/**", "internal/grill/**", "internal/initrepo/**", "internal/inspect/**", "internal/journey/**", "internal/measure/**",
		"internal/ops/**", "internal/planning/**", "internal/prbody/**", "internal/production/**", "internal/ratchet/**", "internal/ready/**",
		"internal/scope/**", "internal/seats/**", "internal/skills/**", "internal/status/**", "internal/survey/**", "internal/tell/**",
		"internal/testbin/**", "internal/triage/**", "internal/ui/**", "internal/uireview/**", "internal/upgrade/**",
		"loops/**", "work-items/**", "skills/**", "seats/**", "examples/benefits-qa/ask/**"}},
	{L04, "Domain / policy", QualityArchitect, []string{"internal/verify/**", "internal/proof/**", "CONSTRAINTS.md", "examples/benefits-qa/policy/**"}},
	{L05, "Intelligence", DataAIArchitect, []string{"internal/models/**", "internal/evals/**", "evals/**", "examples/benefits-qa/answer/**"}},
	{L06, "Contracts / integration", CodeArchitect, []string{"internal/github/**", "internal/hosts/**", "internal/integrations/**", "internal/notify/**", "hosts/**", "examples/benefits-qa/contracts/**"}},
	{L07, "Operational data / state", ProductionEngineer, []string{"internal/ledger/**", "internal/config/**", "internal/secrets/**", "internal/identity/**", "internal/actions/**", "ops/**", "examples/benefits-qa/store/**"}},
	{L08, "Analytics / reporting", DataAIArchitect, []string{"internal/telemetry/**", "examples/benefits-qa/analytics/**"}},
	{L09, "Platform / operations", DevOps, []string{".github/**", ".agents/**", "embed.go", "go.mod", "go.sum", "examples/benefits-qa/deploy/**"}},
}

// Host IDs.
const (
	HostAmp    HostID = "amp"
	HostCursor HostID = "cursor"
	HostClaude HostID = "claude"
	HostCodex  HostID = "codex" // OpenAI Codex CLI and IDE extension (WP3.2b)
	// HostCopilot is GitHub Copilot: Copilot CLI and VS Code agent mode, which
	// share one runtime and its skill, hook, and MCP formats.
	HostCopilot HostID = "copilot"
)

// Hosts lists the primary hosts.
var Hosts = []Host{
	{ID: HostAmp, Name: "Amp"},
	{ID: HostCursor, Name: "Cursor", Excludes: []Family{FamilyOpenAI}}, // GPT lanes fall back to Grok on Cursor
	{ID: HostClaude, Name: "Claude Code"},
	{ID: HostCodex, Name: "OpenAI Codex"},
	{ID: HostCopilot, Name: "GitHub Copilot"},
}

// Lane IDs.
const (
	LaneBuild    LaneID = "build"
	LaneSpec     LaneID = "spec"
	LanePlan     LaneID = "plan"
	LaneReview   LaneID = "review"
	LaneUIReview LaneID = "ui-review"
	LaneChore    LaneID = "chore"
	LaneMission  LaneID = "mission"
)

// Model IDs as the AI gateway lists them (GET /v1/models, verified 2026-10-11).
const (
	ModelClaudeOpus   = "claude-opus-5-5"
	ModelClaudeSonnet = "claude-sonnet-5-5"
	ModelClaudeHaiku  = "claude-haiku-5-5"
	ModelClaudeFable  = "claude-fable-5-1"
	ModelGPTSol       = "gpt-6.1-sol"
	ModelGPTAstra     = "gpt-6-astra"
	ModelGrok         = "grok-4.7"
	ModelGrokPrior    = "grok-4.6"
	// ModelComposer is Cursor's own model. Cursor does not publish its base
	// model, so its family is unknown (Decision #469, detail 10): a review
	// between it and any other build is same-family (rung C).
	ModelComposer = "composer-2.5"
	// ModelGemini is the ui-review lane's model, called at high reasoning
	// effort (founder: "aayu to default to 3.8-flash-high gemini model for all
	// UI/UX review"; the gateway has no gemini-3.8-flash-high ID, so "-high" is
	// the effort). It is interim (D-6.6b): the target is gemini-4-argon-high,
	// which needs the founder's paid Google plan before the gateway routes it.
	ModelGemini = "gemini-3.8-flash"
)

// Model IDs as GitHub Copilot lists them (GET https://api.githubcopilot.com/models,
// read 2026-10-18): Copilot names Claude models with a dot, the gateway with a dash.
const (
	CopilotClaudeOpus   = "claude-opus-5.5"
	CopilotClaudeSonnet = "claude-sonnet-5.5"
	CopilotGPTSol       = "gpt-6.1-sol"
	CopilotGeminiArgon  = "gemini-4-argon"
)

// PeerReviewCandidates are the review-lane models inspect's peer mode picks
// its second reviewer from, in order: the first whose family differs from the
// builder's and the first reviewer's (WP4.8b).
var PeerReviewCandidates = []string{ModelGPTSol, ModelGrok, ModelGemini, ModelClaudeOpus}

func builderDefaults() []ModelDefault {
	return []ModelDefault{
		{Host: HostAmp, Model: "Amp medium", Family: FamilyHostManaged, Escalation: "Amp high"},
		{Host: HostCursor, Model: "Grok 4.7", ID: ModelGrok, Family: FamilyXAI, Plan: PlanX},
		{Host: HostCursor, Model: "Claude Opus 5.5", ID: ModelClaudeOpus, Family: FamilyAnthropic, Alternative: "Claude Sonnet 5.5 (" + ModelClaudeSonnet + ")", AlternativeID: ModelClaudeSonnet, Plan: PlanLegacy},
		{Host: HostClaude, Model: "Claude Opus 5.5", ID: ModelClaudeOpus, Family: FamilyAnthropic, Alternative: "Claude Sonnet 5.5 (" + ModelClaudeSonnet + ")", AlternativeID: ModelClaudeSonnet},
		{Host: HostCodex, Model: "GPT-6.1 Sol", ID: ModelGPTSol, Family: FamilyOpenAI, Escalation: "GPT-6 Astra at high reasoning effort", EscalationID: ModelGPTAstra, EscalationEffort: "high"},
		// Copilot builds with a named model, never an Auto tier: Balance and
		// Intelligence picked claude-opus-5.5 for a hard prompt, at twice
		// Sonnet's per-token price, without saying so beforehand.
		{Host: HostCopilot, Model: "Claude Sonnet 5.5", ID: CopilotClaudeSonnet, Family: FamilyAnthropic,
			Escalation: "Claude Opus 5.5", EscalationID: CopilotClaudeOpus},
	}
}

// Copilot's Auto tiers. aayu cannot pick a model inside Copilot, so a lane on
// Copilot is the Auto tier the person or the session selects (/model auto
// <tier> in Copilot CLI, Optimize for in VS Code), and the family is the
// model Copilot reports for each response.
const (
	CopilotEfficiency   = "Copilot Auto (Efficiency)"
	CopilotBalance      = "Copilot Auto (Balance)"
	CopilotIntelligence = "Copilot Auto (Intelligence)"
)

// Copilot model selectors: what [models.<lane>.copilot] model may name
// besides a concrete Copilot model ID. Each is the Copilot CLI flag pair
// --model auto --auto-tier <tier>, or --experimental --model hydrafusion for
// HydraFusion, a research preview and so only ever opt-in
// (docs.github.com/en/copilot/reference/copilot-cli-reference/cli-command-reference).
const (
	CopilotAutoEfficiency   = "auto:efficiency"
	CopilotAutoBalance      = "auto:balance"
	CopilotAutoIntelligence = "auto:intelligence"
	CopilotHydraFusion      = "hydrafusion"
)

func copilotAuto(tier, escalation string) ModelDefault {
	return ModelDefault{Host: HostCopilot, Model: tier, ID: copilotTierID[tier], Family: FamilyHostManaged, Escalation: escalation}
}

var copilotTierID = map[string]string{CopilotEfficiency: CopilotAutoEfficiency, CopilotBalance: CopilotAutoBalance, CopilotIntelligence: CopilotAutoIntelligence}

func reviewDefault(h HostID) ModelDefault {
	return ModelDefault{Host: h, Model: "GPT-6.1 Sol", ID: ModelGPTSol, Family: FamilyOpenAI,
		Escalation: "GPT-6 Astra at high reasoning effort for heavy reviews", EscalationID: ModelGPTAstra, EscalationEffort: "high"}
}

func withPlan(d ModelDefault, plan string) ModelDefault {
	d.Plan = plan
	return d
}

func geminiDefault(h HostID) ModelDefault {
	return ModelDefault{Host: h, Model: "Gemini 3.8 Flash (high)", ID: ModelGemini, Family: FamilyGoogle, Effort: "high",
		Alternative: "Fable 5.1 (" + ModelClaudeFable + "; anthropic family, so only when the builder is not anthropic)"}
}

// Lanes lists the model lanes with their settled per-host defaults.
var Lanes = []Lane{
	{ID: LaneBuild, Summary: "Writes code and artifacts.", Defaults: builderDefaults()},
	{ID: LaneSpec, Summary: "Authors specs, decisions, briefs, and copy.", Defaults: builderDefaults()},
	{ID: LanePlan, Summary: "Slices and plans work.", Defaults: builderDefaults()},
	{ID: LaneMission, Summary: "Runs a Feature's mission: starts its stories and follows its milestones.", Defaults: builderDefaults()},
	{ID: LaneReview, Summary: "Adversarial code and spec review.", Adversarial: true, Defaults: []ModelDefault{
		reviewDefault(HostAmp),
		// On a new plan the builder is Grok 4.7, and no other family is
		// included: another tier of the family reviews, labelled same-family
		// (Decision #469, detail 7).
		{Host: HostCursor, Model: "Grok 4.6", ID: ModelGrokPrior, Family: FamilyXAI, Plan: PlanX},
		{Host: HostCursor, Model: "Grok 4.7", ID: ModelGrok, Family: FamilyXAI, Plan: PlanLegacy},
		reviewDefault(HostClaude),
		// Codex builds with OpenAI, so its reviewer is another family.
		{Host: HostCodex, Model: "Claude Opus 5.5", ID: ModelClaudeOpus, Family: FamilyAnthropic, Alternative: "Grok 4.7 (" + ModelGrok + ")", AlternativeID: ModelGrok},
		// Copilot reviews with a concrete model (--model) from a family the
		// builder was not, read from its Aayudha-Model lines: GPT-6.1 Sol on
		// the default Anthropic build, Claude Opus 5.5 on one with no Anthropic
		// response, Gemini 4 Argon on one that used both. A build that used all
		// three is reviewed same-family. HydraFusion Critique stays opt-in
		// ([models.review.copilot]).
		{Host: HostCopilot, Model: "GPT-6.1 Sol", ID: CopilotGPTSol, Family: FamilyOpenAI,
			Alternative: "Claude Opus 5.5 (" + CopilotClaudeOpus + ") when the build has no anthropic response, Gemini 4 Argon (" + CopilotGeminiArgon + ") when it used both anthropic and openai", AlternativeID: CopilotClaudeOpus},
	}},
	// Interim model gemini-3.8-flash; the target is gemini-4-argon-high once the
	// gateway routes it (ModelGemini, D-6.6b).
	{ID: LaneUIReview, Summary: "Visual and interaction critique of UI.", Adversarial: true, Defaults: []ModelDefault{
		{Host: HostAmp, Model: "Amp high", Family: FamilyHostManaged},
		{Host: HostCursor, Model: "Grok 4.6", ID: ModelGrokPrior, Family: FamilyXAI, Plan: PlanX},
		withPlan(geminiDefault(HostCursor), PlanLegacy),
		geminiDefault(HostClaude),
		geminiDefault(HostCodex),
		// The founder's default: Gemini 4 Argon through Copilot wherever it is
		// available. The binary's lane A calls run it through Copilot CLI on
		// any host when copilot is installed and signed in, else Gemini 3.8
		// Flash (high) through the gateway, which the ux evals gate.
		{Host: HostCopilot, Model: "Gemini 4 Argon (high)", ID: CopilotGeminiArgon, Family: FamilyGoogle, Effort: "high",
			Alternative: "Gemini 3.8 Flash (high) (" + ModelGemini + ") through the AI gateway when copilot is not installed or signed in, a call fails, or [models.ui-review.copilot] model = \"\"; Argon costs about 0.043 USD per lane A call (Copilot AI credits: input x 2 USD/M + visible output x 10 USD/M; reasoning not billed)", AlternativeID: ModelGemini},
	}},
	{ID: LaneChore, Summary: "Cheap mechanical work: triage, status, checks.", Defaults: []ModelDefault{
		{Host: HostAmp, Model: "Amp low", Family: FamilyHostManaged},
		{Host: HostCursor, Model: "Composer 2.5", ID: ModelComposer, Note: "family unknown: Cursor does not publish Composer's base model", Plan: PlanX},
		{Host: HostCursor, Model: "Haiku 5.5", ID: ModelClaudeHaiku, Family: FamilyAnthropic, ContextTokens: 100000, Plan: PlanLegacy},
		{Host: HostClaude, Model: "Haiku 5.5", ID: ModelClaudeHaiku, Family: FamilyAnthropic, ContextTokens: 100000},
		{Host: HostCodex, Model: "Haiku 5.5", ID: ModelClaudeHaiku, Family: FamilyAnthropic, ContextTokens: 100000},
		copilotAuto(CopilotEfficiency, ""),
	}},
}

// DirectHost is the host whose defaults the binary uses when it calls a model
// itself (ground rule 5: inspect, review lane A, eval judge, brief adversarial
// pass, triage in Actions) and the current host's default is a host-managed mode.
const DirectHost = HostClaude

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
	GateStableRequest GateID = "stable-request"
	GateScopeBudget   GateID = "scope-budget"
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
	{GateStableRequest, "Promoting to main needs a person's stable-release request: their comment \"Stable release: <version>\" on the Release issue after its rolling snapshot, or their yes at a terminal."},
	{GateScopeBudget, "After a person accepts a release's baseline, drift stays within [scope] budget_percent, or a person accepts the change through GitHub's device flow against an accepted Decision."},
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

func humanOnly(in Input) Input { in.HumanOnly = true; return in }

func required(in Input) Input { in.Required = true; return in }

func readOnlyWith(in Input, values ...string) Input { in.ReadOnlyValues = values; return in }

func enumIn(name string, required, positional bool, desc string, values ...string) Input {
	return Input{Name: name, Type: TypeEnum, Required: required, Positional: positional, Enum: values, Description: desc}
}

// Verbs lists the 32 verbs in plan order (V7, WP0.3), then chart, the Data & AI
// Architect's verb (Decision #283, Appendix B S-5.1g), then survey (WP5.9,
// Decision #287), then the Production Engineer's verbs (WP5.10, Decision #324); brain (v0.3.0) follows triage.
var Verbs = []Command{
	{
		Name: "grill", WP: "WP5.3", Stage: StageShape,
		Summary: "Interview a human until a work item of the given type is complete and passes validation, then write it as an issue.",
		Reader:  "Interviews you, one question at a time, until an idea is a complete work item, then files it on GitHub.",
		Seat:    ProductStrategist, Lane: LaneSpec, Layers: AllLayers(),
		Inputs: []Input{
			enumIn("type", true, true, "Work-item type (internal/workitems), or brand for design/brand.yaml.", "goal", "initiative", "feature", "story", "bug", "decision", "experiment", "release", "feedback", "incident", "brand"),
			flag("parent", TypeIssue, "Parent issue to link the new work item under."),
			flag("title", TypeString, "Title of the drafted work item."),
			flag("body", TypeString, "Drafted issue body in the issue-form layout (### <Field> sections), or the brand YAML for brand."),
			flag("labels", TypeString, "Comma-separated label-held fields, e.g. effort:S,priority:P2."),
			flag("idea", TypeString, "The request in the human's own words. With --assess, the idea to assess; with a draft, the request its Provenance must quote and resolve."),
			flag("assess", TypeBool, "Read-only fact pass for the idea: the vagueness gate (unpack or pass, with its reasons), its vague qualifiers, abbreviations, and missing outcome with the dimensions to ask about, the product types, and the routes, states, components, tokens, tests, events, related stories, floors, and layers it touches (repository content is reported as untrusted data)."),
			readOnlyWith(flag("create", TypeBool, "Write the validated draft: the issue (through the actions log), or design/brand.yaml; without it grill only validates."), "false"),
		},
		Outputs:  []string{"issue", "design/brand.yaml", "assessment"},
		Gates:    []GateID{GateValidator, GateBaseline, GateHalt},
		Surfaces: cliSkillMCP, Identity: IdentityBot, Mode: ModeHost, Exposure: ExposeRead,
	},
	{
		Name: "slice", WP: "WP5.4", Stage: StageBreakDown,
		Summary: "Split a Feature into XS-L stories with observable criteria, dependencies, and a layer footprint; close XL items as split.",
		Reader:  "Splits a Feature into small stories, each with checkable promises, the parts of the code it touches, and its order.",
		Seat:    ProductStrategist, Lane: LanePlan, Layers: AllLayers(),
		Inputs: []Input{
			pos("feature", TypeIssue, "Feature (or XL story) to slice."),
			flag("draft", TypePath, "JSON file with the stories the host's model sliced (evals/schemas/slice.json); without it slice reports the footprint to slice within."),
			readOnlyWith(flag("create", TypeBool, "Create the validated stories under the Feature (through the actions log) and close an XL story as split; without it slice only validates."), "false"),
		},
		Outputs:  []string{"stories", "footprint"},
		Gates:    []GateID{GateValidator, GateHalt},
		Surfaces: cliSkillMCP, Identity: IdentityBot, Mode: ModeHost, Exposure: ExposeRead,
	},
	{
		Name: "plan", WP: "WP5.4", Stage: StageBreakDown,
		Summary: "Plan a story: one checkpoint per layer, contracts, tests, instrumentation, and rollback.",
		Reader:  "Plans a story one layer at a time: what changes, how it is tested, and how to roll it back.",
		Seat:    CodeArchitect, Lane: LanePlan, Layers: AllLayers(),
		Inputs: []Input{
			pos("story", TypeIssue, "Story or bug to plan, or a Feature to write its technical design for."),
			flag("draft", TypePath, "JSON file with the plan the host's model wrote (evals/schemas/plan.json), or a Feature's technical design (contracts, layers, story order, rollback); without it plan reports what to plan."),
			readOnlyWith(flag("create", TypeBool, "Write the validated plan to the ledger (specs/<story>/plan.json) and the story, or a Feature's design (specs/<feature>/design.json); without it plan only validates."), "false"),
			humanOnly(flag("approve", TypeBool, "Record the Engineering Manager's plan-time review of a plan that touches an [autonomy] never layer (human identity only).")),
			humanOnly(flag("solo-bypass", TypeBool, "Solo mode only (one human, no bot App): record that a never-layer plan proceeds with no plan-time review, after your story comment starting \"Solo-mode bypass:\"; a bypass, never an approval.")),
		},
		Outputs:  []string{"plan", "technical-design", "footprint", "em-review"},
		Gates:    []GateID{GateValidator, GateHalt},
		Surfaces: cliSkillMCP, Identity: IdentityBot, Mode: ModeHost, Exposure: ExposeRead,
	},
	{
		Name: "decide", WP: "WP5.6", Stage: StageBreakDown,
		Summary: "Record a Decision with options, evaluation, and recommendation; accept it, supersede it, or file a floor waiver.",
		Reader:  "Records a real choice with its options and a recommendation; only a person with the authority accepts it.",
		Seat:    Founder, Lane: LaneSpec, Layers: AllLayers(),
		Inputs: []Input{
			optPos("decision", TypeString, "Decision issue; without one, the SLA sweep over every open Decision. With --waiver, the story or bug to waive a floor on. With --pack, any issue, or a question in quotes."),
			humanOnly(flag("accept", TypeString, "Option to accept, by its number or its name (human identity only).")),
			flag("waiver", TypeString, "Floor to waive; creates a floor-waiver Decision."),
			readOnlyWith(flag("pack", TypeBool, "On demand, read-only: the owning seat's twin, its persona at the team stage, the cited facts, and the decision-pack outline (WP5.1b)."), "true"),
			flag("seat", TypeString, "With --pack: the seat whose twin works the decision instead of the one the topic picks."),
			flag("draft", TypeString, "With --pack: a pack draft (Markdown file) to check for every part and cited claims."),
		},
		Outputs:  []string{"decision", "waiver", "decision-pack"},
		Gates:    []GateID{GateAuthority, GateHumanIdentity},
		Surfaces: cliSkillMCP, Identity: IdentityHuman, Mode: ModeHost, Exposure: ExposeRead,
	},
	{
		Name: "counsel", WP: "WP5.6d", Stage: StageExplain,
		Summary: "Consult the Founder's Sage on any topic, issue, Decision, path, or PR: the call and its door, three readings, one recommendation, cited proof, and the next aayu step; advice only, never a review.",
		Reader:  "Asks the founder's sage for a steer on anything, with the evidence behind it and the next step; it advises and never approves or decides.",
		Seat:    Founder, Lane: LaneSpec, Layers: AllLayers(),
		Inputs: []Input{
			optPos("topic", TypeString, "A question in quotes, an issue or Decision as #N, or a path in this checkout; or use --pr or --issue."),
			flag("pr", TypePR, "Pull request to counsel on: its changed files by layer, its proof pack, and its builder family."),
			flag("issue", TypeIssue, "Issue or Decision to counsel on: it, its parent, and its links."),
			flag("draft", TypePath, "Markdown counsel the host's model wrote; counsel checks every part, the stage's artefact, and every citation. Without it, counsel prints the grounding and the outline."),
			readOnlyWith(flag("post", TypeBool, "Post the checked counsel on the PR or issue as one comment labelled counsel, replaced in place on a rerun (through the actions log); needs --draft and --pr or --issue. Never a review or an approval."), "false"),
		},
		Outputs:  []string{"counsel", "grounding"},
		Gates:    []GateID{GateHalt},
		Surfaces: cliSkillMCP, Identity: IdentityBot, Mode: ModeHost, Exposure: ExposeRead,
	},
	{
		Name: "triage", WP: "WP2.9", Stage: StageOperate,
		Summary: "Classify intake issues as Feedback, Bug, or Incident by label only; never edits bodies or touches non-intake issues.",
		Reader:  "Sorts incoming reports into feedback, bugs, and incidents by label, without editing what people wrote.",
		Seat:    ProductStrategist, Lane: LaneChore, Layers: NoLayers(),
		Inputs: []Input{
			optPos("issue", TypeIssue, "Intake issue to classify; without it, list the open intake issues and change nothing (the weekly triage loop)."),
			enumIn("as", false, false, "Class chosen by the host's model; without it the binary asks the triage lane's model.", "feedback", "bug", "incident"),
		},
		Outputs:  []string{"labels"},
		Gates:    []GateID{GateLabelsOnly, GateHalt},
		Surfaces: cliSkillMCP, Identity: IdentityBot, Mode: ModeHybrid, ModeNote: "the host's model passes --as; never in product CI", Exposure: ExposeWrite,
	},
	{
		Name: "brain", WP: "v0.3.0", Stage: StageExplain,
		Summary: "Keep the product brain: an Obsidian vault on the orphan brain branch; init, checkpoint its generated links, check, or open it.",
		Reader:  "Keeps the product's lasting knowledge as linked notes that link to issues and outlive any one person.",
		Seat:    ProductStrategist, Lane: LaneChore, Layers: NoLayers(),
		Inputs: []Input{
			readOnlyWith(enumIn("action", true, true, "init: create the brain branch and the vault; checkpoint: refresh the generated blocks from the repository's records and keep every word a person wrote; check: links, frontmatter, sources, STE sentence length, and no pasted Completion Goal line or issue label; open: print how to open the vault in Obsidian.", "init", "checkpoint", "check", "open"), "check", "open"),
			flag("workstream", TypeString, "With init: add a workstream folder with its own index and map of content, linked up into the root vault."),
			flag("dir", TypeString, "With check: check a local folder (a worktree of the brain branch) instead of the branch's tip on the remote."),
		},
		Outputs:  []string{"vault", "check-report"},
		Gates:    []GateID{GateHalt},
		Surfaces: cliSkillMCP, Identity: IdentityBot, Mode: ModeNone, Exposure: ExposeRead,
	},
	{
		Name: "build", WP: "WP5.5", Stage: StageBuild,
		Summary: "Build a Ready story or bug in a worktree with checkpoint commits per layer, verify each checkpoint, and open a PR.",
		Reader:  "Writes the code for a Ready story in small checked steps, one layer at a time, and opens a pull request.",
		Seat:    AINativeEngineer, Lane: LaneBuild, Layers: AllLayers(),
		Inputs: []Input{
			optPos("issue", TypeIssue, "Ready story or bug to build; required unless --next."),
			enumIn("next", false, false, "The build-ready loop, with no model: ship lists the loop's own open PRs, oldest first, and changes nothing; pick blocks the loop's stale claims, then, under [build] agents, claims the first Ready story or bug (the current release's accepted baseline in its order, else priority, then age) and cuts its worktree.", "ship", "pick"),
			flag("stop", TypeString, "Stop this build for good: move the story to status:blocked with this reason and page its owner; the lease stays until it expires."),
			flag("resume", TypeBool, "Resume from the last Aayudha-Checkpoint trailer."),
			enumIn("delegate", false, false, "Hand the story to another agent's cloud agent with the delegation brief, or sync it: its PR, inspect's findings, and follow-ups (auto picks the cheapest adequate connected one). OpenCode has no cloud agent, so it always runs locally.", "codex", "cursor", "claude", "amp", "opencode", "auto"),
			flag("local", TypeBool, "With --delegate: run the receiver's CLI on this machine in an aayu worktree, with no GitHub credential; aayu commits, verifies, pushes, and opens the PR."),
			flag("thread", TypeString, "With --delegate amp: record the Amp thread (T-…) that the host created for a host hand-off. Build prints a host hand-off in an Amp thread whose amp CLI is not signed in. Only an Amp thread ID is recorded."),
			flag("asked", TypeString, "The person's own words asking for this story. An agent needs them to pick up a Ready story that did not come out of grill in this session; without them build stops with a warning before taking the story."),
			flag("decline", TypeString, "Decline open items of the last session review, one \"<id>: <why>\" a line; the reason goes into the PR's session review summary, and the merge gate's inspect still judges them."),
		},
		Outputs:  []string{"pr", "checkpoint-commits", "progress-comment", "delegation"},
		Gates:    []GateID{GateReady, GateBudget, GateHalt},
		Surfaces: cliSkillMCP, Identity: IdentityBot, Mode: ModeHost, Exposure: ExposeWrite,
	},
	{
		Name: "verify", WP: "WP4.3", Stage: StageProve,
		Summary: "Run the deterministic floors (coverage, SAST, secrets, deps, a11y, perf) and record results in a proof pack.",
		Reader:  "Runs the fixed checks (tests, security, secrets, dependencies, accessibility, speed) and records the results.",
		Seat:    AINativeEngineer, Owns: QualityArchitect, Lane: LaneChore, Layers: AllLayers(),
		Inputs: []Input{
			optPos("pr", TypePR, "Pull request to verify; defaults to the current worktree."),
			flag("floor", TypeString, "Run only this floor."),
			flag("map", TypeBool, "Only the feature-map drift check (WP4.5b): compare every .agents/skills/verify-*/ map with the source, run nothing, and exit 3 on drift."),
			flag("check", TypeString, "Only the behaviour floor, on these feature-map checks: <feature>, <feature>.<check>, or all (the sweep); a run outside the merge gate is a preview."),
			enumIn("maintain", false, false, "With --map or --check: post the maintain brief on the tracking issue for the agent app [verify] maintainer names, when the map drifted or a check failed (drift), or every time (always).", "drift", "always"),
			flag("security", TypeBool, "Only the security floors (sast, secrets, deps, dast) on the checked-out commit, with no PR or story: what aayudha-security.yml runs on every push and nightly. With --ci, each failing finding files or updates one aayudha:security Bug."),
			flag("since", TypeString, "With --security: scan secrets only in the commits after this revision (a push's before); without it, or with all zeros or an unknown revision, the whole history of HEAD."),
		},
		Outputs:  []string{"floor-results", "pack", "map-drift", "security-bugs"},
		Gates:    []GateID{GateHalt},
		Surfaces: cliSkillMCP, Identity: IdentityBot, Mode: ModeNone, Exposure: ExposeWrite,
	},
	{
		Name: "inspect", WP: "WP4.8", Stage: StageProve,
		Summary: "Adversarially review a PR against its plan, pack, and criteria; return a structured disposition: merge, fix, or rebuild.",
		Reader:  "A model from another vendor, or an agent app you choose, reviews the pull request against every promise; aayu reads its reply as merge, fix, or rebuild.",
		Seat:    EngineeringManager, Lane: LaneReview, Layers: AllLayers(),
		Inputs: []Input{
			pos("pr", TypePR, "Pull request to inspect."),
			enumIn("mode", false, false, "general: one reviewer; peer: two reviewers of different families cross-check each other (default from [inspect]).", "general", "peer"),
			humanOnly(flag("dismiss", TypeString, "Record the finding with this ID as dismissed in this repository, so inspect stops raising findings like it.")),
			humanOnly(flag("approve", TypeBool, "As the [autonomy] approver, record your review of the PR's head on the ledger and post it as the human review the merge gate reads; a person-verified step, and the only way a human review counts. To approve several such PRs with one device code, use `aayu inbox`.")),
			humanOnly(flag("reason", TypeString, "Why the finding is dismissed (with --dismiss), or what you checked (with --approve).")),
		},
		Outputs:  []string{"disposition (merge|fix|rebuild)", "findings", "review-comments"},
		Gates:    []GateID{GatePack, GateBudget},
		Surfaces: cliSkillMCP, Identity: IdentityBot, Mode: ModeDirect, Exposure: ExposeWrite,
	},
	{
		Name: "ship", WP: "WP5.5", Stage: StageShip,
		Summary: "Squash-merge a PR after a green merge gate and a human pass, write the Complete section, and clean up.",
		Reader:  "Merges a proven pull request after your yes, and writes down who approved what, on which evidence.",
		Seat:    EngineeringManager, Lane: LaneChore, Layers: AllLayers(),
		Inputs: []Input{
			pos("pr", TypePR, "Pull request to ship."),
			humanOnly(flag("solo-bypass", TypeBool, "Solo mode only (one human, no bot App): merge without a second person's approval, recorded in the PR, the ship record, and the actions log.")),
		},
		Outputs:  []string{"merge-commit", "complete-section", "status:merged"},
		Gates:    []GateID{GateMergeGate, GateHumanPass},
		Surfaces: cliSkill, Identity: IdentityHuman, Mode: ModeNone, Exposure: ExposeNever,
	},
	{
		Name: "release", WP: "WP7.8", Stage: StageRelease,
		Summary: "Promote a Release along dev, rolling, main; stamp included stories released and trigger measurement.",
		Reader:  "Moves shipped work up to a snapshot or stable release, after a full gate, and schedules the measurement.",
		Seat:    DevOps, Lane: LaneChore, Layers: OnlyLayers(L09),
		Inputs: []Input{
			pos("release", TypeIssue, "Release issue."),
			enumIn("to", true, false, "Branch to promote to.", "rolling", "main"),
			flag("at", TypeString, "With --to rolling: promote this dev commit instead of dev's newest, such as the newest one with tier-2 eval results for every gating group."),
		},
		Outputs:  []string{"tag", "released-stories", "release-notes"},
		Gates:    []GateID{GateHumanIdentity, GateStableRequest, GateHalt},
		Surfaces: cliSkillMCP, Identity: IdentityHuman, Mode: ModeNone, Exposure: ExposeWrite,
	},
	{
		Name: "proof", WP: "WP4.2", Stage: StageProve,
		Summary: "Push, get, verify, or list proof packs on the ledger branch; land a merged PR's gate pack; comment is the merge gate's pack check.",
		Reader:  "Stores, fetches, and checks the proof packs that hold the evidence for each change.",
		Seat:    QualityArchitect, Lane: LaneChore, Layers: AllLayers(),
		Inputs: []Input{
			readOnlyWith(enumIn("action", true, true, "Proof operation: push an advisory pack, get, verify, or list packs; land: for a PR merged outside aayu ship, land the merge gate's own aayudha-pack artifact for the merged head as the authoritative pack, after the checks ship makes; or (merge gate only) comment: check the run's pack and keep its PR comment.", "push", "get", "verify", "list", "comment", "land"), "get", "verify", "list"),
			optPos("story", TypeIssue, "Story whose pack to operate on; with land, the merged pull request."),
			flag("media", TypeBool, "Include proof media; needs a bucket in [proof] media, since media never go to the ledger or Git LFS."),
		},
		Outputs:  []string{"pack", "verification-report"},
		Surfaces: cliSkillMCP, Identity: IdentityBot, Mode: ModeNone, Exposure: ExposeRead,
	},
	{
		Name: "ready", WP: "WP2.7", Stage: StageReady,
		Summary: "Validate a work item for Ready, label it status:ready, and post the Ready snapshot; --check is read-only.",
		Reader:  "Checks that a work item is complete and freezes exactly what was agreed, so later edits are visible.",
		Seat:    ProductStrategist, Lane: LaneChore, Layers: AllLayers(),
		Inputs: []Input{
			pos("issue", TypeIssue, "Work item to make Ready. On a Feature this is Approve to build: a person approves its baselines, stories, and budget through GitHub's device flow, and it is refused for an agent lane."),
			readOnlyWith(flag("check", TypeBool, "Validate only; write nothing."), "true"),
			readOnlyWith(flag("guard", TypeBool, "Ready guard (the aayudha-ready-guard workflow): strip status:ready when the issue no longer validates or drifted from its snapshot."), "false"),
			enumIn("part", false, false, "Feature part to baseline: a person approves its waiting draft, through GitHub's device flow every time; refused for an agent lane.", "intent", "spec", "tech", "ux"),
			flag("request", TypeBool, "With --part: draft the part's next baseline on the ledger (specs/<f>/baselines/<part>-v<k>.draft.json) and ask a person to approve it; never approves."),
			flag("changes", TypePath, "With --request: a JSON array with one entry per change since the last baseline (ref, what, why, need). A version after v1 needs what changed, why it matters, and the need behind it for each change."),
			flag("fixes", TypeString, "With --request: comma-separated IDs (<part>-v<k>/<seat>/<n>) of this part's review findings that the new version fixes; the draft's hash covers them."),
			flag("early-build", TypeIssue, "A child story of the Feature, labelled early-build, that a person approves to build before Approve to build, within [baselines] early_build_budget × [autonomy.budget] cost_usd; GitHub's device flow every time, refused for an agent lane. Its PR never merges into dev."),
			flag("milestones", TypeString, "With Approve to build: the mission's milestones in order, as 'MS1=20,21 MS2=22,23'. Each story and Bug of the Feature is in exactly one; with none, the last plan's milestones hold, or for a first plan one milestone MS1 holds them all. Written to missions/<f>/plan.json."),
			flag("agents", TypeInt, "With Approve to build: how many of the Feature's stories its mission runs at once, written to specs/<f>/approved.json. With none, the last approval's value holds, or for a first approval [build] agents on the integration branch applies."),
		},
		Outputs:  []string{"status:ready", "ready-snapshot", "feature-baseline", "early-build-approval"},
		Gates:    []GateID{GateValidator},
		Surfaces: cliSkillMCP, Identity: IdentityBot, Mode: ModeNone, Exposure: ExposeRead,
	},
	{
		Name: "mission", WP: "v0.4.0", Stage: StageBuild,
		Summary: "Start a Feature's approved mission, show its plan and clock, list what can start now, gate each milestone, stop, resume, end, extend, recover, retry, or wake it, or run the hourly tick.",
		Reader:  "Starts a Feature a person approved, shows its mission, says what can start next, checks each milestone, lets a person stop it, and wakes a missed Kanha.",
		Seat:    EngineeringManager, Lane: LaneMission, Layers: AllLayers(),
		Inputs: []Input{
			readOnlyWith(enumIn("action", true, true, "start: a person starts the approved mission, through GitHub's device flow, and the plan records them, their tool for this repository, and the budget, then Start begins the mission's Kanha on that tool; given a Tatkal Feature's story, it starts that story on the person's own tool; refused for an agent lane. show: print the plan, each number with its source. next: list the mission's stories and Bugs that can start now, in order (priority, slice order, Ready time), and why each other one waits, with no model call; it claims each start on the ledger (missions/<f>/claims.jsonl), so two runs never start one story, and only the mission's recorded tool runs it. It also lists every event since events_cursor in missions/<f>/plan.json (each story's close and each line of its delegation history), and moves the cursor once the list and its claims are written. With --json it is an aayudha.work/v1 list: {schema, scope {kind, id}, items [{key, action, reason, subthread}], events [{at, story, event}]}; once a mission's clock reaches [mission] max_hours its stories wait. stop: a person stops the mission (or one story or Bug with --story), through GitHub's device flow and from any tool: mission next starts nothing and ship merges none of its PRs; each running Amp, Claude Code, or Cursor thread is told to commit at its next checkpoint and stop, and a thread aayu cannot reach is named; claims, branches, and PRs stay, and other missions and loops run on. resume: a person on the mission's recorded tool continues it (or one story with --story) and starts stories up to its agents. Each control is recorded in the plan's controls; both are refused for an agent lane. gate: run the gate of each milestone whose last story merged on the dev tip (the required checks, aayu verify --check all, and the evals of the seats the milestone touched; no model decides), mark it passed or failed with the dev SHA in missions/<f>/plan.json, and list one fix per failed check, claimed in one agents slot; a check that failed for its runner reruns once; after [mission] max_no_go failed runs the mission stops and asks a person in aayu inbox. budget: a person raises the budget with a reason, or keeps a fix PR waiting. extend: a person, through GitHub's device flow, gives a mission that reached its time limit another [mission] max_hours, with --reason; refused for an agent lane. tick: the build-ready loop's hourly safety tick, with no model and no Feature. It wakes each started mission's cloud Kanha that has a free agent and a story that can start and was told nothing in the last hour, or a failed story whose retry came due since it was last told, at most once a tick, and records the wake in missions/<f>/plan.json. Then it syncs each open cloud pull request of Codex, Cursor, or Claude Code once with aayu build <n> --delegate <receiver>, and lists those at merge-ready for aayu ship. It never builds, claims, or starts a story. recover: on the mission's recorded tool, and on each Kanha wake, read the record and each host, never the Kanha's memory: re-attach each story whose thread lives, keep each story with a PR on it, and restart each story whose agent is gone with no PR from its branch and brief, under a claim line, so no story has two live agents; a host it cannot read marks its stories unreadable: <source> and keeps that source's cursor, and a start that got no answer stays maybe started until the host and GitHub show it did not start, or, on its first hand-off, the host lists the one task that holds it: recover re-attaches that task, and the next sync opens its PR once it is done; a failed step retries after [mission] retry_backoff_minutes, each later retry after twice the wait, up to [mission] retry_max retries, then waits on the person who started the mission; one line per story, and a run that changes nothing writes nothing. retry: a person on the mission's recorded tool runs one more attempt of a story's or Bug's failed step (--story), counted and costed in the plan; a story with a PR syncs it again and never reruns a judged review; refused for an agent lane. wake: stateless, for an event host or a timer: on the machine of a local Kanha (Codex, Copilot, OpenCode, or Cursor with no API key) it gives that Kanha the events since its last check, every [mission] check_minutes (aayu inbox does it at once as it opens); elsewhere, on the mission's tool with its cloud agent connected, it starts the next stories as the Kanha would and leaves events_cursor, and else, or when a start fails, it only records them in the plan's host_wakes for the Kanha.", "start", "show", "next", "gate", "budget", "stop", "resume", "extend", "tick", "recover", "retry", "wake"), "show"),
			optPos("feature", TypeIssue, "Feature whose mission to start, show, list, gate, budget, stop, resume, extend, recover, retry, or wake; with start, also a Tatkal Feature's story to start; tick takes none."),
			flag("story", TypeIssue, "With stop or resume: only this story or Bug of the mission; the rest runs on. With retry: the story or Bug whose failed step runs once more."),
			flag("reason", TypeString, "With stop: why, in one line; it is recorded and shown wherever the stop is. With budget --to: why the budget rises. With start: why the mission starts although its learned working hours pass [mission] max_hours (needed then). With extend: why it may run another [mission] max_hours (needed)."),
			flag("end", TypeBool, "With stop: end the mission, only by the person who started it. Merged stories stay merged; each open story and Bug loses its claims and goes back to status:ready, its open PRs close, the lock ends, and each locked issue gets one comment."),
			flag("pr", TypePR, "With gate: record this fix PR in the plan's gate_fixes, with the failed check its body names on a line `Aayudha-Gate-Fix: <milestone> <check>`; it files no issue. A fix that would pass the budget waits (exit 6), and an inbox card asks a person."),
			humanOnly(flag("once-more", TypeBool, "With gate: run the stopped gate once more, after the presence check; refused for an agent lane.")),
			humanOnly(flag("to", TypeInt, "With budget: the new budget in USD, after the presence check.")),
			humanOnly(flag("keep", TypePR, "With budget: keep this fix PR waiting, and close its inbox card.")),
		},
		Outputs:  []string{"mission-plan", "work-list"},
		Gates:    []GateID{GateHumanIdentity},
		Surfaces: cliSkillMCP, Identity: IdentityHuman, Mode: ModeNone, Exposure: ExposeRead,
	},
	{
		Name: "research", WP: "WP6.5", Stage: StageDesign,
		Summary: "Gather pinned design references (URLs and IDs only) and user research for a Feature.",
		Reader:  "Collects design references and user research for a Feature.",
		Seat:    UXResearcher, Lane: LaneSpec, Layers: OnlyLayers(L01),
		Inputs: []Input{
			pos("feature", TypeIssue, "Feature to research."),
			flag("from", TypePath, "Directory with stage.json: the references the host gathered (URLs and IDs only) and Markdown research notes."),
			flag("query", TypeString, "Search Mobbin screens for this; needs [integrations.mobbin]."),
			enumIn("platform", false, false, "Platform to search Mobbin on.", "ios", "android", "web"),
		},
		Outputs:  []string{"reference-set"},
		Gates:    []GateID{GateHalt},
		Surfaces: cliSkillMCP, Identity: IdentityBot, Mode: ModeHost, Exposure: ExposeRead,
	},
	{
		Name: "wireframe", WP: "WP6.5", Stage: StageDesign,
		Summary: "Produce wireframes for a Feature's states from its research and criteria.",
		Reader:  "Sketches the screens a Feature needs, from its research and promises.",
		Seat:    UXResearcher, Lane: LaneBuild, Layers: OnlyLayers(L01),
		Inputs: []Input{
			pos("feature", TypeIssue, "Feature to wireframe."),
			required(flag("from", TypePath, "Directory with stage.json and one wireframe per state (SVG, HTML, Markdown, or an image).")),
		},
		Outputs:  []string{"wireframes"},
		Gates:    []GateID{GateHalt, GateBudget},
		Surfaces: cliSkillMCP, Identity: IdentityBot, Mode: ModeHost, Exposure: ExposeWrite,
	},
	{
		Name: "mockup", WP: "WP6.5", Stage: StageDesign,
		Summary: "Produce design-system mockups for a Feature from its wireframes.",
		Reader:  "Turns wireframes into mockups that use your design system.",
		Seat:    UXResearcher, Lane: LaneBuild, Layers: OnlyLayers(L01),
		Inputs: []Input{
			pos("feature", TypeIssue, "Feature to mock up."),
			required(flag("from", TypePath, "Directory with stage.json and one SVG or HTML mockup per wireframe state, coloured only with design tokens.")),
		},
		Outputs:  []string{"mockups"},
		Gates:    []GateID{GateHalt, GateBudget},
		Surfaces: cliSkillMCP, Identity: IdentityBot, Mode: ModeHost, Exposure: ExposeWrite,
	},
	{
		Name: "prototype", WP: "WP6.5", Stage: StageDesign,
		Summary: "Build a flagged, working prototype of a Feature with Code Connect mapping and a Figma screenshot diff.",
		Reader:  "Builds a working prototype behind a flag and compares it with the design.",
		Seat:    UXResearcher, Lane: LaneBuild, Layers: OnlyLayers(L01, L02),
		Inputs: []Input{
			pos("feature", TypeIssue, "Feature to prototype."),
			required(flag("from", TypePath, "Directory with stage.json: the OpenFeature flag, Figma file, Code Connect sources, components, and each state's path and Figma frame.")),
			flag("captures", TypePath, "Diff these captures (captures.json with PNGs and computed styles) instead of rendering the states."),
		},
		Outputs:  []string{"prototype", "feature-flag", "figma-diff"},
		Gates:    []GateID{GateFigmaDiff, GateHalt, GateBudget},
		Surfaces: cliSkillMCP, Identity: IdentityBot, Mode: ModeHost, Exposure: ExposeWrite,
	},
	{
		Name: "review", WP: "WP6.6", Stage: StageDesign,
		Summary: "Critique UI with a model lane and a deterministic lane; dispositions recapture, rebuild, fix, or ship.",
		Reader:  "Critiques a pull request's screens with a model and with fixed accessibility checks.",
		Seat:    UXResearcher, Lane: LaneUIReview, Layers: OnlyLayers(L01),
		Inputs: []Input{
			pos("pr", TypePR, "Pull request whose UI to review."),
			flag("deep", TypeBool, "Run lane A at high effort when the lane's default effort is lower."),
		},
		Outputs:  []string{"critique", "disposition (recapture|rebuild|fix|ship)"},
		Gates:    []GateID{GateBudget},
		Surfaces: cliSkillMCP, Identity: IdentityBot, Mode: ModeHybrid, ModeNote: "lane A direct", Exposure: ExposeWrite,
	},
	{
		Name: "polish", WP: "WP6.6", Stage: StageDesign,
		Summary: "Apply a review critique to a PR's UI and re-run review.",
		Reader:  "Applies a design critique to a pull request and reviews it again.",
		Seat:    UXResearcher, Lane: LaneBuild, Layers: OnlyLayers(L01),
		Inputs:   []Input{pos("pr", TypePR, "Pull request to polish.")},
		Outputs:  []string{"commits"},
		Gates:    []GateID{GateHalt, GateBudget},
		Surfaces: cliSkillMCP, Identity: IdentityBot, Mode: ModeHost, Exposure: ExposeWrite,
	},
	{
		Name: "baseline", WP: "WP6.6", Stage: StageDesign,
		Summary: "Capture visual baselines for a commit SHA.",
		Reader:  "Captures reference screenshots of a commit to compare later changes against.",
		Seat:    UXResearcher, Lane: LaneChore, Layers: OnlyLayers(L01),
		Inputs:   []Input{flag("sha", TypeString, "Commit to baseline; defaults to HEAD.")},
		Outputs:  []string{"baseline-captures"},
		Gates:    []GateID{GateHalt},
		Surfaces: cliSkillMCP, Identity: IdentityBot, Mode: ModeNone, Exposure: ExposeWrite,
	},
	{
		Name: "experiment", WP: "WP6.7", Stage: StageLearn,
		Summary: "Design an experiment for a Feature: hypothesis, metric, flag, and minimum sample size.",
		Reader:  "Designs an A/B test for a Feature: what you expect, what you measure, and how many people you need.",
		Seat:    RevenueArchitect, Lane: LanePlan, Layers: OnlyLayers(L03, L08),
		Inputs: []Input{
			pos("feature", TypeIssue, "Feature (or Initiative) to experiment on."),
			required(flag("hypothesis", TypeString, "If we do X, the metric moves by Z because W.")),
			required(flag("metric", TypeString, "Conversion event the experiment moves (a PostHog event name).")),
			required(flag("flag", TypeString, "Feature-flag key that splits traffic: the PostHog flag key and GrowthBook tracking key.")),
			required(flag("mde", TypeString, "Minimum detectable effect: an absolute rate change (0.02) or a relative one (10%).")),
			flag("exposure", TypeString, "HogQL predicate for the population the baseline counts; default every person with an event."),
			flag("alpha", TypeString, "Two-sided significance level; default 0.05."),
			flag("power", TypeString, "Statistical power; default 0.8."),
			flag("days", TypeInt, "Baseline window in whole UTC days before today; default 28."),
		},
		Outputs:  []string{"experiment", "feature-flag"},
		Gates:    []GateID{GateBaseline, GateHalt},
		Surfaces: cliSkillMCP, Identity: IdentityBot, Mode: ModeHost, Exposure: ExposeWrite,
	},
	{
		Name: "readout", WP: "WP6.7", Stage: StageLearn,
		Summary: "Read out an experiment's lift once it reaches minimum sample and write the Result.",
		Reader:  "Reads out an experiment once it has enough people, and writes down the measured lift.",
		Seat:    RevenueArchitect, Lane: LaneSpec, Layers: OnlyLayers(L08),
		Inputs: []Input{
			pos("experiment", TypeIssue, "Experiment to read out."),
			enumIn("source", false, false, "Where the per-arm counts come from; default posthog.", "posthog", "growthbook"),
			flag("metric", TypeString, "GrowthBook metric ID (with --source growthbook); PostHog uses the Experiment's Metric."),
			flag("from", TypeString, "Start of the PostHog readout window (YYYY-MM-DD, UTC); default the Experiment's creation."),
		},
		Outputs:  []string{"result"},
		Gates:    []GateID{GateMinSample},
		Surfaces: cliSkillMCP, Identity: IdentityBot, Mode: ModeHost, Exposure: ExposeWrite,
	},
	{
		Name: "respond", WP: "WP5.6", Stage: StageOperate,
		Summary: "Respond to an incident: propose an ops catalog action, execute it only through human approval, keep the timeline.",
		Reader:  "Helps handle an incident: proposes a fix from your runbook, runs it only after a person approves, and keeps the timeline.",
		Seat:    ProductionEngineer, Lane: LaneBuild, Layers: OnlyLayers(L07, L09),
		Inputs: []Input{
			pos("incident", TypeIssue, "Incident issue."),
			flag("action", TypeString, "Catalog action to propose."),
			flag("params", TypeString, "Action parameters as key=value pairs separated by commas."),
			flag("postmortem", TypePath, "Blameless postmortem (Markdown) to check against the incident, its timeline, and its investigation note, and post on the incident (replaced in place)."),
			flag("close", TypeBool, "Close the incident: file its linked Bug (carrying the investigation note's implicated code and durable fix) and the retro entry; a Sev0 or Sev1 incident needs its postmortem first."),
		},
		Outputs:  []string{"timeline", "proposed-action", "postmortem", "linked-bug"},
		Gates:    []GateID{GateCatalog, GateEnvironment, GateHumanIdentity},
		Surfaces: cliSkill, Identity: IdentityHuman, Mode: ModeHost, Exposure: ExposeNever,
	},
	{
		Name: "status", WP: "WP2.8", Stage: StageExplain,
		Summary: "Roll up status, health, risk, proof coverage, decisions, and cost per Goal, Initiative, and Feature.",
		Reader:  "Rolls up progress, risk, proof, open decisions, and cost for each Goal, Initiative, and Feature.",
		Seat:    EngineeringManager, Lane: LaneChore, Layers: AllLayers(),
		Inputs: []Input{
			optPos("issue", TypeIssue, "Goal, Initiative, or Feature to roll up; defaults to all Goals."),
			readOnlyWith(flag("digest", TypeBool, "Send the digest through [notify]."), "false"),
			flag("all", TypeBool, "Iterate [fleet] repos."),
			readOnlyWith(flag("apply", TypeBool, "Post the replace-in-place Goal comments and close Features the parent-close rule allows; with --streak, record new corrections on the ledger."), "false"),
			flag("streak", TypeBool, "Compute the autonomy streak of every layer from merged-PR history instead of the rollups (WP5.7)."),
			flag("work", TypeBool, "List the work in flight instead: aayu worktrees (agent, story, branch, PR, age) and stories delegated to cloud agents (WP5.5b)."),
		},
		Outputs:  []string{"rollup", "goal-comment", "digest", "streaks", "work-in-flight"},
		Surfaces: cliSkillMCP, Identity: IdentityBot, Mode: ModeNone, Exposure: ExposeRead,
	},
	{
		Name: "audit", WP: "WP7.6", Stage: StageExplain,
		Summary: "Audit floors, proof coverage, waivers, decision SLAs, autonomy exceptions, and unlogged writes; write a reproducible report.",
		Reader:  "Writes a report anyone can reproduce: checks, proof, waivers, deadlines, bypasses, and any write that skipped the log.",
		Seat:    Auditor, Lane: LaneReview, Layers: AllLayers(),
		Inputs: []Input{
			flag("since", TypeString, "Window start (YYYY-MM-DD UTC or RFC 3339); default the cursor of the newest audit report on the ledger, else 7 days back."),
			flag("until", TypeString, "Window end and GitHub event cursor (RFC 3339, exclusive); default now. With --since and --ledger it regenerates a report."),
			flag("ledger", TypeString, "Ledger commit to read; default the ledger tip."),
			flag("out", TypePath, "Also write the report to this file."),
			flag("replay", TypeString, "Regenerate a report from the GitHub inputs snapshot at this ledger path (audit/<date>/…inputs.json); writes nothing."),
			flag("mark-merged", TypeString, "Move these stories (e.g. 38-50) that merged without proof, before or after the merge gate, to status:merged with a comment saying which; no report."),
		},
		Outputs:  []string{"audit-report"},
		Surfaces: cliSkillMCP, Identity: IdentityBot, Mode: ModeNone, Exposure: ExposeWrite,
	},
	{
		Name: "retro", WP: "WP7.6", Stage: StageLearn,
		Summary: "Compare estimates with observed delivery, corrections, floor misses, and cost; file findings as Feedback issues.",
		Reader:  "Compares plans with what happened and files what to improve as feedback you can accept or reject.",
		Seat:    EngineeringManager, Lane: LaneSpec, Layers: AllLayers(),
		Inputs: []Input{
			flag("since", TypeString, "Window start (YYYY-MM-DD UTC or RFC 3339); default the cursor of the newest retro report on the ledger, else 7 days back."),
			flag("until", TypeString, "Window end (RFC 3339, exclusive); default now."),
			flag("ledger", TypeString, "Ledger commit to read; default the ledger tip."),
			flag("out", TypePath, "Also write the report to this file."),
			flag("replay", TypeString, "Regenerate a report from the GitHub inputs snapshot at this ledger path (retro/<date>/…inputs.json); files nothing."),
			flag("classify", TypeIssue, "Feedback issue to classify by the highest layer that can fix its correction for good (needs --layer and --reason); writes no report."),
			enumIn("layer", false, false, "With --classify: codebase (a type, invariant, or boundary), check (a lint rule, test, or CI check), prose (skill or AGENTS.md guidance), or review (what is surfaced for human review).",
				"codebase", "check", "prose", "review"),
			flag("reason", TypeString, "With --classify: why this is the highest layer that can fix the correction for good."),
			flag("not-catchable", TypeString, "With --classify: why no type, invariant, lint rule, or test can catch it; required for prose and review, and lets evolve rewrite a skill for a codebase or check correction."),
			flag("criterion", TypeString, "With --classify and a layer other than prose: the Completion Goal criterion of the Story filed for it, `[evidence] state: …; expected: …`, whose proof is the new type, invariant, lint rule, test, or review surface."),
		},
		Outputs:  []string{"retro-report", "feedback-issues", "fix-layer"},
		Gates:    []GateID{GateHalt},
		Surfaces: cliSkillMCP, Identity: IdentityBot, Mode: ModeHost, Exposure: ExposeWrite,
	},
	{
		Name: "measure", WP: "WP7.2", Stage: StageLearn,
		Summary: "Measure a Goal or Initiative against its baseline from analytics exports or live cloud pulls and write the Result.",
		Reader:  "Measures a Goal against its baseline, the same way as before, and writes the result into the Goal.",
		Seat:    DataAIArchitect, Owns: RevenueArchitect, Lane: LaneSpec, Layers: OnlyLayers(L08),
		Inputs: []Input{
			optPos("issue", TypeIssue, "Goal or Initiative to measure; without it, only the cohorts and North Star are reported."),
			flag("metric", TypeString, "acquisition, activation, retention, referral, revenue, north_star, aayu:founder_minutes_per_pr (from the actions log on the ledger), or <source>:<metric> [filter]; default the issue's Metric."),
			flag("from", TypeString, "Window start (YYYY-MM-DD, UTC); default 28 days before --to."),
			flag("to", TypeString, "Window end, exclusive (YYYY-MM-DD, UTC); default today."),
			flag("billing", TypePath, "Billing export (CSV)."),
			flag("appinsights", TypePath, "Application Insights events export (JSON lines)."),
			flag("aws", TypePath, "AWS CloudWatch RUM events export (JSON lines)."),
			flag("gcp", TypePath, "Google Analytics 4 / Firebase BigQuery events export (JSON lines)."),
			flag("live", TypeString, "Pull the window's events live from configured cloud integrations instead of an export file: azure, aws, gcp, comma-separated."),
			enumIn("engine", false, false, "Query engine for the snapshot; duckdb is optional and shelled out.", "sqlite", "duckdb"),
		},
		Outputs:  []string{"result", "snapshot"},
		Gates:    []GateID{GateBaseline},
		Surfaces: cliSkillMCP, Identity: IdentityBot, Mode: ModeNone, Exposure: ExposeWrite,
	},
	{
		Name: "evolve", WP: "WP7.7", Stage: StageLearn,
		Summary: "Turn accepted retro Feedback into a failing eval case, then a skill diff that passes it, opened as an agent-authored PR.",
		Reader:  "Turns accepted feedback into a test for a skill, then a skill change that passes it, for a person to review.",
		Seat:    AINativeEngineer, Lane: LaneBuild, Layers: OnlyLayers(L05),
		Inputs: []Input{
			flag("skill", TypeString, "Skill to evolve: skills/<skill>/, or a stub under evals/skills/<skill>/."),
			flag("feedback", TypeIssue, "Accepted Feedback issue filed by `aayu retro` that the change answers; without it, list the accepted retro Feedback."),
		},
		Outputs:  []string{"eval-case", "skill-pr"},
		Gates:    []GateID{GateEvalFirst, GateCodeowner, GateHalt},
		Surfaces: cliSkillMCP, Identity: IdentityBot, Mode: ModeHost, Exposure: ExposeWrite,
	},
	{
		Name: "brief", WP: "WP7.3", Stage: StageExplain,
		Summary: "Explain work to an audience in a chosen register and purpose as a grounded, accessible HTML brief.",
		Reader:  "Explains a piece of work to one audience in a short page, where every claim is checked against its sources.",
		Seat:    Mentalist, Lane: LaneSpec, Layers: AllLayers(),
		Inputs: []Input{
			pos("subject", TypeIssue, "Work item to brief on."),
			enumIn("register", false, false, "Audience register.", "eli5", "ste", "engineer", "regulatory"),
			enumIn("purpose", false, false, "Purpose of the brief.", "explain", "ideate", "innovate", "close"),
			flag("draft", TypePath, "Markdown draft to check and render; without it, print the grounding to write from."),
		},
		Outputs:  []string{"html-brief"},
		Gates:    []GateID{GatePack, GateResult},
		Surfaces: cliSkillMCP, Identity: IdentityBot, Mode: ModeHybrid, ModeNote: "adversarial pass direct", Exposure: ExposeRead,
	},
	{
		Name: "tell", WP: "WP7.5", Stage: StageExplain,
		Summary: "Write docs, guides, changelogs, release notes, launch copy, or brand copy that cite only proven Features; publish is human-only.",
		Reader:  "Writes docs, release notes, and launch copy in your brand's voice, linking only work that is proven.",
		Seat:    Marketer, Lane: LaneSpec, Layers: OnlyLayers(L01),
		Inputs: []Input{
			enumIn("kind", true, true, "What to write.", "docs", "guide", "changelog", "release", "launch", "brand"),
			flag("from", TypePath, "Markdown draft the host wrote; tell checks its claims and tone and tags its links. Without it, tell prints the brief: tone, proven Features, and UTM tags."),
			flag("tag", TypeString, "Release tag the copy is about (release, changelog); publishing release notes also sets that GitHub Release's notes."),
			flag("campaign", TypeString, "UTM campaign; default the tag, else <kind>-<YYYY-MM-DD>."),
			flag("site", TypePath, "With docs: build the static docs site (README, getting started, verbs, skills, governance pages) into this new or empty directory; every page passes the claims check."),
			humanOnly(flag("publish", TypeBool, "Publish the checked copy to the ledger (and the tag's GitHub Release); human identity only, logged.")),
		},
		Outputs:  []string{"draft", "published-content"},
		Gates:    []GateID{GatePack, GateHumanIdentity},
		Surfaces: cliSkillMCP, Identity: IdentityHuman, Mode: ModeHost, ModeNote: "publish requires the human identity", Exposure: ExposeWrite,
	},
	{
		Name: "init", WP: "WP2.5", Stage: StageSetup,
		Summary: "Set up a product repo: labels, issue types, forms, config, ledger branch, and optionally projects, gates, identity, integrations.",
		Reader:  "Sets up a repository for aayudha: labels, issue forms, config, and the records branch; more on request.",
		Seat:    Founder, Lane: LaneChore, Layers: OnlyLayers(L09),
		Inputs: []Input{
			flag("resume", TypeBool, "Resume a partial run from .aayudha/state.json."),
			flag("projects", TypeBool, "Create Projects v2 views."),
			flag("gates", TypeBool, "Install gate workflows."),
			flag("identity", TypeBool, "Create the GitHub App bot identity."),
			flag("integrations", TypeBool, "Configure external integrations."),
			flag("verify-skill", TypeBool, "Write the product's verify skill: an empty feature map and a control tool in the product's stack."),
		},
		Outputs:  []string{"labels", "issue-types", "forms", "config", "ledger-branch", "state"},
		Gates:    []GateID{GateTTY},
		Surfaces: cliOnly, Identity: IdentityHuman, Mode: ModeNone, Exposure: ExposeNever,
	},
	{
		Name: "install", WP: "WP3.2", Stage: StageSetup,
		Summary: "Install skill copies, lock.toml, MCP registration, and hooks for each host; shows a diff unless --apply.",
		Reader:  "Installs the aayudha skills, tools, and hooks into your agent, and sets up its cloud environment and loops.",
		Seat:    DevOps, Lane: LaneChore, Layers: OnlyLayers(L09),
		Inputs: []Input{
			enumIn("host", false, false, "Host to install for.", "amp", "cursor", "claude", "codex", "copilot", "all"),
			flag("apply", TypeBool, "Write the changes instead of showing a diff."),
			flag("pin", TypeString, "Skill version to pin."),
			flag("env", TypeBool, "Set up agent environments instead: discover them from the code, ask what it cannot say, and show each host's generated configuration and UI values."),
			flag("verify", TypeBool, "With --env: write the files and verify each host's environment in a clean container (setup cold, setup warm, the build and tests), fixing and retrying."),
			flag("cloud", TypeBool, "With --env: prove the live cloud environment with a probe task where the host's CLI can (Amp, Codex) and record it."),
			flag("record", TypeString, "With --env --host: record a probe reply copied from the host's UI; with --loops --host: record loops as set up there, <loop>=<ID>[,…] or all=<ID> (an empty ID forgets one)."),
			flag("loops", TypeBool, "Set up this repository's scheduled loops (loops/*.toml) in the one agent tool you pick from those detected: Amp's schedules and Cursor's Terraform with --apply, a ready prompt per loop for Claude Code and Codex; warns when another tool already runs them."),
			humanOnly(flag("run-now", TypeString, "With --loops: test one loop now, once, with the tool's own CLI in this checkout.")),
		},
		Outputs:  []string{"skill-copies", "lock", "mcp-registration", "hooks", "agent-environments", "loops"},
		Gates:    []GateID{GateTTY},
		Surfaces: cliOnly, Identity: IdentityHuman, Mode: ModeNone, Exposure: ExposeNever,
	},
	{
		Name: "doctor", WP: "WP1.7", Stage: StageSetup,
		Summary: "Check tools, tokens, identity, config, host installs, and existing CI; --fix re-syncs copies after showing a diff.",
		Reader:  "Checks your machine and setup, and says exactly what to fix.",
		Seat:    DevOps, Lane: LaneChore, Layers: OnlyLayers(L09),
		Inputs: []Input{
			readOnlyWith(flag("fix", TypeBool, "Re-sync copies and adapters after showing a diff."), "false"),
			flag("all", TypeBool, "Iterate [fleet] repos."),
		},
		Outputs:  []string{"report"},
		Surfaces: cliSkillMCP, Identity: IdentityHuman, Mode: ModeNone, Exposure: ExposeRead, ReadOnly: true,
	},
	{
		Name: "chart", WP: "WP5.1g", Stage: StageBreakDown,
		Summary: "Chart an issue's data or AI design before planning: decisions, metrics, sources, platform, and retention; or evidence, permissions, abstention, and evals.",
		Reader:  "Before a data or AI change is planned, works out with you which decisions it serves, which data or evidence answers them, and how to prove it right.",
		Seat:    DataAIArchitect, Lane: LaneSpec, Layers: OnlyLayers(L05, L07, L08),
		Inputs: []Input{
			pos("issue", TypeIssue, "Feature, Initiative, Story, or Bug whose data or AI design to chart."),
			enumIn("kind", true, false, "data: decisions, metrics with grain and window, sources, platform and its trigger, retention and deletion; ai: simpler options, evidence and permissions, abstention, and an evaluation set.", "data", "ai"),
			flag("draft", TypePath, "Markdown chart note the host's model wrote; chart checks every part, the stage's artefact, and every citation. Without it, chart prints the grounding and the outline."),
			readOnlyWith(flag("create", TypeBool, "Post the checked note on the issue as one comment per kind, replaced in place on a rerun (through the actions log); needs --draft."), "false"),
		},
		Outputs:  []string{"chart-note", "grounding"},
		Gates:    []GateID{GateHalt},
		Surfaces: cliSkillMCP, Identity: IdentityBot, Mode: ModeHost, Exposure: ExposeRead,
	},
	{
		Name: "survey", WP: "WP5.9", Stage: StageShape,
		Summary: "Survey a codebase with no product docs: index it, check an evidence-backed analysis and backlog, and capture golden outputs for parity.",
		Reader:  "Reads a codebase nobody documented, says what it does with a citation for every claim, and drafts the backlog to modernise it.",
		Seat:    CodeArchitect, Lane: LanePlan, Layers: AllLayers(),
		Inputs: []Input{
			optPos("path", TypePath, "Checkout to survey; default the repository root."),
			flag("out", TypePath, "Directory for the survey's files (index.json, SURVEY.md, backlog.md, golden/); default .aayudha/survey under the surveyed checkout."),
			flag("budget", TypeInt, "Module size in tokens (about four bytes each) when grouping files; default 20000."),
			flag("module", TypeString, "Print one module's fact pack (files, targets, entry points, surfaces, markers, symbols, calls in and out) from the index; read-only."),
			flag("docs", TypePath, "User manuals and notes from outside the checkout (a file or a directory of text, Markdown, HTML, man pages, or PDF through pdftotext), indexed as manual:<name> documents a claim cites by line; their text is written under <out>/manuals."),
			flag("draft", TypePath, "Survey draft to check, whole or some of its sections (JSON: claims at the engineering, capability, and product levels, trap findings, the do-not-carry list, the compatibility contract, unknowns, answers, modernisation path): every claim cites the index, a manual, or a probe, or is marked unknown. Each section is saved with its findings under <out>/sections and resumed by later runs; with every section present it writes survey.json and renders SURVEY.md."),
			readOnlyWith(flag("sections", TypeBool, "Report the survey sections saved under <out>/sections: which pass, which have problems, and which are missing or stale."), "false"),
			flag("backlog", TypePath, "Backlog draft to check (JSON: Goal, Initiatives, Features, Stories, Bugs, and do-not-carry Decisions) against the work-item schema, the survey draft, and its compatibility contract; renders backlog.md and its review digest."),
			readOnlyWith(flag("create", TypeBool, "With --backlog: file the reviewed backlog as issues through the actions log, skipping items already filed; needs --reviewed."), "false"),
			flag("reviewed", TypeString, "With --create: the review digest of the exact backlog the person reviewed (printed by --backlog)."),
			flag("limit", TypeInt, "With --create: the most issues one run files; default 25."),
			flag("golden", TypePath, "Characterisation cases (JSON) to run against the legacy build (--bin), writing each case's stdout, stderr, and exit code with SHA-256 under golden/."),
			flag("parity", TypePath, "Characterisation cases (JSON) to replay against the new build (--bin) and compare with the golden outputs; exit 3 on any difference."),
			flag("compat", TypePath, "A legacy test suite as cases (JSON, each test with its suite and area) to run against the new build (--bin) after --golden ran it on the legacy build: scores passes per area, appends to compat-history.jsonl, exit 3 when a test the legacy build passes fails."),
			flag("bin", TypeString, "With --golden, --parity, or --compat: the program the cases run as {bin}."),
			readOnlyWith(flag("run", TypeBool, "With --golden, --parity, or --compat: run the cases' program; without it the cases file is only checked."), "false"),
			flag("shadow", TypeString, "With --golden --bin <legacy> --run: the new program, run beside the legacy one on the same inputs; reports every difference without golden files, exit 3 on any."),
			readOnlyWith(flag("probe", TypeBool, "With --golden --bin --run: also run each case twice and under a changed time zone, locale, and HOME (and under strace when installed), writing non-determinism, environment dependencies, and files read and written to probes.json."), "false"),
			flag("check-bin", TypeString, "With --probe: a sanitiser, checker, or differently optimised build of the legacy program; cases whose stderr carries a checker report, or whose stdout or exit code differs from the capture, are recorded."),
			flag("coverage", TypePath, "With --probe: an lcov tracefile from running the cases under coverage; records the index's functions the cases never reach."),
		},
		Outputs:  []string{"index", "fact-pack", "survey-report", "backlog", "issues", "golden", "parity", "compat", "shadow", "probes"},
		Gates:    []GateID{GateValidator, GateHalt},
		Surfaces: cliSkillMCP, Identity: IdentityBot, Mode: ModeHost, Exposure: ExposeRead,
	},
	{
		Name: "investigate", WP: "WP5.10", Stage: StageOperate,
		Summary: "Investigate an incident from bounded telemetry queries to the deployed revision and the implicated code, and check the note; it proposes a mitigation, never runs one.",
		Reader:  "Works out what went wrong in production from your telemetry and the code that was actually deployed, and suggests a fix for a person to approve.",
		Seat:    ProductionEngineer, Lane: LaneSpec, Layers: AllLayers(),
		Inputs: []Input{
			pos("incident", TypeIssue, "Incident issue."),
			flag("telemetry", TypePath, "Sanitised OpenTelemetry export directory: traces.jsonl (OTLP JSON), logs.jsonl (JSON lines with trace_id), deploys.jsonl (deploy markers); read with no network. Give it or --source."),
			enumIn("source", false, false, "Query the configured Sentry or PostHog project instead of an export, with a read-only key; needs --service. PostHog answers logs and exceptions only, and its exceptions are project-wide (error tracking has no service filter).", "sentry", "posthog"),
			flag("service", TypeString, "Service to scope every query to; default the only service in the export."),
			flag("environment", TypeString, "Deployment environment to scope to; default the deploy marker's."),
			flag("revision", TypeString, "Deployed revision; default the deploy marker serving the window, else service.version on the spans."),
			flag("from", TypeString, "Window start (RFC 3339, UTC); default an hour before the incident started, or with --source and no start, 24 hours before --to. The window is at most 7 days."),
			flag("to", TypeString, "Window end, exclusive (RFC 3339, UTC); default the export's last record, or now for --source."),
			flag("query", TypeString, "Run one more bounded query and log it as qN: <kind> [by f,…] [where k=v …] [limit N] [bucket 5m]."),
			flag("draft", TypePath, "Investigation note (Markdown) to check: every part, the stage's artefact, cited evidence, two hypotheses, bounded queries, and a mitigation only proposed from the catalog."),
			readOnlyWith(flag("create", TypeBool, "Post the checked note on the incident, replaced in place, and add a timeline line (through the actions log); needs --draft."), "false"),
		},
		Outputs:  []string{"investigation-note", "grounding", "query-log"},
		Gates:    []GateID{GateHalt},
		Surfaces: cliSkillMCP, Identity: IdentityBot, Mode: ModeHost, Exposure: ExposeRead,
	},
	{
		Name: "slo", WP: "WP5.10", Stage: StageOperate,
		Summary: "Define SLOs and error budgets from the user's side, measure them from a sanitised export, and record the budget that gates release.",
		Reader:  "Sets how reliable the product must be for its users, measures how much room for failure is left, and holds back risky releases when it runs out.",
		Seat:    ProductionEngineer, Lane: LaneSpec, Layers: AllLayers(),
		Inputs: []Input{
			flag("telemetry", TypePath, "Sanitised OpenTelemetry export directory to measure each SLO in ops/slos.toml from (traces.jsonl), read with no network."),
			flag("service", TypeString, "Service to scope the baseline queries to; default the only service in the export."),
			flag("query", TypeString, "Run one more bounded query and log it as qN: <kind> [by f,…] [where k=v …] [limit N]."),
			flag("draft", TypePath, "SLO document (Markdown) to check: every part, the stage's artefact, cited evidence, objectives as percentages with windows, a burn-rate alert, the budget policy, and a proposed ops/slos.toml in a toml block that parses."),
			readOnlyWith(flag("create", TypeBool, "Write the checked draft's toml block to ops/slos.toml in the checkout, for a pull request a person reviews; needs --draft."), "false"),
			readOnlyWith(flag("record", TypeBool, "Record each SLO's measure and budget on the ledger (slo/status.json), which release reads; needs --telemetry."), "false"),
		},
		Outputs:  []string{"slo-document", "slo-status", "query-log"},
		Gates:    []GateID{GateHalt},
		Surfaces: cliSkillMCP, Identity: IdentityBot, Mode: ModeHost, Exposure: ExposeRead,
	},
	{
		Name: "runbook", WP: "WP5.10", Stage: StageOperate,
		Summary: "Write and check runbooks: confirm a known failure with a bounded query, mitigate it only through the ops catalog, verify it, and roll it back.",
		Reader:  "Writes down how to spot and fix each known production problem with the approved actions, so whoever is on call can follow it.",
		Seat:    ProductionEngineer, Lane: LaneSpec, Layers: AllLayers(),
		Inputs: []Input{
			flag("draft", TypePath, "Runbook entry (Markdown) to check: every part, the stage's artefact, cited evidence, an SLO named, a bounded confirm query, a catalog action, its rollback, and no executed change."),
			readOnlyWith(flag("create", TypeBool, "Write the checked draft to ops/runbooks/<name>.md in the checkout, for a pull request a person reviews; needs --draft."), "false"),
		},
		Outputs:  []string{"runbook", "runbook-findings"},
		Gates:    []GateID{GateHalt},
		Surfaces: cliSkillMCP, Identity: IdentityBot, Mode: ModeHost, Exposure: ExposeRead,
	},
	{
		Name: "scope", WP: "v0.3.0", Stage: StageBreakDown,
		Summary: "Plan a release's scope in its version milestone, draft it, have a person baseline it, hand it off, assign Feedback, and hold drift to the change budget.",
		Reader:  "Plans what a release contains, freezes it once a person accepts it, and stops later changes beyond the budget unless a person decides.",
		Seat:    ProductStrategist, Lane: LanePlan, Layers: AllLayers(),
		Inputs: []Input{
			pos("release", TypeIssue, "Release issue (Version vX.Y.Z) whose scope to plan; without another flag, scope reports the plan, its points, and its drift."),
			flag("add", TypeIssue, "Initiative, Feature, Story, or Bug to add to the release's milestone; after a baseline it counts as drift."),
			flag("drop", TypeIssue, "Item to remove from the release's milestone, so it rides a later release; after a baseline it counts as drift."),
			flag("assign", TypeIssue, "Feedback issue to assign with --to; sets its milestone and horizon label, never its body, and is not drift."),
			enumIn("to", false, false, "With --assign: this release, the next release milestone, or the backlog.", "this", "next", "backlog"),
			flag("draft", TypeBool, "Snapshot the plan on the ledger (releases/<version>/draft.json) and ask a person to accept it; refuses an unsized, XL, or unsliced plan."),
			flag("decision", TypeIssue, "Accepted Decision that names the Release: with --add, --drop, or --draft over budget, it asks a person to accept the change; never applies it."),
			humanOnly(flag("accept", TypeBool, "Accept the Release's waiting request, a drafted baseline or a change over budget, through GitHub's device flow every time; aayu inbox shows the same card.")),
			flag("handoff", TypeBool, "Hand the baselined plan off: write the Release's includes links and post the plan on it."),
		},
		Outputs:  []string{"release-plan", "scope-draft", "scope-baseline", "scope-drift"},
		Gates:    []GateID{GateScopeBudget, GateHalt},
		Surfaces: cliSkillMCP, Identity: IdentityBot, Mode: ModeHost, Exposure: ExposeWrite,
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
	plumb("auth", "WP1.4", DevOps, IdentityHuman, "Show the human token and bot identity aayudha resolves, with scope warnings; set, delete, or list named secrets.",
		enumIn("action", false, true, "Auth operation (default status).", "status", "set-secret", "delete-secret", "list-secrets"),
		optPos("name", TypeString, "Secret name for set-secret and delete-secret; set-secret reads the value from stdin.")),
	plumb("actions", "WP1.5", DevOps, IdentityHuman, "Tail, search, or verify the hash chain of the actions log.",
		enumIn("action", true, true, "Actions-log operation.", "tail", "grep", "verify-chain"),
		optPos("pattern", TypeString, "Pattern for grep.")),
	plumb("models", "WP1.6", DevOps, IdentityHuman, "Show the resolved model for each lane and host.",
		enumIn("action", true, true, "Models operation.", "show")),
	plumb("mcp", "WP3.3", DevOps, IdentityBot, "Serve verbs as MCP tools over stdio.",
		flag("skills", TypeString, "Skills version of the host plugin starting the server; the server refuses to start when this binary carries another (WP3.2b).")),
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
		pos("name", TypeString, "Hook name."),
		flag("skills", TypeString, "Skills version of the host plugin running the hook; session-start reports a mismatch with this binary (WP3.2b)."),
		flag("event", TypeString, "The host's event as JSON, for a host that cannot send it on stdin (the Amp plugin).")),
	plumb("seat", "WP5.1", Founder, IdentityHuman, "Add, remove, or list seats and their paired humans.",
		enumIn("action", true, true, "Seat operation.", "add", "remove", "list"),
		optPos("seat", TypeString, "Seat slug."),
		flag("human", TypeString, "GitHub login of the paired human (add).")),
	plumb("lfs-agent", "0.2 (V6)", DevOps, IdentityHuman, "git-lfs custom transfer agent for S3/R2 storage."),
	// WP1.1b (founder request, Appendix B S-1.1b): the CLI experience. Running
	// `aayu` with no command in a checkout shows the home screen.
	plumb("next", "WP1.1b", EngineeringManager, IdentityHuman, "Print the single best next step for this repository: the exact command to run and why."),
	plumb("tour", "WP1.1b", Founder, IdentityHuman, "Guided first run: check this machine, then set the repository up at Level 1 in a few prompts; refused for agents."),
	// Story #431 (founder request): the founder's inbox.
	plumb("inbox", "WP5.6c", Founder, IdentityHuman, "Every human-only step waiting for you, one card at a time with the twin's counsel; Enter takes the recommendation and C clears the housekeeping. Refused for agents and needs a terminal, except --list.",
		flag("list", TypeBool, "Read-only: print what waits as the same cards, with the counsel and how a person acts on them, as Markdown (or JSON with --json); writes nothing, needs no terminal, and an agent may run it.")),
	plumb("completion", "WP1.1b", DevOps, IdentityHuman, "Print the shell completion script, generated from the registry, for bash, zsh, fish, or PowerShell.",
		enumIn("shell", true, true, "Shell to complete for.", "bash", "zsh", "fish", "powershell")),
}
