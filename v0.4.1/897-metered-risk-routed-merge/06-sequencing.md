# Sequencing: #897 metered, risk-routed merge

The slices are the proposed ones in [05-implementation-plan.md](05-implementation-plan.md) (proposed, not filed).

## Before any build

1. File the Stories (`aayu slice 897`), restated against Decisions #902, #910, and #1255. A person approves the
   baselines and gives Approve to build ([af81cb48](https://github.com/rahulnakmol/agentic-ayudha/commit/af81cb48),
   #1018; [45a18e74](https://github.com/rahulnakmol/agentic-ayudha/commit/45a18e74), #1024).
2. Draft PR [#1282](https://github.com/rahulnakmol/agentic-ayudha/issues/1282) closed unmerged on 2026-11-07 when the mission ended; branch `feat/1281-auto-merge-on-proof` at `6f2aae7` kept (`_data/closed-prs.tsv`). It edited
   `.aayudha/config.toml`, `internal/config/`, `internal/cli/shippass.go`, and `internal/cli/shipproof.go`, which 897-A
   and 897-F also touch (`gh pr view 1282 --json files`). If Story #1281 is built again, sequence it before 897-A.
3. Decide what #897 needs from #871, or record that it needs nothing.

## Waves

| Wave | Lane | Slice | Files or packages it edits |
|---|---|---|---|
| 1 | 1 | 897-A risk tier | `internal/inspect/risk.go`, `internal/cli/proof.go`, `internal/config/config.go`, `internal/config/defaults.toml`, `internal/config/gen` |
| 1 | 2 | 897-B merge record workflow | `.github/workflows/aayudha-merge-record.yml` (new, name proposed), `internal/ledger/`, `internal/measure/record.go` (new) |
| 2 | 1 | 897-C nightly back-fill | `.github/workflows/aayudha-merge-record.yml` (a second trigger), `internal/measure/backfill.go` (new) |
| 2 | 2 | 897-E review path on every tier | `internal/inspect/inspect.go`, `internal/inspect/apps.go`, `internal/cli/inspect.go`, `internal/cli/inspect_app.go`, `docs/guide/how-merge-gate.md` |
| 3 | 1 | 897-D factory meter | `internal/measure/meter.go` (new), `internal/cli/measure.go` |
| 3 | 2 | 897-F rollout and kill switch | `internal/config/`, `internal/cli/ship.go` |

Order notes:

- 897-A and 897-F both edit `internal/config/`: waves 1 and 3.
- 897-B and 897-C share the record workflow: waves 1 and 2.
- 897-E edits `internal/inspect/inspect.go`, which #899 (899-B, 899-E, 899-D) also edits. Do not run 897-E in the same
  wave as an #899 lane on that file.

## Why fewer than 10 lanes

The Feature has six slices on two chains: tier then review then rollout, and record then back-fill then meter. The
widest wave has two lanes. The meter must wait for real records.

## Shared hot files

- `internal/inspect/inspect.go` (897-E; also #899).
- `internal/config/defaults.toml` and `internal/config/gen` (897-A, 897-F): generated `CONSTRAINTS.md` follows.
- `internal/cli/ship.go` (897-F). Closed PR #1282 did not edit it (it edited `ship_test.go`, `shippass.go`, and
  `shipproof.go`).

## Steps that need a person

| Step | Why | Source |
|---|---|---|
| 897-A, 897-F | `internal/config/` is in `always_person`; `internal/config/defaults.toml` is in `always_person_config` and `.github/CODEOWNERS`; #902 says "the PR that builds this needs a person to ship it". | `.aayudha/config.toml:75-102`; #902 Evaluation |
| 897-B, 897-C | New workflows under `/.github/`, a protected path; a workflow that writes the ledger holds a credential. | `.github/CODEOWNERS`; `AGENTS.md` |
| 897-F | `internal/cli/ship.go` is in `always_person` on `dev`; closed PR #1282 would have changed that, and Story #1281 is still open. | `.aayudha/config.toml:77` |
| 897-E | `[inspect]` and `[review]` are in `always_person_sections`. | `internal/config/defaults.toml:70` |
| Moving from silent to visible to enforced | The issue sets a 2-week silent period. Inferred: a person flips each state. | #897 Scope |
