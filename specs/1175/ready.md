# rahulnakmol/agentic-ayudha#1175: The morning is set: the inbox orders the day's cards and ends with the day's plan

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `1e36799ba990c04b56b66d39fc3613d5305e65bff7f53080fb829ae9aaecdd7e` |
| Completion Goal hash | `0951e4eb9d78a6c88712d9600f903a4fde92791dbc874f4e80672399523eef4f` |
| Approved to build | #1170 approval 2 by @rahulnakmol |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1175/ready.md` |

Completion Goal at Ready:

1. [recording] state: a Decision, #889's three Baseline cards, its Approve to build card, and its Mission start card wait; expected: `aayu inbox` shows them in that order: decisions, baselines, Approve to build, mission start, then the other cards
2. [screenshot] state: he approved #889 and started its mission in the session; expected: the last card says `The day is set` with one line per started Feature: today's milestone, its stories, and the evening review at the end of `[time] business_hours`
3. [api-trace] state: he leaves before the Mission start card; expected: no `The day is set` line, and the cards wait for the next session

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

As @rahulnakmol, I want the morning cards in the order I decide them, and a last line that says the day is set, so that I know what will be built today and when I see it.

### Completion Goal

1. [recording] state: a Decision, #889's three Baseline cards, its Approve to build card, and its Mission start card wait; expected: `aayu inbox` shows them in that order: decisions, baselines, Approve to build, mission start, then the other cards
2. [screenshot] state: he approved #889 and started its mission in the session; expected: the last card says `The day is set` with one line per started Feature: today's milestone, its stories, and the evening review at the end of `[time] business_hours`
3. [api-trace] state: he leaves before the Mission start card; expected: no `The day is set` line, and the cards wait for the next session

### Measure

Completion Goal criteria observed

### Threshold

all 3 criteria

### Scenario

Open the inbox with the morning's cards, take them all, then repeat and leave before the start.

### Environment

go test of internal/cli with a fake GitHub, a fake ledger, a fake clock, and a fake device flow; never production

### Links

depends-on #1172
depends-on #1173

### Checkpoints

1. The day's cards come in decision order (criteria 1)
2. A started day ends with its plan (criteria 2, 3)

### Assumptions

#### Notes

Only the order and the last line change; each card keeps its own action.

#### Review guidance

- Check: criterion 2: the line names only Features started in this session
- Ignore: the colours of the last card
- Risk: a set line for a Feature that did not start

#### Behaviours

1. says: The morning ends with what will be built today and when it is reviewed; rule: limit; source: criteria 2

### Provenance

Request: "I start my day by answering a series of questionnaires about the things a human needs to review and act upon … At the end of the day … I see that what I agreed upon in the morning has been implemented, delivered, and proven" (@rahulnakmol, 2026-10-30, Decision #1168)
1. stated (request): "a series of questionnaires"
2. stated (request): "what I agreed upon in the morning"
3. default (scope): nothing is set until the start
````
