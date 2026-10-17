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

Cursor also reads `.claude/skills/` when "Include Third-Party Plugins, Skills, and Other Configs" is on (Settings →
Agents; on by default). So when you install for Claude Code as well, Cursor uses Claude Code's copies in
`.claude/skills/` and `install` writes no second set to `.agents/skills/`. Only Claude Code and Codex together need
copies in both folders; then Cursor lists each skill twice, and `aayu doctor` warns: turn the import off on that
machine, or install for one of the two. If you turn the import off with Claude Code and Cursor installed and Codex not,
Cursor sees no aayudha skills; run `aayu install --host codex --apply` to add `.agents/skills/`.

Use the aayudha Cursor plugin or `install` in a repository, not both. Cursor has no setting a repository can commit to
turn a plugin off, so with both, Cursor lists each skill and the MCP server twice; disable the plugin for that
workspace in Cursor. aayu sees the plugin only in Cursor's local plugin folder (`~/.cursor/plugins/local/aayudha`). It
cannot see a plugin you installed from a marketplace, which Cursor keeps with your account, so `aayu doctor` cannot
report that double. Check it with:

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
