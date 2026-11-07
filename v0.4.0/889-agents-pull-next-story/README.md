# Feature #889: Agents pull the next story on merge, each in its own thread

| | |
|---|---|
| Issue | [#889](https://github.com/rahulnakmol/agentic-ayudha/issues/889) |
| Milestone | v0.4.0 (Release [#906](https://github.com/rahulnakmol/agentic-ayudha/issues/906)); 9 of its sub-issues are in milestone v0.4.1 |
| Status label | `status:planned`, `priority:P1` (issue state OPEN) |
| Ledger baselines | `specs/889/baselines/intent-v1.json`, `spec-v1.json`, `tech-v1.json`, and `specs/889/design.json` (all approved by @rahulnakmol on 2026-11-02) |
| Facts as of | `origin/dev` = `884d81b`, 2026-11-07; since then `bc78349` (PR #1238, Story #1178 of Feature #1170) is on `dev`, and it closes no sub-issue of #889 |
| Paused | The founders paused v0.4.0 and v0.4.1 on 2026-11-07, and the repository is halted (`aayu halt`, Actions variable `AAYUDHA_HALT`); every loop schedule and subscription is paused. None of the open PRs of #889 (#1183, #1202, #1260, #1269, #1277) was closed by the end of the mission of #1170 |

## Summary

This Feature built the mission machinery of aayu. A person approves a Feature to build. Then `aayu mission start`
records the plan, the tool, and the budget, and starts one coordinating agent session, the Kanha. On each event (a
merge, a block, a failed start, an answered ask), the Kanha runs `aayu mission recover` and `aayu mission next`, and
it starts one story thread for each `start` line with `aayu build <n> --delegate <tool>`. Milestone gates, time
limits, a stop and resume control, a lock on the Feature's issues, an hourly safety tick, and a local Kanha for
Codex, Copilot, and OpenCode are on `dev` (see [05-implementation-plan.md](05-implementation-plan.md)). The Feature
body asked for `aayu build --next --dispatch` ([#889](https://github.com/rahulnakmol/agentic-ayudha/issues/889),
Summary). That flag does not exist: `aayu build --next` takes only `ship` or `pick` (`aayu build --help`, built from
`884d81b`). The dispatch went into `aayu mission` and the Kanha instead (`internal/verbs/registry.go`, verb
`mission`). What is not built: the per-Feature event record and progress comment
([#1010](https://github.com/rahulnakmol/agentic-ayudha/issues/1010)), paging
([#1011](https://github.com/rahulnakmol/agentic-ayudha/issues/1011)), the feedback pass
([#1014](https://github.com/rahulnakmol/agentic-ayudha/issues/1014)), merge on proof inside a mission
([#1030](https://github.com/rahulnakmol/agentic-ayudha/issues/1030)), the 10-step build loop in the brief
([#1104](https://github.com/rahulnakmol/agentic-ayudha/issues/1104)), the local wake loop
([#1126](https://github.com/rahulnakmol/agentic-ayudha/issues/1126)), and the four v0.4.1 learning stories.

## Status

Counts are of the 65 sub-issues in `_data/subs.tsv`. "Delivered" means a squash merge on `dev` since v0.3.0
(`_data/merge-refs.tsv`) that refers to the issue, or a merged PR whose body names the issue but has no `Refs` line
(three cases, named in [05-implementation-plan.md](05-implementation-plan.md)).

| State | Stories | Bugs | Total |
|---|---|---|---|
| Delivered on `dev` | 16 | 25 | 41 |
| Partly delivered | 0 | 1 | 1 |
| In review (open PR) | 0 | 5 | 5 |
| Pending (not started) | 12 | 4 | 16 |
| Closed with no PR of its own | 0 | 2 | 2 |
| Total | 28 | 37 | 65 |

Notes:

- 38 issues are delivered through PRs with a `Refs` line (`_data/merge-refs.tsv`). Bugs
  [#1130](https://github.com/rahulnakmol/agentic-ayudha/issues/1130),
  [#1133](https://github.com/rahulnakmol/agentic-ayudha/issues/1133), and
  [#1195](https://github.com/rahulnakmol/agentic-ayudha/issues/1195) are delivered through PRs
  [#1135](https://github.com/rahulnakmol/agentic-ayudha/issues/1135),
  [#1145](https://github.com/rahulnakmol/agentic-ayudha/issues/1145), and
  [#1196](https://github.com/rahulnakmol/agentic-ayudha/issues/1196) with no `Refs` line. Their labels still say
  `status:ready`.
- Every delivered issue is still OPEN. The 38 with a `Refs` line have `status:merged`; the three above have
  `status:ready`. A story merges before its release closes it (comment on
  `lastGatePassed`, `internal/cli/missionnext.go`).
- The ledger has no `specs/889/approved.json` and no `missions/889/` (`git ls-tree origin/ledger`). So #889 never
  passed Approve to build and never ran as a mission itself. Its stories were built one by one.

## Files

1. [01-goal-and-intent.md](01-goal-and-intent.md): the goal, the customer, the problem, and what is out of scope.
2. [02-spec.md](02-spec.md): functional and non-functional requirements, each traced to a Story or Bug.
3. [03-technical-design.md](03-technical-design.md): architecture, layers, files, ledger records, config keys, and decisions.
4. [04-ux-design.md](04-ux-design.md): commands, flags, terminal output, inbox cards, and the agent-facing surface.
5. [05-implementation-plan.md](05-implementation-plan.md): every Story and Bug, with its evidence.
6. [06-sequencing.md](06-sequencing.md): how to build what is pending in waves of at most 10 parallel lanes.
