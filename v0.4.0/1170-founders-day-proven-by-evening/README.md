# Feature #1170: The founder's day: decide in the morning, see each Feature proven by evening

| | |
|---|---|
| Issue | [#1170](https://github.com/rahulnakmol/agentic-ayudha/issues/1170) (OPEN) |
| Milestone | v0.4.0 (Release [#906](https://github.com/rahulnakmol/agentic-ayudha/issues/906)) |
| Status label | `status:ready`, `priority:P1`, `effort:L`, `risk:medium`, `horizon:now` |
| Decision behind it | [#1168](https://github.com/rahulnakmol/agentic-ayudha/issues/1168) (closed, `status:done`); constrained later by [#1255](https://github.com/rahulnakmol/agentic-ayudha/issues/1255) (closed, `status:done`) |
| Ledger baselines | `specs/1170/baselines/intent-v1.json`, `spec-v1.json`, `spec-v2.json`, `spec-v3.json`, `tech-v1.json`, `specs/1170/design.json`, `specs/1170/approved.json` (approval version 3), `specs/1170/ready.md` |
| Mission record | `missions/1170/plan.json` (state `ended (founders regroup: v0.4 paused)`), `missions/1170/gates.jsonl`, `missions/1170/claims.jsonl` on `origin/ledger` |
| Facts as of | `origin/dev` = `bc78349` (`884d81b` plus PR #1238), read on 2026-11-07 after the mission ended |

## Summary

This Feature puts the founder's whole day into `aayu inbox`. In the morning, one inbox session with one device code
approves each baseline draft, Approve to build, and the mission start, in that order, and ends with "The day is set".
In the evening, the inbox shows one card per story agreed that morning, with each criterion next to its evidence, and
says "The day is proven" only when every criterion has evidence. A Feature's open Bugs go into a final BUGS milestone.
`aayu grill --edit` changes a filed body before Ready. After the evening review, an Overnight card sets what runs
overnight, bounded in hours and cost. All of this is on `dev`
([c858a80](https://github.com/rahulnakmol/agentic-ayudha/commit/c858a80),
[eb51d1b](https://github.com/rahulnakmol/agentic-ayudha/commit/eb51d1b),
[2715ef2](https://github.com/rahulnakmol/agentic-ayudha/commit/2715ef2),
[188bbb0](https://github.com/rahulnakmol/agentic-ayudha/commit/188bbb0),
[66397d3](https://github.com/rahulnakmol/agentic-ayudha/commit/66397d3),
[322e2ac](https://github.com/rahulnakmol/agentic-ayudha/commit/322e2ac),
[197ec9d](https://github.com/rahulnakmol/agentic-ayudha/commit/197ec9d),
[bc78349](https://github.com/rahulnakmol/agentic-ayudha/commit/bc78349)). The overnight card
([#1178](https://github.com/rahulnakmol/agentic-ayudha/issues/1178), PR
[#1238](https://github.com/rahulnakmol/agentic-ayudha/issues/1238)) merged on proof at 2026-11-07T12:34:51Z as
`bc78349`. Before the merge, @rahulnakmol accepted four of its findings as follow-ups. They are pending work, and no
issue holds them ([05-implementation-plan.md](05-implementation-plan.md), "Pending"). Not on `dev`: draft-first CI
([#1250](https://github.com/rahulnakmol/agentic-ayudha/issues/1250)), the two person windows
([#1254](https://github.com/rahulnakmol/agentic-ayudha/issues/1254)), merge on proof for every agent PR
([#1281](https://github.com/rahulnakmol/agentic-ayudha/issues/1281)), and 15 of the 23 Bugs.

The mission of #1170 started on 2026-10-31 (`missions/1170/plan.json`, control `start`). Milestone MS1 passed its
gate on `dev` `884d81b` on 2026-11-03T16:52:25Z (`missions/1170/gates.jsonl`, last line). On 2026-11-07T12:35:56Z
@rahulnakmol ended the mission with the reason "founders regroup: v0.4 paused" (`missions/1170/plan.json`, control
`end`; #1170 comment `aayudha:mission-ended`). The end closed 13 PRs unmerged between 12:38:55Z and 12:39:20Z. Each
refers to a sub-issue of #1170, and each branch is still on `origin` at the same head (`_data/closed-prs.tsv`;
`git ls-remote origin`). So nothing of #1170 is in review now. The 13 PRs are #1261 (#1250), #1270 (#1254), #1282
(#1281), and ten Bug PRs: #1199, #1214, #1220, #1236, #1249, #1259, #1262, #1265, #1275, #1276. Each must be rebuilt,
or reopened from its kept branch ([05-implementation-plan.md](05-implementation-plan.md), "Closed unmerged").

An ended mission holds nothing at ship: `missionHold` returns early with the comment "a mission that has not
started, or has ended, holds nothing" (`internal/cli/missionstop.go`). So MS2's gate no longer holds the BUGS PRs. An
ended mission also starts nothing: `aayu mission next` refuses with "The mission of #1170 ended (…). Nothing starts."
(`internal/cli/missionnext.go`, `missionNext`), and `stop` and `resume` refuse it (`missionstop.go`, `controllable`).
To build again, a person must approve #1170 to build again and start the mission again (how the code treats a
restart of this ended plan is unverified: [06-sequencing.md](06-sequencing.md), Wave 0). At 12:50:50Z @rahulnakmol
also halted the repository (Actions variable `AAYUDHA_HALT`), so a person must lift the halt first. The design block
is still there: Stories #1250 and #1254 (and #1281, which is outside the plan) touch layers outside the design's L02
and L03 (`specs/1170/design.json`), so no plan for them can pass (`internal/planning/design.go`, `CheckInDesign`).

## Status

Counts are of the 34 sub-issues in `_data/subs.tsv` (feature 1170). The GraphQL `subIssues` field of #1170 lists the
same 34. "Delivered" means a squash merge on `dev` that refers to the issue (`_data/merge-refs.tsv`), checked with
`git merge-base --is-ancestor <sha> origin/dev`.

| State | Stories | Bugs | Total |
|---|---|---|---|
| Delivered on `dev` | 8 | 8 | 16 |
| In review (open PR) | 0 | 0 | 0 |
| Closed unmerged on 2026-11-07, branch kept | 3 | 10 | 13 |
| Pending (no PR) | 0 | 5 | 5 |
| Total | 11 | 23 | 34 |

Notes:

- Every sub-issue is still OPEN on GitHub. Before the end, the delivered ones had the label `status:merged`, and
  #1178 had `status:in-progress`. The end set all 16 to `status:ready` or to no status. It also set nine
  `status:in-progress` items to `status:ready`, and removed `status:planned` from #1280, #1281, and #1283
  (`_data/relabel.tsv`). A restore script waits for the founder. This folder uses the labels from before the end.
- The mission counted "15 merged x $60" on 2026-11-02 (#1170 comment `aayudha:mission-ask` `budget-1271`). That
  matched the 15 delivered issues then. #1178 merged after that, so 16 are delivered now.
- [#1191](https://github.com/rahulnakmol/agentic-ayudha/issues/1191) is named in the `Refs` of merged PR
  [#1224](https://github.com/rahulnakmol/agentic-ayudha/issues/1224), but that PR fixes Bug
  [#1207](https://github.com/rahulnakmol/agentic-ayudha/issues/1207) (its body). #1191's own fix was PR
  [#1199](https://github.com/rahulnakmol/agentic-ayudha/issues/1199), closed unmerged. So this folder counts #1191 as
  closed unmerged.
- [#1281](https://github.com/rahulnakmol/agentic-ayudha/issues/1281) is a sub-issue of #1170, but it is not in
  `specs/1170/approved.json` and not in the mission graph of `missions/1170/plan.json`.
- PR [#1273](https://github.com/rahulnakmol/agentic-ayudha/issues/1273) (the skill text for #1251, no `Refs`) is
  still open. It was not closed by the end.
- Four gate fix PRs refer to #1170 itself and are on `dev`: [#1229](https://github.com/rahulnakmol/agentic-ayudha/issues/1229),
  [#1233](https://github.com/rahulnakmol/agentic-ayudha/issues/1233),
  [#1271](https://github.com/rahulnakmol/agentic-ayudha/issues/1271),
  [#1284](https://github.com/rahulnakmol/agentic-ayudha/issues/1284) (all `skills/retro/SKILL.md`).

## Files

1. [01-goal-and-intent.md](01-goal-and-intent.md): goal, customer, problem, measures, out of scope.
2. [02-spec.md](02-spec.md): functional and non-functional requirements, traced to Stories, Bugs, and criteria.
3. [03-technical-design.md](03-technical-design.md): layers, packages, ledger records, config keys, decisions.
4. [04-ux-design.md](04-ux-design.md): commands, inbox cards, terminal lines, the agent's surface.
5. [05-implementation-plan.md](05-implementation-plan.md): every Story and Bug, the closed PRs and their kept branches, blockers.
6. [06-sequencing.md](06-sequencing.md): waves and lanes to rebuild and finish the work, and the steps that need a person.
