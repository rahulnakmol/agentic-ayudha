package hostcall

import (
	"net/url"
	"regexp"
	"strings"
)

// Server is a host's registration of a PostHog or Sentry MCP server, as
// doctor reads it from the host's config.
type Server struct {
	URL       string
	Command   string
	Args      []string
	Env       map[string]string
	Headers   map[string]string
	Committed bool // the config file is in the repository
}

// Finding is one thing to change in a server's registration. Secret is set
// when the registration holds a credential in a file the repository shares.
type Finding struct {
	Problem, Fix string
	Secret       bool
}

// keyLiteral matches a PostHog or Sentry credential written out in full
// (phx_…, sntrys_…, sntryu_…), not a ${VAR} reference.
var keyLiteral = regexp.MustCompile(`\b(phx_[A-Za-z0-9]{16,}|sntry[su]_[A-Za-z0-9_=.+/-]{16,})`)

// Audit says what to change so a server reads only, scoped to one project,
// and holds no shared secret. aayudha's pre-tool hook refuses writes either
// way; these settings make the server itself offer less (defence in depth).
// The provider is the one the registration names; "" when it names neither.
func Audit(s Server) (provider string, findings []Finding) {
	hay := strings.ToLower(strings.Join(append([]string{s.URL, s.Command}, s.Args...), " "))
	u, _ := url.Parse(s.URL)
	q := url.Values{}
	if u != nil {
		q = u.Query()
	}
	header := func(name string) string {
		for k, v := range s.Headers {
			if strings.EqualFold(k, name) {
				return v
			}
		}
		return ""
	}
	truthy := func(v string) bool { return v == "true" || v == "1" }
	if s.Committed {
		texts := append([]string{s.URL}, s.Args...)
		for _, v := range s.Headers {
			texts = append(texts, v)
		}
		for _, v := range s.Env {
			texts = append(texts, v)
		}
		if keyLiteral.MatchString(strings.Join(texts, " ")) {
			findings = append(findings, Finding{Secret: true, Problem: "a PostHog or Sentry key is written into this file, which the repository shares",
				Fix: "revoke that key now, then reference an environment variable instead (for example \"Authorization\": \"Bearer ${POSTHOG_API_KEY}\") or move the server to your user config"})
		}
	}
	switch {
	case strings.Contains(hay, "posthog"):
		provider = PostHog
		if strings.Contains(hay, "mcp.posthog.com") {
			if !truthy(q.Get("readonly")) && !truthy(header("x-posthog-read-only")) {
				findings = append(findings, Finding{Problem: "the server offers PostHog's write tools",
					Fix: "add readonly=true to the URL (https://mcp.posthog.com/mcp?readonly=true) or the header x-posthog-read-only: true, so it lists only read tools"})
			}
			if q.Get("project_id") == "" && header("x-posthog-project-id") == "" {
				findings = append(findings, Finding{Problem: "the server is not pinned to one project, so an agent can switch projects",
					Fix: "add project_id=<your project ID> to the URL, or the header x-posthog-project-id"})
			}
		}
	case strings.Contains(hay, "sentry"):
		provider = Sentry
		skills := q.Get("skills")
		for i, a := range s.Args {
			if v, ok := strings.CutPrefix(a, "--skills="); ok {
				skills = v
			} else if a == "--skills" && i+1 < len(s.Args) {
				skills = s.Args[i+1]
			}
		}
		if v := s.Env["MCP_SKILLS"]; v != "" {
			skills = v
		}
		if skills != "inspect" {
			findings = append(findings, Finding{Problem: "the server offers Sentry skills beyond inspect (seer starts paid runs; triage and project-management write)",
				Fix: "limit it to the inspect skill: ?skills=inspect on https://mcp.sentry.dev/mcp, or --skills=inspect for npx @sentry/mcp-server"})
		}
		if strings.Contains(hay, "mcp.sentry.dev") {
			if u != nil && strings.Count(strings.Trim(u.Path, "/"), "/") < 1 {
				findings = append(findings, Finding{Problem: "the server is not limited to one organization",
					Fix: "use https://mcp.sentry.dev/mcp/<org>/<project>"})
			}
			if header("authorization") == "" {
				findings = append(findings, Finding{Problem: "the hosted server signs in with OAuth, and Sentry grants that token write scopes whatever skills you pick",
					Fix: "prefer a read-only token: run npx @sentry/mcp-server --access-token=${SENTRY_ACCESS_TOKEN} --skills=inspect (stdio), with the token aayudha's doctor accepts"})
			}
		}
	}
	return provider, findings
}
