package config

import (
	"cmp"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
	"github.com/rahulnakmol/agentic-ayudha/internal/workitems"
)

// Version is the config schema version this binary reads and writes
// (`version` at the top of every config file). Older files are migrated on load;
// `aayu upgrade` (WP1.9) rewrites them on disk.
const Version = 1

// Config is the merged configuration. The JSON Schema in schema.json is
// generated from these structs (go generate ./...); unknown keys are rejected.
type Config struct {
	Version       int                    `koanf:"version" jsonschema_description:"Config schema version."`
	Seats         map[string]Seat        `koanf:"seats" jsonschema_description:"Seat modes keyed by seat slug (registry seats)."`
	Models        Models                 `koanf:"models" jsonschema_description:"Model overrides per lane and host, and the optional AI gateway."`
	LFS           LFS                    `koanf:"lfs"`
	Proof         Proof                  `koanf:"proof"`
	Beneficiaries map[string]string      `koanf:"beneficiaries" jsonschema_description:"Beneficiary groups a Goal can name, keyed by slug, with a one-line description."`
	Autonomy      Autonomy               `koanf:"autonomy"`
	Decisions     Decisions              `koanf:"decisions"`
	Notify        Notify                 `koanf:"notify"`
	Budget        Budget                 `koanf:"budget"`
	MCP           MCP                    `koanf:"mcp"`
	Integrations  map[string]Integration `koanf:"integrations" jsonschema_description:"External integrations keyed by name. Values name secrets; they never hold them."`
	Verify        Verify                 `koanf:"verify"`
	Time          Time                   `koanf:"time"`
	Identity      Identity               `koanf:"identity" jsonschema_description:"Bot identity for agent lanes and CI (WP1.4)."`
	Telemetry     Telemetry              `koanf:"telemetry" jsonschema_description:"Traces of verb runs (WP1.8): a local file by default; nothing leaves the machine unless otlp_endpoint is set."`
	Fleet         Fleet                  `koanf:"fleet" jsonschema_description:"User-level only: the product repos that status --all and doctor --all iterate."`
	Branches      Branches               `koanf:"branches" jsonschema_description:"The product's branches that play dev, rolling, and main when they have other names (written by init; nothing is renamed)."`
	Measure       Measure                `koanf:"measure" jsonschema_description:"The events measure (WP7.2) reads as AARRR stages and the North Star."`
	Design        Design                 `koanf:"design" jsonschema_description:"Where the design verbs (WP6.5) find the design tokens and the OpenFeature flag definitions."`
	Review        Review                 `koanf:"review" jsonschema_description:"Tolerances of the prototype's Figma screenshot diff (WP6.5) and UI review (WP6.6)."`
	Scope         Scope                  `koanf:"scope" jsonschema_description:"A release's change budget after its baseline, and the effort weights drift is measured in (aayu scope)."`
	Team          Team                   `koanf:"team" jsonschema_description:"The team the seat twins adapt their practice to (WP5.1b): depth and ceremony only, never the floors, the evidence rule, or human-only gates."`
	UI            UI                     `koanf:"ui" jsonschema_description:"How aayu looks on a person's terminal (WP1.1b); usually set in the user-level config. Machines (--json, MCP, agents, CI, pipes) always get plain, stable text."`
	Inspect       Inspect                `koanf:"inspect" jsonschema_description:"How inspect reviews a PR (WP4.8b): its mode, noise filters, and peer reviewer."`
	Delegate      Delegate               `koanf:"delegate" jsonschema_description:"How build --delegate reaches other agents (WP5.5b). Values name secrets; they never hold them."`
	Build         Build                  `koanf:"build" jsonschema_description:"The build-ready loop (aayu build --next): how many items it has in progress at once."`
	Hosts         Hosts                  `koanf:"hosts" jsonschema_description:"What the agent hosts include on the user's plan, which picks each lane's default model there (Decision #469)."`
	Approvals     Approvals              `koanf:"approvals" jsonschema_description:"Who approves agents' human-only steps by a comment, and which steps always need GitHub's device code (Decision #488)."`
}

// Hosts is [hosts] (Decision #469).
type Hosts struct {
	Cursor CursorHost `koanf:"cursor"`
}

// CursorHost is [hosts.cursor].
type CursorHost struct {
	Plan string `koanf:"plan" jsonschema:"enum=,enum=x,enum=legacy" jsonschema_description:"x (default): a Cursor plan from 12 October 2026 on, which includes only Cursor's own models, so the lanes default to Grok and Composer; legacy: a legacy Pro, Pro+, or Ultra plan, which includes Claude and Gemini. Read only from the user config, AAYU_HOSTS__CURSOR__PLAN, or --set, never from the checkout."`
}

// Approvals is [approvals] (Decision #488, Story #481).
type Approvals struct {
	Approvers []string `koanf:"approvers" jsonschema_description:"GitHub logins of the people whose /aayu approve reply on a batch counts as their approval. Their GitHub credentials must be in no agent's reach (aayu doctor checks); with none, every batch needs the device code."`
	DeviceFor []string `koanf:"device_for" jsonschema:"enum=one-way,enum=all" jsonschema_description:"Batches that always need the device code, even when comment approvals are on: one-way (any irreversible item: an irreversible Decision, a floor waiver) or all."`
}

// Build is [build]: the build-ready loop's limit.
type Build struct {
	InFlight int `koanf:"in_flight" jsonschema:"minimum=0" jsonschema_description:"Items the build-ready loop has in progress at once, from claim to merge: its open pull requests and its claims with no pull request yet. 0 stops its picks."`
}

