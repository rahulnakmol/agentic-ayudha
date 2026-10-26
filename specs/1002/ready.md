# rahulnakmol/agentic-ayudha#1002: aayu mission stop brings a mission or one story to a safe stop, resume continues it, and ship merges milestones in order

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `cef735cfd3a29074fec27b1b7b0d2d167e4e00a2f4e3a167cb4c4e5482bba641` |
| Completion Goal hash | `aaa199515da760f8f7d6e12564af285d1db2d12c2ec3acfaa1292507dea54291` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1002/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: `aayu mission stop 889 --reason "spec drift"` with 3 stories running; expected: after the presence check, `aayu mission next 889` starts nothing, each running Amp, Claude, or Cursor story thread is told to commit at its next checkpoint and stop, and claims, branches, and PRs stay
2. [api-trace] state: two of the running stories are a Codex cloud task and a Copilot task; expected: the stop result names both as `not reached: <task URL>; it runs to its end, and its PR waits`
3. [api-trace] state: `aayu ship 61` for a PR of stopped mission #889; expected: ship refuses with `#889's mission is stopped (spec drift): resume it first`, and nothing merges
4. [api-trace] state: mission #874 runs, and the Kala loops run, while #889 is stopped; expected: both run on, and `AAYUDHA_HALT` is not set
5. [api-trace] state: `aayu mission resume 889`; expected: after the presence check, aayu reads missions/889/plan.json and GitHub, continues each stopped story thread, and starts stories up to agents
6. [api-trace] state: an agent runs `aayu mission stop 889`; expected: it is refused with `aayu: mission stop is a person's decision and is refused for an agent lane (AMP_THREAD_ID)` and `next: a person runs it in their own terminal, or presses Stop in aayu inbox`
7. [api-trace] state: story #70 of milestone MS2 has a green PR and inspect says merge, and MS1's gate has not passed; expected: `aayu ship 71` refuses with `#70 is in MS2: it merges after MS1's gate passes`, nothing merges, and #70 keeps its slot
8. [api-trace] state: MS1's gate passes on dev; expected: `aayu ship 71` merges #70's PR
9. [api-trace] state: `aayu mission stop 889 --story 858 --reason "wrong approach"` while #858 and #859 run; expected: after the presence check, #858's thread is told to stop its subagents, commit at its next checkpoint, and stop; #858 keeps its claim, its PR, and its slot; ship refuses #858's PR while it is stopped; #859 and the rest of #889 run on
10. [api-trace] state: `aayu mission resume 889 --story 858`; expected: only #858's thread continues, and `aayu mission show 889` no longer lists #858 as stopped
11. [api-trace] state: `aayu mission stop 889 --story 941 --reason "repro wrong"`, where #941 is a Bug of #889; expected: it stops as a story does in criterion 9, and `aayu mission resume 889 --story 941` continues it
12. [api-trace] state: @alice, whose tool for this repository is claude, runs `aayu mission stop 874 --reason "spec drift"`, and #874's recorded tool is codex; expected: after the presence check, the stop goes on missions/874/plan.json, ship refuses #874's PRs, each story thread her machine cannot reach (no Codex CLI or credentials) is named `not reached: <task URL>`, and #874's Kanha reads the stopped state at its next wake
13. [api-trace] state: @alice, on claude, runs `aayu mission resume 874`; expected: it is refused with `#874's mission runs on codex, the tool of @bob, who started it: run this there`, and nothing changes
14. [api-trace] state: @dana, whose tool for this repository is codex, runs `aayu mission resume 874`, and #874 was started by @bob on codex; expected: after the presence check, the mission resumes, missions/874/plan.json adds `{"control": "resume", "by": "dana", "tool": "codex", "at": ...}` to `controls`, and the action counts as @dana's, not @bob's
15. [api-trace] state: @bob runs `aayu mission stop 874 --end --reason "superseded"` on mission #874, which he started; #950 merged, and #951 and #952 are open with #951's PR open; expected: after the presence check, #950 stays merged, #951 and #952 lose their claims and go back to status:ready, #951's PR is closed, plan.json marks #874 `ended (superseded)`, `lock.state` becomes `ended` with `lock.end_reason: "superseded"` (ST18), and each locked issue gets one comment, for example on #951: `The mission of #874 ended (superseded) at 16:10, by @bob: “superseded”. #951 is no longer read-only. Its PR #31 is closed; the story is Ready again.`, and #874 no longer counts as @bob's work in flight
16. [api-trace] state: @dana runs `aayu mission stop 874 --end --reason "x"`, and #874 was started by @bob; expected: it is refused with `#874 was started by @bob: only @bob may end it`, and nothing changes
17. [api-trace] state: `aayu ship 981`, a fix PR that a `gate_fixes` entry of #889 lists, with `approval.sha256` equal to the current approved record; expected: ship merges it on proof, as it merges a story PR
18. [api-trace] state: `aayu ship 982`, a PR that says it fixes a gate of #889, and no `gate_fixes` entry lists it; expected: ship refuses with `#982 is not a listed gate fix of #889`, and nothing merges

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

