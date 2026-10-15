# aayudha in Amp

Amp gets aayudha as a project plugin: palette commands for the main verbs, your journey at the start of every thread,
and the hooks. The skills come as copies in `.agents/skills/`, and the MCP server goes in your Amp settings.

## Install

```sh
aayu install --host amp --apply
```

| File | What it holds |
|---|---|
| `.agents/skills/<verb>/` | One skill per verb |
| `AGENTS.md` | The aayudha sections |
| `.amp/plugins/aayudha.ts` | The aayudha plugin |
| `~/.config/amp/settings.json` | The `aayudha` MCP server; install asks before it edits your home directory |

Reload plugins from the command palette (`plugins: reload`), or start a new thread. Check it:

```sh
amp plugins list
```

## What you get

Open the command palette and type `aayudha`:

| Command | What it does |
|---|---|
| Home: where the work stands | The home screen |
| Next step | The next step; for an agent step, it asks the agent to take it |
| Status of every Goal | `aayu status` |
| Doctor: check this machine and repo | `aayu doctor` |
| Shape a story, Check a story for Ready, Plan, Build, Verify, Inspect | Asks the agent in this thread to run that verb's skill |

At the start of a thread, Amp shows one line of where the work stands, such as
`aayudha | Ready 2 | next: aayu build 12`.

Steps that belong to a person (ship, release, accepting a Decision, responding to an incident) are never in the
palette. Run them in your terminal.

## Safe by default

The plugin calls aayu by the full path `install` checked, and on load it asks that binary for its version. If you
update aayu, the plugin pauses its commands and tells you to run `aayu install --host amp --apply` again.
