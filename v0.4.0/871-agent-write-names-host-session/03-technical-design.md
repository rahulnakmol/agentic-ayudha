# Technical design: host and session on every agent write

Status: planned, not built. No tech baseline exists (`specs/871/` is absent on `origin/ledger`). The planned design
is the #871 body. The table "What exists on dev" is the code at 884d81b.

## Layers touched

| Layer | Owner seat | Paths (from #871 Expected paths) |
|---|---|---|
| L02 Interaction / API boundary | Code Architect | `internal/cli/asked.go`, `internal/cli/actions.go`, `internal/cli/build.go`, `internal/cli/env.go`, `internal/cli/install.go` |
| L03 Application / workflow | Engineering Manager | `internal/delegate/**` |
| L04 Domain / policy | Quality Architect | `CONSTRAINTS.md` (generated from `internal/config/defaults.toml`) |
| L06 Contracts / integration | Code Architect | `internal/hosts/hook.go`, `internal/hosts/install.go` |
| L07 Operational data / state | Production Engineer | `internal/actions/**`, `internal/config/config.go`, `internal/config/defaults.toml`, `internal/config/gen/main.go` |

Layer mapping from LAYERS.md globs.

## What exists on dev

| Part | Where (symbol) | What it records | Since |
|---|---|---|---|
| Session reader | `internal/cli/asked.go:37` `Env.session` | The first of `AMP_THREAD_ID`, `CODEX_THREAD_ID`, `CLAUDE_CODE_SESSION_ID`, `COPILOT_AGENT_SESSION_ID`. Used by the asked gate (`recordGrilled`, `grilledHere`) and baseline drafts. | before v0.3.0 ([86f4a85](https://github.com/rahulnakmol/agentic-ayudha/commit/86f4a85d), #280) |
| Host detection | `internal/doctor/checks.go:229` `DetectHost` | The `--host` flag, else the first agent marker in `hosts.AgentMarkers` (`internal/hosts/hook.go:275`), else `CURSOR_TRACE_ID`. | before v0.3.0 |
| Actions record host | `internal/actions/actions.go:38` `Action.Host`, `:88` `Options.Host` | The field exists. `Env.Actions()` sets `Options.Host` to `e.Host`, the `--host` flag (`internal/cli/actions.go:85`), so it is empty without the flag. No `session` field exists on `Action`. | — |
| Checkpoint commit trailers | `internal/cli/build.go:620-673` | `Aayudha-Host` (from `DetectHost`), `Aayudha-Model`, `Aayudha-Effort`, or `Aayudha-Delegate` for a delegated build. These are git trailers on commits, not ledger records. `internal/autonomy/autonomy.go:39` `HostTrailer` and `:184` `ampThreadLine` read them for the streak. | before v0.3.0 |
| Build state | `internal/cli/build.go:99` `BuildState`; `CheckpointRecord.Model` (`:135`) | The model of each checkpoint. No host, session, or URL. | — |
| Delegate state | `internal/delegate/state.go:58` `State` | `Receiver`, `Task`, `TaskURL` (the receiver's task), and `Kanha`, `KanhaURL` (the mission's Kanha session). No source of the run that wrote it. | `Kanha` fields: mission work under #889 |
| Mission parent | `internal/ready/approve.go:45` `Parent` | `Host`, `Session`, `URL`, `Mode`, `Machine` ("login@machine of a local Kanha"). Its comment says "It is information, never authority (#871)." | [9325551](https://github.com/rahulnakmol/agentic-ayudha/commit/93255510) (#1092, Story #1004) |
| Baseline draft | `internal/ready/baseline.go:51` `SourceSession` | "the host session that drafted it". | [af81cb4](https://github.com/rahulnakmol/agentic-ayudha/commit/af81cb48) (#1018, Story #990) |
| Lease holder | `internal/cli/lease.go:81` `leaseHolder` | `<login>@<machine>` in the lease comment on the issue. | WP5.5, before v0.3.0 |
| Host call log | `internal/integrations/hostcall/log.go:27-28` `Entry.Host`, `Entry.Session` | Host and session of a PostHog or Sentry call that the post-tool hook logs. | before v0.3.0 |

## Planned design (from #871 Scope)

1. `Env.source()`: one function for host, session, URL, and `login@machine` (#871 Scope). Planned, not built.
   Inferred: it can reuse `Env.session`, `doctor.DetectHost`, and `leaseHolder`, which hold each part today.
2. An optional `session` field on actions records "that keeps the hash chain valid" (#871 Scope). Planned, not built.
   Inferred: with `omitempty`, records written before the change hash the same, so old segments still verify.
3. The source in `build/<n>/state.json` and in delegate state (#871 Scope). Planned, not built.
4. Config: #871 Expected paths list `internal/config/config.go`, `internal/config/defaults.toml`,
   `internal/config/gen/main.go`, and `CONSTRAINTS.md`. The issue names no key. Unverified which key is meant.

## Data and ledger records

| Record | Change |
|---|---|
| `telemetry/actions.jsonl` (local) and the `actions` segments on the ledger | add optional `session`; fill `host` |
| `build/<n>/state.json` | add the source |
| `delegate/<n>/state.json` | add the source |

## Decisions

No Decision issue is linked from #871. The core design that names S2 is not linked as an issue; the evidence does not
show where it is kept.
