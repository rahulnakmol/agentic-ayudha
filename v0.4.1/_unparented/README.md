# v0.4.1 issues outside the v0.4.1 Features

Milestone v0.4.1 (Release [#1105](https://github.com/rahulnakmol/agentic-ayudha/issues/1105)) holds 31 issues
(`gh issue list -R rahulnakmol/agentic-ayudha --milestone v0.4.1 --state all --limit 200`, read 2026-11-07). 16 are
Features. 2 are sub-issues of a v0.4.1 Feature: Stories
[#1107](https://github.com/rahulnakmol/agentic-ayudha/issues/1107) and
[#1108](https://github.com/rahulnakmol/agentic-ayudha/issues/1108) under #890. The other 13 are listed here. Each has
a parent, but every parent is a v0.4.0 Feature. All 13 are open.

Parents come from GitHub's sub-issue `parent` field (GraphQL). Open PRs come from `_data/open-prs.tsv` and a search of
PR bodies for the issue number. Merged PRs come from `_data/merge-refs.tsv`.

## Bugs

| # | Title | Type | Status | Priority, effort, risk | Parent (milestone) | Open PR | Notes |
|---|---|---|---|---|---|---|---|
| [#1283](https://github.com/rahulnakmol/agentic-ayudha/issues/1283) | Retro eval cases fix their dates in the past, so scores move with the real date | Bug | planned | P2, S, low | [#1170](https://github.com/rahulnakmol/agentic-ayudha/issues/1170) The founder's day (v0.4.0) | none found | The issue says MS1 of #1170 failed its gate on evals run 41594177837 (not checked here). |
| [#1274](https://github.com/rahulnakmol/agentic-ayudha/issues/1274) | Every other human-only verb still starts the device flow in a dry run | Bug | ready | P3, S, low | #1170 (v0.4.0) | none found | Follow-up of Bug #1252 and its PR #1262 (issue Summary). |
| [#1266](https://github.com/rahulnakmol/agentic-ayudha/issues/1266) | The Amp host hand-off lapses by the hand-off's age and records a late or malformed thread ID | Bug | ready | P2, S, medium | [#889](https://github.com/rahulnakmol/agentic-ayudha/issues/889) Agents pull the next story on merge (v0.4.0) | [#1269](https://github.com/rahulnakmol/agentic-ayudha/issues/1269), draft, head `7b8fc9b` | Filed from the review of merged PR #1258 ([ab8ab07](https://github.com/rahulnakmol/agentic-ayudha/commit/ab8ab076)). #1269's RAID log says criterion 2 of #1266 waits for an amendment by @rahulnakmol. |
| [#1264](https://github.com/rahulnakmol/agentic-ayudha/issues/1264) | A delegated build reports a state the ledger did not take when the ledger write fails | Bug | merged (issue still open) | P2, M, medium | #889 (v0.4.0) | [#1278](https://github.com/rahulnakmol/agentic-ayudha/issues/1278), draft, head `ac1daf3` (a follow-up; its body names Bug #1264) | Fix merged: PR [#1272](https://github.com/rahulnakmol/agentic-ayudha/issues/1272), commit [6cf3876](https://github.com/rahulnakmol/agentic-ayudha/commit/6cf3876c), 2026-11-03. |
| [#1252](https://github.com/rahulnakmol/agentic-ayudha/issues/1252) | aayu ready --dry-run for Approve to build starts the device flow and waits for a person | Bug | in-progress | P2, S, low | #1170 (v0.4.0) | none open: [#1262](https://github.com/rahulnakmol/agentic-ayudha/issues/1262) closed unmerged on 2026-11-07 when the mission ended; branch `story/1252-aayu-ready-dry-run-for-approve-to-build` at `ac3769a` kept (`_data/closed-prs.tsv`) | — |
| [#1204](https://github.com/rahulnakmol/agentic-ayudha/issues/1204) | A pull request from the branch outside the open window blocks a merged pull request's own gate run | Bug | ready | P3, XS, low | #889 (v0.4.0) | [#1277](https://github.com/rahulnakmol/agentic-ayudha/issues/1277), draft, head `9c450d7` | #1277's RAID log says criterion 1 of #1204 waits for an amendment by @rahulnakmol. |
| [#1203](https://github.com/rahulnakmol/agentic-ayudha/issues/1203) | A closed pull request whose commit list lost the head can still let its gate run count for a merged pull request | Bug | ready | P2, S, low | #889 (v0.4.0) | none found | The defect is in `hadHead`, which the issue says comes from PR #1183. `hadHead` is not on `dev` at [884d81b](https://github.com/rahulnakmol/agentic-ayudha/commit/884d81b) (`internal/cli/home.go` has `mergedRunOf` only), so it is a defect in open PR #1183. |
| [#1161](https://github.com/rahulnakmol/agentic-ayudha/issues/1161) | A gate run of a pull request that once shared the branch and head can count for a merged pull request | Bug | ready | P2, S, low | #889 (v0.4.0) | [#1183](https://github.com/rahulnakmol/agentic-ayudha/issues/1183), ready for review, head `30c9105`, last updated 2026-10-31 | The code it changes, `mergedRunOf`, came from PR #1160 for Bug #1159 ([4e9bd37](https://github.com/rahulnakmol/agentic-ayudha/commit/4e9bd372)). |

## Stories

| # | Title | Type | Status | Priority, effort, risk | Parent (milestone) | Open PR | Notes |
|---|---|---|---|---|---|---|---|
| [#1112](https://github.com/rahulnakmol/agentic-ayudha/issues/1112) | Each milestone's findings propose the order of the next milestone's stories | Story | planned | P1, M, medium | #889 (v0.4.0) | none | — |
| [#1111](https://github.com/rahulnakmol/agentic-ayudha/issues/1111) | Inspect flags function beyond the story's ask, and each milestone records whether its work will matter | Story | planned | P1, M, medium | #889 (v0.4.0) | none | A body search also matches PR #78 (merged); that is a false match on the number. |
| [#1110](https://github.com/rahulnakmol/agentic-ayudha/issues/1110) | Real-run journeys on the built binary prove each story, and the milestone gate re-runs them at the dev tip | Story | planned | P1, L, high | #889 (v0.4.0) | none | — |
| [#1109](https://github.com/rahulnakmol/agentic-ayudha/issues/1109) | Proof packs record the red run of each test, and a missing red run warns | Story | planned | P1, M, medium | #889 (v0.4.0) | none | — |
| [#1106](https://github.com/rahulnakmol/agentic-ayudha/issues/1106) | A state-changing high-risk story plans its failure modes, and its open questions age like asks | Story | planned | P2, M, low | [#874](https://github.com/rahulnakmol/agentic-ayudha/issues/874) Feature baselines are drafted, approved, and pinned (v0.4.0) | none | Release #1105 says its open questions "age through #883's SLA", so it waits on v0.4.1 Feature #883. |

## Counts

| Type | Total | Merged fix | Open PR | No PR |
|---|---|---|---|---|
| Bug | 8 | 1 (#1264) | 4 (#1266, #1204, #1161, and #1264's follow-up) | 4 (#1283, #1274, #1203, and #1252, whose PR #1262 closed unmerged on 2026-11-07) |
| Story | 5 | 0 | 0 | 5 |

Bug #1264 is in two columns: its fix merged, and a follow-up PR is open. By parent: #889 has 9 (5 Bugs, 4 Stories), #1170 has 3 Bugs, #874 has 1 Story.

## What this means for the regroup

- These 13 items carry v0.4.0 work into v0.4.1 under v0.4.0 parents. A plan for v0.4.1 alone does not see them
  through its Features.
- Four PRs are open on them: #1183 ready for review, #1269 and #1277 draft, plus draft follow-up #1278. PR #1262
  (Bug #1252) closed unmerged on 2026-11-07 when the mission ended; branch
  `story/1252-aayu-ready-dry-run-for-approve-to-build` at `ac3769a` is kept (`_data/closed-prs.tsv`). The Bugs #1204 and #1266 also wait for a person to amend a criterion (RAID logs of #1277 and #1269).
- #1203 and #1161 are one area (`mergedRunOf` in `internal/cli/home.go`). #1203's defect is in #1183's code, so they
  should move together.
