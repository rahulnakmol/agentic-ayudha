// Package identity resolves the GitHub identities aayudha writes under (WP1.4):
// the human's own token, and the bot identity (a GitHub App installation token
// or a machine user) that agent lanes and CI use. It also decides which reviews
// count as a human pass.
package identity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"slices"
	"strings"

	"github.com/rahulnakmol/agentic-ayudha/internal/secrets"
)

// HumanTokenSecret is the secrets-store name of a stored human token.
const HumanTokenSecret = "github-token"

// Token is a resolved GitHub token. String never prints the value.
type Token struct {
	Value  string
	Source string // where it came from, e.g. "AAYU_GITHUB_TOKEN"
}

func (t Token) String() string { return "token from " + t.Source }

// ErrNoToken means no source produced a human token.
var ErrNoToken = errors.New("no GitHub token found")

// Resolver resolves identities. Zero-value function fields use the real system.
type Resolver struct {
	Getenv  func(string) string
	CI      bool                                      // --ci: GITHUB_TOKEN is acceptable as the human token
	GH      func(ctx context.Context) (string, error) // `gh auth token`
	Secrets *secrets.Store
	Prompt  func(ctx context.Context) (string, error) // nil when there is no TTY
	Hint    string                                    // appended to the no-token error, for where this resolver runs
}

func (r Resolver) gh(ctx context.Context) (string, error) {
	if r.GH != nil {
		return r.GH(ctx)
	}
	if _, err := exec.LookPath("gh"); err != nil {
		return "", err
	}
	out, err := exec.CommandContext(ctx, "gh", "auth", "token").Output()
	return strings.TrimSpace(string(out)), err
}

// Human resolves the human token: AAYU_GITHUB_TOKEN, then GITHUB_TOKEN (only
// with --ci), then `gh auth token`, then the secrets store, then a TTY prompt.
func (r Resolver) Human(ctx context.Context) (Token, error) {
	if v := r.Getenv("AAYU_GITHUB_TOKEN"); v != "" {
		return Token{v, "AAYU_GITHUB_TOKEN"}, nil
	}
	if r.CI {
		if v := r.Getenv("GITHUB_TOKEN"); v != "" {
			return Token{v, "GITHUB_TOKEN (--ci)"}, nil
		}
	}
	if v, err := r.gh(ctx); err == nil && v != "" {
		return Token{v, "gh auth token"}, nil
	}
	if r.Secrets != nil {
		if v, backend, err := r.Secrets.Get(HumanTokenSecret); err == nil {
			return Token{v, "secrets store (" + backend + ")"}, nil
		}
	}
	if r.Prompt != nil {
		if v, err := r.Prompt(ctx); err == nil && v != "" {
			return Token{v, "prompt"}, nil
		}
	}
	return Token{}, fmt.Errorf("%w: set AAYU_GITHUB_TOKEN, run `gh auth login`, or `aayu auth set-secret %s`%s", ErrNoToken, HumanTokenSecret, r.Hint)
}

// User is the account behind a token, with the classic-token scopes GitHub reports.
type User struct {
	Login  string   `json:"login"`
	Type   string   `json:"type"`
	Scopes []string `json:"scopes,omitempty"` // nil for fine-grained and App tokens
	Listed bool     `json:"scopes_listed"`    // whether GitHub returned X-OAuth-Scopes
}

// WhoAmI calls GET /user with the token.
// StatusError is GitHub's answer to GET /user when it is not 200 OK.
type StatusError struct {
	Code int
	msg  string
}

func (e *StatusError) Error() string { return e.msg }

func WhoAmI(ctx context.Context, c *http.Client, api string, t Token) (User, error) {
	var u User
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, api+"/user", nil)
	if err != nil {
		return u, err
	}
	req.Header.Set("Authorization", "Bearer "+t.Value)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := c.Do(req)
	if err != nil {
		return u, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return u, &StatusError{Code: resp.StatusCode, msg: fmt.Sprintf("GET /user with %s: %s", t, resp.Status)}
	}
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return u, err
	}
	if h, ok := resp.Header["X-Oauth-Scopes"]; ok {
		u.Listed = true
		for _, s := range strings.Split(strings.Join(h, ","), ",") {
			if s = strings.TrimSpace(s); s != "" {
				u.Scopes = append(u.Scopes, s)
			}
		}
		slices.Sort(u.Scopes)
	}
	return u, nil
}

// ScopeWarnings returns one warning per over-broad scope on a human token.
// Day-to-day verbs need repo access only; `init` is the one admin-scope verb
// and asks for confirmation on a TTY.
func ScopeWarnings(u User) []string {
	var w []string
	for _, s := range u.Scopes {
		switch {
		case s == "workflow":
			w = append(w, "token has the workflow scope: it can rewrite GitHub Actions workflows, including the gates; drop it unless you are running init --gates")
		case strings.HasPrefix(s, "admin:"):
			w = append(w, fmt.Sprintf("token has the %s scope: administrative access beyond what verbs need; only init needs admin, and it confirms on a TTY", s))
		case s == "delete_repo":
			w = append(w, "token has the delete_repo scope: it can delete repositories; no aayudha verb needs it")
		}
	}
	if !u.Listed {
		w = append(w, "scopes not listed (fine-grained or App token): verify its permissions in GitHub settings")
	}
	return w
}
