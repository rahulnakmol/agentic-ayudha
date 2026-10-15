# aayudha in OpenAI Codex

Codex, in the terminal and in the IDE extension, reads aayudha's skills, `AGENTS.md`, and MCP server. You can add
aayudha as a plugin, or install it into the repository.

## Before you start

Install aayu and check it is on your `PATH` (see [Your first hour](../start.md)):

```sh
aayu version
```

## Option 1: the plugin

Codex reads the same marketplace as Claude Code:

```sh
codex plugin marketplace add rahulnakmol/agentic-ayudha
codex plugin add aayudha@aayudha
```

You get one skill per verb (`$aayudha:build`, `$aayudha:ready`, …, also under `/skills`), the home and next commands,
and the `aayudha` MCP server. Check it:

```sh
codex plugin list
codex mcp list
```

## Option 2: install into the repository

```sh
aayu install --host codex --apply
```

This writes:

| File | What it holds |
|---|---|
| `.agents/skills/<verb>/` | One skill per verb, which Codex finds in every repository folder |
| `AGENTS.md` | The aayudha sections, which Codex reads as project guidance |
| `.codex/config.toml` | The `aayudha` MCP server, between `# BEGIN aayudha` and `# END aayudha`; the rest of the file is yours |
| `.codex/hooks.json` | The `session-start` and `pre-tool` hooks |

Codex loads a project's `.codex/config.toml` only when you trust the project, and it runs new hooks only after you
review them: open `/hooks` in Codex once and trust aayudha's. Then check:

```sh
codex mcp list
aayu doctor
```

Use the plugin or `install`, not both, or every skill appears twice.

## Human-only steps

Shipping, releasing, accepting a Decision, and responding to an incident are for a person. The MCP server never offers
them, and the `pre-tool` hook stops Codex if it tries to run them in a shell, with the exact command for you to run.

## Models

On Codex, aayudha's build, spec, and plan lanes default to GPT-6.1 Sol, and its review lane to Claude Opus 5.5, so the
reviewer is never the same family as the builder. `aayu models show --host codex` lists them all.