// Delegate is [delegate] (WP5.5b).
type Delegate struct {
	CursorSecret        string   `koanf:"cursor_secret" jsonschema_description:"Name of the secret holding the Cursor API key (aayu auth set-secret)."`
	CursorAPI           string   `koanf:"cursor_api" jsonschema_description:"Base URL of Cursor's cloud agents API."`
	CursorModel         string   `koanf:"cursor_model" jsonschema_description:"Model ID the Cursor cloud agent runs; empty: Cursor's default (its family is then unknown to aayu, so inspect names concrete reviewer families)."`
	CodexEnv            string   `koanf:"codex_env" jsonschema_description:"ID of the Codex cloud environment whose Install script runs bash .agents/setup; empty: Codex runs only locally."`
	AmpOrbSize          string   `koanf:"amp_orb_size" jsonschema_description:"Orb size of a delegated Amp thread; empty: the Amp project's default."`
	ClaudeTrigger       string   `koanf:"claude_trigger" jsonschema_description:"Unused since WP5.5c: Claude Code is reached through its own CLI's cloud sessions (claude --cloud), not claude-code-action, so no AI key sits in the repository's CI. Kept so 0.1 configurations still load."`
	Checks              []string `koanf:"checks" jsonschema_description:"Commands the brief tells a receiver to run before it finishes; empty: the checks AGENTS.md lists."`
	LocalTimeoutMinutes int      `koanf:"local_timeout_minutes" jsonschema:"minimum=0" jsonschema_description:"Longest run of a receiver's CLI in an aayu worktree (--local)."`
}

// Inspect is [inspect] (WP4.8b).
type Inspect struct {
	Mode          string   `koanf:"mode" jsonschema:"enum=general,enum=peer" jsonschema_description:"general: one reviewer; peer: two reviewers of different families (both different from the builder's) cross-check each other's findings."`
	DelegatedMode string   `koanf:"delegated_mode" jsonschema:"enum=general,enum=peer" jsonschema_description:"Mode for a PR another agent built through build --delegate (WP5.5b)."`
	MinConfidence float64  `koanf:"min_confidence" jsonschema:"minimum=0,maximum=1" jsonschema_description:"P2 and P3 findings below this reviewer confidence are not posted; P0 and P1 always are."`
	SuppressAfter int      `koanf:"suppress_after" jsonschema:"minimum=0" jsonschema_description:"A finding resembling this many distinct human dismissals in the same file or directory is not posted (never a P0 in security, data, or concurrency); 0 turns suppression off."`
	PeerModel     string   `koanf:"peer_model" jsonschema_description:"Model ID of the peer reviewer; empty picks the first of the registry's review candidates whose family differs from the builder's and the first reviewer's."`
	Generated     []string `koanf:"generated" jsonschema_description:"Path patterns (path.Match) of generated files that inspect does not review; the drift check owns them. Files whose header says Code generated ... DO NOT EDIT are skipped too."`
	// The merge gate's reviewer (founder, 2026-10-18): the CI review by
	// default, with the code-review environment's key, the one AI key in a
	// product repository's CI.
	Reviewer    string            `koanf:"reviewer" jsonschema:"enum=ci,enum=codex,enum=coderabbit,enum=greptile,enum=claude,enum=cursor,enum=human,enum=model" jsonschema_description:"The one review tool of the merge gate. ci (the default): the merge gate calls the model the code-review environment names (variables AAYU_REVIEW_MODEL_URL and AAYU_REVIEW_MODEL, secret AAYU_REVIEW_MODEL_KEY) and fails closed until they are set; codex, coderabbit, greptile, claude, cursor: that installed app (a same-family app is a labelled rung-C review; CodeRabbit and Greptile count as rung A for any builder); human: a person; model: the review lane's model (aayudha's evals). When the chosen tool cannot review (a usage-limit or quota reply, or no reply within 30 minutes), the next installed app in the order codex, coderabbit, greptile, claude, cursor reviews, then ci, then a person."`
	Apps        []string          `koanf:"apps" jsonschema_description:"Agent apps installed on the repository's org or account that review pull requests: codex, coderabbit, greptile, claude, cursor. aayu doctor and aayu install detect them."`
	AppFamilies map[string]string `koanf:"app_families" jsonschema_description:"Model family of each reviewer app (anthropic, openai, google, xai). An app without one cannot review, since the different-family rule needs it; Cursor runs the model its user picks."`

	// PaidReviewer is rung B of the review ladder (Decision #469; founder,
	// 2026-10-18).
	PaidReviewer string `koanf:"paid_reviewer" jsonschema_description:"Rung B: the model ID (grok-4.7, gemini-3.8-flash, ...) the CI review calls when the code-review environment's AAYU_REVIEW_MODEL names none, and the fallback when no installed app can review the head. It must be of a family the builder cannot be; each PR's spend is capped by [budget] per_pr_inspect_max, and the review is labelled (B, paid)."`
	PaidEffort   string `koanf:"paid_effort" jsonschema:"enum=,enum=low,enum=medium,enum=high,enum=xhigh" jsonschema_description:"Reasoning effort every review model call is made at (low, medium, high, xhigh); empty: high, and medium for Claude Opus, Claude Fable, and GPT-6 Astra. Every review asks for fast (priority) serving."`
}

// Team stages (WP5.1b): how much ceremony the seat twins' practice carries.
const (
	StageStartup    = "startup"    // the one-page version: a short decision note, a two-line risk list
	StageScaleup    = "scaleup"    // the working version: a dated plan, owners, a weekly cadence
	StageEnterprise = "enterprise" // the full artefact: design doc, review board, control evidence
)

// TeamStages lists the stages from least to most ceremony.
var TeamStages = []string{StageStartup, StageScaleup, StageEnterprise}

// Team is [team] (WP5.1b).
type Team struct {
	Stage string `koanf:"stage" jsonschema:"enum=startup,enum=scaleup,enum=enterprise" jsonschema_description:"startup, scaleup, or enterprise. Unset: inferred from size (15 or fewer people startup, up to 150 scaleup, more enterprise)."`
	Size  int    `koanf:"size" jsonschema:"minimum=0" jsonschema_description:"People on the product team. Unset: the paired seat humans plus the founder."`
}

