# #892 Sequencing

The slices are proposed, not filed (see [05-implementation-plan.md](05-implementation-plan.md)). This plan uses at
most 2 lanes in a wave, not 10. The Feature has four slices, and each later slice reads the config keys of P1, so
more lanes would only wait on P1 or edit the same files.

## Wave 0: person steps

| Step | Who | Command or place |
|---|---|---|
| File P1 to P4 as Stories under #892 | agent drafts, person agrees | `aayu slice`, then `aayu grill` per Story |
| Intent, spec, and tech baselines | person approves each draft | `aayu ready 892 --part intent\|spec\|tech --request` (agent), then `aayu ready 892 --part <part>` (person, device flow) |
| Approve to build | person | `aayu ready 892` (device flow); it writes `specs/892/approved.json` and `missions/892/plan.json` |
| Start the mission | person | `aayu mission start 892` |

## Wave 1

| Lane | Slice | Files |
|---|---|---|
| 1 | P1 config keys | `internal/config/config.go`, `internal/config/defaults.toml`, `internal/config/gen/main.go`, `CONSTRAINTS.md` (generated) |

## Wave 2

| Lane | Slice | Files |
|---|---|---|
| 1 | P2 breaker | `internal/models/gateway.go`, `internal/cli/build.go`, `internal/cli/delegate.go`, `internal/cli/buildnext.go`, `internal/cli/lease.go`, `internal/delegate/usage.go` |
| 2 | P3 init question | `internal/cli/init.go`, `internal/initrepo/**` |

P2 and P3 edit no file in common. Both read P1's fields, so both wait for P1 to merge.

## Wave 3

| Lane | Slice | Files |
|---|---|---|
| 1 | P4 read-out | `internal/cli/status.go`, `internal/status/status.go`, `docs/guide/how-models.md` |

## Hot files and serialized merges

- `internal/config/config.go`, `internal/config/defaults.toml`, `internal/config/gen/main.go`, and the generated
  `CONSTRAINTS.md`. Other v0.4.1 Features also add keys here (#890, #884, #883). Merge their config slices one at a
  time and run `go generate ./...` after each rebase.
- `internal/cli/build.go` and `internal/cli/delegate.go` are large files that many Bugs touch. Open Bug PRs on them
  today: [#1269](https://github.com/rahulnakmol/agentic-ayudha/issues/1269) and
  [#1278](https://github.com/rahulnakmol/agentic-ayudha/issues/1278) (`_data/open-prs.tsv`; their files are not
  checked here).
- `ratchets.toml`: the coverage ratchet can force a re-run after another PR merges (AGENTS.md, "Coverage ratchet").

## Steps that need a person

- P1 touches `internal/config/` (in `[autonomy] always_person`) and `CONSTRAINTS.md` and `internal/config/defaults.toml`
  (CODEOWNERS). Ship comments "Ready to ship" and the PR waits in `aayu inbox`.
- P3 writes to `.aayudha/config.toml` at run time in a product repository. In this repository a change to that
  file is in `always_person_config`.
- #892 itself says "Its PR needs a person to ship."
- Draft PR [#1282](https://github.com/rahulnakmol/agentic-ayudha/issues/1282) ("only the policy keys wait for a
  person") would have changed this list. It closed unmerged on 2026-11-07 when the mission ended; branch
  `feat/1281-auto-merge-on-proof` at `6f2aae7` is kept (`_data/closed-prs.tsv`). The list above is the one on `dev`.
