# rahulnakmol/agentic-ayudha#1172: A waiting baseline draft is an inbox card, approved under the session's one device code

### Ready snapshot

| | |
|---|---|
| Schema | work items v1, story |
| Body hash | `687c63d34142a4e37600954f14cc197c020b4c33a9884bd61d5118f56765969f` |
| Completion Goal hash | `787006b329b4ccb23454535dd94264ca9adbfdfdebab01817c5323700b7eae3d` |
| Approved to build | #1170 approval 2 by @rahulnakmol |
| Technical Owner | the Code Architect twin |
| Written by | rahulnakmol (`identity:human-fallback`) |
| Ledger | `specs/1172/ready.md` |

Completion Goal at Ready:

1. [recording] state: Feature #889 has intent, spec, and tech v1 drafts waiting, and @rahulnakmol opens `aayu inbox`; expected: three Baseline cards, one per part, each with the part, its version, what it pins, and the counsel of the part's owner seat
2. [api-trace] state: he approves the three cards in one inbox session; expected: GitHub's device flow asks for one code, and specs/889/baselines/intent-v1.json, spec-v1.json, and tech-v1.json are written with approved_by rahulnakmol, as `aayu ready 889 --part <part>` writes them
3. [api-trace] state: the spec of #889 changed after the card was read; expected: approving the spec card writes nothing, and the card says `The spec of #889 changed after the draft. An agent drafts it again.`
4. [api-trace] state: an agent lane runs `aayu inbox --list --json`; expected: the list has the three Baseline cards with their checklists, and no approve action
5. [screenshot] state: `aayu inbox --plain` shows the spec card of #889; expected: one line per story with its title, criteria count, and effort, then the checklist: each story has criteria, none is XL, and each is in the Feature's paths
6. [api-trace] state: a v2 tech draft waits after an approved v1; expected: its card shows each change since v1 with what changed, why, and the need behind it

An edit to the body after Ready removes `status:ready` (the Ready guard). Run `aayu ready` again to make a new snapshot.

## Body at Ready

````markdown
### Summary

As @rahulnakmol, I want each waiting baseline draft of a Feature to be a card in my morning inbox, so that I approve intent, spec, and tech with one device code instead of three.

### Completion Goal

1. [recording] state: Feature #889 has intent, spec, and tech v1 drafts waiting, and @rahulnakmol opens `aayu inbox`; expected: three Baseline cards, one per part, each with the part, its version, what it pins, and the counsel of the part's owner seat
2. [api-trace] state: he approves the three cards in one inbox session; expected: GitHub's device flow asks for one code, and specs/889/baselines/intent-v1.json, spec-v1.json, and tech-v1.json are written with approved_by rahulnakmol, as `aayu ready 889 --part <part>` writes them
3. [api-trace] state: the spec of #889 changed after the card was read; expected: approving the spec card writes nothing, and the card says `The spec of #889 changed after the draft. An agent drafts it again.`
4. [api-trace] state: an agent lane runs `aayu inbox --list --json`; expected: the list has the three Baseline cards with their checklists, and no approve action
5. [screenshot] state: `aayu inbox --plain` shows the spec card of #889; expected: one line per story with its title, criteria count, and effort, then the checklist: each story has criteria, none is XL, and each is in the Feature's paths
6. [api-trace] state: a v2 tech draft waits after an approved v1; expected: its card shows each change since v1 with what changed, why, and the need behind it

### Measure

Completion Goal criteria observed

### Threshold

all 6 criteria

### Scenario

Draft three baselines of a Feature, open the inbox, approve all three in one session, then repeat with a draft that drifted and with an agent lane.

### Environment

go test of internal/cli with a fake GitHub, a fake ledger, a fake clock, and a fake device flow; never production

### Checkpoints

1. Baseline cards show each waiting draft with its pins and checklist (criteria 1, 5, 6)
2. Approving them in one session writes each baseline after one device code (criteria 2, 3)
3. An agent lane sees the cards and cannot approve (criteria 4)

### Assumptions

#### Notes

The card approves through the code that `aayu ready <f> --part <p>` runs (`partRun.approve`, internal/cli/ready.go), so the hash a person sees is the hash written. One inbox session asks for one code (`presenceCheck`, internal/cli/presence.go). The UX part has a card only when the Feature's footprint has L01.

#### Review guidance

- Check: criterion 3: the draft and pins are read again after the code, as `partRun.approve` does
- Check: criterion 4: an agent lane gets no approve action
- Ignore: the wording of the counsel
- Risk: a card that approves a different hash than the one shown

#### Behaviours

1. says: Each waiting baseline draft is a card, approved once with the session's code; rule: limit; source: criteria 1, 2
2. says: A draft that changed after it was read is never approved; rule: never; source: criteria 3
3. says: An agent lane never approves a baseline; rule: never; source: criteria 4

### Provenance

Request: "I start my day by answering a series of questionnaires about the things a human needs to review and act upon … At the end of the day … I see that what I agreed upon in the morning has been implemented, delivered, and proven" (@rahulnakmol, 2026-10-30, Decision #1168)
1. stated (request): the morning is a set of cards a person answers
2. stated (request): one code a session
3. default (scope): the same rule as `aayu ready --part`
4. default (scope): agents read, never approve
5. default (scope): the spec card shows what the person agrees
6. default (scope): a new version says why, as Story #993 asks
````
