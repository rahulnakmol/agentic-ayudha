# rahulnakmol/agentic-ayudha#997: Approve to build writes the mission plan, and Start records the starter, the tool, and the budget

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `0f11472454e568be0cb10c352d5f91c8636a250bc3e2725d8bc4a9598d5762e7` |
| Completion Goal hash | `2ac0768f2850d7669320abe62fb32b629cbf498c7fe9369347e9eaa2fef426c7` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/997/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: @rahulnakmol approves #889 to build with milestones MS1 (#20, #21) and MS2 (#22, #23), then presses `1 Start the mission` on #889's mission start card, which runs `aayu mission start 889`, and passes the presence check; expected: missions/889/plan.json on the ledger holds the story graph, each story's milestone, agents, the budget, `started_by: rahulnakmol`, `tool: cursor` from his user config's entry for this repository, and the estimate `runs ≥ 4 stories + 2 × 2 milestones = 8`
2. [api-trace] state: a draft plan leaves story #24 and Bug #25 in no milestone; expected: Approve to build refuses with `#24 and #25 are in no milestone: every story and Bug belongs to one`
3. [api-trace] state: the plan names one milestone with all stories; expected: it passes; the gate runs when the last story merges
4. [api-trace] state: the Tatkal Feature #920 of Release #906 is approved once with its ad hoc budget; expected: its plan has no Feature-done milestone, and its mission is complete only when Release #906 is ready to promote and publish
5. [api-trace] state: `aayu mission show 889`; expected: it prints each milestone with its stories and state, the agents value, the budget used and left, and the estimate, each number with its source; when the record holds a pause, it prints one row `paused: <reason>` with the reason the record gives, never `quiet`
6. [api-trace] state: #889 started on cursor, and @rahulnakmol then sets his tool for this repository to claude; expected: #889's Kanha, its story threads, its orbs, and a teammate's `aayu mission next 889` still use cursor from missions/889/plan.json; the new tool applies only to missions he starts later
7. [api-trace] state: @rahulnakmol approves #889 to build; #20 and #21 have no depends-on outside #889, and #22 depends on #874's story #950, which has not merged; expected: plan.json's `missions` has mission 1 with #20 and #21, and #22 waits for mission 2 with `waits on #950 (#874)`
8. [api-trace] state: #950 merges; expected: mission 2 of #889 starts with #22 and every other story whose depends-on outside #889 have merged, with no new Approve to build, on the tool and under the approval recorded in plan.json
9. [api-trace] state: #886 estimates #889's cost at $420, range $360–$660; expected: the start card shows `BUDGET $660 · the top of the range, locked when you start`, and Start writes `budget: 660` with its basis to plan.json; the budget does not change while the mission runs
10. [api-trace] state: an agent runs `aayu mission start 889`; expected: it is refused with `aayu: mission start is a person's decision and is refused for an agent lane (AMP_THREAD_ID)`, and nothing is written

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

As @rahulnakmol, I want one mission plan that I approve with the Feature and start with one press, so that every story has a milestone and the mission has numbers before it starts.

### Completion Goal

