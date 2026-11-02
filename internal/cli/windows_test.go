package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Story #1254, criterion 1: in a repository whose config sets no [time]
// windows, `aayu config show` lists the two default windows, 10:30-11:30 and
// 18:30-19:30, with the zone they are in.
func TestConfigShowListsPersonWindows(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		".git/HEAD":            "ref: refs/heads/dev\n",
		".aayudha/config.toml": "version = 1\n[time]\ntz = \"Asia/Kolkata\"\n",
	} {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	code, out, errOut := run(t, map[string]string{}, Options{Dir: dir}, "config", "show")
	want := "[time]\nbusiness_hours = 'Mon-Fri 09:00-18:00'\ntz = 'Asia/Kolkata'\nwindows = ['10:30-11:30', '18:30-19:30']\n"
	if code != ExitOK || !strings.Contains(out, want) {
		t.Fatalf("config show: exit %d, want %q in:\n%s%s", code, want, out, errOut)
	}
}
