# Test plan: #212 Purge proof media by kind (re-plan after PR #219)

The riskiest thing here is not the purge, it is the record of it: criterion 2 of #212 needs the kind in every
tombstone, and `Tombstone` does not hold it yet (`internal/ledger/tombstone.go:12-16`).

| Risk | Test, at the lowest layer that catches it | Floor |
|---|---|---|
| A purge by kind leaves a tombstone without its kind (#212 criterion 2) | unit test on `PurgeByKind` in `internal/proof/purge_test.go` | coverage 90.0 (`CONSTRAINTS.md`) |
| The migration drops or rewrites existing tombstones | table test over old and new records in `internal/ledger/migrate_test.go` | coverage 90.0 (`CONSTRAINTS.md`) |
| A purge of one kind removes another kind's media | unit test with mixed kinds on `internal/proof/purge.go:40-47` | coverage 90.0 (`CONSTRAINTS.md`) |

Charter: explore `internal/ledger/migrate.go` with a ledger holding tombstones from before CP3, to discover what an
interrupted migration leaves behind; thirty minutes, notes on #212.

The migration's tests go in the same checkpoint as the migration, so the floor is met first time; CP4 missed it at
86.4 % (`ledger/packs/212/pack.json`).
