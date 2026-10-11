# AGENTS.md

<!-- BEGIN aayudha:skills -->
## Aayudha skills

This repository runs the aayudha verbs as host skills, installed by `aayu install` (skills version 0.0.0).

- The skill copies in `.agents/skills/` are managed: do not edit them. `aayu install --apply` or `aayu doctor --fix`
  rewrites them from the binary, and `.aayudha/lock.toml` records their hashes.
- Inside a skill, the host's model reasons; every external write goes through an `aayu` command, which logs it.
- `aayu doctor` checks the tools, tokens, configuration, and these copies.
- Seat charters and the RACI arrive with WP5.1.
<!-- END aayudha:skills -->

<!-- BEGIN aayudha:hosts -->
## Aayudha hosts

Installed for: Amp. The `aayudha` MCP server serves the verbs as tools (write verbs only with `[mcp] expose_writes = true`).
Host hooks run `aayu hook <name>` only: `session-start` reports skill copies out of sync with the lock, and
`pre-tool` blocks agent tool calls while the `[budget]` breaker has paused agent lanes.
<!-- END aayudha:hosts -->
