# Technical design: one benchmark line per merged story

Status: planned, not built. No tech baseline exists. The planned design is the #872 body.

## Layers touched

| Layer | Owner seat | Paths (from #872 Expected paths) |
|---|---|---|
| L02 Interaction / API boundary | Code Architect | `internal/cli/ship.go`, `internal/cli/release.go`, `internal/cli/build.go`, `internal/cli/audit.go`, `internal/cli/retro.go`, and their tests |
| L03 Application / workflow | Engineering Manager | `internal/audit/**` |
| L07 Operational data / state | Production Engineer | `internal/config/**` |
| none today | — | `internal/delivery/**`: a new package. It matches no layer glob in LAYERS.md (`internal/verbs/registry.go:363` lists L03's globs; `internal/delivery` is not among them). |

A slice whose paths fall in no layer is refused: "the paths fall in no layer of LAYERS.md"
(`internal/planning/slice.go:248`). So the first slice must add `internal/delivery/**` to a layer in
`internal/verbs/registry.go` and run `go generate ./...` (inferred from that rule).

## What exists on dev (884d81b)

| Part | Where | Relation to #872 |
|---|---|---|
| Ship record | `internal/cli/ship.go:67` `ShipRecord`; written at `:553-562` to `proof/<story>/<head>/authoritative/ship.json` | Per PR, at merge: story, PR, head, merge commit, pack, gate run, review rung, footprint, `shipped_at`, `shipped_by`. No drafted, Ready, build, or review times; no cost, host, or model. 103 such files are on the ledger on 2026-11-07, the last for #1178 (PR #1238) (`git ls-tree -r origin/ledger proof`). |
| Release rung | `internal/cli/release.go:633` `rungPath`, written at `:510-518` | `releases/<n>/<target>.json` per release and target, plus `measure/due/…`. Not a per-story line. |
| Measured cost | `internal/cli/build.go:1142` `Env.buildCost`; `internal/cli/status.go:97` `Env.costs` | Sums model spend per story from the local actions log. |
| Story run in a mission | `internal/cli/missiontime.go:294` `StoryRunRecord` | Start, end, and waits of an ended story in `missions/<f>/plan.json` (Story #1015). |
| Code that waits for #872 | `internal/cli/missionnext.go:242`, `:369` (at bc78349; `:241`, `:359` at 884d81b); `internal/cli/mission.go:1399-1406`; `internal/cli/missiongate.go:1345`; `internal/cli/missionask.go:72`; `internal/cli/missionrecover.go:368-373` | Each counts a story at the per-story cap "until #872 measures spend". |
| `delivery/` on the ledger | none | `git ls-tree -d origin/ledger` lists actions, audit, autonomy, build, delegate, inspect, measure, missions, proof, releases, retro, specs, and tell. |

## Planned design (from #872)

1. `internal/delivery`: the line schema and the append. Planned, not built.
2. `ship` appends the story line at merge (#872 Summary). Planned, not built. Inferred: it fits next to the ship
   record write at `internal/cli/ship.go:553-562`, in the same ledger commit.
3. `release` appends one line per release (#872 Summary). Planned, not built.
4. `build` supplies the first build start and the origin (inferred from #872 Expected paths `internal/cli/build.go`).
5. `audit` and `retro` read the lines (inferred from #872 Expected paths).
6. Config: `internal/config/**` is listed; the issue names no key. Unverified which key is meant.

## Data and ledger records

| Record | Planned content |
|---|---|
| `delivery/stories.jsonl` | one line per merged story, fields from #872 Scope |
| `delivery/releases.jsonl` | one line per release |

## Decisions

No Decision is linked from #872. Decision #868, which planned to read `delivery/stories.jsonl` for `in_flight`, was
cancelled and superseded by Decision #918 (#918 Links: `supersedes #868`).
