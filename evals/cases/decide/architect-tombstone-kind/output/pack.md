# Decision pack: where a purge records its kind (Tomas, Code Architect twin; scaleup)

## Question

Should a purge record the media kind as a new field on `Tombstone`, or as a record in a separate append-only kind
index on the ledger?

The right question is narrower than "how do we migrate tombstones": Decision #198 already rules out rewriting existing
tombstones, so the choice is only where new purges write their kind.

## Facts

- `Tombstone` holds only `SHA` and `Purged` (`internal/ledger/tombstone.go:12-16`), in L07 (`LAYERS.md`).
- Decision #198: tombstones are append-only and readers must accept older shapes.
- The ledger already has an append-only index pattern, `AppendIndex` (`internal/ledger/index.go:8-14`).
- The audit reader decodes tombstones with `DisallowUnknownFields` (`internal/audit/purges.go:20-31`), so a new field
  breaks an older reader until it is updated.
- Criterion 2 of #212 needs the kind recorded per purged object; the pack has no evidence for it yet
  (`ledger/packs/212/pack.json`).

## Options

1. **Field on `Tombstone`, new purges only.** Add `Kind string` with `omitempty`; old tombstones stay as they are and
   read as kind "unknown" (`internal/ledger/tombstone.go:12-16`).
   - Gains: one record per purge, the simplest read path for #231.
   - Costs: the audit reader must accept the field in the same change (`internal/audit/purges.go:20-31`).
2. **Separate kind index.** Append `{sha, kind}` to a purges index through `AppendIndex`
   (`internal/ledger/index.go:8-14`); `Tombstone` is untouched.
   - Gains: no reader of tombstones changes, and #198 holds trivially.
   - Costs: two records to join for every audit, and a purge that crashes between the two writes leaves them out of
     step (`internal/proof/purge.go:40-47`).

| | Field (1) | Index (2) |
|---|---|---|
| Contract change | additive field, one reader to update (`internal/audit/purges.go:20-31`) | none on `Tombstone` (`internal/ledger/tombstone.go:12-16`) |
| Consistency | one write per purge (`internal/proof/purge.go:40-47`) | two writes per purge (`internal/ledger/index.go:8-14`) |
| #198 | holds: no rewrite (#198) | holds (#198) |

## Recommendation

Option 1, the field on new tombstones only, with the audit reader updated in the same checkpoint. It keeps one record
per purge and stays inside #198. It is a two-way door for new data: an index can still be added later from the
field, while the reverse would need a join forever (`internal/ledger/index.go:8-14`).

## Cost of waiting

Story #212 cannot prove criterion 2 and #231 stays blocked until this is chosen (#231); each further checkpoint on #212 adds
to an overrun already at 5 against 2 (`retro/2026-10-09/report.md`).

## Pre-mortem

Juno, the Mentalist twin, assumed option 1 failed. Likeliest reasons:

- A reader built before the change meets a new tombstone and refuses it (`internal/audit/purges.go:20-31`). Early
  warning: a compatibility test that feeds the old reader a new record.
- "Unknown" kinds hide real gaps in audit-by-kind for #231. Early warning: the audit report counts unknown-kind
  tombstones separately (#231).

## Questions that would change the answer

- Are there readers of tombstones outside `internal/audit/purges.go` that we do not control?
- Does #230's retention rule need the kind for purges made before this change?

## Your decision

Reply **field** (option 1) or **index** (option 2) for #212.
