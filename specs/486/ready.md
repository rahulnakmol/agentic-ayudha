# rahulnakmol/agentic-ayudha#486: aayu doctor tries each PostHog and Sentry capability with one cheap read and names the exact fix

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `aa3ec6b51af28f715e04cb8a0d34e27ae68c6e3d74d2a899bf4942e5de06a1d6` |
| Completion Goal hash | `6fe33adbec939b893790a3654afe9707e2d6580871fed4ee0e9e475f3788fd90` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/486/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: `aayu doctor` with `[integrations.posthog]` and `[integrations.sentry]` set and keys whose providers answer each capability's read; expected: under the `integrations` row, one line per capability naming its scopes and the verbs that use it and ending `works:` with what answered (PostHog: the project answered `SELECT 1`; Sentry: the organization, its region, and the project, then the last hour of sessions and of error counts)
2. [api-trace] state: the providers answer as a real account would when the setup is wrong: PostHog 401 from the other cloud, 403 missing scope, 403 project not allowed on the key, 404 unknown project, and 429 spent query budget; Sentry organization in another region, unknown organization, unknown project, 401 invalid token, and 403 missing scope; expected: the capability's line says `does not work:` with the problem and `Fix:` with the exact change (the scope to add and where, the other region's URL to set, or the project or organization to set), Sentry's later reads say they were skipped until the organization check passes, and the row is FAIL
3. [api-trace] state: hosts with PostHog and Sentry MCP servers registered in the repository's `.mcp.json`, `.cursor/mcp.json`, or `.codex/config.toml`, or in the person's `~/.claude.json`, `~/.cursor/mcp.json`, Amp settings, or Codex config: a PostHog server without read-only mode or a pinned project, a Sentry server on OAuth or with skills beyond inspect, a server set up to read only, and a key written into a committed file; expected: `provider-mcp` is WARN with one line per loose setting naming the fix, PASS for the read-only server, FAIL for the committed key, and PASS with no provider server registered
4. [api-trace] state: the recorded provider answers in `internal/integrations/providers/testdata/doctor` and `TestLive` run without `AAYU_LIVE=1` and the `AAYU_LIVE_*` variables; expected: each recording's `source` names the provider's OpenAPI specification, its version or commit, and the date read, and says it is not live-proven; `TestLive` skips naming the variable to set, and with them set it runs the same capability checks against the real account

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As a team bringing its own PostHog or Sentry key to aayudha, I want my first `aayu doctor` to tell me in one command which of the things aayudha reads from the provider work and which do not, each with the exact fix, so that I learn whether my setup is right before a verb fails halfway (Decision #449, phase P0). Today doctor checks only that a key is reachable and reads only; a key with the wrong region, a missing scope, or an unknown project passes and fails later in `measure` or `readout`, and nothing checks the PostHog or Sentry MCP servers a host has registered. This Story makes doctor try each capability with one cheap read, turns a real account's errors into the problem and its fix, and adds a `provider-mcp` check of every host's PostHog and Sentry MCP servers.

### Completion Goal

1. [api-trace] state: `aayu doctor` with `[integrations.posthog]` and `[integrations.sentry]` set and keys whose providers answer each capability's read; expected: under the `integrations` row, one line per capability naming its scopes and the verbs that use it and ending `works:` with what answered (PostHog: the project answered `SELECT 1`; Sentry: the organization, its region, and the project, then the last hour of sessions and of error counts)
2. [api-trace] state: the providers answer as a real account would when the setup is wrong: PostHog 401 from the other cloud, 403 missing scope, 403 project not allowed on the key, 404 unknown project, and 429 spent query budget; Sentry organization in another region, unknown organization, unknown project, 401 invalid token, and 403 missing scope; expected: the capability's line says `does not work:` with the problem and `Fix:` with the exact change (the scope to add and where, the other region's URL to set, or the project or organization to set), Sentry's later reads say they were skipped until the organization check passes, and the row is FAIL
3. [api-trace] state: hosts with PostHog and Sentry MCP servers registered in the repository's `.mcp.json`, `.cursor/mcp.json`, or `.codex/config.toml`, or in the person's `~/.claude.json`, `~/.cursor/mcp.json`, Amp settings, or Codex config: a PostHog server without read-only mode or a pinned project, a Sentry server on OAuth or with skills beyond inspect, a server set up to read only, and a key written into a committed file; expected: `provider-mcp` is WARN with one line per loose setting naming the fix, PASS for the read-only server, FAIL for the committed key, and PASS with no provider server registered
4. [api-trace] state: the recorded provider answers in `internal/integrations/providers/testdata/doctor` and `TestLive` run without `AAYU_LIVE=1` and the `AAYU_LIVE_*` variables; expected: each recording's `source` names the provider's OpenAPI specification, its version or commit, and the date read, and says it is not live-proven; `TestLive` skips naming the variable to set, and with them set it runs the same capability checks against the real account

### Measure

Completion Goal criteria observed

### Threshold

all 4 criteria

### Scenario

Replay the recorded PostHog and Sentry answers, one recording per setup, into the capability checks and into `aayu doctor` with both integrations configured, and read each line. Write each host's MCP configuration with the provider servers into a temporary repository and home, and read the `provider-mcp` row. Read each recording's source, and run `TestLive` with and without the live variables.

### Environment

local checkout and this repository's CI build job on a pull request; recorded provider answers shaped from the providers' OpenAPI specifications, no PostHog or Sentry account

### Checkpoints

1. `go test ./internal/integrations/providers ./internal/cli -run 'TestCapabilities|TestDoctorIntegrations|TestLive'` shows each capability working or failing with its fix, and the live test's skip (criteria 1, 2, 4)
2. `go test ./internal/doctor ./internal/integrations/hostcall -run 'TestChecksGolden|TestIntegrationCapabilities|TestAudit'` shows the `integrations` lines and the `provider-mcp` row for each host setup (criteria 1, 3)

### Assumptions

- the recordings are shaped from Sentry's OpenAPI specification (getsentry/sentry-api-schema at b1eac6b, 2026-10-16) and PostHog's (OpenAPI 3.1 v1.0.0 from us.posthog.com/api/schema), both read 2026-10-17, with error bodies from each provider's source; the founder has no product account, so no capability is live-proven and a user's first `aayu doctor` is the first real run
- each capability makes at most one cheap read per run (PostHog `SELECT 1` reads no events; Sentry reads one hour), so doctor spends almost none of a provider's query budget
- a key in a person's own config file is the person's choice; only a key in a file the repository commits fails
- capabilities that later phases add (P1 to P4) bring their own checks

### Provenance

Request: "aayu doctor checks the key scope and makes one cheap read per configured capability, showing exactly which work and which do not, so a user learns in one command whether their setup is right"
Term: "capability" → one thing aayudha reads from a provider for a verb (PostHog HogQL queries; Sentry crash-free rates and error counts), with the scopes it needs; "one cheap read per configured capability"
Term: "MCP" → a Model Context Protocol server that a host connects, whose tools the host's agent calls; "Read-only allowlists for host MCP tools"
Vague: "actionable" → criteria 2, 3
1. stated (request): "makes one cheap read per configured capability, showing exactly which work and which do not"
2. stated (request): "Make errors from a real account actionable: name the missing scope, the wrong region, or the unknown project, each with the exact fix"
3. stated (Decision #449): "Read-only allowlists for host MCP tools" and "Keys with the least scope, checked by doctor"
4. stated (request): "Cite the spec version and the date you read it" and "Keep the live tests, skipped with a clear reason when the AAYU_LIVE_* variables are absent"
````