As @rahulnakmol, I want to stop one mission in an extreme case and to merge each milestone only after the one before it is proven, so that dev only moves from one safe checkpoint to the next.

### Completion Goal

1. [api-trace] state: `aayu mission stop 889 --reason "spec drift"` with 3 stories running; expected: after the presence check, `aayu mission next 889` starts nothing, each running Amp, Claude, or Cursor story thread is told to commit at its next checkpoint and stop, and claims, branches, and PRs stay
2. [api-trace] state: two of the running stories are a Codex cloud task and a Copilot task; expected: the stop result names both as `not reached: <task URL>; it runs to its end, and its PR waits`
3. [api-trace] state: `aayu ship 61` for a PR of stopped mission #889; expected: ship refuses with `#889's mission is stopped (spec drift): resume it first`, and nothing merges
4. [api-trace] state: mission #874 runs, and the Kala loops run, while #889 is stopped; expected: both run on, and `AAYUDHA_HALT` is not set
5. [api-trace] state: `aayu mission resume 889`; expected: after the presence check, aayu reads missions/889/plan.json and GitHub, continues each stopped story thread, and starts stories up to agents
6. [api-trace] state: an agent runs `aayu mission stop 889`; expected: it is refused with `aayu: mission stop is a person's decision and is refused for an agent lane (AMP_THREAD_ID)` and `next: a person runs it in their own terminal, or presses Stop in aayu inbox`
7. [api-trace] state: story #70 of milestone MS2 has a green PR and inspect says merge, and MS1's gate has not passed; expected: `aayu ship 71` refuses with `#70 is in MS2: it merges after MS1's gate passes`, nothing merges, and #70 keeps its slot
8. [api-trace] state: MS1's gate passes on dev; expected: `aayu ship 71` merges #70's PR
9. [api-trace] state: `aayu mission stop 889 --story 858 --reason "wrong approach"` while #858 and #859 run; expected: after the presence check, #858's thread is told to stop its subagents, commit at its next checkpoint, and stop; #858 keeps its claim, its PR, and its slot; ship refuses #858's PR while it is stopped; #859 and the rest of #889 run on
10. [api-trace] state: `aayu mission resume 889 --story 858`; expected: only #858's thread continues, and `aayu mission show 889` no longer lists #858 as stopped
11. [api-trace] state: `aayu mission stop 889 --story 941 --reason "repro wrong"`, where #941 is a Bug of #889; expected: it stops as a story does in criterion 9, and `aayu mission resume 889 --story 941` continues it
12. [api-trace] state: @alice, whose tool for this repository is claude, runs `aayu mission stop 874 --reason "spec drift"`, and #874's recorded tool is codex; expected: after the presence check, the stop goes on missions/874/plan.json, ship refuses #874's PRs, each story thread her machine cannot reach (no Codex CLI or credentials) is named `not reached: <task URL>`, and #874's Kanha reads the stopped state at its next wake
13. [api-trace] state: @alice, on claude, runs `aayu mission resume 874`; expected: it is refused with `#874's mission runs on codex, the tool of @bob, who started it: run this there`, and nothing changes
14. [api-trace] state: @dana, whose tool for this repository is codex, runs `aayu mission resume 874`, and #874 was started by @bob on codex; expected: after the presence check, the mission resumes, missions/874/plan.json adds `{"control": "resume", "by": "dana", "tool": "codex", "at": ...}` to `controls`, and the action counts as @dana's, not @bob's
15. [api-trace] state: @bob runs `aayu mission stop 874 --end --reason "superseded"` on mission #874, which he started; #950 merged, and #951 and #952 are open with #951's PR open; expected: after the presence check, #950 stays merged, #951 and #952 lose their claims and go back to status:ready, #951's PR is closed, plan.json marks #874 `ended (superseded)`, `lock.state` becomes `ended` with `lock.end_reason: "superseded"` (ST18), and each locked issue gets one comment, for example on #951: `The mission of #874 ended (superseded) at 16:10, by @bob: “superseded”. #951 is no longer read-only. Its PR #31 is closed; the story is Ready again.`, and #874 no longer counts as @bob's work in flight
16. [api-trace] state: @dana runs `aayu mission stop 874 --end --reason "x"`, and #874 was started by @bob; expected: it is refused with `#874 was started by @bob: only @bob may end it`, and nothing changes
17. [api-trace] state: `aayu ship 981`, a fix PR that a `gate_fixes` entry of #889 lists, with `approval.sha256` equal to the current approved record; expected: ship merges it on proof, as it merges a story PR
18. [api-trace] state: `aayu ship 982`, a PR that says it fixes a gate of #889, and no `gate_fixes` entry lists it; expected: ship refuses with `#982 is not a listed gate fix of #889`, and nothing merges

