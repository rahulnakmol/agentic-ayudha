// Package doctor implements `aayu doctor` (WP1.7): environment, identity,
// configuration, and coexistence checks. Each check is a pure function of a
// Probe, so tests drive every check through pass and fail fixtures. Features
// from later work packages report "not yet available" (ground rule 1a).
package doctor

import (
	"context"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/identity"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations"
	"github.com/rahulnakmol/agentic-ayudha/internal/secrets"
)

// Status of a check.
type Status string

// Statuses.
const (
	Pass Status = "pass"
	Warn Status = "warn"
	Fail Status = "fail"
	NA   Status = "n/a" // not yet available: the feature's work package has not shipped
)

// Result is one check's outcome.
type Result struct {
	ID       string   `json:"id"`
	Status   Status   `json:"status"`
	Blocking bool     `json:"blocking,omitempty"` // a floor-blocking failure: doctor exits nonzero
	Detail   string   `json:"detail"`
	Lines    []string `json:"lines,omitempty"`
}

// Probe is everything checks may observe. Tests supply fakes.
type Probe struct {
	Root     string // repository root; "" outside a repository
	Getenv   func(string) string
	LookPath func(string) (string, error)
	Exec     func(ctx context.Context, dir, name string, args ...string) (string, error)
	// Combined is Exec with stdout and stderr together (agent CLIs print their sign-in status on stderr).
	Combined  func(ctx context.Context, dir, name string, args ...string) (string, error)
	Config    *config.Loaded
	ConfigErr error
	Secrets   *secrets.Store
	Human     func(ctx context.Context) (identity.Token, error)
	// CursorMCPHuman resolves the human token as the aayudha MCP server does
	// when Cursor starts it, with only Cursor's variables (D-3.2b.p).
	CursorMCPHuman func(ctx context.Context) (identity.Token, error)
	WhoAmI         func(ctx context.Context, t identity.Token) (identity.User, error)
	API            func(ctx context.Context, path string, out any) error // GitHub REST GET; nil without a token
	Repo           string                                                // owner/name; "" when unknown
	Version        string
	Latest         func(ctx context.Context) (string, error)
	Host           string // --host
	// Integration resolves a configured integration and asks its provider for
	// the key's account and scope (providers.Check); nil when unavailable.
	Integration func(ctx context.Context, name string) (integrations.Scope, error)
	TracePath   string
}

// Check is one named check.
type Check struct {
	ID  string
	Run func(ctx context.Context, p *Probe) Result
}

// Checks run in this order.
var Checks = []Check{
	{"binary", checkBinary},
	{"git", checkGit},
	{"git-lfs", checkGitLFS},
	{"gh", checkGH},
	{"token", checkToken},
	{"bot-identity", checkBot},
	{"config", checkConfig},
	{"schema", checkSchema},
	{"host", checkHost},
	{"secrets", checkSecrets},
	{"skills-copies", checkSkillCopies},
	{"mcp", checkMCP},
	{"codex-hooks", checkCodexHooks},
	{"lfs-quota", checkLFSQuota},
	{"ledger-media", checkLedgerMedia},
	{"ledger-rulesets", checkLedgerRulesets},
	{"proof", checkProof},
	{"issue-types", checkIssueTypes},
	{"integrations", checkIntegrations},
	{"optional-tools", checkOptionalTools},
	{"worktrees", checkWorktrees},
	{"agent-env", checkAgentEnv},
	{"agents", checkAgents},
	{"reviewer-apps", checkReviewerApps},
	{"retired-workflows", checkRetiredWorkflows},
	{"loops", checkLoops},
	{"leases", checkLeases},
	{"feedback-layers", checkFeedbackLayers},
	{"coexistence", checkCoexistence},
	{"telemetry", checkTelemetry},
}

// Run runs every check.
func Run(ctx context.Context, p *Probe) []Result {
	out := make([]Result, 0, len(Checks))
	for _, c := range Checks {
		r := c.Run(ctx, p)
		r.ID = c.ID
		out = append(out, r)
	}
	return out
}

// Blocking reports whether any result is a floor-blocking failure.
func Blocking(rs []Result) bool {
	for _, r := range rs {
		if r.Status == Fail && r.Blocking {
			return true
		}
	}
	return false
}

// Render writes results as an aligned table, with detail lines indented.
func Render(w io.Writer, rs []Result) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, r := range rs {
		detail := r.Detail
		if r.Blocking && r.Status == Fail {
			detail += " [blocking]"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\n", strings.ToUpper(string(r.Status)), r.ID, detail)
		for _, l := range r.Lines {
			fmt.Fprintf(tw, "\t\t  %s\n", l)
		}
	}
	return tw.Flush()
}