// TeamStage is the effective team stage and how it was decided: [team] stage
// when set, else from [team] size, else from the pod's humans (the paired
// seats plus the founder), so a solo pod is a startup.
func (c *Config) TeamStage() (stage, why string) {
	if c.Team.Stage != "" {
		return c.Team.Stage, "[team] stage"
	}
	size, why := c.Team.Size, "[team] size"
	if size <= 0 {
		size, why = 1, "the pod's humans"
		for name, s := range c.Seats {
			if s.Mode == "paired" && name != string(verbs.Founder) {
				size++
			}
		}
	}
	switch {
	case size <= 15:
		return StageStartup, fmt.Sprintf("%s (%d)", why, size)
	case size <= 150:
		return StageScaleup, fmt.Sprintf("%s (%d)", why, size)
	}
	return StageEnterprise, fmt.Sprintf("%s (%d)", why, size)
}

// UI is [ui] (WP1.1b).
type UI struct {
	Plain bool   `koanf:"plain" jsonschema_description:"Words only on a terminal: no colour, motion, emoji, or box drawing (screen-reader friendly); the same as --plain."`
	Theme string `koanf:"theme" jsonschema:"enum=auto,enum=saffron,enum=contrast,enum=mono,enum=nord,enum=catppuccin-mocha,enum=catppuccin-latte,enum=gruvbox-dark,enum=gruvbox-light,enum=dracula,enum=tokyonight,enum=solarized" jsonschema_description:"Colour theme: auto (the terminal's own 16 colours, so a themed terminal's palette and its light or dark background apply; default), saffron (warm true colour for dark terminals), contrast (bright and bold), mono (no hue), or a named palette for a terminal that is not themed: nord, catppuccin-mocha, catppuccin-latte, gruvbox-dark, gruvbox-light, dracula, tokyonight, or solarized (dark or light by the terminal's background)."`
	// Motion is nil (on) unless set; false keeps the inbox still.
	Motion *bool `koanf:"motion" jsonschema_description:"Animate the full-screen inbox (loading, the next card, the selection, results); default true. false keeps it still. --plain, AAYU_PLAIN=1, [ui] plain, TERM=dumb, and NO_COLOR are always still."`
}

// Design is [design] (WP6.5). Paths are relative to the repository root.
type Design struct {
	Tokens string `koanf:"tokens" jsonschema_description:"Design-token source (tokens.json in the W3C design-tokens format) whose colours and type scale mockups and prototypes must use."`
	Flags  string `koanf:"flags" jsonschema_description:"OpenFeature flag definitions (flagd JSON) that must declare each prototype's flag, off by default."`
}

// Review is [review] (WP6.5, WP6.6).
type Review struct {
	FigmaDiffMax float64  `koanf:"figma_diff_max" jsonschema:"minimum=0,maximum=100" jsonschema_description:"Largest share of pixels, in percent, a prototype screenshot may differ from its Figma frame after the anti-aliasing mask; colour and typography token mismatches must be zero regardless."`
	Baseline     []string `koanf:"baseline" jsonschema_description:"URL paths on the [verify.environments] target that baseline captures for each commit (WP6.6)."`
}

// Scope is [scope] (aayu scope).
type Scope struct {
	BudgetPercent float64 `koanf:"budget_percent" jsonschema:"minimum=0,maximum=100" jsonschema_description:"Drift from a release's accepted baseline, in percent of its effort points, that changes may reach without a person's accepted Decision."`
	EffortPoints  []int   `koanf:"effort_points" jsonschema:"minItems=5,maxItems=5" jsonschema_description:"Effort points, each at least 1, of XS, S, M, L, and XL, in that order: the weights of Stories and Bugs in a plan and its drift."`
}

// Measure is [measure]: which exported events mark each AARRR stage. Event
// names match the exports' event names exactly.
type Measure struct {
	Acquisition    string `koanf:"acquisition" jsonschema_description:"Event that acquires a user (e.g. signed_up); empty means a user's first event in the window."`
	Activation     string `koanf:"activation" jsonschema_description:"Event that activates an acquired user within activation_days."`
	ActivationDays int    `koanf:"activation_days" jsonschema_description:"Days after acquisition an activation counts; default 7."`
	Retention      string `koanf:"retention" jsonschema_description:"Event that counts as activity in days 7-13 after acquisition; empty means any event."`
	Referral       string `koanf:"referral" jsonschema_description:"Event by which an acquired user refers someone (e.g. invited_user)."`
	NorthStar      string `koanf:"north_star" jsonschema_description:"North Star: an event (distinct users who sent it in the window) or revenue (paid total in the window)."`
}

// Branches is [branches] (WP2.5 coexistence). Empty means the branch of the same name.
type Branches struct {
	Dev     string `koanf:"dev" jsonschema_description:"Integration branch; pull requests target it."`
	Rolling string `koanf:"rolling" jsonschema_description:"Canary snapshot branch; empty when the product has none."`
	Main    string `koanf:"main" jsonschema_description:"Stable release branch."`
}

// Names are the dev, rolling, and main branch names, role by role: an unset
// role keeps its default name, except rolling in a repository that maps dev
// (coexistence), where an unset rolling means the product has none ("").
func (b Branches) Names() [3]string {
	rolling := b.Rolling
	if rolling == "" && b.Dev == "" {
		rolling = "rolling"
	}
	return [3]string{cmp.Or(b.Dev, "dev"), rolling, cmp.Or(b.Main, "main")}
}

// Seat is [seats.<name>].
type Seat struct {
	Mode  string `koanf:"mode" jsonschema:"enum=solo,enum=paired" jsonschema_description:"solo: agent twin only; paired: agent twin plus a human."`
	Human string `koanf:"human" jsonschema_description:"GitHub login of the paired human."`
	Page  string `koanf:"page" jsonschema_description:"Where to page this seat (overrides [notify] target)."`
}

