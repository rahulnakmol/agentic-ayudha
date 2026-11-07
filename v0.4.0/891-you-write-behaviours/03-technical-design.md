# Technical design: you write behaviours

Status: planned, not built. No tech baseline exists. The planned design is Decision #867 option 1 and the #891 body.

## Layers touched

| Layer | Owner seat | Paths (from #891 Expected paths, plus inferred) |
|---|---|---|
| L02 Interaction / API boundary | Code Architect | `internal/cli/decide.go`, `internal/cli/ship.go`; `internal/cli/inspect.go`, `internal/cli/build.go` (inferred from #867) |
| L03 Application / workflow | Engineering Manager | `internal/inspect/`, `internal/workitems/` |
| L04 Domain / policy | Quality Architect | `internal/verify/` |
| L09 Platform / operations | DevOps & DevEx Tinkerer | `.agents/skills/verify-aayudha/` |
| not in a layer glob | — | `ETHOS.md`, `.aayudha/config.toml` (`always_person`) |

## What exists on dev (884d81b)

| Part | Where | Today | Since |
|---|---|---|---|
| Feature map | `internal/verify/featuremap.go:22-26` (package comment), `FeatureMap`, `Feature`, `Check` (`:71-77`) | A map is a project skill `.agents/skills/verify-<product>/`; each check has `id`, `kind`, `evidence`, `requires`, `steps`. `CheckKinds` are `ui`, `cli`, `api`, `lib` (`:32-33`). No `says`, `rule`, or `source`. | WP4.5b, before v0.3.0 |
| Behaviour floor | `internal/verify/behaviour.go:20-30` | Runs each check through the map's control tool; "No model is involved (rule 7), and any failed check fails the floor." | WP4.5b.2, before v0.3.0 |
| Commands | `aayu verify --check <feature>[.<check>]` or `all`; `aayu verify --map` (`aayu verify --help`) | The behaviour floor on chosen checks; the drift check that exits 3 on drift. | before v0.3.0 |
| Nightly sweep | `loops/verify-sweep.toml` | Runs every check (#867 Evaluation). | before v0.3.0 |
| Evidence map | `internal/verify/evidence.go:43-56` `EvidenceMap` | `.aayudha/evidence/<story>.toml` links a criterion to tests and to map checks. | before v0.3.0 |
| Maps in this repository | `examples/benefits-qa/.agents/skills/verify-benefits-qa/`, `examples/notes/.agents/skills/verify-notes/` | Two example product maps (`git ls-tree -r origin/dev`). No map for aayu itself. | — |
| Decide | `aayu decide --help` | Flags `--accept`, `--draft`, `--pack`, `--seat`, `--waiver`. No `--behaviour` or `--sets`. | — |
| ETHOS | `ETHOS.md` | Ten numbered principles (1 to 10). The evidence does not show a principle 11, which principle 12 would follow. | — |
| Narrow set | `.aayudha/config.toml:75-101` `always_person` | Does not list `internal/verify/featuremap.go`, `internal/verify/behaviour.go`, or an approval reader. | — |
| Behaviours in Story bodies | for example #1221, #1223 bodies, `#### Behaviours` | Lines such as `says: …; rule: limit; source: criteria 1, 6`. No code reads them: `rg -i '\bsays\b' internal/verify internal/workitems` finds only comments, and no `Says` field. | text only |

## Planned design (Decision #867 option 1)

1. Check fields `says`, `rule`, `source`; derived status. Planned, not built.
2. `behaviours/approvals.jsonl` on the ledger, one line per approval with the hashes of `says`+`rule` and of the steps.
   Planned, not built.
3. `inspect` compares a PR's map changes with the approvals and holds the PR on a change to an approved check; a
   BEHAVIOUR CHANGE card in Decide (a Need kind, needs #873). Planned, not built.
4. `aayu decide --behaviour <ref> --pr <n> --approve|--refuse`, human-only, pinning the PR head. Planned, not built.
5. `aayu decide --accept` requires `--sets`. Planned, not built.
6. `; standing` criteria need a map check with `source` `#n/k` before `ship` merges. Planned, not built.
7. This repository's map, starting with the inbox behaviours. Planned, not built.
8. ETHOS principle 12; the gate code in `always_person`. Planned, not built.

Why the gate is in `inspect` and not only in CODEOWNERS: "`/.agents/` is in CODEOWNERS, so a map change in this
repository already needs the founder's bypass. That gate does not say which sentence changed, and other products do
not have it." (#867 Evaluation).

## Data and ledger records

| Record | Planned |
|---|---|
| `behaviours/approvals.jsonl` (ledger) | one line per approval (#867 option 1) |
| `.agents/skills/verify-aayudha/features/*.md` | the map of this repository |

## Decisions

- [#867](https://github.com/rahulnakmol/agentic-ayudha/issues/867) (accepted 2026-10-23): option 1. Marked not
  reversible (#867 Reversible: false).
- Depends on [#877](https://github.com/rahulnakmol/agentic-ayudha/issues/877) (Approve to build) by #891 Links; the
  body does not say which part.
