package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fixture lays out a user dir and a repo dir and returns Sources pointing at them.
func fixture(t *testing.T, files map[string]string) Sources {
	t.Helper()
	root := t.TempDir()
	src := Sources{UserDir: filepath.Join(root, "home", ".config", "aayudha"), RepoDir: filepath.Join(root, "repo")}
	for rel, body := range files {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return src
}

func TestPrecedence(t *testing.T) {
	// One key, a distinct value in every layer. Each row removes the layers above
	// the expected winner, so a layer that is read in the wrong order or skipped fails.
	user := map[string]string{"home/.config/aayudha/config.toml": "version = 1\n[budget]\nper_day_usd = 1.0\n"}
	repo := map[string]string{"repo/.aayudha/config.toml": "version = 1\n[budget]\nper_day_usd = 2.0\n"}
	local := map[string]string{"repo/.aayudha/config.local.toml": "version = 1\n[budget]\nper_day_usd = 3.0\n"}
	env := []string{"AAYU_BUDGET__PER_DAY_USD=4", "AAYU_AGENT=1", "AAYU_SECRET_X__Y=not-config"}
	set := []string{"budget.per_day_usd=5"}

	merge := func(ms ...map[string]string) map[string]string {
		out := map[string]string{}
		for _, m := range ms {
			for k, v := range m {
				out[k] = v
			}
		}
		return out
	}
	tests := []struct {
		name       string
		files      map[string]string
		env, set   []string
		want       float64
		wantLayer  string
		wantSource string
	}{
		{"flag beats env", merge(user, repo, local), env, set, 5, LayerFlag, "--set budget.per_day_usd=5"},
		{"env beats local", merge(user, repo, local), env, nil, 4, LayerEnv, "AAYU_BUDGET__PER_DAY_USD"},
		{"local beats repo", merge(user, repo, local), nil, nil, 3, LayerLocal, "config.local.toml"},
		{"repo beats user", merge(user, repo), nil, nil, 2, LayerRepo, filepath.Join(".aayudha", "config.toml")},
		{"user beats default", user, nil, nil, 1, LayerUser, filepath.Join(".config", "aayudha", "config.toml")},
		{"default", nil, nil, nil, 100, LayerDefault, "defaults.toml (embedded)"},
		{"repo TOML beats repo JSON", merge(repo, map[string]string{"repo/.aayudha/config.json": `{"version": 1, "budget": {"per_day_usd": 7}}`}), nil, nil, 2, LayerRepo, "config.toml"},
		{"repo JSON alone is accepted", map[string]string{"repo/.aayudha/config.json": `{"version": 1, "budget": {"per_day_usd": 7}}`}, nil, nil, 7, LayerRepo, "config.json"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := fixture(t, tt.files)
			src.Environ, src.Sets = tt.env, tt.set
			l, err := Load(src)
			if err != nil {
				t.Fatal(err)
			}
			layer, source := l.Origin("budget.per_day_usd")
			if l.Config.Budget.PerDayUSD != tt.want || layer != tt.wantLayer || !strings.HasSuffix(source, tt.wantSource) {
				t.Fatalf("per_day_usd = %v from %s (%s); want %v from %s (…%s)", l.Config.Budget.PerDayUSD, layer, source, tt.want, tt.wantLayer, tt.wantSource)
			}
			// A key set only by a lower layer keeps that layer's value and origin.
			if layer, _ := l.Origin("time.tz"); layer != LayerDefault || l.Config.Time.TZ != "Europe/London" {
				t.Fatalf("time.tz = %q from %s", l.Config.Time.TZ, layer)
			}
		})
	}
}

