package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
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
	return identity.Resolver{Getenv: e.Getenv, CI: e.CI, GH: e.GHToken, Secrets: e.SecretStore()}
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
		if sig := e.agentSignal(); sig != "" { // no terminal needed: a person may pipe the value in
			return Errorf(ExitBlocked, "auth set-secret is for humans only and is refused for an agent lane (%s)", sig)
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
