# A repository's loops run under Kala, on every host

| Field | Value |
|---|---|
| Issue | [#911](https://github.com/rahulnakmol/agentic-ayudha/issues/911) |
| Milestone | v0.4.1 (Release [#1105](https://github.com/rahulnakmol/agentic-ayudha/issues/1105)) |
| Status label | `status:planned` (also `priority:P2`, `effort:L`, `risk:medium`, `horizon:next`) |
| Depends on | [#889](https://github.com/rahulnakmol/agentic-ayudha/issues/889) (v0.4.0, open; most Stories merged), [#893](https://github.com/rahulnakmol/agentic-ayudha/issues/893) (v0.4.1, not built) |
| Code checked | `dev` at [884d81b](https://github.com/rahulnakmol/agentic-ayudha/commit/884d81b), 2026-11-03 |

## Summary

A repository's agent loops run under one coordinator thread, Kala, in the loop owner's agent tool. Each loop keeps one
subthread that each run wakes again, instead of a new thread per run. aayu decides which loops are due (`aayu loop
--due`, no model); Kala only wakes them ([#911](https://github.com/rahulnakmol/agentic-ayudha/issues/911), Summary and
Scope). Kala shares one code path with Kanha, the Feature mission coordinator of #889 (decision Y10, issue text). Today
each loop is its own Amp schedule with `"run_mode": "fresh"` (`internal/loops/loops.go`, `AmpSchedule`), and
`loops/active.json` records one Amp thread per loop. On `dev`, the only Kala code is two comments that reserve a
`repo` scope for it. Nothing of this Feature is built, and it has no Stories yet.

## Status

| Item | Delivered | In review | Pending | Not sliced |
|---|---|---|---|---|
| Stories | 0 | 0 | 0 | all of Scope |
| Bugs | 0 | 0 | 0 | — |

Sources: the GitHub sub-issues API returns `[]` for #911 (2026-11-07); `_data/subs.tsv` has no row for 911;
`_data/merge-refs.tsv` and `_data/open-prs.tsv` have no PR that refs 911; the ledger has no `specs/911/`.

## Files

1. [Goal and intent](01-goal-and-intent.md)
2. [Specification](02-spec.md)
3. [Technical design](03-technical-design.md)
4. [UX design](04-ux-design.md)
5. [Implementation plan](05-implementation-plan.md)
6. [Sequencing](06-sequencing.md)