1. [api-trace] state: @rahulnakmol approves #889 to build with milestones MS1 (#20, #21) and MS2 (#22, #23), then presses `1 Start the mission` on #889's mission start card, which runs `aayu mission start 889`, and passes the presence check; expected: missions/889/plan.json on the ledger holds the story graph, each story's milestone, agents, the budget, `started_by: rahulnakmol`, `tool: cursor` from his user config's entry for this repository, and the estimate `runs ≥ 4 stories + 2 × 2 milestones = 8`
2. [api-trace] state: a draft plan leaves story #24 and Bug #25 in no milestone; expected: Approve to build refuses with `#24 and #25 are in no milestone: every story and Bug belongs to one`
3. [api-trace] state: the plan names one milestone with all stories; expected: it passes; the gate runs when the last story merges
4. [api-trace] state: the Tatkal Feature #920 of Release #906 is approved once with its ad hoc budget; expected: its plan has no Feature-done milestone, and its mission is complete only when Release #906 is ready to promote and publish
5. [api-trace] state: `aayu mission show 889`; expected: it prints each milestone with its stories and state, the agents value, the budget used and left, and the estimate, each number with its source; when the record holds a pause, it prints one row `paused: <reason>` with the reason the record gives, never `quiet`
6. [api-trace] state: #889 started on cursor, and @rahulnakmol then sets his tool for this repository to claude; expected: #889's Kanha, its story threads, its orbs, and a teammate's `aayu mission next 889` still use cursor from missions/889/plan.json; the new tool applies only to missions he starts later
7. [api-trace] state: @rahulnakmol approves #889 to build; #20 and #21 have no depends-on outside #889, and #22 depends on #874's story #950, which has not merged; expected: plan.json's `missions` has mission 1 with #20 and #21, and #22 waits for mission 2 with `waits on #950 (#874)`
8. [api-trace] state: #950 merges; expected: mission 2 of #889 starts with #22 and every other story whose depends-on outside #889 have merged, with no new Approve to build, on the tool and under the approval recorded in plan.json
9. [api-trace] state: #886 estimates #889's cost at $420, range $360–$660; expected: the start card shows `BUDGET $660 · the top of the range, locked when you start`, and Start writes `budget: 660` with its basis to plan.json; the budget does not change while the mission runs
10. [api-trace] state: an agent runs `aayu mission start 889`; expected: it is refused with `aayu: mission start is a person's decision and is refused for an agent lane (AMP_THREAD_ID)`, and nothing is written

### Measure

Missions started with a story in no milestone, or with no estimate.

### Threshold

0

### Scenario

@rahulnakmol approves #889 with four milestones. The plan shows 15 stories, 4 gates, at least 23 runs, and 900 USD before anything starts.

### Environment

go test of internal/cli with a fake ledger and seeded issues; never production.

### Assumptions

#### Notes

Depends on, outside this slice: #874, #877 S1, #877 S2, #915.

The verb is `aayu mission` and its model lane is `mission`: every command, flag, label, lane, and key is common English, and Yagna and Kanha stay in the docs (M10, 2026-10-22). `internal/cli/mission.go` is a new file in L02. D1 A (@rahulnakmol, 2026-10-23): MS1 to MS3 build right after #874 and #877 S1 and S2; they do not wait for #875 or #876, which run beside them. ST14 is the root of #889's graph, so every later story waits for these through it. X1 A (Decision #974 (amends #915)): a Feature may run more than one mission, one at a time; each has its own number, stories, and clock in plan.json, and the approval, tool, starter, and lock are the Feature's. `aayu mission start <f>` is this story's: it is human-only and writes the starter, the tool, and the budget; ST18 adds the start card's words and the lock, and ST9 starts the Kanha on it.

#### Review guidance

check: The plan's graph and milestones are approved at Approve to build, which is human-only (#877, #903); the starter, the tool, and the budget are written at Start, which is human-only too (0.4.0 UX baseline v44, approved by @rahulnakmol). The Tatkal rule comes from #877 S8; this story only records the long-lived mission. The cost range is #886's.
ignore: The layout of `aayu mission show`.
risk: A plan that a bot path can rewrite changes what runs. Only a person's approval writes or changes it; a re-plan is a proposal (Decision #915).

#### Behaviours

1. [never] says: Every story and Bug of a mission belongs to exactly one milestone; source: Decision #915 (milestones); your rules of 2026-10-22; criteria: 2
2. [authority] says: Only a person's Approve to build or Start writes or changes a mission plan; source: Decision #915; #903; 0.4.0 UX baseline v44, approved by @rahulnakmol; criteria: 1, 10
3. [authority] says: A mission's tool is the tool of the person who started it, fixed in the plan at start; source: R9 per person, @rahulnakmol 2026-10-23; criteria: 1
4. [never] says: A mission never switches tools mid-flight; everyone reads its tool from the plan, never from a config; source: R9, @rahulnakmol 2026-10-23 (final tool rule); criteria: 6
5. [order] says: A mission takes only stories whose depends-on in other Features have merged; the Feature's next mission takes the rest with no new Approve to build; source: X1 A, Decision #974 (amends #915); criteria: 7, 8
6. [limit] says: A mission's budget is the top of its cost estimate's range, fixed at Start; source: 0.4.0 UX baseline v44, approved by @rahulnakmol (Mission start card); criteria: 9
````
