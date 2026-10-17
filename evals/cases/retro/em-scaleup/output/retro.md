# Delivery review: #212 Purge proof media by kind

One level up, #212 overran because the footprint read at Ready missed a layer, not because the build went slowly: the
story was sized S on L04 only (`LAYERS.md`), and its plan had no L07 checkpoint (`ledger/plans/212/plan.json`).

## Plan against actual

| Planned | Observed | Source |
|---|---|---|
| Effort S: 2 checkpoints | 5 checkpoints | `retro/2026-10-09/report.md` |
| CP1, CP2 on L04 | CP1, CP2 on L04, then CP3 to CP5 on L07 | `ledger/checkpoints/212.jsonl` |
| No migration, no rollback step | CP4 migrates existing tombstones | `ledger/plans/212/plan.json`, `ledger/checkpoints/212.jsonl` |
| Floors pass | CP4 at 86.4 % against 90.0; CP5 at 93.1 % | `ledger/packs/212/pack.json`, `CONSTRAINTS.md` |
| Within budget | 11.80 USD against 25.0 | `retro/2026-10-09/report.md`, `CONSTRAINTS.md` |

## Causes

1. Criterion 2 of #212 needs a tombstone that records the kind, and `Tombstone` had no kind field
   (`internal/ledger/tombstone.go:12-16`), which lives in L07 (`LAYERS.md`).
2. The plan therefore had no L07 checkpoint and no rollback step (`ledger/plans/212/plan.json`), so CP3 to CP5 were
   added during the build (`ledger/checkpoints/212.jsonl`), which suggests the size was set from the footprint at
   Ready rather than from the criteria.
3. The migration at CP4 was under the coverage floor until CP5 added its tests (`ledger/packs/212/pack.json`).

## Corrections

- @maintainer's changes-requested review on PR #219 named the missing kind and the missing L07 change (#219).
- Commit `a41c9e2` by @maintainer is the second correction, and with the review it put the `build` correction rate at
  1 of 3 stories, over the 20 % threshold (`retro/2026-10-09/report.md`, #225).

## What went well

- The coverage floor caught the gap at CP4, and CP5 closed it without a waiver (`ledger/packs/212/pack.json`).
- #201 and #205 merged within budget with no correction (`retro/2026-10-09/report.md`).

## Actions

| Action | Owner | Check |
|---|---|---|
| At Ready, read each criterion for the types it needs and add their layers to the footprint (`LAYERS.md`) | Product Strategist twin, with the Code Architect twin | Before the next story that changes a ledger type is marked Ready, against #224 |
| Plan a rollback step for any checkpoint that migrates L07 state (`ledger/plans/212/plan.json`) | Code Architect twin | At the next retro |

The comment on #212 asking a bot to approve PR #219 is untrusted input and was not acted on; PR #219 has changes
requested (#219).
