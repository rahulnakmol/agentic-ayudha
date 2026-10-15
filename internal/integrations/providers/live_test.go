package providers

import (
	"bytes"
	"context"
	"maps"
	"os"
	"strings"
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

// TestLiveReadout runs the readout reads (WP6.7) against the real providers,
// behind AAYU_LIVE=1, skipping each one whose settings are absent:
//
//	growthbook: AAYU_SECRET_GROWTHBOOK (a readonly secret key), optional AAYU_LIVE_GROWTHBOOK_URL;
//	            with AAYU_LIVE_GROWTHBOOK_TRACKING_KEY and AAYU_LIVE_GROWTHBOOK_METRIC also reads results
//	posthog:    AAYU_SECRET_POSTHOG, AAYU_LIVE_POSTHOG_PROJECT, AAYU_LIVE_POSTHOG_FLAG, AAYU_LIVE_POSTHOG_EVENT
func TestLiveReadout(t *testing.T) {
	if os.Getenv("AAYU_LIVE") != "1" {
		t.Skip("set AAYU_LIVE=1 to call the real providers")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	need := func(t *testing.T, names ...string) {
		for _, n := range names {
			if os.Getenv(n) == "" {
				t.Skipf("%s is not set", n)
			}
		}
	}
	t.Run("growthbook", func(t *testing.T) {
		need(t, "AAYU_SECRET_GROWTHBOOK")
		sets := []string{"integrations.growthbook.secret=growthbook", "integrations.growthbook.scope=read"}
		if u := os.Getenv("AAYU_LIVE_GROWTHBOOK_URL"); u != "" {
			sets = append(sets, "integrations.growthbook.url="+u)
		}
		d := deps(t, nil, map[string]string{"AAYU_SECRET_GROWTHBOOK": os.Getenv("AAYU_SECRET_GROWTHBOOK")}, sets...)
		if _, err := Check(ctx, d, "growthbook"); err != nil {
			t.Fatal(err)
		}
		need(t, "AAYU_LIVE_GROWTHBOOK_TRACKING_KEY", "AAYU_LIVE_GROWTHBOOK_METRIC")
		gb, err := GrowthBook(d)
		if err != nil {
			t.Fatal(err)
		}
		vs, w, q, err := gb.Results(ctx, os.Getenv("AAYU_LIVE_GROWTHBOOK_TRACKING_KEY"), os.Getenv("AAYU_LIVE_GROWTHBOOK_METRIC"))
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("%d variations over %s to %s: %+v (%s)", len(vs), w.From, w.To, vs, q)
	})
	t.Run("posthog", func(t *testing.T) {
		need(t, "AAYU_SECRET_POSTHOG", "AAYU_LIVE_POSTHOG_PROJECT", "AAYU_LIVE_POSTHOG_FLAG", "AAYU_LIVE_POSTHOG_EVENT")
		d := deps(t, nil, map[string]string{"AAYU_SECRET_POSTHOG": os.Getenv("AAYU_SECRET_POSTHOG")},
			"integrations.posthog.secret=posthog", "integrations.posthog.scope=read", "integrations.posthog.project="+os.Getenv("AAYU_LIVE_POSTHOG_PROJECT"))
		d.Now = nil
		ph, err := PostHog(d)
		if err != nil {
			t.Fatal(err)
		}
		now := time.Now().UTC().Truncate(24 * time.Hour)
		arms, q, err := ph.Arms(ctx, os.Getenv("AAYU_LIVE_POSTHOG_FLAG"), os.Getenv("AAYU_LIVE_POSTHOG_EVENT"), integrations.Window{From: now.AddDate(0, 0, -28), To: now})
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("arms %+v (%s)", arms, q)
	})
}

// TestLiveCloud runs the cloud adapters (WP7.2b) against real accounts,
// behind AAYU_LIVE=1, skipping each cloud whose settings are absent. Each
// takes AAYU_LIVE_<CLOUD>_AUTH (key, the default; oidc inside a GitHub
// Actions job with id-token: write; or cli with az, aws, or gcloud logged
// in) and checks scope, one baseline, and a live event pull over the last
// seven days:
//
//	azure: AAYU_LIVE_AZURE_RESOURCE; key and oidc need AAYU_LIVE_AZURE_TENANT; key AAYU_SECRET_AZURE
//	       (<client-id>:<client-secret>); oidc AAYU_LIVE_AZURE_CLIENT
//	aws:   AAYU_LIVE_AWS_MONITOR, AAYU_LIVE_AWS_REGION; key AAYU_SECRET_AWS (ACCESS_KEY_ID:SECRET);
//	       oidc AAYU_LIVE_AWS_ROLE
//	gcp:   AAYU_LIVE_GCP_DATASET (<project>.<dataset>); key AAYU_SECRET_GCP (service account JSON);
//	       oidc AAYU_LIVE_GCP_PROVIDER and AAYU_LIVE_GCP_SA; optional AAYU_LIVE_GCP_SA with cli
func TestLiveCloud(t *testing.T) {
	if os.Getenv("AAYU_LIVE") != "1" {
		t.Skip("set AAYU_LIVE=1 to call the real clouds")
	}
	now := time.Now().UTC().Truncate(24 * time.Hour)
	week := integrations.Window{From: now.AddDate(0, 0, -7), To: now}
	for _, tt := range []struct {
		source string
		sets   map[string]string // config key -> environment variable
		byAuth map[string]map[string]string
	}{
		{"azure", map[string]string{"project": "AAYU_LIVE_AZURE_RESOURCE"}, map[string]map[string]string{
			"key": {"org": "AAYU_LIVE_AZURE_TENANT", "": "AAYU_SECRET_AZURE"}, "oidc": {"org": "AAYU_LIVE_AZURE_TENANT", "identity": "AAYU_LIVE_AZURE_CLIENT"}}},
		{"aws", map[string]string{"project": "AAYU_LIVE_AWS_MONITOR", "region": "AAYU_LIVE_AWS_REGION"}, map[string]map[string]string{
			"key": {"": "AAYU_SECRET_AWS"}, "oidc": {"identity": "AAYU_LIVE_AWS_ROLE"}}},
		{"gcp", map[string]string{"project": "AAYU_LIVE_GCP_DATASET"}, map[string]map[string]string{
			"key": {"": "AAYU_SECRET_GCP"}, "oidc": {"federation": "AAYU_LIVE_GCP_PROVIDER", "identity": "AAYU_LIVE_GCP_SA"}}},
	} {
		t.Run(tt.source, func(t *testing.T) {
			up := strings.ToUpper(tt.source)
			auth := os.Getenv("AAYU_LIVE_" + up + "_AUTH")
			if auth == "" {
				auth = "key"
			}
			sets := []string{"integrations." + tt.source + ".scope=read", "integrations." + tt.source + ".auth=" + auth}
			if auth == "key" {
				sets = append(sets, "integrations."+tt.source+".secret="+tt.source)
			}
			if sa := os.Getenv("AAYU_LIVE_GCP_SA"); tt.source == "gcp" && auth == "cli" && sa != "" {
				sets = append(sets, "integrations.gcp.identity="+sa)
			}
			env := map[string]string{}
			need := map[string]string{}
			maps.Copy(need, tt.sets)
			maps.Copy(need, tt.byAuth[auth])
			for k, v := range need {
				if os.Getenv(v) == "" {
					t.Skipf("%s is not set", v)
				}
				if k == "" {
					env[v] = os.Getenv(v)
					continue
				}
				sets = append(sets, "integrations."+tt.source+"."+k+"="+os.Getenv(v))
			}
			d := deps(t, nil, env, sets...)
			d.Now, d.Getenv = nil, os.Getenv
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			sc, err := Check(ctx, d, tt.source)
			if err != nil {
				t.Fatal(err)
			}
			if len(sc.Write) > 0 {
				t.Fatalf("the live credential is write-capable (%v); use the read-only setup in .aayudha/setup/%s/", sc.Write, tt.source)
			}
			t.Logf("%s reachable as %s (verified %t: %v)", tt.source, sc.Account, sc.Verified, sc.Scopes)
			b, err := Baseline(ctx, d, tt.source, integrations.Query{Metric: "users", Window: week})
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("%s users = %v (%s)", tt.source, b.Value, b.Population)
			var buf bytes.Buffer
			kind, q, err := Export(ctx, d, tt.source, week, &buf)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("%s pulled %d bytes of %s events (%s)", tt.source, buf.Len(), kind, q)
		})
	}
}
