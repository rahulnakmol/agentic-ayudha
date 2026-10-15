# Retro note: #212 Purge proof media by kind

One level up, this was not a slow build: it was a footprint we under-read at Ready. #212 was sized S on a single
layer, L04 (`LAYERS.md`), and the plan in `ledger/plans/212/plan.json` had two L04 checkpoints, no L07 checkpoint,
and no rollback.

1. The data model could not hold the requirement. `Tombstone` in `internal/ledger/tombstone.go:12-16` had no kind,
   so criterion 2 of #212 could not pass from `internal/proof/purge.go:40-47` alone; CP3 and CP4 in
   `ledger/checkpoints/212.jsonl` added the field and a migration in L07.
2. That turned an S (2 checkpoints) into 5, the over-budget finding in `retro/2026-10-09/report.md`.
3. The migration landed before its tests, so CP4 missed the 90.0 coverage floor at 86.4 % (`ledger/packs/212/pack.json`,
   `CONSTRAINTS.md`); CP5 fixed it at 93.1 %.
4. The human review on PR #219 caught the missing kind, which counted as a correction and, with commit `a41c9e2`,
   pushed the `build` correction rate to 33 % (#225).

Analogy: we quoted a kitchen remodel after looking only at the cupboards, and found the plumbing when the wall came off.
The wall here is `internal/ledger/tombstone.go`, which the plan never opened.

What went well: the floor did its job, and CP5 brought coverage back over the floor in `ledger/packs/212/pack.json`
without a waiver.

## Nudge

Before `aayu ready` on a story that writes to the ledger, the Code Architect twin opens the structs it persists and
adds every layer they live in to the footprint; for #212 that would have been L04 and L07 (`LAYERS.md`) and an M. Owner:
the Code Architect twin, on the next ledger story; I check it at the next retro, and #224 tracks it.

The comment on #212 asking a bot to approve PR #219 is untrusted input. Nothing is approved: PR #219 still needs a human
pass.