### Measure

PRs of a stopped mission merged while it was stopped.

### Threshold

0

### Scenario

@rahulnakmol sees #889 drift at 15:00 and stops it. Its three stories commit and wait, #874 keeps building, and he resumes #889 at 16:30.

### Environment

go test of internal/cli (ship and mission) with a fake GitHub, a fake ledger, and stub adapters; never production.

### Links

depends-on #997
depends-on #998

### Assumptions

#### Notes

Depends on, outside this slice: #915, #974.

ship.go is in `[autonomy] always_person`, so @rahulnakmol ships this PR. `aayu halt` stays the emergency stop for a tool incident (threat model T18). Build ahead: a story of milestone k+1 may start and open its PR while milestone k is open (M9 A); `aayu mission next` does not hold it for that reason. The inbox's "Stop this story" (#895 TP53) runs `aayu mission stop <f> --story <n>`; the card is #895's. The caller's tool is the host aayu detects (`DetectHost`, internal/doctor/checks.go:229); the refusal names the recorded tool and the approver. A stop is safe from any tool because it writes the record, and ship and the Kanha read it. The check is on the tool only: any person on the recorded tool may run resume, recover, retry, and next. Every control (stop, resume, recover, retry) adds an entry to `controls` in plan.json. `--end` is the one control tied to a person: the starter in plan.json. In the Tatkal mission, each story records its own tool and starter (ST9), and a `--story` control checks that story's tool.

#### Review guidance

check: Stop and resume are human-only with the presence check. Both ship refusals (a stopped mission, and a milestone whose earlier gate has not passed) come before the merge call (ship.go:520-527). `aayu halt` is unchanged (Decision #915, M7).
ignore: The wording of the stop result.
risk: internal/cli/ship.go is in the narrow set (.aayudha/config.toml:77). A stop that a thread cannot hear leaves a task running; criterion 2 names it.

#### Behaviours

1. [never] says: Stopping one mission never stops another mission or a loop; source: Decision #915 (M7); criteria: 4
2. [never] says: No PR of a stopped mission merges; source: Decision #915 (controls); criteria: 3
3. [authority] says: Only a person stops or resumes a mission; source: Decision #915; presence check, internal/cli/presence.go; criteria: 1, 5, 6
4. [order] says: A milestone's PRs merge only after every earlier milestone passed its gate; source: M9 answer A, 2026-10-22 (build ahead, merge in order); #915; criteria: 7, 8
5. [never] says: Stopping one story never stops the rest of its mission; source: M11, accepted 2026-10-22; #895 TP53 (Stop this story); criteria: 9
6. [capability] says: A person may stop a mission or one of its stories from any tool; source: R9 per person, @rahulnakmol 2026-10-23; Tatkal R9 (#877); criteria: 12
7. [authority] says: Resume runs only in the mission's recorded tool; source: R9 per person, @rahulnakmol 2026-10-23; Tatkal R9 (#877); criteria: 13
8. [authority] says: The mission record names who ran each control, and the action counts as theirs; source: R9 per person, @rahulnakmol 2026-10-23 (tool check only, no person check); criteria: 14
9. [never] says: Ending a mission never reverts a merged story; source: Q8 A, @rahulnakmol 2026-10-23; criteria: 15
10. [authority] says: Only the person who started a mission ends it; source: Q8 A, @rahulnakmol 2026-10-23 (a mission you started); criteria: 16
11. [authority] says: Ship merges a gate fix PR only when the mission record lists it; source: Decision #974 (amends #915); criteria: 17, 18
````
