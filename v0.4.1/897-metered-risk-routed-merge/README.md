# Metered, risk-routed merge: one record per merged PR and review depth by risk

| Field | Value |
|---|---|
| Issue | [#897](https://github.com/rahulnakmol/agentic-ayudha/issues/897) |
| Milestone | v0.4.1 (Release [#1105](https://github.com/rahulnakmol/agentic-ayudha/issues/1105)); first listed in v0.4.0 (Release [#906](https://github.com/rahulnakmol/agentic-ayudha/issues/906), "includes #897") |
| Status label | `status:planned` (also `priority:P1`; no effort or risk label) |
| Depends on | [#871](https://github.com/rahulnakmol/agentic-ayudha/issues/871) (v0.4.0, open, no filed Stories) |
| Needed by | [#899](https://github.com/rahulnakmol/agentic-ayudha/issues/899), [#900](https://github.com/rahulnakmol/agentic-ayudha/issues/900) (both depends-on #897), [#898](https://github.com/rahulnakmol/agentic-ayudha/issues/898) (the meter read tool) |
| Related decisions | [#902](https://github.com/rahulnakmol/agentic-ayudha/issues/902) (done, risk tiers), [#910](https://github.com/rahulnakmol/agentic-ayudha/issues/910) (done, review on every tier), [#1255](https://github.com/rahulnakmol/agentic-ayudha/issues/1255) (done, merge on proof at any risk) |
| Code checked | `dev` at [884d81b](https://github.com/rahulnakmol/agentic-ayudha/commit/884d81b), 2026-11-03 |

## Summary

Every merged pull request is to leave one record that the pull request cannot write, and review depth is to follow a
risk tier that the binary computes from the diff ([#897](https://github.com/rahulnakmol/agentic-ayudha/issues/897),
Summary). Three later decisions change the Scope: #902 fixes the tier rules; #910 refuses a lighter review for low
risk ("review stays on every tier"); #1255 lets every agent PR merge on proof at any risk, which shrinks the
`always_person` set that #897's high tier is built on. On `dev`, `inspect.RiskOf` computes a different, older tier
(`internal/inspect/risk.go:45`), and only a Story PR merged by `aayu ship` leaves a record
(`proof/<story>/<head>/authoritative/ship.json`, `internal/cli/ship.go`, `ShipRecord`). Nothing of this Feature is
built, and it has no Stories yet.

## Status

| Item | Delivered | In review | Pending | Not sliced |
|---|---|---|---|---|
| Stories | 0 | 0 | 0 | all of Scope |
| Bugs | 0 | 0 | 0 | — |

Sources: GitHub sub-issues API `[]` for #897 (2026-11-07); no row in `_data/subs.tsv`; no PR refs 897 in
`_data/merge-refs.tsv` or `_data/open-prs.tsv`; no `specs/897/` on `origin/ledger`.

## Files

1. [Goal and intent](01-goal-and-intent.md)
2. [Specification](02-spec.md)
3. [Technical design](03-technical-design.md)
4. [UX design](04-ux-design.md)
5. [Implementation plan](05-implementation-plan.md)
6. [Sequencing](06-sequencing.md)
