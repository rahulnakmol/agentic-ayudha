# rahulnakmol/agentic-ayudha#1178: The overnight card: the person sets what runs overnight, bounded in hours and cost

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `dcf995a45d2b7d7d82d3e10138881530f829176e1b5cb01545227eb7283ddd33` |
| Completion Goal hash | `aae6614a8e4d8bd5d38b5ac9c1def48e4133e947b57d7c66063a1ef66de2bd45` |
| Approved to build | #1170 approval 2 by @rahulnakmol |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1178/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: the evening review is done and #982 is next in the release order; expected: an Overnight card proposes: author #982 to the approval line, the verify sweep, and the evals, each with its hours and cost bound
2. [api-trace] state: he approves it; expected: missions/overnight/<date>.json records each run, its bounds, and its end at the next start of `[time] business_hours`, under the session's code
3. [api-trace] state: after the end of `[time] business_hours`, a mission has a startable story; expected: `aayu mission next` starts nothing, and says `outside the day: starts at <time>`
4. [api-trace] state: an overnight run reaches its hour or cost bound; expected: it stops, and the morning inbox names it
5. [api-trace] state: he leaves the Overnight card; expected: no overnight run starts except loops scheduled before

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

As @rahulnakmol, I want to set in the evening what runs overnight, so that tomorrow's Feature is ready for the morning and nothing runs that I did not set.

### Completion Goal

1. [api-trace] state: the evening review is done and #982 is next in the release order; expected: an Overnight card proposes: author #982 to the approval line, the verify sweep, and the evals, each with its hours and cost bound
2. [api-trace] state: he approves it; expected: missions/overnight/<date>.json records each run, its bounds, and its end at the next start of `[time] business_hours`, under the session's code
3. [api-trace] state: after the end of `[time] business_hours`, a mission has a startable story; expected: `aayu mission next` starts nothing, and says `outside the day: starts at <time>`
4. [api-trace] state: an overnight run reaches its hour or cost bound; expected: it stops, and the morning inbox names it
5. [api-trace] state: he leaves the Overnight card; expected: no overnight run starts except loops scheduled before

### Measure

Completion Goal criteria observed

### Threshold

all 5 criteria

### Scenario

Approve an overnight plan, list a mission after hours, let a run hit its bound, and leave the card once.

### Environment

go test of internal/cli with a fake GitHub, a fake ledger, a fake clock, and a fake device flow; never production

### Links

depends-on #1176

### Checkpoints

1. The card proposes and records bounded runs (criteria 1, 2, 5)
2. Missions start no story outside the day (criteria 3)
3. A run that hits its bound stops and reports (criteria 4)

### Assumptions

#### Notes

Ship is not changed: a PR started in the day may merge after the day ends, and it is first in the next evening review. Authoring writes drafts only; every approval stays in the morning.

#### Review guidance

- Check: criterion 3: missions start nothing outside the day
- Check: criterion 4: bounds come from config, never from a prompt
- Ignore: the order of the proposed runs
- Risk: an overnight run with no bound

#### Behaviours

1. says: Nothing starts overnight that the person did not set in the evening; rule: never; source: criteria 3, 5
2. says: Each overnight run stops at its bound; rule: limit; source: criteria 4

### Provenance

Request: "I start my day by answering a series of questionnaires about the things a human needs to review and act upon … At the end of the day … I see that what I agreed upon in the morning has been implemented, delivered, and proven" (@rahulnakmol, 2026-10-30, Decision #1168)
1. stated (request): "Anything that needs to run overnight is properly scheduled"
2. stated (request): set in the evening session
3. stated (request): "I'm not looking for a 24/7 run"
4. default (scope): bounds as every loop
5. default (scope): nothing is set by default
````
