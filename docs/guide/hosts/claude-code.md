# aayudha in Claude Code

Claude Code gets aayudha as a plugin: every verb as a slash command and skill, `/aayudha:home` and `/aayudha:next` for
where the work stands, the aayudha MCP server, and the hooks that keep agents inside the rules.

## Before you start

Install aayu and check it is on your `PATH` (see [Your first hour](../start.md)):

```sh
aayu version
```

The plugin runs `aayu` from your `PATH`. It passes the skills version it carries, and aayu refuses to serve a plugin
whose skills differ from its own, so the two never drift apart quietly.

## Install the plugin

In Claude Code:

```text
/plugin marketplace add rahulnakmol/agentic-ayudha
/plugin install aayudha@aayudha
```

Or from your shell:

```sh
claude plugin marketplace add rahulnakmol/agentic-ayudha
claude plugin install aayudha@aayudha
```

Run `/reload-plugins`, or start a new session.

## What you get

| In Claude Code | What it does |
|---|---|
| `/aayudha:home` | Where this repository stands, and the next step |
| `/aayudha:next` | The single best next step; Claude takes it when it may |
| `/aayudha:grill`, `/aayudha:ready`, `/aayudha:build`, … | One skill per verb, from the same `skills/` the binary carries |
| The `aayudha` MCP server | The verbs as tools; write verbs only with `[mcp] expose_writes = true` |
| Hooks | `session-start` reports skill drift; `pre-tool` pauses agents when the budget breaker trips or the repository is halted, and refuses PostHog or Sentry writes through another MCP server or CLI; `post-tool` logs the PostHog and Sentry reads it lets through |

Shipping, releasing, accepting a Decision, and responding to an incident are for a person. Their skills only run when
you type them, the MCP server never offers them, and the `pre-tool` hook blocks Claude if it tries to run them in a
shell. When the next step is yours, Claude shows you the exact command to run in your own terminal.

## A status line with your journey

A plugin cannot set Claude Code's status line, so add one line to `.claude/settings.json` (or your own
`~/.claude/settings.json`):

```json
{
  "statusLine": { "type": "command", "command": "aayu hook status-line --host claude" }
}
```

It reads the last home screen, so it never waits on GitHub:

```text
aayudha | Ready 2 | waiting for you 1 | next: aayu ship 4
```

Run `aayu` now and then to refresh it.

## Or install it into the repository

If you would rather commit the setup with the repository, use `aayu install` instead of the plugin:

```sh
aayu install --host claude --apply
```

It writes skill copies to `.claude/skills/`, the plugin's commands to `.claude/commands/aayudha/` (the same
`/aayudha:home`, `/aayudha:next`, and hand-offs), the MCP server to `.mcp.json`, and the hooks and the status line to
`.claude/settings.json`, each calling aayu by its full, version-checked path. Your product's verify skill stays in
`.agents/skills/verify-<product>/`; Claude Code reaches it as `/aayudha:verify-<product>`, a short command that points
there, so Cursor, which reads `.claude/skills/` too, lists it once. Each teammate runs it once on their
machine. What `install` writes is the same whether or not you have the plugin, so a teammate who clones the repository
without the plugin still gets every skill.

If you also have the plugin, `install` turns it off for this repository: `.claude/settings.json` gets
`"enabledPlugins": {"aayudha@aayudha": false}`, which Claude Code applies over your user setting. Claude Code then lists
each skill, command, and the `aayudha` server once, runs each hook once, and keeps the plugin on in your other
repositories. The commands keep their names, because `install` writes them itself. An `enabledPlugins`
entry in `.claude/settings.local.json` turns the plugin back on: its hooks stand aside, and `aayu doctor` reports the
doubled skills.

## Check it worked

```sh
claude plugin details aayudha
aayu doctor
```
