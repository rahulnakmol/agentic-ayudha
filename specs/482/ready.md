# rahulnakmol/agentic-ayudha#482: A host's PostHog or Sentry MCP server or CLI only reads: the pre-tool hook refuses every write

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `4c4248926343b0cd13f2589d0750ccb540e4a1bad5b4e95c8d5b4cd823c520a1` |
| Completion Goal hash | `5c0bb08ed9aeabdc056fb06fd406c18418a4728d947abc6a133b0bdd0c0a1d0f` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/482/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: `aayu hook pre-tool` receives, in Claude Code's, Codex's, Cursor's, and the Amp plugin's event format, a PostHog MCP read (`feature-flag-get-all`), a Sentry MCP read (`search_issues`), Sentry's `execute_sentry_tool` naming `find_releases`, and `sentry-cli releases list`; expected: exit 0 for each, and the call runs
2. [api-trace] state: the same hook receives a PostHog MCP write (`update-feature-flag`), PostHog's `exec` tool with a `call experiment-launch` line, a Sentry MCP write (`update_issue`), Sentry's `execute_sentry_tool` naming `create_alert_rule`, Sentry's paid Seer tool (`analyze_issue_with_seer`), an unknown PostHog tool, and `sentry-cli releases new 1.2.3`; expected: exit 2 (Cursor: a `"permission":"deny"` answer) with a reason that names the tool and says a write is a catalog action a person approves, or that the tool is not on aayudha's read allowlist
3. [api-trace] state: a personal-data read (`persons-list`, `get_replay_details`) with no opt-in, then with `allow_pii = true` under `[integrations.posthog]` and `[integrations.sentry]` in `.aayudha/config.toml`, and `allow_pii` set in the user config, an environment layer, or `--set`; expected: refused naming `allow_pii = true`, then allowed, and the user, environment, and `--set` settings rejected by config load naming `.aayudha/config.toml`
4. [api-trace] state: the embedded allowlists in `internal/integrations/hostcall`; expected: each names its upstream source and commit, no tool is both a read and a personal-data tool, and no read tool's name starts with a write verb (create, delete, update, launch, enable, disable, archive, resolve)
5. [api-trace] state: `LAYERS.md` after `go generate ./...`; expected: `internal/integrations/**` is in L06, Contracts / integration, owned by the Code Architect

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

As a team using aayudha in a host (Claude Code, Codex, Cursor, or Amp) that also has a PostHog or Sentry MCP server or CLI connected, I want the host's agent to be able to read from those tools but never write through them, so that a flag flip, an experiment launch, an issue resolution, or a new alert always goes through a catalog action a person approves (Decision #449, phase P0). Today aayudha's pre-tool hook guards only the halt, the budget breaker, and human-only commands; a host's own PostHog or Sentry tools can write with no approval. This Story makes the pre-tool hook judge every PostHog and Sentry MCP tool and CLI command against read-only allowlists pinned to the providers' published tool definitions, keeps personal-data reads off unless the repository opts in, and moves `internal/integrations/**` into the contracts layer L06.

### Completion Goal

1. [api-trace] state: `aayu hook pre-tool` receives, in Claude Code's, Codex's, Cursor's, and the Amp plugin's event format, a PostHog MCP read (`feature-flag-get-all`), a Sentry MCP read (`search_issues`), Sentry's `execute_sentry_tool` naming `find_releases`, and `sentry-cli releases list`; expected: exit 0 for each, and the call runs
2. [api-trace] state: the same hook receives a PostHog MCP write (`update-feature-flag`), PostHog's `exec` tool with a `call experiment-launch` line, a Sentry MCP write (`update_issue`), Sentry's `execute_sentry_tool` naming `create_alert_rule`, Sentry's paid Seer tool (`analyze_issue_with_seer`), an unknown PostHog tool, and `sentry-cli releases new 1.2.3`; expected: exit 2 (Cursor: a `"permission":"deny"` answer) with a reason that names the tool and says a write is a catalog action a person approves, or that the tool is not on aayudha's read allowlist
3. [api-trace] state: a personal-data read (`persons-list`, `get_replay_details`) with no opt-in, then with `allow_pii = true` under `[integrations.posthog]` and `[integrations.sentry]` in `.aayudha/config.toml`, and `allow_pii` set in the user config, an environment layer, or `--set`; expected: refused naming `allow_pii = true`, then allowed, and the user, environment, and `--set` settings rejected by config load naming `.aayudha/config.toml`
4. [api-trace] state: the embedded allowlists in `internal/integrations/hostcall`; expected: each names its upstream source and commit, no tool is both a read and a personal-data tool, and no read tool's name starts with a write verb (create, delete, update, launch, enable, disable, archive, resolve)
5. [api-trace] state: `LAYERS.md` after `go generate ./...`; expected: `internal/integrations/**` is in L06, Contracts / integration, owned by the Code Architect

### Measure

Completion Goal criteria observed

### Threshold

all 5 criteria

### Scenario

Pipe each host's recorded pre-tool event for the reads, writes, personal-data reads, and CLI commands into `aayu hook pre-tool --host <host>` (the Amp plugin passes it with `--event`), and read the exit code, stdout, and stderr. Repeat the personal-data reads with `allow_pii = true` in the repository config, and load the config with `allow_pii` in each other layer. Read the embedded allowlists and the generated LAYERS.md.

### Environment

local checkout and this repository's CI build job on a pull request; recorded host events only, no PostHog or Sentry account

### Checkpoints

1. `go test ./internal/integrations/hostcall ./internal/cli -run 'TestDecide|TestFromHook|TestAllowlists|TestHookProviderCalls'` shows each read allowed and each write refused with its reason (criteria 1, 2, 3, 4)
2. `go test ./internal/config -run TestAllowPIIRepoOnly` and `go generate ./...` with no drift show the opt-in only from the repository config and the layer move (criteria 3, 5)

### Assumptions

- the allowlists are generated from PostHog `services/mcp/schema/tool-definitions-all.json` at 8bb4013 and getsentry/sentry-mcp `packages/mcp-core/src/toolDefinitions.json` at 5044167 (read 2026-10-17); a read is a tool marked read-only whose required scopes are all read scopes and that spends no AI credits
- a tool the lists do not know is refused, so a provider's new write tool stays refused until the lists are regenerated
- the call log of the reads that pass is the next Story; doctor's checks of a host's provider MCP servers are the one after it
- no PostHog or Sentry account is used; the provider tools' behaviour is not live-proven here

### Provenance

Request: "does aayu work when Claude Code (or another host) has the PostHog MCP, the Sentry MCP, or their CLIs connected"
Term: "MCP" → a Model Context Protocol server that a host connects, whose tools the host's agent calls; "the PostHog MCP, the Sentry MCP, or their CLIs connected"
Term: "work when … connected" → Decision #449 option 1: the host's provider tools may read, and every write is a catalog action a person approves; "accepted, keys later, 3 yes, 4 yes"
Vague: "works when … connected" → criteria 1, 2, 3
1. stated (request): "has the PostHog MCP, the Sentry MCP, or their CLIs connected"
2. stated (Decision #449): "Any write … goes through the action catalog with a person's approval"
3. stated (Decision #449): "personal data is opt-in"
4. stated (Decision #449): "Read-only allowlists for host MCP tools"
5. stated (Decision #449): the hook and integration contracts live in the integration layer
````
