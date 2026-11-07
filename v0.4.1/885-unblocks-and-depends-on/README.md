# Each item knows what it unblocks, and depends-on is enforced

- Issue: [#885](https://github.com/rahulnakmol/agentic-ayudha/issues/885)
- Milestone: v0.4.1 (Release [#1105](https://github.com/rahulnakmol/agentic-ayudha/issues/1105))
- Status label: `status:planned`, `priority:P2` ([#885](https://github.com/rahulnakmol/agentic-ayudha/issues/885))
- Parent: Initiative [#870](https://github.com/rahulnakmol/agentic-ayudha/issues/870) (v0.4.0)
- Core design step: S16 ([#885](https://github.com/rahulnakmol/agentic-ayudha/issues/885) Summary)

## Summary

A lineage read should find each item's parents, children, dependents, release, and who waits on it. The pick loop
should skip a story whose depends-on target has not merged
([#885](https://github.com/rahulnakmol/agentic-ayudha/issues/885)). Half of the second part is already on `dev`,
built for missions under Feature #889: `aayu mission next` holds a story while a depends-on target is open or
cancelled (`heldDeps`, `internal/cli/missionnext.go:478`; PR #1088 and PR #1184). The repository loop
`aayu build --next pick` does not check depends-on (`pickNext`, `internal/cli/buildnext.go:416`). No lineage package
and no reader of dependents exist. The Feature is not sliced.

## Status

| Item kind | Delivered | In review | Pending | Not sliced |
|---|---|---|---|---|
| Stories | 0 | 0 | 0 | whole Feature (3 proposed slices) |
| Bugs | 0 | 0 | 0 | — |

Related work delivered under another Feature: Story #999 (PR
[#1088](https://github.com/rahulnakmol/agentic-ayudha/issues/1088), commit
[9f8e457](https://github.com/rahulnakmol/agentic-ayudha/commit/9f8e457b)) and Bug #1169 (PR
[#1184](https://github.com/rahulnakmol/agentic-ayudha/issues/1184), commit
[c2d6bd7](https://github.com/rahulnakmol/agentic-ayudha/commit/c2d6bd70)), both under #889. They are not counted
above. Sources: no sub-issues of #885; no row in `_data/subs.tsv`; no PR that refers to 885; no `specs/885` on the
ledger.

## Files

1. [Goal and intent](01-goal-and-intent.md)
2. [Spec](02-spec.md)
3. [Technical design](03-technical-design.md)
4. [UX design](04-ux-design.md)
5. [Implementation plan](05-implementation-plan.md)
6. [Sequencing](06-sequencing.md)
