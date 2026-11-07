# Questions have an SLA, and overdue ones come first

- Issue: [#883](https://github.com/rahulnakmol/agentic-ayudha/issues/883)
- Milestone: v0.4.1 (Release [#1105](https://github.com/rahulnakmol/agentic-ayudha/issues/1105))
- Status label: `status:planned`, `priority:P2` ([#883](https://github.com/rahulnakmol/agentic-ayudha/issues/883))
- Parent: Initiative [#870](https://github.com/rahulnakmol/agentic-ayudha/issues/870) (v0.4.0)
- Core design step: S14 ([#883](https://github.com/rahulnakmol/agentic-ayudha/issues/883) Summary)
- Decision: [#862](https://github.com/rahulnakmol/agentic-ayudha/issues/862), accepted 2026-10-23

## Summary

`decideSweep` should walk questions as well as Decisions: Overdue at 24 h, then a provisional answer at 48 h for
two-way questions only ([#883](https://github.com/rahulnakmol/agentic-ayudha/issues/883)). On `dev` at
[884d81b](https://github.com/rahulnakmol/agentic-ayudha/commit/884d81b), the sweep handles Decisions only
(`decideSweep`, `internal/cli/decide.go:160`), and questions do not exist: there is no `aayu ask` verb (Feature #880
is not sliced). The Feature is not sliced. Release #1105 names it: Story #1106's open questions "age through #883's
SLA". Story [#940](https://github.com/rahulnakmol/agentic-ayudha/issues/940), under the v0.4.0 inbox Feature #975,
holds the Overdue area's terminal design.

## Status

| Item kind | Delivered | In review | Pending | Not sliced |
|---|---|---|---|---|
| Stories | 0 | 0 | 0 | whole Feature (4 proposed slices) |
| Bugs | 0 | 0 | 0 | — |

Sources: no sub-issues of #883; no row in `_data/subs.tsv`; no PR that refers to 883; no `specs/883` on the ledger.

## Files

1. [Goal and intent](01-goal-and-intent.md)
2. [Spec](02-spec.md)
3. [Technical design](03-technical-design.md)
4. [UX design](04-ux-design.md)
5. [Implementation plan](05-implementation-plan.md)
6. [Sequencing](06-sequencing.md)