func TestRejects(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
		env   []string
		set   []string
		want  string
	}{
		{"unknown key in a known section", map[string]string{"repo/.aayudha/config.toml": "version = 1\n[budget]\nper_week_usd = 1.0\n"}, nil, nil, "per_week_usd"},
		{"unknown section", map[string]string{"repo/.aayudha/config.local.toml": "version = 1\n[colour]\nhue = 1\n"}, nil, nil, "colour"},
		{"unknown key from env", nil, []string{"AAYU_BUDGET__PER_WEEK_USD=1"}, nil, "AAYU_BUDGET__PER_WEEK_USD"},
		{"unknown key from --set", nil, nil, []string{"budget.per_week_usd=1"}, "per_week_usd"},
		{"unknown seat", map[string]string{"repo/.aayudha/config.toml": "version = 1\n[seats.janitor]\nmode = \"solo\"\n"}, nil, nil, "[seats.janitor]: unknown seat"},
		{"unknown lane", map[string]string{"repo/.aayudha/config.toml": "version = 1\n[models.vibes.cursor]\nmodel = \"x\"\n"}, nil, nil, "[models.vibes]: unknown lane"},
		{"secret value instead of a name", map[string]string{"repo/.aayudha/config.toml": "version = 1\n[integrations.posthog]\nsecret = \"phc_AbC123xyz\"\nscope = \"read\"\n"}, nil, nil, "never the key"},
		{"fleet outside the user config", map[string]string{"repo/.aayudha/config.toml": "version = 1\n[fleet]\nrepos = [\"a/b\"]\n"}, nil, nil, "user-level only"},
		{"bad time zone", nil, nil, []string{"time.tz=Mars/Olympus"}, "Mars/Olympus"},
		{"newer schema", map[string]string{"repo/.aayudha/config.toml": "version = 2\n"}, nil, nil, "newer than this binary"},
		{"decision authority key", nil, nil, []string{"decisions.authority.sprint=founder"}, "[decisions.authority] sprint"},
		{"decision authority value", nil, nil, []string{"decisions.authority.story=not a login!"}, "want a seat slug or a GitHub login"},
		{"escalation login", nil, nil, []string{"decisions.escalate_to=founder b"}, "escalate_to"},
		{"team stage", nil, nil, []string{"team.stage=unicorn"}, `[team] stage = "unicorn"`},
		{"auth on a non-cloud integration", nil, nil, []string{"integrations.posthog.secret=posthog", "integrations.posthog.scope=read", "integrations.posthog.auth=oidc"}, "only [integrations.azure]"},
		{"approvals door", nil, nil, []string{"approvals.device_for=two-way"}, `[approvals] device_for "two-way": want one-way or all`},
		{"approvals login", nil, nil, []string{"approvals.approvers=founder b"}, `[approvals] approvers "founder b": want a GitHub login`},
		{"cloud key auth without a secret", nil, nil, []string{"integrations.aws.scope=read", "integrations.aws.region=eu-west-1"}, `[integrations.aws] secret = ""`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := fixture(t, tt.files)
			src.Environ, src.Sets = tt.env, tt.set
			_, err := Load(src)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v, want it to mention %q", err, tt.want)
			}
		})
	}
	src := fixture(t, map[string]string{"repo/.aayudha/config.toml": "version = 9\n"})
	if _, err := Load(src); !errors.Is(err, ErrNewerSchema) {
		t.Fatalf("err = %v, want ErrNewerSchema", err)
	}
}

// TestTeamStage: a set stage wins; else the size decides at the 15/16 and
// 150/151 edges; else the pod's paired humans plus the founder, so a solo pod
// is a startup (WP5.1b).
func TestTeamStage(t *testing.T) {
	paired := map[string]Seat{}
	for i, id := range []string{"code-architect", "engineering-manager", "quality-architect"} {
		paired[id] = Seat{Mode: "paired", Human: fmt.Sprintf("h%d", i)}
	}
	paired["founder"] = Seat{Mode: "paired", Human: "f"} // the founder is counted once
	tests := []struct {
		team  Team
		seats map[string]Seat
		want  string
	}{
		{Team{}, nil, StageStartup},
		{Team{}, paired, StageStartup},
		{Team{Size: 15}, nil, StageStartup},
		{Team{Size: 16}, nil, StageScaleup},
		{Team{Size: 150}, nil, StageScaleup},
		{Team{Size: 151}, nil, StageEnterprise},
		{Team{Stage: StageEnterprise, Size: 3}, nil, StageEnterprise},
	}
	for _, tt := range tests {
		c := Config{Team: tt.team, Seats: tt.seats}
		if got, why := c.TeamStage(); got != tt.want {
			t.Errorf("%+v with %d seats: stage %s (%s), want %s", tt.team, len(tt.seats), got, why, tt.want)
		}
	}
	if _, why := (&Config{Seats: paired}).TeamStage(); why != "the pod's humans (4)" {
		t.Errorf("why = %q, want the pod's humans (4)", why)
	}
}

