// Package cloud holds what the Azure, AWS, and GCP adapters share (WP7.2b):
// the GitHub Actions OIDC token they federate with, the local CLI fallback
// (az, aws, gcloud) run with a scrubbed environment, short-lived token
// caching, OAuth token responses, and filter checks. Standard library only:
// no cloud SDK is linked (ground rule 2, the binary size budget).
package cloud

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/rahulnakmol/agentic-ayudha/internal/integrations"
)

// Exec runs a program with exactly env as its environment and returns its
// standard output. Tests replace it; nil means os/exec.
type Exec func(ctx context.Context, name string, args, env []string) ([]byte, error)

// Env is what the adapters read from the process: environment variables
// (the GitHub Actions OIDC request), the CLI runner, and the clock.
type Env struct {
	Getenv func(string) string
	Exec   Exec
	Now    func() time.Time
}

func (e Env) getenv(k string) string {
	if e.Getenv == nil {
		return os.Getenv(k)
	}
	return e.Getenv(k)
}

// Clock returns the current time.
func (e Env) Clock() time.Time {
	if e.Now == nil {
		return time.Now()
	}
	return e.Now()
}

// passEnv are the variables a cloud CLI may see besides PATH and HOME: its
// own configuration and login state, never a token aayudha holds (§0.1).
var passEnv = []string{
	"AZURE_CONFIG_DIR",
	"AWS_PROFILE", "AWS_CONFIG_FILE", "AWS_SHARED_CREDENTIALS_FILE", "AWS_REGION", "AWS_DEFAULT_REGION",
	"CLOUDSDK_CONFIG", "CLOUDSDK_ACTIVE_CONFIG_NAME",
	"USERPROFILE", "APPDATA", "LOCALAPPDATA", "SYSTEMROOT", "TEMP", "TMP", // Windows
}

// CLI runs a cloud CLI (az, aws, gcloud) for the auth = "cli" fallback. The
// child gets PATH, HOME, and only its own configuration variables.
func (e Env) CLI(ctx context.Context, name string, args ...string) ([]byte, error) {
	env := []string{"PATH=" + e.getenv("PATH"), "HOME=" + e.getenv("HOME")}
	for _, k := range passEnv {
		if v := e.getenv(k); v != "" {
			env = append(env, k+"="+v)
		}
	}
	run := e.Exec
	if run == nil {
		run = func(ctx context.Context, name string, args, env []string) ([]byte, error) {
			cmd := exec.CommandContext(ctx, name, args...)
			cmd.Env = env
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			out, err := cmd.Output()
			if err != nil {
				return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
			}
			return out, nil
		}
	}
	out, err := run(ctx, name, args, env)
	if err != nil {
		return nil, fmt.Errorf("%w: `%s %s` failed (auth = \"cli\" needs you logged in with %s): %w", integrations.ErrNotConfigured, name, strings.Join(args, " "), name, err)
	}
	return out, nil
}

// GitHubOIDC requests a GitHub Actions OIDC token for audience. The job
// needs `permissions: id-token: write`, which sets the two request variables.
func (e Env) GitHubOIDC(ctx context.Context, hc *http.Client, audience string) (string, error) {
	u, tok := e.getenv("ACTIONS_ID_TOKEN_REQUEST_URL"), e.getenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN")
	if u == "" || tok == "" {
		return "", fmt.Errorf("%w: auth = \"oidc\" needs a GitHub Actions job with `permissions: id-token: write` (ACTIONS_ID_TOKEN_REQUEST_URL is not set); use auth = \"cli\" or \"key\" elsewhere", integrations.ErrNotConfigured)
	}
	ru, err := url.Parse(u)
	if err != nil {
		return "", fmt.Errorf("%w: ACTIONS_ID_TOKEN_REQUEST_URL: %w", integrations.ErrNotConfigured, err)
	}
	q := ru.Query()
	q.Set("audience", audience)
	ru.RawQuery = q.Encode()
	req, err := integrations.NewRequest(ctx, http.MethodGet, ru.String(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	var res struct {
		Value string `json:"value"`
	}
	if err := integrations.DoJSON(hc, req, &res); err != nil {
		return "", err
	}
	if res.Value == "" {
		return "", fmt.Errorf("%w: the GitHub Actions OIDC endpoint returned no token", integrations.ErrProvider)
	}
	return res.Value, nil
}

// Token is a short-lived bearer token.
type Token struct {
	Value   string
	Expires time.Time
}

// Cache keeps one token per key until a minute before it expires.
type Cache struct {
	mu sync.Mutex
	m  map[string]Token
}

// Get returns the cached token for key, or mints, caches, and returns one.
func (c *Cache) Get(key string, now time.Time, mint func() (Token, error)) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if t, ok := c.m[key]; ok && now.Add(time.Minute).Before(t.Expires) {
		return t.Value, nil
	}
	t, err := mint()
	if err != nil {
		return "", err
	}
	if c.m == nil {
		c.m = map[string]Token{}
	}
	c.m[key] = t
	return t.Value, nil
}

// PostForm sends an application/x-www-form-urlencoded token request and
// decodes the JSON answer into out.
func PostForm(ctx context.Context, hc *http.Client, u string, form url.Values, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("%w: %w", integrations.ErrQuery, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "aayudha")
	return integrations.DoJSON(hc, req, out)
}

// PostJSON sends body as JSON with a bearer token (or none) and decodes the answer.
func PostJSON(ctx context.Context, hc *http.Client, u, bearer string, body, out any) error {
	b, err := Marshal(body)
	if err != nil {
		return err
	}
	req, err := integrations.NewRequest(ctx, http.MethodPost, u, bytes.NewReader(b))
	if err != nil {
		return err
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	return integrations.DoJSON(hc, req, out)
}

// Marshal encodes v as compact JSON without HTML escaping, so a query in a
// source_query stays readable.
func Marshal(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, fmt.Errorf("%w: %w", integrations.ErrQuery, err)
	}
	return bytes.TrimSpace(buf.Bytes()), nil
}

// OAuth is an OAuth 2.0 token response (Entra, Google, and Google STS).
type OAuth struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"`
}

// Token converts the response, refusing an empty token.
func (o OAuth) Token(now time.Time, from string) (Token, error) {
	if o.AccessToken == "" {
		return Token{}, fmt.Errorf("%w: %s returned no access token", integrations.ErrProvider, from)
	}
	return Token{Value: o.AccessToken, Expires: now.Add(time.Duration(o.ExpiresIn) * time.Second)}, nil
}

// CheckFilter refuses a baseline filter that could leave the predicate it is
// placed in. The filter is a predicate in the provider's query language and
// comes from an issue's Metric field, which is untrusted (§0.1): a statement
// separator, a line break, or the language's pipe to another operator (extra)
// is never needed in a predicate. The adapters end the filter's line right
// after it, so a trailing comment can only break the query, not shorten it.
func CheckFilter(source, f string, extra ...string) error {
	for _, bad := range append([]string{";", "\n", "\r"}, extra...) {
		if strings.Contains(f, bad) {
			return fmt.Errorf("%w: %s filter %q: a filter is one predicate; %q is not allowed in it", integrations.ErrQuery, source, f, bad)
		}
	}
	return nil
}
