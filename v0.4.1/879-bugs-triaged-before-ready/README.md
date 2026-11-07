# Bugs are triaged before Ready

- Issue: [#879](https://github.com/rahulnakmol/agentic-ayudha/issues/879)
- Milestone: v0.4.1 (Release [#1105](https://github.com/rahulnakmol/agentic-ayudha/issues/1105))
- Status label: `status:planned`, `priority:P2` ([#879](https://github.com/rahulnakmol/agentic-ayudha/issues/879))
- Parent: Initiative [#870](https://github.com/rahulnakmol/agentic-ayudha/issues/870) (v0.4.0)
- Core design step: S10 ([#879](https://github.com/rahulnakmol/agentic-ayudha/issues/879) Summary)
- Decision: [#861](https://github.com/rahulnakmol/agentic-ayudha/issues/861), accepted 2026-10-23

## Summary

`aayu triage <bug> --decide hotfix|feature|repro|not-a-bug` (human-only) should record how a Bug is worked, and
Ready should refuse a Bug with no hotfix triage ([#879](https://github.com/rahulnakmol/agentic-ayudha/issues/879)).
Decision #861 made the rule. The Story half of #861 is on `dev`: a Story is Ready only while its Feature's approval
covers it (`storyCovered`, `internal/cli/readycover.go:23`; Story #996, PR #1026). The Bug half is not: `aayu ready`
applies no extra rule to a Bug (`internal/cli/ready.go:150-158`), and `aayu triage` only sets labels
(`internal/triage/triage.go`). The Feature is not sliced. Story
[#954](https://github.com/rahulnakmol/agentic-ayudha/issues/954), under the v0.4.0 inbox Feature #975, holds the
TRIAGE card's terminal design.

## Status

| Item kind | Delivered | In review | Pending | Not sliced |
|---|---|---|---|---|
| Stories | 0 | 0 | 0 | whole Feature (4 proposed slices) |
| Bugs | 0 | 0 | 0 | — |

Sources: no sub-issues of #879; no row in `_data/subs.tsv`; no PR that refers to 879; no `specs/879` on the ledger.

## Files

1. [Goal and intent](01-goal-and-intent.md)
2. [Spec](02-spec.md)
3. [Technical design](03-technical-design.md)
4. [UX design](04-ux-design.md)
5. [Implementation plan](05-implementation-plan.md)
6. [Sequencing](06-sequencing.md)