func TestMigrateV0(t *testing.T) {
	v0, err := os.ReadFile("testdata/v0.toml")
	if err != nil {
		t.Fatal(err)
	}
	src := fixture(t, map[string]string{"repo/.aayudha/config.toml": string(v0)})
	l, err := Load(src)
	if err != nil {
		t.Fatal(err)
	}
	if l.Config.Version != Version || l.Get("lfs.server") != nil || l.Config.Budget.PerStoryUSD != 12.5 || l.Config.Time.TZ != "Asia/Kolkata" {
		t.Fatalf("migrated config = %+v", l.Config)
	}
	if len(l.Notices) != 1 || !strings.Contains(l.Notices[0], "schema v0") {
		t.Fatalf("notices = %q", l.Notices)
	}

	s3 := strings.Replace(string(v0), `server = "github"`, `server = "s3://bucket/lfs"`, 1)
	src = fixture(t, map[string]string{"repo/.aayudha/config.toml": s3})
	if _, err := Load(src); err == nil || !strings.Contains(err.Error(), "only GitHub LFS") {
		t.Fatalf("err = %v, want the V6 refusal", err)
	}
}

func TestBusinessHoursSLA(t *testing.T) {
	c, err := NewClock("Europe/London", "Mon-Fri 09:00-18:00")
	if err != nil {
		t.Fatal(err)
	}
	london := c.Loc
	at := func(y int, m time.Month, d, h, min int) time.Time { return time.Date(y, m, d, h, min, 0, 0, london) }
	tests := []struct {
		name  string
		start time.Time
		hours int
		want  time.Time
	}{
		// Friday 17:00 has one business hour left; Monday 09:00-16:00 adds seven.
		{"8 h from Friday 17:00 ends Monday 16:00", at(2026, 10, 9, 17, 0), 8, at(2026, 10, 12, 16, 0)},
		// Same SLA across the 25 October 2026 clock change: still 16:00 local, now GMT.
		{"8 h across the October clock change ends Monday 16:00 local", at(2026, 10, 23, 17, 0), 8, at(2026, 10, 26, 16, 0)},
		// 24 business hours: Fri 1 + Mon 9 + Tue 9 + Wed 5 (Appendix B D-1.2a).
		{"24 h from Friday 17:00 ends Wednesday 14:00", at(2026, 10, 9, 17, 0), 24, at(2026, 10, 14, 14, 0)},
		{"24 h across the October clock change ends Wednesday 14:00 local", at(2026, 10, 23, 17, 0), 24, at(2026, 10, 28, 14, 0)},
		{"start before opening counts from 09:00", at(2026, 10, 12, 7, 30), 2, at(2026, 10, 12, 11, 0)},
		{"start on Saturday counts from Monday 09:00", at(2026, 10, 10, 12, 0), 1, at(2026, 10, 12, 10, 0)},
		{"exactly the rest of the day ends at 18:00", at(2026, 10, 12, 9, 0), 9, at(2026, 10, 12, 18, 0)},
		{"one more minute rolls to the next day", at(2026, 10, 12, 17, 59), 1, at(2026, 10, 13, 9, 59)},
		{"48 h is a full working week plus three hours", at(2026, 10, 12, 9, 0), 48, at(2026, 10, 19, 12, 0)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c.Add(tt.start, time.Duration(tt.hours)*time.Hour)
			if !got.Equal(tt.want) || got.In(london).Format("15:04") != tt.want.Format("15:04") {
				t.Fatalf("got %s, want %s", got.In(london).Format(time.RFC1123), tt.want.Format(time.RFC1123))
			}
		})
	}
	if _, err := NewClock("Europe/London", "weekdays"); err == nil {
		t.Fatal("malformed business_hours accepted")
	}
	// Between counts business time only: Friday 17:00 to Monday 16:00 London is 8 hours (D-1.2a).
	if got := c.Between(at(2026, 10, 16, 17, 0), at(2026, 10, 19, 16, 0)); got != 8*time.Hour {
		t.Fatalf("Between Friday 17:00 and Monday 16:00 = %s, want 8h", got)
	}
	if got := c.Between(at(2026, 10, 19, 16, 0), at(2026, 10, 16, 17, 0)); got != 0 {
		t.Fatalf("Between backwards = %s, want 0", got)
	}
}

