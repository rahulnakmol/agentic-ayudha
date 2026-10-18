package hostcall

import (
	"strings"
	"testing"
)

func TestDecide(t *testing.T) {
	pii := Options{AllowPII: map[string]bool{PostHog: true, Sentry: true}}
	for _, tt := range []struct {
		name     string
		e        Event
		o        Options
		provider string
		allow    bool
		reason   string // a substring of the refusal
	}{
		// PostHog MCP in tools mode (Claude Code, Codex, Amp name the server).
		{"posthog read", Event{Tool: "mcp__posthog__feature-flag-get-all"}, Options{}, PostHog, true, ""},
		{"posthog write", Event{Tool: "mcp__posthog__update-feature-flag"}, Options{}, PostHog, false, "catalog action a person approves"},
		{"posthog launch", Event{Tool: "mcp__posthog__experiment-launch"}, Options{}, PostHog, false, "`experiment-launch`"},
		{"posthog read needing a write scope", Event{Tool: "mcp__posthog__feature-flags-copy-dependencies-check"}, Options{}, PostHog, false, "not on aayudha's read allowlist"},
		{"posthog AI-credit read", Event{Tool: "mcp__posthog__surveys-summarize-responses-create"}, Options{}, PostHog, false, "not on aayudha's read allowlist"},
		{"posthog person data", Event{Tool: "mcp__posthog__persons-list"}, Options{}, PostHog, false, "allow_pii = true"},
		{"posthog person data opted in", Event{Tool: "mcp__posthog__persons-list"}, pii, PostHog, true, ""},
		{"posthog session recording", Event{Tool: "mcp__posthog__session-recording-get"}, Options{}, PostHog, false, "allow_pii = true"},
		{"posthog session recording opted in", Event{Tool: "mcp__posthog__session-recording-get"}, pii, PostHog, true, ""},
		{"posthog recordings query", Event{Tool: "mcp__posthog__query-session-recordings-list"}, Options{}, PostHog, false, "personal data"},
		{"posthog recording playlist", Event{Tool: "mcp__posthog__session-recording-playlist-get"}, Options{}, PostHog, false, "personal data"},
		{"posthog replay vision observations", Event{Tool: "mcp__posthog__vision-observations-list"}, Options{}, PostHog, false, "personal data"},
		{"posthog unknown tool", Event{Tool: "mcp__posthog__brand-new-tool"}, Options{}, PostHog, false, "not on aayudha's read allowlist"},
		{"posthog server named by its user", Event{Tool: "mcp__plugin_posthog_PostHog__delete-feature-flag"}, Options{}, PostHog, false, "`delete-feature-flag`"},
		// PostHog MCP in CLI mode: one exec tool whose command names the tool.
		{"exec read verbs", Event{Tool: "mcp__posthog__exec", Input: map[string]any{"command": "search flags\ninfo feature-flag-get-all"}}, Options{}, PostHog, true, ""},
		{"exec call read", Event{Tool: "mcp__posthog__exec", Input: map[string]any{"command": "call --json execute-sql {\n  \"query\": \"select 1\"\n}"}}, Options{}, PostHog, true, ""},
		{"exec call write", Event{Tool: "mcp__posthog__exec", Input: map[string]any{"command": "call --confirm delete-feature-flag {\"id\": 3}"}}, Options{}, PostHog, false, "`delete-feature-flag`"},
		{"exec batch with one write", Event{Tool: "mcp__posthog__exec", Input: map[string]any{"command": "call feature-flag-get-all {}\ncall feature-flag-enable {\"id\": 3}"}}, Options{}, PostHog, false, "`feature-flag-enable`"},
		{"exec unknown verb", Event{Tool: "mcp__posthog__exec", Input: map[string]any{"command": "run something"}}, Options{}, PostHog, false, "not a command"},
		// Sentry MCP.
		{"sentry read", Event{Tool: "mcp__sentry__search_issues"}, Options{}, Sentry, true, ""},
		{"sentry write", Event{Tool: "mcp__sentry__update_issue"}, Options{}, Sentry, false, "`update_issue`"},
		{"sentry seer", Event{Tool: "mcp__sentry__analyze_issue_with_seer"}, Options{}, Sentry, false, "`analyze_issue_with_seer`"},
		{"sentry onboarding update", Event{Tool: "mcp__sentry__onboarding_status_update"}, Options{}, Sentry, false, ""},
		{"sentry proxy read", Event{Tool: "mcp__sentry__execute_sentry_tool", Input: map[string]any{"name": "find_releases"}}, Options{}, Sentry, true, ""},
		{"sentry proxy write", Event{Tool: "mcp__sentry__execute_sentry_tool", Input: map[string]any{"name": "create_alert_rule"}}, Options{}, Sentry, false, "`create_alert_rule`"},
		{"sentry replay content", Event{Tool: "mcp__sentry__get_replay_details"}, Options{}, Sentry, false, "[integrations.sentry] allow_pii"},
		{"sentry replay search", Event{Tool: "mcp__sentry__search_replays"}, Options{}, Sentry, false, "[integrations.sentry] allow_pii"},
		// Cursor names the tool alone and reports the server's URL or command.
		{"cursor posthog url", Event{Tool: "update-feature-flag", Server: "https://mcp.posthog.com/mcp?readonly=true"}, Options{}, PostHog, false, ""},
		{"cursor sentry stdio", Event{Tool: "search_issues", Server: "npx @sentry/mcp-server --access-token=…"}, Options{}, Sentry, true, ""},
		// A server registered under another name is known by its URL, command, or package.
		{"renamed sentry server", Event{Tool: "mcp__errors__update_issue"}, Options{Servers: map[string]string{"errors": Sentry}}, Sentry, false, "`update_issue`"},
		{"renamed posthog server read", Event{Tool: "mcp__analytics__feature-flag-get-all"}, Options{Servers: map[string]string{"analytics": PostHog}}, PostHog, true, ""},
		// GitHub Copilot names an MCP tool <server>-<tool>; the tool part may hold hyphens.
		{"copilot posthog read", Event{Tool: "posthog-feature-flag-get-all", Dashed: true}, Options{}, PostHog, true, ""},
		{"copilot posthog write", Event{Tool: "posthog-delete-feature-flag", Dashed: true}, Options{}, PostHog, false, "`delete-feature-flag`"},
		{"copilot hyphenated server", Event{Tool: "sentry-prod-update_issue", Dashed: true}, Options{Servers: map[string]string{"sentry-prod": Sentry}}, Sentry, false, "`update_issue`"},
		{"copilot renamed server", Event{Tool: "errors-update_issue", Dashed: true}, Options{Servers: map[string]string{"errors": Sentry}}, Sentry, false, "`update_issue`"},
		{"copilot other server", Event{Tool: "aayudha-status", Dashed: true}, Options{Servers: map[string]string{"errors": Sentry}}, "", true, ""},
		// Other servers and tools are not decided here.
		{"other server", Event{Tool: "mcp__github__create_issue"}, Options{}, "", true, ""},
		{"built-in tool", Event{Tool: "edit_file"}, Options{}, "", true, ""},
		// Provider CLIs.
		{"sentry-cli read", Event{Shell: "sentry-cli releases list --org acme"}, Options{}, Sentry, true, ""},
		{"sentry-cli release", Event{Shell: "sentry-cli releases new 1.2.3"}, Options{}, Sentry, false, "product's CI"},
		{"sentry-cli through npx", Event{Shell: "npx -y @sentry/cli issues resolve 42"}, Options{}, Sentry, false, "`sentry-cli issues resolve 42`"},
		{"sentry-cli after an env assignment", Event{Shell: "SENTRY_AUTH_TOKEN=x sentry-cli deploys new -e prod"}, Options{}, Sentry, false, ""},
		{"a later write refuses the line", Event{Shell: "git log -1 && sentry-cli releases list && sentry-cli releases finalize 1.2.3"}, Options{}, Sentry, false, "finalize"},
		{"sentry cli noun read", Event{Shell: "sentry issue view ACME-12 --json"}, Options{}, Sentry, true, ""},
		{"sentry cli replay", Event{Shell: "sentry replay view 4f1c --json"}, Options{}, Sentry, false, "[integrations.sentry] allow_pii"},
		{"sentry cli replay opted in", Event{Shell: "sentry replay view 4f1c --json"}, pii, Sentry, true, ""},
		{"sentry cli user feedback", Event{Shell: "sentry feedback list"}, Options{}, Sentry, false, "personal data"},
		{"sentry cli resolve", Event{Shell: "sentry issue resolve ACME-12"}, Options{}, Sentry, false, ""},
		{"sentry cli seer", Event{Shell: "sentry issue explain ACME-12"}, Options{}, Sentry, false, ""},
		{"sentry cli raw api", Event{Shell: "sentry api /organizations/acme/issues/ -X PUT"}, Options{}, Sentry, false, ""},
		{"posthog-cli query", Event{Shell: `posthog-cli exp query run "select count() from events"`}, Options{}, PostHog, true, ""},
		{"posthog-cli api read", Event{Shell: "posthog-cli api call --json feature-flag-get-all '{}'"}, Options{}, PostHog, true, ""},
		{"posthog-cli api write", Event{Shell: "posthog-cli api call --confirm create-feature-flag '{}'"}, Options{}, PostHog, false, "`create-feature-flag`"},
		{"posthog-cli upload", Event{Shell: "posthog-cli sourcemap upload --directory dist"}, Options{}, PostHog, false, ""},
		{"sentry-cli in bash -c", Event{Shell: `bash -c 'sentry-cli releases new 1.2.3'`}, Options{}, Sentry, false, "`sentry-cli releases new 1.2.3`"},
		{"sentry-cli in sh -lc with an env assignment", Event{Shell: `sh -lc "SENTRY_AUTH_TOKEN=x sentry-cli deploys new -e prod"`}, Options{}, Sentry, false, "deploys new"},
		{"sentry-cli after env", Event{Shell: "env FOO=1 sentry-cli releases finalize 1.2.3"}, Options{}, Sentry, false, "finalize"},
		{"npx under timeout", Event{Shell: "timeout 30 npx -y @sentry/cli releases new 1.2.3"}, Options{}, Sentry, false, "releases new"},
		{"sentry-cli through xargs", Event{Shell: "echo 1.2.3 | xargs -n 1 sentry-cli releases delete"}, Options{}, Sentry, false, "releases delete"},
		{"sentry-cli in a command substitution", Event{Shell: "echo $(sentry-cli releases new 1.2.3)"}, Options{}, Sentry, false, "releases new"},
		{"posthog-cli in the background under nohup", Event{Shell: "nohup posthog-cli sourcemap upload --directory dist &"}, Options{}, PostHog, false, "sourcemap upload"},
		{"a read through bash -lc", Event{Shell: `bash -lc "sentry-cli releases list --org acme"`}, Options{}, Sentry, true, ""},
		{"global flags before the subcommand", Event{Shell: "sentry-cli --auth-token abc123 --org acme issues list"}, Options{}, Sentry, true, ""},
		{"no provider CLI", Event{Shell: "echo sentry-cli releases new && go test ./..."}, Options{}, "", true, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			v := Decide(tt.e, tt.o)
			if v.Provider != tt.provider || v.Allow != tt.allow {
				t.Fatalf("Decide = %+v; want provider %q allow %v", v, tt.provider, tt.allow)
			}
			if !tt.allow && !strings.Contains(v.Reason, tt.reason) {
				t.Errorf("reason %q; want it to contain %q", v.Reason, tt.reason)
			}
			if !tt.allow && v.Reason == "" {
				t.Error("a refusal has no reason")
			}
		})
	}
}