// Models is [models]: [models.<lane>.<host>] overrides plus [models.gateway].
type Models struct {
	Gateway Gateway `koanf:"gateway"`
	Prices  []Price `koanf:"prices" jsonschema_description:"USD per million tokens per model ID, for cost accounting and the [budget] breaker. A response that reports its own cost wins; an entry here overrides the model's built-in list price (models.ListPrices); a model with neither is charged at the conservative fallback rate, with a warning in eval results."`
	// Lanes holds [models.<lane>.<host>] keyed by lane, then host.
	Lanes map[string]map[string]ModelChoice `koanf:",remain"`
}

// Price is one [[models.prices]] entry.
type Price struct {
	Model         string  `koanf:"model"`
	InputPerMTok  float64 `koanf:"input_usd_per_mtok" jsonschema:"minimum=0"`
	OutputPerMTok float64 `koanf:"output_usd_per_mtok" jsonschema:"minimum=0"`
	// Cache reads and writes; 0 charges them at the input rate.
	CachedInputPerMTok float64 `koanf:"cached_input_usd_per_mtok" jsonschema:"minimum=0"`
	CacheWritePerMTok  float64 `koanf:"cache_write_usd_per_mtok" jsonschema:"minimum=0"`
}

// ModelChoice is [models.<lane>.<host>].
type ModelChoice struct {
	Model  string `koanf:"model" jsonschema_description:"Model ID as the provider or gateway names it."`
	Family string `koanf:"family" jsonschema:"enum=,enum=anthropic,enum=openai,enum=google,enum=xai,enum=host-managed" jsonschema_description:"Model family; inferred from the model ID when empty."`
}

// Gateway is [models.gateway]: when base_url is set, every lane's model calls go
// through this one endpoint (WP1.6); direct provider clients remain the fallback
// when it is unset. The wire format is chosen per model: a styles entry for the
// model ID, then one for its family, then api_style, then the family default (anthropic: messages,
// openai: responses, others: chat). The model family never depends on the endpoint.
type Gateway struct {
	BaseURL  string   `koanf:"base_url" jsonschema_description:"Gateway endpoint including the version path, e.g. https://gateway.example/v1; empty disables the gateway."`
	Secret   string   `koanf:"secret" jsonschema_description:"Name of the secret holding the gateway key (sent as Authorization: Bearer)."`
	APIStyle string   `koanf:"api_style" jsonschema:"enum=,enum=messages,enum=responses,enum=chat" jsonschema_description:"Wire format for every model without a styles entry: messages (Anthropic Messages), responses (OpenAI Responses), chat (OpenAI Chat Completions). Empty uses the family default."`
	Styles   []string `koanf:"styles" jsonschema_description:"Wire format overrides as \"<model-id or family>=<style>\", e.g. \"grok-4.7=chat\" (model IDs contain dots, so this is a list, not a table)."`
}

// LFS is [lfs]. 0.1 supports GitHub LFS only (V6).
type LFS struct {
	QuotaWarnPercent int `koanf:"quota_warn_percent" jsonschema:"minimum=1,maximum=100" jsonschema_description:"doctor warns when LFS storage or bandwidth use reaches this percentage."`
}

// Proof is [proof].
type Proof struct {
	Media         string `koanf:"media" jsonschema_description:"Optional private bucket that keeps proof media beyond the merge gate's run artifact (aayu ship copies them there), and holds design and UI baseline media on a public repo (V3): s3://bucket[/prefix][?region=..&endpoint=..], r2://<account>/bucket[/prefix], azblob://<account>/<container>[/prefix][?auth=sas|key|oidc|cli&tenant=..&client=..&subscription=..&group=..], or gs://<bucket>[/prefix][?auth=key|oidc|cli&provider=..&sa=..]."`
	MediaSecret   string `koanf:"media_secret" jsonschema_description:"Name of the secret holding the bucket's credential (ACCESS_KEY_ID:SECRET_ACCESS_KEY for s3 and r2; a SAS token or <client-id>:<client-secret> for azblob; a service account JSON key for gs; none with auth oidc or cli); empty means proof-media."`
	RetentionDays int    `koanf:"retention_days" jsonschema:"minimum=0" jsonschema_description:"Days the merge gate keeps its aayudha-pack run artifact, at most the repository's artifact retention limit; also the bucket's lifecycle rule. 0 keeps as long as the limit allows."`
}

// Autonomy is [autonomy].
type Autonomy struct {
	Streak   int            `koanf:"streak" jsonschema:"minimum=0" jsonschema_description:"Correction-free merges per layer footprint before agent lanes may self-ship."`
	Never    []string       `koanf:"never" jsonschema_description:"Layers (L01-L09) that never run autonomously."`
	Approver string         `koanf:"approver" jsonschema_description:"GitHub login that approves when autonomy does not apply."`
	Budget   AutonomyBudget `koanf:"budget"`
	// CorrectionDays is how long after a merge a linked Bug or Incident still
	// counts as a correction of that PR (WP5.7).
	CorrectionDays int `koanf:"correction_days" jsonschema:"minimum=0" jsonschema_description:"Days after a merge in which a linked Bug or Incident counts as a correction of the PR."`
	// Merge on proof (Decision #593): ship merges a PR into the integration
	// branch with no human pass outside the narrow set.
	Proof                bool     `koanf:"proof" jsonschema_description:"Merge a PR into the integration branch on proof alone: required checks green, the pack verified with its floors holding, inspect at merge with a proof_rungs rung, and nothing in the narrow set."`
	ProofRungs           []string `koanf:"proof_rungs" jsonschema_description:"Review rungs at the head (A, B, C) that let ship merge on proof."`
	AlwaysPerson         []string `koanf:"always_person" jsonschema_description:"The narrow set: paths (a trailing / for a directory, else a glob) whose change always needs a person per PR."`
	AlwaysPersonConfig   []string `koanf:"always_person_config" jsonschema_description:"TOML config files whose always_person_sections need a person when they change."`
	AlwaysPersonSections []string `koanf:"always_person_sections" jsonschema_description:"Config tables (by first key) that decide who may approve or bypass."`
}

