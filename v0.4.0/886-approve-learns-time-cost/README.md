# Approve to build learns time and cost from merged stories

Feature [#886](https://github.com/rahulnakmol/agentic-ayudha/issues/886) · milestone v0.4.0 · Release
[#906](https://github.com/rahulnakmol/agentic-ayudha/issues/906) · label `status:planned` · priority P2.

## Summary

Approve to build's time and cost estimates are to learn from merged stories' history in `delivery/stories.jsonl`,
and the budget is to hold on measured spend (#886 Summary and Scope). This is step S17 of the core design under
Initiative [#870](https://github.com/rahulnakmol/agentic-ayudha/issues/870). It depends on
[#872](https://github.com/rahulnakmol/agentic-ayudha/issues/872), which is not built, so no history exists yet. The
scope was narrowed on 2026-10-28: `in_flight`, waves, the critical path, and workstreams went out (#886 Scope, Out).
The Feature is not sliced. On `dev`, Approve to build and the mission already call two empty hooks for it:
`Env.costEstimate` returns nil "until #886 merges" (`internal/cli/mission.go:752-759`), and `Env.workingHours`
returns nil "until #886 lands" (`internal/cli/missiontime.go:49-56`). Today the budget line ends with "no cost estimate from #886
yet" (`internal/cli/mission.go:895`).

## Status

| Item | Delivered | In review | Pending | Not sliced |
|---|---|---|---|---|
| Stories | 0 | 0 | 0 | all; 4 slices proposed, not filed |
| Bugs | 0 | 0 | 0 | none named |

Sources: `_data/subs.tsv` has no row for 886; `_data/merge-refs.tsv` and `_data/open-prs.tsv` name no PR for #886;
no `specs/886/` on `origin/ledger`.

## Files

1. [Goal and intent](01-goal-and-intent.md)
2. [Spec](02-spec.md)
3. [Technical design](03-technical-design.md)
4. [UX design](04-ux-design.md)
5. [Implementation plan](05-implementation-plan.md)
6. [Sequencing](06-sequencing.md)
