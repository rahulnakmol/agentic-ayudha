# Feature #895: aayu inbox, wave 1: ship and review steps as terminal cards

- Issue: [#895](https://github.com/rahulnakmol/agentic-ayudha/issues/895) "aayu inbox, wave 1: every ship and review step
  a person takes is a card in the terminal inbox" (open).
- Milestone: v0.4.0 (Release issue [#906](https://github.com/rahulnakmol/agentic-ayudha/issues/906)).
- Labels: `type:feature`, `status:planned`, `priority:P2`, `effort:L`, `risk:medium`, `horizon:next` (issue #895).
- Parent: Initiative [#870](https://github.com/rahulnakmol/agentic-ayudha/issues/870) "aayu core: the human-dependency
  engine". #895 is wave 1 of 4 inbox Features: #895, [#975](https://github.com/rahulnakmol/agentic-ayudha/issues/975),
  [#976](https://github.com/rahulnakmol/agentic-ayudha/issues/976), and
  [#977](https://github.com/rahulnakmol/agentic-ayudha/issues/977) (issue #895, Summary).

## Summary

Feature #895 makes every ship and review step that needs a person a card in the terminal `aayu inbox`. Each card
says why it waits and shows its evidence. A batch ships after one presence check. A PR whose review cannot finish
becomes a card, not a silent no-reply. Every result shows on the card it acted on (issue #895, Summary). The
Feature body says that wave 1 "needs no change to the core" (issue #895, Summary). No Story or Bug of this Feature
is merged. The `aayu inbox` code on dev (`internal/cli/inbox*.go`) holds parts that the Stories build on, but it
satisfies no Completion Goal in full (see [05-implementation-plan.md](05-implementation-plan.md)). The ledger holds
no baselines for this Feature: `git ls-tree origin/ledger specs/895` lists no files.

## Status

Counts are from `_data/subs.tsv` (15 sub-issues), `_data/merge-refs.tsv` (no squash merge refs a sub-issue of #895),
and `_data/open-prs.tsv` (no open PR refs a sub-issue of #895).

| | Stories | Bugs | Total |
|---|---|---|---|
| Delivered (merged into dev) | 0 | 0 | 0 |
| In review (open PR) | 0 | 0 | 0 |
| Pending (not built) | 12 | 3 | 15 |
| Work not sliced into Stories | see [05-implementation-plan.md](05-implementation-plan.md), "Pending" | | |

All 15 sub-issues are open with `status:planned` (`_data/subs.tsv`).

## Files

1. [01-goal-and-intent.md](01-goal-and-intent.md): goal, outcome, problem, measures, out of scope.
2. [02-spec.md](02-spec.md): functional and non-functional requirements, traced to Stories and Bugs.
3. [03-technical-design.md](03-technical-design.md): layers, files, the code on dev today, and the planned design.
4. [04-ux-design.md](04-ux-design.md): commands, keys, cards, and words.
5. [05-implementation-plan.md](05-implementation-plan.md): every Story and Bug, with evidence.
6. [06-sequencing.md](06-sequencing.md): waves and lanes for the pending work, and the shared hot files.