// AutonomyBudget is [autonomy.budget], the per-story build budget (WP5.5).
type AutonomyBudget struct {
	WallHours float64 `koanf:"wall_hours" jsonschema:"minimum=0"`
	CostUSD   float64 `koanf:"cost_usd" jsonschema:"minimum=0"`
	Attempts  int     `koanf:"attempts" jsonschema:"minimum=0"`
}

// MinSoloProvisionalHours is the hard minimum of [decisions]
// solo_provisional_after_hours (Story #432): no configuration goes below it.
const MinSoloProvisionalHours = 12

// Decisions is [decisions]. SLA durations are business hours ([time]).
type Decisions struct {
	Authority        map[string]string `koanf:"authority" jsonschema_description:"Decision type or layer to the seat or login that may accept it."`
	SLAHours         int               `koanf:"sla_hours" jsonschema:"minimum=0" jsonschema_description:"Business hours before an open Decision pages its authority and escalates."`
	ProvisionalAfter int               `koanf:"provisional_after_hours" jsonschema:"minimum=0" jsonschema_description:"Business hours before a reversible Decision's recommendation applies provisionally (V8)."`
	SoloProvisional  int               `koanf:"solo_provisional_after_hours" jsonschema:"minimum=12" jsonschema_description:"In solo mode (at most one human), the shorter window before a reversible Decision's recommendation applies provisionally (V8); at least 12, and read only from .aayudha/config.toml."`
	EscalateTo       string            `koanf:"escalate_to" jsonschema_description:"GitHub login mentioned on escalation (founder B)."`
}

// Notify is [notify].
type Notify struct {
	Channel    string `koanf:"channel" jsonschema:"enum=,enum=slack,enum=email,enum=github,enum=file"`
	Target     string `koanf:"target" jsonschema_description:"slack: name of the secret holding the webhook URL; email: recipient address; github: login to mention; file: path of the JSON-lines outbox (default .aayudha/state/notify-outbox.jsonl), which the Actions loops keep as an artifact."`
	Issue      string `koanf:"issue" jsonschema_description:"github channel: issue the mention comment is posted on, as owner/name#number."`
	SMTPAddr   string `koanf:"smtp_addr" jsonschema_description:"email channel: SMTP server as host:port (STARTTLS)."`
	From       string `koanf:"from" jsonschema_description:"email channel: sender address, also the SMTP user."`
	SMTPSecret string `koanf:"smtp_secret" jsonschema_description:"email channel: name of the secret holding the SMTP password."`
}

// Budget is [budget].
type Budget struct {
	PerStoryUSD     float64 `koanf:"per_story_usd" jsonschema:"minimum=0"`
	PerDayUSD       float64 `koanf:"per_day_usd" jsonschema:"minimum=0"`
	PerPRInspectMax float64 `koanf:"per_pr_inspect_max" jsonschema:"minimum=0" jsonschema_description:"Model spend in USD for inspect on one PR across all its cycles."`
	CIMinutesMonth  int     `koanf:"ci_minutes_month" jsonschema:"minimum=0"`
}

// MCP is [mcp].
type MCP struct {
	ExposeWrites bool `koanf:"expose_writes" jsonschema_description:"Expose write verbs over MCP (WP3.3)."`
}

// Integration is [integrations.<name>].
type Integration struct {
	Secret string `koanf:"secret" jsonschema_description:"Name of the secret holding the key (WP1.3); never the key itself."`
	Scope  string `koanf:"scope" jsonschema:"enum=read,enum=write"`
	// Provider details the adapters need (WP5.2, WP6.4).
	URL       string `koanf:"url" jsonschema_description:"Provider base URL when not the adapter's default (EU region, self-hosted), e.g. https://eu.posthog.com."`
	Org       string `koanf:"org" jsonschema_description:"Provider organization: the Sentry organization slug, or the Microsoft Entra tenant ID (azure)."`
	Project   string `koanf:"project" jsonschema_description:"Provider project: the PostHog project ID, the Sentry project ID, the owner/name GitHub baselines read (default: the current repository), the Azure resource ID of the Log Analytics workspace or Application Insights component (azure), the CloudWatch RUM app monitor name (aws), or <project>.<dataset> of the GA4 BigQuery export (gcp)."`
	AllowPII  bool   `koanf:"allow_pii" jsonschema_description:"posthog, sentry: let an agent call the host MCP tools that return personal data (person records, replay content, raw LLM prompts, support tickets). Off by default (Decision #449)."`
	Transport string `koanf:"transport" jsonschema:"enum=rest,enum=mcp" jsonschema_description:"Figma only: rest (personal access token, the default) or mcp (the remote MCP server, opt-in; the secret is an OAuth access token)."`
	// Cloud integrations (azure, aws, gcp; WP7.2b).
	Auth       string `koanf:"auth" jsonschema:"enum=key,enum=oidc,enum=cli" jsonschema_description:"azure, aws, gcp: key (a credential in the secret, the default), oidc (GitHub Actions OIDC federation: short-lived tokens, no stored key), or cli (the az, aws, or gcloud login on this machine)."`
	Identity   string `koanf:"identity" jsonschema_description:"azure, aws, gcp with auth = \"oidc\": the Entra application (client) ID, the IAM role ARN, or the service account email to impersonate (optional for gcp)."`
	Federation string `koanf:"federation" jsonschema_description:"gcp with auth = \"oidc\": the workload identity provider, projects/<number>/locations/global/workloadIdentityPools/<pool>/providers/<provider>."`
	Region     string `koanf:"region" jsonschema_description:"aws: the region of the app monitor and log group, e.g. eu-west-1; gcp: the BigQuery location when it is not US or EU."`
}

// CloudIntegrations take auth, identity, federation, and region (WP7.2b).
var CloudIntegrations = []string{"azure", "aws", "gcp"}

