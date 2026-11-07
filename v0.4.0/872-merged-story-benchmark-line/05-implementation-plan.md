# Implementation plan: one benchmark line per merged story

## Stories and Bugs

Feature #872 has no Stories or Bugs (`_data/subs.tsv`). The rows below are proposed, not filed. No source estimates effort
or risk.

| # | Title | Type | Status | Effort | Risk | Depends on | Layers or paths | Evidence |
|---|---|---|---|---|---|---|---|---|
| P1 | The delivery record: the story and release line schema and the append, in a new `internal/delivery` package with its layer glob | Story (proposed, not filed) | not filed | not estimated | not estimated | #871 (host, session) | `internal/delivery/**` (new), `internal/verbs/registry.go` (layer glob), generated `LAYERS.md` | not started |
| P2 | `ship` appends one story line at merge, with the build start and origin from `build` | Story (proposed, not filed) | not filed | not estimated | not estimated | P1 | L02: `internal/cli/ship.go`, `internal/cli/build.go`, tests | not started |
| P3 | `release` appends one release line and the released time | Story (proposed, not filed) | not filed | not estimated | not estimated | P1 | L02: `internal/cli/release.go`, tests | not started |
| P4 | `audit` and `retro` read the delivery lines (inferred from Expected paths) | Story (proposed, not filed) | not filed | not estimated | not estimated | P2 | L02, L03: `internal/cli/audit.go`, `internal/cli/retro.go`, `internal/audit/**` | not started |

## Delivered

Nothing under #872. Related records on `dev` that P2 and P3 can build on: the ship record
(`internal/cli/ship.go:67`) and the release rung (`internal/cli/release.go:633`), both from before v0.3.0. Mission
code that counts each story at its cap "until #872 measures spend" came with
[d941d15](https://github.com/rahulnakmol/agentic-ayudha/commit/d941d151) (#1084, Story #997),
[2114e00](https://github.com/rahulnakmol/agentic-ayudha/commit/2114e008) (#1087, Story #998),
[3ece84e](https://github.com/rahulnakmol/agentic-ayudha/commit/3ece84e7) (#1090, Story #1001), and
[bdd02c1](https://github.com/rahulnakmol/agentic-ayudha/commit/bdd02c13) (#1102, Story #1003), all under
Feature #889 (`git log -S 'until #872 measures spend'`).

## In review

None (`_data/open-prs.tsv`).

## Pending

- All of #872 Scope: P1 to P4.
- Person steps: baselines and Approve to build for #872 (none on the ledger).
- The "estimate and its basis" field: the estimate comes from #886, which depends on #872 (#886 Links). Inferred: the
  first lines can only record "no estimate" or the per-story cap basis that `mission.go:1399-1406` uses.
- The config key the Expected paths imply: unverified.
