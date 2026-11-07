# One Need registry, and every page names a kind

Feature [#873](https://github.com/rahulnakmol/agentic-ayudha/issues/873) · milestone v0.4.0 · Release
[#906](https://github.com/rahulnakmol/agentic-ayudha/issues/906) · label `status:planned` · priority P1.

## Summary

The inbox kinds are to move into one Need registry, `internal/needs`. Each Need kind gets an area, a seat, a door, a
due rule, wake rules, actions, a page key, a question, and a next action (#873 Summary). The areas are Now, Overdue,
Ship, Answer, Decide, Ready, and Housekeeping (#873 Scope). A lint fails on a page with no kind. This is step S4 of
the core design under Initiative [#870](https://github.com/rahulnakmol/agentic-ayudha/issues/870). It depends on
Bug [#859](https://github.com/rahulnakmol/agentic-ayudha/issues/859) for deferred pages (#873 Scope); #859 is open.
Features [#880](https://github.com/rahulnakmol/agentic-ayudha/issues/880) and
[#891](https://github.com/rahulnakmol/agentic-ayudha/issues/891) depend on it. The Feature is not sliced, and
`internal/needs` does not exist on `dev`. The inbox still hard-codes its kinds in `internal/cli/inbox.go`. There are
now 18 kinds, not the ten the issue counted, because later Stories added kinds to the same list
(`internal/cli/inbox.go:46-63` on `dev` at bc78349; 17 at 884d81b, then Story #1178 added `overnight`). Eleven page call sites still use free-form keys (`rg 'notify.Page\{' internal`).

## Status

| Item | Delivered | In review | Pending | Not sliced |
|---|---|---|---|---|
| Stories | 0 | 0 | 0 | all; 8 slices proposed, not filed |
| Bugs | 0 | 0 | 1 prerequisite outside the Feature (#859) | — |

Sources: `_data/subs.tsv` has no row for 873; `_data/merge-refs.tsv` and `_data/open-prs.tsv` name no PR for #873 or #859; no `specs/873/` on `origin/ledger`.

## Files

1. [Goal and intent](01-goal-and-intent.md)
2. [Spec](02-spec.md)
3. [Technical design](03-technical-design.md)
4. [UX design](04-ux-design.md)
5. [Implementation plan](05-implementation-plan.md)
6. [Sequencing](06-sequencing.md)
