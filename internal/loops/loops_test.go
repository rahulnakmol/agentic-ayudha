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
}

func TestParseRefuses(t *testing.T) {
	for name, body := range map[string]string{
		"--yes":              `command = ["status", "--digest", "--yes"]` + "\ncron = \"0 9 * * *\"",
		"a human-only flag":  `command = ["decide", "4", "--accept", "A"]` + "\ncron = \"0 9 * * *\"",
		"a human pass":       `command = ["ship", "4"]` + "\ncron = \"0 9 * * *\"",
		"a terminal":         `command = ["init"]` + "\ncron = \"0 9 * * *\"",
		"not a command":      `command = ["rm", "-rf"]` + "\ncron = \"0 9 * * *\"",
		"day of both kinds":  `command = ["status"]` + "\ncron = \"0 9 1 * 1\"",
		"two minutes":        `command = ["status"]` + "\ncron = \"0,30 9 * * *\"",
		"unknown key":        `command = ["status"]` + "\ncron = \"0 9 * * *\"\nshell = \"x\"",
		"unknown host":       `command = ["status"]` + "\ncron = \"0 9 * * *\"\nhosts = [\"jenkins\"]",
		"hourly on weekdays": `command = ["status"]` + "\ncron = \"0 * * * 1-5\"",
		"an unpinned setup":  `command = ["status"]` + "\ncron = \"0 9 * * *\"\nsetup = [\"python\"]",
	} {
		if _, err := Parse("x", []byte("description = \"x\"\n"+body+"\n")); err == nil {
			t.Errorf("%s: parsed", name)
		}
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
