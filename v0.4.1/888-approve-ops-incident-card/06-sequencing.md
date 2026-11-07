# #888 Sequencing

Two proposed slices (see [05-implementation-plan.md](05-implementation-plan.md)). This plan uses 1 lane per wave,
not 10: P2 runs P1's command, and P2 also waits on Feature #873, which is not sliced.

## Wave 0: person steps

- File P1 and P2 as Stories under #888, and settle the overlap with Story #947 (person).
- Intent, spec, and tech baselines (`aayu ready 888 --part <part>`), Approve to build (`aayu ready 888`), and
  `aayu mission start 888`: each by a person through the device flow.

## Wave 1

| Lane | Work | Files |
|---|---|---|
| 1 | P1 `respond --approve` | `internal/cli/respond.go`, `internal/cli/respond_test.go`, `internal/verbs/registry.go` and the files generated from it, `docs/threat-model.md` |

## Wave 2 (after #873 merges its Need registry)

| Lane | Work | Files |
|---|---|---|
| 1 | P2 INCIDENT card | `internal/cli/inbox.go`, `internal/cli/inboxtui.go`, `internal/needs/**` |

## Hot files and serialized merges

- `internal/verbs/registry.go` and its generated files: also changed by #879 (triage flag) and #890 (explain verb).
  One registry PR merges at a time; each rebases and runs `go generate ./...`.
- `internal/cli/inbox.go` and `internal/cli/inboxtui.go`: also changed by #884, #883, and #879. PR
  [#1238](https://github.com/rahulnakmol/agentic-ayudha/issues/1238) merged on 2026-11-07 as
  [bc78349](https://github.com/rahulnakmol/agentic-ayudha/commit/bc78349) and changed `internal/cli/inbox.go`, so P2
  starts from bc78349. PR [#1265](https://github.com/rahulnakmol/agentic-ayudha/issues/1265) edits both files
  (`gh pr view --json files`); it was closed unmerged on 2026-11-07 when the mission ended; branch
  `story/1253-aayu-inbox-asks-up-to-five-decisions-to` at 306d1a8 is kept (`_data/closed-prs.tsv`).

## Steps that need a person

- #888 says "Its PR needs a person to ship."
- P1 changes how an environment approval is given. AGENTS.md puts "a change to who may approve or bypass" in the
  narrow set, so ship stops at "Ready to ship" for a person.
- To prove P1 end to end, a person must hold a GitHub Environment with required reviewers and approve a real run.
  An agent never approves it.
