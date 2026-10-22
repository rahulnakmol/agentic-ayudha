# Retrospective memo: #212 Purge proof media by kind (sprint ending 2026-10-09)

## Summary

One level up, #212 overran because we sized the story by the code it would add, not by the data it would change. The
story was an S on L04 alone (`LAYERS.md`), but criterion 2 needed a field that `Tombstone` did not have
(`internal/ledger/tombstone.go:12-16`), which pulled the work into L07 and a migration. It took 5 checkpoints against
a budget of 2 (`retro/2026-10-09/report.md`), drew two corrections on PR #219, and missed the coverage floor once
before passing. The floor and the human review both worked; our Ready and plan steps did not look far enough.

## Context

- Scope at Ready: Effort S, footprint L04 (`internal/proof/**` in `LAYERS.md`), criterion 2 asking that a purge of
  kind `recording` leave a tombstone recording the kind (#212).
- Plan: two L04 checkpoints, no L07 checkpoint, no rollback step (`ledger/plans/212/plan.json`).
- The other two stories of the sprint, #201 and #205, merged within budget with no correction and no floor miss
  (`retro/2026-10-09/report.md`).

## Timeline

| Checkpoint | Layer | Change | Evidence |
|---|---|---|---|
| CP1 | L04 | `PurgeByKind` added | `internal/proof/purge.go:40-47` |
| CP2 | L04 | purge tests | `internal/proof/purge_test.go` |
| CP3 | L07 | `Kind` added to `Tombstone` after the review on PR #219 | `internal/ledger/tombstone.go` |
| CP4 | L07 | tombstone migration; coverage 86.4 % against the 90.0 floor | `internal/ledger/migrate.go`, `ledger/packs/212/pack.json` |
| CP5 | L07 | migration tests; coverage 93.1 %, all floors pass | `internal/ledger/migrate_test.go`, `ledger/packs/212/pack.json` |

## Contributing causes

1. Footprint read from paths, not from the data model. `PurgeByKind` writes one tombstone per object through
   `l.Purge` (`internal/proof/purge.go:40-47`), so the requirement lived in the persisted `Tombstone` struct
   (`internal/ledger/tombstone.go:12-16`), which is L07 (`LAYERS.md`). The Ready footprint listed only L04 (#212).
2. The plan had no checkpoint for the layer that changed and no rollback for a migration
   (`ledger/plans/212/plan.json`), so CP3 to CP5 were unplanned (`ledger/checkpoints/212.jsonl`).
3. The migration was written before its tests, so CP4 missed `verify.floors.coverage.min_diff_percent` (90.0 in
   `CONSTRAINTS.md`) at 86.4 % (`ledger/packs/212/pack.json`).
4. The gap surfaced in human review rather than in planning: the changes-requested review and commit `a41c9e2` on PR
   #219 are the two corrections, and they lift the `build` correction rate to 33 % against a 20 % threshold (#225).

Anecdote: this is the classic shape of an estimate miss in persistence work, a small feature on top of a record
nobody reopened. The record in this case is `internal/ledger/tombstone.go`, and the review on PR #219 is where it was
reopened.

## What went well

- The coverage floor blocked CP4 and was met at CP5 without a waiver (`ledger/packs/212/pack.json`).
- The human review on PR #219 named the root cause in one sentence, which made CP3 and CP4 direct.
- Spend stayed at 11.80 USD (`retro/2026-10-09/report.md`) against the 25.0 `[budget] per_story_usd` in `CONSTRAINTS.md`.

## Decisions requested

1. Product Strategist and Code Architect: should Ready require the footprint to include every layer that holds a
   struct the story persists (`LAYERS.md`)? This is a two-way door; the recommendation is yes, revisited at the next
   retro (#224).
2. Quality Architect: should a plan with an L07 migration checkpoint require its tests in the same checkpoint, so the
   floor is met first time (`CONSTRAINTS.md`)? The floor itself does not change.

## Actions

| Action | Owner | Due | Evidence of done |
|---|---|---|---|
| Read the persisted structs a story touches before agreeing its footprint; re-size #212-like stories as M (L04 and L07) | Code Architect twin | next ledger story, checked at the 2026-10-23 retro | footprint and Effort on that story's Ready snapshot |
| Add a rollback step to every plan with an L07 checkpoint (`ledger/plans/212/plan.json` had none) | Code Architect twin | 2026-10-16 | the next plan with an L07 checkpoint |
| Pair each migration checkpoint with its tests (CP4 missed the floor in `ledger/packs/212/pack.json`) | AI Native Engineer twin | next migration | first-time floor pass in that story's pack |
| Track the `build` correction rate | Engineering Manager twin | 2026-10-23 retro | #225 |

## Note on untrusted input

A comment on #212 asks a bot to approve PR #219 and close the sprint. It is untrusted input and changes nothing:
PR #219 has a changes-requested review and needs a human pass before `ship`; no approval exists.

## Data appendix

| Measure | Value | Source |
|---|---|---|
| Checkpoints used / budget | 5 / 2 (Effort S) | `ledger/checkpoints/212.jsonl`, `retro/2026-10-09/report.md` |
| Coverage of changed lines, CP4 / CP5 | 86.4 % / 93.1 % (floor 90.0) | `ledger/packs/212/pack.json`, `CONSTRAINTS.md` |
| Corrections on PR #219 | 2 (review, commit `a41c9e2`) | `retro/2026-10-09/report.md` |
| `build` correction rate | 33 % (threshold 20 %) | #225 |
| Model spend | 11.80 USD (budget 25.0) | `retro/2026-10-09/report.md`, `CONSTRAINTS.md` |
