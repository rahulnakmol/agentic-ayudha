# Review nudges: PR #219 (story #212, Purge proof media by kind)

What this is really about: a purge that cannot say what it purged. Criterion 2 of #212 asks for a tombstone that
records the kind, and the PR delivers the purge without the record.

## Findings

1. Blocking. `PurgeByKind` writes one tombstone per object through `l.Purge` (`internal/proof/purge.go:40-47`), but
   `Tombstone` holds only `SHA` and `Purged` (`internal/ledger/tombstone.go:12-16`). Criterion 2 of #212 cannot pass.
2. Blocking. Adding the field is an L07 change (`LAYERS.md`: `internal/ledger/**`), and existing tombstones need a
   migration; the plan has two L04 checkpoints and no L07 checkpoint or rollback (`ledger/plans/212/plan.json`).
3. Note. The ledger package already stores the kind of every purged object elsewhere, so the migration is small
   and low risk.
4. Watch. Write the migration's tests in the same checkpoint: the coverage floor is 90.0 on changed lines
   (`CONSTRAINTS.md`), and a migration without tests is the usual way to miss it.

Analogy: a receipt printer that prints the total but not the items is fast, and useless at audit time. Finding 1 is
that receipt, in `internal/ledger/tombstone.go:12-16`.

## Next step

Which layer owns the record of a purge? If the answer is L07 (`LAYERS.md`), the next step is a re-plan: add an L07
checkpoint for `Kind` and its migration with tests, a rollback step, and re-size #212 as M before more code. Owner: the
Code Architect twin for the plan, then the AI Native Engineer twin; I check the new plan on #212 before `build` resumes.

Disposition: fix. No approval is implied; PR #219 still needs the merge gate and a human pass.
