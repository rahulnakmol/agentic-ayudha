# rahulnakmol/agentic-ayudha#520: Host hook guard misses provider CLIs in sh -c or env and MCP servers under other names

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `236d0d3a0d79d807a87743e208734320b7a1686ab770eae01af3c077f8215617` |
| Completion Goal hash | `83bf1cedaeb8fbace2e0871991549e24965cbbc94baf3b5ddae226a8a8dfe8c6` |
| Technical Owner | devops-devex-tinkerer |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/520/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: the pre-tool decision for provider CLI writes wrapped in `bash -c`, `sh -lc` with an assignment, `env`, `timeout 30 npx`, `xargs -n 1`, `$( … )`, and `nohup … &`; expected: each refused as a write naming the unwrapped command, and a read through `bash -lc` allowed
2. [api-trace] state: the pre-tool hook given `mcp__errors__update_issue` where `.mcp.json` registers `errors` with the package `@sentry/mcp-server`; expected: refused, naming `update_issue`
3. [api-trace] state: the pre-tool decision for `sentry-cli --auth-token <token> --org acme issues list`; expected: allowed as a read

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

The pre-tool hook misses a provider CLI or MCP server that an agent reaches indirectly (Decision #449: a host's PostHog or Sentry tools only read). A CLI wrapped in `sh -c`, `bash -c`, `env`, `timeout`, `xargs`, `nohup`, a subshell, or a command substitution was not checked, so `bash -c 'sentry-cli releases new 1.2.3'` ran unrefused. An MCP server registered under a name without `posthog` or `sentry` (for example `errors` running `@sentry/mcp-server`) was not recognised in Claude Code, Codex, or Amp, so its writes passed. A global flag before the subcommand (`sentry-cli --org acme issues list`) also made a read look unknown.

### Steps to reproduce

1. In a repository set up with `aayu install`, let the host run `bash -c 'sentry-cli releases new 1.2.3'`.
2. Register `@sentry/mcp-server` in `.mcp.json` under the name `errors` and let the host call `mcp__errors__update_issue`.

### Expected behaviour

Both are refused as writes, with the reason and the route a person takes instead.

### Actual behaviour

Both pass the hook.

### Completion Goal

1. [api-trace] state: the pre-tool decision for provider CLI writes wrapped in `bash -c`, `sh -lc` with an assignment, `env`, `timeout 30 npx`, `xargs -n 1`, `$( … )`, and `nohup … &`; expected: each refused as a write naming the unwrapped command, and a read through `bash -lc` allowed
2. [api-trace] state: the pre-tool hook given `mcp__errors__update_issue` where `.mcp.json` registers `errors` with the package `@sentry/mcp-server`; expected: refused, naming `update_issue`
3. [api-trace] state: the pre-tool decision for `sentry-cli --auth-token <token> --org acme issues list`; expected: allowed as a read

### Measure

Completion Goal criteria observed

### Threshold

all 3 criteria

### Scenario

Run the pre-tool decision on wrapped CLI commands and on a call to a renamed MCP server registered in the repository's host config, and read each verdict.

### Environment

local checkout and this repository's CI build job on a pull request; no provider account

### Technical Owner

devops-devex-tinkerer

### Checkpoints

1. `go test ./internal/integrations/hostcall -run TestDecide` and `go test ./internal/cli -run TestHookProviderCalls` show each wrapped or renamed call decided (criteria 1, 2, 3)

### Assumptions

- quotes are dropped before a command is split, so a provider CLI named inside other quoted text may be refused; refusing too much is safer than missing a write
- a server whose name, URL, command, and arguments never mention PostHog or Sentry (a generic proxy) cannot be recognised
- Claude Code, Codex, and Amp name an MCP tool `mcp__<server>__<tool>` with the name the server is registered under

### Provenance

Request: "The hook guard skips CLIs wrapped in sh -c (or bash -c, env and the like), and MCP servers registered under names that don't contain posthog or sentry. Match on the server's command or package, not only its name."
Term: "MCP" → Model Context Protocol, the protocol a host uses to call a provider's tool server; "MCP servers registered under names that don't contain posthog or sentry"
1. stated (request): "skips CLIs wrapped in sh -c (or bash -c, env and the like)"
2. stated (request): "Match on the server's command or package, not only its name"
3. default (Decision #449, reads pass): a read written with a global flag first must still read as a read once flags are skipped correctly
````
