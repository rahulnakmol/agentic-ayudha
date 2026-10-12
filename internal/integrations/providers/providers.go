// Package providers is the catalog of integration adapters this binary
// carries: what each one is for, the config fields it needs, and the entry
// points verbs (Baseline, Figma, Mobbin) and doctor (Check) use.
package providers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/config"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations/aws"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations/azure"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations/cloud"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations/figma"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations/gcp"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations/githubread"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations/growthbook"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations/mobbin"
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
	// Cloud adapters (azure, aws, gcp): the environment (GitHub Actions
	// OIDC variables), the az/aws/gcloud runner for auth = "cli", and the
	// wait between polls. nil means the process's own.
	Getenv func(string) string
	Exec   cloud.Exec
	Sleep  func(time.Duration)
}

func (d Deps) cloud() cloud.Env { return cloud.Env{Getenv: d.Getenv, Exec: d.Exec, Now: d.Now} }

func (d Deps) sleep() func(time.Duration) {
	if d.Sleep == nil {
		return time.Sleep
	}
	return d.Sleep
}

func (d Deps) now() time.Time {
	if d.Now == nil {
		return time.Now()
	}
	return d.Now()
}

// scoper is what every adapter implements: doctor's reachability and scope check.
type scoper interface {
	Scope(ctx context.Context) (integrations.Scope, error)
}

// baseliner is what a baseline adapter adds.
type baseliner interface {
	scoper
	Baseline(ctx context.Context, q integrations.Query) (integrations.Baseline, error)
}

// exporter is what an adapter that pulls raw events for measure adds.
type exporter interface {
	scoper
	Export(ctx context.Context, w integrations.Window, out io.Writer) (kind, query string, err error)
}

// Provider is one adapter.
type Provider struct {
	Name    string
	Purpose string
	Need    []integrations.Field
	open    func(d Deps, c integrations.Conn) (scoper, error)
}

// Known lists the adapters in this binary, in the order doctor reports them.
var Known = []Provider{
	{
		Name: "posthog", Purpose: "product analytics baselines", Need: posthog.Need,
		open: func(d Deps, c integrations.Conn) (scoper, error) { return posthog.New(c, d.Transport, d.now) },
	},
	{
		Name: "sentry", Purpose: "reliability baselines", Need: sentry.Need,
		open: func(d Deps, c integrations.Conn) (scoper, error) { return sentry.New(c, d.Transport, d.now) },
	},
	{
		Name: "github", Purpose: "delivery baselines from issues and pull requests",
		open: func(d Deps, c integrations.Conn) (scoper, error) {
			repo := c.Project
			if repo == "" {
				repo = d.Repo
			}
			return githubread.New(c, repo, d.Transport, d.now)
		},
	},
	{
		Name: "growthbook", Purpose: "experiment results for readout and measure",
		open: func(d Deps, c integrations.Conn) (scoper, error) { return growthbook.New(c, d.Transport, d.now), nil },
	},
	{
		Name: "azure", Purpose: "Application Insights baselines and events (Log Analytics query API)", Need: azure.Need,
		open: func(d Deps, c integrations.Conn) (scoper, error) { return azure.New(c, d.Transport, d.cloud()) },
	},
	{
		Name: "aws", Purpose: "CloudWatch RUM baselines (Logs Insights) and events", Need: aws.Need,
		open: func(d Deps, c integrations.Conn) (scoper, error) {
			cl, err := aws.New(c, d.Transport, d.cloud())
			if cl != nil {
				cl.Sleep = d.sleep()
			}
			return cl, err
		},
	},
	{
		Name: "gcp", Purpose: "GA4 BigQuery export baselines and events", Need: gcp.Need,
		open: func(d Deps, c integrations.Conn) (scoper, error) {
			cl, err := gcp.New(c, d.Transport, d.cloud())
			if cl != nil {
				cl.Sleep = d.sleep()
			}
			return cl, err
		},
	},
	{
		Name: "figma", Purpose: "design files: nodes, rendered images, and variables",
		open: func(d Deps, c integrations.Conn) (scoper, error) { return figma.New(c, d.Transport) },
	},
	{
		Name: "mobbin", Purpose: "design references as URLs and IDs",
		open: func(d Deps, c integrations.Conn) (scoper, error) { return mobbin.New(c, d.Transport, d.now), nil },
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

func open(d Deps, name string) (Provider, scoper, error) {
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
// github, growthbook; each adapter's Metrics lists what it answers). Nothing is sent when the integration is missing, incomplete,
// configured with scope = "write", or the query is invalid; the adapter then
// refuses a key the provider reports as write-capable before the data call.
func Baseline(ctx context.Context, d Deps, source string, q integrations.Query) (integrations.Baseline, error) {
	_, s, err := open(d, source)
	if err != nil {
		return integrations.Baseline{}, err
	}
	b, ok := s.(baseliner)
	if !ok {
		return integrations.Baseline{}, fmt.Errorf("%w: %s gives no baselines", integrations.ErrQuery, source)
	}
	return b.Baseline(ctx, q)
}

// Exporters are the sources Export pulls events from (measure --live), each
// with the export kind its events are read as.
var Exporters = map[string]string{"azure": "appinsights", "aws": "aws", "gcp": "gcp"}

// Export pulls the window's raw events from the named cloud source into out
// as JSON lines in the format exports.Read parses as kind, so measure builds
// cohorts from them exactly as from an export file. It returns the kind and
// the exact query. Nothing is sent when the integration is missing,
// incomplete, or write-scoped; the adapter refuses a write-capable
// credential before the data call.
func Export(ctx context.Context, d Deps, source string, w integrations.Window, out io.Writer) (string, string, error) {
	if _, ok := Exporters[source]; !ok {
		return "", "", fmt.Errorf("%w: %q pulls no events; want one of azure, aws, gcp", integrations.ErrQuery, source)
	}
	_, s, err := open(d, source)
	if err != nil {
		return "", "", err
	}
	return s.(exporter).Export(ctx, w, out)
}

// Check resolves the named integration and asks its provider who the key
// belongs to and what it may do: doctor's reachability and scope check. It
// returns the same errors as Baseline for a missing, write-scoped, or
// unset integration, without calling the provider.
func Check(ctx context.Context, d Deps, name string) (integrations.Scope, error) {
	_, s, err := open(d, name)
	if err != nil {
		return integrations.Scope{}, err
	}
	return s.Scope(ctx)
}

// PostHog opens [integrations.posthog] for experiment and readout (WP6.7).
func PostHog(d Deps) (*posthog.Client, error) {
	_, s, err := open(d, "posthog")
	if err != nil {
		return nil, err
	}
	return s.(*posthog.Client), nil
}

// GrowthBook opens [integrations.growthbook] for readout (WP6.7).
func GrowthBook(d Deps) (*growthbook.Client, error) {
	_, s, err := open(d, "growthbook")
	if err != nil {
		return nil, err
	}
	return s.(*growthbook.Client), nil
}

// Figma opens [integrations.figma] for a read verb (research, mockup,
// prototype; WP6.5).
func Figma(d Deps) (*figma.Client, error) {
	_, s, err := open(d, "figma")
	if err != nil {
		return nil, err
	}
	return s.(*figma.Client), nil
}

// Mobbin opens [integrations.mobbin] for a read verb (research, WP6.5).
func Mobbin(d Deps) (*mobbin.Client, error) {
	_, s, err := open(d, "mobbin")
	if err != nil {
		return nil, err
	}
	return s.(*mobbin.Client), nil
}
