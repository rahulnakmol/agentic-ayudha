# aayudha in your agent

aayudha works inside the agent you already use. Pick yours:

| Agent | How aayudha arrives |
|---|---|
| [Claude Code](claude-code.md) | A plugin from this repository's marketplace, or `aayu install --host claude` |
| [OpenAI Codex](codex.md) | The same plugin, or `aayu install --host codex` |
| [GitHub Copilot](copilot.md) | `aayu install --host copilot`: skills, hooks, and the MCP server for Copilot CLI and VS Code |
| [Amp](amp.md) | `aayu install --host amp`: skills, a project plugin, and the MCP server |
| [Cursor](cursor.md) | `aayu install --host cursor`: skills, rules, MCP, and hooks |

Every host gets the same rules: hooks call only `aayu hook <name>`, steps that belong to a person are never offered
to an agent, and write verbs reach the MCP server only when you set `[mcp] expose_writes = true`.

## The config your agent runs

Your agent runs some files straight from the checkout: hooks and MCP servers in `.mcp.json`, `.claude/`,
`.cursor/`, `.codex/`, `.github/hooks/`, and the Amp plugin in `.amp/plugins/`. They stay in the repository,
because that is where each agent looks for them, so aayudha guards them two ways:

- `aayu init --gates` adds an `aayudha:host-config` block to `.github/CODEOWNERS`. A pull request that changes
  any of those files then needs a code owner's review before it merges.
- `aayu doctor` fails its `host-config` row when an installed hook or MCP entry differs from what
  `aayu install` would write, and names the file. `aayu install --host <host> --apply` puts it back.

One risk remains. Review happens when a change merges, not when you check it out. If you check out someone
else's branch in a folder your agent trusts, the agent runs that branch's hooks and MCP servers, with your
environment and your tokens, before anyone has reviewed them. Before you start your agent on a branch you did not
write, run `aayu doctor` or read the branch's changes to those files.

## The cloud environment each agent works in

A hand-off to an agent's cloud needs an environment there that can build and test your repository.
`aayu install --env` works it out from the code: toolchains and their versions, the package manager, services
such as PostgreSQL, browsers, system packages, and the names (never the values) of the variables and secrets.
It asks only what the code cannot say, each question with a default and its reason, and reads the result back.

```sh
aayu install --env              # show the spec, every host's files, and the values for host UIs
aayu install --env --verify     # write them, then prove each host in a clean container
aayu install --env --host amp --cloud     # prove the live cloud environment and record it
aayu doctor                     # each host: verified, stale, or missing
```

One script, `.agents/setup`, installs everything with pinned versions and checksums, and every host calls it:
Amp runs it natively, Cursor through `.cursor/environment.json`, Claude Code on the web through a
`SessionStart` hook in `.claude/settings.json`, and Codex through the Install script you paste into its
settings. `.agents/check` proves an environment by running your build and tests. Where a host is configured
only in its web UI (Codex, Claude Code), `install --env` prints the exact values and steps, and
`--record '<reply>'` records the reply to the probe task it gives you.

`aayu doctor`'s `agents` row lists which agent CLIs are installed and signed in, and which clouds can take a
hand-off.

Then [make it yours](make-it-yours.md): themes, plain mode, and shell completions.
