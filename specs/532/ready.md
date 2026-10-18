# rahulnakmol/agentic-ayudha#532: Host guard does not check PostHog or Sentry calls under GitHub Copilot

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, bug |
| Body hash | `f0b701262e4eda1961c974f6567136ed68f9177fe9b41a2615de82720413a776` |
| Completion Goal hash | `cbc9415abb71b0fe4681548756db9aba78c56958fe5b673418c697699f0553b9` |
| Technical Owner | devops-devex-tinkerer |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/532/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: Copilot `preToolUse` events for a Sentry MCP read, a PostHog MCP write, a `bash` PostHog CLI read with `toolArgs` as a JSON string, and a `bash` Sentry CLI release wrapped in `bash -c`; expected: the reads exit 0, and the writes exit 2 with Copilot's deny decision and the reason
2. [api-trace] state: the pre-tool decision for Copilot MCP tool names whose tool part holds hyphens, whose server is registered under a hyphenated or unrelated name, and a non-provider server; expected: each split at the registered server name and decided as that provider's, and the non-provider tool not decided
3. [api-trace] state: the AGENTS.md section `aayu install` writes for Copilot, and the Copilot host guide; expected: neither says the guard does not cover Copilot

Editing the body after Ready removes `status:ready` (Ready guard); run `aayu ready` again to re-snapshot.

## Body at Ready

````markdown
### Summary

Under GitHub Copilot, the pre-tool hook does not check PostHog or Sentry calls at all (Decision #449: a host's PostHog or Sentry tools only read). Copilot's `preToolUse` event sends camelCase `toolName` and `toolArgs`, and names an MCP tool `<server>-<tool>`; `hostcall.FromHook` reads only the snake_case events of Claude Code, Codex, Cursor, and Amp, so it returned false and every Copilot call passed. The repository's AGENTS.md and the Copilot host guide say so as a known gap.

### Steps to reproduce

1. In a repository set up with `aayu install --host copilot`, let Copilot call PostHog's MCP tool `update-feature-flag` (toolName `posthog-update-feature-flag`).
2. Let it run `bash -c 'sentry-cli releases new 1.2.3'` in its `bash` tool.

### Expected behaviour

Both are refused with exit code 2 and Copilot's `{"permissionDecision":"deny",…}`, naming the write and the route a person takes instead; reads pass.

### Actual behaviour

Both pass the hook.

### Completion Goal

1. [api-trace] state: Copilot `preToolUse` events for a Sentry MCP read, a PostHog MCP write, a `bash` PostHog CLI read with `toolArgs` as a JSON string, and a `bash` Sentry CLI release wrapped in `bash -c`; expected: the reads exit 0, and the writes exit 2 with Copilot's deny decision and the reason
2. [api-trace] state: the pre-tool decision for Copilot MCP tool names whose tool part holds hyphens, whose server is registered under a hyphenated or unrelated name, and a non-provider server; expected: each split at the registered server name and decided as that provider's, and the non-provider tool not decided
3. [api-trace] state: the AGENTS.md section `aayu install` writes for Copilot, and the Copilot host guide; expected: neither says the guard does not cover Copilot

### Measure

Completion Goal criteria observed

### Threshold

all 3 criteria

### Scenario

Run the pre-tool hook on Copilot's own event format for MCP and shell calls, and read the exit codes, the deny output, and the installed AGENTS.md section.

### Environment

local checkout and this repository's CI build job on a pull request; event shapes seen live with Copilot CLI 1.0.98 on 2026-10-17 by the Copilot host thread; no provider account

### Technical Owner

devops-devex-tinkerer

### Checkpoints

1. `go test ./internal/integrations/hostcall -run 'TestFromHook|TestDecide'`, `go test ./internal/cli -run TestHookProviderCalls`, and `go test ./internal/hosts -run TestGoldenTrees` show the Copilot events decided and the gap note gone (criteria 1, 2, 3)

### Assumptions

- Copilot's Windows shell tool is `powershell` (not proven live); its command is checked like `bash`'s
- Copilot reads MCP servers from the repository's `.mcp.json` and from `~/.copilot/mcp-config.json`; a `COPILOT_HOME` elsewhere is not read
- the calls the guard lets through are not logged under Copilot until aayu installs Copilot's `postToolUse` hook

### Provenance

Request: "the Copilot hook format, so the PostHog and Sentry read guard can cover Copilot once #525 lands"
Term: "MCP" → Model Context Protocol, the protocol a host uses to call a provider's tool server; "An MCP tool's toolName is \"<server>-<tool>\""
1. stated (Copilot host thread): "hostcall.FromHook returns false today, so Decide never runs under Copilot"
2. stated (Copilot host thread): "split on the first hyphen after a known server name, not the last"
3. stated (Copilot host thread): "Please remove that line in the same PR that extends the guard"
````
