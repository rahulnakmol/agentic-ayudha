package identity

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const codeowners = `# comment
/.github/                @rahulnakmol
/skills/                 @rahulnakmol
/CONSTRAINTS.md          @rahulnakmol
/ops/                    @rahulnakmol @second-human
/ops/catalog-free.md     @second-human
`

func review(login, typ, state, sha string) Review {
	return Review{User: Account{Login: login, Type: typ}, State: state, CommitID: sha}
}

func TestProtectedPathsGate(t *testing.T) {
	const head = "h1"
	human := review("rahulnakmol", "User", "APPROVED", head)
	tests := []struct {
		name    string
		changed []string
		author  string
		reviews []Review
		pass    bool
		want    string
	}{
		{"no protected path", []string{"internal/cli/root.go", "CONSTRAINTS.md.bak", ".githubx/a"}, "rahulnakmol", nil, true, "No protected paths"},
		{"protected path, no review", []string{"skills/ready/SKILL.md"}, "rahulnakmol", nil, false, "FAIL skills/ready/SKILL.md (/skills/)"},
		{"founder's own PR cannot pass", []string{".github/workflows/ci.yml"}, "rahulnakmol", []Review{human}, false, "PR author's own"},
		{"bot-authored PR approved by the human CODEOWNER on head", []string{".github/workflows/ci.yml", "internal/x.go"}, "aayudha-bot[bot]", []Review{human}, true, "approved by CODEOWNER rahulnakmol"},
		{"approval on an older commit", []string{"CONSTRAINTS.md"}, "aayudha-bot[bot]", []Review{review("rahulnakmol", "User", "APPROVED", "h0")}, false, "not the head commit"},
		{"bot approval", []string{"skills/x"}, "someone", []Review{review("rahulnakmol[bot]", "Bot", "APPROVED", head)}, false, "FAIL"},
		{"machine user in bot_logins", []string{"skills/x"}, "aayudha-bot[bot]", []Review{review("rahulnakmol", "User", "APPROVED", head)}, false, "bot identity"},
		{"human who is not a CODEOWNER", []string{"skills/x"}, "aayudha-bot[bot]", []Review{review("drive-by", "User", "APPROVED", head)}, false, "FAIL"},
		{"approval then changes requested", []string{"skills/x"}, "aayudha-bot[bot]", []Review{human, review("rahulnakmol", "User", "CHANGES_REQUESTED", head)}, false, "CHANGES_REQUESTED"},
		{"comment after approval keeps the approval", []string{"skills/x"}, "aayudha-bot[bot]", []Review{human, review("rahulnakmol", "User", "COMMENTED", head)}, true, "approved"},
		{"last matching rule wins: only the narrower owner counts", []string{"ops/catalog-free.md"}, "aayudha-bot[bot]", []Review{human}, false, "@second-human"},
		{"every protected path needs its own owner", []string{"ops/a.toml", "skills/x"}, "aayudha-bot[bot]", []Review{review("second-human", "User", "APPROVED", head)}, false, "FAIL skills/x"},
		{"rename out of a protected path", []string{"skills/x", "docs/x"}, "aayudha-bot[bot]", nil, false, "FAIL skills/x"},
	}
	for _, glob := range []string{"*.md @rahulnakmol", "docs/ @rahulnakmol", "/skills/**/x @rahulnakmol"} {
		res := ProtectedPathsGate(GateInput{Codeowners: codeowners + glob + "\n", Changed: []string{"README.md"}, Author: "a", HeadSHA: head})
		if res.Pass || !strings.Contains(res.Lines[0], "fails closed") {
			t.Errorf("unsupported pattern %q: %v", glob, res)
		}
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var botLogins []string
			if strings.Contains(tt.name, "bot_logins") {
				botLogins = []string{"rahulnakmol"}
			}
			res := ProtectedPathsGate(GateInput{Codeowners: codeowners, Changed: tt.changed, Author: tt.author, HeadSHA: head, Reviews: tt.reviews, BotLogins: botLogins})
			out := strings.Join(res.Lines, "\n")
			if res.Pass != tt.pass || !strings.Contains(out, tt.want) {
				t.Fatalf("pass = %v, want %v; output:\n%s\nwant it to contain %q", res.Pass, tt.pass, out, tt.want)
			}
		})
	}
}

// Decision #1255: this repository's CODEOWNERS protects only the policy keys.
// A change to skills, the host config, the gate's code, or a workflow passes
// the gate with no review; a change to CODEOWNERS or a config file still
// needs a CODEOWNER's approval.
func TestRepositoryCodeownersProtectOnlyPolicyKeys(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", ".github", "CODEOWNERS"))
	if err != nil {
		t.Fatal(err)
	}
	gate := func(p string) GateResult {
		return ProtectedPathsGate(GateInput{Codeowners: string(b), Changed: []string{p}, Author: "aayudha-agent", HeadSHA: "abc"})
	}
	for _, p := range []string{"skills/build/SKILL.md", "seats/founder/persona.toml", "ops/catalog.toml", "CONSTRAINTS.md", ".agents/setup",
		".claude/settings.json", ".mcp.json", "hosts/amp/aayudha.ts", "internal/cli/ship.go", ".github/workflows/ci.yml", ".github/pull_request_template.md"} {
		if r := gate(p); !r.Pass {
			t.Errorf("%s is protected: %v", p, r.Lines)
		}
	}
	for _, p := range []string{".github/CODEOWNERS", ".aayudha/config.toml", "internal/config/defaults.toml", ".aayudha/config.json",
		".aayudha/config.local.toml", ".aayudha/config.local.json"} {
		if r := gate(p); r.Pass || !slices.Equal(r.Unapproved, []string{p}) {
			t.Errorf("%s passes the gate unreviewed: %+v", p, r)
		}
	}
}
