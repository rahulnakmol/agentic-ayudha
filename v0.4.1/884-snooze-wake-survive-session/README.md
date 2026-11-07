# Snooze, with others, and wake survive the session

- Issue: [#884](https://github.com/rahulnakmol/agentic-ayudha/issues/884)
- Milestone: v0.4.1 (Release [#1105](https://github.com/rahulnakmol/agentic-ayudha/issues/1105))
- Status label: `status:planned`, `priority:P2` ([#884](https://github.com/rahulnakmol/agentic-ayudha/issues/884))
- Parent: Initiative [#870](https://github.com/rahulnakmol/agentic-ayudha/issues/870) (v0.4.0)
- Core design step: S15 ([#884](https://github.com/rahulnakmol/agentic-ayudha/issues/884) Summary)

## Summary

A snooze or a consult should be recorded on the ledger, so the terminal, the web, the phone, and a second machine
agree ([#884](https://github.com/rahulnakmol/agentic-ayudha/issues/884)). On `dev` at
[884d81b](https://github.com/rahulnakmol/agentic-ayudha/commit/884d81b), the inbox's `s` key marks a card "skipped"
in a map that lives only for the session (`internal/cli/inboxtui.go:442`, `:1394-1398`). There is no snooze, no wake
rule, and no `inbox/` folder on the ledger. The Feature is not sliced. Story
[#939](https://github.com/rahulnakmol/agentic-ayudha/issues/939), under the v0.4.0 inbox Feature #975, holds the
snooze menus' terminal design.

## Status

| Item kind | Delivered | In review | Pending | Not sliced |
|---|---|---|---|---|
| Stories | 0 | 0 | 0 | whole Feature (4 proposed slices) |
| Bugs | 0 | 0 | 0 | — |

Sources: no sub-issues of #884; no row in `_data/subs.tsv`; no PR that refers to 884; no `specs/884` on the ledger;
the ledger's top-level folders are `actions audit autonomy build delegate inspect measure missions proof releases
retro specs tell` (no `inbox`).

## Files

1. [Goal and intent](01-goal-and-intent.md)
2. [Spec](02-spec.md)
3. [Technical design](03-technical-design.md)
4. [UX design](04-ux-design.md)
5. [Implementation plan](05-implementation-plan.md)
6. [Sequencing](06-sequencing.md)