// StoredKey reports whether the integration reads its credential from its
// secret: every integration but a cloud one with auth = "oidc" or "cli".
func (in Integration) StoredKey() bool { return in.Auth == "" || in.Auth == "key" }

// Verify is [verify].
type Verify struct {
	DAST           bool                   `koanf:"dast" jsonschema_description:"Run the ZAP floor for web and api platforms (Decision #485)."`
	Platforms      []string               `koanf:"platforms" jsonschema_description:"Platforms the product ships on, e.g. web, ios, android."`
	UpstreamChecks []string               `koanf:"upstream_checks" jsonschema_description:"Existing product CI check names the merge gate also requires."`
	Seed           []string               `koanf:"seed" jsonschema_description:"Command (argv) that loads seeded, synthetic data into the app before the web floors capture (WP4.5); the product provides it, e.g. under .aayudha/seed/."`
	Environments   map[string]Environment `koanf:"environments" jsonschema_description:"Apps the web, DAST, and API floors run against, by name; AAYU_ENV picks one (default local). A production environment is never captured."`
	Floors         Floors                 `koanf:"floors" jsonschema_description:"Floor thresholds; CONSTRAINTS.md documents each one."`
	// RedOnBaseBlockingFrom is the date (UTC, YYYY-MM-DD) from which a Bug
	// story's proving test that passes on the base fails the merge gate (P5,
	// founder decision 4); before it the red-on-base check is advisory.
	// DriftBlockingFrom is the date (UTC, YYYY-MM-DD) from which feature-map
	// drift fails the merge gate (WP4.5b, founder decision 2026-10-16).
	NewFloorsBlockingFrom string `koanf:"new_floors_blocking_from" jsonschema_description:"Date (UTC, YYYY-MM-DD) from which the floors Decision #480 added fail; before it, or empty, their misses are advisory."`
	DriftBlockingFrom     string `koanf:"drift_blocking_from" jsonschema_description:"Date (UTC, YYYY-MM-DD) from which feature-map drift fails the merge gate's feature-map floor; before it, or empty, drift is a warning."`
	RedOnBaseBlockingFrom string `koanf:"red_on_base_blocking_from" jsonschema_description:"Date (UTC, YYYY-MM-DD) from which a Bug story whose mapped proving test passes on the base commit fails the merge gate; before it, or empty, the red-on-base check is advisory."`
	Maintainer            string `koanf:"maintainer" jsonschema:"enum=amp,enum=claude,enum=codex,enum=cursor" jsonschema_description:"Agent app the feature maps' maintain brief @-mentions (WP4.5b.4): claude, codex, or cursor, installed on the repository; amp has no mention, so its scheduled loop thread reads the brief. Unset: the brief mentions no app."`
}

// Environment is [verify.environments.<name>].
type Environment struct {
	URL        string `koanf:"url" jsonschema_description:"Base URL of the running app."`
	Production bool   `koanf:"production" jsonschema_description:"A production environment: verify refuses to capture it (exit 4), and any target on its host."`
	ActiveScan bool   `koanf:"active_scan" jsonschema_description:"Let the DAST floor run its active rules against this URL although verify did not start the app on a loopback host (Decision #485); read only from .aayudha/config.toml."`
}

// Floors is [verify.floors]: the deterministic floors (rule 7).
type Floors struct {
	Coverage struct {
		MinDiffPercent float64  `koanf:"min_diff_percent" jsonschema:"minimum=0,maximum=100"`
		Command        []string `koanf:"command" jsonschema_description:"Test command that writes the coverage report, as argv; empty runs go test -coverprofile in a Go module."`
		Report         string   `koanf:"report" jsonschema_description:"Coverage report the command writes (lcov, Cobertura XML, or Go coverprofile), relative to the repository root."`
	} `koanf:"coverage"`
	SAST    MaxCriticalHigh `koanf:"sast"`
	Secrets struct {
		MaxFindings int `koanf:"max_findings" jsonschema:"minimum=0"`
	} `koanf:"secrets"`
	Deps MaxCriticalHigh `koanf:"deps"`
	A11y struct {
		Standard         string `koanf:"standard"`
		MaxCritical      int    `koanf:"max_critical" jsonschema:"minimum=0"`
		MaxSerious       int    `koanf:"max_serious" jsonschema:"minimum=0"`
		KeyboardOnly     bool   `koanf:"keyboard_only"`
		ReducedMotion    bool   `koanf:"reduced_motion"`
		TextScalePercent int    `koanf:"text_scale_percent" jsonschema:"minimum=100"`
		MinTargetPx      int    `koanf:"min_target_px" jsonschema:"minimum=0"`
	} `koanf:"a11y"`
	ScreenReader struct {
		Layers            []string `koanf:"layers"`
		MaxMissingPhrases int      `koanf:"max_missing_phrases" jsonschema:"minimum=0"`
	} `koanf:"screen_reader"`
	Perf struct {
		MinScore float64 `koanf:"min_score" jsonschema:"minimum=0,maximum=1"`
		MaxLCPMs float64 `koanf:"max_lcp_ms" jsonschema:"minimum=0"`
		MaxCLS   float64 `koanf:"max_cls" jsonschema:"minimum=0"`
		MaxTBTMs float64 `koanf:"max_tbt_ms" jsonschema:"minimum=0"`
	} `koanf:"perf" jsonschema_description:"Lighthouse budgets on every Completion Goal state (WP4.5)."`
	DAST struct {
		MaxHigh           int  `koanf:"max_high" jsonschema:"minimum=0"`
		SkippedBlocksShip bool `koanf:"skipped_blocks_ship"`
	} `koanf:"dast"`
	API struct {
		Script string `koanf:"script" jsonschema_description:"k6 script whose thresholds are the API and CLI contract (WP4.6); BASE_URL is the target environment's URL. Empty skips the floor."`
	} `koanf:"api"`
	Mobile struct {
		SkippedBlocksShip bool `koanf:"skipped_blocks_ship"`
	} `koanf:"mobile"`
}

