// Package hostcall decides the PostHog and Sentry calls an agent makes
// through its host rather than through aayudha's own adapters (Decision
// #449): a tool of a host-connected PostHog or Sentry MCP server, or a
// command of their CLIs. Reads on the pinned allowlists pass; every write is
// refused, because a write to a provider is a catalog action a person
// approves. The pre-tool hook asks Decide before the host runs the call.
package hostcall

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"
)

//go:embed posthog-mcp-read.txt
var posthogList string

//go:embed sentry-mcp-read.txt
var sentryList string

// list is one provider's allowlist: the read tools, and the read tools that
// return personal data.
type list struct{ read, pii map[string]bool }

func parse(s string) list {
	l := list{map[string]bool{}, map[string]bool{}}
	set := l.read
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case line == "" || strings.HasPrefix(line, "#"):
		case line == "[pii]":
			set = l.pii
		default:
			set[line] = true
		}
	}
	return l
}

var lists = map[string]list{"posthog": parse(posthogList), "sentry": parse(sentryList)}

// Names of the providers, as their [integrations.<name>] sections name them.
const (
	PostHog = "posthog"
	Sentry  = "sentry"
)

// Event is one tool call as a host reports it to the pre-tool hook.
type Event struct {
	Tool   string         // the host's tool name: mcp__<server>__<tool> (Claude Code, Codex, Amp) or the bare tool (Cursor)
	Input  map[string]any // the tool's arguments
	Server string         // Cursor: the MCP server's URL or command
	Shell  string         // the shell command, for a shell tool
}

// Verdict is what Decide says about one call. Provider is "" when the call
// is neither PostHog's nor Sentry's; such a call is not decided here.
type Verdict struct {
	Provider string `json:"provider,omitempty"` // posthog, sentry
	Channel  string `json:"channel,omitempty"`  // mcp, cli
	Call     string `json:"call,omitempty"`     // the tool or command decided, after unwrapping exec, execute_sentry_tool, or npx
	Allow    bool   `json:"allow"`
	Reason   string `json:"reason,omitempty"` // why a call is refused, for the agent
}

// Options are the per-provider settings a project chose.
type Options struct {
	AllowPII map[string]bool // provider → [integrations.<provider>] allow_pii
}

// Decide decides one call. It is pure: no network, no files.
func Decide(e Event, o Options) Verdict {
	if e.Shell != "" {
		return decideShell(e.Shell, o)
	}
	provider, tool := mcpTool(e)
	if provider == "" {
		return Verdict{Allow: true}
	}
	v := Verdict{Provider: provider, Channel: "mcp", Call: tool}
	switch {
	case provider == PostHog && tool == "exec":
		return posthogExec(v, str(e.Input["command"]), o)
	case provider == Sentry && tool == "execute_sentry_tool":
		inner := str(e.Input["name"])
		v.Call = tool + " " + inner
		return check(v, inner, o)
	}
	return check(v, tool, o)
}

// check allows tool when it is on the provider's read list (or its personal
// data list with allow_pii) and refuses it otherwise.
func check(v Verdict, tool string, o Options) Verdict {
	l := lists[v.Provider]
	name := map[string]string{PostHog: "PostHog", Sentry: "Sentry"}[v.Provider]
	switch {
	case l.read[tool]:
		v.Allow = true
	case l.pii[tool] && o.AllowPII[v.Provider]:
		v.Allow = true
	case l.pii[tool]:
		v.Reason = piiReason(v.Provider, quote(tool))
	default:
		v.Reason = fmt.Sprintf("aayudha: %s %s is not on aayudha's read allowlist, so it may write or spend: agents only read %s through a host. "+
			"A write is a catalog action a person approves: propose it with the verb that owns it, which runs it through ops/actions.toml.", name, quote(tool), name)
	}
	return v
}

// piiReason refuses a read that returns personal data while allow_pii is off.
func piiReason(provider, call string) string {
	name := map[string]string{PostHog: "PostHog", Sentry: "Sentry"}[provider]
	return fmt.Sprintf("aayudha: %s %s returns personal data, which is opt-in: a person sets [integrations.%s] allow_pii = true in .aayudha/config.toml to allow it.", name, call, provider)
}

func quote(s string) string {
	if s == "" {
		return "(no tool named)"
	}
	return "`" + s + "`"
}

func str(v any) string { s, _ := v.(string); return strings.TrimSpace(s) }