func TestFromHook(t *testing.T) {
	for _, tt := range []struct {
		name, event string
		want        Event
	}{
		{"claude mcp", `{"session_id":"s","tool_name":"mcp__sentry__update_issue","tool_input":{"issueId":"X-1"}}`,
			Event{Tool: "mcp__sentry__update_issue"}},
		{"claude bash", `{"tool_name":"Bash","tool_input":{"command":"sentry-cli releases new 1"}}`,
			Event{Tool: "Bash", Shell: "sentry-cli releases new 1"}},
		{"cursor mcp", `{"hook_event_name":"beforeMCPExecution","tool_name":"update-feature-flag","tool_input":"{\"id\":3}","url":"https://mcp.posthog.com/mcp"}`,
			Event{Tool: "update-feature-flag", Server: "https://mcp.posthog.com/mcp"}},
		{"cursor stdio mcp", `{"tool_name":"search_issues","tool_input":"{}","command":"npx @sentry/mcp-server"}`,
			Event{Tool: "search_issues", Server: "npx @sentry/mcp-server"}},
		{"cursor shell", `{"hook_event_name":"beforeShellExecution","command":"posthog-cli sourcemap upload"}`,
			Event{Shell: "posthog-cli sourcemap upload"}},
		{"copilot mcp", `{"sessionId":"05b234bf-9c41-41f2-8604-0ff1d8dc3011","cwd":"/r","timestamp":1792286000000,"toolName":"sentry-find_dropped_events","toolArgs":{"statsPeriod":"7d"}}`,
			Event{Tool: "sentry-find_dropped_events", Dashed: true}},
		{"copilot bash", `{"sessionId":"05b234bf-9c41-41f2-8604-0ff1d8dc3011","cwd":"/r","timestamp":1792286000000,"toolName":"bash","toolArgs":"{\"command\":\"posthog-cli exp query run \\\"SELECT 1 LIMIT 1\\\"\"}"}`,
			Event{Tool: "bash", Shell: `posthog-cli exp query run "SELECT 1 LIMIT 1"`}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			e, ok := FromHook([]byte(tt.event))
			if !ok || e.Tool != tt.want.Tool || e.Shell != tt.want.Shell || e.Server != tt.want.Server || e.Dashed != tt.want.Dashed {
				t.Fatalf("FromHook = %+v, %v; want %+v", e, ok, tt.want)
			}
		})
	}
	if e, _ := FromHook([]byte(`{"tool_name":"x","tool_input":"{\"id\":3}"}`)); e.Input["id"] != float64(3) {
		t.Errorf("a JSON-string tool_input decodes to %v; want id 3", e.Input)
	}
}

// TestAllowlists checks the embedded lists: no tool is both read and personal
// data, no write-looking name slipped in, and the tools the decisions above
// rely on are where they should be.
func TestAllowlists(t *testing.T) {
	for p, l := range lists {
		if len(l.read) == 0 || len(l.pii) == 0 {
			t.Fatalf("%s: %d read and %d personal-data tools; want both non-empty", p, len(l.read), len(l.pii))
		}
		for tool := range l.read {
			if l.pii[tool] {
				t.Errorf("%s: %s is in both sections", p, tool)
			}
			// PostHog names a read that takes a POST body <thing>-create
			// (feature-flags-test-evaluation-create), so only prefixes count.
			for _, w := range []string{"create", "delete", "update", "destroy", "launch", "enable", "disable", "archive", "resolve"} {
				if strings.HasPrefix(tool, w+"-") || strings.HasPrefix(tool, w+"_") {
					t.Errorf("%s: read list holds %s, which looks like a write", p, tool)
				}
			}
		}
	}
}
