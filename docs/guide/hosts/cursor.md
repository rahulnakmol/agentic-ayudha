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
| `.cursor/hooks.json` | `sessionStart`, `beforeShellExecution`, `beforeMCPExecution`, `afterShellExecution`, and `afterMCPExecution` |
| `.cursor/commands/aayudha-<name>.md` | `/aayudha-home`, `/aayudha-next`, and the hand-offs `/aayudha-claude`, `/aayudha-codex`, `/aayudha-amp` |
| `.cursor/cli.json` | `Shell(aayu)` and `Mcp(aayudha:*)` in `permissions.allow`, so the Cursor CLI may run aayu and the aayudha tools |

Every entry calls aayu by its full, version-checked path, so each teammate runs `install` once on their machine.

## Approve the MCP server once

The Cursor CLI loads a project's MCP server only after you approve it. Run this once in the repository:

```sh
cursor-agent mcp enable aayudha
cursor-agent mcp list        # aayudha: ready
```

The Cursor IDE asks on its own. Cursor ties the approval to the exact `.cursor/mcp.json` entry, so approve it again
after an `install` that changes it.

## Run Cursor headless

Cursor's print mode (`cursor-agent -p`) runs a shell command or an MCP tool without asking only when the CLI's
permissions allow it. `install` adds `Shell(aayu)` and `Mcp(aayudha:*)` to `.cursor/cli.json` and leaves your other
entries alone. The hooks still refuse a human-only verb, and the MCP server never offers one. Without these entries,
print mode rejects the calls ("User rejected MCP: aayudha-grill") unless you pass `--force`.

## The GitHub token in the MCP server

Cursor starts the aayudha MCP server with only `HOME`, `LOGNAME`, `PATH`, `SHELL`, `TERM`, `USER`, and `PWD` from your
environment. A token that reaches aayu through `AAYU_GITHUB_TOKEN`, `GH_TOKEN`, `GH_CONFIG_DIR`, or `XDG_CONFIG_HOME`
works in Cursor's terminal but not in the MCP server, which then says "no GitHub token". Keep the token where the server
finds it without them: `gh auth login` with gh's default config folder, or `aayu auth set-secret github-token`.
`aayu doctor`'s `mcp` row checks this when Cursor is installed. Writing `${env:GH_TOKEN}` into `.cursor/mcp.json` does
not help: Cursor passes an unset variable on as the literal text, and a changed value voids the approval above.

Cursor also reads `.claude/skills/` when "Include Third-Party Plugins, Skills, and Other Configs" is on (Settings →
Agents; on by default). So when you install for Claude Code as well, Cursor uses Claude Code's copies in
`.claude/skills/` and `install` writes no second set to `.agents/skills/`. Only Claude Code and Codex together need
copies in both folders; then Cursor lists each skill twice, and `aayu doctor` warns: turn the import off on that
machine, or install for one of the two. If you turn the import off with Claude Code and Cursor installed and Codex not,
Cursor sees no aayudha skills; run `aayu install --host codex --apply` to add `.agents/skills/`.

With Third-Party Imports on, Cursor also runs Claude Code's hooks from `.claude/settings.json` and from the Claude Code
plugin. Each aayudha hook answers once per event: a Claude Code hook that Cursor runs stands aside when Cursor has its
own aayudha hooks here (from `install` or the local plugin), and otherwise answers in Cursor's format. Cursor reads only
top-level files in `.claude/commands/` and `.cursor/commands/`, so it does not list the Claude Code commands in
`.claude/commands/aayudha/`; it gets its own as `.cursor/commands/aayudha-<name>.md`.

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
