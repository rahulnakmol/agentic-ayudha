package providers

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/integrations"
)

// TestLive runs each adapter against the real provider, behind AAYU_LIVE=1. A
// provider whose read-only key (AAYU_SECRET_<NAME>) or project settings are
// absent is skipped:
//
//	posthog: AAYU_SECRET_POSTHOG, AAYU_LIVE_POSTHOG_PROJECT, optional AAYU_LIVE_POSTHOG_URL
//	sentry:  AAYU_SECRET_SENTRY, AAYU_LIVE_SENTRY_ORG, AAYU_LIVE_SENTRY_PROJECT, optional AAYU_LIVE_SENTRY_URL
//	github:  AAYU_SECRET_GITHUB_READ, optional AAYU_LIVE_GITHUB_REPO (default rahulnakmol/agentic-ayudha)
func TestLive(t *testing.T) {
	if os.Getenv("AAYU_LIVE") != "1" {
		t.Skip("set AAYU_LIVE=1 to call the real providers")
	}
	now := time.Now().UTC().Truncate(24 * time.Hour)
	week := integrations.Window{From: now.AddDate(0, 0, -7), To: now}
	for _, tt := range []struct {
		source, secret, metric string
		need                   []string // environment variables, in the order of sets
		sets                   []string
	}{
		{"posthog", "AAYU_SECRET_POSTHOG", "events", []string{"AAYU_LIVE_POSTHOG_PROJECT"}, []string{"project"}},
		{"sentry", "AAYU_SECRET_SENTRY", "errors", []string{"AAYU_LIVE_SENTRY_ORG", "AAYU_LIVE_SENTRY_PROJECT"}, []string{"org", "project"}},
		{"github", "AAYU_SECRET_GITHUB_READ", "issues_opened", nil, nil},
	} {
		t.Run(tt.source, func(t *testing.T) {
			if os.Getenv(tt.secret) == "" {
				t.Skipf("%s is not set", tt.secret)
			}
			secret := map[string]string{"posthog": "posthog", "sentry": "sentry", "github": "github-read"}[tt.source]
			sets := []string{"integrations." + tt.source + ".secret=" + secret, "integrations." + tt.source + ".scope=read"}
			for i, v := range tt.need {
				if os.Getenv(v) == "" {
					t.Skipf("%s is not set", v)
				}
				sets = append(sets, "integrations."+tt.source+"."+tt.sets[i]+"="+os.Getenv(v))
			}
			if u := os.Getenv("AAYU_LIVE_" + map[string]string{"posthog": "POSTHOG", "sentry": "SENTRY", "github": "GITHUB"}[tt.source] + "_URL"); u != "" {
				sets = append(sets, "integrations."+tt.source+".url="+u)
			}
			if tt.source == "github" {
				repo := os.Getenv("AAYU_LIVE_GITHUB_REPO")
				if repo == "" {
					repo = "rahulnakmol/agentic-ayudha"
				}
				sets = append(sets, "integrations.github.project="+repo)
			}
			d := deps(t, nil, map[string]string{tt.secret: os.Getenv(tt.secret)}, sets...)
			d.Now = nil
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			sc, err := Check(ctx, d, tt.source)
			if err != nil {
				t.Fatal(err)
			}
			if len(sc.Write) > 0 {
				t.Fatalf("the live key is write-capable (%v); use a read-only key", sc.Write)
			}
			b, err := Baseline(ctx, d, tt.source, integrations.Query{Metric: tt.metric, Window: week})
			if err != nil {
				t.Fatal(err)
			}
			if b.Value < 0 || b.SourceQuery == "" || b.FetchedAt.IsZero() || !b.Window.From.Equal(week.From) {
				t.Fatalf("baseline %+v", b)
			}
			t.Logf("%s %s = %v (%s)", tt.source, b.Metric, b.Value, b.Population)
		})
	}
}
