# #884 Sequencing

Four proposed slices (see [05-implementation-plan.md](05-implementation-plan.md)). This plan uses at most 2 lanes,
not 10. Every slice edits the inbox files, and P2 to P4 need P1's record. More lanes would collide in
`internal/cli/inboxtui.go`.

## Wave 0: person steps

- File P1 to P4 under #884 and settle the overlap with #939 (person).
- Baselines (`aayu ready 884 --part <part>`), Approve to build (`aayu ready 884`), `aayu mission start 884`, each by
  a person.

## Wave 1

| Lane | Work | Files |
|---|---|---|
| 1 | P1 ledger record | `internal/cli/inboxsnooze.go` (new), `internal/cli/inboxtui.go`, `internal/cli/inbox.go`, `internal/config/config.go`, `internal/config/defaults.toml`, `internal/config/gen/main.go`, `CONSTRAINTS.md` |

## Wave 2

| Lane | Work | Files |
|---|---|---|
| 1 | P2 wake | `internal/cli/inboxsnooze.go`, `internal/cli/inbox.go` |
| 2 | P4 with others | `internal/cli/inboxtui.go`, `internal/cli/inboxothers.go` (new) |

P2 and P4 edit no file in common if P2 keeps its change to the gather in `inbox.go` and P4 keeps its menu in
`inboxtui.go`.

## Wave 3 (after #883 and #880 give questions a due time)

| Lane | Work | Files |
|---|---|---|
| 1 | P3 past due | `internal/cli/inboxsnooze.go`, `internal/cli/inboxtui.go` |

## Hot files and serialized merges

- `internal/cli/inbox.go` and `internal/cli/inboxtui.go`: also changed by #883, #888, and #879. PR
  [#1238](https://github.com/rahulnakmol/agentic-ayudha/issues/1238) merged on 2026-11-07 as
  [bc78349](https://github.com/rahulnakmol/agentic-ayudha/commit/bc78349) and changed `internal/cli/inbox.go` (not
  `inboxtui.go`), so slices start from bc78349. PR [#1265](https://github.com/rahulnakmol/agentic-ayudha/issues/1265)
  edits both files (`gh pr view --json files`); it was closed unmerged on 2026-11-07 when the mission ended; branch
  `story/1253-aayu-inbox-asks-up-to-five-decisions-to` at 306d1a8 is kept (`_data/closed-prs.tsv`).
- `internal/config/*` and generated `CONSTRAINTS.md`: shared with #892, #890, #883. One config PR merges at a time.

## Steps that need a person

- P1 adds config keys in `internal/config/` (`[autonomy] always_person`) and changes `CONSTRAINTS.md` and
  `internal/config/defaults.toml` (CODEOWNERS).
- Proof of P1 to P4 needs a person at a terminal: the TUI is refused for agents (`aayu inbox --help`).
