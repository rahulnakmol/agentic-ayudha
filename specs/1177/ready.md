# rahulnakmol/agentic-ayudha#1177: The day is proven: the evening says it only when every agreed criterion has its evidence

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `c24f991e211d5f957081d1561ddaf56545cff60f547da0ef17781efa0bccdc46` |
| Completion Goal hash | `22b2ccc7e484f5f0b507c1a1b077e94938c64c1891b4276b300c57d0790deaa1` |
| Approved to build | #1170 approval 2 by @rahulnakmol |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1177/ready.md` |

Completion Goal at Ready:

1. [recording] state: every story agreed this morning merged, every criterion has evidence in its pack, and he accepted each; expected: the inbox shows `The day is proven` with the count of stories and criteria, with the inbox's celebration, and records the milestone day-proven for the date
2. [screenshot] state: criterion 3 of #1011 has no evidence in its pack; expected: no proven line, and the card says `#1011 criterion 3 has no evidence` and the next step
3. [api-trace] state: `[ui] plain = true`; expected: the same words, with no motion
4. [api-trace] state: no Feature was started this morning; expected: no proven line

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

As @rahulnakmol, I want the evening to tell me the day is proven only when it is, so that the moment means something.

### Completion Goal

1. [recording] state: every story agreed this morning merged, every criterion has evidence in its pack, and he accepted each; expected: the inbox shows `The day is proven` with the count of stories and criteria, with the inbox's celebration, and records the milestone day-proven for the date
2. [screenshot] state: criterion 3 of #1011 has no evidence in its pack; expected: no proven line, and the card says `#1011 criterion 3 has no evidence` and the next step
3. [api-trace] state: `[ui] plain = true`; expected: the same words, with no motion
4. [api-trace] state: no Feature was started this morning; expected: no proven line

### Measure

Completion Goal criteria observed

### Threshold

all 4 criteria

### Scenario

Close a day with all evidence, with one criterion missing, in plain mode, and with nothing started.

### Environment

go test of internal/cli with a fake GitHub, a fake ledger, a fake clock, and a fake device flow; never production

### Links

depends-on #1176

### Checkpoints

1. The proven line comes only with full evidence (criteria 1, 2, 4)
2. Plain mode keeps the words (criteria 3)

### Assumptions

#### Notes

The celebration reuses `inboxCheer` (internal/cli/inboxcheer.go, Story #798) and the milestone hook that `aayu ship` uses for `Your first proven ship!` (internal/cli/ship.go:618).

#### Review guidance

- Check: criterion 2: a missing piece of evidence always stops the line
- Ignore: the words of the seat's voice line
- Risk: a proven line on partial evidence

#### Behaviours

1. says: The day is proven only when every agreed criterion has evidence; rule: never; source: criteria 1, 2

### Provenance

Request: "I start my day by answering a series of questionnaires about the things a human needs to review and act upon … At the end of the day … I see that what I agreed upon in the morning has been implemented, delivered, and proven" (@rahulnakmol, 2026-10-30, Decision #1168)
1. stated (request): "he is gratified that he has built something"
2. stated (request): "we have to be truthful and clear"
3. default (scope): plain mode, as every card
4. default (scope): nothing to prove, no line
````
