# Feature #975: aayu inbox, wave 2: the day in areas, asks, missions, and decisions

- Issue: [#975](https://github.com/rahulnakmol/agentic-ayudha/issues/975) "aayu inbox, wave 2: the day in areas, asks
  from any seat, missions, and every decision a person takes" (open).
- Milestone: v0.4.0 (Release issue [#906](https://github.com/rahulnakmol/agentic-ayudha/issues/906)).
- Labels: `type:feature`, `status:planned`, `priority:P2`, `effort:L`, `risk:high`, `horizon:next` (issue #975).
- Parent: Initiative [#870](https://github.com/rahulnakmol/agentic-ayudha/issues/870). #975 is wave 2 of 4 inbox
  Features: [#895](https://github.com/rahulnakmol/agentic-ayudha/issues/895), #975,
  [#976](https://github.com/rahulnakmol/agentic-ayudha/issues/976), and
  [#977](https://github.com/rahulnakmol/agentic-ayudha/issues/977) (issue #975, Summary).

## Summary

Feature #975 makes `aayu inbox` show every decision, answer, and alignment that a person owes the agents. Cards sit in
areas (Now, Overdue, Ship, Answer, Decide, Ready, Housekeeping), and each mission shows its state. Any seat asks on
an ANSWER card, and the answer reaches its thread. Decisions, plan reviews, blocked builds, incidents, the emergency
stop, Feedback, Feature baselines, Approve to build, triage, story reviews, overrides, and the stable release each
become a card with its next step (issue #975, Summary). Unlike wave 1, this wave "uses the core Features of
Initiative #870" and depends on 19 of them, #895 included (issue #975, Links). No Story of this Feature is merged.
Most of the core Features it depends on are not sliced into Stories yet (see
[05-implementation-plan.md](05-implementation-plan.md)). The ledger holds no baselines for this Feature:
`git ls-tree origin/ledger specs/975` lists no files.

## Status

Counts are from `_data/subs.tsv` (20 sub-issues), `_data/merge-refs.tsv` (no squash merge refs a sub-issue of #975),
and `_data/open-prs.tsv` (no open PR refs a sub-issue of #975).

| | Stories | Bugs | Total |
|---|---|---|---|
| Delivered (merged into dev) | 0 | 0 | 0 |
| In review (open PR) | 0 | 0 | 0 |
| Pending (not built) | 20 | 0 | 20 |
| Work not sliced into Stories | 13 of the 19 Features this one depends on have no sub-issues (GitHub API) | | |

All 20 sub-issues are open with `status:planned` (`_data/subs.tsv`). Some cards that these Stories describe exist in
part on dev, built by other Features: the BASELINE and Approve to build cards (Feature #1170) and the mission start,
gate, and time cards (Feature #889). See [05-implementation-plan.md](05-implementation-plan.md), "Delivered".

## Files

1. [01-goal-and-intent.md](01-goal-and-intent.md): goal, outcome, problem, measures, out of scope.
2. [02-spec.md](02-spec.md): functional and non-functional requirements, traced to Stories.
3. [03-technical-design.md](03-technical-design.md): layers, files, the code on dev today, the planned design, and
   the core dependencies.
4. [04-ux-design.md](04-ux-design.md): areas, cards, keys, and words.
5. [05-implementation-plan.md](05-implementation-plan.md): every Story, with evidence and its blocking core Features.
6. [06-sequencing.md](06-sequencing.md): waves and lanes for the pending work, and the shared hot files.
