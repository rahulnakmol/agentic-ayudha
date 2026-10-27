package agentenv

import (
	"context"
	"encoding/json"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// Probe is what Detect may observe. Exec returns a command's combined stdout
// and stderr and a non-nil error on a nonzero exit; tests inject fakes.
type Probe struct {
	Root         string
	Repo         string // owner/name of the repository, "" when unknown
	LookPath     func(string) (string, error)
	Exec         Exec
	Getenv       func(string) string
	HasSecret    func(name string) bool // aayu's secret store has a non-empty value
	CodexEnv     string                 // [delegate] codex_env
	CursorSecret string                 // [delegate] cursor_secret; empty: cursor-api-key
}

// Agent is what this machine has of one agent: its CLI, whether it is signed
// in, and whether its cloud agent can take a hand-off. It never holds a
// secret value. Install and SignIn are exact commands, one per line.
type Agent struct {
	Name        string   `json:"name"`
	CLI         string   `json:"cli,omitempty"` // the CLI's path; "" when not installed
	Version     string   `json:"version,omitempty"`
	Warnings    []string `json:"warnings,omitempty"` // what the CLI printed beside its version, such as a stderr warning
	SignedIn    bool     `json:"signed_in"`          // the CLI can answer here: signed in, or (Codex) a model provider with its own credentials
	Via         string   `json:"via,omitempty"`      // how it answers when that is not its own sign-in: "the model provider <id>"
	Cloud       bool     `json:"cloud"`
	CloudDetail string   `json:"cloud_detail"`
	Install     []string `json:"install,omitempty"`
	SignIn      []string `json:"sign_in,omitempty"`
}

// codexProvider reports whether a Codex that is not signed in answers through
// its configured model provider (Appendix B D-5.5c.24). `codex login status`
// knows only Codex's own sign-in, and a wrapper may point Codex at another
// CODEX_HOME, so aayu asks Codex itself: `codex doctor --json` resolves the
// configuration the way `codex exec` does, and its auth.credentials check is
// ok when the active provider needs no OpenAI sign-in and has what it needs
// (an auth command, or its env_key set). A credential in an environment
// variable other than CODEX_API_KEY does not count: a local receiver's
// scrubbed environment never passes a model provider's key (D-5.5c.5).
func codexProvider(ctx context.Context, p Probe, cli string) (bool, string) {
	c, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	out, err := p.Exec(c, p.Root, cli, "doctor", "--json")
	if err != nil && out == "" {
		return false, ""
	}
	var report struct {
		Checks map[string]struct { // by check ID
			Status  string         `json:"status"`
			Details map[string]any `json:"details"`
		} `json:"checks"`
	}
	// Codex may print warnings before the report.
	if i := strings.Index(out, "\n{"); i >= 0 && !strings.HasPrefix(out, "{") {
		out = out[i+1:]
	}
	if json.Unmarshal([]byte(out), &report) != nil {
		return false, ""
	}
	provider, ok := "", false
	for id, ch := range report.Checks {
		detail := func(k string) string { v, _ := ch.Details[k].(string); return v }
		switch id {
		case "config.load":
			provider = detail("model provider")
		case "auth.credentials":
			env := detail("provider auth env var")
			ok = ch.Status == "ok" && detail("model provider requires OpenAI auth") == "false" &&
				(env == "" || strings.HasPrefix(env, "CODEX_API_KEY "))
		}
	}
	if !ok || provider == "" {
		return false, ""
	}
	return true, "the model provider " + provider
}

// CombinedExec runs a command for Detect: combined output, error on a nonzero exit.
func CombinedExec(ctx context.Context, dir, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// Detect reports the four agents. It makes no model or task call and sends
// nothing over the network itself: PATH lookups, `--version`, each CLI's
// local sign-in status, aayu's secret store and config, and the committed
// verification record (for the Amp project). Each probe gets five seconds.
// One exception: a Codex that is not signed in is asked `codex doctor --json`,
// which also checks that its model provider's endpoint answers.
func Detect(ctx context.Context, p Probe) []Agent {
	if p.Getenv == nil {
		p.Getenv = func(string) string { return "" }
	}
	if p.HasSecret == nil {
		p.HasSecret = func(string) bool { return false }
	}
	run := func(name string, args ...string) (string, bool) {
		c, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		out, err := p.Exec(c, p.Root, name, args...)
		return strings.TrimSpace(out), err == nil
	}
	look := func(bins ...string) string {
		for _, b := range bins {
			if path, err := p.LookPath(b); err == nil {
				return path
			}
		}
		return ""
	}
	version := func(bin string) (string, []string) {
		out, _ := run(bin, "--version")
		return cliVersion(out)
	}
	var out []Agent

	// Codex: cloud tasks need a ChatGPT login (not an API key) and an environment ID.
	cx := Agent{Name: "codex", CLI: look("codex"),
		Install: []string{"npm install -g @openai/codex"},
		SignIn:  []string{"codex login"}}
	chatgpt := false
	if cx.CLI != "" {
		cx.Version, cx.Warnings = version(cx.CLI)
		st, ok := run(cx.CLI, "login", "status")
		cx.SignedIn = ok && !strings.Contains(strings.ToLower(st), "not logged in")
		chatgpt = cx.SignedIn && strings.Contains(st, "ChatGPT")
		if !cx.SignedIn {
			cx.SignedIn, cx.Via = codexProvider(ctx, p, cx.CLI)
		}
	}
	switch {
	case cx.CLI == "":
		cx.CloudDetail = "the codex CLI is not installed; Codex cloud hand-offs go through `codex cloud exec`"
	case cx.Via != "":
		cx.CloudDetail = "codex runs through " + cx.Via + "; cloud tasks need a ChatGPT login"
	case !cx.SignedIn:
		cx.CloudDetail = "codex is not signed in; cloud tasks need a ChatGPT login"
	case !chatgpt:
		cx.CloudDetail = "codex is signed in with an API key; cloud tasks need a ChatGPT login (`codex logout`, then `codex login`)"
	case p.CodexEnv == "":
		cx.CloudDetail = "no Codex cloud environment: `aayu install --env --host codex` prints the values to create one, then set [delegate] codex_env"
	default:
		cx.Cloud, cx.CloudDetail = true, "codex cloud exec --env "+p.CodexEnv
	}
	out = append(out, cx)

	// Cursor: the cloud agents API with a Cursor API key.
	cu := Agent{Name: "cursor", CLI: look("cursor-agent", "agent"),
		Install: []string{"curl https://cursor.com/install -fsS | bash"},
		SignIn:  []string{"cursor-agent login"}}
	if cu.CLI != "" {
		cu.Version, cu.Warnings = version(cu.CLI)
		st, ok := run(cu.CLI, "status")
		cu.SignedIn = ok && st != "" && !strings.Contains(strings.ToLower(st), "not logged in")
	}
	secret := orStr(p.CursorSecret, "cursor-api-key")
	switch {
	case p.HasSecret(secret):
		cu.Cloud, cu.CloudDetail = true, "a Cursor API key is in aayu's secret store ("+secret+")"
	case p.Getenv("CURSOR_API_KEY") != "":
		cu.Cloud, cu.CloudDetail = true, "CURSOR_API_KEY is set"
	default:
		cu.CloudDetail = "no Cursor API key: create one at https://cursor.com/dashboard (Integrations, API keys), then `aayu auth set-secret " + secret + "` (the value on stdin)"
	}
	out = append(out, cu)

	// Claude Code: `claude --cloud` needs a claude.ai sign-in; an API key is not cloud-capable.
	cl := Agent{Name: "claude", CLI: look("claude"),
		Install: []string{"npm install -g @anthropic-ai/claude-code"},
		SignIn:  []string{"claude auth login"}}
	method := ""
	if cl.CLI != "" {
		cl.Version, cl.Warnings = version(cl.CLI)
		if st, _ := run(cl.CLI, "auth", "status", "--json"); strings.Contains(st, "{") {
			var a struct {
				LoggedIn   bool   `json:"loggedIn"`
				AuthMethod string `json:"authMethod"`
			}
			if json.Unmarshal([]byte(st[strings.Index(st, "{"):]), &a) == nil {
				cl.SignedIn, method = a.LoggedIn, a.AuthMethod
			}
		}
	}
	switch {
	case cl.CLI == "":
		cl.CloudDetail = "the claude CLI is not installed; Claude Code cloud hand-offs go through `claude --cloud`"
	case !cl.SignedIn:
		cl.CloudDetail = "claude is not signed in; cloud sessions need a claude.ai account"
	case method != "claude.ai":
		cl.CloudDetail = "claude is signed in with " + orStr(method, "an unknown method") + "; cloud sessions need a claude.ai account (`claude auth logout`, then `claude auth login`)"
	default:
		cl.Cloud, cl.CloudDetail = true, "claude --cloud with a claude.ai account"
	}
	out = append(out, cl)

	// Amp: signed in, and the repository matches an Amp project (recorded by a live probe).
	am := Agent{Name: "amp", CLI: look("amp"),
		Install: []string{"curl -fsSL https://ampcode.com/install.sh | bash"},
		SignIn:  []string{"amp login"}}
	if am.CLI != "" {
		am.Version, am.Warnings = version(am.CLI)
		st, ok := run(am.CLI, "account", "list")
		am.SignedIn = p.Getenv("AMP_API_KEY") != "" || (ok && st != "" && !strings.Contains(st, "No saved accounts"))
	}
	project := ""
	if p.Root != "" {
		if v, err := LoadVerified(p.Root); err == nil {
			project = v.Cloud[Amp].Project
		}
	}
	switch {
	case am.CLI == "":
		am.CloudDetail = "the amp CLI is not installed; Amp cloud hand-offs start an orb with `amp -ox`"
	case !am.SignedIn:
		am.CloudDetail = "amp is not signed in (no AMP_API_KEY and no saved account)"
	case project == "":
		am.CloudDetail = "no Amp project recorded for this repository: `aayu install --env --host amp --cloud` finds it and proves the orb environment"
	case p.Repo != "" && !strings.EqualFold(project, p.Repo):
		am.CloudDetail = "the recorded Amp project " + project + " is not this repository (" + p.Repo + ")"
	default:
		am.Cloud, am.CloudDetail = true, "amp -ox --project "+project
	}
	out = append(out, am)

	// OpenCode: its CLI on the model provider its own config names; it has no
	// hosted cloud agent, so its stories run locally (Story #1012).
	oc := Agent{Name: "opencode", CLI: look("opencode"),
		Install:     []string{"npm install -g opencode-ai"},
		SignIn:      []string{"opencode auth login"},
		CloudDetail: "OpenCode has no hosted cloud agent; its stories run as local sessions (`--local`)"}
	if oc.CLI != "" {
		oc.Version, oc.Warnings = version(oc.CLI)
		_, oc.SignedIn = run(oc.CLI, "auth", "list")
	}
	out = append(out, oc)
	return out
}

var (
	versionLine = regexp.MustCompile(`\d+\.\d+`)
	warningLine = regexp.MustCompile(`(?i)^(warning|warn|error|note)\b`)
)

// cliVersion splits a CLI's --version output, stdout and stderr together,
// into its version line and the other lines it printed: Codex can print
// "WARNING: proceeding, even though we could not create PATH aliases: …"
// on stderr before its version.
func cliVersion(out string) (string, []string) {
	version, rest := "", []string(nil)
	for _, l := range strings.Split(out, "\n") {
		switch l = strings.TrimSpace(l); {
		case l == "":
		case version == "" && versionLine.MatchString(l) && !warningLine.MatchString(l):
			version = l
		default:
			rest = append(rest, l)
		}
	}
	return version, rest
}
