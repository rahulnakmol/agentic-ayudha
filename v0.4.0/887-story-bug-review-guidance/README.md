# Every Story and Bug carries Review guidance

Feature [#887](https://github.com/rahulnakmol/agentic-ayudha/issues/887) · milestone v0.4.0 · Release
[#906](https://github.com/rahulnakmol/agentic-ayudha/issues/906) · label `status:planned` · priority P1.

## Summary

A Story or Bug carries Review guidance: `Check`, `Risk`, and `Ignore` lines. The guidance is frozen with the story at
Ready. The build brief and `aayu inspect` read the frozen copy. An Ignore line can drop only a low finding, and never a
P0 or P1 finding, or a security, data, or concurrency finding, or a floor. This is step S18 of the core design under
Initiative [#870](https://github.com/rahulnakmol/agentic-ayudha/issues/870) (#887 body). Decision
[#864](https://github.com/rahulnakmol/agentic-ayudha/issues/864) accepted the field with hard limits on Ignore on
2026-10-23 (#864 body, Decision section). The Feature is sliced into three Stories. Its intent, spec, and tech
baselines were approved on 2026-11-02 (`specs/887/baselines/*-v1.json` on the ledger). Approve to build has not
happened: the ledger has no `specs/887/approved.json` (`git ls-tree origin/ledger specs/887`). No code for the
Feature is on `dev` at 884d81b.

## Status

| Item | Delivered | In review | Pending | Not sliced |
|---|---|---|---|---|
| Stories | 0 | 0 | 3 ([#1221](https://github.com/rahulnakmol/agentic-ayudha/issues/1221), [#1222](https://github.com/rahulnakmol/agentic-ayudha/issues/1222), [#1223](https://github.com/rahulnakmol/agentic-ayudha/issues/1223)) | none named |
| Bugs | 0 | 0 | 0 | none named |

Sources: `_data/subs.tsv` (all three `OPEN`, `status:planned`); `_data/merge-refs.tsv` and `_data/open-prs.tsv`
name none of #887, #1221, #1222, or #1223.

## Files

1. [Goal and intent](01-goal-and-intent.md)
2. [Spec](02-spec.md)
3. [Technical design](03-technical-design.md)
4. [UX design](04-ux-design.md)
5. [Implementation plan](05-implementation-plan.md)
6. [Sequencing](06-sequencing.md)
