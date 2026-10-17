package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpgradeAndNewerSchemaGuard(t *testing.T) {
	repo := gitRepo(t)
	cfg := filepath.Join(repo, ".aayudha", "config.toml")
	_ = os.MkdirAll(filepath.Dir(cfg), 0o755)

	// A repo written by a newer aayu: write verbs refuse with exit 2 and the hint.
	_ = os.WriteFile(cfg, []byte("version = 2\n"), 0o644)
	code, _, errs := run(t, nil, Options{Dir: repo}, "build", "12")
	if code != ExitUsage || !strings.Contains(errs, "newer than this aayu") || !strings.Contains(errs, "aayu self-update") {
		t.Fatalf("build on a newer repo: exit %d: %s", code, errs)
	}

	// A v0 config: upgrade shows the diff, writes nothing, then --apply writes it.
	v0 := "[lfs]\nserver = \"github\"\n"
	_ = os.WriteFile(cfg, []byte(v0), 0o644)
	code, out, errs := run(t, nil, Options{Dir: repo}, "upgrade")
	if code != 0 || !strings.Contains(out, "-server = \"github\"") || !strings.Contains(out, "+version = 1") {
		t.Fatalf("upgrade: exit %d\n%s%s", code, out, errs)
	}
	if b, _ := os.ReadFile(cfg); string(b) != v0 {
		t.Fatal("upgrade without --apply wrote the file")
	}
	if code, _, _ := run(t, map[string]string{"AAYU_AGENT": "1"}, Options{Dir: repo}, "upgrade", "--apply"); code != ExitBlocked {
		t.Fatalf("agent --apply: exit %d", code)
	}
	if code, _, errs := run(t, nil, Options{IsTerminal: stdinTTY, Dir: repo}, "upgrade", "--apply"); code != 0 {
		t.Fatal(errs)
	}
	if b, _ := os.ReadFile(cfg); string(b) != "version = 1\n\n[lfs]\n" {
		t.Fatalf("applied config = %q", b)
	}
	if _, out, _ := run(t, nil, Options{Dir: repo}, "upgrade"); !strings.Contains(out, "nothing to upgrade") {
		t.Fatalf("second upgrade: %s", out)
	}
}
