package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGateFromAPIFiles(t *testing.T) {
	tests := []struct {
		name, codeowners, reviews string
		pass                      bool
		want                      string
	}{
		{"human CODEOWNER approval on head passes a bot PR, across paginated files and a rename", "CODEOWNERS", "reviews-human.json", true, "ok   skills/old/SKILL.md"},
		{"a bot_logins machine user listed as CODEOWNER still cannot pass it", "CODEOWNERS-machine", "reviews-machine.json", false, "bot identity"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out strings.Builder
			pass, err := run([]string{"-codeowners", "testdata/" + tt.codeowners, "-pr", "testdata/pr.json",
				"-files", "testdata/files.json", "-reviews", "testdata/" + tt.reviews, "-repo", "testdata/repo"}, &out)
			if err != nil || pass != tt.pass || !strings.Contains(out.String(), tt.want) {
				t.Fatalf("pass = %v, err = %v, output:\n%s", pass, err, out.String())
			}
		})
	}
}

// TestLocalConfigIgnoredInCI (Bug #640): a PR that commits a
// .aayudha/config.local.toml clearing bot_logins does not let the machine
// user pass as a CODEOWNER in CI; the gate warns that it ignored the file.
func TestLocalConfigIgnoredInCI(t *testing.T) {
	repo := t.TempDir()
	for name, body := range map[string]string{
		".aayudha/config.toml":       "version = 1\n[identity]\nbot_logins = [\"aayu-machine\"]\n",
		".aayudha/config.local.toml": "version = 1\n[identity]\nbot_logins = []\n",
	} {
		p := filepath.Join(repo, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("GITHUB_ACTIONS", "true")
	var out strings.Builder
	pass, err := run([]string{"-codeowners", "testdata/CODEOWNERS-machine", "-pr", "testdata/pr.json",
		"-files", "testdata/files.json", "-reviews", "testdata/reviews-machine.json", "-repo", repo}, &out)
	if err != nil || pass || !strings.Contains(out.String(), "bot identity") || !strings.Contains(out.String(), "warning: ") || !strings.Contains(out.String(), "config.local.toml") {
		t.Fatalf("pass = %v, err = %v, output:\n%s", pass, err, out.String())
	}
}
