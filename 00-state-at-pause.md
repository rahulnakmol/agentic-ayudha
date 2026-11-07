# State at the pause (2026-11-07)

The founders paused v0.4.0 and v0.4.1 on 2026-11-07, because the direction of aayu changed. This page records what
was stopped, how, and what was left, so the work can restart from a known state. Times are UTC.

## What was stopped

| Time | Step | Who | Evidence |
|---|---|---|---|
| 11:37 | The founder's window schedule (10:00, 10:30, 18:00, 18:30 IST) of the #1170 Kanha thread paused | agent | Amp schedule 93920470 |
| 12:21 | All 12 scheduled loop runs paused (alert-triage, ci-failed, decisions, error-budget-review, evolve, housekeeping, patch-sweep, release-measure, retro, seat-review, triage-daily, verify-maintain) | agent | `loops/active.json` names their Amp threads |
| about 12:25 | All 160 Amp GitHub subscriptions for the repository disabled, including the `triage-opened` spawner (5d10ad31) and every builder's PR-comment subscription | agent | Amp subscriptions |
| 12:34 | PR [#1238](https://github.com/rahulnakmol/agentic-ayudha/pull/1238) (Story #1178) merged on proof as [bc78349](https://github.com/rahulnakmol/agentic-ayudha/commit/bc78349a4910ab24fc04411b1b821fac172d633b) | agent, `aayu ship 1238` | ship record `proof/1178/e22c1ba.../authoritative/ship.json` on the ledger |
| about 12:45 | Mission of [#1170](https://github.com/rahulnakmol/agentic-ayudha/issues/1170) ended: `aayu mission stop 1170 --end --reason "founders regroup: v0.4 paused"` | @rahulnakmol (device flow) | ledger `missions/1170/plan.json` (e5ad0be): `ended (founders regroup: v0.4 paused)` |
| 12:50:51 | Repository halted: `aayu halt --reason "founders regroup: v0.4.0 and v0.4.1 paused"` | @rahulnakmol | Actions variable `AAYUDHA_HALT`; every workflow and verb refuses until `aayu resume` |

The founder accepted four review findings on #1238 as follow-ups, not filed as issues during the pause: F1 (no worker
carries out, cancels, or confirms an overnight run), F2 (a run does not stop itself at its bound), F4 (spend is read
from the local actions log of one clone) and CodeRabbit's night-retry bug (`dueRetries` in
`internal/cli/missiontick.go`). The #1170 folder lists them as pending.

## PRs the mission end closed

Ending the mission closed every open PR of its stories, unmerged. Each branch is still on origin (checked with
`git ls-remote` on 2026-11-07). To restart one, reopen it or rebuild from its branch, rebase onto dev, and pass the
gate again.

