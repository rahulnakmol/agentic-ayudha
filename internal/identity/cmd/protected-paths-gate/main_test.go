package main

import (
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
