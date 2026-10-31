# rahulnakmol/agentic-ayudha#1179: aayu grill edits a filed body before Ready, and checks link targets as Ready does

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `05f328a7488ac6b4f7fed138c7de265a17d565d4e7ba5ee412c5e4c561c21b2a` |
| Completion Goal hash | `655c09a3b029d46d39cc074268de030c21445e23fde8801cbb4b76080bec898f` |
| Approved to build | #1170 approval 2 by @rahulnakmol |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1179/ready.md` |

Completion Goal at Ready:

1. [api-trace] state: Bug #1169 is status:planned and its Links fail Ready; `aayu grill bug --edit 1169 --body <new>` runs; expected: grill checks the new body, writes it through the actions log, and prints the diff
2. [api-trace] state: the new body fails Create; expected: nothing is written, and each problem is listed
3. [api-trace] state: the issue is status:ready; expected: it is refused with `#1169 is Ready; its body is frozen`
4. [api-trace] state: a running mission locks the issue; expected: it is refused, naming the mission
5. [api-trace] state: --title is given with --edit; expected: the title and the body are written in one write
6. [api-trace] state: a Bug draft has `caused-by #999`, and #999 is a Story; expected: `aayu grill bug --dry-run` reports `caused-by #999: #999 is a story; caused-by links to an incident`, as `aayu ready` does
7. [api-trace] state: a Feature draft has its Expected paths on one line, separated by commas; expected: `aayu grill feature --dry-run` and `aayu slice` read one path per comma, the same paths

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

As an agent, I want to change a filed issue's body through aayu, checked as grill checks a draft, so that no change goes through a paste script.

### Completion Goal

1. [api-trace] state: Bug #1169 is status:planned and its Links fail Ready; `aayu grill bug --edit 1169 --body <new>` runs; expected: grill checks the new body, writes it through the actions log, and prints the diff
2. [api-trace] state: the new body fails Create; expected: nothing is written, and each problem is listed
3. [api-trace] state: the issue is status:ready; expected: it is refused with `#1169 is Ready; its body is frozen`
4. [api-trace] state: a running mission locks the issue; expected: it is refused, naming the mission
5. [api-trace] state: --title is given with --edit; expected: the title and the body are written in one write
6. [api-trace] state: a Bug draft has `caused-by #999`, and #999 is a Story; expected: `aayu grill bug --dry-run` reports `caused-by #999: #999 is a story; caused-by links to an incident`, as `aayu ready` does
7. [api-trace] state: a Feature draft has its Expected paths on one line, separated by commas; expected: `aayu grill feature --dry-run` and `aayu slice` read one path per comma, the same paths

### Measure

Completion Goal criteria observed

### Threshold

all 7 criteria

### Scenario

Edit a planned Bug's body, try a body that fails, a Ready issue, a locked issue, and a title change; check a draft whose link names a Story.

### Environment

go test of internal/cli with a fake GitHub, a fake ledger, a fake clock, and a fake device flow; never production

### Checkpoints

1. --edit writes a checked body before Ready (criteria 1, 2, 5)
2. --edit refuses a Ready or locked issue (criteria 3, 4)
3. grill checks link targets as Ready does, and slice reads comma-separated paths (criteria 6, 7)

### Assumptions

#### Notes

On 2026-10-30, `aayu grill bug --dry-run` passed #1169, and `aayu ready 1169` refused its `caused-by #999` link, so a paste script had to fix the body. The same day #1170's comma-separated Expected paths passed grill and failed slice (`planning.Paths`, internal/planning/slice.go:124 splits on lines only); #982 and #895 have the same shape. The Ready guard and the mission lock stay the owners of frozen bodies.

#### Review guidance

- Check: criterion 3: a Ready body is never edited
- Check: criterion 6: grill and Ready give the same answer
- Ignore: the diff's format
- Risk: an edit that bypasses the Ready guard

#### Behaviours

1. says: A filed body changes only through grill, and only before Ready; rule: never; source: criteria 1, 3, 4
2. says: grill passes a draft only when Ready and slice would; rule: limit; source: criteria 6, 7

### Provenance

Request: "I start my day by answering a series of questionnaires about the things a human needs to review and act upon … At the end of the day … I see that what I agreed upon in the morning has been implemented, delivered, and proven" (@rahulnakmol, 2026-10-30, Decision #1168)
1. stated (request): "use aayu to build aayu but … make this frictionless"
2. default (scope): the same checks as a draft
3. default (scope): Ready freezes a body
4. default (scope): a mission lock freezes a body
5. default (scope): one write
6. stated (request): the friction of 2026-10-30, #1169
7. stated (request): the friction of 2026-10-30, #1170
````
