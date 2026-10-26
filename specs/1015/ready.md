# rahulnakmol/agentic-ayudha#1015: Mission time limits: warn at 24 hours, hold new starts at 48 hours, and a person's reason to start or extend past them

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `74e91b9507582d74de1d42aa9e0a471abaaf0461f3164e6fea8724c478abcecc` |
| Completion Goal hash | `440de60664610acd4757437174b6fa45f36f81c160c127ff261c1f3eef6e6a74` |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1015/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: Approve to build of #889, whose mission 1's longest chain is 13 stories (7 S, 4 M, 2 L), and #886's read gives learned working hours S 1 h, M 1.5 h, and L 3 h; expected: the estimate is `about 19 working hours (7 × 1 h + 4 × 1.5 h + 2 × 3 h; source: #886 history)`, and it passes with no warning
2. [api-trace] state: a mission's longest chain is 4 S stories with a learned 1 h each and 1 M story with no learned estimate, and #886's working-hour default for the story's effort (#886 names the key) is 22 h for M; expected: the estimate is `about 26 h (S: #886 history; M: #886 default)`, and Approve to build warns `over [mission] warn_hours (24)` and goes on
3. [api-trace] state: the learned working hours along the longest chain come to 60 h; expected: Approve to build approves with `Over 48 h: split this Feature, or override with a reason (person only)`, and plan.json marks the estimate `over 48 h`; `aayu mission start 889` with no reason is refused with `#889's estimate is 60 working hours, over 48 h: split this Feature, or start it with --reason`, and nothing starts
4. [api-trace] state: no size on the longest chain has a learned estimate, and #886's working-hour default for the story's effort (#886 names the key) brings it to 220 h; expected: Approve to build warns `about 220 h, no history: over [mission] max_hours (48)` and approves; it refuses nothing, and plan.json marks the estimate `no history`
5. [api-trace] state: the chain has 30 learned working hours and 40 default hours; expected: Approve to build warns and does not refuse, because the learned hours alone (30) are under 48; it says `30 h learned + 40 h default (no history for M)`
6. [api-trace] state: the estimate is marked `over 48 h`, and @rahulnakmol runs `aayu mission start 889 --reason "one migration"` (the start card asks for the reason); expected: after the presence check the mission starts, and plan.json records the override, its reason, and him; an agent that runs it gets `aayu: mission start is a person's decision and is refused for an agent lane (AMP_THREAD_ID)`
7. [api-trace] state: #889's mission 1 ran 30 h on the wall clock; for 5 h every running story waited on a person (an ask, a Ready to ship), for 2 h on a usage limit, and for 4 h on GitHub CI (the merge gate, evals, end-to-end tests, and the CI review); expected: `aayu mission show 889` says `19 h of 48 (11 h not counted: 5 h person, 2 h usage limit, 4 h CI)`, and missions/889/plan.json keeps each story's working total in `missions[1].stories[].working_hours`
8. [api-trace] state: story #20 waits on its CI review while story #21 is still working; expected: the clock runs, because at least one story thread works
9. [api-trace] state: #20's thread runs `.agents/check --full` before a push for 25 minutes; expected: the 25 minutes count toward the clock, because the local check is work, not a wait on GitHub CI
10. [api-trace] state: #889's mission time reaches 24 h; expected: one inbox card `#889 has run 24 h`, with no recommendation, and nothing stops
11. [screenshot] state: #889's mission time reaches 48 h, with 2 stories running and #25 startable; expected: `aayu mission next 889` shows `waits #25: #889 reached max_hours (48 h)`, the 2 running stories run to their end, and one inbox card `#889 reached its 48 h time limit` says `new starts wait: the mission of #889 used its 48 working hours. Running stories keep going.` and `No recommendation: the mission gives none.`, with `1 Extend for 48 h, with a reason` (presence check) and `2 Stop the mission`
12. [api-trace] state: @rahulnakmol runs `aayu mission extend 889 --reason "two stories left"`; expected: after the presence check, new starts resume for another `[mission] max_hours`, and plan.json records the reason; an agent that runs it gets `aayu: mission extend is a person's decision and is refused for an agent lane (AMP_THREAD_ID)`
13. [api-trace] state: #889's whole plan is estimated at 70 h, mission 1 at 30 h and mission 2 at 40 h; expected: Approve to build warns for both missions (`over [mission] warn_hours (24)`) and refuses none, because `max_hours` applies to each mission; mission 2 starts its own clock at 0 h

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

