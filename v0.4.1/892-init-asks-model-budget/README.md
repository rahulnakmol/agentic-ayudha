# aayu init asks how you pay for models, and sets the budget

- Issue: [#892](https://github.com/rahulnakmol/agentic-ayudha/issues/892)
- Milestone: v0.4.1 (Release [#1105](https://github.com/rahulnakmol/agentic-ayudha/issues/1105))
- Status label: `status:planned`, `priority:P2` ([#892](https://github.com/rahulnakmol/agentic-ayudha/issues/892))
- Parent: Initiative [#870](https://github.com/rahulnakmol/agentic-ayudha/issues/870) "aayu core: the human-dependency engine" (v0.4.0)
- Core design step: S23 ([#892](https://github.com/rahulnakmol/agentic-ayudha/issues/892) Summary)

## Summary

`aayu init` should ask, for each host, whether the person pays for models by subscription or pay as you go. It
should then ask for a daily budget, show what that budget buys, and write `[budget]`
([#892](https://github.com/rahulnakmol/agentic-ayudha/issues/892)). Decision
[#918](https://github.com/rahulnakmol/agentic-ayudha/issues/918) (accepted 2026-10-23) gave the init budget
question to this Feature. Nothing of it is built on `dev` at
[884d81b](https://github.com/rahulnakmol/agentic-ayudha/commit/884d81b): `runInit` (`internal/cli/init.go:130`)
asks only for confirmation, and no `[budget.hosts]` key exists (`config.Budget`, `internal/config/config.go:365`).
The Feature is not sliced into Stories, and it has no baselines on the ledger.

## Status

| Item kind | Delivered | In review | Pending | Not sliced |
|---|---|---|---|---|
| Stories | 0 | 0 | 0 | whole Feature (4 proposed slices) |
| Bugs | 0 | 0 | 0 | — |

Sources: `_data/subs.tsv` has no row for 892; GitHub lists no sub-issues of
[#892](https://github.com/rahulnakmol/agentic-ayudha/issues/892); `_data/merge-refs.tsv` and `_data/open-prs.tsv`
name no PR for it; `git ls-tree origin/ledger specs/` has no `specs/892`.

## Files

1. [Goal and intent](01-goal-and-intent.md)
2. [Spec](02-spec.md)
3. [Technical design](03-technical-design.md)
4. [UX design](04-ux-design.md)
5. [Implementation plan](05-implementation-plan.md)
6. [Sequencing](06-sequencing.md)