// mcpTool names the provider and tool of an MCP call. A server is PostHog's
// or Sentry's when its name (mcp__<server>__…) or, in Cursor, its URL or
// command says so.
func mcpTool(e Event) (provider, tool string) {
	server, tool := "", e.Tool
	if rest, ok := strings.CutPrefix(e.Tool, "mcp__"); ok {
		if i := strings.Index(rest, "__"); i > 0 {
			server, tool = rest[:i], rest[i+2:]
		}
	}
	if server == "" {
		server = e.Server
	}
	s := strings.ToLower(server)
	switch {
	case server == "":
		return "", ""
	case strings.Contains(s, "posthog"):
		return PostHog, tool
	case strings.Contains(s, "sentry"):
		return Sentry, tool
	}
	return "", ""
}

// execReads are the PostHog exec verbs that only look at the catalog.
var execReads = []string{"help", "learn", "tools", "search", "info", "schema"}

// posthogExec decides PostHog's CLI-mode exec tool: its command holds one or
// more lines, each "<verb> …"; only "call <tool>" runs a tool, and each called
// tool must pass.
func posthogExec(v Verdict, command string, o Options) Verdict {
	v.Call = "exec"
	if command == "" {
		v.Reason = "aayudha: PostHog `exec` with no command is refused."
		return v
	}
	for _, line := range execLines(command) {
		f := strings.Fields(line)
		if slices.Contains(execReads, f[0]) {
			continue
		}
		if f[0] != "call" {
			v.Reason = fmt.Sprintf("aayudha: PostHog `exec %s` is not a command aayudha knows to be a read.", f[0])
			return v
		}
		tool := ""
		for _, t := range f[1:] {
			if !strings.HasPrefix(t, "--") {
				tool = t
				break
			}
		}
		if r := check(Verdict{Provider: PostHog, Channel: "mcp"}, tool, o); !r.Allow {
			r.Call = "exec call " + tool
			return r
		}
		v.Call = "exec call " + tool
	}
	v.Allow = true
	return v
}

// execLines splits an exec command into its commands: a line that starts
// with an exec verb starts a new one (a call's JSON body may span lines).
func execLines(command string) []string {
	var out []string
	for _, line := range strings.Split(command, "\n") {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		verb := strings.Fields(t)[0]
		if len(out) == 0 || verb == "call" || slices.Contains(execReads, verb) {
			out = append(out, t)
		}
	}
	return out
}

// shellSplit splits a shell command into simple commands at ;, &&, ||, |,
// and newlines. It is deliberately simple: a provider CLI hidden in a
// subshell or a quoted string is not seen.
var shellSplit = regexp.MustCompile(`&&|\|\||[;|\n]`)

// runners are commands that run a package's binary: npx @sentry/cli …
var runners = map[string]int{"npx": 1, "bunx": 1, "pnpx": 1, "pnpm": 2, "yarn": 2} // pnpm dlx, yarn dlx

func decideShell(shell string, o Options) Verdict {
	first := Verdict{Allow: true}
	for _, part := range shellSplit.Split(shell, -1) {
		f := strings.Fields(part)
		for len(f) > 0 && strings.Contains(f[0], "=") && !strings.HasPrefix(f[0], "-") { // FOO=bar cmd
			f = f[1:]
		}
		if len(f) > 0 {
			if n, ok := runners[path.Base(f[0])]; ok && len(f) > n {
				f = f[n:]
				for len(f) > 0 && strings.HasPrefix(f[0], "-") { // npx -y @sentry/cli
					f = f[1:]
				}
			}
		}
		if len(f) == 0 {
			continue
		}
		v := decideCLI(cliName(f[0]), f[1:], o)
		switch {
		case v.Provider == "":
		case !v.Allow:
			return v // any part that may write refuses the whole command
		case first.Provider == "":
			first = v
		}
	}
	return first
}

// cliName maps a program to the CLI it is: sentry-cli, sentry, posthog-cli.
func cliName(prog string) string {
	p := path.Base(prog)
	if i := strings.LastIndex(p, "@"); i > 0 { // @sentry/cli@3, posthog-cli@0.18
		p = p[:i]
	}
	switch {
	case p == "sentry-cli" || prog == "@sentry/cli" || strings.HasPrefix(prog, "@sentry/cli@"):
		return "sentry-cli"
	case p == "sentry":
		return "sentry"
	case p == "posthog-cli" || strings.HasPrefix(prog, "@posthog/cli"):
		return "posthog-cli"
	}
	return ""
}

// args drops flags (and the value of a --flag=value form) so the
// subcommands remain.
func words(args []string) []string {
	var out []string
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			out = append(out, a)
		}
	}
	return out
}

