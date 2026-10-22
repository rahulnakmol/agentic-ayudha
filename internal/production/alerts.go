package production

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
)

// AlertState is what a runbook's alert check found for one alert name.
type AlertState string

// The alert check's results. Without a configured provider an alert is not
// checked, never missing: a fixture run and a live run stay distinguishable.
const (
	AlertFound      AlertState = "found"
	AlertMissing    AlertState = "missing"
	AlertNotChecked AlertState = "not checked"
)

// AlertStatus is one alert a runbook's Symptom names and where it was found.
type AlertStatus struct {
	Name  string     `json:"name"`
	State AlertState `json:"state"`
	Where string     `json:"where,omitempty"` // the provider that has it
}

// RunbookAlerts checks the alerts a runbook's Symptom names in backticks
// (every name that is not an SLO in ops/slos.toml) against the alerts each
// configured provider has (Decision #449, phase P1): found maps a provider to
// the slugs of its alerts and monitors. With no provider, each alert is not
// checked.
func RunbookAlerts(md string, slos *SLOs, found map[string][]string) []AlertStatus {
	symptom, _ := Part(md, "Symptom")
	var out []AlertStatus
	for _, m := range backticked.FindAllStringSubmatch(symptom, -1) {
		name := m[1]
		if slos != nil && slices.ContainsFunc(slos.SLOs, func(o SLO) bool { return o.Name == name }) {
			continue
		}
		if slices.ContainsFunc(out, func(a AlertStatus) bool { return a.Name == name }) {
			continue
		}
		st := AlertStatus{Name: name, State: AlertNotChecked}
		if len(found) > 0 {
			st.State = AlertMissing
			providers := make([]string, 0, len(found))
			for p := range found {
				providers = append(providers, p)
			}
			slices.Sort(providers)
			for _, p := range providers {
				if slices.Contains(found[p], name) {
					st.State, st.Where = AlertFound, p
					break
				}
			}
		}
		out = append(out, st)
	}
	return out
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// Slug is the form a runbook names an alert in: lower case, words joined by
// hyphens ("High error rate" is `high-error-rate`).
func Slug(name string) string {
	return strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(name), "-"), "-")
}

// Alerts lists the slugs of the project's alerts and monitors: its detectors,
// the organization's workflows, and its cron monitors (by name and by slug).
func (s Sentry) Alerts(ctx context.Context) ([]string, Provenance, error) {
	pv := Provenance{Source: "sentry project " + s.API.Project()}
	h := sha256.New()
	var out []string
	for _, r := range []struct {
		path string
		v    url.Values
	}{
		{"/detectors/", url.Values{"project": {s.API.Project()}}},
		{"/workflows/", nil},
		{"/monitors/", url.Values{"project": {s.API.Project()}}},
	} {
		b, req, err := s.API.Get(ctx, r.path, r.v)
		pv.Requests = append(pv.Requests, req)
		if err != nil {
			return nil, pv, err
		}
		fmt.Fprintf(h, "%s\n%s\n", req, b)
		var items []struct {
			Name string `json:"name"`
			Slug string `json:"slug"`
		}
		if err := json.Unmarshal(b, &items); err != nil {
			return nil, pv, fmt.Errorf("%w: undecodable %s answer: %w", errSource, strings.Trim(r.path, "/"), err)
		}
		for _, it := range items {
			out = append(out, Slug(it.Name))
			if it.Slug != "" {
				out = append(out, Slug(it.Slug))
			}
		}
	}
	pv.Digest = hex.EncodeToString(h.Sum(nil))
	return out, pv, nil
}

// PostHogLister reads one list under a PostHog project (posthog.Client).
type PostHogLister interface {
	Get(ctx context.Context, resource string, v url.Values) (body []byte, request string, err error)
}

// PostHogAlerts lists the slugs of a PostHog project's insight alerts.
func PostHogAlerts(ctx context.Context, api PostHogLister, project string) ([]string, Provenance, error) {
	pv := Provenance{Source: "posthog project " + project}
	b, req, err := api.Get(ctx, "alerts", url.Values{"limit": {"100"}})
	pv.Requests = append(pv.Requests, req)
	if err != nil {
		return nil, pv, err
	}
	sum := sha256.Sum256([]byte(req + "\n" + string(b) + "\n"))
	pv.Digest = hex.EncodeToString(sum[:])
	var body struct {
		Results []struct {
			Name string `json:"name"`
		} `json:"results"`
	}
	if err := json.Unmarshal(b, &body); err != nil {
		return nil, pv, fmt.Errorf("%w: undecodable alerts answer: %w", errSource, err)
	}
	var out []string
	for _, a := range body.Results {
		out = append(out, Slug(a.Name))
	}
	return out, pv, nil
}
