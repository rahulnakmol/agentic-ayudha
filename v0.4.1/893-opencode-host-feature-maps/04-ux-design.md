# UX design: #893 OpenCode as a host

The person's surface is `aayu install --host opencode`, `aayu doctor`, `aayu init`, and config. The agent's surface is
the aayudha MCP server and skills inside OpenCode. The #893 parts are not built.

## Today (delivered building blocks)

On [884d81b](https://github.com/rahulnakmol/agentic-ayudha/commit/884d81b):

- The global flag: `--host string   host running aayudha: amp, cursor, claude, codex, copilot (default: detected)`
  (`aayu --help`). OpenCode is not a value.
- `aayu build --help`: "`--delegate` … OpenCode has no cloud agent, so it always runs locally. One of: codex, cursor,
  claude, amp, opencode, auto." So a person can already hand a story to OpenCode's CLI on their machine.
- Detection reports OpenCode's cloud state as "OpenCode has no hosted cloud agent; its stories run as local sessions
  (`--local`)" (`internal/agentenv/detect.go:242`).
- `[hosts] repos = ["owner/name=opencode"]` is refused: "want owner/name=tool, with tool one of …" lists only the
  registry hosts (`internal/config/config.go:643`).

## Planned (planned, not built)

| Moment | The person types | What they see (from #893) |
|---|---|---|
| Install | `aayu install --host opencode` | A diff of the skill copies and the `opencode.json` MCP entry; `--apply` writes them. |
| Check | `aayu doctor` | Checks for the OpenCode files. The text is not stated. |
| Start | `aayu init` | OpenCode is offered as a host. |
| Loops | `aayu install --loops --host opencode` | Loop prompts for OpenCode. |
| Feature maps | `[verify] maintainer = "opencode"` | The maintain brief goes to OpenCode. The exact value is not stated. |
| Inside OpenCode | The agent calls aayu | Human-only verbs are refused, as in other hosts, once the marker is known. |

The guide `docs/guide/hosts/opencode.md` explains these steps (#893 Expected paths). It does not exist yet.

## Agent-facing surface

- The aayudha MCP tools, prompts, and resources, as on other hosts (`internal/mcp/server.go`).
- The skills that OpenCode reads (the issue says `.agents/skills`; unverified).
- Hand-off commands under `hosts/commands/opencode/**` (an Expected path). `hosts/README.md` says each host gets
  `common/` and every agent but itself.

## Accessibility

Terminal output only. The `--plain` flag applies (`aayu --help`).