As @rahulnakmol, I want a long mission to tell me and a too-long mission to stop starting work, so that a Feature that should be split gets split.

### Completion Goal

1. [api-trace] state: Approve to build of #889, whose mission 1's longest chain is 13 stories (7 S, 4 M, 2 L), and #886's read gives learned working hours S 1 h, M 1.5 h, and L 3 h; expected: the estimate is `about 19 working hours (7 × 1 h + 4 × 1.5 h + 2 × 3 h; source: #886 history)`, and it passes with no warning
2. [api-trace] state: a mission's longest chain is 4 S stories with a learned 1 h each and 1 M story with no learned estimate, and #886's working-hour default for the story's effort (#886 names the key) is 22 h for M; expected: the estimate is `about 26 h (S: #886 history; M: #886 default)`, and Approve to build warns `over [mission] warn_hours (24)` and goes on
3. [api-trace] state: the learned working hours along the longest chain come to 60 h; expected: Approve to build approves with `Over 48 h: split this Feature, or override with a reason (person only)`, and plan.json marks the estimate `over 48 h`; `aayu mission start 889` with no reason is refused with `#889's estimate is 60 working hours, over 48 h: split this Feature, or start it with --reason`, and nothing starts
4. [api-trace] state: no size on the longest chain has a learned estimate, and #886's working-hour default for the story's effort (#886 names the key) brings it to 220 h; expected: Approve to build warns `about 220 h, no history: over [mission] max_hours (48)` and approves; it refuses nothing, and plan.json marks the estimate `no history`
5. [api-trace] state: the chain has 30 learned working hours and 40 default hours; expected: Approve to build warns and does not refuse, because the learned hours alone (30) are under 48; it says `30 h learned + 40 h default (no history for M)`
6. [api-trace] state: the estimate is marked `over 48 h`, and @rahulnakmol runs `aayu mission start 889 --reason "one migration"` (the start card asks for the reason); expected: after the presence check the mission starts, and plan.json records the override, its reason, and him; an agent that runs it gets `aayu: mission start is a person's decision and is refused for an agent lane (AMP_THREAD_ID)`
7. [api-trace] state: #889's mission 1 ran 30 h on the wall clock; for 5 h every running story waited on a person (an ask, a Ready to ship), for 2 h on a usage limit, and for 4 h on GitHub CI (the merge gate, evals, end-to-end tests, and the CI review); expected: `aayu mission show 889` says `19 h of 48 (11 h not counted: 5 h person, 2 h usage limit, 4 h CI)`, and missions/889/plan.json keeps each story's working total in `missions[1].stories[].working_hours`
8. [api-trace] state: story #20 waits on its CI review while story #21 is still working; expected: the clock runs, because at least one story thread works
9. [api-trace] state: #20's thread runs `.agents/check --full` before a push for 25 minutes; expected: the 25 minutes count toward the clock, because the local check is work, not a wait on GitHub CI
10. [api-trace] state: #889's mission time reaches 24 h; expected: one inbox card `#889 has run 24 h`, with no recommendation, and nothing stops
11. [screenshot] state: #889's mission time reaches 48 h, with 2 stories running and #25 startable; expected: `aayu mission next 889` shows `waits #25: #889 reached max_hours (48 h)`, the 2 running stories run to their end, and one inbox card `#889 reached its 48 h time limit` says `new starts wait: the mission of #889 used its 48 working hours. Running stories keep going.` and `No recommendation: the mission gives none.`, with `1 Extend for 48 h, with a reason` (presence check) and `2 Stop the mission`
12. [api-trace] state: @rahulnakmol runs `aayu mission extend 889 --reason "two stories left"`; expected: after the presence check, new starts resume for another `[mission] max_hours`, and plan.json records the reason; an agent that runs it gets `aayu: mission extend is a person's decision and is refused for an agent lane (AMP_THREAD_ID)`
13. [api-trace] state: #889's whole plan is estimated at 70 h, mission 1 at 30 h and mission 2 at 40 h; expected: Approve to build warns for both missions (`over [mission] warn_hours (24)`) and refuses none, because `max_hours` applies to each mission; mission 2 starts its own clock at 0 h

