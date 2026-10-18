package loops

import (
	"flag"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

var update = flag.Bool("update-golden", false, "rewrite internal/loops/testdata/golden")

func loop(t *testing.T, cron string) Loop {
	t.Helper()
	l, err := Parse("x", []byte("description = \"x\"\ncommand = [\"status\", \"--digest\"]\ncron = \""+cron+"\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func zone(t *testing.T, tz string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(tz)
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

// Local crons converted to UTC, one per offset the zone uses; the expected
// values are worked out by hand from each zone's standard and summer offsets.
func TestUTCCrons(t *testing.T) {
	tests := []struct {
		tz, cron string
		want     []string
	}{
		{"Europe/London", "5 9 * * 1-5", []string{"5 8 * * 1-5", "5 9 * * 1-5"}}, // BST +1, GMT +0
		{"Asia/Kolkata", "30 0 * * 1-5", []string{"0 19 * * 0-4"}},               // +5:30, no DST: Sunday–Thursday 19:00
		{"America/New_York", "0 22 * * 5", []string{"0 2 * * 6", "0 3 * * 6"}},   // EDT −4, EST −5: Saturday
		{"Asia/Kolkata", "15 * * * *", []string{"45 * * * *"}},                   // hourly: only the minute moves
		{"Europe/London", "15 9,13,17 * * 1-5", []string{"15 8,12,16 * * 1-5", "15 9,13,17 * * 1-5"}},
		{"Europe/London", "0 10 2 1,4,7,10 *", []string{"0 9 2 1,4,7,10 *", "0 10 2 1,4,7,10 *"}},
		// 21:00 in New York is 01:00 (EDT −4) or 02:00 (EST −5) UTC on the next day: its own cron, Tuesday–Saturday.
		{"America/New_York", "0 9,13,17,21 * * 1-5", []string{"0 13,17,21 * * 1-5", "0 1 * * 2-6", "0 14,18,22 * * 1-5", "0 2 * * 2-6"}},
	}
	for _, tt := range tests {
		got, err := loop(t, tt.cron).UTCCrons(zone(t, tt.tz))
		if err != nil || !slices.Equal(got, tt.want) {
			t.Errorf("%s %q: %v %v, want %v", tt.tz, tt.cron, got, err, tt.want)
		}
	}
	// 10:00 on the 1st in Sydney is the previous day in UTC, which a day-of-month cron cannot say.
	if _, err := loop(t, "0 10 1 * *").UTCCrons(zone(t, "Australia/Sydney")); err == nil {
		t.Error("Sydney day-of-month across midnight UTC: no error")
	}
}

// Of a DST zone's two crons exactly one runs each day; outside Actions a run
// is due within the window after the local time.
func TestDue(t *testing.T) {
	london := zone(t, "Europe/London")
	l := loop(t, "5 9 * * 1-5")
	tests := []struct {
		name  string
		now   string
		fired string
		want  bool
	}{
		{"summer, the BST cron", "2026-10-12T08:05:00Z", "5 8 * * 1-5", true},
		{"summer, the GMT cron (10:05 local)", "2026-10-12T09:05:00Z", "5 9 * * 1-5", false},
		{"winter, the GMT cron", "2026-11-02T09:05:00Z", "5 9 * * 1-5", true},
		{"winter, the BST cron (08:05 local)", "2026-11-02T08:05:00Z", "5 8 * * 1-5", false},
		{"summer, the BST cron delayed 40 minutes", "2026-10-12T08:45:00Z", "5 8 * * 1-5", true},
		{"host run inside the window", "2026-10-12T08:50:00Z", "", true},
		{"host run 60 minutes late", "2026-10-12T09:05:00Z", "", false},
		{"host run before the time", "2026-10-12T07:59:00Z", "", false},
		{"Saturday", "2026-10-17T08:05:00Z", "", false},
	}
	for _, tt := range tests {
		now, _ := time.Parse(time.RFC3339, tt.now)
		if got := l.Due(london, now, tt.fired); got != tt.want {
			t.Errorf("%s: due %v, want %v", tt.name, got, tt.want)
		}
	}
	// 21:00 on Monday in New York (EDT) is 01:00 UTC on Tuesday, started by the next-day cron.
	ny, late := zone(t, "America/New_York"), loop(t, "0 9,21 * * 1-5")
	tue, _ := time.Parse(time.RFC3339, "2026-10-13T01:00:00Z")
	if !late.Due(ny, tue, "0 1 * * 2-6") || late.Due(ny, tue, "0 2 * * 2-6") {
		t.Error("New York 21:00: the EDT next-day cron must be due, and only it")
	}
}

func TestParseRefuses(t *testing.T) {
	for name, body := range map[string]string{
		"--yes":               `command = ["status", "--digest", "--yes"]` + "\ncron = \"0 9 * * *\"",
		"a human-only flag":   `command = ["decide", "4", "--accept", "A"]` + "\ncron = \"0 9 * * *\"",
		"a human pass":        `command = ["ship", "4"]` + "\ncron = \"0 9 * * *\"",
		"a terminal":          `command = ["init"]` + "\ncron = \"0 9 * * *\"",
		"not a command":       `command = ["rm", "-rf"]` + "\ncron = \"0 9 * * *\"",
		"day of both kinds":   `command = ["status"]` + "\ncron = \"0 9 1 * 1\"",
		"two minutes":         `command = ["status"]` + "\ncron = \"0,30 9 * * *\"",
		"unknown key":         `command = ["status"]` + "\ncron = \"0 9 * * *\"\nshell = \"x\"",
		"unknown host":        `command = ["status"]` + "\ncron = \"0 9 * * *\"\nhosts = [\"jenkins\"]",
		"hourly on weekdays":  `command = ["status"]` + "\ncron = \"0 * * * 1-5\"",
		"an unpinned setup":   `command = ["status"]` + "\ncron = \"0 9 * * *\"\nsetup = [\"python\"]",
		"no cron":             `command = ["status"]`,
		"a scheduled subject": `command = ["triage", "{number}"]` + "\ncron = \"0 9 * * *\"",
		// WP7.1b: an event loop names a known event and the label, has no cron, and runs only where a trigger exists.
		"an event, no label":      `command = ["triage"]` + "\non = \"issues.opened\"",
		"cron and an event":       `command = ["triage"]` + "\ncron = \"0 9 * * *\"\non = \"issues.opened\"\nfilter = { label = \"intake\" }",
		"a filter, no event":      `command = ["status"]` + "\ncron = \"0 9 * * *\"\nfilter = { label = \"x\" }",
		"an unknown event":        `command = ["triage"]` + "\non = \"issues.labeled\"\nfilter = { label = \"intake\" }",
		"an unknown filter key":   `command = ["triage"]` + "\non = \"issues.opened\"\nfilter = { label = \"intake\", author = \"x\" }",
		"an event in Claude Code": `command = ["triage"]` + "\non = \"issues.opened\"\nfilter = { label = \"intake\" }\nhosts = [\"amp\", \"claude\"]",
		"an event in Actions":     `command = ["status"]` + "\non = \"issues.opened\"\nfilter = { label = \"intake\" }\nagent = false",
		"an event subject":        `command = ["triage", "{number}"]` + "\non = \"issues.opened\"\nfilter = { label = \"intake\" }",
		// D-7.1f: the Actions fallback runs only loops that need no agent, and those need neither a tool nor the bot.
		"an agent loop in Actions":    `command = ["triage"]` + "\ncron = \"0 9 * * *\"\nhosts = [\"amp\", \"actions\"]",
		"Actions setup, agent loop":   `command = ["triage"]` + "\ncron = \"0 9 * * *\"\nsetup = [\"node\"]",
		"no agent, in an agent tool":  `command = ["status"]` + "\ncron = \"0 9 * * *\"\nagent = false\nhosts = [\"amp\"]",
		"no agent, writes as the bot": `command = ["decide"]` + "\ncron = \"0 9 * * *\"\nagent = false\nbot = true",
		"no agent, a prompt":          `command = ["status"]` + "\ncron = \"0 9 * * *\"\nagent = false\nprompt = \"x\"",
	} {
		if _, err := Parse("x", []byte("description = \"x\"\n"+body+"\n")); err == nil {
			t.Errorf("%s: parsed", name)
		}
	}
	// A tool with no trigger for the event is named, with what covers it.
	if _, err := Parse("x", []byte("description = \"x\"\ncommand = [\"triage\"]\non = \"issues.opened\"\nfilter = { label = \"intake\" }\nhosts = [\"codex\"]\n")); err == nil || !strings.Contains(err.Error(), "codex has no issues.opened trigger") {
		t.Errorf("an event loop in Codex: %v", err)
	}
}

// WP7.1b: an event loop runs where a trigger for its event exists (Amp and
// Cursor for issues.opened); a tool without one shows it as covered by the
// scheduled loop with the same command, not as missing; a changed filter
// turns its record stale.
func TestEventLoop(t *testing.T) {
	parse := func(name, body string) Loop {
		t.Helper()
		l, err := Parse(name, []byte(body))
		if err != nil {
			t.Fatal(err)
		}
		return l
	}
	ev := parse("arrival", "description = \"x\"\ncommand = [\"triage\"]\non = \"issues.opened\"\nfilter = { label = \"intake\" }\n")
	if !slices.Equal(ev.Hosts, []string{HostAmp, HostCursor}) || !ev.IsEvent() {
		t.Errorf("hosts %v, event %v", ev.Hosts, ev.IsEvent())
	}
	other := parse("arrival", "description = \"x\"\ncommand = [\"triage\"]\non = \"issues.opened\"\nfilter = { label = \"feedback\" }\n")
	if ev.Digest("o/r", "dev", "UTC") == other.Digest("o/r", "dev", "UTC") {
		t.Error("a changed filter label keeps the same digest")
	}
	ls := []Loop{parse("daily", "description = \"x\"\ncommand = [\"triage\"]\ncron = \"0 9 * * 1-5\"\n"),
		parse("status", "description = \"x\"\ncommand = [\"status\"]\ncron = \"0 8 * * 1-5\"\n"), ev}
	e := Entry{Digest: ev.Digest("o/r", "dev", "UTC"), ID: "subscription s-1", At: "2026-10-18"}
	for _, tt := range []struct {
		name          string
		rec           Record
		status, owner string
		detail        string
	}{
		{"no tool recorded", Record{}, Missing, "", ""},
		{"Claude Code runs the loops", Record{HostClaude: {"daily": e}}, Fallback, HostClaude, "the scheduled loop daily covers it"},
		{"Amp runs the others", Record{HostAmp: {"daily": e}}, Missing, "", ""},
		{"Amp holds the subscription", Record{HostAmp: {"daily": e, "arrival": e}}, Active, HostAmp, "subscription s-1"},
	} {
		s := States(ls, tt.rec, "o/r", "dev", "UTC")[2]
		if s.Status != tt.status || s.Owner != tt.owner || !strings.Contains(s.Detail, tt.detail) {
			t.Errorf("%s: %+v, want %s in %q (%q)", tt.name, s, tt.status, tt.owner, tt.detail)
		}
	}
	// With no scheduled loop of the same command, nothing covers it: missing, not fallback.
	if s := States([]Loop{ls[1], ev}, Record{HostClaude: {"status": e}}, "o/r", "dev", "UTC")[1]; s.Status != Missing {
		t.Errorf("no cover: %+v", s)
	}
	if c, ok := Covers(ls, ev, HostCodex); !ok || c.Name != "daily" {
		t.Errorf("covers in codex: %v %v", c.Name, ok)
	}
}

// WP7.1 accept: golden outputs per host from one fixture loop set.
func TestGolden(t *testing.T) {
	ls, err := Load(filepath.Join("testdata", "fixture"))
	if err != nil {
		t.Fatal(err)
	}
	implemented := func(v string) bool { return v != "retro" }
	files, err := Files(ls, "acme/shop", "main", "Europe/London", "v0.4.0", implemented)
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	_ = filepath.WalkDir(filepath.Join("testdata", "golden"), func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			want = append(want, filepath.ToSlash(strings.TrimPrefix(p, filepath.Join("testdata", "golden")+string(filepath.Separator))))
		}
		return nil
	})
	var got []string
	for p, b := range files {
		got = append(got, p)
		golden := filepath.Join("testdata", "golden", filepath.FromSlash(p))
		if *update {
			_ = os.MkdirAll(filepath.Dir(golden), 0o755)
			_ = os.WriteFile(golden, b, 0o644)
			continue
		}
		if w, err := os.ReadFile(golden); err != nil || string(w) != string(b) {
			t.Errorf("%s differs from %s (go test ./internal/loops -update-golden)", p, golden)
		}
	}
	slices.Sort(got)
	slices.Sort(want)
	if !*update && !slices.Equal(got, want) {
		t.Errorf("files %v, golden %v", got, want)
	}
}

// aayudha's own loops parse.
func TestRepositoryLoops(t *testing.T) {
	ls, err := Load(filepath.Join("..", ".."))
	if err != nil || len(ls) == 0 {
		t.Fatalf("%d loops: %v", len(ls), err)
	}
}

// D-7.1h: a product's default verify-sweep sets up what its lockfile names, installs through that package manager,
// and installs the Playwright browser with it; with no lockfile it has no setup. The workflow pins each tool setup.
func TestDefaultsSetup(t *testing.T) {
	const browser = "playwright install --with-deps chromium"
	for _, tt := range []struct {
		name  string
		locks []string
		tools []string
		run   []string
	}{
		{"pnpm", []string{"pnpm-lock.yaml"}, []string{"node", "pnpm"}, []string{"pnpm install --frozen-lockfile", "pnpm exec " + browser}},
		{"bun", []string{"bun.lock"}, []string{"bun"}, []string{"bun install --frozen-lockfile", "bunx " + browser}},
		{"bun binary lock", []string{"bun.lockb"}, []string{"bun"}, []string{"bun install --frozen-lockfile", "bunx " + browser}},
		{"pnpm and bun", []string{"pnpm-lock.yaml", "bun.lock"}, []string{"node", "pnpm", "bun"}, []string{"pnpm install --frozen-lockfile", "pnpm exec " + browser}},
		{"npm", []string{"package-lock.json"}, []string{"node"}, []string{"npm ci", "npx " + browser}},
		{"no lockfile", nil, nil, nil},
	} {
		dir := t.TempDir()
		for _, f := range tt.locks {
			_ = os.WriteFile(filepath.Join(dir, f), []byte("{}\n"), 0o644)
		}
		defs, err := Defaults(os.DirFS(filepath.Join("..", "..")), dir)
		if err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
		l, err := Parse("verify-sweep", defs["loops/verify-sweep.toml"])
		if err != nil || !slices.Equal(l.Setup, tt.tools) || !slices.Equal(l.SetupRun, tt.run) {
			t.Errorf("%s: setup %q, setup_run %q, %v; want %q, %q", tt.name, l.Setup, l.SetupRun, err, tt.tools, tt.run)
			continue
		}
		wf, err := ActionsWorkflow([]Loop{l}, "UTC", time.UTC, "v0.4.0", func(string) bool { return true })
		if err != nil {
			t.Fatal(err)
		}
		for _, x := range tt.tools {
			if !strings.Contains(string(wf), "uses: "+strings.SplitN(setupActions[x], "\n", 2)[0]) {
				t.Errorf("%s: the workflow does not set up %s", tt.name, x)
			}
		}
		if tt.tools == nil && strings.Contains(string(wf), "Set up for") {
			t.Errorf("%s: a setup step with no lockfile", tt.name)
		}
	}
}