// TestSoloProvisionalKey (Story #432, criterion 3): the solo window has a hard
// minimum of 12 business hours, and only the protected repository config sets
// it; every other layer is refused, even with an allowed value.
func TestSoloProvisionalKey(t *testing.T) {
	key := "[decisions]\nsolo_provisional_after_hours = 24\n"
	tests := []struct {
		name     string
		files    map[string]string
		env, set []string
		want     string
	}{
		{"below the minimum", map[string]string{"repo/.aayudha/config.toml": "version = 1\n[decisions]\nsolo_provisional_after_hours = 11\n"}, nil, nil, "solo_provisional_after_hours = 11: the minimum is 12 business hours"},
		{"user config", map[string]string{"home/.config/aayudha/config.toml": "version = 1\n" + key}, nil, nil, "read only from .aayudha/config.toml"},
		{"local config", map[string]string{"repo/.aayudha/config.local.toml": "version = 1\n" + key}, nil, nil, "read only from .aayudha/config.toml"},
		{"environment", nil, []string{"AAYU_DECISIONS__SOLO_PROVISIONAL_AFTER_HOURS=24"}, nil, "read only from .aayudha/config.toml"},
		{"--set", nil, nil, []string{"decisions.solo_provisional_after_hours=24"}, "read only from .aayudha/config.toml"},
	}
	for _, tt := range tests {
		src := fixture(t, tt.files)
		src.Environ, src.Sets = tt.env, tt.set
		if _, err := Load(src); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s: err = %v, want %q", tt.name, err, tt.want)
		}
	}
	for body, want := range map[string]int{"": 24, "[decisions]\nsolo_provisional_after_hours = 12\n": 12} {
		l, err := Load(fixture(t, map[string]string{"repo/.aayudha/config.toml": "version = 1\n" + body}))
		if err != nil || l.Config.Decisions.SoloProvisional != want {
			t.Errorf("repository config %q: %v; want %d", body, err, want)
		}
	}
}

// TestAllowPIIRepoOnly (Decision #449): personal data from a host's PostHog or
// Sentry MCP is the repository's opt-in, so only the protected repository
// config turns it on; a person's or an agent's own layer is refused.
func TestAllowPIIRepoOnly(t *testing.T) {
	key := "[integrations.posthog]\nallow_pii = true\n"
	for name, tt := range map[string]struct {
		files    map[string]string
		env, set []string
	}{
		"user config":  {files: map[string]string{"home/.config/aayudha/config.toml": "version = 1\n" + key}},
		"local config": {files: map[string]string{"repo/.aayudha/config.local.toml": "version = 1\n" + key}},
		"environment":  {env: []string{"AAYU_INTEGRATIONS__SENTRY__ALLOW_PII=true"}},
		"--set":        {set: []string{"integrations.posthog.allow_pii=true"}},
	} {
		src := fixture(t, tt.files)
		src.Environ, src.Sets = tt.env, tt.set
		if _, err := Load(src); err == nil || !strings.Contains(err.Error(), "allow_pii is read only from .aayudha/config.toml") {
			t.Errorf("%s: err = %v; want the repository-only refusal", name, err)
		}
	}
	l, err := Load(fixture(t, map[string]string{"repo/.aayudha/config.toml": "version = 1\n" + key + "secret = \"posthog\"\nscope = \"read\"\nproject = \"1\"\n"}))
	if err != nil || !l.Config.Integrations["posthog"].AllowPII || l.Config.Integrations["sentry"].AllowPII {
		t.Errorf("repository config: %v; want posthog allow_pii on and sentry off", err)
	}
}
