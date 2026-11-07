# Sequencing: Approve to build learns time and cost

The slices are proposed, not filed (see [05-implementation-plan.md](05-implementation-plan.md)).

## Before any lane starts

1. #872 merges its record and the ship append (#872 P1, P2), because #886 depends on #872 (#886 Links).
2. A person approves #886's baselines and runs Approve to build (`aayu ready 886`).

## Waves

The hooks split the work by file: time in `missiontime.go`, cost in `mission.go`, the hold in `missionnext.go`. So
three lanes run at once in wave 1. P4 waits for the numbers.

### Wave 1

| Lane | Slice | Files |
|---|---|---|
| 1 | P1 working hours | `internal/delivery/hours.go` (new, name inferred), `internal/cli/missiontime.go`, `internal/config/**`, `CONSTRAINTS.md` |
| 2 | P2 cost estimate | `internal/delivery/cost.go` (new, name inferred), `internal/cli/mission.go` (`costEstimate` only) |
| 3 | P3 budget hold on measured spend | `internal/cli/missionnext.go`, `internal/cli/missiongate.go`, `internal/cli/missionask.go`, `internal/cli/mission.go` (`missionUsed` only) |

P2 and P3 both edit `internal/cli/mission.go`: P2 `costEstimate` (`:752-759`), P3 `missionUsed` (`:1398-1406`).
Merge P2 first, and P3 rebases on it.

### Wave 2 (after P1 and P2)

| Lane | Slice | Files |
|---|---|---|
| 1 | P4 estimate in slice, grill, and the guide | `internal/planning/**`, `internal/grill/**`, `skills/slice/**`, `docs/guide/**` |

## Hot files

- Mission files: PR [#1275](https://github.com/rahulnakmol/agentic-ayudha/issues/1275) edits
  `internal/cli/missionask.go` and `internal/cli/missiongate.go`, and PR
  [#1276](https://github.com/rahulnakmol/agentic-ayudha/issues/1276) edits `internal/cli/mission.go`
  (`gh pr view --json files`, read on 2026-11-07). Both were closed unmerged on 2026-11-07 when the mission ended;
  branch `fix/gate-count-since-pass` at 9e8b72b and branch `fix/approve-keeps-gate` at bcafd0f are kept
  (`_data/closed-prs.tsv`). If either is reopened and merges first, P2 and P3 rebase on it.
- `internal/config/defaults.toml` and `CONSTRAINTS.md` (generated together): shared with #871, #872, #880, #881, and
  #882.

## Person steps and protected paths

- P1's config key is under `internal/config/` (`[autonomy] always_person`, `.aayudha/config.toml:83`) and in
  `.github/CODEOWNERS` (`/internal/config/defaults.toml`, `/CONSTRAINTS.md`). A key in `[budget]`, `[build]`, or
  `[mission]` is also in `always_person_sections` (`internal/config/defaults.toml:70`). It needs a person to ship.
- P4's `skills/slice/**` is under `/skills/` in CODEOWNERS.
- P3 changes when a mission stops on budget. It touches no listed protected path. Inferred: a person should review the
  rule change, since it moves money limits.
