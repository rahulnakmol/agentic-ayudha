# aayudha in Cursor

Cursor reads aayudha's skills, `AGENTS.md`, and MCP server, and runs its hooks.

## Install

```sh
aayu install --host cursor --apply
```

| File | What it holds |
|---|---|
| `.agents/skills/<verb>/` | One skill per verb |
| `AGENTS.md` | The aayudha sections, which Cursor reads as project rules |
| `.cursor/mcp.json` | The `aayudha` MCP server |
| `.cursor/hooks.json` | `sessionStart`, `beforeShellExecution`, and `beforeMCPExecution` |

Every entry calls aayu by its full, version-checked path, so each teammate runs `install` once on their machine.

If you use the aayudha plugin from Cursor's local plugin folder (`~/.cursor/plugins/local/aayudha`), `install` writes
none of these for Cursor: the plugin carries the skills, hooks, and MCP server. aayu cannot see a plugin you installed
from a marketplace, which Cursor keeps with your account, so use the plugin or `install`, not both. Cursor also reads
`.claude/skills/`, so installing for Claude Code here gives Cursor a second copy; `aayu doctor` reports it when the
plugin is in the local folder. Check it with:

```sh
aayu doctor
```

## Human-only steps

Shipping, releasing, accepting a Decision, and responding to an incident are for a person. The MCP server never offers
them, and the `beforeShellExecution` hook denies a shell command that runs one, with the exact command for you to run
in your own terminal.

## Models

Cursor does not run OpenAI models, so a lane set to GPT falls back to Grok 4.7, and aayu logs the swap.
`aayu models show --host cursor` lists every lane.
