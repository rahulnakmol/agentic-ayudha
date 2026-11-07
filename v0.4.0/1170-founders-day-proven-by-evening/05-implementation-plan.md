# #1170 Implementation plan

Sources: `_data/subs.tsv` (feature 1170), the GraphQL `subIssues` of #1170 (34 items, the same set),
`_data/merge-refs.tsv`, `_data/open-prs.tsv`, `_data/closed-prs.tsv`, `_data/relabel.tsv`, `missions/1170/plan.json`
(graph, milestones, controls), and `gh pr view` and `gh pr checks` read on 2026-11-07. "Depends on" is from the plan
graph and the issues' Links. "Milestone" is the mission milestone in `missions/1170/plan.json`. "Status label" is the
label from before the mission ended on 2026-11-07, because the end changed many labels wrongly (`_data/relabel.tsv`).

## Every Story and Bug

| # | Title (short) | Type | Status label | Effort | Risk | Milestone | Depends on | Layers or paths | Evidence |
|---|---|---|---|---|---|---|---|---|---|
| [#1172](https://github.com/rahulnakmol/agentic-ayudha/issues/1172) | Baseline draft is an inbox card | story | merged | M | medium | MS1 | — | L02 | PR [#1219](https://github.com/rahulnakmol/agentic-ayudha/issues/1219), [c858a80](https://github.com/rahulnakmol/agentic-ayudha/commit/c858a80) |
| [#1173](https://github.com/rahulnakmol/agentic-ayudha/issues/1173) | Approve to build is an inbox card | story | merged | M | medium | MS1 | #1172 | L02 | PR [#1225](https://github.com/rahulnakmol/agentic-ayudha/issues/1225), [eb51d1b](https://github.com/rahulnakmol/agentic-ayudha/commit/eb51d1b) |
| [#1174](https://github.com/rahulnakmol/agentic-ayudha/issues/1174) | Bugs come last in BUGS | story | merged | M | medium | MS2 | — | L02 | PR [#1216](https://github.com/rahulnakmol/agentic-ayudha/issues/1216), [322e2ac](https://github.com/rahulnakmol/agentic-ayudha/commit/322e2ac) |
| [#1175](https://github.com/rahulnakmol/agentic-ayudha/issues/1175) | The morning is set | story | merged | S | low | MS2 | #1172, #1173 | L02 | PR [#1227](https://github.com/rahulnakmol/agentic-ayudha/issues/1227), [2715ef2](https://github.com/rahulnakmol/agentic-ayudha/commit/2715ef2); test fix PR [#1235](https://github.com/rahulnakmol/agentic-ayudha/issues/1235), [4faab61](https://github.com/rahulnakmol/agentic-ayudha/commit/4faab61) |
| [#1176](https://github.com/rahulnakmol/agentic-ayudha/issues/1176) | The evening review | story | merged | M | medium | MS2 | — | L02 | PR [#1213](https://github.com/rahulnakmol/agentic-ayudha/issues/1213), [188bbb0](https://github.com/rahulnakmol/agentic-ayudha/commit/188bbb0) |
| [#1177](https://github.com/rahulnakmol/agentic-ayudha/issues/1177) | The day is proven | story | merged | S | low | MS2 | #1176 | L02 | PR [#1237](https://github.com/rahulnakmol/agentic-ayudha/issues/1237), [66397d3](https://github.com/rahulnakmol/agentic-ayudha/commit/66397d3) |
| [#1178](https://github.com/rahulnakmol/agentic-ayudha/issues/1178) | The overnight card | story | in-progress (merged since) | M | medium | MS2 | #1176 | L02 (`internal/cli/inboxovernight.go`, `missionnext.go`, `missiontick.go`) | PR [#1238](https://github.com/rahulnakmol/agentic-ayudha/issues/1238), [bc78349](https://github.com/rahulnakmol/agentic-ayudha/commit/bc78349) (2026-11-07) |
| [#1179](https://github.com/rahulnakmol/agentic-ayudha/issues/1179) | grill edits a filed body | story | merged | M | medium | MS1 | — | L02, L03 | PR [#1217](https://github.com/rahulnakmol/agentic-ayudha/issues/1217), [197ec9d](https://github.com/rahulnakmol/agentic-ayudha/commit/197ec9d) |
| [#1182](https://github.com/rahulnakmol/agentic-ayudha/issues/1182) | Orb Terminal gets the device code | bug | merged | S | high | MS1 | — | L02 | PR [#1188](https://github.com/rahulnakmol/agentic-ayudha/issues/1188), [c64bccf](https://github.com/rahulnakmol/agentic-ayudha/commit/c64bccf) |
| [#1191](https://github.com/rahulnakmol/agentic-ayudha/issues/1191) | Loop agents carry `AAYU_AGENT=1` | bug | ready | S | high | BUGS | #1182 | L02, L03 (`internal/cli/env.go`, `presence.go`, `internal/agentenv/detect.go`) | PR [#1199](https://github.com/rahulnakmol/agentic-ayudha/issues/1199) closed unmerged on 2026-11-07 when the mission ended; branch `bug-1191-loop-lane` at `edb25bc` kept |
| [#1192](https://github.com/rahulnakmol/agentic-ayudha/issues/1192) | ship merges on `inspect --approve` | bug | in-progress | S | high | BUGS | — | L02 (`internal/cli/ship.go`, `shipproof.go`, `shippass.go`) | PR [#1220](https://github.com/rahulnakmol/agentic-ayudha/issues/1220) closed unmerged on 2026-11-07 when the mission ended; branch `story/1192-ship-asks-for-a-second-code-after-a-pers` at `0fdcfd7` kept |
| [#1193](https://github.com/rahulnakmol/agentic-ayudha/issues/1193) | Mission start needs the person's tool | bug | merged | S | low | BUGS | — | L07, L02 | PR [#1194](https://github.com/rahulnakmol/agentic-ayudha/issues/1194), [26537bd](https://github.com/rahulnakmol/agentic-ayudha/commit/26537bd) |
| [#1201](https://github.com/rahulnakmol/agentic-ayudha/issues/1201) | Superseded Decision chain stays open | bug | in-progress | S | low | BUGS | — | L03, L02 (`internal/decide/decide.go`, the inbox Decision card; from the issue Summary) | not started: no PR found, no `delegate/1201/state.json`; claimed at the mission start (2026-10-31T14:24:53Z) and released (`missions/1170/claims.jsonl`, lines 3 and 7) |
| [#1206](https://github.com/rahulnakmol/agentic-ayudha/issues/1206) | Device code only after read-only checks | bug | in-progress | S | high | BUGS | — | L02 (`internal/cli/ship.go`) | PR [#1214](https://github.com/rahulnakmol/agentic-ayudha/issues/1214) closed unmerged on 2026-11-07 when the mission ended; branch `story/1206-ship-asks-for-the-device-code-before-che` at `b9aad80` kept |
| [#1207](https://github.com/rahulnakmol/agentic-ayudha/issues/1207) | Keep a story on its outside PR | bug | merged | S | medium | BUGS | — | L02 | PR [#1224](https://github.com/rahulnakmol/agentic-ayudha/issues/1224), [a17d469](https://github.com/rahulnakmol/agentic-ayudha/commit/a17d469) |
| [#1208](https://github.com/rahulnakmol/agentic-ayudha/issues/1208) | Amp mission with no CLI sign-in | bug | merged | M | medium | BUGS | — | L02, L03 | PR [#1209](https://github.com/rahulnakmol/agentic-ayudha/issues/1209), [932851d](https://github.com/rahulnakmol/agentic-ayudha/commit/932851d) |
| [#1210](https://github.com/rahulnakmol/agentic-ayudha/issues/1210) | Directory paths and layers in slice | bug | merged | S | low | BUGS | — | L03 | PR [#1215](https://github.com/rahulnakmol/agentic-ayudha/issues/1215), [bed5524](https://github.com/rahulnakmol/agentic-ayudha/commit/bed5524) |
| [#1211](https://github.com/rahulnakmol/agentic-ayudha/issues/1211) | Stale amp environment record | bug | merged | M | medium | BUGS | — | L02, L03 | PR [#1218](https://github.com/rahulnakmol/agentic-ayudha/issues/1218), [7d3ec79](https://github.com/rahulnakmol/agentic-ayudha/commit/7d3ec79) |
| [#1226](https://github.com/rahulnakmol/agentic-ayudha/issues/1226) | pnpm-installed Playwright | bug | merged | S | low | BUGS | — | L04, L02 | PR [#1228](https://github.com/rahulnakmol/agentic-ayudha/issues/1228), [e39b5cb](https://github.com/rahulnakmol/agentic-ayudha/commit/e39b5cb) |
| [#1230](https://github.com/rahulnakmol/agentic-ayudha/issues/1230) | Gate fix records every check | bug | merged | S | low | BUGS | — | L02 | PR [#1232](https://github.com/rahulnakmol/agentic-ayudha/issues/1232), [0c4abee](https://github.com/rahulnakmol/agentic-ayudha/commit/0c4abee) |
| [#1234](https://github.com/rahulnakmol/agentic-ayudha/issues/1234) | ship merges only on the dev tip | bug | in-progress | S | medium | BUGS | — | L02 (`internal/cli/ship.go`, `mission.go`, `missionnext.go`, `missionstop.go`) | PR [#1236](https://github.com/rahulnakmol/agentic-ayudha/issues/1236) closed unmerged on 2026-11-07 when the mission ended; branch `story/1234-fix-ship-ship-merges-a-pr-proven-on-an-o` at `cc60811` kept |
| [#1248](https://github.com/rahulnakmol/agentic-ayudha/issues/1248) | Extend in a window before the limit | bug | in-progress | S | low | BUGS | — | L02, L07, L04 (`internal/cli/missiontime.go`, `internal/config/defaults.toml`, `CONSTRAINTS.md`) | PR [#1249](https://github.com/rahulnakmol/agentic-ayudha/issues/1249) closed unmerged on 2026-11-07 when the mission ended; branch `story/1248-fix-mission-extend-refuses-before-the-ti` at `ec32cdc` kept |
| [#1250](https://github.com/rahulnakmol/agentic-ayudha/issues/1250) | Draft-first CI | story | in-progress | S | medium | MS2 | — | L09, L02, L03, L06 (`.github/workflows/ci.yml`, `internal/cli/ship.go`, `internal/delegate/brief.go`, `internal/github/fake/fake.go`) | PR [#1261](https://github.com/rahulnakmol/agentic-ayudha/issues/1261) closed unmerged on 2026-11-07 when the mission ended; branch `story/1250-builders-iterate-on-a-draft-pr-locally-a` at `4fb58ed` kept |
| [#1251](https://github.com/rahulnakmol/agentic-ayudha/issues/1251) | No `--ack` in ready's refusals | bug | ready | XS | low | BUGS | — | L02 (`internal/cli/readycover.go`, `readyapprove.go`) | PR [#1259](https://github.com/rahulnakmol/agentic-ayudha/issues/1259) closed unmerged on 2026-11-07 when the mission ended; branch `bug-1251-ready-ack` at `2e896ac` kept; skill text in PR [#1273](https://github.com/rahulnakmol/agentic-ayudha/issues/1273), head `8193f2f` (no `Refs`), still open |
| [#1252](https://github.com/rahulnakmol/agentic-ayudha/issues/1252) | Dry run of Approve to build asks no code | bug | in-progress | S | low | BUGS | — | L02 (`internal/cli/readyapprove.go`, `mission.go`, `missiongate.go`, `missiontime.go`) | PR [#1262](https://github.com/rahulnakmol/agentic-ayudha/issues/1262) closed unmerged on 2026-11-07 when the mission ended; branch `story/1252-aayu-ready-dry-run-for-approve-to-build` at `ac3769a` kept |
| [#1253](https://github.com/rahulnakmol/agentic-ayudha/issues/1253) | One inbox card per Feature | bug | in-progress | M | medium | BUGS | — | L02 (`internal/cli/inbox.go`, `inboxbaseline.go`, `inboxapprove.go`, `readyapprove.go`) | PR [#1265](https://github.com/rahulnakmol/agentic-ayudha/issues/1265) closed unmerged on 2026-11-07 when the mission ended; branch `story/1253-aayu-inbox-asks-up-to-five-decisions-to` at `306d1a8` kept |
| [#1254](https://github.com/rahulnakmol/agentic-ayudha/issues/1254) | Two person windows a day | story | in-progress | M | medium | MS2 | — | L02, L03, L04, L06, L07, L09 (65 files, see PR) | draft PR [#1270](https://github.com/rahulnakmol/agentic-ayudha/issues/1270) closed unmerged on 2026-11-07 when the mission ended; branch `story/1254-two-person-windows-a-day-outside-them-no` at `7364899` kept |
| [#1256](https://github.com/rahulnakmol/agentic-ayudha/issues/1256) | Goldfish review in the build brief | bug | ready | S | low | BUGS | #1250 | L03 (`internal/delegate/brief.go`), L02 (`internal/cli/inspect.go` peer pick) | not started: no PR found, no delegation |
| [#1267](https://github.com/rahulnakmol/agentic-ayudha/issues/1267) | Approve again keeps a passed gate | bug | ready | S | medium | BUGS | — | L02 (`internal/cli/mission.go`) | PR [#1276](https://github.com/rahulnakmol/agentic-ayudha/issues/1276) closed unmerged on 2026-11-07 when the mission ended; branch `fix/approve-keeps-gate` at `bcafd0f` kept |
| [#1268](https://github.com/rahulnakmol/agentic-ayudha/issues/1268) | Gate failures count from the last pass | bug | ready | S | medium | BUGS | — | L02 (`internal/cli/missiongate.go`, `missionask.go`) | PR [#1275](https://github.com/rahulnakmol/agentic-ayudha/issues/1275) closed unmerged on 2026-11-07 when the mission ended; branch `fix/gate-count-since-pass` at `9e8b72b` kept |
| [#1274](https://github.com/rahulnakmol/agentic-ayudha/issues/1274) | Every human-only verb has a dry run | bug | ready | S | low | BUGS | — (builds on #1252, issue Summary) | L02 (`presenceCheck` callers: `missionstop.go`, `missionrecover.go`, `missiongate.go`, `ready.go`, `ready_early.go`, `scope.go`, `inspect.go`) | not started: no PR found |
| [#1280](https://github.com/rahulnakmol/agentic-ayudha/issues/1280) | Resume moves the events cursor | bug | planned | S | low | BUGS | — | L02 (`internal/cli/missionstop.go`, `missionnext.go`, `missionwake.go`; from the issue Summary) | not started: no PR found |
| [#1281](https://github.com/rahulnakmol/agentic-ayudha/issues/1281) | Every agent PR merges on proof | story | planned | S | high | not in the plan | — | L07, L09, L04, L02 (`.aayudha/config.toml`, `.github/CODEOWNERS`, `internal/config/`, `internal/cli/shipproof.go`) | draft PR [#1282](https://github.com/rahulnakmol/agentic-ayudha/issues/1282) closed unmerged on 2026-11-07 when the mission ended; branch `feat/1281-auto-merge-on-proof` at `6f2aae7` kept |
| [#1283](https://github.com/rahulnakmol/agentic-ayudha/issues/1283) | Retro eval cases fix their dates | bug | planned | S | low | BUGS | — | L05 (`evals/cases/retro/**`), `internal/evals` harness (from the issue Summary) | not started: no PR found |

## Delivered

What works on `dev` `bc78349` now (each commit checked with `git merge-base --is-ancestor`):

- The morning (MS1 and part of MS2): Baseline cards ([c858a80](https://github.com/rahulnakmol/agentic-ayudha/commit/c858a80)),
  the Approve to build card in the same session ([eb51d1b](https://github.com/rahulnakmol/agentic-ayudha/commit/eb51d1b)),
  and the decision order with "The day is set" ([2715ef2](https://github.com/rahulnakmol/agentic-ayudha/commit/2715ef2)).
- The evening: Evening review cards with evidence and accept or comment
  ([188bbb0](https://github.com/rahulnakmol/agentic-ayudha/commit/188bbb0)) and "The day is proven"
  ([66397d3](https://github.com/rahulnakmol/agentic-ayudha/commit/66397d3)).
- Bugs last: the BUGS milestone and a proof pack per Bug at its gate
  ([322e2ac](https://github.com/rahulnakmol/agentic-ayudha/commit/322e2ac)).
- `aayu grill --edit` ([197ec9d](https://github.com/rahulnakmol/agentic-ayudha/commit/197ec9d)).
- The overnight card ([bc78349](https://github.com/rahulnakmol/agentic-ayudha/commit/bc78349), PR #1238, merged on
  proof at 2026-11-07T12:34:51Z): the Overnight inbox card (`internal/cli/inbox.go` `InboxOvernight`), its record
  `missions/overnight/<date>.json` (`internal/cli/inboxovernight.go` `OvernightPath`, `overnightAct`), the hourly
  tick that records each run's start and stop (`tickOvernight`, `overnightStep`), and `aayu mission next` that says
  `outside the day: starts at <time>` (`internal/cli/missionnext.go`). No worker carries out a run yet: see the
  accepted follow-ups under "Pending".
- Eight Bug fixes that unblocked the mission itself: #1182, #1193, #1207, #1208, #1210, #1211, #1226, #1230 (see the
  table).
- Four MS1 gate fixes in `skills/retro/SKILL.md`, each `Refs #1170`:
  [2bd966e](https://github.com/rahulnakmol/agentic-ayudha/commit/2bd966e) (#1229),
  [b83e187](https://github.com/rahulnakmol/agentic-ayudha/commit/b83e187) (#1233),
  [fb412a5](https://github.com/rahulnakmol/agentic-ayudha/commit/fb412a5) (#1271),
  [884d81b](https://github.com/rahulnakmol/agentic-ayudha/commit/884d81b) (#1284). They are listed in
  `missions/1170/plan.json` `gate_fixes`.

Mission facts (`missions/1170/plan.json`, `gates.jsonl`, and #1170 comments):

- Started 2026-10-31T14:24:15Z by @rahulnakmol, tool `amp`, 10 agents.
- MS1 gate: 6 runs. Failed on `eb51d1b`, `0c4abee`, `ab8ab07`, and `0cd53b4` (retro evals; on `0cd53b4` also
  Playwright `pending`). Passed on `b83e187` (2026-11-02T04:05:05Z) and on `884d81b` (2026-11-03T16:52:25Z). The
  passed state on `b83e187` did not hold. Approval version 3 at 2026-11-02T17:45:16Z ("Approve to build again", with
  the same MS1 stories) set MS1's gate back to `waits`, and `aayu ship 1261` then refused with "#1250 is in MS2: it
  merges after MS1's gate passes" (#1267 Summary). That is Bug #1267, and it cost three more gate runs.
- The person ran `gate once more` twice (2026-11-03T10:51:07Z and 16:52:38Z) and raised the budget to 2500 USD
  (2026-11-03T05:53:50Z).
- Time: one extend of 48 h on 2026-11-02T15:18:52Z. The #1170 comment `aayudha:mission-time` (`hours=96`, last edited
  2026-11-05T04:34:30Z) says: "#1170 reached its 96 h time limit. new starts wait … Running stories keep going."
- The plan's `events_cursor` is 2026-11-05T09:46:51Z, the last time the mission read its events.
- Ended 2026-11-07T12:35:56Z by @rahulnakmol, reason "founders regroup: v0.4 paused" (control `end`; plan `state`
  `ended (founders regroup: v0.4 paused)`). MS2 and BUGS still say `waits` in the plan. The end's help text says "each
  open story and Bug loses its claims and goes back to status:ready, and its open PRs close"
  (`internal/cli/missionstop.go`, the stop effect).

## Closed unmerged

Nothing of #1170 is in review now. On 2026-11-07, between 12:38:55Z and 12:39:20Z, the end of the mission closed the
13 PRs below unmerged (`gh pr view <n> --json closedAt`). Each branch is still on `origin` at the head below
(`_data/closed-prs.tsv`; `git ls-remote origin`). To finish each one, rebuild it or reopen it from the kept branch,
then rebase it onto `dev` `bc78349` or later. PR [#1273](https://github.com/rahulnakmol/agentic-ayudha/issues/1273)
(the #1251 skill text, no `Refs`) was not closed and is still open.

The other columns are the state before the close, read on 2026-11-07. The required checks are `build`,
`generate-drift`, `lint`, `commits`, `security`, `docs`, `release-snapshot`, and `protected-paths` (AGENTS.md). "ci
green" means each of the first seven passed or was skipped by its path filter. "Behind" is commits behind `dev`
`884d81b` (GitHub compare API); `dev` has one more commit now (`bc78349`). The last `aayudha:inspect` disposition is
from the PR's inspect comment. A new head needs its checks and inspect again.

| PR | Issue | Kept branch, head | Draft | Behind | Checks | Inspect | What blocked it |
|---|---|---|---|---|---|---|---|
| [#1261](https://github.com/rahulnakmol/agentic-ayudha/issues/1261) | #1250 (MS2) | `story/1250-builders-iterate-on-a-draft-pr-locally-a`, `4fb58ed` | no | 5 | ci green; `protected-paths` fail (`.github/workflows/ci.yml`); merge-gate, pack, inspect pass | merge (2026-11-02T21:13:51Z) | (1) No plan can pass: the PR touches never layer L09, so ship needs a plan-time review, and a story plan with an L09 checkpoint is outside #1170's design (L02, L03) (`internal/planning/design.go` `CheckInDesign`). (2) A person's ship: `ci.yml` is protected and names the `live-tests` environment (PR body). (3) Rebase onto `dev`. `delegate/1250/state.json` says `merge-ready` (inspect merge on `4fb58ed`, last seen 2026-11-05). |
| [#1270](https://github.com/rahulnakmol/agentic-ayudha/issues/1270) | #1254 (MS2) | `story/1254-two-person-windows-a-day-outside-them-no`, `7364899` | yes | 3 | ci green; `protected-paths` fail; merge-gate, pack, inspect skipped (draft) | none on this head | (1) A person's call on Goldfish P1 findings 3.7 and 3.11 (recommended: accept as follow-up Bugs). (2) The PR touches never layers L04, L07, L09, so the same plan block as #1261 applies (inferred). (3) Protected paths and the narrow set need a person's ship. (4) After merge, a person runs `aayu install --loops --host amp --apply` (PR RAID). (5) Mark ready, then CI and review. |
| [#1282](https://github.com/rahulnakmol/agentic-ayudha/issues/1282) | #1281 (no milestone) | `feat/1281-auto-merge-on-proof`, `6f2aae7` | yes | 1 | ci green; `protected-paths` fail (CODEOWNERS, `.aayudha/config.toml`); merge-gate, pack, inspect skipped (draft) | none on this head | (1) #1281 has no Ready snapshot; it is not in `specs/1170/approved.json`, and a Story is Ready only under a Feature approved to build (PR body cites Decision #861). The person picks: refile as a Bug under #1170, a new Feature, or #1170 approved again with #1281. (2) The merge gate runs dev's `aayu`, which does not know `[autonomy] cross_family`, so verify fails on this config. (3) A person's ship (CODEOWNERS and `[autonomy]`). |
| [#1199](https://github.com/rahulnakmol/agentic-ayudha/issues/1199) | #1191 (BUGS) | `bug-1191-loop-lane`, `edb25bc` | no | 26 | ci green (the failing `build` is the `pages` workflow, run 41198237997); `protected-paths` pass; `aayudha/inspect` fail; `aayudha/inspect-reply` fail | fix (2026-10-31T12:20:01Z) | Inspect says `fix` at this head. A person must look at an authority change (Decision #1027) and accept or reject the reduced criterion 4; the PR asks for `--solo-bypass`. Rebase (26 behind). |
| [#1220](https://github.com/rahulnakmol/agentic-ayudha/issues/1220) | #1192 (BUGS) | `story/1192-ship-asks-for-a-second-code-after-a-pers`, `0fdcfd7` | no | 21 | ci green; `protected-paths` pass; merge-gate, pack, inspect pass; `aayudha/inspect-reply` fail | merge (2026-11-01T11:18:25Z) | Narrow set (`ship.go`, `shipproof.go`): a person's look and ship. Three CodeRabbit Major findings are left for a person (PR RAID). Rebase. |
| [#1214](https://github.com/rahulnakmol/agentic-ayudha/issues/1214) | #1206 (BUGS) | `story/1206-ship-asks-for-the-device-code-before-che`, `b9aad80` | no | 1 | ci green (`docs` skipped); `protected-paths` pass; merge-gate, pack, inspect pass | merge (2026-11-03T07:43:11Z) | Narrow set (`ship.go`): a person's ship (Decision #1255 lists it as waiting only for that). Rebase. |
| [#1236](https://github.com/rahulnakmol/agentic-ayudha/issues/1236) | #1234 (BUGS) | `story/1234-fix-ship-ship-merges-a-pr-proven-on-an-o`, `cc60811` | no | 3 | ci green; `protected-paths` pass; merge-gate, pack, inspect pass; `aayudha/inspect-reply` fail | merge (2026-11-03T07:01:13Z) | Narrow set (`ship.go`): ship posted "Ready to ship" at an older head `2425f54` (2026-11-02); a person's ship. Rebase. |
| [#1249](https://github.com/rahulnakmol/agentic-ayudha/issues/1249) | #1248 (BUGS) | `story/1248-fix-mission-extend-refuses-before-the-ti`, `ec32cdc` | no | 5 | ci green; `protected-paths` fail (`internal/config/defaults.toml`, `CONSTRAINTS.md`, `skills/mission/SKILL.md`); merge-gate, pack, inspect pass | merge (2026-11-02T20:14:12Z) | Protected paths and never layers L04, L07: a person's plan review and ship. Two design questions for @rahulnakmol (a new code per inbox extend; who computes the mission clock). Rebase. |
| [#1259](https://github.com/rahulnakmol/agentic-ayudha/issues/1259) | #1251 (BUGS) | `bug-1251-ready-ack`, `2e896ac` | no | 5 | ci green (`docs` skipped); `protected-paths` pass; merge-gate, pack, inspect pass | merge (2026-11-02T21:15:10Z) | Rebase. |
| [#1273](https://github.com/rahulnakmol/agentic-ayudha/issues/1273) (still open) | #1251 (no `Refs`) | open, `8193f2f` | no | 4 | ci green; `protected-paths` fail (`skills/`); merge-gate, inspect pass | merge (2026-11-02T21:53:20Z) | Protected path `skills/`: the founder's `--solo-bypass`. The PR says to ship #1259 first or in the same window. |
| [#1262](https://github.com/rahulnakmol/agentic-ayudha/issues/1262) | #1252 (BUGS) | `story/1252-aayu-ready-dry-run-for-approve-to-build`, `ac3769a` | no | 4 | ci green (`docs` skipped); `protected-paths` pass; merge-gate, pack, inspect pass | merge (2026-11-02T22:21:20Z) | Rebase. |
| [#1265](https://github.com/rahulnakmol/agentic-ayudha/issues/1265) | #1253 (BUGS) | `story/1253-aayu-inbox-asks-up-to-five-decisions-to`, `306d1a8` | no | 4 | ci green (`docs` skipped); `protected-paths` pass; merge-gate, pack, inspect pass | merge (2026-11-03T00:31:59Z) | Rebase. |
| [#1276](https://github.com/rahulnakmol/agentic-ayudha/issues/1276) | #1267 (BUGS) | `fix/approve-keeps-gate`, `bcafd0f` | no | 4 | ci green (`docs` skipped); `protected-paths` pass; merge-gate, pack, inspect pass | merge, human review by @rahulnakmol (`inspect --approve` at `bcafd0f`, 2026-11-03T18:02:34Z) | Rebase (a new head needs a new review). |
| [#1275](https://github.com/rahulnakmol/agentic-ayudha/issues/1275) | #1268 (BUGS) | `fix/gate-count-since-pass`, `9e8b72b` | no | 4 | ci green (`docs` skipped); `protected-paths` pass; merge-gate, pack, inspect pass | merge (2026-11-03T00:23:07Z) | Rebase. |

Before the end, ship held every BUGS PR above until MS2's gate passed (`internal/cli/missionstop.go`, `missionHold`:
`#<n> is in BUGS: it merges after MS2's gate passes`). An ended mission holds nothing: `missionHold` returns early
for a plan whose `state` is set ("a mission that has not started, or has ended, holds nothing"). So the hold is gone
now. If a person approves #1170 to build again and starts the mission with the same milestones, the hold comes back,
because MS2's gate still says `waits` (`missions/1170/plan.json`; inferred from the same code). MS2's gate runs only
when its last story merged (`internal/cli/missiongate.go`, header comment), so it would wait for #1250 and #1254.
Once #1236 (#1234) merges, ship refuses any PR whose head does not contain the dev tip.

## Pending

Not built, with no PR:

| Item | What is missing | Estimate in a source |
|---|---|---|
| Bug #1201 | the superseded-chain closer and its housekeeping card | effort S (label) |
| Bug #1256 | the Goldfish step in the build brief; depends on #1250 | effort S |
| Bug #1274 | a dry run for every other human-only verb, and a test over every `presenceCheck` caller | effort S |
| Bug #1280 | `mission resume` must keep the events cursor | effort S |
| Bug #1283 | retro eval cases with fixed dates | effort S |

The mission ended, so it starts none of these: `aayu mission next` refuses an ended mission
(`internal/cli/missionnext.go`, "The mission of #%d %s. Nothing starts."). A person must approve #1170 to build again
and start the mission again, after the repository halt is lifted (Actions variable `AAYUDHA_HALT`, set
2026-11-07T12:50:50Z). #1280 and #1283 had `status:planned` before the end, so they also need Ready first.

Accepted follow-ups of PR #1238. Before the merge, @rahulnakmol accepted these four findings as follow-ups. They are
pending work. No issue holds them yet:

| Follow-up | What is missing | Source |
|---|---|---|
| Goldfish F1 (P1) | No worker carries out an overnight run, cancels it, or confirms it. The tick only records start and stop (`internal/cli/inboxovernight.go` `tickOvernight`). A fix needs `loops/build-ready.toml` (in `[autonomy] always_person`) and a runner | PR #1238 RAID log, "Goldfish 1" |
| Goldfish F2 (P1) | A run does not stop itself at its bound. The bounds are checked only on the hourly tick; a stop records the time its bound was reached | PR #1238 RAID log, "Goldfish 2" (partly fixed) |
| Goldfish F4 (P1) | Spend is read only from this clone's local actions log, not from the host that spends (`internal/cli/inboxovernight.go` `overnightSpend`; `build.go` `buildCost`) | PR #1238 RAID log, "Goldfish 4" |
| CodeRabbit night retry (Major) | `dueRetries` drops a retry that comes due outside the day (`if !day.Within(now)`). A later Kanha event moves `told` past `RetryAt`, so after the day opens no recovery wake follows, and the story stays `failed` until a person runs `aayu mission recover` or `aayu mission retry` | CodeRabbit review comment on `internal/cli/missiontick.go` line 327, PR #1238, 2026-11-03T18:37:50Z; the code is on `dev` (`missiontick.go` `dueRetries`) |

No source gives an estimate for them.

Work that no Story or Bug holds yet (not sliced):

| Gap | Source | Estimate |
|---|---|---|
| A Story or Bug for each accepted #1238 follow-up above (F1, F2, F4, the night retry) | PR #1238 RAID ("A follow-up story does that"); inspect nudge "add the worker … Owner: @rahulnakmol. Date: 2026-11-17" | none in a source |
| Two follow-up Bugs from #1254: a lapsed claim has no retry inside the window (3.7); a bot-only session cannot write the outbox (3.11) | PR #1270 RAID ("Recommended: accept as a follow-up Bug"); no person ruled on them before the PR closed (the evidence does not show a ruling) | none |
| A tech baseline v2 (or a new design) for #1170 that covers L04, L06, L07, and L09, so #1250, #1254, and #1281 can get plans | `specs/1170/design.json` `layers`; `internal/planning/design.go` `CheckInDesign` | none |
| Ready and an approval for #1281 (spec v4 and Approve to build again, or a refile) | PR #1282 RAID | none |
| Loops set up again for the new `[time] tz` after #1254, and a `loops/active.json` PR | PR #1270 RAID | none |
| The evening "comment becomes a story" step | #1170 Scope names [#1014](https://github.com/rahulnakmol/agentic-ayudha/issues/1014) (OPEN, `status:planned`, another Feature) | none |
| The Feature's `depends-on` [#1029](https://github.com/rahulnakmol/agentic-ayudha/issues/1029) and [#1030](https://github.com/rahulnakmol/agentic-ayudha/issues/1030) | `specs/1170/ready.md` Links; both OPEN, `status:planned` | none |

A search for issues created on or after 2026-11-02 that name "overnight" or "window" found only #1255 and #1281, so
the evidence does not show these follow-ups filed. The four #1238 follow-ups are not filed as issues.
