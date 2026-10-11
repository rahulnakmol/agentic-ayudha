package identity

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func loadReviews(t *testing.T) []Review {
	t.Helper()
	b, err := os.ReadFile("testdata/reviews.json")
	if err != nil {
		t.Fatal(err)
	}
	var rs []Review
	if err := json.Unmarshal(b, &rs); err != nil {
		t.Fatal(err)
	}
	return rs
}

func TestHumanPass(t *testing.T) {
	rs := loadReviews(t)
	botLogins := []string{"@AAYU-Machine"} // case and @ must not matter
	tests := []struct {
		review int
		author string
		want   string // "" means it counts as a human pass
	}{
		{0, "aayudha-bot[bot]", "from a bot identity"}, // the App approving its own work
		{0, "someone", "from a bot identity"},          // an App approving anyone's
		{1, "aayudha-bot[bot]", "from a bot identity"}, // machine user listed in bot_logins
		{2, "aayudha-bot[bot]", "from a bot identity"}, // type Bot without the [bot] suffix
		{3, "aayudha-bot[bot]", "not the head commit"}, // stale approval
		{4, "aayudha-bot[bot]", "not an approval"},     // a comment
		{5, "rahulnakmol", "PR author's own"},          // self-approval
		{5, "aayudha-bot[bot]", ""},                    // human on a bot-authored PR: the pass
	}
	for _, tt := range tests {
		err := HumanPass(rs[tt.review], tt.author, "HEADSHA", botLogins)
		switch {
		case tt.want == "" && err != nil:
			t.Errorf("review %d by %s on a PR by %s: %v, want a human pass", tt.review+1, rs[tt.review].User.Login, tt.author, err)
		case tt.want != "" && (err == nil || !strings.Contains(err.Error(), tt.want)):
			t.Errorf("review %d by %s on a PR by %s: %v, want %q", tt.review+1, rs[tt.review].User.Login, tt.author, err, tt.want)
		}
	}
	// Without bot_logins the machine user is indistinguishable from a human:
	// that is why [identity] bot_logins exists.
	if err := HumanPass(rs[1], "aayudha-bot[bot]", "HEADSHA", nil); err != nil {
		t.Errorf("unlisted machine user: %v", err)
	}
}
