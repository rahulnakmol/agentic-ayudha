# Every PR into dev carries a whiteboard defense

- Issue: [#890](https://github.com/rahulnakmol/agentic-ayudha/issues/890)
- Milestone: v0.4.1 (Release [#1105](https://github.com/rahulnakmol/agentic-ayudha/issues/1105))
- Status label: `status:planned`, `priority:P2` ([#890](https://github.com/rahulnakmol/agentic-ayudha/issues/890))
- Parent: Initiative [#870](https://github.com/rahulnakmol/agentic-ayudha/issues/870) (v0.4.0)
- Core design step: S21 ([#890](https://github.com/rahulnakmol/agentic-ayudha/issues/890) Summary)
- Decision: [#904](https://github.com/rahulnakmol/agentic-ayudha/issues/904), accepted 2026-10-23, which supersedes
  [#865](https://github.com/rahulnakmol/agentic-ayudha/issues/865)

## Summary

Every PR that can merge into `dev` gets a Defense section that cites code. A cold challenge blocks a risky PR whose
defense the code does not support ([#890](https://github.com/rahulnakmol/agentic-ayudha/issues/890)). Release #1105
also gives this Feature the Definition of Done floors in verify: contract compatibility, generated reference docs,
layer boundaries, and test fixtures. Two Stories hold those floors,
[#1107](https://github.com/rahulnakmol/agentic-ayudha/issues/1107) and
[#1108](https://github.com/rahulnakmol/agentic-ayudha/issues/1108). Both are `status:planned`. The Defense part
itself (ETHOS principle 11, the PR template section, the check, the challenge, the spot check, `aayu explain`) is
not sliced. Nothing is built on `dev` at [884d81b](https://github.com/rahulnakmol/agentic-ayudha/commit/884d81b).

## Status

| Item kind | Delivered | In review | Pending | Not sliced |
|---|---|---|---|---|
| Stories | 0 | 0 | 2 (#1107, #1108) | the Defense part (5 proposed slices) |
| Bugs | 0 | 0 | 0 | — |

Sources: `_data/subs.tsv` rows for 890; GitHub sub-issues of #890 (1107, 1108); no PR in `_data/merge-refs.tsv` or
`_data/open-prs.tsv` refers to 890, 1107, or 1108; the ledger has no `specs/890`, `specs/1107`, or `specs/1108`.

## Files

1. [Goal and intent](01-goal-and-intent.md)
2. [Spec](02-spec.md)
3. [Technical design](03-technical-design.md)
4. [UX design](04-ux-design.md)
5. [Implementation plan](05-implementation-plan.md)
6. [Sequencing](06-sequencing.md)
