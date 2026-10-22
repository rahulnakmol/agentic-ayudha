package sentry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/integrations"
)

// capability is one thing aayudha reads from Sentry, the token scopes it
// needs, and its cheapest read.
type capability struct {
	name   string
	scopes []string
	verbs  []string
	probe  func(ctx context.Context, c *Client) (string, string, error) // detail, request
}

// capabilities are what this adapter reads today; each phase of Decision
// #449 adds its own. The first proves the organization, its region, and the
// project, so the reads after it fail only for their own reasons.
var capabilities = []capability{
	{"organization, region, and project", []string{"org:read"}, nil, func(ctx context.Context, c *Client) (string, string, error) { return c.probeOrg(ctx) }},
	{"crash-free rates", []string{"org:read"}, []string{"measure", "grill"}, func(ctx context.Context, c *Client) (string, string, error) {
		return c.probeRead(ctx, "sessions", url.Values{"field": {"crash_free_rate(session)"}, "interval": {"1h"}, "includeSeries": {"0"}})
	}},
	{"error counts", []string{"org:read"}, []string{"measure", "grill"}, func(ctx context.Context, c *Client) (string, string, error) {
		return c.probeRead(ctx, "events", url.Values{"field": {"count()"}, "dataset": {"errors"}})
	}},
	// Phase P1 reads: issues with their stack traces, releases with their
	// deploys, and the alerts and monitors a runbook names.
	{"issues and their events", []string{"event:read"}, []string{"investigate"}, func(ctx context.Context, c *Client) (string, string, error) {
		return c.probeRead(ctx, "issues", url.Values{"limit": {"1"}})
	}},
	{"releases and deploys", []string{"project:read"}, []string{"investigate", "slo"}, func(ctx context.Context, c *Client) (string, string, error) {
		return c.probeList(ctx, "releases")
	}},
	{"alerts and monitors", []string{"alerts:read"}, []string{"runbook"}, func(ctx context.Context, c *Client) (string, string, error) {
		return c.probeList(ctx, "detectors")
	}},
}

// Scopes are the read scopes a Sentry token needs for every capability
// doctor checks, sorted: what init and the docs tell a person to grant.
func Scopes() []string {
	var out []string
	for _, cp := range capabilities {
		for _, s := range cp.scopes {
			if !slices.Contains(out, s) {
				out = append(out, s)
			}
		}
	}
	slices.Sort(out)
	return out
}

// organization is one entry of GET /api/0/organizations/: its slug and the
// region its data lives in.
type organization struct {
	Slug  string `json:"slug"`
	Links struct {
		RegionURL string `json:"regionUrl"`
	} `json:"links"`
}

// regionError: the organization lives in another region than the configured URL.
type regionError struct{ org, region string }

func (e *regionError) Error() string { return "organization " + e.org + " lives in " + e.region }

// projectError: the organization has no project with the configured ID.
type projectError struct{ org, project string }

func (e *projectError) Error() string {
	return "organization " + e.org + " has no project with ID " + e.project
}

// orgError: the token sees no organization with the configured slug.
type orgError struct{ org string }

func (e *orgError) Error() string { return "the token sees no organization " + e.org }

// probeOrg lists the organizations the token sees (each with its region), then
// the organization's projects, and checks the configured ones are there.
func (c *Client) probeOrg(ctx context.Context) (string, string, error) {
	u := c.base + "/api/0/organizations/"
	var orgs []organization
	if err := c.get(ctx, u, &orgs); err != nil {
		return "", "GET " + u, err
	}
	i := slices.IndexFunc(orgs, func(o organization) bool { return o.Slug == c.org })
	if i < 0 {
		return "", "GET " + u, &orgError{c.org}
	}
	if r := orgs[i].Links.RegionURL; strings.Contains(r, "de.sentry.io") && !strings.Contains(c.base, "de.sentry.io") {
		return "", "GET " + u, &regionError{c.org, r}
	}
	pu := c.base + "/api/0/organizations/" + c.org + "/projects/"
	var projects []struct {
		ID   string `json:"id"`
		Slug string `json:"slug"`
	}
	if err := c.get(ctx, pu, &projects); err != nil {
		return "", "GET " + pu, err
	}
	for _, p := range projects {
		if p.ID == c.project {
			return fmt.Sprintf("organization %s (%s) has project %s (%s)", c.org, orgs[i].Links.RegionURL, p.Slug, p.ID), "GET " + pu, nil
		}
	}
	return "", "GET " + pu, &projectError{c.org, c.project}
}

// probeRead reads the last hour of one endpoint for the project.
func (c *Client) probeRead(ctx context.Context, endpoint string, v url.Values) (string, string, error) {
	v.Set("project", c.project)
	v.Set("statsPeriod", "1h")
	u := c.base + "/api/0/organizations/" + c.org + "/" + endpoint + "/?" + v.Encode()
	var res json.RawMessage
	if err := c.get(ctx, u, &res); err != nil {
		return "", "GET " + u, err
	}
	return "read the last hour of " + endpoint + " for project " + c.project, "GET " + u, nil
}

