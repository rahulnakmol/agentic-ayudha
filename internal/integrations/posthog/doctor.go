package posthog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/integrations"
)

// capability is one thing aayudha reads from PostHog, the read scopes it
// needs, and its cheapest read.
type capability struct {
	name   string
	scopes []string
	verbs  []string
	probe  func(ctx context.Context, c *Client) (string, string, error) // detail, request
}

// capabilities are what this adapter reads today; each phase of Decision
// #449 adds its own.
var capabilities = []capability{
	{"HogQL queries", []string{"query:read"}, []string{"measure", "grill", "experiment", "readout"},
		func(ctx context.Context, c *Client) (string, string, error) { return c.probeQuery(ctx) }},
}

// Scopes are the read scopes a PostHog personal API key needs for every
// capability doctor checks, sorted: what init and the docs tell a person to grant.
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

// probeQuery runs SELECT 1: it proves the key, the project, and the query
// endpoint, and reads no events.
func (c *Client) probeQuery(ctx context.Context) (string, string, error) {
	path := "/api/projects/" + c.project + "/query/"
	body, _ := json.Marshal(map[string]any{"query": map[string]string{"kind": "HogQLQuery", "query": "SELECT 1"}, "name": "aayudha doctor"})
	req, err := c.request(ctx, http.MethodPost, path, body)
	if err != nil {
		return "", "", err
	}
	var res struct {
		Results [][]any `json:"results"`
	}
	src := "POST " + c.base + path + " SELECT 1"
	if err := integrations.DoJSON(c.hc, req, &res); err != nil {
		return "", src, err
	}
	if len(res.Results) != 1 {
		return "", src, fmt.Errorf("%w: SELECT 1 returned %d rows", integrations.ErrProvider, len(res.Results))
	}
	return fmt.Sprintf("project %s answered SELECT 1", c.project), src, nil
}

// Capabilities checks each capability for doctor: the key's scopes first (a
// missing scope fails with no call), then one cheap read each.
func (c *Client) Capabilities(ctx context.Context) []integrations.Check {
	scope, scopeErr := c.Scope(ctx)
	var out []integrations.Check
	for _, cp := range capabilities {
		ch := integrations.Check{Capability: cp.name, Scopes: cp.scopes, Verbs: cp.verbs}
		if scopeErr != nil {
			ch.Detail, ch.Fix = c.Explain(scopeErr)
			out = append(out, ch)
			continue
		}
		if missing := missingScopes(scope.Scopes, cp.scopes); len(missing) > 0 {
			ch.Detail = "the key lacks " + strings.Join(missing, ", ")
			ch.Fix = c.addScopes(missing)
			out = append(out, ch)
			continue
		}
		detail, req, err := cp.probe(ctx, c)
		ch.Request = req
		if err != nil {
			ch.Detail, ch.Fix = c.Explain(err)
		} else {
			ch.OK, ch.Detail = true, detail
		}
		out = append(out, ch)
	}
	return out
}

// missingScopes are the wanted scopes the key does not hold. A key with "*"
// holds every scope (doctor refuses it as write-capable anyway).
func missingScopes(have, want []string) []string {
	if slices.Contains(have, "*") {
		return nil
	}
	var out []string
	for _, w := range want {
		obj := strings.TrimSuffix(w, ":read")
		if !slices.Contains(have, w) && !slices.Contains(have, obj+":write") {
			out = append(out, w)
		}
	}
	return out
}

func (c *Client) addScopes(scopes []string) string {
	return fmt.Sprintf("edit the key at %s/settings/user-api-keys and add the scope %s (read only), or create a read-only key with it and run `aayu auth set-secret posthog`",
		c.base, strings.Join(scopes, ", "))
}

var (
	missingScope  = regexp.MustCompile(`API key missing required scope '([a-z_]+:[a-z]+)'`)
	projectAccess = regexp.MustCompile(`does not have access to the requested project`)
)

// Explain turns a failed PostHog read into what is wrong with the setup and
// the exact fix. PostHog answers errors as {"type", "code", "detail",
// "attr"} (posthog/exceptions.py, drf-exceptions-hog).
func (c *Client) Explain(err error) (problem, fix string) {
	var he *integrations.HTTPError
	if !errors.As(err, &he) {
		return err.Error(), "check the network and [integrations.posthog] url, then run `aayu doctor` again"
	}
	var body struct {
		Type, Code, Detail string
	}
	_ = json.Unmarshal(he.Body, &body)
	region := "https://us.posthog.com"
	if strings.Contains(c.base, "us.posthog.com") {
		region = "https://eu.posthog.com"
	}
	switch {
	case he.Status == http.StatusUnauthorized:
		return fmt.Sprintf("PostHog at %s does not accept the key (%s)", c.base, body.Detail),
			fmt.Sprintf("a PostHog key works only in its own region: if the project is on the other cloud, set url = %q in [integrations.posthog]; otherwise create a new read-only personal API key and run `aayu auth set-secret posthog`", region)
	case he.Status == http.StatusForbidden && missingScope.MatchString(body.Detail):
		s := missingScope.FindStringSubmatch(body.Detail)[1]
		return "the key lacks the scope " + s, c.addScopes([]string{s})
	case he.Status == http.StatusForbidden && projectAccess.MatchString(body.Detail):
		return fmt.Sprintf("the key cannot read project %s", c.project),
			fmt.Sprintf("allow project %s on the key at %s/settings/user-api-keys, or set project = to a project the key can read (Project settings → Project ID)", c.project, c.base)
	case he.Status == http.StatusNotFound:
		return fmt.Sprintf("PostHog at %s has no project %s", c.base, c.project),
			fmt.Sprintf("set project = to the numeric ID in Project settings → Project ID; if the project is on the other cloud, set url = %q", region)
	case he.Status == http.StatusTooManyRequests || body.Code == "api_queries_budget_exceeded":
		return "PostHog is rate-limiting this key or the project's hourly query budget is spent (" + body.Detail + ")",
			"wait and run `aayu doctor` again: the query budget refills hourly; aayudha's reads share it with the team's own API use"
	case he.Status == http.StatusPaymentRequired:
		return "PostHog refused the read for billing (" + body.Detail + ")", "check the organization's billing and quota limits in PostHog"
	case he.Status >= 500:
		return fmt.Sprintf("PostHog answered %s", he.StatusText), "PostHog is failing, not your setup: try again later (status.posthog.com)"
	}
	return strings.TrimSpace(fmt.Sprintf("PostHog answered %s: %s", he.StatusText, body.Detail)),
		"check [integrations.posthog] and the key, then run `aayu doctor` again"
}
