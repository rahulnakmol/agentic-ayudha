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
the hooks, and the `aayudha` MCP server. Check it:

```sh
codex plugin list
codex mcp list
```

### Trust the hooks

Codex does not run a plugin's hooks until you trust them. Start Codex, open `/hooks`, and trust the two aayudha hooks:
`SessionStart` and `PreToolUse`. Until you do, nothing stops an agent in Codex from running `aayu ship` or
`aayu release` itself. Codex asks again after each plugin update, because the hooks name the plugin's version.

`aayu doctor` tells you when Codex would skip one:

```text
FAIL  codex-hooks  Codex skips aayudha hooks it does not trust, so no guard rail stops an agent from running a human-only verb there: open /hooks in Codex and trust them
                     plugin aayudha@aayudha: the PreToolUse hook is not trusted yet; trust it in Codex's /hooks
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

Codex loads a project's `.codex/` files only when you trust the project, and it runs new hooks only after you
review them: open `/hooks` in Codex once and trust aayudha's. Then check:

```sh
codex mcp list
aayu doctor
```

Use the plugin or `install`, not both. If the plugin is enabled when you run `install`, install writes no skill copies,
hooks, or MCP server of its own (and removes the ones it wrote before), so Codex lists each skill once and runs each
hook once. If you add the plugin later, the plugin's hooks stand aside and tell you to run
`aayu install --host codex --apply` again; until you do, `aayu doctor` reports the doubled skills.

Amp and Cursor read `.agents/skills/` too. If you also install for one of them, the copies stay for it and Codex lists
each skill twice; `aayu doctor` says so. Use the plugin for Cursor as well, or `install` for Codex.

## How Codex takes a request

Codex does not reach for the grill skill on its own, so aayudha tells it how. The `AGENTS.md` section that `install`
writes, and the `session-start` hook, say: before changing anything, run
`aayu grill story --assess --idea "<your words>" --json`; when the verdict is `unpack`, ask you its questions and wait
for your answers; and work only on the issue you asked for, never an open or Ready story it picked itself. You can
still say "Using aayudha: …" to be explicit.

## Human-only steps

Shipping, releasing, accepting a Decision, and responding to an incident are for a person. The MCP server never offers
them, and the `pre-tool` hook stops Codex if it tries to run them in a shell, with the exact command for you to run.

## The MCP tools

Codex offers the model the aayudha tools under `mcp__aayudha` (`status`, `doctor`, `grill`, …). It sends them to the
model as one Responses API `namespace` tool. If you route Codex through your own AI gateway, the gateway must pass
`namespace` tools through. One that drops them leaves the model with Codex's own resource tools only: it can read
aayudha's resources but cannot call a verb. The skills still work then, because they run `aayu` in the shell.

## Models

On Codex, aayudha's build, spec, and plan lanes default to GPT-6.1 Sol, and its review lane to Claude Opus 5.5, so the
reviewer is never the same family as the builder. `aayu models show --host codex` lists them all.
