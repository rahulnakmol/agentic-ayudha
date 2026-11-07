# #883 Sequencing

Four proposed slices (see [05-implementation-plan.md](05-implementation-plan.md)). This plan uses at most 2 lanes,
not 10. Every slice needs P1's question clock, and P2 and P3 both edit `internal/cli/decide.go`. Nothing can start
until Feature #880 defines questions.

## Wave 0: prerequisites and person steps

- Feature [#880](https://github.com/rahulnakmol/agentic-ayudha/issues/880) is sliced and its question record merges.
  [#873](https://github.com/rahulnakmol/agentic-ayudha/issues/873) is linked too.
- Bug [#859](https://github.com/rahulnakmol/agentic-ayudha/issues/859) is fixed first or folded into P2 (a person
  decides).
- File P1 to P4 under #883; settle the overlap with #940 (person).
- Baselines (`aayu ready 883 --part <part>`), Approve to build (`aayu ready 883`), `aayu mission start 883`, each by a
  person.

## Wave 1

| Lane | Work | Files |
|---|---|---|
| 1 | P1 question clock | `internal/decide/**`, maybe `internal/config/config.go` and `defaults.toml` |

## Wave 2

| Lane | Work | Files |
|---|---|---|
| 1 | P2 sweep and page | `internal/cli/decide.go`, `internal/cli/decide_test.go` |
| 2 | P4 Overdue area | `internal/cli/inbox.go`, `internal/cli/inboxtui.go` |

## Wave 3

| Lane | Work | Files |
|---|---|---|
| 1 | P3 provisional answer | `internal/cli/decide.go`, `internal/decide/**` |

## Hot files and serialized merges

- `internal/cli/inbox.go` and `inboxtui.go`: shared with #884, #888, #879 and open inbox PRs. P4 and #884's slices
  merge one at a time.
- `internal/config/*` and generated `CONSTRAINTS.md`, if P1 adds a key: shared with #892, #890, #884.
- `loops/decisions.toml` is not in `always_person`; `loops/build-ready.toml` is, and no slice here touches it.

## Steps that need a person

- Any change to `[decisions]` keys touches `internal/config/` (`always_person`) and `internal/config/defaults.toml`
  (CODEOWNERS).
- Proof of P3 needs a reversible question that ages past 48 business hours, or a fake clock. #862 is not reversible
  itself, so it never applies provisionally.