### Measure

Stories that a mission started after max_hours with no override.

### Threshold

0

### Scenario

#889's mission 1 is estimated at 19 learned working hours. At 24 h of mission time a card appears, and mission 1 ends at 22 h on the clock; plan.json keeps each story's working total.

### Environment

go test of internal/cli with a fake clock, ledger, and GitHub; never production.

### Links

depends-on #997
depends-on #998

### Assumptions

#### Notes

Depends on, outside this slice: #974, #886 S1.

Decision #974 (amends #915) is accepted. The estimate sums the time per size along the longest chain of each mission's story graph, because merges in a row set the time (D1 A: about 13 for the first mission). The waits are: on a person, an open ask (#880), a Ready to ship wait (internal/cli/ship.go), or a stopped mission or story (ST16); on a usage limit, a host's usage pause (ST2 criterion 2); on GitHub CI, a running required check, eval, end-to-end test, or CI review. The inbox card's view is #895's. #886 S1 has no depends-on, so it lands early; until #886 S8 lands, its read returns the working-hour defaults for every size. A mixed estimate refuses only when its learned hours alone pass `max_hours` (criterion 5). A story's working total is the time its own thread worked, less its waits; this story keeps it in plan.json, and #872 ST8 copies it to its story line as `hours.working`. The card words come from 0.4.0 UX baseline v44, approved by @rahulnakmol (web card j-inbox-card-889, terminal frame tui-list). The override is given at Start, not at Approve to build (baseline v44, V1 A); `aayu mission start` is ST14's, and this story adds its `--reason` check.

#### Review guidance

check: Every hour limit is a key in ST6: `[mission] warn_hours = 24` and `max_hours = 48`. The hours per effort size are #886's working hours: learned (S8) when they exist, else #886's working-hour default for the story's effort, which #886 names; not the elapsed `[delivery.default_hours]`. This story adds no key for story time (R5). A refusal at Approve to build rests only on learned hours. The clock pauses only while every running story waits on a person, a usage limit, or GitHub CI; the local `.agents/check --full` counts (C2 A).
ignore: The card's wording (#895).
risk: A wrong clock refuses good work or lets a stuck mission run on. The clock is in the record, and `aayu mission show` prints it with its parts.

#### Behaviours

1. [limit] says: Past warn_hours, a mission only warns; source: Decision #974 (amends #915); criteria: 2, 10
2. [limit] says: Past max_hours, a mission starts no new story until a person overrides, and running stories finish; source: Decision #974 (amends #915); criteria: 3, 11
3. [limit] says: An estimate over 48 h on learned working hours starts only with a person's reason at Start; on default hours it only warns and says no history; source: @rahulnakmol 2026-10-23 (answer 1, A + both); Decision #974 (amends #915); 0.4.0 UX baseline v44 (V1 A); criteria: 3, 4, 5
4. [limit] says: The clock runs while at least one story thread works, and pauses while every running story waits on a person, a usage limit, or GitHub CI; source: C1 A and C2 A, Decision #974 (amends #915); criteria: 7, 8, 9
5. [authority] says: Only a person overrides a time limit, with a reason; source: Decision #974 (amends #915); presence check, internal/cli/presence.go; criteria: 6, 12
6. [limit] says: Each mission of a Feature has its own clock, estimate, and limits; source: X1 A, Decision #974 (amends #915); criteria: 13
````
