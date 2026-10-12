// Package providers is the catalog of integration adapters this binary
// carries: what each one is for, the metrics it answers, the config fields it
// needs, and the entry points verbs (Baseline) and doctor (Check) use.
package providers

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations/githubread"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations/posthog"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations/sentry"
	"github.com/rahulnakmol/agentic-ayudha/internal/secrets"
)

// Deps is what an adapter needs from the running verb.
type Deps struct {
	Config    *config.Config
	Secrets   *secrets.Store
	Transport http.RoundTripper // nil means http.DefaultTransport; tests replay fixtures
	Repo      string            // the current repository (owner/name), GitHub's default
	Now       func() time.Time
}

func (d Deps) now() time.Time {
	if d.Now == nil {
		return time.Now()
	}
	return d.Now()
}

// baseliner is what every baseline adapter implements.
type baseliner interface {
	Scope(ctx context.Context) (integrations.Scope, error)
	Baseline(ctx context.Context, q integrations.Query) (integrations.Baseline, error)
}

// Provider is one adapter.
type Provider struct {
	Name    string
	Purpose string
	Need    []integrations.Field
	open    func(d Deps, c integrations.Conn) (baseliner, error)
}

// Known lists the adapters in this binary, in the order doctor reports them.
var Known = []Provider{
	{
		Name: "posthog", Purpose: "product analytics baselines", Need: posthog.Need,
		open: func(d Deps, c integrations.Conn) (baseliner, error) { return posthog.New(c, d.Transport, d.now) },
	},
	{
		Name: "sentry", Purpose: "reliability baselines", Need: sentry.Need,
		open: func(d Deps, c integrations.Conn) (baseliner, error) { return sentry.New(c, d.Transport, d.now) },
	},
	{
		Name: "github", Purpose: "delivery baselines from issues and pull requests",
		open: func(d Deps, c integrations.Conn) (baseliner, error) {
			repo := c.Project
			if repo == "" {
				repo = d.Repo
			}
			return githubread.New(c, repo, d.Transport, d.now)
		},
	},
}

// Find returns the provider named name.
func Find(name string) (Provider, bool) {
	i := slices.IndexFunc(Known, func(p Provider) bool { return p.Name == name })
	if i < 0 {
		return Provider{}, false
	}
	return Known[i], true
}

func open(d Deps, name string) (Provider, baseliner, error) {
	p, ok := Find(name)
	if !ok {
		names := make([]string, len(Known))
		for i, k := range Known {
			names[i] = k.Name
		}
		return p, nil, fmt.Errorf("%w: %q (this binary has %s)", integrations.ErrNoAdapter, name, strings.Join(names, ", "))
	}
	c, err := integrations.Resolve(d.Config, d.Secrets, name, p.Need...)
	if err != nil {
		return p, nil, err
	}
	b, err := p.open(d, c)
	return p, b, err
}

// Baseline fetches one baseline from the named source (posthog, sentry,
// github; each adapter's Metrics lists what it answers). Nothing is sent when the integration is missing, incomplete,
// configured with scope = "write", or the query is invalid; the adapter then
// refuses a key the provider reports as write-capable before the data call.
func Baseline(ctx context.Context, d Deps, source string, q integrations.Query) (integrations.Baseline, error) {
	_, b, err := open(d, source)
	if err != nil {
		return integrations.Baseline{}, err
	}
	return b.Baseline(ctx, q)
}

// Check resolves the named integration and asks its provider who the key
// belongs to and what it may do: doctor's reachability and scope check. It
// returns the same errors as Baseline for a missing, write-scoped, or
// unset integration, without calling the provider.
func Check(ctx context.Context, d Deps, name string) (integrations.Scope, error) {
	_, b, err := open(d, name)
	if err != nil {
		return integrations.Scope{}, err
	}
	return b.Scope(ctx)
}
