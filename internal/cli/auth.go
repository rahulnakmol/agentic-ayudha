package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"slices"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/identity"
	"github.com/rahulnakmol/agentic-ayudha/internal/secrets"
)

func init() { register("auth", runAuth) }

// SecretStore returns the secrets store: keychain, then AAYU_SECRET_<NAME>, then
// the age-encrypted file under XDG (WP1.3).
func (e *Env) SecretStore() *secrets.Store {
	if e.Secrets == nil {
		e.Secrets = secrets.Default(e.Getenv, e.Environ)
	}
	return e.Secrets
}

// Resolver returns the identity resolver for this invocation (WP1.4).
func (e *Env) Resolver() identity.Resolver {
	r := identity.Resolver{Getenv: e.Getenv, CI: e.CI, GH: e.GHToken, Secrets: e.SecretStore()}
	if e.MCP {
		r.Hint = mcpTokenHint
	}
	return r
}

// mcpTokenHint says why the MCP server may find no token that the shell
// finds (D-3.2b.p): a host may start it without the person's environment.
const mcpTokenHint = ". This is the aayudha MCP server, which a host may start without your shell's environment: Cursor passes it only " + cursorMCPEnvList +
	", so AAYU_GITHUB_TOKEN, GH_TOKEN, GH_CONFIG_DIR, and XDG_CONFIG_HOME do not reach it. Use a source that needs none of them: `gh auth login` with gh's default config folder, or `aayu auth set-secret " + identity.HumanTokenSecret + "`; `aayu doctor` checks it"

// cursorMCPEnv are the only variables Cursor's CLI gives a stdio MCP server
// besides its mcp.json env (the MCP SDK's default environment, and PWD):
// seen in cursor-agent 2026.10.16-e3cb207 (D-3.2b.p).
var cursorMCPEnv = []string{"HOME", "LOGNAME", "PATH", "SHELL", "TERM", "USER", "PWD"}

const cursorMCPEnvList = "HOME, LOGNAME, PATH, SHELL, TERM, USER, and PWD"

// cursorMCPResolver resolves the human token the way the aayudha MCP server
// does when Cursor starts it: from Cursor's variables only, with gh and the
// secrets store seeing the same.
func (e *Env) cursorMCPResolver() identity.Resolver {
	var environ []string
	for _, k := range cursorMCPEnv {
		if v := e.Getenv(k); v != "" {
			environ = append(environ, k+"="+v)
		}
	}
	getenv := func(k string) string {
		if slices.Contains(cursorMCPEnv, k) {
			return e.Getenv(k)
		}
		return ""
	}
	gh := e.GHToken
	if gh == nil {
		gh = func(ctx context.Context) (string, error) {
			path, err := exec.LookPath("gh")
			if err != nil {
				return "", err
			}
			cmd := exec.CommandContext(ctx, path, "auth", "token")
			cmd.Env = environ
			out, err := cmd.Output()
			return strings.TrimSpace(string(out)), err
		}
	}
	// The keychain is the system's, reached from this process; a keychain an
	// MCP server cannot reach without the session's variables is not seen.
	return identity.Resolver{Getenv: getenv, GH: gh, Secrets: secrets.Default(getenv, environ)}
}

func runAuth(ctx context.Context, e *Env, in Values) error {
	name := in.String("name")
	action := in.String("action")
	if (action == "set-secret" || action == "delete-secret") && name == "" {
		return Errorf(ExitUsage, "auth %s: name the secret", action)
	}
	if name != "" && action != "set-secret" && action != "delete-secret" {
		return Errorf(ExitUsage, "auth %s takes no name", action)
	}
	if name != "" {
		if err := secrets.ValidName(name); err != nil {
			return Wrap(ExitUsage, err)
		}
	}
	store := e.SecretStore()
	switch action {
	case "set-secret":
		if e.Agent() {
			return Errorf(ExitBlocked, "auth set-secret is for humans only and is refused when AAYU_AGENT=1")
		}
		fmt.Fprintf(e.Err, "Enter the value for %s on one line (input is read from stdin, never from arguments):\n", name)
		line, err := bufio.NewReader(e.In).ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		value := strings.TrimRight(line, "\r\n")
		if value == "" {
			return Errorf(ExitUsage, "auth set-secret %s: empty value", name)
		}
		if e.DryRun {
			return e.Emit(map[string]any{"name": name, "stored": false}, func(w io.Writer) error {
				_, err := fmt.Fprintf(w, "--dry-run: would store %s\n", name)
				return err
			})
		}
		backend, err := store.Set(name, value)
		if err != nil {
			return Wrap(ExitExternal, err)
		}
		return e.Emit(map[string]any{"name": name, "backend": backend}, func(w io.Writer) error {
			_, err := fmt.Fprintf(w, "stored %s in the %s backend\n", name, backend)
			return err
		})
	case "delete-secret":
		if e.DryRun {
			return e.Emit(map[string]any{"name": name, "deleted": false}, func(w io.Writer) error {
				_, err := fmt.Fprintf(w, "--dry-run: would delete %s\n", name)
				return err
			})
		}
		from, err := store.Delete(name)
		if err != nil {
			return Wrap(ExitExternal, err)
		}
		return e.Emit(map[string]any{"name": name, "deleted_from": from}, func(w io.Writer) error {
			if len(from) == 0 {
				_, err := fmt.Fprintf(w, "%s was not stored in a writable backend (an AAYU_SECRET_* variable must be unset by you)\n", name)
				return err
			}
			_, err := fmt.Fprintf(w, "deleted %s from %s\n", name, strings.Join(from, ", "))
			return err
		})
	case "list-secrets":
		entries, err := store.List(e.referencedSecrets())
		if err != nil {
			return Wrap(ExitExternal, err)
		}
		return e.Emit(entries, func(w io.Writer) error {
			for _, en := range entries {
				b := en.Backend
				if b == "" {
					b = "not set"
				}
				fmt.Fprintf(w, "%-24s %s\n", en.Name, b)
			}
			return nil
		})
	default:
		return authStatus(ctx, e)
	}
}