// MaxCriticalHigh is a findings floor by severity.
type MaxCriticalHigh struct {
	MaxCritical int `koanf:"max_critical" jsonschema:"minimum=0"`
	MaxHigh     int `koanf:"max_high" jsonschema:"minimum=0"`
}

// Time is [time].
type Time struct {
	TZ            string `koanf:"tz" jsonschema_description:"IANA time zone for business hours."`
	BusinessHours string `koanf:"business_hours" jsonschema_description:"Business days and hours, e.g. Mon-Fri 09:00-18:00."`
}

// Identity is [identity] (WP1.4). With neither an App nor a machine user, every
// verb runs under the human identity and its writes are tagged identity:human-fallback.
type Identity struct {
	AppID             int64    `koanf:"app_id" jsonschema_description:"GitHub App ID of the bot identity (created by init --identity)."`
	AppSlug           string   `koanf:"app_slug" jsonschema_description:"GitHub App slug; the App's login is <slug>[bot]."`
	Broker            string   `koanf:"broker" jsonschema_description:"Token broker workflow file in this repo that mints App installation tokens for laptops."`
	MachineUser       string   `koanf:"machine_user" jsonschema_description:"Machine-user login, the fallback bot identity."`
	MachineUserSecret string   `koanf:"machine_user_secret" jsonschema_description:"Name of the secret holding the machine user's token."`
	BotLogins         []string `koanf:"bot_logins" jsonschema_description:"Logins whose reviews never count as a human pass (machine users and other bots)."`
	DeviceClientID    string   `koanf:"device_client_id" jsonschema_description:"Client ID (public) of the GitHub OAuth App or GitHub App, with Device Flow enabled, that a person approves human-only commands through from any device; unset, the repository variable AAYUDHA_DEVICE_CLIENT_ID or AAYUDHA_APP_CLIENT_ID."`
}

// Telemetry is [telemetry].
type Telemetry struct {
	Disabled     bool   `koanf:"disabled" jsonschema_description:"Write no traces at all."`
	OTLPEndpoint string `koanf:"otlp_endpoint" jsonschema_description:"Opt-in: also send traces as OTLP/HTTP JSON to this collector, e.g. https://collector.example:4318. Empty sends nothing."`
}

// Fleet is [fleet] (user-level config only).
type Fleet struct {
	Repos []string `koanf:"repos" jsonschema_description:"owner/name of each product repo."`
}

// Clock returns the business-hours clock for [time].
func (c *Config) Clock() (Clock, error) { return NewClock(c.Time.TZ, c.Time.BusinessHours) }

var (
	secretName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)
	repoName   = regexp.MustCompile(`^[A-Za-z0-9-]+/[A-Za-z0-9._-]+$`)
	loginName  = regexp.MustCompile(`^@?[A-Za-z0-9](?:[A-Za-z0-9-]{0,38})$`)
	issueRef   = regexp.MustCompile(`^[A-Za-z0-9-]+/[A-Za-z0-9._-]+#[1-9][0-9]*$`)
	apiStyles  = []string{"", "messages", "responses", "chat"}
	families   = []string{"", string(verbs.FamilyAnthropic), string(verbs.FamilyOpenAI), string(verbs.FamilyGoogle), string(verbs.FamilyXAI), string(verbs.FamilyHostManaged)}
)

