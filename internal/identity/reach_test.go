package identity

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// Decision #488: Reachable names every GitHub credential a process can reach
// by where it was found and whose it is. The same token found in two places
// is asked about once and listed with both sources; values that are not
// GitHub user tokens, sources that fail, and tokens that do not authenticate
// are left out; and no value appears in the result.
func TestReachable(t *testing.T) {
	legacy, other := strings.Repeat("ab12", 10), strings.Repeat("cd34", 10) // 40-hex: a legacy GitHub token, or another service's key
	environ := []string{"GH_TOKEN=ghu_same", "GH_PAT=ghp_other", "HOME=/home/user", "AAYU_BOT_TOKEN=ghs_install", "OLD=github_pat_revoked",
		"FLAKY=gho_flaky", "MY_GITHUB_TOKEN=" + legacy, "SENTRY_TOKEN=" + other}
	sources := map[string]func(context.Context) (string, error){
		"gh auth token":         func(context.Context) (string, error) { return "ghu_same\n", nil },
		"git credential helper": func(context.Context) (string, error) { return "", errors.New("no helper") },
		"keychain":              func(context.Context) (string, error) { return "  \n", nil },
	}
	logins := map[string]string{"ghu_same": "founder", "ghp_other": "machine", legacy: "founder2"}
	asked := map[string]int{}
	got := Reachable(context.Background(), environ, sources, func(_ context.Context, tok Token) (User, error) {
		asked[tok.Value]++
		if l, ok := logins[tok.Value]; ok {
			return User{Login: l}, nil
		}
		if tok.Value == "gho_flaky" {
			return User{}, errors.New("GET /user: context deadline exceeded")
		}
		return User{}, &StatusError{Code: 401, msg: "401 Unauthorized"}
	})
	// A rejected token is left out; one GitHub could not identify just now stays, with no login.
	want := "[{GH_TOKEN, gh auth token founder} {GH_PAT machine} {FLAKY } {MY_GITHUB_TOKEN founder2}]"
	if fmt.Sprint(got) != want || asked["ghu_same"] != 1 || asked["ghs_install"] != 0 || asked[other] != 0 {
		t.Fatalf("Reachable = %v (asked %v), want %s", got, asked, want)
	}
	if ex := Exposed(got, []string{"@Founder"}); len(ex) != 2 || ex[0].Login != "founder" || ex[1].Source != "FLAKY" {
		t.Fatalf("Exposed = %v", ex)
	}
}
