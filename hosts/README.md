# hosts

Per-host templates that `aayu install` (WP3.2) writes into a product repository. Embedded by the root `embed.go`.
Most host files are JSON that `install` merges into what is already there (`internal/hosts`); only files that are
whole templates live here.

| Host | Skill copies | MCP registration | Hooks | Other |
|---|---|---|---|---|
| Amp | `.agents/skills/<verb>/` | `amp.mcpServers` in `~/.config/amp/settings.json` (a home edit, confirmed separately) | `.amp/plugins/aayudha.ts`, from [`amp/aayudha.ts`](amp/aayudha.ts) | `AGENTS.md` sections |
| Cursor | `.agents/skills/<verb>/` | `.cursor/mcp.json` | `.cursor/hooks.json` | `AGENTS.md` sections |
| Claude Code | `.claude/skills/<verb>/` | `.mcp.json`, enabled in `.claude/settings.json` | `.claude/settings.json` (and its `statusLine`) | `CLAUDE.md` stub importing `@AGENTS.md` |
| OpenAI Codex | `.agents/skills/<verb>/` | `[mcp_servers.aayudha]` in a marked block of `.codex/config.toml` (trusted projects only) | `.codex/hooks.json` | `AGENTS.md` sections |

Every hook runs `"<absolute path to aayu>" hook <name> --host <host>` and nothing else. Copies are real files, never
symlinks (V2), recorded with their hashes in `.aayudha/lock.toml`.

## The marketplace plugin (Claude Code and Codex)

The repository root is also a plugin marketplace: [`.claude-plugin/marketplace.json`](../.claude-plugin/marketplace.json)
lists one plugin, `aayudha`, whose root is the repository itself, so its skills are [`skills/`](../skills/) with no
copy. Its manifest, [`.claude-plugin/plugin.json`](../.claude-plugin/plugin.json), adds the slash commands in
[`commands/`](commands/) (host-neutral command sources, Claude Code format), the hooks in
[`claude/hooks.json`](claude/hooks.json), and the MCP server. Codex reads the same marketplace and plugin.

A marketplace plugin cannot know where `aayu` is on each machine, so it runs `aayu` from `PATH` and passes the skills
version it carries (`--skills`): the MCP server refuses to start on a mismatch and `session-start` reports one. Use
either the plugin or `aayu install` for a host in a repository, not both, or the skills appear twice.
