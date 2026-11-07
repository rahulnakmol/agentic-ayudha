# Feature #982: Outcomes over output: every Feature names the customer outcome it serves

- Issue: [#982](https://github.com/rahulnakmol/agentic-ayudha/issues/982) (full title on GitHub: "… and aayu reads it
  after it ships")
- Milestone: v0.4.0 (Release issue [#906](https://github.com/rahulnakmol/agentic-ayudha/issues/906))
- Labels: `type:feature`, `status:planned`, `priority:P1`, `effort:M`, `risk:medium`, `horizon:now` ([#982](https://github.com/rahulnakmol/agentic-ayudha/issues/982))
- Decision: [#981](https://github.com/rahulnakmol/agentic-ayudha/issues/981), option 1 "Outcomes over output, in four
  parts", accepted by @rahulnakmol through the device code on 2026-10-24 (comment on #981)

## Summary

"Every Feature names the customer outcome it serves, aayu asks "did it move?" after it ships, and releases and retros
show outcomes next to counts. Each Goal states what the team does best, so new work is checked against it."
(`specs/982/baselines/intent-v1.json`, `summary`). One Story is on dev: `aayu measure` reads founder minutes per
merged PR from the actions log ([#989](https://github.com/rahulnakmol/agentic-ayudha/issues/989), PR
[#1016](https://github.com/rahulnakmol/agentic-ayudha/issues/1016),
[2e62c53](https://github.com/rahulnakmol/agentic-ayudha/commit/2e62c53)). The other five Stories are not started.

## Ledger state

| Record | State | Source |
|---|---|---|
| Intent v1 | approved by @rahulnakmol, 2026-11-02T17:23:46Z, device flow | `specs/982/baselines/intent-v1.json` |
| Spec v1 | approved by @rahulnakmol, 2026-11-02T17:25:05Z, device flow; pins #984–#989 | `specs/982/baselines/spec-v1.json` |
| Tech v1 | approved by @rahulnakmol, 2026-11-02T17:26:27Z, device flow; pins `specs/982/design.json` | `specs/982/baselines/tech-v1.json` |
| Technical design | footprint L03; six contracts; story order 984–989; a rollback plan | `specs/982/design.json` |
| UX design baseline | none on the ledger | `git ls-tree origin/ledger specs/982` |
| Approve to build | not given: no `specs/982/approved.json` | `git ls-tree origin/ledger specs/982` |

With the three baselines approved and no approval record, the inbox shows an Approve to build card for a Feature
(`approveCard` in `internal/cli/inboxapprove.go`). The UX part is needed only when the footprint has L01, and the
design footprint is L03. That the card waits in the founder's inbox now is from the task brief; I did not run
`aayu inbox`.

## Status

| | Stories | Bugs |
|---|---|---|
| Delivered (merged into dev) | 1 (#989, merged by a Solo-mode bypass, not by `aayu ship`; its proof pack landed on the ledger 5 days later) | 0 |
| In review (open PR) | 0 | 0 |
| Pending (not started) | 5 (#984–#988) | 0 |
| Not sliced | 1 scope item: the new 0.4.0 Goal | — |

Sources: `_data/subs.tsv`, `_data/merge-refs.tsv` (only PR #1016 refers to a Story of #982), `_data/open-prs.tsv`
(none).

## Files

1. [Goal and intent](01-goal-and-intent.md)
2. [Specification](02-spec.md)
3. [Technical design](03-technical-design.md)
4. [UX design](04-ux-design.md)
5. [Implementation plan](05-implementation-plan.md)
6. [Sequencing](06-sequencing.md)
