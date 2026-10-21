package providers

import (
	"context"
	"strings"
	"testing"

	"github.com/rahulnakmol/agentic-ayudha/internal/integrations/fixture"
)

// TestCapabilities (Decision #449): doctor tries each capability with one
// cheap read, and a failure from a real account names what is wrong and the
// exact fix. The recordings in testdata/doctor are shaped from the providers'
// OpenAPI specs and source (each file's source says which); they are not
// live-proven.
func TestCapabilities(t *testing.T) {
	type want struct {
		ok          bool
		detail, fix string // substrings
	}
	for _, tt := range []struct {
		file, source, auth string
		want               []want
	}{
		{"posthog-works", "posthog", "Bearer phx_test", []want{{true, "project 12345 answered SELECT 1", ""}, {true, "project 12345 listed its insight alerts", ""}}},
		{"posthog-missing-scope", "posthog", "Bearer phx_test", []want{{false, "the key lacks query:read", "https://us.posthog.com/settings/user-api-keys and add the scope query:read"}, {false, "the key lacks alert:read", "add the scope alert:read"}}},
		{"posthog-scope-refused", "posthog", "Bearer phx_test", []want{{false, "the key lacks the scope query:read", "add the scope query:read"}, {false, "the key lacks alert:read", "add the scope alert:read"}}},
		{"posthog-project-access", "posthog", "Bearer phx_test", []want{{false, "the key cannot read project 12345", "allow project 12345 on the key"}, {false, "the key lacks alert:read", "add the scope alert:read"}}},
		{"posthog-wrong-region", "posthog", "Bearer phx_test", []want{{false, "does not accept the key", `set url = "https://eu.posthog.com"`}, {false, "does not accept the key", `set url = "https://eu.posthog.com"`}}},
		{"posthog-unknown-project", "posthog", "Bearer phx_test", []want{{false, "has no project 12345", "Project settings → Project ID"}, {false, "the key lacks alert:read", "add the scope alert:read"}}},
		{"posthog-budget", "posthog", "Bearer phx_test", []want{{false, "hourly query budget is spent", "refills hourly"}, {false, "the key lacks alert:read", "add the scope alert:read"}}},
		{"sentry-works", "sentry", "Bearer sntrys_test", []want{
			{true, "organization acme (https://us.sentry.io) has project web (4507)", ""},
			{true, "last hour of sessions for project 4507", ""},
			{true, "last hour of events for project 4507", ""},
			{true, "last hour of issues for project 4507", ""},
			{true, "listed releases for project 4507", ""},
			{true, "listed detectors for project 4507", ""},
		}},
		{"sentry-region", "sentry", "Bearer sntrys_test", []want{
			{false, "lives in https://de.sentry.io", `set url = "https://de.sentry.io"`},
			{false, "skipped", "organization check first"},
			{false, "skipped", "organization check first"},
			{false, "skipped", "organization check first"},
			{false, "skipped", "organization check first"},
			{false, "skipped", "organization check first"},
		}},
		{"sentry-unknown-org", "sentry", "Bearer sntrys_test", []want{{false, "the token sees no organization acme", "https://<slug>.sentry.io"}, {false, "skipped", ""}, {false, "skipped", ""}, {false, "skipped", ""}, {false, "skipped", ""}, {false, "skipped", ""}}},
		{"sentry-unknown-project", "sentry", "Bearer sntrys_test", []want{{false, "acme has no project with ID 4507", "General Settings"}, {false, "skipped", ""}, {false, "skipped", ""}, {false, "skipped", ""}, {false, "skipped", ""}, {false, "skipped", ""}}},
		{"sentry-missing-scope", "sentry", "Bearer sntrys_test", []want{{false, "the token lacks org:read", "Permissions"}, {false, "lacks org:read", ""}, {false, "lacks org:read", ""}, {false, "skipped", ""}, {false, "skipped", ""}, {false, "the token lacks alerts:read", "give the token alerts:read"}}},
		{"sentry-forbidden", "sentry", "Bearer sntrys_test", []want{{true, "has project web", ""}, {false, "may not read this", "give the token org:read"}, {true, "events", ""}, {true, "issues", ""}, {true, "releases", ""}, {true, "detectors", ""}}},
		{"sentry-invalid-token", "sentry", "Bearer sntrys_test", []want{{false, "does not accept the token (Invalid token)", "aayu auth set-secret sentry"}, {false, "Invalid token", ""}, {false, "Invalid token", ""}, {false, "Invalid token", ""}, {false, "Invalid token", ""}, {false, "Invalid token", ""}}},
	} {
		t.Run(tt.file, func(t *testing.T) {
			rp := &fixture.Replayer{T: t, Rec: fixture.Load(t, "testdata/doctor/"+tt.file+".json"), AuthHeader: "Authorization", AuthValue: tt.auth}
			// sentry-invalid-token answers once: every later check reuses the failed scope read.
			got, err := Capabilities(context.Background(), deps(t, rp, keys, configured...), tt.source)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("%d checks %+v; want %d", len(got), got, len(tt.want))
			}
			for i, w := range tt.want {
				g := got[i]
				if g.OK != w.ok || !strings.Contains(g.Detail, w.detail) || !strings.Contains(g.Fix, w.fix) || (!g.OK && g.Fix == "") {
					t.Errorf("check %d (%s): ok=%v detail %q fix %q; want ok=%v detail ~%q fix ~%q", i, g.Capability, g.OK, g.Detail, g.Fix, w.ok, w.detail, w.fix)
				}
			}
			rp.Done()
			// A recording says where it came from: the provider's spec, its
			// version, the date it was read, and that no account recorded it.
			src := rp.Rec.Source
			spec := map[string]string{"sentry": "getsentry/sentry-api-schema", "posthog": "OpenAPI"}[tt.source]
			if !strings.Contains(src, spec) || !strings.Contains(src, "read 2026-10-17") || !strings.Contains(src, "Not live-proven") {
				t.Errorf("source %q does not cite the %s spec, the date read, and that it is not live-proven", src, spec)
			}
		})
	}
}
