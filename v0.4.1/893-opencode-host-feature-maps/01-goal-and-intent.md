# Goal and intent: #893 OpenCode as a host

No intent baseline exists (no `specs/893/` on `origin/ledger`, 2026-11-07). The source is
[#893](https://github.com/rahulnakmol/agentic-ayudha/issues/893), checked against `dev` at
[884d81b](https://github.com/rahulnakmol/agentic-ayudha/commit/884d81b).

## Goal

"aayu installs into OpenCode: MCP, skills, the agent marker, loops, and the [verify] maintainer value. This is step S24
of the core design (aayu core: the human-dependency engine, agreed by @rahulnakmol on 2026-10-21)." (#893, Summary.)
The core design is Initiative [#870](https://github.com/rahulnakmol/agentic-ayudha/issues/870).

## Who it serves

Teams that use OpenCode as their agent tool. With this Feature, OpenCode can "maintain the feature maps" (#893 title
and Problem), the job that `[verify] maintainer` hands to an agent app (`internal/verify/maintain.go`, `Maintainers`).

## Problem, checked against the code

"OpenCode reads .agents/skills and supports MCP, but aayu has no OpenCode host, so OpenCode cannot maintain the
feature maps." (#893, Problem.) What OpenCode reads is OpenCode's behaviour; it is not checked here. Unverified.

| Part | On 884d81b |
|---|---|
| Host ID | Absent. `verbs.Hosts` is Amp, Cursor, Claude Code, OpenAI Codex, GitHub Copilot (`internal/verbs/registry.go:391-397`). The global `--host` flag lists `amp, cursor, claude, codex, copilot` (`aayu --help`). |
| Host files | Absent: no `internal/hosts/opencode.go`, no `hosts/opencode/`, no `hosts/commands/opencode/`, no `docs/guide/hosts/opencode.md`. |
| Agent marker | Absent. `hosts.AgentMarkers` names Amp, Claude Code, Cursor, Codex, Copilot, and two cross-host conventions (`internal/hosts/hook.go:275-289`). |
| Loops | Absent. `loops.Hosts` is `amp, cursor, claude, codex, actions` (`internal/loops/loops.go:48`). |
| `[verify] maintainer` | Absent. The enum is `amp, claude, codex, cursor` (`internal/config/config.go:417`); `verify.Maintainers` has the same four. |
| Person's tool per repository | Not allowed. `[hosts] repos` accepts only `config.RepoTools()`, which lists the registry hosts (`internal/config/config.go:78-84`, `:636-643`). |
| Detection | Present. `agentenv.Detect` finds the `opencode` CLI, with install `npm install -g opencode-ai` and sign-in `opencode auth login` (`internal/agentenv/detect.go:237-248`). |
| Delegate receiver | Present. `delegate.OpenCode` is in `delegate.Receivers`; `--delegate opencode` always runs locally (`internal/cli/delegate.go:334-335`; `LocalArgs`: `opencode run <prompt>`). |
| Local Kanha | Present. `delegate.LocalKanha` includes OpenCode (`KanhaStart`: `run --format json`; `KanhaResume`: `run --session <id>`). |

The last three rows came in [80af4e1a](https://github.com/rahulnakmol/agentic-ayudha/commit/80af4e1a) (#1125, and
Story #1012 under #889). The code comment on `delegate.OpenCode` calls it "#893 story F" (`internal/delegate/state.go:20-22`).
No Story F of #893 is filed.

## Success measures

From #893 Scope: the host ID; `internal/hosts/opencode.go`; the `opencode.json` MCP entry for each config version; the
agent marker; loop prompts; the maintainer value. The issue states no numeric measure.

## Out of scope

"A delegate receiver for OpenCode." (#893, Scope Out.) It exists on `dev` anyway, from #1012 (see the table).
