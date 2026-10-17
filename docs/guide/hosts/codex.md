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
| `.agents/skills/aayudha-source-command-<name>/` | The plugin's commands, as Codex names them: `$aayudha:source-command-next`, `$aayudha:source-command-home`, and the hand-offs |
| `AGENTS.md` | The aayudha sections, which Codex reads as project guidance |
| `.codex/config.toml` | The `aayudha` MCP server, between `# BEGIN aayudha` and `# END aayudha`; the rest of the file is yours |
| `.codex/hooks.json` | The `session-start` and `pre-tool` hooks |

Codex loads a project's `.codex/` files only when you trust the project, and it runs new hooks only after you
review them: open `/hooks` in Codex once and trust aayudha's. Until you trust the project, `aayu doctor` shows that step
on its `mcp` and `codex-hooks` rows. Then check:

```sh
codex mcp list
aayu doctor
```

What `install` writes is the same whether or not you have the plugin, so a teammate who clones the repository without
the plugin still gets every skill. If you also have the plugin, `install` turns it off for this repository: the
aayudha block in `.codex/config.toml` sets `[plugins."aayudha@aayudha"] enabled = false`, so Codex lists each skill and
the `aayudha` server once and runs each hook once. Codex reads that file only in a trusted project, but it reads
`.agents/skills/` either way, so until you trust the project it lists each skill twice (the plugin's and the
repository's), and the plugin's hooks do not stand aside; `aayu doctor` reports the double until you trust it.

Cursor and Amp read the same `.agents/skills/` copies, so installing for them as well adds no second copy.

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

Codex starts the `aayudha` server with only a few of your variables, such as `HOME` and `PATH`. The plugin and the
`install` block also pass it `AAYU_GITHUB_TOKEN`, `GH_TOKEN`, `GH_CONFIG_DIR`, and `XDG_CONFIG_HOME`, the places aayu
looks for your GitHub token, so the server finds the token your shell finds. You should see the `status` tool answer
with your repository's state, not "no GitHub token found".

## Codex through your own model provider

A Codex that answers through a custom model provider (a `[model_providers.<id>]` entry, perhaps behind a wrapper that
sets `CODEX_HOME`) can say "Not logged in" to `codex login status` and still run. So when Codex is not signed in, aayu
asks `codex doctor --json`, which reads the configuration the way `codex exec` does. Codex counts as ready to take a
local hand-off (`aayu build <n> --delegate codex --local`) when the active provider needs no OpenAI sign-in and has its
credentials: an `auth` command in its config, or `CODEX_API_KEY`. A provider whose key comes from another environment
variable (`env_key`) does not count, because aayu never passes a model provider's key to a local receiver; give the
provider an `auth` command instead. A `CODEX_HOME` set where you run aayu reaches the receiver, so it runs the same
Codex home that aayu checked. `aayu doctor` then shows "codex: … runs through model provider `<id>`". `codex
doctor` also checks that the provider's endpoint answers, so this one probe does reach the network.

## Models

On Codex, aayudha's build, spec, and plan lanes default to GPT-6.1 Sol, and its review lane to Claude Opus 5.5, so the
reviewer is never the same family as the builder. `aayu models show --host codex` lists them all.
