package identity

import (
	"fmt"
	"strings"
)

// Account is a GitHub account as the REST API reports it.
type Account struct {
	Login string `json:"login"`
	Type  string `json:"type"` // "User", "Bot", "Organization"
}

// Review is a pull request review as the REST API reports it
// (GET /repos/{owner}/{repo}/pulls/{n}/reviews).
type Review struct {
	User     Account `json:"user"`
	State    string  `json:"state"` // APPROVED, CHANGES_REQUESTED, COMMENTED, DISMISSED, PENDING
	CommitID string  `json:"commit_id"`
}

// IsBot reports whether an account is a bot: GitHub type Bot, an App login
// ending in [bot], or a login listed in [identity] bot_logins (machine users).
func IsBot(a Account, botLogins []string) bool {
	if a.Type == "Bot" || strings.HasSuffix(strings.ToLower(a.Login), "[bot]") {
		return true
	}
	for _, b := range botLogins {
		if strings.EqualFold(strings.TrimPrefix(b, "@"), a.Login) {
			return true
		}
	}
	return false
}

// HumanPass checks that review is a human pass on headSHA for a PR authored by
// author: an approval, on the head commit, by a human who is not the author.
// `ship`, `decide accept`, `tell publish`, and `respond` use it (trust model §0.1).
func HumanPass(r Review, author, headSHA string, botLogins []string) error {
	switch {
	case r.State != "APPROVED":
		return fmt.Errorf("review by %s is %s, not an approval", r.User.Login, r.State)
	case IsBot(r.User, botLogins):
		return fmt.Errorf("review by %s is from a bot identity and is not a human pass", r.User.Login)
	case strings.EqualFold(r.User.Login, author):
		return fmt.Errorf("review by %s is the PR author's own", r.User.Login)
	case headSHA != "" && r.CommitID != headSHA:
		return fmt.Errorf("approval by %s is on %s, not the head commit %s", r.User.Login, short(r.CommitID), short(headSHA))
	}
	return nil
}

func short(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// LatestReviews keeps each reviewer's latest decisive review, as GitHub does
// when it decides whether a PR is approved: a COMMENTED or PENDING review does
// not replace an earlier approval or change request.
func LatestReviews(rs []Review) []Review {
	idx := map[string]int{}
	var out []Review
	for _, r := range rs {
		switch r.State {
		case "APPROVED", "CHANGES_REQUESTED", "DISMISSED":
		default:
			continue
		}
		k := strings.ToLower(r.User.Login)
		if i, ok := idx[k]; ok {
			out[i] = r
			continue
		}
		idx[k] = len(out)
		out = append(out, r)
	}
	return out
}
