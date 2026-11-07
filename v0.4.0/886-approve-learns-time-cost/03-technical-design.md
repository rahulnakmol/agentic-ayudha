# Technical design: Approve to build learns time and cost

Status: planned, not built. No tech baseline exists. The planned design is the #886 body and the hooks on `dev`
that name #886.

## Layers touched

| Layer | Owner seat | Paths (from #886 Expected paths, plus the hooks) |
|---|---|---|
| L02 Interaction / API boundary | Code Architect | `internal/cli/mission.go`, `internal/cli/missiontime.go`, `internal/cli/missionnext.go`, `internal/cli/env.go` (hooks; not in Expected paths) |
| L03 Application / workflow | Engineering Manager | `internal/workitems/**`, `internal/initrepo/**`, `internal/ready/**`, `internal/planning/**`, `internal/grill/**` |
| L06 Contracts / integration | Code Architect | `internal/github/**` |
| L07 Operational data / state | Production Engineer | `internal/config/**` |
| none today | — | `internal/delivery/**` (created by #872); `docs/guide/**`; `skills/slice/**` |

## What exists on dev (884d81b)

| Part | Where | Today | Since |
|---|---|---|---|
| Cost hook | `internal/cli/env.go:107-110` `Options.CostEstimate`; `internal/cli/mission.go:752-759` `Env.costEstimate` | Returns the test fake or nil. "nil until #886 merges (tests set Options.CostEstimate)". | [d941d15](https://github.com/rahulnakmol/agentic-ayudha/commit/d941d151) (#1084, Story #997) |
| Cost type | `internal/cli/mission.go:309` `CostEstimate` | `usd`, `low_usd`, `high_usd`, `source` ("#886"). | same |
| Budget basis | `internal/cli/mission.go:893-897` | `the approved budget, N x <cap> ([budget] per_story_usd): no cost estimate from #886 yet`; a non-nil estimate with `HighUSD > 0` changes it. | same |
| Time hook | `internal/cli/env.go:112-113` `Options.WorkingHours`; `internal/cli/missiontime.go:49-56` `Env.workingHours` | Returns the test fake or nil: "with none, the plan has no estimate in hours, and nothing warns or refuses at Start". | [2c6c6b0](https://github.com/rahulnakmol/agentic-ayudha/commit/2c6c6b04) (#1091, Story #1015) |
| Time type | `internal/cli/missiontime.go:39` `WorkingHours`, `:58` `MissionHours` | Learned and default hours per effort size; the mission estimate sums hours "along the longest chain of the mission's story graph". | same |
| Per-story effort | `internal/cli/mission.go:163` | Each mission story keeps its `effort:<size>` label, "which #886's working hours are per". | [2c6c6b0](https://github.com/rahulnakmol/agentic-ayudha/commit/2c6c6b04) (#1091) |
| Budget hold on the cap | `internal/cli/missionnext.go:369` (`:359` at 884d81b); `internal/cli/mission.go:1398-1406` `missionUsed`; `internal/cli/missiongate.go:1345` | A story counts at its per-story cap; "until #872 measures spend". | [2114e00](https://github.com/rahulnakmol/agentic-ayudha/commit/2114e008) (#1087), [3ece84e](https://github.com/rahulnakmol/agentic-ayudha/commit/3ece84e7) (#1090) |
| Measured spend per story | `internal/cli/build.go:1142` `buildCost`; `internal/cli/status.go:97` `costs` | From the local actions log. Gate fix PRs already count "at its measured spend" when closed (`missionUsed` comment). | before v0.3.0 |

## Planned design (from #886)

1. Time and cost estimates learned from `delivery/stories.jsonl` (#886 Scope). Planned, not built. Inferred: they
   plug into the two hooks above, so the mission code needs no other change.
2. The budget hold on measured spend (#886 Scope). Planned, not built. Inferred: it replaces the per-story cap in
   `missionnext.go:369` and `missionUsed` with each story's measured spend from `delivery/stories.jsonl`.
3. The default working hours "under the key #886 names" (`internal/cli/missiontime.go:41-46`). The key is not named
   in any source read here. The comment at `missiontime.go:42` mentions `[delivery.default_hours]`, which is not in
   `internal/config/defaults.toml` (`rg '^\[delivery' internal/config/defaults.toml` is empty). Unverified.
4. "#886 S8" in `missiontime.go:40` names a step of #886 that the #886 body does not list. Unverified.

## Decisions

- [#868](https://github.com/rahulnakmol/agentic-ayudha/issues/868): cancelled; cited by #886.
- [#918](https://github.com/rahulnakmol/agentic-ayudha/issues/918) (accepted 2026-10-23): supersedes #868; "#886
  drops S5, S6 and S7"; the init budget question belongs to #892.
