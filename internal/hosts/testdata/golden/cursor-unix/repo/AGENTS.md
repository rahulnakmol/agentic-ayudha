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

Installed for: Cursor. The `aayudha` MCP server serves the verbs as tools (write verbs only with `[mcp] expose_writes = true`).
Host hooks run `aayu hook <name>` only: `session-start` reports skill copies out of sync with the lock,
`pre-tool` blocks agent tool calls while the `[budget]` breaker has paused agent lanes and blocks shell commands that
run a human-only verb, and `status-line` shows where the work stands.

Before you change anything for a request, run `aayu grill story --assess --idea "<the person's words>" --json` (the
grill skill). On `unpack`, ask the person its questions, riskiest first, and wait for their answers before you file,
ready, or build anything. Work only on the issue the person asked for: never pick up an open or Ready story on your
own. `aayu build` holds a story the person has not asked for in this session: ask them, and pass their words with
`--asked "<their words>"`.
<!-- END aayudha:hosts -->
