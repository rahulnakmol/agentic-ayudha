package cli

import (
	"context"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/github"
	"github.com/rahulnakmol/agentic-ayudha/internal/identity"
	"github.com/rahulnakmol/agentic-ayudha/internal/ledger"
)

// newGitHub builds the run's GitHub client (WP2.3) for a token: every write
// goes through the actions log, stamped with ident.
func (e *Env) newGitHub(tok identity.Token, ident, capability string) (*github.Client, error) {
	log, err := e.Actions()
	if err != nil {
		return nil, err
	}
	// The run's HTTP timeout bounds each GitHub request too (Bug #822): the
	// client builds its own http.Client from the transport alone.
	o := github.Options{Token: tok.Value, Log: log, Identity: ident, Capability: capability, Transport: e.HTTP.Transport, Timeout: e.HTTP.Timeout, WriteGap: e.GitHubGap, Writes: e.writes}
	if e.GitHubGap < 0 {
		o.ServerRetries = -1 // tests without write gaps inject one-shot 5xx failures and want them seen
	}
	if e.GitHubAPI != "" {
		o.BaseURL = e.GitHubAPI
	}
	return github.New(o)
}

// HumanGitHub is a client under the human's own token (human-identity verbs and reads).
func (e *Env) HumanGitHub(ctx context.Context) (*github.Client, identity.Token, error) {
	tok, err := e.Resolver().Human(ctx)
	if err != nil {
		return nil, tok, Wrap(ExitBlocked, err)
	}
	c, err := e.newGitHub(tok, "human", "")
	return c, tok, err
}

// BotGitHub is a client under the bot identity, or under the human token
// tagged identity:human-fallback when the repository has no bot (WP1.4 day one).
func (e *Env) BotGitHub(ctx context.Context, capability string) (*github.Client, identity.Bot, error) {
	l, err := e.Config()
	if err != nil {
		return nil, identity.Bot{}, err
	}
	r := e.Resolver()
	var broker identity.Minter
	id := l.Config.Identity
	// The broker mints with the person's token; an agent that already holds
	// the bot's token (AAYU_BOT_TOKEN, a bot-only session) needs neither.
	if identity.Configured(id) == identity.KindApp && e.Getenv("AAYU_BOT_TOKEN") == "" {
		h, err := r.Human(ctx)
		if err != nil {
			return nil, identity.Bot{}, Wrap(ExitBlocked, err)
		}
		repo, err := e.RepoName(ctx)
		if err != nil {
			return nil, identity.Bot{}, err
		}
		broker = identity.Broker{API: e.githubAPI(), Repo: repo, Workflow: id.Broker, Human: h, HTTP: e.HTTP}
	}
	login := ""
	if identity.Configured(id) == "" {
		if h, err := r.Human(ctx); err == nil {
			if u, err := identity.WhoAmI(ctx, e.HTTP, e.githubAPI(), h); err == nil {
				login = u.Login
			}
		}
	}
	bot, err := r.Bot(ctx, id, broker, login)
	if err != nil {
		return nil, bot, Wrap(ExitBlocked, err)
	}
	ident := "bot"
	if bot.Kind == identity.KindHumanFallback {
		ident = identity.KindHumanFallback
	}
	c, err := e.newGitHub(bot.Token, ident, capability)
	return c, bot, err
}

// Ledger returns the ledger writer for the current checkout (WP2.4).
func (e *Env) Ledger(ctx context.Context, tok identity.Token, ident, tag string) (*ledger.Writer, error) {
	root := config.RepoRoot(e.Dir)
	if root == "" {
		return nil, Errorf(ExitUsage, "ledger writes need a checkout of the repository")
	}
	repo, err := e.RepoName(ctx)
	if err != nil {
		return nil, err
	}
	log, err := e.Actions()
	if err != nil {
		return nil, err
	}
	return &ledger.Writer{Dir: root, Repo: repo, Token: tok.Value, Log: log, Identity: ident, Tag: tag, OnAttempt: e.LedgerAttempt}, nil
}
