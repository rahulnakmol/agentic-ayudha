# Technical design: a build waits on a person without a reset

Status: planned, not built. No tech baseline exists. The planned design is the #882 body.

## Layers touched

| Layer | Owner seat | Paths (from #882 Expected paths) |
|---|---|---|
| L02 Interaction / API boundary | Code Architect | `internal/cli/build.go`, `internal/cli/buildnext.go`, `internal/cli/lease.go`, `internal/cli/ask.go` (from #880), `internal/cli/gc.go` |
| L04 Domain / policy | Quality Architect | `CONSTRAINTS.md` (generated) |
| L07 Operational data / state | Production Engineer | `internal/config/config.go`, `internal/config/defaults.toml`, `internal/config/gen/main.go` |

## What exists on dev (884d81b)

| Part | Where | Today |
|---|---|---|
| Build state | `internal/cli/build.go:99` `BuildState` | `started_at`, `failed_attempts`, `failures_by_layer`, `checkpoints`, `blocked`, `unblocked`, review spend, loop, claim. No `waiting` or `paused`. |
| The reset | `internal/cli/build.go:337-344` | When `st.Blocked != ""`, a new `BuildState` keeps `Unblocked` (with a time and the old reason), `Checkpoints`, `ReviewUSD`, `ReviewPR`, `Loop`, and `Claim`; `StartedAt` is set again to now. |
| Wall clock | `internal/cli/build.go:375-377`, breach at `:409-410` | `WallHours = now - StartedAt`; past `[autonomy.budget] wall_hours` the build blocks: "wall-time budget breached". |
| Block | `internal/cli/build.go:1193` `blockBuild`, `:1208` `markBlocked` | `status:blocked`, a page, a Kanha line; next step "`aayu ready <n>` lets the build continue with a fresh budget". |
| Usage-limit pause (delegated) | `internal/cli/build.go:540-550`; `delegate.Event.Until` (`internal/delegate/state.go:94`) | A receiver's usage-limit pause is recorded with its reset time; "a pause is no failed attempt". If the wait would pass the wall clock, the story blocks. |
| Lease | `internal/cli/lease.go:25-26` `LeaseTTL = 4 * time.Hour`; `:82` `leaseHolder`; `takeLease` | Every build run renews it; another holder is refused until it expires. |
| Stale sweep | `internal/cli/gc.go:43` `runGC`; `internal/cli/buildnext.go:367-372` `StaleGrace`, `:492-511` | `gc` reclaims expired leases; the loop's pick blocks a claim whose lease ran out more than `StaleGrace` ago with no PR. |
| Mission clock | `internal/cli/missiontime.go:30-37`, `:274` `MissionClock` | "It pauses while every running story waits on a person, a usage limit, or GitHub CI." An open ask "waits on #880" (`:458`). Per mission, not per build. From [2c6c6b0](https://github.com/rahulnakmol/agentic-ayudha/commit/2c6c6b04) (#1091, Story #1015). |
| Slot on answer | `internal/cli/kanha.go:194-197` `answeredAsk` | "the story keeps its slot (its status, lease, and claim stay; #882 Q2)". Not called yet. |

## Planned design (from #882)

1. `waiting` and `paused` in `build/<n>/state.json` (#882 Scope). Planned, not built. Inferred: the wall clock
   subtracts the paused time, as the mission clock subtracts waits.
2. The lease extends to "the provisional time plus 4 h" (#882 Scope). Planned, not built. Inferred: the provisional
   time is the ask's `provisional_after_hours` due time from Decision #862.
3. The stale sweep skips a waiting build (#882 Scope). Inferred: both `gc` and the loop's stale check.
4. No reset after a clarifying answer; "Change the story" keeps the reset (#882 Scope).
5. Config: the Expected paths list config and `CONSTRAINTS.md`; no key is named. Inferred: the 4 h margin may become
   a key. Unverified.

## Decisions

- [#862](https://github.com/rahulnakmol/agentic-ayudha/issues/862) (accepted): the ask SLA and provisional time
  (inferred link).
- "#882 Q2" in `internal/cli/kanha.go:195` names a question of #882 that the evidence here does not show.
  Unverified.
