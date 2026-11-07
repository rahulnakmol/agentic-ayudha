# Implementation plan: Review guidance

## Stories and Bugs

| # | Title | Type | Status | Effort | Risk | Depends on | Layers or paths | Evidence |
|---|---|---|---|---|---|---|---|---|
| [#1221](https://github.com/rahulnakmol/agentic-ayudha/issues/1221) | A Story or Bug's Review guidance is parsed and checked at Ready, with the Ignore limits | Story | planned | M | medium | none | L02, L03: `internal/workitems/`, `internal/cli/ready.go`, `internal/cli/slice.go`, `internal/ready/` | not started |
| [#1222](https://github.com/rahulnakmol/agentic-ayudha/issues/1222) | The build brief carries the story's Review guidance from the Ready snapshot | Story | planned | S | low | #1221 | L02, L03: `internal/delegate/brief.go`, `internal/cli/delegate.go` | not started |
| [#1223](https://github.com/rahulnakmol/agentic-ayudha/issues/1223) | inspect reads the frozen Review guidance, keeps every limited finding, and re-reviews when it changes | Story | planned | M | medium | #1221 | L02, L03: `internal/inspect/`, `internal/cli/inspect.go`, `internal/cli/buildreview.go` | not started |

Sources: labels and titles from `_data/subs.tsv`; depends-on from each body's Links and from `spec-v1.json`
`depends_on`; layers from the `layer:` labels; paths from #887 Expected paths and each Story's Notes. No merge in
`_data/merge-refs.tsv` and no open PR in `_data/open-prs.tsv` refers to these issues.

## Delivered

Nothing of #887 is on `dev`. What exists and is reused:

- The approved baselines: intent, spec, and tech v1, approved 2026-11-02 (`specs/887/baselines/*-v1.json`). The
  machinery that records them came with [af81cb4](https://github.com/rahulnakmol/agentic-ayudha/commit/af81cb48)
  (#1018, Story #990 of Feature #874).
- The Ready snapshot reader `ready.ParseSnapshot` (`internal/ready/ready.go:358`), the brief (`internal/delegate/brief.go`),
  inspect's `DiffHash` (`internal/inspect/inspect.go:193`), and the Dropped list (`internal/inspect/inspect.go:229`).
  These predate the Feature. See [03-technical-design.md](03-technical-design.md).
- Approve to build itself, which this Feature waits for, is on `dev`:
  [45a18e7](https://github.com/rahulnakmol/agentic-ayudha/commit/45a18e74) (#1024, Story #995) and
  [52f8b96](https://github.com/rahulnakmol/agentic-ayudha/commit/52f8b96b) (#1026, Story #996).

## In review

None. `_data/open-prs.tsv` has no PR that refers to #887, #1221, #1222, or #1223.

## Pending

- All three Stories. Spec v1 gives their efforts: M, S, M.
- Approve to build for #887. It is a person's step: `aayu ready 887` through GitHub's device flow (`aayu ready
  --help`). The ledger has no `specs/887/approved.json`.
- Not sliced: the three differences from Decision #864 in [02-spec.md](02-spec.md) (line shape, the `always_person`
  path limit, required at Ready). The evidence does not show a Story for them. No source estimates them.
- Not sliced: a writer of the block. #1221 Notes assume slice writes it today; the code on `dev` does not (see
  [03-technical-design.md](03-technical-design.md)). No source estimates this.
