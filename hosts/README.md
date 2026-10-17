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

`aayu init --verify-skill` writes a product's own verify skill (WP4.5b.3) from [`verify-skill/`](verify-skill/):
`SKILL.md`, a feature map with no features yet, and a control tool in the stack the checkout already has (`go.mod`:
[`go/control/main.go`](verify-skill/go/control/main.go); `package.json`: [`node/control.mjs`](verify-skill/node/control.mjs);
neither: a [stub](verify-skill/other/control.sh) to replace). The examples' control tools are these templates
(`TestInitVerifySkill`). `aayu install` gives Claude Code a pointer to each product `.agents/skills/verify-*/` in
`.claude/commands/aayudha/` and records it under `[product_skills]` in `.aayudha/lock.toml`.

Every hook runs `"<absolute path to aayu>" hook <name> --host <host>` and nothing else. Copies are real files, never
symlinks (V2), recorded with their hashes in `.aayudha/lock.toml`.

## The plugins (Claude Code, Codex, Cursor) and the Amp plugin

The repository root is also a plugin marketplace: [`.claude-plugin/marketplace.json`](../.claude-plugin/marketplace.json)
(Claude Code, and Codex, which reads it too) and [`.cursor-plugin/marketplace.json`](../.cursor-plugin/marketplace.json)
list one plugin, `aayudha`, whose root is the repository itself, so its skills are [`skills/`](../skills/) with no
copy. Each host reads its own manifest first: [`.claude-plugin/plugin.json`](../.claude-plugin/plugin.json),
[`.codex-plugin/plugin.json`](../.codex-plugin/plugin.json), and [`.cursor-plugin/plugin.json`](../.cursor-plugin/plugin.json).
Each adds the slash commands, the host's hooks ([`claude/`](claude/hooks.json), [`codex/`](codex/hooks.json),
[`cursor/`](cursor/hooks.json)), and the MCP server. The Amp plugin, [`amp/aayudha.ts`](amp/aayudha.ts), gets the same
commands from `aayu install --host amp`.

The commands are host-neutral sources in Claude Code's format in [`commands/`](commands/): `common/` holds `home` and
`next`; `<agent>/` holds the hand-off to that agent (`aayu build --delegate`). Every host gets `common/` and every
agent but itself, so no host offers a hand-off to itself. The host appends what the user typed after the command, so
no source uses `$ARGUMENTS` (Codex drops a command that does). How a user calls them:

| Host | Install | Hand-offs offered | How to call one |
|---|---|---|---|
| Claude Code | `/plugin marketplace add rahulnakmol/agentic-ayudha`, then `/plugin install aayudha@aayudha` | Codex, Cursor, Amp | `/aayudha:codex <story or goal>` |
| Codex | `codex plugin marketplace add rahulnakmol/agentic-ayudha`, then `codex plugin add aayudha@aayudha` | Claude Code, Cursor, Amp | `$aayudha:source-command-cursor <story or goal>`, or pick it from `/skills` |
| Cursor | `agent plugin marketplace add https://github.com/rahulnakmol/agentic-ayudha` (CLI), or the plugin in `~/.cursor/plugins/local/aayudha` (IDE) | Codex, Claude Code, Amp | `/codex <story or goal>` (Cursor may show it as `/aayudha:codex`) |
| Amp | `aayu install --host amp --apply` | Codex, Cursor, Claude Code | the command palette: `aayudha: Hand a Ready story …` |

A marketplace plugin cannot know where `aayu` is on each machine, so it runs `aayu` from `PATH` and passes the skills
version it carries (`--skills`): the MCP server refuses to start on a mismatch and `session-start` reports one. Use
What `aayu install` commits never depends on one machine's plugin setting: it always writes the skill copies, hooks,
and MCP server, so a teammate without the plugin gets everything. Where a host lets a repository turn a plugin off, it
does: `enabledPlugins` `false` in `.claude/settings.json` (Claude Code; project settings beat user settings) and
`[plugins."aayudha@aayudha"] enabled = false` in `.codex/config.toml` (Codex, in a trusted project). Cursor has no such
switch, so its plugin and `install` together list each skill twice. Skill copies go to one folder per reader:
`.claude/skills/` for Claude Code, `.agents/skills/` for Codex, and Cursor and Amp share whichever exists (Cursor reads
both through Third-Party Imports); only Claude Code plus Codex plus Cursor leaves Cursor two copies. The product's
`verify-*` skills stay in `.agents/skills/`, and Claude Code reaches each through a pointer command in
`.claude/commands/aayudha/`, so Cursor lists each once. Because the plugin is off there, `install` writes the plugin's
commands itself: Claude Code's in `.claude/commands/aayudha/` (the same `/aayudha:<name>`), and Codex's as skills in
`.agents/skills/aayudha-source-command-<name>/` (the same `aayudha:source-command-<name>`). A Claude Code hook that
Cursor runs (Third-Party Imports) stands aside where Cursor has its own aayudha hooks, so each event is checked once. A plugin hook (it passes `--skills`) stands aside where the
host runs the repository's hooks. `aayu doctor`'s `skills-copies` row reports a host that lists the skills twice. Amp
keeps the first skill of each name, so it never lists one twice.

Codex runs a plugin's or a project's hooks only after the person trusts them in `/hooks`, and again after any change
to them (a plugin update changes `--skills`). Until then nothing refuses a human-only verb in Codex. `aayu doctor`'s
`codex-hooks` row compares the trust Codex records in `config.toml` (`[hooks.state]`) with each aayudha hook.