// Validate checks values that the struct types alone cannot.
func (c *Config) Validate() error {
	var errs []error
	bad := func(format string, args ...any) { errs = append(errs, fmt.Errorf(format, args...)) }

	// Empty means advisory; a malformed date must not make the new floors advisory forever.
	if d := c.Verify.NewFloorsBlockingFrom; d != "" {
		if _, err := time.Parse(time.DateOnly, d); err != nil {
			bad("[verify] new_floors_blocking_from = %q is not a YYYY-MM-DD date", d)
		}
	}

	for name, s := range c.Seats {
		if _, ok := verbs.FindSeat(verbs.SeatID(name)); !ok {
			bad("[seats.%s]: unknown seat", name)
		}
		switch s.Mode {
		case "", "solo":
		case "paired":
			if s.Human == "" {
				bad("[seats.%s]: mode = \"paired\" needs human = \"<login>\"", name)
			}
		default:
			bad("[seats.%s] mode = %q: want solo or paired", name, s.Mode)
		}
	}
	for lane, hosts := range c.Models.Lanes {
		if _, ok := verbs.FindLane(verbs.LaneID(lane)); !ok {
			bad("[models.%s]: unknown lane", lane)
		}
		for host, m := range hosts {
			if _, ok := verbs.FindHost(verbs.HostID(host)); !ok {
				bad("[models.%s.%s]: unknown host", lane, host)
			}
			// model = "" on the ui-review lane's copilot row turns the binary's
			// Copilot route off; every other row names a model.
			if m.Model == "" && (lane != string(verbs.LaneUIReview) || host != string(verbs.HostCopilot)) {
				bad("[models.%s.%s]: model is required", lane, host)
			}
			if !slices.Contains(families, m.Family) {
				bad("[models.%s.%s] family = %q: want one of %s", lane, host, m.Family, strings.Join(families[1:], ", "))
			}
		}
	}
	if g := c.Models.Gateway; g.BaseURL != "" {
		if !strings.HasPrefix(g.BaseURL, "https://") && !strings.HasPrefix(g.BaseURL, "http://localhost") && !strings.HasPrefix(g.BaseURL, "http://127.0.0.1") {
			bad("[models.gateway] base_url = %q: want an https:// URL", g.BaseURL)
		}
		if !slices.Contains(apiStyles, g.APIStyle) {
			bad("[models.gateway] api_style = %q: want messages, responses, or chat", g.APIStyle)
		}
		for _, st := range g.Styles {
			k, style, ok := strings.Cut(st, "=")
			if !ok || k == "" || style == "" || !slices.Contains(apiStyles, style) {
				bad("[models.gateway] styles entry %q: want \"<model-id or family>=messages|responses|chat\"", st)
			}
		}
		if !secretName.MatchString(g.Secret) {
			bad("[models.gateway] secret = %q: want the name of a secret (lowercase letters, digits, - and _), never the key", g.Secret)
		}
	}
	for key, layers := range map[string][]string{"[autonomy] never": c.Autonomy.Never, "[verify.floors.screen_reader] layers": c.Verify.Floors.ScreenReader.Layers} {
		for _, l := range layers {
			if _, ok := verbs.FindLayer(verbs.LayerID(l)); !ok {
				bad("%s: unknown layer %q", key, l)
			}
		}
	}
	for _, d := range c.Approvals.DeviceFor {
		if d != "one-way" && d != "all" {
			bad("[approvals] device_for %q: want one-way or all", d)
		}
	}
	for _, a := range c.Approvals.Approvers {
		if !loginName.MatchString(strings.TrimPrefix(a, "@")) {
			bad("[approvals] approvers %q: want a GitHub login", a)
		}
	}
	switch n := c.Notify; n.Channel {
	case "":
	case "slack":
		if !secretName.MatchString(n.Target) {
			bad("[notify] channel = \"slack\" needs target = the name of the secret holding the webhook URL")
		}
	case "email":
		if !strings.Contains(n.Target, "@") || n.SMTPAddr == "" || n.From == "" {
			bad("[notify] channel = \"email\" needs target (recipient), smtp_addr, and from")
		}
	case "github":
		if n.Target == "" || !issueRef.MatchString(n.Issue) {
			bad("[notify] channel = \"github\" needs target (login) and issue = \"owner/name#number\"")
		}
	case "file":
	default:
		bad("[notify] channel = %q: want slack, email, github, or file", c.Notify.Channel)
	}
	for k, v := range c.Decisions.Authority {
		_, isType := workitems.FindType(k)
		if _, isLayer := verbs.FindLayer(verbs.LayerID(k)); !isType && !isLayer {
			bad("[decisions.authority] %s: want a work-item type (the Decision's parent) or a layer L01-L09", k)
		}
		if _, isSeat := verbs.FindSeat(verbs.SeatID(v)); !isSeat && !loginName.MatchString(v) {
			bad("[decisions.authority] %s = %q: want a seat slug or a GitHub login", k, v)
		}
	}
	if m := c.Inspect.PaidReviewer; m != "" {
		if f := verbs.FamilyOf(m); f == "" || f == verbs.FamilyHostManaged {
			bad("[inspect] paid_reviewer = %q: want a model ID of a known family (anthropic, openai, google, xai)", m)
		}
	}
	if e := c.Inspect.PaidEffort; e != "" && !slices.Contains([]string{"low", "medium", "high", "xhigh"}, e) {
		bad("[inspect] paid_effort = %q: want low, medium, high, or xhigh", e)
	}
	if pl := c.Hosts.Cursor.Plan; pl != "" && pl != verbs.PlanX && pl != verbs.PlanLegacy {
		bad("[hosts.cursor] plan = %q: want %s or %s", pl, verbs.PlanX, verbs.PlanLegacy)
	}
	if st := c.Team.Stage; st != "" && !slices.Contains(TeamStages, st) {
		bad("[team] stage = %q: want %s", st, strings.Join(TeamStages, ", "))
	}
	if c.Team.Size < 0 {
		bad("[team] size = %d: want a number of people, 0 or more", c.Team.Size)
	}
	if h := c.Decisions.SoloProvisional; h < MinSoloProvisionalHours {
		bad("[decisions] solo_provisional_after_hours = %d: the minimum is %d business hours", h, MinSoloProvisionalHours)
	}
	if e := c.Decisions.EscalateTo; e != "" && !loginName.MatchString(e) {
		bad("[decisions] escalate_to = %q: want a GitHub login", e)
	}
	for name, in := range c.Integrations {
		cloud := slices.Contains(CloudIntegrations, name)
		if in.Auth != "" && (!cloud || !slices.Contains([]string{"key", "oidc", "cli"}, in.Auth)) {
			bad("[integrations.%s] auth = %q: only [integrations.azure], [integrations.aws], and [integrations.gcp] take auth, key, oidc, or cli", name, in.Auth)
		}
		if !cloud && (in.Identity != "" || in.Federation != "" || in.Region != "") {
			bad("[integrations.%s]: identity, federation, and region are for the cloud integrations (azure, aws, gcp)", name)
		}
		if (in.Secret != "" || in.StoredKey()) && !secretName.MatchString(in.Secret) {
			bad("[integrations.%s] secret = %q: want the name of a secret (lowercase letters, digits, - and _), never the key", name, in.Secret)
		}
		if in.Scope != "read" && in.Scope != "write" {
			bad("[integrations.%s] scope = %q: want read or write", name, in.Scope)
		}
		if in.URL != "" && !strings.HasPrefix(in.URL, "https://") && !strings.HasPrefix(in.URL, "http://localhost") && !strings.HasPrefix(in.URL, "http://127.0.0.1") {
			bad("[integrations.%s] url = %q: want an https:// URL", name, in.URL)
		}
		if in.Transport != "" && (name != "figma" || (in.Transport != "rest" && in.Transport != "mcp")) {
			bad("[integrations.%s] transport = %q: only [integrations.figma] takes transport, rest or mcp", name, in.Transport)
		}
	}
	if id := c.Identity; id.MachineUser != "" && !secretName.MatchString(id.MachineUserSecret) {
		bad("[identity] machine_user_secret = %q: want the name of a secret holding the machine user's token", id.MachineUserSecret)
	}
	if id := c.Identity; id.AppID != 0 && id.AppSlug == "" {
		bad("[identity] app_id needs app_slug")
	}
	for _, r := range c.Fleet.Repos {
		if !repoName.MatchString(r) {
			bad("[fleet] repos: %q is not owner/name", r)
		}
	}
	if _, err := c.Clock(); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}
