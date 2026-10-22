# Decision pack: #212 split or re-plan (Drona, Engineering Manager twin; startup)

## Question

Should #212 be split into an L04 purge story now and an L07 tombstone story next, or re-planned as one M story while
PR #219 waits?

Is that the right question? It is, because #231 is blocked on #212 and PR #219 cannot merge with criterion 2 unproven
(`ledger/packs/212/pack.json`).

## Facts

- #212 is an S on L04 only, and has used 5 checkpoints against a budget of 2 (`retro/2026-10-09/report.md`).
- Criterion 2 needs the kind in each tombstone, and `Tombstone` has no kind field
  (`internal/ledger/tombstone.go:12-16`), which lives in L07 (`LAYERS.md`).
- The plan has no L07 checkpoint and no rollback (`ledger/plans/212/plan.json`).
- Decision #198 says existing tombstones are never rewritten, so any L07 change must be additive.
- #231 is blocked by #212.

## Options

1. **Split.** Narrow #212 to the purge (L04) without criterion 2, merge PR #219, and file an L07 story for the kind.
   - Gains: #231's dependency on the purge clears this week (#231).
   - Costs: #212's Completion Goal changes, so it must be re-readied, and audit-by-kind waits for the L07 story.
2. **Re-plan as M.** Keep criterion 2, add an L07 checkpoint (additive field, per #198) with its tests and a rollback,
   and hold PR #219 until it lands (`ledger/plans/212/plan.json`).
   - Gains: one story, one proof, criterion 2 met as written (#212).
   - Costs: PR #219 waits for two to three more checkpoints, and #231 stays blocked meanwhile.

## Recommendation

Option 2, re-plan #212 as an M. Criterion 2 is the point of the story, and the L07 change is small and additive
under #198. This is a two-way door: if the L07 checkpoint stalls, we can still split later (#212).

## Cost of waiting

Each day undecided keeps #231 blocked and PR #219 open with a changes-requested review (#219), and the `build`
correction rate stays above its 20 % threshold (#225).

## Pre-mortem

Sanjay, the Mentalist twin, assumed option 2 failed a week from now. Likeliest reasons:

- The audit reader refuses the new field, because it decodes with `DisallowUnknownFields`
  (`internal/audit/purges.go:20-31`). Early warning: a test that reads an old and a new tombstone fails.
- The migration temptation returns and breaks #198. Early warning: any checkpoint that touches existing tombstone
  lines in `internal/ledger/tombstone.go`.

## Questions that would change the answer

- Does #231 need audit-by-kind, or only the purge itself?
- Is a second re-ready of #212 acceptable to the Product Strategist this week?

## Your decision

Reply **re-plan** (option 2) or **split** (option 1) for #212.