// referencedSecrets lists the secret names config refers to.
func (e *Env) referencedSecrets() []string {
	names := []string{identity.HumanTokenSecret}
	l, err := e.Config()
	if err != nil {
		return names
	}
	c := l.Config
	for _, in := range c.Integrations {
		names = append(names, in.Secret)
	}
	if c.Models.Gateway.Secret != "" {
		names = append(names, c.Models.Gateway.Secret)
	}
	if c.Identity.MachineUserSecret != "" {
		names = append(names, c.Identity.MachineUserSecret)
	}
	return names
}

type authReport struct {
	Human struct {
		Source   string   `json:"source"`
		Login    string   `json:"login,omitempty"`
		Type     string   `json:"type,omitempty"`
		Scopes   []string `json:"scopes,omitempty"`
		Warnings []string `json:"warnings,omitempty"`
	} `json:"human"`
	Bot struct {
		Kind    string `json:"kind"` // app, machine-user, or not-configured
		Login   string `json:"login,omitempty"`
		Warning string `json:"warning,omitempty"`
	} `json:"bot"`
	Secrets []secrets.Status `json:"secrets_backends"`
}

func authStatus(ctx context.Context, e *Env) error {
	l, err := e.Config()
	if err != nil {
		return err
	}
	var rep authReport
	tok, err := e.Resolver().Human(ctx)
	if err != nil {
		return Wrap(ExitBlocked, err)
	}
	rep.Human.Source = tok.Source
	u, err := identity.WhoAmI(ctx, e.HTTP, e.githubAPI(), tok)
	if err != nil {
		return Wrap(ExitExternal, err)
	}
	rep.Human.Login, rep.Human.Type, rep.Human.Scopes = u.Login, u.Type, u.Scopes
	rep.Human.Warnings = identity.ScopeWarnings(u)

	id := l.Config.Identity
	switch identity.Configured(id) {
	case identity.KindApp:
		rep.Bot.Kind, rep.Bot.Login = identity.KindApp, id.AppSlug+"[bot]"
	case identity.KindMachineUser:
		rep.Bot.Kind, rep.Bot.Login = identity.KindMachineUser, id.MachineUser
	default:
		rep.Bot.Kind = "not-configured"
		rep.Bot.Warning = "bot identity: not configured; agent writes run under the human identity, tagged " + identity.HumanFallbackTag
	}
	if e.CI && e.Getenv("AAYU_BOT_TOKEN") == "" {
		rep.Bot.Warning = identity.ErrNoBotInCI.Error()
	}
	rep.Secrets = e.SecretStore().Status()

	return e.Emit(rep, func(w io.Writer) error {
		fmt.Fprintf(w, "human token: %s -> %s (%s)\n", rep.Human.Source, rep.Human.Login, rep.Human.Type)
		if len(rep.Human.Scopes) > 0 {
			fmt.Fprintf(w, "  scopes: %s\n", strings.Join(rep.Human.Scopes, ", "))
		}
		for _, wn := range rep.Human.Warnings {
			fmt.Fprintf(w, "  warning: %s\n", wn)
		}
		switch rep.Bot.Kind {
		case "not-configured":
			fmt.Fprintln(w, "bot identity: not configured")
		default:
			fmt.Fprintf(w, "bot identity: %s %s\n", rep.Bot.Kind, rep.Bot.Login)
		}
		if rep.Bot.Warning != "" {
			fmt.Fprintf(w, "  warning: %s\n", rep.Bot.Warning)
		}
		fmt.Fprintln(w, "secrets backends:")
		for _, s := range rep.Secrets {
			if s.Available {
				fmt.Fprintf(w, "  %-8s available\n", s.Backend)
			} else {
				fmt.Fprintf(w, "  %-8s unavailable: %s\n", s.Backend, s.Reason)
			}
		}
		return nil
	})
}

func (e *Env) githubAPI() string {
	if e.GitHubAPI != "" {
		return e.GitHubAPI
	}
	return "https://api.github.com"
}
