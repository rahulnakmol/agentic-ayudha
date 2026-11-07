# Goal and intent: Approve to build learns time and cost

No intent baseline exists (`specs/886/` is absent on `origin/ledger`). The source is the #886 body.

## Goal

"Approve to build's time and cost estimates learn from the merged stories' history, and the budget holds on measured
spend." (#886 Summary).

## Who it serves and the outcome

- The person who runs Approve to build (`aayu ready <feature>`, human-only, #877 Summary): the plan shows time and
  cost from history, not only the approved per-story cap.
- The mission: its clock and its budget hold use measured numbers (#886 Scope: "that Approve to build and the mission
  clock read").

## Problem

"`costEstimate` returns nil until #886 merges (`internal/cli/mission.go:627-634`), so Approve to build shows no
history for time and cost (G9, Q11). Decision #868." (#886 Problem). On `dev` at 884d81b, `costEstimate` is at
`internal/cli/mission.go:752-759`, and a second empty hook, `workingHours`, was added by Story #1015
(`internal/cli/missiontime.go:49-56`).

Decision [#868](https://github.com/rahulnakmol/agentic-ayudha/issues/868), which the issue cites, is cancelled
(`status:cancelled`). Decision [#918](https://github.com/rahulnakmol/agentic-ayudha/issues/918) superseded it on
2026-10-23 and says "#886 drops S5, S6 and S7" (#918 Options, option 1).

## Success measures

None is named in the issue.

## Out of scope

"`in_flight`, `in_flight_max`, and `target_prs_month`, which `[build] agents` replaced (#1089); waves and the
critical path, which mission milestones cover (#1093); the Workstream field, `[workstreams]`, and rotation across
workstreams (v0.4.1); a cap per workstream (off)." (#886 Scope, Out, narrowed 2026-10-28).

In scope by reference: Feature #877 leaves "a stop on the Feature's measured spend (#886)" to this Feature (#877
Scope, Out).
