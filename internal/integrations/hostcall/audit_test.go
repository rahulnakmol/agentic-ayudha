package hostcall

import (
	"strings"
	"testing"
)

func TestAudit(t *testing.T) {
	for _, tt := range []struct {
		name     string
		s        Server
		provider string
		want     []string // substrings of the problems, in order; none means set up to read only
		secret   bool
	}{
		{"posthog read-only by header and pinned", Server{URL: "https://mcp.posthog.com/mcp?mode=cli", Headers: map[string]string{"X-PostHog-Read-Only": "1", "x-posthog-project-id": "12345"}}, PostHog, nil, false},
		{"posthog read-only by query, not pinned", Server{URL: "https://mcp.posthog.com/mcp?readonly=true"}, PostHog, []string{"not pinned"}, false},
		{"posthog key in a personal file is not a shared secret", Server{URL: "https://mcp.posthog.com/mcp?readonly=true&project_id=1", Headers: map[string]string{"Authorization": "Bearer phx_0123456789abcdefABCDEF"}}, PostHog, nil, false},
		{"posthog key in a committed file", Server{Committed: true, URL: "https://mcp.posthog.com/mcp?readonly=true&project_id=1", Headers: map[string]string{"Authorization": "Bearer phx_0123456789abcdefABCDEF"}}, PostHog, []string{"key is written"}, true},
		{"posthog env reference in a committed file", Server{Committed: true, URL: "https://mcp.posthog.com/mcp?readonly=true&project_id=1", Headers: map[string]string{"Authorization": "Bearer ${POSTHOG_API_KEY}"}}, PostHog, nil, false},
		{"sentry hosted, scoped, with a token", Server{URL: "https://mcp.sentry.dev/mcp/acme/web?skills=inspect", Headers: map[string]string{"Authorization": "Bearer ${SENTRY_TOKEN}"}}, Sentry, nil, false},
		{"sentry hosted on OAuth", Server{URL: "https://mcp.sentry.dev/mcp/acme?skills=inspect"}, Sentry, []string{"OAuth"}, false},
		{"sentry stdio with skills from the environment", Server{Command: "npx", Args: []string{"-y", "@sentry/mcp-server"}, Env: map[string]string{"MCP_SKILLS": "inspect"}}, Sentry, nil, false},
		{"sentry stdio with seer", Server{Command: "npx", Args: []string{"@sentry/mcp-server", "--skills", "inspect,seer"}}, Sentry, []string{"beyond inspect"}, false},
		{"neither", Server{URL: "https://api.githubcopilot.com/mcp/"}, "", nil, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p, fs := Audit(tt.s)
			if p != tt.provider || len(fs) != len(tt.want) {
				t.Fatalf("Audit = %q %+v; want %q with %d findings", p, fs, tt.provider, len(tt.want))
			}
			secret := false
			for i, f := range fs {
				secret = secret || f.Secret
				if !strings.Contains(f.Problem, tt.want[i]) || f.Fix == "" {
					t.Errorf("finding %d: %+v; want a problem with %q and a fix", i, f, tt.want[i])
				}
			}
			if secret != tt.secret {
				t.Errorf("secret = %v; want %v", secret, tt.secret)
			}
		})
	}
}
