package actions

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

// Capabilities. GitHub has no label-only token permission (Appendix B P5), so
// the restriction lives here: an action that carries a capability may only make
// the GitHub requests that capability allows, whatever its token could do.
const (
	CapLabelsOnly = "labels-only"
)

var (
	labelsPath = regexp.MustCompile(`^/repos/[^/]+/[^/]+/issues/[0-9]+/labels$`)
	labelPath  = regexp.MustCompile(`^/repos/[^/]+/[^/]+/issues/[0-9]+/labels/[^/]+$`)
)

// AllowedRequest reports whether capability permits an HTTP request. Reads are
// always allowed; an empty capability allows everything.
func AllowedRequest(capability, method, path string) error {
	if capability == "" || method == http.MethodGet || method == http.MethodHead {
		return nil
	}
	path, _, _ = strings.Cut(path, "?")
	switch capability {
	case CapLabelsOnly:
		switch {
		case (method == http.MethodPost || method == http.MethodPut) && labelsPath.MatchString(path):
			return nil
		case method == http.MethodDelete && (labelPath.MatchString(path) || labelsPath.MatchString(path)):
			return nil
		}
	default:
		return fmt.Errorf("%w: unknown capability %q", ErrCapability, capability)
	}
	return fmt.Errorf("%w: %s %s is outside %s", ErrCapability, method, path, capability)
}

// Allowed checks an action against its capability before anything is sent. A
// capability-bound action must declare its request so the check is possible.
func Allowed(a Action) error {
	if a.Capability == "" {
		return nil
	}
	if a.Request == nil {
		return fmt.Errorf("%w: %s action %s declares no request to check", ErrCapability, a.Capability, a.Kind)
	}
	return AllowedRequest(a.Capability, a.Request.Method, a.Request.Path)
}

// Guard wraps an HTTP transport so every request is checked against
// capability before it leaves the process. The GitHub client (WP2.3) uses it
// for capability-bound runs, so a perform function cannot exceed its action.
func Guard(capability string, next http.RoundTripper) http.RoundTripper {
	if next == nil {
		next = http.DefaultTransport
	}
	return guard{capability, next}
}

type guard struct {
	capability string
	next       http.RoundTripper
}

func (g guard) RoundTrip(r *http.Request) (*http.Response, error) {
	if err := AllowedRequest(g.capability, r.Method, r.URL.Path); err != nil {
		return nil, err
	}
	return g.next.RoundTrip(r)
}

// Redaction: payload keys whose name contains one of these are replaced, and
// string values that look like credentials are replaced wherever they appear.
// "token" matches as a suffix only, so access_token is redacted and max_tokens is not.
var redactKeys = []string{"secret", "password", "passwd", "authorization", "api_key", "apikey", "private_key", "cookie", "credential"}

var credential = regexp.MustCompile(`(gh[pousr]_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]{20,}|sk-[A-Za-z0-9_-]{20,}|xox[abpr]-[A-Za-z0-9-]{10,}|-----BEGIN [A-Z ]*PRIVATE KEY-----[^-]*)`)

const redacted = "[REDACTED]"

// Redact returns a copy of payload with secret-named keys and credential-shaped
// values replaced. extra adds key names (case-insensitive substrings).
func Redact(payload map[string]any, extra []string) map[string]any {
	if payload == nil {
		return nil
	}
	out, _ := redactAny(payload, extra).(map[string]any)
	return out
}

func redactAny(v any, extra []string) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			if secretKey(k, extra) {
				out[k] = redacted
				continue
			}
			out[k] = redactAny(val, extra)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, val := range x {
			out[i] = redactAny(val, extra)
		}
		return out
	case string:
		return redactString(x)
	}
	return v
}

func secretKey(k string, extra []string) bool {
	k = strings.ToLower(k)
	if strings.HasSuffix(k, "token") {
		return true
	}
	for _, s := range append(redactKeys[:len(redactKeys):len(redactKeys)], extra...) {
		if strings.Contains(k, strings.ToLower(s)) {
			return true
		}
	}
	return false
}

func redactString(s string) string { return credential.ReplaceAllString(s, redacted) }

// RedactString replaces credential-shaped substrings, as payload redaction
// does; the eval harness uses it on reply excerpts it records.
func RedactString(s string) string { return redactString(s) }
