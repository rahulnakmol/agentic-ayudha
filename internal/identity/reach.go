package identity

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"slices"
	"strings"
)

// Reach is one GitHub credential a process can reach, named by where it was
// found, never by its value (Decision #488).
type Reach struct {
	Source string `json:"source"` // "GH_TOKEN", "gh auth token", "git credential helper", ...
	Login  string `json:"login"`  // the account it authenticates as; "" when GitHub did not say (unknown counts as exposed)
}

// tokenPrefixes are the GitHub token kinds that act as a person or an OAuth
// grant: classic and fine-grained PATs, OAuth App tokens, and GitHub App user
// tokens. Installation tokens (ghs_) act as an App and cannot read /user.
var tokenPrefixes = []string{"ghp_", "github_pat_", "gho_", "ghu_"}

// ErrUnreadable is a source's answer when it holds credentials but cannot be
// read just now (a locked keychain, an unreadable store), as opposed to
// holding none. Reachable counts it as an unknown credential.
var ErrUnreadable = errors.New("credential source unreadable")

// legacyToken is the old 40-character hex token format, recognised only in a
// variable named for GitHub so another service's secret is never sent to it.
var legacyToken = regexp.MustCompile(`^[0-9a-f]{40}$`)

// offLimits reports a variable with a WTW name segment (WTW_*, GH_WTW_*): it
// belongs to another organisation, and aayudha never reads or sends it, not
// even to ask GitHub whose it is.
func offLimits(name string) bool {
	return slices.Contains(strings.Split(strings.ToUpper(name), "_"), "WTW")
}

func githubNamed(name string) bool {
	for _, part := range strings.Split(strings.ToUpper(name), "_") {
		if part == "GH" || part == "GITHUB" || part == "PAT" {
			return true
		}
	}
	return false
}

// Reachable lists the accounts behind every GitHub credential this process
// can reach: token-shaped environment values (any variable name except the
// off-limits WTW ones), the gh
// CLI's token, and the git credential helper's answer for github.com. Each
// distinct credential is asked who it is once; one GitHub rejects (401) is
// left out, and one it cannot identify now is kept with no login. Values never leave this function.
func Reachable(ctx context.Context, environ []string, sources map[string]func(context.Context) (string, error), whoami func(context.Context, Token) (User, error)) []Reach {
	type cand struct {
		names []string
		value string
	}
	var cands []*cand
	add := func(name, value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		for _, c := range cands {
			if c.value == value {
				c.names = append(c.names, name)
				return
			}
		}
		cands = append(cands, &cand{names: []string{name}, value: value})
	}
	for _, kv := range environ {
		name, value, ok := strings.Cut(kv, "=")
		if ok && !offLimits(name) && (slices.ContainsFunc(tokenPrefixes, func(p string) bool { return strings.HasPrefix(value, p) }) || githubNamed(name) && legacyToken.MatchString(value)) {
			add(name, value)
		}
	}
	names := make([]string, 0, len(sources))
	for name := range sources {
		names = append(names, name)
	}
	slices.Sort(names)
	var out []Reach
	for _, name := range names {
		v, err := sources[name](ctx)
		switch {
		case err == nil:
			add(name, v)
		case errors.Is(err, ErrUnreadable): // it may hold anyone's token: it counts, with no login
			out = append(out, Reach{Source: name + " (unreadable now)"})
		}
	}
	for _, c := range cands {
		u, err := whoami(ctx, Token{Value: c.value, Source: c.names[0]})
		var se *StatusError
		if errors.As(err, &se) && se.Code == http.StatusUnauthorized {
			continue // GitHub says it authenticates as no one
		}
		if err != nil { // a timeout, a rate limit, a 5xx: it may still be anyone's, so it counts
			u.Login = ""
		}
		out = append(out, Reach{Source: strings.Join(c.names, ", "), Login: u.Login})
	}
	return out
}

// Exposed returns the reachable credentials that act as one of logins
// (case-insensitive, an optional leading @), or whose account is unknown.
func Exposed(reach []Reach, logins []string) []Reach {
	var out []Reach
	for _, r := range reach {
		if r.Login == "" || slices.ContainsFunc(logins, func(l string) bool { return strings.EqualFold(strings.TrimPrefix(l, "@"), r.Login) }) {
			out = append(out, r)
		}
	}
	return out
}
