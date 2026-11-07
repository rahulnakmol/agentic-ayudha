# Lean standing context, cut by experiment

| Field | Value |
|---|---|
| Issue | [#901](https://github.com/rahulnakmol/agentic-ayudha/issues/901) |
| Milestone | v0.4.1 (Release [#1105](https://github.com/rahulnakmol/agentic-ayudha/issues/1105)); first listed in v0.4.0 (Release [#906](https://github.com/rahulnakmol/agentic-ayudha/issues/906), "includes #901") |
| Status label | `status:planned` (also `priority:P2`; no effort or risk label) |
| Depends on | [#898](https://github.com/rahulnakmol/agentic-ayudha/issues/898) (v0.4.1, not built) |
| Code checked | `dev` at [884d81b](https://github.com/rahulnakmol/agentic-ayudha/commit/884d81b), 2026-11-03 |

## Summary

aayu is to measure the standing context of each build and inspect session, test an index in place of full skills on
the build and inspect evals, keep each cut that holds every eval bar, and remove verbs and loops unused for 30 days
([#901](https://github.com/rahulnakmol/agentic-ayudha/issues/901), Summary and Scope). The founder agreed it on
2026-10-21 for v0.4.0 (D8, issue text); Release #1105 says the 2026-10-26 scope triage moved Features to v0.4.1. On
`dev` there is no measure of standing context, no index variant of a skill, and no call log that marks a call as MCP
or CLI. The eval bars and their ratchet exist. Nothing of this Feature is built, and it has no Stories yet.

## Status

| Item | Delivered | In review | Pending | Not sliced |
|---|---|---|---|---|
| Stories | 0 | 0 | 0 | all of Scope |
| Bugs | 0 | 0 | 0 | — |

Sources: GitHub sub-issues API `[]` for #901 (2026-11-07); no row in `_data/subs.tsv`; no PR refs 901 in
`_data/merge-refs.tsv` or `_data/open-prs.tsv`; no `specs/901/` on `origin/ledger`.

## Files

1. [Goal and intent](01-goal-and-intent.md)
2. [Specification](02-spec.md)
3. [Technical design](03-technical-design.md)
4. [UX design](04-ux-design.md)
5. [Implementation plan](05-implementation-plan.md)
6. [Sequencing](06-sequencing.md)
