# hosts

Per-host templates that `aayu install` (WP3.2) writes into a product repository. Embedded by the root `embed.go`.
Most host files are JSON that `install` merges into what is already there (`internal/hosts`); only files that are
whole templates live here.

| Host | Skill copies | MCP registration | Hooks | Other |
|---|---|---|---|---|
| Amp | `.agents/skills/<verb>/` | `amp.mcpServers` in `~/.config/amp/settings.json` (a home edit, confirmed separately) | `.amp/plugins/aayudha.ts`, from [`amp/aayudha.ts`](amp/aayudha.ts) | `AGENTS.md` sections |
| Cursor | `.agents/skills/<verb>/` | `.cursor/mcp.json` | `.cursor/hooks.json` | `AGENTS.md` sections |
| Claude Code | `.claude/skills/<verb>/` | `.mcp.json`, enabled in `.claude/settings.json` | `.claude/settings.json` | `CLAUDE.md` stub importing `@AGENTS.md` |

Every hook runs `"<absolute path to aayu>" hook <name> --host <host>` and nothing else. Copies are real files, never
symlinks (V2), recorded with their hashes in `.aayudha/lock.toml`.