| PR | Story or Bug | Branch | Head | State before the end |
|---|---|---|---|---|
| [#1199](https://github.com/rahulnakmol/agentic-ayudha/pull/1199) | #1191 | `bug-1191-loop-lane` | edb25bc | ready; inspect `fix` |
| [#1214](https://github.com/rahulnakmol/agentic-ayudha/pull/1214) | #1206 | `story/1206-ship-asks-for-the-device-code-before-che` | b9aad80 | proven, held for MS2 |
| [#1220](https://github.com/rahulnakmol/agentic-ayudha/pull/1220) | #1192 | `story/1192-ship-asks-for-a-second-code-after-a-pers` | 0fdcfd7 | proven, held for MS2 |
| [#1236](https://github.com/rahulnakmol/agentic-ayudha/pull/1236) | #1234 | `story/1234-fix-ship-ship-merges-a-pr-proven-on-an-o` | cc60811 | ship to re-check; held for MS2 |
| [#1249](https://github.com/rahulnakmol/agentic-ayudha/pull/1249) | #1248 | `story/1248-fix-mission-extend-refuses-before-the-ti` | ec32cdc | inspect merge; protected path; held for MS2 |
| [#1259](https://github.com/rahulnakmol/agentic-ayudha/pull/1259) | #1251 | `bug-1251-ready-ack` | 2e896ac | proven, held for MS2 |
| [#1261](https://github.com/rahulnakmol/agentic-ayudha/pull/1261) | #1250 | `story/1250-builders-iterate-on-a-draft-pr-locally-a` | 4fb58ed | no plan possible (L09 outside #1170's design) |
| [#1262](https://github.com/rahulnakmol/agentic-ayudha/pull/1262) | #1252 | `story/1252-aayu-ready-dry-run-for-approve-to-build` | ac3769a | proven, held for MS2 |
| [#1265](https://github.com/rahulnakmol/agentic-ayudha/pull/1265) | #1253 | `story/1253-aayu-inbox-asks-up-to-five-decisions-to` | 306d1a8 | proven, held for MS2; 3 calls for the founder |
| [#1270](https://github.com/rahulnakmol/agentic-ayudha/pull/1270) | #1254 | `story/1254-two-person-windows-a-day-outside-them-no` | 7364899 | draft; Goldfish P1s 3.7 and 3.11 open |
| [#1275](https://github.com/rahulnakmol/agentic-ayudha/pull/1275) | #1268 | `fix/gate-count-since-pass` | 9e8b72b | proven, held for MS2 |
| [#1276](https://github.com/rahulnakmol/agentic-ayudha/pull/1276) | #1267 | `fix/approve-keeps-gate` | bcafd0f | approved by the founder, held for MS2 |
| [#1282](https://github.com/rahulnakmol/agentic-ayudha/pull/1282) | #1281 | `feat/1281-auto-merge-on-proof` | 6f2aae7 | draft; 3 P1s for the founder; no Ready snapshot |

Source: `_data/closed-prs.tsv`, and the builders' reports.

## Labels the mission end changed wrongly

`aayu mission stop --end` said "merged stories stay merged", but it set 16 merged Stories and Bugs of #1170 to
`status:ready` or to no status label, and it removed `status:planned` from #1280, #1281 and #1283
(`_data/relabel.tsv`: issue, label before, label after). The docs use the labels from before the end. A restore
script for the founder sets them back: [`_tools/restore-labels-after-end.sh`](_tools/restore-labels-after-end.sh) on this
branch. Run it in a Terminal with `gh` signed in as a person; it shows the plan first and asks y/N. This is a defect in `aayu mission stop --end`; it is not filed, because no issues
are filed during the pause.

## PRs still open

These were not part of the #1170 mission and are untouched. Source: `gh pr list --base dev --state open` on
2026-11-07.

| PR | State | Refs | Title |
|---|---|---|---|
| [#1278](https://github.com/rahulnakmol/agentic-ayudha/pull/1278) | draft | - | fix(delegate): say the ledger does not confirm a record when it cannot |
| [#1277](https://github.com/rahulnakmol/agentic-ayudha/pull/1277) | draft | #1204 | fix(proof): a PR from the branch opened after the merge holds no run |
| [#1273](https://github.com/rahulnakmol/agentic-ayudha/pull/1273) | ready | - | docs(ready): name the spec, inbox, and ready steps for a story that waits |
| [#1269](https://github.com/rahulnakmol/agentic-ayudha/pull/1269) | draft | #1266 | fix(delegate): a host hand-off lapses with its claim |
| [#1260](https://github.com/rahulnakmol/agentic-ayudha/pull/1260) | ready | #1138 | fix(mission): a cloud Kanha gets text aayu did not write inside an untrusted block |
| [#1241](https://github.com/rahulnakmol/agentic-ayudha/pull/1241) | ready | - | fix(verify): make the weekly-pass checks prove the claims they skipped |
| [#1231](https://github.com/rahulnakmol/agentic-ayudha/pull/1231) | ready | - | fix(verify): a Playwright with an invalid package.json is an error |
| [#1202](https://github.com/rahulnakmol/agentic-ayudha/pull/1202) | ready | #1198 | fix(config): a config error names a rejected secret field and its rule |
| [#1189](https://github.com/rahulnakmol/agentic-ayudha/pull/1189) | ready | - | docs(security): name the halt claim-release invariant and its test |
| [#1183](https://github.com/rahulnakmol/agentic-ayudha/pull/1183) | ready | #1161 | fix(proof): a gate run that another PR from the branch may have had |
| [#817](https://github.com/rahulnakmol/agentic-ayudha/pull/817) | ready | - | docs(design): add the aayu iconography |
| [#604](https://github.com/rahulnakmol/agentic-ayudha/pull/604) | draft | #594 | feat: create the owner's aayudha App once from a phone |
| [#546](https://github.com/rahulnakmol/agentic-ayudha/pull/546) | draft | - | chore(inspect): list the Claude GitHub app as a reviewer app |

## To restart

1. `aayu resume` (a person) lifts the halt.
2. Run the label restore script, if it has not run.
3. Re-enable the loop schedules and the `triage-opened` subscription that are still wanted (each loop's Amp thread
   is in `loops/active.json`), or run `aayu install --loops` again.
4. For each Feature that restarts: its baselines, then Approve to build, then `aayu mission start` (each Feature's
   `06-sequencing.md` lists the person steps).
