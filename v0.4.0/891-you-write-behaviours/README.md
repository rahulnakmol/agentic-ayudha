# You write behaviours, and a change to one needs you

Feature [#891](https://github.com/rahulnakmol/agentic-ayudha/issues/891) · milestone v0.4.0 · Release
[#906](https://github.com/rahulnakmol/agentic-ayudha/issues/906) · label `status:planned` · priority P1.

## Summary

Each feature-map check is to carry the founder's sentence (`says`), its `rule`, and its `source`. The founder
approves the sentence, and a change to an approved check waits for him (#891 Summary). Decision
[#867](https://github.com/rahulnakmol/agentic-ayudha/issues/867) accepted this design on 2026-10-23. This is step
S22 of the core design under Initiative [#870](https://github.com/rahulnakmol/agentic-ayudha/issues/870). It
depends on [#873](https://github.com/rahulnakmol/agentic-ayudha/issues/873),
[#877](https://github.com/rahulnakmol/agentic-ayudha/issues/877), and
[#880](https://github.com/rahulnakmol/agentic-ayudha/issues/880). "Its PR needs a person to ship." (#891 Scope).
The Feature is not sliced. The engine under it exists on `dev` from before v0.3.0: feature maps, behaviour checks run
with no model, and the `aayu verify --check` and `--map` floors (`internal/verify/featuremap.go`,
`internal/verify/behaviour.go`). None of the new parts exists: no `says`, `rule`, or `source` field on a check
(`verify.Check`, `internal/verify/featuremap.go:71-77`), no `behaviours/approvals.jsonl`, no BEHAVIOUR CHANGE hold,
no `decide --behaviour` or `--sets`, and no map for this repository (the only maps are the examples
`examples/benefits-qa/.agents/skills/verify-benefits-qa/` and `examples/notes/.agents/skills/verify-notes/`).

## Status

| Item | Delivered | In review | Pending | Not sliced |
|---|---|---|---|---|
| Stories | 0 | 0 | 0 | all; 9 slices proposed, not filed |
| Bugs | 0 | 0 | 0 | none named |

Sources: `_data/subs.tsv` has no row for 891; `_data/merge-refs.tsv` and `_data/open-prs.tsv` name no PR for #891;
no `specs/891/` on `origin/ledger`.

## Files

1. [Goal and intent](01-goal-and-intent.md)
2. [Spec](02-spec.md)
3. [Technical design](03-technical-design.md)
4. [UX design](04-ux-design.md)
5. [Implementation plan](05-implementation-plan.md)
6. [Sequencing](06-sequencing.md)
