# Master test plan: #212 Purge proof media by kind (re-plan after PR #219)

## Scope

In scope: `PurgeByKind` (`internal/proof/purge.go:40-47`, L04 in `LAYERS.md`), the `Kind` field on `Tombstone` and
its migration (`internal/ledger/tombstone.go`, `internal/ledger/migrate.go`, L07 in `LAYERS.md`). Out of scope: media
storage back ends, which #212 does not change.

## Risk analysis

| ID | Risk | Likelihood | Impact | Source |
|---|---|---|---|---|
| R1 | A purge by kind leaves a tombstone without its kind, so criterion 2 fails and the purge cannot be audited | high | high | #212, review on PR #219 |
| R2 | The migration drops or rewrites existing tombstones | medium | high | `internal/ledger/migrate.go` |
| R3 | A purge of one kind removes another kind's media | low | high | `internal/proof/purge.go:40-47` |
| R4 | Changed-line coverage misses the floor again | medium | medium | `ledger/packs/212/pack.json` (86.4 % at CP4) |

## Test levels and types

| Risk | Unit | Integration | Exploratory |
|---|---|---|---|
| R1 | `internal/proof/purge_test.go`: kind recorded for each purged object | ledger round trip: purge, read tombstones | — |
| R2 | `internal/ledger/migrate_test.go`: table of pre-CP3 and post-CP3 records | migrate a fixture ledger twice (idempotence) | charter: interrupted migration |
| R3 | mixed-kind purge in `internal/proof/purge_test.go` | — | charter: purge while a capture is written |
| R4 | tests land in the migration's own checkpoint (`ledger/checkpoints/212.jsonl` shows them a checkpoint late) | — | — |

## Entry and exit criteria

- Entry: the re-plan on #212 has an L07 checkpoint and a rollback step (`ledger/plans/212/plan.json` today has neither).
- Exit: every floor in `CONSTRAINTS.md` passes on the merge gate's authoritative pack for PR #219, including coverage
  of changed lines at or above 90.0; no waiver is proposed (a waiver would be a Founder Decision).

## Environments and data

Synthetic fixture ledgers only, with tombstones written before and after CP3 (`internal/ledger/tombstone.go`).

## Roles and schedule

| Activity | Owner | When |
|---|---|---|
| Unit and integration tests | AI Native Engineer twin | with each L07 checkpoint on PR #219 |
| Exploratory sessions | Quality Architect twin | after the migration checkpoint in `internal/ledger/migrate.go` |
| Pack review | Quality Architect twin, with its paired human when there is one | on the merge gate's pack for PR #219 |

## Traceability

| Requirement | Tests | Evidence |
|---|---|---|
| #212 criterion 2 | R1 unit and integration | `ledger/packs/212/pack.json` criterion map |
| No data loss in migration | R2 unit, integration, charter | `ledger/packs/212/pack.json` |
