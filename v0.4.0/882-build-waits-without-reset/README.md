# A build waits on a person without a reset

Feature [#882](https://github.com/rahulnakmol/agentic-ayudha/issues/882) · milestone v0.4.0 · Release
[#906](https://github.com/rahulnakmol/agentic-ayudha/issues/906) · label `status:planned` · priority P2.

## Summary

A build that asks is to keep its lease and budget, and its wall clock is to pause until the answer comes (#882
Summary). The build state gets `waiting` and `paused`; the lease extends to the provisional time plus 4 h; the stale
sweep skips a waiting build; a clarifying answer causes no reset (#882 Scope). This is step S13 of the core design
under Initiative [#870](https://github.com/rahulnakmol/agentic-ayudha/issues/870). It depends on
[#880](https://github.com/rahulnakmol/agentic-ayudha/issues/880), which is not built. The Feature is not sliced. On
`dev` the reset it removes is still there: when a blocked story is made Ready again, `build` starts a new
`BuildState`, so its start time and failed attempts start over (`internal/cli/build.go:337-344`). Two related
pauses exist for other cases: a delegated build's pause on a usage limit is "no failed attempt"
(`internal/cli/build.go:540-550`), and a mission's clock pauses while every running story waits on a person
(`internal/cli/missiontime.go:30-37`, Story #1015).

## Status

| Item | Delivered | In review | Pending | Not sliced |
|---|---|---|---|---|
| Stories | 0 | 0 | 0 | all; 3 slices proposed, not filed |
| Bugs | 0 | 0 | 0 | none named |

Sources: `_data/subs.tsv` has no row for 882; `_data/merge-refs.tsv` and `_data/open-prs.tsv` name no PR for #882;
no `specs/882/` on `origin/ledger`.

## Files

1. [Goal and intent](01-goal-and-intent.md)
2. [Spec](02-spec.md)
3. [Technical design](03-technical-design.md)
4. [UX design](04-ux-design.md)
5. [Implementation plan](05-implementation-plan.md)
6. [Sequencing](06-sequencing.md)
