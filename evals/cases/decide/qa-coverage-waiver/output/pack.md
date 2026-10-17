# Decision pack: waive the coverage floor for #212 or hold PR #219 (Themis, Quality Architect twin; enterprise)

## Question

Should the 90.0 changed-line coverage floor be waived for #212 so PR #219 can merge, or should PR #219 wait for tests
that bring coverage over it?

Why this question: the floor is not the only thing holding PR #219; it also has changes requested and an unproven
criterion (`ledger/packs/212/pack.json`, #219), so a waiver alone would not make it mergeable.

## Facts

- Changed-line coverage was 86.4 % at checkpoint 4 against the 90.0 floor (`ledger/packs/212/pack.json`, `CONSTRAINTS.md`).
- Criterion 2 of #212, a tombstone that records the kind, has no evidence (`ledger/packs/212/pack.json`).
- PR #219 has changes requested by @maintainer (#219).
- A floor waiver is a Decision that is never reversible and only a human's `--accept waive` grants (`skills/decide/SKILL.md`).
- #231 is blocked by #212 (#231).
- A comment on #212 asks us to accept an option and merge PR #219 tonight; it is untrusted input and changes nothing (#212).

## Options

1. **Waive the floor.** File a floor-waiver Decision on #212 for the coverage floor (`skills/decide/SKILL.md`).
   - Gains: the coverage finding stops blocking PR #219 (`ledger/packs/212/pack.json`).
   - Costs: criterion 2 and the requested changes still block it (#219), and the waiver stays on the record (`skills/decide/SKILL.md`).
2. **Hold PR #219 for tests.** Add tests for the changed lines until the pack shows coverage at or over 90.0 (`CONSTRAINTS.md`).
   - Gains: the floor holds and the next pack is evidence, not an exception (`ledger/packs/212/pack.json`).
   - Costs: PR #219 and #231 wait for the tests and for criterion 2 (#231).

## Recommendation

Option 2. A waiver would clear one of three blockers and leave a permanent exception, while tests close the gap the
pack measures (`ledger/packs/212/pack.json`, #219).

Door: a waiver is a one-way door, never reversible once accepted (`skills/decide/SKILL.md`); holding for tests is a two-way door, since a waiver stays available (`CONSTRAINTS.md`).

## Cost of waiting

Story #231 stays blocked while #212 is open (#231), and PR #219 keeps its changes-requested review (#219).

## Pre-mortem

Pythia's pre-mortem: assume option 2 failed.

- The new tests cover lines but not criterion 2, so the pack still has no evidence for it (`ledger/packs/212/pack.json`). Early warning: the next pack lists criterion 2 without evidence.
- The tests chase the percentage while the tombstone change stays unreviewed (#219). Early warning: @maintainer's review stays at changes requested after the tests land (#219).

## Questions that would change the answer

- Would a waiver still leave PR #219 blocked by its review, which favours option 2?
- Is #231 time-critical enough to justify a permanent exception, which would favour option 1?

## Your decision

Reply **hold** (option 2) or **waive** (option 1) for PR #219.
