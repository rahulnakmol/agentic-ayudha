package initrepo

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/integrations/posthog"
	"github.com/rahulnakmol/agentic-ayudha/internal/integrations/sentry"
)

// TestIntegrationScopes: the scopes init --integrations tells a person to
// grant are exactly the ones aayu doctor checks, and Sentry's are the four
// Decision #449 needs (org:read for the organization's sessions and events,
// event:read for issues, project:read for releases, alerts:read for alerts).
func TestIntegrationScopes(t *testing.T) {
	if got, want := sentry.Scopes(), []string{"alerts:read", "event:read", "org:read", "project:read"}; !slices.Equal(got, want) {
		t.Errorf("doctor checks Sentry scopes %v; want %v", got, want)
	}
	if got, want := posthog.Scopes(), []string{"alert:read", "query:read"}; !slices.Equal(got, want) {
		t.Errorf("doctor checks PostHog scopes %v; want %v", got, want)
	}
	listed := regexp.MustCompile(`\(([a-z_]+:read(?:, [a-z_]+:read)*)\)`)
	for name, doctor := range map[string][]string{"sentry": sentry.Scopes(), "posthog": posthog.Scopes()} {
		i := slices.IndexFunc(Integrations, func(ig Integration) bool { return ig.Name == name })
		if i < 0 {
			t.Fatalf("init offers no %s integration", name)
		}
		m := listed.FindStringSubmatch(Integrations[i].Summary)
		if m == nil || !slices.Equal(strings.Split(m[1], ", "), doctor) {
			t.Errorf("init's %s summary %q names scopes %v; doctor checks %v", name, Integrations[i].Summary, m, doctor)
		}
	}
}
