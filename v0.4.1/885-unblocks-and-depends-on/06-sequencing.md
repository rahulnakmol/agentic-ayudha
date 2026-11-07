# #885 Sequencing

Three proposed slices (see [05-implementation-plan.md](05-implementation-plan.md)). This plan uses at most 2 lanes,
not 10: P2 and P3 both edit `internal/cli/buildnext.go`, and P3 needs P1's dependents count.

## Wave 0: person steps

- File P1 to P3 as Stories under #885; decide whether P3 also changes `mission next` (person).
- Baselines (`aayu ready 885 --part <part>`), Approve to build (`aayu ready 885`), and `aayu mission start 885`, each
  by a person.

## Wave 1

| Lane | Work | Files |
|---|---|---|
| 1 | P1 lineage read | `internal/lineage/**` (new), `internal/verbs/registry.go` (layer glob), generated `LAYERS.md` and `seats/*/CHARTER.md` (inferred from PR #1215, which changed both with the layer globs) |
| 2 | P2 depends-on in `pickNext` | `internal/cli/buildnext.go`, `internal/cli/buildnext_test.go` |

## Wave 2

| Lane | Work | Files |
|---|---|---|
| 1 | P3 unblocks key | `internal/cli/buildnext.go`, maybe `internal/cli/missionnext.go` |

## Hot files and serialized merges

- `internal/verbs/registry.go` and its generated files (P1's layer glob). Other Features change it too (#879, #888,
  #890). Merge one at a time; each runs `go generate ./...`.
- `internal/cli/missionnext.go`: open draft PR [#1269](https://github.com/rahulnakmol/agentic-ayudha/issues/1269)
  edits it (`gh pr view 1269 --json files`, read on 2026-11-07). Other open mission PRs are not checked here.

## Steps that need a person

- No proposed slice touches a path in `[autonomy] always_person` or CODEOWNERS, except `seats/*/CHARTER.md` if
  `go generate` rewrites it (CODEOWNERS `/seats/`). PR #1215, which changed the layer globs in
  `internal/verbs/registry.go`, also changed `seats/engineering-manager/CHARTER.md` (`gh pr view 1215 --json files`).
  So P1 likely rewrites a charter too and then needs the founder's bypass (inferred).
- Do not edit `loops/build-ready.toml` (in `always_person`); P2 changes the code the loop runs, not the loop file.
