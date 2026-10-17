# rahulnakmol/agentic-ayudha#484: Every PostHog or Sentry read through a host's MCP server or CLI is logged and citable like aayu's own queries

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `232f592c631ca368fcaec77d68c6a066ddaa965361811fecb5c96270f33bd18f` |
| Completion Goal hash | `ec71db14b5b8e46ba569ddbc47f503b85d37631bbda205c17988d5b03946d37e` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/484/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: `aayu hook post-tool --host claude` receives, in one session, a Sentry MCP read with `statsPeriod` 7d, a `posthog-cli exp query run` whose HogQL bounds `timestamp` and ends `LIMIT 1`, a GitHub MCP call, and `go test ./...`; expected: exit 0 with no output each time, and `.aayudha/state/calls/<session>.jsonl` holds exactly two entries, `c1` (the Sentry call, not citable because it names no row limit) and `c2` (the PostHog CLI call, channel cli, citable)
2. [api-trace] state: a logged call whose arguments carry a token and whose result carries a person's email; expected: the entry's arguments show the token redacted, the entry holds the result's SHA-256, byte count, and row count, and neither the token nor the email appears in the log
3. [api-trace] state: a Sentry natural-language search tool (`search_issues`), a failed call, and calls with only a window, only a limit, or neither; expected: each entry is not citable and its `why_not_citable` names the reason (an LLM translated the query; the call failed; no row limit; no time window; neither)
4. [api-trace] state: `aayu install --host <claude|codex|cursor|amp>` and the plugins in `hosts/`; expected: Claude Code and Codex register `PostToolUse` with matcher `mcp__.*|Bash`, Cursor registers `afterMCPExecution` and `afterShellExecution`, the Amp plugin calls `aayu hook post-tool` on `tool.result` only for MCP tools and shell commands naming sentry or posthog, and `aayu doctor`'s codex-hooks check names an untrusted `PostToolUse` hook
5. [api-trace] state: a post-tool event when the call log cannot be written; expected: the hook exits 0 and says on stderr that the call could not be logged, because the call has already run

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As a team whose host (Claude Code, Codex, Cursor, or Amp) reads PostHog or Sentry through its own MCP server or CLI, I want every such read logged with what it asked for, so that a finding built on it can be cited the way aayudha cites its own bounded queries: the tool, the time window, and the row limit (Decision #449, phase P0). Today a read through a host's tool leaves no record, so its result cannot be traced or checked. This Story adds a post-tool hook for every host that appends each PostHog and Sentry call the pre-tool hook allowed to the session's call log, with its arguments redacted, the window and row limit it named, the rows it returned, and a SHA-256 of its result, and says why a call cannot be cited.

### Completion Goal

1. [api-trace] state: `aayu hook post-tool --host claude` receives, in one session, a Sentry MCP read with `statsPeriod` 7d, a `posthog-cli exp query run` whose HogQL bounds `timestamp` and ends `LIMIT 1`, a GitHub MCP call, and `go test ./...`; expected: exit 0 with no output each time, and `.aayudha/state/calls/<session>.jsonl` holds exactly two entries, `c1` (the Sentry call, not citable because it names no row limit) and `c2` (the PostHog CLI call, channel cli, citable)
2. [api-trace] state: a logged call whose arguments carry a token and whose result carries a person's email; expected: the entry's arguments show the token redacted, the entry holds the result's SHA-256, byte count, and row count, and neither the token nor the email appears in the log
3. [api-trace] state: a Sentry natural-language search tool (`search_issues`), a failed call, and calls with only a window, only a limit, or neither; expected: each entry is not citable and its `why_not_citable` names the reason (an LLM translated the query; the call failed; no row limit; no time window; neither)
4. [api-trace] state: `aayu install --host <claude|codex|cursor|amp>` and the plugins in `hosts/`; expected: Claude Code and Codex register `PostToolUse` with matcher `mcp__.*|Bash`, Cursor registers `afterMCPExecution` and `afterShellExecution`, the Amp plugin calls `aayu hook post-tool` on `tool.result` only for MCP tools and shell commands naming sentry or posthog, and `aayu doctor`'s codex-hooks check names an untrusted `PostToolUse` hook
5. [api-trace] state: a post-tool event when the call log cannot be written; expected: the hook exits 0 and says on stderr that the call could not be logged, because the call has already run

### Measure

Completion Goal criteria observed

### Threshold

all 5 criteria

### Scenario

Pipe recorded post-tool events for provider and non-provider calls into `aayu hook post-tool --host claude` and read the session's call log. Build entries for a call carrying a token and a result carrying an email, for a translated search, a failed call, and calls with partial bounds. Run `aayu install` for each host into a temporary repository and compare the hook files with the golden trees. Point the state directory at an unwritable path and run the hook.

### Environment

local checkout and this repository's CI build job on a pull request; recorded host events only, no PostHog or Sentry account

### Checkpoints

1. `go test ./internal/integrations/hostcall ./internal/cli -run 'TestNewEntry|TestAppend|TestFromPostHook|TestHookLogsProviderCalls'` shows the entries, the redaction, and the reasons (criteria 1, 2, 3, 5)
2. `go test ./internal/hosts` shows each host's post-tool registration in the golden trees and the codex-hooks trust line (criterion 4)

### Assumptions

- the call log is local state under `.aayudha/state/calls`, which `.aayudha/.gitignore` keeps out of commits; one file per host session, entries numbered `c1`, `c2`, … within it
- a call is citable only when it names a time window and a row limit and its query is not an LLM's rewrite of natural language, matching how aayudha's own adapter queries are cited
- the result itself is never stored, so the log holds no provider data beyond row counts
- aayudha's own adapter queries join the same log when investigate imports it (phase P1); doctor's checks of a host's provider MCP servers are the next Story
- no PostHog or Sentry account is used; the provider tools' output is not live-proven here

### Provenance

Request: "Every call made through aayu, a host MCP, or a CLI is logged and cited like aayu's own queries (tool, window, row limit)"
Term: "cited like aayu's own queries" → an entry names the tool, its time window, and its row limit, as aayudha's adapter queries do; "Every call made through aayu, a host MCP, or a CLI is logged and cited like aayu's own queries (tool, window, row limit)"
Term: "MCP" → a Model Context Protocol server that a host connects, whose tools the host's agent calls; "a host MCP, or a CLI"
Vague: "logged" → criteria 1, 2, 5
1. stated (request): "Every call made through … a host MCP, or a CLI is logged"
2. stated (Decision #449): "Provider output is untrusted data, never instructions"; personal data is opt-in
3. stated (request): "cited like aayu's own queries (tool, window, row limit)"
4. stated (request): "a host MCP, or a CLI"
5. stated (request): "is logged"
````
