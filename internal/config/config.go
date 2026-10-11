package config

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/verbs"
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
	Fleet         Fleet                  `koanf:"fleet" jsonschema_description:"User-level only: the product repos that status --all and doctor --all iterate."`
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
	// Lanes holds [models.<lane>.<host>] keyed by lane, then host.
	Lanes map[string]map[string]ModelChoice `koanf:",remain"`
}

// ModelChoice is [models.<lane>.<host>].
type ModelChoice struct {
	Model  string `koanf:"model" jsonschema_description:"Model ID as the provider or gateway names it."`
	Family string `koanf:"family" jsonschema:"enum=,enum=anthropic,enum=openai,enum=google,enum=xai,enum=host-managed" jsonschema_description:"Model family; inferred from the model ID when empty."`
}

// Gateway is [models.gateway]: when base_url is set, every lane's model calls go
// through this one endpoint (WP1.6); direct provider clients remain the fallback
// when it is unset. The wire format is chosen per model: styles.<model-id>, then
// styles.<family>, then api_style, then the family default (anthropic: messages,
// openai: responses, others: chat). The model family never depends on the endpoint.
type Gateway struct {
	BaseURL  string            `koanf:"base_url" jsonschema_description:"Gateway endpoint including the version path, e.g. https://gateway.example/v1; empty disables the gateway."`
	Secret   string            `koanf:"secret" jsonschema_description:"Name of the secret holding the gateway key (sent as Authorization: Bearer)."`
	APIStyle string            `koanf:"api_style" jsonschema:"enum=,enum=messages,enum=responses,enum=chat" jsonschema_description:"Wire format for every model without a styles entry: messages (Anthropic Messages), responses (OpenAI Responses), chat (OpenAI Chat Completions). Empty uses the family default."`
	Styles   map[string]string `koanf:"styles" jsonschema_description:"Wire format per model ID or family, overriding api_style."`
}

// LFS is [lfs]. 0.1 supports GitHub LFS only (V6).
type LFS struct {
	QuotaWarnPercent int `koanf:"quota_warn_percent" jsonschema:"minimum=1,maximum=100" jsonschema_description:"doctor warns when LFS storage or bandwidth use reaches this percentage."`
}

// Proof is [proof].
type Proof struct {
	Media         string `koanf:"media" jsonschema_description:"Private bucket for proof media; required on public repos (V3)."`
	RetentionDays int    `koanf:"retention_days" jsonschema:"minimum=0" jsonschema_description:"Days to keep proof media before purge; 0 keeps forever."`
}

// Autonomy is [autonomy].
type Autonomy struct {
	Streak   int            `koanf:"streak" jsonschema:"minimum=0" jsonschema_description:"Correction-free merges per layer footprint before agent lanes may self-ship."`
	Never    []string       `koanf:"never" jsonschema_description:"Layers (L01-L09) that never run autonomously."`
	Approver string         `koanf:"approver" jsonschema_description:"GitHub login that approves when autonomy does not apply."`
	Budget   AutonomyBudget `koanf:"budget"`
}

// AutonomyBudget is [autonomy.budget], the per-story build budget (WP5.5).
type AutonomyBudget struct {
	WallHours float64 `koanf:"wall_hours" jsonschema:"minimum=0"`
	CostUSD   float64 `koanf:"cost_usd" jsonschema:"minimum=0"`
	Attempts  int     `koanf:"attempts" jsonschema:"minimum=0"`
}

// Decisions is [decisions]. SLA durations are business hours ([time]).
type Decisions struct {
	Authority        map[string]string `koanf:"authority" jsonschema_description:"Decision type or layer to the seat or login that may accept it."`
	SLAHours         int               `koanf:"sla_hours" jsonschema:"minimum=0" jsonschema_description:"Business hours before an open Decision pages its authority and escalates."`
	ProvisionalAfter int               `koanf:"provisional_after_hours" jsonschema:"minimum=0" jsonschema_description:"Business hours before a reversible Decision's recommendation applies provisionally (V8)."`
	EscalateTo       string            `koanf:"escalate_to" jsonschema_description:"GitHub login mentioned on escalation (founder B)."`
}

// Notify is [notify].
type Notify struct {
	Channel string `koanf:"channel" jsonschema:"enum=,enum=slack,enum=email,enum=github"`
	Target  string `koanf:"target" jsonschema_description:"Slack webhook secret name, email address, or GitHub login, by channel."`
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
}

// Verify is [verify].
type Verify struct {
	DAST           bool     `koanf:"dast" jsonschema_description:"Run the ZAP baseline floor (WP4.6)."`
	Platforms      []string `koanf:"platforms" jsonschema_description:"Platforms the product ships on, e.g. web, ios, android."`
	UpstreamChecks []string `koanf:"upstream_checks" jsonschema_description:"Existing product CI check names the merge gate also requires."`
	Floors         Floors   `koanf:"floors" jsonschema_description:"Floor thresholds; CONSTRAINTS.md documents each one."`
}

// Floors is [verify.floors]: the deterministic floors (rule 7).
type Floors struct {
	Coverage struct {
		MinDiffPercent float64 `koanf:"min_diff_percent" jsonschema:"minimum=0,maximum=100"`
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
	DAST struct {
		MaxHigh int `koanf:"max_high" jsonschema:"minimum=0"`
	} `koanf:"dast"`
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

// Fleet is [fleet] (user-level config only).
type Fleet struct {
	Repos []string `koanf:"repos" jsonschema_description:"owner/name of each product repo."`
}

// Clock returns the business-hours clock for [time].
func (c *Config) Clock() (Clock, error) { return NewClock(c.Time.TZ, c.Time.BusinessHours) }

var (
	secretName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)
	repoName   = regexp.MustCompile(`^[A-Za-z0-9-]+/[A-Za-z0-9._-]+$`)
	apiStyles  = []string{"", "messages", "responses", "chat"}
	families   = []string{"", string(verbs.FamilyAnthropic), string(verbs.FamilyOpenAI), string(verbs.FamilyGoogle), string(verbs.FamilyXAI), string(verbs.FamilyHostManaged)}
)

// Validate checks values that the struct types alone cannot.
func (c *Config) Validate() error {
	var errs []error
	bad := func(format string, args ...any) { errs = append(errs, fmt.Errorf(format, args...)) }

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
			if m.Model == "" {
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
		for k, style := range g.Styles {
			if style == "" || !slices.Contains(apiStyles, style) {
				bad("[models.gateway.styles] %s = %q: want messages, responses, or chat", k, style)
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
	switch c.Notify.Channel {
	case "", "slack", "email", "github":
	default:
		bad("[notify] channel = %q: want slack, email, or github", c.Notify.Channel)
	}
	for name, in := range c.Integrations {
		if !secretName.MatchString(in.Secret) {
			bad("[integrations.%s] secret = %q: want the name of a secret (lowercase letters, digits, - and _), never the key", name, in.Secret)
		}
		if in.Scope != "read" && in.Scope != "write" {
			bad("[integrations.%s] scope = %q: want read or write", name, in.Scope)
		}
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
