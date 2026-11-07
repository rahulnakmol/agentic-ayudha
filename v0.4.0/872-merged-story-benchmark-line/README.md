# Each merged story adds one benchmark line to the ledger

Feature [#872](https://github.com/rahulnakmol/agentic-ayudha/issues/872) · milestone v0.4.0 · Release
[#906](https://github.com/rahulnakmol/agentic-ayudha/issues/906) · label `status:planned` · priority P2.

## Summary

`aayu ship` is to append one line per story to `delivery/stories.jsonl` at merge, and `aayu release` one line per
release to `delivery/releases.jsonl` (#872 Summary). Each story line holds the story's times (drafted, Ready, first
build start, PR, first review, merged, released), its rounds, measured cost, host, model, estimate, and origin (#872
Scope). This is step S3 of the core design under Initiative
[#870](https://github.com/rahulnakmol/agentic-ayudha/issues/870). It depends on
[#871](https://github.com/rahulnakmol/agentic-ayudha/issues/871). Feature
[#886](https://github.com/rahulnakmol/agentic-ayudha/issues/886) depends on it. The Feature is not sliced, and none
of it is on `dev`: no `internal/delivery` package exists, and the ledger has no `delivery/` directory
(`git ls-tree -d origin/ledger`). Mission code on `dev` already waits for it: it counts each story at its per-story
cap "until #872 measures spend" (`internal/cli/missionnext.go:369` at bc78349; `:359` at 884d81b).

## Status

| Item | Delivered | In review | Pending | Not sliced |
|---|---|---|---|---|
| Stories | 0 | 0 | 0 | all; 4 slices proposed, not filed |
| Bugs | 0 | 0 | 0 | none named |

Sources: `_data/subs.tsv` has no row for 872; `_data/merge-refs.tsv` and `_data/open-prs.tsv` name no PR for #872;
no `specs/872/` on `origin/ledger`.

## Files

1. [Goal and intent](01-goal-and-intent.md)
2. [Spec](02-spec.md)
3. [Technical design](03-technical-design.md)
4. [UX design](04-ux-design.md)
5. [Implementation plan](05-implementation-plan.md)
6. [Sequencing](06-sequencing.md)
