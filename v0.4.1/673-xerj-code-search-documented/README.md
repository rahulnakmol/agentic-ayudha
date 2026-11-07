# Use XERJ code search beside aayu, documented

- Issue: [#673](https://github.com/rahulnakmol/agentic-ayudha/issues/673) (title "Feature: Use XERJ code search beside
  aayu, documented")
- Milestone: v0.4.1 (Release [#1105](https://github.com/rahulnakmol/agentic-ayudha/issues/1105))
- Status label: `status:planned`, `horizon:next`, no priority label
  ([#673](https://github.com/rahulnakmol/agentic-ayudha/issues/673))
- Parent: Initiative [#30](https://github.com/rahulnakmol/agentic-ayudha/issues/30) "Dogfood the agentic delivery pod
  on aayudha itself"

## Summary

One guide page should tell an aayu user how to run XERJ (github.com/xerj-org/xerj, Apache-2.0) beside aayu, and
what to avoid. aayu does not run, proxy, configure, or depend on XERJ
([#673](https://github.com/rahulnakmol/agentic-ayudha/issues/673)). The page is not on `dev` at
[884d81b](https://github.com/rahulnakmol/agentic-ayudha/commit/884d81b): `docs/guide/guide-xerj.md` does not exist,
and `sitePages` in `internal/tell/site.go` has no XERJ entry. The v0.3.0 release notes list it under "Planned next,
for v0.4" (`docs/releases/v0.3.0.md:462`). The Feature is not sliced. It is one small docs change.

## Status

| Item kind | Delivered | In review | Pending | Not sliced |
|---|---|---|---|---|
| Stories | 0 | 0 | 0 | whole Feature (1 proposed slice) |
| Bugs | 0 | 0 | 0 | — |

Sources: no sub-issues of #673; no row in `_data/subs.tsv`; no PR in `_data/merge-refs.tsv` or `_data/open-prs.tsv`;
no `specs/673` on the ledger. `git log --all -- docs/guide/guide-xerj.md` is empty.

## Files

1. [Goal and intent](01-goal-and-intent.md)
2. [Spec](02-spec.md)
3. [Technical design](03-technical-design.md)
4. [UX design](04-ux-design.md)
5. [Implementation plan](05-implementation-plan.md)
6. [Sequencing](06-sequencing.md)