// probeList reads one entry of an endpoint's list for the project.
func (c *Client) probeList(ctx context.Context, endpoint string) (string, string, error) {
	u := c.base + "/api/0/organizations/" + c.org + "/" + endpoint + "/?" + url.Values{"project": {c.project}, "per_page": {"1"}}.Encode()
	var res json.RawMessage
	if err := c.get(ctx, u, &res); err != nil {
		return "", "GET " + u, err
	}
	return "listed " + endpoint + " for project " + c.project, "GET " + u, nil
}

// Capabilities checks each capability for doctor: the token's scopes first
// (a missing scope fails with no call), then one cheap read each. When the
// organization check fails, the reads after it are skipped: they would fail
// for the same reason.
func (c *Client) Capabilities(ctx context.Context) []integrations.Check {
	scope, scopeErr := c.Scope(ctx)
	var out []integrations.Check
	setupFailed := false
	for _, cp := range capabilities {
		ch := integrations.Check{Capability: cp.name, Scopes: cp.scopes, Verbs: cp.verbs}
		var missing []string
		if scopeErr == nil {
			for _, s := range cp.scopes {
				if !slices.Contains(scope.Scopes, s) && !slices.Contains(scope.Scopes, strings.TrimSuffix(s, ":read")+":write") {
					missing = append(missing, s)
				}
			}
		}
		switch {
		case scopeErr != nil:
			ch.Detail, ch.Fix = c.Explain(scopeErr, cp.scopes)
		case len(missing) > 0:
			ch.Detail = "the token lacks " + strings.Join(missing, ", ")
			ch.Fix = addScopes(missing)
			setupFailed = setupFailed || cp.name == capabilities[0].name
		case setupFailed:
			ch.Detail = "skipped: the organization check above failed"
			ch.Fix = "fix the organization check first"
		default:
			detail, req, err := cp.probe(ctx, c)
			ch.Request = req
			if err != nil {
				ch.Detail, ch.Fix = c.Explain(err, cp.scopes)
				setupFailed = setupFailed || cp.name == capabilities[0].name
			} else {
				ch.OK, ch.Detail = true, detail
			}
		}
		out = append(out, ch)
	}
	return out
}

func addScopes(scopes []string) string {
	return "give the token " + strings.Join(scopes, ", ") + ": in Sentry, Settings → Developer Settings → Custom Integrations → your internal integration → Permissions, set the matching resource to Read (never Write or Admin), save, and run `aayu auth set-secret sentry` if the token changed"
}

// Explain turns a failed Sentry read into what is wrong with the setup and
// the exact fix. Sentry answers errors as {"detail": "…"}: "Invalid token"
// (401, src/sentry/api/authentication.py), "You do not have permission to
// perform this action." (403), and "The requested resource does not exist"
// (404).
func (c *Client) Explain(err error, scopes []string) (problem, fix string) {
	var re *regionError
	var pe *projectError
	var oe *orgError
	var he *integrations.HTTPError
	switch {
	case errors.As(err, &re):
		return fmt.Sprintf("organization %s lives in %s, but [integrations.sentry] reads %s", re.org, re.region, c.base),
			fmt.Sprintf("set url = %q in [integrations.sentry]", re.region)
	case errors.As(err, &pe):
		return pe.Error(), "set project = to the numeric ID in Sentry: Settings → Projects → your project → General Settings (the Project ID), or the project= value in an issue URL"
	case errors.As(err, &oe):
		return oe.Error(), fmt.Sprintf("set org = to the slug in your Sentry URL (https://<slug>.sentry.io), and make sure the token belongs to that organization; an EU organization also needs url = %q", "https://de.sentry.io")
	case !errors.As(err, &he):
		return err.Error(), "check the network and [integrations.sentry] url, then run `aayu doctor` again"
	}
	var body struct {
		Detail string `json:"detail"`
	}
	_ = json.Unmarshal(he.Body, &body)
	switch he.Status {
	case http.StatusUnauthorized:
		return fmt.Sprintf("Sentry at %s does not accept the token (%s)", c.base, body.Detail),
			"create a read-only internal integration token (Settings → Developer Settings → Custom Integrations) and run `aayu auth set-secret sentry`; a token from an EU organization also needs url = \"https://de.sentry.io\""
	case http.StatusForbidden:
		return "the token may not read this (" + body.Detail + ")", addScopes(scopes)
	case http.StatusNotFound:
		return fmt.Sprintf("Sentry at %s has no organization %s or no such resource (%s)", c.base, c.org, body.Detail),
			"check org = (the slug in https://<slug>.sentry.io) and the region: an EU organization needs url = \"https://de.sentry.io\""
	case http.StatusTooManyRequests:
		return "Sentry is rate-limiting this token (" + he.Header.Get("X-Sentry-Rate-Limit-Reset") + ")", "wait for X-Sentry-Rate-Limit-Reset and run `aayu doctor` again"
	}
	if he.Status >= 500 {
		return "Sentry answered " + he.StatusText, "Sentry is failing, not your setup: try again later (status.sentry.io)"
	}
	return strings.TrimSpace("Sentry answered " + he.StatusText + ": " + body.Detail), "check [integrations.sentry] and the token, then run `aayu doctor` again"
}