// Read subcommands of each CLI, as "<sub> <sub>" prefixes. A command whose
// words start with one of them reads; any other command is refused.
var cliReads = map[string][]string{
	"sentry-cli": {"", "info", "help", "organizations list", "projects list", "repos list", "releases list", "releases info",
		"deploys list", "issues list", "events list", "logs list", "monitors list", "sourcemaps resolve", "debug-files check",
		"debug-files find", "debug-files print-sources"},
	"sentry": {"", "help", "schema", "explore", "auth status", "trace logs", "alert issues list", "alert issues view",
		"alert metrics list", "alert metrics view", "alert metrics status"},
	"posthog-cli": {"", "help", "exp query run", "exp query check", "exp query editor", "exp endpoints list", "exp endpoints get",
		"exp endpoints pull", "exp endpoints run", "exp schema pull", "exp schema status", "api tools", "api search", "api info", "api schema"},
}

// sentryNouns are the new sentry CLI's nouns whose list, view, and events
// subcommands read; replays and user feedback return personal data, so they
// read only with allow_pii.
var (
	sentryNouns = []string{"issue", "event", "span", "trace", "log", "replay", "feedback", "dashboard", "release", "monitor", "org", "project", "team", "repo", "dsn"}
	sentryPII   = []string{"replay", "feedback"}
)

func decideCLI(cli string, args []string, o Options) Verdict {
	if cli == "" {
		return Verdict{}
	}
	provider := Sentry
	if cli == "posthog-cli" {
		provider = PostHog
	}
	w := words(args)
	cmd := strings.TrimSpace(cli + " " + strings.Join(w, " "))
	v := Verdict{Provider: provider, Channel: "cli", Call: cmd}
	if cli == "posthog-cli" && len(w) >= 3 && w[0] == "api" && w[1] == "call" {
		r := check(Verdict{Provider: PostHog, Channel: "cli"}, w[2], o)
		r.Call = cmd
		return r
	}
	if cli == "sentry" && len(w) >= 2 && slices.Contains(sentryNouns, w[0]) && slices.Contains([]string{"list", "view", "events"}, w[1]) {
		if slices.Contains(sentryPII, w[0]) && !o.AllowPII[Sentry] {
			v.Reason = piiReason(Sentry, "`"+cmd+"`")
			return v
		}
		v.Allow = true
		return v
	}
	for _, r := range cliReads[cli] {
		rw := strings.Fields(r)
		if (r == "" && len(w) == 0) || (len(rw) > 0 && len(w) >= len(rw) && slices.Equal(w[:len(rw)], rw)) {
			v.Allow = true
			return v
		}
	}
	name := map[string]string{PostHog: "PostHog", Sentry: "Sentry"}[provider]
	v.Reason = fmt.Sprintf("aayudha: `%s` is not a %s read aayudha knows, so it may write or spend: agents only read %s. "+
		"Releases, deploys, and source-map uploads belong to the product's CI; any other write is a catalog action a person approves, proposed with the verb that owns it.", cmd, name, name)
	return v
}

// FromHook reads a host's pre-tool event: Claude Code and Codex send
// tool_name and tool_input (an object); Cursor's beforeMCPExecution sends
// tool_name, tool_input (a JSON string), and the server's url or command, and
// beforeShellExecution sends command; the Amp plugin sends what Claude Code
// does.
func FromHook(event []byte) (Event, bool) {
	var raw struct {
		ToolName  string          `json:"tool_name"`
		ToolInput json.RawMessage `json:"tool_input"`
		URL       string          `json:"url"`
		Command   string          `json:"command"`
	}
	if json.Unmarshal(event, &raw) != nil {
		return Event{}, false
	}
	e := Event{Tool: raw.ToolName, Server: raw.URL}
	in := map[string]any{}
	if len(raw.ToolInput) > 0 {
		if json.Unmarshal(raw.ToolInput, &in) != nil {
			var s string
			if json.Unmarshal(raw.ToolInput, &s) == nil {
				_ = json.Unmarshal([]byte(s), &in)
			}
		}
	}
	e.Input = in
	if c, ok := in["command"].(string); ok && isShellTool(raw.ToolName) {
		e.Shell = c
	}
	switch {
	case raw.ToolName == "" && raw.Command != "": // Cursor beforeShellExecution
		e.Shell = raw.Command
	case raw.ToolName != "" && raw.Command != "" && e.Server == "": // Cursor beforeMCPExecution of a stdio server
		e.Server = raw.Command
	}
	return e, e.Tool != "" || e.Shell != ""
}

// isShellTool reports a host's shell tool, whose command input is a shell command.
func isShellTool(name string) bool {
	switch strings.ToLower(name) {
	case "bash", "shell", "shell_command", "local_shell", "exec_command":
		return true
	}
	return false
}
