package config

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
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
		{"gateway endpoint from the checkout", map[string]string{"repo/.aayudha/config.toml": "version = 1\n[models.gateway]\nbase_url = \"https://evil.example/v1\"\n"}, nil, nil, "never from the checkout"},
		{"Cursor plan from the checkout", map[string]string{"repo/.aayudha/config.toml": "version = 1\n[hosts.cursor]\nplan = \"legacy\"\n"}, nil, nil, "[hosts.cursor] plan is read only from the user config"},
		{"gateway secret from a committed local file", map[string]string{"repo/.aayudha/config.local.toml": "version = 1\n[models.gateway]\nsecret = \"github-token\"\n"}, nil, nil, "never from the checkout"},
		{"Copilot route opt-in from the checkout", map[string]string{"repo/.aayudha/config.toml": "version = 1\n[models.ui-review.copilot]\nmodel = \"gemini-4-argon\"\n"}, nil, nil, "never from the checkout"},
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
	// The person's own config may set the gateway.
	src = fixture(t, map[string]string{"home/.config/aayudha/config.toml": "version = 1\n[models.gateway]\nbase_url = \"https://gw.example/v1\"\nsecret = \"ai-gateway\"\n"})
	if l, err := Load(src); err != nil || l.Config.Models.Gateway.BaseURL != "https://gw.example/v1" {
		t.Fatalf("gateway from the user config: %v", err)
	}
	src = fixture(t, map[string]string{"home/.config/aayudha/config.toml": "version = 1\n[models.ui-review.copilot]\nmodel = \"gemini-4-argon\"\n",
		"repo/.aayudha/config.toml": "version = 1\n[models.review.copilot]\nmodel = \"auto:intelligence\"\n"})
	if l, err := Load(src); err != nil || l.Config.Models.Lanes["ui-review"]["copilot"].Model != "gemini-4-argon" {
		t.Fatalf("Copilot opt-in from the user config, another copilot row from the checkout: %v", err)
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

// TestIntegrationURLFromCheckout (Bug #598): a stored key is sent to its
// integration's url, so a checkout's config names only the provider's cloud
// host; any other url, including a look-alike host, comes only from the
// person's own layers.
func TestIntegrationURLFromCheckout(t *testing.T) {
	section := func(p, u string) string {
		return "version = 1\n[integrations." + p + "]\nsecret = \"" + p + "\"\nscope = \"read\"\nurl = \"" + u + "\"\n"
	}
	for _, file := range []string{"repo/.aayudha/config.toml", "repo/.aayudha/config.local.toml"} {
		for _, c := range [][2]string{
			{"posthog", "https://collector.example.com"},
			{"posthog", "http://us.posthog.com"},
			{"posthog", "https://us.posthog.com.example.com"},
			{"posthog", "https://eu.posthog.com:8443"},
			{"sentry", "https://collector.example.com"},
			{"sentry", "https://sentry.io.example.com"},
			{"sentry", "https://evilsentry.io"},
			{"github", "https://collector.example.com"},
			{"growthbook", "https://growthbook.example.com"},
		} {
			_, err := Load(fixture(t, map[string]string{file: section(c[0], c[1])}))
			if err == nil || !strings.Contains(err.Error(), "[integrations."+c[0]+"] url") || !strings.Contains(err.Error(), "--set") {
				t.Errorf("%s with %s url %s: err = %v; want the checkout refusal", file, c[0], c[1], err)
			}
		}
	}
	for _, c := range [][2]string{{"posthog", "https://eu.posthog.com"}, {"sentry", "https://de.sentry.io"}, {"sentry", "https://acme.sentry.io/"}} {
		l, err := Load(fixture(t, map[string]string{"repo/.aayudha/config.toml": section(c[0], c[1])}))
		if err != nil || l.Config.Integrations[c[0]].URL != c[1] {
			t.Errorf("checkout %s url %s: %v; want it accepted", c[0], c[1], err)
		}
	}
	self := "https://posthog.example.com"
	for name, tt := range map[string]struct {
		files    map[string]string
		env, set []string
	}{
		"user config": {files: map[string]string{"home/.config/aayudha/config.toml": section("posthog", self)}},
		"environment": {files: map[string]string{"repo/.aayudha/config.toml": section("posthog", "https://eu.posthog.com")}, env: []string{"AAYU_INTEGRATIONS__POSTHOG__URL=" + self}},
		"--set":       {files: map[string]string{"repo/.aayudha/config.toml": section("posthog", "https://eu.posthog.com")}, set: []string{"integrations.posthog.url=" + self}},
	} {
		src := fixture(t, tt.files)
		src.Environ, src.Sets = tt.env, tt.set
		l, err := Load(src)
		if err != nil || l.Config.Integrations["posthog"].URL != self {
			t.Errorf("%s: %v; want url %s", name, err, self)
		}
	}
}

// TestBuildBudgetDefaults (founder decision, 2026-10-17): a build may run 8
// wall hours, 3 failed attempts, and 60 USD of metered spend, and the
// gateway breaker's per-story limit is never below the build's, so the
// breaker does not stop a story before its build budget does.
func TestBuildBudgetDefaults(t *testing.T) {
	l, err := Load(Sources{})
	if err != nil {
		t.Fatal(err)
	}
	b, g := l.Config.Autonomy.Budget, l.Config.Budget
	if b.WallHours != 8 || b.Attempts != 3 || b.CostUSD != 60 || g.PerDayUSD != 100 {
		t.Errorf("[autonomy.budget] %+v, [budget] per_day_usd %g", b, g.PerDayUSD)
	}
	if g.PerStoryUSD < b.CostUSD {
		t.Errorf("[budget] per_story_usd %g is below [autonomy.budget] cost_usd %g", g.PerStoryUSD, b.CostUSD)
	}
}

// The security workflow and doctor's row scan the branches role by role: a partial [branches] map keeps the other
// defaults, and only a repository that maps dev (coexistence) can have no rolling.
func TestBranchNames(t *testing.T) {
	for _, tc := range []struct {
		b    Branches
		want [3]string
	}{
		{Branches{}, [3]string{"dev", "rolling", "main"}},
		{Branches{Main: "prod"}, [3]string{"dev", "rolling", "prod"}},
		{Branches{Rolling: "canary"}, [3]string{"dev", "canary", "main"}},
		{Branches{Dev: "trunk", Main: "prod"}, [3]string{"trunk", "", "prod"}},
		{Branches{Dev: "trunk", Rolling: "canary"}, [3]string{"trunk", "canary", "main"}},
	} {
		if got := tc.b.Names(); got != tc.want {
			t.Errorf("%+v: %q, want %q", tc.b, got, tc.want)
		}
	}
}

// A repository config read from elsewhere (a base revision's text) is the
// repo layer: its values hold, and a key the schema does not know, or text
// that is not TOML, fails naming where it came from.
func TestLoadRepoTOML(t *testing.T) {
	l, err := Load(Sources{RepoTOML: []byte("version = 1\n[autonomy]\nproof = true\n"), RepoSource: "base config"})
	if err != nil || !l.Config.Autonomy.Proof {
		t.Fatalf("load: %v %+v", err, l)
	}
	for _, text := range []string{"version = 1\n[autonomy]\nprooof = true\n", "version = 1\n[autonomy\n"} {
		if _, err := Load(Sources{RepoTOML: []byte(text), RepoSource: "base config"}); err == nil || !strings.Contains(err.Error(), "base config") {
			t.Errorf("load %q: %v", text, err)
		}
	}
}

// TestRepositoryMergesOnProof keeps Decision #593 (option 1, accepted 2026-10-18) for this repository: ship merges
// into dev on proof, rung C counts until rung B is live, and the narrow set of the Decision always needs a person.
// The built-in defaults leave proof off, so a repository opts in.
func TestRepositoryMergesOnProof(t *testing.T) {
	def, err := Load(Sources{})
	if err != nil {
		t.Fatal(err)
	}
	l, err := Load(Sources{RepoDir: "../.."})
	if err != nil {
		t.Fatal(err)
	}
	a := l.Config.Autonomy
	narrow := []string{".github/CODEOWNERS", "internal/cli/ship.go", "internal/cli/shippass.go", "internal/cli/shipproof.go",
		"internal/cli/presence.go", "internal/identity/", "internal/autonomy/", "internal/config/", ".github/workflows/aayudha-merge-gate.yml",
		".github/workflows/protected-paths*.yml", ".github/workflows/release.yml", ".github/workflows/aayudha-approve.yml",
		".github/workflows/aayudha-token.yml", ".goreleaser.yaml", "install.sh", "go.mod", "go.sum",
		// The config's other files load into the same layers, so they can change the policy too.
		".aayudha/config.json", ".aayudha/config.local.toml", ".aayudha/config.local.json",
		// The build-ready loop's definition is the standing ask build --next acts on.
		"loops/build-ready.toml"}
	if !slices.Equal(def.Config.Autonomy.AlwaysPerson, []string{".github/CODEOWNERS", ".aayudha/config.json", ".aayudha/config.local.toml", ".aayudha/config.local.json", "loops/build-ready.toml"}) {
		t.Fatalf("default narrow set: %v", def.Config.Autonomy.AlwaysPerson)
	}
	if def.Config.Autonomy.Proof || !a.Proof || !slices.Equal(a.ProofRungs, []string{"A", "B", "C"}) || !slices.Equal(a.AlwaysPerson, narrow) ||
		!slices.Equal(a.AlwaysPersonConfig, []string{".aayudha/config.toml", "internal/config/defaults.toml"}) ||
		!slices.Equal(a.AlwaysPersonSections, []string{"identity", "approvals", "autonomy", "seats", "verify", "inspect", "review", "models", "branches", "decisions", "proof",
			"delegate", "notify", "integrations", "telemetry", "mcp", "hosts", "lfs", "measure", "budget", "scope", "build"}) {
		t.Fatalf("defaults proof=%v; repository [autonomy]: %+v", def.Config.Autonomy.Proof, a)
	}
}

// Decision #485 G1: an active DAST scan of a URL verify did not start is the
// protected repository config's call alone; every other layer is refused.
func TestActiveScanKey(t *testing.T) {
	key := "[verify.environments.staging]\nurl = \"https://staging.example.test\"\nactive_scan = true\n"
	for _, tt := range []struct {
		name     string
		files    map[string]string
		env, set []string
	}{
		{"user config", map[string]string{"home/.config/aayudha/config.toml": "version = 1\n" + key}, nil, nil},
		{"local config", map[string]string{"repo/.aayudha/config.local.toml": "version = 1\n" + key}, nil, nil},
		// The repository's JSON config loads as the repo layer too, but CODEOWNERS protects only the TOML.
		{"repo JSON config", map[string]string{"repo/.aayudha/config.json": `{"version": 1, "verify": {"environments": {"staging": {"url": "https://staging.example.test", "active_scan": true}}}}`}, nil, nil},
		{"environment", nil, []string{"AAYU_VERIFY__ENVIRONMENTS__STAGING__ACTIVE_SCAN=true"}, nil},
		{"--set", nil, nil, []string{"verify.environments.staging.active_scan=true"}},
	} {
		src := fixture(t, tt.files)
		src.Environ, src.Sets = tt.env, tt.set
		if _, err := Load(src); err == nil || !strings.Contains(strings.ToLower(err.Error()), "verify.environments.staging.active_scan is read only from .aayudha/config.toml") {
			t.Errorf("%s: err = %v", tt.name, err)
		}
	}
	// CodeRabbit on #509: a case variant must not opt in either. The decoder now matches keys exactly (Bug #640),
	// so the local file's Active_Scan is refused as an unknown key; the repository names the url but does not opt in.
	if _, err := Load(fixture(t, map[string]string{
		"repo/.aayudha/config.toml":       "version = 1\n[verify.environments.staging]\nurl = \"https://staging.example.test\"\n",
		"repo/.aayudha/config.local.toml": "version = 1\n[verify.environments.staging]\nActive_Scan = true\n"})); err == nil || !strings.Contains(err.Error(), "invalid keys: Active_Scan") {
		t.Errorf("local config, another case: err = %v", err)
	}
	// The local layer may still set an environment's URL; the repository config may opt it in.
	l, err := Load(fixture(t, map[string]string{"repo/.aayudha/config.toml": "version = 1\n" + key,
		"repo/.aayudha/config.local.toml": "version = 1\n[verify.environments.local]\nurl = \"http://127.0.0.1:9000\"\n"}))
	if err != nil || !l.Config.Verify.Environments["staging"].ActiveScan || l.Config.Verify.Environments["local"].URL != "http://127.0.0.1:9000" {
		t.Fatalf("repository config: %v %+v", err, l.Config.Verify.Environments)
	}
	// Codex review of #509: the approval covers the url the protected config names; a later layer cannot repoint it.
	for _, tt := range []struct {
		name     string
		files    map[string]string
		env, set []string
	}{
		{"local url", map[string]string{"repo/.aayudha/config.local.toml": "version = 1\n[verify.environments.staging]\nurl = \"https://victim.example.test\"\n"}, nil, nil},
		{"environment url", nil, []string{"AAYU_VERIFY__ENVIRONMENTS__STAGING__URL=https://victim.example.test"}, nil},
		{"--set url", nil, nil, []string{"verify.environments.staging.url=https://victim.example.test"}},
	} {
		tt.files = maps.Clone(tt.files)
		if tt.files == nil {
			tt.files = map[string]string{}
		}
		tt.files["repo/.aayudha/config.toml"] = "version = 1\n" + key
		src := fixture(t, tt.files)
		src.Environ, src.Sets = tt.env, tt.set
		if _, err := Load(src); err == nil || !strings.Contains(err.Error(), `[verify.environments.staging] url "https://victim.example.test": active_scan = true approves only the url`) {
			t.Errorf("%s: err = %v", tt.name, err)
		}
	}
	// An approval with no url of its own approves nothing, whichever layer supplies one.
	if _, err := Load(fixture(t, map[string]string{"repo/.aayudha/config.toml": "version = 1\n[verify.environments.staging]\nactive_scan = true\n",
		"home/.config/aayudha/config.toml": "version = 1\n[verify.environments.staging]\nurl = \"https://victim.example.test\"\n"})); err == nil || !strings.Contains(err.Error(), "approves only the url") {
		t.Errorf("url from the user config: err = %v", err)
	}
	// A base revision's config.toml, given as text the way ship reads it, may opt in too.
	if b, err := Load(Sources{RepoTOML: []byte("version = 1\n" + key), RepoSource: ".aayudha/config.toml at 18aca3b"}); err != nil || !b.Config.Verify.Environments["staging"].ActiveScan {
		t.Fatalf("base config.toml: %v", err)
	}
	if l.Config.Verify.DAST != true || l.Config.Verify.NewFloorsBlockingFrom != "2026-11-14" || !l.Config.Verify.Floors.DAST.SkippedBlocksShip {
		t.Fatalf("defaults: %+v", l.Config.Verify)
	}
	// Empty means advisory; a typo must not make the new floors advisory forever (CodeRabbit on #509).
	for _, d := range []string{"2026-11-31", "2026/11/14"} {
		if _, err := Load(fixture(t, map[string]string{"repo/.aayudha/config.toml": "version = 1\n[verify]\nnew_floors_blocking_from = \"" + d + "\"\n"})); err == nil || !strings.Contains(err.Error(), "is not a YYYY-MM-DD date") {
			t.Errorf("%s: err = %v", d, err)
		}
	}
	if _, err := Load(fixture(t, map[string]string{"repo/.aayudha/config.toml": "version = 1\n[verify]\nnew_floors_blocking_from = \"\"\n"})); err != nil {
		t.Errorf("empty date: %v", err)
	}
}

// TestProtectedOnlyKeysFromTOML (Bug #640, criterion 1): .aayudha/config.json
// loads as the repository layer but need not be code-owned, so a protected-only
// key set there is refused, naming the file; .aayudha/config.toml still sets it.
func TestProtectedOnlyKeysFromTOML(t *testing.T) {
	for key, body := range map[string]string{
		"solo_provisional_after_hours": `{"version": 1, "decisions": {"solo_provisional_after_hours": 24}}`,
		"allow_pii":                    `{"version": 1, "integrations": {"posthog": {"allow_pii": true, "secret": "posthog", "scope": "read", "project": "1"}}}`,
	} {
		src := fixture(t, map[string]string{"repo/.aayudha/config.json": body})
		_, err := Load(src)
		file := filepath.Join(src.RepoDir, ".aayudha", "config.json")
		if err == nil || !strings.Contains(err.Error(), file+": ") || !strings.Contains(err.Error(), key+" is read only from .aayudha/config.toml") {
			t.Errorf("%s from config.json: err = %v; want a refusal naming %s", key, err, file)
		}
	}
	l, err := Load(fixture(t, map[string]string{"repo/.aayudha/config.toml": "version = 1\n[decisions]\nsolo_provisional_after_hours = 12\n" +
		"[integrations.posthog]\nallow_pii = true\nsecret = \"posthog\"\nscope = \"read\"\nproject = \"1\"\n"}))
	if err != nil || l.Config.Decisions.SoloProvisional != 12 || !l.Config.Integrations["posthog"].AllowPII {
		t.Errorf("config.toml: %v; want both keys applied", err)
	}
	// A base revision's config.toml, given as text the way ship reads it, is the code-owned file too.
	l, err = Load(Sources{RepoTOML: []byte("version = 1\n[decisions]\nsolo_provisional_after_hours = 12\n"), RepoSource: ".aayudha/config.toml at 1c978f1"})
	if err != nil || l.Config.Decisions.SoloProvisional != 12 {
		t.Errorf("base config.toml: %v; want the key applied", err)
	}
}

// TestCaseVariantKeysRefused (Bug #640): keys match the schema with exact
// case, so a case variant of a key a per-layer rule guards is an unknown key,
// refused, and never sets the field.
func TestCaseVariantKeysRefused(t *testing.T) {
	for _, tt := range []struct{ name, file, body, key string }{
		{"allow_pii", "config.json", `{"version": 1, "integrations": {"posthog": {"Allow_PII": true, "secret": "posthog", "scope": "read", "project": "1"}}}`, "Allow_PII"},
		{"solo_provisional_after_hours", "config.json", `{"version": 1, "decisions": {"Solo_Provisional_After_Hours": 12}}`, "Solo_Provisional_After_Hours"},
		{"solo_provisional_after_hours", "config.local.json", `{"version": 1, "decisions": {"SOLO_PROVISIONAL_AFTER_HOURS": 12}}`, "SOLO_PROVISIONAL_AFTER_HOURS"},
		{"models.gateway.base_url", "config.local.json", `{"version": 1, "models": {"gateway": {"Base_URL": "https://evil.example.com"}}}`, "Base_URL"},
	} {
		l, err := Load(fixture(t, map[string]string{"repo/.aayudha/" + tt.file: tt.body}))
		if err == nil || !strings.Contains(err.Error(), tt.file+": ") || !strings.Contains(err.Error(), tt.key) {
			got := ""
			if l != nil {
				got = fmt.Sprintf("allow_pii=%v solo=%v base_url=%q", l.Config.Integrations["posthog"].AllowPII, l.Config.Decisions.SoloProvisional, l.Config.Models.Gateway.BaseURL)
			}
			t.Errorf("%s as %s in %s: err = %v (%s); want it refused as an unknown key", tt.name, tt.key, tt.file, err, got)
		}
	}
}

// TestCaseVariantFileNamesIgnored (Bug #640): only a directory entry named
// exactly config.json or config.local.json is read. On macOS and Windows
// opening config.json finds Config.json, which CODEOWNERS does not own; the
// case-insensitive readFile below stands in for that filesystem.
func TestCaseVariantFileNamesIgnored(t *testing.T) {
	old := readFile
	t.Cleanup(func() { readFile = old })
	readFile = func(path string) ([]byte, error) {
		dir, base := filepath.Split(path)
		des, err := os.ReadDir(dir)
		if err != nil {
			return nil, err
		}
		for _, de := range des {
			if strings.EqualFold(de.Name(), base) {
				return os.ReadFile(filepath.Join(dir, de.Name()))
			}
		}
		return nil, os.ErrNotExist
	}
	body := `{"version": 1, "budget": {"per_day_usd": 7}}`
	for _, name := range []string{"Config.json", "CONFIG.LOCAL.JSON"} {
		for _, ci := range []bool{false, true} {
			src := fixture(t, map[string]string{"repo/.aayudha/" + name: body})
			src.CI = ci
			l, err := Load(src)
			if err != nil {
				t.Fatalf("%s, CI=%v: %v", name, ci, err)
			}
			if layer, source := l.Origin("budget.per_day_usd"); layer != LayerDefault || len(l.Notices) != 0 {
				t.Errorf("%s, CI=%v: per_day_usd = %v from %s (%s), notices %q; want the file ignored", name, ci, l.Config.Budget.PerDayUSD, layer, source, l.Notices)
			}
		}
	}
}

// TestConfigDirUnlistable (Bug #640): files are read only by exact name from
// the directory's listing, so a config directory that cannot be listed fails
// the load, naming it, rather than reading as if it held no config.
func TestConfigDirUnlistable(t *testing.T) {
	src := fixture(t, map[string]string{"repo/.aayudha": "not a directory\n"})
	if _, err := Load(src); err == nil || !strings.Contains(err.Error(), filepath.Join(src.RepoDir, ".aayudha")) {
		t.Fatalf("err = %v; want a failure naming %s", err, filepath.Join(src.RepoDir, ".aayudha"))
	}
}

// TestMalformedLocalConfigInCI (Bug #640): CI never parses the local layer,
// so a committed config.local.toml that is not valid TOML does not fail the
// load and still gets the notice; outside CI it fails as before.
func TestMalformedLocalConfigInCI(t *testing.T) {
	files := map[string]string{"repo/.aayudha/config.local.toml": "version = 1\n[budget\nper_day_usd = = 3\n"}
	src := fixture(t, files)
	src.CI = true
	l, err := Load(src)
	file := filepath.Join(src.RepoDir, ".aayudha", "config.local.toml")
	if err != nil || len(l.Notices) != 1 || !strings.Contains(l.Notices[0], file+" ignored") {
		t.Fatalf("CI: err = %v; want no error and one notice naming %s", err, file)
	}
	src = fixture(t, files)
	if _, err := Load(src); err == nil || !strings.Contains(err.Error(), "config.local.toml") {
		t.Fatalf("outside CI: err = %v; want the parse error naming config.local.toml", err)
	}
}
