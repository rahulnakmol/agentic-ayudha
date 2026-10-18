package identity

import (
	"context"
	"slices"
	"strings"
)

// Reach is one GitHub credential a process can reach, named by where it was
// found, never by its value (Decision #488).
type Reach struct {
	Source string `json:"source"` // "GH_TOKEN", "gh auth token", "git credential helper", ...
	Login  string `json:"login"`  // the account it authenticates as
}

// tokenPrefixes are the GitHub token kinds that act as a person or an OAuth
// grant: classic and fine-grained PATs, OAuth App tokens, and GitHub App user
// tokens. Installation tokens (ghs_) act as an App and cannot read /user.
var tokenPrefixes = []string{"ghp_", "github_pat_", "gho_", "ghu_"}

// Reachable lists the accounts behind every GitHub credential this process
// can reach: token-shaped environment values (any variable name), the gh
// CLI's token, and the git credential helper's answer for github.com. Each
// distinct credential is asked who it is once; one that does not
// authenticate is left out. Values never leave this function.
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
		if ok && slices.ContainsFunc(tokenPrefixes, func(p string) bool { return strings.HasPrefix(value, p) }) {
			add(name, value)
		}
	}
	names := make([]string, 0, len(sources))
	for name := range sources {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		if v, err := sources[name](ctx); err == nil {
			add(name, v)
		}
	}
	var out []Reach
	for _, c := range cands {
		u, err := whoami(ctx, Token{Value: c.value, Source: c.names[0]})
		if err != nil || u.Login == "" {
			continue
		}
		out = append(out, Reach{Source: strings.Join(c.names, ", "), Login: u.Login})
	}
	return out
}

// Exposed returns the reachable credentials that act as one of logins
// (case-insensitive, an optional leading @).
func Exposed(reach []Reach, logins []string) []Reach {
	var out []Reach
	for _, r := range reach {
		if slices.ContainsFunc(logins, func(l string) bool { return strings.EqualFold(strings.TrimPrefix(l, "@"), r.Login) }) {
			out = append(out, r)
		}
	}
	return out
}
