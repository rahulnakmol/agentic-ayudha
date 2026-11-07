# Feature #874: Feature baselines are drafted, approved, and pinned

| | |
|---|---|
| Issue | [#874](https://github.com/rahulnakmol/agentic-ayudha/issues/874) |
| Milestone | v0.4.0 (Release [#906](https://github.com/rahulnakmol/agentic-ayudha/issues/906)) |
| Status label | `status:ready` (Ready snapshot `specs/874/ready.md` on `ledger`, 2026-11-02) |
| Core design step | S5 ([#874](https://github.com/rahulnakmol/agentic-ayudha/issues/874) Summary) |
| Depends on | [#871](https://github.com/rahulnakmol/agentic-ayudha/issues/871) ([#874](https://github.com/rahulnakmol/agentic-ayudha/issues/874) Links) |

## Summary

Each part of a Feature (intent, spec, technical design, UX design) gets a baseline. An agent drafts it, only a person
approves it at the hash they saw, and a later edit shows as drift
([#874](https://github.com/rahulnakmol/agentic-ayudha/issues/874) Summary). Five of the seven Stories are merged on
`dev`: drafts and approvals, the Feature technical design in `aayu plan`, the per-part pins, drift with reasons, and
the early-build approval (commits in [05-implementation-plan.md](05-implementation-plan.md)). Two Stories are not
built: the risk register in each story plan
([#1028](https://github.com/rahulnakmol/agentic-ayudha/issues/1028)) and the failure-modes table
([#1106](https://github.com/rahulnakmol/agentic-ayudha/issues/1106), which has milestone v0.4.1). The founder
approved the Feature to build on 2026-11-02 (`specs/874/approved.json`, version 1, by `rahulnakmol`). Its mission
plan has no Start yet: `missions/874/plan.json` has no `started_by` and no `started_at`.

## Status

| State | Stories | Bugs |
|---|---|---|
| Delivered (merged into `dev`) | 5 ([#990](https://github.com/rahulnakmol/agentic-ayudha/issues/990), [#991](https://github.com/rahulnakmol/agentic-ayudha/issues/991), [#992](https://github.com/rahulnakmol/agentic-ayudha/issues/992), [#993](https://github.com/rahulnakmol/agentic-ayudha/issues/993), [#994](https://github.com/rahulnakmol/agentic-ayudha/issues/994)) | 0 |
| In review (open PR) | 0 (`_data/open-prs.tsv` has no PR that refs these issues) | 0 |
| Pending (not started) | 2 ([#1028](https://github.com/rahulnakmol/agentic-ayudha/issues/1028), [#1106](https://github.com/rahulnakmol/agentic-ayudha/issues/1106)) | 0 |
| Not sliced | none named by a source | none |

Source: `_data/subs.tsv` rows for 874, `_data/merge-refs.tsv`, `_data/open-prs.tsv`.

## Files

1. [01-goal-and-intent.md](01-goal-and-intent.md): goal, customer outcome, problem, measures, out of scope.
2. [02-spec.md](02-spec.md): functional and non-functional requirements, traced to Stories.
3. [03-technical-design.md](03-technical-design.md): layers, packages, ledger records, config keys, decisions.
4. [04-ux-design.md](04-ux-design.md): commands, flags, messages, and inbox cards.
5. [05-implementation-plan.md](05-implementation-plan.md): every Story with its evidence.
6. [06-sequencing.md](06-sequencing.md): how to build the rest in parallel lanes.
