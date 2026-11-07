# #890 Sequencing

Seven units of work: two filed Stories and five proposed slices (see
[05-implementation-plan.md](05-implementation-plan.md)). The plan uses at most 3 lanes in a wave, not 10. Two
groups of files force a strict order: the verify config keys (#1107, #1108, P3 all add to `internal/config/`) and
the PR-body path (P1, P2, P3). More lanes would edit the same files.

## Prerequisites outside this Feature

| Prerequisite | Needed by | State |
|---|---|---|
| [#1028](https://github.com/rahulnakmol/agentic-ayudha/issues/1028) risk register | #1107 | Story, `status:planned`, v0.4.0 |
| [#887](https://github.com/rahulnakmol/agentic-ayudha/issues/887) Review guidance | P3 ("where it fails" covers the risk lines, #865) | Stories #1221 to #1223 planned |
| Decision [#902](https://github.com/rahulnakmol/agentic-ayudha/issues/902) tiers in code | P3 | not visible in `internal/inspect/risk.go` |
| [#897](https://github.com/rahulnakmol/agentic-ayudha/issues/897) one record per merged PR | P4 | Feature, not sliced |
| [#880](https://github.com/rahulnakmol/agentic-ayudha/issues/880), [#871](https://github.com/rahulnakmol/agentic-ayudha/issues/871) | P5 | Features, not sliced |

## Wave 0: person steps

- File P1 to P5 as Stories (agent drafts with `aayu slice`; a person agrees).
- Intent, spec, tech baselines for #890, each approved by a person (`aayu ready 890 --part <part>`).
- Approve to build (`aayu ready 890`) and mission start (`aayu mission start 890`), by a person.

## Wave 1

| Lane | Work | Files |
|---|---|---|
| 1 | P1 Defense text | `ETHOS.md`, `.github/pull_request_template.md`, `internal/prbody/pull_request_template.md` |
| 2 | P5 `aayu explain` (only once #880 and #871 have merged; otherwise move it to a later wave) | `internal/cli/explain.go` (new), `internal/verbs/registry.go`, generated `docs/README.md`, `LAYERS.md`, `docs/RACI.md`, `skills/*/SKILL.md`, `internal/mcp/schemas/*.json` |
| 3 | #1107 contracts floors (only once #1028 has merged) | `internal/verify/` (new floor file), `internal/config/config.go`, `internal/config/defaults.toml`, `internal/config/gen/main.go`, `CONSTRAINTS.md` |

P1 names #1107 and #1108 in its DoD checklist, but it needs only their names, not their code.

## Wave 2

| Lane | Work | Files |
|---|---|---|
| 1 | P2 Defense check | `internal/prbody/check.go`, `internal/cli/inspect_app.go`, `internal/cli/prbody.go` |
| 2 | #1108 boundaries and fixtures floors | `internal/verify/` (new floor files), `internal/config/config.go`, `internal/config/defaults.toml`, `internal/config/gen/main.go`, `CONSTRAINTS.md` |

## Wave 3

| Lane | Work | Files |
|---|---|---|
| 1 | P3 cold challenge | `internal/inspect/**`, `internal/cli/inspect.go`, `internal/cli/inspect_app.go`, `internal/config/config.go`, `internal/config/defaults.toml` |

P3 waits for P2 (same file `inspect_app.go`) and #1108 (same config files).

## Wave 4

| Lane | Work | Files |
|---|---|---|
| 1 | P4 spot check and ledger copy | `.github/workflows/aayudha-defense.yml` (new), `internal/cli/ship.go` |

## Hot files and serialized merges

- `internal/config/config.go`, `defaults.toml`, `gen/main.go`, generated `CONSTRAINTS.md`: #1107, #1108, P3 here,
  and config slices of #892, #884, #883. Merge in that order, one at a time, and run `go generate ./...` after each
  rebase (`generate-drift` check).
- `internal/verbs/registry.go` and everything generated from it (AGENTS.md, "Generated files"): P5 here and the new
  verb flags in #879 and #888.
- `internal/cli/inspect_app.go`: P2 and P3, in that order.

## Steps that need a person

- P1 edits `.github/pull_request_template.md`, a CODEOWNERS path (`/.github/`): the `protected-paths` check fails by
  design, and only the founder's recorded admin bypass merges it (AGENTS.md, "Protected paths").
- P4 adds a workflow under `.github/`. If it reads a secret or names an environment, it is in the narrow set by rule
  (AGENTS.md, Story PR rule).
- P3, #1107, #1108 touch `internal/config/` (`[autonomy] always_person`) and `CONSTRAINTS.md` (CODEOWNERS).
- P4 touches `internal/cli/ship.go` (`always_person`).
- #890 says "Its PR needs a person to ship."
- A person sets `[verify] dod_floors_blocking_from`; until then the new floors are advisory (#1107 criterion 5).
