package identity

import (
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
